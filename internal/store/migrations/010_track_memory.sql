CREATE TABLE IF NOT EXISTS track_memory (
  project_id TEXT NOT NULL,
  track_id TEXT NOT NULL,
  title TEXT,
  purpose TEXT,
  operating_mode TEXT,
  active_scope TEXT,
  current_objective TEXT,
  decisions TEXT,
  blockers TEXT,
  open_questions TEXT,
  next_actions TEXT,
  source_checkpoint_ids TEXT,
  source_evidence_ids TEXT,
  source_review_ids TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(project_id, track_id)
);
CREATE INDEX IF NOT EXISTS idx_track_memory_updated ON track_memory(project_id, updated_at);
