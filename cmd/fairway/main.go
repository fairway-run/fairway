package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	fairwaydocs "github.com/subashram/fairway/docs"
	"github.com/subashram/fairway/internal/audit"
	"github.com/subashram/fairway/internal/automationreport"
	"github.com/subashram/fairway/internal/completionhandback"
	"github.com/subashram/fairway/internal/config"
	coord "github.com/subashram/fairway/internal/coordinator"
	"github.com/subashram/fairway/internal/dashboard"
	"github.com/subashram/fairway/internal/deliveryreport"
	"github.com/subashram/fairway/internal/evidencemodel"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/importer"
	"github.com/subashram/fairway/internal/livewindow"
	"github.com/subashram/fairway/internal/provenance"
	"github.com/subashram/fairway/internal/reconcile"
	"github.com/subashram/fairway/internal/registry"
	"github.com/subashram/fairway/internal/reviewpolicy"
	"github.com/subashram/fairway/internal/reviewstate"
	"github.com/subashram/fairway/internal/roughedge"
	"github.com/subashram/fairway/internal/rules"
	"github.com/subashram/fairway/internal/state"
	"github.com/subashram/fairway/internal/store"
	"github.com/subashram/fairway/internal/tracker"
	planetracker "github.com/subashram/fairway/internal/tracker/plane"
	"gopkg.in/yaml.v3"
)

var version = "0.1.0-dev"

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
	if len(args) > 1 && isHelpOnly(args[1:]) {
		if printCommandHelp(args[0]) {
			return nil
		}
	}
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			return fmt.Errorf("unexpected help arguments: %s", strings.Join(args[1:], " "))
		}
		usage()
		return nil
	case "init":
		return cmdInit(ctx, opts, args[1:])
	case "import":
		return cmdImport(ctx, opts, args[1:])
	case "add":
		return cmdAdd(ctx, opts, args[1:])
	case "spawn":
		return cmdSpawn(ctx, opts, args[1:])
	case "update":
		return cmdUpdate(ctx, opts, args[1:])
	case "tree":
		return cmdTree(ctx, opts, args[1:])
	case "list":
		return cmdList(ctx, opts, args[1:])
	case "ready":
		return cmdReady(ctx, opts, args[1:])
	case "claim":
		return cmdClaim(ctx, opts, args[1:])
	case "set-status":
		return cmdSetStatus(ctx, opts, args[1:])
	case "record":
		return cmdRecord(ctx, opts, args[1:])
	case "usage":
		return cmdUsage(ctx, opts, args[1:])
	case "task-detail":
		if len(args) < 2 {
			return errors.New("task-detail requires task id")
		}
		if len(args) > 2 {
			return fmt.Errorf("unexpected task-detail arguments: %s", strings.Join(args[2:], " "))
		}
		return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
			return printDetail(ctx, cfg, s, args[1], opts.JSON)
		})
	case "status-report":
		return cmdStatusReport(ctx, opts, args[1:])
	case "health-report":
		return cmdHealthReport(ctx, opts, args[1:])
	case "timing-report":
		return cmdTimingReport(ctx, opts, args[1:])
	case "completion-handback-report":
		return cmdCompletionHandbackReport(ctx, opts, args[1:])
	case "delivery":
		return cmdDelivery(ctx, opts, args[1:])
	case "rough-edge":
		return cmdRoughEdge(ctx, opts, args[1:])
	case "provenance":
		return cmdProvenance(ctx, opts, args[1:])
	case "recipe":
		return cmdRecipe(ctx, opts, args[1:])
	case "dispatch-plan":
		return cmdDispatchPlan(ctx, opts, args[1:])
	case "git-check":
		return cmdGitCheck(opts, args[1:])
	case "preflight":
		return cmdPreflight(opts, args[1:])
	case "workflow":
		return cmdWorkflow(ctx, opts, args[1:])
	case "batch":
		return cmdBatch(ctx, opts, args[1:])
	case "audit":
		return cmdAudit(ctx, opts, args[1:])
	case "advisory":
		return cmdAdvisory(ctx, opts, args[1:])
	case "notify":
		return cmdNotify(ctx, opts, args[1:])
	case "automation":
		return cmdAutomation(ctx, opts, args[1:])
	case "merge-ready":
		return cmdMergeReady(ctx, opts, args[1:])
	case "review-waits":
		return cmdReviewWaits(ctx, opts, args[1:])
	case "review-policy":
		return cmdReviewPolicy(ctx, opts, args[1:])
	case "live-window":
		return cmdLiveWindow(ctx, opts, args[1:])
	case "wait":
		return cmdWait(ctx, opts, args[1:])
	case "route":
		if len(args) >= 2 && args[1] == "review" {
			return cmdRouteReview(ctx, opts, args[2:])
		}
		return errors.New("route requires subcommand: review")
	case "review":
		if len(args) >= 2 && args[1] == "checkout" {
			return cmdReviewCheckout(ctx, opts, args[2:])
		}
		return errors.New("review requires subcommand: checkout")
	case "worktree":
		return cmdWorktree(opts, args[1:])
	case "session":
		return cmdSession(ctx, opts, args[1:])
	case "reconcile":
		return cmdReconcile(ctx, opts, args[1:])
	case "coordinator":
		return cmdCoordinator(ctx, opts, args[1:])
	case "adoption":
		return cmdAdoption(ctx, opts, args[1:])
	case "parity":
		return cmdParity(ctx, opts, args[1:])
	case "readiness":
		return cmdReadiness(ctx, opts, args[1:])
	case "checkpoint":
		return cmdCheckpoint(ctx, opts, args[1:])
	case "memory":
		return cmdMemory(ctx, opts, args[1:])
	case "packet":
		return cmdPacket(ctx, opts, args[1:])
	case "watcher":
		return cmdWatcher(ctx, opts, args[1:])
	case "release":
		return cmdRelease(ctx, opts, args[1:])
	case "rules":
		return cmdRules(ctx, opts, args[1:])
	case "regression-pack":
		return cmdRegressionPack(opts, args[1:])
	case "prune-stale":
		return cmdPruneStale(ctx, opts, args[1:])
	case "register":
		return cmdRegister(opts, args[1:])
	case "unregister":
		return cmdUnregister(opts, args[1:])
	case "projects":
		return cmdProjects(opts, args[1:])
	case "tracker":
		return cmdTracker(ctx, opts, args[1:])
	case "tui":
		return cmdTUI(ctx, opts, args[1:])
	case "config":
		if len(args) >= 2 && args[1] == "validate" {
			if len(args) > 2 {
				return fmt.Errorf("unexpected config validate arguments: %s", strings.Join(args[2:], " "))
			}
			_, _, path, err := loadConfig(opts)
			if err != nil {
				return err
			}
			fmt.Println("valid", path)
			return nil
		}
	case "agent-guide":
		return cmdAgentGuide(args[1:])
	case "dashboard":
		return cmdDashboard(ctx, opts, args[1:])
	case "server":
		return cmdServer(ctx, opts, args[1:])
	case "db":
		return cmdDB(ctx, opts, args[1:])
	case "version":
		if len(args) > 1 {
			return fmt.Errorf("unexpected version arguments: %s", strings.Join(args[1:], " "))
		}
		fmt.Println(version)
		return nil
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func cmdServer(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("server", "[--listen <addr>] --read-only | --mode api-write-pilot --write")
		return nil
	}
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	listen := fs.String("listen", "", "listen address")
	readOnly := fs.Bool("read-only", false, "serve shared-team API in read-only mode")
	mode := fs.String("mode", "", "server mode")
	write := fs.Bool("write", false, "enable the append-only shared-team write API pilot")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected server arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		serverMode := strings.TrimSpace(cfg.Server.Mode)
		if serverMode == "" {
			serverMode = "disabled"
		}
		if *mode != "" {
			serverMode = strings.TrimSpace(*mode)
		}
		if *readOnly {
			serverMode = "read_only"
			cfg.Server.ReadOnly = true
			cfg.Server.WriteEnabled = false
		}
		if *write {
			serverMode = firstNonEmpty(strings.TrimSpace(*mode), "api-write-pilot")
			cfg.Server.ReadOnly = false
			cfg.Server.WriteEnabled = true
		}
		switch serverMode {
		case "read_only", "api-read-only":
			if cfg.Server.WriteEnabled {
				return errors.New("server read-only mode requires write_enabled = false")
			}
		case "write_pilot", "api-write-pilot", "api_write_pilot":
			if !cfg.Server.WriteEnabled {
				return errors.New("server api-write-pilot requires --write or [server] write_enabled = true")
			}
			if cfg.Server.ReadOnly {
				return errors.New("server api-write-pilot requires [server] read_only = false")
			}
		case "disabled":
			return errors.New("server mode is disabled; pass --read-only or set [server] mode = \"read_only\"")
		default:
			return fmt.Errorf("server mode %q is not implemented; FW-271 supports read-only and append-only api-write-pilot only", serverMode)
		}
		addr := cfg.Server.Listen
		if addr == "" {
			addr = "127.0.0.1:7880"
		}
		if *listen != "" {
			addr = *listen
		}
		cfg.Server.Mode = serverMode
		cfg.Server.Listen = addr
		cfg.Dashboard.ReadOnly = true
		if err := config.Validate(cfg); err != nil {
			return err
		}
		url := dashboard.URL(addr)
		if !isLoopbackAddr(addr) {
			return fmt.Errorf("shared-team server mode is loopback-only until a reviewed deployment/public exposure task; refusing listen address %q", addr)
		}
		fmt.Println("server", url)
		worktrees, err := collectWorktreeStatus(cfg, root)
		if err != nil {
			return err
		}
		return http.ListenAndServe(addr, dashboard.NewWithRoot(s, cfg, roleNames(cfg), dashboardWorktrees(worktrees), root).ReadOnlyAPIHandler())
	})
}

func cmdAdd(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("add requires task id")
	}
	task := store.TaskDefinition{ID: args[0]}
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	title := fs.String("title", "", "task title")
	kind := fs.String("kind", "", "task kind")
	parent := fs.String("parent", "", "parent task id")
	priority := fs.Int("priority", -1, "priority rank")
	sequence := fs.Int("sequence", -1, "sibling sequence")
	role := fs.String("role", "", "owning role")
	notes := fs.String("notes", "", "notes")
	deps := fs.String("dependencies", "", "comma-separated dependency ids")
	var acceptance multiFlag
	fs.Var(&acceptance, "acceptance", "acceptance check; repeatable")
	metadata := addTaskMetadataFlags(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected add arguments: %s", strings.Join(fs.Args(), " "))
	}
	task.Title = *title
	task.Kind = *kind
	task.ParentID = *parent
	task.Role = *role
	task.Notes = *notes
	if *priority >= 0 {
		task.Priority = priority
	}
	if *sequence >= 0 {
		task.Sequence = sequence
	}
	if *deps != "" {
		task.Dependencies = splitCSV(*deps)
	}
	if len(acceptance) > 0 {
		task.AcceptanceChecks = cleanRepeatedValues(acceptance)
	}
	applyTaskMetadataFlags(&task, metadata, nil)
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		if task.Kind == "" {
			task.Kind = config.DefaultTaskKind(cfg)
		}
		if task.Priority == nil {
			task.Priority = config.DefaultPriority(cfg)
		}
		if err := validateTaskMetadata([]store.TaskDefinition{task}, cfg); err != nil {
			return err
		}
		if err := s.AddTask(ctx, task); err != nil {
			return err
		}
		fmt.Println("added", task.ID)
		return nil
	})
}

func cmdSpawn(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("spawn", flag.ContinueOnError)
	id := fs.String("id", "", "task id")
	title := fs.String("title", "", "task title")
	kind := fs.String("kind", "", "task kind")
	parent := fs.String("parent", "", "parent task id")
	child := fs.Bool("child", false, "spawn as child of current task")
	sibling := fs.Bool("sibling", false, "spawn as sibling of current task")
	rootTask := fs.Bool("root", false, "spawn as root task")
	fromTask := fs.String("from-task", "", "current task id")
	priority := fs.Int("priority", -1, "priority rank")
	sequence := fs.Int("sequence", -1, "sibling sequence")
	role := fs.String("role", "", "owning role")
	notes := fs.String("notes", "", "notes")
	deps := fs.String("dependencies", "", "comma-separated dependency ids")
	var acceptance multiFlag
	fs.Var(&acceptance, "acceptance", "acceptance check; repeatable")
	metadata := addTaskMetadataFlags(fs)
	force := fs.Bool("force", false, "suppress granularity warnings")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected spawn arguments: %s", strings.Join(fs.Args(), " "))
	}
	changed := visitedFlags(fs)
	if *id == "" {
		return errors.New("spawn requires --id")
	}
	if *title == "" {
		return errors.New("spawn requires --title")
	}
	if boolCount(*child, *sibling, *rootTask, *parent != "") > 1 {
		return errors.New("choose only one of --child, --sibling, --root, or --parent")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		currentID := *fromTask
		if currentID == "" {
			var err error
			currentID, err = inferCurrentTaskID(ctx, opts, s)
			if err != nil {
				return err
			}
		}
		var current *store.Task
		if currentID != "" {
			task, _, _, _, _, err := s.TaskDetail(ctx, currentID)
			if err != nil {
				return err
			}
			current = &task
		}
		task := store.TaskDefinition{
			ID:       *id,
			Title:    *title,
			Kind:     *kind,
			ParentID: *parent,
			Role:     *role,
			Notes:    *notes,
		}
		if current != nil {
			copyTaskMetadata(&task, current.Definition)
		}
		if *rootTask {
			task.ParentID = ""
		} else if *child {
			if current == nil {
				return errors.New("--child requires a current task or --from-task")
			}
			task.ParentID = current.Definition.ID
			if isLeafKind(current.Definition.Kind) && !*force {
				fmt.Fprintf(os.Stderr, "warning: %s is a leaf task (kind=%s); use --force to suppress this warning\n", current.Definition.ID, current.Definition.Kind)
			}
		} else if task.ParentID == "" {
			if current == nil {
				return errors.New("spawn requires --parent, --root, or an inferred current task")
			}
			task.ParentID = current.Definition.ParentID
		}
		if task.Kind == "" {
			task.Kind = config.DefaultTaskKind(cfg)
		}
		if task.Role == "" {
			if current != nil {
				task.Role = current.Definition.Role
			} else {
				task.Role = resolveRole(opts)
			}
		}
		if *priority >= 0 {
			task.Priority = priority
		} else if current != nil && current.Definition.Priority != nil {
			v := *current.Definition.Priority
			task.Priority = &v
		} else {
			task.Priority = config.DefaultPriority(cfg)
		}
		if *sequence >= 0 {
			task.Sequence = sequence
		}
		if *deps != "" {
			task.Dependencies = splitCSV(*deps)
		}
		if len(acceptance) > 0 {
			task.AcceptanceChecks = cleanRepeatedValues(acceptance)
		}
		applyTaskMetadataFlags(&task, metadata, changed)
		if err := validateTaskMetadata([]store.TaskDefinition{task}, cfg); err != nil {
			return err
		}
		if err := s.AddTask(ctx, task); err != nil {
			return err
		}
		fmt.Println("spawned", task.ID)
		return nil
	})
}

func cmdUpdate(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("update requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	title := fs.String("title", "", "task title")
	kind := fs.String("kind", "", "task kind")
	parent := fs.String("parent", "", "parent task id")
	priority := fs.Int("priority", -1, "priority rank")
	sequence := fs.Int("sequence", -1, "sibling sequence")
	role := fs.String("role", "", "owning role")
	notes := fs.String("notes", "", "notes")
	deps := fs.String("dependencies", "", "comma-separated dependency ids")
	var acceptance multiFlag
	fs.Var(&acceptance, "acceptance", "acceptance check; repeatable")
	metadata := addTaskMetadataFlags(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected update arguments: %s", strings.Join(fs.Args(), " "))
	}
	changed := visitedFlags(fs)
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		current, _, _, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		task := current.Definition
		if changed["title"] {
			task.Title = *title
		}
		if changed["kind"] {
			task.Kind = *kind
		}
		if changed["parent"] {
			task.ParentID = *parent
		}
		if changed["priority"] {
			if *priority >= 0 {
				task.Priority = priority
			} else {
				task.Priority = nil
			}
		}
		if changed["sequence"] {
			if *sequence >= 0 {
				task.Sequence = sequence
			} else {
				task.Sequence = nil
			}
		}
		if changed["role"] {
			task.Role = *role
		}
		if changed["notes"] {
			task.Notes = *notes
		}
		if changed["dependencies"] {
			task.Dependencies = splitCSV(*deps)
		}
		if changed["acceptance"] {
			if len(acceptance) == 0 || len(cleanRepeatedValues(acceptance)) == 0 {
				task.AcceptanceChecks = nil
			} else {
				task.AcceptanceChecks = cleanRepeatedValues(acceptance)
			}
		}
		applyTaskMetadataFlags(&task, metadata, changed)
		if err := validateTaskMetadata([]store.TaskDefinition{task}, cfg); err != nil {
			return err
		}
		if err := s.UpdateTask(ctx, task); err != nil {
			return err
		}
		fmt.Println("updated", task.ID)
		return nil
	})
}

type treeNode struct {
	Task     store.Task `json:"task"`
	Children []treeNode `json:"children,omitempty"`
}

func cmdTree(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("tree requires task id")
	}
	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	depth := fs.Int("depth", -1, "maximum descendant depth")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tree arguments: %s", strings.Join(fs.Args(), " "))
	}
	rootID := args[0]
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		tasks, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		node, ok := buildTree(rootID, tasks, *depth)
		if !ok {
			return store.ErrNotFound
		}
		if opts.JSON {
			return printJSON(node)
		}
		printTree(node, 0)
		return nil
	})
}

type taskListRow struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Role                string   `json:"role"`
	Kind                string   `json:"kind"`
	Status              string   `json:"status"`
	Owner               string   `json:"owner,omitempty"`
	Claimant            string   `json:"claimant,omitempty"`
	Ready               bool     `json:"ready"`
	Dependencies        []string `json:"dependencies,omitempty"`
	DependencySummary   string   `json:"dependency_summary"`
	BlockedDependencies []string `json:"blocked_dependencies,omitempty"`
	MissingDependencies []string `json:"missing_dependencies,omitempty"`
}

func cmdList(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	var statuses multiFlag
	fs.Var(&statuses, "status", "comma-separated status filter; repeatable")
	role := fs.String("role", "", "role filter; defaults to --as when omitted")
	readyOnly := fs.Bool("ready", false, "only include claimable ready tasks")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected list arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		tasks, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		statusFilter := stringSet(splitRepeatedCSV(statuses))
		roleFilter := strings.TrimSpace(*role)
		if roleFilter == "" {
			roleFilter = resolveRole(opts)
		}
		rows := taskListRows(tasks, cfg.States.Terminal)
		var filtered []taskListRow
		for _, row := range rows {
			if len(statusFilter) > 0 && !statusFilter[row.Status] {
				continue
			}
			if roleFilter != "" && row.Role != roleFilter {
				continue
			}
			if *readyOnly && !row.Ready {
				continue
			}
			filtered = append(filtered, row)
		}
		if opts.JSON {
			return printJSON(filtered)
		}
		if len(filtered) == 0 {
			fmt.Println("no tasks matched filters")
			return nil
		}
		for _, row := range filtered {
			owner := firstNonEmpty(row.Claimant, row.Owner, "-")
			ready := "not_ready"
			if row.Ready {
				ready = "ready"
			}
			fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.ID, row.Role, firstNonEmpty(row.Kind, "task"), row.Status, owner, ready, row.DependencySummary, row.Title)
		}
		return nil
	})
}

type timingReport struct {
	TotalSeconds int            `json:"total_seconds"`
	ByRole       map[string]int `json:"by_role"`
	ByTask       map[string]int `json:"by_task"`
}

func cmdTimingReport(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected timing-report arguments: %s", strings.Join(args, " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		tasks, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		report := timingReport{ByRole: map[string]int{}, ByTask: map[string]int{}}
		for _, task := range tasks {
			_, _, evidence, _, _, err := s.TaskDetail(ctx, task.Definition.ID)
			if err != nil {
				return err
			}
			for _, ev := range evidence {
				if ev.DurationSeconds == nil {
					continue
				}
				report.TotalSeconds += *ev.DurationSeconds
				report.ByTask[task.Definition.ID] += *ev.DurationSeconds
				report.ByRole[task.Definition.Role] += *ev.DurationSeconds
			}
		}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("total_seconds: %d\n", report.TotalSeconds)
		fmt.Println("by role:")
		for role, seconds := range report.ByRole {
			fmt.Printf("- %s: %d\n", role, seconds)
		}
		fmt.Println("by task:")
		for taskID, seconds := range report.ByTask {
			fmt.Printf("- %s: %d\n", taskID, seconds)
		}
		return nil
	})
}

type completionHandbackIdleReport struct {
	GeneratedAt    string                                   `json:"generated_at"`
	IncludeClosed  bool                                     `json:"include_closed"`
	AckTimeout     string                                   `json:"ack_timeout"`
	TotalRows      int                                      `json:"total_rows"`
	StaleCount     int                                      `json:"stale_count"`
	CompletedCount int                                      `json:"completed_count"`
	OpenCount      int                                      `json:"open_count"`
	Rows           []completionHandbackIdleRow              `json:"rows"`
	ByTask         map[string]completionHandbackIdleSummary `json:"by_task"`
	ByWorkstream   map[string]completionHandbackIdleSummary `json:"by_workstream"`
}

type completionHandbackIdleRow struct {
	TaskID            string `json:"task_id"`
	Title             string `json:"title,omitempty"`
	Workstream        string `json:"workstream"`
	TaskStatus        string `json:"task_status"`
	CompletionState   string `json:"completion_state,omitempty"`
	DeliveryStatus    string `json:"delivery_status"`
	DeliveryState     string `json:"delivery_state,omitempty"`
	ToRole            string `json:"to_role"`
	NextAction        string `json:"next_action"`
	HandbackAt        string `json:"handback_at"`
	DecisionAt        string `json:"decision_at,omitempty"`
	DecisionOwner     string `json:"decision_owner,omitempty"`
	DecisionSummary   string `json:"decision_summary,omitempty"`
	IdleSeconds       int64  `json:"idle_seconds"`
	Stale             bool   `json:"stale"`
	StaleAge          string `json:"stale_age,omitempty"`
	Provider          string `json:"provider,omitempty"`
	Target            string `json:"target,omitempty"`
	SuggestedAction   string `json:"suggested_action,omitempty"`
	SuggestedCommand  string `json:"suggested_command,omitempty"`
	LiveWindowPhase   string `json:"live_window_phase,omitempty"`
	ApprovalBoundary  string `json:"approval_boundary,omitempty"`
	ActualThreadProof bool   `json:"actual_thread_proof"`
}

type completionHandbackIdleSummary struct {
	Rows             int   `json:"rows"`
	StaleCount       int   `json:"stale_count"`
	CompletedCount   int   `json:"completed_count"`
	OpenCount        int   `json:"open_count"`
	TotalIdleSeconds int64 `json:"total_idle_seconds"`
	MaxIdleSeconds   int64 `json:"max_idle_seconds"`
}

func cmdCompletionHandbackReport(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("completion-handback-report", "[--include-closed] [--format human|markdown]")
		return nil
	}
	fs := flag.NewFlagSet("completion-handback-report", flag.ContinueOnError)
	includeClosed := fs.Bool("include-closed", false, "include tasks in terminal states")
	format := fs.String("format", "human", "human or markdown")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected completion-handback-report arguments: %s", strings.Join(fs.Args(), " "))
	}
	switch *format {
	case "human", "markdown":
	default:
		return fmt.Errorf("unsupported --format %q", *format)
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		report, err := buildCompletionHandbackIdleReport(ctx, cfg, s, *includeClosed, time.Now().UTC())
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(report)
		}
		if *format == "markdown" {
			printCompletionHandbackIdleMarkdown(report)
			return nil
		}
		printCompletionHandbackIdleHuman(report)
		return nil
	})
}

func buildCompletionHandbackIdleReport(ctx context.Context, cfg config.Config, s *store.Store, includeClosed bool, now time.Time) (completionHandbackIdleReport, error) {
	ackTimeout, err := reviewWaitAckTimeout(cfg)
	if err != nil {
		return completionHandbackIdleReport{}, err
	}
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return completionHandbackIdleReport{}, err
	}
	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return completionHandbackIdleReport{}, err
	}
	liveWindows := map[string]livewindow.Status{}
	for _, status := range livewindow.StatusesFromCheckpoints(checkpoints) {
		liveWindows[status.TaskID] = status
	}
	terminal := terminalStatusSet(cfg.States.Terminal)
	report := completionHandbackIdleReport{
		GeneratedAt:   now.Format(time.RFC3339Nano),
		IncludeClosed: includeClosed,
		AckTimeout:    ackTimeout.String(),
		ByTask:        map[string]completionHandbackIdleSummary{},
		ByWorkstream:  map[string]completionHandbackIdleSummary{},
	}
	for _, task := range tasks {
		if !includeClosed && terminal[task.Status] {
			continue
		}
		_, _, evidence, handoffs, _, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return completionHandbackIdleReport{}, err
		}
		notifications, err := s.Notifications(ctx, task.Definition.ID)
		if err != nil {
			return completionHandbackIdleReport{}, err
		}
		liveWindowPhase := liveWindows[task.Definition.ID].Phase
		for _, handback := range completionhandback.RowsWithOptions(task.Definition.ID, handoffs, notifications, completionhandback.RowOptions{
			Now:             now,
			AckTimeout:      ackTimeout,
			TaskStatus:      task.Status,
			LiveWindowPhase: liveWindowPhase,
			Superseded:      completionhandback.SupersedesFromEvidence(evidence),
		}) {
			row := completionHandbackIdleRow{
				TaskID:            task.Definition.ID,
				Title:             task.Definition.Title,
				Workstream:        taskWorkstream(task),
				TaskStatus:        task.Status,
				CompletionState:   handback.CompletionState,
				DeliveryStatus:    handback.DeliveryStatus,
				DeliveryState:     handback.DeliveryState,
				ToRole:            handback.ToRole,
				NextAction:        handback.NextAction,
				HandbackAt:        firstNonEmpty(handback.DeliveredAt, handback.CreatedAt),
				Provider:          handback.Provider,
				Target:            handback.Target,
				SuggestedAction:   handback.SuggestedAction,
				SuggestedCommand:  handback.SuggestedCommand,
				LiveWindowPhase:   handback.LiveWindowPhase,
				ApprovalBoundary:  handback.ApprovalBoundary,
				ActualThreadProof: handback.ActualThreadDelivery,
			}
			if decision, ok := nextCompletionHandbackDecision(checkpoints, handback, row.HandbackAt); ok {
				row.DecisionAt = decision.CreatedAt
				row.DecisionOwner = decision.Owner
				row.DecisionSummary = decision.Summary
			}
			row.IdleSeconds = elapsedSeconds(row.HandbackAt, firstNonEmpty(row.DecisionAt, now.Format(time.RFC3339Nano)))
			if row.DecisionAt == "" && ackTimeout > 0 && time.Duration(row.IdleSeconds)*time.Second >= ackTimeout {
				row.Stale = true
				row.StaleAge = (time.Duration(row.IdleSeconds) * time.Second).Truncate(time.Second).String()
			}
			if handback.Stale {
				row.Stale = true
				row.StaleAge = firstNonEmpty(row.StaleAge, handback.StaleAge)
			}
			report.Rows = append(report.Rows, row)
			addCompletionHandbackIdleSummary(report.ByTask, row.TaskID, row)
			addCompletionHandbackIdleSummary(report.ByWorkstream, row.Workstream, row)
		}
	}
	sort.SliceStable(report.Rows, func(i, j int) bool {
		if report.Rows[i].Stale != report.Rows[j].Stale {
			return report.Rows[i].Stale
		}
		if report.Rows[i].IdleSeconds != report.Rows[j].IdleSeconds {
			return report.Rows[i].IdleSeconds > report.Rows[j].IdleSeconds
		}
		return report.Rows[i].TaskID < report.Rows[j].TaskID
	})
	report.TotalRows = len(report.Rows)
	for _, row := range report.Rows {
		if row.Stale {
			report.StaleCount++
		}
		if row.DecisionAt != "" {
			report.CompletedCount++
		} else {
			report.OpenCount++
		}
	}
	return report, nil
}

func nextCompletionHandbackDecision(checkpoints []store.Checkpoint, handback completionhandback.Handback, origin string) (store.Checkpoint, bool) {
	originAt, err := time.Parse(time.RFC3339Nano, origin)
	if err != nil {
		return store.Checkpoint{}, false
	}
	var best store.Checkpoint
	for _, checkpoint := range checkpoints {
		if checkpoint.TaskID != handback.TaskID {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339Nano, checkpoint.CreatedAt)
		if err != nil || !createdAt.After(originAt) {
			continue
		}
		owner := strings.TrimSpace(checkpoint.Owner)
		if owner != handback.ToRole && owner != "arch" && owner != "architecture" && owner != "coordinator" && owner != "orchestrator" {
			continue
		}
		if best.CreatedAt == "" || checkpoint.CreatedAt < best.CreatedAt {
			best = checkpoint
		}
	}
	return best, best.CreatedAt != ""
}

func elapsedSeconds(start, end string) int64 {
	startAt, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return 0
	}
	endAt, err := time.Parse(time.RFC3339Nano, end)
	if err != nil || endAt.Before(startAt) {
		return 0
	}
	return int64(endAt.Sub(startAt).Truncate(time.Second).Seconds())
}

func taskWorkstream(task store.Task) string {
	return firstNonEmpty(task.Definition.Profile, task.Definition.OwningDomain, task.Definition.Role, "unassigned")
}

func addCompletionHandbackIdleSummary(m map[string]completionHandbackIdleSummary, key string, row completionHandbackIdleRow) {
	summary := m[key]
	summary.Rows++
	summary.TotalIdleSeconds += row.IdleSeconds
	if row.IdleSeconds > summary.MaxIdleSeconds {
		summary.MaxIdleSeconds = row.IdleSeconds
	}
	if row.Stale {
		summary.StaleCount++
	}
	if row.DecisionAt != "" {
		summary.CompletedCount++
	} else {
		summary.OpenCount++
	}
	m[key] = summary
}

func printCompletionHandbackIdleHuman(report completionHandbackIdleReport) {
	fmt.Printf("completion_handback_idle_report: rows=%d stale=%d completed=%d open=%d ack_timeout=%s\n", report.TotalRows, report.StaleCount, report.CompletedCount, report.OpenCount, report.AckTimeout)
	fmt.Println("by workstream:")
	for _, key := range sortedSummaryKeys(report.ByWorkstream) {
		summary := report.ByWorkstream[key]
		fmt.Printf("- %s: rows=%d stale=%d completed=%d open=%d max_idle_seconds=%d\n", key, summary.Rows, summary.StaleCount, summary.CompletedCount, summary.OpenCount, summary.MaxIdleSeconds)
	}
	fmt.Println("rows:")
	for _, row := range report.Rows {
		fmt.Printf("- %s workstream=%s status=%s completion_state=%s delivery_status=%s stale=%t idle_seconds=%d decision_at=%s next_action=%s\n", row.TaskID, row.Workstream, row.TaskStatus, firstNonEmpty(row.CompletionState, "unspecified"), row.DeliveryStatus, row.Stale, row.IdleSeconds, firstNonEmpty(row.DecisionAt, "none"), row.NextAction)
	}
}

func printCompletionHandbackIdleMarkdown(report completionHandbackIdleReport) {
	fmt.Println("# Completion Handback Idle Report")
	fmt.Printf("\nRows: %d  Stale: %d  Completed: %d  Open: %d  Ack timeout: `%s`\n", report.TotalRows, report.StaleCount, report.CompletedCount, report.OpenCount, report.AckTimeout)
	fmt.Println("\n## By Workstream")
	fmt.Println("| Workstream | Rows | Stale | Completed | Open | Max idle seconds |")
	fmt.Println("|---|---:|---:|---:|---:|---:|")
	for _, key := range sortedSummaryKeys(report.ByWorkstream) {
		summary := report.ByWorkstream[key]
		fmt.Printf("| %s | %d | %d | %d | %d | %d |\n", key, summary.Rows, summary.StaleCount, summary.CompletedCount, summary.OpenCount, summary.MaxIdleSeconds)
	}
	fmt.Println("\n## Rows")
	fmt.Println("| Task | Workstream | State | Delivery | Stale | Idle seconds | Decision | Next action |")
	fmt.Println("|---|---|---|---|---:|---:|---|---|")
	for _, row := range report.Rows {
		fmt.Printf("| %s | %s | %s | %s | %t | %d | %s | %s |\n", row.TaskID, row.Workstream, firstNonEmpty(row.CompletionState, "unspecified"), row.DeliveryStatus, row.Stale, row.IdleSeconds, firstNonEmpty(row.DecisionAt, "none"), row.NextAction)
	}
}

func sortedSummaryKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type statusReport struct {
	Total    int                       `json:"total"`
	ByStatus map[string]int            `json:"by_status"`
	ByRole   map[string]map[string]int `json:"by_role"`
}

func cmdStatusReport(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected status-report arguments: %s", strings.Join(args, " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		tasks, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		report := statusReport{ByStatus: map[string]int{}, ByRole: map[string]map[string]int{}}
		for _, task := range tasks {
			report.Total++
			report.ByStatus[task.Status]++
			role := task.Definition.Role
			if role == "" {
				role = "unassigned"
			}
			if report.ByRole[role] == nil {
				report.ByRole[role] = map[string]int{}
			}
			report.ByRole[role][task.Status]++
		}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("total: %d\n", report.Total)
		fmt.Println("by status:")
		for status, count := range report.ByStatus {
			fmt.Printf("- %s: %d\n", status, count)
		}
		fmt.Println("by role:")
		for role, counts := range report.ByRole {
			fmt.Printf("- %s", role)
			for status, count := range counts {
				fmt.Printf(" %s=%d", status, count)
			}
			fmt.Println()
		}
		return nil
	})
}

func cmdGitCheck(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("git-check", flag.ContinueOnError)
	base := fs.String("base", "", "base ref")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected git-check arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	if *base == "" {
		*base = cfg.Fairway.MainBranch
	}
	status, err := fairwaygit.Check(root, *base)
	if err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(status)
	}
	fmt.Printf("root: %s\nbranch: %s\nbase: %s\ndirty: %t\nstaged: %t\nuntracked: %t\nahead: %d\nbehind: %d\nbase_ancestor: %t\n",
		status.Root, status.Branch, status.Base, status.Dirty, status.Staged, status.Untracked, status.Ahead, status.Behind, status.BaseAncestor)
	if len(status.ChangedFiles) > 0 {
		fmt.Println("changed_files:")
		for _, path := range status.ChangedFiles {
			fmt.Printf("- %s\n", path)
		}
	}
	return nil
}

type preflightReport struct {
	OK     bool               `json:"ok"`
	Role   string             `json:"role"`
	Git    fairwaygit.Status  `json:"git"`
	Issues []string           `json:"issues"`
	Config preflightConfigRef `json:"config"`
}

type preflightConfigRef struct {
	Path string `json:"path"`
	Root string `json:"root"`
}

type workflowCheckReport struct {
	OK                   bool                     `json:"ok"`
	Mode                 string                   `json:"mode"`
	Git                  fairwaygit.Status        `json:"git"`
	AllowedArtifactPaths []string                 `json:"allowed_artifact_paths,omitempty"`
	DirtyPaths           []string                 `json:"dirty_paths,omitempty"`
	Reconcile            reconcile.ActiveReport   `json:"reconcile"`
	Closeout             reconcile.CloseoutReport `json:"closeout,omitempty"`
	RuleEvaluations      []ruleEvidenceEvaluation `json:"rule_evaluations,omitempty"`
	Issues               []string                 `json:"issues"`
	Warnings             []string                 `json:"warnings,omitempty"`
	Recommendations      []string                 `json:"recommendations,omitempty"`
}

type worktreeCleanliness struct {
	Dirty                bool
	DirtyPaths           []string
	AllowedArtifactPaths []string
}

func cmdPreflight(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	roleFlag := fs.String("role", "", "role name")
	base := fs.String("base", "", "base ref")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected preflight arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg, root, path, err := loadConfig(opts)
	if err != nil {
		return err
	}
	role := *roleFlag
	if role == "" {
		role = resolveRole(opts)
	}
	if *base == "" {
		*base = cfg.Fairway.MainBranch
	}
	gitStatus, err := fairwaygit.Check(root, *base)
	if err != nil {
		return err
	}
	report := preflightReport{
		OK:     true,
		Role:   role,
		Git:    gitStatus,
		Config: preflightConfigRef{Path: path, Root: root},
	}
	roleSet := config.RoleSet(cfg)
	roleBranches := map[string]string{}
	for _, configured := range cfg.Roles {
		roleBranches[configured.Name] = configured.Branch
	}
	if len(roleSet) > 0 {
		if role == "" {
			report.Issues = append(report.Issues, "role is required when roles are configured")
		} else if !roleSet[role] {
			report.Issues = append(report.Issues, fmt.Sprintf("unknown role %q", role))
		} else if expected := roleBranches[role]; expected != "" && gitStatus.Branch != expected {
			report.Issues = append(report.Issues, fmt.Sprintf("branch %q does not match role %q branch %q", gitStatus.Branch, role, expected))
		}
	}
	if gitStatus.Dirty {
		report.Issues = append(report.Issues, "worktree has uncommitted changes")
	}
	if *base != "" && !gitStatus.BaseAncestor {
		report.Issues = append(report.Issues, fmt.Sprintf("HEAD is not based on %q", *base))
	}
	if gitStatus.Behind > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("branch is %d commit(s) behind %q", gitStatus.Behind, *base))
	}
	report.OK = len(report.Issues) == 0
	if opts.JSON {
		if err := printJSON(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("preflight: %t\nrole: %s\nbranch: %s\nbase: %s\n", report.OK, report.Role, report.Git.Branch, report.Git.Base)
		if len(report.Issues) > 0 {
			fmt.Println("issues:")
			for _, issue := range report.Issues {
				fmt.Printf("- %s\n", issue)
			}
		}
	}
	if !report.OK {
		return errors.New("preflight failed")
	}
	return nil
}

func cmdWorkflow(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("workflow", "check|closeout")
		return nil
	}
	switch args[0] {
	case "check":
		return cmdWorkflowCheck(ctx, opts, args[1:])
	case "closeout":
		return cmdWorkflowCloseout(ctx, opts, args[1:])
	default:
		return errors.New("workflow requires subcommand: check|closeout")
	}
}

func cmdWorkflowCheck(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("workflow check", flag.ContinueOnError)
	base := fs.String("base", "", "base ref")
	mode := fs.String("mode", "task", "workflow mode: task, deploy, close")
	taskID := fs.String("task-id", "", "task id for closeout mode")
	preserveReason := fs.String("preserve-branch-reason", "", "explicit reason to preserve an unmerged task branch in closeout mode")
	requireClean := fs.Bool("require-clean", false, "fail if the worktree is dirty")
	requirePushed := fs.Bool("require-pushed", false, "fail if HEAD has unpushed commits")
	staleAfter := fs.Duration("stale-checkpoint-after", 0, "warn on active checkpoints older than duration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected workflow check arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		if *base == "" {
			*base = cfg.Fairway.MainBranch
		}
		gitStatus, err := fairwaygit.Check(root, *base)
		if err != nil {
			return err
		}
		cleanliness := evaluateWorktreeCleanliness(gitStatus, localArtifactAllowlist(cfg, nil))
		reconcileReport, err := reconcile.Active(ctx, s, reconcile.ActiveOptions{
			Terminal:             cfg.States.Terminal,
			StaleCheckpointAfter: *staleAfter,
		})
		if err != nil {
			return err
		}
		report := workflowCheckReport{
			OK:                   true,
			Mode:                 *mode,
			Git:                  gitStatus,
			AllowedArtifactPaths: cleanliness.AllowedArtifactPaths,
			DirtyPaths:           cleanliness.DirtyPaths,
			Reconcile:            reconcileReport,
		}
		var closeoutReport reconcile.CloseoutReport
		if *mode == "close" {
			if *taskID == "" {
				report.Issues = append(report.Issues, "close mode requires --task-id")
			} else {
				task, _, evidence, _, _, err := s.TaskDetail(ctx, *taskID)
				if err != nil {
					return err
				}
				cleanliness = evaluateWorktreeCleanliness(gitStatus, localArtifactAllowlist(cfg, evidence))
				report.AllowedArtifactPaths = cleanliness.AllowedArtifactPaths
				report.DirtyPaths = cleanliness.DirtyPaths
				closeoutGit := buildCloseoutGit(root, *base, task, gitStatus, localArtifactAllowlist(cfg, evidence))
				closeoutReport, err = reconcile.Closeout(ctx, s, reconcile.CloseoutOptions{
					TaskID:         *taskID,
					Role:           task.Definition.Role,
					Terminal:       cfg.States.Terminal,
					Git:            closeoutGit,
					PreserveReason: *preserveReason,
				})
				if err != nil {
					return err
				}
				report.Closeout = closeoutReport
				if !closeoutReport.OK {
					report.Issues = append(report.Issues, fmt.Sprintf("lane closeout has %d blocker(s)", closeoutReport.Summary.Blockers))
					report.Recommendations = append(report.Recommendations, "run fairway workflow closeout "+*taskID+" --dry-run and resolve lane closeout debt")
				}
				ruleEvaluations, err := ruleEvidenceEvaluations(cfg, root, task, evidence)
				if err != nil {
					return err
				}
				report.RuleEvaluations = ruleEvaluations
				for _, evaluation := range ruleEvaluations {
					if evaluation.Status != "missing" {
						continue
					}
					message := ruleEvidenceMessage(evaluation)
					if evaluation.Mode == "blocking" {
						report.Issues = append(report.Issues, message)
					} else {
						report.Warnings = append(report.Warnings, message)
					}
				}
			}
		}
		if cleanliness.Dirty {
			message := "worktree has uncommitted changes"
			if *requireClean {
				report.Issues = append(report.Issues, message)
			} else {
				report.Warnings = append(report.Warnings, message)
			}
			if changedDocs(cleanliness.DirtyPaths) {
				report.Recommendations = append(report.Recommendations, "commit completed documentation updates as their own coherent slice")
			}
			if changedCode(cleanliness.DirtyPaths) {
				report.Recommendations = append(report.Recommendations, "run focused tests before committing changed code")
			}
		}
		if !gitStatus.HasUpstream {
			report.Warnings = append(report.Warnings, "branch has no upstream; push tracking cannot be evaluated")
		} else if gitStatus.Unpushed > 0 {
			message := fmt.Sprintf("branch has %d unpushed commit(s)", gitStatus.Unpushed)
			if *requirePushed {
				report.Issues = append(report.Issues, message)
			} else {
				report.Warnings = append(report.Warnings, message)
			}
			report.Recommendations = append(report.Recommendations, "push integration-ready commits so CI can run")
		}
		if gitStatus.Unpulled > 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("branch is %d commit(s) behind upstream", gitStatus.Unpulled))
		}
		if *base != "" && !gitStatus.BaseAncestor {
			report.Issues = append(report.Issues, fmt.Sprintf("HEAD is not based on %q", *base))
		}
		if !reconcileReport.OK {
			report.Issues = append(report.Issues, fmt.Sprintf("active reconciliation has %d finding(s)", len(reconcileReport.Findings)))
			report.Recommendations = append(report.Recommendations, "run fairway reconcile active --dry-run and resolve stale/unattended work")
		}
		switch *mode {
		case "deploy":
			report.Recommendations = append(report.Recommendations, "create one deploy-run task for the release/deploy attempt")
			report.Recommendations = append(report.Recommendations, "create CI-FIX/CD-FIX/UAT-BUG/OPS-FIX/HARNESS-FIX/DOC-FIX follow-ups only for actionable findings")
			if cleanliness.Dirty {
				report.Issues = append(report.Issues, "deploy mode requires a clean committed source SHA")
			}
			if gitStatus.HasUpstream && gitStatus.Unpushed > 0 {
				report.Issues = append(report.Issues, "deploy mode requires pushed commits so CI can run")
			}
		case "task", "close":
		default:
			report.Warnings = append(report.Warnings, fmt.Sprintf("unknown workflow mode %q; expected task, close, or deploy", *mode))
		}
		report.OK = len(report.Issues) == 0
		if opts.JSON {
			if err := printJSON(report); err != nil {
				return err
			}
		} else {
			fmt.Printf("workflow_check: %t\nmode: %s\nbranch: %s\nbase: %s\n", report.OK, report.Mode, report.Git.Branch, report.Git.Base)
			if report.Git.HasUpstream {
				fmt.Printf("upstream: %s\nunpushed: %d\nunpulled: %d\n", report.Git.Upstream, report.Git.Unpushed, report.Git.Unpulled)
			}
			printWorktreeCleanliness(report.DirtyPaths, report.AllowedArtifactPaths)
			if len(report.Issues) > 0 {
				fmt.Println("issues:")
				for _, issue := range report.Issues {
					fmt.Printf("- %s\n", issue)
				}
			}
			if len(report.Warnings) > 0 {
				fmt.Println("warnings:")
				for _, warning := range report.Warnings {
					fmt.Printf("- %s\n", warning)
				}
			}
			if len(report.Recommendations) > 0 {
				fmt.Println("recommendations:")
				for _, recommendation := range uniqueStrings(report.Recommendations) {
					fmt.Printf("- %s\n", recommendation)
				}
			}
			if len(report.RuleEvaluations) > 0 {
				fmt.Println("rule_evidence:")
				for _, evaluation := range report.RuleEvaluations {
					fmt.Printf("- %s: %s mode=%s evidence=%s\n", evaluation.RuleID, evaluation.Status, evaluation.Mode, strings.Join(evaluation.RequiredEvidence, ","))
				}
			}
			if *mode == "close" && closeoutReport.TaskID != "" {
				printCloseoutReport(closeoutReport)
			}
		}
		if !report.OK {
			return errors.New("workflow check failed")
		}
		return nil
	})
}

func cmdWorkflowCloseout(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("workflow closeout", "<task-id> [--dry-run] [--apply] [--base <ref>] [--preserve-branch-reason <reason>]")
		return nil
	}
	if len(args) < 1 {
		return errors.New("workflow closeout requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("workflow closeout", flag.ContinueOnError)
	base := fs.String("base", "", "base ref")
	_ = fs.Bool("dry-run", true, "report lane closeout state without mutation")
	apply := fs.Bool("apply", false, "apply safe cleanup; currently refuses destructive branch/worktree cleanup")
	preserveReason := fs.String("preserve-branch-reason", "", "explicit reason to preserve an unmerged task branch")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected workflow closeout arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		if *base == "" {
			*base = cfg.Fairway.MainBranch
		}
		task, _, evidence, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		gitStatus, err := fairwaygit.Check(root, *base)
		if err != nil {
			return err
		}
		closeoutGit := buildCloseoutGit(root, *base, task, gitStatus, localArtifactAllowlist(cfg, evidence))
		report, err := reconcile.Closeout(ctx, s, reconcile.CloseoutOptions{
			TaskID:         taskID,
			Role:           task.Definition.Role,
			Terminal:       cfg.States.Terminal,
			Git:            closeoutGit,
			PreserveReason: *preserveReason,
		})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(report)
		}
		printCloseoutReport(report)
		if !report.OK {
			return errors.New("workflow closeout failed")
		}
		if *apply {
			if !report.Apply.DeleteRemoteBranch {
				fmt.Println("closeout apply: no eligible remote branch cleanup")
				return nil
			}
			if err := fairwaygit.DeleteRemoteBranch(root, report.Apply.Remote, report.Apply.Branch); err != nil {
				return err
			}
			fmt.Printf("deleted remote branch %s/%s\n", firstNonEmpty(report.Apply.Remote, "origin"), report.Apply.Branch)
		}
		return nil
	})
}

func buildCloseoutGit(root, base string, task store.Task, gitStatus fairwaygit.Status, allowedArtifacts []string) reconcile.CloseoutGit {
	branch := firstNonEmpty(task.Branch, gitStatus.Branch)
	worktreePath := gitStatus.Root
	cleanliness := evaluateWorktreeCleanliness(gitStatus, allowedArtifacts)
	worktreeDirty := cleanliness.Dirty
	allowedArtifactPaths := cleanliness.AllowedArtifactPaths
	if branch != "" && branch != gitStatus.Branch {
		if worktree, ok := worktreeForBranch(root, branch); ok {
			worktreePath = worktree.Path
			if status, err := fairwaygit.Check(worktree.Path, base); err == nil {
				cleanliness = evaluateWorktreeCleanliness(status, allowedArtifacts)
				worktreeDirty = cleanliness.Dirty
				allowedArtifactPaths = cleanliness.AllowedArtifactPaths
			}
		}
	}
	return reconcile.CloseoutGit{
		Branch:               branch,
		Base:                 base,
		WorktreePath:         worktreePath,
		WorktreeDirty:        worktreeDirty,
		AllowedArtifactPaths: allowedArtifactPaths,
		BranchExists:         fairwaygit.BranchExists(root, branch),
		BranchMerged:         fairwaygit.BranchMerged(root, branch, base),
		RemoteBranchExists:   branch != base && fairwaygit.RemoteBranchExists(root, branch),
	}
}

func localArtifactAllowlist(cfg config.Config, evidence []store.Evidence) []string {
	allowlist := append([]string(nil), cfg.Fairway.LocalArtifactPaths...)
	for _, ev := range evidence {
		artifact := strings.TrimSpace(ev.ArtifactPath)
		if artifact == "" || strings.Contains(artifact, "://") {
			continue
		}
		allowlist = append(allowlist, artifact)
	}
	return uniqueStrings(allowlist)
}

func evaluateWorktreeCleanliness(status fairwaygit.Status, allowedArtifacts []string) worktreeCleanliness {
	var result worktreeCleanliness
	for _, path := range status.TrackedChangedFiles {
		result.DirtyPaths = append(result.DirtyPaths, path)
	}
	for _, path := range status.UntrackedFiles {
		if localArtifactPathAllowed(path, allowedArtifacts) {
			result.AllowedArtifactPaths = append(result.AllowedArtifactPaths, path)
			continue
		}
		result.DirtyPaths = append(result.DirtyPaths, path)
	}
	if len(status.TrackedChangedFiles) == 0 && len(status.UntrackedFiles) == 0 && status.Dirty {
		result.DirtyPaths = append(result.DirtyPaths, status.ChangedFiles...)
	}
	result.Dirty = len(result.DirtyPaths) > 0
	sort.Strings(result.DirtyPaths)
	sort.Strings(result.AllowedArtifactPaths)
	return result
}

func localArtifactPathAllowed(pathValue string, allowlist []string) bool {
	pathValue = normalizeLocalArtifactPath(pathValue)
	if pathValue == "" {
		return false
	}
	for _, allowed := range allowlist {
		allowed = normalizeLocalArtifactPath(allowed)
		if allowed == "" {
			continue
		}
		if pathValue == allowed || strings.HasPrefix(pathValue, strings.TrimSuffix(allowed, "/")+"/") {
			return true
		}
	}
	return false
}

func normalizeLocalArtifactPath(pathValue string) string {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" || strings.Contains(pathValue, "://") {
		return ""
	}
	pathValue = filepath.ToSlash(filepath.Clean(pathValue))
	pathValue = strings.TrimPrefix(pathValue, "./")
	if pathValue == "." {
		return ""
	}
	return pathValue
}

func printWorktreeCleanliness(dirtyPaths, allowedArtifactPaths []string) {
	if len(dirtyPaths) > 0 {
		fmt.Println("dirty_paths:")
		for _, path := range dirtyPaths {
			fmt.Printf("- %s\n", path)
		}
	}
	if len(allowedArtifactPaths) > 0 {
		fmt.Println("allowed_local_artifacts:")
		for _, path := range allowedArtifactPaths {
			fmt.Printf("- %s\n", path)
		}
	}
}

func worktreeForBranch(root, branch string) (fairwaygit.Worktree, bool) {
	worktrees, err := fairwaygit.Worktrees(root)
	if err != nil {
		return fairwaygit.Worktree{}, false
	}
	for _, worktree := range worktrees {
		if worktree.Branch == branch {
			return worktree, true
		}
	}
	return fairwaygit.Worktree{}, false
}

func printCloseoutReport(report reconcile.CloseoutReport) {
	fmt.Printf("lane_closeout: %t\ntask: %s\nrole: %s\nbranch: %s\nworktree: %s\ncommit: %s\n", report.OK, report.TaskID, report.Role, report.Branch, report.Worktree, report.Commit)
	fmt.Printf("summary: blockers=%d warnings=%d active_sessions=%d active_watchers=%d missing_review_domains=%d review_notifications=%d missing_commits=%d verification_evidence=%d pending_verification=%d dirty_worktrees=%d unmerged_branches=%d remote_branches=%d remote_branches_without_intent=%d safe_branches=%d preserved_branches=%d\n",
		report.Summary.Blockers,
		report.Summary.Warnings,
		report.Summary.ActiveSessions,
		report.Summary.ActiveWatchers,
		report.Summary.MissingReviewDomains,
		report.Summary.ReviewNotifications,
		report.Summary.MissingCommits,
		report.Summary.VerificationEvidence,
		report.Summary.PendingVerification,
		report.Summary.DirtyWorktrees,
		report.Summary.UnmergedBranches,
		report.Summary.RemoteBranchesPresent,
		report.Summary.RemoteBranchesNoIntent,
		report.Summary.SafeToDeleteBranches,
		report.Summary.PreservedBranches,
	)
	if len(report.Findings) == 0 {
		fmt.Println("findings: none")
		return
	}
	fmt.Println("findings:")
	for _, finding := range report.Findings {
		domains := ""
		if len(finding.MissingDomains) > 0 {
			domains = " domains=" + strings.Join(finding.MissingDomains, ",")
		}
		extra := domains
		if finding.Commit != "" {
			extra += " commit=" + finding.Commit
		}
		if finding.EvidenceType != "" {
			extra += " evidence=" + finding.EvidenceType
		}
		if finding.PushIntent != "" {
			extra += " push_intent=" + finding.PushIntent
		}
		if finding.Domain != "" {
			extra += " domain=" + finding.Domain
		}
		if finding.NotificationStatus != "" {
			extra += " notification_status=" + finding.NotificationStatus
		}
		if finding.Path != "" {
			extra += " path=" + finding.Path
		}
		fmt.Printf("- %s %s action=%s%s reason=%s\n", finding.Severity, finding.Kind, finding.Action, extra, finding.Reason)
	}
}

func cmdBatch(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("batch", "create|add|remove|evidence|link|show|list")
		return nil
	}
	switch args[0] {
	case "create":
		return cmdBatchCreate(ctx, opts, args[1:])
	case "add":
		return cmdBatchAdd(ctx, opts, args[1:])
	case "remove":
		return cmdBatchRemove(ctx, opts, args[1:])
	case "evidence":
		return cmdBatchEvidence(ctx, opts, args[1:])
	case "link":
		return cmdBatchLink(ctx, opts, args[1:])
	case "show":
		return cmdBatchShow(ctx, opts, args[1:])
	case "list":
		return cmdBatchList(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown batch subcommand %q", args[0])
	}
}

func cmdBatchCreate(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("batch create", "<batch-id> --title <title> [--task <id>] [--validation-command <cmd>] [--review-domain <domain>] [--branch <branch>] [--worktree <path>] [--expected-ci <run>] [--deploy-run-id <id>] [--pipeline-id <id-or-url>]")
		return nil
	}
	if len(args) < 1 {
		return errors.New("batch create requires batch id")
	}
	batchID := args[0]
	fs := flag.NewFlagSet("batch create", flag.ContinueOnError)
	title := fs.String("title", "", "batch title")
	branch := fs.String("branch", "", "shared branch")
	worktree := fs.String("worktree", "", "shared worktree path")
	rollback := fs.String("rollback-criteria", "", "rollback criteria")
	split := fs.String("split-criteria", "", "split criteria")
	expectedCI := fs.String("expected-ci", "", "expected CI/deploy run")
	deployRunID := fs.String("deploy-run-id", "", "deploy-run task or run id")
	pipelineID := fs.String("pipeline-id", "", "pipeline id or URL")
	var commands multiFlag
	var reviewDomains multiFlag
	var tasks multiFlag
	fs.Var(&commands, "validation-command", "shared validation command; may repeat")
	fs.Var(&reviewDomains, "review-domain", "required review domain; may repeat or comma-separate")
	fs.Var(&tasks, "task", "task id to include; may repeat")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected batch create arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *title == "" {
		return errors.New("--title is required")
	}
	batch := store.WorkBatch{
		ID:                 batchID,
		Title:              *title,
		Branch:             *branch,
		WorktreePath:       *worktree,
		ValidationCommands: commands,
		ReviewDomains:      splitRepeatedCSV(reviewDomains),
		RollbackCriteria:   *rollback,
		SplitCriteria:      *split,
		ExpectedCI:         *expectedCI,
		DeployRunID:        *deployRunID,
		PipelineID:         *pipelineID,
		Tasks:              tasks,
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if err := s.UpsertWorkBatch(ctx, batch); err != nil {
			return err
		}
		if opts.JSON {
			created, evidence, err := s.WorkBatch(ctx, batchID)
			if err != nil {
				return err
			}
			return printJSON(struct {
				Batch    store.WorkBatch           `json:"batch"`
				Evidence []store.WorkBatchEvidence `json:"evidence"`
			}{created, evidence})
		}
		fmt.Printf("batch %s created\n", batchID)
		return nil
	})
}

func cmdBatchAdd(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("batch add", "<batch-id> <task-id> [task-id...]")
		return nil
	}
	if len(args) < 2 {
		return errors.New("batch add requires batch id and task id")
	}
	batchID, taskIDs := args[0], args[1:]
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if err := s.AddTasksToWorkBatch(ctx, batchID, taskIDs); err != nil {
			return err
		}
		if !opts.JSON {
			fmt.Printf("batch %s added %d task(s)\n", batchID, len(taskIDs))
		}
		return nil
	})
}

func cmdBatchRemove(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("batch remove", "<batch-id> <task-id> [task-id...]")
		return nil
	}
	if len(args) < 2 {
		return errors.New("batch remove requires batch id and task id")
	}
	batchID, taskIDs := args[0], args[1:]
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if err := s.RemoveTasksFromWorkBatch(ctx, batchID, taskIDs); err != nil {
			return err
		}
		if !opts.JSON {
			fmt.Printf("batch %s removed %d task(s)\n", batchID, len(taskIDs))
		}
		return nil
	})
}

func cmdBatchEvidence(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("batch evidence", "<batch-id> --command-text <cmd> --result <pass|partial|fail> [--artifact <path>] [--artifact-type <type>] [--notes <text>] [--map-to-tasks=false]")
		return nil
	}
	if len(args) < 1 {
		return errors.New("batch evidence requires batch id")
	}
	batchID := args[0]
	fs := flag.NewFlagSet("batch evidence", flag.ContinueOnError)
	commandText := fs.String("command-text", "", "command")
	result := fs.String("result", "", "result")
	artifact := fs.String("artifact", "", "artifact")
	artifactType := fs.String("artifact-type", "work-batch", "artifact type")
	notes := fs.String("notes", "", "notes")
	mapToTasks := fs.Bool("map-to-tasks", true, "record mapped evidence rows on member tasks")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected batch evidence arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *commandText == "" {
		return errors.New("--command-text is required")
	}
	if *result == "" {
		return errors.New("--result is required")
	}
	ev := store.WorkBatchEvidence{BatchID: batchID, CommandText: *commandText, Result: *result, ArtifactPath: *artifact, ArtifactType: *artifactType, Notes: *notes}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		recorded, err := s.RecordWorkBatchEvidence(ctx, batchID, ev, *mapToTasks)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(recorded)
		}
		fmt.Printf("batch %s evidence recorded\n", batchID)
		return nil
	})
}

func cmdBatchLink(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("batch link", "<batch-id> [--deploy-run-id <id>] [--pipeline-id <id-or-url>]")
		return nil
	}
	if len(args) < 1 {
		return errors.New("batch link requires batch id")
	}
	batchID := args[0]
	fs := flag.NewFlagSet("batch link", flag.ContinueOnError)
	deployRunID := fs.String("deploy-run-id", "", "deploy-run task or run id")
	pipelineID := fs.String("pipeline-id", "", "pipeline id or URL")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected batch link arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *deployRunID == "" && *pipelineID == "" {
		return errors.New("batch link requires --deploy-run-id or --pipeline-id")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if err := s.LinkWorkBatch(ctx, batchID, *deployRunID, *pipelineID); err != nil {
			return err
		}
		if !opts.JSON {
			fmt.Printf("batch %s linked\n", batchID)
		}
		return nil
	})
}

func cmdBatchShow(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("batch show", "<batch-id>")
		return nil
	}
	if len(args) < 1 {
		return errors.New("batch show requires batch id")
	}
	batchID := args[0]
	if len(args) > 1 {
		return fmt.Errorf("unexpected batch show arguments: %s", strings.Join(args[1:], " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		batch, evidence, err := s.WorkBatch(ctx, batchID)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(struct {
				Batch    store.WorkBatch           `json:"batch"`
				Evidence []store.WorkBatchEvidence `json:"evidence"`
			}{batch, evidence})
		}
		fmt.Printf("%s %s\nbranch: %s\nworktree: %s\ndeploy_run: %s\npipeline: %s\n", batch.ID, batch.Title, fallback(batch.Branch, "none"), fallback(batch.WorktreePath, "none"), fallback(batch.DeployRunID, "none"), fallback(batch.PipelineID, "none"))
		fmt.Println("tasks:")
		for _, taskID := range batch.Tasks {
			fmt.Printf("- %s\n", taskID)
		}
		fmt.Println("validation:")
		for _, command := range batch.ValidationCommands {
			fmt.Printf("- %s\n", command)
		}
		fmt.Println("evidence:")
		for _, ev := range evidence {
			fmt.Printf("- %s %s %s\n", ev.Result, ev.CommandText, ev.ArtifactPath)
		}
		return nil
	})
}

func cmdBatchList(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("batch list", "")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected batch list arguments: %s", strings.Join(args, " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		batches, err := s.WorkBatches(ctx)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(batches)
		}
		for _, batch := range batches {
			fmt.Printf("%s\t%d tasks\t%s\t%s\n", batch.ID, len(batch.Tasks), fallback(batch.Branch, "no-branch"), batch.Title)
		}
		if len(batches) == 0 {
			fmt.Println("no work batches")
		}
		return nil
	})
}

func splitRepeatedCSV(values []string) []string {
	var out []string
	for _, value := range values {
		out = append(out, splitCSV(value)...)
	}
	return out
}

func cmdAudit(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("audit", "work-coverage|ci-learning|failure-routing|notifications|docs-backlog")
		return nil
	}
	switch args[0] {
	case "work-coverage":
		return cmdAuditWorkCoverage(ctx, opts, args[1:])
	case "ci-learning":
		return cmdAuditCILearning(ctx, opts, "ci-learning", args[1:])
	case "failure-routing":
		return cmdAuditCILearning(ctx, opts, "failure-routing", args[1:])
	case "notifications":
		return cmdAuditNotifications(ctx, opts, args[1:])
	case "docs-backlog":
		return cmdAuditDocsBacklog(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown audit subcommand %q", args[0])
	}
}

func cmdAuditDocsBacklog(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway audit docs-backlog [--doc <path>]...")
		fmt.Println("  Advisory coordination docs-to-backlog coverage audit; does not mutate task, review, merge, or release state.")
		return nil
	}
	fs := flag.NewFlagSet("audit docs-backlog", flag.ContinueOnError)
	var docs multiFlag
	fs.Var(&docs, "doc", "coordination doc path to scan; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected audit docs-backlog arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, root string, s *store.Store) error {
		report, err := audit.BuildDocsBacklogReport(ctx, root, s, audit.DocsBacklogOptions{DocPaths: docs})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("docs_backlog_ok: %t\n", report.OK)
		fmt.Printf("summary: docs_scanned=%d docs_with_backlog_coverage=%d doc_only_capabilities=%d command_examples_uncovered=%d stale_completed_tasks=%d gpuaas_lessons=%d\n",
			report.Summary.DocsScanned,
			report.Summary.DocsWithBacklogCoverage,
			report.Summary.DocOnlyCapabilities,
			report.Summary.CommandExamplesUncovered,
			report.Summary.StaleCompletedTasks,
			report.Summary.GPUaaSLessons)
		if len(report.Docs) > 0 {
			fmt.Println("docs:")
			for _, doc := range report.Docs {
				fmt.Printf("- path=%s mentioned_tasks=%s covering_tasks=%s topics=%s commands=%d\n",
					doc.Path,
					firstNonEmpty(strings.Join(doc.MentionedTasks, ","), "none"),
					firstNonEmpty(strings.Join(doc.CoveringTasks, ","), "none"),
					firstNonEmpty(strings.Join(doc.Topics, ","), "none"),
					len(doc.CommandExamples))
			}
		}
		if len(report.Findings) == 0 {
			fmt.Println("no docs-backlog coverage findings")
			return nil
		}
		for _, finding := range report.Findings {
			fmt.Printf("%s\t%s\tdoc=%s\ttask=%s\ttopic=%s\treason=%s\n",
				finding.Severity,
				finding.Kind,
				finding.DocPath,
				finding.TaskID,
				finding.Topic,
				finding.Reason)
			if finding.Command != "" {
				fmt.Printf("  command: %s\n", finding.Command)
			}
			if len(finding.Related) > 0 {
				fmt.Printf("  related_tasks: %s\n", strings.Join(finding.Related, ", "))
			}
			if finding.Recommended != "" {
				fmt.Printf("  next: %s\n", finding.Recommended)
			}
		}
		return nil
	})
}

type notificationAuditRow struct {
	Source             string `json:"source"`
	TaskID             string `json:"task_id"`
	TaskStatus         string `json:"task_status,omitempty"`
	Domain             string `json:"domain,omitempty"`
	Provider           string `json:"provider,omitempty"`
	Target             string `json:"target,omitempty"`
	State              string `json:"state"`
	HandoffID          int64  `json:"handoff_id,omitempty"`
	LastNotificationID int64  `json:"last_notification_id,omitempty"`
	LastNotifiedAt     string `json:"last_notified_at,omitempty"`
	StaleAge           string `json:"stale_age,omitempty"`
	Action             string `json:"action"`
	SuggestedCommand   string `json:"suggested_command,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Terminal           bool   `json:"terminal,omitempty"`
	Superseded         bool   `json:"superseded,omitempty"`
}

func cmdAuditNotifications(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway audit notifications [--task <task-id>] [--all]")
		fmt.Println("  Report provider notification lifecycle rows from existing handoffs, notifications, waits, and coordinator projections.")
		return nil
	}
	fs := flag.NewFlagSet("audit notifications", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	all := fs.Bool("all", false, "include resolved, terminal, superseded, and non-actionable acknowledged rows")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected audit notifications arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		rows, err := buildNotificationAuditRows(ctx, cfg, root, s, *taskID, *all)
		if err != nil {
			return err
		}
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].TaskID != rows[j].TaskID {
				return rows[i].TaskID < rows[j].TaskID
			}
			if rows[i].Source != rows[j].Source {
				return rows[i].Source < rows[j].Source
			}
			return rows[i].Domain < rows[j].Domain
		})
		if opts.JSON {
			return printJSON(rows)
		}
		if len(rows) == 0 {
			fmt.Println("notification_audit: none")
			return nil
		}
		fmt.Println("notification_audit:")
		for _, row := range rows {
			fmt.Printf("- task=%s source=%s domain=%s state=%s action=%s provider=%s target=%s handoff_id=%d notification_id=%d stale_age=%s terminal=%t superseded=%t reason=%s\n",
				row.TaskID,
				row.Source,
				firstNonEmpty(row.Domain, "none"),
				row.State,
				row.Action,
				firstNonEmpty(row.Provider, "none"),
				firstNonEmpty(row.Target, "none"),
				row.HandoffID,
				row.LastNotificationID,
				firstNonEmpty(row.StaleAge, "none"),
				row.Terminal,
				row.Superseded,
				row.Reason)
			if row.SuggestedCommand != "" {
				fmt.Printf("  suggested_command: %s\n", row.SuggestedCommand)
			}
		}
		return nil
	})
}

func buildNotificationAuditRows(ctx context.Context, cfg config.Config, root string, s *store.Store, taskFilter string, includeAll bool) ([]notificationAuditRow, error) {
	now := time.Now().UTC()
	ackTimeout, err := reviewWaitAckTimeout(cfg)
	if err != nil {
		return nil, err
	}
	terminal := terminalStatusSet(cfg.States.Terminal)
	tasks, err := tasksForAudit(ctx, s, taskFilter)
	if err != nil {
		return nil, err
	}
	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return nil, err
	}
	liveWindowByTask := map[string]livewindow.Status{}
	for _, status := range livewindow.StatusesFromCheckpoints(checkpoints) {
		liveWindowByTask[status.TaskID] = status
	}

	var rows []notificationAuditRow
	waitOpts := reviewWaitOptions(cfg)
	waitOpts.AckTimeout = ackTimeout
	waitOpts.Now = now
	waitOpts.Terminal = cfg.States.Terminal
	statusesByTask := map[string]string{}
	for _, task := range tasks {
		taskID := task.Definition.ID
		statusesByTask[taskID] = task.Status
		_, _, evidence, handoffs, reviews, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return nil, err
		}
		notifications, err := s.Notifications(ctx, taskID)
		if err != nil {
			return nil, err
		}
		reviewStatuses := reviewstate.StatusesForTask(task, handoffs, reviews, notifications)
		reviewStatusByDomain := map[string]reviewstate.ReviewNotificationStatus{}
		for _, status := range reviewStatuses {
			reviewStatusByDomain[status.Domain] = status
		}
		for _, wait := range reviewstate.WaitsForTask(task, handoffs, reviews, notifications, waitOpts) {
			status := reviewStatusByDomain[wait.Domain]
			latest, _ := latestNotificationForAudit(notifications, wait.Domain, 0)
			state := notificationAuditState(firstNonEmpty(status.LastState, status.Status), wait.State)
			row := notificationAuditRow{
				Source:             "review_wait",
				TaskID:             taskID,
				TaskStatus:         task.Status,
				Domain:             wait.Domain,
				Provider:           firstNonEmpty(status.Provider, wait.TargetProvider),
				Target:             firstNonEmpty(status.Target, wait.WakeThreadID, wait.TargetID),
				State:              state,
				HandoffID:          status.HandoffID,
				LastNotificationID: latest.ID,
				LastNotifiedAt:     firstNonEmpty(status.LastNotificationAt, wait.LastNotifiedAt),
				StaleAge:           notificationAuditStaleAge(state, firstNonEmpty(status.LastNotificationAt, status.LastHandoffAt, task.UpdatedAt), now),
				Action:             notificationAuditAction("review_wait", state, wait.Action),
				SuggestedCommand:   notificationAuditSuggestion("review_wait", taskID, wait.Domain, status.HandoffID, wait.Action),
				Reason:             firstNonEmpty(wait.Reason, status.Reason),
				Terminal:           terminal[task.Status],
				Superseded:         !wait.Blocking || state == "review_recorded",
			}
			if includeNotificationAuditRow(row, includeAll) {
				rows = append(rows, row)
			}
		}
		livePhase := liveWindowByTask[taskID].Phase
		for _, handback := range completionhandback.RowsWithOptions(taskID, handoffs, notifications, completionhandback.RowOptions{
			Now:             now,
			AckTimeout:      ackTimeout,
			TaskStatus:      task.Status,
			LiveWindowPhase: livePhase,
			Superseded:      completionhandback.SupersedesFromEvidence(evidence),
		}) {
			latest, _ := latestNotificationForAudit(notifications, handback.ToRole, handback.HandoffID)
			state := notificationAuditState(handback.DeliveryState, handback.DeliveryStatus)
			row := notificationAuditRow{
				Source:             "completion_handback",
				TaskID:             taskID,
				TaskStatus:         task.Status,
				Domain:             handback.ToRole,
				Provider:           handback.Provider,
				Target:             handback.Target,
				State:              state,
				HandoffID:          handback.HandoffID,
				LastNotificationID: latest.ID,
				LastNotifiedAt:     firstNonEmpty(handback.DeliveredAt, handback.CreatedAt),
				StaleAge:           firstNonEmpty(handback.StaleAge, notificationAuditStaleAge(state, firstNonEmpty(handback.DeliveredAt, handback.CreatedAt), now)),
				Action:             notificationAuditAction("completion_handback", state, handback.SuggestedAction),
				SuggestedCommand:   firstNonEmpty(handback.SuggestedCommand, notificationAuditSuggestion("completion_handback", taskID, handback.ToRole, handback.HandoffID, handback.SuggestedAction)),
				Reason:             firstNonEmpty(handback.Reason, handback.NextAction, handback.DeliveryStatus),
				Terminal:           terminal[task.Status],
				Superseded:         completionHandbackAuditSuperseded(handback),
			}
			if includeNotificationAuditRow(row, includeAll) {
				rows = append(rows, row)
			}
		}
	}

	genericRows, err := projectedWaitRows(ctx, cfg, root, s, 24*time.Hour)
	if err != nil {
		return nil, err
	}
	allNotifications, err := s.Notifications(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, wait := range genericRows {
		if wait.Source == "review_waits" || wait.Source == "completion_handbacks" {
			continue
		}
		if strings.TrimSpace(taskFilter) != "" && wait.TaskID != taskFilter {
			continue
		}
		provider, target := completionWakeTarget(cfg.ProviderTargets, wait.Owner)
		latest, _ := latestNotificationForAudit(filterNotificationsByTask(allNotifications, wait.TaskID), wait.Owner, 0)
		taskStatus := statusesByTask[wait.TaskID]
		state := notificationAuditState("", wait.State)
		row := notificationAuditRow{
			Source:             wait.Source,
			TaskID:             wait.TaskID,
			TaskStatus:         taskStatus,
			Domain:             wait.Owner,
			Provider:           provider,
			Target:             target,
			State:              state,
			LastNotificationID: latest.ID,
			LastNotifiedAt:     latest.CreatedAt,
			StaleAge:           notificationAuditStaleAge(state, latest.CreatedAt, now),
			Action:             notificationAuditAction(wait.Source, state, wait.Action),
			SuggestedCommand:   firstNonEmpty(wait.SuggestedCommand, notificationAuditSuggestion(wait.Source, wait.TaskID, wait.Owner, 0, wait.Action)),
			Reason:             wait.Reason,
			Terminal:           terminal[taskStatus],
			Superseded:         false,
		}
		if includeNotificationAuditRow(row, includeAll) {
			rows = append(rows, row)
		}
	}
	return dedupeNotificationAuditRows(rows), nil
}

func tasksForAudit(ctx context.Context, s *store.Store, taskFilter string) ([]store.Task, error) {
	if strings.TrimSpace(taskFilter) != "" {
		task, _, _, _, _, err := s.TaskDetail(ctx, strings.TrimSpace(taskFilter))
		if err != nil {
			return nil, err
		}
		return []store.Task{task}, nil
	}
	return s.AllTasks(ctx)
}

func notificationAuditState(primary, fallback string) string {
	switch strings.TrimSpace(fallback) {
	case "stale":
		return "stale"
	case "failed", "notification_failed":
		if strings.TrimSpace(primary) == "" {
			return strings.TrimSpace(fallback)
		}
	}
	state := strings.TrimSpace(primary)
	if state == "" {
		state = strings.TrimSpace(fallback)
	}
	switch state {
	case "sent_awaiting_ack":
		return "sent"
	case "delivered":
		return "notification_delivered"
	case "pending", "open", "":
		return "handoff_recorded"
	default:
		return state
	}
}

func notificationAuditAction(source, state, fallback string) string {
	switch state {
	case "missing_notification", "handoff_recorded", "intent":
		return firstNonEmpty(fallback, "deliver_notification")
	case "sent":
		return "record_delivery_proof_or_failure"
	case "stale":
		return "escalate_or_record_notification_outcome"
	case "failed", "notification_failed":
		return "repair_mapping_or_record_alternate_delivery"
	case "acknowledged", "review_acknowledged", "review_recorded", "notification_delivered", "thread_steered":
		return firstNonEmpty(fallback, "wait_for_next_owner_or_terminal_closeout")
	default:
		return firstNonEmpty(fallback, "inspect_notification_lifecycle")
	}
}

func notificationAuditSuggestion(source, taskID, domain string, handoffID int64, action string) string {
	switch strings.TrimSpace(source) {
	case "review_wait":
		switch strings.TrimSpace(action) {
		case "mapping_required":
			return "update review/provider routing for domain " + domain
		case "resolve", "inspect_review_verdict":
			return fmt.Sprintf("fairway record review %s --domain %s --verdict approve|changes --reviewer <reviewer>", taskID, domain)
		default:
			return fmt.Sprintf("fairway record notification %s --domain %s --state thread_steered --provider <provider> --target <target>", taskID, domain)
		}
	case "completion_handback":
		if handoffID > 0 {
			return fmt.Sprintf("fairway record notification %s --handoff-id %d --domain %s --state thread_steered|notification_failed --provider <provider> --target <target> --reason <reason>", taskID, handoffID, domain)
		}
		return fmt.Sprintf("fairway record completion-handback %s --to %s --next-action <next-action> --state thread_steered --provider <provider> --target <target>", taskID, domain)
	default:
		if strings.TrimSpace(taskID) == "" || strings.TrimSpace(domain) == "" {
			return ""
		}
		return fmt.Sprintf("fairway record notification %s --domain %s --state thread_steered|notification_failed --provider <provider> --target <target> --reason <reason>", taskID, domain)
	}
}

func notificationAuditStaleAge(state, origin string, now time.Time) string {
	if strings.TrimSpace(state) != "stale" || strings.TrimSpace(origin) == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(origin))
	if err != nil {
		return ""
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if now.Before(parsed) {
		return ""
	}
	return now.Sub(parsed).Truncate(time.Second).String()
}

func completionHandbackAuditSuperseded(handback completionhandback.Handback) bool {
	if handback.Superseded {
		return true
	}
	switch strings.TrimSpace(handback.DeliveryStatus) {
	case "delivered":
		return true
	default:
		return false
	}
}

func includeNotificationAuditRow(row notificationAuditRow, includeAll bool) bool {
	if includeAll {
		return true
	}
	if row.Terminal || row.Superseded {
		return false
	}
	if row.Source == "completion_handback" {
		return true
	}
	switch row.State {
	case "notification_delivered", "thread_steered", "acknowledged", "review_acknowledged", "review_recorded", "resolved", "cancelled":
		return false
	default:
		return true
	}
}

func latestNotificationForAudit(notifications []store.Notification, domain string, handoffID int64) (store.Notification, bool) {
	var latest store.Notification
	for _, notification := range notifications {
		if handoffID > 0 {
			if notification.HandoffID == nil || *notification.HandoffID != handoffID {
				continue
			}
		} else if strings.TrimSpace(domain) != "" && strings.TrimSpace(notification.Domain) != strings.TrimSpace(domain) {
			continue
		}
		if latest.ID == 0 || notification.CreatedAt >= latest.CreatedAt {
			latest = notification
		}
	}
	return latest, latest.ID != 0
}

func filterNotificationsByTask(notifications []store.Notification, taskID string) []store.Notification {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	var out []store.Notification
	for _, notification := range notifications {
		if notification.TaskID == taskID {
			out = append(out, notification)
		}
	}
	return out
}

func dedupeNotificationAuditRows(rows []notificationAuditRow) []notificationAuditRow {
	seen := map[string]bool{}
	var out []notificationAuditRow
	for _, row := range rows {
		key := strings.Join([]string{
			row.Source,
			row.TaskID,
			row.Domain,
			strconv.FormatInt(row.HandoffID, 10),
			strconv.FormatInt(row.LastNotificationID, 10),
			row.State,
		}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, row)
	}
	return out
}

type advisoryRecommendation struct {
	Provider      string   `json:"provider,omitempty"`
	Action        string   `json:"action"`
	TaskID        string   `json:"task_id"`
	TargetRole    string   `json:"target_role"`
	Confidence    float64  `json:"confidence"`
	RequiresHuman bool     `json:"requires_human"`
	Rationale     string   `json:"rationale"`
	RiskFlags     []string `json:"risk_flags,omitempty"`
	CitedFacts    []string `json:"cited_fairway_facts"`
}

type advisoryValidationReport struct {
	OK             bool                   `json:"ok"`
	Recommendation advisoryRecommendation `json:"recommendation"`
	Issues         []string               `json:"issues,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
	Recorded       bool                   `json:"recorded"`
}

func cmdAdvisory(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("advisory", "adapters|validate <task-id>")
		return nil
	}
	switch args[0] {
	case "adapters":
		return cmdAdvisoryAdapters(ctx, opts, args[1:])
	case "validate":
		return cmdAdvisoryValidate(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown advisory subcommand %q", args[0])
	}
}

func cmdAdvisoryAdapters(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway advisory adapters [--include-disabled]")
		fmt.Println("  List configured advisory provider adapters; read-only and advisory-only.")
		return nil
	}
	fs := flag.NewFlagSet("advisory adapters", flag.ContinueOnError)
	includeDisabled := fs.Bool("include-disabled", false, "include disabled adapters")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected advisory adapters arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, _ *store.Store) error {
		adapters := advisoryAdaptersForOutput(cfg.AdvisoryAdapters, *includeDisabled)
		if opts.JSON {
			return printJSON(adapters)
		}
		fmt.Println("advisory_adapters:")
		if len(adapters) == 0 {
			fmt.Println("- none")
			return nil
		}
		for _, adapter := range adapters {
			fmt.Printf("- %s provider=%s type=%s mode=%s trust=%s\n",
				adapter.Name,
				adapter.Provider,
				firstNonEmpty(adapter.Type, "noop"),
				firstNonEmpty(adapter.Mode, "advisory"),
				firstNonEmpty(adapter.Trust, "low"))
			if adapter.Model != "" {
				fmt.Printf("  model=%s\n", adapter.Model)
			}
			if adapter.EndpointEnv != "" {
				fmt.Printf("  endpoint_env=%s\n", adapter.EndpointEnv)
			}
			printPacketList("  capabilities", adapter.Capabilities)
			printPacketList("  allowed_actions", adapter.AllowedActions)
		}
		return nil
	})
}

func advisoryAdaptersForOutput(adapters []config.AdvisoryAdapter, includeDisabled bool) []config.AdvisoryAdapter {
	out := make([]config.AdvisoryAdapter, 0, len(adapters))
	for _, adapter := range adapters {
		adapter.Name = strings.TrimSpace(adapter.Name)
		adapter.Provider = strings.TrimSpace(adapter.Provider)
		adapter.Type = firstNonEmpty(strings.TrimSpace(adapter.Type), "noop")
		adapter.Mode = firstNonEmpty(strings.TrimSpace(adapter.Mode), "advisory")
		adapter.Trust = firstNonEmpty(strings.TrimSpace(adapter.Trust), "low")
		adapter.Model = strings.TrimSpace(adapter.Model)
		adapter.EndpointEnv = strings.TrimSpace(adapter.EndpointEnv)
		adapter.Capabilities = trimStrings(adapter.Capabilities)
		adapter.AllowedActions = trimStrings(adapter.AllowedActions)
		if !includeDisabled && adapter.Mode == "disabled" {
			continue
		}
		out = append(out, adapter)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func cmdAdvisoryValidate(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway advisory validate <task-id> --action <action> --target-role <role> --confidence <0..1> --rationale <text> --cited-fact <fact>... [--provider <adapter>] [--requires-human] [--risk-flag <flag>]... [--record-evidence]")
		fmt.Println("  Validate an advisory recommendation; optional recording writes advisory evidence only.")
		return nil
	}
	if len(args) < 1 {
		return errors.New("advisory validate requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("advisory validate", flag.ContinueOnError)
	provider := fs.String("provider", "", "configured advisory provider adapter name")
	action := fs.String("action", "", "recommended advisory action")
	targetRole := fs.String("target-role", "", "target role for the action")
	confidence := fs.Float64("confidence", -1, "confidence from 0.0 to 1.0")
	requiresHuman := fs.Bool("requires-human", false, "recommendation requires human decision")
	rationale := fs.String("rationale", "", "recommendation rationale")
	recordEvidence := fs.Bool("record-evidence", false, "record accepted recommendation as advisory evidence")
	var riskFlags multiFlag
	var citedFacts multiFlag
	fs.Var(&riskFlags, "risk-flag", "risk flag; may repeat")
	fs.Var(&citedFacts, "cited-fact", "cited Fairway fact; may repeat")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected advisory validate arguments: %s", strings.Join(fs.Args(), " "))
	}
	rec := advisoryRecommendation{
		Provider:      strings.TrimSpace(*provider),
		Action:        strings.TrimSpace(*action),
		TaskID:        taskID,
		TargetRole:    strings.TrimSpace(*targetRole),
		Confidence:    *confidence,
		RequiresHuman: *requiresHuman,
		Rationale:     strings.TrimSpace(*rationale),
		RiskFlags:     trimStrings(riskFlags),
		CitedFacts:    trimStrings(citedFacts),
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		task, _, _, _, reviews, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		report := validateAdvisoryRecommendation(cfg, task, reviews, rec)
		if report.OK && *recordEvidence {
			notes, err := json.Marshal(report.Recommendation)
			if err != nil {
				return err
			}
			ev := store.Evidence{
				CommandText:  "fairway advisory validate " + taskID + " --action " + shellToken(rec.Action),
				Result:       "pass",
				ArtifactType: "advisory-recommendation",
				Notes:        string(notes),
			}
			if err := s.RecordEvidence(ctx, taskID, ev); err != nil {
				return err
			}
			report.Recorded = true
		}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("advisory_valid: %t\n", report.OK)
		fmt.Printf("task: %s\nprovider: %s\naction: %s\ntarget_role: %s\nconfidence: %.2f\nrequires_human: %t\n", rec.TaskID, firstNonEmpty(rec.Provider, "none"), rec.Action, rec.TargetRole, rec.Confidence, rec.RequiresHuman)
		printPacketList("Risk Flags", rec.RiskFlags)
		printPacketList("Cited Fairway Facts", rec.CitedFacts)
		if rec.Rationale != "" {
			fmt.Printf("\n## Rationale\n%s\n", rec.Rationale)
		}
		printPacketList("Issues", report.Issues)
		printPacketList("Warnings", report.Warnings)
		if report.Recorded {
			fmt.Println("recorded: advisory-recommendation evidence")
		}
		if !report.OK {
			return errors.New("advisory recommendation validation failed")
		}
		return nil
	})
}

func validateAdvisoryRecommendation(cfg config.Config, task store.Task, reviews []store.Review, rec advisoryRecommendation) advisoryValidationReport {
	report := advisoryValidationReport{OK: true, Recommendation: rec}
	addIssue := func(issue string) {
		report.OK = false
		report.Issues = append(report.Issues, issue)
	}
	if rec.Action == "" {
		addIssue("--action is required")
	} else if !allowedAdvisoryAction(rec.Action) {
		addIssue("action is not in the advisory allowed-action enum")
	}
	if rec.Provider != "" {
		adapter, ok := configuredAdvisoryAdapter(cfg, rec.Provider)
		if !ok {
			addIssue("configured advisory provider adapter not found: " + rec.Provider)
		} else {
			mode := firstNonEmpty(strings.TrimSpace(adapter.Mode), "advisory")
			if mode == "disabled" {
				addIssue("configured advisory provider adapter is disabled: " + rec.Provider)
			}
			if len(adapter.AllowedActions) > 0 && !containsString(trimStrings(adapter.AllowedActions), rec.Action) {
				addIssue("configured advisory provider adapter does not allow action: " + rec.Action)
			}
			if strings.TrimSpace(adapter.Trust) == "low" && len(rec.RiskFlags) > 0 {
				report.Warnings = append(report.Warnings, "low-trust advisory provider output with risk flags requires human review")
			}
		}
	}
	if rec.TargetRole == "" {
		addIssue("--target-role is required")
	} else if !configuredRole(cfg, rec.TargetRole) {
		addIssue("target role is not configured")
	}
	if rec.Confidence < 0 || rec.Confidence > 1 {
		addIssue("--confidence must be between 0 and 1")
	}
	if rec.Rationale == "" {
		addIssue("--rationale is required")
	}
	if len(rec.CitedFacts) == 0 {
		addIssue("--cited-fact is required")
	}
	for _, fact := range rec.CitedFacts {
		if !validAdvisoryFact(fact, task.Definition.ID) {
			addIssue("cited fact must name an existing Fairway fact prefix and matching task id: " + fact)
		}
	}
	if len(rec.RiskFlags) > 0 && !rec.RequiresHuman {
		addIssue("risk flags require --requires-human")
	}
	if providerTargetAction(rec.Action) && !providerTargetConfigured(cfg.ProviderTargets, rec.TargetRole) {
		report.Warnings = append(report.Warnings, "target role has no configured provider target; recommendation is not routable as a wake")
	}
	if actionBlockedByTaskState(cfg, task, rec.Action) {
		addIssue("action is not applicable to task status " + task.Status)
	}
	if rec.Action == "route_review" && len(task.Definition.ReviewDomains) > 0 && len(missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)) == 0 {
		addIssue("route_review is not applicable because required review domains are already approved")
	}
	return report
}

func allowedAdvisoryAction(action string) bool {
	return config.AllowedAdvisoryAction(action)
}

func providerTargetAction(action string) bool {
	return action == "wake_provider"
}

func actionBlockedByTaskState(cfg config.Config, task store.Task, action string) bool {
	if !isTerminal(task.Status, cfg.States.Terminal) {
		return false
	}
	switch action {
	case "inspect_task", "render_packet", "create_follow_up":
		return false
	default:
		return true
	}
}

func configuredRole(cfg config.Config, role string) bool {
	if len(cfg.Roles) == 0 {
		return true
	}
	for _, configured := range cfg.Roles {
		if configured.Name == role {
			return true
		}
	}
	return false
}

func providerTargetConfigured(targets []config.ProviderTarget, role string) bool {
	for _, target := range targets {
		if target.Domain == role && strings.TrimSpace(target.Provider) != "" && strings.TrimSpace(target.Target) != "" {
			return true
		}
	}
	return false
}

func configuredAdvisoryAdapter(cfg config.Config, name string) (config.AdvisoryAdapter, bool) {
	name = strings.TrimSpace(name)
	for _, adapter := range cfg.AdvisoryAdapters {
		if strings.TrimSpace(adapter.Name) == name {
			return adapter, true
		}
	}
	return config.AdvisoryAdapter{}, false
}

type notifyDryRunReport struct {
	OK             bool   `json:"ok"`
	Notifier       string `json:"notifier"`
	Type           string `json:"type"`
	Mode           string `json:"mode"`
	TaskID         string `json:"task_id"`
	Domain         string `json:"domain"`
	Target         string `json:"target,omitempty"`
	Template       string `json:"template"`
	RecordIntent   bool   `json:"record_intent"`
	RecordedState  string `json:"recorded_state,omitempty"`
	RecordedReason string `json:"recorded_reason,omitempty"`
	Warning        string `json:"warning,omitempty"`
}

type notifySendReport struct {
	OK             bool   `json:"ok"`
	Notifier       string `json:"notifier"`
	Type           string `json:"type"`
	Mode           string `json:"mode"`
	TaskID         string `json:"task_id"`
	Domain         string `json:"domain"`
	Target         string `json:"target,omitempty"`
	Template       string `json:"template"`
	AttemptState   string `json:"attempt_state,omitempty"`
	DeliveryState  string `json:"delivery_state,omitempty"`
	RecordedReason string `json:"recorded_reason,omitempty"`
	Error          string `json:"error,omitempty"`
}

func cmdNotify(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("notify", "notifiers|dry-run|send")
		return nil
	}
	switch args[0] {
	case "notifiers":
		return cmdNotifyNotifiers(ctx, opts, args[1:])
	case "dry-run":
		return cmdNotifyDryRun(ctx, opts, args[1:])
	case "send":
		return cmdNotifySend(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown notify subcommand %q", args[0])
	}
}

func cmdNotifyNotifiers(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway notify notifiers [--include-disabled]")
		fmt.Println("  List configured optional external notifiers; read-only.")
		return nil
	}
	fs := flag.NewFlagSet("notify notifiers", flag.ContinueOnError)
	includeDisabled := fs.Bool("include-disabled", false, "include disabled notifiers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected notify notifiers arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, _ *store.Store) error {
		notifiers := externalNotifiersForOutput(cfg.ExternalNotifiers, *includeDisabled)
		if opts.JSON {
			return printJSON(notifiers)
		}
		fmt.Println("external_notifiers:")
		if len(notifiers) == 0 {
			fmt.Println("- none")
			return nil
		}
		for _, notifier := range notifiers {
			fmt.Printf("- %s type=%s mode=%s\n", notifier.Name, notifier.Type, notifier.Mode)
			if notifier.TargetEnv != "" {
				fmt.Printf("  target_env=%s\n", notifier.TargetEnv)
			}
			if notifier.TokenEnv != "" {
				fmt.Printf("  token_env=%s\n", notifier.TokenEnv)
			}
			if notifier.TemplateName != "" {
				fmt.Printf("  template=%s\n", notifier.TemplateName)
			}
			if notifier.RateLimitPerMinute > 0 {
				fmt.Printf("  rate_limit_per_minute=%d\n", notifier.RateLimitPerMinute)
			}
			printPacketList("  domains", notifier.Domains)
		}
		return nil
	})
}

func cmdNotifyDryRun(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway notify dry-run --notifier <name> --task <task-id> --domain <domain> [--template <name>] [--target <target>] [--record-intent]")
		fmt.Println("  Render a bounded external notification request; optional recording writes notification intent only.")
		return nil
	}
	fs := flag.NewFlagSet("notify dry-run", flag.ContinueOnError)
	notifierName := fs.String("notifier", "", "configured notifier name")
	taskID := fs.String("task", "", "task id")
	domain := fs.String("domain", "", "notification domain or target role")
	target := fs.String("target", "", "optional target label")
	templateName := fs.String("template", "", "fixed template name")
	recordIntent := fs.Bool("record-intent", false, "record notification intent only")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected notify dry-run arguments: %s", strings.Join(fs.Args(), " "))
	}
	*notifierName = strings.TrimSpace(*notifierName)
	*taskID = strings.TrimSpace(*taskID)
	*domain = strings.TrimSpace(*domain)
	*templateName = strings.TrimSpace(*templateName)
	if *notifierName == "" {
		return errors.New("--notifier is required")
	}
	if *taskID == "" {
		return errors.New("--task is required")
	}
	if *domain == "" {
		return errors.New("--domain is required")
	}
	if strings.TrimSpace(*target) != "" && !validExternalNotifierTargetLabel(strings.TrimSpace(*target)) {
		return errors.New("--target must be a safe label containing only letters, digits, dot, dash, or underscore")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		notifier, ok := configuredExternalNotifier(cfg, *notifierName)
		if !ok {
			return fmt.Errorf("external notifier %q is not configured", *notifierName)
		}
		notifier = normalizeExternalNotifier(notifier)
		if notifier.Mode == "disabled" {
			return fmt.Errorf("external notifier %q is disabled", notifier.Name)
		}
		if len(notifier.Domains) > 0 && !containsString(notifier.Domains, *domain) {
			return fmt.Errorf("external notifier %q does not allow domain %q", notifier.Name, *domain)
		}
		template := firstNonEmpty(*templateName, notifier.TemplateName, "external_notification")
		report := notifyDryRunReport{
			OK:           true,
			Notifier:     notifier.Name,
			Type:         notifier.Type,
			Mode:         notifier.Mode,
			TaskID:       *taskID,
			Domain:       *domain,
			Target:       firstNonEmpty(strings.TrimSpace(*target), notifier.TargetEnv),
			Template:     template,
			RecordIntent: *recordIntent,
			Warning:      "dry-run/log notifier does not prove external delivery",
		}
		if *recordIntent {
			reason := fmt.Sprintf("external_notifier_intent notifier=%s mode=%s template=%s", notifier.Name, notifier.Mode, shellToken(template))
			recorded, err := s.RecordNotification(ctx, store.Notification{
				TaskID:   *taskID,
				Domain:   *domain,
				Provider: "external-notifier/" + notifier.Name,
				Target:   report.Target,
				State:    "intent",
				Reason:   reason,
			})
			if err != nil {
				return err
			}
			report.RecordedState = recorded.State
			report.RecordedReason = recorded.Reason
		}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("notify_dry_run: true\nnotifier: %s\ntype: %s\nmode: %s\ntask: %s\ndomain: %s\ntarget: %s\ntemplate: %s\nwarning: %s\n",
			report.Notifier, report.Type, report.Mode, report.TaskID, report.Domain, firstNonEmpty(report.Target, "none"), report.Template, report.Warning)
		if report.RecordedState != "" {
			fmt.Printf("recorded_state: %s\nrecorded_reason: %s\n", report.RecordedState, report.RecordedReason)
		}
		return nil
	})
}

func cmdNotifySend(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway notify send --notifier <name> --task <task-id> --domain <domain> [--template <name>] [--target <label>]")
		fmt.Println("  Deliver through an explicitly configured external notifier and record attempted/delivered/failed notification evidence.")
		return nil
	}
	fs := flag.NewFlagSet("notify send", flag.ContinueOnError)
	notifierName := fs.String("notifier", "", "configured notifier name")
	taskID := fs.String("task", "", "task id")
	domain := fs.String("domain", "", "notification domain or target role")
	target := fs.String("target", "", "optional safe target label for recorded evidence")
	templateName := fs.String("template", "", "fixed template name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected notify send arguments: %s", strings.Join(fs.Args(), " "))
	}
	*notifierName = strings.TrimSpace(*notifierName)
	*taskID = strings.TrimSpace(*taskID)
	*domain = strings.TrimSpace(*domain)
	*templateName = strings.TrimSpace(*templateName)
	if *notifierName == "" {
		return errors.New("--notifier is required")
	}
	if *taskID == "" {
		return errors.New("--task is required")
	}
	if *domain == "" {
		return errors.New("--domain is required")
	}
	if strings.TrimSpace(*target) != "" && !validExternalNotifierTargetLabel(strings.TrimSpace(*target)) {
		return errors.New("--target must be a safe label containing only letters, digits, dot, dash, or underscore")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		notifier, ok := configuredExternalNotifier(cfg, *notifierName)
		if !ok {
			return fmt.Errorf("external notifier %q is not configured", *notifierName)
		}
		notifier = normalizeExternalNotifier(notifier)
		if notifier.Mode == "disabled" {
			return fmt.Errorf("external notifier %q is disabled", notifier.Name)
		}
		if notifier.Mode != "send" {
			return fmt.Errorf("external notifier %q is mode %q; send requires mode=send", notifier.Name, notifier.Mode)
		}
		if notifier.Type == "noop" {
			return fmt.Errorf("external notifier %q type noop cannot deliver", notifier.Name)
		}
		if len(notifier.Domains) > 0 && !containsString(notifier.Domains, *domain) {
			return fmt.Errorf("external notifier %q does not allow domain %q", notifier.Name, *domain)
		}
		template := firstNonEmpty(*templateName, notifier.TemplateName, "external_notification")
		targetLabel := firstNonEmpty(strings.TrimSpace(*target), notifier.TargetEnv)
		report := notifySendReport{
			OK:       true,
			Notifier: notifier.Name,
			Type:     notifier.Type,
			Mode:     notifier.Mode,
			TaskID:   *taskID,
			Domain:   *domain,
			Target:   targetLabel,
			Template: template,
		}
		rateLimited, rateReason, err := externalNotifierRateLimited(ctx, s, notifier)
		if err != nil {
			return err
		}
		if rateLimited {
			recorded, err := s.RecordNotification(ctx, store.Notification{
				TaskID:   *taskID,
				Domain:   *domain,
				Provider: "external-notifier/" + notifier.Name,
				Target:   targetLabel,
				State:    "notification_failed",
				Reason:   rateReason,
			})
			if err != nil {
				return err
			}
			report.OK = false
			report.DeliveryState = recorded.State
			report.RecordedReason = recorded.Reason
			report.Error = "rate_limited"
			if opts.JSON {
				return printJSON(report)
			}
			fmt.Printf("notify_send: false\nnotifier: %s\ntype: %s\nmode: %s\ntask: %s\ndomain: %s\ntarget: %s\ntemplate: %s\ndelivery_state: %s\nreason: %s\n",
				report.Notifier, report.Type, report.Mode, report.TaskID, report.Domain, firstNonEmpty(report.Target, "none"), report.Template, report.DeliveryState, report.RecordedReason)
			return fmt.Errorf("external notifier %q rate limited", notifier.Name)
		}
		attemptReason := fmt.Sprintf("external_notifier_attempted notifier=%s type=%s template=%s", notifier.Name, notifier.Type, shellToken(template))
		attempted, err := s.RecordNotification(ctx, store.Notification{
			TaskID:   *taskID,
			Domain:   *domain,
			Provider: "external-notifier/" + notifier.Name,
			Target:   targetLabel,
			State:    "sent",
			Reason:   attemptReason,
		})
		if err != nil {
			return err
		}
		report.AttemptState = attempted.State
		deliveryErr := deliverExternalNotification(ctx, notifier, notifyPayload{
			TaskID:   *taskID,
			Domain:   *domain,
			Template: template,
		})
		state := "notification_delivered"
		reason := fmt.Sprintf("external_notifier_delivered notifier=%s type=%s template=%s", notifier.Name, notifier.Type, shellToken(template))
		if deliveryErr != nil {
			state = "notification_failed"
			reason = fmt.Sprintf("external_notifier_failed notifier=%s type=%s template=%s error=%s", notifier.Name, notifier.Type, shellToken(template), shellToken(redactNotifierError(deliveryErr.Error())))
			report.OK = false
			report.Error = redactNotifierError(deliveryErr.Error())
		}
		recorded, err := s.RecordNotification(ctx, store.Notification{
			TaskID:   *taskID,
			Domain:   *domain,
			Provider: "external-notifier/" + notifier.Name,
			Target:   targetLabel,
			State:    state,
			Reason:   reason,
		})
		if err != nil {
			return err
		}
		report.DeliveryState = recorded.State
		report.RecordedReason = recorded.Reason
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("notify_send: %t\nnotifier: %s\ntype: %s\nmode: %s\ntask: %s\ndomain: %s\ntarget: %s\ntemplate: %s\nattempt_state: %s\ndelivery_state: %s\nrecorded_reason: %s\n",
			report.OK, report.Notifier, report.Type, report.Mode, report.TaskID, report.Domain, firstNonEmpty(report.Target, "none"), report.Template, report.AttemptState, report.DeliveryState, report.RecordedReason)
		if report.Error != "" {
			fmt.Printf("error: %s\n", report.Error)
			return fmt.Errorf("external notifier %q delivery failed", notifier.Name)
		}
		return nil
	})
}

func externalNotifiersForOutput(notifiers []config.ExternalNotifier, includeDisabled bool) []config.ExternalNotifier {
	out := make([]config.ExternalNotifier, 0, len(notifiers))
	for _, notifier := range notifiers {
		notifier = normalizeExternalNotifier(notifier)
		if !includeDisabled && notifier.Mode == "disabled" {
			continue
		}
		out = append(out, notifier)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func configuredExternalNotifier(cfg config.Config, name string) (config.ExternalNotifier, bool) {
	name = strings.TrimSpace(name)
	for _, notifier := range cfg.ExternalNotifiers {
		if strings.TrimSpace(notifier.Name) == name {
			return notifier, true
		}
	}
	return config.ExternalNotifier{}, false
}

func normalizeExternalNotifier(notifier config.ExternalNotifier) config.ExternalNotifier {
	notifier.Name = strings.TrimSpace(notifier.Name)
	notifier.Type = firstNonEmpty(strings.TrimSpace(notifier.Type), "noop")
	notifier.Mode = firstNonEmpty(strings.TrimSpace(notifier.Mode), "dry_run")
	notifier.TargetEnv = strings.TrimSpace(notifier.TargetEnv)
	notifier.TokenEnv = strings.TrimSpace(notifier.TokenEnv)
	notifier.TemplateName = strings.TrimSpace(notifier.TemplateName)
	notifier.Domains = trimStrings(notifier.Domains)
	if notifier.Mode == "send" && notifier.RateLimitPerMinute == 0 {
		notifier.RateLimitPerMinute = 30
	}
	return notifier
}

type notifyPayload struct {
	TaskID   string
	Domain   string
	Template string
}

func externalNotifierRateLimited(ctx context.Context, s *store.Store, notifier config.ExternalNotifier) (bool, string, error) {
	limit := notifier.RateLimitPerMinute
	if limit <= 0 {
		return false, "", nil
	}
	notifications, err := s.Notifications(ctx, "")
	if err != nil {
		return false, "", err
	}
	provider := "external-notifier/" + notifier.Name
	cutoff := time.Now().UTC().Add(-1 * time.Minute)
	count := 0
	for _, notification := range notifications {
		if notification.Provider != provider {
			continue
		}
		created, err := time.Parse(time.RFC3339Nano, notification.CreatedAt)
		if err != nil || created.Before(cutoff) {
			continue
		}
		switch notification.State {
		case "sent":
			count++
		}
	}
	if count < limit {
		return false, "", nil
	}
	return true, fmt.Sprintf("external_notifier_rate_limited notifier=%s limit_per_minute=%d", notifier.Name, limit), nil
}

func deliverExternalNotification(ctx context.Context, notifier config.ExternalNotifier, payload notifyPayload) error {
	target, ok := os.LookupEnv(notifier.TargetEnv)
	if !ok || strings.TrimSpace(target) == "" {
		return fmt.Errorf("target env %s is unset", notifier.TargetEnv)
	}
	body := externalNotifierBody(notifier, payload)
	switch notifier.Type {
	case "log":
		return deliverLogNotification(strings.TrimSpace(target), body)
	case "webhook":
		return deliverWebhookNotification(ctx, strings.TrimSpace(target), notifier, body)
	default:
		return fmt.Errorf("unsupported notifier type %s", notifier.Type)
	}
}

func externalNotifierBody(notifier config.ExternalNotifier, payload notifyPayload) string {
	data := map[string]string{
		"notifier": notifier.Name,
		"type":     notifier.Type,
		"task_id":  payload.TaskID,
		"domain":   payload.Domain,
		"template": payload.Template,
		"sent_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func deliverLogNotification(path string, body string) error {
	if path == "" {
		return errors.New("log target path is empty")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(body + "\n")
	return err
}

func deliverWebhookNotification(ctx context.Context, endpoint string, notifier config.ExternalNotifier, body string) error {
	if endpoint == "" {
		return errors.New("webhook endpoint is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if notifier.TokenEnv != "" {
		if token := strings.TrimSpace(os.Getenv(notifier.TokenEnv)); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func redactNotifierError(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown"
	}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 || len(parts[1]) < 8 {
			continue
		}
		raw = strings.ReplaceAll(raw, parts[1], "<redacted>")
	}
	return raw
}

func validExternalNotifierTargetLabel(label string) bool {
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 128 {
		return false
	}
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func validAdvisoryFact(fact, taskID string) bool {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return false
	}
	prefixes := []string{"task:", "evidence:", "review:", "checkpoint:", "session:", "handoff:", "notification:"}
	hasPrefix := false
	for _, prefix := range prefixes {
		if strings.HasPrefix(fact, prefix) {
			hasPrefix = true
			break
		}
	}
	return hasPrefix && strings.Contains(fact, taskID)
}

func trimStrings(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func cmdAuditWorkCoverage(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("audit work-coverage", flag.ContinueOnError)
	sinceRef := fs.String("since-ref", "", "base ref for commit coverage")
	sinceDuration := fs.Duration("since-duration", 0, "duration window for commit coverage")
	taskID := fs.String("task-id", "", "limit task-level checks to one task")
	dryRun := fs.Bool("dry-run", false, "report only; no mutation is performed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected audit work-coverage arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *sinceRef != "" && *sinceDuration > 0 {
		return errors.New("use --since-ref or --since-duration, not both")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		report, err := audit.BuildWorkCoverageReport(ctx, cfg, root, s, audit.WorkCoverageOptions{SinceRef: *sinceRef, SinceDuration: *sinceDuration, TaskID: *taskID})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(report)
		}
		if *dryRun {
			fmt.Println("dry-run: advisory audit only; no Fairway state mutated")
		}
		fmt.Printf("work_coverage_ok: %t\n", report.OK)
		if report.SinceRef != "" {
			fmt.Printf("since_ref: %s\n", report.SinceRef)
		}
		if report.SinceDuration != "" {
			fmt.Printf("since_duration: %s\n", report.SinceDuration)
		}
		if report.TaskID != "" {
			fmt.Printf("task_id: %s\n", report.TaskID)
		}
		fmt.Printf("commits: %d\n", report.CommitCount)
		fmt.Printf("summary: commits_without_task_id=%d changed_files_uncovered=%d orphan_evidence=%d evidence_status_decisions=%d done_without_required_evidence=%d missing_required_reviews=%d work_batch_candidates=%d\n",
			report.Summary.CommitsWithoutTaskID,
			report.Summary.ChangedFilesUncovered,
			report.Summary.OrphanEvidence,
			report.Summary.EvidenceStatusDecisions,
			report.Summary.DoneWithoutRequiredEvidence,
			report.Summary.MissingRequiredReviews,
			report.Summary.WorkBatchCandidates)
		if len(report.Findings) == 0 {
			fmt.Println("no work coverage findings")
			return nil
		}
		for _, finding := range report.Findings {
			fmt.Printf("%s\t%s\ttask=%s\tcommit=%s\treason=%s\n", finding.Severity, finding.Kind, finding.TaskID, finding.Commit, finding.Reason)
			if len(finding.Files) > 0 {
				fmt.Printf("  files: %s\n", strings.Join(finding.Files, ", "))
			}
			if len(finding.Missing) > 0 {
				fmt.Printf("  missing: %s\n", strings.Join(finding.Missing, ", "))
			}
			if len(finding.RelatedTasks) > 0 {
				fmt.Printf("  related_tasks: %s\n", strings.Join(finding.RelatedTasks, ", "))
			}
			if finding.Recommended != "" {
				fmt.Printf("  next: %s\n", finding.Recommended)
			}
		}
		return nil
	})
}

func cmdAuditCILearning(ctx context.Context, opts globalOptions, command string, args []string) error {
	if isHelpOnly(args) {
		switch command {
		case "failure-routing":
			fmt.Println("fairway audit failure-routing [--task-id <task-id>] [--template]")
			fmt.Println("  Print advisory known-failure routing recommendations without creating tasks or changing state.")
		default:
			fmt.Println("fairway audit ci-learning [--task-id <task-id>] [--template]")
			fmt.Println("  Print advisory CI/deploy learning findings and optional follow-up templates.")
		}
		return nil
	}
	fs := flag.NewFlagSet("audit "+command, flag.ContinueOnError)
	taskID := fs.String("task-id", "", "limit audit to one task")
	template := fs.Bool("template", false, "render learning artifact templates")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected audit %s arguments: %s", command, strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		report, err := audit.BuildCILearningReport(ctx, cfg, s, audit.CILearningOptions{TaskID: *taskID, RenderTemplates: *template})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(report)
		}
		if command == "failure-routing" {
			fmt.Printf("failure_routing_ok: %t\n", report.OK)
		} else {
			fmt.Printf("ci_learning_ok: %t\n", report.OK)
		}
		if report.TaskID != "" {
			fmt.Printf("task_id: %s\n", report.TaskID)
		}
		fmt.Printf("summary: failed_evidence=%d missing_follow_ups=%d missed_local_gates=%d missed_review_gates=%d ci_environment_only=%d flaky_runner_or_cache=%d approval_gated_blocker=%d artifact_contract=%d provider_api=%d browser_surface=%d setup_gate=%d callback_missing=%d redaction_finding=%d commit_boundary=%d undelivered_handoff=%d\n",
			report.Summary.FailedEvidence,
			report.Summary.MissingFollowUps,
			report.Summary.MissedLocalGates,
			report.Summary.MissedReviewGates,
			report.Summary.CIEnvironmentOnly,
			report.Summary.FlakyRunnerOrCache,
			report.Summary.ApprovalGatedBlocker,
			report.Summary.ArtifactContract,
			report.Summary.ProviderAPI,
			report.Summary.BrowserSurface,
			report.Summary.SetupGate,
			report.Summary.CallbackMissing,
			report.Summary.RedactionFinding,
			report.Summary.CommitBoundary,
			report.Summary.UndeliveredHandoff)
		if len(report.Findings) == 0 {
			if command == "failure-routing" {
				fmt.Println("no known-failure routing findings")
			} else {
				fmt.Println("no CI/deploy learning findings")
			}
			return nil
		}
		for _, finding := range report.Findings {
			fmt.Printf("warning\t%s\ttask=%s\tfollow_up=%s\tcommand=%s\n", finding.FailureClass, finding.TaskID, finding.FollowUpTask, finding.CommandText)
			if finding.FollowUpMissing {
				fmt.Printf("  missing follow-up: create %s-* task, suggested %s kind=%s owner=%s layer=%s\n", finding.RecommendedFollowUpPrefix, finding.RecommendedFollowUpTaskID, finding.RecommendedFollowUpTaskKind, firstNonEmpty(finding.OwningDomain, "unknown"), firstNonEmpty(finding.OwningLayer, "unknown"))
			}
			if finding.ArtifactPath != "" {
				fmt.Printf("  artifact: %s\n", finding.ArtifactPath)
			}
			if finding.ExpectedLocalReproduction != "" {
				fmt.Printf("  reproduce: %s\n", finding.ExpectedLocalReproduction)
			}
			if finding.MissedGate != "" {
				fmt.Printf("  missed_gate: %s\n", finding.MissedGate)
			}
			if len(finding.ForbiddenActions) > 0 {
				fmt.Printf("  forbidden_until_reviewed: %s\n", strings.Join(finding.ForbiddenActions, ", "))
			}
			fmt.Printf("  root_cause: %s\n", finding.RootCause)
		}
		for _, artifact := range report.Templates {
			fmt.Println()
			fmt.Println(artifact.Markdown)
		}
		return nil
	})
}

func cmdDelivery(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		fmt.Println("fairway delivery report --since <duration> [--profile <name>] [--format text|json]")
		fmt.Println("  Read-only delivery velocity and process overhead report from existing Fairway state.")
		return nil
	}
	switch args[0] {
	case "report":
		return cmdDeliveryReport(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown delivery subcommand %q", args[0])
	}
}

func cmdDeliveryReport(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway delivery report --since <duration> [--profile <name>] [--format text|json]")
		fmt.Println("  Report completed tasks, blocked/review-wait time, process overhead, outcome sources, and loop signals without mutating workflow.")
		return nil
	}
	fs := flag.NewFlagSet("delivery report", flag.ContinueOnError)
	since := fs.Duration("since", 7*24*time.Hour, "time window")
	profile := fs.String("profile", "", "limit to task profile")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected delivery report arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *since <= 0 {
		return errors.New("--since must be positive")
	}
	if *format != "text" && *format != "json" {
		return errors.New("--format must be text or json")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		report, err := deliveryreport.Build(ctx, cfg, s, deliveryreport.Options{Since: *since, Profile: *profile})
		if err != nil {
			return err
		}
		if opts.JSON || *format == "json" {
			return printJSON(report)
		}
		fmt.Printf("delivery_report_ok: %t\n", report.OK)
		fmt.Printf("since: %s\n", report.Since)
		if report.Profile != "" {
			fmt.Printf("profile: %s\n", report.Profile)
		}
		fmt.Printf("summary: completed=%d blocked_opened=%d blocked_resolved=%d blocked_seconds=%d review_wait_seconds=%d first_evidence_to_done_seconds=%d done_to_merge_ready_seconds_observed=%d approval_loops=%d reopen_retry_count=%d\n",
			report.Summary.CompletedTasks,
			report.Summary.BlockedOpened,
			report.Summary.BlockedResolved,
			report.Summary.TotalBlockedSeconds,
			report.Summary.TotalReviewWaitSeconds,
			report.Summary.FirstEvidenceToDoneSeconds,
			report.Summary.DoneToMergeReadySecondsObserved,
			report.Summary.ApprovalLoops,
			report.Summary.ReopenRetryCount)
		fmt.Printf("overhead: reviews=%d approvals=%d changes_requested=%d same_lane_mappings=%d notifications=%d notification_failures=%d wakes=%d handoffs=%d approval_loops=%d reopen_retry_count=%d review_waits_no_changes=%d review_usefulness_ratio=%.2f tasks_with_process_overhead=%d tasks_with_engineering_output=%d\n",
			report.Overhead.ReviewRecords,
			report.Overhead.ReviewApprovals,
			report.Overhead.ReviewChangesRequested,
			report.Overhead.SameLaneReviewMappings,
			report.Overhead.Notifications,
			report.Overhead.NotificationFailures,
			report.Overhead.Wakes,
			report.Overhead.Handoffs,
			report.Overhead.ApprovalLoops,
			report.Overhead.ReopenRetryCount,
			report.Overhead.ReviewWaitsNoChanges,
			report.Overhead.ReviewUsefulnessRatio,
			report.Overhead.TasksWithProcessOverhead,
			report.Overhead.TasksWithEngineeringOutput)
		if len(report.OutcomeSources) > 0 {
			fmt.Println("outcome_sources:")
			for _, source := range report.OutcomeSources {
				fmt.Printf("- %s=%d\n", source.Source, source.Count)
			}
		}
		if len(report.Loops) > 0 {
			fmt.Println("loop_signals:")
			for _, loop := range report.Loops {
				fmt.Printf("- task=%s signal=%s count=%d next=%s\n", loop.TaskID, loop.Signal, loop.Count, loop.Recommended)
			}
		}
		if len(report.Rehearsals) > 0 {
			fmt.Println("rehearsal_failures:")
			for _, rehearsal := range report.Rehearsals {
				fmt.Printf("- packet=%s check=%s count=%d tasks=%s\n", rehearsal.PacketID, rehearsal.CheckID, rehearsal.Count, strings.Join(rehearsal.TaskIDs, ","))
			}
		}
		if len(report.Batches) > 0 {
			fmt.Println("batches:")
			for _, batch := range report.Batches {
				fmt.Printf("- batch=%s tasks=%d completed=%d blocked_seconds=%d review_wait_seconds=%d reviews=%d approval_loops=%d reopen_retry_count=%d notifications=%d handoffs=%d\n",
					batch.BatchID,
					batch.Tasks,
					batch.CompletedTasks,
					batch.BlockedSeconds,
					batch.ReviewWaitSeconds,
					batch.ReviewRecords,
					batch.ApprovalLoops,
					batch.ReopenRetryCount,
					batch.Notifications,
					batch.Handoffs)
			}
		}
		if len(report.Rows) == 0 {
			fmt.Println("rows: none")
			return nil
		}
		fmt.Println("rows:")
		for _, row := range report.Rows {
			fmt.Printf("- task=%s status=%s completed_at=%s blocked_seconds=%d review_wait_seconds=%d reviews=%d changes_requested=%d approval_loops=%d reopen_retry_count=%d notifications=%d handoffs=%d outcome=%s defect_source=%s loop=%s\n",
				row.TaskID,
				row.Status,
				firstNonEmpty(row.CompletedAt, "none"),
				row.BlockedSeconds,
				row.ReviewWaitSeconds,
				row.ReviewRecords,
				row.ReviewChangesRequested,
				row.ApprovalLoops,
				row.ReopenRetryCount,
				row.Notifications,
				row.Handoffs,
				firstNonEmpty(row.OutcomeSource, "none"),
				firstNonEmpty(row.DefectSource, "none"),
				firstNonEmpty(row.LoopSignal, "none"))
		}
		return nil
	})
}

func cmdRoughEdge(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		fmt.Println("fairway rough-edge add --task <task-id> --owner <role> --severity <low|medium|high|critical> --decision <fix-now|defer> --summary <text> [--expires <date>] [--artifact <path>]")
		fmt.Println("fairway rough-edge list [--task <task-id>] [--owner <role>] [--expired] [--format text|json]")
		fmt.Println("  Record and inspect owner rough edges found while using the product; read-only list does not mutate backlog or workflow.")
		return nil
	}
	switch args[0] {
	case "add":
		return cmdRoughEdgeAdd(ctx, opts, args[1:])
	case "list":
		return cmdRoughEdgeList(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown rough-edge subcommand %q", args[0])
	}
}

func cmdRoughEdgeAdd(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("rough-edge add", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	owner := fs.String("owner", "", "owner role or lane")
	severity := fs.String("severity", "medium", "low, medium, high, or critical")
	decision := fs.String("decision", "defer", "fix-now or defer")
	summary := fs.String("summary", "", "rough-edge summary")
	expires := fs.String("expires", "", "expiry date or timestamp")
	artifact := fs.String("artifact", "", "linked evidence artifact path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected rough-edge add arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*taskID) == "" {
		return errors.New("--task is required")
	}
	if strings.TrimSpace(*owner) == "" {
		return errors.New("--owner is required")
	}
	if strings.TrimSpace(*summary) == "" {
		return errors.New("--summary is required")
	}
	switch strings.ToLower(strings.TrimSpace(*severity)) {
	case "low", "medium", "high", "critical":
	default:
		return errors.New("--severity must be low, medium, high, or critical")
	}
	switch strings.ToLower(strings.TrimSpace(*decision)) {
	case "fix-now", "defer":
	default:
		return errors.New("--decision must be fix-now or defer")
	}
	if strings.TrimSpace(*expires) != "" {
		if _, ok := roughedge.ParseExpiry(*expires); !ok {
			return errors.New("--expires must be RFC3339Nano, RFC3339, or YYYY-MM-DD")
		}
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		ev, err := roughedge.Evidence("rough-edge: "+strings.TrimSpace(*summary), "partial", *artifact, roughedge.Notes{
			Owner:    *owner,
			Severity: *severity,
			Decision: *decision,
			Expires:  *expires,
			Summary:  *summary,
		})
		if err != nil {
			return err
		}
		if err := s.RecordEvidence(ctx, *taskID, ev); err != nil {
			return err
		}
		if opts.JSON {
			rows, err := roughedge.Rows(ctx, s, time.Now().UTC())
			if err != nil {
				return err
			}
			for _, row := range rows {
				if row.TaskID == *taskID && row.Summary == strings.TrimSpace(*summary) {
					return printJSON(row)
				}
			}
			return printJSON(map[string]string{"task_id": *taskID, "summary": *summary})
		}
		fmt.Printf("rough_edge recorded task=%s owner=%s severity=%s decision=%s\n", *taskID, *owner, *severity, *decision)
		return nil
	})
}

func cmdRoughEdgeList(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("rough-edge list", flag.ContinueOnError)
	taskID := fs.String("task", "", "filter by task id")
	owner := fs.String("owner", "", "filter by owner")
	expiredOnly := fs.Bool("expired", false, "only expired rough edges")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected rough-edge list arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *format != "text" && *format != "json" {
		return errors.New("--format must be text or json")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		rows, err := roughedge.Rows(ctx, s, time.Now().UTC())
		if err != nil {
			return err
		}
		filtered := make([]roughedge.Row, 0, len(rows))
		for _, row := range rows {
			if *taskID != "" && row.TaskID != *taskID {
				continue
			}
			if *owner != "" && row.Owner != *owner {
				continue
			}
			if *expiredOnly && !row.Expired {
				continue
			}
			filtered = append(filtered, row)
		}
		if opts.JSON || *format == "json" {
			return printJSON(filtered)
		}
		if len(filtered) == 0 {
			fmt.Println("rough_edges: none")
			return nil
		}
		fmt.Println("rough_edges:")
		for _, row := range filtered {
			fmt.Printf("- task=%s owner=%s severity=%s decision=%s expires=%s expired=%t artifact=%s summary=%s\n",
				row.TaskID,
				row.Owner,
				row.Severity,
				row.Decision,
				firstNonEmpty(row.Expires, "none"),
				row.Expired,
				firstNonEmpty(row.ArtifactPath, "none"),
				row.Summary)
		}
		return nil
	})
}

func cmdProvenance(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		fmt.Println("fairway provenance report [--task <task-id>|--since <duration>] [--format text|markdown|json]")
		fmt.Println("fairway provenance prompt-packet --task <task-id> [--format markdown|json]")
		fmt.Println("fairway provenance manifest --path <file>... [--format text|json]")
		fmt.Println("  Export metadata-only supply-chain provenance and bounded task prompt packets from existing Fairway state.")
		return nil
	}
	switch args[0] {
	case "report":
		return cmdProvenanceReport(ctx, opts, args[1:])
	case "prompt-packet":
		return cmdProvenancePromptPacket(ctx, opts, args[1:])
	case "manifest":
		return cmdProvenanceManifest(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown provenance subcommand %q", args[0])
	}
}

func cmdProvenanceReport(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway provenance report [--task <task-id>|--since <duration>] [--format text|markdown|json]")
		fmt.Println("  Export task/day/range provenance refs without raw prompts, transcripts, tool bodies, generated content, or secrets.")
		return nil
	}
	fs := flag.NewFlagSet("provenance report", flag.ContinueOnError)
	taskID := fs.String("task", "", "limit to task id")
	since := fs.Duration("since", 7*24*time.Hour, "time window when --task is omitted")
	format := fs.String("format", "text", "text, markdown, or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected provenance report arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *taskID != "" && *since != 7*24*time.Hour {
		return errors.New("--task and --since cannot be combined")
	}
	if *taskID == "" && *since <= 0 {
		return errors.New("--since must be positive")
	}
	if *format != "text" && *format != "markdown" && *format != "json" {
		return errors.New("--format must be text, markdown, or json")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, configPath string, s *store.Store) error {
		report, err := provenance.Build(ctx, cfg, configPath, s, provenance.Options{TaskID: *taskID, Since: *since})
		if err != nil {
			return err
		}
		if opts.JSON || *format == "json" {
			return printJSON(report)
		}
		if *format == "markdown" {
			printProvenanceReportMarkdown(report)
			return nil
		}
		printProvenanceReportText(report)
		return nil
	})
}

func cmdProvenancePromptPacket(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway provenance prompt-packet --task <task-id> [--format markdown|json]")
		fmt.Println("  Render a bounded task prompt packet from existing metadata and refs; does not authorize execution.")
		return nil
	}
	fs := flag.NewFlagSet("provenance prompt-packet", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	format := fs.String("format", "markdown", "markdown or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected provenance prompt-packet arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *taskID == "" {
		return errors.New("provenance prompt-packet requires --task")
	}
	if *format != "markdown" && *format != "json" {
		return errors.New("--format must be markdown or json")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, configPath string, s *store.Store) error {
		packet, err := provenance.BuildPromptPacket(ctx, cfg, configPath, s, *taskID, time.Time{})
		if err != nil {
			return err
		}
		if opts.JSON || *format == "json" {
			return printJSON(packet)
		}
		printProvenancePromptPacketMarkdown(packet)
		return nil
	})
}

func cmdProvenanceManifest(_ context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway provenance manifest --path <file>... [--format text|json]")
		fmt.Println("  Build a content-free sha256 manifest for selected evidence or provenance exports.")
		return nil
	}
	fs := flag.NewFlagSet("provenance manifest", flag.ContinueOnError)
	var paths multiFlag
	fs.Var(&paths, "path", "artifact or export path; repeatable")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected provenance manifest arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *format != "text" && *format != "json" {
		return errors.New("--format must be text or json")
	}
	manifest, err := provenance.BuildManifest(provenance.ManifestOptions{Paths: paths})
	if err != nil {
		return err
	}
	if opts.JSON || *format == "json" {
		if err := printJSON(manifest); err != nil {
			return err
		}
	} else {
		printProvenanceManifestText(manifest)
	}
	if !manifest.OK {
		return errors.New("provenance manifest failed")
	}
	return nil
}

func printProvenanceReportText(report provenance.Report) {
	fmt.Println("provenance_report_ok: true")
	fmt.Printf("schema: %s\n", report.Schema)
	fmt.Printf("generated_at: %s\n", report.GeneratedAt)
	fmt.Printf("project: %s\n", report.Project.Name)
	if report.Scope.TaskID != "" {
		fmt.Printf("task: %s\n", report.Scope.TaskID)
	} else {
		fmt.Printf("window: %s..%s\n", firstNonEmpty(report.Scope.Since, "all"), firstNonEmpty(report.Scope.Until, "now"))
	}
	fmt.Printf("privacy: raw_prompts=%t transcripts=%t tool_bodies=%t generated_content=%t redaction_applied=%t\n",
		report.Privacy.RawPromptsIncluded,
		report.Privacy.TranscriptsIncluded,
		report.Privacy.ToolBodiesIncluded,
		report.Privacy.GeneratedContentIncluded,
		report.Privacy.RedactionApplied)
	if len(report.Warnings) > 0 {
		fmt.Println("warnings:")
		for _, warning := range report.Warnings {
			fmt.Printf("- %s\n", warning)
		}
	}
	if len(report.Tasks) == 0 {
		fmt.Println("tasks: none")
		return
	}
	fmt.Println("tasks:")
	for _, task := range report.Tasks {
		fmt.Printf("- %s status=%s role=%s evidence=%d reviews=%d checkpoints=%d sessions=%d usage=%d commits=%s\n",
			task.ID,
			task.Status,
			firstNonEmpty(task.Role, "unknown"),
			len(task.EvidenceRefs),
			len(task.ReviewRefs),
			len(task.CheckpointRefs),
			len(task.SessionRefs),
			len(task.UsageRefs),
			firstNonEmpty(strings.Join(task.CommitRefs, ","), "none"))
		for _, gate := range task.ValidationGates {
			fmt.Printf("  gate: %s\n", gate)
		}
		for _, ref := range task.EvidenceRefs {
			fmt.Printf("  evidence: %s result=%s artifact=%s command=%s\n", ref.Ref, firstNonEmpty(ref.Result, "unknown"), firstNonEmpty(ref.ArtifactPath, "none"), firstNonEmpty(ref.CommandText, "none"))
		}
		for _, ref := range task.ReviewRefs {
			fmt.Printf("  review: %s domain=%s verdict=%s reviewer=%s\n", ref.Ref, firstNonEmpty(ref.Domain, "unspecified"), firstNonEmpty(ref.Verdict, "unknown"), firstNonEmpty(ref.Reviewer, "unknown"))
		}
	}
}

func printProvenanceManifestText(manifest provenance.Manifest) {
	fmt.Printf("provenance_manifest_ok: %t\n", manifest.OK)
	fmt.Printf("schema: %s\n", manifest.Schema)
	fmt.Printf("generated_at: %s\n", manifest.GeneratedAt)
	fmt.Printf("algorithm: %s\n", manifest.Algorithm)
	fmt.Println("privacy: file contents are hashed only and are not embedded")
	if len(manifest.Entries) == 0 {
		fmt.Println("entries: none")
	} else {
		fmt.Println("entries:")
		for _, entry := range manifest.Entries {
			fmt.Printf("- %s status=%s bytes=%d sha256=%s\n", entry.Path, entry.Status, entry.Bytes, firstNonEmpty(entry.SHA256, "none"))
		}
	}
	if len(manifest.Issues) > 0 {
		fmt.Println("issues:")
		for _, issue := range manifest.Issues {
			fmt.Printf("- %s\n", issue)
		}
	}
	if len(manifest.Warnings) > 0 {
		fmt.Println("warnings:")
		for _, warning := range manifest.Warnings {
			fmt.Printf("- %s\n", warning)
		}
	}
}

func printProvenanceReportMarkdown(report provenance.Report) {
	fmt.Println("# Fairway Provenance Report")
	fmt.Println()
	fmt.Printf("- schema: `%s`\n", report.Schema)
	fmt.Printf("- generated_at: `%s`\n", report.GeneratedAt)
	fmt.Printf("- project: `%s`\n", report.Project.Name)
	if report.Scope.TaskID != "" {
		fmt.Printf("- task: `%s`\n", report.Scope.TaskID)
	} else {
		fmt.Printf("- window: `%s` to `%s`\n", firstNonEmpty(report.Scope.Since, "all"), firstNonEmpty(report.Scope.Until, "now"))
	}
	fmt.Println("- privacy: raw prompts, private transcripts, raw tool bodies, and generated-content dumps are excluded")
	if len(report.Warnings) > 0 {
		fmt.Println()
		fmt.Println("## Warnings")
		for _, warning := range report.Warnings {
			fmt.Printf("- %s\n", warning)
		}
	}
	fmt.Println()
	fmt.Println("## Tasks")
	if len(report.Tasks) == 0 {
		fmt.Println("- none")
		return
	}
	for _, task := range report.Tasks {
		fmt.Printf("\n### %s\n\n", task.ID)
		fmt.Printf("- title: %s\n", task.Title)
		fmt.Printf("- status: `%s`\n", task.Status)
		fmt.Printf("- role: `%s`\n", firstNonEmpty(task.Role, "unknown"))
		fmt.Printf("- evidence refs: %d\n", len(task.EvidenceRefs))
		fmt.Printf("- review refs: %d\n", len(task.ReviewRefs))
		fmt.Printf("- checkpoint refs: %d\n", len(task.CheckpointRefs))
		fmt.Printf("- session refs: %d\n", len(task.SessionRefs))
		fmt.Printf("- usage refs: %d\n", len(task.UsageRefs))
		if len(task.CommitRefs) > 0 {
			fmt.Printf("- commit refs: `%s`\n", strings.Join(task.CommitRefs, "`, `"))
		}
		if len(task.ValidationGates) > 0 {
			fmt.Println("- validation gates:")
			for _, gate := range task.ValidationGates {
				fmt.Printf("  - `%s`\n", gate)
			}
		}
		if len(task.EvidenceRefs) > 0 {
			fmt.Println("- evidence:")
			for _, ref := range task.EvidenceRefs {
				fmt.Printf("  - `%s` result=%s artifact=%s command=`%s`\n", ref.Ref, firstNonEmpty(ref.Result, "unknown"), firstNonEmpty(ref.ArtifactPath, "none"), firstNonEmpty(ref.CommandText, "none"))
			}
		}
		if len(task.ReviewRefs) > 0 {
			fmt.Println("- reviews:")
			for _, ref := range task.ReviewRefs {
				fmt.Printf("  - `%s` domain=%s verdict=%s reviewer=%s\n", ref.Ref, firstNonEmpty(ref.Domain, "unspecified"), firstNonEmpty(ref.Verdict, "unknown"), firstNonEmpty(ref.Reviewer, "unknown"))
			}
		}
	}
}

func printProvenancePromptPacketMarkdown(packet provenance.PromptPacket) {
	fmt.Printf("# Fairway Prompt Packet: %s\n\n", packet.TaskID)
	fmt.Printf("- schema: `%s`\n", packet.Schema)
	fmt.Printf("- generated_at: `%s`\n", packet.GeneratedAt)
	fmt.Println()
	fmt.Println("## Objective")
	fmt.Println(packet.Objective)
	if len(packet.Scope) > 0 {
		fmt.Println()
		fmt.Println("## Scope")
		for _, item := range packet.Scope {
			fmt.Printf("- %s\n", item)
		}
	}
	if len(packet.Acceptance) > 0 {
		fmt.Println()
		fmt.Println("## Acceptance")
		for _, item := range packet.Acceptance {
			fmt.Printf("- %s\n", item)
		}
	}
	fmt.Println()
	fmt.Println("## Source Facts")
	for _, fact := range packet.SourceFacts {
		fmt.Printf("- %s\n", fact)
	}
	fmt.Println()
	fmt.Println("## Forbidden Actions")
	for _, item := range packet.ForbiddenActions {
		fmt.Printf("- %s\n", item)
	}
	if len(packet.ValidationGates) > 0 {
		fmt.Println()
		fmt.Println("## Validation Gates")
		for _, gate := range packet.ValidationGates {
			fmt.Printf("- `%s`\n", gate)
		}
	}
	if len(packet.EvidenceRefs) > 0 {
		fmt.Println()
		fmt.Println("## Evidence Refs")
		for _, ref := range packet.EvidenceRefs {
			fmt.Printf("- `%s` result=%s artifact=%s\n", ref.Ref, firstNonEmpty(ref.Result, "unknown"), firstNonEmpty(ref.ArtifactPath, "none"))
		}
	}
	if len(packet.ReviewRefs) > 0 {
		fmt.Println()
		fmt.Println("## Review Refs")
		for _, ref := range packet.ReviewRefs {
			fmt.Printf("- `%s` domain=%s verdict=%s\n", ref.Ref, firstNonEmpty(ref.Domain, "unspecified"), firstNonEmpty(ref.Verdict, "unknown"))
		}
	}
	fmt.Println()
	fmt.Println("## Privacy")
	fmt.Println("- raw prompts, private transcripts, raw tool bodies, generated-content dumps, secrets, and credentials are excluded")
}

type taskRecipe struct {
	Schema           string            `json:"schema"`
	Name             string            `json:"name"`
	Title            string            `json:"title"`
	SourceTaskID     string            `json:"source_task_id"`
	SourceFacts      []string          `json:"source_facts"`
	Objective        string            `json:"objective"`
	Scope            []string          `json:"scope"`
	Inputs           []string          `json:"inputs"`
	ForbiddenActions []string          `json:"forbidden_actions"`
	ValidationGates  []string          `json:"validation_gates"`
	ExpectedEvidence []string          `json:"expected_evidence"`
	CloseoutRules    []string          `json:"closeout_rules"`
	Substitutions    map[string]string `json:"substitutions,omitempty"`
	Privacy          recipePrivacy     `json:"privacy"`
}

type recipePrivacy struct {
	RawPromptsIncluded       bool     `json:"raw_prompts_included"`
	TranscriptsIncluded      bool     `json:"transcripts_included"`
	ToolBodiesIncluded       bool     `json:"tool_bodies_included"`
	GeneratedContentIncluded bool     `json:"generated_content_included"`
	Rejected                 bool     `json:"rejected"`
	Warnings                 []string `json:"warnings,omitempty"`
}

const taskRecipeSchema = "fairway.task-recipe.v1"

func cmdRecipe(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		fmt.Println("fairway recipe extract|render|list ...")
		fmt.Println("  Extract completed tasks into privacy-bounded recipe packets and render them for new tasks; recipes do not store raw prompts or transcripts.")
		return nil
	}
	switch args[0] {
	case "extract":
		return cmdRecipeExtract(ctx, opts, args[1:])
	case "render":
		return cmdRecipeRender(ctx, opts, args[1:])
	case "list":
		return cmdRecipeList(opts, args[1:])
	default:
		return fmt.Errorf("unknown recipe subcommand %q", args[0])
	}
}

func cmdRecipeExtract(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway recipe extract --task <task-id> --name <name> [--output <path>] [--input <text>]... [--forbidden-action <text>]... [--closeout-rule <text>]...")
		return nil
	}
	fs := flag.NewFlagSet("recipe extract", flag.ContinueOnError)
	taskID := fs.String("task", "", "completed source task id")
	name := fs.String("name", "", "recipe name")
	output := fs.String("output", "", "output recipe JSON path")
	var inputs multiFlag
	var forbidden multiFlag
	var closeout multiFlag
	fs.Var(&inputs, "input", "recipe input placeholder or required input")
	fs.Var(&forbidden, "forbidden-action", "forbidden action; may repeat")
	fs.Var(&closeout, "closeout-rule", "closeout rule; may repeat")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected recipe extract arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*taskID) == "" {
		return errors.New("recipe extract requires --task")
	}
	if strings.TrimSpace(*name) == "" {
		return errors.New("recipe extract requires --name")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, *taskID)
		if err != nil {
			return err
		}
		if !terminalStatusSet(cfg.States.Terminal)[task.Status] {
			return fmt.Errorf("recipe extract requires completed source task %s, status=%s", task.Definition.ID, task.Status)
		}
		recipe := buildTaskRecipe(*name, task, evidence, reviews, inputs, forbidden, closeout)
		if err := validateRecipePrivacy(recipe); err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(recipe)
		}
		path := strings.TrimSpace(*output)
		if path == "" {
			path = filepath.Join(root, ".fairway", "recipes", sanitizeRecipeFileName(*name)+".json")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		data, err := json.MarshalIndent(recipe, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("recipe extracted %s source_task=%s path=%s\n", recipe.Name, recipe.SourceTaskID, path)
		return nil
	})
}

func cmdRecipeRender(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway recipe render --recipe <path> --task <task-id> [--field <key=value>]... [--format markdown|json]")
		return nil
	}
	fs := flag.NewFlagSet("recipe render", flag.ContinueOnError)
	recipePath := fs.String("recipe", "", "recipe JSON path")
	taskID := fs.String("task", "", "target task id for rendered packet")
	format := fs.String("format", "markdown", "markdown or json")
	var fields multiFlag
	fs.Var(&fields, "field", "substitution field as key=value; may repeat")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected recipe render arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*recipePath) == "" {
		return errors.New("recipe render requires --recipe")
	}
	if strings.TrimSpace(*taskID) == "" {
		return errors.New("recipe render requires --task")
	}
	if *format != "markdown" && *format != "json" {
		return errors.New("--format must be markdown or json")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, root string, s *store.Store) error {
		recipe, err := readTaskRecipe(root, *recipePath)
		if err != nil {
			return err
		}
		values, err := parsePacketTemplateFields(fields)
		if err != nil {
			return err
		}
		rendered, err := renderTaskRecipe(ctx, s, recipe, *taskID, values)
		if err != nil {
			return err
		}
		if opts.JSON || *format == "json" {
			return printJSON(rendered)
		}
		printTaskRecipePacket(rendered)
		return nil
	})
}

func cmdRecipeList(opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway recipe list [--dir <path>] [--format text|json]")
		return nil
	}
	fs := flag.NewFlagSet("recipe list", flag.ContinueOnError)
	dir := fs.String("dir", ".fairway/recipes", "recipe directory")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected recipe list arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *format != "text" && *format != "json" {
		return errors.New("--format must be text or json")
	}
	entries, err := recipeList(*dir)
	if err != nil {
		return err
	}
	if opts.JSON || *format == "json" {
		return printJSON(entries)
	}
	fmt.Println("recipes:")
	if len(entries) == 0 {
		fmt.Println("- none")
	}
	for _, recipe := range entries {
		fmt.Printf("- %s source_task=%s path=%s title=%s\n", recipe.Name, recipe.SourceTaskID, recipePathDisplay(recipe), recipe.Title)
	}
	return nil
}

func buildTaskRecipe(name string, task store.Task, evidence []store.Evidence, reviews []store.Review, inputs, forbidden, closeout []string) taskRecipe {
	recipe := taskRecipe{
		Schema:       taskRecipeSchema,
		Name:         strings.TrimSpace(name),
		Title:        firstNonEmpty(task.Definition.Title, task.Definition.ID),
		SourceTaskID: task.Definition.ID,
		Objective:    firstNonEmpty(task.Definition.Notes, task.Definition.Title),
		Scope: cleanRepeatedValues(append(append([]string{},
			task.Definition.SourcePaths...),
			task.Definition.TargetPaths...,
		)),
		Inputs:           cleanRepeatedValues(inputs),
		ForbiddenActions: cleanRepeatedValues(forbidden),
		CloseoutRules:    cleanRepeatedValues(closeout),
		Substitutions:    map[string]string{"task_id": "{{task_id}}"},
		Privacy: recipePrivacy{
			RawPromptsIncluded:       false,
			TranscriptsIncluded:      false,
			ToolBodiesIncluded:       false,
			GeneratedContentIncluded: false,
		},
	}
	if len(recipe.ForbiddenActions) == 0 {
		recipe.ForbiddenActions = []string{"do not include raw prompt bodies", "do not include private transcripts", "do not include raw tool bodies", "do not include credentials or secrets", "do not treat recipe output as approval, merge, deploy, release, or live-operation authority"}
	}
	for _, ev := range evidence {
		ref := firstNonEmpty(ev.ArtifactPath, ev.ArtifactType, ev.CommandText)
		if ref != "" {
			recipe.SourceFacts = append(recipe.SourceFacts, fmt.Sprintf("evidence:%s result=%s ref=%s", task.Definition.ID, firstNonEmpty(ev.Result, "unknown"), ref))
		}
		if ev.ArtifactType != "" {
			recipe.ExpectedEvidence = append(recipe.ExpectedEvidence, ev.ArtifactType)
		}
		if ev.CommandText != "" && (ev.Result == "pass" || ev.Result == "partial") {
			recipe.ValidationGates = append(recipe.ValidationGates, ev.CommandText)
		}
	}
	for _, review := range reviews {
		recipe.SourceFacts = append(recipe.SourceFacts, fmt.Sprintf("review:%s domain=%s verdict=%s", task.Definition.ID, firstNonEmpty(review.Domain, "unspecified"), firstNonEmpty(review.Verdict, "unknown")))
	}
	recipe.SourceFacts = uniqueStrings(recipe.SourceFacts)
	recipe.ExpectedEvidence = uniqueStrings(recipe.ExpectedEvidence)
	recipe.ValidationGates = uniqueStrings(recipe.ValidationGates)
	return recipe
}

func readTaskRecipe(root, path string) (taskRecipe, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return taskRecipe{}, err
	}
	var recipe taskRecipe
	if err := json.Unmarshal(data, &recipe); err != nil {
		return taskRecipe{}, err
	}
	if recipe.Schema != taskRecipeSchema {
		if recipe.Schema == "" {
			return taskRecipe{}, errors.New("recipe missing schema")
		}
		return taskRecipe{}, fmt.Errorf("unsupported recipe schema %q", recipe.Schema)
	}
	if recipe.Name == "" {
		return taskRecipe{}, errors.New("recipe missing name")
	}
	if len(recipe.SourceFacts) == 0 {
		return taskRecipe{}, errors.New("recipe missing source facts")
	}
	if err := validateRecipePrivacy(recipe); err != nil {
		return taskRecipe{}, err
	}
	return recipe, nil
}

func renderTaskRecipe(ctx context.Context, s *store.Store, recipe taskRecipe, taskID string, values map[string][]string) (taskRecipe, error) {
	task, _, _, _, _, err := s.TaskDetail(ctx, taskID)
	if err != nil {
		return taskRecipe{}, err
	}
	rendered := recipe
	rendered.SourceTaskID = recipe.SourceTaskID
	rendered.Substitutions = map[string]string{"task_id": task.Definition.ID, "task_title": task.Definition.Title}
	for key, vals := range values {
		if len(vals) > 0 {
			rendered.Substitutions[key] = vals[len(vals)-1]
		}
	}
	apply := func(value string) string {
		for key, replacement := range rendered.Substitutions {
			value = strings.ReplaceAll(value, "{{"+key+"}}", replacement)
		}
		return value
	}
	rendered.Objective = apply(rendered.Objective)
	for i := range rendered.Scope {
		rendered.Scope[i] = apply(rendered.Scope[i])
	}
	for i := range rendered.Inputs {
		rendered.Inputs[i] = apply(rendered.Inputs[i])
	}
	for i := range rendered.ForbiddenActions {
		rendered.ForbiddenActions[i] = apply(rendered.ForbiddenActions[i])
	}
	for i := range rendered.ValidationGates {
		rendered.ValidationGates[i] = apply(rendered.ValidationGates[i])
	}
	for i := range rendered.ExpectedEvidence {
		rendered.ExpectedEvidence[i] = apply(rendered.ExpectedEvidence[i])
	}
	for i := range rendered.CloseoutRules {
		rendered.CloseoutRules[i] = apply(rendered.CloseoutRules[i])
	}
	if err := validateRecipePrivacy(rendered); err != nil {
		return taskRecipe{}, err
	}
	return rendered, nil
}

func printTaskRecipePacket(recipe taskRecipe) {
	fmt.Printf("# Recipe Packet: %s\n\n", recipe.Name)
	fmt.Printf("source_task: %s\n", recipe.SourceTaskID)
	fmt.Printf("title: %s\n", recipe.Title)
	fmt.Println("\n## Objective")
	fmt.Println(recipe.Objective)
	printRecipeSection("Scope", recipe.Scope)
	printRecipeSection("Inputs", recipe.Inputs)
	printRecipeSection("Forbidden Actions", recipe.ForbiddenActions)
	printRecipeSection("Validation Gates", recipe.ValidationGates)
	printRecipeSection("Expected Evidence", recipe.ExpectedEvidence)
	printRecipeSection("Closeout Rules", recipe.CloseoutRules)
	printRecipeSection("Source Facts", recipe.SourceFacts)
	fmt.Println("\n## Privacy")
	fmt.Println("- raw prompts, private transcripts, raw tool bodies, generated-content dumps, secrets, and credentials are excluded")
}

func printRecipeSection(title string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Printf("\n## %s\n", title)
	for _, value := range values {
		fmt.Printf("- %s\n", value)
	}
}

func validateRecipePrivacy(recipe taskRecipe) error {
	if recipe.Privacy.RawPromptsIncluded || recipe.Privacy.TranscriptsIncluded || recipe.Privacy.ToolBodiesIncluded || recipe.Privacy.GeneratedContentIncluded || recipe.Privacy.Rejected {
		return errors.New("recipe privacy rejected: raw prompts, transcripts, tool bodies, or generated content are not allowed")
	}
	for _, value := range recipeTextValues(recipe) {
		lower := strings.ToLower(value)
		for _, marker := range []string{"raw_prompt:", "raw_prompt=", "transcript:", "tool_body:", "tool_body=", "generated_content:", "generated_content=", "authorization:", "bearer ", "api_key=", "access_token=", "refresh_token=", "client_secret=", "password=", "secret="} {
			if strings.Contains(lower, marker) {
				return fmt.Errorf("recipe privacy rejected: disallowed marker %q", marker)
			}
		}
	}
	return nil
}

func recipeTextValues(recipe taskRecipe) []string {
	values := []string{recipe.Schema, recipe.Name, recipe.Title, recipe.SourceTaskID, recipe.Objective}
	values = append(values, recipe.SourceFacts...)
	values = append(values, recipe.Scope...)
	values = append(values, recipe.Inputs...)
	values = append(values, recipe.ForbiddenActions...)
	values = append(values, recipe.ValidationGates...)
	values = append(values, recipe.ExpectedEvidence...)
	values = append(values, recipe.CloseoutRules...)
	values = append(values, recipe.Privacy.Warnings...)
	for key, value := range recipe.Substitutions {
		values = append(values, key, value)
	}
	return values
}

func recipeList(dir string) ([]taskRecipe, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var recipes []taskRecipe
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		recipe, err := readTaskRecipe("", path)
		if err != nil {
			return nil, fmt.Errorf("invalid recipe %s: %w", path, err)
		}
		recipes = append(recipes, recipe)
	}
	sort.SliceStable(recipes, func(i, j int) bool { return recipes[i].Name < recipes[j].Name })
	return recipes, nil
}

func recipePathDisplay(recipe taskRecipe) string {
	return filepath.Join(".fairway", "recipes", sanitizeRecipeFileName(recipe.Name)+".json")
}

func sanitizeRecipeFileName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '/':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "recipe"
	}
	return out
}

func cmdAutomation(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		fmt.Println("fairway automation candidates --since <duration> [--threshold <n>] [--format text|json]")
		fmt.Println("  Read-only repeated-work automation candidate report; does not create tasks or mutate workflow.")
		return nil
	}
	switch args[0] {
	case "candidates":
		return cmdAutomationCandidates(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown automation subcommand %q", args[0])
	}
}

func cmdAutomationCandidates(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway automation candidates --since <duration> [--threshold <n>] [--format text|json]")
		fmt.Println("  Report repeated deterministic work candidates without auto-creating tasks.")
		return nil
	}
	fs := flag.NewFlagSet("automation candidates", flag.ContinueOnError)
	since := fs.Duration("since", 7*24*time.Hour, "time window")
	threshold := fs.Int("threshold", 3, "minimum repeated occurrences")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected automation candidates arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *since <= 0 {
		return errors.New("--since must be positive")
	}
	if *threshold < 2 {
		return errors.New("--threshold must be at least 2")
	}
	if *format != "text" && *format != "json" {
		return errors.New("--format must be text or json")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		report, err := automationreport.Build(ctx, s, automationreport.Options{Since: *since, Threshold: *threshold})
		if err != nil {
			return err
		}
		if opts.JSON || *format == "json" {
			return printJSON(report)
		}
		fmt.Printf("automation_candidates_ok: %t\n", report.OK)
		fmt.Printf("since: %s\n", report.Since)
		fmt.Printf("threshold: %d\n", report.Threshold)
		if len(report.Candidates) == 0 {
			fmt.Println("candidates: none")
			return nil
		}
		fmt.Println("candidates:")
		for _, candidate := range report.Candidates {
			fmt.Printf("- kind=%s pattern=%s frequency=%d owner=%s surface=%s cost=%s next=%s\n",
				candidate.Kind,
				candidate.Pattern,
				candidate.Frequency,
				firstNonEmpty(candidate.LikelyOwner, "unknown"),
				candidate.SuggestedSurface,
				candidate.EstimatedCoordinationCost,
				candidate.RecommendedAction)
			if len(candidate.RecentTaskIDs) > 0 {
				fmt.Printf("  recent_tasks: %s\n", strings.Join(candidate.RecentTaskIDs, ", "))
			}
			if len(candidate.RepresentativeCommands) > 0 {
				fmt.Printf("  representative_commands: %s\n", strings.Join(candidate.RepresentativeCommands, " | "))
			}
			if len(candidate.RepresentativeArtifacts) > 0 {
				fmt.Printf("  representative_artifacts: %s\n", strings.Join(candidate.RepresentativeArtifacts, ", "))
			}
		}
		return nil
	})
}

type mergeReadyReport struct {
	OK                   bool                     `json:"ok"`
	TaskID               string                   `json:"task_id"`
	Git                  fairwaygit.Status        `json:"git"`
	AllowedArtifactPaths []string                 `json:"allowed_artifact_paths,omitempty"`
	DirtyPaths           []string                 `json:"dirty_paths,omitempty"`
	Issues               []string                 `json:"issues"`
	Warnings             []string                 `json:"warnings,omitempty"`
	MissingReviewDomains []string                 `json:"missing_review_domains,omitempty"`
	ReviewPolicy         reviewpolicy.Evaluation  `json:"review_policy,omitempty"`
	GateEvaluations      []adoptionGateEvaluation `json:"gate_evaluations,omitempty"`
	RuleEvaluations      []ruleEvidenceEvaluation `json:"rule_evaluations,omitempty"`
}

func cmdMergeReady(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("merge-ready requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("merge-ready", flag.ContinueOnError)
	base := fs.String("base", "", "base ref")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected merge-ready arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		if *base == "" {
			*base = cfg.Fairway.MainBranch
		}
		gitStatus, err := fairwaygit.Check(root, *base)
		if err != nil {
			return err
		}
		report := mergeReadyReport{OK: true, TaskID: taskID, Git: gitStatus}
		cleanliness := evaluateWorktreeCleanliness(gitStatus, localArtifactAllowlist(cfg, evidence))
		report.AllowedArtifactPaths = cleanliness.AllowedArtifactPaths
		report.DirtyPaths = cleanliness.DirtyPaths
		if cleanliness.Dirty {
			report.Issues = append(report.Issues, "worktree has uncommitted changes")
		}
		if *base != "" && !gitStatus.BaseAncestor {
			report.Issues = append(report.Issues, fmt.Sprintf("HEAD is not based on %q", *base))
		}
		if cfg.Gates.RequireEvidenceBeforeDone {
			ok, err := s.HasEvidence(ctx, taskID)
			if err != nil {
				return err
			}
			if !ok {
				report.Issues = append(report.Issues, "missing evidence")
			}
		}
		if cfg.Gates.RequireReviewBeforeDone {
			ok, err := s.HasApprovedReview(ctx, taskID)
			if err != nil {
				return err
			}
			if !ok {
				report.Issues = append(report.Issues, "missing approved review")
			}
		}
		report.ReviewPolicy, err = reviewPolicyEvaluation(ctx, cfg, s, task, reviews, cleanliness.DirtyPaths)
		if err != nil {
			return err
		}
		report.MissingReviewDomains = report.ReviewPolicy.MissingReviewDomains
		for _, domain := range report.MissingReviewDomains {
			message := "missing approved review for domain " + domain + " (" + reviewPolicyDomainReason(report.ReviewPolicy, domain) + ")"
			if report.ReviewPolicy.Mode == "advisory" {
				report.Warnings = append(report.Warnings, "advisory review profile: "+message)
			} else {
				report.Issues = append(report.Issues, message)
			}
		}
		if cfg.Gates.RequireHandoffBeforeMergeReady {
			ok, err := s.HasHandoff(ctx, taskID)
			if err != nil {
				return err
			}
			if !ok {
				report.Issues = append(report.Issues, "missing handoff")
			}
		}
		report.GateEvaluations = evaluateTaskProfileGates(cfg, task, evidence, time.Now().UTC())
		for _, evaluation := range report.GateEvaluations {
			if evaluation.Status != "missing" {
				continue
			}
			message := mergeReadyGateMessage(evaluation)
			if evaluation.Mode == "blocking" {
				report.Issues = append(report.Issues, message)
			} else {
				report.Warnings = append(report.Warnings, message)
			}
		}
		ruleEvaluations, err := ruleEvidenceEvaluations(cfg, root, task, evidence)
		if err != nil {
			return err
		}
		report.RuleEvaluations = ruleEvaluations
		for _, evaluation := range ruleEvaluations {
			if evaluation.Status != "missing" {
				continue
			}
			message := ruleEvidenceMessage(evaluation)
			if evaluation.Mode == "blocking" {
				report.Issues = append(report.Issues, message)
			} else {
				report.Warnings = append(report.Warnings, message)
			}
		}
		report.OK = len(report.Issues) == 0
		if opts.JSON {
			if err := printJSON(report); err != nil {
				return err
			}
		} else {
			fmt.Printf("merge_ready: %t\ntask: %s\nbranch: %s\nbase: %s\n", report.OK, report.TaskID, report.Git.Branch, report.Git.Base)
			printWorktreeCleanliness(report.DirtyPaths, report.AllowedArtifactPaths)
			if len(report.Issues) > 0 {
				fmt.Println("issues:")
				for _, issue := range report.Issues {
					fmt.Printf("- %s\n", issue)
				}
			}
			if len(report.Warnings) > 0 {
				fmt.Println("warnings:")
				for _, warning := range report.Warnings {
					fmt.Printf("- %s\n", warning)
				}
			}
			if len(report.GateEvaluations) > 0 {
				fmt.Println("profile_gates:")
				for _, evaluation := range report.GateEvaluations {
					fmt.Printf("- %s/%s: %s (%d/%d satisfied)\n", evaluation.Profile, evaluation.Gate, evaluation.Status, evaluation.SatisfiedCount, evaluation.TaskCount)
				}
			}
			if len(report.RuleEvaluations) > 0 {
				fmt.Println("rule_evidence:")
				for _, evaluation := range report.RuleEvaluations {
					fmt.Printf("- %s: %s mode=%s evidence=%s\n", evaluation.RuleID, evaluation.Status, evaluation.Mode, strings.Join(evaluation.RequiredEvidence, ","))
				}
			}
			if len(report.ReviewPolicy.Requirements) > 0 {
				printReviewPolicyEvaluation(report.ReviewPolicy)
			}
		}
		if !report.OK {
			return errors.New("merge-ready failed")
		}
		return nil
	})
}

func cmdRouteReview(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("route review requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("route review", flag.ContinueOnError)
	reviewer := fs.String("reviewer", "", "reviewer role")
	reason := fs.String("reason", "", "route reason")
	paths := multiFlag{}
	fs.Var(&paths, "path", "changed path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected route review arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		task, _, _, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		selected := *reviewer
		routeReason := *reason
		if selected == "" {
			selected, routeReason = matchReviewRoute(cfg.ReviewRoutes, []string(paths))
		}
		if selected == "" && len(paths) == 0 {
			changed, err := fairwaygit.ChangedFiles(root, cfg.Fairway.MainBranch)
			if err != nil {
				return err
			}
			selected, routeReason = matchReviewRoute(cfg.ReviewRoutes, changed)
		}
		if selected == "" {
			return errors.New("no review route matched; pass --reviewer or --path")
		}
		if selected == task.Owner || selected == task.Claimant {
			return errors.New("reviewer cannot review their own task")
		}
		if err := validateReviewer(selected, cfg); err != nil {
			return err
		}
		if issues := reviewstate.UnroutableRequiredDomains(task, reviewWaitOptions(cfg)); len(issues) > 0 {
			return fmt.Errorf("required review domain %s for %s is not routable: %s", issues[0].Domain, taskID, issues[0].Reason)
		}
		if routeReason == "" {
			routeReason = "manual route"
		}
		if err := s.RouteReview(ctx, taskID, selected, routeReason); err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(struct {
				TaskID   string `json:"task_id"`
				Reviewer string `json:"reviewer"`
				Reason   string `json:"reason"`
			}{taskID, selected, routeReason})
		}
		fmt.Printf("routed %s to %s: %s\n", taskID, selected, routeReason)
		return nil
	})
}

func cmdReviewWaits(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("review-waits requires subcommand: list, wake")
	}
	if isHelpOnly(args) {
		subcommandUsage("review-waits", "list [--blocking] [--task <task-id>] [--stale] | wake [--task <task-id>] [--send]")
		return nil
	}
	switch args[0] {
	case "list":
		return cmdReviewWaitsList(ctx, opts, args[1:])
	case "wake":
		return cmdReviewWaitsWake(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown review-waits subcommand %q", args[0])
	}
}

type reviewPolicyReportRow struct {
	Profile               string   `json:"profile"`
	Mode                  string   `json:"mode"`
	ProcessHypothesis     string   `json:"process_hypothesis,omitempty"`
	OutcomeMetrics        []string `json:"outcome_metrics,omitempty"`
	TaskCount             int      `json:"task_count"`
	RequiredReviewDomains int      `json:"required_review_domains"`
	ApprovedReviews       int      `json:"approved_reviews"`
	MissingReviews        int      `json:"missing_reviews"`
	InheritedReviews      int      `json:"inherited_reviews"`
	WaivedReviews         int      `json:"waived_reviews"`
	DeferredReviews       int      `json:"deferred_reviews"`
	DefectsCaught         int      `json:"defects_caught"`
	ReworkReducedSignals  int      `json:"rework_reduced_signals"`
	AvoidedUnsafeActions  int      `json:"avoided_unsafe_actions"`
	BlockedTasks          int      `json:"blocked_tasks"`
	CompletedTasks        int      `json:"completed_tasks"`
	LoopDetected          int      `json:"loop_detected"`
	CausalResetAdvice     []string `json:"causal_reset_advice,omitempty"`
	Recommendation        string   `json:"recommendation"`
}

func cmdReviewPolicy(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("review-policy", "report [--profile <name>]")
		return nil
	}
	switch args[0] {
	case "report":
		return cmdReviewPolicyReport(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown review-policy subcommand %q", args[0])
	}
}

func cmdReviewPolicyReport(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("review-policy", "report [--profile <name>]")
		return nil
	}
	fs := flag.NewFlagSet("review-policy report", flag.ContinueOnError)
	profileName := fs.String("profile", "", "review profile name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected review-policy report arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		rows, err := reviewPolicyReportRows(ctx, cfg, s, *profileName)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(rows)
		}
		fmt.Println("review_policy_report:")
		if len(rows) == 0 {
			fmt.Println("- none")
			return nil
		}
		for _, row := range rows {
			fmt.Printf("- profile=%s mode=%s tasks=%d required_domains=%d approved=%d missing=%d inherited=%d waived=%d deferred=%d defects_caught=%d rework_signals=%d avoided_unsafe=%d blocked_tasks=%d completed_tasks=%d loop_detected=%d recommendation=%s\n",
				row.Profile,
				row.Mode,
				row.TaskCount,
				row.RequiredReviewDomains,
				row.ApprovedReviews,
				row.MissingReviews,
				row.InheritedReviews,
				row.WaivedReviews,
				row.DeferredReviews,
				row.DefectsCaught,
				row.ReworkReducedSignals,
				row.AvoidedUnsafeActions,
				row.BlockedTasks,
				row.CompletedTasks,
				row.LoopDetected,
				row.Recommendation,
			)
			if row.ProcessHypothesis != "" {
				fmt.Printf("  hypothesis=%s\n", row.ProcessHypothesis)
			}
			if len(row.OutcomeMetrics) > 0 {
				fmt.Printf("  outcome_metrics=%s\n", strings.Join(row.OutcomeMetrics, ", "))
			}
			for _, advice := range row.CausalResetAdvice {
				fmt.Printf("  causal_reset=%s\n", advice)
			}
		}
		return nil
	})
}

func reviewPolicyReportRows(ctx context.Context, cfg config.Config, s *store.Store, profileName string) ([]reviewPolicyReportRow, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return nil, err
	}
	rows := map[string]*reviewPolicyReportRow{}
	for _, task := range tasks {
		detailTask, _, evidence, _, reviews, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return nil, err
		}
		eval, err := reviewPolicyEvaluation(ctx, cfg, s, detailTask, reviews, nil)
		if err != nil {
			return nil, err
		}
		if eval.Profile == "" {
			continue
		}
		if strings.TrimSpace(profileName) != "" && eval.Profile != profileName {
			continue
		}
		row := rows[eval.Profile]
		if row == nil {
			row = &reviewPolicyReportRow{
				Profile:           eval.Profile,
				Mode:              firstNonEmpty(eval.Mode, "blocking"),
				ProcessHypothesis: eval.ProcessHypothesis,
				OutcomeMetrics:    append([]string{}, eval.OutcomeMetrics...),
			}
			rows[eval.Profile] = row
		}
		row.TaskCount++
		row.MissingReviews += len(eval.MissingReviewDomains)
		for _, req := range eval.Requirements {
			switch req.Status {
			case "required":
				row.RequiredReviewDomains++
				if !containsString(eval.MissingReviewDomains, req.Domain) {
					row.ApprovedReviews++
				}
			case "inherited":
				row.InheritedReviews++
			case "waived":
				row.WaivedReviews++
			case "deferred":
				row.DeferredReviews++
			}
		}
		if detailTask.Status == "blocked" {
			row.BlockedTasks++
		}
		if detailTask.Status == "done" {
			row.CompletedTasks++
		}
		for _, ev := range evidence {
			switch strings.TrimSpace(ev.Result) {
			case "fail":
				row.DefectsCaught++
			case "partial":
				row.ReworkReducedSignals++
			case "blocked":
				row.AvoidedUnsafeActions++
			}
		}
		loop := reviewpolicy.DetectLoop(detailTask, eval, evidence, reviews)
		if loop.Detected {
			row.LoopDetected++
			row.CausalResetAdvice = append(row.CausalResetAdvice, reviewPolicyLoopAdvice(detailTask.Definition.ID, loop))
		}
	}
	var out []reviewPolicyReportRow
	for _, row := range rows {
		row.Recommendation = reviewPolicyReportRecommendation(*row)
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Profile < out[j].Profile })
	return out, nil
}

func reviewPolicyReportRecommendation(row reviewPolicyReportRow) string {
	if row.LoopDetected > 0 {
		return "recommend causal reset with lighter safe-boundary review before another retry"
	}
	if row.Mode == "advisory" && row.DefectsCaught+row.ReworkReducedSignals+row.AvoidedUnsafeActions == 0 && row.MissingReviews > 0 {
		return "consider removing or narrowing this process before making it blocking"
	}
	if row.Mode == "advisory" {
		return "continue pilot until speed, quality, or safety hypothesis has enough evidence"
	}
	return "keep blocking only while outcomes justify overhead"
}

func reviewPolicyLoopAdvice(taskID string, loop reviewpolicy.LoopRecommendation) string {
	parts := []string{taskID, loop.Reason}
	if len(loop.FailureChain) > 0 {
		parts = append(parts, "failure_chain="+strings.Join(loop.FailureChain, " | "))
	}
	if len(loop.RealUnknowns) > 0 {
		parts = append(parts, "real_unknowns="+strings.Join(loop.RealUnknowns, "; "))
	}
	if len(loop.RequiredProofBeforeRetry) > 0 {
		parts = append(parts, "required_proof_before_retry="+strings.Join(loop.RequiredProofBeforeRetry, "; "))
	}
	if loop.LighterReviewPlan != "" {
		parts = append(parts, "lighter_review_plan="+loop.LighterReviewPlan)
	}
	return strings.Join(parts, "; ")
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cmdReviewWaitsList(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("review-waits", "list [--blocking] [--task <task-id>] [--stale]")
		return nil
	}
	fs := flag.NewFlagSet("review-waits list", flag.ContinueOnError)
	blockingOnly := fs.Bool("blocking", false, "show only blocking waits")
	taskID := fs.String("task", "", "task id")
	staleOnly := fs.Bool("stale", false, "show only stale waits")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected review-waits list arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		waits, err := reviewWaitRows(ctx, cfg, s, *taskID)
		if err != nil {
			return err
		}
		var filtered []reviewstate.ReviewWait
		for _, wait := range waits {
			if *blockingOnly && !wait.Blocking {
				continue
			}
			if *staleOnly && wait.State != "stale" {
				continue
			}
			filtered = append(filtered, wait)
		}
		if opts.JSON {
			return printJSON(filtered)
		}
		if len(filtered) == 0 {
			fmt.Println("review_waits: none")
			return nil
		}
		fmt.Println("review_waits:")
		for _, wait := range filtered {
			fmt.Printf("- %s domain=%s state=%s blocking=%t action=%s policy=%s profile=%s target=%s:%s expected_response_at=%s reason=%s\n",
				wait.TaskID,
				wait.Domain,
				wait.State,
				wait.Blocking,
				wait.Action,
				firstNonEmpty(wait.PolicyStatus, "required"),
				firstNonEmpty(wait.ReviewProfile, "task-review-domains"),
				firstNonEmpty(wait.TargetProvider, "none"),
				firstNonEmpty(wait.TargetID, "none"),
				firstNonEmpty(wait.ExpectedResponseAt, "none"),
				wait.Reason,
			)
		}
		return nil
	})
}

type reviewWaitWake struct {
	TaskID       string                   `json:"task_id"`
	TaskStatus   string                   `json:"task_status,omitempty"`
	Kind         string                   `json:"kind"`
	Provider     string                   `json:"provider,omitempty"`
	Target       string                   `json:"target,omitempty"`
	TargetStatus string                   `json:"target_status,omitempty"`
	TargetAction string                   `json:"target_action,omitempty"`
	TargetReason string                   `json:"target_reason,omitempty"`
	State        string                   `json:"state,omitempty"`
	Prompt       string                   `json:"prompt"`
	Waits        []reviewstate.ReviewWait `json:"waits"`
	Signature    string                   `json:"signature"`
	ReviewOnly   bool                     `json:"review_only,omitempty"`
	Suppressed   bool                     `json:"suppressed,omitempty"`
	Error        string                   `json:"error,omitempty"`
}

func cmdReviewWaitsWake(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("review-waits", "wake [--task <task-id>] [--send] [--state <sent|notification_delivered|thread_steered>] [--provider <name>] [--target <thread-id>]")
		return nil
	}
	fs := flag.NewFlagSet("review-waits wake", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	send := fs.Bool("send", false, "record bounded wake delivery/failure notification rows")
	state := fs.String("state", "sent", "notification state to record with --send")
	provider := fs.String("provider", "", "override provider label")
	target := fs.String("target", "", "override provider thread/adapter target")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected review-waits wake arguments: %s", strings.Join(fs.Args(), " "))
	}
	if !*send && (*provider != "" || *target != "" || *state != "sent") {
		return errors.New("--provider, --target, and --state require --send")
	}
	switch *state {
	case "sent", "notification_delivered", "thread_steered":
	default:
		return fmt.Errorf("invalid review-waits wake --state %q", *state)
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		waits, statusByTask, err := reviewWaitRowsWithTaskStatus(ctx, cfg, s, *taskID)
		if err != nil {
			return err
		}
		wakes := selectReviewWaitWakes(waits, statusByTask, terminalStatusSet(cfg.States.Terminal))
		for i := range wakes {
			if *provider != "" {
				wakes[i].Provider = *provider
			}
			if *target != "" {
				wakes[i].Target = *target
			}
			wakes[i].State = *state
			notifications, err := s.Notifications(ctx, wakes[i].TaskID)
			if err != nil {
				return err
			}
			if reviewWaitWakeSuppressed(wakes[i], notifications) {
				wakes[i].Suppressed = true
				continue
			}
			if !*send {
				continue
			}
			recordState := *state
			reason := "review_wait_wake signature=" + wakes[i].Signature + " kind=" + wakes[i].Kind
			if !wakeTargetRoutable(wakes[i].TargetStatus, wakes[i].Target) {
				recordState = "notification_failed"
				reason += " failed=no_wake_target action=mapping_required"
				if wakes[i].TargetReason != "" {
					reason += " target_reason=" + strings.ReplaceAll(wakes[i].TargetReason, " ", "_")
				}
				wakes[i].TargetAction = "mapping_required"
				wakes[i].Error = firstNonEmpty(wakes[i].TargetReason, "no wake target configured")
			}
			if _, err := s.RecordNotification(ctx, store.Notification{
				TaskID:   wakes[i].TaskID,
				Domain:   "coordinator",
				Provider: wakes[i].Provider,
				Target:   wakes[i].Target,
				State:    recordState,
				Reason:   reason,
			}); err != nil {
				return err
			}
			wakes[i].State = recordState
		}
		if opts.JSON {
			return printJSON(wakes)
		}
		if len(wakes) == 0 {
			fmt.Println("review_wait_wakes: none")
			return nil
		}
		fmt.Println("review_wait_wakes:")
		for _, wake := range wakes {
			status := "ready"
			if wake.Suppressed {
				status = "suppressed"
			}
			if wake.Error != "" {
				status = "failed"
			}
			reviewOnly := ""
			if wake.ReviewOnly {
				reviewOnly = " review_only=true"
			}
			targetNote := ""
			if wake.TargetAction != "" {
				targetNote = " target_action=" + wake.TargetAction
			}
			fmt.Printf("- %s kind=%s status=%s task_status=%s provider=%s target=%s signature=%s%s%s\n", wake.TaskID, wake.Kind, status, firstNonEmpty(wake.TaskStatus, "unknown"), firstNonEmpty(wake.Provider, "none"), firstNonEmpty(wake.Target, "none"), wake.Signature, reviewOnly, targetNote)
			fmt.Print(wake.Prompt)
			if !strings.HasSuffix(wake.Prompt, "\n") {
				fmt.Println()
			}
		}
		return nil
	})
}

func reviewWaitRows(ctx context.Context, cfg config.Config, s *store.Store, taskID string) ([]reviewstate.ReviewWait, error) {
	waits, _, err := reviewWaitRowsWithTaskStatus(ctx, cfg, s, taskID)
	return waits, err
}

func reviewWaitRowsWithTaskStatus(ctx context.Context, cfg config.Config, s *store.Store, taskID string) ([]reviewstate.ReviewWait, map[string]string, error) {
	ackTimeout, err := reviewWaitAckTimeout(cfg)
	if err != nil {
		return nil, nil, err
	}
	waitOpts := reviewWaitOptions(cfg)
	waitOpts.AckTimeout = ackTimeout
	waitOpts.Now = time.Now().UTC()
	waitOpts.Terminal = cfg.States.Terminal
	var tasks []store.Task
	if strings.TrimSpace(taskID) != "" {
		task, _, _, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return nil, nil, err
		}
		tasks = []store.Task{task}
	} else {
		tasks, err = s.AllTasks(ctx)
		if err != nil {
			return nil, nil, err
		}
	}
	var waits []reviewstate.ReviewWait
	statusByTask := map[string]string{}
	for _, task := range tasks {
		statusByTask[task.Definition.ID] = task.Status
		detailTask, _, _, handoffs, reviews, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return nil, nil, err
		}
		policy, err := reviewPolicyEvaluation(ctx, cfg, s, detailTask, reviews, nil)
		if err != nil {
			return nil, nil, err
		}
		detailTask.Definition.ReviewDomains = policy.EffectiveDomains
		notifications, err := s.Notifications(ctx, task.Definition.ID)
		if err != nil {
			return nil, nil, err
		}
		taskWaits := reviewstate.WaitsForTask(detailTask, handoffs, reviews, notifications, waitOpts)
		for i := range taskWaits {
			taskWaits[i].ReviewProfile = policy.Profile
			taskWaits[i].PolicyStatus = "required"
			taskWaits[i].PolicyReason = reviewPolicyDomainReason(policy, taskWaits[i].Domain)
			if policy.Mode == "advisory" {
				taskWaits[i].Blocking = false
				taskWaits[i].Reason = "advisory review profile: " + taskWaits[i].Reason
			}
		}
		waits = append(waits, taskWaits...)
		waits = append(waits, reviewPolicyWaitRows(detailTask, policy)...)
	}
	return waits, statusByTask, nil
}

func reviewPolicyWaitRows(task store.Task, policy reviewpolicy.Evaluation) []reviewstate.ReviewWait {
	var rows []reviewstate.ReviewWait
	for _, req := range policy.Requirements {
		switch req.Status {
		case "inherited", "waived", "deferred", "cancelled":
		default:
			continue
		}
		state := "resolved"
		action := "none"
		blocking := false
		if req.Status == "deferred" {
			state = "cancelled"
			action = "deferred_review"
		} else if req.Status == "cancelled" {
			state = "cancelled"
		}
		rows = append(rows, reviewstate.ReviewWait{
			WaitID:        task.Definition.ID + "/" + req.Domain,
			TaskID:        task.Definition.ID,
			Domain:        req.Domain,
			State:         state,
			Blocking:      blocking,
			Reason:        req.Reason,
			Action:        action,
			ReviewProfile: policy.Profile,
			PolicyStatus:  req.Status,
			PolicyReason:  req.Reason,
		})
	}
	return rows
}

func selectReviewWaitWakes(waits []reviewstate.ReviewWait, statusByTask map[string]string, terminal map[string]bool) []reviewWaitWake {
	byTask := map[string][]reviewstate.ReviewWait{}
	for _, wait := range waits {
		byTask[wait.TaskID] = append(byTask[wait.TaskID], wait)
	}
	var taskIDs []string
	for taskID := range byTask {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	var wakes []reviewWaitWake
	for _, taskID := range taskIDs {
		taskStatus := strings.TrimSpace(statusByTask[taskID])
		if terminal[taskStatus] {
			continue
		}
		taskWaits := byTask[taskID]
		sort.SliceStable(taskWaits, func(i, j int) bool { return taskWaits[i].WaitID < taskWaits[j].WaitID })
		resolved := len(taskWaits) > 0
		for _, wait := range taskWaits {
			if wait.Blocking || wait.State != "resolved" {
				resolved = false
			}
			if wait.Blocking && (wait.State == "stale" || wait.State == "notification_failed") {
				wakes = append(wakes, buildReviewWaitWake(taskID, taskStatus, wait.State, taskWaits, wait))
			}
		}
		if resolved {
			wakes = append(wakes, buildReviewWaitWake(taskID, taskStatus, "resolved", taskWaits, firstWakeTarget(taskWaits)))
		}
	}
	return wakes
}

func terminalStatusSet(statuses []string) map[string]bool {
	out := map[string]bool{}
	for _, status := range statuses {
		out[strings.TrimSpace(status)] = true
	}
	return out
}

func buildReviewWaitWake(taskID, taskStatus, kind string, waits []reviewstate.ReviewWait, target reviewstate.ReviewWait) reviewWaitWake {
	reviewOnly := kind == "resolved" && taskStatus != "review"
	targetStatus := "routable"
	targetAction := ""
	targetReason := ""
	if strings.TrimSpace(firstNonEmpty(target.WakeThreadID, target.TargetID)) == "" {
		targetStatus = "notification_failed"
		targetAction = "mapping_required"
		targetReason = "no wake target configured for review wait domain " + strings.TrimSpace(target.Domain)
	}
	signature := reviewWaitWakeSignature(taskID, taskStatus, kind, reviewOnly, waits)
	return reviewWaitWake{
		TaskID:       taskID,
		TaskStatus:   taskStatus,
		Kind:         kind,
		Provider:     target.TargetProvider,
		Target:       firstNonEmpty(target.WakeThreadID, target.TargetID),
		TargetStatus: targetStatus,
		TargetAction: targetAction,
		TargetReason: targetReason,
		Prompt:       renderReviewWaitWakePrompt(taskID, taskStatus, kind, reviewOnly, waits, targetReason),
		Waits:        waits,
		Signature:    signature,
		ReviewOnly:   reviewOnly,
	}
}

func firstWakeTarget(waits []reviewstate.ReviewWait) reviewstate.ReviewWait {
	for _, wait := range waits {
		if wait.WakeThreadID != "" {
			return wait
		}
	}
	if len(waits) > 0 {
		return waits[0]
	}
	return reviewstate.ReviewWait{}
}

func reviewWaitWakeSignature(taskID, taskStatus, kind string, reviewOnly bool, waits []reviewstate.ReviewWait) string {
	parts := []string{taskID, kind, "task_status=" + taskStatus}
	if reviewOnly {
		parts = append(parts, "review_only=true")
	}
	for _, wait := range waits {
		parts = append(parts, wait.WaitID+"="+wait.State+"/"+wait.Action+"/"+wait.ExpectedResponseAt+"/"+wait.ResolvedAt)
	}
	return strings.Join(parts, "|")
}

func renderReviewWaitWakePrompt(taskID, taskStatus, kind string, reviewOnly bool, waits []reviewstate.ReviewWait, targetReason string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Review wait update for %s:\n", taskID)
	if taskStatus != "" {
		fmt.Fprintf(&b, "Task status: %s\n", taskStatus)
	}
	if targetReason != "" {
		fmt.Fprintf(&b, "Wake target: mapping_required reason=%s\n", targetReason)
	}
	for _, wait := range waits {
		fmt.Fprintf(&b, "- %s: %s", wait.Domain, wait.State)
		if wait.Action != "" {
			fmt.Fprintf(&b, " action=%s", wait.Action)
		}
		if wait.ExpectedResponseAt != "" {
			fmt.Fprintf(&b, " expected_response_at=%s", wait.ExpectedResponseAt)
		}
		if wait.Reason != "" {
			fmt.Fprintf(&b, " reason=%s", wait.Reason)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nNext action:\n")
	fmt.Fprintf(&b, "1. Re-run fairway review-waits list --task %s.\n", taskID)
	switch {
	case kind == "resolved" && reviewOnly:
		fmt.Fprintf(&b, "2. Treat this as review-wait-only; task status is %s, so review resolution does not authorize merge-ready or closeout.\n", firstNonEmpty(taskStatus, "unknown"))
		b.WriteString("3. Resolve the task status and task-level gates before reviewed-lane closeout.\n")
	case kind == "resolved":
		fmt.Fprintf(&b, "2. Re-run fairway merge-ready %s.\n", taskID)
		b.WriteString("3. If gates pass, continue reviewed-lane closeout.\n")
	default:
		b.WriteString("2. Address the blocking review wait before merge-ready or closeout.\n")
		b.WriteString("3. Re-run this wake or the list command after notification or mapping state changes.\n")
	}
	return b.String()
}

func reviewWaitWakeSuppressed(wake reviewWaitWake, notifications []store.Notification) bool {
	needle := "review_wait_wake signature=" + wake.Signature
	for _, notification := range notifications {
		if notification.Domain == "coordinator" && strings.Contains(notification.Reason, needle) {
			switch notification.State {
			case "sent", "notification_delivered", "thread_steered", "acknowledged", "review_acknowledged":
				return true
			}
		}
	}
	return false
}

type completionHandbackWakeRow struct {
	TaskID           string                       `json:"task_id"`
	TaskStatus       string                       `json:"task_status,omitempty"`
	Kind             string                       `json:"kind"`
	Provider         string                       `json:"provider,omitempty"`
	Target           string                       `json:"target,omitempty"`
	TargetStatus     string                       `json:"target_status,omitempty"`
	TargetAction     string                       `json:"target_action,omitempty"`
	TargetReason     string                       `json:"target_reason,omitempty"`
	State            string                       `json:"state,omitempty"`
	Prompt           string                       `json:"prompt"`
	Signature        string                       `json:"signature"`
	Handback         *completionhandback.Handback `json:"completion_handback,omitempty"`
	LiveWindow       *livewindow.Status           `json:"live_window,omitempty"`
	SuggestedAction  string                       `json:"suggested_action,omitempty"`
	SuggestedCommand string                       `json:"suggested_command,omitempty"`
	Suppressed       bool                         `json:"suppressed,omitempty"`
	Error            string                       `json:"error,omitempty"`
}

func selectCompletionHandbackWakes(plan coord.Plan, cfg config.Config, taskStatus map[string]string, terminal map[string]bool, onlyTaskID string) []completionHandbackWakeRow {
	var wakes []completionHandbackWakeRow
	for _, action := range plan.Actions {
		if action.Classification != "completion-handback" {
			continue
		}
		if strings.TrimSpace(onlyTaskID) != "" && action.TaskID != onlyTaskID {
			continue
		}
		status := strings.TrimSpace(taskStatus[action.TaskID])
		if terminal[status] {
			continue
		}
		switch action.Action {
		case "escalate_completion_handback":
			if action.CompletionHandback == nil || !action.CompletionHandback.Stale {
				continue
			}
			wakes = append(wakes, buildCompletionHandbackWake(action, cfg, status, "stale-handback"))
		case "escalate_closeout_completion_handback":
			if action.LiveWindow == nil {
				continue
			}
			wakes = append(wakes, buildCompletionHandbackWake(action, cfg, status, "stale-closeout"))
		}
	}
	sort.SliceStable(wakes, func(i, j int) bool {
		if wakes[i].TaskID != wakes[j].TaskID {
			return wakes[i].TaskID < wakes[j].TaskID
		}
		return wakes[i].Signature < wakes[j].Signature
	})
	return wakes
}

func buildCompletionHandbackWake(action coord.PlanAction, cfg config.Config, taskStatus, kind string) completionHandbackWakeRow {
	targetInfo := resolveWakeTarget(cfg.ProviderTargets, action.Role)
	row := completionHandbackWakeRow{
		TaskID:       action.TaskID,
		TaskStatus:   taskStatus,
		Kind:         kind,
		Provider:     targetInfo.Provider,
		Target:       targetInfo.Target,
		TargetStatus: targetInfo.Status,
		TargetAction: targetInfo.Action,
		TargetReason: targetInfo.Reason,
		State:        "sent",
	}
	if action.CompletionHandback != nil {
		handback := *action.CompletionHandback
		row.Handback = &handback
		row.SuggestedAction = firstNonEmpty(handback.SuggestedAction, action.Action)
		row.SuggestedCommand = handback.SuggestedCommand
		row.Signature = completionHandbackWakeSignature(row.TaskID, taskStatus, kind, handback, nil)
		row.Prompt = renderCompletionHandbackWakePrompt(row, action.Reason)
		return row
	}
	if action.LiveWindow != nil {
		liveWindow := *action.LiveWindow
		row.LiveWindow = &liveWindow
		row.SuggestedAction = action.Action
		row.SuggestedCommand = fmt.Sprintf("fairway record completion-handback %s --to %s --next-action %q --completion-state live-window-closeout --state thread_steered --provider <provider> --target <target>", row.TaskID, firstNonEmpty(action.Role, "<role>"), firstNonEmpty(liveWindow.NextAction, "record next decision"))
		row.Signature = completionHandbackWakeSignature(row.TaskID, taskStatus, kind, completionhandback.Handback{}, &liveWindow)
		row.Prompt = renderCompletionHandbackWakePrompt(row, action.Reason)
	}
	return row
}

func completionWakeTarget(targets []config.ProviderTarget, role string) (string, string) {
	target := resolveWakeTarget(targets, role)
	return target.Provider, target.Target
}

type wakeTargetResolution struct {
	Role     string
	Provider string
	Target   string
	Type     string
	Status   string
	Action   string
	Reason   string
}

func resolveWakeTarget(targets []config.ProviderTarget, role string) wakeTargetResolution {
	role = strings.TrimSpace(role)
	resolution := wakeTargetResolution{Role: role, Status: "routable"}
	if role == "" {
		resolution.Status = "notification_failed"
		resolution.Action = "mapping_required"
		resolution.Reason = "wake owner is empty"
		return resolution
	}
	for _, target := range targets {
		if strings.TrimSpace(target.Domain) != role {
			continue
		}
		resolution.Provider = strings.TrimSpace(target.Provider)
		resolution.Target = strings.TrimSpace(target.Target)
		resolution.Type = strings.TrimSpace(target.Type)
		switch {
		case resolution.Provider == "":
			resolution.Status = "notification_failed"
			resolution.Action = "mapping_required"
			resolution.Reason = fmt.Sprintf("provider target for %s has no provider", role)
		case resolution.Target == "":
			resolution.Status = "notification_failed"
			resolution.Action = "mapping_required"
			resolution.Reason = fmt.Sprintf("provider target for %s has no target", role)
		}
		return resolution
	}
	resolution.Status = "notification_failed"
	resolution.Action = "mapping_required"
	resolution.Reason = fmt.Sprintf("no provider target configured for %s", role)
	return resolution
}

func wakeTargetRoutable(status, target string) bool {
	status = strings.TrimSpace(status)
	if status != "" && status != "routable" {
		return false
	}
	return strings.TrimSpace(target) != ""
}

func completionHandbackWakeSignature(taskID, taskStatus, kind string, handback completionhandback.Handback, liveWindow *livewindow.Status) string {
	parts := []string{taskID, kind, "task_status=" + strings.TrimSpace(taskStatus)}
	if liveWindow != nil {
		parts = append(parts,
			"phase="+strings.TrimSpace(liveWindow.Phase),
			"checkpoint_at="+strings.TrimSpace(liveWindow.CheckpointAt),
			"next_owner="+strings.TrimSpace(liveWindow.NextOwner),
			"next_action="+strings.TrimSpace(liveWindow.NextAction),
		)
		return strings.Join(parts, "|")
	}
	parts = append(parts,
		fmt.Sprintf("handoff_id=%d", handback.HandoffID),
		"to="+strings.TrimSpace(handback.ToRole),
		"completion_state="+strings.TrimSpace(handback.CompletionState),
		"delivery_state="+strings.TrimSpace(handback.DeliveryState),
		"created_at="+strings.TrimSpace(handback.CreatedAt),
		"next_action="+strings.TrimSpace(handback.NextAction),
	)
	return strings.Join(parts, "|")
}

func renderCompletionHandbackWakePrompt(wake completionHandbackWakeRow, reason string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Completion handback wake for %s:\n", wake.TaskID)
	if wake.TaskStatus != "" {
		fmt.Fprintf(&b, "Task status: %s\n", wake.TaskStatus)
	}
	switch {
	case wake.Handback != nil:
		fmt.Fprintf(&b, "Kind: stale completion handback\n")
		fmt.Fprintf(&b, "Handoff: %d to %s\n", wake.Handback.HandoffID, wake.Handback.ToRole)
		fmt.Fprintf(&b, "Completion state: %s\n", firstNonEmpty(wake.Handback.CompletionState, "unspecified"))
		fmt.Fprintf(&b, "Delivery status: %s", wake.Handback.DeliveryStatus)
		if wake.Handback.StaleAge != "" {
			fmt.Fprintf(&b, " stale_age=%s", wake.Handback.StaleAge)
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "Next action: %s\n", wake.Handback.NextAction)
	case wake.LiveWindow != nil:
		fmt.Fprintf(&b, "Kind: stale live-window closeout\n")
		fmt.Fprintf(&b, "Live-window phase: %s\n", wake.LiveWindow.Phase)
		fmt.Fprintf(&b, "Next owner: %s\n", firstNonEmpty(wake.LiveWindow.NextOwner, "unknown"))
		fmt.Fprintf(&b, "Next action: %s\n", firstNonEmpty(wake.LiveWindow.NextAction, "record next decision"))
	}
	if reason != "" {
		fmt.Fprintf(&b, "Reason: %s\n", reason)
	}
	if wake.TargetReason != "" {
		fmt.Fprintf(&b, "Wake target: mapping_required reason=%s\n", wake.TargetReason)
	}
	b.WriteString("\nNext action:\n")
	fmt.Fprintf(&b, "1. Re-run fairway coordinator tick --completion-handback-wake --task %s.\n", wake.TaskID)
	if wake.SuggestedCommand != "" {
		fmt.Fprintf(&b, "2. Run or adapt: %s.\n", wake.SuggestedCommand)
	} else {
		b.WriteString("2. Record delivered or failed completion handback notification proof.\n")
	}
	b.WriteString("3. Do not treat this wake as approval, merge, deploy, or dashboard send authority.\n")
	return b.String()
}

func completionHandbackWakeSuppressed(wake completionHandbackWakeRow, notifications []store.Notification) bool {
	needle := "completion_handback_wake signature=" + wake.Signature
	for _, notification := range notifications {
		if notification.Domain == "coordinator" && strings.Contains(notification.Reason, needle) {
			switch notification.State {
			case "sent", "notification_delivered", "thread_steered", "acknowledged", "review_acknowledged":
				return true
			}
		}
	}
	return false
}

func printCompletionHandbackWakes(wakes []completionHandbackWakeRow) {
	if len(wakes) == 0 {
		fmt.Println("completion_handback_wakes: none")
		return
	}
	fmt.Println("completion_handback_wakes:")
	for _, wake := range wakes {
		status := "ready"
		if wake.Suppressed {
			status = "suppressed"
		}
		if wake.Error != "" {
			status = "failed"
		}
		targetNote := ""
		if wake.TargetAction != "" {
			targetNote = " target_action=" + wake.TargetAction
		}
		fmt.Printf("- %s kind=%s status=%s task_status=%s provider=%s target=%s signature=%s%s\n", wake.TaskID, wake.Kind, status, firstNonEmpty(wake.TaskStatus, "unknown"), firstNonEmpty(wake.Provider, "none"), firstNonEmpty(wake.Target, "none"), wake.Signature, targetNote)
		fmt.Print(wake.Prompt)
		if !strings.HasSuffix(wake.Prompt, "\n") {
			fmt.Println()
		}
	}
}

func taskStatuses(ctx context.Context, s *store.Store) (map[string]string, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, task := range tasks {
		out[task.Definition.ID] = task.Status
	}
	return out, nil
}

func reviewWaitOptions(cfg config.Config) reviewstate.ReviewWaitOptions {
	return reviewstate.ReviewWaitOptions{
		ProviderTargets: cfg.ProviderTargets,
		ReviewRoutes:    cfg.ReviewRoutes,
		Roles:           cfg.Roles,
		Terminal:        cfg.States.Terminal,
	}
}

func reviewWaitAckTimeout(cfg config.Config) (time.Duration, error) {
	raw := strings.TrimSpace(cfg.Coordinator.NotificationAckTimeout)
	if raw == "" {
		raw = "24h"
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid coordinator notification_ack_timeout %q: %w", raw, err)
	}
	return timeout, nil
}

func cmdLiveWindow(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("live-window", "record <task-id> --phase <phase> [--next-owner <role>] [--next-action <action>] [--target-close-by <time>] [--artifact <path>] | status [--task <task-id>] | control-room [--task <task-id>] [--stale] | retry-budget record|status ...")
		return nil
	}
	switch args[0] {
	case "record":
		return cmdLiveWindowRecord(ctx, opts, args[1:])
	case "status":
		return cmdLiveWindowStatus(ctx, opts, args[1:])
	case "control-room":
		return cmdLiveWindowControlRoom(ctx, opts, args[1:])
	case "retry-budget":
		return cmdLiveWindowRetryBudget(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown live-window subcommand %q", args[0])
	}
}

func cmdLiveWindowRecord(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("live-window", "record <task-id> --phase <phase> [--next-owner <role>] [--next-action <action>] [--authorization-state <state>] [--prompt <text>] [--command <cmd>] [--missed-deadline-action <action>] [--target-close-by <time>] [--artifact <path>]")
		return nil
	}
	if len(args) < 1 {
		return errors.New("live-window record requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("live-window record", flag.ContinueOnError)
	phase := fs.String("phase", "", "phase: "+strings.Join(livewindow.PhaseList(), ", "))
	nextOwner := fs.String("next-owner", "", "next actor/role")
	nextAction := fs.String("next-action", "", "next safe action")
	authorizationState := fs.String("authorization-state", "", "authorization state for the handoff")
	prompt := fs.String("prompt", "", "fixed prompt to deliver to the next actor")
	command := fs.String("command", "", "exact Fairway/operator command for the next actor")
	missedDeadlineAction := fs.String("missed-deadline-action", "", "action if target close-by/deadline is missed")
	targetCloseBy := fs.String("target-close-by", "", "expected closeout/window end in YYYY-MM-DD or RFC3339")
	artifact := fs.String("artifact", "", "phase artifact path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected live-window record arguments: %s", strings.Join(fs.Args(), " "))
	}
	summary, err := livewindow.SummaryWithOptions(livewindow.SummaryOptions{
		Phase:                *phase,
		NextOwner:            *nextOwner,
		NextAction:           *nextAction,
		AuthorizationState:   *authorizationState,
		Prompt:               *prompt,
		Command:              *command,
		MissedDeadlineAction: *missedDeadlineAction,
	})
	if err != nil {
		return err
	}
	state := "awaiting_input"
	if liveWindowActivePhase(*phase) {
		state = "active"
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: taskID, State: state, Owner: *nextOwner, TargetCloseBy: *targetCloseBy, Summary: summary, ArtifactPath: *artifact}); err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(struct {
				TaskID string `json:"task_id"`
				Phase  string `json:"phase"`
				State  string `json:"checkpoint_state"`
			}{taskID, *phase, state})
		}
		fmt.Printf("live_window recorded %s phase=%s state=%s\n", taskID, *phase, state)
		return nil
	})
}

func cmdLiveWindowRetryBudget(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("live-window", "retry-budget record <task-id> --meaningful-failures <n> --coordination-failures <n> --budget <n> [--reset-task <task-id>] [--reset-reason <text>] | status [--task <task-id>]")
		return nil
	}
	switch args[0] {
	case "record":
		return cmdLiveWindowRetryBudgetRecord(ctx, opts, args[1:])
	case "status":
		return cmdLiveWindowRetryBudgetStatus(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown live-window retry-budget subcommand %q", args[0])
	}
}

func cmdLiveWindowRetryBudgetRecord(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("live-window", "retry-budget record <task-id> --meaningful-failures <n> --coordination-failures <n> --budget <n> [--reset-task <task-id>] [--reset-reason <text>]")
		return nil
	}
	if len(args) < 1 {
		return errors.New("live-window retry-budget record requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("live-window retry-budget record", flag.ContinueOnError)
	meaningfulFailures := fs.Int("meaningful-failures", 0, "meaningful live-operation failures that count against the retry budget")
	coordinationFailures := fs.Int("coordination-failures", 0, "coordination-only failures that do not count against the retry budget")
	budget := fs.Int("budget", 3, "meaningful failure budget before causal reset is required")
	resetTask := fs.String("reset-task", "", "causal reset task that clears an exhausted retry budget")
	resetReason := fs.String("reset-reason", "", "reason or proof for the causal reset")
	artifact := fs.String("artifact", "", "retry budget artifact path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected live-window retry-budget record arguments: %s", strings.Join(fs.Args(), " "))
	}
	summary, err := livewindow.RetryBudgetSummary(*meaningfulFailures, *coordinationFailures, *budget, *resetTask, *resetReason)
	if err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if strings.TrimSpace(*resetTask) != "" {
			if strings.TrimSpace(*resetReason) == "" {
				return errors.New("live-window retry-budget reset clearance requires --reset-reason")
			}
			if _, _, _, _, _, err := s.TaskDetail(ctx, strings.TrimSpace(*resetTask)); err != nil {
				return fmt.Errorf("live-window retry-budget reset task %q not found: %w", *resetTask, err)
			}
		}
		if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: taskID, State: "awaiting_input", Owner: "backend", Summary: summary, ArtifactPath: *artifact}); err != nil {
			return err
		}
		checkpoints, err := s.Checkpoints(ctx, "", true)
		if err != nil {
			return err
		}
		budgetRow, _ := livewindow.RetryBudgetForTask(checkpoints, taskID)
		if opts.JSON {
			return printJSON(budgetRow)
		}
		fmt.Printf("live_window_retry_budget recorded %s meaningful_failures=%d coordination_failures=%d budget=%d requires_reset=%t\n",
			taskID, budgetRow.MeaningfulFailures, budgetRow.CoordinationFailures, budgetRow.Budget, budgetRow.RequiresReset)
		return nil
	})
}

func cmdLiveWindowRetryBudgetStatus(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("live-window", "retry-budget status [--task <task-id>]")
		return nil
	}
	fs := flag.NewFlagSet("live-window retry-budget status", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected live-window retry-budget status arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		checkpoints, err := s.Checkpoints(ctx, "", true)
		if err != nil {
			return err
		}
		rows := livewindow.RetryBudgetsFromCheckpoints(checkpoints)
		if strings.TrimSpace(*taskID) != "" {
			filtered := rows[:0]
			for _, row := range rows {
				if row.TaskID == *taskID {
					filtered = append(filtered, row)
				}
			}
			rows = filtered
		}
		if opts.JSON {
			return printJSON(rows)
		}
		fmt.Println("live_operation_retry_budgets:")
		if len(rows) == 0 {
			fmt.Println("- none")
			return nil
		}
		for _, row := range rows {
			fmt.Printf("- %s meaningful_failures=%d coordination_failures=%d budget=%d next_iteration=%d exhausted=%t requires_reset=%t",
				row.TaskID, row.MeaningfulFailures, row.CoordinationFailures, row.Budget, row.NextIteration, row.Exhausted, row.RequiresReset)
			if row.ResetTask != "" {
				fmt.Printf(" reset_task=%s", row.ResetTask)
			}
			if row.ResetReason != "" {
				fmt.Printf(" reset_reason=%s", row.ResetReason)
			}
			fmt.Println()
		}
		return nil
	})
}

type liveWindowControlRoomRow struct {
	livewindow.Status
	DeadlineState string `json:"deadline_state"`
	StaleAge      string `json:"stale_age,omitempty"`
	Suggested     string `json:"suggested_action,omitempty"`
}

func cmdLiveWindowControlRoom(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("live-window", "control-room [--task <task-id>] [--stale]")
		return nil
	}
	fs := flag.NewFlagSet("live-window control-room", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	staleOnly := fs.Bool("stale", false, "show only missed-deadline control rows")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected live-window control-room arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		checkpoints, err := s.Checkpoints(ctx, "", true)
		if err != nil {
			return err
		}
		statuses := livewindow.StatusesFromCheckpoints(checkpoints)
		now := time.Now().UTC()
		rows := make([]liveWindowControlRoomRow, 0, len(statuses))
		for _, status := range statuses {
			if strings.TrimSpace(*taskID) != "" && status.TaskID != *taskID {
				continue
			}
			row := buildLiveWindowControlRoomRow(status, now)
			if *staleOnly && row.DeadlineState != "missed" {
				continue
			}
			rows = append(rows, row)
		}
		if opts.JSON {
			return printJSON(rows)
		}
		fmt.Println("live_operation_control_room:")
		if len(rows) == 0 {
			fmt.Println("- none")
			return nil
		}
		for _, row := range rows {
			fmt.Printf("- %s phase=%s next_actor=%s deadline=%s deadline_state=%s authorization=%s action=%s\n",
				row.TaskID,
				row.Phase,
				firstNonEmpty(row.NextOwner, "unknown"),
				firstNonEmpty(row.TargetCloseBy, "none"),
				row.DeadlineState,
				firstNonEmpty(row.AuthorizationState, "unspecified"),
				firstNonEmpty(row.NextAction, "advance live-operation handoff"),
			)
			if row.Command != "" {
				fmt.Printf("  command=%s\n", row.Command)
			}
			if row.Prompt != "" {
				fmt.Printf("  prompt=%s\n", row.Prompt)
			}
			if row.MissedDeadlineAction != "" || row.Suggested != "" {
				fmt.Printf("  missed_deadline_action=%s\n", firstNonEmpty(row.MissedDeadlineAction, row.Suggested))
			}
			if row.StaleAge != "" {
				fmt.Printf("  stale_age=%s\n", row.StaleAge)
			}
		}
		return nil
	})
}

func buildLiveWindowControlRoomRow(status livewindow.Status, now time.Time) liveWindowControlRoomRow {
	row := liveWindowControlRoomRow{Status: status, DeadlineState: "open"}
	if liveWindowFinalPhase(status.Phase) {
		row.DeadlineState = "closed"
		return row
	}
	if strings.TrimSpace(status.TargetCloseBy) == "" {
		row.DeadlineState = "unbounded"
		row.Suggested = "record target close-by deadline"
		return row
	}
	deadline, err := parseFlexibleTime(status.TargetCloseBy)
	if err != nil {
		row.DeadlineState = "invalid_deadline"
		row.Suggested = "record RFC3339 target close-by deadline"
		return row
	}
	if now.After(deadline) {
		row.DeadlineState = "missed"
		row.StaleAge = roundDuration(now.Sub(deadline)).String()
		if row.MissedDeadlineAction == "" {
			row.Suggested = "escalate live-operation handoff"
		}
	}
	return row
}

func parseFlexibleTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("empty time")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q", raw)
}

func liveWindowActivePhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case "gate-running", "operator_running":
		return true
	default:
		return false
	}
}

func liveWindowFinalPhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case "done", "blocked":
		return true
	default:
		return false
	}
}

func roundDuration(d time.Duration) time.Duration {
	if d < 0 {
		d = -d
	}
	switch {
	case d >= time.Hour:
		return d.Round(time.Minute)
	case d >= time.Minute:
		return d.Round(time.Second)
	default:
		return d.Round(time.Millisecond)
	}
}

func cmdLiveWindowStatus(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("live-window", "status [--task <task-id>]")
		return nil
	}
	fs := flag.NewFlagSet("live-window status", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected live-window status arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		checkpoints, err := s.Checkpoints(ctx, "", true)
		if err != nil {
			return err
		}
		statuses := livewindow.StatusesFromCheckpoints(checkpoints)
		if strings.TrimSpace(*taskID) != "" {
			var filtered []livewindow.Status
			for _, status := range statuses {
				if status.TaskID == *taskID {
					filtered = append(filtered, status)
				}
			}
			statuses = filtered
		}
		if opts.JSON {
			return printJSON(statuses)
		}
		if len(statuses) == 0 {
			fmt.Println("live_windows: none")
			return nil
		}
		fmt.Println("live_windows:")
		for _, status := range statuses {
			fmt.Printf("- %s phase=%s next_owner=%s next_action=%s target_close_by=%s artifact=%s checkpoint_at=%s",
				status.TaskID,
				status.Phase,
				firstNonEmpty(status.NextOwner, "none"),
				firstNonEmpty(status.NextAction, "none"),
				firstNonEmpty(status.TargetCloseBy, "none"),
				firstNonEmpty(status.ArtifactPath, "none"),
				firstNonEmpty(status.CheckpointAt, "none"),
			)
			if status.AuthorizationState != "" {
				fmt.Printf(" authorization=%s", status.AuthorizationState)
			}
			if status.Command != "" {
				fmt.Printf(" command=%s", status.Command)
			}
			if status.Prompt != "" {
				fmt.Printf(" prompt=%s", status.Prompt)
			}
			if status.MissedDeadlineAction != "" {
				fmt.Printf(" missed_deadline_action=%s", status.MissedDeadlineAction)
			}
			fmt.Println()
		}
		return nil
	})
}

func cmdReviewCheckout(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("review checkout requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("review checkout", flag.ContinueOnError)
	sourceRole := fs.String("source-role", "", "source role")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected review checkout arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		task, _, _, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		roleName := *sourceRole
		if roleName == "" {
			roleName = task.Definition.Role
		}
		role, ok := findRole(cfg, roleName)
		if !ok {
			return fmt.Errorf("unknown source role %q", roleName)
		}
		sourceBranch := config.RoleBranch(role)
		reviewBranch := config.ReviewBranch(cfg, role)
		if err := fairwaygit.CheckoutReviewBranch(root, sourceBranch, reviewBranch); err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(struct {
				TaskID       string `json:"task_id"`
				SourceRole   string `json:"source_role"`
				SourceBranch string `json:"source_branch"`
				ReviewBranch string `json:"review_branch"`
			}{taskID, roleName, sourceBranch, reviewBranch})
		}
		fmt.Printf("checked out %s from %s for %s\n", reviewBranch, sourceBranch, taskID)
		return nil
	})
}

type worktreeStatus struct {
	Role       string `json:"role"`
	Branch     string `json:"branch"`
	Path       string `json:"path"`
	Registered bool   `json:"registered"`
	Exists     bool   `json:"exists"`
	Dirty      bool   `json:"dirty"`
	LastCommit string `json:"last_commit"`
}

func cmdWorktree(opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("worktree requires subcommand: setup, status, prune")
	}
	if isHelpOnly(args) {
		subcommandUsage("worktree", "setup|status|prune")
		return nil
	}
	switch args[0] {
	case "setup":
		if len(args) > 1 {
			return fmt.Errorf("unexpected worktree setup arguments: %s", strings.Join(args[1:], " "))
		}
		return cmdWorktreeSetup(opts)
	case "status":
		if len(args) > 1 {
			return fmt.Errorf("unexpected worktree status arguments: %s", strings.Join(args[1:], " "))
		}
		return cmdWorktreeStatus(opts)
	case "prune":
		fs := flag.NewFlagSet("worktree prune", flag.ContinueOnError)
		force := fs.Bool("force", false, "remove stale worktrees")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() > 0 {
			return fmt.Errorf("unexpected worktree prune arguments: %s", strings.Join(fs.Args(), " "))
		}
		return cmdWorktreePrune(opts, *force)
	default:
		return fmt.Errorf("unknown worktree subcommand %q", args[0])
	}
}

func cmdWorktreeSetup(opts globalOptions) error {
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	if len(cfg.Roles) == 0 {
		return errors.New("no roles configured")
	}
	for _, role := range cfg.Roles {
		branch := config.RoleBranch(role)
		wtPath := config.WorktreePath(cfg, root, role)
		if err := fairwaygit.EnsureWorktree(root, cfg.Fairway.MainBranch, branch, wtPath); err != nil {
			return fmt.Errorf("setup %s: %w", role.Name, err)
		}
		fmt.Printf("%s\t%s\t%s\n", role.Name, branch, wtPath)
	}
	return nil
}

func cmdWorktreeStatus(opts globalOptions) error {
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	statuses, err := collectWorktreeStatus(cfg, root)
	if err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(statuses)
	}
	for _, status := range statuses {
		fmt.Printf("%s\t%s\tregistered=%t\texists=%t\tdirty=%t\t%s\t%s\n",
			status.Role, status.Branch, status.Registered, status.Exists, status.Dirty, status.LastCommit, status.Path)
	}
	return nil
}

func cmdWorktreePrune(opts globalOptions, force bool) error {
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	expected := map[string]bool{}
	for _, role := range cfg.Roles {
		expected[config.WorktreePath(cfg, root, role)] = true
	}
	wtRoot := config.WorktreeRoot(cfg, root)
	worktrees, err := fairwaygit.Worktrees(root)
	if err != nil {
		return err
	}
	var stale []string
	for _, wt := range worktrees {
		if wt.Path == root || !strings.HasPrefix(wt.Path, wtRoot+string(filepath.Separator)) || expected[wt.Path] {
			continue
		}
		stale = append(stale, wt.Path)
	}
	if len(stale) == 0 {
		fmt.Println("no stale worktrees")
		return nil
	}
	for _, path := range stale {
		if !force {
			fmt.Println("would prune", path)
			continue
		}
		if err := fairwaygit.RemoveWorktree(root, path, true); err != nil {
			return err
		}
		fmt.Println("pruned", path)
	}
	if !force {
		return errors.New("pass --force to prune stale worktrees")
	}
	return nil
}

func collectWorktreeStatus(cfg config.Config, root string) ([]worktreeStatus, error) {
	registered := map[string]bool{}
	worktrees, err := fairwaygit.Worktrees(root)
	if err != nil {
		return nil, err
	}
	for _, wt := range worktrees {
		registered[wt.Path] = true
	}
	statuses := make([]worktreeStatus, 0, len(cfg.Roles))
	for _, role := range cfg.Roles {
		branch := config.RoleBranch(role)
		wtPath := config.WorktreePath(cfg, root, role)
		status := worktreeStatus{
			Role:       role.Name,
			Branch:     branch,
			Path:       wtPath,
			Registered: registered[wtPath],
		}
		if _, err := os.Stat(wtPath); err == nil {
			status.Exists = true
			status.LastCommit = fairwaygit.LastCommit(wtPath)
			gitStatus, err := fairwaygit.Check(wtPath, "")
			if err == nil {
				status.Dirty = gitStatus.Dirty
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func cmdSession(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("session requires subcommand: upsert, status, end, reconcile, launch")
	}
	if isHelpOnly(args) {
		subcommandUsage("session", "upsert|status|end|reconcile|launch")
		return nil
	}
	switch args[0] {
	case "upsert":
		return cmdSessionUpsert(ctx, opts, args[1:])
	case "status":
		return cmdSessionStatus(ctx, opts, args[1:])
	case "end":
		return cmdSessionEnd(ctx, opts, args[1:])
	case "reconcile":
		return cmdSessionReconcile(ctx, opts, args[1:])
	case "launch":
		return cmdSessionLaunch(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown session subcommand %q", args[0])
	}
}

func cmdSessionUpsert(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("session upsert", flag.ContinueOnError)
	id := fs.String("id", "", "session id")
	role := fs.String("role", "", "role")
	lane := fs.String("lane", "", "lane")
	backend := fs.String("backend", "", "session backend")
	provider := fs.String("provider", "", "provider")
	name := fs.String("name", "", "session name")
	taskID := fs.String("task-id", "", "task id")
	pid := fs.Int("pid", -1, "process id")
	worktreePath := fs.String("worktree", "", "worktree path")
	branch := fs.String("branch", "", "branch")
	tmuxPane := fs.String("tmux-pane", "", "tmux pane")
	transcript := fs.String("transcript", "", "transcript path")
	monitorKind := fs.String("monitor-kind", "", "monitor kind for watcher/CI/deploy/smoke sessions")
	automationID := fs.String("automation-id", "", "backing automation id for monitor sessions")
	externalRunID := fs.String("external-run-id", "", "external run id for monitor sessions")
	pollCommand := fs.String("poll-command", "", "poll command that proves or refreshes external monitor state")
	manualUntil := fs.String("manual-until", "", "manual monitor checkpoint expiry in YYYY-MM-DD or RFC3339")
	status := fs.String("status", "running", "session status")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected session upsert arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		sessionRole := *role
		if sessionRole == "" {
			sessionRole = resolveRole(opts)
		}
		if sessionRole == "" {
			return errors.New("session role is required")
		}
		roleCfg, ok := findRole(cfg, sessionRole)
		if ok {
			if *provider == "" {
				*provider = roleCfg.Provider
			}
			if *branch == "" {
				*branch = config.RoleBranch(roleCfg)
			}
			if *worktreePath == "" {
				*worktreePath = config.WorktreePath(cfg, root, roleCfg)
			}
		}
		if *backend == "" {
			*backend = cfg.Sessions.DefaultBackend
		}
		if *branch == "" {
			*branch = fairwaygit.CurrentBranch(root)
		}
		if *worktreePath == "" {
			*worktreePath = root
		}
		sessionID := *id
		if sessionID == "" {
			sessionID = generatedSessionID(sessionRole, *pid)
		}
		session := store.Session{
			ID:             sessionID,
			Role:           sessionRole,
			Lane:           *lane,
			WorktreePath:   *worktreePath,
			Branch:         *branch,
			SessionBackend: *backend,
			Provider:       *provider,
			SessionName:    *name,
			TaskID:         *taskID,
			TmuxPane:       *tmuxPane,
			TranscriptPath: *transcript,
			MonitorKind:    *monitorKind,
			AutomationID:   *automationID,
			ExternalRunID:  *externalRunID,
			PollCommand:    *pollCommand,
			ManualUntil:    *manualUntil,
			Status:         *status,
		}
		if *pid >= 0 {
			session.PID = pid
		}
		if err := s.UpsertSession(ctx, session); err != nil {
			return err
		}
		if opts.JSON {
			session.ID = sessionID
			return printJSON(session)
		}
		fmt.Println("session", sessionID)
		return nil
	})
}

func cmdSessionStatus(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("session status", flag.ContinueOnError)
	all := fs.Bool("all", false, "include ended sessions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected session status arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		sessions, err := s.Sessions(ctx, *all)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(sessions)
		}
		for _, session := range sessions {
			fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n", session.ID, session.Role, session.Status, session.Branch, session.TaskID, session.WorktreePath)
		}
		return nil
	})
}

func cmdSessionEnd(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("session end requires session id")
	}
	fs := flag.NewFlagSet("session end", flag.ContinueOnError)
	status := fs.String("status", "ended", "terminal status")
	reason := fs.String("reason", "normal", "end reason")
	exitCode := fs.Int("exit-code", -1, "exit code")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected session end arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		var code *int
		if *exitCode >= 0 {
			code = exitCode
		}
		if err := s.EndSession(ctx, args[0], *status, *reason, code); err != nil {
			return err
		}
		fmt.Println("ended", args[0])
		return nil
	})
}

type releaseVerifyReport struct {
	OK                 bool                 `json:"ok"`
	Version            string               `json:"version"`
	Tag                string               `json:"tag"`
	SourceSHA          string               `json:"source_sha,omitempty"`
	ReleaseState       string               `json:"release_state,omitempty"`
	ReleaseURL         string               `json:"release_url,omitempty"`
	HomebrewVersion    string               `json:"homebrew_version,omitempty"`
	HomebrewTapCommit  string               `json:"homebrew_tap_commit,omitempty"`
	ProvenanceBundle   string               `json:"provenance_bundle,omitempty"`
	Statuses           map[string]string    `json:"statuses,omitempty"`
	AssetResults       []releaseAssetResult `json:"asset_results,omitempty"`
	Issues             []string             `json:"issues,omitempty"`
	Warnings           []string             `json:"warnings,omitempty"`
	Recommendations    []string             `json:"recommendations,omitempty"`
	VerificationInputs []string             `json:"verification_inputs,omitempty"`
}

type releaseAssetResult struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
	OK     bool   `json:"ok"`
}

func cmdRelease(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("release", "verify")
		return nil
	}
	switch args[0] {
	case "verify":
		return cmdReleaseVerify(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown release subcommand %q", args[0])
	}
}

func cmdReleaseVerify(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("release verify", flag.ContinueOnError)
	version := fs.String("version", "", "release version, usually vX.Y.Z")
	tag := fs.String("tag", "", "release tag")
	sourceSHA := fs.String("source-sha", "", "source commit sha")
	releaseNotes := fs.String("release-notes", "docs/release-notes.md", "release notes path")
	changelog := fs.String("changelog", "CHANGELOG.md", "changelog path")
	ciStatus := fs.String("ci-status", "", "CI status: pass, fail, skipped, or blocked")
	docsStatus := fs.String("docs-status", "", "docs status: pass, fail, skipped, or blocked")
	signingStatus := fs.String("signing-status", "", "signing status: pass, fail, skipped, or blocked")
	notaryStatus := fs.String("notary-status", "", "notary status: pass, fail, skipped, or blocked")
	releaseState := fs.String("release-state", "", "GitHub release state: public or draft")
	releaseURL := fs.String("release-url", "", "GitHub release URL")
	homebrewVersion := fs.String("homebrew-version", "", "Homebrew cask version")
	homebrewTapCommit := fs.String("homebrew-tap-commit", "", "Homebrew tap commit sha")
	brewFetchStatus := fs.String("brew-fetch-status", "", "brew fetch status: pass, fail, skipped, or blocked")
	provenanceBundle := fs.String("provenance-bundle", "", "path to Fairway provenance bundle for this release")
	var assets multiFlag
	var verification multiFlag
	fs.Var(&assets, "asset", "asset check as URL=STATUS; may repeat")
	fs.Var(&verification, "verification-command", "verification command; may repeat")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected release verify arguments: %s", strings.Join(fs.Args(), " "))
	}
	report, err := buildReleaseVerifyReport(releaseVerifyInput{
		Version:              *version,
		Tag:                  *tag,
		SourceSHA:            *sourceSHA,
		ReleaseNotesPath:     *releaseNotes,
		ChangelogPath:        *changelog,
		CIStatus:             *ciStatus,
		DocsStatus:           *docsStatus,
		SigningStatus:        *signingStatus,
		NotaryStatus:         *notaryStatus,
		ReleaseState:         *releaseState,
		ReleaseURL:           *releaseURL,
		HomebrewVersion:      *homebrewVersion,
		HomebrewTapCommit:    *homebrewTapCommit,
		BrewFetchStatus:      *brewFetchStatus,
		ProvenanceBundlePath: *provenanceBundle,
		Assets:               assets,
		VerificationCommands: verification,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		if err := printJSON(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("release_verify: %t\nversion: %s\ntag: %s\n", report.OK, report.Version, report.Tag)
		if report.ReleaseState != "" {
			fmt.Printf("release_state: %s\n", report.ReleaseState)
		}
		if report.HomebrewVersion != "" {
			fmt.Printf("homebrew_version: %s\n", report.HomebrewVersion)
		}
		if report.ProvenanceBundle != "" {
			fmt.Printf("provenance_bundle: %s\n", report.ProvenanceBundle)
		}
		if len(report.AssetResults) > 0 {
			fmt.Println("assets:")
			for _, asset := range report.AssetResults {
				fmt.Printf("- %s status=%d ok=%t\n", asset.URL, asset.Status, asset.OK)
			}
		}
		if len(report.Issues) > 0 {
			fmt.Println("issues:")
			for _, issue := range report.Issues {
				fmt.Printf("- %s\n", issue)
			}
		}
		if len(report.Warnings) > 0 {
			fmt.Println("warnings:")
			for _, warning := range report.Warnings {
				fmt.Printf("- %s\n", warning)
			}
		}
		if len(report.Recommendations) > 0 {
			fmt.Println("recommendations:")
			for _, recommendation := range uniqueStrings(report.Recommendations) {
				fmt.Printf("- %s\n", recommendation)
			}
		}
	}
	if !report.OK {
		return errors.New("release verification failed")
	}
	return nil
}

type releaseVerifyInput struct {
	Version              string
	Tag                  string
	SourceSHA            string
	ReleaseNotesPath     string
	ChangelogPath        string
	CIStatus             string
	DocsStatus           string
	SigningStatus        string
	NotaryStatus         string
	ReleaseState         string
	ReleaseURL           string
	HomebrewVersion      string
	HomebrewTapCommit    string
	BrewFetchStatus      string
	ProvenanceBundlePath string
	Assets               []string
	VerificationCommands []string
}

func buildReleaseVerifyReport(input releaseVerifyInput) (releaseVerifyReport, error) {
	version := strings.TrimSpace(input.Version)
	tag := strings.TrimSpace(input.Tag)
	if version == "" && tag != "" {
		version = tag
	}
	if tag == "" && version != "" {
		tag = version
	}
	report := releaseVerifyReport{
		OK:                 true,
		Version:            version,
		Tag:                tag,
		SourceSHA:          strings.TrimSpace(input.SourceSHA),
		ReleaseState:       strings.ToLower(strings.TrimSpace(input.ReleaseState)),
		ReleaseURL:         strings.TrimSpace(input.ReleaseURL),
		HomebrewVersion:    strings.TrimSpace(input.HomebrewVersion),
		HomebrewTapCommit:  strings.TrimSpace(input.HomebrewTapCommit),
		ProvenanceBundle:   strings.TrimSpace(input.ProvenanceBundlePath),
		VerificationInputs: append([]string{}, input.VerificationCommands...),
		Statuses: map[string]string{
			"ci":         strings.ToLower(strings.TrimSpace(input.CIStatus)),
			"docs":       strings.ToLower(strings.TrimSpace(input.DocsStatus)),
			"signing":    strings.ToLower(strings.TrimSpace(input.SigningStatus)),
			"notary":     strings.ToLower(strings.TrimSpace(input.NotaryStatus)),
			"brew_fetch": strings.ToLower(strings.TrimSpace(input.BrewFetchStatus)),
		},
	}
	if report.Version == "" {
		report.Issues = append(report.Issues, "missing release version")
	}
	if report.Tag == "" {
		report.Issues = append(report.Issues, "missing release tag")
	} else if report.Version != "" && report.Tag != report.Version {
		report.Issues = append(report.Issues, fmt.Sprintf("release tag %q does not match version %q", report.Tag, report.Version))
	}
	if report.SourceSHA == "" {
		report.Warnings = append(report.Warnings, "missing source SHA")
	}
	checkReleaseDoc(&report, "release notes", input.ReleaseNotesPath, report.Version)
	checkReleaseDoc(&report, "changelog", input.ChangelogPath, report.Version)
	for name, status := range report.Statuses {
		switch status {
		case "pass":
		case "":
			report.Issues = append(report.Issues, fmt.Sprintf("missing %s status", strings.ReplaceAll(name, "_", " ")))
		case "fail", "failed", "blocked":
			report.Issues = append(report.Issues, fmt.Sprintf("%s status is %s", strings.ReplaceAll(name, "_", " "), status))
		case "skipped":
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s status was skipped", strings.ReplaceAll(name, "_", " ")))
		default:
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s status is unrecognized: %s", strings.ReplaceAll(name, "_", " "), status))
		}
	}
	if report.ReleaseState == "" {
		report.Issues = append(report.Issues, "missing GitHub release state")
	} else if report.ReleaseState != "public" && report.ReleaseState != "draft" {
		report.Warnings = append(report.Warnings, "release state should be public or draft")
	}
	for _, raw := range input.Assets {
		asset, err := parseReleaseAsset(raw)
		if err != nil {
			return releaseVerifyReport{}, err
		}
		report.AssetResults = append(report.AssetResults, asset)
		if !asset.OK {
			report.Issues = append(report.Issues, fmt.Sprintf("asset URL failed: %s status=%d", asset.URL, asset.Status))
		}
	}
	if len(report.AssetResults) == 0 {
		report.Issues = append(report.Issues, "missing asset URL verification")
	}
	checkReleaseProvenanceBundle(&report, report.ProvenanceBundle)
	if report.HomebrewVersion == "" {
		report.Issues = append(report.Issues, "missing Homebrew cask version")
	} else if report.Version != "" && !sameReleaseVersion(report.HomebrewVersion, report.Version) {
		report.Issues = append(report.Issues, fmt.Sprintf("Homebrew cask version %q does not match release version %q", report.HomebrewVersion, report.Version))
	}
	if report.HomebrewTapCommit == "" {
		report.Issues = append(report.Issues, "missing Homebrew tap commit")
	}
	if report.ReleaseState == "draft" && report.HomebrewVersion != "" && sameReleaseVersion(report.HomebrewVersion, report.Version) {
		report.Issues = append(report.Issues, "Homebrew cask points to this version while GitHub release is still draft")
		report.Recommendations = append(report.Recommendations, "publish the reviewed GitHub release draft before treating the Homebrew cask as usable")
	}
	if len(report.VerificationInputs) == 0 {
		report.Warnings = append(report.Warnings, "missing verification command list")
	}
	report.OK = len(report.Issues) == 0
	return report, nil
}

func checkReleaseProvenanceBundle(report *releaseVerifyReport, bundlePath string) {
	bundlePath = strings.TrimSpace(bundlePath)
	if bundlePath == "" {
		report.Warnings = append(report.Warnings, "missing Fairway provenance bundle")
		report.Recommendations = append(report.Recommendations, "generate a release provenance bundle with fairway provenance report and publish or archive its reviewed reference")
		return
	}
	data, err := os.ReadFile(filepath.Clean(bundlePath))
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("missing provenance bundle at %s", bundlePath))
		return
	}
	text := string(data)
	if report.Version != "" && !strings.Contains(text, report.Version) {
		report.Warnings = append(report.Warnings, "provenance bundle does not mention release version")
	}
	if report.SourceSHA != "" && !strings.Contains(text, report.SourceSHA) {
		report.Warnings = append(report.Warnings, "provenance bundle does not mention source SHA")
	}
}

func sameReleaseVersion(a, b string) bool {
	return normalizeReleaseVersion(a) == normalizeReleaseVersion(b)
}

func normalizeReleaseVersion(value string) string {
	value = strings.TrimSpace(value)
	return strings.TrimPrefix(value, "v")
}

func checkReleaseDoc(report *releaseVerifyReport, label, path, version string) {
	path = strings.TrimSpace(path)
	if path == "" {
		report.Issues = append(report.Issues, "missing "+label+" path")
		return
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("missing %s at %s", label, path))
		return
	}
	if version != "" && !strings.Contains(string(data), version) {
		report.Issues = append(report.Issues, fmt.Sprintf("%s does not mention %s", label, version))
	}
}

func parseReleaseAsset(raw string) (releaseAssetResult, error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 {
		return releaseAssetResult{}, fmt.Errorf("release asset %q must use URL=STATUS", raw)
	}
	url := strings.TrimSpace(parts[0])
	if url == "" {
		return releaseAssetResult{}, errors.New("release asset URL is required")
	}
	status, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return releaseAssetResult{}, fmt.Errorf("release asset %q has invalid status: %w", raw, err)
	}
	return releaseAssetResult{URL: url, Status: status, OK: status >= 200 && status < 400}, nil
}

type reconcileAction struct {
	SessionID  string `json:"session_id,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Backend    string `json:"backend,omitempty"`
	Role       string `json:"role"`
	TaskID     string `json:"task_id,omitempty"`
	TaskStatus string `json:"task_status,omitempty"`
	PID        *int   `json:"pid,omitempty"`
	TmuxPane   string `json:"tmux_pane,omitempty"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
}

func cmdSessionReconcile(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("session reconcile", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show actions without applying")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected session reconcile arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		sessions, err := s.Sessions(ctx, false)
		if err != nil {
			return err
		}
		tasks, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		taskByID := map[string]store.Task{}
		for _, task := range tasks {
			taskByID[task.Definition.ID] = task
		}
		activeSessionByTask := map[string]bool{}
		for _, session := range sessions {
			if session.TaskID != "" {
				activeSessionByTask[session.TaskID] = true
			}
		}
		var actions []reconcileAction
		for _, session := range sessions {
			task := taskByID[session.TaskID]
			reason := ""
			if session.PID != nil && !processAlive(*session.PID) {
				reason = "pid not running"
			}
			if reason == "" && session.SessionBackend == "tmux" && session.TmuxPane != "" && !tmuxPaneAlive(session.TmuxPane) {
				reason = "tmux pane not found"
			}
			if reason == "" && session.TaskID != "" && task.Definition.ID == "" {
				reason = "session task not found"
			}
			if reason == "" && session.TaskID != "" && isTerminal(task.Status, cfg.States.Terminal) {
				reason = "session task is terminal"
			}
			if reason == "" {
				continue
			}
			action := reconcileAction{SessionID: session.ID, Provider: session.Provider, Backend: session.SessionBackend, Role: session.Role, TaskID: session.TaskID, TaskStatus: task.Status, PID: session.PID, TmuxPane: session.TmuxPane, Action: "mark_stale", Reason: reason}
			actions = append(actions, action)
			if !*dryRun {
				if err := s.EndSession(ctx, session.ID, "stale", "reconciled", nil); err != nil {
					return err
				}
			}
		}
		for _, task := range tasks {
			if task.Status != "in_progress" || activeSessionByTask[task.Definition.ID] {
				continue
			}
			actions = append(actions, reconcileAction{Role: task.Definition.Role, TaskID: task.Definition.ID, TaskStatus: task.Status, Action: "report_unattended_in_progress", Reason: "in_progress task has no running session"})
		}
		if opts.JSON {
			return printJSON(actions)
		}
		if len(actions) == 0 {
			fmt.Println("no session reconciliation actions")
			return nil
		}
		for _, action := range actions {
			mode := "applied"
			if *dryRun {
				mode = "would apply"
			}
			if action.SessionID == "" {
				mode = "reported"
			}
			fmt.Printf("%s\t%s\tsession=%s\trole=%s\ttask=%s\tstatus=%s\treason=%s\n", mode, action.Action, action.SessionID, action.Role, action.TaskID, action.TaskStatus, action.Reason)
		}
		return nil
	})
}

func cmdReconcile(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("reconcile requires subcommand: active")
	}
	if isHelpOnly(args) {
		subcommandUsage("reconcile", "active")
		return nil
	}
	switch args[0] {
	case "active":
		return cmdReconcileActive(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown reconcile subcommand %q", args[0])
	}
}

func cmdReconcileActive(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("reconcile active", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show findings without applying limited fixes")
	staleCheckpointAfter := fs.Duration("stale-checkpoint-after", 2*time.Hour, "age after which an active checkpoint is stale")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected reconcile active arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		report, err := reconcile.Active(ctx, s, reconcile.ActiveOptions{Terminal: cfg.States.Terminal, StaleCheckpointAfter: *staleCheckpointAfter})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(report)
		}
		if len(report.Findings) == 0 {
			fmt.Println("no active reconciliation findings")
			return nil
		}
		fmt.Printf("ok: %t\n", report.OK)
		fmt.Printf("summary: stale_sessions=%d unattended_in_progress=%d status_decision_required=%d active_parent_without_rollup=%d stale_checkpoints=%d monitor_sessions_no_proof=%d monitor_resume_needed=%d provider_lifecycle_missing=%d\n",
			report.Summary.StaleSessions,
			report.Summary.UnattendedInProgress,
			report.Summary.StatusDecisionRequired,
			report.Summary.ActiveParentWithoutRollup,
			report.Summary.StaleCheckpoints,
			report.Summary.MonitorSessionsNoProof,
			report.Summary.MonitorResumeNeeded,
			report.Summary.ProviderLifecycleMissing)
		for _, finding := range report.Findings {
			mode := "reported"
			if !*dryRun && finding.Action == "mark_session_stale" && finding.SessionID != "" {
				if err := s.EndSession(ctx, finding.SessionID, "stale", "reconciled by active reconcile", nil); err != nil {
					return err
				}
				mode = "applied"
			}
			fmt.Printf("%s\t%s\trole=%s\ttask=%s\tstatus=%s\tsession=%s\texternal=%s\tprovider=%s\texpected_checkpoint=%s\taction=%s\treason=%s\n",
				mode, finding.Kind, finding.Role, finding.TaskID, finding.TaskStatus, finding.SessionID, finding.ExternalSessionID, finding.Provider, finding.ExpectedCheckpoint, finding.Action, finding.Reason)
		}
		return nil
	})
}

func cmdSessionLaunch(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("session launch", flag.ContinueOnError)
	role := fs.String("role", "", "role")
	backend := fs.String("backend", "", "backend")
	provider := fs.String("provider", "", "provider")
	taskID := fs.String("task-id", "", "task id")
	name := fs.String("name", "", "session name")
	promptFile := fs.String("prompt-file", "", "prompt file to feed to the launched provider")
	promptText := fs.String("prompt", "", "prompt text to write to --prompt-file or a generated prompt file")
	transcript := fs.String("transcript", "", "transcript path for launched output")
	commandText := fs.String("command", "", "provider command to run; defaults to shell login command")
	dryRun := fs.Bool("dry-run", false, "print launch plan without starting a process or recording a session")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected session launch arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		sessionRole := *role
		if sessionRole == "" {
			sessionRole = resolveRole(opts)
		}
		if sessionRole == "" {
			return errors.New("session launch requires --role")
		}
		roleCfg, ok := findRole(cfg, sessionRole)
		if !ok && len(cfg.Roles) > 0 {
			return fmt.Errorf("unknown role %q", sessionRole)
		}
		launchBackend := *backend
		if launchBackend == "" {
			launchBackend = cfg.Sessions.DefaultBackend
		}
		if launchBackend != "shell" {
			return fmt.Errorf("session backend %q is not implemented yet; use session upsert for external adapters", launchBackend)
		}
		branch := fairwaygit.CurrentBranch(root)
		worktreePath := root
		providerName := *provider
		if ok {
			branch = config.RoleBranch(roleCfg)
			worktreePath = config.WorktreePath(cfg, root, roleCfg)
			if providerName == "" {
				providerName = roleCfg.Provider
			}
		}
		sessionName := *name
		if sessionName == "" {
			sessionName = "fairway-" + sessionRole
		}
		sessionID := generatedSessionID(sessionRole, -1)
		resolvedPrompt, err := ensureLaunchPromptFile(worktreePath, sessionRole, *taskID, *promptFile, *promptText, *dryRun)
		if err != nil {
			return err
		}
		resolvedTranscript := *transcript
		if resolvedTranscript == "" {
			resolvedTranscript = filepath.Join(".fairway", "transcripts", sessionID+".log")
		}
		providerCommand := *commandText
		if providerCommand == "" {
			providerCommand = "${SHELL:-bash} -l"
		}
		plan := sessionLaunchPlan{
			SessionID:       sessionID,
			Role:            sessionRole,
			TaskID:          *taskID,
			Backend:         launchBackend,
			Provider:        providerName,
			SessionName:     sessionName,
			WorktreePath:    worktreePath,
			Branch:          branch,
			PromptFile:      resolvedPrompt,
			TranscriptPath:  resolvedTranscript,
			ProviderCommand: providerCommand,
			DryRun:          *dryRun,
		}
		if *dryRun {
			if opts.JSON {
				return printJSON(plan)
			}
			printSessionLaunchPlan(plan)
			return nil
		}
		pid, err := launchShellSession(plan)
		if err != nil {
			return err
		}
		session := store.Session{
			ID:             sessionID,
			Role:           sessionRole,
			WorktreePath:   worktreePath,
			Branch:         branch,
			SessionBackend: launchBackend,
			Provider:       providerName,
			SessionName:    sessionName,
			TaskID:         *taskID,
			PID:            &pid,
			TranscriptPath: resolvedTranscript,
			Status:         "running",
		}
		if err := s.UpsertSession(ctx, session); err != nil {
			return err
		}
		if *taskID != "" {
			if err := s.RecordCheckpoint(ctx, store.Checkpoint{
				TaskID:       *taskID,
				State:        "active",
				Owner:        sessionRole,
				Summary:      fmt.Sprintf("Started shell-backed %s session from prompt file %s; transcript: %s", firstNonEmpty(providerName, "provider"), resolvedPrompt, resolvedTranscript),
				ArtifactPath: resolvedTranscript,
			}); err != nil {
				return err
			}
		}
		if opts.JSON {
			return printJSON(session)
		}
		fmt.Printf("export FAIRWAY_SESSION_ID=%s\n", sessionID)
		if *taskID != "" {
			fmt.Printf("export FAIRWAY_TASK_ID=%s\n", *taskID)
		}
		fmt.Printf("cd %s\n", worktreePath)
		fmt.Printf("transcript %s\n", resolvedTranscript)
		return nil
	})
}

type sessionLaunchPlan struct {
	SessionID       string `json:"session_id"`
	Role            string `json:"role"`
	TaskID          string `json:"task_id,omitempty"`
	Backend         string `json:"backend"`
	Provider        string `json:"provider,omitempty"`
	SessionName     string `json:"session_name"`
	WorktreePath    string `json:"worktree_path"`
	Branch          string `json:"branch"`
	PromptFile      string `json:"prompt_file"`
	TranscriptPath  string `json:"transcript_path"`
	ProviderCommand string `json:"provider_command"`
	DryRun          bool   `json:"dry_run"`
}

func ensureLaunchPromptFile(root, role, taskID, promptFile, promptText string, dryRun bool) (string, error) {
	explicitPromptFile := strings.TrimSpace(promptFile) != ""
	if !explicitPromptFile {
		nameParts := []string{sanitizePathToken(role)}
		if strings.TrimSpace(taskID) != "" {
			nameParts = append(nameParts, sanitizePathToken(taskID))
		}
		nameParts = append(nameParts, time.Now().UTC().Format("20060102T150405Z"))
		promptFile = filepath.Join(".fairway", "prompts", strings.Join(nameParts, "-")+".md")
	}
	if dryRun {
		return promptFile, nil
	}
	path := promptFile
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	if strings.TrimSpace(promptText) != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(promptText), 0o600); err != nil {
			return "", err
		}
		return promptFile, nil
	}
	if _, err := os.Stat(path); err != nil {
		if explicitPromptFile {
			return "", fmt.Errorf("prompt file %s: %w", promptFile, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			return "", err
		}
	}
	return promptFile, nil
}

func launchShellSession(plan sessionLaunchPlan) (int, error) {
	if err := os.MkdirAll(filepath.Dir(absOrRelPath(plan.TranscriptPath, plan.WorktreePath)), 0o755); err != nil {
		return 0, err
	}
	promptPath := absOrRelPath(plan.PromptFile, plan.WorktreePath)
	transcriptPath := absOrRelPath(plan.TranscriptPath, plan.WorktreePath)
	script := fmt.Sprintf("cat %s | %s > %s 2>&1", shellQuote(promptPath), plan.ProviderCommand, shellQuote(transcriptPath))
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}
	cmd := exec.Command(shellPath, "-lc", script)
	cmd.Dir = plan.WorktreePath
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return cmd.Process.Pid, nil
}

func printSessionLaunchPlan(plan sessionLaunchPlan) {
	fmt.Println("session launch dry-run")
	fmt.Printf("session_id: %s\n", plan.SessionID)
	fmt.Printf("role: %s\n", plan.Role)
	fmt.Printf("backend: %s\n", plan.Backend)
	fmt.Printf("provider: %s\n", plan.Provider)
	fmt.Printf("command: %s\n", plan.ProviderCommand)
	fmt.Printf("prompt_file: %s\n", plan.PromptFile)
	fmt.Printf("transcript: %s\n", plan.TranscriptPath)
	fmt.Printf("worktree: %s\n", plan.WorktreePath)
	fmt.Printf("branch: %s\n", plan.Branch)
	if plan.TaskID != "" {
		fmt.Printf("task_id: %s\n", plan.TaskID)
	}
}

func sanitizePathToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "session"
	}
	return out
}

func absOrRelPath(pathValue, root string) string {
	if filepath.IsAbs(pathValue) {
		return pathValue
	}
	return filepath.Join(root, pathValue)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func tmuxPaneAlive(pane string) bool {
	if strings.TrimSpace(pane) == "" {
		return false
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", pane, "#{pane_id}").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func findRole(cfg config.Config, name string) (config.Role, bool) {
	for _, role := range cfg.Roles {
		if role.Name == name {
			return role, true
		}
	}
	return config.Role{}, false
}

func generatedSessionID(role string, pid int) string {
	if pid >= 0 {
		return fmt.Sprintf("%s-%d-%d", role, pid, time.Now().UTC().Unix())
	}
	return fmt.Sprintf("%s-%d", role, time.Now().UTC().UnixNano())
}

func inferCurrentTaskID(ctx context.Context, opts globalOptions, s *store.Store) (string, error) {
	if taskID := os.Getenv("FAIRWAY_TASK_ID"); taskID != "" {
		return taskID, nil
	}
	sessionID := os.Getenv("FAIRWAY_SESSION_ID")
	role := resolveRole(opts)
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		return "", err
	}
	for _, session := range sessions {
		if sessionID != "" && session.ID == sessionID {
			return session.TaskID, nil
		}
	}
	if role == "" {
		return "", nil
	}
	var found string
	for _, session := range sessions {
		if session.Role != role || session.TaskID == "" {
			continue
		}
		if found != "" {
			return "", fmt.Errorf("multiple live sessions for role %s; pass --from-task", role)
		}
		found = session.TaskID
	}
	return found, nil
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func isLeafKind(kind string) bool {
	switch kind {
	case "task", "bug", "spike":
		return true
	default:
		return false
	}
}

type coordinatorReport struct {
	OK              bool             `json:"ok"`
	Health          store.Health     `json:"health"`
	ReadyCount      int              `json:"ready_count"`
	ReadyByRole     map[string]int   `json:"ready_by_role"`
	LiveSessions    []store.Session  `json:"live_sessions"`
	Worktrees       []worktreeStatus `json:"worktrees"`
	Issues          []string         `json:"issues"`
	Recommendations []string         `json:"recommendations"`
}

type dispatchPlan struct {
	Ready           []store.Task     `json:"ready"`
	LiveSessions    []store.Session  `json:"live_sessions"`
	Issues          []string         `json:"issues"`
	Recommendations []string         `json:"recommendations"`
	Worktrees       []worktreeStatus `json:"worktrees"`
	Health          store.Health     `json:"health"`
}

func cmdDispatchPlan(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("dispatch-plan", flag.ContinueOnError)
	role := fs.String("role", "", "role filter")
	limit := fs.Int("limit", 10, "maximum ready tasks")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected dispatch-plan arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		roleFilter := *role
		if roleFilter == "" {
			roleFilter = resolveRole(opts)
		}
		ready, err := s.Ready(ctx, roleFilter, cfg.States.Terminal)
		if err != nil {
			return err
		}
		if *limit >= 0 && len(ready) > *limit {
			ready = ready[:*limit]
		}
		report, err := buildCoordinatorReport(ctx, cfg, root, s)
		if err != nil {
			return err
		}
		plan := dispatchPlan{
			Ready:           ready,
			LiveSessions:    report.LiveSessions,
			Issues:          report.Issues,
			Recommendations: append([]string{}, report.Recommendations...),
			Worktrees:       report.Worktrees,
			Health:          report.Health,
		}
		for _, task := range ready {
			plan.Recommendations = append(plan.Recommendations, fmt.Sprintf("claim %s (%s)", task.Definition.ID, task.Definition.Title))
		}
		if opts.JSON {
			return printJSON(plan)
		}
		fmt.Println("dispatch plan")
		if len(plan.Issues) > 0 {
			fmt.Println("issues:")
			for _, issue := range plan.Issues {
				fmt.Printf("- %s\n", issue)
			}
		}
		fmt.Println("ready:")
		for _, task := range plan.Ready {
			fmt.Printf("- %s [%s] %s\n", task.Definition.ID, task.Definition.Role, task.Definition.Title)
		}
		if len(plan.Recommendations) > 0 {
			fmt.Println("recommendations:")
			for _, rec := range plan.Recommendations {
				fmt.Printf("- %s\n", rec)
			}
		}
		return nil
	})
}

func cmdCoordinator(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("coordinator requires subcommand: preflight, status, tick, plan")
	}
	if isHelpOnly(args) {
		subcommandUsage("coordinator", "preflight|status|tick|plan")
		return nil
	}
	switch args[0] {
	case "preflight":
		return cmdCoordinatorReport(ctx, opts, args[1:], true, false)
	case "status":
		return cmdCoordinatorReport(ctx, opts, args[1:], false, false)
	case "tick":
		return cmdCoordinatorPlan(ctx, opts, args[1:], true)
	case "plan":
		return cmdCoordinatorPlan(ctx, opts, args[1:], false)
	default:
		return fmt.Errorf("unknown coordinator subcommand %q", args[0])
	}
}

func cmdCoordinatorPlan(ctx context.Context, opts globalOptions, args []string, tick bool) error {
	if isHelpOnly(args) {
		if tick {
			subcommandUsage("coordinator", "tick [--completion-handback-wake] [--task <task-id>] [--send] [--state <sent|notification_delivered|thread_steered>] [--provider <name>] [--target <thread-id>]")
		} else {
			subcommandUsage("coordinator", "plan [--ready-limit <n>] [--recommendation-limit <n>] [--allow-utility-monitor]")
		}
		return nil
	}
	fs := flag.NewFlagSet("coordinator plan", flag.ContinueOnError)
	readyLimit := fs.Int("ready-limit", 10, "maximum ready tasks to include")
	recommendationLimit := fs.Int("recommendation-limit", 20, "maximum next actions to include")
	staleAfter := fs.Duration("stale-checkpoint-after", 2*time.Hour, "active checkpoint stale threshold")
	monitorHandbackAfter := fs.Duration("monitor-handback-after", 2*time.Hour, "recent monitor handback window")
	allowUtility := fs.Bool("allow-utility-monitor", false, "recommend continuing configured utility monitors when present")
	completionHandbackWake := fs.Bool("completion-handback-wake", false, "render stale completion-handback wake prompts during coordinator tick")
	taskID := fs.String("task", "", "limit completion-handback wake prompts to one task")
	send := fs.Bool("send", false, "record bounded completion-handback wake delivery/failure notification rows")
	state := fs.String("state", "sent", "notification state to record with --send")
	provider := fs.String("provider", "", "override provider label for completion-handback wake")
	target := fs.String("target", "", "override provider thread/adapter target for completion-handback wake")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected coordinator plan arguments: %s", strings.Join(fs.Args(), " "))
	}
	if !tick && *completionHandbackWake {
		return errors.New("--completion-handback-wake is only supported on coordinator tick")
	}
	if !*completionHandbackWake && (*send || *provider != "" || *target != "" || *state != "sent" || *taskID != "") {
		return errors.New("--task, --send, --provider, --target, and --state require --completion-handback-wake")
	}
	if !*send && (*provider != "" || *target != "" || *state != "sent") {
		return errors.New("--provider, --target, and --state require --send")
	}
	switch *state {
	case "sent", "notification_delivered", "thread_steered":
	default:
		return fmt.Errorf("invalid completion-handback wake --state %q", *state)
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		worktrees, err := collectWorktreeStatus(cfg, root)
		if err != nil {
			return err
		}
		plan, err := coord.BuildPlan(ctx, cfg, s, coord.PlanOptions{
			Worktrees:             coordinatorWorktreeFacts(worktrees),
			StaleCheckpointAfter:  *staleAfter,
			MonitorHandbackAfter:  *monitorHandbackAfter,
			ReadyLimit:            *readyLimit,
			RecommendationLimit:   *recommendationLimit,
			UtilityMonitorAllowed: *allowUtility,
		})
		if err != nil {
			return err
		}
		var wakes []completionHandbackWakeRow
		if *completionHandbackWake {
			taskStatuses, err := taskStatuses(ctx, s)
			if err != nil {
				return err
			}
			wakes = selectCompletionHandbackWakes(plan, cfg, taskStatuses, terminalStatusSet(cfg.States.Terminal), *taskID)
			for i := range wakes {
				if *provider != "" {
					wakes[i].Provider = *provider
				}
				if *target != "" {
					wakes[i].Target = *target
				}
				wakes[i].State = *state
				notifications, err := s.Notifications(ctx, wakes[i].TaskID)
				if err != nil {
					return err
				}
				if completionHandbackWakeSuppressed(wakes[i], notifications) {
					wakes[i].Suppressed = true
					continue
				}
				if !*send {
					continue
				}
				recordState := *state
				reason := "completion_handback_wake signature=" + wakes[i].Signature + " kind=" + wakes[i].Kind
				if !wakeTargetRoutable(wakes[i].TargetStatus, wakes[i].Target) {
					recordState = "notification_failed"
					reason += " failed=no_wake_target action=mapping_required"
					if wakes[i].TargetReason != "" {
						reason += " target_reason=" + strings.ReplaceAll(wakes[i].TargetReason, " ", "_")
					}
					wakes[i].TargetAction = "mapping_required"
					wakes[i].Error = firstNonEmpty(wakes[i].TargetReason, "no wake target configured")
				}
				if _, err := s.RecordNotification(ctx, store.Notification{
					TaskID:   wakes[i].TaskID,
					Domain:   "coordinator",
					Provider: wakes[i].Provider,
					Target:   wakes[i].Target,
					State:    recordState,
					Reason:   reason,
				}); err != nil {
					return err
				}
				wakes[i].State = recordState
			}
		}
		if opts.JSON {
			if *completionHandbackWake {
				return printJSON(struct {
					Plan  coord.Plan                  `json:"plan"`
					Wakes []completionHandbackWakeRow `json:"completion_handback_wakes"`
				}{plan, wakes})
			}
			return printJSON(plan)
		}
		if tick {
			fmt.Println("coordinator tick")
		} else {
			fmt.Println("coordinator plan")
		}
		printCoordinatorPlan(plan)
		if *completionHandbackWake {
			printCompletionHandbackWakes(wakes)
		}
		return nil
	})
}

func coordinatorWorktreeFacts(worktrees []worktreeStatus) []coord.WorktreeFact {
	facts := make([]coord.WorktreeFact, 0, len(worktrees))
	for _, worktree := range worktrees {
		facts = append(facts, coord.WorktreeFact{
			Role:       worktree.Role,
			Branch:     worktree.Branch,
			Path:       worktree.Path,
			Registered: worktree.Registered,
			Exists:     worktree.Exists,
			Dirty:      worktree.Dirty,
			LastCommit: worktree.LastCommit,
		})
	}
	return facts
}

func printCoordinatorPlan(plan coord.Plan) {
	fmt.Printf("dry_run: %t\nok: %t\n", plan.DryRun, plan.OK)
	fmt.Printf("summary: top=%s ready=%d active=%d waiting=%d blocked=%d stale=%d complete=%d review_gated=%d review_complete=%d review_debt=%d approval_gated=%d utility_gated=%d batch_recommended=%d\n",
		plan.Summary.TopClassification,
		plan.Summary.Ready,
		plan.Summary.Active,
		plan.Summary.Waiting,
		plan.Summary.Blocked,
		plan.Summary.Stale,
		plan.Summary.Complete,
		plan.Summary.ReviewGated,
		plan.Summary.ReviewComplete,
		plan.Summary.ReviewDebt,
		plan.Summary.ApprovalGated,
		plan.Summary.UtilityGated,
		plan.Summary.BatchRecommended,
	)
	if plan.Summary.TopReason != "" {
		fmt.Printf("why: %s\n", plan.Summary.TopReason)
	}
	if len(plan.StopConditions) > 0 {
		fmt.Println("stop_conditions:")
		for _, stop := range plan.StopConditions {
			fmt.Printf("- %s task=%s role=%s reason=%s\n", stop.Kind, stop.TaskID, stop.Role, stop.Reason)
		}
	}
	if len(plan.Actions) > 0 {
		fmt.Println("next_actions:")
		for _, action := range plan.Actions {
			target := action.TaskID
			if target == "" && len(action.TaskIDs) > 0 {
				target = strings.Join(action.TaskIDs, ",")
			}
			fmt.Printf("- [%s] %s task=%s role=%s reason=%s\n", action.Classification, action.Action, target, action.Role, action.Reason)
			if action.ReviewHandback != nil {
				fmt.Printf("  review_complete_next_action: task=%s commit=%s review_signature=%s approved=%s missing=%s command=%s status=%s\n",
					action.ReviewHandback.TaskID,
					firstNonEmpty(action.ReviewHandback.Commit, "unknown"),
					firstNonEmpty(action.ReviewHandback.ReviewSignature, "unknown"),
					strings.Join(action.ReviewHandback.ApprovedDomains, ","),
					firstNonEmpty(strings.Join(action.ReviewHandback.MissingDomains, ","), "none"),
					action.ReviewHandback.SuggestedCommand,
					action.ReviewHandback.MergeReadyStatus,
				)
			}
			if action.ReviewNotify != nil {
				fmt.Printf("  review_notification: domain=%s status=%s handoff_id=%d last_handoff_at=%s last_notification_state=%s last_notification_at=%s provider=%s target=%s suggested_action=%s\n",
					action.ReviewNotify.Domain,
					action.ReviewNotify.Status,
					action.ReviewNotify.HandoffID,
					firstNonEmpty(action.ReviewNotify.LastHandoffAt, "none"),
					firstNonEmpty(action.ReviewNotify.LastState, "none"),
					firstNonEmpty(action.ReviewNotify.LastNotificationAt, "none"),
					firstNonEmpty(action.ReviewNotify.Provider, "none"),
					firstNonEmpty(action.ReviewNotify.Target, "none"),
					action.ReviewNotify.SuggestedAction,
				)
			}
			if action.ReviewWait != nil {
				fmt.Printf("  review_wait: wait_id=%s domain=%s state=%s blocking=%t action=%s target=%s:%s expected_response_at=%s\n",
					action.ReviewWait.WaitID,
					action.ReviewWait.Domain,
					action.ReviewWait.State,
					action.ReviewWait.Blocking,
					action.ReviewWait.Action,
					firstNonEmpty(action.ReviewWait.TargetProvider, "none"),
					firstNonEmpty(action.ReviewWait.TargetID, "none"),
					firstNonEmpty(action.ReviewWait.ExpectedResponseAt, "none"),
				)
			}
			if action.LiveWindow != nil {
				fmt.Printf("  live_window: phase=%s next_owner=%s next_action=%s target_close_by=%s checkpoint_at=%s\n",
					action.LiveWindow.Phase,
					firstNonEmpty(action.LiveWindow.NextOwner, "none"),
					firstNonEmpty(action.LiveWindow.NextAction, "none"),
					firstNonEmpty(action.LiveWindow.TargetCloseBy, "none"),
					firstNonEmpty(action.LiveWindow.CheckpointAt, "none"),
				)
			}
			if action.CompletionHandback != nil {
				fmt.Printf("  completion_handback: handoff_id=%d to=%s delivery_status=%s delivery_state=%s completion_state=%s task_status=%s live_window_phase=%s stale=%t stale_age=%s actual_thread_delivery=%t provider=%s target=%s suggested_action=%s next_action=%s\n",
					action.CompletionHandback.HandoffID,
					action.CompletionHandback.ToRole,
					action.CompletionHandback.DeliveryStatus,
					firstNonEmpty(action.CompletionHandback.DeliveryState, "none"),
					firstNonEmpty(action.CompletionHandback.CompletionState, "unspecified"),
					firstNonEmpty(action.CompletionHandback.TaskStatus, "unknown"),
					firstNonEmpty(action.CompletionHandback.LiveWindowPhase, "none"),
					action.CompletionHandback.Stale,
					firstNonEmpty(action.CompletionHandback.StaleAge, "none"),
					action.CompletionHandback.ActualThreadDelivery,
					firstNonEmpty(action.CompletionHandback.Provider, "none"),
					firstNonEmpty(action.CompletionHandback.Target, "none"),
					firstNonEmpty(action.CompletionHandback.SuggestedAction, "none"),
					action.CompletionHandback.NextAction,
				)
			}
		}
	}
}

func cmdCoordinatorReport(ctx context.Context, opts globalOptions, args []string, failOnIssue, tick bool) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected coordinator arguments: %s", strings.Join(args, " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		report, err := buildCoordinatorReport(ctx, cfg, root, s)
		if err != nil {
			return err
		}
		if opts.JSON {
			if err := printJSON(report); err != nil {
				return err
			}
		} else if tick {
			printCoordinatorTick(report)
		} else {
			printCoordinatorStatus(report)
		}
		if failOnIssue && !report.OK {
			return errors.New("coordinator preflight failed")
		}
		return nil
	})
}

func buildCoordinatorReport(ctx context.Context, cfg config.Config, root string, s *store.Store) (coordinatorReport, error) {
	health, err := s.Health(ctx)
	if err != nil {
		return coordinatorReport{}, err
	}
	ready, err := s.Ready(ctx, "", cfg.States.Terminal)
	if err != nil {
		return coordinatorReport{}, err
	}
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return coordinatorReport{}, err
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		return coordinatorReport{}, err
	}
	worktrees, err := collectWorktreeStatus(cfg, root)
	if err != nil {
		return coordinatorReport{}, err
	}
	report := coordinatorReport{
		OK:           true,
		Health:       health,
		ReadyCount:   len(ready),
		ReadyByRole:  map[string]int{},
		LiveSessions: sessions,
		Worktrees:    worktrees,
	}
	for _, task := range ready {
		report.ReadyByRole[task.Definition.Role]++
	}
	if len(cfg.Roles) == 0 {
		report.Issues = append(report.Issues, "no roles configured")
	}
	for _, wt := range worktrees {
		if !wt.Exists || !wt.Registered {
			report.Issues = append(report.Issues, fmt.Sprintf("worktree for role %s is not set up", wt.Role))
		}
		if wt.Dirty {
			report.Issues = append(report.Issues, fmt.Sprintf("worktree for role %s has uncommitted changes", wt.Role))
		}
	}
	if health.StaleInProgress > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("%d stale in-progress task(s)", health.StaleInProgress))
	}
	if health.UnacknowledgedOver1Hour > 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("%d handoff(s) unacknowledged over 1h", health.UnacknowledgedOver1Hour))
	}
	for _, task := range tasks {
		if task.Status == "todo" || isTerminal(task.Status, cfg.States.Terminal) {
			continue
		}
		for _, issue := range reviewstate.UnroutableRequiredDomains(task, reviewWaitOptions(cfg)) {
			report.Issues = append(report.Issues, fmt.Sprintf("task %s required review domain %s is not routable: %s; action=%s", issue.TaskID, issue.Domain, issue.Reason, issue.Action))
		}
	}
	for _, issue := range coordinatorWakeTargetIssues(ctx, cfg, root, s, tasks) {
		report.Issues = append(report.Issues, issue)
	}
	if report.ReadyCount > 0 {
		report.Recommendations = append(report.Recommendations, fmt.Sprintf("claim from %d ready task(s)", report.ReadyCount))
	}
	if health.UnroutedReviews > 0 {
		report.Recommendations = append(report.Recommendations, fmt.Sprintf("route %d pending review(s)", health.UnroutedReviews))
	}
	if health.UnacknowledgedHandoff > 0 {
		report.Recommendations = append(report.Recommendations, fmt.Sprintf("acknowledge or act on %d handoff(s)", health.UnacknowledgedHandoff))
	}
	report.OK = len(report.Issues) == 0
	return report, nil
}

func coordinatorWakeTargetIssues(ctx context.Context, cfg config.Config, root string, s *store.Store, tasks []store.Task) []string {
	statuses := map[string]string{}
	for _, task := range tasks {
		statuses[task.Definition.ID] = task.Status
	}
	rows, err := projectedWaitRows(ctx, cfg, root, s, 24*time.Hour)
	if err != nil {
		return nil
	}
	terminal := terminalStatusSet(cfg.States.Terminal)
	seen := map[string]bool{}
	var issues []string
	for _, row := range rows {
		taskID := strings.TrimSpace(row.TaskID)
		if taskID == "" && row.Kind != "track_memory" {
			continue
		}
		status := strings.TrimSpace(statuses[taskID])
		if taskID != "" && (status == "todo" || terminal[status]) {
			continue
		}
		if !row.Stale && row.State != "failed" && row.State != "notification_failed" && !providerSessionWakeCandidate(row) {
			continue
		}
		target := resolveWakeTarget(cfg.ProviderTargets, row.Owner)
		if wakeTargetRoutable(target.Status, target.Target) {
			continue
		}
		key := strings.Join([]string{firstNonEmpty(taskID, "taskless"), row.Kind, row.WaitID, target.Reason}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		subject := "task " + taskID
		if taskID == "" {
			subject = "taskless"
		}
		issues = append(issues, fmt.Sprintf("%s wait %s kind %s owner %s is not wake-routable: %s; action=mapping_required", subject, row.WaitID, row.Kind, firstNonEmpty(row.Owner, "none"), target.Reason))
	}
	sort.Strings(issues)
	return issues
}

func printCoordinatorStatus(report coordinatorReport) {
	fmt.Printf("ok: %t\nready: %d\nlive_sessions: %d\n", report.OK, report.ReadyCount, len(report.LiveSessions))
	fmt.Printf("health: in_progress=%d stale=%d handoffs=%d reviews=%d\n", report.Health.InProgress, report.Health.StaleInProgress, report.Health.UnacknowledgedHandoff, report.Health.UnroutedReviews)
	if len(report.ReadyByRole) > 0 {
		fmt.Println("ready_by_role:")
		for role, count := range report.ReadyByRole {
			fmt.Printf("- %s: %d\n", role, count)
		}
	}
	if len(report.LiveSessions) > 0 {
		fmt.Println("sessions:")
		for _, session := range report.LiveSessions {
			fmt.Printf("- %s %s/%s task=%s pane=%s transcript=%s\n", session.ID, session.SessionBackend, session.Provider, session.TaskID, session.TmuxPane, session.TranscriptPath)
		}
	}
	if len(report.Issues) > 0 {
		fmt.Println("issues:")
		for _, issue := range report.Issues {
			fmt.Printf("- %s\n", issue)
		}
	}
}

func printCoordinatorTick(report coordinatorReport) {
	printCoordinatorStatus(report)
	if len(report.Recommendations) > 0 {
		fmt.Println("recommendations:")
		for _, rec := range report.Recommendations {
			fmt.Printf("- %s\n", rec)
		}
	}
}

type adoptionArtifact struct {
	ArtifactType     string                   `json:"artifact_type"`
	GeneratedAt      string                   `json:"generated_at"`
	Project          string                   `json:"project"`
	Gates            adoptionGateSummary      `json:"gates"`
	ProfileGates     []adoptionProfileGate    `json:"profile_gates,omitempty"`
	GateEvaluations  []adoptionGateEvaluation `json:"gate_evaluations,omitempty"`
	TaskCount        int                      `json:"task_count"`
	ReadyCount       int                      `json:"ready_count"`
	ReadyByRole      map[string]int           `json:"ready_by_role"`
	ReadySample      []adoptionTaskSample     `json:"ready_sample"`
	RouteSamples     []adoptionRouteSample    `json:"route_samples"`
	RegressionPacks  adoptionRegressionPack   `json:"regression_packs"`
	EvidenceGapCount int                      `json:"evidence_gap_count"`
	EvidenceGaps     []string                 `json:"evidence_gaps"`
	Health           store.Health             `json:"health"`
	Coordinator      coordinatorReport        `json:"coordinator"`
}

type adoptionGateSummary struct {
	EvidenceBeforeDone      string `json:"evidence_before_done"`
	ReviewBeforeDone        string `json:"review_before_done"`
	HandoffBeforeMergeReady string `json:"handoff_before_merge_ready"`
	BlockedReason           string `json:"blocked_reason"`
}

type adoptionProfileGate struct {
	Profile               string   `json:"profile"`
	Name                  string   `json:"name"`
	Group                 string   `json:"group,omitempty"`
	Mode                  string   `json:"mode"`
	TaskKinds             []string `json:"task_kinds,omitempty"`
	EvidenceType          string   `json:"evidence_type,omitempty"`
	RequiredEvidenceCount int      `json:"required_evidence_count,omitempty"`
	AcceptedResults       []string `json:"accepted_results,omitempty"`
	ArtifactRequired      bool     `json:"artifact_required,omitempty"`
	OwnerSignoffRequired  bool     `json:"owner_signoff_required,omitempty"`
	ExpiresAfter          string   `json:"expires_after,omitempty"`
	Description           string   `json:"description,omitempty"`
}

type adoptionTaskSample struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Title    string `json:"title"`
	Priority *int   `json:"priority,omitempty"`
}

type adoptionRouteSample struct {
	Path     string `json:"path"`
	Reviewer string `json:"reviewer"`
	Reason   string `json:"reason"`
	OK       bool   `json:"ok"`
}

type adoptionRegressionPack struct {
	CatalogPath string   `json:"catalog_path,omitempty"`
	OK          bool     `json:"ok"`
	PackCount   int      `json:"pack_count"`
	Issues      []string `json:"issues,omitempty"`
}

type adoptionGateEvaluation struct {
	Profile        string                 `json:"profile"`
	Gate           string                 `json:"gate"`
	Group          string                 `json:"group,omitempty"`
	Mode           string                 `json:"mode"`
	EvidenceType   string                 `json:"evidence_type,omitempty"`
	TaskCount      int                    `json:"task_count"`
	SatisfiedCount int                    `json:"satisfied_count"`
	MissingCount   int                    `json:"missing_count"`
	Status         string                 `json:"status"`
	Missing        []adoptionGateTaskMiss `json:"missing,omitempty"`
}

type adoptionGateTaskMiss struct {
	TaskID   string   `json:"task_id"`
	Title    string   `json:"title"`
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Reasons  []string `json:"reasons"`
	Matching int      `json:"matching_evidence_count"`
}

func cmdParity(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("parity requires subcommand: artifact")
	}
	if isHelpOnly(args) {
		subcommandUsage("parity", "artifact")
		return nil
	}
	switch args[0] {
	case "artifact":
		return cmdAdoptionArtifact(ctx, opts, "parity", args[1:])
	default:
		return fmt.Errorf("unknown parity subcommand %q", args[0])
	}
}

func cmdAdoption(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("adoption requires subcommand: artifact")
	}
	if isHelpOnly(args) {
		subcommandUsage("adoption", "artifact")
		return nil
	}
	switch args[0] {
	case "artifact":
		return cmdAdoptionArtifact(ctx, opts, "adoption", args[1:])
	default:
		return fmt.Errorf("unknown adoption subcommand %q", args[0])
	}
}

type readinessReport struct {
	OK              bool                     `json:"ok"`
	Profile         string                   `json:"profile,omitempty"`
	TaskCount       int                      `json:"task_count"`
	BlockingMissing int                      `json:"blocking_missing"`
	AdvisoryMissing int                      `json:"advisory_missing"`
	GateEvaluations []adoptionGateEvaluation `json:"gate_evaluations"`
}

func cmdReadiness(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("readiness requires subcommand: report")
	}
	if isHelpOnly(args) {
		subcommandUsage("readiness", "report")
		return nil
	}
	if args[0] != "report" {
		return fmt.Errorf("unknown readiness subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("readiness report", flag.ContinueOnError)
	profileName := fs.String("profile", "", "workstream profile")
	gapLimit := fs.Int("gap-limit", 5, "maximum missing task examples per gate")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected readiness report arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		if *profileName != "" {
			profile, ok := findWorkstreamProfile(cfg, *profileName)
			if !ok {
				return fmt.Errorf("unknown workstream profile %q", *profileName)
			}
			cfg.WorkstreamProfiles = []config.WorkstreamProfile{profile}
		}
		tasks, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		evaluations, err := evaluateProfileGates(ctx, cfg, s, tasks, *gapLimit)
		if err != nil {
			return err
		}
		report := readinessReport{
			OK:              true,
			Profile:         *profileName,
			TaskCount:       len(tasks),
			GateEvaluations: evaluations,
		}
		for _, evaluation := range evaluations {
			if evaluation.Status != "missing" {
				continue
			}
			if evaluation.Mode == "blocking" {
				report.BlockingMissing += evaluation.MissingCount
				report.OK = false
			} else {
				report.AdvisoryMissing += evaluation.MissingCount
			}
		}
		if opts.JSON {
			return printJSON(report)
		}
		printReadinessReport(report)
		if !report.OK {
			return errors.New("readiness report has missing blocking gates")
		}
		return nil
	})
}

func findWorkstreamProfile(cfg config.Config, name string) (config.WorkstreamProfile, bool) {
	for _, profile := range cfg.WorkstreamProfiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return config.WorkstreamProfile{}, false
}

func printReadinessReport(report readinessReport) {
	fmt.Println("# Fairway Readiness Report")
	if report.Profile != "" {
		fmt.Printf("\nprofile: %s\n", report.Profile)
	}
	fmt.Printf("ready: %t\ntasks: %d\nblocking_missing: %d\nadvisory_missing: %d\n", report.OK, report.TaskCount, report.BlockingMissing, report.AdvisoryMissing)
	fmt.Println("\n## Gates")
	for _, evaluation := range report.GateEvaluations {
		label := evaluation.Profile + "/" + evaluation.Gate
		group := evaluation.Group
		if group == "" {
			group = "general"
		}
		fmt.Printf("- %s: %s (%s; %d/%d satisfied)\n", label, evaluation.Status, group, evaluation.SatisfiedCount, evaluation.TaskCount)
		for _, miss := range evaluation.Missing {
			fmt.Printf("  - %s [%s] %s: %s\n", miss.TaskID, miss.Kind, miss.Title, strings.Join(miss.Reasons, "; "))
		}
	}
}

func cmdAdoptionArtifact(ctx context.Context, opts globalOptions, artifactType string, args []string) error {
	fs := flag.NewFlagSet(artifactType+" artifact", flag.ContinueOnError)
	catalogPath := fs.String("catalog", "", "regression pack catalog path")
	limit := fs.Int("limit", 20, "maximum ready tasks to include")
	gapLimit := fs.Int("gap-limit", 50, "maximum evidence gaps to include")
	var routePaths multiFlag
	fs.Var(&routePaths, "route", "path to sample through review routing; may repeat")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected %s artifact arguments: %s", artifactType, strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		artifact, err := buildAdoptionArtifact(ctx, cfg, root, s, artifactType, *catalogPath, routePaths, *limit, *gapLimit)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(artifact)
		}
		printAdoptionArtifact(artifact)
		return nil
	})
}

func buildAdoptionArtifact(ctx context.Context, cfg config.Config, root string, s *store.Store, artifactType, catalogPath string, routePaths []string, limit, gapLimit int) (adoptionArtifact, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return adoptionArtifact{}, err
	}
	ready, err := s.Ready(ctx, "", cfg.States.Terminal)
	if err != nil {
		return adoptionArtifact{}, err
	}
	health, err := s.Health(ctx)
	if err != nil {
		return adoptionArtifact{}, err
	}
	coordinator, err := buildCoordinatorReport(ctx, cfg, root, s)
	if err != nil {
		coordinator = coordinatorReport{
			OK:          false,
			Health:      health,
			ReadyCount:  len(ready),
			ReadyByRole: map[string]int{},
			Issues:      []string{fmt.Sprintf("coordinator report unavailable: %v", err)},
		}
		for _, task := range ready {
			coordinator.ReadyByRole[task.Definition.Role]++
		}
	}
	if len(routePaths) == 0 {
		routePaths = defaultAdoptionRouteSamples(cfg)
	}
	artifact := adoptionArtifact{
		ArtifactType:    artifactType,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Project:         cfg.Fairway.ProjectName,
		Gates:           adoptionGates(cfg),
		ProfileGates:    adoptionProfileGates(cfg),
		TaskCount:       len(tasks),
		ReadyCount:      len(ready),
		ReadyByRole:     map[string]int{},
		RouteSamples:    make([]adoptionRouteSample, 0, len(routePaths)),
		RegressionPacks: validateAdoptionRegressionCatalog(catalogPath),
		Health:          health,
		Coordinator:     coordinator,
	}
	artifact.GateEvaluations, err = evaluateProfileGates(ctx, cfg, s, tasks, gapLimit)
	if err != nil {
		return adoptionArtifact{}, err
	}
	for _, task := range ready {
		artifact.ReadyByRole[task.Definition.Role]++
	}
	if limit < 0 || limit > len(ready) {
		limit = len(ready)
	}
	for _, task := range ready[:limit] {
		artifact.ReadySample = append(artifact.ReadySample, adoptionTaskSample{
			ID:       task.Definition.ID,
			Role:     task.Definition.Role,
			Status:   task.Status,
			Title:    task.Definition.Title,
			Priority: task.Definition.Priority,
		})
	}
	for _, routePath := range routePaths {
		reviewer, reason := matchReviewRoute(cfg.ReviewRoutes, []string{routePath})
		artifact.RouteSamples = append(artifact.RouteSamples, adoptionRouteSample{
			Path:     routePath,
			Reviewer: reviewer,
			Reason:   reason,
			OK:       reviewer != "",
		})
	}
	var evidenceGaps []string
	for _, task := range tasks {
		if !isTerminal(task.Status, cfg.States.Terminal) {
			continue
		}
		ok, err := s.HasEvidence(ctx, task.Definition.ID)
		if err != nil {
			return adoptionArtifact{}, err
		}
		if !ok {
			evidenceGaps = append(evidenceGaps, fmt.Sprintf("%s done without evidence", task.Definition.ID))
		}
	}
	sort.Strings(evidenceGaps)
	artifact.EvidenceGapCount = len(evidenceGaps)
	if gapLimit < 0 || gapLimit > len(evidenceGaps) {
		gapLimit = len(evidenceGaps)
	}
	artifact.EvidenceGaps = evidenceGaps[:gapLimit]
	return artifact, nil
}

func adoptionGates(cfg config.Config) adoptionGateSummary {
	return adoptionGateSummary{
		EvidenceBeforeDone:      gateMode(cfg.Gates.RequireEvidenceBeforeDone),
		ReviewBeforeDone:        gateMode(cfg.Gates.RequireReviewBeforeDone),
		HandoffBeforeMergeReady: gateMode(cfg.Gates.RequireHandoffBeforeMergeReady),
		BlockedReason:           gateMode(cfg.Gates.RequireBlockedReason),
	}
}

func adoptionProfileGates(cfg config.Config) []adoptionProfileGate {
	var gates []adoptionProfileGate
	for _, profile := range cfg.WorkstreamProfiles {
		for _, gate := range profile.Gates {
			gates = append(gates, adoptionProfileGate{
				Profile:               profile.Name,
				Name:                  gate.Name,
				Group:                 profileGateGroup(profile, gate),
				Mode:                  gate.Mode,
				TaskKinds:             gate.TaskKinds,
				EvidenceType:          gate.EvidenceType,
				RequiredEvidenceCount: gate.RequiredEvidenceCount,
				AcceptedResults:       gate.AcceptedResults,
				ArtifactRequired:      gate.ArtifactRequired,
				OwnerSignoffRequired:  gate.OwnerSignoffRequired,
				ExpiresAfter:          gate.ExpiresAfter,
				Description:           gate.Description,
			})
		}
	}
	return gates
}

func evaluateProfileGates(ctx context.Context, cfg config.Config, s *store.Store, tasks []store.Task, gapLimit int) ([]adoptionGateEvaluation, error) {
	var evaluations []adoptionGateEvaluation
	now := time.Now().UTC()
	for _, profile := range cfg.WorkstreamProfiles {
		if len(profile.Gates) == 0 {
			continue
		}
		for _, gate := range profile.Gates {
			profileTasks := tasksForProfileGate(profile, gate, tasks)
			evaluation := adoptionGateEvaluation{
				Profile:      profile.Name,
				Gate:         gate.Name,
				Group:        profileGateGroup(profile, gate),
				Mode:         gate.Mode,
				EvidenceType: gate.EvidenceType,
				TaskCount:    len(profileTasks),
				Status:       "satisfied",
			}
			if len(profileTasks) == 0 {
				evaluation.Status = "no_tasks"
				evaluations = append(evaluations, evaluation)
				continue
			}
			for _, task := range profileTasks {
				_, _, evidence, _, _, err := s.TaskDetail(ctx, task.Definition.ID)
				if err != nil {
					return nil, err
				}
				ok, matching, reasons := evaluateGateForTask(gate, evidence, now)
				if ok {
					evaluation.SatisfiedCount++
					continue
				}
				evaluation.MissingCount++
				if gapLimit < 0 || len(evaluation.Missing) < gapLimit {
					evaluation.Missing = append(evaluation.Missing, adoptionGateTaskMiss{
						TaskID:   task.Definition.ID,
						Title:    task.Definition.Title,
						Kind:     task.Definition.Kind,
						Status:   task.Status,
						Reasons:  reasons,
						Matching: matching,
					})
				}
			}
			if evaluation.MissingCount > 0 {
				evaluation.Status = "missing"
			}
			evaluations = append(evaluations, evaluation)
		}
	}
	return evaluations, nil
}

func profileGateGroup(profile config.WorkstreamProfile, gate config.WorkstreamProfileGate) string {
	if strings.TrimSpace(gate.Group) != "" {
		return strings.TrimSpace(gate.Group)
	}
	if len(gate.TaskKinds) > 0 {
		return strings.Join(gate.TaskKinds, ", ")
	}
	if gate.EvidenceType != "" {
		return gate.EvidenceType
	}
	return "general"
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}

func evaluateTaskProfileGates(cfg config.Config, task store.Task, evidence []store.Evidence, now time.Time) []adoptionGateEvaluation {
	var evaluations []adoptionGateEvaluation
	for _, profile := range cfg.WorkstreamProfiles {
		if len(profile.Gates) == 0 || !profileAppliesToTask(profile, task) {
			continue
		}
		for _, gate := range profile.Gates {
			if !gateAppliesToTask(gate, task) {
				continue
			}
			ok, matching, reasons := evaluateGateForTask(gate, evidence, now)
			evaluation := adoptionGateEvaluation{
				Profile:        profile.Name,
				Gate:           gate.Name,
				Group:          profileGateGroup(profile, gate),
				Mode:           gate.Mode,
				EvidenceType:   gate.EvidenceType,
				TaskCount:      1,
				SatisfiedCount: 1,
				Status:         "satisfied",
			}
			if !ok {
				evaluation.SatisfiedCount = 0
				evaluation.MissingCount = 1
				evaluation.Status = "missing"
				evaluation.Missing = []adoptionGateTaskMiss{{
					TaskID:   task.Definition.ID,
					Title:    task.Definition.Title,
					Kind:     task.Definition.Kind,
					Status:   task.Status,
					Reasons:  reasons,
					Matching: matching,
				}}
			}
			evaluations = append(evaluations, evaluation)
		}
	}
	return evaluations
}

func mergeReadyGateMessage(evaluation adoptionGateEvaluation) string {
	label := evaluation.Profile + "/" + evaluation.Gate
	if len(evaluation.Missing) == 0 {
		return "profile gate " + label + " missing"
	}
	return fmt.Sprintf("profile gate %s missing for %s: %s", label, evaluation.Missing[0].TaskID, strings.Join(evaluation.Missing[0].Reasons, "; "))
}

type ruleEvidenceEvaluation struct {
	TaskID           string   `json:"task_id"`
	RuleID           string   `json:"rule_id"`
	Mode             string   `json:"mode"`
	Status           string   `json:"status"`
	RequiredEvidence []string `json:"required_evidence,omitempty"`
	MissingEvidence  []string `json:"missing_evidence,omitempty"`
}

func ruleEvidenceEvaluations(cfg config.Config, root string, task store.Task, evidence []store.Evidence) ([]ruleEvidenceEvaluation, error) {
	packs, err := rules.LoadConfigured(cfg, root, rules.LoadOptions{
		Root:            root,
		KnownDomains:    rules.ReviewDomainSet(cfg),
		KnownEvidence:   rules.ConfigGateEvidenceSet(cfg),
		IncludeDisabled: true,
	})
	if err != nil {
		return nil, err
	}
	recorded := map[string]bool{}
	for _, row := range evidence {
		evidenceType := strings.TrimSpace(row.ArtifactType)
		if evidenceType != "" {
			recorded[evidenceType] = true
		}
	}
	var evaluations []ruleEvidenceEvaluation
	for _, match := range rules.MatchTask(cfg, packs, task) {
		if match.Status != "selected" {
			continue
		}
		evaluation := ruleEvidenceEvaluation{
			TaskID:           task.Definition.ID,
			RuleID:           match.Rule.ID,
			Mode:             firstNonEmpty(match.Rule.Mode, "advisory"),
			Status:           "satisfied",
			RequiredEvidence: append([]string(nil), match.Rule.RequiredEvidence...),
		}
		for _, evidenceType := range match.Rule.RequiredEvidence {
			if !recorded[evidenceType] {
				evaluation.MissingEvidence = append(evaluation.MissingEvidence, evidenceType)
			}
		}
		if len(evaluation.MissingEvidence) > 0 {
			evaluation.Status = "missing"
		}
		evaluations = append(evaluations, evaluation)
	}
	return evaluations, nil
}

func ruleEvidenceMessage(evaluation ruleEvidenceEvaluation) string {
	return fmt.Sprintf("rule evidence missing task=%s rule=%s mode=%s evidence=%s", evaluation.TaskID, evaluation.RuleID, evaluation.Mode, strings.Join(evaluation.MissingEvidence, ","))
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
	var missing []string
	seen := map[string]bool{}
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

func reviewPolicyEvaluation(ctx context.Context, cfg config.Config, s *store.Store, task store.Task, reviews []store.Review, changedPaths []string) (reviewpolicy.Evaluation, error) {
	var parent *store.Task
	var parentReviews []store.Review
	if strings.TrimSpace(task.Definition.ParentID) != "" {
		parentTask, _, _, _, parentTaskReviews, err := s.TaskDetail(ctx, task.Definition.ParentID)
		if err == nil {
			parent = &parentTask
			parentReviews = parentTaskReviews
		}
	}
	return reviewpolicy.Evaluate(cfg, reviewpolicy.Options{
		Task:          task,
		Parent:        parent,
		Reviews:       reviews,
		ParentReviews: parentReviews,
		ChangedPaths:  changedPaths,
	}), nil
}

func reviewPolicyDomainReason(eval reviewpolicy.Evaluation, domain string) string {
	for _, req := range eval.Requirements {
		if req.Domain == domain {
			return req.Reason
		}
	}
	return "required review domain"
}

func printReviewPolicyEvaluation(eval reviewpolicy.Evaluation) {
	fmt.Println("review_policy:")
	fmt.Printf("- profile: %s mode=%s group_review=%t inheritance_blocked=%t\n", firstNonEmpty(eval.Profile, "task-review-domains"), firstNonEmpty(eval.Mode, "blocking"), eval.GroupReview, eval.InheritanceBlocked)
	if eval.SafeIterationZone {
		fmt.Printf("- safe_iteration_zone: true defect_class=%s control=%s\n", firstNonEmpty(eval.SafeIterationDefectClass, "unspecified"), firstNonEmpty(eval.SafeIterationControl, "unspecified"))
	}
	if eval.ExtraReviewerRationale != "" {
		fmt.Printf("- extra_reviewer_rationale: %s\n", eval.ExtraReviewerRationale)
	}
	if eval.ProcessHypothesis != "" {
		fmt.Printf("- process_hypothesis: %s\n", eval.ProcessHypothesis)
	}
	if len(eval.OutcomeMetrics) > 0 {
		fmt.Printf("- outcome_metrics: %s\n", strings.Join(eval.OutcomeMetrics, ", "))
	}
	if len(eval.InheritanceBlockers) > 0 {
		fmt.Printf("- inheritance_blockers: %s\n", strings.Join(eval.InheritanceBlockers, "; "))
	}
	for _, req := range eval.Requirements {
		fmt.Printf("- %s: %s (%s)\n", req.Domain, req.Status, req.Reason)
	}
}

func tasksForProfile(profile config.WorkstreamProfile, tasks []store.Task) []store.Task {
	if len(profile.TaskKinds) == 0 {
		return tasks
	}
	var out []store.Task
	for _, task := range tasks {
		if profileAppliesToTask(profile, task) {
			out = append(out, task)
		}
	}
	return out
}

func tasksForProfileGate(profile config.WorkstreamProfile, gate config.WorkstreamProfileGate, tasks []store.Task) []store.Task {
	var out []store.Task
	for _, task := range tasksForProfile(profile, tasks) {
		if gateAppliesToTask(gate, task) {
			out = append(out, task)
		}
	}
	return out
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

func evaluateGateForTask(gate config.WorkstreamProfileGate, evidence []store.Evidence, now time.Time) (bool, int, []string) {
	requiredCount := gate.RequiredEvidenceCount
	if requiredCount == 0 && (gate.EvidenceType != "" || len(gate.AcceptedResults) > 0 || gate.ArtifactRequired || gate.ExpiresAfter != "" || gate.OwnerSignoffRequired) {
		requiredCount = 1
	}
	accepted := map[string]bool{}
	for _, result := range gate.AcceptedResults {
		accepted[result] = true
	}
	var matching int
	for _, ev := range evidence {
		if gate.EvidenceType != "" && ev.ArtifactType != gate.EvidenceType {
			continue
		}
		if len(accepted) > 0 && !accepted[ev.Result] {
			continue
		}
		if gate.ArtifactRequired && ev.ArtifactPath == "" {
			continue
		}
		if gate.ExpiresAfter != "" && !evidenceIsFresh(ev, gate.ExpiresAfter, now) {
			continue
		}
		if gate.OwnerSignoffRequired && !evidenceHasOwnerSignoff(ev) {
			continue
		}
		matching++
	}
	var reasons []string
	if matching < requiredCount {
		reasons = append(reasons, fmt.Sprintf("needs %d matching evidence row(s), found %d", requiredCount, matching))
		if gate.ArtifactRequired {
			reasons = append(reasons, "matching rows must include evidence artifacts")
		}
		if gate.ExpiresAfter != "" {
			reasons = append(reasons, "matching rows must be fresh")
		}
		if gate.OwnerSignoffRequired {
			reasons = append(reasons, "matching rows must include owner signoff evidence notes")
		}
	}
	return len(reasons) == 0, matching, reasons
}

func evidenceHasOwnerSignoff(ev store.Evidence) bool {
	notes := strings.ToLower(ev.Notes)
	return strings.Contains(notes, "signoff") || strings.Contains(notes, "sign-off")
}

func evidenceIsFresh(ev store.Evidence, expiresAfter string, now time.Time) bool {
	if expiresAfter == "" {
		return true
	}
	ttl, err := time.ParseDuration(expiresAfter)
	if err != nil {
		return false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ev.CreatedAt)
	if err != nil {
		return false
	}
	return now.Sub(createdAt) <= ttl
}

func gateMode(blocking bool) string {
	if blocking {
		return "blocking"
	}
	return "advisory"
}

func defaultAdoptionRouteSamples(cfg config.Config) []string {
	var samples []string
	seen := map[string]bool{}
	for _, profile := range cfg.WorkstreamProfiles {
		for _, sample := range profile.RouteSamples {
			if seen[sample] {
				continue
			}
			samples = append(samples, sample)
			seen[sample] = true
		}
	}
	if len(samples) > 0 {
		return samples
	}
	return gpuaasCompatibilityRouteSamples(cfg)
}

func gpuaasCompatibilityRouteSamples(cfg config.Config) []string {
	// Compatibility path for early GPUaaS parity configs. Generic adoption should
	// use [[workstream_profiles]].route_samples instead.
	if strings.EqualFold(cfg.Fairway.ProjectName, "gpuaas") || config.RoleSet(cfg)["A-backend"] {
		return []string{
			"doc/api/openapi.draft.yaml",
			"cmd/api/routes.go",
			"packages/services/billing/service.go",
			"cmd/node-agent/main.go",
			"scripts/ci/contracts_validate.sh",
		}
	}
	return nil
}

func validateAdoptionRegressionCatalog(path string) adoptionRegressionPack {
	if path == "" {
		path = defaultRegressionCatalogPath()
	}
	result := adoptionRegressionPack{CatalogPath: path}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.Issues = append(result.Issues, "catalog not found")
			return result
		}
		result.Issues = append(result.Issues, err.Error())
		return result
	}
	catalog, err := loadRegressionCatalog(path)
	if err != nil {
		result.Issues = append(result.Issues, err.Error())
		return result
	}
	result.OK = true
	result.PackCount = len(catalog.Packs)
	return result
}

func isTerminal(status string, terminal []string) bool {
	for _, value := range terminal {
		if status == value {
			return true
		}
	}
	return false
}

func printAdoptionArtifact(artifact adoptionArtifact) {
	title := "Adoption"
	if artifact.ArtifactType == "parity" {
		title = "Parity"
	}
	fmt.Printf("# Fairway %s Artifact\n\nproject: %s\ngenerated_at: %s\n", title, artifact.Project, artifact.GeneratedAt)
	fmt.Printf("tasks: %d\nready: %d\n", artifact.TaskCount, artifact.ReadyCount)
	fmt.Println("\n## Gates")
	fmt.Printf("- evidence_before_done: %s\n", artifact.Gates.EvidenceBeforeDone)
	fmt.Printf("- review_before_done: %s\n", artifact.Gates.ReviewBeforeDone)
	fmt.Printf("- handoff_before_merge_ready: %s\n", artifact.Gates.HandoffBeforeMergeReady)
	fmt.Printf("- blocked_reason: %s\n", artifact.Gates.BlockedReason)
	if len(artifact.ProfileGates) > 0 {
		fmt.Println("\n## Profile Gates")
		for _, gate := range artifact.ProfileGates {
			label := gate.Name
			if gate.Profile != "" {
				label = gate.Profile + "/" + label
			}
			if gate.EvidenceType != "" {
				fmt.Printf("- %s: %s (%s, group: %s)\n", label, gate.Mode, gate.EvidenceType, fallback(gate.Group, "general"))
			} else {
				fmt.Printf("- %s: %s (group: %s)\n", label, gate.Mode, fallback(gate.Group, "general"))
			}
			var requirements []string
			if len(gate.TaskKinds) > 0 {
				requirements = append(requirements, "task kinds: "+strings.Join(gate.TaskKinds, ", "))
			}
			if gate.RequiredEvidenceCount > 0 {
				requirements = append(requirements, fmt.Sprintf("count >= %d", gate.RequiredEvidenceCount))
			}
			if len(gate.AcceptedResults) > 0 {
				requirements = append(requirements, "results: "+strings.Join(gate.AcceptedResults, ", "))
			}
			if gate.ArtifactRequired {
				requirements = append(requirements, "artifact required")
			}
			if gate.OwnerSignoffRequired {
				requirements = append(requirements, "owner signoff required")
			}
			if gate.ExpiresAfter != "" {
				requirements = append(requirements, "expires after "+gate.ExpiresAfter)
			}
			if len(requirements) > 0 {
				fmt.Printf("  requirements: %s\n", strings.Join(requirements, "; "))
			}
		}
	}
	if len(artifact.GateEvaluations) > 0 {
		fmt.Println("\n## Gate Evaluation")
		for _, evaluation := range artifact.GateEvaluations {
			label := evaluation.Profile + "/" + evaluation.Gate
			fmt.Printf("- %s: %s (%d/%d satisfied)\n", label, evaluation.Status, evaluation.SatisfiedCount, evaluation.TaskCount)
			for _, miss := range evaluation.Missing {
				fmt.Printf("  - %s [%s] %s: %s\n", miss.TaskID, miss.Kind, miss.Title, strings.Join(miss.Reasons, "; "))
			}
			if evaluation.MissingCount > len(evaluation.Missing) {
				fmt.Printf("  - ... %d more missing\n", evaluation.MissingCount-len(evaluation.Missing))
			}
		}
	}
	if len(artifact.ReadyByRole) > 0 {
		fmt.Println("\n## Ready By Role")
		roles := make([]string, 0, len(artifact.ReadyByRole))
		for role := range artifact.ReadyByRole {
			roles = append(roles, role)
		}
		sort.Strings(roles)
		for _, role := range roles {
			fmt.Printf("- %s: %d\n", role, artifact.ReadyByRole[role])
		}
	}
	if len(artifact.ReadySample) > 0 {
		fmt.Println("\n## Ready Sample")
		for _, task := range artifact.ReadySample {
			fmt.Printf("- %s [%s] %s\n", task.ID, task.Role, task.Title)
		}
	}
	if len(artifact.RouteSamples) > 0 {
		fmt.Println("\n## Review Route Samples")
		for _, sample := range artifact.RouteSamples {
			if sample.OK {
				fmt.Printf("- %s -> %s (%s)\n", sample.Path, sample.Reviewer, sample.Reason)
			} else {
				fmt.Printf("- %s -> no match\n", sample.Path)
			}
		}
	}
	fmt.Println("\n## Regression Packs")
	if artifact.RegressionPacks.OK {
		fmt.Printf("- valid %s (%d packs)\n", artifact.RegressionPacks.CatalogPath, artifact.RegressionPacks.PackCount)
	} else {
		fmt.Printf("- not valid %s\n", artifact.RegressionPacks.CatalogPath)
		for _, issue := range artifact.RegressionPacks.Issues {
			fmt.Printf("  - %s\n", issue)
		}
	}
	fmt.Println("\n## Health")
	fmt.Printf("- in_progress: %d\n- stale_in_progress: %d\n- blocked_over_24h: %d\n- unacknowledged_handoffs: %d\n- unrouted_reviews: %d\n",
		artifact.Health.InProgress, artifact.Health.StaleInProgress, artifact.Health.BlockedOver24h, artifact.Health.UnacknowledgedHandoff, artifact.Health.UnroutedReviews)
	if len(artifact.EvidenceGaps) > 0 {
		fmt.Println("\n## Evidence Gaps")
		fmt.Printf("showing %d of %d\n", len(artifact.EvidenceGaps), artifact.EvidenceGapCount)
		for _, gap := range artifact.EvidenceGaps {
			fmt.Printf("- %s\n", gap)
		}
	}
	if len(artifact.Coordinator.Issues) > 0 {
		fmt.Println("\n## Coordinator Issues")
		for _, issue := range artifact.Coordinator.Issues {
			fmt.Printf("- %s\n", issue)
		}
	}
	if len(artifact.Coordinator.Recommendations) > 0 {
		fmt.Println("\n## Recommendations")
		for _, recommendation := range artifact.Coordinator.Recommendations {
			fmt.Printf("- %s\n", recommendation)
		}
	}
}

func cmdCheckpoint(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("checkpoint requires subcommand: record, status, stale")
	}
	if isHelpOnly(args) {
		subcommandUsage("checkpoint", "record|status|stale")
		return nil
	}
	switch args[0] {
	case "record":
		return cmdCheckpointRecord(ctx, opts, args[1:])
	case "status":
		return cmdCheckpointStatus(ctx, opts, args[1:], false)
	case "stale":
		return cmdCheckpointStatus(ctx, opts, args[1:], true)
	default:
		return fmt.Errorf("unknown checkpoint subcommand %q", args[0])
	}
}

func cmdCheckpointRecord(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("checkpoint record requires task id")
	}
	fs := flag.NewFlagSet("checkpoint record", flag.ContinueOnError)
	stateFlag := fs.String("state", "active", "checkpoint state")
	owner := fs.String("owner", "", "owner role or lane")
	target := fs.String("target-close-by", "", "target close date")
	summary := fs.String("summary", "", "summary")
	artifact := fs.String("artifact", "", "artifact path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected checkpoint record arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		cp := store.Checkpoint{TaskID: args[0], State: *stateFlag, Owner: *owner, TargetCloseBy: *target, Summary: *summary, ArtifactPath: *artifact}
		if err := s.RecordCheckpoint(ctx, cp); err != nil {
			return err
		}
		fmt.Println("checkpoint recorded", args[0])
		return nil
	})
}

func cmdCheckpointStatus(ctx context.Context, opts globalOptions, args []string, stale bool) error {
	fs := flag.NewFlagSet("checkpoint status", flag.ContinueOnError)
	all := fs.Bool("all", false, "include closed checkpoints")
	before := fs.String("before", time.Now().UTC().Format("2006-01-02"), "stale before date")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected checkpoint arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		staleBefore := ""
		if stale {
			staleBefore = *before
		}
		checkpoints, err := s.Checkpoints(ctx, staleBefore, *all)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(checkpoints)
		}
		for _, cp := range checkpoints {
			fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\n", cp.ID, cp.TaskID, cp.State, cp.Owner, cp.TargetCloseBy, cp.Summary)
		}
		return nil
	})
}

func cmdMemory(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("memory requires subcommand: show, update, append, packet, stale")
	}
	if args[0] == "--help" || args[0] == "-h" {
		subcommandUsage("memory", "show|update|append|packet|stale")
		return nil
	}
	switch args[0] {
	case "show":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			subcommandUsage("memory", "show [--track <track-id>]")
			return nil
		}
		return cmdMemoryShow(ctx, opts, args[1:])
	case "update":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			subcommandUsage("memory", "update --track <track-id> [fields]")
			return nil
		}
		return cmdMemoryUpsert(ctx, opts, args[1:], false)
	case "append":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			subcommandUsage("memory", "append --track <track-id> [fields]")
			return nil
		}
		return cmdMemoryUpsert(ctx, opts, args[1:], true)
	case "packet":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			subcommandUsage("memory", "packet --track <track-id> [--for <provider>]")
			return nil
		}
		return cmdMemoryPacket(ctx, opts, args[1:])
	case "stale":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			subcommandUsage("memory", "stale [--older-than <duration>]")
			return nil
		}
		return cmdMemoryStale(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown memory subcommand %q", args[0])
	}
}

func cmdWait(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("wait requires subcommand: add, ack, list, tick, wake")
	}
	if args[0] == "--help" || args[0] == "-h" {
		subcommandUsage("wait", "add|ack|list|tick|wake [--task <task-id>] [--stale] [--kind <kind>]")
		return nil
	}
	switch args[0] {
	case "add":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			subcommandUsage("wait", "add --task <task-id> --track <track-id> --on <condition> [--kind <kind>] [--target <target>] [--deadline <time>] [--deadline-source <origin>] [--action <action>] [--reason <text>] [--suggested-command <cmd>]")
			return nil
		}
		return cmdWaitAdd(ctx, opts, args[1:])
	case "ack":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			subcommandUsage("wait", "ack <wait-id> [--reason <text>] [--actor <role-or-track>]")
			return nil
		}
		return cmdWaitAck(ctx, opts, args[1:])
	case "list", "tick":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			subcommandUsage("wait", args[0]+" [--task <task-id>] [--stale] [--kind <kind>]")
			return nil
		}
		return cmdWaitList(ctx, opts, args[1:], args[0] == "tick")
	case "wake":
		return cmdWaitWake(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown wait subcommand %q", args[0])
	}
}

const manualWaitSummaryPrefix = "fairway_wait:"

type waitRow struct {
	WaitID           string `json:"wait_id"`
	Kind             string `json:"kind"`
	TaskID           string `json:"task_id,omitempty"`
	TrackID          string `json:"track_id,omitempty"`
	Owner            string `json:"owner,omitempty"`
	Condition        string `json:"condition,omitempty"`
	Target           string `json:"target,omitempty"`
	State            string `json:"state"`
	Action           string `json:"action"`
	Reason           string `json:"reason"`
	Stale            bool   `json:"stale,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	Deadline         string `json:"deadline,omitempty"`
	DeadlineSource   string `json:"deadline_source,omitempty"`
	StaleAge         string `json:"stale_age,omitempty"`
	LastWakeAttempt  string `json:"last_wake_attempt_at,omitempty"`
	Source           string `json:"source"`
	SuggestedCommand string `json:"suggested_command,omitempty"`
}

type manualWaitPayload struct {
	Event            string `json:"event"`
	WaitID           string `json:"wait_id"`
	Kind             string `json:"kind"`
	TaskID           string `json:"task_id"`
	TrackID          string `json:"track_id"`
	Condition        string `json:"condition"`
	Target           string `json:"target,omitempty"`
	Action           string `json:"action,omitempty"`
	Reason           string `json:"reason,omitempty"`
	DeadlineSource   string `json:"deadline_source,omitempty"`
	SuggestedCommand string `json:"suggested_command,omitempty"`
	AckReason        string `json:"ack_reason,omitempty"`
}

func cmdWaitAdd(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("wait add", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	trackID := fs.String("track", "", "durable track or owner waiting on this condition")
	kind := fs.String("kind", "generic", "wait kind")
	condition := fs.String("on", "", "wait condition")
	target := fs.String("target", "", "provider target, thread id, external run id, or actor this wait is waiting on")
	deadline := fs.String("deadline", "", "deadline or expected closeout time")
	targetCloseBy := fs.String("target-close-by", "", "alias for --deadline")
	deadlineSource := fs.String("deadline-source", "manual", "deadline or ack timeout origin")
	action := fs.String("action", "", "next action while wait is open")
	reason := fs.String("reason", "", "wait reason")
	suggestedCommand := fs.String("suggested-command", "", "suggested deterministic command")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected wait add arguments: %s", strings.Join(fs.Args(), " "))
	}
	*taskID = strings.TrimSpace(*taskID)
	*trackID = strings.TrimSpace(*trackID)
	*kind = strings.TrimSpace(*kind)
	*condition = strings.TrimSpace(*condition)
	if *taskID == "" {
		return errors.New("--task is required")
	}
	if *trackID == "" {
		return errors.New("--track is required")
	}
	if *condition == "" {
		return errors.New("--on is required")
	}
	if *kind == "" {
		*kind = "generic"
	}
	deadlineValue := strings.TrimSpace(*deadline)
	if deadlineValue == "" {
		deadlineValue = strings.TrimSpace(*targetCloseBy)
	}
	if deadlineValue != "" {
		if _, err := parseFlexibleTime(deadlineValue); err != nil {
			return fmt.Errorf("invalid --deadline %q: %w", deadlineValue, err)
		}
	}
	deadlineSourceValue := strings.TrimSpace(*deadlineSource)
	if deadlineSourceValue == "" {
		deadlineSourceValue = "manual"
	}
	waitID := manualWaitID(*taskID, *kind, *trackID, *condition)
	payload := manualWaitPayload{
		Event:            "add",
		WaitID:           waitID,
		Kind:             *kind,
		TaskID:           *taskID,
		TrackID:          *trackID,
		Condition:        *condition,
		Target:           strings.TrimSpace(*target),
		Action:           firstNonEmpty(strings.TrimSpace(*action), "inspect_wait"),
		Reason:           firstNonEmpty(strings.TrimSpace(*reason), "waiting on "+*condition),
		DeadlineSource:   deadlineSourceValue,
		SuggestedCommand: strings.TrimSpace(*suggestedCommand),
	}
	if payload.SuggestedCommand == "" {
		payload.SuggestedCommand = fmt.Sprintf("fairway wait list --task %s --kind %s", payload.TaskID, payload.Kind)
	}
	summary, err := encodeManualWaitSummary(payload)
	if err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if err := s.RecordCheckpoint(ctx, store.Checkpoint{
			TaskID:        payload.TaskID,
			State:         "awaiting_input",
			Owner:         payload.TrackID,
			TargetCloseBy: deadlineValue,
			Summary:       summary,
		}); err != nil {
			return err
		}
		row := waitRow{
			WaitID:           payload.WaitID,
			Kind:             payload.Kind,
			TaskID:           payload.TaskID,
			TrackID:          payload.TrackID,
			Owner:            payload.TrackID,
			Condition:        payload.Condition,
			Target:           payload.Target,
			State:            "open",
			Action:           payload.Action,
			Reason:           payload.Reason,
			Deadline:         deadlineValue,
			DeadlineSource:   payload.DeadlineSource,
			Source:           "manual_wait",
			SuggestedCommand: payload.SuggestedCommand,
		}
		if opts.JSON {
			return printJSON(row)
		}
		fmt.Printf("wait added %s kind=%s task=%s track=%s condition=%s\n", row.WaitID, row.Kind, row.TaskID, row.TrackID, row.Condition)
		if row.SuggestedCommand != "" {
			fmt.Printf("suggested_command: %s\n", row.SuggestedCommand)
		}
		return nil
	})
}

func cmdWaitAck(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("wait ack requires wait id")
	}
	waitID := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("wait ack", flag.ContinueOnError)
	reason := fs.String("reason", "", "acknowledgement reason")
	actor := fs.String("actor", "", "actor recording the acknowledgement")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected wait ack arguments: %s", strings.Join(fs.Args(), " "))
	}
	if waitID == "" {
		return errors.New("wait ack requires wait id")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		rows, err := projectedWaitRows(ctx, cfg, root, s, 24*time.Hour)
		if err != nil {
			return err
		}
		var row waitRow
		for _, candidate := range rows {
			if candidate.WaitID == waitID && candidate.Source == "manual_wait" {
				row = candidate
				break
			}
		}
		if row.WaitID == "" {
			return fmt.Errorf("manual wait %q not found or already acknowledged", waitID)
		}
		payload := manualWaitPayload{
			Event:     "ack",
			WaitID:    row.WaitID,
			Kind:      row.Kind,
			TaskID:    row.TaskID,
			TrackID:   firstNonEmpty(strings.TrimSpace(*actor), row.TrackID, row.Owner),
			Condition: row.Condition,
			Target:    row.Target,
			AckReason: strings.TrimSpace(*reason),
		}
		if payload.AckReason == "" {
			payload.AckReason = "acknowledged"
		}
		summary, err := encodeManualWaitSummary(payload)
		if err != nil {
			return err
		}
		if err := s.RecordCheckpoint(ctx, store.Checkpoint{
			TaskID:  row.TaskID,
			State:   "done",
			Owner:   payload.TrackID,
			Summary: summary,
		}); err != nil {
			return err
		}
		row.State = "acknowledged"
		row.Action = "none"
		row.Reason = payload.AckReason
		row.Stale = false
		row.StaleAge = ""
		if opts.JSON {
			return printJSON(row)
		}
		fmt.Printf("wait acknowledged %s task=%s track=%s reason=%s\n", row.WaitID, row.TaskID, firstNonEmpty(payload.TrackID, "none"), payload.AckReason)
		return nil
	})
}

func cmdWaitList(ctx context.Context, opts globalOptions, args []string, tick bool) error {
	fs := flag.NewFlagSet("wait list", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	kind := fs.String("kind", "", "wait kind")
	staleOnly := fs.Bool("stale", false, "show only stale waits")
	staleMemoryAfter := fs.Duration("memory-stale-after", 24*time.Hour, "track memory stale threshold")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected wait arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		rows, err := projectedWaitRows(ctx, cfg, root, s, *staleMemoryAfter)
		if err != nil {
			return err
		}
		rows = filterWaitRows(rows, *taskID, *kind, *staleOnly)
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].Stale != rows[j].Stale {
				return rows[i].Stale
			}
			if rows[i].TaskID != rows[j].TaskID {
				return rows[i].TaskID < rows[j].TaskID
			}
			return rows[i].WaitID < rows[j].WaitID
		})
		if opts.JSON {
			return printJSON(rows)
		}
		if tick {
			fmt.Println("wait_tick: dry-run")
		} else {
			fmt.Println("waits:")
		}
		if len(rows) == 0 {
			fmt.Println("- none")
			return nil
		}
		for _, row := range rows {
			stale := ""
			if row.Stale {
				stale = " stale=true"
			}
			target := ""
			if row.Target != "" {
				target = " target=" + row.Target
			}
			deadline := ""
			if row.Deadline != "" {
				deadline = " deadline=" + row.Deadline
			}
			deadlineSource := ""
			if row.DeadlineSource != "" {
				deadlineSource = " deadline_source=" + row.DeadlineSource
			}
			staleAge := ""
			if row.StaleAge != "" {
				staleAge = " stale_age=" + row.StaleAge
			}
			lastWake := ""
			if row.LastWakeAttempt != "" {
				lastWake = " last_wake_attempt=" + row.LastWakeAttempt
			}
			fmt.Printf("- %s kind=%s task=%s state=%s owner=%s action=%s source=%s%s%s%s%s%s%s reason=%s\n",
				row.WaitID, row.Kind, firstNonEmpty(row.TaskID, "none"), row.State, firstNonEmpty(row.Owner, "none"), row.Action, firstNonEmpty(row.Source, "unknown"), target, deadline, deadlineSource, stale, staleAge, lastWake, row.Reason)
			if row.SuggestedCommand != "" {
				fmt.Printf("  suggested_command: %s\n", row.SuggestedCommand)
			}
		}
		return nil
	})
}

type genericWaitWake struct {
	TaskID       string  `json:"task_id"`
	TaskStatus   string  `json:"task_status,omitempty"`
	WaitID       string  `json:"wait_id"`
	Kind         string  `json:"kind"`
	Owner        string  `json:"owner,omitempty"`
	Provider     string  `json:"provider,omitempty"`
	Target       string  `json:"target,omitempty"`
	TargetStatus string  `json:"target_status,omitempty"`
	TargetAction string  `json:"target_action,omitempty"`
	TargetReason string  `json:"target_reason,omitempty"`
	State        string  `json:"state,omitempty"`
	Prompt       string  `json:"prompt"`
	Signature    string  `json:"signature"`
	Wait         waitRow `json:"wait"`
	Suppressed   bool    `json:"suppressed,omitempty"`
	Error        string  `json:"error,omitempty"`
}

func cmdWaitWake(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("wait", "wake [--task <task-id>] [--kind <kind>] [--send] [--state <sent|notification_delivered|thread_steered>] [--provider <name>] [--target <thread-id>]")
		return nil
	}
	fs := flag.NewFlagSet("wait wake", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	kind := fs.String("kind", "", "wait kind")
	send := fs.Bool("send", false, "record bounded wake delivery/failure notification rows")
	state := fs.String("state", "sent", "notification state to record with --send")
	provider := fs.String("provider", "", "override provider label")
	target := fs.String("target", "", "override provider thread/adapter target")
	staleMemoryAfter := fs.Duration("memory-stale-after", 24*time.Hour, "track memory stale threshold")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected wait wake arguments: %s", strings.Join(fs.Args(), " "))
	}
	if !*send && (*provider != "" || *target != "" || *state != "sent") {
		return errors.New("--provider, --target, and --state require --send")
	}
	switch *state {
	case "sent", "notification_delivered", "thread_steered":
	default:
		return fmt.Errorf("invalid wait wake --state %q", *state)
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		rows, err := projectedWaitRows(ctx, cfg, root, s, *staleMemoryAfter)
		if err != nil {
			return err
		}
		rows = filterWaitRows(rows, *taskID, *kind, false)
		statuses, err := taskStatuses(ctx, s)
		if err != nil {
			return err
		}
		wakes := selectGenericWaitWakes(rows, cfg.ProviderTargets, statuses, terminalStatusSet(cfg.States.Terminal))
		for i := range wakes {
			if *provider != "" {
				wakes[i].Provider = *provider
			}
			if *target != "" {
				wakes[i].Target = *target
			}
			wakes[i].State = *state
			notifications, err := s.Notifications(ctx, wakes[i].TaskID)
			if err != nil {
				return err
			}
			if genericWaitWakeSuppressed(wakes[i], notifications) {
				wakes[i].Suppressed = true
				continue
			}
			if !*send {
				continue
			}
			recordState := *state
			reason := "generic_wait_wake signature=" + wakes[i].Signature + " kind=" + wakes[i].Kind + " wait_id=" + wakes[i].WaitID
			if strings.TrimSpace(wakes[i].Wait.TaskID) == "" {
				recordState = "notification_failed"
				reason += " failed=no_task_notification_target action=mapping_required"
				wakes[i].TargetAction = "mapping_required"
				wakes[i].Error = "taskless wait cannot record task notification delivery"
				wakes[i].State = recordState
				continue
			}
			if !wakeTargetRoutable(wakes[i].TargetStatus, wakes[i].Target) {
				recordState = "notification_failed"
				reason += " failed=no_wake_target action=mapping_required"
				if wakes[i].TargetReason != "" {
					reason += " target_reason=" + strings.ReplaceAll(wakes[i].TargetReason, " ", "_")
				}
				wakes[i].TargetAction = "mapping_required"
				wakes[i].Error = firstNonEmpty(wakes[i].TargetReason, "no wake target configured")
			}
			if _, err := s.RecordNotification(ctx, store.Notification{
				TaskID:   wakes[i].TaskID,
				Domain:   "coordinator",
				Provider: wakes[i].Provider,
				Target:   wakes[i].Target,
				State:    recordState,
				Reason:   reason,
			}); err != nil {
				return err
			}
			wakes[i].State = recordState
		}
		if opts.JSON {
			return printJSON(wakes)
		}
		printGenericWaitWakes(wakes)
		return nil
	})
}

func projectedWaitRows(ctx context.Context, cfg config.Config, root string, s *store.Store, staleMemoryAfter time.Duration) ([]waitRow, error) {
	worktrees, err := collectWorktreeStatus(cfg, root)
	if err != nil {
		worktrees = nil
	}
	ackTimeout, err := reviewWaitAckTimeout(cfg)
	if err != nil {
		return nil, err
	}
	plan, err := coord.BuildPlan(ctx, cfg, s, coord.PlanOptions{
		Worktrees:              coordinatorWorktreeFacts(worktrees),
		StaleCheckpointAfter:   2 * time.Hour,
		MonitorHandbackAfter:   2 * time.Hour,
		NotificationAckTimeout: ackTimeout,
		ReadyLimit:             10,
		RecommendationLimit:    50,
		UtilityMonitorAllowed:  true,
	})
	if err != nil {
		return nil, err
	}
	rows := waitRowsFromPlan(plan)
	reviewWaits, _, err := reviewWaitRowsWithTaskStatus(ctx, cfg, s, "")
	if err != nil {
		return nil, err
	}
	rows = append(rows, waitRowsFromReviewWaits(reviewWaits)...)
	memories, err := s.TrackMemories(ctx)
	if err != nil {
		return nil, err
	}
	rows = append(rows, waitRowsFromTrackMemory(memories, staleMemoryAfter)...)
	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return nil, err
	}
	notifications, err := s.Notifications(ctx, "")
	if err != nil {
		return nil, err
	}
	rows = append(rows, waitRowsFromManualCheckpoints(checkpoints, notifications)...)
	return dedupeWaitRows(rows), nil
}

func waitRowsFromReviewWaits(waits []reviewstate.ReviewWait) []waitRow {
	rows := make([]waitRow, 0, len(waits))
	for _, rw := range waits {
		state := strings.TrimSpace(rw.State)
		if state == "" {
			state = "open"
		}
		rows = append(rows, waitRow{
			WaitID:           firstNonEmpty(rw.WaitID, "review:"+rw.TaskID+":"+rw.Domain),
			Kind:             "review",
			TaskID:           rw.TaskID,
			Owner:            rw.Domain,
			State:            state,
			Action:           rw.Action,
			Reason:           rw.Reason,
			Stale:            state == "stale" || state == "notification_failed",
			Source:           "review_waits",
			SuggestedCommand: suggestedReviewWaitCommand(rw),
		})
	}
	return rows
}

func dedupeWaitRows(rows []waitRow) []waitRow {
	seen := map[string]bool{}
	var out []waitRow
	for _, row := range rows {
		key := strings.Join([]string{row.Source, row.WaitID, row.TaskID, row.Kind}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, row)
	}
	return out
}

func selectGenericWaitWakes(rows []waitRow, targets []config.ProviderTarget, statuses map[string]string, terminal map[string]bool) []genericWaitWake {
	var wakes []genericWaitWake
	for _, row := range rows {
		notificationTaskID := strings.TrimSpace(row.TaskID)
		if notificationTaskID == "" && row.Kind == "track_memory" {
			notificationTaskID = row.WaitID
		}
		if notificationTaskID == "" {
			continue
		}
		taskStatus := strings.TrimSpace(statuses[row.TaskID])
		if row.TaskID != "" && terminal[taskStatus] {
			continue
		}
		if !row.Stale && row.State != "failed" && row.State != "notification_failed" && !providerSessionWakeCandidate(row) {
			continue
		}
		targetInfo := resolveWakeTarget(targets, row.Owner)
		wake := genericWaitWake{
			TaskID:       notificationTaskID,
			TaskStatus:   taskStatus,
			WaitID:       row.WaitID,
			Kind:         row.Kind,
			Owner:        row.Owner,
			Provider:     targetInfo.Provider,
			Target:       targetInfo.Target,
			TargetStatus: targetInfo.Status,
			TargetAction: targetInfo.Action,
			TargetReason: targetInfo.Reason,
			State:        "sent",
			Wait:         row,
		}
		wake.Signature = genericWaitWakeSignature(wake)
		wake.Prompt = renderGenericWaitWakePrompt(wake)
		wakes = append(wakes, wake)
	}
	sort.SliceStable(wakes, func(i, j int) bool {
		if wakes[i].TaskID != wakes[j].TaskID {
			return wakes[i].TaskID < wakes[j].TaskID
		}
		return wakes[i].Signature < wakes[j].Signature
	})
	return wakes
}

func genericWaitWakeSignature(wake genericWaitWake) string {
	parts := []string{
		wake.TaskID,
		wake.Kind,
		"task_status=" + strings.TrimSpace(wake.TaskStatus),
		"wait_id=" + strings.TrimSpace(wake.WaitID),
		"state=" + strings.TrimSpace(wake.Wait.State),
		"action=" + strings.TrimSpace(wake.Wait.Action),
		"source=" + strings.TrimSpace(wake.Wait.Source),
	}
	return strings.Join(parts, "|")
}

func providerSessionWakeCandidate(row waitRow) bool {
	return row.Kind == "provider_session" && row.Action == "record_provider_event_checkpoint"
}

func renderGenericWaitWakePrompt(wake genericWaitWake) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Generic wait wake for %s:\n", wake.TaskID)
	if wake.TaskStatus != "" {
		fmt.Fprintf(&b, "Task status: %s\n", wake.TaskStatus)
	}
	fmt.Fprintf(&b, "Wait: %s kind=%s state=%s owner=%s\n", wake.WaitID, wake.Kind, wake.Wait.State, firstNonEmpty(wake.Owner, "unknown"))
	fmt.Fprintf(&b, "Action: %s\n", firstNonEmpty(wake.Wait.Action, "inspect_wait"))
	if wake.Wait.Reason != "" {
		fmt.Fprintf(&b, "Reason: %s\n", wake.Wait.Reason)
	}
	if wake.Wait.SuggestedCommand != "" {
		fmt.Fprintf(&b, "Suggested command: %s\n", wake.Wait.SuggestedCommand)
	}
	if wake.TargetReason != "" {
		fmt.Fprintf(&b, "Wake target: mapping_required reason=%s\n", wake.TargetReason)
	}
	b.WriteString("\nNext action:\n")
	if wake.Wait.TaskID != "" {
		fmt.Fprintf(&b, "1. Re-run fairway wait list --task %s --kind %s.\n", wake.Wait.TaskID, wake.Kind)
	} else {
		fmt.Fprintf(&b, "1. Re-run fairway wait list --kind %s.\n", wake.Kind)
	}
	b.WriteString("2. Re-run the source-specific command named above before acting.\n")
	b.WriteString("3. Do not treat this wake as approval, merge, deploy, live execution, or dashboard send authority.\n")
	return b.String()
}

func genericWaitWakeSuppressed(wake genericWaitWake, notifications []store.Notification) bool {
	needle := "generic_wait_wake signature=" + wake.Signature
	for _, notification := range notifications {
		if notification.Domain == "coordinator" && strings.Contains(notification.Reason, needle) {
			switch notification.State {
			case "sent", "notification_delivered", "thread_steered", "acknowledged", "review_acknowledged":
				return true
			}
		}
	}
	return false
}

func printGenericWaitWakes(wakes []genericWaitWake) {
	if len(wakes) == 0 {
		fmt.Println("generic_wait_wakes: none")
		return
	}
	fmt.Println("generic_wait_wakes:")
	for _, wake := range wakes {
		status := "ready"
		if wake.Suppressed {
			status = "suppressed"
		}
		if wake.Error != "" {
			status = "failed"
		}
		targetNote := ""
		if wake.TargetAction != "" {
			targetNote = " target_action=" + wake.TargetAction
		}
		fmt.Printf("- %s kind=%s wait=%s status=%s task_status=%s provider=%s target=%s signature=%s%s\n", wake.TaskID, wake.Kind, wake.WaitID, status, firstNonEmpty(wake.TaskStatus, "unknown"), firstNonEmpty(wake.Provider, "none"), firstNonEmpty(wake.Target, "none"), wake.Signature, targetNote)
		fmt.Print(wake.Prompt)
		if !strings.HasSuffix(wake.Prompt, "\n") {
			fmt.Println()
		}
	}
}

func waitRowsFromPlan(plan coord.Plan) []waitRow {
	var rows []waitRow
	for _, rw := range plan.ReviewWaits {
		state := strings.TrimSpace(rw.State)
		if state == "" {
			state = "open"
		}
		rows = append(rows, waitRow{
			WaitID:           firstNonEmpty(rw.WaitID, "review:"+rw.TaskID+":"+rw.Domain),
			Kind:             "review",
			TaskID:           rw.TaskID,
			Owner:            rw.Domain,
			State:            state,
			Action:           rw.Action,
			Reason:           rw.Reason,
			Stale:            state == "stale" || state == "notification_failed",
			Source:           "review_waits",
			SuggestedCommand: suggestedReviewWaitCommand(rw),
		})
	}
	for _, handback := range plan.CompletionHandbacks {
		state := "open"
		if handback.Stale {
			state = "stale"
		}
		if strings.Contains(handback.DeliveryStatus, "failed") || strings.Contains(handback.DeliveryState, "failed") {
			state = "failed"
		}
		rows = append(rows, waitRow{
			WaitID:           fmt.Sprintf("completion:%d", handback.HandoffID),
			Kind:             "completion_handback",
			TaskID:           handback.TaskID,
			Owner:            handback.ToRole,
			State:            state,
			Action:           firstNonEmpty(handback.SuggestedAction, "record_delivery_or_next_decision"),
			Reason:           firstNonEmpty(handback.NextAction, handback.DeliveryStatus),
			Stale:            handback.Stale || state == "failed",
			Source:           "completion_handbacks",
			SuggestedCommand: handback.SuggestedCommand,
		})
	}
	for _, action := range plan.Actions {
		if action.ReviewWait != nil || action.CompletionHandback != nil {
			continue
		}
		kind := waitKindFromAction(action)
		if kind == "" {
			continue
		}
		state := "open"
		stale := strings.Contains(action.Action, "stale") || strings.Contains(action.Reason, "stale") || strings.Contains(action.Reason, "missed")
		if stale {
			state = "stale"
		}
		if strings.Contains(action.Action, "failed") || strings.Contains(action.Reason, "failed") {
			state = "failed"
			stale = true
		}
		rows = append(rows, waitRow{
			WaitID: waitIDForAction(action, kind),
			Kind:   kind,
			TaskID: action.TaskID,
			Owner:  action.Role,
			State:  state,
			Action: action.Action,
			Reason: action.Reason,
			Stale:  stale,
			Source: "coordinator_plan",
		})
	}
	return rows
}

func waitRowsFromTrackMemory(memories []store.TrackMemory, staleAfter time.Duration) []waitRow {
	var rows []waitRow
	cutoff := time.Now().UTC().Add(-staleAfter)
	for _, mem := range memories {
		updated, err := time.Parse(time.RFC3339Nano, mem.UpdatedAt)
		stale := err != nil || updated.Before(cutoff)
		if !stale {
			continue
		}
		rows = append(rows, waitRow{
			WaitID:           "memory:" + mem.TrackID,
			Kind:             "track_memory",
			Owner:            mem.TrackID,
			State:            "stale",
			Action:           "refresh_track_memory",
			Reason:           "track memory is older than configured stale threshold",
			Stale:            true,
			Source:           "track_memory",
			SuggestedCommand: "fairway memory update --track " + mem.TrackID + " --from-checkpoints",
		})
	}
	return rows
}

func waitRowsFromManualCheckpoints(checkpoints []store.Checkpoint, notifications []store.Notification) []waitRow {
	type manualWaitState struct {
		payload    manualWaitPayload
		checkpoint store.Checkpoint
		acked      bool
		ackAt      string
	}
	states := map[string]manualWaitState{}
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		payload, ok := parseManualWaitSummary(cp.Summary)
		if !ok || payload.WaitID == "" {
			continue
		}
		current := states[payload.WaitID]
		switch payload.Event {
		case "ack":
			current.acked = true
			current.ackAt = cp.CreatedAt
			if current.payload.WaitID == "" {
				current.payload = payload
				current.checkpoint = cp
			}
		case "add":
			current.payload = payload
			current.checkpoint = cp
			current.acked = false
			current.ackAt = ""
		}
		states[payload.WaitID] = current
	}
	lastWake := lastGenericWaitWakeAttempts(notifications)
	now := time.Now().UTC()
	rows := make([]waitRow, 0, len(states))
	for _, state := range states {
		if state.acked || state.payload.WaitID == "" {
			continue
		}
		payload := state.payload
		row := waitRow{
			WaitID:           payload.WaitID,
			Kind:             payload.Kind,
			TaskID:           payload.TaskID,
			TrackID:          payload.TrackID,
			Owner:            payload.TrackID,
			Condition:        payload.Condition,
			Target:           payload.Target,
			State:            "open",
			Action:           firstNonEmpty(payload.Action, "inspect_wait"),
			Reason:           firstNonEmpty(payload.Reason, "waiting on "+payload.Condition),
			CreatedAt:        state.checkpoint.CreatedAt,
			Deadline:         state.checkpoint.TargetCloseBy,
			DeadlineSource:   firstNonEmpty(payload.DeadlineSource, "manual"),
			Source:           "manual_wait",
			SuggestedCommand: firstNonEmpty(payload.SuggestedCommand, fmt.Sprintf("fairway wait list --task %s --kind %s", payload.TaskID, payload.Kind)),
			LastWakeAttempt:  lastWake[payload.WaitID],
		}
		if row.Kind == "" {
			row.Kind = "generic"
		}
		if row.Deadline != "" {
			if deadline, err := parseFlexibleTime(row.Deadline); err == nil && now.After(deadline) {
				row.State = "stale"
				row.Stale = true
				row.StaleAge = roundDuration(now.Sub(deadline)).String()
				if row.Action == "inspect_wait" {
					row.Action = "wake_or_ack_wait"
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func lastGenericWaitWakeAttempts(notifications []store.Notification) map[string]string {
	out := map[string]string{}
	for _, notification := range notifications {
		if notification.Domain != "coordinator" || !strings.Contains(notification.Reason, "generic_wait_wake") {
			continue
		}
		waitID := notificationReasonField(notification.Reason, "wait_id")
		if waitID == "" {
			continue
		}
		if out[waitID] == "" || notification.CreatedAt > out[waitID] {
			out[waitID] = notification.CreatedAt
		}
	}
	return out
}

func notificationReasonField(reason, field string) string {
	prefix := field + "="
	for _, token := range strings.Fields(reason) {
		if strings.HasPrefix(token, prefix) {
			return strings.TrimPrefix(token, prefix)
		}
	}
	return ""
}

func manualWaitID(taskID, kind, trackID, condition string) string {
	parts := []string{"manual", taskID, kind, trackID, condition}
	for i := range parts {
		parts[i] = slugToken(parts[i])
	}
	return strings.Join(parts, ":")
}

func slugToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "none"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "none"
	}
	return out
}

func encodeManualWaitSummary(payload manualWaitPayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return manualWaitSummaryPrefix + string(data), nil
}

func parseManualWaitSummary(summary string) (manualWaitPayload, bool) {
	summary = strings.TrimSpace(summary)
	if !strings.HasPrefix(summary, manualWaitSummaryPrefix) {
		return manualWaitPayload{}, false
	}
	var payload manualWaitPayload
	if err := json.Unmarshal([]byte(strings.TrimPrefix(summary, manualWaitSummaryPrefix)), &payload); err != nil {
		return manualWaitPayload{}, false
	}
	return payload, true
}

func waitKindFromAction(action coord.PlanAction) string {
	switch action.Classification {
	case "review-gated", "review-notification":
		return "review"
	case "completion-handback":
		return "completion_handback"
	case "live-window":
		return "live_window"
	case "approval-gated":
		return "approval"
	case "waiting":
		if strings.Contains(strings.ToLower(action.Reason), "approval") {
			return "approval"
		}
		if action.SessionID != "" {
			return "provider_session"
		}
		return "checkpoint"
	case "active-reconcile", "session":
		return "provider_session"
	case "stale":
		if action.SessionID != "" {
			return "provider_session"
		}
	case "utility-monitor":
		return "monitor"
	}
	if action.WatcherID != "" {
		return "monitor"
	}
	if action.LiveWindow != nil {
		return "live_window"
	}
	return ""
}

func waitIDForAction(action coord.PlanAction, kind string) string {
	parts := []string{kind, firstNonEmpty(action.TaskID, action.SessionID, action.WatcherID, "none"), action.Action}
	return strings.Join(parts, ":")
}

func suggestedReviewWaitCommand(rw reviewstate.ReviewWait) string {
	switch rw.Action {
	case "deliver_notification", "nudge_reviewer":
		return fmt.Sprintf("fairway record notification %s --domain %s --state thread_steered --provider <provider> --target <target>", rw.TaskID, rw.Domain)
	case "mapping_required":
		return "update review/provider routing for domain " + rw.Domain
	case "resolve":
		return fmt.Sprintf("fairway record review %s --domain %s --verdict approve|changes --reviewer <reviewer>", rw.TaskID, rw.Domain)
	default:
		return ""
	}
}

func filterWaitRows(rows []waitRow, taskID, kind string, staleOnly bool) []waitRow {
	var out []waitRow
	for _, row := range rows {
		if taskID != "" && row.TaskID != taskID {
			continue
		}
		if kind != "" && row.Kind != kind {
			continue
		}
		if staleOnly && !row.Stale {
			continue
		}
		out = append(out, row)
	}
	return out
}

func cmdMemoryShow(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("memory show", flag.ContinueOnError)
	track := fs.String("track", "", "track id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected memory show arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if *track != "" {
			mem, err := s.TrackMemory(ctx, *track)
			if err != nil {
				return err
			}
			return printTrackMemories(opts, []store.TrackMemory{mem})
		}
		memories, err := s.TrackMemories(ctx)
		if err != nil {
			return err
		}
		return printTrackMemories(opts, memories)
	})
}

func cmdMemoryUpsert(ctx context.Context, opts globalOptions, args []string, appendFields bool) error {
	fs := flag.NewFlagSet("memory", flag.ContinueOnError)
	track := fs.String("track", "", "track id")
	title := fs.String("title", "", "title")
	purpose := fs.String("purpose", "", "purpose")
	mode := fs.String("operating-mode", "", "operating mode")
	scope := fs.String("active-scope", "", "active scope")
	objective := fs.String("current-objective", "", "current objective")
	decision := multiStringFlag{}
	blocker := multiStringFlag{}
	question := multiStringFlag{}
	nextAction := multiStringFlag{}
	checkpointID := multiInt64Flag{}
	evidenceID := multiInt64Flag{}
	reviewID := multiInt64Flag{}
	fs.Var(&decision, "decision", "curated decision summary")
	fs.Var(&blocker, "blocker", "curated blocker summary")
	fs.Var(&question, "open-question", "curated open question")
	fs.Var(&nextAction, "next-action", "curated next action")
	fs.Var(&checkpointID, "source-checkpoint-id", "source checkpoint id")
	fs.Var(&evidenceID, "source-evidence-id", "source evidence id")
	fs.Var(&reviewID, "source-review-id", "source review id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected memory arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *track == "" {
		return errors.New("--track is required")
	}
	mem := store.TrackMemory{
		TrackID:             *track,
		Title:               *title,
		Purpose:             *purpose,
		OperatingMode:       *mode,
		ActiveScope:         *scope,
		CurrentObjective:    *objective,
		Decisions:           []string(decision),
		Blockers:            []string(blocker),
		OpenQuestions:       []string(question),
		NextActions:         []string(nextAction),
		SourceCheckpointIDs: []int64(checkpointID),
		SourceEvidenceIDs:   []int64(evidenceID),
		SourceReviewIDs:     []int64(reviewID),
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		updated, err := s.UpsertTrackMemory(ctx, mem, appendFields)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(updated)
		}
		fmt.Printf("memory updated %s\n", updated.TrackID)
		return nil
	})
}

func cmdMemoryPacket(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("memory packet", flag.ContinueOnError)
	track := fs.String("track", "", "track id")
	forProvider := fs.String("for", "", "target provider or surface")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected memory packet arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *track == "" {
		return errors.New("--track is required")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		mem, err := s.TrackMemory(ctx, *track)
		if err != nil {
			return err
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
		packet := buildMemoryPacket(mem, *forProvider, tasks, sessions, checkpoints, cfg.States.Terminal)
		if opts.JSON {
			return printJSON(packet)
		}
		printMemoryPacket(packet)
		return nil
	})
}

func cmdMemoryStale(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("memory stale", flag.ContinueOnError)
	olderThan := fs.Duration("older-than", 24*time.Hour, "stale threshold")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected memory stale arguments: %s", strings.Join(fs.Args(), " "))
	}
	cutoff := time.Now().UTC().Add(-*olderThan)
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		memories, err := s.TrackMemories(ctx)
		if err != nil {
			return err
		}
		var stale []store.TrackMemory
		for _, mem := range memories {
			updated, err := time.Parse(time.RFC3339Nano, mem.UpdatedAt)
			if err != nil || updated.Before(cutoff) {
				stale = append(stale, mem)
			}
		}
		return printTrackMemories(opts, stale)
	})
}

type memoryPacket struct {
	Track          store.TrackMemory `json:"track"`
	ForProvider    string            `json:"for_provider,omitempty"`
	ActiveTasks    []string          `json:"active_tasks,omitempty"`
	ActiveSessions []string          `json:"active_sessions,omitempty"`
	Blockers       []string          `json:"blockers,omitempty"`
	NextActions    []string          `json:"next_actions,omitempty"`
	Checkpoints    []string          `json:"checkpoints,omitempty"`
}

func buildMemoryPacket(mem store.TrackMemory, forProvider string, tasks []store.Task, sessions []store.Session, checkpoints []store.Checkpoint, terminal []string) memoryPacket {
	packet := memoryPacket{Track: mem, ForProvider: strings.TrimSpace(forProvider), Blockers: append([]string{}, mem.Blockers...), NextActions: append([]string{}, mem.NextActions...)}
	terminalSet := map[string]bool{}
	for _, state := range terminal {
		terminalSet[state] = true
	}
	for _, task := range tasks {
		if task.Status == "in_progress" || task.Status == "blocked" || task.Status == "todo" {
			packet.ActiveTasks = append(packet.ActiveTasks, task.Definition.ID+" "+task.Status+" "+task.Definition.Title)
		}
		if task.Status == "blocked" {
			packet.Blockers = append(packet.Blockers, task.Definition.ID+" blocked")
		}
		if !terminalSet[task.Status] && task.Status != "" {
			packet.NextActions = append(packet.NextActions, "inspect "+task.Definition.ID+" status="+task.Status)
		}
	}
	for _, session := range sessions {
		packet.ActiveSessions = append(packet.ActiveSessions, session.ID+" "+session.Status+" task="+firstNonEmpty(session.TaskID, "none"))
	}
	for _, checkpoint := range checkpoints {
		packet.Checkpoints = append(packet.Checkpoints, fmt.Sprintf("%s %s %s", checkpoint.TaskID, checkpoint.State, checkpoint.Summary))
		if len(packet.Checkpoints) >= 8 {
			break
		}
	}
	return packet
}

func printTrackMemories(opts globalOptions, memories []store.TrackMemory) error {
	if opts.JSON {
		return printJSON(memories)
	}
	if len(memories) == 0 {
		fmt.Println("track_memory: none")
		return nil
	}
	for _, mem := range memories {
		fmt.Printf("%s updated=%s title=%s objective=%s\n", mem.TrackID, mem.UpdatedAt, firstNonEmpty(mem.Title, "none"), firstNonEmpty(mem.CurrentObjective, "none"))
		if len(mem.Blockers) > 0 {
			fmt.Printf("  blockers: %s\n", strings.Join(mem.Blockers, "; "))
		}
		if len(mem.NextActions) > 0 {
			fmt.Printf("  next_actions: %s\n", strings.Join(mem.NextActions, "; "))
		}
	}
	return nil
}

func printMemoryPacket(packet memoryPacket) {
	mem := packet.Track
	fmt.Printf("# Track Memory Packet: %s\n\n", mem.TrackID)
	if packet.ForProvider != "" {
		fmt.Printf("for: %s\n", packet.ForProvider)
	}
	fmt.Printf("title: %s\npurpose: %s\noperating_mode: %s\nactive_scope: %s\ncurrent_objective: %s\nupdated_at: %s\n",
		mem.Title, mem.Purpose, mem.OperatingMode, mem.ActiveScope, mem.CurrentObjective, mem.UpdatedAt)
	printStringSection("Decisions", mem.Decisions)
	printStringSection("Blockers", packet.Blockers)
	printStringSection("Open Questions", mem.OpenQuestions)
	printStringSection("Next Actions", packet.NextActions)
	printStringSection("Active Tasks", packet.ActiveTasks)
	printStringSection("Active Sessions", packet.ActiveSessions)
	printStringSection("Recent Checkpoints", packet.Checkpoints)
	if len(mem.SourceCheckpointIDs)+len(mem.SourceEvidenceIDs)+len(mem.SourceReviewIDs) > 0 {
		fmt.Println("\n## Source Fact References")
		fmt.Printf("- checkpoints: %s\n", formatInt64s(mem.SourceCheckpointIDs))
		fmt.Printf("- evidence: %s\n", formatInt64s(mem.SourceEvidenceIDs))
		fmt.Printf("- reviews: %s\n", formatInt64s(mem.SourceReviewIDs))
	}
}

func printStringSection(title string, values []string) {
	fmt.Printf("\n## %s\n", title)
	if len(values) == 0 {
		fmt.Println("- none")
		return
	}
	for _, value := range values {
		fmt.Printf("- %s\n", value)
	}
}

func formatInt64s(values []int64) string {
	if len(values) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatInt(value, 10))
	}
	return strings.Join(parts, ",")
}

type multiStringFlag []string

func (m *multiStringFlag) String() string { return strings.Join(*m, ",") }
func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

type multiInt64Flag []int64

func (m *multiInt64Flag) String() string { return formatInt64s([]int64(*m)) }
func (m *multiInt64Flag) Set(value string) error {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return fmt.Errorf("invalid id %q", value)
	}
	*m = append(*m, parsed)
	return nil
}

func cmdPacket(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("packet requires subcommand: context, bugfix, retry, watcher, release-run, template, rules, architecture-map, boundary-guard, vertical-slice")
	}
	if isHelpOnly(args) {
		subcommandUsage("packet", "context|bugfix|retry|watcher|release-run|template|rules|architecture-map|boundary-guard|vertical-slice")
		return nil
	}
	switch args[0] {
	case "context":
		return cmdPacketContext(ctx, opts, args[1:])
	case "bugfix":
		return cmdPacketBugfix(ctx, opts, args[1:])
	case "retry":
		return cmdPacketRetry(ctx, opts, args[1:])
	case "watcher":
		return cmdPacketWatcher(opts, args[1:])
	case "release-run":
		return cmdPacketReleaseRun(ctx, opts, args[1:])
	case "template":
		return cmdPacketTemplate(ctx, opts, args[1:])
	case "rules":
		return cmdPacketRules(ctx, opts, args[1:])
	case "architecture-map":
		return cmdPacketArchitectureMap(ctx, opts, args[1:])
	case "boundary-guard":
		return cmdPacketBoundaryGuard(ctx, opts, args[1:])
	case "vertical-slice":
		return cmdPacketVerticalSlice(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown packet subcommand %q", args[0])
	}
}

func cmdPacketTemplate(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 2 {
		return errors.New("packet template requires template name and task id")
	}
	templateName := args[0]
	taskID := args[1]
	fs := flag.NewFlagSet("packet template", flag.ContinueOnError)
	var fields multiFlag
	fs.Var(&fields, "field", "template field as key=value; may repeat")
	instantiateWaits := fs.Bool("instantiate-waits", false, "record rehearsal waits for packet checks that need follow-up")
	var childTasks multiFlag
	fs.Var(&childTasks, "child-task", "instantiate child task as id=field; may repeat")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet template arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		template, ok := packetTemplateByName(cfg.PacketTemplates, templateName)
		if !ok {
			return fmt.Errorf("packet template %q is not configured", templateName)
		}
		values, err := parsePacketTemplateFields(fields)
		if err != nil {
			return err
		}
		if err := validatePacketTemplateValues(template, values); err != nil {
			return err
		}
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		instantiatedWaits, instantiatedTasks, err := instantiatePacketTemplate(ctx, s, template, task, values, *instantiateWaits, childTasks)
		if err != nil {
			return err
		}
		packet := struct {
			Template          config.PacketTemplate  `json:"template"`
			Task              store.Task             `json:"task"`
			Fields            map[string][]string    `json:"fields"`
			Evidence          []store.Evidence       `json:"evidence"`
			Reviews           []store.Review         `json:"reviews"`
			Authorization     string                 `json:"authorization"`
			InstantiatedWaits []waitRow              `json:"instantiated_waits,omitempty"`
			InstantiatedTasks []store.TaskDefinition `json:"instantiated_tasks,omitempty"`
		}{template, task, values, evidence, reviews, packetTemplateAuthorization(template.Name), instantiatedWaits, instantiatedTasks}
		if opts.JSON {
			return printJSON(packet)
		}
		fmt.Printf("# %s Packet: %s\n\n", packetTemplateTitle(template.Name), task.Definition.ID)
		fmt.Printf("title: %s\nstatus: %s\nrole: %s\n", task.Definition.Title, task.Status, task.Definition.Role)
		if task.Definition.Profile != "" || task.Definition.OwningDomain != "" || task.Definition.RiskLevel != "" {
			fmt.Printf("profile: %s\ndomain: %s\nrisk: %s\n", task.Definition.Profile, task.Definition.OwningDomain, task.Definition.RiskLevel)
		}
		fmt.Println("\n## Fields")
		for _, field := range packetTemplateFieldOrder(template) {
			printPacketTemplateField(field, values[field])
		}
		fmt.Println("\n## Authorization Boundary")
		fmt.Println(packet.Authorization)
		if len(packet.InstantiatedWaits) > 0 {
			fmt.Println("\n## Instantiated Waits")
			for _, wait := range packet.InstantiatedWaits {
				fmt.Printf("- %s kind=%s owner=%s condition=%s action=%s\n", wait.WaitID, wait.Kind, firstNonEmpty(wait.Owner, "none"), wait.Condition, wait.Action)
			}
		}
		if len(packet.InstantiatedTasks) > 0 {
			fmt.Println("\n## Instantiated Child Tasks")
			for _, child := range packet.InstantiatedTasks {
				fmt.Printf("- %s %s\n", child.ID, child.Title)
			}
		}
		printEvidenceSummary(evidence)
		fmt.Println("\n## Reviews")
		for _, review := range reviews {
			fmt.Printf("- %s by %s: %s\n", review.Verdict, review.Reviewer, review.Reason)
		}
		return nil
	})
}

type retryPacket struct {
	Task                 store.Task       `json:"task"`
	Kind                 string           `json:"kind"`
	SourceSHA            string           `json:"source_sha"`
	OperatorSurface      string           `json:"operator_surface"`
	ArtifactDir          string           `json:"artifact_dir"`
	EvidenceContract     []string         `json:"evidence_contract"`
	AllowedActions       []string         `json:"allowed_actions"`
	ForbiddenActions     []string         `json:"forbidden_actions"`
	ExpiresAt            string           `json:"expires_at"`
	PriorFailureClosure  string           `json:"prior_failure_closure"`
	IterationCount       int              `json:"iteration_count,omitempty"`
	MeaningfulFailures   int              `json:"meaningful_failures,omitempty"`
	CoordinationFailures int              `json:"coordination_failures,omitempty"`
	RetryBudget          int              `json:"retry_budget,omitempty"`
	ResetTask            string           `json:"reset_task,omitempty"`
	ResetReason          string           `json:"reset_reason,omitempty"`
	NextAction           string           `json:"next_action,omitempty"`
	Authorization        string           `json:"authorization"`
	Evidence             []store.Evidence `json:"evidence"`
	Reviews              []store.Review   `json:"reviews"`
}

func cmdPacketRetry(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		fmt.Println("fairway packet retry <task-id> --kind <preflight|live-operation> --source-sha <sha> --operator-surface <surface> --artifact-dir <path> --evidence-contract <text>... --allowed-action <text>... --forbidden-action <text>... --expires-at <time-or-window> --prior-failure-closure <text> [--next-action <text>]")
		fmt.Println("  Render a bounded retry packet; packet rendering is not execution authorization.")
		return nil
	}
	if len(args) < 1 {
		return errors.New("packet retry requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("packet retry", flag.ContinueOnError)
	kind := fs.String("kind", "preflight", "retry packet kind: preflight or live-operation")
	sourceSHA := fs.String("source-sha", "", "source commit sha for this retry")
	operatorSurface := fs.String("operator-surface", "", "operator/provider surface expected to run the retry")
	artifactDir := fs.String("artifact-dir", "", "artifact directory for retry evidence")
	expiresAt := fs.String("expires-at", "", "packet expiry time or exact retry window")
	priorFailureClosure := fs.String("prior-failure-closure", "", "evidence that the prior failure is closed or bounded")
	nextAction := fs.String("next-action", "", "next safe action or command for the operator")
	var evidenceContract multiFlag
	var allowedActions multiFlag
	var forbiddenActions multiFlag
	fs.Var(&evidenceContract, "evidence-contract", "required evidence item; may repeat")
	fs.Var(&allowedActions, "allowed-action", "allowed action for this retry; may repeat")
	fs.Var(&forbiddenActions, "forbidden-action", "forbidden action until fresh approval; may repeat")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet retry arguments: %s", strings.Join(fs.Args(), " "))
	}
	if err := validateRetryPacketInputs(*kind, *sourceSHA, *operatorSurface, *artifactDir, *expiresAt, *priorFailureClosure, evidenceContract, allowedActions, forbiddenActions); err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		checkpoints, err := s.Checkpoints(ctx, "", true)
		if err != nil {
			return err
		}
		retryBudget, hasRetryBudget := livewindow.RetryBudgetForTask(checkpoints, taskID)
		if hasRetryBudget && retryBudget.RequiresReset {
			return fmt.Errorf("packet retry requires causal reset before another %s packet: task=%s meaningful_failures=%d budget=%d action=record live-window retry-budget with --reset-task after reset proof", *kind, taskID, retryBudget.MeaningfulFailures, retryBudget.Budget)
		}
		if hasRetryBudget && retryBudget.Exhausted && strings.TrimSpace(retryBudget.ResetTask) != "" {
			if _, _, _, _, _, err := s.TaskDetail(ctx, strings.TrimSpace(retryBudget.ResetTask)); err != nil {
				return fmt.Errorf("packet retry reset task %q not found: %w", retryBudget.ResetTask, err)
			}
		}
		packet := retryPacket{
			Task:                task,
			Kind:                *kind,
			SourceSHA:           *sourceSHA,
			OperatorSurface:     *operatorSurface,
			ArtifactDir:         *artifactDir,
			EvidenceContract:    append([]string{}, evidenceContract...),
			AllowedActions:      append([]string{}, allowedActions...),
			ForbiddenActions:    append([]string{}, forbiddenActions...),
			ExpiresAt:           *expiresAt,
			PriorFailureClosure: *priorFailureClosure,
			NextAction:          *nextAction,
			Authorization:       "packet rendering only; this is not execution authorization; no hidden approval is granted by this packet",
			Evidence:            evidence,
			Reviews:             reviews,
		}
		if hasRetryBudget {
			packet.IterationCount = retryBudget.NextIteration
			packet.MeaningfulFailures = retryBudget.MeaningfulFailures
			packet.CoordinationFailures = retryBudget.CoordinationFailures
			packet.RetryBudget = retryBudget.Budget
			packet.ResetTask = retryBudget.ResetTask
			packet.ResetReason = retryBudget.ResetReason
		}
		if opts.JSON {
			return printJSON(packet)
		}
		printRetryPacket(packet)
		return nil
	})
}

func validateRetryPacketInputs(kind, sourceSHA, operatorSurface, artifactDir, expiresAt, priorFailureClosure string, evidenceContract, allowedActions, forbiddenActions []string) error {
	switch kind {
	case "preflight", "live-operation":
	default:
		return fmt.Errorf("packet retry --kind must be preflight or live-operation, got %q", kind)
	}
	var missing []string
	required := map[string]string{
		"--source-sha":            sourceSHA,
		"--operator-surface":      operatorSurface,
		"--artifact-dir":          artifactDir,
		"--expires-at":            expiresAt,
		"--prior-failure-closure": priorFailureClosure,
	}
	for flag, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, flag)
		}
	}
	if len(evidenceContract) == 0 {
		missing = append(missing, "--evidence-contract")
	}
	if len(allowedActions) == 0 {
		missing = append(missing, "--allowed-action")
	}
	if len(forbiddenActions) == 0 {
		missing = append(missing, "--forbidden-action")
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("packet retry requires %s", strings.Join(missing, ", "))
	}
	return nil
}

func printRetryPacket(packet retryPacket) {
	fmt.Printf("# Retry Packet: %s\n\n", packet.Task.Definition.ID)
	fmt.Printf("title: %s\nstatus: %s\nrole: %s\nkind: %s\nsource_sha: %s\noperator_surface: %s\nartifact_dir: %s\nexpires_at: %s\n",
		packet.Task.Definition.Title,
		packet.Task.Status,
		packet.Task.Definition.Role,
		packet.Kind,
		packet.SourceSHA,
		packet.OperatorSurface,
		packet.ArtifactDir,
		packet.ExpiresAt)
	if packet.RetryBudget > 0 {
		fmt.Printf("iteration_count: %d\nmeaningful_failures: %d\ncoordination_failures: %d\nretry_budget: %d\n",
			packet.IterationCount,
			packet.MeaningfulFailures,
			packet.CoordinationFailures,
			packet.RetryBudget)
		if packet.ResetTask != "" {
			fmt.Printf("reset_task: %s\n", packet.ResetTask)
		}
		if packet.ResetReason != "" {
			fmt.Printf("reset_reason: %s\n", packet.ResetReason)
		}
	}
	fmt.Printf("\n## Authorization Boundary\n%s. Re-run Fairway status, review, and gate checks before acting.\n", packet.Authorization)
	fmt.Printf("\n## Prior Failure Closure\n%s\n", packet.PriorFailureClosure)
	printPacketList("Evidence Contract", packet.EvidenceContract)
	printPacketList("Allowed Actions", packet.AllowedActions)
	printPacketList("Forbidden Actions", packet.ForbiddenActions)
	if packet.NextAction != "" {
		fmt.Printf("\n## Next Action\n%s\n", packet.NextAction)
	}
	printEvidenceSummary(packet.Evidence)
	fmt.Println("\n## Reviews")
	for _, review := range packet.Reviews {
		fmt.Printf("- %s by %s: %s\n", review.Verdict, review.Reviewer, review.Reason)
	}
}

type rulePacket struct {
	TaskID        string           `json:"task_id"`
	Title         string           `json:"title"`
	Status        string           `json:"status"`
	Profile       string           `json:"profile,omitempty"`
	Risk          string           `json:"risk,omitempty"`
	Selected      []rulePacketRule `json:"selected"`
	NonApplicable []rulePacketRule `json:"non_applicable,omitempty"`
}

type rulePacketRule struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title,omitempty"`
	Group               string   `json:"group,omitempty"`
	Source              string   `json:"source,omitempty"`
	Mode                string   `json:"mode,omitempty"`
	Status              string   `json:"status"`
	Reasons             []string `json:"reasons,omitempty"`
	RequiredEvidence    []string `json:"required_evidence,omitempty"`
	RecommendedCommands []string `json:"recommended_commands,omitempty"`
	ReviewDomains       []string `json:"review_domains,omitempty"`
	StopConditions      []string `json:"stop_conditions,omitempty"`
	ResidualRiskFields  []string `json:"residual_risk_fields,omitempty"`
}

func cmdPacketRules(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("packet rules requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("packet rules", flag.ContinueOnError)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet rules arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		task, _, _, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		packs, err := rules.LoadConfigured(cfg, root, rules.LoadOptions{
			Root:            root,
			KnownDomains:    rules.ReviewDomainSet(cfg),
			KnownEvidence:   rules.ConfigGateEvidenceSet(cfg),
			IncludeDisabled: true,
		})
		if err != nil {
			return err
		}
		packet := rulePacket{
			TaskID:  task.Definition.ID,
			Title:   task.Definition.Title,
			Status:  task.Status,
			Profile: task.Definition.Profile,
			Risk:    task.Definition.RiskLevel,
		}
		for _, match := range rules.MatchTask(cfg, packs, task) {
			rule := rulePacketRule{
				ID:                  match.Rule.ID,
				Title:               match.Rule.Title,
				Group:               match.Rule.Group,
				Source:              match.Rule.SourceName,
				Mode:                firstNonEmpty(match.Rule.Mode, "advisory"),
				Status:              match.Status,
				Reasons:             append([]string(nil), match.Reasons...),
				RequiredEvidence:    append([]string(nil), match.Rule.RequiredEvidence...),
				RecommendedCommands: append([]string(nil), match.Rule.RecommendedCommands...),
				ReviewDomains:       append([]string(nil), match.Rule.ReviewDomains...),
				StopConditions:      append([]string(nil), match.Rule.StopConditions...),
				ResidualRiskFields:  append([]string(nil), match.Rule.StopConditions...),
			}
			if match.Status == "selected" {
				packet.Selected = append(packet.Selected, rule)
			} else {
				packet.NonApplicable = append(packet.NonApplicable, rule)
			}
		}
		if opts.JSON {
			return printJSON(packet)
		}
		fmt.Printf("# Rule Packet: %s\n\n", packet.TaskID)
		fmt.Printf("title: %s\nstatus: %s\nprofile: %s\nrisk: %s\n", packet.Title, packet.Status, packet.Profile, packet.Risk)
		fmt.Println("\n## Selected Rules")
		if len(packet.Selected) == 0 {
			fmt.Println("- none")
		}
		for _, rule := range packet.Selected {
			printRulePacketRule(rule)
		}
		fmt.Println("\n## Non-Applicable Rules")
		if len(packet.NonApplicable) == 0 {
			fmt.Println("- none")
		}
		for _, rule := range packet.NonApplicable {
			printRulePacketRule(rule)
		}
		fmt.Println("\n## Evidence Recording")
		fmt.Printf("- fairway record evidence %s --artifact-type rule-packet --artifact <path> --result pass --command-text \"fairway packet rules %s\"\n", packet.TaskID, packet.TaskID)
		return nil
	})
}

func printRulePacketRule(rule rulePacketRule) {
	fmt.Printf("- %s (%s, %s)\n", rule.ID, rule.Status, rule.Mode)
	if rule.Title != "" {
		fmt.Printf("  - title: %s\n", rule.Title)
	}
	if rule.Group != "" {
		fmt.Printf("  - group: %s\n", rule.Group)
	}
	printIndentedList("required evidence", rule.RequiredEvidence)
	printIndentedList("recommended commands", rule.RecommendedCommands)
	printIndentedList("review domains", rule.ReviewDomains)
	printIndentedList("residual risk / stop conditions", rule.ResidualRiskFields)
	printIndentedList("rationale", rule.Reasons)
}

func printIndentedList(label string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Printf("  - %s: %s\n", label, strings.Join(values, "; "))
}

func cmdPacketContext(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("packet context requires task id")
	}
	fs := flag.NewFlagSet("packet context", flag.ContinueOnError)
	goal := fs.String("goal", "", "goal")
	owner := fs.String("owner", "", "owner")
	acceptance := fs.String("acceptance", "", "acceptance")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet context arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, transitions, evidence, handoffs, reviews, err := s.TaskDetail(ctx, args[0])
		if err != nil {
			return err
		}
		packet := struct {
			Task        store.Task         `json:"task"`
			Goal        string             `json:"goal"`
			Owner       string             `json:"owner"`
			Acceptance  string             `json:"acceptance"`
			Transitions []store.Transition `json:"transitions"`
			Evidence    []store.Evidence   `json:"evidence"`
			Handoffs    []store.Handoff    `json:"handoffs"`
			Reviews     []store.Review     `json:"reviews"`
		}{task, *goal, *owner, *acceptance, transitions, evidence, handoffs, reviews}
		if opts.JSON {
			return printJSON(packet)
		}
		fmt.Printf("# Context Packet: %s\n\n", task.Definition.ID)
		fmt.Printf("title: %s\nstatus: %s\nrole: %s\nowner: %s\n", task.Definition.Title, task.Status, task.Definition.Role, *owner)
		if *goal != "" {
			fmt.Printf("\n## Goal\n%s\n", *goal)
		}
		if *acceptance != "" {
			fmt.Printf("\n## Acceptance\n%s\n", *acceptance)
		}
		if task.Definition.Notes != "" {
			fmt.Printf("\n## Notes\n%s\n", task.Definition.Notes)
		}
		fmt.Println("\n## Dependencies")
		for _, dep := range task.Definition.Dependencies {
			fmt.Printf("- %s\n", dep)
		}
		fmt.Println("\n## Recent History")
		for _, tr := range transitions {
			fmt.Printf("- %s -> %s by %s: %s\n", tr.FromStatus, tr.ToStatus, tr.Actor, tr.Reason)
		}
		fmt.Println("\n## Evidence")
		for _, ev := range evidence {
			fmt.Printf("- %s: %s %s\n", ev.Result, ev.CommandText, ev.ArtifactPath)
		}
		fmt.Println("\n## Handoffs")
		for _, h := range handoffs {
			fmt.Printf("- to %s: %s\n", h.ToRole, h.Payload)
		}
		fmt.Println("\n## Reviews")
		for _, review := range reviews {
			fmt.Printf("- %s by %s: %s\n", review.Verdict, review.Reviewer, review.Reason)
		}
		return nil
	})
}

func cmdPacketBugfix(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("packet bugfix requires task id")
	}
	fs := flag.NewFlagSet("packet bugfix", flag.ContinueOnError)
	summary := fs.String("bug-summary", "", "bug summary")
	rootCause := fs.String("root-cause", "", "root cause")
	owningLayer := fs.String("owning-layer", "", "owning layer")
	proof := fs.String("proof-command", "", "proof command")
	coverage := fs.String("regression-coverage", "", "regression coverage")
	residualRisk := fs.String("residual-risk", "", "residual risk")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet bugfix arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, args[0])
		if err != nil {
			return err
		}
		packet := struct {
			Task               store.Task       `json:"task"`
			BugSummary         string           `json:"bug_summary"`
			RootCause          string           `json:"root_cause"`
			OwningLayer        string           `json:"owning_layer"`
			ProofCommand       string           `json:"proof_command"`
			RegressionCoverage string           `json:"regression_coverage"`
			ResidualRisk       string           `json:"residual_risk"`
			Evidence           []store.Evidence `json:"evidence"`
			Reviews            []store.Review   `json:"reviews"`
		}{task, *summary, *rootCause, *owningLayer, *proof, *coverage, *residualRisk, evidence, reviews}
		if opts.JSON {
			return printJSON(packet)
		}
		fmt.Printf("# Bugfix Packet: %s\n\n", task.Definition.ID)
		fmt.Printf("title: %s\nstatus: %s\nrole: %s\n", task.Definition.Title, task.Status, task.Definition.Role)
		fmt.Printf("\n## Bug Summary\n%s\n", *summary)
		fmt.Printf("\n## Root Cause\n%s\n", *rootCause)
		fmt.Printf("\n## Owning Layer\n%s\n", *owningLayer)
		fmt.Printf("\n## Proof Command\n%s\n", *proof)
		fmt.Printf("\n## Regression Coverage\n%s\n", *coverage)
		fmt.Printf("\n## Residual Risk\n%s\n", *residualRisk)
		fmt.Println("\n## Existing Evidence")
		for _, ev := range evidence {
			fmt.Printf("- %s: %s %s\n", ev.Result, ev.CommandText, ev.ArtifactPath)
		}
		return nil
	})
}

func cmdPacketWatcher(opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("packet watcher requires watch id")
	}
	watchID := args[0]
	fs := flag.NewFlagSet("packet watcher", flag.ContinueOnError)
	owner := fs.String("owner", "", "owner role or lane")
	process := fs.String("process", "", "process to watch")
	command := fs.String("command", "", "watch command")
	success := fs.String("success", "", "success condition")
	failure := fs.String("failure", "", "failure condition")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet watcher arguments: %s", strings.Join(fs.Args(), " "))
	}
	packet := struct {
		WatchID string `json:"watch_id"`
		Owner   string `json:"owner"`
		Process string `json:"process"`
		Command string `json:"command"`
		Success string `json:"success"`
		Failure string `json:"failure"`
	}{watchID, *owner, *process, *command, *success, *failure}
	if opts.JSON {
		return printJSON(packet)
	}
	fmt.Printf("# Watcher Packet: %s\n\n", watchID)
	fmt.Printf("owner: %s\nprocess: %s\ncommand: %s\n", *owner, *process, *command)
	fmt.Printf("\n## Success\n%s\n", *success)
	fmt.Printf("\n## Failure\n%s\n", *failure)
	return nil
}

func cmdPacketReleaseRun(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("packet release-run requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("packet release-run", flag.ContinueOnError)
	version := fs.String("version", "", "release version")
	tag := fs.String("tag", "", "release tag")
	sourceSHA := fs.String("source-sha", "", "source sha")
	releaseNotes := fs.String("release-notes", "", "release notes path or status")
	changelogState := fs.String("changelog-state", "", "changelog state")
	ciStatus := fs.String("ci-status", "", "CI status")
	docsStatus := fs.String("docs-status", "", "docs status")
	signingStatus := fs.String("signing-status", "", "signing status")
	notaryStatus := fs.String("notary-status", "", "notary status")
	releaseURL := fs.String("release-url", "", "release URL")
	homebrewTapCommit := fs.String("homebrew-tap-commit", "", "Homebrew tap commit")
	var commands multiFlag
	fs.Var(&commands, "verification-command", "verification command; may repeat")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet release-run arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		packet := struct {
			Task                store.Task       `json:"task"`
			Version             string           `json:"version"`
			Tag                 string           `json:"tag"`
			SourceSHA           string           `json:"source_sha"`
			ReleaseNotes        string           `json:"release_notes"`
			ChangelogState      string           `json:"changelog_state"`
			CIStatus            string           `json:"ci_status"`
			DocsStatus          string           `json:"docs_status"`
			SigningStatus       string           `json:"signing_status"`
			NotaryStatus        string           `json:"notary_status"`
			ReleaseURL          string           `json:"release_url"`
			HomebrewTapCommit   string           `json:"homebrew_tap_commit"`
			VerificationCommand []string         `json:"verification_commands"`
			Evidence            []store.Evidence `json:"evidence"`
			Reviews             []store.Review   `json:"reviews"`
		}{task, *version, *tag, *sourceSHA, *releaseNotes, *changelogState, *ciStatus, *docsStatus, *signingStatus, *notaryStatus, *releaseURL, *homebrewTapCommit, commands, evidence, reviews}
		if opts.JSON {
			return printJSON(packet)
		}
		fmt.Printf("# Release Run Packet: %s\n\n", task.Definition.ID)
		fmt.Printf("title: %s\nstatus: %s\nrole: %s\n", task.Definition.Title, task.Status, task.Definition.Role)
		fmt.Printf("\n## Release Identity\nversion: %s\ntag: %s\nsource_sha: %s\n", *version, *tag, *sourceSHA)
		fmt.Printf("\n## Documentation\nrelease_notes: %s\nchangelog_state: %s\n", *releaseNotes, *changelogState)
		fmt.Printf("\n## Verification State\nci: %s\ndocs: %s\nsigning: %s\nnotary: %s\nrelease_url: %s\nhomebrew_tap_commit: %s\n", *ciStatus, *docsStatus, *signingStatus, *notaryStatus, *releaseURL, *homebrewTapCommit)
		printPacketList("Verification Commands", commands)
		fmt.Println("\n## Required Evidence")
		for _, item := range []string{
			"local checks pass",
			"pushed main checks pass",
			"tag push recorded",
			"release workflow result recorded",
			"GitHub release is public before Homebrew is treated as usable",
			"asset URLs return success",
			"Homebrew cask version matches tag",
			"brew fetch succeeds",
		} {
			fmt.Printf("- %s\n", item)
		}
		printEvidenceSummary(evidence)
		fmt.Println("\n## Reviews")
		for _, review := range reviews {
			fmt.Printf("- %s by %s: %s\n", review.Verdict, review.Reviewer, review.Reason)
		}
		return nil
	})
}

func cmdPacketArchitectureMap(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("packet architecture-map requires task id")
	}
	fs := flag.NewFlagSet("packet architecture-map", flag.ContinueOnError)
	scope := fs.String("scope", "", "map scope")
	currentOwner := fs.String("current-owner", "", "current owner")
	targetOwner := fs.String("target-owner", "", "target owner")
	migrationRisk := fs.String("migration-risk", "", "migration risk")
	acceptance := fs.String("acceptance", "", "acceptance")
	var sourceDocs multiFlag
	fs.Var(&sourceDocs, "source-doc", "source doc path or URL; may repeat")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet architecture-map arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, args[0])
		if err != nil {
			return err
		}
		packet := struct {
			Task          store.Task       `json:"task"`
			Scope         string           `json:"scope"`
			CurrentOwner  string           `json:"current_owner"`
			TargetOwner   string           `json:"target_owner"`
			MigrationRisk string           `json:"migration_risk"`
			SourceDocs    []string         `json:"source_docs"`
			Acceptance    string           `json:"acceptance"`
			Evidence      []store.Evidence `json:"evidence"`
			Reviews       []store.Review   `json:"reviews"`
		}{task, *scope, *currentOwner, *targetOwner, *migrationRisk, sourceDocs, *acceptance, evidence, reviews}
		if opts.JSON {
			return printJSON(packet)
		}
		fmt.Printf("# Architecture Map Packet: %s\n\n", task.Definition.ID)
		fmt.Printf("title: %s\nstatus: %s\nrole: %s\n", task.Definition.Title, task.Status, task.Definition.Role)
		fmt.Printf("\n## Scope\n%s\n", *scope)
		fmt.Printf("\n## Ownership\ncurrent: %s\ntarget: %s\n", *currentOwner, *targetOwner)
		fmt.Printf("\n## Migration Risk\n%s\n", *migrationRisk)
		printPacketList("Source Docs", sourceDocs)
		fmt.Printf("\n## Acceptance\n%s\n", *acceptance)
		printEvidenceSummary(evidence)
		return nil
	})
}

func cmdPacketBoundaryGuard(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("packet boundary-guard requires task id")
	}
	fs := flag.NewFlagSet("packet boundary-guard", flag.ContinueOnError)
	intent := fs.String("guard-intent", "", "guard intent")
	graduation := fs.String("graduation-criteria", "", "graduation criteria")
	var findings multiFlag
	var falsePositives multiFlag
	var proofCommands multiFlag
	fs.Var(&findings, "finding", "report-only finding; may repeat")
	fs.Var(&falsePositives, "false-positive", "known false positive; may repeat")
	fs.Var(&proofCommands, "proof-command", "proof command; may repeat")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet boundary-guard arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, args[0])
		if err != nil {
			return err
		}
		packet := struct {
			Task               store.Task       `json:"task"`
			GuardIntent        string           `json:"guard_intent"`
			Findings           []string         `json:"findings"`
			FalsePositives     []string         `json:"false_positives"`
			GraduationCriteria string           `json:"graduation_criteria"`
			ProofCommands      []string         `json:"proof_commands"`
			Evidence           []store.Evidence `json:"evidence"`
			Reviews            []store.Review   `json:"reviews"`
		}{task, *intent, findings, falsePositives, *graduation, proofCommands, evidence, reviews}
		if opts.JSON {
			return printJSON(packet)
		}
		fmt.Printf("# Boundary Guard Packet: %s\n\n", task.Definition.ID)
		fmt.Printf("title: %s\nstatus: %s\nrole: %s\n", task.Definition.Title, task.Status, task.Definition.Role)
		fmt.Printf("\n## Guard Intent\n%s\n", *intent)
		printPacketList("Report-Only Findings", findings)
		printPacketList("False Positives", falsePositives)
		fmt.Printf("\n## Graduation Criteria\n%s\n", *graduation)
		printPacketList("Proof Commands", proofCommands)
		printEvidenceSummary(evidence)
		return nil
	})
}

func cmdPacketVerticalSlice(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("packet vertical-slice requires task id")
	}
	fs := flag.NewFlagSet("packet vertical-slice", flag.ContinueOnError)
	targetSeam := fs.String("target-seam", "", "target seam")
	oldPath := fs.String("old-path", "", "old path")
	newPath := fs.String("new-path", "", "new path")
	adapter := fs.String("adapter", "", "adapter or compatibility boundary")
	rollbackPlan := fs.String("rollback-plan", "", "rollback plan")
	var proofCommands multiFlag
	fs.Var(&proofCommands, "proof-command", "proof command; may repeat")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected packet vertical-slice arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, args[0])
		if err != nil {
			return err
		}
		packet := struct {
			Task          store.Task       `json:"task"`
			TargetSeam    string           `json:"target_seam"`
			OldPath       string           `json:"old_path"`
			NewPath       string           `json:"new_path"`
			Adapter       string           `json:"adapter"`
			ProofCommands []string         `json:"proof_commands"`
			RollbackPlan  string           `json:"rollback_plan"`
			Evidence      []store.Evidence `json:"evidence"`
			Reviews       []store.Review   `json:"reviews"`
		}{task, *targetSeam, *oldPath, *newPath, *adapter, proofCommands, *rollbackPlan, evidence, reviews}
		if opts.JSON {
			return printJSON(packet)
		}
		fmt.Printf("# Vertical Slice Packet: %s\n\n", task.Definition.ID)
		fmt.Printf("title: %s\nstatus: %s\nrole: %s\n", task.Definition.Title, task.Status, task.Definition.Role)
		fmt.Printf("\n## Target Seam\n%s\n", *targetSeam)
		fmt.Printf("\n## Path Movement\nold: %s\nnew: %s\n", *oldPath, *newPath)
		fmt.Printf("\n## Adapter\n%s\n", *adapter)
		printPacketList("Proof Commands", proofCommands)
		fmt.Printf("\n## Rollback Plan\n%s\n", *rollbackPlan)
		printEvidenceSummary(evidence)
		return nil
	})
}

func printPacketList(title string, values []string) {
	fmt.Printf("\n## %s\n", title)
	for _, value := range values {
		fmt.Printf("- %s\n", value)
	}
}

func printEvidenceSummary(evidence []store.Evidence) {
	fmt.Println("\n## Existing Evidence")
	for _, ev := range evidence {
		fmt.Printf("- %s: %s %s\n", ev.Result, ev.CommandText, ev.ArtifactPath)
	}
}

func cmdWatcher(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("watcher requires subcommand: start, finish, status")
	}
	if isHelpOnly(args) {
		subcommandUsage("watcher", "start|finish|status")
		return nil
	}
	switch args[0] {
	case "start":
		return cmdWatcherStart(ctx, opts, args[1:])
	case "finish":
		return cmdWatcherFinish(ctx, opts, args[1:])
	case "status":
		return cmdWatcherStatus(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown watcher subcommand %q", args[0])
	}
}

func cmdWatcherStart(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("watcher start requires watch id")
	}
	fs := flag.NewFlagSet("watcher start", flag.ContinueOnError)
	taskID := fs.String("task", "", "task id")
	owner := fs.String("owner", "", "owner")
	process := fs.String("process", "", "process")
	command := fs.String("command", "", "command")
	success := fs.String("success", "", "success")
	failure := fs.String("failure", "", "failure")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected watcher start arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		watcher := store.Watcher{ID: args[0], TaskID: *taskID, Owner: *owner, Process: *process, Command: *command, Success: *success, Failure: *failure}
		if err := s.StartWatcher(ctx, watcher); err != nil {
			return err
		}
		fmt.Println("watcher started", args[0])
		return nil
	})
}

func cmdWatcherFinish(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("watcher finish requires watch id")
	}
	fs := flag.NewFlagSet("watcher finish", flag.ContinueOnError)
	result := fs.String("result", "", "result")
	artifact := fs.String("artifact", "", "artifact path")
	duration := fs.Int("duration-seconds", -1, "duration seconds")
	notes := fs.String("notes", "", "notes")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected watcher finish arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		var dur *int
		if *duration >= 0 {
			dur = duration
		}
		if err := s.FinishWatcher(ctx, args[0], *result, *artifact, dur, *notes); err != nil {
			return err
		}
		fmt.Println("watcher finished", args[0])
		return nil
	})
}

func cmdWatcherStatus(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("watcher status", flag.ContinueOnError)
	includeDone := fs.Bool("include-done", false, "include completed watchers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected watcher status arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		watchers, err := s.Watchers(ctx, *includeDone)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(watchers)
		}
		for _, watcher := range watchers {
			fmt.Printf("%s\t%s\t%s\t%s\t%s\n", watcher.ID, watcher.TaskID, watcher.Status, watcher.Owner, watcher.Process)
		}
		return nil
	})
}

type regressionCatalog struct {
	Packs []regressionPack `json:"packs" yaml:"packs"`
}

type regressionBlocking struct {
	Default        bool            `json:"default" yaml:"default"`
	PerEnvironment map[string]bool `json:"per_environment,omitempty" yaml:"per_environment,omitempty"`
}

func (b *regressionBlocking) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var flag bool
		if err := value.Decode(&flag); err != nil {
			return err
		}
		b.Default = flag
		b.PerEnvironment = nil
		return nil
	case yaml.MappingNode:
		var perEnvironment map[string]bool
		if err := value.Decode(&perEnvironment); err != nil {
			return err
		}
		b.Default = false
		for _, enabled := range perEnvironment {
			if enabled {
				b.Default = true
				break
			}
		}
		b.PerEnvironment = perEnvironment
		return nil
	default:
		return fmt.Errorf("blocking must be a boolean or environment map")
	}
}

func (b *regressionBlocking) UnmarshalJSON(data []byte) error {
	var flag bool
	if err := json.Unmarshal(data, &flag); err == nil {
		b.Default = flag
		b.PerEnvironment = nil
		return nil
	}
	var perEnvironment map[string]bool
	if err := json.Unmarshal(data, &perEnvironment); err != nil {
		return err
	}
	b.Default = false
	for _, enabled := range perEnvironment {
		if enabled {
			b.Default = true
			break
		}
	}
	b.PerEnvironment = perEnvironment
	return nil
}

func (b regressionBlocking) String() string {
	if len(b.PerEnvironment) == 0 {
		return fmt.Sprintf("%t", b.Default)
	}
	keys := make([]string, 0, len(b.PerEnvironment))
	for key := range b.PerEnvironment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%t", key, b.PerEnvironment[key]))
	}
	return strings.Join(parts, ",")
}

type regressionPack struct {
	ID                   string             `json:"id" yaml:"id"`
	Title                string             `json:"title" yaml:"title"`
	Owner                string             `json:"owner" yaml:"owner"`
	TargetEnvironments   []string           `json:"target_environments" yaml:"target_environments"`
	Blocking             regressionBlocking `json:"blocking" yaml:"blocking"`
	RequiredSeedData     []string           `json:"required_seed_data" yaml:"required_seed_data"`
	LowestReliableLayer  string             `json:"lowest_reliable_layer" yaml:"lowest_reliable_layer"`
	RequiredProof        []string           `json:"required_proof" yaml:"required_proof"`
	ArtifactRequirements []string           `json:"artifact_requirements" yaml:"artifact_requirements"`
	CurrentAutomation    []string           `json:"current_automation" yaml:"current_automation"`
	Gaps                 []string           `json:"gaps" yaml:"gaps"`
}

func cmdRegressionPack(opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("regression-pack requires subcommand: list, show, validate")
	}
	if isHelpOnly(args) {
		subcommandUsage("regression-pack", "list|show|validate")
		return nil
	}
	switch args[0] {
	case "list":
		return cmdRegressionPackList(opts, args[1:])
	case "show":
		return cmdRegressionPackShow(opts, args[1:])
	case "validate":
		return cmdRegressionPackValidate(args[1:])
	default:
		return fmt.Errorf("unknown regression-pack subcommand %q", args[0])
	}
}

func cmdRegressionPackList(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("regression-pack list", flag.ContinueOnError)
	catalogPath := fs.String("catalog", defaultRegressionCatalogPath(), "catalog path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected regression-pack list arguments: %s", strings.Join(fs.Args(), " "))
	}
	catalog, err := loadRegressionCatalog(*catalogPath)
	if err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(catalog.Packs)
	}
	for _, pack := range catalog.Packs {
		fmt.Printf("%s\t%s\t%s\tblocking=%s\n", pack.ID, pack.Owner, pack.Title, pack.Blocking.String())
	}
	return nil
}

func cmdRegressionPackShow(opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("regression-pack show requires pack id")
	}
	fs := flag.NewFlagSet("regression-pack show", flag.ContinueOnError)
	catalogPath := fs.String("catalog", defaultRegressionCatalogPath(), "catalog path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected regression-pack show arguments: %s", strings.Join(fs.Args(), " "))
	}
	catalog, err := loadRegressionCatalog(*catalogPath)
	if err != nil {
		return err
	}
	for _, pack := range catalog.Packs {
		if pack.ID != args[0] {
			continue
		}
		if opts.JSON {
			return printJSON(pack)
		}
		printRegressionPack(pack)
		return nil
	}
	return store.ErrNotFound
}

func cmdRegressionPackValidate(args []string) error {
	path := defaultRegressionCatalogPath()
	if len(args) > 1 {
		return fmt.Errorf("unexpected regression-pack validate arguments: %s", strings.Join(args[1:], " "))
	}
	if len(args) == 1 {
		path = args[0]
	}
	catalog, err := loadRegressionCatalog(path)
	if err != nil {
		return err
	}
	if err := validateRegressionCatalog(catalog); err != nil {
		return err
	}
	fmt.Printf("valid %s (%d packs)\n", path, len(catalog.Packs))
	return nil
}

func cmdPruneStale(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected prune-stale arguments: %s", strings.Join(args, " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		result, err := s.PruneStale(ctx)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(result)
		}
		fmt.Printf("pruned state=%d history=%d evidence=%d handoffs=%d reviews=%d checkpoints=%d watchers=%d\n",
			result.StateRows, result.HistoryRows, result.EvidenceRows, result.HandoffRows, result.ReviewRows, result.CheckpointRows, result.WatcherRows)
		return nil
	})
}

func cmdRegister(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	name := fs.String("name", "", "project name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected register arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg, root, configPath, err := loadConfig(opts)
	if err != nil {
		return err
	}
	projectName := *name
	if projectName == "" {
		projectName = cfg.Fairway.ProjectName
	}
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}
	reg, err = registry.Register(reg, registry.Project{Name: projectName, Path: root, DBPath: config.DBPath(cfg, root), ConfigPath: configPath})
	if err != nil {
		return err
	}
	if err := registry.Save(regPath, reg); err != nil {
		return err
	}
	fmt.Println("registered", projectName)
	return nil
}

func cmdUnregister(opts globalOptions, args []string) error {
	name := ""
	if len(args) > 1 {
		return fmt.Errorf("unexpected unregister arguments: %s", strings.Join(args[1:], " "))
	}
	if len(args) == 1 {
		name = args[0]
	}
	if name == "" {
		cfg, _, _, err := loadConfig(opts)
		if err != nil {
			return err
		}
		name = cfg.Fairway.ProjectName
	}
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}
	reg, removed := registry.Unregister(reg, name)
	if !removed {
		return store.ErrNotFound
	}
	if err := registry.Save(regPath, reg); err != nil {
		return err
	}
	fmt.Println("unregistered", name)
	return nil
}

func cmdProjects(opts globalOptions, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected projects arguments: %s", strings.Join(args, " "))
	}
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(reg.Projects)
	}
	for _, project := range reg.Projects {
		fmt.Printf("%s\t%s\t%s\t%s\n", project.Name, project.Path, project.DBPath, project.ConfigPath)
	}
	return nil
}

func cmdTracker(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("tracker requires subcommand: providers, configure, import, link, links, export-status, resolve, reconcile, plane")
	}
	if isHelpOnly(args) {
		subcommandUsage("tracker", "providers|configure|import|link|links|export-status|resolve|reconcile|plane")
		return nil
	}
	switch args[0] {
	case "providers":
		return cmdTrackerProviders(ctx, opts, args[1:])
	case "configure":
		return cmdTrackerConfigure(ctx, opts, args[1:])
	case "import":
		return cmdTrackerImport(ctx, opts, args[1:])
	case "link":
		return cmdTrackerLink(ctx, opts, args[1:])
	case "links":
		return cmdTrackerLinks(ctx, opts, args[1:])
	case "export-status", "export-comment":
		return cmdTrackerExportStatus(ctx, opts, args[0], args[1:])
	case "resolve":
		return cmdTrackerResolve(ctx, opts, args[1:])
	case "reconcile":
		return cmdTrackerReconcile(ctx, opts, args[1:])
	case "plane":
		return cmdTrackerPlane(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown tracker subcommand %q", args[0])
	}
}

func cmdTrackerPlane(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("tracker plane requires subcommand: export, import, comment")
	}
	if isHelpOnly(args) {
		subcommandUsage("tracker plane", "export|import|comment")
		return nil
	}
	switch args[0] {
	case "export":
		return cmdTrackerPlaneExport(ctx, opts, args[1:])
	case "import":
		return cmdTrackerPlaneImport(ctx, opts, args[1:])
	case "comment":
		return cmdTrackerPlaneComment(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown tracker plane subcommand %q", args[0])
	}
}

func cmdTrackerPlaneExport(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("tracker plane export", flag.ContinueOnError)
	taskID := fs.String("task-id", "", "specific Fairway task id to export")
	limit := fs.Int("limit", 10, "maximum tasks to include when task-id is omitted")
	apply := fs.Bool("apply", false, "attempt remote apply; unsupported in this spike")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker plane export arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *apply {
		return planetracker.ApplyUnsupported()
	}
	cfg, err := planetracker.ConfigFromEnv()
	if err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		var payloads []planetracker.IssuePayload
		if *taskID != "" {
			payload, err := planeIssuePayloadForTask(ctx, s, *taskID, cfg)
			if err != nil {
				return err
			}
			payloads = append(payloads, payload)
		} else {
			tasks, err := s.AllTasks(ctx)
			if err != nil {
				return err
			}
			if *limit <= 0 {
				*limit = 10
			}
			for _, task := range tasks {
				if len(payloads) >= *limit {
					break
				}
				payload, err := planeIssuePayloadForTask(ctx, s, task.Definition.ID, cfg)
				if err != nil {
					return err
				}
				payloads = append(payloads, payload)
			}
		}
		report := struct {
			Provider      string                      `json:"provider"`
			DryRun        bool                        `json:"dry_run"`
			Config        planetracker.Config         `json:"config"`
			Payloads      []planetracker.IssuePayload `json:"payloads"`
			ApplyBoundary string                      `json:"apply_boundary"`
		}{"plane", true, cfg, payloads, "dry-run only; remote Plane writes require an explicit future apply command and must not mutate Fairway execution state"}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("plane export dry_run=true payloads=%d workspace=%s project=%s\n", len(payloads), cfg.Workspace, cfg.Project)
		for _, payload := range payloads {
			fmt.Printf("%s\t%s\tlabels=%s\n", payload.SourceTaskID, payload.Name, strings.Join(payload.Labels, ","))
		}
		return nil
	})
}

func cmdTrackerPlaneImport(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("tracker plane import", flag.ContinueOnError)
	fixturePath := fs.String("fixture", "", "Plane evaluation fixture YAML")
	apply := fs.Bool("apply", false, "attempt Fairway task import; unsupported in this spike")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker plane import arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *apply {
		return planetracker.ApplyUnsupported()
	}
	if *fixturePath == "" {
		return errors.New("tracker plane import requires --fixture")
	}
	cfg, err := planetracker.ConfigFromEnv()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*fixturePath)
	if err != nil {
		return err
	}
	var fixture planetracker.Fixture
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		return err
	}
	preview, err := planetracker.ImportFixture(fixture, cfg)
	if err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(preview)
	}
	fmt.Printf("plane import dry_run=true tasks=%d workspace=%s project=%s\n", len(preview.Tasks), preview.Workspace, preview.Project)
	for _, task := range preview.Tasks {
		fmt.Printf("%s\t%s\t%s\t%s\n", task.ID, task.Role, task.Kind, task.Title)
	}
	return nil
}

func cmdTrackerPlaneComment(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("tracker plane comment", flag.ContinueOnError)
	taskID := fs.String("task-id", "", "Fairway task id")
	externalID := fs.String("external-id", "", "Plane issue id")
	apply := fs.Bool("apply", false, "attempt remote comment write; unsupported in this spike")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker plane comment arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *apply {
		return planetracker.ApplyUnsupported()
	}
	if *taskID == "" {
		return errors.New("tracker plane comment requires --task-id")
	}
	cfg, err := planetracker.ConfigFromEnv()
	if err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, _, evidence, _, reviews, err := s.TaskDetail(ctx, *taskID)
		if err != nil {
			return err
		}
		payload := planetracker.ExportComment(task, evidence, reviews, *externalID, cfg)
		if opts.JSON {
			return printJSON(payload)
		}
		fmt.Printf("plane comment dry_run=true task=%s external=%s\n%s", *taskID, *externalID, payload.Body)
		return nil
	})
}

func planeIssuePayloadForTask(ctx context.Context, s *store.Store, taskID string, cfg planetracker.Config) (planetracker.IssuePayload, error) {
	task, _, evidence, _, reviews, err := s.TaskDetail(ctx, taskID)
	if err != nil {
		return planetracker.IssuePayload{}, err
	}
	return planetracker.ExportIssue(task, evidence, reviews, cfg), nil
}

func cmdTrackerProviders(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected tracker providers arguments: %s", strings.Join(args, " "))
	}
	providers := tracker.SupportedProviders()
	if opts.JSON {
		return printJSON(providers)
	}
	for _, provider := range providers {
		var ops []string
		for _, op := range provider.Operations {
			ops = append(ops, string(op))
		}
		fmt.Printf("%s\t%s\t%s\n", provider.Name, provider.Kind, strings.Join(ops, ","))
	}
	return nil
}

func cmdTrackerConfigure(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("tracker configure requires provider")
	}
	provider := args[0]
	fs := flag.NewFlagSet("tracker configure", flag.ContinueOnError)
	urlValue := fs.String("url", "", "tracker base URL")
	workspace := fs.String("workspace", "", "tracker workspace")
	project := fs.String("project", "", "tracker project")
	team := fs.String("team", "", "tracker team")
	dryRun := fs.Bool("dry-run", true, "show configuration without writing")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker configure arguments: %s", strings.Join(fs.Args(), " "))
	}
	spec, err := tracker.Provider(provider)
	if err != nil {
		return err
	}
	report := struct {
		Provider     tracker.ProviderSpec `json:"provider"`
		DryRun       bool                 `json:"dry_run"`
		URL          string               `json:"url,omitempty"`
		Workspace    string               `json:"workspace,omitempty"`
		Project      string               `json:"project,omitempty"`
		Team         string               `json:"team,omitempty"`
		PlanningOnly bool                 `json:"planning_only"`
		Note         string               `json:"note"`
	}{spec, *dryRun, *urlValue, *workspace, *project, *team, true, "configuration is an adapter contract preview; credentials must come from environment or OS credential store"}
	if opts.JSON {
		return printJSON(report)
	}
	fmt.Printf("tracker configure provider=%s dry_run=%t url=%s workspace=%s project=%s team=%s\n", spec.Name, report.DryRun, report.URL, report.Workspace, report.Project, report.Team)
	fmt.Println(report.Note)
	return nil
}

func cmdTrackerImport(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("tracker import requires provider")
	}
	provider := args[0]
	fs := flag.NewFlagSet("tracker import", flag.ContinueOnError)
	query := fs.String("query", "", "provider query or filter")
	parent := fs.String("parent", "", "parent Fairway task id")
	dryRun := fs.Bool("dry-run", true, "preview import without writing Fairway tasks")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker import arguments: %s", strings.Join(fs.Args(), " "))
	}
	spec, err := tracker.Provider(provider)
	if err != nil {
		return err
	}
	mappings, err := tracker.DefaultMappings(spec.Name)
	if err != nil {
		return err
	}
	report := struct {
		Provider     tracker.ProviderSpec `json:"provider"`
		DryRun       bool                 `json:"dry_run"`
		Query        string               `json:"query,omitempty"`
		Parent       string               `json:"parent,omitempty"`
		Mappings     []tracker.Mapping    `json:"mappings"`
		PlanningOnly bool                 `json:"planning_only"`
		Note         string               `json:"note"`
	}{spec, *dryRun, *query, *parent, mappings, true, "dry-run import defines mapping only; no Fairway tasks or remote tracker issues are changed"}
	if opts.JSON {
		return printJSON(report)
	}
	fmt.Printf("tracker import provider=%s dry_run=%t query=%q parent=%s\n", spec.Name, report.DryRun, report.Query, report.Parent)
	fmt.Println(report.Note)
	return nil
}

func cmdTrackerLink(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("tracker link requires task id")
	}
	fs := flag.NewFlagSet("tracker link", flag.ContinueOnError)
	provider := fs.String("provider", "", "tracker provider")
	externalID := fs.String("external-id", "", "external issue id")
	url := fs.String("url", "", "external issue url")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker link arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		if err := tracker.ValidateProvider(*provider); err != nil {
			return err
		}
		link := store.TrackerLink{TaskID: args[0], Provider: *provider, ExternalID: *externalID, URL: *url}
		if err := s.UpsertTrackerLink(ctx, link); err != nil {
			return err
		}
		fmt.Printf("linked %s to %s:%s\n", args[0], *provider, *externalID)
		return nil
	})
}

func cmdTrackerExportStatus(ctx context.Context, opts globalOptions, command string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("tracker %s requires task id", command)
	}
	fs := flag.NewFlagSet("tracker "+command, flag.ContinueOnError)
	provider := fs.String("provider", "", "tracker provider")
	externalID := fs.String("external-id", "", "external issue id")
	dryRun := fs.Bool("dry-run", true, "preview exported status/comment without remote write")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker %s arguments: %s", command, strings.Join(fs.Args(), " "))
	}
	if *provider != "" {
		if err := tracker.ValidateProvider(*provider); err != nil {
			return err
		}
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		task, transitions, evidence, _, reviews, err := s.TaskDetail(ctx, args[0])
		if err != nil {
			return err
		}
		report := struct {
			Operation    string             `json:"operation"`
			DryRun       bool               `json:"dry_run"`
			Provider     string             `json:"provider,omitempty"`
			ExternalID   string             `json:"external_id,omitempty"`
			Task         store.Task         `json:"task"`
			EvidenceRows int                `json:"evidence_rows"`
			Reviews      int                `json:"reviews"`
			Transitions  []store.Transition `json:"transitions"`
			PlanningOnly bool               `json:"planning_only"`
			Mutates      string             `json:"mutates"`
			Note         string             `json:"note"`
		}{command, *dryRun, *provider, *externalID, task, len(evidence), len(reviews), transitions, true, "remote tracker comment/status only when an adapter apply command exists", "Fairway execution state is not changed by tracker export"}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("tracker %s task=%s dry_run=%t provider=%s external_id=%s evidence=%d reviews=%d\n", command, args[0], report.DryRun, report.Provider, report.ExternalID, report.EvidenceRows, report.Reviews)
		fmt.Println(report.Note)
		return nil
	})
}

func cmdTrackerResolve(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("tracker resolve", flag.ContinueOnError)
	provider := fs.String("provider", "", "tracker provider")
	externalID := fs.String("external-id", "", "external issue id")
	urlValue := fs.String("url", "", "external issue URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker resolve arguments: %s", strings.Join(fs.Args(), " "))
	}
	ref, err := tracker.ResolveReference(*provider, *externalID, *urlValue)
	if err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(ref)
	}
	fmt.Printf("%s\t%s\t%s\n", ref.Provider, ref.ExternalID, ref.URL)
	return nil
}

func cmdTrackerLinks(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected tracker links arguments: %s", strings.Join(args, " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		links, err := s.TrackerLinks(ctx)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(links)
		}
		for _, link := range links {
			fmt.Printf("%s\t%s\t%s\t%s\n", link.TaskID, link.Provider, link.ExternalID, link.URL)
		}
		return nil
	})
}

func cmdTrackerReconcile(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("tracker reconcile", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", true, "show proposed actions without applying")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tracker reconcile arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		links, err := s.TrackerLinks(ctx)
		if err != nil {
			return err
		}
		report := tracker.BuildReconcileReport(links, *dryRun)
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("tracker reconcile dry_run=%t links=%d\n", report.DryRun, len(report.Links))
		fmt.Println(report.Note)
		for _, action := range report.Actions {
			fmt.Printf("would %s\tprovider=%s\ttask=%s\texternal=%s\treason=%s\n", action.Action, action.Provider, action.TaskID, action.ExternalID, action.Reason)
		}
		return nil
	})
}

func cmdTUI(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	once := fs.Bool("once", false, "render once and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected tui arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		if err := printTUIScreen(ctx, cfg, opts, s); err != nil {
			return err
		}
		if *once {
			return nil
		}
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("fairway> ")
			if !scanner.Scan() {
				return scanner.Err()
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			switch parts[0] {
			case "q", "quit", "exit":
				return nil
			case "ready", "r":
				if err := printTUIReady(ctx, cfg, opts, s); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			case "status", "s":
				if err := printTUIStatus(ctx, s); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			case "detail", "d":
				if len(parts) < 2 {
					fmt.Fprintln(os.Stderr, "detail requires task id")
					continue
				}
				if err := printDetail(ctx, cfg, s, parts[1], false); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			case "claim", "c":
				if len(parts) < 2 {
					fmt.Fprintln(os.Stderr, "claim requires task id")
					continue
				}
				branch := fairwaygit.CurrentBranch(".")
				if err := s.Claim(ctx, parts[1], resolveRole(opts), branch); err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				fmt.Println("claimed", parts[1])
			case "set":
				if len(parts) < 3 {
					fmt.Fprintln(os.Stderr, "set requires task id and status")
					continue
				}
				reason := strings.Join(parts[3:], " ")
				if err := setStatusWithConfig(ctx, cfg, ".", s, parts[1], parts[2], reason, "", false); err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				fmt.Println("status", parts[1], parts[2])
			case "evidence", "ev":
				if len(parts) < 4 {
					fmt.Fprintln(os.Stderr, "evidence requires task id, result, and command text")
					continue
				}
				ev := store.Evidence{Result: parts[2], CommandText: strings.Join(parts[3:], " ")}
				if err := s.RecordEvidence(ctx, parts[1], ev); err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				fmt.Println("evidence", parts[1], parts[2])
			case "merge-ready", "mr":
				if len(parts) < 2 {
					fmt.Fprintln(os.Stderr, "merge-ready requires task id")
					continue
				}
				if err := cmdMergeReady(ctx, opts, parts[1:]); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			case "readiness":
				args := []string{"report"}
				if len(parts) > 1 {
					args = append(args, "--profile", parts[1])
				}
				if err := cmdReadiness(ctx, opts, args); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			case "refresh":
				if err := printTUIScreen(ctx, cfg, opts, s); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			case "help", "h":
				printTUIHelp()
			default:
				fmt.Fprintln(os.Stderr, "unknown command; type help")
			}
		}
	})
}

func printTUIScreen(ctx context.Context, cfg config.Config, opts globalOptions, s *store.Store) error {
	fmt.Println("fairway tui")
	printTUIHelp()
	if err := printTUIStatus(ctx, s); err != nil {
		return err
	}
	return printTUIReady(ctx, cfg, opts, s)
}

func printTUIHelp() {
	fmt.Println("commands: ready|r, claim|c <id>, set <id> <status> [reason], evidence|ev <id> <result> <command>, merge-ready|mr <id>, readiness [profile], status|s, detail|d <id>, refresh, help|h, quit|q")
}

func printTUIStatus(ctx context.Context, s *store.Store) error {
	health, err := s.Health(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("health: in_progress=%d stale=%d handoffs=%d reviews=%d\n", health.InProgress, health.StaleInProgress, health.UnacknowledgedHandoff, health.UnroutedReviews)
	return nil
}

func printTUIReady(ctx context.Context, cfg config.Config, opts globalOptions, s *store.Store) error {
	tasks, err := s.Ready(ctx, resolveRole(opts), cfg.States.Terminal)
	if err != nil {
		return err
	}
	fmt.Println("ready:")
	printTasks(tasks)
	return nil
}

func defaultRegressionCatalogPath() string {
	return "regression-packs.yaml"
}

func loadRegressionCatalog(path string) (regressionCatalog, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return regressionCatalog{}, err
	}
	var catalog regressionCatalog
	if strings.HasSuffix(path, ".json") {
		err = json.Unmarshal(data, &catalog)
	} else {
		err = yaml.Unmarshal(data, &catalog)
	}
	if err != nil {
		return regressionCatalog{}, err
	}
	if err := validateRegressionCatalog(catalog); err != nil {
		return regressionCatalog{}, err
	}
	return catalog, nil
}

func validateRegressionCatalog(catalog regressionCatalog) error {
	seen := map[string]bool{}
	for _, pack := range catalog.Packs {
		if pack.ID == "" || pack.Title == "" || pack.Owner == "" {
			return errors.New("regression packs require id, title, and owner")
		}
		if seen[pack.ID] {
			return fmt.Errorf("duplicate regression pack id %s", pack.ID)
		}
		seen[pack.ID] = true
		if len(pack.TargetEnvironments) == 0 || pack.LowestReliableLayer == "" || len(pack.RequiredProof) == 0 {
			return fmt.Errorf("regression pack %s is missing required coverage fields", pack.ID)
		}
	}
	return nil
}

func printRegressionPack(pack regressionPack) {
	fmt.Printf("# Regression Pack: %s\n\n", pack.ID)
	fmt.Printf("title: %s\nowner: %s\nblocking: %s\nlowest_reliable_layer: %s\n", pack.Title, pack.Owner, pack.Blocking.String(), pack.LowestReliableLayer)
	printStringList("target_environments", pack.TargetEnvironments)
	printStringList("required_seed_data", pack.RequiredSeedData)
	printStringList("required_proof", pack.RequiredProof)
	printStringList("artifact_requirements", pack.ArtifactRequirements)
	printStringList("current_automation", pack.CurrentAutomation)
	printStringList("gaps", pack.Gaps)
}

func printStringList(title string, values []string) {
	fmt.Printf("\n## %s\n", title)
	for _, value := range values {
		fmt.Printf("- %s\n", value)
	}
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func packetTemplateByName(templates []config.PacketTemplate, name string) (config.PacketTemplate, bool) {
	for _, template := range templates {
		if template.Name == name {
			return template, true
		}
	}
	for _, template := range builtinPacketTemplates() {
		if template.Name == name {
			return template, true
		}
	}
	return config.PacketTemplate{}, false
}

func builtinPacketTemplates() []config.PacketTemplate {
	return []config.PacketTemplate{{
		Name: "environment-deploy-preflight",
		RequiredFields: []string{
			"environment",
			"deploy_kind",
			"source_sha",
			"operator_surface",
			"route_readback",
			"worker_access",
			"smoke_scope",
			"rollback_plan",
			"evidence_contract",
			"next_owner",
			"next_action",
			"handoff_deadline",
		},
		OptionalFields: []string{"known_limits", "manual_checks", "forbidden_actions", "approval_boundary", "related_batch", "release_url"},
	}}
}

func parsePacketTemplateFields(fields []string) (map[string][]string, error) {
	values := map[string][]string{}
	for _, raw := range fields {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("packet template field %q must use key=value", raw)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("packet template field %q has empty key", raw)
		}
		values[key] = append(values[key], strings.TrimSpace(value))
	}
	return values, nil
}

func validatePacketTemplateValues(template config.PacketTemplate, values map[string][]string) error {
	allowed := map[string]bool{}
	for _, field := range template.RequiredFields {
		allowed[field] = true
		if !hasNonEmptyPacketTemplateValue(values[field]) {
			return fmt.Errorf("packet template %q requires field %q", template.Name, field)
		}
	}
	for _, field := range template.OptionalFields {
		allowed[field] = true
	}
	for field := range values {
		if !allowed[field] {
			return fmt.Errorf("packet template %q does not define field %q", template.Name, field)
		}
	}
	return nil
}

func hasNonEmptyPacketTemplateValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func packetTemplateFieldOrder(template config.PacketTemplate) []string {
	fields := append([]string(nil), template.RequiredFields...)
	fields = append(fields, template.OptionalFields...)
	return fields
}

func packetTemplateTitle(name string) string {
	words := strings.Fields(strings.ReplaceAll(name, "-", " "))
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func printPacketTemplateField(field string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Printf("\n### %s\n", strings.ReplaceAll(field, "_", " "))
	for _, value := range values {
		fmt.Printf("- %s\n", value)
	}
}

func packetTemplateAuthorization(name string) string {
	switch name {
	case "environment-deploy-preflight":
		return "Packet rendering and instantiation are readiness/handoff context only. They do not authorize live execution, deploy, rollback, production mutation, credential use, approval acceptance, merge, release, DNS/proxy mutation, public exposure, or dashboard write/send authority."
	default:
		return "Packet rendering is advisory context only. It does not authorize approval, merge, deploy, release, live execution, provider wake, or dashboard mutation."
	}
}

func instantiatePacketTemplate(ctx context.Context, s *store.Store, template config.PacketTemplate, task store.Task, values map[string][]string, instantiateWaits bool, childTaskSpecs []string) ([]waitRow, []store.TaskDefinition, error) {
	var waits []waitRow
	var children []store.TaskDefinition
	if instantiateWaits {
		if template.Name != "environment-deploy-preflight" {
			return nil, nil, fmt.Errorf("--instantiate-waits is only supported for environment-deploy-preflight packets")
		}
		var err error
		waits, err = instantiateEnvironmentRehearsalWaits(ctx, s, task, values)
		if err != nil {
			return nil, nil, err
		}
	}
	if len(childTaskSpecs) > 0 {
		if template.Name != "environment-deploy-preflight" {
			return nil, nil, fmt.Errorf("--child-task is only supported for environment-deploy-preflight packets")
		}
		var err error
		children, err = instantiateEnvironmentRehearsalChildTasks(ctx, s, task, values, childTaskSpecs)
		if err != nil {
			return nil, nil, err
		}
	}
	return waits, children, nil
}

func instantiateEnvironmentRehearsalWaits(ctx context.Context, s *store.Store, task store.Task, values map[string][]string) ([]waitRow, error) {
	owner := firstTemplateValue(values, "next_owner")
	if owner == "" {
		owner = task.Definition.Role
	}
	deadline := firstTemplateValue(values, "handoff_deadline")
	nextAction := firstTemplateValue(values, "next_action")
	if deadline != "" {
		if _, err := parseFlexibleTime(deadline); err != nil {
			return nil, fmt.Errorf("environment-deploy-preflight handoff_deadline %q is invalid: %w", deadline, err)
		}
	}
	var rows []waitRow
	for _, field := range environmentRehearsalCheckFields() {
		for _, value := range values[field] {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			condition := "environment-deploy-preflight/" + field
			waitID := manualWaitID(task.Definition.ID, "environment-rehearsal", owner, condition)
			payload := manualWaitPayload{
				Event:            "add",
				WaitID:           waitID,
				Kind:             "environment-rehearsal",
				TaskID:           task.Definition.ID,
				TrackID:          owner,
				Condition:        condition,
				Target:           firstTemplateValue(values, "operator_surface"),
				Action:           firstNonEmpty(nextAction, "record_"+field+"_evidence"),
				Reason:           fmt.Sprintf("packet=environment-deploy-preflight check=%s value=%s", field, value),
				DeadlineSource:   "environment-deploy-preflight.handoff_deadline",
				SuggestedCommand: fmt.Sprintf("fairway wait list --task %s --kind environment-rehearsal", task.Definition.ID),
			}
			summary, err := encodeManualWaitSummary(payload)
			if err != nil {
				return nil, err
			}
			if err := s.RecordCheckpoint(ctx, store.Checkpoint{
				TaskID:        payload.TaskID,
				State:         "awaiting_input",
				Owner:         payload.TrackID,
				TargetCloseBy: deadline,
				Summary:       summary,
			}); err != nil {
				return nil, err
			}
			rows = append(rows, waitRow{
				WaitID:           waitID,
				Kind:             payload.Kind,
				TaskID:           payload.TaskID,
				TrackID:          payload.TrackID,
				Owner:            payload.TrackID,
				Condition:        payload.Condition,
				Target:           payload.Target,
				State:            "open",
				Action:           payload.Action,
				Reason:           payload.Reason,
				Deadline:         deadline,
				DeadlineSource:   payload.DeadlineSource,
				Source:           "manual_wait",
				SuggestedCommand: payload.SuggestedCommand,
			})
		}
	}
	return rows, nil
}

func instantiateEnvironmentRehearsalChildTasks(ctx context.Context, s *store.Store, parent store.Task, values map[string][]string, specs []string) ([]store.TaskDefinition, error) {
	owner := firstTemplateValue(values, "next_owner")
	if owner == "" {
		owner = parent.Definition.Role
	}
	var children []store.TaskDefinition
	for _, raw := range specs {
		childID, field, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--child-task %q must use id=field", raw)
		}
		childID = strings.TrimSpace(childID)
		field = strings.TrimSpace(field)
		if childID == "" || field == "" {
			return nil, fmt.Errorf("--child-task %q must use non-empty id=field", raw)
		}
		if !containsString(environmentRehearsalCheckFields(), field) {
			return nil, fmt.Errorf("--child-task field %q is not an environment-deploy-preflight check field", field)
		}
		value := strings.Join(cleanRepeatedValues(values[field]), "; ")
		if value == "" {
			return nil, fmt.Errorf("--child-task field %q has no packet value", field)
		}
		children = append(children, store.TaskDefinition{
			ID:               childID,
			ParentID:         parent.Definition.ID,
			Kind:             "workflow-guard",
			Title:            fmt.Sprintf("Rehearse %s for %s", strings.ReplaceAll(field, "_", " "), parent.Definition.ID),
			Role:             owner,
			Notes:            fmt.Sprintf("packet=environment-deploy-preflight check=%s value=%s\n%s", field, value, packetTemplateAuthorization("environment-deploy-preflight")),
			AcceptanceChecks: []string{fmt.Sprintf("Record evidence for %s: %s", field, value), "Do not treat this child task as deploy/live execution authority."},
			Dependencies:     []string{parent.Definition.ID},
			Profile:          parent.Definition.Profile,
			OwningDomain:     parent.Definition.OwningDomain,
			OwningLayer:      "environment-rehearsal",
			Tags:             []string{"environment-rehearsal", "packet:environment-deploy-preflight", "check:" + field},
			RiskLevel:        "medium",
			MigrationType:    "rehearsal-packet-child",
		})
	}
	if len(children) > 0 {
		for _, child := range children {
			if err := s.AddTask(ctx, child); err != nil {
				return nil, err
			}
		}
	}
	return children, nil
}

func environmentRehearsalCheckFields() []string {
	return []string{"route_readback", "worker_access", "smoke_scope", "rollback_plan", "evidence_contract"}
}

func firstTemplateValue(values map[string][]string, field string) string {
	for _, value := range values[field] {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type taskMetadataFlags struct {
	Profile       *string
	OwningDomain  *string
	OwningLayer   *string
	SourcePaths   *multiFlag
	TargetPaths   *multiFlag
	ReviewDomains *multiFlag
	Tags          *multiFlag
	RiskLevel     *string
	MigrationType *string
}

func addTaskMetadataFlags(fs *flag.FlagSet) taskMetadataFlags {
	var sourcePaths multiFlag
	var targetPaths multiFlag
	var reviewDomains multiFlag
	var tags multiFlag
	fs.Var(&sourcePaths, "source-paths", "comma-separated source paths; repeatable")
	fs.Var(&targetPaths, "target-paths", "comma-separated target paths; repeatable")
	fs.Var(&reviewDomains, "review-domains", "comma-separated review domains; repeatable")
	fs.Var(&tags, "tag", "task tag; repeatable or comma-separated; supports key:value")
	return taskMetadataFlags{
		Profile:       fs.String("profile", "", "workstream profile name"),
		OwningDomain:  fs.String("owning-domain", "", "owning domain metadata"),
		OwningLayer:   fs.String("owning-layer", "", "owning layer metadata"),
		SourcePaths:   &sourcePaths,
		TargetPaths:   &targetPaths,
		ReviewDomains: &reviewDomains,
		Tags:          &tags,
		RiskLevel:     fs.String("risk-level", "", "risk level metadata"),
		MigrationType: fs.String("migration-type", "", "migration type metadata"),
	}
}

func applyTaskMetadataFlags(task *store.TaskDefinition, metadata taskMetadataFlags, changed map[string]bool) {
	if changed == nil || changed["profile"] {
		task.Profile = *metadata.Profile
	}
	if changed == nil || changed["owning-domain"] {
		task.OwningDomain = *metadata.OwningDomain
	}
	if changed == nil || changed["owning-layer"] {
		task.OwningLayer = *metadata.OwningLayer
	}
	if changed == nil || changed["source-paths"] {
		task.SourcePaths = splitRepeatedCSV(*metadata.SourcePaths)
	}
	if changed == nil || changed["target-paths"] {
		task.TargetPaths = splitRepeatedCSV(*metadata.TargetPaths)
	}
	if changed == nil || changed["review-domains"] {
		task.ReviewDomains = splitRepeatedCSV(*metadata.ReviewDomains)
	}
	if changed == nil || changed["tag"] {
		task.Tags = splitRepeatedCSV(*metadata.Tags)
	}
	if changed == nil || changed["risk-level"] {
		task.RiskLevel = *metadata.RiskLevel
	}
	if changed == nil || changed["migration-type"] {
		task.MigrationType = *metadata.MigrationType
	}
}

func copyTaskMetadata(task *store.TaskDefinition, source store.TaskDefinition) {
	task.Profile = source.Profile
	task.OwningDomain = source.OwningDomain
	task.OwningLayer = source.OwningLayer
	task.SourcePaths = append([]string(nil), source.SourcePaths...)
	task.TargetPaths = append([]string(nil), source.TargetPaths...)
	task.ReviewDomains = append([]string(nil), source.ReviewDomains...)
	task.Tags = append([]string(nil), source.Tags...)
	task.RiskLevel = source.RiskLevel
	task.MigrationType = source.MigrationType
}

func cmdHealthReport(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected health-report arguments: %s", strings.Join(args, " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		health, err := s.Health(ctx)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(health)
		}
		fmt.Printf("in_progress: %d\n", health.InProgress)
		fmt.Printf("stale_in_progress: %d\n", health.StaleInProgress)
		fmt.Printf("blocked_over_24h: %d\n", health.BlockedOver24h)
		fmt.Printf("unacknowledged_handoffs: %d\n", health.UnacknowledgedHandoff)
		fmt.Printf("unacknowledged_handoffs_over_1h: %d\n", health.UnacknowledgedOver1Hour)
		fmt.Printf("unrouted_reviews: %d\n", health.UnroutedReviews)
		return nil
	})
}

func cmdClaim(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	within := fs.String("in", "", "claim first ready descendant of task")
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		taskID := args[0]
		if len(args) > 1 {
			return fmt.Errorf("unexpected claim arguments: %s", strings.Join(args[1:], " "))
		}
		return claimTask(ctx, opts, taskID)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *within == "" {
		return errors.New("claim requires task id or --in")
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected claim arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		role := resolveRole(opts)
		tasks, err := s.Ready(ctx, role, cfg.States.Terminal)
		if err != nil {
			return err
		}
		all, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		tasks = filterByAncestor(tasks, all, *within)
		if len(tasks) == 0 {
			return fmt.Errorf("no ready tasks under %s", *within)
		}
		owner := role
		if owner == "" {
			owner = "manual"
		}
		if err := s.Claim(ctx, tasks[0].Definition.ID, owner, fairwaygit.CurrentBranch(root)); err != nil {
			return err
		}
		fmt.Println("claimed", tasks[0].Definition.ID)
		return nil
	})
}

func claimTask(ctx context.Context, opts globalOptions, taskID string) error {
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		owner := resolveRole(opts)
		if owner == "" {
			owner = "manual"
		}
		if err := s.Claim(ctx, taskID, owner, fairwaygit.CurrentBranch(root)); err != nil {
			return err
		}
		fmt.Println("claimed", taskID)
		return nil
	})
}

func cmdReady(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("ready", flag.ContinueOnError)
	priority := fs.Int("priority", -1, "maximum priority rank")
	within := fs.String("in", "", "only show descendants of task")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected ready arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		role := resolveRole(opts)
		tasks, err := s.Ready(ctx, role, cfg.States.Terminal)
		if err != nil {
			return err
		}
		allTasks, err := s.AllTasks(ctx)
		if err != nil {
			return err
		}
		explainTasks := filterReadyExplanationTasks(allTasks, role)
		if *priority >= 0 {
			tasks = filterByPriority(tasks, *priority)
			explainTasks = filterByPriority(explainTasks, *priority)
		}
		if *within != "" {
			tasks = filterByAncestor(tasks, allTasks, *within)
			explainTasks = filterByAncestor(explainTasks, allTasks, *within)
		}
		sessions, err := s.Sessions(ctx, false)
		if err != nil {
			return err
		}
		checkpoints, err := s.Checkpoints(ctx, "", true)
		if err != nil {
			return err
		}
		explanation := coord.ExplainReadyQueue(explainTasks, tasks, sessions, checkpoints, cfg.States.Terminal)
		report := readyReport{Tasks: tasks, Readiness: explanation, ClaimableCount: explanation.ClaimableCount, NonReadyTodoCount: explanation.NonReadyTodoCount, Blockers: explanation.Blockers}
		if opts.JSON {
			return printJSON(report)
		}
		printTasks(tasks)
		if len(tasks) == 0 && explanation.NonReadyTodoCount > 0 {
			printReadyExplanation(explanation)
		}
		return nil
	})
}

type readyReport struct {
	Tasks             []store.Task                     `json:"tasks"`
	ClaimableCount    int                              `json:"claimable_count"`
	NonReadyTodoCount int                              `json:"non_ready_todo_count"`
	Blockers          []coord.ReadinessBlockerCategory `json:"blocker_categories,omitempty"`
	Readiness         coord.ReadinessExplanation       `json:"readiness_explanation"`
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

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func cleanRepeatedValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func changedDocs(paths []string) bool {
	for _, path := range paths {
		if strings.HasPrefix(path, "docs/") ||
			strings.HasPrefix(path, "website/") ||
			strings.HasPrefix(path, "agents/") ||
			path == "README.md" ||
			path == "AGENTS.md" ||
			path == "CLAUDE.md" ||
			path == "CHANGELOG.md" ||
			strings.HasSuffix(path, ".md") {
			return true
		}
	}
	return false
}

func changedCode(paths []string) bool {
	for _, path := range paths {
		switch {
		case strings.HasPrefix(path, "cmd/"),
			strings.HasPrefix(path, "internal/"),
			strings.HasPrefix(path, "examples/session-adapters/"),
			strings.HasSuffix(path, ".go"),
			strings.HasSuffix(path, ".sh"):
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	return visited
}

func cmdInit(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	refreshAgentContract := fs.Bool("refresh-agent-contract", false, "overwrite .fairway/AGENTS.md with the current generated contract")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected init arguments: %s", strings.Join(fs.Args(), " "))
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	path := config.DefaultConfigPath
	if opts.ConfigPath != "" {
		path = opts.ConfigPath
	}
	agentContractPath := filepath.Join(filepath.Dir(path), "AGENTS.md")
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("%s already exists; leaving existing config unchanged\n", path)
		if err := ensureInitAgentContract(agentContractPath, *refreshAgentContract); err != nil {
			return err
		}
		printInitAgentBootstrap(agentContractPath)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := config.WriteDefault(path, root); err != nil {
		return err
	}
	if err := ensureInitAgentContract(agentContractPath, *refreshAgentContract); err != nil {
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
	if err := s.SetTaskIDPattern(cfg.Fairway.TaskIDPattern); err != nil {
		_ = s.Close()
		return err
	}
	defer s.Close()
	fmt.Println("initialized fairway")
	printInitAgentBootstrap(agentContractPath)
	return nil
}

func cmdAgentGuide(args []string) error {
	fs := flag.NewFlagSet("agent-guide", flag.ContinueOnError)
	printPath := fs.Bool("path", false, "print the embedded guide source path and version")
	outputPath := fs.String("output", "", "write the embedded guide to this path instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected agent-guide arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *printPath && *outputPath != "" {
		return errors.New("agent-guide accepts either --path or --output, not both")
	}
	if *printPath {
		fmt.Printf("docs/agent-guide.md version=%s source=%s\n", version, fairwayVersionedAgentGuideURL())
		return nil
	}
	if *outputPath != "" {
		if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(*outputPath, []byte(fairwaydocs.AgentGuide), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", *outputPath)
		return nil
	}
	fmt.Print(fairwaydocs.AgentGuide)
	if !strings.HasSuffix(fairwaydocs.AgentGuide, "\n") {
		fmt.Println()
	}
	return nil
}

func ensureInitAgentContract(path string, refresh bool) error {
	if _, err := os.Stat(path); err == nil && !refresh {
		fmt.Printf("%s already exists; leaving edited agent contract unchanged (use fairway init --refresh-agent-contract to regenerate)\n", path)
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(initAgentContract()), 0o644); err != nil {
		return err
	}
	if refresh {
		fmt.Printf("refreshed %s\n", path)
	} else {
		fmt.Printf("wrote %s\n", path)
	}
	return nil
}

func initAgentContract() string {
	return fmt.Sprintf(`# Fairway Agent Contract

This repository uses Fairway for multi-agent engineering coordination.

## Execution Source Of Truth

Fairway task state, sessions, checkpoints, evidence, reviews, and merge
readiness live in the Fairway database and must be changed with Fairway
commands. Do not edit SQLite rows, generated dashboard artifacts, or queue state
out-of-band.

## Start Of Session Ritual

Run these commands before editing code:

`+"```bash"+`
fairway config validate
fairway preflight --role <role>
fairway ready
fairway task-detail <task-id>
fairway session upsert --id <provider-session-id> --provider <codex|claude|gemini|shell> --role <role> --task-id <task-id> --status running
fairway checkpoint record <task-id> --state active --owner <role> --summary "Started work in <provider-session-id>"
`+"```"+`

## Role Resolution Order

Resolve the active role in this order: explicit `+"`--as <role>`"+` or command
flag, `+"`FAIRWAY_ROLE`"+`, the current Fairway session/task owner, configured
worktree role, then coordinator instruction. If the role is still ambiguous,
ask the coordinator before claiming or editing.

## Session Registration Expectation

Active provider work needs both task state and session state. Before editing,
register or refresh the provider attachment with `+"`fairway session upsert`"+`
and record an active checkpoint or provider event. Provider chat is useful
context, but Fairway remains the coordination source of truth.

## Full Guide

For an offline copy embedded in the installed binary, run:

`+"```bash"+`
fairway agent-guide
`+"```"+`

Read the source guide that matches the installed Fairway release:

%s

`, fairwayVersionedAgentGuideURL())
}

func fairwayVersionedAgentGuideURL() string {
	ref := strings.TrimSpace(version)
	if ref == "" || strings.Contains(ref, "dev") {
		ref = "main"
	}
	return fmt.Sprintf("https://github.com/fairway-run/fairway/blob/%s/docs/agent-guide.md", ref)
}

func printInitAgentBootstrap(agentContractPath string) {
	fmt.Println()
	fmt.Println("Root AGENTS.md / CLAUDE.md bootstrap block:")
	fmt.Println("```markdown")
	fmt.Println("## Fairway")
	fmt.Println()
	fmt.Println("This repo uses Fairway for multi-agent coordination.")
	fmt.Printf("Read `%s` before changing code.\n", filepath.ToSlash(agentContractPath))
	fmt.Println("Use Fairway commands as the source of truth for tasks, sessions, checkpoints, evidence, reviews, and merge readiness.")
	fmt.Println("Offline guide: `fairway agent-guide`")
	fmt.Printf("Full guide: %s\n", fairwayVersionedAgentGuideURL())
	fmt.Println("```")
}

func cmdImport(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("import requires yaml or json path")
	}
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	stateOnce := fs.Bool("state-once", false, "seed legacy mutable state once")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected import arguments: %s", strings.Join(fs.Args(), " "))
	}
	result, err := importer.File(args[0])
	if err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		if err := applyImportDefaults(result.Tasks, cfg); err != nil {
			return err
		}
		if err := validateTaskMetadata(result.Tasks, cfg); err != nil {
			return err
		}
		if *stateOnce {
			if err := validateImportedStates(result.States, cfg); err != nil {
				return err
			}
		}
		if err := s.ImportTasks(ctx, result.Tasks); err != nil {
			return err
		}
		appliedStates := 0
		if *stateOnce {
			var err error
			appliedStates, err = s.ImportTaskStatesOnce(ctx, result.States)
			if err != nil {
				return err
			}
		}
		if *stateOnce {
			fmt.Printf("imported %d tasks, seeded %d states\n", len(result.Tasks), appliedStates)
			return nil
		}
		fmt.Printf("imported %d tasks\n", len(result.Tasks))
		return nil
	})
}

func applyImportDefaults(tasks []store.TaskDefinition, cfg config.Config) error {
	defaultKind := config.DefaultTaskKind(cfg)
	defaultPriority := config.DefaultPriority(cfg)
	for i := range tasks {
		if tasks[i].Kind == "" {
			tasks[i].Kind = defaultKind
		}
		if tasks[i].Priority == nil {
			tasks[i].Priority = defaultPriority
		}
	}
	return nil
}

func validateTaskMetadata(tasks []store.TaskDefinition, cfg config.Config) error {
	roles := config.RoleSet(cfg)
	kinds := config.TaskKindSet(cfg)
	priorities := config.PrioritySet(cfg)
	profiles := config.WorkstreamProfileSet(cfg)
	for _, task := range tasks {
		if len(roles) > 0 && !roles[task.Role] {
			return fmt.Errorf("task %s uses unknown role %q", task.ID, task.Role)
		}
		if len(kinds) > 0 && !kinds[task.Kind] {
			return fmt.Errorf("task %s uses unknown kind %q", task.ID, task.Kind)
		}
		if task.Priority != nil && len(priorities) > 0 && !priorities[*task.Priority] {
			return fmt.Errorf("task %s uses unknown priority %d", task.ID, *task.Priority)
		}
		if task.Profile != "" && len(profiles) > 0 && !profiles[task.Profile] {
			return fmt.Errorf("task %s uses unknown profile %q", task.ID, task.Profile)
		}
	}
	return nil
}

func validateImportedStates(states []store.ImportedTaskState, cfg config.Config) error {
	allowed := map[string]bool{}
	for _, state := range cfg.States.Allowed {
		allowed[state] = true
	}
	for _, state := range states {
		if state.Status == "" {
			continue
		}
		if !allowed[state.Status] {
			return fmt.Errorf("task %s imports unknown status %q", state.TaskID, state.Status)
		}
	}
	return nil
}

func validateReviewer(reviewer string, cfg config.Config) error {
	roles := config.RoleSet(cfg)
	if len(roles) > 0 && !roles[reviewer] {
		return fmt.Errorf("unknown reviewer %q", reviewer)
	}
	return nil
}

func matchReviewRoute(routes []config.ReviewRoute, paths []string) (string, string) {
	for _, changedPath := range paths {
		for _, route := range routes {
			if routeMatches(route.Match, changedPath) {
				return route.Reviewer, fmt.Sprintf("%s matched %s", changedPath, route.Match)
			}
		}
	}
	return "", ""
}

func routeMatches(pattern, changedPath string) bool {
	if pattern == "**" {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		return strings.HasPrefix(changedPath, strings.TrimSuffix(pattern, "**"))
	}
	ok, err := path.Match(pattern, changedPath)
	return err == nil && ok
}

func cmdSetStatus(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 2 {
		return errors.New("set-status requires task id and state")
	}
	fs := flag.NewFlagSet("set-status", flag.ContinueOnError)
	reason := fs.String("reason", "", "reason")
	commitSHA := fs.String("commit", "", "commit SHA to associate with a terminal status; defaults to HEAD for terminal CLI transitions")
	reopen := fs.Bool("reopen", false, "reopen terminal task")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		if err := setStatusWithConfig(ctx, cfg, root, s, args[0], args[1], *reason, *commitSHA, *reopen); err != nil {
			return err
		}
		fmt.Println("status", args[0], args[1])
		return nil
	})
}

func setStatusWithConfig(ctx context.Context, cfg config.Config, root string, s *store.Store, taskID, target, reason, commitSHA string, reopen bool) error {
	current, err := s.CurrentStatus(ctx, taskID)
	if err != nil {
		return err
	}
	stateCfg := state.Config{Allowed: cfg.States.Allowed, Terminal: cfg.States.Terminal, Transitions: cfg.States.Transitions}
	terminal := state.IsTerminal(stateCfg, target)
	if err := state.ValidateTransition(stateCfg, current, target, reopen); err != nil {
		return err
	}
	if terminal {
		if err := validateTerminalGates(ctx, cfg, s, taskID); err != nil {
			return err
		}
		if strings.TrimSpace(commitSHA) == "" {
			commitSHA = fairwaygit.LastCommit(root)
		}
	}
	if !terminal {
		commitSHA = ""
	}
	return s.SetStatusWithCommit(ctx, taskID, target, reason, commitSHA, cfg.Gates.RequireBlockedReason)
}

func cmdRecord(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 2 {
		if isHelpOnly(args) {
			subcommandUsage("record", "evidence|guard-report|handoff|completion-handback|completion-handback-supersede|notification|review|usage|push-intent")
			return nil
		}
		return errors.New("record requires type and task id")
	}
	if isHelpOnly(args) {
		subcommandUsage("record", "evidence|guard-report|handoff|completion-handback|completion-handback-supersede|notification|review|usage|push-intent")
		return nil
	}
	switch args[0] {
	case "evidence":
		return recordEvidence(ctx, opts, args[1:])
	case "guard-report":
		return recordGuardReport(ctx, opts, args[1:])
	case "handoff":
		return recordHandoff(ctx, opts, args[1:])
	case "completion-handback":
		return recordCompletionHandback(ctx, opts, args[1:])
	case "completion-handback-supersede":
		return recordCompletionHandbackSupersede(ctx, opts, args[1:])
	case "notification":
		return recordNotification(ctx, opts, args[1:])
	case "review":
		return recordReview(ctx, opts, args[1:])
	case "usage":
		return recordUsage(ctx, opts, args[1:])
	case "push-intent":
		return recordPushIntent(ctx, opts, args[1:])
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
	duration := fs.Int("duration-seconds", 0, "duration in seconds")
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
		var durationPtr *int
		if *duration > 0 {
			durationPtr = duration
		}
		if err := s.RecordEvidence(ctx, taskID, store.Evidence{CommandText: *commandText, Result: *result, ArtifactPath: *artifact, ArtifactType: *artifactType, DurationSeconds: durationPtr, Notes: *notes}); err != nil {
			return err
		}
		if !opts.JSON {
			fmt.Println("evidence recorded", taskID)
			printEvidenceStatusPrompt(*result)
		}
		return nil
	})
}

func cmdRules(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("rules", "validate <dir>|evidence-types|match <task-id>")
		return nil
	}
	switch args[0] {
	case "validate":
		return cmdRulesValidate(opts, args[1:])
	case "evidence-types":
		return cmdRulesEvidenceTypes(ctx, opts, args[1:])
	case "match":
		return cmdRulesMatch(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown rules subcommand %q", args[0])
	}
}

func cmdRulesValidate(opts globalOptions, args []string) error {
	if len(args) != 1 {
		return errors.New("rules validate requires rule-pack directory")
	}
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	dir := args[0]
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	pack, err := rules.LoadDir(dir, filepath.Base(dir), "advisory", rules.LoadOptions{Root: root, KnownDomains: rules.ReviewDomainSet(cfg)})
	if err != nil {
		return err
	}
	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(pack)
	}
	printRulePackSummary(pack)
	if rules.HasErrors(pack.Findings) {
		return errors.New("rule pack validation failed")
	}
	return nil
}

func cmdRulesEvidenceTypes(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected rules evidence-types arguments: %s", strings.Join(args, " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		recorded, err := s.EvidenceTypes(ctx)
		if err != nil {
			return err
		}
		knownEvidence := rules.ConfigGateEvidenceSet(cfg)
		for _, evidence := range recorded {
			knownEvidence[evidence] = true
		}
		packs, err := rules.LoadConfigured(cfg, root, rules.LoadOptions{
			Root:          root,
			KnownDomains:  rules.ReviewDomainSet(cfg),
			KnownEvidence: knownEvidence,
		})
		if err != nil {
			return err
		}
		type row struct {
			EvidenceType string   `json:"evidence_type"`
			FromRules    []string `json:"from_rules,omitempty"`
			ConfigGate   bool     `json:"config_gate,omitempty"`
			Recorded     bool     `json:"recorded,omitempty"`
		}
		byRule := rules.EvidenceTypes(packs)
		recordedSet := map[string]bool{}
		for _, evidence := range recorded {
			recordedSet[evidence] = true
		}
		keys := map[string]bool{}
		for evidence := range byRule {
			keys[evidence] = true
		}
		for evidence := range rules.ConfigGateEvidenceSet(cfg) {
			keys[evidence] = true
		}
		for evidence := range recordedSet {
			keys[evidence] = true
		}
		var names []string
		for evidence := range keys {
			names = append(names, evidence)
		}
		sort.Strings(names)
		var rows []row
		for _, evidence := range names {
			rows = append(rows, row{
				EvidenceType: evidence,
				FromRules:    byRule[evidence],
				ConfigGate:   rules.ConfigGateEvidenceSet(cfg)[evidence],
				Recorded:     recordedSet[evidence],
			})
		}
		if opts.JSON {
			return json.NewEncoder(os.Stdout).Encode(struct {
				Packs         []rules.Pack `json:"packs"`
				EvidenceTypes []row        `json:"evidence_types"`
			}{Packs: packs, EvidenceTypes: rows})
		}
		for _, pack := range packs {
			printRulePackSummary(pack)
		}
		fmt.Println("evidence types:")
		for _, row := range rows {
			parts := []string{}
			if len(row.FromRules) > 0 {
				parts = append(parts, "rules="+strings.Join(row.FromRules, ","))
			}
			if row.ConfigGate {
				parts = append(parts, "config_gate")
			}
			if row.Recorded {
				parts = append(parts, "recorded")
			}
			fmt.Printf("- %s", row.EvidenceType)
			if len(parts) > 0 {
				fmt.Printf(" (%s)", strings.Join(parts, " "))
			}
			fmt.Println()
		}
		return nil
	})
}

func cmdRulesMatch(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) != 1 {
		return errors.New("rules match requires task id")
	}
	taskID := args[0]
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		task, _, _, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		packs, err := rules.LoadConfigured(cfg, root, rules.LoadOptions{
			Root:            root,
			KnownDomains:    rules.ReviewDomainSet(cfg),
			KnownEvidence:   rules.ConfigGateEvidenceSet(cfg),
			IncludeDisabled: true,
		})
		if err != nil {
			return err
		}
		matches := rules.MatchTask(cfg, packs, task)
		if opts.JSON {
			return json.NewEncoder(os.Stdout).Encode(matches)
		}
		fmt.Printf("rules for %s:\n", taskID)
		for _, match := range matches {
			fmt.Printf("- %s [%s] group=%s risk_floor=%s", match.Rule.ID, match.Status, match.Rule.Group, firstNonEmpty(match.Rule.RiskFloor, "none"))
			if len(match.Reasons) > 0 {
				fmt.Printf(" reason=%s", strings.Join(match.Reasons, "; "))
			}
			fmt.Println()
		}
		return nil
	})
}

func printRulePackSummary(pack rules.Pack) {
	fmt.Printf("rule pack %s: rules=%d groups=%d findings=%d\n", pack.SourceName, len(pack.Rules), len(pack.Groups), len(pack.Findings))
	for _, group := range pack.Groups {
		fmt.Printf("- group: %s\n", group)
	}
	for _, finding := range pack.Findings {
		fmt.Printf("- %s: %s", finding.Severity, finding.Message)
		if finding.Path != "" {
			fmt.Printf(" (%s)", finding.Path)
		}
		fmt.Println()
	}
}

func printEvidenceStatusPrompt(result string) {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "pass":
		fmt.Println("next: mark done, or record an active checkpoint with --target-close-by explaining the bounded closeout window")
	case "fail", "partial", "skipped", "blocked":
		fmt.Println("next: set blocked, reset to todo with a reason, or create/record explicit follow-up work")
	default:
		fmt.Println("next: run fairway reconcile active before leaving this work block")
	}
}

func recordPushIntent(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("record push-intent requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("push-intent", flag.ContinueOnError)
	intent := fs.String("intent", "", "push intent: main-validation, integration, review, release, backup, or exception")
	branch := fs.String("branch", "", "branch being pushed; defaults to current branch")
	remote := fs.String("remote", "origin", "remote name")
	reason := fs.String("reason", "", "required for exception intent")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	normalizedIntent := strings.TrimSpace(*intent)
	if !validRecordPushIntent(normalizedIntent) {
		return fmt.Errorf("unsupported push intent %q", normalizedIntent)
	}
	if normalizedIntent == "exception" && strings.TrimSpace(*reason) == "" {
		return errors.New("exception push intent requires --reason")
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, root string, s *store.Store) error {
		recordBranch := strings.TrimSpace(*branch)
		if recordBranch == "" {
			recordBranch = fairwaygit.CurrentBranch(root)
		}
		if recordBranch == "" {
			return errors.New("branch is required when current branch cannot be detected")
		}
		recordRemote := strings.TrimSpace(*remote)
		if recordRemote == "" {
			recordRemote = "origin"
		}
		commandText := fmt.Sprintf("fairway record push-intent %s intent=%s branch=%s remote=%s", taskID, normalizedIntent, recordBranch, recordRemote)
		notes := fmt.Sprintf("intent=%s branch=%s remote=%s", normalizedIntent, recordBranch, recordRemote)
		if strings.TrimSpace(*reason) != "" {
			commandText += " reason=" + shellToken(strings.TrimSpace(*reason))
			notes += " reason=" + shellToken(strings.TrimSpace(*reason))
		}
		ev := store.Evidence{
			CommandText:  commandText,
			Result:       "pass",
			ArtifactPath: recordRemote + "/" + recordBranch,
			ArtifactType: "push-intent",
			Notes:        notes,
		}
		if err := s.RecordEvidence(ctx, taskID, ev); err != nil {
			return err
		}
		if !opts.JSON {
			fmt.Printf("push intent recorded %s intent=%s branch=%s remote=%s\n", taskID, normalizedIntent, recordBranch, recordRemote)
		}
		return nil
	})
}

func validRecordPushIntent(intent string) bool {
	switch intent {
	case "main-validation", "integration", "review", "release", "backup", "exception":
		return true
	default:
		return false
	}
}

func shellToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "\t", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	return value
}

func recordGuardReport(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("record guard-report requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("guard-report", flag.ContinueOnError)
	guard := fs.String("guard", "", "guard name")
	mode := fs.String("mode", "report_only", "guard mode: report_only, warning, or blocking")
	result := fs.String("result", "", "evidence result override")
	artifact := fs.String("artifact", "", "artifact path or URL")
	graduation := fs.String("graduation-criteria", "", "graduation criteria")
	var findings multiFlag
	var falsePositives multiFlag
	var allowedDebt multiFlag
	fs.Var(&findings, "finding", "guard finding; may repeat")
	fs.Var(&falsePositives, "false-positive", "false positive; may repeat")
	fs.Var(&allowedDebt, "allowed-debt", "allowed debt; may repeat")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected guard-report arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *guard == "" {
		return errors.New("--guard is required")
	}
	if !validGuardMode(*mode) {
		return fmt.Errorf("invalid guard mode %q", *mode)
	}
	evidenceResult := *result
	if evidenceResult == "" {
		evidenceResult = guardModeResult(*mode)
	}
	report := guardEvidenceReport{
		Guard:              *guard,
		Mode:               *mode,
		Findings:           findings,
		FalsePositives:     falsePositives,
		AllowedDebt:        allowedDebt,
		GraduationCriteria: *graduation,
	}
	notes, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		ev := store.Evidence{
			CommandText:  "guard report: " + *guard,
			Result:       evidenceResult,
			ArtifactPath: *artifact,
			ArtifactType: "guard-report",
			Notes:        string(notes),
		}
		if err := s.RecordEvidence(ctx, taskID, ev); err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(struct {
				TaskID string              `json:"task_id"`
				Result string              `json:"result"`
				Report guardEvidenceReport `json:"report"`
			}{taskID, evidenceResult, report})
		}
		fmt.Printf("recorded guard report for %s: %s (%s)\n", taskID, *guard, evidenceResult)
		return nil
	})
}

func recordUsage(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("record usage requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("usage", flag.ContinueOnError)
	provider := fs.String("provider", "", "provider name")
	externalSessionID := fs.String("external-session-id", "", "external provider session id")
	sessionID := fs.String("session-id", "", "fairway session id")
	role := fs.String("role", "", "role/lane")
	phase := fs.String("phase", "", "work phase")
	source := fs.String("source", "unknown", "usage source: provider_reported, derived_snapshot, manual, unknown")
	confidence := fs.String("confidence", "unknown", "usage confidence: exact, estimated, unknown")
	startedAt := fs.String("started-at", "", "usage window start timestamp")
	completedAt := fs.String("completed-at", "", "usage window end timestamp")
	startedSnapshot := fs.String("started-token-snapshot", "", "starting provider token snapshot")
	completedSnapshot := fs.String("completed-token-snapshot", "", "completed provider token snapshot")
	inputTokens := fs.String("input-tokens", "", "input token count")
	cachedInputTokens := fs.String("cached-input-tokens", "", "cached input token count")
	uncachedInputTokens := fs.String("uncached-input-tokens", "", "uncached input token count")
	outputTokens := fs.String("output-tokens", "", "output token count")
	reasoningTokens := fs.String("reasoning-tokens", "", "reasoning token count")
	totalTokens := fs.String("total-tokens", "", "total token count")
	elapsedSeconds := fs.String("elapsed-seconds", "", "elapsed seconds")
	model := fs.String("model", "", "provider model label")
	var metadata multiFlag
	fs.Var(&metadata, "metadata", "usage metadata key=value; may repeat; no prompts, transcripts, secrets, or generated content")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected usage arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *provider == "" {
		return errors.New("--provider is required")
	}
	metadataJSON, err := usageMetadataJSON(metadata)
	if err != nil {
		return err
	}
	usage := store.ProviderUsage{
		Provider:               *provider,
		ExternalSessionID:      *externalSessionID,
		SessionID:              *sessionID,
		TaskID:                 taskID,
		Role:                   *role,
		Phase:                  *phase,
		Source:                 *source,
		Confidence:             *confidence,
		StartedAt:              *startedAt,
		CompletedAt:            *completedAt,
		StartedTokenSnapshot:   optionalNonNegativeInt(*startedSnapshot),
		CompletedTokenSnapshot: optionalNonNegativeInt(*completedSnapshot),
		InputTokens:            optionalNonNegativeInt(*inputTokens),
		CachedInputTokens:      optionalNonNegativeInt(*cachedInputTokens),
		UncachedInputTokens:    optionalNonNegativeInt(*uncachedInputTokens),
		OutputTokens:           optionalNonNegativeInt(*outputTokens),
		ReasoningTokens:        optionalNonNegativeInt(*reasoningTokens),
		TotalTokens:            optionalNonNegativeInt(*totalTokens),
		ElapsedSeconds:         optionalNonNegativeInt(*elapsedSeconds),
		Model:                  *model,
		MetadataJSON:           metadataJSON,
	}
	if usage.StartedTokenSnapshot == nil && *startedSnapshot != "" {
		return fmt.Errorf("invalid --started-token-snapshot %q", *startedSnapshot)
	}
	if usage.CompletedTokenSnapshot == nil && *completedSnapshot != "" {
		return fmt.Errorf("invalid --completed-token-snapshot %q", *completedSnapshot)
	}
	if usage.InputTokens == nil && *inputTokens != "" {
		return fmt.Errorf("invalid --input-tokens %q", *inputTokens)
	}
	if usage.CachedInputTokens == nil && *cachedInputTokens != "" {
		return fmt.Errorf("invalid --cached-input-tokens %q", *cachedInputTokens)
	}
	if usage.UncachedInputTokens == nil && *uncachedInputTokens != "" {
		return fmt.Errorf("invalid --uncached-input-tokens %q", *uncachedInputTokens)
	}
	if usage.OutputTokens == nil && *outputTokens != "" {
		return fmt.Errorf("invalid --output-tokens %q", *outputTokens)
	}
	if usage.ReasoningTokens == nil && *reasoningTokens != "" {
		return fmt.Errorf("invalid --reasoning-tokens %q", *reasoningTokens)
	}
	if usage.TotalTokens == nil && *totalTokens != "" {
		return fmt.Errorf("invalid --total-tokens %q", *totalTokens)
	}
	if usage.ElapsedSeconds == nil && *elapsedSeconds != "" {
		return fmt.Errorf("invalid --elapsed-seconds %q", *elapsedSeconds)
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		recorded, err := s.RecordProviderUsage(ctx, usage)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(recorded)
		}
		fmt.Printf("usage recorded %s provider=%s total=%s confidence=%s source=%s\n", taskID, recorded.Provider, formatUsageInt(recorded.TotalTokens), recorded.Confidence, recorded.Source)
		return nil
	})
}

func optionalNonNegativeInt(raw string) *int {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return nil
	}
	return &value
}

func usageMetadataJSON(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	metadata := map[string]string{}
	for _, raw := range values {
		key, value, ok := strings.Cut(raw, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return "", fmt.Errorf("usage metadata must be key=value: %q", raw)
		}
		if usageMetadataKeyForbidden(key) {
			return "", fmt.Errorf("usage metadata key %q is not allowed; usage metadata must not contain prompts, transcripts, secrets, inputs, outputs, messages, or generated content", key)
		}
		if len(value) > 512 {
			return "", fmt.Errorf("usage metadata value for %q is too long", key)
		}
		metadata[key] = value
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func usageMetadataKeyForbidden(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, blocked := range []string{"prompt", "transcript", "secret", "password", "cookie", "api_key", "token", "bearer", "authorization", "input", "output", "content", "completion", "message"} {
		if strings.Contains(normalized, blocked) {
			return true
		}
	}
	return false
}

type guardEvidenceReport struct {
	Guard              string   `json:"guard"`
	Mode               string   `json:"mode"`
	Findings           []string `json:"findings,omitempty"`
	FalsePositives     []string `json:"false_positives,omitempty"`
	AllowedDebt        []string `json:"allowed_debt,omitempty"`
	GraduationCriteria string   `json:"graduation_criteria,omitempty"`
}

func validGuardMode(mode string) bool {
	switch mode {
	case "report_only", "warning", "blocking":
		return true
	default:
		return false
	}
}

func cmdUsage(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("usage", "report|cost-report")
		return nil
	}
	switch args[0] {
	case "report":
		return cmdUsageReport(ctx, opts, args[1:])
	case "cost-report":
		return cmdUsageCostReport(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown usage subcommand %q", args[0])
	}
}

func cmdUsageReport(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("usage", "report [--by <provider|task|epic|role|day|kind|phase|model>]")
		return nil
	}
	fs := flag.NewFlagSet("usage report", flag.ContinueOnError)
	groupBy := fs.String("by", "provider", "group by provider, task, epic, role, day, kind, or phase")
	taskID := fs.String("task-id", "", "limit to task id")
	sinceDuration := fs.String("since-duration", "", "limit to usage recorded within duration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected usage report arguments: %s", strings.Join(fs.Args(), " "))
	}
	if !validUsageRollupGroup(*groupBy) {
		return fmt.Errorf("invalid usage report group %q", *groupBy)
	}
	since := ""
	if *sinceDuration != "" {
		duration, err := time.ParseDuration(*sinceDuration)
		if err != nil {
			return err
		}
		since = time.Now().UTC().Add(-duration).Format(time.RFC3339Nano)
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		rollups, err := s.UsageRollups(ctx, store.UsageRollupOptions{GroupBy: *groupBy, TaskID: *taskID, Since: since})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(rollups)
		}
		fmt.Printf("usage report by %s\n", *groupBy)
		for _, roll := range rollups {
			fmt.Printf("- %s events=%d total=%s input=%s cached=%s output=%s elapsed=%s\n", roll.Key, roll.Events, formatUsageInt(roll.TotalTokens), formatUsageInt(roll.InputTokens), formatUsageInt(roll.CachedInputTokens), formatUsageInt(roll.OutputTokens), formatUsageInt(roll.ElapsedSeconds))
		}
		if len(rollups) == 0 {
			fmt.Println("- no usage recorded")
		}
		return nil
	})
}

func validUsageRollupGroup(group string) bool {
	switch group {
	case "provider", "task", "epic", "role", "day", "kind", "phase", "model":
		return true
	default:
		return false
	}
}

type usageCostReport struct {
	GroupBy           string               `json:"group_by"`
	Since             string               `json:"since,omitempty"`
	ForecastDays      float64              `json:"forecast_days,omitempty"`
	PricingConfigured int                  `json:"pricing_configured"`
	Rows              []usageCostReportRow `json:"rows"`
	Totals            usageCostReportRow   `json:"totals"`
}

type usageCostReportRow struct {
	Group               string   `json:"group"`
	Events              int      `json:"events"`
	KnownCostEvents     int      `json:"known_cost_events"`
	UnknownCostEvents   int      `json:"unknown_cost_events"`
	TotalTokens         *int     `json:"total_tokens,omitempty"`
	InputTokens         *int     `json:"input_tokens,omitempty"`
	CachedInputTokens   *int     `json:"cached_input_tokens,omitempty"`
	UncachedInputTokens *int     `json:"uncached_input_tokens,omitempty"`
	OutputTokens        *int     `json:"output_tokens,omitempty"`
	ReasoningTokens     *int     `json:"reasoning_tokens,omitempty"`
	EstimatedCostUSD    *float64 `json:"estimated_cost_usd,omitempty"`
	ForecastCostUSD     *float64 `json:"forecast_cost_usd,omitempty"`
	CacheReadRatio      *float64 `json:"cache_read_ratio,omitempty"`
	PriceStatus         string   `json:"price_status"`
}

type modelPrice struct {
	Provider              string
	Model                 string
	InputPerMillion       *float64
	CachedInputPerMillion *float64
	OutputPerMillion      *float64
	ReasoningPerMillion   *float64
	TotalPerMillion       *float64
}

func cmdUsageCostReport(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("usage", "cost-report [--by <provider|task|epic|role|day|kind|phase|model>] [--forecast-days <n>]")
		return nil
	}
	fs := flag.NewFlagSet("usage cost-report", flag.ContinueOnError)
	groupBy := fs.String("by", "task", "group by provider, task, epic, role, day, kind, phase, or model")
	taskID := fs.String("task-id", "", "limit to task id")
	sinceDuration := fs.String("since-duration", "", "limit to usage recorded within duration")
	forecastDays := fs.Float64("forecast-days", 0, "forecast cost for this many days from the since-duration rate")
	format := fs.String("format", "human", "output format: human or markdown")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected usage cost-report arguments: %s", strings.Join(fs.Args(), " "))
	}
	if !validUsageRollupGroup(*groupBy) {
		return fmt.Errorf("invalid usage cost-report group %q", *groupBy)
	}
	if *forecastDays < 0 {
		return errors.New("--forecast-days must not be negative")
	}
	if *forecastDays > 0 && *sinceDuration == "" {
		return errors.New("--forecast-days requires --since-duration so the forecast window is explicit")
	}
	if *format != "human" && *format != "markdown" {
		return fmt.Errorf("invalid usage cost-report format %q", *format)
	}
	since := ""
	var sinceWindow time.Duration
	if *sinceDuration != "" {
		duration, err := time.ParseDuration(*sinceDuration)
		if err != nil {
			return err
		}
		if duration <= 0 {
			return errors.New("--since-duration must be positive")
		}
		sinceWindow = duration
		since = time.Now().UTC().Add(-duration).Format(time.RFC3339Nano)
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		events, err := s.ProviderUsageEvents(ctx, store.UsageRollupOptions{TaskID: *taskID, Since: since})
		if err != nil {
			return err
		}
		tasks := map[string]store.Task{}
		if *groupBy == "kind" || *groupBy == "epic" {
			all, err := s.AllTasks(ctx)
			if err != nil {
				return err
			}
			for _, task := range all {
				tasks[task.Definition.ID] = task
			}
		}
		report := buildUsageCostReport(events, cfg.ProviderModelPrices, *groupBy, tasks, since, sinceWindow, *forecastDays)
		if opts.JSON {
			return printJSON(report)
		}
		switch *format {
		case "markdown":
			printUsageCostMarkdown(report)
		default:
			printUsageCostHuman(report)
		}
		return nil
	})
}

func buildUsageCostReport(events []store.ProviderUsage, prices []config.ProviderModelPrice, groupBy string, tasks map[string]store.Task, since string, sinceWindow time.Duration, forecastDays float64) usageCostReport {
	priceTable := buildModelPriceTable(prices)
	report := usageCostReport{GroupBy: groupBy, Since: since, ForecastDays: forecastDays, PricingConfigured: len(prices)}
	byGroup := map[string]*usageCostReportRow{}
	for _, ev := range events {
		key := usageCostGroupKey(ev, groupBy, tasks)
		row := byGroup[key]
		if row == nil {
			row = &usageCostReportRow{Group: key, PriceStatus: "unknown_price"}
			byGroup[key] = row
		}
		row.Events++
		report.Totals.Events++
		addUsageCostInt(&row.TotalTokens, ev.TotalTokens)
		addUsageCostInt(&report.Totals.TotalTokens, ev.TotalTokens)
		addUsageCostInt(&row.InputTokens, ev.InputTokens)
		addUsageCostInt(&report.Totals.InputTokens, ev.InputTokens)
		addUsageCostInt(&row.CachedInputTokens, ev.CachedInputTokens)
		addUsageCostInt(&report.Totals.CachedInputTokens, ev.CachedInputTokens)
		addUsageCostInt(&row.UncachedInputTokens, ev.UncachedInputTokens)
		addUsageCostInt(&report.Totals.UncachedInputTokens, ev.UncachedInputTokens)
		addUsageCostInt(&row.OutputTokens, ev.OutputTokens)
		addUsageCostInt(&report.Totals.OutputTokens, ev.OutputTokens)
		addUsageCostInt(&row.ReasoningTokens, ev.ReasoningTokens)
		addUsageCostInt(&report.Totals.ReasoningTokens, ev.ReasoningTokens)
		price, priced := lookupModelPrice(priceTable, ev.Provider, ev.Model)
		if priced {
			if cost, ok := estimateUsageCost(ev, price); ok {
				addUsageFloat(&row.EstimatedCostUSD, cost)
				addUsageFloat(&report.Totals.EstimatedCostUSD, cost)
				row.KnownCostEvents++
				report.Totals.KnownCostEvents++
				row.PriceStatus = combinedPriceStatus(row.PriceStatus, "priced")
				report.Totals.PriceStatus = combinedPriceStatus(report.Totals.PriceStatus, "priced")
			} else {
				row.UnknownCostEvents++
				report.Totals.UnknownCostEvents++
				row.PriceStatus = combinedPriceStatus(row.PriceStatus, "unknown_tokens")
				report.Totals.PriceStatus = combinedPriceStatus(report.Totals.PriceStatus, "unknown_tokens")
			}
		} else {
			row.UnknownCostEvents++
			report.Totals.UnknownCostEvents++
			row.PriceStatus = combinedPriceStatus(row.PriceStatus, "unknown_price")
			report.Totals.PriceStatus = combinedPriceStatus(report.Totals.PriceStatus, "unknown_price")
		}
	}
	report.Rows = make([]usageCostReportRow, 0, len(byGroup))
	for _, row := range byGroup {
		row.CacheReadRatio = usageCacheReadRatio(row.InputTokens, row.CachedInputTokens)
		if forecastDays > 0 && row.EstimatedCostUSD != nil && sinceWindow > 0 {
			forecast := *row.EstimatedCostUSD * forecastDays / sinceWindow.Hours() * 24
			row.ForecastCostUSD = &forecast
		}
		report.Rows = append(report.Rows, *row)
	}
	report.Totals.Group = "total"
	report.Totals.CacheReadRatio = usageCacheReadRatio(report.Totals.InputTokens, report.Totals.CachedInputTokens)
	if forecastDays > 0 && report.Totals.EstimatedCostUSD != nil && sinceWindow > 0 {
		forecast := *report.Totals.EstimatedCostUSD * forecastDays / sinceWindow.Hours() * 24
		report.Totals.ForecastCostUSD = &forecast
	}
	if report.Totals.PriceStatus == "" {
		report.Totals.PriceStatus = "unknown_price"
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		left, right := -1.0, -1.0
		if report.Rows[i].EstimatedCostUSD != nil {
			left = *report.Rows[i].EstimatedCostUSD
		}
		if report.Rows[j].EstimatedCostUSD != nil {
			right = *report.Rows[j].EstimatedCostUSD
		}
		if left != right {
			return left > right
		}
		return report.Rows[i].Group < report.Rows[j].Group
	})
	return report
}

func buildModelPriceTable(prices []config.ProviderModelPrice) map[string]modelPrice {
	out := map[string]modelPrice{}
	for _, price := range prices {
		key := modelPriceKey(price.Provider, price.Model)
		out[key] = modelPrice{
			Provider:              strings.TrimSpace(price.Provider),
			Model:                 strings.TrimSpace(price.Model),
			InputPerMillion:       price.InputPerMillion,
			CachedInputPerMillion: price.CachedInputPerMillion,
			OutputPerMillion:      price.OutputPerMillion,
			ReasoningPerMillion:   price.ReasoningPerMillion,
			TotalPerMillion:       price.TotalPerMillion,
		}
	}
	return out
}

func lookupModelPrice(prices map[string]modelPrice, provider, model string) (modelPrice, bool) {
	for _, key := range []string{
		modelPriceKey(provider, model),
		modelPriceKey(provider, "*"),
		modelPriceKey("*", model),
		modelPriceKey("*", "*"),
	} {
		if price, ok := prices[key]; ok {
			return price, true
		}
	}
	return modelPrice{}, false
}

func modelPriceKey(provider, model string) string {
	return strings.TrimSpace(provider) + "\x00" + strings.TrimSpace(model)
}

func estimateUsageCost(ev store.ProviderUsage, price modelPrice) (float64, bool) {
	total := 0.0
	charged := false
	if price.InputPerMillion != nil {
		uncached := ev.UncachedInputTokens
		if uncached == nil && ev.InputTokens != nil && ev.CachedInputTokens != nil && *ev.InputTokens >= *ev.CachedInputTokens {
			v := *ev.InputTokens - *ev.CachedInputTokens
			uncached = &v
		}
		if uncached == nil && ev.InputTokens != nil && ev.CachedInputTokens == nil {
			uncached = ev.InputTokens
		}
		if uncached != nil {
			total += usageTokenCost(*uncached, *price.InputPerMillion)
			charged = true
		}
	}
	if price.CachedInputPerMillion != nil && ev.CachedInputTokens != nil {
		total += usageTokenCost(*ev.CachedInputTokens, *price.CachedInputPerMillion)
		charged = true
	}
	if price.OutputPerMillion != nil && ev.OutputTokens != nil {
		total += usageTokenCost(*ev.OutputTokens, *price.OutputPerMillion)
		charged = true
	}
	if price.ReasoningPerMillion != nil && ev.ReasoningTokens != nil {
		total += usageTokenCost(*ev.ReasoningTokens, *price.ReasoningPerMillion)
		charged = true
	}
	if !charged && price.TotalPerMillion != nil && ev.TotalTokens != nil {
		total += usageTokenCost(*ev.TotalTokens, *price.TotalPerMillion)
		charged = true
	}
	return total, charged
}

func usageTokenCost(tokens int, perMillion float64) float64 {
	return float64(tokens) * perMillion / 1000000
}

func usageCostGroupKey(ev store.ProviderUsage, groupBy string, tasks map[string]store.Task) string {
	switch groupBy {
	case "task":
		return firstNonEmpty(ev.TaskID, "unassigned")
	case "epic":
		if task, ok := tasks[ev.TaskID]; ok {
			return firstNonEmpty(task.Definition.ParentID, task.Definition.ID, "unassigned")
		}
		return "unassigned"
	case "role":
		return firstNonEmpty(ev.Role, "unknown")
	case "day":
		if len(ev.CreatedAt) >= len("2006-01-02") {
			return ev.CreatedAt[:len("2006-01-02")]
		}
		return "unknown"
	case "kind":
		if task, ok := tasks[ev.TaskID]; ok {
			return firstNonEmpty(task.Definition.Kind, "unknown")
		}
		return "unknown"
	case "phase":
		return firstNonEmpty(ev.Phase, "unknown")
	case "model":
		return firstNonEmpty(ev.Model, "unknown")
	case "provider":
		fallthrough
	default:
		return firstNonEmpty(ev.Provider, "unknown")
	}
}

func combinedPriceStatus(current, next string) string {
	if current == "" {
		return next
	}
	if current == next {
		return current
	}
	return "partial_unknown"
}

func usageCacheReadRatio(inputTokens, cachedInputTokens *int) *float64 {
	if inputTokens == nil || cachedInputTokens == nil || *inputTokens == 0 {
		return nil
	}
	ratio := float64(*cachedInputTokens) / float64(*inputTokens)
	return &ratio
}

func addUsageCostInt(total **int, value *int) {
	if value == nil {
		return
	}
	if *total == nil {
		v := *value
		*total = &v
		return
	}
	**total += *value
}

func addUsageFloat(total **float64, value float64) {
	if *total == nil {
		v := value
		*total = &v
		return
	}
	**total += value
}

func printUsageCostHuman(report usageCostReport) {
	fmt.Printf("usage cost report by %s\n", report.GroupBy)
	if report.PricingConfigured == 0 {
		fmt.Println("pricing: no [[provider_model_prices]] entries configured")
	}
	for _, row := range report.Rows {
		fmt.Printf("- %s events=%d cost=%s forecast=%s total=%s cache_read=%s status=%s unknown_cost_events=%d\n", row.Group, row.Events, formatUsageCost(row.EstimatedCostUSD), formatUsageCost(row.ForecastCostUSD), formatUsageInt(row.TotalTokens), formatUsageRatio(row.CacheReadRatio), row.PriceStatus, row.UnknownCostEvents)
	}
	if len(report.Rows) == 0 {
		fmt.Println("- no usage recorded")
	}
	fmt.Printf("total events=%d cost=%s forecast=%s total_tokens=%s cache_read=%s status=%s unknown_cost_events=%d\n", report.Totals.Events, formatUsageCost(report.Totals.EstimatedCostUSD), formatUsageCost(report.Totals.ForecastCostUSD), formatUsageInt(report.Totals.TotalTokens), formatUsageRatio(report.Totals.CacheReadRatio), report.Totals.PriceStatus, report.Totals.UnknownCostEvents)
}

func printUsageCostMarkdown(report usageCostReport) {
	fmt.Printf("# Usage Cost Report\n\n")
	fmt.Printf("Grouped by `%s`. Costs are advisory planning estimates and are not task gates.\n\n", report.GroupBy)
	fmt.Println("| group | events | estimated cost | forecast | total tokens | cache read | price status | unknown cost events |")
	fmt.Println("|---|---:|---:|---:|---:|---:|---|---:|")
	for _, row := range report.Rows {
		fmt.Printf("| %s | %d | %s | %s | %s | %s | %s | %d |\n", row.Group, row.Events, formatUsageCost(row.EstimatedCostUSD), formatUsageCost(row.ForecastCostUSD), formatUsageInt(row.TotalTokens), formatUsageRatio(row.CacheReadRatio), row.PriceStatus, row.UnknownCostEvents)
	}
	fmt.Printf("| total | %d | %s | %s | %s | %s | %s | %d |\n", report.Totals.Events, formatUsageCost(report.Totals.EstimatedCostUSD), formatUsageCost(report.Totals.ForecastCostUSD), formatUsageInt(report.Totals.TotalTokens), formatUsageRatio(report.Totals.CacheReadRatio), report.Totals.PriceStatus, report.Totals.UnknownCostEvents)
}

func formatUsageCost(value *float64) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprintf("$%.6f", *value)
}

func formatUsageRatio(value *float64) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprintf("%.1f%%", *value*100)
}

func guardModeResult(mode string) string {
	switch mode {
	case "blocking":
		return "fail"
	case "warning":
		return "partial"
	default:
		return "partial"
	}
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

func recordCompletionHandback(ctx context.Context, opts globalOptions, args []string) error {
	if isHelpOnly(args) {
		subcommandUsage("record", "completion-handback <task-id> --to <role> --next-action <text> [--completion-state <state>] [--evidence <path>]... [--approval-boundary <text>] [--provider <name>] [--target <thread-or-adapter>] [--state <handoff_recorded|notification_delivered|thread_steered|notification_failed>] [--reason <text>]")
		return nil
	}
	if len(args) < 1 {
		return errors.New("record completion-handback requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("completion-handback", flag.ContinueOnError)
	to := fs.String("to", "", "next actor role or lane")
	nextAction := fs.String("next-action", "", "next safe action for the receiving actor")
	completionState := fs.String("completion-state", "", "completion outcome: "+strings.Join(completionhandback.CompletionStateList(), ", "))
	approvalBoundary := fs.String("approval-boundary", "", "approval, authority, or no-authority boundary for the handback")
	provider := fs.String("provider", "", "provider name for delivery proof")
	target := fs.String("target", "", "provider target such as thread id, tmux pane, or adapter destination")
	state := fs.String("state", "handoff_recorded", "notification state for the linked handback delivery")
	reason := fs.String("reason", "", "delivery/failure reason or handback signature detail")
	var evidence multiFlag
	fs.Var(&evidence, "evidence", "evidence path; may be repeated")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected completion-handback arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*to) == "" {
		return errors.New("--to is required")
	}
	payload, err := completionhandback.RenderPayloadWithState(*nextAction, *completionState, evidence, *approvalBoundary)
	if err != nil {
		return err
	}
	if !validCompletionHandbackState(*state) {
		return fmt.Errorf("invalid completion handback notification state %q", *state)
	}
	if (*state == "failed" || *state == "notification_failed") && strings.TrimSpace(*reason) == "" {
		return errors.New("failed completion handback notification requires --reason")
	}
	switch strings.TrimSpace(*state) {
	case "notification_delivered", "thread_steered":
		if strings.TrimSpace(*provider) == "" || strings.TrimSpace(*target) == "" {
			return errors.New("delivered completion handback states require --provider and --target")
		}
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		handoff, err := s.RecordHandoffWithID(ctx, taskID, store.Handoff{ToRole: *to, Payload: payload})
		if err != nil {
			return err
		}
		notification, err := s.RecordNotification(ctx, store.Notification{
			TaskID:    taskID,
			HandoffID: &handoff.ID,
			Domain:    *to,
			Provider:  *provider,
			Target:    *target,
			State:     *state,
			Reason:    *reason,
		})
		if err != nil {
			return err
		}
		row := completionhandback.Rows(taskID, []store.Handoff{handoff}, []store.Notification{notification})[0]
		if opts.JSON {
			return printJSON(row)
		}
		fmt.Printf("completion_handback recorded %s handoff_id=%d to=%s completion_state=%s delivery_status=%s delivery_state=%s actual_thread_delivery=%t next_action=%s\n",
			taskID,
			row.HandoffID,
			row.ToRole,
			firstNonEmpty(row.CompletionState, "unspecified"),
			row.DeliveryStatus,
			firstNonEmpty(row.DeliveryState, "none"),
			row.ActualThreadDelivery,
			row.NextAction,
		)
		return nil
	})
}

func recordCompletionHandbackSupersede(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("record", "completion-handback-supersede <task-id> --handoff-id <id> --reason <text> [--replacement-handoff-id <id>] [--evidence <path>]")
		return nil
	}
	taskID := args[0]
	fs := flag.NewFlagSet("completion-handback-supersede", flag.ContinueOnError)
	handoffID := fs.Int64("handoff-id", 0, "completion handback handoff id to mark superseded")
	replacementID := fs.Int64("replacement-handoff-id", 0, "replacement completion handback id, when one exists")
	reason := fs.String("reason", "", "why the older handback is obsolete")
	evidencePath := fs.String("evidence", "", "optional artifact proving the supersede decision")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected completion-handback-supersede arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *handoffID <= 0 {
		return errors.New("--handoff-id is required")
	}
	if strings.TrimSpace(*reason) == "" {
		return errors.New("--reason is required")
	}
	if *replacementID == *handoffID {
		return errors.New("--replacement-handoff-id must differ from --handoff-id")
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		task, _, evidence, handoffs, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		notifications, err := s.Notifications(ctx, taskID)
		if err != nil {
			return err
		}
		handback, ok := completionHandbackByID(taskID, handoffs, notifications, evidence, *handoffID, cfg)
		if !ok {
			return fmt.Errorf("completion handback handoff %d not found for %s", *handoffID, taskID)
		}
		if completionhandback.IsResolved(handback) && !handback.Stale && !handback.Superseded {
			return fmt.Errorf("completion handback %d is already resolved with delivery_status=%s", *handoffID, handback.DeliveryStatus)
		}
		if *replacementID > 0 {
			if _, ok := completionHandbackByID(taskID, handoffs, notifications, evidence, *replacementID, cfg); !ok {
				return fmt.Errorf("replacement completion handback %d not found for %s", *replacementID, taskID)
			}
		}
		terminal := terminalStatusSet(cfg.States.Terminal)
		if !terminal[task.Status] && task.Status != "blocked" && *replacementID <= 0 {
			return errors.New("non-terminal completion handback supersede requires --replacement-handoff-id or task status blocked")
		}
		commandText := fmt.Sprintf("completion-handback-supersede handoff_id=%d", *handoffID)
		if *replacementID > 0 {
			commandText += fmt.Sprintf(" replacement_handoff_id=%d", *replacementID)
		}
		if err := s.RecordEvidence(ctx, taskID, store.Evidence{
			CommandText:  commandText,
			Result:       "pass",
			ArtifactPath: *evidencePath,
			ArtifactType: "completion-handback-superseded",
			Notes:        *reason,
		}); err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(map[string]any{
				"task_id":                 taskID,
				"handoff_id":              *handoffID,
				"replacement_handoff_id":  *replacementID,
				"artifact_type":           "completion-handback-superseded",
				"reason":                  strings.TrimSpace(*reason),
				"evidence_path":           strings.TrimSpace(*evidencePath),
				"completion_handback_old": handback,
			})
		}
		fmt.Printf("completion_handback_superseded %s handoff_id=%d replacement_handoff_id=%d reason=%s\n", taskID, *handoffID, *replacementID, strings.TrimSpace(*reason))
		return nil
	})
}

func completionHandbackByID(taskID string, handoffs []store.Handoff, notifications []store.Notification, evidence []store.Evidence, handoffID int64, cfg config.Config) (completionhandback.Handback, bool) {
	ackTimeout, err := reviewWaitAckTimeout(cfg)
	if err != nil {
		ackTimeout = 0
	}
	for _, handback := range completionhandback.RowsWithOptions(taskID, handoffs, notifications, completionhandback.RowOptions{
		AckTimeout: ackTimeout,
		Superseded: completionhandback.SupersedesFromEvidence(evidence),
	}) {
		if handback.HandoffID == handoffID {
			return handback, true
		}
	}
	return completionhandback.Handback{}, false
}

func validNotificationState(state string) bool {
	switch strings.TrimSpace(state) {
	case "intent", "handoff_recorded", "sent", "notification_delivered", "thread_steered", "acknowledged", "review_acknowledged", "review_recorded", "failed", "notification_failed":
		return true
	default:
		return false
	}
}

func validCompletionHandbackState(state string) bool {
	switch strings.TrimSpace(state) {
	case "handoff_recorded", "notification_delivered", "thread_steered", "notification_failed":
		return true
	default:
		return false
	}
}

func recordNotification(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("record notification requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("notification", flag.ContinueOnError)
	handoffID := fs.String("handoff-id", "", "handoff id associated with this notification")
	domain := fs.String("domain", "", "review domain or target role")
	provider := fs.String("provider", "", "provider name")
	target := fs.String("target", "", "provider target such as thread id, tmux pane, or adapter destination")
	state := fs.String("state", "intent", "notification state: intent, handoff_recorded, sent, notification_delivered, thread_steered, acknowledged, review_acknowledged, review_recorded, failed, notification_failed")
	reason := fs.String("reason", "", "reason or failure detail")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected notification arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *domain == "" {
		return errors.New("--domain is required")
	}
	var handoffPtr *int64
	if strings.TrimSpace(*handoffID) != "" {
		parsed, err := strconv.ParseInt(strings.TrimSpace(*handoffID), 10, 64)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("invalid --handoff-id %q", *handoffID)
		}
		handoffPtr = &parsed
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		recorded, err := s.RecordNotification(ctx, store.Notification{
			TaskID:    taskID,
			HandoffID: handoffPtr,
			Domain:    *domain,
			Provider:  *provider,
			Target:    *target,
			State:     *state,
			Reason:    *reason,
		})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(recorded)
		}
		fmt.Printf("notification recorded %s domain=%s state=%s provider=%s target=%s\n", taskID, recorded.Domain, recorded.State, firstNonEmpty(recorded.Provider, "none"), firstNonEmpty(recorded.Target, "none"))
		return nil
	})
}

func recordReview(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 1 {
		return errors.New("record review requires task id")
	}
	taskID := args[0]
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	reviewer := fs.String("reviewer", "", "reviewer")
	domain := fs.String("domain", "", "review domain satisfied by this reviewer")
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
		return s.RecordReview(ctx, taskID, store.Review{Reviewer: *reviewer, Domain: *domain, Verdict: *verdict, Reason: *reason, Commit: *commit})
	})
}

func cmdDashboard(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) > 0 {
		if isHelpOnly(args) {
			subcommandUsage("dashboard", "[--listen <addr>] [--no-open] [--read-only] [--multi] | start|stop|restart|status")
			return nil
		}
		switch args[0] {
		case "start":
			return cmdDashboardLifecycle(ctx, opts, "start", args[1:])
		case "stop":
			return cmdDashboardLifecycle(ctx, opts, "stop", args[1:])
		case "restart":
			return cmdDashboardLifecycle(ctx, opts, "restart", args[1:])
		case "status":
			return cmdDashboardLifecycle(ctx, opts, "status", args[1:])
		}
	}
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	listen := fs.String("listen", "", "listen address")
	noOpen := fs.Bool("no-open", false, "do not open browser")
	readOnly := fs.Bool("read-only", false, "serve dashboard in shared read-only mode")
	multi := fs.Bool("multi", false, "show registered projects")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = noOpen
	if *multi {
		return cmdDashboardMulti(ctx, opts, *listen, *noOpen)
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		addr := cfg.Dashboard.Listen
		if *listen != "" {
			addr = *listen
		}
		if *readOnly {
			cfg.Dashboard.ReadOnly = true
		}
		url := dashboard.URL(addr)
		if !isLoopbackAddr(addr) {
			if cfg.Dashboard.ReadOnly {
				fmt.Fprintln(os.Stderr, "warning: dashboard is not bound to a loopback address; read-only mode is not authentication")
			} else {
				fmt.Fprintln(os.Stderr, "warning: dashboard is not bound to a loopback address; v0.1 has no authentication")
			}
		}
		fmt.Println("dashboard", url)
		if cfg.Dashboard.AutoOpen && !*noOpen {
			_ = openBrowser(url)
		}
		worktrees, err := collectWorktreeStatus(cfg, root)
		if err != nil {
			return err
		}
		return dashboard.NewWithRoot(s, cfg, roleNames(cfg), dashboardWorktrees(worktrees), root).ListenAndServe(addr)
	})
}

func cmdDashboardMulti(ctx context.Context, opts globalOptions, listen string, noOpen bool) error {
	cfg, _, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	addr := cfg.Dashboard.Listen
	if listen != "" {
		addr = listen
	}
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}
	projects := make([]dashboard.ProjectStore, 0, len(reg.Projects))
	for _, project := range reg.Projects {
		dbPath := registry.ResolveDBPath(project)
		projectStore, err := store.Open(ctx, dbPath, project.Name)
		if err != nil {
			projects = append(projects, dashboard.ProjectStore{Name: project.Name, Path: project.Path, DBPath: dbPath, ConfigPath: registry.ResolveConfigPath(project), Error: fmt.Sprintf("open project %s: %v", project.Name, err)})
			continue
		}
		projects = append(projects, dashboard.ProjectStore{Name: project.Name, Path: project.Path, DBPath: dbPath, ConfigPath: registry.ResolveConfigPath(project), Store: projectStore})
	}
	url := dashboard.URL(addr)
	if !isLoopbackAddr(addr) {
		fmt.Fprintln(os.Stderr, "warning: dashboard is not bound to a loopback address; v0.1 has no authentication")
	}
	fmt.Println("dashboard", url)
	if cfg.Dashboard.AutoOpen && !noOpen {
		_ = openBrowser(url)
	}
	return http.ListenAndServe(addr, dashboard.NewMulti(projects))
}

type dashboardLifecycleStatus struct {
	PIDFile          string `json:"pid_file"`
	LogFile          string `json:"log_file"`
	PID              int    `json:"pid,omitempty"`
	Running          bool   `json:"running"`
	State            string `json:"state"`
	URL              string `json:"url,omitempty"`
	Version          string `json:"version"`
	BinaryPath       string `json:"binary_path,omitempty"`
	ListenerDetected bool   `json:"listener_detected,omitempty"`
	Warning          string `json:"warning,omitempty"`
}

func cmdDashboardLifecycle(ctx context.Context, opts globalOptions, action string, args []string) error {
	fs := flag.NewFlagSet("dashboard "+action, flag.ContinueOnError)
	listen := fs.String("listen", "", "listen address")
	noOpen := fs.Bool("no-open", false, "do not open browser")
	open := fs.Bool("open", false, "open browser after starting")
	readOnly := fs.Bool("read-only", false, "serve dashboard in shared read-only mode")
	multi := fs.Bool("multi", false, "show registered projects")
	pidFile := fs.String("pid-file", "", "pid file")
	logFile := fs.String("log-file", "", "log file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected dashboard %s arguments: %s", action, strings.Join(fs.Args(), " "))
	}
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	childNoOpen := true
	if *open {
		childNoOpen = false
	}
	if *noOpen {
		childNoOpen = true
	}
	addr := cfg.Dashboard.Listen
	if *listen != "" {
		addr = *listen
	}
	resolvedPIDFile, resolvedLogFile := dashboardLifecycleFiles(root, *pidFile, *logFile, *multi)
	status, err := readDashboardLifecycleStatus(resolvedPIDFile, resolvedLogFile, addr)
	if err != nil {
		return err
	}
	switch action {
	case "status":
		return printDashboardLifecycleStatus(status, opts.JSON)
	case "stop":
		if err := stopDashboardLifecycle(status); err != nil {
			return err
		}
		status.Running = false
		status.PID = 0
		if opts.JSON {
			return printJSON(status)
		}
		fmt.Println("dashboard stopped")
		return nil
	case "restart":
		if status.Running {
			if err := stopDashboardLifecycle(status); err != nil {
				return err
			}
		}
		return startDashboardLifecycle(opts, addr, childNoOpen, *readOnly, *multi, resolvedPIDFile, resolvedLogFile, true)
	case "start":
		if status.Running {
			return printDashboardLifecycleStatus(status, opts.JSON)
		}
		return startDashboardLifecycle(opts, addr, childNoOpen, *readOnly, *multi, resolvedPIDFile, resolvedLogFile, false)
	default:
		return fmt.Errorf("unknown dashboard lifecycle action %q", action)
	}
}

func dashboardLifecycleFiles(root, pidFile, logFile string, multi bool) (string, string) {
	name := "dashboard"
	if multi {
		name = "dashboard-multi"
	}
	if pidFile == "" {
		pidFile = filepath.Join(root, ".fairway", name+".pid")
	}
	if logFile == "" {
		logFile = filepath.Join(root, ".fairway", name+".log")
	}
	return pidFile, logFile
}

func readDashboardLifecycleStatus(pidFile, logFile, addr string) (dashboardLifecycleStatus, error) {
	status := dashboardLifecycleStatus{PIDFile: pidFile, LogFile: logFile, State: "stopped", URL: dashboard.URL(addr), Version: version}
	if exe, err := os.Executable(); err == nil {
		status.BinaryPath = exe
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			status.markUnknownIfListenerDetected(addr)
			return status, nil
		}
		return status, err
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		status.markUnknownIfListenerDetected(addr)
		return status, nil
	}
	var pid int
	if _, err := fmt.Sscanf(raw, "%d", &pid); err != nil {
		return status, fmt.Errorf("read dashboard pid file %s: invalid pid %q", pidFile, raw)
	}
	status.PID = pid
	status.Running = processAlive(pid)
	if status.Running {
		status.State = "running"
	}
	if !status.Running {
		_ = os.Remove(pidFile)
		status.PID = 0
		status.markUnknownIfListenerDetected(addr)
	}
	return status, nil
}

func (status *dashboardLifecycleStatus) markUnknownIfListenerDetected(addr string) {
	if dashboardAddressListening(addr) {
		status.State = "unknown"
		status.ListenerDetected = true
		status.Warning = "requested address is listening but the configured pid file does not identify the process; pass the matching --pid-file for managed multi-instance dashboards"
	}
}

func dashboardAddressListening(addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		_ = ln.Close()
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "address already in use")
}

func printDashboardLifecycleStatus(status dashboardLifecycleStatus, asJSON bool) error {
	if asJSON {
		return printJSON(status)
	}
	state := status.State
	if state == "" {
		state = "stopped"
	}
	fmt.Printf("dashboard %s", state)
	if status.PID > 0 {
		fmt.Printf("\tpid=%d", status.PID)
	}
	if status.URL != "" {
		fmt.Printf("\t%s", status.URL)
	}
	if status.Version != "" {
		fmt.Printf("\tversion=%s", status.Version)
	}
	if status.BinaryPath != "" {
		fmt.Printf("\tbinary=%s", status.BinaryPath)
	}
	if status.ListenerDetected {
		fmt.Print("\tlistener=detected")
	}
	if status.Warning != "" {
		fmt.Printf("\twarning=%s", status.Warning)
	}
	fmt.Printf("\tpid_file=%s\tlog_file=%s\n", status.PIDFile, status.LogFile)
	return nil
}

func stopDashboardLifecycle(status dashboardLifecycleStatus) error {
	if !status.Running || status.PID <= 0 {
		_ = os.Remove(status.PIDFile)
		return nil
	}
	if err := syscall.Kill(status.PID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	for i := 0; i < 20; i++ {
		if !processAlive(status.PID) {
			_ = os.Remove(status.PIDFile)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := syscall.Kill(status.PID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	_ = os.Remove(status.PIDFile)
	return nil
}

func startDashboardLifecycle(opts globalOptions, addr string, noOpen bool, readOnly bool, multi bool, pidFile string, logFile string, restarted bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return err
	}
	log, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	childArgs := dashboardLifecycleChildArgs(opts, addr, noOpen, readOnly, multi)
	cmd := exec.Command(exe, childArgs...)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	verb := "started"
	if restarted {
		verb = "restarted"
	}
	fmt.Printf("dashboard %s\tpid=%d\t%s\tversion=%s\tbinary=%s\tpid_file=%s\tlog_file=%s\n", verb, cmd.Process.Pid, dashboard.URL(addr), version, exe, pidFile, logFile)
	return nil
}

func dashboardLifecycleChildArgs(opts globalOptions, addr string, noOpen bool, readOnly bool, multi bool) []string {
	var args []string
	if opts.ConfigPath != "" {
		args = append(args, "--config", opts.ConfigPath)
	}
	if opts.DBPath != "" {
		args = append(args, "--db", opts.DBPath)
	}
	if opts.Role != "" {
		args = append(args, "--as", opts.Role)
	}
	args = append(args, "dashboard", "--listen", addr)
	if noOpen {
		args = append(args, "--no-open")
	}
	if readOnly {
		args = append(args, "--read-only")
	}
	if multi {
		args = append(args, "--multi")
	}
	return args
}

func dashboardWorktrees(statuses []worktreeStatus) []dashboard.WorktreeStatus {
	out := make([]dashboard.WorktreeStatus, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, dashboard.WorktreeStatus{
			Role:       status.Role,
			Branch:     status.Branch,
			Path:       status.Path,
			Registered: status.Registered,
			Exists:     status.Exists,
			Dirty:      status.Dirty,
			LastCommit: status.LastCommit,
		})
	}
	return out
}

func cmdDB(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("db requires backup, export, migrate, compat, or rehearsal")
	}
	if isHelpOnly(args) {
		subcommandUsage("db", "backup|export|migrate|compat|rehearsal")
		return nil
	}
	switch args[0] {
	case "backup":
		return cmdDBBackup(ctx, opts, args[1:])
	case "export":
		return cmdDBExport(ctx, opts, args[1:])
	case "migrate":
		return cmdDBMigrate(ctx, opts, args[1:])
	case "compat":
		return cmdDBCompat(args[1:])
	case "rehearsal":
		return cmdDBRehearsal(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown db command %q", args[0])
	}
}

func cmdDBMigrate(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("db migrate", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show pending migrations without applying")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected db migrate arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	dbPath := resolveDBPath(root, cfg, opts)
	if *dryRun {
		pending, err := store.PendingMigrations(ctx, dbPath)
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(pending)
		}
		if len(pending) == 0 {
			fmt.Println("no pending migrations")
			return nil
		}
		for _, migration := range pending {
			fmt.Printf("%03d\t%s\n", migration.Version, migration.Name)
		}
		return nil
	}
	s, err := store.Open(ctx, dbPath, cfg.Fairway.ProjectName)
	if err != nil {
		return err
	}
	if err := s.SetTaskIDPattern(cfg.Fairway.TaskIDPattern); err != nil {
		_ = s.Close()
		return err
	}
	defer s.Close()
	fmt.Println("migrations applied")
	return nil
}

func cmdDBCompat(args []string) error {
	fs := flag.NewFlagSet("db compat", flag.ContinueOnError)
	backend := fs.String("backend", "", "backend")
	printDDL := fs.Bool("print-ddl", false, "print backend ddl")
	applyDDL := fs.Bool("apply-ddl", false, "apply backend ddl")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected db compat arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *backend != "postgres" {
		return errors.New("db compat currently supports --backend postgres")
	}
	if *applyDDL {
		return errors.New("db compat --apply-ddl is not implemented for postgres")
	}
	if *printDDL {
		ddl, err := store.PostgresCompatDDL()
		if err != nil {
			return err
		}
		fmt.Print(ddl)
		return nil
	}
	report, err := store.PostgresCompatReport()
	if err != nil {
		return err
	}
	if !report.OK {
		for _, finding := range report.Findings {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", finding.Migration, finding.Token, finding.Message)
		}
		return errors.New("postgres compatibility checks failed")
	}
	fmt.Printf("postgres compatibility checks passed (%d migrations)\n", len(report.Files))
	return nil
}

type dbRehearsalReport struct {
	Backend               string                `json:"backend"`
	OK                    bool                  `json:"ok"`
	ProjectID             string                `json:"project_id"`
	CreatedAt             string                `json:"created_at"`
	SourceDBPath          string                `json:"source_db_path"`
	OutputDir             string                `json:"output_dir"`
	BackupPath            string                `json:"backup_path"`
	SourceExportPath      string                `json:"source_export_path"`
	RehearsalExportPath   string                `json:"rehearsal_export_path"`
	CompatReportPath      string                `json:"compat_report_path"`
	CompatDDLPath         string                `json:"compat_ddl_path"`
	EquivalencePath       string                `json:"equivalence_path"`
	RollbackPath          string                `json:"rollback_path"`
	ManifestPath          string                `json:"manifest_path"`
	CompatOK              bool                  `json:"compat_ok"`
	EquivalenceOK         bool                  `json:"equivalence_ok"`
	PostgresProofOK       bool                  `json:"postgres_proof_ok,omitempty"`
	PostgresSchema        string                `json:"postgres_schema,omitempty"`
	PostgresApplyPath     string                `json:"postgres_apply_path,omitempty"`
	PostgresImportPath    string                `json:"postgres_import_path,omitempty"`
	PostgresReadbackPath  string                `json:"postgres_readback_path,omitempty"`
	PostgresProofError    string                `json:"postgres_proof_error,omitempty"`
	SourceCounts          dbRehearsalCounts     `json:"source_counts"`
	RehearsalCounts       dbRehearsalCounts     `json:"rehearsal_counts"`
	PostgresCounts        dbRehearsalCounts     `json:"postgres_counts,omitempty"`
	EquivalenceMismatches []dbRehearsalMismatch `json:"equivalence_mismatches,omitempty"`
	Artifacts             map[string]string     `json:"artifacts"`
	Boundaries            []string              `json:"boundaries"`
}

type dbRehearsalCounts struct {
	Tasks       int            `json:"tasks"`
	Transitions int            `json:"transitions"`
	Evidence    int            `json:"evidence"`
	Handoffs    int            `json:"handoffs"`
	Reviews     int            `json:"reviews"`
	Sessions    int            `json:"sessions"`
	ByStatus    map[string]int `json:"by_status"`
}

type dbRehearsalTaskSummary struct {
	Status      string `json:"status"`
	Transitions int    `json:"transitions"`
	Evidence    int    `json:"evidence"`
	Handoffs    int    `json:"handoffs"`
	Reviews     int    `json:"reviews"`
}

type dbRehearsalEquivalence struct {
	OK         bool                              `json:"ok"`
	Source     dbRehearsalCounts                 `json:"source"`
	Rehearsal  dbRehearsalCounts                 `json:"rehearsal"`
	Tasks      map[string]dbRehearsalTaskSummary `json:"tasks"`
	Mismatches []dbRehearsalMismatch             `json:"mismatches,omitempty"`
}

type dbRehearsalMismatch struct {
	Scope string `json:"scope"`
	Field string `json:"field"`
	Want  string `json:"want"`
	Got   string `json:"got"`
}

func cmdDBRehearsal(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("db rehearsal", flag.ContinueOnError)
	backend := fs.String("backend", "", "backend")
	outDir := fs.String("out", "", "output directory")
	applyDSNEnv := fs.String("apply-dsn-env", "", "environment variable containing a disposable Postgres DSN for apply/import/readback proof")
	postgresSchema := fs.String("postgres-schema", "fairway_rehearsal", "disposable Postgres schema for apply/import/readback proof")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected db rehearsal arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *backend != "postgres" {
		return errors.New("db rehearsal currently supports --backend postgres")
	}
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	dbPath := resolveDBPath(root, cfg, opts)
	if *outDir == "" {
		*outDir = filepath.Join(root, ".fairway", "rehearsals", "postgres-"+time.Now().UTC().Format("20060102T150405Z"))
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	s, err := store.Open(ctx, dbPath, cfg.Fairway.ProjectName)
	if err != nil {
		return err
	}
	if err := s.SetTaskIDPattern(cfg.Fairway.TaskIDPattern); err != nil {
		_ = s.Close()
		return err
	}
	defer s.Close()

	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	backupPath := filepath.Join(*outDir, "sqlite-backup.db")
	sourceExportPath := filepath.Join(*outDir, "source-export.json")
	rehearsalExportPath := filepath.Join(*outDir, "rehearsal-export.json")
	compatReportPath := filepath.Join(*outDir, "postgres-compat-report.json")
	compatDDLPath := filepath.Join(*outDir, "postgres-compat-ddl.sql")
	equivalencePath := filepath.Join(*outDir, "readmodel-equivalence.json")
	rollbackPath := filepath.Join(*outDir, "rollback.md")
	manifestPath := filepath.Join(*outDir, "manifest.json")

	sourceSnapshot, err := s.Snapshot(ctx)
	if err != nil {
		return err
	}
	sourceSessions, err := s.Sessions(ctx, true)
	if err != nil {
		return err
	}
	if err := writeJSONFile(sourceExportPath, sourceSnapshot); err != nil {
		return err
	}
	if err := s.Backup(ctx, backupPath); err != nil {
		return err
	}

	rehearsalStore, err := store.Open(ctx, backupPath, cfg.Fairway.ProjectName)
	if err != nil {
		return err
	}
	if err := rehearsalStore.SetTaskIDPattern(cfg.Fairway.TaskIDPattern); err != nil {
		_ = rehearsalStore.Close()
		return err
	}
	rehearsalSnapshot, err := rehearsalStore.Snapshot(ctx)
	if closeErr := rehearsalStore.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	rehearsalStore, err = store.Open(ctx, backupPath, cfg.Fairway.ProjectName)
	if err != nil {
		return err
	}
	rehearsalSessions, err := rehearsalStore.Sessions(ctx, true)
	if closeErr := rehearsalStore.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := writeJSONFile(rehearsalExportPath, rehearsalSnapshot); err != nil {
		return err
	}

	compatReport, err := store.PostgresCompatReport()
	if err != nil {
		return err
	}
	if err := writeJSONFile(compatReportPath, compatReport); err != nil {
		return err
	}
	ddl, err := store.PostgresCompatDDL()
	if err != nil {
		return err
	}
	if err := os.WriteFile(compatDDLPath, []byte(ddl), 0o644); err != nil {
		return err
	}

	equivalence := compareRehearsalSnapshots(sourceSnapshot, rehearsalSnapshot, sourceSessions, rehearsalSessions)
	if err := writeJSONFile(equivalencePath, equivalence); err != nil {
		return err
	}
	rollback := renderDBRehearsalRollback(cfg.Fairway.ProjectName, dbPath, backupPath, createdAt)
	if err := os.WriteFile(rollbackPath, []byte(rollback), 0o644); err != nil {
		return err
	}

	report := dbRehearsalReport{
		Backend:               "postgres",
		OK:                    compatReport.OK && equivalence.OK,
		ProjectID:             cfg.Fairway.ProjectName,
		CreatedAt:             createdAt,
		SourceDBPath:          dbPath,
		OutputDir:             *outDir,
		BackupPath:            backupPath,
		SourceExportPath:      sourceExportPath,
		RehearsalExportPath:   rehearsalExportPath,
		CompatReportPath:      compatReportPath,
		CompatDDLPath:         compatDDLPath,
		EquivalencePath:       equivalencePath,
		RollbackPath:          rollbackPath,
		ManifestPath:          manifestPath,
		CompatOK:              compatReport.OK,
		EquivalenceOK:         equivalence.OK,
		SourceCounts:          equivalence.Source,
		RehearsalCounts:       equivalence.Rehearsal,
		EquivalenceMismatches: equivalence.Mismatches,
		Artifacts: map[string]string{
			"sqlite_backup":      backupPath,
			"source_export":      sourceExportPath,
			"rehearsal_export":   rehearsalExportPath,
			"postgres_compat":    compatReportPath,
			"postgres_ddl":       compatDDLPath,
			"equivalence_report": equivalencePath,
			"rollback":           rollbackPath,
		},
		Boundaries: []string{
			"disposable rehearsal only",
			"no production store switch",
			"no shared dashboard restart",
			"no public exposure",
			"no release tag or publish",
		},
	}
	if strings.TrimSpace(*applyDSNEnv) != "" {
		report.Boundaries = append(report.Boundaries, "postgres DDL applied only to the disposable schema named by --postgres-schema")
		proof, err := runPostgresRehearsalProof(ctx, *applyDSNEnv, *postgresSchema, ddl, sourceSnapshot, sourceSessions, *outDir)
		if err != nil {
			report.OK = false
			report.PostgresProofOK = false
			report.PostgresProofError = err.Error()
		} else {
			report.PostgresProofOK = proof.OK
			report.PostgresSchema = proof.Schema
			report.PostgresApplyPath = proof.ApplyPath
			report.PostgresImportPath = proof.ImportPath
			report.PostgresReadbackPath = proof.ReadbackPath
			report.PostgresCounts = proof.Counts
			report.Artifacts["postgres_apply"] = proof.ApplyPath
			report.Artifacts["postgres_import"] = proof.ImportPath
			report.Artifacts["postgres_readback"] = proof.ReadbackPath
			if !proof.OK {
				report.OK = false
				report.EquivalenceMismatches = append(report.EquivalenceMismatches, proof.Mismatches...)
			}
		}
	} else {
		report.Boundaries = append(report.Boundaries, "postgres DDL is review output, not applied by this command")
	}
	if err := writeJSONFile(manifestPath, report); err != nil {
		return err
	}
	if opts.JSON {
		return printJSON(report)
	}
	fmt.Printf("db_rehearsal: ok=%t backend=postgres project=%s out=%s\n", report.OK, report.ProjectID, report.OutputDir)
	fmt.Printf("compat_ok=%t equivalence_ok=%t tasks=%d evidence=%d reviews=%d\n", report.CompatOK, report.EquivalenceOK, report.SourceCounts.Tasks, report.SourceCounts.Evidence, report.SourceCounts.Reviews)
	if strings.TrimSpace(*applyDSNEnv) != "" {
		fmt.Printf("postgres_proof_ok=%t schema=%s sessions=%d\n", report.PostgresProofOK, report.PostgresSchema, report.PostgresCounts.Sessions)
	}
	fmt.Printf("manifest=%s\nbackup=%s\nrollback=%s\n", report.ManifestPath, report.BackupPath, report.RollbackPath)
	if !report.OK {
		return errors.New("db rehearsal failed")
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

type dbPostgresProof struct {
	OK           bool
	Schema       string
	ApplyPath    string
	ImportPath   string
	ReadbackPath string
	Counts       dbRehearsalCounts
	Mismatches   []dbRehearsalMismatch
}

func runPostgresRehearsalProof(ctx context.Context, dsnEnv, schema, ddl string, snapshot store.Snapshot, sessions []store.Session, outDir string) (dbPostgresProof, error) {
	dsnEnv = strings.TrimSpace(dsnEnv)
	if dsnEnv == "" {
		return dbPostgresProof{}, errors.New("postgres rehearsal proof requires --apply-dsn-env")
	}
	dsn := strings.TrimSpace(os.Getenv(dsnEnv))
	if dsn == "" {
		return dbPostgresProof{}, fmt.Errorf("postgres rehearsal proof DSN environment variable %s is empty", dsnEnv)
	}
	if !validPostgresSchemaName(schema) {
		return dbPostgresProof{}, fmt.Errorf("invalid postgres schema %q", schema)
	}
	if _, err := exec.LookPath("psql"); err != nil {
		return dbPostgresProof{}, fmt.Errorf("psql is required for postgres rehearsal proof: %w", err)
	}
	applyPath := filepath.Join(outDir, "postgres-apply.sql")
	importPath := filepath.Join(outDir, "postgres-import.sql")
	readbackPath := filepath.Join(outDir, "postgres-readback.json")
	importSQL := renderPostgresImportSQL(snapshot, sessions)
	if err := os.WriteFile(importPath, []byte(importSQL), 0o644); err != nil {
		return dbPostgresProof{}, err
	}
	applySQL := strings.Join([]string{
		"-- Fairway disposable Postgres rehearsal proof.",
		"-- Applies only to the validated isolated schema in the DSN named by --apply-dsn-env.",
		fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE;", schema),
		fmt.Sprintf("CREATE SCHEMA %s;", schema),
		fmt.Sprintf("SET search_path TO %s;", schema),
		postgresApplyDDL(ddl),
		importSQL,
	}, "\n\n")
	if err := os.WriteFile(applyPath, []byte(applySQL), 0o644); err != nil {
		return dbPostgresProof{}, err
	}
	if err := runPSQL(ctx, dsn, "-v", "ON_ERROR_STOP=1", "-f", applyPath); err != nil {
		return dbPostgresProof{}, err
	}
	counts, err := postgresReadbackCounts(ctx, dsn, schema)
	if err != nil {
		return dbPostgresProof{}, err
	}
	if err := writeJSONFile(readbackPath, counts); err != nil {
		return dbPostgresProof{}, err
	}
	sourceCounts := rehearsalCounts(snapshot)
	sourceCounts.Sessions = len(sessions)
	proof := dbPostgresProof{
		OK:           true,
		Schema:       schema,
		ApplyPath:    applyPath,
		ImportPath:   importPath,
		ReadbackPath: readbackPath,
		Counts:       counts,
	}
	compareInt := func(field string, want, got int) {
		if want != got {
			proof.OK = false
			proof.Mismatches = append(proof.Mismatches, dbRehearsalMismatch{Scope: "postgres", Field: field, Want: strconv.Itoa(want), Got: strconv.Itoa(got)})
		}
	}
	compareInt("tasks", sourceCounts.Tasks, counts.Tasks)
	compareInt("transitions", sourceCounts.Transitions, counts.Transitions)
	compareInt("evidence", sourceCounts.Evidence, counts.Evidence)
	compareInt("handoffs", sourceCounts.Handoffs, counts.Handoffs)
	compareInt("reviews", sourceCounts.Reviews, counts.Reviews)
	compareInt("sessions", sourceCounts.Sessions, counts.Sessions)
	for status, want := range sourceCounts.ByStatus {
		compareInt("status:"+status, want, counts.ByStatus[status])
	}
	return proof, nil
}

func validPostgresSchemaName(schema string) bool {
	if schema == "" {
		return false
	}
	lower := strings.ToLower(schema)
	if !strings.HasPrefix(lower, "fairway_") {
		return false
	}
	switch lower {
	case "public", "pg_catalog", "information_schema", "pg_toast":
		return false
	}
	if strings.HasPrefix(lower, "pg_") {
		return false
	}
	for i, r := range schema {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			continue
		case i > 0 && r >= '0' && r <= '9':
			continue
		default:
			return false
		}
	}
	return true
}

func postgresApplyDDL(ddl string) string {
	return strings.ReplaceAll(ddl, "id INTEGER PRIMARY KEY", "id BIGSERIAL PRIMARY KEY")
}

func runPSQL(ctx context.Context, dsn string, args ...string) error {
	cmd, err := postgresPSQLCommand(ctx, dsn, args...)
	if err != nil {
		return err
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		out := strings.TrimSpace(string(output))
		if out == "" {
			return fmt.Errorf("psql failed: %w", err)
		}
		return fmt.Errorf("psql failed: %w: %s", err, out)
	}
	return nil
}

func postgresPSQLCommand(ctx context.Context, dsn string, args ...string) (*exec.Cmd, error) {
	env, err := postgresPSQLEnv(os.Environ(), dsn)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "psql", args...)
	cmd.Env = env
	return cmd, nil
}

func postgresPSQLEnv(base []string, dsn string) ([]string, error) {
	conn, err := parsePostgresURLDSN(dsn)
	if err != nil {
		return nil, err
	}
	env := make([]string, 0, len(base)+2)
	for _, item := range base {
		if strings.HasPrefix(item, "PGDATABASE=") ||
			strings.HasPrefix(item, "PGCONNECT_TIMEOUT=") ||
			strings.HasPrefix(item, "PGHOST=") ||
			strings.HasPrefix(item, "PGPORT=") ||
			strings.HasPrefix(item, "PGUSER=") ||
			strings.HasPrefix(item, "PGPASSWORD=") ||
			strings.HasPrefix(item, "PGSSLMODE=") {
			continue
		}
		env = append(env, item)
	}
	env = append(env,
		"PGCONNECT_TIMEOUT=10",
		"PGHOST="+conn.Host,
		"PGPORT="+conn.Port,
		"PGUSER="+conn.User,
		"PGDATABASE="+conn.Database,
	)
	if conn.Password != "" {
		env = append(env, "PGPASSWORD="+conn.Password)
	}
	if conn.SSLMode != "" {
		env = append(env, "PGSSLMODE="+conn.SSLMode)
	}
	return env, nil
}

type postgresURLDSN struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

func parsePostgresURLDSN(dsn string) (postgresURLDSN, error) {
	parsed, err := url.Parse(strings.TrimSpace(dsn))
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return postgresURLDSN{}, errors.New("postgres rehearsal proof DSN must be a postgres:// or postgresql:// URL")
	}
	if parsed.Hostname() == "" {
		return postgresURLDSN{}, errors.New("postgres rehearsal proof DSN is missing host")
	}
	database := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if database == "" {
		return postgresURLDSN{}, errors.New("postgres rehearsal proof DSN is missing database")
	}
	dbName, err := url.PathUnescape(database)
	if err != nil || dbName == "" {
		return postgresURLDSN{}, errors.New("postgres rehearsal proof DSN database is invalid")
	}
	user := ""
	password := ""
	if parsed.User != nil {
		user = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	if user == "" {
		return postgresURLDSN{}, errors.New("postgres rehearsal proof DSN is missing user")
	}
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	return postgresURLDSN{
		Host:     parsed.Hostname(),
		Port:     port,
		User:     user,
		Password: password,
		Database: dbName,
		SSLMode:  parsed.Query().Get("sslmode"),
	}, nil
}

func postgresReadbackCounts(ctx context.Context, dsn, schema string) (dbRehearsalCounts, error) {
	query := fmt.Sprintf(`SET search_path TO %s;
SELECT 'tasks', count(*) FROM task_state
UNION ALL SELECT 'transitions', count(*) FROM task_state_history
UNION ALL SELECT 'evidence', count(*) FROM task_evidence
UNION ALL SELECT 'handoffs', count(*) FROM task_handoffs
UNION ALL SELECT 'reviews', count(*) FROM task_reviews
UNION ALL SELECT 'sessions', count(*) FROM agent_sessions
UNION ALL SELECT 'status:' || status, count(*) FROM task_state GROUP BY status
ORDER BY 1;`, schema)
	cmd, err := postgresPSQLCommand(ctx, dsn, "-At", "-F", "\t", "-v", "ON_ERROR_STOP=1", "-c", query)
	if err != nil {
		return dbRehearsalCounts{}, err
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		out := strings.TrimSpace(string(output))
		if out == "" {
			return dbRehearsalCounts{}, fmt.Errorf("postgres readback failed: %w", err)
		}
		return dbRehearsalCounts{}, fmt.Errorf("postgres readback failed: %w: %s", err, out)
	}
	counts := dbRehearsalCounts{ByStatus: map[string]int{}}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "SET" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return dbRehearsalCounts{}, fmt.Errorf("unexpected postgres readback row %q", line)
		}
		value, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return dbRehearsalCounts{}, fmt.Errorf("unexpected postgres readback count %q: %w", parts[1], err)
		}
		key := strings.TrimSpace(parts[0])
		switch {
		case key == "tasks":
			counts.Tasks = value
		case key == "transitions":
			counts.Transitions = value
		case key == "evidence":
			counts.Evidence = value
		case key == "handoffs":
			counts.Handoffs = value
		case key == "reviews":
			counts.Reviews = value
		case key == "sessions":
			counts.Sessions = value
		case strings.HasPrefix(key, "status:"):
			counts.ByStatus[strings.TrimPrefix(key, "status:")] = value
		}
	}
	return counts, nil
}

func renderPostgresImportSQL(snapshot store.Snapshot, sessions []store.Session) string {
	taskIDs := map[string]bool{}
	for _, row := range snapshot.Tasks {
		taskIDs[row.Task.Definition.ID] = true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "-- Fairway rehearsal import for project %s exported at %s.\n", sqlQuote(snapshot.ProjectID), sqlQuote(snapshot.ExportedAt))
	for _, row := range snapshot.Tasks {
		def := row.Task.Definition
		task := row.Task
		fmt.Fprintf(&b, "INSERT INTO task_definitions (project_id, id, parent_id, kind, title, role, notes, acceptance_checks, dependencies, priority, sequence, profile, owning_domain, owning_layer, source_paths, target_paths, review_domains, tags, risk_level, migration_type, created_at, created_by, updated_at) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s);\n",
			sqlQuote(snapshot.ProjectID),
			sqlQuote(def.ID),
			sqlNullString(parentIDForImport(def.ParentID, taskIDs)),
			sqlQuote(def.Kind),
			sqlQuote(def.Title),
			sqlQuote(def.Role),
			sqlNullString(def.Notes),
			sqlJSON(def.AcceptanceChecks),
			sqlJSON(def.Dependencies),
			sqlIntPtr(def.Priority),
			sqlIntPtr(def.Sequence),
			sqlNullString(def.Profile),
			sqlNullString(def.OwningDomain),
			sqlNullString(def.OwningLayer),
			sqlJSON(def.SourcePaths),
			sqlJSON(def.TargetPaths),
			sqlJSON(def.ReviewDomains),
			sqlJSON(def.Tags),
			sqlNullString(def.RiskLevel),
			sqlNullString(def.MigrationType),
			sqlQuote(firstNonEmpty(task.UpdatedAt, snapshot.ExportedAt)),
			sqlQuote("db-rehearsal"),
			sqlQuote(firstNonEmpty(task.UpdatedAt, snapshot.ExportedAt)),
		)
		reviewRequired := "0"
		if task.ReviewStatus != "" || len(row.Reviews) > 0 || len(def.ReviewDomains) > 0 {
			reviewRequired = "1"
		}
		fmt.Fprintf(&b, "INSERT INTO task_state (project_id, task_id, status, owner, claimant, branch, claimed_at, completed_at, commit_sha, review_required, review_status, reviewer, reviewed_at, review_note, updated_at) VALUES (%s, %s, %s, %s, %s, %s, NULL, %s, %s, %s, %s, NULL, NULL, NULL, %s);\n",
			sqlQuote(snapshot.ProjectID),
			sqlQuote(def.ID),
			sqlQuote(task.Status),
			sqlNullString(task.Owner),
			sqlNullString(task.Claimant),
			sqlNullString(task.Branch),
			sqlNullString(task.CompletedAt),
			sqlNullString(task.CommitSHA),
			reviewRequired,
			sqlNullString(task.ReviewStatus),
			sqlQuote(firstNonEmpty(task.UpdatedAt, snapshot.ExportedAt)),
		)
		for _, transition := range row.Transitions {
			fmt.Fprintf(&b, "INSERT INTO task_state_history (project_id, task_id, from_status, to_status, actor, reason, at) VALUES (%s, %s, %s, %s, %s, %s, %s);\n",
				sqlQuote(snapshot.ProjectID),
				sqlQuote(def.ID),
				sqlNullString(transition.FromStatus),
				sqlQuote(transition.ToStatus),
				sqlQuote(transition.Actor),
				sqlNullString(transition.Reason),
				sqlQuote(transition.At),
			)
		}
		for _, handoff := range row.Handoffs {
			fmt.Fprintf(&b, "INSERT INTO task_handoffs (project_id, task_id, from_role, to_role, payload, acknowledged_at, created_at) VALUES (%s, %s, %s, %s, %s, %s, %s);\n",
				sqlQuote(snapshot.ProjectID),
				sqlQuote(def.ID),
				sqlQuote(firstNonEmpty(handoff.FromRole, "unknown")),
				sqlQuote(handoff.ToRole),
				sqlNullString(handoff.Payload),
				sqlNullString(handoff.AcknowledgedAt),
				sqlQuote(handoff.CreatedAt),
			)
		}
		for _, evidence := range row.Evidence {
			fmt.Fprintf(&b, "INSERT INTO task_evidence (project_id, task_id, command_text, result, artifact_path, artifact_type, duration_seconds, notes, created_at) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s);\n",
				sqlQuote(snapshot.ProjectID),
				sqlQuote(def.ID),
				sqlNullString(evidence.CommandText),
				sqlNullString(evidence.Result),
				sqlNullString(evidence.ArtifactPath),
				sqlNullString(evidence.ArtifactType),
				sqlIntPtr(evidence.DurationSeconds),
				sqlNullString(evidence.Notes),
				sqlQuote(evidence.CreatedAt),
			)
		}
		for _, review := range row.Reviews {
			fmt.Fprintf(&b, "INSERT INTO task_reviews (project_id, task_id, reviewer, review_domain, verdict, reviewed_commit_sha, route_reason, notes, created_at) VALUES (%s, %s, %s, %s, %s, %s, NULL, %s, %s);\n",
				sqlQuote(snapshot.ProjectID),
				sqlQuote(def.ID),
				sqlQuote(review.Reviewer),
				sqlNullString(review.Domain),
				sqlQuote(review.Verdict),
				sqlNullString(review.Commit),
				sqlNullString(review.Reason),
				sqlQuote(review.CreatedAt),
			)
		}
	}
	for _, session := range sessions {
		taskID := session.TaskID
		if !taskIDs[taskID] {
			taskID = ""
		}
		fmt.Fprintf(&b, "INSERT INTO agent_sessions (project_id, id, role, lane, worktree_path, branch, session_backend, provider, session_name, task_id, pid, tmux_pane, transcript_path, monitor_kind, automation_id, external_run_id, poll_command, manual_until, status, started_at, last_heartbeat_at, ended_at, exit_code, end_reason) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s);\n",
			sqlQuote(snapshot.ProjectID),
			sqlQuote(session.ID),
			sqlQuote(session.Role),
			sqlNullString(session.Lane),
			sqlNullString(session.WorktreePath),
			sqlNullString(session.Branch),
			sqlNullString(session.SessionBackend),
			sqlNullString(session.Provider),
			sqlNullString(session.SessionName),
			sqlNullString(taskID),
			sqlIntPtr(session.PID),
			sqlNullString(session.TmuxPane),
			sqlNullString(session.TranscriptPath),
			sqlNullString(session.MonitorKind),
			sqlNullString(session.AutomationID),
			sqlNullString(session.ExternalRunID),
			sqlNullString(session.PollCommand),
			sqlNullString(session.ManualUntil),
			sqlQuote(session.Status),
			sqlQuote(session.StartedAt),
			sqlNullString(session.LastHeartbeatAt),
			sqlNullString(session.EndedAt),
			sqlIntPtr(session.ExitCode),
			sqlNullString(session.EndReason),
		)
	}
	return b.String()
}

func parentIDForImport(parentID string, taskIDs map[string]bool) string {
	if parentID == "" || !taskIDs[parentID] {
		return ""
	}
	return parentID
}

func sqlJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "NULL"
	}
	if string(data) == "null" {
		return "NULL"
	}
	return sqlQuote(string(data))
}

func sqlIntPtr(value *int) string {
	if value == nil {
		return "NULL"
	}
	return strconv.Itoa(*value)
}

func sqlNullString(value string) string {
	if value == "" {
		return "NULL"
	}
	return sqlQuote(value)
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func compareRehearsalSnapshots(source, rehearsal store.Snapshot, sourceSessions, rehearsalSessions []store.Session) dbRehearsalEquivalence {
	sourceCounts := rehearsalCounts(source)
	sourceCounts.Sessions = len(sourceSessions)
	rehearsalCounts := rehearsalCounts(rehearsal)
	rehearsalCounts.Sessions = len(rehearsalSessions)
	eq := dbRehearsalEquivalence{
		OK:        true,
		Source:    sourceCounts,
		Rehearsal: rehearsalCounts,
		Tasks:     map[string]dbRehearsalTaskSummary{},
	}
	compareInt := func(scope, field string, want, got int) {
		if want != got {
			eq.OK = false
			eq.Mismatches = append(eq.Mismatches, dbRehearsalMismatch{Scope: scope, Field: field, Want: strconv.Itoa(want), Got: strconv.Itoa(got)})
		}
	}
	compareInt("summary", "tasks", sourceCounts.Tasks, rehearsalCounts.Tasks)
	compareInt("summary", "transitions", sourceCounts.Transitions, rehearsalCounts.Transitions)
	compareInt("summary", "evidence", sourceCounts.Evidence, rehearsalCounts.Evidence)
	compareInt("summary", "handoffs", sourceCounts.Handoffs, rehearsalCounts.Handoffs)
	compareInt("summary", "reviews", sourceCounts.Reviews, rehearsalCounts.Reviews)
	compareInt("summary", "sessions", sourceCounts.Sessions, rehearsalCounts.Sessions)
	for status, want := range sourceCounts.ByStatus {
		compareInt("status", status, want, rehearsalCounts.ByStatus[status])
	}
	sourceTasks := rehearsalTaskSummaries(source)
	rehearsalTasks := rehearsalTaskSummaries(rehearsal)
	for taskID, want := range sourceTasks {
		got, ok := rehearsalTasks[taskID]
		if !ok {
			eq.OK = false
			eq.Mismatches = append(eq.Mismatches, dbRehearsalMismatch{Scope: "task", Field: taskID, Want: "present", Got: "missing"})
			continue
		}
		eq.Tasks[taskID] = got
		if want != got {
			eq.OK = false
			wantJSON, _ := json.Marshal(want)
			gotJSON, _ := json.Marshal(got)
			eq.Mismatches = append(eq.Mismatches, dbRehearsalMismatch{Scope: "task", Field: taskID, Want: string(wantJSON), Got: string(gotJSON)})
		}
	}
	for taskID := range rehearsalTasks {
		if _, ok := sourceTasks[taskID]; !ok {
			eq.OK = false
			eq.Mismatches = append(eq.Mismatches, dbRehearsalMismatch{Scope: "task", Field: taskID, Want: "missing", Got: "present"})
		}
	}
	return eq
}

func rehearsalCounts(snapshot store.Snapshot) dbRehearsalCounts {
	counts := dbRehearsalCounts{Tasks: len(snapshot.Tasks), ByStatus: map[string]int{}}
	for _, task := range snapshot.Tasks {
		counts.Transitions += len(task.Transitions)
		counts.Evidence += len(task.Evidence)
		counts.Handoffs += len(task.Handoffs)
		counts.Reviews += len(task.Reviews)
		counts.ByStatus[task.Task.Status]++
	}
	return counts
}

func rehearsalTaskSummaries(snapshot store.Snapshot) map[string]dbRehearsalTaskSummary {
	out := map[string]dbRehearsalTaskSummary{}
	for _, task := range snapshot.Tasks {
		out[task.Task.Definition.ID] = dbRehearsalTaskSummary{
			Status:      task.Task.Status,
			Transitions: len(task.Transitions),
			Evidence:    len(task.Evidence),
			Handoffs:    len(task.Handoffs),
			Reviews:     len(task.Reviews),
		}
	}
	return out
}

func renderDBRehearsalRollback(project, sourceDB, backupPath, createdAt string) string {
	return fmt.Sprintf(`# Fairway DB Rehearsal Rollback

- project: %s
- created_at: %s
- source_db: %s
- backup: %s

This rehearsal did not switch the Fairway runtime store and did not apply
Postgres DDL. The SQLite backup is rollback input only.

If a future reviewed cutover fails after using this backup as input:

1. Stop shared writers or place Fairway into read-only/drain mode.
2. Preserve the failed target store for investigation.
3. Restore the SQLite backup to the configured Fairway DB path only after an
   operator verifies no divergent writes need manual reconciliation.
4. Run:

   fairway db migrate --dry-run
   fairway ready
   fairway reconcile active --dry-run

5. Record rollback evidence and any conflict/reconciliation packet in Fairway.

This file is a rehearsal artifact, not production rollback authorization.
`, project, createdAt, sourceDB, backupPath)
}

func cmdDBBackup(ctx context.Context, opts globalOptions, args []string) error {
	path := ""
	if len(args) > 0 {
		path = args[0]
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, root string, s *store.Store) error {
		if path == "" {
			path = defaultBackupPath(root)
		}
		if err := s.Backup(ctx, path); err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	})
}

func cmdDBExport(ctx context.Context, opts globalOptions, args []string) error {
	path := ""
	if len(args) > 0 {
		path = args[0]
	}
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if path == "" || path == "-" {
			_, err = os.Stdout.Write(data)
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	})
}

func defaultBackupPath(root string) string {
	name := "state-" + time.Now().UTC().Format("20060102T150405Z") + ".db"
	return filepath.Join(root, ".fairway", name)
}

func withStore(ctx context.Context, opts globalOptions, fn func(context.Context, config.Config, string, *store.Store) error) error {
	cfg, root, _, err := loadConfig(opts)
	if err != nil {
		return err
	}
	dbPath := resolveDBPath(root, cfg, opts)
	s, err := store.Open(ctx, dbPath, cfg.Fairway.ProjectName)
	if err != nil {
		return err
	}
	if err := s.SetTaskIDPattern(cfg.Fairway.TaskIDPattern); err != nil {
		_ = s.Close()
		return err
	}
	defer s.Close()
	return fn(ctx, cfg, root, s)
}

func resolveDBPath(root string, cfg config.Config, opts globalOptions) string {
	if opts.DBPath != "" {
		return opts.DBPath
	}
	return config.DBPath(cfg, root)
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
	task, _, _, handoffs, _, err := s.TaskDetail(ctx, taskID)
	if err != nil {
		return err
	}
	notifications, err := s.Notifications(ctx, taskID)
	if err != nil {
		return err
	}
	for _, handback := range completionhandback.Rows(taskID, handoffs, notifications) {
		if completionhandback.IsResolved(handback) {
			continue
		}
		if sameActor(handback.ToRole, task.Definition.Role, task.Owner) {
			continue
		}
		return fmt.Errorf("terminal transition requires completion handback delivery or failure proof for handoff %d to %s", handback.HandoffID, handback.ToRole)
	}
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

func sameActor(actor string, current ...string) bool {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return false
	}
	for _, value := range current {
		if actor == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}
	return ip.IsLoopback()
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func printTasks(tasks []store.Task) {
	for _, task := range tasks {
		fmt.Printf("%s\t%s\t%s\t%s\n", task.Definition.ID, task.Definition.Role, task.Status, task.Definition.Title)
	}
}

func printReadyExplanation(explanation coord.ReadinessExplanation) {
	fmt.Printf("no ready tasks; non-ready todo tasks: %d\n", explanation.NonReadyTodoCount)
	if len(explanation.Blockers) == 0 {
		fmt.Println("blockers: none classified")
		fmt.Println("next: fairway list --status todo")
		return
	}
	fmt.Println("blockers:")
	for _, blocker := range explanation.Blockers {
		fmt.Printf("- %s: count=%d tasks=%s", blocker.Category, blocker.Count, strings.Join(blocker.TaskIDs, ","))
		if len(blocker.BlockerIDs) > 0 {
			fmt.Printf(" blocker_tasks=%s", strings.Join(blocker.BlockerIDs, ","))
		}
		if blocker.Suggested != "" {
			fmt.Printf(" next=%q", blocker.Suggested)
		}
		fmt.Println()
	}
}

func filterReadyExplanationTasks(tasks []store.Task, role string) []store.Task {
	filtered := make([]store.Task, 0, len(tasks))
	for _, task := range tasks {
		if role != "" && task.Definition.Role != role {
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered
}

func taskListRows(tasks []store.Task, terminal []string) []taskListRow {
	statuses := map[string]string{}
	if len(terminal) == 0 {
		terminal = []string{"done"}
	}
	terminalSet := stringSet(terminal)
	for _, task := range tasks {
		statuses[task.Definition.ID] = task.Status
	}
	rows := make([]taskListRow, 0, len(tasks))
	for _, task := range tasks {
		blocked, missing := dependencyGaps(task.Definition.Dependencies, statuses, terminalSet)
		summary := "deps=none"
		if len(task.Definition.Dependencies) > 0 {
			summary = fmt.Sprintf("deps=%d blocked=%d missing=%d", len(task.Definition.Dependencies), len(blocked), len(missing))
		}
		rows = append(rows, taskListRow{
			ID:                  task.Definition.ID,
			Title:               task.Definition.Title,
			Role:                task.Definition.Role,
			Kind:                firstNonEmpty(task.Definition.Kind, "task"),
			Status:              task.Status,
			Owner:               task.Owner,
			Claimant:            task.Claimant,
			Ready:               task.Status == "todo" && len(blocked) == 0 && len(missing) == 0,
			Dependencies:        append([]string(nil), task.Definition.Dependencies...),
			DependencySummary:   summary,
			BlockedDependencies: blocked,
			MissingDependencies: missing,
		})
	}
	return rows
}

func dependencyGaps(deps []string, statuses map[string]string, terminal map[string]bool) ([]string, []string) {
	var blocked []string
	var missing []string
	for _, dep := range deps {
		status, ok := statuses[dep]
		if !ok {
			missing = append(missing, dep)
			continue
		}
		if !terminal[status] {
			blocked = append(blocked, dep+":"+status)
		}
	}
	return blocked, missing
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func filterByPriority(tasks []store.Task, maxPriority int) []store.Task {
	filtered := tasks[:0]
	for _, task := range tasks {
		if task.Definition.Priority != nil && *task.Definition.Priority <= maxPriority {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func filterByAncestor(tasks, all []store.Task, ancestor string) []store.Task {
	parent := map[string]string{}
	for _, task := range all {
		parent[task.Definition.ID] = task.Definition.ParentID
	}
	filtered := tasks[:0]
	for _, task := range tasks {
		for cursor := parent[task.Definition.ID]; cursor != ""; cursor = parent[cursor] {
			if cursor == ancestor {
				filtered = append(filtered, task)
				break
			}
		}
	}
	return filtered
}

func buildTree(rootID string, tasks []store.Task, maxDepth int) (treeNode, bool) {
	byID := map[string]store.Task{}
	children := map[string][]store.Task{}
	for _, task := range tasks {
		byID[task.Definition.ID] = task
		children[task.Definition.ParentID] = append(children[task.Definition.ParentID], task)
	}
	for parentID := range children {
		sort.SliceStable(children[parentID], func(i, j int) bool {
			left := children[parentID][i].Definition
			right := children[parentID][j].Definition
			leftPriority, rightPriority := 9999, 9999
			if left.Priority != nil {
				leftPriority = *left.Priority
			}
			if right.Priority != nil {
				rightPriority = *right.Priority
			}
			if leftPriority != rightPriority {
				return leftPriority < rightPriority
			}
			leftSequence, rightSequence := 9999, 9999
			if left.Sequence != nil {
				leftSequence = *left.Sequence
			}
			if right.Sequence != nil {
				rightSequence = *right.Sequence
			}
			if leftSequence != rightSequence {
				return leftSequence < rightSequence
			}
			return left.ID < right.ID
		})
	}
	root, ok := byID[rootID]
	if !ok {
		return treeNode{}, false
	}
	var walk func(store.Task, int) treeNode
	walk = func(task store.Task, depth int) treeNode {
		node := treeNode{Task: task}
		if maxDepth >= 0 && depth >= maxDepth {
			return node
		}
		for _, child := range children[task.Definition.ID] {
			node.Children = append(node.Children, walk(child, depth+1))
		}
		return node
	}
	return walk(root, 0), true
}

func printTree(node treeNode, depth int) {
	prefix := strings.Repeat("  ", depth)
	fmt.Printf("%s%s [%s] %s\n", prefix, node.Task.Definition.ID, node.Task.Status, node.Task.Definition.Title)
	for _, child := range node.Children {
		printTree(child, depth+1)
	}
}

func printDetail(ctx context.Context, cfg config.Config, s *store.Store, taskID string, asJSON bool) error {
	task, transitions, evidence, handoffs, reviews, err := s.TaskDetail(ctx, taskID)
	if err != nil {
		return err
	}
	sessions, err := sessionsForTask(ctx, s, taskID)
	if err != nil {
		return err
	}
	usageEvents, err := s.ProviderUsageForTask(ctx, taskID)
	if err != nil {
		return err
	}
	usageRollups, err := s.UsageRollups(ctx, store.UsageRollupOptions{GroupBy: "provider", TaskID: taskID})
	if err != nil {
		return err
	}
	batches, err := batchesForTask(ctx, s, taskID)
	if err != nil {
		return err
	}
	notifications, err := s.Notifications(ctx, taskID)
	if err != nil {
		return err
	}
	ackTimeout, err := reviewWaitAckTimeout(cfg)
	if err != nil {
		return err
	}
	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return err
	}
	liveWindowPhase := ""
	for _, status := range livewindow.StatusesFromCheckpoints(checkpoints) {
		if status.TaskID == taskID {
			liveWindowPhase = status.Phase
			break
		}
	}
	completionHandbacks := completionhandback.RowsWithOptions(taskID, handoffs, notifications, completionhandback.RowOptions{
		AckTimeout:      ackTimeout,
		TaskStatus:      task.Status,
		LiveWindowPhase: liveWindowPhase,
		Superseded:      completionhandback.SupersedesFromEvidence(evidence),
	})
	reviewPolicy, err := reviewPolicyEvaluation(ctx, cfg, s, task, reviews, nil)
	if err != nil {
		return err
	}
	uxMediaEvidence := evidencemodel.UXMediaRows(evidence)
	uxMediaSummary := evidencemodel.UXMediaSummaryFor(uxMediaEvidence)
	if asJSON {
		missingReviewDomains := reviewPolicy.MissingReviewDomains
		reviewStatus := effectiveReviewStatus(task.ReviewStatus, missingReviewDomains)
		reviewHandback, hasReviewHandback := coord.ReviewHandbackForTask(cfg, task, evidence, handoffs, reviews, coord.ReviewHandbackOptions{IncludeHistorical: true, Notifications: notifications})
		reviewNotifications := reviewstate.StatusesForTask(task, handoffs, reviews, notifications)
		return printJSON(struct {
			Task                 store.Task                             `json:"task"`
			ReviewStatus         string                                 `json:"review_status"`
			Transitions          []store.Transition                     `json:"transitions"`
			Evidence             []store.Evidence                       `json:"evidence"`
			UXMediaEvidence      []evidencemodel.UXMediaEvidence        `json:"ux_media_evidence,omitempty"`
			UXMediaSummary       evidencemodel.UXMediaSummary           `json:"ux_media_summary"`
			Handoffs             []store.Handoff                        `json:"handoffs"`
			CompletionHandbacks  []completionhandback.Handback          `json:"completion_handbacks,omitempty"`
			Reviews              []store.Review                         `json:"reviews"`
			ReviewPolicy         reviewpolicy.Evaluation                `json:"review_policy,omitempty"`
			MissingReviewDomains []string                               `json:"missing_review_domains,omitempty"`
			ReviewHandback       *coord.ReviewCompletionHandback        `json:"review_handback,omitempty"`
			ReviewNotifications  []reviewstate.ReviewNotificationStatus `json:"review_notifications,omitempty"`
			Sessions             []store.Session                        `json:"sessions"`
			Usage                []store.ProviderUsage                  `json:"usage"`
			UsageRollups         []store.UsageRollup                    `json:"usage_rollups"`
			Batches              []store.WorkBatch                      `json:"batches"`
			Notifications        []store.Notification                   `json:"notifications"`
		}{task, reviewStatus, transitions, evidence, uxMediaEvidence, uxMediaSummary, handoffs, completionHandbacks, reviews, reviewPolicy, missingReviewDomains, optionalReviewHandback(reviewHandback, hasReviewHandback), reviewNotifications, sessions, usageEvents, usageRollups, batches, notifications})
	}
	missingReviewDomains := reviewPolicy.MissingReviewDomains
	reviewStatus := effectiveReviewStatus(task.ReviewStatus, missingReviewDomains)
	reviewHandback, hasReviewHandback := coord.ReviewHandbackForTask(cfg, task, evidence, handoffs, reviews, coord.ReviewHandbackOptions{IncludeHistorical: true, Notifications: notifications})
	reviewNotifications := reviewstate.StatusesForTask(task, handoffs, reviews, notifications)
	fmt.Printf("%s %s\nstatus: %s\nrole: %s\nowner: %s\nreview: %s\n\n%s\n", task.Definition.ID, task.Definition.Title, task.Status, task.Definition.Role, task.Owner, reviewStatus, task.Definition.Notes)
	printTaskMetadata(task.Definition)
	if len(reviewPolicy.Requirements) > 0 {
		fmt.Println()
		printReviewPolicyEvaluation(reviewPolicy)
	}
	fmt.Println("\ndependencies:")
	for _, dep := range task.Definition.Dependencies {
		fmt.Printf("- %s\n", dep)
	}
	fmt.Println("\nacceptance:")
	for _, check := range task.Definition.AcceptanceChecks {
		fmt.Printf("- %s\n", check)
	}
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
		notes := ""
		if strings.TrimSpace(ev.Notes) != "" {
			notes = " " + strings.TrimSpace(ev.Notes)
		}
		fmt.Printf("- %s %s %s%s\n", ev.Result, ev.CommandText, ev.ArtifactPath, notes)
	}
	fmt.Println("\nux media evidence:")
	if len(uxMediaEvidence) == 0 {
		fmt.Println("- none")
	} else {
		fmt.Printf("- summary screenshots=%d videos=%d browser_traces=%d uat=%d exercised=%t\n", uxMediaSummary.Screenshots, uxMediaSummary.Videos, uxMediaSummary.BrowserTraces, uxMediaSummary.UAT, uxMediaSummary.Exercised)
		for _, row := range uxMediaEvidence {
			fmt.Printf("- kind=%s artifact_type=%s result=%s artifact=%s redaction_required=%t boundary=%s\n",
				row.Kind,
				row.ArtifactType,
				firstNonEmpty(row.Result, "unknown"),
				firstNonEmpty(row.ArtifactPath, "none"),
				row.RedactionRequired,
				row.Boundary,
			)
		}
	}
	fmt.Println("\nhandoffs:")
	for _, h := range handoffs {
		fmt.Printf("- #%d to %s: %s\n", h.ID, h.ToRole, h.Payload)
	}
	if len(completionHandbacks) > 0 {
		fmt.Println("\ncompletion handbacks:")
		for _, handback := range completionHandbacks {
			fmt.Printf("- handoff_id=%d to=%s completion_state=%s delivery_status=%s delivery_state=%s task_status=%s live_window_phase=%s stale=%t stale_age=%s actual_thread_delivery=%t provider=%s target=%s suggested_action=%s next_action=%s evidence=%s approval_boundary=%s\n",
				handback.HandoffID,
				handback.ToRole,
				firstNonEmpty(handback.CompletionState, "unspecified"),
				handback.DeliveryStatus,
				firstNonEmpty(handback.DeliveryState, "none"),
				firstNonEmpty(handback.TaskStatus, "unknown"),
				firstNonEmpty(handback.LiveWindowPhase, "none"),
				handback.Stale,
				firstNonEmpty(handback.StaleAge, "none"),
				handback.ActualThreadDelivery,
				firstNonEmpty(handback.Provider, "none"),
				firstNonEmpty(handback.Target, "none"),
				firstNonEmpty(handback.SuggestedAction, "none"),
				handback.NextAction,
				firstNonEmpty(strings.Join(handback.EvidencePaths, ","), "none"),
				firstNonEmpty(handback.ApprovalBoundary, "none"),
			)
			if handback.Superseded {
				fmt.Printf("  superseded=true reason=%s replacement_handoff_id=%d evidence=%s at=%s\n",
					firstNonEmpty(handback.SupersededReason, "none"),
					handback.ReplacementHandoffID,
					firstNonEmpty(handback.SupersededEvidence, "none"),
					firstNonEmpty(handback.SupersededAt, "none"),
				)
			}
		}
	}
	fmt.Println("\nnotifications:")
	if len(notifications) == 0 {
		fmt.Println("- none")
	}
	for _, n := range notifications {
		fmt.Printf("- %s domain=%s provider=%s target=%s reason=%s\n", n.State, n.Domain, firstNonEmpty(n.Provider, "none"), firstNonEmpty(n.Target, "none"), firstNonEmpty(n.Reason, "none"))
	}
	if len(reviewNotifications) > 0 {
		fmt.Println("\nreview notification status:")
		for _, status := range reviewNotifications {
			fmt.Printf("- domain=%s status=%s blocking=%t handoff_id=%d last_notification_state=%s provider=%s target=%s action=%s\n",
				status.Domain,
				status.Status,
				status.Blocking,
				status.HandoffID,
				firstNonEmpty(status.LastState, "none"),
				firstNonEmpty(status.Provider, "none"),
				firstNonEmpty(status.Target, "none"),
				status.SuggestedAction,
			)
		}
	}
	fmt.Println("\nreviews:")
	for _, r := range reviews {
		domain := ""
		if strings.TrimSpace(r.Domain) != "" {
			domain = " for " + strings.TrimSpace(r.Domain)
		}
		fmt.Printf("- %s by %s%s: %s\n", r.Verdict, r.Reviewer, domain, r.Reason)
	}
	if len(missingReviewDomains) > 0 {
		fmt.Println("\nmissing review domains:")
		for _, domain := range missingReviewDomains {
			fmt.Printf("- %s\n", domain)
		}
	}
	if hasReviewHandback {
		fmt.Println("\nreview handback:")
		fmt.Printf("- merge_ready_status: %s\n", reviewHandback.MergeReadyStatus)
		fmt.Printf("- commit: %s\n", firstNonEmpty(reviewHandback.Commit, "unknown"))
		fmt.Printf("- review_signature: %s\n", firstNonEmpty(reviewHandback.ReviewSignature, "unknown"))
		fmt.Printf("- suggested_command: %s\n", reviewHandback.SuggestedCommand)
		fmt.Printf("- recommended_action: %s\n", reviewHandback.RecommendedAction)
		fmt.Printf("- required_domains: %s\n", strings.Join(reviewHandback.RequiredDomains, ", "))
		fmt.Printf("- approved_domains: %s\n", strings.Join(reviewHandback.ApprovedDomains, ", "))
		fmt.Println("- missing_domains: none")
		if reviewHandback.NotificationState != "" {
			fmt.Printf("- notification_state: %s\n", reviewHandback.NotificationState)
		}
		for _, verdict := range reviewHandback.LatestVerdicts {
			fmt.Printf("- latest %s: %s by %s\n", verdict.Domain, firstNonEmpty(verdict.Verdict, "none"), firstNonEmpty(verdict.Reviewer, "unknown"))
		}
		for _, blocker := range reviewHandback.Blockers {
			fmt.Printf("- blocker: %s\n", blocker)
		}
	}
	fmt.Println("\nsessions:")
	for _, session := range sessions {
		fmt.Printf("- %s %s/%s %s pane=%s transcript=%s\n", session.ID, session.SessionBackend, session.Provider, session.Status, session.TmuxPane, session.TranscriptPath)
	}
	fmt.Println("\nusage:")
	if len(usageEvents) == 0 {
		fmt.Println("- unknown")
	}
	for _, ev := range usageEvents {
		fmt.Printf("- %s %s/%s total=%s input=%s cached=%s output=%s phase=%s session=%s\n", ev.Provider, ev.Source, ev.Confidence, formatUsageInt(ev.TotalTokens), formatUsageInt(ev.InputTokens), formatUsageInt(ev.CachedInputTokens), formatUsageInt(ev.OutputTokens), firstNonEmpty(ev.Phase, "unknown"), firstNonEmpty(ev.SessionID, ev.ExternalSessionID, "unknown"))
	}
	fmt.Println("\nbatches:")
	if len(batches) == 0 {
		fmt.Println("- none")
	}
	for _, batch := range batches {
		fmt.Printf("- %s %s branch=%s pipeline=%s\n", batch.ID, batch.Title, firstNonEmpty(batch.Branch, "none"), firstNonEmpty(batch.PipelineID, "none"))
	}
	return nil
}

func formatUsageInt(value *int) string {
	if value == nil {
		return "unknown"
	}
	return strconv.Itoa(*value)
}

func optionalReviewHandback(handback coord.ReviewCompletionHandback, ok bool) *coord.ReviewCompletionHandback {
	if !ok {
		return nil
	}
	return &handback
}

func effectiveReviewStatus(stored string, missingReviewDomains []string) string {
	if stored == "approved" && len(missingReviewDomains) > 0 {
		return "partial_approval"
	}
	return stored
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sessionsForTask(ctx context.Context, s *store.Store, taskID string) ([]store.Session, error) {
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		return nil, err
	}
	var out []store.Session
	for _, session := range sessions {
		if session.TaskID == taskID {
			out = append(out, session)
		}
	}
	return out, nil
}

func batchesForTask(ctx context.Context, s *store.Store, taskID string) ([]store.WorkBatch, error) {
	batches, err := s.WorkBatches(ctx)
	if err != nil {
		return nil, err
	}
	var out []store.WorkBatch
	for _, batch := range batches {
		for _, member := range batch.Tasks {
			if member == taskID {
				out = append(out, batch)
				break
			}
		}
	}
	return out, nil
}

func printTaskMetadata(task store.TaskDefinition) {
	if task.Profile == "" && task.OwningDomain == "" && task.OwningLayer == "" && len(task.SourcePaths) == 0 && len(task.TargetPaths) == 0 && len(task.ReviewDomains) == 0 && len(task.Tags) == 0 && task.RiskLevel == "" && task.MigrationType == "" {
		return
	}
	fmt.Println("\nmetadata:")
	if task.Profile != "" {
		fmt.Printf("- profile: %s\n", task.Profile)
	}
	if task.OwningDomain != "" {
		fmt.Printf("- owning_domain: %s\n", task.OwningDomain)
	}
	if task.OwningLayer != "" {
		fmt.Printf("- owning_layer: %s\n", task.OwningLayer)
	}
	if len(task.SourcePaths) > 0 {
		fmt.Printf("- source_paths: %s\n", strings.Join(task.SourcePaths, ", "))
	}
	if len(task.TargetPaths) > 0 {
		fmt.Printf("- target_paths: %s\n", strings.Join(task.TargetPaths, ", "))
	}
	if len(task.ReviewDomains) > 0 {
		fmt.Printf("- review_domains: %s\n", strings.Join(task.ReviewDomains, ", "))
	}
	if len(task.Tags) > 0 {
		fmt.Printf("- tags: %s\n", strings.Join(task.Tags, ", "))
	}
	if task.RiskLevel != "" {
		fmt.Printf("- risk_level: %s\n", task.RiskLevel)
	}
	if task.MigrationType != "" {
		fmt.Printf("- migration_type: %s\n", task.MigrationType)
	}
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
	fmt.Println("fairway - Governed Agentic Engineering coordination")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  fairway <command> [args]")
	fmt.Println("  fairway <command> --help")
	fmt.Println()
	fmt.Println("Queue and task state:")
	fmt.Println("  init, agent-guide, import, add, spawn, update, tree, list, ready, claim, set-status, task-detail")
	fmt.Println("Evidence and review:")
	fmt.Println("  record evidence|guard-report|handoff|completion-handback|completion-handback-supersede|notification|review|usage|push-intent, route review, merge-ready, review checkout, review-waits, review-policy, live-window")
	fmt.Println("Sessions, worktrees, and workflow:")
	fmt.Println("  session, worktree, reconcile active, workflow check|closeout, checkpoint, memory, wait, batch")
	fmt.Println("Coordinator and readiness:")
	fmt.Println("  coordinator plan|tick|status|preflight, readiness report, adoption artifact, parity artifact")
	fmt.Println("Rules, packets, reports, and audits:")
	fmt.Println("  rules, packet, recipe extract|render|list, regression-pack, provenance report|prompt-packet, advisory validate, notify notifiers|dry-run|send, automation candidates, rough-edge add|list, usage report|cost-report, delivery report, audit work-coverage|ci-learning|failure-routing|notifications|docs-backlog, status-report, health-report, timing-report, completion-handback-report")
	fmt.Println("Dashboard, release, and configuration:")
	fmt.Println("  dashboard, server, release verify, tracker, register, unregister, projects, db, config validate, tui, version")
	fmt.Println()
	fmt.Println("Run `fairway agent-guide` for the offline agent operating guide.")
	fmt.Println("Run `fairway <command> --help` for concise command usage.")
}

func printCommandHelp(command string) bool {
	help := map[string]string{
		"init":                       "fairway init [--refresh-agent-contract]\n  Scaffold .fairway/config.toml, the SQLite DB, and .fairway/AGENTS.md.",
		"agent-guide":                "fairway agent-guide [--path | --output <path>]\n  Print the embedded offline agent guide, show its source/version path, or write it to a file.",
		"import":                     "fairway import <yaml-or-json-path> [--state-once]\n  Import task definitions from YAML or JSON.",
		"add":                        "fairway add <task-id> --title <title> [--role <role>] [metadata flags]\n  Add one task definition.",
		"spawn":                      "fairway spawn --id <task-id> --title <title> [--child|--sibling|--root] [metadata flags]\n  Spawn related work from an existing task context.",
		"update":                     "fairway update <task-id> [--title <title>] [--dependencies <ids>] [metadata flags]\n  Update task definition metadata.",
		"list":                       "fairway list [--status <state[,state]>]... [--role <role>] [--ready]\n  List tasks with dependency visibility.",
		"ready":                      "fairway ready [--in <epic-id>] [--priority <n>]\n  Show claimable work for the current role and explain empty queues.",
		"claim":                      "fairway claim <task-id> | fairway claim --in <epic-id>\n  Claim a ready task.",
		"set-status":                 "fairway set-status <task-id> <state> [--reason <text>] [--commit <sha>] [--reopen]\n  Move a task through the configured state machine.",
		"task-detail":                "fairway task-detail <task-id>\n  Show task state, metadata, evidence, sessions, reviews, and readiness.",
		"status-report":              "fairway status-report\n  Print task counts by status.",
		"health-report":              "fairway health-report\n  Print aggregate health diagnostics for the local Fairway state.",
		"timing-report":              "fairway timing-report\n  Print task timing and flow metrics.",
		"completion-handback-report": "fairway completion-handback-report [--include-closed] [--format human|markdown]\n  Print completion handback idle-time metrics by task and workstream.",
		"dispatch-plan":              "fairway dispatch-plan [--role <role>] [--limit <n>]\n  Print a ready-work dispatch plan for a role.",
		"git-check":                  "fairway git-check [--base <ref>]\n  Check git/worktree readiness against a base ref.",
		"preflight":                  "fairway preflight [--role <role>] [--base <ref>]\n  Run local readiness checks before claim or closeout.",
		"record":                     "fairway record evidence|guard-report|handoff|completion-handback|completion-handback-supersede|notification|review|usage|push-intent ...\n  Record execution facts without editing the DB directly.",
		"review-waits":               "fairway review-waits list|wake [--task <task-id>]\n  List derived review-wait rows or emit bounded fixed-template wake prompts for parked provider threads.",
		"review-policy":              "fairway review-policy report [--profile <name>]\n  Report review profile overhead against recorded outcome signals.",
		"live-window":                "fairway live-window record <task-id> --phase <phase> [control fields] | fairway live-window status [--task <task-id>] | fairway live-window control-room [--stale] | fairway live-window retry-budget record|status ...\n  Record and inspect repeated live-operation handshake phases and retry budgets via task checkpoints.",
		"memory":                     "fairway memory show|update|append|packet|stale ...\n  Store curated track memory and render compact provider-independent packets.",
		"wait":                       "fairway wait add|ack|list|tick|wake [--task <task-id>] [--stale] [--kind <kind>]\n  Record durable parked-work waits, acknowledge them, project wait state, and emit bounded wake prompts.",
		"session":                    "fairway session upsert|status|end|reconcile|launch ...\n  Register provider attachments and reconcile session state.",
		"worktree":                   "fairway worktree setup|status|prune\n  Manage configured role worktrees.",
		"workflow":                   "fairway workflow check|closeout ...\n  Check task, closeout, and deploy workflow boundaries.",
		"coordinator":                "fairway coordinator plan|tick|status|preflight\n  Print dry-run coordinator recommendations and stop conditions.",
		"rules":                      "fairway rules validate <dir>|evidence-types|match <task-id>\n  Validate rule packs and inspect rule/evidence applicability.",
		"packet":                     "fairway packet context|bugfix|retry|watcher|release-run|template|rules|architecture-map|boundary-guard|vertical-slice ...\n  Render bounded task, retry, release, rule, and profile packets; packets do not authorize execution.",
		"provenance":                 "fairway provenance report [--task <task-id>|--since <duration>] [--format text|markdown|json] | fairway provenance prompt-packet --task <task-id> [--format markdown|json] | fairway provenance manifest --path <file>...\n  Export metadata-only supply-chain provenance, bounded task prompt packets, and content-free hash manifests.",
		"recipe":                     "fairway recipe extract|render|list ...\n  Extract completed tasks into reusable privacy-bounded recipe packets and render them for new tasks.",
		"advisory":                   "fairway advisory adapters|validate <task-id> ...\n  List configured advisory provider adapters or validate advisory recommendations as evidence only.",
		"notify":                     "fairway notify notifiers|dry-run|send ...\n  Inspect optional external notifier config, render dry-run notification intent, or deliver through an explicitly configured notifier.",
		"automation":                 "fairway automation candidates --since <duration> [--threshold <n>] [--format text|json]\n  Read-only repeated-work automation candidate report.",
		"delivery":                   "fairway delivery report --since <duration> [--profile <name>] [--format text|json]\n  Read-only delivery velocity and process overhead report.",
		"rough-edge":                 "fairway rough-edge add --task <task-id> --owner <role> --severity <level> --decision <fix-now|defer> --summary <text> | fairway rough-edge list [--task <task-id>] [--owner <role>] [--expired]\n  Record and inspect owner rough edges found while using the product; list is read-only.",
		"dashboard":                  "fairway dashboard [--listen <addr>] [--multi] [--no-open] [--read-only]\n  Run the local dashboard; use start|stop|restart|status for lifecycle mode.",
		"server":                     "fairway server --read-only [--listen <addr>] | fairway server --mode api-write-pilot --write\n  Run the shared-team API skeleton. The write pilot is append-only evidence/checkpoints only.",
		"release":                    "fairway release verify --version <vX.Y.Z> --tag <vX.Y.Z> [--provenance-bundle <path>] ...\n  Verify release evidence, provenance bundle reference, and publication state.",
		"config":                     "fairway config validate\n  Validate .fairway/config.toml.",
		"db":                         "fairway db backup|export|migrate|compat|rehearsal ...\n  Manage the local Fairway database.",
		"audit":                      "fairway audit work-coverage|ci-learning|failure-routing|notifications|docs-backlog ...\n  Run advisory coverage, CI/deploy learning, known-failure routing, provider notification lifecycle, and docs-to-backlog reports.",
		"usage":                      "fairway usage report|cost-report [--by <provider|task|epic|role|day|kind|phase|model>]\n  Report provider-neutral usage attribution and advisory cost forecasts.",
		"register":                   "fairway register [--name <name>]\n  Add the current project to the local Fairway registry.",
		"unregister":                 "fairway unregister [<name>]\n  Remove a project from the local Fairway registry.",
		"projects":                   "fairway projects\n  List registered Fairway projects.",
		"tui":                        "fairway tui [--once]\n  Open the interactive terminal workflow.",
		"version":                    "fairway version\n  Print the Fairway binary version.",
	}
	text, ok := help[command]
	if !ok {
		return false
	}
	fmt.Println(text)
	return true
}

func isHelpOnly(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func subcommandUsage(command, subcommands string) {
	fmt.Printf("fairway %s %s\n", command, subcommands)
}
