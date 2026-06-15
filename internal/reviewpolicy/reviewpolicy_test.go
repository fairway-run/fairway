package reviewpolicy

import (
	"testing"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestEvaluateOneReviewMicroSlice(t *testing.T) {
	cfg := config.Config{ReviewProfiles: []config.ReviewProfile{{
		Name:                     "micro-slice",
		Mode:                     "advisory",
		MatchKinds:               []string{"docs"},
		MatchRiskLevels:          []string{"low"},
		RequiredReviewDomains:    []string{"governance"},
		ExtraReviewerRationale:   "governance catches evidence contract drift",
		SafeIterationZone:        true,
		SafeIterationDefectClass: "harness",
		SafeIterationControl:     "non-live disposable boundary",
	}}}
	task := store.Task{Definition: store.TaskDefinition{ID: "T-001", Kind: "docs", RiskLevel: "low", ReviewDomains: []string{"backend"}}}
	eval := Evaluate(cfg, Options{Task: task, Reviews: []store.Review{{Domain: "governance", Verdict: "approve"}}})
	if eval.Profile != "micro-slice" {
		t.Fatalf("profile=%q", eval.Profile)
	}
	if eval.Mode != "advisory" {
		t.Fatalf("mode=%q", eval.Mode)
	}
	if !eval.SafeIterationZone || eval.SafeIterationDefectClass != "harness" || eval.SafeIterationControl != "non-live disposable boundary" {
		t.Fatalf("safe iteration fields=%+v", eval)
	}
	if eval.ExtraReviewerRationale != "governance catches evidence contract drift" {
		t.Fatalf("rationale=%q", eval.ExtraReviewerRationale)
	}
	if len(eval.EffectiveDomains) != 2 || len(eval.MissingReviewDomains) != 1 || eval.MissingReviewDomains[0] != "backend" {
		t.Fatalf("eval=%+v", eval)
	}
}

func TestEvaluateGroupedChildReviewInheritance(t *testing.T) {
	cfg := config.Config{ReviewProfiles: []config.ReviewProfile{{
		Name:                  "grouped-slice",
		MatchTags:             []string{"slice:grouped"},
		RequiredReviewDomains: []string{"backend", "governance"},
		InheritFromParent:     true,
		InheritReviewDomains:  []string{"backend", "governance"},
		GroupReview:           true,
	}}}
	parent := store.Task{Definition: store.TaskDefinition{ID: "EPIC-001"}}
	task := store.Task{Definition: store.TaskDefinition{ID: "CHILD-001", ParentID: "EPIC-001", Tags: []string{"slice:grouped"}}}
	eval := Evaluate(cfg, Options{
		Task:          task,
		Parent:        &parent,
		ParentReviews: []store.Review{{Domain: "backend", Verdict: "approve"}, {Domain: "governance", Verdict: "approve"}},
	})
	if !eval.GroupReview || len(eval.EffectiveDomains) != 0 || len(eval.MissingReviewDomains) != 0 {
		t.Fatalf("eval=%+v", eval)
	}
	for _, req := range eval.Requirements {
		if req.Status != "inherited" {
			t.Fatalf("requirement=%+v, want inherited", req)
		}
	}
}

func TestEvaluateLatestReviewVerdictWins(t *testing.T) {
	cfg := config.Config{ReviewProfiles: []config.ReviewProfile{{
		Name:                  "grouped-slice",
		MatchTags:             []string{"slice:grouped"},
		RequiredReviewDomains: []string{"backend", "governance"},
		InheritFromParent:     true,
		InheritReviewDomains:  []string{"backend", "governance"},
		GroupReview:           true,
	}}}
	parent := store.Task{Definition: store.TaskDefinition{ID: "EPIC-004"}}
	task := store.Task{Definition: store.TaskDefinition{ID: "CHILD-004", ParentID: "EPIC-004", Tags: []string{"slice:grouped"}}}
	eval := Evaluate(cfg, Options{
		Task:   task,
		Parent: &parent,
		Reviews: []store.Review{
			{Domain: "backend", Verdict: "approve"},
			{Domain: "backend", Verdict: "changes"},
		},
		ParentReviews: []store.Review{
			{Domain: "backend", Verdict: "approve"},
			{Domain: "backend", Verdict: "changes"},
			{Domain: "governance", Verdict: "approve"},
		},
	})
	if len(eval.MissingReviewDomains) != 1 || eval.MissingReviewDomains[0] != "backend" {
		t.Fatalf("missing=%v eval=%+v, want backend missing after latest changes verdict", eval.MissingReviewDomains, eval)
	}
	for _, req := range eval.Requirements {
		if req.Domain == "backend" && req.Status != "required" {
			t.Fatalf("backend requirement=%+v, want required after latest parent changes verdict", req)
		}
		if req.Domain == "governance" && req.Status != "inherited" {
			t.Fatalf("governance requirement=%+v, want inherited", req)
		}
	}
}

func TestEvaluateFullEpicReviewAndSameLaneAuthoring(t *testing.T) {
	cfg := config.Config{ReviewProfiles: []config.ReviewProfile{{
		Name:                  "epic",
		MatchKinds:            []string{"epic"},
		MatchAuthoringDomains: []string{"backend"},
		RequiredReviewDomains: []string{"arch", "backend", "governance", "ops", "security"},
	}}}
	task := store.Task{Definition: store.TaskDefinition{ID: "EPIC-002", Kind: "epic", Role: "backend"}}
	eval := Evaluate(cfg, Options{Task: task, Reviews: []store.Review{{Domain: "backend", Verdict: "approve"}}})
	if eval.Profile != "epic" || len(eval.MissingReviewDomains) != 4 {
		t.Fatalf("eval=%+v", eval)
	}
}

func TestEvaluateNoInheritanceTriggers(t *testing.T) {
	cfg := config.Config{ReviewProfiles: []config.ReviewProfile{{
		Name:                  "live-window",
		MatchTags:             []string{"slice:grouped"},
		RequiredReviewDomains: []string{"security"},
		InheritFromParent:     true,
		InheritReviewDomains:  []string{"security"},
		NoInheritanceTags:     []string{"authority:live"},
		NoInheritancePaths:    []string{"deploy/"},
	}}}
	parent := store.Task{Definition: store.TaskDefinition{ID: "EPIC-003"}}
	task := store.Task{Definition: store.TaskDefinition{ID: "LIVE-001", Tags: []string{"slice:grouped", "authority:live"}, TargetPaths: []string{"deploy/live.sh"}}}
	eval := Evaluate(cfg, Options{Task: task, Parent: &parent, ParentReviews: []store.Review{{Domain: "security", Verdict: "approve"}}})
	if !eval.InheritanceBlocked || len(eval.EffectiveDomains) != 1 || eval.MissingReviewDomains[0] != "security" {
		t.Fatalf("eval=%+v", eval)
	}
}

func TestDetectLoopRecommendsCausalReset(t *testing.T) {
	eval := Evaluation{
		Profile:                  "micro-slice",
		SafeIterationZone:        true,
		SafeIterationDefectClass: "harness",
		SafeIterationControl:     "non-live disposable boundary",
	}
	task := store.Task{Definition: store.TaskDefinition{ID: "LOOP-001"}}
	loop := DetectLoop(task, eval, []store.Evidence{
		{Result: "pass", CommandText: "near-ready harness readback"},
		{Result: "fail", ArtifactType: "harness", Notes: "browser smoke harness failed"},
		{Result: "blocked", ArtifactType: "harness", Notes: "same harness launch failure after review"},
	}, []store.Review{{Domain: "governance", Verdict: "approve"}})
	if !loop.Detected {
		t.Fatalf("loop not detected: %+v", loop)
	}
	if len(loop.FailureChain) != 2 || len(loop.RealUnknowns) == 0 || len(loop.RequiredProofBeforeRetry) == 0 {
		t.Fatalf("loop recommendation incomplete: %+v", loop)
	}
	if loop.LighterReviewPlan != "stay inside non-live disposable boundary for harness fixes with one accountable review until a boundary exit is requested" {
		t.Fatalf("lighter review plan=%q", loop.LighterReviewPlan)
	}
}
