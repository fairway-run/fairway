package dashboard

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/controlanalytics"
)

func TestBuildControlViewDataFiltersCohortsAndPreservesFacts(t *testing.T) {
	report := controlanalytics.Report{
		Coverage: controlanalytics.Coverage{EligibleCommits: 10, CoveredCommits: 9, CommitCoverageRatio: 0.9},
		Controls: []controlanalytics.ControlResult{
			{ControlID: "review:backend", Family: "quality_gate", Profile: "standard", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, Classification: "discriminating", Applicable: 3, Eligible: 2, Observed: 1, Bypassed: 1},
			{ControlID: "review:security", Family: "security_invariant", Profile: "standard", RiskBand: "high", SizeBand: "large", HorizonDays: 14, Classification: "mandatory_invariant", Applicable: 1, RightCensored: 1},
			{ControlID: "review:backend", Family: "quality_gate", Profile: "standard", RiskBand: "medium", SizeBand: "small", HorizonDays: 30, Classification: "insufficient_sample", Applicable: 3, RightCensored: 3},
		},
		TaskFacts: []controlanalytics.TaskFact{
			{TaskID: "T-002", ControlID: "review:backend", Family: "quality_gate", Profile: "standard", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, ControlState: "bypassed"},
			{TaskID: "T-001", ControlID: "review:backend", Family: "quality_gate", Profile: "standard", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, ControlState: "observed", PromotionCommit: "1234567890abcdef"},
		},
	}
	data := buildControlViewData(report, ControlFilters{Profile: "standard", RiskBand: "medium", SizeBand: "small", Family: "quality_gate", Horizon: 14})
	if len(data.Rows) != 1 || data.Rows[0].Result.ControlID != "review:backend" {
		t.Fatalf("rows=%+v", data.Rows)
	}
	if len(data.Rows[0].Facts) != 2 || data.Rows[0].Facts[0].TaskID != "T-001" {
		t.Fatalf("facts=%+v", data.Rows[0].Facts)
	}
	if data.Summary.Controls != 1 || data.Summary.Cohorts != 1 || data.Summary.Eligible != 2 {
		t.Fatalf("summary=%+v", data.Summary)
	}
	if strings.Join(data.FilterOptions.Families, ",") != "quality_gate,security_invariant" {
		t.Fatalf("families=%v", data.FilterOptions.Families)
	}
}

func TestControlFiltersUseBoundedChoices(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/controls?window=999&horizon=7&profile=standard&risk=high&size=large&family=security_invariant", nil)
	filters := controlFiltersFromRequest(req)
	if filters.WindowDays != 30 || filters.Horizon != 7 || filters.Profile != "standard" || filters.RiskBand != "high" || filters.SizeBand != "large" || filters.Family != "security_invariant" {
		t.Fatalf("filters=%+v", filters)
	}
}

func TestControlsTemplateRendersCoverageAndInspectableFacts(t *testing.T) {
	report := controlanalytics.Report{
		Advisory:              true,
		AuthorityBoundary:     "Fairway records and Git remain authoritative.",
		AsOf:                  "2026-08-02T12:00:00Z",
		ConfigurationRevision: "v1",
		ConfigurationDigest:   "abcdef1234567890",
		AnalyzedTip:           "fedcba0987654321",
		Exclusions:            []config.ControlPathExclusion{{Pattern: "vendor/**", Category: "generated", Rationale: "third-party code"}},
		Coverage: controlanalytics.Coverage{
			EligibleCommits: 10, CoveredCommits: 8, CommitCoverageRatio: 0.8,
			EligibleChangedFiles: 20, CoveredChangedFiles: 15, FileCoverageRatio: 0.75,
		},
		Controls: []controlanalytics.ControlResult{{
			ControlID: "review:backend", Family: "quality_gate", Profile: "standard", RiskBand: "medium", SizeBand: "small", HorizonDays: 14,
			Classification: "discriminating", Applicable: 2, Eligible: 2, Observed: 1, Bypassed: 1, ObservedOutcomeRate: 0, BypassedOutcomeRate: 1, OutcomeDelta: -1, TriggerYield: 0.5,
		}},
		TaskFacts: []controlanalytics.TaskFact{{
			TaskID: "T-001", ControlID: "review:backend", Profile: "standard", RiskBand: "medium", SizeBand: "small", HorizonDays: 14,
			ControlState: "observed", Triggered: true, Mature: true, OutcomeKnown: true, PromotionCommit: "1234567890abcdef",
			OutcomeFacts: []controlanalytics.OutcomeFact{{Kind: "incident", SourceRef: "incident:INC-7"}}, TouchCommits: []string{"beef123"}, EligibleFiles: []string{"internal/example.go"},
		}},
	}
	data := buildControlViewData(report, ControlFilters{WindowDays: 30, Horizon: 14})
	var output bytes.Buffer
	if err := controlsTemplate.ExecuteTemplate(&output, "layout", data); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	for _, want := range []string{"Control Effectiveness", "80.0%", "75.0%", "review:backend", "discriminating", `href="/tasks/T-001"`, "1234567890abcdef", "incident:INC-7", "beef123", "internal/example.go", "fedcba0987654321", "abcdef1234567890", "vendor/**", "third-party code"} {
		if !strings.Contains(body, want) {
			t.Fatalf("controls body missing %q:\n%s", want, body)
		}
	}
	if !strings.Contains(body, `class="active" href="/controls"`) {
		t.Fatalf("controls navigation is not active:\n%s", body)
	}
}
