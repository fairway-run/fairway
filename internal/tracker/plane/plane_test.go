package plane

import (
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/store"
)

func TestExportIssueMapsFairwayTaskToPlanePayload(t *testing.T) {
	priority := 1
	task := store.Task{
		Definition: store.TaskDefinition{
			ID:               "T-001",
			Title:            "Build tracker spike",
			Role:             "backend",
			Kind:             "task",
			OwningDomain:     "tracker",
			RiskLevel:        "medium",
			ReviewDomains:    []string{"arch", "governance"},
			AcceptanceChecks: []string{"dry-run payload renders"},
			Priority:         &priority,
		},
		Status: "in_progress",
		Owner:  "backend",
	}
	payload := ExportIssue(task, []store.Evidence{{Result: "pass", ArtifactType: "test", ArtifactPath: "dist/test.log"}}, []store.Review{{Reviewer: "arch", Verdict: "approve"}}, Config{Workspace: "fairway-eval", Project: "FWPLANE"})
	if payload.Provider != "plane" || payload.SourceTaskID != "T-001" || !payload.PlanningOnly {
		t.Fatalf("payload=%+v", payload)
	}
	if payload.Name != "Build tracker spike" || payload.ExternalRef != "T-001" || payload.CustomFields["fairway_status"] != "in_progress" {
		t.Fatalf("payload mapping failed: %+v", payload)
	}
	for _, want := range []string{"role:backend", "domain:tracker", "kind:task", "review:arch"} {
		if !contains(payload.Labels, want) {
			t.Fatalf("labels=%v missing %s", payload.Labels, want)
		}
	}
	if !strings.Contains(payload.Description, "Boundary: Plane is a planning mirror") {
		t.Fatalf("description missing boundary:\n%s", payload.Description)
	}
}

func TestImportFixtureProducesDryRunTaskDefinitions(t *testing.T) {
	fixture := Fixture{}
	fixture.Workspace.Slug = "fairway-eval"
	fixture.Project.Identifier = "FWPLANE"
	fixture.Issues = []FixtureIssue{{
		FairwayID:   "FW-122",
		Title:       "Add Plane adapter spike",
		Role:        "backend",
		Kind:        "task",
		Domain:      "tracker",
		Labels:      []string{"review:arch", "review:backend"},
		Description: "Planning mirror issue",
		Acceptance:  []string{"preview maps"},
	}}
	preview, err := ImportFixture(fixture, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || !preview.PlanningOnly || len(preview.Tasks) != 1 {
		t.Fatalf("preview=%+v", preview)
	}
	task := preview.Tasks[0]
	if task.ID != "FW-122" || task.OwningDomain != "tracker" || task.Profile != "tracker-plane" {
		t.Fatalf("task=%+v", task)
	}
	for _, want := range []string{"arch", "backend"} {
		if !contains(task.ReviewDomains, want) {
			t.Fatalf("review domains=%v missing %s", task.ReviewDomains, want)
		}
	}
}

func TestConfigFromEnvRequiresNonSecretConfig(t *testing.T) {
	t.Setenv(EnvBaseURL, "")
	t.Setenv(EnvWorkspace, "")
	t.Setenv(EnvProject, "")
	t.Setenv(EnvAPIToken, "secret")
	if _, err := ConfigFromEnv(); err == nil || !strings.Contains(err.Error(), EnvBaseURL) {
		t.Fatalf("ConfigFromEnv error=%v, want missing config", err)
	}
	t.Setenv(EnvBaseURL, "http://localhost:8088")
	t.Setenv(EnvWorkspace, "fairway-eval")
	t.Setenv(EnvProject, "FWPLANE")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TokenPresent || cfg.TokenSource != EnvAPIToken {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestApplyUnsupported(t *testing.T) {
	if err := ApplyUnsupported(); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("ApplyUnsupported error=%v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
