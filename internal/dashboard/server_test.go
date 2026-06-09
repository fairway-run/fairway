package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
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
	if err := s.SetTaskIDPattern(`^[A-Z]+(?:-[A-Z]+)?-[0-9]+$`); err != nil {
		t.Fatal(err)
	}
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
			Tags:          []string{"production-readiness", "environment:cloudflare"},
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
	req := httptest.NewRequest(http.MethodGet, "/board?tag=production-readiness", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Workstream Dashboard", "production-readiness", "environment:cloudflare", "T-001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `data-task-href="/tasks/T-002"`) {
		t.Fatalf("tag-filtered dashboard included untagged task:\n%s", body)
	}
	req = httptest.NewRequest(http.MethodGet, "/board?tab=diagnostics", nil)
	rec = httptest.NewRecorder()
	server.board(rec, req)
	body = rec.Body.String()
	for _, want := range []string{"Sessions", "Worktrees", "Watchers", "Checkpoints", "Workstreams", "E-001", "W-001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("diagnostics body missing %q:\n%s", want, body)
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

	reportsReq := httptest.NewRequest(http.MethodGet, "/reports", nil)
	reportsRec := httptest.NewRecorder()
	server.reports(reportsRec, reportsReq)
	reportsBody := reportsRec.Body.String()
	if !strings.Contains(reportsBody, "reports-layout") || !strings.Contains(reportsBody, `class="active" href="/reports"`) {
		t.Fatalf("/reports did not render reports surface:\n%s", reportsBody)
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

func TestReportsRetrospectiveSeparatesDeliveryAndBookkeeping(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SetTaskIDPattern(`^[A-Z]+(?:-[A-Z]+)?-[0-9]+$`); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Ship portal route", Kind: "feature", Role: "backend", OwningDomain: "portal", ReviewDomains: []string{"architecture"}},
		{ID: "W-001", Title: "CI monitor bookkeeping", Kind: "ci-monitor", Role: "ops/watch", OwningDomain: "release"},
		{ID: "CI-FIX-001", Title: "Fix generated client drift", Kind: "bugfix", Role: "backend", OwningDomain: "ci"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "done", "done today", false); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass", ArtifactType: "local-test", ArtifactPath: "dist/test.log"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "W-001", "done", "ci passed", false); err != nil {
		t.Fatal(err)
	}
	if err := s.StartWatcher(ctx, store.Watcher{ID: "watch-ci", TaskID: "W-001", Owner: "ops/watch", Process: "ci", Command: "gh run watch"}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishWatcher(ctx, "watch-ci", "pass", "gh-run-1", nil, "green"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "W-001", store.Evidence{CommandText: "gh run view 1", Result: "pass", ArtifactType: "ci_pipeline", ArtifactPath: "gh-run-1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", store.Review{Reviewer: "governance", Verdict: "approve", Reason: "ok"}); err != nil {
		t.Fatal(err)
	}
	total := 150
	cached := 40
	if _, err := s.RecordProviderUsage(ctx, store.ProviderUsage{Provider: "codex", TaskID: "T-001", Role: "backend", Phase: "implementation", Source: "provider_reported", Confidence: "exact", TotalTokens: &total, CachedInputTokens: &cached}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertWorkBatch(ctx, store.WorkBatch{ID: "BATCH-001", Title: "Portal validation batch", Branch: "feature/portal", Tasks: []string{"T-001", "W-001"}}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend", "ops/watch"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rec := httptest.NewRecorder()
	server.reports(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"Daily Report",
		"Delivery done",
		"Monitor/deploy bookkeeping",
		"Ship portal route",
		"portal / feature",
		"CI, Deploy, And UAT Timeline",
		"watch-ci",
		"CI-FIX",
		"Missing Required Review Domains",
		"Provider Usage Attribution",
		"Work batches / batched tasks",
		"codex",
		"150",
		"Markdown",
		"JSON",
		"CSV",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("reports body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<td>W-001</td>") || strings.Contains(body, `href="/tasks/W-001">W-001</a></td>`) {
		t.Fatalf("default report drill-down table included monitor bookkeeping:\n%s", body)
	}
	req = httptest.NewRequest(http.MethodGet, "/reports?include_bookkeeping=1", nil)
	rec = httptest.NewRecorder()
	server.reports(rec, req)
	if body = rec.Body.String(); !strings.Contains(body, "CI monitor bookkeeping") {
		t.Fatalf("include_bookkeeping report did not include monitor row:\n%s", body)
	}
}

func TestReportsExportsMatchFilteredScope(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SetTaskIDPattern(`^[A-Z]+(?:-[A-Z]+)?-[0-9]+$`); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "DOC-FIX-001", Title: "Portal docs correction", Kind: "docs", Role: "governance", OwningDomain: "portal", Tags: []string{"docs-portal", "environment:cloudflare"}},
		{ID: "OPS-FIX-001", Title: "Release job correction", Kind: "ops", Role: "ops", OwningDomain: "release", Tags: []string{"production-readiness"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "DOC-FIX-001", "done", "done", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "OPS-FIX-001", "done", "done", false); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"governance", "ops"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/reports?q=Portal&format=csv", nil)
	rec := httptest.NewRecorder()
	server.reports(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "DOC-FIX-001") {
		t.Fatalf("CSV export missing filtered report row:\n%s", body)
	}
	if strings.Contains(body, "OPS-FIX-001") {
		t.Fatalf("CSV export included row outside filtered scope:\n%s", body)
	}
	req = httptest.NewRequest(http.MethodGet, "/reports?tag=docs-portal&format=csv", nil)
	rec = httptest.NewRecorder()
	server.reports(rec, req)
	body = rec.Body.String()
	if !strings.Contains(body, "DOC-FIX-001") || !strings.Contains(body, "docs-portal;environment:cloudflare") {
		t.Fatalf("tag-filtered CSV export missing tagged report row:\n%s", body)
	}
	if strings.Contains(body, "OPS-FIX-001") {
		t.Fatalf("tag-filtered CSV export included row outside tag scope:\n%s", body)
	}
}

func TestReportWindowLocalDayBoundaries(t *testing.T) {
	loc := time.FixedZone("test", -6*60*60)
	now := time.Date(2026, 6, 6, 0, 30, 0, 0, loc)
	window, start, end := reportWindowFromQuery(nil, now, loc)
	if window.StartDate != "2026-06-06" || window.EndDate != "2026-06-06" {
		t.Fatalf("today window=%+v", window)
	}
	if got := end.Sub(start); got != 24*time.Hour {
		t.Fatalf("today duration=%s, want 24h", got)
	}
	values := make(url.Values)
	values.Set("range", "last7")
	window, start, end = reportWindowFromQuery(values, now, loc)
	if window.StartDate != "2026-05-31" || window.EndDate != "2026-06-06" || end.Sub(start) != 7*24*time.Hour {
		t.Fatalf("last7 window=%+v duration=%s", window, end.Sub(start))
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

func TestBoardWorkstreamsCompactAndExpandWithFilterState(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var defs []store.TaskDefinition
	for i := 1; i <= 15; i++ {
		defs = append(defs, store.TaskDefinition{
			ID:      fmt.Sprintf("WS-%03d", i),
			Title:   fmt.Sprintf("Dashboard workstream %02d", i),
			Kind:    fmt.Sprintf("kind-%02d", i),
			Role:    "ui",
			Profile: "dashboard-v2",
		})
	}
	if err := s.ImportTasks(ctx, defs); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "WS-015", "in_progress", "", false); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"ui"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?profile=dashboard-v2&sort=workstream", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "showing 12 of 15 workstream(s)") {
		t.Fatalf("compact workstream count missing:\n%s", body)
	}
	if !strings.Contains(body, `href="/board?profile=dashboard-v2&amp;sort=workstream&amp;workstreams=all"`) {
		t.Fatalf("show-all href did not preserve board filters:\n%s", body)
	}
	activeIndex := strings.Index(body, "dashboard-v2 / kind-15")
	firstIndex := strings.Index(body, "dashboard-v2 / kind-01")
	if activeIndex < 0 || firstIndex < 0 || activeIndex > firstIndex {
		t.Fatalf("active workstream was not favored in compact view: active=%d first=%d\n%s", activeIndex, firstIndex, body)
	}

	req = httptest.NewRequest(http.MethodGet, "/board?profile=dashboard-v2&sort=workstream&workstreams=all", nil)
	rec = httptest.NewRecorder()
	server.board(rec, req)
	body = rec.Body.String()
	for _, want := range []string{
		"showing 15 of 15 workstream(s)",
		`href="/board?profile=dashboard-v2&amp;sort=workstream"`,
		`<input type="hidden" name="workstreams" value="all">`,
		"dashboard-v2 / kind-14",
		"dashboard-v2 / kind-15",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expanded workstream view missing %q:\n%s", want, body)
		}
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

func TestBoardSortSearchAndFilterChipsUseURLState(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-002", Title: "Zulu backend route", Kind: "feature", Role: "backend", Profile: "dashboard-v2", OwningDomain: "platform", RiskLevel: "medium", ReviewDomains: []string{"arch"}, SourcePaths: []string{"cmd/api/routes.go"}},
		{ID: "T-001", Title: "Alpha frontend view", Kind: "bug", Role: "ui", Profile: "dashboard-v2", OwningDomain: "dashboard", RiskLevel: "low", ReviewDomains: []string{"ui"}, SourcePaths: []string{"internal/dashboard/assets/js/board.js"}},
		{ID: "T-003", Title: "Registry worker", Kind: "task", Role: "ops", Profile: "ops", OwningDomain: "registry", RiskLevel: "high"},
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	cfg.Fairway.ProjectName = "fairway-test"
	server := New(s, cfg, []string{"backend", "ui", "ops"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?q=dashboard&profile=dashboard-v2&project=fairway-test&status=todo&role=ui&kind=bug&owning_domain=dashboard&risk_level=low&review_domain=ui&sort=title", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		`name="sort" value="title"`,
		`name="project" form="board-filter-form"`,
		`<b>Search</b> dashboard`,
		`<b>Project</b> fairway-test`,
		`<b>Status</b> todo`,
		`Clear filters`,
		`sort=title`,
		`aria-sort="ascending"><a href="/board?`,
		`data-sort-key="title">Title</a>`,
		"Alpha frontend view",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("sorted filtered board missing %q:\n%s", want, body)
		}
	}
	for _, unexpected := range []string{"Zulu backend route", "Registry worker"} {
		if strings.Contains(body, unexpected) {
			t.Fatalf("sorted filtered board included %q:\n%s", unexpected, body)
		}
	}
	if !strings.Contains(body, `href="/board?`) || !strings.Contains(body, `sort=title`) || strings.Contains(body, `href="/board?sort=title&amp;q=dashboard`) {
		t.Fatalf("filter chip hrefs did not preserve URL state clearly:\n%s", body)
	}
}

func TestBoardSortOrderAndToggleLinks(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Beta", Role: "backend", Kind: "task"},
		{ID: "T-002", Title: "Alpha", Role: "backend", Kind: "task"},
	}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?sort=title", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	alpha := strings.Index(body, "Alpha")
	beta := strings.Index(body, "Beta")
	if alpha < 0 || beta < 0 || alpha > beta {
		t.Fatalf("title ascending sort order wrong: alpha=%d beta=%d\n%s", alpha, beta, body)
	}
	if !strings.Contains(body, `sort=-title`) {
		t.Fatalf("title header did not toggle to descending:\n%s", body)
	}
	req = httptest.NewRequest(http.MethodGet, "/board?sort=-title", nil)
	rec = httptest.NewRecorder()
	server.board(rec, req)
	body = rec.Body.String()
	alpha = strings.Index(body, "Alpha")
	beta = strings.Index(body, "Beta")
	if alpha < 0 || beta < 0 || beta > alpha {
		t.Fatalf("title descending sort order wrong: alpha=%d beta=%d\n%s", alpha, beta, body)
	}
}

func TestBoardColumnChooserUsesURLState(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{
			ID:            "T-001",
			Title:         "Column task",
			Role:          "ui",
			Kind:          "dashboard",
			Profile:       "dashboard-v2",
			OwningDomain:  "fairway",
			RiskLevel:     "low",
			ReviewDomains: []string{"arch"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"ui"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?columns=title,id,profile,owning_domain,risk_level,review_domains&sort=profile", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"Columns</span><b>6 shown",
		`columns=title%2Cid%2Cprofile%2Cowning_domain%2Crisk_level%2Creview_domains`,
		`aria-sort="ascending"><a href="/board?`,
		`data-sort-key="profile">Profile</a>`,
		"Column task",
		"dashboard-v2",
		"fairway",
		"low",
		"arch",
		"Move Title right",
		"Hide</a>",
		"Show</a>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("column chooser board missing %q:\n%s", want, body)
		}
	}
	titleIndex := strings.Index(body, ">Title</a>")
	idIndex := strings.Index(body, ">ID</a>")
	profileIndex := strings.Index(body, ">Profile</a>")
	if titleIndex < 0 || idIndex < 0 || profileIndex < 0 || !(titleIndex < idIndex && idIndex < profileIndex) {
		t.Fatalf("column order wrong: title=%d id=%d profile=%d\n%s", titleIndex, idIndex, profileIndex, body)
	}
	if strings.Contains(body, ">Role</a>") || strings.Contains(body, ">Status</a>") {
		t.Fatalf("hidden default columns rendered in table:\n%s", body)
	}
}

func TestBoardServerWindowsAboveThreshold(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var tasks []store.TaskDefinition
	for i := 1; i <= 201; i++ {
		tasks = append(tasks, store.TaskDefinition{ID: fmt.Sprintf("T-%03d", i), Title: fmt.Sprintf("Task %03d", i), Role: "ui", Kind: "dashboard"})
	}
	if err := s.ImportTasks(ctx, tasks); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"ui"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?table_limit=200", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		`data-server-window="200"`,
		"server window 1 of 2",
		"showing 1-200 of 201 filtered tasks",
		`href="/board?page=2&amp;table_limit=200"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("server-windowed board missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Task 201") {
		t.Fatalf("first board window should not render rows outside the server slice:\n%s", body)
	}
	req = httptest.NewRequest(http.MethodGet, "/board?table_limit=200&page=2", nil)
	rec = httptest.NewRecorder()
	server.board(rec, req)
	body = rec.Body.String()
	for _, want := range []string{
		"Task 201",
		"server window 2 of 2",
		"showing 201-201 of 201 filtered tasks",
		`href="/board?table_limit=200"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("second board window missing %q:\n%s", want, body)
		}
	}
}

func TestBoardDoesNotVirtualizeAtThreshold(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var tasks []store.TaskDefinition
	for i := 1; i <= 200; i++ {
		tasks = append(tasks, store.TaskDefinition{ID: fmt.Sprintf("T-%03d", i), Title: fmt.Sprintf("Task %03d", i), Role: "ui", Kind: "dashboard"})
	}
	if err := s.ImportTasks(ctx, tasks); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"ui"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/board?table_limit=200", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	if strings.Contains(body, `data-server-window`) {
		t.Fatalf("board should not server-window at threshold:\n%s", body)
	}
	if !strings.Contains(body, "showing 1-200 of 200 filtered tasks") {
		t.Fatalf("board threshold footer wrong:\n%s", body)
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
	for _, want := range []string{"Todo task", "Blocked task", "<b>Status</b> todo", "<b>Status</b> blocked", "showing 1-2 of 2 filtered tasks", `name="status" value="todo"`, `name="status" value="blocked"`} {
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

func TestTaskDetailRendersProfileGateReadiness(t *testing.T) {
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
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{
		CommandText:  "make docs-link-check",
		Result:       "pass",
		ArtifactPath: "docs/source-map.json",
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
			RequiredEvidenceCount: 2,
			AcceptedResults:       []string{"pass"},
			ArtifactRequired:      true,
			Description:           "Each migrated page links back to source docs.",
		}},
	}}
	server := New(s, cfg, []string{"docs"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tasks/T-001", nil)
	rec := httptest.NewRecorder()
	server.task(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"Gate Readiness",
		"docusaurus-portal / source-docs-linked",
		"Each migrated page links back to source docs.",
		"source-doc-check",
		"satisfied",
		"<td>2</td><td>satisfied</td>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("satisfied task gate detail missing %q:\n%s", want, body)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/tasks/T-002", nil)
	rec = httptest.NewRecorder()
	server.task(rec, req)
	body = rec.Body.String()
	for _, want := range []string{
		"Gate Readiness",
		"docusaurus-portal / source-docs-linked",
		"missing",
		"needs 2 matching evidence row(s), found 0",
		"matching rows must include evidence artifacts",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing task gate detail missing %q:\n%s", want, body)
		}
	}
}

func TestBoardDiagnosticsRendersCleanCoverageAndLearningCounts(t *testing.T) {
	ctx := context.Background()
	root := initDashboardGitRepo(t)
	writeDashboardGitFile(t, root, "cmd/api/routes.go", "package api\n")
	runDashboardGit(t, root, "add", ".")
	runDashboardGit(t, root, "commit", "-m", "T-001 cover api")
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "API task", Role: "backend", Kind: "task", SourcePaths: []string{"cmd/api/**"}}}); err != nil {
		t.Fatal(err)
	}
	server := NewWithRoot(s, config.Defaults(root), []string{"backend"}, nil, root)
	req := httptest.NewRequest(http.MethodGet, "/board?tab=diagnostics", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"Coverage Diagnostics",
		"Uncovered commits",
		"Uncovered changed files",
		"Orphan evidence",
		"Failed CI follow-ups",
		"no work coverage findings",
		"no CI/deploy learning findings",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("clean diagnostics missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "commit_without_task_coverage") || strings.Contains(body, "changed_file_uncovered") {
		t.Fatalf("clean diagnostics contained coverage findings:\n%s", body)
	}
}

func TestBoardDiagnosticsRendersUncoveredCommitAndChangedFile(t *testing.T) {
	ctx := context.Background()
	root := initDashboardGitRepo(t)
	writeDashboardGitFile(t, root, "docs/plan.md", "# plan\n")
	runDashboardGit(t, root, "add", ".")
	runDashboardGit(t, root, "commit", "-m", "untracked docs")
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "API task", Role: "backend", Kind: "task", SourcePaths: []string{"cmd/api/**"}}}); err != nil {
		t.Fatal(err)
	}
	server := NewWithRoot(s, config.Defaults(root), []string{"backend"}, nil, root)
	req := httptest.NewRequest(http.MethodGet, "/board?tab=diagnostics", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"commit_without_task_coverage",
		"changed_file_uncovered",
		"docs/plan.md",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("uncovered diagnostics missing %q:\n%s", want, body)
		}
	}
}

func TestBoardDiagnosticsRendersFailedCILearningWithoutFollowUp(t *testing.T) {
	ctx := context.Background()
	root := initDashboardGitRepo(t)
	writeDashboardGitFile(t, root, "cmd/api/routes.go", "package api\n")
	runDashboardGit(t, root, "add", ".")
	runDashboardGit(t, root, "commit", "-m", "T-001 cover api")
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "API task", Role: "backend", Kind: "task", SourcePaths: []string{"cmd/api/**"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "fail", ArtifactType: "ci", ArtifactPath: "https://ci.example/jobs/1"}); err != nil {
		t.Fatal(err)
	}
	server := NewWithRoot(s, config.Defaults(root), []string{"backend"}, nil, root)
	req := httptest.NewRequest(http.MethodGet, "/board?tab=diagnostics", nil)
	rec := httptest.NewRecorder()
	server.board(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"CI Learning Findings",
		"missed_local_gate",
		"fairway add CI-FIX-T-001",
		"go test ./...",
		"Failed CI follow-ups",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("ci learning diagnostics missing %q:\n%s", want, body)
		}
	}
}

func TestTaskDetailRendersTaskScopedCoverage(t *testing.T) {
	ctx := context.Background()
	root := initDashboardGitRepo(t)
	writeDashboardGitFile(t, root, "cmd/api/routes.go", "package api\n")
	runDashboardGit(t, root, "add", ".")
	runDashboardGit(t, root, "commit", "-m", "T-001 cover api")
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "API task", Role: "backend", Kind: "task", SourcePaths: []string{"cmd/api/**"}}}); err != nil {
		t.Fatal(err)
	}
	server := NewWithRoot(s, config.Defaults(root), []string{"backend"}, nil, root)
	req := httptest.NewRequest(http.MethodGet, "/tasks/T-001", nil)
	rec := httptest.NewRecorder()
	server.task(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"Coverage Diagnostics",
		"Recent changed files are covered by this task's source_paths or target_paths.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("task coverage detail missing %q:\n%s", want, body)
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

func TestTaskDetailRendersPartialApprovalWhenReviewDomainsMissing(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{
		ID:            "T-001",
		Title:         "Needs domain reviews",
		Kind:          "dashboard",
		Role:          "ui",
		ReviewDomains: []string{"architecture", "governance"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", store.Review{Reviewer: "architecture", Verdict: "approve", Reason: "arch ok"}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"ui"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tasks/T-001", nil)
	rec := httptest.NewRecorder()
	server.task(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"review partial_approval", "Missing required domains", "governance"} {
		if !strings.Contains(body, want) {
			t.Fatalf("task detail partial approval missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "review approved") {
		t.Fatalf("dashboard summarized partial review as approved:\n%s", body)
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

func TestDashboardBulkActionsRequireCSRFAndAuditPerTask(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "First", Role: "backend"},
		{ID: "T-002", Title: "Second", Role: "ui"},
	}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend", "ui"}, nil)

	badReq := httptest.NewRequest(http.MethodPost, "/actions/bulk/evidence", strings.NewReader("task_id=T-001&csrf=bad&command_text=go+test&result=pass"))
	badReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badRec := httptest.NewRecorder()
	server.bulkEvidence(badRec, badReq)
	if badRec.Code != http.StatusForbidden {
		t.Fatalf("bad csrf status=%d, want 403", badRec.Code)
	}

	form := url.Values{}
	form.Add("csrf", server.csrfToken)
	form.Add("task_id", "T-001")
	form.Add("task_id", "T-002")
	form.Set("command_text", "go test ./...")
	form.Set("result", "pass")
	form.Set("artifact_type", "verification")
	req := httptest.NewRequest(http.MethodPost, "/actions/bulk/evidence", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.bulkEvidence(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("bulk evidence status=%d, want 303 body=%s", rec.Code, rec.Body.String())
	}
	for _, taskID := range []string{"T-001", "T-002"} {
		_, _, evidence, _, _, err := s.TaskDetail(ctx, taskID)
		if err != nil {
			t.Fatal(err)
		}
		if len(evidence) != 1 || evidence[0].Result != "pass" || evidence[0].ArtifactType != "verification" {
			t.Fatalf("%s evidence=%+v, want one pass verification", taskID, evidence)
		}
	}
	auditRows, err := s.AuditCount(ctx, "dashboard.bulk.evidence")
	if err != nil {
		t.Fatal(err)
	}
	if auditRows != 2 {
		t.Fatalf("audit rows=%d, want 2", auditRows)
	}
}

func TestDashboardBulkSetStatusRejectsTerminalStatus(t *testing.T) {
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
	form := url.Values{}
	form.Set("csrf", server.csrfToken)
	form.Set("task_id", "T-001")
	form.Set("status", "done")
	req := httptest.NewRequest(http.MethodPost, "/actions/bulk/set-status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.bulkSetStatus(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("terminal bulk status code=%d, want 400", rec.Code)
	}
	task, _, _, _, _, err := s.TaskDetail(ctx, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "todo" {
		t.Fatalf("status=%q, want unchanged todo", task.Status)
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
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"fairway multi-project", "fairway", "T-001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("multi dashboard body missing %q:\n%s", want, body)
		}
	}
}

func TestMultiDashboardBoardFiltersByProject(t *testing.T) {
	ctx := context.Background()
	left, err := store.Open(ctx, filepath.Join(t.TempDir(), "left.db"), "left")
	if err != nil {
		t.Fatal(err)
	}
	defer left.Close()
	right, err := store.Open(ctx, filepath.Join(t.TempDir(), "right.db"), "right")
	if err != nil {
		t.Fatal(err)
	}
	defer right.Close()
	if err := left.ImportTasks(ctx, []store.TaskDefinition{{ID: "L-001", Title: "Left project task", Role: "backend", Kind: "dashboard"}}); err != nil {
		t.Fatal(err)
	}
	if err := right.ImportTasks(ctx, []store.TaskDefinition{{ID: "R-001", Title: "Right project task", Role: "ui", Kind: "dashboard"}}); err != nil {
		t.Fatal(err)
	}
	handler := NewMulti([]ProjectStore{
		{Name: "left-project", Path: "/tmp/left", Store: left},
		{Name: "right-project", Path: "/tmp/right", Store: right},
	})
	req := httptest.NewRequest(http.MethodGet, "/board?project=right-project", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"Workstream Dashboard",
		`<b>Project</b> right-project`,
		"Project</a>",
		"Right project task",
		"right-project",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("multi board body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Left project task") {
		t.Fatalf("multi board included task outside project filter:\n%s", body)
	}
}

func TestMultiDashboardWallGroupsLanesByProject(t *testing.T) {
	ctx := context.Background()
	left, err := store.Open(ctx, filepath.Join(t.TempDir(), "left.db"), "left")
	if err != nil {
		t.Fatal(err)
	}
	defer left.Close()
	right, err := store.Open(ctx, filepath.Join(t.TempDir(), "right.db"), "right")
	if err != nil {
		t.Fatal(err)
	}
	defer right.Close()
	if err := left.ImportTasks(ctx, []store.TaskDefinition{{ID: "L-001", Title: "Left wall task", Role: "backend", Kind: "dashboard"}}); err != nil {
		t.Fatal(err)
	}
	if err := right.ImportTasks(ctx, []store.TaskDefinition{{ID: "R-001", Title: "Right wall task", Role: "ui", Kind: "dashboard"}}); err != nil {
		t.Fatal(err)
	}
	if err := right.SetStatus(ctx, "R-001", "in_progress", "", false); err != nil {
		t.Fatal(err)
	}
	handler := NewMulti([]ProjectStore{
		{Name: "left-project", Path: "/tmp/left", Store: left},
		{Name: "right-project", Path: "/tmp/right", Store: right},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"class=\"project-wall-group\"",
		"left-project",
		"right-project",
		"Left wall task",
		"Right wall task",
		`href="/board?project=right-project&amp;role=ui"`,
		"Project Readiness",
		"[right-project]",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("multi wall body missing %q:\n%s", want, body)
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
		Rollups              map[string]Rollup
		ActiveReport         reconcile.ActiveReport
		Audit                AuditDiagnostics
		ProjectGroups        []ProjectWallGroup
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
		Rollups:              map[string]Rollup{"T-002": {Done: 1, Total: 3}},
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
		`aria-label="Switch to dark theme"`,
		`data-theme-toggle`,
		`/assets/js/common.js`,
		`/assets/js/wall.js`,
		`/assets/css/wall.css`,
		`data-lane-toggle="backend"`,
		`data-lane-panel="backend"`,
		`data-wall-role="backend"`,
		`data-task-id="T-002"`,
		`data-handoff-count`,
		`data-wall-ticker`,
		`aria-expanded="false"`,
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
		"Open full details for backend",
		"backend lane details",
		"Queue",
		"Working",
		"Pending review",
		"Latest events",
		"gates 1/3",
		"handoff backend to ui",
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
		FilterChips:      boardFilterChips(TaskFilters{Search: "dashboard", Role: "backend", Statuses: []string{"todo"}, Profile: "dashboard-v2", Kind: "task", OwningDomain: "fairway", RiskLevel: "medium", Tab: "tasks", ActivityLimit: 25}),
		ClearFiltersHref: boardClearFiltersHref(TaskFilters{Search: "dashboard", Role: "backend", Statuses: []string{"todo"}, Profile: "dashboard-v2", Kind: "task", OwningDomain: "fairway", RiskLevel: "medium", Tab: "tasks", ActivityLimit: 25}),
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
		`data-board-search=".board-table"`,
		"Diagnostics",
		"<b>Role</b> backend",
		"Clear filters",
		`data-sort-key="title"`,
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
		`data-board-row data-task-href="/tasks/T-001"`,
		`data-keyboard-panel="columns"`,
		`data-keyboard-panel="views"`,
		`data-keyboard-help-open`,
		`id="keyboard-help-dialog"`,
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
		ActiveReport: reconcile.ActiveReport{
			Findings: []reconcile.ActiveFinding{
				{Kind: "monitor_session_without_backing_proof", TaskID: "T-001", SessionID: "s-1", Action: "mark_session_stale", Reason: "monitor session has no backing automation, process proof, external polling proof, or fresh bounded manual checkpoint"},
				{Kind: "monitor_completion_resume_needed", TaskID: "T-002", Action: "record_resume_checkpoint_or_continue_ready_work", Reason: "all monitors are complete and ready work remains"},
			},
			Summary: reconcile.ActiveSummary{MonitorSessionsNoProof: 1, MonitorResumeNeeded: 1},
		},
		CloseoutReports: []reconcile.CloseoutReport{{
			TaskID:   "T-003",
			Branch:   "agent/backend",
			Summary:  reconcile.CloseoutSummary{Blockers: 1, Warnings: 1},
			Findings: []reconcile.CloseoutFinding{{Kind: "dirty_worktree"}, {Kind: "remote_branch_present"}},
		}},
		FilterOptions: FilterOptions{ActivityKinds: []string{"checkpoint", "evidence"}},
		Activity:      []store.Activity{{Kind: "checkpoint", TaskID: "T-001", Summary: "working", CreatedAt: "2026-06-04T00:00:00Z"}},
		ActivityTotal: 1,
	}
	var out strings.Builder
	if err := boardTemplate.ExecuteTemplate(&out, "layout", data); err != nil {
		t.Fatalf("board diagnostics render error = %v", err)
	}
	body := out.String()
	for _, want := range []string{
		`href="/board?tab=diagnostics"`,
		"Sessions",
		"Active Reconciliation",
		"monitor proof: 1",
		"monitor resume: 1",
		"closeout debt: 1",
		"monitor_session_without_backing_proof",
		"monitor_completion_resume_needed",
		"mark_session_stale",
		"Lane Closeout",
		"dirty_worktree",
		"remote_branch_present",
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
	if strings.Contains(body, "task-table-section") {
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
		{path: "/assets/js/board.js", want: []string{
			`event.key === "g"`,
			`event.key === "w"`,
			`window.location.assign("/")`,
			`setCursor(cursorIndex + 1)`,
			`setCursor(cursorIndex - 1)`,
			`input[data-board-search]`,
			`openCursorDialog("bulk-status-dialog")`,
			`openCursorDialog("bulk-handoff-dialog")`,
			`data-keyboard-panel="columns"`,
			`data-keyboard-panel="views"`,
			`keyboard-help-dialog`,
			`isTextInput(document.activeElement)`,
		}},
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
		{path: "/assets/css/wall.css", want: []string{".wall-layout", ".wall-lane", ".lane-states", ".wall-rail", ".handoff-arc", ".heartbeat-fresh", ".heartbeat-warm", ".heartbeat-stale"}},
		{path: "/assets/css/board.css", want: []string{".board-layout", ".control-room-head", ".gate-grid", ".board-table", ".board-rail", `th[aria-sort="ascending"] a[data-sort-key]`}},
		{path: "/assets/css/components.css", want: []string{"a:focus-visible", "button:focus-visible", "summary:focus-visible", "textarea:focus-visible"}},
		{path: "/assets/js/wall.js", want: []string{"data-lane-toggle", "aria-expanded", "data-lane-panel", `new EventSource("/events")`, "drawHandoffArc", "session_heartbeat", "data-handoff-count", "data-wall-ticker"}},
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

func initDashboardGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runDashboardGit(t, root, "init")
	runDashboardGit(t, root, "config", "user.email", "fairway@example.test")
	runDashboardGit(t, root, "config", "user.name", "Fairway Test")
	writeDashboardGitFile(t, root, "cmd/api/base.go", "package api\n")
	runDashboardGit(t, root, "add", ".")
	runDashboardGit(t, root, "commit", "-m", "T-001 base")
	return root
}

func writeDashboardGitFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runDashboardGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}
