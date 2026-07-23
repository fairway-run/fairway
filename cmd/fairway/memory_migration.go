package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/subashram/fairway/internal/config"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/knowledge"
	"github.com/subashram/fairway/internal/memorymigration"
	"github.com/subashram/fairway/internal/store"
)

type memoryImportResult struct {
	TrackID    string                   `json:"track_id"`
	Applied    bool                     `json:"applied"`
	AppendOnly bool                     `json:"append_only"`
	Existing   bool                     `json:"existing"`
	Document   memorymigration.Document `json:"document"`
	Warnings   []string                 `json:"warnings,omitempty"`
	Memory     *store.TrackMemory       `json:"memory,omitempty"`
}

func cmdMemoryImport(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("memory import", flag.ContinueOnError)
	file := fs.String("file", "", "repo-local tmp-ux memory Markdown file")
	track := fs.String("track", "", "target track id")
	apply := fs.Bool("apply", false, "apply the bounded proposal to track memory")
	title := fs.String("title", "", "override extracted title")
	purpose := fs.String("purpose", "", "override extracted purpose")
	mode := fs.String("operating-mode", "", "override extracted operating mode")
	scope := fs.String("active-scope", "", "override extracted active scope")
	objective := fs.String("current-objective", "", "override extracted current objective")
	owner := fs.String("owner", "", "override extracted owner")
	reviewBy := fs.String("review-by", "", "override extracted review date")
	checkpointIDs := multiInt64Flag{}
	evidenceIDs := multiInt64Flag{}
	reviewIDs := multiInt64Flag{}
	fs.Var(&checkpointIDs, "source-checkpoint-id", "source checkpoint id")
	fs.Var(&evidenceIDs, "source-evidence-id", "source evidence id")
	fs.Var(&reviewIDs, "source-review-id", "source review id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected memory import arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*file) == "" || strings.TrimSpace(*track) == "" {
		return errors.New("memory import requires --file and --track")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, root string, s *store.Store) error {
		document, err := memorymigration.Load(root, *file)
		if err != nil {
			return err
		}
		proposal := document.Proposal
		override(&proposal.Title, *title)
		override(&proposal.Purpose, *purpose)
		override(&proposal.OperatingMode, *mode)
		override(&proposal.ActiveScope, *scope)
		override(&proposal.CurrentObjective, *objective)
		override(&proposal.Owner, *owner)
		override(&proposal.ReviewBy, *reviewBy)
		proposal.SourceCheckpointIDs = appendUniqueInt64(proposal.SourceCheckpointIDs, []int64(checkpointIDs)...)
		proposal.SourceEvidenceIDs = appendUniqueInt64(proposal.SourceEvidenceIDs, []int64(evidenceIDs)...)
		proposal.SourceReviewIDs = appendUniqueInt64(proposal.SourceReviewIDs, []int64(reviewIDs)...)
		if err := memorymigration.ValidateProposal(proposal); err != nil {
			return err
		}
		document.Proposal = proposal
		existingMemory, existingErr := s.TrackMemory(ctx, *track)
		existing := existingErr == nil
		if existingErr != nil && !errors.Is(existingErr, store.ErrNotFound) {
			return existingErr
		}
		if existing {
			if *apply && (existingMemory.Disposition == "archived" || existingMemory.Disposition == "superseded") {
				return fmt.Errorf("cannot import into %s track memory %q", existingMemory.Disposition, existingMemory.TrackID)
			}
			preserved := preserveExistingMemoryScalars(&proposal, existingMemory)
			document.Proposal = proposal
			document.Warnings = append(document.Warnings, preserved...)
		}
		result := memoryImportResult{TrackID: strings.TrimSpace(*track), Applied: false, AppendOnly: true, Existing: existing, Document: document}
		if !*apply {
			result.Warnings = importPreviewWarnings(proposal, existing)
			return printMemoryImportResult(opts, result)
		}
		updated, err := s.UpsertTrackMemory(ctx, proposalToTrackMemory(*track, proposal), true)
		if err != nil {
			return fmt.Errorf("apply memory import: %w", err)
		}
		result.Applied = true
		result.Memory = &updated
		return printMemoryImportResult(opts, result)
	})
}

func cmdMemoryCoverage(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("memory coverage", flag.ContinueOnError)
	rootFlag := fs.String("root", "tmp-ux", "repo-local tmp-ux inventory root")
	track := fs.String("track", "", "restrict coverage to one track")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected memory coverage arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, root string, s *store.Store) error {
		documents, err := memorymigration.Discover(root, *rootFlag)
		if err != nil {
			return err
		}
		memories, err := s.TrackMemories(ctx)
		if err != nil {
			return err
		}
		projected := projectMemories(memories)
		rows := make([]memorymigration.Coverage, 0, len(documents))
		covered := 0
		for _, document := range documents {
			row := memorymigration.AssessCoverage(document, projected, *track)
			if row.Status == "covered" {
				covered++
			}
			rows = append(rows, row)
		}
		report := struct {
			Complete bool                       `json:"complete"`
			ReadOnly bool                       `json:"read_only"`
			Files    int                        `json:"files"`
			Covered  int                        `json:"covered"`
			Rows     []memorymigration.Coverage `json:"rows"`
		}{Complete: covered == len(rows), ReadOnly: true, Files: len(rows), Covered: covered, Rows: rows}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("memory_coverage: %t\nread_only: true\nfiles: %d\ncovered: %d\n", report.Complete, report.Files, report.Covered)
		for _, row := range rows {
			fmt.Printf("- path=%s status=%s track=%s disposition=%s represented=%d/%d reason=%s\n", row.Path, row.Status, firstNonEmpty(row.TrackID, "none"), firstNonEmpty(row.Disposition, "none"), row.RepresentedFacts, row.ExtractedFacts, row.Reason)
		}
		return nil
	})
}

type memoryColdStart struct {
	Schema          string                 `json:"schema"`
	Packet          memoryPacket           `json:"packet"`
	Knowledge       *knowledge.QueryPacket `json:"knowledge,omitempty"`
	KnowledgeBudget int                    `json:"knowledge_budget_bytes,omitempty"`
	Git             coldStartGit           `json:"git"`
	Warnings        []string               `json:"warnings,omitempty"`
	Bounded         bool                   `json:"bounded"`
	ReadOnly        bool                   `json:"read_only"`
}

type coldStartGit struct {
	Branch       string   `json:"branch,omitempty"`
	Commit       string   `json:"commit,omitempty"`
	Dirty        bool     `json:"dirty"`
	ChangedFiles []string `json:"changed_files,omitempty"`
}

const (
	coldStartStringBytes = 512
	coldStartTotalBytes  = 64 * 1024
)

func cmdMemoryColdStart(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("memory cold-start", flag.ContinueOnError)
	track := fs.String("track", "", "track id")
	forProvider := fs.String("for", "", "target provider or surface")
	olderThan := fs.Duration("older-than", 24*time.Hour, "age that creates a stale warning")
	knowledgeTopics := multiStringFlag{}
	fs.Var(&knowledgeTopics, "knowledge-topic", "optional relevant engineering knowledge topic; repeatable")
	knowledgeBudget := fs.Int("knowledge-budget-bytes", 12*1024, "separate optional engineering knowledge budget")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected memory cold-start arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*track) == "" {
		return errors.New("memory cold-start requires --track")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		memory, err := s.TrackMemory(ctx, *track)
		if err != nil {
			return err
		}
		if memory.Disposition == "archived" || memory.Disposition == "superseded" {
			return fmt.Errorf("track memory %q is %s and excluded from cold-start packets", memory.TrackID, memory.Disposition)
		}
		if err := memorymigration.ValidateProposal(trackMemoryToProposal(memory)); err != nil {
			return fmt.Errorf("track memory is unsafe to render: %w", err)
		}
		tasks, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		sessions, err := s.Sessions(ctx, false)
		if err != nil {
			return err
		}
		checkpoints, err := s.Checkpoints(ctx, "", true)
		if err != nil {
			return err
		}
		rootTaskIDs, err := memorySourceTaskIDs(ctx, s, memory)
		if err != nil {
			return fmt.Errorf("resolve cold-start source facts: %w", err)
		}
		tasks, sessions, checkpoints = trackScopedColdStartFacts(memory, rootTaskIDs, tasks, sessions, checkpoints)
		packet, err := boundMemoryPacket(buildMemoryPacket(memory, *forProvider, tasks, sessions, checkpoints, cfg.States.Terminal))
		if err != nil {
			return err
		}
		warnings := coldStartWarnings(memory, time.Now().UTC(), *olderThan)
		gitReadback, err := boundColdStartGit(coldStartGit{Branch: fairwaygit.CurrentBranch(root), Commit: fairwaygit.LastCommit(root)})
		if err != nil {
			return err
		}
		if status, statusErr := fairwaygit.Check(root, ""); statusErr == nil {
			gitReadback.Dirty = status.Dirty
			gitReadback.ChangedFiles, err = boundStrings(status.ChangedFiles, 12)
			if err != nil {
				return err
			}
		} else {
			warnings = append(warnings, "git posture unavailable")
		}
		warnings, err = boundStrings(uniqueSorted(warnings), 12)
		if err != nil {
			return err
		}
		report := memoryColdStart{Schema: "fairway.memory-cold-start.v1", Packet: packet, Git: gitReadback, Warnings: warnings, Bounded: true, ReadOnly: true}
		if len(knowledgeTopics) > 0 {
			query, queryErr := knowledge.Query(knowledge.QueryOptions{
				Options: knowledgeStoreOptions(ctx, cfg, root, "", s),
				Topic:   strings.Join(knowledgeTopics, " "), TaskID: memory.TrackID,
				TaskTerms:   []string{memory.Title, memory.Purpose, memory.ActiveScope, memory.CurrentObjective},
				BudgetBytes: *knowledgeBudget,
			})
			if queryErr != nil {
				return fmt.Errorf("compose cold-start knowledge: %w", queryErr)
			}
			query, queryErr = finalizeColdStartKnowledge(query, memory, *knowledgeBudget)
			if queryErr != nil {
				return queryErr
			}
			report.Knowledge = &query
			report.KnowledgeBudget = *knowledgeBudget
		}
		rendered, err := json.Marshal(report)
		if err != nil {
			return fmt.Errorf("marshal bounded cold-start packet: %w", err)
		}
		if err := memorymigration.ValidateRendered(rendered, coldStartTotalBytes+maxInt(report.KnowledgeBudget, 0)); err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("# Fairway Cold Start: %s\n\nschema: %s\nbounded: true\nread_only: true\n", memory.TrackID, report.Schema)
		fmt.Printf("branch: %s\ncommit: %s\ndirty: %t\n", firstNonEmpty(report.Git.Branch, "unknown"), firstNonEmpty(report.Git.Commit, "unknown"), report.Git.Dirty)
		printStringSection("Warnings", report.Warnings)
		fmt.Println()
		printMemoryPacket(report.Packet)
		if report.Knowledge != nil {
			fmt.Println()
			printKnowledgePacket(*report.Knowledge)
		}
		return nil
	})
}

func dedupeKnowledgeSourcesWithMemory(sources []knowledge.QuerySource, memory store.TrackMemory) []knowledge.QuerySource {
	memoryEvidence := map[string]bool{}
	for _, id := range memory.SourceEvidenceIDs {
		memoryEvidence[fmt.Sprintf("fairway:evidence:%d", id)] = true
	}
	result := append([]knowledge.QuerySource{}, sources...)
	for index := range result {
		if memoryEvidence[result[index].Key] {
			result[index].MemoryReferenced = true
		}
	}
	return result
}

func finalizeColdStartKnowledge(query knowledge.QueryPacket, memory store.TrackMemory, budget int) (knowledge.QueryPacket, error) {
	query.Sources = dedupeKnowledgeSourcesWithMemory(query.Sources, memory)
	if err := knowledge.FinalizeQueryPacket(&query); err != nil {
		return knowledge.QueryPacket{}, fmt.Errorf("finalize cold-start knowledge packet: %w", err)
	}
	if query.Bytes > budget {
		return knowledge.QueryPacket{}, errors.New("composed cold-start knowledge exceeds its separate byte budget")
	}
	return query, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cmdMemoryRetireFile(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("memory retire-file", flag.ContinueOnError)
	file := fs.String("file", "", "repo-local tmp-ux memory Markdown file")
	track := fs.String("track", "", "authoritative track id")
	reason := fs.String("reason", "", "retirement reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected memory retire-file arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*file) == "" || strings.TrimSpace(*track) == "" || strings.TrimSpace(*reason) == "" {
		return errors.New("memory retire-file requires --file, --track, and --reason")
	}
	if err := memorymigration.ValidateProposal(memorymigration.Proposal{Purpose: strings.TrimSpace(*reason)}); err != nil {
		return errors.New("memory retirement reason contains prohibited secret-like material")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, root string, s *store.Store) error {
		document, err := memorymigration.Load(root, *file)
		if err != nil {
			return err
		}
		memory, err := s.TrackMemory(ctx, *track)
		if err != nil {
			return err
		}
		plan := memorymigration.PlanRetirement(document, projectMemory(memory), *reason)
		if opts.JSON {
			if err := printJSON(plan); err != nil {
				return err
			}
		} else {
			fmt.Printf("memory_retire_file: %t\nread_only: true\ndeletes_source: false\npath: %s\nsha256: %s\ntrack: %s\ndisposition: %s\ncoverage: %s\nreason: %s\nevidence: %s\n", plan.Eligible, plan.Path, plan.SHA256, plan.TrackID, plan.Disposition, plan.CoverageStatus, plan.Reason, plan.SuggestedEvidence)
		}
		if !plan.Eligible {
			return errors.New("legacy memory file is not eligible for retirement; require exact coverage, source facts, and archived or superseded disposition")
		}
		return nil
	})
}

func proposalToTrackMemory(track string, proposal memorymigration.Proposal) store.TrackMemory {
	return store.TrackMemory{TrackID: strings.TrimSpace(track), Title: proposal.Title, Purpose: proposal.Purpose, OperatingMode: proposal.OperatingMode, ActiveScope: proposal.ActiveScope, CurrentObjective: proposal.CurrentObjective, Decisions: proposal.Decisions, Blockers: proposal.Blockers, OpenQuestions: proposal.OpenQuestions, NextActions: proposal.NextActions, SourceCheckpointIDs: proposal.SourceCheckpointIDs, SourceEvidenceIDs: proposal.SourceEvidenceIDs, SourceReviewIDs: proposal.SourceReviewIDs, Owner: proposal.Owner, ReviewBy: proposal.ReviewBy, Disposition: "active"}
}

func trackMemoryToProposal(memory store.TrackMemory) memorymigration.Proposal {
	return memorymigration.Proposal{Title: memory.Title, Purpose: memory.Purpose, OperatingMode: memory.OperatingMode, ActiveScope: memory.ActiveScope, CurrentObjective: memory.CurrentObjective, Decisions: memory.Decisions, Blockers: memory.Blockers, OpenQuestions: memory.OpenQuestions, NextActions: memory.NextActions, SourceCheckpointIDs: memory.SourceCheckpointIDs, SourceEvidenceIDs: memory.SourceEvidenceIDs, SourceReviewIDs: memory.SourceReviewIDs, Owner: memory.Owner, ReviewBy: memory.ReviewBy}
}

func projectMemory(memory store.TrackMemory) memorymigration.Memory {
	return memorymigration.Memory{TrackID: memory.TrackID, Title: memory.Title, Purpose: memory.Purpose, OperatingMode: memory.OperatingMode, ActiveScope: memory.ActiveScope, CurrentObjective: memory.CurrentObjective, Decisions: memory.Decisions, Blockers: memory.Blockers, OpenQuestions: memory.OpenQuestions, NextActions: memory.NextActions, Owner: memory.Owner, ReviewBy: memory.ReviewBy, Disposition: memory.Disposition, SourceFactCount: len(memory.SourceCheckpointIDs) + len(memory.SourceEvidenceIDs) + len(memory.SourceReviewIDs), SourceCheckpointIDs: memory.SourceCheckpointIDs, SourceEvidenceIDs: memory.SourceEvidenceIDs, SourceReviewIDs: memory.SourceReviewIDs}
}

func projectMemories(memories []store.TrackMemory) []memorymigration.Memory {
	out := make([]memorymigration.Memory, 0, len(memories))
	for _, memory := range memories {
		out = append(out, projectMemory(memory))
	}
	return out
}

func override(target *string, value string) {
	if strings.TrimSpace(value) != "" {
		*target = strings.TrimSpace(value)
	}
}

func appendUniqueInt64(existing []int64, values ...int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(existing)+len(values))
	for _, value := range append(append([]int64{}, existing...), values...) {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func importPreviewWarnings(proposal memorymigration.Proposal, existing bool) []string {
	warnings := []string{"preview only; rerun with --apply to write bounded fields", "raw legacy content is not stored"}
	if !existing && strings.TrimSpace(proposal.Owner) == "" {
		warnings = append(warnings, "new active track memory requires owner")
	}
	if !existing && strings.TrimSpace(proposal.ReviewBy) == "" {
		warnings = append(warnings, "new active track memory requires review date")
	}
	if !existing && len(proposal.SourceCheckpointIDs)+len(proposal.SourceEvidenceIDs)+len(proposal.SourceReviewIDs) == 0 {
		warnings = append(warnings, "new active track memory requires at least one existing Fairway source fact")
	}
	return warnings
}

func preserveExistingMemoryScalars(proposal *memorymigration.Proposal, existing store.TrackMemory) []string {
	warnings := []string{}
	fields := []struct {
		name     string
		proposed *string
		existing string
	}{
		{"title", &proposal.Title, existing.Title},
		{"purpose", &proposal.Purpose, existing.Purpose},
		{"operating_mode", &proposal.OperatingMode, existing.OperatingMode},
		{"active_scope", &proposal.ActiveScope, existing.ActiveScope},
		{"current_objective", &proposal.CurrentObjective, existing.CurrentObjective},
		{"owner", &proposal.Owner, existing.Owner},
		{"review_by", &proposal.ReviewBy, existing.ReviewBy},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.existing) == "" || strings.TrimSpace(*field.proposed) == "" || strings.TrimSpace(field.existing) == strings.TrimSpace(*field.proposed) {
			continue
		}
		*field.proposed = ""
		warnings = append(warnings, "existing authoritative "+field.name+" was preserved")
	}
	return warnings
}

func printMemoryImportResult(opts globalOptions, result memoryImportResult) error {
	if opts.JSON {
		return printJSON(result)
	}
	fmt.Printf("memory_import: %s\ntrack: %s\napplied: %t\nappend_only: true\nexisting: %t\nsource_sha256: %s\nraw_omitted: true\n", map[bool]string{true: "applied", false: "preview"}[result.Applied], result.TrackID, result.Applied, result.Existing, result.Document.SHA256)
	printStringSection("Warnings", append(result.Document.Warnings, result.Warnings...))
	if result.Memory != nil {
		fmt.Printf("updated_at: %s\n", result.Memory.UpdatedAt)
	}
	return nil
}

func memorySourceTaskIDs(ctx context.Context, s *store.Store, memory store.TrackMemory) ([]string, error) {
	kinds := []struct {
		kind string
		ids  []int64
	}{
		{kind: "checkpoint", ids: memory.SourceCheckpointIDs},
		{kind: "evidence", ids: memory.SourceEvidenceIDs},
		{kind: "review", ids: memory.SourceReviewIDs},
	}
	seen := map[string]bool{}
	result := []string{}
	for _, group := range kinds {
		for _, id := range group.ids {
			taskID, err := s.SourceFactTaskID(ctx, group.kind, id)
			if err != nil {
				return nil, fmt.Errorf("%s %d: %w", group.kind, id, err)
			}
			if !seen[taskID] {
				seen[taskID] = true
				result = append(result, taskID)
			}
		}
	}
	sort.Strings(result)
	return result, nil
}

func trackScopedColdStartFacts(memory store.TrackMemory, sourceTaskIDs []string, tasks []store.Task, sessions []store.Session, checkpoints []store.Checkpoint) ([]store.Task, []store.Session, []store.Checkpoint) {
	roots := map[string]bool{}
	for _, task := range tasks {
		if task.Definition.ID == memory.TrackID {
			roots[task.Definition.ID] = true
		}
	}
	for _, taskID := range sourceTaskIDs {
		if strings.TrimSpace(taskID) != "" {
			roots[taskID] = true
		}
	}
	parents := make(map[string]string, len(tasks))
	byID := make(map[string]store.Task, len(tasks))
	for _, task := range tasks {
		parents[task.Definition.ID] = task.Definition.ParentID
		byID[task.Definition.ID] = task
	}
	members := map[string]bool{}
	changed := true
	for changed {
		changed = false
		for _, task := range tasks {
			if !members[task.Definition.ID] && taskInTrack(task.Definition.ID, parents, roots) {
				members[task.Definition.ID] = true
				changed = true
			}
		}
		for taskID := range members {
			for _, dependency := range byID[taskID].Definition.Dependencies {
				if !roots[dependency] {
					roots[dependency] = true
					changed = true
				}
			}
		}
	}
	scopedTasks := make([]store.Task, 0, len(members))
	for _, task := range tasks {
		if members[task.Definition.ID] {
			scopedTasks = append(scopedTasks, task)
		}
	}
	scopedSessions := make([]store.Session, 0)
	for _, session := range sessions {
		if members[session.TaskID] {
			scopedSessions = append(scopedSessions, session)
		}
	}
	scopedCheckpoints := make([]store.Checkpoint, 0)
	for _, checkpoint := range checkpoints {
		if members[checkpoint.TaskID] {
			scopedCheckpoints = append(scopedCheckpoints, checkpoint)
		}
	}
	return scopedTasks, scopedSessions, scopedCheckpoints
}

func taskInTrack(taskID string, parents map[string]string, roots map[string]bool) bool {
	seen := map[string]bool{}
	for cursor := strings.TrimSpace(taskID); cursor != ""; cursor = strings.TrimSpace(parents[cursor]) {
		if roots[cursor] {
			return true
		}
		if seen[cursor] {
			return false
		}
		seen[cursor] = true
	}
	return false
}

func boundMemoryPacket(packet memoryPacket) (memoryPacket, error) {
	var err error
	packet.Track.TrackID, err = boundString(packet.Track.TrackID)
	if err != nil {
		return memoryPacket{}, err
	}
	scalars := []*string{
		&packet.Track.Title, &packet.Track.Purpose, &packet.Track.OperatingMode,
		&packet.Track.ActiveScope, &packet.Track.CurrentObjective, &packet.Track.Owner,
		&packet.Track.ReviewBy, &packet.Track.Disposition, &packet.Track.PromotionTarget,
		&packet.Track.CanonicalCommit, &packet.Track.SupersededByTrackID, &packet.Track.UpdatedAt,
		&packet.ForProvider,
	}
	for _, scalar := range scalars {
		*scalar, err = boundString(*scalar)
		if err != nil {
			return memoryPacket{}, err
		}
	}
	for target, limit := range map[*[]string]int{
		&packet.Track.Decisions: 12, &packet.Track.Blockers: 12,
		&packet.Track.OpenQuestions: 12, &packet.Track.NextActions: 12,
		&packet.ActiveTasks: 12, &packet.ActiveSessions: 8,
		&packet.Blockers: 12, &packet.NextActions: 12, &packet.Checkpoints: 8,
	} {
		*target, err = boundStrings(*target, limit)
		if err != nil {
			return memoryPacket{}, err
		}
	}
	packet.Track.SourceCheckpointIDs = boundInt64s(packet.Track.SourceCheckpointIDs, 24)
	packet.Track.SourceEvidenceIDs = boundInt64s(packet.Track.SourceEvidenceIDs, 24)
	packet.Track.SourceReviewIDs = boundInt64s(packet.Track.SourceReviewIDs, 24)
	return packet, nil
}

func boundColdStartGit(readback coldStartGit) (coldStartGit, error) {
	var err error
	readback.Branch, err = boundString(readback.Branch)
	if err != nil {
		return coldStartGit{}, err
	}
	readback.Commit, err = boundString(readback.Commit)
	if err != nil {
		return coldStartGit{}, err
	}
	return readback, nil
}

func boundStrings(values []string, limit int) ([]string, error) {
	if len(values) > limit {
		values = values[:limit]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		bounded, err := boundString(value)
		if err != nil {
			return nil, err
		}
		out = append(out, bounded)
	}
	return out, nil
}

func boundString(value string) (string, error) {
	if err := memorymigration.ValidateSafeText(value); err != nil {
		return "", errors.New("cold-start packet contains prohibited secret-like material")
	}
	if len(value) <= coldStartStringBytes {
		return value, nil
	}
	value = value[:coldStartStringBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value), nil
}

func boundInt64s(values []int64, limit int) []int64 {
	if len(values) > limit {
		values = values[:limit]
	}
	return append([]int64{}, values...)
}

func coldStartWarnings(memory store.TrackMemory, now time.Time, olderThan time.Duration) []string {
	if olderThan <= 0 {
		olderThan = 24 * time.Hour
	}
	warnings := []string{}
	updated, err := time.Parse(time.RFC3339Nano, memory.UpdatedAt)
	if err != nil || !updated.After(now.Add(-olderThan)) {
		warnings = append(warnings, "track memory is stale and should be refreshed")
	}
	reviewAt, err := parseMemoryReviewDate(memory.ReviewBy)
	if err != nil {
		warnings = append(warnings, "track memory has no valid review date")
	} else if !reviewAt.After(now) {
		warnings = append(warnings, "track memory review date has elapsed")
	}
	if strings.TrimSpace(memory.Owner) == "" {
		warnings = append(warnings, "track memory has no accountable owner")
	}
	if len(memory.SourceCheckpointIDs)+len(memory.SourceEvidenceIDs)+len(memory.SourceReviewIDs) == 0 {
		warnings = append(warnings, "track memory has no Fairway source facts")
	}
	if memory.Disposition == "promote" {
		warnings = append(warnings, "track memory has unresolved promotion debt")
	}
	return warnings
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
