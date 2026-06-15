package completionhandback

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/store"
)

const payloadPrefix = "completion-handback "

type Payload struct {
	NextAction       string   `json:"next_action"`
	CompletionState  string   `json:"completion_state,omitempty"`
	EvidencePaths    []string `json:"evidence_paths,omitempty"`
	ApprovalBoundary string   `json:"approval_boundary,omitempty"`
}

type Handback struct {
	TaskID               string   `json:"task_id"`
	HandoffID            int64    `json:"handoff_id"`
	FromRole             string   `json:"from_role,omitempty"`
	ToRole               string   `json:"to_role"`
	NextAction           string   `json:"next_action"`
	CompletionState      string   `json:"completion_state,omitempty"`
	EvidencePaths        []string `json:"evidence_paths,omitempty"`
	ApprovalBoundary     string   `json:"approval_boundary,omitempty"`
	TaskStatus           string   `json:"task_status,omitempty"`
	LiveWindowPhase      string   `json:"live_window_phase,omitempty"`
	DeliveryState        string   `json:"delivery_state,omitempty"`
	DeliveryStatus       string   `json:"delivery_status"`
	Provider             string   `json:"provider,omitempty"`
	Target               string   `json:"target,omitempty"`
	Reason               string   `json:"reason,omitempty"`
	ActualThreadDelivery bool     `json:"actual_thread_delivery"`
	Stale                bool     `json:"stale,omitempty"`
	StaleAge             string   `json:"stale_age,omitempty"`
	SuggestedAction      string   `json:"suggested_action,omitempty"`
	SuggestedCommand     string   `json:"suggested_command,omitempty"`
	Superseded           bool     `json:"superseded,omitempty"`
	SupersededReason     string   `json:"superseded_reason,omitempty"`
	ReplacementHandoffID int64    `json:"replacement_handoff_id,omitempty"`
	SupersededEvidence   string   `json:"superseded_evidence,omitempty"`
	SupersededAt         string   `json:"superseded_at,omitempty"`
	CreatedAt            string   `json:"created_at,omitempty"`
	DeliveredAt          string   `json:"delivered_at,omitempty"`
}

type Supersede struct {
	HandoffID            int64
	Reason               string
	ReplacementHandoffID int64
	EvidencePath         string
	CreatedAt            string
}

type RowOptions struct {
	Now             time.Time
	AckTimeout      time.Duration
	TaskStatus      string
	LiveWindowPhase string
	Superseded      map[int64]Supersede
}

func RenderPayload(nextAction string, evidencePaths []string, approvalBoundary string) (string, error) {
	return RenderPayloadWithState(nextAction, "", evidencePaths, approvalBoundary)
}

func RenderPayloadWithState(nextAction, completionState string, evidencePaths []string, approvalBoundary string) (string, error) {
	nextAction = strings.TrimSpace(nextAction)
	if nextAction == "" {
		return "", fmt.Errorf("completion handback next action is required")
	}
	completionState = strings.TrimSpace(completionState)
	if completionState != "" && !ValidCompletionState(completionState) {
		return "", fmt.Errorf("invalid completion state %q", completionState)
	}
	payload := Payload{
		NextAction:       nextAction,
		CompletionState:  completionState,
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
	return RowsWithOptions(taskID, handoffs, notifications, RowOptions{})
}

func RowsWithOptions(taskID string, handoffs []store.Handoff, notifications []store.Notification, opts RowOptions) []Handback {
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
			CompletionState:  payload.CompletionState,
			EvidencePaths:    payload.EvidencePaths,
			ApprovalBoundary: payload.ApprovalBoundary,
			TaskStatus:       strings.TrimSpace(opts.TaskStatus),
			LiveWindowPhase:  strings.TrimSpace(opts.LiveWindowPhase),
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
		if supersede, ok := opts.Superseded[handoff.ID]; ok {
			row.Superseded = true
			row.SupersededReason = supersede.Reason
			row.ReplacementHandoffID = supersede.ReplacementHandoffID
			row.SupersededEvidence = supersede.EvidencePath
			row.SupersededAt = supersede.CreatedAt
			row.DeliveryStatus = "superseded"
			row.SuggestedAction = "inspect_superseded_completion_handback"
		}
		applyStaleness(&row, opts)
		applySuggestions(&row)
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
	payload.CompletionState = strings.TrimSpace(payload.CompletionState)
	payload.ApprovalBoundary = strings.TrimSpace(payload.ApprovalBoundary)
	payload.EvidencePaths = cleanList(payload.EvidencePaths)
	if payload.NextAction == "" {
		return Payload{}, false
	}
	if payload.CompletionState != "" && !ValidCompletionState(payload.CompletionState) {
		return Payload{}, false
	}
	return payload, true
}

func ValidCompletionState(state string) bool {
	switch strings.TrimSpace(state) {
	case "done", "reviewed", "merge-ready", "blocked-with-follow-up", "monitor-completed", "live-window-closeout", "live-window-next-decision":
		return true
	default:
		return false
	}
}

func CompletionStateList() []string {
	return []string{"blocked-with-follow-up", "done", "live-window-closeout", "live-window-next-decision", "merge-ready", "monitor-completed", "reviewed"}
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

func applyStaleness(row *Handback, opts RowOptions) {
	if IsResolved(*row) || opts.AckTimeout <= 0 {
		return
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	origin := strings.TrimSpace(row.DeliveredAt)
	if origin == "" {
		origin = row.CreatedAt
	}
	createdAt, err := time.Parse(time.RFC3339Nano, origin)
	if err != nil {
		return
	}
	age := now.Sub(createdAt)
	if age < opts.AckTimeout {
		return
	}
	row.Stale = true
	row.StaleAge = age.Truncate(time.Second).String()
	row.DeliveryStatus = "stale"
}

func applySuggestions(row *Handback) {
	switch row.DeliveryStatus {
	case "stale":
		row.SuggestedAction = "escalate_completion_handback"
		row.SuggestedCommand = fmt.Sprintf("fairway record notification %s --handoff-id %d --domain %s --state notification_failed --reason <reason>", row.TaskID, row.HandoffID, row.ToRole)
	case "pending":
		row.SuggestedAction = "deliver_or_record_completion_handback"
		row.SuggestedCommand = fmt.Sprintf("fairway record completion-handback %s --to %s --next-action %q --state thread_steered --provider <provider> --target <target>", row.TaskID, row.ToRole, row.NextAction)
	case "failed":
		row.SuggestedAction = "record_alternate_completion_handback_or_control_decision"
	case "delivered":
		row.SuggestedAction = "wait_for_next_owner_or_record_follow_up"
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
	if row.Superseded {
		return true
	}
	switch row.DeliveryStatus {
	case "delivered", "failed", "superseded":
		return true
	default:
		return false
	}
}

func SupersedesFromEvidence(evidence []store.Evidence) map[int64]Supersede {
	out := map[int64]Supersede{}
	for _, ev := range evidence {
		if strings.TrimSpace(ev.ArtifactType) != "completion-handback-superseded" {
			continue
		}
		values := keyValues(strings.Fields(ev.CommandText))
		handoffID := parsePositiveInt64(values["handoff_id"])
		if handoffID <= 0 {
			continue
		}
		out[handoffID] = Supersede{
			HandoffID:            handoffID,
			Reason:               strings.TrimSpace(ev.Notes),
			ReplacementHandoffID: parsePositiveInt64(values["replacement_handoff_id"]),
			EvidencePath:         strings.TrimSpace(ev.ArtifactPath),
			CreatedAt:            ev.CreatedAt,
		}
	}
	return out
}

func keyValues(fields []string) map[string]string {
	out := map[string]string{}
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func parsePositiveInt64(raw string) int64 {
	var value int64
	for _, r := range strings.TrimSpace(raw) {
		if r < '0' || r > '9' {
			return 0
		}
		value = value*10 + int64(r-'0')
	}
	return value
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
