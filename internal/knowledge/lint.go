package knowledge

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	markdownLinkPattern = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+[^)]*)?\)`)
	shaPattern          = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)
	secretPatterns      = []regexp.Regexp{
		*regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY-----`),
		*regexp.MustCompile(`(?i)\bauthorization\s*:\s*(?:bearer|basic)\s+\S+`),
		*regexp.MustCompile(`(?i)\b(?:access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|api[_-]?key|password|passwd|cookie)\s*[:=]\s*["']?[^\s"']{4,}`),
		*regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		*regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`),
		*regexp.MustCompile(`\bsk_live_[A-Za-z0-9]{16,}\b`),
	}
	allowedStatuses = map[string]bool{
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
	if err := state.walk(); err != nil {
		return Report{}, err
	}
	state.validateIdentities()
	state.validateLinks()
	state.validateIndex()
	state.sort()
	return state.report, nil
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
	} else if s.opts.SourceRevision != "" && !sameRevision(meta.SourceSHA, s.opts.SourceRevision) {
		s.add("source_revision_stale", SeverityWarning, path, "page source revision differs from the requested revision")
	}
	if len(meta.Sources) == 0 {
		s.add("sources_missing", SeverityWarning, path, "page has no provenance sources")
	}
	for _, source := range meta.Sources {
		s.validateSource(path, source)
	}
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

func (s *scanState) validateSource(pagePath string, source Source) {
	count := 0
	if source.Path != "" {
		count++
	}
	if source.FairwayDecision != "" {
		count++
	}
	if source.FairwayEvidence != "" {
		count++
	}
	if count != 1 {
		s.add("source_identity_invalid", SeverityError, pagePath, "each source must contain exactly one supported identity")
		return
	}
	if source.Path == "" {
		return
	}
	if _, err := safeProjectFile(s.paths, source.Path); err != nil {
		s.add("source_path_invalid", SeverityError, pagePath, "cited source is missing, unsafe, or not a regular file")
	}
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
	for _, pattern := range secretPatterns {
		if pattern.Match(data) {
			return true
		}
	}
	return false
}

func normalizedIdentity(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

func sameRevision(page, current string) bool {
	page = strings.ToLower(strings.TrimSpace(page))
	current = strings.ToLower(strings.TrimSpace(current))
	return strings.HasPrefix(page, current) || strings.HasPrefix(current, page)
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
