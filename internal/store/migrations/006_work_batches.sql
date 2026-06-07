CREATE TABLE IF NOT EXISTS work_batches (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  title TEXT NOT NULL,
  branch TEXT,
  worktree_path TEXT,
  validation_commands TEXT,
  review_domains TEXT,
  rollback_criteria TEXT,
  split_criteria TEXT,
  expected_ci TEXT,
  deploy_run_id TEXT,
  pipeline_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(project_id, id)
);

CREATE TABLE IF NOT EXISTS work_batch_tasks (
  project_id TEXT NOT NULL,
  batch_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(project_id, batch_id, task_id),
  FOREIGN KEY(project_id, batch_id) REFERENCES work_batches(project_id, id),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_work_batch_tasks_task ON work_batch_tasks(project_id, task_id);

CREATE TABLE IF NOT EXISTS work_batch_evidence (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  batch_id TEXT NOT NULL,
  command_text TEXT,
  result TEXT,
  artifact_path TEXT,
  artifact_type TEXT,
  notes TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, batch_id) REFERENCES work_batches(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_work_batch_evidence_batch ON work_batch_evidence(project_id, batch_id, created_at);
