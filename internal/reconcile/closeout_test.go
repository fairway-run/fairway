package reconcile

import (
	"context"
	"testing"

	"github.com/subashram/fairway/internal/store"
)

func TestCloseoutCleanMergedBranchIsEligible(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend", ReviewDomains: []string{"arch"}}}); err != nil {
		t.Fatal(err)
	}
	seedDoneState(t, ctx, s, "T-001", "agent/backend", "abc123")
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "gh run view --json conclusion", Result: "pass", ArtifactType: "ci"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", store.Review{Reviewer: "arch", Verdict: "approve", Reason: "ok"}); err != nil {
		t.Fatal(err)
	}
	report, err := Closeout(ctx, s, CloseoutOptions{
		TaskID:   "T-001",
		Terminal: []string{"done"},
		Git:      CloseoutGit{Branch: "agent/backend", BranchExists: true, BranchMerged: true, WorktreePath: "/tmp/backend"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.SafeToDeleteBranches != 1 || report.Summary.VerificationEvidence != 1 {
		t.Fatalf("report=%+v, want clean safe merged branch", report)
	}
}

func TestCloseoutReportsReviewSessionWatcherAndUnmergedBranchBlockers(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend", ReviewDomains: []string{"arch", "ops"}}}); err != nil {
		t.Fatal(err)
	}
	seedDoneState(t, ctx, s, "T-001", "agent/backend", "abc123")
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSession(ctx, store.Session{ID: "s-1", Role: "backend", TaskID: "T-001", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := s.StartWatcher(ctx, store.Watcher{ID: "w-1", TaskID: "T-001", Owner: "ops", Process: "ci", Command: "gh run watch"}); err != nil {
		t.Fatal(err)
	}

	report, err := Closeout(ctx, s, CloseoutOptions{
		TaskID:   "T-001",
		Terminal: []string{"done"},
		Git:      CloseoutGit{Branch: "agent/backend", BranchExists: true, BranchMerged: false, WorktreePath: "/tmp/backend"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"missing_review_domains", "active_session", "verification_pending", "unmerged_branch"} {
		if !hasCloseoutFinding(report, want) {
			t.Fatalf("report missing %s: %+v", want, report)
		}
	}
	if report.OK || report.Summary.Blockers != 4 || report.Summary.MissingReviewDomains != 2 || report.Summary.PendingVerification != 1 {
		t.Fatalf("report=%+v, want blockers, missing domains, and pending verification", report)
	}
}

func TestCloseoutPreservesUnmergedBranchWithReason(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	seedDoneState(t, ctx, s, "T-001", "agent/backend", "abc123")
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass"}); err != nil {
		t.Fatal(err)
	}

	report, err := Closeout(ctx, s, CloseoutOptions{
		TaskID:         "T-001",
		Terminal:       []string{"done"},
		PreserveReason: "release branch retained until tag cut",
		Git:            CloseoutGit{Branch: "release/fairway", BranchExists: true, BranchMerged: false, RemoteBranchExists: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.PreservedBranches != 1 || report.Summary.RemoteBranchesPresent != 1 {
		t.Fatalf("report=%+v, want preserved branch warning without blocker", report)
	}
}

func TestCloseoutDirtyWorktreeBlocks(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	seedDoneState(t, ctx, s, "T-001", "agent/backend", "abc123")
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass"}); err != nil {
		t.Fatal(err)
	}

	report, err := Closeout(ctx, s, CloseoutOptions{
		TaskID:   "T-001",
		Terminal: []string{"done"},
		Git:      CloseoutGit{Branch: "agent/backend", BranchExists: true, BranchMerged: true, WorktreeDirty: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasCloseoutFinding(report, "dirty_worktree") {
		t.Fatalf("report=%+v, want dirty worktree blocker", report)
	}
}

func TestCloseoutMissingCommitAssociationBlocks(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	seedDoneState(t, ctx, s, "T-001", "agent/backend", "")
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass"}); err != nil {
		t.Fatal(err)
	}

	report, err := Closeout(ctx, s, CloseoutOptions{
		TaskID:   "T-001",
		Terminal: []string{"done"},
		Git:      CloseoutGit{Branch: "agent/backend", BranchExists: true, BranchMerged: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasCloseoutFinding(report, "missing_commit_association") || report.Summary.MissingCommits != 1 {
		t.Fatalf("report=%+v, want missing commit blocker", report)
	}
}

func TestCloseoutSafeMergedRemoteBranchDryRunPlan(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	seedDoneState(t, ctx, s, "T-001", "agent/backend", "abc123")
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "gh run view --json conclusion", Result: "pass", ArtifactType: "ci"}); err != nil {
		t.Fatal(err)
	}

	report, err := Closeout(ctx, s, CloseoutOptions{
		TaskID:   "T-001",
		Terminal: []string{"done"},
		Git:      CloseoutGit{Branch: "agent/backend", BranchExists: true, BranchMerged: true, RemoteBranchExists: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || !report.Apply.DeleteRemoteBranch || !hasCloseoutFinding(report, "safe_merged_remote_branch") {
		t.Fatalf("report=%+v, want safe remote deletion dry-run plan", report)
	}
}

func TestCloseoutDoesNotClassifyIncidentalCITextAsVerification(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	seedDoneState(t, ctx, s, "T-001", "agent/backend", "abc123")
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{
		CommandText: "record verification association notes",
		Result:      "pass",
		Notes:       "association and verification are bookkeeping notes",
	}); err != nil {
		t.Fatal(err)
	}

	report, err := Closeout(ctx, s, CloseoutOptions{
		TaskID:   "T-001",
		Terminal: []string{"done"},
		Git:      CloseoutGit{Branch: "agent/backend", BranchExists: true, BranchMerged: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.VerificationEvidence != 0 || hasCloseoutFinding(report, "verification_evidence") {
		t.Fatalf("report=%+v, incidental words must not count as CI verification", report)
	}
}

func TestCloseoutDoesNotClassifyIncidentalWatcherTextAsVerificationPending(t *testing.T) {
	ctx := context.Background()
	s := newReconcileTestStore(t)
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Done", Role: "backend"}}); err != nil {
		t.Fatal(err)
	}
	seedDoneState(t, ctx, s, "T-001", "agent/backend", "abc123")
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass"}); err != nil {
		t.Fatal(err)
	}
	if err := s.StartWatcher(ctx, store.Watcher{
		ID:      "w-1",
		TaskID:  "T-001",
		Owner:   "backend",
		Process: "association verifier",
		Command: "record verification notes",
	}); err != nil {
		t.Fatal(err)
	}

	report, err := Closeout(ctx, s, CloseoutOptions{
		TaskID:   "T-001",
		Terminal: []string{"done"},
		Git:      CloseoutGit{Branch: "agent/backend", BranchExists: true, BranchMerged: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.PendingVerification != 0 || hasCloseoutFinding(report, "verification_pending") || !hasCloseoutFinding(report, "active_watcher") {
		t.Fatalf("report=%+v, incidental watcher text must remain generic active watcher", report)
	}
}

func seedDoneState(t *testing.T, ctx context.Context, s *store.Store, taskID, branch, commit string) {
	t.Helper()
	updated, err := s.ImportTaskStatesOnce(ctx, []store.ImportedTaskState{{
		TaskID:    taskID,
		Status:    "done",
		Owner:     "backend",
		Branch:    branch,
		CommitSHA: commit,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("state import updated=%d, want 1", updated)
	}
}

func hasCloseoutFinding(report CloseoutReport, kind string) bool {
	for _, finding := range report.Findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}
