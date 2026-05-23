package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/dashboard"
	"github.com/subashram/fairway/internal/importer"
	"github.com/subashram/fairway/internal/state"
	"github.com/subashram/fairway/internal/store"
)

const version = "0.1.0-dev"

type globalOptions struct {
	ConfigPath string
	DBPath     string
	Role       string
	JSON       bool
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCode(err))
	}
}

func run(ctx context.Context, args []string) error {
	opts, args, err := parseGlobalFlags(args)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "init":
		return cmdInit(ctx, opts)
	case "import":
		return cmdImport(ctx, opts, args[1:])
	case "ready":
		return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
			role := resolveRole(opts)
			tasks, err := s.Ready(ctx, role)
			if err != nil {
				return err
			}
			if opts.JSON {
				return printJSON(tasks)
			}
			printTasks(tasks)
			return nil
		})
	case "claim":
		if len(args) < 2 {
			return errors.New("claim requires task id")
		}
		return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
			owner := resolveRole(opts)
			if owner == "" {
				owner = "manual"
			}
			if err := s.Claim(ctx, args[1], owner, ""); err != nil {
				return err
			}
			fmt.Println("claimed", args[1])
			return nil
		})
	case "set-status":
		return cmdSetStatus(ctx, opts, args[1:])
	case "record":
		return cmdRecord(ctx, opts, args[1:])
	case "task-detail":
		if len(args) < 2 {
			return errors.New("task-detail requires task id")
		}
		return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
			return printDetail(ctx, s, args[1], opts.JSON)
		})
	case "config":
		if len(args) >= 2 && args[1] == "validate" {
			_, _, path, err := loadConfig(opts)
			if err != nil {
				return err
			}
			fmt.Println("valid", path)
			return nil
		}
	case "dashboard":
		return cmdDashboard(ctx, opts, args[1:])
	case "version":
		fmt.Println(version)
		return nil
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func parseGlobalFlags(args []string) (globalOptions, []string, error) {
	var opts globalOptions
	for len(args) > 0 {
		switch args[0] {
		case "--config":
			if len(args) < 2 {
				return opts, nil, errors.New("--config requires path")
			}
			opts.ConfigPath = args[1]
			args = args[2:]
		case "--db":
			if len(args) < 2 {
				return opts, nil, errors.New("--db requires path")
			}
			opts.DBPath = args[1]
			args = args[2:]
		case "--as":
			if len(args) < 2 {
				return opts, nil, errors.New("--as requires role")
			}
			opts.Role = args[1]
			args = args[2:]
		case "--json":
			opts.JSON = true
			args = args[1:]
		default:
			return opts, args, nil
		}
	}
	return opts, args, nil
}

func cmdInit(ctx context.Context, opts globalOptions) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	path := config.DefaultConfigPath
	if opts.ConfigPath != "" {
		path = opts.ConfigPath
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := config.WriteDefault(path, root); err != nil {
		return err
	}
	cfg, _, err := config.Load(path)
	if err != nil {
		return err
	}
	dbPath := config.DBPath(cfg, root)
	if opts.DBPath != "" {
		dbPath = opts.DBPath
	}
	s, err := store.Open(ctx, dbPath, cfg.Fairway.ProjectName)
	if err != nil {
		return err
	}
	defer s.Close()
	fmt.Println("initialized fairway")
	return nil
}

func cmdImport(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("import requires yaml or json path")
	}
	tasks, err := importer.Tasks(args[0])
	if err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		if err := validateImportRoles(tasks, cfg); err != nil {
			return err
		}
		if err := s.ImportTasks(ctx, tasks); err != nil {
			return err
		}
		fmt.Printf("imported %d tasks\n", len(tasks))
		return nil
	})
}

func validateImportRoles(tasks []store.TaskDefinition, cfg config.Config) error {
	roles := config.RoleSet(cfg)
	if len(roles) == 0 {
		return nil
	}
	for _, task := range tasks {
		if !roles[task.Role] {
			return fmt.Errorf("task %s uses unknown role %q", task.ID, task.Role)
		}
	}
	return nil
}

func cmdSetStatus(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 2 {
		return errors.New("set-status requires task id and state")
	}
	fs := flag.NewFlagSet("set-status", flag.ContinueOnError)
	reason := fs.String("reason", "", "reason")
	reopen := fs.Bool("reopen", false, "reopen terminal task")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		current, err := s.CurrentStatus(ctx, args[0])
		if err != nil {
			return err
		}
		stateCfg := state.Config{Allowed: cfg.States.Allowed, Terminal: cfg.States.Terminal, Transitions: cfg.States.Transitions}
		if err := state.ValidateTransition(stateCfg, current, args[1], *reopen); err != nil {
			return err
		}
		if state.IsTerminal(stateCfg, args[1]) {
			if err := validateTerminalGates(ctx, cfg, s, args[0]); err != nil {
				return err
			}
		}
		if err := s.SetStatus(ctx, args[0], args[1], *reason, cfg.Gates.RequireBlockedReason); err != nil {
			return err
		}
		fmt.Println("status", args[0], args[1])
		return nil
	})
}

func cmdRecord(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 2 {
		return errors.New("record requires type and task id")
	}
	switch args[0] {
	case "evidence":
		return recordEvidence(ctx, opts, args[1:])
	case "handoff":
		return recordHandoff(ctx, opts, args[1:])
	case "review":
		return recordReview(ctx, opts, args[1:])
	}
	return fmt.Errorf("unknown record type %q", args[0])
}

func recordEvidence(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("record evidence requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("evidence", flag.ContinueOnError)
	commandText := fs.String("command-text", "", "command")
	result := fs.String("result", "", "result")
	artifact := fs.String("artifact", "", "artifact")
	artifactType := fs.String("artifact-type", "", "artifact type")
	notes := fs.String("notes", "", "notes")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *commandText == "" {
		return errors.New("--command-text is required")
	}
	if *result == "" {
		return errors.New("--result is required")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		return s.RecordEvidence(ctx, taskID, store.Evidence{CommandText: *commandText, Result: *result, ArtifactPath: *artifact, ArtifactType: *artifactType, Notes: *notes})
	})
}

func recordHandoff(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("record handoff requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("handoff", flag.ContinueOnError)
	to := fs.String("to", "", "to role")
	payload := fs.String("payload", "", "payload")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *to == "" {
		return errors.New("--to is required")
	}
	if *payload == "" {
		return errors.New("--payload is required")
	}
	if strings.HasPrefix(*payload, "@") {
		data, err := os.ReadFile(strings.TrimPrefix(*payload, "@"))
		if err != nil {
			return err
		}
		*payload = string(data)
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		return s.RecordHandoff(ctx, taskID, store.Handoff{ToRole: *to, Payload: *payload})
	})
}

func recordReview(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("record review requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	reviewer := fs.String("reviewer", "", "reviewer")
	verdict := fs.String("verdict", "", "verdict")
	reason := fs.String("reason", "", "reason")
	commit := fs.String("commit", "", "commit")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *reviewer == "" {
		return errors.New("--reviewer is required")
	}
	if *verdict == "" {
		return errors.New("--verdict is required")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		return s.RecordReview(ctx, taskID, store.Review{Reviewer: *reviewer, Verdict: *verdict, Reason: *reason, Commit: *commit})
	})
}

func cmdDashboard(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	listen := fs.String("listen", "", "listen address")
	noOpen := fs.Bool("no-open", false, "do not open browser")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = noOpen
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		addr := cfg.Dashboard.Listen
		if *listen != "" {
			addr = *listen
		}
		fmt.Println("dashboard", dashboard.URL(addr))
		return dashboard.New(s, roleNames(cfg)).ListenAndServe(addr)
	})
}

func withStore(ctx context.Context, opts globalOptions, fn func(context.Context, config.Config, string, *store.Store) error) error {
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	dbPath := config.DBPath(cfg, root)
	if opts.DBPath != "" {
		dbPath = opts.DBPath
	}
	s, err := store.Open(ctx, dbPath, cfg.Fairway.ProjectName)
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(ctx, cfg, root, s)
}

func loadConfig(opts globalOptions) (config.Config, string, string, error) {
	path := opts.ConfigPath
	if path == "" {
		path = os.Getenv("FAIRWAY_CONFIG")
	}
	if path == "" {
		var err error
		path, err = config.FindConfig("")
		if err != nil {
			return config.Config{}, "", "", err
		}
	}
	cfg, root, err := config.Load(path)
	return cfg, root, path, err
}

func resolveRole(opts globalOptions) string {
	if opts.Role != "" {
		return opts.Role
	}
	return os.Getenv("FAIRWAY_ROLE")
}

func roleNames(cfg config.Config) []string {
	roles := make([]string, 0, len(cfg.Roles))
	for _, role := range cfg.Roles {
		roles = append(roles, role.Name)
	}
	return roles
}

func validateTerminalGates(ctx context.Context, cfg config.Config, s *store.Store, taskID string) error {
	if cfg.Gates.RequireEvidenceBeforeDone {
		ok, err := s.HasEvidence(ctx, taskID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("terminal transition requires evidence")
		}
	}
	if cfg.Gates.RequireReviewBeforeDone {
		ok, err := s.HasApprovedReview(ctx, taskID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("terminal transition requires approved review")
		}
	}
	return nil
}

func printTasks(tasks []store.Task) {
	for _, task := range tasks {
		fmt.Printf("%s\t%s\t%s\t%s\n", task.Definition.ID, task.Definition.Role, task.Status, task.Definition.Title)
	}
}

func printDetail(ctx context.Context, s *store.Store, taskID string, asJSON bool) error {
	task, transitions, evidence, handoffs, reviews, err := s.TaskDetail(ctx, taskID)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSON(struct {
			Task        store.Task         `json:"task"`
			Transitions []store.Transition `json:"transitions"`
			Evidence    []store.Evidence   `json:"evidence"`
			Handoffs    []store.Handoff    `json:"handoffs"`
			Reviews     []store.Review     `json:"reviews"`
		}{task, transitions, evidence, handoffs, reviews})
	}
	fmt.Printf("%s %s\nstatus: %s\nrole: %s\nowner: %s\nreview: %s\n\n%s\n", task.Definition.ID, task.Definition.Title, task.Status, task.Definition.Role, task.Owner, task.ReviewStatus, task.Definition.Notes)
	fmt.Println("\nhistory:")
	for _, tr := range transitions {
		from := tr.FromStatus
		if from == "" {
			from = "new"
		}
		fmt.Printf("- %s -> %s by %s %s\n", from, tr.ToStatus, tr.Actor, tr.Reason)
	}
	fmt.Println("\nevidence:")
	for _, ev := range evidence {
		fmt.Printf("- %s %s %s\n", ev.Result, ev.CommandText, ev.ArtifactPath)
	}
	fmt.Println("\nhandoffs:")
	for _, h := range handoffs {
		fmt.Printf("- to %s: %s\n", h.ToRole, h.Payload)
	}
	fmt.Println("\nreviews:")
	for _, r := range reviews {
		fmt.Printf("- %s by %s: %s\n", r.Verdict, r.Reviewer, r.Reason)
	}
	return nil
}

func printJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func exitCode(err error) int {
	if errors.Is(err, store.ErrNotFound) {
		return 3
	}
	if errors.Is(err, store.ErrAlreadyClaimed) || errors.Is(err, store.ErrInvalidTransition) {
		return 4
	}
	return 1
}

func usage() {
	fmt.Println("fairway init|import|ready|claim|set-status|record|task-detail|config validate|dashboard|version")
}
