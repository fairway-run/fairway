CREATE TABLE IF NOT EXISTS task_definitions (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  parent_id TEXT,
  kind TEXT,
  title TEXT NOT NULL,
  role TEXT NOT NULL,
  notes TEXT,
  acceptance_checks TEXT,
  dependencies TEXT,
  priority INTEGER,
  sequence INTEGER,
  created_at TEXT NOT NULL,
  created_by TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(project_id, id),
  FOREIGN KEY(project_id, parent_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_task_definitions_parent ON task_definitions(project_id, parent_id);

CREATE TABLE IF NOT EXISTS task_state (
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  status TEXT NOT NULL,
  owner TEXT,
  claimant TEXT,
  branch TEXT,
  claimed_at TEXT,
  completed_at TEXT,
  commit_sha TEXT,
  review_required INTEGER NOT NULL DEFAULT 0,
  review_status TEXT,
  reviewer TEXT,
  reviewed_at TEXT,
  review_note TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(project_id, task_id),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_task_state_owner_status ON task_state(project_id, owner, status);
CREATE INDEX IF NOT EXISTS idx_task_state_status ON task_state(project_id, status);
CREATE INDEX IF NOT EXISTS idx_task_state_claimant ON task_state(project_id, claimant);

CREATE TABLE IF NOT EXISTS task_state_history (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  from_status TEXT,
  to_status TEXT NOT NULL,
  from_owner TEXT,
  to_owner TEXT,
  from_branch TEXT,
  to_branch TEXT,
  from_commit_sha TEXT,
  to_commit_sha TEXT,
  command_source TEXT,
  actor TEXT NOT NULL,
  reason TEXT,
  at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_state(project_id, task_id)
);
CREATE INDEX IF NOT EXISTS idx_task_state_history_task ON task_state_history(project_id, task_id, at);

CREATE TABLE IF NOT EXISTS task_handoffs (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  from_role TEXT NOT NULL,
  to_role TEXT NOT NULL,
  payload TEXT,
  commit_sha TEXT,
  changed_files TEXT,
  commands TEXT,
  results TEXT,
  risks TEXT,
  blockers TEXT,
  next_step TEXT,
  acknowledged_at TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_state(project_id, task_id)
);
CREATE INDEX IF NOT EXISTS idx_task_handoffs_to_role ON task_handoffs(project_id, to_role, acknowledged_at);

CREATE TABLE IF NOT EXISTS task_evidence (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  handoff_id INTEGER,
  command_text TEXT,
  result TEXT,
  artifact_path TEXT,
  artifact_type TEXT,
  duration_seconds INTEGER,
  notes TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_state(project_id, task_id)
);

CREATE TABLE IF NOT EXISTS task_reviews (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  reviewer TEXT NOT NULL,
  verdict TEXT NOT NULL,
  reviewed_commit_sha TEXT,
  route_reason TEXT,
  notes TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_state(project_id, task_id)
);

CREATE TABLE IF NOT EXISTS agent_sessions (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  role TEXT NOT NULL,
  lane TEXT,
  worktree_path TEXT,
  branch TEXT,
  session_backend TEXT,
  provider TEXT,
  session_name TEXT,
  task_id TEXT,
  pid INTEGER,
  tmux_pane TEXT,
  transcript_path TEXT,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  last_heartbeat_at TEXT,
  ended_at TEXT,
  exit_code INTEGER,
  end_reason TEXT,
  PRIMARY KEY(project_id, id),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_role_live ON agent_sessions(project_id, role, ended_at);

CREATE TABLE IF NOT EXISTS task_checkpoints (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  state TEXT NOT NULL,
  owner TEXT,
  target_close_by TEXT,
  summary TEXT NOT NULL,
  artifact_path TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_task_checkpoints_task ON task_checkpoints(project_id, task_id, created_at);
CREATE INDEX IF NOT EXISTS idx_task_checkpoints_state_target ON task_checkpoints(project_id, state, target_close_by);

CREATE TABLE IF NOT EXISTS task_watchers (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  owner TEXT,
  process TEXT,
  command TEXT,
  success TEXT,
  failure TEXT,
  status TEXT NOT NULL,
  result TEXT,
  artifact_path TEXT,
  duration_seconds INTEGER,
  notes TEXT,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  PRIMARY KEY(project_id, id),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_task_watchers_status ON task_watchers(project_id, status, started_at);

CREATE TABLE IF NOT EXISTS tracker_links (
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  provider TEXT NOT NULL,
  external_id TEXT NOT NULL,
  url TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(project_id, task_id, provider),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_tracker_links_external ON tracker_links(project_id, provider, external_id);
