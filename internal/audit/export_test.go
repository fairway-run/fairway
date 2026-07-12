package audit

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/subashram/fairway/internal/store"
)

func TestAuditExportGenesisVerifiesAndMinimizesDetail(t *testing.T) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "genesis")
	manifest, err := ExportAuditPackage(AuditExportOptions{
		Records: auditTestRecords(3), OutputDirectory: out, GeneratedAt: time.Date(2026, 7, 12, 18, 0, 0, 0, time.UTC),
		PolicyID: "sovereign-audit-v1", SourceVersion: "commit:abc123", TrustedTimeSource: "customer-ntp",
		TrustedTimeEvidence: "artifacts/time-proof.json", TrustedTimeEvidenceSHA256: strings.Repeat("a", 64),
		RetentionPolicy: "customer-retention-v1", LegalHold: "none", ExternalTarget: "worm:customer/archive",
		SigningKeyBase64: base64.StdEncoding.EncodeToString(private), Genesis: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.Genesis || manifest.RecordCount != 3 || manifest.ChainHead == "" {
		t.Fatalf("manifest = %+v", manifest)
	}
	data, err := os.ReadFile(filepath.Join(out, "records.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "authorization: Bearer SHOULD_NOT_EXPORT") || !strings.Contains(string(data), "detail_sha256") {
		t.Fatalf("records leak detail or omit digest: %s", data)
	}
	report, err := VerifyAuditPackage(AuditVerifyOptions{Directory: out, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(public)})
	if err != nil || !report.OK || report.SignatureStatus != "verified_pinned" || report.ContinuityStatus != "genesis" || report.TrustedTimeStatus != "evidence_bound" {
		t.Fatalf("verify report=%+v err=%v", report, err)
	}
	otherPublic, _, _ := ed25519.GenerateKey(nil)
	untrusted, _ := VerifyAuditPackage(AuditVerifyOptions{Directory: out, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(otherPublic)})
	if untrusted.OK || untrusted.SignatureStatus != "untrusted" {
		t.Fatalf("untrusted report = %+v", untrusted)
	}
}

func TestAuditExportDetectsTamperingAndUnknownFiles(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(nil)
	makeExport := func(name string) string {
		t.Helper()
		out := filepath.Join(t.TempDir(), name)
		_, err := ExportAuditPackage(AuditExportOptions{Records: auditTestRecords(3), OutputDirectory: out, GeneratedAt: time.Now().UTC(), PolicyID: "policy-v1", SourceVersion: "commit:abc",
			TrustedTimeSource: "customer-ntp", TrustedTimeEvidence: "time.json", TrustedTimeEvidenceSHA256: strings.Repeat("b", 64), RetentionPolicy: "retain-v1", LegalHold: "none",
			ExternalTarget: "worm:archive", SigningKeyBase64: base64.StdEncoding.EncodeToString(private), Genesis: true})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	trusted := base64.StdEncoding.EncodeToString(public)
	for _, tc := range []struct {
		name string
		edit func(string)
	}{
		{name: "deleted", edit: func(dir string) {
			data, _ := os.ReadFile(filepath.Join(dir, "records.jsonl"))
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			_ = os.WriteFile(filepath.Join(dir, "records.jsonl"), []byte(strings.Join(lines[:2], "\n")+"\n"), 0o600)
		}},
		{name: "reordered", edit: func(dir string) {
			data, _ := os.ReadFile(filepath.Join(dir, "records.jsonl"))
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			lines[0], lines[1] = lines[1], lines[0]
			_ = os.WriteFile(filepath.Join(dir, "records.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
		}},
		{name: "substituted_manifest", edit: func(dir string) {
			data, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
			data = []byte(strings.Replace(string(data), "customer-ntp", "customer-ptp", 1))
			_ = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600)
		}},
		{name: "unknown_file", edit: func(dir string) { _ = os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("x"), 0o600) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := makeExport(tc.name)
			tc.edit(dir)
			report, _ := VerifyAuditPackage(AuditVerifyOptions{Directory: dir, TrustedPublicKeyBase64: trusted})
			if report.OK || len(report.Issues) == 0 {
				t.Fatalf("tampered report = %+v", report)
			}
		})
	}
}

func TestAuditExportContinuityDetectsRollbackAndSupportsKeyRotation(t *testing.T) {
	oldPublic, oldPrivate, _ := ed25519.GenerateKey(nil)
	newPublic, newPrivate, _ := ed25519.GenerateKey(nil)
	root := t.TempDir()
	genesisDir := filepath.Join(root, "genesis")
	genesis, err := ExportAuditPackage(baseAuditOptions(auditTestRecords(2), genesisDir, oldPrivate))
	if err != nil {
		t.Fatal(err)
	}
	genesisBytes, _ := os.ReadFile(filepath.Join(genesisDir, "manifest.json"))
	genesisDigest := sha256.Sum256(genesisBytes)
	current := auditTestRecords(4)
	nextOpts := baseAuditOptions(current, filepath.Join(root, "next"), newPrivate)
	nextOpts.Genesis = false
	nextOpts.GeneratedAt = nextOpts.GeneratedAt.Add(time.Hour)
	nextOpts.PolicyID = "policy-v2"
	nextOpts.SourceVersion = "commit:def"
	nextOpts.TrustedTimeSource = "customer-ptp"
	nextOpts.Previous = &genesis
	nextOpts.PreviousManifestSHA256 = hex.EncodeToString(genesisDigest[:])
	if _, err := ExportAuditPackage(nextOpts); err != nil {
		t.Fatal(err)
	}
	report, _ := VerifyAuditPackage(AuditVerifyOptions{Directory: nextOpts.OutputDirectory, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(newPublic), PreviousDirectory: genesisDir, PreviousTrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(oldPublic)})
	if !report.OK || report.ContinuityStatus != "verified_previous" {
		t.Fatalf("rotated continuity report = %+v", report)
	}
	withoutPrevious, _ := VerifyAuditPackage(AuditVerifyOptions{Directory: nextOpts.OutputDirectory, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(newPublic)})
	if withoutPrevious.OK || withoutPrevious.ContinuityStatus != "previous_required" {
		t.Fatalf("missing previous report = %+v", withoutPrevious)
	}
	rolledBack := baseAuditOptions(auditTestRecords(1), filepath.Join(root, "rollback"), newPrivate)
	rolledBack.Genesis = false
	rolledBack.GeneratedAt = rolledBack.GeneratedAt.Add(time.Hour)
	rolledBack.Previous = &genesis
	rolledBack.PreviousManifestSHA256 = hex.EncodeToString(genesisDigest[:])
	if _, err := ExportAuditPackage(rolledBack); err == nil || !strings.Contains(err.Error(), "behind") {
		t.Fatalf("rollback error = %v", err)
	}
	changed := auditTestRecords(3)
	changed[0].Actor = "substituted-actor"
	diverged := baseAuditOptions(changed, filepath.Join(root, "diverged"), newPrivate)
	diverged.Genesis = false
	diverged.GeneratedAt = diverged.GeneratedAt.Add(time.Hour)
	diverged.Previous = &genesis
	diverged.PreviousManifestSHA256 = hex.EncodeToString(genesisDigest[:])
	if _, err := ExportAuditPackage(diverged); err == nil || !strings.Contains(err.Error(), "does not extend") {
		t.Fatalf("divergence error = %v", err)
	}
}

func TestAuditJSONRejectsDuplicateKeys(t *testing.T) {
	var manifest AuditExportManifest
	err := strictAuditJSON([]byte(`{"schema":"fairway.sovereign-audit-export.v1","schema":"duplicate"}`), &manifest)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate JSON error = %v", err)
	}
}

func TestAuditExportRejectsNonMonotonicRecordTime(t *testing.T) {
	_, private, _ := ed25519.GenerateKey(nil)
	records := auditTestRecords(2)
	records[1].CreatedAt = time.Date(2026, 7, 12, 16, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	opts := baseAuditOptions(records, filepath.Join(t.TempDir(), "time-regression"), private)
	if _, err := ExportAuditPackage(opts); err == nil || !strings.Contains(err.Error(), "not monotonic") {
		t.Fatalf("non-monotonic time error = %v", err)
	}
}

func TestAuditExportRejectsSecretMetadataWithoutEcho(t *testing.T) {
	_, private, _ := ed25519.GenerateKey(nil)
	opts := baseAuditOptions(auditTestRecords(1), filepath.Join(t.TempDir(), "unsafe"), private)
	opts.RetentionPolicy = "secret=SHOULD_NOT_ECHO"
	_, err := ExportAuditPackage(opts)
	if err == nil || !strings.Contains(err.Error(), "secret-like") || strings.Contains(err.Error(), "SHOULD_NOT_ECHO") {
		t.Fatalf("unsafe metadata error = %v", err)
	}
}

func TestLocalAuditEvidenceDigestRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "time.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reference, digest, err := LocalAuditEvidenceDigest(root, "time.json")
	if err != nil || reference != "time.json" || !isSHA256(digest) {
		t.Fatalf("local evidence reference=%q digest=%q err=%v", reference, digest, err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"../outside.json", "https://example.test/time.json", "linked/outside.json"} {
		if _, _, err := LocalAuditEvidenceDigest(root, unsafe); err == nil {
			t.Fatalf("unsafe evidence %q accepted", unsafe)
		}
	}
}

func baseAuditOptions(records []store.AuditRecord, out string, private ed25519.PrivateKey) AuditExportOptions {
	return AuditExportOptions{Records: records, OutputDirectory: out, GeneratedAt: time.Date(2026, 7, 12, 18, 0, 0, 0, time.UTC), PolicyID: "policy-v1", SourceVersion: "commit:abc",
		TrustedTimeSource: "customer-ntp", TrustedTimeEvidence: "time.json", TrustedTimeEvidenceSHA256: strings.Repeat("c", 64), RetentionPolicy: "retain-v1", LegalHold: "none",
		ExternalTarget: "worm:archive", SigningKeyBase64: base64.StdEncoding.EncodeToString(private), Genesis: true}
}

func auditTestRecords(count int) []store.AuditRecord {
	records := make([]store.AuditRecord, 0, count)
	for i := 1; i <= count; i++ {
		records = append(records, store.AuditRecord{ID: int64(i), ProjectID: "test-project", Actor: "actor-" + string(rune('a'+i-1)), Action: "test.action", TaskID: "T-001",
			Detail: "authorization: Bearer SHOULD_NOT_EXPORT", CreatedAt: time.Date(2026, 7, 12, 17, 0, i, 0, time.UTC).Format(time.RFC3339Nano)})
	}
	return records
}
