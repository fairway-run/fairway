package dashboard

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
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
	mux.Handle("/assets/", dashboardAssetHandler())
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
	mux.Handle("/assets/", dashboardAssetHandler())
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
	sessions, err := s.store.Sessions(r.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		Sessions    []store.Session
		Rollup      Rollup
		CSRFToken   string
		States      []string
	}{task, transitions, evidence, handoffs, reviews, sessionsForDashboardTask(sessions, id), rollups[task.Definition.ID], s.csrfToken, dashboardMutableStates(s.cfg)}
	_ = detailTemplate.Execute(w, data)
}

func sessionsForDashboardTask(sessions []store.Session, taskID string) []store.Session {
	var out []store.Session
	for _, session := range sessions {
		if session.TaskID == taskID {
			out = append(out, session)
		}
	}
	return out
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
	initial, err := s.store.EventSources(r.Context(), dashboardEventPollLimit)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
		flusher.Flush()
		return
	}
	seen := map[string]bool{}
	for _, source := range initial {
		for _, event := range sseEventsFromSource(source) {
			seen[event.ID] = true
		}
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sources, err := s.store.EventSources(r.Context(), dashboardEventPollLimit)
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
				flusher.Flush()
				return
			}
			for i := len(sources) - 1; i >= 0; i-- {
				source := sources[i]
				events := sseEventsFromSource(source)
				if len(events) == 0 {
					continue
				}
				sourceSeen := true
				for _, event := range events {
					if !seen[event.ID] {
						sourceSeen = false
						break
					}
				}
				if sourceSeen {
					continue
				}
				for _, event := range events {
					if seen[event.ID] {
						continue
					}
					if err := writeSSEEvent(w, event); err != nil {
						fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
						flusher.Flush()
						return
					}
					seen[event.ID] = true
				}
				gateEvents, err := s.gateChangeEvents(r.Context(), events[0].ID, source.Cursor.At)
				if err != nil {
					fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
					flusher.Flush()
					return
				}
				for _, event := range gateEvents {
					if seen[event.ID] {
						continue
					}
					if err := writeSSEEvent(w, event); err != nil {
						fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
						flusher.Flush()
						return
					}
					seen[event.ID] = true
				}
				if err := writeLegacyRefresh(w, events[0].ID); err != nil {
					fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
					flusher.Flush()
					return
				}
				flusher.Flush()
			}
		}
	}
}

//go:embed assets/templates/*.html assets/templates/partials/*.html assets/css/*.css assets/js/*.js
var dashboardAssets embed.FS

var indexTemplate = mustEmbeddedTemplate("index", "assets/templates/index.html", template.FuncMap{
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
})

var detailTemplate = mustEmbeddedTemplate("detail", "assets/templates/task-detail.html", nil)

var wallTemplate = mustEmbeddedTemplateSet("wall", []string{
	"assets/templates/layout.html",
	"assets/templates/wall.html",
	"assets/templates/partials/lane-card.html",
	"assets/templates/partials/gate-gauge.html",
	"assets/templates/partials/provider-chip.html",
}, dashboardTemplateFuncs())

func URL(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}

var multiTemplate = mustEmbeddedTemplate("multi", "assets/templates/multi.html", nil)

func dashboardAssetHandler() http.Handler {
	assets, err := fs.Sub(dashboardAssets, "assets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/assets/", http.FileServer(http.FS(assets)))
}

func dashboardTemplateFuncs() template.FuncMap {
	funcs := template.FuncMap{
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
		"wallLaneTasks":       wallLaneTasks,
		"wallProviderClass":   wallProviderClass,
		"wallActiveSessions":  wallActiveSessions,
		"wallTasksMoving":     wallTasksMoving,
		"wallHandoffCount":    wallHandoffCount,
		"wallDoneToday":       wallDoneToday,
		"wallTaskHasProvider": wallTaskHasProvider,
		"safeClass":           safeDashboardClass,
		"dict":                templateDict,
	}
	return funcs
}

func mustEmbeddedTemplate(name, path string, funcs template.FuncMap) *template.Template {
	tmpl := template.New(name)
	if funcs != nil {
		tmpl = tmpl.Funcs(funcs)
	}
	data, err := dashboardAssets.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return template.Must(tmpl.Parse(string(data)))
}

func mustEmbeddedTemplateSet(name string, paths []string, funcs template.FuncMap) *template.Template {
	tmpl := template.New(name)
	if funcs != nil {
		tmpl = tmpl.Funcs(funcs)
	}
	for _, path := range paths {
		data, err := dashboardAssets.ReadFile(path)
		if err != nil {
			panic(err)
		}
		if _, err := tmpl.Parse(string(data)); err != nil {
			panic(err)
		}
	}
	return tmpl
}

func wallLaneTasks(tasks []store.Task, lane string, sessions []store.Session) []store.Task {
	var out []store.Task
	for _, task := range tasks {
		switch lane {
		case "backlog":
			if task.Status == "todo" {
				out = append(out, task)
			}
		case "claimed":
			if task.Status == "in_progress" && !wallTaskHasProvider(task, sessions) {
				out = append(out, task)
			}
		case "working":
			if task.Status == "in_progress" && wallTaskHasProvider(task, sessions) {
				out = append(out, task)
			}
		case "review":
			if task.Status != "done" && task.ReviewStatus != "" && task.ReviewStatus != "approved" {
				out = append(out, task)
			}
		case "done":
			if task.Status == "done" {
				out = append(out, task)
			}
		}
	}
	return out
}

func wallTaskHasProvider(task store.Task, sessions []store.Session) bool {
	for _, session := range sessions {
		if session.TaskID == task.Definition.ID && isActiveDashboardSession(session) {
			return true
		}
	}
	return false
}

func wallProviderClass(task store.Task, sessions []store.Session) string {
	for _, session := range sessions {
		if session.TaskID == task.Definition.ID && strings.TrimSpace(session.Provider) != "" {
			return safeDashboardClass(session.Provider)
		}
	}
	if task.Owner != "" {
		return safeDashboardClass(task.Owner)
	}
	return safeDashboardClass(task.Definition.Role)
}

func wallActiveSessions(sessions []store.Session) []store.Session {
	var out []store.Session
	for _, session := range sessions {
		if isActiveDashboardSession(session) {
			out = append(out, session)
		}
	}
	return out
}

func isActiveDashboardSession(session store.Session) bool {
	if session.EndedAt != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(session.Status)) {
	case "", "running", "active", "attached":
		return true
	default:
		return false
	}
}

func wallTasksMoving(groups []RoleGroup) int {
	count := 0
	for _, group := range groups {
		for _, task := range group.Tasks {
			if task.Status == "in_progress" {
				count++
			}
		}
	}
	return count
}

func wallHandoffCount(activity []store.Activity) int {
	count := 0
	for _, item := range activity {
		if item.Kind == "handoff" {
			count++
		}
	}
	return count
}

func wallDoneToday(groups []RoleGroup) int {
	today := time.Now().UTC().Format("2006-01-02")
	count := 0
	for _, group := range groups {
		for _, task := range group.Tasks {
			if task.Status != "done" {
				continue
			}
			if task.UpdatedAt == "" || strings.HasPrefix(task.UpdatedAt, today) {
				count++
			}
		}
	}
	return count
}

func safeDashboardClass(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func templateDict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict requires key/value pairs")
	}
	out := map[string]any{}
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key at index %d is not a string", i)
		}
		out[key] = values[i+1]
	}
	return out, nil
}
