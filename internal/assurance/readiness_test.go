package assurance

import "testing"

func TestBuildReadinessUsesBoundedStatusVocabulary(t *testing.T) {
	profile := mappingProfile()
	mapped := EvidenceMap{
		TaskID: "T-1", Applicable: true, EvaluatedAt: "2026-07-12T12:00:00Z",
		Facts: []EvidenceFact{{Reference: "evidence:T-1:1", Class: "evidence", Result: "pass", Timestamp: "2026-07-12T11:00:00Z", State: "current"}},
		Controls: []ControlMapping{{
			ControlID: "EX-1", State: "supported",
			Requirements: []RequirementMapping{{Class: "evidence", MinimumCount: 1, Matched: 1, State: "supported", FactReferences: []string{"evidence:T-1:1"}}},
		}},
	}
	report := BuildReadiness(profile, "task_set", []EvidenceMap{mapped})
	if got := report.Controls[0].Status; got != "satisfied_by_recorded_evidence" {
		t.Fatalf("status=%s", got)
	}
	if len(report.Gaps) != 0 || report.AuthorityBoundary == "" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestBuildReadinessReportsStaleConflictAndMissingGaps(t *testing.T) {
	profile := mappingProfile()
	for _, state := range []string{"stale", "conflicting", "missing"} {
		profile.Controls[0].Evidence[0].MaximumAge = ""
		var facts []EvidenceFact
		switch state {
		case "stale":
			profile.Controls[0].Evidence[0].MaximumAge = "1h"
			facts = []EvidenceFact{{Reference: "evidence:T-1:1", Class: "evidence", Result: "pass", Timestamp: "2026-07-12T10:00:00Z", State: "current"}}
		case "conflicting":
			facts = []EvidenceFact{
				{Reference: "evidence:T-1:1", Class: "evidence", Result: "pass", Timestamp: "2026-07-12T11:00:00Z", State: "current"},
				{Reference: "evidence:T-1:2", Class: "evidence", Result: "fail", Timestamp: "2026-07-12T11:00:00Z", State: "current"},
			}
		}
		mapped := EvidenceMap{
			TaskID: "T-1", Applicable: true, EvaluatedAt: "2026-07-12T12:00:00Z",
			Facts:    facts,
			Controls: []ControlMapping{{ControlID: "EX-1", State: "not_supported", Requirements: []RequirementMapping{{Class: "evidence", MinimumCount: 1, State: state}}}},
		}
		report := BuildReadiness(profile, "task_set", []EvidenceMap{mapped})
		if report.Controls[0].Status != state || len(report.Gaps) != 1 || report.Gaps[0].NextEvidenceAction == "" {
			t.Fatalf("state=%s report=%+v", state, report)
		}
	}
}

func TestBuildReadinessPreservesCustomerAndAssessorBoundaries(t *testing.T) {
	profile := mappingProfile()
	profile.Controls = []Control{
		{ID: "CUSTOMER", Title: "Customer", Objective: "Customer proof.", Responsibility: "customer", AssessmentObjectives: []string{"Inspect."}, Evidence: []EvidenceRequirement{{Class: "configuration", MinimumCount: 1, AcceptedResults: []string{"verified"}}}},
		{ID: "ASSESS", Title: "Assess", Objective: "External proof.", Responsibility: "external_assessor", AssessmentObjectives: []string{"Inspect."}, ExternalAssessmentRequired: true, Evidence: []EvidenceRequirement{{Class: "external_assessment", MinimumCount: 1, AcceptedResults: []string{"verified"}}}},
	}
	report := BuildReadiness(profile, "project", []EvidenceMap{{TaskID: "T-1", Applicable: true, EvaluatedAt: "2026-07-12T12:00:00Z", Controls: []ControlMapping{{}, {}}}})
	if report.Controls[0].Status != "customer_responsibility" || report.Controls[1].Status != "external_assessment_required" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Gaps) != 2 {
		t.Fatalf("gaps=%+v", report.Gaps)
	}
}

func TestBuildReadinessOutOfScopePrecedesResponsibilityGaps(t *testing.T) {
	profile := mappingProfile()
	profile.Controls = []Control{
		{ID: "CUSTOMER", Title: "Customer", Objective: "Customer proof.", Responsibility: "customer", AssessmentObjectives: []string{"Inspect."}, Evidence: []EvidenceRequirement{{Class: "configuration", MinimumCount: 1, AcceptedResults: []string{"verified"}}}},
		{ID: "ASSESS", Title: "Assess", Objective: "External proof.", Responsibility: "external_assessor", AssessmentObjectives: []string{"Inspect."}, ExternalAssessmentRequired: true, Evidence: []EvidenceRequirement{{Class: "external_assessment", MinimumCount: 1, AcceptedResults: []string{"verified"}}}},
	}
	report := BuildReadiness(profile, "task_set", []EvidenceMap{{TaskID: "T-1", Applicable: false, EvaluatedAt: "2026-07-12T12:00:00Z"}})
	for _, control := range report.Controls {
		if control.Status != "not_applicable_with_rationale" {
			t.Fatalf("out-of-scope control produced debt: %+v", control)
		}
	}
	if len(report.Gaps) != 0 {
		t.Fatalf("out-of-scope report produced gaps: %+v", report.Gaps)
	}
}

func TestBuildReadinessAggregatesEvidenceAcrossSelectedScope(t *testing.T) {
	profile := mappingProfile()
	mapWithEvidence := EvidenceMap{TaskID: "T-1", Applicable: true, EvaluatedAt: "2026-07-12T12:00:00Z",
		Facts: []EvidenceFact{{Reference: "evidence:T-1:1", Class: "evidence", Result: "pass", Timestamp: "2026-07-12T11:00:00Z", State: "current"}}}
	mapWithoutEvidence := EvidenceMap{TaskID: "T-2", Applicable: true, EvaluatedAt: "2026-07-12T12:00:00Z"}
	report := BuildReadiness(profile, "project", []EvidenceMap{mapWithEvidence, mapWithoutEvidence})
	if report.Controls[0].Status != "satisfied_by_recorded_evidence" {
		t.Fatalf("scope evidence was incorrectly required per task: %+v", report)
	}
}
