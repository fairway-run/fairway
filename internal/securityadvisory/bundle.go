package securityadvisory

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	AdvisorySchema        = "fairway.security-advisory.v1"
	ManifestSchema        = "fairway.restricted-advisory-manifest.v1"
	SignatureSchema       = "fairway.restricted-advisory-signature.v1"
	VerificationSchema    = "fairway.restricted-advisory-verification.v1"
	AcknowledgementSchema = "fairway.restricted-advisory-acknowledgement.v1"
	AuthorityBoundary     = "security advisory evidence and customer-controlled receipt only; not certification, compliance, risk acceptance, patch import, deployment approval, or live-operation authority"
	patchPath             = "patch/patch-bundle.bin"
	maxMetadataBytes      = 2 << 20
	maxPatchBytes         = int64(8 << 30)
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{2,127}$`)
	privatePattern    = regexp.MustCompile(`(?i)(-----BEGIN [A-Z ]*PRIVATE KEY-----|\bbearer\s+[A-Za-z0-9._~+/-]+=*|\b(?:password|passwd|api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret)\s*[:=])`)
)

var fixedNonClaims = []string{
	"no_certification_or_compliance_claim",
	"no_patch_import_or_deployment_authority",
	"no_risk_acceptance",
	"no_live_operation_authority",
}

type Advisory struct {
	Schema            string      `json:"schema"`
	ID                string      `json:"id"`
	PublishedAt       string      `json:"published_at"`
	Severity          string      `json:"severity"`
	Summary           string      `json:"summary"`
	AffectedVersions  []string    `json:"affected_versions"`
	FixedVersions     []string    `json:"fixed_versions"`
	Mitigations       []string    `json:"mitigations"`
	VEXUpdates        []VEXUpdate `json:"vex_updates"`
	PatchBundleID     string      `json:"patch_bundle_id"`
	RollbackBundleID  string      `json:"rollback_bundle_id"`
	SupportTrack      string      `json:"support_track"`
	Synthetic         bool        `json:"synthetic"`
	AuthorityBoundary string      `json:"authority_boundary"`
}

type VEXUpdate struct {
	VulnerabilityID string `json:"vulnerability_id"`
	Status          string `json:"status"`
	Justification   string `json:"justification"`
}

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Manifest struct {
	Schema            string   `json:"schema"`
	CreatedAt         string   `json:"created_at"`
	AdvisoryID        string   `json:"advisory_id"`
	PatchBundleID     string   `json:"patch_bundle_id"`
	RollbackBundleID  string   `json:"rollback_bundle_id"`
	SigningKeyID      string   `json:"signing_key_id"`
	Files             []File   `json:"files"`
	NonClaims         []string `json:"non_claims"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

type Signature struct {
	Schema         string `json:"schema"`
	Algorithm      string `json:"algorithm"`
	KeyID          string `json:"key_id"`
	PublicKey      string `json:"public_key"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Signature      string `json:"signature"`
}

type ExportOptions struct {
	Advisory         Advisory
	PatchBundlePath  string
	OutputDirectory  string
	SigningKeyBase64 string
}

type VerifyOptions struct {
	Directory                string
	TrustedPublicKeyBase64   string
	ExpectedAdvisoryID       string
	ExpectedPatchBundleID    string
	ExpectedRollbackBundleID string
}

type Verification struct {
	Schema            string   `json:"schema"`
	OK                bool     `json:"ok"`
	AdvisoryID        string   `json:"advisory_id,omitempty"`
	PatchBundleID     string   `json:"patch_bundle_id,omitempty"`
	RollbackBundleID  string   `json:"rollback_bundle_id,omitempty"`
	Severity          string   `json:"severity,omitempty"`
	SigningKeyID      string   `json:"signing_key_id,omitempty"`
	ManifestSHA256    string   `json:"manifest_sha256,omitempty"`
	PatchSHA256       string   `json:"patch_sha256,omitempty"`
	SignatureStatus   string   `json:"signature_status"`
	InventoryStatus   string   `json:"inventory_status"`
	Issues            []string `json:"issues,omitempty"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

type AcknowledgeOptions struct {
	VerifyOptions
	OutputPath        string
	CustomerReference string
	Status            string
	AcknowledgedAt    time.Time
}

type Acknowledgement struct {
	Schema            string `json:"schema"`
	AdvisoryID        string `json:"advisory_id"`
	PatchBundleID     string `json:"patch_bundle_id"`
	RollbackBundleID  string `json:"rollback_bundle_id"`
	PatchSHA256       string `json:"patch_sha256"`
	ManifestSHA256    string `json:"manifest_sha256"`
	SigningKeyID      string `json:"signing_key_id"`
	CustomerReference string `json:"customer_reference"`
	Status            string `json:"status"`
	AcknowledgedAt    string `json:"acknowledged_at"`
	AuthorityBoundary string `json:"authority_boundary"`
}

func LoadAdvisory(path string) (Advisory, error) {
	var advisory Advisory
	data, err := readRegularFile(path, maxMetadataBytes)
	if err != nil {
		return advisory, errors.New("read advisory input")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return advisory, errors.New("advisory input has duplicate JSON keys")
	}
	if err := decodeStrictJSON(data, &advisory); err != nil {
		return advisory, errors.New("decode advisory input")
	}
	if err := Validate(advisory); err != nil {
		return advisory, err
	}
	return advisory, nil
}

func Validate(advisory Advisory) error {
	if advisory.Schema != AdvisorySchema {
		return errors.New("unsupported advisory schema")
	}
	if !identifierPattern.MatchString(advisory.ID) || !identifierPattern.MatchString(advisory.PatchBundleID) || !identifierPattern.MatchString(advisory.RollbackBundleID) {
		return errors.New("advisory, patch bundle, and rollback bundle identifiers must use bounded identifier syntax")
	}
	if _, err := time.Parse(time.RFC3339, advisory.PublishedAt); err != nil {
		return errors.New("advisory published_at must be RFC3339")
	}
	if !oneOf(advisory.Severity, "low", "medium", "high", "critical") {
		return errors.New("advisory severity must be low, medium, high, or critical")
	}
	if !oneOf(advisory.SupportTrack, "standard", "lts", "emergency") {
		return errors.New("advisory support_track must be standard, lts, or emergency")
	}
	if advisory.AuthorityBoundary != "" && advisory.AuthorityBoundary != AuthorityBoundary {
		return errors.New("advisory authority boundary must be omitted or use the fixed Fairway boundary")
	}
	if err := validateText("summary", advisory.Summary, 500); err != nil {
		return err
	}
	if err := validateTextList("affected_versions", advisory.AffectedVersions, 32); err != nil {
		return err
	}
	if err := validateTextList("fixed_versions", advisory.FixedVersions, 32); err != nil {
		return err
	}
	if err := validateTextList("mitigations", advisory.Mitigations, 32); err != nil {
		return err
	}
	if len(advisory.VEXUpdates) == 0 || len(advisory.VEXUpdates) > 128 {
		return errors.New("advisory requires between 1 and 128 VEX updates")
	}
	seen := map[string]bool{}
	for _, update := range advisory.VEXUpdates {
		if !identifierPattern.MatchString(update.VulnerabilityID) || seen[update.VulnerabilityID] {
			return errors.New("VEX vulnerability identifiers must be unique bounded identifiers")
		}
		seen[update.VulnerabilityID] = true
		if !oneOf(update.Status, "affected", "fixed", "not_affected", "under_investigation") {
			return errors.New("VEX status must be affected, fixed, not_affected, or under_investigation")
		}
		if err := validateText("VEX justification", update.Justification, 500); err != nil {
			return err
		}
	}
	return nil
}

func Export(opts ExportOptions) (Manifest, error) {
	advisory := opts.Advisory
	advisory.AuthorityBoundary = AuthorityBoundary
	if err := Validate(advisory); err != nil {
		return Manifest{}, err
	}
	privateKey, err := decodePrivateKey(opts.SigningKeyBase64)
	if err != nil {
		return Manifest{}, err
	}
	out := filepath.Clean(opts.OutputDirectory)
	if strings.TrimSpace(opts.OutputDirectory) == "" || out == "." {
		return Manifest{}, errors.New("advisory output directory is required")
	}
	parent := filepath.Dir(out)
	if err := ensureRealDirectory(parent); err != nil {
		return Manifest{}, err
	}
	if _, err := os.Lstat(out); err == nil || !os.IsNotExist(err) {
		return Manifest{}, errors.New("advisory output directory must not already exist")
	}
	tmp, err := os.MkdirTemp(parent, ".fairway-advisory-")
	if err != nil {
		return Manifest{}, errors.New("create advisory staging directory")
	}
	defer os.RemoveAll(tmp)
	if err := os.Chmod(tmp, 0o700); err != nil {
		return Manifest{}, errors.New("secure advisory staging directory")
	}
	if err := os.Mkdir(filepath.Join(tmp, "patch"), 0o700); err != nil {
		return Manifest{}, errors.New("create advisory patch directory")
	}
	advisoryBytes, _ := stableJSON(advisory)
	markdown := []byte(renderMarkdown(advisory))
	files := make([]File, 0, 3)
	for _, item := range []struct {
		path string
		data []byte
	}{{"advisory.json", advisoryBytes}, {"advisory.md", markdown}} {
		entry, err := writeFile(tmp, item.path, item.data)
		if err != nil {
			return Manifest{}, err
		}
		files = append(files, entry)
	}
	patchEntry, err := copyFile(tmp, patchPath, opts.PatchBundlePath)
	if err != nil {
		return Manifest{}, err
	}
	files = append(files, patchEntry)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	publicKey := privateKey.Public().(ed25519.PublicKey)
	keyDigest := sha256.Sum256(publicKey)
	manifest := Manifest{
		Schema:            ManifestSchema,
		CreatedAt:         advisory.PublishedAt,
		AdvisoryID:        advisory.ID,
		PatchBundleID:     advisory.PatchBundleID,
		RollbackBundleID:  advisory.RollbackBundleID,
		SigningKeyID:      "sha256:" + hex.EncodeToString(keyDigest[:]),
		Files:             files,
		NonClaims:         append([]string(nil), fixedNonClaims...),
		AuthorityBoundary: AuthorityBoundary,
	}
	manifestBytes, _ := stableJSON(manifest)
	manifestDigest := sha256.Sum256(manifestBytes)
	signature := Signature{
		Schema:         SignatureSchema,
		Algorithm:      "ed25519-sha256",
		KeyID:          manifest.SigningKeyID,
		PublicKey:      base64.StdEncoding.EncodeToString(publicKey),
		ManifestSHA256: hex.EncodeToString(manifestDigest[:]),
		Signature:      base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, manifestDigest[:])),
	}
	signatureBytes, _ := stableJSON(signature)
	if _, err := writeFile(tmp, "manifest.json", manifestBytes); err != nil {
		return Manifest{}, err
	}
	if _, err := writeFile(tmp, "signature.json", signatureBytes); err != nil {
		return Manifest{}, err
	}
	if err := os.Rename(tmp, out); err != nil {
		return Manifest{}, errors.New("publish advisory directory")
	}
	return manifest, nil
}

func Verify(opts VerifyOptions) (Verification, error) {
	report := Verification{Schema: VerificationSchema, SignatureStatus: "invalid", InventoryStatus: "invalid", AuthorityBoundary: AuthorityBoundary}
	if strings.TrimSpace(opts.TrustedPublicKeyBase64) == "" || !identifierPattern.MatchString(opts.ExpectedAdvisoryID) || !identifierPattern.MatchString(opts.ExpectedPatchBundleID) || !identifierPattern.MatchString(opts.ExpectedRollbackBundleID) {
		return report, errors.New("verification requires a pinned public key and exact advisory, patch, and rollback bundle identifiers")
	}
	dir := filepath.Clean(opts.Directory)
	info, err := os.Lstat(dir)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return report, errors.New("advisory package must be a non-symlink directory")
	}
	manifestBytes, err := readRegularFile(filepath.Join(dir, "manifest.json"), maxMetadataBytes)
	if err != nil || rejectDuplicateJSONKeys(manifestBytes) != nil {
		return report, errors.New("read strict advisory manifest")
	}
	var manifest Manifest
	if decodeStrictJSON(manifestBytes, &manifest) != nil || manifest.Schema != ManifestSchema {
		return report, errors.New("decode advisory manifest")
	}
	report.AdvisoryID, report.PatchBundleID, report.RollbackBundleID, report.SigningKeyID = manifest.AdvisoryID, manifest.PatchBundleID, manifest.RollbackBundleID, manifest.SigningKeyID
	manifestDigest := sha256.Sum256(manifestBytes)
	report.ManifestSHA256 = hex.EncodeToString(manifestDigest[:])
	if manifest.AdvisoryID != opts.ExpectedAdvisoryID || manifest.PatchBundleID != opts.ExpectedPatchBundleID || manifest.RollbackBundleID != opts.ExpectedRollbackBundleID || manifest.AuthorityBoundary != AuthorityBoundary {
		return report, errors.New("advisory manifest identity or authority boundary mismatch")
	}
	if !equalStrings(manifest.NonClaims, fixedNonClaims) {
		return report, errors.New("advisory manifest nonclaim contract mismatch")
	}
	signatureBytes, err := readRegularFile(filepath.Join(dir, "signature.json"), maxMetadataBytes)
	if err != nil || rejectDuplicateJSONKeys(signatureBytes) != nil {
		return report, errors.New("read strict advisory signature")
	}
	var signature Signature
	if decodeStrictJSON(signatureBytes, &signature) != nil || signature.Schema != SignatureSchema || signature.Algorithm != "ed25519-sha256" {
		return report, errors.New("decode advisory signature")
	}
	trusted, err := decodePublicKey(opts.TrustedPublicKeyBase64)
	if err != nil {
		return report, err
	}
	embedded, err := base64.StdEncoding.DecodeString(signature.PublicKey)
	if err != nil || len(embedded) != ed25519.PublicKeySize || subtle.ConstantTimeCompare(embedded, trusted) != 1 {
		return report, errors.New("advisory embedded key does not match pinned key")
	}
	keyDigest := sha256.Sum256(trusted)
	keyID := "sha256:" + hex.EncodeToString(keyDigest[:])
	if manifest.SigningKeyID != keyID || signature.KeyID != keyID || signature.ManifestSHA256 != report.ManifestSHA256 {
		return report, errors.New("advisory signing identity mismatch")
	}
	sig, err := base64.StdEncoding.DecodeString(signature.Signature)
	if err != nil || !ed25519.Verify(trusted, manifestDigest[:], sig) {
		return report, errors.New("advisory signature verification failed")
	}
	report.SignatureStatus = "verified_pinned"
	if err := verifyInventory(dir, manifest.Files); err != nil {
		return report, err
	}
	advisoryBytes, err := readRegularFile(filepath.Join(dir, "advisory.json"), maxMetadataBytes)
	if err != nil || rejectDuplicateJSONKeys(advisoryBytes) != nil {
		return report, errors.New("read strict advisory record")
	}
	var advisory Advisory
	if decodeStrictJSON(advisoryBytes, &advisory) != nil || Validate(advisory) != nil {
		return report, errors.New("decode advisory record")
	}
	if advisory.ID != manifest.AdvisoryID || advisory.PatchBundleID != manifest.PatchBundleID || advisory.RollbackBundleID != manifest.RollbackBundleID || advisory.PublishedAt != manifest.CreatedAt || advisory.AuthorityBoundary != AuthorityBoundary {
		return report, errors.New("advisory record does not match manifest")
	}
	markdown, err := readRegularFile(filepath.Join(dir, "advisory.md"), maxMetadataBytes)
	if err != nil || !bytes.Equal(markdown, []byte(renderMarkdown(advisory))) {
		return report, errors.New("generated advisory human view mismatch")
	}
	for _, entry := range manifest.Files {
		if entry.Path == patchPath {
			report.PatchSHA256 = entry.SHA256
		}
	}
	if report.PatchSHA256 == "" {
		return report, errors.New("advisory patch artifact is missing")
	}
	report.Severity = advisory.Severity
	report.InventoryStatus = "verified_exact"
	report.OK = true
	return report, nil
}

func Acknowledge(opts AcknowledgeOptions) (Acknowledgement, error) {
	verification, err := Verify(opts.VerifyOptions)
	if err != nil || !verification.OK {
		return Acknowledgement{}, errors.New("cannot acknowledge an unverified advisory package")
	}
	if !identifierPattern.MatchString(opts.CustomerReference) || !oneOf(opts.Status, "received", "deferred", "rejected") {
		return Acknowledgement{}, errors.New("acknowledgement requires a bounded customer reference and received, deferred, or rejected status")
	}
	if opts.AcknowledgedAt.IsZero() {
		return Acknowledgement{}, errors.New("acknowledgement time is required")
	}
	ack := Acknowledgement{
		Schema: AcknowledgementSchema, AdvisoryID: verification.AdvisoryID, PatchBundleID: verification.PatchBundleID, RollbackBundleID: verification.RollbackBundleID,
		PatchSHA256: verification.PatchSHA256, ManifestSHA256: verification.ManifestSHA256, SigningKeyID: verification.SigningKeyID,
		CustomerReference: opts.CustomerReference, Status: opts.Status, AcknowledgedAt: opts.AcknowledgedAt.UTC().Format(time.RFC3339), AuthorityBoundary: AuthorityBoundary,
	}
	data, _ := stableJSON(ack)
	path := filepath.Clean(opts.OutputPath)
	if strings.TrimSpace(opts.OutputPath) == "" || path == "." {
		return Acknowledgement{}, errors.New("acknowledgement output path is required")
	}
	if err := ensureRealDirectory(filepath.Dir(path)); err != nil {
		return Acknowledgement{}, err
	}
	if _, err := os.Lstat(path); err == nil || !os.IsNotExist(err) {
		return Acknowledgement{}, errors.New("acknowledgement output must not already exist")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Acknowledgement{}, errors.New("write acknowledgement")
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return Acknowledgement{}, errors.New("write acknowledgement")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return Acknowledgement{}, errors.New("write acknowledgement")
	}
	return ack, nil
}

func renderMarkdown(advisory Advisory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Security Advisory %s\n\n", markdown(advisory.ID))
	fmt.Fprintf(&b, "- Published: `%s`\n- Severity: `%s`\n- Support track: `%s`\n- Patch bundle: `%s`\n- Rollback bundle: `%s`\n- Synthetic rehearsal: `%t`\n\n", advisory.PublishedAt, advisory.Severity, advisory.SupportTrack, advisory.PatchBundleID, advisory.RollbackBundleID, advisory.Synthetic)
	fmt.Fprintf(&b, "## Summary\n\n%s\n\n## Affected Versions\n\n", markdown(advisory.Summary))
	for _, value := range advisory.AffectedVersions {
		fmt.Fprintf(&b, "- `%s`\n", markdown(value))
	}
	b.WriteString("\n## Fixed Versions\n\n")
	for _, value := range advisory.FixedVersions {
		fmt.Fprintf(&b, "- `%s`\n", markdown(value))
	}
	b.WriteString("\n## Mitigations\n\n")
	for _, value := range advisory.Mitigations {
		fmt.Fprintf(&b, "- %s\n", markdown(value))
	}
	b.WriteString("\n## VEX Updates\n\n| Vulnerability | Status | Justification |\n|---|---|---|\n")
	for _, update := range advisory.VEXUpdates {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", markdown(update.VulnerabilityID), update.Status, markdown(update.Justification))
	}
	fmt.Fprintf(&b, "\n## Authority Boundary\n\n%s.\n", AuthorityBoundary)
	return b.String()
}

func verifyInventory(dir string, files []File) error {
	expected := map[string]File{"manifest.json": {Path: "manifest.json"}, "signature.json": {Path: "signature.json"}}
	for _, entry := range files {
		if !safeRelativePath(entry.Path) || entry.Bytes < 0 || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(entry.SHA256) {
			return errors.New("advisory manifest contains invalid file metadata")
		}
		if _, exists := expected[entry.Path]; exists {
			return errors.New("advisory manifest contains duplicate file paths")
		}
		expected[entry.Path] = entry
	}
	for _, required := range []string{"advisory.json", "advisory.md", patchPath} {
		if _, ok := expected[required]; !ok {
			return errors.New("advisory manifest is incomplete")
		}
	}
	seen := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("walk advisory package")
		}
		if path == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return errors.New("resolve advisory package path")
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("advisory package contains a symlink")
		}
		if entry.IsDir() {
			if rel != "patch" {
				return errors.New("advisory package contains an unknown directory")
			}
			return nil
		}
		want, ok := expected[rel]
		if !ok {
			return errors.New("advisory package contains an unknown file")
		}
		seen[rel] = true
		if rel == "manifest.json" || rel == "signature.json" {
			return nil
		}
		limit := int64(maxMetadataBytes)
		if rel == patchPath {
			limit = maxPatchBytes
		}
		if want.Bytes > limit {
			return errors.New("advisory package file exceeds its size limit")
		}
		actual, err := hashRegularFile(path, limit)
		if err != nil {
			return err
		}
		if actual.SHA256 != want.SHA256 || actual.Bytes != want.Bytes {
			return errors.New("advisory package file digest mismatch")
		}
		return nil
	})
	if err != nil {
		return err
	}
	for path := range expected {
		if !seen[path] {
			return errors.New("advisory package file is missing")
		}
	}
	if len(seen) != 5 {
		return errors.New("advisory package inventory is not exact")
	}
	return nil
}

func validateTextList(label string, values []string, max int) error {
	if len(values) == 0 || len(values) > max {
		return fmt.Errorf("advisory %s requires between 1 and %d values", label, max)
	}
	seen := map[string]bool{}
	for _, value := range values {
		if err := validateText(label, value, 500); err != nil {
			return err
		}
		if seen[value] {
			return fmt.Errorf("advisory %s values must be unique", label)
		}
		seen[value] = true
	}
	return nil
}

func validateText(label, value string, max int) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > max || strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("advisory %s must be bounded single-line text", label)
	}
	if privatePattern.MatchString(value) {
		return fmt.Errorf("advisory %s contains prohibited private content", label)
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func markdown(value string) string {
	r := strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "|", "\\|")
	return r.Replace(value)
}

func writeFile(root, rel string, data []byte) (File, error) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return File{}, errors.New("write advisory file")
	}
	digest := sha256.Sum256(data)
	return File{Path: rel, SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(data))}, nil
}

func copyFile(root, rel, source string) (File, error) {
	in, before, err := openRegularNoFollow(source, maxPatchBytes)
	if err != nil {
		return File{}, errors.New("open patch bundle")
	}
	defer in.Close()
	out, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(rel)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return File{}, errors.New("create patch bundle copy")
	}
	written, digest, copyErr := consumeOpenedRegular(source, in, before, maxPatchBytes, out)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written <= 0 || written > maxPatchBytes {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(rel)))
		return File{}, errors.New("copy bounded patch bundle")
	}
	return File{Path: rel, SHA256: digest, Bytes: written}, nil
}

func hashRegularFile(path string, limit int64) (File, error) {
	file, before, err := openRegularNoFollow(path, limit)
	if err != nil {
		return File{}, errors.New("hash advisory regular file")
	}
	defer file.Close()
	written, digest, err := consumeOpenedRegular(path, file, before, limit, nil)
	if err != nil {
		return File{}, errors.New("hash advisory file")
	}
	return File{SHA256: digest, Bytes: written}, nil
}

func readRegularFile(path string, limit int64) ([]byte, error) {
	file, before, err := openRegularNoFollow(path, limit)
	if err != nil {
		return nil, errors.New("read bounded regular file")
	}
	defer file.Close()
	var buffer bytes.Buffer
	if _, _, err := consumeOpenedRegular(path, file, before, limit, &buffer); err != nil {
		return nil, errors.New("read bounded regular file")
	}
	return buffer.Bytes(), nil
}

func openRegularNoFollow(path string, limit int64) (*os.File, os.FileInfo, error) {
	clean := filepath.Clean(path)
	file, err := os.OpenFile(clean, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, nil, errors.New("open non-symlink regular file")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > limit {
		_ = file.Close()
		return nil, nil, errors.New("opened file is not a bounded regular file")
	}
	return file, info, nil
}

func consumeOpenedRegular(path string, file *os.File, before os.FileInfo, limit int64, destination io.Writer) (int64, string, error) {
	hash := sha256.New()
	writer := io.Writer(hash)
	if destination != nil {
		writer = io.MultiWriter(destination, hash)
	}
	written, err := io.Copy(writer, io.LimitReader(file, limit+1))
	if err != nil || written > limit {
		return 0, "", errors.New("bounded regular file read failed")
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != written || after.Size() != written || !before.ModTime().Equal(after.ModTime()) {
		return 0, "", errors.New("regular file changed while reading")
	}
	current, err := os.Lstat(filepath.Clean(path))
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(before, current) {
		return 0, "", errors.New("regular file path changed while reading")
	}
	return written, hex.EncodeToString(hash.Sum(nil)), nil
}

func ensureRealDirectory(path string) error {
	clean := filepath.Clean(path)
	info, err := os.Lstat(clean)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("output parent must be an existing directory")
	}
	return nil
}

func safeRelativePath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return path == clean && clean != "." && !strings.HasPrefix(clean, "/") && !strings.HasPrefix(clean, "../") && !strings.Contains(clean, "/../")
}

func stableJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func decodePrivateKey(encoded string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, errors.New("decode advisory signing key")
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(raw)
	if err != nil {
		return nil, errors.New("advisory signing key must be Ed25519 seed, private key, or PKCS8 DER")
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("advisory signing key is not Ed25519")
	}
	return key, nil
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, errors.New("decode advisory trusted public key")
	}
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}
	parsed, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, errors.New("advisory trusted key must be Ed25519 public key or PKIX DER")
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("advisory trusted key is not Ed25519")
	}
	return key, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing JSON content")
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok || seen[key] {
					return errors.New("duplicate JSON object key")
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	return walk()
}
