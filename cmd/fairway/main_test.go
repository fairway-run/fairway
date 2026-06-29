package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
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
	runOK(t, "record", "handoff", "T-001", "--to", "ui", "--payload", "please review")
	runOK(t, "record", "notification", "T-001", "--domain", "ui", "--provider", "codex", "--target", "thread-1", "--state", "sent")
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
	runOK(t, "completion-handback-report")
	runOK(t, "--json", "completion-handback-report")
	runOK(t, "usage", "cost-report")
	runOK(t, "--json", "usage", "cost-report")
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
		assertContains(t, out, "fairway - Governed Agentic Engineering coordination")
		assertContains(t, out, "Queue and task state:")
		assertContains(t, out, "Evidence and review:")
		assertContains(t, out, "Sessions, worktrees, and workflow:")
		assertContains(t, out, "Coordinator and readiness:")
		assertContains(t, out, "Rules, packets, reports, and audits:")
		assertContains(t, out, "Dashboard, release, and configuration:")
		assertContains(t, out, "fairway agent-guide")
	}
}

func TestCLI_CommandHelpCleanExit(t *testing.T) {
	out := runCapture(t, "import", "--help")
	assertContains(t, out, "fairway import <yaml-or-json-path> [--state-once]")
	assertContains(t, out, "Import task definitions")
	assertNotContains(t, out, "error:")

	out = runCapture(t, "agent-guide", "--help")
	assertContains(t, out, "fairway agent-guide [--path | --output <path>]")
}

func TestCLI_TopLevelCommandHelpCleanExit(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    string
	}{
		{"preflight", "fairway preflight [--role <role>]"},
		{"git-check", "fairway git-check [--base <ref>]"},
		{"status-report", "fairway status-report"},
		{"health-report", "fairway health-report"},
		{"timing-report", "fairway timing-report"},
		{"completion-handback-report", "fairway completion-handback-report"},
		{"dispatch-plan", "fairway dispatch-plan [--role <role>]"},
		{"register", "fairway register [--name <name>]"},
		{"unregister", "fairway unregister [<name>]"},
		{"projects", "fairway projects"},
		{"tui", "fairway tui [--once]"},
	} {
		out := runCapture(t, tc.command, "--help")
		assertContains(t, out, tc.want)
		assertNotContains(t, out, "error:")
		assertNotContains(t, out, "Usage of")
	}
}

func TestCLI_UnknownCommandStillErrors(t *testing.T) {
	out, err := captureRun("does-not-exist")
	if err == nil {
		t.Fatal("unknown command succeeded, expected error")
	}
	assertNotContains(t, out, "Queue and task state:")
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
		{[]string{"record", "--help"}, "fairway record evidence|guard-report|handoff|completion-handback|completion-handback-supersede|notification|review|usage|push-intent"},
		{[]string{"record", "completion-handback", "--help"}, "fairway record completion-handback <task-id> --to <role> --next-action <text>"},
		{[]string{"record", "completion-handback-supersede", "--help"}, "fairway record completion-handback-supersede <task-id> --handoff-id <id> --reason <text>"},
		{[]string{"review-waits", "--help"}, "fairway review-waits list|wake [--task <task-id>]"},
		{[]string{"review-waits", "list", "--help"}, "fairway review-waits list [--blocking] [--task <task-id>] [--stale]"},
		{[]string{"review-waits", "wake", "--help"}, "fairway review-waits wake [--task <task-id>]"},
		{[]string{"review-policy", "report", "--help"}, "fairway review-policy report [--profile <name>]"},
		{[]string{"coordinator", "tick", "--help"}, "fairway coordinator tick [--completion-handback-wake]"},
		{[]string{"live-window", "--help"}, "fairway live-window record <task-id> --phase <phase>"},
		{[]string{"live-window", "record", "--help"}, "fairway live-window record <task-id> --phase <phase>"},
		{[]string{"live-window", "status", "--help"}, "fairway live-window status [--task <task-id>]"},
		{[]string{"live-window", "control-room", "--help"}, "fairway live-window control-room [--task <task-id>] [--stale]"},
		{[]string{"memory", "--help"}, "fairway memory show|update|append|packet|stale"},
		{[]string{"memory", "show", "--help"}, "fairway memory show [--track <track-id>]"},
		{[]string{"memory", "packet", "--help"}, "fairway memory packet --track <track-id>"},
		{[]string{"wait", "--help"}, "fairway wait add|ack|list|tick|wake"},
		{[]string{"wait", "add", "--help"}, "fairway wait add --task <task-id> --track <track-id> --on <condition>"},
		{[]string{"wait", "ack", "--help"}, "fairway wait ack <wait-id>"},
		{[]string{"wait", "list", "--help"}, "fairway wait list [--task <task-id>]"},
		{[]string{"wait", "tick", "--help"}, "fairway wait tick [--task <task-id>]"},
		{[]string{"wait", "wake", "--help"}, "fairway wait wake [--task <task-id>]"},
		{[]string{"packet", "--help"}, "fairway packet context|bugfix|retry|watcher"},
		{[]string{"packet", "retry", "--help"}, "fairway packet retry <task-id> --kind <preflight|live-operation>"},
		{[]string{"audit", "--help"}, "fairway audit work-coverage|ci-learning|failure-routing|notifications|docs-backlog"},
		{[]string{"advisory", "--help"}, "fairway advisory adapters|validate <task-id>"},
		{[]string{"advisory", "adapters", "--help"}, "fairway advisory adapters [--include-disabled]"},
		{[]string{"advisory", "validate", "--help"}, "fairway advisory validate <task-id> --action <action>"},
		{[]string{"notify", "--help"}, "fairway notify notifiers|dry-run|send"},
		{[]string{"notify", "notifiers", "--help"}, "fairway notify notifiers [--include-disabled]"},
		{[]string{"notify", "dry-run", "--help"}, "fairway notify dry-run --notifier <name> --task <task-id> --domain <domain>"},
		{[]string{"notify", "send", "--help"}, "fairway notify send --notifier <name> --task <task-id> --domain <domain>"},
		{[]string{"automation", "--help"}, "fairway automation candidates --since <duration> [--threshold <n>] [--format text|json]"},
		{[]string{"automation", "candidates", "--help"}, "fairway automation candidates --since <duration> [--threshold <n>] [--format text|json]"},
		{[]string{"delivery", "--help"}, "fairway delivery report --since <duration> [--profile <name>] [--format text|json]"},
		{[]string{"delivery", "report", "--help"}, "fairway delivery report --since <duration> [--profile <name>] [--format text|json]"},
		{[]string{"provenance", "--help"}, "fairway provenance report [--task <task-id>|--since <duration>] [--format text|markdown|json]"},
		{[]string{"provenance", "report", "--help"}, "fairway provenance report [--task <task-id>|--since <duration>] [--format text|markdown|json]"},
		{[]string{"provenance", "prompt-packet", "--help"}, "fairway provenance prompt-packet --task <task-id> [--format markdown|json]"},
		{[]string{"provenance", "manifest", "--help"}, "fairway provenance manifest --path <file>... [--format text|json]"},
		{[]string{"usage", "--help"}, "fairway usage report|cost-report"},
		{[]string{"usage", "report", "--help"}, "fairway usage report [--by <provider|task|epic|role|day|kind|phase|model>]"},
		{[]string{"usage", "cost-report", "--help"}, "fairway usage cost-report [--by <provider|task|epic|role|day|kind|phase|model>]"},
		{[]string{"audit", "ci-learning", "--help"}, "fairway audit ci-learning [--task-id <task-id>] [--template]"},
		{[]string{"audit", "failure-routing", "--help"}, "fairway audit failure-routing [--task-id <task-id>] [--template]"},
		{[]string{"audit", "notifications", "--help"}, "fairway audit notifications [--task <task-id>] [--all]"},
		{[]string{"audit", "docs-backlog", "--help"}, "fairway audit docs-backlog [--doc <path>]..."},
		{[]string{"dashboard", "--help"}, "fairway dashboard [--listen <addr>]"},
		{[]string{"db", "--help"}, "fairway db backup|export|migrate|compat"},
		{[]string{"workflow", "--help"}, "fairway workflow check|closeout"},
		{[]string{"batch", "--help"}, "fairway batch create|add|remove|evidence|link|show|list"},
		{[]string{"batch", "create", "--help"}, "fairway batch create <batch-id> --title <title>"},
		{[]string{"batch", "add", "--help"}, "fairway batch add <batch-id> <task-id>"},
		{[]string{"batch", "evidence", "--help"}, "fairway batch evidence <batch-id> --command-text <cmd> --result"},
		{[]string{"batch", "link", "--help"}, "fairway batch link <batch-id>"},
		{[]string{"batch", "show", "--help"}, "fairway batch show <batch-id>"},
		{[]string{"batch", "list", "--help"}, "fairway batch list"},
		{[]string{"audit", "--help"}, "fairway audit work-coverage|ci-learning|failure-routing|notifications|docs-backlog"},
		{[]string{"release", "--help"}, "fairway release verify"},
		{[]string{"rules", "--help"}, "fairway rules validate <dir>|evidence-types|match <task-id>"},
	} {
		out := runCapture(t, tc.args...)
		assertContains(t, out, tc.want)
	}
}

func TestCLI_InitWritesAgentBreadcrumb(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	out := runCapture(t, "init")
	assertContains(t, out, "initialized fairway")
	assertContains(t, out, "wrote .fairway/AGENTS.md")
	assertContains(t, out, "Root AGENTS.md / CLAUDE.md bootstrap block")
	assertContains(t, out, "Read `.fairway/AGENTS.md` before changing code.")
	assertContains(t, out, "github.com/fairway-run/fairway")

	body, err := os.ReadFile(".fairway/AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	contract := string(body)
	assertContains(t, contract, "Execution Source Of Truth")
	assertContains(t, contract, "Start Of Session Ritual")
	assertContains(t, contract, "fairway config validate")
	assertContains(t, contract, "fairway session upsert")
	assertContains(t, contract, "fairway agent-guide")
	assertContains(t, contract, "Role Resolution Order")
	assertContains(t, contract, "Session Registration Expectation")
	assertContains(t, contract, "Full Guide")
}

func TestCLI_InitPreservesEditedAgentBreadcrumb(t *testing.T) {
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
	custom := "# Custom Agent Notes\n\nDo not overwrite this file.\n"
	writeFile(t, ".fairway/AGENTS.md", custom)
	out := runCapture(t, "init")
	assertContains(t, out, ".fairway/config.toml already exists")
	assertContains(t, out, ".fairway/AGENTS.md already exists")
	assertContains(t, out, "--refresh-agent-contract")
	body, err := os.ReadFile(".fairway/AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != custom {
		t.Fatalf("agent breadcrumb was clobbered:\n%s", body)
	}

	out = runCapture(t, "init", "--refresh-agent-contract")
	assertContains(t, out, "refreshed .fairway/AGENTS.md")
	body, err = os.ReadFile(".fairway/AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, string(body), "Fairway Agent Contract")
	assertNotContains(t, string(body), "Do not overwrite this file.")
}

func TestCLI_DashboardStatusIncludesVersionReadback(t *testing.T) {
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

	text := runCapture(t, "dashboard", "status")
	assertContains(t, text, "version="+version)
	assertContains(t, text, "binary=")

	out := runCapture(t, "--json", "dashboard", "status")
	var status struct {
		Running    bool   `json:"running"`
		URL        string `json:"url"`
		Version    string `json:"version"`
		BinaryPath string `json:"binary_path"`
	}
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatalf("dashboard status json: %v\n%s", err, out)
	}
	if status.Running {
		t.Fatal("expected stopped dashboard in fresh test repo")
	}
	if status.Version != version {
		t.Fatalf("version=%q, want %q", status.Version, version)
	}
	if status.BinaryPath == "" {
		t.Fatal("expected binary path readback")
	}
	if status.URL == "" {
		t.Fatal("expected dashboard URL readback")
	}
}

func TestCLI_AgentGuideOutputsEmbeddedGuide(t *testing.T) {
	out := runCapture(t, "agent-guide")
	assertContains(t, out, "# Agent Guide")
	assertContains(t, out, "Fairway DB is the execution source of truth")
	assertContains(t, out, "Start Of Session")
}

func TestCLI_AgentGuidePathAndOutputOptions(t *testing.T) {
	out := runCapture(t, "agent-guide", "--path")
	assertContains(t, out, "docs/agent-guide.md")
	assertContains(t, out, "version=")
	assertContains(t, out, "github.com/fairway-run/fairway")

	repo := t.TempDir()
	guidePath := filepath.Join(repo, "agent-guide.md")
	out = runCapture(t, "agent-guide", "--output", guidePath)
	assertContains(t, out, "wrote "+guidePath)
	body, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, string(body), "# Agent Guide")
}

func TestCLI_AgentGuideRejectsInvalidOptions(t *testing.T) {
	if _, err := captureRun("agent-guide", "--path", "--output", filepath.Join(t.TempDir(), "guide.md")); err == nil {
		t.Fatal("agent-guide accepted --path with --output, expected error")
	}
	if _, err := captureRun("agent-guide", "--bogus"); err == nil {
		t.Fatal("agent-guide accepted --bogus, expected error")
	}
}

func TestCLI_RulesValidateAndEvidenceTypes(t *testing.T) {
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
	writeFile(t, "rules-platform/schemas/rule.schema.yaml", `type: object
required:
  - id
  - title
  - status
`)
	writeFile(t, "rules-platform/rules/core/contract.md", `---
id: platform.contract-first
title: Contract first
status: draft
risk_floor: medium
required_evidence:
  - generated-artifacts-clean
review_domains:
  - backend
---

body
`)
	appendFile(t, ".fairway/config.toml", `
[[rule_sources]]
name = "platform"
source = "path:rules-platform"
mode = "advisory"

[[workstream_profiles]]
name = "platform-foundation"
rule_groups = ["platform.core"]
review_domains = ["backend"]

[[workstream_profiles.gates]]
name = "codegen"
mode = "advisory"
evidence_type = "generated-artifacts-clean"
accepted_results = ["pass"]
`)
	out := runCapture(t, "rules", "validate", "rules-platform")
	assertContains(t, out, "rule pack rules-platform: rules=1 groups=1 findings=0")
	assertContains(t, out, "group: rules-platform.core")

	runOK(t, "add", "T-001", "--title", "Rules evidence", "--role", "backend", "--profile", "platform-foundation", "--source-paths", "doc/api/openapi.yaml", "--tag", "surface:api", "--risk-level", "medium")
	runOK(t, "record", "evidence", "T-001", "--command-text", "make codegen", "--result", "pass", "--artifact-type", "generated-artifacts-clean")
	out = runCapture(t, "rules", "evidence-types")
	assertContains(t, out, "rule pack platform: rules=1 groups=1 findings=0")
	assertContains(t, out, "- generated-artifacts-clean")
	assertContains(t, out, "config_gate")
	assertContains(t, out, "recorded")
	out = runCapture(t, "rules", "match", "T-001")
	assertContains(t, out, "platform.contract-first [selected]")
	assertContains(t, out, "group=platform.core")
}

func TestCLI_ListStatusFiltersAndDependencyVisibility(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Complete dependency", "--role", "backend")
	runOK(t, "add", "T-002", "--title", "Blocked dependency", "--role", "backend")
	runOK(t, "add", "T-003", "--title", "Ready todo", "--role", "backend", "--dependencies", "T-001")
	runOK(t, "add", "T-004", "--title", "Blocked todo", "--role", "backend", "--dependencies", "T-002")
	runOK(t, "set-status", "T-001", "done")
	runOK(t, "set-status", "T-002", "in_progress")

	todo := runCapture(t, "list", "--status", "todo")
	assertContains(t, todo, "T-003")
	assertContains(t, todo, "ready")
	assertContains(t, todo, "deps=1 blocked=0 missing=0")
	assertContains(t, todo, "T-004")
	assertContains(t, todo, "not_ready")
	assertContains(t, todo, "deps=1 blocked=1 missing=0")
	if strings.Contains(todo, "T-001") {
		t.Fatalf("done task appeared in todo list:\n%s", todo)
	}

	combined := runCapture(t, "list", "--status", "done,in_progress")
	assertContains(t, combined, "T-001")
	assertContains(t, combined, "done")
	assertContains(t, combined, "T-002")
	assertContains(t, combined, "in_progress")
	if strings.Contains(combined, "T-003") {
		t.Fatalf("todo task appeared in combined done/in_progress list:\n%s", combined)
	}

	ready := runCapture(t, "list", "--status", "todo", "--ready")
	assertContains(t, ready, "T-003")
	if strings.Contains(ready, "T-004") {
		t.Fatalf("not-ready tasks appeared in ready-only list:\n%s", ready)
	}

	none := runCapture(t, "list", "--status", "review")
	assertContains(t, none, "no tasks matched filters")

	jsonOut := runCapture(t, "--json", "list", "--status", "todo")
	for _, want := range []string{
		`"id": "T-003"`,
		`"ready": true`,
		`"dependency_summary": "deps=1 blocked=0 missing=0"`,
		`"id": "T-004"`,
		`"blocked_dependencies": [`,
		`"T-002:in_progress"`,
	} {
		assertContains(t, jsonOut, want)
	}
}

func TestTaskListRowsReportsMissingDependencies(t *testing.T) {
	rows := taskListRows([]store.Task{
		{
			Definition: store.TaskDefinition{
				ID:           "T-001",
				Title:        "Missing dependency",
				Role:         "backend",
				Kind:         "feature",
				Dependencies: []string{"T-999"},
			},
			Status: "todo",
		},
	}, []string{"done"})
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
	row := rows[0]
	if row.Ready {
		t.Fatalf("row is ready with missing dependency: %+v", row)
	}
	if row.DependencySummary != "deps=1 blocked=0 missing=1" {
		t.Fatalf("dependency summary=%q", row.DependencySummary)
	}
	if !reflect.DeepEqual([]string{"T-999"}, row.MissingDependencies) {
		t.Fatalf("missing dependencies=%v", row.MissingDependencies)
	}
}

func TestCLI_ReadyExplainsEmptyQueue(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Dependency in progress", "--role", "backend")
	runOK(t, "add", "T-002", "--title", "Blocked todo", "--role", "backend", "--dependencies", "T-001")
	runOK(t, "set-status", "T-001", "in_progress")

	out := runCapture(t, "ready")
	assertContains(t, out, "no ready tasks; non-ready todo tasks: 1")
	assertContains(t, out, "dependency-blocked: count=1 tasks=T-002 blocker_tasks=T-001")
	assertContains(t, out, `next="fairway task-detail T-001"`)

	jsonOut := runCapture(t, "--json", "ready")
	for _, want := range []string{
		`"claimable_count": 0`,
		`"non_ready_todo_count": 1`,
		`"category": "dependency-blocked"`,
		`"task_ids": [`,
		`"T-002"`,
		`"blocker_task_ids": [`,
		`"T-001"`,
	} {
		assertContains(t, jsonOut, want)
	}
}

func TestCLI_WorkflowCloseoutCleanTask(t *testing.T) {
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
	commit := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "--short", "HEAD"))

	runOK(t, "add", "T-001", "--title", "Closeout", "--role", "backend")
	runOK(t, "claim", "T-001")
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass")
	runOK(t, "set-status", "T-001", "done")
	out := runCapture(t, "workflow", "closeout", "T-001", "--dry-run")
	assertContains(t, out, "lane_closeout: true")
	assertContains(t, out, "commit: "+commit)
	assertContains(t, out, "safe_merged_branch")
	runOK(t, "workflow", "check", "--mode", "close", "--task-id", "T-001")
}

func TestCLI_CleanGatesAllowConfiguredAndEvidenceArtifacts(t *testing.T) {
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
	configBytes, err := os.ReadFile(".fairway/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	configText := strings.Replace(string(configBytes), "task_id_pattern = \"^[A-Z]+-[0-9]+$\"\n", "task_id_pattern = \"^[A-Z]+-[0-9]+$\"\nlocal_artifact_paths = [\"local-artifacts/fairway\"]\n", 1)
	if err := os.WriteFile(".fairway/config.toml", []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, ".gitignore", "scratch/\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")
	commit := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "--short", "HEAD"))

	runOK(t, "add", "T-001", "--title", "Artifact clean", "--role", "backend")
	runOK(t, "claim", "T-001")
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass", "--artifact", "evidence/local.txt")
	runOK(t, "set-status", "T-001", "done", "--commit", commit)
	writeFile(t, "local-artifacts/fairway/report.md", "local artifact\n")
	writeFile(t, "evidence/local.txt", "recorded evidence artifact\n")
	writeFile(t, "scratch/ignored.txt", "ignored scratch\n")

	mergeReady := runCapture(t, "merge-ready", "T-001")
	assertContains(t, mergeReady, "merge_ready: true")
	assertContains(t, mergeReady, "allowed_local_artifacts:")
	assertContains(t, mergeReady, "local-artifacts/fairway/report.md")
	assertContains(t, mergeReady, "evidence/local.txt")
	assertNotContains(t, mergeReady, "worktree has uncommitted changes")

	closeout := runCapture(t, "workflow", "closeout", "T-001", "--dry-run")
	assertContains(t, closeout, "lane_closeout: true")
	assertContains(t, closeout, "allowed_local_artifact")
	assertContains(t, closeout, "path=local-artifacts/fairway/report.md")
	assertContains(t, closeout, "path=evidence/local.txt")

	writeFile(t, "cmd/dirty.go", "package dirty\n")
	dirty := runCaptureAllowError(t, "merge-ready", "T-001")
	assertContains(t, dirty, "merge_ready: false")
	assertContains(t, dirty, "dirty_paths:")
	assertContains(t, dirty, "cmd/dirty.go")
	assertContains(t, dirty, "worktree has uncommitted changes")
}

func TestCLI_WorkflowCloseoutApplyDeletesVerifiedMergedRemoteBranch(t *testing.T) {
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
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")
	git(t, repo, "push", "-u", "origin", "main")
	git(t, repo, "checkout", "-b", "agent/backend")
	writeFile(t, "feature.txt", "feature\n")
	git(t, repo, "add", "feature.txt")
	git(t, repo, "commit", "-m", "T-001 feature")
	commit := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	git(t, repo, "push", "-u", "origin", "agent/backend")
	git(t, repo, "checkout", "main")
	git(t, repo, "merge", "--no-ff", "agent/backend", "-m", "merge T-001")
	git(t, repo, "push", "origin", "main")

	writeFile(t, "tasks.yaml", `- id: T-001
  title: Closeout remote
  role: backend
  status: done
  branch: agent/backend
  commit_sha: `+commit+`
`)
	runOK(t, "import", "tasks.yaml", "--state-once")
	if err := os.Remove("tasks.yaml"); err != nil {
		t.Fatal(err)
	}
	runOK(t, "record", "evidence", "T-001", "--command-text", "gh run view --json conclusion", "--result", "pass", "--artifact-type", "ci")
	runOK(t, "record", "push-intent", "T-001", "--intent", "main-validation", "--branch", "agent/backend", "--remote", "origin")
	out := runCapture(t, "workflow", "closeout", "T-001", "--dry-run")
	assertContains(t, out, "remote_push_intent")
	assertContains(t, out, "safe_merged_remote_branch")
	runOK(t, "workflow", "closeout", "T-001", "--apply")
	if err := exec.Command("git", "-C", remote, "show-ref", "--verify", "--quiet", "refs/heads/agent/backend").Run(); err == nil {
		t.Fatal("remote branch still exists after closeout apply")
	}
}

func TestCLI_RecordPushIntent(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Push intent", "--role", "backend")
	out := runCapture(t, "record", "push-intent", "T-001", "--intent", "review", "--branch", "review/T-001", "--remote", "origin")
	assertContains(t, out, "push intent recorded T-001 intent=review branch=review/T-001 remote=origin")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "push-intent")
	assertContains(t, detail, "intent=review branch=review/T-001 remote=origin")
	if err := run(context.Background(), []string{"record", "push-intent", "T-001", "--intent", "exception", "--branch", "scratch/T-001"}); err == nil {
		t.Fatal("expected exception push intent to require reason")
	}
	runOK(t, "record", "push-intent", "T-001", "--intent", "exception", "--branch", "scratch/T-001", "--reason", "operator approved backup")
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
		"--source-paths", "packages/services",
		"--target-paths", "packages/services/platform",
		"--review-domains", "architecture,security",
		"--review-domains", "governance",
		"--tag", "production-readiness",
		"--tag", "environment:staging,docs-portal",
		"--acceptance", "map current owner",
		"--acceptance", "map target owner",
		"--risk-level", "medium",
		"--migration-type", "facade")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "metadata:")
	assertContains(t, detail, "profile: platform-foundation")
	assertContains(t, detail, "source_paths: cmd/api, doc/api, packages/services")
	assertContains(t, detail, "tags: production-readiness, environment:staging, docs-portal")
	assertContains(t, detail, "- map current owner")
	assertContains(t, detail, "- map target owner")

	jsonDetail := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, jsonDetail, `"owning_domain": "platform"`)
	assertContains(t, jsonDetail, `"review_domains": [`)
	assertContains(t, jsonDetail, `"governance"`)
	assertContains(t, jsonDetail, `"tags": [`)
	assertContains(t, jsonDetail, `"environment:staging"`)

	runOK(t, "update", "T-001", "--risk-level", "high",
		"--source-paths", "cmd/api/routes.go",
		"--source-paths", "doc/api/openapi.yaml,packages/services/platform",
		"--tag", "security-review",
		"--tag", "environment:cloudflare",
		"--acceptance", "updated acceptance one",
		"--acceptance", "updated acceptance two")
	updated := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, updated, `"risk_level": "high"`)
	assertContains(t, updated, `"cmd/api/routes.go"`)
	assertContains(t, updated, `"doc/api/openapi.yaml"`)
	assertContains(t, updated, `"packages/services/platform"`)
	assertContains(t, updated, `"security-review"`)
	assertContains(t, updated, `"environment:cloudflare"`)
	if strings.Contains(updated, `"production-readiness"`) {
		t.Fatalf("update should replace tags when --tag is provided:\n%s", updated)
	}
	assertContains(t, updated, `"updated acceptance one"`)
	assertContains(t, updated, `"updated acceptance two"`)

	if err := run(context.Background(), []string{"add", "T-002", "--title", "Bad profile", "--role", "backend", "--profile", "missing"}); err == nil {
		t.Fatal("expected unknown profile validation error")
	}
}

func TestCLI_ImportExportTaskTags(t *testing.T) {
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
  title: Tagged task
  role: backend
  tags:
    - production-readiness
    - environment:cloudflare
`)
	runOK(t, "import", "tasks.yaml")
	detail := runCapture(t, "--json", "task-detail", "T-001")
	first := strings.Index(detail, `"production-readiness"`)
	second := strings.Index(detail, `"environment:cloudflare"`)
	if first < 0 || second < 0 || first > second {
		t.Fatalf("task-detail tags not present in import order:\n%s", detail)
	}
	runOK(t, "db", "export", "snapshot.json")
	data, err := os.ReadFile("snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := string(data)
	first = strings.Index(snapshot, `"production-readiness"`)
	second = strings.Index(snapshot, `"environment:cloudflare"`)
	if first < 0 || second < 0 || first > second {
		t.Fatalf("snapshot tags not present in import order:\n%s", snapshot)
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

func TestCLI_MergeReadyAndWorkflowCheckEvaluateRuleEvidence(t *testing.T) {
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
	writeRulePack(t, "rules-blocking", "blocking.contract", "generated-artifacts-clean")
	writeRulePack(t, "rules-advisory", "advisory.contract", "generated-artifacts-clean")
	writeRulePack(t, "rules-disabled", "disabled.contract", "generated-artifacts-clean")
	appendFile(t, ".fairway/config.toml", `
[[rule_sources]]
name = "blocking"
source = "path:rules-blocking"
mode = "blocking"

[[rule_sources]]
name = "advisory"
source = "path:rules-advisory"
mode = "advisory"

[[rule_sources]]
name = "disabled"
source = "path:rules-disabled"
mode = "disabled"

[[workstream_profiles]]
name = "blocking-profile"
rule_groups = ["blocking.core", "advisory.core", "disabled.core"]
review_domains = ["backend"]

[[workstream_profiles]]
name = "advisory-profile"
rule_groups = ["advisory.core", "disabled.core"]
review_domains = ["backend"]
`)
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")

	runOK(t, "add", "T-001", "--title", "Blocking rule", "--role", "backend", "--profile", "blocking-profile", "--source-paths", "doc/api/openapi.yaml", "--tag", "surface:api", "--risk-level", "medium")
	failed := runCaptureAllowError(t, "merge-ready", "T-001")
	assertContains(t, failed, "rule evidence missing task=T-001 rule=blocking.contract mode=blocking evidence=generated-artifacts-clean")
	assertContains(t, failed, "rule evidence missing task=T-001 rule=advisory.contract mode=advisory evidence=generated-artifacts-clean")
	if strings.Contains(failed, "disabled.contract") {
		t.Fatalf("disabled rule source produced merge-ready evidence finding:\n%s", failed)
	}

	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass", "--artifact-type", "local-test")
	runOK(t, "set-status", "T-001", "done")
	closeReport := runCaptureAllowError(t, "workflow", "check", "--mode", "close", "--task-id", "T-001")
	assertContains(t, closeReport, "rule evidence missing task=T-001 rule=blocking.contract mode=blocking evidence=generated-artifacts-clean")

	runOK(t, "record", "evidence", "T-001", "--command-text", "make codegen", "--result", "pass", "--artifact-type", "generated-artifacts-clean")
	runOK(t, "merge-ready", "T-001")
	jsonReport := runCapture(t, "--json", "merge-ready", "T-001")
	assertContains(t, jsonReport, `"rule_evaluations"`)
	assertContains(t, jsonReport, `"status": "satisfied"`)

	runOK(t, "add", "T-002", "--title", "Advisory rule", "--role", "backend", "--profile", "advisory-profile", "--source-paths", "doc/api/openapi.yaml", "--tag", "surface:api", "--risk-level", "medium")
	advisory := runCapture(t, "merge-ready", "T-002")
	assertContains(t, advisory, "warnings:")
	assertContains(t, advisory, "rule evidence missing task=T-002 rule=advisory.contract mode=advisory evidence=generated-artifacts-clean")
}

func TestCLI_RulesEvidenceTypesReportsMissingAdvisorySource(t *testing.T) {
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
	writeRulePack(t, "rules-one", "one.contract", "one-evidence")
	writeRulePack(t, "rules-two", "two.contract", "two-evidence")
	appendFile(t, ".fairway/config.toml", `
[[rule_sources]]
name = "one"
source = "path:rules-one"
mode = "advisory"

[[rule_sources]]
name = "missing"
source = "path:missing-rules"
mode = "advisory"

[[rule_sources]]
name = "two"
source = "path:rules-two"
mode = "advisory"
`)

	out := runCapture(t, "rules", "evidence-types")
	assertContains(t, out, "rule pack one: rules=1")
	assertContains(t, out, "rule pack missing: rules=0 groups=0 findings=1")
	assertContains(t, out, "error:")
	assertContains(t, out, `rule source "missing" mode=advisory`)
	assertContains(t, out, "rule pack two: rules=1")

	jsonOut := runCapture(t, "--json", "rules", "evidence-types")
	for _, want := range []string{
		`"source_name":"one"`,
		`"source_name":"missing"`,
		`"mode":"advisory"`,
		`"path":`,
		`rule source \"missing\" mode=advisory`,
		`"source_name":"two"`,
		`"evidence_type":"one-evidence"`,
		`"evidence_type":"two-evidence"`,
	} {
		assertContains(t, jsonOut, want)
	}
}

func TestCLI_RulesEvidenceTypesFailsClosedForMissingBlockingSource(t *testing.T) {
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
	writeRulePack(t, "rules-valid", "valid.contract", "valid-evidence")
	appendFile(t, ".fairway/config.toml", `
[[rule_sources]]
name = "valid"
source = "path:rules-valid"
mode = "advisory"

[[rule_sources]]
name = "blocking-missing"
source = "path:missing-rules"
mode = "blocking"
`)

	if err := run(context.Background(), []string{"rules", "evidence-types"}); err == nil {
		t.Fatal("rules evidence-types succeeded, want missing blocking source error")
	} else {
		if !strings.Contains(err.Error(), `rule source "blocking-missing" mode=blocking`) || !strings.Contains(err.Error(), "missing-rules") {
			t.Fatalf("error=%v, want blocking source name/mode/path", err)
		}
	}
}

func TestCLI_RulesEvidenceTypesFailsClosedForUnreadableBlockingSource(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Join(repo, "rules-unreadable", "rules", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	appendFile(t, ".fairway/config.toml", `
[[rule_sources]]
name = "blocking-unreadable"
source = "path:rules-unreadable"
mode = "blocking"
`)

	if err := run(context.Background(), []string{"rules", "evidence-types"}); err == nil {
		t.Fatal("rules evidence-types succeeded, want unreadable blocking source error")
	} else if !strings.Contains(err.Error(), `rule source "blocking-unreadable" mode=blocking`) || !strings.Contains(err.Error(), "rule.schema.yaml") {
		t.Fatalf("error=%v, want unreadable blocking source name/mode/schema path", err)
	}
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
	detail = runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "review: partial_approval")
	if strings.Contains(detail, "review: approved") {
		t.Fatalf("task detail summarized partial review as approved:\n%s", detail)
	}
	jsonDetail = runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, jsonDetail, `"review_status": "partial_approval"`)
	runOK(t, "record", "review", "T-001", "--reviewer", "security", "--verdict", "approve", "--reason", "security ok")
	detail = runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "review: approved")
	runOK(t, "merge-ready", "T-001")

	runOK(t, "add", "T-002", "--title", "Ops-owned task", "--role", "ops", "--review-domains", "ops")
	runOK(t, "--as", "ops", "claim", "T-002")
	runCaptureAllowError(t, "record", "review", "T-002", "--reviewer", "ops", "--verdict", "approve", "--reason", "self")
	runOK(t, "record", "review", "T-002", "--reviewer", "ops-reviewer", "--domain", "ops", "--verdict", "approve", "--reason", "independent ops review")
	detail = runCapture(t, "task-detail", "T-002")
	assertContains(t, detail, "review: approved")
	assertContains(t, detail, "approve by ops-reviewer for ops")
	runOK(t, "merge-ready", "T-002")
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
	runOK(t, "add", "T-001", "--title", "Prompt launch", "--role", "backend")
	dryRun := runCapture(t, "session", "launch", "--role", "backend", "--task-id", "T-001", "--prompt-file", "prompts/FW-101.md", "--transcript", ".fairway/transcripts/FW-101.log", "--command", "cat", "--dry-run")
	assertContains(t, dryRun, "session launch dry-run")
	assertContains(t, dryRun, "command: cat")
	assertContains(t, dryRun, "prompt_file: prompts/FW-101.md")
	assertContains(t, dryRun, "transcript: .fairway/transcripts/FW-101.log")
	assertContains(t, dryRun, "task_id: T-001")
	if _, err := os.Stat(filepath.Join("..", "worktrees", filepath.Base(repo)+"-backend", "prompts", "FW-101.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run wrote prompt file or stat failed: %v", err)
	}
	launch := runCapture(t, "session", "launch", "--role", "backend", "--task-id", "T-001", "--prompt", "hello from prompt", "--transcript", ".fairway/transcripts/FW-101.log", "--command", "cat")
	assertContains(t, launch, "export FAIRWAY_SESSION_ID=backend-")
	assertContains(t, launch, "export FAIRWAY_TASK_ID=T-001")
	assertContains(t, launch, "transcript .fairway/transcripts/FW-101.log")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, ".fairway/transcripts/FW-101.log")
	checkpoints := runCapture(t, "checkpoint", "status", "--all")
	assertContains(t, checkpoints, "Started shell-backed codex session from prompt file")
	task := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, task, `"Status": "todo"`)
}

func TestCLI_ProviderUsageAccounting(t *testing.T) {
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
  title: Usage
  role: backend
  kind: feature
`)
	runOK(t, "import", "tasks.yaml")
	runOK(t, "session", "upsert", "--id", "codex-usage-1", "--role", "backend", "--provider", "codex", "--backend", "codex", "--task-id", "T-001", "--status", "running")
	runOK(t, "record", "usage", "T-001", "--provider", "codex", "--session-id", "codex-usage-1", "--external-session-id", "usage-1", "--role", "backend", "--phase", "implementation", "--source", "provider_reported", "--confidence", "exact", "--input-tokens", "120", "--cached-input-tokens", "40", "--output-tokens", "30", "--total-tokens", "150", "--model", "gpt-5-codex")
	runOK(t, "record", "usage", "T-001", "--provider", "unknown-provider", "--session-id", "unknown-1", "--role", "backend", "--source", "unknown", "--confidence", "unknown")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "codex provider_reported/exact total=150 input=120 cached=40 output=30")
	assertContains(t, detail, "unknown-provider unknown/unknown total=unknown input=unknown cached=unknown output=unknown")
	report := runCapture(t, "usage", "report", "--by", "provider")
	assertContains(t, report, "codex events=1 total=150 input=120 cached=40 output=30")
	assertContains(t, report, "unknown-provider events=1 total=unknown")
	if err := run(context.Background(), []string{"record", "usage", "T-001", "--provider", "codex", "--metadata", "prompt=do not store"}); err == nil {
		t.Fatal("expected prompt-like usage metadata key to be rejected")
	}
	if err := run(context.Background(), []string{"record", "usage", "T-001", "--provider", "codex", "--metadata", "access_token=do not store"}); err == nil {
		t.Fatal("expected token-like usage metadata key to be rejected")
	}
}

func TestCLI_ProvenanceReportAndPromptPacket(t *testing.T) {
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
	runOK(t, "add", "T-001",
		"--title", "Provenance",
		"--role", "backend",
		"--risk-level", "medium",
		"--target-paths", "cmd/fairway/main.go,internal/provenance/report.go",
		"--source-paths", "docs/design/supply-chain-provenance.md",
		"--acceptance", "export metadata-only provenance refs",
		"--acceptance", "redact token=supersecret before rendering")
	runOK(t, "add", "T-002", "--title", "No evidence yet", "--role", "governance")
	runOK(t, "add", "T-003", "--title", "Release word only", "--role", "ops")
	runOK(t, "session", "upsert", "--id", "codex-provenance-1", "--role", "backend", "--provider", "codex", "--backend", "codex", "--task-id", "T-001", "--status", "running")
	runOK(t, "record", "evidence", "T-001",
		"--command-text", "go test ./... && curl -H 'authorization: Bearer supersecret' 'https://example.invalid?token=supersecret'",
		"--result", "pass",
		"--artifact", "artifacts/provenance.md?api_key=supersecret",
		"--artifact-type", "validation",
		"--notes", "release verification without storing raw prompts")
	runOK(t, "record", "evidence", "T-001",
		"--command-text", "fairway release verify --version v0.1.2 --tag v0.1.2",
		"--result", "pass",
		"--artifact", "artifacts/release-verify-v0.1.2.md",
		"--artifact-type", "release-verify")
	runOK(t, "record", "evidence", "T-003",
		"--command-text", "echo release wording only",
		"--result", "pass",
		"--artifact-type", "validation",
		"--notes", "ordinary note mentions release but is not release verification")
	runOK(t, "record", "review", "T-001", "--domain", "backend", "--reviewer", "backend-reviewer", "--verdict", "approve", "--commit", "abc1234")
	runOK(t, "record", "usage", "T-001", "--provider", "codex", "--session-id", "codex-provenance-1", "--role", "backend", "--source", "provider_reported", "--confidence", "exact", "--total-tokens", "42")

	markdown := runCapture(t, "provenance", "report", "--task", "T-001", "--format", "markdown")
	assertContains(t, markdown, "# Fairway Provenance Report")
	assertContains(t, markdown, "### T-001")
	assertContains(t, markdown, "`evidence:T-001:1`")
	assertContains(t, markdown, "`review:T-001:1`")
	assertContains(t, markdown, "<redacted>")
	assertContains(t, markdown, "raw prompts, private transcripts, raw tool bodies, and generated-content dumps are excluded")
	assertNotContains(t, markdown, "supersecret")

	jsonOut := runCapture(t, "--json", "provenance", "report", "--task", "T-001")
	assertContains(t, jsonOut, `"schema": "fairway.provenance.v1"`)
	assertContains(t, jsonOut, `"raw_prompts_included": false`)
	assertContains(t, jsonOut, `"redaction_applied": true`)
	assertContains(t, jsonOut, `"total_tokens": 42`)
	assertContains(t, jsonOut, `"release_refs": [`)
	assertContains(t, jsonOut, `"artifacts/release-verify-v0.1.2.md"`)
	assertNotContains(t, jsonOut, "supersecret")

	falsePositive := runCapture(t, "--json", "provenance", "report", "--task", "T-003")
	assertContains(t, falsePositive, `"id": "T-003"`)
	assertNotContains(t, falsePositive, `"release_refs"`)

	rangeJSON := runCapture(t, "--json", "provenance", "report", "--since", "720h")
	assertContains(t, rangeJSON, `"id": "T-001"`)
	assertContains(t, rangeJSON, `"id": "T-002"`)
	assertContains(t, rangeJSON, "task=T-002 has no evidence refs")

	packet := runCapture(t, "provenance", "prompt-packet", "--task", "T-001")
	assertContains(t, packet, "# Fairway Prompt Packet: T-001")
	assertContains(t, packet, "## Forbidden Actions")
	assertContains(t, packet, "do not include raw prompt bodies")
	assertContains(t, packet, "`evidence:T-001:1`")
	assertNotContains(t, packet, "supersecret")
}

func TestCLI_ProvenanceManifest(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	writeFile(t, "artifacts/provenance.json", `{"schema":"fairway.provenance.v1","task":"T-001"}`)
	first := runCapture(t, "--json", "provenance", "manifest", "--path", "artifacts/provenance.json")
	assertContains(t, first, `"schema": "fairway.provenance-manifest.v1"`)
	assertContains(t, first, `"ok": true`)
	assertContains(t, first, `"status": "hashed"`)
	assertContains(t, first, `"sha256":`)
	assertNotContains(t, first, `"schema\":\"fairway.provenance.v1\"`)

	writeFile(t, "artifacts/provenance.json", `{"schema":"fairway.provenance.v1","task":"T-001","changed":true}`)
	second := runCapture(t, "--json", "provenance", "manifest", "--path", "artifacts/provenance.json")
	if extractJSONValue(first, "sha256") == extractJSONValue(second, "sha256") {
		t.Fatalf("manifest hash did not change after artifact content changed:\nfirst=%s\nsecond=%s", first, second)
	}

	missing := runCaptureAllowError(t, "provenance", "manifest", "--path", "artifacts/missing.json")
	assertContains(t, missing, "provenance_manifest_ok: false")
	assertContains(t, missing, "missing artifact artifacts/missing.json")

	if err := os.MkdirAll("artifacts/directory", 0o755); err != nil {
		t.Fatal(err)
	}
	directory := runCaptureAllowError(t, "provenance", "manifest", "--path", "artifacts/directory")
	assertContains(t, directory, "directory_rejected")
	assertContains(t, directory, "refusing directory artifact artifacts/directory")

	writeFile(t, "artifacts/secret-token.json", "do-not-export")
	rejected := runCaptureAllowError(t, "provenance", "manifest", "--path", "artifacts/secret-token.json")
	assertContains(t, rejected, "privacy_rejected")
	assertContains(t, rejected, "refusing suspicious evidence path artifacts/secret-token.json")
	assertNotContains(t, rejected, "do-not-export")
}

func TestCLI_UsageCostReport(t *testing.T) {
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
	cfgPath := filepath.Join(".fairway", "config.toml")
	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfgText := string(cfgBytes) + `

[[provider_model_prices]]
provider = "codex"
model = "gpt-5-codex"
input_per_million = 1.0
cached_input_per_million = 0.1
output_per_million = 10.0
reasoning_per_million = 10.0

[[provider_model_prices]]
provider = "codex"
model = "snapshot-model"
total_per_million = 2.0
`
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "tasks.yaml", `- id: EPIC-001
  title: Usage epic
  role: governance
  kind: epic
- id: T-001
  title: Exact usage
  role: backend
  kind: feature
  parent_id: EPIC-001
- id: T-002
  title: Snapshot usage
  role: ops
  kind: readiness-guard
- id: T-003
  title: Unknown usage
  role: governance
  kind: docs
`)
	runOK(t, "import", "tasks.yaml")
	runOK(t, "record", "usage", "T-001", "--provider", "codex", "--role", "backend", "--phase", "implementation", "--source", "provider_reported", "--confidence", "exact", "--input-tokens", "1000", "--cached-input-tokens", "400", "--output-tokens", "200", "--reasoning-tokens", "50", "--total-tokens", "1250", "--model", "gpt-5-codex")
	runOK(t, "record", "usage", "T-002", "--provider", "codex", "--role", "ops", "--phase", "review", "--source", "derived_snapshot", "--confidence", "estimated", "--started-token-snapshot", "10000", "--completed-token-snapshot", "13000", "--model", "snapshot-model")
	runOK(t, "record", "usage", "T-003", "--provider", "codex", "--role", "governance", "--source", "manual", "--confidence", "unknown", "--model", "unknown-model")

	human := runCapture(t, "usage", "cost-report", "--by", "task", "--since-duration", "24h", "--forecast-days", "7")
	assertContains(t, human, "usage cost report by task")
	assertContains(t, human, "T-001 events=1 cost=$0.003140")
	assertContains(t, human, "cache_read=40.0%")
	assertContains(t, human, "T-002 events=1 cost=$0.006000")
	assertContains(t, human, "T-003 events=1 cost=unknown")
	assertContains(t, human, "unknown_cost_events=1")

	modelReport := runCapture(t, "usage", "report", "--by", "model")
	assertContains(t, modelReport, "gpt-5-codex events=1 total=1250")
	assertContains(t, modelReport, "snapshot-model events=1 total=3000")

	markdown := runCapture(t, "usage", "cost-report", "--by", "provider", "--format", "markdown")
	assertContains(t, markdown, "# Usage Cost Report")
	assertContains(t, markdown, "| group | events | estimated cost | forecast | total tokens | cache read | price status | unknown cost events |")

	jsonOut := runCapture(t, "--json", "usage", "cost-report", "--by", "model")
	var report usageCostReport
	if err := json.Unmarshal([]byte(jsonOut), &report); err != nil {
		t.Fatalf("usage cost JSON: %v\n%s", err, jsonOut)
	}
	if report.GroupBy != "model" || len(report.Rows) != 3 {
		t.Fatalf("unexpected report shape: %#v", report)
	}
	for _, row := range report.Rows {
		if row.Group == "unknown-model" && row.EstimatedCostUSD != nil {
			t.Fatalf("unknown model cost should remain unknown, got %#v", row.EstimatedCostUSD)
		}
		if row.Group == "gpt-5-codex" && row.EstimatedCostUSD == nil {
			t.Fatalf("priced model cost missing: %#v", row)
		}
	}

	cfgText = strings.Replace(cfgText, "output_per_million = 10.0", "output_per_million = 20.0", 1)
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o644); err != nil {
		t.Fatal(err)
	}
	changed := runCapture(t, "usage", "cost-report", "--by", "task")
	assertContains(t, changed, "T-001 events=1 cost=$0.005140")
}

func TestCLI_WorkBatches(t *testing.T) {
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
  title: API facade
  role: backend
  owning_domain: platform
- id: T-002
  title: Frontend contract
  role: ui
  owning_domain: platform
- id: T-003
  title: Separate task
  role: backend
`)
	runOK(t, "import", "tasks.yaml")
	runOK(t, "batch", "create", "BATCH-001", "--title", "Platform slice", "--branch", "feature/platform-slice", "--worktree", "../worktrees/platform", "--validation-command", "go test ./...", "--validation-command", "npm test", "--review-domain", "arch,backend", "--task", "T-001", "--task", "T-002", "--rollback-criteria", "revert branch", "--split-criteria", "ownership diverges", "--expected-ci", "github actions")
	runOK(t, "batch", "add", "BATCH-001", "T-003")
	runOK(t, "batch", "remove", "BATCH-001", "T-003")
	runOK(t, "batch", "link", "BATCH-001", "--deploy-run-id", "DEPLOY-001", "--pipeline-id", "gh-run-1")
	runOK(t, "batch", "evidence", "BATCH-001", "--command-text", "go test ./... && npm test", "--result", "pass", "--artifact", "dist/batch.log", "--artifact-type", "ci")
	show := runCapture(t, "batch", "show", "BATCH-001")
	assertContains(t, show, "BATCH-001 Platform slice")
	assertContains(t, show, "T-001")
	assertContains(t, show, "T-002")
	assertContains(t, show, "gh-run-1")
	assertContains(t, show, "go test ./... && npm test")
	list := runCapture(t, "batch", "list")
	assertContains(t, list, "BATCH-001")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "batch BATCH-001: go test ./... && npm test")
	assertContains(t, detail, "work_batch=BATCH-001")
	assertContains(t, detail, "BATCH-001 Platform slice branch=feature/platform-slice pipeline=gh-run-1")
	jsonDetail := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, jsonDetail, `"batches"`)
}

func TestProviderOTelIngestAdapter(t *testing.T) {
	repo := t.TempDir()
	script := filepath.Clean(filepath.Join(mustGetwd(t), "..", "..", "examples", "session-adapters", "provider-otel-ingest.sh"))
	codexOTel := filepath.Join(repo, "codex-otel.json")
	if err := os.WriteFile(codexOTel, []byte(`{
  "resourceLogs": [{
    "resource": {"attributes": [
      {"key":"fairway.task_id","value":{"stringValue":"T-001"}},
      {"key":"fairway.session_id","value":{"stringValue":"codex-s-1"}},
      {"key":"fairway.role","value":{"stringValue":"backend"}},
      {"key":"fairway.phase","value":{"stringValue":"implementation"}},
      {"key":"gen_ai.system","value":{"stringValue":"codex"}}
    ]},
    "scopeLogs": [{"logRecords": [{
      "timeUnixNano": "1767225600000000000",
      "attributes": [
        {"key":"event.name","value":{"stringValue":"response.completed"}},
        {"key":"provider.session_id","value":{"stringValue":"thread-1"}},
        {"key":"gen_ai.response.model","value":{"stringValue":"gpt-5-codex"}},
        {"key":"gen_ai.usage.input_tokens","value":{"intValue":"120"}},
        {"key":"gen_ai.usage.cached_input_tokens","value":{"intValue":"40"}},
        {"key":"gen_ai.usage.output_tokens","value":{"intValue":"30"}},
        {"key":"gen_ai.usage.reasoning_tokens","value":{"intValue":"5"}},
        {"key":"gen_ai.usage.total_tokens","value":{"intValue":"155"}}
      ]
    }]}]
  }]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runAdapter(t, script, "--input", codexOTel, "--dry-run")
	for _, want := range []string{"record usage T-001", "--provider codex", "--session-id codex-s-1", "--external-session-id thread-1", "--input-tokens 120", "--cached-input-tokens 40", "--output-tokens 30", "--reasoning-tokens 5", "--total-tokens 155"} {
		assertContains(t, out, want)
	}

	claudeOTel := filepath.Join(repo, "claude-otel.json")
	if err := os.WriteFile(claudeOTel, []byte(`{
  "resourceMetrics": [{
    "resource": {"attributes": [
      {"key":"fairway.task_id","value":{"stringValue":"T-002"}},
      {"key":"fairway.role","value":{"stringValue":"governance"}},
      {"key":"service.name","value":{"stringValue":"claude"}}
    ]},
    "scopeMetrics": [{"metrics": [{
      "name": "claude_code.token.usage",
      "sum": {"dataPoints": [
        {"asInt": "50", "attributes": [{"key":"token.type","value":{"stringValue":"input"}},{"key":"provider.session_id","value":{"stringValue":"claude-run-1"}}]},
        {"asInt": "20", "attributes": [{"key":"token.type","value":{"stringValue":"cache_read"}},{"key":"provider.session_id","value":{"stringValue":"claude-run-1"}}]},
        {"asInt": "8", "attributes": [{"key":"token.type","value":{"stringValue":"cache_creation"}},{"key":"provider.session_id","value":{"stringValue":"claude-run-1"}}]},
        {"asInt": "30", "attributes": [{"key":"token.type","value":{"stringValue":"output"}},{"key":"provider.session_id","value":{"stringValue":"claude-run-1"}}]},
        {"asInt": "88", "attributes": [{"key":"token.type","value":{"stringValue":"total"}},{"key":"provider.session_id","value":{"stringValue":"claude-run-1"}}]}
      ]}
    }, {
      "name": "claude_code.cost.usage",
      "sum": {"dataPoints": [{
        "asDouble": 0.42,
        "attributes": [
          {"key":"provider.session_id","value":{"stringValue":"claude-run-1"}},
          {"key":"claude_code.query.source","value":{"stringValue":"cli"}}
        ]
      }]}
    }]}]
  }]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out = runAdapter(t, script, "--input", claudeOTel, "--dry-run")
	assertContains(t, out, "record usage T-002")
	assertContains(t, out, "--provider claude")
	assertContains(t, out, "--external-session-id claude-run-1")
	assertContains(t, out, "--input-tokens 50")
	assertContains(t, out, "--cached-input-tokens 20")
	assertContains(t, out, "--output-tokens 30")
	assertContains(t, out, "--total-tokens 88")
	assertContains(t, out, "--metadata cache_creation=8")
	assertContains(t, out, "--metadata cost=0.42")
	assertContains(t, out, "--metadata query_source=cli")
	assertNotContains(t, out, "--input-tokens 0")

	claudeRequestOTel := filepath.Join(repo, "claude-request-otel.json")
	if err := os.WriteFile(claudeRequestOTel, []byte(`{
  "resourceLogs": [{
    "resource": {"attributes": [
      {"key":"fairway.task_id","value":{"stringValue":"T-CLAUDE"}},
      {"key":"fairway.role","value":{"stringValue":"backend"}},
      {"key":"service.name","value":{"stringValue":"claude_code"}}
    ]},
    "scopeLogs": [{"logRecords": [{
      "attributes": [
        {"key":"claude_code.session.id","value":{"stringValue":"claude-session-9"}},
        {"key":"claude_code.api.request.id","value":{"stringValue":"req-9"}},
        {"key":"claude_code.api.request.model","value":{"stringValue":"claude-opus-4"}},
        {"key":"claude_code.api.request.input_tokens","value":{"intValue":"101"}},
        {"key":"claude_code.api.request.cache_read_input_tokens","value":{"intValue":"60"}},
        {"key":"claude_code.api.request.cache_creation_input_tokens","value":{"intValue":"11"}},
        {"key":"claude_code.api.response.output_tokens","value":{"intValue":"33"}},
        {"key":"claude_code.api.request.total_tokens","value":{"intValue":"134"}},
        {"key":"claude_code.cost.usage","value":{"doubleValue":0.7}},
        {"key":"claude_code.query.source","value":{"stringValue":"mcp"}}
      ]
    }]}]
  }]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out = runAdapter(t, script, "--input", claudeRequestOTel, "--dry-run")
	assertContains(t, out, "record usage T-CLAUDE")
	assertContains(t, out, "--provider claude")
	assertContains(t, out, "--external-session-id claude-session-9")
	assertContains(t, out, "--model claude-opus-4")
	assertContains(t, out, "--input-tokens 101")
	assertContains(t, out, "--cached-input-tokens 60")
	assertContains(t, out, "--output-tokens 33")
	assertContains(t, out, "--total-tokens 134")
	assertContains(t, out, "--metadata request_id=req-9")
	assertContains(t, out, "--metadata cache_creation=11")
	assertContains(t, out, "--metadata cost=0.7")
	assertContains(t, out, "--metadata query_source=mcp")

	unknownOTel := filepath.Join(repo, "unknown-otel.json")
	if err := os.WriteFile(unknownOTel, []byte(`{"fairway.task_id":"T-003","provider":"shell","fairway.usage.source":"unknown","fairway.usage.confidence":"unknown"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out = runAdapter(t, script, "--input", unknownOTel, "--dry-run")
	assertContains(t, out, "record usage T-003")
	assertContains(t, out, "--provider shell")
	assertContains(t, out, "--source unknown")
	assertContains(t, out, "--confidence unknown")
	assertNotContains(t, out, "--total-tokens 0")

	sensitiveOTel := filepath.Join(repo, "sensitive-otel.json")
	if err := os.WriteFile(sensitiveOTel, []byte(`{"fairway.task_id":"T-004","provider":"codex","prompt_text":"do not store"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, "--input", sensitiveOTel, "--dry-run")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected sensitive OTel attribute rejection, got success:\n%s", out)
	}
}

func TestCodexUsageAdapter(t *testing.T) {
	repo := t.TempDir()
	script := filepath.Clean(filepath.Join(mustGetwd(t), "..", "..", "examples", "session-adapters", "codex-usage-adapter.sh"))

	execJSON := filepath.Join(repo, "codex-exec.jsonl")
	if err := os.WriteFile(execJSON, []byte(`{"type":"turn.started","content":"ignored generated text"}
{"type":"turn.completed","thread_id":"codex-thread-1","session_id":"codex-session-1","role":"backend","phase":"implementation","model":"gpt-5-codex","usage":{"input_tokens":120,"cached_input_tokens":40,"output_tokens":30,"reasoning_output_tokens":5,"total_tokens":155}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runAdapter(t, script, "--mode", "exec-json", "--input", execJSON, "--task-id", "T-001", "--dry-run")
	for _, want := range []string{"record usage T-001", "--provider codex", "--session-id codex-session-1", "--external-session-id codex-thread-1", "--input-tokens 120", "--cached-input-tokens 40", "--output-tokens 30", "--reasoning-tokens 5", "--total-tokens 155"} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "ignored generated text")

	desktopEventJSON := filepath.Join(repo, "codex-desktop-event-msg.jsonl")
	if err := os.WriteFile(desktopEventJSON, []byte(`{"type":"event_msg","thread_id":"codex-thread-desktop","session_id":"codex-session-desktop","model":"gpt-5-codex","content":"ignored desktop transcript","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":321,"cache_read_input_tokens":100,"output_tokens":45,"reasoning_tokens":6,"total_tokens":372}}}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out = runAdapter(t, script, "--mode", "exec-json", "--input", desktopEventJSON, "--task-id", "T-DESKTOP", "--dry-run")
	for _, want := range []string{"record usage T-DESKTOP", "--provider codex", "--session-id codex-session-desktop", "--external-session-id codex-thread-desktop", "--model gpt-5-codex", "--input-tokens 321", "--cached-input-tokens 100", "--output-tokens 45", "--reasoning-tokens 6", "--total-tokens 372"} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "ignored desktop transcript")

	unknownJSON := filepath.Join(repo, "codex-unknown.json")
	if err := os.WriteFile(unknownJSON, []byte(`{"type":"turn.completed","usage":{},"session_id":"codex-session-unknown"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out = runAdapter(t, script, "--mode", "exec-json", "--input", unknownJSON, "--task-id", "T-002", "--role", "backend", "--dry-run")
	assertContains(t, out, "record usage T-002")
	assertContains(t, out, "--session-id codex-session-unknown")
	assertNotContains(t, out, "--total-tokens 0")

	out = runAdapter(t, script, "--mode", "snapshot", "--task-id", "T-003", "--session-id", "codex-snapshot", "--started-token-snapshot", "1000", "--completed-token-snapshot", "1250", "--dry-run")
	assertContains(t, out, "record usage T-003")
	assertContains(t, out, "--source derived_snapshot")
	assertContains(t, out, "--confidence estimated")
	assertContains(t, out, "--started-token-snapshot 1000")
	assertContains(t, out, "--completed-token-snapshot 1250")

	otelJSON := filepath.Join(repo, "codex-otel.json")
	if err := os.WriteFile(otelJSON, []byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"fairway.task_id","value":{"stringValue":"T-004"}},{"key":"gen_ai.system","value":{"stringValue":"codex"}}]},"scopeLogs":[{"logRecords":[{"attributes":[{"key":"event.name","value":{"stringValue":"response.completed"}},{"key":"gen_ai.usage.total_tokens","value":{"intValue":"77"}}]}]}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out = runAdapter(t, script, "--mode", "otel", "--input", otelJSON, "--task-id", "T-004", "--dry-run")
	assertContains(t, out, "record usage T-004")
	assertContains(t, out, "--provider codex")
	assertContains(t, out, "--total-tokens 77")

	sensitiveJSON := filepath.Join(repo, "codex-sensitive.json")
	if err := os.WriteFile(sensitiveJSON, []byte(`{"type":"turn.completed","usage":{"prompt":"do not store","total_tokens":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, "--mode", "exec-json", "--input", sensitiveJSON, "--task-id", "T-005", "--dry-run")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected sensitive Codex usage key rejection, got success:\n%s", out)
	}
}

func TestCompletionHandbackReport(t *testing.T) {
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
	configPath := filepath.Join(repo, ".fairway", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), `notification_ack_timeout = "24h"`, `notification_ack_timeout = "1ns"`, 1)
	if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "tasks.yaml", `- id: T-001
  title: Delivered handback
  role: ops
  profile: drill
- id: T-002
  title: Open handback
  role: ops
  profile: drill
- id: T-003
  title: Closed handback
  role: ops
  profile: closed-drill
`)
	runOK(t, "import", "tasks.yaml")
	runOK(t, "record", "completion-handback", "T-001", "--to", "arch", "--next-action", "assign follow-up", "--completion-state", "blocked-with-follow-up", "--state", "thread_steered", "--provider", "codex", "--target", "thread-arch")
	runOK(t, "checkpoint", "record", "T-001", "--owner", "arch", "--state", "active", "--summary", "assigned follow-up owner")
	runOK(t, "record", "completion-handback", "T-002", "--to", "arch", "--next-action", "assign next packet", "--completion-state", "live-window-closeout")
	runOK(t, "record", "completion-handback", "T-003", "--to", "arch", "--next-action", "closed follow-up", "--completion-state", "done", "--state", "thread_steered", "--provider", "codex", "--target", "thread-arch")
	runOK(t, "set-status", "T-003", "done", "--reason", "closed task excluded from default idle report")

	out := runCapture(t, "completion-handback-report")
	for _, want := range []string{"completion_handback_idle_report", "rows=2", "stale=1", "completed=1", "open=1", "T-001", "T-002", "by workstream", "drill"} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "T-003")
	assertNotContains(t, out, "by role")

	markdown := runCapture(t, "completion-handback-report", "--format", "markdown")
	assertContains(t, markdown, "# Completion Handback Idle Report")
	assertContains(t, markdown, "| Workstream | Rows | Stale | Completed | Open | Max idle seconds |")

	jsonOut := runCapture(t, "--json", "completion-handback-report")
	assertContains(t, jsonOut, `"total_rows": 2`)
	assertContains(t, jsonOut, `"stale_count": 1`)
	assertContains(t, jsonOut, `"completed_count": 1`)
	assertNotContains(t, jsonOut, "Closed handback")

	includeClosed := runCapture(t, "completion-handback-report", "--include-closed")
	assertContains(t, includeClosed, "T-003")
}

func TestCIMonitorAdapterDryRun(t *testing.T) {
	script := filepath.Clean(filepath.Join(mustGetwd(t), "..", "..", "examples", "session-adapters", "ci-monitor.sh"))

	pass := runAdapter(t, script,
		"--task-id", "T-001",
		"--batch-id", "BATCH-001",
		"--external-run-id", "gha-123",
		"--poll-command", "gh run view gha-123",
		"--source-sha", "abc123",
		"--manual-until", "2026-06-07",
		"--artifact", "https://ci.example/gha-123",
		"--result", "pass",
		"--dry-run",
	)
	for _, want := range []string{
		"session upsert",
		"--provider utility",
		"--backend ci-monitor",
		"--monitor-kind ci",
		"--external-run-id gha-123",
		"--poll-command",
		"watcher start",
		"checkpoint record T-001 --state active",
		"record evidence T-001",
		"--result pass",
		"work_batch=BATCH-001",
		"source_sha=abc123",
		"watcher finish",
		"session end ci-monitor-ci-gha-123",
		"coordinator_handback=ci-monitor ci gha-123 result=pass",
		"reconcile active --dry-run",
	} {
		assertContains(t, pass, want)
	}

	retry := runAdapter(t, script,
		"--task-id", "T-001",
		"--external-run-id", "gha-123",
		"--run-suffix", "retry-1",
		"--poll-command", "gh run view gha-123",
		"--result", "pass",
		"--dry-run",
	)
	for _, want := range []string{
		"session end ci-monitor-ci-gha-123-retry-1",
		"watcher finish ci-monitor-ci-gha-123-retry-1",
		"run_suffix=retry-1",
		"--external-run-id gha-123",
	} {
		assertContains(t, retry, want)
	}

	fail := runAdapter(t, script,
		"--task-id", "T-001",
		"--monitor-kind", "deploy",
		"--external-run-id", "deploy-1",
		"--poll-command", "deployctl status deploy-1",
		"--result", "fail",
		"--dry-run",
	)
	for _, want := range []string{
		"--monitor-kind deploy",
		"--result fail",
		"recommended_followup=CD-FIX-<next>",
		"session end ci-monitor-deploy-deploy-1 --status failed",
		"coordinator_handback=ci-monitor deploy deploy-1 result=fail",
	} {
		assertContains(t, fail, want)
	}

	timeout := runAdapter(t, script,
		"--task-id", "T-001",
		"--monitor-kind", "uat",
		"--external-run-id", "uat-1",
		"--poll-command", "uat status uat-1",
		"--result", "timeout",
		"--dry-run",
	)
	for _, want := range []string{
		"--monitor-kind uat",
		"--result blocked",
		"recommended_followup=UAT-BUG-<next>",
		"session end ci-monitor-uat-uat-1 --status stale",
		"coordinator_handback=ci-monitor uat uat-1 result=timeout",
	} {
		assertContains(t, timeout, want)
	}
}

func TestCIMonitorAdapterLiveSmoke(t *testing.T) {
	repo := t.TempDir()
	fairwayBin := filepath.Join(repo, "fairway")
	build := exec.Command("go", "build", "-o", fairwayBin, ".")
	build.Dir = mustGetwd(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fairway: %v\n%s", err, out)
	}
	runExternal := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(fairwayBin, args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fairway %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	runExternal("init")
	runExternal("add", "T-001", "--title", "Monitor CI", "--role", "ops")

	script := filepath.Clean(filepath.Join(mustGetwd(t), "..", "..", "examples", "session-adapters", "ci-monitor.sh"))
	cmd := exec.Command("bash", script,
		"--task-id", "T-001",
		"--external-run-id", "gha-live",
		"--poll-command", "printf success",
		"--artifact", "dist/gha-live.log",
		"--result", "pass",
	)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "FAIRWAY_BIN="+fairwayBin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-monitor live smoke failed: %v\n%s", err, out)
	}
	detail := runExternal("task-detail", "T-001")
	for _, want := range []string{
		"external_run=gha-live",
		"pass printf success dist/gha-live.log",
	} {
		assertContains(t, detail, want)
	}
	checkpoints := runExternal("checkpoint", "status", "--all")
	assertContains(t, checkpoints, "ci-monitor watching ci run gha-live")
	assertContains(t, checkpoints, "ci-monitor completed ci run gha-live: pass")
	sessions := runExternal("session", "status", "--all")
	assertContains(t, sessions, "ci-monitor-ci-gha-live")
	assertContains(t, sessions, "ended")

	cmd = exec.Command("bash", script,
		"--task-id", "T-001",
		"--external-run-id", "gha-live",
		"--run-suffix", "retry-1",
		"--poll-command", "printf success",
		"--artifact", "dist/gha-live-retry.log",
		"--result", "pass",
	)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "FAIRWAY_BIN="+fairwayBin)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-monitor live retry smoke failed: %v\n%s", err, out)
	}
	detail = runExternal("task-detail", "T-001")
	assertContains(t, detail, "run_suffix=retry-1")
	sessions = runExternal("session", "status", "--all")
	assertContains(t, sessions, "ci-monitor-ci-gha-live-retry-1")
	assertContains(t, sessions, "ended")
}

func TestUtilityEventAdapterDryRun(t *testing.T) {
	script := filepath.Clean(filepath.Join(mustGetwd(t), "..", "..", "examples", "session-adapters", "utility-event.sh"))

	started := runAdapter(t, script,
		"--task-id", "T-001",
		"--batch-id", "BATCH-001",
		"--utility-name", "codegen-drift",
		"--utility-kind", "codegen",
		"--command", "make codegen-check",
		"--external-run-id", "codegen-1",
		"--source-sha", "abc123",
		"--manual-until", "2026-06-07",
		"--artifact", "dist/codegen.log",
		"--state", "started",
		"--dry-run",
	)
	for _, want := range []string{
		"session upsert",
		"--provider utility",
		"--backend codegen-drift",
		"--monitor-kind codegen",
		"--external-run-id codegen-1",
		"--poll-command",
		"watcher start",
		"checkpoint record T-001 --state active",
		"utility=codegen-drift",
		"work_batch=BATCH-001",
		"source_sha=abc123",
	} {
		assertContains(t, started, want)
	}

	completed := runAdapter(t, script,
		"--task-id", "T-001",
		"--utility-name", "release-assets",
		"--utility-kind", "release-asset",
		"--command", "scripts/check-release-assets.sh",
		"--external-run-id", "v0.1.3",
		"--artifact", "dist/release-assets.md",
		"--state", "completed",
		"--recommended-next-action", "continue release checklist",
		"--dry-run",
	)
	for _, want := range []string{
		"checkpoint record T-001 --state done",
		"record evidence T-001",
		"--result pass",
		"--artifact-type release-asset_utility",
		"watcher finish release-assets-release-asset-v0.1.3 --result pass",
		"session end release-assets-release-asset-v0.1.3 --status ended",
		"utility_handback=release-assets kind=release-asset task=T-001 result=pass decision_required=false",
		"recommended_next_action=continue release checklist",
		"reconcile active --dry-run",
	} {
		assertContains(t, completed, want)
	}

	failed := runAdapter(t, script,
		"--task-id", "T-001",
		"--utility-name", "registry-freshness",
		"--utility-kind", "registry",
		"--command", "scripts/check-image-freshness.sh",
		"--external-run-id", "registry-1",
		"--artifact", "dist/registry-freshness.md",
		"--state", "failed",
		"--decision-required",
		"--recommended-next-action", "create OPS-FIX for stale image",
		"--dry-run",
	)
	for _, want := range []string{
		"checkpoint record T-001 --state awaiting_input",
		"--result fail",
		"decision_required=true",
		"utility_handback=registry-freshness kind=registry task=T-001 result=fail decision_required=true",
	} {
		assertContains(t, failed, want)
	}

	stale := runAdapter(t, script,
		"--task-id", "T-001",
		"--utility-name", "stale-branch-scan",
		"--utility-kind", "stale-branch",
		"--command", "git for-each-ref refs/heads",
		"--artifact", "dist/stale-branch-scan.md",
		"--state", "stale",
		"--result", "blocked",
		"--dry-run",
	)
	for _, want := range []string{
		"checkpoint record T-001 --state awaiting_input",
		"--result blocked",
		"session end stale-branch-scan-stale-branch-T-001-stale-branch --status stale",
		"utility_handback=stale-branch-scan kind=stale-branch task=T-001 result=stale",
	} {
		assertContains(t, stale, want)
	}
}

func TestProviderEventAdapterFailsClosedOnSessionStatusParsing(t *testing.T) {
	script := filepath.Clean(filepath.Join(mustGetwd(t), "..", "..", "examples", "session-adapters", "provider-event.sh"))
	fairwayBin := writeFakeFairwaySessionStatus(t)
	baseArgs := []string{
		"--provider", "codex",
		"--external-session-id", "thread-1",
		"--role", "backend",
		"--task-id", "T-001",
		"--state", "running",
		"--summary", "still working",
	}

	out, err := runAdapterWithEnv(t, []string{"FAIRWAY_BIN=" + fairwayBin, `FAKE_SESSION_JSON=[]`}, script, baseArgs...)
	if err != nil {
		t.Fatalf("missing session should be allowed for first event: %v\n%s", err, out)
	}
	assertContains(t, out, "session upsert")

	for _, tc := range []struct {
		name string
		json string
		want string
	}{
		{name: "malformed", json: `{not-json`, want: "refusing provider event because Fairway session status could not be parsed"},
		{name: "missing task", json: `[{"id":"codex-thread-1","status":"running"}]`, want: "refusing provider event because Fairway session status could not be parsed"},
		{name: "mismatched task", json: `[{"id":"codex-thread-1","status":"running","task_id":"OTHER-001"}]`, want: "session/task mismatch for codex-thread-1"},
		{name: "ended session", json: `[{"id":"codex-thread-1","status":"ended","task_id":"T-001"}]`, want: "refusing provider event running for terminal session codex-thread-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runAdapterWithEnv(t, []string{"FAIRWAY_BIN=" + fairwayBin, "FAKE_SESSION_JSON=" + tc.json}, script, baseArgs...)
			if err == nil {
				t.Fatalf("adapter succeeded, expected fail-closed\n%s", out)
			}
			assertContains(t, out, tc.want)
		})
	}
}

func TestUtilityEventAdapterFailsClosedOnSessionStatusParsing(t *testing.T) {
	script := filepath.Clean(filepath.Join(mustGetwd(t), "..", "..", "examples", "session-adapters", "utility-event.sh"))
	fairwayBin := writeFakeFairwaySessionStatus(t)
	baseArgs := []string{
		"--task-id", "T-001",
		"--utility-name", "release-assets",
		"--utility-kind", "release-asset",
		"--command", "scripts/check-release-assets.sh",
		"--external-run-id", "v0.1.3",
		"--state", "heartbeat",
	}

	out, err := runAdapterWithEnv(t, []string{"FAIRWAY_BIN=" + fairwayBin, `FAKE_SESSION_JSON=[]`}, script, baseArgs...)
	if err != nil {
		t.Fatalf("missing session should be allowed for first utility event: %v\n%s", err, out)
	}
	assertContains(t, out, "session upsert")

	for _, tc := range []struct {
		name string
		json string
		want string
	}{
		{name: "malformed", json: `{not-json`, want: "refusing utility event because Fairway session status could not be parsed"},
		{name: "missing task", json: `[{"id":"release-assets-release-asset-v0.1.3","status":"running"}]`, want: "refusing utility event because Fairway session status could not be parsed"},
		{name: "mismatched task", json: `[{"id":"release-assets-release-asset-v0.1.3","status":"running","task_id":"OTHER-001"}]`, want: "session/task mismatch for release-assets-release-asset-v0.1.3"},
		{name: "ended session", json: `[{"id":"release-assets-release-asset-v0.1.3","status":"ended","task_id":"T-001"}]`, want: "refusing utility event heartbeat for terminal session release-assets-release-asset-v0.1.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runAdapterWithEnv(t, []string{"FAIRWAY_BIN=" + fairwayBin, "FAKE_SESSION_JSON=" + tc.json}, script, baseArgs...)
			if err == nil {
				t.Fatalf("adapter succeeded, expected fail-closed\n%s", out)
			}
			assertContains(t, out, tc.want)
		})
	}
}

func TestUtilityEventAdapterLiveSmoke(t *testing.T) {
	repo := t.TempDir()
	fairwayBin := filepath.Join(repo, "fairway")
	build := exec.Command("go", "build", "-o", fairwayBin, ".")
	build.Dir = mustGetwd(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fairway: %v\n%s", err, out)
	}
	runExternal := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(fairwayBin, args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fairway %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	runExternal("init")
	runExternal("add", "T-001", "--title", "Release asset check", "--role", "ops")

	script := filepath.Clean(filepath.Join(mustGetwd(t), "..", "..", "examples", "session-adapters", "utility-event.sh"))
	for _, args := range [][]string{
		{
			"--task-id", "T-001",
			"--utility-name", "release-assets",
			"--utility-kind", "release-asset",
			"--command", "scripts/check-release-assets.sh",
			"--external-run-id", "v0.1.3",
			"--artifact", "dist/release-assets.md",
			"--state", "started",
		},
		{
			"--task-id", "T-001",
			"--utility-name", "release-assets",
			"--utility-kind", "release-asset",
			"--command", "scripts/check-release-assets.sh",
			"--external-run-id", "v0.1.3",
			"--artifact", "dist/release-assets.md",
			"--state", "completed",
			"--recommended-next-action", "continue release checklist",
		},
	} {
		cmd := exec.Command("bash", append([]string{script}, args...)...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "FAIRWAY_BIN="+fairwayBin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("utility-event live smoke failed: %v\n%s", err, out)
		}
	}

	detail := runExternal("task-detail", "T-001")
	for _, want := range []string{
		"pass scripts/check-release-assets.sh dist/release-assets.md",
		"utility=release-assets",
		"next_action=continue release checklist",
	} {
		assertContains(t, detail, want)
	}
	sessions := runExternal("session", "status", "--all")
	assertContains(t, sessions, "release-assets-release-asset-v0.1.3")
	assertContains(t, sessions, "ended")
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
	plan := runCapture(t, "--json", "coordinator", "plan")
	for _, want := range []string{`"dry_run": true`, `"actions":`, `"top_classification":`} {
		if !strings.Contains(plan, want) {
			t.Fatalf("coordinator plan missing %q:\n%s", want, plan)
		}
	}
	runOK(t, "--json", "coordinator", "tick")
	runOK(t, "coordinator", "preflight")
	runOK(t, "dispatch-plan")
	runOK(t, "--json", "dispatch-plan", "--role", "backend")
}

func TestCLI_ReviewWaitsListHumanAndJSON(t *testing.T) {
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

[[roles]]
name = "arch"
provider = "codex"
`)
	runOK(t, "add", "T-001", "--title", "Needs review", "--role", "backend", "--review-domains", "arch")
	runOK(t, "add", "T-002", "--title", "Backlog review later", "--role", "backend", "--review-domains", "arch")
	runOK(t, "set-status", "T-001", "in_progress", "--reason", "entered review wait")

	out := runCapture(t, "review-waits", "list", "--task", "T-001", "--blocking")
	assertContains(t, out, "review_waits:")
	assertContains(t, out, "domain=arch")
	assertContains(t, out, "state=pending")
	assertContains(t, out, "action=deliver_notification")

	jsonOut := runCapture(t, "--json", "review-waits", "list", "--task", "T-001")
	assertContains(t, jsonOut, `"wait_id": "T-001/arch"`)
	assertContains(t, jsonOut, `"action": "deliver_notification"`)

	staleOut := runCapture(t, "review-waits", "list", "--task", "T-001", "--stale")
	assertContains(t, staleOut, "review_waits: none")

	todoOut := runCapture(t, "review-waits", "list", "--task", "T-002", "--blocking")
	assertContains(t, todoOut, "review_waits: none")
}

func TestCLI_ReviewPolicyProfilesExplainReviewRequirements(t *testing.T) {
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
[[review_profiles]]
name = "micro-slice"
mode = "advisory"
match_tags = ["review:micro"]
required_review_domains = ["governance"]
waive_review_domains = ["backend"]
defer_review_domains = ["ops"]
safe_iteration_zone = true
safe_iteration_defect_class = "harness"
safe_iteration_control = "non-live disposable boundary"
extra_reviewer_rationale = "governance catches evidence contract drift"
process_hypothesis = "one governance review should catch evidence drift without full matrix overhead"
outcome_metrics = ["defects_caught", "cycle_time", "avoided_unsafe_actions"]

[[review_profiles]]
name = "grouped-slice"
match_tags = ["review:grouped"]
required_review_domains = ["backend", "governance"]
inherit_from_parent = true
inherit_review_domains = ["backend", "governance"]
group_review = true
`)

	runOK(t, "add", "MICRO-001", "--title", "Tiny docs slice", "--role", "backend", "--tag", "review:micro", "--review-domains", "backend,ops")
	runOK(t, "set-status", "MICRO-001", "in_progress", "--reason", "review policy check")
	detail := runCapture(t, "task-detail", "MICRO-001")
	for _, want := range []string{
		"review_policy:",
		"profile: micro-slice mode=advisory",
		"backend: waived",
		"ops: deferred",
		"governance: required",
		"safe_iteration_zone: true defect_class=harness control=non-live disposable boundary",
		"extra_reviewer_rationale: governance catches evidence contract drift",
		"process_hypothesis: one governance review should catch evidence drift without full matrix overhead",
		"outcome_metrics: avoided_unsafe_actions, cycle_time, defects_caught",
		"missing review domains:",
		"- governance",
	} {
		assertContains(t, detail, want)
	}

	waits := runCapture(t, "review-waits", "list", "--task", "MICRO-001")
	for _, want := range []string{
		"domain=backend state=resolved blocking=false action=none policy=waived profile=micro-slice",
		"domain=ops state=cancelled blocking=false action=deferred_review policy=deferred profile=micro-slice",
		"domain=governance state=notification_failed blocking=false action=mapping_required policy=required profile=micro-slice",
	} {
		assertContains(t, waits, want)
	}
	runOK(t, "record", "evidence", "MICRO-001", "--command-text", "near-ready harness readback", "--result", "pass", "--notes", "ready to retry after review")
	runOK(t, "record", "evidence", "MICRO-001", "--command-text", "offline harness check", "--result", "fail", "--artifact-type", "harness", "--notes", "defect caught during advisory pilot")
	runOK(t, "record", "evidence", "MICRO-001", "--command-text", "offline harness retry", "--result", "blocked", "--artifact-type", "harness", "--notes", "same harness failure after near-ready claim")
	report := runCapture(t, "review-policy", "report", "--profile", "micro-slice")
	for _, want := range []string{
		"review_policy_report:",
		"profile=micro-slice mode=advisory",
		"defects_caught=1",
		"avoided_unsafe=1",
		"loop_detected=1",
		"recommendation=recommend causal reset with lighter safe-boundary review before another retry",
		"hypothesis=one governance review should catch evidence drift without full matrix overhead",
		"outcome_metrics=avoided_unsafe_actions, cycle_time, defects_caught",
		"causal_reset=MICRO-001; loop detected: repeated meaningful failures, same-layer=harness, near-ready-claim",
		"required_proof_before_retry=record a causal-reset task or evidence packet that explains the failure chain; record passing proof for harness before another live or broad retry packet",
		"lighter_review_plan=stay inside non-live disposable boundary for harness fixes with one accountable review until a boundary exit is requested",
	} {
		assertContains(t, report, want)
	}

	runOK(t, "add", "EPIC-001", "--title", "Grouped packet", "--role", "backend")
	runOK(t, "record", "review", "EPIC-001", "--reviewer", "fairway-reviewer", "--domain", "backend", "--verdict", "approve", "--reason", "group packet")
	runOK(t, "record", "review", "EPIC-001", "--reviewer", "fairway-reviewer", "--domain", "governance", "--verdict", "approve", "--reason", "group packet")
	runOK(t, "add", "CHILD-001", "--title", "Grouped child", "--role", "backend", "--parent", "EPIC-001", "--tag", "review:grouped")
	childWaits := runCapture(t, "review-waits", "list", "--task", "CHILD-001")
	assertContains(t, childWaits, "domain=backend state=resolved blocking=false action=none policy=inherited profile=grouped-slice")
	assertContains(t, childWaits, "domain=governance state=resolved blocking=false action=none policy=inherited profile=grouped-slice")
}

func TestCLI_ReviewWaitsWakeSelectionAndSuppression(t *testing.T) {
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
	replaceInFile(t, ".fairway/config.toml", `notification_ack_timeout = "24h"`, `notification_ack_timeout = "1ns"`)
	appendFile(t, ".fairway/config.toml", `
[[roles]]
name = "backend"

[[roles]]
name = "arch"
provider = "codex"

[[roles]]
name = "ops"
provider = "codex"

[[provider_targets]]
domain = "arch"
provider = "codex"
target = "thread-arch"
type = "thread"

[[provider_targets]]
domain = "ops"
provider = "codex"
target = "thread-ops"
type = "thread"
`)
	runOK(t, "add", "T-001", "--title", "Stale review wait", "--role", "backend", "--review-domains", "arch")
	runOK(t, "add", "T-002", "--title", "Failed review wait", "--role", "backend", "--review-domains", "product")
	runOK(t, "add", "T-003", "--title", "Resolved review wait", "--role", "backend", "--review-domains", "ops")
	runOK(t, "add", "T-004", "--title", "Historical resolved wait", "--role", "backend", "--review-domains", "ops")
	runOK(t, "add", "T-005", "--title", "Blocked resolved review wait", "--role", "backend", "--review-domains", "ops")
	for _, taskID := range []string{"T-001", "T-002", "T-003", "T-004", "T-005"} {
		runOK(t, "set-status", taskID, "in_progress", "--reason", "entered review wait")
	}
	runOK(t, "record", "notification", "T-001", "--domain", "arch", "--provider", "codex", "--target", "thread-arch", "--state", "notification_delivered", "--reason", "sent")
	runOK(t, "record", "notification", "T-002", "--domain", "product", "--provider", "codex", "--target", "missing", "--state", "notification_failed", "--reason", "no mapping")
	runOK(t, "record", "review", "T-003", "--reviewer", "ops-reviewer", "--domain", "ops", "--verdict", "approve", "--reason", "ok")
	runOK(t, "record", "review", "T-004", "--reviewer", "ops-reviewer", "--domain", "ops", "--verdict", "approve", "--reason", "ok")
	runOK(t, "record", "review", "T-005", "--reviewer", "ops-reviewer", "--domain", "ops", "--verdict", "approve", "--reason", "ok")
	runOK(t, "set-status", "T-004", "done", "--reason", "closed already")
	runOK(t, "set-status", "T-005", "blocked", "--reason", "task-level blocker remains")

	time.Sleep(time.Millisecond)
	dryRun := runCapture(t, "review-waits", "wake", "--task", "T-001")
	assertContains(t, dryRun, "review_wait_wakes:")
	assertContains(t, dryRun, "kind=stale")
	assertContains(t, dryRun, "target=thread-arch")
	assertContains(t, dryRun, "Review wait update for T-001:")
	assertContains(t, dryRun, "1. Re-run fairway review-waits list --task T-001.")
	assertContains(t, dryRun, "2. Address the blocking review wait before merge-ready or closeout.")
	assertNotContains(t, dryRun, "fairway merge-ready T-001")

	sent := runCapture(t, "review-waits", "wake", "--task", "T-001", "--send", "--state", "thread_steered")
	assertContains(t, sent, "kind=stale")
	assertContains(t, sent, "signature=T-001|stale|task_status=in_progress|")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "thread_steered domain=coordinator provider=codex target=thread-arch")
	assertContains(t, detail, "review_wait_wake signature=T-001|stale|task_status=in_progress|")

	suppressed := runCapture(t, "review-waits", "wake", "--task", "T-001", "--send", "--state", "thread_steered")
	assertContains(t, suppressed, "status=suppressed")

	failed := runCapture(t, "review-waits", "wake", "--task", "T-002", "--send")
	assertContains(t, failed, "kind=notification_failed")
	assertContains(t, failed, "status=failed")
	failedDetail := runCapture(t, "task-detail", "T-002")
	assertContains(t, failedDetail, "notification_failed domain=coordinator")
	assertContains(t, failedDetail, "failed=no_wake_target")

	resolved := runCapture(t, "review-waits", "wake", "--task", "T-003")
	assertContains(t, resolved, "kind=resolved")
	assertContains(t, resolved, "task_status=in_progress")
	assertContains(t, resolved, "review_only=true")
	assertContains(t, resolved, "target=thread-ops")
	assertContains(t, resolved, "- ops: resolved action=run_merge_ready")
	assertNotContains(t, resolved, "fairway merge-ready T-003")
	assertContains(t, resolved, "review resolution does not authorize merge-ready or closeout")

	blockedResolved := runCapture(t, "review-waits", "wake", "--task", "T-005")
	assertContains(t, blockedResolved, "kind=resolved")
	assertContains(t, blockedResolved, "task_status=blocked")
	assertContains(t, blockedResolved, "review_only=true")
	assertContains(t, blockedResolved, "Task status: blocked")
	assertContains(t, blockedResolved, "task status is blocked")
	assertContains(t, blockedResolved, "- ops: resolved action=run_merge_ready")
	assertNotContains(t, blockedResolved, "fairway merge-ready T-005")
	assertNotContains(t, blockedResolved, "If gates pass, continue reviewed-lane closeout.")

	historical := runCapture(t, "review-waits", "wake", "--task", "T-004")
	assertContains(t, historical, "review_wait_wakes: none")
}

func TestCLI_RouteReviewAndPreflightReportUnroutableDomains(t *testing.T) {
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
name = "arch"
`)
	runOK(t, "add", "T-001", "--title", "Unroutable review", "--role", "backend", "--review-domains", "product")

	_, err = captureRun("route", "review", "T-001", "--reviewer", "arch", "--reason", "manual")
	if err == nil {
		t.Fatal("route review succeeded, expected unroutable-domain failure")
	}
	if !strings.Contains(err.Error(), "required review domain product") || !strings.Contains(err.Error(), "not routable") {
		t.Fatalf("route review error = %v", err)
	}

	runOK(t, "set-status", "T-001", "in_progress", "--reason", "active review routing check")
	preflight, err := captureRun("coordinator", "preflight")
	if err == nil {
		t.Fatalf("coordinator preflight succeeded, expected unroutable-domain issue:\n%s", preflight)
	}
	assertContains(t, preflight, "required review domain product is not routable")
	assertContains(t, preflight, "action=mapping_required")
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

func TestCLI_TrackMemoryPackets(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Memory task", "--role", "backend")
	runOK(t, "claim", "T-001")
	runOK(t, "session", "upsert", "--id", "s-1", "--role", "backend", "--provider", "codex", "--task-id", "T-001", "--status", "running")
	runOK(t, "checkpoint", "record", "T-001", "--state", "active", "--owner", "backend", "--summary", "implement memory packet")
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./cmd/fairway", "--result", "pass")

	runOK(t, "memory", "update",
		"--track", "architecture-control",
		"--title", "Architecture Control",
		"--purpose", "coordinate tracks",
		"--operating-mode", "control",
		"--active-scope", "fairway",
		"--current-objective", "finish memory packet",
		"--decision", "store curated summaries only",
		"--next-action", "route review")
	runOK(t, "memory", "append", "--track", "architecture-control", "--blocker", "none", "--open-question", "review route")

	show := runCapture(t, "memory", "show", "--track", "architecture-control")
	assertContains(t, show, "architecture-control")
	assertContains(t, show, "finish memory packet")
	assertContains(t, show, "route review")

	packet := runCapture(t, "memory", "packet", "--track", "architecture-control", "--for", "codex")
	assertContains(t, packet, "# Track Memory Packet: architecture-control")
	assertContains(t, packet, "for: codex")
	assertContains(t, packet, "T-001 in_progress Memory task")
	assertContains(t, packet, "s-1 running task=T-001")
	assertNotContains(t, packet, "raw transcript")
	assertNotContains(t, packet, "prompt body")

	jsonPacket := runCapture(t, "--json", "memory", "packet", "--track", "architecture-control")
	assertContains(t, jsonPacket, `"track_id": "architecture-control"`)
	assertContains(t, jsonPacket, `"active_tasks"`)

	stale := runCapture(t, "memory", "stale", "--older-than", "0s")
	assertContains(t, stale, "architecture-control")

	if out, err := captureRun("memory", "update", "--track", "bad-source", "--source-checkpoint-id", "999999"); err == nil {
		t.Fatalf("memory update with missing source succeeded:\n%s", out)
	}
}

func TestCLI_GenericWaitListAndTick(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Needs approval", "--role", "backend")
	runOK(t, "claim", "T-001")
	runOK(t, "checkpoint", "record", "T-001", "--state", "awaiting_input", "--owner", "arch", "--summary", "approval required before retry")
	runOK(t, "memory", "update", "--track", "architecture-control", "--current-objective", "watch parked work")

	waits := runCapture(t, "wait", "list", "--task", "T-001")
	assertContains(t, waits, "kind=approval")
	assertContains(t, waits, "approval required before retry")
	assertNotContains(t, waits, "send provider")

	tick := runCapture(t, "wait", "tick", "--task", "T-001")
	assertContains(t, tick, "wait_tick: dry-run")
	assertContains(t, tick, "kind=approval")
	assertContains(t, tick, "approval required before retry")

	staleOnly := runCapture(t, "wait", "tick", "--task", "T-001", "--stale")
	assertContains(t, staleOnly, "wait_tick: dry-run")
	assertContains(t, staleOnly, "- none")
	assertNotContains(t, staleOnly, "kind=approval")

	staleMemory := runCapture(t, "wait", "tick", "--kind", "track_memory", "--memory-stale-after", "0s")
	assertContains(t, staleMemory, "wait_tick: dry-run")
	assertContains(t, staleMemory, "kind=track_memory")
	assertContains(t, staleMemory, "refresh_track_memory")

	jsonWaits := runCapture(t, "--json", "wait", "list", "--task", "T-001")
	assertContains(t, jsonWaits, `"kind": "approval"`)
	assertContains(t, jsonWaits, `"source": "coordinator_plan"`)
}

func TestCLI_GenericWaitAddAndAck(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Operator handoff", "--role", "backend")

	added := runCapture(t, "wait", "add",
		"--task", "T-001",
		"--track", "architecture-control",
		"--kind", "live_window",
		"--on", "operator closeout",
		"--target", "thread-operator",
		"--deadline", "2000-01-01",
		"--deadline-source", "manual-live-window",
		"--action", "collect_closeout",
		"--reason", "operator must report final state before next decision",
		"--suggested-command", "fairway live-window control-room --task T-001",
	)
	waitID := "manual:t-001:live_window:architecture-control:operator-closeout"
	assertContains(t, added, "wait added "+waitID)
	assertContains(t, added, "suggested_command: fairway live-window control-room --task T-001")

	runOK(t, "record", "notification", "T-001",
		"--domain", "coordinator",
		"--state", "sent",
		"--reason", "generic_wait_wake signature=test kind=live_window wait_id="+waitID,
	)

	waits := runCapture(t, "wait", "list", "--task", "T-001", "--kind", "live_window")
	assertContains(t, waits, waitID)
	assertContains(t, waits, "source")
	assertContains(t, waits, "target=thread-operator")
	assertContains(t, waits, "deadline=2000-01-01")
	assertContains(t, waits, "deadline_source=manual-live-window")
	assertContains(t, waits, "stale=true")
	assertContains(t, waits, "stale_age=")
	assertContains(t, waits, "last_wake_attempt=")
	assertContains(t, waits, "operator must report final state")

	jsonWaits := runCapture(t, "--json", "wait", "list", "--task", "T-001", "--kind", "live_window")
	assertContains(t, jsonWaits, `"source": "manual_wait"`)
	assertContains(t, jsonWaits, `"condition": "operator closeout"`)
	assertContains(t, jsonWaits, `"deadline_source": "manual-live-window"`)
	assertContains(t, jsonWaits, `"last_wake_attempt_at":`)

	ack := runCapture(t, "wait", "ack", waitID, "--actor", "architecture-control", "--reason", "operator closeout received")
	assertContains(t, ack, "wait acknowledged "+waitID)
	assertContains(t, ack, "operator closeout received")

	afterAck := runCapture(t, "wait", "list", "--task", "T-001", "--kind", "live_window")
	assertContains(t, afterAck, "waits:")
	assertContains(t, afterAck, "- none")

	checkpoints := runCapture(t, "checkpoint", "status", "--all")
	assertContains(t, checkpoints, "fairway_wait:")
}

func TestCLI_GenericWaitWakeDelivery(t *testing.T) {
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
	replaceInFile(t, ".fairway/config.toml", `notification_ack_timeout = "24h"`, `notification_ack_timeout = "1ns"`)
	appendFile(t, ".fairway/config.toml", `
[[roles]]
name = "backend"

[[roles]]
name = "arch"
provider = "codex"

[[roles]]
name = "ops"
provider = "codex"

[[provider_targets]]
domain = "arch"
provider = "codex"
target = "thread-arch"
type = "thread"

[[provider_targets]]
domain = "ops"
provider = "codex"
target = "thread-ops"
type = "thread"
`)
	runOK(t, "add", "T-001", "--title", "Stale review wait", "--role", "backend", "--review-domains", "arch")
	runOK(t, "add", "T-002", "--title", "Unroutable review wait", "--role", "backend", "--review-domains", "product")
	runOK(t, "add", "T-003", "--title", "Resolved review wait", "--role", "backend", "--review-domains", "ops")
	for _, taskID := range []string{"T-001", "T-002", "T-003"} {
		runOK(t, "set-status", taskID, "in_progress", "--reason", "entered wait")
	}
	runOK(t, "record", "notification", "T-001", "--domain", "arch", "--provider", "codex", "--target", "thread-arch", "--state", "notification_delivered", "--reason", "sent")
	runOK(t, "record", "notification", "T-002", "--domain", "product", "--provider", "codex", "--target", "missing", "--state", "notification_failed", "--reason", "no mapping")
	runOK(t, "record", "review", "T-003", "--reviewer", "ops-reviewer", "--domain", "ops", "--verdict", "approve", "--reason", "ok")
	time.Sleep(time.Millisecond)

	dryRun := runCapture(t, "wait", "wake", "--task", "T-001", "--kind", "review")
	assertContains(t, dryRun, "generic_wait_wakes:")
	assertContains(t, dryRun, "kind=review")
	assertContains(t, dryRun, "target=thread-arch")
	assertContains(t, dryRun, "Generic wait wake for T-001:")
	assertContains(t, dryRun, "1. Re-run fairway wait list --task T-001 --kind review.")
	assertContains(t, dryRun, "2. Re-run the source-specific command named above before acting.")
	assertContains(t, dryRun, "Do not treat this wake as approval, merge, deploy, live execution, or dashboard send authority.")

	sent := runCapture(t, "wait", "wake", "--task", "T-001", "--kind", "review", "--send", "--state", "thread_steered")
	assertContains(t, sent, "status=ready")
	assertContains(t, sent, "signature=T-001|review|task_status=in_progress|")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "thread_steered domain=coordinator provider=codex target=thread-arch")
	assertContains(t, detail, "generic_wait_wake signature=T-001|review|task_status=in_progress|")

	suppressed := runCapture(t, "wait", "wake", "--task", "T-001", "--kind", "review", "--send", "--state", "thread_steered")
	assertContains(t, suppressed, "status=suppressed")

	failed := runCapture(t, "wait", "wake", "--task", "T-002", "--kind", "review", "--send")
	assertContains(t, failed, "status=failed")
	assertContains(t, failed, "target=none")
	assertContains(t, failed, "target_action=mapping_required")
	assertContains(t, failed, "Wake target: mapping_required")
	failedDetail := runCapture(t, "task-detail", "T-002")
	assertContains(t, failedDetail, "notification_failed domain=coordinator")
	assertContains(t, failedDetail, "generic_wait_wake signature=T-002|review|task_status=in_progress|")
	assertContains(t, failedDetail, "failed=no_wake_target")
	assertContains(t, failedDetail, "action=mapping_required")

	resolved := runCapture(t, "wait", "wake", "--task", "T-003", "--kind", "review")
	assertContains(t, resolved, "generic_wait_wakes: none")
}

func TestCLI_GenericWaitWakeCoversTrackMemoryAndProviderSessionTargets(t *testing.T) {
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
name = "product"
provider = "codex"
`)
	runOK(t, "memory", "update", "--track", "product", "--title", "Product memory", "--current-objective", "refresh packet")
	time.Sleep(time.Millisecond)

	memoryWake := runCapture(t, "wait", "wake", "--kind", "track_memory", "--memory-stale-after", "1ns")
	assertContains(t, memoryWake, "kind=track_memory")
	assertContains(t, memoryWake, "wait=memory:product")
	assertContains(t, memoryWake, "status=ready")
	assertContains(t, memoryWake, "target_action=mapping_required")
	assertContains(t, memoryWake, "Wake target: mapping_required")
	assertContains(t, memoryWake, "1. Re-run fairway wait list --kind track_memory.")
	memorySend := runCapture(t, "wait", "wake", "--kind", "track_memory", "--memory-stale-after", "1ns", "--send")
	assertContains(t, memorySend, "kind=track_memory")
	assertContains(t, memorySend, "status=failed")
	assertContains(t, memorySend, "target_action=mapping_required")

	runOK(t, "add", "T-004", "--title", "Provider session lifecycle", "--role", "product")
	runOK(t, "set-status", "T-004", "in_progress", "--reason", "session active")
	runOK(t, "session", "upsert",
		"--id", "product-session",
		"--role", "product",
		"--backend", "codex-thread",
		"--provider", "codex",
		"--task-id", "T-004",
		"--status", "running",
	)
	sessionWake := runCapture(t, "wait", "wake", "--task", "T-004", "--kind", "provider_session")
	assertContains(t, sessionWake, "kind=provider_session")
	assertContains(t, sessionWake, "status=ready")
	assertContains(t, sessionWake, "target_action=mapping_required")
	assertContains(t, sessionWake, "Wake target: mapping_required")
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

func TestCLI_ReconcileActiveAllowsBoundedLiveOperationEvidence(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Approved live drill", "--role", "ops")
	t.Setenv("FAIRWAY_ROLE", "ops")
	runOK(t, "claim", "T-001")
	runOK(t, "session", "upsert",
		"--id", "codex-live-op",
		"--role", "ops",
		"--backend", "codex-thread",
		"--provider", "codex",
		"--task-id", "T-001",
	)
	runOK(t, "checkpoint", "record", "T-001",
		"--state", "active",
		"--owner", "ops",
		"--target-close-by", time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano),
		"--summary", "Provider session codex-live-op active; approved live operation window with expected closeout.",
	)
	runOK(t, "record", "evidence", "T-001",
		"--command-text", "admin readiness gate && pre-mutation validator",
		"--result", "pass",
		"--artifact-type", "live-operation-gate",
		"--notes", "GPUaaS 21:15 pattern: gate evidence captured during bounded active operation.",
	)

	report := runCapture(t, "reconcile", "active", "--dry-run")
	assertContains(t, report, "no active reconciliation findings")
	assertNotContains(t, report, "status_decision_required")
}

func TestCLI_LiveWindowRecordAndStatus(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Repeated live drill", "--role", "ops")
	out := runCapture(t, "live-window", "record", "T-001",
		"--phase", "gate-running",
		"--next-owner", "ops",
		"--next-action", "run browser smoke",
		"--target-close-by", "2026-06-13T03:15:00Z",
		"--artifact", "packet.md",
	)
	assertContains(t, out, "live_window recorded T-001 phase=gate-running state=active")

	status := runCapture(t, "live-window", "status", "--task", "T-001")
	assertContains(t, status, "live_windows:")
	assertContains(t, status, "T-001 phase=gate-running")
	assertContains(t, status, "next_owner=ops")
	assertContains(t, status, "next_action=run browser smoke")
	assertContains(t, status, "target_close_by=2026-06-13T03:15:00Z")

	jsonStatus := runCapture(t, "--json", "live-window", "status", "--task", "T-001")
	assertContains(t, jsonStatus, `"phase": "gate-running"`)
	assertContains(t, jsonStatus, `"next_action": "run browser smoke"`)
}

func TestCLI_LiveWindowControlRoom(t *testing.T) {
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
	runOK(t, "add", "MFA-1320", "--title", "MFA 13:20 drill", "--role", "ops")
	runOK(t, "add", "DONE-001", "--title", "Completed drill", "--role", "ops")
	deadline := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	runOK(t, "live-window", "record", "MFA-1320",
		"--phase", "approvals_ready",
		"--next-owner", "architecture-control",
		"--next-action", "authorize operator handoff",
		"--authorization-state", "approvals recorded; execution not authorized",
		"--command", "fairway live-window record MFA-1320 --phase execution_authorized",
		"--prompt", "Authorize the drill operator for the approved 13:20 MFA window",
		"--missed-deadline-action", "escalate to architecture control and reschedule window",
		"--target-close-by", deadline,
		"--artifact", ".fairway/artifacts/mfa-1320/packet.md",
	)
	runOK(t, "live-window", "record", "DONE-001",
		"--phase", "done",
		"--next-owner", "architecture-control",
		"--next-action", "archive packet",
		"--target-close-by", deadline,
	)

	status := runCapture(t, "live-window", "status", "--task", "MFA-1320")
	assertContains(t, status, "phase=approvals_ready")
	assertContains(t, status, "authorization=approvals recorded; execution not authorized")
	assertContains(t, status, "command=fairway live-window record MFA-1320 --phase execution_authorized")
	assertContains(t, status, "missed_deadline_action=escalate to architecture control and reschedule window")

	room := runCapture(t, "live-window", "control-room", "--task", "MFA-1320")
	assertContains(t, room, "live_operation_control_room:")
	assertContains(t, room, "MFA-1320 phase=approvals_ready")
	assertContains(t, room, "next_actor=architecture-control")
	assertContains(t, room, "deadline_state=missed")
	assertContains(t, room, "authorization=approvals recorded; execution not authorized")
	assertContains(t, room, "command=fairway live-window record MFA-1320 --phase execution_authorized")
	assertContains(t, room, "prompt=Authorize the drill operator for the approved 13:20 MFA window")
	assertContains(t, room, "missed_deadline_action=escalate to architecture control and reschedule window")

	stale := runCapture(t, "live-window", "control-room", "--stale")
	assertContains(t, stale, "MFA-1320 phase=approvals_ready")
	assertNotContains(t, stale, "DONE-001")

	jsonRoom := runCapture(t, "--json", "live-window", "control-room", "--task", "MFA-1320")
	assertContains(t, jsonRoom, `"phase": "approvals_ready"`)
	assertContains(t, jsonRoom, `"deadline_state": "missed"`)
	assertContains(t, jsonRoom, `"missed_deadline_action": "escalate to architecture control and reschedule window"`)
}

func TestCLI_RecordCompletionHandback(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Delegated closeout", "--role", "backend")
	_ = runCaptureAllowError(t, "record", "completion-handback", "T-001",
		"--to", "ops",
		"--next-action", "schedule next live window",
		"--state", "review_recorded",
	)
	_ = runCaptureAllowError(t, "record", "completion-handback", "T-001",
		"--to", "ops",
		"--next-action", "schedule next live window",
		"--state", "acknowledged",
	)
	_ = runCaptureAllowError(t, "record", "completion-handback", "T-001",
		"--to", "ops",
		"--next-action", "schedule next live window",
		"--completion-state", "chat-only",
	)
	out := runCapture(t, "record", "completion-handback", "T-001",
		"--to", "ops",
		"--next-action", "schedule next live window",
		"--completion-state", "blocked-with-follow-up",
		"--evidence", "packet.md",
		"--evidence", "rollback-proof.md",
		"--approval-boundary", "no deploy authority",
		"--provider", "codex",
		"--target", "thread-ops",
		"--state", "thread_steered",
	)
	assertContains(t, out, "completion_handback recorded T-001")
	assertContains(t, out, "completion_state=blocked-with-follow-up")
	assertContains(t, out, "delivery_status=delivered")
	assertContains(t, out, "actual_thread_delivery=true")

	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "completion handbacks:")
	assertContains(t, detail, "to=ops")
	assertContains(t, detail, "next_action=schedule next live window")
	assertContains(t, detail, "completion_state=blocked-with-follow-up")
	assertContains(t, detail, "evidence=packet.md,rollback-proof.md")
	assertContains(t, detail, "approval_boundary=no deploy authority")

	jsonDetail := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, jsonDetail, `"completion_handbacks": [`)
	assertContains(t, jsonDetail, `"completion_state": "blocked-with-follow-up"`)
	assertContains(t, jsonDetail, `"delivery_status": "delivered"`)
	assertContains(t, jsonDetail, `"actual_thread_delivery": true`)
}

func TestCLI_TerminalStatusRequiresCompletionHandbackDeliveryDecision(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Delegated closeout", "--role", "backend")
	runOK(t, "record", "completion-handback", "T-001",
		"--to", "ops",
		"--next-action", "decide retry packet",
	)
	_ = runCaptureAllowError(t, "set-status", "T-001", "done")
	assertContains(t, runCapture(t, "task-detail", "T-001"), "status: todo")

	detail := runCapture(t, "--json", "task-detail", "T-001")
	handoffID := jsonIntField(t, detail, "handoff_id")
	runOK(t, "record", "notification", "T-001",
		"--handoff-id", fmt.Sprintf("%d", handoffID),
		"--domain", "ops",
		"--state", "notification_failed",
		"--reason", "thread steering unavailable; coordinator notified out of band",
	)
	runOK(t, "set-status", "T-001", "done")
}

func TestCLI_CompletionHandbackSupersede(t *testing.T) {
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
	replaceInFile(t, ".fairway/config.toml", `notification_ack_timeout = "24h"`, `notification_ack_timeout = "1ns"`)
	runOK(t, "add", "T-001", "--title", "Superseded completion handback", "--role", "backend")
	runOK(t, "set-status", "T-001", "in_progress", "--reason", "active closeout")
	oldJSON := runCapture(t, "--json", "record", "completion-handback", "T-001", "--to", "coordinator", "--next-action", "old next action", "--completion-state", "blocked-with-follow-up")
	oldID := jsonIntField(t, oldJSON, "handoff_id")
	replacementJSON := runCapture(t, "--json", "record", "completion-handback", "T-001", "--to", "coordinator", "--next-action", "replacement next action", "--completion-state", "blocked-with-follow-up")
	replacementID := jsonIntField(t, replacementJSON, "handoff_id")
	runOK(t, "record", "completion-handback-supersede", "T-001", "--handoff-id", fmt.Sprintf("%d", oldID), "--replacement-handoff-id", fmt.Sprintf("%d", replacementID), "--reason", "replacement handback carries current next action", "--evidence", "artifacts/supersede.md")

	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "superseded=true")
	assertContains(t, detail, fmt.Sprintf("replacement_handoff_id=%d", replacementID))
	assertContains(t, detail, "completion-handback-supersede")

	plan := runCapture(t, "coordinator", "plan")
	assertNotContains(t, plan, fmt.Sprintf("completion handback %d", oldID))
	assertContains(t, plan, fmt.Sprintf("completion handback %d", replacementID))

	audit := runCapture(t, "audit", "notifications")
	assertNotContains(t, audit, fmt.Sprintf("handoff_id=%d", oldID))
	assertContains(t, audit, fmt.Sprintf("handoff_id=%d", replacementID))
	allAudit := runCapture(t, "audit", "notifications", "--all")
	assertContains(t, allAudit, fmt.Sprintf("handoff_id=%d", oldID))
	assertContains(t, allAudit, "superseded=true")

	runOK(t, "add", "T-002", "--title", "Unsafe suppression", "--role", "backend")
	runOK(t, "set-status", "T-002", "in_progress", "--reason", "active closeout")
	unsafeJSON := runCapture(t, "--json", "record", "completion-handback", "T-002", "--to", "coordinator", "--next-action", "needs owner")
	unsafeID := jsonIntField(t, unsafeJSON, "handoff_id")
	if _, err := captureRun("record", "completion-handback-supersede", "T-002", "--handoff-id", fmt.Sprintf("%d", unsafeID), "--reason", "hide it"); err == nil || !strings.Contains(err.Error(), "non-terminal completion handback supersede requires --replacement-handoff-id or task status blocked") {
		t.Fatalf("unsafe supersede error = %v", err)
	}

	runOK(t, "set-status", "T-002", "blocked", "--reason", "explicit blocked decision replaces handback")
	runOK(t, "record", "completion-handback-supersede", "T-002", "--handoff-id", fmt.Sprintf("%d", unsafeID), "--reason", "blocked decision recorded")
	blockedDetail := runCapture(t, "task-detail", "T-002")
	assertContains(t, blockedDetail, "superseded=true")

	writeFile(t, "terminal.yaml", `- id: T-003
  title: Terminal cleanup
  role: backend
  status: done
`)
	runOK(t, "import", "terminal.yaml", "--state-once")
	terminalJSON := runCapture(t, "--json", "record", "completion-handback", "T-003", "--to", "coordinator", "--next-action", "historical handback cleanup")
	terminalID := jsonIntField(t, terminalJSON, "handoff_id")
	runOK(t, "record", "completion-handback-supersede", "T-003", "--handoff-id", fmt.Sprintf("%d", terminalID), "--reason", "terminal historical cleanup")
	terminalDetail := runCapture(t, "task-detail", "T-003")
	assertContains(t, terminalDetail, "status: done")
	assertContains(t, terminalDetail, "superseded=true")
}

func TestCLI_CoordinatorTickCompletionHandbackWake(t *testing.T) {
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
	replaceInFile(t, ".fairway/config.toml", `notification_ack_timeout = "24h"`, `notification_ack_timeout = "24h"

[[provider_targets]]
domain = "arch"
provider = "codex"
target = "thread-arch"
type = "thread"`)
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "init")
	for _, taskID := range []string{"T-001", "T-002", "T-003", "T-004", "T-005", "T-006"} {
		runOK(t, "add", taskID, "--title", "Completion handback wake", "--role", "backend")
	}
	runOK(t, "record", "completion-handback", "T-001", "--to", "arch", "--next-action", "decide retry", "--completion-state", "blocked-with-follow-up")
	fresh := runCapture(t, "coordinator", "tick", "--completion-handback-wake", "--task", "T-001")
	assertContains(t, fresh, "completion_handback_wakes: none")

	runOK(t, "record", "completion-handback", "T-002", "--to", "arch", "--next-action", "assign fix", "--completion-state", "blocked-with-follow-up")
	runOK(t, "record", "completion-handback", "T-003", "--to", "product", "--next-action", "assign owner", "--completion-state", "blocked-with-follow-up")
	runOK(t, "live-window", "record", "T-004", "--phase", "closeout", "--next-owner", "arch", "--next-action", "decide next window")
	runOK(t, "set-status", "T-004", "done", "--reason", "closed")
	runOK(t, "live-window", "record", "T-005", "--phase", "closeout", "--next-owner", "arch", "--next-action", "decide next window")
	runOK(t, "set-status", "T-005", "blocked", "--reason", "awaiting control")
	runOK(t, "live-window", "record", "T-006", "--phase", "closeout", "--next-owner", "product", "--next-action", "decide next window")
	runOK(t, "set-status", "T-006", "blocked", "--reason", "awaiting unmapped control")

	replaceInFile(t, ".fairway/config.toml", `notification_ack_timeout = "24h"`, `notification_ack_timeout = "1ns"`)
	time.Sleep(time.Millisecond)

	stale := runCapture(t, "coordinator", "tick", "--completion-handback-wake", "--task", "T-001")
	assertContains(t, stale, "completion_handback_wakes:")
	assertContains(t, stale, "kind=stale-handback")
	assertContains(t, stale, "target=thread-arch")
	assertContains(t, stale, "Completion handback wake for T-001:")
	assertContains(t, stale, "Do not treat this wake as approval, merge, deploy, or dashboard send authority.")
	_ = runCaptureAllowError(t, "coordinator", "plan", "--completion-handback-wake", "--task", "T-001", "--send")
	assertNotContains(t, runCapture(t, "task-detail", "T-001"), "completion_handback_wake signature=T-001|stale-handback|")

	sent := runCapture(t, "coordinator", "tick", "--completion-handback-wake", "--task", "T-001", "--send", "--state", "thread_steered")
	assertContains(t, sent, "kind=stale-handback")
	assertContains(t, sent, "signature=T-001|stale-handback|")
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "thread_steered domain=coordinator provider=codex target=thread-arch")
	assertContains(t, detail, "completion_handback_wake signature=T-001|stale-handback|")

	suppressed := runCapture(t, "coordinator", "tick", "--completion-handback-wake", "--task", "T-001", "--send", "--state", "thread_steered")
	assertContains(t, suppressed, "status=suppressed")

	failed := runCapture(t, "coordinator", "tick", "--completion-handback-wake", "--task", "T-003", "--send")
	assertContains(t, failed, "kind=stale-handback")
	assertContains(t, failed, "status=failed")
	assertContains(t, failed, "target_action=mapping_required")
	assertContains(t, failed, "Wake target: mapping_required")
	failedDetail := runCapture(t, "task-detail", "T-003")
	assertContains(t, failedDetail, "notification_failed domain=coordinator")
	assertContains(t, failedDetail, "failed=no_wake_target")
	assertContains(t, failedDetail, "action=mapping_required")
	preflight := runCaptureAllowError(t, "coordinator", "preflight")
	assertContains(t, preflight, "wait completion_handback:T-006:escalate_closeout_completion_handback kind completion_handback owner product is not wake-routable")
	assertContains(t, preflight, "action=mapping_required")

	closed := runCapture(t, "coordinator", "tick", "--completion-handback-wake", "--task", "T-004")
	assertContains(t, closed, "completion_handback_wakes: none")

	closeout := runCapture(t, "coordinator", "tick", "--completion-handback-wake", "--task", "T-005")
	assertContains(t, closeout, "kind=stale-closeout")
	assertContains(t, closeout, "Live-window phase: closeout")
	assertContains(t, closeout, "target=thread-arch")

	unmappedCloseout := runCapture(t, "coordinator", "tick", "--completion-handback-wake", "--task", "T-006", "--send")
	assertContains(t, unmappedCloseout, "kind=stale-closeout")
	assertContains(t, unmappedCloseout, "status=failed")
	assertContains(t, unmappedCloseout, "target_action=mapping_required")
	assertContains(t, unmappedCloseout, "Wake target: mapping_required")
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
	runOK(t, "add", "CI-001", "--title", "API CI monitor", "--role", "ops", "--kind", "ci-monitor", "--owning-domain", "platform")
	runOK(t, "add", "CI-002", "--title", "Frontend CI monitor", "--role", "ops", "--kind", "ci-monitor", "--owning-domain", "platform")
	runOK(t, "set-status", "T-002", "done")
	runOK(t, "set-status", "CI-001", "done")
	runOK(t, "set-status", "CI-002", "done")
	replaceInFile(t, ".fairway/config.toml", "require_evidence_before_done = false", "require_evidence_before_done = true")
	runOK(t, "record", "evidence", "T-003", "--command-text", "go test ./...", "--result", "pass")
	runOK(t, "record", "evidence", "CI-001", "--command-text", "gh run watch api", "--result", "pass", "--artifact-type", "ci_pipeline", "--artifact", "gh-run-api")
	runOK(t, "record", "evidence", "CI-002", "--command-text", "gh run watch frontend", "--result", "pass", "--artifact-type", "ci_pipeline", "--artifact", "gh-run-frontend")

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
		"work_batch_candidate",
		"related_tasks: CI-001, CI-002",
		"docs/plan.md",
		"T-002",
		"T-003",
	} {
		assertContains(t, report, want)
	}

	jsonReport := runCapture(t, "--json", "audit", "work-coverage", "--since-ref", baseRef)
	assertContains(t, jsonReport, `"kind": "commit_without_task_coverage"`)
	assertContains(t, jsonReport, `"done_without_required_evidence": 1`)
	assertContains(t, jsonReport, `"work_batch_candidates": 1`)
	assertContains(t, jsonReport, `"related_tasks": [`)
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

func TestCLI_AuditDocsBacklog(t *testing.T) {
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
	runOK(t, "add", "FW-196", "--title", "Add first-class track memory packets", "--role", "backend", "--source-paths", "docs/design/coordination-intelligence.md")
	runOK(t, "set-status", "FW-196", "done")
	writeFile(t, "docs/design/coordination-intelligence.md", "FW-196 implements track memory.\n`fairway memory packet --track architecture-control`\n")
	writeFile(t, "docs/design/uncovered-coordination.md", "Review waits need an operator command.\n`fairway review-waits list --blocking`\n")

	report := runCapture(t, "audit", "docs-backlog", "--doc", "docs/design/coordination-intelligence.md", "--doc", "docs/design/uncovered-coordination.md")
	for _, want := range []string{
		"docs_backlog_ok: false",
		"docs_scanned=2",
		"docs_with_backlog_coverage=1",
		"doc_only_capability",
		"command_example_uncovered",
		"docs/design/uncovered-coordination.md",
		"related_tasks: FW-179, FW-180, FW-181, FW-182, FW-183, FW-184",
	} {
		assertContains(t, report, want)
	}

	jsonReport := runCapture(t, "--json", "audit", "docs-backlog", "--doc", "docs/design/coordination-intelligence.md")
	assertContains(t, jsonReport, `"docs_with_backlog_coverage": 1`)
	assertContains(t, jsonReport, `"covering_tasks": [`)
	assertContains(t, jsonReport, `"FW-196"`)
}

func TestCLI_DeliveryReport(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Reviewed feature", "--role", "backend", "--profile", "fairway-adoption", "--review-domains", "arch")
	runOK(t, "claim", "T-001")
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass", "--artifact-type", "ci")
	runOK(t, "record", "review", "T-001", "--reviewer", "arch", "--domain", "arch", "--verdict", "changes", "--reason", "caught regression")
	runOK(t, "set-status", "T-001", "done", "--reason", "closed after review")

	runOK(t, "add", "T-002", "--title", "Looping task", "--role", "backend", "--profile", "fairway-adoption")
	runOK(t, "claim", "T-002")
	runOK(t, "record", "evidence", "T-002", "--command-text", "preflight failed", "--result", "fail", "--artifact-type", "preflight")
	runOK(t, "record", "evidence", "T-002", "--command-text", "preflight failed again", "--result", "blocked", "--artifact-type", "preflight")
	runOK(t, "record", "review", "T-002", "--reviewer", "arch", "--domain", "arch", "--verdict", "changes", "--reason", "same layer failure")

	report := runCapture(t, "delivery", "report", "--since", "720h", "--profile", "fairway-adoption")
	for _, want := range []string{
		"delivery_report_ok: true",
		"profile: fairway-adoption",
		"completed=1",
		"reviews=2",
		"changes_requested=2",
		"outcome_sources:",
		"- review=2",
		"loop_signals:",
		"task=T-002",
		"repeated_failures_after_review",
	} {
		assertContains(t, report, want)
	}

	jsonReport := runCapture(t, "delivery", "report", "--since", "720h", "--format", "json")
	assertContains(t, jsonReport, `"completed_tasks": 1`)
	assertContains(t, jsonReport, `"review_changes_requested": 2`)
	assertContains(t, jsonReport, `"signal": "repeated_failures_after_review"`)
}

func TestCLI_AutomationCandidates(t *testing.T) {
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
	for _, id := range []string{"T-001", "T-002", "T-003"} {
		runOK(t, "add", id, "--title", "Repeated work "+id, "--role", "backend")
		runOK(t, "record", "evidence", id, "--command-text", "fairway merge-ready "+id, "--result", "blocked", "--artifact-type", "merge-ready")
		runOK(t, "record", "notification", id, "--domain", "backend", "--provider", "codex", "--target", "thread-"+id, "--state", "thread_steered")
	}

	report := runCapture(t, "automation", "candidates", "--since", "720h", "--threshold", "3")
	for _, want := range []string{
		"automation_candidates_ok: true",
		"threshold: 3",
		"kind=command",
		"pattern=fairway merge-ready <task>",
		"surface=fairway cli",
		"recent_tasks: T-003, T-002, T-001",
		"kind=notification",
		"pattern=backend:thread_steered",
	} {
		assertContains(t, report, want)
	}

	jsonReport := runCapture(t, "automation", "candidates", "--since", "720h", "--threshold", "3", "--format", "json")
	assertContains(t, jsonReport, `"pattern": "fairway merge-ready \u003ctask\u003e"`)
	assertContains(t, jsonReport, `"suggested_surface": "fairway cli"`)
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

func TestCLI_KnownFailureRoutingRecommendations(t *testing.T) {
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
	cases := []struct {
		id       string
		title    string
		role     string
		layer    string
		command  string
		artifact string
		notes    string
		class    string
		prefix   string
		kind     string
	}{
		{"ART-001", "Artifact mismatch", "backend", "artifact-contract", "validate artifact schema", "artifacts/result.json", "artifact mismatch schema mismatch", "artifact_contract", "HARNESS-FIX", "bugfix"},
		{"PROV-001", "Provider 4xx", "ops", "provider-api", "provider API proof", "artifacts/provider.md", "provider 4xx 403 unknown provider behavior", "provider_api", "OPS-FIX", "proof"},
		{"BROW-001", "Browser surface", "backend", "browser", "browser smoke", "artifacts/browser.md", "browser launch permission failure playwright", "browser_surface", "HARNESS-FIX", "readiness"},
		{"SETUP-001", "Setup gate", "ops", "setup", "setup gate", "artifacts/setup.md", "setup gate failed readback failed", "setup_gate", "OPS-FIX", "task"},
		{"CALL-001", "Callback missing", "backend", "callback", "browser flow smoke", "artifacts/callback.md", "callback missing redirect missing", "callback_missing", "UAT-BUG", "bug"},
		{"RED-001", "Redaction finding", "ops", "redaction", "redaction self-test", "artifacts/redaction.md", "redaction finding unredacted token leak", "redaction_finding", "OPS-FIX", "guard"},
		{"COMMIT-001", "Commit boundary", "backend", "commit-boundary", "merge-ready", "artifacts/merge-ready.md", "uncommitted reviewed files merge-ready dirty", "commit_boundary", "OPS-FIX", "task"},
		{"HAND-001", "Undelivered handoff", "ops", "wait-wake", "handoff check", "artifacts/handoff.md", "review handoff not delivered missing notification delivery", "undelivered_handoff", "OPS-FIX", "task"},
	}
	for _, tc := range cases {
		runOK(t, "add", tc.id, "--title", tc.title, "--role", tc.role, "--owning-layer", tc.layer)
		runOK(t, "record", "evidence", tc.id, "--command-text", tc.command, "--result", "fail", "--artifact", tc.artifact, "--artifact-type", "smoke", "--notes", tc.notes)
	}

	report := runCapture(t, "audit", "failure-routing", "--template")
	for _, want := range []string{
		"failure_routing_ok: false",
		"artifact_contract=1",
		"provider_api=1",
		"browser_surface=1",
		"setup_gate=1",
		"callback_missing=1",
		"redaction_finding=1",
		"commit_boundary=1",
		"undelivered_handoff=1",
		"artifact: artifacts/provider.md",
		"forbidden_until_reviewed: live execution, production mutation, credential action, approval acceptance, merge/deploy",
		"# CI/Deploy Learning: ART-001",
		"Follow-up task kind: bugfix",
		"Forbidden until reviewed: live execution, production mutation, credential action, approval acceptance, merge/deploy",
	} {
		assertContains(t, report, want)
	}
	for _, tc := range cases {
		assertContains(t, report, tc.class)
		assertContains(t, report, fmt.Sprintf("suggested %s-%s kind=%s", tc.prefix, tc.id, tc.kind))
	}
	help := runCapture(t, "audit", "failure-routing", "--help")
	assertContains(t, help, "fairway audit failure-routing [--task-id <task-id>] [--template]")
	assertContains(t, help, "advisory known-failure routing recommendations")
	assertNotContains(t, help, "Usage of audit ci-learning")
	assertNotContains(t, help, "error:")

	jsonReport := runCapture(t, "--json", "audit", "failure-routing")
	for _, want := range []string{
		`"failure_class": "provider_api"`,
		`"recommended_follow_up_prefix": "OPS-FIX"`,
		`"recommended_follow_up_task_kind": "proof"`,
		`"owning_layer": "provider-api"`,
		`"forbidden_actions": [`,
		`"live execution"`,
	} {
		assertContains(t, jsonReport, want)
	}
}

func TestCLI_NotificationLifecycleAudit(t *testing.T) {
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
	replaceInFile(t, ".fairway/config.toml", `notification_ack_timeout = "24h"`, `notification_ack_timeout = "1ns"`)
	appendFile(t, ".fairway/config.toml", `
[[roles]]
name = "backend"

[[roles]]
name = "security"
provider = "codex"

[[roles]]
name = "coordinator"
provider = "codex"

[[provider_targets]]
domain = "security"
provider = "codex"
target = "thread-security"
type = "thread"

[[provider_targets]]
domain = "coordinator"
provider = "codex"
target = "thread-control"
type = "thread"
`)
	for _, taskID := range []string{"STALE-001", "FAIL-001", "RESOLVED-001", "DONE-001", "HAND-001"} {
		runOK(t, "add", taskID, "--title", "Notification audit fixture", "--role", "backend", "--review-domains", "security")
		runOK(t, "set-status", taskID, "in_progress", "--reason", "audit fixture active")
	}
	runOK(t, "record", "notification", "STALE-001", "--domain", "security", "--provider", "codex", "--target", "thread-security", "--state", "sent", "--reason", "sent but no ack")
	runOK(t, "record", "notification", "FAIL-001", "--domain", "security", "--state", "notification_failed", "--reason", "thread target missing")
	runOK(t, "record", "notification", "RESOLVED-001", "--domain", "security", "--provider", "codex", "--target", "thread-security", "--state", "thread_steered", "--reason", "delivered")
	runOK(t, "record", "review", "RESOLVED-001", "--reviewer", "security-reviewer", "--domain", "security", "--verdict", "approve", "--reason", "clean notification")
	runOK(t, "record", "notification", "DONE-001", "--domain", "security", "--provider", "codex", "--target", "thread-security", "--state", "sent", "--reason", "closed task notification")
	runOK(t, "set-status", "DONE-001", "done", "--reason", "terminal tasks are suppressed by default")
	runOK(t, "record", "completion-handback", "HAND-001", "--to", "coordinator", "--next-action", "decide retry packet", "--completion-state", "blocked-with-follow-up")

	time.Sleep(time.Millisecond)
	out := runCapture(t, "audit", "notifications")
	for _, want := range []string{
		"notification_audit:",
		"task=STALE-001 source=review_wait domain=security state=stale",
		"action=escalate_or_record_notification_outcome",
		"provider=codex target=thread-security",
		"task=FAIL-001 source=review_wait domain=security state=notification_failed",
		"reason=thread target missing",
		"task=HAND-001 source=completion_handback domain=coordinator state=stale",
		"suggested_command: fairway record notification",
	} {
		assertContains(t, out, want)
	}
	assertNotContains(t, out, "RESOLVED-001")
	assertNotContains(t, out, "DONE-001")

	allOut := runCapture(t, "audit", "notifications", "--all")
	assertContains(t, allOut, "task=RESOLVED-001")
	assertContains(t, allOut, "state=review_recorded")
	assertContains(t, allOut, "task=DONE-001")
	assertContains(t, allOut, "terminal=true")
	assertContains(t, allOut, "superseded=true")

	filtered := runCapture(t, "audit", "notifications", "--task", "FAIL-001")
	assertContains(t, filtered, "task=FAIL-001")
	assertNotContains(t, filtered, "STALE-001")

	jsonOut := runCapture(t, "--json", "audit", "notifications")
	for _, want := range []string{
		`"source": "review_wait"`,
		`"state": "notification_failed"`,
		`"source": "completion_handback"`,
		`"last_notification_id":`,
		`"suggested_command": "fairway record notification`,
	} {
		assertContains(t, jsonOut, want)
	}
}

func TestCLI_NotificationLifecycleAuditKeepsUnresolvedCompletionAcknowledgement(t *testing.T) {
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
	runOK(t, "add", "ACKH-001", "--title", "Acknowledged but unresolved completion handback", "--role", "backend")
	runOK(t, "set-status", "ACKH-001", "in_progress", "--reason", "audit fixture active")
	runOK(t, "record", "completion-handback", "ACKH-001", "--to", "coordinator", "--next-action", "decide next owner", "--completion-state", "blocked-with-follow-up")
	detail := runCapture(t, "--json", "task-detail", "ACKH-001")
	handoffID := jsonIntField(t, detail, "handoff_id")
	runOK(t, "record", "notification", "ACKH-001", "--handoff-id", fmt.Sprintf("%d", handoffID), "--domain", "coordinator", "--state", "acknowledged", "--reason", "control saw the handback but no delivery proof recorded")

	out := runCapture(t, "audit", "notifications")
	assertContains(t, out, "task=ACKH-001 source=completion_handback domain=coordinator state=acknowledged")
	assertContains(t, out, "action=deliver_or_record_completion_handback")
	assertContains(t, out, "superseded=false")
	assertContains(t, out, "reason=control saw the handback but no delivery proof recorded")
}

func TestCLI_AdvisoryRecommendationValidate(t *testing.T) {
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

[[provider_targets]]
domain = "backend"
provider = "codex-thread"
target = "thread-backend"
type = "thread"

[[advisory_provider_adapters]]
name = "local-rules"
provider = "ollama"
type = "local_ollama"
mode = "advisory"
trust = "low"
model = "llama3.1"
endpoint_env = "FAIRWAY_OLLAMA_ENDPOINT"
capabilities = ["summarize_evidence", "rank_ready_tasks"]
allowed_actions = ["inspect_task", "render_packet", "wake_provider"]

[[advisory_provider_adapters]]
name = "disabled-provider"
provider = "codex"
type = "codex"
mode = "disabled"
allowed_actions = ["inspect_task"]
`)
	runOK(t, "config", "validate")
	runOK(t, "add", "T-001", "--title", "Advisory target", "--role", "backend")

	adapters := runCapture(t, "advisory", "adapters")
	assertContains(t, adapters, "advisory_adapters:")
	assertContains(t, adapters, "local-rules provider=ollama type=local_ollama mode=advisory trust=low")
	assertContains(t, adapters, "endpoint_env=FAIRWAY_OLLAMA_ENDPOINT")
	assertContains(t, adapters, "summarize_evidence")
	assertNotContains(t, adapters, "disabled-provider")

	allAdapters := runCapture(t, "advisory", "adapters", "--include-disabled")
	assertContains(t, allAdapters, "disabled-provider provider=codex type=codex mode=disabled")

	help := runCapture(t, "advisory", "validate", "--help")
	assertContains(t, help, "fairway advisory validate <task-id> --action <action>")
	assertContains(t, help, "advisory evidence only")
	assertNotContains(t, help, "error:")

	report := runCapture(t, "advisory", "validate", "T-001",
		"--provider", "local-rules",
		"--action", "wake_provider",
		"--target-role", "backend",
		"--confidence", "0.82",
		"--requires-human",
		"--risk-flag", "approval",
		"--rationale", "Reviewer should inspect task facts before action.",
		"--cited-fact", "task:T-001 status=todo",
		"--record-evidence")
	for _, want := range []string{
		"advisory_valid: true",
		"provider: local-rules",
		"action: wake_provider",
		"target_role: backend",
		"requires_human: true",
		"Risk Flags",
		"Cited Fairway Facts",
		"recorded: advisory-recommendation evidence",
	} {
		assertContains(t, report, want)
	}
	detail := runCapture(t, "--json", "task-detail", "T-001")
	assertContains(t, detail, "advisory-recommendation")
	assertContains(t, detail, "wake_provider")

	jsonReport := runCapture(t, "--json", "advisory", "validate", "T-001",
		"--provider", "local-rules",
		"--action", "render_packet",
		"--target-role", "backend",
		"--confidence", "0.5",
		"--rationale", "Render bounded context only.",
		"--cited-fact", "task:T-001 status=todo")
	assertContains(t, jsonReport, `"ok": true`)
	assertContains(t, jsonReport, `"provider": "local-rules"`)
	assertContains(t, jsonReport, `"action": "render_packet"`)

	disallowed := runCaptureAllowError(t, "advisory", "validate", "T-001",
		"--provider", "local-rules",
		"--action", "route_review",
		"--target-role", "backend",
		"--confidence", "0.5",
		"--rationale", "Adapter is not allowed to route review.",
		"--cited-fact", "task:T-001 status=todo")
	assertContains(t, disallowed, "configured advisory provider adapter does not allow action: route_review")

	disabled := runCaptureAllowError(t, "advisory", "validate", "T-001",
		"--provider", "disabled-provider",
		"--action", "inspect_task",
		"--target-role", "backend",
		"--confidence", "0.5",
		"--rationale", "Disabled provider should not be accepted.",
		"--cited-fact", "task:T-001 status=todo")
	assertContains(t, disabled, "configured advisory provider adapter is disabled: disabled-provider")

	invalid := runCaptureAllowError(t, "advisory", "validate", "T-001",
		"--action", "approve_review",
		"--target-role", "backend",
		"--confidence", "1.2",
		"--risk-flag", "deploy",
		"--rationale", "Unsafe action",
		"--cited-fact", "chat:T-001 said ok")
	assertContains(t, invalid, "advisory_valid: false")
	assertContains(t, invalid, "action is not in the advisory allowed-action enum")
	assertContains(t, invalid, "--confidence must be between 0 and 1")
	assertContains(t, invalid, "risk flags require --requires-human")
	assertContains(t, invalid, "cited fact must name an existing Fairway fact prefix")

	runOK(t, "add", "T-002", "--title", "Done advisory target", "--role", "backend")
	runOK(t, "set-status", "T-002", "done")
	doneRoute := runCaptureAllowError(t, "advisory", "validate", "T-002",
		"--action", "route_review",
		"--target-role", "backend",
		"--confidence", "0.8",
		"--rationale", "Route a review",
		"--cited-fact", "task:T-002 status=done")
	assertContains(t, doneRoute, "action is not applicable to task status done")

	runOK(t, "add", "T-003", "--title", "Reviewed advisory target", "--role", "backend", "--review-domains", "backend")
	runOK(t, "record", "review", "T-003", "--reviewer", "fairway-reviewer", "--domain", "backend", "--verdict", "approve", "--reason", "already reviewed")
	reviewedRoute := runCaptureAllowError(t, "advisory", "validate", "T-003",
		"--action", "route_review",
		"--target-role", "backend",
		"--confidence", "0.8",
		"--rationale", "Route a review",
		"--cited-fact", "task:T-003 review=approved")
	assertContains(t, reviewedRoute, "route_review is not applicable because required review domains are already approved")

	warning := runCapture(t, "advisory", "validate", "T-001",
		"--action", "wake_provider",
		"--target-role", "ops",
		"--confidence", "0.7",
		"--rationale", "Ops should be woken if a target exists.",
		"--cited-fact", "task:T-001 status=todo")
	assertContains(t, warning, "advisory_valid: true")
	assertContains(t, warning, "target role has no configured provider target")
}

func TestCLI_NotifyDryRunExternalNotifier(t *testing.T) {
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
[[external_notifiers]]
name = "control-log"
type = "log"
mode = "dry_run"
target_env = "FAIRWAY_NOTIFY_LOG"
domains = ["coordinator", "ops"]
template_name = "control_room_handoff"

[[external_notifiers]]
name = "disabled-hook"
type = "noop"
mode = "disabled"

[[external_notifiers]]
name = "ops-log"
type = "log"
mode = "send"
target_env = "FAIRWAY_NOTIFY_LOG_PATH"
domains = ["ops"]
template_name = "ops_handoff"
rate_limit_per_minute = 5
`)
	runOK(t, "config", "validate")
	runOK(t, "add", "T-001", "--title", "Notify target", "--role", "backend")

	notifiers := runCapture(t, "notify", "notifiers")
	assertContains(t, notifiers, "external_notifiers:")
	assertContains(t, notifiers, "control-log type=log mode=dry_run")
	assertContains(t, notifiers, "target_env=FAIRWAY_NOTIFY_LOG")
	assertContains(t, notifiers, "control_room_handoff")
	assertNotContains(t, notifiers, "disabled-hook")

	allNotifiers := runCapture(t, "notify", "notifiers", "--include-disabled")
	assertContains(t, allNotifiers, "disabled-hook type=noop mode=disabled")
	assertContains(t, allNotifiers, "ops-log type=log mode=send")
	assertContains(t, allNotifiers, "rate_limit_per_minute=5")

	dryRun := runCapture(t, "notify", "dry-run",
		"--notifier", "control-log",
		"--task", "T-001",
		"--domain", "coordinator")
	assertContains(t, dryRun, "notify_dry_run: true")
	assertContains(t, dryRun, "warning: dry-run/log notifier does not prove external delivery")
	assertContains(t, dryRun, "template: control_room_handoff")
	assertNotContains(t, dryRun, "recorded_state:")

	recorded := runCapture(t, "notify", "dry-run",
		"--notifier", "control-log",
		"--task", "T-001",
		"--domain", "coordinator",
		"--record-intent")
	assertContains(t, recorded, "recorded_state: intent")
	assertContains(t, recorded, "external_notifier_intent")
	assertContains(t, recorded, "template=control_room_handoff")
	assertNotContains(t, recorded, "operator handback ready")

	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "intent domain=coordinator provider=external-notifier/control-log")
	assertContains(t, detail, "template=control_room_handoff")
	assertNotContains(t, detail, "notification_delivered domain=coordinator")

	_, err = captureRun("notify", "dry-run",
		"--notifier", "disabled-hook",
		"--task", "T-001",
		"--domain", "coordinator")
	if err == nil || !strings.Contains(err.Error(), `external notifier "disabled-hook" is disabled`) {
		t.Fatalf("disabled notifier error = %v", err)
	}

	_, err = captureRun("notify", "dry-run",
		"--notifier", "control-log",
		"--task", "T-001",
		"--domain", "security")
	if err == nil || !strings.Contains(err.Error(), `does not allow domain "security"`) {
		t.Fatalf("wrong domain error = %v", err)
	}

	logPath := filepath.Join(repo, "notify.log")
	t.Setenv("FAIRWAY_NOTIFY_LOG_PATH", logPath)
	sent := runCapture(t, "notify", "send",
		"--notifier", "ops-log",
		"--task", "T-001",
		"--domain", "ops")
	assertContains(t, sent, "notify_send: true")
	assertContains(t, sent, "attempt_state: sent")
	assertContains(t, sent, "delivery_state: notification_delivered")
	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"notifier":"ops-log"`, `"task_id":"T-001"`, `"domain":"ops"`, `"template":"ops_handoff"`} {
		if !strings.Contains(string(logBody), want) {
			t.Fatalf("log notifier body missing %q:\n%s", want, string(logBody))
		}
	}
	detail = runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "sent domain=ops provider=external-notifier/ops-log")
	assertContains(t, detail, "notification_delivered domain=ops provider=external-notifier/ops-log")
}

func TestCLI_NotifySendWebhookExternalNotifier(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Webhook notify target", "--role", "backend")
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization header=%q", got)
		}
		raw := new(bytes.Buffer)
		if _, err := raw.ReadFrom(r.Body); err != nil {
			t.Fatal(err)
		}
		gotBody = raw.String()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	t.Setenv("FAIRWAY_NOTIFY_WEBHOOK", server.URL)
	t.Setenv("FAIRWAY_NOTIFY_TOKEN", "test-token")
	appendFile(t, ".fairway/config.toml", `
[[external_notifiers]]
name = "control-webhook"
type = "webhook"
mode = "send"
target_env = "FAIRWAY_NOTIFY_WEBHOOK"
token_env = "FAIRWAY_NOTIFY_TOKEN"
domains = ["coordinator"]
template_name = "control_room_handoff"
rate_limit_per_minute = 2
`)
	runOK(t, "config", "validate")
	sent := runCapture(t, "notify", "send",
		"--notifier", "control-webhook",
		"--task", "T-001",
		"--domain", "coordinator")
	assertContains(t, sent, "delivery_state: notification_delivered")
	assertContains(t, gotBody, `"notifier":"control-webhook"`)
	assertContains(t, gotBody, `"template":"control_room_handoff"`)
	assertNotContains(t, sent, server.URL)
	assertNotContains(t, runCapture(t, "task-detail", "T-001"), server.URL)

	second := runCapture(t, "notify", "send",
		"--notifier", "control-webhook",
		"--task", "T-001",
		"--domain", "coordinator",
		"--target", "control-room")
	assertContains(t, second, "delivery_state: notification_delivered")
	assertContains(t, second, "target: control-room")

	_, err = captureRun("notify", "send",
		"--notifier", "control-webhook",
		"--task", "T-001",
		"--domain", "coordinator")
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("rate limit error=%v", err)
	}
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "notification_failed domain=coordinator provider=external-notifier/control-webhook")
	assertContains(t, detail, "external_notifier_rate_limited")
	assertNotContains(t, detail, "test-token")

	_, err = captureRun("notify", "send",
		"--notifier", "control-webhook",
		"--task", "T-001",
		"--domain", "coordinator",
		"--target", server.URL)
	if err == nil || !strings.Contains(err.Error(), "--target must be a safe label") {
		t.Fatalf("unsafe target error=%v", err)
	}
	assertNotContains(t, runCapture(t, "task-detail", "T-001"), server.URL)
}

func TestCLI_NotifySendWebhookRecordsFailure(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Webhook notify failure", "--role", "backend")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	t.Setenv("FAIRWAY_NOTIFY_WEBHOOK_FAIL", server.URL)
	appendFile(t, ".fairway/config.toml", `
[[external_notifiers]]
name = "failing-webhook"
type = "webhook"
mode = "send"
target_env = "FAIRWAY_NOTIFY_WEBHOOK_FAIL"
domains = ["ops"]
template_name = "ops_handoff"
`)
	runOK(t, "config", "validate")
	_, err = captureRun("notify", "send",
		"--notifier", "failing-webhook",
		"--task", "T-001",
		"--domain", "ops")
	if err == nil || !strings.Contains(err.Error(), "delivery failed") {
		t.Fatalf("failure error=%v", err)
	}
	detail := runCapture(t, "task-detail", "T-001")
	assertContains(t, detail, "sent domain=ops provider=external-notifier/failing-webhook")
	assertContains(t, detail, "notification_failed domain=ops provider=external-notifier/failing-webhook")
	assertContains(t, detail, "webhook_returned_status_502")
	assertNotContains(t, detail, server.URL)
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
	writeFile(t, "artifacts/provenance-v0.1.2.json", `{"schema":"fairway.provenance.v1","release":"v0.1.2","source_sha":"abc123","tasks":["FW-232"]}`)

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
		"--provenance-bundle", "artifacts/provenance-v0.1.2.json",
		"--verification-command", "brew fetch --cask --force fairway-run/tap/fairway",
	}
	runOK(t, cleanArgs...)
	cleanJSON := runCapture(t, append([]string{"--json"}, cleanArgs...)...)
	assertContains(t, cleanJSON, `"ok": true`)
	assertContains(t, cleanJSON, `"provenance_bundle": "artifacts/provenance-v0.1.2.json"`)

	homebrewNormalizedArgs := append([]string{}, cleanArgs...)
	for i := range homebrewNormalizedArgs {
		if homebrewNormalizedArgs[i] == "v0.1.2" && i > 0 && homebrewNormalizedArgs[i-1] == "--homebrew-version" {
			homebrewNormalizedArgs[i] = "0.1.2"
			break
		}
	}
	runOK(t, homebrewNormalizedArgs...)

	homebrewMismatchArgs := append([]string{}, cleanArgs...)
	for i := range homebrewMismatchArgs {
		if homebrewMismatchArgs[i] == "v0.1.2" && i > 0 && homebrewMismatchArgs[i-1] == "--homebrew-version" {
			homebrewMismatchArgs[i] = "0.1.3"
			break
		}
	}
	homebrewMismatch := runCaptureAllowError(t, homebrewMismatchArgs...)
	assertContains(t, homebrewMismatch, `Homebrew cask version "0.1.3" does not match release version "v0.1.2"`)

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

	missingProvenanceArgs := append([]string{}, cleanArgs...)
	for i := range missingProvenanceArgs {
		if missingProvenanceArgs[i] == "artifacts/provenance-v0.1.2.json" && i > 0 && missingProvenanceArgs[i-1] == "--provenance-bundle" {
			missingProvenanceArgs[i] = "artifacts/missing-provenance.json"
			break
		}
	}
	missingProvenance := runCaptureAllowError(t, missingProvenanceArgs...)
	assertContains(t, missingProvenance, "missing provenance bundle at artifacts/missing-provenance.json")

	failedAsset := runCaptureAllowError(t, "release", "verify", "--version", "v0.1.2", "--tag", "v0.1.2", "--ci-status", "pass", "--docs-status", "pass", "--signing-status", "pass", "--notary-status", "pass", "--release-state", "public", "--asset", "https://github.com/fairway-run/fairway/releases/download/v0.1.2/fairway.tar.gz=404", "--homebrew-version", "v0.1.2", "--homebrew-tap-commit", "tap123", "--brew-fetch-status", "pass")
	assertContains(t, failedAsset, "asset URL failed")
	assertContains(t, failedAsset, "status=404")
}

func TestCLI_PacketRetry(t *testing.T) {
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
	runOK(t, "add", "T-001", "--title", "Retry drill", "--role", "backend")
	runOK(t, "record", "evidence", "T-001", "--command-text", "prior smoke failed", "--result", "fail", "--artifact", ".fairway/artifacts/T-001/prior.md")
	runOK(t, "record", "review", "T-001", "--reviewer", "fairway-reviewer", "--domain", "ops", "--verdict", "approve", "--reason", "retry packet reviewed")

	help := runCapture(t, "packet", "retry", "--help")
	assertContains(t, help, "fairway packet retry <task-id> --kind <preflight|live-operation>")
	assertContains(t, help, "packet rendering is not execution authorization")
	assertNotContains(t, help, "error:")
	assertNotContains(t, help, "Usage of")

	packet := runCapture(t, "packet", "retry", "T-001",
		"--kind", "live-operation",
		"--source-sha", "abc1234",
		"--operator-surface", "drill-operator-v2",
		"--artifact-dir", ".fairway/artifacts/T-001/retry-002",
		"--evidence-contract", "flow map before implementation",
		"--evidence-contract", "rollback readback",
		"--allowed-action", "run bounded non-prod preflight",
		"--forbidden-action", "production mutation",
		"--forbidden-action", "credential action",
		"--expires-at", "2026-06-14T20:00:00-05:00",
		"--prior-failure-closure", "HARNESS-FIX-T-001 merged and smoke passed",
		"--next-action", "run fairway workflow check before retry")
	for _, want := range []string{
		"# Retry Packet: T-001",
		"kind: live-operation",
		"source_sha: abc1234",
		"operator_surface: drill-operator-v2",
		"packet rendering only; this is not execution authorization",
		"flow map before implementation",
		"rollback readback",
		"run bounded non-prod preflight",
		"production mutation",
		"credential action",
		"HARNESS-FIX-T-001 merged and smoke passed",
		"run fairway workflow check before retry",
		"prior smoke failed",
		"approve by fairway-reviewer: retry packet reviewed",
	} {
		assertContains(t, packet, want)
	}

	jsonPacket := runCapture(t, "--json", "packet", "retry", "T-001",
		"--kind", "preflight",
		"--source-sha", "def5678",
		"--operator-surface", "local-shell",
		"--artifact-dir", ".fairway/artifacts/T-001/preflight",
		"--evidence-contract", "preflight output",
		"--allowed-action", "run non-live smoke",
		"--forbidden-action", "live execution",
		"--expires-at", "2026-06-14T21:00:00-05:00",
		"--prior-failure-closure", "prior failure acknowledged")
	for _, want := range []string{
		`"kind": "preflight"`,
		`"source_sha": "def5678"`,
		`"operator_surface": "local-shell"`,
		`"authorization": "packet rendering only; this is not execution authorization; no hidden approval is granted by this packet"`,
		`"evidence_contract": [`,
	} {
		assertContains(t, jsonPacket, want)
	}

	err = run(context.Background(), []string{"packet", "retry", "T-001",
		"--source-sha", "abc1234",
		"--operator-surface", "local-shell"})
	if err == nil {
		t.Fatal("expected missing packet retry fields error")
	}
	missing := err.Error()
	for _, want := range []string{"packet retry requires", "--artifact-dir", "--evidence-contract", "--allowed-action", "--forbidden-action"} {
		if !strings.Contains(missing, want) {
			t.Fatalf("missing fields error = %q, want %q", missing, want)
		}
	}

	err = run(context.Background(), []string{"packet", "retry", "T-001",
		"--kind", "drill",
		"--source-sha", "abc1234",
		"--operator-surface", "local-shell",
		"--artifact-dir", ".fairway/artifacts/T-001",
		"--evidence-contract", "preflight output",
		"--allowed-action", "run smoke",
		"--forbidden-action", "live execution",
		"--expires-at", "2026-06-14T21:00:00-05:00",
		"--prior-failure-closure", "prior failure acknowledged"})
	if err == nil {
		t.Fatal("expected invalid retry packet kind error")
	}
	if !strings.Contains(err.Error(), "packet retry --kind must be preflight or live-operation") {
		t.Fatalf("bad kind error = %q", err.Error())
	}
}

func TestCLI_LiveWindowRetryBudgetGuardsRetryPacket(t *testing.T) {
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
	runOK(t, "add", "LIVE-001", "--title", "Retry budget drill", "--role", "backend")

	help := runCapture(t, "live-window", "retry-budget", "--help")
	assertContains(t, help, "retry-budget record <task-id>")
	assertContains(t, help, "meaningful-failures")
	statusHelp := runCapture(t, "live-window", "retry-budget", "status", "--help")
	assertContains(t, statusHelp, "retry-budget status [--task <task-id>]")
	assertNotContains(t, statusHelp, "Usage of")

	recorded := runCapture(t, "live-window", "retry-budget", "record", "LIVE-001",
		"--meaningful-failures", "2",
		"--coordination-failures", "4",
		"--budget", "3")
	assertContains(t, recorded, "meaningful_failures=2")
	assertContains(t, recorded, "coordination_failures=4")
	assertContains(t, recorded, "requires_reset=false")

	status := runCapture(t, "live-window", "retry-budget", "status", "--task", "LIVE-001")
	assertContains(t, status, "next_iteration=3")
	assertContains(t, status, "exhausted=false")

	packet := runCapture(t, "packet", "retry", "LIVE-001",
		"--kind", "live-operation",
		"--source-sha", "abc1234",
		"--operator-surface", "drill-operator-v2",
		"--artifact-dir", ".fairway/artifacts/LIVE-001/retry-003",
		"--evidence-contract", "bounded retry proof",
		"--allowed-action", "run exact approved command",
		"--forbidden-action", "live mutation outside packet",
		"--expires-at", "2026-06-14T21:00:00-05:00",
		"--prior-failure-closure", "prior causal fix verified")
	for _, want := range []string{
		"iteration_count: 3",
		"meaningful_failures: 2",
		"coordination_failures: 4",
		"retry_budget: 3",
		"no hidden approval is granted by this packet",
	} {
		assertContains(t, packet, want)
	}

	runOK(t, "live-window", "retry-budget", "record", "LIVE-001",
		"--meaningful-failures", "3",
		"--coordination-failures", "4",
		"--budget", "3")
	err = run(context.Background(), []string{"packet", "retry", "LIVE-001",
		"--kind", "live-operation",
		"--source-sha", "abc1234",
		"--operator-surface", "drill-operator-v2",
		"--artifact-dir", ".fairway/artifacts/LIVE-001/retry-004",
		"--evidence-contract", "bounded retry proof",
		"--allowed-action", "run exact approved command",
		"--forbidden-action", "live mutation outside packet",
		"--expires-at", "2026-06-14T22:00:00-05:00",
		"--prior-failure-closure", "prior causal fix verified"})
	if err == nil {
		t.Fatal("expected exhausted retry budget to block packet rendering")
	}
	assertContains(t, err.Error(), "packet retry requires causal reset")

	err = run(context.Background(), []string{"live-window", "retry-budget", "record", "LIVE-001",
		"--meaningful-failures", "3",
		"--coordination-failures", "4",
		"--budget", "3",
		"--reset-task", "MISSING-RESET",
		"--reset-reason", "claimed reset"})
	if err == nil {
		t.Fatal("expected missing reset task to fail")
	}
	assertContains(t, err.Error(), "reset task")
	assertContains(t, err.Error(), "not found")

	err = run(context.Background(), []string{"live-window", "retry-budget", "record", "LIVE-001",
		"--meaningful-failures", "3",
		"--coordination-failures", "4",
		"--budget", "3",
		"--reset-task", "RESET-001"})
	if err == nil {
		t.Fatal("expected missing reset reason to fail")
	}
	assertContains(t, err.Error(), "requires --reset-reason")

	runOK(t, "add", "RESET-001", "--title", "Causal reset", "--role", "backend")
	resetPacket := runCapture(t, "live-window", "retry-budget", "record", "LIVE-001",
		"--meaningful-failures", "3",
		"--coordination-failures", "4",
		"--budget", "3",
		"--reset-task", "RESET-001",
		"--reset-reason", "causal model refreshed with replacement surface proof")
	assertContains(t, resetPacket, "requires_reset=false")

	packet = runCapture(t, "packet", "retry", "LIVE-001",
		"--kind", "live-operation",
		"--source-sha", "def5678",
		"--operator-surface", "drill-operator-v3",
		"--artifact-dir", ".fairway/artifacts/LIVE-001/retry-after-reset",
		"--evidence-contract", "post-reset proof",
		"--allowed-action", "run exact approved command",
		"--forbidden-action", "implicit approval",
		"--expires-at", "2026-06-14T23:00:00-05:00",
		"--prior-failure-closure", "RESET-001 completed")
	assertContains(t, packet, "iteration_count: 4")
	assertContains(t, packet, "reset_task: RESET-001")
	assertContains(t, packet, "reset_reason: causal model refreshed with replacement surface proof")

	jsonStatus := runCapture(t, "--json", "live-window", "retry-budget", "status", "--task", "LIVE-001")
	assertContains(t, jsonStatus, `"meaningful_failures": 3`)
	assertContains(t, jsonStatus, `"reset_task": "RESET-001"`)
}

func TestCLI_LiveWindowRetryBudgetCoordinationFailuresDoNotExhaust(t *testing.T) {
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
	runOK(t, "add", "LIVE-002", "--title", "Coordination-only retry gap", "--role", "backend")
	runOK(t, "live-window", "retry-budget", "record", "LIVE-002",
		"--meaningful-failures", "0",
		"--coordination-failures", "9",
		"--budget", "1")

	packet := runCapture(t, "packet", "retry", "LIVE-002",
		"--kind", "preflight",
		"--source-sha", "abc1234",
		"--operator-surface", "local-shell",
		"--artifact-dir", ".fairway/artifacts/LIVE-002/preflight",
		"--evidence-contract", "non-live preflight proof",
		"--allowed-action", "run bounded preflight",
		"--forbidden-action", "live execution",
		"--expires-at", "2026-06-14T21:00:00-05:00",
		"--prior-failure-closure", "coordination handoff repaired")
	assertContains(t, packet, "iteration_count: 1")
	assertContains(t, packet, "meaningful_failures: 0")
	assertContains(t, packet, "coordination_failures: 9")
	assertContains(t, packet, "retry_budget: 1")
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

func TestCLI_PacketRules(t *testing.T) {
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
	writeRulePack(t, "rules-platform", "platform.contract-first", "generated-artifacts-clean")
	writeFile(t, filepath.Join("rules-platform", "rules", "core", "docs.md"), `---
id: platform.docs-only
title: Docs-only rule
status: draft
applies_when:
  source_paths:
    - docs/**
required_evidence:
  - docs-review
review_domains:
  - governance
stop_conditions:
  - docs are stale
---

body
`)
	appendFile(t, ".fairway/config.toml", `
[[rule_sources]]
name = "platform"
source = "path:rules-platform"
mode = "advisory"

[[workstream_profiles]]
name = "platform-foundation"
rule_groups = ["platform.core"]
review_domains = ["backend", "governance"]
`)
	runOK(t, "add", "T-001", "--title", "Rules packet", "--role", "backend", "--profile", "platform-foundation", "--source-paths", "doc/api/openapi.yaml", "--tag", "surface:api", "--risk-level", "medium")
	packet := runCapture(t, "packet", "rules", "T-001")
	assertContains(t, packet, "# Rule Packet: T-001")
	assertContains(t, packet, "## Selected Rules")
	assertContains(t, packet, "platform.contract-first")
	assertContains(t, packet, "required evidence: generated-artifacts-clean")
	assertContains(t, packet, "review domains: backend")
	assertContains(t, packet, "residual risk / stop conditions")
	assertContains(t, packet, "## Non-Applicable Rules")
	assertContains(t, packet, "platform.docs-only")
	assertContains(t, packet, "rationale: source paths do not match")
	assertContains(t, packet, "--artifact-type rule-packet")

	jsonPacket := runCapture(t, "--json", "packet", "rules", "T-001")
	assertContains(t, jsonPacket, `"task_id": "T-001"`)
	assertContains(t, jsonPacket, `"selected"`)
	assertContains(t, jsonPacket, `"non_applicable"`)
	assertContains(t, jsonPacket, `"required_evidence"`)
	assertContains(t, jsonPacket, `"residual_risk_fields"`)
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

func TestCLI_PlaneTrackerDryRunSpike(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	t.Setenv("PLANE_BASE_URL", "http://localhost:8088")
	t.Setenv("PLANE_WORKSPACE", "fairway-eval")
	t.Setenv("PLANE_PROJECT", "FWPLANE")
	t.Setenv("PLANE_API_TOKEN", "test-secret")

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "Tracked", "--role", "backend", "--owning-domain", "tracker", "--review-domains", "arch,backend")
	runOK(t, "claim", "T-001")
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass", "--artifact-type", "test", "--artifact", "dist/test.log")
	before := runCapture(t, "--json", "task-detail", "T-001")
	exported := runCapture(t, "--json", "tracker", "plane", "export", "--task-id", "T-001")
	for _, want := range []string{`"provider": "plane"`, `"dry_run": true`, `"source_task_id": "T-001"`, `"token_present": true`} {
		if !strings.Contains(exported, want) {
			t.Fatalf("plane export missing %q:\n%s", want, exported)
		}
	}
	comment := runCapture(t, "--json", "tracker", "plane", "comment", "--task-id", "T-001", "--external-id", "FWPLANE-1")
	if !strings.Contains(comment, "Fairway execution summary") || !strings.Contains(comment, `"external_id": "FWPLANE-1"`) {
		t.Fatalf("plane comment output unexpected:\n%s", comment)
	}
	fixture := filepath.Clean(filepath.Join(oldwd, "..", "..", "examples", "tracker-adapters", "plane", "evaluation-workspace.yaml"))
	preview := runCapture(t, "--json", "tracker", "plane", "import", "--fixture", fixture)
	if !strings.Contains(preview, `"dry_run": true`) || !strings.Contains(preview, `"id": "FW-122"`) {
		t.Fatalf("plane import preview unexpected:\n%s", preview)
	}
	after := runCapture(t, "--json", "task-detail", "T-001")
	if before != after {
		t.Fatalf("dry-run plane commands mutated task detail\nbefore=%s\nafter=%s", before, after)
	}
	runCaptureAllowError(t, "tracker", "plane", "export", "--task-id", "T-001", "--apply")
}

func TestCLI_PlaneTrackerMissingConfig(t *testing.T) {
	repo := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	t.Setenv("PLANE_BASE_URL", "")
	t.Setenv("PLANE_WORKSPACE", "")
	t.Setenv("PLANE_PROJECT", "")

	runOK(t, "init")
	runOK(t, "add", "T-001", "--title", "Tracked", "--role", "backend")
	runCaptureAllowError(t, "tracker", "plane", "export", "--task-id", "T-001")
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
	}, "127.0.0.1:7878", true, true, true)
	want := []string{
		"--config", "/tmp/fairway.toml",
		"--db", "/tmp/fairway.db",
		"--as", "backend",
		"dashboard", "--listen", "127.0.0.1:7878", "--no-open", "--read-only", "--multi",
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

func runAdapter(t *testing.T, script string, args ...string) string {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("adapter %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func runAdapterWithEnv(t *testing.T, env []string, script string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeFakeFairwaySessionStatus(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fairway")
	body := `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--json" && "${2:-}" == "session" && "${3:-}" == "status" && "${4:-}" == "--all" ]]; then
  printf '%s\n' "${FAKE_SESSION_JSON:-[]}"
  exit "${FAKE_SESSION_STATUS_EXIT:-0}"
fi
printf 'fairway'
printf ' %q' "$@"
printf '\n'
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
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
	readDone := make(chan error, 1)
	go func() {
		_, readErr := out.ReadFrom(r)
		readDone <- readErr
	}()
	runErr := run(context.Background(), args)
	_ = w.Close()
	if readErr := <-readDone; readErr != nil {
		return "", readErr
	}
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

func extractJSONValue(body, key string) string {
	needle := `"` + key + `": "`
	idx := strings.Index(body, needle)
	if idx < 0 {
		return ""
	}
	start := idx + len(needle)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
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

func writeRulePack(t *testing.T, dir, ruleID, evidenceType string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "schemas", "rule.schema.yaml"), `type: object
required:
  - id
  - title
  - status
`)
	writeFile(t, filepath.Join(dir, "rules", "core", "contract.md"), fmt.Sprintf(`---
id: %s
title: Contract rule
status: draft
applies_when:
  source_paths:
    - doc/api/**
  tags:
    - surface:api
  task_kinds:
    - task
risk_floor: medium
required_evidence:
  - %s
review_domains:
  - backend
---

body
`, ruleID, evidenceType))
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

func jsonIntField(t *testing.T, raw, field string) int64 {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, raw)
	}
	value, ok := findJSONField(decoded, field)
	if !ok {
		t.Fatalf("field %q not found in JSON:\n%s", field, raw)
	}
	number, ok := value.(float64)
	if !ok {
		t.Fatalf("field %q=%T, want number", field, value)
	}
	return int64(number)
}

func findJSONField(value any, field string) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if found, ok := typed[field]; ok {
			return found, true
		}
		for _, child := range typed {
			if found, ok := findJSONField(child, field); ok {
				return found, true
			}
		}
	case []any:
		for _, child := range typed {
			if found, ok := findJSONField(child, field); ok {
				return found, true
			}
		}
	}
	return nil, false
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
