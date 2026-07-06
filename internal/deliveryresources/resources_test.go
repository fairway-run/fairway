package deliveryresources

import (
	"testing"
	"time"

	"github.com/subashram/fairway/internal/store"
)

func TestFromTasksClassifiesDeliveryResources(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	tasks := []store.Task{
		task("FW-1", "Restart shared dashboard", "ops", "done", "project-a"),
		task("FW-2", "Publish docs portal", "ops", "done", "project-a"),
		task("FW-3", "GitLab CI pipeline readback", "ops", "done", "project-a"),
		task("FW-4", "Demo environment preflight packet", "ops", "blocked", "project-a"),
	}
	evidence := map[string][]store.Evidence{
		"FW-1": {{Result: "pass", ArtifactType: "dashboard-status", ArtifactPath: "dashboard.txt", CommandText: "version v0.1.10 commit abcdef1", CreatedAt: now.Format(time.RFC3339Nano)}},
		"FW-2": {{Result: "pass", ArtifactType: "docs-publish", ArtifactPath: "docs.txt", CreatedAt: now.Format(time.RFC3339Nano)}},
		"FW-3": {{Result: "fail", ArtifactType: "ci", ArtifactPath: "pipeline.txt", CreatedAt: now.Format(time.RFC3339Nano)}},
		"FW-4": {{Result: "partial", ArtifactType: "preflight", ArtifactPath: "preflight.txt", CreatedAt: now.Format(time.RFC3339Nano)}},
	}
	rows := FromTasks(tasks, evidence, Options{Now: now})
	if len(rows) != 4 {
		t.Fatalf("rows=%d, want 4: %#v", len(rows), rows)
	}
	byID := map[string]Resource{}
	for _, row := range rows {
		byID[row.SourceTaskID] = row
	}
	if got := byID["FW-1"]; got.Type != "dashboard" || got.State != "verified" || got.LastVerifiedVersion != "v0.1.10" || got.LastVerifiedCommit != "abcdef1" {
		t.Fatalf("dashboard row = %#v", got)
	}
	if got := byID["FW-2"]; got.Type != "docs_portal" || got.State != "verified" {
		t.Fatalf("docs row = %#v", got)
	}
	if got := byID["FW-3"]; got.Type != "ci_pipeline" || got.State != "failed_verification" || len(got.OpenBlockers) == 0 {
		t.Fatalf("ci row = %#v", got)
	}
	if got := byID["FW-4"]; got.Type != "preflight_packet" || got.State != "blocked" || len(got.OpenBlockers) == 0 {
		t.Fatalf("preflight row = %#v", got)
	}
}

func TestFromTasksCoversDuplicateStaleAndHandoffReadyResources(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	old := now.Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano)
	tasks := []store.Task{
		task("L-1", "Demo environment", "ops", "done", "left"),
		task("R-1", "Demo environment", "ops", "done", "right"),
		task("P-1", "Environment rehearsal packet", "ops", "done", "left"),
	}
	evidence := map[string][]store.Evidence{
		"L-1": {{Result: "pass", ArtifactType: "smoke", CreatedAt: old}},
		"R-1": {{Result: "pass", ArtifactType: "smoke", CreatedAt: now.Format(time.RFC3339Nano)}},
		"P-1": {{Result: "pass", ArtifactType: "handoff-packet", ArtifactPath: "packet.md", CreatedAt: now.Format(time.RFC3339Nano)}},
	}
	rows := FromTasks(tasks, evidence, Options{Now: now})
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows {
		key := row.Project + "/" + row.Name
		if seen[key] {
			t.Fatalf("duplicate project/name key %s in %#v", key, rows)
		}
		seen[key] = true
		switch row.SourceTaskID {
		case "L-1":
			if row.State != "stale" {
				t.Fatalf("L-1 state=%s, want stale", row.State)
			}
		case "P-1":
			if row.State != "handoff_ready" {
				t.Fatalf("P-1 state=%s, want handoff_ready", row.State)
			}
		}
	}
}

func task(id, title, role, status, project string) store.Task {
	return store.Task{
		Definition: store.TaskDefinition{ID: id, Title: title, Role: role, Kind: "task"},
		Project:    project,
		Status:     status,
		Owner:      role,
		UpdatedAt:  "2026-07-06T12:00:00Z",
	}
}
