package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/config"
	fairwaygit "github.com/subashram/fairway/internal/git"
	"github.com/subashram/fairway/internal/store"
)

func TestWorkCoverageReportsDenominatorsOutcomesAndPostPromotionTouches(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	writeAuditFile(t, root, "README.md", "init\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")
	initial := gitOutput(t, root, "rev-parse", "HEAD")

	db, err := store.Open(ctx, filepath.Join(root, ".fairway", "state.db"), "coverage-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Promotion", Role: "backend", TargetPaths: []string{"src/**"}},
		{ID: "T-002", Title: "Correction", Role: "backend", TargetPaths: []string{"src/**"}},
	}); err != nil {
		t.Fatal(err)
	}

	writeAuditFile(t, root, "src/service.go", "package service\n")
	runGit(t, root, "add", "src/service.go")
	runGit(t, root, "commit", "-m", "T-001 add service")
	promotion := gitOutput(t, root, "rev-parse", "HEAD")
	if err := db.SetStatusWithCommit(ctx, "T-001", "done", "promoted", promotion, false); err != nil {
		t.Fatal(err)
	}

	writeAuditFile(t, root, "src/service.go", "package service\n// follow-up\n")
	runGit(t, root, "add", "src/service.go")
	runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2001-01-01T00:00:00Z", "GIT_COMMITTER_DATE=" + time.Now().UTC().Add(time.Second).Format(time.RFC3339)}, "commit", "-m", "follow-up touch")
	writeAuditFile(t, root, "generated-output/generated.txt", "generated\n")
	runGit(t, root, "add", "generated-output/generated.txt")
	runGit(t, root, "commit", "-m", "generated output")

	if _, err := db.RecordTaskOutcome(ctx, store.TaskOutcome{TaskID: "T-001", Kind: "incident", SourceRef: "incident:INC-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RecordTaskOutcome(ctx, store.TaskOutcome{TaskID: "T-001", Kind: "corrective", RelatedTaskID: "T-002"}); err != nil {
		t.Fatal(err)
	}
	detail, err := fairwaygit.ResolveCommit(root, promotion)
	if err != nil {
		t.Fatal(err)
	}
	promotedAt, err := time.Parse(time.RFC3339, detail.AuthorDate)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(root)
	cfg.Fairway.MainBranch = initial
	cfg.ControlEffectiveness.PathExclusions = []config.ControlPathExclusion{{Pattern: "generated-output/**", Category: "generated", Rationale: "test build output"}}
	report, err := BuildWorkCoverageReport(ctx, cfg, root, db, WorkCoverageOptions{SinceRef: initial, Now: promotedAt.Add(31 * 24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if report.AsOf == "" {
		t.Fatal("report as_of is empty")
	}
	if report.AnalyzedTip == "" {
		t.Fatal("report analyzed_tip is empty")
	}
	if report.Denominators.ObservedCommits != 3 || report.Denominators.EligibleCommits != 2 || report.Denominators.CoveredCommits != 1 || report.Denominators.ExcludedOnlyCommits != 1 {
		t.Fatalf("denominators=%+v", report.Denominators)
	}
	if report.Denominators.EligibleChangedFiles != 2 || report.Denominators.CoveredChangedFiles != 2 || report.Denominators.ExcludedChangedFiles != 1 {
		t.Fatalf("file denominators=%+v", report.Denominators)
	}
	var followUp CommitCoverageFact
	for _, fact := range report.CommitFacts {
		if fact.Subject == "follow-up touch" {
			followUp = fact
			break
		}
	}
	if len(followUp.TaskIDs) != 0 || len(followUp.PathTaskIDs) != 2 {
		t.Fatalf("follow-up explicit/path task links=%v/%v", followUp.TaskIDs, followUp.PathTaskIDs)
	}
	if len(report.TouchFacts) != 1 || !report.TouchFacts[0].Available || len(report.TouchFacts[0].Windows) != 3 {
		t.Fatalf("touch facts=%+v", report.TouchFacts)
	}
	for _, window := range report.TouchFacts[0].Windows {
		if !window.Mature || !window.Touched || len(window.TouchCommits) != 1 {
			t.Fatalf("window=%+v", window)
		}
	}
	if len(report.Outcomes) != 2 || report.Outcomes[0].Kind != "incident" || report.Outcomes[1].RelatedTaskID != "T-002" {
		t.Fatalf("outcomes=%+v", report.Outcomes)
	}
}

func TestTasksWithCommitUsesCanonicalFullOrBoundedShortSHA(t *testing.T) {
	tasks := []store.Task{
		{Definition: store.TaskDefinition{ID: "T-001"}, CommitSHA: "1234567890abcdef"},
		{Definition: store.TaskDefinition{ID: "T-002"}, CommitSHA: "abcdef0"},
		{Definition: store.TaskDefinition{ID: "T-003"}, CommitSHA: "123"},
	}
	if got := tasksWithCommit("1234567890abcdef", tasks); len(got) != 1 || got[0] != "T-001" {
		t.Fatalf("full match=%v", got)
	}
	if got := tasksWithCommit("abcdef0123456789", tasks); len(got) != 1 || got[0] != "T-002" {
		t.Fatalf("short match=%v", got)
	}
	if got := tasksWithCommit("1239999999999999", tasks); len(got) != 0 {
		t.Fatalf("unsafe short match=%v", got)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runGitEnv(t *testing.T, root string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func writeAuditFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
