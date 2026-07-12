package config

import (
	"os"
	"strings"
	"testing"
)

func TestValidateSovereignOfflineRuntimeProfile(t *testing.T) {
	clearSovereignNetworkEnv(t)
	cfg := sovereignOfflineTestConfig(t)
	cfg.ProviderTargets = []ProviderTarget{{Domain: "ops", Provider: "shell", Type: "tmux", Target: "%1"}}
	cfg.AdvisoryAdapters = []AdvisoryAdapter{{Name: "local-model", Provider: "ollama", Type: "local_ollama", EndpointEnv: "LOCAL_MODEL_ENDPOINT"}}
	cfg.ExternalNotifiers = []ExternalNotifier{{Name: "local-log", Type: "log", Mode: "send", TargetEnv: "LOCAL_NOTIFY_PATH"}}
	t.Setenv("LOCAL_MODEL_ENDPOINT", "http://127.0.0.1:11434")
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() safe sovereign profile error = %v", err)
	}

	tests := []struct {
		name  string
		apply func(*Config)
		want  string
	}{
		{name: "dashboard remote", apply: func(c *Config) { c.Dashboard.Listen = "0.0.0.0:7878" }, want: "dashboard-listen"},
		{name: "dashboard proxy identity", apply: func(c *Config) { c.Dashboard.TrustedProxy = "cloudflare_access" }, want: "dashboard-identity"},
		{name: "server remote", apply: func(c *Config) {
			c.Server.Enabled = true
			c.Server.Mode = "read_only"
			c.Server.Listen = "10.0.0.8:7880"
		}, want: "server-listen"},
		{name: "server remote identity", apply: func(c *Config) {
			c.Server.Enabled = true
			c.Server.Mode = "read_only"
			c.Server.IdentityMode = "trusted_proxy_read_only"
			c.Server.TrustedProxyVerified = true
		}, want: "server-identity"},
		{name: "thread provider", apply: func(c *Config) {
			c.ProviderTargets = []ProviderTarget{{Domain: "ops", Provider: "codex", Type: "thread", Target: "thread-1"}}
		}, want: "provider-target"},
		{name: "remote adapter", apply: func(c *Config) {
			c.AdvisoryAdapters = []AdvisoryAdapter{{Name: "remote", Provider: "codex", Type: "codex"}}
		}, want: "advisory-adapter"},
		{name: "DNS local adapter", apply: func(c *Config) {
			t.Setenv("DNS_MODEL_ENDPOINT", "http://localhost:11434")
			c.AdvisoryAdapters = []AdvisoryAdapter{{Name: "dns", Provider: "ollama", Type: "local_ollama", EndpointEnv: "DNS_MODEL_ENDPOINT"}}
		}, want: "advisory-adapter"},
		{name: "webhook notifier", apply: func(c *Config) {
			c.ExternalNotifiers = []ExternalNotifier{{Name: "hook", Type: "webhook", Mode: "send", TargetEnv: "HOOK_URL"}}
		}, want: "external-notifier"},
		{name: "remote rule", apply: func(c *Config) {
			c.RuleSources = []RuleSource{{Name: "remote", Source: "github:org/rules", Mode: "advisory"}}
		}, want: "rule-source"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := sovereignOfflineTestConfig(t)
			tc.apply(&candidate)
			err := Validate(candidate)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestValidateRejectsUnknownRuntimeProfile(t *testing.T) {
	cfg := Defaults(t.TempDir())
	cfg.Runtime.Profile = "sovereign-connected"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("Validate() error = %v, want unsupported profile", err)
	}
}

func TestSovereignOfflineRejectsProxyAndTrackerEnvironmentWithoutLeakingValues(t *testing.T) {
	clearSovereignNetworkEnv(t)
	cfg := sovereignOfflineTestConfig(t)
	t.Setenv("HTTPS_PROXY", "http://user:SHOULD_NOT_RENDER@proxy.example:8080")
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "proxy-env:https_proxy") {
		t.Fatalf("proxy Validate() error = %v", err)
	}
	if strings.Contains(err.Error(), "SHOULD_NOT_RENDER") || strings.Contains(err.Error(), "proxy.example") {
		t.Fatalf("proxy error leaked value: %v", err)
	}

	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("PLANE_API_TOKEN", "SHOULD_NOT_RENDER")
	err = Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "tracker-env:plane_api_token") {
		t.Fatalf("tracker Validate() error = %v", err)
	}
	if strings.Contains(err.Error(), "SHOULD_NOT_RENDER") {
		t.Fatalf("tracker error leaked value: %v", err)
	}
}

func TestRuntimeNetworkDependenciesReportsDisabledRemoteEdges(t *testing.T) {
	clearSovereignNetworkEnv(t)
	cfg := sovereignOfflineTestConfig(t)
	cfg.AdvisoryAdapters = []AdvisoryAdapter{{Name: "future-codex", Provider: "codex", Type: "codex", Mode: "disabled"}}
	cfg.ExternalNotifiers = []ExternalNotifier{{Name: "future-hook", Type: "webhook", Mode: "disabled", TargetEnv: "HOOK_URL"}}
	cfg.RuleSources = []RuleSource{{Name: "future-rules", Source: "github:org/rules", Mode: "disabled", CommitSHA: strings.Repeat("a", 40), Checksum: "sha256:abc"}}
	if err := Validate(cfg); err != nil {
		t.Fatalf("disabled remote dependencies should be representable: %v", err)
	}
	rows := RuntimeNetworkDependencies(cfg, os.LookupEnv)
	for _, id := range []string{"advisory-adapter:future-codex", "external-notifier:future-hook", "rule-source:future-rules", "product-remote-assets", "product-telemetry", "product-update-check"} {
		found := false
		for _, row := range rows {
			if row.ID == id {
				found = true
				if row.Blocking || row.Status != "disabled" {
					t.Fatalf("row %+v, want disabled non-blocking", row)
				}
			}
		}
		if !found {
			t.Fatalf("missing dependency row %s", id)
		}
	}
}

func TestRuntimeNetworkDependenciesReportsSovereignSignedIdentityAsLocal(t *testing.T) {
	clearSovereignNetworkEnv(t)
	cfg := sovereignSignedTestConfig(t, false)
	rows := RuntimeNetworkDependencies(cfg, os.LookupEnv)
	for _, row := range rows {
		if row.ID != "server-identity" {
			continue
		}
		if row.Mode != "sovereign_signed" || row.Status != "local" || row.Blocking {
			t.Fatalf("server identity row = %+v, want local signed non-blocking identity", row)
		}
		return
	}
	t.Fatal("server-identity dependency row missing")
}

func TestIsLocalProvider(t *testing.T) {
	for _, tc := range []struct {
		provider string
		backend  string
		want     bool
	}{
		{"shell", "shell", true},
		{"ollama", "shell", true},
		{"", "tmux", true},
		{"codex", "shell", false},
		{"claude", "tmux", false},
		{"gemini", "provider-session", false},
	} {
		if got := IsLocalProvider(tc.provider, tc.backend); got != tc.want {
			t.Fatalf("IsLocalProvider(%q, %q) = %t, want %t", tc.provider, tc.backend, got, tc.want)
		}
	}
}

func sovereignOfflineTestConfig(t *testing.T) Config {
	t.Helper()
	cfg := Defaults(t.TempDir())
	cfg.Runtime.Profile = RuntimeProfileSovereignOffline
	cfg.Dashboard.AutoOpen = false
	return cfg
}

func clearSovereignNetworkEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy", "PLANE_BASE_URL", "PLANE_WORKSPACE", "PLANE_PROJECT", "PLANE_API_TOKEN"} {
		value, set := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if set {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}
