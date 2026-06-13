package completionhandback

import (
	"testing"
	"time"

	"github.com/subashram/fairway/internal/store"
)

func TestRenderAndProjectCompletionHandback(t *testing.T) {
	payload, err := RenderPayload("schedule next window", []string{"packet.md", "packet.md", "evidence.md"}, "no deploy authority")
	if err != nil {
		t.Fatal(err)
	}
	rows := Rows("T-001", []store.Handoff{{ID: 7, FromRole: "backend", ToRole: "ops", Payload: payload, CreatedAt: "2026-06-13T00:00:00Z"}}, []store.Notification{{
		ID:        9,
		TaskID:    "T-001",
		HandoffID: int64Ptr(7),
		Domain:    "ops",
		Provider:  "codex",
		Target:    "thread-ops",
		State:     "thread_steered",
		CreatedAt: "2026-06-13T00:01:00Z",
	}})
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
	row := rows[0]
	if row.NextAction != "schedule next window" || row.ApprovalBoundary != "no deploy authority" {
		t.Fatalf("row payload=%+v", row)
	}
	if len(row.EvidencePaths) != 2 || row.EvidencePaths[0] != "packet.md" || row.EvidencePaths[1] != "evidence.md" {
		t.Fatalf("evidence paths=%+v", row.EvidencePaths)
	}
	if row.DeliveryStatus != "delivered" || !row.ActualThreadDelivery {
		t.Fatalf("delivery=%+v", row)
	}
}

func TestCompletionHandbackStateAndStaleProjection(t *testing.T) {
	payload, err := RenderPayloadWithState("assign harness fix", "blocked-with-follow-up", []string{"rollback.md"}, "ops closeout only")
	if err != nil {
		t.Fatal(err)
	}
	rows := RowsWithOptions("DRILL-001", []store.Handoff{{
		ID:        23,
		FromRole:  "ops",
		ToRole:    "arch",
		Payload:   payload,
		CreatedAt: "2026-06-13T04:55:00Z",
	}}, []store.Notification{{
		ID:        24,
		TaskID:    "DRILL-001",
		HandoffID: int64Ptr(23),
		Domain:    "arch",
		State:     "handoff_recorded",
		CreatedAt: "2026-06-13T04:55:00Z",
	}}, RowOptions{
		Now:             time.Date(2026, 6, 13, 12, 55, 0, 0, time.UTC),
		AckTimeout:      time.Hour,
		TaskStatus:      "blocked",
		LiveWindowPhase: "closeout",
	})
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
	row := rows[0]
	if row.CompletionState != "blocked-with-follow-up" || row.TaskStatus != "blocked" || row.LiveWindowPhase != "closeout" {
		t.Fatalf("row context=%+v", row)
	}
	if row.DeliveryStatus != "stale" || !row.Stale || row.StaleAge != "8h0m0s" || IsResolved(row) {
		t.Fatalf("stale row=%+v", row)
	}
	if row.SuggestedAction != "escalate_completion_handback" || row.SuggestedCommand == "" {
		t.Fatalf("suggestion missing: %+v", row)
	}
}

func TestCompletionHandbackPendingAndFailedDelivery(t *testing.T) {
	payload, err := RenderPayload("fix blocker", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	pending := Rows("T-001", []store.Handoff{{ID: 1, ToRole: "arch", Payload: payload}}, nil)
	if len(pending) != 1 || pending[0].DeliveryStatus != "pending" || IsResolved(pending[0]) {
		t.Fatalf("pending row=%+v", pending)
	}
	failed := Rows("T-001", []store.Handoff{{ID: 1, ToRole: "arch", Payload: payload}}, []store.Notification{{
		ID:        2,
		TaskID:    "T-001",
		HandoffID: int64Ptr(1),
		Domain:    "arch",
		State:     "notification_failed",
		Reason:    "thread tool unavailable",
		CreatedAt: "2026-06-13T00:01:00Z",
	}})
	if len(failed) != 1 || failed[0].DeliveryStatus != "failed" || !IsResolved(failed[0]) {
		t.Fatalf("failed row=%+v", failed)
	}
	reviewRecorded := Rows("T-001", []store.Handoff{{ID: 1, ToRole: "arch", Payload: payload}}, []store.Notification{{
		ID:        3,
		TaskID:    "T-001",
		HandoffID: int64Ptr(1),
		Domain:    "arch",
		State:     "review_recorded",
		CreatedAt: "2026-06-13T00:02:00Z",
	}})
	if len(reviewRecorded) != 1 || reviewRecorded[0].DeliveryStatus != "pending" || IsResolved(reviewRecorded[0]) {
		t.Fatalf("review-recorded state should not resolve completion handback: %+v", reviewRecorded)
	}
	acknowledged := Rows("T-001", []store.Handoff{{ID: 1, ToRole: "arch", Payload: payload}}, []store.Notification{{
		ID:        4,
		TaskID:    "T-001",
		HandoffID: int64Ptr(1),
		Domain:    "arch",
		State:     "acknowledged",
		CreatedAt: "2026-06-13T00:03:00Z",
	}})
	if len(acknowledged) != 1 || acknowledged[0].DeliveryStatus != "pending" || IsResolved(acknowledged[0]) {
		t.Fatalf("acknowledged state should not resolve completion handback: %+v", acknowledged)
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}
