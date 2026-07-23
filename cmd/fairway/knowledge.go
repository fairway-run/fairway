package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/knowledge"
)

func cmdKnowledge(_ context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("knowledge", "init|status|lint [--root <project-relative-path>]")
		return nil
	}
	switch args[0] {
	case "init":
		return cmdKnowledgeInit(opts, args[1:])
	case "status", "lint":
		return cmdKnowledgeReport(opts, args[0], args[1:])
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

func cmdKnowledgeReport(opts globalOptions, command string, args []string) error {
	fs := flag.NewFlagSet("knowledge "+command, flag.ContinueOnError)
	rootFlag := fs.String("root", "", "project-relative knowledge root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected knowledge %s arguments: %s", command, strings.Join(fs.Args(), " "))
	}
	cfg, projectRoot, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	options := knowledge.Options{ProjectRoot: projectRoot, KnowledgeRoot: firstNonEmpty(strings.TrimSpace(*rootFlag), cfg.Knowledge.Root), SourceRevision: fairwaygit.LastCommit(projectRoot)}
	var report knowledge.Report
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
}

func hasKnowledgeErrors(report knowledge.Report) bool {
	for _, finding := range report.Findings {
		if finding.Severity == knowledge.SeverityError {
			return true
		}
	}
	return false
}
