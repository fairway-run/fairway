package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ManifestOptions struct {
	Paths       []string
	GeneratedAt time.Time
}

type Manifest struct {
	Schema      string          `json:"schema"`
	GeneratedAt string          `json:"generated_at"`
	Algorithm   string          `json:"algorithm"`
	OK          bool            `json:"ok"`
	Entries     []ManifestEntry `json:"entries"`
	Issues      []string        `json:"issues,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
	Privacy     Privacy         `json:"privacy"`
}

type ManifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
	Status string `json:"status"`
}

func BuildManifest(opts ManifestOptions) (Manifest, error) {
	generated := opts.GeneratedAt
	if generated.IsZero() {
		generated = time.Now().UTC()
	}
	manifest := Manifest{
		Schema:      "fairway.provenance-manifest.v1",
		GeneratedAt: generated.Format(time.RFC3339),
		Algorithm:   "sha256",
		OK:          true,
		Privacy:     defaultPrivacy(),
	}
	if len(opts.Paths) == 0 {
		manifest.OK = false
		manifest.Issues = append(manifest.Issues, "at least one --path is required")
		return manifest, nil
	}
	paths := append([]string{}, opts.Paths...)
	sort.Strings(paths)
	seen := map[string]bool{}
	for _, raw := range paths {
		clean := filepath.Clean(strings.TrimSpace(raw))
		if clean == "." || clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true
		entry := ManifestEntry{Path: clean}
		if suspiciousEvidencePath(clean) {
			entry.Status = "privacy_rejected"
			manifest.OK = false
			manifest.Issues = append(manifest.Issues, fmt.Sprintf("refusing suspicious evidence path %s", clean))
			manifest.Entries = append(manifest.Entries, entry)
			continue
		}
		info, err := os.Stat(clean)
		if err != nil {
			entry.Status = "missing"
			manifest.OK = false
			manifest.Issues = append(manifest.Issues, fmt.Sprintf("missing artifact %s", clean))
			manifest.Entries = append(manifest.Entries, entry)
			continue
		}
		if info.IsDir() {
			entry.Status = "directory_rejected"
			manifest.OK = false
			manifest.Issues = append(manifest.Issues, fmt.Sprintf("refusing directory artifact %s", clean))
			manifest.Entries = append(manifest.Entries, entry)
			continue
		}
		sum, bytesRead, err := hashFile(clean)
		if err != nil {
			entry.Status = "read_failed"
			manifest.OK = false
			manifest.Issues = append(manifest.Issues, fmt.Sprintf("failed to hash artifact %s: %v", clean, err))
			manifest.Entries = append(manifest.Entries, entry)
			continue
		}
		entry.Status = "hashed"
		entry.SHA256 = sum
		entry.Bytes = bytesRead
		manifest.Entries = append(manifest.Entries, entry)
	}
	if len(manifest.Entries) == 0 {
		manifest.OK = false
		manifest.Issues = append(manifest.Issues, "no artifact paths selected")
	}
	sort.Strings(manifest.Issues)
	sort.Strings(manifest.Warnings)
	return manifest, nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(hash.Sum(nil)), n, nil
}

func suspiciousEvidencePath(path string) bool {
	lower := strings.ToLower(path)
	for _, marker := range []string{"secret", "token", "password", "credential", "private-key", "apikey", "api_key"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
