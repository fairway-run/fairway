package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
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
	case "checkpoint":
		return cmdCheckpoint(ctx, opts, args[1:])
	case "packet":
		return cmdPacket(ctx, opts, args[1:])
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
		return errors.New("packet requires subcommand: context")
	}
	switch args[0] {
	case "context":
		return cmdPacketContext(ctx, opts, args[1:])
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
		if err := applyImportDefaults(tasks, cfg); err != nil {
			return err
		}
		if err := validateTaskMetadata(tasks, cfg); err != nil {
			return err
		}
		if err := s.ImportTasks(ctx, tasks); err != nil {
			return err
		}
		fmt.Printf("imported %d tasks\n", len(tasks))
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = noOpen
	return withStore(ctx, opts, func(ctx context.Context, cfg config.Config, _ string, s *store.Store) error {
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
		return dashboard.New(s, roleNames(cfg)).ListenAndServe(addr)
	})
}

func cmdDB(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("db requires backup or export")
	}
	switch args[0] {
	case "backup":
		return cmdDBBackup(ctx, opts, args[1:])
	case "export":
		return cmdDBExport(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown db command %q", args[0])
	}
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
	fmt.Println("fairway init|import|add|spawn|update|tree|ready|claim|set-status|record|task-detail|status-report|health-report|timing-report|git-check|preflight|merge-ready|route review|review checkout|worktree|session|coordinator|checkpoint|packet|config validate|dashboard|version")
}
