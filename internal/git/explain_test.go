package git

import (
	"path/filepath"
	"testing"
)

func TestResolveCommitFileAndBlameLine(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(repo, "main.go"), "package main\n\nfunc Run() {}\n")
	git(t, repo, "add", "main.go")
	git(t, repo, "commit", "-m", "add run")
	detail, err := ResolveCommit(repo, "HEAD")
	if err != nil || detail.SHA == "" || len(detail.ChangedFiles) != 1 || detail.ChangedFiles[0] != "main.go" {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
	data, err := FileAtCommit(repo, detail.SHA, "main.go")
	if err != nil || string(data) != "package main\n\nfunc Run() {}\n" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	blame, err := BlameLine(repo, detail.SHA, "main.go", 3)
	if err != nil || blame != detail.SHA {
		t.Fatalf("blame=%q want=%q err=%v", blame, detail.SHA, err)
	}
	if _, err := FileAtCommit(repo, detail.SHA, "../secret"); err == nil {
		t.Fatal("expected traversal path rejection")
	}
}
