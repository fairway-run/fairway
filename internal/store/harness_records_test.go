package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/harnessrecord"
)

func TestHarnessRecordsIngestReplayConflictAndReadback(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "one", Role: "backend"}, {ID: "T-002", Title: "two", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	batch := harnessrecord.Batch{Schema: harnessrecord.BatchSchema,
		ExternalRuns: []harnessrecord.ExternalRun{{Schema: harnessrecord.ExternalRunSchema, SourceID: "codex", SourceVersion: "1", ExternalRunID: "run-1", TaskID: "T-001", SubmissionID: "submit-1", ObservedAt: "2026-08-21T00:00:00Z"}},
		Observations: []harnessrecord.Observation{
			{Schema: harnessrecord.ObservationSchema, ObservationID: "obs-1", SourceID: "codex", SourceVersion: "1", TaskID: "T-001", ExternalRunRef: &harnessrecord.RecordRef{SourceID: "codex", ID: "run-1"}, Kind: "experiment", SubjectType: "task", SubjectRef: "T-001", Summary: "hypothesis rejected", ObservedAt: "2026-08-21T00:01:00Z", Outcome: "rejected", SourceMode: "measured", Hypothesis: "fix works", ExpectedObservation: "test passes"},
			{Schema: harnessrecord.ObservationSchema, ObservationID: "obs-ci", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", Kind: "execution", SubjectType: "commit", SubjectRef: "abc", Summary: "lint passed", ObservedAt: "2026-08-21T00:02:00Z", Outcome: "confirmed", SourceMode: "measured"},
		},
		EvaluatorResults: []harnessrecord.EvaluatorResult{{Schema: harnessrecord.EvaluationSchema, EvaluationID: "eval-1", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", ExternalRunRef: &harnessrecord.RecordRef{SourceID: "codex", ID: "run-1"}, ObservationRef: &harnessrecord.RecordRef{SourceID: "codex", ID: "obs-1"}, EvaluatorID: "go-test", EvaluatorVersion: "1", SubjectType: "commit", SubjectRef: "abc", Result: "fail", Mode: "deterministic", EvaluatedAt: "2026-08-21T00:03:00Z"}},
	}
	first, err := s.IngestHarnessBatch(ctx, batch)
	if err != nil || first.ExternalRunsInserted != 1 || first.ObservationsInserted != 2 || first.EvaluatorResultsInserted != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := s.IngestHarnessBatch(ctx, batch)
	if err != nil || second.ExternalRunsExisting != 1 || second.ObservationsExisting != 2 || second.EvaluatorResultsExisting != 1 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	record, err := s.HarnessRecord(ctx, "codex", "run-1")
	if err != nil || len(record.Observations) != 1 || len(record.EvaluatorResults) != 1 {
		t.Fatalf("record=%+v err=%v", record, err)
	}
	taskRecords, err := s.HarnessRecordsForTask(ctx, "T-001")
	if err != nil || len(taskRecords.Runs) != 1 || len(taskRecords.RunIndependentObservations) != 1 {
		t.Fatalf("task=%+v err=%v", taskRecords, err)
	}

	conflict := batch
	conflict.ExternalRuns = append([]harnessrecord.ExternalRun(nil), batch.ExternalRuns...)
	conflict.ExternalRuns[0].TerminalStatus = "success"
	if _, err := s.IngestHarnessBatch(ctx, conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestHarnessRecordsAuditCountsAreTaskSpecific(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "one", Role: "backend"}, {ID: "T-002", Title: "two", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	batch := harnessrecord.Batch{Schema: harnessrecord.BatchSchema, Observations: []harnessrecord.Observation{
		{Schema: harnessrecord.ObservationSchema, ObservationID: "o1", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", Kind: "execution", SubjectType: "task", SubjectRef: "T-001", Summary: "one", ObservedAt: "2026-08-21T00:00:00Z", Outcome: "confirmed", SourceMode: "measured"},
		{Schema: harnessrecord.ObservationSchema, ObservationID: "o2", SourceID: "ci", SourceVersion: "1", TaskID: "T-002", Kind: "execution", SubjectType: "task", SubjectRef: "T-002", Summary: "two", ObservedAt: "2026-08-21T00:00:00Z", Outcome: "confirmed", SourceMode: "measured"},
	}}
	if _, err := s.IngestHarnessBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, detail FROM audit_events WHERE action='harness_records_ingested' ORDER BY task_id`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close audit rows: %v", err)
		}
	})
	count := 0
	for rows.Next() {
		var taskID, detail string
		if err := rows.Scan(&taskID, &detail); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(detail, "observations_inserted=1") || strings.Contains(detail, "observations_inserted=2") {
			t.Fatalf("task=%s detail=%s", taskID, detail)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("audit rows=%d", count)
	}
}

func TestHarnessRecordsRejectCrossTaskReferenceAtomically(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.ImportTasks(ctx, []TaskDefinition{{ID: "T-001", Title: "one", Role: "backend"}, {ID: "T-002", Title: "two", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	batch := harnessrecord.Batch{Schema: harnessrecord.BatchSchema,
		ExternalRuns: []harnessrecord.ExternalRun{{Schema: harnessrecord.ExternalRunSchema, SourceID: "codex", SourceVersion: "1", ExternalRunID: "run-1", TaskID: "T-001", SubmissionID: "submit-1", ObservedAt: "2026-08-21T00:00:00Z"}},
		Observations: []harnessrecord.Observation{{Schema: harnessrecord.ObservationSchema, ObservationID: "obs-1", SourceID: "ci", SourceVersion: "1", TaskID: "T-002", ExternalRunRef: &harnessrecord.RecordRef{SourceID: "codex", ID: "run-1"}, Kind: "execution", SubjectType: "task", SubjectRef: "T-002", Summary: "bad link", ObservedAt: "2026-08-21T00:01:00Z", Outcome: "blocked", SourceMode: "reported"}},
	}
	if _, err := s.IngestHarnessBatch(ctx, batch); err == nil {
		t.Fatal("expected cross-task rejection")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM harness_external_runs`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("atomic count=%d err=%v", count, err)
	}
}
