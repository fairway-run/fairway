// Package knowledge manages project-owned engineering knowledge scaffolds and
// deterministic validation.
package knowledge

import "time"

const (
	// DefaultRoot is the conventional project-relative knowledge directory.
	DefaultRoot = "doc/agent-wiki"

	defaultMaxPages     = 2048
	defaultMaxPageBytes = int64(1 << 20)
	defaultMaxLinks     = 4096
)

// Options controls a bounded knowledge scan.
type Options struct {
	ProjectRoot    string
	KnowledgeRoot  string
	SourceRevision string
	Now            time.Time
	MaxPages       int
	MaxPageBytes   int64
	MaxLinks       int
}

// ScaffoldOptions controls creation of the initial knowledge tree.
type ScaffoldOptions struct {
	ProjectRoot   string
	KnowledgeRoot string
}

// ScaffoldResult reports files created without overwriting existing content.
type ScaffoldResult struct {
	Root     string   `json:"root"`
	Created  []string `json:"created"`
	Existing []string `json:"existing,omitempty"`
}

// PageMetadata is the maintained Markdown frontmatter contract.
type PageMetadata struct {
	KnowledgeVersion int      `yaml:"knowledge_version" json:"knowledge_version"`
	Title            string   `yaml:"title" json:"title"`
	Status           string   `yaml:"status" json:"status"`
	Owner            string   `yaml:"owner" json:"owner"`
	LastVerified     string   `yaml:"last_verified" json:"last_verified"`
	ReviewBy         string   `yaml:"review_by" json:"review_by"`
	SourceSHA        string   `yaml:"source_sha" json:"source_sha"`
	Sources          []Source `yaml:"sources" json:"sources"`
	Supersedes       []string `yaml:"supersedes" json:"supersedes"`
}

// Source identifies one safe provenance reference.
type Source struct {
	Path            string `yaml:"path,omitempty" json:"path,omitempty"`
	FairwayDecision string `yaml:"fairway_decision,omitempty" json:"fairway_decision,omitempty"`
	FairwayEvidence string `yaml:"fairway_evidence,omitempty" json:"fairway_evidence,omitempty"`
}

// Page is a bounded inventory entry.
type Page struct {
	Path      string       `json:"path"`
	Metadata  PageMetadata `json:"metadata"`
	LinkCount int          `json:"link_count"`
}

// Severity classifies a deterministic finding.
type Severity string

const (
	// SeverityError identifies invalid or unsafe knowledge state.
	SeverityError Severity = "error"
	// SeverityWarning identifies state requiring owner attention.
	SeverityWarning Severity = "warning"
)

// Finding is a bounded deterministic lint result. Detail never includes source
// content or a matched secret value.
type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Path     string   `json:"path,omitempty"`
	Detail   string   `json:"detail"`
}

// Report is the stable output of Status and Lint.
type Report struct {
	Root            string    `json:"root"`
	SourceRevision  string    `json:"source_revision,omitempty"`
	PageCount       int       `json:"page_count"`
	VerifiedCount   int       `json:"verified_count"`
	DraftCount      int       `json:"draft_count"`
	StaleCount      int       `json:"stale_count"`
	ConflictedCount int       `json:"conflicted_count"`
	SupersededCount int       `json:"superseded_count"`
	Pages           []Page    `json:"pages"`
	Findings        []Finding `json:"findings"`
}
