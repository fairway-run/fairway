package livewindow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

var phases = map[string]bool{
	"packet-prepared":    true,
	"reviews-routed":     true,
	"approvals-readback": true,
	"gate-authorized":    true,
	"gate-running":       true,
	"closeout":           true,
	"next-decision":      true,
}

type Status struct {
	TaskID        string `json:"task_id"`
	Phase         string `json:"phase"`
	NextOwner     string `json:"next_owner,omitempty"`
	NextAction    string `json:"next_action,omitempty"`
	TargetCloseBy string `json:"target_close_by,omitempty"`
	ArtifactPath  string `json:"artifact_path,omitempty"`
	CheckpointAt  string `json:"checkpoint_at,omitempty"`
	Summary       string `json:"summary,omitempty"`
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
	phase = strings.TrimSpace(phase)
	if !ValidPhase(phase) {
		return "", fmt.Errorf("invalid live-window phase %q", phase)
	}
	parts := []string{"live-window", "phase=" + phase}
	if strings.TrimSpace(nextOwner) != "" {
		parts = append(parts, "next_owner="+encodeToken(nextOwner))
	}
	if strings.TrimSpace(nextAction) != "" {
		parts = append(parts, "next_action="+encodeToken(nextAction))
	}
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
		}
	}
	if !ValidPhase(status.Phase) {
		return Status{}, false
	}
	return status, true
}

func encodeToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "\t", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	return value
}

func decodeToken(value string) string {
	return strings.ReplaceAll(value, "_", " ")
}
