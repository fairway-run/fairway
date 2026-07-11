package dashboard

import (
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/reconcile"
	"github.com/subashram/fairway/internal/store"
)

func TestRecommendCommonPathProgressiveAndFailClosed(t *testing.T) {
	task := store.Task{Definition: store.TaskDefinition{ID: "T-001"}, Status: "in_progress"}
	verify := RecommendCommonPath(CommonPathInput{Task: task, ActiveSessions: 1, MissingReviewDomains: []string{"ops"}, ReviewMode: "blocking"})
	if verify.Classification != "verify" || len(verify.Blockers) != 0 {
		t.Fatalf("pre-evidence recommendation=%+v", verify)
	}
	review := RecommendCommonPath(CommonPathInput{Task: task, ActiveSessions: 1, EvidenceCount: 1, MissingReviewDomains: []string{"ops"}, ReviewMode: "blocking"})
	if review.Classification != "review" || !strings.Contains(review.Blockers[0], "ops") {
		t.Fatalf("review recommendation=%+v", review)
	}
	ambiguous := RecommendCommonPath(CommonPathInput{Task: task, ActiveSessions: 2, EvidenceCount: 1})
	if ambiguous.Classification != "ambiguous" || ambiguous.BoundaryStatus != "blocked" {
		t.Fatalf("ambiguous recommendation=%+v", ambiguous)
	}
	close := RecommendCommonPath(CommonPathInput{Task: task, ActiveSessions: 1, EvidenceCount: 1, ActiveFindings: []reconcile.ActiveFinding{{Kind: "status_decision_required", TaskID: "T-001"}}})
	if close.Classification != "close" || !strings.Contains(close.CurrentAction, "status decision") {
		t.Fatalf("close recommendation=%+v", close)
	}
	done := task
	done.Status = "done"
	terminalDebt := RecommendCommonPath(CommonPathInput{Task: done, ActiveSessions: 1, EvidenceCount: 1})
	if terminalDebt.Classification != "blocked" || !strings.Contains(terminalDebt.Blockers[0], "active session") {
		t.Fatalf("terminal session debt=%+v", terminalDebt)
	}
}
