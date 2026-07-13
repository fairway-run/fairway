package provenance

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleaseAssuranceBundleExportVerifyAndTamper(t *testing.T) {
	root := t.TempDir()
	artifact := filepath.Join(root, "fairway_linux_arm64.tar.gz")
	if err := os.WriteFile(artifact, []byte("candidate artifact\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	evidence := writeReleaseFixtureEvidence(t, root, "v1.2.3", strings.Repeat("a", 40), "github:fairway-run/release@macos-15")
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "bundle")
	manifest, err := ExportReleaseBundle(ReleaseBundleOptions{OutputDirectory: out, Version: "v1.2.3", SourceSHA: strings.Repeat("a", 40), BuilderID: "github:fairway-run/release@macos-15", PolicyVersion: "sovereign-release-v1", CreatedAt: time.Now().UTC(), SigningKeyBase64: base64.StdEncoding.EncodeToString(private), Artifacts: map[string]string{filepath.Base(artifact): artifact}, Evidence: evidence, SLSA: ReleaseSLSAProperties{SourceVersioned: true, BuildServiceGenerated: true, ProvenanceAvailable: true, BuilderIdentityRecorded: true, BuildRecipeRecorded: true, DependenciesRecorded: true}})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SLSA.LevelClaimed || len(manifest.Evidence) != len(RequiredReleaseEvidence) || len(manifest.Artifacts) != 1 {
		t.Fatalf("manifest=%+v", manifest)
	}
	verify := ReleaseBundleVerifyOptions{Directory: out, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(public), ExpectedVersion: "v1.2.3", ExpectedSourceSHA: strings.Repeat("a", 40), ExpectedBuilderID: "github:fairway-run/release@macos-15", ExpectedPolicyVersion: "sovereign-release-v1"}
	report, err := VerifyReleaseBundle(verify)
	if err != nil || !report.OK {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if err := os.WriteFile(filepath.Join(out, "artifacts", filepath.Base(artifact)), []byte("tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, _ = VerifyReleaseBundle(verify)
	if report.OK || len(report.Issues) == 0 {
		t.Fatalf("tampered report=%+v", report)
	}
}

func TestReleaseAssuranceBundleRequiresCompleteEvidenceAndNoLevelClaim(t *testing.T) {
	_, private, _ := ed25519.GenerateKey(nil)
	root := t.TempDir()
	artifact := filepath.Join(root, "fairway.tar.gz")
	_ = os.WriteFile(artifact, []byte("x"), 0o600)
	opts := ReleaseBundleOptions{OutputDirectory: filepath.Join(root, "bundle"), Version: "v1", SourceSHA: strings.Repeat("a", 40), BuilderID: "builder", PolicyVersion: "policy", CreatedAt: time.Now().UTC(), SigningKeyBase64: base64.StdEncoding.EncodeToString(private), Artifacts: map[string]string{"fairway.tar.gz": artifact}, Evidence: map[string]string{}}
	if _, err := ExportReleaseBundle(opts); err == nil || !strings.Contains(err.Error(), "missing required evidence") {
		t.Fatalf("missing evidence error=%v", err)
	}
	for _, class := range RequiredReleaseEvidence {
		path := filepath.Join(root, class+".json")
		_ = os.WriteFile(path, []byte("{}\n"), 0o600)
		opts.Evidence[class] = path
	}
	opts.SLSA.LevelClaimed = true
	if _, err := ExportReleaseBundle(opts); err == nil || !strings.Contains(err.Error(), "must not claim") {
		t.Fatalf("SLSA claim error=%v", err)
	}
}

func TestReleaseAssuranceBundleRejectsPrivateEvidenceAndDuplicateNestedJSON(t *testing.T) {
	_, private, _ := ed25519.GenerateKey(nil)
	root := t.TempDir()
	artifact := filepath.Join(root, "fairway.tar.gz")
	_ = os.WriteFile(artifact, []byte("x"), 0o600)
	evidence := writeReleaseFixtureEvidence(t, root, "v1", strings.Repeat("a", 40), "builder")
	_ = os.WriteFile(evidence["test_summary"], []byte(`{"authorization":"Bearer SHOULD_NOT_COPY"}`+"\n"), 0o600)
	_, err := ExportReleaseBundle(ReleaseBundleOptions{OutputDirectory: filepath.Join(root, "bundle"), Version: "v1", SourceSHA: strings.Repeat("a", 40), BuilderID: "builder", PolicyVersion: "policy", CreatedAt: time.Now().UTC(), SigningKeyBase64: base64.StdEncoding.EncodeToString(private), Artifacts: map[string]string{"fairway.tar.gz": artifact}, Evidence: evidence})
	if err == nil || !strings.Contains(err.Error(), "prohibited private content") {
		t.Fatalf("private evidence error=%v", err)
	}
	var manifest ReleaseBundleManifest
	if err := strictReleaseJSON([]byte(`{"schema":"fairway.release-assurance-manifest.v1","artifacts":[{"name":"a","name":"b"}]}`), &manifest); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate nested JSON error=%v", err)
	}
}

func writeReleaseFixtureEvidence(t *testing.T, root, version, source, builder string) map[string]string {
	t.Helper()
	created := time.Now().UTC().Format(time.RFC3339)
	values := map[string]string{
		"sbom":                      `{"spdxVersion":"SPDX-2.3","SPDXID":"SPDXRef-DOCUMENT","packages":[{}]}`,
		"vex":                       fmt.Sprintf(`{"@context":"https://openvex.dev/ns/v0.2.0","@id":"https://example.test/vex","author":"test","timestamp":%q,"version":1,"statements":[]}`, created),
		"dependencies":              `{"Path":"example.test/module","Version":"v1.0.0"}`,
		"licenses":                  `example.test/module,https://example.test/LICENSE,MIT`,
		"license_disposition":       `{"schema":"fairway.release-license-overrides.v1","overrides":[{}]}`,
		"source_provenance":         fmt.Sprintf(`{"schema":"fairway.release-source-provenance.v1","version":%q,"source_sha":%q,"repository":"test/repo","ref":"refs/tags/test"}`, version, source),
		"build_provenance":          fmt.Sprintf(`{"schema":"fairway.release-build-provenance.v1","builder_id":%q,"run_id":"1","run_attempt":"1","runner_os":"test","runner_arch":"test","go_version":"go test","goreleaser_version":"test","created_at":%q}`, builder, created),
		"build_recipe":              fmt.Sprintf(`{"schema":"fairway.release-build-recipe.v1","source":".goreleaser.yaml","sha256":%q}`, strings.Repeat("b", 64)),
		"test_summary":              fmt.Sprintf(`{"schema":"fairway.release-test-summary.v1","created_at":%q,"commands":["go test ./..."],"result":"pass"}`, created),
		"vulnerability_disposition": `{"schema":"fairway.release-vulnerability-disposition.v1","scanner":"govulncheck","result":"no_findings","report":"govulncheck.json","authority_boundary":"scanner result only"}`,
	}
	out := map[string]string{}
	for _, class := range RequiredReleaseEvidence {
		path := filepath.Join(root, class+".json")
		if err := os.WriteFile(path, []byte(values[class]+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out[class] = path
	}
	return out
}
