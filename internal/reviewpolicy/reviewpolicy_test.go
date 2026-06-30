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

func TestEvaluateDefaultReversibleProfile(t *testing.T) {
	task := store.Task{Definition: store.TaskDefinition{ID: "REV-001", RiskLevel: "reversible", ReviewDomains: []string{"backend", "governance"}}}
	eval := Evaluate(config.Config{}, Options{Task: task})
	if eval.Profile != "reversible" || eval.Mode != "advisory" || !eval.SafeIterationZone {
		t.Fatalf("eval=%+v, want reversible advisory safe iteration profile", eval)
	}
	if len(eval.EffectiveDomains) != 0 || len(eval.MissingReviewDomains) != 0 {
		t.Fatalf("eval=%+v, reversible profile should not block on review domains", eval)
	}
	statusByDomain := map[string]string{}
	for _, req := range eval.Requirements {
		statusByDomain[req.Domain] = req.Status
	}
	for _, domain := range []string{"backend", "governance"} {
		if statusByDomain[domain] != "waived" {
			t.Fatalf("domain %s status=%q, want waived; eval=%+v", domain, statusByDomain[domain], eval)
		}
	}
}

func TestEvaluateDefaultBoundaryProfiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		task store.Task
		want []string
	}{
		{
			name: "irreversible",
			task: store.Task{Definition: store.TaskDefinition{ID: "IRR-001", RiskLevel: "irreversible"}},
			want: []string{"architecture", "governance", "ops", "security"},
		},
		{
			name: "live-boundary",
			task: store.Task{Definition: store.TaskDefinition{ID: "LIVE-001", Kind: "live-window"}},
			want: []string{"backend", "governance", "ops", "security"},
		},
		{
			name: "release-boundary",
			task: store.Task{Definition: store.TaskDefinition{ID: "REL-001", Kind: "release-risk"}},
			want: []string{"governance", "ops", "security"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eval := Evaluate(config.Config{}, Options{Task: tc.task})
			if eval.Profile != tc.name || eval.Mode != "blocking" {
				t.Fatalf("eval=%+v, want profile %s blocking", eval, tc.name)
			}
			if len(eval.EffectiveDomains) != len(tc.want) || len(eval.MissingReviewDomains) != len(tc.want) {
				t.Fatalf("eval=%+v, want domains %v", eval, tc.want)
			}
			for i, want := range tc.want {
				if eval.EffectiveDomains[i] != want || eval.MissingReviewDomains[i] != want {
					t.Fatalf("eval=%+v, want sorted domain %s at %d", eval, want, i)
				}
			}
		})
	}
}

func TestEvaluateConfiguredProfileOverridesDefaultName(t *testing.T) {
	cfg := config.Config{ReviewProfiles: []config.ReviewProfile{{
		Name:                  "reversible",
		MatchTags:             []string{"custom-reversible"},
		RequiredReviewDomains: []string{"governance"},
	}}}
	task := store.Task{Definition: store.TaskDefinition{ID: "REV-002", RiskLevel: "reversible"}}
	eval := Evaluate(cfg, Options{Task: task})
	if eval.Profile != "" || len(eval.Requirements) != 0 {
		t.Fatalf("eval=%+v, configured profile name should replace default reversible matching", eval)
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

func TestEvaluateGroupedChildBoundaryMarkersBlockInheritance(t *testing.T) {
	cfg := config.Config{ReviewProfiles: []config.ReviewProfile{{
		Name:                  "grouped-slice",
		MatchTags:             []string{"slice:grouped"},
		RequiredReviewDomains: []string{"backend", "governance"},
		InheritFromParent:     true,
		InheritReviewDomains:  []string{"backend", "governance"},
		GroupReview:           true,
	}}}
	parent := store.Task{Definition: store.TaskDefinition{ID: "EPIC-001"}}
	parentReviews := []store.Review{
		{Domain: "architecture", Verdict: "approve"},
		{Domain: "backend", Verdict: "approve"},
		{Domain: "governance", Verdict: "approve"},
		{Domain: "ops", Verdict: "approve"},
		{Domain: "security", Verdict: "approve"},
	}
	safe := store.Task{Definition: store.TaskDefinition{ID: "SAFE-001", ParentID: "EPIC-001", Tags: []string{"slice:grouped"}}}
	safeEval := Evaluate(cfg, Options{Task: safe, Parent: &parent, ParentReviews: parentReviews})
	if safeEval.InheritanceBlocked || len(safeEval.EffectiveDomains) != 0 || len(safeEval.MissingReviewDomains) != 0 {
		t.Fatalf("safe grouped child eval=%+v, want inherited parent coverage", safeEval)
	}
	for _, tc := range []struct {
		name string
		task store.Task
		want []string
	}{
		{
			name: "irreversible",
			task: store.Task{Definition: store.TaskDefinition{ID: "IRR-001", ParentID: "EPIC-001", RiskLevel: "irreversible", Tags: []string{"slice:grouped"}}},
			want: []string{"architecture", "backend", "governance", "ops", "security"},
		},
		{
			name: "live",
			task: store.Task{Definition: store.TaskDefinition{ID: "LIVE-001", ParentID: "EPIC-001", RiskLevel: "live-boundary", Tags: []string{"slice:grouped"}}},
			want: []string{"backend", "governance", "ops", "security"},
		},
		{
			name: "release",
			task: store.Task{Definition: store.TaskDefinition{ID: "REL-001", ParentID: "EPIC-001", RiskLevel: "release-boundary", Tags: []string{"slice:grouped"}}},
			want: []string{"backend", "governance", "ops", "security"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eval := Evaluate(cfg, Options{Task: tc.task, Parent: &parent, ParentReviews: parentReviews})
			if !eval.GroupReview || !eval.InheritanceBlocked {
				t.Fatalf("eval=%+v, want grouped profile with blocked inheritance", eval)
			}
			if len(eval.EffectiveDomains) != len(tc.want) || len(eval.MissingReviewDomains) != len(tc.want) {
				t.Fatalf("eval=%+v, want missing domains %v", eval, tc.want)
			}
			for i, want := range tc.want {
				if eval.EffectiveDomains[i] != want || eval.MissingReviewDomains[i] != want {
					t.Fatalf("eval=%+v, want domain %s at %d", eval, want, i)
				}
			}
		})
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
