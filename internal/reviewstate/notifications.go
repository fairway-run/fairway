package reviewstate

import (
	"fmt"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

type ReviewNotificationStatus struct {
	TaskID             string `json:"task_id"`
	Domain             string `json:"domain"`
	Status             string `json:"status"`
	Blocking           bool   `json:"blocking"`
	HandoffID          int64  `json:"handoff_id,omitempty"`
	LastHandoffAt      string `json:"last_handoff_at,omitempty"`
	LastNotificationAt string `json:"last_notification_at,omitempty"`
	LastState          string `json:"last_state,omitempty"`
	Provider           string `json:"provider,omitempty"`
	Target             string `json:"target,omitempty"`
	Reason             string `json:"reason,omitempty"`
	SuggestedAction    string `json:"suggested_action"`
}

func StatusesForTask(task store.Task, handoffs []store.Handoff, reviews []store.Review, notifications []store.Notification) []ReviewNotificationStatus {
	required := normalizedUnique(task.Definition.ReviewDomains)
	if len(required) == 0 {
		return nil
	}
	latestReview := latestReviewByDomain(reviews)
	out := make([]ReviewNotificationStatus, 0, len(required))
	for _, domain := range required {
		status := ReviewNotificationStatus{
			TaskID:          task.Definition.ID,
			Domain:          domain,
			Status:          "missing_notification",
			Blocking:        true,
			Reason:          "required review domain has no delivered reviewer notification or recorded review",
			SuggestedAction: fmt.Sprintf("record and deliver reviewer notification for %s, then record notification_delivered/thread_steered or review_acknowledged", domain),
		}
		if review, ok := latestReview[domain]; ok {
			status.Status = "review_recorded"
			status.Blocking = false
			status.Reason = fmt.Sprintf("latest review verdict recorded: %s", firstNonEmpty(review.Verdict, "unknown"))
			status.SuggestedAction = fmt.Sprintf("resolve latest %s review verdict if approval is still missing", domain)
			out = append(out, status)
			continue
		}
		if handoff, ok := latestHandoffForDomain(handoffs, domain); ok {
			status.Status = "handoff_recorded"
			status.HandoffID = handoff.ID
			status.LastHandoffAt = handoff.CreatedAt
			status.Reason = "Fairway handoff exists, but no delivered reviewer notification or acknowledgement is recorded"
			status.SuggestedAction = fmt.Sprintf("deliver handoff %d to %s reviewer and record notification delivery", handoff.ID, domain)
		}
		if notification, ok := latestNotificationForDomain(notifications, domain, status.HandoffID, status.LastHandoffAt); ok {
			status.LastNotificationAt = notification.CreatedAt
			status.LastState = notification.State
			status.Provider = notification.Provider
			status.Target = notification.Target
			status.Reason = firstNonEmpty(notification.Reason, status.Reason)
			switch strings.TrimSpace(notification.State) {
			case "notification_delivered", "thread_steered":
				status.Status = "notification_delivered"
				status.Blocking = false
				status.SuggestedAction = fmt.Sprintf("wait for %s review or route follow-up if stale", domain)
			case "acknowledged", "review_acknowledged":
				status.Status = "review_acknowledged"
				status.Blocking = false
				status.SuggestedAction = fmt.Sprintf("wait for %s review verdict", domain)
			case "review_recorded":
				status.Status = "review_recorded"
				status.Blocking = false
				status.SuggestedAction = fmt.Sprintf("inspect %s review verdict", domain)
			case "failed", "notification_failed":
				status.Status = "notification_failed"
				status.Blocking = true
				status.SuggestedAction = fmt.Sprintf("retry or manually deliver %s reviewer notification", domain)
			case "sent":
				status.Status = "sent_awaiting_ack"
				status.Blocking = true
				status.SuggestedAction = fmt.Sprintf("confirm %s reviewer notification delivery or record acknowledgement", domain)
			case "handoff_recorded", "intent":
				status.Status = "handoff_recorded"
				status.Blocking = true
				status.SuggestedAction = fmt.Sprintf("deliver %s reviewer notification and record delivery", domain)
			}
		}
		out = append(out, status)
	}
	return out
}

func BlockingStatuses(statuses []ReviewNotificationStatus, domains []string) []ReviewNotificationStatus {
	needed := map[string]bool{}
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			needed[domain] = true
		}
	}
	var out []ReviewNotificationStatus
	for _, status := range statuses {
		if status.Blocking && needed[status.Domain] {
			out = append(out, status)
		}
	}
	return out
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
	return out
}

func latestReviewByDomain(reviews []store.Review) map[string]store.Review {
	out := map[string]store.Review{}
	for _, review := range reviews {
		domain := firstNonEmpty(review.Domain, review.Reviewer)
		if domain == "" {
			continue
		}
		out[domain] = review
	}
	return out
}

func latestHandoffForDomain(handoffs []store.Handoff, domain string) (store.Handoff, bool) {
	var latest store.Handoff
	for _, handoff := range handoffs {
		if strings.TrimSpace(handoff.ToRole) != domain {
			continue
		}
		if latest.ID == 0 || handoff.CreatedAt > latest.CreatedAt ||
			(handoff.CreatedAt == latest.CreatedAt && handoff.ID > latest.ID) {
			latest = handoff
		}
	}
	return latest, latest.ID != 0
}

func latestNotificationForDomain(notifications []store.Notification, domain string, handoffID int64, since string) (store.Notification, bool) {
	if handoffID != 0 {
		var latest store.Notification
		for _, notification := range notifications {
			if strings.TrimSpace(notification.Domain) != domain || notification.HandoffID == nil || *notification.HandoffID != handoffID {
				continue
			}
			if laterNotification(notification, latest) {
				latest = notification
			}
		}
		if latest.ID != 0 {
			return latest, true
		}
	}

	var latest store.Notification
	for _, notification := range notifications {
		if strings.TrimSpace(notification.Domain) != domain {
			continue
		}
		// A notification explicitly bound to a different handoff cannot describe
		// the current handoff. Unbound legacy rows retain timestamp fallback.
		if handoffID != 0 && notification.HandoffID != nil {
			continue
		}
		if since != "" && notification.CreatedAt < since {
			continue
		}
		if laterNotification(notification, latest) {
			latest = notification
		}
	}
	return latest, latest.ID != 0
}

func laterNotification(candidate, current store.Notification) bool {
	return current.ID == 0 || candidate.CreatedAt > current.CreatedAt ||
		(candidate.CreatedAt == current.CreatedAt && candidate.ID > current.ID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
