package knowledge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryAndPromoteNeverLeakExpandedSecretClasses(t *testing.T) {
	secrets := []string{
		"secret=QUERY_SECRET_SENTINEL",
		"ssh_private_key=SSH_PRIVATE_SENTINEL",
		"AWS_SECRET_ACCESS_KEY=AWS_SECRET_SENTINEL",
		"set-cookie: session=COOKIE_SECRET_SENTINEL",
	}
	for index, secret := range secrets {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			project := newKnowledgeProject(t)
			revision := gitRevision(t, project)
			writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, []string{"unsafe.md"}, "docs/source.md")
			writePage(t, project, "unsafe.md", "Node trust", "verified", "2026-12-31", revision, nil, "docs/source.md")
			path := filepath.Join(project, DefaultRoot, "unsafe.md")
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString("\n" + secret + "\n"); err != nil {
				t.Fatal(err)
			}
			_ = file.Close()

			packet, err := Query(QueryOptions{Options: Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")}, Topic: "node trust", BudgetBytes: 4096})
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := json.Marshal(packet)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(rendered), strings.Split(secret, "=")[len(strings.Split(secret, "="))-1]) ||
				strings.Contains(string(rendered), "COOKIE_SECRET_SENTINEL") {
				t.Fatalf("query leaked secret: %s", rendered)
			}

			result, promoteErr := Promote(PromoteOptions{
				Options:  Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")},
				PagePath: "unsafe.md", TargetPath: "docs/source.md", ReviewedCommit: revision,
			})
			promotionOutput, _ := json.Marshal(result)
			if promoteErr == nil || strings.Contains(promoteErr.Error(), "SENTINEL") || strings.Contains(string(promotionOutput), "SENTINEL") {
				t.Fatalf("promotion leak/result=%s err=%v", promotionOutput, promoteErr)
			}
		})
	}
}

func TestQueryAndPromoteFailClosedOnGlobalSourceClassErrors(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, []string{"node.md"}, "docs/source.md")
	writePage(t, project, "node.md", "Node trust", "verified", "2026-12-31", revision, nil, "docs/source.md")
	invalidManifest := `knowledge_sources_version: 1
classes:
  project-file:
    kind: project_file
    authority: canonical
    roots: []
`
	if err := os.WriteFile(filepath.Join(project, DefaultRoot, DefaultSourceManifest), []byte(invalidManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Query(QueryOptions{Options: Options{ProjectRoot: project}, Topic: "node", BudgetBytes: 4096}); err == nil {
		t.Fatal("query accepted invalid global source class")
	}
	if _, err := Promote(PromoteOptions{
		Options: Options{ProjectRoot: project}, PagePath: "node.md",
		TargetPath: "docs/source.md", ReviewedCommit: revision,
	}); err == nil {
		t.Fatal("promotion accepted invalid global source class")
	}
}

func TestQueryExcludesRelevantButOrphanedPage(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, []string{"reachable.md"}, "docs/source.md")
	writePage(t, project, "reachable.md", "Billing operations", "verified", "2026-12-31", revision, nil, "docs/source.md")
	writePage(t, project, "orphan.md", "Node trust orphan", "verified", "2026-12-31", revision, nil, "docs/source.md")
	packet, err := Query(QueryOptions{Options: Options{ProjectRoot: project}, Topic: "node trust", BudgetBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Pages) != 0 {
		t.Fatalf("query selected orphaned page: %+v", packet.Pages)
	}
}

func TestQueryDedupePreservesHighestAuthorityAndPerPageVerification(t *testing.T) {
	project := newKnowledgeProject(t)
	manifest := `knowledge_sources_version: 1
classes:
  canonical-file:
    kind: project_file
    authority: canonical
    roots: [docs]
  operational-file:
    kind: project_file
    authority: operational
    roots: [docs]
`
	if err := os.WriteFile(filepath.Join(project, DefaultRoot, DefaultSourceManifest), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	revision := gitRevision(t, project)
	index := strings.Replace(pageBody("Index", "verified", "2026-12-31", revision, []string{"verified.md", "draft.md"}, "docs/source.md"), "class: project-file", "class: canonical-file", 1)
	verified := strings.Replace(pageBody("Node trust verified", "verified", "2026-12-31", revision, nil, "docs/source.md"), "class: project-file", "class: canonical-file", 1)
	draft := strings.Replace(pageBody("Node trust draft", "draft", "2026-12-31", revision, nil, "docs/source.md"), "class: project-file", "class: operational-file", 1)
	writeKnowledgeFile(t, project, "index.md", index)
	writeKnowledgeFile(t, project, "verified.md", verified)
	writeKnowledgeFile(t, project, "draft.md", draft)

	packet, err := Query(QueryOptions{Options: Options{ProjectRoot: project}, Topic: "node trust", BudgetBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Pages) != 2 || len(packet.Sources) != 1 {
		t.Fatalf("unexpected packet: %+v", packet)
	}
	source := packet.Sources[0]
	if source.Authority != "canonical" || source.Class != "canonical-file" || !source.Verified || len(source.Citations) != 2 {
		t.Fatalf("dedupe lost authority or citations: %+v", source)
	}
	states := map[string]bool{}
	for _, citation := range source.Citations {
		states[citation.Page] = citation.Verified
	}
	if !states["verified.md"] || states["draft.md"] {
		t.Fatalf("per-page verification lost: %+v", source.Citations)
	}
}

func TestQueryPacketByteAccountingIsExactFixedPoint(t *testing.T) {
	packet := QueryPacket{
		Schema: "fairway.knowledge-query.v1", Topic: strings.Repeat("context ", 70),
		Pages:       []QueryPage{{Path: "architecture/node.md", Title: "Node", Excerpt: strings.Repeat("x", 750)}},
		Sources:     []QuerySource{{Key: "file:docs/source.md", Citations: []QuerySourceCitation{{Page: "architecture/node.md", Verified: true}}}},
		BudgetBytes: 8192, Bounded: true, ReadOnly: true,
	}
	if err := FinalizeQueryPacket(&packet); err != nil {
		t.Fatal(err)
	}
	rendered, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Bytes != len(rendered) {
		t.Fatalf("bytes=%d rendered=%d", packet.Bytes, len(rendered))
	}
}

func TestIngestReadsOpenedSourceWhenVisibleFileIsReplaced(t *testing.T) {
	project, options := ingestSwapProject(t)
	sourcePath := filepath.Join(project, "docs", "source.md")
	options.CustodyHook = func(stage string) {
		if stage != "ingest_source_after_open" {
			return
		}
		if err := os.Rename(sourcePath, sourcePath+".original"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sourcePath, []byte("AWS_SECRET_ACCESS_KEY=REPLACEMENT\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Ingest(options)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Preview || result.SourceRevision != options.SourceRevision {
		t.Fatalf("unexpected ingest preview: %+v", result)
	}
	rendered, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(rendered, []byte("REPLACEMENT")) {
		t.Fatalf("replacement source leaked into preview: %s", rendered)
	}
}

func TestIngestDescriptorBindingSurvivesParentSwap(t *testing.T) {
	project, options := ingestSwapProject(t)
	knowledgeRoot := filepath.Join(project, DefaultRoot)
	originalRoot := filepath.Join(project, "doc", "agent-wiki-original")
	options.Apply = true
	options.CustodyHook = func(stage string) {
		if stage != "ingest_after_bind" {
			return
		}
		if err := os.Rename(knowledgeRoot, originalRoot); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(knowledgeRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(knowledgeRoot, "index.md"), []byte("DECOY_INDEX\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Ingest(options); err != nil {
		t.Fatal(err)
	}
	decoy, _ := os.ReadFile(filepath.Join(knowledgeRoot, "index.md"))
	if string(decoy) != "DECOY_INDEX\n" {
		t.Fatalf("swapped visible directory was modified: %s", decoy)
	}
	if _, err := os.Stat(filepath.Join(originalRoot, "architecture", "node-trust.md")); err != nil {
		t.Fatalf("bound original directory did not receive page: %v", err)
	}
}

func TestIngestFailsClosedOnIndexFileSwapAndCleansPage(t *testing.T) {
	project, options := ingestSwapProject(t)
	options.Apply = true
	indexPath := filepath.Join(project, DefaultRoot, "index.md")
	options.CustodyHook = func(stage string) {
		if stage != "ingest_before_index_replace" {
			return
		}
		if err := os.Rename(indexPath, indexPath+".old"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(indexPath, []byte("INDEX_REPLACEMENT\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Ingest(options); err == nil {
		t.Fatal("ingest accepted index replacement")
	}
	if _, err := os.Stat(filepath.Join(project, DefaultRoot, "architecture", "node-trust.md")); !os.IsNotExist(err) {
		t.Fatalf("failed ingest retained created page: %v", err)
	}
	replacement, _ := os.ReadFile(indexPath)
	if string(replacement) != "INDEX_REPLACEMENT\n" {
		t.Fatalf("replacement index was overwritten: %s", replacement)
	}
}

func TestPromoteDescriptorBindingSurvivesParentSwap(t *testing.T) {
	project, options := promotionSwapProject(t)
	architecture := filepath.Join(project, DefaultRoot, "architecture")
	original := filepath.Join(project, DefaultRoot, "architecture-original")
	options.Apply = true
	options.CustodyHook = func(stage string) {
		if stage != "promote_before_replace" {
			return
		}
		if err := os.Rename(architecture, original); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(architecture, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(architecture, "node.md"), []byte("DECOY_PAGE\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Promote(options); err != nil {
		t.Fatal(err)
	}
	decoy, _ := os.ReadFile(filepath.Join(architecture, "node.md"))
	if string(decoy) != "DECOY_PAGE\n" {
		t.Fatalf("swapped visible directory was modified: %s", decoy)
	}
	promoted, _ := os.ReadFile(filepath.Join(original, "node.md"))
	if !strings.Contains(string(promoted), "promotion_target: docs/canonical.md") {
		t.Fatalf("bound original page was not promoted: %s", promoted)
	}
}

func TestPromoteFailsClosedOnFinalFileSwap(t *testing.T) {
	project, options := promotionSwapProject(t)
	options.Apply = true
	pagePath := filepath.Join(project, DefaultRoot, "architecture", "node.md")
	options.CustodyHook = func(stage string) {
		if stage != "promote_before_replace" {
			return
		}
		if err := os.Rename(pagePath, pagePath+".old"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pagePath, []byte("PAGE_REPLACEMENT\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Promote(options); err == nil {
		t.Fatal("promotion accepted final-file replacement")
	}
	replacement, _ := os.ReadFile(pagePath)
	if string(replacement) != "PAGE_REPLACEMENT\n" {
		t.Fatalf("replacement page was overwritten: %s", replacement)
	}
}

func ingestSwapProject(t *testing.T) (string, IngestOptions) {
	t.Helper()
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "draft", "2026-12-31", revision, nil, "docs/source.md")
	return project, IngestOptions{
		Options:    Options{ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22")},
		SourcePath: "docs/source.md", PagePath: "architecture/node-trust.md",
		Owner: "security", ReviewBy: "2026-12-31",
	}
}

func promotionSwapProject(t *testing.T) (string, PromoteOptions) {
	t.Helper()
	project := newKnowledgeProject(t)
	if err := os.WriteFile(filepath.Join(project, "docs", "canonical.md"), []byte("# Canonical\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", "docs/canonical.md")
	gitRun(t, project, "commit", "-m", "canonical")
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, []string{"architecture/node.md"}, "docs/source.md")
	writePage(t, project, "architecture/node.md", "Node trust", "verified", "2026-12-31", revision, nil, "docs/source.md")
	return project, PromoteOptions{
		Options:  Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")},
		PagePath: "architecture/node.md", TargetPath: "docs/canonical.md", ReviewedCommit: revision,
	}
}
