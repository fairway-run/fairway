package coordinator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/completionhandback"
	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/livewindow"
	"github.com/subashram/fairway/internal/reconcile"
	"github.com/subashram/fairway/internal/reviewpolicy"
	"github.com/subashram/fairway/internal/reviewstate"
	"github.com/subashram/fairway/internal/store"
)

type WorktreeFact struct {
	Role           string `json:"role"`
	Branch         string `json:"branch"`
	Path           string `json:"path"`
	Registered     bool   `json:"registered"`
	Exists         bool   `json:"exists"`
	Dirty          bool   `json:"dirty"`
	LastCommit     string `json:"last_commit"`
	GitUnavailable bool   `json:"git_unavailable,omitempty"`
	Diagnostic     string `json:"diagnostic,omitempty"`
}

type PlanOptions struct {
	Worktrees              []WorktreeFact
	StaleCheckpointAfter   time.Duration
	MonitorHandbackAfter   time.Duration
	ReviewHandbackFreshFor time.Duration
	NotificationAckTimeout time.Duration
	ReadyLimit             int
	RecommendationLimit    int
	UtilityMonitorAllowed  bool
}

type Plan struct {
	OK                  bool                          `json:"ok"`
	DryRun              bool                          `json:"dry_run"`
	GeneratedAt         string                        `json:"generated_at"`
	Summary             PlanSummary                   `json:"summary"`
	Actions             []PlanAction                  `json:"actions"`
	Ready               []TaskRef                     `json:"ready,omitempty"`
	Active              []TaskRef                     `json:"active,omitempty"`
	Waiting             []TaskRef                     `json:"waiting,omitempty"`
	Blocked             []TaskRef                     `json:"blocked,omitempty"`
	ReviewGated         []TaskRef                     `json:"review_gated,omitempty"`
	ReviewComplete      []TaskRef                     `json:"review_complete,omitempty"`
	ReviewDebt          []TaskRef                     `json:"review_debt,omitempty"`
	NotificationGated   []TaskRef                     `json:"notification_gated,omitempty"`
	ReviewWaits         []reviewstate.ReviewWait      `json:"review_waits,omitempty"`
	LiveWindows         []livewindow.Status           `json:"live_windows,omitempty"`
	CompletionHandbacks []completionhandback.Handback `json:"completion_handbacks,omitempty"`
	UtilityGated        []TaskRef                     `json:"utility_gated,omitempty"`
	Readiness           ReadinessExplanation          `json:"readiness"`
	StopConditions      []PlanStopCondition           `json:"stop_conditions,omitempty"`
	Reconcile           reconcile.ActiveReport        `json:"reconcile"`
	SessionCount        int                           `json:"session_count"`
	WatcherCount        int                           `json:"watcher_count"`
	CheckpointCount     int                           `json:"checkpoint_count"`
	WorktreeCount       int                           `json:"worktree_count"`
	WorkBatchCount      int                           `json:"work_batch_count"`
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
	ReviewComplete    int    `json:"review_complete"`
	BatchRecommended  int    `json:"batch_recommended"`
	LoopDetected      int    `json:"loop_detected"`
}

type PlanAction struct {
	Priority           int                                   `json:"priority"`
	Classification     string                                `json:"classification"`
	Action             string                                `json:"action"`
	Reason             string                                `json:"reason"`
	TaskID             string                                `json:"task_id,omitempty"`
	Role               string                                `json:"role,omitempty"`
	SessionID          string                                `json:"session_id,omitempty"`
	WatcherID          string                                `json:"watcher_id,omitempty"`
	BatchKey           string                                `json:"batch_key,omitempty"`
	TaskIDs            []string                              `json:"task_ids,omitempty"`
	ReviewHandback     *ReviewCompletionHandback             `json:"review_handback,omitempty"`
	ReviewNotify       *reviewstate.ReviewNotificationStatus `json:"review_notification,omitempty"`
	ReviewWait         *reviewstate.ReviewWait               `json:"review_wait,omitempty"`
	LiveWindow         *livewindow.Status                    `json:"live_window,omitempty"`
	CompletionHandback *completionhandback.Handback          `json:"completion_handback,omitempty"`
	Stop               bool                                  `json:"stop,omitempty"`
}

type PlanStopCondition struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	TaskID string `json:"task_id,omitempty"`
	Role   string `json:"role,omitempty"`
}

type ReviewCompletionHandback struct {
	TaskID            string                `json:"task_id"`
	Commit            string                `json:"commit,omitempty"`
	ReviewSignature   string                `json:"review_signature,omitempty"`
	RequiredDomains   []string              `json:"required_domains"`
	ApprovedDomains   []string              `json:"approved_domains"`
	MissingDomains    []string              `json:"missing_domains,omitempty"`
	LatestVerdicts    []ReviewDomainVerdict `json:"latest_verdicts"`
	MergeReadyStatus  string                `json:"merge_ready_status"`
	SuggestedCommand  string                `json:"suggested_command"`
	RecommendedAction string                `json:"recommended_action"`
	NotificationState string                `json:"notification_state,omitempty"`
	Blockers          []string              `json:"blockers,omitempty"`
}

type ReviewDomainVerdict struct {
	Domain   string `json:"domain"`
	Reviewer string `json:"reviewer,omitempty"`
	Verdict  string `json:"verdict,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Commit   string `json:"commit,omitempty"`
}

// CompletionHandbackActionsForTask projects only the completion-handback
// actions needed by task detail. It does not build the project coordinator
// plan or mutate workflow state.
func CompletionHandbackActionsForTask(task store.Task, handbacks []completionhandback.Handback, status livewindow.Status, ackTimeout time.Duration, now time.Time) []PlanAction {
	var plan Plan
	for _, handback := range handbacks {
		if completionhandback.IsResolved(handback) {
			continue
		}
		reason := fmt.Sprintf("completion handback %d to %s has no delivered or failed provider notification; task_status=%s completion_state=%s live_window_phase=%s next_action=%s",
			handback.HandoffID,
			handback.ToRole,
			firstNonEmpty(handback.TaskStatus, "unknown"),
			firstNonEmpty(handback.CompletionState, "unspecified"),
			firstNonEmpty(handback.LiveWindowPhase, "none"),
			handback.NextAction,
		)
		action := firstNonEmpty(handback.SuggestedAction, "deliver_or_record_completion_handback")
		if handback.Stale {
			reason += fmt.Sprintf("; stale_age=%s suggested_command=%s", handback.StaleAge, handback.SuggestedCommand)
		}
		addCompletionHandbackAction(&plan, 11, "completion-handback", action, reason, task.Definition.ID, handback.ToRole, handback, true)
	}
	if liveWindowCloseoutPhase(status.Phase) && !closeoutCoveredByCompletionHandback(status, handbacks) {
		owner := firstNonEmpty(status.NextOwner, task.Definition.Role)
		action := "record_closeout_completion_handback"
		reason := fmt.Sprintf("live-window phase=%s needs completion handback to next owner=%s; task_status=%s next_action=%s",
			status.Phase,
			firstNonEmpty(owner, "unknown"),
			firstNonEmpty(task.Status, "unknown"),
			firstNonEmpty(status.NextAction, "record next decision"),
		)
		if staleAge := liveWindowStaleAge(status, ackTimeout, now); staleAge != "" {
			action = "escalate_closeout_completion_handback"
			reason += fmt.Sprintf("; stale_age=%s suggested_command=fairway record completion-handback %s --to %s --next-action %q --completion-state live-window-closeout --state thread_steered --provider <provider> --target <target>",
				staleAge,
				status.TaskID,
				firstNonEmpty(owner, "<role>"),
				firstNonEmpty(status.NextAction, "record next decision"),
			)
		}
		addLiveWindowAction(&plan, 11, "completion-handback", action, reason, status.TaskID, owner, status, true)
	}
	return uniquePlanActions(plan.Actions)
}

type ReviewHandbackOptions struct {
	FreshFor             time.Duration
	Now                  time.Time
	IncludeHistorical    bool
	SuppressAcknowledged bool
	Notifications        []store.Notification
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
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	evidenceByTask, err := s.EvidenceByTaskIDs(ctx, taskIDs)
	if err != nil {
		return Plan{}, err
	}
	handoffsByTask, err := s.HandoffsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return Plan{}, err
	}
	reviewsByTask, err := s.ReviewsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return Plan{}, err
	}
	allNotifications, err := s.Notifications(ctx, "")
	if err != nil {
		return Plan{}, err
	}
	notificationsByTask := make(map[string][]store.Notification)
	for _, notification := range allNotifications {
		notificationsByTask[notification.TaskID] = append(notificationsByTask[notification.TaskID], notification)
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
	ackTimeout, err := notificationAckTimeout(cfg, opts)
	if err != nil {
		return Plan{}, err
	}
	notificationGaps, err := s.HandoffNotificationGapsFiltered(ctx, store.HandoffNotificationGapOptions{
		TerminalStatuses:     cfg.States.Terminal,
		SentStaleBefore:      notificationStaleBefore(ackTimeout),
		ExcludePayloadPrefix: "completion-handback ",
	})
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
	taskByID := map[string]store.Task{}
	for _, task := range tasks {
		taskStatusByID[task.Definition.ID] = task.Status
		taskByID[task.Definition.ID] = task
	}
	if opts.ReadyLimit <= 0 {
		opts.ReadyLimit = 10
	}
	if opts.RecommendationLimit <= 0 {
		opts.RecommendationLimit = 20
	}
	if opts.ReviewHandbackFreshFor <= 0 {
		opts.ReviewHandbackFreshFor = 24 * time.Hour
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
			addAction(&plan, 40, "utility-gated", action, "utility or monitor session is still running", session.TaskID, session.Role, session.ID, "", nil, nil, false)
		}
		if strings.Contains(session.Status, "approval") {
			plan.Summary.ApprovalGated++
			addStop(&plan, "approval-gated", "provider session is waiting on approval", session.TaskID, session.Role)
			addAction(&plan, 5, "approval-gated", "request_or_record_approval", "provider session is waiting on approval", session.TaskID, session.Role, session.ID, "", nil, nil, true)
		}
	}
	liveWindowByTask := map[string]livewindow.Status{}
	for _, status := range livewindow.StatusesFromCheckpoints(checkpoints) {
		liveWindowByTask[status.TaskID] = status
		if terminal[taskStatusByID[status.TaskID]] && !liveWindowCloseoutPhase(status.Phase) {
			continue
		}
		plan.LiveWindows = append(plan.LiveWindows, status)
		if liveWindowCloseoutPhase(status.Phase) {
			continue
		}
		owner := firstNonEmpty(status.NextOwner, taskRole(tasks, status.TaskID))
		action := firstNonEmpty(status.NextAction, "advance_live_window_phase")
		reason := liveWindowActionReason(status)
		addLiveWindowAction(&plan, 13, "live-window", action, reason, status.TaskID, owner, status, liveWindowControlStopPhase(status.Phase))
	}
	completionHandbacksByTask := map[string][]completionhandback.Handback{}
	for _, task := range tasks {
		evidence := evidenceByTask[task.Definition.ID]
		handoffs := handoffsByTask[task.Definition.ID]
		notifications := notificationsByTask[task.Definition.ID]
		liveWindowStatus := liveWindowByTask[task.Definition.ID]
		for _, handback := range completionhandback.RowsWithOptions(task.Definition.ID, handoffs, notifications, completionhandback.RowOptions{
			AckTimeout:      ackTimeout,
			TaskStatus:      task.Status,
			LiveWindowPhase: liveWindowStatus.Phase,
			Superseded:      completionhandback.SupersedesFromEvidence(evidence),
		}) {
			completionHandbacksByTask[task.Definition.ID] = append(completionHandbacksByTask[task.Definition.ID], handback)
			plan.CompletionHandbacks = append(plan.CompletionHandbacks, handback)
			if completionhandback.IsResolved(handback) {
				continue
			}
			plan.Summary.NotificationGated++
			plan.NotificationGated = appendTaskRef(plan.NotificationGated, TaskRef{ID: task.Definition.ID, Role: handback.ToRole, Status: handback.DeliveryStatus})
			reason := fmt.Sprintf("completion handback %d to %s has no delivered or failed provider notification; task_status=%s completion_state=%s live_window_phase=%s next_action=%s",
				handback.HandoffID,
				handback.ToRole,
				firstNonEmpty(handback.TaskStatus, "unknown"),
				firstNonEmpty(handback.CompletionState, "unspecified"),
				firstNonEmpty(handback.LiveWindowPhase, "none"),
				handback.NextAction,
			)
			action := firstNonEmpty(handback.SuggestedAction, "deliver_or_record_completion_handback")
			if handback.Stale {
				reason += fmt.Sprintf("; stale_age=%s suggested_command=%s", handback.StaleAge, handback.SuggestedCommand)
			}
			addCompletionHandbackAction(&plan, 11, "completion-handback", action, reason, task.Definition.ID, handback.ToRole, handback, true)
			addStop(&plan, "completion-handback", reason, task.Definition.ID, handback.ToRole)
		}
	}
	for _, status := range liveWindowByTask {
		if !liveWindowCloseoutPhase(status.Phase) || closeoutCoveredByCompletionHandback(status, completionHandbacksByTask[status.TaskID]) {
			continue
		}
		task := taskByID[status.TaskID]
		owner := firstNonEmpty(status.NextOwner, task.Definition.Role)
		action := "record_closeout_completion_handback"
		statusLabel := "closeout-awaiting-handback"
		staleAge := liveWindowStaleAge(status, ackTimeout, time.Now().UTC())
		reason := fmt.Sprintf("live-window phase=%s needs completion handback to next owner=%s; task_status=%s next_action=%s",
			status.Phase,
			firstNonEmpty(owner, "unknown"),
			firstNonEmpty(task.Status, "unknown"),
			firstNonEmpty(status.NextAction, "record next decision"),
		)
		if staleAge != "" {
			action = "escalate_closeout_completion_handback"
			statusLabel = "stale-closeout-awaiting-handback"
			reason += fmt.Sprintf("; stale_age=%s suggested_command=fairway record completion-handback %s --to %s --next-action %q --completion-state live-window-closeout --state thread_steered --provider <provider> --target <target>",
				staleAge,
				status.TaskID,
				firstNonEmpty(owner, "<role>"),
				firstNonEmpty(status.NextAction, "record next decision"),
			)
		}
		plan.Summary.NotificationGated++
		plan.NotificationGated = appendTaskRef(plan.NotificationGated, TaskRef{ID: status.TaskID, Role: owner, Status: statusLabel})
		addLiveWindowAction(&plan, 11, "completion-handback", action, reason, status.TaskID, owner, status, true)
		addStop(&plan, "completion-handback", reason, status.TaskID, owner)
	}
	for _, watcher := range watchers {
		plan.Summary.UtilityGated++
		plan.UtilityGated = appendTaskRef(plan.UtilityGated, TaskRef{ID: watcher.TaskID, Role: watcher.Owner, Status: watcher.Status})
		action := "wait_for_watcher_handback"
		if opts.UtilityMonitorAllowed {
			action = "continue_configured_utility_monitor"
		}
		addAction(&plan, 40, "utility-gated", action, "watcher or monitor is still active", watcher.TaskID, watcher.Owner, "", watcher.ID, nil, nil, false)
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
		addAction(&plan, priority, classification, finding.Action, finding.Reason, finding.TaskID, finding.Role, finding.SessionID, "", nil, nil, stop)
	}
	for _, gap := range notificationGaps {
		plan.Summary.NotificationGated++
		status := firstNonEmpty(gap.NotificationStatus, "never-sent")
		plan.NotificationGated = appendTaskRef(plan.NotificationGated, TaskRef{ID: gap.TaskID, Role: gap.Role, Status: status})
		reason := fmt.Sprintf("handoff %d to %s has no delivered provider/thread notification; provider_target=%s", gap.HandoffID, gap.Domain, providerTargetSummary(cfg.ProviderTargets, gap.Domain))
		action := "send_or_record_provider_notification"
		if status == "stale-sent" {
			action = "escalate_stale_sent_notification"
			reason = fmt.Sprintf("handoff %d to %s has stale sent notification without acknowledgement or review; provider_target=%s", gap.HandoffID, gap.Domain, providerTargetSummary(cfg.ProviderTargets, gap.Domain))
		}
		if gap.LastState != "" {
			reason += "; latest notification state=" + gap.LastState
		}
		if gap.LastNotificationAt != "" {
			reason += "; last_notification_at=" + gap.LastNotificationAt
		}
		if status != "" {
			reason += "; notification_status=" + status
		}
		reason += "; last_handoff_at=" + gap.LastHandoffAt
		addAction(&plan, 18, "notification-gated", action, reason, gap.TaskID, gap.Domain, "", "", nil, nil, false)
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
			addAction(&plan, 20, "waiting", "resolve_awaiting_input_checkpoint", checkpoint.Summary, checkpoint.TaskID, checkpoint.Owner, "", "", nil, nil, false)
		case "review":
			addAction(&plan, 25, "review-gated", "complete_review_checkpoint", checkpoint.Summary, checkpoint.TaskID, checkpoint.Owner, "", "", nil, nil, false)
		}
	}
	readyRefs := make([]TaskRef, 0, len(ready))
	for _, task := range ready {
		readyRefs = append(readyRefs, taskRef(task))
	}
	plan.Ready = limitTaskRefs(readyRefs, opts.ReadyLimit)
	plan.Summary.Ready = len(ready)
	plan.Readiness = ExplainReadyQueue(tasks, ready, sessions, checkpoints, cfg.States.Terminal)
	if len(ready) == 0 && plan.Readiness.NonReadyTodoCount > 0 {
		reason := fmt.Sprintf("ready queue empty; %d non-ready todo task(s) remain", plan.Readiness.NonReadyTodoCount)
		var taskIDs []string
		if len(plan.Readiness.Blockers) > 0 {
			top := plan.Readiness.Blockers[0]
			reason += fmt.Sprintf("; top blocker %s=%d", top.Category, top.Count)
			taskIDs = top.TaskIDs
		}
		addAction(&plan, 55, "blocked", "inspect_ready_blockers", reason, "", "", "", "", taskIDs, nil, false)
	}
	for _, task := range tasks {
		switch {
		case task.Status == "in_progress":
			plan.Summary.Active++
			plan.Active = appendTaskRef(plan.Active, taskRef(task))
			if _, ok := activeSessionByTask[task.Definition.ID]; ok {
				addAction(&plan, 50, "active", "continue_active_session", "task has an active provider/session attachment", task.Definition.ID, task.Definition.Role, activeSessionByTask[task.Definition.ID].ID, "", nil, nil, false)
			}
		case task.Status == "blocked":
			plan.Summary.Blocked++
			plan.Blocked = appendTaskRef(plan.Blocked, taskRef(task))
			addAction(&plan, 25, "blocked", "resolve_blocked_task", "task is blocked and needs an unblock decision", task.Definition.ID, task.Definition.Role, "", "", nil, nil, false)
		case terminal[task.Status]:
			plan.Summary.Complete++
		}
		detailTask := task
		evidence := evidenceByTask[task.Definition.ID]
		handoffs := handoffsByTask[task.Definition.ID]
		reviews := reviewsByTask[task.Definition.ID]
		reviewPolicy, err := reviewPolicyEvaluation(cfg, detailTask, reviews, taskByID, reviewsByTask)
		if err != nil {
			return Plan{}, err
		}
		if loop := reviewpolicy.DetectLoop(detailTask, reviewPolicy, evidence, reviews); loop.Detected && !terminal[task.Status] {
			plan.Summary.LoopDetected++
			reason := loop.Reason
			if len(loop.FailureChain) > 0 {
				reason += "; failure_chain=" + strings.Join(loop.FailureChain, " | ")
			}
			if len(loop.RealUnknowns) > 0 {
				reason += "; real_unknowns=" + strings.Join(loop.RealUnknowns, "; ")
			}
			if len(loop.RequiredProofBeforeRetry) > 0 {
				reason += "; required_proof_before_retry=" + strings.Join(loop.RequiredProofBeforeRetry, "; ")
			}
			if loop.LighterReviewPlan != "" {
				reason += "; lighter_review_plan=" + loop.LighterReviewPlan
			}
			addAction(&plan, 16, "loop-detected", "recommend_causal_reset", reason, task.Definition.ID, task.Definition.Role, "", "", nil, nil, false)
		}
		if len(task.Definition.ReviewDomains) > 0 && (terminal[task.Status] || task.Status == "review") {
			missing := missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)
			if len(missing) > 0 {
				reason := "missing required review domains: " + strings.Join(missing, ", ")
				if terminal[task.Status] {
					plan.Summary.ReviewDebt++
					plan.ReviewDebt = appendTaskRef(plan.ReviewDebt, taskRef(task))
					addAction(&plan, 45, "review-debt", "sweep_historical_review_debt", reason, task.Definition.ID, task.Definition.Role, "", "", nil, nil, false)
				} else {
					notifications := notificationsByTask[task.Definition.ID]
					statuses := reviewstate.StatusesForTask(detailTask, handoffs, reviews, notifications)
					waits := reviewstate.WaitsForTask(detailTask, handoffs, reviews, notifications, reviewstate.ReviewWaitOptions{
						ProviderTargets: cfg.ProviderTargets,
						ReviewRoutes:    cfg.ReviewRoutes,
						Roles:           cfg.Roles,
						AckTimeout:      ackTimeout,
						Now:             time.Now().UTC(),
						Terminal:        cfg.States.Terminal,
					})
					plan.ReviewWaits = append(plan.ReviewWaits, waits...)
					blockedNotifications := reviewstate.BlockingStatuses(statuses, missing)
					if len(blockedNotifications) > 0 {
						for _, status := range blockedNotifications {
							wait := reviewWaitForDomain(waits, status.Domain)
							plan.Summary.NotificationGated++
							plan.NotificationGated = appendTaskRef(plan.NotificationGated, TaskRef{ID: task.Definition.ID, Role: status.Domain, Status: status.Status})
							reason := fmt.Sprintf("required review domain %s is blocked on reviewer notification status=%s; last_handoff_id=%d last_handoff_at=%s last_notification_state=%s last_notification_at=%s provider=%s target=%s; action=%s",
								status.Domain,
								status.Status,
								status.HandoffID,
								firstNonEmpty(status.LastHandoffAt, "none"),
								firstNonEmpty(status.LastState, "none"),
								firstNonEmpty(status.LastNotificationAt, "none"),
								firstNonEmpty(status.Provider, "none"),
								firstNonEmpty(status.Target, "none"),
								status.SuggestedAction,
							)
							actionName := "deliver_or_retry_review_notification"
							if wait.Action != "" {
								actionName = wait.Action
							}
							addReviewNotificationAction(&plan, 14, "notification-blocked", actionName, reason, task.Definition.ID, task.Definition.Role, status, wait)
							addStop(&plan, "notification-blocked", reason, task.Definition.ID, status.Domain)
						}
						continue
					}
					plan.Summary.ReviewGated++
					plan.ReviewGated = appendTaskRef(plan.ReviewGated, taskRef(task))
					for _, wait := range waits {
						addReviewWaitAction(&plan, 15, "review-gated", wait.Action, reason, task.Definition.ID, task.Definition.Role, wait, true)
					}
					if len(waits) == 0 {
						addAction(&plan, 15, "review-gated", "record_required_reviews", reason, task.Definition.ID, task.Definition.Role, "", "", nil, nil, true)
					}
					addStop(&plan, "review-gated", reason, task.Definition.ID, task.Definition.Role)
				}
			} else {
				notifications := notificationsByTask[task.Definition.ID]
				handback, ok := ReviewHandbackForTask(cfg, detailTask, evidence, handoffs, reviews, ReviewHandbackOptions{FreshFor: opts.ReviewHandbackFreshFor, Now: time.Now().UTC(), SuppressAcknowledged: true, Notifications: notifications})
				if ok {
					if len(handback.Blockers) > 0 {
						reason := "required reviews are approved but merge-ready has non-review blockers: " + strings.Join(handback.Blockers, "; ")
						addAction(&plan, 22, "review-complete-blocked", "resolve_merge_ready_blockers", reason, task.Definition.ID, task.Definition.Role, "", "", nil, &handback, false)
					} else {
						plan.Summary.ReviewComplete++
						plan.ReviewComplete = appendTaskRef(plan.ReviewComplete, taskRef(task))
						addAction(&plan, 12, "review-complete", "run_merge_ready_after_review", handback.RecommendedAction, task.Definition.ID, task.Definition.Role, "", "", nil, &handback, false)
					}
				}
			}
		}
	}
	gitUnavailable := false
	for _, wt := range opts.Worktrees {
		if !wt.GitUnavailable {
			continue
		}
		gitUnavailable = true
		reason := firstNonEmpty(wt.Diagnostic, "git executable unavailable; worktree state cannot be verified")
		addStop(&plan, "worktree-state-deferred", reason, "", wt.Role)
		addAction(&plan, 5, "blocked", "restore_git_visibility_before_dispatch", reason, "", wt.Role, "", "", nil, nil, true)
	}
	if !gitUnavailable {
		for _, group := range relatedReadyGroups(cfg, ready) {
			plan.Summary.BatchRecommended++
			addAction(&plan, 35, "ready", "consider_work_batch", group.Reason, "", "", "", "", group.TaskIDs, nil, false)
		}
		for _, task := range ready {
			addAction(&plan, 60, "ready", "claim_ready_task", "task is ready and no higher-priority stop condition applies", task.Definition.ID, task.Definition.Role, "", "", nil, nil, false)
		}
	}
	for _, wt := range opts.Worktrees {
		if wt.Dirty {
			addStop(&plan, "unsafe-to-infer", "worktree has uncommitted changes", "", wt.Role)
			addAction(&plan, 5, "blocked", "clean_or_commit_worktree_before_dispatch", "worktree has uncommitted changes", "", wt.Role, "", "", nil, nil, true)
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
	Reason  string
}

func relatedReadyGroups(cfg config.Config, tasks []store.Task) []relatedReadyGroup {
	groups := map[string][]string{}
	reasons := map[string]string{}
	for _, task := range tasks {
		eval := reviewpolicy.Evaluate(cfg, reviewpolicy.Options{Task: task})
		reviewDomains := task.Definition.ReviewDomains
		reason := "related ready tasks share role/domain/kind/review surface"
		if eval.Profile != "" {
			reviewDomains = eval.EffectiveDomains
			reason = "review profile " + eval.Profile + " recommends grouped review for related ready tasks"
			if eval.SafeIterationZone {
				reason = "review profile " + eval.Profile + " recommends continuing safe-boundary iteration; reserve full review matrix for boundary exit"
			}
		}
		keyParts := []string{task.Definition.Role, task.Definition.OwningDomain, task.Definition.Kind, strings.Join(reviewDomains, ",")}
		key := strings.Join(keyParts, "\x00")
		if strings.Trim(key, "\x00") == "" {
			continue
		}
		groups[key] = append(groups[key], task.Definition.ID)
		reasons[key] = reason
	}
	var out []relatedReadyGroup
	for key, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		out = append(out, relatedReadyGroup{Key: key, TaskIDs: ids, Reason: reasons[key]})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].TaskIDs) != len(out[j].TaskIDs) {
			return len(out[i].TaskIDs) > len(out[j].TaskIDs)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func reviewPolicyEvaluation(cfg config.Config, task store.Task, reviews []store.Review, tasksByID map[string]store.Task, reviewsByTask map[string][]store.Review) (reviewpolicy.Evaluation, error) {
	var parent *store.Task
	var parentReviews []store.Review
	if strings.TrimSpace(task.Definition.ParentID) != "" {
		if parentTask, ok := tasksByID[task.Definition.ParentID]; ok {
			parent = &parentTask
			parentReviews = reviewsByTask[task.Definition.ParentID]
		}
	}
	return reviewpolicy.Evaluate(cfg, reviewpolicy.Options{
		Task:          task,
		Parent:        parent,
		Reviews:       reviews,
		ParentReviews: parentReviews,
	}), nil
}

func notificationAckTimeout(cfg config.Config, opts PlanOptions) (time.Duration, error) {
	if opts.NotificationAckTimeout > 0 {
		return opts.NotificationAckTimeout, nil
	}
	raw := strings.TrimSpace(cfg.Coordinator.NotificationAckTimeout)
	if raw == "" {
		raw = "24h"
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid coordinator notification_ack_timeout %q: %w", raw, err)
	}
	return timeout, nil
}

func notificationStaleBefore(timeout time.Duration) string {
	if timeout <= 0 {
		return ""
	}
	return time.Now().UTC().Add(-timeout).Format(time.RFC3339Nano)
}

func liveWindowCloseoutPhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case "closeout", "next-decision", "closeout_required":
		return true
	default:
		return false
	}
}

func liveWindowControlStopPhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case "reviews-routed", "approvals-readback", "gate-authorized", "packet_ready", "approvals_ready", "execution_authorized", "closeout_required":
		return true
	default:
		return false
	}
}

func liveWindowActionReason(status livewindow.Status) string {
	reason := fmt.Sprintf("live-window phase=%s next_owner=%s next_action=%s", status.Phase, firstNonEmpty(status.NextOwner, "none"), firstNonEmpty(status.NextAction, "none"))
	if status.TargetCloseBy != "" {
		reason += " deadline=" + status.TargetCloseBy
		if staleAge := liveWindowDeadlineAge(status.TargetCloseBy, time.Now().UTC()); staleAge != "" {
			reason += " deadline_state=missed stale_age=" + staleAge
		}
	}
	if status.AuthorizationState != "" {
		reason += " authorization=" + status.AuthorizationState
	}
	if status.Command != "" {
		reason += " command=" + status.Command
	}
	if status.Prompt != "" {
		reason += " prompt=" + status.Prompt
	}
	if status.MissedDeadlineAction != "" {
		reason += " missed_deadline_action=" + status.MissedDeadlineAction
	}
	return reason
}

func liveWindowDeadlineAge(raw string, now time.Time) string {
	deadline, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil || now.Before(deadline) {
		return ""
	}
	return now.Sub(deadline).Round(time.Second).String()
}

func liveWindowStaleAge(status livewindow.Status, timeout time.Duration, now time.Time) string {
	if timeout <= 0 || strings.TrimSpace(status.CheckpointAt) == "" {
		return ""
	}
	checkpointAt, err := time.Parse(time.RFC3339Nano, status.CheckpointAt)
	if err != nil {
		return ""
	}
	age := now.Sub(checkpointAt)
	if age < timeout {
		return ""
	}
	return age.Truncate(time.Second).String()
}

func closeoutCoveredByCompletionHandback(status livewindow.Status, handbacks []completionhandback.Handback) bool {
	if len(handbacks) == 0 {
		return false
	}
	checkpointAt := strings.TrimSpace(status.CheckpointAt)
	if checkpointAt == "" {
		return false
	}
	for _, handback := range handbacks {
		if strings.TrimSpace(handback.CreatedAt) >= checkpointAt {
			return true
		}
	}
	return false
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

func addAction(plan *Plan, priority int, classification, action, reason, taskID, role, sessionID, watcherID string, taskIDs []string, reviewHandback *ReviewCompletionHandback, stop bool) {
	plan.Actions = append(plan.Actions, PlanAction{Priority: priority, Classification: classification, Action: action, Reason: reason, TaskID: taskID, Role: role, SessionID: sessionID, WatcherID: watcherID, TaskIDs: taskIDs, ReviewHandback: reviewHandback, Stop: stop})
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

func addReviewNotificationAction(plan *Plan, priority int, classification, action, reason, taskID, role string, status reviewstate.ReviewNotificationStatus, wait reviewstate.ReviewWait) {
	addAction(plan, priority, classification, action, reason, taskID, role, "", "", nil, nil, true)
	plan.Actions[len(plan.Actions)-1].ReviewNotify = &status
	if wait.WaitID != "" {
		plan.Actions[len(plan.Actions)-1].ReviewWait = &wait
	}
}

func addReviewWaitAction(plan *Plan, priority int, classification, action, reason, taskID, role string, wait reviewstate.ReviewWait, stop bool) {
	if action == "" {
		action = "record_required_reviews"
	}
	addAction(plan, priority, classification, action, reason, taskID, role, "", "", nil, nil, stop)
	plan.Actions[len(plan.Actions)-1].ReviewWait = &wait
}

func addLiveWindowAction(plan *Plan, priority int, classification, action, reason, taskID, role string, status livewindow.Status, stop bool) {
	addAction(plan, priority, classification, action, reason, taskID, role, "", "", nil, nil, stop)
	plan.Actions[len(plan.Actions)-1].LiveWindow = &status
}

func addCompletionHandbackAction(plan *Plan, priority int, classification, action, reason, taskID, role string, handback completionhandback.Handback, stop bool) {
	addAction(plan, priority, classification, action, reason, taskID, role, "", "", nil, nil, stop)
	plan.Actions[len(plan.Actions)-1].CompletionHandback = &handback
}

func reviewWaitForDomain(waits []reviewstate.ReviewWait, domain string) reviewstate.ReviewWait {
	for _, wait := range waits {
		if wait.Domain == domain {
			return wait
		}
	}
	return reviewstate.ReviewWait{}
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

func taskRole(tasks []store.Task, taskID string) string {
	for _, task := range tasks {
		if task.Definition.ID == taskID {
			return task.Definition.Role
		}
	}
	return ""
}

func ReviewHandbackForTask(cfg config.Config, task store.Task, evidence []store.Evidence, handoffs []store.Handoff, reviews []store.Review, opts ReviewHandbackOptions) (ReviewCompletionHandback, bool) {
	required := normalizedUnique(task.Definition.ReviewDomains)
	if len(required) == 0 {
		return ReviewCompletionHandback{}, false
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if opts.FreshFor <= 0 {
		opts.FreshFor = 24 * time.Hour
	}
	latest := latestReviewVerdicts(required, reviews)
	approved := make([]string, 0, len(required))
	for _, domain := range required {
		if verdict, ok := latest[domain]; ok && verdict.Verdict == "approve" {
			approved = append(approved, domain)
		}
	}
	if len(approved) != len(required) {
		return ReviewCompletionHandback{}, false
	}
	if opts.SuppressAcknowledged && reviewHandbackAcknowledged(evidence) {
		return ReviewCompletionHandback{}, false
	}
	if !opts.IncludeHistorical && reviewHandbackIsHistorical(task, evidence, reviews, opts) {
		return ReviewCompletionHandback{}, false
	}
	orderedVerdicts := orderedReviewVerdicts(required, latest)
	commit := strings.TrimSpace(task.CommitSHA)
	if commit == "" {
		commit = latestReviewCommit(orderedVerdicts)
	}
	handback := ReviewCompletionHandback{
		TaskID:            task.Definition.ID,
		Commit:            commit,
		RequiredDomains:   required,
		ApprovedDomains:   approved,
		MissingDomains:    []string{},
		LatestVerdicts:    orderedVerdicts,
		MergeReadyStatus:  "review_complete_next_merge_ready_check",
		SuggestedCommand:  fmt.Sprintf("fairway merge-ready %s", task.Definition.ID),
		RecommendedAction: fmt.Sprintf("run fairway merge-ready %s, then perform the configured coordinator merge/push/release action if it passes", task.Definition.ID),
	}
	handback.ReviewSignature = reviewHandbackSignature(handback)
	if opts.SuppressAcknowledged {
		if state, ok := reviewHandbackNotificationAcknowledged(opts.Notifications, handback); ok {
			return ReviewCompletionHandback{}, false
		} else if state != "" {
			handback.NotificationState = state
		}
	}
	if cfg.Gates.RequireEvidenceBeforeDone && len(evidence) == 0 {
		handback.Blockers = append(handback.Blockers, "missing evidence")
	}
	if cfg.Gates.RequireHandoffBeforeMergeReady && len(handoffs) == 0 {
		handback.Blockers = append(handback.Blockers, "missing handoff")
	}
	if len(handback.Blockers) > 0 {
		handback.MergeReadyStatus = "blocked_by_non_review_gate"
		handback.RecommendedAction = "resolve non-review merge-ready blockers before coordinator merge/push/release action"
	}
	return handback, true
}

func latestReviewCommit(verdicts []ReviewDomainVerdict) string {
	for _, verdict := range verdicts {
		if strings.TrimSpace(verdict.Commit) != "" {
			return strings.TrimSpace(verdict.Commit)
		}
	}
	return ""
}

func reviewHandbackNotificationAcknowledged(notifications []store.Notification, handback ReviewCompletionHandback) (string, bool) {
	signature := strings.TrimSpace(handback.ReviewSignature)
	if signature == "" {
		signature = reviewHandbackSignature(handback)
	}
	latestState := ""
	for _, notification := range notifications {
		if !reviewHandbackNotificationStateAcknowledges(notification.State) {
			continue
		}
		latestState = notification.State
		reason := strings.TrimSpace(notification.Reason)
		if signature != "" && notificationReasonHasReviewSignature(reason, signature) {
			return notification.State, true
		}
	}
	return latestState, false
}

func notificationReasonHasReviewSignature(reason, signature string) bool {
	if strings.TrimSpace(signature) == "" {
		return false
	}
	want := "review_signature=" + strings.TrimSpace(signature)
	for _, field := range strings.Fields(reason) {
		if strings.Trim(field, ",;") == want {
			return true
		}
	}
	return strings.Trim(reason, ",;") == want
}

func reviewHandbackNotificationStateAcknowledges(state string) bool {
	switch strings.TrimSpace(state) {
	case "acknowledged", "notification_delivered", "thread_steered", "review_recorded":
		return true
	default:
		return false
	}
}

func reviewHandbackSignature(handback ReviewCompletionHandback) string {
	var parts []string
	if strings.TrimSpace(handback.Commit) != "" {
		parts = append(parts, "commit="+strings.TrimSpace(handback.Commit))
	}
	for _, verdict := range handback.LatestVerdicts {
		parts = append(parts, verdict.Domain+"="+verdict.Verdict+"@"+verdict.Commit)
	}
	return strings.Join(parts, "|")
}

func reviewHandbackAcknowledged(evidence []store.Evidence) bool {
	for _, ev := range evidence {
		switch strings.TrimSpace(ev.ArtifactType) {
		case "review-handback-ack", "merge-ready", "push-intent", "lane-closeout":
			return true
		}
	}
	return false
}

func reviewHandbackIsHistorical(task store.Task, evidence []store.Evidence, reviews []store.Review, opts ReviewHandbackOptions) bool {
	if reviewHandbackHasClosureEvidence(evidence) {
		return true
	}
	completedAt, err := time.Parse(time.RFC3339Nano, task.CompletedAt)
	if err != nil {
		return false
	}
	if strings.TrimSpace(task.CommitSHA) != "" {
		return true
	}
	return opts.Now.Sub(completedAt) > opts.FreshFor
}

func reviewHandbackHasClosureEvidence(evidence []store.Evidence) bool {
	for _, ev := range evidence {
		switch strings.TrimSpace(ev.ArtifactType) {
		case "push-intent", "lane-closeout", "release-run", "release-verify":
			return true
		}
	}
	return false
}

func latestReviewVerdicts(required []string, reviews []store.Review) map[string]ReviewDomainVerdict {
	requiredSet := map[string]bool{}
	for _, domain := range required {
		requiredSet[domain] = true
	}
	latest := map[string]ReviewDomainVerdict{}
	for _, review := range reviews {
		domain := strings.TrimSpace(firstNonEmpty(review.Domain, review.Reviewer))
		if domain == "" || !requiredSet[domain] {
			continue
		}
		latest[domain] = ReviewDomainVerdict{
			Domain:   domain,
			Reviewer: strings.TrimSpace(review.Reviewer),
			Verdict:  strings.TrimSpace(review.Verdict),
			Reason:   strings.TrimSpace(review.Reason),
			Commit:   strings.TrimSpace(review.Commit),
		}
	}
	return latest
}

func orderedReviewVerdicts(required []string, latest map[string]ReviewDomainVerdict) []ReviewDomainVerdict {
	out := make([]ReviewDomainVerdict, 0, len(required))
	for _, domain := range required {
		verdict, ok := latest[domain]
		if !ok {
			verdict = ReviewDomainVerdict{Domain: domain}
		}
		out = append(out, verdict)
	}
	return out
}

func normalizedUnique(values []string) []string {
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
