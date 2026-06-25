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
	Fairway             FairwayConfig        `toml:"fairway"`
	Dashboard           DashboardConfig      `toml:"dashboard"`
	Worktrees           WorktreesConfig      `toml:"worktrees"`
	Sessions            SessionsConfig       `toml:"sessions"`
	Coordinator         CoordinatorConfig    `toml:"coordinator"`
	States              StatesConfig         `toml:"states"`
	Gates               GatesConfig          `toml:"gates"`
	TaskKinds           TaskKindsConfig      `toml:"task_kinds"`
	TaskPriorities      TaskPrioritiesConfig `toml:"task_priorities"`
	Roles               []Role               `toml:"roles"`
	ReviewRoutes        []ReviewRoute        `toml:"review_routes"`
	WorkstreamProfiles  []WorkstreamProfile  `toml:"workstream_profiles"`
	ReviewProfiles      []ReviewProfile      `toml:"review_profiles"`
	PacketTemplates     []PacketTemplate     `toml:"packet_templates"`
	RuleSources         []RuleSource         `toml:"rule_sources"`
	ProviderTargets     []ProviderTarget     `toml:"provider_targets"`
	ProviderModelPrices []ProviderModelPrice `toml:"provider_model_prices"`
	AdvisoryAdapters    []AdvisoryAdapter    `toml:"advisory_provider_adapters"`
	ExternalNotifiers   []ExternalNotifier   `toml:"external_notifiers"`
}

type FairwayConfig struct {
	ProjectName        string   `toml:"project_name"`
	DBPath             string   `toml:"db_path"`
	QueueSource        string   `toml:"queue_source"`
	MainBranch         string   `toml:"main_branch"`
	TaskIDPattern      string   `toml:"task_id_pattern"`
	LocalArtifactPaths []string `toml:"local_artifact_paths"`
}

type DashboardConfig struct {
	Listen       string `toml:"listen"`
	AutoOpen     bool   `toml:"auto_open"`
	ReadOnly     bool   `toml:"read_only"`
	TrustedProxy string `toml:"trusted_proxy"`
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

type CoordinatorConfig struct {
	NotificationAckTimeout string `toml:"notification_ack_timeout"`
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
	RuleGroups      []string                `toml:"rule_groups"`
	TagGroups       []WorkstreamTagGroup    `toml:"tag_groups"`
	ReviewDomains   []string                `toml:"review_domains"`
	RouteSamples    []string                `toml:"route_samples"`
	Gates           []WorkstreamProfileGate `toml:"gates"`
}

type WorkstreamTagGroup struct {
	Name        string   `toml:"name"`
	Tags        []string `toml:"tags"`
	Description string   `toml:"description"`
}

type WorkstreamProfileGate struct {
	Name                  string   `toml:"name"`
	Group                 string   `toml:"group"`
	Mode                  string   `toml:"mode"`
	TaskKinds             []string `toml:"task_kinds"`
	EvidenceType          string   `toml:"evidence_type"`
	RequiredEvidenceCount int      `toml:"required_evidence_count"`
	AcceptedResults       []string `toml:"accepted_results"`
	ArtifactRequired      bool     `toml:"artifact_required"`
	OwnerSignoffRequired  bool     `toml:"owner_signoff_required"`
	ExpiresAfter          string   `toml:"expires_after"`
	Description           string   `toml:"description"`
}

type ReviewProfile struct {
	Name                     string   `toml:"name"`
	Mode                     string   `toml:"mode"`
	MatchKinds               []string `toml:"match_kinds"`
	MatchRiskLevels          []string `toml:"match_risk_levels"`
	MatchTags                []string `toml:"match_tags"`
	MatchAuthoringDomains    []string `toml:"match_authoring_domains"`
	MatchOwningDomains       []string `toml:"match_owning_domains"`
	MatchPaths               []string `toml:"match_paths"`
	RequiredReviewDomains    []string `toml:"required_review_domains"`
	ExtraReviewerRationale   string   `toml:"extra_reviewer_rationale"`
	InheritFromParent        bool     `toml:"inherit_from_parent"`
	InheritReviewDomains     []string `toml:"inherit_review_domains"`
	WaiveReviewDomains       []string `toml:"waive_review_domains"`
	DeferReviewDomains       []string `toml:"defer_review_domains"`
	SafeIterationZone        bool     `toml:"safe_iteration_zone"`
	SafeIterationDefectClass string   `toml:"safe_iteration_defect_class"`
	SafeIterationControl     string   `toml:"safe_iteration_control"`
	NoInheritanceKinds       []string `toml:"no_inheritance_kinds"`
	NoInheritanceRiskLevels  []string `toml:"no_inheritance_risk_levels"`
	NoInheritanceTags        []string `toml:"no_inheritance_tags"`
	NoInheritancePaths       []string `toml:"no_inheritance_paths"`
	GroupReview              bool     `toml:"group_review"`
	ProcessHypothesis        string   `toml:"process_hypothesis"`
	OutcomeMetrics           []string `toml:"outcome_metrics"`
}

type PacketTemplate struct {
	Profiles       []string `toml:"profiles"`
	Name           string   `toml:"name"`
	RequiredFields []string `toml:"required_fields"`
	OptionalFields []string `toml:"optional_fields"`
}

type RuleSource struct {
	Name      string `toml:"name"`
	Source    string `toml:"source"`
	Mode      string `toml:"mode"`
	CommitSHA string `toml:"commit_sha"`
	Checksum  string `toml:"checksum"`
}

type ProviderTarget struct {
	Domain   string `toml:"domain"`
	Provider string `toml:"provider"`
	Target   string `toml:"target"`
	Type     string `toml:"type"`
}

type ProviderModelPrice struct {
	Provider              string   `toml:"provider"`
	Model                 string   `toml:"model"`
	InputPerMillion       *float64 `toml:"input_per_million"`
	CachedInputPerMillion *float64 `toml:"cached_input_per_million"`
	OutputPerMillion      *float64 `toml:"output_per_million"`
	ReasoningPerMillion   *float64 `toml:"reasoning_per_million"`
	TotalPerMillion       *float64 `toml:"total_per_million"`
}

type AdvisoryAdapter struct {
	Name           string   `toml:"name"`
	Provider       string   `toml:"provider"`
	Type           string   `toml:"type"`
	Mode           string   `toml:"mode"`
	Trust          string   `toml:"trust"`
	Model          string   `toml:"model"`
	EndpointEnv    string   `toml:"endpoint_env"`
	Capabilities   []string `toml:"capabilities"`
	AllowedActions []string `toml:"allowed_actions"`
}

type ExternalNotifier struct {
	Name               string   `toml:"name"`
	Type               string   `toml:"type"`
	Mode               string   `toml:"mode"`
	TargetEnv          string   `toml:"target_env"`
	TokenEnv           string   `toml:"token_env"`
	Domains            []string `toml:"domains"`
	TemplateName       string   `toml:"template_name"`
	RateLimitPerMinute int      `toml:"rate_limit_per_minute"`
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
			Listen:       "127.0.0.1:7878",
			AutoOpen:     true,
			TrustedProxy: "none",
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
		Coordinator: CoordinatorConfig{
			NotificationAckTimeout: "24h",
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
	switch strings.TrimSpace(cfg.Dashboard.TrustedProxy) {
	case "", "none", "cloudflare_access", "identity_aware_proxy":
	default:
		return fmt.Errorf("[dashboard] trusted_proxy %q is unsupported", cfg.Dashboard.TrustedProxy)
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
	if err := validateReviewProfiles(cfg.ReviewProfiles, kindSet); err != nil {
		return err
	}
	if err := validatePacketTemplates(cfg.PacketTemplates, workstreamProfileSet(cfg.WorkstreamProfiles)); err != nil {
		return err
	}
	if err := validateRuleSources(cfg.RuleSources); err != nil {
		return err
	}
	if err := validateProviderTargets(cfg.ProviderTargets); err != nil {
		return err
	}
	if err := validateProviderModelPrices(cfg.ProviderModelPrices); err != nil {
		return err
	}
	if err := validateAdvisoryAdapters(cfg.AdvisoryAdapters); err != nil {
		return err
	}
	if err := validateExternalNotifiers(cfg.ExternalNotifiers); err != nil {
		return err
	}
	return nil
}

func validateExternalNotifiers(notifiers []ExternalNotifier) error {
	seen := map[string]bool{}
	for _, notifier := range notifiers {
		name := strings.TrimSpace(notifier.Name)
		if name == "" {
			return errors.New("[[external_notifiers]] name is required")
		}
		if seen[name] {
			return fmt.Errorf("duplicate external notifier %q", name)
		}
		seen[name] = true
		notifierType := strings.TrimSpace(notifier.Type)
		if notifierType == "" {
			notifierType = "noop"
		}
		switch notifierType {
		case "noop", "log", "webhook":
		default:
			return fmt.Errorf("[[external_notifiers]] type %q is invalid for %q", notifier.Type, name)
		}
		mode := strings.TrimSpace(notifier.Mode)
		if mode == "" {
			mode = "dry_run"
		}
		switch mode {
		case "dry_run", "send", "disabled":
		default:
			return fmt.Errorf("[[external_notifiers]] mode %q is invalid for %q", notifier.Mode, name)
		}
		if mode == "send" && notifierType == "noop" {
			return fmt.Errorf("[[external_notifiers]] mode send requires a delivery-capable type for %q", name)
		}
		if err := validateAdapterTokenList("[[external_notifiers]] domain", name, notifier.Domains); err != nil {
			return err
		}
		if targetEnv := strings.TrimSpace(notifier.TargetEnv); targetEnv != "" && !validEnvName(targetEnv) {
			return fmt.Errorf("[[external_notifiers]] target_env %q is invalid for %q", notifier.TargetEnv, name)
		}
		if mode == "send" && strings.TrimSpace(notifier.TargetEnv) == "" {
			return fmt.Errorf("[[external_notifiers]] target_env is required for send notifier %q", name)
		}
		if tokenEnv := strings.TrimSpace(notifier.TokenEnv); tokenEnv != "" && !validEnvName(tokenEnv) {
			return fmt.Errorf("[[external_notifiers]] token_env %q is invalid for %q", notifier.TokenEnv, name)
		}
		if notifier.RateLimitPerMinute < 0 {
			return fmt.Errorf("[[external_notifiers]] rate_limit_per_minute must be non-negative for %q", name)
		}
		if strings.ContainsAny(strings.TrimSpace(notifier.TemplateName), "\r\n") {
			return fmt.Errorf("[[external_notifiers]] template_name for %q must be a single line", name)
		}
	}
	return nil
}

func validateAdvisoryAdapters(adapters []AdvisoryAdapter) error {
	seen := map[string]bool{}
	for _, adapter := range adapters {
		name := strings.TrimSpace(adapter.Name)
		if name == "" {
			return errors.New("[[advisory_provider_adapters]] name is required")
		}
		if seen[name] {
			return fmt.Errorf("duplicate advisory provider adapter %q", name)
		}
		seen[name] = true
		if strings.TrimSpace(adapter.Provider) == "" {
			return fmt.Errorf("[[advisory_provider_adapters]] provider is required for %q", name)
		}
		adapterType := strings.TrimSpace(adapter.Type)
		if adapterType == "" {
			adapterType = "noop"
		}
		switch adapterType {
		case "noop", "rules-only", "local_ollama", "local_llamacpp", "openai-compatible", "codex", "claude", "gemini":
		default:
			return fmt.Errorf("[[advisory_provider_adapters]] type %q is invalid for %q", adapter.Type, name)
		}
		mode := strings.TrimSpace(adapter.Mode)
		if mode == "" {
			mode = "advisory"
		}
		switch mode {
		case "advisory", "report_only", "disabled":
		default:
			return fmt.Errorf("[[advisory_provider_adapters]] mode %q is invalid for %q", adapter.Mode, name)
		}
		trust := strings.TrimSpace(adapter.Trust)
		if trust == "" {
			trust = "low"
		}
		switch trust {
		case "low", "medium", "high":
		default:
			return fmt.Errorf("[[advisory_provider_adapters]] trust %q is invalid for %q", adapter.Trust, name)
		}
		if err := validateAdapterTokenList("[[advisory_provider_adapters]] capability", name, adapter.Capabilities); err != nil {
			return err
		}
		for _, action := range adapter.AllowedActions {
			action = strings.TrimSpace(action)
			if action == "" {
				return fmt.Errorf("[[advisory_provider_adapters]] allowed_actions contains empty action for %q", name)
			}
			if !AllowedAdvisoryAction(action) {
				return fmt.Errorf("[[advisory_provider_adapters]] allowed action %q is invalid for %q", action, name)
			}
		}
		if endpointEnv := strings.TrimSpace(adapter.EndpointEnv); endpointEnv != "" && !validEnvName(endpointEnv) {
			return fmt.Errorf("[[advisory_provider_adapters]] endpoint_env %q is invalid for %q", adapter.EndpointEnv, name)
		}
	}
	return nil
}

func validateAdapterTokenList(label, adapterName string, values []string) error {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s contains empty value for %q", label, adapterName)
		}
		if strings.ContainsAny(value, " \t\r\n") {
			return fmt.Errorf("%s %q for %q must be a single token", label, value, adapterName)
		}
	}
	return nil
}

func validEnvName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func AllowedAdvisoryAction(action string) bool {
	switch action {
	case "inspect_task", "route_review", "record_evidence", "refresh_memory", "render_packet", "create_follow_up", "wake_provider", "run_preflight", "record_checkpoint":
		return true
	default:
		return false
	}
}

func validateProviderModelPrices(prices []ProviderModelPrice) error {
	seen := map[string]bool{}
	for _, price := range prices {
		provider := strings.TrimSpace(price.Provider)
		model := strings.TrimSpace(price.Model)
		if provider == "" {
			return errors.New("[[provider_model_prices]] provider is required")
		}
		if model == "" {
			return errors.New("[[provider_model_prices]] model is required; use * for a provider default")
		}
		key := provider + "\x00" + model
		if seen[key] {
			return fmt.Errorf("duplicate provider model price for provider %q model %q", provider, model)
		}
		seen[key] = true
		values := []struct {
			name  string
			value *float64
		}{
			{"input_per_million", price.InputPerMillion},
			{"cached_input_per_million", price.CachedInputPerMillion},
			{"output_per_million", price.OutputPerMillion},
			{"reasoning_per_million", price.ReasoningPerMillion},
			{"total_per_million", price.TotalPerMillion},
		}
		known := false
		for _, item := range values {
			if item.value == nil {
				continue
			}
			known = true
			if *item.value < 0 {
				return fmt.Errorf("[[provider_model_prices]] %s must not be negative", item.name)
			}
		}
		if !known {
			return fmt.Errorf("[[provider_model_prices]] provider %q model %q must set at least one price field", provider, model)
		}
	}
	return nil
}

func validateProviderTargets(targets []ProviderTarget) error {
	seen := map[string]bool{}
	for _, target := range targets {
		domain := strings.TrimSpace(target.Domain)
		if domain == "" {
			return errors.New("[[provider_targets]] domain is required")
		}
		if strings.TrimSpace(target.Provider) == "" {
			return errors.New("[[provider_targets]] provider is required")
		}
		if strings.TrimSpace(target.Target) == "" {
			return errors.New("[[provider_targets]] target is required")
		}
		targetType := strings.TrimSpace(target.Type)
		if targetType == "" {
			targetType = "generic"
		}
		switch targetType {
		case "generic", "thread", "tmux", "cli", "webhook":
		default:
			return fmt.Errorf("[[provider_targets]] type %q is invalid", target.Type)
		}
		key := domain + "\x00" + strings.TrimSpace(target.Provider) + "\x00" + strings.TrimSpace(target.Target)
		if seen[key] {
			return fmt.Errorf("duplicate provider target for domain %q", domain)
		}
		seen[key] = true
	}
	return nil
}

func validateRuleSources(sources []RuleSource) error {
	seen := map[string]bool{}
	for _, source := range sources {
		name := strings.TrimSpace(source.Name)
		if name == "" {
			return errors.New("[[rule_sources]] name is required")
		}
		if seen[name] {
			return fmt.Errorf("duplicate rule source %q", name)
		}
		seen[name] = true

		rawSource := strings.TrimSpace(source.Source)
		if rawSource == "" {
			return fmt.Errorf("rule source %q source is required", name)
		}
		mode := strings.TrimSpace(source.Mode)
		if mode == "" {
			mode = "advisory"
		}
		switch mode {
		case "advisory", "blocking", "disabled":
		default:
			return fmt.Errorf("rule source %q has unknown mode %q", name, source.Mode)
		}

		scheme, value, ok := strings.Cut(rawSource, ":")
		if !ok || strings.TrimSpace(scheme) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("rule source %q source must use scheme:path form", name)
		}
		switch scheme {
		case "path", "file":
			// Local sources are the only enabled source type for the first loader.
		case "github":
			if mode != "disabled" {
				return fmt.Errorf("rule source %q uses github source; remote fetch is not supported yet, set mode=\"disabled\"", name)
			}
			if strings.TrimSpace(source.CommitSHA) == "" || strings.TrimSpace(source.Checksum) == "" {
				return fmt.Errorf("rule source %q remote source must include commit_sha and checksum before it can be represented safely", name)
			}
		default:
			return fmt.Errorf("rule source %q has unsupported source scheme %q", name, scheme)
		}
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
		if err := validateStringList("[[workstream_profiles]] rule_groups", profile.RuleGroups); err != nil {
			return err
		}
		tagGroups := map[string]bool{}
		for _, group := range profile.TagGroups {
			if group.Name == "" {
				return fmt.Errorf("[[workstream_profiles.tag_groups]] name is required for profile %q", profile.Name)
			}
			if tagGroups[group.Name] {
				return fmt.Errorf("duplicate tag group %q in workstream profile %q", group.Name, profile.Name)
			}
			tagGroups[group.Name] = true
			if err := validateStringList("[[workstream_profiles.tag_groups]] tags", group.Tags); err != nil {
				return err
			}
			if len(group.Tags) == 0 {
				return fmt.Errorf("[[workstream_profiles.tag_groups]] tags is required for group %q in profile %q", group.Name, profile.Name)
			}
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
			if strings.TrimSpace(gate.Group) != gate.Group {
				return fmt.Errorf("[[workstream_profiles.gates]] group for gate %q must not have leading or trailing whitespace", gate.Name)
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
			if err := validateStringList("[[workstream_profiles.gates]] task_kinds", gate.TaskKinds); err != nil {
				return err
			}
			if len(kindSet) > 0 {
				for _, kind := range gate.TaskKinds {
					if !kindSet[kind] {
						return fmt.Errorf("[[workstream_profiles.gates]] task kind %q for gate %q is not in [task_kinds].allowed", kind, gate.Name)
					}
				}
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

func validateReviewProfiles(profiles []ReviewProfile, kindSet map[string]bool) error {
	seen := map[string]bool{}
	for _, profile := range profiles {
		if profile.Name == "" {
			return errors.New("[[review_profiles]] name is required")
		}
		if seen[profile.Name] {
			return fmt.Errorf("duplicate review profile %q", profile.Name)
		}
		seen[profile.Name] = true
		switch strings.TrimSpace(profile.Mode) {
		case "", "advisory", "blocking":
		default:
			return fmt.Errorf("[[review_profiles]] mode %q for profile %q must be advisory or blocking", profile.Mode, profile.Name)
		}
		kindLists := map[string][]string{
			"match_kinds":          profile.MatchKinds,
			"no_inheritance_kinds": profile.NoInheritanceKinds,
		}
		for label, values := range kindLists {
			if err := validateStringList("[[review_profiles]] "+label, values); err != nil {
				return err
			}
			if len(kindSet) > 0 {
				for _, kind := range values {
					if !kindSet[kind] {
						return fmt.Errorf("[[review_profiles]] task kind %q for profile %q is not in [task_kinds].allowed", kind, profile.Name)
					}
				}
			}
		}
		lists := map[string][]string{
			"match_risk_levels":          profile.MatchRiskLevels,
			"match_tags":                 profile.MatchTags,
			"match_authoring_domains":    profile.MatchAuthoringDomains,
			"match_owning_domains":       profile.MatchOwningDomains,
			"match_paths":                profile.MatchPaths,
			"required_review_domains":    profile.RequiredReviewDomains,
			"inherit_review_domains":     profile.InheritReviewDomains,
			"waive_review_domains":       profile.WaiveReviewDomains,
			"defer_review_domains":       profile.DeferReviewDomains,
			"no_inheritance_risk_levels": profile.NoInheritanceRiskLevels,
			"no_inheritance_tags":        profile.NoInheritanceTags,
			"no_inheritance_paths":       profile.NoInheritancePaths,
			"outcome_metrics":            profile.OutcomeMetrics,
		}
		for label, values := range lists {
			if err := validateStringList("[[review_profiles]] "+label, values); err != nil {
				return err
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
read_only = false
trusted_proxy = "none"

[worktrees]
root = "../worktrees"
naming = "{repo}-{role}"
review_branch_naming = "review/{role}"

[sessions]
default_backend = "shell"
stale_after = "12h"

[coordinator]
notification_ack_timeout = "24h"

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
