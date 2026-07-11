.headers on
.mode column

-- FW-295 read-only cohort definitions. This file contains SELECT statements
-- only and is intended for: sqlite3 .fairway/state.db < this-file

WITH cohort(task_id, grp) AS (
  VALUES
    ('FW-274','baseline'), ('FW-278','baseline'),
    ('FW-279','baseline'), ('FW-280','baseline'),
    ('FW-303','common'), ('FW-293','common'), ('FW-306','common'),
    ('FW-294','common'), ('FW-299','common'), ('FW-297','common'),
    ('FW-298','common'), ('FW-300','common'), ('FW-301','common')
), rows AS (
  SELECT
    c.grp,
    c.task_id,
    (SELECT COUNT(*) FROM task_evidence e WHERE e.task_id=c.task_id) evidence,
    (SELECT COUNT(*) FROM task_checkpoints p WHERE p.task_id=c.task_id) checkpoints,
    (SELECT COUNT(*) FROM task_reviews r WHERE r.task_id=c.task_id) reviews,
    (SELECT COUNT(*) FROM task_reviews r WHERE r.task_id=c.task_id AND r.verdict IN ('changes','reject')) review_findings,
    (SELECT COUNT(*) FROM task_notifications n WHERE n.task_id=c.task_id) notifications,
    (SELECT COUNT(*) FROM task_handoffs h WHERE h.task_id=c.task_id) handoffs,
    (SELECT COUNT(*) FROM task_state_history sh WHERE sh.task_id=c.task_id) transitions
  FROM cohort c
)
SELECT
  grp,
  COUNT(*) tasks,
  SUM(evidence) evidence,
  ROUND(AVG(checkpoints), 2) checkpoints_per_task,
  ROUND(AVG(reviews), 2) reviews_per_task,
  SUM(review_findings) review_findings,
  ROUND(AVG(notifications), 2) notifications_per_task,
  SUM(handoffs) handoffs,
  ROUND(AVG(transitions), 2) task_state_transitions_per_task
FROM rows
GROUP BY grp;

WITH cohort(task_id, grp) AS (
  VALUES
    ('FW-274','baseline'), ('FW-278','baseline'),
    ('FW-279','baseline'), ('FW-280','baseline'),
    ('FW-303','common'), ('FW-293','common'), ('FW-306','common'),
    ('FW-294','common'), ('FW-299','common'), ('FW-297','common'),
    ('FW-298','common'), ('FW-300','common'), ('FW-301','common')
), spans AS (
  SELECT
    c.grp,
    c.task_id,
    (SELECT MIN(at) FROM task_state_history h WHERE h.task_id=c.task_id AND h.to_status='in_progress') started,
    (SELECT MAX(at) FROM task_state_history h WHERE h.task_id=c.task_id AND h.to_status='done') completed,
    (SELECT MIN(created_at) FROM task_evidence e WHERE e.task_id=c.task_id) first_evidence,
    (SELECT MAX(created_at) FROM task_evidence e
      WHERE e.task_id=c.task_id
        AND e.created_at <= (SELECT MAX(at) FROM task_state_history h WHERE h.task_id=c.task_id AND h.to_status='done')
        AND e.artifact_type NOT IN ('commit','ci','docs','push-intent')) last_validation
  FROM cohort c
)
SELECT
  grp,
  COUNT(*) tasks,
  ROUND(AVG((julianday(first_evidence)-julianday(started))*86400), 1) avg_start_to_first_evidence_seconds,
  ROUND(AVG((julianday(completed)-julianday(started))*86400), 1) avg_active_to_done_seconds,
  ROUND(AVG((julianday(completed)-julianday(first_evidence))*86400), 1) avg_first_evidence_to_done_seconds,
  ROUND(AVG((julianday(completed)-julianday(last_validation))*86400), 1) avg_validation_to_done_seconds
FROM spans
GROUP BY grp;

WITH cohort(task_id, grp) AS (
  VALUES
    ('FW-274','baseline'), ('FW-278','baseline'),
    ('FW-279','baseline'), ('FW-280','baseline'),
    ('FW-303','common'), ('FW-293','common'), ('FW-306','common'),
    ('FW-294','common'), ('FW-299','common'), ('FW-297','common'),
    ('FW-298','common'), ('FW-300','common'), ('FW-301','common')
), coverage AS (
  SELECT
    c.grp,
    c.task_id,
    EXISTS(SELECT 1 FROM task_evidence e WHERE e.task_id=c.task_id AND e.result='pass') has_pass_evidence,
    EXISTS(SELECT 1 FROM task_checkpoints p WHERE p.task_id=c.task_id AND p.state IN ('active','started')) has_active_checkpoint,
    EXISTS(SELECT 1 FROM agent_sessions s WHERE s.task_id=c.task_id) has_session,
    (SELECT COUNT(*) FROM task_evidence e WHERE e.task_id=c.task_id AND lower(e.command_text||' '||e.notes||' '||e.artifact_type) LIKE '%rollback%') rollback_rows
  FROM cohort c
)
SELECT
  grp,
  COUNT(*) tasks,
  SUM(has_pass_evidence) with_pass_evidence,
  SUM(has_active_checkpoint) with_active_checkpoint,
  SUM(has_session) with_session,
  SUM(rollback_rows) rollback_referencing_evidence
FROM coverage
GROUP BY grp;

WITH cohort(task_id, grp) AS (
  VALUES
    ('FW-274','baseline'), ('FW-278','baseline'),
    ('FW-279','baseline'), ('FW-280','baseline'),
    ('FW-303','common'), ('FW-293','common'), ('FW-306','common'),
    ('FW-294','common'), ('FW-299','common'), ('FW-297','common'),
    ('FW-298','common'), ('FW-300','common'), ('FW-301','common')
)
SELECT
  c.grp,
  COUNT(d.id) decisions,
  SUM(CASE WHEN a.quality_state='accepted' THEN 1 ELSE 0 END) accepted,
  SUM(CASE WHEN a.quality_state='insufficient' THEN 1 ELSE 0 END) insufficient
FROM cohort c
LEFT JOIN task_decisions d ON d.task_id=c.task_id
LEFT JOIN task_decision_assessments a ON a.decision_id=d.id
GROUP BY c.grp;
