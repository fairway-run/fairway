package evidencemodel

import (
	"fmt"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

type GateEvaluation struct {
	Satisfied bool     `json:"satisfied"`
	Matching  int      `json:"matching"`
	Reasons   []string `json:"reasons,omitempty"`
}

func EvaluateGate(gate config.WorkstreamProfileGate, evidence []store.Evidence, now time.Time) GateEvaluation {
	requiredCount := gate.RequiredEvidenceCount
	if requiredCount == 0 && (gate.EvidenceType != "" || len(gate.AcceptedResults) > 0 || gate.ArtifactRequired || gate.ExpiresAfter != "" || gate.OwnerSignoffRequired) {
		requiredCount = 1
	}
	accepted := map[string]bool{}
	for _, result := range gate.AcceptedResults {
		accepted[result] = true
	}
	matching := 0
	for _, ev := range evidence {
		if gate.EvidenceType != "" && ev.ArtifactType != gate.EvidenceType {
			continue
		}
		if len(accepted) > 0 && !accepted[ev.Result] {
			continue
		}
		if gate.ArtifactRequired && ev.ArtifactPath == "" {
			continue
		}
		if gate.ExpiresAfter != "" && !EvidenceIsFresh(ev, gate.ExpiresAfter, now) {
			continue
		}
		if gate.OwnerSignoffRequired && !EvidenceHasOwnerSignoff(ev) {
			continue
		}
		matching++
	}
	var reasons []string
	if matching < requiredCount {
		reasons = append(reasons, fmt.Sprintf("needs %d matching evidence row(s), found %d", requiredCount, matching))
		if gate.ArtifactRequired {
			reasons = append(reasons, "matching rows must include evidence artifacts")
		}
		if gate.ExpiresAfter != "" {
			reasons = append(reasons, "matching rows must be fresh")
		}
		if gate.OwnerSignoffRequired {
			reasons = append(reasons, "matching rows must include owner signoff evidence notes")
		}
	}
	return GateEvaluation{Satisfied: len(reasons) == 0, Matching: matching, Reasons: reasons}
}

func EvidenceHasOwnerSignoff(ev store.Evidence) bool {
	notes := strings.ToLower(ev.Notes)
	return strings.Contains(notes, "signoff") || strings.Contains(notes, "sign-off")
}

func EvidenceIsFresh(ev store.Evidence, expiresAfter string, now time.Time) bool {
	if expiresAfter == "" {
		return true
	}
	ttl, err := time.ParseDuration(expiresAfter)
	if err != nil {
		return false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ev.CreatedAt)
	if err != nil {
		return false
	}
	return now.Sub(createdAt) <= ttl
}
