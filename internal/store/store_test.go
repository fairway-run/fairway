package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestClaim_AllowsExactlyOneWinner(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Race", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.Claim(ctx, "T-001", "backend", "")
		}()
	}
	wg.Wait()
	close(errs)

	var wins, claimed int
	for err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrAlreadyClaimed):
			claimed++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 || claimed != 1 {
		t.Fatalf("wins=%d claimed=%d, want 1/1", wins, claimed)
	}
}

func TestMigrate_RecordsAppliedMigration(t *testing.T) {
	s := newTestStore(t)
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=1`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration count=%d, want 1", count)
	}
}

func TestTaskDetail_AllowsNullMetadataAfterMigration(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Old row", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE task_definitions SET source_paths=NULL, target_paths=NULL, review_domains=NULL WHERE id='T-001'`); err != nil {
		t.Fatal(err)
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Definition.SourcePaths) != 0 || len(task.Definition.TargetPaths) != 0 || len(task.Definition.ReviewDomains) != 0 {
		t.Fatalf("metadata arrays=%v/%v/%v, want empty", task.Definition.SourcePaths, task.Definition.TargetPaths, task.Definition.ReviewDomains)
	}
}

func TestPostgresCompatReport(t *testing.T) {
	report, err := PostgresCompatReport()
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("findings=%+v, want ok", report.Findings)
	}
	if len(report.Files) == 0 {
		t.Fatal("expected at least one migration file")
	}
}

func TestRecordReview_MaterializesTaskState(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Review", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", Review{Reviewer: "ui", Verdict: "changes", Reason: "needs tests"}); err != nil {
		t.Fatal(err)
	}

	task, _, _, _, reviews, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if task.ReviewStatus != "changes_requested" {
		t.Fatalf("review status=%q, want changes_requested", task.ReviewStatus)
	}
	if len(reviews) != 1 || reviews[0].Verdict != "changes" {
		t.Fatalf("reviews=%+v, want one changes review", reviews)
	}
}

func TestRecordReview_RejectsSelfReview(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Review", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	err := s.RecordReview(ctx, "T-001", Review{Reviewer: "backend", Verdict: "approve", Reason: "self"})
	if err == nil {
		t.Fatal("expected self-review error")
	}
}

func TestSetStatus_BlockedReleasesClaim(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Blocked", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "blocked", "waiting", true); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", ""); err != nil {
		t.Fatalf("claim after blocked failed: %v", err)
	}
}

func TestSetStatus_DoneReleasesClaimForReopen(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "todo", "reopen", false); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", ""); err != nil {
		t.Fatalf("claim after reopen failed: %v", err)
	}
}

func TestImportTasks_RejectsInvalidID(t *testing.T) {
	s := newTestStore(t)
	err := s.ImportTasks(context.Background(), []TaskDefinition{{ID: "task-1", Title: "bad", Role: "backend"}})
	if !errors.Is(err, ErrInvalidTaskID) {
		t.Fatalf("err=%v, want ErrInvalidTaskID", err)
	}
}

func TestImportTasks_RejectsDuplicateID(t *testing.T) {
	s := newTestStore(t)
	err := s.ImportTasks(context.Background(), []TaskDefinition{
		{ID: "T-001", Title: "one", Role: "backend"},
		{ID: "T-001", Title: "two", Role: "backend"},
	})
	if err == nil {
		t.Fatal("expected duplicate task id error")
	}
}

func TestImportTasks_RejectsUnknownDependency(t *testing.T) {
	s := newTestStore(t)
	err := s.ImportTasks(context.Background(), []TaskDefinition{
		{ID: "T-001", Title: "one", Role: "backend", Dependencies: []string{"T-404"}},
	})
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
}

func TestUpdateTask_RejectsParentCycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{
		{ID: "T-001", Title: "root", Role: "backend"},
		{ID: "T-002", ParentID: "T-001", Title: "child", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	err := s.UpdateTask(ctx, TaskDefinition{ID: "T-001", ParentID: "T-002", Title: "root", Role: "backend"})
	if err == nil {
		t.Fatal("expected parent cycle error")
	}
}

func TestHealth_CountsUnacknowledgedHandoff(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Handoff", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordHandoff(ctx, "T-001", Handoff{ToRole: "ui", Payload: "please check"}); err != nil {
		t.Fatal(err)
	}
	health, err := s.Health(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if health.UnacknowledgedHandoff != 1 {
		t.Fatalf("handoffs=%d, want 1", health.UnacknowledgedHandoff)
	}
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	pid := 123
	if err := s.UpsertSession(ctx, Session{ID: "backend-123", Role: "backend", Branch: "agent/backend", Status: "running", PID: &pid}); err != nil {
		t.Fatal(err)
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != "backend-123" || sessions[0].PID == nil || *sessions[0].PID != pid {
		t.Fatalf("sessions=%+v, want live backend session", sessions)
	}
	if err := s.EndSession(ctx, "backend-123", "ended", "normal", nil); err != nil {
		t.Fatal(err)
	}
	sessions, err = s.Sessions(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("live sessions=%+v, want none", sessions)
	}
}

func TestCheckpointLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Checkpoint", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, Checkpoint{TaskID: "T-001", State: "active", Owner: "backend", TargetCloseBy: "2026-01-01", Summary: "working"}); err != nil {
		t.Fatal(err)
	}
	checkpoints, err := s.Checkpoints(ctx, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 1 || checkpoints[0].TaskID != "T-001" {
		t.Fatalf("checkpoints=%+v, want T-001", checkpoints)
	}
	stale, err := s.Checkpoints(ctx, "2026-02-01", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 {
		t.Fatalf("stale=%+v, want one stale checkpoint", stale)
	}
}

func TestReady_UsesConfiguredTerminalStates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{
		{ID: "T-001", Title: "Dep", Role: "backend"},
		{ID: "T-002", Title: "Ready", Role: "backend", Dependencies: []string{"T-001"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "failed", "", false); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.Ready(ctx, "backend", []string{"done", "failed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Definition.ID != "T-002" {
		t.Fatalf("ready=%+v, want T-002", tasks)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
