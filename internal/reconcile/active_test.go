package reconcile

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/store"
)

func TestActiveMonitorSessionWithBackingAutomationIsClean(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Monitor CI", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "ops", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{
		ID:           "ci-monitor",
		Role:         "ops/watch",
		TaskID:       "T-001",
		Status:       "running",
		MonitorKind:  "ci",
		AutomationID: "gha-monitor-123",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.MonitorSessionsNoProof != 0 {
		t.Fatalf("report=%+v, want clean automation-backed monitor", report)
	}
}

func TestActiveMonitorSessionWithoutBackingProofIsReported(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Monitor CI", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "ops", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{
		ID:          "ci-monitor",
		Role:        "ops/watch",
		TaskID:      "T-001",
		Status:      "running",
		MonitorKind: "ci",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	assertMonitorNoProof(t, report, "T-001", "ci-monitor")
}

func TestActiveMonitorSessionWithExpiredManualCheckpointIsReported(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Manual CI monitor", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "ops", ""); err != nil {
		t.Fatal(err)
	}
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "T-001", State: "active", Owner: "ops/watch", TargetCloseBy: yesterday, Summary: "manual CI watch until yesterday"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{
		ID:          "manual-monitor",
		Role:        "ops/watch",
		TaskID:      "T-001",
		Status:      "running",
		MonitorKind: "ci",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	assertMonitorNoProof(t, report, "T-001", "manual-monitor")
}

func TestActiveMonitorCompletionWithReadyWorkReportsResumeNeeded(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Monitor CI", Role: "ops"},
		{ID: "T-002", Title: "Next branch push", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "done", "monitor complete", false); err != nil {
		t.Fatal(err)
	}
	if err := s.StartWatcher(ctx, store.Watcher{ID: "W-CI", TaskID: "T-001", Owner: "ops/watch", Process: "ci", Command: "gh run watch"}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishWatcher(ctx, "W-CI", "pass", "ci.log", nil, "CI monitor passed"); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("report OK, want resume-needed finding: %+v", report)
	}
	if report.Summary.MonitorResumeNeeded != 1 {
		t.Fatalf("monitor resume needed=%d, want 1 in %+v", report.Summary.MonitorResumeNeeded, report)
	}
	for _, finding := range report.Findings {
		if finding.Kind == "monitor_completion_resume_needed" && finding.TaskID == "T-002" && finding.CompletedMonitorID == "W-CI" && finding.ReadyTaskCount == 1 {
			return
		}
	}
	t.Fatalf("missing resume-needed finding in %+v", report.Findings)
}

func TestActiveMonitorCompletionWithNoReadyWorkIsClean(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Monitor CI", Role: "ops"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "done", "monitor complete", false); err != nil {
		t.Fatal(err)
	}
	if err := s.StartWatcher(ctx, store.Watcher{ID: "W-CI", TaskID: "T-001", Owner: "ops/watch", Process: "ci", Command: "gh run watch"}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishWatcher(ctx, "W-CI", "pass", "ci.log", nil, "CI monitor passed"); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.MonitorResumeNeeded != 0 {
		t.Fatalf("report=%+v, want clean when no ready work remains", report)
	}
}

func TestActiveMonitorCompletionFollowedByNextSessionIsClean(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Monitor CI", Role: "ops"},
		{ID: "T-002", Title: "Next branch push", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "done", "monitor complete", false); err != nil {
		t.Fatal(err)
	}
	if err := s.StartWatcher(ctx, store.Watcher{ID: "W-CI", TaskID: "T-001", Owner: "ops/watch", Process: "ci", Command: "gh run watch"}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishWatcher(ctx, "W-CI", "pass", "ci.log", nil, "CI monitor passed"); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-002", "backend", "agent/backend"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "branch-push", Role: "backend", TaskID: "T-002", Status: "running", SessionBackend: "shell"}); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.MonitorResumeNeeded != 0 {
		t.Fatalf("report=%+v, want clean when next branch/session has resumed", report)
	}
}

func TestActiveProviderSessionWithoutStartedCheckpointIsReported(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Provider work", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "T-001", State: "active", Owner: "backend", Summary: "generic active checkpoint without provider session identity"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{
		ID:             "codex-thread-123",
		Role:           "backend",
		TaskID:         "T-001",
		Status:         "running",
		SessionBackend: "codex-thread",
		Provider:       "codex",
		SessionName:    "thread-123",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	assertProviderLifecycleMissing(t, report, "T-001", "codex-thread-123", "thread-123", "active")
}

func TestActiveProviderSessionWithStartedCheckpointIsClean(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Provider work", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{
		ID:             "codex-thread-123",
		Role:           "backend",
		TaskID:         "T-001",
		Status:         "running",
		SessionBackend: "codex-thread",
		Provider:       "codex",
		SessionName:    "thread-123",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "T-001", State: "active", Owner: "backend", Summary: "Provider session codex-thread-123 started: implementation"}); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.ProviderLifecycleMissing != 0 {
		t.Fatalf("report=%+v, want provider lifecycle clean", report)
	}
}

func TestActiveProviderSessionWaitingAndFailedCheckpointAreClean(t *testing.T) {
	for _, tc := range []struct {
		name          string
		sessionStatus string
		summary       string
	}{
		{name: "waiting", sessionStatus: "running", summary: "Provider session codex-thread-123 waiting on input: need decision"},
		{name: "failed", sessionStatus: "failed", summary: "Provider session codex-thread-123 failed: command failed"},
		{name: "stale", sessionStatus: "stale", summary: "Provider session codex-thread-123 stale: no progress"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := newReconcileTestStore(t)
			if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Provider work", Role: "backend"}}); err != nil {
				t.Fatal(err)
			}
			if err := s.Claim(ctx, "T-001", "backend", ""); err != nil {
				t.Fatal(err)
			}
			if err := s.UpsertSession(ctx, store.Session{
				ID:             "codex-thread-123",
				Role:           "backend",
				TaskID:         "T-001",
				Status:         tc.sessionStatus,
				SessionBackend: "codex-thread",
				Provider:       "codex",
				SessionName:    "thread-123",
			}); err != nil {
				t.Fatal(err)
			}
			if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "T-001", State: "awaiting_input", Owner: "backend", Summary: tc.summary}); err != nil {
				t.Fatal(err)
			}
			report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
			if err != nil {
				t.Fatal(err)
			}
			if !report.OK || report.Summary.ProviderLifecycleMissing != 0 {
				t.Fatalf("report=%+v, want provider lifecycle clean for %s", report, tc.sessionStatus)
			}
		})
	}
}

func TestActiveProviderSessionCompletedCheckpointIsClean(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Provider work", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{
		ID:             "codex-thread-123",
		Role:           "backend",
		TaskID:         "T-001",
		Status:         "running",
		SessionBackend: "codex-thread",
		Provider:       "codex",
		SessionName:    "thread-123",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "T-001", State: "done", Owner: "backend", Summary: "Provider session codex-thread-123 completed: implementation"}); err != nil {
		t.Fatal(err)
	}
	report, err := Active(ctx, s, ActiveOptions{Terminal: []string{"done"}, StaleCheckpointAfter: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.ProviderLifecycleMissing != 0 {
		t.Fatalf("report=%+v, want provider lifecycle clean for completed session", report)
	}
}

func assertMonitorNoProof(t *testing.T, report ActiveReport, taskID, sessionID string) {
	t.Helper()
	if report.OK {
		t.Fatalf("report OK, want monitor proof finding: %+v", report)
	}
	if report.Summary.MonitorSessionsNoProof != 1 {
		t.Fatalf("monitor no proof=%d, want 1 in %+v", report.Summary.MonitorSessionsNoProof, report)
	}
	for _, finding := range report.Findings {
		if finding.Kind == "monitor_session_without_backing_proof" && finding.TaskID == taskID && finding.SessionID == sessionID && finding.Action == "mark_session_stale" {
			return
		}
	}
	t.Fatalf("missing monitor proof finding for %s/%s in %+v", taskID, sessionID, report.Findings)
}

func assertProviderLifecycleMissing(t *testing.T, report ActiveReport, taskID, sessionID, externalID, expected string) {
	t.Helper()
	if report.OK {
		t.Fatalf("report OK, want provider lifecycle finding: %+v", report)
	}
	if report.Summary.ProviderLifecycleMissing != 1 {
		t.Fatalf("provider lifecycle missing=%d, want 1 in %+v", report.Summary.ProviderLifecycleMissing, report)
	}
	for _, finding := range report.Findings {
		if finding.Kind == "provider_session_missing_lifecycle_checkpoint" &&
			finding.TaskID == taskID &&
			finding.SessionID == sessionID &&
			finding.ExternalSessionID == externalID &&
			finding.ExpectedCheckpoint == expected &&
			finding.Action == "record_provider_event_checkpoint" {
			return
		}
	}
	t.Fatalf("missing provider lifecycle finding for %s/%s in %+v", taskID, sessionID, report.Findings)
}

func newReconcileTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
