package dashboard

import (
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/completionhandback"
	coord "github.com/subashram/fairway/internal/coordinator"
	"github.com/subashram/fairway/internal/reviewstate"
	"github.com/subashram/fairway/internal/store"
)

type CoordinationIntelligence struct {
	MemoryTotal          int                  `json:"memory_total"`
	MemoryStale          int                  `json:"memory_stale"`
	OpenWaits            int                  `json:"open_waits"`
	StaleWaits           int                  `json:"stale_waits"`
	NotificationFailures int                  `json:"notification_failures"`
	NextActions          []CoordinationAction `json:"next_actions,omitempty"`
	Waits                []CoordinationWait   `json:"waits,omitempty"`
	Memories             []CoordinationMemory `json:"memories,omitempty"`
}

type CoordinationAction struct {
	Classification string `json:"classification"`
	Action         string `json:"action"`
	TaskID         string `json:"task_id,omitempty"`
	Role           string `json:"role,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

type CoordinationWait struct {
	Source           string `json:"source"`
	TaskID           string `json:"task_id,omitempty"`
	Owner            string `json:"owner,omitempty"`
	State            string `json:"state"`
	Action           string `json:"action,omitempty"`
	TargetProvider   string `json:"target_provider,omitempty"`
	Target           string `json:"target,omitempty"`
	LastWakeAttempt  string `json:"last_wake_attempt,omitempty"`
	SuggestedCommand string `json:"suggested_command,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

type CoordinationMemory struct {
	TrackID       string `json:"track_id"`
	Title         string `json:"title,omitempty"`
	Owner         string `json:"owner,omitempty"`
	ReviewBy      string `json:"review_by,omitempty"`
	Disposition   string `json:"disposition,omitempty"`
	PromotionDebt bool   `json:"promotion_debt"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	Stale         bool   `json:"stale"`
	NextAction    string `json:"next_action,omitempty"`
	OpenBlocker   string `json:"open_blocker,omitempty"`
}

func dashboardCoordinationIntelligence(plan coord.Plan, memories []store.TrackMemory, now time.Time, staleMemoryAfter time.Duration) CoordinationIntelligence {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if staleMemoryAfter <= 0 {
		staleMemoryAfter = 24 * time.Hour
	}
	out := CoordinationIntelligence{
		MemoryTotal: len(memories),
		NextActions: coordinationActions(plan.Actions, 5),
	}
	for _, mem := range memories {
		row := CoordinationMemory{
			TrackID:       mem.TrackID,
			Title:         mem.Title,
			Owner:         mem.Owner,
			ReviewBy:      mem.ReviewBy,
			Disposition:   mem.Disposition,
			PromotionDebt: mem.Disposition == "promote" && (mem.PromotionTarget == "" || mem.CanonicalCommit == ""),
			UpdatedAt:     mem.UpdatedAt,
			Stale:         trackMemoryStale(mem, now, staleMemoryAfter),
			NextAction:    firstNonEmptyString(mem.NextActions),
			OpenBlocker:   firstNonEmptyString(mem.Blockers),
		}
		if row.Stale {
			out.MemoryStale++
		}
		out.Memories = append(out.Memories, row)
	}
	out.Waits = append(out.Waits, coordinationReviewWaits(plan.ReviewWaits)...)
	out.Waits = append(out.Waits, coordinationCompletionWaits(plan.CompletionHandbacks)...)
	for _, wait := range out.Waits {
		if wait.State != "resolved" && wait.State != "cancelled" && wait.State != "superseded" {
			out.OpenWaits++
		}
		if wait.State == "stale" || strings.Contains(wait.Action, "escalate") {
			out.StaleWaits++
		}
		if wait.State == "notification_failed" || wait.Action == "mapping_required" {
			out.NotificationFailures++
		}
	}
	sort.SliceStable(out.Waits, func(i, j int) bool {
		if waitRank(out.Waits[i]) != waitRank(out.Waits[j]) {
			return waitRank(out.Waits[i]) < waitRank(out.Waits[j])
		}
		if out.Waits[i].TaskID != out.Waits[j].TaskID {
			return out.Waits[i].TaskID < out.Waits[j].TaskID
		}
		return out.Waits[i].Owner < out.Waits[j].Owner
	})
	if len(out.Waits) > 6 {
		out.Waits = out.Waits[:6]
	}
	if len(out.Memories) > 4 {
		out.Memories = out.Memories[:4]
	}
	return out
}

func coordinationActions(actions []coord.PlanAction, limit int) []CoordinationAction {
	if limit <= 0 {
		limit = 5
	}
	var out []CoordinationAction
	for _, action := range actions {
		out = append(out, CoordinationAction{
			Classification: action.Classification,
			Action:         action.Action,
			TaskID:         action.TaskID,
			Role:           action.Role,
			Reason:         action.Reason,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func coordinationReviewWaits(waits []reviewstate.ReviewWait) []CoordinationWait {
	var out []CoordinationWait
	for _, wait := range waits {
		out = append(out, CoordinationWait{
			Source:           "review_wait",
			TaskID:           wait.TaskID,
			Owner:            wait.Domain,
			State:            wait.State,
			Action:           wait.Action,
			TargetProvider:   wait.TargetProvider,
			Target:           firstNonEmpty(wait.TargetID, wait.WakeThreadID),
			LastWakeAttempt:  wait.LastNotifiedAt,
			SuggestedCommand: reviewWaitSuggestedCommand(wait),
			Reason:           wait.Reason,
		})
	}
	return out
}

func coordinationCompletionWaits(handbacks []completionhandback.Handback) []CoordinationWait {
	var out []CoordinationWait
	for _, handback := range handbacks {
		if completionhandback.IsResolved(handback) {
			continue
		}
		out = append(out, CoordinationWait{
			Source:           "completion_handback",
			TaskID:           handback.TaskID,
			Owner:            handback.ToRole,
			State:            firstNonEmpty(handback.DeliveryStatus, handback.DeliveryState),
			Action:           handback.SuggestedAction,
			TargetProvider:   handback.Provider,
			Target:           handback.Target,
			LastWakeAttempt:  firstNonEmpty(handback.DeliveredAt, handback.CreatedAt),
			SuggestedCommand: handback.SuggestedCommand,
			Reason:           firstNonEmpty(handback.Reason, handback.NextAction),
		})
	}
	return out
}

func trackMemoryStale(mem store.TrackMemory, now time.Time, staleAfter time.Duration) bool {
	updatedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(mem.UpdatedAt))
	if err != nil {
		return true
	}
	return now.Sub(updatedAt) >= staleAfter
}

func reviewWaitSuggestedCommand(wait reviewstate.ReviewWait) string {
	switch wait.Action {
	case "mapping_required":
		return "fairway route review " + wait.TaskID + " --reviewer " + wait.Domain
	case "record_delivery_proof":
		return "fairway record notification " + wait.TaskID + " --domain " + wait.Domain + " --state notification_delivered"
	case "deliver_notification", "nudge_reviewer":
		return "fairway review-waits wake --task " + wait.TaskID
	default:
		if wait.TaskID != "" {
			return "fairway review-waits list --task " + wait.TaskID
		}
		return "fairway review-waits list"
	}
}

func waitRank(wait CoordinationWait) int {
	switch wait.State {
	case "notification_failed":
		return 0
	case "stale":
		return 1
	case "pending":
		return 2
	case "resolved":
		return 4
	default:
		return 3
	}
}

func firstNonEmptyString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
