// Package knowledge manages project-owned engineering knowledge scaffolds and
// deterministic validation.
package knowledge

import "time"

const (
	// DefaultRoot is the conventional project-relative knowledge directory.
	DefaultRoot = "doc/agent-wiki"
	// DefaultSourceManifest is the conventional source-class manifest.
	DefaultSourceManifest = "sources.yaml"

	defaultMaxPages     = 2048
	defaultMaxPageBytes = int64(1 << 20)
	defaultMaxLinks     = 4096
	defaultQueryBudget  = 12 * 1024
	maxQueryBudget      = 32 * 1024
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
	// ValidateFairwayReference resolves a structurally valid Fairway reference
	// against the coordinator store. Without it, Fairway references are
	// reported as requiring validation and do not satisfy verified provenance.
	ValidateFairwayReference func(FairwayReferenceRequirement) error
	// CustodyHook is deterministic test instrumentation invoked only after
	// sensitive file descriptors are bound. Production callers leave it nil.
	CustodyHook func(stage string)
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

// SourceManifest defines the source classes accepted by maintained pages.
type SourceManifest struct {
	Version int                    `yaml:"knowledge_sources_version" json:"knowledge_sources_version"`
	Classes map[string]SourceClass `yaml:"classes" json:"classes"`
}

// SourceClass defines one configured provenance class.
type SourceClass struct {
	Kind                    string   `yaml:"kind" json:"kind"`
	Authority               string   `yaml:"authority" json:"authority"`
	Roots                   []string `yaml:"roots,omitempty" json:"roots,omitempty"`
	FairwayKind             string   `yaml:"fairway_kind,omitempty" json:"fairway_kind,omitempty"`
	RequiresStoreValidation bool     `yaml:"requires_store_validation,omitempty" json:"requires_store_validation,omitempty"`
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
	PromotionTarget  string   `yaml:"promotion_target,omitempty" json:"promotion_target,omitempty"`
	PromotionCommit  string   `yaml:"promotion_commit,omitempty" json:"promotion_commit,omitempty"`
}

// Source identifies one safe provenance reference.
type Source struct {
	Class   string            `yaml:"class" json:"class"`
	Path    string            `yaml:"path,omitempty" json:"path,omitempty"`
	Fairway *FairwayReference `yaml:"fairway,omitempty" json:"fairway,omitempty"`
}

// FairwayReference is a typed coordinator/store-backed provenance identity.
type FairwayReference struct {
	Kind string `yaml:"kind" json:"kind"`
	ID   string `yaml:"id" json:"id"`
}

// FairwayReferenceRequirement exposes a reference that the CLI/store
// integration must validate before it can satisfy verified provenance.
type FairwayReferenceRequirement struct {
	PagePath    string           `json:"page_path"`
	SourceClass string           `json:"source_class"`
	Reference   FairwayReference `json:"reference"`
}

// Page is a bounded inventory entry.
type Page struct {
	Path          string       `json:"path"`
	Metadata      PageMetadata `json:"metadata"`
	LinkCount     int          `json:"link_count"`
	Reachable     bool         `json:"reachable"`
	contentDigest [32]byte
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
	Root              string                        `json:"root"`
	SourceRevision    string                        `json:"source_revision,omitempty"`
	PageCount         int                           `json:"page_count"`
	VerifiedCount     int                           `json:"verified_count"`
	DraftCount        int                           `json:"draft_count"`
	StaleCount        int                           `json:"stale_count"`
	ConflictedCount   int                           `json:"conflicted_count"`
	SupersededCount   int                           `json:"superseded_count"`
	Pages             []Page                        `json:"pages"`
	Findings          []Finding                     `json:"findings"`
	FairwayReferences []FairwayReferenceRequirement `json:"fairway_references,omitempty"`
}

// Change describes one bounded project-file mutation proposed by a lifecycle
// command. Content is generated metadata and prose, never copied source text.
type Change struct {
	Path    string `json:"path"`
	Action  string `json:"action"`
	SHA256  string `json:"sha256"`
	Bytes   int    `json:"bytes"`
	Preview string `json:"preview"`
}

// IngestOptions controls deterministic, preview-first page creation.
type IngestOptions struct {
	Options
	SourcePath  string
	SourceClass string
	PagePath    string
	Title       string
	Owner       string
	ReviewBy    string
	Apply       bool
}

// IngestResult reports a bounded proposal and whether it was applied.
type IngestResult struct {
	Applied        bool     `json:"applied"`
	Preview        bool     `json:"preview"`
	SourcePath     string   `json:"source_path"`
	SourceClass    string   `json:"source_class"`
	SourceRevision string   `json:"source_revision"`
	Changes        []Change `json:"changes"`
}

// QueryOptions controls deterministic task/topic-aware knowledge selection.
type QueryOptions struct {
	Options
	Topic       string
	TaskID      string
	TaskTerms   []string
	MaxResults  int
	BudgetBytes int
}

// QuerySource is a deduplicated provenance reference.
type QuerySource struct {
	Key              string                `json:"key"`
	Class            string                `json:"class"`
	Authority        string                `json:"authority"`
	Kind             string                `json:"kind"`
	Path             string                `json:"path,omitempty"`
	FairwayID        string                `json:"fairway_id,omitempty"`
	Verified         bool                  `json:"verified"`
	MemoryReferenced bool                  `json:"memory_referenced,omitempty"`
	Citations        []QuerySourceCitation `json:"citations"`
}

// QuerySourceCitation preserves verification state for each selected page
// while the source identity itself is deduplicated.
type QuerySourceCitation struct {
	Page     string `json:"page"`
	Verified bool   `json:"verified"`
}

// QueryPage is a bounded selected-page projection.
type QueryPage struct {
	Path            string `json:"path"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	Owner           string `json:"owner"`
	ReviewBy        string `json:"review_by"`
	SourceSHA       string `json:"source_sha"`
	SourceFreshness string `json:"source_freshness"`
	Excerpt         string `json:"excerpt,omitempty"`
	Score           int    `json:"score"`
	SourceCount     int    `json:"source_count"`
	Verified        bool   `json:"verified"`
	Conflict        bool   `json:"conflict"`
	Stale           bool   `json:"stale"`
}

// QueryPacket is the stable bounded retrieval surface.
type QueryPacket struct {
	Schema             string        `json:"schema"`
	Topic              string        `json:"topic,omitempty"`
	TaskID             string        `json:"task_id,omitempty"`
	RepositoryRevision string        `json:"repository_revision,omitempty"`
	Pages              []QueryPage   `json:"pages"`
	Sources            []QuerySource `json:"sources"`
	Warnings           []string      `json:"warnings,omitempty"`
	BudgetBytes        int           `json:"budget_bytes"`
	Bytes              int           `json:"bytes"`
	Bounded            bool          `json:"bounded"`
	ReadOnly           bool          `json:"read_only"`
}

// PromoteOptions controls preview-first promotion recording.
type PromoteOptions struct {
	Options
	PagePath       string
	TargetPath     string
	ReviewedCommit string
	Apply          bool
}

// PromoteResult reports the verified canonical target and page mutation.
type PromoteResult struct {
	Applied        bool     `json:"applied"`
	Preview        bool     `json:"preview"`
	PagePath       string   `json:"page_path"`
	TargetPath     string   `json:"target_path"`
	ReviewedCommit string   `json:"reviewed_commit"`
	Changes        []Change `json:"changes"`
}
