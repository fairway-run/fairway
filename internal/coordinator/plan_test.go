package coordinator

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/config"
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

func TestBuildPlanSegmentsTerminalReviewCheckpointAsReviewDebt(t *testing.T) {
	ctx := context.Background()
	s := openPlanStore(t, ctx)
	cfg := config.Defaults(t.TempDir())
	cfg.States.Terminal = []string{"done"}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "DONE-001", Title: "Historical review", Kind: "task", Role: "backend"}}); err != nil {
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
		t.Fatalf("terminal review checkpoint summary=%+v, want review_debt only", plan.Summary)
	}
	if hasPlanAction(plan, "review-gated", "complete_review_checkpoint", "DONE-001") {
		t.Fatalf("terminal review checkpoint surfaced as active review gate: %+v", plan.Actions)
	}
	if !hasPlanAction(plan, "review-debt", "sweep_historical_review_debt", "DONE-001") {
		t.Fatalf("terminal review checkpoint did not surface as review debt: %+v", plan.Actions)
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
	for _, candidate := range plan.Actions {
		if candidate.Classification == classification && candidate.Action == action && (taskID == "" || candidate.TaskID == taskID) {
			return true
		}
	}
	return false
}

func hasPlanActionReason(plan Plan, classification, taskID, contains string) bool {
	for _, action := range plan.Actions {
		if action.Classification == classification && action.TaskID == taskID && strings.Contains(action.Reason, contains) {
			return true
		}
	}
	return false
}
