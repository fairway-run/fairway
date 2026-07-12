package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"
)

const (
	RuntimeProfileStandard         = "standard"
	RuntimeProfileSovereignOffline = "sovereign-offline"
)

// NetworkDependency is a redacted configuration-level inventory row. Detail
// names settings and environment-variable keys, never their secret-bearing
// values.
type NetworkDependency struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Mode     string `json:"mode"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Blocking bool   `json:"blocking"`
}

func RuntimeProfile(cfg Config) string {
	profile := strings.TrimSpace(cfg.Runtime.Profile)
	if profile == "" {
		return RuntimeProfileStandard
	}
	return profile
}

func IsSovereignOffline(cfg Config) bool {
	return RuntimeProfile(cfg) == RuntimeProfileSovereignOffline
}

func validateRuntimeProfile(cfg Config) error {
	switch RuntimeProfile(cfg) {
	case RuntimeProfileStandard:
		return nil
	case RuntimeProfileSovereignOffline:
		dependencies := RuntimeNetworkDependencies(cfg, os.LookupEnv)
		var blockers []string
		for _, dependency := range dependencies {
			if dependency.Blocking {
				blockers = append(blockers, dependency.ID+": "+dependency.Detail)
			}
		}
		if len(blockers) > 0 {
			return fmt.Errorf("[runtime] sovereign-offline network boundary rejected configured dependencies: %s", strings.Join(blockers, "; "))
		}
		return nil
	default:
		return fmt.Errorf("[runtime] profile %q is unsupported", cfg.Runtime.Profile)
	}
}

// RuntimeNetworkDependencies returns every configured Fairway network or
// identity edge relevant to the selected runtime profile. lookupEnv is
// injected so tests and doctor output can prove the environment boundary
// without exposing values.
func RuntimeNetworkDependencies(cfg Config, lookupEnv func(string) (string, bool)) []NetworkDependency {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	profile := RuntimeProfile(cfg)
	rows := []NetworkDependency{
		dependency("dashboard-listen", "listener", "dashboard", "active", localListenStatus(cfg.Dashboard.Listen), "listen="+strings.TrimSpace(cfg.Dashboard.Listen), profile == RuntimeProfileSovereignOffline && !IsLoopbackListen(cfg.Dashboard.Listen)),
		dependency("product-remote-assets", "asset", "dashboard assets", "embedded", "disabled", "all dashboard CSS, JavaScript, templates, and images are embedded", false),
		dependency("product-telemetry", "telemetry", "product telemetry", "not_implemented", "disabled", "Fairway has no product telemetry exporter", false),
		dependency("product-update-check", "update", "automatic update check", "not_implemented", "disabled", "Fairway performs no automatic update network check", false),
	}
	trustedProxy := firstNonEmpty(strings.TrimSpace(cfg.Dashboard.TrustedProxy), "none")
	rows = append(rows, dependency("dashboard-identity", "identity", "dashboard trusted proxy", trustedProxy, inactiveOrRemoteStatus(trustedProxy == "none"), "trusted_proxy="+trustedProxy, profile == RuntimeProfileSovereignOffline && trustedProxy != "none"))

	serverMode := firstNonEmpty(strings.TrimSpace(cfg.Server.Mode), "disabled")
	serverActive := cfg.Server.Enabled || serverMode != "disabled"
	rows = append(rows, dependency("server-listen", "listener", "shared-team server", serverMode, inactiveOrLocalStatus(!serverActive, IsLoopbackListen(cfg.Server.Listen)), "listen="+strings.TrimSpace(cfg.Server.Listen), profile == RuntimeProfileSovereignOffline && serverActive && !IsLoopbackListen(cfg.Server.Listen)))
	identityMode := firstNonEmpty(strings.TrimSpace(cfg.Server.IdentityMode), "no_edge_local")
	remoteIdentity := serverActive && identityMode == "trusted_proxy_read_only"
	rows = append(rows, dependency("server-identity", "identity", "shared-team server identity", identityMode, inactiveOrRemoteStatus(!remoteIdentity), "identity_mode="+identityMode, profile == RuntimeProfileSovereignOffline && remoteIdentity))

	for _, source := range cfg.RuleSources {
		mode := firstNonEmpty(strings.TrimSpace(source.Mode), "advisory")
		scheme, _, _ := strings.Cut(strings.TrimSpace(source.Source), ":")
		remote := mode != "disabled" && scheme != "path" && scheme != "file"
		rows = append(rows, dependency("rule-source:"+source.Name, "rule_source", source.Name, mode, inactiveOrRemoteStatus(!remote), "source_scheme="+firstNonEmpty(scheme, "unknown"), profile == RuntimeProfileSovereignOffline && remote))
	}

	for _, target := range cfg.ProviderTargets {
		targetType := firstNonEmpty(strings.TrimSpace(target.Type), "generic")
		local := targetType == "tmux"
		rows = append(rows, dependency("provider-target:"+target.Domain+":"+target.Provider, "provider", firstNonEmpty(target.Domain, target.Provider), targetType, localOrRemoteStatus(local), "target_type="+targetType, profile == RuntimeProfileSovereignOffline && !local))
	}

	for _, adapter := range cfg.AdvisoryAdapters {
		mode := firstNonEmpty(strings.TrimSpace(adapter.Mode), "advisory")
		adapterType := firstNonEmpty(strings.TrimSpace(adapter.Type), "noop")
		active := mode != "disabled"
		local := adapterType == "noop" || adapterType == "rules-only"
		detail := "type=" + adapterType
		if active && (adapterType == "local_ollama" || adapterType == "local_llamacpp") {
			endpoint, set := lookupEnv(strings.TrimSpace(adapter.EndpointEnv))
			local = set && isNumericLoopbackHTTPBase(endpoint)
			detail += " endpoint_env=" + firstNonEmpty(strings.TrimSpace(adapter.EndpointEnv), "unset")
		}
		status := "disabled"
		if active {
			status = localOrRemoteStatus(local)
		}
		rows = append(rows, dependency("advisory-adapter:"+adapter.Name, "provider", adapter.Name, mode, status, detail, profile == RuntimeProfileSovereignOffline && active && !local))
	}

	for _, notifier := range cfg.ExternalNotifiers {
		mode := firstNonEmpty(strings.TrimSpace(notifier.Mode), "dry_run")
		notifierType := firstNonEmpty(strings.TrimSpace(notifier.Type), "noop")
		active := mode == "send"
		local := notifierType == "noop" || notifierType == "log"
		status := "disabled"
		if mode == "dry_run" {
			status = "dry_run"
		} else if active {
			status = localOrRemoteStatus(local)
		}
		rows = append(rows, dependency("external-notifier:"+notifier.Name, "notifier", notifier.Name, mode, status, "type="+notifierType+" target_env="+firstNonEmpty(strings.TrimSpace(notifier.TargetEnv), "unset"), profile == RuntimeProfileSovereignOffline && active && !local))
	}

	for _, envName := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		if value, ok := lookupEnv(envName); ok && strings.TrimSpace(value) != "" {
			rows = append(rows, dependency("proxy-env:"+strings.ToLower(envName), "proxy", envName, "configured", "remote", "environment variable is set; value redacted", profile == RuntimeProfileSovereignOffline))
		}
	}
	for _, envName := range []string{"PLANE_BASE_URL", "PLANE_WORKSPACE", "PLANE_PROJECT", "PLANE_API_TOKEN"} {
		if value, ok := lookupEnv(envName); ok && strings.TrimSpace(value) != "" {
			rows = append(rows, dependency("tracker-env:"+strings.ToLower(envName), "tracker", "Plane", "configured", "remote", "environment variable "+envName+" is set; value redacted", profile == RuntimeProfileSovereignOffline))
		}
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func IsLoopbackListen(address string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func IsLocalProvider(provider, backend string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	backend = strings.ToLower(strings.TrimSpace(backend))
	if provider == "" {
		return backend == "" || backend == "shell" || backend == "tmux" || backend == "zellij" || backend == "local"
	}
	switch provider {
	case "shell", "local", "tmux", "zellij", "noop", "rules-only", "ollama", "llamacpp":
		return true
	default:
		return false
	}
}

func isNumericLoopbackHTTPBase(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	ip := net.ParseIP(parsed.Hostname())
	return ip != nil && ip.IsLoopback()
}

func dependency(id, kind, name, mode, status, detail string, blocking bool) NetworkDependency {
	return NetworkDependency{ID: id, Kind: kind, Name: name, Mode: mode, Status: status, Detail: detail, Blocking: blocking}
}

func localListenStatus(address string) string {
	return localOrRemoteStatus(IsLoopbackListen(address))
}

func localOrRemoteStatus(local bool) string {
	if local {
		return "local"
	}
	return "remote"
}

func inactiveOrRemoteStatus(inactive bool) string {
	if inactive {
		return "disabled"
	}
	return "remote"
}

func inactiveOrLocalStatus(inactive, local bool) string {
	if inactive {
		return "disabled"
	}
	return localOrRemoteStatus(local)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
