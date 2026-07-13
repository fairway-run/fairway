package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/provenance"
)

func TestCLI_ReleaseOfflineExportVerifyAndTamper(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	privateB64 := base64.StdEncoding.EncodeToString(privateKey)
	publicB64 := base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))
	t.Setenv("OFFLINE_SIGNING_KEY", privateB64)
	t.Setenv("OFFLINE_PUBLIC_KEY", publicB64)

	currentVersion, rollbackVersion := "v1.2.0", "v1.1.0"
	currentSHA, rollbackSHA := strings.Repeat("a", 40), strings.Repeat("b", 40)
	builder, policy := "github:test-builder", "sovereign-release-v1"
	currentDir := writeOfflineCLIReleaseAssurance(t, currentVersion, currentSHA, builder, policy, privateB64)
	rollbackDir := writeOfflineCLIReleaseAssurance(t, rollbackVersion, rollbackSHA, builder, policy, privateB64)

	asset := func(name, body string, executable bool) string {
		path := filepath.Join(t.TempDir(), name)
		mode := os.FileMode(0o600)
		if executable {
			mode = 0o700
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	args := []string{
		"release", "offline", "export",
		"--out", filepath.Join(t.TempDir(), "offline"),
		"--current-assurance-dir", currentDir,
		"--rollback-assurance-dir", rollbackDir,
		"--trusted-public-key-env", "OFFLINE_PUBLIC_KEY",
		"--signing-key-env", "OFFLINE_SIGNING_KEY",
		"--created-at", "2026-07-12T00:00:00Z",
		"--current-version", currentVersion,
		"--current-source-sha", currentSHA,
		"--current-builder-id", builder,
		"--current-policy-version", policy,
		"--rollback-version", rollbackVersion,
		"--rollback-source-sha", rollbackSHA,
		"--rollback-builder-id", builder,
		"--rollback-policy-version", policy,
		"--asset", "documentation:operator-guide.md=" + asset("operator-guide.md", "operator guide\n", false),
		"--asset", "configuration:fairway.toml=" + asset("fairway.toml", "[fairway]\n", false),
		"--asset", "deployment_baseline:single-host.yaml=" + asset("single-host.yaml", "schema: test\n", false),
	}
	for _, target := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"} {
		name := "fairway-offline-verify_" + target
		args = append(args, "--asset", "verifier:"+name+"="+asset(name, target+" verifier\n", true))
	}
	out := runCapture(t, args...)
	assertContains(t, out, "release_offline_export:")
	assertContains(t, out, "current_version: "+currentVersion)
	bundleDir := args[4]
	verifyArgs := []string{
		"release", "offline", "verify", "--dir", bundleDir,
		"--trusted-public-key-env", "OFFLINE_PUBLIC_KEY",
		"--current-version", currentVersion, "--current-source-sha", currentSHA, "--current-builder-id", builder, "--current-policy-version", policy,
		"--rollback-version", rollbackVersion, "--rollback-source-sha", rollbackSHA, "--rollback-builder-id", builder, "--rollback-policy-version", policy,
	}
	verified := runCapture(t, verifyArgs...)
	assertContains(t, verified, "release_offline_verify: true")
	jsonOut := runCapture(t, append([]string{"--json"}, verifyArgs...)...)
	assertContains(t, jsonOut, `"schema": "fairway.offline-distribution-verification.v1"`)

	if err := os.WriteFile(filepath.Join(bundleDir, "assets", "documentation", "operator-guide.md"), []byte("tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := captureRun(verifyArgs...); err == nil || !strings.Contains(err.Error(), "inventory mismatch") {
		t.Fatalf("tamper error=%v", err)
	}
}

func writeOfflineCLIReleaseAssurance(t *testing.T, version, source, builder, policy, privateKey string) string {
	t.Helper()
	root := t.TempDir()
	created := "2026-07-12T00:00:00Z"
	values := map[string]string{
		"sbom":                      `{"spdxVersion":"SPDX-2.3","SPDXID":"SPDXRef-DOCUMENT","packages":[{}]}`,
		"vex":                       `{"@context":"https://openvex.dev/ns/v0.2.0","@id":"https://example.invalid/vex","author":"test","timestamp":"2026-07-12T00:00:00Z","version":1,"statements":[]}`,
		"dependencies":              `{"Path":"github.com/subashram/fairway","Main":true}`,
		"licenses":                  `github.com/subashram/fairway,https://example.invalid/LICENSE,Apache-2.0`,
		"license_disposition":       `{"schema":"fairway.release-license-overrides.v1","overrides":[{}]}`,
		"source_provenance":         fmt.Sprintf(`{"schema":"fairway.release-source-provenance.v1","version":%q,"source_sha":%q,"repository":"test","ref":"test"}`, version, source),
		"build_provenance":          fmt.Sprintf(`{"schema":"fairway.release-build-provenance.v1","builder_id":%q,"run_id":"1","run_attempt":"1","runner_os":"test","runner_arch":"test","go_version":"go test","goreleaser_version":"test","created_at":%q}`, builder, created),
		"build_recipe":              fmt.Sprintf(`{"schema":"fairway.release-build-recipe.v1","source":".goreleaser.yaml","sha256":%q}`, strings.Repeat("c", 64)),
		"test_summary":              fmt.Sprintf(`{"schema":"fairway.release-test-summary.v1","created_at":%q,"commands":["go test ./..."],"result":"pass"}`, created),
		"vulnerability_disposition": `{"schema":"fairway.release-vulnerability-disposition.v1","scanner":"govulncheck","result":"no_findings","report":"govulncheck.json","authority_boundary":"scanner result only"}`,
	}
	evidence := map[string]string{}
	for _, class := range provenance.RequiredReleaseEvidence {
		path := filepath.Join(root, class+".json")
		if err := os.WriteFile(path, []byte(values[class]+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		evidence[class] = path
	}
	artifacts := map[string]string{}
	fixtureBinary := filepath.Join(root, "fairway")
	if err := os.WriteFile(fixtureBinary, []byte("#!/bin/sh\necho fixture\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"} {
		artifactName := "fairway_" + strings.TrimPrefix(version, "v") + "_" + target + ".tar.gz"
		artifact := filepath.Join(root, artifactName)
		writeOfflineCLIArchive(t, artifact, fixtureBinary)
		artifacts[artifactName] = artifact
	}
	out := filepath.Join(root, "assurance")
	_, err := provenance.ExportReleaseBundle(provenance.ReleaseBundleOptions{
		OutputDirectory: out, Version: version, SourceSHA: source, BuilderID: builder, PolicyVersion: policy,
		CreatedAt: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC), SigningKeyBase64: privateKey,
		Artifacts: artifacts, Evidence: evidence,
		SLSA: provenance.ReleaseSLSAProperties{SourceVersioned: true, BuildServiceGenerated: true, ProvenanceAvailable: true, BuilderIdentityRecorded: true, BuildRecipeRecorded: true, DependenciesRecorded: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func writeOfflineCLIArchive(t *testing.T, path, binaryPath string) {
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
