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

	"github.com/subashram/fairway/internal/audit"
	"github.com/subashram/fairway/internal/config"
	coord "github.com/subashram/fairway/internal/coordinator"
	"github.com/subashram/fairway/internal/dashboard"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/importer"
	"github.com/subashram/fairway/internal/reconcile"
	"github.com/subashram/fairway/internal/registry"
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
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			return fmt.Errorf("unexpected help arguments: %s", strings.Join(args[1:], " "))
		}
		usage()
		return nil
	case "init":
		return cmdInit(ctx, opts)
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
		return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
			return printDetail(ctx, s, args[1], opts.JSON)
		})
	case "status-report":
		return cmdStatusReport(ctx, opts, args[1:])
	case "health-report":
		return cmdHealthReport(ctx, opts, args[1:])
	case "timing-report":
		return cmdTimingReport(ctx, opts, args[1:])
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
	case "merge-ready":
		return cmdMergeReady(ctx, opts, args[1:])
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
	case "packet":
		return cmdPacket(ctx, opts, args[1:])
	case "watcher":
		return cmdWatcher(ctx, opts, args[1:])
	case "release":
		return cmdRelease(ctx, opts, args[1:])
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
	case "dashboard":
		return cmdDashboard(ctx, opts, args[1:])
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
	OK              bool                     `json:"ok"`
	Mode            string                   `json:"mode"`
	Git             fairwaygit.Status        `json:"git"`
	Reconcile       reconcile.ActiveReport   `json:"reconcile"`
	Closeout        reconcile.CloseoutReport `json:"closeout,omitempty"`
	Issues          []string                 `json:"issues"`
	Warnings        []string                 `json:"warnings,omitempty"`
	Recommendations []string                 `json:"recommendations,omitempty"`
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
		reconcileReport, err := reconcile.Active(ctx, s, reconcile.ActiveOptions{
			Terminal:             cfg.States.Terminal,
			StaleCheckpointAfter: *staleAfter,
		})
		if err != nil {
			return err
		}
		report := workflowCheckReport{
			OK:        true,
			Mode:      *mode,
			Git:       gitStatus,
			Reconcile: reconcileReport,
		}
		var closeoutReport reconcile.CloseoutReport
		if *mode == "close" {
			if *taskID == "" {
				report.Issues = append(report.Issues, "close mode requires --task-id")
			} else {
				task, _, _, _, _, err := s.TaskDetail(ctx, *taskID)
				if err != nil {
					return err
				}
				closeoutGit := buildCloseoutGit(root, *base, task, gitStatus)
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
			}
		}
		if gitStatus.Dirty {
			message := "worktree has uncommitted changes"
			if *requireClean {
				report.Issues = append(report.Issues, message)
			} else {
				report.Warnings = append(report.Warnings, message)
			}
			if changedDocs(gitStatus.ChangedFiles) {
				report.Recommendations = append(report.Recommendations, "commit completed documentation updates as their own coherent slice")
			}
			if changedCode(gitStatus.ChangedFiles) {
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
			if gitStatus.Dirty {
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
		task, _, _, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			return err
		}
		gitStatus, err := fairwaygit.Check(root, *base)
		if err != nil {
			return err
		}
		closeoutGit := buildCloseoutGit(root, *base, task, gitStatus)
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

func buildCloseoutGit(root, base string, task store.Task, gitStatus fairwaygit.Status) reconcile.CloseoutGit {
	branch := firstNonEmpty(task.Branch, gitStatus.Branch)
	worktreePath := gitStatus.Root
	worktreeDirty := gitStatus.Dirty
	if branch != "" && branch != gitStatus.Branch {
		if worktree, ok := worktreeForBranch(root, branch); ok {
			worktreePath = worktree.Path
			if status, err := fairwaygit.Check(worktree.Path, base); err == nil {
				worktreeDirty = status.Dirty
			}
		}
	}
	return reconcile.CloseoutGit{
		Branch:             branch,
		Base:               base,
		WorktreePath:       worktreePath,
		WorktreeDirty:      worktreeDirty,
		BranchExists:       fairwaygit.BranchExists(root, branch),
		BranchMerged:       fairwaygit.BranchMerged(root, branch, base),
		RemoteBranchExists: branch != base && fairwaygit.RemoteBranchExists(root, branch),
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
	fmt.Printf("summary: blockers=%d warnings=%d active_sessions=%d active_watchers=%d missing_review_domains=%d missing_commits=%d verification_evidence=%d pending_verification=%d dirty_worktrees=%d unmerged_branches=%d remote_branches=%d remote_branches_without_intent=%d safe_branches=%d preserved_branches=%d\n",
		report.Summary.Blockers,
		report.Summary.Warnings,
		report.Summary.ActiveSessions,
		report.Summary.ActiveWatchers,
		report.Summary.MissingReviewDomains,
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
		subcommandUsage("audit", "work-coverage|ci-learning")
		return nil
	}
	switch args[0] {
	case "work-coverage":
		return cmdAuditWorkCoverage(ctx, opts, args[1:])
	case "ci-learning":
		return cmdAuditCILearning(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown audit subcommand %q", args[0])
	}
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

func cmdAuditCILearning(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("audit ci-learning", flag.ContinueOnError)
	taskID := fs.String("task-id", "", "limit audit to one task")
	template := fs.Bool("template", false, "render learning artifact templates")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected audit ci-learning arguments: %s", strings.Join(fs.Args(), " "))
	}
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
		report, err := audit.BuildCILearningReport(ctx, cfg, s, audit.CILearningOptions{TaskID: *taskID, RenderTemplates: *template})
		if err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("ci_learning_ok: %t\n", report.OK)
		if report.TaskID != "" {
			fmt.Printf("task_id: %s\n", report.TaskID)
		}
		fmt.Printf("summary: failed_evidence=%d missing_follow_ups=%d missed_local_gates=%d missed_review_gates=%d ci_environment_only=%d flaky_runner_or_cache=%d approval_gated_blocker=%d\n",
			report.Summary.FailedEvidence,
			report.Summary.MissingFollowUps,
			report.Summary.MissedLocalGates,
			report.Summary.MissedReviewGates,
			report.Summary.CIEnvironmentOnly,
			report.Summary.FlakyRunnerOrCache,
			report.Summary.ApprovalGatedBlocker)
		if len(report.Findings) == 0 {
			fmt.Println("no CI/deploy learning findings")
			return nil
		}
		for _, finding := range report.Findings {
			fmt.Printf("warning\t%s\ttask=%s\tfollow_up=%s\tcommand=%s\n", finding.FailureClass, finding.TaskID, finding.FollowUpTask, finding.CommandText)
			if finding.FollowUpMissing {
				fmt.Printf("  missing follow-up: create %s-* task, suggested %s\n", finding.RecommendedFollowUpPrefix, finding.RecommendedFollowUpTaskID)
			}
			if finding.ExpectedLocalReproduction != "" {
				fmt.Printf("  reproduce: %s\n", finding.ExpectedLocalReproduction)
			}
			if finding.MissedGate != "" {
				fmt.Printf("  missed_gate: %s\n", finding.MissedGate)
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

type mergeReadyReport struct {
	OK                   bool                     `json:"ok"`
	TaskID               string                   `json:"task_id"`
	Git                  fairwaygit.Status        `json:"git"`
	Issues               []string                 `json:"issues"`
	Warnings             []string                 `json:"warnings,omitempty"`
	MissingReviewDomains []string                 `json:"missing_review_domains,omitempty"`
	GateEvaluations      []adoptionGateEvaluation `json:"gate_evaluations,omitempty"`
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
		if gitStatus.Dirty {
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
		report.MissingReviewDomains = missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)
		for _, domain := range report.MissingReviewDomains {
			report.Issues = append(report.Issues, "missing approved review for domain "+domain)
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
		report.OK = len(report.Issues) == 0
		if opts.JSON {
			if err := printJSON(report); err != nil {
				return err
			}
		} else {
			fmt.Printf("merge_ready: %t\ntask: %s\nbranch: %s\nbase: %s\n", report.OK, report.TaskID, report.Git.Branch, report.Git.Base)
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
	if report.HomebrewVersion == "" {
		report.Issues = append(report.Issues, "missing Homebrew cask version")
	} else if report.Version != "" && report.HomebrewVersion != report.Version {
		report.Issues = append(report.Issues, fmt.Sprintf("Homebrew cask version %q does not match release version %q", report.HomebrewVersion, report.Version))
	}
	if report.HomebrewTapCommit == "" {
		report.Issues = append(report.Issues, "missing Homebrew tap commit")
	}
	if report.ReleaseState == "draft" && report.HomebrewVersion != "" && report.HomebrewVersion == report.Version {
		report.Issues = append(report.Issues, "Homebrew cask points to this version while GitHub release is still draft")
		report.Recommendations = append(report.Recommendations, "publish the reviewed GitHub release draft before treating the Homebrew cask as usable")
	}
	if len(report.VerificationInputs) == 0 {
		report.Warnings = append(report.Warnings, "missing verification command list")
	}
	report.OK = len(report.Issues) == 0
	return report, nil
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
	cmd := exec.Command("tmux", "display-message", "-p", "-t", pane, "#{pane_id}")
	return cmd.Run() == nil
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
	fs := flag.NewFlagSet("coordinator plan", flag.ContinueOnError)
	readyLimit := fs.Int("ready-limit", 10, "maximum ready tasks to include")
	recommendationLimit := fs.Int("recommendation-limit", 20, "maximum next actions to include")
	staleAfter := fs.Duration("stale-checkpoint-after", 2*time.Hour, "active checkpoint stale threshold")
	monitorHandbackAfter := fs.Duration("monitor-handback-after", 2*time.Hour, "recent monitor handback window")
	allowUtility := fs.Bool("allow-utility-monitor", false, "recommend continuing configured utility monitors when present")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected coordinator plan arguments: %s", strings.Join(fs.Args(), " "))
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
		if opts.JSON {
			return printJSON(plan)
		}
		if tick {
			fmt.Println("coordinator tick")
		} else {
			fmt.Println("coordinator plan")
		}
		printCoordinatorPlan(plan)
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
	fmt.Printf("summary: top=%s ready=%d active=%d waiting=%d blocked=%d stale=%d complete=%d review_gated=%d review_debt=%d approval_gated=%d utility_gated=%d batch_recommended=%d\n",
		plan.Summary.TopClassification,
		plan.Summary.Ready,
		plan.Summary.Active,
		plan.Summary.Waiting,
		plan.Summary.Blocked,
		plan.Summary.Stale,
		plan.Summary.Complete,
		plan.Summary.ReviewGated,
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

func missingApprovedReviewDomains(domains []string, reviews []store.Review) []string {
	if len(domains) == 0 {
		return nil
	}
	approved := map[string]bool{}
	for _, review := range reviews {
		if review.Verdict == "approve" {
			approved[review.Reviewer] = true
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

func cmdPacket(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("packet requires subcommand: context, bugfix, watcher, release-run, template, architecture-map, boundary-guard, vertical-slice")
	}
	if isHelpOnly(args) {
		subcommandUsage("packet", "context|bugfix|watcher|release-run|template|architecture-map|boundary-guard|vertical-slice")
		return nil
	}
	switch args[0] {
	case "context":
		return cmdPacketContext(ctx, opts, args[1:])
	case "bugfix":
		return cmdPacketBugfix(ctx, opts, args[1:])
	case "watcher":
		return cmdPacketWatcher(opts, args[1:])
	case "release-run":
		return cmdPacketReleaseRun(ctx, opts, args[1:])
	case "template":
		return cmdPacketTemplate(ctx, opts, args[1:])
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
		packet := struct {
			Template config.PacketTemplate `json:"template"`
			Task     store.Task            `json:"task"`
			Fields   map[string][]string   `json:"fields"`
			Evidence []store.Evidence      `json:"evidence"`
			Reviews  []store.Review        `json:"reviews"`
		}{template, task, values, evidence, reviews}
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
		printEvidenceSummary(evidence)
		fmt.Println("\n## Reviews")
		for _, review := range reviews {
			fmt.Printf("- %s by %s: %s\n", review.Verdict, review.Reviewer, review.Reason)
		}
		return nil
	})
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
	cfg, root, _, err := loadConfig(opts)
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
	reg, err = registry.Register(reg, registry.Project{Name: projectName, Path: root, DBPath: config.DBPath(cfg, root)})
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
		fmt.Printf("%s\t%s\t%s\n", project.Name, project.Path, project.DBPath)
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
				if err := printDetail(ctx, s, parts[1], false); err != nil {
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
	return config.PacketTemplate{}, false
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
		if *priority >= 0 {
			tasks = filterByPriority(tasks, *priority)
		}
		if *within != "" {
			all, err := s.AllTasks(ctx)
			if err != nil {
				return err
			}
			tasks = filterByAncestor(tasks, all, *within)
		}
		if opts.JSON {
			return printJSON(tasks)
		}
		printTasks(tasks)
		return nil
	})
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
	if err := s.SetTaskIDPattern(cfg.Fairway.TaskIDPattern); err != nil {
		_ = s.Close()
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
			subcommandUsage("record", "evidence|guard-report|handoff|review|usage|push-intent")
			return nil
		}
		return errors.New("record requires type and task id")
	}
	if isHelpOnly(args) {
		subcommandUsage("record", "evidence|guard-report|handoff|review|usage|push-intent")
		return nil
	}
	switch args[0] {
	case "evidence":
		return recordEvidence(ctx, opts, args[1:])
	case "guard-report":
		return recordGuardReport(ctx, opts, args[1:])
	case "handoff":
		return recordHandoff(ctx, opts, args[1:])
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

func printEvidenceStatusPrompt(result string) {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "pass":
		fmt.Println("next: mark done, or record a checkpoint explaining why the task remains open")
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
		subcommandUsage("usage", "report")
		return nil
	}
	switch args[0] {
	case "report":
		return cmdUsageReport(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown usage subcommand %q", args[0])
	}
}

func cmdUsageReport(ctx context.Context, opts globalOptions, args []string) error {
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
	case "provider", "task", "epic", "role", "day", "kind", "phase":
		return true
	default:
		return false
	}
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
	if len(args) > 0 {
		if isHelpOnly(args) {
			subcommandUsage("dashboard", "[--listen <addr>] [--no-open] [--multi] | start|stop|restart|status")
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
		url := dashboard.URL(addr)
		if !isLoopbackAddr(addr) {
			fmt.Fprintln(os.Stderr, "warning: dashboard is not bound to a loopback address; v0.1 has no authentication")
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
		dbPath := project.DBPath
		if dbPath == "" {
			dbPath = filepath.Join(project.Path, ".fairway", "state.db")
		}
		projectStore, err := store.Open(ctx, dbPath, project.Name)
		if err != nil {
			return fmt.Errorf("open project %s: %w", project.Name, err)
		}
		projects = append(projects, dashboard.ProjectStore{Name: project.Name, Path: project.Path, Store: projectStore})
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
	PIDFile string `json:"pid_file"`
	LogFile string `json:"log_file"`
	PID     int    `json:"pid,omitempty"`
	Running bool   `json:"running"`
	URL     string `json:"url,omitempty"`
}

func cmdDashboardLifecycle(ctx context.Context, opts globalOptions, action string, args []string) error {
	fs := flag.NewFlagSet("dashboard "+action, flag.ContinueOnError)
	listen := fs.String("listen", "", "listen address")
	noOpen := fs.Bool("no-open", false, "do not open browser")
	open := fs.Bool("open", false, "open browser after starting")
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
		return startDashboardLifecycle(opts, addr, childNoOpen, *multi, resolvedPIDFile, resolvedLogFile, true)
	case "start":
		if status.Running {
			return printDashboardLifecycleStatus(status, opts.JSON)
		}
		return startDashboardLifecycle(opts, addr, childNoOpen, *multi, resolvedPIDFile, resolvedLogFile, false)
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
	status := dashboardLifecycleStatus{PIDFile: pidFile, LogFile: logFile, URL: dashboard.URL(addr)}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return status, nil
		}
		return status, err
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return status, nil
	}
	var pid int
	if _, err := fmt.Sscanf(raw, "%d", &pid); err != nil {
		return status, fmt.Errorf("read dashboard pid file %s: invalid pid %q", pidFile, raw)
	}
	status.PID = pid
	status.Running = processAlive(pid)
	if !status.Running {
		_ = os.Remove(pidFile)
		status.PID = 0
	}
	return status, nil
}

func printDashboardLifecycleStatus(status dashboardLifecycleStatus, asJSON bool) error {
	if asJSON {
		return printJSON(status)
	}
	state := "stopped"
	if status.Running {
		state = "running"
	}
	fmt.Printf("dashboard %s", state)
	if status.PID > 0 {
		fmt.Printf("\tpid=%d", status.PID)
	}
	if status.URL != "" {
		fmt.Printf("\t%s", status.URL)
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

func startDashboardLifecycle(opts globalOptions, addr string, noOpen bool, multi bool, pidFile string, logFile string, restarted bool) error {
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
	childArgs := dashboardLifecycleChildArgs(opts, addr, noOpen, multi)
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
	fmt.Printf("dashboard %s\tpid=%d\t%s\tpid_file=%s\tlog_file=%s\n", verb, cmd.Process.Pid, dashboard.URL(addr), pidFile, logFile)
	return nil
}

func dashboardLifecycleChildArgs(opts globalOptions, addr string, noOpen bool, multi bool) []string {
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
		return errors.New("db requires backup, export, migrate, or compat")
	}
	if isHelpOnly(args) {
		subcommandUsage("db", "backup|export|migrate|compat")
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

func printDetail(ctx context.Context, s *store.Store, taskID string, asJSON bool) error {
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
	if asJSON {
		missingReviewDomains := missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)
		reviewStatus := effectiveReviewStatus(task.ReviewStatus, missingReviewDomains)
		return printJSON(struct {
			Task                 store.Task            `json:"task"`
			ReviewStatus         string                `json:"review_status"`
			Transitions          []store.Transition    `json:"transitions"`
			Evidence             []store.Evidence      `json:"evidence"`
			Handoffs             []store.Handoff       `json:"handoffs"`
			Reviews              []store.Review        `json:"reviews"`
			MissingReviewDomains []string              `json:"missing_review_domains,omitempty"`
			Sessions             []store.Session       `json:"sessions"`
			Usage                []store.ProviderUsage `json:"usage"`
			UsageRollups         []store.UsageRollup   `json:"usage_rollups"`
			Batches              []store.WorkBatch     `json:"batches"`
		}{task, reviewStatus, transitions, evidence, handoffs, reviews, missingReviewDomains, sessions, usageEvents, usageRollups, batches})
	}
	missingReviewDomains := missingApprovedReviewDomains(task.Definition.ReviewDomains, reviews)
	reviewStatus := effectiveReviewStatus(task.ReviewStatus, missingReviewDomains)
	fmt.Printf("%s %s\nstatus: %s\nrole: %s\nowner: %s\nreview: %s\n\n%s\n", task.Definition.ID, task.Definition.Title, task.Status, task.Definition.Role, task.Owner, reviewStatus, task.Definition.Notes)
	printTaskMetadata(task.Definition)
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
	fmt.Println("\nhandoffs:")
	for _, h := range handoffs {
		fmt.Printf("- to %s: %s\n", h.ToRole, h.Payload)
	}
	fmt.Println("\nreviews:")
	for _, r := range reviews {
		fmt.Printf("- %s by %s: %s\n", r.Verdict, r.Reviewer, r.Reason)
	}
	if len(missingReviewDomains) > 0 {
		fmt.Println("\nmissing review domains:")
		for _, domain := range missingReviewDomains {
			fmt.Printf("- %s\n", domain)
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
	fmt.Println("fairway init|import|add|spawn|update|tree|ready|claim|set-status|record|usage report|task-detail|status-report|health-report|timing-report|dispatch-plan|git-check|preflight|workflow check|closeout|batch create|add|remove|evidence|link|show|list|audit work-coverage|ci-learning|release verify|merge-ready|route review|review checkout|worktree|session|reconcile active|coordinator|readiness|adoption|parity|checkpoint|packet|packet template|watcher|regression-pack|tracker|register|unregister|projects|tui|config validate|dashboard [start|stop|restart|status]|version")
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
