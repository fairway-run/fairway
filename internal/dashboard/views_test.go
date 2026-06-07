package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestSavedViewsLoadPersonalAndTeam(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writeViewsFile(t, filepath.Join(home, ".fairway", "views.json"), savedViewFile{Views: []SavedView{
		{Name: "Mine", Query: "status=blocked&columns=id,title&sort=title&page=4"},
	}})
	writeViewsFile(t, filepath.Join(root, ".fairway", "views.json"), savedViewFile{Views: []SavedView{
		{Name: "Team Review", Query: "review_domain=governance&status=done"},
	}})

	personal, team, err := loadDashboardSavedViews(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(personal) != 1 || personal[0].Source != "personal" || personal[0].Shortcut != 1 {
		t.Fatalf("personal views=%+v", personal)
	}
	if personal[0].Href != "/board?columns=id%2Ctitle&sort=title&status=blocked" {
		t.Fatalf("personal href=%q", personal[0].Href)
	}
	if personal[0].Filters["status"][0] != "blocked" || strings.Join(personal[0].Columns, ",") != "id,title" || personal[0].Sort != "title" {
		t.Fatalf("personal parsed metadata=%+v", personal[0])
	}
	if len(team) != 1 || team[0].Source != "team" || team[0].Href != "/board?review_domain=governance&status=done" {
		t.Fatalf("team views=%+v", team)
	}
}

func TestSaveDashboardPersonalViewUpsertsCurrentView(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := saveDashboardPersonalView("Mine", "?status=todo&sort=-updated&page=3&columns=id,title,status"); err != nil {
		t.Fatal(err)
	}
	if _, err := saveDashboardPersonalView("Mine", "status=done&profile=dashboard-v2"); err != nil {
		t.Fatal(err)
	}
	views, err := readSavedViews(filepath.Join(home, ".fairway", "views.json"), "personal")
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 {
		t.Fatalf("views=%+v, want one upserted view", views)
	}
	if views[0].Query != "profile=dashboard-v2&status=done" {
		t.Fatalf("query=%q", views[0].Query)
	}
}

func TestDashboardSaveViewRequiresCSRFAndPersistsPersonalView(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	server := NewWithRoot(s, config.Defaults(root), []string{"backend"}, nil, root)

	badReq := httptest.NewRequest(http.MethodPost, "/actions/views/save", strings.NewReader("name=Mine&csrf=bad"))
	badReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badRec := httptest.NewRecorder()
	server.saveView(badRec, badReq)
	if badRec.Code != http.StatusForbidden {
		t.Fatalf("bad csrf status=%d, want 403", badRec.Code)
	}

	form := url.Values{}
	form.Set("csrf", server.csrfToken)
	form.Set("name", "Blocked")
	form.Set("query", "status=blocked&columns=id,title&sort=title&page=2")
	form.Set("return_to", "/board?status=blocked&page=2")
	req := httptest.NewRequest(http.MethodPost, "/actions/views/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.saveView(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save view status=%d, want 303 body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/board?status=blocked&page=2" {
		t.Fatalf("redirect=%q", got)
	}
	views, err := readSavedViews(filepath.Join(home, ".fairway", "views.json"), "personal")
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Name != "Blocked" || views[0].Query != "columns=id%2Ctitle&sort=title&status=blocked" {
		t.Fatalf("saved views=%+v", views)
	}
}

func TestBoardRendersPersonalAndTeamSavedViews(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writeViewsFile(t, filepath.Join(home, ".fairway", "views.json"), savedViewFile{Views: []SavedView{
		{Name: "Mine", Query: "status=blocked"},
	}})
	writeViewsFile(t, filepath.Join(root, ".fairway", "views.json"), savedViewFile{Views: []SavedView{
		{Name: "Team Done", Query: "status=done"},
	}})
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	server := NewWithRoot(s, config.Defaults(root), []string{"backend"}, nil, root)
	req := httptest.NewRequest(http.MethodGet, "/board?status=blocked&columns=id,title&sort=title&page=2", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"Save current view",
		"Mine",
		"Team Done",
		`data-saved-view-shortcut="1"`,
		`name="query" value="columns=id%2Ctitle&amp;sort=title&amp;status=blocked"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("board saved views missing %q:\n%s", want, body)
		}
	}
}

func writeViewsFile(t *testing.T, path string, file savedViewFile) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
