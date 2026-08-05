package controlanalytics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/audit"
	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/evidencemodel"
	"github.com/subashram/fairway/internal/reviewpolicy"
	"github.com/subashram/fairway/internal/store"
)

const Schema = "fairway.control-effectiveness.v1"

type Options struct {
	Since   time.Duration
	Profile string
	Control string
	Now     time.Time
}

type Thresholds struct {
	MinimumSampleSize      int     `json:"minimum_sample_size"`
	MinimumCoverageRatio   float64 `json:"minimum_coverage_ratio"`
	MaterialOutcomeDelta   float64 `json:"material_outcome_delta"`
	HighFrictionP90Seconds int     `json:"high_friction_p90_seconds"`
}

type Report struct {
	Schema                string                        `json:"schema"`
	Advisory              bool                          `json:"advisory"`
	AuthorityBoundary     string                        `json:"authority_boundary"`
	AsOf                  string                        `json:"as_of"`
	Since                 string                        `json:"since"`
	AnalyzedTip           string                        `json:"analyzed_tip"`
	Profile               string                        `json:"profile,omitempty"`
	Control               string                        `json:"control,omitempty"`
	ConfigurationRevision string                        `json:"configuration_revision"`
	ConfigurationDigest   string                        `json:"configuration_digest"`
	Thresholds            Thresholds                    `json:"thresholds"`
	Coverage              Coverage                      `json:"coverage"`
	Exclusions            []config.ControlPathExclusion `json:"exclusions,omitempty"`
	Controls              []ControlResult               `json:"controls"`
	TaskFacts             []TaskFact                    `json:"task_facts"`
	Limitations           []string                      `json:"limitations"`
}

type Coverage struct {
	EligibleCommits         int     `json:"eligible_commits"`
	CoveredCommits          int     `json:"covered_commits"`
	ExplicitlyLinkedCommits int     `json:"explicitly_linked_commits"`
	CommitCoverageRatio     float64 `json:"commit_coverage_ratio"`
	EligibleChangedFiles    int     `json:"eligible_changed_files"`
	CoveredChangedFiles     int     `json:"covered_changed_files"`
	FileCoverageRatio       float64 `json:"file_coverage_ratio"`
	ExcludedMergeCommits    int     `json:"excluded_merge_commits"`
	ExcludedOnlyCommits     int     `json:"excluded_only_commits"`
	ExcludedChangedFiles    int     `json:"excluded_changed_files"`
}

type TaskFact struct {
	TaskID            string        `json:"task_id"`
	ControlID         string        `json:"control_id"`
	Family            string        `json:"family"`
	Profile           string        `json:"profile,omitempty"`
	RiskBand          string        `json:"risk_band"`
	SizeBand          string        `json:"size_band"`
	HorizonDays       int           `json:"horizon_days"`
	Applicable        bool          `json:"applicable"`
	ControlState      string        `json:"control_state"`
	BypassReason      string        `json:"bypass_reason,omitempty"`
	BypassAuthority   string        `json:"bypass_authority,omitempty"`
	BypassSource      string        `json:"bypass_source,omitempty"`
	Mature            bool          `json:"mature"`
	OutcomeKnown      bool          `json:"outcome_known"`
	AnyOutcome        bool          `json:"any_outcome"`
	OutcomeKinds      []string      `json:"outcome_kinds,omitempty"`
	OutcomeFacts      []OutcomeFact `json:"outcome_facts,omitempty"`
	Triggered         bool          `json:"triggered"`
	Passed            bool          `json:"passed"`
	FrictionAvailable bool          `json:"friction_available"`
	FrictionState     string        `json:"friction_state"`
	FrictionSource    string        `json:"friction_source,omitempty"`
	FrictionSeconds   int           `json:"friction_seconds,omitempty"`
	FrictionSampleIDs []int64       `json:"friction_sample_ids,omitempty"`
	FrictionReasons   []string      `json:"friction_reasons,omitempty"`
	PromotionCommit   string        `json:"promotion_commit,omitempty"`
	EligibleFiles     []string      `json:"eligible_files,omitempty"`
	ExcludedFiles     []string      `json:"excluded_files,omitempty"`
	TouchCommits      []string      `json:"touch_commits,omitempty"`
}

type OutcomeFact struct {
	Kind          string `json:"kind"`
	OccurredAt    string `json:"occurred_at,omitempty"`
	SourceRef     string `json:"source_ref,omitempty"`
	RelatedTaskID string `json:"related_task_id,omitempty"`
	TransitionID  int64  `json:"transition_id,omitempty"`
}

type ControlResult struct {
	ControlID             string   `json:"control_id"`
	Family                string   `json:"family"`
	Profile               string   `json:"profile,omitempty"`
	RiskBand              string   `json:"risk_band"`
	SizeBand              string   `json:"size_band"`
	HorizonDays           int      `json:"horizon_days"`
	Classification        string   `json:"classification"`
	MandatoryInvariant    bool     `json:"mandatory_invariant"`
	Applicable            int      `json:"applicable"`
	NotApplicable         int      `json:"not_applicable"`
	KnownControlState     int      `json:"known_control_state"`
	UnknownControlState   int      `json:"unknown_control_state"`
	RightCensored         int      `json:"right_censored"`
	OutcomeUnavailable    int      `json:"outcome_unavailable"`
	Eligible              int      `json:"eligible"`
	Observed              int      `json:"observed"`
	Bypassed              int      `json:"bypassed"`
	ObservedOutcomes      int      `json:"observed_outcomes"`
	BypassedOutcomes      int      `json:"bypassed_outcomes"`
	ObservedOutcomeRate   float64  `json:"observed_outcome_rate"`
	BypassedOutcomeRate   float64  `json:"bypassed_outcome_rate"`
	OutcomeDelta          float64  `json:"outcome_delta"`
	Triggered             int      `json:"triggered"`
	Passed                int      `json:"passed"`
	TriggerYield          float64  `json:"trigger_yield"`
	ControlStateCoverage  float64  `json:"control_state_coverage"`
	FrictionAvailable     bool     `json:"friction_available"`
	FrictionSamples       int      `json:"friction_samples"`
	FrictionOpen          int      `json:"friction_open"`
	FrictionUnavailable   int      `json:"friction_unavailable"`
	FrictionMissing       int      `json:"friction_missing"`
	FrictionLegacy        int      `json:"friction_legacy"`
	FrictionMedianSeconds int      `json:"friction_median_seconds,omitempty"`
	FrictionP90Seconds    int      `json:"friction_p90_seconds,omitempty"`
	TaskIDs               []string `json:"task_ids"`
	Limitations           []string `json:"limitations"`
}

type controlDefinition struct {
	ID        string
	Family    string
	Kind      string
	Domain    string
	Profile   string
	Gate      config.WorkstreamProfileGate
	Mandatory bool
}

type ClassificationContext struct {
	CommitCoverageRatio float64
	FileCoverageRatio   float64
}

func Build(ctx context.Context, cfg config.Config, root string, s *store.Store, opts Options) (Report, error) {
	if opts.Since <= 0 {
		return Report{}, fmt.Errorf("control report since duration must be positive")
	}
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return Report{}, err
	}
	allTaskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		allTaskIDs = append(allTaskIDs, task.Definition.ID)
	}
	transitionsByTask, err := s.TransitionsByTaskIDs(ctx, allTaskIDs)
	if err != nil {
		return Report{}, err
	}
	cutoff := now.Add(-opts.Since)
	var population []store.Task
	promotionAtByTask := map[string]string{}
	for _, task := range tasks {
		promotionAt := strings.TrimSpace(task.CompletedAt)
		if promotionAt == "" {
			promotionAt = latestDoneTransitionAt(transitionsByTask[task.Definition.ID])
		}
		if promotionAt == "" || strings.TrimSpace(task.CommitSHA) == "" {
			continue
		}
		completed, err := time.Parse(time.RFC3339Nano, promotionAt)
		if err != nil || completed.Before(cutoff) || completed.After(now) {
			continue
		}
		if opts.Profile != "" && task.Definition.Profile != opts.Profile {
			continue
		}
		task.CompletedAt = promotionAt
		population = append(population, task)
		promotionAtByTask[task.Definition.ID] = promotionAt
	}
	taskIDs := make([]string, 0, len(population))
	for _, task := range population {
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	coverageReport, err := audit.BuildWorkCoverageReport(ctx, cfg, root, s, audit.WorkCoverageOptions{SinceDuration: opts.Since, TaskIDs: taskIDs, RestrictTaskIDs: true, PromotionAtByTask: promotionAtByTask, Now: now})
	if err != nil {
		return Report{}, err
	}
	evidenceByTask, err := s.EvidenceByTaskIDs(ctx, taskIDs)
	if err != nil {
		return Report{}, err
	}
	reviewsByTask, err := s.ReviewsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return Report{}, err
	}
	outcomes, err := s.TaskOutcomes(ctx, "")
	if err != nil {
		return Report{}, err
	}
	outcomesByTask := map[string][]store.TaskOutcome{}
	for _, outcome := range outcomes {
		outcomesByTask[outcome.TaskID] = append(outcomesByTask[outcome.TaskID], outcome)
	}
	frictionSamples, err := s.ControlFrictionSamples(ctx, "")
	if err != nil {
		return Report{}, err
	}
	frictionByTaskControl := map[string][]store.ControlFrictionSample{}
	for _, sample := range frictionSamples {
		key := sample.TaskID + "\x00" + sample.ControlID
		frictionByTaskControl[key] = append(frictionByTaskControl[key], sample)
	}
	taskByID := map[string]store.Task{}
	for _, task := range tasks {
		taskByID[task.Definition.ID] = task
	}
	touchByTask := map[string]audit.PostPromotionTouchFact{}
	for _, fact := range coverageReport.TouchFacts {
		touchByTask[fact.TaskID] = fact
	}
	mandatory := stringSet(cfg.ControlEffectiveness.MandatoryControlIDs)
	evaluations := map[string]reviewpolicy.Evaluation{}
	definitions := map[string]controlDefinition{}
	for _, task := range population {
		var parent *store.Task
		if candidate, ok := taskByID[task.Definition.ParentID]; ok {
			copy := candidate
			parent = &copy
		}
		parentReviews := reviewsByTask[task.Definition.ParentID]
		eval := reviewpolicy.Evaluate(cfg, reviewpolicy.Options{Task: task, Parent: parent, Reviews: reviewsByTask[task.Definition.ID], ParentReviews: parentReviews, ChangedPaths: append(append([]string{}, touchByTask[task.Definition.ID].EligibleFiles...), touchByTask[task.Definition.ID].ExcludedFiles...)})
		evaluations[task.Definition.ID] = eval
		for _, req := range eval.Requirements {
			id := "review:" + req.Domain
			definitions[id] = controlDefinition{ID: id, Family: "quality_gate", Kind: "review", Domain: req.Domain, Mandatory: mandatory[id]}
		}
	}
	for _, profile := range cfg.WorkstreamProfiles {
		for _, gate := range profile.Gates {
			applicableInPopulation := false
			for _, task := range population {
				if task.Definition.Profile == profile.Name && (len(gate.TaskKinds) == 0 || contains(gate.TaskKinds, task.Definition.Kind)) {
					applicableInPopulation = true
					break
				}
			}
			if !applicableInPopulation {
				continue
			}
			id := "gate:" + profile.Name + ":" + gate.Name
			family := "quality_gate"
			if strings.Contains(strings.ToLower(gate.Name+" "+gate.Group+" "+gate.Description), "security") {
				family = "security_invariant"
			}
			definitions[id] = controlDefinition{ID: id, Family: family, Kind: "evidence", Profile: profile.Name, Gate: gate, Mandatory: mandatory[id]}
		}
	}
	definitionList := make([]controlDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if opts.Control == "" || definition.ID == opts.Control {
			definitionList = append(definitionList, definition)
		}
	}
	sort.Slice(definitionList, func(i, j int) bool { return definitionList[i].ID < definitionList[j].ID })
	var facts []TaskFact
	for _, task := range population {
		for _, definition := range definitionList {
			base := taskControlFact(task, definition, evaluations[task.Definition.ID], reviewsByTask[task.Definition.ID], evidenceByTask[task.Definition.ID], now)
			applyControlFriction(&base, frictionByTaskControl[task.Definition.ID+"\x00"+definition.ID])
			touch := touchByTask[task.Definition.ID]
			base.SizeBand = sizeBand(len(touch.EligibleFiles))
			base.PromotionCommit = touch.PromotionCommit
			base.EligibleFiles = append([]string(nil), touch.EligibleFiles...)
			base.ExcludedFiles = append([]string(nil), touch.ExcludedFiles...)
			promotionAt, _ := time.Parse(time.RFC3339, touch.PromotionAt)
			for _, days := range []int{7, 14, 30} {
				fact := base
				fact.HorizonDays = days
				fact.Mature = !promotionAt.IsZero() && !now.Before(promotionAt.Add(time.Duration(days)*24*time.Hour))
				window, windowFound := touchWindow(touch.Windows, days)
				outcomeFacts := structuredOutcomeFacts(outcomesByTask[task.Definition.ID], promotionAt, days)
				kinds := outcomeFactKinds(outcomeFacts)
				if windowFound && touch.Available {
					fact.OutcomeKnown = true
					if window.Touched {
						kinds = append(kinds, "post_promotion_touch")
						fact.TouchCommits = append([]string(nil), window.TouchCommits...)
					}
				} else if len(kinds) > 0 {
					fact.OutcomeKnown = true
				}
				fact.OutcomeKinds = uniqueSorted(kinds)
				fact.OutcomeFacts = outcomeFacts
				fact.AnyOutcome = len(fact.OutcomeKinds) > 0
				facts = append(facts, fact)
			}
		}
	}
	thresholds := normalizedThresholds(cfg.ControlEffectiveness)
	denom := coverageReport.Denominators
	coverage := Coverage{EligibleCommits: denom.EligibleCommits, CoveredCommits: denom.CoveredCommits, ExplicitlyLinkedCommits: denom.ExplicitlyLinkedCommits, CommitCoverageRatio: ratio(denom.CoveredCommits, denom.EligibleCommits), EligibleChangedFiles: denom.EligibleChangedFiles, CoveredChangedFiles: denom.CoveredChangedFiles, FileCoverageRatio: ratio(denom.CoveredChangedFiles, denom.EligibleChangedFiles), ExcludedMergeCommits: denom.ExcludedMergeCommits, ExcludedOnlyCommits: denom.ExcludedOnlyCommits, ExcludedChangedFiles: denom.ExcludedChangedFiles}
	results := Aggregate(facts, definitions, thresholds, ClassificationContext{CommitCoverageRatio: coverage.CommitCoverageRatio, FileCoverageRatio: coverage.FileCoverageRatio})
	digest, err := configDigest(cfg)
	if err != nil {
		return Report{}, err
	}
	revision := strings.TrimSpace(cfg.ControlEffectiveness.Revision)
	if revision == "" {
		revision = "unversioned"
	}
	report := Report{
		Schema: Schema, Advisory: true,
		AuthorityBoundary: "observational report only; cannot approve, waive, merge, deploy, release, or mutate policy",
		AsOf:              now.Format(time.RFC3339), Since: opts.Since.String(), AnalyzedTip: coverageReport.AnalyzedTip,
		Profile: opts.Profile, Control: opts.Control, ConfigurationRevision: revision, ConfigurationDigest: digest,
		Thresholds: thresholds,
		Coverage:   coverage,
		Exclusions: append([]config.ControlPathExclusion(nil), cfg.ControlEffectiveness.PathExclusions...), Controls: results, TaskFacts: facts,
		Limitations: []string{
			"Observational associations do not establish causal impact.",
			"Post-promotion same-file touches are a rework proxy and may include planned follow-up or adjacent work.",
			"Only explicit review waivers or deferrals with retained configuration authority form the bypass cohort; skipped evidence without durable actor and stable row identity remains unknown.",
			"Review and configured workstream evidence gates are covered in v1; rule-pack trigger telemetry requires later instrumentation.",
			"Explicit friction samples distinguish measured, open, unavailable, and missing states; evidence duration retained from older records is labeled measured_legacy.",
			"Model, provider, team, and product drift may remain inside a bounded time window.",
		},
	}
	return report, nil
}

func taskControlFact(task store.Task, definition controlDefinition, eval reviewpolicy.Evaluation, reviews []store.Review, evidence []store.Evidence, now time.Time) TaskFact {
	fact := TaskFact{TaskID: task.Definition.ID, ControlID: definition.ID, Family: definition.Family, Profile: task.Definition.Profile, RiskBand: fallback(task.Definition.RiskLevel, "unknown"), SizeBand: "unknown", ControlState: "not_applicable", FrictionState: "missing"}
	switch definition.Kind {
	case "review":
		var requirement *reviewpolicy.Requirement
		for i := range eval.Requirements {
			if eval.Requirements[i].Domain == definition.Domain {
				requirement = &eval.Requirements[i]
				break
			}
		}
		if requirement == nil {
			return fact
		}
		fact.Applicable = true
		switch requirement.Status {
		case "waived", "deferred":
			fact.ControlState = "bypassed"
			fact.BypassReason = requirement.Reason
			fact.BypassAuthority = "reviewed project review-profile configuration"
			fact.BypassSource = "review-profile:" + fallback(eval.Profile, "task-review-domains")
		case "inherited":
			fact.ControlState, fact.Passed = "observed", true
		default:
			fact.ControlState = "unknown"
			for _, review := range reviews {
				if review.Domain != definition.Domain {
					continue
				}
				switch review.Verdict {
				case "approve":
					fact.ControlState, fact.Passed = "observed", true
				case "changes", "changes_requested":
					fact.ControlState, fact.Triggered = "observed", true
				}
			}
			if fact.Triggered {
				fact.Passed = false
			}
		}
	case "evidence":
		gate := definition.Gate
		if task.Definition.Profile != definition.Profile || (len(gate.TaskKinds) > 0 && !contains(gate.TaskKinds, task.Definition.Kind)) {
			return fact
		}
		fact.Applicable, fact.ControlState = true, "unknown"
		gateEvaluation := evidencemodel.EvaluateGate(gate, evidence, now)
		var durations []int
		observed := false
		for _, item := range evidence {
			if gate.EvidenceType != "" && item.ArtifactType != gate.EvidenceType {
				continue
			}
			if item.Result == "skipped" {
				continue
			}
			observed = true
			if item.Result == "fail" || item.Result == "blocked" || item.Result == "partial" {
				fact.Triggered = true
			}
			if item.DurationSeconds != nil {
				durations = append(durations, *item.DurationSeconds)
			}
		}
		if observed {
			fact.ControlState = "observed"
			fact.Passed = gateEvaluation.Satisfied
			if !gateEvaluation.Satisfied {
				fact.Triggered = true
			}
		}
		if len(durations) > 0 {
			fact.FrictionAvailable = true
			fact.FrictionState = "measured_legacy"
			fact.FrictionSource = "evidence_duration"
			for _, duration := range durations {
				fact.FrictionSeconds += duration
			}
		}
		if fact.Triggered {
			fact.Passed = false
		}
	}
	return fact
}

func applyControlFriction(fact *TaskFact, samples []store.ControlFrictionSample) {
	if len(samples) == 0 {
		return
	}
	var measured, open, unavailable bool
	var seconds int
	for _, sample := range samples {
		fact.FrictionSampleIDs = append(fact.FrictionSampleIDs, sample.ID)
		switch sample.Status {
		case "resolved":
			start, startErr := time.Parse(time.RFC3339Nano, sample.StartedAt)
			end, endErr := time.Parse(time.RFC3339Nano, sample.ResolvedAt)
			if startErr == nil && endErr == nil && !end.Before(start) {
				measured = true
				seconds += int(end.Sub(start).Seconds())
			}
		case "open":
			open = true
		case "unavailable":
			unavailable = true
			if sample.Reason != "" {
				fact.FrictionReasons = append(fact.FrictionReasons, sample.Reason)
			}
		}
	}
	fact.FrictionSampleIDs = uniqueInt64(fact.FrictionSampleIDs)
	fact.FrictionReasons = uniqueSorted(fact.FrictionReasons)
	switch {
	case measured:
		fact.FrictionAvailable = true
		fact.FrictionState = "measured"
		fact.FrictionSource = "control_friction_samples"
		fact.FrictionSeconds = seconds
	case open:
		fact.FrictionAvailable = false
		fact.FrictionState = "open"
		fact.FrictionSource = "control_friction_samples"
		fact.FrictionSeconds = 0
	case unavailable:
		fact.FrictionAvailable = false
		fact.FrictionState = "unavailable"
		fact.FrictionSource = "control_friction_samples"
		fact.FrictionSeconds = 0
	}
}

func Aggregate(facts []TaskFact, definitions map[string]controlDefinition, thresholds Thresholds, context ClassificationContext) []ControlResult {
	groups := map[string]*ControlResult{}
	friction := map[string][]int{}
	for _, fact := range facts {
		key := strings.Join([]string{fact.ControlID, fact.Profile, fact.RiskBand, fact.SizeBand, fmt.Sprint(fact.HorizonDays)}, "\x00")
		row := groups[key]
		if row == nil {
			row = &ControlResult{ControlID: fact.ControlID, Family: fact.Family, Profile: fact.Profile, RiskBand: fact.RiskBand, SizeBand: fact.SizeBand, HorizonDays: fact.HorizonDays, MandatoryInvariant: definitions[fact.ControlID].Mandatory}
			groups[key] = row
		}
		row.TaskIDs = append(row.TaskIDs, fact.TaskID)
		if !fact.Applicable {
			row.NotApplicable++
			continue
		}
		row.Applicable++
		if fact.FrictionAvailable {
			friction[key] = append(friction[key], fact.FrictionSeconds)
		}
		switch fact.FrictionState {
		case "open":
			row.FrictionOpen++
		case "unavailable":
			row.FrictionUnavailable++
		case "missing":
			row.FrictionMissing++
		case "measured_legacy":
			row.FrictionLegacy++
		}
		if fact.ControlState != "observed" && fact.ControlState != "bypassed" {
			row.UnknownControlState++
			continue
		}
		row.KnownControlState++
		if !fact.Mature {
			row.RightCensored++
			continue
		}
		if !fact.OutcomeKnown {
			row.OutcomeUnavailable++
			continue
		}
		row.Eligible++
		if fact.ControlState == "observed" {
			row.Observed++
			if fact.AnyOutcome {
				row.ObservedOutcomes++
			}
			if fact.Triggered {
				row.Triggered++
			}
			if fact.Passed {
				row.Passed++
			}
		} else {
			row.Bypassed++
			if fact.AnyOutcome {
				row.BypassedOutcomes++
			}
		}
	}
	results := make([]ControlResult, 0, len(groups))
	for key, row := range groups {
		if row.Applicable == 0 {
			continue
		}
		row.ControlStateCoverage = ratio(row.KnownControlState, row.Applicable)
		row.ObservedOutcomeRate = ratio(row.ObservedOutcomes, row.Observed)
		row.BypassedOutcomeRate = ratio(row.BypassedOutcomes, row.Bypassed)
		row.OutcomeDelta = row.ObservedOutcomeRate - row.BypassedOutcomeRate
		row.TriggerYield = ratio(row.Triggered, row.Observed)
		values := friction[key]
		if len(values) > 0 {
			sort.Ints(values)
			row.FrictionAvailable, row.FrictionSamples = true, len(values)
			row.FrictionMedianSeconds = percentile(values, 0.5)
			row.FrictionP90Seconds = percentile(values, 0.9)
		}
		row.Classification, row.Limitations = classify(*row, thresholds, context)
		row.TaskIDs = uniqueSorted(row.TaskIDs)
		results = append(results, *row)
	}
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.ControlID != b.ControlID {
			return a.ControlID < b.ControlID
		}
		if a.Profile != b.Profile {
			return a.Profile < b.Profile
		}
		if a.RiskBand != b.RiskBand {
			return a.RiskBand < b.RiskBand
		}
		if a.SizeBand != b.SizeBand {
			return a.SizeBand < b.SizeBand
		}
		return a.HorizonDays < b.HorizonDays
	})
	return results
}

func uniqueInt64(values []int64) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func classify(row ControlResult, thresholds Thresholds, context ClassificationContext) (string, []string) {
	if row.MandatoryInvariant {
		return "mandatory_invariant", []string{"Classification is authoritative configuration and cannot be relaxed by observational results."}
	}
	if context.CommitCoverageRatio < thresholds.MinimumCoverageRatio || context.FileCoverageRatio < thresholds.MinimumCoverageRatio {
		return "insufficient_coverage", []string{"Commit/task or changed-file coverage is below the configured threshold; outcome ranking is suppressed."}
	}
	if row.ControlStateCoverage < thresholds.MinimumCoverageRatio {
		return "insufficient_coverage", []string{"Known observed/bypassed control-state coverage is below the configured threshold."}
	}
	if row.Observed < thresholds.MinimumSampleSize || row.Bypassed < thresholds.MinimumSampleSize {
		return "insufficient_sample", []string{"A contemporaneous observed-versus-explicit-bypass comparison requires each cohort to meet the configured minimum sample."}
	}
	if row.OutcomeDelta <= -thresholds.MaterialOutcomeDelta {
		return "discriminating", []string{"Observed work has a lower outcome rate than the explicit bypass cohort in this bounded stratum; this is association, not causation."}
	}
	if row.FrictionAvailable && row.FrictionP90Seconds >= thresholds.HighFrictionP90Seconds {
		return "high_friction", []string{"Measured attributable p90 cost exceeds the configured threshold without a matching outcome signal."}
	}
	return "redesign_candidate", []string{"No measurable incremental signal under the current sample, coverage, risk controls, and outcome definition."}
}

func normalizedThresholds(control config.ControlEffectivenessConfig) Thresholds {
	result := Thresholds{MinimumSampleSize: control.MinimumSampleSize, MinimumCoverageRatio: control.MinimumCoverageRatio, MaterialOutcomeDelta: control.MaterialOutcomeDelta, HighFrictionP90Seconds: control.HighFrictionP90Seconds}
	if result.MinimumSampleSize == 0 {
		result.MinimumSampleSize = 5
	}
	if result.MinimumCoverageRatio == 0 {
		result.MinimumCoverageRatio = 0.8
	}
	if result.MaterialOutcomeDelta == 0 {
		result.MaterialOutcomeDelta = 0.1
	}
	if result.HighFrictionP90Seconds == 0 {
		result.HighFrictionP90Seconds = 900
	}
	return result
}

func configDigest(cfg config.Config) (string, error) {
	control := cfg.ControlEffectiveness
	effective := struct {
		Revision            string                        `json:"revision"`
		Thresholds          Thresholds                    `json:"thresholds"`
		MandatoryControlIDs []string                      `json:"mandatory_control_ids,omitempty"`
		PathExclusions      []config.ControlPathExclusion `json:"path_exclusions,omitempty"`
		ReviewProfiles      []config.ReviewProfile        `json:"review_profiles"`
		WorkstreamProfiles  []config.WorkstreamProfile    `json:"workstream_profiles"`
		ReviewDomainAliases map[string]string             `json:"review_domain_aliases,omitempty"`
	}{
		Revision:            fallback(strings.TrimSpace(control.Revision), "unversioned"),
		Thresholds:          normalizedThresholds(control),
		MandatoryControlIDs: uniqueSorted(control.MandatoryControlIDs),
		PathExclusions:      append([]config.ControlPathExclusion(nil), control.PathExclusions...),
		ReviewProfiles:      reviewpolicy.EffectiveProfiles(cfg.ReviewProfiles),
		WorkstreamProfiles:  append([]config.WorkstreamProfile(nil), cfg.WorkstreamProfiles...),
		ReviewDomainAliases: cfg.ReviewDomainAliases,
	}
	data, err := json.Marshal(effective)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func structuredOutcomeFacts(outcomes []store.TaskOutcome, promotionAt time.Time, days int) []OutcomeFact {
	if promotionAt.IsZero() {
		return nil
	}
	deadline := promotionAt.Add(time.Duration(days) * 24 * time.Hour)
	var facts []OutcomeFact
	for _, outcome := range outcomes {
		at, err := time.Parse(time.RFC3339Nano, outcome.OccurredAt)
		if err == nil && !at.Before(promotionAt) && !at.After(deadline) {
			facts = append(facts, OutcomeFact{
				Kind: outcome.Kind, OccurredAt: outcome.OccurredAt, SourceRef: outcome.SourceRef,
				RelatedTaskID: outcome.RelatedTaskID, TransitionID: outcome.TransitionID,
			})
		}
	}
	return facts
}

func outcomeFactKinds(facts []OutcomeFact) []string {
	kinds := make([]string, 0, len(facts))
	for _, fact := range facts {
		kinds = append(kinds, fact.Kind)
	}
	return kinds
}

func latestDoneTransitionAt(transitions []store.Transition) string {
	for i := len(transitions) - 1; i >= 0; i-- {
		if transitions[i].ToStatus == "done" {
			return transitions[i].At
		}
	}
	return ""
}

func touchWindow(windows []audit.TouchWindowFact, days int) (audit.TouchWindowFact, bool) {
	for _, window := range windows {
		if window.Days == days {
			return window, true
		}
	}
	return audit.TouchWindowFact{}, false
}

func sizeBand(files int) string {
	switch {
	case files == 0:
		return "unknown"
	case files <= 3:
		return "small"
	case files <= 10:
		return "medium"
	default:
		return "large"
	}
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
func percentile(values []int, p float64) int {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1)*p + 0.999999)
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}
func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[strings.TrimSpace(value)] = true
	}
	return out
}
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
