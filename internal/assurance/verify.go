package assurance

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const VerificationSchema = "fairway.assurance-package-verification.v1"

var requiredPackageFilesV1 = []string{
	"VERIFY.md", "controls.csv", "controls.json", "controls.md", "decisions.json",
	"evidence-index.json", "exceptions.json", "gaps.json", "oscal-control-map.json",
	"profile.json", "provenances.json", "readiness.json", "responsibilities.json",
	"reviews.json", "scope.json",
}

var requiredPackageFilesV2 = append(append([]string(nil), requiredPackageFilesV1...), "oscal-component-definition.json")

type VerifyOptions struct {
	Directory              string
	TrustedPublicKeyBase64 string
}

type VerificationReport struct {
	Schema                string   `json:"schema"`
	PackageSchema         string   `json:"package_schema,omitempty"`
	ProfileID             string   `json:"profile_id,omitempty"`
	ProfileVersion        string   `json:"profile_version,omitempty"`
	Project               string   `json:"project,omitempty"`
	Scope                 string   `json:"scope,omitempty"`
	ScopeID               string   `json:"scope_id,omitempty"`
	IntegrityOK           bool     `json:"integrity_ok"`
	ControlSufficiency    string   `json:"control_sufficiency"`
	SignatureStatus       string   `json:"signature_status"`
	ExternalCertification string   `json:"external_certification"`
	OK                    bool     `json:"ok"`
	Issues                []string `json:"issues,omitempty"`
	IntegrityIssues       []string `json:"integrity_issues,omitempty"`
	SufficiencyIssues     []string `json:"sufficiency_issues,omitempty"`
	TrustIssues           []string `json:"trust_issues,omitempty"`
	AuthorityBoundary     string   `json:"authority_boundary"`
}

type packageEvidenceIndex struct {
	Schema string         `json:"schema"`
	Facts  []EvidenceFact `json:"facts"`
}

func VerifyPackage(opts VerifyOptions) (VerificationReport, error) {
	report := VerificationReport{Schema: VerificationSchema, ControlSufficiency: "not_evaluated", SignatureStatus: "not_checked",
		ExternalCertification: "not_evaluated", AuthorityBoundary: "integrity and recorded-evidence verification only; no certification, compliance, authorization, approval, or risk acceptance"}
	dir, err := resolvePackageDirectory(opts.Directory)
	if err != nil {
		return report, err
	}
	manifestBytes, err := readBoundedPackageFile(dir, "manifest.json")
	if err != nil {
		return report, err
	}
	var manifest PackageManifest
	if err := strictPackageJSON(manifestBytes, &manifest); err != nil {
		return report, errors.New("decode assurance package manifest")
	}
	report.PackageSchema, report.ProfileID, report.ProfileVersion, report.Project, report.Scope, report.ScopeID = manifest.Schema, manifest.ProfileID, manifest.ProfileVersion, manifest.Project, manifest.Scope, manifest.ScopeID
	requiredFiles, isV2, ok := packageContract(manifest)
	if !ok {
		return report, errors.New("unsupported assurance package manifest schema or version")
	}
	if manifest.AuthorityBoundary != "assessor-ready evidence package only; not certification, compliance, authorization, approval, or risk acceptance" {
		report.Issues = append(report.Issues, "manifest authority boundary is invalid")
	}
	if !manifest.Signed && manifest.SigningKeyID != "" {
		report.Issues = append(report.Issues, "unsigned manifest must not declare a signing key identity")
	}
	createdAt, createdAtErr := time.Parse(time.RFC3339Nano, manifest.CreatedAt)
	if createdAtErr != nil {
		report.Issues = append(report.Issues, "manifest creation time is invalid")
	}
	if isV2 {
		if err := validIdentifier("assurance package product version", manifest.ProductVersion, identifierPattern); err != nil {
			report.Issues = append(report.Issues, "manifest product version is invalid")
		}
		if manifest.ReviewDate != createdAt.UTC().Format(time.DateOnly) {
			report.Issues = append(report.Issues, "manifest review date does not match creation time")
		}
	} else if manifest.ProductVersion != "" || manifest.ReviewDate != "" {
		report.Issues = append(report.Issues, "v1 manifest must not declare v2 product metadata")
	}
	if !contains([]string{"project", "task_set", "release"}, manifest.Scope) || strings.TrimSpace(manifest.ScopeID) == "" {
		report.Issues = append(report.Issues, "manifest scope is invalid")
	}
	if strings.TrimSpace(manifest.Project) == "" {
		report.Issues = append(report.Issues, "manifest source project is missing")
	}
	if len(manifest.TaskIDs) == 0 || !sortedUniqueNonEmpty(manifest.TaskIDs) {
		report.Issues = append(report.Issues, "manifest task ids must be sorted, unique, and non-empty")
	}

	manifestNames := map[string]bool{}
	lastManifestName := ""
	for _, entry := range manifest.Files {
		if !safePackageName(entry.Path) || manifestNames[entry.Path] || (lastManifestName != "" && entry.Path <= lastManifestName) {
			report.Issues = append(report.Issues, "manifest contains an unsafe or duplicate file path")
			continue
		}
		lastManifestName = entry.Path
		manifestNames[entry.Path] = true
		data, err := readBoundedPackageFile(dir, entry.Path)
		if err != nil {
			report.Issues = append(report.Issues, "manifest file is missing or unsafe: "+entry.Path)
			continue
		}
		digest := sha256.Sum256(data)
		if entry.Bytes != len(data) || subtle.ConstantTimeCompare([]byte(entry.SHA256), []byte(hex.EncodeToString(digest[:]))) != 1 {
			report.Issues = append(report.Issues, "manifest digest or size mismatch: "+entry.Path)
		}
	}
	for _, name := range requiredFiles {
		if !manifestNames[name] {
			report.Issues = append(report.Issues, "required package file is missing from manifest: "+name)
		}
	}
	for name := range manifestNames {
		if !contains(requiredFiles, name) {
			report.Issues = append(report.Issues, "manifest contains a file outside the fixed package contract: "+name)
		}
	}
	if err := rejectUnknownPackageFiles(dir, manifestNames, manifest.Signed); err != nil {
		report.Issues = append(report.Issues, err.Error())
	}

	profileBytes, profileErr := readBoundedPackageFile(dir, "profile.json")
	var profile Profile
	if profileErr != nil || strictPackageJSON(profileBytes, &profile) != nil || Validate(profile) != nil || validatePackageClaims(profile) != nil {
		report.Issues = append(report.Issues, "packaged assurance profile is invalid")
	} else {
		digest := sha256.Sum256(profileBytes)
		if profile.ID != manifest.ProfileID || profile.Version != manifest.ProfileVersion || subtle.ConstantTimeCompare([]byte(manifest.ProfileSHA256), []byte(hex.EncodeToString(digest[:]))) != 1 {
			report.Issues = append(report.Issues, "packaged profile identity or digest does not match manifest")
		}
	}

	readinessBytes, readinessErr := readBoundedPackageFile(dir, "readiness.json")
	var readiness ReadinessReport
	if readinessErr != nil || strictPackageJSON(readinessBytes, &readiness) != nil || readiness.Schema != ReadinessSchema {
		report.Issues = append(report.Issues, "packaged readiness report is invalid")
	} else if readiness.ProfileID != manifest.ProfileID || readiness.ProfileVersion != manifest.ProfileVersion || readiness.Scope != manifest.Scope || readiness.ScopeID != manifest.ScopeID || !equalStrings(readiness.TaskIDs, manifest.TaskIDs) {
		report.Issues = append(report.Issues, "readiness profile or scope does not match manifest")
	}

	scopeBytes, scopeErr := readBoundedPackageFile(dir, "scope.json")
	var scope packageScope
	expectedScopeSchema := "fairway.assurance-package-scope.v1"
	if isV2 {
		expectedScopeSchema = "fairway.assurance-package-scope.v2"
	}
	if scopeErr != nil || strictPackageJSON(scopeBytes, &scope) != nil || scope.Schema != expectedScopeSchema || scope.ProfileID != manifest.ProfileID || scope.ProfileVersion != manifest.ProfileVersion || scope.Project != manifest.Project || scope.Scope != manifest.Scope || scope.ScopeID != manifest.ScopeID || !equalStrings(scope.TaskIDs, manifest.TaskIDs) {
		report.Issues = append(report.Issues, "packaged scope does not match manifest")
	}
	if isV2 && (scope.ProductVersion != manifest.ProductVersion || scope.ReviewDate != manifest.ReviewDate) {
		report.Issues = append(report.Issues, "packaged scope product metadata does not match manifest")
	}
	if !isV2 && (scope.ProductVersion != "" || scope.ReviewDate != "") {
		report.Issues = append(report.Issues, "v1 scope must not declare v2 product metadata")
	}
	if err := validatePackageSemanticClaims(manifest, readiness, scope); err != nil {
		report.Issues = append(report.Issues, err.Error())
	}
	verifyReadinessSemantics(&report, profile, readiness, scope)

	indexBytes, indexErr := readBoundedPackageFile(dir, "evidence-index.json")
	var index packageEvidenceIndex
	if indexErr != nil || strictPackageJSON(indexBytes, &index) != nil || index.Schema != "fairway.assurance-evidence-index.v1" {
		report.Issues = append(report.Issues, "packaged evidence index is invalid")
	} else {
		verifyEvidenceIndex(&report, manifest, readiness, index)
		verifyReferenceGroups(&report, dir, index)
		verifyRecomputedReadiness(&report, profile, manifest, readiness, readinessBytes, index)
	}
	verifyReadinessViews(&report, dir, profile, readiness, manifest, createdAt, isV2)
	verifyPackageSignature(&report, dir, manifest, manifestBytes, opts.TrustedPublicKeyBase64)
	report.IntegrityOK = len(report.Issues) == 0
	report.IntegrityIssues = append(report.IntegrityIssues, report.Issues...)
	verifyControlSufficiency(&report, readiness)
	sort.Strings(report.Issues)
	report.OK = report.IntegrityOK && report.ControlSufficiency == "sufficient_recorded_evidence" && (report.SignatureStatus == "unsigned" || report.SignatureStatus == "verified_pinned")
	return report, nil
}

func packageContract(manifest PackageManifest) ([]string, bool, bool) {
	switch {
	case manifest.Schema == PackageManifestSchemaV1 && manifest.PackageVersion == "v1":
		return requiredPackageFilesV1, false, true
	case manifest.Schema == PackageManifestSchema && manifest.PackageVersion == "v2":
		return requiredPackageFilesV2, true, true
	default:
		return nil, false, false
	}
}

func verifyReadinessSemantics(report *VerificationReport, profile Profile, readiness ReadinessReport, scope packageScope) {
	if readiness.AuthorityBoundary != "readiness evidence and gaps only; not certification, compliance, approval, or risk acceptance" || scope.AuthorityBoundary != readiness.AuthorityBoundary {
		report.Issues = append(report.Issues, "readiness or scope authority boundary is invalid")
	}
	if readiness.EvaluatedAt != scope.EvaluatedAt {
		report.Issues = append(report.Issues, "readiness and scope evaluation clocks differ")
	}
	if _, err := time.Parse(time.RFC3339Nano, readiness.EvaluatedAt); err != nil {
		report.Issues = append(report.Issues, "readiness evaluation clock is invalid")
	}
	if len(readiness.Controls) != len(profile.Controls) || readiness.Summary.TotalControls != len(profile.Controls) {
		report.Issues = append(report.Issues, "readiness control count does not match profile")
		return
	}
	allowed := []string{"satisfied_by_recorded_evidence", "partial", "missing", "stale", "conflicting", "customer_responsibility", "external_assessment_required", "exception_recorded", "not_applicable_with_rationale"}
	counts := map[string]int{}
	controlStatus := map[string]string{}
	for i, control := range readiness.Controls {
		definition := profile.Controls[i]
		if control.ControlID != definition.ID || control.Title != definition.Title || control.Responsibility != definition.Responsibility || !contains(allowed, control.Status) || !sortedUniqueNonEmptyAllowEmpty(control.SourceReferences) {
			report.Issues = append(report.Issues, "readiness control metadata is invalid: "+control.ControlID)
		}
		if control.Status == "customer_responsibility" && definition.Responsibility != "customer" {
			report.Issues = append(report.Issues, "customer responsibility status does not match profile: "+control.ControlID)
		}
		if control.Status == "external_assessment_required" && !definition.ExternalAssessmentRequired {
			report.Issues = append(report.Issues, "external assessment status does not match profile: "+control.ControlID)
		}
		counts[control.Status]++
		controlStatus[control.ControlID] = control.Status
	}
	if !equalIntMap(counts, readiness.Summary.ByStatus) {
		report.Issues = append(report.Issues, "readiness summary does not match controls")
	}
	gapsByControl := map[string]int{}
	for _, gap := range readiness.Gaps {
		if controlStatus[gap.ControlID] == "" || gap.Owner == "" || gap.NextEvidenceAction == "" || gap.Freshness == "" || gap.AssessorBoundary == "" || !sortedUniqueNonEmptyAllowEmpty(gap.SourceReferences) {
			report.Issues = append(report.Issues, "readiness gap metadata is invalid: "+gap.ControlID)
		}
		gapsByControl[gap.ControlID]++
	}
	for controlID, status := range controlStatus {
		shouldHaveGap := status != "satisfied_by_recorded_evidence" && status != "not_applicable_with_rationale"
		if shouldHaveGap != (gapsByControl[controlID] > 0) {
			report.Issues = append(report.Issues, "readiness gap coverage does not match control status: "+controlID)
		}
	}
}

func validatePackageSemanticClaims(manifest PackageManifest, readiness ReadinessReport, scope packageScope) error {
	values := []string{manifest.AuthorityBoundary, manifest.ScopeID, readiness.AuthorityBoundary, scope.AuthorityBoundary}
	for _, control := range readiness.Controls {
		values = append(values, control.Title, control.Status, control.Rationale, control.AssessorBoundary)
	}
	for _, gap := range readiness.Gaps {
		values = append(values, gap.Status, gap.NextEvidenceAction, gap.AssessorBoundary)
	}
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, marker := range []string{"certified", "is compliant", "compliance achieved", "is authorized", "fips-validated", "fips validated"} {
			if strings.Contains(lower, marker) {
				return errors.New("assurance package contains a prohibited certification or authorization claim")
			}
		}
	}
	return nil
}

func verifyEvidenceIndex(report *VerificationReport, manifest PackageManifest, readiness ReadinessReport, index packageEvidenceIndex) {
	refs := map[string]EvidenceFact{}
	taskFacts := map[string]bool{}
	for _, fact := range index.Facts {
		if fact.Reference == "" || refs[fact.Reference].Reference != "" {
			report.Issues = append(report.Issues, "evidence index contains an empty or duplicate reference")
			continue
		}
		if fact.TaskID == "" || !contains(manifest.TaskIDs, fact.TaskID) || fact.Project != manifest.Project {
			report.Issues = append(report.Issues, "evidence fact is outside manifest scope: "+fact.Reference)
		}
		if err := validatePackagedEvidenceFact(fact); err != nil {
			report.Issues = append(report.Issues, "evidence fact is invalid: "+fact.Reference)
		}
		if fact.Class == "task" && fact.Reference == "task:"+fact.TaskID {
			taskFacts[fact.TaskID] = true
		}
		refs[fact.Reference] = fact
	}
	for _, taskID := range manifest.TaskIDs {
		if !taskFacts[taskID] {
			report.Issues = append(report.Issues, "source task state fact is missing: "+taskID)
		}
	}
	for _, control := range readiness.Controls {
		for _, ref := range control.SourceReferences {
			if refs[ref].Reference == "" {
				report.Issues = append(report.Issues, "control references unknown evidence: "+ref)
			}
		}
		if control.Status == "exception_recorded" && !hasClassReference(control.SourceReferences, refs, "exception") {
			report.Issues = append(report.Issues, "recorded exception control lacks linked exception evidence: "+control.ControlID)
		}
	}
	for _, gap := range readiness.Gaps {
		for _, ref := range gap.SourceReferences {
			if refs[ref].Reference == "" {
				report.Issues = append(report.Issues, "gap references unknown evidence: "+ref)
			}
		}
		if gap.Status == "exception_recorded" && !hasClassReference(gap.SourceReferences, refs, "exception") {
			report.Issues = append(report.Issues, "recorded exception gap lacks linked exception evidence: "+gap.ControlID)
		}
	}
}

func verifyRecomputedReadiness(report *VerificationReport, profile Profile, manifest PackageManifest, readiness ReadinessReport, readinessBytes []byte, index packageEvidenceIndex) {
	byTask := map[string][]EvidenceFact{}
	applicable := map[string]bool{}
	for _, fact := range index.Facts {
		byTask[fact.TaskID] = append(byTask[fact.TaskID], fact)
		if fact.ProfileApplicable {
			applicable[fact.TaskID] = true
		}
	}
	maps := make([]EvidenceMap, 0, len(manifest.TaskIDs))
	for _, taskID := range manifest.TaskIDs {
		facts := byTask[taskID]
		sort.Slice(facts, func(i, j int) bool {
			if facts[i].Class != facts[j].Class {
				return facts[i].Class < facts[j].Class
			}
			return facts[i].Reference < facts[j].Reference
		})
		maps = append(maps, EvidenceMap{ProfileID: profile.ID, ProfileVersion: profile.Version, TaskID: taskID,
			Applicable: applicable[taskID], EvaluatedAt: readiness.EvaluatedAt, Facts: facts})
	}
	recomputed := BuildReadiness(profile, manifest.Scope, maps)
	recomputed.ScopeID = manifest.ScopeID
	expected, _ := stableJSON(recomputed)
	if subtle.ConstantTimeCompare(readinessBytes, expected) != 1 {
		report.Issues = append(report.Issues, "readiness report does not match deterministic evidence recomputation")
	}
}

func validatePackagedEvidenceFact(fact EvidenceFact) error {
	if !allowedEvidenceClasses[fact.Class] || !allowedResults[fact.Result] {
		return errors.New("unsupported class or result")
	}
	if _, err := time.Parse(time.RFC3339Nano, fact.Timestamp); err != nil {
		return errors.New("invalid timestamp")
	}
	if !contains([]string{"current", "conflicting", "superseded", "unreviewed", "out_of_scope", "external_assertion"}, fact.State) {
		return errors.New("unsupported fact state")
	}
	if !contains([]string{"requirement_relative", "unknown"}, fact.Freshness) {
		return errors.New("unsupported freshness")
	}
	if fact.ConfidenceBoundary == "" || fact.Producer == "" {
		return errors.New("missing confidence or producer")
	}
	return nil
}

func verifyReferenceGroups(report *VerificationReport, dir string, index packageEvidenceIndex) {
	for _, class := range []string{"decision", "review", "provenance", "exception"} {
		data, err := readBoundedPackageFile(dir, class+"s.json")
		var group packageReferenceGroup
		if err != nil || strictPackageJSON(data, &group) != nil || group.Schema != "fairway.assurance-"+class+"-references.v1" {
			report.Issues = append(report.Issues, "packaged "+class+" references are invalid")
			continue
		}
		expected, _ := stableJSON(packageReferenceGroup{Schema: group.Schema, Facts: factsByClass(index.Facts, class)})
		if subtle.ConstantTimeCompare(data, expected) != 1 {
			report.Issues = append(report.Issues, "packaged "+class+" references do not match evidence index")
		}
	}
}

func verifyReadinessViews(report *VerificationReport, dir string, profile Profile, readiness ReadinessReport, manifest PackageManifest, createdAt time.Time, isV2 bool) {
	expectedControls, _ := stableJSON(map[string]any{"schema": "fairway.assurance-control-view.v1", "controls": readiness.Controls})
	controls, err := readBoundedPackageFile(dir, "controls.json")
	if err != nil || subtle.ConstantTimeCompare(controls, expectedControls) != 1 {
		report.Issues = append(report.Issues, "packaged control view does not match readiness report")
	}
	expectedGaps, _ := stableJSON(map[string]any{"schema": "fairway.assurance-gap-view.v1", "gaps": readiness.Gaps})
	gaps, err := readBoundedPackageFile(dir, "gaps.json")
	if err != nil || subtle.ConstantTimeCompare(gaps, expectedGaps) != 1 {
		report.Issues = append(report.Issues, "packaged gap view does not match readiness report")
	}
	expectedMarkdown := controlMarkdown(readiness)
	expectedCSV := controlCSV(readiness)
	if isV2 {
		opts := PackageOptions{Profile: profile, Readiness: readiness, ProductVersion: manifest.ProductVersion, CreatedAt: createdAt}
		expectedMarkdown = controlMarkdownV2(opts)
		expectedCSV = controlCSVV2(opts)
	}
	markdown, err := readBoundedPackageFile(dir, "controls.md")
	if err != nil || subtle.ConstantTimeCompare(markdown, expectedMarkdown) != 1 {
		report.Issues = append(report.Issues, "packaged Markdown control view does not match readiness report")
	}
	csvView, err := readBoundedPackageFile(dir, "controls.csv")
	if err != nil || subtle.ConstantTimeCompare(csvView, expectedCSV) != 1 {
		report.Issues = append(report.Issues, "packaged CSV control view does not match readiness report")
	}
	expectedBoundary, _ := stableJSON(oscalBoundaryForReadiness(readiness))
	boundaryData, err := readBoundedPackageFile(dir, "oscal-control-map.json")
	if err != nil || subtle.ConstantTimeCompare(boundaryData, expectedBoundary) != 1 {
		report.Issues = append(report.Issues, "OSCAL control map boundary is invalid")
	}
	expectedInstructions := verificationInstructionsV1()
	if isV2 {
		expectedInstructions = verificationInstructionsV2()
		expectedOSCAL, _ := stableJSON(oscalComponentForPackage(PackageOptions{Profile: profile, Readiness: readiness, ProductVersion: manifest.ProductVersion, CreatedAt: createdAt}))
		oscalData, oscalErr := readBoundedPackageFile(dir, "oscal-component-definition.json")
		if oscalErr != nil || subtle.ConstantTimeCompare(oscalData, expectedOSCAL) != 1 {
			report.Issues = append(report.Issues, "OSCAL component definition does not match package state")
		}
	}
	instructions, err := readBoundedPackageFile(dir, "VERIFY.md")
	if err != nil || subtle.ConstantTimeCompare(instructions, expectedInstructions) != 1 {
		report.Issues = append(report.Issues, "verification instructions are invalid")
	}
	expectedResponsibilities, _ := stableJSON(packageResponsibilitiesForProfile(profile))
	responsibilities, err := readBoundedPackageFile(dir, "responsibilities.json")
	if err != nil || subtle.ConstantTimeCompare(responsibilities, expectedResponsibilities) != 1 {
		report.Issues = append(report.Issues, "packaged responsibilities do not match profile")
	}
}

func verifyControlSufficiency(report *VerificationReport, readiness ReadinessReport) {
	if readiness.Schema != ReadinessSchema {
		report.ControlSufficiency = "not_evaluated"
		return
	}
	report.ControlSufficiency = "sufficient_recorded_evidence"
	for _, control := range readiness.Controls {
		switch control.Status {
		case "satisfied_by_recorded_evidence", "not_applicable_with_rationale":
		case "stale":
			reason := "control evidence is stale: " + control.ControlID
			report.ControlSufficiency = "insufficient"
			report.SufficiencyIssues = append(report.SufficiencyIssues, reason)
			report.Issues = append(report.Issues, reason)
		default:
			reason := "control is not supported by sufficient recorded evidence: " + control.ControlID
			report.ControlSufficiency = "insufficient"
			report.SufficiencyIssues = append(report.SufficiencyIssues, reason)
			report.Issues = append(report.Issues, reason)
		}
	}
}

func verifyPackageSignature(report *VerificationReport, dir string, manifest PackageManifest, manifestBytes []byte, trusted string) {
	signaturePath := filepath.Join(dir, "signature.json")
	if !manifest.Signed {
		if _, err := os.Lstat(signaturePath); !os.IsNotExist(err) {
			report.Issues = append(report.Issues, "unsigned manifest must not include signature.json")
		}
		if strings.TrimSpace(trusted) != "" {
			report.SignatureStatus = "untrusted"
			report.TrustIssues = append(report.TrustIssues, "trusted public key supplied but package is unsigned")
		} else {
			report.SignatureStatus = "unsigned"
		}
		return
	}
	signatureBytes, err := readBoundedPackageFile(dir, "signature.json")
	if err != nil {
		report.Issues = append(report.Issues, "signed manifest is missing signature.json")
		report.SignatureStatus = "invalid"
		return
	}
	var signature PackageSignature
	if strictPackageJSON(signatureBytes, &signature) != nil || signature.Schema != PackageSignatureSchema || signature.Algorithm != "ed25519-sha256" {
		report.Issues = append(report.Issues, "package signature metadata is invalid")
		report.SignatureStatus = "invalid"
		return
	}
	digest := sha256.Sum256(manifestBytes)
	if subtle.ConstantTimeCompare([]byte(signature.ManifestSHA256), []byte(hex.EncodeToString(digest[:]))) != 1 {
		report.Issues = append(report.Issues, "package signature manifest digest mismatch")
		report.SignatureStatus = "invalid"
		return
	}
	publicKey, err := base64.StdEncoding.DecodeString(signature.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		report.Issues = append(report.Issues, "package signature public key is invalid")
		report.SignatureStatus = "invalid"
		return
	}
	keyDigest := sha256.Sum256(publicKey)
	if manifest.SigningKeyID != "sha256:"+hex.EncodeToString(keyDigest[:]) {
		report.Issues = append(report.Issues, "package signing key identity mismatch")
		report.SignatureStatus = "invalid"
		return
	}
	sig, err := base64.StdEncoding.DecodeString(signature.Signature)
	if err != nil || !ed25519.Verify(publicKey, digest[:], sig) {
		report.Issues = append(report.Issues, "package signature verification failed")
		report.SignatureStatus = "invalid"
		return
	}
	if strings.TrimSpace(trusted) != "" {
		trustedKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(trusted))
		if err != nil || len(trustedKey) != ed25519.PublicKeySize || subtle.ConstantTimeCompare(publicKey, trustedKey) != 1 {
			report.TrustIssues = append(report.TrustIssues, "package signing key does not match trusted public key")
			report.SignatureStatus = "untrusted"
			return
		}
		report.SignatureStatus = "verified_pinned"
	} else {
		report.SignatureStatus = "verified_unpinned"
		report.TrustIssues = append(report.TrustIssues, "package signature is valid but the signing key is not pinned")
	}
}

func resolvePackageDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return "", errors.New("assurance package must be a local directory")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", errors.New("read assurance package directory")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("assurance package must be a non-symlink directory")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", errors.New("resolve assurance package directory")
	}
	return resolved, nil
}

func readBoundedPackageFile(dir, name string) ([]byte, error) {
	if !safePackageName(name) {
		return nil, errors.New("unsafe assurance package file path")
	}
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return nil, errors.New("assurance package file is missing, unsafe, or too large")
	}
	return os.ReadFile(path)
}

func rejectUnknownPackageFiles(dir string, manifestNames map[string]bool, signed bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.New("read assurance package directory")
	}
	allowed := map[string]bool{"manifest.json": true}
	if signed {
		allowed["signature.json"] = true
	}
	for name := range manifestNames {
		allowed[name] = true
	}
	for _, entry := range entries {
		if !allowed[entry.Name()] || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("assurance package contains an unknown or unsafe file: " + entry.Name())
		}
	}
	return nil
}

func strictPackageJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("JSON must contain one value")
	}
	return nil
}

func safePackageName(name string) bool {
	return name != "" && name == filepath.Base(name) && name != "." && name != ".." && !strings.ContainsAny(name, `/\\`)
}

func sortedUniqueNonEmpty(values []string) bool {
	for i, value := range values {
		if strings.TrimSpace(value) == "" || (i > 0 && values[i-1] >= value) {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedUniqueNonEmptyAllowEmpty(values []string) bool {
	for i, value := range values {
		if strings.TrimSpace(value) == "" || (i > 0 && values[i-1] >= value) {
			return false
		}
	}
	return true
}

func equalIntMap(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

func hasClassReference(refs []string, facts map[string]EvidenceFact, class string) bool {
	for _, ref := range refs {
		if facts[ref].Class == class {
			return true
		}
	}
	return false
}
