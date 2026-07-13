package assurance

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha1"
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
	PackageManifestSchemaV1 = "fairway.assurance-package-manifest.v1"
	PackageManifestSchema   = "fairway.assurance-package-manifest.v2"
	PackageSignatureSchema  = "fairway.assurance-package-signature.v1"
	OSCALVersion            = "1.1.3"
	oscalPropertyNamespace  = "https://docs.fairway.run/ns/assurance"
)

type PackageOptions struct {
	Profile          Profile
	Readiness        ReadinessReport
	Maps             []EvidenceMap
	OutputDirectory  string
	CreatedAt        time.Time
	ProductVersion   string
	SigningKeyBase64 string
}

type PackageManifest struct {
	Schema            string                `json:"schema"`
	PackageVersion    string                `json:"package_version"`
	CreatedAt         string                `json:"created_at"`
	ProfileID         string                `json:"profile_id"`
	ProfileVersion    string                `json:"profile_version"`
	ProductVersion    string                `json:"product_version,omitempty"`
	ReviewDate        string                `json:"review_date,omitempty"`
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
	ProductVersion    string   `json:"product_version,omitempty"`
	ReviewDate        string   `json:"review_date,omitempty"`
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

type oscalComponentDocument struct {
	ComponentDefinition oscalComponentDefinition `json:"component-definition"`
}

type oscalComponentDefinition struct {
	UUID       string           `json:"uuid"`
	Metadata   oscalMetadata    `json:"metadata"`
	Components []oscalComponent `json:"components"`
}

type oscalMetadata struct {
	Title        string      `json:"title"`
	LastModified string      `json:"last-modified"`
	Version      string      `json:"version"`
	OSCALVersion string      `json:"oscal-version"`
	Props        []oscalProp `json:"props,omitempty"`
}

type oscalComponent struct {
	UUID                   string                       `json:"uuid"`
	Type                   string                       `json:"type"`
	Title                  string                       `json:"title"`
	Description            string                       `json:"description"`
	Props                  []oscalProp                  `json:"props,omitempty"`
	ControlImplementations []oscalControlImplementation `json:"control-implementations"`
}

type oscalControlImplementation struct {
	UUID                    string                        `json:"uuid"`
	Source                  string                        `json:"source"`
	Description             string                        `json:"description"`
	ImplementedRequirements []oscalImplementedRequirement `json:"implemented-requirements"`
}

type oscalImplementedRequirement struct {
	UUID        string      `json:"uuid"`
	ControlID   string      `json:"control-id"`
	Description string      `json:"description"`
	Props       []oscalProp `json:"props,omitempty"`
	Links       []oscalLink `json:"links,omitempty"`
}

type oscalProp struct {
	Name  string `json:"name"`
	NS    string `json:"ns"`
	Value string `json:"value"`
	Group string `json:"group,omitempty"`
}

type oscalLink struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
	Text string `json:"text,omitempty"`
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
	if err := validIdentifier("assurance package product version", opts.ProductVersion, identifierPattern); err != nil {
		return PackageManifest{}, err
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
	manifest := PackageManifest{Schema: PackageManifestSchema, PackageVersion: "v2", CreatedAt: opts.CreatedAt.UTC().Format(time.RFC3339Nano),
		ProfileID: opts.Profile.ID, ProfileVersion: opts.Profile.Version, ProfileSHA256: hex.EncodeToString(profileDigest[:]),
		ProductVersion: opts.ProductVersion, ReviewDate: opts.CreatedAt.UTC().Format(time.DateOnly),
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
	scope := packageScope{Schema: "fairway.assurance-package-scope.v2", ProfileID: opts.Profile.ID, ProfileVersion: opts.Profile.Version,
		FrameworkID: opts.Profile.Framework.ID, FrameworkVersion: opts.Profile.Framework.Version, Project: firstMapProject(opts.Maps), Scope: opts.Readiness.Scope,
		ScopeID: opts.Readiness.ScopeID, TaskIDs: append([]string(nil), opts.Readiness.TaskIDs...), EvaluatedAt: opts.Readiness.EvaluatedAt,
		ProductVersion: opts.ProductVersion, ReviewDate: opts.CreatedAt.UTC().Format(time.DateOnly),
		AuthorityBoundary: opts.Readiness.AuthorityBoundary}
	files["scope.json"], _ = stableJSON(scope)
	files["controls.json"], _ = stableJSON(map[string]any{"schema": "fairway.assurance-control-view.v1", "controls": opts.Readiness.Controls})
	files["controls.md"] = controlMarkdownV2(opts)
	files["controls.csv"] = controlCSVV2(opts)
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
	oscalDocument := oscalComponentForPackage(opts)
	files["oscal-component-definition.json"], _ = stableJSON(oscalDocument)
	files["VERIFY.md"] = verificationInstructionsV2()
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

func controlMarkdownV2(opts PackageOptions) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "# Assurance control view\n\nProfile: `%s@%s`  \nProduct version: `%s`  \nReview date: `%s`\n\n",
		markdownCell(opts.Profile.ID), markdownCell(opts.Profile.Version), markdownCell(opts.ProductVersion), opts.CreatedAt.UTC().Format(time.DateOnly))
	out.WriteString("| Control | Status | Responsibility | Customer responsibility | Evidence references | Assessment objectives |\n|---|---|---|---|---|---|\n")
	controls := make(map[string]Control, len(opts.Profile.Controls))
	for _, control := range opts.Profile.Controls {
		controls[control.ID] = control
	}
	for _, row := range opts.Readiness.Controls {
		definition := controls[row.ControlID]
		customer := "no"
		if definition.Responsibility == "customer" || definition.Responsibility == "shared" {
			customer = "yes"
		}
		fmt.Fprintf(&out, "| %s | %s | %s | %s | %s | %s |\n",
			markdownCell(row.ControlID), markdownCell(row.Status), markdownCell(row.Responsibility), customer,
			markdownCell(strings.Join(row.SourceReferences, ", ")), markdownCell(strings.Join(definition.AssessmentObjectives, "; ")))
	}
	out.WriteString("\nThis is recorded-evidence readiness, not a certification or compliance conclusion.\n")
	return []byte(out.String())
}

func controlCSVV2(opts PackageOptions) []byte {
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	_ = w.Write([]string{"profile_id", "profile_version", "product_version", "review_date", "control_id", "status", "responsibility", "customer_responsibility", "source_references", "assessment_objectives", "assessor_boundary"})
	controls := make(map[string]Control, len(opts.Profile.Controls))
	for _, control := range opts.Profile.Controls {
		controls[control.ID] = control
	}
	for _, row := range opts.Readiness.Controls {
		definition := controls[row.ControlID]
		customer := "false"
		if definition.Responsibility == "customer" || definition.Responsibility == "shared" {
			customer = "true"
		}
		_ = w.Write([]string{opts.Profile.ID, opts.Profile.Version, opts.ProductVersion, opts.CreatedAt.UTC().Format(time.DateOnly), row.ControlID, row.Status, row.Responsibility, customer, strings.Join(row.SourceReferences, ";"), strings.Join(definition.AssessmentObjectives, ";"), row.AssessorBoundary})
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

func oscalComponentForPackage(opts PackageOptions) oscalComponentDocument {
	created := opts.CreatedAt.UTC().Format(time.RFC3339Nano)
	reviewDate := opts.CreatedAt.UTC().Format(time.DateOnly)
	rootMaterial := strings.Join([]string{opts.Profile.ID, opts.Profile.Version, opts.ProductVersion, opts.Readiness.Scope, opts.Readiness.ScopeID, created}, "|")
	component := oscalComponent{
		UUID:        deterministicUUID(rootMaterial + "|component"),
		Type:        "software",
		Title:       "Fairway " + opts.ProductVersion,
		Description: "Fairway engineering evidence and control-record component for assessor preparation. This description does not declare control satisfaction, certification, compliance, authorization, or risk acceptance.",
		Props: []oscalProp{
			oscalProperty("product-version", opts.ProductVersion),
			oscalProperty("profile-id", opts.Profile.ID),
			oscalProperty("profile-version", opts.Profile.Version),
			oscalProperty("review-date", reviewDate),
		},
	}
	implementation := oscalControlImplementation{
		UUID:        deterministicUUID(rootMaterial + "|control-implementation"),
		Source:      opts.Profile.Framework.Source,
		Description: "Evidence-preparation statements for " + opts.Profile.Framework.Title + " " + opts.Profile.Framework.Version + ". An authorized assessor determines applicability and outcomes.",
	}
	readinessByID := make(map[string]ControlReadiness, len(opts.Readiness.Controls))
	for _, row := range opts.Readiness.Controls {
		readinessByID[row.ControlID] = row
	}
	for _, control := range opts.Profile.Controls {
		row := readinessByID[control.ID]
		requirement := oscalImplementedRequirement{
			UUID:        deterministicUUID(rootMaterial + "|control|" + control.ID),
			ControlID:   control.ID,
			Description: control.Objective + " Fairway readiness status: " + row.Status + ". " + row.Rationale,
			Props: []oscalProp{
				oscalProperty("implementation-status", row.Status),
				oscalProperty("responsibility", control.Responsibility),
				oscalProperty("profile-id", opts.Profile.ID),
				oscalProperty("profile-version", opts.Profile.Version),
				oscalProperty("product-version", opts.ProductVersion),
				oscalProperty("review-date", reviewDate),
				oscalProperty("assessor-boundary", row.AssessorBoundary),
			},
		}
		for index, objective := range control.AssessmentObjectives {
			prop := oscalProperty("assessment-objective", objective)
			prop.Group = fmt.Sprintf("objective-%d", index+1)
			requirement.Props = append(requirement.Props, prop)
		}
		for _, reference := range row.SourceReferences {
			requirement.Links = append(requirement.Links, oscalLink{Href: "evidence-index.json", Rel: "evidence-reference", Text: reference})
		}
		implementation.ImplementedRequirements = append(implementation.ImplementedRequirements, requirement)
	}
	component.ControlImplementations = []oscalControlImplementation{implementation}
	return oscalComponentDocument{ComponentDefinition: oscalComponentDefinition{
		UUID: deterministicUUID(rootMaterial + "|document"),
		Metadata: oscalMetadata{
			Title:        "Fairway assessor input for " + opts.Profile.Title,
			LastModified: created,
			Version:      opts.Profile.Version + "+" + opts.ProductVersion,
			OSCALVersion: OSCALVersion,
			Props: []oscalProp{
				oscalProperty("profile-id", opts.Profile.ID),
				oscalProperty("profile-version", opts.Profile.Version),
				oscalProperty("product-version", opts.ProductVersion),
				oscalProperty("review-date", reviewDate),
				oscalProperty("authority-boundary", "assessment preparation only; no certification, compliance, authorization, approval, or risk acceptance"),
			},
		},
		Components: []oscalComponent{component},
	}}
}

func oscalProperty(name, value string) oscalProp {
	return oscalProp{Name: name, NS: oscalPropertyNamespace, Value: value}
}

func deterministicUUID(material string) string {
	// UUIDv5 keeps deterministic package bytes while satisfying OSCAL's UUID
	// lexical contract. SHA-1 is used only by the UUID identifier algorithm,
	// never for integrity or signature decisions.
	namespaceURL := []byte{0x6b, 0xa7, 0xb8, 0x11, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}
	hash := sha1.New()
	_, _ = hash.Write(namespaceURL)
	_, _ = hash.Write([]byte(material))
	raw := hash.Sum(nil)[:16]
	raw[6] = (raw[6] & 0x0f) | 0x50
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw)
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}

func verificationInstructionsV1() []byte {
	return []byte("# Verify this assurance package\n\nRun `fairway assurance package verify --dir <package-directory>` in an offline environment. A signed package requires trust pinning for overall success: place the expected base64 Ed25519 public key in an environment variable and add `--trusted-public-key-env <name>`. Verification checks schemas, all manifest file digests, profile identity/digest, scope/task source state, evidence references/freshness, exception linkage, view consistency, claim guards, and the optional signature. Output separates package integrity, recorded-evidence sufficiency, signature trust, and external certification. Verification is read-only and does not write findings to Fairway.\n")
}

func verificationInstructionsV2() []byte {
	return []byte("# Verify this assurance package\n\nRun `fairway assurance package verify --dir <package-directory>` in an offline environment. A signed package requires trust pinning for overall success: place the expected base64 Ed25519 public key in an environment variable and add `--trusted-public-key-env <name>`. Verification checks schemas, all manifest file digests, product/profile/scope identity, evidence references and freshness, exception linkage, fixed human-readable views, the deterministic OSCAL component definition, documentation claim guards, and the optional signature. OSCAL compatibility does not replace validation by the assessor's selected authoritative OSCAL toolchain. Output separates package integrity, recorded-evidence sufficiency, signature trust, and external certification. Verification is read-only and does not write findings to Fairway.\n")
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
