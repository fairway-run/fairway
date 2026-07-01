package deliveryreport

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/reviewstate"
	"github.com/subashram/fairway/internal/store"
)

type Options struct {
	Since   time.Duration
	Profile string
	Now     time.Time
}

type Report struct {
	OK             bool               `json:"ok"`
	Since          string             `json:"since"`
	Profile        string             `json:"profile,omitempty"`
	Summary        Summary            `json:"summary"`
	Overhead       Overhead           `json:"overhead"`
	OutcomeSources []OutcomeSource    `json:"outcome_sources,omitempty"`
	Loops          []LoopSummary      `json:"loops,omitempty"`
	Rehearsals     []RehearsalFailure `json:"rehearsals,omitempty"`
	Batches        []BatchDeliveryRow `json:"batches,omitempty"`
	Rows           []TaskDeliveryRow  `json:"rows,omitempty"`
}

type Summary struct {
	CompletedTasks                  int `json:"completed_tasks"`
	BlockedOpened                   int `json:"blocked_opened"`
	BlockedResolved                 int `json:"blocked_resolved"`
	TotalBlockedSeconds             int `json:"total_blocked_seconds"`
	TotalReviewWaitSeconds          int `json:"total_review_wait_seconds"`
	FirstEvidenceToDoneSeconds      int `json:"first_evidence_to_done_seconds"`
	DoneToMergeReadySecondsObserved int `json:"done_to_merge_ready_seconds_observed"`
	ApprovalLoops                   int `json:"approval_loops"`
	ReopenRetryCount                int `json:"reopen_retry_count"`
}

type Overhead struct {
	ReviewRecords              int     `json:"review_records"`
	ReviewApprovals            int     `json:"review_approvals"`
	ReviewChangesRequested     int     `json:"review_changes_requested"`
	SameLaneReviewMappings     int     `json:"same_lane_review_mappings"`
	Notifications              int     `json:"notifications"`
	NotificationFailures       int     `json:"notification_failures"`
	Wakes                      int     `json:"wakes"`
	Handoffs                   int     `json:"handoffs"`
	ApprovalLoops              int     `json:"approval_loops"`
	ReopenRetryCount           int     `json:"reopen_retry_count"`
	ReviewWaitsNoChanges       int     `json:"review_waits_no_changes"`
	ReviewUsefulnessRatio      float64 `json:"review_usefulness_ratio"`
	TasksWithProcessOverhead   int     `json:"tasks_with_process_overhead"`
	TasksWithEngineeringOutput int     `json:"tasks_with_engineering_output"`
}

type OutcomeSource struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

type LoopSummary struct {
	TaskID      string `json:"task_id"`
	Signal      string `json:"signal"`
	Count       int    `json:"count"`
	Recommended string `json:"recommended_action"`
}

type RehearsalFailure struct {
	PacketID string   `json:"packet_id"`
	CheckID  string   `json:"check_id"`
	Count    int      `json:"count"`
	TaskIDs  []string `json:"task_ids"`
}

type TaskDeliveryRow struct {
	TaskID                    string `json:"task_id"`
	Title                     string `json:"title"`
	Profile                   string `json:"profile,omitempty"`
	Status                    string `json:"status"`
	CompletedAt               string `json:"completed_at,omitempty"`
	BlockedSeconds            int    `json:"blocked_seconds"`
	ReviewWaitSeconds         int    `json:"review_wait_seconds"`
	FirstEvidenceToDone       int    `json:"first_evidence_to_done_seconds"`
	ReviewRecords             int    `json:"review_records"`
	ReviewChangesRequested    int    `json:"review_changes_requested"`
	Notifications             int    `json:"notifications"`
	Handoffs                  int    `json:"handoffs"`
	ApprovalLoops             int    `json:"approval_loops"`
	ReopenRetryCount          int    `json:"reopen_retry_count"`
	OutcomeSource             string `json:"outcome_source,omitempty"`
	DefectSource              string `json:"defect_source,omitempty"`
	LoopSignal                string `json:"loop_signal,omitempty"`
	RecommendedAction         string `json:"recommended_action,omitempty"`
	MergeReadyTimingAvailable bool   `json:"merge_ready_timing_available"`
}

type BatchDeliveryRow struct {
	BatchID           string          `json:"batch_id"`
	Title             string          `json:"title"`
	Tasks             int             `json:"tasks"`
	CompletedTasks    int             `json:"completed_tasks"`
	BlockedSeconds    int             `json:"blocked_seconds"`
	ReviewWaitSeconds int             `json:"review_wait_seconds"`
	ReviewRecords     int             `json:"review_records"`
	ApprovalLoops     int             `json:"approval_loops"`
	ReopenRetryCount  int             `json:"reopen_retry_count"`
	Notifications     int             `json:"notifications"`
	Handoffs          int             `json:"handoffs"`
	OutcomeSources    []OutcomeSource `json:"outcome_sources,omitempty"`
}

func Build(ctx context.Context, cfg config.Config, s *store.Store, opts Options) (Report, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if opts.Since <= 0 {
		opts.Since = 7 * 24 * time.Hour
	}
	start := now.Add(-opts.Since)
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return Report{}, err
	}
	ackTimeout, _ := time.ParseDuration(strings.TrimSpace(cfg.Coordinator.NotificationAckTimeout))
	report := Report{OK: true, Since: opts.Since.String(), Profile: opts.Profile}
	outcomes := map[string]int{}
	rehearsals := map[string]RehearsalFailure{}
	for _, task := range tasks {
		if opts.Profile != "" && task.Definition.Profile != opts.Profile {
			continue
		}
		detail, transitions, evidence, handoffs, reviews, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return Report{}, err
		}
		notifications, err := s.Notifications(ctx, task.Definition.ID)
		if err != nil {
			return Report{}, err
		}
		row := taskRow(cfg, detail, transitions, evidence, handoffs, reviews, notifications, ackTimeout, start, now)
		if !rowInWindow(row, transitions, evidence, reviews, notifications, handoffs, start) {
			continue
		}
		report.Rows = append(report.Rows, row)
		addSummary(&report.Summary, row)
		addOverhead(&report.Overhead, detail, evidence, reviews, notifications, handoffs)
		report.Overhead.ReopenRetryCount += row.ReopenRetryCount
		if row.OutcomeSource != "" {
			outcomes[row.OutcomeSource]++
		}
		if row.LoopSignal != "" {
			report.Loops = append(report.Loops, LoopSummary{
				TaskID:      row.TaskID,
				Signal:      row.LoopSignal,
				Count:       row.ReviewChangesRequested,
				Recommended: row.RecommendedAction,
			})
		}
		addRehearsalFailures(rehearsals, detail.Definition.ID, evidence)
	}
	report.OutcomeSources = outcomeSources(outcomes)
	report.Rehearsals = rehearsalFailures(rehearsals)
	batches, err := s.WorkBatches(ctx)
	if err != nil {
		return Report{}, err
	}
	report.Batches = batchRows(batches, report.Rows)
	sort.SliceStable(report.Rows, func(i, j int) bool {
		if report.Rows[i].CompletedAt != report.Rows[j].CompletedAt {
			return report.Rows[i].CompletedAt > report.Rows[j].CompletedAt
		}
		return report.Rows[i].TaskID < report.Rows[j].TaskID
	})
	if report.Overhead.ReviewRecords > 0 {
		report.Overhead.ReviewUsefulnessRatio = float64(report.Overhead.ReviewChangesRequested) / float64(report.Overhead.ReviewRecords)
	}
	return report, nil
}

func addRehearsalFailures(rows map[string]RehearsalFailure, taskID string, evidence []store.Evidence) {
	for _, ev := range evidence {
		if ev.Result != "fail" && ev.Result != "blocked" && ev.Result != "partial" {
			continue
		}
		packetID, checkID := rehearsalPacketCheck(ev)
		if packetID == "" || checkID == "" {
			continue
		}
		key := packetID + "\x00" + checkID
		row := rows[key]
		if row.PacketID == "" {
			row.PacketID = packetID
			row.CheckID = checkID
		}
		row.Count++
		if !stringSliceContains(row.TaskIDs, taskID) {
			row.TaskIDs = append(row.TaskIDs, taskID)
			sort.Strings(row.TaskIDs)
		}
		rows[key] = row
	}
}

func rehearsalPacketCheck(ev store.Evidence) (string, string) {
	packetID := noteValue(ev.Notes, "packet")
	checkID := noteValue(ev.Notes, "check")
	if packetID == "" && strings.HasPrefix(ev.ArtifactType, "environment-") {
		packetID = "environment-deploy-preflight"
	}
	if checkID == "" {
		checkID = strings.TrimPrefix(ev.ArtifactType, "environment-")
	}
	if checkID == "" {
		checkID = noteValue(ev.CommandText, "check")
	}
	return packetID, checkID
}

func noteValue(text, key string) string {
	for _, field := range strings.Fields(text) {
		rawKey, value, ok := strings.Cut(field, "=")
		if ok && strings.TrimSpace(rawKey) == key {
			return strings.Trim(strings.TrimSpace(value), ",;")
		}
	}
	return ""
}

func rehearsalFailures(rows map[string]RehearsalFailure) []RehearsalFailure {
	out := make([]RehearsalFailure, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PacketID != out[j].PacketID {
			return out[i].PacketID < out[j].PacketID
		}
		return out[i].CheckID < out[j].CheckID
	})
	return out
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func taskRow(cfg config.Config, task store.Task, transitions []store.Transition, evidence []store.Evidence, handoffs []store.Handoff, reviews []store.Review, notifications []store.Notification, ackTimeout time.Duration, start, now time.Time) TaskDeliveryRow {
	row := TaskDeliveryRow{
		TaskID:        task.Definition.ID,
		Title:         task.Definition.Title,
		Profile:       task.Definition.Profile,
		Status:        task.Status,
		CompletedAt:   task.CompletedAt,
		ReviewRecords: len(reviews),
		Notifications: len(notifications),
		Handoffs:      len(handoffs),
		OutcomeSource: outcomeSource(evidence, reviews),
	}
	row.DefectSource = defectSource(evidence, reviews)
	for _, review := range reviews {
		if review.Verdict == "changes" || review.Verdict == "reject" {
			row.ReviewChangesRequested++
		}
	}
	row.ApprovalLoops = approvalLoopCount(reviews)
	row.ReopenRetryCount = reopenRetryCount(transitions)
	row.BlockedSeconds = blockedSeconds(transitions, now)
	row.FirstEvidenceToDone = firstEvidenceToDoneSeconds(task, evidence)
	if ackTimeout > 0 {
		waits := reviewstate.WaitsForTask(task, handoffs, reviews, notifications, reviewstate.ReviewWaitOptions{
			ProviderTargets: cfg.ProviderTargets,
			ReviewRoutes:    cfg.ReviewRoutes,
			Roles:           cfg.Roles,
			AckTimeout:      ackTimeout,
			Now:             now,
			Terminal:        cfg.States.Terminal,
		})
		for _, wait := range waits {
			if wait.Blocking && wait.State != "resolved" && wait.State != "cancelled" {
				row.ReviewWaitSeconds += int(ackTimeout.Seconds())
			}
		}
	}
	row.LoopSignal, row.RecommendedAction = loopSignal(evidence, reviews)
	return row
}

func rowInWindow(row TaskDeliveryRow, transitions []store.Transition, evidence []store.Evidence, reviews []store.Review, notifications []store.Notification, handoffs []store.Handoff, start time.Time) bool {
	for _, value := range []string{row.CompletedAt} {
		if at, ok := parseTime(value); ok && !at.Before(start) {
			return true
		}
	}
	for _, transition := range transitions {
		if at, ok := parseTime(transition.At); ok && !at.Before(start) {
			return true
		}
	}
	for _, ev := range evidence {
		if at, ok := parseTime(ev.CreatedAt); ok && !at.Before(start) {
			return true
		}
	}
	for _, review := range reviews {
		if at, ok := parseTime(review.CreatedAt); ok && !at.Before(start) {
			return true
		}
	}
	for _, notification := range notifications {
		if at, ok := parseTime(notification.CreatedAt); ok && !at.Before(start) {
			return true
		}
	}
	for _, handoff := range handoffs {
		if at, ok := parseTime(handoff.CreatedAt); ok && !at.Before(start) {
			return true
		}
	}
	return false
}

func addSummary(summary *Summary, row TaskDeliveryRow) {
	if row.Status == "done" && row.CompletedAt != "" {
		summary.CompletedTasks++
	}
	if row.BlockedSeconds > 0 {
		summary.BlockedOpened++
		summary.TotalBlockedSeconds += row.BlockedSeconds
		if row.Status != "blocked" {
			summary.BlockedResolved++
		}
	}
	summary.TotalReviewWaitSeconds += row.ReviewWaitSeconds
	summary.FirstEvidenceToDoneSeconds += row.FirstEvidenceToDone
	summary.ApprovalLoops += row.ApprovalLoops
	summary.ReopenRetryCount += row.ReopenRetryCount
}

func addOverhead(overhead *Overhead, task store.Task, evidence []store.Evidence, reviews []store.Review, notifications []store.Notification, handoffs []store.Handoff) {
	overhead.ReviewRecords += len(reviews)
	overhead.Notifications += len(notifications)
	overhead.Handoffs += len(handoffs)
	overhead.ApprovalLoops += approvalLoopCount(reviews)
	if len(reviews) > 0 || len(notifications) > 0 || len(handoffs) > 0 {
		overhead.TasksWithProcessOverhead++
	}
	if len(evidence) > 0 {
		overhead.TasksWithEngineeringOutput++
	}
	for _, review := range reviews {
		switch review.Verdict {
		case "approve":
			overhead.ReviewApprovals++
			overhead.ReviewWaitsNoChanges++
		case "changes", "reject":
			overhead.ReviewChangesRequested++
		}
		if review.Domain != "" && review.Domain == task.Definition.Role {
			overhead.SameLaneReviewMappings++
		}
	}
	for _, notification := range notifications {
		switch notification.State {
		case "notification_failed", "failed":
			overhead.NotificationFailures++
		case "thread_steered", "notification_delivered", "sent":
			overhead.Wakes++
		}
	}
}

func approvalLoopCount(reviews []store.Review) int {
	loops := 0
	for _, review := range reviews {
		if review.Verdict == "changes" || review.Verdict == "reject" {
			loops++
		}
	}
	return loops
}

func reopenRetryCount(transitions []store.Transition) int {
	count := 0
	for _, transition := range transitions {
		fromTerminal := transition.FromStatus == "done" || transition.FromStatus == "blocked" || transition.FromStatus == "cancelled"
		toActive := transition.ToStatus == "todo" || transition.ToStatus == "in_progress" || transition.ToStatus == "review"
		reason := strings.ToLower(transition.Reason)
		if toActive && (strings.Contains(reason, "reopen") || strings.Contains(reason, "retry") || strings.Contains(reason, "rerun")) {
			count++
			continue
		}
		if fromTerminal && toActive {
			count++
		}
	}
	return count
}

func batchRows(batches []store.WorkBatch, rows []TaskDeliveryRow) []BatchDeliveryRow {
	byTask := make(map[string]TaskDeliveryRow, len(rows))
	for _, row := range rows {
		byTask[row.TaskID] = row
	}
	out := make([]BatchDeliveryRow, 0, len(batches))
	for _, batch := range batches {
		row := BatchDeliveryRow{BatchID: batch.ID, Title: batch.Title}
		outcomes := map[string]int{}
		for _, taskID := range batch.Tasks {
			taskRow, ok := byTask[taskID]
			if !ok {
				continue
			}
			row.Tasks++
			if taskRow.Status == "done" && taskRow.CompletedAt != "" {
				row.CompletedTasks++
			}
			row.BlockedSeconds += taskRow.BlockedSeconds
			row.ReviewWaitSeconds += taskRow.ReviewWaitSeconds
			row.ReviewRecords += taskRow.ReviewRecords
			row.ApprovalLoops += taskRow.ApprovalLoops
			row.ReopenRetryCount += taskRow.ReopenRetryCount
			row.Notifications += taskRow.Notifications
			row.Handoffs += taskRow.Handoffs
			if taskRow.OutcomeSource != "" {
				outcomes[taskRow.OutcomeSource]++
			}
		}
		if row.Tasks == 0 {
			continue
		}
		row.OutcomeSources = outcomeSources(outcomes)
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CompletedTasks != out[j].CompletedTasks {
			return out[i].CompletedTasks > out[j].CompletedTasks
		}
		return out[i].BatchID < out[j].BatchID
	})
	return out
}

func blockedSeconds(transitions []store.Transition, now time.Time) int {
	var total time.Duration
	var opened time.Time
	for _, transition := range transitions {
		at, ok := parseTime(transition.At)
		if !ok {
			continue
		}
		if transition.ToStatus == "blocked" && opened.IsZero() {
			opened = at
			continue
		}
		if !opened.IsZero() && transition.FromStatus == "blocked" && transition.ToStatus != "blocked" {
			total += at.Sub(opened)
			opened = time.Time{}
		}
	}
	if !opened.IsZero() {
		total += now.Sub(opened)
	}
	if total < 0 {
		return 0
	}
	return int(total.Seconds())
}

func firstEvidenceToDoneSeconds(task store.Task, evidence []store.Evidence) int {
	doneAt, ok := parseTime(task.CompletedAt)
	if !ok {
		return 0
	}
	var first time.Time
	for _, ev := range evidence {
		at, ok := parseTime(ev.CreatedAt)
		if !ok {
			continue
		}
		if first.IsZero() || at.Before(first) {
			first = at
		}
	}
	if first.IsZero() || doneAt.Before(first) {
		return 0
	}
	return int(doneAt.Sub(first).Seconds())
}

func outcomeSource(evidence []store.Evidence, reviews []store.Review) string {
	for _, review := range reviews {
		if review.Verdict == "changes" || review.Verdict == "reject" {
			return "review"
		}
	}
	return sourceFromText(evidenceText(evidence), len(evidence) > 0)
}

func defectSource(evidence []store.Evidence, reviews []store.Review) string {
	for _, review := range reviews {
		if review.Verdict == "changes" || review.Verdict == "reject" {
			return "review"
		}
	}
	issueEvidence := make([]store.Evidence, 0, len(evidence))
	for _, ev := range evidence {
		switch strings.ToLower(strings.TrimSpace(ev.Result)) {
		case "fail", "blocked", "partial":
			issueEvidence = append(issueEvidence, ev)
		}
	}
	if len(issueEvidence) == 0 {
		return ""
	}
	return sourceFromText(evidenceText(issueEvidence), true)
}

func sourceFromText(raw string, hasEvidence bool) string {
	text := strings.ToLower(raw)
	switch {
	case strings.Contains(text, "live"):
		return "live-window"
	case strings.Contains(text, "uat"):
		return "uat"
	case strings.Contains(text, "deploy"):
		return "deploy"
	case strings.Contains(text, "ci") || strings.Contains(text, "workflow"):
		return "ci"
	case strings.Contains(text, "preflight"):
		return "preflight"
	case strings.Contains(text, "test") || strings.Contains(text, "go test"):
		return "tests"
	case strings.Contains(text, "manual"):
		return "manual"
	default:
		if hasEvidence {
			return "evidence"
		}
		return ""
	}
}

func loopSignal(evidence []store.Evidence, reviews []store.Review) (string, string) {
	failures := 0
	for _, ev := range evidence {
		if ev.Result == "fail" || ev.Result == "blocked" {
			failures++
		}
	}
	changes := 0
	for _, review := range reviews {
		if review.Verdict == "changes" || review.Verdict == "reject" {
			changes++
		}
	}
	switch {
	case failures >= 2 && changes > 0:
		return "repeated_failures_after_review", "consider causal reset with failure chain and proof-before-retry"
	case failures >= 3:
		return "repeated_meaningful_failures", "pause reruns and capture real unknowns before retry"
	case changes >= 2:
		return "repeated_review_changes", "group next review around defect class and expected risk-control value"
	default:
		return "", ""
	}
}

func evidenceText(evidence []store.Evidence) string {
	parts := make([]string, 0, len(evidence)*4)
	for _, ev := range evidence {
		parts = append(parts, ev.CommandText, ev.Result, ev.ArtifactPath, ev.ArtifactType, ev.Notes)
	}
	return strings.Join(parts, " ")
}

func outcomeSources(counts map[string]int) []OutcomeSource {
	var out []OutcomeSource
	for source, count := range counts {
		out = append(out, OutcomeSource{Source: source, Count: count})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func parseTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
