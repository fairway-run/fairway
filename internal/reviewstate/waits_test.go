package reviewstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestWaitsForTaskProjectsStatesAndActions(t *testing.T) {
	now := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	old := now.Add(-2 * time.Hour).Format(time.RFC3339Nano)
	recent := now.Add(-10 * time.Minute).Format(time.RFC3339Nano)
	task := store.Task{
		Definition: store.TaskDefinition{
			ID:            "REVIEW-001",
			ReviewDomains: []string{"arch", "ops", "security", "governance", "done", "changes", "cancelled"},
		},
		Status:    "review",
		UpdatedAt: recent,
	}
	handoffs := []store.Handoff{{ID: 7, ToRole: "ops", CreatedAt: recent}}
	notifications := []store.Notification{
		{ID: 1, TaskID: "REVIEW-001", Domain: "security", State: "thread_steered", Provider: "codex", Target: "thread-security", CreatedAt: old},
		{ID: 2, TaskID: "REVIEW-001", Domain: "governance", State: "notification_failed", Provider: "codex", Target: "thread-governance", Reason: "tool unavailable", CreatedAt: recent},
	}
	reviews := []store.Review{
		{Domain: "done", Reviewer: "done-reviewer", Verdict: "approve", CreatedAt: recent},
		{Domain: "changes", Reviewer: "changes-reviewer", Verdict: "changes", CreatedAt: recent},
	}

	waits := WaitsForTask(task, handoffs, reviews, notifications, ReviewWaitOptions{
		ProviderTargets: []config.ProviderTarget{{Domain: "security", Provider: "codex", Target: "thread-security", Type: "thread"}},
		Roles:           []config.Role{{Name: "arch", Provider: "codex"}, {Name: "ops", Provider: "codex"}, {Name: "governance", Provider: "codex"}},
		AckTimeout:      time.Hour,
		Now:             now,
		Terminal:        []string{"done"},
	})

	assertWait(t, waits, "arch", "pending", "deliver_notification", true)
	assertWait(t, waits, "ops", "pending", "record_delivery_proof", true)
	assertWait(t, waits, "security", "stale", "nudge_reviewer", true)
	assertWait(t, waits, "governance", "notification_failed", "deliver_notification", true)
	assertWait(t, waits, "done", "resolved", "run_merge_ready", false)
	assertWait(t, waits, "changes", "resolved", "address_review_changes", false)
	assertWait(t, waits, "cancelled", "notification_failed", "mapping_required", true)

	terminalTask := task
	terminalTask.Status = "done"
	terminalTask.Definition.ReviewDomains = []string{"arch"}
	cancelled := WaitsForTask(terminalTask, nil, nil, nil, ReviewWaitOptions{Roles: []config.Role{{Name: "arch"}}, Terminal: []string{"done"}, Now: now})
	assertWait(t, cancelled, "arch", "cancelled", "none", false)
}

func TestUnroutableRequiredDomains(t *testing.T) {
	task := store.Task{Definition: store.TaskDefinition{ID: "REVIEW-002", ReviewDomains: []string{"arch", "compliance", "product", "security"}}}
	issues := UnroutableRequiredDomains(task, ReviewWaitOptions{
		Roles:           []config.Role{{Name: "arch"}},
		DomainAliases:   map[string]string{"compliance": "arch"},
		ProviderTargets: []config.ProviderTarget{{Domain: "security", Provider: "codex", Target: "thread-security"}},
	})
	if len(issues) != 1 || issues[0].Domain != "product" || issues[0].Action != "mapping_required" {
		t.Fatalf("issues=%+v, want one product mapping_required issue", issues)
	}
}

func TestWaitsForTaskSkipsTodoBacklog(t *testing.T) {
	task := store.Task{
		Definition: store.TaskDefinition{ID: "TODO-001", ReviewDomains: []string{"arch"}},
		Status:     "todo",
	}
	waits := WaitsForTask(task, nil, nil, nil, ReviewWaitOptions{Roles: []config.Role{{Name: "arch"}}})
	if len(waits) != 0 {
		t.Fatalf("todo waits=%+v, want none", waits)
	}
}

func TestReviewWaitProjectionDoesNotAddWaitStore(t *testing.T) {
	migrations, err := filepath.Glob(filepath.Join("..", "store", "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) == 0 {
		t.Fatal("no store migrations found")
	}
	for _, migration := range migrations {
		body, err := os.ReadFile(migration)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(body)), "review_wait") {
			t.Fatalf("review wait projection added store state in %s", migration)
		}
	}
}

func assertWait(t *testing.T, waits []ReviewWait, domain, state, action string, blocking bool) {
	t.Helper()
	for _, wait := range waits {
		if wait.Domain != domain {
			continue
		}
		if wait.State != state || wait.Action != action || wait.Blocking != blocking {
			t.Fatalf("%s wait = state/action/blocking %s/%s/%t, want %s/%s/%t; full=%+v", domain, wait.State, wait.Action, wait.Blocking, state, action, blocking, waits)
		}
		return
	}
	t.Fatalf("missing wait for %s in %+v", domain, waits)
}
