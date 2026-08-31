package knowledge

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	bundleSchema      = "fairway.knowledge-bundle.v1"
	maxBundleBytes    = 32 << 20
	maxBundleFiles    = 2048
	bundleManifest    = "manifest.json"
	bundlePagePrefix  = "knowledge/"
	bundleIndexPrefix = "derived/"
)

type bundleManifestDocument struct {
	Schema             string               `json:"schema"`
	CreatedAt          string               `json:"created_at"`
	RepositoryRevision string               `json:"repository_revision"`
	ExternalUntrusted  bool                 `json:"external_untrusted"`
	Files              []bundleManifestFile `json:"files"`
}

type bundleManifestFile struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

// BundleExportOptions controls preview-first portable knowledge export.
type BundleExportOptions struct {
	Options
	OutputPath       string
	DerivedIndexPath string
	IncludeIndex     bool
	Apply            bool
}

// BundleImportOptions controls preview-first import into an untrusted draft namespace.
type BundleImportOptions struct {
	Options
	BundlePath string
	Apply      bool
}

// BundleResult reports a portable exchange proposal.
type BundleResult struct {
	Applied           bool     `json:"applied"`
	Preview           bool     `json:"preview"`
	BundlePath        string   `json:"bundle_path"`
	BundleID          string   `json:"bundle_id,omitempty"`
	Files             int      `json:"files"`
	Pages             int      `json:"pages"`
	ExternalUntrusted bool     `json:"external_untrusted"`
	Changes           []Change `json:"changes"`
}

// ExportBundle packages verified current Markdown and optional disposable index with checksums.
func ExportBundle(opts BundleExportOptions) (BundleResult, error) {
	if strings.TrimSpace(opts.OutputPath) == "" {
		return BundleResult{}, errors.New("knowledge export requires an output path")
	}
	report, err := Lint(opts.Options)
	if err != nil {
		return BundleResult{}, err
	}
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, false)
	if err != nil {
		return BundleResult{}, err
	}
	files := map[string][]byte{}
	sourceManifest, _, err := readBoundProjectFile(paths, filepath.ToSlash(filepath.Join(paths.relRoot, DefaultSourceManifest)), effectivePageLimit(opts.MaxPageBytes), "bundle_export_manifest_after_open", opts.CustodyHook)
	if err != nil || containsSecret(sourceManifest) {
		return BundleResult{}, errors.New("read safe knowledge source manifest")
	}
	files[bundlePagePrefix+DefaultSourceManifest] = sourceManifest
	pageCount := 0
	for _, page := range report.Pages {
		if page.Path == "README.md" || page.Path == "index.md" || page.Path == "log.md" || page.Metadata.Status != "verified" || !page.Reachable || hasBlockingKnowledgeFinding(findingsForPath(report.Findings, page.Path)) {
			continue
		}
		data, _, readErr := readBoundProjectFile(paths, filepath.ToSlash(filepath.Join(paths.relRoot, page.Path)), effectivePageLimit(opts.MaxPageBytes), "bundle_export_page_after_open", opts.CustodyHook)
		if readErr != nil || sha256.Sum256(data) != page.contentDigest || containsSecret(data) {
			continue
		}
		files[bundlePagePrefix+page.Path] = data
		pageCount++
	}
	if opts.IncludeIndex {
		indexPath, pathErr := resolveIndexPath(opts.ProjectRoot, opts.DerivedIndexPath)
		if pathErr != nil {
			return BundleResult{}, pathErr
		}
		data, readErr := os.ReadFile(indexPath)
		if readErr != nil || len(data) > maxBundleBytes {
			return BundleResult{}, errors.New("read optional derived knowledge index")
		}
		files[bundleIndexPrefix+"knowledge-index.json"] = data
	}
	manifest := bundleManifestDocument{Schema: bundleSchema, CreatedAt: opts.NowOrCurrent().Format(time.RFC3339), RepositoryRevision: report.SourceRevision, ExternalUntrusted: true, Files: []bundleManifestFile{}}
	pathsInBundle := make([]string, 0, len(files))
	for path, data := range files {
		pathsInBundle = append(pathsInBundle, path)
		digest := sha256.Sum256(data)
		kind := "knowledge_page"
		if path == bundlePagePrefix+DefaultSourceManifest {
			kind = "source_manifest"
		} else if strings.HasPrefix(path, bundleIndexPrefix) {
			kind = "derived_index"
		}
		manifest.Files = append(manifest.Files, bundleManifestFile{Path: path, Kind: kind, SHA256: hex.EncodeToString(digest[:]), Bytes: len(data)})
	}
	sort.Strings(pathsInBundle)
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BundleResult{}, err
	}
	manifestData = append(manifestData, '\n')
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, path := range append([]string{bundleManifest}, pathsInBundle...) {
		data := files[path]
		if path == bundleManifest {
			data = manifestData
		}
		entry, createErr := writer.Create(path)
		if createErr != nil {
			return BundleResult{}, createErr
		}
		if _, writeErr := entry.Write(data); writeErr != nil {
			return BundleResult{}, writeErr
		}
	}
	if err := writer.Close(); err != nil {
		return BundleResult{}, err
	}
	if buffer.Len() > maxBundleBytes {
		return BundleResult{}, errors.New("knowledge bundle exceeds size limit")
	}
	output, err := resolvePortablePath(opts.ProjectRoot, opts.OutputPath)
	if err != nil {
		return BundleResult{}, err
	}
	result := BundleResult{Applied: opts.Apply, Preview: !opts.Apply, BundlePath: filepath.ToSlash(opts.OutputPath), Files: len(manifest.Files), Pages: pageCount, ExternalUntrusted: true, Changes: []Change{describeChange(filepath.ToSlash(opts.OutputPath), "create-bundle", buffer.Bytes(), string(manifestData))}}
	if !opts.Apply {
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return BundleResult{}, err
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return BundleResult{}, errors.New("knowledge bundle output already exists or is unsafe")
	}
	if _, err := file.Write(buffer.Bytes()); err != nil {
		_ = file.Close()
		_ = os.Remove(output)
		return BundleResult{}, errors.New("write knowledge bundle")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(output)
		return BundleResult{}, errors.New("close knowledge bundle")
	}
	return result, nil
}

// ImportBundle validates a portable bundle and proposes untrusted draft pages under imports/.
func ImportBundle(opts BundleImportOptions) (BundleResult, error) {
	bundlePath, err := resolvePortablePath(opts.ProjectRoot, opts.BundlePath)
	if err != nil {
		return BundleResult{}, err
	}
	data, err := os.ReadFile(bundlePath)
	if err != nil || len(data) > maxBundleBytes {
		return BundleResult{}, errors.New("read bounded knowledge bundle")
	}
	digest := sha256.Sum256(data)
	bundleID := hex.EncodeToString(digest[:])[:12]
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(reader.File) == 0 || len(reader.File) > maxBundleFiles {
		return BundleResult{}, errors.New("knowledge bundle archive is invalid or unbounded")
	}
	archive := map[string][]byte{}
	totalBytes := 0
	for _, file := range reader.File {
		if !safeBundlePath(file.Name) || file.UncompressedSize64 > uint64(effectivePageLimit(opts.MaxPageBytes)) {
			return BundleResult{}, errors.New("knowledge bundle contains an unsafe path or file")
		}
		stream, openErr := file.Open()
		if openErr != nil {
			return BundleResult{}, errors.New("open knowledge bundle entry")
		}
		entry, readErr := io.ReadAll(io.LimitReader(stream, effectivePageLimit(opts.MaxPageBytes)+1))
		_ = stream.Close()
		if readErr != nil || int64(len(entry)) > effectivePageLimit(opts.MaxPageBytes) || containsSecret(entry) {
			return BundleResult{}, errors.New("knowledge bundle entry is unsafe")
		}
		totalBytes += len(entry)
		if totalBytes > maxBundleBytes {
			return BundleResult{}, errors.New("knowledge bundle expanded content exceeds size limit")
		}
		archive[file.Name] = entry
	}
	var manifest bundleManifestDocument
	if json.Unmarshal(archive[bundleManifest], &manifest) != nil || manifest.Schema != bundleSchema || !manifest.ExternalUntrusted {
		return BundleResult{}, errors.New("knowledge bundle manifest is invalid")
	}
	if err := validateBundleManifest(manifest, archive); err != nil {
		return BundleResult{}, err
	}
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, false)
	if err != nil {
		return BundleResult{}, err
	}
	indexData, _, err := readBoundProjectFile(paths, filepath.ToSlash(filepath.Join(paths.relRoot, "index.md")), effectivePageLimit(opts.MaxPageBytes), "bundle_import_index_after_open", opts.CustodyHook)
	if err != nil {
		return BundleResult{}, errors.New("read knowledge index")
	}
	indexNext := append([]byte{}, indexData...)
	pageData := map[string][]byte{}
	changes := []Change{}
	for _, file := range manifest.Files {
		if file.Kind != "knowledge_page" {
			continue
		}
		meta, body, splitErr := splitPage(archive[file.Path])
		if splitErr != nil {
			return BundleResult{}, errors.New("knowledge bundle page metadata is invalid")
		}
		meta.Status = "draft"
		meta.Owner = "import-review-required"
		meta.LastVerified = opts.NowOrCurrent().Format("2006-01-02")
		meta.ReviewBy = meta.LastVerified
		meta.SourceSHA = firstNonEmptyBundle(opts.SourceRevision, "0000000")
		meta.Sources = []Source{}
		meta.Supersedes = []string{}
		meta.PromotionTarget = ""
		meta.PromotionCommit = ""
		body = "> Imported, untrusted draft. Original authority, verification, ownership, and citations must be re-established in this project.\n\n" + body
		next, renderErr := renderPage(meta, body)
		if renderErr != nil || containsSecret(next) {
			return BundleResult{}, errors.New("render imported knowledge draft")
		}
		target := filepath.ToSlash(filepath.Join("imports", bundleID, strings.TrimPrefix(file.Path, bundlePagePrefix)))
		if _, _, resolveErr := resolveKnowledgePage(paths, target, false); resolveErr != nil {
			return BundleResult{}, resolveErr
		}
		pageData[target] = next
		indexNext, err = appendIndexLink(indexNext, target, meta.Title+" (imported draft)")
		if err != nil {
			return BundleResult{}, err
		}
		changes = append(changes, describeChange(target, "create-untrusted-draft", next, string(next)))
	}
	changes = append(changes, describeChange("index.md", "update", indexNext, fmt.Sprintf("append imported draft namespace %s", bundleID)))
	result := BundleResult{Applied: opts.Apply, Preview: !opts.Apply, BundlePath: filepath.ToSlash(opts.BundlePath), BundleID: bundleID, Files: len(manifest.Files), Pages: len(pageData), ExternalUntrusted: true, Changes: changes}
	if !opts.Apply {
		return result, nil
	}
	if err := applyImportedPages(paths, pageData, indexData, indexNext, opts.CustodyHook); err != nil {
		return BundleResult{}, err
	}
	return result, nil
}

func validateBundleManifest(manifest bundleManifestDocument, archive map[string][]byte) error {
	if len(manifest.Files) > maxBundleFiles {
		return errors.New("knowledge bundle manifest exceeds file limit")
	}
	seen := map[string]bool{}
	for _, file := range manifest.Files {
		data, ok := archive[file.Path]
		if !ok || seen[file.Path] || file.Bytes != len(data) || (file.Kind != "knowledge_page" && file.Kind != "source_manifest" && file.Kind != "derived_index") {
			return errors.New("knowledge bundle manifest does not match archive")
		}
		digest := sha256.Sum256(data)
		if file.SHA256 != hex.EncodeToString(digest[:]) {
			return errors.New("knowledge bundle checksum mismatch")
		}
		seen[file.Path] = true
	}
	for path := range archive {
		if path != bundleManifest && !seen[path] {
			return errors.New("knowledge bundle contains an unmanifested entry")
		}
	}
	return nil
}

func applyImportedPages(paths resolvedPaths, pages map[string][]byte, indexOld, indexNext []byte, hook func(string)) error {
	keys := make([]string, 0, len(pages))
	for path := range pages {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	type createdFile struct {
		dir  *boundDirectory
		name string
	}
	created := []createdFile{}
	rollback := func() {
		for _, file := range created {
			removeBoundFile(file.dir, file.name)
			_ = file.dir.Close()
		}
	}
	for _, page := range keys {
		projectRel := filepath.ToSlash(filepath.Join(paths.relRoot, page))
		dirRel, name, err := splitBoundPath(projectRel)
		if err != nil {
			rollback()
			return err
		}
		dir, err := openBoundDirectory(paths.project, dirRel, true)
		if err != nil {
			rollback()
			return err
		}
		exists, err := boundFileExists(dir, name)
		if err != nil || exists || createBoundFile(dir, name, pages[page], 0o644) != nil {
			_ = dir.Close()
			rollback()
			return errors.New("knowledge import target already exists or is unsafe")
		}
		created = append(created, createdFile{dir: dir, name: name})
	}
	indexDir, err := openBoundDirectory(paths.project, paths.relRoot, false)
	if err != nil {
		rollback()
		return err
	}
	defer func() { _ = indexDir.Close() }()
	if err := replaceBoundFile(indexDir, "index.md", indexOld, indexNext, 0o644, "bundle_import_before_index_replace", hook); err != nil {
		rollback()
		return err
	}
	for _, file := range created {
		_ = file.dir.Close()
	}
	return nil
}

func safeBundlePath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == path && path != "" && !filepath.IsAbs(path) && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.Contains(path, "\\")
}

func resolvePortablePath(projectRoot, requested string) (string, error) {
	value := strings.TrimSpace(requested)
	if value == "" || filepath.IsAbs(value) || filepath.Clean(value) == ".." || strings.HasPrefix(filepath.Clean(value), ".."+string(filepath.Separator)) {
		return "", errors.New("knowledge portable path must be project-relative")
	}
	return filepath.Join(projectRoot, filepath.Clean(value)), nil
}

func firstNonEmptyBundle(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
