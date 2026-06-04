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

	"github.com/subashram/fairway/internal/config"
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Sessions", "Worktrees", "Watchers", "Checkpoints", "Workstreams", "platform-foundation / facade", "architecture", "security", "E-001", "W-001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard body missing %q:\n%s", want, body)
		}
	}
}

func TestDashboardRoutesRespectSurfaceConfig(t *testing.T) {
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
	if !strings.Contains(body, "Workstream Dashboard") {
		t.Fatalf("v1 / did not render legacy dashboard:\n%s", body)
	}
	if strings.Contains(body, "wall-layout") {
		t.Fatalf("v1 / rendered wall surface:\n%s", body)
	}

	cfg.Dashboard.Surface = "v2"
	server = New(s, cfg, []string{"backend"}, nil)
	rec = httptest.NewRecorder()
	server.index(rec, req)
	body = rec.Body.String()
	if !strings.Contains(body, "wall-layout") {
		t.Fatalf("v2 / did not render wall dashboard:\n%s", body)
	}
	if !strings.Contains(body, `class="active" href="/"`) {
		t.Fatalf("v2 / did not mark wall toggle active:\n%s", body)
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

func TestIndexReadyMetricUsesDependencyReadiness(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "<div class=\"metric\"><b>1</b><span>Ready</span></div>") {
		t.Fatalf("dashboard ready metric did not match dependency-ready count:\n%s", body)
	}
	if !strings.Contains(body, "<span>1 ready</span>") {
		t.Fatalf("workstream ready count did not match dependency-ready count:\n%s", body)
	}
}

func TestIndexFiltersByProfileMetadata(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/?profile=platform-foundation&review_domain=architecture", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Platform facade", "selected", "architecture"} {
		if !strings.Contains(body, want) {
			t.Fatalf("filtered dashboard body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Billing bug") {
		t.Fatalf("filtered dashboard body included unmatching task:\n%s", body)
	}
}

func TestIndexFiltersBySearchAndStatus(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/?q=platform-foundation&status=in_progress", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Portal source metadata", "value=\"platform-foundation\"", "selected>in_progress", "filtered: 1 / 2"} {
		if !strings.Contains(body, want) {
			t.Fatalf("search dashboard body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "API reference page") {
		t.Fatalf("search dashboard body included unmatching task:\n%s", body)
	}
}

func TestIndexRendersProfileGateReadiness(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Gate Readiness", "docusaurus-portal / content coverage", "1 gate(s)", "1 blocking missing", "docusaurus-portal / source-docs-linked", "1/2 satisfied", "1 missing", "T-002", "needs 1 matching evidence row"} {
		if !strings.Contains(body, want) {
			t.Fatalf("gate dashboard body missing %q:\n%s", want, body)
		}
	}
}

func TestIndexRendersGroupedGateRollupsExceptionFirst(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"platform-foundation / guards",
		"platform-foundation / facades",
		"platform-foundation / maps",
		"1 blocking missing",
		"1 advisory missing",
		"ready",
		"Gate details",
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

func TestIndexProfileGateReadinessRespectsGateTaskKinds(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
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

func TestIndexFiltersActivityAndLimitsRows(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/?activity_kind=evidence&activity_limit=1&table_limit=2", nil)
	rec := httptest.NewRecorder()
	server.index(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"showing 1 of 1", "make docs-portal-check", "showing first 2 of 4 tasks"} {
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
	rec := httptest.NewRecorder()
	server.task(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Metadata", "platform-foundation", "platform", "architecture", "cmd/api", "ownership-map", "tmux-arch", ".fairway/transcripts/T-001.log"} {
		if !strings.Contains(body, want) {
			t.Fatalf("task detail body missing %q:\n%s", want, body)
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

func TestWallTemplateRendersRoleLanesAndProviders(t *testing.T) {
	data := struct {
		View       string
		Groups     []RoleGroup
		GateGroups []GateGroup
		Sessions   []store.Session
		Activity   []store.Activity
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
				},
			},
			{Role: "ui"},
		},
		GateGroups: []GateGroup{{Label: "dashboard-v2 / foundation", TaskCount: 2, SatisfiedCount: 1, MissingTaskCount: 1}},
		Sessions:   []store.Session{{ID: "s-1", Role: "backend", Provider: "codex", TaskID: "T-002", Status: "running"}},
		Activity:   []store.Activity{{Kind: "handoff", TaskID: "T-002", Summary: "backend to ui", CreatedAt: "2026-06-04T00:00:00Z"}},
	}
	var out strings.Builder
	if err := wallTemplate.ExecuteTemplate(&out, "layout", data); err != nil {
		t.Fatalf("wall template render error = %v", err)
	}
	body := out.String()
	for _, want := range []string{
		"dashboard v2",
		`data-theme-toggle`,
		`/assets/js/common.js`,
		`/assets/css/wall.css`,
		"backend",
		"ui",
		"backlog",
		"claimed",
		"working",
		"review",
		"done",
		"codex",
		"idle . waiting",
		"dashboard-v2 / foundation",
		"Backlog task",
		"Working task",
		"Done task",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("wall template missing %q:\n%s", want, body)
		}
	}
}

func TestBoardTemplateRendersToolbarTableAndRail(t *testing.T) {
	data := struct {
		View       string
		Summary    DashboardSummary
		Groups     []RoleGroup
		GateGroups []GateGroup
		Sessions   []store.Session
		Activity   []store.Activity
		Filters    TaskFilters
		Rollups    map[string]Rollup
	}{
		View:    "board",
		Summary: DashboardSummary{Filtered: 2},
		Groups: []RoleGroup{{
			Role: "backend",
			Tasks: []store.Task{
				{Definition: store.TaskDefinition{ID: "T-001", Title: "Backlog task", Role: "backend", Kind: "task"}, Status: "todo", Owner: "backend"},
				{Definition: store.TaskDefinition{ID: "T-002", Title: "Working task", Role: "backend", Kind: "dashboard"}, Status: "in_progress", Owner: "ui"},
			},
		}},
		GateGroups: []GateGroup{{Label: "dashboard-v2 / foundation", TaskCount: 2, SatisfiedCount: 1, MissingTaskCount: 1}},
		Sessions:   []store.Session{{ID: "s-1", Role: "backend", Provider: "codex", TaskID: "T-002", Status: "running"}},
		Activity:   []store.Activity{{Kind: "evidence", TaskID: "T-002", Summary: "test pass", CreatedAt: "2026-06-04T00:00:00Z"}},
		Filters:    TaskFilters{Search: "dashboard", Status: "todo", Profile: "dashboard-v2", Kind: "task", OwningDomain: "fairway", RiskLevel: "medium"},
		Rollups:    map[string]Rollup{"T-001": {Done: 1, Total: 2}},
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
		"Search tasks",
		"Columns",
		"Views",
		"Export",
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
		"showing 2 of 2 filtered tasks",
		"dashboard-v2 / foundation",
		"evidence test pass",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("board template missing %q:\n%s", want, body)
		}
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
		{path: "/assets/css/board.css", want: []string{".board-layout", ".toolbar", ".board-table", ".board-rail"}},
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
