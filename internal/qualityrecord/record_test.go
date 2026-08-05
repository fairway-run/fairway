package qualityrecord

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/store"
)

func TestBuildProjectsCitedStatesWithoutInventingAuthority(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "fairway.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.ImportTasks(ctx, []store.TaskDefinition{
		{ID: "T-001", Title: "Quality record", Role: "backend", Kind: "task", AcceptanceChecks: []string{"record is cited"}},
		{ID: "T-002", Title: "Corrective", Role: "backend", Kind: "task"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "fail", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEvidence(ctx, "T-001", store.Evidence{CommandText: "go test ./...", Result: "pass", ArtifactType: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReview(ctx, "T-001", store.Review{Reviewer: "arch", Domain: "arch", Verdict: "approve"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordTaskOutcome(ctx, store.TaskOutcome{TaskID: "T-001", Kind: "corrective", RelatedTaskID: "T-002"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkControlFrictionUnavailable(ctx, store.ControlFrictionSample{TaskID: "T-001", ControlID: "review:arch", Reason: "historical timing unavailable"}); err != nil {
		t.Fatal(err)
	}

	record, err := Build(ctx, s, "T-001", time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if record.Schema != Schema || !record.Advisory || record.AuthorityBoundary == "" || len(record.Sections) != 9 {
		t.Fatalf("record=%+v", record)
	}
	states := map[string]string{}
	for _, section := range record.Sections {
		states[section.ID] = section.State
		if len(section.Sources) == 0 {
			t.Fatalf("section %s lacks durable or expected source citation", section.ID)
		}
		for _, source := range section.Sources {
			switch source.Availability {
			case "present", "missing", "unavailable", "conflicting", "externally_owned":
			default:
				t.Fatalf("section %s source=%+v", section.ID, source)
			}
		}
	}
	if states["intent"] != "present" || states["verification"] != "conflicting" || states["judgment"] != "present" || states["promotion"] != "externally_owned" || states["outcomes"] != "present" || states["lessons"] != "present" {
		t.Fatalf("states=%+v", states)
	}
	if record.Summary.Conflicting != 1 || record.Summary.ExternallyOwned != 1 {
		t.Fatalf("summary=%+v", record.Summary)
	}
}

func TestBuildKeepsMissingAndUnavailableDistinct(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "fairway.db"), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.ImportTasks(ctx, []store.TaskDefinition{{ID: "T-001", Title: "Sparse", Role: "backend", Kind: "task"}}); err != nil {
		t.Fatal(err)
	}
	record, err := Build(ctx, s, "T-001", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, section := range record.Sections {
		states[section.ID] = section.State
	}
	if states["intent"] != "missing" || states["decisions"] != "missing" || states["outcomes"] != "unavailable" {
		t.Fatalf("states=%+v", states)
	}
}
