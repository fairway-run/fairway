// Package memorymigration provides deterministic, bounded migration helpers for
// legacy project-local memory files. Fairway track memory remains authoritative.
package memorymigration

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxFileBytes = 256 * 1024
	maxScalar    = 1000
	maxItem      = 500
	maxItems     = 12
)

// Proposal is the bounded subset of a legacy Markdown file that can be
// proposed for the existing Fairway track-memory record.
type Proposal struct {
	Title               string   `json:"title,omitempty"`
	Purpose             string   `json:"purpose,omitempty"`
	OperatingMode       string   `json:"operating_mode,omitempty"`
	ActiveScope         string   `json:"active_scope,omitempty"`
	CurrentObjective    string   `json:"current_objective,omitempty"`
	Decisions           []string `json:"decisions,omitempty"`
	Blockers            []string `json:"blockers,omitempty"`
	OpenQuestions       []string `json:"open_questions,omitempty"`
	NextActions         []string `json:"next_actions,omitempty"`
	SourceCheckpointIDs []int64  `json:"source_checkpoint_ids,omitempty"`
	SourceEvidenceIDs   []int64  `json:"source_evidence_ids,omitempty"`
	SourceReviewIDs     []int64  `json:"source_review_ids,omitempty"`
	Owner               string   `json:"owner,omitempty"`
	ReviewBy            string   `json:"review_by,omitempty"`
}

// Document describes a safely loaded legacy memory file without retaining its
// raw body.
type Document struct {
	Path       string   `json:"path"`
	SHA256     string   `json:"sha256"`
	SizeBytes  int64    `json:"size_bytes"`
	Proposal   Proposal `json:"proposal"`
	Warnings   []string `json:"warnings,omitempty"`
	RawOmitted bool     `json:"raw_omitted"`
	IssueCode  string   `json:"issue_code,omitempty"`
}

// Memory is the track-memory projection required for coverage comparison.
type Memory struct {
	TrackID             string
	Title               string
	Purpose             string
	OperatingMode       string
	ActiveScope         string
	CurrentObjective    string
	Decisions           []string
	Blockers            []string
	OpenQuestions       []string
	NextActions         []string
	Owner               string
	ReviewBy            string
	Disposition         string
	SourceFactCount     int
	SourceCheckpointIDs []int64
	SourceEvidenceIDs   []int64
	SourceReviewIDs     []int64
}

// Coverage reports whether one legacy file is represented by one authoritative
// track-memory record.
type Coverage struct {
	Path             string `json:"path"`
	SHA256           string `json:"sha256"`
	Status           string `json:"status"`
	TrackID          string `json:"track_id,omitempty"`
	Disposition      string `json:"disposition,omitempty"`
	RepresentedFacts int    `json:"represented_facts"`
	ExtractedFacts   int    `json:"extracted_facts"`
	Reason           string `json:"reason"`
}

// RetirementPlan is a read-only plan. It never moves or deletes the source.
type RetirementPlan struct {
	Path              string `json:"path"`
	SHA256            string `json:"sha256"`
	TrackID           string `json:"track_id"`
	Reason            string `json:"reason"`
	CoverageStatus    string `json:"coverage_status"`
	Disposition       string `json:"disposition,omitempty"`
	Eligible          bool   `json:"eligible"`
	ReadOnly          bool   `json:"read_only"`
	DeletesSource     bool   `json:"deletes_source"`
	SuggestedEvidence string `json:"suggested_evidence"`
}

var (
	headingPattern = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)
	listPattern    = regexp.MustCompile(`^(?:[-*+]\s+|\d+[.)]\s+)(.*)$`)
	idPattern      = regexp.MustCompile(`\b[1-9][0-9]*\b`)
	secretAssign   = regexp.MustCompile("(?i)(access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|ssh[_-]?private[_-]?key|api[_-]?key|password|authorization|cookie|set-cookie|secret)\\s*[:=]\\s*[\\\"']?([^\\s\\\"'`]+)")
	bearerPattern  = regexp.MustCompile(`(?i)\bauthorization\s*:\s*bearer\s+\S+|\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`)
	jwtPattern     = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
	knownSecret    = regexp.MustCompile(`\b(?:sk_live_|rk_live_|ghp_|github_pat_|AKIA)[A-Za-z0-9_-]{8,}`)
	pemBoundary    = regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9][A-Z0-9 _-]*-----`)
	placeholderVar = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*\}$`)
	placeholderTag = regexp.MustCompile(`^<(?:redacted|masked|unset|none|example|changeme|secret|token|password)>$`)
	shellPrompt    = regexp.MustCompile(`^(?:[$#>]\s+|[A-Za-z0-9_.-]+@[^[:space:]:]+:[^$#]*[$#]\s+)`)
	logPrefix      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ][0-9:.+-]+(?:Z|[+-][0-9:]+)?\s+(?:TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\b`)
	stackFrame     = regexp.MustCompile(`^(?:goroutine \d+|panic:|at [A-Za-z0-9_.$/<>-]+\(|\s*File ".+", line \d+)`)
	commandPrefix  = regexp.MustCompile(`^(?:bash|sh|zsh|fish|curl|wget|git|go|make|docker|kubectl|helm|terraform|python3?|node|npm|pnpm|yarn|psql|ssh)\s+`)
)

// Load reads and extracts one repo-local tmp-ux memory Markdown file.
func Load(repositoryRoot, name string) (Document, error) {
	path, rel, info, err := resolveLegacyPath(repositoryRoot, name, false)
	if err != nil {
		return Document{}, err
	}
	if info.Size() > maxFileBytes {
		return Document{}, fmt.Errorf("legacy memory file exceeds %d bytes", maxFileBytes)
	}
	if err := rejectSensitive([]byte(filepath.ToSlash(rel))); err != nil {
		return Document{}, errors.New("legacy memory path contains prohibited secret-like material")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("read legacy memory file: %w", err)
	}
	if err := rejectSensitive(body); err != nil {
		return Document{}, err
	}
	proposal, warnings, err := extract(body)
	if err != nil {
		return Document{}, err
	}
	digest := sha256.Sum256(body)
	return Document{
		Path:       filepath.ToSlash(rel),
		SHA256:     hex.EncodeToString(digest[:]),
		SizeBytes:  info.Size(),
		Proposal:   proposal,
		Warnings:   warnings,
		RawOmitted: true,
	}, nil
}

// Discover inventories safely readable legacy memory Markdown files below a
// repo-local tmp-ux directory.
func Discover(repositoryRoot, scanRoot string) ([]Document, error) {
	if strings.TrimSpace(scanRoot) == "" {
		scanRoot = "tmp-ux"
	}
	path, _, _, err := resolveLegacyPath(repositoryRoot, scanRoot, true)
	if errors.Is(err, fs.ErrNotExist) {
		return []Document{}, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	err = filepath.WalkDir(path, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || !isMemoryMarkdown(entry.Name()) {
			return nil
		}
		names = append(names, candidate)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory legacy memory files: %w", err)
	}
	sort.Strings(names)
	documents := make([]Document, 0, len(names))
	for _, name := range names {
		document, err := Load(repositoryRoot, name)
		if err != nil {
			rel, relErr := filepath.Rel(repositoryRoot, name)
			if relErr != nil {
				return nil, errors.New("resolve rejected legacy memory inventory path")
			}
			documents = append(documents, Document{Path: filepath.ToSlash(rel), RawOmitted: true, IssueCode: migrationIssueCode(err), Warnings: []string{"file rejected by bounded migration safety checks"}})
			continue
		}
		documents = append(documents, document)
	}
	return documents, nil
}

// ValidateProposal applies the same secret and size checks to CLI overrides as
// to extracted Markdown.
func ValidateProposal(proposal Proposal) error {
	values := []string{proposal.Title, proposal.Purpose, proposal.OperatingMode, proposal.ActiveScope, proposal.CurrentObjective, proposal.Owner, proposal.ReviewBy}
	values = append(values, proposal.Decisions...)
	values = append(values, proposal.Blockers...)
	values = append(values, proposal.OpenQuestions...)
	values = append(values, proposal.NextActions...)
	for _, value := range values {
		if len(value) > maxScalar {
			return errors.New("memory proposal contains an overlong field")
		}
		if err := rejectSensitive([]byte(value)); err != nil {
			return err
		}
	}
	return nil
}

// ValidateSafeText applies the migration secret scanner without retaining or
// returning the inspected value.
func ValidateSafeText(value string) error {
	return rejectSensitive([]byte(value))
}

// ValidateRendered applies an aggregate size bound and the same secret scan to
// a fully rendered packet.
func ValidateRendered(rendered []byte, maxBytes int) error {
	if maxBytes <= 0 || len(rendered) > maxBytes {
		return errors.New("rendered memory packet exceeds the aggregate size limit")
	}
	if err := rejectSensitive(rendered); err != nil {
		return errors.New("rendered memory packet contains prohibited secret-like material")
	}
	return nil
}

// AssessCoverage compares extracted facts to authoritative memory. An explicit
// track restricts comparison and prevents accidental cross-track attribution.
func AssessCoverage(document Document, memories []Memory, track string) Coverage {
	result := Coverage{Path: document.Path, SHA256: document.SHA256, Status: "uncovered", Reason: "no authoritative track memory represents the extracted facts"}
	if document.IssueCode != "" {
		result.Status = "rejected"
		result.Reason = document.IssueCode
		return result
	}
	extracted := factCount(document.Proposal)
	result.ExtractedFacts = extracted
	if extracted == 0 {
		result.Status = "no_extractable_memory"
		result.Reason = "file has no supported bounded memory fields"
		return result
	}
	var matches []Coverage
	for _, memory := range memories {
		if strings.TrimSpace(track) != "" && memory.TrackID != strings.TrimSpace(track) {
			continue
		}
		represented := representedFacts(document.Proposal, memory)
		if represented != extracted {
			continue
		}
		matches = append(matches, Coverage{Path: document.Path, SHA256: document.SHA256, Status: "covered", TrackID: memory.TrackID, Disposition: memory.Disposition, RepresentedFacts: represented, ExtractedFacts: extracted, Reason: "all extracted bounded facts are represented by track memory"})
	}
	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		result.Status = "ambiguous"
		result.Reason = "multiple track memories represent the extracted facts; specify --track"
		return result
	}
	if strings.TrimSpace(track) != "" {
		result.TrackID = strings.TrimSpace(track)
		result.Reason = "selected track memory does not represent all extracted bounded facts"
	}
	return result
}

func migrationIssueCode(err error) string {
	text := err.Error()
	switch {
	case strings.Contains(text, "secret-like"), strings.Contains(text, "raw or transcript-like"):
		return "unsafe_content"
	case strings.Contains(text, "exceeds"):
		return "size_limit"
	default:
		return "invalid_legacy_memory"
	}
}

// PlanRetirement produces a non-mutating plan only when the selected track
// covers the file and has a durable non-active disposition.
func PlanRetirement(document Document, memory Memory, reason string) RetirementPlan {
	coverage := AssessCoverage(document, []Memory{memory}, memory.TrackID)
	disposed := memory.Disposition == "archived" || memory.Disposition == "superseded"
	eligible := coverage.Status == "covered" && disposed && memory.SourceFactCount > 0 && strings.TrimSpace(reason) != ""
	return RetirementPlan{
		Path:              document.Path,
		SHA256:            document.SHA256,
		TrackID:           memory.TrackID,
		Reason:            strings.TrimSpace(reason),
		CoverageStatus:    coverage.Status,
		Disposition:       memory.Disposition,
		Eligible:          eligible,
		ReadOnly:          true,
		DeletesSource:     false,
		SuggestedEvidence: fmt.Sprintf("record reviewed retirement evidence for %s sha256=%s track=%s; archive manually after approval", document.Path, document.SHA256, memory.TrackID),
	}
}

func resolveLegacyPath(repositoryRoot, name string, directory bool) (string, string, fs.FileInfo, error) {
	rootInput, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve repository root: %w", err)
	}
	root, err := filepath.EvalSymlinks(rootInput)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve repository root: %w", err)
	}
	cleanName := filepath.Clean(name)
	var rel string
	if filepath.IsAbs(cleanName) {
		candidateInput, absErr := filepath.Abs(cleanName)
		if absErr != nil {
			return "", "", nil, fmt.Errorf("resolve legacy memory path: %w", absErr)
		}
		rel, err = filepath.Rel(rootInput, candidateInput)
		if !insideRelative(rel, err) {
			rel, err = filepath.Rel(root, candidateInput)
		}
		if !insideRelative(rel, err) {
			resolvedParent, resolveErr := filepath.EvalSymlinks(filepath.Dir(candidateInput))
			if resolveErr == nil {
				rel, err = filepath.Rel(root, filepath.Join(resolvedParent, filepath.Base(candidateInput)))
			}
		}
	} else {
		rel = cleanName
	}
	if !insideRelative(rel, err) {
		return "", "", nil, errors.New("legacy memory path must stay inside the repository")
	}
	candidate := filepath.Join(root, rel)
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	if len(parts) == 0 || parts[0] != "tmp-ux" {
		return "", "", nil, errors.New("legacy memory path must be below tmp-ux")
	}
	walk := root
	for _, part := range parts {
		walk = filepath.Join(walk, part)
		info, statErr := os.Lstat(walk)
		if statErr != nil {
			return "", "", nil, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", "", nil, errors.New("legacy memory path must not contain symlinks")
		}
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", "", nil, err
	}
	if directory {
		if !info.IsDir() {
			return "", "", nil, errors.New("legacy memory inventory root must be a directory")
		}
	} else {
		if !info.Mode().IsRegular() || !isMemoryMarkdown(info.Name()) {
			return "", "", nil, errors.New("legacy memory input must be a regular Markdown file with memory in its name")
		}
	}
	return candidate, rel, info, nil
}

func insideRelative(rel string, err error) bool {
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func isMemoryMarkdown(name string) bool {
	lower := strings.ToLower(name)
	ext := strings.ToLower(filepath.Ext(lower))
	return strings.Contains(lower, "memory") && (ext == ".md" || ext == ".markdown")
}

func rejectSensitive(body []byte) error {
	text := string(body)
	if pemBoundary.MatchString(text) || bearerPattern.MatchString(text) || jwtPattern.MatchString(text) || knownSecret.MatchString(text) {
		return errors.New("legacy memory file contains prohibited secret-like material")
	}
	for _, match := range secretAssign.FindAllStringSubmatch(text, -1) {
		if len(match) < 3 || placeholder(match[2]) {
			continue
		}
		return errors.New("legacy memory file contains prohibited secret-like material")
	}
	return nil
}

func placeholder(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if placeholderVar.MatchString(value) || placeholderTag.MatchString(value) {
		return true
	}
	value = strings.Trim(value, "[]{}()*,.;")
	if value == "" || value == "redacted" || value == "none" || value == "unset" || value == "example" || value == "changeme" || value == "masked" || value == "xxx" {
		return true
	}
	return false
}

func extract(body []byte) (Proposal, []string, error) {
	var proposal Proposal
	sections := map[string][]string{}
	current := ""
	inFence := false
	warnings := []string{}
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	scanner.Buffer(make([]byte, 4096), maxFileBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			if !inFence && current != "" {
				return Proposal{}, nil, errors.New("legacy memory file contains raw or transcript-like content")
			}
			inFence = !inFence
			if inFence {
				warnings = appendUnique(warnings, "fenced code was omitted from extraction")
			}
			continue
		}
		if inFence || line == "" {
			continue
		}
		if match := headingPattern.FindStringSubmatch(line); len(match) == 2 {
			label := normalizeHeading(match[1])
			if prohibitedSection(label) {
				return Proposal{}, nil, errors.New("legacy memory file contains a prohibited raw log, transcript, prompt, or tool-output section")
			}
			if proposal.Title == "" && strings.HasPrefix(line, "# ") {
				proposal.Title = bounded(cleanMarkdown(match[1]), maxScalar, &warnings)
			}
			current = canonicalSection(label)
			continue
		}
		if key, value, ok := inlineField(line); ok {
			if looksRawContent(value) {
				return Proposal{}, nil, errors.New("legacy memory file contains raw or transcript-like content")
			}
			sections[key] = append(sections[key], value)
			continue
		}
		if current != "" {
			if looksRawContent(line) {
				return Proposal{}, nil, errors.New("legacy memory file contains raw or transcript-like content")
			}
			if listSection(current) && listPattern.FindStringSubmatch(line) == nil {
				return Proposal{}, nil, errors.New("legacy memory list fields require explicit Markdown list items")
			}
			sections[current] = append(sections[current], line)
		}
	}
	if err := scanner.Err(); err != nil {
		return Proposal{}, nil, fmt.Errorf("scan legacy memory file: %w", err)
	}
	proposal.Purpose = scalar(sections["purpose"], &warnings)
	proposal.OperatingMode = scalar(sections["operating_mode"], &warnings)
	proposal.ActiveScope = scalar(sections["active_scope"], &warnings)
	proposal.CurrentObjective = scalar(sections["current_objective"], &warnings)
	proposal.Owner = scalar(sections["owner"], &warnings)
	proposal.ReviewBy = scalar(sections["review_by"], &warnings)
	proposal.Decisions = items(sections["decisions"], &warnings)
	proposal.Blockers = items(sections["blockers"], &warnings)
	proposal.OpenQuestions = items(sections["open_questions"], &warnings)
	proposal.NextActions = items(sections["next_actions"], &warnings)
	proposal.SourceCheckpointIDs = ids(sections["source_checkpoint_ids"])
	proposal.SourceEvidenceIDs = ids(sections["source_evidence_ids"])
	proposal.SourceReviewIDs = ids(sections["source_review_ids"])
	if err := ValidateProposal(proposal); err != nil {
		return Proposal{}, nil, err
	}
	return proposal, warnings, nil
}

func normalizeHeading(value string) string {
	value = strings.ToLower(cleanMarkdown(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func canonicalSection(label string) string {
	switch label {
	case "purpose":
		return "purpose"
	case "operating mode", "mode":
		return "operating_mode"
	case "active scope", "scope":
		return "active_scope"
	case "current objective", "objective":
		return "current_objective"
	case "decisions", "accepted decisions":
		return "decisions"
	case "blockers", "current blockers":
		return "blockers"
	case "open questions", "questions":
		return "open_questions"
	case "next actions", "next action":
		return "next_actions"
	case "owner":
		return "owner"
	case "review by", "review date":
		return "review_by"
	case "source checkpoint ids", "source checkpoints":
		return "source_checkpoint_ids"
	case "source evidence ids", "source evidence":
		return "source_evidence_ids"
	case "source review ids", "source reviews":
		return "source_review_ids"
	default:
		return ""
	}
}

func prohibitedSection(label string) bool {
	switch label {
	case "raw log", "raw logs", "transcript", "transcripts", "raw transcript", "raw transcripts", "prompt", "prompt body", "raw prompt", "tool output", "tool outputs", "tool body", "tool bodies":
		return true
	default:
		return false
	}
}

func listSection(section string) bool {
	switch section {
	case "decisions", "blockers", "open_questions", "next_actions", "source_checkpoint_ids", "source_evidence_ids", "source_review_ids":
		return true
	default:
		return false
	}
}

func looksRawContent(line string) bool {
	line = strings.TrimSpace(line)
	if match := listPattern.FindStringSubmatch(line); len(match) == 2 {
		line = strings.TrimSpace(match[1])
	}
	lower := strings.ToLower(line)
	if line == "" {
		return false
	}
	if shellPrompt.MatchString(line) || logPrefix.MatchString(line) || stackFrame.MatchString(line) || commandPrefix.MatchString(lower) {
		return true
	}
	if (strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}")) ||
		(strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") && strings.Contains(line, `"`)) {
		return true
	}
	for _, prefix := range []string{
		"tool call:", "tool output:", "tool result:", "command output:", "stdout:", "stderr:",
		"exit code:", "process exited", "script running with cell id", "transcript:", "assistant:", "user:",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.Contains(lower, "<codex_delegation") ||
		strings.Contains(lower, "<tool_call") ||
		strings.Contains(lower, "custom_tool_call_output")
}

func inlineField(line string) (string, string, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := canonicalSection(normalizeHeading(parts[0]))
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(parts[1]), true
}

func scalar(lines []string, warnings *[]string) string {
	var values []string
	for _, line := range lines {
		line = cleanMarkdown(line)
		if line != "" {
			values = append(values, line)
		}
	}
	return bounded(strings.Join(values, " "), maxScalar, warnings)
}

func items(lines []string, warnings *[]string) []string {
	var out []string
	for _, line := range lines {
		if match := listPattern.FindStringSubmatch(strings.TrimSpace(line)); len(match) == 2 {
			line = match[1]
		}
		line = bounded(cleanMarkdown(line), maxItem, warnings)
		if line == "" {
			continue
		}
		out = appendUnique(out, line)
		if len(out) == maxItems {
			if len(lines) > len(out) {
				*warnings = appendUnique(*warnings, "a list was truncated to 12 items")
			}
			break
		}
	}
	return out
}

func ids(lines []string) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, line := range lines {
		for _, raw := range idPattern.FindAllString(line, -1) {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value <= 0 || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func cleanMarkdown(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSpace(strings.Trim(value, "`*_"))
	value = strings.TrimPrefix(value, "[ ] ")
	value = strings.TrimPrefix(value, "[x] ")
	value = strings.TrimPrefix(value, "[X] ")
	return strings.Join(strings.Fields(value), " ")
}

func bounded(value string, limit int, warnings *[]string) string {
	if len(value) <= limit {
		return value
	}
	*warnings = appendUnique(*warnings, fmt.Sprintf("extracted text was truncated to %d bytes", limit))
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func factCount(proposal Proposal) int {
	count := 0
	for _, value := range []string{proposal.Title, proposal.Purpose, proposal.OperatingMode, proposal.ActiveScope, proposal.CurrentObjective, proposal.Owner, proposal.ReviewBy} {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	count += len(proposal.Decisions) + len(proposal.Blockers) + len(proposal.OpenQuestions) + len(proposal.NextActions)
	count += len(proposal.SourceCheckpointIDs) + len(proposal.SourceEvidenceIDs) + len(proposal.SourceReviewIDs)
	return count
}

func representedFacts(proposal Proposal, memory Memory) int {
	count := 0
	for _, pair := range [][2]string{{proposal.Title, memory.Title}, {proposal.Purpose, memory.Purpose}, {proposal.OperatingMode, memory.OperatingMode}, {proposal.ActiveScope, memory.ActiveScope}, {proposal.CurrentObjective, memory.CurrentObjective}, {proposal.Owner, memory.Owner}, {proposal.ReviewBy, memory.ReviewBy}} {
		if strings.TrimSpace(pair[0]) != "" && normalizedEqual(pair[0], pair[1]) {
			count++
		}
	}
	for _, pair := range []struct{ proposed, stored []string }{{proposal.Decisions, memory.Decisions}, {proposal.Blockers, memory.Blockers}, {proposal.OpenQuestions, memory.OpenQuestions}, {proposal.NextActions, memory.NextActions}} {
		for _, value := range pair.proposed {
			if containsNormalized(pair.stored, value) {
				count++
			}
		}
	}
	for _, pair := range []struct{ proposed, stored []int64 }{{proposal.SourceCheckpointIDs, memory.SourceCheckpointIDs}, {proposal.SourceEvidenceIDs, memory.SourceEvidenceIDs}, {proposal.SourceReviewIDs, memory.SourceReviewIDs}} {
		for _, value := range pair.proposed {
			if containsInt64(pair.stored, value) {
				count++
			}
		}
	}
	return count
}

func normalizedEqual(left, right string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(left), " "), strings.Join(strings.Fields(right), " "))
}

func containsNormalized(values []string, target string) bool {
	for _, value := range values {
		if normalizedEqual(value, target) {
			return true
		}
	}
	return false
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
