package audit

import (
	"context"
	"fmt"
	"os"
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
	SinceRef          string
	SinceDuration     time.Duration
	TaskID            string
	TaskIDs           []string
	RestrictTaskIDs   bool
	PromotionAtByTask map[string]string
	Now               time.Time
}

type WorkCoverageReport struct {
	OK            bool                          `json:"ok"`
	AsOf          string                        `json:"as_of"`
	AnalyzedTip   string                        `json:"analyzed_tip"`
	SinceRef      string                        `json:"since_ref,omitempty"`
	SinceDuration string                        `json:"since_duration,omitempty"`
	TaskID        string                        `json:"task_id,omitempty"`
	CommitCount   int                           `json:"commit_count"`
	Denominators  WorkCoverageDenominators      `json:"denominators"`
	Exclusions    []config.ControlPathExclusion `json:"exclusions,omitempty"`
	CommitFacts   []CommitCoverageFact          `json:"commit_facts,omitempty"`
	TouchFacts    []PostPromotionTouchFact      `json:"post_promotion_touch_facts,omitempty"`
	Outcomes      []store.TaskOutcome           `json:"outcomes,omitempty"`
	Findings      []WorkCoverageFinding         `json:"findings"`
	Summary       WorkCoverageSummary           `json:"summary"`
}

type WorkCoverageDenominators struct {
	ObservedCommits         int `json:"observed_commits"`
	EligibleCommits         int `json:"eligible_commits"`
	CoveredCommits          int `json:"covered_commits"`
	ExplicitlyLinkedCommits int `json:"explicitly_linked_commits"`
	ExcludedMergeCommits    int `json:"excluded_merge_commits"`
	ExcludedOnlyCommits     int `json:"excluded_only_commits"`
	EligibleChangedFiles    int `json:"eligible_changed_files"`
	CoveredChangedFiles     int `json:"covered_changed_files"`
	ExcludedChangedFiles    int `json:"excluded_changed_files"`
	TasksWithCommit         int `json:"tasks_with_commit"`
	TasksWithTouchFacts     int `json:"tasks_with_touch_facts"`
	UnavailableTouchFacts   int `json:"unavailable_touch_facts"`
	MatureTouchWindows      int `json:"mature_touch_windows"`
}

type CommitCoverageFact struct {
	Commit          string   `json:"commit"`
	Subject         string   `json:"subject"`
	Eligible        bool     `json:"eligible"`
	Exclusion       string   `json:"exclusion,omitempty"`
	TaskIDs         []string `json:"task_ids,omitempty"`
	ExplicitTaskIDs []string `json:"explicit_task_ids,omitempty"`
	PathTaskIDs     []string `json:"path_task_ids,omitempty"`
	EligibleFiles   []string `json:"eligible_files,omitempty"`
	ExcludedFiles   []string `json:"excluded_files,omitempty"`
}

type PostPromotionTouchFact struct {
	TaskID            string            `json:"task_id"`
	PromotionCommit   string            `json:"promotion_commit"`
	PromotionAt       string            `json:"promotion_at,omitempty"`
	EligibleFiles     []string          `json:"eligible_files,omitempty"`
	ExcludedFiles     []string          `json:"excluded_files,omitempty"`
	Available         bool              `json:"available"`
	UnavailableReason string            `json:"unavailable_reason,omitempty"`
	Windows           []TouchWindowFact `json:"windows,omitempty"`
}

type TouchWindowFact struct {
	Days         int      `json:"days"`
	Mature       bool     `json:"mature"`
	Touched      bool     `json:"touched"`
	TouchCommits []string `json:"touch_commits,omitempty"`
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
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tip, err := fairwaygit.ResolveCommit(root, "HEAD")
	if err != nil {
		return WorkCoverageReport{}, err
	}
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
	} else if opts.RestrictTaskIDs {
		wanted := map[string]bool{}
		for _, id := range opts.TaskIDs {
			wanted[strings.TrimSpace(id)] = true
		}
		var filtered []store.Task
		for _, task := range tasks {
			if wanted[task.Definition.ID] {
				filtered = append(filtered, task)
			}
		}
		tasks = filtered
	}
	report := WorkCoverageReport{OK: true, AsOf: now.Format(time.RFC3339), AnalyzedTip: tip.SHA, TaskID: opts.TaskID, Exclusions: append([]config.ControlPathExclusion(nil), cfg.ControlEffectiveness.PathExclusions...)}
	var commits []fairwaygit.Commit
	switch {
	case opts.SinceDuration > 0:
		commits, err = fairwaygit.CommitsSinceAt(root, now.Add(-opts.SinceDuration), tip.SHA)
		report.SinceDuration = opts.SinceDuration.String()
	default:
		sinceRef := opts.SinceRef
		if sinceRef == "" {
			sinceRef = cfg.Fairway.MainBranch
		}
		commits, err = fairwaygit.CommitsBetween(root, sinceRef, tip.SHA)
		report.SinceRef = sinceRef
	}
	if err != nil {
		return WorkCoverageReport{}, err
	}
	report.CommitCount = len(commits)
	report.Denominators.ObservedCommits = len(commits)
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	taskPattern := taskIDPattern(taskIDs)
	pathCoverage := buildTaskPathCoverage(tasks)
	evidenceByTask, err := s.EvidenceByTaskIDs(ctx, taskIDs)
	if err != nil {
		return WorkCoverageReport{}, err
	}
	reviewsByTask, err := s.ReviewsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return WorkCoverageReport{}, err
	}
	taskCommitsByTask, err := s.TaskCommitsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return WorkCoverageReport{}, err
	}
	for _, commit := range commits {
		fact := CommitCoverageFact{Commit: commit.ShortSHA, Subject: commit.Subject}
		if commit.ParentCount > 1 {
			fact.Exclusion = "merge_commit"
			report.Denominators.ExcludedMergeCommits++
			report.CommitFacts = append(report.CommitFacts, fact)
			continue
		}
		fact.EligibleFiles, fact.ExcludedFiles = partitionExcludedPaths(commit.ChangedFiles, cfg.ControlEffectiveness.PathExclusions)
		report.Denominators.ExcludedChangedFiles += len(fact.ExcludedFiles)
		if len(fact.EligibleFiles) == 0 {
			fact.Exclusion = "all_paths_excluded"
			report.Denominators.ExcludedOnlyCommits++
			report.CommitFacts = append(report.CommitFacts, fact)
			continue
		}
		fact.Eligible = true
		report.Denominators.EligibleCommits++
		report.Denominators.EligibleChangedFiles += len(fact.EligibleFiles)
		mentioned := mentionedTaskIDs(commit.Subject+"\n"+commit.Body, taskPattern, tasks)
		coveredTasks := tasksCoveringFiles(fact.EligibleFiles, pathCoverage)
		canonicalTasks := tasksWithCommit(commit.SHA, tasks)
		fact.ExplicitTaskIDs = tasksWithExplicitCommit(commit.SHA, taskCommitsByTask)
		fact.TaskIDs = unionStrings(unionStrings(mentioned, canonicalTasks), fact.ExplicitTaskIDs)
		fact.PathTaskIDs = coveredTasks
		if len(fact.TaskIDs) > 0 {
			report.Denominators.CoveredCommits++
		}
		if len(fact.ExplicitTaskIDs) > 0 {
			report.Denominators.ExplicitlyLinkedCommits++
		}
		report.CommitFacts = append(report.CommitFacts, fact)
		if len(fact.TaskIDs) == 0 {
			report.Findings = append(report.Findings, WorkCoverageFinding{
				Kind:        "commit_without_task_coverage",
				Severity:    "warning",
				Reason:      "commit has no explicit Fairway task association, task id in its message, or canonical completion link",
				Commit:      commit.ShortSHA,
				Subject:     commit.Subject,
				Files:       fact.EligibleFiles,
				Recommended: "run fairway record commit for the owning task or close work through the normal Fairway path",
			})
			report.Summary.CommitsWithoutTaskID++
		}
		for _, file := range fact.EligibleFiles {
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
			} else {
				report.Denominators.CoveredChangedFiles++
			}
		}
	}
	outcomes, err := s.TaskOutcomes(ctx, opts.TaskID)
	if err != nil {
		return WorkCoverageReport{}, err
	}
	report.Outcomes = outcomes
	for _, task := range tasks {
		if strings.TrimSpace(task.CommitSHA) == "" {
			continue
		}
		if promotionAt := strings.TrimSpace(opts.PromotionAtByTask[task.Definition.ID]); promotionAt != "" {
			task.CompletedAt = promotionAt
		}
		report.Denominators.TasksWithCommit++
		candidateCommits := []fairwaygit.Commit(nil)
		useCandidateCommits := opts.RestrictTaskIDs && opts.SinceDuration > 0
		if useCandidateCommits {
			candidateCommits = commits
		}
		fact := buildPostPromotionTouchFact(root, task, cfg.ControlEffectiveness.PathExclusions, now, tip.SHA, candidateCommits, useCandidateCommits)
		if fact.Available {
			report.Denominators.TasksWithTouchFacts++
			for _, window := range fact.Windows {
				if window.Mature {
					report.Denominators.MatureTouchWindows++
				}
			}
		} else {
			report.Denominators.UnavailableTouchFacts++
		}
		report.TouchFacts = append(report.TouchFacts, fact)
	}
	for _, task := range tasks {
		evidence := evidenceByTask[task.Definition.ID]
		reviews := reviewsByTask[task.Definition.ID]
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

func tasksWithCommit(commitSHA string, tasks []store.Task) []string {
	var out []string
	commitSHA = strings.TrimSpace(commitSHA)
	for _, task := range tasks {
		taskSHA := strings.TrimSpace(task.CommitSHA)
		if taskSHA == "" {
			continue
		}
		if commitSHA == taskSHA || (len(taskSHA) >= 7 && strings.HasPrefix(commitSHA, taskSHA)) || (len(commitSHA) >= 7 && strings.HasPrefix(taskSHA, commitSHA)) {
			out = append(out, task.Definition.ID)
		}
	}
	return out
}

func tasksWithExplicitCommit(commitSHA string, byTask map[string][]store.TaskCommit) []string {
	var out []string
	commitSHA = strings.TrimSpace(commitSHA)
	for taskID, associations := range byTask {
		for _, association := range associations {
			if association.AssociationKind == "work_base" {
				continue
			}
			if commitMatches(commitSHA, association.CommitSHA) {
				out = append(out, taskID)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func commitMatches(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return left == right || (len(left) >= 7 && strings.HasPrefix(right, left)) || (len(right) >= 7 && strings.HasPrefix(left, right))
}

func buildPostPromotionTouchFact(root string, task store.Task, exclusions []config.ControlPathExclusion, now time.Time, analyzedTip string, candidateCommits []fairwaygit.Commit, useCandidateCommits bool) PostPromotionTouchFact {
	fact := PostPromotionTouchFact{TaskID: task.Definition.ID, PromotionCommit: task.CommitSHA}
	detail, err := fairwaygit.ResolveCommit(root, task.CommitSHA)
	if err != nil {
		fact.UnavailableReason = err.Error()
		return fact
	}
	promotionAt, err := time.Parse(time.RFC3339Nano, task.CompletedAt)
	if err != nil {
		promotionAt, err = time.Parse(time.RFC3339, detail.CommitDate)
		if err != nil {
			fact.UnavailableReason = "task completion and promotion commit dates are unavailable"
			return fact
		}
	}
	fact.PromotionCommit = detail.SHA
	fact.PromotionAt = promotionAt.UTC().Format(time.RFC3339)
	fact.EligibleFiles, fact.ExcludedFiles = partitionExcludedPaths(detail.ChangedFiles, exclusions)
	if len(fact.EligibleFiles) == 0 {
		fact.UnavailableReason = "promotion commit has no eligible changed files"
		return fact
	}
	commits := candidateCommits
	if !useCandidateCommits {
		commits, err = fairwaygit.CommitsBetween(root, detail.SHA, analyzedTip)
		if err != nil {
			fact.UnavailableReason = err.Error()
			return fact
		}
	}
	fact.Available = true
	owned := make(map[string]bool, len(fact.EligibleFiles))
	for _, file := range fact.EligibleFiles {
		owned[file] = true
	}
	for _, days := range []int{7, 14, 30} {
		deadline := promotionAt.Add(time.Duration(days) * 24 * time.Hour)
		window := TouchWindowFact{Days: days, Mature: !now.Before(deadline)}
		for _, commit := range commits {
			if commit.SHA == detail.SHA {
				continue
			}
			commitAt, err := time.Parse(time.RFC3339, commit.CommitDate)
			if err != nil || commitAt.Before(promotionAt.Truncate(time.Second)) || commitAt.After(deadline) || commit.ParentCount > 1 {
				continue
			}
			eligible, _ := partitionExcludedPaths(commit.ChangedFiles, exclusions)
			if pathsIntersect(owned, eligible) {
				window.Touched = true
				window.TouchCommits = append(window.TouchCommits, commit.ShortSHA)
			}
		}
		fact.Windows = append(fact.Windows, window)
	}
	return fact
}

func partitionExcludedPaths(files []string, exclusions []config.ControlPathExclusion) ([]string, []string) {
	var eligible, excluded []string
	for _, file := range files {
		matched := false
		for _, exclusion := range exclusions {
			if pathPatternMatches(exclusion.Pattern, file) {
				matched = true
				break
			}
		}
		if matched {
			excluded = append(excluded, file)
		} else {
			eligible = append(eligible, file)
		}
	}
	return eligible, excluded
}

func pathsIntersect(owned map[string]bool, files []string) bool {
	for _, file := range files {
		if owned[file] {
			return true
		}
	}
	return false
}

type CILearningOptions struct {
	TaskID          string
	RenderTemplates bool
}

type DocsBacklogOptions struct {
	DocPaths []string
}

type DocsBacklogReport struct {
	OK       bool                 `json:"ok"`
	Docs     []DocsBacklogDoc     `json:"docs"`
	Findings []DocsBacklogFinding `json:"findings"`
	Summary  DocsBacklogSummary   `json:"summary"`
}

type DocsBacklogSummary struct {
	DocsScanned              int `json:"docs_scanned"`
	DocsWithBacklogCoverage  int `json:"docs_with_backlog_coverage"`
	DocOnlyCapabilities      int `json:"doc_only_capabilities"`
	CommandExamplesUncovered int `json:"command_examples_uncovered"`
	StaleCompletedTasks      int `json:"stale_completed_tasks"`
	ConsumerLessons          int `json:"consumer_lessons"`
	// Deprecated: retained for one compatibility window for existing JSON readers.
	LegacyConsumerLessons int `json:"gpuaas_lessons,omitempty"`
}

type DocsBacklogDoc struct {
	Path            string   `json:"path"`
	MentionedTasks  []string `json:"mentioned_tasks,omitempty"`
	CoveringTasks   []string `json:"covering_tasks,omitempty"`
	CommandExamples []string `json:"command_examples,omitempty"`
	Topics          []string `json:"topics,omitempty"`
}

type DocsBacklogFinding struct {
	Kind        string   `json:"kind"`
	Severity    string   `json:"severity"`
	DocPath     string   `json:"doc_path,omitempty"`
	TaskID      string   `json:"task_id,omitempty"`
	Topic       string   `json:"topic,omitempty"`
	Command     string   `json:"command,omitempty"`
	Reason      string   `json:"reason"`
	Related     []string `json:"related_tasks,omitempty"`
	Recommended string   `json:"recommended_action,omitempty"`
}

var defaultCoordinationDocPaths = []string{
	"docs/design/review-wait-notification-model.md",
	"docs/design/provider-notifications.md",
	"docs/design/coordinator-loop.md",
	"docs/design/live-operation-control-room.md",
	"docs/design/coordination-intelligence.md",
	"docs/design/provider-surface-capability-readiness.md",
	"docs/design/consumer-critical-flow-governance.md",
	"docs/design/review-policy-profiles.md",
	"docs/agent-guide.md",
}

type docsBacklogTopic struct {
	Name           string
	Terms          []string
	TaskIDs        []string
	ConsumerLesson bool
	Optional       bool
}

var coordinationBacklogTopics = []docsBacklogTopic{
	{Name: "review-wait", Terms: []string{"review wait", "review-waits"}, TaskIDs: []string{"FW-179", "FW-180", "FW-181", "FW-182", "FW-183", "FW-184"}},
	{Name: "completion-handback", Terms: []string{"completion handback", "completion-handback"}, TaskIDs: []string{"FW-187", "FW-188", "FW-189", "FW-190", "FW-191"}},
	{Name: "live-operation-control-room", Terms: []string{"live-operation control", "control room", "live-window"}, TaskIDs: []string{"FW-194", "FW-206"}},
	{Name: "track-memory", Terms: []string{"track memory", "memory packet"}, TaskIDs: []string{"FW-196", "FW-202"}},
	{Name: "generic-waits", Terms: []string{"generic wait", "wait/watch", "parked work"}, TaskIDs: []string{"FW-197", "FW-198", "FW-202"}},
	{Name: "known-failure-routing", Terms: []string{"known-failure", "failure routing"}, TaskIDs: []string{"FW-199"}},
	{Name: "retry-packets", Terms: []string{"retry packet", "bounded rerun"}, TaskIDs: []string{"FW-200", "FW-206"}},
	{Name: "advisory-recommendations", Terms: []string{"advisory recommendation", "recommendation contract"}, TaskIDs: []string{"FW-201"}},
	{Name: "dashboard-projections", Terms: []string{"dashboard", "read-only dashboard", "diagnostics tab"}, TaskIDs: []string{"FW-181", "FW-190", "FW-202"}},
	{Name: "notification-lifecycle", Terms: []string{"notification lifecycle", "notification audit"}, TaskIDs: []string{"FW-203"}},
	{Name: "completion-supersede", Terms: []string{"supersede", "superseded"}, TaskIDs: []string{"FW-204"}},
	{Name: "wake-routability", Terms: []string{"routability", "provider target"}, TaskIDs: []string{"FW-205"}},
	{Name: "critical-flow-governance", Terms: []string{"critical-flow", "flow map before implementation", "non-live preflight"}, TaskIDs: []string{"FW-208"}, ConsumerLesson: true},
	{Name: "review-profiles", Terms: []string{"review profile", "safe iteration", "safe-boundary", "causal reset"}, TaskIDs: []string{"FW-209"}},
	{Name: "delivery-overhead", Terms: []string{"delivery velocity", "process overhead", "review usefulness"}, TaskIDs: []string{"FW-210"}},
	{Name: "automation-candidates", Terms: []string{"automation candidate", "third time automate", "repeated deterministic work"}, TaskIDs: []string{"FW-211"}},
}

func BuildDocsBacklogReport(ctx context.Context, root string, s *store.Store, opts DocsBacklogOptions) (DocsBacklogReport, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return DocsBacklogReport{}, err
	}
	taskByID := map[string]store.Task{}
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskByID[task.Definition.ID] = task
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	taskPattern := taskIDPattern(taskIDs)
	pathCoverage := buildTaskPathCoverage(tasks)
	docPaths := append([]string{}, opts.DocPaths...)
	if len(docPaths) == 0 {
		docPaths = append(docPaths, defaultCoordinationDocPaths...)
	}
	report := DocsBacklogReport{OK: true}
	for _, docPath := range docPaths {
		docPath = strings.TrimSpace(filepath.ToSlash(docPath))
		if docPath == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(docPath)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return DocsBacklogReport{}, err
		}
		text := string(body)
		mentioned := mentionedTaskIDs(text, taskPattern, tasks)
		covering := tasksCoveringFile(docPath, pathCoverage)
		commands := fairwayCommandExamples(text)
		topics := docTopics(text)
		doc := DocsBacklogDoc{
			Path:            docPath,
			MentionedTasks:  mentioned,
			CoveringTasks:   covering,
			CommandExamples: commands,
			Topics:          topics,
		}
		report.Docs = append(report.Docs, doc)
		report.Summary.DocsScanned++
		if len(mentioned) > 0 || len(covering) > 0 {
			report.Summary.DocsWithBacklogCoverage++
		}
		covered := unionStrings(mentioned, covering)
		for _, topic := range coordinationBacklogTopics {
			if !docHasTopic(text, topic) {
				continue
			}
			if topic.ConsumerLesson {
				report.Summary.ConsumerLessons++
				report.Summary.LegacyConsumerLessons++
			}
			if !hasAnyTaskID(covered, topic.TaskIDs) && !anyTaskExists(taskByID, topic.TaskIDs) {
				report.Findings = append(report.Findings, DocsBacklogFinding{
					Kind:        "doc_only_capability",
					Severity:    "warning",
					DocPath:     docPath,
					Topic:       topic.Name,
					Reason:      "coordination design topic appears in docs but no matching Fairway backlog task id exists in runtime state",
					Related:     append([]string{}, topic.TaskIDs...),
					Recommended: "add or import the corresponding Fairway backlog task, or record why the topic is intentionally documentation-only",
				})
				report.Summary.DocOnlyCapabilities++
			}
		}
		if len(commands) > 0 && len(covered) == 0 {
			for _, command := range commands {
				report.Findings = append(report.Findings, DocsBacklogFinding{
					Kind:        "command_example_uncovered",
					Severity:    "info",
					DocPath:     docPath,
					Command:     command,
					Reason:      "Fairway command example is documented in a coordination doc that is not covered by task source_paths/target_paths or explicit task ids",
					Recommended: "link the doc to a Fairway task source_paths/target_paths entry or cite the task id near the command example",
				})
				report.Summary.CommandExamplesUncovered++
			}
		}
	}
	for _, task := range tasks {
		if task.Status != "done" || !isCoordinationBacklogTask(task.Definition.ID) {
			continue
		}
		coveredDocs := append([]string{}, task.Definition.SourcePaths...)
		coveredDocs = append(coveredDocs, task.Definition.TargetPaths...)
		if !taskReferencedInDocs(task.Definition.ID, coveredDocs, report.Docs) {
			report.Findings = append(report.Findings, DocsBacklogFinding{
				Kind:        "completed_task_stale_docs",
				Severity:    "info",
				TaskID:      task.Definition.ID,
				Reason:      "completed coordination task is not referenced by its configured docs or those docs were not scanned",
				Recommended: "update the coordination design or roadmap docs with the implemented command/report name, or remove stale task path metadata",
			})
			report.Summary.StaleCompletedTasks++
		}
	}
	sort.SliceStable(report.Docs, func(i, j int) bool { return report.Docs[i].Path < report.Docs[j].Path })
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Kind != report.Findings[j].Kind {
			return report.Findings[i].Kind < report.Findings[j].Kind
		}
		if report.Findings[i].DocPath != report.Findings[j].DocPath {
			return report.Findings[i].DocPath < report.Findings[j].DocPath
		}
		if report.Findings[i].Topic != report.Findings[j].Topic {
			return report.Findings[i].Topic < report.Findings[j].Topic
		}
		return report.Findings[i].TaskID < report.Findings[j].TaskID
	})
	report.OK = len(report.Findings) == 0
	return report, nil
}

type CILearningReport struct {
	OK                    bool                            `json:"ok"`
	TaskID                string                          `json:"task_id,omitempty"`
	Findings              []CILearningFinding             `json:"findings"`
	NonActionableEvidence []CILearningEvidenceDisposition `json:"non_actionable_evidence,omitempty"`
	Summary               CILearningSummary               `json:"summary"`
	Templates             []LearningArtifact              `json:"learning_artifacts,omitempty"`
}

type CILearningSummary struct {
	FailedEvidence        int `json:"failed_evidence"`
	MissingFollowUps      int `json:"missing_follow_ups"`
	MissedLocalGates      int `json:"missed_local_gates"`
	MissedReviewGates     int `json:"missed_review_gates"`
	CIEnvironmentOnly     int `json:"ci_environment_only"`
	FlakyRunnerOrCache    int `json:"flaky_runner_or_cache"`
	ApprovalGatedBlocker  int `json:"approval_gated_blocker"`
	ArtifactContract      int `json:"artifact_contract"`
	ProviderAPI           int `json:"provider_api"`
	BrowserSurface        int `json:"browser_surface"`
	SetupGate             int `json:"setup_gate"`
	CallbackMissing       int `json:"callback_missing"`
	RedactionFinding      int `json:"redaction_finding"`
	CommitBoundary        int `json:"commit_boundary"`
	UndeliveredHandoff    int `json:"undelivered_handoff"`
	NonActionableEvidence int `json:"non_actionable_evidence"`
	SupersededByPass      int `json:"superseded_by_pass"`
	CoveredByFollowUp     int `json:"covered_by_follow_up"`
	TerminalTaskEvidence  int `json:"terminal_task_evidence"`
}

type CILearningEvidenceDisposition struct {
	TaskID       string `json:"task_id"`
	EvidenceType string `json:"evidence_type"`
	Result       string `json:"result"`
	CommandText  string `json:"command_text"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	RoutingState string `json:"routing_state"`
	Reason       string `json:"reason"`
	FollowUpTask string `json:"follow_up_task,omitempty"`
}

type CILearningFinding struct {
	TaskID                      string   `json:"task_id"`
	TaskTitle                   string   `json:"task_title"`
	EvidenceType                string   `json:"evidence_type"`
	Result                      string   `json:"result"`
	CommandText                 string   `json:"command_text"`
	ArtifactPath                string   `json:"artifact_path,omitempty"`
	FailureClass                string   `json:"failure_class"`
	RootCause                   string   `json:"root_cause"`
	MissedGate                  string   `json:"missed_gate,omitempty"`
	ExpectedLocalReproduction   string   `json:"expected_local_reproduction,omitempty"`
	Owner                       string   `json:"owner,omitempty"`
	OwningDomain                string   `json:"owning_domain,omitempty"`
	OwningLayer                 string   `json:"owning_layer,omitempty"`
	FollowUpTask                string   `json:"follow_up_task,omitempty"`
	FollowUpMissing             bool     `json:"follow_up_missing"`
	RecommendedFollowUpPrefix   string   `json:"recommended_follow_up_prefix,omitempty"`
	RecommendedFollowUpTaskID   string   `json:"recommended_follow_up_task_id,omitempty"`
	RecommendedFollowUpTaskKind string   `json:"recommended_follow_up_task_kind,omitempty"`
	ForbiddenActions            []string `json:"forbidden_actions,omitempty"`
}

type LearningArtifact struct {
	TaskID                    string   `json:"task_id"`
	FailureClass              string   `json:"failure_class"`
	RootCause                 string   `json:"root_cause"`
	MissedGate                string   `json:"missed_gate,omitempty"`
	ExpectedLocalReproduction string   `json:"expected_local_reproduction,omitempty"`
	Owner                     string   `json:"owner,omitempty"`
	OwningDomain              string   `json:"owning_domain,omitempty"`
	OwningLayer               string   `json:"owning_layer,omitempty"`
	FollowUpTask              string   `json:"follow_up_task,omitempty"`
	FollowUpTaskKind          string   `json:"follow_up_task_kind,omitempty"`
	ForbiddenActions          []string `json:"forbidden_actions,omitempty"`
	Markdown                  string   `json:"markdown"`
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
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	evidenceByTask, err := s.EvidenceByTaskIDs(ctx, taskIDs)
	if err != nil {
		return CILearningReport{}, err
	}
	reviewsByTask, err := s.ReviewsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return CILearningReport{}, err
	}
	allFollowUps := findLearningFollowUps(taskByID)
	report := CILearningReport{OK: true, TaskID: opts.TaskID}
	for _, task := range tasks {
		evidence := evidenceByTask[task.Definition.ID]
		reviews := reviewsByTask[task.Definition.ID]
		for i, ev := range evidence {
			if !isFailedPipelineEvidence(ev) {
				continue
			}
			report.Summary.FailedEvidence++
			if disposition, ok := nonActionableLearningEvidence(cfg, task, ev, evidence[i+1:], allFollowUps); ok {
				report.NonActionableEvidence = append(report.NonActionableEvidence, disposition)
				report.Summary.NonActionableEvidence++
				switch disposition.RoutingState {
				case "superseded_by_pass":
					report.Summary.SupersededByPass++
				case "follow_up_exists":
					report.Summary.CoveredByFollowUp++
				case "source_task_terminal":
					report.Summary.TerminalTaskEvidence++
				}
				continue
			}
			finding := classifyLearningFailure(cfg, task, ev, reviews, allFollowUps)
			report.Findings = append(report.Findings, finding)
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
			case "artifact_contract":
				report.Summary.ArtifactContract++
			case "provider_api":
				report.Summary.ProviderAPI++
			case "browser_surface":
				report.Summary.BrowserSurface++
			case "setup_gate":
				report.Summary.SetupGate++
			case "callback_missing":
				report.Summary.CallbackMissing++
			case "redaction_finding":
				report.Summary.RedactionFinding++
			case "commit_boundary":
				report.Summary.CommitBoundary++
			case "undelivered_handoff":
				report.Summary.UndeliveredHandoff++
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
	sort.SliceStable(report.NonActionableEvidence, func(i, j int) bool {
		if report.NonActionableEvidence[i].TaskID != report.NonActionableEvidence[j].TaskID {
			return report.NonActionableEvidence[i].TaskID < report.NonActionableEvidence[j].TaskID
		}
		if report.NonActionableEvidence[i].RoutingState != report.NonActionableEvidence[j].RoutingState {
			return report.NonActionableEvidence[i].RoutingState < report.NonActionableEvidence[j].RoutingState
		}
		return report.NonActionableEvidence[i].CommandText < report.NonActionableEvidence[j].CommandText
	})
	report.OK = len(report.Findings) == 0
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

var fairwayCommandPattern = regexp.MustCompile("`(fairway [^`\\n]+)`")

func fairwayCommandExamples(text string) []string {
	seen := map[string]bool{}
	for _, match := range fairwayCommandPattern.FindAllStringSubmatch(text, -1) {
		command := strings.TrimSpace(match[1])
		if command != "" {
			seen[command] = true
		}
	}
	var out []string
	for command := range seen {
		out = append(out, command)
	}
	sort.Strings(out)
	return out
}

func docTopics(text string) []string {
	var topics []string
	for _, topic := range coordinationBacklogTopics {
		if docHasTopic(text, topic) {
			topics = append(topics, topic.Name)
		}
	}
	sort.Strings(topics)
	return topics
}

func docHasTopic(text string, topic docsBacklogTopic) bool {
	lower := strings.ToLower(text)
	for _, term := range topic.Terms {
		if strings.Contains(lower, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func unionStrings(groups ...[]string) []string {
	seen := map[string]bool{}
	for _, group := range groups {
		for _, value := range group {
			if value != "" {
				seen[value] = true
			}
		}
	}
	var out []string
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hasAnyTaskID(values, candidates []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, candidate := range candidates {
		if seen[candidate] {
			return true
		}
	}
	return false
}

func anyTaskExists(tasks map[string]store.Task, taskIDs []string) bool {
	for _, taskID := range taskIDs {
		if _, ok := tasks[taskID]; ok {
			return true
		}
	}
	return false
}

func isCoordinationBacklogTask(taskID string) bool {
	for _, topic := range coordinationBacklogTopics {
		for _, candidate := range topic.TaskIDs {
			if taskID == candidate {
				return true
			}
		}
	}
	return false
}

func taskReferencedInDocs(taskID string, paths []string, docs []DocsBacklogDoc) bool {
	pathSet := map[string]bool{}
	for _, pathValue := range paths {
		pathSet[strings.TrimSpace(filepath.ToSlash(pathValue))] = true
	}
	for _, doc := range docs {
		if !pathSet[doc.Path] && !pathSet[strings.TrimSuffix(doc.Path, "/")] {
			continue
		}
		for _, mentioned := range doc.MentionedTasks {
			if mentioned == taskID {
				return true
			}
		}
		for _, covering := range doc.CoveringTasks {
			if covering == taskID {
				return true
			}
		}
	}
	return false
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
	keywords := []string{
		"ci", "pipeline", "deploy", "deployment", "smoke", "uat", "release", "workflow", "github actions", "gitlab",
		"artifact", "schema", "contract", "provider", "4xx", "401", "403", "404", "409", "429", "browser", "chrome",
		"playwright", "callback", "redirect", "redaction", "secret", "token", "uncommitted", "reviewed files", "handoff",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func nonActionableLearningEvidence(cfg config.Config, task store.Task, ev store.Evidence, later []store.Evidence, followUps map[string]string) (CILearningEvidenceDisposition, bool) {
	disposition := CILearningEvidenceDisposition{
		TaskID:       task.Definition.ID,
		EvidenceType: ev.ArtifactType,
		Result:       ev.Result,
		CommandText:  ev.CommandText,
		ArtifactPath: ev.ArtifactPath,
	}
	if learningEvidenceSupersededByPass(ev, later) {
		disposition.RoutingState = "superseded_by_pass"
		disposition.Reason = "a later passing evidence row covers the same command or artifact route"
		return disposition, true
	}
	if followUp := strings.TrimSpace(followUps[task.Definition.ID]); followUp != "" {
		disposition.RoutingState = "follow_up_exists"
		disposition.Reason = "an existing scoped follow-up task owns the actionable work"
		disposition.FollowUpTask = followUp
		return disposition, true
	}
	if isTerminalStatus(task.Status, cfg.States.Terminal) {
		disposition.RoutingState = "source_task_terminal"
		disposition.Reason = "the source task has a terminal status; inspect its evidence without creating a duplicate routing recommendation"
		return disposition, true
	}
	return CILearningEvidenceDisposition{}, false
}

func learningEvidenceSupersededByPass(failed store.Evidence, later []store.Evidence) bool {
	key := learningEvidenceRouteKey(failed)
	if key == "" {
		return false
	}
	for _, candidate := range later {
		if strings.EqualFold(strings.TrimSpace(candidate.Result), "pass") && learningEvidenceRouteKey(candidate) == key {
			return true
		}
	}
	return false
}

func learningEvidenceRouteKey(ev store.Evidence) string {
	if command := strings.Join(strings.Fields(strings.ToLower(ev.CommandText)), " "); command != "" {
		return "command:" + command
	}
	artifactType := strings.TrimSpace(strings.ToLower(ev.ArtifactType))
	artifactPath := strings.TrimSpace(strings.ToLower(ev.ArtifactPath))
	if artifactType == "" && artifactPath == "" {
		return ""
	}
	return "artifact:" + artifactType + "|" + artifactPath
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
		OwningDomain:              firstNonEmpty(task.Definition.OwningDomain, task.Definition.Role),
		OwningLayer:               task.Definition.OwningLayer,
		RecommendedFollowUpPrefix: learningFollowUpPrefix(text, ev.ArtifactType),
		ForbiddenActions:          knownFailureForbiddenActions(),
	}
	missingReviews := missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)
	switch {
	case containsAny(text, "artifact missing", "missing artifact", "artifact mismatch", "schema mismatch", "invalid artifact", "contract mismatch", "artifact contract"):
		finding.FailureClass = "artifact_contract"
		finding.RootCause = "evidence artifact was missing, mismatched, or failed its schema/contract"
		finding.MissedGate = "artifact contract preflight"
		finding.RecommendedFollowUpPrefix = "HARNESS-FIX"
		finding.RecommendedFollowUpTaskKind = "bugfix"
	case containsAny(text, "provider 4xx", " 4xx", "401", "403", "404", "409", "429", "provider api", "unknown provider behavior"):
		finding.FailureClass = "provider_api"
		finding.RootCause = "provider API behavior or authorization response needs isolated proof"
		finding.MissedGate = "provider API proof"
		finding.RecommendedFollowUpPrefix = "OPS-FIX"
		finding.RecommendedFollowUpTaskKind = "proof"
	case containsAny(text, "browser launch", "chrome launch", "playwright", "browser permission", "browser surface", "headed", "headless"):
		finding.FailureClass = "browser_surface"
		finding.RootCause = "browser execution surface failed before product behavior could be trusted"
		finding.MissedGate = "browser/provider surface capability probe"
		finding.RecommendedFollowUpPrefix = "HARNESS-FIX"
		finding.RecommendedFollowUpTaskKind = "readiness"
	case containsAny(text, "setup gate", "setup failed", "readback failed", "preflight failed", "preflight gate", "missing prerequisite"):
		finding.FailureClass = "setup_gate"
		finding.RootCause = "setup or readback gate failed before the target flow was exercised"
		finding.MissedGate = "setup/readback preflight"
		finding.RecommendedFollowUpPrefix = "OPS-FIX"
		finding.RecommendedFollowUpTaskKind = "task"
	case containsAny(text, "callback missing", "missing callback", "redirect missing", "oidc callback", "webhook missing"):
		finding.FailureClass = "callback_missing"
		finding.RootCause = "expected callback, redirect, or webhook did not arrive"
		finding.MissedGate = "browser-flow callback contract"
		finding.RecommendedFollowUpPrefix = "UAT-BUG"
		finding.RecommendedFollowUpTaskKind = "bug"
	case containsAny(text, "redaction", "leaked secret", "secret leak", "token leak", "credential leak", "unredacted"):
		finding.FailureClass = "redaction_finding"
		finding.RootCause = "evidence or output contained a redaction/privacy finding"
		finding.MissedGate = "redaction guard"
		finding.RecommendedFollowUpPrefix = "OPS-FIX"
		finding.RecommendedFollowUpTaskKind = "guard"
	case containsAny(text, "uncommitted reviewed files", "reviewed files uncommitted", "commit boundary", "merge-ready dirty", "dirty reviewed"):
		finding.FailureClass = "commit_boundary"
		finding.RootCause = "reviewed files were not committed or staged in the expected boundary"
		finding.MissedGate = "commit-boundary closeout"
		finding.RecommendedFollowUpPrefix = "OPS-FIX"
		finding.RecommendedFollowUpTaskKind = "task"
	case containsAny(text, "review handoff not delivered", "undelivered review handoff", "handoff not delivered", "missing thread_steered", "missing notification delivery"):
		finding.FailureClass = "undelivered_handoff"
		finding.RootCause = "Fairway handoff existed without delivered provider/thread notification proof"
		finding.MissedGate = "wait/wake notification delivery"
		finding.RecommendedFollowUpPrefix = "OPS-FIX"
		finding.RecommendedFollowUpTaskKind = "task"
	case ev.Result == "blocked" || containsAny(text, "approval", "manual gate", "permission", "forbidden", "unauthorized", "secret", "credential"):
		finding.FailureClass = "approval_gated_blocker"
		finding.RootCause = "execution was blocked by approval, permission, secret, or credential state"
		finding.MissedGate = "approval or secret readiness gate"
		finding.RecommendedFollowUpTaskKind = "task"
	case containsAny(text, "flaky", "flake", "runner", "cache", "timeout", "rate limit", "network", "connection reset"):
		finding.FailureClass = "flaky_runner_cache"
		finding.RootCause = "failure appears tied to runner, cache, network, or intermittent infrastructure behavior"
		finding.MissedGate = "runner/cache reliability check"
		finding.RecommendedFollowUpTaskKind = "bug"
	case containsAny(text, "ci only", "ci-only", "github actions", "gitlab", "container", "linux", "environment", "env var"):
		finding.FailureClass = "ci_environment_only"
		finding.RootCause = "failure appears specific to CI/deploy environment differences"
		finding.MissedGate = "environment parity check"
		finding.RecommendedFollowUpTaskKind = "bug"
	case len(missingReviews) > 0 || (cfg.Gates.RequireReviewBeforeDone && task.ReviewStatus != "approved"):
		finding.FailureClass = "missed_review_gate"
		finding.RootCause = "failure happened while required review coverage was missing"
		finding.MissedGate = "review domains: " + strings.Join(missingReviews, ", ")
		finding.RecommendedFollowUpTaskKind = "task"
	default:
		finding.FailureClass = "missed_local_gate"
		finding.RootCause = "failure should have been caught by a local verification command before CI/deploy"
		finding.MissedGate = "local verification gate"
		finding.RecommendedFollowUpTaskKind = "bug"
	}
	finding.ExpectedLocalReproduction = expectedLocalReproduction(ev)
	if followUp, ok := followUps[finding.TaskID]; ok {
		finding.FollowUpTask = followUp
	} else {
		finding.FollowUpMissing = true
		finding.RecommendedFollowUpTaskID = finding.RecommendedFollowUpPrefix + "-" + sanitizeFollowUpSuffix(finding.TaskID)
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
		OwningDomain:              finding.OwningDomain,
		OwningLayer:               finding.OwningLayer,
		FollowUpTask:              followUp,
		FollowUpTaskKind:          finding.RecommendedFollowUpTaskKind,
		ForbiddenActions:          append([]string{}, finding.ForbiddenActions...),
	}
	artifact.Markdown = fmt.Sprintf(`# CI/Deploy Learning: %s

- Failure class: %s
- Root cause: %s
- Missed gate: %s
- Expected local reproduction: %s
- Owner: %s
- Owning domain: %s
- Owning layer: %s
- Follow-up task: %s
- Follow-up task kind: %s
- Forbidden until reviewed: %s
`, artifact.TaskID, artifact.FailureClass, artifact.RootCause, artifact.MissedGate, artifact.ExpectedLocalReproduction, artifact.Owner, artifact.OwningDomain, artifact.OwningLayer, artifact.FollowUpTask, artifact.FollowUpTaskKind, strings.Join(artifact.ForbiddenActions, ", "))
	return artifact
}

func knownFailureForbiddenActions() []string {
	return []string{
		"live execution",
		"production mutation",
		"credential action",
		"approval acceptance",
		"merge/deploy",
	}
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
