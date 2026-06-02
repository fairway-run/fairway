package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/state"
	"github.com/subashram/fairway/internal/store"
)

type Server struct {
	store     *store.Store
	cfg       config.Config
	roles     []string
	worktrees []WorktreeStatus
	csrfToken string
}

type WorktreeStatus struct {
	Role       string
	Branch     string
	Path       string
	Registered bool
	Exists     bool
	Dirty      bool
	LastCommit string
}

type Rollup struct {
	Done  int
	Total int
}

type ProjectStore struct {
	Name  string
	Path  string
	Store *store.Store
}

func New(s *store.Store, cfg config.Config, roles []string, worktrees []WorktreeStatus) *Server {
	return &Server{store: s, cfg: cfg, roles: roles, worktrees: worktrees, csrfToken: newCSRFToken()}
}

func NewMulti(projects []ProjectStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		type projectView struct {
			Name            string
			Path            string
			Tasks           []store.Task
			TaskCount       int
			SessionCount    int
			CheckpointCount int
			WatcherCount    int
			Error           string
		}
		var views []projectView
		for _, project := range projects {
			view := projectView{Name: project.Name, Path: project.Path}
			tasks, err := project.Store.AllTasks(r.Context())
			if err != nil {
				view.Error = err.Error()
				views = append(views, view)
				continue
			}
			sessions, _ := project.Store.Sessions(r.Context(), false)
			checkpoints, _ := project.Store.Checkpoints(r.Context(), "", false)
			watchers, _ := project.Store.Watchers(r.Context(), false)
			view.Tasks = tasks
			view.TaskCount = len(tasks)
			view.SessionCount = len(sessions)
			view.CheckpointCount = len(checkpoints)
			view.WatcherCount = len(watchers)
			views = append(views, view)
		}
		_ = multiTemplate.Execute(w, struct{ Projects []projectView }{views})
	})
	return mux
}

type RoleGroup struct {
	Role    string
	Current *store.Task
	Tasks   []store.Task
}

type WorkstreamGroup struct {
	Label      string
	Tasks      []store.Task
	Total      int
	Done       int
	InProgress int
	Blocked    int
	Ready      int
}

type DashboardSummary struct {
	Total       int
	Ready       int
	InProgress  int
	Blocked     int
	Done        int
	Filtered    int
	Profiles    int
	Workstreams int
}

type GateStatus struct {
	Profile        string
	Name           string
	Group          string
	Mode           string
	EvidenceType   string
	Description    string
	TaskCount      int
	SatisfiedCount int
	MissingCount   int
	Status         string
	Missing        []GateTaskMiss
}

type GateGroup struct {
	Profile          string
	Name             string
	Label            string
	Status           string
	GateCount        int
	TaskCount        int
	SatisfiedCount   int
	MissingTaskCount int
	BlockingMissing  int
	AdvisoryMissing  int
	ReportOnlyMisses int
	NoTaskCount      int
	Gates            []GateStatus
}

type GateTaskMiss struct {
	TaskID   string
	Title    string
	Kind     string
	Status   string
	Reasons  []string
	Matching int
}

type TaskFilters struct {
	Search        string
	Status        string
	Profile       string
	Kind          string
	OwningDomain  string
	RiskLevel     string
	ReviewDomain  string
	ActivityKind  string
	ActivityLimit int
	TableLimit    int
}

type FilterOptions struct {
	Statuses      []string
	Profiles      []string
	Kinds         []string
	OwningDomains []string
	RiskLevels    []string
	ReviewDomains []string
	ActivityKinds []string
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/tasks/", s.task)
	mux.HandleFunc("/actions/claim", s.claim)
	mux.HandleFunc("/actions/set-status", s.setStatus)
	mux.HandleFunc("/events", s.events)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.AllTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	health, err := s.store.Health(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sessions, err := s.store.Sessions(r.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	checkpoints, err := s.store.Checkpoints(r.Context(), "", false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	staleCheckpoints, err := s.store.Checkpoints(r.Context(), time.Now().UTC().Format("2006-01-02"), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	watchers, err := s.store.Watchers(r.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filters := taskFiltersFromRequest(r)
	activity, err := s.store.Activity(r.Context(), maxActivityFetchLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filteredActivity, activityTotal := filterActivity(activity, filters.ActivityKind, filters.ActivityLimit)
	gates, err := s.dashboardGateStatuses(r.Context(), tasks, 8)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	readyTasks, err := s.store.Ready(r.Context(), "", s.cfg.States.Terminal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	readySet := taskIDSet(readyTasks)
	gateGroups := groupGateStatuses(gates)
	displayTasks := filterTasks(tasks, filters)
	rollups := taskRollups(tasks, map[string]bool{"done": true})
	workstreams := groupWorkstreams(displayTasks, readySet)
	data := struct {
		Summary          DashboardSummary
		Gates            []GateStatus
		GateGroups       []GateGroup
		Groups           []RoleGroup
		Workstreams      []WorkstreamGroup
		Filters          TaskFilters
		FilterOptions    FilterOptions
		Activity         []store.Activity
		ActivityTotal    int
		Health           store.Health
		Sessions         []store.Session
		Worktrees        []WorktreeStatus
		Checkpoints      []store.Checkpoint
		StaleCheckpoints []store.Checkpoint
		Watchers         []store.Watcher
		Rollups          map[string]Rollup
	}{dashboardSummary(tasks, displayTasks, workstreams, readySet), gates, gateGroups, groupTasks(displayTasks, s.roles), workstreams, filters, filterOptions(tasks, activity), filteredActivity, activityTotal, health, sessions, s.worktrees, checkpoints, staleCheckpoints, watchers, rollups}
	_ = indexTemplate.Execute(w, data)
}

const (
	defaultActivityLimit  = 25
	defaultTableLimit     = 25
	maxActivityFetchLimit = 500
	maxActivityLimit      = 200
	maxTableLimit         = 200
)

func groupTasks(tasks []store.Task, roles []string) []RoleGroup {
	byRole := map[string]int{}
	var groups []RoleGroup
	addRole := func(role string) int {
		if role == "" {
			role = "unassigned"
		}
		if index, ok := byRole[role]; ok {
			return index
		}
		groups = append(groups, RoleGroup{Role: role})
		index := len(groups) - 1
		byRole[role] = index
		return index
	}
	for _, role := range roles {
		addRole(role)
	}
	for _, task := range tasks {
		index := addRole(task.Definition.Role)
		groups[index].Tasks = append(groups[index].Tasks, task)
		if task.Status == "in_progress" && groups[index].Current == nil {
			copy := task
			groups[index].Current = &copy
		}
	}
	return groups
}

func taskFiltersFromRequest(r *http.Request) TaskFilters {
	query := r.URL.Query()
	return TaskFilters{
		Search:        strings.TrimSpace(query.Get("q")),
		Status:        strings.TrimSpace(query.Get("status")),
		Profile:       strings.TrimSpace(query.Get("profile")),
		Kind:          strings.TrimSpace(query.Get("kind")),
		OwningDomain:  strings.TrimSpace(query.Get("owning_domain")),
		RiskLevel:     strings.TrimSpace(query.Get("risk_level")),
		ReviewDomain:  strings.TrimSpace(query.Get("review_domain")),
		ActivityKind:  strings.TrimSpace(query.Get("activity_kind")),
		ActivityLimit: boundedQueryInt(query.Get("activity_limit"), defaultActivityLimit, maxActivityLimit),
		TableLimit:    boundedQueryInt(query.Get("table_limit"), defaultTableLimit, maxTableLimit),
	}
}

func boundedQueryInt(raw string, fallback, max int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func filterTasks(tasks []store.Task, filters TaskFilters) []store.Task {
	var out []store.Task
	for _, task := range tasks {
		if filters.Search != "" && !taskMatchesSearch(task, filters.Search) {
			continue
		}
		if filters.Status != "" && task.Status != filters.Status {
			continue
		}
		if filters.Profile != "" && task.Definition.Profile != filters.Profile {
			continue
		}
		if filters.Kind != "" && task.Definition.Kind != filters.Kind {
			continue
		}
		if filters.OwningDomain != "" && task.Definition.OwningDomain != filters.OwningDomain {
			continue
		}
		if filters.RiskLevel != "" && task.Definition.RiskLevel != filters.RiskLevel {
			continue
		}
		if filters.ReviewDomain != "" && !containsString(task.Definition.ReviewDomains, filters.ReviewDomain) {
			continue
		}
		out = append(out, task)
	}
	return out
}

func taskMatchesSearch(task store.Task, raw string) bool {
	needle := strings.ToLower(strings.TrimSpace(raw))
	if needle == "" {
		return true
	}
	haystacks := []string{
		task.Definition.ID,
		task.Definition.Title,
		task.Definition.Kind,
		task.Definition.Profile,
		task.Definition.OwningDomain,
		task.Definition.OwningLayer,
		task.Definition.RiskLevel,
		task.Definition.MigrationType,
		task.Status,
		task.Owner,
	}
	haystacks = append(haystacks, task.Definition.SourcePaths...)
	haystacks = append(haystacks, task.Definition.TargetPaths...)
	haystacks = append(haystacks, task.Definition.ReviewDomains...)
	for _, value := range haystacks {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func filterOptions(tasks []store.Task, activity []store.Activity) FilterOptions {
	var options FilterOptions
	statuses := map[string]bool{}
	profiles := map[string]bool{}
	kinds := map[string]bool{}
	domains := map[string]bool{}
	risks := map[string]bool{}
	reviewDomains := map[string]bool{}
	activityKinds := map[string]bool{}
	for _, task := range tasks {
		addFilterValue(statuses, task.Status)
		addFilterValue(profiles, task.Definition.Profile)
		addFilterValue(kinds, task.Definition.Kind)
		addFilterValue(domains, task.Definition.OwningDomain)
		addFilterValue(risks, task.Definition.RiskLevel)
		for _, domain := range task.Definition.ReviewDomains {
			addFilterValue(reviewDomains, domain)
		}
	}
	for _, item := range activity {
		addFilterValue(activityKinds, item.Kind)
	}
	options.Statuses = sortedFilterValues(statuses)
	options.Profiles = sortedFilterValues(profiles)
	options.Kinds = sortedFilterValues(kinds)
	options.OwningDomains = sortedFilterValues(domains)
	options.RiskLevels = sortedFilterValues(risks)
	options.ReviewDomains = sortedFilterValues(reviewDomains)
	options.ActivityKinds = sortedFilterValues(activityKinds)
	return options
}

func addFilterValue(values map[string]bool, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values[value] = true
	}
}

func sortedFilterValues(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func groupWorkstreams(tasks []store.Task, readySet map[string]bool) []WorkstreamGroup {
	byLabel := map[string]int{}
	var groups []WorkstreamGroup
	for _, task := range tasks {
		label := workstreamLabel(task)
		if label == "" {
			continue
		}
		index, ok := byLabel[label]
		if !ok {
			groups = append(groups, WorkstreamGroup{Label: label})
			index = len(groups) - 1
			byLabel[label] = index
		}
		groups[index].Tasks = append(groups[index].Tasks, task)
		groups[index].Total++
		switch task.Status {
		case "done":
			groups[index].Done++
		case "in_progress":
			groups[index].InProgress++
		case "blocked":
			groups[index].Blocked++
		case "todo":
			if !readySet[task.Definition.ID] {
				continue
			}
			groups[index].Ready++
		}
	}
	return groups
}

func dashboardSummary(allTasks, displayTasks []store.Task, workstreams []WorkstreamGroup, readySet map[string]bool) DashboardSummary {
	summary := DashboardSummary{Total: len(allTasks), Filtered: len(displayTasks), Workstreams: len(workstreams)}
	profiles := map[string]bool{}
	for _, task := range allTasks {
		if task.Definition.Profile != "" {
			profiles[task.Definition.Profile] = true
		}
		switch task.Status {
		case "done":
			summary.Done++
		case "in_progress":
			summary.InProgress++
		case "blocked":
			summary.Blocked++
		case "todo":
			if !readySet[task.Definition.ID] {
				continue
			}
			summary.Ready++
		}
	}
	summary.Profiles = len(profiles)
	return summary
}

func taskIDSet(tasks []store.Task) map[string]bool {
	out := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		out[task.Definition.ID] = true
	}
	return out
}

func workstreamLabel(task store.Task) string {
	profile := task.Definition.Profile
	kind := task.Definition.Kind
	switch {
	case profile != "" && kind != "":
		return profile + " / " + kind
	case profile != "":
		return profile
	case kind != "" && kind != "task":
		return kind
	default:
		return ""
	}
}

func filterActivity(activity []store.Activity, kind string, limit int) ([]store.Activity, int) {
	var filtered []store.Activity
	for _, item := range activity {
		if kind != "" && item.Kind != kind {
			continue
		}
		filtered = append(filtered, item)
	}
	total := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, total
}

func (s *Server) dashboardGateStatuses(ctx context.Context, tasks []store.Task, gapLimit int) ([]GateStatus, error) {
	var statuses []GateStatus
	now := time.Now().UTC()
	for _, profile := range s.cfg.WorkstreamProfiles {
		if len(profile.Gates) == 0 {
			continue
		}
		for _, gate := range profile.Gates {
			profileTasks := tasksForProfileGate(profile, gate, tasks)
			status := GateStatus{
				Profile:      profile.Name,
				Name:         gate.Name,
				Mode:         gate.Mode,
				EvidenceType: gate.EvidenceType,
				Description:  gate.Description,
				TaskCount:    len(profileTasks),
				Status:       "satisfied",
			}
			status.Group = gateGroupLabel(profile, gate)
			if len(profileTasks) == 0 {
				status.Status = "no_tasks"
				statuses = append(statuses, status)
				continue
			}
			for _, task := range profileTasks {
				_, _, evidence, _, _, err := s.store.TaskDetail(ctx, task.Definition.ID)
				if err != nil {
					return nil, err
				}
				ok, matching, reasons := evaluateGateForTask(gate, evidence, now)
				if ok {
					status.SatisfiedCount++
					continue
				}
				status.MissingCount++
				if gapLimit < 0 || len(status.Missing) < gapLimit {
					status.Missing = append(status.Missing, GateTaskMiss{
						TaskID:   task.Definition.ID,
						Title:    task.Definition.Title,
						Kind:     task.Definition.Kind,
						Status:   task.Status,
						Reasons:  reasons,
						Matching: matching,
					})
				}
			}
			if status.MissingCount > 0 {
				status.Status = "missing"
			}
			statuses = append(statuses, status)
		}
	}
	sortGateStatuses(statuses)
	return statuses, nil
}

func gateGroupLabel(profile config.WorkstreamProfile, gate config.WorkstreamProfileGate) string {
	if strings.TrimSpace(gate.Group) != "" {
		return strings.TrimSpace(gate.Group)
	}
	if len(gate.TaskKinds) > 0 {
		return strings.Join(gate.TaskKinds, ", ")
	}
	if gate.EvidenceType != "" {
		return gate.EvidenceType
	}
	return "general"
}

func groupGateStatuses(gates []GateStatus) []GateGroup {
	byKey := map[string]int{}
	var groups []GateGroup
	for _, gate := range gates {
		groupName := gate.Group
		if groupName == "" {
			groupName = "general"
		}
		key := gate.Profile + "\x00" + groupName
		index, ok := byKey[key]
		if !ok {
			groups = append(groups, GateGroup{
				Profile: gate.Profile,
				Name:    groupName,
				Label:   gate.Profile + " / " + groupName,
				Status:  "satisfied",
			})
			index = len(groups) - 1
			byKey[key] = index
		}
		group := groups[index]
		group.GateCount++
		group.TaskCount += gate.TaskCount
		group.SatisfiedCount += gate.SatisfiedCount
		group.MissingTaskCount += gate.MissingCount
		group.Gates = append(group.Gates, gate)
		switch gate.Status {
		case "missing":
			group.Status = "missing"
			switch gate.Mode {
			case "blocking":
				group.BlockingMissing += gate.MissingCount
			case "advisory":
				group.AdvisoryMissing += gate.MissingCount
			case "report_only":
				group.ReportOnlyMisses += gate.MissingCount
			}
		case "no_tasks":
			group.NoTaskCount++
			if group.Status == "satisfied" {
				group.Status = "no_tasks"
			}
		}
		groups[index] = group
	}
	sort.Slice(groups, func(i, j int) bool {
		leftRank := gateGroupStatusRank(groups[i])
		rightRank := gateGroupStatusRank(groups[j])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if groups[i].BlockingMissing != groups[j].BlockingMissing {
			return groups[i].BlockingMissing > groups[j].BlockingMissing
		}
		if groups[i].MissingTaskCount != groups[j].MissingTaskCount {
			return groups[i].MissingTaskCount > groups[j].MissingTaskCount
		}
		return groups[i].Label < groups[j].Label
	})
	return groups
}

func gateGroupStatusRank(group GateGroup) int {
	if group.BlockingMissing > 0 {
		return 0
	}
	if group.MissingTaskCount > 0 {
		return 1
	}
	if group.Status == "no_tasks" {
		return 2
	}
	return 3
}

func sortGateStatuses(statuses []GateStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		leftRank := gateStatusRank(statuses[i])
		rightRank := gateStatusRank(statuses[j])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if statuses[i].MissingCount != statuses[j].MissingCount {
			return statuses[i].MissingCount > statuses[j].MissingCount
		}
		leftLabel := statuses[i].Profile + "/" + statuses[i].Group + "/" + statuses[i].Name
		rightLabel := statuses[j].Profile + "/" + statuses[j].Group + "/" + statuses[j].Name
		return leftLabel < rightLabel
	})
}

func gateStatusRank(status GateStatus) int {
	if status.Status == "missing" && status.Mode == "blocking" {
		return 0
	}
	if status.Status == "missing" {
		return 1
	}
	if status.Status == "no_tasks" {
		return 2
	}
	return 3
}

func tasksForProfile(profile config.WorkstreamProfile, tasks []store.Task) []store.Task {
	if len(profile.TaskKinds) == 0 {
		return tasks
	}
	var out []store.Task
	for _, task := range tasks {
		if profileAppliesToTask(profile, task) {
			out = append(out, task)
		}
	}
	return out
}

func tasksForProfileGate(profile config.WorkstreamProfile, gate config.WorkstreamProfileGate, tasks []store.Task) []store.Task {
	var out []store.Task
	for _, task := range tasksForProfile(profile, tasks) {
		if gateAppliesToTask(gate, task) {
			out = append(out, task)
		}
	}
	return out
}

func profileAppliesToTask(profile config.WorkstreamProfile, task store.Task) bool {
	if len(profile.TaskKinds) == 0 {
		return true
	}
	for _, kind := range profile.TaskKinds {
		if task.Definition.Kind == kind {
			return true
		}
	}
	return false
}

func gateAppliesToTask(gate config.WorkstreamProfileGate, task store.Task) bool {
	if len(gate.TaskKinds) == 0 {
		return true
	}
	for _, kind := range gate.TaskKinds {
		if task.Definition.Kind == kind {
			return true
		}
	}
	return false
}

func evaluateGateForTask(gate config.WorkstreamProfileGate, evidence []store.Evidence, now time.Time) (bool, int, []string) {
	requiredCount := gate.RequiredEvidenceCount
	if requiredCount == 0 && (gate.EvidenceType != "" || len(gate.AcceptedResults) > 0 || gate.ArtifactRequired || gate.ExpiresAfter != "" || gate.OwnerSignoffRequired) {
		requiredCount = 1
	}
	accepted := map[string]bool{}
	for _, result := range gate.AcceptedResults {
		accepted[result] = true
	}
	var matching int
	for _, ev := range evidence {
		if gate.EvidenceType != "" && ev.ArtifactType != gate.EvidenceType {
			continue
		}
		if len(accepted) > 0 && !accepted[ev.Result] {
			continue
		}
		if gate.ArtifactRequired && ev.ArtifactPath == "" {
			continue
		}
		if gate.ExpiresAfter != "" && !evidenceIsFresh(ev, gate.ExpiresAfter, now) {
			continue
		}
		if gate.OwnerSignoffRequired && !evidenceHasOwnerSignoff(ev) {
			continue
		}
		matching++
	}
	var reasons []string
	if matching < requiredCount {
		reasons = append(reasons, fmt.Sprintf("needs %d matching evidence row(s), found %d", requiredCount, matching))
		if gate.ArtifactRequired {
			reasons = append(reasons, "matching rows must include evidence artifacts")
		}
		if gate.ExpiresAfter != "" {
			reasons = append(reasons, "matching rows must be fresh")
		}
		if gate.OwnerSignoffRequired {
			reasons = append(reasons, "matching rows must include owner signoff evidence notes")
		}
	}
	return len(reasons) == 0, matching, reasons
}

func evidenceHasOwnerSignoff(ev store.Evidence) bool {
	notes := strings.ToLower(ev.Notes)
	return strings.Contains(notes, "signoff") || strings.Contains(notes, "sign-off")
}

func evidenceIsFresh(ev store.Evidence, expiresAfter string, now time.Time) bool {
	if expiresAfter == "" {
		return true
	}
	ttl, err := time.ParseDuration(expiresAfter)
	if err != nil {
		return false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ev.CreatedAt)
	if err != nil {
		return false
	}
	return now.Sub(createdAt) <= ttl
}

func (s *Server) task(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/tasks/"):]
	task, transitions, evidence, handoffs, reviews, err := s.store.TaskDetail(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	tasks, err := s.store.AllTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rollups := taskRollups(tasks, map[string]bool{"done": true})
	data := struct {
		Task        store.Task
		Transitions []store.Transition
		Evidence    []store.Evidence
		Handoffs    []store.Handoff
		Reviews     []store.Review
		Rollup      Rollup
		CSRFToken   string
		States      []string
	}{task, transitions, evidence, handoffs, reviews, rollups[task.Definition.ID], s.csrfToken, dashboardMutableStates(s.cfg)}
	_ = detailTemplate.Execute(w, data)
}

func (s *Server) claim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.FormValue("csrf") != s.csrfToken {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	taskID := r.FormValue("task_id")
	task, _, _, _, _, err := s.store.TaskDetail(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := s.store.Claim(r.Context(), taskID, task.Definition.Role, ""); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = s.store.RecordAudit(r.Context(), store.AuditEvent{Action: "dashboard.claim", TaskID: taskID, Detail: "claimed from dashboard"})
	http.Redirect(w, r, "/tasks/"+taskID, http.StatusSeeOther)
}

func (s *Server) setStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.FormValue("csrf") != s.csrfToken {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	taskID := r.FormValue("task_id")
	target := strings.TrimSpace(r.FormValue("status"))
	reason := strings.TrimSpace(r.FormValue("reason"))
	task, _, _, _, _, err := s.store.TaskDetail(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	stateCfg := state.Config{Allowed: s.cfg.States.Allowed, Terminal: s.cfg.States.Terminal, Transitions: s.cfg.States.Transitions}
	if state.IsTerminal(stateCfg, target) {
		http.Error(w, "terminal status changes use CLI gates", http.StatusBadRequest)
		return
	}
	if err := state.ValidateTransition(stateCfg, task.Status, target, false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.SetStatus(r.Context(), taskID, target, reason, s.cfg.Gates.RequireBlockedReason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = s.store.RecordAudit(r.Context(), store.AuditEvent{Action: "dashboard.set-status", TaskID: taskID, Detail: "status=" + target})
	http.Redirect(w, r, "/tasks/"+taskID, http.StatusSeeOther)
}

func dashboardMutableStates(cfg config.Config) []string {
	stateCfg := state.Config{Allowed: cfg.States.Allowed, Terminal: cfg.States.Terminal, Transitions: cfg.States.Transitions}
	var out []string
	for _, status := range cfg.States.Allowed {
		if !state.IsTerminal(stateCfg, status) {
			out = append(out, status)
		}
	}
	return out
}

func taskRollups(tasks []store.Task, terminal map[string]bool) map[string]Rollup {
	parent := map[string]string{}
	status := map[string]string{}
	for _, task := range tasks {
		parent[task.Definition.ID] = task.Definition.ParentID
		status[task.Definition.ID] = task.Status
	}
	rollups := map[string]Rollup{}
	for _, task := range tasks {
		for cursor := parent[task.Definition.ID]; cursor != ""; cursor = parent[cursor] {
			rollup := rollups[cursor]
			rollup.Total++
			if terminal[status[task.Definition.ID]] {
				rollup.Done++
			}
			rollups[cursor] = rollup
		}
	}
	return rollups
}

func newCSRFToken() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes[:])
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	last, err := s.store.LatestHistoryID(r.Context())
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
		flusher.Flush()
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			current, err := s.store.LatestHistoryID(r.Context())
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
				flusher.Flush()
				return
			}
			if current > last {
				last = current
				fmt.Fprintf(w, "event: refresh\ndata: %d\n\n", current)
				flusher.Flush()
			}
		}
	}
}

var indexTemplate = template.Must(template.New("index").Funcs(template.FuncMap{
	"percent": func(done, total int) float64 {
		if total == 0 {
			return 0
		}
		return float64(done) / float64(total) * 100
	},
	"takeTasks": func(tasks []store.Task, limit int) []store.Task {
		if limit <= 0 || len(tasks) <= limit {
			return tasks
		}
		return tasks[:limit]
	},
	"moreTasks": func(tasks []store.Task, limit int) int {
		if limit <= 0 || len(tasks) <= limit {
			return 0
		}
		return len(tasks) - limit
	},
}).Parse(`<!doctype html>
<html><head><title>fairway</title><style>
:root{--bg:#f6f7f9;--panel:#fff;--line:#d9dee7;--text:#182230;--muted:#667085;--good:#027a48;--bad:#b42318;--warn:#b54708;--accent:#245b6b}
body{font-family:Inter,system-ui,sans-serif;margin:0;background:var(--bg);color:var(--text)}
a{color:#175cd3;text-decoration:none}a:hover{text-decoration:underline}
.shell{max-width:1440px;margin:0 auto;padding:24px}
.topbar{display:flex;align-items:flex-start;justify-content:space-between;gap:24px;margin-bottom:18px}.eyebrow{font-size:12px;text-transform:uppercase;letter-spacing:.08em;color:var(--muted);font-weight:700}.topbar h1{margin:.15rem 0;font-size:34px}.topbar p{margin:.25rem 0;color:var(--muted)}
table{border-collapse:collapse;width:100%;background:var(--panel);margin-bottom:24px}td,th{border-bottom:1px solid var(--line);padding:9px;text-align:left;vertical-align:top}th{font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.04em}
.summary{display:grid;grid-template-columns:repeat(6,minmax(120px,1fr));gap:12px;margin:18px 0}.metric{background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:14px}.metric b{display:block;font-size:28px;line-height:1}.metric span{display:block;color:var(--muted);font-size:12px;margin-top:4px}
.status{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}.role{font-weight:650}.badges,.lanes,.filters{display:flex;gap:8px;margin:16px 0;flex-wrap:wrap}.badge,.lane{background:var(--panel);border:1px solid var(--line);padding:7px 9px;border-radius:7px}.lane{min-width:180px}.lane b{display:block}.filters{background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:10px}.filters label{display:grid;gap:4px;font-size:12px;color:var(--muted)}.filters select{min-width:150px}.filters button{align-self:end}.layout{display:grid;grid-template-columns:minmax(0,1fr) 390px;gap:24px}.panel{background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:14px;margin-bottom:16px}.panel h2{margin-top:0}.panel p{border-bottom:1px solid #eef1f5;padding-bottom:8px}.muted{color:var(--muted)}.bad{color:var(--bad)}.ok{color:var(--good)}.warn{color:var(--warn)}
.filters input[type=search]{min-width:240px}
.workstream-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:12px;margin-bottom:22px}.workstream-card{background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:12px}.workstream-card h3{margin:.1rem 0 .5rem;font-size:16px}.progress{height:8px;background:#eef1f5;border-radius:999px;overflow:hidden;margin:10px 0}.progress span{display:block;height:100%;background:var(--accent)}.mini{display:flex;gap:8px;flex-wrap:wrap;color:var(--muted);font-size:12px}.section-head{display:flex;justify-content:space-between;align-items:end;gap:12px}.section-head h2{margin-bottom:8px}
.gate-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(300px,1fr));gap:12px;margin:16px 0}.gate-group,.gate{background:var(--panel);border:1px solid var(--line);border-left:4px solid var(--good);border-radius:8px;padding:12px}.gate-group.missing,.gate.missing{border-left-color:var(--warn)}.gate-group.no_tasks,.gate.no_tasks{border-left-color:var(--muted)}.gate-group h3,.gate h3{font-size:16px;margin:.1rem 0 .35rem}.gate ul{margin:.5rem 0 0;padding-left:18px}.gate-list{margin-top:10px}.gate-list summary{cursor:pointer;color:var(--muted);font-size:13px}.gate-list .gate{margin-top:8px;border-left-width:3px}.table-note{margin:-16px 0 24px;color:var(--muted);font-size:12px}.activity-head{display:flex;justify-content:space-between;gap:8px;align-items:end}.activity-form{display:flex;gap:8px;flex-wrap:wrap}.activity-form label{display:grid;gap:3px;font-size:12px;color:var(--muted)}.activity-form select{max-width:130px}
@media(max-width:980px){.layout{grid-template-columns:1fr}.summary{grid-template-columns:repeat(2,1fr)}.topbar{display:block}}
</style><script>
const events = new EventSource("/events");
events.addEventListener("refresh", () => window.location.reload());
</script></head><body>
<div class="shell">
<div class="topbar">
<div><div class="eyebrow">Fairway control room</div><h1>Workstream Dashboard</h1><p>Tasks, lanes, reviews, evidence, and health for the current project.</p></div>
<div class="badge">filtered: {{.Summary.Filtered}} / {{.Summary.Total}}</div>
</div>
<section class="summary" aria-label="Track summary">
<div class="metric"><b>{{.Summary.Total}}</b><span>Total tasks</span></div>
<div class="metric"><b>{{.Summary.Ready}}</b><span>Ready</span></div>
<div class="metric"><b>{{.Summary.InProgress}}</b><span>In progress</span></div>
<div class="metric"><b>{{.Summary.Blocked}}</b><span>Blocked</span></div>
<div class="metric"><b>{{.Summary.Done}}</b><span>Done</span></div>
<div class="metric"><b>{{.Summary.Workstreams}}</b><span>Workstreams</span></div>
</section>
<div class="badges">
<span class="badge">in progress: {{.Health.InProgress}}</span>
<span class="badge">stale claims: {{.Health.StaleInProgress}}</span>
<span class="badge">blocked &gt;24h: {{.Health.BlockedOver24h}}</span>
<span class="badge">handoffs &gt;1h: {{.Health.UnacknowledgedOver1Hour}}</span>
<span class="badge">reviews: {{.Health.UnroutedReviews}}</span>
<span class="badge">stale checkpoints: {{len .StaleCheckpoints}}</span>
<span class="badge">active watchers: {{len .Watchers}}</span>
<span class="badge">sessions: {{len .Sessions}}</span>
</div>
<form class="filters" method="get">
<label>Search<input type="search" name="q" value="{{.Filters.Search}}" placeholder="ID, title, path, domain"></label>
<label>Status<select name="status"><option value="">all</option>{{range .FilterOptions.Statuses}}<option value="{{.}}" {{if eq $.Filters.Status .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Profile<select name="profile"><option value="">all</option>{{range .FilterOptions.Profiles}}<option value="{{.}}" {{if eq $.Filters.Profile .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Kind<select name="kind"><option value="">all</option>{{range .FilterOptions.Kinds}}<option value="{{.}}" {{if eq $.Filters.Kind .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Domain<select name="owning_domain"><option value="">all</option>{{range .FilterOptions.OwningDomains}}<option value="{{.}}" {{if eq $.Filters.OwningDomain .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Risk<select name="risk_level"><option value="">all</option>{{range .FilterOptions.RiskLevels}}<option value="{{.}}" {{if eq $.Filters.RiskLevel .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Review<select name="review_domain"><option value="">all</option>{{range .FilterOptions.ReviewDomains}}<option value="{{.}}" {{if eq $.Filters.ReviewDomain .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Table rows<select name="table_limit"><option value="25" {{if eq .Filters.TableLimit 25}}selected{{end}}>25</option><option value="50" {{if eq .Filters.TableLimit 50}}selected{{end}}>50</option><option value="100" {{if eq .Filters.TableLimit 100}}selected{{end}}>100</option><option value="200" {{if eq .Filters.TableLimit 200}}selected{{end}}>200</option></select></label>
<input type="hidden" name="activity_kind" value="{{.Filters.ActivityKind}}">
<input type="hidden" name="activity_limit" value="{{.Filters.ActivityLimit}}">
<button type="submit">Apply</button><a href="/">Clear</a><span class="muted">filtered: {{.Summary.Filtered}} / {{.Summary.Total}}</span>
</form>
{{if .GateGroups}}
<div class="section-head"><h2>Gate Readiness</h2><span class="muted">profile gates evaluated against all tasks</span></div>
<section class="gate-grid">
{{range .GateGroups}}
<article class="gate-group {{.Status}}">
<h3>{{.Label}}</h3>
<div class="mini"><span>{{.GateCount}} gate(s)</span><span>{{.SatisfiedCount}}/{{.TaskCount}} task checks satisfied</span>{{if .BlockingMissing}}<span class="bad">{{.BlockingMissing}} blocking missing</span>{{end}}{{if .AdvisoryMissing}}<span class="warn">{{.AdvisoryMissing}} advisory missing</span>{{end}}{{if .ReportOnlyMisses}}<span class="warn">{{.ReportOnlyMisses}} report-only missing</span>{{end}}{{if .NoTaskCount}}<span class="muted">{{.NoTaskCount}} no-task gate(s)</span>{{end}}{{if and (eq .Status "satisfied") (eq .MissingTaskCount 0)}}<span class="ok">ready</span>{{end}}</div>
<details class="gate-list" {{if .MissingTaskCount}}open{{end}}><summary>Gate details</summary>
{{range .Gates}}
<article class="gate {{.Status}}">
<h3>{{.Profile}} / {{.Name}}</h3>
<div class="mini"><span>{{.Group}}</span><span>{{.Mode}}</span><span>{{.EvidenceType}}</span><span>{{.SatisfiedCount}}/{{.TaskCount}} satisfied</span>{{if .MissingCount}}<span class="warn">{{.MissingCount}} missing</span>{{else if eq .Status "no_tasks"}}<span class="muted">no tasks</span>{{else}}<span class="ok">ready</span>{{end}}</div>
{{if .Description}}<p class="muted">{{.Description}}</p>{{end}}
{{if .Missing}}<ul>{{range .Missing}}<li><a href="/tasks/{{.TaskID}}">{{.TaskID}}</a> {{.Title}} <span class="muted">({{.Kind}}, {{.Status}}; matching evidence {{.Matching}})</span>{{if .Reasons}}<br><small>{{range .Reasons}}{{.}} {{end}}</small>{{end}}</li>{{end}}</ul>{{end}}
</article>
{{end}}
</details>
</article>
{{end}}
</section>
{{end}}
{{if .Workstreams}}
<div class="section-head"><h2>Workstream Progress</h2><span class="muted">{{.Summary.Profiles}} profile(s)</span></div>
<section class="workstream-grid">
{{range .Workstreams}}
<article class="workstream-card">
<h3>{{.Label}}</h3>
<div class="progress" title="{{.Done}} of {{.Total}} done"><span style="width:{{if .Total}}{{printf "%.0f" (percent .Done .Total)}}{{else}}0{{end}}%"></span></div>
<div class="mini"><span>{{.Done}}/{{.Total}} done</span><span>{{.InProgress}} active</span><span>{{.Ready}} ready</span><span>{{.Blocked}} blocked</span></div>
</article>
{{end}}
</section>
{{end}}
<div class="lanes">
{{range .Groups}}<div class="lane"><b>{{.Role}}</b>{{if .Current}}<a href="/tasks/{{.Current.Definition.ID}}">{{.Current.Definition.ID}}</a> {{.Current.Definition.Title}}{{else}}idle{{end}}</div>{{end}}
</div>
<div class="layout">
<main>
<h2>Sessions</h2>
<table><tr><th>ID</th><th>Role</th><th>Status</th><th>Branch</th><th>Task</th><th>Backend</th></tr>
{{range .Sessions}}<tr><td>{{.ID}}</td><td class="role">{{.Role}}</td><td class="status">{{.Status}}</td><td>{{.Branch}}</td><td>{{if .TaskID}}<a href="/tasks/{{.TaskID}}">{{.TaskID}}</a>{{end}}</td><td>{{.SessionBackend}} {{.Provider}}</td></tr>{{else}}<tr><td colspan="6">no live sessions</td></tr>{{end}}
</table>
<h2>Worktrees</h2>
<table><tr><th>Role</th><th>Branch</th><th>State</th><th>Commit</th><th>Path</th></tr>
{{range .Worktrees}}<tr><td class="role">{{.Role}}</td><td>{{.Branch}}</td><td>{{if .Dirty}}<span class="bad">dirty</span>{{else}}<span class="ok">clean</span>{{end}} {{if not .Exists}}missing{{else if not .Registered}}unregistered{{end}}</td><td>{{.LastCommit}}</td><td class="muted">{{.Path}}</td></tr>{{else}}<tr><td colspan="5">no configured worktrees</td></tr>{{end}}
</table>
{{if .Workstreams}}
<h2>Workstreams</h2>
{{range .Workstreams}}
<h3>{{.Label}}</h3>
<table><tr><th>ID</th><th>Title</th><th>Role</th><th>Status</th><th>Domain</th><th>Risk</th><th>Review domains</th></tr>
{{range takeTasks .Tasks $.Filters.TableLimit}}<tr><td><a href="/tasks/{{.Definition.ID}}">{{.Definition.ID}}</a></td><td>{{.Definition.Title}}</td><td class="role">{{.Definition.Role}}</td><td class="status">{{.Status}}</td><td>{{.Definition.OwningDomain}}</td><td>{{.Definition.RiskLevel}}</td><td>{{range .Definition.ReviewDomains}}<code>{{.}}</code> {{end}}</td></tr>{{else}}<tr><td colspan="7">no tasks</td></tr>{{end}}
</table>
{{if gt (moreTasks .Tasks $.Filters.TableLimit) 0}}<p class="table-note">showing first {{$.Filters.TableLimit}} of {{.Total}} tasks; narrow filters or raise the table row limit to see more.</p>{{end}}
{{end}}
{{end}}
{{range .Groups}}
<h2>{{.Role}}</h2>
<table><tr><th>ID</th><th>Title</th><th>Kind</th><th>Profile</th><th>Status</th><th>Owner</th><th>Review</th><th>Rollup</th></tr>
{{range takeTasks .Tasks $.Filters.TableLimit}}<tr><td><a href="/tasks/{{.Definition.ID}}">{{.Definition.ID}}</a></td><td>{{.Definition.Title}}</td><td>{{.Definition.Kind}}</td><td>{{.Definition.Profile}}</td><td class="status">{{.Status}}</td><td>{{.Owner}}</td><td>{{.ReviewStatus}}</td><td>{{with index $.Rollups .Definition.ID}}{{.Done}}/{{.Total}}{{else}}-{{end}}</td></tr>{{else}}<tr><td colspan="8">no tasks</td></tr>{{end}}
</table>
{{if gt (moreTasks .Tasks $.Filters.TableLimit) 0}}<p class="table-note">showing first {{$.Filters.TableLimit}} of {{len .Tasks}} tasks; narrow filters or raise the table row limit to see more.</p>{{end}}
{{end}}
</main>
<aside>
<section class="panel"><h2>Watchers</h2>{{range .Watchers}}<p><b>{{.ID}}</b> <a href="/tasks/{{.TaskID}}">{{.TaskID}}</a><br><code>{{.Status}}</code> {{.Owner}} {{.Process}}<br><small>{{.Command}}</small></p>{{else}}<p>none</p>{{end}}</section>
<section class="panel"><h2>Checkpoints</h2>{{range .Checkpoints}}<p><b><a href="/tasks/{{.TaskID}}">{{.TaskID}}</a></b> <code>{{.State}}</code><br>{{.Summary}}<br><small>{{.Owner}} {{.TargetCloseBy}}</small></p>{{else}}<p>none</p>{{end}}</section>
<section class="panel"><div class="activity-head"><h2>Activity</h2><form class="activity-form" method="get">
<input type="hidden" name="q" value="{{.Filters.Search}}"><input type="hidden" name="status" value="{{.Filters.Status}}"><input type="hidden" name="profile" value="{{.Filters.Profile}}"><input type="hidden" name="kind" value="{{.Filters.Kind}}"><input type="hidden" name="owning_domain" value="{{.Filters.OwningDomain}}"><input type="hidden" name="risk_level" value="{{.Filters.RiskLevel}}"><input type="hidden" name="review_domain" value="{{.Filters.ReviewDomain}}"><input type="hidden" name="table_limit" value="{{.Filters.TableLimit}}">
<label>Kind<select name="activity_kind"><option value="">all</option>{{range .FilterOptions.ActivityKinds}}<option value="{{.}}" {{if eq $.Filters.ActivityKind .}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<label>Rows<select name="activity_limit"><option value="10" {{if eq .Filters.ActivityLimit 10}}selected{{end}}>10</option><option value="25" {{if eq .Filters.ActivityLimit 25}}selected{{end}}>25</option><option value="50" {{if eq .Filters.ActivityLimit 50}}selected{{end}}>50</option><option value="100" {{if eq .Filters.ActivityLimit 100}}selected{{end}}>100</option><option value="200" {{if eq .Filters.ActivityLimit 200}}selected{{end}}>200</option></select></label>
<button type="submit">Apply</button></form></div>
<p class="muted">showing {{len .Activity}} of {{.ActivityTotal}}</p>
{{range .Activity}}<p><b>{{.TaskID}}</b> <code>{{.Kind}}</code><br>{{.Summary}}<br><small>{{.CreatedAt}} {{.Actor}}</small></p>{{else}}<p>none</p>{{end}}</section>
</aside>
</div>
</div>
</body></html>`))

var detailTemplate = template.Must(template.New("detail").Parse(`<!doctype html>
<html><head><title>{{.Task.Definition.ID}}</title><style>
body{font-family:system-ui,sans-serif;margin:32px;max-width:960px}.meta{color:#555}pre{background:#f4f4f4;padding:12px}table{border-collapse:collapse;width:100%;margin-bottom:24px}td,th{border-bottom:1px solid #ddd;padding:8px;text-align:left;vertical-align:top}code{background:#f4f4f4;padding:1px 3px}
</style></head><body>
<p><a href="/">back</a></p>
<h1>{{.Task.Definition.ID}}: {{.Task.Definition.Title}}</h1>
<p class="meta">role={{.Task.Definition.Role}} status={{.Task.Status}} owner={{.Task.Owner}} review={{.Task.ReviewStatus}}</p>
{{if .Rollup.Total}}<p class="meta">descendants done: {{.Rollup.Done}}/{{.Rollup.Total}}</p>{{end}}
<form method="post" action="/actions/claim"><input type="hidden" name="csrf" value="{{.CSRFToken}}"><input type="hidden" name="task_id" value="{{.Task.Definition.ID}}"><button type="submit">Claim</button></form>
<form method="post" action="/actions/set-status">
<input type="hidden" name="csrf" value="{{.CSRFToken}}">
<input type="hidden" name="task_id" value="{{.Task.Definition.ID}}">
<select name="status">{{range .States}}<option value="{{.}}" {{if eq $.Task.Status .}}selected{{end}}>{{.}}</option>{{end}}</select>
<input name="reason" placeholder="reason">
<button type="submit">Set status</button>
</form>
<h2>Metadata</h2>
<table>
<tr><th>Kind</th><td>{{.Task.Definition.Kind}}</td></tr>
<tr><th>Profile</th><td>{{.Task.Definition.Profile}}</td></tr>
<tr><th>Owning domain</th><td>{{.Task.Definition.OwningDomain}}</td></tr>
<tr><th>Owning layer</th><td>{{.Task.Definition.OwningLayer}}</td></tr>
<tr><th>Source paths</th><td>{{range .Task.Definition.SourcePaths}}<code>{{.}}</code> {{else}}none{{end}}</td></tr>
<tr><th>Target paths</th><td>{{range .Task.Definition.TargetPaths}}<code>{{.}}</code> {{else}}none{{end}}</td></tr>
<tr><th>Review domains</th><td>{{range .Task.Definition.ReviewDomains}}<code>{{.}}</code> {{else}}none{{end}}</td></tr>
<tr><th>Risk</th><td>{{.Task.Definition.RiskLevel}}</td></tr>
<tr><th>Migration type</th><td>{{.Task.Definition.MigrationType}}</td></tr>
</table>
<h2>Notes</h2><pre>{{.Task.Definition.Notes}}</pre>
<h2>Dependencies</h2><ul>{{range .Task.Definition.Dependencies}}<li>{{.}}</li>{{else}}<li>none</li>{{end}}</ul>
<h2>Acceptance</h2><ul>{{range .Task.Definition.AcceptanceChecks}}<li>{{.}}</li>{{else}}<li>none</li>{{end}}</ul>
<h2>History</h2>{{range .Transitions}}<p><code>{{if .FromStatus}}{{.FromStatus}}{{else}}new{{end}} -> {{.ToStatus}}</code> by {{.Actor}} {{.Reason}}</p>{{else}}<p>none</p>{{end}}
<h2>Evidence</h2>{{range .Evidence}}<p><code>{{.Result}}</code> {{.CommandText}} {{.ArtifactPath}}</p>{{else}}<p>none</p>{{end}}
<h2>Handoffs</h2>{{range .Handoffs}}<p>to <b>{{.ToRole}}</b>: {{.Payload}}</p>{{else}}<p>none</p>{{end}}
<h2>Reviews</h2>{{range .Reviews}}<p><b>{{.Verdict}}</b> by {{.Reviewer}}: {{.Reason}}</p>{{else}}<p>none</p>{{end}}
</body></html>`))

func URL(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}

var multiTemplate = template.Must(template.New("multi").Parse(`<!doctype html>
<html><head><title>fairway multi-project</title><style>
body{font-family:system-ui,sans-serif;margin:32px;background:#f7f7f5;color:#1f2933}
table{border-collapse:collapse;width:100%;background:white;margin-bottom:24px}td,th{border-bottom:1px solid #ddd;padding:8px;text-align:left}
.project{background:white;border:1px solid #ddd;padding:16px;margin-bottom:24px}.badges{display:flex;gap:8px;flex-wrap:wrap}.badge{border:1px solid #ddd;border-radius:6px;padding:6px 8px}.muted{color:#667085}.status{font-family:monospace}
</style></head><body>
<h1>fairway multi-project</h1>
{{range .Projects}}
<section class="project">
<h2>{{.Name}}</h2>
<p class="muted">{{.Path}}</p>
{{if .Error}}<p>{{.Error}}</p>{{else}}
<div class="badges"><span class="badge">tasks: {{.TaskCount}}</span><span class="badge">sessions: {{.SessionCount}}</span><span class="badge">checkpoints: {{.CheckpointCount}}</span><span class="badge">watchers: {{.WatcherCount}}</span></div>
<table><tr><th>ID</th><th>Title</th><th>Role</th><th>Status</th><th>Review</th></tr>
{{range .Tasks}}<tr><td>{{.Definition.ID}}</td><td>{{.Definition.Title}}</td><td>{{.Definition.Role}}</td><td class="status">{{.Status}}</td><td>{{.ReviewStatus}}</td></tr>{{else}}<tr><td colspan="5">no tasks</td></tr>{{end}}
</table>
{{end}}
</section>
{{else}}<p>no registered projects</p>{{end}}
</body></html>`))
