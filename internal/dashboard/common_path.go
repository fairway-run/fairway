package dashboard

import (
	"fmt"
	"strings"

	"github.com/subashram/fairway/internal/reconcile"
	"github.com/subashram/fairway/internal/store"
)

type CommonPathInput struct {
	Task                 store.Task
	ActiveSessions       int
	EvidenceCount        int
	MissingReviewDomains []string
	ReviewMode           string
	ActiveFindings       []reconcile.ActiveFinding
	DecisionAttention    int
}

type CommonPathRecommendation struct {
	Classification    string   `json:"classification"`
	CurrentAction     string   `json:"current_action"`
	SuggestedCommand  string   `json:"suggested_command"`
	BoundaryStatus    string   `json:"boundary_status"`
	Blockers          []string `json:"blockers,omitempty"`
	Advisories        []string `json:"advisories,omitempty"`
	AuditCommand      string   `json:"audit_command"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

func RecommendCommonPath(input CommonPathInput) CommonPathRecommendation {
	taskID := input.Task.Definition.ID
	out := CommonPathRecommendation{
		Classification:    "inspect",
		CurrentAction:     "Inspect durable task detail.",
		SuggestedCommand:  "fairway task-detail " + taskID,
		BoundaryStatus:    "clear",
		AuditCommand:      "fairway work status " + taskID + " --explain",
		AuthorityBoundary: "recommendations are deterministic and advisory; existing review, merge, deploy, release, credential, public-exposure, and live-operation gates remain authoritative",
	}
	switch input.Task.Status {
	case "todo":
		if input.ActiveSessions != 0 {
			out.Classification = "ambiguous"
			out.CurrentAction = "Resolve the unexpected provider-session attachment before starting."
			out.SuggestedCommand = "fairway session status"
			out.BoundaryStatus = "blocked"
			out.Blockers = append(out.Blockers, fmt.Sprintf("todo task has %d active session attachment(s)", input.ActiveSessions))
			return out
		}
		out.Classification = "start"
		out.CurrentAction = "Start the ready task and attach one provider session."
		out.SuggestedCommand = "fairway work start " + taskID
		return out
	case "blocked":
		out.Classification = "blocked"
		out.CurrentAction = "Resolve the recorded blocker before resuming work."
		out.BoundaryStatus = "blocked"
		out.Blockers = append(out.Blockers, "task status is blocked")
		return out
	case "done":
		if input.ActiveSessions > 0 {
			out.Classification = "blocked"
			out.CurrentAction = "End or reconcile provider sessions still attached to the completed task."
			out.SuggestedCommand = "fairway reconcile active --dry-run"
			out.BoundaryStatus = "blocked"
			out.Blockers = append(out.Blockers, fmt.Sprintf("completed task still has %d active session attachment(s)", input.ActiveSessions))
			return out
		}
		out.Classification = "complete"
		out.CurrentAction = "Task is complete; inspect evidence or handback only if needed."
		return out
	case "in_progress":
	default:
		out.Classification = "ambiguous"
		out.CurrentAction = "Resolve the unsupported task lifecycle state."
		out.BoundaryStatus = "blocked"
		out.Blockers = append(out.Blockers, "unsupported task status "+input.Task.Status)
		return out
	}
	if input.ActiveSessions != 1 {
		out.Classification = "ambiguous"
		out.CurrentAction = "Resolve provider-session attachment before continuing."
		out.BoundaryStatus = "blocked"
		out.Blockers = append(out.Blockers, fmt.Sprintf("expected exactly one active session, found %d", input.ActiveSessions))
		out.SuggestedCommand = "fairway session status"
		return out
	}
	statusDecision := false
	for _, finding := range input.ActiveFindings {
		if finding.Kind == "status_decision_required" {
			statusDecision = true
			continue
		}
		out.Blockers = append(out.Blockers, finding.Kind+": "+finding.Reason)
	}
	if len(out.Blockers) > 0 {
		out.Classification = "blocked"
		out.CurrentAction = "Resolve active reconciliation findings before continuing."
		out.BoundaryStatus = "blocked"
		out.SuggestedCommand = "fairway reconcile active --dry-run"
		return out
	}
	if input.DecisionAttention > 0 {
		out.Classification = "decision_attention"
		out.CurrentAction = "Resolve required decision-quality assessment before closeout."
		out.BoundaryStatus = "consequential-gates-pending"
		out.SuggestedCommand = "fairway decision list " + taskID
		return out
	}
	if input.EvidenceCount == 0 {
		out.Classification = "verify"
		out.CurrentAction = "Run the bounded validation externally, then record its summary."
		out.SuggestedCommand = "fairway work verify " + taskID + " --command-text <summary> --result <result>"
		return out
	}
	if len(input.MissingReviewDomains) > 0 {
		domains := strings.Join(input.MissingReviewDomains, ",")
		if input.ReviewMode == "advisory" {
			out.Advisories = append(out.Advisories, "advisory reviews pending: "+domains)
		} else {
			out.Classification = "review"
			out.CurrentAction = "Route and complete the required independent reviews."
			out.BoundaryStatus = "consequential-gates-pending"
			out.Blockers = append(out.Blockers, "missing approved review domains: "+domains)
			out.SuggestedCommand = "fairway route review " + taskID
			return out
		}
	}
	out.Classification = "close"
	out.CurrentAction = "Check merge readiness, then close the task and attached session."
	out.SuggestedCommand = "fairway merge-ready " + taskID + " && fairway work close " + taskID
	if statusDecision {
		out.CurrentAction = "Make the explicit closeout status decision through guarded work close."
	}
	return out
}

func taskDecisionAcceptanceRequired(task store.Task) bool {
	text := strings.ToLower(strings.Join(append([]string{task.Definition.RiskLevel, task.Definition.MigrationType, task.Definition.Profile, task.Definition.OwningLayer}, task.Definition.Tags...), " "))
	for _, marker := range []string{"high", "critical", "release-boundary", "security", "live", "production", "deploy", "release", "credential", "public-exposure", "public_exposure", "irreversible", "migration"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
