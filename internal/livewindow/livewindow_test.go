package livewindow

import (
	"testing"

	"github.com/subashram/fairway/internal/store"
)

func TestSummaryAndStatusFromCheckpoint(t *testing.T) {
	summary, err := Summary("gate-running", "ops", "run browser smoke")
	if err != nil {
		t.Fatal(err)
	}
	cp := store.Checkpoint{
		TaskID:        "LIVE-001",
		State:         "active",
		Owner:         "ops",
		TargetCloseBy: "2026-06-13T03:15:00Z",
		Summary:       summary,
		ArtifactPath:  "packet.md",
		CreatedAt:     "2026-06-13T02:15:00Z",
	}
	status, ok := StatusFromCheckpoint(cp)
	if !ok {
		t.Fatalf("checkpoint did not parse: %q", summary)
	}
	if status.Phase != "gate-running" || status.NextOwner != "ops" || status.NextAction != "run browser smoke" || status.TargetCloseBy != cp.TargetCloseBy || status.ArtifactPath != cp.ArtifactPath {
		t.Fatalf("status=%+v", status)
	}
}

func TestSummaryRejectsUnknownPhase(t *testing.T) {
	if _, err := Summary("improvise", "ops", "continue"); err == nil {
		t.Fatal("expected invalid phase error")
	}
}
