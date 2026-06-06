package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestCLI_Smoke(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	writeFile(t, "tasks.yaml", `- id: T-001
  title: Smoke
  role: backend
  notes: Test task
`)
	runOK(t, "import", "tasks.yaml")
	runOK(t, "ready")
	runOK(t, "--json", "ready")
	t.Setenv("FAIRWAY_ROLE", "backend")
	runOK(t, "claim", "T-001")
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass", "--duration-seconds", "1")
	runOK(t, "--json", "record", "guard-report", "T-001", "--guard", "import-boundary", "--mode", "report_only", "--finding", "cmd/api imports billing internals", "--false-positive", "generated code", "--allowed-debt", "legacy route", "--graduation-criteria", "zero critical findings", "--artifact", "dist/import-boundary.json")
	runOK(t, "route", "review", "T-001", "--reviewer", "ui", "--reason", "smoke")
	runOK(t, "record", "review", "T-001", "--reviewer", "ui", "--verdict", "approve", "--reason", "ok")
	runOK(t, "set-status", "T-001", "done")
	runOK(t, "task-detail", "T-001")
	runOK(t, "--json", "task-detail", "T-001")
	runOK(t, "status-report")
	runOK(t, "--json", "status-report")
	runOK(t, "health-report")
	runOK(t, "--json", "health-report")
	runOK(t, "timing-report")
	runOK(t, "--json", "timing-report")
	runOK(t, "db", "export", "snapshot.json")
	runOK(t, "db", "backup", "backup.db")
}

func TestCLI_HelpAliases(t *testing.T) {
	for _, args := range [][]string{
		{"help"},
		{"--help"},
		{"-h"},
	} {
		out := runCapture(t, args...)
		assertContains(t, out, "fairway init|import|add")
	}
}

func TestCLI_GroupHelpAliases(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"session", "--help"}, "fairway session upsert|status|end|reconcile|launch"},
		{[]string{"session", "help"}, "fairway session upsert|status|end|reconcile|launch"},
		{[]string{"reconcile", "--help"}, "fairway reconcile active"},
		{[]string{"worktree", "-h"}, "fairway worktree setup|status|prune"},
		{[]string{"record", "--help"}, "fairway record evidence|guard-report|handoff|review"},
		{[]string{"dashboard", "--help"}, "fairway dashboard [--listen <addr>]"},
		{[]string{"db", "--help"}, "fairway db backup|export|migrate|compat"},
		{[]string{"workflow", "--help"}, "fairway workflow check"},
		{[]string{"audit", "--help"}, "fairway audit work-coverage|ci-learning"},
		{[]string{"release", "--help"}, "fairway release verify"},
	} {
		out := runCapture(t, tc.args...)
		assertContains(t, out, tc.want)
	}
}

func TestCLI_RequiresEvidenceGate(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	replaceInFile(t, ".fairway/config.toml", "require_evidence_before_done = false", "require_evidence_before_done = true")
	writeFile(t, "tasks.yaml", `- id: T-001
  title: Gate
  role: backend
`)
	runOK(t, "import", "tasks.yaml")
	if err := run(context.Background(), []string{"set-status", "T-001", "done"}); err == nil {
		t.Fatal("expected evidence gate error")
	}
}

func TestCLI_ReadyPriorityFilter(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	writeFile(t, "tasks.yaml", `- id: T-001
  title: High
  role: backend
  priority: 1
- id: T-002
  title: Low
  role: backend
  priority: 3
`)
	runOK(t, "import", "tasks.yaml")
	runOK(t, "ready", "--priority", "1")
}

func TestCLI_ReadyInFiltersDescendants(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	writeFile(t, "tasks.yaml", `- id: E-001
  title: Epic
  role: backend
- id: T-001
  parent_id: E-001
  title: Child
  role: backend
- id: T-002
  title: Other
  role: backend
`)
	runOK(t, "import", "tasks.yaml")
	runOK(t, "ready", "--in", "E-001")
	runOK(t, "claim", "--in", "E-001")
}

func TestCLI_AddTask(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "Root", "--role", "backend")
	runOK(t, "add", "T-002", "--title", "Child", "--role", "backend", "--parent", "T-001", "--dependencies", "T-001", "--acceptance", "go test ./...")
	runOK(t, "update", "T-002", "--title", "Updated child", "--notes", "kept in db", "--priority", "1")
	runOK(t, "task-detail", "T-002")
	runOK(t, "tree", "T-001")
	runOK(t, "--json", "tree", "T-001")
}

func TestCLI_SpawnTask(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "add", "E-001", "--title", "Epic", "--kind", "epic", "--role", "backend", "--priority", "1")
	runOK(t, "add", "T-001", "--title", "Current", "--role", "backend", "--parent", "E-001", "--priority", "1")
	runOK(t, "session", "upsert", "--id", "s-1", "--role", "backend", "--task-id", "T-001")
	t.Setenv("FAIRWAY_ROLE", "backend")
	runOK(t, "spawn", "--id", "T-002", "--title", "Sibling follow-up")
	runOK(t, "spawn", "--id", "B-001", "--title", "Child bug", "--kind", "bug", "--child", "--force")
	runOK(t, "tree", "E-001")
}

func TestCLI_TaskMetadata(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[workstream_profiles]]
name = "platform-foundation"
task_kinds = ["architecture-map"]
`)
	runOK(t, "add", "T-001",
		"--title", "Map route ownership",
		"--role", "backend",
		"--kind", "architecture-map",
		"--profile", "platform-foundation",
		"--owning-domain", "platform",
		"--owning-layer", "api",
		"--source-paths", "cmd/api,doc/api",
		"--target-paths", "packages/services/platform",
		"--review-domains", "architecture,security",
		"--risk-level", "medium",
		"--migration-type", "facade")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "metadata:")
	assertContains(t, detail, "profile: platform-foundation")
	assertContains(t, detail, "source_paths: cmd/api, doc/api")

	jsonDetail := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, jsonDetail, `"owning_domain": "platform"`)
	assertContains(t, jsonDetail, `"review_domains": [`)

	runOK(t, "update", "T-001", "--risk-level", "high", "--source-paths", "cmd/api/routes.go")
	updated := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, updated, `"risk_level": "high"`)
	assertContains(t, updated, `"cmd/api/routes.go"`)

	if err := run(context.Background(), []string{"add", "T-002", "--title", "Bad profile", "--role", "backend", "--profile", "missing"}); err == nil {
		t.Fatal("expected unknown profile validation error")
	}
}

func TestCLI_Preflight(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	runOK(t, "init")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")
	runOK(t, "preflight", "--role", "backend")
	runOK(t, "--json", "preflight", "--role", "backend")
	runOK(t, "add", "T-001", "--title", "Merge", "--role", "backend")
	runOK(t, "merge-ready", "T-001")
	runOK(t, "--json", "merge-ready", "T-001")
}

func TestCLI_WorkflowCheckWarnsOnDirtyDocsAndUnpushedCommits(t *testing.T) {
	repo := t.TempDir()
	remote := filepath.Join(t.TempDir(), "origin.git")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, remote, "init", "--bare")
	git(t, repo, "remote", "add", "origin", remote)
	runOK(t, "init")
	writeFile(t, "README.md", "hello\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")
	git(t, repo, "push", "-u", "origin", "main")

	runOK(t, "workflow", "check")
	runOK(t, "--json", "workflow", "check")

	appendFile(t, "README.md", "docs update\n")
	dirty := runCapture(t, "workflow", "check")
	assertContains(t, dirty, "worktree has uncommitted changes")
	assertContains(t, dirty, "commit completed documentation updates")

	git(t, repo, "add", "README.md")
	git(t, repo, "commit", "-m", "docs: update readme")
	unpushed := runCapture(t, "workflow", "check")
	assertContains(t, unpushed, "branch has 1 unpushed commit(s)")
	assertContains(t, unpushed, "push integration-ready commits so CI can run")
	if err := run(context.Background(), []string{"workflow", "check", "--require-pushed"}); err == nil {
		t.Fatal("expected require-pushed to fail")
	}
	deploy := runCaptureAllowError(t, "workflow", "check", "--mode", "deploy")
	assertContains(t, deploy, "deploy mode requires pushed commits so CI can run")
	assertContains(t, deploy, "create one deploy-run task")
}

func TestCLI_MergeReadyEvaluatesBlockingProfileGates(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[workstream_profiles]]
name = "release-readiness"
task_kinds = ["release-risk"]

[[workstream_profiles.gates]]
name = "security-review"
mode = "blocking"
evidence_type = "security-review"
required_evidence_count = 1
accepted_results = ["pass"]
artifact_required = true
`)
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")

	runOK(t, "add", "T-001", "--title", "Risk", "--role", "backend", "--kind", "release-risk")
	if err := run(context.Background(), []string{"merge-ready", "T-001"}); err == nil {
		t.Fatal("expected merge-ready profile gate error")
	}
	failed := runCaptureAllowError(t, "merge-ready", "T-001")
	assertContains(t, failed, "profile gate release-readiness/security-review missing")

	runOK(t, "record", "evidence", "T-001", "--command-text", "security review", "--result", "pass", "--artifact", "dist/security.txt", "--artifact-type", "security-review")
	runOK(t, "merge-ready", "T-001")
	jsonReport := runCapture(t, "--json", "merge-ready", "T-001")
	assertContains(t, jsonReport, `"gate_evaluations"`)
	assertContains(t, jsonReport, `"status": "satisfied"`)
}

func TestCLI_ReadinessReportEvaluatesProfileGates(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[workstream_profiles]]
name = "release-readiness"
task_kinds = ["release-risk"]

[[workstream_profiles.gates]]
name = "release-owner-approval"
mode = "blocking"
evidence_type = "approval"
required_evidence_count = 1
accepted_results = ["pass"]
`)
	runOK(t, "add", "T-001", "--title", "Approve release", "--role", "backend", "--kind", "release-risk")
	failed := runCaptureAllowError(t, "readiness", "report", "--profile", "release-readiness")
	assertContains(t, failed, "ready: false")
	assertContains(t, failed, "release-readiness/release-owner-approval")
	jsonFailed := runCapture(t, "--json", "readiness", "report", "--profile", "release-readiness")
	assertContains(t, jsonFailed, `"ok": false`)

	runOK(t, "record", "evidence", "T-001", "--command-text", "release owner approval", "--result", "pass", "--artifact-type", "approval")
	report := runCapture(t, "readiness", "report", "--profile", "release-readiness")
	assertContains(t, report, "ready: true")
	assertContains(t, report, "satisfied")
}

func TestCLI_ProfileGateTaskKindsScopeReadiness(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[workstream_profiles]]
name = "platform-foundation"
task_kinds = ["architecture-map", "boundary-guard"]

[[workstream_profiles.gates]]
name = "boundary-guard-report"
mode = "advisory"
task_kinds = ["boundary-guard"]
evidence_type = "guard-report"
required_evidence_count = 1
accepted_results = ["pass"]
artifact_required = true
`)
	runOK(t, "add", "PF-001", "--title", "Ownership map", "--role", "orchestrator", "--kind", "architecture-map")
	runOK(t, "add", "PF-002", "--title", "Boundary guard", "--role", "backend", "--kind", "boundary-guard")

	jsonReport := runCapture(t, "--json", "readiness", "report", "--profile", "platform-foundation")
	assertContains(t, jsonReport, `"task_count": 1`)
	assertContains(t, jsonReport, `"missing_count": 1`)
	assertContains(t, jsonReport, `"task_id": "PF-002"`)
	if strings.Contains(jsonReport, `"task_id": "PF-001"`) {
		t.Fatalf("gate scoped to boundary-guard included architecture-map task:\n%s", jsonReport)
	}
}

func TestCLI_MergeReadyRequiresReviewDomains(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	runOK(t, "init")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")
	runOK(t, "add", "T-001", "--title", "Needs domains", "--role", "backend", "--review-domains", "architecture,security")

	failed := runCaptureAllowError(t, "merge-ready", "T-001")
	assertContains(t, failed, "missing approved review for domain architecture")
	assertContains(t, failed, "missing approved review for domain security")
	jsonFailed := runCaptureAllowError(t, "--json", "merge-ready", "T-001")
	assertContains(t, jsonFailed, `"missing_review_domains"`)
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "missing review domains:")
	assertContains(t, detail, "- architecture")
	assertContains(t, detail, "- security")
	jsonDetail := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, jsonDetail, `"missing_review_domains"`)

	runOK(t, "record", "review", "T-001", "--reviewer", "architecture", "--verdict", "approve", "--reason", "arch ok")
	failed = runCaptureAllowError(t, "merge-ready", "T-001")
	assertContains(t, failed, "missing approved review for domain security")
	runOK(t, "record", "review", "T-001", "--reviewer", "security", "--verdict", "approve", "--reason", "security ok")
	runOK(t, "merge-ready", "T-001")
}

func TestCLI_WorktreeSetupStatus(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[roles]]
name = "backend"

[[roles]]
name = "ui"
branch = "agent/ui"
`)
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")

	runOK(t, "worktree", "setup")
	runOK(t, "worktree", "status")
	runOK(t, "--json", "worktree", "status")
}

func TestCLI_ReviewCheckout(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[roles]]
name = "backend"
branch = "agent/backend"
`)
	runOK(t, "add", "T-001", "--title", "Review me", "--role", "backend")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")
	git(t, repo, "branch", "agent/backend")

	runOK(t, "review", "checkout", "T-001")
	runOK(t, "--json", "git-check", "--base", "main")
}

func TestCLI_SessionLifecycle(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[roles]]
name = "backend"
branch = "agent/backend"
provider = "codex"
`)
	runOK(t, "session", "upsert", "--id", "s-1", "--role", "backend", "--pid", "123")
	runOK(t, "session", "status")
	runOK(t, "--json", "session", "status")
	runOK(t, "session", "end", "s-1", "--reason", "normal", "--exit-code", "0")
	runOK(t, "session", "status", "--all")
	runOK(t, "session", "upsert", "--id", "s-dead", "--role", "backend", "--pid", "999999")
	runOK(t, "session", "reconcile", "--dry-run")
	runOK(t, "session", "reconcile")
	runOK(t, "session", "launch", "--role", "backend")
}

func TestCLI_CoordinatorStatus(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[roles]]
name = "backend"
`)
	writeFile(t, "tasks.yaml", `- id: T-001
  title: Coord
  role: backend
`)
	runOK(t, "import", "tasks.yaml")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")
	runOK(t, "worktree", "setup")
	runOK(t, "coordinator", "status")
	runOK(t, "--json", "coordinator", "tick")
	runOK(t, "coordinator", "preflight")
	runOK(t, "dispatch-plan")
	runOK(t, "--json", "dispatch-plan", "--role", "backend")
}

func TestCLI_CheckpointAndPacket(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "Packet", "--role", "backend", "--notes", "carry context")
	runOK(t, "checkpoint", "record", "T-001", "--state", "active", "--owner", "backend", "--target-close-by", "2026-01-01", "--summary", "working")
	runOK(t, "checkpoint", "status")
	runOK(t, "--json", "checkpoint", "stale", "--before", "2026-02-01")
	runOK(t, "packet", "context", "T-001", "--goal", "finish", "--owner", "backend", "--acceptance", "tests pass")
	runOK(t, "--json", "packet", "context", "T-001", "--goal", "finish")
	runOK(t, "packet", "bugfix", "T-001", "--bug-summary", "broken", "--root-cause", "missing guard", "--proof-command", "go test ./...", "--regression-coverage", "unit")
	runOK(t, "--json", "packet", "watcher", "W-001", "--owner", "ops/watch", "--process", "ci", "--command", "gh run watch", "--success", "green", "--failure", "red")
	releasePacket := runCapture(t, "packet", "release-run", "T-001", "--version", "v0.1.2", "--tag", "v0.1.2", "--source-sha", "abc123", "--release-notes", "docs/release-notes.md", "--changelog-state", "updated", "--ci-status", "pass", "--docs-status", "pass", "--signing-status", "pass", "--notary-status", "pass", "--release-url", "https://github.com/fairway-run/fairway/releases/tag/v0.1.2", "--homebrew-tap-commit", "tap123", "--verification-command", "brew fetch --cask --force fairway-run/tap/fairway")
	assertContains(t, releasePacket, "# Release Run Packet: T-001")
	assertContains(t, releasePacket, "GitHub release is public before Homebrew is treated as usable")
	runOK(t, "packet", "architecture-map", "T-001", "--scope", "package ownership", "--current-owner", "mixed", "--target-owner", "backend", "--migration-risk", "move-only churn", "--source-doc", "doc/architecture/platform-foundation/ownership.md", "--acceptance", "map reviewed")
	runOK(t, "--json", "packet", "boundary-guard", "T-001", "--guard-intent", "report imports across package boundaries", "--finding", "cmd/api imports internal billing", "--false-positive", "generated code", "--graduation-criteria", "zero critical findings", "--proof-command", "go test ./...")
	runOK(t, "packet", "vertical-slice", "T-001", "--target-seam", "platform evidence facade", "--old-path", "cmd/api/evidence.go", "--new-path", "packages/services/platform/evidence.go", "--adapter", "thin route adapter", "--proof-command", "go test ./cmd/api ./packages/services/platform", "--rollback-plan", "revert adapter wiring")
	runOK(t, "watcher", "start", "W-001", "--task", "T-001", "--owner", "ops/watch", "--process", "ci", "--command", "gh run watch", "--success", "green", "--failure", "red")
	runOK(t, "watcher", "status")
	runOK(t, "watcher", "finish", "W-001", "--result", "pass", "--artifact", "ci.txt", "--duration-seconds", "5")
	runOK(t, "--json", "watcher", "status", "--include-done")
}

func TestCLI_TmuxSessionTranscriptAndReconcile(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "Tmux lane", "--role", "backend")
	runOK(t, "session", "upsert",
		"--id", "tmux-backend-test",
		"--role", "backend",
		"--backend", "tmux",
		"--provider", "claude",
		"--name", "fairway-backend",
		"--task-id", "T-001",
		"--tmux-pane", "%fairway-missing-pane",
		"--transcript", ".fairway/transcripts/backend-T-001.log",
		"--worktree", repo,
		"--branch", "agent/backend",
	)
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "tmux-backend-test")
	assertContains(t, detail, ".fairway/transcripts/backend-T-001.log")
	jsonDetail := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, jsonDetail, `"transcript_path": ".fairway/transcripts/backend-T-001.log"`)

	dryRun := runCapture(t, "session", "reconcile", "--dry-run")
	assertContains(t, dryRun, "tmux pane not found")
	runOK(t, "session", "reconcile")
	allSessions := runCapture(t, "session", "status", "--all")
	assertContains(t, allSessions, "tmux-backend-test")
	assertContains(t, allSessions, "stale")
	task := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, task, `"Status": "todo"`)
}

func TestCLI_SessionReconcileReportsHousekeepingMismatches(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "Completed but attached", "--role", "backend")
	runOK(t, "add", "T-002", "--title", "Claimed without session", "--role", "ops")
	t.Setenv("FAIRWAY_ROLE", "ops")
	runOK(t, "claim", "T-002")
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass")
	runOK(t, "set-status", "T-001", "done")
	runOK(t, "session", "upsert",
		"--id", "codex-stale",
		"--role", "orchestrator",
		"--backend", "codex-thread",
		"--provider", "codex",
		"--task-id", "T-001",
	)

	dryRun := runCapture(t, "session", "reconcile", "--dry-run")
	assertContains(t, dryRun, "mark_stale")
	assertContains(t, dryRun, "session task is terminal")
	assertContains(t, dryRun, "report_unattended_in_progress")
	assertContains(t, dryRun, "T-002")

	jsonReport := runCapture(t, "--json", "session", "reconcile", "--dry-run")
	assertContains(t, jsonReport, `"action": "mark_stale"`)
	assertContains(t, jsonReport, `"task_id": "T-001"`)
	assertContains(t, jsonReport, `"action": "report_unattended_in_progress"`)

	runOK(t, "session", "reconcile")
	allSessions := runCapture(t, "session", "status", "--all")
	assertContains(t, allSessions, "codex-stale")
	assertContains(t, allSessions, "stale")
}

func TestCLI_ReconcileActiveReportsTaskEvidenceAndParentFindings(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "add", "E-001", "--title", "Parent backlog", "--kind", "epic", "--role", "ops")
	runOK(t, "add", "T-001", "--title", "Child work", "--role", "ops", "--parent", "E-001")
	t.Setenv("FAIRWAY_ROLE", "ops")
	runOK(t, "claim", "E-001")
	runOK(t, "claim", "T-001")
	evidenceOut := runCapture(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass")
	assertContains(t, evidenceOut, "next: mark done")
	runOK(t, "session", "upsert",
		"--id", "codex-active",
		"--role", "ops",
		"--backend", "codex-thread",
		"--provider", "codex",
		"--task-id", "T-001",
	)

	report := runCapture(t, "reconcile", "active", "--dry-run")
	assertContains(t, report, "status_decision_required")
	assertContains(t, report, "active_parent_without_rollup")
	assertContains(t, report, "E-001")
	assertContains(t, report, "T-001")

	jsonReport := runCapture(t, "--json", "reconcile", "active", "--dry-run")
	assertContains(t, jsonReport, `"status_decision_required": 1`)
	assertContains(t, jsonReport, `"kind": "active_parent_without_rollup"`)
}

func TestCLI_ReconcileActiveReportsMonitorSessionsWithoutBackingProof(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "CI monitor without proof", "--role", "ops")
	runOK(t, "add", "T-002", "--title", "CI monitor with automation", "--role", "ops")
	t.Setenv("FAIRWAY_ROLE", "ops")
	runOK(t, "claim", "T-001")
	runOK(t, "claim", "T-002")
	runOK(t, "session", "upsert",
		"--id", "ci-missing",
		"--role", "ops/watch",
		"--backend", "ci-monitor",
		"--task-id", "T-001",
		"--monitor-kind", "ci",
	)
	runOK(t, "session", "upsert",
		"--id", "ci-backed",
		"--role", "ops/watch",
		"--backend", "ci-monitor",
		"--task-id", "T-002",
		"--monitor-kind", "ci",
		"--automation-id", "heartbeat-ci-backed",
	)

	report := runCapture(t, "reconcile", "active", "--dry-run")
	assertContains(t, report, "monitor_session_without_backing_proof")
	assertContains(t, report, "monitor_sessions_no_proof=1")
	assertContains(t, report, "ci-missing")
	assertNotContains(t, report, "ci-backed")

	jsonReport := runCapture(t, "--json", "reconcile", "active", "--dry-run")
	assertContains(t, jsonReport, `"monitor_sessions_no_proof": 1`)
	assertContains(t, jsonReport, `"session_id": "ci-missing"`)
	assertNotContains(t, jsonReport, `"session_id": "ci-backed"`)
}

func TestCLI_AuditWorkCoverage(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	gitInit(t, repo)

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "Covered API", "--role", "backend", "--source-paths", "cmd/api")
	runOK(t, "add", "T-002", "--title", "Needs evidence", "--role", "backend", "--review-domains", "architecture")
	runOK(t, "add", "T-003", "--title", "Evidence open", "--role", "backend")
	runOK(t, "set-status", "T-002", "done")
	replaceInFile(t, ".fairway/config.toml", "require_evidence_before_done = false", "require_evidence_before_done = true")
	runOK(t, "record", "evidence", "T-003", "--command-text", "go test ./...", "--result", "pass")

	writeFile(t, "README.md", "base\n")
	gitAddCommit(t, repo, "base")
	baseRef := gitRevParse(t, repo, "HEAD")
	writeFile(t, "cmd/api/routes.go", "package main\n")
	gitAddCommit(t, repo, "T-001 route update")
	writeFile(t, "docs/plan.md", "plan\n")
	gitAddCommit(t, repo, "misc docs update")

	report := runCapture(t, "audit", "work-coverage", "--since-ref", baseRef, "--dry-run")
	for _, want := range []string{
		"dry-run: advisory audit only",
		"commit_without_task_coverage",
		"changed_file_uncovered",
		"done_without_required_evidence",
		"evidence_without_status_decision",
		"missing_required_review_domains",
		"docs/plan.md",
		"T-002",
		"T-003",
	} {
		assertContains(t, report, want)
	}

	jsonReport := runCapture(t, "--json", "audit", "work-coverage", "--since-ref", baseRef)
	assertContains(t, jsonReport, `"kind": "commit_without_task_coverage"`)
	assertContains(t, jsonReport, `"done_without_required_evidence": 1`)
	assertContains(t, jsonReport, `"missing": [`)
	assertContains(t, jsonReport, `"architecture"`)

	taskReport := runCapture(t, "audit", "work-coverage", "--since-ref", baseRef, "--task-id", "T-001")
	assertContains(t, taskReport, "task_id: T-001")
	if strings.Contains(taskReport, "T-002") || strings.Contains(taskReport, "T-003") {
		t.Fatalf("task-scoped audit included unrelated task findings:\n%s", taskReport)
	}

	durationReport := runCapture(t, "--json", "audit", "work-coverage", "--since-duration", "24h")
	assertContains(t, durationReport, `"since_duration": "24h0m0s"`)
}

func TestCLI_AuditCILearning(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	replaceInFile(t, ".fairway/config.toml", `task_id_pattern = "^[A-Z]+-[0-9]+$"`, `task_id_pattern = "^[A-Z][A-Z0-9-]*$"`)
	runOK(t, "add", "T-001", "--title", "CI failure", "--role", "backend")
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "fail", "--artifact-type", "ci")

	report := runCapture(t, "audit", "ci-learning", "--template")
	for _, want := range []string{
		"ci_learning_ok: false",
		"missed_local_gate",
		"missing follow-up: create CI-FIX-* task",
		"# CI/Deploy Learning: T-001",
		"Expected local reproduction: go test ./...",
	} {
		assertContains(t, report, want)
	}

	jsonReport := runCapture(t, "--json", "audit", "ci-learning")
	assertContains(t, jsonReport, `"failure_class": "missed_local_gate"`)
	assertContains(t, jsonReport, `"recommended_follow_up_task_id": "CI-FIX-T-001"`)

	runOK(t, "add", "T-002", "--title", "CI environment only", "--role", "ops")
	runOK(t, "record", "evidence", "T-002", "--command-text", "github actions deploy smoke", "--result", "fail", "--artifact-type", "deploy", "--notes", "CI only environment variable differs")
	runOK(t, "add", "CD-FIX-T-002", "--title", "Follow up T-002 deploy env", "--role", "ops")
	envReport := runCapture(t, "--json", "audit", "ci-learning", "--task-id", "T-002", "--template")
	assertContains(t, envReport, `"failure_class": "ci_environment_only"`)
	assertContains(t, envReport, `"follow_up_task": "CD-FIX-T-002"`)
	assertContains(t, envReport, `"missing_follow_ups": 0`)

	runOK(t, "add", "T-003", "--title", "Clean pipeline", "--role", "ops")
	runOK(t, "record", "evidence", "T-003", "--command-text", "ci pipeline", "--result", "pass", "--artifact-type", "ci")
	cleanReport := runCapture(t, "audit", "ci-learning", "--task-id", "T-003")
	assertContains(t, cleanReport, "ci_learning_ok: true")
	assertContains(t, cleanReport, "no CI/deploy learning findings")
}

func TestCLI_ReleaseVerifyScenarios(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	writeFile(t, "docs/release-notes.md", "## v0.1.2\n- release notes\n")
	writeFile(t, "CHANGELOG.md", "## v0.1.2\n- changes\n")

	cleanArgs := []string{
		"release", "verify",
		"--version", "v0.1.2",
		"--tag", "v0.1.2",
		"--source-sha", "abc123",
		"--ci-status", "pass",
		"--docs-status", "pass",
		"--signing-status", "pass",
		"--notary-status", "pass",
		"--release-state", "public",
		"--release-url", "https://github.com/fairway-run/fairway/releases/tag/v0.1.2",
		"--asset", "https://github.com/fairway-run/fairway/releases/download/v0.1.2/fairway.tar.gz=200",
		"--homebrew-version", "v0.1.2",
		"--homebrew-tap-commit", "tap123",
		"--brew-fetch-status", "pass",
		"--verification-command", "brew fetch --cask --force fairway-run/tap/fairway",
	}
	runOK(t, cleanArgs...)
	cleanJSON := runCapture(t, append([]string{"--json"}, cleanArgs...)...)
	assertContains(t, cleanJSON, `"ok": true`)

	draftArgs := append([]string{}, cleanArgs...)
	for i := range draftArgs {
		if draftArgs[i] == "public" {
			draftArgs[i] = "draft"
			break
		}
	}
	draft := runCaptureAllowError(t, draftArgs...)
	assertContains(t, draft, "Homebrew cask points to this version while GitHub release is still draft")

	missingNotes := runCaptureAllowError(t, "release", "verify", "--version", "v0.1.2", "--tag", "v0.1.2", "--release-notes", "docs/missing.md", "--changelog", "CHANGELOG.md", "--ci-status", "pass", "--docs-status", "pass", "--signing-status", "pass", "--notary-status", "pass", "--release-state", "public", "--asset", "https://example.test/fairway.tar.gz=200", "--homebrew-version", "v0.1.2", "--homebrew-tap-commit", "tap123", "--brew-fetch-status", "pass")
	assertContains(t, missingNotes, "missing release notes")

	failedAsset := runCaptureAllowError(t, "release", "verify", "--version", "v0.1.2", "--tag", "v0.1.2", "--ci-status", "pass", "--docs-status", "pass", "--signing-status", "pass", "--notary-status", "pass", "--release-state", "public", "--asset", "https://github.com/fairway-run/fairway/releases/download/v0.1.2/fairway.tar.gz=404", "--homebrew-version", "v0.1.2", "--homebrew-tap-commit", "tap123", "--brew-fetch-status", "pass")
	assertContains(t, failedAsset, "asset URL failed")
	assertContains(t, failedAsset, "status=404")
}

func TestCLI_PacketTemplate(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[packet_templates]]
name = "release-risk"
required_fields = ["risk", "mitigation"]
optional_fields = ["owner", "proof_command"]
`)
	runOK(t, "config", "validate")
	runOK(t, "add", "T-001", "--title", "Templated packet", "--role", "backend", "--profile", "", "--risk-level", "high")
	packet := runCapture(t, "packet", "template", "release-risk", "T-001", "--field", "risk=deploy may fail", "--field", "mitigation=rollback", "--field", "proof_command=go test ./...")
	assertContains(t, packet, "# Release Risk Packet: T-001")
	assertContains(t, packet, "deploy may fail")
	jsonPacket := runCapture(t, "--json", "packet", "template", "release-risk", "T-001", "--field", "risk=deploy may fail", "--field", "mitigation=rollback")
	assertContains(t, jsonPacket, `"Name": "release-risk"`)
	assertContains(t, jsonPacket, `"risk"`)
	if err := run(context.Background(), []string{"packet", "template", "release-risk", "T-001", "--field", "risk=deploy may fail"}); err == nil {
		t.Fatal("expected missing required template field error")
	}
	if err := run(context.Background(), []string{"packet", "template", "release-risk", "T-001", "--field", "risk=deploy may fail", "--field", "mitigation=rollback", "--field", "extra=nope"}); err == nil {
		t.Fatal("expected unknown template field error")
	}
}

func TestCLI_RegressionPacks(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	writeFile(t, "regression-packs.yaml", `packs:
  - id: RP-001
    title: Smoke flow
    owner: backend
    target_environments: [local, ci]
    blocking: true
    required_seed_data: [sample]
    lowest_reliable_layer: integration
    required_proof: ["go test ./..."]
    artifact_requirements: [logs]
    current_automation: [ci]
    gaps: [browser]
`)
	runOK(t, "regression-pack", "validate")
	runOK(t, "regression-pack", "list")
	runOK(t, "--json", "regression-pack", "list")
	runOK(t, "regression-pack", "show", "RP-001")
}

func TestCLI_GPUaaSParityFixtures(t *testing.T) {
	sourceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	gpuaasConfigPath := filepath.Join(sourceRoot, "examples", "gpuaas-a-b-c-d-e-config.toml")
	cfg, _, err := config.Load(gpuaasConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		".github/workflows/ci.yml":                          "C-ops",
		"scripts/ci/contracts_validate.sh":                  "E-governance",
		"scripts/ops/agent_queue.sh":                        "C-ops",
		"doc/api/openapi.draft.yaml":                        "D-arch",
		"doc/architecture/platform-foundation/ownership.md": "D-arch",
		"doc/architecture/Runtime.md":                       "D-arch",
		"doc/governance/Agent_Queue.md":                     "E-governance",
		"docs/design/state-machine.md":                      "D-arch",
		"packages/web/src/App.tsx":                          "B-ui",
		"cmd/api/routes.go":                                 "E-governance",
		"cmd/terminal-gateway/main.go":                      "E-governance",
		"cmd/node-agent/main.go":                            "E-governance",
		"cmd/billing-worker/main.go":                        "E-governance",
		"cmd/provisioning-worker/main.go":                   "E-governance",
		"packages/services/auth/service.go":                 "E-governance",
		"packages/services/billing/service.go":              "E-governance",
		"packages/services/payments/service.go":             "E-governance",
		"packages/services/provisioning/worker/service.go":  "E-governance",
		"packages/services/terminal/proxy.go":               "E-governance",
	}
	for changedPath, want := range cases {
		got, reason := matchReviewRoute(cfg.ReviewRoutes, []string{changedPath})
		if got != want {
			t.Fatalf("route for %s = %q (%s), want %q", changedPath, got, reason, want)
		}
	}

	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	replaceInFile(t, ".fairway/config.toml", `task_id_pattern = "^[A-Z]+-[0-9]+$"`, `task_id_pattern = "^[A-Z][A-Z0-9-]*$"`)
	writeFile(t, "gpuaas-queue.yaml", `version: 1
tasks:
  - id: A-DEMO-UAT-001
    title: Done dependency
    role: A-backend
    status: done
    owner: Codex
    branch: agent/A-backend
    commit: abc123
  - id: B-DEMO-UAT-002
    title: Ready after dependency
    role: B-ui
    depends_on: [A-DEMO-UAT-001]
`)
	runOK(t, "import", "gpuaas-queue.yaml", "--state-once")
	ready := runCapture(t, "ready")
	assertContains(t, ready, "B-DEMO-UAT-002")

	runOK(t, "add", "T-001", "--title", "GPUaaS parity", "--role", "backend", "--notes", "carry context")
	contextPacket := runCapture(t, "packet", "context", "T-001", "--goal", "prove parity", "--owner", "backend", "--acceptance", "no missing fields")
	assertContains(t, contextPacket, "# Context Packet: T-001")
	assertContains(t, contextPacket, "## Goal")
	assertContains(t, contextPacket, "## Acceptance")
	assertContains(t, contextPacket, "## Recent History")

	bugfixPacket := runCapture(t, "packet", "bugfix", "T-001", "--bug-summary", "broken parity", "--root-cause", "missing fixture", "--owning-layer", "queue", "--proof-command", "go test ./...", "--regression-coverage", "fixture", "--residual-risk", "none")
	assertContains(t, bugfixPacket, "## Root Cause")
	assertContains(t, bugfixPacket, "## Owning Layer")
	assertContains(t, bugfixPacket, "## Proof Command")
	assertContains(t, bugfixPacket, "## Regression Coverage")
	assertContains(t, bugfixPacket, "## Residual Risk")

	watcherPacket := runCapture(t, "packet", "watcher", "W-001", "--owner", "C-ops/watch", "--process", "ci", "--command", "gh run watch", "--success", "green", "--failure", "red")
	assertContains(t, watcherPacket, "# Watcher Packet: W-001")
	assertContains(t, watcherPacket, "## Success")
	assertContains(t, watcherPacket, "## Failure")

	runOK(t, "regression-pack", "validate", filepath.Join(sourceRoot, "examples", "gpuaas-regression-packs.yaml"))
	runOK(t, "parity", "artifact", "--catalog", filepath.Join(sourceRoot, "examples", "gpuaas-regression-packs.yaml"))
	runOK(t, "--json", "parity", "artifact", "--catalog", filepath.Join(sourceRoot, "examples", "gpuaas-regression-packs.yaml"), "--route", "doc/api/openapi.draft.yaml")
	adoptionArtifact := runCapture(t, "adoption", "artifact", "--catalog", filepath.Join(sourceRoot, "examples", "gpuaas-regression-packs.yaml"), "--gap-limit", "1")
	assertContains(t, adoptionArtifact, "# Fairway Adoption Artifact")
	assertContains(t, adoptionArtifact, "## Gates")
}

func TestCLI_ProjectRegistry(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	t.Setenv("FAIRWAY_REGISTRY", filepath.Join(t.TempDir(), "registry.toml"))

	runOK(t, "init")
	runOK(t, "register")
	runOK(t, "projects")
	runOK(t, "--json", "projects")
	runOK(t, "unregister")
}

func TestCLI_RemainingParity(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "prune-stale")
	runOK(t, "--json", "prune-stale")
	runOK(t, "db", "migrate", "--dry-run")
	runOK(t, "db", "migrate")
	runOK(t, "db", "compat", "--backend", "postgres")
	runOK(t, "db", "compat", "--backend", "postgres", "--print-ddl")
	runOK(t, "tui", "--once")
}

func TestCLI_TrackerLinks(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "Tracked", "--role", "backend")
	runOK(t, "tracker", "providers")
	runOK(t, "--json", "tracker", "providers")
	runOK(t, "tracker", "configure", "plane", "--url", "http://localhost:8088", "--workspace", "fairway-eval", "--project", "FWPLANE", "--dry-run")
	runOK(t, "--json", "tracker", "import", "plane", "--query", "label:CI-FIX", "--parent", "T-001", "--dry-run")
	runOK(t, "tracker", "link", "T-001", "--provider", "linear", "--external-id", "LIN-1", "--url", "https://linear.app/example/issue/LIN-1")
	runOK(t, "tracker", "link", "T-001", "--provider", "plane", "--external-id", "FWPLANE-1", "--url", "http://localhost:8088/fairway-eval/FWPLANE-1")
	runOK(t, "tracker", "links")
	runOK(t, "tracker", "export-status", "T-001", "--provider", "plane", "--external-id", "FWPLANE-1", "--dry-run")
	runOK(t, "--json", "tracker", "resolve", "--provider", "plane", "--external-id", "FWPLANE-1", "--url", "http://localhost:8088/fairway-eval/FWPLANE-1")
	runOK(t, "--json", "tracker", "reconcile", "--dry-run")
	runCaptureAllowError(t, "tracker", "link", "T-001", "--provider", "notion", "--external-id", "N-1")
}

func TestDefaultAdoptionRouteSamplesUsesProfiles(t *testing.T) {
	cfg := config.Defaults("/tmp/repo")
	cfg.WorkstreamProfiles = []config.WorkstreamProfile{
		{
			Name:         "platform-foundation",
			RouteSamples: []string{"doc/api/openapi.yaml", "cmd/api/routes.go"},
		},
		{
			Name:         "release-readiness",
			RouteSamples: []string{"cmd/api/routes.go", "scripts/release/check.sh"},
		},
	}
	got := defaultAdoptionRouteSamples(cfg)
	want := []string{"doc/api/openapi.yaml", "cmd/api/routes.go", "scripts/release/check.sh"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("route samples = %#v, want %#v", got, want)
	}
}

func TestCLI_AdoptionArtifactEvaluatesProfileGates(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	runOK(t, "init")
	appendFile(t, ".fairway/config.toml", `
[[workstream_profiles]]
name = "release-readiness"
task_kinds = ["release-risk"]

[[workstream_profiles.gates]]
name = "security-review"
mode = "advisory"
evidence_type = "security-review"
required_evidence_count = 1
accepted_results = ["pass"]
artifact_required = true
`)
	runOK(t, "add", "T-001", "--title", "Covered", "--role", "backend", "--kind", "release-risk")
	runOK(t, "add", "T-002", "--title", "Missing", "--role", "backend", "--kind", "release-risk")
	runOK(t, "record", "evidence", "T-001", "--command-text", "security review", "--result", "pass", "--artifact", "dist/security.txt", "--artifact-type", "security-review")

	artifact := runCapture(t, "adoption", "artifact", "--gap-limit", "5")
	assertContains(t, artifact, "## Gate Evaluation")
	assertContains(t, artifact, "release-readiness/security-review: missing (1/2 satisfied)")
	assertContains(t, artifact, "T-002 [release-risk] Missing")

	jsonArtifact := runCapture(t, "--json", "adoption", "artifact")
	assertContains(t, jsonArtifact, `"gate_evaluations"`)
	assertContains(t, jsonArtifact, `"missing_count": 1`)
}

func TestEvaluateGateForTaskCountsOnlyRowsMeetingAllRequirements(t *testing.T) {
	now := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	gate := config.WorkstreamProfileGate{
		Name:                  "security-review",
		Mode:                  "advisory",
		EvidenceType:          "security-review",
		RequiredEvidenceCount: 2,
		AcceptedResults:       []string{"pass"},
		ArtifactRequired:      true,
		OwnerSignoffRequired:  true,
		ExpiresAfter:          "24h",
	}
	evidence := []store.Evidence{
		{Result: "pass", ArtifactType: "security-review", ArtifactPath: "dist/a.txt", Notes: "owner signoff", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)},
		{Result: "pass", ArtifactType: "security-review", ArtifactPath: "", Notes: "owner signoff", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)},
		{Result: "pass", ArtifactType: "security-review", ArtifactPath: "dist/stale.txt", Notes: "owner signoff", CreatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339Nano)},
		{Result: "pass", ArtifactType: "security-review", ArtifactPath: "dist/no-signoff.txt", Notes: "reviewed", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)},
	}
	ok, matching, reasons := evaluateGateForTask(gate, evidence, now)
	if ok {
		t.Fatal("expected gate to be missing")
	}
	if matching != 1 {
		t.Fatalf("matching=%d, want 1", matching)
	}
	if len(reasons) == 0 {
		t.Fatal("expected missing reasons")
	}
}

func TestDashboardLifecycleFilesDefaultAndMulti(t *testing.T) {
	root := t.TempDir()
	pidFile, logFile := dashboardLifecycleFiles(root, "", "", false)
	if pidFile != filepath.Join(root, ".fairway", "dashboard.pid") {
		t.Fatalf("pid file = %s", pidFile)
	}
	if logFile != filepath.Join(root, ".fairway", "dashboard.log") {
		t.Fatalf("log file = %s", logFile)
	}
	pidFile, logFile = dashboardLifecycleFiles(root, "", "", true)
	if pidFile != filepath.Join(root, ".fairway", "dashboard-multi.pid") {
		t.Fatalf("multi pid file = %s", pidFile)
	}
	if logFile != filepath.Join(root, ".fairway", "dashboard-multi.log") {
		t.Fatalf("multi log file = %s", logFile)
	}
	pidFile, logFile = dashboardLifecycleFiles(root, "custom.pid", "custom.log", false)
	if pidFile != "custom.pid" || logFile != "custom.log" {
		t.Fatalf("custom files = %s %s", pidFile, logFile)
	}
}

func TestDashboardLifecycleStatusRemovesStalePIDFile(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "dashboard.pid")
	logFile := filepath.Join(dir, "dashboard.log")
	writeFile(t, pidFile, "999999\n")

	status, err := readDashboardLifecycleStatus(pidFile, logFile, "127.0.0.1:7878")
	if err != nil {
		t.Fatal(err)
	}
	if status.Running {
		t.Fatalf("status running = true, want false")
	}
	if status.PID != 0 {
		t.Fatalf("pid = %d, want 0", status.PID)
	}
	if _, err := os.Stat(pidFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pid file still exists or unexpected error: %v", err)
	}
}

func TestDashboardLifecycleChildArgsPreservesGlobalOptions(t *testing.T) {
	args := dashboardLifecycleChildArgs(globalOptions{
		ConfigPath: "/tmp/fairway.toml",
		DBPath:     "/tmp/fairway.db",
		Role:       "backend",
	}, "127.0.0.1:7878", true, true)
	want := []string{
		"--config", "/tmp/fairway.toml",
		"--db", "/tmp/fairway.db",
		"--as", "backend",
		"dashboard", "--listen", "127.0.0.1:7878", "--no-open", "--multi",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func runOK(t *testing.T, args ...string) {
	t.Helper()
	_ = runCapture(t, args...)
}

func runCapture(t *testing.T, args ...string) string {
	t.Helper()
	out, err := captureRun(args...)
	if err != nil {
		t.Fatalf("fairway %v: %v", args, err)
	}
	return out
}

func runCaptureAllowError(t *testing.T, args ...string) string {
	t.Helper()
	out, err := captureRun(args...)
	if err == nil {
		t.Fatalf("fairway %v succeeded, expected error", args)
	}
	return out
}

func captureRun(args ...string) (string, error) {
	stdout := os.Stdout
	stderr := os.Stderr
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
	}()
	var out bytes.Buffer
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	runErr := run(context.Background(), args)
	_ = w.Close()
	_, _ = out.ReadFrom(r)
	return out.String(), runErr
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q; got:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("expected output not to contain %q; got:\n%s", want, got)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init", "-b", "main")
	git(t, dir, "config", "user.name", "Fairway Test")
	git(t, dir, "config", "user.email", "fairway@example.test")
}

func gitAddCommit(t *testing.T, dir, message string) {
	t.Helper()
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", message)
}

func gitRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v\n%s", ref, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	clean := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clean, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Clean(path), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func replaceInFile(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(data), old, new)
	if text == string(data) {
		t.Fatalf("%q not found in %s", old, path)
	}
	if err := os.WriteFile(filepath.Clean(path), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
