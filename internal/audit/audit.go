package audit

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/store"
)

type WorkCoverageOptions struct {
	SinceRef      string
	SinceDuration time.Duration
	TaskID        string
}

type WorkCoverageReport struct {
	OK            bool                  `json:"ok"`
	SinceRef      string                `json:"since_ref,omitempty"`
	SinceDuration string                `json:"since_duration,omitempty"`
	TaskID        string                `json:"task_id,omitempty"`
	CommitCount   int                   `json:"commit_count"`
	Findings      []WorkCoverageFinding `json:"findings"`
	Summary       WorkCoverageSummary   `json:"summary"`
}

type WorkCoverageSummary struct {
	CommitsWithoutTaskID        int `json:"commits_without_task_id"`
	ChangedFilesUncovered       int `json:"changed_files_uncovered"`
	OrphanEvidence              int `json:"orphan_evidence"`
	EvidenceStatusDecisions     int `json:"evidence_status_decisions"`
	DoneWithoutRequiredEvidence int `json:"done_without_required_evidence"`
	MissingRequiredReviews      int `json:"missing_required_reviews"`
	WorkBatchCandidates         int `json:"work_batch_candidates"`
}

type WorkCoverageFinding struct {
	Kind         string   `json:"kind"`
	Severity     string   `json:"severity"`
	Reason       string   `json:"reason"`
	TaskID       string   `json:"task_id,omitempty"`
	Commit       string   `json:"commit,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	Files        []string `json:"files,omitempty"`
	RelatedTasks []string `json:"related_tasks,omitempty"`
	Missing      []string `json:"missing,omitempty"`
	Recommended  string   `json:"recommended_action,omitempty"`
}

func BuildWorkCoverageReport(ctx context.Context, cfg config.Config, root string, s *store.Store, opts WorkCoverageOptions) (WorkCoverageReport, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return WorkCoverageReport{}, err
	}
	if opts.TaskID != "" {
		var filtered []store.Task
		for _, task := range tasks {
			if task.Definition.ID == opts.TaskID {
				filtered = append(filtered, task)
				break
			}
		}
		if len(filtered) == 0 {
			return WorkCoverageReport{}, store.ErrNotFound
		}
		tasks = filtered
	}
	report := WorkCoverageReport{OK: true, TaskID: opts.TaskID}
	var commits []fairwaygit.Commit
	switch {
	case opts.SinceDuration > 0:
		commits, err = fairwaygit.CommitsSince(root, time.Now().UTC().Add(-opts.SinceDuration))
		report.SinceDuration = opts.SinceDuration.String()
	default:
		sinceRef := opts.SinceRef
		if sinceRef == "" {
			sinceRef = cfg.Fairway.MainBranch
		}
		commits, err = fairwaygit.CommitsSinceRef(root, sinceRef)
		report.SinceRef = sinceRef
	}
	if err != nil {
		return WorkCoverageReport{}, err
	}
	report.CommitCount = len(commits)
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	taskPattern := taskIDPattern(taskIDs)
	pathCoverage := buildTaskPathCoverage(tasks)
	evidenceByTask := map[string][]store.Evidence{}
	for _, commit := range commits {
		mentioned := mentionedTaskIDs(commit.Subject+"\n"+commit.Body, taskPattern, tasks)
		coveredTasks := tasksCoveringFiles(commit.ChangedFiles, pathCoverage)
		if len(mentioned) == 0 && len(coveredTasks) == 0 {
			report.Findings = append(report.Findings, WorkCoverageFinding{
				Kind:        "commit_without_task_coverage",
				Severity:    "warning",
				Reason:      "commit does not mention a Fairway task id and changed files do not map to task source_paths/target_paths",
				Commit:      commit.ShortSHA,
				Subject:     commit.Subject,
				Files:       commit.ChangedFiles,
				Recommended: "link the work to an existing task, add task path metadata, or create a follow-up task",
			})
			report.Summary.CommitsWithoutTaskID++
		}
		for _, file := range commit.ChangedFiles {
			if len(tasksCoveringFile(file, pathCoverage)) == 0 {
				report.Findings = append(report.Findings, WorkCoverageFinding{
					Kind:        "changed_file_uncovered",
					Severity:    "warning",
					Reason:      "changed file is not covered by any task source_paths or target_paths",
					Commit:      commit.ShortSHA,
					Subject:     commit.Subject,
					Files:       []string{file},
					Recommended: "add source_paths/target_paths metadata to the responsible task",
				})
				report.Summary.ChangedFilesUncovered++
			}
		}
	}
	for _, task := range tasks {
		_, _, evidence, _, reviews, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return WorkCoverageReport{}, err
		}
		evidenceByTask[task.Definition.ID] = evidence
		if len(evidence) > 0 && !isTerminalStatus(task.Status, cfg.States.Terminal) {
			report.Findings = append(report.Findings, WorkCoverageFinding{
				Kind:        "evidence_without_status_decision",
				Severity:    "warning",
				Reason:      "task has evidence but is not in a terminal state",
				TaskID:      task.Definition.ID,
				Recommended: "mark done, block/reset with a reason, or create explicit follow-up work",
			})
			report.Summary.EvidenceStatusDecisions++
		}
		if task.Status == "done" && len(evidence) == 0 && evidenceRequiredForTask(cfg, task) {
			report.Findings = append(report.Findings, WorkCoverageFinding{
				Kind:        "done_without_required_evidence",
				Severity:    "warning",
				Reason:      "task is done but required evidence is missing",
				TaskID:      task.Definition.ID,
				Recommended: "record evidence or document an explicit exception",
			})
			report.Summary.DoneWithoutRequiredEvidence++
		}
		missingDomains := missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)
		if task.Status == "done" && len(missingDomains) > 0 {
			report.Findings = append(report.Findings, WorkCoverageFinding{
				Kind:        "missing_required_review_domains",
				Severity:    "warning",
				Reason:      "task is done but configured review domains are not approved",
				TaskID:      task.Definition.ID,
				Missing:     missingDomains,
				Recommended: "record approvals for missing review domains or document why they are not required",
			})
			report.Summary.MissingRequiredReviews++
		}
	}
	batches, err := s.WorkBatches(ctx)
	if err != nil {
		return WorkCoverageReport{}, err
	}
	for _, finding := range workBatchCandidateFindings(tasks, evidenceByTask, batches) {
		report.Findings = append(report.Findings, finding)
		report.Summary.WorkBatchCandidates++
	}
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Kind != report.Findings[j].Kind {
			return report.Findings[i].Kind < report.Findings[j].Kind
		}
		if report.Findings[i].TaskID != report.Findings[j].TaskID {
			return report.Findings[i].TaskID < report.Findings[j].TaskID
		}
		if report.Findings[i].Commit != report.Findings[j].Commit {
			return report.Findings[i].Commit < report.Findings[j].Commit
		}
		return strings.Join(report.Findings[i].Files, ",") < strings.Join(report.Findings[j].Files, ",")
	})
	report.OK = len(report.Findings) == 0
	return report, nil
}

type CILearningOptions struct {
	TaskID          string
	RenderTemplates bool
}

type CILearningReport struct {
	OK        bool                `json:"ok"`
	TaskID    string              `json:"task_id,omitempty"`
	Findings  []CILearningFinding `json:"findings"`
	Summary   CILearningSummary   `json:"summary"`
	Templates []LearningArtifact  `json:"learning_artifacts,omitempty"`
}

type CILearningSummary struct {
	FailedEvidence       int `json:"failed_evidence"`
	MissingFollowUps     int `json:"missing_follow_ups"`
	MissedLocalGates     int `json:"missed_local_gates"`
	MissedReviewGates    int `json:"missed_review_gates"`
	CIEnvironmentOnly    int `json:"ci_environment_only"`
	FlakyRunnerOrCache   int `json:"flaky_runner_or_cache"`
	ApprovalGatedBlocker int `json:"approval_gated_blocker"`
}

type CILearningFinding struct {
	TaskID                      string `json:"task_id"`
	TaskTitle                   string `json:"task_title"`
	EvidenceType                string `json:"evidence_type"`
	Result                      string `json:"result"`
	CommandText                 string `json:"command_text"`
	ArtifactPath                string `json:"artifact_path,omitempty"`
	FailureClass                string `json:"failure_class"`
	RootCause                   string `json:"root_cause"`
	MissedGate                  string `json:"missed_gate,omitempty"`
	ExpectedLocalReproduction   string `json:"expected_local_reproduction,omitempty"`
	Owner                       string `json:"owner,omitempty"`
	FollowUpTask                string `json:"follow_up_task,omitempty"`
	FollowUpMissing             bool   `json:"follow_up_missing"`
	RecommendedFollowUpPrefix   string `json:"recommended_follow_up_prefix,omitempty"`
	RecommendedFollowUpTaskID   string `json:"recommended_follow_up_task_id,omitempty"`
	RecommendedFollowUpTaskKind string `json:"recommended_follow_up_task_kind,omitempty"`
}

type LearningArtifact struct {
	TaskID                    string `json:"task_id"`
	FailureClass              string `json:"failure_class"`
	RootCause                 string `json:"root_cause"`
	MissedGate                string `json:"missed_gate,omitempty"`
	ExpectedLocalReproduction string `json:"expected_local_reproduction,omitempty"`
	Owner                     string `json:"owner,omitempty"`
	FollowUpTask              string `json:"follow_up_task,omitempty"`
	Markdown                  string `json:"markdown"`
}

func BuildCILearningReport(ctx context.Context, cfg config.Config, s *store.Store, opts CILearningOptions) (CILearningReport, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return CILearningReport{}, err
	}
	taskByID := map[string]store.Task{}
	for _, task := range tasks {
		taskByID[task.Definition.ID] = task
	}
	if opts.TaskID != "" {
		task, ok := taskByID[opts.TaskID]
		if !ok {
			return CILearningReport{}, store.ErrNotFound
		}
		tasks = []store.Task{task}
	}
	allFollowUps := findLearningFollowUps(taskByID)
	report := CILearningReport{OK: true, TaskID: opts.TaskID}
	for _, task := range tasks {
		_, _, evidence, _, reviews, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return CILearningReport{}, err
		}
		for _, ev := range evidence {
			if !isFailedPipelineEvidence(ev) {
				continue
			}
			finding := classifyLearningFailure(cfg, task, ev, reviews, allFollowUps)
			report.Findings = append(report.Findings, finding)
			report.Summary.FailedEvidence++
			if finding.FollowUpMissing {
				report.Summary.MissingFollowUps++
			}
			switch finding.FailureClass {
			case "missed_local_gate":
				report.Summary.MissedLocalGates++
			case "missed_review_gate":
				report.Summary.MissedReviewGates++
			case "ci_environment_only":
				report.Summary.CIEnvironmentOnly++
			case "flaky_runner_cache":
				report.Summary.FlakyRunnerOrCache++
			case "approval_gated_blocker":
				report.Summary.ApprovalGatedBlocker++
			}
			if opts.RenderTemplates {
				report.Templates = append(report.Templates, learningArtifactForFinding(finding))
			}
		}
	}
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].TaskID != report.Findings[j].TaskID {
			return report.Findings[i].TaskID < report.Findings[j].TaskID
		}
		return report.Findings[i].CommandText < report.Findings[j].CommandText
	})
	report.OK = report.Summary.MissingFollowUps == 0 && report.Summary.MissedLocalGates == 0 && report.Summary.MissedReviewGates == 0 && report.Summary.ApprovalGatedBlocker == 0
	return report, nil
}

type taskPathCoverage struct {
	TaskID   string
	Patterns []string
}

func buildTaskPathCoverage(tasks []store.Task) []taskPathCoverage {
	var coverage []taskPathCoverage
	for _, task := range tasks {
		patterns := append([]string{}, task.Definition.SourcePaths...)
		patterns = append(patterns, task.Definition.TargetPaths...)
		var cleaned []string
		for _, pattern := range patterns {
			pattern = strings.TrimSpace(filepath.ToSlash(pattern))
			if pattern != "" {
				cleaned = append(cleaned, pattern)
			}
		}
		if len(cleaned) > 0 {
			coverage = append(coverage, taskPathCoverage{TaskID: task.Definition.ID, Patterns: cleaned})
		}
	}
	return coverage
}

func tasksCoveringFiles(files []string, coverage []taskPathCoverage) []string {
	seen := map[string]bool{}
	for _, file := range files {
		for _, taskID := range tasksCoveringFile(file, coverage) {
			seen[taskID] = true
		}
	}
	var out []string
	for taskID := range seen {
		out = append(out, taskID)
	}
	sort.Strings(out)
	return out
}

func workBatchCandidateFindings(tasks []store.Task, evidenceByTask map[string][]store.Evidence, batches []store.WorkBatch) []WorkCoverageFinding {
	batched := map[string]bool{}
	for _, batch := range batches {
		for _, taskID := range batch.Tasks {
			batched[taskID] = true
		}
	}
	type candidate struct {
		domain string
		tasks  []string
	}
	groups := map[string]candidate{}
	for _, task := range tasks {
		if batched[task.Definition.ID] || !taskLooksBatchable(task) || !hasPipelineEvidence(evidenceByTask[task.Definition.ID]) {
			continue
		}
		domain := strings.TrimSpace(task.Definition.OwningDomain)
		if domain == "" {
			domain = strings.TrimSpace(task.Definition.Role)
		}
		if domain == "" {
			domain = "unassigned"
		}
		key := domain + "|" + strings.TrimSpace(task.Definition.Role)
		group := groups[key]
		group.domain = domain
		group.tasks = append(group.tasks, task.Definition.ID)
		groups[key] = group
	}
	var findings []WorkCoverageFinding
	for _, group := range groups {
		if len(group.tasks) < 2 {
			continue
		}
		sort.Strings(group.tasks)
		findings = append(findings, WorkCoverageFinding{
			Kind:         "work_batch_candidate",
			Severity:     "info",
			Reason:       "multiple related tasks in the same domain have separate CI/deploy evidence and may be over-split",
			RelatedTasks: append([]string{}, group.tasks...),
			Recommended:  "consider a work batch when these tasks share branch, worktree, validation commands, review domains, and rollback behavior",
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return strings.Join(findings[i].RelatedTasks, ",") < strings.Join(findings[j].RelatedTasks, ",")
	})
	return findings
}

func taskLooksBatchable(task store.Task) bool {
	text := strings.ToLower(strings.Join([]string{
		task.Definition.Kind,
		task.Definition.Role,
		task.Definition.OwningDomain,
		task.Definition.OwningLayer,
		task.Definition.MigrationType,
		task.Definition.Title,
	}, " "))
	for _, keyword := range []string{"ci", "deploy", "smoke", "uat", "release", "monitor", "validation"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func hasPipelineEvidence(evidence []store.Evidence) bool {
	for _, ev := range evidence {
		text := strings.ToLower(strings.Join([]string{ev.ArtifactType, ev.CommandText, ev.ArtifactPath, ev.Notes}, " "))
		for _, keyword := range []string{"ci", "pipeline", "deploy", "smoke", "uat", "gh run", "gitlab", "workflow"} {
			if strings.Contains(text, keyword) {
				return true
			}
		}
	}
	return false
}

func tasksCoveringFile(file string, coverage []taskPathCoverage) []string {
	file = strings.TrimSpace(filepath.ToSlash(file))
	var out []string
	for _, item := range coverage {
		for _, pattern := range item.Patterns {
			if pathPatternMatches(pattern, file) {
				out = append(out, item.TaskID)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func pathPatternMatches(pattern, file string) bool {
	pattern = strings.TrimPrefix(strings.TrimSpace(filepath.ToSlash(pattern)), "./")
	file = strings.TrimPrefix(strings.TrimSpace(filepath.ToSlash(file)), "./")
	if pattern == "" || file == "" {
		return false
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return file == prefix || strings.HasPrefix(file, prefix+"/")
	}
	if ok, err := path.Match(pattern, file); err == nil && ok {
		return true
	}
	if file == pattern {
		return true
	}
	return strings.HasPrefix(file, strings.TrimSuffix(pattern, "/")+"/")
}

func taskIDPattern(taskIDs []string) *regexp.Regexp {
	if len(taskIDs) == 0 {
		return regexp.MustCompile(`a^`)
	}
	sort.SliceStable(taskIDs, func(i, j int) bool {
		return len(taskIDs[i]) > len(taskIDs[j])
	})
	parts := make([]string, 0, len(taskIDs))
	for _, id := range taskIDs {
		parts = append(parts, regexp.QuoteMeta(id))
	}
	return regexp.MustCompile(`\b(?:` + strings.Join(parts, "|") + `)\b`)
}

func mentionedTaskIDs(text string, pattern *regexp.Regexp, tasks []store.Task) []string {
	matches := pattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	known := map[string]bool{}
	for _, task := range tasks {
		known[task.Definition.ID] = true
	}
	seen := map[string]bool{}
	for _, match := range matches {
		if known[match] {
			seen[match] = true
		}
	}
	var out []string
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func evidenceRequiredForTask(cfg config.Config, task store.Task) bool {
	if cfg.Gates.RequireEvidenceBeforeDone {
		return true
	}
	for _, profile := range cfg.WorkstreamProfiles {
		if !profileAppliesToTask(profile, task) {
			continue
		}
		for _, gate := range profile.Gates {
			if !gateAppliesToTask(gate, task) {
				continue
			}
			if gate.EvidenceType != "" || gate.RequiredEvidenceCount > 0 || len(gate.AcceptedResults) > 0 || gate.ArtifactRequired || gate.ExpiresAfter != "" || gate.OwnerSignoffRequired {
				return true
			}
		}
	}
	return false
}

func isTerminalStatus(status string, terminal []string) bool {
	for _, value := range terminal {
		if status == value {
			return true
		}
	}
	return false
}

func isFailedPipelineEvidence(ev store.Evidence) bool {
	switch ev.Result {
	case "fail", "blocked":
	default:
		return false
	}
	text := strings.ToLower(strings.Join([]string{ev.ArtifactType, ev.CommandText, ev.ArtifactPath, ev.Notes}, " "))
	keywords := []string{"ci", "pipeline", "deploy", "deployment", "smoke", "uat", "release", "workflow", "github actions", "gitlab"}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func classifyLearningFailure(cfg config.Config, task store.Task, ev store.Evidence, reviews []store.Review, followUps map[string]string) CILearningFinding {
	text := strings.ToLower(strings.Join([]string{ev.CommandText, ev.ArtifactPath, ev.ArtifactType, ev.Notes}, " "))
	finding := CILearningFinding{
		TaskID:                    task.Definition.ID,
		TaskTitle:                 task.Definition.Title,
		EvidenceType:              ev.ArtifactType,
		Result:                    ev.Result,
		CommandText:               ev.CommandText,
		ArtifactPath:              ev.ArtifactPath,
		Owner:                     firstNonEmpty(task.Owner, task.Definition.Role),
		RecommendedFollowUpPrefix: learningFollowUpPrefix(text, ev.ArtifactType),
	}
	missingReviews := missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)
	switch {
	case ev.Result == "blocked" || containsAny(text, "approval", "manual gate", "permission", "forbidden", "unauthorized", "secret", "credential"):
		finding.FailureClass = "approval_gated_blocker"
		finding.RootCause = "execution was blocked by approval, permission, secret, or credential state"
		finding.MissedGate = "approval or secret readiness gate"
	case containsAny(text, "flaky", "flake", "runner", "cache", "timeout", "rate limit", "network", "connection reset"):
		finding.FailureClass = "flaky_runner_cache"
		finding.RootCause = "failure appears tied to runner, cache, network, or intermittent infrastructure behavior"
		finding.MissedGate = "runner/cache reliability check"
	case containsAny(text, "ci only", "ci-only", "github actions", "gitlab", "container", "linux", "environment", "env var"):
		finding.FailureClass = "ci_environment_only"
		finding.RootCause = "failure appears specific to CI/deploy environment differences"
		finding.MissedGate = "environment parity check"
	case len(missingReviews) > 0 || (cfg.Gates.RequireReviewBeforeDone && task.ReviewStatus != "approved"):
		finding.FailureClass = "missed_review_gate"
		finding.RootCause = "failure happened while required review coverage was missing"
		finding.MissedGate = "review domains: " + strings.Join(missingReviews, ", ")
	default:
		finding.FailureClass = "missed_local_gate"
		finding.RootCause = "failure should have been caught by a local verification command before CI/deploy"
		finding.MissedGate = "local verification gate"
	}
	finding.ExpectedLocalReproduction = expectedLocalReproduction(ev)
	if followUp, ok := followUps[finding.TaskID]; ok {
		finding.FollowUpTask = followUp
	} else {
		finding.FollowUpMissing = true
		finding.RecommendedFollowUpTaskID = finding.RecommendedFollowUpPrefix + "-" + sanitizeFollowUpSuffix(finding.TaskID)
		finding.RecommendedFollowUpTaskKind = "bug"
	}
	return finding
}

func findLearningFollowUps(tasks map[string]store.Task) map[string]string {
	prefixes := []string{"CI-FIX-", "CD-FIX-", "OPS-FIX-", "HARNESS-FIX-", "UAT-BUG-", "DOC-FIX-"}
	out := map[string]string{}
	for _, task := range tasks {
		if !hasAnyPrefix(task.Definition.ID, prefixes) {
			continue
		}
		text := strings.ToLower(strings.Join([]string{task.Definition.ID, task.Definition.Title, task.Definition.Notes}, " "))
		for sourceID := range tasks {
			if strings.Contains(text, strings.ToLower(sourceID)) {
				out[sourceID] = task.Definition.ID
			}
		}
	}
	return out
}

func learningFollowUpPrefix(text, evidenceType string) string {
	joined := strings.ToLower(text + " " + evidenceType)
	switch {
	case strings.Contains(joined, "uat"):
		return "UAT-BUG"
	case strings.Contains(joined, "doc") || strings.Contains(joined, "docusaurus"):
		return "DOC-FIX"
	case strings.Contains(joined, "deploy") || strings.Contains(joined, "release"):
		return "CD-FIX"
	case containsAny(joined, "runner", "cache", "harness", "workflow"):
		return "HARNESS-FIX"
	case containsAny(joined, "ops", "secret", "credential", "permission", "approval"):
		return "OPS-FIX"
	default:
		return "CI-FIX"
	}
}

func expectedLocalReproduction(ev store.Evidence) string {
	if strings.TrimSpace(ev.CommandText) != "" {
		return ev.CommandText
	}
	switch {
	case strings.Contains(strings.ToLower(ev.ArtifactType), "deploy"):
		return "run the documented deploy dry-run or smoke command"
	case strings.Contains(strings.ToLower(ev.ArtifactType), "uat"):
		return "run the documented UAT smoke command"
	default:
		return "run the local command that should reproduce this CI failure"
	}
}

func learningArtifactForFinding(finding CILearningFinding) LearningArtifact {
	followUp := finding.FollowUpTask
	if followUp == "" {
		followUp = finding.RecommendedFollowUpTaskID
	}
	artifact := LearningArtifact{
		TaskID:                    finding.TaskID,
		FailureClass:              finding.FailureClass,
		RootCause:                 finding.RootCause,
		MissedGate:                finding.MissedGate,
		ExpectedLocalReproduction: finding.ExpectedLocalReproduction,
		Owner:                     finding.Owner,
		FollowUpTask:              followUp,
	}
	artifact.Markdown = fmt.Sprintf(`# CI/Deploy Learning: %s

- Failure class: %s
- Root cause: %s
- Missed gate: %s
- Expected local reproduction: %s
- Owner: %s
- Follow-up task: %s
`, artifact.TaskID, artifact.FailureClass, artifact.RootCause, artifact.MissedGate, artifact.ExpectedLocalReproduction, artifact.Owner, artifact.FollowUpTask)
	return artifact
}

func missingApprovedReviewDomains(domains []string, reviews []store.Review) []string {
	if len(domains) == 0 {
		return nil
	}
	approved := map[string]bool{}
	for _, review := range reviews {
		if review.Verdict == "approve" {
			approved[review.Reviewer] = true
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

func profileAppliesToTask(profile config.WorkstreamProfile, task store.Task) bool {
	if len(profile.TaskKinds) == 0 {
		return true
	}
	for _, kind := range profile.TaskKinds {
		if task.Definition.Kind == kind {
			return true
		}
	}
	return false
}

func gateAppliesToTask(gate config.WorkstreamProfileGate, task store.Task) bool {
	if len(gate.TaskKinds) == 0 {
		return true
	}
	for _, kind := range gate.TaskKinds {
		if task.Definition.Kind == kind {
			return true
		}
	}
	return false
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(text string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func sanitizeFollowUpSuffix(taskID string) string {
	replacer := strings.NewReplacer("_", "-", ".", "-", "/", "-")
	return strings.ToUpper(replacer.Replace(taskID))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
