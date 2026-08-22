package harnessrecord

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeBatchRejectsDuplicateKeysAndSecrets(t *testing.T) {
	for _, input := range []string{
		`{"schema":"fairway.harness-record-batch.v1","schema":"fairway.harness-record-batch.v1","observations":[]}`,
		`{"schema":"fairway.harness-record-batch.v1","external_runs":[{"schema":"fairway.harness.external-run.v1","source_id":"codex","source_version":"1","external_run_id":"r1","task_id":"T-001","submission_id":"s1","observed_at":"2026-08-21T00:00:00Z","metadata":{"authorization":"Bearer abcdefghijklmnop"}}]}`,
		`{"schema":"fairway.harness-record-batch.v1","external_runs":[{"schema":"fairway.harness.external-run.v1","source_id":"codex","source_version":"1","external_run_id":"r1","task_id":"T-001","submission_id":"s1","observed_at":"2026-08-21T00:00:00Z","metadata":{"vendor.transcript":"ordinary non-secret provider text"}}]}`,
		`{"schema":"fairway.harness-record-batch.v1","external_runs":[{"schema":"fairway.harness.external-run.v1","source_id":"codex","source_version":"1","external_run_id":"r1","task_id":"T-001","submission_id":"s1","observed_at":"2026-08-21T00:00:00Z","model":null}]}`,
		`{"schema":"fairway.harness-record-batch.v1","external_runs":[{"schema":"fairway.harness.external-run.v1","source_id":"codex","source_version":"1","external_run_id":"r1","task_id":"T-001","submission_id":"s1","observed_at":"2026-08-21T00:00:00Z","model":""}]}`,
	} {
		if _, err := DecodeBatch(strings.NewReader(input)); err == nil {
			t.Fatalf("expected rejection for %s", input)
		}
	}
}

func TestValidateBatchAllowsRunIndependentObservation(t *testing.T) {
	batch := Batch{Schema: BatchSchema, Observations: []Observation{{Schema: ObservationSchema, ObservationID: "o1", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", Kind: "execution", SubjectType: "commit", SubjectRef: "abc", Summary: "tests failed", ObservedAt: "2026-08-21T00:00:00Z", Outcome: "rejected", SourceMode: "measured"}}}
	if err := ValidateBatch(batch, time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalStable(t *testing.T) {
	value := Observation{Schema: ObservationSchema, ObservationID: "o1", SourceID: "ci", SourceVersion: "1", TaskID: "T-001", Kind: "execution", SubjectType: "commit", SubjectRef: "abc", Summary: "ok", ObservedAt: "2026-08-21T00:00:00Z", Outcome: "confirmed", SourceMode: "measured", Metadata: map[string]any{"z": 1.0, "a": "x"}}
	first, digest, err := Canonical(value)
	if err != nil {
		t.Fatal(err)
	}
	second, digest2, err := Canonical(value)
	if err != nil || string(first) != string(second) || digest != digest2 {
		t.Fatalf("canonical mismatch %s %s", digest, digest2)
	}
}

func TestProhibitedContentKeyVariants(t *testing.T) {
	for _, key := range []string{"vendor.raw_prompt_body", "vendor.private_reasoning_trace", "vendor.chain_of_thought", "vendor.transcript_text", "vendor.raw_tool_body_dump", "vendor.generated_content_dump", "vendor.credential_blob"} {
		if !prohibitedContentKeyName(key) {
			t.Errorf("key %q was not prohibited", key)
		}
	}
	for _, key := range []string{"vendor.prompt_version", "vendor.reasoning_model_id", "vendor.tool_name", "vendor.output_digest"} {
		if prohibitedContentKeyName(key) {
			t.Errorf("safe metadata key %q was prohibited", key)
		}
	}
}
