package deliveryresources

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/store"
)

const staleAfter = 14 * 24 * time.Hour

var (
	commitPattern  = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	versionPattern = regexp.MustCompile(`\bv[0-9]+(?:\.[0-9]+){1,3}(?:[-+][A-Za-z0-9_.-]+)?\b`)
)

type Resource struct {
	Project             string   `json:"project,omitempty"`
	Type                string   `json:"type"`
	Name                string   `json:"name"`
	Owner               string   `json:"owner,omitempty"`
	State               string   `json:"state"`
	SourceTaskID        string   `json:"source_task_id"`
	SourceTaskTitle     string   `json:"source_task_title"`
	Provenance          string   `json:"provenance"`
	LastVerifiedAt      string   `json:"last_verified_at,omitempty"`
	LastVerifiedCommit  string   `json:"last_verified_commit,omitempty"`
	LastVerifiedVersion string   `json:"last_verified_version,omitempty"`
	RequiredEvidence    []string `json:"required_evidence,omitempty"`
	OpenBlockers        []string `json:"open_blockers,omitempty"`
	NextSafeAction      string   `json:"next_safe_action"`
	EvidenceRefs        []string `json:"evidence_refs,omitempty"`
}

type Options struct {
	Type      string
	Project   string
	StaleOnly bool
	Now       time.Time
}

func Build(ctx context.Context, s *store.Store, opts Options) ([]Resource, error) {
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return nil, err
	}
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.Definition.ID)
	}
	evidence, err := s.EvidenceByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	return FromTasks(tasks, evidence, opts), nil
}

func FromTasks(tasks []store.Task, evidenceByTask map[string][]store.Evidence, opts Options) []Resource {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var resources []Resource
	for _, task := range tasks {
		evs := append([]store.Evidence(nil), evidenceByTask[task.Definition.ID]...)
		resType := resourceType(task, evs)
		if resType == "" {
			continue
		}
		resource := resourceFromTask(task, evs, resType, now)
		if opts.Type != "" && resource.Type != opts.Type {
			continue
		}
		if opts.Project != "" && resource.Project != opts.Project {
			continue
		}
		if opts.StaleOnly && resource.State != "stale" {
			continue
		}
		resources = append(resources, resource)
	}
	sort.SliceStable(resources, func(i, j int) bool {
		if resources[i].Project != resources[j].Project {
			return resources[i].Project < resources[j].Project
		}
		if resources[i].Type != resources[j].Type {
			return resources[i].Type < resources[j].Type
		}
		if resources[i].Name != resources[j].Name {
			return resources[i].Name < resources[j].Name
		}
		return resources[i].SourceTaskID < resources[j].SourceTaskID
	})
	return resources
}

func resourceFromTask(task store.Task, evs []store.Evidence, resType string, now time.Time) Resource {
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].CreatedAt > evs[j].CreatedAt })
	latestPass, hasPass := latestEvidence(evs, "pass")
	latestProblem, hasProblem := latestProblemEvidence(evs)
	lastVerifiedAt := ""
	lastVerifiedCommit := ""
	lastVerifiedVersion := ""
	if hasPass {
		lastVerifiedAt = latestPass.CreatedAt
		lastVerifiedCommit = firstMatch(commitPattern, evidenceText(latestPass))
		lastVerifiedVersion = firstMatch(versionPattern, evidenceText(latestPass))
	}
	state := resourceState(task, latestPass, hasPass, latestProblem, hasProblem, now)
	blockers := resourceBlockers(task, latestProblem, hasProblem)
	return Resource{
		Project:             firstNonEmpty(task.Project, "current"),
		Type:                resType,
		Name:                resourceName(task, resType),
		Owner:               firstNonEmpty(task.Owner, task.Definition.Role),
		State:               state,
		SourceTaskID:        task.Definition.ID,
		SourceTaskTitle:     task.Definition.Title,
		Provenance:          resourceProvenance(task, latestPass, hasPass),
		LastVerifiedAt:      lastVerifiedAt,
		LastVerifiedCommit:  lastVerifiedCommit,
		LastVerifiedVersion: lastVerifiedVersion,
		RequiredEvidence:    requiredEvidence(resType),
		OpenBlockers:        blockers,
		NextSafeAction:      nextSafeAction(state, resType),
		EvidenceRefs:        evidenceRefs(evs),
	}
}

func resourceType(task store.Task, evs []store.Evidence) string {
	haystack := taskHaystack(task, evs)
	switch {
	case containsAny(haystack, "dashboard"):
		return "dashboard"
	case containsAny(haystack, "docs portal", "docusaurus", "cloudflare pages", "documentation portal"):
		return "docs_portal"
	case containsAny(haystack, "release verify", "tag", "publish release"):
		return "release_artifact"
	case containsAny(haystack, "release artifact", "github release", "homebrew", "goreleaser"):
		return "release_artifact"
	case containsAny(haystack, "binary"):
		return "binary"
	case containsAny(haystack, "pipeline", "gitlab", "github actions", "ci"):
		return "ci_pipeline"
	case containsAny(haystack, "preflight", "packet"):
		return "preflight_packet"
	case containsAny(haystack, "rehearsal", "staging", "airgap", "demo", "dev env", "kind env"):
		return "rehearsal_target"
	case containsAny(haystack, "environment", "env", "deployment"):
		return "environment"
	default:
		return ""
	}
}

func taskHaystack(task store.Task, evs []store.Evidence) string {
	parts := []string{
		task.Definition.ID,
		task.Definition.Title,
		task.Definition.Kind,
		task.Definition.Profile,
		task.Definition.OwningDomain,
		task.Definition.OwningLayer,
		task.Definition.MigrationType,
		strings.Join(task.Definition.Tags, " "),
		strings.Join(task.Definition.SourcePaths, " "),
		strings.Join(task.Definition.TargetPaths, " "),
	}
	for _, ev := range evs {
		parts = append(parts, ev.ArtifactType, ev.ArtifactPath, ev.CommandText, ev.Notes)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func resourceState(task store.Task, latestPass store.Evidence, hasPass bool, latestProblem store.Evidence, hasProblem bool, now time.Time) string {
	if task.Status == "blocked" {
		return "blocked"
	}
	if hasProblem {
		switch strings.ToLower(latestProblem.Result) {
		case "fail", "failed", "blocked":
			return "failed_verification"
		default:
			return "needs_attention"
		}
	}
	if hasPass {
		if parsed, ok := parseTime(latestPass.CreatedAt); ok && now.Sub(parsed) > staleAfter {
			return "stale"
		}
		if task.Status == "done" && containsAny(taskHaystack(task, []store.Evidence{latestPass}), "handoff", "readiness", "packet") {
			return "handoff_ready"
		}
		return "verified"
	}
	if task.Status == "done" {
		return "recorded"
	}
	return "unverified"
}

func resourceBlockers(task store.Task, latestProblem store.Evidence, hasProblem bool) []string {
	var blockers []string
	if task.Status == "blocked" {
		blockers = append(blockers, "source task is blocked")
	}
	if hasProblem {
		label := firstNonEmpty(latestProblem.ArtifactType, latestProblem.CommandText, latestProblem.Result)
		blockers = append(blockers, "latest problem evidence: "+label)
	}
	return blockers
}

func nextSafeAction(state string, resType string) string {
	switch state {
	case "blocked":
		return "resolve blocker or create owner handoff before retry"
	case "failed_verification":
		return "inspect failed evidence and rerun the scoped verification"
	case "needs_attention":
		return "triage partial evidence and decide fix-now or defer"
	case "stale":
		return "refresh readback before handoff"
	case "handoff_ready":
		return "handoff with recorded evidence; do not mutate from report"
	case "verified":
		return "safe to reference in closeout; boundary actions still need explicit task"
	case "recorded":
		return "add current verification evidence before operational handoff"
	default:
		if resType == "preflight_packet" || resType == "rehearsal_target" {
			return "run packet/readback proof and record evidence"
		}
		return "record verification evidence"
	}
}

func resourceName(task store.Task, resType string) string {
	title := strings.TrimSpace(task.Definition.Title)
	if title == "" {
		title = task.Definition.ID
	}
	return strings.ToLower(strings.Join(strings.Fields(resType+" "+title), "-"))
}

func resourceProvenance(task store.Task, latestPass store.Evidence, hasPass bool) string {
	parts := []string{"task=" + task.Definition.ID}
	if task.CommitSHA != "" {
		parts = append(parts, "commit="+task.CommitSHA)
	}
	if hasPass {
		if latestPass.ArtifactPath != "" {
			parts = append(parts, "evidence="+latestPass.ArtifactPath)
		} else if latestPass.ArtifactType != "" {
			parts = append(parts, "evidence_type="+latestPass.ArtifactType)
		}
	}
	return strings.Join(parts, " ")
}

func requiredEvidence(resType string) []string {
	switch resType {
	case "dashboard":
		return []string{"status readback", "version or binary readback", "protected boundary check"}
	case "docs_portal":
		return []string{"build", "publish readback", "navigation check"}
	case "binary":
		return []string{"build", "version readback", "release verification"}
	case "release_artifact":
		return []string{"tag/publish approval", "release verify", "asset/Homebrew readback"}
	case "ci_pipeline":
		return []string{"pipeline URL", "job result", "rerun or blocker owner"}
	case "preflight_packet":
		return []string{"packet render", "check results", "rollback or next action"}
	case "rehearsal_target":
		return []string{"apply/readback proof", "equivalence check", "rollback plan"}
	case "environment":
		return []string{"preflight", "smoke test", "rollback plan"}
	default:
		return []string{"verification evidence"}
	}
}

func evidenceRefs(evs []store.Evidence) []string {
	seen := map[string]bool{}
	var refs []string
	for _, ev := range evs {
		ref := strings.TrimSpace(ev.ArtifactPath)
		if ref == "" {
			ref = strings.TrimSpace(ev.ArtifactType)
		}
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
		if len(refs) >= 5 {
			break
		}
	}
	return refs
}

func latestEvidence(evs []store.Evidence, result string) (store.Evidence, bool) {
	for _, ev := range evs {
		if strings.EqualFold(strings.TrimSpace(ev.Result), result) {
			return ev, true
		}
	}
	return store.Evidence{}, false
}

func latestProblemEvidence(evs []store.Evidence) (store.Evidence, bool) {
	for _, ev := range evs {
		switch strings.ToLower(strings.TrimSpace(ev.Result)) {
		case "fail", "failed", "blocked", "partial":
			return ev, true
		}
	}
	return store.Evidence{}, false
}

func evidenceText(ev store.Evidence) string {
	return strings.Join([]string{ev.CommandText, ev.ArtifactPath, ev.ArtifactType, ev.Notes}, " ")
}

func parseTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func firstMatch(re *regexp.Regexp, text string) string {
	match := re.FindString(text)
	return match
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
