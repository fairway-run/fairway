package knowledge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	defaultQueryResults = 6
	queryExcerptBytes   = 480
)

// Ingest creates a bounded source-linked draft proposal and writes it only
// when Apply is explicit. It never copies source content into knowledge.
func Ingest(opts IngestOptions) (IngestResult, error) {
	if err := validateLifecycleInputs(opts.SourcePath, opts.SourceClass, opts.PagePath, opts.Title, opts.Owner, opts.ReviewBy); err != nil {
		return IngestResult{}, err
	}
	if strings.TrimSpace(opts.SourceRevision) == "" || !shaPattern.MatchString(opts.SourceRevision) {
		return IngestResult{}, errors.New("knowledge ingest requires a valid source revision")
	}
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, false)
	if err != nil {
		return IngestResult{}, err
	}
	manifest, err := readManifest(paths, opts.Options)
	if err != nil {
		return IngestResult{}, err
	}
	sourceRel, className, err := resolveIngestSource(paths, manifest, opts)
	if err != nil {
		return IngestResult{}, err
	}
	sourceData, _, err := readBoundProjectFile(paths, sourceRel, effectivePageLimit(opts.MaxPageBytes), "ingest_source_after_open", opts.CustodyHook)
	if err != nil {
		return IngestResult{}, errors.New("read bound knowledge ingest source")
	}
	match, err := sourceContentMatchesRevision(paths.project, sourceRel, sourceData, opts.SourceRevision, effectivePageLimit(opts.MaxPageBytes))
	if err != nil || !match {
		return IngestResult{}, errors.New("knowledge ingest source is uncommitted or does not match source revision")
	}
	pageRel, pageAbs, err := resolveKnowledgePage(paths, opts.PagePath, false)
	if err != nil {
		return IngestResult{}, err
	}
	if _, err := os.Lstat(pageAbs); err == nil {
		return IngestResult{}, errors.New("knowledge ingest target already exists")
	} else if !os.IsNotExist(err) {
		return IngestResult{}, errors.New("inspect knowledge ingest target")
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = titleFromPath(sourceRel)
	}
	if strings.TrimSpace(opts.Owner) == "" {
		return IngestResult{}, errors.New("knowledge ingest requires owner")
	}
	if _, err := parseDate(opts.ReviewBy); err != nil {
		return IngestResult{}, errors.New("knowledge ingest review-by must use YYYY-MM-DD")
	}
	meta := PageMetadata{
		KnowledgeVersion: 1,
		Title:            title,
		Status:           "draft",
		Owner:            strings.TrimSpace(opts.Owner),
		LastVerified:     dateOnly(opts.NowOrCurrent()).Format("2006-01-02"),
		ReviewBy:         strings.TrimSpace(opts.ReviewBy),
		SourceSHA:        opts.SourceRevision,
		Sources:          []Source{{Class: className, Path: sourceRel}},
		Supersedes:       []string{},
	}
	body := fmt.Sprintf("# %s\n\nThis draft is derived from [%s](%s). Verify conclusions against the cited source before changing its status.\n",
		meta.Title, sourceRel, relativeMarkdownLink(paths.relRoot, pageRel, sourceRel))
	pageData, err := renderPage(meta, body)
	if err != nil {
		return IngestResult{}, err
	}
	if containsSecret(pageData) {
		return IngestResult{}, errors.New("knowledge ingest proposal contains prohibited secret-like material")
	}
	indexData, _, err := readBoundProjectFile(paths, filepath.ToSlash(filepath.Join(paths.relRoot, "index.md")), effectivePageLimit(opts.MaxPageBytes), "ingest_index_after_open", opts.CustodyHook)
	if err != nil {
		return IngestResult{}, errors.New("read knowledge index")
	}
	indexNext, err := appendIndexLink(indexData, pageRel, title)
	if err != nil {
		return IngestResult{}, err
	}
	if int64(len(pageData)) > effectivePageLimit(opts.MaxPageBytes) || int64(len(indexNext)) > effectivePageLimit(opts.MaxPageBytes) {
		return IngestResult{}, errors.New("knowledge ingest proposal exceeds configured page size limit")
	}
	result := IngestResult{
		Applied: opts.Apply, Preview: !opts.Apply, SourcePath: sourceRel,
		SourceClass: className, SourceRevision: opts.SourceRevision,
		Changes: []Change{
			describeChange(pageRel, "create", pageData, string(pageData)),
			describeChange("index.md", "update", indexNext, fmt.Sprintf("append: - [%s](%s)", title, pageRel)),
		},
	}
	if !opts.Apply {
		return result, nil
	}
	if err := applyNewPageAndIndex(paths, pageRel, pageData, indexData, indexNext, opts.CustodyHook); err != nil {
		return IngestResult{}, err
	}
	return result, nil
}

// Query returns deterministic, bounded page projections with deduplicated
// provenance. Selection uses the topic and caller-supplied task terms.
func Query(opts QueryOptions) (QueryPacket, error) {
	if err := validateLifecycleInputs(opts.Topic, opts.TaskID); err != nil {
		return QueryPacket{}, err
	}
	if err := validateLifecycleInputs(opts.TaskTerms...); err != nil {
		return QueryPacket{}, err
	}
	if strings.TrimSpace(opts.Topic) == "" && strings.TrimSpace(opts.TaskID) == "" && len(opts.TaskTerms) == 0 {
		return QueryPacket{}, errors.New("knowledge query requires a topic or task")
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = defaultQueryResults
	}
	if opts.MaxResults > 24 {
		return QueryPacket{}, errors.New("knowledge query max-results exceeds 24")
	}
	if opts.BudgetBytes <= 0 {
		opts.BudgetBytes = defaultQueryBudget
	}
	if opts.BudgetBytes > maxQueryBudget || opts.BudgetBytes < 1024 {
		return QueryPacket{}, fmt.Errorf("knowledge query budget must be between 1024 and %d bytes", maxQueryBudget)
	}
	report, err := Lint(opts.Options)
	if err != nil {
		return QueryPacket{}, err
	}
	if hasGlobalSourceErrors(report.Findings) {
		return QueryPacket{}, errors.New("knowledge query requires a valid sources manifest and source classes")
	}
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, false)
	if err != nil {
		return QueryPacket{}, err
	}
	manifest, err := readManifest(paths, opts.Options)
	if err != nil {
		return QueryPacket{}, err
	}
	terms := queryTerms(opts.Topic, opts.TaskID, opts.TaskTerms)
	type candidate struct {
		page    QueryPage
		sources []QuerySource
	}
	candidates := []candidate{}
	for _, page := range report.Pages {
		if page.Path == "README.md" || page.Path == "index.md" || page.Path == "log.md" || page.Metadata.Status == "superseded" || !page.Reachable {
			continue
		}
		data, readErr := readBounded(filepath.Join(paths.root, filepath.FromSlash(page.Path)), effectivePageLimit(opts.MaxPageBytes))
		if readErr != nil || containsSecret(data) {
			continue
		}
		excerpt := pageExcerpt(data, queryExcerptBytes)
		score := relevanceScore(terms, page.Path+" "+page.Metadata.Title+" "+excerpt)
		if score == 0 {
			continue
		}
		findings := findingsForPath(report.Findings, page.Path)
		verified := page.Metadata.Status == "verified" && !hasBlockingKnowledgeFinding(findings)
		projected := QueryPage{
			Path: page.Path, Title: page.Metadata.Title, Status: page.Metadata.Status,
			Owner: page.Metadata.Owner, ReviewBy: page.Metadata.ReviewBy, SourceSHA: page.Metadata.SourceSHA,
			Excerpt: excerpt, Score: score, SourceCount: len(page.Metadata.Sources),
			Verified: verified, Conflict: page.Metadata.Status == "conflicted", Stale: page.Metadata.Status == "stale" || hasStaleFinding(findings),
		}
		candidates = append(candidates, candidate{page: projected, sources: projectQuerySources(page.Path, page.Metadata.Sources, manifest, verified)})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].page.Score != candidates[j].page.Score {
			return candidates[i].page.Score > candidates[j].page.Score
		}
		if candidates[i].page.Verified != candidates[j].page.Verified {
			return candidates[i].page.Verified
		}
		return candidates[i].page.Path < candidates[j].page.Path
	})
	packet := QueryPacket{
		Schema: "fairway.knowledge-query.v1", Topic: strings.TrimSpace(opts.Topic),
		TaskID: strings.TrimSpace(opts.TaskID), Pages: []QueryPage{}, Sources: []QuerySource{},
		BudgetBytes: opts.BudgetBytes, Bounded: true, ReadOnly: true,
	}
	excludedUnsafe := 0
	for _, finding := range report.Findings {
		if finding.Code == "secret_pattern" || finding.Code == "metadata_invalid" || finding.Code == "metadata_missing" {
			excludedUnsafe++
		}
	}
	if excludedUnsafe > 0 {
		packet.Warnings = append(packet.Warnings, fmt.Sprintf("%d unsafe or structurally invalid knowledge page(s) were excluded", excludedUnsafe))
	}
	sourceByKey := map[string]QuerySource{}
	for _, item := range candidates {
		if len(packet.Pages) >= opts.MaxResults {
			break
		}
		next := packet
		next.Pages = append(append([]QueryPage{}, packet.Pages...), item.page)
		nextSources := cloneSourceMap(sourceByKey)
		for _, source := range item.sources {
			nextSources[source.Key] = mergeQuerySource(nextSources[source.Key], source)
		}
		next.Sources = sortedSources(nextSources)
		if err := FinalizeQueryPacket(&next); err != nil {
			return QueryPacket{}, err
		}
		if next.Bytes > opts.BudgetBytes {
			packet.Warnings = append(packet.Warnings, "knowledge budget reached before all matching pages were included")
			break
		}
		packet = next
		sourceByKey = nextSources
	}
	if len(packet.Pages) == 0 {
		packet.Warnings = append(packet.Warnings, "no indexed knowledge page matched the requested context")
	}
	packet.Sources = sortedSources(sourceByKey)
	if err := FinalizeQueryPacket(&packet); err != nil {
		return QueryPacket{}, err
	}
	if packet.Bytes > packet.BudgetBytes {
		return QueryPacket{}, errors.New("knowledge query packet exceeded budget")
	}
	return packet, nil
}

// Promote verifies the page and reviewed canonical target, then records the
// promotion on the derived page only when Apply is explicit.
func Promote(opts PromoteOptions) (PromoteResult, error) {
	if err := validateLifecycleInputs(opts.PagePath, opts.TargetPath, opts.ReviewedCommit); err != nil {
		return PromoteResult{}, err
	}
	if strings.TrimSpace(opts.TargetPath) == "" || strings.TrimSpace(opts.ReviewedCommit) == "" {
		return PromoteResult{}, errors.New("knowledge promote requires canonical target and reviewed commit")
	}
	if !shaPattern.MatchString(opts.ReviewedCommit) {
		return PromoteResult{}, errors.New("knowledge promote reviewed commit is invalid")
	}
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, false)
	if err != nil {
		return PromoteResult{}, err
	}
	pageRel, _, err := resolveKnowledgePage(paths, opts.PagePath, true)
	if err != nil {
		return PromoteResult{}, err
	}
	report, err := Lint(opts.Options)
	if err != nil {
		return PromoteResult{}, err
	}
	if hasGlobalSourceErrors(report.Findings) {
		return PromoteResult{}, errors.New("knowledge promotion requires a valid sources manifest and source classes")
	}
	var page *Page
	for index := range report.Pages {
		if report.Pages[index].Path == pageRel {
			page = &report.Pages[index]
			break
		}
	}
	if page == nil {
		return PromoteResult{}, errors.New("knowledge promotion page is not indexed")
	}
	if page.Metadata.Status != "verified" || hasBlockingKnowledgeFinding(findingsForPath(report.Findings, pageRel)) {
		return PromoteResult{}, errors.New("knowledge promotion requires a verified, current, conflict-free citation chain")
	}
	manifest, err := readManifest(paths, opts.Options)
	if err != nil {
		return PromoteResult{}, err
	}
	targetRel, _, err := resolveCanonicalTarget(paths, manifest, opts.TargetPath)
	if err != nil {
		return PromoteResult{}, err
	}
	if !commitIsAncestor(paths.project, opts.ReviewedCommit) {
		return PromoteResult{}, errors.New("reviewed commit is not an ancestor of the current source revision")
	}
	targetData, _, err := readBoundProjectFile(paths, targetRel, effectivePageLimit(opts.MaxPageBytes), "promote_target_after_open", opts.CustodyHook)
	if err != nil {
		return PromoteResult{}, errors.New("read bound canonical promotion target")
	}
	match, err := sourceContentMatchesRevision(paths.project, targetRel, targetData, opts.ReviewedCommit, effectivePageLimit(opts.MaxPageBytes))
	if err != nil || !match {
		return PromoteResult{}, errors.New("canonical target is missing, unsafe, uncommitted, or differs from reviewed commit")
	}
	data, _, err := readBoundProjectFile(paths, filepath.ToSlash(filepath.Join(paths.relRoot, pageRel)), effectivePageLimit(opts.MaxPageBytes), "promote_page_after_open", opts.CustodyHook)
	if err != nil || containsSecret(data) {
		return PromoteResult{}, errors.New("knowledge promotion page cannot be read safely")
	}
	meta, body, err := splitPage(data)
	if err != nil {
		return PromoteResult{}, err
	}
	if err := validatePromotionSourcesBound(paths, manifest, meta, opts); err != nil {
		return PromoteResult{}, err
	}
	meta.Status = "superseded"
	meta.PromotionTarget = targetRel
	meta.PromotionCommit = strings.TrimSpace(opts.ReviewedCommit)
	next, err := renderPage(meta, body)
	if err != nil || containsSecret(next) {
		return PromoteResult{}, errors.New("knowledge promotion proposal is unsafe")
	}
	if int64(len(next)) > effectivePageLimit(opts.MaxPageBytes) {
		return PromoteResult{}, errors.New("knowledge promotion proposal exceeds configured page size limit")
	}
	result := PromoteResult{
		Applied: opts.Apply, Preview: !opts.Apply, PagePath: pageRel,
		TargetPath: targetRel, ReviewedCommit: opts.ReviewedCommit,
		Changes: []Change{describeChange(pageRel, "update", next, string(next))},
	}
	if !opts.Apply {
		return result, nil
	}
	if err := replaceBoundProjectFile(paths, filepath.ToSlash(filepath.Join(paths.relRoot, pageRel)), data, next, 0o644, "promote_before_replace", opts.CustodyHook); err != nil {
		return PromoteResult{}, err
	}
	return result, nil
}

func (opts Options) NowOrCurrent() time.Time {
	if opts.Now.IsZero() {
		return time.Now().UTC()
	}
	return opts.Now.UTC()
}

func readManifest(paths resolvedPaths, opts Options) (SourceManifest, error) {
	data, _, err := readBoundProjectFile(paths, filepath.ToSlash(filepath.Join(paths.relRoot, DefaultSourceManifest)), effectivePageLimit(opts.MaxPageBytes), "manifest_after_open", opts.CustodyHook)
	if err != nil {
		return SourceManifest{}, errors.New("read knowledge source manifest")
	}
	var manifest SourceManifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil || validateSourceManifest(manifest, paths.relRoot) != nil {
		return SourceManifest{}, errors.New("knowledge source manifest is invalid")
	}
	return manifest, nil
}

func validateSourceManifest(manifest SourceManifest, knowledgeRoot string) error {
	if manifest.Version != 1 || len(manifest.Classes) == 0 {
		return errors.New("source manifest requires version 1 and source classes")
	}
	for name, class := range manifest.Classes {
		if !sourceClassPattern.MatchString(name) || strings.TrimSpace(class.Authority) == "" {
			return errors.New("source manifest contains invalid source class")
		}
		switch class.Kind {
		case "project_file":
			if class.FairwayKind != "" || class.RequiresStoreValidation || len(class.Roots) == 0 {
				return errors.New("project-file source class is invalid")
			}
			for _, root := range class.Roots {
				if !safeSourceRoot(root) || (class.Authority == "canonical" && isLegacyMemoryRoot(root)) ||
					sourceWithinAllowedRoots(filepath.ToSlash(filepath.Clean(knowledgeRoot)), []string{root}, "") {
					return errors.New("project-file source class root is unsafe")
				}
			}
		case "fairway":
			if len(class.Roots) != 0 || (class.FairwayKind != "decision" && class.FairwayKind != "evidence") || !class.RequiresStoreValidation {
				return errors.New("Fairway source class is invalid")
			}
		default:
			return errors.New("source class kind is unsupported")
		}
	}
	return nil
}

func validatePromotionSourcesBound(paths resolvedPaths, manifest SourceManifest, meta PageMetadata, opts PromoteOptions) error {
	if meta.Status != "verified" {
		return errors.New("knowledge promotion requires verified page metadata")
	}
	reviewBy, err := parseDate(meta.ReviewBy)
	if err != nil || reviewBy.Before(dateOnly(opts.NowOrCurrent())) {
		return errors.New("knowledge promotion page review date is invalid or overdue")
	}
	if len(meta.Sources) == 0 || !shaPattern.MatchString(meta.SourceSHA) {
		return errors.New("knowledge promotion requires a verified citation chain")
	}
	valid := 0
	for _, source := range meta.Sources {
		class, ok := manifest.Classes[source.Class]
		if !ok {
			return errors.New("knowledge promotion source class is invalid")
		}
		switch class.Kind {
		case "project_file":
			if source.Path == "" || source.Fairway != nil || !sourceWithinAllowedRoots(source.Path, class.Roots, paths.relRoot) {
				return errors.New("knowledge promotion project source is unsafe")
			}
			data, _, readErr := readBoundProjectFile(paths, source.Path, effectivePageLimit(opts.MaxPageBytes), "promote_source_after_open", opts.CustodyHook)
			if readErr != nil {
				return errors.New("knowledge promotion source cannot be read safely")
			}
			match, matchErr := sourceContentMatchesRevision(paths.project, source.Path, data, meta.SourceSHA, effectivePageLimit(opts.MaxPageBytes))
			if matchErr != nil || !match {
				return errors.New("knowledge promotion source differs from cited revision")
			}
			valid++
		case "fairway":
			if source.Fairway == nil || source.Path != "" || opts.ValidateFairwayReference == nil {
				return errors.New("knowledge promotion Fairway source is unverified")
			}
			requirement := FairwayReferenceRequirement{SourceClass: source.Class, Reference: *source.Fairway}
			if requirement.Reference.Kind != class.FairwayKind || opts.ValidateFairwayReference(requirement) != nil {
				return errors.New("knowledge promotion Fairway source is unverified")
			}
			valid++
		default:
			return errors.New("knowledge promotion source kind is unsupported")
		}
	}
	if valid == 0 {
		return errors.New("knowledge promotion has no verified citation")
	}
	return nil
}

func sourceContentMatchesRevision(projectRoot, relativePath string, current []byte, revision string, limit int64) (bool, error) {
	var committed boundedBuffer
	committed.limit = limit
	cmd := exec.Command("git", "-C", projectRoot, "show", revision+":"+filepath.ToSlash(relativePath))
	cmd.Stdout = &committed
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return false, errors.New("read source at revision")
	}
	return bytes.Equal(current, committed.Bytes()), nil
}

func replaceBoundProjectFile(paths resolvedPaths, relativePath string, expected, next []byte, mode uint32, stage string, hook func(string)) error {
	dirRel, name, err := splitBoundPath(relativePath)
	if err != nil {
		return err
	}
	dir, err := openBoundDirectory(paths.project, dirRel, false)
	if err != nil {
		return err
	}
	defer dir.Close()
	return replaceBoundFile(dir, name, expected, next, mode, stage, hook)
}

func resolveIngestSource(paths resolvedPaths, manifest SourceManifest, opts IngestOptions) (string, string, error) {
	source := filepath.ToSlash(filepath.Clean(strings.TrimSpace(opts.SourcePath)))
	if source == "" || source == "." || filepath.IsAbs(source) || source == ".." || strings.HasPrefix(source, "../") {
		return "", "", errors.New("knowledge ingest source path is unsafe")
	}
	className := strings.TrimSpace(opts.SourceClass)
	if className != "" {
		class, ok := manifest.Classes[className]
		if !ok || class.Kind != "project_file" || !sourceWithinAllowedRoots(source, class.Roots, paths.relRoot) {
			return "", "", errors.New("knowledge ingest source does not satisfy requested source class")
		}
		return source, className, nil
	}
	names := make([]string, 0, len(manifest.Classes))
	for name := range manifest.Classes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		class := manifest.Classes[name]
		if class.Kind == "project_file" && sourceWithinAllowedRoots(source, class.Roots, paths.relRoot) {
			return source, name, nil
		}
	}
	return "", "", errors.New("knowledge ingest source is outside configured project-file roots")
}

func resolveKnowledgePage(paths resolvedPaths, requested string, mustExist bool) (string, string, error) {
	rel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(requested)))
	if rel == "" || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") || !strings.EqualFold(filepath.Ext(rel), ".md") {
		return "", "", errors.New("knowledge page must be a safe relative Markdown path")
	}
	abs := filepath.Join(paths.root, filepath.FromSlash(rel))
	inside, err := filepath.Rel(paths.root, abs)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", "", errors.New("knowledge page escapes configured root")
	}
	parent := filepath.Dir(abs)
	if err := validateExistingPathChain(paths.project, nearestExisting(parent)); err != nil {
		return "", "", err
	}
	if mustExist {
		info, err := os.Lstat(abs)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return "", "", errors.New("knowledge page is missing or unsafe")
		}
	}
	return rel, abs, nil
}

func resolveCanonicalTarget(paths resolvedPaths, manifest SourceManifest, requested string) (string, string, error) {
	rel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(requested)))
	if rel == "" || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", errors.New("canonical target path is unsafe")
	}
	allowed := false
	for _, class := range manifest.Classes {
		if class.Kind == "project_file" && class.Authority == "canonical" && sourceWithinAllowedRoots(rel, class.Roots, paths.relRoot) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", "", errors.New("canonical target is outside configured canonical roots")
	}
	abs, err := safeProjectFile(paths, rel)
	if err != nil {
		return "", "", errors.New("canonical target is missing or unsafe")
	}
	return rel, abs, nil
}

func renderPage(meta PageMetadata, body string) ([]byte, error) {
	frontmatter, err := yaml.Marshal(meta)
	if err != nil {
		return nil, errors.New("marshal knowledge metadata")
	}
	return []byte("---\n" + string(frontmatter) + "---\n\n" + strings.TrimLeft(body, "\r\n")), nil
}

func splitPage(data []byte) (PageMetadata, string, error) {
	meta, findings := parseMetadata(data, "page")
	if len(findings) > 0 {
		return PageMetadata{}, "", errors.New("knowledge page metadata is invalid")
	}
	text := string(data)
	start := strings.Index(text, "\n---")
	if start < 0 {
		return PageMetadata{}, "", errors.New("knowledge page frontmatter is incomplete")
	}
	end := start + len("\n---")
	if end < len(text) && text[end] == '\r' {
		end++
	}
	if end < len(text) && text[end] == '\n' {
		end++
	}
	return meta, strings.TrimLeft(text[end:], "\r\n"), nil
}

func appendIndexLink(index []byte, pagePath, title string) ([]byte, error) {
	link := fmt.Sprintf("- [%s](%s)", title, filepath.ToSlash(pagePath))
	if strings.Contains(string(index), "]("+filepath.ToSlash(pagePath)+")") {
		return nil, errors.New("knowledge index already links the ingest target")
	}
	if containsSecret(index) {
		return nil, errors.New("knowledge index contains prohibited secret-like material")
	}
	return append(append(bytes.TrimRight(index, "\r\n"), '\n'), []byte(link+"\n")...), nil
}

func applyNewPageAndIndex(paths resolvedPaths, pageRel string, pageData, indexOld, indexNext []byte, hook func(string)) error {
	pageProjectRel := filepath.ToSlash(filepath.Join(paths.relRoot, pageRel))
	pageDirRel, pageName, err := splitBoundPath(pageProjectRel)
	if err != nil {
		return err
	}
	pageDir, err := openBoundDirectory(paths.project, pageDirRel, true)
	if err != nil {
		return err
	}
	defer pageDir.Close()
	indexDir, err := openBoundDirectory(paths.project, paths.relRoot, false)
	if err != nil {
		return err
	}
	defer indexDir.Close()
	currentIndex, _, err := readBoundFileAt(indexDir, "index.md", int64(max(len(indexOld), len(indexNext)))+1, "", nil)
	if err != nil || !bytes.Equal(currentIndex, indexOld) {
		return errors.New("knowledge index changed during ingest")
	}
	exists, err := boundFileExists(pageDir, pageName)
	if err != nil || exists {
		return errors.New("knowledge ingest target already exists or is unsafe")
	}
	if hook != nil {
		hook("ingest_after_bind")
	}
	if err := createBoundFile(pageDir, pageName, pageData, 0o644); err != nil {
		return err
	}
	if err := replaceBoundFile(indexDir, "index.md", indexOld, indexNext, 0o644, "ingest_before_index_replace", hook); err != nil {
		removeBoundFile(pageDir, pageName)
		return err
	}
	return nil
}

func describeChange(path, action string, data []byte, preview string) Change {
	sum := sha256.Sum256(data)
	return Change{
		Path: filepath.ToSlash(path), Action: action, SHA256: hex.EncodeToString(sum[:]),
		Bytes: len(data), Preview: boundedChangePreview(preview),
	}
}

func boundedChangePreview(value string) string {
	const limit = 2048
	if len(value) > limit {
		value = value[:limit]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
		value = strings.TrimRight(value, "\r\n ") + "\n[preview truncated]"
	}
	return value
}

func titleFromPath(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(name)
	for i := range words {
		if words[i] != "" {
			words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
		}
	}
	return strings.Join(words, " ")
}

func relativeMarkdownLink(knowledgeRoot, pageRel, sourceRel string) string {
	pageProjectPath := filepath.Join(filepath.FromSlash(knowledgeRoot), filepath.FromSlash(pageRel))
	link, err := filepath.Rel(filepath.Dir(pageProjectPath), filepath.FromSlash(sourceRel))
	if err != nil {
		return sourceRel
	}
	return filepath.ToSlash(link)
}

func queryTerms(values ...any) []string {
	seen := map[string]bool{}
	result := []string{}
	add := func(value string) {
		for _, term := range tokenize(value) {
			if !seen[term] {
				seen[term] = true
				result = append(result, term)
			}
		}
	}
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			add(typed)
		case []string:
			for _, item := range typed {
				add(item)
			}
		}
	}
	sort.Strings(result)
	return result
}

func tokenize(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	result := []string{}
	for _, field := range fields {
		if utf8.RuneCountInString(field) >= 2 {
			result = append(result, field)
		}
	}
	return result
}

func relevanceScore(terms []string, content string) int {
	content = strings.ToLower(content)
	score := 0
	for _, term := range terms {
		if strings.Contains(content, term) {
			score++
		}
	}
	return score
}

func pageExcerpt(data []byte, limit int) string {
	_, body, err := splitPage(data)
	if err != nil {
		return ""
	}
	lines := strings.Split(body, "\n")
	parts := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "#-* "))
		if line == "" {
			continue
		}
		parts = append(parts, line)
		if len(strings.Join(parts, " ")) >= limit {
			break
		}
	}
	excerpt := strings.Join(parts, " ")
	if len(excerpt) > limit {
		excerpt = excerpt[:limit]
		for !utf8.ValidString(excerpt) {
			excerpt = excerpt[:len(excerpt)-1]
		}
	}
	return strings.TrimSpace(excerpt)
}

func findingsForPath(findings []Finding, path string) []Finding {
	result := []Finding{}
	for _, finding := range findings {
		if finding.Path == path {
			result = append(result, finding)
		}
	}
	return result
}

func hasBlockingKnowledgeFinding(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity == SeverityError || finding.Code == "review_overdue" || finding.Code == "source_revision_stale" {
			return true
		}
	}
	return false
}

func hasStaleFinding(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Code == "review_overdue" || finding.Code == "source_revision_stale" {
			return true
		}
	}
	return false
}

func projectQuerySources(pagePath string, sources []Source, manifest SourceManifest, verified bool) []QuerySource {
	result := []QuerySource{}
	for _, source := range sources {
		class := manifest.Classes[source.Class]
		projected := QuerySource{Class: source.Class, Authority: class.Authority, Kind: class.Kind, Verified: verified, Citations: []QuerySourceCitation{{Page: pagePath, Verified: verified}}}
		if source.Path != "" {
			projected.Path = source.Path
			projected.Key = "file:" + source.Path
		} else if source.Fairway != nil {
			projected.FairwayID = source.Fairway.ID
			projected.Kind = "fairway_" + source.Fairway.Kind
			projected.Key = "fairway:" + source.Fairway.Kind + ":" + source.Fairway.ID
		}
		if projected.Key != "" {
			result = append(result, projected)
		}
	}
	return result
}

func mergeQuerySource(existing, candidate QuerySource) QuerySource {
	if existing.Key == "" {
		return candidate
	}
	if authorityRank(candidate.Authority) > authorityRank(existing.Authority) ||
		(authorityRank(candidate.Authority) == authorityRank(existing.Authority) && candidate.Class < existing.Class) {
		existing.Class = candidate.Class
		existing.Authority = candidate.Authority
		existing.Kind = candidate.Kind
		existing.Path = candidate.Path
		existing.FairwayID = candidate.FairwayID
	}
	existing.Verified = existing.Verified || candidate.Verified
	existing.MemoryReferenced = existing.MemoryReferenced || candidate.MemoryReferenced
	seen := map[string]bool{}
	for _, citation := range existing.Citations {
		seen[citation.Page] = true
	}
	for _, citation := range candidate.Citations {
		if !seen[citation.Page] {
			existing.Citations = append(existing.Citations, citation)
			seen[citation.Page] = true
		}
	}
	sort.Slice(existing.Citations, func(i, j int) bool { return existing.Citations[i].Page < existing.Citations[j].Page })
	return existing
}

func authorityRank(authority string) int {
	switch authority {
	case "canonical":
		return 3
	case "operational":
		return 2
	case "evidence":
		return 1
	default:
		return 0
	}
}

func cloneSourceMap(source map[string]QuerySource) map[string]QuerySource {
	result := make(map[string]QuerySource, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func sortedSources(source map[string]QuerySource) []QuerySource {
	result := make([]QuerySource, 0, len(source))
	for _, value := range source {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

// FinalizeQueryPacket sets Bytes to the exact rendered JSON size, including
// the Bytes field itself. The decimal width converges in a bounded number of
// iterations.
func FinalizeQueryPacket(packet *QueryPacket) error {
	packet.Bytes = 0
	for range 8 {
		data, err := json.Marshal(packet)
		if err != nil {
			return err
		}
		size := len(data)
		if packet.Bytes == size {
			return nil
		}
		packet.Bytes = size
	}
	return errors.New("knowledge query packet byte accounting did not converge")
}

func commitIsAncestor(projectRoot, revision string) bool {
	cmd := exec.Command("git", "-C", projectRoot, "merge-base", "--is-ancestor", revision, "HEAD")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func effectivePageLimit(value int64) int64 {
	if value <= 0 {
		return defaultMaxPageBytes
	}
	return value
}

func validateLifecycleInputs(values ...string) error {
	for _, value := range values {
		if len(value) > 512 {
			return errors.New("knowledge lifecycle input exceeds 512-byte limit")
		}
		if containsSecret([]byte(value)) {
			return errors.New("knowledge lifecycle input contains prohibited secret-like material")
		}
	}
	return nil
}

func hasGlobalSourceErrors(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity != SeverityError {
			continue
		}
		switch finding.Code {
		case "source_manifest_missing", "source_manifest_invalid", "source_class_invalid":
			return true
		}
	}
	return false
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}

func nearestExisting(path string) string {
	for {
		if _, err := os.Lstat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}
