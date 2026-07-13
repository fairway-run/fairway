package offlinebundle

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/provenance"
)

func TestExportVerifyAndTamperOfflineBundle(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	privateB64 := base64.StdEncoding.EncodeToString(privateKey)
	publicB64 := base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))
	current := ReleaseIdentity{Version: "v1.2.0", SourceSHA: strings.Repeat("a", 40), BuilderID: "github:test-builder", PolicyVersion: "sovereign-release-v1"}
	rollback := ReleaseIdentity{Version: "v1.1.0", SourceSHA: strings.Repeat("b", 40), BuilderID: "github:test-builder", PolicyVersion: "sovereign-release-v1"}
	currentDir := writeReleaseAssurance(t, "current", current, privateB64, "")
	rollbackDir := writeReleaseAssurance(t, "rollback", rollback, privateB64, "")
	assets := []Asset{
		fixtureAsset(t, "documentation", "operator-guide.md", false),
		fixtureAsset(t, "configuration", "fairway-config.toml", false),
		fixtureAsset(t, "deployment_baseline", "single-host.yaml", false),
	}
	for _, name := range requiredVerifierNames {
		assets = append(assets, fixtureAsset(t, "verifier", name, true))
	}
	out := filepath.Join(t.TempDir(), "offline")
	manifest, err := Export(ExportOptions{
		OutputDirectory: out, CurrentAssuranceDir: currentDir, RollbackAssuranceDir: rollbackDir,
		TrustedPublicKeyBase64: publicB64, SigningKeyBase64: privateB64,
		CurrentExpected: current, RollbackExpected: rollback, Assets: assets, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Current.Path != "releases/current" || manifest.Rollback.Path != "releases/rollback" || len(manifest.Files) == 0 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	for _, script := range []string{"verify.sh", "install.sh", "rollback.sh"} {
		path := filepath.Join(out, "scripts", script)
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("script %s mode=%v err=%v", script, info.Mode().Perm(), err)
		}
		body, _ := os.ReadFile(path)
		if strings.Contains(string(body), "curl ") || strings.Contains(string(body), "wget ") {
			t.Fatalf("script %s contains network command", script)
		}
	}
	report, err := Verify(VerifyOptions{Directory: out, TrustedPublicKeyBase64: publicB64, CurrentExpected: current, RollbackExpected: rollback})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.SignatureStatus != "verified" || report.CurrentAssuranceStatus != "verified" || report.RollbackAssuranceStatus != "verified" || report.RequiredAssetClassStatus != "complete" {
		t.Fatalf("unexpected verification: %+v", report)
	}

	if err := os.WriteFile(filepath.Join(out, "assets", "documentation", "operator-guide.md"), []byte("tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(VerifyOptions{Directory: out, TrustedPublicKeyBase64: publicB64, CurrentExpected: current, RollbackExpected: rollback}); err == nil || !strings.Contains(err.Error(), "inventory mismatch") {
		t.Fatalf("tamper error=%v", err)
	}
}

func TestOfflineBundleLifecycleInstallUpgradeRollbackAndBackup(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	build := func(pkg, output, version string) {
		t.Helper()
		args := []string{"build", "-trimpath", "-o", output}
		if version != "" {
			args = append(args, "-ldflags", "-X main.version="+version)
		}
		args = append(args, pkg)
		cmd := exec.Command("go", args...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", pkg, err, out)
		}
	}
	binDir := t.TempDir()
	currentBinary := filepath.Join(binDir, "fairway-current")
	rollbackBinary := filepath.Join(binDir, "fairway-rollback")
	verifierBinary := filepath.Join(binDir, "fairway-offline-verify")
	build("./cmd/fairway", currentBinary, "9.9.1")
	build("./cmd/fairway", rollbackBinary, "9.9.0")
	build("./cmd/fairway-offline-verify", verifierBinary, "")

	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	privateB64 := base64.StdEncoding.EncodeToString(privateKey)
	publicB64 := base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))
	current := ReleaseIdentity{Version: "v9.9.1", SourceSHA: strings.Repeat("a", 40), BuilderID: "local:lifecycle-test", PolicyVersion: "sovereign-release-v1"}
	rollback := ReleaseIdentity{Version: "v9.9.0", SourceSHA: strings.Repeat("b", 40), BuilderID: "local:lifecycle-test", PolicyVersion: "sovereign-release-v1"}
	currentDir := writeReleaseAssurance(t, "current", current, privateB64, currentBinary)
	rollbackDir := writeReleaseAssurance(t, "rollback", rollback, privateB64, rollbackBinary)
	assets := []Asset{
		fixtureAsset(t, "documentation", "operator-guide.md", false),
		{Class: "configuration", Name: "fairway-config.toml", Path: filepath.Join(repoRoot, "examples", "fairway-config.toml")},
		fixtureAsset(t, "deployment_baseline", "single-host.yaml", false),
	}
	for _, name := range requiredVerifierNames {
		assets = append(assets, Asset{Class: "verifier", Name: name, Path: verifierBinary, Executable: true})
	}
	bundle := filepath.Join(t.TempDir(), "bundle")
	if _, err := Export(ExportOptions{OutputDirectory: bundle, CurrentAssuranceDir: currentDir, RollbackAssuranceDir: rollbackDir, TrustedPublicKeyBase64: publicB64, SigningKeyBase64: privateB64, CurrentExpected: current, RollbackExpected: rollback, Assets: assets, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	artifacts := strings.TrimSpace(os.Getenv("FAIRWAY_OFFLINE_REHEARSAL_ARTIFACT_DIR"))
	if artifacts == "" {
		artifacts = filepath.Join(t.TempDir(), "artifacts")
	} else {
		artifacts, err = filepath.Abs(artifacts)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(artifacts); !os.IsNotExist(err) {
			t.Fatalf("retained rehearsal artifact path must not already exist: %s", artifacts)
		}
	}
	rehearsal := exec.Command(filepath.Join(repoRoot, "scripts", "ci", "offline_distribution_rehearsal.sh"), bundle, artifacts, current.Version, rollback.Version)
	rehearsal.Env = append(os.Environ(), "FAIRWAY_OFFLINE_TRUSTED_PUBLIC_KEY="+publicB64)
	if out, err := rehearsal.CombinedOutput(); err != nil {
		t.Fatalf("offline rehearsal: %v\n%s", err, out)
	}
	for path, markers := range map[string][]string{
		"readback.txt":            {"result=pass", "current_version=9.9.1", "rollback_version=9.9.0", "data_path=", "pre_upgrade_backup="},
		"cleanup.txt":             {"cleanup=pass"},
		"task-after-rollback.txt": {"Offline lifecycle compatibility proof"},
		"digests.txt":             {"pre-upgrade.db", "post-upgrade.db", "post-rollback.db"},
	} {
		data, err := os.ReadFile(filepath.Join(artifacts, path))
		if err != nil {
			t.Fatalf("read rehearsal artifact %s: %v", path, err)
		}
		for _, marker := range markers {
			if !strings.Contains(string(data), marker) {
				t.Fatalf("artifact %s missing %q:\n%s", path, marker, data)
			}
		}
	}
	symlinkTarget := t.TempDir()
	symlinkPrefix := filepath.Join(t.TempDir(), "linked-prefix")
	if err := os.Symlink(symlinkTarget, symlinkPrefix); err != nil {
		t.Fatal(err)
	}
	unsafeInstall := exec.Command(filepath.Join(bundle, "scripts", "install.sh"), symlinkPrefix)
	unsafeInstall.Env = append(os.Environ(), "FAIRWAY_OFFLINE_TRUSTED_PUBLIC_KEY="+publicB64)
	if out, err := unsafeInstall.CombinedOutput(); err == nil || !strings.Contains(string(out), "must not be a symlink") {
		t.Fatalf("symlinked install prefix output=%q err=%v", out, err)
	}
}

func TestOfflineBundleFailsClosed(t *testing.T) {
	current := ReleaseIdentity{Version: "v1.2.0", SourceSHA: strings.Repeat("a", 40), BuilderID: "builder", PolicyVersion: "policy-v1"}
	rollback := ReleaseIdentity{Version: "v1.1.0", SourceSHA: strings.Repeat("b", 40), BuilderID: "builder", PolicyVersion: "policy-v1"}
	if err := validateExpectedPair(current, current); err == nil || !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("same current/rollback error=%v", err)
	}
	current.SourceSHA = "not-a-sha"
	if err := validateExpectedPair(current, rollback); err == nil || !strings.Contains(err.Error(), "source sha") {
		t.Fatalf("invalid source error=%v", err)
	}
	if err := validateAsset(Asset{Class: "documentation", Name: "../escape", Path: "doc"}); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unsafe asset error=%v", err)
	}
	privateAsset := fixtureAsset(t, "configuration", "private.toml", false)
	if err := os.WriteFile(privateAsset.Path, []byte("client_secret=SHOULD_NOT_COPY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateAssetPrivacy(privateAsset); err == nil || !strings.Contains(err.Error(), "prohibited private content") {
		t.Fatalf("private asset error=%v", err)
	}
	if err := validMetadata("version", " v1.2.0"); err == nil {
		t.Fatal("metadata with surrounding whitespace accepted")
	}
	if err := verifyRequiredVerifierAssets([]Asset{{Class: "verifier", Name: requiredVerifierNames[0], Executable: true}}); err == nil {
		t.Fatal("incomplete verifier platform set accepted")
	}
	malformedArchive := filepath.Join(t.TempDir(), "fairway.tar.gz")
	if err := os.WriteFile(malformedArchive, []byte("not a gzip archive\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyFairwayArchive(malformedArchive); err == nil {
		t.Fatal("malformed release archive accepted")
	}
	if err := rejectDuplicateJSONKeys([]byte(`{"schema":"one","nested":{"key":1,"key":2}}`)); err == nil {
		t.Fatal("nested duplicate JSON key accepted")
	}
}

func writeReleaseAssurance(t *testing.T, _ string, identity ReleaseIdentity, privateKey, binaryPath string) string {
	t.Helper()
	dir := t.TempDir()
	inputs := filepath.Join(dir, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(filename, body string) string {
		path := filepath.Join(inputs, filename)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	created := time.Now().UTC().Format(time.RFC3339Nano)
	evidence := map[string]string{
		"sbom":                      write("sbom.json", `{"spdxVersion":"SPDX-2.3","SPDXID":"SPDXRef-DOCUMENT","name":"fairway","dataLicense":"CC0-1.0","documentNamespace":"https://example.invalid/spdx/fairway","creationInfo":{"created":"2026-01-01T00:00:00Z","creators":["Tool: test"]},"packages":[{"name":"fairway","SPDXID":"SPDXRef-Package"}]}`),
		"vex":                       write("vex.json", `{"@context":"https://openvex.dev/ns/v0.2.0","@id":"https://example.invalid/vex","author":"test","timestamp":"2026-01-01T00:00:00Z","version":1,"statements":[]}`),
		"dependencies":              write("dependencies.jsonl", `{"Path":"github.com/subashram/fairway","Main":true}`+"\n"),
		"licenses":                  write("licenses.csv", "github.com/subashram/fairway,https://example.invalid/LICENSE,Apache-2.0\n"),
		"license_disposition":       write("license-disposition.json", `{"schema":"fairway.release-license-overrides.v1","overrides":[{}]}`),
		"source_provenance":         write("source.json", fmt.Sprintf(`{"schema":"fairway.release-source-provenance.v1","version":%q,"source_sha":%q,"repository":"test","ref":"test"}`, identity.Version, identity.SourceSHA)),
		"build_provenance":          write("build.json", fmt.Sprintf(`{"schema":"fairway.release-build-provenance.v1","builder_id":%q,"run_id":"1","run_attempt":"1","runner_os":"test","runner_arch":"test","go_version":"go test","goreleaser_version":"test","created_at":%q}`, identity.BuilderID, created)),
		"build_recipe":              write("recipe.json", fmt.Sprintf(`{"schema":"fairway.release-build-recipe.v1","source":".goreleaser.yaml","sha256":%q}`, strings.Repeat("c", 64))),
		"test_summary":              write("tests.json", fmt.Sprintf(`{"schema":"fairway.release-test-summary.v1","created_at":%q,"commands":["go test ./..."],"result":"pass"}`, created)),
		"vulnerability_disposition": write("vuln.json", `{"schema":"fairway.release-vulnerability-disposition.v1","scanner":"govulncheck","result":"no_findings","report":"govulncheck.json","authority_boundary":"scanner result only"}`),
	}
	artifacts := map[string]string{}
	if binaryPath == "" {
		binaryPath = write("fairway-fixture", "#!/bin/sh\necho fixture\n")
		if err := os.Chmod(binaryPath, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range supportedTargets {
		artifactName := "fairway_" + strings.TrimPrefix(identity.Version, "v") + "_" + target + ".tar.gz"
		artifactPath := filepath.Join(inputs, artifactName)
		writeFairwayArchive(t, artifactPath, binaryPath)
		artifacts[artifactName] = artifactPath
	}
	out := filepath.Join(dir, "assurance")
	_, err := provenance.ExportReleaseBundle(provenance.ReleaseBundleOptions{
		OutputDirectory: out, Version: identity.Version, SourceSHA: identity.SourceSHA, BuilderID: identity.BuilderID,
		PolicyVersion: identity.PolicyVersion, CreatedAt: time.Now().UTC(), SigningKeyBase64: privateKey,
		Artifacts: artifacts, Evidence: evidence,
		SLSA: provenance.ReleaseSLSAProperties{Specification: "https://slsa.dev/spec/v1.2", SourceVersioned: true, BuildServiceGenerated: true, ProvenanceAvailable: true, BuilderIdentityRecorded: true, BuildRecipeRecorded: true, DependenciesRecorded: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func writeFairwayArchive(t *testing.T, path, binaryPath string) {
	t.Helper()
	source, err := os.Open(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "fairway", Mode: 0o755, Size: info.Size(), ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(tw, source); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func fixtureAsset(t *testing.T, class, name string, executable bool) Asset {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	mode := os.FileMode(0o600)
	if executable {
		mode = 0o700
	}
	if err := os.WriteFile(path, []byte(class+" fixture\n"), mode); err != nil {
		t.Fatal(err)
	}
	return Asset{Class: class, Name: name, Path: path, Executable: executable}
}
