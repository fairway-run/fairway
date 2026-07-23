package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIReleaseRehearsalCreateAndVerify(t *testing.T) {
	dir := t.TempDir()
	version := "v1.2.3"
	sourceSHA := strings.Repeat("a", 40)
	builderID := "fairway-run/fairway/.github/workflows/release-rehearsal.yml@refs/heads/main"
	policyVersion := "sovereign-release-v1"
	writeReleaseRehearsalAssets(t, dir, version)

	created := runCapture(t,
		"release", "rehearsal", "create",
		"--dir", dir,
		"--version", version,
		"--source-sha", sourceSHA,
		"--builder-id", builderID,
		"--policy-version", policyVersion,
		"--created-at", "2026-07-23T12:00:00Z",
	)
	assertContains(t, created, "release_rehearsal_create: pass")
	assertContains(t, created, "assets: 7")

	verified := runCapture(t,
		"release", "rehearsal", "verify",
		"--dir", dir,
		"--version", version,
		"--source-sha", sourceSHA,
		"--builder-id", builderID,
		"--policy-version", policyVersion,
	)
	assertContains(t, verified, "release_rehearsal_verify: true")

	jsonOutput := runCapture(t,
		"--json", "release", "rehearsal", "verify",
		"--dir", dir,
		"--version", version,
		"--source-sha", sourceSHA,
		"--builder-id", builderID,
		"--policy-version", policyVersion,
	)
	assertContains(t, jsonOutput, `"ok": true`)
}

func TestCLIReleaseRehearsalVerifyFailsClosed(t *testing.T) {
	dir := t.TempDir()
	version := "v1.2.3"
	sourceSHA := strings.Repeat("a", 40)
	builderID := "fairway-run/fairway/.github/workflows/release-rehearsal.yml@refs/heads/main"
	policyVersion := "sovereign-release-v1"
	writeReleaseRehearsalAssets(t, dir, version)
	runOK(t,
		"release", "rehearsal", "create",
		"--dir", dir,
		"--version", version,
		"--source-sha", sourceSHA,
		"--builder-id", builderID,
		"--policy-version", policyVersion,
	)
	path := filepath.Join(dir, "fairway_1.2.3_linux_arm64.tar.gz")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("tamper"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	output := runCaptureAllowError(t,
		"release", "rehearsal", "verify",
		"--dir", dir,
		"--version", version,
		"--source-sha", sourceSHA,
		"--builder-id", builderID,
		"--policy-version", policyVersion,
	)
	assertContains(t, output, "release_rehearsal_verify: false")
	assertContains(t, output, "digest or size mismatch")
}

func writeReleaseRehearsalAssets(t *testing.T, dir, version string) {
	t.Helper()
	normalized := strings.TrimPrefix(version, "v")
	var checksums []string
	for _, target := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"} {
		name := "fairway_" + normalized + "_" + target + ".tar.gz"
		content := []byte("archive:" + target)
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(content)
		checksums = append(checksums, hex.EncodeToString(sum[:])+"  "+name)
	}
	if err := os.WriteFile(filepath.Join(dir, "fairway_"+normalized+"_checksums.txt"), []byte(strings.Join(checksums, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	assuranceName := "fairway_" + version + "_release_assurance.tar.gz"
	assurance := []byte("assurance")
	if err := os.WriteFile(filepath.Join(dir, assuranceName), assurance, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(assurance)
	if err := os.WriteFile(filepath.Join(dir, assuranceName+".sha256"), []byte(hex.EncodeToString(sum[:])+"  "+assuranceName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
