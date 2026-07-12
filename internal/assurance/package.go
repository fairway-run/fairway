package assurance

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	PackageManifestSchema  = "fairway.assurance-package-manifest.v1"
	PackageSignatureSchema = "fairway.assurance-package-signature.v1"
)

type PackageOptions struct {
	Profile          Profile
	Readiness        ReadinessReport
	Maps             []EvidenceMap
	OutputDirectory  string
	CreatedAt        time.Time
	SigningKeyBase64 string
}

type PackageManifest struct {
	Schema            string                `json:"schema"`
	PackageVersion    string                `json:"package_version"`
	CreatedAt         string                `json:"created_at"`
	ProfileID         string                `json:"profile_id"`
	ProfileVersion    string                `json:"profile_version"`
	ProfileSHA256     string                `json:"profile_sha256"`
	Project           string                `json:"project"`
	Scope             string                `json:"scope"`
	ScopeID           string                `json:"scope_id"`
	TaskIDs           []string              `json:"task_ids"`
	Files             []PackageManifestFile `json:"files"`
	Signed            bool                  `json:"signed"`
	SigningKeyID      string                `json:"signing_key_id,omitempty"`
	AuthorityBoundary string                `json:"authority_boundary"`
}

type PackageManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type PackageSignature struct {
	Schema         string `json:"schema"`
	Algorithm      string `json:"algorithm"`
	ManifestSHA256 string `json:"manifest_sha256"`
	PublicKey      string `json:"public_key"`
	Signature      string `json:"signature"`
}

type packageScope struct {
	Schema            string   `json:"schema"`
	ProfileID         string   `json:"profile_id"`
	ProfileVersion    string   `json:"profile_version"`
	FrameworkID       string   `json:"framework_id"`
	FrameworkVersion  string   `json:"framework_version"`
	Project           string   `json:"project"`
	Scope             string   `json:"scope"`
	ScopeID           string   `json:"scope_id"`
	TaskIDs           []string `json:"task_ids"`
	EvaluatedAt       string   `json:"evaluated_at"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

type packageReferenceGroup struct {
	Schema string         `json:"schema"`
	Facts  []EvidenceFact `json:"facts"`
}

type packageResponsibilities struct {
	Schema                     string   `json:"schema"`
	CustomerControls           []string `json:"customer_controls"`
	SharedControls             []string `json:"shared_controls"`
	ExternalAssessmentControls []string `json:"external_assessment_controls"`
}

type oscalBoundary struct {
	Schema           string                 `json:"schema"`
	NotOSCALDocument bool                   `json:"not_oscal_document"`
	Boundary         string                 `json:"boundary"`
	Controls         []oscalBoundaryControl `json:"controls"`
}

type oscalBoundaryControl struct {
	ControlID          string   `json:"control_id"`
	FairwayStatus      string   `json:"fairway_status"`
	EvidenceReferences []string `json:"evidence_references,omitempty"`
	AssessorBoundary   string   `json:"assessor_boundary"`
}

func ExportPackage(opts PackageOptions) (PackageManifest, error) {
	if err := Validate(opts.Profile); err != nil {
		return PackageManifest{}, err
	}
	if err := validatePackageClaims(opts.Profile); err != nil {
		return PackageManifest{}, err
	}
	if opts.Readiness.Schema != ReadinessSchema || opts.Readiness.ProfileID != opts.Profile.ID || opts.Readiness.ProfileVersion != opts.Profile.Version {
		return PackageManifest{}, errors.New("assurance readiness report does not match profile")
	}
	if opts.CreatedAt.IsZero() {
		return PackageManifest{}, errors.New("assurance package creation time is required")
	}
	var signingKey ed25519.PrivateKey
	if strings.TrimSpace(opts.SigningKeyBase64) != "" {
		var err error
		signingKey, err = decodePackageSigningKey(opts.SigningKeyBase64)
		if err != nil {
			return PackageManifest{}, err
		}
	}
	outputDirectory, err := preparePackageDirectory(opts.OutputDirectory)
	if err != nil {
		return PackageManifest{}, err
	}

	files, err := buildPackageFiles(opts)
	if err != nil {
		_ = os.RemoveAll(outputDirectory)
		return PackageManifest{}, err
	}
	profileDigest := sha256.Sum256(files["profile.json"])
	project, err := packageProject(opts.Maps)
	if err != nil {
		_ = os.RemoveAll(outputDirectory)
		return PackageManifest{}, err
	}
	manifest := PackageManifest{Schema: PackageManifestSchema, PackageVersion: "v1", CreatedAt: opts.CreatedAt.UTC().Format(time.RFC3339Nano),
		ProfileID: opts.Profile.ID, ProfileVersion: opts.Profile.Version, ProfileSHA256: hex.EncodeToString(profileDigest[:]),
		Project: project, Scope: opts.Readiness.Scope, ScopeID: opts.Readiness.ScopeID, TaskIDs: append([]string(nil), opts.Readiness.TaskIDs...),
		Signed:            strings.TrimSpace(opts.SigningKeyBase64) != "",
		AuthorityBoundary: "assessor-ready evidence package only; not certification, compliance, authorization, approval, or risk acceptance"}
	if len(signingKey) > 0 {
		publicKey := signingKey.Public().(ed25519.PublicKey)
		keyDigest := sha256.Sum256(publicKey)
		manifest.SigningKeyID = "sha256:" + hex.EncodeToString(keyDigest[:])
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := files[name]
		digest := sha256.Sum256(data)
		manifest.Files = append(manifest.Files, PackageManifestFile{Path: name, SHA256: hex.EncodeToString(digest[:]), Bytes: len(data)})
		if err := os.WriteFile(filepath.Join(outputDirectory, name), data, 0o600); err != nil {
			_ = os.RemoveAll(outputDirectory)
			return PackageManifest{}, errors.New("write assurance package file")
		}
	}
	manifestBytes, err := stableJSON(manifest)
	if err != nil {
		_ = os.RemoveAll(outputDirectory)
		return PackageManifest{}, err
	}
	if err := os.WriteFile(filepath.Join(outputDirectory, "manifest.json"), manifestBytes, 0o600); err != nil {
		_ = os.RemoveAll(outputDirectory)
		return PackageManifest{}, errors.New("write assurance package manifest")
	}
	if manifest.Signed {
		signature, err := signPackageManifest(manifestBytes, signingKey)
		if err != nil {
			_ = os.RemoveAll(outputDirectory)
			return PackageManifest{}, err
		}
		signatureBytes, _ := stableJSON(signature)
		if err := os.WriteFile(filepath.Join(outputDirectory, "signature.json"), signatureBytes, 0o600); err != nil {
			_ = os.RemoveAll(outputDirectory)
			return PackageManifest{}, errors.New("write assurance package signature")
		}
	}
	return manifest, nil
}

func buildPackageFiles(opts PackageOptions) (map[string][]byte, error) {
	files := map[string][]byte{}
	files["profile.json"], _ = stableJSON(opts.Profile)
	files["readiness.json"], _ = stableJSON(opts.Readiness)
	scope := packageScope{Schema: "fairway.assurance-package-scope.v1", ProfileID: opts.Profile.ID, ProfileVersion: opts.Profile.Version,
		FrameworkID: opts.Profile.Framework.ID, FrameworkVersion: opts.Profile.Framework.Version, Project: firstMapProject(opts.Maps), Scope: opts.Readiness.Scope,
		ScopeID: opts.Readiness.ScopeID, TaskIDs: append([]string(nil), opts.Readiness.TaskIDs...), EvaluatedAt: opts.Readiness.EvaluatedAt,
		AuthorityBoundary: opts.Readiness.AuthorityBoundary}
	files["scope.json"], _ = stableJSON(scope)
	files["controls.json"], _ = stableJSON(map[string]any{"schema": "fairway.assurance-control-view.v1", "controls": opts.Readiness.Controls})
	files["controls.md"] = controlMarkdown(opts.Readiness)
	files["controls.csv"] = controlCSV(opts.Readiness)
	files["gaps.json"], _ = stableJSON(map[string]any{"schema": "fairway.assurance-gap-view.v1", "gaps": opts.Readiness.Gaps})

	var allFacts []EvidenceFact
	for _, mapped := range opts.Maps {
		allFacts = append(allFacts, mapped.Facts...)
	}
	sort.Slice(allFacts, func(i, j int) bool { return allFacts[i].Reference < allFacts[j].Reference })
	files["evidence-index.json"], _ = stableJSON(map[string]any{"schema": "fairway.assurance-evidence-index.v1", "facts": allFacts})
	for _, class := range []string{"decision", "review", "provenance", "exception"} {
		files[class+"s.json"], _ = stableJSON(packageReferenceGroup{Schema: "fairway.assurance-" + class + "-references.v1", Facts: factsByClass(allFacts, class)})
	}
	responsibilities := packageResponsibilitiesForProfile(opts.Profile)
	files["responsibilities.json"], _ = stableJSON(responsibilities)
	boundary := oscalBoundaryForReadiness(opts.Readiness)
	files["oscal-control-map.json"], _ = stableJSON(boundary)
	files["VERIFY.md"] = verificationInstructions()
	return files, nil
}

func packageProject(maps []EvidenceMap) (string, error) {
	project := ""
	for _, mapped := range maps {
		value := strings.TrimSpace(mapped.Project)
		if value == "" {
			return "", errors.New("assurance package source project is missing")
		}
		if project == "" {
			project = value
		} else if project != value {
			return "", errors.New("assurance package cannot mix source projects")
		}
		for _, fact := range mapped.Facts {
			if fact.Project != value {
				return "", errors.New("assurance package fact project does not match source project")
			}
		}
	}
	if project == "" {
		return "", errors.New("assurance package requires at least one source project")
	}
	return project, nil
}

func firstMapProject(maps []EvidenceMap) string {
	if len(maps) == 0 {
		return ""
	}
	return maps[0].Project
}

func preparePackageDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return "", errors.New("assurance package output must be a local directory")
	}
	clean, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", errors.New("resolve assurance package output")
	}
	parent := filepath.Dir(clean)
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", errors.New("assurance package parent directory must exist")
	}
	clean = filepath.Join(resolvedParent, filepath.Base(clean))
	if _, err := os.Lstat(clean); !os.IsNotExist(err) {
		return "", errors.New("assurance package output must not already exist")
	}
	if err := os.Mkdir(clean, 0o700); err != nil {
		return "", errors.New("create assurance package output")
	}
	return clean, nil
}

func validatePackageClaims(profile Profile) error {
	values := []string{profile.Title, profile.Description, profile.Framework.Title, profile.Applicability.Description}
	for _, control := range profile.Controls {
		values = append(values, control.Title, control.Objective)
		values = append(values, control.AssessmentObjectives...)
	}
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, marker := range []string{"certified", "is compliant", "compliance achieved", "is authorized", "fips-validated", "fips validated"} {
			if strings.Contains(lower, marker) {
				return errors.New("assurance profile contains a prohibited generated certification or authorization claim")
			}
		}
	}
	return nil
}

func decodePackageSigningKey(encodedKey string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil {
		return nil, errors.New("decode assurance package signing key")
	}
	var privateKey ed25519.PrivateKey
	switch len(raw) {
	case ed25519.SeedSize:
		privateKey = ed25519.NewKeyFromSeed(raw)
	case ed25519.PrivateKeySize:
		privateKey = ed25519.PrivateKey(raw)
	default:
		return nil, errors.New("assurance package signing key must be an Ed25519 seed or private key")
	}
	return privateKey, nil
}

func signPackageManifest(manifest []byte, privateKey ed25519.PrivateKey) (PackageSignature, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return PackageSignature{}, errors.New("assurance package signing key is invalid")
	}
	digest := sha256.Sum256(manifest)
	signature := ed25519.Sign(privateKey, digest[:])
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return PackageSignature{Schema: PackageSignatureSchema, Algorithm: "ed25519-sha256", ManifestSHA256: hex.EncodeToString(digest[:]),
		PublicKey: base64.StdEncoding.EncodeToString(publicKey), Signature: base64.StdEncoding.EncodeToString(signature)}, nil
}

func stableJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func controlMarkdown(report ReadinessReport) []byte {
	var out strings.Builder
	out.WriteString("# Assurance control view\n\n")
	out.WriteString("| Control | Status | Responsibility | Evidence references |\n|---|---|---|---|\n")
	for _, control := range report.Controls {
		fmt.Fprintf(&out, "| %s | %s | %s | %s |\n", markdownCell(control.ControlID), markdownCell(control.Status), markdownCell(control.Responsibility), markdownCell(strings.Join(control.SourceReferences, ", ")))
	}
	out.WriteString("\nThis is recorded-evidence readiness, not a certification or compliance conclusion.\n")
	return []byte(out.String())
}

func controlCSV(report ReadinessReport) []byte {
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	_ = w.Write([]string{"control_id", "status", "responsibility", "source_references", "assessor_boundary"})
	for _, control := range report.Controls {
		_ = w.Write([]string{control.ControlID, control.Status, control.Responsibility, strings.Join(control.SourceReferences, ";"), control.AssessorBoundary})
	}
	w.Flush()
	return out.Bytes()
}

func factReferences(facts []EvidenceFact, class string) []string {
	var refs []string
	for _, fact := range facts {
		if fact.Class == class {
			refs = append(refs, fact.Reference)
		}
	}
	return uniqueSorted(refs)
}

func factsByClass(facts []EvidenceFact, class string) []EvidenceFact {
	var selected []EvidenceFact
	for _, fact := range facts {
		if fact.Class == class {
			selected = append(selected, fact)
		}
	}
	return selected
}

func packageResponsibilitiesForProfile(profile Profile) packageResponsibilities {
	responsibilities := packageResponsibilities{Schema: "fairway.assurance-responsibilities.v1"}
	for _, control := range profile.Controls {
		switch control.Responsibility {
		case "customer":
			responsibilities.CustomerControls = append(responsibilities.CustomerControls, control.ID)
		case "shared":
			responsibilities.SharedControls = append(responsibilities.SharedControls, control.ID)
		}
		if control.ExternalAssessmentRequired {
			responsibilities.ExternalAssessmentControls = append(responsibilities.ExternalAssessmentControls, control.ID)
		}
	}
	return responsibilities
}

func oscalBoundaryForReadiness(readiness ReadinessReport) oscalBoundary {
	boundary := oscalBoundary{Schema: "fairway.oscal-control-map.v1", NotOSCALDocument: true,
		Boundary: "mapping input only; transform and validate with an authoritative OSCAL toolchain before assessor use"}
	for _, control := range readiness.Controls {
		boundary.Controls = append(boundary.Controls, oscalBoundaryControl{ControlID: control.ControlID, FairwayStatus: control.Status,
			EvidenceReferences: control.SourceReferences, AssessorBoundary: control.AssessorBoundary})
	}
	return boundary
}

func verificationInstructions() []byte {
	return []byte("# Verify this assurance package\n\nRun `fairway assurance package verify --dir <package-directory>` in an offline environment. A signed package requires trust pinning for overall success: place the expected base64 Ed25519 public key in an environment variable and add `--trusted-public-key-env <name>`. Verification checks schemas, all manifest file digests, profile identity/digest, scope/task source state, evidence references/freshness, exception linkage, view consistency, claim guards, and the optional signature. Output separates package integrity, recorded-evidence sufficiency, signature trust, and external certification. Verification is read-only and does not write findings to Fairway.\n")
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
