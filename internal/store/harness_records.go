package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/subashram/fairway/internal/harnessrecord"
)

// HarnessIngestResult summarizes one atomic harness-record ingestion.
type HarnessIngestResult struct {
	ExternalRunsInserted     int `json:"external_runs_inserted"`
	ExternalRunsExisting     int `json:"external_runs_existing"`
	ObservationsInserted     int `json:"observations_inserted"`
	ObservationsExisting     int `json:"observations_existing"`
	EvaluatorResultsInserted int `json:"evaluator_results_inserted"`
	EvaluatorResultsExisting int `json:"evaluator_results_existing"`
}

// HarnessRunRecord is one external run with its directly correlated facts.
type HarnessRunRecord struct {
	Run              harnessrecord.ExternalRun       `json:"run"`
	Observations     []harnessrecord.Observation     `json:"observations"`
	EvaluatorResults []harnessrecord.EvaluatorResult `json:"evaluator_results"`
}

// HarnessTaskRecords projects run-bound and run-independent facts for a task.
type HarnessTaskRecords struct {
	TaskID                     string                          `json:"task_id"`
	Runs                       []HarnessRunRecord              `json:"runs"`
	RunIndependentObservations []harnessrecord.Observation     `json:"run_independent_observations"`
	RunIndependentEvaluations  []harnessrecord.EvaluatorResult `json:"run_independent_evaluator_results"`
}

// IngestHarnessBatch validates and atomically appends harness records.
func (s *Store) IngestHarnessBatch(ctx context.Context, batch harnessrecord.Batch) (HarnessIngestResult, error) {
	if err := harnessrecord.ValidateBatch(batch, time.Now().UTC()); err != nil {
		return HarnessIngestResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return HarnessIngestResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	tasks := map[string]bool{}
	for _, run := range batch.ExternalRuns {
		tasks[run.TaskID] = true
	}
	for _, observation := range batch.Observations {
		tasks[observation.TaskID] = true
	}
	for _, evaluation := range batch.EvaluatorResults {
		tasks[evaluation.TaskID] = true
	}
	for taskID := range tasks {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_definitions WHERE project_id=? AND id=?`, s.projectID, taskID).Scan(&count); err != nil {
			return HarnessIngestResult{}, err
		}
		if count == 0 {
			return HarnessIngestResult{}, fmt.Errorf("harness record task %s: %w", taskID, ErrNotFound)
		}
	}
	for _, run := range batch.ExternalRuns {
		if run.SessionID == "" {
			continue
		}
		var taskID string
		if err := tx.QueryRowContext(ctx, `SELECT task_id FROM agent_sessions WHERE project_id=? AND id=?`, s.projectID, run.SessionID).Scan(&taskID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return HarnessIngestResult{}, fmt.Errorf("external run %s session not found", run.ExternalRunID)
			}
			return HarnessIngestResult{}, err
		}
		if taskID != run.TaskID {
			return HarnessIngestResult{}, fmt.Errorf("external run %s session belongs to another task", run.ExternalRunID)
		}
	}
	batchRuns := map[string]string{}
	for _, run := range batch.ExternalRuns {
		batchRuns[run.SourceID+"\x00"+run.ExternalRunID] = run.TaskID
	}
	batchObservations := map[string]string{}
	for _, observation := range batch.Observations {
		batchObservations[observation.SourceID+"\x00"+observation.ObservationID] = observation.TaskID
	}
	for _, run := range batch.ExternalRuns {
		if run.PriorRunRef != nil {
			if err := s.requireHarnessRunTask(ctx, tx, batchRuns, run.PriorRunRef.SourceID, run.PriorRunRef.ID, run.TaskID); err != nil {
				return HarnessIngestResult{}, fmt.Errorf("external run %s prior reference: %w", run.ExternalRunID, err)
			}
		}
	}
	for _, observation := range batch.Observations {
		if observation.ExternalRunRef != nil {
			if err := s.requireHarnessRunTask(ctx, tx, batchRuns, observation.ExternalRunRef.SourceID, observation.ExternalRunRef.ID, observation.TaskID); err != nil {
				return HarnessIngestResult{}, fmt.Errorf("observation %s run reference: %w", observation.ObservationID, err)
			}
		}
	}
	for _, evaluation := range batch.EvaluatorResults {
		if evaluation.ExternalRunRef != nil {
			if err := s.requireHarnessRunTask(ctx, tx, batchRuns, evaluation.ExternalRunRef.SourceID, evaluation.ExternalRunRef.ID, evaluation.TaskID); err != nil {
				return HarnessIngestResult{}, fmt.Errorf("evaluation %s run reference: %w", evaluation.EvaluationID, err)
			}
		}
		if evaluation.ObservationRef != nil {
			if err := s.requireHarnessObservationTask(ctx, tx, batchObservations, evaluation.ObservationRef.SourceID, evaluation.ObservationRef.ID, evaluation.TaskID); err != nil {
				return HarnessIngestResult{}, fmt.Errorf("evaluation %s observation reference: %w", evaluation.EvaluationID, err)
			}
		}
	}

	result := HarnessIngestResult{}
	taskResults := map[string]*HarnessIngestResult{}
	for taskID := range tasks {
		taskResults[taskID] = &HarnessIngestResult{}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, run := range batch.ExternalRuns {
		data, digest, err := harnessrecord.Canonical(run)
		if err != nil {
			return HarnessIngestResult{}, err
		}
		var submittedRunID, submittedDigest string
		err = tx.QueryRowContext(ctx, `SELECT external_run_id, payload_digest FROM harness_external_runs WHERE project_id=? AND source_id=? AND submission_id=?`, s.projectID, run.SourceID, run.SubmissionID).Scan(&submittedRunID, &submittedDigest)
		if err == nil && (submittedRunID != run.ExternalRunID || submittedDigest != digest) {
			return HarnessIngestResult{}, fmt.Errorf("external run %s submission identity: %w", run.ExternalRunID, ErrIdempotencyConflict)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return HarnessIngestResult{}, err
		}
		inserted, err := ingestHarnessRow(ctx, tx,
			`SELECT payload_digest FROM harness_external_runs WHERE project_id=? AND source_id=? AND external_run_id=?`,
			`INSERT INTO harness_external_runs(project_id,source_id,external_run_id,task_id,submission_id,session_id,payload_digest,record_json,observed_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			[]any{s.projectID, run.SourceID, run.ExternalRunID}, []any{s.projectID, run.SourceID, run.ExternalRunID, run.TaskID, run.SubmissionID, nullString(run.SessionID), digest, string(data), run.ObservedAt, now}, digest)
		if err != nil {
			return HarnessIngestResult{}, fmt.Errorf("external run %s: %w", run.ExternalRunID, err)
		}
		if inserted {
			result.ExternalRunsInserted++
			taskResults[run.TaskID].ExternalRunsInserted++
		} else {
			result.ExternalRunsExisting++
			taskResults[run.TaskID].ExternalRunsExisting++
		}
	}
	for _, observation := range batch.Observations {
		data, digest, err := harnessrecord.Canonical(observation)
		if err != nil {
			return HarnessIngestResult{}, err
		}
		var runSource, runID any
		if observation.ExternalRunRef != nil {
			runSource, runID = observation.ExternalRunRef.SourceID, observation.ExternalRunRef.ID
		}
		inserted, err := ingestHarnessRow(ctx, tx,
			`SELECT payload_digest FROM harness_observations WHERE project_id=? AND source_id=? AND observation_id=?`,
			`INSERT INTO harness_observations(project_id,source_id,observation_id,task_id,external_run_source_id,external_run_id,payload_digest,record_json,observed_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			[]any{s.projectID, observation.SourceID, observation.ObservationID}, []any{s.projectID, observation.SourceID, observation.ObservationID, observation.TaskID, runSource, runID, digest, string(data), observation.ObservedAt, now}, digest)
		if err != nil {
			return HarnessIngestResult{}, fmt.Errorf("observation %s: %w", observation.ObservationID, err)
		}
		if inserted {
			result.ObservationsInserted++
			taskResults[observation.TaskID].ObservationsInserted++
		} else {
			result.ObservationsExisting++
			taskResults[observation.TaskID].ObservationsExisting++
		}
	}
	for _, evaluation := range batch.EvaluatorResults {
		data, digest, err := harnessrecord.Canonical(evaluation)
		if err != nil {
			return HarnessIngestResult{}, err
		}
		var runSource, runID, observationSource, observationID any
		if evaluation.ExternalRunRef != nil {
			runSource, runID = evaluation.ExternalRunRef.SourceID, evaluation.ExternalRunRef.ID
		}
		if evaluation.ObservationRef != nil {
			observationSource, observationID = evaluation.ObservationRef.SourceID, evaluation.ObservationRef.ID
		}
		inserted, err := ingestHarnessRow(ctx, tx,
			`SELECT payload_digest FROM harness_evaluator_results WHERE project_id=? AND source_id=? AND evaluation_id=?`,
			`INSERT INTO harness_evaluator_results(project_id,source_id,evaluation_id,task_id,external_run_source_id,external_run_id,observation_source_id,observation_id,payload_digest,record_json,evaluated_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			[]any{s.projectID, evaluation.SourceID, evaluation.EvaluationID}, []any{s.projectID, evaluation.SourceID, evaluation.EvaluationID, evaluation.TaskID, runSource, runID, observationSource, observationID, digest, string(data), evaluation.EvaluatedAt, now}, digest)
		if err != nil {
			return HarnessIngestResult{}, fmt.Errorf("evaluation %s: %w", evaluation.EvaluationID, err)
		}
		if inserted {
			result.EvaluatorResultsInserted++
			taskResults[evaluation.TaskID].EvaluatorResultsInserted++
		} else {
			result.EvaluatorResultsExisting++
			taskResults[evaluation.TaskID].EvaluatorResultsExisting++
		}
	}
	for taskID, taskResult := range taskResults {
		detail := fmt.Sprintf("runs_inserted=%d observations_inserted=%d evaluations_inserted=%d idempotent_existing=%d", taskResult.ExternalRunsInserted, taskResult.ObservationsInserted, taskResult.EvaluatorResultsInserted, taskResult.ExternalRunsExisting+taskResult.ObservationsExisting+taskResult.EvaluatorResultsExisting)
		if err := insertAudit(ctx, tx, s.projectID, AuditEvent{Action: "harness_records_ingested", TaskID: taskID, Detail: detail}); err != nil {
			return HarnessIngestResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return HarnessIngestResult{}, err
	}
	return result, nil
}

func ingestHarnessRow(ctx context.Context, tx *sql.Tx, selectSQL, insertSQL string, selectArgs, insertArgs []any, digest string) (bool, error) {
	var existing string
	err := tx.QueryRowContext(ctx, selectSQL, selectArgs...).Scan(&existing)
	if err == nil {
		if existing == digest {
			return false, nil
		}
		return false, ErrIdempotencyConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, insertSQL, insertArgs...); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) requireHarnessRunTask(ctx context.Context, tx *sql.Tx, batch map[string]string, sourceID, runID, taskID string) error {
	if batchTask, ok := batch[sourceID+"\x00"+runID]; ok {
		if batchTask != taskID {
			return errors.New("referenced run belongs to another task")
		}
		return nil
	}
	var actual string
	if err := tx.QueryRowContext(ctx, `SELECT task_id FROM harness_external_runs WHERE project_id=? AND source_id=? AND external_run_id=?`, s.projectID, sourceID, runID).Scan(&actual); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("referenced run not found")
		}
		return err
	}
	if actual != taskID {
		return errors.New("referenced run belongs to another task")
	}
	return nil
}

func (s *Store) requireHarnessObservationTask(ctx context.Context, tx *sql.Tx, batch map[string]string, sourceID, observationID, taskID string) error {
	if batchTask, ok := batch[sourceID+"\x00"+observationID]; ok {
		if batchTask != taskID {
			return errors.New("referenced observation belongs to another task")
		}
		return nil
	}
	var actual string
	if err := tx.QueryRowContext(ctx, `SELECT task_id FROM harness_observations WHERE project_id=? AND source_id=? AND observation_id=?`, s.projectID, sourceID, observationID).Scan(&actual); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("referenced observation not found")
		}
		return err
	}
	if actual != taskID {
		return errors.New("referenced observation belongs to another task")
	}
	return nil
}

// HarnessRecord returns one source-qualified run and its directly correlated facts.
func (s *Store) HarnessRecord(ctx context.Context, sourceID, runID string) (HarnessRunRecord, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT record_json FROM harness_external_runs WHERE project_id=? AND source_id=? AND external_run_id=?`, s.projectID, sourceID, runID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HarnessRunRecord{}, ErrNotFound
		}
		return HarnessRunRecord{}, err
	}
	var result HarnessRunRecord
	if err := json.Unmarshal([]byte(raw), &result.Run); err != nil {
		return HarnessRunRecord{}, err
	}
	observations, err := queryHarnessJSON[harnessrecord.Observation](ctx, s.db, `SELECT record_json FROM harness_observations WHERE project_id=? AND external_run_source_id=? AND external_run_id=? ORDER BY observed_at, source_id, observation_id`, s.projectID, sourceID, runID)
	if err != nil {
		return HarnessRunRecord{}, err
	}
	evaluations, err := queryHarnessJSON[harnessrecord.EvaluatorResult](ctx, s.db, `SELECT record_json FROM harness_evaluator_results WHERE project_id=? AND external_run_source_id=? AND external_run_id=? ORDER BY evaluated_at, source_id, evaluation_id`, s.projectID, sourceID, runID)
	if err != nil {
		return HarnessRunRecord{}, err
	}
	result.Observations, result.EvaluatorResults = observations, evaluations
	return result, nil
}

// HarnessRecordsForTask returns all run-bound and independent harness facts for a task.
func (s *Store) HarnessRecordsForTask(ctx context.Context, taskID string) (HarnessTaskRecords, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_definitions WHERE project_id=? AND id=?`, s.projectID, taskID).Scan(&count); err != nil {
		return HarnessTaskRecords{}, err
	}
	if count == 0 {
		return HarnessTaskRecords{}, ErrNotFound
	}
	runs, err := queryHarnessJSON[harnessrecord.ExternalRun](ctx, s.db, `SELECT record_json FROM harness_external_runs WHERE project_id=? AND task_id=? ORDER BY observed_at, source_id, external_run_id`, s.projectID, taskID)
	if err != nil {
		return HarnessTaskRecords{}, err
	}
	result := HarnessTaskRecords{TaskID: taskID, Runs: make([]HarnessRunRecord, 0, len(runs))}
	for _, run := range runs {
		record, err := s.HarnessRecord(ctx, run.SourceID, run.ExternalRunID)
		if err != nil {
			return HarnessTaskRecords{}, err
		}
		result.Runs = append(result.Runs, record)
	}
	result.RunIndependentObservations, err = queryHarnessJSON[harnessrecord.Observation](ctx, s.db, `SELECT record_json FROM harness_observations WHERE project_id=? AND task_id=? AND external_run_id IS NULL ORDER BY observed_at, source_id, observation_id`, s.projectID, taskID)
	if err != nil {
		return HarnessTaskRecords{}, err
	}
	result.RunIndependentEvaluations, err = queryHarnessJSON[harnessrecord.EvaluatorResult](ctx, s.db, `SELECT record_json FROM harness_evaluator_results WHERE project_id=? AND task_id=? AND external_run_id IS NULL ORDER BY evaluated_at, source_id, evaluation_id`, s.projectID, taskID)
	return result, err
}

func queryHarnessJSON[T any](ctx context.Context, db *sql.DB, query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []T{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value T
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
