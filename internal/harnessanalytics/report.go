// Package harnessanalytics builds deterministic read-only analysis from harness records.
package harnessanalytics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/harnessrecord"
	"github.com/subashram/fairway/internal/store"
)

// Report is the cited verified-outcome efficiency and trajectory projection for one task.
type Report struct {
	Schema            string              `json:"schema"`
	TaskID            string              `json:"task_id"`
	AuthorityBoundary string              `json:"authority_boundary"`
	Attempts          int                 `json:"attempts"`
	Actions           int                 `json:"recorded_actions"`
	Evaluations       int                 `json:"evaluations"`
	VerifiedOutcomes  int                 `json:"evaluator_backed_outcomes"`
	Usage             UsageSummary        `json:"usage"`
	Cohort            CohortSummary       `json:"cohort"`
	Efficiency        EfficiencySummary   `json:"efficiency"`
	Trajectory        []TrajectoryFinding `json:"trajectory_findings"`
	Limitations       []string            `json:"limitations"`
}

// UsageSummary exposes complete and missing usage denominators.
type UsageSummary struct {
	Events             int    `json:"events"`
	KnownTokenEvents   int    `json:"known_token_events"`
	TotalTokens        *int   `json:"total_tokens,omitempty"`
	KnownElapsedEvents int    `json:"known_elapsed_events"`
	ElapsedSeconds     *int   `json:"elapsed_seconds,omitempty"`
	CostStatus         string `json:"cost_status"`
	CostReason         string `json:"cost_reason"`
	AttributionStatus  string `json:"attribution_status"`
	AttributionReason  string `json:"attribution_reason"`
	ConfidenceStatus   string `json:"confidence_status"`
}

// CohortSummary names the compatibility boundary used for efficiency ratios.
type CohortSummary struct {
	Status     string            `json:"status"`
	Name       string            `json:"name,omitempty"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Missing    []string          `json:"missing,omitempty"`
}

// EfficiencySummary reports ratios only when their denominators are complete.
type EfficiencySummary struct {
	AttemptsPerVerifiedOutcome *float64 `json:"attempts_per_verified_outcome,omitempty"`
	ActionsPerVerifiedOutcome  *float64 `json:"actions_per_verified_outcome,omitempty"`
	TokensPerVerifiedOutcome   *float64 `json:"tokens_per_verified_outcome,omitempty"`
	ElapsedPerVerifiedOutcome  *float64 `json:"elapsed_seconds_per_verified_outcome,omitempty"`
	Status                     string   `json:"status"`
	Missing                    []string `json:"missing"`
}

// TrajectoryFinding is an advisory pattern backed by cited record identities.
type TrajectoryFinding struct {
	Kind               string   `json:"kind"`
	Recommendation     string   `json:"recommendation"`
	Summary            string   `json:"summary"`
	SourceRefs         []string `json:"source_refs"`
	FalsePositiveLimit string   `json:"false_positive_limit"`
}

const defaultNoEvidenceThreshold = 2 * time.Hour

// Build produces a read-only task report without mutating Fairway state.
func Build(ctx context.Context, s *store.Store, taskID string) (Report, error) {
	return BuildAt(ctx, s, taskID, time.Now().UTC(), defaultNoEvidenceThreshold)
}

// BuildAt supports deterministic report generation and tests with a caller-supplied clock.
func BuildAt(ctx context.Context, s *store.Store, taskID string, now time.Time, noEvidenceThreshold time.Duration) (Report, error) {
	records, err := s.HarnessRecordsForTask(ctx, taskID)
	if err != nil {
		return Report{}, err
	}
	usage, err := s.ProviderUsageForTask(ctx, taskID)
	if err != nil {
		return Report{}, err
	}
	_, _, evidence, _, _, err := s.TaskDetail(ctx, taskID)
	if err != nil {
		return Report{}, err
	}
	decisions, err := s.TaskDecisions(ctx, taskID)
	if err != nil {
		return Report{}, err
	}
	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return Report{}, err
	}
	sessions, err := s.Sessions(ctx, false)
	if err != nil {
		return Report{}, err
	}
	report := Report{Schema: "fairway.harness-analysis.v1", TaskID: taskID, AuthorityBoundary: "advisory readback only; cannot redirect execution or change task, evidence, review, policy, or promotion state", Attempts: len(records.Runs), Usage: summarizeUsage(usage), Limitations: []string{"only explicitly ingested harness records and provider usage are observed", "absence of a finding is not evidence that a trajectory is healthy", "cross-provider comparison is unavailable without compatible evaluator, subject, environment, and completeness"}}
	var observations []harnessrecord.Observation
	var evaluations []harnessrecord.EvaluatorResult
	for _, run := range records.Runs {
		observations = append(observations, run.Observations...)
		evaluations = append(evaluations, run.EvaluatorResults...)
	}
	observations = append(observations, records.RunIndependentObservations...)
	evaluations = append(evaluations, records.RunIndependentEvaluations...)
	for _, observation := range observations {
		if observation.ActionFingerprint != "" {
			report.Actions++
		}
	}
	report.Evaluations = len(evaluations)
	for _, evaluation := range evaluations {
		if evaluation.Result != "error" && evaluation.Result != "unavailable" {
			report.VerifiedOutcomes++
		}
	}
	report.Cohort, report.Usage = summarizeCohort(records, observations, evaluations, usage, report.Usage)
	report.Efficiency = summarizeEfficiency(report.Attempts, report.Actions, report.VerifiedOutcomes, report.Usage, report.Cohort)
	report.Trajectory = detectTrajectory(records, observations, evaluations)
	if finding := detectNoNewEvidence(taskID, now, noEvidenceThreshold, sessions, checkpoints, evidence, decisions, records.Runs, observations, evaluations); finding != nil {
		report.Trajectory = append(report.Trajectory, *finding)
		sort.Slice(report.Trajectory, func(i, j int) bool {
			return report.Trajectory[i].Kind+report.Trajectory[i].Summary < report.Trajectory[j].Kind+report.Trajectory[j].Summary
		})
	}
	return report, nil
}

func summarizeUsage(events []store.ProviderUsage) UsageSummary {
	summary := UsageSummary{Events: len(events), CostStatus: "unavailable", CostReason: "provider usage records do not retain one comparable exact cost denominator", AttributionStatus: "unavailable", AttributionReason: "usage has not been correlated to the named run cohort", ConfidenceStatus: "unavailable"}
	tokens, elapsed := 0, 0
	confidences := map[string]bool{}
	for _, event := range events {
		confidences[event.Confidence] = true
		if event.TotalTokens != nil {
			summary.KnownTokenEvents++
			tokens += *event.TotalTokens
		}
		if event.ElapsedSeconds != nil {
			summary.KnownElapsedEvents++
			elapsed += *event.ElapsedSeconds
		}
	}
	if len(events) > 0 && summary.KnownTokenEvents == len(events) {
		summary.TotalTokens = &tokens
	}
	if len(events) > 0 && summary.KnownElapsedEvents == len(events) {
		summary.ElapsedSeconds = &elapsed
	}
	if len(confidences) == 1 {
		for confidence := range confidences {
			summary.ConfidenceStatus = reported(confidence)
		}
	} else if len(confidences) > 1 {
		summary.ConfidenceStatus = "mixed"
	}
	return summary
}

func summarizeCohort(records store.HarnessTaskRecords, observations []harnessrecord.Observation, evaluations []harnessrecord.EvaluatorResult, usageEvents []store.ProviderUsage, usage UsageSummary) (CohortSummary, UsageSummary) {
	cohort := CohortSummary{Status: "compatible", Dimensions: map[string]string{}}
	runSignatures := map[string]bool{}
	runSessions := map[string]bool{}
	for _, record := range records.Runs {
		run := record.Run
		runSignatures[strings.Join([]string{run.SourceID, run.SourceVersion, run.Provider, run.Model, run.Harness}, "|")] = true
		if run.SessionID != "" {
			runSessions[run.SessionID] = true
		}
	}
	actionSignatures := map[string]bool{}
	actionsAttributed := true
	for _, observation := range observations {
		if observation.ActionFingerprint != "" {
			actionSignatures[strings.Join([]string{observation.SourceID, observation.SourceVersion, observation.Completeness}, "|")] = true
			if observation.ExternalRunRef == nil {
				actionsAttributed = false
			}
		}
	}
	evaluationSignatures := map[string]bool{}
	evaluationsAttributed := true
	for _, evaluation := range evaluations {
		evaluationSignatures[strings.Join([]string{evaluation.SourceID, evaluation.SourceVersion, evaluation.EvaluatorID, evaluation.EvaluatorVersion, evaluation.SubjectType, evaluation.SubjectRef, evaluation.Environment, evaluation.Completeness}, "|")] = true
		if evaluation.ExternalRunRef == nil {
			evaluationsAttributed = false
		}
	}
	usageSignatures := map[string]bool{}
	attributedUsage := len(usageEvents) > 0
	for _, event := range usageEvents {
		usageSignatures[strings.Join([]string{event.Provider, event.Model, event.Source, event.Confidence}, "|")] = true
		if event.SessionID == "" || !runSessions[event.SessionID] {
			attributedUsage = false
		}
	}
	for label, signatures := range map[string]map[string]bool{
		"external-run source/provider/model/harness": runSignatures,
		"action source/completeness":                 actionSignatures,
		"evaluator/source/subject/environment":       evaluationSignatures,
		"usage provider/model/source/confidence":     usageSignatures,
	} {
		if len(signatures) > 1 {
			cohort.Status = "incompatible"
			cohort.Missing = append(cohort.Missing, "multiple "+label+" dimensions")
		}
	}
	if len(evaluationSignatures) == 0 {
		cohort.Status = "insufficient"
		cohort.Missing = append(cohort.Missing, "named evaluator cohort")
	} else if len(evaluationSignatures) == 1 {
		for signature := range evaluationSignatures {
			parts := strings.Split(signature, "|")
			cohort.Dimensions["evaluation_source"] = parts[0] + "@" + parts[1]
			cohort.Dimensions["evaluator"] = parts[2] + "@" + parts[3]
			cohort.Dimensions["subject_type"] = parts[4]
			cohort.Dimensions["subject_ref"] = parts[5]
			cohort.Dimensions["environment"] = reported(parts[6])
			cohort.Dimensions["completeness"] = reported(parts[7])
			cohort.Name = strings.Join([]string{cohort.Dimensions["evaluator"], cohort.Dimensions["subject_type"] + ":" + cohort.Dimensions["subject_ref"], cohort.Dimensions["environment"], cohort.Dimensions["completeness"]}, "/")
		}
	}
	if !actionsAttributed || !evaluationsAttributed {
		if cohort.Status == "compatible" {
			cohort.Status = "insufficient_attribution"
		}
		if !actionsAttributed {
			cohort.Missing = append(cohort.Missing, "external-run attribution for every recorded action")
		}
		if !evaluationsAttributed {
			cohort.Missing = append(cohort.Missing, "external-run attribution for every evaluator result")
		}
	}
	sort.Strings(cohort.Missing)
	if attributedUsage {
		usage.AttributionStatus = "correlated"
		usage.AttributionReason = "every usage event names a session referenced by the named external-run cohort"
	} else {
		usage.TotalTokens = nil
		usage.ElapsedSeconds = nil
		if len(usageEvents) == 0 {
			usage.AttributionReason = "no provider usage record exists for the task"
		}
	}
	return cohort, usage
}

func reported(value string) string {
	if value == "" {
		return "unreported"
	}
	return value
}

func summarizeEfficiency(attempts, actions, outcomes int, usage UsageSummary, cohort CohortSummary) EfficiencySummary {
	result := EfficiencySummary{Status: "available"}
	if outcomes == 0 {
		result.Status = "insufficient_outcomes"
		result.Missing = append(result.Missing, "no evaluator-backed outcome")
		return result
	}
	if cohort.Status != "compatible" {
		result.Status = "unavailable"
		result.Missing = append(result.Missing, cohort.Missing...)
		return result
	}
	result.AttemptsPerVerifiedOutcome = ratio(attempts, outcomes)
	result.ActionsPerVerifiedOutcome = ratio(actions, outcomes)
	if usage.TotalTokens != nil {
		result.TokensPerVerifiedOutcome = ratio(*usage.TotalTokens, outcomes)
	} else {
		result.Missing = append(result.Missing, "complete token denominator")
	}
	if usage.ElapsedSeconds != nil {
		result.ElapsedPerVerifiedOutcome = ratio(*usage.ElapsedSeconds, outcomes)
	} else {
		result.Missing = append(result.Missing, "complete elapsed-time denominator")
	}
	if len(result.Missing) > 0 {
		result.Status = "partial"
	}
	if usage.ConfidenceStatus != "exact" && (result.TokensPerVerifiedOutcome != nil || result.ElapsedPerVerifiedOutcome != nil) {
		result.Status = "partial_estimated"
		result.Missing = append(result.Missing, "exact usage confidence; reported usage ratio confidence is "+usage.ConfidenceStatus)
	}
	return result
}

func ratio(value, denominator int) *float64 {
	result := float64(value) / float64(denominator)
	return &result
}

func detectNoNewEvidence(
	taskID string,
	now time.Time,
	threshold time.Duration,
	sessions []store.Session,
	checkpoints []store.Checkpoint,
	evidence []store.Evidence,
	decisions []store.TaskDecision,
	runs []store.HarnessRunRecord,
	observations []harnessrecord.Observation,
	evaluations []harnessrecord.EvaluatorResult,
) *TrajectoryFinding {
	if threshold <= 0 {
		return nil
	}
	var activeSession *store.Session
	for i := range sessions {
		if sessions[i].TaskID == taskID && sessions[i].Status == "running" {
			activeSession = &sessions[i]
			break
		}
	}
	if activeSession == nil {
		return nil
	}
	started, ok := parseTime(activeSession.StartedAt)
	if !ok || now.Sub(started) < threshold {
		return nil
	}
	var latest *store.Checkpoint
	for i := range checkpoints {
		if checkpoints[i].TaskID == taskID {
			latest = &checkpoints[i]
			break
		}
	}
	latestMaterial := started
	latestRef := "session:" + activeSession.ID
	consider := func(raw, ref string) {
		if when, valid := parseTime(raw); valid && when.After(latestMaterial) {
			latestMaterial = when
			latestRef = ref
		}
	}
	for _, item := range evidence {
		consider(item.CreatedAt, "evidence:"+item.CreatedAt)
	}
	for _, item := range decisions {
		consider(item.CreatedAt, fmt.Sprintf("decision:%d", item.ID))
	}
	lastRevision := ""
	for _, item := range runs {
		if item.Run.Revision != "" && item.Run.Revision != lastRevision {
			consider(item.Run.ObservedAt, "revision:"+item.Run.Revision+"@"+item.Run.SourceID+"/"+item.Run.ExternalRunID)
			lastRevision = item.Run.Revision
		}
	}
	for _, item := range observations {
		consider(item.ObservedAt, "observation:"+item.SourceID+"/"+item.ObservationID)
	}
	for _, item := range evaluations {
		consider(item.EvaluatedAt, "evaluation:"+item.SourceID+"/"+item.EvaluationID)
	}
	if now.Sub(latestMaterial) < threshold {
		return nil
	}
	refs := []string{"session:" + activeSession.ID}
	if latest != nil {
		refs = append(refs, fmt.Sprintf("checkpoint:%d", latest.ID))
	}
	if latestRef != refs[0] {
		refs = append(refs, latestRef)
	}
	return &TrajectoryFinding{
		Kind:               "no_new_evidence",
		Recommendation:     "request_input",
		Summary:            fmt.Sprintf("active work has no new durable observation, evaluator result, evidence, decision, or repository revision for at least %s", threshold),
		SourceRefs:         refs,
		FalsePositiveLimit: "work may be progressing in an attached harness that has not emitted a durable observation; confirm with the owner before redirecting",
	}
}

func parseTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	return parsed, err == nil
}

func detectTrajectory(records store.HarnessTaskRecords, observations []harnessrecord.Observation, evaluations []harnessrecord.EvaluatorResult) []TrajectoryFinding {
	var findings []TrajectoryFinding
	type evaluationGroup struct {
		refs       []string
		revisions  map[string]bool
		hypotheses map[string]bool
	}
	groups := map[string]*evaluationGroup{}
	observationByRef := map[string]harnessrecord.Observation{}
	for _, observation := range observations {
		observationByRef[observation.SourceID+"/"+observation.ObservationID] = observation
	}
	runRevision := map[string]string{}
	for _, run := range records.Runs {
		runRevision[run.Run.SourceID+"/"+run.Run.ExternalRunID] = run.Run.Revision
	}
	for _, evaluation := range evaluations {
		if evaluation.Result != "fail" && evaluation.Result != "inconclusive" {
			continue
		}
		key := strings.Join([]string{evaluation.EvaluatorID, evaluation.EvaluatorVersion, evaluation.SubjectType, evaluation.SubjectRef}, "|")
		group := groups[key]
		if group == nil {
			group = &evaluationGroup{revisions: map[string]bool{}, hypotheses: map[string]bool{}}
			groups[key] = group
		}
		group.refs = append(group.refs, "evaluation:"+evaluation.SourceID+"/"+evaluation.EvaluationID)
		if evaluation.RepositoryRev != "" {
			group.revisions[evaluation.RepositoryRev] = true
		} else if evaluation.ExternalRunRef != nil {
			group.revisions[runRevision[evaluation.ExternalRunRef.SourceID+"/"+evaluation.ExternalRunRef.ID]] = true
		}
		if evaluation.ObservationRef != nil {
			if observation, ok := observationByRef[evaluation.ObservationRef.SourceID+"/"+evaluation.ObservationRef.ID]; ok && observation.Hypothesis != "" {
				group.hypotheses[observation.Hypothesis] = true
			}
		}
	}
	for key, group := range groups {
		delete(group.revisions, "")
		if len(group.refs) >= 2 && len(group.revisions) <= 1 && len(group.hypotheses) <= 1 {
			sort.Strings(group.refs)
			findings = append(findings, TrajectoryFinding{Kind: "repeated_evaluator_failure", Recommendation: "reframe_hypothesis", Summary: fmt.Sprintf("%d repeated failed or inconclusive evaluations for %s without a changed revision or hypothesis", len(group.refs), key), SourceRefs: group.refs, FalsePositiveLimit: "a meaningful strategy change not represented by revision or hypothesis records will be missed"})
		}
	}
	actions := map[string][]string{}
	for _, observation := range observations {
		if observation.ActionFingerprint != "" && (observation.Outcome == "rejected" || observation.Outcome == "inconclusive") {
			actions[observation.ActionFingerprint] = append(actions[observation.ActionFingerprint], "observation:"+observation.SourceID+"/"+observation.ObservationID)
		}
	}
	for fingerprint, refs := range actions {
		if len(refs) >= 2 {
			sort.Strings(refs)
			findings = append(findings, TrajectoryFinding{Kind: "repeated_action", Recommendation: "change_execution_profile", Summary: fmt.Sprintf("action fingerprint %s repeated %d times with rejected or inconclusive outcomes", fingerprint, len(refs)), SourceRefs: refs, FalsePositiveLimit: "the fingerprint may hide materially different inputs or environments"})
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].Kind+findings[i].Summary < findings[j].Kind+findings[j].Summary
	})
	return findings
}
