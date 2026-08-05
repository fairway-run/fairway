CREATE TABLE control_friction_samples (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  control_id TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT,
  resolved_at TEXT,
  started_by TEXT,
  resolved_by TEXT,
  source_ref TEXT,
  reason TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  CHECK(status IN ('open', 'resolved', 'unavailable')),
  CHECK(
    (status = 'open' AND started_at IS NOT NULL AND started_by IS NOT NULL AND resolved_at IS NULL AND resolved_by IS NULL) OR
    (status = 'resolved' AND started_at IS NOT NULL AND started_by IS NOT NULL AND resolved_at IS NOT NULL AND resolved_by IS NOT NULL) OR
    (status = 'unavailable' AND started_at IS NULL AND started_by IS NULL AND resolved_at IS NULL AND resolved_by IS NOT NULL)
  ),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);

CREATE INDEX idx_control_friction_samples_task
  ON control_friction_samples(project_id, task_id, control_id, created_at, id);
