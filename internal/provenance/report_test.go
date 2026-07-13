package provenance

import (
	"net/url"
	"strings"
	"testing"
)

func TestPublicProvenancePathSanitization(t *testing.T) {
	tests := []struct {
		name  string
		root  string
		value string
		want  string
		warn  bool
	}{
		{name: "unix inside", root: "/Users/alice/src/fairway", value: "/Users/alice/src/fairway/.fairway/config.toml", want: ".fairway/config.toml"},
		{name: "unix outside", root: "/Users/alice/src/fairway", value: "/tmp/fairway-release", want: redactedLocalPath, warn: true},
		{name: "relative", root: "/Users/alice/src/fairway", value: `artifacts/report.json`, want: "artifacts/report.json"},
		{name: "relative windows separators", root: "/Users/alice/src/fairway", value: `artifacts\report.json`, want: "artifacts/report.json"},
		{name: "relative traversal", root: "/Users/alice/src/fairway", value: "../private/report.json", want: redactedLocalPath, warn: true},
		{name: "windows inside", root: `C:\Users\alice\fairway`, value: `c:\users\alice\fairway\docs\report.md`, want: "docs/report.md"},
		{name: "windows outside", root: `C:\Users\alice\fairway`, value: `D:\private\report.md`, want: redactedLocalPath, warn: true},
		{name: "windows UNC outside", root: `C:\Users\alice\fairway`, value: `\\server\share\report.md`, want: redactedLocalPath, warn: true},
		{name: "windows device outside", root: `C:\Users\alice\fairway`, value: `\\?\C:\private\report.md`, want: redactedLocalPath, warn: true},
		{name: "windows dot device outside", root: `C:\Users\alice\fairway`, value: `\\.\device\report.md`, want: redactedLocalPath, warn: true},
		{name: "file url", root: "/Users/alice/src/fairway", value: "file:///Users/alice/private.txt", want: redactedLocalPath, warn: true},
		{name: "remote url", root: "/Users/alice/src/fairway", value: "https://example.invalid/report.json", want: "https://example.invalid/report.json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var warnings []string
			got := publicProvenancePath(tc.root, tc.value, &warnings, tc.name)
			if got != tc.want {
				t.Fatalf("publicProvenancePath(%q, %q) = %q, want %q", tc.root, tc.value, got, tc.want)
			}
			if (len(warnings) > 0) != tc.warn {
				t.Fatalf("warnings = %v, want warning=%t", warnings, tc.warn)
			}
		})
	}
}

func TestRedactPublicTextRemovesLocalAbsolutePaths(t *testing.T) {
	root := "/Users/alice/src/fairway"
	var warnings []string
	got := redactPublicText(`go test /Users/alice/src/fairway/internal/provenance --sibling=/Users/alice/src/fairway-private/secret inspect [/Users/alice/bracket] {/Users/alice/brace} value,/Users/alice/comma next;/Users/alice/semicolon --unc=\\server\share\private --device=\\?\C:\private\trace --dot-device=\\.\device\private --out=/tmp/private.json --remote=https://example.invalid/report`, root, &warnings, "T-001", "command")
	if strings.Contains(got, root) || strings.Contains(got, "/tmp/private.json") || strings.Contains(got, "fairway-private") || strings.Contains(got, `server\share`) || strings.Contains(got, `C:\private`) {
		t.Fatalf("local path leaked: %s", got)
	}
	for _, want := range []string{"./internal/provenance", "--sibling=" + redactedLocalPath, "[" + redactedLocalPath, "{" + redactedLocalPath, "," + redactedLocalPath, ";" + redactedLocalPath, "--unc=" + redactedLocalPath, "--device=" + redactedLocalPath, "--dot-device=" + redactedLocalPath, "--out=" + redactedLocalPath, "https://example.invalid/report"} {
		if !strings.Contains(got, want) {
			t.Fatalf("redacted text %q missing %q", got, want)
		}
	}
	if len(warnings) == 0 {
		t.Fatal("expected local path redaction warning")
	}
}

func TestPublicProvenanceURLSanitizesMetadata(t *testing.T) {
	root := "/Users/alice/src/fairway"
	var warnings []string
	got := publicProvenancePath(root, "https://user:password@example.invalid/report?Access%5FToken=abc&inside=/Users/alice/src/fairway/report.json&outside=/tmp/private.json#C:\\private\\trace", &warnings, "artifact")
	for _, leaked := range []string{"user", "password", root, "/tmp/private.json", `C:\private\trace`} {
		if strings.Contains(got, leaked) {
			t.Fatalf("URL %q leaked %q", got, leaked)
		}
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.User != nil {
		t.Fatalf("userinfo retained: %s", got)
	}
	if parsed.Query().Get("Access_Token") != "<redacted>" || parsed.Query().Get("inside") != "./report.json" || parsed.Query().Get("outside") != redactedLocalPath || parsed.Fragment != redactedLocalPath {
		t.Fatalf("unexpected sanitized URL: %s", got)
	}
	if len(warnings) == 0 {
		t.Fatal("expected URL metadata redaction warnings")
	}
}
