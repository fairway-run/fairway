package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/qualityrecord"
	"github.com/subashram/fairway/internal/store"
)

func TestQualityRowsKeepRecordStatesDistinct(t *testing.T) {
	tasks := []store.Task{{Definition: store.TaskDefinition{ID: "T-001", Title: "Inspectable", Role: "backend"}, Status: "done"}}
	records := []qualityrecord.Record{{TaskID: "T-001", Sections: []qualityrecord.Section{
		{ID: "intent", Title: "Intent", State: "present"},
		{ID: "decisions", Title: "Material Decisions", State: "missing"},
		{ID: "verification", Title: "Automatic Verification", State: "conflicting"},
		{ID: "promotion", Title: "Promotion Decision", State: "externally_owned"},
		{ID: "outcomes", Title: "Operational Outcomes", State: "unavailable"},
	}}}
	rows, summary := qualityRows(tasks, records)
	if len(rows) != 1 || !rows[0].Attention || len(rows[0].AttentionReasons) != 3 {
		t.Fatalf("rows=%+v", rows)
	}
	if summary.Present != 1 || summary.Missing != 1 || summary.Conflicting != 1 || summary.ExternallyOwned != 1 || summary.Unavailable != 1 || summary.AttentionTasks != 1 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestQualityWorkspaceRendersLifecycleMatrixAndCitedTaskLinks(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Quality workspace", Role: "ui", Kind: "dashboard", Profile: "fairway-adoption", RiskLevel: "medium", AcceptanceChecks: []string{"workspace is inspectable"}},
		{ID: "T-002", Title: "Filtered task", Role: "backend", Kind: "task"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./internal/dashboard", Result: "pass", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"ui", "backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/quality?q=workspace&role=ui", nil)
	rec := httptest.NewRecorder()
	server.quality(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Quality Workspace", "Lifecycle Matrix", "Quality workspace", `href="/tasks/T-001#quality-record"`, "Present stages", "Missing stages", "Unavailable stages", "Conflicting stages", "External authority", `class="active" href="/quality"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("quality body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<span>Filtered task</span>") {
		t.Fatalf("quality workspace ignored filters:\n%s", body)
	}
}

func TestQualityPaginationPreservesFilters(t *testing.T) {
	filters := QualityFilters{Search: "api", Status: "done", Role: "backend", Profile: "standard", Risk: "high", Page: 2, PageSize: 10}
	href := qualityPageHref(filters, 3)
	for _, want := range []string{"/quality?", "page=3", "limit=10", "q=api", "status=done", "role=backend", "profile=standard", "risk=high"} {
		if !strings.Contains(href, want) {
			t.Fatalf("href=%q missing %q", href, want)
		}
	}
}
