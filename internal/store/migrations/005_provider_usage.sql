CREATE TABLE IF NOT EXISTS provider_usage_events (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  provider TEXT NOT NULL,
  external_session_id TEXT,
  session_id TEXT,
  task_id TEXT,
  role TEXT,
  phase TEXT,
  source TEXT NOT NULL,
  confidence TEXT NOT NULL,
  started_at TEXT,
  completed_at TEXT,
  started_token_snapshot INTEGER,
  completed_token_snapshot INTEGER,
  input_tokens INTEGER,
  cached_input_tokens INTEGER,
  uncached_input_tokens INTEGER,
  output_tokens INTEGER,
  reasoning_tokens INTEGER,
  total_tokens INTEGER,
  elapsed_seconds INTEGER,
  model TEXT,
  metadata_json TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_provider_usage_task ON provider_usage_events(project_id, task_id, created_at);
CREATE INDEX IF NOT EXISTS idx_provider_usage_provider ON provider_usage_events(project_id, provider, created_at);
CREATE INDEX IF NOT EXISTS idx_provider_usage_role ON provider_usage_events(project_id, role, created_at);
CREATE INDEX IF NOT EXISTS idx_provider_usage_created ON provider_usage_events(project_id, created_at);
