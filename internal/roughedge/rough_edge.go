package roughedge

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/store"
)

const ArtifactType = "rough-edge"

type Notes struct {
	Owner    string `json:"owner,omitempty"`
	Severity string `json:"severity,omitempty"`
	Decision string `json:"decision,omitempty"`
	Expires  string `json:"expires,omitempty"`
	Summary  string `json:"summary,omitempty"`
}

type Row struct {
	TaskID       string `json:"task_id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Owner        string `json:"owner"`
	Severity     string `json:"severity"`
	Decision     string `json:"decision"`
	Expires      string `json:"expires,omitempty"`
	Expired      bool   `json:"expired"`
	Summary      string `json:"summary"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	CreatedAt    string `json:"created_at"`
}

func Evidence(commandText, result, artifactPath string, notes Notes) (store.Evidence, error) {
	payload, err := json.Marshal(notes)
	if err != nil {
		return store.Evidence{}, err
	}
	return store.Evidence{
		CommandText:  commandText,
		Result:       result,
		ArtifactPath: artifactPath,
		ArtifactType: ArtifactType,
		Notes:        string(payload),
	}, nil
}

func Rows(ctx context.Context, s *store.Store, now time.Time) ([]Row, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return nil, err
	}
	rows := []Row{}
	for _, listed := range tasks {
		task, _, evidence, _, _, err := s.TaskDetail(ctx, listed.Definition.ID)
		if err != nil {
			return nil, err
		}
		for _, ev := range evidence {
			if strings.TrimSpace(ev.ArtifactType) != ArtifactType {
				continue
			}
			var notes Notes
			_ = json.Unmarshal([]byte(ev.Notes), &notes)
			row := Row{
				TaskID:       task.Definition.ID,
				Title:        task.Definition.Title,
				Status:       task.Status,
				Owner:        firstNonEmpty(notes.Owner, task.Owner, task.Definition.Role),
				Severity:     firstNonEmpty(notes.Severity, "medium"),
				Decision:     firstNonEmpty(notes.Decision, "defer"),
				Expires:      strings.TrimSpace(notes.Expires),
				Summary:      firstNonEmpty(notes.Summary, ev.CommandText),
				ArtifactPath: ev.ArtifactPath,
				CreatedAt:    ev.CreatedAt,
			}
			if expires, ok := ParseExpiry(row.Expires); ok {
				row.Expired = expires.Before(now)
			}
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Expired != rows[j].Expired {
			return rows[i].Expired
		}
		if severityRank(rows[i].Severity) != severityRank(rows[j].Severity) {
			return severityRank(rows[i].Severity) > severityRank(rows[j].Severity)
		}
		if rows[i].CreatedAt != rows[j].CreatedAt {
			return rows[i].CreatedAt > rows[j].CreatedAt
		}
		return rows[i].TaskID < rows[j].TaskID
	})
	return rows, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func severityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func ParseExpiry(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
