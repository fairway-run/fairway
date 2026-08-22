CREATE TABLE harness_external_runs (
  project_id TEXT NOT NULL,
  source_id TEXT NOT NULL,
  external_run_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  submission_id TEXT NOT NULL,
  session_id TEXT,
  payload_digest TEXT NOT NULL,
  record_json TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(project_id, source_id, external_run_id),
  UNIQUE(project_id, source_id, submission_id),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);

CREATE INDEX idx_harness_external_runs_task
  ON harness_external_runs(project_id, task_id, observed_at, source_id, external_run_id);

CREATE TABLE harness_observations (
  project_id TEXT NOT NULL,
  source_id TEXT NOT NULL,
  observation_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  external_run_source_id TEXT,
  external_run_id TEXT,
  payload_digest TEXT NOT NULL,
  record_json TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(project_id, source_id, observation_id),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id),
  CHECK((external_run_source_id IS NULL AND external_run_id IS NULL) OR
        (external_run_source_id IS NOT NULL AND external_run_id IS NOT NULL))
);

CREATE INDEX idx_harness_observations_task
  ON harness_observations(project_id, task_id, observed_at, source_id, observation_id);

CREATE INDEX idx_harness_observations_run
  ON harness_observations(project_id, external_run_source_id, external_run_id);

CREATE TABLE harness_evaluator_results (
  project_id TEXT NOT NULL,
  source_id TEXT NOT NULL,
  evaluation_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  external_run_source_id TEXT,
  external_run_id TEXT,
  observation_source_id TEXT,
  observation_id TEXT,
  payload_digest TEXT NOT NULL,
  record_json TEXT NOT NULL,
  evaluated_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(project_id, source_id, evaluation_id),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id),
  CHECK((external_run_source_id IS NULL AND external_run_id IS NULL) OR
        (external_run_source_id IS NOT NULL AND external_run_id IS NOT NULL)),
  CHECK((observation_source_id IS NULL AND observation_id IS NULL) OR
        (observation_source_id IS NOT NULL AND observation_id IS NOT NULL))
);

CREATE INDEX idx_harness_evaluator_results_task
  ON harness_evaluator_results(project_id, task_id, evaluated_at, source_id, evaluation_id);

CREATE INDEX idx_harness_evaluator_results_run
  ON harness_evaluator_results(project_id, external_run_source_id, external_run_id);

CREATE INDEX idx_harness_evaluator_results_observation
  ON harness_evaluator_results(project_id, observation_source_id, observation_id);
