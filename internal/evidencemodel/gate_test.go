package evidencemodel

import (
	"testing"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestEvaluateGateEnforcesCountArtifactFreshnessAndSignoff(t *testing.T) {
	now := time.Now().UTC()
	gate := config.WorkstreamProfileGate{EvidenceType: "test", RequiredEvidenceCount: 2, AcceptedResults: []string{"pass"}, ArtifactRequired: true, ExpiresAfter: "1h", OwnerSignoffRequired: true}
	evidence := []store.Evidence{
		{ArtifactType: "test", Result: "pass", ArtifactPath: "one.log", Notes: "owner signoff", CreatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano)},
		{ArtifactType: "test", Result: "pass", ArtifactPath: "", Notes: "owner signoff", CreatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano)},
		{ArtifactType: "test", Result: "pass", ArtifactPath: "old.log", Notes: "owner signoff", CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano)},
	}
	result := EvaluateGate(gate, evidence, now)
	if result.Satisfied || result.Matching != 1 || len(result.Reasons) == 0 {
		t.Fatalf("result=%+v", result)
	}
	evidence = append(evidence, store.Evidence{ArtifactType: "test", Result: "pass", ArtifactPath: "two.log", Notes: "owner sign-off", CreatedAt: now.Format(time.RFC3339Nano)})
	result = EvaluateGate(gate, evidence, now)
	if !result.Satisfied || result.Matching != 2 {
		t.Fatalf("result=%+v", result)
	}
}
