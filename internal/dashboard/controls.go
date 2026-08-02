package dashboard

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/controlanalytics"
	"github.com/subashram/fairway/internal/store"
)

const defaultControlWindowDays = 30

type ControlViewData struct {
	View          string                  `json:"-"`
	Filters       ControlFilters          `json:"filters"`
	FilterOptions ControlFilterOptions    `json:"filter_options"`
	Report        controlanalytics.Report `json:"report"`
	Summary       ControlDashboardSummary `json:"summary"`
	Rows          []ControlDashboardRow   `json:"rows"`
	Sessions      []store.Session         `json:"-"`
	Activity      []store.Activity        `json:"-"`
	Groups        []RoleGroup             `json:"-"`
	TaskRoles     map[string]string       `json:"-"`
	ReadOnly      bool                    `json:"-"`
}

type ControlFilters struct {
	WindowDays int    `json:"window_days"`
	Profile    string `json:"profile,omitempty"`
	RiskBand   string `json:"risk_band,omitempty"`
	SizeBand   string `json:"size_band,omitempty"`
	Family     string `json:"family,omitempty"`
	Control    string `json:"control,omitempty"`
	Horizon    int    `json:"horizon_days"`
}

type ControlFilterOptions struct {
	Windows  []int    `json:"windows"`
	Profiles []string `json:"profiles"`
	Risks    []string `json:"risks"`
	Sizes    []string `json:"sizes"`
	Families []string `json:"families"`
	Controls []string `json:"controls"`
	Horizons []int    `json:"horizons"`
}

type ControlDashboardSummary struct {
	Controls            int            `json:"controls"`
	Cohorts             int            `json:"cohorts"`
	Applicable          int            `json:"applicable"`
	Eligible            int            `json:"eligible"`
	UnknownControlState int            `json:"unknown_control_state"`
	RightCensored       int            `json:"right_censored"`
	OutcomeUnavailable  int            `json:"outcome_unavailable"`
	Classifications     map[string]int `json:"classifications"`
}

type ControlDashboardRow struct {
	Result controlanalytics.ControlResult `json:"result"`
	Facts  []controlanalytics.TaskFact    `json:"facts"`
}

func (s *Server) controls(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	timing := newDashboardTiming("controls", r)
	defer timing.logIfSlow()
	data, err := s.controlViewData(r, timing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	if err := controlsTemplate.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	timing.add("template.controls", time.Since(start), "")
}

func (s *Server) controlViewData(r *http.Request, timing *dashboardTiming) (ControlViewData, error) {
	if dashboardSnapshotCacheable(r) {
		key := dashboardSnapshotKey("controls", r)
		data, status, err := dashboardSnapshotGet(s.snapshots, key, func() (ControlViewData, error) {
			return s.buildControlViewData(r, timing)
		})
		timing.add("controls.snapshot_cache", 0, "status="+status)
		return data, err
	}
	return s.buildControlViewData(r, timing)
}

func (s *Server) buildControlViewData(r *http.Request, timing *dashboardTiming) (ControlViewData, error) {
	filters := controlFiltersFromRequest(r)
	var report controlanalytics.Report
	if err := timing.step("controls.report", func() error {
		var err error
		report, err = controlanalytics.Build(r.Context(), s.cfg, s.root, s.store, controlanalytics.Options{
			Since:   time.Duration(filters.WindowDays) * 24 * time.Hour,
			Profile: filters.Profile,
			Now:     s.now().UTC(),
		})
		return err
	}); err != nil {
		return ControlViewData{}, err
	}
	data := buildControlViewData(report, filters)
	var profiles []string
	data.Sessions, data.Activity, data.Groups, data.TaskRoles, profiles = s.controlHeaderContext(r)
	data.FilterOptions.Profiles = profiles
	data.ReadOnly = s.cfg.Dashboard.ReadOnly
	return data, nil
}

func (s *Server) controlHeaderContext(r *http.Request) ([]store.Session, []store.Activity, []RoleGroup, map[string]string, []string) {
	sessions, _ := s.store.Sessions(r.Context(), false)
	activity, _ := s.store.Activity(r.Context(), defaultActivityLimit)
	tasks, _ := s.store.AllTasks(r.Context())
	groups := groupTasks(tasks, s.roles)
	profileSet := map[string]struct{}{}
	for _, task := range tasks {
		addControlOption(profileSet, task.Definition.Profile)
	}
	return sessions, activity, groups, taskRoleMap(tasks), sortedControlOptions(profileSet)
}

func controlFiltersFromRequest(r *http.Request) ControlFilters {
	query := r.URL.Query()
	window := boundedControlChoice(query.Get("window"), []int{7, 14, 30, 90}, defaultControlWindowDays)
	horizon := boundedControlChoice(query.Get("horizon"), []int{7, 14, 30}, 14)
	return ControlFilters{
		WindowDays: window,
		Profile:    strings.TrimSpace(query.Get("profile")),
		RiskBand:   strings.TrimSpace(query.Get("risk")),
		SizeBand:   strings.TrimSpace(query.Get("size")),
		Family:     strings.TrimSpace(query.Get("family")),
		Control:    strings.TrimSpace(query.Get("control")),
		Horizon:    horizon,
	}
}

func boundedControlChoice(raw string, allowed []int, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

func buildControlViewData(report controlanalytics.Report, filters ControlFilters) ControlViewData {
	options := controlFilterOptions(report.Controls)
	factsByKey := map[string][]controlanalytics.TaskFact{}
	for _, fact := range report.TaskFacts {
		factsByKey[controlCohortKey(fact.ControlID, fact.Profile, fact.RiskBand, fact.SizeBand, fact.HorizonDays)] = append(
			factsByKey[controlCohortKey(fact.ControlID, fact.Profile, fact.RiskBand, fact.SizeBand, fact.HorizonDays)], fact,
		)
	}
	rows := make([]ControlDashboardRow, 0, len(report.Controls))
	controls := map[string]struct{}{}
	summary := ControlDashboardSummary{Classifications: map[string]int{}}
	for _, result := range report.Controls {
		if filters.Profile != "" && result.Profile != filters.Profile ||
			filters.RiskBand != "" && result.RiskBand != filters.RiskBand ||
			filters.SizeBand != "" && result.SizeBand != filters.SizeBand ||
			filters.Family != "" && result.Family != filters.Family ||
			filters.Control != "" && result.ControlID != filters.Control ||
			filters.Horizon > 0 && result.HorizonDays != filters.Horizon {
			continue
		}
		facts := append([]controlanalytics.TaskFact(nil), factsByKey[controlCohortKey(result.ControlID, result.Profile, result.RiskBand, result.SizeBand, result.HorizonDays)]...)
		sort.Slice(facts, func(i, j int) bool { return facts[i].TaskID < facts[j].TaskID })
		rows = append(rows, ControlDashboardRow{Result: result, Facts: facts})
		controls[result.ControlID] = struct{}{}
		summary.Cohorts++
		summary.Applicable += result.Applicable
		summary.Eligible += result.Eligible
		summary.UnknownControlState += result.UnknownControlState
		summary.RightCensored += result.RightCensored
		summary.OutcomeUnavailable += result.OutcomeUnavailable
		summary.Classifications[result.Classification]++
	}
	summary.Controls = len(controls)
	return ControlViewData{
		View:          "controls",
		Filters:       filters,
		FilterOptions: options,
		Report:        report,
		Summary:       summary,
		Rows:          rows,
	}
}

func controlCohortKey(control, profile, risk, size string, horizon int) string {
	return strings.Join([]string{control, profile, risk, size, strconv.Itoa(horizon)}, "\x00")
}

func controlFilterOptions(rows []controlanalytics.ControlResult) ControlFilterOptions {
	profiles, risks, sizes, families, controls := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, row := range rows {
		addControlOption(profiles, row.Profile)
		addControlOption(risks, row.RiskBand)
		addControlOption(sizes, row.SizeBand)
		addControlOption(families, row.Family)
		addControlOption(controls, row.ControlID)
	}
	return ControlFilterOptions{
		Windows: []int{7, 14, 30, 90}, Horizons: []int{7, 14, 30},
		Profiles: sortedControlOptions(profiles), Risks: sortedControlOptions(risks), Sizes: sortedControlOptions(sizes),
		Families: sortedControlOptions(families), Controls: sortedControlOptions(controls),
	}
}

func addControlOption(values map[string]struct{}, value string) {
	if value = strings.TrimSpace(value); value != "" {
		values[value] = struct{}{}
	}
}

func sortedControlOptions(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
