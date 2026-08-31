package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/knowledge"
	"github.com/subashram/fairway/internal/store"
)

func cmdKnowledge(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("knowledge", "init|status|lint|ingest|capture|index|query|export|import|promote ...")
		return nil
	}
	switch args[0] {
	case "init":
		return cmdKnowledgeInit(opts, args[1:])
	case "status", "lint":
		return cmdKnowledgeReport(ctx, opts, args[0], args[1:])
	case "ingest":
		return cmdKnowledgeIngest(ctx, opts, args[1:])
	case "capture":
		return cmdKnowledgeCapture(ctx, opts, args[1:])
	case "index":
		return cmdKnowledgeIndex(ctx, opts, args[1:])
	case "query":
		return cmdKnowledgeQuery(ctx, opts, args[1:])
	case "export":
		return cmdKnowledgeExport(ctx, opts, args[1:])
	case "import":
		return cmdKnowledgeImport(ctx, opts, args[1:])
	case "promote":
		return cmdKnowledgePromote(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown knowledge subcommand %q", args[0])
	}
}

func cmdKnowledgeExport(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("knowledge export", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	output := fs.String("output", "", "project-relative portable bundle path")
	includeIndex := fs.Bool("include-index", false, "include the optional disposable semantic index")
	indexPath := fs.String("semantic-index", ".fairway/knowledge-index.json", "project-relative derived semantic index")
	apply := fs.Bool("apply", false, "write the portable bundle")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 || strings.TrimSpace(*output) == "" {
		return errors.New("knowledge export requires --output and no positional arguments")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, projectRoot string, s *store.Store) error {
		result, err := knowledge.ExportBundle(knowledge.BundleExportOptions{Options: knowledgeStoreOptions(ctx, cfg, projectRoot, strings.TrimSpace(*rootFlag), s), OutputPath: *output, DerivedIndexPath: *indexPath, IncludeIndex: *includeIndex, Apply: *apply})
		if err != nil {
			return err
		}
		return printKnowledgeBundleResult(opts, "export", result)
	})
}

func cmdKnowledgeImport(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("knowledge import", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	bundle := fs.String("bundle", "", "project-relative portable bundle path")
	apply := fs.Bool("apply", false, "write untrusted draft pages under imports/")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 || strings.TrimSpace(*bundle) == "" {
		return errors.New("knowledge import requires --bundle and no positional arguments")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, projectRoot string, s *store.Store) error {
		result, err := knowledge.ImportBundle(knowledge.BundleImportOptions{Options: knowledgeStoreOptions(ctx, cfg, projectRoot, strings.TrimSpace(*rootFlag), s), BundlePath: *bundle, Apply: *apply})
		if err != nil {
			return err
		}
		return printKnowledgeBundleResult(opts, "import", result)
	})
}

func printKnowledgeBundleResult(opts globalOptions, operation string, result knowledge.BundleResult) error {
	if opts.JSON {
		return printJSON(result)
	}
	fmt.Printf("knowledge_%s: %s\nbundle: %s\nbundle_id: %s\nfiles: %d\npages: %d\nexternal_untrusted: true\n", operation, map[bool]string{true: "applied", false: "preview"}[result.Applied], result.BundlePath, firstNonEmpty(result.BundleID, "none"), result.Files, result.Pages)
	for _, change := range result.Changes {
		fmt.Printf("- action=%s path=%s bytes=%d sha256=%s\n", change.Action, change.Path, change.Bytes, change.SHA256)
	}
	if !result.Applied {
		fmt.Println("next: inspect checksums and untrusted draft changes, then rerun with --apply when appropriate")
	}
	return nil
}

func cmdKnowledgeIndex(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("knowledge index", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	indexPath := fs.String("output", ".fairway/knowledge-index.json", "project-relative derived index path")
	embedCommand := fs.String("embed-command", "", "explicit local embedding adapter executable")
	model := fs.String("embedding-model", "", "embedding model identity recorded in the derived index")
	apply := fs.Bool("apply", false, "write the disposable derived index")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected knowledge index arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*embedCommand) == "" || strings.TrimSpace(*model) == "" {
		return errors.New("knowledge index requires --embed-command and --embedding-model")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, projectRoot string, s *store.Store) error {
		result, err := knowledge.BuildSemanticIndex(knowledge.SemanticIndexOptions{
			Options: knowledgeStoreOptions(ctx, cfg, projectRoot, strings.TrimSpace(*rootFlag), s), IndexPath: *indexPath,
			Model: *model, Embed: commandEmbedder(*embedCommand, *model), Apply: *apply,
		})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(result)
		}
		fmt.Printf("knowledge_index: %s\nmodel: %s\npages: %d\n", map[bool]string{true: "applied", false: "preview"}[result.Applied], result.Model, result.Pages)
		for _, change := range result.Changes {
			fmt.Printf("- action=%s path=%s bytes=%d sha256=%s\n", change.Action, change.Path, change.Bytes, change.SHA256)
		}
		return nil
	})
}

func cmdKnowledgeCapture(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("knowledge capture", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	taskID := fs.String("task", "", "completed Fairway task to capture")
	page := fs.String("page", "", "knowledge-root-relative Markdown page path")
	title := fs.String("title", "", "lesson title; defaults to the task title")
	owner := fs.String("owner", "", "accountable page owner")
	reviewBy := fs.String("review-by", "", "page review date in YYYY-MM-DD")
	lesson := fs.String("lesson", "", "short reusable conclusion; defaults from the latest task decision")
	appliesWhen := fs.String("applies-when", "", "bounded reuse condition")
	doesNotApplyWhen := fs.String("does-not-apply-when", "", "bounded exclusion condition")
	apply := fs.Bool("apply", false, "write the reviewed proposal as a normal Git diff")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected knowledge capture arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*taskID) == "" || strings.TrimSpace(*page) == "" || strings.TrimSpace(*owner) == "" || strings.TrimSpace(*reviewBy) == "" {
		return errors.New("knowledge capture requires --task, --page, --owner, and --review-by")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, projectRoot string, s *store.Store) error {
		task, _, _, _, _, err := s.TaskDetail(ctx, strings.TrimSpace(*taskID))
		if err != nil {
			return err
		}
		if !containsString(cfg.States.Terminal, task.Status) {
			return errors.New("knowledge capture requires a terminal task so transient execution state is not retained as a lesson")
		}
		decisions, err := s.TaskDecisions(ctx, task.Definition.ID)
		if err != nil {
			return err
		}
		evidence, err := s.TaskEvidenceFacts(ctx, task.Definition.ID)
		if err != nil {
			return err
		}
		reviews, err := s.TaskReviewFacts(ctx, task.Definition.ID)
		if err != nil {
			return err
		}
		outcomes, err := s.TaskOutcomes(ctx, task.Definition.ID)
		if err != nil {
			return err
		}
		commits, err := s.TaskCommits(ctx, task.Definition.ID)
		if err != nil {
			return err
		}
		facts := captureFacts(decisions, evidence, reviews, outcomes, commits)
		resolvedLesson := strings.TrimSpace(*lesson)
		if resolvedLesson == "" && len(decisions) > 0 {
			latest := decisions[len(decisions)-1]
			resolvedLesson = firstNonEmpty(strings.TrimSpace(latest.Chosen), strings.TrimSpace(latest.Decision))
			if strings.TrimSpace(latest.Reason) != "" {
				resolvedLesson += ": " + strings.TrimSpace(latest.Reason)
			}
		}
		if resolvedLesson == "" {
			return errors.New("knowledge capture requires --lesson when the task has no decision-derived conclusion")
		}
		result, err := knowledge.Capture(knowledge.CaptureOptions{
			Options: knowledgeStoreOptions(ctx, cfg, projectRoot, strings.TrimSpace(*rootFlag), s), TaskID: task.Definition.ID,
			PagePath: *page, Title: firstNonEmpty(strings.TrimSpace(*title), task.Definition.Title), Owner: *owner, ReviewBy: *reviewBy,
			Lesson: resolvedLesson, AppliesWhen: *appliesWhen, DoesNotApplyWhen: *doesNotApplyWhen, Facts: facts, Apply: *apply,
		})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(result)
		}
		fmt.Printf("knowledge_capture: %s\ntask: %s\nsource_revision: %s\nfacts: %d\n", map[bool]string{true: "applied", false: "preview"}[result.Applied], result.TaskID, result.SourceRevision, len(result.Facts))
		for _, change := range result.Changes {
			fmt.Printf("- action=%s path=%s bytes=%d sha256=%s\n", change.Action, change.Path, change.Bytes, change.SHA256)
			fmt.Printf("  preview=%q\n", change.Preview)
		}
		if !result.Applied {
			fmt.Println("next: review the bounded lesson and citations, then rerun with --apply")
		}
		return nil
	})
}

func captureFacts(decisions []store.TaskDecision, evidence []store.TaskEvidenceFact, reviews []store.TaskReviewFact, outcomes []store.TaskOutcome, commits []store.TaskCommit) []knowledge.CaptureFact {
	facts := make([]knowledge.CaptureFact, 0, len(decisions)+len(evidence)+len(reviews)+len(outcomes)+len(commits))
	for _, decision := range decisions {
		facts = append(facts, knowledge.CaptureFact{Kind: "decision", ID: decision.ID, Summary: firstNonEmpty(strings.TrimSpace(decision.Chosen), strings.TrimSpace(decision.Decision), "recorded decision")})
	}
	for _, item := range evidence {
		facts = append(facts, knowledge.CaptureFact{Kind: "evidence", ID: item.ID, Summary: item.Summary})
	}
	for _, item := range reviews {
		facts = append(facts, knowledge.CaptureFact{Kind: "review", ID: item.ID, Summary: "verdict=" + item.Verdict + " domain=" + firstNonEmpty(item.Domain, "none") + " reviewer=" + item.Reviewer})
	}
	for _, item := range outcomes {
		facts = append(facts, knowledge.CaptureFact{Kind: "outcome", ID: item.ID, Summary: fmt.Sprintf("kind=%s related_task=%s transition_id=%d", item.Kind, firstNonEmpty(item.RelatedTaskID, "none"), item.TransitionID)})
	}
	for _, item := range commits {
		facts = append(facts, knowledge.CaptureFact{Kind: "commit", ID: item.ID, Summary: item.CommitSHA + " " + item.AssociationKind})
	}
	if len(facts) > 64 {
		facts = facts[len(facts)-64:]
	}
	return facts
}

func cmdKnowledgeInit(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("knowledge init", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected knowledge init arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg, projectRoot, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	result, err := knowledge.Scaffold(knowledge.ScaffoldOptions{ProjectRoot: projectRoot, KnowledgeRoot: firstNonEmpty(strings.TrimSpace(*rootFlag), cfg.Knowledge.Root)})
	if err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(result)
	}
	fmt.Printf("knowledge_init: true\nroot: %s\ncreated: %s\nexisting: %s\n", result.Root, strings.Join(result.Created, ","), strings.Join(result.Existing, ","))
	return nil
}

func cmdKnowledgeReport(ctx context.Context, opts globalOptions, command string, args []string) error {
	fs := flag.NewFlagSet("knowledge "+command, flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	failOnWarning := false
	if command == "lint" {
		fs.BoolVar(&failOnWarning, "fail-on-warning", false, "return nonzero when lint reports warnings")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected knowledge %s arguments: %s", command, strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, projectRoot string, s *store.Store) error {
		validator := func(requirement knowledge.FairwayReferenceRequirement) error {
			id, err := strconv.ParseInt(requirement.Reference.ID, 10, 64)
			if err != nil {
				return err
			}
			_, err = s.SourceFactTaskID(ctx, requirement.Reference.Kind, id)
			return err
		}
		options := knowledge.Options{ProjectRoot: projectRoot, KnowledgeRoot: firstNonEmpty(strings.TrimSpace(*rootFlag), cfg.Knowledge.Root), SourceRevision: fairwaygit.LastCommit(projectRoot), ValidateFairwayReference: validator}
		var report knowledge.Report
		var err error
		if command == "lint" {
			report, err = knowledge.Lint(options)
		} else {
			report, err = knowledge.Status(options)
		}
		if err != nil {
			return err
		}
		failed := hasKnowledgeErrors(report) || (command == "lint" && failOnWarning && hasKnowledgeWarnings(report))
		if opts.JSON {
			if err := printJSON(report); err != nil {
				return err
			}
		} else {
			fmt.Printf("knowledge_%s: %t\nroot: %s\npages: %d\nfindings: %d\n", command, !failed, report.Root, report.PageCount, len(report.Findings))
			for _, finding := range report.Findings {
				fmt.Printf("- severity=%s code=%s path=%s detail=%s\n", finding.Severity, finding.Code, firstNonEmpty(finding.Path, "none"), finding.Detail)
			}
		}
		if command == "lint" && failed {
			return errors.New("engineering knowledge lint found findings at or above the configured severity")
		}
		return nil
	})
}

func cmdKnowledgeIngest(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("knowledge ingest", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	source := fs.String("source", "", "project-relative canonical source path")
	sourceClass := fs.String("source-class", "", "configured project-file source class")
	page := fs.String("page", "", "knowledge-root-relative Markdown page path")
	title := fs.String("title", "", "derived page title; defaults from source filename")
	owner := fs.String("owner", "", "accountable page owner")
	reviewBy := fs.String("review-by", "", "page review date in YYYY-MM-DD")
	apply := fs.Bool("apply", false, "write the reviewed proposal as a normal Git diff")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected knowledge ingest arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*source) == "" || strings.TrimSpace(*page) == "" {
		return errors.New("knowledge ingest requires --source and --page")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, projectRoot string, s *store.Store) error {
		options := knowledgeStoreOptions(ctx, cfg, projectRoot, strings.TrimSpace(*rootFlag), s)
		result, err := knowledge.Ingest(knowledge.IngestOptions{
			Options: options, SourcePath: *source, SourceClass: *sourceClass,
			PagePath: *page, Title: *title, Owner: *owner, ReviewBy: *reviewBy, Apply: *apply,
		})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(result)
		}
		fmt.Printf("knowledge_ingest: %s\nsource: %s\nsource_class: %s\nsource_revision: %s\n",
			map[bool]string{true: "applied", false: "preview"}[result.Applied], result.SourcePath, result.SourceClass, result.SourceRevision)
		for _, change := range result.Changes {
			fmt.Printf("- action=%s path=%s bytes=%d sha256=%s\n", change.Action, change.Path, change.Bytes, change.SHA256)
			fmt.Printf("  preview=%q\n", change.Preview)
		}
		if !result.Applied {
			fmt.Println("next: review the bounded proposal, then rerun with --apply")
		}
		return nil
	})
}

func cmdKnowledgeQuery(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("knowledge query", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	topic := fs.String("topic", "", "bounded topic terms")
	taskID := fs.String("task", "", "Fairway task whose metadata supplies query terms")
	format := fs.String("format", "packet", "output format; packet only")
	maxResults := fs.Int("max-results", 6, "maximum selected pages")
	budget := fs.Int("budget-bytes", 12*1024, "separate knowledge packet byte budget")
	semanticIndex := fs.String("semantic-index", "", "optional project-relative derived semantic index")
	embedCommand := fs.String("embed-command", "", "explicit local embedding adapter executable")
	embeddingModel := fs.String("embedding-model", "", "embedding model identity used by the adapter")
	semanticMinScore := fs.Float64("semantic-min-score", 0.55, "minimum cosine score for semantic-only admission")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected knowledge query arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *format != "packet" {
		return errors.New("knowledge query currently supports only --format packet")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, projectRoot string, s *store.Store) error {
		taskTerms, err := knowledgeTaskTerms(ctx, s, strings.TrimSpace(*taskID))
		if err != nil {
			return err
		}
		var embed func(string) ([]float64, error)
		if strings.TrimSpace(*embedCommand) != "" {
			if strings.TrimSpace(*embeddingModel) == "" {
				return errors.New("knowledge query --embed-command requires --embedding-model")
			}
			embed = commandEmbedder(*embedCommand, *embeddingModel)
		}
		packet, err := knowledge.Query(knowledge.QueryOptions{
			Options: knowledgeStoreOptions(ctx, cfg, projectRoot, strings.TrimSpace(*rootFlag), s),
			Topic:   *topic, TaskID: *taskID, TaskTerms: taskTerms, MaxResults: *maxResults, BudgetBytes: *budget,
			SemanticIndexPath: *semanticIndex, SemanticModel: *embeddingModel, SemanticMinScore: *semanticMinScore, Embed: embed,
		})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(packet)
		}
		printKnowledgePacket(packet)
		return nil
	})
}

func commandEmbedder(executable, model string) func(string) ([]float64, error) {
	return func(input string) ([]float64, error) {
		request, err := json.Marshal(map[string]string{"model": strings.TrimSpace(model), "input": input})
		if err != nil {
			return nil, err
		}
		commandContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(commandContext, strings.TrimSpace(executable))
		command.Stdin = bytes.NewReader(request)
		stdout := boundedEmbeddingOutput{limit: 1 << 20}
		command.Stdout = &stdout
		if err := command.Run(); err != nil {
			return nil, err
		}
		if stdout.overflow {
			return nil, errors.New("embedding adapter output exceeds 1 MiB limit")
		}
		var response struct {
			Embedding []float64 `json:"embedding"`
		}
		if err := json.Unmarshal(stdout.buffer.Bytes(), &response); err != nil {
			return nil, err
		}
		return response.Embedding, nil
	}
}

type boundedEmbeddingOutput struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func (output *boundedEmbeddingOutput) Write(value []byte) (int, error) {
	remaining := output.limit - output.buffer.Len()
	if remaining < len(value) {
		output.overflow = true
		if remaining > 0 {
			_, _ = output.buffer.Write(value[:remaining])
		}
		return len(value), nil
	}
	return output.buffer.Write(value)
}

func cmdKnowledgePromote(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("knowledge promote", flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	target := fs.String("target", "", "explicit project-relative canonical target")
	reviewedCommit := fs.String("reviewed-commit", "", "reviewed commit containing the canonical target")
	apply := fs.Bool("apply", false, "record promotion on the derived page as a normal Git diff")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("knowledge promote requires exactly one knowledge page path")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, projectRoot string, s *store.Store) error {
		result, err := knowledge.Promote(knowledge.PromoteOptions{
			Options:  knowledgeStoreOptions(ctx, cfg, projectRoot, strings.TrimSpace(*rootFlag), s),
			PagePath: fs.Arg(0), TargetPath: *target, ReviewedCommit: *reviewedCommit, Apply: *apply,
		})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(result)
		}
		fmt.Printf("knowledge_promote: %s\npage: %s\ntarget: %s\nreviewed_commit: %s\n",
			map[bool]string{true: "applied", false: "preview"}[result.Applied], result.PagePath, result.TargetPath, result.ReviewedCommit)
		for _, change := range result.Changes {
			fmt.Printf("- action=%s path=%s bytes=%d sha256=%s\n", change.Action, change.Path, change.Bytes, change.SHA256)
			fmt.Printf("  preview=%q\n", change.Preview)
		}
		return nil
	})
}

func knowledgeStoreOptions(ctx context.Context, cfg config.Config, projectRoot, rootFlag string, s *store.Store) knowledge.Options {
	return knowledge.Options{
		ProjectRoot: projectRoot, KnowledgeRoot: firstNonEmpty(rootFlag, cfg.Knowledge.Root),
		SourceRevision: fairwaygit.LastCommit(projectRoot),
		ValidateFairwayReference: func(requirement knowledge.FairwayReferenceRequirement) error {
			id, err := strconv.ParseInt(requirement.Reference.ID, 10, 64)
			if err != nil {
				return err
			}
			_, err = s.SourceFactTaskID(ctx, requirement.Reference.Kind, id)
			return err
		},
	}
}

func knowledgeTaskTerms(ctx context.Context, s *store.Store, taskID string) ([]string, error) {
	if taskID == "" {
		return nil, nil
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("resolve knowledge query task %s: %w", taskID, err)
	}
	terms := []string{task.Definition.ID, task.Definition.Title, task.Definition.OwningDomain, task.Definition.OwningLayer}
	terms = append(terms, task.Definition.AcceptanceChecks...)
	terms = append(terms, task.Definition.SourcePaths...)
	terms = append(terms, task.Definition.TargetPaths...)
	return terms, nil
}

func printKnowledgePacket(packet knowledge.QueryPacket) {
	fmt.Printf("# Fairway Knowledge Query\n\nschema: %s\nbounded: true\nread_only: true\nretrieval_mode: %s\nbytes: %d/%d\ntopic: %s\ntask: %s\nrepository_revision: %s\n",
		packet.Schema, packet.RetrievalMode, packet.Bytes, packet.BudgetBytes, firstNonEmpty(packet.Topic, "none"), firstNonEmpty(packet.TaskID, "none"), firstNonEmpty(packet.RepositoryRevision, "unknown"))
	for _, page := range packet.Pages {
		fmt.Printf("- page=%s title=%q status=%s verified=%t stale=%t conflict=%t score=%d owner=%s review_by=%s source_freshness=%s sources=%d\n",
			page.Path, page.Title, page.Status, page.Verified, page.Stale, page.Conflict, page.Score, page.Owner, page.ReviewBy, page.SourceFreshness, page.SourceCount)
		if page.Excerpt != "" {
			fmt.Printf("  excerpt: %s\n", page.Excerpt)
		}
	}
	for _, source := range packet.Sources {
		fmt.Printf("- source=%s class=%s authority=%s verified=%t\n", source.Key, source.Class, source.Authority, source.Verified)
	}
	for _, warning := range packet.Warnings {
		fmt.Printf("- warning=%s\n", warning)
	}
}

func hasKnowledgeErrors(report knowledge.Report) bool {
	for _, finding := range report.Findings {
		if finding.Severity == knowledge.SeverityError {
			return true
		}
	}
	return false
}

func hasKnowledgeWarnings(report knowledge.Report) bool {
	for _, finding := range report.Findings {
		if finding.Severity == knowledge.SeverityWarning {
			return true
		}
	}
	return false
}
