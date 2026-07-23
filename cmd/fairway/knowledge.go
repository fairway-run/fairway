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
		subcommandUsage("knowledge", "init|status|lint [--root <project-relative-path>]")
		return nil
	}
	switch args[0] {
	case "init":
		return cmdKnowledgeInit(opts, args[1:])
	case "status", "lint":
		return cmdKnowledgeReport(ctx, opts, args[0], args[1:])
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
		if opts.JSON {
			if err := printJSON(report); err != nil {
				return err
			}
		} else {
			fmt.Printf("knowledge_%s: %t\nroot: %s\npages: %d\nfindings: %d\n", command, !hasKnowledgeErrors(report), report.Root, report.PageCount, len(report.Findings))
			for _, finding := range report.Findings {
				fmt.Printf("- severity=%s code=%s path=%s detail=%s\n", finding.Severity, finding.Code, firstNonEmpty(finding.Path, "none"), finding.Detail)
			}
		}
		if command == "lint" && hasKnowledgeErrors(report) {
			return errors.New("engineering knowledge lint found errors")
		}
		return nil
	})
}

func hasKnowledgeErrors(report knowledge.Report) bool {
	for _, finding := range report.Findings {
		if finding.Severity == knowledge.SeverityError {
			return true
		}
	}
	return false
}
