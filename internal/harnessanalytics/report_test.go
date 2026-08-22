package harnessanalytics

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/harnessrecord"
	"github.com/subashram/fairway/internal/store"
)

func TestBuildReportsEfficiencyAndRepeatedTrajectory(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "test", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "session-1", Role: "backend", TaskID: "T-001", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	batch := harnessrecord.Batch{Schema: harnessrecord.BatchSchema,
		ExternalRuns: []harnessrecord.ExternalRun{{Schema: harnessrecord.ExternalRunSchema, SourceID: "agent", SourceVersion: "1", ExternalRunID: "r1", TaskID: "T-001", SubmissionID: "s1", SessionID: "session-1", Revision: "abc", ObservedAt: "2026-08-21T00:00:00Z"}},
		Observations: []harnessrecord.Observation{
			{Schema: harnessrecord.ObservationSchema, ObservationID: "o1", SourceID: "agent", SourceVersion: "1", TaskID: "T-001", ExternalRunRef: &harnessrecord.RecordRef{SourceID: "agent", ID: "r1"}, Kind: "experiment", SubjectType: "task", SubjectRef: "T-001", Summary: "first", ObservedAt: "2026-08-21T00:01:00Z", Outcome: "rejected", SourceMode: "measured", Hypothesis: "same hypothesis", ExpectedObservation: "pass", ActionFingerprint: "same-action"},
			{Schema: harnessrecord.ObservationSchema, ObservationID: "o2", SourceID: "agent", SourceVersion: "1", TaskID: "T-001", ExternalRunRef: &harnessrecord.RecordRef{SourceID: "agent", ID: "r1"}, Kind: "experiment", SubjectType: "task", SubjectRef: "T-001", Summary: "second", ObservedAt: "2026-08-21T00:02:00Z", Outcome: "inconclusive", SourceMode: "measured", Hypothesis: "same hypothesis", ExpectedObservation: "pass", ActionFingerprint: "same-action"},
		},
		EvaluatorResults: []harnessrecord.EvaluatorResult{
			{Schema: harnessrecord.EvaluationSchema, EvaluationID: "e1", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", ExternalRunRef: &harnessrecord.RecordRef{SourceID: "agent", ID: "r1"}, ObservationRef: &harnessrecord.RecordRef{SourceID: "agent", ID: "o1"}, EvaluatorID: "test", EvaluatorVersion: "1", SubjectType: "commit", SubjectRef: "abc", Result: "fail", Mode: "deterministic", EvaluatedAt: "2026-08-21T00:03:00Z"},
			{Schema: harnessrecord.EvaluationSchema, EvaluationID: "e2", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", ExternalRunRef: &harnessrecord.RecordRef{SourceID: "agent", ID: "r1"}, ObservationRef: &harnessrecord.RecordRef{SourceID: "agent", ID: "o2"}, EvaluatorID: "test", EvaluatorVersion: "1", SubjectType: "commit", SubjectRef: "abc", Result: "inconclusive", Mode: "deterministic", EvaluatedAt: "2026-08-21T00:04:00Z"},
		},
	}
	if _, err := s.IngestHarnessBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}
	tokens, elapsed := 100, 20
	if _, err := s.RecordProviderUsage(ctx, store.ProviderUsage{Provider: "codex", TaskID: "T-001", SessionID: "session-1", Source: "provider_reported", Confidence: "exact", TotalTokens: &tokens, ElapsedSeconds: &elapsed}); err != nil {
		t.Fatal(err)
	}
	report, err := Build(ctx, s, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if report.Attempts != 1 || report.Actions != 2 || report.VerifiedOutcomes != 2 || report.Efficiency.Status != "available" {
		t.Fatalf("report=%+v", report)
	}
	if report.Efficiency.TokensPerVerifiedOutcome == nil || *report.Efficiency.TokensPerVerifiedOutcome != 50 {
		t.Fatalf("efficiency=%+v", report.Efficiency)
	}
	if len(report.Trajectory) != 2 {
		t.Fatalf("trajectory=%+v", report.Trajectory)
	}
}

func TestBuildExposesMissingDenominators(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "test", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	report, err := Build(ctx, s, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if report.Efficiency.Status != "insufficient_outcomes" || report.Usage.CostStatus != "unavailable" || len(report.Limitations) == 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestBuildAtReportsNoNewEvidenceWithoutMutatingTask(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "test", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "T-001", State: "active", Owner: "worker", Summary: "started"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "session-1", Role: "backend", TaskID: "T-001", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	started, err := time.Parse(time.RFC3339Nano, sessions[0].StartedAt)
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildAt(ctx, s, "T-001", started.Add(3*time.Hour), 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Trajectory) != 1 || report.Trajectory[0].Kind != "no_new_evidence" {
		t.Fatalf("trajectory=%+v", report.Trajectory)
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "todo" {
		t.Fatalf("status=%q, report must not mutate task", task.Status)
	}
}

func TestBuildWithholdsEfficiencyForIncompatibleEvaluatorCohorts(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "test", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	batch := harnessrecord.Batch{Schema: harnessrecord.BatchSchema, EvaluatorResults: []harnessrecord.EvaluatorResult{
		{Schema: harnessrecord.EvaluationSchema, EvaluationID: "e1", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", EvaluatorID: "test", EvaluatorVersion: "1", SubjectType: "commit", SubjectRef: "abc", Result: "pass", Mode: "deterministic", EvaluatedAt: "2026-08-21T00:00:00Z", Environment: "linux"},
		{Schema: harnessrecord.EvaluationSchema, EvaluationID: "e2", SourceID: "ci", SourceVersion: "2", TaskID: "T-001", EvaluatorID: "test", EvaluatorVersion: "2", SubjectType: "artifact", SubjectRef: "bundle", Result: "pass", Mode: "deterministic", EvaluatedAt: "2026-08-21T00:01:00Z", Environment: "darwin"},
	}}
	if _, err := s.IngestHarnessBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}
	report, err := Build(ctx, s, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if report.Cohort.Status != "incompatible" || report.Efficiency.Status != "unavailable" || report.Efficiency.AttemptsPerVerifiedOutcome != nil {
		t.Fatalf("report=%+v", report)
	}
}

func TestNoNewEvidenceRequiresActiveSession(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "test", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "T-001", State: "active", Owner: "worker", Summary: "stale checkpoint"}); err != nil {
		t.Fatal(err)
	}
	report, err := BuildAt(ctx, s, "T-001", time.Now().UTC().Add(24*time.Hour), 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Trajectory) != 0 {
		t.Fatalf("stale checkpoint without active session produced finding: %+v", report.Trajectory)
	}
}

func TestNoNewEvidenceDoesNotTreatSameRevisionRetryAsProgress(t *testing.T) {
	started := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	finding := detectNoNewEvidence(
		"T-001",
		started.Add(4*time.Hour),
		2*time.Hour,
		[]store.Session{{ID: "session-1", TaskID: "T-001", Status: "running", StartedAt: started.Format(time.RFC3339Nano)}},
		nil,
		nil,
		nil,
		[]store.HarnessRunRecord{
			{Run: harnessrecord.ExternalRun{SourceID: "agent", ExternalRunID: "r1", Revision: "abc", ObservedAt: started.Add(time.Hour).Format(time.RFC3339Nano)}},
			{Run: harnessrecord.ExternalRun{SourceID: "agent", ExternalRunID: "r2", Revision: "abc", ObservedAt: started.Add(150 * time.Minute).Format(time.RFC3339Nano)}},
		},
		nil,
		nil,
	)
	if finding == nil || finding.Kind != "no_new_evidence" {
		t.Fatalf("same-revision retry suppressed finding: %+v", finding)
	}
}

func TestBuildWithholdsRunIndependentOutcomeEfficiency(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "test", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	batch := harnessrecord.Batch{Schema: harnessrecord.BatchSchema, EvaluatorResults: []harnessrecord.EvaluatorResult{{Schema: harnessrecord.EvaluationSchema, EvaluationID: "e1", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", EvaluatorID: "test", EvaluatorVersion: "1", SubjectType: "commit", SubjectRef: "abc", Result: "pass", Mode: "deterministic", EvaluatedAt: "2026-08-21T00:00:00Z"}}}
	if _, err := s.IngestHarnessBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}
	report, err := Build(ctx, s, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if report.Cohort.Status != "insufficient_attribution" || report.Efficiency.Status != "unavailable" {
		t.Fatalf("report=%+v", report)
	}
}
