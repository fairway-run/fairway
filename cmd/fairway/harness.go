package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/harnessanalytics"
	"github.com/subashram/fairway/internal/harnessrecord"
	"github.com/subashram/fairway/internal/store"
)

func cmdHarness(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		fmt.Println("fairway harness ingest --file <records.json>|--stdin")
		fmt.Println("fairway harness runs --task <task-id> [--format text|json]")
		fmt.Println("fairway harness record <external-run-id> --source <source-id> [--format text|json]")
		fmt.Println("fairway harness report --task <task-id> [--format text|json]")
		return nil
	}
	switch args[0] {
	case "ingest":
		return cmdHarnessIngest(ctx, opts, args[1:])
	case "runs":
		return cmdHarnessRuns(ctx, opts, args[1:])
	case "record":
		return cmdHarnessRecord(ctx, opts, args[1:])
	case "report":
		return cmdHarnessReport(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown harness subcommand %q", args[0])
	}
}

func cmdHarnessReport(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway harness report --task <task-id> [--format text|json]")
		return nil
	}
	fs := flag.NewFlagSet("harness report", flag.ContinueOnError)
	taskID := fs.String("task", "", "Fairway task id")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 || strings.TrimSpace(*taskID) == "" {
		return errors.New("harness report requires --task")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		report, err := harnessanalytics.Build(ctx, s, *taskID)
		if err != nil {
			return err
		}
		if opts.JSON || *format == "json" {
			return printJSON(report)
		}
		if *format != "text" {
			return fmt.Errorf("unsupported harness report format %q", *format)
		}
		fmt.Printf("harness_analysis task=%s attempts=%d actions=%d evaluations=%d evaluator_backed_outcomes=%d efficiency=%s\n", report.TaskID, report.Attempts, report.Actions, report.Evaluations, report.VerifiedOutcomes, report.Efficiency.Status)
		fmt.Printf("cohort status=%s name=%s missing=%s\n", report.Cohort.Status, report.Cohort.Name, strings.Join(report.Cohort.Missing, "; "))
		fmt.Printf("usage events=%d known_tokens=%d known_elapsed=%d confidence=%s attribution=%s reason=%s cost=%s reason=%s\n", report.Usage.Events, report.Usage.KnownTokenEvents, report.Usage.KnownElapsedEvents, report.Usage.ConfidenceStatus, report.Usage.AttributionStatus, report.Usage.AttributionReason, report.Usage.CostStatus, report.Usage.CostReason)
		if len(report.Efficiency.Missing) > 0 {
			fmt.Println("efficiency missing:", strings.Join(report.Efficiency.Missing, "; "))
		}
		for _, finding := range report.Trajectory {
			fmt.Printf("- %s recommendation=%s refs=%s\n", finding.Kind, finding.Recommendation, strings.Join(finding.SourceRefs, ","))
		}
		for _, limitation := range report.Limitations {
			fmt.Println("limitation:", limitation)
		}
		fmt.Println("authority:", report.AuthorityBoundary)
		return nil
	})
}

func cmdHarnessIngest(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway harness ingest --file <records.json>|--stdin")
		return nil
	}
	fs := flag.NewFlagSet("harness ingest", flag.ContinueOnError)
	path := fs.String("file", "", "JSON harness record batch")
	stdin := fs.Bool("stdin", false, "read JSON harness record batch from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected harness ingest arguments: %s", strings.Join(fs.Args(), " "))
	}
	if (*path == "") == !*stdin {
		return errors.New("harness ingest requires exactly one of --file or --stdin")
	}
	var reader io.Reader = os.Stdin
	if *path != "" {
		file, err := os.Open(*path)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	batch, err := harnessrecord.DecodeBatch(reader)
	if err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		result, err := s.IngestHarnessBatch(ctx, batch)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(result)
		}
		fmt.Printf("harness records ingested runs=%d existing_runs=%d observations=%d existing_observations=%d evaluations=%d existing_evaluations=%d\n", result.ExternalRunsInserted, result.ExternalRunsExisting, result.ObservationsInserted, result.ObservationsExisting, result.EvaluatorResultsInserted, result.EvaluatorResultsExisting)
		return nil
	})
}

func cmdHarnessRuns(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway harness runs --task <task-id> [--format text|json]")
		return nil
	}
	fs := flag.NewFlagSet("harness runs", flag.ContinueOnError)
	taskID := fs.String("task", "", "Fairway task id")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 || strings.TrimSpace(*taskID) == "" {
		return errors.New("harness runs requires --task")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		records, err := s.HarnessRecordsForTask(ctx, *taskID)
		if err != nil {
			return err
		}
		if opts.JSON || *format == "json" {
			return printJSON(records)
		}
		if *format != "text" {
			return fmt.Errorf("unsupported harness runs format %q", *format)
		}
		fmt.Printf("harness_records task=%s runs=%d run_independent_observations=%d run_independent_evaluations=%d\n", records.TaskID, len(records.Runs), len(records.RunIndependentObservations), len(records.RunIndependentEvaluations))
		for _, record := range records.Runs {
			fmt.Printf("- source=%s run=%s observations=%d evaluations=%d terminal=%s\n", record.Run.SourceID, record.Run.ExternalRunID, len(record.Observations), len(record.EvaluatorResults), firstNonEmpty(record.Run.TerminalStatus, "unknown"))
		}
		return nil
	})
}

func cmdHarnessRecord(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway harness record <external-run-id> --source <source-id> [--format text|json]")
		return nil
	}
	if len(args) == 0 {
		return errors.New("harness record requires external run id")
	}
	runID := args[0]
	if isHelpOnly(args[1:]) {
		fmt.Println("fairway harness record <external-run-id> --source <source-id> [--format text|json]")
		return nil
	}
	fs := flag.NewFlagSet("harness record", flag.ContinueOnError)
	sourceID := fs.String("source", "", "source identity owning the run id")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 || strings.TrimSpace(*sourceID) == "" {
		return errors.New("harness record requires --source")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		record, err := s.HarnessRecord(ctx, *sourceID, runID)
		if err != nil {
			return err
		}
		if opts.JSON || *format == "json" {
			return printJSON(record)
		}
		if *format != "text" {
			return fmt.Errorf("unsupported harness record format %q", *format)
		}
		fmt.Printf("harness_record source=%s run=%s task=%s observations=%d evaluations=%d terminal=%s\n", record.Run.SourceID, record.Run.ExternalRunID, record.Run.TaskID, len(record.Observations), len(record.EvaluatorResults), firstNonEmpty(record.Run.TerminalStatus, "unknown"))
		return nil
	})
}

type harnessContractCatalog struct {
	Schema            string                 `json:"schema"`
	Version           string                 `json:"version"`
	InputSchemas      []string               `json:"input_schemas"`
	Contracts         []harnessInputContract `json:"contracts"`
	Compatibility     []string               `json:"compatibility"`
	PrivacyExclusions []string               `json:"privacy_exclusions"`
	AuthorityLimits   []string               `json:"authority_limits"`
}

type harnessInputContract struct {
	Schema         string              `json:"schema"`
	Name           string              `json:"name"`
	Purpose        string              `json:"purpose"`
	RequiredFields []string            `json:"required_fields"`
	Enums          map[string][]string `json:"enums,omitempty"`
	ReferenceRules []string            `json:"reference_rules,omitempty"`
}

func cmdContractHarnessRecord(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("contract harness-record", flag.ContinueOnError)
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected contract harness-record arguments: %s", strings.Join(fs.Args(), " "))
	}
	catalog := harnessContractCatalog{
		Schema:       "fairway.harness-record-contracts.v1",
		Version:      "1.0",
		InputSchemas: []string{harnessrecord.BatchSchema, harnessrecord.ExternalRunSchema, harnessrecord.ObservationSchema, harnessrecord.EvaluationSchema},
		Contracts: []harnessInputContract{
			{Schema: harnessrecord.BatchSchema, Name: "batch", Purpose: "Atomically validate and append ordered harness records.", RequiredFields: []string{"schema", "at least one record"}},
			{Schema: harnessrecord.ExternalRunSchema, Name: "external_run", Purpose: "Correlate one source-owned execution attempt with a Fairway task.", RequiredFields: []string{"schema", "source_id", "source_version", "external_run_id", "task_id", "submission_id", "observed_at"}, ReferenceRules: []string{"session_id must belong to task_id", "prior_run_ref is fully source-qualified and must belong to task_id"}},
			{Schema: harnessrecord.ObservationSchema, Name: "execution_observation", Purpose: "Retain one bounded sourced experiment or material execution fact.", RequiredFields: []string{"schema", "observation_id", "source_id", "source_version", "task_id", "kind", "subject_type", "subject_ref", "summary", "observed_at", "outcome", "source_mode"}, Enums: map[string][]string{"kind": {"experiment", "execution", "artifact", "policy", "usage", "terminal"}, "outcome": {"confirmed", "rejected", "inconclusive", "blocked", "unavailable"}, "source_mode": {"measured", "reported", "derived", "human_judgment"}}, ReferenceRules: []string{"external_run_ref is optional, fully source-qualified, and must belong to task_id", "experiment kind also requires hypothesis and expected_observation"}},
			{Schema: harnessrecord.EvaluationSchema, Name: "evaluator_result", Purpose: "Retain one sourced evaluator judgment without creating a Fairway review.", RequiredFields: []string{"schema", "evaluation_id", "source_id", "source_version", "task_id", "evaluator_id", "evaluator_version", "subject_type", "subject_ref", "result", "mode", "evaluated_at"}, Enums: map[string][]string{"result": {"pass", "fail", "partial", "inconclusive", "error", "unavailable"}, "mode": {"deterministic", "statistical", "human_judgment"}}, ReferenceRules: []string{"external_run_ref and observation_ref are optional, fully source-qualified, and must belong to task_id"}},
		},
		Compatibility:     []string{"optional fields are additive within schema major v1", "unknown fields are rejected unless carried in bounded metadata", "same source-scoped identity and canonical payload is an idempotent replay", "same identity with a different canonical payload is a conflict"},
		PrivacyExclusions: []string{"raw prompts", "private reasoning", "transcripts", "raw tool bodies", "credentials and secrets", "generated-content dumps"},
		AuthorityLimits:   []string{"ingestion records sourced facts only", "does not change task or session state", "does not accept evidence or create review verdicts", "does not approve, merge, release, deploy, cancel, or redirect execution"},
	}
	if opts.JSON || *format == "json" {
		return printJSON(catalog)
	}
	if *format != "text" {
		return fmt.Errorf("unsupported contract harness-record format %q", *format)
	}
	data, _ := json.Marshal(catalog.InputSchemas)
	fmt.Printf("harness_record_contracts schema=%s version=%s input_schemas=%s\n", catalog.Schema, catalog.Version, data)
	return nil
}
