package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/subashram/fairway/internal/releaserehearsal"
)

func cmdReleaseRehearsal(opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("release rehearsal", "create|verify|extract-assurance")
		return nil
	}
	switch args[0] {
	case "create":
		return cmdReleaseRehearsalCreate(opts, args[1:])
	case "verify":
		return cmdReleaseRehearsalVerify(opts, args[1:])
	case "extract-assurance":
		return cmdReleaseRehearsalExtractAssurance(args[1:])
	default:
		return fmt.Errorf("unknown release rehearsal subcommand %q", args[0])
	}
}

func cmdReleaseRehearsalExtractAssurance(args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("release rehearsal extract-assurance", "--dir <path> --version <vX.Y.Z> --out <path>")
		return nil
	}
	fs := flag.NewFlagSet("release rehearsal extract-assurance", flag.ContinueOnError)
	dir := fs.String("dir", "", "candidate packet directory")
	version := fs.String("version", "", "candidate release version")
	output := fs.String("out", "", "new output directory for the safely extracted assurance package")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected release rehearsal extract-assurance arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *dir == "" || *version == "" || *output == "" {
		return errors.New("release rehearsal extract-assurance requires --dir, --version, and --out")
	}
	if err := releaserehearsal.ExtractAssurance(*dir, *version, *output); err != nil {
		return err
	}
	fmt.Println("release_rehearsal_extract_assurance:", *output)
	return nil
}

func cmdReleaseRehearsalCreate(opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("release rehearsal create", "--dir <path> --version <vX.Y.Z> --source-sha <sha> --builder-id <id> --policy-version <id> [--created-at <RFC3339>]")
		return nil
	}
	fs := flag.NewFlagSet("release rehearsal create", flag.ContinueOnError)
	dir := fs.String("dir", "", "candidate packet directory containing the exact seven release assets")
	version := fs.String("version", "", "candidate release version")
	sourceSHA := fs.String("source-sha", "", "exact candidate source SHA")
	builderID := fs.String("builder-id", "", "trusted rehearsal workflow identity")
	policyVersion := fs.String("policy-version", "", "release policy version")
	createdAt := fs.String("created-at", "", "optional deterministic RFC3339 creation time")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected release rehearsal create arguments: %s", strings.Join(fs.Args(), " "))
	}
	manifest, err := releaserehearsal.Create(*dir, *version, *sourceSHA, *builderID, *policyVersion, *createdAt)
	if err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(manifest)
	}
	fmt.Printf("release_rehearsal_create: pass\nversion: %s\nsource_sha: %s\nbuilder_id: %s\nassets: %d\n", manifest.Version, manifest.SourceSHA, manifest.BuilderID, len(manifest.Assets))
	return nil
}

func cmdReleaseRehearsalVerify(opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("release rehearsal verify", "--dir <path> --version <vX.Y.Z> --source-sha <sha> --builder-id <id> --policy-version <id>")
		return nil
	}
	fs := flag.NewFlagSet("release rehearsal verify", flag.ContinueOnError)
	dir := fs.String("dir", "", "candidate packet directory")
	version := fs.String("version", "", "expected candidate release version")
	sourceSHA := fs.String("source-sha", "", "expected source SHA")
	builderID := fs.String("builder-id", "", "expected rehearsal workflow identity")
	policyVersion := fs.String("policy-version", "", "expected release policy version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected release rehearsal verify arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *dir == "" || *version == "" || *sourceSHA == "" || *builderID == "" || *policyVersion == "" {
		return errors.New("release rehearsal verify requires --dir, --version, --source-sha, --builder-id, and --policy-version")
	}
	report, err := releaserehearsal.Verify(releaserehearsal.VerifyOptions{
		Dir:                   *dir,
		ExpectedVersion:       *version,
		ExpectedSourceSHA:     *sourceSHA,
		ExpectedBuilderID:     *builderID,
		ExpectedPolicyVersion: *policyVersion,
	})
	if opts.JSON {
		if printErr := printJSON(report); printErr != nil {
			return printErr
		}
	} else {
		fmt.Printf("release_rehearsal_verify: %t\nversion: %s\nsource_sha: %s\nbuilder_id: %s\nassets: %d\n", report.OK, report.Version, report.SourceSHA, report.BuilderID, report.AssetCount)
		for _, issue := range report.Issues {
			fmt.Println("issue:", issue)
		}
	}
	return err
}
