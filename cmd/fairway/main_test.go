package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
	runOK(t, "record", "evidence", "T-001", "--command-text", "go test ./...", "--result", "pass")
	runOK(t, "record", "review", "T-001", "--reviewer", "ui", "--verdict", "approve", "--reason", "ok")
	runOK(t, "set-status", "T-001", "done")
	runOK(t, "task-detail", "T-001")
	runOK(t, "--json", "task-detail", "T-001")
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Clean(path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
