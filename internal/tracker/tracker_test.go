package tracker

import (
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/store"
)

func TestProviderRegistryIncludesPlanningTargets(t *testing.T) {
	providers := SupportedProviders()
	got := map[string]bool{}
	for _, provider := range providers {
		got[provider.Name] = true
		if len(provider.Operations) == 0 {
			t.Fatalf("provider %s has no operations", provider.Name)
		}
	}
	for _, want := range []string{"plane", "jira", "linear"} {
		if !got[want] {
			t.Fatalf("missing provider %s in %#v", want, providers)
		}
	}
}

func TestUnsupportedProviderErrors(t *testing.T) {
	if err := ValidateProvider("notion"); err == nil || !strings.Contains(err.Error(), "unsupported tracker provider") {
		t.Fatalf("ValidateProvider error=%v, want unsupported provider", err)
	}
	if _, err := DefaultMappings("notion"); err == nil || !strings.Contains(err.Error(), "unsupported tracker provider") {
		t.Fatalf("DefaultMappings error=%v, want unsupported provider", err)
	}
}

func TestBuildReconcileReportIsDryRunPlanningOnly(t *testing.T) {
	report := BuildReconcileReport([]store.TrackerLink{{TaskID: "T-001", Provider: "plane", ExternalID: "PLN-1"}}, true)
	if !report.DryRun || !report.PlanningOnly {
		t.Fatalf("report=%+v, want dry-run planning-only", report)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("actions=%d, want 1", len(report.Actions))
	}
	action := report.Actions[0]
	if action.Mutates != "none" || !action.DryRun || action.Provider != "plane" || action.TaskID != "T-001" {
		t.Fatalf("action=%+v", action)
	}
}
