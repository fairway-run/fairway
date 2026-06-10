package dashboard

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/rules"
	"github.com/subashram/fairway/internal/store"
)

const (
	defaultReportTableLimit = 25
	maxReportTableLimit     = 200
)

type ReportViewData struct {
	View          string            `json:"-"`
	Window        ReportWindow      `json:"window"`
	Filters       ReportFilters     `json:"filters"`
	FilterOptions FilterOptions     `json:"filter_options"`
	Summary       ReportSummary     `json:"summary"`
	Lanes         []ReportLane      `json:"lanes"`
	Timeline      []ReportRun       `json:"timeline"`
	FollowUps     []ReportBucket    `json:"follow_ups"`
	ReviewSummary ReportReview      `json:"review_summary"`
	RuleSummary   ReportRuleSummary `json:"rule_summary"`
	Usage         ReportUsage       `json:"usage"`
	Rows          []ReportTaskRow   `json:"rows"`
	TableRows     []ReportTaskRow   `json:"-"`
	Pagination    TablePagination   `json:"pagination"`
	Sessions      []store.Session   `json:"-"`
	Activity      []store.Activity  `json:"-"`
	Groups        []RoleGroup       `json:"-"`
	TaskRoles     map[string]string
	ExportBase    string `json:"-"`
}

type ReportWindow struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Label     string `json:"label"`
}

type ReportFilters struct {
	Search             string
	Role               string
	Profile            string
	Kind               string
	OwningDomain       string
	RiskLevel          string
	Tags               []string
	IncludeBookkeeping bool
	TableLimit         int
	TablePage          int
}

type ReportSummary struct {
	DeliveryDone       int `json:"delivery_done"`
	BookkeepingDone    int `json:"bookkeeping_done"`
	MovingTasks        int `json:"moving_tasks"`
	CreatedTasks       int `json:"created_tasks"`
	BlockedOpened      int `json:"blocked_opened"`
	BlockedResolved    int `json:"blocked_resolved"`
	EvidenceRows       int `json:"evidence_rows"`
	FailedEvidence     int `json:"failed_evidence"`
	ReviewsRecorded    int `json:"reviews_recorded"`
	FollowUpsCreated   int `json:"follow_ups_created"`
	CIDeployUATPassed  int `json:"ci_deploy_uat_passed"`
	CIDeployUATFailed  int `json:"ci_deploy_uat_failed"`
	CIDeployUATRunning int `json:"ci_deploy_uat_running"`
	MissingReviews     int `json:"missing_reviews"`
	WorkBatches        int `json:"work_batches"`
	BatchedTasks       int `json:"batched_tasks"`
}

type ReportLane struct {
	Role           string          `json:"role"`
	Groups         []ReportGroup   `json:"groups"`
	Completed      int             `json:"completed"`
	Moving         int             `json:"moving"`
	Bookkeeping    int             `json:"bookkeeping"`
	LatestEvidence string          `json:"latest_evidence,omitempty"`
	Tasks          []ReportTaskRow `json:"tasks"`
}

type ReportGroup struct {
	Label     string          `json:"label"`
	Completed int             `json:"completed"`
	Moving    int             `json:"moving"`
	Tasks     []ReportTaskRow `json:"tasks"`
}

type ReportRun struct {
	Kind            string `json:"kind"`
	TaskID          string `json:"task_id"`
	TaskTitle       string `json:"task_title"`
	Identifier      string `json:"identifier"`
	Branch          string `json:"branch,omitempty"`
	Environment     string `json:"environment,omitempty"`
	Result          string `json:"result"`
	Artifact        string `json:"artifact,omitempty"`
	ElapsedSeconds  int    `json:"elapsed_seconds,omitempty"`
	FollowUpTaskIDs string `json:"follow_up_task_ids,omitempty"`
	At              string `json:"at"`
}

type ReportBucket struct {
	Prefix string          `json:"prefix"`
	Count  int             `json:"count"`
	Tasks  []ReportTaskRow `json:"tasks"`
}

type ReportReview struct {
	ReviewsRecorded int                   `json:"reviews_recorded"`
	EvidenceRows    int                   `json:"evidence_rows"`
	FailedEvidence  int                   `json:"failed_evidence"`
	MissingDomains  []ReportMissingReview `json:"missing_domains"`
}

type ReportUsage struct {
	Events      int                 `json:"events"`
	TotalTokens *int                `json:"total_tokens,omitempty"`
	ByProvider  []store.UsageRollup `json:"by_provider"`
	ByRole      []store.UsageRollup `json:"by_role"`
	ByKind      []store.UsageRollup `json:"by_kind"`
	ByPhase     []store.UsageRollup `json:"by_phase"`
	ByDay       []store.UsageRollup `json:"by_day"`
}

type ReportMissingReview struct {
	TaskID  string   `json:"task_id"`
	Title   string   `json:"title"`
	Domains []string `json:"domains"`
}

type ReportTaskRow struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Role             string   `json:"role"`
	Status           string   `json:"status"`
	Kind             string   `json:"kind"`
	Profile          string   `json:"profile,omitempty"`
	OwningDomain     string   `json:"owning_domain,omitempty"`
	RiskLevel        string   `json:"risk_level,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	DeliveryClass    string   `json:"delivery_class"`
	Completed        bool     `json:"completed"`
	Moving           bool     `json:"moving"`
	Created          bool     `json:"created"`
	EvidenceCount    int      `json:"evidence_count"`
	ReviewCount      int      `json:"review_count"`
	MissingReviews   []string `json:"missing_reviews,omitempty"`
	LatestEvidence   string   `json:"latest_evidence,omitempty"`
	LastActivityAt   string   `json:"last_activity_at"`
	FollowUpTaxonomy string   `json:"follow_up_taxonomy,omitempty"`
}

type reportTaskFacts struct {
	Task             store.Task
	Transitions      []store.Transition
	Evidence         []store.Evidence
	Reviews          []store.Review
	ReviewActivities []store.Activity
	EvidenceWindow   []store.Evidence
	Completed        bool
	Moving           bool
	Created          bool
	BlockedOpened    bool
	BlockedResolved  bool
	LastActivityAt   string
}

func (s *Server) reportViewData(r *http.Request) (ReportViewData, error) {
	now := time.Now()
	window, start, end := reportWindowFromQuery(r.URL.Query(), now, time.Local)
	filters := reportFiltersFromRequest(r)
	tasks, err := s.store.AllTasks(r.Context())
	if err != nil {
		return ReportViewData{}, err
	}
	sessions, err := s.store.Sessions(r.Context(), false)
	if err != nil {
		return ReportViewData{}, err
	}
	activity, err := s.store.ActivityFiltered(r.Context(), store.ActivityOptions{
		Limit:       maxActivityFetchLimit,
		CreatedFrom: start.Format(time.RFC3339Nano),
		CreatedTo:   end.Format(time.RFC3339Nano),
	})
	if err != nil {
		return ReportViewData{}, err
	}
	watchers, err := s.store.Watchers(r.Context(), true)
	if err != nil {
		return ReportViewData{}, err
	}
	batches, err := s.store.WorkBatches(r.Context())
	if err != nil {
		return ReportViewData{}, err
	}
	usage, err := s.reportUsage(r.Context(), start, end)
	if err != nil {
		return ReportViewData{}, err
	}
	reviewActivityByTask := reportReviewActivityByTask(activity, start, end)
	facts := make([]reportTaskFacts, 0, len(tasks))
	for _, task := range tasks {
		detailTask, transitions, evidence, _, reviews, err := s.store.TaskDetail(r.Context(), task.Definition.ID)
		if err != nil {
			return ReportViewData{}, err
		}
		fact := reportTaskFacts{Task: detailTask, Transitions: transitions, Evidence: evidence, Reviews: reviews, ReviewActivities: reviewActivityByTask[task.Definition.ID]}
		fact.applyWindow(start, end)
		if reportTaskInWindow(fact, start, end) {
			facts = append(facts, fact)
		}
	}
	rows := reportRowsFromFacts(facts, filters)
	packs, err := rules.LoadConfigured(s.cfg, s.root, rules.LoadOptions{
		Root:            s.root,
		KnownDomains:    rules.ReviewDomainSet(s.cfg),
		KnownEvidence:   rules.ConfigGateEvidenceSet(s.cfg),
		IncludeDisabled: true,
	})
	if err != nil {
		return ReportViewData{}, err
	}
	summaryFilters := filters
	summaryFilters.IncludeBookkeeping = true
	summaryRows := reportRowsFromFacts(facts, summaryFilters)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].LastActivityAt != rows[j].LastActivityAt {
			return rows[i].LastActivityAt > rows[j].LastActivityAt
		}
		return rows[i].ID < rows[j].ID
	})
	tableRows, pagination := paginateReportRows(rows, filters)
	return ReportViewData{
		View:          "reports",
		Window:        window,
		Filters:       filters,
		FilterOptions: filterOptions(tasks, activity, s.cfg.Fairway.ProjectName),
		Summary:       reportSummary(facts, summaryRows, watchers, batches, start, end),
		Lanes:         reportLanes(rows),
		Timeline:      reportTimeline(facts, watchers, rows, start, end),
		FollowUps:     reportFollowUps(rows),
		ReviewSummary: reportReviewSummary(facts, rows),
		RuleSummary:   reportRuleSummary(s.cfg, packs, facts),
		Usage:         usage,
		Rows:          rows,
		TableRows:     tableRows,
		Pagination:    pagination,
		Sessions:      sessions,
		Activity:      activity,
		Groups:        groupTasks(tasks, s.roles),
		TaskRoles:     taskRoleMap(tasks),
		ExportBase:    reportExportBase(r.URL.Query()),
	}, nil
}

func (s *Server) reportUsage(ctx context.Context, start, end time.Time) (ReportUsage, error) {
	since := start.UTC().Format(time.RFC3339Nano)
	until := end.UTC().Format(time.RFC3339Nano)
	byProvider, err := s.store.UsageRollups(ctx, store.UsageRollupOptions{GroupBy: "provider", Since: since, Until: until})
	if err != nil {
		return ReportUsage{}, err
	}
	byRole, err := s.store.UsageRollups(ctx, store.UsageRollupOptions{GroupBy: "role", Since: since, Until: until})
	if err != nil {
		return ReportUsage{}, err
	}
	byKind, err := s.store.UsageRollups(ctx, store.UsageRollupOptions{GroupBy: "kind", Since: since, Until: until})
	if err != nil {
		return ReportUsage{}, err
	}
	byPhase, err := s.store.UsageRollups(ctx, store.UsageRollupOptions{GroupBy: "phase", Since: since, Until: until})
	if err != nil {
		return ReportUsage{}, err
	}
	byDay, err := s.store.UsageRollups(ctx, store.UsageRollupOptions{GroupBy: "day", Since: since, Until: until})
	if err != nil {
		return ReportUsage{}, err
	}
	usage := ReportUsage{ByProvider: byProvider, ByRole: byRole, ByKind: byKind, ByPhase: byPhase, ByDay: byDay}
	for _, roll := range byProvider {
		usage.Events += roll.Events
		addReportUsageInt(&usage.TotalTokens, roll.TotalTokens)
	}
	return usage, nil
}

func addReportUsageInt(total **int, value *int) {
	if value == nil {
		return
	}
	if *total == nil {
		v := *value
		*total = &v
		return
	}
	**total += *value
}

func reportWindowFromQuery(query url.Values, now time.Time, loc *time.Location) (ReportWindow, time.Time, time.Time) {
	if loc == nil {
		loc = time.Local
	}
	today := now.In(loc)
	startDay := dateOnly(today)
	endDay := startDay
	label := "Today"
	switch strings.TrimSpace(query.Get("range")) {
	case "yesterday":
		startDay = startDay.AddDate(0, 0, -1)
		endDay = startDay
		label = "Yesterday"
	case "last7":
		startDay = startDay.AddDate(0, 0, -6)
		label = "Last seven days"
	}
	if parsed, ok := parseReportDate(query.Get("start"), loc); ok {
		startDay = parsed
		label = "Custom"
	}
	if parsed, ok := parseReportDate(query.Get("end"), loc); ok {
		endDay = parsed
		label = "Custom"
	}
	if endDay.Before(startDay) {
		startDay, endDay = endDay, startDay
	}
	endExclusive := endDay.AddDate(0, 0, 1)
	return ReportWindow{StartDate: startDay.Format("2006-01-02"), EndDate: endDay.Format("2006-01-02"), Label: label}, startDay, endExclusive
}

func parseReportDate(raw string, loc *time.Location) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, loc)
	if err != nil {
		return time.Time{}, false
	}
	return dateOnly(parsed), true
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func reportFiltersFromRequest(r *http.Request) ReportFilters {
	query := r.URL.Query()
	return ReportFilters{
		Search:             strings.TrimSpace(query.Get("q")),
		Role:               strings.TrimSpace(query.Get("role")),
		Profile:            strings.TrimSpace(query.Get("profile")),
		Kind:               strings.TrimSpace(query.Get("kind")),
		OwningDomain:       strings.TrimSpace(query.Get("owning_domain")),
		RiskLevel:          strings.TrimSpace(query.Get("risk_level")),
		Tags:               splitQueryValues(query["tag"]),
		IncludeBookkeeping: query.Get("include_bookkeeping") == "1" || query.Get("include_bookkeeping") == "true",
		TableLimit:         boundedQueryInt(query.Get("table_limit"), defaultReportTableLimit, maxReportTableLimit),
		TablePage:          boundedQueryInt(query.Get("page"), 1, 999999),
	}
}

func reportReviewActivityByTask(activity []store.Activity, start, end time.Time) map[string][]store.Activity {
	out := map[string][]store.Activity{}
	for _, item := range activity {
		if item.Kind != "review" || !timeInWindow(item.CreatedAt, start, end) {
			continue
		}
		out[item.TaskID] = append(out[item.TaskID], item)
	}
	return out
}

func (f *reportTaskFacts) applyWindow(start, end time.Time) {
	for _, transition := range f.Transitions {
		if !timeInWindow(transition.At, start, end) {
			continue
		}
		f.Moving = true
		f.LastActivityAt = maxTimeString(f.LastActivityAt, transition.At)
		if transition.ToStatus == "done" {
			f.Completed = true
		}
		if transition.ToStatus == "blocked" {
			f.BlockedOpened = true
		}
		if transition.FromStatus == "blocked" && transition.ToStatus != "blocked" {
			f.BlockedResolved = true
		}
		if transition.FromStatus == "" || transition.FromStatus == "new" {
			f.Created = true
		}
	}
	for _, ev := range f.Evidence {
		if timeInWindow(ev.CreatedAt, start, end) {
			f.EvidenceWindow = append(f.EvidenceWindow, ev)
			f.LastActivityAt = maxTimeString(f.LastActivityAt, ev.CreatedAt)
		}
	}
	for _, review := range f.ReviewActivities {
		f.LastActivityAt = maxTimeString(f.LastActivityAt, review.CreatedAt)
	}
	if timeInWindow(f.Task.UpdatedAt, start, end) {
		f.LastActivityAt = maxTimeString(f.LastActivityAt, f.Task.UpdatedAt)
	}
}

func reportTaskInWindow(f reportTaskFacts, start, end time.Time) bool {
	if f.LastActivityAt != "" {
		return true
	}
	for _, ev := range f.Evidence {
		if timeInWindow(ev.CreatedAt, start, end) {
			return true
		}
	}
	return false
}

func reportRowsFromFacts(facts []reportTaskFacts, filters ReportFilters) []ReportTaskRow {
	var rows []ReportTaskRow
	for _, fact := range facts {
		row := reportRowFromFact(fact)
		if !filters.IncludeBookkeeping && row.DeliveryClass == "bookkeeping" {
			continue
		}
		if filters.Search != "" && !reportRowMatchesSearch(row, filters.Search) {
			continue
		}
		if filters.Role != "" && row.Role != filters.Role {
			continue
		}
		if filters.Profile != "" && row.Profile != filters.Profile {
			continue
		}
		if filters.Kind != "" && row.Kind != filters.Kind {
			continue
		}
		if filters.OwningDomain != "" && row.OwningDomain != filters.OwningDomain {
			continue
		}
		if filters.RiskLevel != "" && row.RiskLevel != filters.RiskLevel {
			continue
		}
		if len(filters.Tags) > 0 && !containsAllStrings(row.Tags, filters.Tags) {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func reportRowFromFact(fact reportTaskFacts) ReportTaskRow {
	task := fact.Task
	missing := dashboardMissingApprovedReviewDomains(task.Definition.ReviewDomains, fact.Reviews)
	row := ReportTaskRow{
		ID:               task.Definition.ID,
		Title:            task.Definition.Title,
		Role:             task.Definition.Role,
		Status:           task.Status,
		Kind:             task.Definition.Kind,
		Profile:          task.Definition.Profile,
		OwningDomain:     task.Definition.OwningDomain,
		RiskLevel:        task.Definition.RiskLevel,
		Tags:             append([]string(nil), task.Definition.Tags...),
		DeliveryClass:    reportDeliveryClass(task),
		Completed:        fact.Completed,
		Moving:           fact.Moving,
		Created:          fact.Created,
		EvidenceCount:    len(fact.EvidenceWindow),
		ReviewCount:      len(fact.ReviewActivities),
		MissingReviews:   missing,
		LatestEvidence:   latestEvidenceArtifact(fact.Evidence),
		LastActivityAt:   fact.LastActivityAt,
		FollowUpTaxonomy: followUpTaxonomy(task.Definition.ID),
	}
	return row
}

func reportDeliveryClass(task store.Task) string {
	text := strings.ToLower(strings.Join([]string{
		task.Definition.ID,
		task.Definition.Title,
		task.Definition.Kind,
		task.Definition.Role,
		task.Definition.MigrationType,
		task.Definition.OwningLayer,
	}, " "))
	for _, keyword := range []string{"monitor", "watch", "deploy-run", "heartbeat", "ci-run", "uat-run", "smoke"} {
		if strings.Contains(text, keyword) {
			return "bookkeeping"
		}
	}
	return "delivery"
}

func reportRowMatchesSearch(row ReportTaskRow, raw string) bool {
	needle := strings.ToLower(strings.TrimSpace(raw))
	if needle == "" {
		return true
	}
	haystacks := []string{row.ID, row.Title, row.Role, row.Status, row.Kind, row.Profile, row.OwningDomain, row.RiskLevel, row.DeliveryClass, row.FollowUpTaxonomy}
	haystacks = append(haystacks, row.Tags...)
	hay := strings.Join(haystacks, " ")
	return strings.Contains(strings.ToLower(hay), needle)
}

func reportSummary(facts []reportTaskFacts, rows []ReportTaskRow, watchers []store.Watcher, batches []store.WorkBatch, start, end time.Time) ReportSummary {
	var summary ReportSummary
	summary.WorkBatches = len(batches)
	seenBatchTasks := map[string]bool{}
	for _, batch := range batches {
		for _, taskID := range batch.Tasks {
			seenBatchTasks[taskID] = true
		}
	}
	summary.BatchedTasks = len(seenBatchTasks)
	rowIDs := map[string]bool{}
	for _, row := range rows {
		rowIDs[row.ID] = true
		if row.Completed {
			if row.DeliveryClass == "bookkeeping" {
				summary.BookkeepingDone++
			} else {
				summary.DeliveryDone++
			}
		}
		if row.Moving {
			summary.MovingTasks++
		}
		if row.Created {
			summary.CreatedTasks++
		}
		if row.FollowUpTaxonomy != "" && row.Created {
			summary.FollowUpsCreated++
		}
		if len(row.MissingReviews) > 0 && row.Status == "done" {
			summary.MissingReviews++
		}
	}
	for _, fact := range facts {
		if !rowIDs[fact.Task.Definition.ID] {
			continue
		}
		if fact.BlockedOpened {
			summary.BlockedOpened++
		}
		if fact.BlockedResolved {
			summary.BlockedResolved++
		}
		for _, ev := range fact.Evidence {
			if !timeInWindow(ev.CreatedAt, start, end) {
				continue
			}
			summary.EvidenceRows++
			if evidenceFailed(ev.Result) {
				summary.FailedEvidence++
			}
			if evidenceLooksRun(ev) {
				switch evidenceRunResult(ev.Result) {
				case "pass":
					summary.CIDeployUATPassed++
				case "fail":
					summary.CIDeployUATFailed++
				default:
					summary.CIDeployUATRunning++
				}
			}
		}
		summary.ReviewsRecorded += len(fact.ReviewActivities)
	}
	for _, watcher := range watchers {
		if !timeInWindow(watcher.FinishedAt, start, end) {
			continue
		}
		switch watcher.Result {
		case "pass":
			summary.CIDeployUATPassed++
		case "fail", "blocked":
			summary.CIDeployUATFailed++
		default:
			summary.CIDeployUATRunning++
		}
	}
	return summary
}

func reportLanes(rows []ReportTaskRow) []ReportLane {
	byRole := map[string]int{}
	var lanes []ReportLane
	for _, row := range rows {
		role := row.Role
		if role == "" {
			role = "unassigned"
		}
		index, ok := byRole[role]
		if !ok {
			lanes = append(lanes, ReportLane{Role: role})
			index = len(lanes) - 1
			byRole[role] = index
		}
		lane := lanes[index]
		if row.Completed {
			lane.Completed++
		}
		if row.Moving {
			lane.Moving++
		}
		if row.DeliveryClass == "bookkeeping" {
			lane.Bookkeeping++
		}
		if lane.LatestEvidence == "" && row.LatestEvidence != "" {
			lane.LatestEvidence = row.LatestEvidence
		}
		lane.Tasks = append(lane.Tasks, row)
		lanes[index] = lane
	}
	for i := range lanes {
		lanes[i].Groups = reportGroups(lanes[i].Tasks)
	}
	sort.SliceStable(lanes, func(i, j int) bool {
		if lanes[i].Completed != lanes[j].Completed {
			return lanes[i].Completed > lanes[j].Completed
		}
		return lanes[i].Role < lanes[j].Role
	})
	return lanes
}

func reportGroups(rows []ReportTaskRow) []ReportGroup {
	byLabel := map[string]int{}
	var groups []ReportGroup
	for _, row := range rows {
		label := strings.Trim(strings.Join([]string{row.OwningDomain, row.Kind}, " / "), " /")
		if label == "" {
			label = "general"
		}
		index, ok := byLabel[label]
		if !ok {
			groups = append(groups, ReportGroup{Label: label})
			index = len(groups) - 1
			byLabel[label] = index
		}
		group := groups[index]
		if row.Completed {
			group.Completed++
		}
		if row.Moving {
			group.Moving++
		}
		if len(group.Tasks) < 4 {
			group.Tasks = append(group.Tasks, row)
		}
		groups[index] = group
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Completed != groups[j].Completed {
			return groups[i].Completed > groups[j].Completed
		}
		return groups[i].Label < groups[j].Label
	})
	return groups
}

func reportTimeline(facts []reportTaskFacts, watchers []store.Watcher, rows []ReportTaskRow, start, end time.Time) []ReportRun {
	taskByID := map[string]store.Task{}
	for _, fact := range facts {
		taskByID[fact.Task.Definition.ID] = fact.Task
	}
	followUps := followUpsBySource(rows)
	var runs []ReportRun
	for _, watcher := range watchers {
		if !timeInWindow(watcher.FinishedAt, start, end) {
			continue
		}
		task := taskByID[watcher.TaskID]
		runs = append(runs, ReportRun{
			Kind:            firstNonEmpty(watcher.Process, "watcher"),
			TaskID:          watcher.TaskID,
			TaskTitle:       task.Definition.Title,
			Identifier:      watcher.ID,
			Result:          firstNonEmpty(watcher.Result, watcher.Status),
			Artifact:        watcher.ArtifactPath,
			ElapsedSeconds:  intValue(watcher.DurationSeconds),
			FollowUpTaskIDs: strings.Join(followUps[watcher.TaskID], ", "),
			At:              watcher.FinishedAt,
		})
	}
	for _, fact := range facts {
		for _, ev := range fact.Evidence {
			if !timeInWindow(ev.CreatedAt, start, end) || !evidenceLooksRun(ev) {
				continue
			}
			runs = append(runs, ReportRun{
				Kind:            firstNonEmpty(ev.ArtifactType, "evidence"),
				TaskID:          fact.Task.Definition.ID,
				TaskTitle:       fact.Task.Definition.Title,
				Identifier:      firstNonEmpty(ev.ArtifactPath, ev.CommandText),
				Branch:          fact.Task.Branch,
				Result:          ev.Result,
				Artifact:        ev.ArtifactPath,
				ElapsedSeconds:  intValue(ev.DurationSeconds),
				FollowUpTaskIDs: strings.Join(followUps[fact.Task.Definition.ID], ", "),
				At:              ev.CreatedAt,
			})
		}
	}
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].At > runs[j].At
	})
	if len(runs) > 12 {
		return runs[:12]
	}
	return runs
}

func reportFollowUps(rows []ReportTaskRow) []ReportBucket {
	prefixes := []string{"CI-FIX", "CD-FIX", "UAT-BUG", "OPS-FIX", "HARNESS-FIX", "DOC-FIX"}
	byPrefix := map[string]int{}
	buckets := make([]ReportBucket, 0, len(prefixes))
	for _, prefix := range prefixes {
		buckets = append(buckets, ReportBucket{Prefix: prefix})
		byPrefix[prefix] = len(buckets) - 1
	}
	for _, row := range rows {
		if row.FollowUpTaxonomy == "" {
			continue
		}
		index := byPrefix[row.FollowUpTaxonomy]
		buckets[index].Count++
		if len(buckets[index].Tasks) < 5 {
			buckets[index].Tasks = append(buckets[index].Tasks, row)
		}
	}
	return buckets
}

func reportReviewSummary(facts []reportTaskFacts, rows []ReportTaskRow) ReportReview {
	var summary ReportReview
	for _, fact := range facts {
		summary.ReviewsRecorded += len(fact.ReviewActivities)
		for _, ev := range fact.EvidenceWindow {
			summary.EvidenceRows++
			if evidenceFailed(ev.Result) {
				summary.FailedEvidence++
			}
		}
	}
	for _, row := range rows {
		if row.Status == "done" && len(row.MissingReviews) > 0 {
			summary.MissingDomains = append(summary.MissingDomains, ReportMissingReview{TaskID: row.ID, Title: row.Title, Domains: row.MissingReviews})
		}
	}
	if len(summary.MissingDomains) > 8 {
		summary.MissingDomains = summary.MissingDomains[:8]
	}
	return summary
}

func paginateReportRows(rows []ReportTaskRow, filters ReportFilters) ([]ReportTaskRow, TablePagination) {
	pageSize := filters.TableLimit
	if pageSize <= 0 {
		pageSize = defaultReportTableLimit
	}
	if pageSize > maxReportTableLimit {
		pageSize = maxReportTableLimit
	}
	total := len(rows)
	totalPages := 1
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	page := filters.TablePage
	if page <= 0 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	startIndex, endIndex := 0, 0
	displayStart, displayEnd := 0, 0
	if total > 0 {
		startIndex = (page - 1) * pageSize
		endIndex = startIndex + pageSize
		if endIndex > total {
			endIndex = total
		}
		displayStart = startIndex + 1
		displayEnd = endIndex
	}
	pagination := TablePagination{Page: page, PageSize: pageSize, TotalRows: total, TotalPages: totalPages, Start: displayStart, End: displayEnd}
	if page > 1 {
		pagination.PrevHref = reportPageHref(filters, page-1)
	}
	if page < totalPages {
		pagination.NextHref = reportPageHref(filters, page+1)
	}
	return rows[startIndex:endIndex], pagination
}

func reportPageHref(filters ReportFilters, page int) string {
	values := reportFilterValues(filters)
	values.Set("page", strconv.Itoa(page))
	return "/reports?" + values.Encode()
}

func reportFilterValues(filters ReportFilters) url.Values {
	values := url.Values{}
	setIf := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	setIf("q", filters.Search)
	setIf("role", filters.Role)
	setIf("profile", filters.Profile)
	setIf("kind", filters.Kind)
	setIf("owning_domain", filters.OwningDomain)
	setIf("risk_level", filters.RiskLevel)
	for _, tag := range filters.Tags {
		values.Add("tag", tag)
	}
	if filters.IncludeBookkeeping {
		values.Set("include_bookkeeping", "1")
	}
	if filters.TableLimit > 0 && filters.TableLimit != defaultReportTableLimit {
		values.Set("table_limit", strconv.Itoa(filters.TableLimit))
	}
	return values
}

func reportExportBase(query url.Values) string {
	values := url.Values{}
	for key, list := range query {
		if key == "format" {
			continue
		}
		for _, value := range list {
			values.Add(key, value)
		}
	}
	if encoded := values.Encode(); encoded != "" {
		return "/reports?" + encoded + "&format="
	}
	return "/reports?format="
}

func writeReportJSON(w http.ResponseWriter, data ReportViewData) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func writeReportCSV(w http.ResponseWriter, data ReportViewData) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="fairway-report.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "title", "role", "status", "kind", "profile", "domain", "risk", "tags", "class", "completed", "moving", "created", "evidence", "reviews", "missing_reviews", "latest_evidence", "last_activity"})
	for _, row := range data.Rows {
		_ = writer.Write([]string{row.ID, row.Title, row.Role, row.Status, row.Kind, row.Profile, row.OwningDomain, row.RiskLevel, strings.Join(row.Tags, ";"), row.DeliveryClass, strconv.FormatBool(row.Completed), strconv.FormatBool(row.Moving), strconv.FormatBool(row.Created), strconv.Itoa(row.EvidenceCount), strconv.Itoa(row.ReviewCount), strings.Join(row.MissingReviews, ";"), row.LatestEvidence, row.LastActivityAt})
	}
	writer.Flush()
}

func writeReportMarkdown(w http.ResponseWriter, data ReportViewData) {
	w.Header().Set("Content-Type", "text/markdown")
	fmt.Fprintf(w, "# Fairway Report: %s to %s\n\n", data.Window.StartDate, data.Window.EndDate)
	fmt.Fprintf(w, "- Delivery done: %d\n- Monitor/deploy bookkeeping done: %d\n- Moving tasks: %d\n- Evidence rows: %d\n- Reviews recorded: %d\n- Follow-ups created: %d\n\n", data.Summary.DeliveryDone, data.Summary.BookkeepingDone, data.Summary.MovingTasks, data.Summary.EvidenceRows, data.Summary.ReviewsRecorded, data.Summary.FollowUpsCreated)
	fmt.Fprintln(w, "## Lane Outcomes")
	for _, lane := range data.Lanes {
		fmt.Fprintf(w, "- %s: %d completed, %d moving\n", lane.Role, lane.Completed, lane.Moving)
	}
	fmt.Fprintln(w, "\n## Tasks")
	for _, row := range data.Rows {
		fmt.Fprintf(w, "- %s %s [%s/%s]\n", row.ID, row.Title, row.Role, row.Status)
	}
}

func timeInWindow(raw string, start, end time.Time) bool {
	if raw == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return raw >= start.Format(time.RFC3339Nano) && raw < end.Format(time.RFC3339Nano)
	}
	local := parsed.In(start.Location())
	return !local.Before(start) && local.Before(end)
}

func maxTimeString(left, right string) string {
	if right == "" {
		return left
	}
	if left == "" || right > left {
		return right
	}
	return left
}

func latestEvidenceArtifact(evidence []store.Evidence) string {
	latestAt := ""
	artifact := ""
	for _, ev := range evidence {
		if ev.CreatedAt >= latestAt {
			latestAt = ev.CreatedAt
			artifact = firstNonEmpty(ev.ArtifactPath, ev.ArtifactType, ev.CommandText)
		}
	}
	return artifact
}

func evidenceLooksRun(ev store.Evidence) bool {
	text := strings.ToLower(strings.Join([]string{ev.ArtifactType, ev.ArtifactPath, ev.CommandText, ev.Notes}, " "))
	for _, keyword := range []string{"ci", "pipeline", "deploy", "uat", "smoke", "release"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func evidenceFailed(result string) bool {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "fail", "failed", "blocked", "partial", "warning":
		return true
	default:
		return false
	}
}

func evidenceRunResult(result string) string {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "pass", "passed", "ok", "success":
		return "pass"
	case "fail", "failed", "blocked", "error":
		return "fail"
	default:
		return "running"
	}
}

func followUpTaxonomy(taskID string) string {
	for _, prefix := range []string{"CI-FIX", "CD-FIX", "UAT-BUG", "OPS-FIX", "HARNESS-FIX", "DOC-FIX"} {
		if strings.HasPrefix(taskID, prefix) {
			return prefix
		}
	}
	return ""
}

func followUpsBySource(rows []ReportTaskRow) map[string][]string {
	out := map[string][]string{}
	for _, row := range rows {
		if row.FollowUpTaxonomy == "" {
			continue
		}
		out[""] = append(out[""], row.ID)
	}
	return out
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
