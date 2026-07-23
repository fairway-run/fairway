package knowledge

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/secretscan"
	"gopkg.in/yaml.v3"
)

var (
	markdownLinkPattern = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+[^)]*)?\)`)
	shaPattern          = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)
	sourceClassPattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	fairwayIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
	allowedStatuses     = map[string]bool{
		"draft": true, "verified": true, "stale": true, "conflicted": true, "superseded": true,
	}
)

type scanState struct {
	opts       Options
	paths      resolvedPaths
	report     Report
	pageByPath map[string]int
	links      map[string][]string
	identities map[string][]string
	manifest   SourceManifest
	linkCount  int
}

// Status inventories the knowledge tree and returns deterministic validation
// findings without changing project files.
func Status(opts Options) (Report, error) {
	return scan(opts)
}

// Lint performs the same deterministic bounded validation as Status. It is a
// separate entry point so CLI wiring can assign different output and exit-code
// policy without duplicating package behavior.
func Lint(opts Options) (Report, error) {
	return scan(opts)
}

func scan(opts Options) (Report, error) {
	if opts.SourceRevision != "" && !shaPattern.MatchString(opts.SourceRevision) {
		return Report{}, errors.New("knowledge source revision must be a 7-64 character hexadecimal revision")
	}
	paths, err := resolvePaths(opts.ProjectRoot, opts.KnowledgeRoot, false)
	if err != nil {
		return Report{}, err
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = defaultMaxPages
	}
	if opts.MaxPageBytes <= 0 {
		opts.MaxPageBytes = defaultMaxPageBytes
	}
	if opts.MaxLinks <= 0 {
		opts.MaxLinks = defaultMaxLinks
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	} else {
		opts.Now = opts.Now.UTC()
	}
	state := scanState{
		opts: opts, paths: paths,
		report:     Report{Root: paths.relRoot, SourceRevision: opts.SourceRevision, Pages: []Page{}, Findings: []Finding{}},
		pageByPath: map[string]int{}, links: map[string][]string{}, identities: map[string][]string{},
	}
	state.loadSourceManifest()
	if err := state.walk(); err != nil {
		return Report{}, err
	}
	state.validateIdentities()
	state.validateLinks()
	state.validateIndex()
	state.sort()
	return state.report, nil
}

func (s *scanState) loadSourceManifest() {
	path := filepath.Join(s.paths.root, DefaultSourceManifest)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		s.add("source_manifest_missing", SeverityError, DefaultSourceManifest, "configured source manifest is required")
		return
	}
	if info.Size() > s.opts.MaxPageBytes {
		s.add("source_manifest_invalid", SeverityError, DefaultSourceManifest, "source manifest exceeds the configured size limit")
		return
	}
	data, err := readBounded(path, s.opts.MaxPageBytes)
	if err != nil {
		s.add("source_manifest_invalid", SeverityError, DefaultSourceManifest, "source manifest cannot be read safely")
		return
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&s.manifest); err != nil {
		s.add("source_manifest_invalid", SeverityError, DefaultSourceManifest, "source manifest does not match the configured schema")
		return
	}
	if s.manifest.Version != 1 || len(s.manifest.Classes) == 0 {
		s.add("source_manifest_invalid", SeverityError, DefaultSourceManifest, "source manifest requires version 1 and at least one source class")
		return
	}
	for name, class := range s.manifest.Classes {
		if !sourceClassPattern.MatchString(name) {
			s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "source class name is invalid")
			continue
		}
		if strings.TrimSpace(class.Authority) == "" {
			s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "source class authority is required")
		}
		switch class.Kind {
		case "project_file":
			if class.FairwayKind != "" || class.RequiresStoreValidation {
				s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "project_file source class has incompatible Fairway settings")
			}
			if len(class.Roots) == 0 {
				s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "project_file source class requires at least one allowed root")
			}
			for _, root := range class.Roots {
				if !safeSourceRoot(root) {
					s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "project_file source class contains an unsafe allowed root")
				}
				if class.Authority == "canonical" && isLegacyMemoryRoot(root) {
					s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "canonical source class cannot use legacy tmp-ux memory as an allowed root")
				}
			}
		case "fairway":
			if len(class.Roots) != 0 {
				s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "fairway source class cannot define file roots")
			}
			if class.FairwayKind != "decision" && class.FairwayKind != "evidence" {
				s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "fairway source class requires decision or evidence kind")
			}
			if !class.RequiresStoreValidation {
				s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "fairway source class must require store validation")
			}
		default:
			s.add("source_class_invalid", SeverityError, DefaultSourceManifest, "source class kind is unsupported")
		}
	}
}

func (s *scanState) walk() error {
	return filepath.WalkDir(s.paths.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("walk knowledge root")
		}
		if path == s.paths.root {
			return nil
		}
		rel, err := filepath.Rel(s.paths.root, path)
		if err != nil {
			return errors.New("resolve knowledge page path")
		}
		rel = filepath.ToSlash(rel)
		info, err := entry.Info()
		if err != nil {
			return errors.New("inspect knowledge entry")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("knowledge root contains symlink at %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("knowledge root contains non-regular file at %s", rel)
		}
		if !strings.EqualFold(filepath.Ext(rel), ".md") {
			return nil
		}
		if len(s.report.Pages) >= s.opts.MaxPages {
			return fmt.Errorf("knowledge page count exceeds limit %d", s.opts.MaxPages)
		}
		return s.readPage(path, rel, info.Size())
	})
}

func (s *scanState) readPage(path, rel string, size int64) error {
	if size > s.opts.MaxPageBytes {
		s.add("page_too_large", SeverityError, rel, fmt.Sprintf("page exceeds %d-byte limit", s.opts.MaxPageBytes))
		return nil
	}
	data, err := readBounded(path, s.opts.MaxPageBytes)
	if err != nil {
		return fmt.Errorf("read knowledge page %s: %w", rel, err)
	}
	hasSecret := containsSecret(data)
	if hasSecret {
		s.add("secret_pattern", SeverityError, rel, "page contains prohibited secret-like content")
	}
	links := extractLinks(data)
	if s.linkCount+len(links) > s.opts.MaxLinks {
		return fmt.Errorf("knowledge link count exceeds limit %d", s.opts.MaxLinks)
	}
	s.linkCount += len(links)
	page := Page{Path: rel, LinkCount: len(links)}
	if rel != "README.md" && rel != "log.md" {
		meta, findings := parseMetadata(data, rel)
		if !hasSecret {
			page.Metadata = meta
		}
		for _, finding := range findings {
			s.report.Findings = append(s.report.Findings, finding)
		}
		s.validateMetadata(rel, meta)
		identity := normalizedIdentity(meta.Title)
		if identity != "" {
			s.identities[identity] = append(s.identities[identity], rel)
		}
	}
	s.pageByPath[rel] = len(s.report.Pages)
	s.links[rel] = links
	s.report.Pages = append(s.report.Pages, page)
	return nil
}

func parseMetadata(data []byte, path string) (PageMetadata, []Finding) {
	findings := []Finding{}
	if !bytes.HasPrefix(data, []byte("---\n")) && !bytes.HasPrefix(data, []byte("---\r\n")) {
		return PageMetadata{}, []Finding{{Code: "metadata_missing", Severity: SeverityError, Path: path, Detail: "maintained page requires YAML frontmatter"}}
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1024), int(defaultMaxPageBytes))
	if !scanner.Scan() {
		return PageMetadata{}, []Finding{{Code: "metadata_invalid", Severity: SeverityError, Path: path, Detail: "frontmatter is empty"}}
	}
	var frontmatter strings.Builder
	closed := false
	for scanner.Scan() {
		if scanner.Text() == "---" {
			closed = true
			break
		}
		frontmatter.WriteString(scanner.Text())
		frontmatter.WriteByte('\n')
	}
	if !closed {
		return PageMetadata{}, []Finding{{Code: "metadata_invalid", Severity: SeverityError, Path: path, Detail: "frontmatter closing delimiter is missing"}}
	}
	var metadata PageMetadata
	decoder := yaml.NewDecoder(strings.NewReader(frontmatter.String()))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil {
		findings = append(findings, Finding{Code: "metadata_invalid", Severity: SeverityError, Path: path, Detail: "frontmatter does not match knowledge page schema"})
	}
	return metadata, findings
}

func (s *scanState) validateMetadata(path string, meta PageMetadata) {
	if meta.KnowledgeVersion != 1 {
		s.add("metadata_version_invalid", SeverityError, path, "knowledge_version must equal 1")
	}
	if strings.TrimSpace(meta.Title) == "" {
		s.add("metadata_title_missing", SeverityError, path, "title is required")
	}
	if !allowedStatuses[meta.Status] {
		s.add("metadata_status_invalid", SeverityError, path, "status is not recognized")
	} else {
		s.incrementStatus(meta.Status)
	}
	s.report.PageCount++
	if strings.TrimSpace(meta.Owner) == "" {
		s.add("metadata_owner_missing", SeverityError, path, "owner is required")
	}
	if _, err := time.Parse(time.DateOnly, meta.LastVerified); err != nil {
		s.add("last_verified_invalid", SeverityError, path, "last_verified must use YYYY-MM-DD")
	}
	reviewBy, err := time.Parse(time.DateOnly, meta.ReviewBy)
	if err != nil {
		s.add("review_by_invalid", SeverityError, path, "review_by must use YYYY-MM-DD")
	} else if reviewBy.Before(dateOnly(s.opts.Now)) {
		s.add("review_overdue", SeverityWarning, path, "review_by is in the past")
	}
	if !shaPattern.MatchString(meta.SourceSHA) {
		s.add("source_revision_invalid", SeverityError, path, "source_sha must be a 7-64 character hexadecimal revision")
	}
	if len(meta.Sources) == 0 {
		s.add("sources_missing", SeverityWarning, path, "page has no provenance sources")
	}
	validProvenance := 0
	for _, source := range meta.Sources {
		if s.validateSource(path, meta.SourceSHA, source) {
			validProvenance++
		}
	}
	if meta.Status == "verified" && validProvenance == 0 {
		s.add("verified_provenance_missing", SeverityError, path, "verified page requires at least one validated source")
	}
	s.validatePromotion(path, meta)
	for _, superseded := range meta.Supersedes {
		target, err := safeRelativeReference(s.paths.root, filepath.Dir(filepath.Join(s.paths.root, filepath.FromSlash(path))), superseded)
		if err != nil || target == "" {
			s.add("supersedes_path_invalid", SeverityError, path, "supersedes contains an unsafe path")
			continue
		}
		info, err := os.Lstat(target)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			s.add("supersedes_broken", SeverityError, path, "superseded page is missing or not a regular file")
		}
	}
}

func (s *scanState) validatePromotion(path string, meta PageMetadata) {
	if meta.PromotionTarget == "" && meta.PromotionCommit == "" {
		return
	}
	if meta.PromotionTarget == "" || meta.PromotionCommit == "" {
		s.add("promotion_metadata_invalid", SeverityError, path, "promotion target and commit must be recorded together")
		return
	}
	if meta.Status != "superseded" {
		s.add("promotion_metadata_invalid", SeverityError, path, "promoted page status must be superseded")
	}
	if !shaPattern.MatchString(meta.PromotionCommit) || !commitIsAncestor(s.paths.project, meta.PromotionCommit) {
		s.add("promotion_commit_invalid", SeverityError, path, "promotion commit is invalid or not an ancestor of HEAD")
		return
	}
	_, target, err := resolveCanonicalTarget(s.paths, s.manifest, meta.PromotionTarget)
	if err != nil {
		s.add("promotion_target_invalid", SeverityError, path, "promotion target is missing, unsafe, or outside canonical roots")
		return
	}
	match, err := sourceMatchesRevision(s.paths.project, target, meta.PromotionCommit, s.opts.MaxPageBytes)
	if err != nil || !match {
		s.add("promotion_target_stale", SeverityError, path, "promotion target differs from the recorded reviewed commit")
	}
}

func (s *scanState) validateSource(pagePath, sourceSHA string, source Source) bool {
	class, ok := s.manifest.Classes[source.Class]
	if !ok || strings.TrimSpace(source.Class) == "" {
		s.add("source_class_unknown", SeverityError, pagePath, "source class is not configured")
		return false
	}
	switch class.Kind {
	case "project_file":
		if source.Path == "" || source.Fairway != nil {
			s.add("source_identity_invalid", SeverityError, pagePath, "project_file source requires only a path identity")
			return false
		}
		path, err := safeProjectFile(s.paths, source.Path)
		if err != nil {
			s.add("source_path_invalid", SeverityError, pagePath, "cited source is missing, unsafe, or not a regular file")
			return false
		}
		if !sourceWithinAllowedRoots(source.Path, class.Roots, s.paths.relRoot) {
			s.add("source_root_invalid", SeverityError, pagePath, "cited source is outside the source class allowed roots")
			return false
		}
		if !shaPattern.MatchString(sourceSHA) {
			return false
		}
		match, err := sourceMatchesRevision(s.paths.project, path, sourceSHA, s.opts.MaxPageBytes)
		if err != nil {
			s.add("source_revision_unverifiable", SeverityError, pagePath, "cited source cannot be verified at source_sha")
			return false
		}
		if !match {
			s.add("source_revision_stale", SeverityWarning, pagePath, "cited source content differs from source_sha")
			return false
		}
		return true
	case "fairway":
		if source.Path != "" || source.Fairway == nil {
			s.add("source_identity_invalid", SeverityError, pagePath, "fairway source requires only a typed Fairway identity")
			return false
		}
		ref := *source.Fairway
		if ref.Kind != class.FairwayKind || !fairwayIDPattern.MatchString(ref.ID) {
			s.add("fairway_source_invalid", SeverityError, pagePath, "Fairway source kind or id is invalid")
			return false
		}
		requirement := FairwayReferenceRequirement{PagePath: pagePath, SourceClass: source.Class, Reference: ref}
		s.report.FairwayReferences = append(s.report.FairwayReferences, requirement)
		if s.opts.ValidateFairwayReference == nil {
			s.add("fairway_source_validation_required", SeverityError, pagePath, "Fairway source requires coordinator store validation")
			return false
		}
		if err := s.opts.ValidateFairwayReference(requirement); err != nil {
			s.add("fairway_source_not_found", SeverityError, pagePath, "Fairway source was not validated by the coordinator store")
			return false
		}
		return true
	default:
		return false
	}
}

func safeSourceRoot(root string) bool {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(root)))
	return clean != "" && clean != "." && clean != ".." && !filepath.IsAbs(clean) && !strings.HasPrefix(clean, "../")
}

func isLegacyMemoryRoot(root string) bool {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(root)))
	return clean == "tmp-ux" || strings.HasPrefix(clean, "tmp-ux/")
}

func sourceWithinAllowedRoots(sourcePath string, roots []string, knowledgeRoot string) bool {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(sourcePath)))
	knowledge := filepath.ToSlash(filepath.Clean(knowledgeRoot))
	if clean == knowledge || strings.HasPrefix(clean, knowledge+"/") {
		return false
	}
	for _, root := range roots {
		allowed := filepath.ToSlash(filepath.Clean(strings.TrimSpace(root)))
		if clean == allowed || strings.HasPrefix(clean, allowed+"/") {
			return true
		}
	}
	return false
}

func sourceMatchesRevision(projectRoot, currentPath, revision string, limit int64) (bool, error) {
	rel, err := filepath.Rel(projectRoot, currentPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, errors.New("source path escapes project")
	}
	current, err := readBounded(currentPath, limit)
	if err != nil {
		return false, err
	}
	var committed boundedBuffer
	committed.limit = limit
	cmd := exec.Command("git", "-C", projectRoot, "show", revision+":"+filepath.ToSlash(rel))
	cmd.Stdout = &committed
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return false, errors.New("read source at revision")
	}
	return bytes.Equal(current, committed.Bytes()), nil
}

type boundedBuffer struct {
	bytes.Buffer
	limit int64
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if int64(b.Len()+len(p)) > b.limit {
		return 0, errors.New("source revision content exceeds configured limit")
	}
	return b.Buffer.Write(p)
}

func (s *scanState) validateIdentities() {
	for _, paths := range s.identities {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		for _, path := range paths {
			s.add("duplicate_identity", SeverityError, path, "normalized page title is not unique")
		}
	}
}

func (s *scanState) validateLinks() {
	for page, links := range s.links {
		base := filepath.Dir(filepath.Join(s.paths.root, filepath.FromSlash(page)))
		for _, link := range links {
			target, err := safeRelativeReference(s.paths.project, base, link)
			if err != nil {
				s.add("link_path_invalid", SeverityError, page, "Markdown link is unsafe or escapes the project")
				continue
			}
			if target == "" {
				continue
			}
			info, err := os.Lstat(target)
			if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				s.add("link_broken", SeverityError, page, "Markdown link target is missing or not a regular file")
			}
		}
	}
}

func (s *scanState) validateIndex() {
	if _, ok := s.pageByPath["index.md"]; !ok {
		s.add("index_missing", SeverityError, "index.md", "knowledge index is required")
		return
	}
	reachable := map[string]bool{"index.md": true}
	queue := []string{"index.md"}
	for len(queue) > 0 {
		page := queue[0]
		queue = queue[1:]
		base := filepath.Dir(filepath.Join(s.paths.root, filepath.FromSlash(page)))
		for _, link := range s.links[page] {
			target, err := safeRelativeReference(s.paths.project, base, link)
			if err != nil || target == "" {
				continue
			}
			rel, err := filepath.Rel(s.paths.root, target)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			rel = filepath.ToSlash(rel)
			if _, ok := s.pageByPath[rel]; ok && !reachable[rel] {
				reachable[rel] = true
				queue = append(queue, rel)
			}
		}
	}
	for path := range s.pageByPath {
		index := s.pageByPath[path]
		page := s.report.Pages[index]
		page.Reachable = reachable[path]
		s.report.Pages[index] = page
		if path == "README.md" || path == "log.md" || reachable[path] {
			continue
		}
		s.add("page_orphaned", SeverityWarning, path, "page is not reachable from index.md")
	}
}

func safeRelativeReference(limitRoot, base, reference string) (string, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" || strings.HasPrefix(reference, "#") {
		return "", nil
	}
	lower := strings.ToLower(reference)
	if strings.Contains(lower, "://") || strings.HasPrefix(lower, "mailto:") {
		return "", nil
	}
	if index := strings.IndexByte(reference, '#'); index >= 0 {
		reference = reference[:index]
	}
	if index := strings.IndexByte(reference, '?'); index >= 0 {
		reference = reference[:index]
	}
	if reference == "" {
		return "", nil
	}
	if filepath.IsAbs(reference) {
		return "", errors.New("absolute reference")
	}
	target := filepath.Clean(filepath.Join(base, filepath.FromSlash(reference)))
	rel, err := filepath.Rel(limitRoot, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("reference escapes root")
	}
	if err := validateExistingPathChain(limitRoot, target); err != nil {
		return "", err
	}
	return target, nil
}

func extractLinks(data []byte) []string {
	matches := markdownLinkPattern.FindAllSubmatch(data, -1)
	links := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			links = append(links, string(match[1]))
		}
	}
	return links
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("page grew beyond configured size limit while reading")
	}
	return data, nil
}

func containsSecret(data []byte) bool {
	return secretscan.Contains(data)
}

func normalizedIdentity(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *scanState) incrementStatus(status string) {
	switch status {
	case "verified":
		s.report.VerifiedCount++
	case "draft":
		s.report.DraftCount++
	case "stale":
		s.report.StaleCount++
	case "conflicted":
		s.report.ConflictedCount++
	case "superseded":
		s.report.SupersededCount++
	}
}

func (s *scanState) add(code string, severity Severity, path, detail string) {
	s.report.Findings = append(s.report.Findings, Finding{Code: code, Severity: severity, Path: path, Detail: detail})
}

func (s *scanState) sort() {
	sort.Slice(s.report.Pages, func(i, j int) bool { return s.report.Pages[i].Path < s.report.Pages[j].Path })
	sort.Slice(s.report.FairwayReferences, func(i, j int) bool {
		a, b := s.report.FairwayReferences[i], s.report.FairwayReferences[j]
		if a.PagePath != b.PagePath {
			return a.PagePath < b.PagePath
		}
		if a.SourceClass != b.SourceClass {
			return a.SourceClass < b.SourceClass
		}
		if a.Reference.Kind != b.Reference.Kind {
			return a.Reference.Kind < b.Reference.Kind
		}
		return a.Reference.ID < b.Reference.ID
	})
	sort.Slice(s.report.Findings, func(i, j int) bool {
		a, b := s.report.Findings[i], s.report.Findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Detail < b.Detail
	})
}
