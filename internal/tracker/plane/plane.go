package plane

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/store"
	"github.com/subashram/fairway/internal/tracker"
)

const (
	EnvBaseURL   = "PLANE_BASE_URL"
	EnvWorkspace = "PLANE_WORKSPACE"
	EnvProject   = "PLANE_PROJECT"
	EnvAPIToken  = "PLANE_API_TOKEN"
)

type Config struct {
	BaseURL      string `json:"base_url"`
	Workspace    string `json:"workspace"`
	Project      string `json:"project"`
	TokenSource  string `json:"token_source,omitempty"`
	TokenPresent bool   `json:"token_present"`
}

type IssuePayload struct {
	Provider      string            `json:"provider"`
	Workspace     string            `json:"workspace,omitempty"`
	Project       string            `json:"project,omitempty"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Priority      string            `json:"priority,omitempty"`
	State         string            `json:"state,omitempty"`
	Labels        []string          `json:"labels,omitempty"`
	ExternalRef   string            `json:"external_ref,omitempty"`
	CustomFields  map[string]string `json:"custom_fields,omitempty"`
	PlanningOnly  bool              `json:"planning_only"`
	SourceTaskID  string            `json:"source_task_id,omitempty"`
	ApplyBoundary string            `json:"apply_boundary"`
}

type CommentPayload struct {
	Provider      string `json:"provider"`
	Workspace     string `json:"workspace,omitempty"`
	Project       string `json:"project,omitempty"`
	ExternalID    string `json:"external_id,omitempty"`
	Body          string `json:"body"`
	PlanningOnly  bool   `json:"planning_only"`
	SourceTaskID  string `json:"source_task_id"`
	ApplyBoundary string `json:"apply_boundary"`
}

type ImportPreview struct {
	Provider      string                 `json:"provider"`
	Workspace     string                 `json:"workspace,omitempty"`
	Project       string                 `json:"project,omitempty"`
	DryRun        bool                   `json:"dry_run"`
	Tasks         []store.TaskDefinition `json:"tasks"`
	Mappings      []tracker.Mapping      `json:"mappings"`
	PlanningOnly  bool                   `json:"planning_only"`
	ApplyBoundary string                 `json:"apply_boundary"`
}

type Fixture struct {
	Workspace struct {
		Slug string `yaml:"slug"`
		Name string `yaml:"name"`
	} `yaml:"workspace"`
	Project struct {
		Identifier  string `yaml:"identifier"`
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	} `yaml:"project"`
	Issues []FixtureIssue `yaml:"issues"`
}

type FixtureIssue struct {
	FairwayID   string   `yaml:"fairway_id"`
	Parent      string   `yaml:"parent"`
	Title       string   `yaml:"title"`
	Type        string   `yaml:"type"`
	Role        string   `yaml:"role"`
	Domain      string   `yaml:"domain"`
	Kind        string   `yaml:"kind"`
	Module      string   `yaml:"module"`
	Cycle       string   `yaml:"cycle"`
	Labels      []string `yaml:"labels"`
	Description string   `yaml:"description"`
	Acceptance  []string `yaml:"acceptance"`
	Comments    []string `yaml:"comments"`
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		BaseURL:      strings.TrimSpace(os.Getenv(EnvBaseURL)),
		Workspace:    strings.TrimSpace(os.Getenv(EnvWorkspace)),
		Project:      strings.TrimSpace(os.Getenv(EnvProject)),
		TokenSource:  EnvAPIToken,
		TokenPresent: strings.TrimSpace(os.Getenv(EnvAPIToken)) != "",
	}
	var missing []string
	if cfg.BaseURL == "" {
		missing = append(missing, EnvBaseURL)
	}
	if cfg.Workspace == "" {
		missing = append(missing, EnvWorkspace)
	}
	if cfg.Project == "" {
		missing = append(missing, EnvProject)
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("missing Plane configuration: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func ExportIssue(task store.Task, evidence []store.Evidence, reviews []store.Review, cfg Config) IssuePayload {
	fields := map[string]string{
		"fairway_id":      task.Definition.ID,
		"fairway_status":  task.Status,
		"fairway_owner":   task.Owner,
		"role":            task.Definition.Role,
		"owning_domain":   task.Definition.OwningDomain,
		"kind":            task.Definition.Kind,
		"risk_level":      task.Definition.RiskLevel,
		"migration_type":  task.Definition.MigrationType,
		"evidence_count":  fmt.Sprintf("%d", len(evidence)),
		"review_count":    fmt.Sprintf("%d", len(reviews)),
		"planning_source": "fairway",
	}
	return IssuePayload{
		Provider:      "plane",
		Workspace:     cfg.Workspace,
		Project:       cfg.Project,
		Name:          task.Definition.Title,
		Description:   issueDescription(task, evidence, reviews),
		Priority:      priorityString(task.Definition.Priority),
		State:         "planning-mirror",
		Labels:        taskLabels(task),
		ExternalRef:   task.Definition.ID,
		CustomFields:  fields,
		PlanningOnly:  true,
		SourceTaskID:  task.Definition.ID,
		ApplyBoundary: "dry-run payload only; applying to Plane requires an explicit future apply command and must not mutate Fairway execution state",
	}
}

func ExportComment(task store.Task, evidence []store.Evidence, reviews []store.Review, externalID string, cfg Config) CommentPayload {
	return CommentPayload{
		Provider:      "plane",
		Workspace:     cfg.Workspace,
		Project:       cfg.Project,
		ExternalID:    externalID,
		Body:          commentBody(task, evidence, reviews),
		PlanningOnly:  true,
		SourceTaskID:  task.Definition.ID,
		ApplyBoundary: "dry-run comment only; applying to Plane requires explicit operator action and does not change Fairway state",
	}
}

func ImportFixture(fixture Fixture, cfg Config) (ImportPreview, error) {
	mappings, err := tracker.DefaultMappings("plane")
	if err != nil {
		return ImportPreview{}, err
	}
	preview := ImportPreview{
		Provider:      "plane",
		Workspace:     firstNonEmpty(cfg.Workspace, fixture.Workspace.Slug),
		Project:       firstNonEmpty(cfg.Project, fixture.Project.Identifier),
		DryRun:        true,
		Mappings:      mappings,
		PlanningOnly:  true,
		ApplyBoundary: "dry-run import preview only; applying to Fairway tasks requires explicit operator action and must not import Plane execution state",
	}
	for _, issue := range fixture.Issues {
		if strings.TrimSpace(issue.FairwayID) == "" || strings.TrimSpace(issue.Title) == "" {
			return ImportPreview{}, errors.New("fixture issues require fairway_id and title")
		}
		preview.Tasks = append(preview.Tasks, store.TaskDefinition{
			ID:               issue.FairwayID,
			ParentID:         issue.Parent,
			Kind:             firstNonEmpty(issue.Kind, issue.Type, "task"),
			Title:            issue.Title,
			Role:             issue.Role,
			Notes:            strings.TrimSpace(issue.Description),
			AcceptanceChecks: append([]string(nil), issue.Acceptance...),
			Profile:          "tracker-plane",
			OwningDomain:     issue.Domain,
			OwningLayer:      "tracker",
			ReviewDomains:    reviewDomainsFromLabels(issue.Labels),
			MigrationType:    "plane-planning-mirror",
		})
	}
	return preview, nil
}

func ApplyUnsupported() error {
	return errors.New("Plane apply is intentionally unsupported in this spike; use dry-run payloads only")
}

func issueDescription(task store.Task, evidence []store.Evidence, reviews []store.Review) string {
	var b strings.Builder
	if strings.TrimSpace(task.Definition.Notes) != "" {
		b.WriteString(strings.TrimSpace(task.Definition.Notes))
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "Fairway task: %s\n", task.Definition.ID)
	fmt.Fprintf(&b, "Status: %s\n", task.Status)
	fmt.Fprintf(&b, "Role: %s\n", task.Definition.Role)
	if task.Definition.OwningDomain != "" {
		fmt.Fprintf(&b, "Domain: %s\n", task.Definition.OwningDomain)
	}
	if len(task.Definition.AcceptanceChecks) > 0 {
		b.WriteString("\nAcceptance checks:\n")
		for _, check := range task.Definition.AcceptanceChecks {
			fmt.Fprintf(&b, "- %s\n", check)
		}
	}
	if len(evidence) > 0 {
		b.WriteString("\nLatest evidence:\n")
		for _, ev := range latestEvidence(evidence, 3) {
			fmt.Fprintf(&b, "- %s %s %s\n", ev.Result, ev.ArtifactType, ev.ArtifactPath)
		}
	}
	if len(reviews) > 0 {
		b.WriteString("\nReviews:\n")
		for _, review := range reviews {
			fmt.Fprintf(&b, "- %s by %s\n", review.Verdict, review.Reviewer)
		}
	}
	b.WriteString("\nBoundary: Plane is a planning mirror. Fairway remains authoritative for execution state, evidence, reviews, and gates.\n")
	return strings.TrimSpace(b.String())
}

func commentBody(task store.Task, evidence []store.Evidence, reviews []store.Review) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Fairway execution summary for `%s`:\n", task.Definition.ID)
	fmt.Fprintf(&b, "- status: %s\n", task.Status)
	fmt.Fprintf(&b, "- owner: %s\n", firstNonEmpty(task.Owner, task.Definition.Role))
	fmt.Fprintf(&b, "- evidence rows: %d\n", len(evidence))
	fmt.Fprintf(&b, "- review rows: %d\n", len(reviews))
	if len(evidence) > 0 {
		latest := latestEvidence(evidence, 1)[0]
		fmt.Fprintf(&b, "- latest evidence: %s %s %s\n", latest.Result, latest.ArtifactType, latest.ArtifactPath)
	}
	b.WriteString("- boundary: Plane comment mirrors Fairway state; it does not drive Fairway state.\n")
	return b.String()
}

func taskLabels(task store.Task) []string {
	labels := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			labels = append(labels, value)
		}
	}
	add("fairway")
	add("role:" + task.Definition.Role)
	add("kind:" + task.Definition.Kind)
	add("domain:" + task.Definition.OwningDomain)
	for _, domain := range task.Definition.ReviewDomains {
		add("review:" + domain)
	}
	sort.Strings(labels)
	return labels
}

func latestEvidence(evidence []store.Evidence, limit int) []store.Evidence {
	out := append([]store.Evidence(nil), evidence...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func reviewDomainsFromLabels(labels []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, label := range labels {
		domain, ok := strings.CutPrefix(label, "review:")
		if !ok {
			continue
		}
		domain = strings.TrimSpace(domain)
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		out = append(out, domain)
	}
	sort.Strings(out)
	return out
}

func priorityString(priority *int) string {
	if priority == nil {
		return ""
	}
	return fmt.Sprintf("P%d", *priority)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
