package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestRootForConfigPath_DefaultFairwayDir(t *testing.T) {
	got := RootForConfigPath(filepath.Join("/tmp", "repo", ".fairway", "config.toml"))
	want := filepath.Join("/tmp", "repo")
	if got != want {
		t.Fatalf("root=%q, want %q", got, want)
	}
}

func TestValidateConsumerReadiness(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.ConsumerReadiness = ConsumerReadinessConfig{
		MinimumVersion: "0.1.12", MinimumSchemaVersion: 13,
		PinnedBinaryPath: "/tmp/fairway", RequiredCapabilities: []string{"managed-binary-cache"},
		RequiredCommands: []string{"binary status"}, RequiredFeatures: []string{"managed_binary_cache"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []struct {
		name  string
		apply func(*ConsumerReadinessConfig)
		want  string
	}{
		{name: "invalid version", apply: func(c *ConsumerReadinessConfig) { c.MinimumVersion = "latest" }, want: "minimum_version"},
		{name: "negative schema", apply: func(c *ConsumerReadinessConfig) { c.MinimumSchemaVersion = -1 }, want: "non-negative"},
		{name: "capability whitespace", apply: func(c *ConsumerReadinessConfig) { c.RequiredCapabilities = []string{"managed cache"} }, want: "required_capabilities"},
		{name: "duplicate feature", apply: func(c *ConsumerReadinessConfig) { c.RequiredFeatures = []string{"cache", "cache"} }, want: "duplicate"},
		{name: "multiline command", apply: func(c *ConsumerReadinessConfig) { c.RequiredCommands = []string{"binary status\nrm"} }, want: "single lines"},
		{name: "multiline path", apply: func(c *ConsumerReadinessConfig) { c.PinnedBinaryPath = "one\ntwo" }, want: "single line"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := Defaults(t.TempDir())
			tc.apply(&candidate.ConsumerReadiness)
			err := Validate(candidate)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestValidateRuleSources(t *testing.T) {
	base := Defaults(t.TempDir())
	base.RuleSources = []RuleSource{
		{Name: "platform", Source: "path:../fairway-rules-platform", Mode: "advisory"},
		{Name: "service-platform", Source: "file:/opt/fairway-rules-service-platform", Mode: "blocking"},
		{Name: "codeguard", Source: "github:fairway-run/fairway-rules-codeguard", Mode: "disabled", CommitSHA: strings.Repeat("a", 40), Checksum: "sha256:abc123"},
	}
	if err := Validate(base); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []struct {
		name    string
		sources []RuleSource
		want    string
	}{
		{
			name:    "duplicate name",
			sources: []RuleSource{{Name: "platform", Source: "path:one"}, {Name: "platform", Source: "path:two"}},
			want:    `duplicate rule source "platform"`,
		},
		{
			name:    "unknown mode",
			sources: []RuleSource{{Name: "platform", Source: "path:../rules", Mode: "strict"}},
			want:    `unknown mode "strict"`,
		},
		{
			name:    "empty source",
			sources: []RuleSource{{Name: "platform", Source: ""}},
			want:    `source is required`,
		},
		{
			name:    "malformed source",
			sources: []RuleSource{{Name: "platform", Source: "../rules"}},
			want:    `scheme:path form`,
		},
		{
			name:    "unsupported scheme",
			sources: []RuleSource{{Name: "platform", Source: "https://example.com/rules"}},
			want:    `unsupported source scheme "https"`,
		},
		{
			name:    "github enabled",
			sources: []RuleSource{{Name: "platform", Source: "github:fairway-run/fairway-rules-platform", Mode: "advisory", CommitSHA: "abc", Checksum: "sha256:abc"}},
			want:    `remote fetch is not supported yet`,
		},
		{
			name:    "github missing pin",
			sources: []RuleSource{{Name: "platform", Source: "github:fairway-run/fairway-rules-platform", Mode: "disabled"}},
			want:    `commit_sha and checksum`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults(t.TempDir())
			cfg.RuleSources = tc.sources
			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestValidateProviderTargets(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.ProviderTargets = []ProviderTarget{
		{Domain: "security", Provider: "codex", Type: "thread", Target: "thread-1"},
		{Domain: "ops", Provider: "tmux", Type: "tmux", Target: "ops:0.1"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []struct {
		name    string
		targets []ProviderTarget
		want    string
	}{
		{
			name:    "missing domain",
			targets: []ProviderTarget{{Provider: "codex", Target: "thread-1"}},
			want:    "domain is required",
		},
		{
			name:    "missing provider",
			targets: []ProviderTarget{{Domain: "security", Target: "thread-1"}},
			want:    "provider is required",
		},
		{
			name:    "missing target",
			targets: []ProviderTarget{{Domain: "security", Provider: "codex"}},
			want:    "target is required",
		},
		{
			name:    "invalid type",
			targets: []ProviderTarget{{Domain: "security", Provider: "codex", Type: "socket", Target: "thread-1"}},
			want:    "type \"socket\" is invalid",
		},
		{
			name: "duplicate",
			targets: []ProviderTarget{
				{Domain: "security", Provider: "codex", Type: "thread", Target: "thread-1"},
				{Domain: "security", Provider: "codex", Type: "thread", Target: "thread-1"},
			},
			want: "duplicate provider target",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults(t.TempDir())
			cfg.ProviderTargets = tc.targets
			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestValidateProviderModelPrices(t *testing.T) {
	cfg := Defaults(t.TempDir())
	input := 1.25
	cached := 0.125
	output := 10.0
	cfg.ProviderModelPrices = []ProviderModelPrice{
		{Provider: "codex", Model: "gpt-5-codex", InputPerMillion: &input, CachedInputPerMillion: &cached, OutputPerMillion: &output},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []struct {
		name   string
		prices []ProviderModelPrice
		want   string
	}{
		{
			name:   "missing provider",
			prices: []ProviderModelPrice{{Model: "*", InputPerMillion: &input}},
			want:   "provider is required",
		},
		{
			name:   "missing model",
			prices: []ProviderModelPrice{{Provider: "codex", InputPerMillion: &input}},
			want:   "model is required",
		},
		{
			name: "duplicate",
			prices: []ProviderModelPrice{
				{Provider: "codex", Model: "*", InputPerMillion: &input},
				{Provider: "codex", Model: "*", OutputPerMillion: &output},
			},
			want: "duplicate provider model price",
		},
		{
			name:   "empty price",
			prices: []ProviderModelPrice{{Provider: "codex", Model: "*"}},
			want:   "must set at least one price field",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults(t.TempDir())
			cfg.ProviderModelPrices = tc.prices
			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}

	negative := -1.0
	cfg = Defaults(t.TempDir())
	cfg.ProviderModelPrices = []ProviderModelPrice{{Provider: "codex", Model: "*", InputPerMillion: &negative}}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "must not be negative") {
		t.Fatalf("Validate() error = %v, want negative price rejection", err)
	}
}

func TestValidateAdvisoryAdapters(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.AdvisoryAdapters = []AdvisoryAdapter{
		{
			Name:           "local-rules",
			Provider:       "ollama",
			Type:           "local_ollama",
			Mode:           "advisory",
			Trust:          "low",
			Model:          "llama3.1",
			EndpointEnv:    "FAIRWAY_OLLAMA_ENDPOINT",
			Capabilities:   []string{"summarize_evidence", "rank_ready_tasks"},
			AllowedActions: []string{"inspect_task", "render_packet"},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []struct {
		name     string
		adapters []AdvisoryAdapter
		want     string
	}{
		{
			name:     "missing name",
			adapters: []AdvisoryAdapter{{Provider: "ollama"}},
			want:     "name is required",
		},
		{
			name:     "missing provider",
			adapters: []AdvisoryAdapter{{Name: "local"}},
			want:     "provider is required",
		},
		{
			name:     "duplicate",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama"}, {Name: "local", Provider: "codex"}},
			want:     "duplicate advisory provider adapter",
		},
		{
			name:     "invalid type",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", Type: "browser"}},
			want:     "type \"browser\" is invalid",
		},
		{
			name:     "invalid mode",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", Mode: "blocking"}},
			want:     "mode \"blocking\" is invalid",
		},
		{
			name:     "invalid trust",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", Trust: "root"}},
			want:     "trust \"root\" is invalid",
		},
		{
			name:     "invalid action",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", AllowedActions: []string{"approve_review"}}},
			want:     "allowed action \"approve_review\" is invalid",
		},
		{
			name:     "invalid endpoint env",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", EndpointEnv: "FAIRWAY-ENDPOINT"}},
			want:     "endpoint_env \"FAIRWAY-ENDPOINT\" is invalid",
		},
		{
			name:     "capability must be token",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", Capabilities: []string{"raw prompt"}}},
			want:     "must be a single token",
		},
		{
			name:     "narrative requires local ollama",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", Type: "openai-compatible", Model: "model", EndpointEnv: "FAIRWAY_ENDPOINT", Capabilities: []string{"explain_code_narrative"}, AllowedActions: []string{"render_packet"}}},
			want:     "requires local_ollama type",
		},
		{
			name:     "narrative requires model",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", Type: "local_ollama", EndpointEnv: "FAIRWAY_ENDPOINT", Capabilities: []string{"explain_code_narrative"}, AllowedActions: []string{"render_packet"}}},
			want:     "requires model",
		},
		{
			name:     "narrative requires endpoint env",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", Type: "local_ollama", Model: "model", Capabilities: []string{"explain_code_narrative"}, AllowedActions: []string{"render_packet"}}},
			want:     "requires endpoint_env",
		},
		{
			name:     "narrative requires render packet",
			adapters: []AdvisoryAdapter{{Name: "local", Provider: "ollama", Type: "local_ollama", Model: "model", EndpointEnv: "FAIRWAY_ENDPOINT", Capabilities: []string{"explain_code_narrative"}}},
			want:     "requires render_packet",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults(t.TempDir())
			cfg.AdvisoryAdapters = tc.adapters
			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestValidateExternalNotifiers(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.ExternalNotifiers = []ExternalNotifier{
		{
			Name:         "control-log",
			Type:         "log",
			Mode:         "dry_run",
			TargetEnv:    "FAIRWAY_NOTIFY_LOG",
			Domains:      []string{"coordinator", "ops"},
			TemplateName: "control_room_handoff",
		},
		{
			Name:               "control-webhook",
			Type:               "webhook",
			Mode:               "send",
			TargetEnv:          "FAIRWAY_NOTIFY_WEBHOOK",
			TokenEnv:           "FAIRWAY_NOTIFY_TOKEN",
			Domains:            []string{"coordinator"},
			TemplateName:       "control_room_handoff",
			RateLimitPerMinute: 10,
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []struct {
		name      string
		notifiers []ExternalNotifier
		want      string
	}{
		{
			name:      "missing name",
			notifiers: []ExternalNotifier{{Type: "log"}},
			want:      "name is required",
		},
		{
			name:      "duplicate",
			notifiers: []ExternalNotifier{{Name: "control"}, {Name: "control"}},
			want:      "duplicate external notifier",
		},
		{
			name:      "invalid type",
			notifiers: []ExternalNotifier{{Name: "control", Type: "slack"}},
			want:      `type "slack" is invalid`,
		},
		{
			name:      "send noop invalid",
			notifiers: []ExternalNotifier{{Name: "control", Type: "noop", Mode: "send", TargetEnv: "FAIRWAY_NOTIFY_LOG"}},
			want:      `mode send requires a delivery-capable type`,
		},
		{
			name:      "invalid target env",
			notifiers: []ExternalNotifier{{Name: "control", TargetEnv: "FAIRWAY-NOTIFY"}},
			want:      `target_env "FAIRWAY-NOTIFY" is invalid`,
		},
		{
			name:      "send requires target env",
			notifiers: []ExternalNotifier{{Name: "control", Type: "log", Mode: "send"}},
			want:      `target_env is required`,
		},
		{
			name:      "invalid token env",
			notifiers: []ExternalNotifier{{Name: "control", Type: "webhook", Mode: "send", TargetEnv: "FAIRWAY_NOTIFY_WEBHOOK", TokenEnv: "BAD-TOKEN"}},
			want:      `token_env "BAD-TOKEN" is invalid`,
		},
		{
			name:      "invalid rate limit",
			notifiers: []ExternalNotifier{{Name: "control", Type: "log", Mode: "send", TargetEnv: "FAIRWAY_NOTIFY_LOG", RateLimitPerMinute: -1}},
			want:      "rate_limit_per_minute must be non-negative",
		},
		{
			name:      "domain must be token",
			notifiers: []ExternalNotifier{{Name: "control", Domains: []string{"ops team"}}},
			want:      "must be a single token",
		},
		{
			name:      "template is single line",
			notifiers: []ExternalNotifier{{Name: "control", TemplateName: "one\ntwo"}},
			want:      "must be a single line",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults(t.TempDir())
			cfg.ExternalNotifiers = tc.notifiers
			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestRootForConfigPath_CustomConfig(t *testing.T) {
	got := RootForConfigPath(filepath.Join("/tmp", "repo", "fairway.toml"))
	want := filepath.Join("/tmp", "repo")
	if got != want {
		t.Fatalf("root=%q, want %q", got, want)
	}
}

func TestValidate_RejectsTerminalOutsideAllowed(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.States.Terminal = []string{"finished"}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected terminal validation error")
	}
}

func TestValidate_RejectsTransitionOutsideAllowed(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.States.Transitions = [][2]string{{"todo", "finished"}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected transition validation error")
	}
}

func TestValidate_RejectsWorktreeNamingWithoutRole(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.Worktrees.Naming = "worker"
	if err := Validate(cfg); err == nil {
		t.Fatal("expected worktree naming validation error")
	}
}

func TestLoadToleratesHistoricalDashboardSurfaceKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".fairway", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`[fairway]
project_name = "legacy"
db_path = ".fairway/state.db"
queue_source = "inline"
main_branch = "main"
task_id_pattern = "^[A-Z]+-[0-9]+$"

[dashboard]
listen = "127.0.0.1:7878"
auto_open = true
surface = "v2"

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
require_blocked_reason = true
allow_force_without_reason = false

[task_kinds]
default = "task"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() historical dashboard surface key error = %v", err)
	}
	if cfg.Dashboard.Listen != "127.0.0.1:7878" {
		t.Fatalf("dashboard listen=%q", cfg.Dashboard.Listen)
	}
}

func TestLoadDashboardSharedReadOnlyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".fairway", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`
[fairway]
project_name = "fairway-test"
db_path = ".fairway/state.db"
queue_source = "inline"
main_branch = "main"
task_id_pattern = "^[A-Z]+-[0-9]+$"

[dashboard]
listen = "127.0.0.1:7878"
auto_open = false
read_only = true
trusted_proxy = "cloudflare_access"

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
require_blocked_reason = true
allow_force_without_reason = false

[task_kinds]
default = "task"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Dashboard.ReadOnly || cfg.Dashboard.TrustedProxy != "cloudflare_access" || cfg.Dashboard.AutoOpen {
		t.Fatalf("dashboard config=%+v, want read-only cloudflare_access with auto_open false", cfg.Dashboard)
	}
}

func TestValidateRejectsUnsupportedDashboardTrustedProxy(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Dashboard.TrustedProxy = "cloudflare"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "trusted_proxy") {
		t.Fatalf("Validate unsupported trusted_proxy err=%v, want trusted_proxy error", err)
	}
}

func TestValidateSharedTeamServerReadOnlyMode(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.Listen = "127.0.0.1:7880"
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate read-only server mode error = %v", err)
	}
}

func TestValidateRejectsSharedTeamServerWriteMode(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Mode = "shared_write"
	cfg.Server.ReadOnly = false
	cfg.Server.WriteEnabled = true
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "append-only api-write-pilot only") {
		t.Fatalf("Validate broad write-capable server mode err=%v, want append-only-only error", err)
	}
}

func TestValidateSharedTeamServerIdentityModes(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Enabled = true
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.IdentityMode = "api_token"
	cfg.Server.APITokenEnv = "FAIRWAY_TEST_API_TOKEN"
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate api_token identity mode error = %v", err)
	}

	cfg.Server.IdentityMode = "service_account"
	cfg.Server.APITokenEnv = ""
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate service_account placeholder error = %v", err)
	}
}

func TestValidateRejectsTrustedProxyIdentityWithoutProof(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Enabled = true
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.IdentityMode = "trusted_proxy_read_only"
	cfg.Server.TrustedProxyVerified = false
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "trusted_proxy_verified") {
		t.Fatalf("Validate unverified trusted proxy err=%v, want trusted_proxy_verified error", err)
	}
}

func TestValidateRejectsUnsupportedServerRole(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Enabled = true
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.AllowedRoles = []string{"root"}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "unsupported role") {
		t.Fatalf("Validate unsupported server role err=%v, want role error", err)
	}
}

func TestValidateRejectsUnsupportedAPITokenRole(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Enabled = true
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.IdentityMode = "api_token"
	cfg.Server.APITokenEnv = "FAIRWAY_TEST_API_TOKEN"
	cfg.Server.APITokenRole = "root"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "api_token_role") || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("Validate unsupported api_token_role err=%v, want api_token_role unsupported error", err)
	}
}

func TestValidateRejectsAPITokenRoleOutsideAllowedRoles(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Enabled = true
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.IdentityMode = "api_token"
	cfg.Server.APITokenEnv = "FAIRWAY_TEST_API_TOKEN"
	cfg.Server.AllowedRoles = []string{"viewer"}
	cfg.Server.APITokenRole = "admin"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "api_token_role") || !strings.Contains(err.Error(), "allowed_roles") {
		t.Fatalf("Validate api_token_role outside allowed_roles err=%v, want allowed_roles membership error", err)
	}
}

func TestValidateSharedTeamServerWritePilotMode(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Enabled = true
	cfg.Server.Mode = "api-write-pilot"
	cfg.Server.ReadOnly = false
	cfg.Server.WriteEnabled = true
	cfg.Server.IdentityMode = "api_token"
	cfg.Server.APITokenEnv = "FAIRWAY_TEST_API_TOKEN"
	cfg.Server.AllowedRoles = []string{"operator"}
	cfg.Server.APITokenRole = "operator"
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate api-write-pilot error = %v", err)
	}
}

func TestValidateRejectsWritePilotWithoutWriteCapableTokenRole(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Enabled = true
	cfg.Server.Mode = "api-write-pilot"
	cfg.Server.ReadOnly = false
	cfg.Server.WriteEnabled = true
	cfg.Server.IdentityMode = "api_token"
	cfg.Server.APITokenEnv = "FAIRWAY_TEST_API_TOKEN"
	cfg.Server.AllowedRoles = []string{"viewer"}
	cfg.Server.APITokenRole = "viewer"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "api_token_role") || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("Validate write pilot viewer role err=%v, want append-only role error", err)
	}
}

func TestValidateRejectsWritePilotWithoutAPITokenIdentity(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Server.Enabled = true
	cfg.Server.Mode = "api-write-pilot"
	cfg.Server.ReadOnly = false
	cfg.Server.WriteEnabled = true
	cfg.Server.IdentityMode = "trusted_proxy_read_only"
	cfg.Server.TrustedProxyVerified = true
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "api_token") || !strings.Contains(err.Error(), "sovereign_signed") {
		t.Fatalf("Validate write pilot identity err=%v, want supported identity error", err)
	}
}

func TestValidateSovereignSignedIdentityProfile(t *testing.T) {
	cfg := sovereignSignedTestConfig(t, true)
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate sovereign signed identity error = %v", err)
	}

	readOnly := sovereignSignedTestConfig(t, false)
	readOnly.Server.AllowedRoles = []string{"viewer"}
	if err := Validate(readOnly); err != nil {
		t.Fatalf("Validate sovereign read-only identity error = %v", err)
	}
}

func TestValidateSovereignIdentityFailsClosed(t *testing.T) {
	tests := []struct {
		name  string
		apply func(*Config)
		want  string
	}{
		{name: "missing key env", apply: func(c *Config) { c.Server.SovereignPublicKeyEnv = "MISSING_SOVEREIGN_KEY" }, want: "not set"},
		{name: "missing issuer", apply: func(c *Config) { c.Server.SovereignIssuer = "" }, want: "issuer"},
		{name: "relative revocations", apply: func(c *Config) { c.Server.SovereignRevocationFile = "revocations.json" }, want: "absolute"},
		{name: "missing authorizer", apply: func(c *Config) { c.Server.AllowedRoles = []string{"operator", "reviewer:security"} }, want: "authorizer"},
		{name: "missing status dual control", apply: func(c *Config) { c.Server.SovereignDualControl = []string{"record:review"} }, want: "set:status"},
		{name: "unsupported dual control", apply: func(c *Config) { c.Server.SovereignDualControl = []string{"set:status", "record:review", "deploy"} }, want: "unsupported command"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := sovereignSignedTestConfig(t, true)
			tc.apply(&cfg)
			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestValidateSovereignOfflineServerRejectsIdentityFallback(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Runtime.Profile = RuntimeProfileSovereignOffline
	cfg.Server.Enabled = true
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.IdentityMode = "api_token"
	cfg.Server.APITokenEnv = "FAIRWAY_TEST_API_TOKEN"
	t.Setenv("FAIRWAY_TEST_API_TOKEN", "secret")
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "sovereign_signed") {
		t.Fatalf("Validate sovereign fallback error = %v", err)
	}
}

func sovereignSignedTestConfig(t *testing.T, write bool) Config {
	t.Helper()
	public, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	keyEnv := "FAIRWAY_TEST_SOVEREIGN_PUBLIC_KEY_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t.Setenv(keyEnv, base64.StdEncoding.EncodeToString(public))
	revocations := filepath.Join(t.TempDir(), "revocations.json")
	if err := os.WriteFile(revocations, []byte(`{"schema":"fairway.sovereign-revocations.v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Defaults(t.TempDir())
	cfg.Runtime.Profile = RuntimeProfileSovereignOffline
	cfg.Server.Enabled = true
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.IdentityMode = "sovereign_signed"
	cfg.Server.AllowedRoles = []string{"viewer"}
	cfg.Server.SovereignPublicKeyEnv = keyEnv
	cfg.Server.SovereignKeyID = "customer-key-1"
	cfg.Server.SovereignIssuer = "https://identity.example.test"
	cfg.Server.SovereignAudience = "fairway"
	cfg.Server.SovereignRevocationFile = revocations
	if write {
		cfg.Server.Mode = "api-write-pilot"
		cfg.Server.ReadOnly = false
		cfg.Server.WriteEnabled = true
		cfg.Server.AllowedRoles = []string{"viewer", "operator", "reviewer:security", "authorizer"}
	}
	return cfg
}

func TestWorktreePathUsesTemplate(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	got := WorktreePath(cfg, "/tmp/repo", Role{Name: "backend"})
	want := filepath.Join("/tmp", "worktrees", "repo-backend")
	if got != want {
		t.Fatalf("path=%q, want %q", got, want)
	}
}

func TestValidate_RejectsDefaultKindOutsideAllowed(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.TaskKinds.Allowed = []string{"epic", "story"}
	cfg.TaskKinds.Default = "task"
	if err := Validate(cfg); err == nil {
		t.Fatal("expected task kind validation error")
	}
}

func TestValidate_RejectsDefaultPriorityOutsideLevels(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	defaultPriority := 2
	cfg.TaskPriorities.Default = &defaultPriority
	cfg.TaskPriorities.Levels = []PriorityLevel{{Rank: 1, Label: "P1"}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected priority validation error")
	}
}

func TestValidate_RejectsReviewRouteOutsideRoles(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.Roles = []Role{{Name: "backend"}}
	cfg.ReviewRoutes = []ReviewRoute{{Match: "**", Reviewer: "arch"}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected review route validation error")
	}
}

func TestValidateReviewDomainAliases(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.Roles = []Role{{Name: "arch"}}
	cfg.ReviewDomainAliases = map[string]string{"security": "arch"}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	for name, aliases := range map[string]map[string]string{
		"unknown role": {"security": "security-reviewer"},
		"self alias":   {"arch": "arch"},
		"empty role":   {"security": ""},
	} {
		t.Run(name, func(t *testing.T) {
			invalid := Defaults("/tmp/repo")
			invalid.Roles = []Role{{Name: "arch"}}
			invalid.ReviewDomainAliases = aliases
			if err := Validate(invalid); err == nil {
				t.Fatal("expected review domain alias validation error")
			}
		})
	}
}

func TestValidate_AcceptsWorkstreamProfilesAndPacketTemplates(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.WorkstreamProfiles = []WorkstreamProfile{{
		Name:            "platform-foundation",
		TaskKinds:       []string{"architecture-map", "boundary-guard"},
		DashboardGroups: []string{"architecture maps", "guards"},
		ReviewDomains:   []string{"architecture", "security"},
		RouteSamples:    []string{"doc/api/openapi.yaml"},
		Gates: []WorkstreamProfileGate{{
			Name:                  "security-review",
			Group:                 "security gates",
			Mode:                  "advisory",
			TaskKinds:             []string{"boundary-guard"},
			EvidenceType:          "security-review",
			RequiredEvidenceCount: 1,
			AcceptedResults:       []string{"pass", "partial"},
			ArtifactRequired:      true,
			ExpiresAfter:          "168h",
		}},
	}}
	cfg.PacketTemplates = []PacketTemplate{{
		Profiles:       []string{"platform-foundation"},
		Name:           "architecture-map",
		RequiredFields: []string{"scope", "current_owner"},
		OptionalFields: []string{"source_doc"},
	}}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidate_RejectsGateGroupWhitespace(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.WorkstreamProfiles = []WorkstreamProfile{{
		Name: "release-readiness",
		Gates: []WorkstreamProfileGate{{
			Name:  "uat-evidence",
			Group: " release evidence ",
			Mode:  "blocking",
		}},
	}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected gate group whitespace validation error")
	}
}

func TestValidate_RejectsProfileTaskKindOutsideAllowed(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.TaskKinds.Allowed = []string{"task", "bug"}
	cfg.WorkstreamProfiles = []WorkstreamProfile{{
		Name:      "platform-foundation",
		TaskKinds: []string{"architecture-map"},
	}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected profile task kind validation error")
	}
}

func TestValidate_RejectsProfileGateTaskKindOutsideAllowed(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.TaskKinds.Allowed = []string{"task", "architecture-map"}
	cfg.WorkstreamProfiles = []WorkstreamProfile{{
		Name:      "platform-foundation",
		TaskKinds: []string{"architecture-map"},
		Gates: []WorkstreamProfileGate{{
			Name:      "boundary-guard-report",
			Mode:      "advisory",
			TaskKinds: []string{"boundary-guard"},
		}},
	}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected profile gate task kind validation error")
	}
}

func TestValidate_RejectsDuplicateWorkstreamProfile(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.WorkstreamProfiles = []WorkstreamProfile{{Name: "platform-foundation"}, {Name: "platform-foundation"}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected duplicate workstream profile validation error")
	}
}

func TestValidate_RejectsPacketTemplateUnknownProfile(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.WorkstreamProfiles = []WorkstreamProfile{{Name: "platform-foundation"}}
	cfg.PacketTemplates = []PacketTemplate{{
		Profiles:       []string{"release-readiness"},
		Name:           "architecture-map",
		RequiredFields: []string{"scope"},
	}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected packet template profile validation error")
	}
}

func TestValidate_RejectsInvalidGateEvidenceConfig(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.WorkstreamProfiles = []WorkstreamProfile{{
		Name: "release-readiness",
		Gates: []WorkstreamProfileGate{{
			Name:            "uat-evidence",
			Mode:            "blocking",
			AcceptedResults: []string{"green"},
		}},
	}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected gate evidence validation error")
	}
}

func TestValidate_RejectsPacketTemplateFieldOverlap(t *testing.T) {
	cfg := Defaults("/tmp/repo")
	cfg.PacketTemplates = []PacketTemplate{{
		Name:           "architecture-map",
		RequiredFields: []string{"scope"},
		OptionalFields: []string{"scope"},
	}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected packet template field overlap validation error")
	}
}

func TestConfigDecode_WorkstreamProfileNestedGates(t *testing.T) {
	var cfg Config
	if _, err := toml.Decode(`
[fairway]
project_name = "demo"
db_path = ".fairway/state.db"
task_id_pattern = "^[A-Z]+-[0-9]+$"

[worktrees]
root = "../worktrees"
naming = "{repo}-{role}"
review_branch_naming = "review/{role}"

[sessions]
default_backend = "shell"

[states]
allowed = ["todo", "done"]
terminal = ["done"]

[task_kinds]
default = "task"

[[workstream_profiles]]
name = "release-readiness"
route_samples = ["cmd/api/main.go"]

[[workstream_profiles.tag_groups]]
name = "release environments"
tags = ["environment:staging", "environment:cloudflare"]
description = "release target environments"

[[workstream_profiles.gates]]
name = "release-owner-approval"
group = "approval gates"
mode = "blocking"
evidence_type = "approval"
required_evidence_count = 1
accepted_results = ["pass"]
artifact_required = true
owner_signoff_required = true
expires_after = "720h"

[[packet_templates]]
profiles = ["release-readiness"]
name = "release-risk"
required_fields = ["risk", "owner"]
`, &cfg); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := cfg.WorkstreamProfiles[0].Gates[0].Name; got != "release-owner-approval" {
		t.Fatalf("gate name = %q", got)
	}
	if got := cfg.WorkstreamProfiles[0].Gates[0].Group; got != "approval gates" {
		t.Fatalf("gate group = %q", got)
	}
	if !cfg.WorkstreamProfiles[0].Gates[0].ArtifactRequired {
		t.Fatal("expected artifact_required to decode")
	}
	if got := cfg.WorkstreamProfiles[0].TagGroups[0].Tags[1]; got != "environment:cloudflare" {
		t.Fatalf("tag group tag = %q", got)
	}
	if got := cfg.PacketTemplates[0].Profiles[0]; got != "release-readiness" {
		t.Fatalf("packet template profile = %q", got)
	}
}
