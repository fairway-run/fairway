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

func TestVerifyPackageRecomputesOSCALComponentDefinition(t *testing.T) {
	dir, _ := exportVerifiablePackage(t, false, false)
	path := filepath.Join(dir, "oscal-component-definition.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document oscalComponentDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	document.ComponentDefinition.Components[0].ControlImplementations[0].ImplementedRequirements[0].Description = "rewritten assertion"
	data, _ = stableJSON(document)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	updateUnsignedManifestFile(t, dir, "oscal-component-definition.json", data)
	report, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.IntegrityOK || !containsIssue(report.Issues, "OSCAL component definition does not match package state") {
		t.Fatalf("rewritten OSCAL component passed: %+v", report)
	}
}

func TestVerifyPackageRetainsV1Compatibility(t *testing.T) {
	dir, _ := exportVerifiablePackage(t, false, false)
	if err := os.Remove(filepath.Join(dir, "oscal-component-definition.json")); err != nil {
		t.Fatal(err)
	}
	scopePath := filepath.Join(dir, "scope.json")
	var scope packageScope
	scopeData, _ := os.ReadFile(scopePath)
	if err := json.Unmarshal(scopeData, &scope); err != nil {
		t.Fatal(err)
	}
	scope.Schema = "fairway.assurance-package-scope.v1"
	scope.ProductVersion = ""
	scope.ReviewDate = ""
	scopeData, _ = stableJSON(scope)
	if err := os.WriteFile(scopePath, scopeData, 0o600); err != nil {
		t.Fatal(err)
	}
	verifyData := verificationInstructionsV1()
	if err := os.WriteFile(filepath.Join(dir, "VERIFY.md"), verifyData, 0o600); err != nil {
		t.Fatal(err)
	}
	var readiness ReadinessReport
	readinessData, _ := os.ReadFile(filepath.Join(dir, "readiness.json"))
	if err := json.Unmarshal(readinessData, &readiness); err != nil {
		t.Fatal(err)
	}
	controlsMarkdown := controlMarkdown(readiness)
	controlsCSV := controlCSV(readiness)
	if err := os.WriteFile(filepath.Join(dir, "controls.md"), controlsMarkdown, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "controls.csv"), controlsCSV, 0o600); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	manifestData, _ := os.ReadFile(manifestPath)
	var manifest PackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Schema = PackageManifestSchemaV1
	manifest.PackageVersion = "v1"
	manifest.ProductVersion = ""
	manifest.ReviewDate = ""
	var files []PackageManifestFile
	for _, entry := range manifest.Files {
		if entry.Path == "oscal-component-definition.json" {
			continue
		}
		if entry.Path == "scope.json" {
			digest := sha256.Sum256(scopeData)
			entry.SHA256, entry.Bytes = hex.EncodeToString(digest[:]), len(scopeData)
		}
		if entry.Path == "VERIFY.md" {
			digest := sha256.Sum256(verifyData)
			entry.SHA256, entry.Bytes = hex.EncodeToString(digest[:]), len(verifyData)
		}
		if entry.Path == "controls.md" {
			digest := sha256.Sum256(controlsMarkdown)
			entry.SHA256, entry.Bytes = hex.EncodeToString(digest[:]), len(controlsMarkdown)
		}
		if entry.Path == "controls.csv" {
			digest := sha256.Sum256(controlsCSV)
			entry.SHA256, entry.Bytes = hex.EncodeToString(digest[:]), len(controlsCSV)
		}
		files = append(files, entry)
	}
	manifest.Files = files
	manifestData, _ = stableJSON(manifest)
	if err := os.WriteFile(manifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || !report.IntegrityOK || report.PackageSchema != PackageManifestSchemaV1 {
		t.Fatalf("v1 package compatibility failed: %+v", report)
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

func TestVerifyPackagePreservesSupersededReviewHistory(t *testing.T) {
	profile := mappingProfile()
	profile.Controls = []Control{{ID: "C-1", Title: "Review", Objective: "Retain review evidence.", Responsibility: "product",
		AssessmentObjectives: []string{"Inspect review facts."}, Evidence: []EvidenceRequirement{{Class: "review", MinimumCount: 1, AcceptedResults: []string{"approve"}}}}}
	at := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	mapped := MapEvidence(profile, Sources{Task: TaskContext{Project: "demo", TaskID: "T-1", Status: "done", UpdatedAt: at.Format(time.RFC3339Nano)}, Reviews: []SourceReview{
		{Index: 1, Domain: "arch", Verdict: "changes", Reviewer: "reviewer", CreatedAt: at.Add(-time.Hour).Format(time.RFC3339Nano)},
		{Index: 2, Domain: "arch", Verdict: "approve", Reviewer: "reviewer", CreatedAt: at.Add(-time.Hour).Format(time.RFC3339Nano)},
	}}, at)
	readiness := BuildReadiness(profile, "task_set", []EvidenceMap{mapped})
	readiness.ScopeID = "review-history"
	dir := filepath.Join(t.TempDir(), "package")
	if _, err := ExportPackage(PackageOptions{Profile: profile, Readiness: readiness, Maps: []EvidenceMap{mapped}, OutputDirectory: dir, CreatedAt: at, ProductVersion: "0.1.13"}); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyPackage(VerifyOptions{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.ControlSufficiency != "sufficient_recorded_evidence" {
		t.Fatalf("review-history package verification=%+v", report)
	}
	var reviews packageReferenceGroup
	data, err := os.ReadFile(filepath.Join(dir, "reviews.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &reviews); err != nil {
		t.Fatal(err)
	}
	if len(reviews.Facts) != 2 || reviews.Facts[0].State != "superseded" || reviews.Facts[1].State != "current" {
		t.Fatalf("packaged review history=%+v", reviews.Facts)
	}
	if strings.Contains(string(data), "reason") || strings.Contains(string(data), "private") {
		t.Fatalf("packaged review history leaked excluded content: %s", data)
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
	if _, err := ExportPackage(PackageOptions{Profile: profile, Readiness: report, Maps: []EvidenceMap{mapped}, OutputDirectory: dir, CreatedAt: at, ProductVersion: "0.1.13", SigningKeyBase64: encoded}); err != nil {
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
