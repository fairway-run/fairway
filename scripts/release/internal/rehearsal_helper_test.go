package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestArchiveFileAndDirectoryAreDeterministic(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fairway")
	if err := os.WriteFile(binary, []byte("binary\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "first.tar.gz")
	second := filepath.Join(dir, "second.tar.gz")
	if err := archiveFile(binary, "fairway", first); err != nil {
		t.Fatal(err)
	}
	if err := archiveFile(binary, "fairway", second); err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := os.ReadFile(first)
	secondBytes, _ := os.ReadFile(second)
	if !reflect.DeepEqual(firstBytes, secondBytes) {
		t.Fatal("file archives are not deterministic")
	}
	entries := readArchive(t, first)
	if len(entries) != 1 || entries[0] != "fairway:0755" {
		t.Fatalf("file archive entries = %#v", entries)
	}

	inputDir := filepath.Join(dir, "bundle")
	if err := os.Mkdir(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "manifest.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirFirst := filepath.Join(dir, "dir-first.tar.gz")
	dirSecond := filepath.Join(dir, "dir-second.tar.gz")
	if err := archiveDirectory(inputDir, "bundle", dirFirst); err != nil {
		t.Fatal(err)
	}
	if err := archiveDirectory(inputDir, "bundle", dirSecond); err != nil {
		t.Fatal(err)
	}
	dirFirstBytes, _ := os.ReadFile(dirFirst)
	dirSecondBytes, _ := os.ReadFile(dirSecond)
	if !reflect.DeepEqual(dirFirstBytes, dirSecondBytes) {
		t.Fatal("directory archives are not deterministic")
	}
}

func TestChecksumFileUsesPortableBasename(t *testing.T) {
	dir := t.TempDir()
	inputDir := filepath.Join(dir, "nested", "artifacts")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(inputDir, "fairway_v0.2.4_release_assurance.tar.gz")
	if err := os.WriteFile(input, []byte("release assurance\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "checksum.txt")
	if err := checksumFile(input, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(data))
	fields := strings.Fields(line)
	if len(fields) != 2 || fields[1] != filepath.Base(input) {
		t.Fatalf("checksum = %q; want digest and portable basename", line)
	}
	if strings.Contains(line, inputDir) {
		t.Fatalf("checksum retained build path: %q", line)
	}
	if err := checksumFile(input, out); err == nil {
		t.Fatal("checksum overwrote existing output")
	}
}

func TestGenerateKeyWritesPrivateModeAndFingerprint(t *testing.T) {
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "private.b64")
	publicPath := filepath.Join(dir, "public.b64")
	if err := generateKey(privatePath, publicPath); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private mode = %o", info.Mode().Perm())
	}
	fingerprint, err := publicKeyFingerprint(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fingerprint) != len("sha256:")+64 {
		t.Fatalf("fingerprint = %q", fingerprint)
	}
}

func TestArchiveRejectsSymlinkAndExistingOutput(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("value"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := archiveFile(link, "fairway", filepath.Join(dir, "link.tar.gz")); err == nil {
		t.Fatal("archiveFile accepted symlink")
	}
	existing := filepath.Join(dir, "existing.tar.gz")
	if err := os.WriteFile(existing, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := archiveFile(target, "fairway", existing); err == nil {
		t.Fatal("archiveFile replaced existing output")
	}
	noiseDir := filepath.Join(dir, "noise")
	if err := os.Mkdir(noiseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noiseDir, "._manifest.json"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := archiveDirectory(noiseDir, "noise", filepath.Join(dir, "noise.tar.gz")); err == nil {
		t.Fatal("archiveDirectory accepted macOS metadata noise")
	}
}

func TestScanRetainedDistinguishesParserMarkersFromCredentialMaterial(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "verifier"), []byte("decode failure: -----BEGIN RSA PRIVATE KEY-----reflect parser marker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := scanRetainedTree(dir); err != nil {
		t.Fatalf("safe parser marker rejected: %v", err)
	}

	pem := filepath.Join(dir, "leak.txt")
	if err := os.WriteFile(pem, []byte("-----BEGIN RSA PRIVATE KEY-----\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n-----END RSA PRIVATE KEY-----\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := scanRetainedTree(dir); err == nil {
		t.Fatal("actual private key block was accepted")
	}
	if err := os.Remove(pem); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "headers.txt"), []byte("Authorization: Bearer abcdefghijklmnopqrstuvwxyz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := scanRetainedTree(dir); err == nil {
		t.Fatal("bearer credential was accepted")
	}
}

func TestScanRetainedRejectsArchiveSecretAndForbiddenName(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config.txt"), []byte("client_secret=abcdefghijklmnopqrstuvwxyz123456\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "bundle.tar.gz")
	if err := archiveDirectory(source, "bundle", archive); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	if err := scanRetainedTree(dir); err == nil {
		t.Fatal("secret assignment in archive was accepted")
	}
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release-private.b64"), []byte("not-a-key"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := scanRetainedTree(dir); err == nil {
		t.Fatal("forbidden private filename was accepted")
	}
}

func TestWriteFailurePacketRemovesStagingAndRetainsExactInventory(t *testing.T) {
	parent := t.TempDir()
	staging := filepath.Join(parent, ".fairway-sovereign-rehearsal-staging.test")
	for _, dir := range []string{"diagnostics", "media", "trust-bootstrap"} {
		if err := os.MkdirAll(filepath.Join(staging, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	partials := map[string]string{
		"diagnostics/build.log":                "unbounded diagnostic output\n",
		"media/partial.tar.gz":                 "partial media\n",
		"trust-bootstrap/release-private.b64":  "partial private material\n",
		"trust-bootstrap/trust-bootstrap.json": "{}\n",
		"readback.json":                        "{}\n",
	}
	for name, value := range partials {
		if err := os.WriteFile(filepath.Join(staging, filepath.FromSlash(name)), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	output := filepath.Join(parent, "failed-rehearsal")
	if err := writeFailurePacket(output, staging, "build-current-archives", 17); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(staging); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staging remains after failure packet: %v", err)
	}
	want := []string{"diagnostics/failure.json"}
	if got := regularFileInventory(t, output); !reflect.DeepEqual(got, want) {
		t.Fatalf("failure output inventory = %#v, want %#v", got, want)
	}
	data, err := os.ReadFile(filepath.Join(output, "diagnostics", "failure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Schema                 string `json:"schema"`
		Phase                  string `json:"phase"`
		ExitCode               int    `json:"exit_code"`
		PrivateSigningMaterial string `json:"private_signing_material"`
	}
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Schema != "fairway.sovereign-rehearsal-build-failure.v1" || packet.Phase != "build-current-archives" || packet.ExitCode != 17 || packet.PrivateSigningMaterial != "not_retained" {
		t.Fatalf("failure packet = %#v", packet)
	}

	existing := filepath.Join(parent, "existing-output")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(existing, "marker")
	if err := os.WriteFile(marker, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	secondStaging := filepath.Join(parent, ".fairway-sovereign-rehearsal-staging.second")
	if err := os.Mkdir(secondStaging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFailurePacket(existing, secondStaging, "tool-preflight", 1); err == nil {
		t.Fatal("failure packet replaced an existing output")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "preserve" {
		t.Fatalf("existing output changed: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(secondStaging); err != nil {
		t.Fatalf("staging was removed before existing-output rejection: %v", err)
	}
}

func TestPromoteRetainedTreeRequiresQuiescentScannedBytes(t *testing.T) {
	parent := t.TempDir()
	staging := filepath.Join(parent, ".fairway-sovereign-rehearsal-staging.success")
	if err := os.MkdirAll(filepath.Join(staging, "diagnostics"), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(staging, "diagnostics", "build.log")
	if err := os.WriteFile(logPath, []byte("complete\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(parent, "verified-output")
	if err := promoteRetainedTree(staging, output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(staging); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staging remains after promotion: %v", err)
	}
	if got := regularFileInventory(t, output); !reflect.DeepEqual(got, []string{"diagnostics/build.log"}) {
		t.Fatalf("promoted inventory = %#v", got)
	}

	mutableStaging := filepath.Join(parent, ".fairway-sovereign-rehearsal-staging.delayed-log")
	if err := os.MkdirAll(filepath.Join(mutableStaging, "diagnostics"), 0o755); err != nil {
		t.Fatal(err)
	}
	mutableLog := filepath.Join(mutableStaging, "diagnostics", "build.log")
	if err := os.WriteFile(mutableLog, []byte("before scan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mutableOutput := filepath.Join(parent, "must-not-promote")
	err := promoteRetainedTreeWithScan(mutableStaging, mutableOutput, func(root string) error {
		if err := scanRetainedTree(root); err != nil {
			return err
		}
		file, err := os.OpenFile(mutableLog, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString("delayed append after scan\n")
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	})
	if err == nil || !strings.Contains(err.Error(), "changed during verification") {
		t.Fatalf("delayed log mutation error = %v", err)
	}
	if _, err := os.Lstat(mutableOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mutable tree was promoted: %v", err)
	}
	if _, err := os.Stat(mutableStaging); err != nil {
		t.Fatalf("mutable staging unavailable for failure cleanup: %v", err)
	}
}

func TestRehearsalBuilderFailureRetainsOnlyBoundedPacket(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	fakeBin := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dirname", "git", "go", "mkdir", "mktemp", "rm"} {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("required test utility %s is unavailable", name)
		}
		if err := os.Symlink(target, filepath.Join(fakeBin, name)); err != nil {
			t.Fatalf("link %s: %v", name, err)
		}
	}
	script, err := filepath.Abs(filepath.Join("..", "build_sovereign_rehearsal_media.sh"))
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	output := filepath.Join(parent, "failed-rehearsal")
	cmd := exec.Command(bash, script,
		"--current-version", "test-current",
		"--rollback-ref", "test-rollback",
		"--rollback-version", "test-rollback",
		"--output-root", output,
		"--builder-id", "test:builder",
		"--policy-version", "test-policy",
		"--created-at", "2026-07-13T00:00:00Z",
	)
	cmd.Env = environmentWithout("PATH", "GOCACHE", "CGO_ENABLED")
	cmd.Env = append(cmd.Env,
		"PATH="+fakeBin,
		"GOCACHE="+filepath.Join(t.TempDir(), "go-cache"),
		"CGO_ENABLED=0",
	)
	combined, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("builder succeeded without goreleaser: %s", combined)
	}
	want := []string{"diagnostics/failure.json"}
	if got := regularFileInventory(t, output); !reflect.DeepEqual(got, want) {
		t.Fatalf("forced failure inventory = %#v, want %#v", got, want)
	}
	data, err := os.ReadFile(filepath.Join(output, "diagnostics", "failure.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"phase":"tool-preflight"`) || !strings.Contains(string(data), `"private_signing_material":"not_retained"`) {
		t.Fatalf("forced failure packet = %s", data)
	}
	staging, err := filepath.Glob(filepath.Join(parent, ".fairway-sovereign-rehearsal-staging.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(staging) != 0 {
		t.Fatalf("forced failure retained staging: %#v", staging)
	}
}

func TestResolveGoReleaserActionTool(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	script, err := filepath.Abs(filepath.Join("..", "resolve_goreleaser_action_tool.sh"))
	if err != nil {
		t.Fatal(err)
	}

	run := func(t *testing.T, root string) (string, error) {
		t.Helper()
		cmd := exec.Command(bash, script, root, "2.17.0")
		output, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(output)), err
	}
	writeTool := func(t *testing.T, root, arch string) string {
		t.Helper()
		path := filepath.Join(root, "goreleaser-action", "2.17.0", arch, "goreleaser")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("zero", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "goreleaser-action", "2.17.0"), 0o755); err != nil {
			t.Fatal(err)
		}
		output, err := run(t, root)
		if err == nil || !strings.Contains(output, "found 0") {
			t.Fatalf("zero-match result = %q, %v", output, err)
		}
	})

	t.Run("one", func(t *testing.T) {
		root := t.TempDir()
		want := writeTool(t, root, "arm64")
		output, err := run(t, root)
		if err != nil || output != want {
			t.Fatalf("one-match result = %q, %v; want %q", output, err, want)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		root := t.TempDir()
		writeTool(t, root, "amd64")
		writeTool(t, root, "arm64")
		output, err := run(t, root)
		if err == nil || !strings.Contains(output, "found 2") {
			t.Fatalf("multiple-match result = %q, %v", output, err)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		target := writeTool(t, t.TempDir(), "arm64")
		link := filepath.Join(root, "goreleaser-action", "2.17.0", "arm64", "goreleaser")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		output, err := run(t, root)
		if err == nil || !strings.Contains(output, "non-symlink") {
			t.Fatalf("symlink result = %q, %v", output, err)
		}
	})
}

func regularFileInventory(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular output %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func environmentWithout(keys ...string) []string {
	blocked := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		blocked[key] = struct{}{}
	}
	var result []string
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if _, skip := blocked[key]; !skip {
			result = append(result, value)
		}
	}
	return result
}

func readArchive(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var entries []string
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, header.Name+":"+formatMode(header.Mode))
	}
	return entries
}

func formatMode(mode int64) string {
	return fmt.Sprintf("%04o", mode&0o777)
}
