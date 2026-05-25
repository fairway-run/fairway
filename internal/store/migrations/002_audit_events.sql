CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  task_id TEXT,
  detail TEXT,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_events_task ON audit_events(project_id, task_id, created_at);
