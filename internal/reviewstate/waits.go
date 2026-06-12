package reviewstate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

type ReviewWaitOptions struct {
	ProviderTargets []config.ProviderTarget
	ReviewRoutes    []config.ReviewRoute
	Roles           []config.Role
	AckTimeout      time.Duration
	Now             time.Time
	Terminal        []string
}

type ReviewWait struct {
	WaitID             string `json:"wait_id"`
	TaskID             string `json:"task_id"`
	Domain             string `json:"domain"`
	State              string `json:"state"`
	Blocking           bool   `json:"blocking"`
	Reason             string `json:"reason"`
	Action             string `json:"action"`
	TargetProvider     string `json:"target_provider,omitempty"`
	TargetID           string `json:"target_id,omitempty"`
	LastNotifiedAt     string `json:"last_notified_at,omitempty"`
	ExpectedResponseAt string `json:"expected_response_at,omitempty"`
	ResolvedAt         string `json:"resolved_at,omitempty"`
	ResolvedBy         string `json:"resolved_by,omitempty"`
	WakeThreadID       string `json:"wake_thread_id,omitempty"`
}

type RoutabilityIssue struct {
	TaskID string `json:"task_id"`
	Domain string `json:"domain"`
	Reason string `json:"reason"`
	Action string `json:"action"`
}

func WaitsForTask(task store.Task, handoffs []store.Handoff, reviews []store.Review, notifications []store.Notification, opts ReviewWaitOptions) []ReviewWait {
	required := normalizedUnique(task.Definition.ReviewDomains)
	if len(required) == 0 {
		return nil
	}
	if strings.TrimSpace(task.Status) == "todo" {
		return nil
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	statuses := StatusesForTask(task, handoffs, reviews, notifications)
	statusByDomain := map[string]ReviewNotificationStatus{}
	for _, status := range statuses {
		statusByDomain[status.Domain] = status
	}
	latestReview := latestReviewByDomain(reviews)
	var waits []ReviewWait
	for _, domain := range required {
		status := statusByDomain[domain]
		if status.Domain == "" {
			status = ReviewNotificationStatus{TaskID: task.Definition.ID, Domain: domain, Status: "missing_notification", Blocking: true}
		}
		wait := ReviewWait{
			WaitID:         task.Definition.ID + "/" + domain,
			TaskID:         task.Definition.ID,
			Domain:         domain,
			Blocking:       true,
			Reason:         status.Reason,
			LastNotifiedAt: status.LastNotificationAt,
		}
		target, routable := targetForDomain(domain, opts)
		wait.TargetProvider = target.Provider
		wait.TargetID = target.Target
		wait.WakeThreadID = wakeThreadID(target)

		if review, ok := latestReview[domain]; ok {
			wait.State = "resolved"
			wait.Blocking = false
			wait.Action = actionForReviewVerdict(review.Verdict)
			wait.Reason = fmt.Sprintf("latest review verdict recorded: %s", firstNonEmpty(review.Verdict, "unknown"))
			wait.ResolvedAt = review.CreatedAt
			wait.ResolvedBy = firstNonEmpty(review.Reviewer, review.Domain)
			waits = append(waits, wait)
			continue
		}
		if isTerminalStatus(task.Status, opts.Terminal) || !domainRequired(task.Definition.ReviewDomains, domain) {
			wait.State = "cancelled"
			wait.Blocking = false
			wait.Action = "none"
			wait.Reason = "task is terminal or review domain is no longer required"
			waits = append(waits, wait)
			continue
		}
		clockStart := firstNonEmpty(status.LastNotificationAt, status.LastHandoffAt, task.UpdatedAt)
		wait.ExpectedResponseAt = expectedResponseAt(clockStart, opts.AckTimeout)
		switch status.Status {
		case "review_recorded":
			wait.State = "resolved"
			wait.Blocking = false
			wait.Action = "inspect_review_verdict"
			wait.ResolvedAt = firstNonEmpty(status.LastNotificationAt, task.UpdatedAt)
			wait.ResolvedBy = domain
		case "notification_failed":
			wait.State = "notification_failed"
			wait.Action = "mapping_required"
			if routable {
				wait.Action = "deliver_notification"
			}
			wait.Reason = firstNonEmpty(status.Reason, "review notification delivery failed")
		case "missing_notification":
			if !routable {
				wait.State = "notification_failed"
				wait.Action = "mapping_required"
				wait.Reason = "required review domain has no configured reviewer role, review route, or provider target"
			} else {
				wait.State = pendingOrStale(wait.ExpectedResponseAt, opts.Now)
				wait.Action = "deliver_notification"
				wait.Reason = firstNonEmpty(status.Reason, "required review domain has no reviewer notification yet")
			}
		case "handoff_recorded":
			wait.State = pendingOrStale(wait.ExpectedResponseAt, opts.Now)
			wait.Action = "record_delivery_proof"
			wait.Reason = firstNonEmpty(status.Reason, "Fairway handoff exists without provider delivery proof")
		case "sent_awaiting_ack":
			wait.State = pendingOrStale(wait.ExpectedResponseAt, opts.Now)
			wait.Action = "record_delivery_proof"
			if wait.State == "stale" {
				wait.Action = "nudge_reviewer"
			}
			wait.Reason = firstNonEmpty(status.Reason, "review notification was sent and is awaiting acknowledgement")
		case "notification_delivered", "review_acknowledged":
			wait.State = pendingOrStale(wait.ExpectedResponseAt, opts.Now)
			wait.Action = "nudge_reviewer"
			wait.Reason = firstNonEmpty(status.Reason, "reviewer notification was delivered; review verdict is still missing")
		default:
			wait.State = "pending"
			wait.Action = "deliver_notification"
		}
		if wait.State == "stale" && wait.Action == "" {
			wait.Action = "nudge_reviewer"
		}
		waits = append(waits, wait)
	}
	sort.SliceStable(waits, func(i, j int) bool {
		if waits[i].Blocking != waits[j].Blocking {
			return waits[i].Blocking
		}
		if waits[i].State != waits[j].State {
			return waits[i].State < waits[j].State
		}
		return waits[i].WaitID < waits[j].WaitID
	})
	return waits
}

func UnroutableRequiredDomains(task store.Task, opts ReviewWaitOptions) []RoutabilityIssue {
	var issues []RoutabilityIssue
	for _, domain := range normalizedUnique(task.Definition.ReviewDomains) {
		if _, ok := targetForDomain(domain, opts); ok {
			continue
		}
		issues = append(issues, RoutabilityIssue{
			TaskID: task.Definition.ID,
			Domain: domain,
			Reason: "required review domain has no configured reviewer role, review route, or provider target",
			Action: "mapping_required",
		})
	}
	return issues
}

func targetForDomain(domain string, opts ReviewWaitOptions) (config.ProviderTarget, bool) {
	domain = strings.TrimSpace(domain)
	for _, target := range opts.ProviderTargets {
		if strings.TrimSpace(target.Domain) == domain {
			return target, true
		}
	}
	for _, role := range opts.Roles {
		if strings.TrimSpace(role.Name) == domain {
			return config.ProviderTarget{Domain: domain, Provider: firstNonEmpty(role.Provider, "fairway"), Target: domain, Type: "role"}, true
		}
	}
	for _, route := range opts.ReviewRoutes {
		if strings.TrimSpace(route.Reviewer) == domain {
			return config.ProviderTarget{Domain: domain, Provider: "fairway", Target: domain, Type: "review_route"}, true
		}
	}
	return config.ProviderTarget{}, false
}

func wakeThreadID(target config.ProviderTarget) string {
	switch strings.TrimSpace(target.Type) {
	case "thread":
		return strings.TrimSpace(target.Target)
	default:
		return ""
	}
}

func expectedResponseAt(start string, timeout time.Duration) string {
	if strings.TrimSpace(start) == "" || timeout <= 0 {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(start))
	if err != nil {
		return ""
	}
	return t.Add(timeout).UTC().Format(time.RFC3339Nano)
}

func pendingOrStale(expected string, now time.Time) string {
	if strings.TrimSpace(expected) == "" {
		return "pending"
	}
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(expected))
	if err != nil {
		return "pending"
	}
	if !now.Before(t) {
		return "stale"
	}
	return "pending"
}

func domainRequired(domains []string, domain string) bool {
	for _, candidate := range domains {
		if strings.TrimSpace(candidate) == domain {
			return true
		}
	}
	return false
}

func isTerminalStatus(status string, terminal []string) bool {
	status = strings.TrimSpace(status)
	for _, value := range terminal {
		if strings.TrimSpace(value) == status {
			return true
		}
	}
	return false
}

func actionForReviewVerdict(verdict string) string {
	switch strings.TrimSpace(verdict) {
	case "approve":
		return "run_merge_ready"
	case "changes", "reject":
		return "address_review_changes"
	default:
		return "inspect_review_verdict"
	}
}
