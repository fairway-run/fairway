package memorymigration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExtractsBoundedMemoryWithoutRawBody(t *testing.T) {
	root := t.TempDir()
	path := writeMemoryFile(t, root, "tmp-ux/program-memory.md", `# Program Memory

## Purpose
Keep provider-neutral context.

## Current Objective
Finish migration.

## Decisions
- Fairway is authoritative.
- Store curated facts only.

## Blockers
- None.

## Open Questions
- Which pilot runs first?

## Next Actions
1. Run focused tests.

Owner: backend
Review by: 2099-01-01

## Source Checkpoint IDs
- 7

## Source Evidence IDs
- 8

## Source Review IDs
- 9

## Notes
This unsupported sentinel must not be retained.

`+"```"+`text
raw command output omitted
`+"```"+`
`)
	document, err := Load(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if document.Path != "tmp-ux/program-memory.md" || !document.RawOmitted || len(document.SHA256) != 64 {
		t.Fatalf("document metadata = %+v", document)
	}
	proposal := document.Proposal
	if proposal.Title != "Program Memory" || proposal.Purpose != "Keep provider-neutral context." || proposal.CurrentObjective != "Finish migration." {
		t.Fatalf("proposal scalars = %+v", proposal)
	}
	if len(proposal.Decisions) != 2 || proposal.NextActions[0] != "Run focused tests." || proposal.Owner != "backend" || proposal.ReviewBy != "2099-01-01" {
		t.Fatalf("proposal fields = %+v", proposal)
	}
	if proposal.SourceCheckpointIDs[0] != 7 || proposal.SourceEvidenceIDs[0] != 8 || proposal.SourceReviewIDs[0] != 9 {
		t.Fatalf("proposal sources = %+v", proposal)
	}
	rendered := strings.Join([]string{proposal.Title, proposal.Purpose, proposal.CurrentObjective, strings.Join(proposal.Decisions, " ")}, " ")
	if strings.Contains(rendered, "unsupported sentinel") || strings.Contains(rendered, "raw command") {
		t.Fatalf("unsupported/raw content retained: %s", rendered)
	}
}

func TestLoadRejectsUnsafePathsAndSensitiveContent(t *testing.T) {
	root := t.TempDir()
	outside := writeMemoryFile(t, t.TempDir(), "tmp-ux/outside-memory.md", "# Outside Memory\n")
	valid := writeMemoryFile(t, root, "tmp-ux/valid-memory.md", "# Valid Memory\n## Current Objective\nProceed safely.\n")
	linked := filepath.Join(root, "tmp-ux", "linked-memory.md")
	if err := os.Symlink(valid, linked); err != nil {
		t.Fatal(err)
	}
	linkedDir := filepath.Join(root, "tmp-ux", "linked")
	if err := os.Symlink(filepath.Dir(valid), linkedDir); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
	}{
		{"outside", outside},
		{"traversal", "../outside-memory.md"},
		{"symlink file", linked},
		{"symlink ancestor", filepath.Join(linkedDir, "valid-memory.md")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(root, tc.path); err == nil {
				t.Fatalf("Load(%q) succeeded", tc.path)
			}
		})
	}

	secrets := []string{
		"# Secret Memory\npassword=supersecret\n",
		"# Secret Memory\nAuthorization: Bearer abcdefghijklmnopqrstuvwxyz\n",
		"# Secret Memory\n-----BEGIN PRIVATE KEY-----\n",
		"# Secret Memory\naccess_token: eyJabcdefgh.abcdefgh.abcdefgh\n",
	}
	for index, body := range secrets {
		path := writeMemoryFile(t, root, filepath.Join("tmp-ux", "secret-memory-"+string(rune('a'+index))+".md"), body)
		if _, err := Load(root, path); err == nil || strings.Contains(err.Error(), "supersecret") || strings.Contains(err.Error(), "abcdefgh") {
			t.Fatalf("secret case %d error = %v", index, err)
		}
	}

	raw := writeMemoryFile(t, root, "tmp-ux/raw-memory.md", "# Raw Memory\n## Raw Logs\nrequest body\n")
	if _, err := Load(root, raw); err == nil {
		t.Fatal("raw log section accepted")
	}
}

func TestDiscoverSkipsSymlinksAndInventoriesMemoryMarkdown(t *testing.T) {
	root := t.TempDir()
	writeMemoryFile(t, root, "tmp-ux/a-memory.md", "# A Memory\n## Purpose\nA\n")
	writeMemoryFile(t, root, "tmp-ux/not-notes.md", "# Notes\n")
	target := writeMemoryFile(t, root, "tmp-ux/nested/b-memory.markdown", "# B Memory\n## Purpose\nB\n")
	if err := os.Symlink(target, filepath.Join(root, "tmp-ux", "linked-memory.md")); err != nil {
		t.Fatal(err)
	}
	documents, err := Discover(root, "tmp-ux")
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 2 || documents[0].Path != "tmp-ux/a-memory.md" || documents[1].Path != "tmp-ux/nested/b-memory.markdown" {
		t.Fatalf("documents = %+v", documents)
	}
}

func TestAssessCoverageRequiresExactUnambiguousRepresentation(t *testing.T) {
	document := Document{Path: "tmp-ux/program-memory.md", SHA256: strings.Repeat("a", 64), Proposal: Proposal{Title: "Program Memory", CurrentObjective: "Finish migration", Decisions: []string{"Fairway is authoritative"}, SourceEvidenceIDs: []int64{8}}}
	exact := Memory{TrackID: "program", Title: "Program Memory", CurrentObjective: "Finish migration", Decisions: []string{"Fairway is authoritative"}, SourceEvidenceIDs: []int64{8}, Disposition: "active"}
	mismatch := exact
	mismatch.TrackID = "other"
	mismatch.CurrentObjective = "Different work"

	covered := AssessCoverage(document, []Memory{exact, mismatch}, "")
	if covered.Status != "covered" || covered.TrackID != "program" || covered.RepresentedFacts != covered.ExtractedFacts {
		t.Fatalf("covered = %+v", covered)
	}
	ambiguous := exact
	ambiguous.TrackID = "duplicate"
	if got := AssessCoverage(document, []Memory{exact, ambiguous}, ""); got.Status != "ambiguous" {
		t.Fatalf("ambiguous = %+v", got)
	}
	if got := AssessCoverage(document, []Memory{exact}, "other"); got.Status != "uncovered" || got.TrackID != "other" {
		t.Fatalf("restricted mismatch = %+v", got)
	}
}

func TestPlanRetirementIsReadOnlyAndRequiresDurableDisposition(t *testing.T) {
	document := Document{Path: "tmp-ux/program-memory.md", SHA256: strings.Repeat("b", 64), Proposal: Proposal{Title: "Program Memory", CurrentObjective: "Historical reference"}}
	memory := Memory{TrackID: "program", Title: "Program Memory", CurrentObjective: "Historical reference", Disposition: "active", SourceFactCount: 1}
	if plan := PlanRetirement(document, memory, "migrated"); plan.Eligible || !plan.ReadOnly || plan.DeletesSource {
		t.Fatalf("active plan = %+v", plan)
	}
	memory.Disposition = "archived"
	plan := PlanRetirement(document, memory, "migrated")
	if !plan.Eligible || !plan.ReadOnly || plan.DeletesSource || !strings.Contains(plan.SuggestedEvidence, document.SHA256) {
		t.Fatalf("archived plan = %+v", plan)
	}
}

func writeMemoryFile(t *testing.T, root, relative, body string) string {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
