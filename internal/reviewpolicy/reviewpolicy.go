package reviewpolicy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

type Requirement struct {
	Domain string `json:"domain"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type Evaluation struct {
	Profile                  string        `json:"profile,omitempty"`
	Mode                     string        `json:"mode,omitempty"`
	GroupReview              bool          `json:"group_review,omitempty"`
	ExtraReviewerRationale   string        `json:"extra_reviewer_rationale,omitempty"`
	ProcessHypothesis        string        `json:"process_hypothesis,omitempty"`
	OutcomeMetrics           []string      `json:"outcome_metrics,omitempty"`
	SafeIterationZone        bool          `json:"safe_iteration_zone,omitempty"`
	SafeIterationDefectClass string        `json:"safe_iteration_defect_class,omitempty"`
	SafeIterationControl     string        `json:"safe_iteration_control,omitempty"`
	InheritanceBlocked       bool          `json:"inheritance_blocked,omitempty"`
	InheritanceBlockers      []string      `json:"inheritance_blockers,omitempty"`
	Requirements             []Requirement `json:"requirements,omitempty"`
	EffectiveDomains         []string      `json:"effective_domains,omitempty"`
	MissingReviewDomains     []string      `json:"missing_review_domains,omitempty"`
}

type LoopRecommendation struct {
	Detected                 bool     `json:"detected"`
	Reason                   string   `json:"reason,omitempty"`
	FailureChain             []string `json:"failure_chain,omitempty"`
	RealUnknowns             []string `json:"real_unknowns,omitempty"`
	RequiredProofBeforeRetry []string `json:"required_proof_before_retry,omitempty"`
	LighterReviewPlan        string   `json:"lighter_review_plan,omitempty"`
}

type Options struct {
	Task          store.Task
	Parent        *store.Task
	Reviews       []store.Review
	ParentReviews []store.Review
	ChangedPaths  []string
}

func DetectLoop(task store.Task, eval Evaluation, evidence []store.Evidence, reviews []store.Review) LoopRecommendation {
	failures := meaningfulFailures(evidence)
	if len(failures) < 2 {
		return LoopRecommendation{}
	}
	layerCounts := map[string]int{}
	for _, ev := range failures {
		layerCounts[failureLayer(ev)]++
	}
	layer := dominantLayer(layerCounts)
	sameLayer := layer != "" && layerCounts[layer] >= 2
	nearReady := hasNearReadyClaim(evidence)
	approvedThenFailed := hasApprovalBeforeFailure(reviews, failures)
	if !sameLayer && !nearReady && !approvedThenFailed {
		return LoopRecommendation{}
	}
	reasonParts := []string{"repeated meaningful failures"}
	if sameLayer {
		reasonParts = append(reasonParts, "same-layer="+layer)
	}
	if nearReady {
		reasonParts = append(reasonParts, "near-ready-claim")
	}
	if approvedThenFailed {
		reasonParts = append(reasonParts, "approval-without-flow-progress")
	}
	if layer == "" {
		layer = "current failure layer"
	}
	return LoopRecommendation{
		Detected:     true,
		Reason:       "loop detected: " + strings.Join(reasonParts, ", "),
		FailureChain: failureChain(failures),
		RealUnknowns: []string{
			"why " + layer + " keeps failing after near-ready or review progress claims",
			"which end-to-end proof closes the causal unknown before the next retry",
		},
		RequiredProofBeforeRetry: []string{
			"record a causal-reset task or evidence packet that explains the failure chain",
			"record passing proof for " + layer + " before another live or broad retry packet",
		},
		LighterReviewPlan: lighterReviewPlan(eval),
	}
}

func Evaluate(cfg config.Config, opts Options) Evaluation {
	task := opts.Task
	profile, hasProfile := selectProfile(cfg.ReviewProfiles, task, opts.ChangedPaths)
	if !hasProfile {
		domains := normalizedUnique(task.Definition.ReviewDomains)
		return Evaluation{Requirements: requiredRequirements(domains, "task review_domains"), EffectiveDomains: domains, MissingReviewDomains: MissingDomains(domains, opts.Reviews)}
	}
	required := normalizedUnique(append(append([]string{}, task.Definition.ReviewDomains...), profile.RequiredReviewDomains...))
	eval := Evaluation{
		Profile:                  profile.Name,
		Mode:                     firstNonEmpty(profile.Mode, "blocking"),
		GroupReview:              profile.GroupReview,
		ExtraReviewerRationale:   strings.TrimSpace(profile.ExtraReviewerRationale),
		ProcessHypothesis:        strings.TrimSpace(profile.ProcessHypothesis),
		OutcomeMetrics:           normalizedUnique(profile.OutcomeMetrics),
		SafeIterationZone:        profile.SafeIterationZone,
		SafeIterationDefectClass: strings.TrimSpace(profile.SafeIterationDefectClass),
		SafeIterationControl:     strings.TrimSpace(profile.SafeIterationControl),
	}
	eval.InheritanceBlocked, eval.InheritanceBlockers = inheritanceBlocked(profile, task, opts.ChangedPaths)
	approvedParent := approvedDomains(opts.ParentReviews)
	waived := set(profile.WaiveReviewDomains)
	deferred := set(profile.DeferReviewDomains)
	inheritDomains := set(profile.InheritReviewDomains)
	for _, domain := range required {
		req := Requirement{Domain: domain, Status: "required", Reason: fmt.Sprintf("required by review profile %s", profile.Name)}
		switch {
		case waived[domain]:
			req.Status = "waived"
			req.Reason = fmt.Sprintf("waived by review profile %s", profile.Name)
		case deferred[domain]:
			req.Status = "deferred"
			req.Reason = fmt.Sprintf("deferred by review profile %s to parent, epic, release, or launch review", profile.Name)
		case profile.InheritFromParent && !eval.InheritanceBlocked && (len(inheritDomains) == 0 || inheritDomains[domain]) && opts.Parent != nil && approvedParent[domain]:
			req.Status = "inherited"
			req.Reason = fmt.Sprintf("inherited from approved parent task %s", opts.Parent.Definition.ID)
		}
		eval.Requirements = append(eval.Requirements, req)
		if req.Status == "required" {
			eval.EffectiveDomains = append(eval.EffectiveDomains, domain)
		}
	}
	eval.MissingReviewDomains = MissingDomains(eval.EffectiveDomains, opts.Reviews)
	return eval
}

func MissingDomains(domains []string, reviews []store.Review) []string {
	approved := approvedDomains(reviews)
	var missing []string
	for _, domain := range normalizedUnique(domains) {
		if !approved[domain] {
			missing = append(missing, domain)
		}
	}
	return missing
}

func meaningfulFailures(evidence []store.Evidence) []store.Evidence {
	var failures []store.Evidence
	for _, ev := range evidence {
		switch strings.TrimSpace(ev.Result) {
		case "fail", "blocked":
			failures = append(failures, ev)
		}
	}
	return failures
}

func failureLayer(ev store.Evidence) string {
	text := strings.ToLower(strings.Join([]string{ev.ArtifactType, ev.CommandText, ev.ArtifactPath, ev.Notes}, " "))
	for _, layer := range []string{"harness", "browser", "provider", "classifier", "readback", "setup", "review", "ci", "docs", "live-window"} {
		if strings.Contains(text, layer) {
			return layer
		}
	}
	return "unknown"
}

func dominantLayer(counts map[string]int) string {
	best := ""
	bestCount := 0
	for layer, count := range counts {
		if count > bestCount || (count == bestCount && layer < best) {
			best = layer
			bestCount = count
		}
	}
	return best
}

func hasNearReadyClaim(evidence []store.Evidence) bool {
	for _, ev := range evidence {
		text := strings.ToLower(strings.Join([]string{ev.CommandText, ev.ArtifactType, ev.Notes}, " "))
		for _, marker := range []string{"near-ready", "merge-ready", "reviewed", "approval", "approved", "ready for retry", "ready to retry"} {
			if strings.Contains(text, marker) {
				return true
			}
		}
	}
	return false
}

func hasApprovalBeforeFailure(reviews []store.Review, failures []store.Evidence) bool {
	if len(reviews) == 0 || len(failures) == 0 {
		return false
	}
	for _, review := range reviews {
		if strings.TrimSpace(review.Verdict) != "approve" || strings.TrimSpace(review.CreatedAt) == "" {
			continue
		}
		for _, failure := range failures {
			if strings.TrimSpace(failure.CreatedAt) != "" && failure.CreatedAt > review.CreatedAt {
				return true
			}
		}
	}
	return false
}

func failureChain(failures []store.Evidence) []string {
	var chain []string
	for _, ev := range failures {
		piece := "result=" + strings.TrimSpace(ev.Result)
		if layer := failureLayer(ev); layer != "" {
			piece += " layer=" + layer
		}
		if ev.ArtifactType != "" {
			piece += " artifact_type=" + strings.TrimSpace(ev.ArtifactType)
		}
		if ev.ArtifactPath != "" {
			piece += " artifact=" + strings.TrimSpace(ev.ArtifactPath)
		}
		if ev.Notes != "" {
			piece += " notes=" + strings.TrimSpace(ev.Notes)
		}
		chain = append(chain, piece)
	}
	return chain
}

func lighterReviewPlan(eval Evaluation) string {
	if eval.SafeIterationZone {
		control := firstNonEmpty(eval.SafeIterationControl, "safe iteration zone")
		defectClass := firstNonEmpty(eval.SafeIterationDefectClass, "current defect class")
		return "stay inside " + control + " for " + defectClass + " fixes with one accountable review until a boundary exit is requested"
	}
	if eval.Profile != "" {
		return "use the lightest review profile that covers the causal-reset proof, then reserve full review for boundary exit"
	}
	return "create a causal-reset packet and use lightweight review for proof-only fixes before retrying broader flow"
}

func selectProfile(profiles []config.ReviewProfile, task store.Task, changedPaths []string) (config.ReviewProfile, bool) {
	for _, profile := range profiles {
		if matchesProfile(profile, task, changedPaths) {
			return profile, true
		}
	}
	return config.ReviewProfile{}, false
}

func matchesProfile(profile config.ReviewProfile, task store.Task, changedPaths []string) bool {
	if len(profile.MatchKinds) > 0 && !contains(profile.MatchKinds, task.Definition.Kind) {
		return false
	}
	if len(profile.MatchRiskLevels) > 0 && !contains(profile.MatchRiskLevels, task.Definition.RiskLevel) {
		return false
	}
	if len(profile.MatchAuthoringDomains) > 0 && !contains(profile.MatchAuthoringDomains, task.Definition.Role) {
		return false
	}
	if len(profile.MatchOwningDomains) > 0 && !contains(profile.MatchOwningDomains, task.Definition.OwningDomain) {
		return false
	}
	if len(profile.MatchTags) > 0 && !anyOverlap(profile.MatchTags, task.Definition.Tags) {
		return false
	}
	if len(profile.MatchPaths) > 0 && !anyPathPrefix(profile.MatchPaths, append(append([]string{}, task.Definition.SourcePaths...), append(task.Definition.TargetPaths, changedPaths...)...)) {
		return false
	}
	return true
}

func inheritanceBlocked(profile config.ReviewProfile, task store.Task, changedPaths []string) (bool, []string) {
	var reasons []string
	if contains(profile.NoInheritanceKinds, task.Definition.Kind) {
		reasons = append(reasons, "task kind blocks inheritance")
	}
	if contains(profile.NoInheritanceRiskLevels, task.Definition.RiskLevel) {
		reasons = append(reasons, "risk level blocks inheritance")
	}
	if anyOverlap(profile.NoInheritanceTags, task.Definition.Tags) {
		reasons = append(reasons, "task tag blocks inheritance")
	}
	if anyPathPrefix(profile.NoInheritancePaths, append(append([]string{}, task.Definition.SourcePaths...), append(task.Definition.TargetPaths, changedPaths...)...)) {
		reasons = append(reasons, "path blocks inheritance")
	}
	return len(reasons) > 0, reasons
}

func requiredRequirements(domains []string, reason string) []Requirement {
	out := make([]Requirement, 0, len(domains))
	for _, domain := range domains {
		out = append(out, Requirement{Domain: domain, Status: "required", Reason: reason})
	}
	return out
}

func approvedDomains(reviews []store.Review) map[string]bool {
	latest := map[string]string{}
	for _, review := range reviews {
		domain := firstNonEmpty(review.Domain, review.Reviewer)
		if domain == "" {
			continue
		}
		latest[domain] = strings.TrimSpace(review.Verdict)
	}
	approved := map[string]bool{}
	for domain, verdict := range latest {
		approved[domain] = verdict == "approve"
	}
	return approved
}

func normalizedUnique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func set(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func contains(values []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}

func anyOverlap(left, right []string) bool {
	for _, value := range left {
		if contains(right, value) {
			return true
		}
	}
	return false
}

func anyPathPrefix(prefixes, paths []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
				return true
			}
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
