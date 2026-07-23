package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const testRevision = "abcdef1234567890"

func TestLintValidKnowledgeTree(t *testing.T) {
	project := newKnowledgeProject(t)
	writePage(t, project, "index.md", "Engineering index", "verified", "2026-12-31", testRevision, []string{"current-state.md"}, "source.md")
	writePage(t, project, "current-state.md", "Current state", "verified", "2026-12-31", testRevision, nil, "source.md")

	report, err := Lint(Options{ProjectRoot: project, SourceRevision: testRevision, Now: mustDate(t, "2026-07-22")})
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

	status, err := Status(Options{ProjectRoot: project, SourceRevision: testRevision, Now: mustDate(t, "2026-07-22")})
	if err != nil || !reflect.DeepEqual(report, status) {
		t.Fatalf("status differs from lint: err=%v\nlint=%+v\nstatus=%+v", err, report, status)
	}
}

func TestLintReportsMetadataIndexLinkSourceAndRevisionFindings(t *testing.T) {
	project := newKnowledgeProject(t)
	writePage(t, project, "index.md", "Duplicate title", "verified", "2026-01-01", "1111111", []string{"a.md", "missing.md", "../../../outside.md"}, "missing-source.md")
	writePage(t, project, "a.md", "Duplicate title", "verified", "2026-01-01", "1111111", nil, "source.md")
	writePage(t, project, "orphan.md", "Orphan", "unknown", "bad-date", "not-a-sha", nil, "source.md")
	orphanPath := filepath.Join(project, DefaultRoot, "orphan.md")
	orphanData, err := os.ReadFile(orphanPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanPath, []byte(strings.Replace(string(orphanData), "last_verified: 2026-07-20", "last_verified: invalid", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Lint(Options{ProjectRoot: project, SourceRevision: testRevision, Now: mustDate(t, "2026-07-22")})
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
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", testRevision, []string{"secret.md"}, "source.md")
	secret := "SHOULD_NOT_RENDER_12345"
	body := pageBody("Sensitive "+secret, "verified", "2026-12-31", testRevision, nil, "source.md") + "\naccess_token=" + secret + "\n"
	writeKnowledgeFile(t, project, "secret.md", body)

	report, err := Lint(Options{ProjectRoot: project, SourceRevision: testRevision, Now: mustDate(t, "2026-07-22")})
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
	body := pageBody("Index", "verified", "2026-12-31", testRevision, nil, "source.md")
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
		if err := os.Symlink(outside, filepath.Join(project, "linked-source.md")); err != nil {
			t.Fatal(err)
		}
		writePage(t, project, "index.md", "Index", "verified", "2026-12-31", testRevision, nil, "linked-source.md")
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
	writePage(t, project, "index.md", "Index", "verified", "2026-12-31", testRevision, []string{"a.md", "missing.md"}, "source.md")
	writePage(t, project, "a.md", "A", "verified", "2026-12-31", testRevision, nil, "source.md")
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

func newKnowledgeProject(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, DefaultRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "source.md"), []byte("# Canonical source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeKnowledgeFile(t, project, "README.md", "# Knowledge\n")
	writeKnowledgeFile(t, project, "log.md", "# Log\n")
	return project
}

func writePage(t *testing.T, project, rel, title, status, reviewBy, revision string, links []string, source string) {
	t.Helper()
	writeKnowledgeFile(t, project, rel, pageBody(title, status, reviewBy, revision, links, source))
}

func pageBody(title, status, reviewBy, revision string, links []string, source string) string {
	var body strings.Builder
	fmt.Fprintf(&body, "---\nknowledge_version: 1\ntitle: %s\nstatus: %s\nowner: platform\nlast_verified: 2026-07-20\nreview_by: %s\nsource_sha: %s\nsources:\n  - path: %s\nsupersedes: []\n---\n\n# %s\n", title, status, reviewBy, revision, source, title)
	for _, link := range links {
		fmt.Fprintf(&body, "- [Page](%s)\n", link)
	}
	return body.String()
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
