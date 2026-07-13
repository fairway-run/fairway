package provenance

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ReleaseBundleManifestSchema           = "fairway.release-assurance-manifest.v1"
	ReleaseBundleSignatureSchema          = "fairway.release-assurance-signature.v1"
	ReleaseBundleArtifactSignaturesSchema = "fairway.release-artifact-signatures.v1"
	ReleaseBundleVerificationSchema       = "fairway.release-assurance-verification.v1"
)

var RequiredReleaseEvidence = []string{"sbom", "vex", "dependencies", "licenses", "license_disposition", "source_provenance", "build_provenance", "build_recipe", "test_summary", "vulnerability_disposition"}

type ReleaseBundleOptions struct {
	OutputDirectory  string
	Version          string
	SourceSHA        string
	BuilderID        string
	PolicyVersion    string
	CreatedAt        time.Time
	SigningKeyBase64 string
	Artifacts        map[string]string
	Evidence         map[string]string
	SLSA             ReleaseSLSAProperties
}

type ReleaseSLSAProperties struct {
	Specification                 string `json:"specification"`
	SourceVersioned               bool   `json:"source_versioned"`
	BuildServiceGenerated         bool   `json:"build_service_generated"`
	ProvenanceAvailable           bool   `json:"provenance_available"`
	BuilderIdentityRecorded       bool   `json:"builder_identity_recorded"`
	BuildRecipeRecorded           bool   `json:"build_recipe_recorded"`
	DependenciesRecorded          bool   `json:"dependencies_recorded"`
	HermeticBuildDemonstrated     bool   `json:"hermetic_build_demonstrated"`
	ReproducibleBuildDemonstrated bool   `json:"reproducible_build_demonstrated"`
	LevelClaimed                  bool   `json:"level_claimed"`
}

type ReleaseBundleManifest struct {
	Schema            string                  `json:"schema"`
	BundleVersion     string                  `json:"bundle_version"`
	CreatedAt         string                  `json:"created_at"`
	Version           string                  `json:"version"`
	SourceSHA         string                  `json:"source_sha"`
	BuilderID         string                  `json:"builder_id"`
	PolicyVersion     string                  `json:"policy_version"`
	SigningKeyID      string                  `json:"signing_key_id"`
	Artifacts         []ReleaseBundleFile     `json:"artifacts"`
	Evidence          []ReleaseBundleEvidence `json:"evidence"`
	Files             []ReleaseBundleFile     `json:"files"`
	SLSA              ReleaseSLSAProperties   `json:"slsa_properties"`
	NonClaims         []string                `json:"non_claims"`
	AuthorityBoundary string                  `json:"authority_boundary"`
}

type ReleaseBundleFile struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ReleaseBundleEvidence struct {
	Class  string `json:"class"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ReleaseArtifactSignatures struct {
	Schema     string                     `json:"schema"`
	Algorithm  string                     `json:"algorithm"`
	KeyID      string                     `json:"key_id"`
	Signatures []ReleaseArtifactSignature `json:"signatures"`
}

type ReleaseArtifactSignature struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}

type ReleaseBundleSignature struct {
	Schema         string `json:"schema"`
	Algorithm      string `json:"algorithm"`
	ManifestSHA256 string `json:"manifest_sha256"`
	PublicKey      string `json:"public_key"`
	Signature      string `json:"signature"`
}

type ReleaseBundleVerifyOptions struct {
	Directory              string
	TrustedPublicKeyBase64 string
	ExpectedVersion        string
	ExpectedSourceSHA      string
	ExpectedBuilderID      string
	ExpectedPolicyVersion  string
}

type ReleaseBundleVerification struct {
	Schema                  string                `json:"schema"`
	OK                      bool                  `json:"ok"`
	Version                 string                `json:"version,omitempty"`
	SourceSHA               string                `json:"source_sha,omitempty"`
	BuilderID               string                `json:"builder_id,omitempty"`
	PolicyVersion           string                `json:"policy_version,omitempty"`
	SignatureStatus         string                `json:"signature_status"`
	ArtifactSignatureStatus string                `json:"artifact_signature_status"`
	CompletenessStatus      string                `json:"completeness_status"`
	SLSA                    ReleaseSLSAProperties `json:"slsa_properties"`
	Issues                  []string              `json:"issues,omitempty"`
	NonClaims               []string              `json:"non_claims"`
	AuthorityBoundary       string                `json:"authority_boundary"`
}

func ExportReleaseBundle(opts ReleaseBundleOptions) (ReleaseBundleManifest, error) {
	if opts.CreatedAt.IsZero() {
		return ReleaseBundleManifest{}, errors.New("release bundle creation time is required")
	}
	for label, value := range map[string]string{"version": opts.Version, "source sha": opts.SourceSHA, "builder id": opts.BuilderID, "policy version": opts.PolicyVersion} {
		if err := validateReleaseMetadata(label, value); err != nil {
			return ReleaseBundleManifest{}, err
		}
	}
	if !validReleaseSourceSHA(opts.SourceSHA) {
		return ReleaseBundleManifest{}, errors.New("release bundle source sha must be a 40- or 64-character hexadecimal revision")
	}
	if len(opts.Artifacts) == 0 {
		return ReleaseBundleManifest{}, errors.New("release bundle requires at least one artifact")
	}
	for _, class := range RequiredReleaseEvidence {
		if strings.TrimSpace(opts.Evidence[class]) == "" {
			return ReleaseBundleManifest{}, fmt.Errorf("release bundle missing required evidence class %s", class)
		}
	}
	if opts.SLSA.LevelClaimed {
		return ReleaseBundleManifest{}, errors.New("release bundle must not claim a SLSA level")
	}
	if opts.SLSA.Specification == "" {
		opts.SLSA.Specification = "https://slsa.dev/spec/v1.2"
	}
	privateKey, err := decodeReleaseSigningKey(opts.SigningKeyBase64)
	if err != nil {
		return ReleaseBundleManifest{}, err
	}
	dir, err := prepareReleaseBundleDirectory(opts.OutputDirectory)
	if err != nil {
		return ReleaseBundleManifest{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := os.Mkdir(filepath.Join(dir, "artifacts"), 0o700); err != nil {
		return ReleaseBundleManifest{}, errors.New("create release artifacts directory")
	}
	if err := os.Mkdir(filepath.Join(dir, "evidence"), 0o700); err != nil {
		return ReleaseBundleManifest{}, errors.New("create release evidence directory")
	}
	manifest := ReleaseBundleManifest{Schema: ReleaseBundleManifestSchema, BundleVersion: "v1", CreatedAt: opts.CreatedAt.UTC().Format(time.RFC3339Nano), Version: opts.Version,
		SourceSHA: opts.SourceSHA, BuilderID: opts.BuilderID, PolicyVersion: opts.PolicyVersion, SLSA: opts.SLSA,
		NonClaims:         []string{"no_slsa_level_claim", "no_reproducibility_claim_without_measured_proof", "no_dependency_trust_claim", "no_certification_or_compliance_claim"},
		AuthorityBoundary: "release assurance evidence only; not certification, compliance, artifact authorization, deployment approval, or risk acceptance"}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	keyDigest := sha256.Sum256(publicKey)
	manifest.SigningKeyID = "sha256:" + hex.EncodeToString(keyDigest[:])
	artifactNames := sortedReleaseKeys(opts.Artifacts)
	var checksumLines []string
	artifactSignatures := ReleaseArtifactSignatures{Schema: ReleaseBundleArtifactSignaturesSchema, Algorithm: "ed25519-sha256", KeyID: manifest.SigningKeyID}
	for _, name := range artifactNames {
		if !safeReleaseName(name) {
			return ReleaseBundleManifest{}, errors.New("release artifact name is unsafe")
		}
		data, err := readReleaseInput(opts.Artifacts[name])
		if err != nil {
			return ReleaseBundleManifest{}, fmt.Errorf("read release artifact %s", name)
		}
		path := filepath.ToSlash(filepath.Join("artifacts", name))
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), data, 0o600); err != nil {
			return ReleaseBundleManifest{}, errors.New("write release artifact")
		}
		digest := sha256.Sum256(data)
		digestHex := hex.EncodeToString(digest[:])
		entry := ReleaseBundleFile{Name: name, Path: path, SHA256: digestHex, Bytes: int64(len(data))}
		manifest.Artifacts = append(manifest.Artifacts, entry)
		manifest.Files = append(manifest.Files, entry)
		checksumLines = append(checksumLines, digestHex+"  "+path)
		artifactSignatures.Signatures = append(artifactSignatures.Signatures, ReleaseArtifactSignature{Path: path, SHA256: digestHex, Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, digest[:]))})
	}
	for _, class := range RequiredReleaseEvidence {
		data, err := readReleaseInput(opts.Evidence[class])
		if err != nil {
			return ReleaseBundleManifest{}, fmt.Errorf("read release evidence %s", class)
		}
		if containsReleasePrivateContent(data) {
			return ReleaseBundleManifest{}, fmt.Errorf("release evidence %s contains prohibited private content", class)
		}
		if err := validateReleaseEvidence(class, data, manifest); err != nil {
			return ReleaseBundleManifest{}, err
		}
		ext := filepath.Ext(opts.Evidence[class])
		if ext == "" || len(ext) > 10 {
			ext = ".json"
		}
		path := filepath.ToSlash(filepath.Join("evidence", class+ext))
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), data, 0o600); err != nil {
			return ReleaseBundleManifest{}, errors.New("write release evidence")
		}
		digest := sha256.Sum256(data)
		entry := ReleaseBundleEvidence{Class: class, Path: path, SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(data))}
		manifest.Evidence = append(manifest.Evidence, entry)
		manifest.Files = append(manifest.Files, ReleaseBundleFile{Name: class, Path: path, SHA256: entry.SHA256, Bytes: entry.Bytes})
	}
	checksums := []byte(strings.Join(checksumLines, "\n") + "\n")
	if err := writeReleaseGeneratedFile(dir, &manifest, "checksums", "checksums.txt", checksums); err != nil {
		return ReleaseBundleManifest{}, err
	}
	signatureBytes, _ := stableReleaseJSON(artifactSignatures)
	if err := writeReleaseGeneratedFile(dir, &manifest, "artifact_signatures", "artifact-signatures.json", signatureBytes); err != nil {
		return ReleaseBundleManifest{}, err
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	if err := validateReleaseManifest(manifest); err != nil {
		return ReleaseBundleManifest{}, err
	}
	manifestBytes, _ := stableReleaseJSON(manifest)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestBytes, 0o600); err != nil {
		return ReleaseBundleManifest{}, errors.New("write release manifest")
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	signature := ReleaseBundleSignature{Schema: ReleaseBundleSignatureSchema, Algorithm: "ed25519-sha256", ManifestSHA256: hex.EncodeToString(manifestDigest[:]), PublicKey: base64.StdEncoding.EncodeToString(publicKey), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, manifestDigest[:]))}
	data, _ := stableReleaseJSON(signature)
	if err := os.WriteFile(filepath.Join(dir, "signature.json"), data, 0o600); err != nil {
		return ReleaseBundleManifest{}, errors.New("write release bundle signature")
	}
	cleanup = false
	return manifest, nil
}

func VerifyReleaseBundle(opts ReleaseBundleVerifyOptions) (ReleaseBundleVerification, error) {
	report := ReleaseBundleVerification{Schema: ReleaseBundleVerificationSchema, SignatureStatus: "invalid", ArtifactSignatureStatus: "invalid", CompletenessStatus: "missing",
		NonClaims:         []string{"no_slsa_level_claim", "no_reproducibility_claim_without_measured_proof", "no_dependency_trust_claim", "no_certification_or_compliance_claim"},
		AuthorityBoundary: "offline release evidence verification only; not certification, compliance, deployment authorization, or risk acceptance"}
	dir, err := resolveReleaseBundleDirectory(opts.Directory)
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, nil
	}
	manifestBytes, err := readReleaseBundleFile(dir, "manifest.json")
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, nil
	}
	var manifest ReleaseBundleManifest
	if strictReleaseJSON(manifestBytes, &manifest) != nil || validateReleaseManifest(manifest) != nil {
		report.Issues = append(report.Issues, "release assurance manifest is invalid")
		return report, nil
	}
	report.Version = manifest.Version
	report.SourceSHA = manifest.SourceSHA
	report.BuilderID = manifest.BuilderID
	report.PolicyVersion = manifest.PolicyVersion
	report.SLSA = manifest.SLSA
	for _, expected := range [][3]string{{"version", manifest.Version, opts.ExpectedVersion}, {"source sha", manifest.SourceSHA, opts.ExpectedSourceSHA}, {"builder id", manifest.BuilderID, opts.ExpectedBuilderID}, {"policy version", manifest.PolicyVersion, opts.ExpectedPolicyVersion}} {
		if strings.TrimSpace(expected[2]) == "" || expected[1] != expected[2] {
			report.Issues = append(report.Issues, fmt.Sprintf("release %s does not match expected value", expected[0]))
		}
	}
	trusted, err := base64.StdEncoding.DecodeString(strings.TrimSpace(opts.TrustedPublicKeyBase64))
	if err != nil || len(trusted) != ed25519.PublicKeySize {
		report.Issues = append(report.Issues, "trusted release public key is invalid")
		return report, nil
	}
	if err := verifyReleaseManifestSignature(dir, manifest, manifestBytes, trusted); err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, nil
	}
	report.SignatureStatus = "verified_pinned"
	expectedFiles := map[string]bool{"manifest.json": true, "signature.json": true}
	for _, file := range manifest.Files {
		expectedFiles[file.Path] = true
		data, err := readReleaseBundleFile(dir, file.Path)
		if err != nil {
			report.Issues = append(report.Issues, err.Error())
			continue
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != file.SHA256 || int64(len(data)) != file.Bytes {
			report.Issues = append(report.Issues, "release bundle file digest or size mismatch: "+file.Path)
		}
	}
	if err := rejectUnknownReleaseFiles(dir, expectedFiles); err != nil {
		report.Issues = append(report.Issues, err.Error())
	}
	if err := verifyArtifactChecksumsAndSignatures(dir, manifest, trusted); err != nil {
		report.Issues = append(report.Issues, err.Error())
	} else {
		report.ArtifactSignatureStatus = "verified_pinned"
	}
	classes := map[string]bool{}
	for _, evidence := range manifest.Evidence {
		classes[evidence.Class] = true
		data, err := readReleaseBundleFile(dir, evidence.Path)
		if err != nil {
			report.Issues = append(report.Issues, err.Error())
			continue
		}
		if err := validateReleaseEvidence(evidence.Class, data, manifest); err != nil {
			report.Issues = append(report.Issues, err.Error())
		}
	}
	complete := true
	for _, class := range RequiredReleaseEvidence {
		if !classes[class] {
			complete = false
			report.Issues = append(report.Issues, "missing release evidence class "+class)
		}
	}
	if complete {
		report.CompletenessStatus = "complete"
	}
	report.OK = len(report.Issues) == 0 && report.SignatureStatus == "verified_pinned" && report.ArtifactSignatureStatus == "verified_pinned" && report.CompletenessStatus == "complete"
	return report, nil
}

func writeReleaseGeneratedFile(dir string, manifest *ReleaseBundleManifest, name, path string, data []byte) error {
	if err := os.WriteFile(filepath.Join(dir, path), data, 0o600); err != nil {
		return errors.New("write generated release evidence")
	}
	d := sha256.Sum256(data)
	manifest.Files = append(manifest.Files, ReleaseBundleFile{Name: name, Path: path, SHA256: hex.EncodeToString(d[:]), Bytes: int64(len(data))})
	return nil
}
func sortedReleaseKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func safeReleaseName(value string) bool {
	return value != "" && value == filepath.Base(value) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\:\r\n") && !containsSensitiveReleaseName(value)
}
func containsSensitiveReleaseName(value string) bool {
	lower := strings.ToLower(value)
	for _, m := range []string{"secret", "token", "password", "credential", "private-key", "private_key", "apikey", "api_key"} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
func containsReleasePrivateContent(data []byte) bool {
	lower := strings.ToLower(string(data))
	for _, marker := range []string{"-----begin private key", "authorization: bearer", `"authorization":"bearer`, `"access_token":"`, `"refresh_token":"`, `"client_secret":"`, `"password":"`, `"api_key":"`, "raw_prompt", "raw prompt", "transcript:", "tool_body", "tool body:", "generated_content"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
func validReleaseSHA(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == sha256.Size
}
func validReleaseSourceSHA(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && (len(raw) == 20 || len(raw) == 32)
}
func validReleaseBundlePath(value string) bool {
	clean := filepath.ToSlash(filepath.Clean(value))
	return value == clean && clean != "." && !filepath.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, "../") && (strings.HasPrefix(clean, "artifacts/") || strings.HasPrefix(clean, "evidence/") || clean == "checksums.txt" || clean == "artifact-signatures.json")
}
func validateReleaseEvidence(class string, data []byte, manifest ReleaseBundleManifest) error {
	switch class {
	case "sbom":
		var doc struct {
			SPDXVersion string            `json:"spdxVersion"`
			SPDXID      string            `json:"SPDXID"`
			Packages    []json.RawMessage `json:"packages"`
		}
		if json.Unmarshal(data, &doc) != nil || !strings.HasPrefix(doc.SPDXVersion, "SPDX-") || doc.SPDXID == "" || len(doc.Packages) == 0 {
			return errors.New("release SBOM is invalid or empty")
		}
	case "vex":
		var doc struct {
			Context    string            `json:"@context"`
			ID         string            `json:"@id"`
			Author     string            `json:"author"`
			Timestamp  string            `json:"timestamp"`
			Version    int               `json:"version"`
			Statements []json.RawMessage `json:"statements"`
		}
		if strictReleaseJSON(data, &doc) != nil || doc.Context != "https://openvex.dev/ns/v0.2.0" || doc.ID == "" || doc.Author == "" || doc.Version <= 0 || doc.Statements == nil {
			return errors.New("release VEX document is invalid")
		}
		if _, err := time.Parse(time.RFC3339, doc.Timestamp); err != nil {
			return errors.New("release VEX timestamp is invalid")
		}
	case "dependencies":
		decoder := json.NewDecoder(bytes.NewReader(data))
		count := 0
		for {
			var row struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			}
			err := decoder.Decode(&row)
			if err == io.EOF {
				break
			}
			if err != nil || row.Path == "" {
				return errors.New("release dependency inventory is invalid")
			}
			count++
		}
		if count == 0 {
			return errors.New("release dependency inventory is empty")
		}
	case "licenses":
		rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
		if err != nil || len(rows) == 0 {
			return errors.New("release license inventory is invalid or empty")
		}
		for _, row := range rows {
			if len(row) != 3 || row[0] == "" || row[1] == "" || row[2] == "" || row[2] == "Unknown" {
				return errors.New("release license inventory contains an unresolved row")
			}
		}
	case "license_disposition":
		var doc struct {
			Schema    string            `json:"schema"`
			Overrides []json.RawMessage `json:"overrides"`
		}
		if strictReleaseJSON(data, &doc) != nil || doc.Schema != "fairway.release-license-overrides.v1" || len(doc.Overrides) == 0 {
			return errors.New("release license disposition is invalid")
		}
	case "source_provenance":
		var doc struct {
			Schema     string `json:"schema"`
			Version    string `json:"version"`
			SourceSHA  string `json:"source_sha"`
			Repository string `json:"repository"`
			Ref        string `json:"ref"`
		}
		if strictReleaseJSON(data, &doc) != nil || doc.Schema != "fairway.release-source-provenance.v1" || doc.Version != manifest.Version || doc.SourceSHA != manifest.SourceSHA || doc.Repository == "" || doc.Ref == "" {
			return errors.New("release source provenance does not match the manifest")
		}
	case "build_provenance":
		var doc struct {
			Schema            string `json:"schema"`
			BuilderID         string `json:"builder_id"`
			RunID             string `json:"run_id"`
			RunAttempt        string `json:"run_attempt"`
			RunnerOS          string `json:"runner_os"`
			RunnerArch        string `json:"runner_arch"`
			GoVersion         string `json:"go_version"`
			GoReleaserVersion string `json:"goreleaser_version"`
			CreatedAt         string `json:"created_at"`
		}
		if strictReleaseJSON(data, &doc) != nil || doc.Schema != "fairway.release-build-provenance.v1" || doc.BuilderID != manifest.BuilderID || doc.RunID == "" || doc.RunnerOS == "" || doc.GoVersion == "" || doc.GoReleaserVersion == "" {
			return errors.New("release build provenance does not match the manifest")
		}
		if _, err := time.Parse(time.RFC3339, doc.CreatedAt); err != nil {
			return errors.New("release build provenance timestamp is invalid")
		}
	case "build_recipe":
		var doc struct {
			Schema string `json:"schema"`
			Source string `json:"source"`
			SHA256 string `json:"sha256"`
		}
		if strictReleaseJSON(data, &doc) != nil || doc.Schema != "fairway.release-build-recipe.v1" || doc.Source == "" || !validReleaseSHA(doc.SHA256) {
			return errors.New("release build recipe is invalid")
		}
	case "test_summary":
		var doc struct {
			Schema    string   `json:"schema"`
			CreatedAt string   `json:"created_at"`
			Commands  []string `json:"commands"`
			Result    string   `json:"result"`
		}
		if strictReleaseJSON(data, &doc) != nil || doc.Schema != "fairway.release-test-summary.v1" || doc.Result != "pass" || len(doc.Commands) == 0 {
			return errors.New("release test summary is not a passing result")
		}
	case "vulnerability_disposition":
		var doc struct {
			Schema            string `json:"schema"`
			Scanner           string `json:"scanner"`
			Result            string `json:"result"`
			Report            string `json:"report"`
			AuthorityBoundary string `json:"authority_boundary"`
		}
		if strictReleaseJSON(data, &doc) != nil || doc.Schema != "fairway.release-vulnerability-disposition.v1" || doc.Scanner == "" || (doc.Result != "no_findings" && doc.Result != "reviewed") || doc.Report == "" || doc.AuthorityBoundary == "" {
			return errors.New("release vulnerability disposition is invalid")
		}
	default:
		return errors.New("unsupported release evidence class")
	}
	return nil
}
func validateReleaseMetadata(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("release bundle %s is required", label)
	}
	if len(value) > 512 || strings.ContainsAny(value, "\r\n") || containsSensitiveReleaseName(value) {
		return fmt.Errorf("release bundle %s contains invalid or sensitive metadata", label)
	}
	return nil
}
func readReleaseInput(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return nil, errors.New("release input must be a local file")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 256<<20 {
		return nil, errors.New("release input is missing, unsafe, or too large")
	}
	return os.ReadFile(path)
}
func decodeReleaseSigningKey(value string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, errors.New("decode release signing key")
	}
	if len(raw) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(raw), nil
	}
	if len(raw) == ed25519.PrivateKeySize {
		return ed25519.PrivateKey(raw), nil
	}
	return nil, errors.New("release signing key must be an Ed25519 seed or private key")
}
func prepareReleaseBundleDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return "", errors.New("release bundle output must be local")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", errors.New("resolve release bundle output")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", errors.New("release bundle parent must exist")
	}
	abs = filepath.Join(parent, filepath.Base(abs))
	if _, err := os.Lstat(abs); !os.IsNotExist(err) {
		return "", errors.New("release bundle output must not already exist")
	}
	if err := os.Mkdir(abs, 0o700); err != nil {
		return "", errors.New("create release bundle output")
	}
	return abs, nil
}
func resolveReleaseBundleDirectory(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("release bundle must be a non-symlink local directory")
	}
	return filepath.EvalSymlinks(path)
}
func readReleaseBundleFile(dir, name string) ([]byte, error) {
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return nil, errors.New("unsafe release bundle path")
	}
	path := filepath.Join(dir, filepath.FromSlash(clean))
	current := dir
	for _, part := range strings.Split(clean, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("release bundle path is missing or contains a symlink: " + clean)
		}
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 256<<20 {
		return nil, errors.New("release bundle file is missing or unsafe: " + clean)
	}
	return os.ReadFile(path)
}
func strictReleaseJSON(data []byte, target any) error {
	if err := rejectDuplicateReleaseObjectKeys(data); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("JSON must contain one value")
	}
	return nil
}

func rejectDuplicateReleaseObjectKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := walkReleaseJSONValue(decoder, true); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing JSON content")
	}
	return nil
}
func walkReleaseJSONValue(decoder *json.Decoder, requireObject bool) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		if requireObject {
			return errors.New("JSON value must be an object")
		}
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			raw, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := raw.(string)
			if !ok || seen[key] {
				return errors.New("JSON object contains a duplicate or invalid key")
			}
			seen[key] = true
			if err := walkReleaseJSONValue(decoder, false); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("JSON object is malformed")
		}
	case '[':
		if requireObject {
			return errors.New("JSON value must be an object")
		}
		for decoder.More() {
			if err := walkReleaseJSONValue(decoder, false); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("JSON array is malformed")
		}
	default:
		return errors.New("JSON delimiter is invalid")
	}
	return nil
}
func stableReleaseJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
func validateReleaseManifest(m ReleaseBundleManifest) error {
	if m.Schema != ReleaseBundleManifestSchema || m.BundleVersion != "v1" || !strings.HasPrefix(m.SigningKeyID, "sha256:") || !validReleaseSHA(strings.TrimPrefix(m.SigningKeyID, "sha256:")) || m.SLSA.LevelClaimed || m.SLSA.Specification != "https://slsa.dev/spec/v1.2" || len(m.Artifacts) == 0 || len(m.Files) == 0 || m.AuthorityBoundary != "release assurance evidence only; not certification, compliance, artifact authorization, deployment approval, or risk acceptance" {
		return errors.New("invalid release assurance manifest")
	}
	for label, value := range map[string]string{"version": m.Version, "source sha": m.SourceSHA, "builder id": m.BuilderID, "policy version": m.PolicyVersion} {
		if validateReleaseMetadata(label, value) != nil {
			return errors.New("invalid release assurance manifest metadata")
		}
	}
	if !validReleaseSourceSHA(m.SourceSHA) {
		return errors.New("release assurance manifest source sha is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, m.CreatedAt); err != nil {
		return err
	}
	requiredNonClaims := map[string]bool{"no_slsa_level_claim": true, "no_reproducibility_claim_without_measured_proof": true, "no_dependency_trust_claim": true, "no_certification_or_compliance_claim": true}
	for _, claim := range m.NonClaims {
		delete(requiredNonClaims, claim)
	}
	if len(requiredNonClaims) > 0 {
		return errors.New("release assurance manifest is missing required non-claims")
	}
	seen := map[string]bool{}
	for _, file := range m.Files {
		if !validReleaseBundlePath(file.Path) || seen[file.Path] || !validReleaseSHA(file.SHA256) || file.Bytes <= 0 {
			return errors.New("release assurance manifest contains invalid or duplicate files")
		}
		seen[file.Path] = true
	}
	classes := map[string]bool{}
	for _, e := range m.Evidence {
		if classes[e.Class] || !seen[e.Path] || !validReleaseSHA(e.SHA256) || e.Bytes <= 0 {
			return errors.New("release assurance manifest contains invalid evidence")
		}
		classes[e.Class] = true
	}
	for _, class := range RequiredReleaseEvidence {
		if !classes[class] {
			return errors.New("release assurance manifest is incomplete")
		}
	}
	for _, a := range m.Artifacts {
		if !safeReleaseName(a.Name) || !seen[a.Path] || a.Path != "artifacts/"+a.Name || !validReleaseSHA(a.SHA256) || a.Bytes <= 0 {
			return errors.New("release assurance manifest contains invalid artifacts")
		}
	}
	return nil
}
func verifyReleaseManifestSignature(dir string, m ReleaseBundleManifest, manifest []byte, trusted []byte) error {
	data, err := readReleaseBundleFile(dir, "signature.json")
	if err != nil {
		return err
	}
	var sig ReleaseBundleSignature
	if strictReleaseJSON(data, &sig) != nil || sig.Schema != ReleaseBundleSignatureSchema || sig.Algorithm != "ed25519-sha256" {
		return errors.New("release bundle signature metadata is invalid")
	}
	digest := sha256.Sum256(manifest)
	if subtle.ConstantTimeCompare([]byte(sig.ManifestSHA256), []byte(hex.EncodeToString(digest[:]))) != 1 {
		return errors.New("release manifest digest mismatch")
	}
	pub, err := base64.StdEncoding.DecodeString(sig.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize || subtle.ConstantTimeCompare(pub, trusted) != 1 {
		return errors.New("release signing key does not match pinned public key")
	}
	kd := sha256.Sum256(pub)
	if m.SigningKeyID != "sha256:"+hex.EncodeToString(kd[:]) {
		return errors.New("release signing key identity mismatch")
	}
	raw, err := base64.StdEncoding.DecodeString(sig.Signature)
	if err != nil || !ed25519.Verify(pub, digest[:], raw) {
		return errors.New("release manifest signature verification failed")
	}
	return nil
}
func verifyArtifactChecksumsAndSignatures(dir string, m ReleaseBundleManifest, trusted []byte) error {
	data, err := readReleaseBundleFile(dir, "artifact-signatures.json")
	if err != nil {
		return err
	}
	var set ReleaseArtifactSignatures
	if strictReleaseJSON(data, &set) != nil || set.Schema != ReleaseBundleArtifactSignaturesSchema || set.Algorithm != "ed25519-sha256" || set.KeyID != m.SigningKeyID {
		return errors.New("artifact signature metadata is invalid")
	}
	byPath := map[string]ReleaseArtifactSignature{}
	for _, sig := range set.Signatures {
		if byPath[sig.Path].Path != "" {
			return errors.New("duplicate artifact signature")
		}
		byPath[sig.Path] = sig
	}
	if len(byPath) != len(m.Artifacts) {
		return errors.New("artifact signature set contains missing or extra entries")
	}
	checksums, err := readReleaseBundleFile(dir, "checksums.txt")
	if err != nil {
		return err
	}
	expectedChecksums := map[string]bool{}
	for _, artifact := range m.Artifacts {
		expectedChecksums[artifact.SHA256+"  "+artifact.Path] = true
	}
	lines := strings.Split(strings.TrimSpace(string(checksums)), "\n")
	if len(lines) != len(expectedChecksums) {
		return errors.New("artifact checksum set contains missing or extra entries")
	}
	for _, line := range lines {
		if !expectedChecksums[line] {
			return errors.New("artifact checksum entry is unknown")
		}
	}
	for _, artifact := range m.Artifacts {
		sig, ok := byPath[artifact.Path]
		if !ok || sig.SHA256 != artifact.SHA256 {
			return errors.New("artifact signature is missing or mismatched")
		}
		digest, err := hex.DecodeString(artifact.SHA256)
		if err != nil {
			return errors.New("artifact digest is invalid")
		}
		raw, err := base64.StdEncoding.DecodeString(sig.Signature)
		if err != nil || !ed25519.Verify(trusted, digest, raw) {
			return errors.New("artifact signature verification failed")
		}
		if !bytes.Contains(checksums, []byte(artifact.SHA256+"  "+artifact.Path+"\n")) {
			return errors.New("artifact checksum entry is missing")
		}
	}
	return nil
}
func rejectUnknownReleaseFiles(dir string, expected map[string]bool) error {
	return filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return errors.New("walk release bundle")
		}
		if path == dir {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("release bundle contains symlink")
		}
		if entry.IsDir() {
			if rel != "artifacts" && rel != "evidence" {
				return errors.New("release bundle contains unknown directory")
			}
			return nil
		}
		if !expected[rel] {
			return errors.New("release bundle contains unknown file: " + rel)
		}
		return nil
	})
}
