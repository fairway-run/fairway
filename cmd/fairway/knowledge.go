package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/subashram/fairway/internal/config"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/knowledge"
	"github.com/subashram/fairway/internal/store"
)

func cmdKnowledge(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("knowledge", "init|status|lint|ingest|query|promote ...")
		return nil
	}
	switch args[0] {
	case "init":
		return cmdKnowledgeInit(opts, args[1:])
	case "status", "lint":
		return cmdKnowledgeReport(ctx, opts, args[0], args[1:])
	case "ingest":
		return cmdKnowledgeIngest(ctx, opts, args[1:])
	case "query":
		return cmdKnowledgeQuery(ctx, opts, args[1:])
	case "promote":
		return cmdKnowledgePromote(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown knowledge subcommand %q", args[0])
	}
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
		packet, err := knowledge.Query(knowledge.QueryOptions{
			Options: knowledgeStoreOptions(ctx, cfg, projectRoot, strings.TrimSpace(*rootFlag), s),
			Topic:   *topic, TaskID: *taskID, TaskTerms: taskTerms, MaxResults: *maxResults, BudgetBytes: *budget,
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
	fmt.Printf("# Fairway Knowledge Query\n\nschema: %s\nbounded: true\nread_only: true\nbytes: %d/%d\ntopic: %s\ntask: %s\nrepository_revision: %s\n",
		packet.Schema, packet.Bytes, packet.BudgetBytes, firstNonEmpty(packet.Topic, "none"), firstNonEmpty(packet.TaskID, "none"), firstNonEmpty(packet.RepositoryRevision, "unknown"))
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
