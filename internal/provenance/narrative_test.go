package provenance

import (
	"os"
	"strings"
	"testing"
)

func TestNarrativeEndpointFailsClosedOutsideLoopback(t *testing.T) {
	for _, value := range []string{
		"https://127.0.0.1:11434",
		"http://192.0.2.10:11434",
		"http://user:secret@127.0.0.1:11434",
		"http://127.0.0.1:11434/custom",
		"http://127.0.0.1:11434?token=secret",
	} {
		t.Setenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT", value)
		if _, err := narrativeEndpoint("FAIRWAY_TEST_NARRATIVE_ENDPOINT"); err == nil || strings.Contains(err.Error(), value) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("value=%q err=%v", value, err)
		}
	}
	t.Setenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT", "http://127.0.0.1:11434")
	endpoint, err := narrativeEndpoint("FAIRWAY_TEST_NARRATIVE_ENDPOINT")
	if err != nil || endpoint != "http://127.0.0.1:11434/api/generate" {
		t.Fatalf("endpoint=%q err=%v", endpoint, err)
	}
	_ = os.Unsetenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT")
}

func TestValidateExplainNarrativeRequiresKnownCitationsAndPrivacy(t *testing.T) {
	packet := ExplainCodePacket{MachineInferenceInputs: []string{"task:T-001", "evidence:T-001:1"}}
	valid := ExplainNarrative{
		Schema: "fairway.explain-narrative.v1",
		Statements: []NarrativeStatement{
			{Label: "recorded", Text: "The task exists.", Citations: []string{"task:T-001"}},
			{Label: "inferred", Text: "The evidence supports a bounded interpretation.", Citations: []string{"evidence:T-001:1"}},
			{Label: "unknown", Text: "No record explains the later caller."},
		},
	}
	if err := ValidateExplainNarrative(packet, valid); err != nil {
		t.Fatal(err)
	}
	unknown := valid
	unknown.Statements = []NarrativeStatement{{Label: "recorded", Text: "Claim", Citations: []string{"task:OTHER"}}}
	if err := ValidateExplainNarrative(packet, unknown); err == nil || !strings.Contains(err.Error(), "unknown packet fact") {
		t.Fatalf("unknown citation err=%v", err)
	}
	uncited := valid
	uncited.Statements = []NarrativeStatement{{Label: "inferred", Text: "Claim"}}
	if err := ValidateExplainNarrative(packet, uncited); err == nil || !strings.Contains(err.Error(), "requires a grounded citation") {
		t.Fatalf("uncited err=%v", err)
	}
	secret := valid
	secret.Statements = []NarrativeStatement{{Label: "recorded", Text: "access_token=do-not-render", Citations: []string{"task:T-001"}}}
	if err := ValidateExplainNarrative(packet, secret); err == nil || !strings.Contains(err.Error(), "privacy-rejected") {
		t.Fatalf("secret err=%v", err)
	}
}

func TestDecodeStrictNarrativeJSONRejectsUnknownAndTrailingFields(t *testing.T) {
	for _, raw := range []string{
		`{"schema":"fairway.explain-narrative.v1","statements":[],"unexpected":true}`,
		`{"schema":"fairway.explain-narrative.v1","statements":[]} {}`,
	} {
		var narrative ExplainNarrative
		if err := decodeStrictNarrativeJSON([]byte(raw), &narrative); err == nil {
			t.Fatalf("expected strict JSON rejection for %s", raw)
		}
	}
}

func TestNarrativePromptIsBounded(t *testing.T) {
	packet := ExplainCodePacket{Schema: "fairway.explain-code.v1", Facts: []ExplainFact{{Ref: "fact:1", Summary: strings.Repeat("x", maxNarrativePromptBytes)}}}
	if _, err := narrativePrompt(packet); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("prompt err=%v", err)
	}
}
