package dashboard

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestBoardExportCSVUsesCurrentViewWithoutPaginationTruncation(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Beta", Role: "ui", Kind: "dashboard", Profile: "dashboard-v2", OwningDomain: "fairway", RiskLevel: "low"},
		{ID: "T-002", Title: "Alpha", Role: "ui", Kind: "dashboard", Profile: "dashboard-v2", OwningDomain: "fairway", RiskLevel: "medium"},
		{ID: "T-003", Title: "Gamma", Role: "backend", Kind: "dashboard"},
	}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"ui", "backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board/export?role=ui&sort=title&columns=title,id,risk_level&table_limit=1", nil)
	rec := httptest.NewRecorder()
	server.boardExport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Fairway-Filtered-Rows"); got != "2" {
		t.Fatalf("filtered header=%q, want 2", got)
	}
	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"Title", "ID", "Risk"},
		{"Alpha", "T-002", "medium"},
		{"Beta", "T-001", "low"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows=%v, want %v", rows, want)
	}
	for i := range want {
		for j := range want[i] {
			if rows[i][j] != want[i][j] {
				t.Fatalf("rows=%v, want %v", rows, want)
			}
		}
	}
}

func TestBoardExportJSONUsesVisibleColumns(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Role: "backend", Kind: "task", ReviewDomains: []string{"arch", "ops"}}}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board/export?format=json&columns=id,review_domains,workstream", nil)
	rec := httptest.NewRecorder()
	server.boardExport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload boardExportPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Format != "json" || payload.FilteredRows != 1 || payload.ExportedRows != 1 {
		t.Fatalf("payload summary=%+v", payload)
	}
	if strings.Join(payload.Columns, ",") != "ID,Review domains,Workstream" {
		t.Fatalf("columns=%v", payload.Columns)
	}
	if got := payload.Rows[0]["Review domains"]; got != "arch, ops" {
		t.Fatalf("review domains=%q", got)
	}
}
