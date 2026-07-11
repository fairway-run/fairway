package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestStartWorkAtomicallyRecordsTaskSessionAndCheckpoint(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Common path", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	result, err := s.StartWork(ctx, "T-001", "backend", "main", Session{ID: "work-t-001", Provider: "codex", SessionBackend: "codex-thread", Status: "running"}, "Started common path in work-t-001", []string{"done"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "in_progress" || result.SessionID != "work-t-001" || result.CheckpointID == 0 || result.AlreadyActive {
		t.Fatalf("result=%+v", result)
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil || task.Status != "in_progress" || task.Owner != "backend" {
		t.Fatalf("task=%+v err=%v", task, err)
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil || len(sessions) != 1 || sessions[0].TaskID != "T-001" {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil || len(checkpoints) != 1 || checkpoints[0].State != "active" {
		t.Fatalf("checkpoints=%+v err=%v", checkpoints, err)
	}
	second, err := s.StartWork(ctx, "T-001", "backend", "main", Session{ID: "work-t-001", Provider: "codex", SessionBackend: "codex-thread"}, "Resumed common path", []string{"done"})
	if err != nil || !second.AlreadyActive {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if second.CheckpointID != result.CheckpointID {
		t.Fatalf("repeat created checkpoint %d, want reused %d", second.CheckpointID, result.CheckpointID)
	}
	if err := s.SetStatus(ctx, "T-001", "blocked", "blocked for test", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartWork(ctx, "T-001", "backend", "main", Session{ID: "blocked"}, "must fail", []string{"done"}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("blocked start err=%v, want invalid transition", err)
	}
}

func TestStartWorkFailsClosedOnUnfinishedDependency(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{
		{ID: "T-001", Title: "Dependency", Role: "backend"},
		{ID: "T-002", Title: "Blocked child", Role: "backend", Dependencies: []string{"T-001"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartWork(ctx, "T-002", "backend", "main", Session{ID: "work-t-002"}, "start work-t-002", []string{"done"}); !errors.Is(err, ErrInvalidTransition) || !strings.Contains(err.Error(), "dependency T-001 is todo") {
		t.Fatalf("dependency error=%v", err)
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-002")
	if err != nil || task.Status != "todo" {
		t.Fatalf("task=%+v err=%v", task, err)
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
}

func TestCloseWorkAtomicallyEndsTaskAndSession(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Close", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartWork(ctx, "T-001", "backend", "main", Session{ID: "work-t-001", Provider: "codex", SessionBackend: "codex-thread"}, "Started close test", []string{"done"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CloseWork(ctx, "T-001", "other-session", "abc123", "done"); err == nil {
		t.Fatal("close with missing session succeeded")
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil || task.Status != "in_progress" {
		t.Fatalf("failed close mutated task=%+v err=%v", task, err)
	}
	if err := s.UpsertSession(ctx, Session{ID: "work-t-001-second", Role: "backend", TaskID: "T-001", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CloseWork(ctx, "T-001", "work-t-001", "abc123", "done"); err == nil || !strings.Contains(err.Error(), "exactly one active session") {
		t.Fatalf("multiple-session close error=%v", err)
	}
	if err := s.EndSession(ctx, "work-t-001-second", "ended", "test cleanup", nil); err != nil {
		t.Fatal(err)
	}
	result, err := s.CloseWork(ctx, "T-001", "work-t-001", "abc123", "gates passed")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "done" || result.CommitSHA != "abc123" {
		t.Fatalf("result=%+v", result)
	}
	task, _, _, _, _, err = s.TaskDetail(ctx, "T-001")
	if err != nil || task.Status != "done" || task.CommitSHA != "abc123" {
		t.Fatalf("task=%+v err=%v", task, err)
	}
	sessions, err := s.Sessions(ctx, true)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	var closed Session
	for _, session := range sessions {
		if session.ID == "work-t-001" {
			closed = session
		}
	}
	if closed.Status != "ended" || closed.EndReason != "gates passed" {
		t.Fatalf("closed session=%+v", closed)
	}
}

func TestTaskDecisionsRecordAssessAndSupersede(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Decision", Role: "backend"}, {ID: "T-002", Title: "Other", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", "main"); err != nil {
		t.Fatal(err)
	}
	first, err := s.RecordTaskDecision(ctx, TaskDecision{
		TaskID: "T-001", Decision: "Centralize session authorization", Trigger: "A handler-only check left a bypass path",
		Alternatives: []string{"Patch the handler", "Centralize session lookup"}, Chosen: "Centralize session lookup",
		Reason: "All handlers then share one authorization boundary", ScopeAdded: []string{"internal/session"}, Risk: "Shared session behavior changes",
		ValidationRefs: []string{"go test ./internal/session"}, FactRefs: []string{"evidence:42"}, CreatedBy: "backend",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 || first.QualityState != "draft" {
		t.Fatalf("first=%+v", first)
	}
	if err := s.AssessTaskDecision(ctx, "T-001", first.ID, "accepted", "backend", "self review"); err == nil || !strings.Contains(err.Error(), "own task decision") {
		t.Fatalf("self assessment error=%v", err)
	}
	if err := s.AssessTaskDecision(ctx, "T-001", first.ID, "accepted", "arch", "Concrete and consistent with the cited fact"); err != nil {
		t.Fatal(err)
	}
	second, err := s.RecordTaskDecision(ctx, TaskDecision{
		TaskID: "T-001", Decision: "Centralize authorization and preserve compatibility", Trigger: "Compatibility coverage found a legacy caller",
		Alternatives: []string{"Remove the caller", "Add a compatibility adapter"}, Chosen: "Add a compatibility adapter",
		Reason: "The adapter preserves the shared boundary without breaking the caller", Risk: "Temporary adapter debt",
		ValidationRefs: []string{"compatibility regression"}, FactRefs: []string{"commit:abc123"}, SupersedesID: first.ID, CreatedBy: "backend",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AssessTaskDecision(ctx, "T-001", second.ID, "insufficient", "arch", "Missing removal date for the temporary adapter"); err != nil {
		t.Fatal(err)
	}
	decisions, err := s.TaskDecisions(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 2 || decisions[0].QualityState != "superseded" || decisions[0].SupersededByID != second.ID || decisions[0].QualityReviewer != "arch" || decisions[1].QualityState != "insufficient" || decisions[1].QualityReviewer != "arch" {
		t.Fatalf("decisions=%+v", decisions)
	}
	duplicate := second
	duplicate.ID = 0
	duplicate.Decision = "Another replacement"
	if _, err := s.RecordTaskDecision(ctx, duplicate); err == nil || !strings.Contains(err.Error(), "already has a replacement") {
		t.Fatalf("duplicate supersession error=%v", err)
	}
	crossTask := second
	crossTask.ID = 0
	crossTask.TaskID = "T-002"
	if _, err := s.RecordTaskDecision(ctx, crossTask); err == nil || !strings.Contains(err.Error(), "another task") {
		t.Fatalf("cross-task supersession error=%v", err)
	}
}

func TestTaskDecisionsRejectPrivateDataAndAuthorityClaims(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Decision", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	base := TaskDecision{TaskID: "T-001", Decision: "Choose bounded storage", Trigger: "A durable record is needed", Alternatives: []string{"Evidence", "Dedicated rows"}, Chosen: "Dedicated rows", Reason: "Structured projection is required", Risk: "Schema migration", ValidationRefs: []string{"store tests"}, FactRefs: []string{"task:T-001"}}
	private := base
	private.Reason = "transcript: provider-private text"
	if _, err := s.RecordTaskDecision(ctx, private); err == nil || !strings.Contains(err.Error(), "private-data") {
		t.Fatalf("private-data error=%v", err)
	}
	authority := base
	authority.Decision = "Authorize the release"
	if _, err := s.RecordTaskDecision(ctx, authority); err == nil || !strings.Contains(err.Error(), "authority") {
		t.Fatalf("authority error=%v", err)
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

func TestReviewsOrderEqualTimestampsByDurableID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-900", Title: "Review order", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-900", Review{Reviewer: "arch", Domain: "arch", Verdict: "changes", Reason: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-900", Review{Reviewer: "arch", Domain: "arch", Verdict: "approve", Reason: "second"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE task_reviews SET created_at='2026-07-11T00:00:00Z' WHERE project_id=? AND task_id=?`, s.projectID, "T-900"); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, reviews, err := s.TaskDetail(ctx, "T-900")
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 2 || reviews[0].Verdict != "changes" || reviews[1].Verdict != "approve" {
		t.Fatalf("task detail reviews=%+v", reviews)
	}
	byTask, err := s.ReviewsByTaskIDs(ctx, []string{"T-900"})
	if err != nil {
		t.Fatal(err)
	}
	if got := byTask["T-900"]; len(got) != 2 || got[0].Verdict != "changes" || got[1].Verdict != "approve" {
		t.Fatalf("batched reviews=%+v", got)
	}
}

func TestSchemaVersionsReadsWithoutMigrating(t *testing.T) {
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "missing.db")
	applied, available, err := SchemaVersions(ctx, missing)
	if err != nil {
		t.Fatal(err)
	}
	if applied != 0 || available < 13 {
		t.Fatalf("missing schema versions = applied=%d available=%d", applied, available)
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("SchemaVersions created missing DB: %v", err)
	}

	path := filepath.Join(t.TempDir(), "state.db")
	s, err := Open(ctx, path, "schema-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	applied, available, err = SchemaVersions(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if applied != available || applied < 13 {
		t.Fatalf("migrated schema versions = applied=%d available=%d", applied, available)
	}
}

func TestTaskDetail_AllowsNullMetadataAfterMigration(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Old row", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE task_definitions SET source_paths=NULL, target_paths=NULL, review_domains=NULL, tags=NULL WHERE id='T-001'`); err != nil {
		t.Fatal(err)
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Definition.SourcePaths) != 0 || len(task.Definition.TargetPaths) != 0 || len(task.Definition.ReviewDomains) != 0 || len(task.Definition.Tags) != 0 {
		t.Fatalf("metadata arrays=%v/%v/%v/%v, want empty", task.Definition.SourcePaths, task.Definition.TargetPaths, task.Definition.ReviewDomains, task.Definition.Tags)
	}
}

func TestTasksFilteredByTagsPreservesImportedOrder(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{
		{ID: "T-001", Title: "Production docs", Role: "backend", Tags: []string{"production-readiness", "environment:cloudflare"}},
		{ID: "T-002", Title: "Staging UAT", Role: "backend", Tags: []string{"uat-hardening", "environment:staging"}},
		{ID: "T-003", Title: "Untagged", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(task.Definition.Tags, ","); got != "production-readiness,environment:cloudflare" {
		t.Fatalf("tags=%q, want imported order", got)
	}
	filtered, err := s.TasksFiltered(ctx, TaskFilterOptions{Tags: []string{"production-readiness", "environment:cloudflare"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Definition.ID != "T-001" {
		t.Fatalf("filtered=%+v, want T-001 only", filtered)
	}
	noTags, err := s.TasksFiltered(ctx, TaskFilterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(noTags) != 3 {
		t.Fatalf("no-tag filter returned %d tasks, want 3", len(noTags))
	}
}

func TestTaskDetail_AllowsEvidenceWithoutArtifact(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Evidence", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", Evidence{CommandText: "go test ./...", Result: "pass"}); err != nil {
		t.Fatal(err)
	}
	_, _, evidence, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].ArtifactPath != "" || evidence[0].Result != "pass" {
		t.Fatalf("evidence=%+v, want one artifact-less pass row", evidence)
	}
}

func TestBatchTaskFactsByTaskIDs(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{
		{ID: "T-001", Title: "One", Role: "backend"},
		{ID: "T-002", Title: "Two", Role: "backend"},
		{ID: "T-003", Title: "Three", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", Evidence{CommandText: "first", Result: "pass", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", Evidence{CommandText: "second", Result: "fail", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-003", Evidence{CommandText: "not requested", Result: "pass"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", Review{Reviewer: "alice", Domain: "backend", Verdict: "approve", Reason: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-002", Review{Reviewer: "bob", Domain: "ops", Verdict: "changes", Reason: "fix"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-003", Review{Reviewer: "carol", Domain: "ui", Verdict: "approve", Reason: "not requested"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "in_progress", "started", false); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordHandoff(ctx, "T-002", Handoff{FromRole: "backend", ToRole: "ops", Payload: "verify"}); err != nil {
		t.Fatal(err)
	}

	evidence, err := s.EvidenceByTaskIDs(ctx, []string{"T-001", "T-001", "", "T-002"})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(evidence["T-001"]); got != 2 {
		t.Fatalf("T-001 evidence count=%d, want 2", got)
	}
	if evidence["T-001"][0].CommandText != "first" || evidence["T-001"][1].CommandText != "second" {
		t.Fatalf("T-001 evidence order=%+v, want created order", evidence["T-001"])
	}
	if got := len(evidence["T-002"]); got != 0 {
		t.Fatalf("T-002 evidence count=%d, want 0", got)
	}
	if _, ok := evidence["T-003"]; ok {
		t.Fatalf("unexpected evidence for unrequested T-003: %+v", evidence["T-003"])
	}

	reviews, err := s.ReviewsByTaskIDs(ctx, []string{"T-002", "T-001", "T-002"})
	if err != nil {
		t.Fatal(err)
	}
	if got := reviews["T-001"]; len(got) != 1 || got[0].Domain != "backend" || got[0].Verdict != "approve" {
		t.Fatalf("T-001 reviews=%+v, want backend approve", got)
	}
	if got := reviews["T-002"]; len(got) != 1 || got[0].Domain != "ops" || got[0].Verdict != "changes" {
		t.Fatalf("T-002 reviews=%+v, want ops changes", got)
	}
	if _, ok := reviews["T-003"]; ok {
		t.Fatalf("unexpected reviews for unrequested T-003: %+v", reviews["T-003"])
	}

	transitions, err := s.TransitionsByTaskIDs(ctx, []string{"T-001", "T-001", "T-002"})
	if err != nil {
		t.Fatal(err)
	}
	if got := transitions["T-001"]; len(got) != 2 || got[0].ToStatus != "todo" || got[1].ToStatus != "in_progress" || got[1].Reason != "started" {
		t.Fatalf("T-001 transitions=%+v, want ordered import and in_progress transitions", got)
	}
	if _, ok := transitions["T-003"]; ok {
		t.Fatalf("unexpected transitions for unrequested T-003: %+v", transitions["T-003"])
	}

	handoffs, err := s.HandoffsByTaskIDs(ctx, []string{"T-002", "", "T-001"})
	if err != nil {
		t.Fatal(err)
	}
	if got := handoffs["T-002"]; len(got) != 1 || got[0].FromRole != "backend" || got[0].ToRole != "ops" {
		t.Fatalf("T-002 handoffs=%+v, want backend to ops", got)
	}
	if got := len(handoffs["T-001"]); got != 0 {
		t.Fatalf("T-001 handoff count=%d, want 0", got)
	}
}

func TestSQLiteBusyTimeoutAllowsBurstEvidenceWrite(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "state.db")
	first, err := Open(ctx, dbPath, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err := first.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Evidence", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	second, err := Open(ctx, dbPath, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })

	conn, err := first.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	releaseDone := make(chan error, 1)
	time.AfterFunc(100*time.Millisecond, func() {
		_, err := conn.ExecContext(ctx, "COMMIT")
		releaseDone <- err
	})
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- second.RecordEvidence(ctx, "T-001", Evidence{CommandText: "go test ./...", Result: "pass"})
	}()

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("write after busy wait: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("write did not complete after lock release")
	}
	if err := <-releaseDone; err != nil {
		t.Fatalf("release lock: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, evidence, _, _, err := first.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 {
		t.Fatalf("evidence rows=%d, want 1", len(evidence))
	}
}

func TestActivityFilteredSupportsStoreLevelPredicates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{
		{ID: "T-001", Title: "Platform evidence", Role: "backend", Profile: "platform"},
		{ID: "T-002", Title: "Docs evidence", Role: "backend", Profile: "docs"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", Evidence{CommandText: "go test ./...", Result: "pass"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", Review{Reviewer: "arch", Verdict: "approve", Reason: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-002", Evidence{CommandText: "npm test", Result: "pass"}); err != nil {
		t.Fatal(err)
	}

	evidence, err := s.ActivityFiltered(ctx, ActivityOptions{Limit: 10, Kind: "evidence"})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 2 {
		t.Fatalf("evidence activity=%+v, want 2 rows", evidence)
	}
	for _, item := range evidence {
		if item.Kind != "evidence" {
			t.Fatalf("kind filter returned %+v", item)
		}
	}

	taskActivity, err := s.ActivityFiltered(ctx, ActivityOptions{Limit: 10, TaskID: "T-002"})
	if err != nil {
		t.Fatal(err)
	}
	if len(taskActivity) != 2 {
		t.Fatalf("task activity=%+v, want import state plus evidence rows for T-002", taskActivity)
	}
	for _, item := range taskActivity {
		if item.TaskID != "T-002" {
			t.Fatalf("task filter returned %+v", item)
		}
	}

	profileActivity, err := s.ActivityFiltered(ctx, ActivityOptions{Limit: 10, Profile: "platform"})
	if err != nil {
		t.Fatal(err)
	}
	if len(profileActivity) != 3 {
		t.Fatalf("profile activity=%+v, want state/evidence/review rows for T-001", profileActivity)
	}
	for _, item := range profileActivity {
		if item.TaskID != "T-001" {
			t.Fatalf("profile filter returned %+v", item)
		}
	}

	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	windowed, err := s.ActivityFiltered(ctx, ActivityOptions{Limit: 10, CreatedFrom: future})
	if err != nil {
		t.Fatal(err)
	}
	if len(windowed) != 0 {
		t.Fatalf("future activity=%+v, want no rows", windowed)
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

func TestNotificationTracksHandoffDeliveryState(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Review", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordHandoff(ctx, "T-001", Handoff{ToRole: "security", Payload: "please review"}); err != nil {
		t.Fatal(err)
	}
	gaps, err := s.HandoffNotificationGaps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 || gaps[0].TaskID != "T-001" || gaps[0].Domain != "security" {
		t.Fatalf("gaps=%+v, want security gap for T-001", gaps)
	}
	recorded, err := s.RecordNotification(ctx, Notification{TaskID: "T-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "sent"})
	if err != nil {
		t.Fatal(err)
	}
	if recorded.ID == 0 || recorded.CreatedAt == "" {
		t.Fatalf("recorded notification missing id/time: %+v", recorded)
	}
	notifications, err := s.Notifications(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(notifications) != 1 || notifications[0].State != "sent" || notifications[0].Provider != "codex" {
		t.Fatalf("notifications=%+v, want sent codex notification", notifications)
	}
	gaps, err = s.HandoffNotificationGaps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Fatalf("gaps after sent notification=%+v, want none", gaps)
	}

	if _, err := s.RecordNotification(ctx, Notification{TaskID: "T-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "handoff_recorded"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, Notification{TaskID: "T-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "notification_delivered"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, Notification{TaskID: "T-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "thread_steered"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, Notification{TaskID: "T-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "review_acknowledged"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, Notification{TaskID: "T-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "notification_failed", Reason: "thread tool unavailable"}); err != nil {
		t.Fatal(err)
	}
}

func TestHandoffNotificationGapsEscalateStaleSentNotifications(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	tasks := []TaskDefinition{
		{ID: "STALE-001", Title: "Stale notification", Role: "backend"},
		{ID: "ACK-001", Title: "Acknowledged notification", Role: "backend"},
		{ID: "REVIEW-001", Title: "Reviewed notification", Role: "backend"},
	}
	if err := s.ImportTasks(ctx, tasks); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"STALE-001", "ACK-001", "REVIEW-001"} {
		if err := s.RecordHandoff(ctx, id, Handoff{ToRole: "security", Payload: "please review"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.RecordNotification(ctx, Notification{TaskID: id, Domain: "security", Provider: "codex", Target: "thread-1", State: "sent"}); err != nil {
			t.Fatal(err)
		}
	}
	oldHandoff := time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339Nano)
	old := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `UPDATE task_handoffs SET created_at=? WHERE project_id=?`, oldHandoff, s.projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE task_notifications SET created_at=? WHERE project_id=?`, old, s.projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, Notification{TaskID: "ACK-001", Domain: "security", Provider: "codex", Target: "thread-1", State: "acknowledged"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "REVIEW-001", Review{Reviewer: "fairway-reviewer", Domain: "security", Verdict: "approve", Reason: "reviewed"}); err != nil {
		t.Fatal(err)
	}

	gaps, err := s.HandoffNotificationGapsFiltered(ctx, HandoffNotificationGapOptions{
		SentStaleBefore: time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 || gaps[0].TaskID != "STALE-001" || gaps[0].NotificationStatus != "stale-sent" {
		t.Fatalf("gaps=%+v, want one stale-sent gap", gaps)
	}
}

func TestHandoffNotificationGapsFilterAcknowledgedAndTerminalTasks(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	tasks := []TaskDefinition{
		{ID: "ACTIVE-001", Title: "Active handoff", Role: "backend"},
		{ID: "ACK-001", Title: "Acknowledged handoff", Role: "backend"},
		{ID: "DONE-001", Title: "Historical done handoff", Role: "backend"},
		{ID: "DONEREVIEW-001", Title: "Terminal pending review", Role: "backend"},
	}
	if err := s.ImportTasks(ctx, tasks); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ACTIVE-001", "ACK-001", "DONE-001", "DONEREVIEW-001"} {
		if err := s.RecordHandoff(ctx, id, Handoff{ToRole: "security", Payload: "please review"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE task_handoffs SET acknowledged_at=? WHERE project_id=? AND task_id=?`, time.Now().UTC().Format(time.RFC3339Nano), s.projectID, "ACK-001"); err != nil {
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

	gaps, err := s.HandoffNotificationGapsFiltered(ctx, HandoffNotificationGapOptions{TerminalStatuses: []string{"done"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 2 {
		t.Fatalf("gaps=%+v, want active and terminal pending-review gaps", gaps)
	}
	if !hasHandoffGap(gaps, "ACTIVE-001") {
		t.Fatalf("active handoff gap missing: %+v", gaps)
	}
	if hasHandoffGap(gaps, "ACK-001") {
		t.Fatalf("acknowledged handoff surfaced as gap: %+v", gaps)
	}
	if hasHandoffGap(gaps, "DONE-001") {
		t.Fatalf("terminal done handoff surfaced as gap: %+v", gaps)
	}
	if !hasHandoffGap(gaps, "DONEREVIEW-001") {
		t.Fatalf("terminal pending-review handoff gap missing: %+v", gaps)
	}
}

func hasHandoffGap(gaps []HandoffNotificationGap, taskID string) bool {
	for _, gap := range gaps {
		if gap.TaskID == taskID {
			return true
		}
	}
	return false
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	pid := 123
	if err := s.UpsertSession(ctx, Session{ID: "backend-123", Role: "backend", Branch: "agent/backend", Status: "running", PID: &pid, MonitorKind: "ci", AutomationID: "auto-123", ExternalRunID: "run-123", PollCommand: "gh run view", ManualUntil: "2099-01-01"}); err != nil {
		t.Fatal(err)
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != "backend-123" || sessions[0].PID == nil || *sessions[0].PID != pid {
		t.Fatalf("sessions=%+v, want live backend session", sessions)
	}
	if sessions[0].MonitorKind != "ci" || sessions[0].AutomationID != "auto-123" || sessions[0].ExternalRunID != "run-123" || sessions[0].PollCommand != "gh run view" || sessions[0].ManualUntil != "2099-01-01" {
		t.Fatalf("session monitor metadata=%+v, want round trip", sessions[0])
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

func TestProviderUsage_NullCountsAndDerivedSnapshots(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "Usage", Role: "backend", Kind: "feature"}}); err != nil {
		t.Fatal(err)
	}
	started := 100
	completed := 175
	input := 120
	cached := 40
	recorded, err := s.RecordProviderUsage(ctx, ProviderUsage{
		Provider:               "codex",
		TaskID:                 "T-001",
		Role:                   "backend",
		Phase:                  "implementation",
		Source:                 "provider_reported",
		Confidence:             "exact",
		StartedTokenSnapshot:   &started,
		CompletedTokenSnapshot: &completed,
		InputTokens:            &input,
		CachedInputTokens:      &cached,
	})
	if err != nil {
		t.Fatal(err)
	}
	if recorded.TotalTokens == nil || *recorded.TotalTokens != 75 {
		t.Fatalf("expected derived total 75, got %#v", recorded.TotalTokens)
	}
	if recorded.UncachedInputTokens == nil || *recorded.UncachedInputTokens != 80 {
		t.Fatalf("expected derived uncached input 80, got %#v", recorded.UncachedInputTokens)
	}
	if _, err := s.RecordProviderUsage(ctx, ProviderUsage{Provider: "unknown-provider", TaskID: "T-001"}); err != nil {
		t.Fatal(err)
	}
	events, err := s.ProviderUsageForTask(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 usage events, got %d", len(events))
	}
	if events[1].TotalTokens != nil || events[1].InputTokens != nil || events[1].CachedInputTokens != nil {
		t.Fatalf("unknown usage should remain nil, got %#v", events[1])
	}
	rollups, err := s.UsageRollups(ctx, UsageRollupOptions{GroupBy: "provider"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rollups) != 2 {
		t.Fatalf("expected 2 provider rollups, got %d", len(rollups))
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
