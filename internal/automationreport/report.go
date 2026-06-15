package automationreport

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/store"
)

type Options struct {
	Since     time.Duration
	Threshold int
	Now       time.Time
}

type Report struct {
	OK         bool        `json:"ok"`
	Since      string      `json:"since"`
	Threshold  int         `json:"threshold"`
	Candidates []Candidate `json:"candidates,omitempty"`
}

type Candidate struct {
	Kind                      string   `json:"kind"`
	Pattern                   string   `json:"pattern"`
	Frequency                 int      `json:"frequency"`
	RecentTaskIDs             []string `json:"recent_task_ids,omitempty"`
	RepresentativeCommands    []string `json:"representative_commands,omitempty"`
	RepresentativeArtifacts   []string `json:"representative_artifacts,omitempty"`
	EstimatedCoordinationCost string   `json:"estimated_coordination_cost"`
	LikelyOwner               string   `json:"likely_owner,omitempty"`
	SuggestedSurface          string   `json:"suggested_surface"`
	RecommendedAction         string   `json:"recommended_action"`
}

type signal struct {
	kind     string
	pattern  string
	taskID   string
	role     string
	command  string
	artifact string
	at       time.Time
}

func Build(ctx context.Context, s *store.Store, opts Options) (Report, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if opts.Since <= 0 {
		opts.Since = 7 * 24 * time.Hour
	}
	if opts.Threshold <= 0 {
		opts.Threshold = 3
	}
	start := now.Add(-opts.Since)
	tasks, err := s.AllTasks(ctx)
	if err != nil {
		return Report{}, err
	}
	var signals []signal
	for _, task := range tasks {
		_, _, evidence, _, _, err := s.TaskDetail(ctx, task.Definition.ID)
		if err != nil {
			return Report{}, err
		}
		for _, ev := range evidence {
			at, ok := parseTime(ev.CreatedAt)
			if !ok || at.Before(start) {
				continue
			}
			if command := normalizeCommand(ev.CommandText); command != "" {
				signals = append(signals, signal{
					kind:     "command",
					pattern:  command,
					taskID:   task.Definition.ID,
					role:     firstNonEmpty(task.Definition.OwningDomain, task.Definition.Role),
					command:  ev.CommandText,
					artifact: ev.ArtifactPath,
					at:       at,
				})
			}
			if pattern := evidencePattern(ev); pattern != "" {
				signals = append(signals, signal{
					kind:     "evidence",
					pattern:  pattern,
					taskID:   task.Definition.ID,
					role:     firstNonEmpty(task.Definition.OwningDomain, task.Definition.Role),
					command:  ev.CommandText,
					artifact: ev.ArtifactPath,
					at:       at,
				})
			}
		}
		notifications, err := s.Notifications(ctx, task.Definition.ID)
		if err != nil {
			return Report{}, err
		}
		for _, notification := range notifications {
			at, ok := parseTime(notification.CreatedAt)
			if !ok || at.Before(start) {
				continue
			}
			pattern := strings.TrimSpace(notification.Domain + ":" + notification.State)
			if pattern == ":" {
				continue
			}
			signals = append(signals, signal{
				kind:    "notification",
				pattern: pattern,
				taskID:  task.Definition.ID,
				role:    firstNonEmpty(notification.Domain, task.Definition.Role),
				at:      at,
			})
		}
	}
	report := Report{OK: true, Since: opts.Since.String(), Threshold: opts.Threshold}
	for _, candidate := range candidatesFromSignals(signals, opts.Threshold) {
		report.Candidates = append(report.Candidates, candidate)
	}
	sort.SliceStable(report.Candidates, func(i, j int) bool {
		if report.Candidates[i].Frequency != report.Candidates[j].Frequency {
			return report.Candidates[i].Frequency > report.Candidates[j].Frequency
		}
		if report.Candidates[i].Kind != report.Candidates[j].Kind {
			return report.Candidates[i].Kind < report.Candidates[j].Kind
		}
		return report.Candidates[i].Pattern < report.Candidates[j].Pattern
	})
	return report, nil
}

func candidatesFromSignals(signals []signal, threshold int) []Candidate {
	groups := map[string][]signal{}
	for _, sig := range signals {
		key := sig.kind + "|" + sig.pattern
		groups[key] = append(groups[key], sig)
	}
	var candidates []Candidate
	for _, group := range groups {
		if len(group) < threshold {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool { return group[i].at.After(group[j].at) })
		first := group[0]
		candidates = append(candidates, Candidate{
			Kind:                      first.kind,
			Pattern:                   first.pattern,
			Frequency:                 len(group),
			RecentTaskIDs:             recentTaskIDs(group, 5),
			RepresentativeCommands:    representativeCommands(group, 3),
			RepresentativeArtifacts:   representativeArtifacts(group, 3),
			EstimatedCoordinationCost: estimatedCost(len(group)),
			LikelyOwner:               likelyOwner(group),
			SuggestedSurface:          suggestedSurface(first.kind, first.pattern),
			RecommendedAction:         recommendedAction(first.kind, first.pattern),
		})
	}
	return candidates
}

func normalizeCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	lower := strings.ToLower(command)
	replacements := []string{"--task ", "--task-id "}
	for _, marker := range replacements {
		if idx := strings.Index(lower, marker); idx >= 0 {
			end := idx + len(marker)
			rest := command[end:]
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				command = command[:end] + "<task>" + strings.TrimPrefix(rest[len(fields[0]):], " ")
			}
		}
	}
	fields := strings.Fields(command)
	for i, field := range fields {
		if looksLikeTaskID(field) {
			fields[i] = "<task>"
		}
	}
	return strings.Join(fields, " ")
}

func looksLikeTaskID(value string) bool {
	value = strings.Trim(value, ".,;:()[]{}\"'")
	if !strings.Contains(value, "-") {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '-':
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}

func evidencePattern(ev store.Evidence) string {
	text := strings.ToLower(strings.Join([]string{ev.CommandText, ev.ArtifactType, ev.Notes}, " "))
	switch {
	case strings.Contains(text, "merge-ready"):
		return "merge-ready check"
	case strings.Contains(text, "review-waits"):
		return "review-wait check"
	case strings.Contains(text, "preflight"):
		return "preflight packet or gate"
	case strings.Contains(text, "redaction") || strings.Contains(text, "secret"):
		return "evidence redaction"
	case strings.Contains(text, "delivery report") || strings.Contains(text, "process overhead"):
		return "delivery-overhead reporting"
	case strings.Contains(text, "commit") || strings.Contains(text, "git diff --check"):
		return "commit-boundary handling"
	case strings.Contains(text, "ci") || strings.Contains(text, "deploy"):
		return "ci/deploy handback"
	default:
		return ""
	}
}

func suggestedSurface(kind, pattern string) string {
	text := strings.ToLower(kind + " " + pattern)
	switch {
	case strings.Contains(text, "review-wait"), strings.Contains(text, "notification"):
		return "watcher"
	case strings.Contains(text, "packet"), strings.Contains(text, "preflight"):
		return "packet template"
	case strings.Contains(text, "delivery"), strings.Contains(text, "report"):
		return "dashboard panel"
	case strings.Contains(text, "merge-ready"), strings.Contains(text, "commit"):
		return "fairway cli"
	default:
		return "script"
	}
}

func recommendedAction(kind, pattern string) string {
	return "capture checklist or implement bounded " + suggestedSurface(kind, pattern) + " for repeated " + pattern
}

func estimatedCost(frequency int) string {
	switch {
	case frequency >= 10:
		return "high"
	case frequency >= 5:
		return "medium"
	default:
		return "low"
	}
}

func recentTaskIDs(group []signal, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, sig := range group {
		if sig.taskID == "" || seen[sig.taskID] {
			continue
		}
		seen[sig.taskID] = true
		out = append(out, sig.taskID)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func representativeCommands(group []signal, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, sig := range group {
		if sig.command == "" || seen[sig.command] {
			continue
		}
		seen[sig.command] = true
		out = append(out, sig.command)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func representativeArtifacts(group []signal, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, sig := range group {
		if sig.artifact == "" || seen[sig.artifact] {
			continue
		}
		seen[sig.artifact] = true
		out = append(out, sig.artifact)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func likelyOwner(group []signal) string {
	counts := map[string]int{}
	for _, sig := range group {
		if sig.role != "" {
			counts[sig.role]++
		}
	}
	best := ""
	for role, count := range counts {
		if best == "" || count > counts[best] || (count == counts[best] && role < best) {
			best = role
		}
	}
	return best
}

func parseTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
