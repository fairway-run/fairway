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

func newReconcileTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
