package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/knowledge"
	"github.com/subashram/fairway/internal/store"
)

func TestBuildMemoryPacketLabelsStateChronologyAndDeduplicatesGuidance(t *testing.T) {
	memory := store.TrackMemory{
		TrackID: "TRACK-1", Disposition: "active",
		Blockers:    []string{"DEP-1 is blocked", "DEP-1 is blocked"},
		NextActions: []string{"run dependency-safe cases", "run dependency-safe cases"},
	}
	tasks := []store.Task{
		{Definition: store.TaskDefinition{ID: "TRACK-1", Title: "Track"}, Status: "todo"},
		{Definition: store.TaskDefinition{ID: "DEP-1", Title: "Dependency"}, Status: "blocked"},
		{Definition: store.TaskDefinition{ID: "UNRELATED-1", Title: "Unrelated"}, Status: "blocked"},
	}
	checkpoints := []store.Checkpoint{
		{TaskID: "TRACK-1", State: "done", Summary: "newer", CreatedAt: "2026-07-23T02:00:00Z"},
		{TaskID: "TRACK-1", State: "active", Summary: "older", CreatedAt: "2026-07-23T01:00:00Z"},
	}

	packet := buildMemoryPacket(memory, "isolated", tasks, nil, checkpoints, []string{"done"})
	if packet.MemoryDisposition != "active" || packet.TrackTaskStatus != "todo" ||
		packet.CheckpointChronology != "newest_first_historical" {
		t.Fatalf("state labels=%+v", packet)
	}
	if !reflect.DeepEqual(packet.Blockers, []string{"DEP-1 is blocked"}) {
		t.Fatalf("blockers=%v", packet.Blockers)
	}
	if !reflect.DeepEqual(packet.NextActions, []string{"run dependency-safe cases"}) {
		t.Fatalf("next actions=%v", packet.NextActions)
	}
	if len(packet.Checkpoints) != 2 ||
		!strings.Contains(packet.Checkpoints[0], "at=2026-07-23T02:00:00Z") ||
		!strings.Contains(packet.Checkpoints[0], "historical=true") {
		t.Fatalf("checkpoint chronology=%v", packet.Checkpoints)
	}
	for _, action := range packet.NextActions {
		if strings.HasPrefix(action, "inspect ") {
			t.Fatalf("synthetic inspect action remained: %v", packet.NextActions)
		}
	}
}

func TestBuildMemoryPacketAddsOnlyCurrentTrackBlocker(t *testing.T) {
	memory := store.TrackMemory{TrackID: "TRACK-1", Disposition: "active"}
	tasks := []store.Task{
		{Definition: store.TaskDefinition{ID: "TRACK-1", Title: "Track"}, Status: "blocked"},
		{Definition: store.TaskDefinition{ID: "TRACK-10", Title: "Related context"}, Status: "blocked"},
	}

	packet := buildMemoryPacket(memory, "", tasks, nil, nil, []string{"done"})
	if !reflect.DeepEqual(packet.Blockers, []string{"TRACK-1 blocked"}) {
		t.Fatalf("blockers=%v", packet.Blockers)
	}
	if containsTaskReference([]string{"TRACK-10 blocked"}, "TRACK-1") {
		t.Fatal("task reference matching accepted a partial task id")
	}
}

func TestBuildMemoryPacketMarksTerminalTaskGuidanceHistorical(t *testing.T) {
	memory := store.TrackMemory{
		TrackID:          "TRACK-1",
		Disposition:      "active",
		CurrentObjective: "finish the old task",
		NextActions:      []string{"perform the old next step"},
	}
	tasks := []store.Task{
		{Definition: store.TaskDefinition{ID: "TRACK-1", Title: "Track"}, Status: "done"},
	}

	packet := buildMemoryPacket(memory, "", tasks, nil, nil, []string{"done"})
	if packet.Actionability != "historical_terminal_task" {
		t.Fatalf("actionability=%q", packet.Actionability)
	}
	if packet.CurrentObjectiveActionable || packet.NextActionsActionable {
		t.Fatalf("terminal guidance remained actionable: %+v", packet)
	}
	if !reflect.DeepEqual(packet.NextActions, memory.NextActions) {
		t.Fatalf("historical guidance was not preserved: %v", packet.NextActions)
	}
}

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

func TestComposeColdStartKnowledgeRendersSharedEvidenceIdentityOnce(t *testing.T) {
	packet := memoryPacket{Track: store.TrackMemory{SourceEvidenceIDs: []int64{7, 9}}}
	query := knowledge.QueryPacket{
		Schema: "fairway.knowledge-query.v1",
		Sources: []knowledge.QuerySource{
			{Key: "fairway:evidence:7", Authority: "canonical"},
			{Key: "fairway:decision:8"},
			{Key: "file:docs/design/example.md"},
		},
		Bounded: true, ReadOnly: true,
	}
	composedPacket, composedQuery, err := composeColdStartKnowledge(packet, query, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(composedPacket.Track.SourceEvidenceIDs, []int64{9}) {
		t.Fatalf("shared evidence remained in memory packet: %v", composedPacket.Track.SourceEvidenceIDs)
	}
	if !composedQuery.Sources[0].MemoryReferenced || composedQuery.Sources[0].Authority != "canonical" {
		t.Fatalf("shared evidence lost memory relationship or authority: %+v", composedQuery.Sources[0])
	}
	rendered, err := json.Marshal(struct {
		Packet    memoryPacket          `json:"packet"`
		Knowledge knowledge.QueryPacket `json:"knowledge"`
	}{Packet: composedPacket, Knowledge: composedQuery})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(rendered), "fairway:evidence:7") != 1 {
		t.Fatalf("shared evidence identity was not rendered exactly once: %s", rendered)
	}
}

func TestFinalizeColdStartKnowledgeRechecksSeparateBudgetAfterMemoryAnnotation(t *testing.T) {
	packet := knowledge.QueryPacket{
		Schema: "fairway.knowledge-query.v1",
		Topic:  "node trust",
		Sources: []knowledge.QuerySource{{
			Key:       "fairway:evidence:7",
			Authority: "canonical",
			Citations: []knowledge.QuerySourceCitation{{Page: "architecture/node.md", Verified: true}},
		}},
		Bounded: true, ReadOnly: true,
	}
	if err := knowledge.FinalizeQueryPacket(&packet); err != nil {
		t.Fatal(err)
	}
	beforeAnnotation := packet.Bytes
	memoryPacket := memoryPacket{Track: store.TrackMemory{SourceEvidenceIDs: []int64{7}}}
	if _, _, err := composeColdStartKnowledge(memoryPacket, packet, beforeAnnotation); err == nil {
		t.Fatal("cold-start accepted packet that exceeded budget after memory annotation")
	}

	resultPacket, result, err := composeColdStartKnowledge(memoryPacket, packet, beforeAnnotation+256)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Bytes != len(rendered) {
		t.Fatalf("bytes=%d rendered=%d", result.Bytes, len(rendered))
	}
	if !result.Sources[0].MemoryReferenced || result.Sources[0].Authority != "canonical" {
		t.Fatalf("memory annotation discarded source authority: %+v", result.Sources[0])
	}
	if len(resultPacket.Track.SourceEvidenceIDs) != 0 {
		t.Fatalf("shared evidence identity remained duplicated: %v", resultPacket.Track.SourceEvidenceIDs)
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
