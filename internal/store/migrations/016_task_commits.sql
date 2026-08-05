CREATE TABLE task_commits (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  commit_sha TEXT NOT NULL,
  association_kind TEXT NOT NULL,
  source TEXT NOT NULL,
  actor TEXT NOT NULL,
  created_at TEXT NOT NULL,
  CHECK(association_kind IN ('work_base', 'work', 'completion', 'manual')),
  UNIQUE(project_id, task_id, commit_sha, association_kind),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id)
);

CREATE INDEX idx_task_commits_task
  ON task_commits(project_id, task_id, created_at, id);
CREATE INDEX idx_task_commits_commit
  ON task_commits(project_id, commit_sha, task_id);
