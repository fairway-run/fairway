CREATE TABLE IF NOT EXISTS server_write_idempotency (
  project_id TEXT NOT NULL,
  command_family TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  actor TEXT NOT NULL,
  role TEXT NOT NULL,
  auth_source TEXT NOT NULL,
  task_id TEXT NOT NULL,
  payload_digest TEXT NOT NULL,
  result_kind TEXT NOT NULL,
  result_id INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(project_id, command_family, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_server_write_idempotency_task ON server_write_idempotency(project_id, task_id, created_at);
