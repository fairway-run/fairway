package tracker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

type Operation string

const (
	OperationConfigure     Operation = "configure"
	OperationDryRunImport  Operation = "dry_run_import"
	OperationLink          Operation = "link"
	OperationExportStatus  Operation = "export_status"
	OperationExportComment Operation = "export_comment"
	OperationReconcile     Operation = "reconcile"
	OperationResolve       Operation = "resolve"
)

type ProviderSpec struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	Operations []Operation `json:"operations"`
	Concepts   []string    `json:"concepts"`
}

type Reference struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
	URL        string `json:"url,omitempty"`
}

type ExternalIssue struct {
	Provider     string            `json:"provider"`
	ExternalID   string            `json:"external_id"`
	URL          string            `json:"url,omitempty"`
	Title        string            `json:"title,omitempty"`
	Description  string            `json:"description,omitempty"`
	State        string            `json:"state,omitempty"`
	Priority     string            `json:"priority,omitempty"`
	Parent       string            `json:"parent,omitempty"`
	Labels       []string          `json:"labels,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

type FairwayTask struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Notes            string   `json:"notes,omitempty"`
	ParentID         string   `json:"parent_id,omitempty"`
	Status           string   `json:"status,omitempty"`
	Priority         *int     `json:"priority,omitempty"`
	Role             string   `json:"role,omitempty"`
	OwningDomain     string   `json:"owning_domain,omitempty"`
	Kind             string   `json:"kind,omitempty"`
	ReviewDomains    []string `json:"review_domains,omitempty"`
	AcceptanceChecks []string `json:"acceptance_checks,omitempty"`
}

type Mapping struct {
	FairwayField string `json:"fairway_field"`
	TrackerField string `json:"tracker_field"`
	Direction    string `json:"direction"`
	Boundary     string `json:"boundary"`
}

type ProposedAction struct {
	Provider   string `json:"provider"`
	TaskID     string `json:"task_id,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	Mutates    string `json:"mutates"`
	DryRun     bool   `json:"dry_run"`
}

type ReconcileReport struct {
	DryRun       bool                `json:"dry_run"`
	Links        []store.TrackerLink `json:"links"`
	Actions      []ProposedAction    `json:"actions"`
	PlanningOnly bool                `json:"planning_only"`
	Note         string              `json:"note"`
}

var providers = map[string]ProviderSpec{
	"plane": {
		Name: "plane",
		Kind: "planning",
		Operations: []Operation{
			OperationConfigure,
			OperationDryRunImport,
			OperationLink,
			OperationExportStatus,
			OperationExportComment,
			OperationReconcile,
			OperationResolve,
		},
		Concepts: []string{"workspace", "project", "issue", "module", "cycle", "label", "comment"},
	},
	"jira": {
		Name: "jira",
		Kind: "planning",
		Operations: []Operation{
			OperationConfigure,
			OperationDryRunImport,
			OperationLink,
			OperationExportStatus,
			OperationExportComment,
			OperationReconcile,
			OperationResolve,
		},
		Concepts: []string{"site", "project", "issue", "epic", "label", "comment"},
	},
	"linear": {
		Name: "linear",
		Kind: "planning",
		Operations: []Operation{
			OperationConfigure,
			OperationDryRunImport,
			OperationLink,
			OperationExportStatus,
			OperationExportComment,
			OperationReconcile,
			OperationResolve,
		},
		Concepts: []string{"workspace", "team", "issue", "project", "cycle", "label", "comment"},
	},
}

func SupportedProviders() []ProviderSpec {
	out := make([]ProviderSpec, 0, len(providers))
	for _, provider := range providers {
		out = append(out, provider)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func ValidateProvider(name string) error {
	if _, ok := providers[strings.ToLower(strings.TrimSpace(name))]; !ok {
		return fmt.Errorf("unsupported tracker provider %q", name)
	}
	return nil
}

func Provider(name string) (ProviderSpec, error) {
	provider, ok := providers[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return ProviderSpec{}, fmt.Errorf("unsupported tracker provider %q", name)
	}
	return provider, nil
}

func DefaultMappings(provider string) ([]Mapping, error) {
	if _, err := Provider(provider); err != nil {
		return nil, err
	}
	return []Mapping{
		{"task_definitions.id", "external issue key/link metadata", "both", "identity link only"},
		{"title", "issue title", "import_export", "planning mirror"},
		{"notes", "issue description/comment", "import_export", "planning mirror"},
		{"parent_id", "epic/parent/project/module", "import_export", "planning mirror"},
		{"status", "issue state", "export_only", "planning mirror; never implicit Fairway state mutation"},
		{"priority", "priority", "import_export", "task definition metadata"},
		{"role", "label/custom field/team", "import_export", "task definition metadata"},
		{"owning_domain", "label/module/project/custom field", "import_export", "task definition metadata"},
		{"kind", "issue type/label", "import_export", "task definition metadata"},
		{"review_domains", "labels/checklist/custom field", "import_export", "planning signal; Fairway reviews remain authoritative"},
		{"acceptance_checks", "description checklist/custom field", "import_export", "task definition metadata"},
		{"evidence", "comment/link", "export_only", "Fairway evidence rows remain authoritative"},
	}, nil
}

func BuildReconcileReport(links []store.TrackerLink, dryRun bool) ReconcileReport {
	report := ReconcileReport{
		DryRun:       dryRun,
		Links:        links,
		PlanningOnly: true,
		Note:         "tracker reconcile is advisory: it reports planning/external-link drift and does not mutate Fairway execution state or remote trackers",
	}
	for _, link := range links {
		report.Actions = append(report.Actions, ProposedAction{
			Provider:   link.Provider,
			TaskID:     link.TaskID,
			ExternalID: link.ExternalID,
			Action:     "check_external_reference",
			Reason:     "linked tracker issue should be resolved by provider adapter before planning mirror changes are proposed",
			Mutates:    "none",
			DryRun:     true,
		})
	}
	return report
}

func ResolveReference(provider, externalID, rawURL string) (Reference, error) {
	if err := ValidateProvider(provider); err != nil {
		return Reference{}, err
	}
	if strings.TrimSpace(externalID) == "" && strings.TrimSpace(rawURL) == "" {
		return Reference{}, fmt.Errorf("external id or url is required")
	}
	return Reference{Provider: strings.ToLower(strings.TrimSpace(provider)), ExternalID: strings.TrimSpace(externalID), URL: strings.TrimSpace(rawURL)}, nil
}
