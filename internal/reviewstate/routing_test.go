package reviewstate

import (
	"testing"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestReviewRoutingCoverageExplainsResolutionSources(t *testing.T) {
	tasks := []store.Task{
		{Definition: store.TaskDefinition{ID: "T-ROLE", ReviewDomains: []string{"backend"}}, Status: "todo"},
		{Definition: store.TaskDefinition{ID: "T-ALIAS", ReviewDomains: []string{"compliance"}}, Status: "todo"},
		{Definition: store.TaskDefinition{ID: "T-ROUTE", ReviewDomains: []string{"governance"}}, Status: "in_progress"},
		{Definition: store.TaskDefinition{ID: "T-TARGET", ReviewDomains: []string{"security"}}, Status: "blocked"},
		{Definition: store.TaskDefinition{ID: "T-MISSING", ReviewDomains: []string{"product"}}, Status: "todo"},
		{Definition: store.TaskDefinition{ID: "T-DONE", ReviewDomains: []string{"ignored"}}, Status: "done"},
	}
	report := ReviewRoutingCoverage(tasks, ReviewWaitOptions{
		Roles:           []config.Role{{Name: "backend", Provider: "codex"}, {Name: "arch", Provider: "codex"}},
		DomainAliases:   map[string]string{"compliance": "arch"},
		ReviewRoutes:    []config.ReviewRoute{{Match: "docs/**", Reviewer: "governance"}},
		ProviderTargets: []config.ProviderTarget{{Domain: "security", Provider: "codex", Target: "thread-security", Type: "thread"}},
		Terminal:        []string{"done"},
	})
	if report.OK {
		t.Fatal("coverage unexpectedly passed with missing product mapping")
	}
	if report.Summary.Domains != 5 || report.Summary.ConfiguredRoles != 1 || report.Summary.ConfiguredAliases != 1 || report.Summary.ReviewRoutes != 1 || report.Summary.ProviderTargets != 1 || report.Summary.MissingMappings != 1 {
		t.Fatalf("summary=%+v", report.Summary)
	}
	want := map[string]string{
		"backend":    "configured_role",
		"compliance": "configured_alias",
		"governance": "review_route",
		"product":    "missing",
		"security":   "provider_target",
	}
	for _, row := range report.Rows {
		if row.Resolution != want[row.Domain] {
			t.Fatalf("domain %s resolution=%s want=%s", row.Domain, row.Resolution, want[row.Domain])
		}
		delete(want, row.Domain)
	}
	if len(want) != 0 {
		t.Fatalf("missing rows: %+v", want)
	}
}
