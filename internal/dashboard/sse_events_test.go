package dashboard

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSSEEventsFromSourceMapsTaxonomy(t *testing.T) {
	src := store.EventSource{Source: "history", TaskID: "T-001", Role: "backend", Owner: "backend", ToStatus: "in_progress", Reason: "claim"}
	src.Cursor.At = "2026-06-04T00:00:00Z"
	src.Cursor.SourceOrder = 10
	src.Cursor.ID = 1
	events := sseEventsFromSource(src)
	if len(events) != 1 || events[0].Name != "claim" {
		t.Fatalf("claim mapping = %#v", events)
	}
	if events[0].Payload["task_id"] != "T-001" || events[0].Payload["owner"] != "backend" {
		t.Fatalf("claim payload = %#v", events[0].Payload)
	}

	src = store.EventSource{Source: "history", TaskID: "T-001", Role: "backend", Owner: "backend", FromStatus: "in_progress", ToStatus: "done", Actor: "agent"}
	src.Cursor.At = "2026-06-04T00:00:01Z"
	src.Cursor.SourceOrder = 10
	src.Cursor.ID = 2
	events = sseEventsFromSource(src)
	if len(events) != 1 || events[0].Name != "done" {
		t.Fatalf("done mapping = %#v", events)
	}

	src = store.EventSource{Source: "evidence", TaskID: "T-001", Role: "backend", EvidenceType: "test", EvidenceCount: 3}
	src.Cursor.At = "2026-06-04T00:00:02Z"
	src.Cursor.SourceOrder = 20
	src.Cursor.ID = 3
	events = sseEventsFromSource(src)
	if len(events) != 1 || events[0].Name != "evidence" || events[0].Payload["count"] != 3 {
		t.Fatalf("evidence mapping = %#v", events)
	}

	src = store.EventSource{Source: "session_detach", TaskID: "T-001", Role: "backend", Provider: "codex", SessionID: "codex-1", EndReason: "completed"}
	src.Cursor.At = "2026-06-04T00:00:03Z"
	src.Cursor.SourceOrder = 70
	src.Cursor.ID = 4
	events = sseEventsFromSource(src)
	if len(events) != 1 || events[0].Name != "session_detach" || events[0].Payload["reason"] != "completed" {
		t.Fatalf("session detach mapping = %#v", events)
	}
	if events[0].Payload["task_id"] != src.TaskID {
		t.Fatalf("session detach task payload = %#v", events[0].Payload)
	}
}

func TestWriteSSEEvent(t *testing.T) {
	var out strings.Builder
	err := writeSSEEvent(&out, sseEvent{CursorID: "fairway-source:test", Name: "claim", Payload: map[string]any{"task_id": "T-001"}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "id: fairway-source:test\n") || !strings.Contains(got, "event: claim\n") || !strings.Contains(got, `"task_id":"T-001"`) {
		t.Fatalf("unexpected SSE output: %q", got)
	}
}

func TestSourceCursorIDRoundTrip(t *testing.T) {
	want := store.EventCursor{At: "2026-07-11T20:00:00.123Z", SourceOrder: 45, ID: 99}
	encoded := sourceCursorID(want)
	got, ok := parseSourceCursorID(encoded)
	if !ok || got != want {
		t.Fatalf("cursor round trip = %#v, %t; want %#v", got, ok, want)
	}
	if _, ok := parseSourceCursorID("review_wait:T-001/ops"); ok {
		t.Fatal("non-source event id unexpectedly parsed")
	}
}

func TestStoreEventSourcesCoverDashboardTaxonomyInputs(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Task", Kind: "task", Role: "backend"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Claim(ctx, "T-001", "backend", "agent/backend"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordHandoff(ctx, "T-001", store.Handoff{ToRole: "arch", Payload: "please review"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", store.Review{Reviewer: "arch", Verdict: "approve", Reason: "ok"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "T-001", Domain: "arch", Provider: "codex", State: "thread_steered"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "codex-1", Role: "backend", Provider: "codex", SessionBackend: "codex-thread", TaskID: "T-001"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EndSession(ctx, "codex-1", "ended", "completed", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "done", "complete", false); err != nil {
		t.Fatal(err)
	}

	sources, err := s.EventSources(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, source := range sources {
		for _, event := range sseEventsFromSource(source) {
			seen[event.Name] = true
		}
	}
	for _, want := range []string{"claim", "done", "evidence", "handoff", "review_verdict", "notification", "session_attach", "session_heartbeat", "session_detach"} {
		if !seen[want] {
			t.Fatalf("missing %s in events from sources %#v", want, sources)
		}
	}
}

func TestStoreEventSourcesAfterUsesCursorAndIncludesNotifications(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Kind: "task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	baseline, err := s.LatestEventCursor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sources, err := s.EventSourcesAfter(ctx, baseline, 100); err != nil || len(sources) != 0 {
		t.Fatalf("idle sources = %#v, %v", sources, err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "T-001", Domain: "ops", Provider: "codex", State: "thread_steered"}); err != nil {
		t.Fatal(err)
	}
	sources, err := s.EventSourcesAfter(ctx, baseline, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0].Source != "evidence" || sources[1].Source != "notification" {
		t.Fatalf("incremental sources = %#v", sources)
	}
	events := sseEventsFromSource(sources[1])
	if len(events) != 1 || events[0].Name != "notification" || events[0].Payload["domain"] != "ops" || events[0].Payload["state"] != "thread_steered" {
		t.Fatalf("notification events = %#v", events)
	}
	latest, err := s.LatestEventCursor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if compareEventCursor(baseline, latest) >= 0 || compareEventCursor(latest, sources[len(sources)-1].Cursor) != 0 {
		t.Fatalf("cursor baseline=%#v latest=%#v sources=%#v", baseline, latest, sources)
	}
}

func TestEventsIdlePollDoesNotHydrateFullSources(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Kind: "task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	server.eventPollInterval = 5 * time.Millisecond
	server.reviewWaitSweepInterval = time.Hour

	requestCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/events", nil).WithContext(requestCtx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.events(rec, req)
		close(done)
	}()
	defer cleanupSSEStream(t, cancel, done)
	waitForSSECondition(t, "three idle cursor checks", func() bool {
		return server.sseStats.cursorChecks.Load() >= 3
	})
	cancel()
	waitForSSECompletion(t, done)

	if checks := server.sseStats.cursorChecks.Load(); checks < 3 {
		t.Fatalf("cursor checks = %d; want at least 3", checks)
	}
	if hydrations := server.sseStats.sourceHydrations.Load(); hydrations != 0 {
		t.Fatalf("idle source hydrations = %d; want 0", hydrations)
	}
	if sweeps := server.sseStats.reviewWaitSweeps.Load(); sweeps != 0 {
		t.Fatalf("idle review wait sweeps = %d; want 0", sweeps)
	}
	if !strings.Contains(rec.Body.String(), ": connected\n\n") {
		t.Fatalf("stream body = %q", rec.Body.String())
	}
}

func TestEventsDeliverIncrementalChangeAndResumeFromCursor(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Kind: "task", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	server := New(s, config.Defaults(t.TempDir()), []string{"backend"}, nil)
	server.eventPollInterval = 5 * time.Millisecond
	server.reviewWaitSweepInterval = time.Hour

	firstBody := runTestEventStream(t, server, "", func() {
		if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "first", Result: "pass", ArtifactType: "test"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(firstBody, "event: evidence\n") || !strings.Contains(firstBody, `"count":1`) {
		t.Fatalf("first stream = %q", firstBody)
	}
	var resumeID string
	for _, line := range strings.Split(firstBody, "\n") {
		if strings.HasPrefix(line, "id: ") {
			resumeID = strings.TrimPrefix(line, "id: ")
		}
	}
	if resumeID == "" {
		t.Fatalf("first stream missing cursor id: %q", firstBody)
	}
	secondBody := runTestEventStream(t, server, resumeID, func() {
		if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "second", Result: "pass", ArtifactType: "test"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(secondBody, `"count":2`) || strings.Contains(secondBody, `"count":1`) {
		t.Fatalf("resumed stream = %q", secondBody)
	}
}

func TestEventsSweepEmitsStaleReviewWait(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Review", Kind: "task", Role: "backend", ReviewDomains: []string{"ops"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "T-001", "in_progress", "review", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "T-001", Domain: "ops", Provider: "codex", Target: "thread-ops", State: "notification_delivered"}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	cfg.Roles = []config.Role{{Name: "ops"}}
	cfg.Coordinator.NotificationAckTimeout = "1ns"
	server := New(s, cfg, []string{"backend"}, nil)
	server.eventPollInterval = time.Hour
	server.reviewWaitSweepInterval = 5 * time.Millisecond

	requestCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/events", nil).WithContext(requestCtx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.events(rec, req)
		close(done)
	}()
	defer cleanupSSEStream(t, cancel, done)
	waitForSSECondition(t, "two review-wait sweeps", func() bool {
		return server.sseStats.reviewWaitSweeps.Load() >= 2
	})
	cancel()
	waitForSSECompletion(t, done)
	if !strings.Contains(rec.Body.String(), "event: review_wait.stale\n") {
		t.Fatalf("stale stream = %q", rec.Body.String())
	}
}

func runTestEventStream(t *testing.T, server *Server, lastEventID string, mutate func()) string {
	t.Helper()
	requestCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/events", nil).WithContext(requestCtx)
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.events(rec, req)
		close(done)
	}()
	defer cleanupSSEStream(t, cancel, done)
	startChecks := server.sseStats.cursorChecks.Load()
	waitForSSECondition(t, "event stream startup", func() bool {
		return server.sseStats.cursorChecks.Load() > startChecks
	})
	startHydrations := server.sseStats.sourceHydrations.Load()
	mutate()
	waitForSSECondition(t, "incremental event hydration", func() bool {
		return server.sseStats.sourceHydrations.Load() > startHydrations
	})
	hydrationChecks := server.sseStats.cursorChecks.Load()
	waitForSSECondition(t, "post-hydration poll", func() bool {
		return server.sseStats.cursorChecks.Load() > hydrationChecks
	})
	cancel()
	waitForSSECompletion(t, done)
	return rec.Body.String()
}

func waitForSSECondition(t *testing.T, description string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !condition() {
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForSSECompletion(t *testing.T, done <-chan struct{}) {
	t.Helper()
	if !awaitSSECompletion(done) {
		t.Fatal("timed out waiting for event stream shutdown")
	}
}

func cleanupSSEStream(t *testing.T, cancel context.CancelFunc, done <-chan struct{}) {
	t.Helper()
	cancel()
	if !awaitSSECompletion(done) {
		t.Error("timed out cleaning up event stream")
	}
}

func awaitSSECompletion(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}

func TestReviewWaitEventsUseSharedProjection(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Stale wait", Kind: "dashboard", Role: "ui", ReviewDomains: []string{"architecture"}},
		{ID: "T-002", Title: "Failed wait", Kind: "dashboard", Role: "ui", ReviewDomains: []string{"product"}},
		{ID: "T-003", Title: "Resolved wait", Kind: "dashboard", Role: "ui", ReviewDomains: []string{"ops"}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, taskID := range []string{"T-001", "T-002", "T-003"} {
		if err := s.SetStatus(ctx, taskID, "in_progress", "entered review wait", false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "T-001", Domain: "architecture", Provider: "codex", Target: "thread-arch", State: "notification_delivered"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordNotification(ctx, store.Notification{TaskID: "T-002", Domain: "product", State: "notification_failed", Reason: "no reviewer mapping"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-003", store.Review{Reviewer: "ops-reviewer", Domain: "ops", Verdict: "approve", Reason: "ok"}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	cfg.Coordinator.NotificationAckTimeout = "1ns"
	cfg.Roles = []config.Role{{Name: "architecture"}, {Name: "ops"}}
	server := New(s, cfg, []string{"ui"}, nil)
	events, err := server.reviewWaitEvents(ctx, time.Now().UTC().Add(time.Second).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]sseEvent{}
	for _, event := range events {
		seen[event.Name] = event
	}
	for _, want := range []string{"review_wait.stale", "review_wait.notification_failed", "review_wait.resolved"} {
		if _, ok := seen[want]; !ok {
			t.Fatalf("missing %s in review wait events %#v", want, events)
		}
	}
	if seen["review_wait.notification_failed"].Payload["action"] != "mapping_required" {
		t.Fatalf("notification_failed payload = %#v", seen["review_wait.notification_failed"].Payload)
	}
	if seen["review_wait.resolved"].Payload["task_id"] != "T-003" {
		t.Fatalf("resolved payload = %#v", seen["review_wait.resolved"].Payload)
	}
}

func TestGateChangeEvents(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Task", Kind: "dashboard", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(t.TempDir())
	cfg.WorkstreamProfiles = []config.WorkstreamProfile{{
		Name:      "dashboard-v2",
		TaskKinds: []string{"dashboard"},
		Gates: []config.WorkstreamProfileGate{{
			Name:                  "dashboard-regression",
			Group:                 "tests",
			Mode:                  "advisory",
			EvidenceType:          "test",
			RequiredEvidenceCount: 1,
			AcceptedResults:       []string{"pass"},
		}},
	}}
	server := New(s, cfg, []string{"backend"}, nil)
	events, err := server.gateChangeEvents(ctx, "source-1", "2026-06-04T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Name != "gate_change" {
		t.Fatalf("gate events = %#v", events)
	}
	if events[0].Payload["profile"] != "dashboard-v2" || events[0].Payload["gate"] != "dashboard-regression" || events[0].Payload["satisfied"] != 1 {
		t.Fatalf("gate payload = %#v", events[0].Payload)
	}
}
