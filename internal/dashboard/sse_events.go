package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/subashram/fairway/internal/reviewstate"
	"github.com/subashram/fairway/internal/store"
)

const dashboardEventPollLimit = 1000

type sseEvent struct {
	ID      string
	Name    string
	Payload map[string]any
}

func sseEventsFromSource(src store.EventSource) []sseEvent {
	id := fmt.Sprintf("%s:%d:%d:%s", src.Cursor.At, src.Cursor.SourceOrder, src.Cursor.ID, src.Source)
	at := src.Cursor.At
	switch src.Source {
	case "history":
		if src.ToStatus == "in_progress" && src.Reason == "claim" {
			return []sseEvent{{
				ID:   id,
				Name: "claim",
				Payload: map[string]any{
					"task_id": src.TaskID,
					"role":    src.Role,
					"owner":   firstNonEmpty(src.Owner, src.Role),
					"at":      at,
				},
			}}
		}
		if src.ToStatus == "done" {
			return []sseEvent{{
				ID:   id,
				Name: "done",
				Payload: map[string]any{
					"task_id": src.TaskID,
					"role":    src.Role,
					"owner":   firstNonEmpty(src.Owner, src.Role),
					"at":      at,
				},
			}}
		}
		return []sseEvent{{
			ID:   id,
			Name: "status_change",
			Payload: map[string]any{
				"task_id": src.TaskID,
				"role":    src.Role,
				"from":    src.FromStatus,
				"to":      src.ToStatus,
				"actor":   src.Actor,
				"at":      at,
			},
		}}
	case "evidence":
		return []sseEvent{{
			ID:   id,
			Name: "evidence",
			Payload: map[string]any{
				"task_id": src.TaskID,
				"role":    src.Role,
				"kind":    firstNonEmpty(src.EvidenceType, "evidence"),
				"count":   src.EvidenceCount,
				"at":      at,
			},
		}}
	case "handoff":
		return []sseEvent{{
			ID:   id,
			Name: "handoff",
			Payload: map[string]any{
				"task_id":   src.TaskID,
				"from_role": src.FromRole,
				"to_role":   src.ToRole,
				"actor":     src.Actor,
				"reason":    src.Reason,
				"at":        at,
			},
		}}
	case "review":
		return []sseEvent{{
			ID:   id,
			Name: "review_verdict",
			Payload: map[string]any{
				"task_id":         src.TaskID,
				"role":            src.Role,
				"verdict":         src.Verdict,
				"reviewer_domain": src.Reviewer,
				"at":              at,
			},
		}}
	case "session_attach", "session_heartbeat", "session_detach":
		name := src.Source
		payload := map[string]any{
			"task_id":    src.TaskID,
			"role":       src.Role,
			"provider":   src.Provider,
			"session_id": src.SessionID,
			"at":         at,
		}
		if name == "session_detach" {
			payload["reason"] = src.EndReason
		}
		return []sseEvent{{ID: id, Name: name, Payload: payload}}
	default:
		return nil
	}
}

func (s *Server) gateChangeEvents(ctx context.Context, sourceID, at string) ([]sseEvent, error) {
	if len(s.cfg.WorkstreamProfiles) == 0 {
		return nil, nil
	}
	tasks, err := s.store.AllTasks(ctx)
	if err != nil {
		return nil, err
	}
	gates, err := s.dashboardGateStatuses(ctx, tasks, 0)
	if err != nil {
		return nil, err
	}
	out := make([]sseEvent, 0, len(gates))
	for _, gate := range gates {
		out = append(out, sseEvent{
			ID:   sourceID + ":gate:" + gate.Profile + ":" + gate.Name,
			Name: "gate_change",
			Payload: map[string]any{
				"profile":   gate.Profile,
				"gate":      gate.Name,
				"satisfied": gate.SatisfiedCount,
				"total":     gate.TaskCount,
				"at":        at,
			},
		})
	}
	return out, nil
}

func (s *Server) reviewWaitEvents(ctx context.Context, at string) ([]sseEvent, error) {
	waits, err := s.reviewWaits(ctx, "", dashboardEventTime(at))
	if err != nil {
		return nil, err
	}
	out := make([]sseEvent, 0, len(waits))
	for _, wait := range waits {
		name := reviewWaitEventName(wait)
		if name == "" {
			continue
		}
		out = append(out, sseEvent{
			ID:   "review_wait:" + wait.WaitID + ":" + wait.State + ":" + wait.Action + ":" + wait.ExpectedResponseAt + ":" + wait.ResolvedAt,
			Name: name,
			Payload: map[string]any{
				"task_id":              wait.TaskID,
				"wait_id":              wait.WaitID,
				"domain":               wait.Domain,
				"state":                wait.State,
				"blocking":             wait.Blocking,
				"action":               wait.Action,
				"target_provider":      wait.TargetProvider,
				"target_id":            wait.TargetID,
				"last_notified_at":     wait.LastNotifiedAt,
				"expected_response_at": wait.ExpectedResponseAt,
				"resolved_at":          wait.ResolvedAt,
				"resolved_by":          wait.ResolvedBy,
				"reason":               wait.Reason,
				"at":                   at,
			},
		})
	}
	return out, nil
}

func reviewWaitEventName(wait reviewstate.ReviewWait) string {
	switch wait.State {
	case "stale":
		return "review_wait.stale"
	case "notification_failed":
		return "review_wait.notification_failed"
	case "resolved":
		return "review_wait.resolved"
	default:
		return ""
	}
}

func dashboardEventTime(at string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, at); err == nil {
		return parsed
	}
	return time.Now().UTC()
}

func writeSSEEvent(w io.Writer, event sseEvent) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Name, payload)
	return err
}

func writeLegacyRefresh(w io.Writer, sourceID string) error {
	_, err := fmt.Fprintf(w, "event: refresh\ndata: %q\n\n", sourceID)
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
