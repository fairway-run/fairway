package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const DefaultConfigPath = ".fairway/config.toml"

type Config struct {
	Fairway        FairwayConfig        `toml:"fairway"`
	Dashboard      DashboardConfig      `toml:"dashboard"`
	States         StatesConfig         `toml:"states"`
	Gates          GatesConfig          `toml:"gates"`
	TaskKinds      TaskKindsConfig      `toml:"task_kinds"`
	TaskPriorities TaskPrioritiesConfig `toml:"task_priorities"`
	Roles          []Role               `toml:"roles"`
}

type FairwayConfig struct {
	ProjectName string `toml:"project_name"`
	DBPath      string `toml:"db_path"`
	QueueSource string `toml:"queue_source"`
	MainBranch  string `toml:"main_branch"`
}

type DashboardConfig struct {
	Listen   string `toml:"listen"`
	AutoOpen bool   `toml:"auto_open"`
}

type StatesConfig struct {
	Allowed     []string    `toml:"allowed"`
	Terminal    []string    `toml:"terminal"`
	Transitions [][2]string `toml:"transitions"`
}

type GatesConfig struct {
	RequireEvidenceBeforeDone      bool `toml:"require_evidence_before_done"`
	RequireReviewBeforeDone        bool `toml:"require_review_before_done"`
	RequireHandoffBeforeMergeReady bool `toml:"require_handoff_before_merge_ready"`
	RequireBlockedReason           bool `toml:"require_blocked_reason"`
	AllowForceWithoutReason        bool `toml:"allow_force_without_reason"`
}

type Role struct {
	Name     string `toml:"name"`
	Branch   string `toml:"branch"`
	Provider string `toml:"provider"`
}

type TaskKindsConfig struct {
	Allowed []string `toml:"allowed"`
	Default string   `toml:"default"`
}

type TaskPrioritiesConfig struct {
	Default *int            `toml:"default"`
	Levels  []PriorityLevel `toml:"levels"`
}

type PriorityLevel struct {
	Rank        int    `toml:"rank"`
	Label       string `toml:"label"`
	Description string `toml:"description"`
}

func Defaults(root string) Config {
	project := filepath.Base(root)
	if project == "." || project == string(filepath.Separator) {
		project = "fairway"
	}
	return Config{
		Fairway: FairwayConfig{
			ProjectName: project,
			DBPath:      ".fairway/state.db",
			QueueSource: "inline",
			MainBranch:  "main",
		},
		Dashboard: DashboardConfig{
			Listen:   "127.0.0.1:7878",
			AutoOpen: true,
		},
		States: StatesConfig{
			Allowed:  []string{"todo", "in_progress", "blocked", "done"},
			Terminal: []string{"done"},
		},
		Gates: GatesConfig{
			RequireBlockedReason:    true,
			AllowForceWithoutReason: false,
		},
		TaskKinds: TaskKindsConfig{
			Default: "task",
		},
	}
}

func Load(path string) (Config, string, error) {
	if path == "" {
		var err error
		path, err = FindConfig("")
		if err != nil {
			return Config{}, "", err
		}
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Config{}, "", err
	}
	path = absPath
	root := RootForConfigPath(path)
	cfg := Defaults(root)
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, "", fmt.Errorf("decode config: %w", err)
	}
	if err := Validate(cfg); err != nil {
		return Config{}, "", err
	}
	return cfg, root, nil
}

func RootForConfigPath(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) == ".fairway" {
		return filepath.Dir(dir)
	}
	return dir
}

func FindConfig(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, DefaultConfigPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no .fairway/config.toml found")
		}
		dir = parent
	}
}

func Validate(cfg Config) error {
	if cfg.Fairway.ProjectName == "" {
		return errors.New("[fairway] project_name is required")
	}
	if cfg.Fairway.DBPath == "" {
		return errors.New("[fairway] db_path is required")
	}
	if len(cfg.States.Allowed) == 0 {
		return errors.New("[states] allowed must not be empty")
	}
	allowed := map[string]bool{}
	for _, state := range cfg.States.Allowed {
		if state == "" {
			return errors.New("[states] allowed contains empty state")
		}
		if allowed[state] {
			return fmt.Errorf("duplicate state %q", state)
		}
		allowed[state] = true
	}
	for _, terminal := range cfg.States.Terminal {
		if !allowed[terminal] {
			return fmt.Errorf("terminal state %q is not in allowed states", terminal)
		}
	}
	for _, transition := range cfg.States.Transitions {
		if transition[0] != "*" && !allowed[transition[0]] {
			return fmt.Errorf("transition from state %q is not in allowed states", transition[0])
		}
		if !allowed[transition[1]] {
			return fmt.Errorf("transition to state %q is not in allowed states", transition[1])
		}
	}
	seen := map[string]bool{}
	for _, role := range cfg.Roles {
		if role.Name == "" {
			return errors.New("[[roles]] name is required")
		}
		if seen[role.Name] {
			return fmt.Errorf("duplicate role %q", role.Name)
		}
		seen[role.Name] = true
	}
	if cfg.TaskKinds.Default == "" {
		return errors.New("[task_kinds] default is required")
	}
	kindSet := map[string]bool{}
	for _, kind := range cfg.TaskKinds.Allowed {
		if kind == "" {
			return errors.New("[task_kinds] allowed contains empty kind")
		}
		if kindSet[kind] {
			return fmt.Errorf("duplicate task kind %q", kind)
		}
		kindSet[kind] = true
	}
	if len(kindSet) > 0 && !kindSet[cfg.TaskKinds.Default] {
		return fmt.Errorf("[task_kinds] default %q is not in allowed kinds", cfg.TaskKinds.Default)
	}
	priorityRanks := map[int]bool{}
	for _, level := range cfg.TaskPriorities.Levels {
		if level.Label == "" {
			return errors.New("[[task_priorities.levels]] label is required")
		}
		if priorityRanks[level.Rank] {
			return fmt.Errorf("duplicate priority rank %d", level.Rank)
		}
		priorityRanks[level.Rank] = true
	}
	if cfg.TaskPriorities.Default != nil && len(priorityRanks) > 0 && !priorityRanks[*cfg.TaskPriorities.Default] {
		return fmt.Errorf("[task_priorities] default %d is not in configured levels", *cfg.TaskPriorities.Default)
	}
	return nil
}

func RoleSet(cfg Config) map[string]bool {
	roles := make(map[string]bool, len(cfg.Roles))
	for _, role := range cfg.Roles {
		roles[role.Name] = true
	}
	return roles
}

func TaskKindSet(cfg Config) map[string]bool {
	kinds := make(map[string]bool, len(cfg.TaskKinds.Allowed))
	for _, kind := range cfg.TaskKinds.Allowed {
		kinds[kind] = true
	}
	return kinds
}

func PrioritySet(cfg Config) map[int]bool {
	priorities := make(map[int]bool, len(cfg.TaskPriorities.Levels))
	for _, level := range cfg.TaskPriorities.Levels {
		priorities[level.Rank] = true
	}
	return priorities
}

func DefaultTaskKind(cfg Config) string {
	if cfg.TaskKinds.Default == "" {
		return "task"
	}
	return cfg.TaskKinds.Default
}

func DefaultPriority(cfg Config) *int {
	if cfg.TaskPriorities.Default == nil {
		return nil
	}
	v := *cfg.TaskPriorities.Default
	return &v
}

func WriteDefault(path, root string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	cfg := Defaults(root)
	text := fmt.Sprintf(`[fairway]
project_name = %q
db_path = ".fairway/state.db"
queue_source = "inline"
main_branch = "main"

[dashboard]
listen = "127.0.0.1:7878"
auto_open = true

[states]
allowed = ["todo", "in_progress", "blocked", "done"]
terminal = ["done"]

[gates]
require_evidence_before_done = false
require_review_before_done = false
require_handoff_before_merge_ready = false
require_blocked_reason = true
allow_force_without_reason = false

[task_kinds]
default = "task"
`, cfg.Fairway.ProjectName)
	return os.WriteFile(path, []byte(text), 0o644)
}

func DBPath(cfg Config, root string) string {
	if filepath.IsAbs(cfg.Fairway.DBPath) {
		return cfg.Fairway.DBPath
	}
	return filepath.Join(root, cfg.Fairway.DBPath)
}
