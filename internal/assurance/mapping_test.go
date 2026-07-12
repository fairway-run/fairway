package assurance

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStarterProfileUnresolvedFactsRemainGaps(t *testing.T) {
	profile, err := LoadFile(filepath.Join("..", "..", "examples", "assurance-profiles", "nist-ssdf-1.1-starter.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	mapped := MapEvidence(profile, Sources{
		Task:    TaskContext{Project: "demo", TaskID: "T-1", Status: "done", UpdatedAt: now.Format(time.RFC3339Nano)},
		Reviews: []SourceReview{{Domain: "security", Verdict: "changes", Reviewer: "independent", CreatedAt: now.Format(time.RFC3339Nano)}},
		Evidence: []SourceEvidence{
			{Result: "fail", ArtifactType: "vulnerability", CreatedAt: now.Format(time.RFC3339Nano)},
			{Result: "partial", ArtifactType: "evidence", CreatedAt: now.Format(time.RFC3339Nano)},
		},
		Decisions: []SourceDecision{{ID: 1, QualityState: "accepted", CreatedBy: "owner", CreatedAt: now.Format(time.RFC3339Nano)}},
	}, now)
	report := BuildReadiness(profile, "project", []EvidenceMap{mapped})
	for _, control := range report.Controls {
		if control.Status == "satisfied_by_recorded_evidence" {
			t.Fatalf("unresolved facts satisfied starter control %s: %+v", control.ControlID, report)
		}
	}
}

func TestMapEvidencePreservesBoundaries(t *testing.T) {
	profile := mappingProfile()
	profile.Applicability.TaskKinds = []string{"task"}
	profile.Controls = []Control{{ID: "C-1", Title: "Review", Objective: "Retain review evidence.", Responsibility: "product",
		AssessmentObjectives: []string{"Inspect review facts."}, Evidence: []EvidenceRequirement{{Class: "review", MinimumCount: 1, MaximumAge: "24h", AcceptedResults: []string{"approve"}}}}}
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	mapped := MapEvidence(profile, Sources{Task: TaskContext{Project: "demo", TaskID: "T-1", Kind: "task", Status: "done", UpdatedAt: now.Format(time.RFC3339Nano)},
		Reviews: []SourceReview{{Domain: "arch", Verdict: "approve", Reviewer: "reviewer", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)}}}, now)
	if !mapped.Applicable || mapped.Controls[0].State != "supported" {
		t.Fatalf("unexpected mapping: %+v", mapped)
	}
	if mapped.Facts[0].Reference == "" || mapped.AuthorityBoundary == "" {
		t.Fatalf("missing bounded metadata: %+v", mapped)
	}
	for _, fact := range mapped.Facts {
		if fact.ConfidenceBoundary == "" {
			t.Fatalf("missing confidence boundary: %+v", fact)
		}
	}
}

func TestMapEvidenceDoesNotUpgradeProblemFacts(t *testing.T) {
	profile := mappingProfile()
	profile.Controls = []Control{{ID: "C-1", Title: "Evidence", Objective: "Retain evidence.", Responsibility: "product",
		AssessmentObjectives: []string{"Inspect evidence facts."}, Evidence: []EvidenceRequirement{{Class: "ci", MinimumCount: 1, MaximumAge: "1h", AcceptedResults: []string{"pass"}}}}}
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	mapped := MapEvidence(profile, Sources{Task: TaskContext{Project: "demo", TaskID: "T-1", Status: "done", UpdatedAt: now.Format(time.RFC3339Nano)},
		Evidence: []SourceEvidence{{Result: "pass", ArtifactType: "test", CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano)}}}, now)
	if got := mapped.Controls[0].Requirements[0].State; got != "stale" {
		t.Fatalf("state=%s want stale", got)
	}
	if mapped.Controls[0].State == "supported" {
		t.Fatal("stale fact upgraded to supported")
	}

	mapped = MapEvidence(profile, Sources{Task: TaskContext{Project: "demo", TaskID: "T-1", Status: "done", UpdatedAt: now.Format(time.RFC3339Nano)},
		Evidence: []SourceEvidence{{Result: "pass", ArtifactType: "test", CreatedAt: now.Format(time.RFC3339Nano)}, {Result: "fail", ArtifactType: "test", CreatedAt: now.Format(time.RFC3339Nano)}}}, now)
	if got := mapped.Controls[0].Requirements[0].State; got != "conflicting" {
		t.Fatalf("state=%s want conflicting", got)
	}
}

func TestMapEvidenceFreshnessIsRequirementSpecific(t *testing.T) {
	profile := mappingProfile()
	profile.Controls = []Control{
		{ID: "STRICT", Title: "Strict", Objective: "Recent evidence.", Responsibility: "product", AssessmentObjectives: []string{"Inspect recent evidence."}, Evidence: []EvidenceRequirement{{Class: "ci", MinimumCount: 1, MaximumAge: "1h", AcceptedResults: []string{"pass"}}}},
		{ID: "LONG", Title: "Long", Objective: "Retained evidence.", Responsibility: "product", AssessmentObjectives: []string{"Inspect retained evidence."}, Evidence: []EvidenceRequirement{{Class: "ci", MinimumCount: 1, MaximumAge: "24h", AcceptedResults: []string{"pass"}}}},
	}
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	mapped := MapEvidence(profile, Sources{Task: TaskContext{Project: "demo", TaskID: "T-1", Status: "done", UpdatedAt: now.Format(time.RFC3339Nano)},
		Evidence: []SourceEvidence{{Result: "pass", ArtifactType: "test", CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano)}}}, now)
	if mapped.Controls[0].Requirements[0].State != "stale" || mapped.Controls[1].Requirements[0].State != "supported" {
		t.Fatalf("freshness collapsed across requirements: %+v", mapped.Controls)
	}
}

func TestMapEvidenceKeepsExternalAndSupersededFactsVisible(t *testing.T) {
	profile := mappingProfile()
	profile.Controls = []Control{{ID: "C-1", Title: "External", Objective: "Retain external evidence.", Responsibility: "external_assessor",
		AssessmentObjectives: []string{"Inspect external facts."}, Evidence: []EvidenceRequirement{{Class: "external_assessment", MinimumCount: 1, AcceptedResults: []string{"verified"}}}}}
	now := time.Now().UTC()
	mapped := MapEvidence(profile, Sources{Task: TaskContext{Project: "demo", TaskID: "T-1", Status: "done", UpdatedAt: now.Format(time.RFC3339Nano)},
		Evidence:  []SourceEvidence{{Result: "verified", ArtifactType: "external-assessment", CreatedAt: now.Format(time.RFC3339Nano)}},
		Decisions: []SourceDecision{{ID: 3, QualityState: "superseded", CreatedAt: now.Format(time.RFC3339Nano)}}}, now)
	var external, superseded bool
	for _, fact := range mapped.Facts {
		if fact.Class == "external_assessment" && fact.State == "external_assertion" && fact.ConfidenceBoundary == "external assertion; assessor validation required" {
			external = true
		}
		if fact.Class == "decision" && fact.State == "superseded" {
			superseded = true
		}
	}
	if !external || !superseded {
		t.Fatalf("external=%t superseded=%t facts=%+v", external, superseded, mapped.Facts)
	}
	if mapped.Controls[0].State == "supported" {
		t.Fatal("external assertion was upgraded to supported")
	}
}

func TestMapEvidenceExternalAssessmentRequirementCannotAutoSatisfy(t *testing.T) {
	profile := mappingProfile()
	profile.Controls[0].ExternalAssessmentRequired = true
	now := time.Now().UTC()
	mapped := MapEvidence(profile, Sources{Task: TaskContext{Project: "demo", TaskID: "T-1", Status: "done", UpdatedAt: now.Format(time.RFC3339Nano)},
		Evidence: []SourceEvidence{{Result: "pass", ArtifactType: "evidence", CreatedAt: now.Format(time.RFC3339Nano)}}}, now)
	if mapped.Controls[0].State != "not_supported" {
		t.Fatalf("external assessment requirement was auto-satisfied: %+v", mapped.Controls[0])
	}
	last := mapped.Controls[0].Requirements[len(mapped.Controls[0].Requirements)-1]
	if last.State != "external_assessment_required" {
		t.Fatalf("missing external assessment boundary: %+v", last)
	}
}

func TestMapEvidenceMarksOutOfScope(t *testing.T) {
	profile := mappingProfile()
	profile.Applicability.Tags = []string{"sovereign"}
	now := time.Now().UTC()
	mapped := MapEvidence(profile, Sources{Task: TaskContext{Project: "demo", TaskID: "T-1", Tags: []string{"general"}, Status: "done", UpdatedAt: now.Format(time.RFC3339Nano)}}, now)
	if mapped.Applicable || mapped.Controls[0].Requirements[0].State != "out_of_scope" {
		t.Fatalf("unexpected mapping: %+v", mapped)
	}
}

func mappingProfile() Profile {
	return Profile{
		Schema: ProfileSchema, ID: "mapping", Version: "v1", Title: "Mapping", Description: "Evidence mapping only.",
		Framework:     Framework{ID: "example", Title: "Example", Version: "v1", Source: "https://example.invalid/framework"},
		Applicability: Applicability{Description: "Example projects."}, Scope: Scope{Types: []string{"project"}},
		Controls: []Control{{ID: "EX-1", Title: "Evidence", Objective: "Retain evidence.", Responsibility: "product",
			AssessmentObjectives: []string{"Inspect evidence references."}, Evidence: []EvidenceRequirement{{Class: "evidence", MinimumCount: 1, AcceptedResults: []string{"pass"}}}}},
		ProhibitedClaims: []string{"certified", "compliant", "authorized"},
		Authority:        Authority{Mode: "evidence_only", ProhibitedActions: []string{"certify", "declare_compliance", "accept_risk", "approve", "mutate_workflow", "merge", "deploy", "release", "use_credentials", "change_public_exposure", "run_live_operation"}},
	}
}
