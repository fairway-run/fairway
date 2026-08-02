CREATE TABLE IF NOT EXISTS task_outcomes (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  source_ref TEXT,
  related_task_id TEXT,
  notes TEXT,
  actor TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id),
  FOREIGN KEY(project_id, related_task_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_task_outcomes_task
  ON task_outcomes(project_id, task_id, occurred_at, id);
CREATE INDEX IF NOT EXISTS idx_task_outcomes_related
  ON task_outcomes(project_id, related_task_id, occurred_at, id);
