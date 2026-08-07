package dashboard

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/subashram/fairway/internal/audit"
	"github.com/subashram/fairway/internal/completionhandback"
	"github.com/subashram/fairway/internal/config"
	coord "github.com/subashram/fairway/internal/coordinator"
	"github.com/subashram/fairway/internal/evidencemodel"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/livewindow"
	"github.com/subashram/fairway/internal/qualityrecord"
	"github.com/subashram/fairway/internal/reconcile"
	"github.com/subashram/fairway/internal/reviewpolicy"
	"github.com/subashram/fairway/internal/reviewstate"
	"github.com/subashram/fairway/internal/state"
	"github.com/subashram/fairway/internal/store"
)

type Server struct {
	store                   *store.Store
	cfg                     config.Config
	root                    string
	roles                   []string
	worktrees               []WorktreeStatus
	csrfToken               string
	snapshots               *dashboardSnapshotCache
	eventPollInterval       time.Duration
	reviewWaitSweepInterval time.Duration
	sseStats                *ssePollStats
	now                     func() time.Time
}

type ssePollStats struct {
	cursorChecks         atomic.Uint64
	sourceHydrations     atomic.Uint64
	reviewWaitSweeps     atomic.Uint64
	targetWaitHydrations atomic.Uint64
}

const taskDetailCompletionActionLimit = 10000

type WorktreeStatus struct {
	Role           string
	Branch         string
	Path           string
	Registered     bool
	Exists         bool
	Dirty          bool
	LastCommit     string
	GitUnavailable bool
	Diagnostic     string
}

type Rollup struct {
	Done  int
	Total int
}

type ProjectStore struct {
	Name       string
	Path       string
	DBPath     string
	ConfigPath string
	Store      *store.Store
	Error      string
}

func New(s *store.Store, cfg config.Config, roles []string, worktrees []WorktreeStatus) *Server {
	return NewWithRoot(s, cfg, roles, worktrees, "")
}

func NewWithRoot(s *store.Store, cfg config.Config, roles []string, worktrees []WorktreeStatus, root string) *Server {
	return &Server{
		store:                   s,
		cfg:                     cfg,
		root:                    root,
		roles:                   roles,
		worktrees:               worktrees,
		csrfToken:               newCSRFToken(),
		snapshots:               newDashboardSnapshotCache(dashboardSnapshotCacheTTL),
		eventPollInterval:       time.Second,
		reviewWaitSweepInterval: reviewWaitEventSweepInterval(cfg),
		sseStats:                &ssePollStats{},
		now:                     time.Now,
	}
}

func NewMulti(projects []ProjectStore) http.Handler {
	mux := http.NewServeMux()
	server := &MultiServer{projects: projects, csrfToken: newCSRFToken()}
	mux.Handle("/assets/", dashboardAssetHandler())
	mux.HandleFunc("/board", server.board)
	mux.HandleFunc("/board/export", server.boardExport)
	mux.HandleFunc("/reports", server.reports)
	mux.HandleFunc("/wall", server.wallRedirect)
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		type projectView struct {
			Name            string
			Path            string
			DBPath          string
			ConfigPath      string
			Tasks           []store.Task
			TaskCount       int
			SessionCount    int
			CheckpointCount int
			WatcherCount    int
			Error           string
		}
		var views []projectView
		for _, project := range projects {
			view := projectView{Name: project.Name, Path: project.Path, DBPath: project.DBPath, ConfigPath: project.ConfigPath, Error: project.Error}
			if project.Store == nil || project.Error != "" {
				if view.Error == "" {
					view.Error = "project store unavailable"
				}
				views = append(views, view)
				continue
			}
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
	mux.HandleFunc("/{$}", server.wall)
	return mux
}

type MultiServer struct {
	projects  []ProjectStore
	csrfToken string
}

type RoleGroup struct {
	Role    string
	Current *store.Task
	Tasks   []store.Task
}

type ProjectWallGroup struct {
	Name    string
	Path    string
	Groups  []RoleGroup
	Summary DashboardSummary
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
	Role          string
	Status        string
	Statuses      []string
	Sort          string
	Columns       []string
	Profile       string
	Kind          string
	OwningDomain  string
	RiskLevel     string
	ReviewDomain  string
	Tags          []string
	Project       string
	ActivityKind  string
	Tab           string
	ActivityLimit int
	TableLimit    int
	TablePage     int
	Workstreams   string
}

type FilterChip struct {
	Key   string
	Label string
	Value string
	Href  string
}

type BoardColumn struct {
	Key          string
	Label        string
	Visible      bool
	Sortable     bool
	ToggleHref   string
	MoveUpHref   string
	MoveDownHref string
}

type TablePagination struct {
	Page        int
	PageSize    int
	TotalRows   int
	TotalPages  int
	Start       int
	End         int
	PrevHref    string
	NextHref    string
	Virtualized bool
	WindowSize  int
}

type FilterOptions struct {
	Statuses      []string
	Projects      []string
	Profiles      []string
	Kinds         []string
	OwningDomains []string
	RiskLevels    []string
	ReviewDomains []string
	Tags          []string
	ActivityKinds []string
}

type DashboardViewData struct {
	View                  string
	MultiProject          bool
	Summary               DashboardSummary
	Gates                 []GateStatus
	GateGroups            []GateGroup
	Groups                []RoleGroup
	ProjectGroups         []ProjectWallGroup
	MissingReviewDomains  map[string][]string
	TableRows             []store.Task
	Pagination            TablePagination
	Workstreams           []WorkstreamGroup
	WorkstreamTotal       int
	WorkstreamsExpanded   bool
	WorkstreamsShowAll    string
	WorkstreamsCompact    string
	Filters               TaskFilters
	FilterOptions         FilterOptions
	FilterChips           []FilterChip
	ClearFiltersHref      string
	BoardColumns          []BoardColumn
	PersonalViews         []SavedView
	TeamViews             []SavedView
	CurrentViewQuery      string
	Activity              []store.Activity
	ActivityTotal         int
	Health                store.Health
	Sessions              []store.Session
	Worktrees             []WorktreeStatus
	Checkpoints           []store.Checkpoint
	StaleCheckpoints      []store.Checkpoint
	Watchers              []store.Watcher
	Rollups               map[string]Rollup
	TaskRoles             map[string]string
	ActiveReport          reconcile.ActiveReport
	CoordinatorPlan       coord.Plan
	Coordination          CoordinationIntelligence
	CloseoutReports       []reconcile.CloseoutReport
	Audit                 AuditDiagnostics
	DiagnosticsDeferred   bool
	GitProjectionDeferred bool
	GitDiagnostic         string
	DiagnosticPanel       string
	ReadOnly              bool
	CSRFToken             string
	MutableStates         []string
	Roles                 []string
}

type TaskDetailViewData struct {
	View                 string
	Groups               []RoleGroup
	Sessions             []store.Session
	Activity             []store.Activity
	BackHref             string
	Task                 store.Task
	Transitions          []store.Transition
	Evidence             []store.Evidence
	UXMediaEvidence      []evidencemodel.UXMediaEvidence
	UXMediaSummary       evidencemodel.UXMediaSummary
	EvidenceViews        []EvidenceViewRow
	Handoffs             []store.Handoff
	Reviews              []store.Review
	MissingReviewDomains []string
	ReviewPolicy         reviewpolicy.Evaluation
	ReviewStatus         string
	ReviewHandback       *coord.ReviewCompletionHandback
	ReviewNotifications  []reviewstate.ReviewNotificationStatus
	ReviewWaits          []reviewstate.ReviewWait
	CompletionHandbacks  []completionhandback.Handback
	CompletionActions    []coord.PlanAction
	TaskSessions         []store.Session
	Usage                []store.ProviderUsage
	UsageRollups         []store.UsageRollup
	Batches              []store.WorkBatch
	TaskGates            []TaskGateStatus
	TaskRules            []TaskRuleStatus
	Rollup               Rollup
	CSRFToken            string
	States               []string
	Audit                AuditDiagnostics
	ActiveFindings       []reconcile.ActiveFinding
	ReadOnly             bool
	Recommendation       CommonPathRecommendation
	QualityRecord        qualityrecord.Record
}

type TaskGateStatus struct {
	Profile      string
	Name         string
	Mode         string
	EvidenceType string
	Description  string
	Status       string
	Matching     int
	Reasons      []string
}

type AuditDiagnostics struct {
	WorkCoverage          audit.WorkCoverageReport
	WorkCoverageAvailable bool
	WorkCoverageError     string
	CILearning            audit.CILearningReport
	CILearningError       string
}

func (d AuditDiagnostics) HighRiskCount() int {
	count := d.WorkCoverage.Summary.CommitsWithoutTaskID
	count += d.WorkCoverage.Summary.ChangedFilesUncovered
	count += d.WorkCoverage.Summary.OrphanEvidence
	count += d.WorkCoverage.Summary.DoneWithoutRequiredEvidence
	count += d.WorkCoverage.Summary.MissingRequiredReviews
	count += d.CILearning.Summary.MissingFollowUps
	count += d.CILearning.Summary.ApprovalGatedBlocker
	return count
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.handler())
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/assets/", dashboardAssetHandler())
	mux.HandleFunc("/wall", s.wall)
	mux.HandleFunc("/board", s.board)
	mux.HandleFunc("/board/panels/diagnostics", s.boardDiagnosticsPanel)
	mux.HandleFunc("/board/export", s.boardExport)
	mux.HandleFunc("/reports", s.reports)
	mux.HandleFunc("/quality", s.quality)
	mux.HandleFunc("/controls", s.controls)
	mux.HandleFunc("/tasks/", s.task)
	mux.HandleFunc("/evidence/artifact", s.artifact)
	mux.HandleFunc("/actions/claim", s.claim)
	mux.HandleFunc("/actions/set-status", s.setStatus)
	mux.HandleFunc("/actions/bulk/claim", s.bulkClaim)
	mux.HandleFunc("/actions/bulk/handoff", s.bulkHandoff)
	mux.HandleFunc("/actions/bulk/set-status", s.bulkSetStatus)
	mux.HandleFunc("/actions/bulk/evidence", s.bulkEvidence)
	mux.HandleFunc("/actions/views/save", s.saveView)
	mux.HandleFunc("/events", s.events)
	mux.HandleFunc("/{$}", s.index)
	return mux
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	s.overview(w, r)
}

func (s *Server) board(w http.ResponseWriter, r *http.Request) {
	timing := newDashboardTiming("board", r)
	defer timing.logIfSlow()
	data, err := s.dashboardViewData(r, "board", timing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	_ = boardTemplate.ExecuteTemplate(w, "layout", data)
	timing.add("template.board", time.Since(start), "")
}

func (s *Server) boardDiagnosticsPanel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	panel := strings.TrimSpace(r.URL.Query().Get("panel"))
	if panel != "" && panel != "coordination" && panel != "reconciliation" && panel != "closeout" && panel != "audit" {
		http.Error(w, "unknown diagnostics panel", http.StatusNotFound)
		return
	}
	timing := newDashboardTiming("board-diagnostics-panel", r)
	defer timing.logIfSlow()
	panelRequest := r.Clone(r.Context())
	u := *r.URL
	query := u.Query()
	query.Set("tab", "diagnostics")
	u.RawQuery = query.Encode()
	panelRequest.URL = &u
	data, err := s.dashboardViewData(panelRequest, "board", timing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	if err := boardTemplate.ExecuteTemplate(w, "diagnostics-panels", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	timing.add("template.diagnostics_panel", time.Since(start), "")
}

func (s *MultiServer) board(w http.ResponseWriter, r *http.Request) {
	timing := newDashboardTiming("multi-board", r)
	defer timing.logIfSlow()
	data, err := s.dashboardViewData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	_ = boardTemplate.ExecuteTemplate(w, "layout", data)
	timing.add("template.board", time.Since(start), "")
}

func (s *MultiServer) wall(w http.ResponseWriter, r *http.Request) {
	timing := newDashboardTiming("multi-wall", r)
	defer timing.logIfSlow()
	data, err := s.dashboardViewData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.View = "wall"
	start := time.Now()
	_ = wallTemplate.ExecuteTemplate(w, "layout", data)
	timing.add("template.wall", time.Since(start), "")
}

func (s *Server) wall(w http.ResponseWriter, r *http.Request) {
	timing := newDashboardTiming("wall", r)
	defer timing.logIfSlow()
	data, err := s.dashboardViewData(r, "wall", timing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	_ = wallTemplate.ExecuteTemplate(w, "layout", data)
	timing.add("template.wall", time.Since(start), "")
}

func (s *Server) reports(w http.ResponseWriter, r *http.Request) {
	timing := newDashboardTiming("reports", r)
	defer timing.logIfSlow()
	data, err := s.reportViewData(r, timing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	switch strings.TrimSpace(r.URL.Query().Get("format")) {
	case "json":
		writeReportJSON(w, data)
	case "csv":
		writeReportCSV(w, data)
	case "md", "markdown":
		writeReportMarkdown(w, data)
	default:
		if err := reportsTemplate.ExecuteTemplate(w, "layout", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	timing.add("template.reports", time.Since(start), "format="+strings.TrimSpace(r.URL.Query().Get("format")))
}

func (s *MultiServer) reports(w http.ResponseWriter, r *http.Request) {
	timing := newDashboardTiming("multi-reports", r)
	defer timing.logIfSlow()
	data, err := s.reportViewData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	switch strings.TrimSpace(r.URL.Query().Get("format")) {
	case "json":
		writeReportJSON(w, data)
	case "csv":
		writeReportCSV(w, data)
	case "md", "markdown":
		writeReportMarkdown(w, data)
	default:
		if err := reportsTemplate.ExecuteTemplate(w, "layout", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	timing.add("template.reports", time.Since(start), "format="+strings.TrimSpace(r.URL.Query().Get("format")))
}

func (s *Server) wallRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/wall", http.StatusTemporaryRedirect)
}

func (s *MultiServer) wallRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) dashboardViewData(r *http.Request, view string, timing *dashboardTiming) (DashboardViewData, error) {
	if dashboardSnapshotCacheable(r) {
		key := dashboardSnapshotKey("dashboard:"+view, r)
		data, status, err := dashboardSnapshotGet(s.snapshots, key, func() (DashboardViewData, error) {
			return s.buildDashboardViewData(r, view, timing)
		})
		timing.add("dashboard.snapshot_cache", 0, "status="+status)
		return data, err
	}
	return s.buildDashboardViewData(r, view, timing)
}

func (s *Server) buildDashboardViewData(r *http.Request, view string, timing *dashboardTiming) (DashboardViewData, error) {
	var tasks []store.Task
	if err := timing.step("dashboard.tasks", func() error {
		var err error
		tasks, err = s.store.AllTasks(r.Context())
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	tasks = tagTasksProject(tasks, s.cfg.Fairway.ProjectName)
	var health store.Health
	if err := timing.step("dashboard.health", func() error {
		var err error
		health, err = s.store.Health(r.Context())
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	var sessions []store.Session
	if err := timing.step("dashboard.sessions", func() error {
		var err error
		sessions, err = s.store.Sessions(r.Context(), false)
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	var checkpoints []store.Checkpoint
	if err := timing.step("dashboard.checkpoints", func() error {
		var err error
		checkpoints, err = s.store.Checkpoints(r.Context(), "", false)
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	var staleCheckpoints []store.Checkpoint
	if err := timing.step("dashboard.stale_checkpoints", func() error {
		var err error
		staleCheckpoints, err = s.store.Checkpoints(r.Context(), time.Now().UTC().Format("2006-01-02"), false)
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	var watchers []store.Watcher
	if err := timing.step("dashboard.watchers", func() error {
		var err error
		watchers, err = s.store.Watchers(r.Context(), false)
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	filters := taskFiltersFromRequest(r)
	diagnosticPanel := ""
	if dashboardDiagnosticsPanelRequest(r) {
		diagnosticPanel = strings.TrimSpace(r.URL.Query().Get("panel"))
	}
	allDiagnosticPanels := diagnosticPanel == ""
	needCoordination := allDiagnosticPanels || diagnosticPanel == "coordination"
	needReconciliation := allDiagnosticPanels || diagnosticPanel == "reconciliation"
	needCloseout := allDiagnosticPanels || diagnosticPanel == "closeout"
	needAudit := allDiagnosticPanels || diagnosticPanel == "audit"
	diagnosticsDeferred := view == "board" && filters.Tab == "diagnostics" && !dashboardDiagnosticsPanelRequest(r)
	boardFastPath := view == "board" && (filters.Tab != "diagnostics" || diagnosticsDeferred)
	wallFastPath := view == "wall"
	heavyDiagnosticsDeferred := boardFastPath || wallFastPath
	var activity []store.Activity
	if err := timing.step("dashboard.activity", func() error {
		var err error
		activity, err = s.store.ActivityFiltered(r.Context(), store.ActivityOptions{
			Limit: maxActivityFetchLimit,
			Kind:  filters.ActivityKind,
		})
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	filteredActivity, activityTotal := filterActivity(activity, filters.ActivityKind, filters.ActivityLimit)
	var gates []GateStatus
	if err := timing.step("dashboard.gates", func() error {
		var err error
		gates, err = s.dashboardGateStatuses(r.Context(), tasks, 8, timing)
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	var readyTasks []store.Task
	if err := timing.step("dashboard.ready", func() error {
		var err error
		readyTasks, err = s.store.Ready(r.Context(), "", s.cfg.States.Terminal)
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	readySet := taskIDSet(readyTasks)
	gateGroups := groupGateStatuses(gates)
	displayTasks := filterTasks(tasks, filters, s.cfg.Fairway.ProjectName)
	rollups := taskRollups(tasks, map[string]bool{"done": true})
	workstreams := groupWorkstreams(displayTasks, readySet)
	visibleWorkstreams := visibleWorkstreams(workstreams, filters)
	groups := groupTasks(displayTasks, s.roles)
	tableSource := append([]store.Task(nil), displayTasks...)
	sortBoardRows(tableSource, filters)
	tableRows, pagination := paginateBoardRows(tableSource, filters)
	reviewDomainScope := displayTasks
	if view == "board" {
		reviewDomainScope = tableRows
	}
	var missingReviewDomains map[string][]string
	if err := timing.step("dashboard.missing_review_domains", func() error {
		var err error
		missingReviewDomains, err = s.dashboardMissingReviewDomainsByTask(r.Context(), reviewDomainScope, tasks, timing)
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	var activeReport reconcile.ActiveReport
	if !boardFastPath && needReconciliation {
		if err := timing.step("dashboard.active_reconcile", func() error {
			var err error
			activeReport, err = reconcile.Active(r.Context(), s.store, reconcile.ActiveOptions{Terminal: s.cfg.States.Terminal, StaleCheckpointAfter: 2 * time.Hour})
			return err
		}); err != nil {
			return DashboardViewData{}, err
		}
	}
	var coordinatorPlan coord.Plan
	if heavyDiagnosticsDeferred {
		coordinatorPlan.Summary.TopClassification = view + " fast path"
		coordinatorPlan.Summary.TopReason = "Open Diagnostics for coordinator plan, waits, active reconcile, and closeout detail."
		if boardFastPath {
			timing.add("dashboard.board_fast_path", 0, "skipped=active_reconcile,coordinator_plan,track_memories,closeout_reports,audit_diagnostics")
		} else {
			timing.add("dashboard.wall_fast_path", 0, "skipped=coordinator_plan,track_memories,closeout_reports,audit_diagnostics")
		}
	} else if needCoordination {
		if err := timing.step("dashboard.coordinator_plan", func() error {
			var err error
			coordinatorPlan, err = coord.BuildPlan(r.Context(), s.cfg, s.store, coord.PlanOptions{
				Worktrees:            dashboardWorktreeFacts(s.worktrees),
				StaleCheckpointAfter: 2 * time.Hour,
				MonitorHandbackAfter: 2 * time.Hour,
				ReadyLimit:           5,
				RecommendationLimit:  5,
			})
			return err
		}); err != nil {
			return DashboardViewData{}, err
		}
	}
	var memories []store.TrackMemory
	if !heavyDiagnosticsDeferred && needCoordination {
		if err := timing.step("dashboard.track_memories", func() error {
			var err error
			memories, err = s.store.TrackMemories(r.Context())
			return err
		}); err != nil {
			return DashboardViewData{}, err
		}
	}
	var closeoutReports []reconcile.CloseoutReport
	if !heavyDiagnosticsDeferred && needCloseout {
		if err := timing.step("dashboard.closeout_reports", func() error {
			var err error
			closeoutReports, err = s.dashboardCloseoutReports(r.Context(), tasks, 8, timing)
			return err
		}); err != nil {
			return DashboardViewData{}, err
		}
	}
	var auditDiagnostics AuditDiagnostics
	if view == "board" && filters.Tab == "diagnostics" && !diagnosticsDeferred && needAudit {
		_ = timing.step("dashboard.audit_diagnostics", func() error {
			auditDiagnostics = s.auditDiagnostics(r.Context(), "")
			return nil
		})
	}
	var personalViews, teamViews []SavedView
	if err := timing.step("dashboard.saved_views", func() error {
		var err error
		personalViews, teamViews, err = loadDashboardSavedViews(s.root)
		return err
	}); err != nil {
		return DashboardViewData{}, err
	}
	return DashboardViewData{
		View:                  view,
		Summary:               dashboardSummary(tasks, displayTasks, workstreams, readySet),
		Gates:                 gates,
		GateGroups:            gateGroups,
		Groups:                groups,
		MissingReviewDomains:  missingReviewDomains,
		TableRows:             tableRows,
		Pagination:            pagination,
		Workstreams:           visibleWorkstreams,
		WorkstreamTotal:       len(workstreams),
		WorkstreamsExpanded:   workstreamsExpanded(filters),
		WorkstreamsShowAll:    boardWorkstreamsHref(filters, "all"),
		WorkstreamsCompact:    boardWorkstreamsHref(filters, ""),
		Filters:               filters,
		FilterOptions:         filterOptions(tasks, activity, s.cfg.Fairway.ProjectName),
		FilterChips:           boardFilterChips(filters),
		ClearFiltersHref:      boardClearFiltersHref(filters),
		BoardColumns:          boardColumns(filters),
		PersonalViews:         personalViews,
		TeamViews:             teamViews,
		CurrentViewQuery:      boardCurrentQuery(filters),
		Activity:              filteredActivity,
		ActivityTotal:         activityTotal,
		Health:                health,
		Sessions:              sessions,
		Worktrees:             s.worktrees,
		Checkpoints:           checkpoints,
		StaleCheckpoints:      staleCheckpoints,
		Watchers:              watchers,
		Rollups:               rollups,
		TaskRoles:             taskRoleMap(tasks),
		ActiveReport:          activeReport,
		CoordinatorPlan:       coordinatorPlan,
		Coordination:          dashboardCoordinationIntelligence(coordinatorPlan, memories, time.Now().UTC(), 24*time.Hour, s.cfg.States.Terminal),
		CloseoutReports:       closeoutReports,
		Audit:                 auditDiagnostics,
		DiagnosticsDeferred:   diagnosticsDeferred,
		GitProjectionDeferred: worktreeGitUnavailable(s.worktrees),
		GitDiagnostic:         worktreeGitDiagnostic(s.worktrees),
		DiagnosticPanel:       diagnosticPanel,
		ReadOnly:              s.cfg.Dashboard.ReadOnly,
		CSRFToken:             s.csrfToken,
		MutableStates:         dashboardMutableStates(s.cfg),
		Roles:                 append([]string(nil), s.roles...),
	}, nil
}

func dashboardSnapshotCacheable(r *http.Request) bool {
	return r != nil && r.Method == http.MethodGet && r.URL != nil
}

func dashboardSnapshotKey(prefix string, r *http.Request) string {
	if r == nil || r.URL == nil {
		return prefix
	}
	return prefix + ":" + r.URL.RequestURI()
}

func dashboardDiagnosticsPanelRequest(r *http.Request) bool {
	return r != nil && r.URL != nil && r.URL.Path == "/board/panels/diagnostics"
}

func (s *Server) dashboardCloseoutReports(ctx context.Context, tasks []store.Task, limit int, timing *dashboardTiming) ([]reconcile.CloseoutReport, error) {
	if limit <= 0 {
		return nil, nil
	}
	var reports []reconcile.CloseoutReport
	checked := 0
	stateCfg := state.Config{Allowed: s.cfg.States.Allowed, Terminal: s.cfg.States.Terminal, Transitions: s.cfg.States.Transitions}
	start := time.Now()
	defer func() {
		if timing != nil {
			timing.add("dashboard.closeout_reports.loop", time.Since(start), fmt.Sprintf("checked=%d reports=%d", checked, len(reports)))
		}
	}()
	for _, task := range tasks {
		if !state.IsTerminal(stateCfg, task.Status) {
			continue
		}
		checked++
		gitInfo := s.closeoutGitForTask(task)
		report, err := reconcile.Closeout(ctx, s.store, reconcile.CloseoutOptions{
			TaskID:   task.Definition.ID,
			Role:     task.Definition.Role,
			Terminal: s.cfg.States.Terminal,
			Git:      gitInfo,
		})
		if err != nil {
			return nil, err
		}
		if report.OK {
			continue
		}
		reports = append(reports, report)
		if len(reports) >= limit {
			break
		}
	}
	return reports, nil
}

func (s *Server) closeoutGitForTask(task store.Task) reconcile.CloseoutGit {
	branch := strings.TrimSpace(task.Branch)
	if branch == "" {
		branch = worktreeBranchForRole(s.worktrees, task.Definition.Role)
	}
	worktreePath, worktreeDirty := worktreeStateForRole(s.worktrees, task.Definition.Role)
	info := reconcile.CloseoutGit{
		Branch:        branch,
		Base:          s.cfg.Fairway.MainBranch,
		WorktreePath:  worktreePath,
		WorktreeDirty: worktreeDirty,
	}
	if worktreeGitUnavailable(s.worktrees) {
		return info
	}
	if s.root != "" && branch != "" {
		info.BranchExists = fairwaygit.BranchExists(s.root, branch)
		info.BranchMerged = fairwaygit.BranchMerged(s.root, branch, s.cfg.Fairway.MainBranch)
		info.RemoteBranchExists = branch != s.cfg.Fairway.MainBranch && fairwaygit.RemoteBranchExists(s.root, branch)
	}
	return info
}

func worktreeGitUnavailable(worktrees []WorktreeStatus) bool {
	for _, worktree := range worktrees {
		if worktree.GitUnavailable {
			return true
		}
	}
	return false
}

func worktreeGitDiagnostic(worktrees []WorktreeStatus) string {
	for _, worktree := range worktrees {
		if worktree.GitUnavailable {
			return worktree.Diagnostic
		}
	}
	return ""
}

func worktreeStateForRole(worktrees []WorktreeStatus, role string) (string, bool) {
	for _, worktree := range worktrees {
		if worktree.Role == role {
			return worktree.Path, worktree.Dirty
		}
	}
	return "", false
}

func worktreeBranchForRole(worktrees []WorktreeStatus, role string) string {
	for _, worktree := range worktrees {
		if worktree.Role == role {
			return worktree.Branch
		}
	}
	return ""
}

func dashboardWorktreeFacts(worktrees []WorktreeStatus) []coord.WorktreeFact {
	facts := make([]coord.WorktreeFact, 0, len(worktrees))
	for _, worktree := range worktrees {
		facts = append(facts, coord.WorktreeFact{
			Role:           worktree.Role,
			Branch:         worktree.Branch,
			Path:           worktree.Path,
			Registered:     worktree.Registered,
			Exists:         worktree.Exists,
			Dirty:          worktree.Dirty,
			LastCommit:     worktree.LastCommit,
			GitUnavailable: worktree.GitUnavailable,
			Diagnostic:     worktree.Diagnostic,
		})
	}
	return facts
}

func (s *MultiServer) dashboardViewData(r *http.Request) (DashboardViewData, error) {
	filters := taskFiltersFromRequest(r)
	tasks, sessions, checkpoints, watchers, activity, err := s.projectFacts(r.Context(), filters)
	if err != nil {
		return DashboardViewData{}, err
	}
	filteredActivity, activityTotal := filterActivity(activity, filters.ActivityKind, filters.ActivityLimit)
	readySet := map[string]bool{}
	displayTasks := filterTasks(tasks, filters, "")
	roles := rolesFromTasks(tasks)
	rollups := taskRollups(tasks, map[string]bool{"done": true})
	workstreams := groupWorkstreams(displayTasks, readySet)
	visibleWorkstreams := visibleWorkstreams(workstreams, filters)
	groups := groupTasks(displayTasks, roles)
	projectGroups := projectWallGroups(displayTasks, roles)
	tableSource := append([]store.Task(nil), displayTasks...)
	sortBoardRows(tableSource, filters)
	tableRows, pagination := paginateBoardRows(tableSource, filters)
	personalViews, teamViews, err := loadDashboardSavedViews("")
	if err != nil {
		return DashboardViewData{}, err
	}
	return DashboardViewData{
		View:                 "board",
		MultiProject:         true,
		Summary:              dashboardSummary(tasks, displayTasks, workstreams, readySet),
		Groups:               groups,
		ProjectGroups:        projectGroups,
		MissingReviewDomains: map[string][]string{},
		TableRows:            tableRows,
		Pagination:           pagination,
		Workstreams:          visibleWorkstreams,
		WorkstreamTotal:      len(workstreams),
		WorkstreamsExpanded:  workstreamsExpanded(filters),
		WorkstreamsShowAll:   boardWorkstreamsHref(filters, "all"),
		WorkstreamsCompact:   boardWorkstreamsHref(filters, ""),
		Filters:              filters,
		FilterOptions:        filterOptions(tasks, activity, ""),
		FilterChips:          boardFilterChips(filters),
		ClearFiltersHref:     boardClearFiltersHref(filters),
		BoardColumns:         boardColumns(filters),
		PersonalViews:        personalViews,
		TeamViews:            teamViews,
		CurrentViewQuery:     boardCurrentQuery(filters),
		Activity:             filteredActivity,
		ActivityTotal:        activityTotal,
		Sessions:             sessions,
		Checkpoints:          checkpoints,
		Watchers:             watchers,
		Rollups:              rollups,
		TaskRoles:            taskRoleMap(tasks),
		CoordinatorPlan: coord.Plan{
			DryRun: true,
			Summary: coord.PlanSummary{
				TopClassification: "multi-project",
				TopReason:         "open an individual project for mutable orchestration recommendations",
			},
		},
		Coordination:  CoordinationIntelligence{},
		ReadOnly:      true,
		CSRFToken:     s.csrfToken,
		MutableStates: []string{"todo", "claimed", "in_progress", "blocked", "review"},
		Roles:         roles,
	}, nil
}

func (s *MultiServer) projectFacts(ctx context.Context, filters TaskFilters) ([]store.Task, []store.Session, []store.Checkpoint, []store.Watcher, []store.Activity, error) {
	var tasks []store.Task
	var sessions []store.Session
	var checkpoints []store.Checkpoint
	var watchers []store.Watcher
	var activity []store.Activity
	for _, project := range s.projects {
		if project.Store == nil || project.Error != "" {
			continue
		}
		projectTasks, err := project.Store.AllTasks(ctx)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("%s tasks: %w", project.Name, err)
		}
		tasks = append(tasks, tagTasksProject(projectTasks, project.Name)...)
		if projectSessions, err := project.Store.Sessions(ctx, false); err == nil {
			sessions = append(sessions, tagSessionsProject(projectSessions, project.Name)...)
		}
		if projectCheckpoints, err := project.Store.Checkpoints(ctx, "", false); err == nil {
			checkpoints = append(checkpoints, projectCheckpoints...)
		}
		if projectWatchers, err := project.Store.Watchers(ctx, false); err == nil {
			watchers = append(watchers, projectWatchers...)
		}
		if projectActivity, err := project.Store.ActivityFiltered(ctx, store.ActivityOptions{Limit: maxActivityFetchLimit, Kind: filters.ActivityKind}); err == nil {
			activity = append(activity, tagActivityProject(projectActivity, project.Name)...)
		}
	}
	sort.SliceStable(activity, func(i, j int) bool {
		return activity[i].CreatedAt > activity[j].CreatedAt
	})
	if len(activity) > maxActivityFetchLimit {
		activity = activity[:maxActivityFetchLimit]
	}
	return tasks, sessions, checkpoints, watchers, activity, nil
}

const (
	defaultActivityLimit   = 25
	defaultTableLimit      = 25
	defaultWorkstreamLimit = 12
	maxActivityFetchLimit  = 500
	maxActivityLimit       = 200
	maxTableLimit          = 200
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
		if task.Status == "in_progress" && (groups[index].Current == nil || dashboardTaskMoreRecent(task, *groups[index].Current)) {
			copy := task
			groups[index].Current = &copy
		}
	}
	return groups
}

func projectWallGroups(tasks []store.Task, roles []string) []ProjectWallGroup {
	byProject := map[string][]store.Task{}
	for _, task := range tasks {
		project := taskProject(task, "")
		if project == "" {
			project = "current"
		}
		byProject[project] = append(byProject[project], task)
	}
	projects := make([]string, 0, len(byProject))
	for project := range byProject {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	out := make([]ProjectWallGroup, 0, len(projects))
	for _, project := range projects {
		projectTasks := byProject[project]
		readySet := map[string]bool{}
		workstreams := groupWorkstreams(projectTasks, readySet)
		out = append(out, ProjectWallGroup{
			Name:    project,
			Groups:  groupTasks(projectTasks, roles),
			Summary: dashboardSummary(projectTasks, projectTasks, workstreams, readySet),
		})
	}
	return out
}

func taskFiltersFromRequest(r *http.Request) TaskFilters {
	query := r.URL.Query()
	statuses := trimmedQueryValues(query["status"])
	tags := splitQueryValues(query["tag"])
	return TaskFilters{
		Search:        strings.TrimSpace(query.Get("q")),
		Role:          strings.TrimSpace(query.Get("role")),
		Status:        strings.Join(statuses, ", "),
		Statuses:      statuses,
		Sort:          normalizeBoardSort(query.Get("sort")),
		Columns:       normalizeBoardColumns(query.Get("columns")),
		Profile:       strings.TrimSpace(query.Get("profile")),
		Kind:          strings.TrimSpace(query.Get("kind")),
		OwningDomain:  strings.TrimSpace(query.Get("owning_domain")),
		RiskLevel:     strings.TrimSpace(query.Get("risk_level")),
		ReviewDomain:  strings.TrimSpace(query.Get("review_domain")),
		Tags:          tags,
		Project:       strings.TrimSpace(query.Get("project")),
		ActivityKind:  strings.TrimSpace(query.Get("activity_kind")),
		Tab:           dashboardTab(query.Get("tab")),
		ActivityLimit: boundedQueryInt(query.Get("activity_limit"), defaultActivityLimit, maxActivityLimit),
		TableLimit:    boundedQueryInt(query.Get("table_limit"), defaultTableLimit, maxTableLimit),
		TablePage:     boundedQueryInt(query.Get("page"), 1, 999999),
		Workstreams:   dashboardWorkstreamsMode(query.Get("workstreams")),
	}
}

func dashboardWorkstreamsMode(raw string) string {
	switch strings.TrimSpace(raw) {
	case "all":
		return "all"
	default:
		return ""
	}
}

func workstreamsExpanded(filters TaskFilters) bool {
	return filters.Workstreams == "all"
}

func dashboardTab(raw string) string {
	switch strings.TrimSpace(raw) {
	case "diagnostics":
		return "diagnostics"
	default:
		return "tasks"
	}
}

func boardTabHref(filters TaskFilters, tab string) string {
	values := url.Values{}
	if tab = dashboardTab(tab); tab == "diagnostics" {
		values.Set("tab", tab)
	}
	setIf := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	setIf("q", filters.Search)
	setIf("role", filters.Role)
	setIf("sort", filters.Sort)
	if columns := boardColumnsParam(filters.Columns); columns != "" {
		values.Set("columns", columns)
	}
	for _, status := range statusFilterValues(filters) {
		values.Add("status", status)
	}
	setIf("profile", filters.Profile)
	setIf("kind", filters.Kind)
	setIf("owning_domain", filters.OwningDomain)
	setIf("risk_level", filters.RiskLevel)
	setIf("review_domain", filters.ReviewDomain)
	for _, tag := range filters.Tags {
		values.Add("tag", tag)
	}
	setIf("project", filters.Project)
	setIf("activity_kind", filters.ActivityKind)
	if filters.ActivityLimit > 0 && filters.ActivityLimit != defaultActivityLimit {
		values.Set("activity_limit", strconv.Itoa(filters.ActivityLimit))
	}
	if filters.TableLimit > 0 && filters.TableLimit != defaultTableLimit {
		values.Set("table_limit", strconv.Itoa(filters.TableLimit))
	}
	if filters.TablePage > 1 {
		values.Set("page", strconv.Itoa(filters.TablePage))
	}
	if filters.Workstreams == "all" {
		values.Set("workstreams", "all")
	}
	if encoded := values.Encode(); encoded != "" {
		return "/board?" + encoded
	}
	return "/board"
}

func boardWorkstreamsHref(filters TaskFilters, mode string) string {
	filters.Workstreams = dashboardWorkstreamsMode(mode)
	return boardTabHref(filters, filters.Tab)
}

func boardDiagnosticsPanelHref(filters TaskFilters) string {
	return boardDiagnosticsPanelHrefFor(filters, "")
}

func boardDiagnosticsPanelHrefFor(filters TaskFilters, panel string) string {
	href := boardTabHref(filters, "diagnostics")
	values := url.Values{}
	if strings.HasPrefix(href, "/board?") {
		values, _ = url.ParseQuery(strings.TrimPrefix(href, "/board?"))
	}
	values.Set("tab", "diagnostics")
	if panel != "" {
		values.Set("panel", panel)
	}
	return "/board/panels/diagnostics?" + values.Encode()
}

func boardPageHref(filters TaskFilters, page int) string {
	filters.TablePage = page
	return boardTabHref(filters, filters.Tab)
}

func boardCurrentQuery(filters TaskFilters) string {
	href := boardTabHref(filters, filters.Tab)
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	values := parsed.Query()
	values.Del("page")
	return values.Encode()
}

func boardExportHref(filters TaskFilters, format string) string {
	href := boardTabHref(filters, filters.Tab)
	parsed, err := url.Parse(href)
	if err != nil {
		return "/board/export?format=" + url.QueryEscape(format)
	}
	parsed.Path = "/board/export"
	values := parsed.Query()
	values.Set("format", format)
	parsed.RawQuery = values.Encode()
	return parsed.RequestURI()
}

func boardSortHref(filters TaskFilters, column string) string {
	column = normalizeBoardSortKey(column)
	current := firstBoardSort(filters.Sort)
	switch current {
	case column:
		filters.Sort = "-" + column
	case "-" + column:
		filters.Sort = strings.TrimSpace(strings.Join(secondaryBoardSorts(filters.Sort), ","))
	default:
		rest := secondaryBoardSorts(filters.Sort)
		if len(rest) > 0 {
			filters.Sort = column + "," + strings.Join(rest, ",")
		} else {
			filters.Sort = column
		}
	}
	filters.TablePage = 1
	return boardTabHref(filters, filters.Tab)
}

func boardSortState(filters TaskFilters, column string) string {
	column = normalizeBoardSortKey(column)
	switch firstBoardSort(filters.Sort) {
	case column:
		return "ascending"
	case "-" + column:
		return "descending"
	default:
		return ""
	}
}

func boardSortAria(filters TaskFilters, column string) string {
	state := boardSortState(filters, column)
	if state == "" {
		return "none"
	}
	return state
}

var boardColumnCatalog = []BoardColumn{
	{Key: "project", Label: "Project", Sortable: true},
	{Key: "id", Label: "ID", Sortable: true},
	{Key: "title", Label: "Title", Sortable: true},
	{Key: "role", Label: "Role", Sortable: true},
	{Key: "status", Label: "Status", Sortable: true},
	{Key: "kind", Label: "Kind", Sortable: true},
	{Key: "started", Label: "Started", Sortable: true},
	{Key: "updated", Label: "Last activity", Sortable: true},
	{Key: "gates", Label: "Gates", Sortable: true},
	{Key: "owner", Label: "Owner", Sortable: true},
	{Key: "profile", Label: "Profile", Sortable: true},
	{Key: "owning_domain", Label: "Domain", Sortable: true},
	{Key: "risk_level", Label: "Risk", Sortable: true},
	{Key: "review_domains", Label: "Review domains", Sortable: true},
	{Key: "tags", Label: "Tags", Sortable: true},
	{Key: "workstream", Label: "Workstream", Sortable: true},
}

var defaultBoardColumns = []string{"project", "id", "title", "role", "status", "kind", "started", "updated", "gates", "owner"}

func boardColumns(filters TaskFilters) []BoardColumn {
	visible := filters.Columns
	if len(visible) == 0 {
		visible = defaultBoardColumns
	}
	visibleSet := map[string]bool{}
	var out []BoardColumn
	for index, key := range visible {
		if column, ok := boardColumnByKey(key); ok {
			column.Visible = true
			column.ToggleHref = boardColumnToggleHref(filters, key)
			column.MoveUpHref = boardColumnMoveHref(filters, index, -1)
			column.MoveDownHref = boardColumnMoveHref(filters, index, 1)
			out = append(out, column)
			visibleSet[key] = true
		}
	}
	for _, column := range boardColumnCatalog {
		if visibleSet[column.Key] {
			continue
		}
		column.Visible = false
		column.ToggleHref = boardColumnToggleHref(filters, column.Key)
		out = append(out, column)
	}
	return out
}

func boardVisibleColumns(columns []BoardColumn) []BoardColumn {
	var out []BoardColumn
	for _, column := range columns {
		if column.Visible {
			out = append(out, column)
		}
	}
	return out
}

func boardColumnByKey(key string) (BoardColumn, bool) {
	for _, column := range boardColumnCatalog {
		if column.Key == key {
			return column, true
		}
	}
	return BoardColumn{}, false
}

func normalizeBoardColumns(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		key := strings.TrimSpace(part)
		if _, ok := boardColumnByKey(key); !ok || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func boardColumnsParam(columns []string) string {
	if len(columns) == 0 || stringSlicesEqual(columns, defaultBoardColumns) {
		return ""
	}
	return strings.Join(columns, ",")
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func boardColumnToggleHref(filters TaskFilters, key string) string {
	columns := filters.Columns
	if len(columns) == 0 {
		columns = append([]string(nil), defaultBoardColumns...)
	} else {
		columns = append([]string(nil), columns...)
	}
	var next []string
	removed := false
	for _, column := range columns {
		if column == key {
			removed = true
			continue
		}
		next = append(next, column)
	}
	if !removed {
		next = append(next, key)
	}
	if len(next) == 0 {
		next = append([]string(nil), defaultBoardColumns...)
	}
	filters.Columns = next
	filters.TablePage = 1
	return boardTabHref(filters, filters.Tab)
}

func boardColumnMoveHref(filters TaskFilters, index, delta int) string {
	columns := filters.Columns
	if len(columns) == 0 {
		columns = append([]string(nil), defaultBoardColumns...)
	} else {
		columns = append([]string(nil), columns...)
	}
	target := index + delta
	if index < 0 || index >= len(columns) || target < 0 || target >= len(columns) {
		return ""
	}
	columns[index], columns[target] = columns[target], columns[index]
	filters.Columns = columns
	filters.TablePage = 1
	return boardTabHref(filters, filters.Tab)
}

func boardColumnCount(columns []BoardColumn) int {
	count := 1
	for _, column := range columns {
		if column.Visible {
			count++
		}
	}
	return count
}

func firstBoardSort(raw string) string {
	parts := boardSortParts(raw)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func secondaryBoardSorts(raw string) []string {
	parts := boardSortParts(raw)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}

func boardSortParts(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		desc := strings.HasPrefix(part, "-")
		key := normalizeBoardSortKey(strings.TrimPrefix(part, "-"))
		if key == "" {
			continue
		}
		if desc {
			key = "-" + key
		}
		if !boardSortContainsKey(out, key) {
			out = append(out, key)
		}
	}
	return out
}

func boardSortContainsKey(parts []string, key string) bool {
	key = strings.TrimPrefix(key, "-")
	for _, part := range parts {
		if strings.TrimPrefix(part, "-") == key {
			return true
		}
	}
	return false
}

func normalizeBoardSort(raw string) string {
	return strings.Join(boardSortParts(raw), ",")
}

func normalizeBoardSortKey(raw string) string {
	switch strings.TrimSpace(raw) {
	case "project", "id", "title", "role", "status", "kind", "started", "updated", "gates", "owner", "profile", "owning_domain", "risk_level", "review_domains", "tags", "workstream":
		return strings.TrimSpace(raw)
	default:
		return ""
	}
}

func boardFilterChips(filters TaskFilters) []FilterChip {
	var chips []FilterChip
	add := func(key, label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		chips = append(chips, FilterChip{Key: key, Label: label, Value: value, Href: boardRemoveFilterHref(filters, key, value)})
	}
	add("q", "Search", filters.Search)
	add("role", "Role", filters.Role)
	for _, status := range statusFilterValues(filters) {
		add("status", "Status", status)
	}
	add("profile", "Profile", filters.Profile)
	add("kind", "Kind", filters.Kind)
	add("owning_domain", "Domain", filters.OwningDomain)
	add("risk_level", "Risk", filters.RiskLevel)
	add("review_domain", "Review", filters.ReviewDomain)
	for _, tag := range filters.Tags {
		add("tag", "Tag", tag)
	}
	add("project", "Project", filters.Project)
	return chips
}

func boardRemoveFilterHref(filters TaskFilters, key, value string) string {
	filters.TablePage = 1
	switch key {
	case "q":
		filters.Search = ""
	case "role":
		filters.Role = ""
	case "status":
		var statuses []string
		for _, status := range statusFilterValues(filters) {
			if status != value {
				statuses = append(statuses, status)
			}
		}
		filters.Statuses = statuses
		filters.Status = strings.Join(statuses, ", ")
	case "profile":
		filters.Profile = ""
	case "kind":
		filters.Kind = ""
	case "owning_domain":
		filters.OwningDomain = ""
	case "risk_level":
		filters.RiskLevel = ""
	case "review_domain":
		filters.ReviewDomain = ""
	case "tag":
		var tags []string
		for _, tag := range filters.Tags {
			if tag != value {
				tags = append(tags, tag)
			}
		}
		filters.Tags = tags
	case "project":
		filters.Project = ""
	}
	return boardTabHref(filters, filters.Tab)
}

func boardClearFiltersHref(filters TaskFilters) string {
	filters.Search = ""
	filters.Role = ""
	filters.Status = ""
	filters.Statuses = nil
	filters.Profile = ""
	filters.Kind = ""
	filters.OwningDomain = ""
	filters.RiskLevel = ""
	filters.ReviewDomain = ""
	filters.Tags = nil
	filters.Project = ""
	filters.TablePage = 1
	return boardTabHref(filters, filters.Tab)
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

func filterTasks(tasks []store.Task, filters TaskFilters, projectName string) []store.Task {
	statuses := statusFilterValues(filters)
	var out []store.Task
	for _, task := range tasks {
		if filters.Project != "" && filters.Project != taskProject(task, projectName) {
			continue
		}
		if filters.Search != "" && !taskMatchesSearch(task, filters.Search) {
			continue
		}
		if filters.Role != "" && task.Definition.Role != filters.Role {
			continue
		}
		if len(statuses) > 0 && !containsString(statuses, task.Status) {
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
		if len(filters.Tags) > 0 && !containsAllStrings(task.Definition.Tags, filters.Tags) {
			continue
		}
		out = append(out, task)
	}
	return out
}

func sortBoardRows(rows []store.Task, filters TaskFilters) {
	parts := boardSortParts(filters.Sort)
	if len(parts) == 0 {
		sort.SliceStable(rows, func(i, j int) bool {
			return dashboardTaskMoreRecent(rows[i], rows[j])
		})
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		for _, part := range parts {
			desc := strings.HasPrefix(part, "-")
			key := strings.TrimPrefix(part, "-")
			cmp := compareBoardTask(rows[i], rows[j], key)
			if cmp == 0 {
				continue
			}
			if desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return rows[i].Definition.ID < rows[j].Definition.ID
	})
}

func compareBoardTask(left, right store.Task, key string) int {
	switch key {
	case "project":
		return strings.Compare(taskProject(left, ""), taskProject(right, ""))
	case "id":
		return strings.Compare(left.Definition.ID, right.Definition.ID)
	case "title":
		return strings.Compare(strings.ToLower(left.Definition.Title), strings.ToLower(right.Definition.Title))
	case "role":
		return strings.Compare(left.Definition.Role, right.Definition.Role)
	case "status":
		return strings.Compare(left.Status, right.Status)
	case "kind":
		return strings.Compare(left.Definition.Kind, right.Definition.Kind)
	case "started", "updated":
		return strings.Compare(left.UpdatedAt, right.UpdatedAt)
	case "gates":
		return 0
	case "owner":
		return strings.Compare(left.Owner, right.Owner)
	case "profile":
		return strings.Compare(left.Definition.Profile, right.Definition.Profile)
	case "owning_domain":
		return strings.Compare(left.Definition.OwningDomain, right.Definition.OwningDomain)
	case "risk_level":
		return strings.Compare(left.Definition.RiskLevel, right.Definition.RiskLevel)
	case "review_domains":
		return strings.Compare(strings.Join(left.Definition.ReviewDomains, ","), strings.Join(right.Definition.ReviewDomains, ","))
	case "tags":
		return strings.Compare(strings.Join(left.Definition.Tags, ","), strings.Join(right.Definition.Tags, ","))
	case "workstream":
		return strings.Compare(boardTaskWorkstream(left), boardTaskWorkstream(right))
	default:
		return strings.Compare(left.Definition.ID, right.Definition.ID)
	}
}

func boardTaskCell(task store.Task, column BoardColumn, rollups map[string]Rollup) template.HTML {
	escape := html.EscapeString
	switch column.Key {
	case "project":
		return template.HTML(escape(taskProject(task, "")))
	case "id":
		id := escape(task.Definition.ID)
		return template.HTML(`<a href="/tasks/` + url.PathEscape(task.Definition.ID) + `">` + id + `</a>`)
	case "title":
		return template.HTML(escape(task.Definition.Title))
	case "role":
		return template.HTML(escape(task.Definition.Role))
	case "status":
		status := escape(task.Status)
		return template.HTML(`<span class="status-pill ` + escape(safeDashboardClass(task.Status)) + `">` + status + `</span>`)
	case "kind":
		return template.HTML(escape(task.Definition.Kind))
	case "started", "updated":
		return template.HTML(escape(task.UpdatedAt))
	case "gates":
		if rollup, ok := rollups[task.Definition.ID]; ok {
			return template.HTML(fmt.Sprintf("%d/%d", rollup.Done, rollup.Total))
		}
		return "-"
	case "owner":
		return template.HTML(escape(task.Owner))
	case "profile":
		return template.HTML(escape(task.Definition.Profile))
	case "owning_domain":
		return template.HTML(escape(task.Definition.OwningDomain))
	case "risk_level":
		return template.HTML(escape(task.Definition.RiskLevel))
	case "review_domains":
		return template.HTML(escape(strings.Join(task.Definition.ReviewDomains, ", ")))
	case "tags":
		return template.HTML(escape(strings.Join(task.Definition.Tags, ", ")))
	case "workstream":
		return template.HTML(escape(boardTaskWorkstream(task)))
	default:
		return ""
	}
}

func boardTaskPlainCell(task store.Task, column BoardColumn, rollups map[string]Rollup) string {
	switch column.Key {
	case "project":
		return taskProject(task, "")
	case "id":
		return task.Definition.ID
	case "title":
		return task.Definition.Title
	case "role":
		return task.Definition.Role
	case "status":
		return task.Status
	case "kind":
		return task.Definition.Kind
	case "started", "updated":
		return task.UpdatedAt
	case "gates":
		if rollup, ok := rollups[task.Definition.ID]; ok {
			return fmt.Sprintf("%d/%d", rollup.Done, rollup.Total)
		}
		return "-"
	case "owner":
		return task.Owner
	case "profile":
		return task.Definition.Profile
	case "owning_domain":
		return task.Definition.OwningDomain
	case "risk_level":
		return task.Definition.RiskLevel
	case "review_domains":
		return strings.Join(task.Definition.ReviewDomains, ", ")
	case "tags":
		return strings.Join(task.Definition.Tags, ", ")
	case "workstream":
		return boardTaskWorkstream(task)
	default:
		return ""
	}
}

func boardTaskWorkstream(task store.Task) string {
	profile := strings.TrimSpace(task.Definition.Profile)
	kind := strings.TrimSpace(task.Definition.Kind)
	switch {
	case profile != "" && kind != "":
		return profile + " / " + kind
	case profile != "":
		return profile
	default:
		return kind
	}
}

func taskProject(task store.Task, fallback string) string {
	if strings.TrimSpace(task.Project) != "" {
		return task.Project
	}
	return strings.TrimSpace(fallback)
}

func tagTasksProject(tasks []store.Task, project string) []store.Task {
	project = strings.TrimSpace(project)
	if project == "" {
		return tasks
	}
	out := append([]store.Task(nil), tasks...)
	for i := range out {
		out[i].Project = project
	}
	return out
}

func tagSessionsProject(sessions []store.Session, project string) []store.Session {
	project = strings.TrimSpace(project)
	if project == "" {
		return sessions
	}
	out := append([]store.Session(nil), sessions...)
	for i := range out {
		lane := strings.TrimSpace(out[i].Lane)
		switch {
		case lane == "":
			out[i].Lane = project
		case strings.HasPrefix(lane, project+"/"):
			out[i].Lane = lane
		default:
			out[i].Lane = project + "/" + lane
		}
	}
	return out
}

func tagActivityProject(activity []store.Activity, project string) []store.Activity {
	project = strings.TrimSpace(project)
	if project == "" {
		return activity
	}
	out := append([]store.Activity(nil), activity...)
	for i := range out {
		out[i].Summary = "[" + project + "] " + out[i].Summary
		if out[i].Actor != "" {
			out[i].Actor = project + "/" + out[i].Actor
		} else {
			out[i].Actor = project
		}
	}
	return out
}

func rolesFromTasks(tasks []store.Task) []string {
	seen := map[string]bool{}
	var roles []string
	for _, task := range tasks {
		role := strings.TrimSpace(task.Definition.Role)
		if role == "" || seen[role] {
			continue
		}
		seen[role] = true
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func trimmedQueryValues(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func splitQueryValues(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}

func statusFilterValues(filters TaskFilters) []string {
	if len(filters.Statuses) > 0 {
		return filters.Statuses
	}
	if strings.TrimSpace(filters.Status) == "" {
		return nil
	}
	return []string{strings.TrimSpace(filters.Status)}
}

func statusSelected(filters TaskFilters, status string) bool {
	return containsString(statusFilterValues(filters), status)
}

func wallRoleHref(role string) string {
	values := url.Values{}
	values.Set("role", role)
	return "/board?" + values.Encode()
}

func wallProjectRoleHref(project, role string) string {
	values := url.Values{}
	if strings.TrimSpace(project) != "" {
		values.Set("project", project)
	}
	if strings.TrimSpace(role) != "" {
		values.Set("role", role)
	}
	if encoded := values.Encode(); encoded != "" {
		return "/board?" + encoded
	}
	return "/board"
}

func wallLaneHref(role, lane string) string {
	values := url.Values{}
	values.Set("role", role)
	switch lane {
	case "backlog":
		values.Set("status", "todo")
	case "claimed", "working":
		values.Set("status", "in_progress")
	case "done":
		values.Set("status", "done")
	}
	return "/board?" + values.Encode()
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
	haystacks = append(haystacks, task.Definition.Tags...)
	for _, value := range haystacks {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func filterOptions(tasks []store.Task, activity []store.Activity, projectName string) FilterOptions {
	var options FilterOptions
	statuses := map[string]bool{}
	projects := map[string]bool{}
	profiles := map[string]bool{}
	kinds := map[string]bool{}
	domains := map[string]bool{}
	risks := map[string]bool{}
	reviewDomains := map[string]bool{}
	tags := map[string]bool{}
	activityKinds := map[string]bool{}
	for _, task := range tasks {
		addFilterValue(projects, taskProject(task, projectName))
		addFilterValue(statuses, task.Status)
		addFilterValue(profiles, task.Definition.Profile)
		addFilterValue(kinds, task.Definition.Kind)
		addFilterValue(domains, task.Definition.OwningDomain)
		addFilterValue(risks, task.Definition.RiskLevel)
		for _, domain := range task.Definition.ReviewDomains {
			addFilterValue(reviewDomains, domain)
		}
		for _, tag := range task.Definition.Tags {
			addFilterValue(tags, tag)
		}
	}
	for _, item := range activity {
		addFilterValue(activityKinds, item.Kind)
	}
	options.Statuses = sortedFilterValues(statuses)
	options.Projects = sortedFilterValues(projects)
	options.Profiles = sortedFilterValues(profiles)
	options.Kinds = sortedFilterValues(kinds)
	options.OwningDomains = sortedFilterValues(domains)
	options.RiskLevels = sortedFilterValues(risks)
	options.ReviewDomains = sortedFilterValues(reviewDomains)
	options.Tags = sortedFilterValues(tags)
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

func containsAllStrings(values []string, wants []string) bool {
	for _, want := range wants {
		if !containsString(values, want) {
			return false
		}
	}
	return true
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
	sort.SliceStable(groups, func(i, j int) bool {
		leftActionable := groups[i].InProgress + groups[i].Ready + groups[i].Blocked
		rightActionable := groups[j].InProgress + groups[j].Ready + groups[j].Blocked
		if leftActionable != rightActionable {
			return leftActionable > rightActionable
		}
		if groups[i].InProgress != groups[j].InProgress {
			return groups[i].InProgress > groups[j].InProgress
		}
		if groups[i].Ready != groups[j].Ready {
			return groups[i].Ready > groups[j].Ready
		}
		if groups[i].Blocked != groups[j].Blocked {
			return groups[i].Blocked > groups[j].Blocked
		}
		if groups[i].Total != groups[j].Total {
			return groups[i].Total > groups[j].Total
		}
		return groups[i].Label < groups[j].Label
	})
	return groups
}

func visibleWorkstreams(groups []WorkstreamGroup, filters TaskFilters) []WorkstreamGroup {
	if workstreamsExpanded(filters) || len(groups) <= defaultWorkstreamLimit {
		return groups
	}
	return groups[:defaultWorkstreamLimit]
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

func paginateBoardRows(rows []store.Task, filters TaskFilters) ([]store.Task, TablePagination) {
	total := len(rows)
	pageSize := filters.TableLimit
	if pageSize <= 0 {
		pageSize = defaultTableLimit
	}
	if pageSize > maxTableLimit {
		pageSize = maxTableLimit
	}
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
	startIndex := 0
	endIndex := 0
	displayStart := 0
	displayEnd := 0
	if total > 0 {
		startIndex = (page - 1) * pageSize
		endIndex = startIndex + pageSize
		if endIndex > total {
			endIndex = total
		}
		displayStart = startIndex + 1
		displayEnd = endIndex
	}
	pagination := TablePagination{
		Page:        page,
		PageSize:    pageSize,
		TotalRows:   total,
		TotalPages:  totalPages,
		Start:       displayStart,
		End:         displayEnd,
		Virtualized: total > maxTableLimit,
		WindowSize:  pageSize,
	}
	if page > 1 {
		pagination.PrevHref = boardPageHref(filters, page-1)
	}
	if page < totalPages {
		pagination.NextHref = boardPageHref(filters, page+1)
	}
	return rows[startIndex:endIndex], pagination
}

func (s *Server) dashboardGateStatuses(ctx context.Context, tasks []store.Task, gapLimit int, timing *dashboardTiming) ([]GateStatus, error) {
	var statuses []GateStatus
	now := time.Now().UTC()
	var taskIDs []string
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	start := time.Now()
	evidenceByTask, err := s.store.EvidenceByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	if timing != nil {
		timing.add("dashboard.gates.batch_evidence", time.Since(start), fmt.Sprintf("tasks=%d", len(taskIDs)))
	}
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
				evidence := evidenceByTask[task.Definition.ID]
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

func (s *Server) taskGateStatuses(task store.Task, evidence []store.Evidence) []TaskGateStatus {
	var statuses []TaskGateStatus
	now := time.Now().UTC()
	for _, profile := range s.cfg.WorkstreamProfiles {
		if len(profile.Gates) == 0 || !profileAppliesToTask(profile, task) {
			continue
		}
		for _, gate := range profile.Gates {
			if !gateAppliesToTask(gate, task) {
				continue
			}
			ok, matching, reasons := evaluateGateForTask(gate, evidence, now)
			status := TaskGateStatus{
				Profile:      profile.Name,
				Name:         gate.Name,
				Mode:         gate.Mode,
				EvidenceType: gate.EvidenceType,
				Description:  gate.Description,
				Status:       "satisfied",
				Matching:     matching,
			}
			if !ok {
				status.Status = "missing"
				status.Reasons = reasons
			}
			statuses = append(statuses, status)
		}
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		if statuses[i].Profile != statuses[j].Profile {
			return statuses[i].Profile < statuses[j].Profile
		}
		return statuses[i].Name < statuses[j].Name
	})
	return statuses
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
	eval := evidencemodel.EvaluateGate(gate, evidence, now)
	return eval.Satisfied, eval.Matching, eval.Reasons
}

func (s *Server) task(w http.ResponseWriter, r *http.Request) {
	timing := newDashboardTiming("task-detail", r)
	defer timing.logIfSlow()
	id := r.URL.Path[len("/tasks/"):]
	start := time.Now()
	task, transitions, evidence, handoffs, reviews, err := s.store.TaskDetail(r.Context(), id)
	timing.add("task_detail.core", time.Since(start), fmt.Sprintf("transitions=%d evidence=%d handoffs=%d reviews=%d", len(transitions), len(evidence), len(handoffs), len(reviews)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	start = time.Now()
	sessions, err := s.store.Sessions(r.Context(), false)
	timing.add("task_detail.sessions", time.Since(start), fmt.Sprintf("rows=%d", len(sessions)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start = time.Now()
	tasks, err := s.store.AllTasks(r.Context())
	timing.add("task_detail.tasks", time.Since(start), fmt.Sprintf("rows=%d", len(tasks)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rollups := taskRollups(tasks, map[string]bool{"done": true})
	start = time.Now()
	activity, err := s.store.Activity(r.Context(), defaultActivityLimit)
	timing.add("task_detail.activity", time.Since(start), fmt.Sprintf("rows=%d", len(activity)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start = time.Now()
	usageEvents, err := s.store.ProviderUsageForTask(r.Context(), id)
	timing.add("task_detail.usage", time.Since(start), fmt.Sprintf("rows=%d", len(usageEvents)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start = time.Now()
	usageRollups, err := s.store.UsageRollups(r.Context(), store.UsageRollupOptions{GroupBy: "provider", TaskID: id})
	timing.add("task_detail.usage_rollups", time.Since(start), fmt.Sprintf("rows=%d", len(usageRollups)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start = time.Now()
	batches, err := dashboardBatchesForTask(r.Context(), s.store, id)
	timing.add("task_detail.batches", time.Since(start), fmt.Sprintf("rows=%d", len(batches)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start = time.Now()
	notifications, err := s.store.Notifications(r.Context(), id)
	timing.add("task_detail.notifications", time.Since(start), fmt.Sprintf("rows=%d", len(notifications)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start = time.Now()
	activeReport, err := reconcile.Active(r.Context(), s.store, reconcile.ActiveOptions{Terminal: s.cfg.States.Terminal, StaleCheckpointAfter: 2 * time.Hour})
	timing.add("task_detail.active_reconcile", time.Since(start), fmt.Sprintf("findings=%d", len(activeReport.Findings)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start = time.Now()
	reviewPolicy, err := s.dashboardReviewPolicyEvaluation(r.Context(), task, reviews)
	timing.add("task_detail.review_policy", time.Since(start), fmt.Sprintf("missing=%d", len(reviewPolicy.MissingReviewDomains)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	missingReviewDomains := reviewPolicy.MissingReviewDomains
	uxMediaEvidence := evidencemodel.UXMediaRows(evidence)
	uxMediaSummary := evidencemodel.UXMediaSummaryFor(uxMediaEvidence)
	reviewHandback, hasReviewHandback := coord.ReviewHandbackForTask(s.cfg, task, evidence, handoffs, reviews, coord.ReviewHandbackOptions{IncludeHistorical: true, Notifications: notifications})
	reviewNotifications := reviewstate.StatusesForTask(task, handoffs, reviews, notifications)
	reviewWaitOptions, err := s.reviewWaitOptions(time.Now().UTC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reviewWaits := reviewstate.WaitsForTask(task, handoffs, reviews, notifications, reviewWaitOptions)
	start = time.Now()
	completionHandbacks, completionActions, err := s.completionHandbackProjection(r.Context(), task, evidence, handoffs, notifications, reviewWaitOptions.AckTimeout)
	timing.add("task_detail.completion_handback", time.Since(start), fmt.Sprintf("handbacks=%d actions=%d", len(completionHandbacks), len(completionActions)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start = time.Now()
	decisions, err := s.store.TaskDecisions(r.Context(), id)
	timing.add("task_detail.decisions", time.Since(start), fmt.Sprintf("rows=%d", len(decisions)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	decisionAttention := 0
	for _, decision := range decisions {
		if decision.QualityState == "superseded" {
			continue
		}
		if decision.QualityState == "insufficient" || (taskDecisionAcceptanceRequired(task) && decision.QualityState != "accepted") {
			decisionAttention++
		}
	}
	taskSessions := sessionsForDashboardTask(sessions, id)
	recommendation := RecommendCommonPath(CommonPathInput{
		Task:                 task,
		ActiveSessions:       len(taskSessions),
		EvidenceCount:        len(evidence),
		MissingReviewDomains: missingReviewDomains,
		ReviewMode:           reviewPolicy.Mode,
		ActiveFindings:       activeFindingsForTask(activeReport.Findings, id),
		DecisionAttention:    decisionAttention,
	})
	start = time.Now()
	auditDiagnostics := s.auditDiagnostics(r.Context(), id)
	timing.add("task_detail.audit", time.Since(start), "task_scoped=true")
	start = time.Now()
	qualityRecord, err := qualityrecord.Build(r.Context(), s.store, id, time.Now().UTC())
	timing.add("task_detail.quality_record", time.Since(start), fmt.Sprintf("sections=%d", len(qualityRecord.Sections)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := TaskDetailViewData{
		View:                 "detail",
		Groups:               groupTasks(tasks, s.roles),
		Sessions:             sessions,
		Activity:             activity,
		BackHref:             localBackHref(r.Referer(), r.Host, task.Definition.Role),
		Task:                 task,
		Transitions:          transitions,
		Evidence:             evidence,
		EvidenceViews:        evidenceViewRows(task.Definition.ID, evidence, s.cfg.Fairway.LocalArtifactPaths),
		UXMediaEvidence:      uxMediaEvidence,
		UXMediaSummary:       uxMediaSummary,
		Handoffs:             handoffs,
		Reviews:              reviews,
		MissingReviewDomains: missingReviewDomains,
		ReviewPolicy:         reviewPolicy,
		ReviewStatus:         dashboardEffectiveReviewStatus(task.ReviewStatus, missingReviewDomains),
		ReviewHandback:       optionalDashboardReviewHandback(reviewHandback, hasReviewHandback),
		ReviewNotifications:  reviewNotifications,
		ReviewWaits:          reviewWaits,
		CompletionHandbacks:  completionHandbacks,
		CompletionActions:    completionActions,
		TaskSessions:         taskSessions,
		Usage:                usageEvents,
		UsageRollups:         usageRollups,
		Batches:              batches,
		TaskGates:            s.taskGateStatuses(task, evidence),
		TaskRules:            s.taskRuleStatuses(r.Context(), task, evidence),
		Rollup:               rollups[task.Definition.ID],
		CSRFToken:            s.csrfToken,
		States:               dashboardMutableStates(s.cfg),
		Audit:                auditDiagnostics,
		ActiveFindings:       activeFindingsForTask(activeReport.Findings, id),
		ReadOnly:             s.cfg.Dashboard.ReadOnly,
		Recommendation:       recommendation,
		QualityRecord:        qualityRecord,
	}
	start = time.Now()
	_ = detailTemplate.ExecuteTemplate(w, "layout", data)
	timing.add("template.task_detail", time.Since(start), "")
}

func (s *Server) completionHandbackProjection(ctx context.Context, task store.Task, evidence []store.Evidence, handoffs []store.Handoff, notifications []store.Notification, ackTimeout time.Duration) ([]completionhandback.Handback, []coord.PlanAction, error) {
	var liveWindowStatus livewindow.Status
	checkpoints, err := s.store.Checkpoints(ctx, "", true)
	if err != nil {
		return nil, nil, err
	}
	for _, status := range livewindow.StatusesFromCheckpoints(checkpoints) {
		if status.TaskID == task.Definition.ID {
			liveWindowStatus = status
			break
		}
	}
	handbacks := completionhandback.RowsWithOptions(task.Definition.ID, handoffs, notifications, completionhandback.RowOptions{
		Now:             time.Now().UTC(),
		AckTimeout:      ackTimeout,
		TaskStatus:      task.Status,
		LiveWindowPhase: liveWindowStatus.Phase,
		Superseded:      completionhandback.SupersedesFromEvidence(evidence),
	})
	actions := coord.CompletionHandbackActionsForTask(task, handbacks, liveWindowStatus, ackTimeout, time.Now().UTC())
	if len(actions) > taskDetailCompletionActionLimit {
		actions = actions[:taskDetailCompletionActionLimit]
	}
	return handbacks, actions, nil
}

func (s *Server) reviewWaitOptions(now time.Time) (reviewstate.ReviewWaitOptions, error) {
	ackTimeout := 24 * time.Hour
	if raw := strings.TrimSpace(s.cfg.Coordinator.NotificationAckTimeout); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return reviewstate.ReviewWaitOptions{}, fmt.Errorf("invalid coordinator notification_ack_timeout %q: %w", raw, err)
		}
		ackTimeout = parsed
	}
	return reviewstate.ReviewWaitOptions{
		ProviderTargets: s.cfg.ProviderTargets,
		ReviewRoutes:    s.cfg.ReviewRoutes,
		Roles:           s.cfg.Roles,
		AckTimeout:      ackTimeout,
		Now:             now,
		Terminal:        s.cfg.States.Terminal,
	}, nil
}

func (s *Server) reviewWaits(ctx context.Context, taskID string, now time.Time) ([]reviewstate.ReviewWait, error) {
	opts, err := s.reviewWaitOptions(now)
	if err != nil {
		return nil, err
	}
	var tasks []store.Task
	if strings.TrimSpace(taskID) != "" {
		task, _, _, _, _, err := s.store.TaskDetail(ctx, taskID)
		if err != nil {
			return nil, err
		}
		tasks = []store.Task{task}
	} else {
		var err error
		tasks, err = s.store.AllTasks(ctx)
		if err != nil {
			return nil, err
		}
	}
	var waits []reviewstate.ReviewWait
	for _, task := range tasks {
		detailTask, _, _, handoffs, reviews, err := s.store.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return nil, err
		}
		notifications, err := s.store.Notifications(ctx, task.Definition.ID)
		if err != nil {
			return nil, err
		}
		waits = append(waits, reviewstate.WaitsForTask(detailTask, handoffs, reviews, notifications, opts)...)
	}
	return waits, nil
}

func (s *Server) auditDiagnostics(ctx context.Context, taskID string) AuditDiagnostics {
	diagnostics := AuditDiagnostics{}
	if s.root == "" {
		diagnostics.WorkCoverageError = "repo root unavailable; start dashboard through fairway dashboard for commit coverage"
	} else {
		report, err := audit.BuildWorkCoverageReport(ctx, s.cfg, s.root, s.store, audit.WorkCoverageOptions{SinceDuration: 24 * time.Hour, TaskID: taskID})
		if err != nil {
			diagnostics.WorkCoverageError = err.Error()
		} else {
			diagnostics.WorkCoverage = report
			diagnostics.WorkCoverageAvailable = true
		}
	}
	report, err := audit.BuildCILearningReport(ctx, s.cfg, s.store, audit.CILearningOptions{TaskID: taskID})
	if err != nil {
		diagnostics.CILearningError = err.Error()
	} else {
		diagnostics.CILearning = report
	}
	return diagnostics
}

func dashboardMissingApprovedReviewDomains(domains []string, reviews []store.Review) []string {
	if len(domains) == 0 {
		return nil
	}
	approved := map[string]bool{}
	for _, review := range reviews {
		if review.Verdict == "approve" {
			approved[firstNonEmpty(review.Domain, review.Reviewer)] = true
		}
	}
	seen := map[string]bool{}
	var missing []string
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		if !approved[domain] {
			missing = append(missing, domain)
		}
	}
	return missing
}

func dashboardEffectiveReviewStatus(stored string, missingReviewDomains []string) string {
	if stored == "approved" && len(missingReviewDomains) > 0 {
		return "partial_approval"
	}
	return stored
}

func optionalDashboardReviewHandback(handback coord.ReviewCompletionHandback, ok bool) *coord.ReviewCompletionHandback {
	if !ok {
		return nil
	}
	return &handback
}

func (s *Server) dashboardMissingReviewDomainsByTask(ctx context.Context, tasks []store.Task, allTasks []store.Task, timing *dashboardTiming) (map[string][]string, error) {
	missingByTask := map[string][]string{}
	start := time.Now()
	byID := make(map[string]store.Task, len(allTasks))
	for _, task := range allTasks {
		byID[task.Definition.ID] = task
	}
	var reviewTaskIDs []string
	for _, task := range tasks {
		reviewTaskIDs = append(reviewTaskIDs, task.Definition.ID)
		if parentID := strings.TrimSpace(task.Definition.ParentID); parentID != "" {
			reviewTaskIDs = append(reviewTaskIDs, parentID)
		}
	}
	reviewsByTask, err := s.store.ReviewsByTaskIDs(ctx, reviewTaskIDs)
	if err != nil {
		return nil, err
	}
	if timing != nil {
		timing.add("dashboard.missing_review_domains.batch_reviews", time.Since(start), fmt.Sprintf("tasks=%d", len(reviewTaskIDs)))
	}
	for _, task := range tasks {
		eval, err := s.dashboardReviewPolicyEvaluationFromMaps(task, byID, reviewsByTask)
		if err != nil {
			return nil, err
		}
		missing := eval.MissingReviewDomains
		if len(missing) > 0 {
			missingByTask[task.Definition.ID] = missing
		}
	}
	return missingByTask, nil
}

func (s *Server) dashboardReviewPolicyEvaluation(ctx context.Context, task store.Task, reviews []store.Review) (reviewpolicy.Evaluation, error) {
	var parent *store.Task
	var parentReviews []store.Review
	if strings.TrimSpace(task.Definition.ParentID) != "" {
		parentTask, _, _, _, parentTaskReviews, err := s.store.TaskDetail(ctx, task.Definition.ParentID)
		if err == nil {
			parent = &parentTask
			parentReviews = parentTaskReviews
		}
	}
	return reviewpolicy.Evaluate(s.cfg, reviewpolicy.Options{
		Task:          task,
		Parent:        parent,
		Reviews:       reviews,
		ParentReviews: parentReviews,
	}), nil
}

func (s *Server) dashboardReviewPolicyEvaluationFromMaps(task store.Task, tasksByID map[string]store.Task, reviewsByTask map[string][]store.Review) (reviewpolicy.Evaluation, error) {
	var parent *store.Task
	if parentID := strings.TrimSpace(task.Definition.ParentID); parentID != "" {
		if parentTask, ok := tasksByID[parentID]; ok {
			parent = &parentTask
		}
	}
	return reviewpolicy.Evaluate(s.cfg, reviewpolicy.Options{
		Task:          task,
		Parent:        parent,
		Reviews:       reviewsByTask[task.Definition.ID],
		ParentReviews: reviewsByTask[task.Definition.ParentID],
	}), nil
}

func localBackHref(referer, currentHost, role string) string {
	fallback := wallRoleHref(role)
	if referer == "" {
		return fallback
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Host != "" && parsed.Host != currentHost {
		return fallback
	}
	if parsed.Path == "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return fallback
	}
	if strings.HasPrefix(parsed.Path, "/tasks/") {
		return fallback
	}
	return parsed.RequestURI()
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

func activeFindingsForTask(findings []reconcile.ActiveFinding, taskID string) []reconcile.ActiveFinding {
	var out []reconcile.ActiveFinding
	for _, finding := range findings {
		if finding.TaskID == taskID {
			out = append(out, finding)
		}
	}
	return out
}

func dashboardBatchesForTask(ctx context.Context, s *store.Store, taskID string) ([]store.WorkBatch, error) {
	batches, err := s.WorkBatches(ctx)
	if err != nil {
		return nil, err
	}
	var out []store.WorkBatch
	for _, batch := range batches {
		for _, member := range batch.Tasks {
			if member == taskID {
				out = append(out, batch)
				break
			}
		}
	}
	return out, nil
}

func (s *Server) claim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowMutation(w) {
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
	s.snapshots.clear()
	http.Redirect(w, r, "/tasks/"+taskID, http.StatusSeeOther)
}

func (s *Server) setStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowMutation(w) {
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
	s.snapshots.clear()
	http.Redirect(w, r, "/tasks/"+taskID, http.StatusSeeOther)
}

func (s *Server) bulkClaim(w http.ResponseWriter, r *http.Request) {
	taskIDs, ok := s.bulkActionRequest(w, r)
	if !ok {
		return
	}
	for _, taskID := range taskIDs {
		task, _, _, _, _, err := s.store.TaskDetail(r.Context(), taskID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err := s.store.Claim(r.Context(), taskID, task.Definition.Role, ""); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = s.store.RecordAudit(r.Context(), store.AuditEvent{Action: "dashboard.bulk.claim", TaskID: taskID, Detail: "claimed from board bulk action"})
	}
	s.snapshots.clear()
	http.Redirect(w, r, bulkReturnTo(r), http.StatusSeeOther)
}

func (s *Server) bulkHandoff(w http.ResponseWriter, r *http.Request) {
	taskIDs, ok := s.bulkActionRequest(w, r)
	if !ok {
		return
	}
	toRole := strings.TrimSpace(r.FormValue("to_role"))
	payload := strings.TrimSpace(r.FormValue("payload"))
	if toRole == "" || payload == "" {
		http.Error(w, "to_role and payload are required", http.StatusBadRequest)
		return
	}
	for _, taskID := range taskIDs {
		if err := s.store.RecordHandoff(r.Context(), taskID, store.Handoff{ToRole: toRole, Payload: payload}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = s.store.RecordAudit(r.Context(), store.AuditEvent{Action: "dashboard.bulk.handoff", TaskID: taskID, Detail: "to=" + toRole})
	}
	s.snapshots.clear()
	http.Redirect(w, r, bulkReturnTo(r), http.StatusSeeOther)
}

func (s *Server) bulkSetStatus(w http.ResponseWriter, r *http.Request) {
	taskIDs, ok := s.bulkActionRequest(w, r)
	if !ok {
		return
	}
	target := strings.TrimSpace(r.FormValue("status"))
	reason := strings.TrimSpace(r.FormValue("reason"))
	stateCfg := state.Config{Allowed: s.cfg.States.Allowed, Terminal: s.cfg.States.Terminal, Transitions: s.cfg.States.Transitions}
	if target == "" {
		http.Error(w, "status is required", http.StatusBadRequest)
		return
	}
	if state.IsTerminal(stateCfg, target) {
		http.Error(w, "terminal status changes use CLI gates", http.StatusBadRequest)
		return
	}
	for _, taskID := range taskIDs {
		task, _, _, _, _, err := s.store.TaskDetail(r.Context(), taskID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
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
		_ = s.store.RecordAudit(r.Context(), store.AuditEvent{Action: "dashboard.bulk.set-status", TaskID: taskID, Detail: "status=" + target})
	}
	s.snapshots.clear()
	http.Redirect(w, r, bulkReturnTo(r), http.StatusSeeOther)
}

func (s *Server) bulkEvidence(w http.ResponseWriter, r *http.Request) {
	taskIDs, ok := s.bulkActionRequest(w, r)
	if !ok {
		return
	}
	commandText := strings.TrimSpace(r.FormValue("command_text"))
	result := strings.TrimSpace(r.FormValue("result"))
	artifact := strings.TrimSpace(r.FormValue("artifact"))
	artifactType := strings.TrimSpace(r.FormValue("artifact_type"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	if commandText == "" || result == "" {
		http.Error(w, "command_text and result are required", http.StatusBadRequest)
		return
	}
	for _, taskID := range taskIDs {
		if err := s.store.RecordEvidence(r.Context(), taskID, store.Evidence{CommandText: commandText, Result: result, ArtifactPath: artifact, ArtifactType: artifactType, Notes: notes}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = s.store.RecordAudit(r.Context(), store.AuditEvent{Action: "dashboard.bulk.evidence", TaskID: taskID, Detail: "result=" + result})
	}
	s.snapshots.clear()
	http.Redirect(w, r, bulkReturnTo(r), http.StatusSeeOther)
}

func (s *Server) saveView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowMutation(w) {
		return
	}
	if r.FormValue("csrf") != s.csrfToken {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	view, err := saveDashboardPersonalView(r.FormValue("name"), r.FormValue("query"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = s.store.RecordAudit(r.Context(), store.AuditEvent{Action: "dashboard.saved-view.save", Detail: "name=" + view.Name})
	s.snapshots.clear()
	http.Redirect(w, r, savedViewReturnTo(r, view), http.StatusSeeOther)
}

func (s *Server) bulkActionRequest(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return nil, false
	}
	if !s.allowMutation(w) {
		return nil, false
	}
	if r.FormValue("csrf") != s.csrfToken {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return nil, false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, false
	}
	taskIDs := trimmedQueryValues(r.Form["task_id"])
	if len(taskIDs) == 0 {
		http.Error(w, "at least one task_id is required", http.StatusBadRequest)
		return nil, false
	}
	return taskIDs, true
}

func (s *Server) allowMutation(w http.ResponseWriter) bool {
	if s.cfg.Dashboard.ReadOnly {
		http.Error(w, "dashboard is running in read-only shared mode", http.StatusForbidden)
		return false
	}
	return true
}

func bulkReturnTo(r *http.Request) string {
	returnTo := strings.TrimSpace(r.FormValue("return_to"))
	if returnTo == "" {
		return "/board"
	}
	parsed, err := url.Parse(returnTo)
	if err != nil || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return "/board"
	}
	return parsed.RequestURI()
}

func savedViewReturnTo(r *http.Request, view SavedView) string {
	returnTo := strings.TrimSpace(r.FormValue("return_to"))
	if returnTo == "" {
		return view.Href
	}
	parsed, err := url.Parse(returnTo)
	if err != nil || parsed.Host != "" || parsed.Path != "/board" {
		return view.Href
	}
	return parsed.RequestURI()
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
	cursor, err := s.store.LatestEventCursor(r.Context())
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
		flusher.Flush()
		return
	}
	if resumed, ok := parseSourceCursorID(r.Header.Get("Last-Event-ID")); ok {
		cursor = resumed
	}
	if _, err := io.WriteString(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	pollInterval := s.eventPollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	waitSweepInterval := s.reviewWaitSweepInterval
	if waitSweepInterval <= 0 {
		waitSweepInterval = maximumReviewWaitSweepInterval
	}
	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()
	waitTicker := time.NewTicker(waitSweepInterval)
	defer waitTicker.Stop()
	waitSeen := map[string]bool{}
	lastKeepalive := time.Now()

	writeWaitEvents := func(events []sseEvent, cursorID string) error {
		for _, event := range events {
			if waitSeen[event.ID] {
				continue
			}
			event.CursorID = cursorID
			if err := writeSSEEvent(w, event); err != nil {
				return err
			}
			waitSeen[event.ID] = true
		}
		return nil
	}
	writeError := func(err error) {
		fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
		flusher.Flush()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-pollTicker.C:
			s.sseStats.cursorChecks.Add(1)
			latest, err := s.store.LatestEventCursor(r.Context())
			if err != nil {
				writeError(err)
				return
			}
			if compareEventCursor(cursor, latest) >= 0 {
				if time.Since(lastKeepalive) >= 15*time.Second {
					if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
						return
					}
					flusher.Flush()
					lastKeepalive = time.Now()
				}
				continue
			}
			sources, err := s.store.EventSourcesAfter(r.Context(), cursor, dashboardEventPollLimit)
			s.sseStats.sourceHydrations.Add(1)
			if err != nil {
				writeError(err)
				return
			}
			affectedTasks := map[string]bool{}
			lastSourceID := ""
			lastCursorID := ""
			lastAt := ""
			for _, source := range sources {
				events := sseEventsFromSource(source)
				for _, event := range events {
					if err := writeSSEEvent(w, event); err != nil {
						return
					}
				}
				if source.TaskID != "" {
					affectedTasks[source.TaskID] = true
				}
				cursor = source.Cursor
				lastAt = source.Cursor.At
				lastCursorID = sourceCursorID(source.Cursor)
				if len(events) > 0 {
					lastSourceID = events[len(events)-1].ID
				}
			}
			if len(sources) == 0 {
				cursor = latest
				continue
			}
			if lastSourceID != "" {
				gateEvents, err := s.gateChangeEvents(r.Context(), lastSourceID, lastAt)
				if err != nil {
					writeError(err)
					return
				}
				for _, event := range gateEvents {
					event.CursorID = lastCursorID
					if err := writeSSEEvent(w, event); err != nil {
						return
					}
				}
			}
			for taskID := range affectedTasks {
				s.sseStats.targetWaitHydrations.Add(1)
				waitEvents, err := s.reviewWaitEventsForTask(r.Context(), taskID, time.Now().UTC().Format(time.RFC3339Nano))
				if err != nil {
					writeError(err)
					return
				}
				if err := writeWaitEvents(waitEvents, lastCursorID); err != nil {
					return
				}
			}
			if lastSourceID != "" {
				if err := writeLegacyRefresh(w, lastSourceID); err != nil {
					return
				}
			}
			flusher.Flush()
			lastKeepalive = time.Now()
		case <-waitTicker.C:
			s.sseStats.reviewWaitSweeps.Add(1)
			waitEvents, err := s.activeReviewWaitEvents(r.Context(), time.Now().UTC().Format(time.RFC3339Nano))
			if err != nil {
				writeError(err)
				return
			}
			if err := writeWaitEvents(waitEvents, ""); err != nil {
				return
			}
			flusher.Flush()
			lastKeepalive = time.Now()
		}
	}
}

//go:embed assets/*.svg assets/templates/*.html assets/templates/partials/*.html assets/css/*.css assets/js/*.js
var dashboardAssets embed.FS

var detailTemplate = mustEmbeddedTemplateSet("detail", []string{
	"assets/templates/layout.html",
	"assets/templates/task-detail.html",
	"assets/templates/partials/provider-chip.html",
}, dashboardTemplateFuncs())

var wallTemplate = mustEmbeddedTemplateSet("wall", []string{
	"assets/templates/layout.html",
	"assets/templates/wall.html",
	"assets/templates/partials/lane-card.html",
	"assets/templates/partials/gate-gauge.html",
	"assets/templates/partials/provider-chip.html",
}, dashboardTemplateFuncs())

var boardTemplate = mustEmbeddedTemplateSet("board", []string{
	"assets/templates/layout.html",
	"assets/templates/board.html",
	"assets/templates/partials/diagnostics-panels.html",
	"assets/templates/partials/gate-gauge.html",
	"assets/templates/partials/provider-chip.html",
}, dashboardTemplateFuncs())

var reportsTemplate = mustEmbeddedTemplateSet("reports", []string{
	"assets/templates/layout.html",
	"assets/templates/reports.html",
	"assets/templates/partials/provider-chip.html",
}, dashboardTemplateFuncs())

var overviewTemplate = mustEmbeddedTemplateSet("overview", []string{
	"assets/templates/layout.html",
	"assets/templates/overview.html",
	"assets/templates/partials/provider-chip.html",
}, dashboardTemplateFuncs())

var qualityTemplate = mustEmbeddedTemplateSet("quality", []string{
	"assets/templates/layout.html",
	"assets/templates/quality.html",
	"assets/templates/partials/provider-chip.html",
}, dashboardTemplateFuncs())

var controlsTemplate = mustEmbeddedTemplateSet("controls", []string{
	"assets/templates/layout.html",
	"assets/templates/controls.html",
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
		"takeWorkCoverageFindings": func(findings []audit.WorkCoverageFinding, limit int) []audit.WorkCoverageFinding {
			if limit <= 0 || len(findings) <= limit {
				return findings
			}
			return findings[:limit]
		},
		"takeCILearningFindings": func(findings []audit.CILearningFinding, limit int) []audit.CILearningFinding {
			if limit <= 0 || len(findings) <= limit {
				return findings
			}
			return findings[:limit]
		},
		"usageInt": func(value *int) string {
			if value == nil {
				return "unknown"
			}
			return strconv.Itoa(*value)
		},
		"ratioPercent": func(value float64) string {
			return fmt.Sprintf("%.1f%%", value*100)
		},
		"signedRatioPercent": func(value float64) string {
			return fmt.Sprintf("%+.1f%%", value*100)
		},
		"durationSeconds": func(value int) string {
			return (time.Duration(value) * time.Second).String()
		},
		"qualityPageSizes": func() []int { return []int{10, 25, 50} },
		"qualityStateLabel": func(value string) string {
			switch value {
			case "externally_owned":
				return "external"
			case "unavailable":
				return "unknown"
			default:
				return value
			}
		},
		"shortSHA": func(value string) string {
			value = strings.TrimSpace(value)
			if len(value) > 10 {
				return value[:10]
			}
			return value
		},
		"multiProject": func(value any) bool {
			candidate := reflect.ValueOf(value)
			if candidate.Kind() == reflect.Pointer {
				candidate = candidate.Elem()
			}
			if !candidate.IsValid() || candidate.Kind() != reflect.Struct {
				return false
			}
			field := candidate.FieldByName("MultiProject")
			return field.IsValid() && field.Kind() == reflect.Bool && field.Bool()
		},
		"moreTasks": func(tasks []store.Task, limit int) int {
			if limit <= 0 || len(tasks) <= limit {
				return 0
			}
			return len(tasks) - limit
		},
		"wallLaneTasks":                wallLaneTasks,
		"wallMissingReviewDomains":     wallMissingReviewDomains,
		"wallProviderClass":            wallProviderClass,
		"wallActiveSessions":           wallActiveSessions,
		"wallRoleActiveSession":        wallRoleActiveSession,
		"wallSessionCheckpoint":        wallSessionCheckpoint,
		"wallSessionTaskRole":          wallSessionTaskRole,
		"wallTasksMoving":              wallTasksMoving,
		"wallHandoffCount":             wallHandoffCount,
		"wallDoneToday":                wallDoneToday,
		"wallTaskHasProvider":          wallTaskHasProvider,
		"wallRoleHref":                 wallRoleHref,
		"wallProjectRoleHref":          wallProjectRoleHref,
		"wallLaneHref":                 wallLaneHref,
		"wallRoleActivity":             wallRoleActivity,
		"boardRows":                    boardRows,
		"boardTabHref":                 boardTabHref,
		"boardDiagnosticsPanelHref":    boardDiagnosticsPanelHref,
		"boardDiagnosticsPanelHrefFor": boardDiagnosticsPanelHrefFor,
		"boardPageHref":                boardPageHref,
		"boardExportHref":              boardExportHref,
		"boardSortHref":                boardSortHref,
		"boardSortState":               boardSortState,
		"boardSortAria":                boardSortAria,
		"boardColumns":                 boardColumns,
		"boardColumnsParam":            boardColumnsParam,
		"boardVisibleColumns":          boardVisibleColumns,
		"boardColumnCount":             boardColumnCount,
		"boardTaskCell":                boardTaskCell,
		"statusFilterValues":           statusFilterValues,
		"statusSelected":               statusSelected,
		"contains":                     containsString,
		"statusClass":                  safeDashboardClass,
		"safeClass":                    safeDashboardClass,
		"dashboardReadOnly":            dashboardReadOnly,
		"dict":                         templateDict,
	}
	return funcs
}

func dashboardReadOnly(data any) bool {
	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return false
	}
	field := value.FieldByName("ReadOnly")
	return field.IsValid() && field.Kind() == reflect.Bool && field.Bool()
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

func wallLaneTasks(tasks []store.Task, lane string, sessions []store.Session, missingReviewDomains map[string][]string) []store.Task {
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
			if wallTaskHasProvider(task, sessions) {
				out = append(out, task)
			}
		case "review":
			if wallTaskNeedsReview(task, missingReviewDomains) {
				out = append(out, task)
			}
		case "done":
			if task.Status == "done" && !wallTaskNeedsReview(task, missingReviewDomains) {
				out = append(out, task)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return dashboardTaskMoreRecent(out[i], out[j])
	})
	return out
}

func wallTaskNeedsReview(task store.Task, missingReviewDomains map[string][]string) bool {
	if len(missingReviewDomains[task.Definition.ID]) > 0 {
		return true
	}
	return task.ReviewStatus != "" && task.ReviewStatus != "approved" && task.ReviewStatus != "not_required"
}

func wallMissingReviewDomains(task store.Task, missingReviewDomains map[string][]string) []string {
	return missingReviewDomains[task.Definition.ID]
}

func wallRoleActivity(role string, activity []store.Activity, taskRoles map[string]string, limit int) []store.Activity {
	var out []store.Activity
	for _, event := range activity {
		if taskRoles[event.TaskID] != role {
			continue
		}
		out = append(out, event)
		if limit > 0 && len(out) >= limit {
			return out
		}
	}
	return out
}

func dashboardTaskMoreRecent(left, right store.Task) bool {
	if left.UpdatedAt != right.UpdatedAt {
		return left.UpdatedAt > right.UpdatedAt
	}
	leftPriority := taskPriorityRank(left)
	rightPriority := taskPriorityRank(right)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	return left.Definition.ID < right.Definition.ID
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
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LastHeartbeatAt != out[j].LastHeartbeatAt {
			return out[i].LastHeartbeatAt > out[j].LastHeartbeatAt
		}
		if out[i].StartedAt != out[j].StartedAt {
			return out[i].StartedAt > out[j].StartedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func wallRoleActiveSession(role string, tasks []store.Task, sessions []store.Session, taskRoles map[string]string) *store.Session {
	taskIDs := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		taskIDs[task.Definition.ID] = true
	}
	active := wallActiveSessions(sessions)
	for i := range active {
		sessionTaskRole := strings.TrimSpace(taskRoles[active[i].TaskID])
		if sessionTaskRole == role || taskIDs[active[i].TaskID] || (active[i].TaskID == "" && active[i].Role == role) {
			return &active[i]
		}
	}
	return nil
}

func wallSessionTaskRole(session store.Session, taskRoles map[string]string, groups []RoleGroup) string {
	if taskRoles != nil {
		if role := strings.TrimSpace(taskRoles[session.TaskID]); role != "" {
			return role
		}
	}
	for _, group := range groups {
		for _, task := range group.Tasks {
			if task.Definition.ID == session.TaskID {
				return task.Definition.Role
			}
		}
	}
	return session.Role
}

func taskRoleMap(tasks []store.Task) map[string]string {
	roles := make(map[string]string, len(tasks))
	for _, task := range tasks {
		if task.Definition.ID == "" || task.Definition.Role == "" {
			continue
		}
		roles[task.Definition.ID] = task.Definition.Role
	}
	return roles
}

func wallSessionCheckpoint(session store.Session, checkpoints []store.Checkpoint) string {
	if strings.TrimSpace(session.TaskID) == "" {
		return "no task checkpoint"
	}
	for _, checkpoint := range checkpoints {
		if checkpoint.TaskID != session.TaskID {
			continue
		}
		summary := strings.TrimSpace(checkpoint.Summary)
		if summary == "" {
			return checkpoint.State
		}
		return checkpoint.State + ": " + summary
	}
	return "no checkpoint"
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
	today := time.Now().Format("2006-01-02")
	count := 0
	for _, group := range groups {
		for _, task := range group.Tasks {
			if task.Status != "done" {
				continue
			}
			if dashboardLocalDate(task.UpdatedAt) == today {
				count++
			}
		}
	}
	return count
}

func dashboardLocalDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts.Local().Format("2006-01-02")
	}
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	return value
}

func boardRows(groups []RoleGroup) []store.Task {
	var rows []store.Task
	for _, group := range groups {
		rows = append(rows, group.Tasks...)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		leftPriority := taskPriorityRank(rows[i])
		rightPriority := taskPriorityRank(rows[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if rows[i].Status != rows[j].Status {
			return rows[i].Status < rows[j].Status
		}
		return rows[i].Definition.ID < rows[j].Definition.ID
	})
	return rows
}

func taskPriorityRank(task store.Task) int {
	if task.Definition.Priority == nil {
		return 999
	}
	return *task.Definition.Priority
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
