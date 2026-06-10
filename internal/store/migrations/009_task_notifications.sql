CREATE TABLE IF NOT EXISTS task_notifications (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  handoff_id INTEGER,
  domain TEXT NOT NULL,
  provider TEXT,
  target TEXT,
  state TEXT NOT NULL,
  reason TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_state(project_id, task_id),
  FOREIGN KEY(handoff_id) REFERENCES task_handoffs(id)
);
CREATE INDEX IF NOT EXISTS idx_task_notifications_task_domain ON task_notifications(project_id, task_id, domain, created_at);
CREATE INDEX IF NOT EXISTS idx_task_notifications_state ON task_notifications(project_id, state, created_at);
