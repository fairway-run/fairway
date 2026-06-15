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

func TestRetryBudgetSummaryAndCheckpoint(t *testing.T) {
	summary, err := RetryBudgetSummary(3, 2, 3, "RESET-001", "causal model refreshed")
	if err != nil {
		t.Fatal(err)
	}
	cp := store.Checkpoint{
		TaskID:    "LIVE-001",
		Summary:   summary,
		CreatedAt: "2026-06-14T20:00:00Z",
	}
	budget, ok := RetryBudgetFromCheckpoint(cp)
	if !ok {
		t.Fatalf("retry budget did not parse: %q", summary)
	}
	if budget.MeaningfulFailures != 3 || budget.CoordinationFailures != 2 || budget.Budget != 3 {
		t.Fatalf("budget=%+v", budget)
	}
	if budget.NextIteration != 4 || budget.Exhausted != true || budget.RequiresReset != false {
		t.Fatalf("budget status=%+v", budget)
	}
	if budget.ResetTask != "RESET-001" || budget.ResetReason != "causal model refreshed" {
		t.Fatalf("reset fields=%+v", budget)
	}

	summary, err = RetryBudgetSummary(3, 2, 3, "", "")
	if err != nil {
		t.Fatal(err)
	}
	budget, ok = RetryBudgetFromCheckpoint(store.Checkpoint{TaskID: "LIVE-001", Summary: summary})
	if !ok || !budget.RequiresReset {
		t.Fatalf("budget=%+v, want reset required", budget)
	}

	summary, err = RetryBudgetSummary(3, 2, 3, "RESET-001", "")
	if err != nil {
		t.Fatal(err)
	}
	budget, ok = RetryBudgetFromCheckpoint(store.Checkpoint{TaskID: "LIVE-001", Summary: summary})
	if !ok || !budget.RequiresReset {
		t.Fatalf("budget=%+v, want reset reason required", budget)
	}
}
