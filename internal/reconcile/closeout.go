package reconcile

import (
	"context"
	"errors"
	"strings"

	"github.com/subashram/fairway/internal/reviewstate"
	"github.com/subashram/fairway/internal/store"
)

type CloseoutOptions struct {
	TaskID         string
	Role           string
	Terminal       []string
	Git            CloseoutGit
	PreserveReason string
}

type CloseoutGit struct {
	Branch               string   `json:"branch,omitempty"`
	Base                 string   `json:"base,omitempty"`
	WorktreePath         string   `json:"worktree_path,omitempty"`
	WorktreeDirty        bool     `json:"worktree_dirty,omitempty"`
	AllowedArtifactPaths []string `json:"allowed_artifact_paths,omitempty"`
	BranchExists         bool     `json:"branch_exists,omitempty"`
	BranchMerged         bool     `json:"branch_merged,omitempty"`
	RemoteBranchExists   bool     `json:"remote_branch_exists,omitempty"`
}

type CloseoutApplyPlan struct {
	DeleteRemoteBranch bool   `json:"delete_remote_branch,omitempty"`
	Remote             string `json:"remote,omitempty"`
	Branch             string `json:"branch,omitempty"`
}

type CloseoutReport struct {
	OK       bool              `json:"ok"`
	TaskID   string            `json:"task_id,omitempty"`
	Role     string            `json:"role,omitempty"`
	Branch   string            `json:"branch,omitempty"`
	Worktree string            `json:"worktree,omitempty"`
	Commit   string            `json:"commit,omitempty"`
	Summary  CloseoutSummary   `json:"summary"`
	Findings []CloseoutFinding `json:"findings"`
	Apply    CloseoutApplyPlan `json:"apply,omitempty"`
}

type CloseoutSummary struct {
	Blockers               int `json:"blockers"`
	Warnings               int `json:"warnings"`
	SafeToDeleteBranches   int `json:"safe_to_delete_branches"`
	PreservedBranches      int `json:"preserved_branches"`
	ActiveSessions         int `json:"active_sessions"`
	ActiveWatchers         int `json:"active_watchers"`
	MissingReviewDomains   int `json:"missing_review_domains"`
	DirtyWorktrees         int `json:"dirty_worktrees"`
	UnmergedBranches       int `json:"unmerged_branches"`
	RemoteBranchesPresent  int `json:"remote_branches_present"`
	RemoteBranchesNoIntent int `json:"remote_branches_without_intent"`
	MissingCommits         int `json:"missing_commits"`
	VerificationEvidence   int `json:"verification_evidence"`
	PendingVerification    int `json:"pending_verification"`
	ReviewNotifications    int `json:"review_notifications"`
}

type CloseoutFinding struct {
	Kind               string   `json:"kind"`
	Severity           string   `json:"severity"`
	Action             string   `json:"action"`
	Reason             string   `json:"reason"`
	TaskID             string   `json:"task_id,omitempty"`
	Role               string   `json:"role,omitempty"`
	Branch             string   `json:"branch,omitempty"`
	Path               string   `json:"path,omitempty"`
	Worktree           string   `json:"worktree,omitempty"`
	SessionID          string   `json:"session_id,omitempty"`
	WatcherID          string   `json:"watcher_id,omitempty"`
	Commit             string   `json:"commit,omitempty"`
	EvidenceType       string   `json:"evidence_type,omitempty"`
	PushIntent         string   `json:"push_intent,omitempty"`
	MissingDomains     []string `json:"missing_domains,omitempty"`
	Domain             string   `json:"domain,omitempty"`
	NotificationStatus string   `json:"notification_status,omitempty"`
}

func Closeout(ctx context.Context, s *store.Store, opts CloseoutOptions) (CloseoutReport, error) {
	task, _, evidence, handoffs, reviews, err := s.TaskDetail(ctx, opts.TaskID)
	if err != nil {
		return CloseoutReport{}, err
	}
	notifications, err := s.Notifications(ctx, opts.TaskID)
	if err != nil {
		return CloseoutReport{}, err
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		return CloseoutReport{}, err
	}
	watchers, err := s.Watchers(ctx, false)
	if err != nil {
		return CloseoutReport{}, err
	}
	memory, memoryErr := s.TrackMemory(ctx, opts.TaskID)
	if memoryErr != nil && !errors.Is(memoryErr, store.ErrNotFound) {
		return CloseoutReport{}, memoryErr
	}

	branch := firstNonEmpty(opts.Git.Branch, task.Branch)
	role := firstNonEmpty(opts.Role, task.Definition.Role)
	report := CloseoutReport{
		OK:       true,
		TaskID:   task.Definition.ID,
		Role:     role,
		Branch:   branch,
		Worktree: opts.Git.WorktreePath,
		Commit:   task.CommitSHA,
	}
	add := func(f CloseoutFinding) {
		if f.TaskID == "" {
			f.TaskID = task.Definition.ID
		}
		if f.Role == "" {
			f.Role = role
		}
		if f.Branch == "" {
			f.Branch = branch
		}
		if f.Worktree == "" {
			f.Worktree = opts.Git.WorktreePath
		}
		report.Findings = append(report.Findings, f)
	}

	if !isTerminal(task.Status, opts.Terminal) {
		add(CloseoutFinding{Kind: "task_not_terminal", Severity: "blocker", Action: "finish_or_reopen_task", Reason: "lane closeout requires a terminal task status"})
	} else if memoryErr == nil {
		switch memory.Disposition {
		case "active":
			add(CloseoutFinding{
				Kind:     "terminal_task_active_memory",
				Severity: "warning",
				Action:   "promote_or_archive_memory",
				Reason:   "task is terminal while same-id working memory remains active; its objective and next actions are historical until disposition is recorded",
			})
		case "promote":
			add(CloseoutFinding{
				Kind:     "terminal_task_memory_promotion_pending",
				Severity: "warning",
				Action:   "complete_memory_promotion",
				Reason:   "task is terminal while same-id working memory still has pending canonical promotion",
			})
		}
	}
	if len(evidence) == 0 {
		add(CloseoutFinding{Kind: "missing_evidence", Severity: "warning", Action: "record_evidence_or_exception", Reason: "task has no recorded evidence for closeout"})
	}
	if strings.TrimSpace(task.CommitSHA) == "" {
		add(CloseoutFinding{Kind: "missing_commit_association", Severity: "blocker", Action: "record_task_commit_sha", Reason: "task has no associated commit SHA for closeout"})
	} else {
		add(CloseoutFinding{Kind: "commit_associated", Severity: "info", Action: "keep_commit_association", Reason: "task has an associated commit SHA", Commit: task.CommitSHA})
	}
	for _, ev := range evidence {
		if kind := verificationEvidenceKind(ev); kind != "" {
			add(CloseoutFinding{Kind: "verification_evidence", Severity: "info", Action: "keep_verification_evidence", Reason: "task has CI/deploy/UAT verification evidence", EvidenceType: kind})
		}
	}
	if missing := missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews); len(missing) > 0 {
		add(CloseoutFinding{Kind: "missing_review_domains", Severity: "blocker", Action: "record_required_reviews_or_waiver", Reason: "task is missing required review-domain approvals", MissingDomains: missing})
		for _, status := range reviewstate.BlockingStatuses(reviewstate.StatusesForTask(task, handoffs, reviews, notifications), missing) {
			add(CloseoutFinding{Kind: "review_notification_blocked", Severity: "blocker", Action: "deliver_or_retry_review_notification", Reason: status.Reason, Domain: status.Domain, NotificationStatus: status.Status})
		}
	}
	for _, session := range sessions {
		if session.TaskID != task.Definition.ID {
			continue
		}
		add(CloseoutFinding{Kind: "active_session", Severity: "blocker", Action: "end_or_handoff_session", Reason: "task still has an active provider/session attachment", SessionID: session.ID})
	}
	for _, watcher := range watchers {
		if watcher.TaskID != task.Definition.ID {
			continue
		}
		if kind := watcherVerificationKind(watcher); kind != "" {
			add(CloseoutFinding{Kind: "verification_pending", Severity: "blocker", Action: "finish_ci_deploy_uat_watcher", Reason: "task still has pending CI/deploy/UAT verification", WatcherID: watcher.ID, EvidenceType: kind})
		} else {
			add(CloseoutFinding{Kind: "active_watcher", Severity: "blocker", Action: "finish_or_handoff_watcher", Reason: "task still has an active watcher or monitor", WatcherID: watcher.ID})
		}
	}
	if strings.TrimSpace(branch) == "" {
		add(CloseoutFinding{Kind: "missing_branch", Severity: "warning", Action: "record_branch_or_preserve_reason", Reason: "task has no branch association for closeout"})
	} else if opts.Git.BranchExists && !opts.Git.BranchMerged {
		if strings.TrimSpace(opts.PreserveReason) != "" {
			add(CloseoutFinding{Kind: "branch_preserved", Severity: "warning", Action: "preserve_branch", Reason: "branch is unmerged and has an explicit preservation reason: " + strings.TrimSpace(opts.PreserveReason)})
		} else {
			add(CloseoutFinding{Kind: "unmerged_branch", Severity: "blocker", Action: "merge_branch_or_record_preserve_reason", Reason: "task branch is not merged into the configured base"})
		}
	}
	if opts.Git.WorktreeDirty {
		add(CloseoutFinding{Kind: "dirty_worktree", Severity: "blocker", Action: "commit_stash_or_revert_local_changes", Reason: "lane worktree has uncommitted changes"})
	}
	for _, path := range opts.Git.AllowedArtifactPaths {
		add(CloseoutFinding{Kind: "allowed_local_artifact", Severity: "info", Action: "keep_or_archive_local_artifact", Reason: "untracked local artifact path is allowed by config or recorded evidence", Path: path})
	}
	if strings.TrimSpace(branch) != "" && opts.Git.RemoteBranchExists {
		intent, ok, reason := pushIntentForBranch(evidence, branch)
		switch {
		case ok:
			add(CloseoutFinding{Kind: "remote_push_intent", Severity: "info", Action: "keep_push_intent_evidence", Reason: "remote branch has recorded push intent", PushIntent: intent})
		case reason != "":
			add(CloseoutFinding{Kind: "invalid_push_intent", Severity: "blocker", Action: "record_valid_push_intent_or_delete_remote_branch", Reason: reason})
		default:
			add(CloseoutFinding{Kind: "remote_branch_without_push_intent", Severity: "blocker", Action: "record_push_intent_or_delete_remote_branch", Reason: "remote branch exists without recorded push intent"})
		}
	}
	if strings.TrimSpace(branch) != "" && opts.Git.BranchExists && opts.Git.BranchMerged && !hasCloseoutBlocker(report.Findings) {
		add(CloseoutFinding{Kind: "safe_merged_branch", Severity: "info", Action: "eligible_for_branch_cleanup", Reason: "branch is merged into base and has no closeout blockers"})
		if opts.Git.RemoteBranchExists {
			add(CloseoutFinding{Kind: "safe_merged_remote_branch", Severity: "info", Action: "delete_remote_branch", Reason: "remote branch is merged and eligible for deletion"})
			report.Apply = CloseoutApplyPlan{DeleteRemoteBranch: true, Remote: "origin", Branch: branch}
		}
	} else if opts.Git.RemoteBranchExists {
		add(CloseoutFinding{Kind: "remote_branch_present", Severity: "warning", Action: "delete_remote_branch_after_safe_closeout_or_preserve", Reason: "remote branch still exists but is not eligible for automatic cleanup"})
	}

	for _, finding := range report.Findings {
		switch finding.Severity {
		case "blocker":
			report.Summary.Blockers++
		case "warning":
			report.Summary.Warnings++
		}
		switch finding.Kind {
		case "safe_merged_branch":
			report.Summary.SafeToDeleteBranches++
		case "safe_merged_remote_branch":
			report.Summary.SafeToDeleteBranches++
			report.Summary.RemoteBranchesPresent++
		case "branch_preserved":
			report.Summary.PreservedBranches++
		case "active_session":
			report.Summary.ActiveSessions++
		case "active_watcher":
			report.Summary.ActiveWatchers++
		case "missing_review_domains":
			report.Summary.MissingReviewDomains += len(finding.MissingDomains)
		case "dirty_worktree":
			report.Summary.DirtyWorktrees++
		case "unmerged_branch":
			report.Summary.UnmergedBranches++
		case "remote_branch_present":
			report.Summary.RemoteBranchesPresent++
		case "remote_branch_without_push_intent", "invalid_push_intent":
			report.Summary.RemoteBranchesNoIntent++
		case "missing_commit_association":
			report.Summary.MissingCommits++
		case "verification_evidence":
			report.Summary.VerificationEvidence++
		case "verification_pending":
			report.Summary.PendingVerification++
		case "review_notification_blocked":
			report.Summary.ReviewNotifications++
		}
	}
	report.OK = report.Summary.Blockers == 0
	return report, nil
}

func pushIntentForBranch(evidence []store.Evidence, branch string) (string, bool, string) {
	branch = strings.TrimSpace(branch)
	var invalidReason string
	var invalidIntent string
	for _, ev := range evidence {
		if strings.ToLower(strings.TrimSpace(ev.ArtifactType)) != "push-intent" {
			continue
		}
		fields := pushIntentFields(ev)
		recordBranch := strings.TrimSpace(fields["branch"])
		if recordBranch != "" && branch != "" && recordBranch != branch {
			continue
		}
		intent := strings.TrimSpace(fields["intent"])
		if !validPushIntent(intent) {
			invalidIntent = intent
			invalidReason = "push-intent evidence has unsupported intent"
			continue
		}
		if intent == "exception" && strings.TrimSpace(fields["reason"]) == "" {
			invalidIntent = intent
			invalidReason = "exception push intent requires a recorded reason"
			continue
		}
		return intent, true, ""
	}
	return invalidIntent, false, invalidReason
}

func pushIntentFields(ev store.Evidence) map[string]string {
	fields := map[string]string{}
	text := strings.Join([]string{ev.CommandText, ev.ArtifactPath, ev.Notes}, "\n")
	for _, token := range strings.Fields(text) {
		key, value, ok := strings.Cut(token, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch key {
		case "intent", "remote", "branch", "reason":
			if value != "" {
				fields[key] = value
			}
		}
	}
	return fields
}

func validPushIntent(intent string) bool {
	switch strings.TrimSpace(intent) {
	case "main-validation", "integration", "review", "release", "backup", "exception":
		return true
	default:
		return false
	}
}

func verificationEvidenceKind(ev store.Evidence) string {
	if kind := verificationKindFromArtifactType(ev.ArtifactType); kind != "" {
		return kind
	}
	text := strings.ToLower(strings.Join([]string{ev.CommandText, ev.ArtifactPath, ev.Notes}, " "))
	tokens := verificationTokens(text)
	switch {
	case tokenContainsAny(tokens, "uat"):
		return "uat"
	case tokenContainsAny(tokens, "deploy", "deployment", "release"):
		return "deploy"
	case tokenContainsAny(tokens, "ci", "pipeline", "workflow", "github", "actions", "gitlab"):
		return "ci"
	default:
		return ""
	}
}

func watcherVerificationKind(watcher store.Watcher) string {
	text := strings.ToLower(strings.Join([]string{watcher.Process, watcher.Command, watcher.Success, watcher.Failure, watcher.Notes}, " "))
	tokens := verificationTokens(text)
	switch {
	case tokenContainsAny(tokens, "uat"):
		return "uat"
	case tokenContainsAny(tokens, "deploy", "deployment", "release"):
		return "deploy"
	case tokenContainsAny(tokens, "ci", "pipeline", "workflow", "github", "actions", "gitlab"):
		return "ci"
	default:
		return ""
	}
}

func verificationKindFromArtifactType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "ci", "pipeline", "workflow", "github-actions", "gitlab":
		return "ci"
	case "deploy", "deployment", "release":
		return "deploy"
	case "uat":
		return "uat"
	default:
		return ""
	}
}

func verificationTokens(text string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if token != "" {
			out[token] = true
		}
	}
	return out
}

func tokenContainsAny(tokens map[string]bool, needles ...string) bool {
	for _, needle := range needles {
		if tokens[needle] {
			return true
		}
	}
	return false
}

func hasCloseoutBlocker(findings []CloseoutFinding) bool {
	for _, finding := range findings {
		if finding.Severity == "blocker" {
			return true
		}
	}
	return false
}

func missingApprovedReviewDomains(domains []string, reviews []store.Review) []string {
	if len(domains) == 0 {
		return nil
	}
	approved := map[string]bool{}
	for _, review := range reviews {
		if review.Verdict == "approve" {
			approved[firstNonEmpty(review.Domain, review.Reviewer)] = true
		}
	}
	seen := map[string]bool{}
	var missing []string
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		if !approved[domain] {
			missing = append(missing, domain)
		}
	}
	return missing
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
