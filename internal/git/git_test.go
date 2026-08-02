package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestCheckPreservesUnstagedTrackedPath(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(repo, "tracked.txt"), "hello\n")
	git(t, repo, "add", "tracked.txt")
	git(t, repo, "commit", "-m", "init")
	writeFile(t, filepath.Join(repo, "tracked.txt"), "changed\n")

	status, err := Check(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Dirty {
		t.Fatalf("status=%+v, want dirty", status)
	}
	if status.Staged {
		t.Fatalf("status=%+v, want unstaged tracked change to remain unstaged", status)
	}
	if len(status.TrackedChangedFiles) != 1 || status.TrackedChangedFiles[0] != "tracked.txt" {
		t.Fatalf("tracked changed files=%v, want tracked.txt", status.TrackedChangedFiles)
	}
	if len(status.ChangedFiles) != 1 || status.ChangedFiles[0] != "tracked.txt" {
		t.Fatalf("changed files=%v, want tracked.txt", status.ChangedFiles)
	}
}

func TestCommitsAfterIncludesParentCount(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(repo, "a.txt"), "one\n")
	git(t, repo, "add", "a.txt")
	git(t, repo, "commit", "-m", "first")
	first := LastCommit(repo)
	writeFile(t, filepath.Join(repo, "a.txt"), "two\n")
	git(t, repo, "add", "a.txt")
	gitWithEnv(t, repo, []string{"GIT_AUTHOR_DATE=2001-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2030-01-01T00:00:00Z"}, "commit", "-m", "second")
	second := LastCommit(repo)
	writeFile(t, filepath.Join(repo, "a.txt"), "three\n")
	git(t, repo, "add", "a.txt")
	git(t, repo, "commit", "-m", "third")
	commits, err := CommitsAfter(repo, first)
	if err != nil || len(commits) != 2 || commits[0].ParentCount != 1 || commits[0].ChangedFiles[0] != "a.txt" {
		t.Fatalf("commits=%+v err=%v", commits, err)
	}
	pinned, err := CommitsBetween(repo, first, second)
	if err != nil || len(pinned) != 1 || !strings.HasPrefix(pinned[0].SHA, second) {
		t.Fatalf("pinned commits=%+v err=%v", pinned, err)
	}
	if !strings.HasPrefix(pinned[0].AuthorDate, "2001-01-01") || !strings.HasPrefix(pinned[0].CommitDate, "2030-01-01") {
		t.Fatalf("author/commit dates=%s/%s", pinned[0].AuthorDate, pinned[0].CommitDate)
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

func gitWithEnv(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
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
