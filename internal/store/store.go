package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrAlreadyClaimed    = errors.New("task already claimed")
	ErrNotFound          = errors.New("task not found")
	ErrInvalidTransition = errors.New("invalid transition")
	ErrInvalidTaskID     = errors.New("invalid task id")
)

var defaultTaskIDPattern = regexp.MustCompile(`^[A-Z]+-[0-9]+$`)

type Store struct {
	db        *sql.DB
	projectID string
}

type TaskDefinition struct {
	ID               string   `json:"id" yaml:"id"`
	ParentID         string   `json:"parent_id" yaml:"parent_id"`
	Kind             string   `json:"kind" yaml:"kind"`
	Title            string   `json:"title" yaml:"title"`
	Role             string   `json:"role" yaml:"role"`
	Notes            string   `json:"notes" yaml:"notes"`
	AcceptanceChecks []string `json:"acceptance_checks" yaml:"acceptance_checks"`
	Dependencies     []string `json:"dependencies" yaml:"dependencies"`
	Priority         *int     `json:"priority" yaml:"priority"`
	Sequence         *int     `json:"sequence" yaml:"sequence"`
}

type Task struct {
	Definition   TaskDefinition
	Status       string
	Owner        string
	Claimant     string
	Branch       string
	ReviewStatus string
	UpdatedAt    string
}

type Evidence struct {
	CommandText     string
	Result          string
	ArtifactPath    string
	ArtifactType    string
	DurationSeconds *int
	Notes           string
}

type Handoff struct {
	ToRole  string
	Payload string
}

type Review struct {
	Reviewer string
	Verdict  string
	Reason   string
	Commit   string
}

type Activity struct {
	Kind      string
	TaskID    string
	Summary   string
	Actor     string
	CreatedAt string
}

type Transition struct {
	FromStatus string
	ToStatus   string
	Actor      string
	Reason     string
	At         string
}

type Health struct {
	InProgress            int
	BlockedOver24h        int
	UnacknowledgedHandoff int
	UnroutedReviews       int
}

func Open(ctx context.Context, path, projectID string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db, projectID: projectID}
	if err := s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, migration001)
	return err
}

func (s *Store) ImportTasks(ctx context.Context, tasks []TaskDefinition) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	actor := Actor()
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.Title) == "" {
			return errors.New("task id and title are required")
		}
		if !defaultTaskIDPattern.MatchString(task.ID) {
			return fmt.Errorf("%w: %s", ErrInvalidTaskID, task.ID)
		}
		if task.Kind == "" {
			task.Kind = "task"
		}
		acceptance, err := json.Marshal(task.AcceptanceChecks)
		if err != nil {
			return err
		}
		deps, err := json.Marshal(task.Dependencies)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO task_definitions
  (project_id, id, parent_id, kind, title, role, notes, acceptance_checks, dependencies, priority, sequence, created_at, created_by, updated_at)
VALUES (?, ?, nullif(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(project_id, id) DO UPDATE SET
  parent_id=excluded.parent_id,
  kind=excluded.kind,
  title=excluded.title,
  role=excluded.role,
  notes=excluded.notes,
  acceptance_checks=excluded.acceptance_checks,
  dependencies=excluded.dependencies,
  priority=excluded.priority,
  sequence=excluded.sequence,
  updated_at=excluded.updated_at`,
			s.projectID, task.ID, task.ParentID, task.Kind, task.Title, task.Role, task.Notes, string(acceptance), string(deps), task.Priority, task.Sequence, now, actor, now)
		if err != nil {
			return fmt.Errorf("upsert task %s: %w", task.ID, err)
		}
		res, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO task_state
  (project_id, task_id, status, owner, review_required, review_status, updated_at)
VALUES (?, ?, 'todo', ?, 0, 'not_required', ?)`, s.projectID, task.ID, task.Role, now)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			if err := insertHistory(ctx, tx, s.projectID, task.ID, "", "todo", "", task.Role, "", actor, "import"); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) Ready(ctx context.Context, role string) ([]Task, error) {
	args := []any{s.projectID}
	roleSQL := ""
	if role != "" {
		roleSQL = " AND d.role = ?"
		args = append(args, role)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT d.id, d.parent_id, d.kind, d.title, d.role, d.notes, d.acceptance_checks, d.dependencies,
       d.priority, d.sequence, st.status, st.owner, st.claimant, st.branch, st.review_status, st.updated_at
FROM task_definitions d
JOIN task_state st ON st.project_id = d.project_id AND st.task_id = d.id
WHERE d.project_id = ? AND st.status = 'todo'`+roleSQL+`
ORDER BY COALESCE(d.priority, 9999), COALESCE(d.sequence, 9999), d.created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates, err := scanTasks(rows)
	if err != nil {
		return nil, err
	}
	statuses, err := s.statusMap(ctx)
	if err != nil {
		return nil, err
	}
	ready := candidates[:0]
	for _, task := range candidates {
		ok := true
		for _, dep := range task.Definition.Dependencies {
			if statuses[dep] != "done" {
				ok = false
				break
			}
		}
		if ok {
			ready = append(ready, task)
		}
	}
	return ready, nil
}

func (s *Store) Claim(ctx context.Context, taskID, owner, branch string) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()
	var fromStatus, fromOwner, claimant string
	err = conn.QueryRowContext(ctx, `SELECT status, COALESCE(owner, ''), COALESCE(claimant, '') FROM task_state WHERE project_id = ? AND task_id = ?`, s.projectID, taskID).Scan(&fromStatus, &fromOwner, &claimant)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if claimant != "" {
		return ErrAlreadyClaimed
	}
	if fromStatus != "todo" && fromStatus != "blocked" {
		return fmt.Errorf("%w: cannot claim from %s", ErrInvalidTransition, fromStatus)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	actor := Actor()
	res, err := conn.ExecContext(ctx, `
UPDATE task_state
SET status='in_progress', owner=?, claimant=?, branch=?, claimed_at=?, updated_at=?
WHERE project_id=? AND task_id=? AND status IN ('todo','blocked') AND claimant IS NULL`,
		owner, actor, branch, now, now, s.projectID, taskID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAlreadyClaimed
	}
	if err := insertHistoryExec(ctx, conn, s.projectID, taskID, fromStatus, "in_progress", fromOwner, owner, branch, actor, "claim"); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *Store) CurrentStatus(ctx context.Context, taskID string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return status, err
}

func (s *Store) HasEvidence(ctx context.Context, taskID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_evidence WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&count)
	return count > 0, err
}

func (s *Store) HasApprovedReview(ctx context.Context, taskID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_reviews WHERE project_id=? AND task_id=? AND verdict='approve'`, s.projectID, taskID).Scan(&count)
	return count > 0, err
}

func (s *Store) SetStatus(ctx context.Context, taskID, status, reason string, requireBlockedReason bool) error {
	if status == "blocked" && requireBlockedReason && strings.TrimSpace(reason) == "" {
		return errors.New("reason is required when blocking a task")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var fromStatus, owner, branch string
	err = tx.QueryRowContext(ctx, `SELECT status, COALESCE(owner,''), COALESCE(branch,'') FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&fromStatus, &owner, &branch)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	completed := any(nil)
	if status == "done" {
		completed = now
	}
	claimantSQL := "claimant"
	if status != "in_progress" {
		claimantSQL = "NULL"
	}
	_, err = tx.ExecContext(ctx, `UPDATE task_state SET status=?, claimant=`+claimantSQL+`, completed_at=?, updated_at=? WHERE project_id=? AND task_id=?`, status, completed, now, s.projectID, taskID)
	if err != nil {
		return err
	}
	if err := insertHistory(ctx, tx, s.projectID, taskID, fromStatus, status, owner, owner, branch, Actor(), reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RecordEvidence(ctx context.Context, taskID string, ev Evidence) error {
	if ev.CommandText == "" {
		return errors.New("command text is required")
	}
	switch ev.Result {
	case "pass", "fail", "partial", "skipped", "blocked":
	default:
		return fmt.Errorf("invalid evidence result %q", ev.Result)
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO task_evidence
  (project_id, task_id, command_text, result, artifact_path, artifact_type, duration_seconds, notes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.projectID, taskID, ev.CommandText, ev.Result, ev.ArtifactPath, ev.ArtifactType, ev.DurationSeconds, ev.Notes, time.Now().UTC().Format(time.RFC3339Nano))
	return checkWriteResult(res, err)
}

func (s *Store) RecordHandoff(ctx context.Context, taskID string, h Handoff) error {
	if h.ToRole == "" {
		return errors.New("handoff target role is required")
	}
	if h.Payload == "" {
		return errors.New("handoff payload is required")
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO task_handoffs (project_id, task_id, from_role, to_role, payload, created_at)
SELECT project_id, task_id, COALESCE(owner, ''), ?, ?, ?
FROM task_state WHERE project_id=? AND task_id=?`,
		h.ToRole, h.Payload, time.Now().UTC().Format(time.RFC3339Nano), s.projectID, taskID)
	return checkWriteResult(res, err)
}

func (s *Store) RecordReview(ctx context.Context, taskID string, r Review) error {
	if r.Reviewer == "" {
		return errors.New("reviewer is required")
	}
	switch r.Verdict {
	case "approve", "changes", "reject":
	default:
		return fmt.Errorf("invalid review verdict %q", r.Verdict)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `
INSERT INTO task_reviews (project_id, task_id, reviewer, verdict, reviewed_commit_sha, route_reason, notes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, s.projectID, taskID, r.Reviewer, r.Verdict, r.Commit, r.Reason, r.Reason, now)
	if err := checkWriteResult(res, err); err != nil {
		return err
	}
	status := "changes_requested"
	if r.Verdict == "approve" {
		status = "approved"
	}
	res, err = tx.ExecContext(ctx, `
UPDATE task_state
SET review_required=1, review_status=?, reviewer=?, reviewed_at=?, review_note=?, updated_at=?
WHERE project_id=? AND task_id=?`, status, r.Reviewer, now, r.Reason, now, s.projectID, taskID)
	if err := checkWriteResult(res, err); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) TaskDetail(ctx context.Context, taskID string) (Task, []Transition, []Evidence, []Handoff, []Review, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT d.id, d.parent_id, d.kind, d.title, d.role, d.notes, d.acceptance_checks, d.dependencies,
       d.priority, d.sequence, st.status, st.owner, st.claimant, st.branch, st.review_status, st.updated_at
FROM task_definitions d JOIN task_state st ON st.project_id=d.project_id AND st.task_id=d.id
WHERE d.project_id=? AND d.id=?`, s.projectID, taskID)
	task, err := scanTask(row)
	if err != nil {
		return Task{}, nil, nil, nil, nil, err
	}
	transitions, err := s.transitions(ctx, taskID)
	if err != nil {
		return Task{}, nil, nil, nil, nil, err
	}
	evidence, err := s.evidence(ctx, taskID)
	if err != nil {
		return Task{}, nil, nil, nil, nil, err
	}
	handoffs, err := s.handoffs(ctx, taskID)
	if err != nil {
		return Task{}, nil, nil, nil, nil, err
	}
	reviews, err := s.reviews(ctx, taskID)
	if err != nil {
		return Task{}, nil, nil, nil, nil, err
	}
	return task, transitions, evidence, handoffs, reviews, nil
}

func (s *Store) AllTasks(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT d.id, d.parent_id, d.kind, d.title, d.role, d.notes, d.acceptance_checks, d.dependencies,
       d.priority, d.sequence, st.status, st.owner, st.claimant, st.branch, st.review_status, st.updated_at
FROM task_definitions d JOIN task_state st ON st.project_id=d.project_id AND st.task_id=d.id
WHERE d.project_id=?
ORDER BY d.role, st.status, COALESCE(d.priority, 9999), d.created_at`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) Activity(ctx context.Context, limit int) ([]Activity, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT kind, task_id, summary, actor, created_at
FROM (
  SELECT 'state' AS kind, task_id, COALESCE(from_status, 'new') || ' -> ' || to_status AS summary, actor, at AS created_at
    FROM task_state_history WHERE project_id=?
  UNION ALL
  SELECT 'evidence', task_id, COALESCE(result, '') || ' ' || COALESCE(command_text, ''), '', created_at
    FROM task_evidence WHERE project_id=?
  UNION ALL
  SELECT 'handoff', task_id, 'to ' || to_role || ': ' || COALESCE(payload, ''), from_role, created_at
    FROM task_handoffs WHERE project_id=?
  UNION ALL
  SELECT 'review', task_id, verdict || ' by ' || reviewer, reviewer, created_at
    FROM task_reviews WHERE project_id=?
)
ORDER BY created_at DESC
LIMIT ?`, s.projectID, s.projectID, s.projectID, s.projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		var item Activity
		if err := rows.Scan(&item.Kind, &item.TaskID, &item.Summary, &item.Actor, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Health(ctx context.Context) (Health, error) {
	var h Health
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_state WHERE project_id=? AND status='in_progress'`, s.projectID).Scan(&h.InProgress); err != nil {
		return Health{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_state WHERE project_id=? AND status='blocked' AND updated_at < ?`, s.projectID, time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339Nano)).Scan(&h.BlockedOver24h); err != nil {
		return Health{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_handoffs WHERE project_id=? AND acknowledged_at IS NULL`, s.projectID).Scan(&h.UnacknowledgedHandoff); err != nil {
		return Health{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_state WHERE project_id=? AND review_required=1 AND COALESCE(review_status, '') IN ('', 'pending')`, s.projectID).Scan(&h.UnroutedReviews); err != nil {
		return Health{}, err
	}
	return h, nil
}

func (s *Store) statusMap(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, status FROM task_state WHERE project_id=?`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		out[id] = status
	}
	return out, rows.Err()
}

func checkWriteResult(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func insertHistory(ctx context.Context, tx *sql.Tx, projectID, taskID, fromStatus, toStatus, fromOwner, toOwner, branch, actor, reason string) error {
	return insertHistoryExec(ctx, tx, projectID, taskID, fromStatus, toStatus, fromOwner, toOwner, branch, actor, reason)
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertHistoryExec(ctx context.Context, ex execer, projectID, taskID, fromStatus, toStatus, fromOwner, toOwner, branch, actor, reason string) error {
	_, err := ex.ExecContext(ctx, `
INSERT INTO task_state_history
  (project_id, task_id, from_status, to_status, from_owner, to_owner, to_branch, command_source, actor, reason, at)
VALUES (?, ?, nullif(?, ''), ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), 'cli', ?, nullif(?, ''), ?)`,
		projectID, taskID, fromStatus, toStatus, fromOwner, toOwner, branch, actor, reason, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (Task, error) {
	var task Task
	var acceptance, deps string
	var parent, kind, notes, owner, claimant, branch, reviewStatus, updated sql.NullString
	var priority, sequence sql.NullInt64
	err := row.Scan(&task.Definition.ID, &parent, &kind, &task.Definition.Title, &task.Definition.Role, &notes, &acceptance, &deps, &priority, &sequence, &task.Status, &owner, &claimant, &branch, &reviewStatus, &updated)
	if err != nil {
		return Task{}, err
	}
	task.Definition.ParentID = parent.String
	task.Definition.Kind = kind.String
	task.Definition.Notes = notes.String
	_ = json.Unmarshal([]byte(acceptance), &task.Definition.AcceptanceChecks)
	_ = json.Unmarshal([]byte(deps), &task.Definition.Dependencies)
	if priority.Valid {
		v := int(priority.Int64)
		task.Definition.Priority = &v
	}
	if sequence.Valid {
		v := int(sequence.Int64)
		task.Definition.Sequence = &v
	}
	task.Owner = owner.String
	task.Claimant = claimant.String
	task.Branch = branch.String
	task.ReviewStatus = reviewStatus.String
	task.UpdatedAt = updated.String
	return task, nil
}

func scanTasks(rows *sql.Rows) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) evidence(ctx context.Context, taskID string) ([]Evidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT command_text, result, artifact_path, artifact_type, duration_seconds, notes FROM task_evidence WHERE project_id=? AND task_id=? ORDER BY created_at`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Evidence
	for rows.Next() {
		var ev Evidence
		var dur sql.NullInt64
		if err := rows.Scan(&ev.CommandText, &ev.Result, &ev.ArtifactPath, &ev.ArtifactType, &dur, &ev.Notes); err != nil {
			return nil, err
		}
		if dur.Valid {
			v := int(dur.Int64)
			ev.DurationSeconds = &v
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) transitions(ctx context.Context, taskID string) ([]Transition, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(from_status, ''), to_status, actor, COALESCE(reason, ''), at FROM task_state_history WHERE project_id=? AND task_id=? ORDER BY at`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transition
	for rows.Next() {
		var tr Transition
		if err := rows.Scan(&tr.FromStatus, &tr.ToStatus, &tr.Actor, &tr.Reason, &tr.At); err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

func (s *Store) handoffs(ctx context.Context, taskID string) ([]Handoff, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT to_role, payload FROM task_handoffs WHERE project_id=? AND task_id=? ORDER BY created_at`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Handoff
	for rows.Next() {
		var h Handoff
		if err := rows.Scan(&h.ToRole, &h.Payload); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) reviews(ctx context.Context, taskID string) ([]Review, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT reviewer, verdict, notes, reviewed_commit_sha FROM task_reviews WHERE project_id=? AND task_id=? ORDER BY created_at`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.Reviewer, &r.Verdict, &r.Reason, &r.Commit); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func Actor() string {
	if sid := os.Getenv("FAIRWAY_SESSION_ID"); sid != "" {
		return sid
	}
	u, _ := user.Current()
	name := "unknown"
	if u != nil && u.Username != "" {
		name = u.Username
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}
	if short, _, err := net.SplitHostPort(host); err == nil {
		host = short
	}
	return name + "@" + host
}

const migration001 = `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, datetime('now'));

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
`
