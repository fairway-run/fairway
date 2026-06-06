package reconcile

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/store"
)

type ActiveOptions struct {
	Terminal             []string
	StaleCheckpointAfter time.Duration
}

type ActiveReport struct {
	OK       bool            `json:"ok"`
	Findings []ActiveFinding `json:"findings"`
	Summary  ActiveSummary   `json:"summary"`
}

type ActiveSummary struct {
	StaleSessions             int `json:"stale_sessions"`
	UnattendedInProgress      int `json:"unattended_in_progress"`
	StatusDecisionRequired    int `json:"status_decision_required"`
	ActiveParentWithoutRollup int `json:"active_parent_without_rollup"`
	StaleCheckpoints          int `json:"stale_checkpoints"`
	MonitorSessionsNoProof    int `json:"monitor_sessions_no_proof"`
}

type ActiveFinding struct {
	Kind                 string `json:"kind"`
	Severity             string `json:"severity"`
	Action               string `json:"action"`
	Reason               string `json:"reason"`
	SessionID            string `json:"session_id,omitempty"`
	Provider             string `json:"provider,omitempty"`
	Backend              string `json:"backend,omitempty"`
	Role                 string `json:"role,omitempty"`
	TaskID               string `json:"task_id,omitempty"`
	TaskStatus           string `json:"task_status,omitempty"`
	LatestEvidenceResult string `json:"latest_evidence_result,omitempty"`
	LatestEvidenceAt     string `json:"latest_evidence_at,omitempty"`
	LatestCheckpoint     string `json:"latest_checkpoint,omitempty"`
	LatestCheckpointAt   string `json:"latest_checkpoint_at,omitempty"`
}

func Active(ctx context.Context, s *store.Store, opts ActiveOptions) (ActiveReport, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return ActiveReport{}, err
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		return ActiveReport{}, err
	}
	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return ActiveReport{}, err
	}

	taskByID := make(map[string]store.Task, len(tasks))
	children := map[string]int{}
	for _, task := range tasks {
		taskByID[task.Definition.ID] = task
		if task.Definition.ParentID != "" {
			children[task.Definition.ParentID]++
		}
	}
	activeSessionByTask := map[string]bool{}
	for _, session := range sessions {
		if session.TaskID != "" {
			activeSessionByTask[session.TaskID] = true
		}
	}
	latestCheckpoint := map[string]store.Checkpoint{}
	for _, checkpoint := range checkpoints {
		if _, ok := latestCheckpoint[checkpoint.TaskID]; !ok {
			latestCheckpoint[checkpoint.TaskID] = checkpoint
		}
	}

	var findings []ActiveFinding
	for _, session := range sessions {
		if session.TaskID == "" {
			continue
		}
		task, ok := taskByID[session.TaskID]
		if !ok {
			findings = append(findings, ActiveFinding{
				Kind:      "running_session_missing_task",
				Severity:  "warning",
				Action:    "mark_session_stale",
				Reason:    "running session is attached to a task that no longer exists",
				SessionID: session.ID,
				Provider:  session.Provider,
				Backend:   session.SessionBackend,
				Role:      session.Role,
				TaskID:    session.TaskID,
			})
			continue
		}
		if isTerminal(task.Status, opts.Terminal) {
			findings = append(findings, ActiveFinding{
				Kind:       "running_session_terminal_task",
				Severity:   "warning",
				Action:     "mark_session_stale",
				Reason:     "running session is attached to a terminal task",
				SessionID:  session.ID,
				Provider:   session.Provider,
				Backend:    session.SessionBackend,
				Role:       session.Role,
				TaskID:     session.TaskID,
				TaskStatus: task.Status,
			})
		}
		if isMonitorSession(session) && !monitorSessionHasBackingProof(session, latestCheckpoint[session.TaskID], opts.StaleCheckpointAfter) {
			findings = append(findings, ActiveFinding{
				Kind:       "monitor_session_without_backing_proof",
				Severity:   "warning",
				Action:     "mark_session_stale",
				Reason:     "monitor session has no backing automation, process proof, external polling proof, or fresh bounded manual checkpoint",
				SessionID:  session.ID,
				Provider:   session.Provider,
				Backend:    session.SessionBackend,
				Role:       session.Role,
				TaskID:     session.TaskID,
				TaskStatus: task.Status,
			})
		}
	}

	for _, task := range tasks {
		if task.Status != "in_progress" {
			continue
		}
		_, _, evidence, _, _, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return ActiveReport{}, err
		}
		latestEvidence := latestEvidence(evidence)
		checkpoint := latestCheckpoint[task.Definition.ID]

		if !activeSessionByTask[task.Definition.ID] {
			findings = append(findings, findingForTask("unattended_in_progress", "warning", "record_checkpoint_or_reset_status", "in_progress task has no running provider session", task, latestEvidence, checkpoint))
		}
		if latestEvidence.Result != "" && timeAfter(latestEvidence.CreatedAt, task.UpdatedAt) {
			findings = append(findings, findingForTask("status_decision_required", "warning", "set_done_blocked_todo_or_followup", "evidence was recorded after the task became active; task still needs an explicit status decision", task, latestEvidence, checkpoint))
		}
		if children[task.Definition.ID] > 0 && !hasRecentRollup(task, latestEvidence, checkpoint) {
			findings = append(findings, findingForTask("active_parent_without_rollup", "warning", "move_execution_to_children_or_record_rollup", "parent task is in_progress without fresh direct rollup evidence or checkpoint", task, latestEvidence, checkpoint))
		}
		if opts.StaleCheckpointAfter > 0 && checkpoint.CreatedAt != "" && checkpoint.State == "active" && checkpointOlderThan(checkpoint.CreatedAt, opts.StaleCheckpointAfter) {
			findings = append(findings, findingForTask("stale_checkpoint", "warning", "refresh_checkpoint_or_reset_status", "latest active checkpoint is stale", task, latestEvidence, checkpoint))
		}
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		if findings[i].TaskID != findings[j].TaskID {
			return findings[i].TaskID < findings[j].TaskID
		}
		return findings[i].SessionID < findings[j].SessionID
	})
	report := ActiveReport{OK: len(findings) == 0, Findings: findings}
	for _, finding := range findings {
		switch finding.Kind {
		case "running_session_terminal_task", "running_session_missing_task":
			report.Summary.StaleSessions++
		case "monitor_session_without_backing_proof":
			report.Summary.MonitorSessionsNoProof++
		case "unattended_in_progress":
			report.Summary.UnattendedInProgress++
		case "status_decision_required":
			report.Summary.StatusDecisionRequired++
		case "active_parent_without_rollup":
			report.Summary.ActiveParentWithoutRollup++
		case "stale_checkpoint":
			report.Summary.StaleCheckpoints++
		}
	}
	return report, nil
}

func isMonitorSession(session store.Session) bool {
	if strings.TrimSpace(session.MonitorKind) != "" {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		session.Lane,
		session.SessionBackend,
		session.Provider,
		session.SessionName,
	}, " "))
	for _, keyword := range []string{"watch", "monitor", "ci", "deploy", "smoke", "uat"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func monitorSessionHasBackingProof(session store.Session, checkpoint store.Checkpoint, staleCheckpointAfter time.Duration) bool {
	if session.PID != nil || strings.TrimSpace(session.TmuxPane) != "" || strings.TrimSpace(session.AutomationID) != "" {
		return true
	}
	if strings.TrimSpace(session.ExternalRunID) != "" && strings.TrimSpace(session.PollCommand) != "" {
		return true
	}
	if manualWindowOpen(session.ManualUntil) {
		return true
	}
	return freshBoundedCheckpoint(checkpoint, staleCheckpointAfter)
}

func freshBoundedCheckpoint(checkpoint store.Checkpoint, staleCheckpointAfter time.Duration) bool {
	switch checkpoint.State {
	case "active", "awaiting_input":
	default:
		return false
	}
	if !manualWindowOpen(checkpoint.TargetCloseBy) {
		return false
	}
	if staleCheckpointAfter > 0 && checkpoint.CreatedAt != "" && checkpointOlderThan(checkpoint.CreatedAt, staleCheckpointAfter) {
		return false
	}
	return true
}

func manualWindowOpen(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	now := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return !parsed.Before(now)
	}
	if parsed, err := time.Parse("2006-01-02", raw); err == nil {
		endOfDay := parsed.Add(24*time.Hour - time.Nanosecond)
		return !endOfDay.Before(now)
	}
	return false
}

func findingForTask(kind, severity, action, reason string, task store.Task, evidence store.Evidence, checkpoint store.Checkpoint) ActiveFinding {
	return ActiveFinding{
		Kind:                 kind,
		Severity:             severity,
		Action:               action,
		Reason:               reason,
		Role:                 task.Definition.Role,
		TaskID:               task.Definition.ID,
		TaskStatus:           task.Status,
		LatestEvidenceResult: evidence.Result,
		LatestEvidenceAt:     evidence.CreatedAt,
		LatestCheckpoint:     checkpoint.State,
		LatestCheckpointAt:   checkpoint.CreatedAt,
	}
}

func latestEvidence(evidence []store.Evidence) store.Evidence {
	if len(evidence) == 0 {
		return store.Evidence{}
	}
	latest := evidence[0]
	for _, item := range evidence[1:] {
		if item.CreatedAt > latest.CreatedAt {
			latest = item
		}
	}
	return latest
}

func hasRecentRollup(task store.Task, evidence store.Evidence, checkpoint store.Checkpoint) bool {
	return timeAfter(evidence.CreatedAt, task.UpdatedAt) || timeAfter(checkpoint.CreatedAt, task.UpdatedAt)
}

func timeAfter(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftTime, err := time.Parse(time.RFC3339Nano, left)
	if err != nil {
		return left > right
	}
	rightTime, err := time.Parse(time.RFC3339Nano, right)
	if err != nil {
		return left > right
	}
	return leftTime.After(rightTime)
}

func checkpointOlderThan(createdAt string, age time.Duration) bool {
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return false
	}
	return time.Since(created) > age
}

func isTerminal(status string, terminal []string) bool {
	for _, value := range terminal {
		if status == value {
			return true
		}
	}
	return false
}
