package securityadvisory

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportVerifyAcknowledgeAndTamper(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	patch := filepath.Join(dir, "offline-patch.tar.gz")
	if err := os.WriteFile(patch, []byte("synthetic offline patch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "advisory-package")
	manifest, err := Export(ExportOptions{
		Advisory:         testAdvisory(),
		PatchBundlePath:  patch,
		OutputDirectory:  out,
		SigningKeyBase64: base64.StdEncoding.EncodeToString(privateKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.AdvisoryID != "FAIRWAY-SA-2099-001" || len(manifest.Files) != 3 {
		t.Fatalf("manifest=%+v", manifest)
	}
	verifyOptions := VerifyOptions{Directory: out, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(publicKey), ExpectedAdvisoryID: "FAIRWAY-SA-2099-001", ExpectedPatchBundleID: "fairway-offline-9.9.9", ExpectedRollbackBundleID: "fairway-offline-9.9.8"}
	report, err := Verify(verifyOptions)
	if err != nil || !report.OK || report.SignatureStatus != "verified_pinned" || report.InventoryStatus != "verified_exact" || report.PatchSHA256 == "" {
		t.Fatalf("verification=%+v err=%v", report, err)
	}
	markdown, err := os.ReadFile(filepath.Join(out, "advisory.md"))
	if err != nil || !strings.Contains(string(markdown), "Synthetic mitigation") || !strings.Contains(string(markdown), AuthorityBoundary) {
		t.Fatalf("markdown=%q err=%v", markdown, err)
	}
	ackPath := filepath.Join(dir, "acknowledgement.json")
	ack, err := Acknowledge(AcknowledgeOptions{VerifyOptions: verifyOptions, OutputPath: ackPath, CustomerReference: "lab-customer-001", Status: "received", AcknowledgedAt: time.Date(2099, 1, 2, 3, 4, 5, 0, time.UTC)})
	if err != nil || ack.Status != "received" || ack.PatchSHA256 != report.PatchSHA256 || ack.RollbackBundleID != "fairway-offline-9.9.8" {
		t.Fatalf("ack=%+v err=%v", ack, err)
	}
	if info, err := os.Stat(ackPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("ack mode info=%v err=%v", info, err)
	}
	file, err := os.OpenFile(filepath.Join(out, filepath.FromSlash(patchPath)), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("tamper"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if report, err := Verify(verifyOptions); err == nil || report.OK || strings.Contains(err.Error(), "synthetic offline patch") {
		t.Fatalf("tampered verification=%+v err=%v", report, err)
	}
	verifyOptions.ExpectedRollbackBundleID = "fairway-offline-9.9.7"
	if report, err := Verify(verifyOptions); err == nil || report.OK || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("rollback mismatch verification=%+v err=%v", report, err)
	}
}

func TestAdvisoryStrictPrivacyAndInventoryGuards(t *testing.T) {
	dir := t.TempDir()
	duplicate := filepath.Join(dir, "duplicate.json")
	if err := os.WriteFile(duplicate, []byte(`{"schema":"fairway.security-advisory.v1","schema":"other"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAdvisory(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate error=%v", err)
	}
	unsafe := testAdvisory()
	unsafe.Mitigations = []string{"Authorization: Bearer SHOULD_NOT_PERSIST"}
	if err := Validate(unsafe); err == nil || !strings.Contains(err.Error(), "private content") || strings.Contains(err.Error(), "SHOULD_NOT_PERSIST") {
		t.Fatalf("privacy error=%v", err)
	}
	unsupported := testAdvisory()
	unsupported.Schema = "unexpected.schema"
	if err := Validate(unsupported); err == nil || !strings.Contains(err.Error(), "unsupported advisory schema") {
		t.Fatalf("schema error=%v", err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	patch := filepath.Join(dir, "patch.bin")
	if err := os.WriteFile(patch, []byte("patch"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "package")
	if _, err := Export(ExportOptions{Advisory: testAdvisory(), PatchBundlePath: patch, OutputDirectory: out, SigningKeyBase64: base64.StdEncoding.EncodeToString(privateKey)}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "unknown.txt"), []byte("extra"), 0o600); err != nil {
		t.Fatal(err)
	}
	options := VerifyOptions{Directory: out, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(publicKey), ExpectedAdvisoryID: "FAIRWAY-SA-2099-001", ExpectedPatchBundleID: "fairway-offline-9.9.9", ExpectedRollbackBundleID: "fairway-offline-9.9.8"}
	if report, err := Verify(options); err == nil || report.OK || !strings.Contains(err.Error(), "unknown file") {
		t.Fatalf("unknown inventory report=%+v err=%v", report, err)
	}
	wrongPublic, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	options.TrustedPublicKeyBase64 = base64.StdEncoding.EncodeToString(wrongPublic)
	if report, err := Verify(options); err == nil || report.OK || strings.Contains(err.Error(), base64.StdEncoding.EncodeToString(publicKey)) {
		t.Fatalf("wrong key report=%+v err=%v", report, err)
	}
}

func TestOpenedRegularRejectsPostOpenGrowthReplacementAndSymlink(t *testing.T) {
	dir := t.TempDir()
	growing := filepath.Join(dir, "growing.bin")
	if err := os.WriteFile(growing, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, before, err := openRegularNoFollow(growing, 256)
	if err != nil {
		t.Fatal(err)
	}
	appendFile, err := os.OpenFile(growing, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appendFile.Write(make([]byte, 300)); err != nil {
		t.Fatal(err)
	}
	if err := appendFile.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := consumeOpenedRegular(growing, file, before, 256, nil); err == nil || !strings.Contains(err.Error(), "bounded") {
		t.Fatalf("post-open growth error=%v", err)
	}
	_ = file.Close()

	original := filepath.Join(dir, "replace.bin")
	if err := os.WriteFile(original, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, before, err = openRegularNoFollow(original, 256)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(dir, "replacement.bin")
	if err := os.WriteFile(replacement, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, original); err != nil {
		t.Fatal(err)
	}
	if _, _, err := consumeOpenedRegular(original, file, before, 256, nil); err == nil || !strings.Contains(err.Error(), "path changed") {
		t.Fatalf("post-open replacement error=%v", err)
	}
	_ = file.Close()

	target := filepath.Join(dir, "target.bin")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if file, _, err := openRegularNoFollow(link, 256); err == nil {
		_ = file.Close()
		t.Fatal("symlink unexpectedly opened with O_NOFOLLOW")
	}
}

func testAdvisory() Advisory {
	return Advisory{
		Schema:           AdvisorySchema,
		ID:               "FAIRWAY-SA-2099-001",
		PublishedAt:      "2099-01-02T03:04:05Z",
		Severity:         "high",
		Summary:          "Synthetic restricted-channel advisory",
		AffectedVersions: []string{"9.9.8"},
		FixedVersions:    []string{"9.9.9"},
		Mitigations:      []string{"Synthetic mitigation"},
		VEXUpdates:       []VEXUpdate{{VulnerabilityID: "CVE-2099-0001", Status: "fixed", Justification: "Synthetic fixed-version proof"}},
		PatchBundleID:    "fairway-offline-9.9.9",
		RollbackBundleID: "fairway-offline-9.9.8",
		SupportTrack:     "lts",
		Synthetic:        true,
	}
}
