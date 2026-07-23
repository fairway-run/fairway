package releaserehearsal

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractAssurance(t *testing.T) {
	dir := t.TempDir()
	writeAssuranceArchive(t, filepath.Join(dir, "fairway_"+testVersion+"_release_assurance.tar.gz"), []tarEntry{
		{name: "fairway-" + testVersion + "-release-assurance/", kind: tar.TypeDir},
		{name: "fairway-" + testVersion + "-release-assurance/manifest.json", body: "{}"},
		{name: "fairway-" + testVersion + "-release-assurance/evidence/test.txt", body: "evidence"},
	})
	output := filepath.Join(t.TempDir(), "assurance")
	if err := ExtractAssurance(dir, testVersion, output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(output, "fairway-"+testVersion+"-release-assurance", "evidence", "test.txt"))
	if err != nil || string(data) != "evidence" {
		t.Fatalf("extracted data = %q, %v", data, err)
	}
}

func TestExtractAssuranceRejectsUnsafeArchive(t *testing.T) {
	tests := []struct {
		name  string
		entry tarEntry
		want  string
	}{
		{name: "traversal", entry: tarEntry{name: "../escape", body: "bad"}, want: "unsafe path"},
		{name: "absolute", entry: tarEntry{name: "/tmp/escape", body: "bad"}, want: "unsafe path"},
		{name: "symlink", entry: tarEntry{name: "fairway-" + testVersion + "-release-assurance/link", kind: tar.TypeSymlink, link: "/tmp"}, want: "unsupported entry type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAssuranceArchive(t, filepath.Join(dir, "fairway_"+testVersion+"_release_assurance.tar.gz"), []tarEntry{
				{name: "fairway-" + testVersion + "-release-assurance/", kind: tar.TypeDir},
				{name: "fairway-" + testVersion + "-release-assurance/manifest.json", body: "{}"},
				test.entry,
			})
			output := filepath.Join(t.TempDir(), "assurance")
			err := ExtractAssurance(dir, testVersion, output)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ExtractAssurance error = %v, want %q", err, test.want)
			}
			if _, statErr := os.Lstat(output); !os.IsNotExist(statErr) {
				t.Fatalf("failed extraction retained output: %v", statErr)
			}
		})
	}
}

type tarEntry struct {
	name string
	body string
	kind byte
	link string
}

func writeAssuranceArchive(t *testing.T, archivePath string, entries []tarEntry) {
	t.Helper()
	file, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	for _, entry := range entries {
		kind := entry.kind
		if kind == 0 {
			kind = tar.TypeReg
		}
		header := &tar.Header{Name: entry.name, Typeflag: kind, Mode: 0o600, Linkname: entry.link}
		if kind == tar.TypeReg {
			header.Size = int64(len(entry.body))
		} else if kind == tar.TypeDir {
			header.Mode = 0o700
		}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if kind == tar.TypeReg {
			if _, err := archive.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
