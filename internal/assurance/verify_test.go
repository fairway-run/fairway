package assurance

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestVerifyPackageSeparatesIntegritySufficiencyAndTrust(t *testing.T) {
	dir, publicKey := exportVerifiablePackage(t, true, false)
	report, err := VerifyPackage(VerifyOptions{Directory: dir, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(publicKey)})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || !report.IntegrityOK || report.ControlSufficiency != "sufficient_recorded_evidence" || report.SignatureStatus != "verified_pinned" || report.ExternalCertification != "not_evaluated" {
		t.Fatalf("unexpected verification: %+v", report)
	}
	unpinned, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if unpinned.OK || !unpinned.IntegrityOK || unpinned.SignatureStatus != "verified_unpinned" || len(unpinned.TrustIssues) != 1 {
		t.Fatalf("unpinned signature was trusted: %+v", unpinned)
	}

	other := make([]byte, ed25519.PublicKeySize)
	untrusted, err := VerifyPackage(VerifyOptions{Directory: dir, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(other)})
	if err != nil {
		t.Fatal(err)
	}
	if untrusted.OK || !untrusted.IntegrityOK || untrusted.SignatureStatus != "untrusted" || len(untrusted.TrustIssues) != 1 {
		t.Fatalf("trusted-key mismatch passed: %+v", untrusted)
	}
}

func TestVerifyPackageDetectsTamperingAndUnsafeFiles(t *testing.T) {
	dir, _ := exportVerifiablePackage(t, false, false)
	if err := os.WriteFile(filepath.Join(dir, "controls.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.IntegrityOK || !containsIssue(report.Issues, "manifest digest or size mismatch: controls.json") {
		t.Fatalf("tampering passed: %+v", report)
	}

	dir, _ = exportVerifiablePackage(t, false, false)
	if err := os.WriteFile(filepath.Join(dir, "unexpected.txt"), []byte("injected"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err = VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.IntegrityOK || !containsIssue(report.Issues, "unknown or unsafe file") {
		t.Fatalf("unknown file passed: %+v", report)
	}
}

func TestVerifyPackageRejectsManifestListedExtraFile(t *testing.T) {
	dir, _ := exportVerifiablePackage(t, false, false)
	extra := []byte("unsupported assertion\n")
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), extra, 0o600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	data, _ := os.ReadFile(manifestPath)
	var manifest PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(extra)
	manifest.Files = append(manifest.Files, PackageManifestFile{Path: "extra.txt", SHA256: hex.EncodeToString(digest[:]), Bytes: len(extra)})
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	data, _ = stableJSON(manifest)
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.IntegrityOK || !containsIssue(report.Issues, "outside the fixed package contract") {
		t.Fatalf("manifest-listed extra file passed: %+v", report)
	}
}

func TestVerifyPackageFailsOverallButKeepsIntegrityForMissingOrStaleEvidence(t *testing.T) {
	dir, _ := exportVerifiablePackage(t, false, true)
	report, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !report.IntegrityOK || report.ControlSufficiency != "insufficient" || !containsIssue(report.Issues, "control evidence is stale") {
		t.Fatalf("stale evidence boundary failed: %+v", report)
	}
}

func TestVerifyPackageRejectsGeneratedClaimsEvenWithUpdatedDigests(t *testing.T) {
	dir, _ := exportVerifiablePackage(t, false, false)
	profilePath := filepath.Join(dir, "profile.json")
	var profile Profile
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	profile.Controls[0].Objective = "Product is certified."
	data, _ = stableJSON(profile)
	if err := os.WriteFile(profilePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	manifestData, _ := os.ReadFile(manifestPath)
	var manifest PackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	manifest.ProfileSHA256 = hex.EncodeToString(digest[:])
	for i := range manifest.Files {
		if manifest.Files[i].Path == "profile.json" {
			manifest.Files[i].SHA256 = hex.EncodeToString(digest[:])
			manifest.Files[i].Bytes = len(data)
		}
	}
	manifestData, _ = stableJSON(manifest)
	if err := os.WriteFile(manifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.IntegrityOK || !containsIssue(report.Issues, "packaged assurance profile is invalid") {
		t.Fatalf("generated claim passed: %+v", report)
	}
}

func TestVerifyPackageRecomputesReadinessInsteadOfTrustingUpdatedManifest(t *testing.T) {
	dir, _ := exportVerifiablePackage(t, false, false)
	path := filepath.Join(dir, "readiness.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var readiness ReadinessReport
	if err := json.Unmarshal(data, &readiness); err != nil {
		t.Fatal(err)
	}
	readiness.Controls[0].Status = "missing"
	readiness.Controls[0].Rationale = "required recorded proof is missing"
	readiness.Summary.ByStatus = map[string]int{"missing": 1}
	readiness.Gaps = []AssuranceGap{{ControlID: readiness.Controls[0].ControlID, EvidenceClass: "evidence", Status: "missing", Owner: "product", NextEvidenceAction: "record a scoped evidence evidence reference with an accepted result", Freshness: "no maximum_age declared", AssessorBoundary: "recorded proof supports assessment preparation only"}}
	data, _ = stableJSON(readiness)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	updateUnsignedManifestFile(t, dir, "readiness.json", data)
	report, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.IntegrityOK || !containsIssue(report.Issues, "deterministic evidence recomputation") {
		t.Fatalf("rewritten readiness passed: %+v", report)
	}
}

func exportVerifiablePackage(t *testing.T, signed, stale bool) (string, ed25519.PublicKey) {
	t.Helper()
	profile := mappingProfile()
	if stale {
		profile.Controls[0].Evidence[0].MaximumAge = "1h"
	}
	at := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	evidenceAt := at.Add(-time.Minute)
	if stale {
		evidenceAt = at.Add(-2 * time.Hour)
	}
	facts := []EvidenceFact{
		{Reference: "task:T-1", Class: "task", Result: "done", Timestamp: at.Add(-time.Hour).Format(time.RFC3339Nano), Producer: "fairway", Project: "demo", TaskID: "T-1", ProfileApplicable: true, Freshness: "requirement_relative", ConfidenceBoundary: "recorded metadata", State: "current"},
		{Reference: "evidence:T-1:1", Class: "evidence", Result: "pass", Timestamp: evidenceAt.Format(time.RFC3339Nano), Producer: "fairway", Project: "demo", TaskID: "T-1", ProfileApplicable: true, Freshness: "requirement_relative", ConfidenceBoundary: "recorded metadata", State: "current"},
	}
	mapped := EvidenceMap{Schema: EvidenceMapSchema, ProfileID: profile.ID, ProfileVersion: profile.Version, Project: "demo", TaskID: "T-1", Applicable: true, EvaluatedAt: at.Format(time.RFC3339Nano), Facts: facts}
	report := BuildReadiness(profile, "task_set", []EvidenceMap{mapped})
	report.ScopeID = "verification"
	dir := filepath.Join(t.TempDir(), "package")
	var encoded string
	var public ed25519.PublicKey
	if signed {
		seed := make([]byte, ed25519.SeedSize)
		for i := range seed {
			seed[i] = byte(i + 1)
		}
		private := ed25519.NewKeyFromSeed(seed)
		public = private.Public().(ed25519.PublicKey)
		encoded = base64.StdEncoding.EncodeToString(seed)
	}
	if _, err := ExportPackage(PackageOptions{Profile: profile, Readiness: report, Maps: []EvidenceMap{mapped}, OutputDirectory: dir, CreatedAt: at, SigningKeyBase64: encoded}); err != nil {
		t.Fatal(err)
	}
	return dir, public
}

func containsIssue(issues []string, substring string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, substring) {
			return true
		}
	}
	return false
}

func updateUnsignedManifestFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	path := filepath.Join(dir, "manifest.json")
	manifestData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest PackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	for i := range manifest.Files {
		if manifest.Files[i].Path == name {
			manifest.Files[i].SHA256 = hex.EncodeToString(digest[:])
			manifest.Files[i].Bytes = len(data)
		}
	}
	manifestData, _ = stableJSON(manifest)
	if err := os.WriteFile(path, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
}
