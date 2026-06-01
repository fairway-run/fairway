package config

import (
	"path/filepath"
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

[[workstream_profiles.gates]]
name = "release-owner-approval"
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
	if !cfg.WorkstreamProfiles[0].Gates[0].ArtifactRequired {
		t.Fatal("expected artifact_required to decode")
	}
	if got := cfg.PacketTemplates[0].Profiles[0]; got != "release-readiness" {
		t.Fatalf("packet template profile = %q", got)
	}
}
