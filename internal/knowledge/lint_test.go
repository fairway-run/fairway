package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestLintValidKnowledgeTree(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Engineering index", "verified", "2026-12-31", revision, []string{"current-state.md"}, "docs/source.md")
	writePage(t, project, "current-state.md", "Current state", "verified", "2026-12-31", revision, nil, "docs/source.md")

	report, err := Lint(Options{ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("unexpected findings: %+v", report.Findings)
	}
	if report.Root != DefaultRoot || report.PageCount != 2 || report.VerifiedCount != 2 || len(report.Pages) != 4 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Pages[1].Path != "current-state.md" || report.Pages[1].LinkCount != 0 {
		t.Fatalf("pages not sorted or counted: %+v", report.Pages)
	}

	status, err := Status(Options{ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22")})
	if err != nil || !reflect.DeepEqual(report, status) {
		t.Fatalf("status differs from lint: err=%v\nlint=%+v\nstatus=%+v", err, report, status)
	}
}

func TestLintReportsMetadataIndexLinkSourceAndRevisionFindings(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Duplicate title", "verified", "2026-01-01", revision, []string{"a.md", "missing.md", "../../../outside.md"}, "docs/missing-source.md")
	writePage(t, project, "a.md", "Duplicate title", "verified", "2026-01-01", revision, nil, "docs/source.md")
	writePage(t, project, "orphan.md", "Orphan", "unknown", "bad-date", "not-a-sha", nil, "docs/source.md")
	if err := os.WriteFile(filepath.Join(project, "docs/source.md"), []byte("# Changed source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	orphanPath := filepath.Join(project, DefaultRoot, "orphan.md")
	orphanData, err := os.ReadFile(orphanPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanPath, []byte(strings.Replace(string(orphanData), "last_verified: 2026-07-20", "last_verified: invalid", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Lint(Options{ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"duplicate_identity", "last_verified_invalid", "link_broken", "link_path_invalid",
		"metadata_status_invalid", "page_orphaned", "review_by_invalid", "review_overdue",
		"source_path_invalid", "source_revision_invalid", "source_revision_stale",
	}
	for _, code := range want {
		if !hasFinding(report, code) {
			t.Errorf("missing finding %s: %+v", code, report.Findings)
		}
	}
	if !sort.SliceIsSorted(report.Findings, func(i, j int) bool {
		a, b := report.Findings[i], report.Findings[j]
		return a.Path < b.Path || (a.Path == b.Path && a.Code <= b.Code)
	}) {
		t.Fatalf("findings are not deterministic: %+v", report.Findings)
	}
}

func TestLintSecretFindingDoesNotEchoContent(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, []string{"secret.md"}, "docs/source.md")
	secret := "SHOULD_NOT_RENDER_12345"
	body := pageBody("Sensitive "+secret, "verified", "2026-12-31", revision, nil, "docs/source.md") + "\naccess_token=" + secret + "\n"
	writeKnowledgeFile(t, project, "secret.md", body)

	report, err := Lint(Options{ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "secret_pattern") {
		t.Fatalf("missing secret finding: %+v", report.Findings)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("report echoed secret-bearing content: %s", encoded)
	}
}

func TestLintRejectsUnknownMetadataField(t *testing.T) {
	project := newKnowledgeProject(t)
	body := pageBody("Index", "verified", "2026-12-31", gitRevision(t, project), nil, "docs/source.md")
	body = strings.Replace(body, "supersedes: []", "unexpected_field: value\nsupersedes: []", 1)
	writeKnowledgeFile(t, project, "index.md", body)

	report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "metadata_invalid") {
		t.Fatalf("missing strict metadata finding: %+v", report.Findings)
	}
}

func TestLintPathCustody(t *testing.T) {
	t.Run("symlinked page", func(t *testing.T) {
		project := newKnowledgeProject(t)
		outside := filepath.Join(t.TempDir(), "outside.md")
		if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(project, DefaultRoot, "escape.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := Lint(Options{ProjectRoot: project}); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink rejection, got %v", err)
		}
	})

	t.Run("symlinked source", func(t *testing.T) {
		project := newKnowledgeProject(t)
		outside := filepath.Join(t.TempDir(), "outside.md")
		if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(project, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(project, "docs", "linked-source.md")); err != nil {
			t.Fatal(err)
		}
		writePage(t, project, "index.md", "Index", "verified", "2026-12-31", gitRevision(t, project), nil, "docs/linked-source.md")
		report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
		if err != nil {
			t.Fatal(err)
		}
		if !hasFinding(report, "source_path_invalid") {
			t.Fatalf("missing source custody finding: %+v", report.Findings)
		}
	})
}

func TestLintEnforcesBounds(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, []string{"a.md", "missing.md"}, "docs/source.md")
	writePage(t, project, "a.md", "A", "verified", "2026-12-31", revision, nil, "docs/source.md")
	if _, err := Lint(Options{ProjectRoot: project, MaxPages: 1}); err == nil || !strings.Contains(err.Error(), "page count") {
		t.Fatalf("expected page count bound, got %v", err)
	}
	if _, err := Lint(Options{ProjectRoot: project, MaxLinks: 1}); err == nil || !strings.Contains(err.Error(), "link count") {
		t.Fatalf("expected link count bound, got %v", err)
	}
	report, err := Lint(Options{ProjectRoot: project, MaxPageBytes: 32, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "page_too_large") {
		t.Fatalf("missing page size finding: %+v", report.Findings)
	}
}

func TestLintRejectsInvalidRequestedRevision(t *testing.T) {
	project := newKnowledgeProject(t)
	if _, err := Lint(Options{ProjectRoot: project, SourceRevision: "HEAD"}); err == nil || !strings.Contains(err.Error(), "source revision") {
		t.Fatalf("expected invalid requested revision error, got %v", err)
	}
}

func TestLintRequiresConfiguredSourceManifest(t *testing.T) {
	project := newKnowledgeProject(t)
	if err := os.Remove(filepath.Join(project, DefaultRoot, DefaultSourceManifest)); err != nil {
		t.Fatal(err)
	}
	writePage(t, project, "index.md", "Index", "draft", "2026-12-31", gitRevision(t, project), nil, "docs/source.md")
	report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "source_manifest_missing") || !hasFinding(report, "source_class_unknown") {
		t.Fatalf("missing manifest findings: %+v", report.Findings)
	}
}

func TestLintExposesTypedFairwayReferencesForStoreValidation(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writeKnowledgeFile(t, project, "index.md", fairwayPageBody("Index", revision, "fairway-decision", "decision", "DEC-123"))

	report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "fairway_source_validation_required") || !hasFinding(report, "verified_provenance_missing") {
		t.Fatalf("unvalidated Fairway source counted as provenance: %+v", report.Findings)
	}
	want := []FairwayReferenceRequirement{{
		PagePath: "index.md", SourceClass: "fairway-decision",
		Reference: FairwayReference{Kind: "decision", ID: "DEC-123"},
	}}
	if !reflect.DeepEqual(report.FairwayReferences, want) {
		t.Fatalf("unexpected Fairway validation requirements: %+v", report.FairwayReferences)
	}

	validated, err := Lint(Options{
		ProjectRoot: project, Now: mustDate(t, "2026-07-22"),
		ValidateFairwayReference: func(requirement FairwayReferenceRequirement) error {
			if requirement.PagePath != "index.md" || requirement.SourceClass != "fairway-decision" ||
				requirement.Reference.Kind != "decision" || requirement.Reference.ID != "DEC-123" {
				return fmt.Errorf("unexpected reference")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasFinding(validated, "fairway_source_validation_required") || hasFinding(validated, "verified_provenance_missing") {
		t.Fatalf("validated Fairway source rejected: %+v", validated.Findings)
	}
}

func TestLintRejectsMalformedOrMismatchedFairwayReferences(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writeKnowledgeFile(t, project, "index.md", fairwayPageBody("Index", revision, "fairway-decision", "evidence", "bad id with spaces"))
	report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "fairway_source_invalid") || len(report.FairwayReferences) != 0 {
		t.Fatalf("malformed Fairway source was exposed as valid: %+v", report)
	}
}

func TestLintLegacyFreeFormFairwaySourceCannotSatisfyProvenance(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	body := fmt.Sprintf("---\nknowledge_version: 1\ntitle: Index\nstatus: verified\nowner: platform\nlast_verified: 2026-07-20\nreview_by: 2026-12-31\nsource_sha: %s\nsources:\n  - fairway_decision: arbitrary-value\nsupersedes: []\n---\n\n# Index\n", revision)
	writeKnowledgeFile(t, project, "index.md", body)
	report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "metadata_invalid") || !hasFinding(report, "verified_provenance_missing") || len(report.FairwayReferences) != 0 {
		t.Fatalf("legacy free-form Fairway source counted as provenance: %+v", report)
	}
}

func TestLintSourceRevisionTracksCitedFileNotRepositoryHead(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, nil, "docs/source.md")

	if err := os.WriteFile(filepath.Join(project, "unrelated.md"), []byte("unrelated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", "unrelated.md")
	gitRun(t, project, "commit", "-m", "unrelated")
	report, err := Lint(Options{ProjectRoot: project, SourceRevision: gitRevision(t, project), Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if hasFinding(report, "source_revision_stale") || hasFinding(report, "source_revision_unverifiable") {
		t.Fatalf("unrelated commit made cited source stale: %+v", report.Findings)
	}

	if err := os.WriteFile(filepath.Join(project, "docs/source.md"), []byte("# Dirty source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err = Lint(Options{ProjectRoot: project, SourceRevision: gitRevision(t, project), Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "source_revision_stale") {
		t.Fatalf("dirty cited source did not become stale: %+v", report.Findings)
	}
}

func TestLintRejectsCanonicalClassWhenSourceFrontmatterDeniesAuthority(t *testing.T) {
	project := newKnowledgeProject(t)
	setProjectFileAuthority(t, project, "canonical")
	source := `---
source_of_truth: false
implementation_state: not-assessed
---
# Operational target model
`
	if err := os.WriteFile(filepath.Join(project, "docs", "source.md"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", "docs/source.md")
	gitRun(t, project, "commit", "-m", "operational source metadata")
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, nil, "docs/source.md")

	report, err := Lint(Options{ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "source_authority_conflict") || !hasFinding(report, "verified_provenance_missing") {
		t.Fatalf("canonical authority contradiction was not rejected: %+v", report.Findings)
	}
}

func TestLintWarnsWhenCanonicalSourceDoesNotAssessImplementation(t *testing.T) {
	project := newKnowledgeProject(t)
	setProjectFileAuthority(t, project, "canonical")
	source := `---
source_of_truth: true
implementation_state: not-assessed
---
# Canonical target decision
`
	if err := os.WriteFile(filepath.Join(project, "docs", "source.md"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", "docs/source.md")
	gitRun(t, project, "commit", "-m", "canonical source metadata")
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, nil, "docs/source.md")

	report, err := Lint(Options{ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "source_implementation_not_assessed") || hasFinding(report, "verified_provenance_missing") {
		t.Fatalf("implementation-state warning is incorrect: %+v", report.Findings)
	}
}

func TestLintParsesCanonicalAuthorityFromVerifiedSourceSnapshot(t *testing.T) {
	project := newKnowledgeProject(t)
	setProjectFileAuthority(t, project, "canonical")
	original := `---
source_of_truth: true
implementation_state: current
---
# Canonical source
`
	replacement := strings.Replace(original, "source_of_truth: true", "source_of_truth: false", 1)
	sourcePath := filepath.Join(project, "docs", "source.md")
	if err := os.WriteFile(sourcePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, project, "add", "docs/source.md")
	gitRun(t, project, "commit", "-m", "canonical source")
	revision := gitRevision(t, project)
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, nil, "docs/source.md")
	swapped := false

	report, err := Lint(Options{
		ProjectRoot: project, SourceRevision: revision, Now: mustDate(t, "2026-07-22"),
		CustodyHook: func(stage string) {
			if stage == "lint_source_before_authority" && !swapped {
				swapped = true
				if err := os.WriteFile(sourcePath, []byte(replacement), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !swapped {
		t.Fatal("source replacement hook did not run")
	}
	if hasFinding(report, "source_authority_conflict") || hasFinding(report, "verified_provenance_missing") {
		t.Fatalf("authority was parsed from a replacement path instead of the verified snapshot: %+v", report.Findings)
	}
}

func TestLintRejectsProjectFileOutsideConfiguredRoots(t *testing.T) {
	project := newKnowledgeProject(t)
	revision := gitRevision(t, project)
	if err := os.MkdirAll(filepath.Join(project, "tmp-ux"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "tmp-ux", "memory.md"), []byte("# Legacy memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", revision, nil, "tmp-ux/memory.md")
	report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report, "source_root_invalid") || !hasFinding(report, "verified_provenance_missing") {
		t.Fatalf("outside-root source satisfied provenance: %+v", report.Findings)
	}
}

func TestLintRejectsCanonicalManifestRootedInLegacyMemory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		root   string
		reject bool
	}{
		{name: "exact", root: "tmp-ux", reject: true},
		{name: "descendant", root: "tmp-ux/private", reject: true},
		{name: "normalized descendant", root: "./tmp-ux/private", reject: true},
		{name: "prefix boundary", root: "tmp-ux2", reject: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := newKnowledgeProject(t)
			manifest := fmt.Sprintf(`knowledge_sources_version: 1
classes:
  legacy:
    kind: project_file
    authority: canonical
    roots: [%s]
`, tc.root)
			if err := os.WriteFile(filepath.Join(project, DefaultRoot, DefaultSourceManifest), []byte(manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			writePage(t, project, "index.md", "Index", "draft", "2026-12-31", gitRevision(t, project), nil, "docs/source.md")
			report, err := Lint(Options{ProjectRoot: project, Now: mustDate(t, "2026-07-22")})
			if err != nil {
				t.Fatal(err)
			}
			if got := hasFinding(report, "source_class_invalid"); got != tc.reject {
				t.Fatalf("source_class_invalid=%t want %t for root %q: %+v", got, tc.reject, tc.root, report.Findings)
			}
		})
	}
}

func newKnowledgeProject(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, DefaultRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "docs/source.md"), []byte("# Canonical source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, DefaultRoot, DefaultSourceManifest), []byte(scaffoldFiles[DefaultSourceManifest]), 0o644); err != nil {
		t.Fatal(err)
	}
	writeKnowledgeFile(t, project, "README.md", "# Knowledge\n")
	writeKnowledgeFile(t, project, "log.md", "# Log\n")
	gitRun(t, project, "init")
	gitRun(t, project, "config", "user.email", "knowledge-test@example.invalid")
	gitRun(t, project, "config", "user.name", "Knowledge Test")
	gitRun(t, project, "add", "docs/source.md")
	gitRun(t, project, "commit", "-m", "source")
	return project
}

func setProjectFileAuthority(t *testing.T, project, authority string) {
	t.Helper()
	path := filepath.Join(project, DefaultRoot, DefaultSourceManifest)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "authority: operational", "authority: "+authority, 1)
	if updated == string(data) {
		t.Fatal("project-file authority was not updated")
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePage(t *testing.T, project, rel, title, status, reviewBy, revision string, links []string, source string) {
	t.Helper()
	writeKnowledgeFile(t, project, rel, pageBody(title, status, reviewBy, revision, links, source))
}

func pageBody(title, status, reviewBy, revision string, links []string, source string) string {
	var body strings.Builder
	fmt.Fprintf(&body, "---\nknowledge_version: 1\ntitle: %s\nstatus: %s\nowner: platform\nlast_verified: 2026-07-20\nreview_by: %s\nsource_sha: %s\nsources:\n  - class: project-file\n    path: %s\nsupersedes: []\n---\n\n# %s\n", title, status, reviewBy, revision, source, title)
	for _, link := range links {
		fmt.Fprintf(&body, "- [Page](%s)\n", link)
	}
	return body.String()
}

func fairwayPageBody(title, revision, class, kind, id string) string {
	return fmt.Sprintf("---\nknowledge_version: 1\ntitle: %s\nstatus: verified\nowner: platform\nlast_verified: 2026-07-20\nreview_by: 2026-12-31\nsource_sha: %s\nsources:\n  - class: %s\n    fairway:\n      kind: %s\n      id: %s\nsupersedes: []\n---\n\n# %s\n", title, revision, class, kind, id, title)
}

func writeKnowledgeFile(t *testing.T, project, rel, body string) {
	t.Helper()
	path := filepath.Join(project, DefaultRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasFinding(report Report, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func gitRevision(t *testing.T, project string) string {
	t.Helper()
	return strings.TrimSpace(gitRun(t, project, "rev-parse", "HEAD"))
}

func gitRun(t *testing.T, project string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", project}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}
