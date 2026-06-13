package completionhandback

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

const payloadPrefix = "completion-handback "

type Payload struct {
	NextAction       string   `json:"next_action"`
	EvidencePaths    []string `json:"evidence_paths,omitempty"`
	ApprovalBoundary string   `json:"approval_boundary,omitempty"`
}

type Handback struct {
	TaskID               string   `json:"task_id"`
	HandoffID            int64    `json:"handoff_id"`
	FromRole             string   `json:"from_role,omitempty"`
	ToRole               string   `json:"to_role"`
	NextAction           string   `json:"next_action"`
	EvidencePaths        []string `json:"evidence_paths,omitempty"`
	ApprovalBoundary     string   `json:"approval_boundary,omitempty"`
	DeliveryState        string   `json:"delivery_state,omitempty"`
	DeliveryStatus       string   `json:"delivery_status"`
	Provider             string   `json:"provider,omitempty"`
	Target               string   `json:"target,omitempty"`
	Reason               string   `json:"reason,omitempty"`
	ActualThreadDelivery bool     `json:"actual_thread_delivery"`
	CreatedAt            string   `json:"created_at,omitempty"`
	DeliveredAt          string   `json:"delivered_at,omitempty"`
}

func RenderPayload(nextAction string, evidencePaths []string, approvalBoundary string) (string, error) {
	nextAction = strings.TrimSpace(nextAction)
	if nextAction == "" {
		return "", fmt.Errorf("completion handback next action is required")
	}
	payload := Payload{
		NextAction:       nextAction,
		EvidencePaths:    cleanList(evidencePaths),
		ApprovalBoundary: strings.TrimSpace(approvalBoundary),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return payloadPrefix + string(data), nil
}

func Rows(taskID string, handoffs []store.Handoff, notifications []store.Notification) []Handback {
	notificationsByHandoff := map[int64][]store.Notification{}
	for _, notification := range notifications {
		if notification.HandoffID == nil {
			continue
		}
		notificationsByHandoff[*notification.HandoffID] = append(notificationsByHandoff[*notification.HandoffID], notification)
	}
	var out []Handback
	for _, handoff := range handoffs {
		payload, ok := ParsePayload(handoff.Payload)
		if !ok {
			continue
		}
		row := Handback{
			TaskID:           taskID,
			HandoffID:        handoff.ID,
			FromRole:         handoff.FromRole,
			ToRole:           handoff.ToRole,
			NextAction:       payload.NextAction,
			EvidencePaths:    payload.EvidencePaths,
			ApprovalBoundary: payload.ApprovalBoundary,
			DeliveryStatus:   "pending",
			CreatedAt:        handoff.CreatedAt,
		}
		if latest, ok := latestNotification(notificationsByHandoff[handoff.ID]); ok {
			row.DeliveryState = latest.State
			row.DeliveryStatus = DeliveryStatus(latest.State)
			row.Provider = latest.Provider
			row.Target = latest.Target
			row.Reason = latest.Reason
			row.DeliveredAt = latest.CreatedAt
			row.ActualThreadDelivery = ActualThreadDelivery(latest.State)
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].HandoffID > out[j].HandoffID
	})
	return out
}

func ParsePayload(raw string) (Payload, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, payloadPrefix) {
		return Payload{}, false
	}
	var payload Payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(raw, payloadPrefix))), &payload); err != nil {
		return Payload{}, false
	}
	payload.NextAction = strings.TrimSpace(payload.NextAction)
	payload.ApprovalBoundary = strings.TrimSpace(payload.ApprovalBoundary)
	payload.EvidencePaths = cleanList(payload.EvidencePaths)
	if payload.NextAction == "" {
		return Payload{}, false
	}
	return payload, true
}

func DeliveryStatus(state string) string {
	switch strings.TrimSpace(state) {
	case "notification_delivered", "thread_steered":
		return "delivered"
	case "failed", "notification_failed":
		return "failed"
	case "":
		return "pending"
	default:
		return "pending"
	}
}

func ActualThreadDelivery(state string) bool {
	switch strings.TrimSpace(state) {
	case "notification_delivered", "thread_steered":
		return true
	default:
		return false
	}
}

func IsResolved(row Handback) bool {
	switch row.DeliveryStatus {
	case "delivered", "failed":
		return true
	default:
		return false
	}
}

func cleanList(values []string) []string {
	var out []string
	seen := map[string]bool{}
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

func latestNotification(notifications []store.Notification) (store.Notification, bool) {
	var latest store.Notification
	for _, notification := range notifications {
		if latest.CreatedAt == "" || notification.CreatedAt > latest.CreatedAt || (notification.CreatedAt == latest.CreatedAt && notification.ID > latest.ID) {
			latest = notification
		}
	}
	return latest, latest.CreatedAt != ""
}
