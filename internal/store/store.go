package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrAlreadyClaimed      = errors.New("task already claimed")
	ErrNotFound            = errors.New("task not found")
	ErrInvalidTransition   = errors.New("invalid transition")
	ErrInvalidTaskID       = errors.New("invalid task id")
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
)

type TaskStateConflict struct {
	TaskID          string
	ExpectedStatus  string
	ActualStatus    string
	ActualOwner     string
	ActualUpdatedAt string
}

func (c TaskStateConflict) Error() string {
	return "task state conflict"
}

var defaultTaskIDPattern = regexp.MustCompile(`^[A-Z]+-[0-9]+$`)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct {
	db            *sql.DB
	projectID     string
	taskIDPattern *regexp.Regexp
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
	Profile          string   `json:"profile" yaml:"profile"`
	OwningDomain     string   `json:"owning_domain" yaml:"owning_domain"`
	OwningLayer      string   `json:"owning_layer" yaml:"owning_layer"`
	SourcePaths      []string `json:"source_paths" yaml:"source_paths"`
	TargetPaths      []string `json:"target_paths" yaml:"target_paths"`
	ReviewDomains    []string `json:"review_domains" yaml:"review_domains"`
	Tags             []string `json:"tags" yaml:"tags"`
	RiskLevel        string   `json:"risk_level" yaml:"risk_level"`
	MigrationType    string   `json:"migration_type" yaml:"migration_type"`
}

type ImportedTaskState struct {
	TaskID      string `json:"task_id" yaml:"task_id"`
	Status      string `json:"status" yaml:"status"`
	Owner       string `json:"owner" yaml:"owner"`
	Branch      string `json:"branch" yaml:"branch"`
	CompletedAt string `json:"completed_at" yaml:"completed_at"`
	CommitSHA   string `json:"commit_sha" yaml:"commit_sha"`
}

type Task struct {
	Definition   TaskDefinition
	Project      string
	Status       string
	Owner        string
	Claimant     string
	Branch       string
	CompletedAt  string
	CommitSHA    string
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
	CreatedAt       string
}

type TaskDecision struct {
	ID                 int64    `json:"id"`
	TaskID             string   `json:"task_id"`
	Decision           string   `json:"decision"`
	Trigger            string   `json:"trigger"`
	Alternatives       []string `json:"alternatives"`
	Chosen             string   `json:"chosen"`
	Reason             string   `json:"reason"`
	ScopeAdded         []string `json:"scope_added,omitempty"`
	Risk               string   `json:"risk"`
	ValidationRefs     []string `json:"validation_refs"`
	FactRefs           []string `json:"fact_refs"`
	SupersedesID       int64    `json:"supersedes_id,omitempty"`
	SupersededByID     int64    `json:"superseded_by_id,omitempty"`
	CreatedBy          string   `json:"created_by"`
	CreatedAt          string   `json:"created_at"`
	QualityState       string   `json:"quality_state"`
	QualityReviewer    string   `json:"quality_reviewer,omitempty"`
	QualityReason      string   `json:"quality_reason,omitempty"`
	AcceptanceRequired bool     `json:"acceptance_required"`
	AuthorityBoundary  string   `json:"authority_boundary"`
}

type Handoff struct {
	ID             int64  `json:"id"`
	FromRole       string `json:"from_role,omitempty"`
	ToRole         string `json:"to_role"`
	Payload        string `json:"payload"`
	AcknowledgedAt string `json:"acknowledged_at,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
}

type Review struct {
	Reviewer  string
	Domain    string
	Verdict   string
	Reason    string
	Commit    string
	CreatedAt string
}

type Notification struct {
	ID        int64  `json:"id"`
	TaskID    string `json:"task_id"`
	HandoffID *int64 `json:"handoff_id,omitempty"`
	Domain    string `json:"domain"`
	Provider  string `json:"provider,omitempty"`
	Target    string `json:"target,omitempty"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"created_at"`
}

type HandoffNotificationGap struct {
	TaskID             string `json:"task_id"`
	Role               string `json:"role"`
	Domain             string `json:"domain"`
	HandoffID          int64  `json:"handoff_id"`
	LastHandoffAt      string `json:"last_handoff_at"`
	LastNotificationAt string `json:"last_notification_at,omitempty"`
	LastState          string `json:"last_state,omitempty"`
	NotificationStatus string `json:"notification_status,omitempty"`
}

type HandoffNotificationGapOptions struct {
	TerminalStatuses     []string
	SentStaleBefore      string
	ExcludePayloadPrefix string
}

type Session struct {
	ID              string `json:"id"`
	Role            string `json:"role"`
	Lane            string `json:"lane"`
	WorktreePath    string `json:"worktree_path"`
	Branch          string `json:"branch"`
	SessionBackend  string `json:"session_backend"`
	Provider        string `json:"provider"`
	SessionName     string `json:"session_name"`
	TaskID          string `json:"task_id"`
	PID             *int   `json:"pid,omitempty"`
	TmuxPane        string `json:"tmux_pane"`
	TranscriptPath  string `json:"transcript_path"`
	MonitorKind     string `json:"monitor_kind"`
	AutomationID    string `json:"automation_id"`
	ExternalRunID   string `json:"external_run_id"`
	PollCommand     string `json:"poll_command"`
	ManualUntil     string `json:"manual_until"`
	Status          string `json:"status"`
	StartedAt       string `json:"started_at"`
	LastHeartbeatAt string `json:"last_heartbeat_at"`
	EndedAt         string `json:"ended_at"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	EndReason       string `json:"end_reason"`
}

type WorkStartResult struct {
	TaskID        string `json:"task_id"`
	Status        string `json:"status"`
	Owner         string `json:"owner"`
	SessionID     string `json:"session_id"`
	CheckpointID  int64  `json:"checkpoint_id"`
	AlreadyActive bool   `json:"already_active"`
}

type WorkCloseResult struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
	CommitSHA string `json:"commit_sha"`
}

type Checkpoint struct {
	ID            int64  `json:"id"`
	TaskID        string `json:"task_id"`
	State         string `json:"state"`
	Owner         string `json:"owner"`
	TargetCloseBy string `json:"target_close_by"`
	Summary       string `json:"summary"`
	ArtifactPath  string `json:"artifact_path"`
	CreatedAt     string `json:"created_at"`
}

type Watcher struct {
	ID              string `json:"id"`
	TaskID          string `json:"task_id"`
	Owner           string `json:"owner"`
	Process         string `json:"process"`
	Command         string `json:"command"`
	Success         string `json:"success"`
	Failure         string `json:"failure"`
	Status          string `json:"status"`
	Result          string `json:"result"`
	ArtifactPath    string `json:"artifact_path"`
	DurationSeconds *int   `json:"duration_seconds,omitempty"`
	Notes           string `json:"notes"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
}

type TrackMemory struct {
	TrackID             string   `json:"track_id"`
	Title               string   `json:"title,omitempty"`
	Purpose             string   `json:"purpose,omitempty"`
	OperatingMode       string   `json:"operating_mode,omitempty"`
	ActiveScope         string   `json:"active_scope,omitempty"`
	CurrentObjective    string   `json:"current_objective,omitempty"`
	Decisions           []string `json:"decisions,omitempty"`
	Blockers            []string `json:"blockers,omitempty"`
	OpenQuestions       []string `json:"open_questions,omitempty"`
	NextActions         []string `json:"next_actions,omitempty"`
	SourceCheckpointIDs []int64  `json:"source_checkpoint_ids,omitempty"`
	SourceEvidenceIDs   []int64  `json:"source_evidence_ids,omitempty"`
	SourceReviewIDs     []int64  `json:"source_review_ids,omitempty"`
	Owner               string   `json:"owner,omitempty"`
	ReviewBy            string   `json:"review_by,omitempty"`
	Disposition         string   `json:"disposition"`
	PromotionTarget     string   `json:"promotion_target,omitempty"`
	CanonicalCommit     string   `json:"canonical_commit,omitempty"`
	SupersededByTrackID string   `json:"superseded_by_track_id,omitempty"`
	UpdatedAt           string   `json:"updated_at,omitempty"`
}

type TrackMemoryLifecycle struct {
	ID                  int64  `json:"id"`
	TrackID             string `json:"track_id"`
	FromDisposition     string `json:"from_disposition"`
	ToDisposition       string `json:"to_disposition"`
	Reason              string `json:"reason"`
	PromotionTarget     string `json:"promotion_target,omitempty"`
	CanonicalCommit     string `json:"canonical_commit,omitempty"`
	SupersededByTrackID string `json:"superseded_by_track_id,omitempty"`
	Actor               string `json:"actor"`
	CreatedAt           string `json:"created_at"`
}

type TrackerLink struct {
	TaskID     string `json:"task_id"`
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
	URL        string `json:"url"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type ProviderUsage struct {
	ID                     int64  `json:"id"`
	Provider               string `json:"provider"`
	ExternalSessionID      string `json:"external_session_id,omitempty"`
	SessionID              string `json:"session_id,omitempty"`
	TaskID                 string `json:"task_id,omitempty"`
	Role                   string `json:"role,omitempty"`
	Phase                  string `json:"phase,omitempty"`
	Source                 string `json:"source"`
	Confidence             string `json:"confidence"`
	StartedAt              string `json:"started_at,omitempty"`
	CompletedAt            string `json:"completed_at,omitempty"`
	StartedTokenSnapshot   *int   `json:"started_token_snapshot,omitempty"`
	CompletedTokenSnapshot *int   `json:"completed_token_snapshot,omitempty"`
	InputTokens            *int   `json:"input_tokens,omitempty"`
	CachedInputTokens      *int   `json:"cached_input_tokens,omitempty"`
	UncachedInputTokens    *int   `json:"uncached_input_tokens,omitempty"`
	OutputTokens           *int   `json:"output_tokens,omitempty"`
	ReasoningTokens        *int   `json:"reasoning_tokens,omitempty"`
	TotalTokens            *int   `json:"total_tokens,omitempty"`
	ElapsedSeconds         *int   `json:"elapsed_seconds,omitempty"`
	Model                  string `json:"model,omitempty"`
	MetadataJSON           string `json:"metadata_json,omitempty"`
	CreatedAt              string `json:"created_at"`
}

type UsageRollupOptions struct {
	GroupBy string
	TaskID  string
	Since   string
	Until   string
}

type UsageRollup struct {
	Group               string `json:"group"`
	Key                 string `json:"key"`
	Events              int    `json:"events"`
	KnownTotalEvents    int    `json:"known_total_events"`
	TotalTokens         *int   `json:"total_tokens,omitempty"`
	InputTokens         *int   `json:"input_tokens,omitempty"`
	CachedInputTokens   *int   `json:"cached_input_tokens,omitempty"`
	UncachedInputTokens *int   `json:"uncached_input_tokens,omitempty"`
	OutputTokens        *int   `json:"output_tokens,omitempty"`
	ReasoningTokens     *int   `json:"reasoning_tokens,omitempty"`
	ElapsedSeconds      *int   `json:"elapsed_seconds,omitempty"`
}

type WorkBatch struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Branch             string   `json:"branch,omitempty"`
	WorktreePath       string   `json:"worktree_path,omitempty"`
	ValidationCommands []string `json:"validation_commands,omitempty"`
	ReviewDomains      []string `json:"review_domains,omitempty"`
	RollbackCriteria   string   `json:"rollback_criteria,omitempty"`
	SplitCriteria      string   `json:"split_criteria,omitempty"`
	ExpectedCI         string   `json:"expected_ci,omitempty"`
	DeployRunID        string   `json:"deploy_run_id,omitempty"`
	PipelineID         string   `json:"pipeline_id,omitempty"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
	Tasks              []string `json:"tasks,omitempty"`
}

type WorkBatchEvidence struct {
	ID           int64  `json:"id"`
	BatchID      string `json:"batch_id"`
	CommandText  string `json:"command_text"`
	Result       string `json:"result"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	ArtifactType string `json:"artifact_type,omitempty"`
	Notes        string `json:"notes,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type AuditEvent struct {
	Actor     string
	Action    string
	TaskID    string
	Detail    string
	CreatedAt string
}

type ServerWriteRequest struct {
	Actor          string
	Role           string
	AuthSource     string
	CommandFamily  string
	IdempotencyKey string
	PayloadDigest  string
}

type ServerWriteResult struct {
	RowID    int64
	Replayed bool
}

type GuardedStatusWrite struct {
	Status               string
	Reason               string
	CommitSHA            string
	ExpectedStatus       string
	RequireBlockedReason bool
}

type Activity struct {
	Kind      string
	TaskID    string
	Summary   string
	Actor     string
	CreatedAt string
}

type ActivityOptions struct {
	Limit       int
	Kind        string
	TaskID      string
	Profile     string
	CreatedFrom string
	CreatedTo   string
}

type TaskFilterOptions struct {
	Tags []string
}

type EventCursor struct {
	At          string
	SourceOrder int
	ID          int64
}

type EventSource struct {
	Cursor        EventCursor
	Source        string
	TaskID        string
	Role          string
	Owner         string
	FromStatus    string
	ToStatus      string
	Actor         string
	Reason        string
	FromRole      string
	ToRole        string
	EvidenceType  string
	EvidenceCount int
	Reviewer      string
	Verdict       string
	SessionID     string
	Provider      string
	EndReason     string
}

type Transition struct {
	FromStatus string
	ToStatus   string
	Actor      string
	Reason     string
	At         string
}

type Health struct {
	InProgress              int
	StaleInProgress         int
	BlockedOver24h          int
	UnacknowledgedHandoff   int
	UnacknowledgedOver1Hour int
	UnroutedReviews         int
}

type Snapshot struct {
	ProjectID            string                 `json:"project_id"`
	ExportedAt           string                 `json:"exported_at"`
	Tasks                []SnapshotTask         `json:"tasks"`
	TrackMemories        []TrackMemory          `json:"track_memories,omitempty"`
	TrackMemoryLifecycle []TrackMemoryLifecycle `json:"track_memory_lifecycle,omitempty"`
}

type SnapshotTask struct {
	Task        Task         `json:"task"`
	Transitions []Transition `json:"transitions"`
	Evidence    []Evidence   `json:"evidence"`
	Handoffs    []Handoff    `json:"handoffs"`
	Reviews     []Review     `json:"reviews"`
}

type PruneResult struct {
	StateRows      int64 `json:"state_rows"`
	HistoryRows    int64 `json:"history_rows"`
	EvidenceRows   int64 `json:"evidence_rows"`
	HandoffRows    int64 `json:"handoff_rows"`
	ReviewRows     int64 `json:"review_rows"`
	CheckpointRows int64 `json:"checkpoint_rows"`
	WatcherRows    int64 `json:"watcher_rows"`
}

type PendingMigration struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
}

const sqliteBusyTimeoutMillis = 5000

func Open(ctx context.Context, path, projectID string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", sqliteBusyTimeoutMillis)); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db, projectID: projectID, taskIDPattern: defaultTaskIDPattern}
	if err := s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) SetTaskIDPattern(pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	s.taskIDPattern = compiled
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}
		var applied int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, version).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		sqlText, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlText)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func PendingMigrations(ctx context.Context, path string) ([]PendingMigration, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return embeddedMigrations()
	} else if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", sqliteBusyTimeoutMillis)); err != nil {
		return nil, err
	}
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var tableName string
	err = db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&tableName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	applied := map[int]bool{}
	if tableName != "" {
		rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var version int
			if err := rows.Scan(&version); err != nil {
				return nil, err
			}
			applied[version] = true
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	var pending []PendingMigration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}
		if !applied[version] {
			pending = append(pending, PendingMigration{Version: version, Name: entry.Name()})
		}
	}
	return pending, nil
}

// SchemaVersions reads the applied and embedded migration versions without
// applying migrations or creating a missing database.
func SchemaVersions(ctx context.Context, path string) (applied int, available int, err error) {
	migrations, err := embeddedMigrations()
	if err != nil {
		return 0, 0, err
	}
	for _, migration := range migrations {
		if migration.Version > available {
			available = migration.Version
		}
	}
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		return 0, available, nil
	} else if statErr != nil {
		return 0, available, statErr
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return 0, available, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA query_only=ON"); err != nil {
		return 0, available, err
	}
	var tableName string
	err = db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, available, nil
	}
	if err != nil {
		return 0, available, err
	}
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&applied); err != nil {
		return 0, available, err
	}
	return applied, available, nil
}

func embeddedMigrations() ([]PendingMigration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var migrations []PendingMigration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, PendingMigration{Version: version, Name: entry.Name()})
	}
	return migrations, nil
}

func migrationVersion(name string) (int, error) {
	prefix := strings.SplitN(name, "_", 2)[0]
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("invalid migration filename %q", name)
	}
	return version, nil
}

func (s *Store) Backup(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
}

func (s *Store) PruneStale(ctx context.Context) (PruneResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PruneResult{}, err
	}
	defer tx.Rollback()
	var result PruneResult
	deleted := func(query string) (int64, error) {
		res, err := tx.ExecContext(ctx, query, s.projectID)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	orphan := `o.project_id=? AND NOT EXISTS (SELECT 1 FROM task_definitions d WHERE d.project_id = o.project_id AND d.id = o.task_id)`
	if result.WatcherRows, err = deleted(`DELETE FROM task_watchers AS o WHERE ` + orphan); err != nil {
		return PruneResult{}, err
	}
	if result.CheckpointRows, err = deleted(`DELETE FROM task_checkpoints AS o WHERE ` + orphan); err != nil {
		return PruneResult{}, err
	}
	if result.EvidenceRows, err = deleted(`DELETE FROM task_evidence AS o WHERE ` + orphan); err != nil {
		return PruneResult{}, err
	}
	if result.HandoffRows, err = deleted(`DELETE FROM task_handoffs AS o WHERE ` + orphan); err != nil {
		return PruneResult{}, err
	}
	if result.ReviewRows, err = deleted(`DELETE FROM task_reviews AS o WHERE ` + orphan); err != nil {
		return PruneResult{}, err
	}
	if result.HistoryRows, err = deleted(`DELETE FROM task_state_history AS o WHERE ` + orphan); err != nil {
		return PruneResult{}, err
	}
	if result.StateRows, err = deleted(`DELETE FROM task_state AS o WHERE ` + orphan); err != nil {
		return PruneResult{}, err
	}
	return result, tx.Commit()
}

func (s *Store) Snapshot(ctx context.Context) (Snapshot, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		ProjectID:  s.projectID,
		ExportedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Tasks:      make([]SnapshotTask, 0, len(tasks)),
	}
	for _, task := range tasks {
		_, transitions, evidence, handoffs, reviews, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Tasks = append(snapshot.Tasks, SnapshotTask{
			Task:        task,
			Transitions: transitions,
			Evidence:    evidence,
			Handoffs:    handoffs,
			Reviews:     reviews,
		})
	}
	snapshot.TrackMemories, err = s.TrackMemories(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.TrackMemoryLifecycle, err = s.TrackMemoryLifecycle(ctx, "")
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) ImportTasks(ctx context.Context, tasks []TaskDefinition) error {
	if err := s.validateTaskDefinitions(tasks, false); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	actor := Actor()
	for _, task := range tasks {
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
		sourcePaths, targetPaths, reviewDomains, tags, err := taskMetadataJSON(task)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO task_definitions
  (project_id, id, parent_id, kind, title, role, notes, acceptance_checks, dependencies, priority, sequence, profile, owning_domain, owning_layer, source_paths, target_paths, review_domains, tags, risk_level, migration_type, created_at, created_by, updated_at)
VALUES (?, ?, nullif(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), ?, ?, ?, ?, nullif(?, ''), nullif(?, ''), ?, ?, ?)
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
  profile=excluded.profile,
  owning_domain=excluded.owning_domain,
  owning_layer=excluded.owning_layer,
  source_paths=excluded.source_paths,
  target_paths=excluded.target_paths,
  review_domains=excluded.review_domains,
  tags=excluded.tags,
  risk_level=excluded.risk_level,
  migration_type=excluded.migration_type,
  updated_at=excluded.updated_at`,
			s.projectID, task.ID, task.ParentID, task.Kind, task.Title, task.Role, task.Notes, string(acceptance), string(deps), task.Priority, task.Sequence, task.Profile, task.OwningDomain, task.OwningLayer, sourcePaths, targetPaths, reviewDomains, tags, task.RiskLevel, task.MigrationType, now, actor, now)
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

func (s *Store) ImportTaskStatesOnce(ctx context.Context, states []ImportedTaskState) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	actor := Actor()
	updated := 0
	for _, state := range states {
		if state.TaskID == "" {
			continue
		}
		status := strings.TrimSpace(state.Status)
		if status == "" {
			status = "todo"
		}
		var currentStatus, currentOwner, currentBranch, currentCommit string
		if err := tx.QueryRowContext(ctx, `
SELECT status, coalesce(owner, ''), coalesce(branch, ''), coalesce(commit_sha, '')
FROM task_state
WHERE project_id=? AND task_id=?`, s.projectID, state.TaskID).Scan(&currentStatus, &currentOwner, &currentBranch, &currentCommit); err != nil {
			return updated, err
		}
		var historyCount int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM task_state_history
WHERE project_id=? AND task_id=?`, s.projectID, state.TaskID).Scan(&historyCount); err != nil {
			return updated, err
		}
		if historyCount > 1 {
			continue
		}
		owner := state.Owner
		if owner == "" {
			owner = currentOwner
		}
		branch := state.Branch
		if branch == "" {
			branch = currentBranch
		}
		commit := state.CommitSHA
		if commit == "" {
			commit = currentCommit
		}
		completedAt := state.CompletedAt
		if completedAt == "" && status == "done" {
			completedAt = now
		}
		_, err := tx.ExecContext(ctx, `
UPDATE task_state
SET status=?, owner=?, branch=nullif(?, ''), completed_at=nullif(?, ''), commit_sha=nullif(?, ''), updated_at=?
WHERE project_id=? AND task_id=?`,
			status, owner, branch, completedAt, commit, now, s.projectID, state.TaskID)
		if err != nil {
			return updated, err
		}
		if err := insertHistory(ctx, tx, s.projectID, state.TaskID, currentStatus, status, currentOwner, owner, branch, actor, "import state-once"); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, tx.Commit()
}

func (s *Store) AddTask(ctx context.Context, task TaskDefinition) error {
	if task.Kind == "" {
		task.Kind = "task"
	}
	if err := s.validateTaskDefinitions([]TaskDefinition{task}, true); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if task.ParentID != "" {
		if err := ensureTaskExists(ctx, tx, s.projectID, task.ParentID); err != nil {
			return fmt.Errorf("parent %s: %w", task.ParentID, err)
		}
	}
	for _, dep := range task.Dependencies {
		if err := ensureTaskExists(ctx, tx, s.projectID, dep); err != nil {
			return fmt.Errorf("dependency %s: %w", dep, err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	actor := Actor()
	acceptance, err := json.Marshal(task.AcceptanceChecks)
	if err != nil {
		return err
	}
	deps, err := json.Marshal(task.Dependencies)
	if err != nil {
		return err
	}
	sourcePaths, targetPaths, reviewDomains, tags, err := taskMetadataJSON(task)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO task_definitions
  (project_id, id, parent_id, kind, title, role, notes, acceptance_checks, dependencies, priority, sequence, profile, owning_domain, owning_layer, source_paths, target_paths, review_domains, tags, risk_level, migration_type, created_at, created_by, updated_at)
VALUES (?, ?, nullif(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), ?, ?, ?, ?, nullif(?, ''), nullif(?, ''), ?, ?, ?)`,
		s.projectID, task.ID, task.ParentID, task.Kind, task.Title, task.Role, task.Notes, string(acceptance), string(deps), task.Priority, task.Sequence, task.Profile, task.OwningDomain, task.OwningLayer, sourcePaths, targetPaths, reviewDomains, tags, task.RiskLevel, task.MigrationType, now, actor, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO task_state
  (project_id, task_id, status, owner, review_required, review_status, updated_at)
VALUES (?, ?, 'todo', ?, 0, 'not_required', ?)`, s.projectID, task.ID, task.Role, now)
	if err != nil {
		return err
	}
	if err := insertHistory(ctx, tx, s.projectID, task.ID, "", "todo", "", task.Role, "", actor, "add"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateTask(ctx context.Context, task TaskDefinition) error {
	if task.Kind == "" {
		task.Kind = "task"
	}
	if task.ParentID == task.ID {
		return fmt.Errorf("task %s cannot be its own parent", task.ID)
	}
	if err := s.validateTaskDefinitions([]TaskDefinition{task}, true); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureTaskExists(ctx, tx, s.projectID, task.ID); err != nil {
		return err
	}
	if task.ParentID != "" {
		if err := ensureTaskExists(ctx, tx, s.projectID, task.ParentID); err != nil {
			return fmt.Errorf("parent %s: %w", task.ParentID, err)
		}
		if err := s.ensureParentDoesNotCycle(ctx, tx, task.ID, task.ParentID); err != nil {
			return err
		}
	}
	for _, dep := range task.Dependencies {
		if err := ensureTaskExists(ctx, tx, s.projectID, dep); err != nil {
			return fmt.Errorf("dependency %s: %w", dep, err)
		}
	}
	acceptance, err := json.Marshal(task.AcceptanceChecks)
	if err != nil {
		return err
	}
	deps, err := json.Marshal(task.Dependencies)
	if err != nil {
		return err
	}
	sourcePaths, targetPaths, reviewDomains, tags, err := taskMetadataJSON(task)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
UPDATE task_definitions
SET parent_id=nullif(?, ''),
    kind=?,
    title=?,
    role=?,
    notes=?,
    acceptance_checks=?,
    dependencies=?,
    priority=?,
    sequence=?,
    profile=nullif(?, ''),
    owning_domain=nullif(?, ''),
    owning_layer=nullif(?, ''),
    source_paths=?,
    target_paths=?,
    review_domains=?,
    tags=?,
    risk_level=nullif(?, ''),
    migration_type=nullif(?, ''),
    updated_at=?
WHERE project_id=? AND id=?`,
		task.ParentID, task.Kind, task.Title, task.Role, task.Notes, string(acceptance), string(deps), task.Priority, task.Sequence, task.Profile, task.OwningDomain, task.OwningLayer, sourcePaths, targetPaths, reviewDomains, tags, task.RiskLevel, task.MigrationType, time.Now().UTC().Format(time.RFC3339Nano), s.projectID, task.ID)
	if err := checkWriteResult(res, err); err != nil {
		return err
	}
	res, err = tx.ExecContext(ctx, `UPDATE task_state SET owner=?, updated_at=? WHERE project_id=? AND task_id=?`, task.Role, time.Now().UTC().Format(time.RFC3339Nano), s.projectID, task.ID)
	if err := checkWriteResult(res, err); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ensureParentDoesNotCycle(ctx context.Context, tx *sql.Tx, taskID, parentID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, COALESCE(parent_id, '') FROM task_definitions WHERE project_id=?`, s.projectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	parents := map[string]string{}
	for rows.Next() {
		var id, parent string
		if err := rows.Scan(&id, &parent); err != nil {
			return err
		}
		parents[id] = parent
	}
	if err := rows.Err(); err != nil {
		return err
	}
	parents[taskID] = parentID
	for cursor := parentID; cursor != ""; cursor = parents[cursor] {
		if cursor == taskID {
			return fmt.Errorf("task %s parent %s would create a cycle", taskID, parentID)
		}
	}
	return nil
}

func (s *Store) validateTaskDefinitions(tasks []TaskDefinition, allowExternalRefs bool) error {
	pattern := s.taskIDPattern
	if pattern == nil {
		pattern = defaultTaskIDPattern
	}
	seen := map[string]bool{}
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.Title) == "" {
			return errors.New("task id and title are required")
		}
		if !pattern.MatchString(task.ID) {
			return fmt.Errorf("%w: %s", ErrInvalidTaskID, task.ID)
		}
		if strings.TrimSpace(task.Role) == "" {
			return fmt.Errorf("task %s role is required", task.ID)
		}
		if seen[task.ID] {
			return fmt.Errorf("duplicate task id %s", task.ID)
		}
		seen[task.ID] = true
	}
	for _, task := range tasks {
		if !allowExternalRefs && task.ParentID != "" && !seen[task.ParentID] {
			return fmt.Errorf("task %s references unknown parent %s", task.ID, task.ParentID)
		}
		for _, dep := range task.Dependencies {
			if !allowExternalRefs && !seen[dep] {
				return fmt.Errorf("task %s references unknown dependency %s", task.ID, dep)
			}
			if dep == task.ID {
				return fmt.Errorf("task %s cannot depend on itself", task.ID)
			}
		}
	}
	return nil
}

func taskMetadataJSON(task TaskDefinition) (string, string, string, string, error) {
	sourcePaths, err := json.Marshal(task.SourcePaths)
	if err != nil {
		return "", "", "", "", err
	}
	targetPaths, err := json.Marshal(task.TargetPaths)
	if err != nil {
		return "", "", "", "", err
	}
	reviewDomains, err := json.Marshal(task.ReviewDomains)
	if err != nil {
		return "", "", "", "", err
	}
	tags, err := json.Marshal(task.Tags)
	if err != nil {
		return "", "", "", "", err
	}
	return string(sourcePaths), string(targetPaths), string(reviewDomains), string(tags), nil
}

func ensureTaskExists(ctx context.Context, tx *sql.Tx, projectID, taskID string) error {
	var exists int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM task_definitions WHERE project_id=? AND id=?`, projectID, taskID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) Ready(ctx context.Context, role string, terminal []string) ([]Task, error) {
	args := []any{s.projectID}
	roleSQL := ""
	if role != "" {
		roleSQL = " AND d.role = ?"
		args = append(args, role)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT d.id, d.parent_id, d.kind, d.title, d.role, d.notes, d.acceptance_checks, d.dependencies,
       d.priority, d.sequence, d.profile, d.owning_domain, d.owning_layer, d.source_paths, d.target_paths, d.review_domains, d.tags, d.risk_level, d.migration_type,
       st.status, st.owner, st.claimant, st.branch, st.completed_at, st.commit_sha, st.review_status, st.updated_at
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
	if len(terminal) == 0 {
		terminal = []string{"done"}
	}
	terminalSet := map[string]bool{}
	for _, status := range terminal {
		terminalSet[status] = true
	}
	ready := candidates[:0]
	for _, task := range candidates {
		ok := true
		for _, dep := range task.Definition.Dependencies {
			if !terminalSet[statuses[dep]] {
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

// StartWork atomically makes a task active, attaches a provider session, and
// records the active checkpoint using the existing Fairway records.
func (s *Store) StartWork(ctx context.Context, taskID, owner, branch string, session Session, summary string, terminal []string) (WorkStartResult, error) {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(owner) == "" || strings.TrimSpace(session.ID) == "" {
		return WorkStartResult{}, errors.New("task id, owner, and session id are required")
	}
	if strings.TrimSpace(summary) == "" {
		return WorkStartResult{}, errors.New("work start checkpoint summary is required")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return WorkStartResult{}, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return WorkStartResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()
	var status, priorOwner, claimant, dependenciesJSON string
	err = conn.QueryRowContext(ctx, `SELECT st.status, COALESCE(st.owner, ''), COALESCE(st.claimant, ''), COALESCE(d.dependencies, '[]') FROM task_state st JOIN task_definitions d ON d.project_id=st.project_id AND d.id=st.task_id WHERE st.project_id=? AND st.task_id=?`, s.projectID, taskID).Scan(&status, &priorOwner, &claimant, &dependenciesJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkStartResult{}, ErrNotFound
	}
	if err != nil {
		return WorkStartResult{}, err
	}
	alreadyActive := status == "in_progress"
	if status != "todo" && !alreadyActive {
		return WorkStartResult{}, fmt.Errorf("%w: work start requires todo or in_progress task, got %s", ErrInvalidTransition, status)
	}
	if alreadyActive && priorOwner != "" && priorOwner != owner {
		return WorkStartResult{}, fmt.Errorf("%w: task is already active for owner %s", ErrAlreadyClaimed, priorOwner)
	}
	if !alreadyActive {
		var dependencies []string
		if err := json.Unmarshal([]byte(dependenciesJSON), &dependencies); err != nil {
			return WorkStartResult{}, fmt.Errorf("decode task dependencies: %w", err)
		}
		terminalSet := map[string]bool{}
		for _, value := range terminal {
			terminalSet[value] = true
		}
		if len(terminalSet) == 0 {
			terminalSet["done"] = true
		}
		for _, dependency := range dependencies {
			var dependencyStatus string
			err := conn.QueryRowContext(ctx, `SELECT status FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, dependency).Scan(&dependencyStatus)
			if errors.Is(err, sql.ErrNoRows) {
				return WorkStartResult{}, fmt.Errorf("%w: dependency %s is missing", ErrInvalidTransition, dependency)
			}
			if err != nil {
				return WorkStartResult{}, err
			}
			if !terminalSet[dependencyStatus] {
				return WorkStartResult{}, fmt.Errorf("%w: dependency %s is %s", ErrInvalidTransition, dependency, dependencyStatus)
			}
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if !alreadyActive {
		actor := Actor()
		res, err := conn.ExecContext(ctx, `UPDATE task_state SET status='in_progress', owner=?, claimant=?, branch=?, claimed_at=?, updated_at=? WHERE project_id=? AND task_id=? AND status='todo'`, owner, actor, branch, now, now, s.projectID, taskID)
		if err := checkWriteResult(res, err); err != nil {
			return WorkStartResult{}, err
		}
		if err := insertHistoryExec(ctx, conn, s.projectID, taskID, status, "in_progress", priorOwner, owner, branch, actor, "work start"); err != nil {
			return WorkStartResult{}, err
		}
	}
	if session.Status == "" {
		session.Status = "running"
	}
	if session.StartedAt == "" {
		session.StartedAt = now
	}
	session.LastHeartbeatAt = now
	_, err = conn.ExecContext(ctx, `
INSERT INTO agent_sessions
  (project_id, id, role, lane, worktree_path, branch, session_backend, provider, session_name, task_id, pid, tmux_pane, transcript_path, monitor_kind, automation_id, external_run_id, poll_command, manual_until, status, started_at, last_heartbeat_at, ended_at, exit_code, end_reason)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL)
ON CONFLICT(project_id, id) DO UPDATE SET role=excluded.role, lane=excluded.lane, worktree_path=excluded.worktree_path, branch=excluded.branch, session_backend=excluded.session_backend, provider=excluded.provider, session_name=excluded.session_name, task_id=excluded.task_id, pid=excluded.pid, tmux_pane=excluded.tmux_pane, transcript_path=excluded.transcript_path, external_run_id=excluded.external_run_id, status='running', last_heartbeat_at=excluded.last_heartbeat_at, ended_at=NULL, exit_code=NULL, end_reason=NULL`,
		s.projectID, session.ID, owner, session.Lane, session.WorktreePath, session.Branch, session.SessionBackend, session.Provider, session.SessionName, taskID, session.PID, session.TmuxPane, session.TranscriptPath, session.MonitorKind, session.AutomationID, session.ExternalRunID, session.PollCommand, session.ManualUntil, session.Status, session.StartedAt, session.LastHeartbeatAt)
	if err != nil {
		return WorkStartResult{}, err
	}
	var cpID int64
	if alreadyActive {
		err = conn.QueryRowContext(ctx, `SELECT id FROM task_checkpoints WHERE project_id=? AND task_id=? AND state='active' AND owner=? AND lower(summary) LIKE ? ORDER BY id DESC LIMIT 1`, s.projectID, taskID, owner, "%"+strings.ToLower(session.ID)+"%").Scan(&cpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return WorkStartResult{}, err
		}
	}
	if cpID == 0 {
		res, err := conn.ExecContext(ctx, `INSERT INTO task_checkpoints (project_id, task_id, state, owner, summary, created_at) VALUES (?, ?, 'active', ?, ?, ?)`, s.projectID, taskID, owner, summary, now)
		if err != nil {
			return WorkStartResult{}, err
		}
		cpID, err = res.LastInsertId()
		if err != nil {
			return WorkStartResult{}, err
		}
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return WorkStartResult{}, err
	}
	committed = true
	return WorkStartResult{TaskID: taskID, Status: "in_progress", Owner: owner, SessionID: session.ID, CheckpointID: cpID, AlreadyActive: alreadyActive}, nil
}

func (s *Store) CurrentStatus(ctx context.Context, taskID string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return status, err
}

// CloseWork commits the task and provider-session closeout as one lifecycle change.
// All policy, review, evidence, git, and reconciliation gates are evaluated by
// the caller before this method is invoked.
func (s *Store) CloseWork(ctx context.Context, taskID, sessionID, commitSHA, reason string) (WorkCloseResult, error) {
	if strings.TrimSpace(sessionID) == "" {
		return WorkCloseResult{}, errors.New("work close session id is required")
	}
	if strings.TrimSpace(commitSHA) == "" {
		return WorkCloseResult{}, errors.New("work close commit SHA is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkCloseResult{}, err
	}
	defer tx.Rollback()
	var status, owner, branch string
	if err := tx.QueryRowContext(ctx, `SELECT status, COALESCE(owner,''), COALESCE(branch,'') FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&status, &owner, &branch); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkCloseResult{}, ErrNotFound
		}
		return WorkCloseResult{}, err
	}
	if status != "in_progress" {
		return WorkCloseResult{}, fmt.Errorf("work close requires in_progress task, got %s", status)
	}
	var sessionTask, sessionStatus string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(task_id,''), status FROM agent_sessions WHERE project_id=? AND id=?`, s.projectID, sessionID).Scan(&sessionTask, &sessionStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkCloseResult{}, fmt.Errorf("work close session %q not found", sessionID)
		}
		return WorkCloseResult{}, err
	}
	if sessionTask != taskID {
		return WorkCloseResult{}, fmt.Errorf("work close session %q is attached to task %q, not %q", sessionID, sessionTask, taskID)
	}
	if sessionStatus != "starting" && sessionStatus != "running" {
		return WorkCloseResult{}, fmt.Errorf("work close session %q is %s", sessionID, sessionStatus)
	}
	var activeSessions int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_sessions WHERE project_id=? AND task_id=? AND status IN ('starting','running')`, s.projectID, taskID).Scan(&activeSessions); err != nil {
		return WorkCloseResult{}, err
	}
	if activeSessions != 1 {
		return WorkCloseResult{}, fmt.Errorf("work close requires exactly one active session for task %q, got %d", taskID, activeSessions)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `
UPDATE task_state
SET status='done', claimant=NULL, completed_at=?, commit_sha=?, updated_at=?
WHERE project_id=? AND task_id=? AND status='in_progress'
  AND (SELECT COUNT(*) FROM agent_sessions WHERE project_id=? AND task_id=? AND status IN ('starting','running'))=1`,
		now, commitSHA, now, s.projectID, taskID, s.projectID, taskID)
	if err := checkWriteResult(res, err); err != nil {
		return WorkCloseResult{}, err
	}
	if err := insertHistory(ctx, tx, s.projectID, taskID, status, "done", owner, owner, branch, Actor(), reason); err != nil {
		return WorkCloseResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE agent_sessions SET status='ended', ended_at=?, exit_code=0, end_reason=? WHERE project_id=? AND id=?`, now, reason, s.projectID, sessionID); err != nil {
		return WorkCloseResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkCloseResult{}, err
	}
	return WorkCloseResult{TaskID: taskID, Status: "done", SessionID: sessionID, CommitSHA: commitSHA}, nil
}

func (s *Store) HasEvidence(ctx context.Context, taskID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_evidence WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&count)
	return count > 0, err
}

func (s *Store) EvidenceTypes(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT artifact_type FROM task_evidence WHERE project_id=? AND COALESCE(artifact_type, '') <> '' ORDER BY artifact_type`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var artifactType string
		if err := rows.Scan(&artifactType); err != nil {
			return nil, err
		}
		out = append(out, artifactType)
	}
	return out, rows.Err()
}

func (s *Store) EvidenceByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]Evidence, error) {
	ids := uniqueNonEmptyStrings(taskIDs)
	out := make(map[string][]Evidence, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, s.projectID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, COALESCE(command_text, ''), COALESCE(result, ''), COALESCE(artifact_path, ''), COALESCE(artifact_type, ''), duration_seconds, COALESCE(notes, ''), created_at FROM task_evidence WHERE project_id=? AND task_id IN (`+sqlPlaceholders(len(ids))+`) ORDER BY task_id, created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID string
		var ev Evidence
		var dur sql.NullInt64
		if err := rows.Scan(&taskID, &ev.CommandText, &ev.Result, &ev.ArtifactPath, &ev.ArtifactType, &dur, &ev.Notes, &ev.CreatedAt); err != nil {
			return nil, err
		}
		if dur.Valid {
			v := int(dur.Int64)
			ev.DurationSeconds = &v
		}
		out[taskID] = append(out[taskID], ev)
	}
	return out, rows.Err()
}

func (s *Store) HasApprovedReview(ctx context.Context, taskID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_reviews WHERE project_id=? AND task_id=? AND verdict='approve'`, s.projectID, taskID).Scan(&count)
	return count > 0, err
}

func (s *Store) ReviewsByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]Review, error) {
	ids := uniqueNonEmptyStrings(taskIDs)
	out := make(map[string][]Review, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, s.projectID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, reviewer, COALESCE(review_domain, ''), verdict, notes, reviewed_commit_sha, created_at FROM task_reviews WHERE project_id=? AND task_id IN (`+sqlPlaceholders(len(ids))+`) ORDER BY task_id, created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID string
		var r Review
		if err := rows.Scan(&taskID, &r.Reviewer, &r.Domain, &r.Verdict, &r.Reason, &r.Commit, &r.CreatedAt); err != nil {
			return nil, err
		}
		out[taskID] = append(out[taskID], r)
	}
	return out, rows.Err()
}

func (s *Store) HasHandoff(ctx context.Context, taskID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_handoffs WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&count)
	return count > 0, err
}

func (s *Store) RouteReview(ctx context.Context, taskID, reviewer, reason string) error {
	if reviewer == "" {
		return errors.New("reviewer is required")
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE task_state
SET review_required=1, review_status='pending', reviewer=?, review_note=?, updated_at=?
WHERE project_id=? AND task_id=?`,
		reviewer, reason, time.Now().UTC().Format(time.RFC3339Nano), s.projectID, taskID)
	return checkWriteResult(res, err)
}

func (s *Store) UpsertSession(ctx context.Context, session Session) error {
	if strings.TrimSpace(session.ID) == "" {
		return errors.New("session id is required")
	}
	if strings.TrimSpace(session.Role) == "" {
		return errors.New("session role is required")
	}
	if session.Status == "" {
		session.Status = "running"
	}
	switch session.Status {
	case "starting", "running", "ended", "failed", "stale":
	default:
		return fmt.Errorf("invalid session status %q", session.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if session.StartedAt == "" {
		session.StartedAt = now
	}
	if session.LastHeartbeatAt == "" && session.Status != "ended" && session.Status != "failed" {
		session.LastHeartbeatAt = now
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO agent_sessions
  (project_id, id, role, lane, worktree_path, branch, session_backend, provider, session_name, task_id, pid, tmux_pane, transcript_path, monitor_kind, automation_id, external_run_id, poll_command, manual_until, status, started_at, last_heartbeat_at, ended_at, exit_code, end_reason)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, nullif(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, nullif(?, ''), nullif(?, ''), ?, nullif(?, ''))
ON CONFLICT(project_id, id) DO UPDATE SET
  role=excluded.role,
  lane=excluded.lane,
  worktree_path=excluded.worktree_path,
  branch=excluded.branch,
  session_backend=excluded.session_backend,
  provider=excluded.provider,
  session_name=excluded.session_name,
  task_id=excluded.task_id,
  pid=excluded.pid,
  tmux_pane=excluded.tmux_pane,
  transcript_path=excluded.transcript_path,
  monitor_kind=excluded.monitor_kind,
  automation_id=excluded.automation_id,
  external_run_id=excluded.external_run_id,
  poll_command=excluded.poll_command,
  manual_until=excluded.manual_until,
  status=excluded.status,
  last_heartbeat_at=excluded.last_heartbeat_at,
  ended_at=excluded.ended_at,
  exit_code=excluded.exit_code,
  end_reason=excluded.end_reason`,
		s.projectID, session.ID, session.Role, session.Lane, session.WorktreePath, session.Branch, session.SessionBackend, session.Provider, session.SessionName, session.TaskID, session.PID, session.TmuxPane, session.TranscriptPath, session.MonitorKind, session.AutomationID, session.ExternalRunID, session.PollCommand, session.ManualUntil, session.Status, session.StartedAt, session.LastHeartbeatAt, session.EndedAt, session.ExitCode, session.EndReason)
	return err
}

func (s *Store) EndSession(ctx context.Context, id, status, reason string, exitCode *int) error {
	if status == "" {
		status = "ended"
	}
	switch status {
	case "ended", "failed", "stale":
	default:
		return fmt.Errorf("invalid terminal session status %q", status)
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE agent_sessions
SET status=?, ended_at=?, exit_code=?, end_reason=?
WHERE project_id=? AND id=?`,
		status, time.Now().UTC().Format(time.RFC3339Nano), exitCode, reason, s.projectID, id)
	return checkWriteResult(res, err)
}

func (s *Store) Sessions(ctx context.Context, includeEnded bool) ([]Session, error) {
	query := `
SELECT id, role, COALESCE(lane, ''), COALESCE(worktree_path, ''), COALESCE(branch, ''),
       COALESCE(session_backend, ''), COALESCE(provider, ''), COALESCE(session_name, ''),
       COALESCE(task_id, ''), pid, COALESCE(tmux_pane, ''), COALESCE(transcript_path, ''),
       COALESCE(monitor_kind, ''), COALESCE(automation_id, ''), COALESCE(external_run_id, ''), COALESCE(poll_command, ''), COALESCE(manual_until, ''),
       status, started_at, COALESCE(last_heartbeat_at, ''), COALESCE(ended_at, ''), exit_code, COALESCE(end_reason, '')
FROM agent_sessions
WHERE project_id=?`
	if !includeEnded {
		query += ` AND ended_at IS NULL`
	}
	query += ` ORDER BY role, started_at DESC`
	rows, err := s.db.QueryContext(ctx, query, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var session Session
		var pid, exitCode sql.NullInt64
		if err := rows.Scan(&session.ID, &session.Role, &session.Lane, &session.WorktreePath, &session.Branch, &session.SessionBackend, &session.Provider, &session.SessionName, &session.TaskID, &pid, &session.TmuxPane, &session.TranscriptPath, &session.MonitorKind, &session.AutomationID, &session.ExternalRunID, &session.PollCommand, &session.ManualUntil, &session.Status, &session.StartedAt, &session.LastHeartbeatAt, &session.EndedAt, &exitCode, &session.EndReason); err != nil {
			return nil, err
		}
		if pid.Valid {
			v := int(pid.Int64)
			session.PID = &v
		}
		if exitCode.Valid {
			v := int(exitCode.Int64)
			session.ExitCode = &v
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

func (s *Store) RecordCheckpoint(ctx context.Context, cp Checkpoint) error {
	_, err := s.RecordCheckpointWithID(ctx, cp)
	return err
}

func (s *Store) RecordCheckpointWithID(ctx context.Context, cp Checkpoint) (int64, error) {
	if cp.TaskID == "" {
		return 0, errors.New("checkpoint task id is required")
	}
	if cp.Summary == "" {
		return 0, errors.New("checkpoint summary is required")
	}
	switch cp.State {
	case "planned", "active", "awaiting_input", "review", "done", "parked", "abandoned":
	default:
		return 0, fmt.Errorf("invalid checkpoint state %q", cp.State)
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO task_checkpoints (project_id, task_id, state, owner, target_close_by, summary, artifact_path, created_at)
VALUES (?, ?, ?, ?, nullif(?, ''), ?, ?, ?)`,
		s.projectID, cp.TaskID, cp.State, cp.Owner, cp.TargetCloseBy, cp.Summary, cp.ArtifactPath, time.Now().UTC().Format(time.RFC3339Nano))
	if err := checkWriteResult(res, err); err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) Checkpoints(ctx context.Context, staleBefore string, includeClosed bool) ([]Checkpoint, error) {
	query := `
SELECT id, task_id, state, COALESCE(owner, ''), COALESCE(target_close_by, ''), summary, COALESCE(artifact_path, ''), created_at
FROM task_checkpoints
WHERE project_id=?`
	args := []any{s.projectID}
	if staleBefore != "" {
		query += ` AND target_close_by IS NOT NULL AND target_close_by < ?`
		args = append(args, staleBefore)
	}
	if !includeClosed {
		query += ` AND state NOT IN ('done', 'parked', 'abandoned')`
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Checkpoint
	for rows.Next() {
		var cp Checkpoint
		if err := rows.Scan(&cp.ID, &cp.TaskID, &cp.State, &cp.Owner, &cp.TargetCloseBy, &cp.Summary, &cp.ArtifactPath, &cp.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, cp)
	}
	return out, rows.Err()
}

func (s *Store) RecordCheckpointWithIdempotency(ctx context.Context, cp Checkpoint, req ServerWriteRequest) (ServerWriteResult, error) {
	if cp.TaskID == "" {
		return ServerWriteResult{}, errors.New("checkpoint task id is required")
	}
	if cp.Summary == "" {
		return ServerWriteResult{}, errors.New("checkpoint summary is required")
	}
	switch cp.State {
	case "planned", "active", "awaiting_input", "review", "done", "parked", "abandoned":
	default:
		return ServerWriteResult{}, fmt.Errorf("invalid checkpoint state %q", cp.State)
	}
	if err := validateServerWriteRequest(req); err != nil {
		return ServerWriteResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ServerWriteResult{}, err
	}
	defer tx.Rollback()
	if result, ok, err := s.serverWriteReplay(ctx, tx, cp.TaskID, "checkpoint", req); err != nil || ok {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `
INSERT INTO task_checkpoints (project_id, task_id, state, owner, target_close_by, summary, artifact_path, created_at)
VALUES (?, ?, ?, ?, nullif(?, ''), ?, ?, ?)`,
		s.projectID, cp.TaskID, cp.State, cp.Owner, cp.TargetCloseBy, cp.Summary, cp.ArtifactPath, now)
	if err := checkWriteResult(res, err); err != nil {
		return ServerWriteResult{}, err
	}
	rowID, err := res.LastInsertId()
	if err != nil {
		return ServerWriteResult{}, err
	}
	if err := s.insertServerWriteIdempotency(ctx, tx, cp.TaskID, "checkpoint", rowID, now, req); err != nil {
		return ServerWriteResult{}, err
	}
	if err := insertAudit(ctx, tx, s.projectID, AuditEvent{
		Actor:  req.Actor,
		Action: "server.api.checkpoint",
		TaskID: cp.TaskID,
		Detail: serverWriteAuditDetail(req, "checkpoint", rowID, false),
	}); err != nil {
		return ServerWriteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ServerWriteResult{}, err
	}
	return ServerWriteResult{RowID: rowID}, nil
}

func (s *Store) TrackMemory(ctx context.Context, trackID string) (TrackMemory, error) {
	var mem TrackMemory
	row := s.db.QueryRowContext(ctx, `
SELECT track_id, COALESCE(title, ''), COALESCE(purpose, ''), COALESCE(operating_mode, ''),
       COALESCE(active_scope, ''), COALESCE(current_objective, ''), COALESCE(decisions, '[]'),
       COALESCE(blockers, '[]'), COALESCE(open_questions, '[]'), COALESCE(next_actions, '[]'),
       COALESCE(source_checkpoint_ids, '[]'), COALESCE(source_evidence_ids, '[]'),
       COALESCE(source_review_ids, '[]'), COALESCE(owner, ''), COALESCE(review_by, ''),
       COALESCE(disposition, 'active'), COALESCE(promotion_target, ''), COALESCE(canonical_commit, ''),
       COALESCE(superseded_by_track_id, ''), updated_at
FROM track_memory
WHERE project_id=? AND track_id=?`, s.projectID, trackID)
	if err := scanTrackMemory(row, &mem); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TrackMemory{}, ErrNotFound
		}
		return TrackMemory{}, err
	}
	return mem, nil
}

func (s *Store) TrackMemories(ctx context.Context) ([]TrackMemory, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT track_id, COALESCE(title, ''), COALESCE(purpose, ''), COALESCE(operating_mode, ''),
       COALESCE(active_scope, ''), COALESCE(current_objective, ''), COALESCE(decisions, '[]'),
       COALESCE(blockers, '[]'), COALESCE(open_questions, '[]'), COALESCE(next_actions, '[]'),
       COALESCE(source_checkpoint_ids, '[]'), COALESCE(source_evidence_ids, '[]'),
       COALESCE(source_review_ids, '[]'), COALESCE(owner, ''), COALESCE(review_by, ''),
       COALESCE(disposition, 'active'), COALESCE(promotion_target, ''), COALESCE(canonical_commit, ''),
       COALESCE(superseded_by_track_id, ''), updated_at
FROM track_memory
WHERE project_id=?
ORDER BY updated_at DESC`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrackMemory
	for rows.Next() {
		var mem TrackMemory
		if err := scanTrackMemory(rows, &mem); err != nil {
			return nil, err
		}
		out = append(out, mem)
	}
	return out, rows.Err()
}

func (s *Store) UpsertTrackMemory(ctx context.Context, mem TrackMemory, appendFields bool) (TrackMemory, error) {
	mem.TrackID = strings.TrimSpace(mem.TrackID)
	if mem.TrackID == "" {
		return TrackMemory{}, errors.New("track id is required")
	}
	existing, existingErr := s.TrackMemory(ctx, mem.TrackID)
	if existingErr != nil && !errors.Is(existingErr, ErrNotFound) {
		return TrackMemory{}, existingErr
	}
	if existingErr == nil {
		if appendFields {
			mem = mergeTrackMemory(existing, mem)
		} else {
			if mem.Owner == "" {
				mem.Owner = existing.Owner
			}
			if mem.ReviewBy == "" {
				mem.ReviewBy = existing.ReviewBy
			}
			if len(mem.SourceCheckpointIDs)+len(mem.SourceEvidenceIDs)+len(mem.SourceReviewIDs) == 0 {
				mem.SourceCheckpointIDs = existing.SourceCheckpointIDs
				mem.SourceEvidenceIDs = existing.SourceEvidenceIDs
				mem.SourceReviewIDs = existing.SourceReviewIDs
			}
			mem.Disposition = existing.Disposition
			mem.PromotionTarget = existing.PromotionTarget
			mem.CanonicalCommit = existing.CanonicalCommit
			mem.SupersededByTrackID = existing.SupersededByTrackID
		}
	}
	if mem.Disposition == "" {
		mem.Disposition = "active"
	}
	if err := validateTrackMemoryLifecycleFields(mem); err != nil {
		return TrackMemory{}, err
	}
	if err := s.validateTrackMemorySources(ctx, mem); err != nil {
		return TrackMemory{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO track_memory
  (project_id, track_id, title, purpose, operating_mode, active_scope, current_objective,
   decisions, blockers, open_questions, next_actions, source_checkpoint_ids, source_evidence_ids,
   source_review_ids, owner, review_by, disposition, promotion_target, canonical_commit,
   superseded_by_track_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(project_id, track_id) DO UPDATE SET
  title=excluded.title,
  purpose=excluded.purpose,
  operating_mode=excluded.operating_mode,
  active_scope=excluded.active_scope,
  current_objective=excluded.current_objective,
  decisions=excluded.decisions,
  blockers=excluded.blockers,
  open_questions=excluded.open_questions,
  next_actions=excluded.next_actions,
  source_checkpoint_ids=excluded.source_checkpoint_ids,
  source_evidence_ids=excluded.source_evidence_ids,
  source_review_ids=excluded.source_review_ids,
  owner=excluded.owner,
  review_by=excluded.review_by,
  disposition=excluded.disposition,
  promotion_target=excluded.promotion_target,
  canonical_commit=excluded.canonical_commit,
  superseded_by_track_id=excluded.superseded_by_track_id,
  updated_at=excluded.updated_at`,
		s.projectID, mem.TrackID, mem.Title, mem.Purpose, mem.OperatingMode, mem.ActiveScope, mem.CurrentObjective,
		mustJSONStrings(mem.Decisions), mustJSONStrings(mem.Blockers), mustJSONStrings(mem.OpenQuestions),
		mustJSONStrings(mem.NextActions), mustJSONInt64s(mem.SourceCheckpointIDs), mustJSONInt64s(mem.SourceEvidenceIDs),
		mustJSONInt64s(mem.SourceReviewIDs), mem.Owner, mem.ReviewBy, mem.Disposition, mem.PromotionTarget,
		mem.CanonicalCommit, mem.SupersededByTrackID, now)
	if err != nil {
		return TrackMemory{}, err
	}
	return s.TrackMemory(ctx, mem.TrackID)
}

func validateTrackMemoryLifecycleFields(mem TrackMemory) error {
	switch mem.Disposition {
	case "active", "promote", "archived", "superseded":
	default:
		return fmt.Errorf("invalid track memory disposition %q", mem.Disposition)
	}
	if mem.Disposition == "active" {
		if strings.TrimSpace(mem.Owner) == "" {
			return errors.New("active track memory owner is required")
		}
		if strings.TrimSpace(mem.ReviewBy) == "" {
			return errors.New("active track memory review date is required")
		}
		if _, err := time.Parse("2006-01-02", mem.ReviewBy); err != nil {
			if _, rfcErr := time.Parse(time.RFC3339, mem.ReviewBy); rfcErr != nil {
				return errors.New("track memory review date must be YYYY-MM-DD or RFC3339")
			}
		}
		if len(mem.SourceCheckpointIDs)+len(mem.SourceEvidenceIDs)+len(mem.SourceReviewIDs) == 0 {
			return errors.New("active track memory requires at least one source fact")
		}
	}
	return nil
}

func (s *Store) RecordTrackMemoryDisposition(ctx context.Context, trackID, disposition, reason, promotionTarget, canonicalCommit, supersededBy string) (TrackMemory, error) {
	trackID = strings.TrimSpace(trackID)
	disposition = strings.TrimSpace(disposition)
	reason = strings.TrimSpace(reason)
	if trackID == "" || reason == "" {
		return TrackMemory{}, errors.New("track id and disposition reason are required")
	}
	if disposition != "active" && disposition != "promote" && disposition != "archived" && disposition != "superseded" {
		return TrackMemory{}, fmt.Errorf("invalid track memory disposition %q", disposition)
	}
	if disposition == "promote" && strings.TrimSpace(promotionTarget) == "" {
		return TrackMemory{}, errors.New("promote disposition requires promotion target")
	}
	if disposition == "superseded" && strings.TrimSpace(supersededBy) == "" {
		return TrackMemory{}, errors.New("superseded disposition requires replacement track id")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TrackMemory{}, err
	}
	defer tx.Rollback()
	var from, existingTarget, existingCommit, existingSupersededBy string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(disposition,'active'), COALESCE(promotion_target,''), COALESCE(canonical_commit,''), COALESCE(superseded_by_track_id,'') FROM track_memory WHERE project_id=? AND track_id=?`, s.projectID, trackID).Scan(&from, &existingTarget, &existingCommit, &existingSupersededBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TrackMemory{}, ErrNotFound
		}
		return TrackMemory{}, err
	}
	if from == disposition {
		return TrackMemory{}, fmt.Errorf("track memory %q is already %s", trackID, disposition)
	}
	if promotionTarget == "" {
		promotionTarget = existingTarget
	}
	if canonicalCommit == "" {
		canonicalCommit = existingCommit
	}
	if supersededBy == "" {
		supersededBy = existingSupersededBy
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO track_memory_lifecycle
  (project_id, track_id, from_disposition, to_disposition, reason, promotion_target, canonical_commit, superseded_by_track_id, actor, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.projectID, trackID, from, disposition, reason, promotionTarget, canonicalCommit, supersededBy, Actor(), now); err != nil {
		return TrackMemory{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE track_memory SET disposition=?, promotion_target=?, canonical_commit=?, superseded_by_track_id=?, updated_at=? WHERE project_id=? AND track_id=?`, disposition, promotionTarget, canonicalCommit, supersededBy, now, s.projectID, trackID); err != nil {
		return TrackMemory{}, err
	}
	if err := tx.Commit(); err != nil {
		return TrackMemory{}, err
	}
	return s.TrackMemory(ctx, trackID)
}

func (s *Store) TrackMemoryLifecycle(ctx context.Context, trackID string) ([]TrackMemoryLifecycle, error) {
	query := `SELECT id, track_id, from_disposition, to_disposition, reason,
       COALESCE(promotion_target,''), COALESCE(canonical_commit,''), COALESCE(superseded_by_track_id,''), actor, created_at
FROM track_memory_lifecycle WHERE project_id=?`
	args := []any{s.projectID}
	if strings.TrimSpace(trackID) != "" {
		query += " AND track_id=?"
		args = append(args, strings.TrimSpace(trackID))
	}
	query += " ORDER BY id"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrackMemoryLifecycle
	for rows.Next() {
		var row TrackMemoryLifecycle
		if err := rows.Scan(&row.ID, &row.TrackID, &row.FromDisposition, &row.ToDisposition, &row.Reason, &row.PromotionTarget, &row.CanonicalCommit, &row.SupersededByTrackID, &row.Actor, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) validateTrackMemorySources(ctx context.Context, mem TrackMemory) error {
	for _, id := range mem.SourceCheckpointIDs {
		if id <= 0 {
			return fmt.Errorf("invalid source checkpoint id %d", id)
		}
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_checkpoints WHERE project_id=? AND id=?`, s.projectID, id).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("source checkpoint id %d not found", id)
		}
	}
	for _, id := range mem.SourceEvidenceIDs {
		if id <= 0 {
			return fmt.Errorf("invalid source evidence id %d", id)
		}
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_evidence WHERE project_id=? AND id=?`, s.projectID, id).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("source evidence id %d not found", id)
		}
	}
	for _, id := range mem.SourceReviewIDs {
		if id <= 0 {
			return fmt.Errorf("invalid source review id %d", id)
		}
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_reviews WHERE project_id=? AND id=?`, s.projectID, id).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("source review id %d not found", id)
		}
	}
	return nil
}

func scanTrackMemory(row rowScanner, mem *TrackMemory) error {
	var decisions, blockers, openQuestions, nextActions string
	var checkpointIDs, evidenceIDs, reviewIDs string
	if err := row.Scan(&mem.TrackID, &mem.Title, &mem.Purpose, &mem.OperatingMode, &mem.ActiveScope, &mem.CurrentObjective,
		&decisions, &blockers, &openQuestions, &nextActions, &checkpointIDs, &evidenceIDs, &reviewIDs,
		&mem.Owner, &mem.ReviewBy, &mem.Disposition, &mem.PromotionTarget, &mem.CanonicalCommit,
		&mem.SupersededByTrackID, &mem.UpdatedAt); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(decisions), &mem.Decisions); err != nil {
		return fmt.Errorf("decode track memory decisions: %w", err)
	}
	if err := json.Unmarshal([]byte(blockers), &mem.Blockers); err != nil {
		return fmt.Errorf("decode track memory blockers: %w", err)
	}
	if err := json.Unmarshal([]byte(openQuestions), &mem.OpenQuestions); err != nil {
		return fmt.Errorf("decode track memory open questions: %w", err)
	}
	if err := json.Unmarshal([]byte(nextActions), &mem.NextActions); err != nil {
		return fmt.Errorf("decode track memory next actions: %w", err)
	}
	if err := json.Unmarshal([]byte(checkpointIDs), &mem.SourceCheckpointIDs); err != nil {
		return fmt.Errorf("decode track memory source checkpoint ids: %w", err)
	}
	if err := json.Unmarshal([]byte(evidenceIDs), &mem.SourceEvidenceIDs); err != nil {
		return fmt.Errorf("decode track memory source evidence ids: %w", err)
	}
	if err := json.Unmarshal([]byte(reviewIDs), &mem.SourceReviewIDs); err != nil {
		return fmt.Errorf("decode track memory source review ids: %w", err)
	}
	return nil
}

func mergeTrackMemory(existing, update TrackMemory) TrackMemory {
	if update.Title != "" {
		existing.Title = update.Title
	}
	if update.Purpose != "" {
		existing.Purpose = update.Purpose
	}
	if update.OperatingMode != "" {
		existing.OperatingMode = update.OperatingMode
	}
	if update.ActiveScope != "" {
		existing.ActiveScope = update.ActiveScope
	}
	if update.CurrentObjective != "" {
		existing.CurrentObjective = update.CurrentObjective
	}
	if update.Owner != "" {
		existing.Owner = update.Owner
	}
	if update.ReviewBy != "" {
		existing.ReviewBy = update.ReviewBy
	}
	existing.Decisions = appendUniqueStrings(existing.Decisions, update.Decisions...)
	existing.Blockers = appendUniqueStrings(existing.Blockers, update.Blockers...)
	existing.OpenQuestions = appendUniqueStrings(existing.OpenQuestions, update.OpenQuestions...)
	existing.NextActions = appendUniqueStrings(existing.NextActions, update.NextActions...)
	existing.SourceCheckpointIDs = appendUniqueInt64s(existing.SourceCheckpointIDs, update.SourceCheckpointIDs...)
	existing.SourceEvidenceIDs = appendUniqueInt64s(existing.SourceEvidenceIDs, update.SourceEvidenceIDs...)
	existing.SourceReviewIDs = appendUniqueInt64s(existing.SourceReviewIDs, update.SourceReviewIDs...)
	return existing
}

func appendUniqueStrings(existing []string, values ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range append(existing, values...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func appendUniqueInt64s(existing []int64, values ...int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, value := range append(existing, values...) {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func mustJSONStrings(values []string) string {
	values = appendUniqueStrings(nil, values...)
	b, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func mustJSONInt64s(values []int64) string {
	values = appendUniqueInt64s(nil, values...)
	b, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (s *Store) StartWatcher(ctx context.Context, watcher Watcher) error {
	if watcher.ID == "" {
		return errors.New("watcher id is required")
	}
	if watcher.TaskID == "" {
		return errors.New("watcher task id is required")
	}
	if watcher.Status == "" {
		watcher.Status = "active"
	}
	if watcher.Status != "active" {
		return fmt.Errorf("invalid watcher start status %q", watcher.Status)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO task_watchers
  (project_id, id, task_id, owner, process, command, success, failure, status, started_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.projectID, watcher.ID, watcher.TaskID, watcher.Owner, watcher.Process, watcher.Command, watcher.Success, watcher.Failure, watcher.Status, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) FinishWatcher(ctx context.Context, id, result, artifact string, duration *int, notes string) error {
	switch result {
	case "pass", "fail", "blocked":
	default:
		return fmt.Errorf("invalid watcher result %q", result)
	}
	status := "done"
	if result == "blocked" {
		status = "blocked"
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE task_watchers
SET status=?, result=?, artifact_path=?, duration_seconds=?, notes=?, finished_at=?
WHERE project_id=? AND id=?`,
		status, result, artifact, duration, notes, time.Now().UTC().Format(time.RFC3339Nano), s.projectID, id)
	return checkWriteResult(res, err)
}

func (s *Store) Watchers(ctx context.Context, includeDone bool) ([]Watcher, error) {
	query := `
SELECT id, task_id, COALESCE(owner, ''), COALESCE(process, ''), COALESCE(command, ''),
       COALESCE(success, ''), COALESCE(failure, ''), status, COALESCE(result, ''),
       COALESCE(artifact_path, ''), duration_seconds, COALESCE(notes, ''), started_at, COALESCE(finished_at, '')
FROM task_watchers
WHERE project_id=?`
	if !includeDone {
		query += ` AND status NOT IN ('done', 'blocked')`
	}
	query += ` ORDER BY started_at DESC`
	rows, err := s.db.QueryContext(ctx, query, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Watcher
	for rows.Next() {
		var watcher Watcher
		var duration sql.NullInt64
		if err := rows.Scan(&watcher.ID, &watcher.TaskID, &watcher.Owner, &watcher.Process, &watcher.Command, &watcher.Success, &watcher.Failure, &watcher.Status, &watcher.Result, &watcher.ArtifactPath, &duration, &watcher.Notes, &watcher.StartedAt, &watcher.FinishedAt); err != nil {
			return nil, err
		}
		if duration.Valid {
			v := int(duration.Int64)
			watcher.DurationSeconds = &v
		}
		out = append(out, watcher)
	}
	return out, rows.Err()
}

func (s *Store) UpsertTrackerLink(ctx context.Context, link TrackerLink) error {
	if link.TaskID == "" || link.Provider == "" || link.ExternalID == "" {
		return errors.New("tracker links require task id, provider, and external id")
	}
	switch link.Provider {
	case "plane", "jira", "linear":
	default:
		return fmt.Errorf("unsupported tracker provider %q", link.Provider)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO tracker_links (project_id, task_id, provider, external_id, url, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(project_id, task_id, provider) DO UPDATE SET
  external_id=excluded.external_id,
  url=excluded.url,
  updated_at=excluded.updated_at`,
		s.projectID, link.TaskID, link.Provider, link.ExternalID, link.URL, now, now)
	return err
}

func (s *Store) RecordAudit(ctx context.Context, event AuditEvent) error {
	if event.Actor == "" {
		event.Actor = Actor()
	}
	if event.Action == "" {
		return errors.New("audit action is required")
	}
	return insertAudit(ctx, s.db, s.projectID, event)
}

func insertAudit(ctx context.Context, ex execer, projectID string, event AuditEvent) error {
	if event.Actor == "" {
		event.Actor = Actor()
	}
	if event.Action == "" {
		return errors.New("audit action is required")
	}
	_, err := ex.ExecContext(ctx, `
INSERT INTO audit_events (project_id, actor, action, task_id, detail, created_at)
VALUES (?, ?, ?, nullif(?, ''), ?, ?)`,
		projectID, event.Actor, event.Action, event.TaskID, event.Detail, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func validateServerWriteRequest(req ServerWriteRequest) error {
	if strings.TrimSpace(req.Actor) == "" {
		return errors.New("server write actor is required")
	}
	if strings.TrimSpace(req.Role) == "" {
		return errors.New("server write role is required")
	}
	if strings.TrimSpace(req.AuthSource) == "" {
		return errors.New("server write auth source is required")
	}
	if strings.TrimSpace(req.CommandFamily) == "" {
		return errors.New("server write command family is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return errors.New("server write idempotency key is required")
	}
	if strings.TrimSpace(req.PayloadDigest) == "" {
		return errors.New("server write payload digest is required")
	}
	return nil
}

func (s *Store) serverWriteReplay(ctx context.Context, q queryer, taskID, resultKind string, req ServerWriteRequest) (ServerWriteResult, bool, error) {
	var existing ServerWriteRequest
	var existingTaskID, existingResultKind string
	var resultID int64
	err := q.QueryRowContext(ctx, `
SELECT actor, role, auth_source, task_id, payload_digest, result_kind, result_id
FROM server_write_idempotency
WHERE project_id=? AND command_family=? AND idempotency_key=?`,
		s.projectID, req.CommandFamily, req.IdempotencyKey).Scan(
		&existing.Actor, &existing.Role, &existing.AuthSource, &existingTaskID, &existing.PayloadDigest, &existingResultKind, &resultID)
	if errors.Is(err, sql.ErrNoRows) {
		return ServerWriteResult{}, false, nil
	}
	if err != nil {
		return ServerWriteResult{}, false, err
	}
	if existing.Actor != req.Actor ||
		existing.Role != req.Role ||
		existing.AuthSource != req.AuthSource ||
		existingTaskID != taskID ||
		existing.PayloadDigest != req.PayloadDigest ||
		existingResultKind != resultKind {
		return ServerWriteResult{}, true, ErrIdempotencyConflict
	}
	return ServerWriteResult{RowID: resultID, Replayed: true}, true, nil
}

func (s *Store) ServerWriteReplay(ctx context.Context, taskID, resultKind string, req ServerWriteRequest) (ServerWriteResult, bool, error) {
	if err := validateServerWriteRequest(req); err != nil {
		return ServerWriteResult{}, false, err
	}
	return s.serverWriteReplay(ctx, s.db, taskID, resultKind, req)
}

func (s *Store) insertServerWriteIdempotency(ctx context.Context, ex execer, taskID, resultKind string, resultID int64, createdAt string, req ServerWriteRequest) error {
	_, err := ex.ExecContext(ctx, `
INSERT INTO server_write_idempotency
  (project_id, command_family, idempotency_key, actor, role, auth_source, task_id, payload_digest, result_kind, result_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.projectID, req.CommandFamily, req.IdempotencyKey, req.Actor, req.Role, req.AuthSource, taskID, req.PayloadDigest, resultKind, resultID, createdAt)
	return err
}

func serverWriteAuditDetail(req ServerWriteRequest, resultKind string, resultID int64, replayed bool) string {
	detail := struct {
		CommandFamily  string `json:"command_family"`
		IdempotencyKey string `json:"idempotency_key"`
		Role           string `json:"role"`
		AuthSource     string `json:"auth_source"`
		PayloadDigest  string `json:"payload_digest"`
		ResultKind     string `json:"result_kind"`
		ResultID       int64  `json:"result_id"`
		Replayed       bool   `json:"replayed"`
	}{
		CommandFamily:  req.CommandFamily,
		IdempotencyKey: req.IdempotencyKey,
		Role:           req.Role,
		AuthSource:     req.AuthSource,
		PayloadDigest:  req.PayloadDigest,
		ResultKind:     resultKind,
		ResultID:       resultID,
		Replayed:       replayed,
	}
	data, err := json.Marshal(detail)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (s *Store) AuditCount(ctx context.Context, action string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE project_id=? AND action=?`, s.projectID, action).Scan(&count)
	return count, err
}

func (s *Store) AuditEvents(ctx context.Context, action string) ([]AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT actor, action, COALESCE(task_id, ''), COALESCE(detail, ''), created_at
FROM audit_events
WHERE project_id=? AND action=?
ORDER BY id`, s.projectID, action)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var ev AuditEvent
		if err := rows.Scan(&ev.Actor, &ev.Action, &ev.TaskID, &ev.Detail, &ev.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

func (s *Store) TrackerLinks(ctx context.Context) ([]TrackerLink, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT task_id, provider, external_id, COALESCE(url, ''), created_at, updated_at
FROM tracker_links
WHERE project_id=?
ORDER BY provider, external_id`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrackerLink
	for rows.Next() {
		var link TrackerLink
		if err := rows.Scan(&link.TaskID, &link.Provider, &link.ExternalID, &link.URL, &link.CreatedAt, &link.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, link)
	}
	return out, rows.Err()
}

func (s *Store) SetStatus(ctx context.Context, taskID, status, reason string, requireBlockedReason bool) error {
	return s.SetStatusWithCommit(ctx, taskID, status, reason, "", requireBlockedReason)
}

func (s *Store) SetStatusWithCommit(ctx context.Context, taskID, status, reason, commitSHA string, requireBlockedReason bool) error {
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
	_, err = tx.ExecContext(ctx, `UPDATE task_state SET status=?, claimant=`+claimantSQL+`, completed_at=?, commit_sha=COALESCE(nullif(?, ''), commit_sha), updated_at=? WHERE project_id=? AND task_id=?`, status, completed, commitSHA, now, s.projectID, taskID)
	if err != nil {
		return err
	}
	if err := insertHistory(ctx, tx, s.projectID, taskID, fromStatus, status, owner, owner, branch, Actor(), reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetStatusWithIdempotency(ctx context.Context, taskID string, write GuardedStatusWrite, req ServerWriteRequest) (ServerWriteResult, error) {
	if write.Status == "" {
		return ServerWriteResult{}, errors.New("status is required")
	}
	if write.ExpectedStatus == "" {
		return ServerWriteResult{}, errors.New("expected_status is required")
	}
	if write.Status == "blocked" && write.RequireBlockedReason && strings.TrimSpace(write.Reason) == "" {
		return ServerWriteResult{}, errors.New("reason is required when blocking a task")
	}
	if err := validateServerWriteRequest(req); err != nil {
		return ServerWriteResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ServerWriteResult{}, err
	}
	defer tx.Rollback()
	if result, ok, err := s.serverWriteReplay(ctx, tx, taskID, "status", req); err != nil || ok {
		return result, err
	}
	var fromStatus, owner, branch, updatedAt string
	err = tx.QueryRowContext(ctx, `SELECT status, COALESCE(owner,''), COALESCE(branch,''), updated_at FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&fromStatus, &owner, &branch, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ServerWriteResult{}, ErrNotFound
	}
	if err != nil {
		return ServerWriteResult{}, err
	}
	if fromStatus != write.ExpectedStatus {
		return ServerWriteResult{}, TaskStateConflict{TaskID: taskID, ExpectedStatus: write.ExpectedStatus, ActualStatus: fromStatus, ActualOwner: owner, ActualUpdatedAt: updatedAt}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	completed := any(nil)
	if write.Status == "done" {
		completed = now
	}
	claimantSQL := "claimant"
	if write.Status != "in_progress" {
		claimantSQL = "NULL"
	}
	res, err := tx.ExecContext(ctx, `UPDATE task_state SET status=?, claimant=`+claimantSQL+`, completed_at=?, commit_sha=COALESCE(nullif(?, ''), commit_sha), updated_at=? WHERE project_id=? AND task_id=?`, write.Status, completed, write.CommitSHA, now, s.projectID, taskID)
	if err := checkWriteResult(res, err); err != nil {
		return ServerWriteResult{}, err
	}
	if err := insertHistory(ctx, tx, s.projectID, taskID, fromStatus, write.Status, owner, owner, branch, req.Actor, write.Reason); err != nil {
		return ServerWriteResult{}, err
	}
	if err := s.insertServerWriteIdempotency(ctx, tx, taskID, "status", 0, now, req); err != nil {
		return ServerWriteResult{}, err
	}
	if err := insertAudit(ctx, tx, s.projectID, AuditEvent{
		Actor:  req.Actor,
		Action: "server.api.status",
		TaskID: taskID,
		Detail: serverWriteAuditDetail(req, "status", 0, false),
	}); err != nil {
		return ServerWriteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ServerWriteResult{}, err
	}
	return ServerWriteResult{}, nil
}

func (s *Store) RecordEvidence(ctx context.Context, taskID string, ev Evidence) error {
	_, err := s.RecordEvidenceWithID(ctx, taskID, ev)
	return err
}

func (s *Store) RecordEvidenceWithID(ctx context.Context, taskID string, ev Evidence) (int64, error) {
	if ev.CommandText == "" {
		return 0, errors.New("command text is required")
	}
	switch ev.Result {
	case "pass", "fail", "partial", "skipped", "blocked":
	default:
		return 0, fmt.Errorf("invalid evidence result %q", ev.Result)
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO task_evidence
  (project_id, task_id, command_text, result, artifact_path, artifact_type, duration_seconds, notes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.projectID, taskID, ev.CommandText, ev.Result, ev.ArtifactPath, ev.ArtifactType, ev.DurationSeconds, ev.Notes, time.Now().UTC().Format(time.RFC3339Nano))
	if err := checkWriteResult(res, err); err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) RecordEvidenceWithIdempotency(ctx context.Context, taskID string, ev Evidence, req ServerWriteRequest) (ServerWriteResult, error) {
	if ev.CommandText == "" {
		return ServerWriteResult{}, errors.New("command text is required")
	}
	switch ev.Result {
	case "pass", "fail", "partial", "skipped", "blocked":
	default:
		return ServerWriteResult{}, fmt.Errorf("invalid evidence result %q", ev.Result)
	}
	if err := validateServerWriteRequest(req); err != nil {
		return ServerWriteResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ServerWriteResult{}, err
	}
	defer tx.Rollback()
	if result, ok, err := s.serverWriteReplay(ctx, tx, taskID, "evidence", req); err != nil || ok {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `
INSERT INTO task_evidence
  (project_id, task_id, command_text, result, artifact_path, artifact_type, duration_seconds, notes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.projectID, taskID, ev.CommandText, ev.Result, ev.ArtifactPath, ev.ArtifactType, ev.DurationSeconds, ev.Notes, now)
	if err := checkWriteResult(res, err); err != nil {
		return ServerWriteResult{}, err
	}
	rowID, err := res.LastInsertId()
	if err != nil {
		return ServerWriteResult{}, err
	}
	if err := s.insertServerWriteIdempotency(ctx, tx, taskID, "evidence", rowID, now, req); err != nil {
		return ServerWriteResult{}, err
	}
	if err := insertAudit(ctx, tx, s.projectID, AuditEvent{
		Actor:  req.Actor,
		Action: "server.api.evidence",
		TaskID: taskID,
		Detail: serverWriteAuditDetail(req, "evidence", rowID, false),
	}); err != nil {
		return ServerWriteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ServerWriteResult{}, err
	}
	return ServerWriteResult{RowID: rowID}, nil
}

var decisionAuthorityPattern = regexp.MustCompile(`(?i)\b(approve[sd]?|authorize[sd]?)\s+(the\s+)?(merge|deploy|release|live[ -]operation|public[ -]exposure|credential[ -]access)\b`)

func (s *Store) RecordTaskDecision(ctx context.Context, decision TaskDecision) (TaskDecision, error) {
	decision.TaskID = strings.TrimSpace(decision.TaskID)
	decision.Decision = strings.TrimSpace(decision.Decision)
	decision.Trigger = strings.TrimSpace(decision.Trigger)
	decision.Chosen = strings.TrimSpace(decision.Chosen)
	decision.Reason = strings.TrimSpace(decision.Reason)
	decision.Risk = strings.TrimSpace(decision.Risk)
	decision.Alternatives = uniqueNonEmptyStrings(decision.Alternatives)
	decision.ScopeAdded = uniqueNonEmptyStrings(decision.ScopeAdded)
	decision.ValidationRefs = uniqueNonEmptyStrings(decision.ValidationRefs)
	decision.FactRefs = uniqueNonEmptyStrings(decision.FactRefs)
	if decision.TaskID == "" || decision.Decision == "" || decision.Trigger == "" || decision.Chosen == "" || decision.Reason == "" || decision.Risk == "" {
		return TaskDecision{}, errors.New("task decision requires task, decision, trigger, chosen option, reason, and risk")
	}
	if len(decision.Alternatives) == 0 || len(decision.ValidationRefs) == 0 || len(decision.FactRefs) == 0 {
		return TaskDecision{}, errors.New("task decision requires at least one alternative, validation reference, and supporting fact reference")
	}
	if err := validateTaskDecisionText(decision); err != nil {
		return TaskDecision{}, err
	}
	if decision.CreatedBy == "" {
		decision.CreatedBy = Actor()
	}
	decision.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	alternativesJSON, _ := json.Marshal(decision.Alternatives)
	scopeAddedJSON, _ := json.Marshal(decision.ScopeAdded)
	validationJSON, _ := json.Marshal(decision.ValidationRefs)
	factRefsJSON, _ := json.Marshal(decision.FactRefs)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskDecision{}, err
	}
	defer tx.Rollback()
	var taskExists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_definitions WHERE project_id=? AND id=?`, s.projectID, decision.TaskID).Scan(&taskExists); err != nil {
		return TaskDecision{}, err
	}
	if taskExists == 0 {
		return TaskDecision{}, ErrNotFound
	}
	var supersedes any
	if decision.SupersedesID > 0 {
		var priorTask string
		if err := tx.QueryRowContext(ctx, `SELECT task_id FROM task_decisions WHERE project_id=? AND id=?`, s.projectID, decision.SupersedesID).Scan(&priorTask); errors.Is(err, sql.ErrNoRows) {
			return TaskDecision{}, errors.New("superseded decision not found")
		} else if err != nil {
			return TaskDecision{}, err
		}
		if priorTask != decision.TaskID {
			return TaskDecision{}, errors.New("superseded decision belongs to another task")
		}
		supersedes = decision.SupersedesID
	}
	res, err := tx.ExecContext(ctx, `
INSERT INTO task_decisions
  (project_id, task_id, decision, trigger_text, alternatives, chosen, reason, scope_added, risk, validation_refs, fact_refs, supersedes_id, created_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.projectID, decision.TaskID, decision.Decision, decision.Trigger, string(alternativesJSON), decision.Chosen, decision.Reason, string(scopeAddedJSON), decision.Risk, string(validationJSON), string(factRefsJSON), supersedes, decision.CreatedBy, decision.CreatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return TaskDecision{}, errors.New("superseded decision already has a replacement")
		}
		return TaskDecision{}, err
	}
	decision.ID, err = res.LastInsertId()
	if err != nil {
		return TaskDecision{}, err
	}
	if err := tx.Commit(); err != nil {
		return TaskDecision{}, err
	}
	decision.QualityState = "draft"
	decision.AuthorityBoundary = taskDecisionAuthorityBoundary
	return decision, nil
}

func (s *Store) AssessTaskDecision(ctx context.Context, taskID string, decisionID int64, quality, reviewer, reason string) error {
	taskID = strings.TrimSpace(taskID)
	quality = strings.TrimSpace(quality)
	reviewer = strings.TrimSpace(reviewer)
	reason = strings.TrimSpace(reason)
	if decisionID <= 0 || taskID == "" || reviewer == "" || reason == "" {
		return errors.New("decision assessment requires task, decision id, reviewer, and reason")
	}
	if quality != "accepted" && quality != "insufficient" {
		return fmt.Errorf("invalid decision quality %q", quality)
	}
	if err := validateTaskDecisionTextValues(reviewer, reason); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner, claimant string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(owner, ''), COALESCE(claimant, '') FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&owner, &claimant); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if reviewer == owner || (claimant != "" && reviewer == claimant) {
		return errors.New("reviewer cannot assess their own task decision")
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_decisions WHERE project_id=? AND task_id=? AND id=?`, s.projectID, taskID, decisionID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errors.New("task decision not found")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO task_decision_assessments (project_id, task_id, decision_id, quality_state, reviewer, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, s.projectID, taskID, decisionID, quality, reviewer, reason, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

const taskDecisionAuthorityBoundary = "decision records explain choices only; they do not grant approval, merge, deploy, credential, release, public-exposure, or live-operation authority"

func validateTaskDecisionText(decision TaskDecision) error {
	values := []string{decision.Decision, decision.Trigger, decision.Chosen, decision.Reason, decision.Risk, decision.CreatedBy}
	values = append(values, decision.Alternatives...)
	values = append(values, decision.ScopeAdded...)
	values = append(values, decision.ValidationRefs...)
	values = append(values, decision.FactRefs...)
	return validateTaskDecisionTextValues(values...)
}

func validateTaskDecisionTextValues(values ...string) error {
	markers := []string{"raw_prompt:", "raw_prompt=", "transcript:", "tool_body:", "tool_body=", "generated_content:", "generated_content=", "authorization:", "bearer ", "api_key=", "access_token=", "refresh_token=", "client_secret=", "password=", "secret="}
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, marker := range markers {
			if strings.Contains(lower, marker) {
				return errors.New("task decision rejected: unsafe private-data marker")
			}
		}
		if decisionAuthorityPattern.MatchString(value) {
			return errors.New("task decision rejected: decision text cannot grant consequential authority")
		}
	}
	return nil
}

func (s *Store) TaskDecisions(ctx context.Context, taskID string) ([]TaskDecision, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, task_id, decision, trigger_text, alternatives, chosen, reason, scope_added, risk, validation_refs, fact_refs,
       COALESCE(supersedes_id, 0), created_by, created_at
FROM task_decisions
WHERE project_id=? AND task_id=?
ORDER BY created_at, id`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var decisions []TaskDecision
	for rows.Next() {
		var decision TaskDecision
		var alternativesJSON, scopeAddedJSON, validationJSON, factRefsJSON string
		if err := rows.Scan(&decision.ID, &decision.TaskID, &decision.Decision, &decision.Trigger, &alternativesJSON, &decision.Chosen, &decision.Reason, &scopeAddedJSON, &decision.Risk, &validationJSON, &factRefsJSON, &decision.SupersedesID, &decision.CreatedBy, &decision.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(alternativesJSON), &decision.Alternatives); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(scopeAddedJSON), &decision.ScopeAdded); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(validationJSON), &decision.ValidationRefs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(factRefsJSON), &decision.FactRefs); err != nil {
			return nil, err
		}
		decision.QualityState = "draft"
		decision.AuthorityBoundary = taskDecisionAuthorityBoundary
		decisions = append(decisions, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	byID := make(map[int64]*TaskDecision, len(decisions))
	for i := range decisions {
		byID[decisions[i].ID] = &decisions[i]
		if decisions[i].SupersedesID > 0 {
			if prior := byID[decisions[i].SupersedesID]; prior != nil {
				prior.QualityState = "superseded"
				prior.SupersededByID = decisions[i].ID
			}
		}
	}
	assessmentRows, err := s.db.QueryContext(ctx, `SELECT decision_id, quality_state, reviewer, reason FROM task_decision_assessments WHERE project_id=? AND task_id=? ORDER BY created_at, id`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer assessmentRows.Close()
	for assessmentRows.Next() {
		var decisionID int64
		var quality, reviewer, reason string
		if err := assessmentRows.Scan(&decisionID, &quality, &reviewer, &reason); err != nil {
			return nil, err
		}
		if decision := byID[decisionID]; decision != nil {
			if decision.QualityState != "superseded" {
				decision.QualityState = quality
			}
			decision.QualityReviewer = reviewer
			decision.QualityReason = reason
		}
	}
	return decisions, assessmentRows.Err()
}

func (s *Store) RecordHandoff(ctx context.Context, taskID string, h Handoff) error {
	_, err := s.RecordHandoffWithID(ctx, taskID, h)
	return err
}

func (s *Store) RecordHandoffWithID(ctx context.Context, taskID string, h Handoff) (Handoff, error) {
	if h.ToRole == "" {
		return Handoff{}, errors.New("handoff target role is required")
	}
	if h.Payload == "" {
		return Handoff{}, errors.New("handoff payload is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO task_handoffs (project_id, task_id, from_role, to_role, payload, created_at)
SELECT project_id, task_id, COALESCE(owner, ''), ?, ?, ?
FROM task_state WHERE project_id=? AND task_id=?`,
		h.ToRole, h.Payload, now, s.projectID, taskID)
	if err := checkWriteResult(res, err); err != nil {
		return Handoff{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Handoff{}, err
	}
	h.ID = id
	h.CreatedAt = now
	return h, nil
}

func (s *Store) RecordNotification(ctx context.Context, n Notification) (Notification, error) {
	n.TaskID = strings.TrimSpace(n.TaskID)
	n.Domain = strings.TrimSpace(n.Domain)
	n.Provider = strings.TrimSpace(n.Provider)
	n.Target = strings.TrimSpace(n.Target)
	n.State = strings.TrimSpace(n.State)
	n.Reason = strings.TrimSpace(n.Reason)
	if n.TaskID == "" {
		return Notification{}, errors.New("notification task id is required")
	}
	if n.Domain == "" {
		return Notification{}, errors.New("notification domain is required")
	}
	switch n.State {
	case "intent", "handoff_recorded", "sent", "notification_delivered", "thread_steered", "acknowledged", "review_acknowledged", "review_recorded", "failed", "notification_failed":
	default:
		return Notification{}, fmt.Errorf("invalid notification state %q", n.State)
	}
	if (n.State == "failed" || n.State == "notification_failed") && n.Reason == "" {
		return Notification{}, errors.New("failed notification requires reason")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO task_notifications
  (project_id, task_id, handoff_id, domain, provider, target, state, reason, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.projectID, n.TaskID, n.HandoffID, n.Domain, n.Provider, n.Target, n.State, n.Reason, now)
	if err := checkWriteResult(res, err); err != nil {
		return Notification{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Notification{}, err
	}
	n.ID = id
	n.CreatedAt = now
	return n, nil
}

func (s *Store) Notifications(ctx context.Context, taskID string) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, task_id, handoff_id, domain, COALESCE(provider, ''), COALESCE(target, ''), state, COALESCE(reason, ''), created_at
FROM task_notifications
WHERE project_id=? AND (? = '' OR task_id=?)
ORDER BY created_at`, s.projectID, taskID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		var handoffID sql.NullInt64
		if err := rows.Scan(&n.ID, &n.TaskID, &handoffID, &n.Domain, &n.Provider, &n.Target, &n.State, &n.Reason, &n.CreatedAt); err != nil {
			return nil, err
		}
		if handoffID.Valid {
			value := handoffID.Int64
			n.HandoffID = &value
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) HandoffNotificationGaps(ctx context.Context) ([]HandoffNotificationGap, error) {
	return s.HandoffNotificationGapsFiltered(ctx, HandoffNotificationGapOptions{})
}

func (s *Store) HandoffNotificationGapsFiltered(ctx context.Context, opts HandoffNotificationGapOptions) ([]HandoffNotificationGap, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT hf.task_id, d.role, hf.to_role, hf.id, hf.created_at,
       st.status,
       st.review_required,
       COALESCE(st.review_status, '') AS review_status,
       COALESCE(MAX(n.created_at), '') AS last_notification_at,
       COALESCE((
         SELECT n2.state
         FROM task_notifications n2
         WHERE n2.project_id=hf.project_id
           AND n2.task_id=hf.task_id
           AND n2.domain=hf.to_role
           AND n2.created_at >= hf.created_at
         ORDER BY n2.created_at DESC
         LIMIT 1
       ), '') AS last_state,
       SUM(CASE WHEN n.state IN ('acknowledged', 'review_acknowledged', 'review_recorded', 'notification_delivered', 'thread_steered') THEN 1 ELSE 0 END) AS resolved_count,
       SUM(CASE WHEN n.state = 'sent' THEN 1 ELSE 0 END) AS sent_count,
       COALESCE(MAX(CASE WHEN n.state = 'sent' THEN n.created_at ELSE '' END), '') AS last_sent_at,
       SUM(CASE WHEN n.state IN ('intent', 'handoff_recorded', 'failed', 'notification_failed') THEN 1 ELSE 0 END) AS unresolved_count,
       (
         SELECT COUNT(*)
         FROM task_reviews r
         WHERE r.project_id=hf.project_id
           AND r.task_id=hf.task_id
           AND (r.review_domain=hf.to_role OR (COALESCE(r.review_domain, '') = '' AND r.reviewer=hf.to_role))
       ) AS review_count,
       COUNT(n.id) AS notification_count
FROM task_handoffs hf
JOIN task_definitions d ON d.project_id=hf.project_id AND d.id=hf.task_id
JOIN task_state st ON st.project_id=hf.project_id AND st.task_id=hf.task_id
LEFT JOIN task_notifications n ON n.project_id=hf.project_id
  AND n.task_id=hf.task_id
  AND n.domain=hf.to_role
  AND n.created_at >= hf.created_at
WHERE hf.project_id=?
  AND (? = '' OR hf.payload NOT LIKE ? || '%')
  AND COALESCE(hf.acknowledged_at, '') = ''
GROUP BY hf.task_id, d.role, hf.to_role, hf.id, hf.created_at, st.status, st.review_required, st.review_status
HAVING resolved_count = 0
  AND review_count = 0
  AND (
    notification_count = 0
    OR unresolved_count > 0
    OR (? <> '' AND sent_count > 0 AND last_sent_at < ?)
  )
ORDER BY hf.created_at DESC`, s.projectID, opts.ExcludePayloadPrefix, opts.ExcludePayloadPrefix, opts.SentStaleBefore, opts.SentStaleBefore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	terminal := map[string]bool{}
	for _, status := range opts.TerminalStatuses {
		status = strings.TrimSpace(status)
		if status != "" {
			terminal[status] = true
		}
	}
	var out []HandoffNotificationGap
	for rows.Next() {
		var gap HandoffNotificationGap
		var resolvedCount, sentCount, unresolvedCount, reviewCount, notificationCount int
		var lastSentAt string
		var taskStatus, reviewStatus string
		var reviewRequired bool
		if err := rows.Scan(&gap.TaskID, &gap.Role, &gap.Domain, &gap.HandoffID, &gap.LastHandoffAt, &taskStatus, &reviewRequired, &reviewStatus, &gap.LastNotificationAt, &gap.LastState, &resolvedCount, &sentCount, &lastSentAt, &unresolvedCount, &reviewCount, &notificationCount); err != nil {
			return nil, err
		}
		gap.NotificationStatus = notificationGapStatus(gap.LastState, gap.LastNotificationAt, opts.SentStaleBefore)
		if terminal[taskStatus] && !terminalNotificationGapStillRelevant(reviewRequired, reviewStatus, gap.LastState) {
			continue
		}
		out = append(out, gap)
	}
	return out, rows.Err()
}

func notificationGapStatus(lastState, lastNotificationAt, sentStaleBefore string) string {
	switch strings.TrimSpace(lastState) {
	case "":
		return "never-sent"
	case "handoff_recorded":
		return "handoff-recorded"
	case "sent":
		if sentStaleBefore != "" && lastNotificationAt != "" && lastNotificationAt < sentStaleBefore {
			return "stale-sent"
		}
		return "sent-awaiting-ack"
	case "notification_delivered":
		return "notification-delivered"
	case "thread_steered":
		return "thread-steered"
	case "acknowledged":
		return "acknowledged"
	case "review_acknowledged":
		return "review-acknowledged"
	case "review_recorded":
		return "review-recorded"
	case "failed", "notification_failed":
		return "notification-failed"
	default:
		return strings.TrimSpace(lastState)
	}
}

func terminalNotificationGapStillRelevant(reviewRequired bool, reviewStatus, lastNotificationState string) bool {
	if reviewRequired && strings.TrimSpace(reviewStatus) != "approved" {
		return true
	}
	switch strings.TrimSpace(lastNotificationState) {
	case "intent", "handoff_recorded", "failed", "notification_failed":
		return true
	default:
		return false
	}
}

func (s *Store) RecordReview(ctx context.Context, taskID string, r Review) error {
	_, err := s.RecordReviewWithID(ctx, taskID, r)
	return err
}

func (s *Store) RecordReviewWithID(ctx context.Context, taskID string, r Review) (int64, error) {
	if r.Reviewer == "" {
		return 0, errors.New("reviewer is required")
	}
	switch r.Verdict {
	case "approve", "changes", "reject":
	default:
		return 0, fmt.Errorf("invalid review verdict %q", r.Verdict)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var owner, claimant string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(owner, ''), COALESCE(claimant, '') FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&owner, &claimant)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if r.Reviewer == owner || (claimant != "" && r.Reviewer == claimant) {
		return 0, errors.New("reviewer cannot review their own task")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	domain := strings.TrimSpace(r.Domain)
	res, err := tx.ExecContext(ctx, `
INSERT INTO task_reviews (project_id, task_id, reviewer, review_domain, verdict, reviewed_commit_sha, route_reason, notes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.projectID, taskID, r.Reviewer, domain, r.Verdict, r.Commit, r.Reason, r.Reason, now)
	if err := checkWriteResult(res, err); err != nil {
		return 0, err
	}
	rowID, err := res.LastInsertId()
	if err != nil {
		return 0, err
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
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return rowID, nil
}

func (s *Store) RecordReviewWithIdempotency(ctx context.Context, taskID string, r Review, req ServerWriteRequest) (ServerWriteResult, error) {
	if r.Reviewer == "" {
		return ServerWriteResult{}, errors.New("reviewer is required")
	}
	switch r.Verdict {
	case "approve", "changes", "reject":
	default:
		return ServerWriteResult{}, fmt.Errorf("invalid review verdict %q", r.Verdict)
	}
	if err := validateServerWriteRequest(req); err != nil {
		return ServerWriteResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ServerWriteResult{}, err
	}
	defer tx.Rollback()
	if result, ok, err := s.serverWriteReplay(ctx, tx, taskID, "review", req); err != nil || ok {
		return result, err
	}
	var owner, claimant string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(owner, ''), COALESCE(claimant, '') FROM task_state WHERE project_id=? AND task_id=?`, s.projectID, taskID).Scan(&owner, &claimant)
	if errors.Is(err, sql.ErrNoRows) {
		return ServerWriteResult{}, ErrNotFound
	}
	if err != nil {
		return ServerWriteResult{}, err
	}
	if r.Reviewer == owner || (claimant != "" && r.Reviewer == claimant) {
		return ServerWriteResult{}, errors.New("reviewer cannot review their own task")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	domain := strings.TrimSpace(r.Domain)
	res, err := tx.ExecContext(ctx, `
INSERT INTO task_reviews (project_id, task_id, reviewer, review_domain, verdict, reviewed_commit_sha, route_reason, notes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.projectID, taskID, r.Reviewer, domain, r.Verdict, r.Commit, r.Reason, r.Reason, now)
	if err := checkWriteResult(res, err); err != nil {
		return ServerWriteResult{}, err
	}
	rowID, err := res.LastInsertId()
	if err != nil {
		return ServerWriteResult{}, err
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
		return ServerWriteResult{}, err
	}
	if err := s.insertServerWriteIdempotency(ctx, tx, taskID, "review", rowID, now, req); err != nil {
		return ServerWriteResult{}, err
	}
	if err := insertAudit(ctx, tx, s.projectID, AuditEvent{
		Actor:  req.Actor,
		Action: "server.api.review",
		TaskID: taskID,
		Detail: serverWriteAuditDetail(req, "review", rowID, false),
	}); err != nil {
		return ServerWriteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ServerWriteResult{}, err
	}
	return ServerWriteResult{RowID: rowID}, nil
}

func (s *Store) TaskDetail(ctx context.Context, taskID string) (Task, []Transition, []Evidence, []Handoff, []Review, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT d.id, d.parent_id, d.kind, d.title, d.role, d.notes, d.acceptance_checks, d.dependencies,
       d.priority, d.sequence, d.profile, d.owning_domain, d.owning_layer, d.source_paths, d.target_paths, d.review_domains, d.tags, d.risk_level, d.migration_type,
       st.status, st.owner, st.claimant, st.branch, st.completed_at, st.commit_sha, st.review_status, st.updated_at
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

func (s *Store) RecordProviderUsage(ctx context.Context, usage ProviderUsage) (ProviderUsage, error) {
	usage.Provider = strings.TrimSpace(usage.Provider)
	usage.TaskID = strings.TrimSpace(usage.TaskID)
	if usage.Provider == "" {
		return ProviderUsage{}, errors.New("provider is required")
	}
	if usage.Source == "" {
		usage.Source = "unknown"
	}
	if usage.Confidence == "" {
		usage.Confidence = "unknown"
	}
	if err := validateUsageSource(usage.Source); err != nil {
		return ProviderUsage{}, err
	}
	if err := validateUsageConfidence(usage.Confidence); err != nil {
		return ProviderUsage{}, err
	}
	if usage.TotalTokens == nil && usage.StartedTokenSnapshot != nil && usage.CompletedTokenSnapshot != nil && *usage.CompletedTokenSnapshot >= *usage.StartedTokenSnapshot {
		v := *usage.CompletedTokenSnapshot - *usage.StartedTokenSnapshot
		usage.TotalTokens = &v
		if usage.Source == "unknown" {
			usage.Source = "derived_snapshot"
		}
	}
	if usage.UncachedInputTokens == nil && usage.InputTokens != nil && usage.CachedInputTokens != nil && *usage.InputTokens >= *usage.CachedInputTokens {
		v := *usage.InputTokens - *usage.CachedInputTokens
		usage.UncachedInputTokens = &v
	}
	if usage.CreatedAt == "" {
		usage.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO provider_usage_events
  (project_id, provider, external_session_id, session_id, task_id, role, phase, source, confidence,
   started_at, completed_at, started_token_snapshot, completed_token_snapshot,
   input_tokens, cached_input_tokens, uncached_input_tokens, output_tokens, reasoning_tokens,
   total_tokens, elapsed_seconds, model, metadata_json, created_at)
VALUES (?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), nullif(?, ''), nullif(?, ''), ?, ?,
        nullif(?, ''), nullif(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, nullif(?, ''), nullif(?, ''), ?)`,
		s.projectID, usage.Provider, usage.ExternalSessionID, usage.SessionID, usage.TaskID, usage.Role, usage.Phase, usage.Source, usage.Confidence,
		usage.StartedAt, usage.CompletedAt, usage.StartedTokenSnapshot, usage.CompletedTokenSnapshot,
		usage.InputTokens, usage.CachedInputTokens, usage.UncachedInputTokens, usage.OutputTokens, usage.ReasoningTokens,
		usage.TotalTokens, usage.ElapsedSeconds, usage.Model, usage.MetadataJSON, usage.CreatedAt)
	if err != nil {
		return ProviderUsage{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ProviderUsage{}, err
	}
	usage.ID = id
	return usage, nil
}

func validateUsageSource(source string) error {
	switch source {
	case "provider_reported", "derived_snapshot", "manual", "unknown":
		return nil
	default:
		return fmt.Errorf("invalid usage source %q", source)
	}
}

func validateUsageConfidence(confidence string) error {
	switch confidence {
	case "exact", "estimated", "unknown":
		return nil
	default:
		return fmt.Errorf("invalid usage confidence %q", confidence)
	}
}

func (s *Store) ProviderUsageForTask(ctx context.Context, taskID string) ([]ProviderUsage, error) {
	rows, err := s.db.QueryContext(ctx, providerUsageSelectSQL(`WHERE project_id=? AND task_id=? ORDER BY created_at, id`), s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProviderUsageRows(rows)
}

func (s *Store) ProviderUsageEvents(ctx context.Context, opts UsageRollupOptions) ([]ProviderUsage, error) {
	where := []string{"project_id=?"}
	args := []any{s.projectID}
	if opts.TaskID != "" {
		where = append(where, "task_id=?")
		args = append(args, opts.TaskID)
	}
	if opts.Since != "" {
		where = append(where, "created_at>=?")
		args = append(args, opts.Since)
	}
	if opts.Until != "" {
		where = append(where, "created_at<?")
		args = append(args, opts.Until)
	}
	rows, err := s.db.QueryContext(ctx, providerUsageSelectSQL("WHERE "+strings.Join(where, " AND ")+" ORDER BY created_at, id"), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProviderUsageRows(rows)
}

func (s *Store) UsageRollups(ctx context.Context, opts UsageRollupOptions) ([]UsageRollup, error) {
	events, err := s.ProviderUsageEvents(ctx, opts)
	if err != nil {
		return nil, err
	}
	groupBy := opts.GroupBy
	if groupBy == "" {
		groupBy = "provider"
	}
	knownTasks := map[string]Task{}
	var tasks []Task
	if groupBy == "kind" || groupBy == "epic" {
		tasks, err = s.AllTasks(ctx)
		if err != nil {
			return nil, err
		}
		for _, task := range tasks {
			knownTasks[task.Definition.ID] = task
		}
	}
	byKey := map[string]*UsageRollup{}
	for _, ev := range events {
		key := usageRollupKey(ev, groupBy, knownTasks)
		roll := byKey[key]
		if roll == nil {
			roll = &UsageRollup{Group: groupBy, Key: key}
			byKey[key] = roll
		}
		roll.Events++
		addUsageInt(&roll.TotalTokens, ev.TotalTokens)
		if ev.TotalTokens != nil {
			roll.KnownTotalEvents++
		}
		addUsageInt(&roll.InputTokens, ev.InputTokens)
		addUsageInt(&roll.CachedInputTokens, ev.CachedInputTokens)
		addUsageInt(&roll.UncachedInputTokens, ev.UncachedInputTokens)
		addUsageInt(&roll.OutputTokens, ev.OutputTokens)
		addUsageInt(&roll.ReasoningTokens, ev.ReasoningTokens)
		addUsageInt(&roll.ElapsedSeconds, ev.ElapsedSeconds)
	}
	out := make([]UsageRollup, 0, len(byKey))
	for _, roll := range byKey {
		out = append(out, *roll)
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := 0, 0
		if out[i].TotalTokens != nil {
			left = *out[i].TotalTokens
		}
		if out[j].TotalTokens != nil {
			right = *out[j].TotalTokens
		}
		if left != right {
			return left > right
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func usageRollupKey(ev ProviderUsage, groupBy string, tasks map[string]Task) string {
	switch groupBy {
	case "task":
		return firstNonEmpty(ev.TaskID, "unassigned")
	case "epic":
		if task, ok := tasks[ev.TaskID]; ok {
			return firstNonEmpty(task.Definition.ParentID, task.Definition.ID, "unassigned")
		}
		return "unassigned"
	case "role":
		return firstNonEmpty(ev.Role, "unknown")
	case "day":
		if len(ev.CreatedAt) >= len("2006-01-02") {
			return ev.CreatedAt[:len("2006-01-02")]
		}
		return "unknown"
	case "kind":
		if task, ok := tasks[ev.TaskID]; ok {
			return firstNonEmpty(task.Definition.Kind, "unknown")
		}
		return "unknown"
	case "phase":
		return firstNonEmpty(ev.Phase, "unknown")
	case "model":
		return firstNonEmpty(ev.Model, "unknown")
	case "provider":
		fallthrough
	default:
		return firstNonEmpty(ev.Provider, "unknown")
	}
}

func addUsageInt(total **int, value *int) {
	if value == nil {
		return
	}
	if *total == nil {
		v := *value
		*total = &v
		return
	}
	**total += *value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Store) UpsertWorkBatch(ctx context.Context, batch WorkBatch) error {
	batch.ID = strings.TrimSpace(batch.ID)
	if batch.ID == "" {
		return errors.New("batch id is required")
	}
	if strings.TrimSpace(batch.Title) == "" {
		return errors.New("batch title is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	validation, err := json.Marshal(batch.ValidationCommands)
	if err != nil {
		return err
	}
	reviewDomains, err := json.Marshal(batch.ReviewDomains)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO work_batches
  (project_id, id, title, branch, worktree_path, validation_commands, review_domains, rollback_criteria, split_criteria, expected_ci, deploy_run_id, pipeline_id, created_at, updated_at)
VALUES (?, ?, ?, nullif(?, ''), nullif(?, ''), ?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), nullif(?, ''), nullif(?, ''), ?, ?)
ON CONFLICT(project_id, id) DO UPDATE SET
  title=excluded.title,
  branch=excluded.branch,
  worktree_path=excluded.worktree_path,
  validation_commands=excluded.validation_commands,
  review_domains=excluded.review_domains,
  rollback_criteria=excluded.rollback_criteria,
  split_criteria=excluded.split_criteria,
  expected_ci=excluded.expected_ci,
  deploy_run_id=excluded.deploy_run_id,
  pipeline_id=excluded.pipeline_id,
  updated_at=excluded.updated_at`,
		s.projectID, batch.ID, batch.Title, batch.Branch, batch.WorktreePath, string(validation), string(reviewDomains), batch.RollbackCriteria, batch.SplitCriteria, batch.ExpectedCI, batch.DeployRunID, batch.PipelineID, now, now)
	if err != nil {
		return err
	}
	if len(batch.Tasks) > 0 {
		return s.AddTasksToWorkBatch(ctx, batch.ID, batch.Tasks)
	}
	return nil
}

func (s *Store) AddTasksToWorkBatch(ctx context.Context, batchID string, taskIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_batches WHERE project_id=? AND id=?`, s.projectID, batchID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, taskID := range taskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_definitions WHERE project_id=? AND id=?`, s.projectID, taskID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("task %s: %w", taskID, ErrNotFound)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO work_batch_tasks(project_id, batch_id, task_id, created_at) VALUES (?, ?, ?, ?)`, s.projectID, batchID, taskID, now); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE work_batches SET updated_at=? WHERE project_id=? AND id=?`, now, s.projectID, batchID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RemoveTasksFromWorkBatch(ctx context.Context, batchID string, taskIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, taskID := range taskIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM work_batch_tasks WHERE project_id=? AND batch_id=? AND task_id=?`, s.projectID, batchID, taskID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE work_batches SET updated_at=? WHERE project_id=? AND id=?`, time.Now().UTC().Format(time.RFC3339Nano), s.projectID, batchID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LinkWorkBatch(ctx context.Context, batchID, deployRunID, pipelineID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE work_batches SET deploy_run_id=COALESCE(NULLIF(?, ''), deploy_run_id), pipeline_id=COALESCE(NULLIF(?, ''), pipeline_id), updated_at=? WHERE project_id=? AND id=?`, deployRunID, pipelineID, time.Now().UTC().Format(time.RFC3339Nano), s.projectID, batchID)
	return checkWriteResult(res, err)
}

func (s *Store) RecordWorkBatchEvidence(ctx context.Context, batchID string, evidence WorkBatchEvidence, mapToTasks bool) (WorkBatchEvidence, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkBatchEvidence{}, err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_batches WHERE project_id=? AND id=?`, s.projectID, batchID).Scan(&exists); err != nil {
		return WorkBatchEvidence{}, err
	}
	if exists == 0 {
		return WorkBatchEvidence{}, ErrNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, `
INSERT INTO work_batch_evidence(project_id, batch_id, command_text, result, artifact_path, artifact_type, notes, created_at)
VALUES (?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), nullif(?, ''), nullif(?, ''), ?)`,
		s.projectID, batchID, evidence.CommandText, evidence.Result, evidence.ArtifactPath, evidence.ArtifactType, evidence.Notes, now)
	if err != nil {
		return WorkBatchEvidence{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return WorkBatchEvidence{}, err
	}
	evidence.ID = id
	evidence.BatchID = batchID
	evidence.CreatedAt = now
	if mapToTasks {
		rows, err := tx.QueryContext(ctx, `SELECT task_id FROM work_batch_tasks WHERE project_id=? AND batch_id=? ORDER BY task_id`, s.projectID, batchID)
		if err != nil {
			return WorkBatchEvidence{}, err
		}
		var taskIDs []string
		for rows.Next() {
			var taskID string
			if err := rows.Scan(&taskID); err != nil {
				rows.Close()
				return WorkBatchEvidence{}, err
			}
			taskIDs = append(taskIDs, taskID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return WorkBatchEvidence{}, err
		}
		rows.Close()
		for _, taskID := range taskIDs {
			notes := strings.TrimSpace(strings.Join([]string{fmt.Sprintf("work_batch=%s", batchID), evidence.Notes}, " "))
			if _, err := tx.ExecContext(ctx, `
INSERT INTO task_evidence(project_id, task_id, command_text, result, artifact_path, artifact_type, notes, created_at)
VALUES (?, ?, ?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), ?)`,
				s.projectID, taskID, "batch "+batchID+": "+evidence.CommandText, evidence.Result, evidence.ArtifactPath, firstNonEmpty(evidence.ArtifactType, "work-batch"), notes, now); err != nil {
				return WorkBatchEvidence{}, err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE work_batches SET updated_at=? WHERE project_id=? AND id=?`, now, s.projectID, batchID); err != nil {
		return WorkBatchEvidence{}, err
	}
	return evidence, tx.Commit()
}

func (s *Store) WorkBatch(ctx context.Context, batchID string) (WorkBatch, []WorkBatchEvidence, error) {
	row := s.db.QueryRowContext(ctx, workBatchSelectSQL(`WHERE project_id=? AND id=?`), s.projectID, batchID)
	batch, err := scanWorkBatch(row)
	if err != nil {
		return WorkBatch{}, nil, err
	}
	tasks, err := s.workBatchTasks(ctx, batchID)
	if err != nil {
		return WorkBatch{}, nil, err
	}
	batch.Tasks = tasks
	evidence, err := s.workBatchEvidence(ctx, batchID)
	if err != nil {
		return WorkBatch{}, nil, err
	}
	return batch, evidence, nil
}

func (s *Store) WorkBatches(ctx context.Context) ([]WorkBatch, error) {
	rows, err := s.db.QueryContext(ctx, workBatchSelectSQL(`WHERE project_id=? ORDER BY updated_at DESC, id`), s.projectID)
	if err != nil {
		return nil, err
	}
	var batches []WorkBatch
	for rows.Next() {
		batch, err := scanWorkBatch(rows)
		if err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range batches {
		tasks, err := s.workBatchTasks(ctx, batches[i].ID)
		if err != nil {
			return nil, err
		}
		batches[i].Tasks = tasks
	}
	return batches, nil
}

func (s *Store) AllTasks(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT d.id, d.parent_id, d.kind, d.title, d.role, d.notes, d.acceptance_checks, d.dependencies,
       d.priority, d.sequence, d.profile, d.owning_domain, d.owning_layer, d.source_paths, d.target_paths, d.review_domains, d.tags, d.risk_level, d.migration_type,
       st.status, st.owner, st.claimant, st.branch, st.completed_at, st.commit_sha, st.review_status, st.updated_at
FROM task_definitions d JOIN task_state st ON st.project_id=d.project_id AND st.task_id=d.id
WHERE d.project_id=?
ORDER BY d.role, st.status, COALESCE(d.priority, 9999), d.created_at`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) TasksFiltered(ctx context.Context, opts TaskFilterOptions) ([]Task, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return nil, err
	}
	tags := normalizedTags(opts.Tags)
	if len(tags) == 0 {
		return tasks, nil
	}
	var out []Task
	for _, task := range tasks {
		if taskHasAllTags(task.Definition.Tags, tags) {
			out = append(out, task)
		}
	}
	return out, nil
}

func taskHasAllTags(taskTags, want []string) bool {
	if len(want) == 0 {
		return true
	}
	set := map[string]bool{}
	for _, tag := range taskTags {
		set[tag] = true
	}
	for _, tag := range want {
		if !set[tag] {
			return false
		}
	}
	return true
}

func normalizedTags(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (s *Store) Activity(ctx context.Context, limit int) ([]Activity, error) {
	return s.ActivityFiltered(ctx, ActivityOptions{Limit: limit})
}

func (s *Store) ActivityFiltered(ctx context.Context, opts ActivityOptions) ([]Activity, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	where := []string{}
	args := []any{s.projectID, s.projectID, s.projectID, s.projectID, s.projectID}
	joinSQL := ""
	if strings.TrimSpace(opts.Profile) != "" {
		joinSQL = "LEFT JOIN task_definitions d ON d.project_id = ? AND d.id = a.task_id"
		args = append(args, s.projectID)
	}
	if strings.TrimSpace(opts.Kind) != "" {
		where = append(where, "a.kind = ?")
		args = append(args, strings.TrimSpace(opts.Kind))
	}
	if strings.TrimSpace(opts.TaskID) != "" {
		where = append(where, "a.task_id = ?")
		args = append(args, strings.TrimSpace(opts.TaskID))
	}
	if strings.TrimSpace(opts.CreatedFrom) != "" {
		where = append(where, "a.created_at >= ?")
		args = append(args, strings.TrimSpace(opts.CreatedFrom))
	}
	if strings.TrimSpace(opts.CreatedTo) != "" {
		where = append(where, "a.created_at <= ?")
		args = append(args, strings.TrimSpace(opts.CreatedTo))
	}
	if strings.TrimSpace(opts.Profile) != "" {
		where = append(where, "COALESCE(d.profile, '') = ?")
		args = append(args, strings.TrimSpace(opts.Profile))
	}
	filterSQL := ""
	if len(where) > 0 {
		filterSQL = "WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, opts.Limit)
	rows, err := s.db.QueryContext(ctx, `
SELECT a.kind, a.task_id, a.summary, a.actor, a.created_at
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
  UNION ALL
  SELECT 'notification', task_id, state || ' to ' || domain || CASE WHEN provider IS NOT NULL AND provider != '' THEN ' via ' || provider ELSE '' END, COALESCE(provider, ''), created_at
    FROM task_notifications WHERE project_id=?
) a
`+joinSQL+`
`+filterSQL+`
ORDER BY a.created_at DESC
LIMIT ?`, args...)
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

func (s *Store) EventSources(ctx context.Context, limit int) ([]EventSource, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT at, source_order, id, source, task_id, role, owner, from_status, to_status,
       actor, reason, from_role, to_role, evidence_type, evidence_count,
       reviewer, verdict, session_id, provider, end_reason
FROM (
  SELECT h.at AS at, 10 AS source_order, h.id AS id, 'history' AS source,
         h.task_id AS task_id, d.role AS role, COALESCE(h.to_owner, st.owner, '') AS owner,
         COALESCE(h.from_status, '') AS from_status, h.to_status AS to_status,
         h.actor AS actor, COALESCE(h.reason, '') AS reason,
         '' AS from_role, '' AS to_role, '' AS evidence_type, 0 AS evidence_count,
         '' AS reviewer, '' AS verdict, '' AS session_id, '' AS provider, '' AS end_reason
    FROM task_state_history h
    JOIN task_definitions d ON d.project_id=h.project_id AND d.id=h.task_id
    JOIN task_state st ON st.project_id=h.project_id AND st.task_id=h.task_id
   WHERE h.project_id=?
  UNION ALL
  SELECT e.created_at, 20, e.id, 'evidence',
         e.task_id, d.role, COALESCE(st.owner, ''),
         '', '', '', '', '', '',
         COALESCE(NULLIF(e.artifact_type, ''), NULLIF(e.result, ''), 'evidence'),
         (SELECT COUNT(*) FROM task_evidence ec WHERE ec.project_id=e.project_id AND ec.task_id=e.task_id),
         '', '', '', '', ''
    FROM task_evidence e
    JOIN task_definitions d ON d.project_id=e.project_id AND d.id=e.task_id
    JOIN task_state st ON st.project_id=e.project_id AND st.task_id=e.task_id
   WHERE e.project_id=?
  UNION ALL
  SELECT hf.created_at, 30, hf.id, 'handoff',
         hf.task_id, d.role, COALESCE(st.owner, ''),
         '', '', COALESCE(hf.from_role, ''), COALESCE(hf.payload, ''),
         COALESCE(hf.from_role, ''), COALESCE(hf.to_role, ''),
         '', 0, '', '', '', '', ''
    FROM task_handoffs hf
    JOIN task_definitions d ON d.project_id=hf.project_id AND d.id=hf.task_id
    JOIN task_state st ON st.project_id=hf.project_id AND st.task_id=hf.task_id
   WHERE hf.project_id=?
  UNION ALL
  SELECT r.created_at, 40, r.id, 'review',
         r.task_id, d.role, COALESCE(st.owner, ''),
         '', '', COALESCE(r.reviewer, ''), COALESCE(r.notes, ''),
         '', '', '', 0, COALESCE(r.reviewer, ''), COALESCE(r.verdict, ''),
         '', '', ''
    FROM task_reviews r
    JOIN task_definitions d ON d.project_id=r.project_id AND d.id=r.task_id
    JOIN task_state st ON st.project_id=r.project_id AND st.task_id=r.task_id
   WHERE r.project_id=?
  UNION ALL
  SELECT s.started_at, 50, s.rowid, 'session_attach',
         COALESCE(s.task_id, ''), s.role, '',
         '', '', '', '', '', '', '', 0, '', '',
         s.id, COALESCE(s.provider, ''), ''
    FROM agent_sessions s
   WHERE s.project_id=?
  UNION ALL
  SELECT s.last_heartbeat_at, 60, s.rowid, 'session_heartbeat',
         COALESCE(s.task_id, ''), s.role, '',
         '', '', '', '', '', '', '', 0, '', '',
         s.id, COALESCE(s.provider, ''), ''
    FROM agent_sessions s
   WHERE s.project_id=? AND s.last_heartbeat_at IS NOT NULL
  UNION ALL
  SELECT s.ended_at, 70, s.rowid, 'session_detach',
         COALESCE(s.task_id, ''), s.role, '',
         '', '', '', '', '', '', '', 0, '', '',
         s.id, COALESCE(s.provider, ''), COALESCE(s.end_reason, s.status, '')
    FROM agent_sessions s
   WHERE s.project_id=? AND s.ended_at IS NOT NULL
)
WHERE at IS NOT NULL AND at != ''
ORDER BY at DESC, source_order DESC, id DESC
LIMIT ?`, s.projectID, s.projectID, s.projectID, s.projectID, s.projectID, s.projectID, s.projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventSource
	for rows.Next() {
		var ev EventSource
		if err := rows.Scan(&ev.Cursor.At, &ev.Cursor.SourceOrder, &ev.Cursor.ID, &ev.Source, &ev.TaskID, &ev.Role, &ev.Owner, &ev.FromStatus, &ev.ToStatus, &ev.Actor, &ev.Reason, &ev.FromRole, &ev.ToRole, &ev.EvidenceType, &ev.EvidenceCount, &ev.Reviewer, &ev.Verdict, &ev.SessionID, &ev.Provider, &ev.EndReason); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) Health(ctx context.Context) (Health, error) {
	var h Health
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_state WHERE project_id=? AND status='in_progress'`, s.projectID).Scan(&h.InProgress); err != nil {
		return Health{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_state WHERE project_id=? AND status='in_progress' AND claimed_at < ?`, s.projectID, time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339Nano)).Scan(&h.StaleInProgress); err != nil {
		return Health{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_state WHERE project_id=? AND status='blocked' AND updated_at < ?`, s.projectID, time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339Nano)).Scan(&h.BlockedOver24h); err != nil {
		return Health{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_handoffs WHERE project_id=? AND acknowledged_at IS NULL`, s.projectID).Scan(&h.UnacknowledgedHandoff); err != nil {
		return Health{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_handoffs WHERE project_id=? AND acknowledged_at IS NULL AND created_at < ?`, s.projectID, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)).Scan(&h.UnacknowledgedOver1Hour); err != nil {
		return Health{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_state WHERE project_id=? AND review_required=1 AND COALESCE(review_status, '') IN ('', 'pending')`, s.projectID).Scan(&h.UnroutedReviews); err != nil {
		return Health{}, err
	}
	return h, nil
}

func (s *Store) LatestHistoryID(ctx context.Context) (int64, error) {
	var id sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(id) FROM task_state_history WHERE project_id=?`, s.projectID).Scan(&id)
	if err != nil {
		return 0, err
	}
	if !id.Valid {
		return 0, nil
	}
	return id.Int64, nil
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

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
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
	var sourcePaths, targetPaths, reviewDomains, tags sql.NullString
	var parent, kind, notes, profile, owningDomain, owningLayer, riskLevel, migrationType, owner, claimant, branch, completedAt, commitSHA, reviewStatus, updated sql.NullString
	var priority, sequence sql.NullInt64
	err := row.Scan(&task.Definition.ID, &parent, &kind, &task.Definition.Title, &task.Definition.Role, &notes, &acceptance, &deps, &priority, &sequence, &profile, &owningDomain, &owningLayer, &sourcePaths, &targetPaths, &reviewDomains, &tags, &riskLevel, &migrationType, &task.Status, &owner, &claimant, &branch, &completedAt, &commitSHA, &reviewStatus, &updated)
	if err != nil {
		return Task{}, err
	}
	task.Definition.ParentID = parent.String
	task.Definition.Kind = kind.String
	task.Definition.Notes = notes.String
	task.Definition.Profile = profile.String
	task.Definition.OwningDomain = owningDomain.String
	task.Definition.OwningLayer = owningLayer.String
	task.Definition.RiskLevel = riskLevel.String
	task.Definition.MigrationType = migrationType.String
	_ = json.Unmarshal([]byte(acceptance), &task.Definition.AcceptanceChecks)
	_ = json.Unmarshal([]byte(deps), &task.Definition.Dependencies)
	if sourcePaths.Valid {
		_ = json.Unmarshal([]byte(sourcePaths.String), &task.Definition.SourcePaths)
	}
	if targetPaths.Valid {
		_ = json.Unmarshal([]byte(targetPaths.String), &task.Definition.TargetPaths)
	}
	if reviewDomains.Valid {
		_ = json.Unmarshal([]byte(reviewDomains.String), &task.Definition.ReviewDomains)
	}
	if tags.Valid {
		_ = json.Unmarshal([]byte(tags.String), &task.Definition.Tags)
	}
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
	task.CompletedAt = completedAt.String
	task.CommitSHA = commitSHA.String
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

func providerUsageSelectSQL(suffix string) string {
	return `
SELECT id, provider, external_session_id, session_id, task_id, role, phase, source, confidence,
       started_at, completed_at, started_token_snapshot, completed_token_snapshot,
       input_tokens, cached_input_tokens, uncached_input_tokens, output_tokens, reasoning_tokens,
       total_tokens, elapsed_seconds, model, metadata_json, created_at
FROM provider_usage_events ` + suffix
}

func scanProviderUsageRows(rows *sql.Rows) ([]ProviderUsage, error) {
	var out []ProviderUsage
	for rows.Next() {
		item, err := scanProviderUsage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanProviderUsage(row rowScanner) (ProviderUsage, error) {
	var item ProviderUsage
	var externalSessionID, sessionID, taskID, role, phase, startedAt, completedAt, model, metadataJSON sql.NullString
	var startedSnapshot, completedSnapshot, inputTokens, cachedInputTokens, uncachedInputTokens, outputTokens, reasoningTokens, totalTokens, elapsedSeconds sql.NullInt64
	err := row.Scan(&item.ID, &item.Provider, &externalSessionID, &sessionID, &taskID, &role, &phase, &item.Source, &item.Confidence,
		&startedAt, &completedAt, &startedSnapshot, &completedSnapshot,
		&inputTokens, &cachedInputTokens, &uncachedInputTokens, &outputTokens, &reasoningTokens,
		&totalTokens, &elapsedSeconds, &model, &metadataJSON, &item.CreatedAt)
	if err != nil {
		return ProviderUsage{}, err
	}
	item.ExternalSessionID = externalSessionID.String
	item.SessionID = sessionID.String
	item.TaskID = taskID.String
	item.Role = role.String
	item.Phase = phase.String
	item.StartedAt = startedAt.String
	item.CompletedAt = completedAt.String
	item.StartedTokenSnapshot = nullableIntPtr(startedSnapshot)
	item.CompletedTokenSnapshot = nullableIntPtr(completedSnapshot)
	item.InputTokens = nullableIntPtr(inputTokens)
	item.CachedInputTokens = nullableIntPtr(cachedInputTokens)
	item.UncachedInputTokens = nullableIntPtr(uncachedInputTokens)
	item.OutputTokens = nullableIntPtr(outputTokens)
	item.ReasoningTokens = nullableIntPtr(reasoningTokens)
	item.TotalTokens = nullableIntPtr(totalTokens)
	item.ElapsedSeconds = nullableIntPtr(elapsedSeconds)
	item.Model = model.String
	item.MetadataJSON = metadataJSON.String
	return item, nil
}

func workBatchSelectSQL(suffix string) string {
	return `
SELECT id, title, branch, worktree_path, validation_commands, review_domains, rollback_criteria, split_criteria, expected_ci, deploy_run_id, pipeline_id, created_at, updated_at
FROM work_batches ` + suffix
}

func scanWorkBatch(row rowScanner) (WorkBatch, error) {
	var batch WorkBatch
	var branch, worktree, rollback, split, expectedCI, deployRun, pipeline sql.NullString
	var validation, reviewDomains string
	if err := row.Scan(&batch.ID, &batch.Title, &branch, &worktree, &validation, &reviewDomains, &rollback, &split, &expectedCI, &deployRun, &pipeline, &batch.CreatedAt, &batch.UpdatedAt); err != nil {
		return WorkBatch{}, err
	}
	batch.Branch = branch.String
	batch.WorktreePath = worktree.String
	batch.RollbackCriteria = rollback.String
	batch.SplitCriteria = split.String
	batch.ExpectedCI = expectedCI.String
	batch.DeployRunID = deployRun.String
	batch.PipelineID = pipeline.String
	_ = json.Unmarshal([]byte(validation), &batch.ValidationCommands)
	_ = json.Unmarshal([]byte(reviewDomains), &batch.ReviewDomains)
	return batch, nil
}

func (s *Store) workBatchTasks(ctx context.Context, batchID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT task_id FROM work_batch_tasks WHERE project_id=? AND batch_id=? ORDER BY task_id`, s.projectID, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []string
	for rows.Next() {
		var taskID string
		if err := rows.Scan(&taskID); err != nil {
			return nil, err
		}
		tasks = append(tasks, taskID)
	}
	return tasks, rows.Err()
}

func (s *Store) workBatchEvidence(ctx context.Context, batchID string) ([]WorkBatchEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, batch_id, COALESCE(command_text, ''), COALESCE(result, ''), COALESCE(artifact_path, ''), COALESCE(artifact_type, ''), COALESCE(notes, ''), created_at FROM work_batch_evidence WHERE project_id=? AND batch_id=? ORDER BY created_at, id`, s.projectID, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var evidence []WorkBatchEvidence
	for rows.Next() {
		var ev WorkBatchEvidence
		if err := rows.Scan(&ev.ID, &ev.BatchID, &ev.CommandText, &ev.Result, &ev.ArtifactPath, &ev.ArtifactType, &ev.Notes, &ev.CreatedAt); err != nil {
			return nil, err
		}
		evidence = append(evidence, ev)
	}
	return evidence, rows.Err()
}

func nullableIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func sqlPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func (s *Store) evidence(ctx context.Context, taskID string) ([]Evidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(command_text, ''), COALESCE(result, ''), COALESCE(artifact_path, ''), COALESCE(artifact_type, ''), duration_seconds, COALESCE(notes, ''), created_at FROM task_evidence WHERE project_id=? AND task_id=? ORDER BY created_at`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Evidence
	for rows.Next() {
		var ev Evidence
		var dur sql.NullInt64
		if err := rows.Scan(&ev.CommandText, &ev.Result, &ev.ArtifactPath, &ev.ArtifactType, &dur, &ev.Notes, &ev.CreatedAt); err != nil {
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
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(from_role, ''), to_role, COALESCE(payload, ''), COALESCE(acknowledged_at, ''), created_at FROM task_handoffs WHERE project_id=? AND task_id=? ORDER BY created_at`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Handoff
	for rows.Next() {
		var h Handoff
		if err := rows.Scan(&h.ID, &h.FromRole, &h.ToRole, &h.Payload, &h.AcknowledgedAt, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) reviews(ctx context.Context, taskID string) ([]Review, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT reviewer, COALESCE(review_domain, ''), verdict, notes, reviewed_commit_sha, created_at FROM task_reviews WHERE project_id=? AND task_id=? ORDER BY created_at`, s.projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.Reviewer, &r.Domain, &r.Verdict, &r.Reason, &r.Commit, &r.CreatedAt); err != nil {
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
