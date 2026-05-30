package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const DefaultConfigPath = ".fairway/config.toml"

type Config struct {
	Fairway            FairwayConfig        `toml:"fairway"`
	Dashboard          DashboardConfig      `toml:"dashboard"`
	Worktrees          WorktreesConfig      `toml:"worktrees"`
	Sessions           SessionsConfig       `toml:"sessions"`
	States             StatesConfig         `toml:"states"`
	Gates              GatesConfig          `toml:"gates"`
	TaskKinds          TaskKindsConfig      `toml:"task_kinds"`
	TaskPriorities     TaskPrioritiesConfig `toml:"task_priorities"`
	Roles              []Role               `toml:"roles"`
	ReviewRoutes       []ReviewRoute        `toml:"review_routes"`
	WorkstreamProfiles []WorkstreamProfile  `toml:"workstream_profiles"`
	PacketTemplates    []PacketTemplate     `toml:"packet_templates"`
}

type FairwayConfig struct {
	ProjectName   string `toml:"project_name"`
	DBPath        string `toml:"db_path"`
	QueueSource   string `toml:"queue_source"`
	MainBranch    string `toml:"main_branch"`
	TaskIDPattern string `toml:"task_id_pattern"`
}

type DashboardConfig struct {
	Listen   string `toml:"listen"`
	AutoOpen bool   `toml:"auto_open"`
}

type WorktreesConfig struct {
	Root               string `toml:"root"`
	Naming             string `toml:"naming"`
	ReviewBranchNaming string `toml:"review_branch_naming"`
}

type SessionsConfig struct {
	DefaultBackend string `toml:"default_backend"`
	StaleAfter     string `toml:"stale_after"`
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

type ReviewRoute struct {
	Match    string `toml:"match"`
	Reviewer string `toml:"reviewer"`
}

type WorkstreamProfile struct {
	Name            string                  `toml:"name"`
	TaskKinds       []string                `toml:"task_kinds"`
	DashboardGroups []string                `toml:"dashboard_groups"`
	ReviewDomains   []string                `toml:"review_domains"`
	RouteSamples    []string                `toml:"route_samples"`
	Gates           []WorkstreamProfileGate `toml:"gates"`
}

type WorkstreamProfileGate struct {
	Name                  string   `toml:"name"`
	Mode                  string   `toml:"mode"`
	EvidenceType          string   `toml:"evidence_type"`
	RequiredEvidenceCount int      `toml:"required_evidence_count"`
	AcceptedResults       []string `toml:"accepted_results"`
	ArtifactRequired      bool     `toml:"artifact_required"`
	OwnerSignoffRequired  bool     `toml:"owner_signoff_required"`
	ExpiresAfter          string   `toml:"expires_after"`
	Description           string   `toml:"description"`
}

type PacketTemplate struct {
	Profiles       []string `toml:"profiles"`
	Name           string   `toml:"name"`
	RequiredFields []string `toml:"required_fields"`
	OptionalFields []string `toml:"optional_fields"`
}

func Defaults(root string) Config {
	project := filepath.Base(root)
	if project == "." || project == string(filepath.Separator) {
		project = "fairway"
	}
	return Config{
		Fairway: FairwayConfig{
			ProjectName:   project,
			DBPath:        ".fairway/state.db",
			QueueSource:   "inline",
			MainBranch:    "main",
			TaskIDPattern: `^[A-Z]+-[0-9]+$`,
		},
		Dashboard: DashboardConfig{
			Listen:   "127.0.0.1:7878",
			AutoOpen: true,
		},
		Worktrees: WorktreesConfig{
			Root:               "../worktrees",
			Naming:             "{repo}-{role}",
			ReviewBranchNaming: "review/{role}",
		},
		Sessions: SessionsConfig{
			DefaultBackend: "shell",
			StaleAfter:     "12h",
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
	if cfg.Fairway.TaskIDPattern == "" {
		return errors.New("[fairway] task_id_pattern is required")
	}
	if _, err := regexp.Compile(cfg.Fairway.TaskIDPattern); err != nil {
		return fmt.Errorf("[fairway] task_id_pattern is invalid: %w", err)
	}
	if cfg.Worktrees.Root == "" {
		return errors.New("[worktrees] root is required")
	}
	if !strings.Contains(cfg.Worktrees.Naming, "{role}") {
		return errors.New("[worktrees] naming must include {role}")
	}
	if !strings.Contains(cfg.Worktrees.ReviewBranchNaming, "{role}") {
		return errors.New("[worktrees] review_branch_naming must include {role}")
	}
	if cfg.Sessions.DefaultBackend == "" {
		return errors.New("[sessions] default_backend is required")
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
	for _, route := range cfg.ReviewRoutes {
		if route.Match == "" {
			return errors.New("[[review_routes]] match is required")
		}
		if route.Reviewer == "" {
			return errors.New("[[review_routes]] reviewer is required")
		}
		if len(seen) > 0 && !seen[route.Reviewer] {
			return fmt.Errorf("[[review_routes]] reviewer %q is not a configured role", route.Reviewer)
		}
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
	if err := validateWorkstreamProfiles(cfg.WorkstreamProfiles, kindSet); err != nil {
		return err
	}
	if err := validatePacketTemplates(cfg.PacketTemplates, workstreamProfileSet(cfg.WorkstreamProfiles)); err != nil {
		return err
	}
	return nil
}

func validateWorkstreamProfiles(profiles []WorkstreamProfile, kindSet map[string]bool) error {
	seen := map[string]bool{}
	validGateModes := map[string]bool{"advisory": true, "blocking": true, "report_only": true}
	validEvidenceResults := map[string]bool{"pass": true, "fail": true, "partial": true, "skipped": true, "blocked": true}
	for _, profile := range profiles {
		if profile.Name == "" {
			return errors.New("[[workstream_profiles]] name is required")
		}
		if seen[profile.Name] {
			return fmt.Errorf("duplicate workstream profile %q", profile.Name)
		}
		seen[profile.Name] = true
		if err := validateStringList("[[workstream_profiles]] task_kinds", profile.TaskKinds); err != nil {
			return err
		}
		if len(kindSet) > 0 {
			for _, kind := range profile.TaskKinds {
				if !kindSet[kind] {
					return fmt.Errorf("[[workstream_profiles]] task kind %q in profile %q is not in [task_kinds].allowed", kind, profile.Name)
				}
			}
		}
		if err := validateStringList("[[workstream_profiles]] dashboard_groups", profile.DashboardGroups); err != nil {
			return err
		}
		if err := validateStringList("[[workstream_profiles]] review_domains", profile.ReviewDomains); err != nil {
			return err
		}
		if err := validateStringList("[[workstream_profiles]] route_samples", profile.RouteSamples); err != nil {
			return err
		}
		gates := map[string]bool{}
		for _, gate := range profile.Gates {
			if gate.Name == "" {
				return fmt.Errorf("[[workstream_profiles.gates]] name is required for profile %q", profile.Name)
			}
			if gates[gate.Name] {
				return fmt.Errorf("duplicate gate %q in workstream profile %q", gate.Name, profile.Name)
			}
			gates[gate.Name] = true
			if gate.Mode == "" {
				return fmt.Errorf("[[workstream_profiles.gates]] mode is required for gate %q", gate.Name)
			}
			if !validGateModes[gate.Mode] {
				return fmt.Errorf("[[workstream_profiles.gates]] mode %q must be advisory, blocking, or report_only", gate.Mode)
			}
			if gate.RequiredEvidenceCount < 0 {
				return fmt.Errorf("[[workstream_profiles.gates]] required_evidence_count for gate %q must be >= 0", gate.Name)
			}
			if err := validateStringList("[[workstream_profiles.gates]] accepted_results", gate.AcceptedResults); err != nil {
				return err
			}
			for _, result := range gate.AcceptedResults {
				if !validEvidenceResults[result] {
					return fmt.Errorf("[[workstream_profiles.gates]] accepted result %q for gate %q must be pass, fail, partial, skipped, or blocked", result, gate.Name)
				}
			}
			if gate.ExpiresAfter != "" {
				if _, err := time.ParseDuration(gate.ExpiresAfter); err != nil {
					return fmt.Errorf("[[workstream_profiles.gates]] expires_after for gate %q is invalid: %w", gate.Name, err)
				}
			}
		}
	}
	return nil
}

func validatePacketTemplates(templates []PacketTemplate, profileSet map[string]bool) error {
	seen := map[string]bool{}
	for _, template := range templates {
		if template.Name == "" {
			return errors.New("[[packet_templates]] name is required")
		}
		if seen[template.Name] {
			return fmt.Errorf("duplicate packet template %q", template.Name)
		}
		seen[template.Name] = true
		if err := validateStringList("[[packet_templates]] profiles", template.Profiles); err != nil {
			return err
		}
		if len(profileSet) > 0 {
			for _, profile := range template.Profiles {
				if !profileSet[profile] {
					return fmt.Errorf("[[packet_templates]] profile %q for template %q is not a configured workstream profile", profile, template.Name)
				}
			}
		}
		if err := validateStringList("[[packet_templates]] required_fields", template.RequiredFields); err != nil {
			return err
		}
		if err := validateStringList("[[packet_templates]] optional_fields", template.OptionalFields); err != nil {
			return err
		}
		fieldSeen := map[string]bool{}
		for _, field := range template.RequiredFields {
			fieldSeen[field] = true
		}
		for _, field := range template.OptionalFields {
			if fieldSeen[field] {
				return fmt.Errorf("[[packet_templates]] field %q appears in required_fields and optional_fields for template %q", field, template.Name)
			}
		}
	}
	return nil
}

func workstreamProfileSet(profiles []WorkstreamProfile) map[string]bool {
	set := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		set[profile.Name] = true
	}
	return set
}

func validateStringList(label string, values []string) error {
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("%s contains empty value", label)
		}
		if seen[value] {
			return fmt.Errorf("%s contains duplicate value %q", label, value)
		}
		seen[value] = true
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

func RoleBranch(role Role) string {
	if role.Branch != "" {
		return role.Branch
	}
	return "agent/" + role.Name
}

func WorktreeRoot(cfg Config, root string) string {
	if filepath.IsAbs(cfg.Worktrees.Root) {
		return cfg.Worktrees.Root
	}
	return filepath.Join(root, cfg.Worktrees.Root)
}

func WorktreePath(cfg Config, root string, role Role) string {
	name := cfg.Worktrees.Naming
	name = strings.ReplaceAll(name, "{repo}", filepath.Base(root))
	name = strings.ReplaceAll(name, "{role}", role.Name)
	return filepath.Join(WorktreeRoot(cfg, root), name)
}

func ReviewBranch(cfg Config, role Role) string {
	name := cfg.Worktrees.ReviewBranchNaming
	name = strings.ReplaceAll(name, "{role}", role.Name)
	return name
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

func WorkstreamProfileSet(cfg Config) map[string]bool {
	profiles := make(map[string]bool, len(cfg.WorkstreamProfiles))
	for _, profile := range cfg.WorkstreamProfiles {
		profiles[profile.Name] = true
	}
	return profiles
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
task_id_pattern = "^[A-Z]+-[0-9]+$"

[dashboard]
listen = "127.0.0.1:7878"
auto_open = true

[worktrees]
root = "../worktrees"
naming = "{repo}-{role}"
review_branch_naming = "review/{role}"

[sessions]
default_backend = "shell"
stale_after = "12h"

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
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	ignorePath := filepath.Join(filepath.Dir(path), ".gitignore")
	if _, err := os.Stat(ignorePath); errors.Is(err, os.ErrNotExist) {
		ignore := "state.db\nstate.db-*\n"
		return os.WriteFile(ignorePath, []byte(ignore), 0o644)
	}
	return nil
}

func DBPath(cfg Config, root string) string {
	if filepath.IsAbs(cfg.Fairway.DBPath) {
		return cfg.Fairway.DBPath
	}
	return filepath.Join(root, cfg.Fairway.DBPath)
}
