package reviewstate

import (
	"testing"

	"github.com/subashram/fairway/internal/store"
)

func TestStatusesForTaskUsesNotificationBoundToLatestHandoff(t *testing.T) {
	oldHandoffID := int64(11)
	latestHandoffID := int64(12)
	task := store.Task{Definition: store.TaskDefinition{ID: "T-001", ReviewDomains: []string{"architecture"}}}
	handoffs := []store.Handoff{
		{ID: oldHandoffID, ToRole: "architecture", CreatedAt: "2026-08-12T12:00:00Z"},
		{ID: latestHandoffID, ToRole: "architecture", CreatedAt: "2026-08-12T12:00:00Z"},
	}
	notifications := []store.Notification{
		{ID: 20, HandoffID: &oldHandoffID, Domain: "architecture", State: "thread_steered", CreatedAt: "2026-08-12T12:00:01Z"},
		{ID: 21, HandoffID: &latestHandoffID, Domain: "architecture", State: "notification_failed", Reason: "delivery failed", CreatedAt: "2026-08-12T11:59:59Z"},
	}

	statuses := StatusesForTask(task, handoffs, nil, notifications)
	if len(statuses) != 1 {
		t.Fatalf("got %d statuses, want 1", len(statuses))
	}
	if got := statuses[0]; got.HandoffID != latestHandoffID || got.Status != "notification_failed" || got.Reason != "delivery failed" {
		t.Fatalf("latest handoff status = %+v", got)
	}
}

func TestStatusesForTaskFallsBackToUnboundNotificationAfterHandoff(t *testing.T) {
	task := store.Task{Definition: store.TaskDefinition{ID: "T-001", ReviewDomains: []string{"architecture"}}}
	handoffs := []store.Handoff{{ID: 12, ToRole: "architecture", CreatedAt: "2026-08-12T12:00:00Z"}}
	notifications := []store.Notification{
		{ID: 20, Domain: "architecture", State: "thread_steered", CreatedAt: "2026-08-12T11:59:59Z"},
		{ID: 21, Domain: "architecture", State: "notification_failed", Reason: "delivery failed", CreatedAt: "2026-08-12T12:00:01Z"},
	}

	statuses := StatusesForTask(task, handoffs, nil, notifications)
	if got := statuses[0]; got.Status != "notification_failed" || got.LastNotificationAt != "2026-08-12T12:00:01Z" {
		t.Fatalf("legacy notification status = %+v", got)
	}
}
