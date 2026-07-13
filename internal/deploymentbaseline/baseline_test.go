package deploymentbaseline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepositoryBaselinesValidateAndPassCompleteObservation(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "sovereign-deployment-baselines", "v1")
	for _, name := range []string{"single-host.yaml", "managed-service.yaml", "container-orchestration.yaml"} {
		baseline, err := LoadBaseline(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		results := make([]ObservationResult, 0, len(baseline.Controls))
		for _, control := range baseline.Controls {
			results = append(results, ObservationResult{ControlID: control.ID, Status: "pass", EvidenceRefs: []string{"artifacts/readback.json"}})
		}
		report, err := Evaluate(baseline, Observation{
			Schema: ObservationSchema, BaselineID: baseline.ID, BaselineVersion: baseline.Version,
			Topology: baseline.Topology, DeploymentID: "test-deployment", ObservedAt: time.Now().UTC().Format(time.RFC3339),
			Results: results, Authority: ObservationAuthority{Mode: "observation_only"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !report.Ready || report.PassingCount != len(baseline.Controls) || len(report.Deviations) != 0 {
			t.Fatalf("unexpected report for %s: %+v", name, report)
		}
	}
}

func TestEvaluateReportsBlockingAdvisoryAndEvidenceDeviations(t *testing.T) {
	baseline := testBaseline()
	observation := testObservation(baseline)
	observation.Results[0].Status = "fail"
	observation.Results[0].Notes = "service account is shared"
	observation.Results[len(observation.Results)-1].EvidenceRefs = nil
	observation.Results[2].Status = "not_applicable"
	observation.Results[2].Notes = "host platform uses an equivalent mandatory policy"
	observation.Results[2].EvidenceRefs = []string{"artifacts/equivalent-policy.json"}
	report, err := Evaluate(baseline, observation)
	if err != nil {
		t.Fatal(err)
	}
	if report.Ready || report.BlockingCount != 1 || report.AdvisoryCount != 1 || len(report.Deviations) != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
	statuses := map[string]string{}
	for _, deviation := range report.Deviations {
		statuses[deviation.ControlID] = deviation.Status
	}
	if statuses[baseline.Controls[0].ID] != "fail" || statuses["optional-observability"] != "missing_evidence" {
		t.Fatalf("unexpected deviations: %+v", report.Deviations)
	}
}

func TestLoadAndEvaluateFailClosed(t *testing.T) {
	baseline := testBaseline()
	observation := testObservation(baseline)
	observation.Results = append(observation.Results, ObservationResult{ControlID: "unknown-control", Status: "pass", EvidenceRefs: []string{"a"}})
	if _, err := Evaluate(baseline, observation); err == nil || !strings.Contains(err.Error(), "unknown control") {
		t.Fatalf("unexpected unknown control error: %v", err)
	}

	dir := t.TempDir()
	unknown := filepath.Join(dir, "unknown.yaml")
	if err := os.WriteFile(unknown, []byte("schema: fairway.sovereign-deployment-baseline.v1\nunknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBaseline(unknown); err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("unexpected field error: %v", err)
	}
	target := filepath.Join(dir, "target.yaml")
	if err := os.WriteFile(target, []byte("schema: invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBaseline(link); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("unexpected symlink error: %v", err)
	}
	if _, err := LoadBaseline("https://example.com/baseline.yaml"); err == nil || !strings.Contains(err.Error(), "local YAML") {
		t.Fatalf("unexpected remote baseline error: %v", err)
	}
	baseline.Controls[0].Expectation = "password=SHOULD_NOT_RENDER"
	if err := ValidateBaseline(baseline); err == nil || !strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "SHOULD_NOT_RENDER") {
		t.Fatalf("unexpected private content error: %v", err)
	}
	baseline = testBaseline()
	baseline.Controls[0].Severity = "advisory"
	if err := ValidateBaseline(baseline); err == nil || !strings.Contains(err.Error(), "must be blocking") {
		t.Fatalf("unexpected weakened severity error: %v", err)
	}
	baseline = testBaseline()
	baseline.Controls[0].EvidenceRequired = false
	if err := ValidateBaseline(baseline); err == nil || !strings.Contains(err.Error(), "must require evidence") {
		t.Fatalf("unexpected weakened evidence error: %v", err)
	}
	baseline = testBaseline()
	baseline.Authority.Mode = "mutating"
	if err := ValidateBaseline(baseline); err == nil || !strings.Contains(err.Error(), "observe_only") {
		t.Fatalf("unexpected authority error: %v", err)
	}
	baseline = testBaseline()
	observation = testObservation(baseline)
	observation.Results[0].EvidenceRefs = []string{"Authorization: Bearer SHOULD_NOT_RENDER"}
	if err := ValidateObservation(observation); err == nil || !strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "SHOULD_NOT_RENDER") {
		t.Fatalf("unexpected private reference error: %v", err)
	}
	observation = testObservation(baseline)
	observation.ObservedAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if err := ValidateObservation(observation); err == nil || !strings.Contains(err.Error(), "future") {
		t.Fatalf("unexpected future observation error: %v", err)
	}
	observation = testObservation(baseline)
	observation.BaselineVersion = "v2"
	if _, err := Evaluate(baseline, observation); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("unexpected identity mismatch error: %v", err)
	}
}

func testBaseline() Baseline {
	controls := make([]Control, 0, len(requiredControlIDs))
	for _, id := range requiredControlIDs {
		controls = append(controls, Control{ID: id, Title: id, Category: "runtime", Severity: "blocking", Expectation: "Record bounded posture evidence.", EvidenceRequired: true, AllowNotApplicable: id == "configuration-signing", NextAction: "Record the missing bounded evidence."})
	}
	controls = append(controls, Control{ID: "optional-observability", Title: "Optional observability", Category: "observability", Severity: "advisory", Expectation: "Record optional telemetry.", EvidenceRequired: true, NextAction: "Record telemetry if useful."})
	return Baseline{
		Schema: BaselineSchema, ID: "test-baseline", Version: "v1", Title: "Test baseline",
		Description: "Test deployment posture.", Topology: "single_host", Controls: controls,
		Authority: Authority{Mode: "observe_only", ProhibitedActions: append([]string(nil), requiredProhibitedActions...)},
	}
}

func testObservation(b Baseline) Observation {
	results := make([]ObservationResult, 0, len(b.Controls))
	for _, control := range b.Controls {
		results = append(results, ObservationResult{ControlID: control.ID, Status: "pass", EvidenceRefs: []string{"artifacts/readback.json"}})
	}
	return Observation{
		Schema: ObservationSchema, BaselineID: b.ID, BaselineVersion: b.Version, Topology: b.Topology,
		DeploymentID: "test-deployment", ObservedAt: time.Now().UTC().Format(time.RFC3339), Results: results,
		Authority: ObservationAuthority{Mode: "observation_only"},
	}
}
