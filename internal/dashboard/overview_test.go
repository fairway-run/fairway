package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestOverviewSummaryUsesRecordedProjectFacts(t *testing.T) {
	tasks := []store.Task{
		{Definition: store.TaskDefinition{ID: "T-001"}, Status: "done"},
		{Definition: store.TaskDefinition{ID: "T-002"}, Status: "in_progress"},
		{Definition: store.TaskDefinition{ID: "T-003"}, Status: "blocked"},
	}
	summary := overviewSummary(tasks,
		map[string][]store.Evidence{"T-001": {{Result: "pass"}}, "T-002": {{Result: "fail"}}},
		map[string][]store.Review{"T-001": {{Reviewer: "arch", Verdict: "approve"}}},
		[]store.Session{{ID: "s-1", Status: "running"}, {ID: "s-2", Status: "ended"}},
	)
	if summary.TotalTasks != 3 || summary.EvidenceBacked != 2 || summary.Reviewed != 1 || summary.Completed != 1 || summary.Active != 1 || summary.Blocked != 1 || summary.ActiveProviders != 1 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestOverviewRendersProductJourneyAndCitedRecord(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{
		ID: "T-001", Title: "Evidence-rich delivery", Role: "backend", Kind: "task", RiskLevel: "medium", AcceptanceChecks: []string{"verified"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", store.Review{Reviewer: "architecture-reviewer", Domain: "arch", Verdict: "approve", Reason: "verified"}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Trust the engineering record", "From intent to operational learning", "Current Project Evidence",
		"Evidence-rich delivery", `href="/tasks/T-001#quality-record"`, "Where authority stays", "Explore Fairway",
		`class="active" href="/"`, `href="/wall"`, "Promote", "explicit external authority", "No quality score", "No autonomous approval",
		"Keep the control plane small", "Subagent", "Task-specific thread", "Durable control surface",
		"archive the thread after closeout", "task, decisions, evidence, reviews, checkpoints, and handbacks",
		"Review independence is routed, not inferred", "Material delegated work still requires a registered session",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("overview missing %q:\n%s", want, body)
		}
	}
}

func TestWallRouteRendersOperationalWall(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Role: "backend", Kind: "task"}}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/wall", nil)
	rec := httptest.NewRecorder()
	server.wall(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wall-layout") || !strings.Contains(rec.Body.String(), `class="active" href="/wall"`) {
		t.Fatalf("wall status=%d body=%s", rec.Code, rec.Body.String())
	}
}
