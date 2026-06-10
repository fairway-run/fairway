package coordinator

import (
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

type ReadinessExplanation struct {
	ClaimableCount    int                        `json:"claimable_count"`
	NonReadyTodoCount int                        `json:"non_ready_todo_count"`
	Blockers          []ReadinessBlockerCategory `json:"blocker_categories,omitempty"`
}

type ReadinessBlockerCategory struct {
	Category   string   `json:"category"`
	Count      int      `json:"count"`
	TaskIDs    []string `json:"task_ids"`
	BlockerIDs []string `json:"blocker_task_ids,omitempty"`
	Suggested  string   `json:"suggested_next_command,omitempty"`
}

func ExplainReadyQueue(tasks, ready []store.Task, sessions []store.Session, checkpoints []store.Checkpoint, terminal []string) ReadinessExplanation {
	readyIDs := map[string]bool{}
	for _, task := range ready {
		readyIDs[task.Definition.ID] = true
	}
	terminalSet := map[string]bool{}
	if len(terminal) == 0 {
		terminal = []string{"done"}
	}
	for _, status := range terminal {
		terminalSet[status] = true
	}
	statuses := map[string]string{}
	for _, task := range tasks {
		statuses[task.Definition.ID] = task.Status
	}
	sessionsByTask := map[string][]store.Session{}
	for _, session := range sessions {
		if session.TaskID != "" {
			sessionsByTask[session.TaskID] = append(sessionsByTask[session.TaskID], session)
		}
	}
	checkpointByTask := latestCheckpointByTask(checkpoints)
	groups := map[string]*ReadinessBlockerCategory{}
	add := func(category, taskID string, blockers ...string) {
		group := groups[category]
		if group == nil {
			group = &ReadinessBlockerCategory{Category: category}
			groups[category] = group
		}
		group.Count++
		group.TaskIDs = appendUniqueString(group.TaskIDs, taskID)
		for _, blocker := range blockers {
			group.BlockerIDs = appendUniqueString(group.BlockerIDs, blocker)
		}
	}
	nonReadyTodo := 0
	for _, task := range tasks {
		if task.Status != "todo" || readyIDs[task.Definition.ID] {
			continue
		}
		nonReadyTodo++
		categorized := false
		for _, dep := range task.Definition.Dependencies {
			if !terminalSet[statuses[dep]] {
				add("dependency-blocked", task.Definition.ID, dep)
				categorized = true
			}
		}
		for _, session := range sessionsByTask[task.Definition.ID] {
			sessionText := strings.ToLower(session.Status + " " + session.SessionBackend + " " + session.Provider)
			if strings.Contains(sessionText, "approval") {
				add("approval-gated", task.Definition.ID)
				categorized = true
			} else if session.Status == "running" || session.Status == "starting" {
				add("session-gated", task.Definition.ID)
				categorized = true
			}
		}
		if checkpoint, ok := checkpointByTask[task.Definition.ID]; ok {
			checkpointText := strings.ToLower(checkpoint.State + " " + checkpoint.Summary)
			switch {
			case checkpoint.State == "review" || strings.Contains(checkpointText, "review"):
				add("review-gated", task.Definition.ID)
				categorized = true
			case checkpoint.State == "awaiting_input" && strings.Contains(checkpointText, "approval"):
				add("approval-gated", task.Definition.ID)
				categorized = true
			case strings.Contains(checkpointText, "profile") || strings.Contains(checkpointText, "gate"):
				add("profile-gated", task.Definition.ID)
				categorized = true
			}
		}
		if !categorized {
			add("unknown", task.Definition.ID)
		}
	}
	out := ReadinessExplanation{ClaimableCount: len(ready), NonReadyTodoCount: nonReadyTodo}
	for _, group := range groups {
		sort.Strings(group.TaskIDs)
		sort.Strings(group.BlockerIDs)
		group.Suggested = readinessSuggestion(group.Category, group.TaskIDs, group.BlockerIDs)
		out.Blockers = append(out.Blockers, *group)
	}
	sort.Slice(out.Blockers, func(i, j int) bool {
		if out.Blockers[i].Count != out.Blockers[j].Count {
			return out.Blockers[i].Count > out.Blockers[j].Count
		}
		return out.Blockers[i].Category < out.Blockers[j].Category
	})
	return out
}

func readinessSuggestion(category string, taskIDs, blockerIDs []string) string {
	switch category {
	case "dependency-blocked":
		if len(blockerIDs) > 0 {
			return "fairway task-detail " + blockerIDs[0]
		}
	case "review-gated", "approval-gated", "profile-gated", "session-gated", "unknown":
		if len(taskIDs) > 0 {
			return "fairway task-detail " + taskIDs[0]
		}
	}
	return "fairway list --status todo"
}

func appendUniqueString(values []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
