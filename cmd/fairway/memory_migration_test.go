package main

import (
	"reflect"
	"testing"

	"github.com/subashram/fairway/internal/knowledge"
	"github.com/subashram/fairway/internal/store"
)

func TestTrackScopedColdStartFactsIncludesSourceTasksDescendantsAndDependencies(t *testing.T) {
	priority := 1
	tasks := []store.Task{
		{Definition: store.TaskDefinition{ID: "TRACK-1", Priority: &priority}},
		{Definition: store.TaskDefinition{ID: "CHILD-1", ParentID: "TRACK-1", Dependencies: []string{"DEP-1"}}},
		{Definition: store.TaskDefinition{ID: "DEP-1", Dependencies: []string{"DEP-2"}}},
		{Definition: store.TaskDefinition{ID: "DEP-2"}},
		{Definition: store.TaskDefinition{ID: "SOURCE-1"}},
		{Definition: store.TaskDefinition{ID: "SOURCE-CHILD", ParentID: "SOURCE-1"}},
		{Definition: store.TaskDefinition{ID: "OTHER"}},
	}
	sessions := []store.Session{{ID: "track", TaskID: "CHILD-1"}, {ID: "source", TaskID: "SOURCE-1"}, {ID: "other", TaskID: "OTHER"}}
	checkpoints := []store.Checkpoint{{ID: 1, TaskID: "DEP-2"}, {ID: 2, TaskID: "SOURCE-CHILD"}, {ID: 3, TaskID: "OTHER"}}

	scopedTasks, scopedSessions, scopedCheckpoints := trackScopedColdStartFacts(store.TrackMemory{TrackID: "TRACK-1"}, []string{"SOURCE-1"}, tasks, sessions, checkpoints)
	if got := taskIDs(scopedTasks); !reflect.DeepEqual(got, []string{"TRACK-1", "CHILD-1", "DEP-1", "DEP-2", "SOURCE-1", "SOURCE-CHILD"}) {
		t.Fatalf("task ids=%v", got)
	}
	if got := sessionIDs(scopedSessions); !reflect.DeepEqual(got, []string{"track", "source"}) {
		t.Fatalf("session ids=%v", got)
	}
	if got := checkpointIDs(scopedCheckpoints); !reflect.DeepEqual(got, []int64{1, 2}) {
		t.Fatalf("checkpoint ids=%v", got)
	}
}

func TestDedupeKnowledgeSourcesWithMemoryRendersSharedEvidenceOnce(t *testing.T) {
	sources := []knowledge.QuerySource{
		{Key: "fairway:evidence:7"},
		{Key: "fairway:decision:8"},
		{Key: "file:docs/design/example.md"},
	}
	result := dedupeKnowledgeSourcesWithMemory(sources, store.TrackMemory{SourceEvidenceIDs: []int64{7}})
	if len(result) != 2 {
		t.Fatalf("deduplicated sources=%+v", result)
	}
	if got := []string{result[0].Key, result[1].Key}; !reflect.DeepEqual(got, []string{"fairway:decision:8", "file:docs/design/example.md"}) {
		t.Fatalf("deduplicated sources=%v", got)
	}
}

func taskIDs(tasks []store.Task) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.Definition.ID)
	}
	return out
}

func sessionIDs(sessions []store.Session) []string {
	out := make([]string, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, session.ID)
	}
	return out
}

func checkpointIDs(checkpoints []store.Checkpoint) []int64 {
	out := make([]int64, 0, len(checkpoints))
	for _, checkpoint := range checkpoints {
		out = append(out, checkpoint.ID)
	}
	return out
}
