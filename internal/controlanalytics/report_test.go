package controlanalytics

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/reviewpolicy"
	"github.com/subashram/fairway/internal/store"
)

func TestTaskControlFactPartitionsTriggeredFromPassed(t *testing.T) {
	task := store.Task{Definition: store.TaskDefinition{ID: "T-1", Profile: "p", RiskLevel: "medium"}}
	definition := controlDefinition{ID: "review:backend", Kind: "review", Domain: "backend", Family: "quality_gate"}
	eval := reviewpolicy.Evaluation{Requirements: []reviewpolicy.Requirement{{Domain: "backend", Status: "required"}}}
	fact := taskControlFact(task, definition, eval, []store.Review{{Domain: "backend", Verdict: "changes"}, {Domain: "backend", Verdict: "approve"}}, nil, time.Now())
	if fact.ControlState != "observed" || !fact.Triggered || fact.Passed {
		t.Fatalf("fact=%+v", fact)
	}
}

func TestTaskControlFactRetainsExplicitBypassProvenance(t *testing.T) {
	task := store.Task{Definition: store.TaskDefinition{ID: "T-1", Profile: "p", RiskLevel: "medium"}}
	definition := controlDefinition{ID: "review:security", Kind: "review", Domain: "security", Family: "security_invariant"}
	eval := reviewpolicy.Evaluation{Profile: "reversible", Requirements: []reviewpolicy.Requirement{{Domain: "security", Status: "deferred", Reason: "deferred to release review"}}}
	fact := taskControlFact(task, definition, eval, nil, nil, time.Now())
	if fact.ControlState != "bypassed" || fact.BypassReason == "" || fact.BypassAuthority == "" || fact.BypassSource != "review-profile:reversible" {
		t.Fatalf("fact=%+v", fact)
	}
}

func TestTaskControlFactKeepsUnattributedSkippedEvidenceUnknown(t *testing.T) {
	now := time.Now().UTC()
	task := store.Task{Definition: store.TaskDefinition{ID: "T-1", Profile: "p", Kind: "task", RiskLevel: "medium"}}
	definition := controlDefinition{ID: "gate:p:test", Kind: "evidence", Profile: "p", Gate: config.WorkstreamProfileGate{Name: "test", EvidenceType: "test", RequiredEvidenceCount: 1}}
	fact := taskControlFact(task, definition, reviewpolicy.Evaluation{}, nil, []store.Evidence{{ArtifactType: "test", Result: "skipped", CreatedAt: now.Format(time.RFC3339Nano)}}, now)
	if fact.ControlState != "unknown" || fact.BypassReason != "" || fact.BypassAuthority != "" || fact.BypassSource != "" {
		t.Fatalf("fact=%+v", fact)
	}
}

func TestStructuredOutcomeFactsPreserveBoundedSourceIdentity(t *testing.T) {
	promotion := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	outcomes := []store.TaskOutcome{
		{Kind: "incident", OccurredAt: promotion.Add(2 * 24 * time.Hour).Format(time.RFC3339Nano), SourceRef: "incident:INC-7"},
		{Kind: "corrective", OccurredAt: promotion.Add(3 * 24 * time.Hour).Format(time.RFC3339Nano), RelatedTaskID: "T-2"},
		{Kind: "reopen", OccurredAt: promotion.Add(20 * 24 * time.Hour).Format(time.RFC3339Nano), TransitionID: 42},
	}
	facts := structuredOutcomeFacts(outcomes, promotion, 14)
	if len(facts) != 2 || facts[0].SourceRef != "incident:INC-7" || facts[1].RelatedTaskID != "T-2" {
		t.Fatalf("facts=%+v", facts)
	}
	if got := strings.Join(outcomeFactKinds(facts), ","); got != "incident,corrective" {
		t.Fatalf("kinds=%q", got)
	}
}

func TestConfigurationDigestIncludesCohortDefiningProfiles(t *testing.T) {
	one := config.Defaults(t.TempDir())
	two := one
	two.WorkstreamProfiles = append([]config.WorkstreamProfile(nil), one.WorkstreamProfiles...)
	two.WorkstreamProfiles = append(two.WorkstreamProfiles, config.WorkstreamProfile{Name: "new-profile", Gates: []config.WorkstreamProfileGate{{Name: "test", EvidenceType: "test"}}})
	oneDigest, err := configDigest(one)
	if err != nil {
		t.Fatal(err)
	}
	twoDigest, err := configDigest(two)
	if err != nil {
		t.Fatal(err)
	}
	if oneDigest == twoDigest {
		t.Fatal("cohort-defining profile change did not alter digest")
	}
}

func TestAggregateClassifiesBoundedCohorts(t *testing.T) {
	definitions := map[string]controlDefinition{
		"review:backend":  {ID: "review:backend", Mandatory: false},
		"review:security": {ID: "review:security", Mandatory: true},
	}
	thresholds := Thresholds{MinimumSampleSize: 2, MinimumCoverageRatio: 0.75, MaterialOutcomeDelta: 0.2, HighFrictionP90Seconds: 100}
	facts := []TaskFact{
		{TaskID: "T-1", ControlID: "review:backend", Family: "quality_gate", Profile: "p", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, Applicable: true, ControlState: "observed", Mature: true, OutcomeKnown: true},
		{TaskID: "T-2", ControlID: "review:backend", Family: "quality_gate", Profile: "p", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, Applicable: true, ControlState: "observed", Mature: true, OutcomeKnown: true},
		{TaskID: "T-3", ControlID: "review:backend", Family: "quality_gate", Profile: "p", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, Applicable: true, ControlState: "bypassed", Mature: true, OutcomeKnown: true, AnyOutcome: true},
		{TaskID: "T-4", ControlID: "review:backend", Family: "quality_gate", Profile: "p", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, Applicable: true, ControlState: "bypassed", Mature: true, OutcomeKnown: true, AnyOutcome: true},
		{TaskID: "T-5", ControlID: "review:backend", Family: "quality_gate", Profile: "p", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, Applicable: true, ControlState: "unknown", Mature: true, OutcomeKnown: true},
		{TaskID: "T-1", ControlID: "review:security", Family: "security_invariant", Profile: "p", RiskBand: "medium", SizeBand: "small", HorizonDays: 14, Applicable: true, ControlState: "observed", Mature: false},
	}
	rows := Aggregate(facts, definitions, thresholds, ClassificationContext{CommitCoverageRatio: 1, FileCoverageRatio: 1})
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	var backend, security ControlResult
	for _, row := range rows {
		if row.ControlID == "review:backend" {
			backend = row
		} else {
			security = row
		}
	}
	if backend.Classification != "discriminating" || backend.ObservedOutcomeRate != 0 || backend.BypassedOutcomeRate != 1 || backend.OutcomeDelta != -1 {
		t.Fatalf("backend=%+v", backend)
	}
	if backend.KnownControlState != 4 || backend.UnknownControlState != 1 || backend.ControlStateCoverage != 0.8 {
		t.Fatalf("backend denominators=%+v", backend)
	}
	if security.Classification != "mandatory_invariant" || security.RightCensored != 1 {
		t.Fatalf("security=%+v", security)
	}
}

func TestAggregateRequiresMinimumSampleInEachComparisonCohort(t *testing.T) {
	definitions := map[string]controlDefinition{"review:backend": {ID: "review:backend"}}
	thresholds := Thresholds{MinimumSampleSize: 5, MinimumCoverageRatio: 0.8, MaterialOutcomeDelta: 0.1, HighFrictionP90Seconds: 10}
	facts := make([]TaskFact, 0, 10)
	for i := 0; i < 5; i++ {
		facts = append(facts, TaskFact{TaskID: fmt.Sprintf("observed-%d", i), ControlID: "review:backend", Profile: "p", RiskBand: "low", SizeBand: "small", HorizonDays: 7, Applicable: true, ControlState: "observed", Mature: true, OutcomeKnown: true})
	}
	for i := 0; i < 4; i++ {
		facts = append(facts, TaskFact{TaskID: fmt.Sprintf("bypassed-%d", i), ControlID: "review:backend", Profile: "p", RiskBand: "low", SizeBand: "small", HorizonDays: 7, Applicable: true, ControlState: "bypassed", Mature: true, OutcomeKnown: true, AnyOutcome: true})
	}
	row := Aggregate(facts, definitions, thresholds, ClassificationContext{CommitCoverageRatio: 1, FileCoverageRatio: 1})[0]
	if row.Eligible != 9 || row.Observed != 5 || row.Bypassed != 4 || row.Classification != "insufficient_sample" {
		t.Fatalf("row=%+v", row)
	}
}

func TestAggregateKeepsUnknownCensoredAndUnavailableOutsideOutcomeDenominator(t *testing.T) {
	definitions := map[string]controlDefinition{"gate:p:test": {ID: "gate:p:test"}}
	thresholds := Thresholds{MinimumSampleSize: 2, MinimumCoverageRatio: 0.8, MaterialOutcomeDelta: 0.1, HighFrictionP90Seconds: 10}
	facts := []TaskFact{
		{TaskID: "T-1", ControlID: "gate:p:test", Profile: "p", RiskBand: "low", SizeBand: "small", HorizonDays: 7, Applicable: false, ControlState: "not_applicable"},
		{TaskID: "T-2", ControlID: "gate:p:test", Profile: "p", RiskBand: "low", SizeBand: "small", HorizonDays: 7, Applicable: true, ControlState: "unknown", Mature: true, OutcomeKnown: true},
		{TaskID: "T-3", ControlID: "gate:p:test", Profile: "p", RiskBand: "low", SizeBand: "small", HorizonDays: 7, Applicable: true, ControlState: "observed", Mature: false, OutcomeKnown: true},
		{TaskID: "T-4", ControlID: "gate:p:test", Profile: "p", RiskBand: "low", SizeBand: "small", HorizonDays: 7, Applicable: true, ControlState: "bypassed", Mature: true, OutcomeKnown: false},
	}
	row := Aggregate(facts, definitions, thresholds, ClassificationContext{CommitCoverageRatio: 1, FileCoverageRatio: 1})[0]
	if row.NotApplicable != 1 || row.UnknownControlState != 1 || row.RightCensored != 1 || row.OutcomeUnavailable != 1 || row.Eligible != 0 {
		t.Fatalf("row=%+v", row)
	}
	if row.Classification != "insufficient_coverage" {
		t.Fatalf("classification=%s", row.Classification)
	}
}

func TestAggregateSuppressesRankingWhenCommitCoverageIsLow(t *testing.T) {
	definitions := map[string]controlDefinition{"review:backend": {ID: "review:backend"}}
	thresholds := Thresholds{MinimumSampleSize: 2, MinimumCoverageRatio: 0.8, MaterialOutcomeDelta: 0.1, HighFrictionP90Seconds: 10}
	facts := []TaskFact{
		{TaskID: "T-1", ControlID: "review:backend", Profile: "p", RiskBand: "low", SizeBand: "small", HorizonDays: 7, Applicable: true, ControlState: "observed", Mature: true, OutcomeKnown: true},
		{TaskID: "T-2", ControlID: "review:backend", Profile: "p", RiskBand: "low", SizeBand: "small", HorizonDays: 7, Applicable: true, ControlState: "bypassed", Mature: true, OutcomeKnown: true, AnyOutcome: true},
	}
	row := Aggregate(facts, definitions, thresholds, ClassificationContext{CommitCoverageRatio: 0.5, FileCoverageRatio: 1})[0]
	if row.Classification != "insufficient_coverage" {
		t.Fatalf("row=%+v", row)
	}
}
