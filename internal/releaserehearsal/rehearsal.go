package releaserehearsal

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	Schema       = "fairway.release-rehearsal.v1"
	ManifestName = "rehearsal.json"
)

var (
	versionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	shaPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type Asset struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Manifest struct {
	Schema        string  `json:"schema"`
	Version       string  `json:"version"`
	SourceSHA     string  `json:"source_sha"`
	BuilderID     string  `json:"builder_id"`
	PolicyVersion string  `json:"policy_version"`
	Result        string  `json:"result"`
	CreatedAt     string  `json:"created_at"`
	Assets        []Asset `json:"assets"`
}

type VerifyOptions struct {
	Dir                   string
	ExpectedVersion       string
	ExpectedSourceSHA     string
	ExpectedBuilderID     string
	ExpectedPolicyVersion string
}

type VerifyReport struct {
	OK            bool     `json:"ok"`
	Schema        string   `json:"schema"`
	Version       string   `json:"version"`
	SourceSHA     string   `json:"source_sha"`
	BuilderID     string   `json:"builder_id"`
	PolicyVersion string   `json:"policy_version"`
	Result        string   `json:"result"`
	AssetCount    int      `json:"asset_count"`
	Issues        []string `json:"issues,omitempty"`
}

func Create(dir, version, sourceSHA, builderID, policyVersion, createdAt string) (Manifest, error) {
	dir, err := validateDirectory(dir)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateIdentity(version, sourceSHA, builderID, policyVersion); err != nil {
		return Manifest{}, err
	}
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
		return Manifest{}, errors.New("created-at must be RFC3339")
	}

	names := expectedAssetNames(version)
	assets := make([]Asset, 0, len(names))
	for _, name := range names {
		asset, err := inspectAsset(dir, name)
		if err != nil {
			return Manifest{}, err
		}
		assets = append(assets, asset)
	}
	if err := rejectUnexpectedFiles(dir, append(names, ManifestName)); err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		Schema:        Schema,
		Version:       version,
		SourceSHA:     sourceSHA,
		BuilderID:     builderID,
		PolicyVersion: policyVersion,
		Result:        "pass",
		CreatedAt:     createdAt,
		Assets:        assets,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	data = append(data, '\n')
	path := filepath.Join(dir, ManifestName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Manifest{}, fmt.Errorf("create rehearsal manifest: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return Manifest{}, errors.New("write rehearsal manifest")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return Manifest{}, errors.New("close rehearsal manifest")
	}
	return manifest, nil
}

func Verify(options VerifyOptions) (VerifyReport, error) {
	dir, err := validateDirectory(options.Dir)
	if err != nil {
		return VerifyReport{}, err
	}
	manifestPath := filepath.Join(dir, ManifestName)
	info, err := os.Lstat(manifestPath)
	if err != nil {
		return VerifyReport{}, errors.New("rehearsal manifest is missing")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return VerifyReport{}, errors.New("rehearsal manifest must be a regular non-symlink file")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return VerifyReport{}, errors.New("read rehearsal manifest")
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return VerifyReport{}, fmt.Errorf("decode rehearsal manifest: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return VerifyReport{}, errors.New("rehearsal manifest contains trailing JSON")
	}

	report := VerifyReport{
		OK:            true,
		Schema:        manifest.Schema,
		Version:       manifest.Version,
		SourceSHA:     manifest.SourceSHA,
		BuilderID:     manifest.BuilderID,
		PolicyVersion: manifest.PolicyVersion,
		Result:        manifest.Result,
		AssetCount:    len(manifest.Assets),
	}
	if manifest.Schema != Schema {
		report.Issues = append(report.Issues, "unexpected rehearsal schema")
	}
	if err := validateIdentity(manifest.Version, manifest.SourceSHA, manifest.BuilderID, manifest.PolicyVersion); err != nil {
		report.Issues = append(report.Issues, err.Error())
	}
	if manifest.Result != "pass" {
		report.Issues = append(report.Issues, "rehearsal result is not pass")
	}
	if _, err := time.Parse(time.RFC3339, manifest.CreatedAt); err != nil {
		report.Issues = append(report.Issues, "rehearsal created_at is not RFC3339")
	}
	checkExpected("version", manifest.Version, options.ExpectedVersion, &report)
	checkExpected("source SHA", manifest.SourceSHA, options.ExpectedSourceSHA, &report)
	checkExpected("builder ID", manifest.BuilderID, options.ExpectedBuilderID, &report)
	checkExpected("policy version", manifest.PolicyVersion, options.ExpectedPolicyVersion, &report)

	expectedNames := expectedAssetNames(manifest.Version)
	expectedSet := make(map[string]bool, len(expectedNames))
	for _, name := range expectedNames {
		expectedSet[name] = true
	}
	seen := map[string]bool{}
	for _, recorded := range manifest.Assets {
		if seen[recorded.Name] {
			report.Issues = append(report.Issues, "duplicate rehearsal asset: "+recorded.Name)
			continue
		}
		seen[recorded.Name] = true
		if !expectedSet[recorded.Name] {
			report.Issues = append(report.Issues, "unexpected rehearsal asset: "+recorded.Name)
			continue
		}
		actual, err := inspectAsset(dir, recorded.Name)
		if err != nil {
			report.Issues = append(report.Issues, err.Error())
			continue
		}
		if actual.SHA256 != recorded.SHA256 || actual.Bytes != recorded.Bytes {
			report.Issues = append(report.Issues, "rehearsal asset digest or size mismatch: "+recorded.Name)
		}
	}
	for _, name := range expectedNames {
		if !seen[name] {
			report.Issues = append(report.Issues, "missing rehearsal asset: "+name)
		}
	}
	if err := rejectUnexpectedFiles(dir, append(expectedNames, ManifestName)); err != nil {
		report.Issues = append(report.Issues, err.Error())
	}
	if err := verifyGoReleaserChecksums(dir, manifest.Version); err != nil {
		report.Issues = append(report.Issues, err.Error())
	}
	if err := verifyAssuranceChecksum(dir, manifest.Version); err != nil {
		report.Issues = append(report.Issues, err.Error())
	}
	report.OK = len(report.Issues) == 0
	if !report.OK {
		return report, errors.New("release rehearsal verification failed")
	}
	return report, nil
}

func validateIdentity(version, sourceSHA, builderID, policyVersion string) error {
	if !versionPattern.MatchString(version) {
		return errors.New("version must match vX.Y.Z")
	}
	if !shaPattern.MatchString(sourceSHA) {
		return errors.New("source SHA must be a lowercase 40-character commit")
	}
	if strings.TrimSpace(builderID) == "" {
		return errors.New("builder ID is required")
	}
	if strings.TrimSpace(policyVersion) == "" {
		return errors.New("policy version is required")
	}
	return nil
}

func validateDirectory(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("rehearsal directory is required")
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.New("resolve rehearsal directory")
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", errors.New("rehearsal directory is missing")
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("rehearsal directory must be a non-symlink directory")
	}
	return absolute, nil
}

func expectedAssetNames(version string) []string {
	normalized := strings.TrimPrefix(version, "v")
	return []string{
		"fairway_" + normalized + "_checksums.txt",
		"fairway_" + normalized + "_darwin_amd64.tar.gz",
		"fairway_" + normalized + "_darwin_arm64.tar.gz",
		"fairway_" + normalized + "_linux_amd64.tar.gz",
		"fairway_" + normalized + "_linux_arm64.tar.gz",
		"fairway_" + version + "_release_assurance.tar.gz",
		"fairway_" + version + "_release_assurance.tar.gz.sha256",
	}
}

func inspectAsset(dir, name string) (Asset, error) {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return Asset{}, errors.New("rehearsal asset name must be a basename")
	}
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil {
		return Asset{}, errors.New("missing rehearsal asset: " + name)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Asset{}, errors.New("rehearsal asset must be a regular non-symlink file: " + name)
	}
	file, err := os.Open(path)
	if err != nil {
		return Asset{}, errors.New("open rehearsal asset: " + name)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return Asset{}, errors.New("inspect opened rehearsal asset: " + name)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return Asset{}, errors.New("rehearsal asset changed during custody validation: " + name)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return Asset{}, errors.New("hash rehearsal asset: " + name)
	}
	return Asset{Name: name, SHA256: hex.EncodeToString(hash.Sum(nil)), Bytes: openedInfo.Size()}, nil
}

func rejectUnexpectedFiles(dir string, allowed []string) error {
	allow := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allow[name] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.New("read rehearsal directory")
	}
	for _, entry := range entries {
		if !allow[entry.Name()] {
			return errors.New("unexpected rehearsal packet entry: " + entry.Name())
		}
	}
	return nil
}

func checkExpected(label, actual, expected string, report *VerifyReport) {
	if expected != "" && actual != expected {
		report.Issues = append(report.Issues, fmt.Sprintf("%s mismatch: got %q want %q", label, actual, expected))
	}
}

func verifyGoReleaserChecksums(dir, version string) error {
	normalized := strings.TrimPrefix(version, "v")
	path := filepath.Join(dir, "fairway_"+normalized+"_checksums.txt")
	file, err := os.Open(path)
	if err != nil {
		return errors.New("open GoReleaser checksum file")
	}
	defer file.Close()
	want := map[string]bool{}
	for _, suffix := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"} {
		want["fairway_"+normalized+"_"+suffix+".tar.gz"] = true
	}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || len(fields[0]) != 64 {
			return errors.New("GoReleaser checksum file has invalid format")
		}
		name := fields[1]
		if !want[name] || seen[name] {
			return errors.New("GoReleaser checksum file has unexpected or duplicate asset: " + name)
		}
		actual, err := inspectAsset(dir, name)
		if err != nil {
			return err
		}
		if actual.SHA256 != fields[0] {
			return errors.New("GoReleaser checksum mismatch: " + name)
		}
		seen[name] = true
	}
	if err := scanner.Err(); err != nil {
		return errors.New("read GoReleaser checksum file")
	}
	if len(seen) != len(want) {
		return errors.New("GoReleaser checksum file does not cover four archives")
	}
	return nil
}

func verifyAssuranceChecksum(dir, version string) error {
	name := "fairway_" + version + "_release_assurance.tar.gz"
	data, err := os.ReadFile(filepath.Join(dir, name+".sha256"))
	if err != nil {
		return errors.New("read assurance checksum")
	}
	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) != 2 || fields[1] != name {
		return errors.New("assurance checksum must use the portable asset basename")
	}
	actual, err := inspectAsset(dir, name)
	if err != nil {
		return err
	}
	if fields[0] != actual.SHA256 {
		return errors.New("assurance checksum mismatch")
	}
	return nil
}
