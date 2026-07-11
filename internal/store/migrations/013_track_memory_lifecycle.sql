ALTER TABLE track_memory ADD COLUMN owner TEXT NOT NULL DEFAULT '';
ALTER TABLE track_memory ADD COLUMN review_by TEXT NOT NULL DEFAULT '';
ALTER TABLE track_memory ADD COLUMN disposition TEXT NOT NULL DEFAULT 'active';
ALTER TABLE track_memory ADD COLUMN promotion_target TEXT NOT NULL DEFAULT '';
ALTER TABLE track_memory ADD COLUMN canonical_commit TEXT NOT NULL DEFAULT '';
ALTER TABLE track_memory ADD COLUMN superseded_by_track_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS track_memory_lifecycle (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  track_id TEXT NOT NULL,
  from_disposition TEXT NOT NULL,
  to_disposition TEXT NOT NULL,
  reason TEXT NOT NULL,
  promotion_target TEXT,
  canonical_commit TEXT,
  superseded_by_track_id TEXT,
  actor TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_track_memory_lifecycle_track
  ON track_memory_lifecycle(project_id, track_id, id);
