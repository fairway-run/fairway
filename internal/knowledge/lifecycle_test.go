package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIngestIsPreviewFirstAndDoesNotCopySourceBody(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "draft", "2026-12-31", revision, nil, "docs/source.md")
	sourcePath := filepath.Join(project, "docs", "source.md")
	if err := os.WriteFile(sourcePath, []byte("# Node trust\n\nRAW_SOURCE_SENTINEL must not be copied.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", "docs/source.md")
	gitRun(t, project, "commit", "-m", "update source")
	revision = gitRevision(t, project)

	options := IngestOptions{
		Options:    Options{ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22")},
		SourcePath: "docs/source.md", PagePath: "architecture/node-trust.md",
		Title: "Node trust", Owner: "security", ReviewBy: "2026-10-22",
	}
	preview, err := Ingest(options)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || !preview.Preview || len(preview.Changes) != 2 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(project, DefaultRoot, "architecture", "node-trust.md")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote page: %v", err)
	}

	options.Apply = true
	applied, err := Ingest(options)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Preview {
		t.Fatalf("unexpected apply result: %+v", applied)
	}
	data, err := os.ReadFile(filepath.Join(project, DefaultRoot, "architecture", "node-trust.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "RAW_SOURCE_SENTINEL") {
		t.Fatal("ingest copied source body")
	}
	for _, expected := range []string{"status: draft", "path: docs/source.md", "Verify conclusions against the cited source"} {
		if !strings.Contains(string(data), expected) {
			t.Fatalf("page missing %q:\n%s", expected, data)
		}
	}
	index, err := os.ReadFile(filepath.Join(project, DefaultRoot, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "(architecture/node-trust.md)") {
		t.Fatalf("index missing applied link:\n%s", index)
	}
	if _, err := Ingest(options); err == nil {
		t.Fatal("second ingest overwrote existing page")
	}
}

func TestQuerySelectsRelevantPagesAndDeduplicatesProvenance(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, []string{"architecture/node-trust.md", "operations/billing.md"}, "docs/source.md")
	writePage(t, project, "architecture/node-trust.md", "Node trust model", "verified", "2026-12-31", revision, nil, "docs/source.md")
	writePage(t, project, "operations/billing.md", "Billing operations", "draft", "2026-12-31", revision, nil, "docs/source.md")

	packet, err := Query(QueryOptions{
		Options: Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")},
		Topic:   "node trust identity", TaskID: "SEC-101",
		TaskTerms: []string{"node certificate trust"}, BudgetBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Pages) != 1 || packet.Pages[0].Path != "architecture/node-trust.md" || !packet.Pages[0].Verified {
		t.Fatalf("unexpected selected pages: %+v", packet.Pages)
	}
	if len(packet.Sources) != 1 || packet.Sources[0].Key != "file:docs/source.md" {
		t.Fatalf("provenance was not deduplicated: %+v", packet.Sources)
	}
	if packet.Bytes > packet.BudgetBytes || !packet.Bounded || !packet.ReadOnly {
		t.Fatalf("packet bound invalid: %+v", packet)
	}
}

func TestQueryLabelsConflictedAndStalePagesWithoutTreatingThemVerified(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, []string{"conflict.md", "stale.md"}, "docs/source.md")
	writePage(t, project, "conflict.md", "Node conflict", "conflicted", "2026-12-31", revision, nil, "docs/source.md")
	writePage(t, project, "stale.md", "Node stale", "stale", "2026-12-31", revision, nil, "docs/source.md")

	packet, err := Query(QueryOptions{Options: Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")}, Topic: "node", BudgetBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Pages) != 2 {
		t.Fatalf("pages=%+v", packet.Pages)
	}
	for _, page := range packet.Pages {
		if page.Verified {
			t.Fatalf("unsafe page labeled verified: %+v", page)
		}
		if page.Status == "conflicted" && !page.Conflict {
			t.Fatalf("conflict label missing: %+v", page)
		}
		if page.Status == "stale" && !page.Stale {
			t.Fatalf("stale label missing: %+v", page)
		}
	}
}

func TestPromoteRequiresVerifiedPageAndReviewedCommittedCanonicalTarget(t *testing.T) {
	project := newKnowledgeProject(t)
	if err := os.WriteFile(filepath.Join(project, "docs", "canonical.md"), []byte("# Canonical node trust\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", "docs/canonical.md")
	gitRun(t, project, "commit", "-m", "canonical target")
	reviewedCommit := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", reviewedCommit, []string{"architecture/node-trust.md"}, "docs/source.md")
	writePage(t, project, "architecture/node-trust.md", "Node trust", "verified", "2026-12-31", reviewedCommit, nil, "docs/source.md")

	options := PromoteOptions{
		Options:  Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")},
		PagePath: "architecture/node-trust.md", TargetPath: "docs/canonical.md",
		ReviewedCommit: reviewedCommit,
	}
	preview, err := Promote(options)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || !preview.Preview {
		t.Fatalf("unexpected promotion preview: %+v", preview)
	}
	before, _ := os.ReadFile(filepath.Join(project, DefaultRoot, options.PagePath))
	if strings.Contains(string(before), "promotion_target") {
		t.Fatal("preview changed page")
	}

	if err := os.WriteFile(filepath.Join(project, "docs", "canonical.md"), []byte("# Dirty target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(options); err == nil {
		t.Fatal("promotion accepted canonical target that differs from reviewed commit")
	}
	gitRun(t, project, "checkout", "--", "docs/canonical.md")

	options.Apply = true
	applied, err := Promote(options)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied {
		t.Fatalf("promotion was not applied: %+v", applied)
	}
	after, _ := os.ReadFile(filepath.Join(project, DefaultRoot, options.PagePath))
	for _, expected := range []string{"status: superseded", "promotion_target: docs/canonical.md", "promotion_commit: " + reviewedCommit} {
		if !strings.Contains(string(after), expected) {
			t.Fatalf("promoted page missing %q:\n%s", expected, after)
		}
	}
	report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.Path == options.PagePath && strings.HasPrefix(finding.Code, "promotion_") {
			t.Fatalf("applied promotion failed lint: %+v", report.Findings)
		}
	}
}

func TestPromoteRejectsDraftConflictUnsafeTargetAndUnverifiedCitation(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "draft", "2026-12-31", revision, []string{"draft.md"}, "docs/source.md")
	writePage(t, project, "draft.md", "Draft", "draft", "2026-12-31", revision, nil, "docs/source.md")
	for _, target := range []string{"../outside.md", "tmp-ux/memory.md", "doc/agent-wiki/index.md"} {
		_, err := Promote(PromoteOptions{
			Options:  Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")},
			PagePath: "draft.md", TargetPath: target, ReviewedCommit: revision,
		})
		if err == nil {
			t.Fatalf("promotion accepted draft/unsafe target %q", target)
		}
	}
}

func TestQueryRejectsSecretLikeAndOversizedInputs(t *testing.T) {
	project := newKnowledgeProject(t)
	for _, topic := range []string{"password=DO_NOT_RENDER_VALUE", strings.Repeat("x", 513)} {
		if _, err := Query(QueryOptions{Options: Options{ProjectRoot: project}, Topic: topic}); err == nil {
			t.Fatalf("query accepted unsafe topic %q", topic[:min(len(topic), 32)])
		}
	}
}
