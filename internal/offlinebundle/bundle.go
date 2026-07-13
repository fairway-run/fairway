package offlinebundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/provenance"
)

const (
	ManifestSchema     = "fairway.offline-distribution-manifest.v1"
	SignatureSchema    = "fairway.offline-distribution-signature.v1"
	VerificationSchema = "fairway.offline-distribution-verification.v1"
	maxManifestSize    = 4 << 20
	maxBundleFiles     = 4096
	maxBundleBytes     = int64(4 << 30)
	maxInputFileBytes  = int64(512 << 20)
	maxTextAssetBytes  = int64(16 << 20)
)

var requiredAssetClasses = []string{"configuration", "deployment_baseline", "documentation", "verifier"}

var requiredVerifierNames = []string{
	"fairway-offline-verify_darwin_amd64",
	"fairway-offline-verify_darwin_arm64",
	"fairway-offline-verify_linux_amd64",
	"fairway-offline-verify_linux_arm64",
}

var supportedTargets = []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"}

type ReleaseIdentity struct {
	Version       string `json:"version"`
	SourceSHA     string `json:"source_sha"`
	BuilderID     string `json:"builder_id"`
	PolicyVersion string `json:"policy_version"`
	Path          string `json:"path"`
}

type File struct {
	Class      string `json:"class"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Bytes      int64  `json:"bytes"`
	Executable bool   `json:"executable,omitempty"`
}

type Manifest struct {
	Schema            string          `json:"schema"`
	BundleVersion     string          `json:"bundle_version"`
	CreatedAt         string          `json:"created_at"`
	Current           ReleaseIdentity `json:"current"`
	Rollback          ReleaseIdentity `json:"rollback"`
	SigningKeyID      string          `json:"signing_key_id"`
	Files             []File          `json:"files"`
	RequiredClasses   []string        `json:"required_asset_classes"`
	NonClaims         []string        `json:"non_claims"`
	AuthorityBoundary string          `json:"authority_boundary"`
}

type Signature struct {
	Schema         string `json:"schema"`
	Algorithm      string `json:"algorithm"`
	ManifestSHA256 string `json:"manifest_sha256"`
	PublicKey      string `json:"public_key"`
	Signature      string `json:"signature"`
}

type Asset struct {
	Class      string
	Name       string
	Path       string
	Executable bool
}

type ExportOptions struct {
	OutputDirectory        string
	CurrentAssuranceDir    string
	RollbackAssuranceDir   string
	TrustedPublicKeyBase64 string
	SigningKeyBase64       string
	CurrentExpected        ReleaseIdentity
	RollbackExpected       ReleaseIdentity
	Assets                 []Asset
	CreatedAt              time.Time
}

type VerifyOptions struct {
	Directory              string
	TrustedPublicKeyBase64 string
	CurrentExpected        ReleaseIdentity
	RollbackExpected       ReleaseIdentity
}

type Verification struct {
	Schema                   string          `json:"schema"`
	OK                       bool            `json:"ok"`
	Current                  ReleaseIdentity `json:"current"`
	Rollback                 ReleaseIdentity `json:"rollback"`
	SignatureStatus          string          `json:"signature_status"`
	InventoryStatus          string          `json:"inventory_status"`
	CurrentAssuranceStatus   string          `json:"current_assurance_status"`
	RollbackAssuranceStatus  string          `json:"rollback_assurance_status"`
	RequiredAssetClassStatus string          `json:"required_asset_class_status"`
	FileCount                int             `json:"file_count"`
	Issues                   []string        `json:"issues,omitempty"`
	NonClaims                []string        `json:"non_claims"`
	AuthorityBoundary        string          `json:"authority_boundary"`
}

func Export(opts ExportOptions) (Manifest, error) {
	if opts.CreatedAt.IsZero() {
		return Manifest{}, errors.New("offline bundle creation time is required")
	}
	if err := validateExpectedPair(opts.CurrentExpected, opts.RollbackExpected); err != nil {
		return Manifest{}, err
	}
	trusted, err := decodePublicKey(opts.TrustedPublicKeyBase64)
	if err != nil {
		return Manifest{}, err
	}
	privateKey, err := decodePrivateKey(opts.SigningKeyBase64)
	if err != nil {
		return Manifest{}, err
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	if subtle.ConstantTimeCompare(publicKey, trusted) != 1 {
		return Manifest{}, errors.New("offline bundle signing key does not match pinned public key")
	}
	if err := verifyNestedRelease(opts.CurrentAssuranceDir, trusted, opts.CurrentExpected); err != nil {
		return Manifest{}, fmt.Errorf("current release assurance: %w", err)
	}
	if err := verifyNestedRelease(opts.RollbackAssuranceDir, trusted, opts.RollbackExpected); err != nil {
		return Manifest{}, fmt.Errorf("rollback release assurance: %w", err)
	}
	dir, err := prepareOutput(opts.OutputDirectory)
	if err != nil {
		return Manifest{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := copyTree(opts.CurrentAssuranceDir, filepath.Join(dir, "releases", "current")); err != nil {
		return Manifest{}, fmt.Errorf("copy current release assurance: %w", err)
	}
	if err := copyTree(opts.RollbackAssuranceDir, filepath.Join(dir, "releases", "rollback")); err != nil {
		return Manifest{}, fmt.Errorf("copy rollback release assurance: %w", err)
	}
	currentWithPath := currentIdentityForPath(opts.CurrentExpected, "releases/current")
	rollbackWithPath := currentIdentityForPath(opts.RollbackExpected, "releases/rollback")
	if err := verifyReleaseArchives(dir, currentWithPath, rollbackWithPath); err != nil {
		return Manifest{}, err
	}
	classCounts := map[string]int{}
	seenAssets := map[string]bool{}
	for _, asset := range opts.Assets {
		if err := validateAsset(asset); err != nil {
			return Manifest{}, err
		}
		rel := filepath.ToSlash(filepath.Join("assets", asset.Class, asset.Name))
		if seenAssets[rel] {
			return Manifest{}, fmt.Errorf("duplicate offline asset path %q", rel)
		}
		seenAssets[rel] = true
		classCounts[asset.Class]++
		if err := validateAssetPrivacy(asset); err != nil {
			return Manifest{}, err
		}
		if err := copyRegular(asset.Path, filepath.Join(dir, filepath.FromSlash(rel)), asset.Executable); err != nil {
			return Manifest{}, fmt.Errorf("copy offline asset %s", asset.Name)
		}
	}
	for _, class := range requiredAssetClasses {
		if classCounts[class] == 0 {
			return Manifest{}, fmt.Errorf("offline bundle missing required asset class %s", class)
		}
	}
	if err := verifyRequiredVerifierAssets(opts.Assets); err != nil {
		return Manifest{}, err
	}
	current := opts.CurrentExpected
	current.Path = "releases/current"
	rollback := opts.RollbackExpected
	rollback.Path = "releases/rollback"
	if err := writeLifecycleScripts(dir, current, rollback); err != nil {
		return Manifest{}, err
	}
	files, err := inventory(dir)
	if err != nil {
		return Manifest{}, err
	}
	keyDigest := sha256.Sum256(publicKey)
	manifest := Manifest{
		Schema: ManifestSchema, BundleVersion: "v1", CreatedAt: opts.CreatedAt.UTC().Format(time.RFC3339Nano),
		Current: current, Rollback: rollback, SigningKeyID: "sha256:" + hex.EncodeToString(keyDigest[:]), Files: files,
		RequiredClasses:   append([]string(nil), requiredAssetClasses...),
		NonClaims:         []string{"no_certification_or_compliance_claim", "no_installation_authorization", "no_dependency_trust_claim", "no_rollback_safety_claim_without_rehearsal", "no_verifier_bootstrap_or_media_custody_claim"},
		AuthorityBoundary: "offline distribution evidence and operator-invoked lifecycle tools only; not verifier bootstrap trust, media custody, certification, compliance, approval, risk acceptance, deployment authorization, credential authority, public exposure, or live-operation authority",
	}
	if err := validateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	manifestBytes, _ := stableJSON(manifest)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestBytes, 0o600); err != nil {
		return Manifest{}, errors.New("write offline bundle manifest")
	}
	digest := sha256.Sum256(manifestBytes)
	signature := Signature{
		Schema: SignatureSchema, Algorithm: "ed25519-sha256", ManifestSHA256: hex.EncodeToString(digest[:]),
		PublicKey: base64.StdEncoding.EncodeToString(publicKey), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, digest[:])),
	}
	signatureBytes, _ := stableJSON(signature)
	if err := os.WriteFile(filepath.Join(dir, "manifest.sig.json"), signatureBytes, 0o600); err != nil {
		return Manifest{}, errors.New("write offline bundle signature")
	}
	cleanup = false
	return manifest, nil
}

func Verify(opts VerifyOptions) (Verification, error) {
	report := Verification{
		Schema: VerificationSchema, SignatureStatus: "failed", InventoryStatus: "failed",
		CurrentAssuranceStatus: "failed", RollbackAssuranceStatus: "failed", RequiredAssetClassStatus: "failed",
		NonClaims:         []string{"no_certification_or_compliance_claim", "no_installation_authorization", "no_dependency_trust_claim", "no_rollback_safety_claim_without_rehearsal", "no_verifier_bootstrap_or_media_custody_claim"},
		AuthorityBoundary: "verification result only; not verifier bootstrap trust, media custody, certification, compliance, approval, risk acceptance, installation authorization, credential authority, public exposure, or live-operation authority",
	}
	if err := validateExpectedPair(opts.CurrentExpected, opts.RollbackExpected); err != nil {
		return report, err
	}
	trusted, err := decodePublicKey(opts.TrustedPublicKeyBase64)
	if err != nil {
		return report, err
	}
	dir, err := safeDirectory(opts.Directory)
	if err != nil {
		return report, err
	}
	manifestBytes, err := readBoundedRegular(filepath.Join(dir, "manifest.json"), maxManifestSize)
	if err != nil {
		return report, errors.New("read offline bundle manifest")
	}
	if err := rejectDuplicateJSONKeys(manifestBytes); err != nil {
		return report, errors.New("offline bundle manifest has duplicate JSON keys")
	}
	var manifest Manifest
	if err := decodeStrictJSON(manifestBytes, &manifest); err != nil {
		return report, errors.New("decode offline bundle manifest")
	}
	if err := validateManifest(manifest); err != nil {
		return report, err
	}
	report.Current, report.Rollback, report.FileCount = manifest.Current, manifest.Rollback, len(manifest.Files)
	if !sameExpected(manifest.Current, opts.CurrentExpected) || !sameExpected(manifest.Rollback, opts.RollbackExpected) {
		return report, errors.New("offline bundle release or rollback identity does not match expected values")
	}
	signatureBytes, err := readBoundedRegular(filepath.Join(dir, "manifest.sig.json"), maxManifestSize)
	if err != nil {
		return report, errors.New("read offline bundle signature")
	}
	if err := rejectDuplicateJSONKeys(signatureBytes); err != nil {
		return report, errors.New("offline bundle signature has duplicate JSON keys")
	}
	var signature Signature
	if err := decodeStrictJSON(signatureBytes, &signature); err != nil {
		return report, errors.New("decode offline bundle signature")
	}
	if err := verifySignature(signature, manifestBytes, manifest.SigningKeyID, trusted); err != nil {
		return report, err
	}
	report.SignatureStatus = "verified"
	if err := verifyInventory(dir, manifest.Files); err != nil {
		return report, err
	}
	report.InventoryStatus = "verified"
	if err := verifyRequiredClasses(manifest.Files); err != nil {
		return report, err
	}
	report.RequiredAssetClassStatus = "complete"
	if err := verifyReleaseArchives(dir, manifest.Current, manifest.Rollback); err != nil {
		return report, err
	}
	if err := verifyNestedRelease(filepath.Join(dir, filepath.FromSlash(manifest.Current.Path)), trusted, opts.CurrentExpected); err != nil {
		return report, fmt.Errorf("current release assurance: %w", err)
	}
	report.CurrentAssuranceStatus = "verified"
	if err := verifyNestedRelease(filepath.Join(dir, filepath.FromSlash(manifest.Rollback.Path)), trusted, opts.RollbackExpected); err != nil {
		return report, fmt.Errorf("rollback release assurance: %w", err)
	}
	report.RollbackAssuranceStatus = "verified"
	report.OK = true
	return report, nil
}

func verifyNestedRelease(dir string, key ed25519.PublicKey, expected ReleaseIdentity) error {
	report, err := provenance.VerifyReleaseBundle(provenance.ReleaseBundleVerifyOptions{
		Directory: dir, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(key),
		ExpectedVersion: expected.Version, ExpectedSourceSHA: expected.SourceSHA,
		ExpectedBuilderID: expected.BuilderID, ExpectedPolicyVersion: expected.PolicyVersion,
	})
	if err != nil {
		return err
	}
	if !report.OK {
		return errors.New("nested release assurance verification failed")
	}
	return nil
}

func validateExpectedPair(current, rollback ReleaseIdentity) error {
	for label, identity := range map[string]ReleaseIdentity{"current": current, "rollback": rollback} {
		for name, value := range map[string]string{"version": identity.Version, "source sha": identity.SourceSHA, "builder id": identity.BuilderID, "policy version": identity.PolicyVersion} {
			if err := validMetadata(label+" "+name, value); err != nil {
				return err
			}
		}
		if !validSHA(identity.SourceSHA) {
			return fmt.Errorf("%s source sha must be 40 or 64 hexadecimal characters", label)
		}
		if !safeName(strings.TrimPrefix(identity.Version, "v")) {
			return fmt.Errorf("%s version is unsafe", label)
		}
	}
	if current.Version == rollback.Version || current.SourceSHA == rollback.SourceSHA {
		return errors.New("offline bundle current and rollback identities must be distinct")
	}
	return nil
}

func validateAsset(asset Asset) error {
	if !safeName(asset.Class) || !safeName(asset.Name) {
		return errors.New("offline asset class or name is unsafe")
	}
	if strings.Contains(asset.Path, "://") || strings.TrimSpace(asset.Path) == "" {
		return errors.New("offline asset must be a local file")
	}
	return nil
}

func prepareOutput(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return "", errors.New("offline bundle output must be a local path")
	}
	clean := filepath.Clean(path)
	if _, err := os.Lstat(clean); err == nil {
		return "", errors.New("offline bundle output already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("inspect offline bundle output")
	}
	if err := os.MkdirAll(clean, 0o700); err != nil {
		return "", errors.New("create offline bundle output")
	}
	if err := os.Chmod(clean, 0o700); err != nil {
		return "", errors.New("secure offline bundle output")
	}
	return clean, nil
}

func copyTree(source, destination string) error {
	root, err := safeDirectory(source)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("walk release assurance input")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("release assurance input path escapes root")
		}
		if rel == "." {
			return os.MkdirAll(destination, 0o700)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("release assurance input contains symlink")
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			return errors.New("release assurance input contains non-regular file")
		}
		return copyRegular(path, target, false)
	})
}

func copyRegular(source, destination string, executable bool) error {
	info, err := os.Lstat(filepath.Clean(source))
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("offline input must be a non-symlink regular file")
	}
	if info.Size() > maxInputFileBytes {
		return errors.New("offline input is too large")
	}
	input, err := os.Open(filepath.Clean(source))
	if err != nil {
		return errors.New("read offline input")
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return errors.New("create offline asset directory")
	}
	mode := os.FileMode(0o600)
	if executable {
		mode = 0o700
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return errors.New("write offline asset")
	}
	written, copyErr := io.Copy(output, io.LimitReader(input, maxInputFileBytes+1))
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil || written != info.Size() || written > maxInputFileBytes {
		_ = os.Remove(destination)
		return errors.New("copy offline asset")
	}
	return nil
}

func inventory(root string) ([]File, error) {
	var files []File
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("walk offline bundle")
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return errors.New("offline bundle contains unsafe file type")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return errors.New("resolve offline bundle path")
		}
		rel = filepath.ToSlash(rel)
		if !safeRelativePath(rel) {
			return errors.New("offline bundle contains unsafe path")
		}
		info, err := entry.Info()
		if err != nil {
			return errors.New("inspect offline bundle file")
		}
		if info.Size() < 0 || info.Size() > maxInputFileBytes {
			return errors.New("offline bundle file exceeds size limit")
		}
		total += info.Size()
		if total > maxBundleBytes || len(files) >= maxBundleFiles {
			return errors.New("offline bundle inventory exceeds limit")
		}
		input, err := os.Open(path)
		if err != nil {
			return errors.New("read offline bundle file")
		}
		hasher := sha256.New()
		written, copyErr := io.Copy(hasher, io.LimitReader(input, maxInputFileBytes+1))
		closeErr := input.Close()
		if copyErr != nil || closeErr != nil || written != info.Size() || written > maxInputFileBytes {
			return errors.New("hash offline bundle file")
		}
		class, name := classify(rel)
		files = append(files, File{Class: class, Name: name, Path: rel, SHA256: hex.EncodeToString(hasher.Sum(nil)), Bytes: info.Size(), Executable: info.Mode().Perm()&0o111 != 0})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func classify(path string) (string, string) {
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[0] == "assets" {
		return parts[1], parts[len(parts)-1]
	}
	if len(parts) >= 2 && parts[0] == "scripts" {
		return "installer", parts[len(parts)-1]
	}
	if len(parts) >= 2 && parts[0] == "releases" {
		return "release_assurance", parts[len(parts)-1]
	}
	return "bundle", parts[len(parts)-1]
}

func verifyInventory(root string, expected []File) error {
	actual, err := inventoryExcludingManifest(root)
	if err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return errors.New("offline bundle file inventory is incomplete or contains unknown files")
	}
	for i := range expected {
		if actual[i].Path != expected[i].Path || actual[i].Class != expected[i].Class || actual[i].Name != expected[i].Name || actual[i].SHA256 != expected[i].SHA256 || actual[i].Bytes != expected[i].Bytes || actual[i].Executable != expected[i].Executable {
			return errors.New("offline bundle file inventory mismatch")
		}
	}
	return nil
}

func inventoryExcludingManifest(root string) ([]File, error) {
	files, err := inventory(root)
	if err != nil {
		return nil, err
	}
	filtered := files[:0]
	for _, file := range files {
		if file.Path != "manifest.json" && file.Path != "manifest.sig.json" {
			filtered = append(filtered, file)
		}
	}
	return filtered, nil
}

func verifyRequiredClasses(files []File) error {
	counts := map[string]int{}
	verifiers := map[string]bool{}
	for _, file := range files {
		counts[file.Class]++
		if file.Class == "verifier" && file.Executable {
			verifiers[file.Name] = true
		}
	}
	for _, class := range requiredAssetClasses {
		if counts[class] == 0 {
			return fmt.Errorf("offline bundle missing required asset class %s", class)
		}
	}
	if counts["installer"] < 3 || counts["release_assurance"] == 0 {
		return errors.New("offline bundle is missing lifecycle scripts or release assurance content")
	}
	for _, name := range requiredVerifierNames {
		if !verifiers[name] {
			return fmt.Errorf("offline bundle missing required verifier %s", name)
		}
	}
	return nil
}

func verifyRequiredReleaseArtifacts(files []File, current, rollback ReleaseIdentity) error {
	paths := map[string]bool{}
	for _, file := range files {
		paths[file.Path] = true
	}
	for _, release := range []struct {
		name     string
		identity ReleaseIdentity
	}{{"current", current}, {"rollback", rollback}} {
		version := strings.TrimPrefix(release.identity.Version, "v")
		for _, target := range supportedTargets {
			path := "releases/" + release.name + "/artifacts/fairway_" + version + "_" + target + ".tar.gz"
			if !paths[path] {
				return fmt.Errorf("offline bundle missing required %s release archive for %s", release.name, target)
			}
		}
	}
	return nil
}

func currentIdentityForPath(identity ReleaseIdentity, path string) ReleaseIdentity {
	identity.Path = path
	return identity
}

func verifyReleaseArchives(root string, current, rollback ReleaseIdentity) error {
	for _, release := range []struct {
		name     string
		identity ReleaseIdentity
	}{{"current", current}, {"rollback", rollback}} {
		version := strings.TrimPrefix(release.identity.Version, "v")
		for _, target := range supportedTargets {
			path := filepath.Join(root, filepath.FromSlash(release.identity.Path), "artifacts", "fairway_"+version+"_"+target+".tar.gz")
			if err := verifyFairwayArchive(path); err != nil {
				return fmt.Errorf("%s release archive for %s is invalid", release.name, target)
			}
		}
	}
	return nil
}

func verifyFairwayArchive(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxInputFileBytes {
		return errors.New("archive is missing, unsafe, or oversized")
	}
	file, err := os.Open(path)
	if err != nil {
		return errors.New("open archive")
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return errors.New("open gzip archive")
	}
	treader := tar.NewReader(gz)
	header, err := treader.Next()
	if err != nil || header.Name != "fairway" || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) || header.Size <= 0 || header.Size > maxInputFileBytes || header.Mode&0o111 == 0 {
		_ = gz.Close()
		return errors.New("archive must contain one executable fairway file")
	}
	written, err := io.Copy(io.Discard, io.LimitReader(treader, maxInputFileBytes+1))
	if err != nil || written != header.Size || written > maxInputFileBytes {
		_ = gz.Close()
		return errors.New("archive fairway file is incomplete or oversized")
	}
	if _, err := treader.Next(); err != io.EOF {
		_ = gz.Close()
		return errors.New("archive contains additional entries")
	}
	if err := gz.Close(); err != nil {
		return errors.New("close gzip archive")
	}
	return nil
}

func verifyRequiredVerifierAssets(assets []Asset) error {
	verifiers := map[string]bool{}
	for _, asset := range assets {
		if asset.Class == "verifier" && asset.Executable {
			verifiers[asset.Name] = true
		}
	}
	for _, name := range requiredVerifierNames {
		if !verifiers[name] {
			return fmt.Errorf("offline bundle missing required verifier %s", name)
		}
	}
	return nil
}

func validateAssetPrivacy(asset Asset) error {
	if asset.Class == "verifier" {
		return nil
	}
	info, err := os.Lstat(filepath.Clean(asset.Path))
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("offline text asset must be a non-symlink regular file")
	}
	if info.Size() > maxTextAssetBytes {
		return errors.New("offline text asset exceeds size limit")
	}
	data, err := os.ReadFile(filepath.Clean(asset.Path))
	if err != nil {
		return errors.New("read offline text asset")
	}
	if containsPrivate(string(data)) {
		return fmt.Errorf("offline asset %s contains prohibited private content", asset.Name)
	}
	return nil
}

func validateManifest(m Manifest) error {
	if m.Schema != ManifestSchema || m.BundleVersion != "v1" || len(m.Files) == 0 || len(m.Files) > maxBundleFiles {
		return errors.New("invalid offline bundle manifest")
	}
	if _, err := time.Parse(time.RFC3339Nano, m.CreatedAt); err != nil {
		return errors.New("offline bundle manifest created_at is invalid")
	}
	if err := validateExpectedPair(m.Current, m.Rollback); err != nil {
		return err
	}
	if m.Current.Path != "releases/current" || m.Rollback.Path != "releases/rollback" {
		return errors.New("offline bundle release paths are invalid")
	}
	if !strings.HasPrefix(m.SigningKeyID, "sha256:") || !validSHA(strings.TrimPrefix(m.SigningKeyID, "sha256:")) {
		return errors.New("offline bundle signing key id is invalid")
	}
	if strings.Join(m.RequiredClasses, ",") != strings.Join(requiredAssetClasses, ",") {
		return errors.New("offline bundle required asset classes are invalid")
	}
	if strings.Join(m.NonClaims, ",") != "no_certification_or_compliance_claim,no_installation_authorization,no_dependency_trust_claim,no_rollback_safety_claim_without_rehearsal,no_verifier_bootstrap_or_media_custody_claim" {
		return errors.New("offline bundle non-claims are invalid")
	}
	if m.AuthorityBoundary != "offline distribution evidence and operator-invoked lifecycle tools only; not verifier bootstrap trust, media custody, certification, compliance, approval, risk acceptance, deployment authorization, credential authority, public exposure, or live-operation authority" {
		return errors.New("offline bundle authority boundary is invalid")
	}
	seen := map[string]bool{}
	var total int64
	for _, file := range m.Files {
		if !safeRelativePath(file.Path) || !safeName(file.Class) || !safeName(file.Name) || !validSHA(file.SHA256) || file.Bytes < 0 || seen[file.Path] || file.Path == "manifest.json" || file.Path == "manifest.sig.json" {
			return errors.New("offline bundle manifest contains invalid files")
		}
		seen[file.Path] = true
		total += file.Bytes
		if total > maxBundleBytes {
			return errors.New("offline bundle manifest exceeds size limit")
		}
	}
	if err := verifyRequiredClasses(m.Files); err != nil {
		return err
	}
	return verifyRequiredReleaseArtifacts(m.Files, m.Current, m.Rollback)
}

func verifySignature(signature Signature, manifestBytes []byte, keyID string, trusted ed25519.PublicKey) error {
	if signature.Schema != SignatureSchema || signature.Algorithm != "ed25519-sha256" {
		return errors.New("offline bundle signature metadata is invalid")
	}
	embedded, err := decodePublicKey(signature.PublicKey)
	if err != nil || subtle.ConstantTimeCompare(embedded, trusted) != 1 {
		return errors.New("offline bundle signing identity does not match pinned key")
	}
	keyDigest := sha256.Sum256(trusted)
	if keyID != "sha256:"+hex.EncodeToString(keyDigest[:]) {
		return errors.New("offline bundle signing key id does not match pinned key")
	}
	digest := sha256.Sum256(manifestBytes)
	if signature.ManifestSHA256 != hex.EncodeToString(digest[:]) {
		return errors.New("offline bundle manifest digest mismatch")
	}
	raw, err := base64.StdEncoding.DecodeString(signature.Signature)
	if err != nil || len(raw) != ed25519.SignatureSize || !ed25519.Verify(trusted, digest[:], raw) {
		return errors.New("offline bundle signature verification failed")
	}
	return nil
}

func writeLifecycleScripts(root string, current, rollback ReleaseIdentity) error {
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o700); err != nil {
		return errors.New("create offline lifecycle script directory")
	}
	verify := fmt.Sprintf(`#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in x86_64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "unsupported architecture" >&2; exit 1 ;; esac
verifier="$root/assets/verifier/fairway-offline-verify_${os}_${arch}"
test -x "$verifier" || { echo "offline verifier is missing for ${os}/${arch}" >&2; exit 1; }
exec "$verifier" --dir "$root" --trusted-public-key-env FAIRWAY_OFFLINE_TRUSTED_PUBLIC_KEY --expected-version %s --expected-source-sha %s --expected-builder-id %s --expected-policy-version %s --expected-rollback-version %s --expected-rollback-source-sha %s --expected-rollback-builder-id %s --expected-rollback-policy-version %s
`, shellQuote(current.Version), shellQuote(current.SourceSHA), shellQuote(current.BuilderID), shellQuote(current.PolicyVersion), shellQuote(rollback.Version), shellQuote(rollback.SourceSHA), shellQuote(rollback.BuilderID), shellQuote(rollback.PolicyVersion))
	install := fmt.Sprintf(`#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
prefix=${1:?usage: install.sh <absolute-prefix>}
case "$prefix" in /*) ;; *) echo "prefix must be absolute" >&2; exit 1 ;; esac
test "$prefix" != / || { echo "prefix must not be root" >&2; exit 1; }
test ! -L "$prefix" || { echo "prefix must not be a symlink" >&2; exit 1; }
"$root/scripts/verify.sh" >/dev/null
os=$(uname -s | tr '[:upper:]' '[:lower:]'); arch=$(uname -m); case "$arch" in x86_64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) exit 1 ;; esac
archive="$root/releases/current/artifacts/fairway_%s_${os}_${arch}.tar.gz"
test -f "$archive" || { echo "current archive is missing" >&2; exit 1; }
entries=$(tar -tzf "$archive"); test "$entries" = fairway || { echo "archive content is unsafe" >&2; exit 1; }
mkdir -p "$prefix"; prefix=$(CDPATH= cd "$prefix" && pwd -P)
mkdir -p "$prefix/bin" "$prefix/backups"
test ! -L "$prefix/bin" && test ! -L "$prefix/backups" && test ! -L "$prefix/bin/fairway" || { echo "installation path contains a symlink" >&2; exit 1; }
tmp=$(mktemp -d "$prefix/.fairway-install.XXXXXX"); trap 'rm -rf "$tmp"' EXIT HUP INT TERM
tar -xzf "$archive" -C "$tmp"; test -f "$tmp/fairway" && test ! -L "$tmp/fairway"
if test -f "$prefix/bin/fairway"; then backup="$prefix/backups/fairway.pre-%s"; test ! -e "$backup" || { echo "installation backup already exists" >&2; exit 1; }; cp "$prefix/bin/fairway" "$backup"; fi
install -m 0755 "$tmp/fairway" "$prefix/bin/fairway.new"; mv "$prefix/bin/fairway.new" "$prefix/bin/fairway"
"$prefix/bin/fairway" version
`, strings.TrimPrefix(current.Version, "v"), strings.TrimPrefix(current.Version, "v"))
	rollbackScript := fmt.Sprintf(`#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
prefix=${1:?usage: rollback.sh <absolute-prefix>}
case "$prefix" in /*) ;; *) echo "prefix must be absolute" >&2; exit 1 ;; esac
test "$prefix" != / || { echo "prefix must not be root" >&2; exit 1; }
test ! -L "$prefix" || { echo "prefix must not be a symlink" >&2; exit 1; }
"$root/scripts/verify.sh" >/dev/null
os=$(uname -s | tr '[:upper:]' '[:lower:]'); arch=$(uname -m); case "$arch" in x86_64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) exit 1 ;; esac
archive="$root/releases/rollback/artifacts/fairway_%s_${os}_${arch}.tar.gz"
test -f "$archive" || { echo "rollback archive is missing" >&2; exit 1; }
entries=$(tar -tzf "$archive"); test "$entries" = fairway || { echo "archive content is unsafe" >&2; exit 1; }
mkdir -p "$prefix"; prefix=$(CDPATH= cd "$prefix" && pwd -P)
mkdir -p "$prefix/bin" "$prefix/backups"
test ! -L "$prefix/bin" && test ! -L "$prefix/backups" && test ! -L "$prefix/bin/fairway" || { echo "rollback path contains a symlink" >&2; exit 1; }
tmp=$(mktemp -d "$prefix/.fairway-rollback.XXXXXX"); trap 'rm -rf "$tmp"' EXIT HUP INT TERM
tar -xzf "$archive" -C "$tmp"; test -f "$tmp/fairway" && test ! -L "$tmp/fairway"
if test -f "$prefix/bin/fairway"; then backup="$prefix/backups/fairway.pre-rollback-to-%s"; test ! -e "$backup" || { echo "rollback backup already exists" >&2; exit 1; }; cp "$prefix/bin/fairway" "$backup"; fi
install -m 0755 "$tmp/fairway" "$prefix/bin/fairway.rollback"; mv "$prefix/bin/fairway.rollback" "$prefix/bin/fairway"
"$prefix/bin/fairway" version
`, strings.TrimPrefix(rollback.Version, "v"), strings.TrimPrefix(rollback.Version, "v"))
	for name, body := range map[string]string{"verify.sh": verify, "install.sh": install, "rollback.sh": rollbackScript} {
		if err := os.WriteFile(filepath.Join(root, "scripts", name), []byte(body), 0o700); err != nil {
			return errors.New("write offline lifecycle script")
		}
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func safeDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return "", errors.New("offline bundle directory must be local")
	}
	clean := filepath.Clean(path)
	info, err := os.Lstat(clean)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("offline bundle directory must be a non-symlink directory")
	}
	return clean, nil
}

func readBoundedRegular(path string, max int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > max {
		return nil, errors.New("file is missing, unsafe, or oversized")
	}
	return os.ReadFile(path)
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("offline bundle public key must be base64 Ed25519 public key")
	}
	return ed25519.PublicKey(raw), nil
}

func decodePrivateKey(value string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || (len(raw) != ed25519.PrivateKeySize && len(raw) != ed25519.SeedSize) {
		return nil, errors.New("offline bundle signing key must be base64 Ed25519 seed or private key")
	}
	if len(raw) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(raw), nil
	}
	return ed25519.PrivateKey(raw), nil
}

func validMetadata(name, value string) error {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || value == "" || len(value) > 512 || strings.ContainsAny(value, "\x00\r\n") || containsPrivate(value) {
		return fmt.Errorf("offline bundle %s is invalid", name)
	}
	return nil
}

func containsPrivate(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"authorization: bearer", "api_key=", "client_secret=", "password=", "private_key=", "-----begin private key-----", "-----begin rsa private key-----", "raw_prompt", "raw transcript", "tool_body"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func safeName(value string) bool {
	if value == "" || len(value) > 160 || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._-", r)) {
			return false
		}
	}
	return true
}

func safeRelativePath(value string) bool {
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || filepath.ToSlash(filepath.Clean(value)) != value {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if !safeName(part) {
			return false
		}
	}
	return true
}

func validSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func sameExpected(actual, expected ReleaseIdentity) bool {
	return actual.Version == expected.Version && actual.SourceSHA == expected.SourceSHA && actual.BuilderID == expected.BuilderID && actual.PolicyVersion == expected.PolicyVersion
}

func stableJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("JSON document contains trailing content")
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var parse func() error
	parse = func() error {
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
				if err := parse(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := parse(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	if err := parse(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("JSON document contains trailing content")
	}
	return nil
}
