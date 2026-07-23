package releaserehearsal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testVersion = "v1.2.3"
	testSHA     = "0123456789abcdef0123456789abcdef01234567"
	testBuilder = "fairway-run/fairway/.github/workflows/release-rehearsal.yml@refs/heads/main"
	testPolicy  = "sovereign-release-v1"
)

func TestCreateAndVerify(t *testing.T) {
	dir := makePacketAssets(t)
	manifest, err := Create(dir, testVersion, testSHA, testBuilder, testPolicy, "2026-07-23T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Assets) != 7 {
		t.Fatalf("asset count = %d, want 7", len(manifest.Assets))
	}
	report, err := Verify(VerifyOptions{
		Dir:                   dir,
		ExpectedVersion:       testVersion,
		ExpectedSourceSHA:     testSHA,
		ExpectedBuilderID:     testBuilder,
		ExpectedPolicyVersion: testPolicy,
	})
	if err != nil || !report.OK || report.AssetCount != 7 {
		t.Fatalf("verify = %#v, %v", report, err)
	}
}

func TestVerifyRejectsTamperUnexpectedAndIdentityMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, dir string)
		want   string
	}{
		{
			name: "archive tamper",
			mutate: func(t *testing.T, dir string) {
				t.Helper()
				appendFile(t, filepath.Join(dir, "fairway_1.2.3_linux_arm64.tar.gz"), "tamper")
			},
			want: "digest or size mismatch",
		},
		{
			name: "unexpected file",
			mutate: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "raw-secret.log"), "not retained")
			},
			want: "unexpected rehearsal packet entry",
		},
		{
			name: "symlink asset",
			mutate: func(t *testing.T, dir string) {
				t.Helper()
				path := filepath.Join(dir, "fairway_1.2.3_linux_arm64.tar.gz")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "fairway_1.2.3_linux_amd64.tar.gz"), path); err != nil {
					t.Fatal(err)
				}
			},
			want: "regular non-symlink",
		},
		{
			name: "wrong identity",
			mutate: func(t *testing.T, dir string) {
				t.Helper()
			},
			want: "source SHA mismatch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := makePacketAssets(t)
			if _, err := Create(dir, testVersion, testSHA, testBuilder, testPolicy, "2026-07-23T12:00:00Z"); err != nil {
				t.Fatal(err)
			}
			test.mutate(t, dir)
			expectedSHA := testSHA
			if test.name == "wrong identity" {
				expectedSHA = strings.Repeat("a", 40)
			}
			report, err := Verify(VerifyOptions{Dir: dir, ExpectedVersion: testVersion, ExpectedSourceSHA: expectedSHA, ExpectedBuilderID: testBuilder, ExpectedPolicyVersion: testPolicy})
			if err == nil || report.OK || !strings.Contains(strings.Join(report.Issues, "; "), test.want) {
				t.Fatalf("verify = %#v, %v; want %q", report, err, test.want)
			}
		})
	}
}

func TestCreateRejectsExistingManifestAndInvalidIdentity(t *testing.T) {
	dir := makePacketAssets(t)
	if _, err := Create(dir, testVersion, testSHA, testBuilder, testPolicy, "2026-07-23T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(dir, testVersion, testSHA, testBuilder, testPolicy, "2026-07-23T12:00:00Z"); err == nil || !strings.Contains(err.Error(), "file exists") {
		t.Fatalf("second create error = %v", err)
	}
	dir = makePacketAssets(t)
	if _, err := Create(dir, "1.2.3", testSHA, testBuilder, testPolicy, "2026-07-23T12:00:00Z"); err == nil || !strings.Contains(err.Error(), "vX.Y.Z") {
		t.Fatalf("bad version error = %v", err)
	}
}

func TestVerifyRejectsUnknownManifestField(t *testing.T) {
	dir := makePacketAssets(t)
	if _, err := Create(dir, testVersion, testSHA, testBuilder, testPolicy, "2026-07-23T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ManifestName)
	var value map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	value["authorization"] = "publish"
	data, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(VerifyOptions{Dir: dir}); err == nil || !strings.Contains(err.Error(), "decode rehearsal manifest") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func makePacketAssets(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	normalized := strings.TrimPrefix(testVersion, "v")
	archives := []string{
		"fairway_" + normalized + "_darwin_amd64.tar.gz",
		"fairway_" + normalized + "_darwin_arm64.tar.gz",
		"fairway_" + normalized + "_linux_amd64.tar.gz",
		"fairway_" + normalized + "_linux_arm64.tar.gz",
	}
	var checksumLines []string
	for _, name := range archives {
		content := "archive:" + name
		writeFile(t, filepath.Join(dir, name), content)
		checksumLines = append(checksumLines, digest(content)+"  "+name)
	}
	writeFile(t, filepath.Join(dir, "fairway_"+normalized+"_checksums.txt"), strings.Join(checksumLines, "\n")+"\n")
	assurance := "signed assurance"
	assuranceName := "fairway_" + testVersion + "_release_assurance.tar.gz"
	writeFile(t, filepath.Join(dir, assuranceName), assurance)
	writeFile(t, filepath.Join(dir, assuranceName+".sha256"), digest(assurance)+"  "+assuranceName+"\n")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
