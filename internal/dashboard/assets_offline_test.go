package dashboard

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func TestEmbeddedDashboardHasNoRemoteAssetDependencies(t *testing.T) {
	remoteAttribute := regexp.MustCompile(`(?i)(src|href)\s*=\s*["'](?:https?:)?//`)
	remoteCSS := regexp.MustCompile(`(?i)(@import\s+["'](?:https?:)?//|url\(\s*["']?(?:https?:)?//)`)
	err := fs.WalkDir(dashboardAssets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := dashboardAssets.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		if remoteAttribute.MatchString(content) || remoteCSS.MatchString(content) {
			t.Errorf("embedded dashboard asset %s references a remote resource", path)
		}
		if strings.HasSuffix(path, ".js") {
			for _, marker := range []string{`fetch("http`, `fetch('http`, `new WebSocket("ws`, `new WebSocket('ws`} {
				if strings.Contains(content, marker) {
					t.Errorf("embedded dashboard script %s contains remote fetch marker %q", path, marker)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
