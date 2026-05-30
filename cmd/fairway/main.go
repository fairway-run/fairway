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
	"strings"
	"syscall"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/dashboard"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/importer"
	"github.com/subashram/fairway/internal/registry"
	"github.com/subashram/fairway/internal/state"
	"github.com/subashram/fairway/internal/store"
	"gopkg.in/yaml.v3"
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
	case "coordinator":
		return cmdCoordinator(ctx, opts, args[1:])
	case "adoption":
		return cmdAdoption(ctx, opts, args[1:])
	case "parity":
		return cmdParity(ctx, opts, args[1:])
	case "checkpoint":
		return cmdCheckpoint(ctx, opts, args[1:])
	case "packet":
		return cmdPacket(ctx, opts, args[1:])
	case "watcher":
		return cmdWatcher(ctx, opts, args[1:])
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
	acceptance := fs.String("acceptance", "", "acceptance check")
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
	if *acceptance != "" {
		task.AcceptanceChecks = []string{*acceptance}
	}
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
	acceptance := fs.String("acceptance", "", "acceptance check")
	force := fs.Bool("force", false, "suppress granularity warnings")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected spawn arguments: %s", strings.Join(fs.Args(), " "))
	}
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
		if *acceptance != "" {
			task.AcceptanceChecks = []string{*acceptance}
		}
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
	acceptance := fs.String("acceptance", "", "acceptance check")
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
			if *acceptance == "" {
				task.AcceptanceChecks = nil
			} else {
				task.AcceptanceChecks = []string{*acceptance}
			}
		}
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

type mergeReadyReport struct {
	OK     bool              `json:"ok"`
	TaskID string            `json:"task_id"`
	Git    fairwaygit.Status `json:"git"`
	Issues []string          `json:"issues"`
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
		if _, _, _, _, _, err := s.TaskDetail(ctx, taskID); err != nil {
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
		if cfg.Gates.RequireHandoffBeforeMergeReady {
			ok, err := s.HasHandoff(ctx, taskID)
			if err != nil {
				return err
			}
			if !ok {
				report.Issues = append(report.Issues, "missing handoff")
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
		return errors.New("session requires subcommand: upsert, status, end")
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

type reconcileAction struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	PID       *int   `json:"pid,omitempty"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
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
	return withStore(ctx, opts, func(ctx context.Context, _ config.Config, _ string, s *store.Store) error {
		sessions, err := s.Sessions(ctx, false)
		if err != nil {
			return err
		}
		var actions []reconcileAction
		for _, session := range sessions {
			if session.PID == nil {
				continue
			}
			if processAlive(*session.PID) {
				continue
			}
			action := reconcileAction{SessionID: session.ID, Role: session.Role, PID: session.PID, Action: "mark_stale", Reason: "pid not running"}
			actions = append(actions, action)
			if !*dryRun {
				if err := s.EndSession(ctx, session.ID, "stale", "reconciled", nil); err != nil {
					return err
				}
			}
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
			fmt.Printf("%s\t%s\t%s\t%s\n", mode, action.Action, action.SessionID, action.Reason)
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
		sessionID := generatedSessionID(sessionRole, os.Getpid())
		session := store.Session{
			ID:             sessionID,
			Role:           sessionRole,
			WorktreePath:   worktreePath,
			Branch:         branch,
			SessionBackend: "shell",
			Provider:       providerName,
			SessionName:    *name,
			TaskID:         *taskID,
			Status:         "running",
		}
		if err := s.UpsertSession(ctx, session); err != nil {
			return err
		}
		if opts.JSON {
			return printJSON(session)
		}
		fmt.Printf("export FAIRWAY_SESSION_ID=%s\n", sessionID)
		if *taskID != "" {
			fmt.Printf("export FAIRWAY_TASK_ID=%s\n", *taskID)
		}
		fmt.Printf("cd %s\n", worktreePath)
		return nil
	})
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
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
		return errors.New("coordinator requires subcommand: preflight, status, tick")
	}
	switch args[0] {
	case "preflight":
		return cmdCoordinatorReport(ctx, opts, args[1:], true, false)
	case "status":
		return cmdCoordinatorReport(ctx, opts, args[1:], false, false)
	case "tick":
		return cmdCoordinatorReport(ctx, opts, args[1:], false, true)
	default:
		return fmt.Errorf("unknown coordinator subcommand %q", args[0])
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
	Mode                  string   `json:"mode"`
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
	switch args[0] {
	case "artifact":
		return cmdAdoptionArtifact(ctx, opts, "adoption", args[1:])
	default:
		return fmt.Errorf("unknown adoption subcommand %q", args[0])
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
				Mode:                  gate.Mode,
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
		profileTasks := tasksForProfile(profile, tasks)
		for _, gate := range profile.Gates {
			evaluation := adoptionGateEvaluation{
				Profile:      profile.Name,
				Gate:         gate.Name,
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

func tasksForProfile(profile config.WorkstreamProfile, tasks []store.Task) []store.Task {
	if len(profile.TaskKinds) == 0 {
		return tasks
	}
	kinds := map[string]bool{}
	for _, kind := range profile.TaskKinds {
		kinds[kind] = true
	}
	var out []store.Task
	for _, task := range tasks {
		if kinds[task.Definition.Kind] {
			out = append(out, task)
		}
	}
	return out
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
				fmt.Printf("- %s: %s (%s)\n", label, gate.Mode, gate.EvidenceType)
			} else {
				fmt.Printf("- %s: %s\n", label, gate.Mode)
			}
			var requirements []string
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
		return errors.New("packet requires subcommand: context, bugfix, watcher, architecture-map, boundary-guard, vertical-slice")
	}
	switch args[0] {
	case "context":
		return cmdPacketContext(ctx, opts, args[1:])
	case "bugfix":
		return cmdPacketBugfix(ctx, opts, args[1:])
	case "watcher":
		return cmdPacketWatcher(opts, args[1:])
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
		return errors.New("tracker requires subcommand: link, links, reconcile")
	}
	switch args[0] {
	case "link":
		return cmdTrackerLink(ctx, opts, args[1:])
	case "links":
		return cmdTrackerLinks(ctx, opts, args[1:])
	case "reconcile":
		return cmdTrackerReconcile(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown tracker subcommand %q", args[0])
	}
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
		link := store.TrackerLink{TaskID: args[0], Provider: *provider, ExternalID: *externalID, URL: *url}
		if err := s.UpsertTrackerLink(ctx, link); err != nil {
			return err
		}
		fmt.Printf("linked %s to %s:%s\n", args[0], *provider, *externalID)
		return nil
	})
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
		report := struct {
			DryRun bool                `json:"dry_run"`
			Links  []store.TrackerLink `json:"links"`
			Note   string              `json:"note"`
		}{*dryRun, links, "external Jira/Linear API sync is intentionally not performed by the local-first prototype"}
		if opts.JSON {
			return printJSON(report)
		}
		fmt.Printf("tracker reconcile dry_run=%t links=%d\n", report.DryRun, len(report.Links))
		fmt.Println(report.Note)
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
	fmt.Println("commands: ready|r, claim|c <id>, status|s, detail|d <id>, help|h, quit|q")
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
		return s.RecordEvidence(ctx, taskID, store.Evidence{CommandText: *commandText, Result: *result, ArtifactPath: *artifact, ArtifactType: *artifactType, DurationSeconds: durationPtr, Notes: *notes})
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
		return dashboard.New(s, roleNames(cfg), dashboardWorktrees(worktrees)).ListenAndServe(addr)
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
	fmt.Println("fairway init|import|add|spawn|update|tree|ready|claim|set-status|record|task-detail|status-report|health-report|timing-report|dispatch-plan|git-check|preflight|merge-ready|route review|review checkout|worktree|session|coordinator|adoption|parity|checkpoint|packet|watcher|regression-pack|tracker|register|unregister|projects|tui|config validate|dashboard|version")
}
