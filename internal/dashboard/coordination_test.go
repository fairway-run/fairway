package dashboard

import (
	"testing"
	"time"

	"github.com/subashram/fairway/internal/completionhandback"
	coord "github.com/subashram/fairway/internal/coordinator"
	"github.com/subashram/fairway/internal/reviewstate"
	"github.com/subashram/fairway/internal/store"
)

func TestDashboardCoordinationIntelligenceProjection(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	plan := coord.Plan{
		Actions: []coord.PlanAction{{
			Classification: "waiting",
			Action:         "resolve_wait",
			TaskID:         "T-001",
			Reason:         "next deterministic action",
		}},
		ReviewWaits: []reviewstate.ReviewWait{{
			TaskID:         "T-001",
			Domain:         "security",
			State:          "stale",
			Action:         "nudge_reviewer",
			TargetProvider: "codex-thread",
			TargetID:       "thread-1",
			LastNotifiedAt: "2026-06-14T10:00:00Z",
			Reason:         "review ack timeout elapsed",
		}},
		CompletionHandbacks: []completionhandback.Handback{{
			TaskID:          "T-002",
			ToRole:          "ops",
			DeliveryStatus:  "notification_failed",
			SuggestedAction: "mapping_required",
			SuggestedCommand: "fairway record notification T-002 --domain ops " +
				"--state notification_delivered",
			Reason: "no provider target",
		}},
	}
	coordination := dashboardCoordinationIntelligence(plan, []store.TrackMemory{
		{TrackID: "fresh", Owner: "arch", ReviewBy: "2026-07-01", Disposition: "promote", PromotionTarget: "docs/design/model.md", UpdatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), NextActions: []string{"keep moving"}},
		{TrackID: "stale", UpdatedAt: now.Add(-25 * time.Hour).Format(time.RFC3339Nano), Blockers: []string{"needs refresh"}},
	}, now, 24*time.Hour)
	if coordination.MemoryTotal != 2 || coordination.MemoryStale != 1 {
		t.Fatalf("memory summary=%+v", coordination)
	}
	if coordination.Memories[0].Owner != "arch" || !coordination.Memories[0].PromotionDebt {
		t.Fatalf("memory lifecycle projection=%+v", coordination.Memories[0])
	}
	if coordination.OpenWaits != 2 || coordination.StaleWaits != 1 || coordination.NotificationFailures != 1 {
		t.Fatalf("wait summary=%+v", coordination)
	}
	if len(coordination.NextActions) != 1 || coordination.NextActions[0].Action != "resolve_wait" {
		t.Fatalf("next actions=%+v", coordination.NextActions)
	}
	if len(coordination.Waits) != 2 || coordination.Waits[0].State != "notification_failed" {
		t.Fatalf("wait ordering=%+v", coordination.Waits)
	}
	if coordination.Waits[1].SuggestedCommand != "fairway review-waits wake --task T-001" {
		t.Fatalf("review wait command=%q", coordination.Waits[1].SuggestedCommand)
	}
}
