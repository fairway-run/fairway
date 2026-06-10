package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckReportsDirtyFiles(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	git(t, repo, "add", "README.md")
	git(t, repo, "commit", "-m", "init")
	writeFile(t, filepath.Join(repo, "dirty.txt"), "dirty\n")

	status, err := Check(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	if status.Branch != "main" {
		t.Fatalf("branch=%q, want main", status.Branch)
	}
	if !status.Dirty || !status.Untracked {
		t.Fatalf("status=%+v, want dirty untracked", status)
	}
	if len(status.UntrackedFiles) != 1 || status.UntrackedFiles[0] != "dirty.txt" {
		t.Fatalf("untracked files=%v, want dirty.txt", status.UntrackedFiles)
	}
	if !status.BaseAncestor {
		t.Fatalf("status=%+v, want base ancestor", status)
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
