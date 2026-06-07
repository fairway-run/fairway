package dashboard

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

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
	err := writeSSEEvent(&out, sseEvent{Name: "claim", Payload: map[string]any{"task_id": "T-001"}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "event: claim\n") || !strings.Contains(got, `"task_id":"T-001"`) {
		t.Fatalf("unexpected SSE output: %q", got)
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
	for _, want := range []string{"claim", "done", "evidence", "handoff", "review_verdict", "session_attach", "session_heartbeat", "session_detach"} {
		if !seen[want] {
			t.Fatalf("missing %s in events from sources %#v", want, sources)
		}
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
