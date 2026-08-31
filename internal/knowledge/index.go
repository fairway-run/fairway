package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

const semanticIndexVersion = 1

type semanticIndex struct {
	Version            int                  `json:"version"`
	Model              string               `json:"model"`
	RepositoryRevision string               `json:"repository_revision"`
	Entries            []semanticIndexEntry `json:"entries"`
}

type semanticIndexEntry struct {
	Path   string    `json:"path"`
	SHA256 string    `json:"sha256"`
	Vector []float64 `json:"vector"`
}

// BuildSemanticIndex creates a disposable embedding index and writes it only when apply is explicit.
func BuildSemanticIndex(opts SemanticIndexOptions) (SemanticIndexResult, error) {
	if strings.TrimSpace(opts.Model) == "" || opts.Embed == nil {
		return SemanticIndexResult{}, errors.New("knowledge index requires a model and embedding adapter")
	}
	report, err := Lint(opts.Options)
	if err != nil {
		return SemanticIndexResult{}, err
	}
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, false)
	if err != nil {
		return SemanticIndexResult{}, err
	}
	index := semanticIndex{Version: semanticIndexVersion, Model: strings.TrimSpace(opts.Model), RepositoryRevision: report.SourceRevision, Entries: []semanticIndexEntry{}}
	dimensions := 0
	for _, page := range report.Pages {
		if page.Path == "README.md" || page.Path == "index.md" || page.Path == "log.md" || page.Metadata.Status == "superseded" || !page.Reachable || hasBlockingKnowledgeFinding(findingsForPath(report.Findings, page.Path)) {
			continue
		}
		data, _, readErr := readBoundProjectFile(paths, filepath.ToSlash(filepath.Join(paths.relRoot, page.Path)), effectivePageLimit(opts.MaxPageBytes), "index_page_after_open", opts.CustodyHook)
		if readErr != nil || sha256.Sum256(data) != page.contentDigest || containsSecret(data) {
			continue
		}
		text := page.Metadata.Title + "\n" + pageExcerpt(data, 4096)
		vector, embedErr := opts.Embed(text)
		if embedErr != nil || !validVector(vector) {
			return SemanticIndexResult{}, errors.New("knowledge index embedding adapter returned an invalid vector")
		}
		if dimensions == 0 {
			dimensions = len(vector)
		} else if dimensions != len(vector) {
			return SemanticIndexResult{}, errors.New("knowledge index embedding dimensions changed during build")
		}
		digest := sha256.Sum256(data)
		index.Entries = append(index.Entries, semanticIndexEntry{Path: page.Path, SHA256: hex.EncodeToString(digest[:]), Vector: vector})
	}
	sort.Slice(index.Entries, func(i, j int) bool { return index.Entries[i].Path < index.Entries[j].Path })
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return SemanticIndexResult{}, err
	}
	data = append(data, '\n')
	path, err := resolveIndexPath(opts.IndexPath)
	if err != nil {
		return SemanticIndexResult{}, err
	}
	result := SemanticIndexResult{Applied: opts.Apply, Preview: !opts.Apply, Model: index.Model, Pages: len(index.Entries), Changes: []Change{describeChange(filepath.ToSlash(firstNonEmptyIndex(opts.IndexPath, ".fairway/knowledge-index.json")), "replace-derived-index", data, string(data))}}
	if !opts.Apply {
		return result, nil
	}
	if err := writeBoundProjectFile(paths, path, data, 0o600, true, "semantic_index_before_replace", opts.CustodyHook); err != nil {
		return SemanticIndexResult{}, errors.New("write knowledge semantic index")
	}
	return result, nil
}

func semanticScores(opts QueryOptions, pages map[string][32]byte, query string) (map[string]float64, string) {
	if strings.TrimSpace(opts.SemanticIndexPath) == "" {
		return nil, ""
	}
	if opts.Embed == nil {
		return nil, "semantic embedding adapter unavailable; lexical fallback used"
	}
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, false)
	if err != nil {
		return nil, "semantic project custody is unavailable; lexical fallback used"
	}
	path, err := resolveIndexPath(opts.SemanticIndexPath)
	if err != nil {
		return nil, "semantic index path is unsafe; lexical fallback used"
	}
	data, _, err := readBoundProjectFile(paths, path, 32<<20, "semantic_query_index_after_open", opts.CustodyHook)
	if err != nil {
		return nil, "semantic index unavailable; lexical fallback used"
	}
	var index semanticIndex
	if json.Unmarshal(data, &index) != nil || index.Version != semanticIndexVersion {
		return nil, "semantic index is invalid or incompatible; lexical fallback used"
	}
	if strings.TrimSpace(opts.SemanticModel) == "" || index.Model != strings.TrimSpace(opts.SemanticModel) {
		return nil, "semantic index model does not match the query adapter; lexical fallback used"
	}
	queryVector, err := opts.Embed(query)
	if err != nil || !validVector(queryVector) {
		return nil, "semantic query embedding failed; lexical fallback used"
	}
	scores := map[string]float64{}
	for _, entry := range index.Entries {
		digest, ok := pages[entry.Path]
		if !ok || hex.EncodeToString(digest[:]) != entry.SHA256 || len(entry.Vector) != len(queryVector) || !validVector(entry.Vector) {
			continue
		}
		scores[entry.Path] = cosine(queryVector, entry.Vector)
	}
	if len(scores) == 0 {
		return nil, "semantic index has no current entries; lexical fallback used"
	}
	return scores, ""
}

func firstNonEmptyIndex(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolveIndexPath(requested string) (string, error) {
	value := strings.TrimSpace(requested)
	if value == "" {
		value = ".fairway/knowledge-index.json"
	}
	if filepath.IsAbs(value) || filepath.Clean(value) == ".." || strings.HasPrefix(filepath.Clean(value), ".."+string(filepath.Separator)) {
		return "", errors.New("knowledge index path must be project-relative")
	}
	return filepath.ToSlash(filepath.Clean(value)), nil
}

func validVector(vector []float64) bool {
	if len(vector) == 0 || len(vector) > 16384 {
		return false
	}
	for _, value := range vector {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func cosine(left, right []float64) float64 {
	var dot, leftNorm, rightNorm float64
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
