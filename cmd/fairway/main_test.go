package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func runOK(t *testing.T, args ...string) {
	t.Helper()
	stdout := os.Stdout
	stderr := os.Stderr
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
	}()
	var out bytes.Buffer
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	if err := run(context.Background(), args); err != nil {
		t.Fatalf("fairway %v: %v", args, err)
	}
	_ = w.Close()
	_, _ = out.ReadFrom(r)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Clean(path), []byte(content), 0o644); err != nil {
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
