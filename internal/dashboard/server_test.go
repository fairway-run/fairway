package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/reconcile"
	"github.com/subashram/fairway/internal/store"
)

func TestIndexRendersDashboardVisibility(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "E-001", Title: "Epic", Kind: "epic", Role: "backend"},
		{
			ID:            "T-001",
			ParentID:      "E-001",
			Title:         "Task",
			Kind:          "facade",
			Role:          "backend",
			Profile:       "platform-foundation",
			OwningDomain:  "platform",
			RiskLevel:     "high",
			ReviewDomains: []string{"architecture", "security"},
		},
		{ID: "T-002", Title: "Other Task", Kind: "bug", Role: "backend", RiskLevel: "low"},
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
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, []WorktreeStatus{{Role: "backend", Branch: "agent/backend", Exists: true, Registered: true}})
	req := httptest.NewRequest(http.MethodGet, "/board?tab=diagnostics", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Sessions", "Worktrees", "Watchers", "Checkpoints", "Workstreams", "E-001", "W-001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard body missing %q:\n%s", want, body)
		}
	}
}

func TestDashboardRoutes(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Role: "backend", Kind: "task"}}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	server := New(s, cfg, []string{"backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "wall-layout") {
		t.Fatalf("/ did not render wall dashboard:\n%s", body)
	}
	if !strings.Contains(body, `class="active" href="/"`) {
		t.Fatalf("/ did not mark wall toggle active:\n%s", body)
	}

	boardReq := httptest.NewRequest(http.MethodGet, "/board", nil)
	boardRec := httptest.NewRecorder()
	server.board(boardRec, boardReq)
	boardBody := boardRec.Body.String()
	if !strings.Contains(boardBody, "board-layout") || !strings.Contains(boardBody, `class="active" href="/board"`) {
		t.Fatalf("/board did not render board surface:\n%s", boardBody)
	}

	wallReq := httptest.NewRequest(http.MethodGet, "/wall", nil)
	wallRec := httptest.NewRecorder()
	server.wallRedirect(wallRec, wallReq)
	if wallRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("/wall status=%d, want %d", wallRec.Code, http.StatusTemporaryRedirect)
	}
	if got := wallRec.Header().Get("Location"); got != "/" {
		t.Fatalf("/wall Location=%q, want /", got)
	}
}

func TestBoardReadyMetricUsesDependencyReadiness(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Claimable", Kind: "task", Role: "backend", Profile: "platform-foundation"},
		{ID: "T-002", Title: "Blocked by dependency", Kind: "task", Role: "backend", Profile: "platform-foundation", Dependencies: []string{"T-003"}},
		{ID: "T-003", Title: "Dependency", Kind: "task", Role: "backend", Profile: "platform-foundation"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-003", "in_progress", "", false); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "<div><b>1</b><span>Ready</span></div>") {
		t.Fatalf("dashboard ready metric did not match dependency-ready count:\n%s", body)
	}
	if !strings.Contains(body, "1 active · 1 ready") {
		t.Fatalf("workstream ready count did not match dependency-ready count:\n%s", body)
	}
}

func TestBoardFiltersByProfileMetadata(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{
			ID:            "T-001",
			Title:         "Platform facade",
			Kind:          "facade",
			Role:          "backend",
			Profile:       "platform-foundation",
			OwningDomain:  "platform",
			RiskLevel:     "high",
			ReviewDomains: []string{"architecture"},
		},
		{ID: "T-002", Title: "Billing bug", Kind: "bug", Role: "backend", OwningDomain: "billing", RiskLevel: "low", ReviewDomains: []string{"backend"}},
	}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?profile=platform-foundation&review_domain=architecture", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Platform facade", "platform-foundation", "architecture"} {
		if !strings.Contains(body, want) {
			t.Fatalf("filtered dashboard body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Billing bug") {
		t.Fatalf("filtered dashboard body included unmatching task:\n%s", body)
	}
}

func TestBoardFiltersBySearchAndStatus(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "DOC-001", Title: "Portal source metadata", Kind: "content-entry", Role: "docs", SourcePaths: []string{"doc/architecture/platform-foundation/README.md"}},
		{ID: "DOC-002", Title: "API reference page", Kind: "content-entry", Role: "docs"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DOC-001", "in_progress", "", false); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"docs"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?q=platform-foundation&status=in_progress", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Portal source metadata", `value="platform-foundation"`, "<b>Status</b> in_progress", "showing 1-1 of 1 filtered tasks"} {
		if !strings.Contains(body, want) {
			t.Fatalf("search dashboard body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "API reference page") {
		t.Fatalf("search dashboard body included unmatching task:\n%s", body)
	}
}

func TestBoardFiltersByRole(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "B-001", Title: "Backend task", Kind: "task", Role: "backend"},
		{ID: "U-001", Title: "UI task", Kind: "task", Role: "ui"},
	}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend", "ui"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?role=backend", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Backend task", "<b>Role</b> backend", "showing 1-1 of 1 filtered tasks"} {
		if !strings.Contains(body, want) {
			t.Fatalf("role-filtered board missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "UI task") {
		t.Fatalf("role-filtered board included unmatching task:\n%s", body)
	}
}

func TestBoardFiltersByMultipleStatuses(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Todo task", Kind: "task", Role: "backend"},
		{ID: "T-002", Title: "Blocked task", Kind: "task", Role: "backend"},
		{ID: "T-003", Title: "Active task", Kind: "task", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-002", "blocked", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-003", "in_progress", "", false); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?status=todo&status=blocked", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Todo task", "Blocked task", "<b>Status</b> todo, blocked", "showing 1-2 of 2 filtered tasks", `name="status" value="todo"`, `name="status" value="blocked"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("multi-status dashboard missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Active task") {
		t.Fatalf("multi-status dashboard included unmatching task:\n%s", body)
	}
}

func TestBoardRendersProfileGateReadiness(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Has source doc evidence", Kind: "content-entry", Role: "docs"},
		{ID: "T-002", Title: "Missing source doc evidence", Kind: "content-entry", Role: "docs"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{
		CommandText:  "make docs-portal-check",
		Result:       "pass",
		ArtifactPath: "packages/docs",
		ArtifactType: "source-doc-check",
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	cfg.WorkstreamProfiles = []config.WorkstreamProfile{{
		Name:      "docusaurus-portal",
		TaskKinds: []string{"content-entry"},
		Gates: []config.WorkstreamProfileGate{{
			Name:                  "source-docs-linked",
			Group:                 "content coverage",
			Mode:                  "blocking",
			EvidenceType:          "source-doc-check",
			RequiredEvidenceCount: 1,
			AcceptedResults:       []string{"pass"},
			ArtifactRequired:      true,
			Description:           "Each migrated page links back to source docs.",
		}},
	}}
	server := New(s, cfg, []string{"docs"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"Fairway control room",
		"Workstream Dashboard",
		"filtered: 2 / 2",
		"Gate Readiness",
		"profile gates evaluated against filtered tasks",
		"docusaurus-portal / content coverage",
		"1 gate(s)",
		"1 blocking missing",
		"docusaurus-portal / source-docs-linked",
		"1/2 satisfied",
		"Workstream Progress",
		"Task Table",
		"T-002",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("gate dashboard body missing %q:\n%s", want, body)
		}
	}
}

func TestBoardRendersGroupedGateRollupsExceptionFirst(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "PF-001", Title: "Ownership map", Kind: "architecture-map", Role: "orchestrator"},
		{ID: "PF-002", Title: "Boundary guard", Kind: "boundary-guard", Role: "governance"},
		{ID: "PF-003", Title: "Evidence facade", Kind: "facade", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "PF-001", store.Evidence{CommandText: "fairway packet template architecture-map", Result: "pass", ArtifactType: "ownership-map", ArtifactPath: "dist/maps.json"}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	cfg.WorkstreamProfiles = []config.WorkstreamProfile{{
		Name:      "platform-foundation",
		TaskKinds: []string{"architecture-map", "boundary-guard", "facade"},
		Gates: []config.WorkstreamProfileGate{
			{Name: "ownership-map-recorded", Group: "maps", Mode: "blocking", TaskKinds: []string{"architecture-map"}, EvidenceType: "ownership-map", RequiredEvidenceCount: 1, AcceptedResults: []string{"pass"}, ArtifactRequired: true},
			{Name: "boundary-guard-report", Group: "guards", Mode: "blocking", TaskKinds: []string{"boundary-guard"}, EvidenceType: "guard-report", RequiredEvidenceCount: 1, AcceptedResults: []string{"pass"}, ArtifactRequired: true},
			{Name: "facade-review", Group: "facades", Mode: "advisory", TaskKinds: []string{"facade"}, EvidenceType: "review", RequiredEvidenceCount: 1, AcceptedResults: []string{"pass"}},
		},
	}}
	server := New(s, cfg, []string{"orchestrator", "governance", "backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"platform-foundation / guards",
		"platform-foundation / facades",
		"platform-foundation / maps",
		"1 gate(s)",
		"1 blocking missing",
		"1 advisory missing",
		"platform-foundation / boundary-guard-report",
		"platform-foundation / facade-review",
		"platform-foundation / ownership-map-recorded",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("grouped gate dashboard body missing %q:\n%s", want, body)
		}
	}
	guards := strings.Index(body, "platform-foundation / guards")
	facades := strings.Index(body, "platform-foundation / facades")
	maps := strings.Index(body, "platform-foundation / maps")
	if !(guards >= 0 && facades > guards && maps > facades) {
		t.Fatalf("gate groups not sorted exception-first: guards=%d facades=%d maps=%d", guards, facades, maps)
	}
}

func TestBoardProfileGateReadinessRespectsGateTaskKinds(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "PF-001", Title: "Ownership map", Kind: "architecture-map", Role: "orchestrator"},
		{ID: "PF-002", Title: "Boundary guard", Kind: "boundary-guard", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	cfg.WorkstreamProfiles = []config.WorkstreamProfile{{
		Name:      "platform-foundation",
		TaskKinds: []string{"architecture-map", "boundary-guard"},
		Gates: []config.WorkstreamProfileGate{{
			Name:                  "boundary-guard-report",
			Mode:                  "advisory",
			TaskKinds:             []string{"boundary-guard"},
			EvidenceType:          "guard-report",
			RequiredEvidenceCount: 1,
			AcceptedResults:       []string{"pass"},
			ArtifactRequired:      true,
		}},
	}}
	server := New(s, cfg, []string{"orchestrator", "backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"platform-foundation / boundary-guard-report", "0/1 satisfied", "PF-002"} {
		if !strings.Contains(body, want) {
			t.Fatalf("gate task-kind dashboard body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "PF-001</a> Ownership map <span class=\"muted\">(architecture-map") {
		t.Fatalf("gate scoped to boundary-guard included architecture-map task:\n%s", body)
	}
}

func TestBoardFiltersActivityAndLimitsRows(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var defs []store.TaskDefinition
	for i := 1; i <= 4; i++ {
		defs = append(defs, store.TaskDefinition{ID: fmt.Sprintf("T-%03d", i), Title: fmt.Sprintf("Task %d", i), Kind: "content-entry", Role: "docs"})
	}
	if err := s.ImportTasks(ctx, defs); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "make docs-portal-check", Result: "pass", ArtifactType: "docs-build"}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"docs"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?activity_kind=evidence&activity_limit=1&table_limit=2", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"showing 1 of 1", "make docs-portal-check", "showing 1-2 of 4 filtered tasks", "page 1 of 2", `href="/board?activity_kind=evidence&amp;activity_limit=1&amp;page=2&amp;table_limit=2"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("limited dashboard body missing %q:\n%s", want, body)
		}
	}
}

func TestTaskDetailRendersMetadata(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{
		ID:            "T-001",
		Title:         "Task",
		Kind:          "architecture-map",
		Role:          "arch",
		Profile:       "platform-foundation",
		OwningDomain:  "platform",
		OwningLayer:   "architecture",
		SourcePaths:   []string{"cmd/api"},
		TargetPaths:   []string{"doc/architecture/platform-foundation/ownership.md"},
		ReviewDomains: []string{"architecture"},
		RiskLevel:     "medium",
		MigrationType: "ownership-map",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "tmux-arch", Role: "arch", SessionBackend: "tmux", Provider: "claude", TaskID: "T-001", TmuxPane: "%1", TranscriptPath: ".fairway/transcripts/T-001.log", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"arch"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tasks/T-001", nil)
	req.Header.Set("Referer", "http://example.com/board?role=arch&status=in_progress")
	rec := httptest.NewRecorder()
	server.task(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"fairway", "dashboard", "/assets/css/detail.css", "Task", "detail", "arch", "Wall", `href="/board?role=arch&amp;status=in_progress"`, `href="/board"`, `href="/board?tab=diagnostics"`, "Metadata", "platform-foundation", "platform", "architecture", "cmd/api", "ownership-map", "tmux-arch", ".fairway/transcripts/T-001.log", "Missing required domains"} {
		if !strings.Contains(body, want) {
			t.Fatalf("task detail body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<style>") {
		t.Fatalf("task detail should not render legacy inline styles:\n%s", body)
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
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
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

func TestDashboardSetStatusRequiresCSRFAndAudits(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	server := New(s, cfg, []string{"backend"}, nil)
	badReq := httptest.NewRequest(http.MethodPost, "/actions/set-status", strings.NewReader("task_id=T-001&status=blocked&csrf=bad"))
	badReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badRec := httptest.NewRecorder()
	server.setStatus(badRec, badReq)
	if badRec.Code != http.StatusForbidden {
		t.Fatalf("bad csrf status=%d, want 403", badRec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/actions/set-status", strings.NewReader("task_id=T-001&status=blocked&reason=waiting&csrf="+server.csrfToken))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.setStatus(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("set-status status=%d, want 303 body=%s", rec.Code, rec.Body.String())
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "blocked" {
		t.Fatalf("status=%q, want blocked", task.Status)
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

func TestDashboardAssetsServeTokens(t *testing.T) {
	handler := dashboardAssetHandler()
	req := httptest.NewRequest(http.MethodGet, "/assets/css/tokens.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("tokens.css status=%d, want 200 body=%s", rec.Code, body)
	}
	for _, want := range []string{`[data-theme="dark"]`, `[data-theme="light"]`, "--claude:", "--codex:", "--compass:", "--transition-base:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tokens.css missing %q:\n%s", want, body)
		}
	}
}

func TestDashboardAssetsServeComponents(t *testing.T) {
	handler := dashboardAssetHandler()
	req := httptest.NewRequest(http.MethodGet, "/assets/css/components.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("components.css status=%d, want 200 body=%s", rec.Code, body)
	}
	for _, want := range []string{".pill", ".status-pill", ".btn", ".card", ".modal", ".gauge-ring", ".ticker-entry", ".theme-toggle", ".view-toggle", ".dropdown-menu"} {
		if !strings.Contains(body, want) {
			t.Fatalf("components.css missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "#") {
		t.Fatalf("components.css should consume tokens only; found hardcoded color in:\n%s", body)
	}
	if regexp.MustCompile(`[0-9]px\b`).MatchString(body) {
		t.Fatalf("components.css should consume tokens only; found hardcoded pixel unit in:\n%s", body)
	}
}

func TestDashboardAssetsServeLogo(t *testing.T) {
	handler := dashboardAssetHandler()
	req := httptest.NewRequest(http.MethodGet, "/assets/logo.svg", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("logo.svg status=%d, want 200 body=%s", rec.Code, body)
	}
	for _, want := range []string{`aria-label="fairway"`, `viewBox="0 0 24 24"`, `currentColor`} {
		if !strings.Contains(body, want) {
			t.Fatalf("logo.svg missing %q:\n%s", want, body)
		}
	}
}

func TestWallTemplateRendersRoleLanesAndProviders(t *testing.T) {
	data := struct {
		View                 string
		Groups               []RoleGroup
		MissingReviewDomains map[string][]string
		GateGroups           []GateGroup
		Sessions             []store.Session
		Checkpoints          []store.Checkpoint
		Activity             []store.Activity
		TaskRoles            map[string]string
		ActiveReport         reconcile.ActiveReport
	}{
		View: "wall",
		Groups: []RoleGroup{
			{
				Role: "backend",
				Current: &store.Task{
					Definition: store.TaskDefinition{ID: "T-002", Title: "Working task", Role: "backend"},
					Status:     "in_progress",
				},
				Tasks: []store.Task{
					{Definition: store.TaskDefinition{ID: "T-001", Title: "Backlog task", Role: "backend"}, Status: "todo"},
					{Definition: store.TaskDefinition{ID: "T-002", Title: "Working task", Role: "backend"}, Status: "in_progress"},
					{Definition: store.TaskDefinition{ID: "T-003", Title: "Done task", Role: "backend"}, Status: "done"},
					{Definition: store.TaskDefinition{ID: "T-004", Title: "More backlog one", Role: "backend"}, Status: "todo"},
					{Definition: store.TaskDefinition{ID: "T-005", Title: "More backlog two", Role: "backend"}, Status: "todo"},
					{Definition: store.TaskDefinition{ID: "T-006", Title: "More backlog three", Role: "backend"}, Status: "todo"},
					{Definition: store.TaskDefinition{ID: "T-007", Title: "Domain review task", Role: "backend", ReviewDomains: []string{"ops", "backend"}}, Status: "done", ReviewStatus: "approved"},
				},
			},
			{Role: "ui"},
		},
		MissingReviewDomains: map[string][]string{"T-007": {"ops", "backend"}},
		GateGroups:           []GateGroup{{Label: "dashboard-v2 / foundation", TaskCount: 2, SatisfiedCount: 1, MissingTaskCount: 1}},
		Sessions:             []store.Session{{ID: "s-1", Role: "backend", Provider: "codex", TaskID: "T-002", Status: "running"}},
		TaskRoles:            map[string]string{"T-002": "backend"},
		Checkpoints: []store.Checkpoint{{
			TaskID:  "T-002",
			State:   "active",
			Owner:   "backend",
			Summary: "provider session attached",
		}},
		Activity: []store.Activity{{Kind: "handoff", TaskID: "T-002", Summary: "backend to ui", CreatedAt: "2026-06-04T00:00:00Z"}},
		ActiveReport: reconcile.ActiveReport{
			Findings: []reconcile.ActiveFinding{{Kind: "unattended_in_progress", TaskID: "T-004"}},
			Summary:  reconcile.ActiveSummary{UnattendedInProgress: 1},
		},
	}
	var out strings.Builder
	if err := wallTemplate.ExecuteTemplate(&out, "layout", data); err != nil {
		t.Fatalf("wall template render error = %v", err)
	}
	body := out.String()
	for _, want := range []string{
		"dashboard",
		`/assets/logo.svg`,
		`aria-label="fairway dashboard"`,
		`data-theme-toggle`,
		`/assets/js/common.js`,
		`/assets/css/wall.css`,
		"backend",
		"ui",
		"backlog",
		"in progress / no session",
		"active session",
		"review",
		"done",
		"codex -&gt; backend / T-002",
		"Active Sessions",
		"active: provider session attached",
		"Active work needs reconciliation",
		"1 unattended in_progress",
		"idle . waiting",
		"Open lane",
		`href="/board?role=backend"`,
		`href="/board?role=backend&amp;status=todo"`,
		"dashboard-v2 / foundation",
		"Backlog task",
		"Working task",
		"Done task",
		"Domain review task",
		"missing",
		"<code>ops</code>",
		"<code>backend</code>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("wall template missing %q:\n%s", want, body)
		}
	}
}

func TestWallLanesPreferRecentlyUpdatedActiveTasks(t *testing.T) {
	tasks := []store.Task{
		{Definition: store.TaskDefinition{ID: "OLD-001", Title: "Old active", Role: "backend"}, Status: "in_progress", UpdatedAt: "2026-06-04T10:00:00Z"},
		{Definition: store.TaskDefinition{ID: "NEW-001", Title: "New active", Role: "backend"}, Status: "in_progress", UpdatedAt: "2026-06-04T23:00:00Z"},
		{Definition: store.TaskDefinition{ID: "TODO-001", Title: "Todo", Role: "backend"}, Status: "todo", UpdatedAt: "2026-06-04T22:00:00Z"},
	}
	groups := groupTasks(tasks, []string{"backend"})
	if groups[0].Current == nil || groups[0].Current.Definition.ID != "NEW-001" {
		t.Fatalf("current = %#v, want NEW-001", groups[0].Current)
	}
	claimed := wallLaneTasks(groups[0].Tasks, "claimed", nil, nil)
	if len(claimed) != 2 {
		t.Fatalf("claimed count = %d, want 2", len(claimed))
	}
	if claimed[0].Definition.ID != "NEW-001" {
		t.Fatalf("first claimed task = %s, want NEW-001", claimed[0].Definition.ID)
	}
	review := wallLaneTasks([]store.Task{
		{Definition: store.TaskDefinition{ID: "NO-REVIEW", Role: "backend"}, Status: "in_progress", ReviewStatus: "not_required"},
		{Definition: store.TaskDefinition{ID: "PENDING", Role: "backend"}, Status: "in_progress", ReviewStatus: "pending"},
		{Definition: store.TaskDefinition{ID: "DONE-MISSING", Role: "backend", ReviewDomains: []string{"ops"}}, Status: "done", ReviewStatus: "approved"},
		{Definition: store.TaskDefinition{ID: "ACTIVE-MISSING", Role: "backend", ReviewDomains: []string{"backend"}}, Status: "in_progress", ReviewStatus: "approved"},
	}, "review", nil, map[string][]string{
		"DONE-MISSING":   {"ops"},
		"ACTIVE-MISSING": {"backend"},
	})
	if len(review) != 3 {
		t.Fatalf("review task count = %d, want 3: %#v", len(review), review)
	}
	gotReviewIDs := []string{review[0].Definition.ID, review[1].Definition.ID, review[2].Definition.ID}
	for _, want := range []string{"PENDING", "DONE-MISSING", "ACTIVE-MISSING"} {
		if !containsString(gotReviewIDs, want) {
			t.Fatalf("review tasks = %#v, missing %s", gotReviewIDs, want)
		}
	}
	activeSessionTasks := wallLaneTasks([]store.Task{
		{Definition: store.TaskDefinition{ID: "DONE-ACTIVE", Role: "backend"}, Status: "done", UpdatedAt: "2026-06-04T20:00:00Z"},
	}, "working", []store.Session{{TaskID: "DONE-ACTIVE", Provider: "codex", Status: "running"}}, nil)
	if len(activeSessionTasks) != 1 || activeSessionTasks[0].Definition.ID != "DONE-ACTIVE" {
		t.Fatalf("active session tasks = %#v, want DONE-ACTIVE even when task is done", activeSessionTasks)
	}
}

func TestWallDoneTodayUsesLocalDashboardDate(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	groups := []RoleGroup{{
		Role: "backend",
		Tasks: []store.Task{
			{Definition: store.TaskDefinition{ID: "DONE-TODAY", Role: "backend"}, Status: "done", UpdatedAt: today + "T12:00:00Z"},
			{Definition: store.TaskDefinition{ID: "DONE-TODAY-FALLBACK", Role: "backend"}, Status: "done", UpdatedAt: today + " local"},
			{Definition: store.TaskDefinition{ID: "DONE-YESTERDAY", Role: "backend"}, Status: "done", UpdatedAt: yesterday + "T12:00:00Z"},
			{Definition: store.TaskDefinition{ID: "TODO-TODAY", Role: "backend"}, Status: "todo", UpdatedAt: today + "T12:00:00Z"},
		},
	}}
	if got := wallDoneToday(groups); got != 2 {
		t.Fatalf("wallDoneToday = %d, want 2", got)
	}
}

func TestBoardTemplateRendersToolbarTableAndRail(t *testing.T) {
	data := DashboardViewData{
		View:    "board",
		Summary: DashboardSummary{Total: 2, Filtered: 2, Ready: 1, InProgress: 1, Workstreams: 1},
		Groups: []RoleGroup{{
			Role: "backend",
			Tasks: []store.Task{
				{Definition: store.TaskDefinition{ID: "T-001", Title: "Backlog task", Role: "backend", Kind: "task"}, Status: "todo", Owner: "backend"},
				{Definition: store.TaskDefinition{ID: "T-002", Title: "Working task", Role: "backend", Kind: "dashboard"}, Status: "in_progress", Owner: "ui"},
			},
		}},
		TableRows: []store.Task{
			{Definition: store.TaskDefinition{ID: "T-001", Title: "Backlog task", Role: "backend", Kind: "task"}, Status: "todo", Owner: "backend"},
			{Definition: store.TaskDefinition{ID: "T-002", Title: "Working task", Role: "backend", Kind: "dashboard"}, Status: "in_progress", Owner: "ui"},
		},
		Pagination:       TablePagination{Page: 1, PageSize: 25, TotalRows: 2, TotalPages: 1, Start: 1, End: 2},
		GateGroups:       []GateGroup{{Label: "dashboard-v2 / foundation", TaskCount: 2, SatisfiedCount: 1, MissingTaskCount: 1}},
		Workstreams:      []WorkstreamGroup{{Label: "dashboard-v2 / task", Total: 2, Done: 1, Ready: 1, InProgress: 1}},
		Sessions:         []store.Session{{ID: "s-1", Role: "backend", Provider: "codex", TaskID: "T-002", Status: "running"}},
		Activity:         []store.Activity{{Kind: "evidence", TaskID: "T-002", Summary: "test pass", CreatedAt: "2026-06-04T00:00:00Z"}},
		Filters:          TaskFilters{Search: "dashboard", Role: "backend", Status: "todo", Profile: "dashboard-v2", Kind: "task", OwningDomain: "fairway", RiskLevel: "medium", Tab: "tasks", ActivityLimit: 25},
		Health:           store.Health{InProgress: 1},
		StaleCheckpoints: []store.Checkpoint{{TaskID: "T-001", State: "active"}},
		Watchers:         []store.Watcher{{ID: "W-001", TaskID: "T-001", Status: "active"}},
		Rollups:          map[string]Rollup{"T-001": {Done: 1, Total: 2}},
	}
	var out strings.Builder
	if err := boardTemplate.ExecuteTemplate(&out, "layout", data); err != nil {
		t.Fatalf("board template render error = %v", err)
	}
	body := out.String()
	for _, want := range []string{
		`class="active" href="/board"`,
		`/assets/js/common.js`,
		`/assets/js/board.js`,
		`/assets/css/board.css`,
		"Search ID, title, owner, path",
		"Diagnostics",
		"<b>Role</b> backend",
		"stale claims",
		"Workstreams",
		"Export CSV",
		"selection-bar",
		"ID",
		"Title",
		"Role",
		"Status",
		"Kind",
		"Started",
		"Last activity",
		"Gates",
		"Owner",
		"Backlog task",
		"Working task",
		"showing 1-2 of 2 filtered tasks",
		"page 1 of 1",
		"dashboard-v2 / foundation",
		"evidence test pass",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("board template missing %q:\n%s", want, body)
		}
	}
}

func TestBoardTemplateRendersDiagnosticsTab(t *testing.T) {
	data := DashboardViewData{
		View:    "board",
		Summary: DashboardSummary{Total: 4, Ready: 1, InProgress: 1, Blocked: 1, Done: 1, Workstreams: 2},
		Filters: TaskFilters{
			Tab:           "diagnostics",
			ActivityLimit: 25,
		},
		Health:           store.Health{InProgress: 1, StaleInProgress: 1, BlockedOver24h: 1, UnacknowledgedOver1Hour: 1, UnroutedReviews: 1},
		Sessions:         []store.Session{{ID: "s-1", Role: "backend", Status: "running", Branch: "agent/backend", TaskID: "T-001", SessionBackend: "tmux", Provider: "codex"}},
		Worktrees:        []WorktreeStatus{{Role: "backend", Branch: "agent/backend", Exists: true, Registered: true, Dirty: true, LastCommit: "abc123", Path: "/tmp/backend"}},
		Watchers:         []store.Watcher{{ID: "W-001", TaskID: "T-001", Status: "active", Owner: "ops", Process: "smoke", Command: "make test"}},
		Checkpoints:      []store.Checkpoint{{TaskID: "T-001", State: "active", Owner: "backend", TargetCloseBy: "today", Summary: "working"}},
		StaleCheckpoints: []store.Checkpoint{{TaskID: "T-002", State: "active", Owner: "ui", Summary: "stale"}},
		FilterOptions:    FilterOptions{ActivityKinds: []string{"checkpoint", "evidence"}},
		Activity:         []store.Activity{{Kind: "checkpoint", TaskID: "T-001", Summary: "working", CreatedAt: "2026-06-04T00:00:00Z"}},
		ActivityTotal:    1,
	}
	var out strings.Builder
	if err := boardTemplate.ExecuteTemplate(&out, "layout", data); err != nil {
		t.Fatalf("board diagnostics render error = %v", err)
	}
	body := out.String()
	for _, want := range []string{
		`href="/board?tab=diagnostics"`,
		"Sessions",
		"Worktrees",
		"Watchers",
		"Checkpoints",
		"s-1",
		"agent/backend",
		"W-001",
		"stale checkpoints: 1",
		"blocked &gt;24h: 1",
		"checkpoint working",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("board diagnostics missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "board-table") {
		t.Fatalf("diagnostics tab should not render task table:\n%s", body)
	}
}

func TestDashboardAssetsServeViewToggleScripts(t *testing.T) {
	handler := dashboardAssetHandler()
	for _, tt := range []struct {
		path string
		want []string
	}{
		{path: "/assets/js/common.js", want: []string{"fairway.dashboard.theme", "data-theme-toggle", "localStorage"}},
		{path: "/assets/js/board.js", want: []string{`event.key === "g"`, `event.key === "w"`, `window.location.assign("/")`}},
	} {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		body := rec.Body.String()
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d, want 200 body=%s", tt.path, rec.Code, body)
		}
		for _, want := range tt.want {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q:\n%s", tt.path, want, body)
			}
		}
	}
}

func TestDashboardAssetsServeSurfaceStyles(t *testing.T) {
	handler := dashboardAssetHandler()
	for _, tt := range []struct {
		path string
		want []string
	}{
		{path: "/assets/css/wall.css", want: []string{".wall-layout", ".wall-lane", ".lane-states", ".wall-rail"}},
		{path: "/assets/css/board.css", want: []string{".board-layout", ".control-room-head", ".gate-grid", ".board-table", ".board-rail"}},
	} {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		body := rec.Body.String()
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d, want 200 body=%s", tt.path, rec.Code, body)
		}
		for _, want := range tt.want {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q:\n%s", tt.path, want, body)
			}
		}
	}
}
