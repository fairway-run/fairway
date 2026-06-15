package coordinator

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/completionhandback"
	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/livewindow"
	"github.com/subashram/fairway/internal/store"
)

func TestBuildPlanClassifiesReadyCompleteReviewAndApproval(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "READY-001", Title: "Ready work", Kind: "task", Role: "backend", OwningDomain: "api", ReviewDomains: []string{"backend"}},
		{ID: "READY-002", Title: "Related ready work", Kind: "task", Role: "backend", OwningDomain: "api", ReviewDomains: []string{"backend"}},
		{ID: "DONE-001", Title: "Complete work", Kind: "task", Role: "backend"},
		{ID: "REVIEW-001", Title: "Review gated work", Kind: "task", Role: "ui", ReviewDomains: []string{"arch"}},
		{ID: "APPROVAL-001", Title: "Approval gated work", Kind: "task", Role: "ops"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DONE-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "REVIEW-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "APPROVAL-001", State: "awaiting_input", Owner: "ops", Summary: "waiting on approval to deploy"}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Ready != 3 || plan.Summary.Complete != 2 {
		t.Fatalf("summary ready/complete = %+v", plan.Summary)
	}
	if plan.Summary.ReviewDebt != 1 || plan.Summary.ReviewGated != 0 {
		t.Fatalf("review_debt/review_gated=%d/%d, want 1/0", plan.Summary.ReviewDebt, plan.Summary.ReviewGated)
	}
	if plan.Summary.ApprovalGated != 1 || len(plan.StopConditions) == 0 {
		t.Fatalf("approval gating not surfaced: summary=%+v stops=%+v", plan.Summary, plan.StopConditions)
	}
	if !hasPlanAction(plan, "ready", "consider_work_batch", "") {
		t.Fatalf("expected batch recommendation in %+v", plan.Actions)
	}
	if !hasPlanAction(plan, "review-debt", "sweep_historical_review_debt", "REVIEW-001") {
		t.Fatalf("expected review-debt action in %+v", plan.Actions)
	}
}

func TestBuildPlanRecommendsSafeIterationGroupedReview(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	cfg.ReviewProfiles = []config.ReviewProfile{{
		Name:                  "micro-slice",
		MatchTags:             []string{"review:micro"},
		RequiredReviewDomains: []string{"governance"},
		SafeIterationZone:     true,
		GroupReview:           true,
	}}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "SAFE-001", Title: "Harness setup", Kind: "task", Role: "backend", OwningDomain: "harness", Tags: []string{"review:micro"}},
		{ID: "SAFE-002", Title: "Harness readback", Kind: "task", Role: "backend", OwningDomain: "harness", Tags: []string{"review:micro"}},
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !hasPlanAction(plan, "ready", "consider_work_batch", "") {
		t.Fatalf("expected grouped review recommendation in %+v", plan.Actions)
	}
	found := false
	for _, action := range plan.Actions {
		if action.Action == "consider_work_batch" && strings.Contains(action.Reason, "continuing safe-boundary iteration") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected safe iteration reason in %+v", plan.Actions)
	}
}

func TestBuildPlanRecommendsCausalResetForLoopDetected(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	cfg.ReviewProfiles = []config.ReviewProfile{{
		Name:                     "micro-slice",
		MatchTags:                []string{"review:micro"},
		RequiredReviewDomains:    []string{"governance"},
		SafeIterationZone:        true,
		SafeIterationDefectClass: "harness",
		SafeIterationControl:     "non-live disposable boundary",
	}}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "LOOP-001", Title: "Repeated harness fix", Kind: "task", Role: "backend", OwningDomain: "harness", Tags: []string{"review:micro"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "LOOP-001", store.Evidence{CommandText: "near-ready harness readback", Result: "pass", ArtifactType: "harness"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "LOOP-001", store.Evidence{CommandText: "browser smoke", Result: "fail", ArtifactType: "harness", Notes: "first harness launch failure"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "LOOP-001", store.Evidence{CommandText: "browser smoke retry", Result: "blocked", ArtifactType: "harness", Notes: "same harness launch failure"}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.LoopDetected != 1 {
		t.Fatalf("loop summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
	if !hasPlanAction(plan, "loop-detected", "recommend_causal_reset", "LOOP-001") {
		t.Fatalf("expected causal reset action in %+v", plan.Actions)
	}
	if !hasPlanActionReason(plan, "loop-detected", "LOOP-001", "required_proof_before_retry") {
		t.Fatalf("expected proof-before-retry reason in %+v", plan.Actions)
	}
	if err := s.SetStatus(ctx, "LOOP-001", "done", "historical loop closed", false); err != nil {
		t.Fatal(err)
	}
	plan, err = BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.LoopDetected != 0 || hasPlanAction(plan, "loop-detected", "recommend_causal_reset", "LOOP-001") {
		t.Fatalf("terminal historical loop should not become coordinator next action: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
}

func TestBuildPlanExplainsEmptyReadyQueue(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "DEP-001", Title: "Dependency", Kind: "task", Role: "backend"},
		{ID: "TODO-001", Title: "Blocked todo", Kind: "task", Role: "backend", Dependencies: []string{"DEP-001"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DEP-001", "in_progress", "", false); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Readiness.ClaimableCount != 0 || plan.Readiness.NonReadyTodoCount != 1 {
		t.Fatalf("readiness=%+v", plan.Readiness)
	}
	if len(plan.Readiness.Blockers) != 1 || plan.Readiness.Blockers[0].Category != "dependency-blocked" {
		t.Fatalf("blockers=%+v", plan.Readiness.Blockers)
	}
	if !hasPlanAction(plan, "blocked", "inspect_ready_blockers", "") {
		t.Fatalf("expected empty-ready blocker action in %+v", plan.Actions)
	}
}

func TestBuildPlanClassifiesStaleSessionUtilityAndDryRunNoMutation(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "ACTIVE-001", Title: "Active work", Kind: "task", Role: "backend"},
		{ID: "UTILITY-001", Title: "CI monitor", Kind: "task", Role: "ops"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "ACTIVE-001", "in_progress", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "ended-task-session", Role: "backend", Provider: "codex", SessionBackend: "codex", TaskID: "ACTIVE-001", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "ACTIVE-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "ci-monitor", Role: "ops/watch", SessionBackend: "ci-monitor", Provider: "shell", TaskID: "UTILITY-001", MonitorKind: "ci", ExternalRunID: "gha-1", PollCommand: "gh run view gha-1", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	before, err := s.CurrentStatus(ctx, "UTILITY-001")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20, UtilityMonitorAllowed: true})
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.CurrentStatus(ctx, "UTILITY-001")
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("dry-run plan mutated task status: before=%s after=%s", before, after)
	}
	if plan.Summary.Stale == 0 || !hasPlanAction(plan, "stale", "mark_session_stale", "ACTIVE-001") {
		t.Fatalf("stale session not surfaced: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
	if plan.Summary.UtilityGated == 0 || !hasPlanAction(plan, "utility-gated", "continue_configured_utility_monitor", "UTILITY-001") {
		t.Fatalf("utility monitor not surfaced: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
}

func TestBuildPlanReportsHandoffWithoutDeliveredNotification(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.ProviderTargets = []config.ProviderTarget{{Domain: "security", Provider: "codex", Target: "019-security", Type: "thread"}}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "REVIEW-001", Title: "Needs review", Kind: "task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordHandoff(ctx, "REVIEW-001", store.Handoff{ToRole: "security", Payload: "please review"}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.NotificationGated != 1 || !hasPlanAction(plan, "notification-gated", "send_or_record_provider_notification", "REVIEW-001") {
		t.Fatalf("notification gap not surfaced: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
	if !hasPlanActionReason(plan, "notification-gated", "REVIEW-001", "provider_target=codex:019-security(thread)") ||
		!hasPlanActionReason(plan, "notification-gated", "REVIEW-001", "last_handoff_at=") {
		t.Fatalf("notification gap did not include target and handoff timing: actions=%+v", plan.Actions)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "REVIEW-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "sent"}); err != nil {
		t.Fatal(err)
	}
	plan, err = BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.NotificationGated != 0 || hasPlanAction(plan, "notification-gated", "send_or_record_provider_notification", "REVIEW-001") {
		t.Fatalf("sent notification still surfaced as gap: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
	time.Sleep(time.Millisecond)
	plan, err = BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20, NotificationAckTimeout: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.NotificationGated != 1 || !hasPlanAction(plan, "notification-gated", "escalate_stale_sent_notification", "REVIEW-001") {
		t.Fatalf("stale sent notification not escalated: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
	if !hasPlanActionReason(plan, "notification-gated", "REVIEW-001", "notification_status=stale-sent") {
		t.Fatalf("stale notification reason missing status: actions=%+v", plan.Actions)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "REVIEW-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "acknowledged"}); err != nil {
		t.Fatal(err)
	}
	plan, err = BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20, NotificationAckTimeout: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.NotificationGated != 0 {
		t.Fatalf("acknowledged notification surfaced as gap: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "REVR-001", Title: "Review recorded", Kind: "task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordHandoff(ctx, "REVR-001", store.Handoff{ToRole: "security", Payload: "please review"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "REVR-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "review_recorded"}); err != nil {
		t.Fatal(err)
	}
	plan, err = BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20, NotificationAckTimeout: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	if hasPlanAction(plan, "notification-gated", "send_or_record_provider_notification", "REVR-001") {
		t.Fatalf("review-recorded notification surfaced as gap: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
}

func TestBuildPlanReportsPendingCompletionHandback(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "PENDING-001", Title: "Pending handback", Kind: "task", Role: "backend"},
		{ID: "DELIVERED-001", Title: "Delivered handback", Kind: "task", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	pendingPayload, err := completionhandback.RenderPayload("schedule retry window", []string{"packet.md"}, "review only")
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.RecordHandoffWithID(ctx, "PENDING-001", store.Handoff{ToRole: "ops", Payload: pendingPayload})
	if err != nil {
		t.Fatal(err)
	}
	deliveredPayload, err := completionhandback.RenderPayload("resume control loop", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := s.RecordHandoffWithID(ctx, "DELIVERED-001", store.Handoff{ToRole: "ops", Payload: deliveredPayload})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "DELIVERED-001", HandoffID: &delivered.ID, Domain: "ops", Provider: "codex", Target: "thread-ops", State: "thread_steered"}); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.CompletionHandbacks) != 2 {
		t.Fatalf("completion handbacks=%+v", plan.CompletionHandbacks)
	}
	if !hasPlanAction(plan, "completion-handback", "deliver_or_record_completion_handback", "PENDING-001") {
		t.Fatalf("pending completion handback action missing: %+v", plan.Actions)
	}
	if plan.OK || len(plan.StopConditions) == 0 {
		t.Fatalf("pending completion handback should stop coordinator: ok=%t stops=%+v", plan.OK, plan.StopConditions)
	}
	if hasPlanAction(plan, "notification-gated", "send_or_record_provider_notification", "PENDING-001") {
		t.Fatalf("completion handback also produced generic notification action: %+v", plan.Actions)
	}
	if hasPlanAction(plan, "completion-handback", "deliver_or_record_completion_handback", "DELIVERED-001") {
		t.Fatalf("delivered completion handback produced action: %+v", plan.Actions)
	}
	if !hasPlanActionReason(plan, "completion-handback", "PENDING-001", "completion handback "+strconv.FormatInt(pending.ID, 10)) {
		t.Fatalf("pending handback reason missing handoff id: %+v", plan.Actions)
	}
}

func TestBuildPlanBlocksReviewWaitOnMissingReviewerNotification(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "HANDOFF-001", Title: "Handoff only", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "FAILED-001", Title: "Failed notification", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "DELIVERED-001", Title: "Delivered notification", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "ACK-001", Title: "Acknowledged notification", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "REVIEWED-001", Title: "Review recorded", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, taskID := range []string{"HANDOFF-001", "FAILED-001", "DELIVERED-001", "ACK-001", "REVIEWED-001"} {
		if err := s.SetStatus(ctx, taskID, "review", "needs review", false); err != nil {
			t.Fatal(err)
		}
		if err := s.RecordHandoff(ctx, taskID, store.Handoff{ToRole: "arch", Payload: "please review"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "FAILED-001", Domain: "arch", Provider: "codex", Target: "thread-arch", State: "notification_failed", Reason: "thread steering tool unavailable"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "DELIVERED-001", Domain: "arch", Provider: "codex", Target: "thread-arch", State: "notification_delivered"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "ACK-001", Domain: "arch", Provider: "codex", Target: "thread-arch", State: "review_acknowledged"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "REVIEWED-001", store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "changes", Reason: "needs fix"}); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		taskID string
		action string
	}{
		{"HANDOFF-001", "record_delivery_proof"},
		{"FAILED-001", "mapping_required"},
	} {
		action, ok := planAction(plan, "notification-blocked", tc.action, tc.taskID)
		if !ok {
			t.Fatalf("%s did not produce notification-blocked %s action: %+v", tc.taskID, tc.action, plan.Actions)
		}
		if action.ReviewNotify == nil || !action.ReviewNotify.Blocking || action.ReviewWait == nil {
			t.Fatalf("%s missing review notification/wait payload: %+v", tc.taskID, action)
		}
		if hasPlanAction(plan, "review-gated", "record_required_reviews", tc.taskID) {
			t.Fatalf("%s also produced normal review wait: %+v", tc.taskID, plan.Actions)
		}
	}
	for _, taskID := range []string{"DELIVERED-001", "ACK-001", "REVIEWED-001"} {
		if hasPlanAction(plan, "notification-blocked", "deliver_or_retry_review_notification", taskID) {
			t.Fatalf("%s produced unexpected notification block: %+v", taskID, plan.Actions)
		}
		if !hasPlanClassification(plan, "review-gated", taskID) {
			t.Fatalf("%s did not produce normal review wait: %+v", taskID, plan.Actions)
		}
	}
	if len(plan.ReviewWaits) == 0 {
		t.Fatalf("plan did not expose review wait rows")
	}
}

func TestBuildPlanFiltersHistoricalTerminalNotificationGaps(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "DONE-001", Title: "Historical done handoff", Kind: "task", Role: "backend"},
		{ID: "DONEREVIEW-001", Title: "Terminal pending review handoff", Kind: "task", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordHandoff(ctx, "DONE-001", store.Handoff{ToRole: "security", Payload: "please review"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordHandoff(ctx, "DONEREVIEW-001", store.Handoff{ToRole: "security", Payload: "please review"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DONE-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.RouteReview(ctx, "DONEREVIEW-001", "security", "terminal task still needs review notification"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DONEREVIEW-001", "done", "", false); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReadyLimit: 10, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if hasPlanAction(plan, "notification-gated", "send_or_record_provider_notification", "DONE-001") {
		t.Fatalf("historical terminal handoff surfaced as notification gap: %+v", plan.Actions)
	}
	if !hasPlanAction(plan, "notification-gated", "send_or_record_provider_notification", "DONEREVIEW-001") {
		t.Fatalf("terminal pending-review handoff did not surface as gap: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
}

func TestBuildPlanAllWorkCompleteIsIdle(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "DONE-001", Title: "Complete", Kind: "task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DONE-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Complete != 1 || plan.Summary.TopClassification != "idle" {
		t.Fatalf("all-complete plan = %+v", plan.Summary)
	}
}

func TestBuildPlanIgnoresAwaitingInputSupersededByDoneCheckpoint(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "DONE-001", Title: "Complete after waiver", Kind: "task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "DONE-001", State: "awaiting_input", Owner: "backend", Summary: "waiting on approval before push"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "DONE-001", State: "done", Owner: "backend", Summary: "approval resolved and pushed"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DONE-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.ApprovalGated != 0 || plan.Summary.Waiting != 0 {
		t.Fatalf("superseded checkpoint still gated plan: summary=%+v stops=%+v actions=%+v", plan.Summary, plan.StopConditions, plan.Actions)
	}
	for _, stop := range plan.StopConditions {
		if stop.TaskID == "DONE-001" {
			t.Fatalf("superseded checkpoint produced stop condition: %+v", plan.StopConditions)
		}
	}
}

func TestBuildPlanSurfacesLiveWindowPhase(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "LIVE-001", Title: "Repeated live drill", Kind: "task", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{
		TaskID:        "LIVE-001",
		State:         "active",
		Owner:         "ops",
		TargetCloseBy: "2026-06-13T03:15:00Z",
		Summary:       "live-window phase=gate-running next_owner=ops next_action=run_browser_smoke",
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	action, ok := planAction(plan, "live-window", "run browser smoke", "LIVE-001")
	if !ok {
		t.Fatalf("live-window action missing: %+v", plan.Actions)
	}
	if action.LiveWindow == nil || action.LiveWindow.Phase != "gate-running" || action.LiveWindow.NextOwner != "ops" || action.LiveWindow.TargetCloseBy != "2026-06-13T03:15:00Z" {
		t.Fatalf("live-window detail missing: %+v", action)
	}
}

func TestBuildPlanSurfacesMissedLiveOperationExecutionHandoff(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "MFA-1320", Title: "MFA drill window", Kind: "task", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	summary, err := livewindow.SummaryWithOptions(livewindow.SummaryOptions{
		Phase:                "approvals_ready",
		NextOwner:            "architecture-control",
		NextAction:           "authorize operator handoff",
		AuthorizationState:   "approvals recorded; execution not authorized",
		Command:              "fairway live-window record MFA-1320 --phase execution_authorized",
		MissedDeadlineAction: "escalate to architecture control and reschedule window",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{
		TaskID:        "MFA-1320",
		State:         "awaiting_input",
		Owner:         "architecture-control",
		TargetCloseBy: deadline,
		Summary:       summary,
		ArtifactPath:  ".fairway/artifacts/mfa-1320/packet.md",
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	action, ok := planAction(plan, "live-window", "authorize operator handoff", "MFA-1320")
	if !ok {
		t.Fatalf("live operation handoff action missing: %+v", plan.Actions)
	}
	if action.LiveWindow == nil || action.LiveWindow.AuthorizationState == "" || action.LiveWindow.MissedDeadlineAction == "" {
		t.Fatalf("live operation control fields missing: %+v", action)
	}
	for _, want := range []string{"deadline_state=missed", "authorization=approvals recorded; execution not authorized", "missed_deadline_action=escalate to architecture control and reschedule window"} {
		if !strings.Contains(action.Reason, want) {
			t.Fatalf("action reason missing %q: %s", want, action.Reason)
		}
	}
	if plan.OK {
		t.Fatalf("missed execution handoff should stop coordinator: %+v", plan.StopConditions)
	}
}

func TestBuildPlanSurfacesStaleLiveWindowCloseoutHandbackGap(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "DRILL-001", Title: "MFA drill", Kind: "task", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DRILL-001", "blocked", "browser smoke failed; follow-up created", false); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{
		TaskID:       "DRILL-001",
		State:        "awaiting_input",
		Owner:        "arch",
		Summary:      "live-window phase=closeout next_owner=arch next_action=assign_macos_launch_fix",
		ArtifactPath: "final_drill_blocked_summary_2355.md",
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{NotificationAckTimeout: time.Nanosecond, StaleCheckpointAfter: time.Hour, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	action, ok := planAction(plan, "completion-handback", "escalate_closeout_completion_handback", "DRILL-001")
	if !ok {
		t.Fatalf("closeout handback gap missing: actions=%+v", plan.Actions)
	}
	if action.LiveWindow == nil || action.LiveWindow.Phase != "closeout" || action.Role != "arch" {
		t.Fatalf("closeout action missing live-window detail: %+v", action)
	}
	if !strings.Contains(action.Reason, "stale_age=") || !strings.Contains(action.Reason, "suggested_command=fairway record completion-handback DRILL-001") {
		t.Fatalf("closeout action reason missing stale handback guidance: %s", action.Reason)
	}
}

func TestBuildPlanSurfacesStaleCompletionHandbackForBlockedFollowUp(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "DRILL-001", Title: "MFA drill", Kind: "task", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DRILL-001", "blocked", "browser smoke failed; follow-up created", false); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{
		TaskID:       "DRILL-001",
		State:        "awaiting_input",
		Owner:        "arch",
		Summary:      "live-window phase=closeout next_owner=arch next_action=assign_macos_launch_fix",
		ArtifactPath: "final_drill_blocked_summary_2355.md",
	}); err != nil {
		t.Fatal(err)
	}
	payload, err := completionhandback.RenderPayloadWithState("assign HARNESS-FIX-MFA-BROWSER-SMOKE-MACOS-LAUNCH-001", "blocked-with-follow-up", []string{"rollback-proof.md"}, "operator closeout only")
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := s.RecordHandoffWithID(ctx, "DRILL-001", store.Handoff{ToRole: "arch", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "DRILL-001", HandoffID: &handoff.ID, Domain: "arch", State: "handoff_recorded"}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{NotificationAckTimeout: time.Nanosecond, StaleCheckpointAfter: time.Hour, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	action, ok := planAction(plan, "completion-handback", "escalate_completion_handback", "DRILL-001")
	if !ok {
		t.Fatalf("stale completion handback missing: actions=%+v", plan.Actions)
	}
	if action.CompletionHandback == nil {
		t.Fatalf("completion handback detail missing: %+v", action)
	}
	row := action.CompletionHandback
	if row.CompletionState != "blocked-with-follow-up" || row.TaskStatus != "blocked" || row.LiveWindowPhase != "closeout" || !row.Stale {
		t.Fatalf("completion handback context missing: %+v", row)
	}
	if hasPlanAction(plan, "completion-handback", "escalate_closeout_completion_handback", "DRILL-001") {
		t.Fatalf("explicit handback should suppress closeout fallback: %+v", plan.Actions)
	}
}

func TestBuildPlanDoesNotLetHistoricalCompletionHandbackHideNewCloseout(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "DRILL-001", Title: "MFA drill", Kind: "task", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	historicalPayload, err := completionhandback.RenderPayloadWithState("schedule prior drill", "live-window-next-decision", nil, "prior closeout")
	if err != nil {
		t.Fatal(err)
	}
	historical, err := s.RecordHandoffWithID(ctx, "DRILL-001", store.Handoff{ToRole: "arch", Payload: historicalPayload})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "DRILL-001", HandoffID: &historical.ID, Domain: "arch", Provider: "codex", Target: "thread-arch", State: "thread_steered"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{
		TaskID:       "DRILL-001",
		State:        "awaiting_input",
		Owner:        "arch",
		Summary:      "live-window phase=closeout next_owner=arch next_action=assign_macos_launch_fix",
		ArtifactPath: "final_drill_blocked_summary_2355.md",
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{NotificationAckTimeout: time.Nanosecond, StaleCheckpointAfter: time.Hour, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := planAction(plan, "completion-handback", "escalate_closeout_completion_handback", "DRILL-001"); !ok {
		t.Fatalf("historical handback suppressed new closeout gap: actions=%+v handbacks=%+v", plan.Actions, plan.CompletionHandbacks)
	}
}

func TestBuildPlanSegmentsTerminalMissingReviewDomainsAsReviewDebt(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "DONE-001", Title: "Historical review", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "DONE-001", State: "review", Owner: "backend", Summary: "old review checkpoint"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DONE-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.ReviewGated != 0 || plan.Summary.ReviewDebt != 1 {
		t.Fatalf("terminal missing review domain summary=%+v, want review_debt only", plan.Summary)
	}
	if hasPlanAction(plan, "review-gated", "complete_review_checkpoint", "DONE-001") {
		t.Fatalf("terminal review checkpoint surfaced as active review gate: %+v", plan.Actions)
	}
	if !hasPlanAction(plan, "review-debt", "sweep_historical_review_debt", "DONE-001") {
		t.Fatalf("terminal missing review domain did not surface as review debt: %+v", plan.Actions)
	}
}

func TestBuildPlanIgnoresTerminalReviewCheckpointWhenReviewsApproved(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "DONE-001", Title: "Reviewed historical task", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "DONE-001", State: "review", Owner: "backend", Summary: "old review checkpoint"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "DONE-001", store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "approve", Reason: "reviewed"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DONE-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.ReviewGated != 0 || plan.Summary.ReviewDebt != 0 {
		t.Fatalf("approved terminal review checkpoint still produced review debt: summary=%+v actions=%+v", plan.Summary, plan.Actions)
	}
	if hasPlanAction(plan, "review-debt", "sweep_historical_review_debt", "DONE-001") {
		t.Fatalf("approved terminal review checkpoint surfaced as debt: %+v", plan.Actions)
	}
}

func TestBuildPlanReviewCompletionHandback(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "PARTIAL-001", Title: "Partial reviews", Kind: "task", Role: "backend", ReviewDomains: []string{"arch", "governance"}},
		{ID: "CHANGES-001", Title: "Changes then approved", Kind: "task", Role: "backend", ReviewDomains: []string{"arch", "governance"}},
		{ID: "OPENCHANGES-001", Title: "Unresolved changes", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"PARTIAL-001", "CHANGES-001", "OPENCHANGES-001"} {
		if err := s.SetStatus(ctx, id, "done", "", false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RecordReview(ctx, "PARTIAL-001", store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "approve", Reason: "arch ok"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "CHANGES-001", store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "changes", Reason: "needs fix"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "CHANGES-001", store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "approve", Reason: "fixed"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "CHANGES-001", store.Review{Reviewer: "gov-reviewer", Domain: "governance", Verdict: "approve", Reason: "governance ok", Commit: "abc123"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "OPENCHANGES-001", store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "approve", Reason: "initial ok"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "OPENCHANGES-001", store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "changes", Reason: "later blocker"}); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if hasPlanAction(plan, "review-complete", "run_merge_ready_after_review", "PARTIAL-001") {
		t.Fatalf("partial approvals produced review-complete handback: %+v", plan.Actions)
	}
	action, ok := planAction(plan, "review-complete", "run_merge_ready_after_review", "CHANGES-001")
	if !ok {
		t.Fatalf("changes-then-approved task missing review-complete handback: %+v", plan.Actions)
	}
	if action.ReviewHandback == nil {
		t.Fatalf("review-complete action missing handback: %+v", action)
	}
	if got := strings.Join(action.ReviewHandback.RequiredDomains, ","); got != "arch,governance" {
		t.Fatalf("required domains=%q", got)
	}
	if got := strings.Join(action.ReviewHandback.ApprovedDomains, ","); got != "arch,governance" {
		t.Fatalf("approved domains=%q", got)
	}
	if action.ReviewHandback.MergeReadyStatus != "review_complete_next_merge_ready_check" {
		t.Fatalf("merge-ready status=%q", action.ReviewHandback.MergeReadyStatus)
	}
	if action.ReviewHandback.Commit != "abc123" {
		t.Fatalf("commit=%q, want abc123", action.ReviewHandback.Commit)
	}
	if action.ReviewHandback.SuggestedCommand != "fairway merge-ready CHANGES-001" {
		t.Fatalf("suggested command=%q", action.ReviewHandback.SuggestedCommand)
	}
	if len(action.ReviewHandback.MissingDomains) != 0 {
		t.Fatalf("missing domains=%v, want none", action.ReviewHandback.MissingDomains)
	}
	if !strings.Contains(action.Reason, "fairway merge-ready CHANGES-001") {
		t.Fatalf("review-complete reason missing merge-ready command: %+v", action)
	}
	if hasPlanAction(plan, "review-complete", "run_merge_ready_after_review", "OPENCHANGES-001") {
		t.Fatalf("unresolved changes-requested verdict produced review-complete handback: %+v", plan.Actions)
	}
}

func TestBuildPlanReviewCompletionBlockedByNonReviewGate(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	cfg.Gates.RequireEvidenceBeforeDone = true
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "RBLOCK-001", Title: "Approved but missing evidence", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "RBLOCK-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "RBLOCK-001", store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "approve", Reason: "review ok"}); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if hasPlanAction(plan, "review-complete", "run_merge_ready_after_review", "RBLOCK-001") {
		t.Fatalf("missing evidence produced false review-complete handback: %+v", plan.Actions)
	}
	action, ok := planAction(plan, "review-complete-blocked", "resolve_merge_ready_blockers", "RBLOCK-001")
	if !ok {
		t.Fatalf("missing evidence did not produce blocked handback action: %+v", plan.Actions)
	}
	if action.ReviewHandback == nil || action.ReviewHandback.MergeReadyStatus != "blocked_by_non_review_gate" {
		t.Fatalf("blocked action missing handback status: %+v", action)
	}
	if len(action.ReviewHandback.Blockers) != 1 || action.ReviewHandback.Blockers[0] != "missing evidence" {
		t.Fatalf("blockers=%v, want missing evidence", action.ReviewHandback.Blockers)
	}
}

func TestBuildPlanSuppressesStaleReviewCompletionHandbacks(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	tasks := []store.TaskDefinition{
		{ID: "FRESH-001", Title: "Fresh review", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "PUSHED-001", Title: "Pushed review", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "OLD-001", Title: "Old review", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "ACK-001", Title: "Acknowledged review", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "NOTIFY-001", Title: "Delivered review", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "AMENDED-001", Title: "Amended review", Kind: "task", Role: "backend", ReviewDomains: []string{"arch"}},
		{ID: "RESET-001", Title: "Review set reset", Kind: "task", Role: "backend", ReviewDomains: []string{"arch", "ops"}},
	}
	if err := s.ImportTasks(ctx, tasks); err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.ID == "FRESH-001" || task.ID == "NOTIFY-001" || task.ID == "AMENDED-001" || task.ID == "RESET-001" {
			if err := s.SetStatus(ctx, task.ID, "review", "fresh review-complete handback", false); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := s.SetStatusWithCommit(ctx, task.ID, "done", "done", strings.ToLower(task.ID[:3])+"123", false); err != nil {
				t.Fatal(err)
			}
		}
		reviewCommit := ""
		switch task.ID {
		case "NOTIFY-001":
			reviewCommit = "not123"
		case "AMENDED-001":
			reviewCommit = "new123"
		case "RESET-001":
			reviewCommit = "same123"
		}
		if err := s.RecordReview(ctx, task.ID, store.Review{Reviewer: "arch-reviewer", Domain: "arch", Verdict: "approve", Reason: "review ok", Commit: reviewCommit}); err != nil {
			t.Fatal(err)
		}
		if task.ID == "RESET-001" {
			if err := s.RecordReview(ctx, task.ID, store.Review{Reviewer: "ops-reviewer", Domain: "ops", Verdict: "approve", Reason: "review ok", Commit: reviewCommit}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.RecordEvidence(ctx, "PUSHED-001", store.Evidence{CommandText: "fairway record push-intent PUSHED-001", Result: "pass", ArtifactType: "push-intent"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "ACK-001", store.Evidence{CommandText: "fairway coordinator acknowledged review handback", Result: "pass", ArtifactType: "review-handback-ack"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "NOTIFY-001", Domain: "coordinator", Provider: "codex", Target: "control-thread", State: "notification_delivered", Reason: "review_complete review_signature=commit=not123|arch=approve@not123"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "AMENDED-001", Domain: "coordinator", Provider: "codex", Target: "control-thread", State: "thread_steered", Reason: "review_complete review_signature=commit=old123|arch=approve@old123"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "RESET-001", Domain: "coordinator", Provider: "codex", Target: "control-thread", State: "notification_delivered", Reason: "review_complete review_signature=commit=same123|arch=approve@same123 commit=same123"}); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(ctx, cfg, s, PlanOptions{StaleCheckpointAfter: time.Hour, ReviewHandbackFreshFor: time.Nanosecond, RecommendationLimit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !hasPlanAction(plan, "review-complete", "run_merge_ready_after_review", "FRESH-001") {
		t.Fatalf("fresh review-complete handback missing: %+v", plan.Actions)
	}
	for _, suppressed := range []string{"PUSHED-001", "OLD-001", "ACK-001", "NOTIFY-001"} {
		if hasPlanAction(plan, "review-complete", "run_merge_ready_after_review", suppressed) {
			t.Fatalf("suppressed task %s produced review-complete action: %+v", suppressed, plan.Actions)
		}
	}
	if !hasPlanAction(plan, "review-complete", "run_merge_ready_after_review", "AMENDED-001") {
		t.Fatalf("amended commit should reset stale notification suppression: %+v", plan.Actions)
	}
	if !hasPlanAction(plan, "review-complete", "run_merge_ready_after_review", "RESET-001") {
		t.Fatalf("changed review set on same commit should reset notification suppression: %+v", plan.Actions)
	}
}

func openPlanStore(t *testing.T, ctx context.Context) *store.Store {
	t.Helper()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func hasPlanAction(plan Plan, classification, action, taskID string) bool {
	_, ok := planAction(plan, classification, action, taskID)
	return ok
}

func hasPlanClassification(plan Plan, classification, taskID string) bool {
	for _, candidate := range plan.Actions {
		if candidate.Classification == classification && (taskID == "" || candidate.TaskID == taskID) {
			return true
		}
	}
	return false
}

func planAction(plan Plan, classification, action, taskID string) (PlanAction, bool) {
	for _, candidate := range plan.Actions {
		if candidate.Classification == classification && candidate.Action == action && (taskID == "" || candidate.TaskID == taskID) {
			return candidate, true
		}
	}
	return PlanAction{}, false
}

func hasPlanActionReason(plan Plan, classification, taskID, contains string) bool {
	for _, action := range plan.Actions {
		if action.Classification == classification && action.TaskID == taskID && strings.Contains(action.Reason, contains) {
			return true
		}
	}
	return false
}
