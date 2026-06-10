package coordinator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/reconcile"
	"github.com/subashram/fairway/internal/store"
)

type WorktreeFact struct {
	Role       string `json:"role"`
	Branch     string `json:"branch"`
	Path       string `json:"path"`
	Registered bool   `json:"registered"`
	Exists     bool   `json:"exists"`
	Dirty      bool   `json:"dirty"`
	LastCommit string `json:"last_commit"`
}

type PlanOptions struct {
	Worktrees             []WorktreeFact
	StaleCheckpointAfter  time.Duration
	MonitorHandbackAfter  time.Duration
	ReadyLimit            int
	RecommendationLimit   int
	UtilityMonitorAllowed bool
}

type Plan struct {
	OK                bool                   `json:"ok"`
	DryRun            bool                   `json:"dry_run"`
	GeneratedAt       string                 `json:"generated_at"`
	Summary           PlanSummary            `json:"summary"`
	Actions           []PlanAction           `json:"actions"`
	Ready             []TaskRef              `json:"ready,omitempty"`
	Active            []TaskRef              `json:"active,omitempty"`
	Waiting           []TaskRef              `json:"waiting,omitempty"`
	Blocked           []TaskRef              `json:"blocked,omitempty"`
	ReviewGated       []TaskRef              `json:"review_gated,omitempty"`
	ReviewDebt        []TaskRef              `json:"review_debt,omitempty"`
	NotificationGated []TaskRef              `json:"notification_gated,omitempty"`
	UtilityGated      []TaskRef              `json:"utility_gated,omitempty"`
	StopConditions    []PlanStopCondition    `json:"stop_conditions,omitempty"`
	Reconcile         reconcile.ActiveReport `json:"reconcile"`
	SessionCount      int                    `json:"session_count"`
	WatcherCount      int                    `json:"watcher_count"`
	CheckpointCount   int                    `json:"checkpoint_count"`
	WorktreeCount     int                    `json:"worktree_count"`
	WorkBatchCount    int                    `json:"work_batch_count"`
}

type PlanSummary struct {
	TopClassification string `json:"top_classification"`
	TopReason         string `json:"top_reason"`
	Ready             int    `json:"ready"`
	Active            int    `json:"active"`
	Waiting           int    `json:"waiting"`
	Blocked           int    `json:"blocked"`
	Stale             int    `json:"stale"`
	Complete          int    `json:"complete"`
	ReviewGated       int    `json:"review_gated"`
	ReviewDebt        int    `json:"review_debt"`
	NotificationGated int    `json:"notification_gated"`
	ApprovalGated     int    `json:"approval_gated"`
	UtilityGated      int    `json:"utility_gated"`
	BatchRecommended  int    `json:"batch_recommended"`
}

type PlanAction struct {
	Priority       int      `json:"priority"`
	Classification string   `json:"classification"`
	Action         string   `json:"action"`
	Reason         string   `json:"reason"`
	TaskID         string   `json:"task_id,omitempty"`
	Role           string   `json:"role,omitempty"`
	SessionID      string   `json:"session_id,omitempty"`
	WatcherID      string   `json:"watcher_id,omitempty"`
	BatchKey       string   `json:"batch_key,omitempty"`
	TaskIDs        []string `json:"task_ids,omitempty"`
	Stop           bool     `json:"stop,omitempty"`
}

type PlanStopCondition struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	TaskID string `json:"task_id,omitempty"`
	Role   string `json:"role,omitempty"`
}

type TaskRef struct {
	ID     string `json:"id"`
	Title  string `json:"title,omitempty"`
	Role   string `json:"role,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Status string `json:"status,omitempty"`
}

func BuildPlan(ctx context.Context, cfg config.Config, s *store.Store, opts PlanOptions) (Plan, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return Plan{}, err
	}
	ready, err := s.Ready(ctx, "", cfg.States.Terminal)
	if err != nil {
		return Plan{}, err
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		return Plan{}, err
	}
	watchers, err := s.Watchers(ctx, false)
	if err != nil {
		return Plan{}, err
	}
	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return Plan{}, err
	}
	batches, err := s.WorkBatches(ctx)
	if err != nil {
		return Plan{}, err
	}
	notificationGaps, err := s.HandoffNotificationGaps(ctx)
	if err != nil {
		return Plan{}, err
	}
	activeReport, err := reconcile.Active(ctx, s, reconcile.ActiveOptions{
		Terminal:             cfg.States.Terminal,
		StaleCheckpointAfter: opts.StaleCheckpointAfter,
		MonitorHandbackAfter: opts.MonitorHandbackAfter,
	})
	if err != nil {
		return Plan{}, err
	}
	terminal := map[string]bool{}
	for _, status := range cfg.States.Terminal {
		terminal[status] = true
	}
	taskStatusByID := map[string]string{}
	for _, task := range tasks {
		taskStatusByID[task.Definition.ID] = task.Status
	}
	if opts.ReadyLimit <= 0 {
		opts.ReadyLimit = 10
	}
	if opts.RecommendationLimit <= 0 {
		opts.RecommendationLimit = 20
	}
	plan := Plan{
		OK:              true,
		DryRun:          true,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Reconcile:       activeReport,
		SessionCount:    len(sessions),
		WatcherCount:    len(watchers),
		CheckpointCount: len(checkpoints),
		WorktreeCount:   len(opts.Worktrees),
		WorkBatchCount:  len(batches),
	}
	activeSessionByTask := map[string]store.Session{}
	for _, session := range sessions {
		if session.TaskID != "" {
			activeSessionByTask[session.TaskID] = session
		}
		if isUtilitySession(session) {
			plan.Summary.UtilityGated++
			plan.UtilityGated = appendTaskRef(plan.UtilityGated, TaskRef{ID: session.TaskID, Role: session.Role, Status: session.Status})
			action := "wait_for_utility_handback"
			if opts.UtilityMonitorAllowed {
				action = "continue_configured_utility_monitor"
			}
			addAction(&plan, 40, "utility-gated", action, "utility or monitor session is still running", session.TaskID, session.Role, session.ID, "", nil, false)
		}
		if strings.Contains(session.Status, "approval") {
			plan.Summary.ApprovalGated++
			addStop(&plan, "approval-gated", "provider session is waiting on approval", session.TaskID, session.Role)
			addAction(&plan, 5, "approval-gated", "request_or_record_approval", "provider session is waiting on approval", session.TaskID, session.Role, session.ID, "", nil, true)
		}
	}
	for _, watcher := range watchers {
		plan.Summary.UtilityGated++
		plan.UtilityGated = appendTaskRef(plan.UtilityGated, TaskRef{ID: watcher.TaskID, Role: watcher.Owner, Status: watcher.Status})
		action := "wait_for_watcher_handback"
		if opts.UtilityMonitorAllowed {
			action = "continue_configured_utility_monitor"
		}
		addAction(&plan, 40, "utility-gated", action, "watcher or monitor is still active", watcher.TaskID, watcher.Owner, "", watcher.ID, nil, false)
	}
	for _, finding := range activeReport.Findings {
		classification := "stale"
		priority := 10
		stop := false
		switch finding.Kind {
		case "provider_session_missing_lifecycle_checkpoint":
			classification = "waiting"
			priority = 20
		case "monitor_session_without_backing_proof", "monitor_completion_resume_needed":
			classification = "utility-gated"
			priority = 10
		case "status_decision_required":
			classification = "complete"
			priority = 15
		case "unattended_in_progress", "stale_checkpoint", "running_session_terminal_task", "running_session_missing_task":
			classification = "stale"
			priority = 10
		}
		if strings.Contains(finding.Action, "approval") {
			classification = "approval-gated"
			stop = true
		}
		addAction(&plan, priority, classification, finding.Action, finding.Reason, finding.TaskID, finding.Role, finding.SessionID, "", nil, stop)
	}
	for _, gap := range notificationGaps {
		plan.Summary.NotificationGated++
		plan.NotificationGated = appendTaskRef(plan.NotificationGated, TaskRef{ID: gap.TaskID, Role: gap.Role, Status: "handoff_unnotified"})
		reason := fmt.Sprintf("handoff %d to %s has no delivered provider/thread notification; provider_target=%s", gap.HandoffID, gap.Domain, providerTargetSummary(cfg.ProviderTargets, gap.Domain))
		if gap.LastState != "" {
			reason += "; latest notification state=" + gap.LastState
		}
		if gap.LastNotificationAt != "" {
			reason += "; last_notification_at=" + gap.LastNotificationAt
		}
		reason += "; last_handoff_at=" + gap.LastHandoffAt
		addAction(&plan, 18, "notification-gated", "send_or_record_provider_notification", reason, gap.TaskID, gap.Domain, "", "", nil, false)
	}
	for _, checkpoint := range latestOpenCheckpointsByTask(checkpoints) {
		if terminal[taskStatusByID[checkpoint.TaskID]] {
			continue
		}
		switch checkpoint.State {
		case "awaiting_input":
			plan.Summary.Waiting++
			plan.Waiting = appendTaskRef(plan.Waiting, TaskRef{ID: checkpoint.TaskID, Role: checkpoint.Owner, Status: checkpoint.State})
			if strings.Contains(strings.ToLower(checkpoint.Summary), "approval") {
				plan.Summary.ApprovalGated++
				addStop(&plan, "approval-gated", checkpoint.Summary, checkpoint.TaskID, checkpoint.Owner)
			}
			addAction(&plan, 20, "waiting", "resolve_awaiting_input_checkpoint", checkpoint.Summary, checkpoint.TaskID, checkpoint.Owner, "", "", nil, false)
		case "review":
			addAction(&plan, 25, "review-gated", "complete_review_checkpoint", checkpoint.Summary, checkpoint.TaskID, checkpoint.Owner, "", "", nil, false)
		}
	}
	readyRefs := make([]TaskRef, 0, len(ready))
	for _, task := range ready {
		readyRefs = append(readyRefs, taskRef(task))
	}
	plan.Ready = limitTaskRefs(readyRefs, opts.ReadyLimit)
	plan.Summary.Ready = len(ready)
	for _, task := range tasks {
		switch {
		case task.Status == "in_progress":
			plan.Summary.Active++
			plan.Active = appendTaskRef(plan.Active, taskRef(task))
			if _, ok := activeSessionByTask[task.Definition.ID]; ok {
				addAction(&plan, 50, "active", "continue_active_session", "task has an active provider/session attachment", task.Definition.ID, task.Definition.Role, activeSessionByTask[task.Definition.ID].ID, "", nil, false)
			}
		case task.Status == "blocked":
			plan.Summary.Blocked++
			plan.Blocked = appendTaskRef(plan.Blocked, taskRef(task))
			addAction(&plan, 25, "blocked", "resolve_blocked_task", "task is blocked and needs an unblock decision", task.Definition.ID, task.Definition.Role, "", "", nil, false)
		case terminal[task.Status]:
			plan.Summary.Complete++
		}
		if len(task.Definition.ReviewDomains) > 0 && (terminal[task.Status] || task.Status == "review") {
			_, _, _, _, reviews, err := s.TaskDetail(ctx, task.Definition.ID)
			if err != nil {
				return Plan{}, err
			}
			missing := missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)
			if len(missing) > 0 {
				reason := "missing required review domains: " + strings.Join(missing, ", ")
				if terminal[task.Status] {
					plan.Summary.ReviewDebt++
					plan.ReviewDebt = appendTaskRef(plan.ReviewDebt, taskRef(task))
					addAction(&plan, 45, "review-debt", "sweep_historical_review_debt", reason, task.Definition.ID, task.Definition.Role, "", "", nil, false)
				} else {
					plan.Summary.ReviewGated++
					plan.ReviewGated = appendTaskRef(plan.ReviewGated, taskRef(task))
					addAction(&plan, 15, "review-gated", "record_required_reviews", reason, task.Definition.ID, task.Definition.Role, "", "", nil, true)
					addStop(&plan, "review-gated", reason, task.Definition.ID, task.Definition.Role)
				}
			}
		}
	}
	for _, group := range relatedReadyGroups(ready) {
		plan.Summary.BatchRecommended++
		addAction(&plan, 35, "ready", "consider_work_batch", "related ready tasks share role/domain/kind/review surface", "", "", "", "", group.TaskIDs, false)
	}
	for _, task := range ready {
		addAction(&plan, 60, "ready", "claim_ready_task", "task is ready and no higher-priority stop condition applies", task.Definition.ID, task.Definition.Role, "", "", nil, false)
	}
	for _, wt := range opts.Worktrees {
		if wt.Dirty {
			addStop(&plan, "unsafe-to-infer", "worktree has uncommitted changes", "", wt.Role)
			addAction(&plan, 5, "blocked", "clean_or_commit_worktree_before_dispatch", "worktree has uncommitted changes", "", wt.Role, "", "", nil, true)
		}
	}
	sort.SliceStable(plan.Actions, func(i, j int) bool {
		if plan.Actions[i].Priority != plan.Actions[j].Priority {
			return plan.Actions[i].Priority < plan.Actions[j].Priority
		}
		if plan.Actions[i].Classification != plan.Actions[j].Classification {
			return plan.Actions[i].Classification < plan.Actions[j].Classification
		}
		if plan.Actions[i].TaskID != plan.Actions[j].TaskID {
			return plan.Actions[i].TaskID < plan.Actions[j].TaskID
		}
		return plan.Actions[i].Action < plan.Actions[j].Action
	})
	plan.Actions = uniquePlanActions(plan.Actions)
	if len(plan.Actions) > opts.RecommendationLimit {
		plan.Actions = plan.Actions[:opts.RecommendationLimit]
	}
	if len(plan.Actions) > 0 {
		plan.Summary.TopClassification = plan.Actions[0].Classification
		plan.Summary.TopReason = plan.Actions[0].Reason
	} else {
		plan.Summary.TopClassification = "idle"
		plan.Summary.TopReason = "no active, ready, blocked, stale, review-gated, approval-gated, utility-gated, or review-debt work found"
	}
	plan.OK = len(plan.StopConditions) == 0 && activeReport.OK
	return plan, nil
}

type relatedReadyGroup struct {
	Key     string
	TaskIDs []string
}

func relatedReadyGroups(tasks []store.Task) []relatedReadyGroup {
	groups := map[string][]string{}
	for _, task := range tasks {
		keyParts := []string{task.Definition.Role, task.Definition.OwningDomain, task.Definition.Kind, strings.Join(task.Definition.ReviewDomains, ",")}
		key := strings.Join(keyParts, "\x00")
		if strings.Trim(key, "\x00") == "" {
			continue
		}
		groups[key] = append(groups[key], task.Definition.ID)
	}
	var out []relatedReadyGroup
	for key, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		out = append(out, relatedReadyGroup{Key: key, TaskIDs: ids})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].TaskIDs) != len(out[j].TaskIDs) {
			return len(out[i].TaskIDs) > len(out[j].TaskIDs)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func latestCheckpointByTask(checkpoints []store.Checkpoint) map[string]store.Checkpoint {
	out := map[string]store.Checkpoint{}
	for _, checkpoint := range checkpoints {
		if _, ok := out[checkpoint.TaskID]; !ok {
			out[checkpoint.TaskID] = checkpoint
		}
	}
	return out
}

func latestOpenCheckpointsByTask(checkpoints []store.Checkpoint) []store.Checkpoint {
	latest := latestCheckpointByTask(checkpoints)
	out := make([]store.Checkpoint, 0, len(latest))
	for _, checkpoint := range latest {
		switch checkpoint.State {
		case "done", "parked", "abandoned":
			continue
		default:
			out = append(out, checkpoint)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].TaskID < out[j].TaskID
	})
	return out
}

func taskRef(task store.Task) TaskRef {
	return TaskRef{ID: task.Definition.ID, Title: task.Definition.Title, Role: task.Definition.Role, Kind: task.Definition.Kind, Status: task.Status}
}

func appendTaskRef(refs []TaskRef, ref TaskRef) []TaskRef {
	if ref.ID == "" {
		return refs
	}
	for _, existing := range refs {
		if existing.ID == ref.ID {
			return refs
		}
	}
	return append(refs, ref)
}

func providerTargetSummary(targets []config.ProviderTarget, domain string) string {
	domain = strings.TrimSpace(domain)
	var labels []string
	for _, target := range targets {
		if strings.TrimSpace(target.Domain) != domain {
			continue
		}
		targetType := strings.TrimSpace(target.Type)
		if targetType == "" {
			targetType = "generic"
		}
		labels = append(labels, fmt.Sprintf("%s:%s(%s)", strings.TrimSpace(target.Provider), strings.TrimSpace(target.Target), targetType))
	}
	if len(labels) == 0 {
		return "unconfigured"
	}
	sort.Strings(labels)
	return strings.Join(labels, ",")
}

func limitTaskRefs(refs []TaskRef, limit int) []TaskRef {
	if limit > 0 && len(refs) > limit {
		return refs[:limit]
	}
	return refs
}

func addAction(plan *Plan, priority int, classification, action, reason, taskID, role, sessionID, watcherID string, taskIDs []string, stop bool) {
	plan.Actions = append(plan.Actions, PlanAction{Priority: priority, Classification: classification, Action: action, Reason: reason, TaskID: taskID, Role: role, SessionID: sessionID, WatcherID: watcherID, TaskIDs: taskIDs, Stop: stop})
	switch classification {
	case "stale":
		plan.Summary.Stale++
	case "waiting":
		if taskID == "" {
			plan.Summary.Waiting++
		}
	case "utility-gated":
		if taskID == "" {
			plan.Summary.UtilityGated++
		}
	case "review-debt":
		if taskID == "" {
			plan.Summary.ReviewDebt++
		}
	}
}

func addStop(plan *Plan, kind, reason, taskID, role string) {
	for _, existing := range plan.StopConditions {
		if existing.Kind == kind && existing.TaskID == taskID && existing.Role == role && existing.Reason == reason {
			return
		}
	}
	plan.StopConditions = append(plan.StopConditions, PlanStopCondition{Kind: kind, Reason: reason, TaskID: taskID, Role: role})
}

func uniquePlanActions(actions []PlanAction) []PlanAction {
	seen := map[string]bool{}
	var out []PlanAction
	for _, action := range actions {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s", action.Classification, action.Action, action.TaskID, action.Role, action.SessionID, strings.Join(action.TaskIDs, ","))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, action)
	}
	return out
}

func isUtilitySession(session store.Session) bool {
	return strings.TrimSpace(session.MonitorKind) != "" || strings.Contains(strings.ToLower(session.SessionBackend), "monitor") || strings.Contains(strings.ToLower(session.SessionBackend), "utility")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func missingApprovedReviewDomains(domains []string, reviews []store.Review) []string {
	approved := map[string]bool{}
	for _, review := range reviews {
		if review.Verdict == "approve" {
			approved[firstNonEmpty(review.Domain, review.Reviewer)] = true
		}
	}
	var missing []string
	for _, domain := range domains {
		if !approved[domain] {
			missing = append(missing, domain)
		}
	}
	sort.Strings(missing)
	return missing
}
