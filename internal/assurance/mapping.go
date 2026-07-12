package assurance

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const EvidenceMapSchema = "fairway.assurance-evidence-map.v1"

type TaskContext struct {
	Project   string
	TaskID    string
	Kind      string
	RiskLevel string
	Tags      []string
	Status    string
	UpdatedAt string
	CommitSHA string
}

type SourceEvidence struct {
	Index        int
	Result       string
	ArtifactType string
	CreatedAt    string
}

type SourceReview struct {
	Index     int
	Domain    string
	Verdict   string
	Reviewer  string
	CreatedAt string
}

type SourceDecision struct {
	ID           int64
	QualityState string
	CreatedBy    string
	CreatedAt    string
}

type Sources struct {
	Task      TaskContext
	Evidence  []SourceEvidence
	Reviews   []SourceReview
	Decisions []SourceDecision
}

type EvidenceFact struct {
	Reference          string `json:"reference"`
	Class              string `json:"class"`
	Result             string `json:"result"`
	Timestamp          string `json:"timestamp,omitempty"`
	Actor              string `json:"actor,omitempty"`
	Producer           string `json:"producer"`
	Project            string `json:"project"`
	TaskID             string `json:"task_id"`
	ProfileApplicable  bool   `json:"profile_applicable"`
	Freshness          string `json:"freshness"`
	ConfidenceBoundary string `json:"confidence_boundary"`
	State              string `json:"state"`
}

type RequirementMapping struct {
	Class           string   `json:"class"`
	MinimumCount    int      `json:"minimum_count"`
	AcceptedResults []string `json:"accepted_results"`
	MaximumAge      string   `json:"maximum_age,omitempty"`
	Matched         int      `json:"matched"`
	State           string   `json:"state"`
	FactReferences  []string `json:"fact_references,omitempty"`
}

type ControlMapping struct {
	ControlID    string               `json:"control_id"`
	State        string               `json:"state"`
	Requirements []RequirementMapping `json:"requirements"`
}

type EvidenceMap struct {
	Schema            string           `json:"schema"`
	ProfileID         string           `json:"profile_id"`
	ProfileVersion    string           `json:"profile_version"`
	Project           string           `json:"project"`
	TaskID            string           `json:"task_id"`
	Applicable        bool             `json:"applicable"`
	ApplicabilityNote string           `json:"applicability_note"`
	EvaluatedAt       string           `json:"evaluated_at"`
	Facts             []EvidenceFact   `json:"facts"`
	Controls          []ControlMapping `json:"controls"`
	AuthorityBoundary string           `json:"authority_boundary"`
}

func MapEvidence(profile Profile, sources Sources, now time.Time) EvidenceMap {
	now = now.UTC()
	applicable, note := profileApplies(profile, sources.Task)
	facts := normalizeFacts(profile, sources, now, applicable)
	returnMap := EvidenceMap{
		Schema: EvidenceMapSchema, ProfileID: profile.ID, ProfileVersion: profile.Version,
		Project: sources.Task.Project, TaskID: sources.Task.TaskID, Applicable: applicable,
		ApplicabilityNote: note, EvaluatedAt: now.Format(time.RFC3339Nano), Facts: facts,
		AuthorityBoundary: "evidence mapping only; not certification, compliance, approval, or risk acceptance",
	}
	for _, control := range profile.Controls {
		mapping := ControlMapping{ControlID: control.ID, State: "supported"}
		for _, requirement := range control.Evidence {
			req := mapRequirement(requirement, facts, applicable, now)
			mapping.Requirements = append(mapping.Requirements, req)
			if req.State != "supported" {
				mapping.State = "not_supported"
			}
		}
		if control.ExternalAssessmentRequired {
			found := false
			for i := range mapping.Requirements {
				if mapping.Requirements[i].Class == "external_assessment" {
					mapping.Requirements[i].State = "external_assessment_required"
					mapping.Requirements[i].Matched = 0
					mapping.Requirements[i].FactReferences = nil
					found = true
				}
			}
			if !found {
				mapping.Requirements = append(mapping.Requirements, RequirementMapping{
					Class: "external_assessment", MinimumCount: 1, State: "external_assessment_required",
				})
			}
			mapping.State = "not_supported"
		}
		returnMap.Controls = append(returnMap.Controls, mapping)
	}
	return returnMap
}

func profileApplies(profile Profile, task TaskContext) (bool, string) {
	if len(profile.Applicability.TaskKinds) > 0 && !contains(profile.Applicability.TaskKinds, task.Kind) {
		return false, "task kind is outside profile applicability"
	}
	if len(profile.Applicability.RiskLevels) > 0 && !contains(profile.Applicability.RiskLevels, task.RiskLevel) {
		return false, "risk level is outside profile applicability"
	}
	for _, required := range profile.Applicability.Tags {
		if !contains(task.Tags, required) {
			return false, "required profile applicability tag is missing"
		}
	}
	return true, "task matches profile applicability"
}

func normalizeFacts(profile Profile, sources Sources, now time.Time, applicable bool) []EvidenceFact {
	var facts []EvidenceFact
	add := func(reference, class, result, timestamp, actor, producer, state string) {
		freshness := "requirement_relative"
		if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
			freshness = "unknown"
		}
		if !applicable {
			state = "out_of_scope"
		}
		facts = append(facts, EvidenceFact{Reference: reference, Class: class, Result: result,
			Timestamp: timestamp, Actor: actor, Producer: producer, Project: sources.Task.Project,
			TaskID: sources.Task.TaskID, ProfileApplicable: applicable, Freshness: freshness,
			ConfidenceBoundary: confidenceBoundary(class), State: state})
	}
	add("task:"+sources.Task.TaskID, "task", sources.Task.Status, sources.Task.UpdatedAt, "", "fairway", "current")
	for i, evidence := range sources.Evidence {
		index := evidence.Index
		if index == 0 {
			index = i + 1
		}
		class := evidenceClass(evidence.ArtifactType)
		state := "current"
		if class == "external_assessment" {
			state = "external_assertion"
		}
		add(fmt.Sprintf("evidence:%s:%d", sources.Task.TaskID, index), class, evidence.Result, evidence.CreatedAt, "", "fairway", state)
	}
	latestReviews := latestReviewPositions(sources.Reviews)
	for i, review := range sources.Reviews {
		index := review.Index
		if index == 0 {
			index = i + 1
		}
		state := "superseded"
		if latestReviews[reviewDomainKey(review)] == i {
			state = "current"
		}
		add(fmt.Sprintf("review:%s:%s:%d", sources.Task.TaskID, review.Domain, index), "review", review.Verdict, review.CreatedAt, review.Reviewer, "fairway-review", state)
	}
	for _, decision := range sources.Decisions {
		state := "current"
		if decision.QualityState == "superseded" {
			state = "superseded"
		}
		if decision.QualityState == "draft" || decision.QualityState == "changes" {
			state = "unreviewed"
		}
		add(fmt.Sprintf("decision:%s:%d", sources.Task.TaskID, decision.ID), "decision", decision.QualityState, decision.CreatedAt, decision.CreatedBy, "fairway-decision", state)
	}
	markConflicts(facts)
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].Class != facts[j].Class {
			return facts[i].Class < facts[j].Class
		}
		return facts[i].Reference < facts[j].Reference
	})
	return facts
}

func latestReviewPositions(reviews []SourceReview) map[string]int {
	latest := map[string]int{}
	for i, review := range reviews {
		key := reviewDomainKey(review)
		previous, ok := latest[key]
		if !ok || reviewIsLater(review, i, reviews[previous], previous) {
			latest[key] = i
		}
	}
	return latest
}

func reviewDomainKey(review SourceReview) string {
	key := strings.ToLower(strings.TrimSpace(review.Domain))
	if key == "" {
		// Legacy rows used reviewer role as the effective review domain.
		key = strings.ToLower(strings.TrimSpace(review.Reviewer))
	}
	return key
}

func reviewIsLater(candidate SourceReview, candidatePosition int, previous SourceReview, previousPosition int) bool {
	candidateAt, candidateErr := time.Parse(time.RFC3339Nano, candidate.CreatedAt)
	previousAt, previousErr := time.Parse(time.RFC3339Nano, previous.CreatedAt)
	if candidateErr == nil && previousErr == nil && !candidateAt.Equal(previousAt) {
		return candidateAt.After(previousAt)
	}
	if candidateErr == nil && previousErr != nil {
		return true
	}
	if candidateErr != nil && previousErr == nil {
		return false
	}
	candidateOrder := candidate.Index
	if candidateOrder == 0 {
		candidateOrder = candidatePosition + 1
	}
	previousOrder := previous.Index
	if previousOrder == 0 {
		previousOrder = previousPosition + 1
	}
	if candidateOrder != previousOrder {
		return candidateOrder > previousOrder
	}
	return candidatePosition > previousPosition
}

func markConflicts(facts []EvidenceFact) {
	results := map[string]map[string]bool{}
	for _, fact := range facts {
		if fact.State != "current" {
			continue
		}
		if results[fact.Class] == nil {
			results[fact.Class] = map[string]bool{}
		}
		results[fact.Class][fact.Result] = true
	}
	for i := range facts {
		if facts[i].State == "current" && len(results[facts[i].Class]) > 1 {
			facts[i].State = "conflicting"
		}
	}
}

func mapRequirement(req EvidenceRequirement, facts []EvidenceFact, applicable bool, now time.Time) RequirementMapping {
	mapping := RequirementMapping{Class: req.Class, MinimumCount: req.MinimumCount,
		AcceptedResults: append([]string(nil), req.AcceptedResults...), MaximumAge: req.MaximumAge, State: "missing"}
	if !applicable {
		mapping.State = "out_of_scope"
		return mapping
	}
	seenProblem := ""
	maximumAge, hasMaximumAge := time.Duration(0), false
	if req.MaximumAge != "" {
		maximumAge, _ = time.ParseDuration(req.MaximumAge)
		hasMaximumAge = true
	}
	for _, fact := range facts {
		if fact.Class != req.Class {
			continue
		}
		if fact.State != "current" {
			if seenProblem == "" {
				seenProblem = fact.State
			}
			continue
		}
		if hasMaximumAge {
			created, err := time.Parse(time.RFC3339Nano, fact.Timestamp)
			if err != nil || now.Sub(created) > maximumAge {
				seenProblem = "stale"
				continue
			}
		}
		if contains(req.AcceptedResults, fact.Result) {
			mapping.Matched++
			mapping.FactReferences = append(mapping.FactReferences, fact.Reference)
		}
	}
	if mapping.Matched >= req.MinimumCount {
		mapping.State = "supported"
	} else if seenProblem != "" {
		mapping.State = seenProblem
	}
	return mapping
}

func evidenceClass(artifactType string) string {
	value := strings.ToLower(strings.TrimSpace(artifactType))
	switch {
	case strings.Contains(value, "external") || strings.Contains(value, "assessment"):
		return "external_assessment"
	case strings.Contains(value, "provenance") || strings.Contains(value, "attestation") || strings.Contains(value, "manifest"):
		return "provenance"
	case strings.Contains(value, "release"):
		return "release"
	case strings.Contains(value, "rehearsal") || strings.Contains(value, "preflight"):
		return "rehearsal"
	case strings.Contains(value, "exception"):
		return "exception"
	case strings.Contains(value, "ci") || strings.Contains(value, "test"):
		return "ci"
	case strings.Contains(value, "backup") || strings.Contains(value, "restore"):
		return "backup_restore"
	case strings.Contains(value, "vulnerab") || strings.Contains(value, "scan"):
		return "vulnerability"
	case strings.Contains(value, "identity") || strings.Contains(value, "auth"):
		return "identity"
	case strings.Contains(value, "audit"):
		return "audit"
	case strings.Contains(value, "config"):
		return "configuration"
	default:
		return "evidence"
	}
}

func confidenceBoundary(class string) string {
	if class == "external_assessment" {
		return "external assertion; assessor validation required"
	}
	return "recorded Fairway metadata; artifact content not evaluated"
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
