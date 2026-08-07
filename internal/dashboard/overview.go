package dashboard

import (
	"net/http"
	"sort"
	"time"

	"github.com/subashram/fairway/internal/qualityrecord"
	"github.com/subashram/fairway/internal/store"
)

const overviewCandidateLimit = 12

type OverviewSummary struct {
	TotalTasks      int
	EvidenceBacked  int
	Reviewed        int
	Completed       int
	Active          int
	Blocked         int
	ActiveProviders int
}

type OverviewViewData struct {
	View           string
	ProjectName    string
	Summary        OverviewSummary
	FeaturedTask   store.Task
	FeaturedRecord qualityrecord.Record
	Sessions       []store.Session
	Activity       []store.Activity
	Groups         []RoleGroup
	TaskRoles      map[string]string
	ReadOnly       bool
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	timing := newDashboardTiming("overview", r)
	defer timing.logIfSlow()
	data, err := s.overviewViewData(r, timing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	start := time.Now()
	if err := overviewTemplate.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	timing.add("template.overview", time.Since(start), "")
}

func (s *Server) overviewViewData(r *http.Request, timing *dashboardTiming) (OverviewViewData, error) {
	if dashboardSnapshotCacheable(r) {
		key := dashboardSnapshotKey("overview", r)
		data, status, err := dashboardSnapshotGet(s.snapshots, key, func() (OverviewViewData, error) {
			return s.buildOverviewViewData(r, timing)
		})
		timing.add("overview.snapshot_cache", 0, "status="+status)
		return data, err
	}
	return s.buildOverviewViewData(r, timing)
}

func (s *Server) buildOverviewViewData(r *http.Request, timing *dashboardTiming) (OverviewViewData, error) {
	var tasks []store.Task
	if err := timing.step("overview.tasks", func() error {
		var err error
		tasks, err = s.store.AllTasks(r.Context())
		return err
	}); err != nil {
		return OverviewViewData{}, err
	}
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	var evidenceByTask map[string][]store.Evidence
	if err := timing.step("overview.evidence", func() error {
		var err error
		evidenceByTask, err = s.store.EvidenceByTaskIDs(r.Context(), taskIDs)
		return err
	}); err != nil {
		return OverviewViewData{}, err
	}
	var reviewsByTask map[string][]store.Review
	if err := timing.step("overview.reviews", func() error {
		var err error
		reviewsByTask, err = s.store.ReviewsByTaskIDs(r.Context(), taskIDs)
		return err
	}); err != nil {
		return OverviewViewData{}, err
	}
	sessions, _ := s.store.Sessions(r.Context(), false)
	activity, _ := s.store.Activity(r.Context(), defaultActivityLimit)
	candidates := overviewCandidates(tasks, evidenceByTask, reviewsByTask)
	var records []qualityrecord.Record
	if err := timing.step("overview.featured_record", func() error {
		var err error
		records, err = qualityrecord.BuildMany(r.Context(), s.store, candidates, s.now().UTC())
		return err
	}); err != nil {
		return OverviewViewData{}, err
	}
	featuredTask, featuredRecord := featuredOverviewRecord(candidates, records)
	return OverviewViewData{
		View:           "overview",
		ProjectName:    s.cfg.Fairway.ProjectName,
		Summary:        overviewSummary(tasks, evidenceByTask, reviewsByTask, sessions),
		FeaturedTask:   featuredTask,
		FeaturedRecord: featuredRecord,
		Sessions:       sessions,
		Activity:       activity,
		Groups:         groupTasks(tasks, s.roles),
		TaskRoles:      taskRoleMap(tasks),
		ReadOnly:       s.cfg.Dashboard.ReadOnly,
	}, nil
}

func overviewSummary(tasks []store.Task, evidenceByTask map[string][]store.Evidence, reviewsByTask map[string][]store.Review, sessions []store.Session) OverviewSummary {
	summary := OverviewSummary{TotalTasks: len(tasks)}
	for _, task := range tasks {
		if len(evidenceByTask[task.Definition.ID]) > 0 {
			summary.EvidenceBacked++
		}
		if len(reviewsByTask[task.Definition.ID]) > 0 {
			summary.Reviewed++
		}
		switch task.Status {
		case "done":
			summary.Completed++
		case "in_progress":
			summary.Active++
		case "blocked":
			summary.Blocked++
		}
	}
	for _, session := range sessions {
		if session.Status == "running" {
			summary.ActiveProviders++
		}
	}
	return summary
}

func overviewCandidates(tasks []store.Task, evidenceByTask map[string][]store.Evidence, reviewsByTask map[string][]store.Review) []store.Task {
	candidates := append([]store.Task(nil), tasks...)
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		leftComplete := left.Status == "done"
		rightComplete := right.Status == "done"
		if leftComplete != rightComplete {
			return leftComplete
		}
		leftCited := len(evidenceByTask[left.Definition.ID]) > 0 && len(reviewsByTask[left.Definition.ID]) > 0
		rightCited := len(evidenceByTask[right.Definition.ID]) > 0 && len(reviewsByTask[right.Definition.ID]) > 0
		if leftCited != rightCited {
			return leftCited
		}
		if left.UpdatedAt != right.UpdatedAt {
			return left.UpdatedAt > right.UpdatedAt
		}
		return left.Definition.ID < right.Definition.ID
	})
	if len(candidates) > overviewCandidateLimit {
		candidates = candidates[:overviewCandidateLimit]
	}
	return candidates
}

func featuredOverviewRecord(tasks []store.Task, records []qualityrecord.Record) (store.Task, qualityrecord.Record) {
	best := -1
	bestPresent := -1
	for i, record := range records {
		if i >= len(tasks) {
			break
		}
		if record.Summary.Present > bestPresent {
			best = i
			bestPresent = record.Summary.Present
		}
	}
	if best < 0 {
		return store.Task{}, qualityrecord.Record{}
	}
	return tasks[best], records[best]
}
