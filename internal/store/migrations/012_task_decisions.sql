CREATE TABLE IF NOT EXISTS task_decisions (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  decision TEXT NOT NULL,
  trigger_text TEXT NOT NULL,
  alternatives TEXT NOT NULL DEFAULT '[]',
  chosen TEXT NOT NULL,
  reason TEXT NOT NULL,
  scope_added TEXT NOT NULL DEFAULT '[]',
  risk TEXT NOT NULL,
  validation_refs TEXT NOT NULL DEFAULT '[]',
  fact_refs TEXT NOT NULL DEFAULT '[]',
  supersedes_id INTEGER,
  created_by TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id),
  FOREIGN KEY(supersedes_id) REFERENCES task_decisions(id)
);

CREATE INDEX IF NOT EXISTS idx_task_decisions_task
  ON task_decisions(project_id, task_id, created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_task_decisions_supersedes
  ON task_decisions(project_id, supersedes_id)
  WHERE supersedes_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS task_decision_assessments (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  decision_id INTEGER NOT NULL,
  quality_state TEXT NOT NULL CHECK(quality_state IN ('accepted', 'insufficient')),
  reviewer TEXT NOT NULL,
  reason TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id),
  FOREIGN KEY(decision_id) REFERENCES task_decisions(id)
);

CREATE INDEX IF NOT EXISTS idx_task_decision_assessments_decision
  ON task_decision_assessments(project_id, task_id, decision_id, created_at, id);
