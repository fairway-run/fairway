package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/store"
)

func TestIndexRendersV2Visibility(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "E-001", Title: "Epic", Kind: "epic", Role: "backend"},
		{ID: "T-001", ParentID: "E-001", Title: "Task", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "s-1", Role: "backend", TaskID: "T-001", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheckpoint(ctx, store.Checkpoint{TaskID: "E-001", State: "active", Owner: "backend", Summary: "working"}); err != nil {
		t.Fatal(err)
	}
	if err := s.StartWatcher(ctx, store.Watcher{ID: "W-001", TaskID: "T-001", Owner: "ops/watch", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	server := New(s, []string{"backend"}, []WorktreeStatus{{Role: "backend", Branch: "agent/backend", Exists: true, Registered: true}})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Sessions", "Worktrees", "Watchers", "Checkpoints", "E-001", "W-001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard body missing %q:\n%s", want, body)
		}
	}
}

func TestDashboardClaimRequiresCSRFAndAudits(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	server := New(s, []string{"backend"}, nil)
	badReq := httptest.NewRequest(http.MethodPost, "/actions/claim", strings.NewReader("task_id=T-001&csrf=bad"))
	badReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badRec := httptest.NewRecorder()
	server.claim(badRec, badReq)
	if badRec.Code != http.StatusForbidden {
		t.Fatalf("bad csrf status=%d, want 403", badRec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/actions/claim", strings.NewReader("task_id=T-001&csrf="+server.csrfToken))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.claim(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("claim status=%d, want 303", rec.Code)
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "in_progress" {
		t.Fatalf("status=%q, want in_progress", task.Status)
	}
}

func TestMultiDashboardRendersProjects(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	handler := NewMulti([]ProjectStore{{Name: "fairway", Path: "/tmp/fairway", Store: s}})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"fairway multi-project", "fairway", "T-001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("multi dashboard body missing %q:\n%s", want, body)
		}
	}
}
