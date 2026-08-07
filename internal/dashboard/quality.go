package dashboard

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/qualityrecord"
	"github.com/subashram/fairway/internal/store"
)

const defaultQualityPageSize = 25

type QualityFilters struct {
	Search   string
	Status   string
	Role     string
	Profile  string
	Risk     string
	Page     int
	PageSize int
}

type QualityFilterOptions struct {
	Statuses []string
	Roles    []string
	Profiles []string
	Risks    []string
}

type QualitySummary struct {
	TotalTasks      int
	FilteredTasks   int
	VisibleTasks    int
	Present         int
	Missing         int
	Unavailable     int
	Conflicting     int
	ExternallyOwned int
	AttentionTasks  int
}

type QualityTaskRow struct {
	Task             store.Task
	Record           qualityrecord.Record
	Attention        bool
	AttentionReasons []string
}

type QualityViewData struct {
	View          string
	Filters       QualityFilters
	FilterOptions QualityFilterOptions
	Summary       QualitySummary
	Rows          []QualityTaskRow
	Pagination    TablePagination
	Sessions      []store.Session
	Activity      []store.Activity
	Groups        []RoleGroup
	TaskRoles     map[string]string
	ReadOnly      bool
}

func (s *Server) quality(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	timing := newDashboardTiming("quality", r)
	defer timing.logIfSlow()
	data, err := s.qualityViewData(r, timing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	if err := qualityTemplate.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	timing.add("template.quality", time.Since(start), "")
}

func (s *Server) qualityViewData(r *http.Request, timing *dashboardTiming) (QualityViewData, error) {
	if dashboardSnapshotCacheable(r) {
		key := dashboardSnapshotKey("quality", r)
		data, status, err := dashboardSnapshotGet(s.snapshots, key, func() (QualityViewData, error) {
			return s.buildQualityViewData(r, timing)
		})
		timing.add("quality.snapshot_cache", 0, "status="+status)
		return data, err
	}
	return s.buildQualityViewData(r, timing)
}

func (s *Server) buildQualityViewData(r *http.Request, timing *dashboardTiming) (QualityViewData, error) {
	filters := qualityFiltersFromRequest(r)
	var tasks []store.Task
	if err := timing.step("quality.tasks", func() error {
		var err error
		tasks, err = s.store.AllTasks(r.Context())
		return err
	}); err != nil {
		return QualityViewData{}, err
	}
	options := qualityFilterOptions(tasks)
	filtered := filterQualityTasks(tasks, filters)
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].UpdatedAt != filtered[j].UpdatedAt {
			return filtered[i].UpdatedAt > filtered[j].UpdatedAt
		}
		return filtered[i].Definition.ID < filtered[j].Definition.ID
	})
	pageTasks, pagination := paginateQualityTasks(filtered, filters)
	var records []qualityrecord.Record
	if err := timing.step("quality.records", func() error {
		var err error
		records, err = qualityrecord.BuildMany(r.Context(), s.store, pageTasks, s.now().UTC())
		return err
	}); err != nil {
		return QualityViewData{}, err
	}
	rows, summary := qualityRows(pageTasks, records)
	summary.TotalTasks = len(tasks)
	summary.FilteredTasks = len(filtered)
	summary.VisibleTasks = len(rows)
	sessions, _ := s.store.Sessions(r.Context(), false)
	activity, _ := s.store.Activity(r.Context(), defaultActivityLimit)
	return QualityViewData{
		View: "quality", Filters: filters, FilterOptions: options, Summary: summary, Rows: rows, Pagination: pagination,
		Sessions: sessions, Activity: activity, Groups: groupTasks(tasks, s.roles), TaskRoles: taskRoleMap(tasks), ReadOnly: s.cfg.Dashboard.ReadOnly,
	}, nil
}

func qualityFiltersFromRequest(r *http.Request) QualityFilters {
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := boundedControlChoice(query.Get("limit"), []int{10, 25, 50}, defaultQualityPageSize)
	return QualityFilters{
		Search: strings.TrimSpace(query.Get("q")), Status: strings.TrimSpace(query.Get("status")), Role: strings.TrimSpace(query.Get("role")),
		Profile: strings.TrimSpace(query.Get("profile")), Risk: strings.TrimSpace(query.Get("risk")), Page: page, PageSize: pageSize,
	}
}

func filterQualityTasks(tasks []store.Task, filters QualityFilters) []store.Task {
	search := strings.ToLower(filters.Search)
	out := make([]store.Task, 0, len(tasks))
	for _, task := range tasks {
		if search != "" && !strings.Contains(strings.ToLower(task.Definition.ID+" "+task.Definition.Title), search) ||
			filters.Status != "" && task.Status != filters.Status ||
			filters.Role != "" && task.Definition.Role != filters.Role ||
			filters.Profile != "" && task.Definition.Profile != filters.Profile ||
			filters.Risk != "" && task.Definition.RiskLevel != filters.Risk {
			continue
		}
		out = append(out, task)
	}
	return out
}

func qualityFilterOptions(tasks []store.Task) QualityFilterOptions {
	statuses, roles, profiles, risks := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, task := range tasks {
		addControlOption(statuses, task.Status)
		addControlOption(roles, task.Definition.Role)
		addControlOption(profiles, task.Definition.Profile)
		addControlOption(risks, task.Definition.RiskLevel)
	}
	return QualityFilterOptions{Statuses: sortedControlOptions(statuses), Roles: sortedControlOptions(roles), Profiles: sortedControlOptions(profiles), Risks: sortedControlOptions(risks)}
}

func paginateQualityTasks(tasks []store.Task, filters QualityFilters) ([]store.Task, TablePagination) {
	total := len(tasks)
	totalPages := (total + filters.PageSize - 1) / filters.PageSize
	if totalPages == 0 {
		totalPages = 1
	}
	page := filters.Page
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * filters.PageSize
	end := start + filters.PageSize
	if end > total {
		end = total
	}
	displayStart := 0
	if total > 0 {
		displayStart = start + 1
	}
	pagination := TablePagination{Page: page, PageSize: filters.PageSize, TotalRows: total, TotalPages: totalPages, Start: displayStart, End: end}
	if page > 1 {
		pagination.PrevHref = qualityPageHref(filters, page-1)
	}
	if page < totalPages {
		pagination.NextHref = qualityPageHref(filters, page+1)
	}
	return tasks[start:end], pagination
}

func qualityPageHref(filters QualityFilters, page int) string {
	values := url.Values{}
	for key, value := range map[string]string{"q": filters.Search, "status": filters.Status, "role": filters.Role, "profile": filters.Profile, "risk": filters.Risk} {
		if value != "" {
			values.Set(key, value)
		}
	}
	if filters.PageSize != defaultQualityPageSize {
		values.Set("limit", strconv.Itoa(filters.PageSize))
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	if encoded := values.Encode(); encoded != "" {
		return "/quality?" + encoded
	}
	return "/quality"
}

func qualityRows(tasks []store.Task, records []qualityrecord.Record) ([]QualityTaskRow, QualitySummary) {
	rows := make([]QualityTaskRow, 0, len(tasks))
	summary := QualitySummary{}
	for i, task := range tasks {
		if i >= len(records) {
			break
		}
		record := records[i]
		row := QualityTaskRow{Task: task, Record: record}
		for _, section := range record.Sections {
			switch section.State {
			case "present":
				summary.Present++
			case "missing":
				summary.Missing++
				row.AttentionReasons = append(row.AttentionReasons, section.Title+" missing")
			case "unavailable":
				summary.Unavailable++
				row.AttentionReasons = append(row.AttentionReasons, section.Title+" unavailable")
			case "conflicting":
				summary.Conflicting++
				row.AttentionReasons = append(row.AttentionReasons, section.Title+" conflicting")
			case "externally_owned":
				summary.ExternallyOwned++
			}
		}
		row.Attention = len(row.AttentionReasons) > 0
		if row.Attention {
			summary.AttentionTasks++
		}
		rows = append(rows, row)
	}
	return rows, summary
}
