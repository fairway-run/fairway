package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

const maxArtifactViewBytes = 512 * 1024

type EvidenceViewRow struct {
	Result       string
	CommandText  string
	ArtifactPath string
	ArtifactType string
	ViewHref     string
	Viewable     bool
	Label        string
}

type artifactDocument struct {
	TaskID       string
	ArtifactPath string
	ArtifactType string
	Label        string
	Body         string
}

func evidenceViewRows(taskID string, evidence []store.Evidence, roots []string) []EvidenceViewRow {
	rows := make([]EvidenceViewRow, 0, len(evidence))
	for _, ev := range evidence {
		row := EvidenceViewRow{
			Result:       ev.Result,
			CommandText:  ev.CommandText,
			ArtifactPath: ev.ArtifactPath,
			ArtifactType: ev.ArtifactType,
			Label:        evidenceArtifactLabel(ev.ArtifactPath),
		}
		if strings.TrimSpace(ev.ArtifactPath) != "" && len(roots) > 0 && !artifactLooksRemote(ev.ArtifactPath) {
			values := url.Values{}
			values.Set("task", taskID)
			values.Set("path", ev.ArtifactPath)
			row.ViewHref = "/evidence/artifact?" + values.Encode()
			row.Viewable = true
		}
		rows = append(rows, row)
	}
	return rows
}

func (s *Server) artifact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := strings.TrimSpace(r.URL.Query().Get("task"))
	artifactPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if taskID == "" || artifactPath == "" {
		http.Error(w, "task and path are required", http.StatusBadRequest)
		return
	}
	task, _, evidence, _, _, err := s.store.TaskDetail(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var matched store.Evidence
	found := false
	for _, ev := range evidence {
		if ev.ArtifactPath == artifactPath {
			matched = ev
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "artifact is not recorded evidence for this task", http.StatusForbidden)
		return
	}
	doc, err := s.openArtifactDocument(task.Definition.ID, matched)
	if err != nil {
		status := http.StatusForbidden
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>Evidence artifact %s</title><style>body{font-family:system-ui,sans-serif;margin:2rem;line-height:1.5}pre{white-space:pre-wrap;background:#f6f8fa;padding:1rem;border-radius:6px;overflow:auto}.badge{display:inline-block;padding:.2rem .45rem;border:1px solid #999;border-radius:999px;font-size:.8rem}</style></head><body>", html.EscapeString(doc.ArtifactPath))
	fmt.Fprintf(w, "<p><a href=\"/tasks/%s\">Back to task</a></p>", url.PathEscape(doc.TaskID))
	fmt.Fprintf(w, "<h1>Evidence Artifact</h1><p><span class=\"badge\">%s</span> <span class=\"badge\">redacted viewer</span> <span class=\"badge\">%s</span></p>", html.EscapeString(doc.Label), html.EscapeString(firstNonEmpty(doc.ArtifactType, "unknown")))
	fmt.Fprintf(w, "<p><b>Task:</b> %s<br><b>Path:</b> <code>%s</code></p>", html.EscapeString(doc.TaskID), html.EscapeString(doc.ArtifactPath))
	fmt.Fprintf(w, "<pre>%s</pre>", html.EscapeString(doc.Body))
	fmt.Fprint(w, "</body></html>")
}

func (s *Server) openArtifactDocument(taskID string, ev store.Evidence) (artifactDocument, error) {
	if len(s.cfg.Fairway.LocalArtifactPaths) == 0 {
		return artifactDocument{}, errors.New("artifact viewer is disabled because fairway.local_artifact_paths is empty")
	}
	resolved, err := resolveArtifactPath(s.root, ev.ArtifactPath, s.cfg.Fairway.LocalArtifactPaths)
	if err != nil {
		return artifactDocument{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return artifactDocument{}, err
	}
	if info.IsDir() {
		return artifactDocument{}, errors.New("artifact path is a directory")
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return artifactDocument{}, err
	}
	if strings.EqualFold(filepath.Ext(resolved), ".json") && json.Valid(data) {
		var v any
		if err := json.Unmarshal(data, &v); err == nil {
			if pretty, err := json.MarshalIndent(v, "", "  "); err == nil {
				return artifactDocument{TaskID: taskID, ArtifactPath: ev.ArtifactPath, ArtifactType: ev.ArtifactType, Label: artifactBoundaryLabel(ev.ArtifactPath), Body: truncateArtifactView(redactArtifactContent(string(pretty)))}, nil
			}
		}
	}
	return artifactDocument{TaskID: taskID, ArtifactPath: ev.ArtifactPath, ArtifactType: ev.ArtifactType, Label: artifactBoundaryLabel(ev.ArtifactPath), Body: truncateArtifactView(redactArtifactContent(string(data)))}, nil
}

func resolveArtifactPath(root, artifactPath string, roots []string) (string, error) {
	if artifactLooksRemote(artifactPath) {
		return "", errors.New("remote artifact URLs are not served by the local artifact viewer")
	}
	cleanArtifact := filepath.Clean(artifactPath)
	if strings.HasPrefix(cleanArtifact, ".."+string(filepath.Separator)) || cleanArtifact == ".." {
		return "", errors.New("artifact path traversal is not allowed")
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	candidate := cleanArtifact
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	for _, rawRoot := range roots {
		if strings.TrimSpace(rawRoot) == "" {
			continue
		}
		allowed := filepath.Clean(rawRoot)
		if !filepath.IsAbs(allowed) {
			allowed = filepath.Join(root, allowed)
		}
		realAllowed, err := filepath.EvalSymlinks(allowed)
		if err != nil {
			continue
		}
		if pathWithin(realCandidate, realAllowed) {
			return realCandidate, nil
		}
	}
	return "", errors.New("artifact path is outside configured local_artifact_paths")
}

func pathWithin(candidate, root string) bool {
	if candidate == root {
		return true
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

func artifactLooksRemote(path string) bool {
	parsed, err := url.Parse(path)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func evidenceArtifactLabel(path string) string {
	if path == "" {
		return ""
	}
	if artifactLooksRemote(path) {
		return "internal-only"
	}
	return artifactBoundaryLabel(path)
}

func artifactBoundaryLabel(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "public") || strings.Contains(lower, "publish"):
		return "publishable"
	case strings.Contains(lower, "internal") || strings.Contains(lower, "private"):
		return "internal-only"
	default:
		return "local-only"
	}
}

var artifactRedactors = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`), `${1}<redacted>`},
	{regexp.MustCompile(`(?i)((?:access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|ssh[_-]?private[_-]?key|api[_-]?key|token|secret|password|cookie|set-cookie)=)([^&\s"']+)`), `${1}<redacted>`},
	{regexp.MustCompile(`(?i)("(?:access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|ssh[_-]?private[_-]?key|api[_-]?key|token|secret|password|cookie|set-cookie)"\s*:\s*")([^"]+)(")`), `${1}<redacted>${3}`},
	{regexp.MustCompile(`(?i)((?:authorization|cookie|set-cookie):\s*)([^\r\n]+)`), `${1}<redacted>`},
	{regexp.MustCompile(`https?://(?:localhost|127\.0\.0\.1|10\.[^\s"'<>)]+|192\.168\.[^\s"'<>)]+|172\.(?:1[6-9]|2[0-9]|3[01])\.[^\s"'<>)]+|100\.(?:6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.[^\s"'<>)]+|[^/\s"'<>)]+\.(?:local|internal))[^\s"'<>)]*`), `<redacted-internal-url>`},
}

func redactArtifactContent(raw string) string {
	redacted := raw
	for _, redactor := range artifactRedactors {
		redacted = redactor.re.ReplaceAllString(redacted, redactor.repl)
	}
	return redacted
}

func truncateArtifactView(body string) string {
	if len(body) <= maxArtifactViewBytes {
		return body
	}
	return body[:maxArtifactViewBytes] + "\n\n[truncated by Fairway artifact viewer after redaction]"
}
