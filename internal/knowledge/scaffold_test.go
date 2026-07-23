package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestScaffoldCreatesTreeWithoutOverwriting(t *testing.T) {
	project := t.TempDir()
	result, err := Scaffold(ScaffoldOptions{ProjectRoot: project})
	if err != nil {
		t.Fatal(err)
	}
	if result.Root != DefaultRoot || len(result.Created) != len(scaffoldFiles) || len(result.Existing) != 0 {
		t.Fatalf("unexpected scaffold result: %+v", result)
	}
	for _, dir := range scaffoldDirectories {
		info, err := os.Stat(filepath.Join(project, DefaultRoot, dir))
		if err != nil || !info.IsDir() {
			t.Fatalf("scaffold directory %s: info=%v err=%v", dir, info, err)
		}
	}
	readme := filepath.Join(project, DefaultRoot, "README.md")
	if err := os.WriteFile(readme, []byte("owned by project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Scaffold(ScaffoldOptions{ProjectRoot: project})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Created) != 0 || len(second.Existing) != len(scaffoldFiles) {
		t.Fatalf("unexpected second scaffold result: %+v", second)
	}
	data, err := os.ReadFile(readme)
	if err != nil || string(data) != "owned by project\n" {
		t.Fatalf("existing file changed: %q err=%v", data, err)
	}
	if !reflect.DeepEqual(result.Created, []string{"README.md", "current-state.md", "index.md", "log.md", "open-questions.md", "sources.yaml"}) {
		t.Fatalf("created files are not deterministic: %v", result.Created)
	}
	manifestData, err := os.ReadFile(filepath.Join(project, DefaultRoot, DefaultSourceManifest))
	if err != nil {
		t.Fatal(err)
	}
	var manifest SourceManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 1 || manifest.Classes["project-file"].Kind != "project_file" ||
		manifest.Classes["fairway-decision"].FairwayKind != "decision" ||
		!manifest.Classes["fairway-evidence"].RequiresStoreValidation {
		t.Fatalf("unexpected scaffold source manifest: %+v", manifest)
	}
	report, err := Status(Options{ProjectRoot: project, Now: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.Severity == SeverityError {
			t.Fatalf("fresh scaffold has error finding: %+v", finding)
		}
	}
}

func TestScaffoldRejectsSymlinkedDirectory(t *testing.T) {
	project := t.TempDir()
	root := filepath.Join(project, DefaultRoot)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "architecture")); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(ScaffoldOptions{ProjectRoot: project}); err == nil {
		t.Fatal("expected symlinked scaffold directory rejection")
	}
}

func TestScaffoldRejectsEscapingRoot(t *testing.T) {
	if _, err := Scaffold(ScaffoldOptions{ProjectRoot: t.TempDir(), KnowledgeRoot: "../outside"}); err == nil {
		t.Fatal("expected escaping knowledge root rejection")
	}
}
