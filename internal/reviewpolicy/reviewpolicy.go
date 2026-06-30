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
		profile, hasProfile = selectDefaultProfile(cfg.ReviewProfiles, task, opts.ChangedPaths)
	}
	if !hasProfile {
		domains := normalizedUnique(task.Definition.ReviewDomains)
		return Evaluation{Requirements: requiredRequirements(domains, "task review_domains"), EffectiveDomains: domains, MissingReviewDomains: MissingDomains(domains, opts.Reviews)}
	}
	profile = applyBoundaryOverlay(profile, task, opts.ChangedPaths)
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

func EffectiveProfiles(configured []config.ReviewProfile) []config.ReviewProfile {
	defaults := DefaultProfiles()
	if len(configured) == 0 {
		return defaults
	}
	overrides := map[string]bool{}
	out := append([]config.ReviewProfile{}, configured...)
	for _, profile := range configured {
		overrides[strings.TrimSpace(profile.Name)] = true
	}
	for _, profile := range defaults {
		if !overrides[profile.Name] {
			out = append(out, profile)
		}
	}
	return out
}

func DefaultProfiles() []config.ReviewProfile {
	return []config.ReviewProfile{
		{
			Name:                     "reversible",
			Mode:                     "advisory",
			MatchRiskLevels:          []string{"reversible"},
			MatchTags:                []string{"reversible", "risk:reversible", "review:reversible"},
			WaiveReviewDomains:       []string{"architecture", "backend", "governance", "ops", "security"},
			SafeIterationZone:        true,
			SafeIterationDefectClass: "product-shape, docs, harness, setup, readback, or prototype defect",
			SafeIterationControl:     "reversible non-live boundary with recorded evidence",
			ExtraReviewerRationale:   "extra reviewers should be requested only when they catch a named defect class or reduce unsafe action",
			ProcessHypothesis:        "reversible product work ships faster with evidence and self-check than with full review ceremony",
			OutcomeMetrics:           []string{"cycle_time", "blocked_time", "defects_caught", "rework_reduced"},
		},
		{
			Name:                    "irreversible",
			Mode:                    "blocking",
			MatchRiskLevels:         []string{"irreversible"},
			MatchTags:               []string{"irreversible", "risk:irreversible", "boundary:irreversible", "credentials", "security", "prod"},
			RequiredReviewDomains:   []string{"architecture", "governance", "ops", "security"},
			NoInheritanceRiskLevels: []string{"high", "irreversible"},
			NoInheritanceTags:       []string{"credentials", "security", "prod", "live", "deploy", "public-exposure"},
			ExtraReviewerRationale:  "irreversible work can expand authority, mutate environments, expose users, or weaken safety controls",
			ProcessHypothesis:       "explicit review on irreversible boundaries avoids unsafe action that evidence-only self-check cannot reverse",
			OutcomeMetrics:          []string{"avoided_unsafe_actions", "defects_caught", "blocked_time"},
		},
		{
			Name:                   "live-boundary",
			Mode:                   "blocking",
			MatchKinds:             []string{"live-window"},
			MatchTags:              []string{"live", "live-window", "boundary:live", "environment:production"},
			RequiredReviewDomains:  []string{"backend", "governance", "ops", "security"},
			NoInheritanceTags:      []string{"credentials", "security", "prod", "live", "deploy", "public-exposure"},
			ExtraReviewerRationale: "live work needs explicit operational, security, and rollback readiness before execution",
			ProcessHypothesis:      "live-boundary approval prevents using drills or UAT as first dependency discovery",
			OutcomeMetrics:         []string{"avoided_unsafe_actions", "defects_caught", "blocked_time"},
		},
		{
			Name:                   "release-boundary",
			Mode:                   "blocking",
			MatchKinds:             []string{"release-risk"},
			MatchTags:              []string{"release", "boundary:release", "deploy", "public-exposure"},
			MatchPaths:             []string{"docs/release", "CHANGELOG", ".goreleaser", "dist/", "scripts/release"},
			RequiredReviewDomains:  []string{"governance", "ops", "security"},
			NoInheritanceTags:      []string{"release", "deploy", "public-exposure"},
			ExtraReviewerRationale: "release work changes public distribution and needs release evidence, rollback, and provenance review",
			ProcessHypothesis:      "release-boundary review catches distribution, provenance, and public-exposure defects before publish",
			OutcomeMetrics:         []string{"defects_caught", "avoided_unsafe_actions", "cycle_time"},
		},
	}
}

func selectDefaultProfile(configured []config.ReviewProfile, task store.Task, changedPaths []string) (config.ReviewProfile, bool) {
	defaults := profileByName(DefaultProfiles())
	overrides := profileByName(configured)
	paths := append(append([]string{}, task.Definition.SourcePaths...), append(task.Definition.TargetPaths, changedPaths...)...)
	tags := normalizedUnique(task.Definition.Tags)
	risk := strings.TrimSpace(task.Definition.RiskLevel)
	kind := strings.TrimSpace(task.Definition.Kind)
	switch {
	case kind == "release-risk" || risk == "release-boundary" || anyOverlap([]string{"release", "boundary:release", "deploy", "public-exposure"}, tags) || anyPathPrefix([]string{"docs/release", "CHANGELOG", ".goreleaser", "dist/", "scripts/release"}, paths):
		return defaultProfile(defaults, overrides, "release-boundary")
	case kind == "live-window" || risk == "live-boundary" || anyOverlap([]string{"live", "live-window", "boundary:live", "environment:production"}, tags):
		return defaultProfile(defaults, overrides, "live-boundary")
	case risk == "irreversible" || anyOverlap([]string{"irreversible", "risk:irreversible", "boundary:irreversible", "credentials", "security", "prod"}, tags):
		return defaultProfile(defaults, overrides, "irreversible")
	case risk == "reversible" || anyOverlap([]string{"reversible", "risk:reversible", "review:reversible"}, tags):
		return defaultProfile(defaults, overrides, "reversible")
	default:
		return config.ReviewProfile{}, false
	}
}

func defaultProfile(defaults, overrides map[string]config.ReviewProfile, name string) (config.ReviewProfile, bool) {
	if _, ok := overrides[name]; ok {
		return config.ReviewProfile{}, false
	}
	profile, ok := defaults[name]
	return profile, ok
}

func applyBoundaryOverlay(profile config.ReviewProfile, task store.Task, changedPaths []string) config.ReviewProfile {
	defaults := profileByName(DefaultProfiles())
	boundaryName := boundaryProfileName(task, changedPaths)
	if boundaryName == "" || profile.Name == boundaryName {
		return profile
	}
	boundary, ok := defaults[boundaryName]
	if !ok {
		return profile
	}
	profile.RequiredReviewDomains = normalizedUnique(append(profile.RequiredReviewDomains, boundary.RequiredReviewDomains...))
	profile.NoInheritanceKinds = normalizedUnique(append(profile.NoInheritanceKinds, boundary.NoInheritanceKinds...))
	profile.NoInheritanceRiskLevels = normalizedUnique(append(profile.NoInheritanceRiskLevels, boundary.NoInheritanceRiskLevels...))
	profile.NoInheritanceTags = normalizedUnique(append(profile.NoInheritanceTags, boundary.NoInheritanceTags...))
	profile.NoInheritancePaths = normalizedUnique(append(profile.NoInheritancePaths, boundary.NoInheritancePaths...))
	return profile
}

func boundaryProfileName(task store.Task, changedPaths []string) string {
	paths := append(append([]string{}, task.Definition.SourcePaths...), append(task.Definition.TargetPaths, changedPaths...)...)
	tags := normalizedUnique(task.Definition.Tags)
	risk := strings.TrimSpace(task.Definition.RiskLevel)
	kind := strings.TrimSpace(task.Definition.Kind)
	switch {
	case kind == "release-risk" || risk == "release-boundary" || anyOverlap([]string{"release", "boundary:release", "deploy", "public-exposure"}, tags) || anyPathPrefix([]string{"docs/release", "CHANGELOG", ".goreleaser", "dist/", "scripts/release"}, paths):
		return "release-boundary"
	case kind == "live-window" || risk == "live-boundary" || anyOverlap([]string{"live", "live-window", "boundary:live", "environment:production"}, tags):
		return "live-boundary"
	case risk == "irreversible" || anyOverlap([]string{"irreversible", "risk:irreversible", "boundary:irreversible", "credentials", "security", "prod"}, tags):
		return "irreversible"
	default:
		return ""
	}
}

func profileByName(profiles []config.ReviewProfile) map[string]config.ReviewProfile {
	out := map[string]config.ReviewProfile{}
	for _, profile := range profiles {
		out[profile.Name] = profile
	}
	return out
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
	if boundary := boundaryProfileName(task, changedPaths); boundary != "" {
		reasons = append(reasons, boundary+" boundary blocks inherited review coverage")
	}
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
