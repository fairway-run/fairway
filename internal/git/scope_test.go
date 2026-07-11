package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateScopeClassifiesDeclaredAcceptedAndUnexplained(t *testing.T) {
	report := EvaluateScope(
		[]string{"cmd/fairway/main.go", "docs/design/new.md", "scripts/generated.sh"},
		[]string{"cmd/fairway"},
		[]string{"docs/design/**"},
	)
	if report.ChangedPaths != 3 || report.DeclaredPaths != 1 || report.DecisionExplained != 1 || report.UnexplainedPaths != 1 {
		t.Fatalf("report=%+v", report)
	}
	if report.Rows[0].Status != "declared" || report.Rows[0].OwnershipDomain != "backend" {
		t.Fatalf("declared row=%+v", report.Rows[0])
	}
	if report.Rows[1].Status != "accepted_decision" || report.Rows[1].OwnershipDomain != "governance" {
		t.Fatalf("decision row=%+v", report.Rows[1])
	}
	if report.Rows[2].Status != "unexplained" || report.Rows[2].OwnershipDomain != "ops" {
		t.Fatalf("unexplained row=%+v", report.Rows[2])
	}
}

func TestChangedScopeFilesCombinesDirtyAndCommittedBranchPaths(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(repo, "README.md"), "initial\n")
	git(t, repo, "add", "README.md")
	git(t, repo, "commit", "-m", "initial")
	git(t, repo, "checkout", "-b", "feature/scope")
	writeFile(t, filepath.Join(repo, "internal.go"), "package example\n")
	git(t, repo, "add", "internal.go")
	git(t, repo, "commit", "-m", "feature")
	if err := os.WriteFile(filepath.Join(repo, "notes.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := ChangedScopeFiles(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "internal.go" || paths[1] != "notes.md" {
		t.Fatalf("paths=%v", paths)
	}
}
