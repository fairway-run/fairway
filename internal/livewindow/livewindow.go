package livewindow

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

var phases = map[string]bool{
	"packet-prepared":      true,
	"reviews-routed":       true,
	"approvals-readback":   true,
	"gate-authorized":      true,
	"gate-running":         true,
	"closeout":             true,
	"next-decision":        true,
	"packet_ready":         true,
	"approvals_ready":      true,
	"execution_authorized": true,
	"operator_running":     true,
	"closeout_required":    true,
	"done":                 true,
	"blocked":              true,
}

type Status struct {
	TaskID               string `json:"task_id"`
	Phase                string `json:"phase"`
	NextOwner            string `json:"next_owner,omitempty"`
	NextAction           string `json:"next_action,omitempty"`
	AuthorizationState   string `json:"authorization_state,omitempty"`
	Prompt               string `json:"prompt,omitempty"`
	Command              string `json:"command,omitempty"`
	MissedDeadlineAction string `json:"missed_deadline_action,omitempty"`
	TargetCloseBy        string `json:"target_close_by,omitempty"`
	ArtifactPath         string `json:"artifact_path,omitempty"`
	CheckpointAt         string `json:"checkpoint_at,omitempty"`
	Summary              string `json:"summary,omitempty"`
}

type SummaryOptions struct {
	Phase                string
	NextOwner            string
	NextAction           string
	AuthorizationState   string
	Prompt               string
	Command              string
	MissedDeadlineAction string
}

func ValidPhase(phase string) bool {
	return phases[strings.TrimSpace(phase)]
}

func PhaseList() []string {
	out := make([]string, 0, len(phases))
	for phase := range phases {
		out = append(out, phase)
	}
	sort.Strings(out)
	return out
}

func Summary(phase, nextOwner, nextAction string) (string, error) {
	return SummaryWithOptions(SummaryOptions{Phase: phase, NextOwner: nextOwner, NextAction: nextAction})
}

func SummaryWithOptions(opts SummaryOptions) (string, error) {
	phase := strings.TrimSpace(opts.Phase)
	if !ValidPhase(phase) {
		return "", fmt.Errorf("invalid live-window phase %q", phase)
	}
	parts := []string{"live-window", "phase=" + phase}
	appendField := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, key+"="+encodeToken(value))
		}
	}
	appendField("next_owner", opts.NextOwner)
	appendField("next_action", opts.NextAction)
	appendField("authorization", opts.AuthorizationState)
	appendField("prompt", opts.Prompt)
	appendField("command", opts.Command)
	appendField("missed_deadline_action", opts.MissedDeadlineAction)
	return strings.Join(parts, " "), nil
}

func StatusesFromCheckpoints(checkpoints []store.Checkpoint) []Status {
	seen := map[string]bool{}
	var out []Status
	for _, checkpoint := range checkpoints {
		if seen[checkpoint.TaskID] {
			continue
		}
		status, ok := StatusFromCheckpoint(checkpoint)
		if !ok {
			continue
		}
		seen[checkpoint.TaskID] = true
		out = append(out, status)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TaskID < out[j].TaskID })
	return out
}

func StatusFromCheckpoint(checkpoint store.Checkpoint) (Status, bool) {
	fields := strings.Fields(checkpoint.Summary)
	if len(fields) == 0 || fields[0] != "live-window" {
		return Status{}, false
	}
	status := Status{
		TaskID:        checkpoint.TaskID,
		TargetCloseBy: checkpoint.TargetCloseBy,
		ArtifactPath:  checkpoint.ArtifactPath,
		CheckpointAt:  checkpoint.CreatedAt,
		Summary:       checkpoint.Summary,
	}
	for _, field := range fields[1:] {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch key {
		case "phase":
			status.Phase = strings.TrimSpace(value)
		case "next_owner":
			status.NextOwner = decodeToken(value)
		case "next_action":
			status.NextAction = decodeToken(value)
		case "authorization":
			status.AuthorizationState = decodeToken(value)
		case "prompt":
			status.Prompt = decodeToken(value)
		case "command":
			status.Command = decodeToken(value)
		case "missed_deadline_action":
			status.MissedDeadlineAction = decodeToken(value)
		}
	}
	if !ValidPhase(status.Phase) {
		return Status{}, false
	}
	return status, true
}

func encodeToken(value string) string {
	return url.QueryEscape(strings.TrimSpace(value))
}

func decodeToken(value string) string {
	if strings.Contains(value, "%") || strings.Contains(value, "+") {
		if decoded, err := url.QueryUnescape(value); err == nil {
			return decoded
		}
	}
	return strings.ReplaceAll(value, "_", " ")
}
