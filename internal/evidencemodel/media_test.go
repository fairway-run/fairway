package evidencemodel

import (
	"testing"

	"github.com/subashram/fairway/internal/store"
)

func TestUXMediaRowsClassifiesMediaEvidence(t *testing.T) {
	rows := UXMediaRows([]store.Evidence{
		{ArtifactType: "screenshot", ArtifactPath: "artifacts/home.png", Result: "pass"},
		{ArtifactType: "video", ArtifactPath: "artifacts/demo.mp4", Result: "pass"},
		{ArtifactType: "playwright-trace", ArtifactPath: "artifacts/trace.zip", Result: "pass"},
		{ArtifactType: "uat-proof", ArtifactPath: "artifacts/uat.md", Result: "partial"},
		{ArtifactType: "log", ArtifactPath: "artifacts/app.log", Result: "pass"},
	})
	if len(rows) != 4 {
		t.Fatalf("rows=%+v, want 4 media evidence rows", rows)
	}
	summary := UXMediaSummaryFor(rows)
	if summary.Screenshots != 1 || summary.Videos != 1 || summary.BrowserTraces != 1 || summary.UAT != 1 || !summary.Exercised {
		t.Fatalf("summary=%+v", summary)
	}
	for _, row := range rows {
		if !row.RedactionRequired {
			t.Fatalf("row=%+v, want redaction required", row)
		}
		if row.Boundary == "" {
			t.Fatalf("row=%+v, want boundary text", row)
		}
	}
}
