package assurance

import (
	"fmt"
	"sort"
	"time"
)

const ReadinessSchema = "fairway.assurance-readiness.v1"

type ReadinessReport struct {
	Schema            string             `json:"schema"`
	ProfileID         string             `json:"profile_id"`
	ProfileVersion    string             `json:"profile_version"`
	Scope             string             `json:"scope"`
	ScopeID           string             `json:"scope_id"`
	EvaluatedAt       string             `json:"evaluated_at"`
	TaskIDs           []string           `json:"task_ids"`
	Controls          []ControlReadiness `json:"controls"`
	Gaps              []AssuranceGap     `json:"gaps"`
	Summary           ReadinessSummary   `json:"summary"`
	AuthorityBoundary string             `json:"authority_boundary"`
}

type ControlReadiness struct {
	ControlID        string   `json:"control_id"`
	Title            string   `json:"title"`
	Status           string   `json:"status"`
	Responsibility   string   `json:"responsibility"`
	Rationale        string   `json:"rationale"`
	SourceReferences []string `json:"source_references,omitempty"`
	AssessorBoundary string   `json:"assessor_boundary"`
}

type AssuranceGap struct {
	ControlID          string   `json:"control_id"`
	EvidenceClass      string   `json:"evidence_class"`
	Status             string   `json:"status"`
	Owner              string   `json:"owner"`
	NextEvidenceAction string   `json:"next_evidence_action"`
	SourceReferences   []string `json:"source_references,omitempty"`
	Freshness          string   `json:"freshness"`
	AssessorBoundary   string   `json:"assessor_boundary"`
}

type ReadinessSummary struct {
	TotalControls int            `json:"total_controls"`
	ByStatus      map[string]int `json:"by_status"`
}

func BuildReadiness(profile Profile, scope string, maps []EvidenceMap) ReadinessReport {
	report := ReadinessReport{Schema: ReadinessSchema, ProfileID: profile.ID, ProfileVersion: profile.Version,
		Scope: scope, Summary: ReadinessSummary{TotalControls: len(profile.Controls), ByStatus: map[string]int{}},
		AuthorityBoundary: "readiness evidence and gaps only; not certification, compliance, approval, or risk acceptance"}
	for _, mapped := range maps {
		report.TaskIDs = append(report.TaskIDs, mapped.TaskID)
		if report.EvaluatedAt == "" || mapped.EvaluatedAt < report.EvaluatedAt {
			report.EvaluatedAt = mapped.EvaluatedAt
		}
	}
	sort.Strings(report.TaskIDs)
	aggregated := aggregateEvidenceMap(profile, maps, report.EvaluatedAt)
	readinessMaps := []EvidenceMap{aggregated}
	for controlIndex, control := range profile.Controls {
		row := summarizeControl(control, controlIndex, readinessMaps)
		report.Controls = append(report.Controls, row)
		report.Summary.ByStatus[row.Status]++
		if row.Status != "satisfied_by_recorded_evidence" && row.Status != "not_applicable_with_rationale" {
			report.Gaps = append(report.Gaps, controlGaps(control, controlIndex, readinessMaps, row.Status)...)
		}
	}
	return report
}

func aggregateEvidenceMap(profile Profile, maps []EvidenceMap, evaluatedAt string) EvidenceMap {
	at, _ := time.Parse(time.RFC3339Nano, evaluatedAt)
	aggregated := EvidenceMap{ProfileID: profile.ID, ProfileVersion: profile.Version, Applicable: false, EvaluatedAt: evaluatedAt}
	for _, mapped := range maps {
		if mapped.Applicable {
			aggregated.Applicable = true
		}
		aggregated.Facts = append(aggregated.Facts, mapped.Facts...)
	}
	markConflicts(aggregated.Facts)
	for _, control := range profile.Controls {
		mappedControl := ControlMapping{ControlID: control.ID, State: "supported"}
		for _, requirement := range control.Evidence {
			req := mapRequirement(requirement, aggregated.Facts, aggregated.Applicable, at)
			mappedControl.Requirements = append(mappedControl.Requirements, req)
			if req.State != "supported" {
				mappedControl.State = "not_supported"
			}
		}
		if control.ExternalAssessmentRequired {
			mappedControl.State = "not_supported"
			mappedControl.Requirements = append(mappedControl.Requirements, RequirementMapping{Class: "external_assessment", MinimumCount: 1, State: "external_assessment_required"})
		}
		aggregated.Controls = append(aggregated.Controls, mappedControl)
	}
	return aggregated
}

func summarizeControl(control Control, index int, maps []EvidenceMap) ControlReadiness {
	row := ControlReadiness{ControlID: control.ID, Title: control.Title, Responsibility: control.Responsibility,
		AssessorBoundary: "Fairway organizes recorded evidence; an authorized assessor determines any certification outcome"}
	if !anyApplicableMap(maps) {
		row.Status = "not_applicable_with_rationale"
		row.Rationale = "no selected task matches profile applicability"
		return row
	}
	if control.ExternalAssessmentRequired {
		row.Status = "external_assessment_required"
		row.Rationale = "an independent external assessment is required and cannot be inferred from Fairway records"
		return row
	}
	if control.Responsibility == "customer" {
		row.Status = "customer_responsibility"
		row.Rationale = "the adopting organization owns the required evidence and assessment"
		return row
	}
	var supported, requirements int
	states := map[string]bool{}
	for _, mapped := range maps {
		if !mapped.Applicable {
			continue
		}
		if index >= len(mapped.Controls) {
			continue
		}
		for _, req := range mapped.Controls[index].Requirements {
			requirements++
			states[req.State] = true
			if req.State == "supported" {
				supported++
				row.SourceReferences = append(row.SourceReferences, req.FactReferences...)
			}
		}
	}
	row.SourceReferences = uniqueSorted(row.SourceReferences)
	switch {
	case states["conflicting"]:
		row.Status = "conflicting"
		row.Rationale = "current recorded facts disagree"
	case states["stale"]:
		row.Status = "stale"
		row.Rationale = "recorded proof exceeds at least one requirement freshness window"
	case hasSupportedClass(maps, index, "exception"):
		row.Status = "exception_recorded"
		row.Rationale = "an exception is recorded; it is not evidence that the control is satisfied"
	case requirements > 0 && supported == requirements:
		row.Status = "satisfied_by_recorded_evidence"
		row.Rationale = "all explicit evidence requirements match current recorded Fairway facts"
	case supported > 0:
		row.Status = "partial"
		row.Rationale = "some but not all explicit evidence requirements have current matching facts"
	default:
		row.Status = "missing"
		row.Rationale = "required recorded proof is missing"
	}
	return row
}

func anyApplicableMap(maps []EvidenceMap) bool {
	for _, mapped := range maps {
		if mapped.Applicable {
			return true
		}
	}
	return false
}

func controlGaps(control Control, index int, maps []EvidenceMap, controlStatus string) []AssuranceGap {
	if control.ExternalAssessmentRequired {
		return []AssuranceGap{{ControlID: control.ID, EvidenceClass: "external_assessment", Status: "external_assessment_required",
			Owner: "external_assessor", NextEvidenceAction: "obtain and record an assessment reference from the authorized external assessor",
			Freshness: "assessor_defined", AssessorBoundary: "Fairway cannot perform or conclude the external assessment"}}
	}
	if control.Responsibility == "customer" {
		return []AssuranceGap{{ControlID: control.ID, EvidenceClass: "customer_evidence", Status: "customer_responsibility",
			Owner: "customer", NextEvidenceAction: "provide a scoped evidence reference for assessor review",
			Freshness: "profile_defined", AssessorBoundary: "customer evidence remains subject to assessor validation"}}
	}
	var gaps []AssuranceGap
	for reqIndex, req := range control.Evidence {
		states := map[string]bool{}
		var refs []string
		for _, mapped := range maps {
			if !mapped.Applicable || index >= len(mapped.Controls) || reqIndex >= len(mapped.Controls[index].Requirements) {
				continue
			}
			mappedReq := mapped.Controls[index].Requirements[reqIndex]
			states[mappedReq.State] = true
			refs = append(refs, mappedReq.FactReferences...)
			for _, fact := range mapped.Facts {
				if fact.Class == req.Class {
					refs = append(refs, fact.Reference)
				}
			}
		}
		if states["supported"] && controlStatus == "satisfied_by_recorded_evidence" {
			continue
		}
		status := controlStatus
		if states["conflicting"] {
			status = "conflicting"
		} else if states["stale"] {
			status = "stale"
		} else if states["supported"] {
			status = "partial"
		}
		gaps = append(gaps, AssuranceGap{ControlID: control.ID, EvidenceClass: req.Class, Status: status,
			Owner: control.Responsibility, NextEvidenceAction: nextEvidenceAction(req.Class, status), SourceReferences: uniqueSorted(refs),
			Freshness: freshnessLabel(req), AssessorBoundary: "recorded proof supports assessment preparation only"})
	}
	return gaps
}

func nextEvidenceAction(class, status string) string {
	switch status {
	case "stale":
		return fmt.Sprintf("record fresh %s evidence within the profile freshness window", class)
	case "conflicting":
		return fmt.Sprintf("resolve and independently validate conflicting %s facts", class)
	case "exception_recorded":
		return fmt.Sprintf("review the %s exception and record disposition evidence", class)
	default:
		return fmt.Sprintf("record a scoped %s evidence reference with an accepted result", class)
	}
}

func freshnessLabel(req EvidenceRequirement) string {
	if req.MaximumAge == "" {
		return "no maximum_age declared"
	}
	return "maximum_age=" + req.MaximumAge
}

func hasSupportedClass(maps []EvidenceMap, controlIndex int, class string) bool {
	for _, mapped := range maps {
		if !mapped.Applicable || controlIndex >= len(mapped.Controls) {
			continue
		}
		for _, req := range mapped.Controls[controlIndex].Requirements {
			if req.Class == class && req.State == "supported" {
				return true
			}
		}
	}
	return false
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
