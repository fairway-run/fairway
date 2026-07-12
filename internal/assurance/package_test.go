package assurance

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestExportPackageDeterministicSignedAndMetadataOnly(t *testing.T) {
	profile := mappingProfile()
	report := BuildReadiness(profile, "task_set", []EvidenceMap{{TaskID: "T-1", Applicable: true, EvaluatedAt: "2026-07-12T12:00:00Z",
		Facts: []EvidenceFact{{Reference: "evidence:T-1:1", Class: "evidence", Result: "pass", Timestamp: "2026-07-12T11:00:00Z", State: "current"}}}})
	report.ScopeID = "assurance-core"
	maps := []EvidenceMap{{Project: "demo", TaskID: "T-1", Applicable: true, EvaluatedAt: report.EvaluatedAt,
		Facts: []EvidenceFact{
			{Reference: "evidence:T-1:1", Class: "evidence", Result: "pass", Timestamp: "2026-07-12T11:00:00Z", Project: "demo", TaskID: "T-1", State: "current"},
			{Reference: "decision:T-1:1", Class: "decision", Result: "accepted", Timestamp: "2026-07-12T10:00:00Z", Project: "demo", TaskID: "T-1", State: "current"},
			{Reference: "review:T-1:arch:1", Class: "review", Result: "approve", Timestamp: "2026-07-12T10:30:00Z", Project: "demo", TaskID: "T-1", State: "current"},
		}}}
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	key := base64.StdEncoding.EncodeToString(seed)
	created := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	manifest, err := ExportPackage(PackageOptions{Profile: profile, Readiness: report, Maps: maps, OutputDirectory: first, CreatedAt: created, SigningKeyBase64: key})
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.Signed || manifest.Schema != PackageManifestSchema {
		t.Fatalf("manifest=%+v", manifest)
	}
	if !strings.HasPrefix(manifest.SigningKeyID, "sha256:") {
		t.Fatalf("missing signing key identity: %+v", manifest)
	}
	if _, err := ExportPackage(PackageOptions{Profile: profile, Readiness: report, Maps: maps, OutputDirectory: second, CreatedAt: created, SigningKeyBase64: key}); err != nil {
		t.Fatal(err)
	}
	firstFiles := readPackageFiles(t, first)
	secondFiles := readPackageFiles(t, second)
	if !reflect.DeepEqual(firstFiles, secondFiles) {
		t.Fatal("repeated package export was not byte-stable")
	}
	joined := string(joinPackageFiles(firstFiles))
	for _, forbidden := range []string{"raw command text", "artifact/body", "private transcript"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("package leaked %q", forbidden)
		}
	}
	if !strings.Contains(joined, `"not_oscal_document": true`) {
		t.Fatal("missing OSCAL compatibility boundary")
	}
	if !strings.Contains(string(firstFiles["VERIFY.md"]), "fairway assurance package verify") {
		t.Fatal("package is missing offline verifier instructions")
	}

	var signature PackageSignature
	if err := json.Unmarshal(firstFiles["signature.json"], &signature); err != nil {
		t.Fatal(err)
	}
	publicKey, _ := base64.StdEncoding.DecodeString(signature.PublicKey)
	sig, _ := base64.StdEncoding.DecodeString(signature.Signature)
	digest, _ := hexDecode(signature.ManifestSHA256)
	if !ed25519.Verify(publicKey, digest, sig) {
		t.Fatal("package signature did not verify")
	}
}

func TestExportPackageFailsClosed(t *testing.T) {
	profile := mappingProfile()
	report := BuildReadiness(profile, "project", []EvidenceMap{{TaskID: "T-1", Applicable: true, EvaluatedAt: "2026-07-12T12:00:00Z"}})
	report.ScopeID = "demo"
	base := t.TempDir()
	existing := filepath.Join(base, "existing")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportPackage(PackageOptions{Profile: profile, Readiness: report, OutputDirectory: existing, CreatedAt: time.Now()}); err == nil || !strings.Contains(err.Error(), "must not already exist") {
		t.Fatalf("existing output error=%v", err)
	}
	profile.Controls[0].Objective = "Product is certified."
	if _, err := ExportPackage(PackageOptions{Profile: profile, Readiness: report, OutputDirectory: filepath.Join(base, "claims"), CreatedAt: time.Now()}); err == nil || !strings.Contains(err.Error(), "prohibited generated") {
		t.Fatalf("claim error=%v", err)
	}
	profile = mappingProfile()
	if _, err := ExportPackage(PackageOptions{Profile: profile, Readiness: report, OutputDirectory: filepath.Join(base, "key"), CreatedAt: time.Now(), SigningKeyBase64: "SHOULD_NOT_RENDER"}); err == nil || strings.Contains(err.Error(), "SHOULD_NOT_RENDER") {
		t.Fatalf("key error=%v", err)
	}
	maps := []EvidenceMap{{Project: "one", TaskID: "T-1"}, {Project: "two", TaskID: "T-2"}}
	if _, err := ExportPackage(PackageOptions{Profile: profile, Readiness: report, Maps: maps, OutputDirectory: filepath.Join(base, "mixed"), CreatedAt: time.Now()}); err == nil || !strings.Contains(err.Error(), "cannot mix source projects") {
		t.Fatalf("mixed project error=%v", err)
	}
}

func readPackageFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string][]byte{}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = data
	}
	return result
}

func joinPackageFiles(files map[string][]byte) []byte {
	var out []byte
	for _, data := range files {
		out = append(out, data...)
	}
	return out
}

func hexDecode(value string) ([]byte, error) {
	const digits = "0123456789abcdef"
	if len(value)%2 != 0 {
		return nil, os.ErrInvalid
	}
	result := make([]byte, len(value)/2)
	for i := range result {
		hi, lo := strings.IndexByte(digits, value[i*2]), strings.IndexByte(digits, value[i*2+1])
		if hi < 0 || lo < 0 {
			return nil, os.ErrInvalid
		}
		result[i] = byte(hi<<4 | lo)
	}
	return result, nil
}
