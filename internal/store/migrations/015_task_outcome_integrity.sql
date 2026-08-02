CREATE TABLE IF NOT EXISTS task_outcome_transition_v015 (
  outcome_id INTEGER PRIMARY KEY,
  transition_id INTEGER
);

DROP INDEX IF EXISTS idx_task_outcomes_task;
DROP INDEX IF EXISTS idx_task_outcomes_related;

ALTER TABLE task_outcomes RENAME TO task_outcomes_v014;

CREATE UNIQUE INDEX IF NOT EXISTS idx_task_state_history_project_task_id
  ON task_state_history(project_id, task_id, id);

CREATE TABLE task_outcomes (
  id INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  source_ref TEXT,
  related_task_id TEXT,
  transition_id INTEGER,
  notes TEXT,
  actor TEXT NOT NULL,
  created_at TEXT NOT NULL,
  CHECK(kind IN ('incident', 'rollback', 'reopen', 'corrective', 'superseding_task')),
  CHECK((kind IN ('incident', 'rollback') AND source_ref IS NOT NULL) OR kind NOT IN ('incident', 'rollback')),
  CHECK((kind IN ('corrective', 'superseding_task') AND related_task_id IS NOT NULL) OR (kind NOT IN ('corrective', 'superseding_task') AND related_task_id IS NULL)),
  CHECK(related_task_id IS NULL OR related_task_id <> task_id),
  CHECK((kind = 'reopen' AND transition_id IS NOT NULL) OR (kind <> 'reopen' AND transition_id IS NULL)),
  FOREIGN KEY(project_id, task_id) REFERENCES task_definitions(project_id, id),
  FOREIGN KEY(project_id, related_task_id) REFERENCES task_definitions(project_id, id),
  FOREIGN KEY(project_id, task_id, transition_id) REFERENCES task_state_history(project_id, task_id, id)
);

INSERT INTO task_outcomes
  (id, project_id, task_id, kind, occurred_at, source_ref, related_task_id, transition_id, notes, actor, created_at)
SELECT o.id, o.project_id, o.task_id, o.kind, o.occurred_at, o.source_ref, o.related_task_id, s.transition_id, o.notes, o.actor, o.created_at
FROM task_outcomes_v014 o
LEFT JOIN task_outcome_transition_v015 s ON s.outcome_id = o.id;

DROP TABLE task_outcomes_v014;
DROP TABLE task_outcome_transition_v015;

CREATE INDEX idx_task_outcomes_task
  ON task_outcomes(project_id, task_id, occurred_at, id);
CREATE INDEX idx_task_outcomes_related
  ON task_outcomes(project_id, related_task_id, occurred_at, id);
