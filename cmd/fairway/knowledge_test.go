package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/store"
)

func TestCaptureFactsExcludeFreeFormOperationalText(t *testing.T) {
	const sentinel = "PRIVATE_FREE_FORM_SENTINEL"
	facts := captureFacts(
		nil,
		[]store.TaskEvidenceFact{{ID: 1, Result: "pass", ArtifactType: "test", Summary: "result=pass artifact_type=test"}},
		[]store.TaskReviewFact{{ID: 2, Reviewer: "arch", Domain: "arch", Verdict: "approve"}},
		[]store.TaskOutcome{{ID: 3, Kind: "incident", SourceRef: sentinel, Notes: sentinel}},
		nil,
	)
	if len(facts) != 3 {
		t.Fatalf("facts=%+v", facts)
	}
	for _, fact := range facts {
		if strings.Contains(fact.Summary, sentinel) {
			t.Fatalf("capture copied free-form operational text: %+v", fact)
		}
	}
	for _, expected := range []string{"result=pass artifact_type=test", "verdict=approve domain=arch reviewer=arch", "kind=incident related_task=none transition_id=0"} {
		found := false
		for _, fact := range facts {
			found = found || fact.Summary == expected
		}
		if !found {
			t.Fatalf("missing structural projection %q: %+v", expected, facts)
		}
	}
}

func TestCommandEmbedderRejectsUnboundedOutput(t *testing.T) {
	script := filepath.Join(t.TempDir(), "oversized-embedder")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n/usr/bin/yes x | /usr/bin/head -c 1048577\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := commandEmbedder(script, "test")("query")
	if err == nil || !strings.Contains(err.Error(), "embedding adapter output exceeds") {
		t.Fatalf("oversized embedding adapter output was not rejected: %v", err)
	}
}
