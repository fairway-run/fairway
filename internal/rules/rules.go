package rules

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/config"
	"gopkg.in/yaml.v3"
)

type Rule struct {
	ID                  string
	Title               string
	Status              string
	AppliesWhen         AppliesWhen
	RiskFloor           string
	RequiredEvidence    []string
	RecommendedCommands []string
	ReviewDomains       []string
	StopConditions      []string
	RelatedRules        []string
	FilePath            string
	SourceName          string
	Group               string
	Mode                string
}

type AppliesWhen struct {
	SourcePaths []string `yaml:"source_paths"`
	TargetPaths []string `yaml:"target_paths"`
	Tags        []string `yaml:"tags"`
	TaskKinds   []string `yaml:"task_kinds"`
	Profiles    []string `yaml:"profiles"`
}

type Pack struct {
	Root       string
	SourceName string
	Mode       string
	Rules      []Rule
	Groups     []string
	Findings   []Finding
}

type Finding struct {
	Severity string
	Path     string
	Message  string
}

type LoadOptions struct {
	Root          string
	KnownDomains  map[string]bool
	KnownEvidence map[string]bool
}

type ruleFrontMatter struct {
	ID                  string      `yaml:"id"`
	Title               string      `yaml:"title"`
	Version             interface{} `yaml:"version"`
	Status              string      `yaml:"status"`
	AppliesWhen         AppliesWhen `yaml:"applies_when"`
	RiskFloor           string      `yaml:"risk_floor"`
	RequiredEvidence    []string    `yaml:"required_evidence"`
	RecommendedCommands []string    `yaml:"recommended_commands"`
	ReviewDomains       []string    `yaml:"review_domains"`
	StopConditions      []string    `yaml:"stop_conditions"`
	RelatedRules        []string    `yaml:"related_rules"`
}

type schemaShape struct {
	Required []string `yaml:"required"`
}

func LoadConfigured(cfg config.Config, root string, opts LoadOptions) ([]Pack, error) {
	if opts.Root == "" {
		opts.Root = root
	}
	var packs []Pack
	for _, source := range cfg.RuleSources {
		mode := ruleSourceMode(source)
		if mode == "disabled" {
			continue
		}
		dir, err := ResolveSourcePath(source.Source, root)
		if err != nil {
			return nil, fmt.Errorf("rule source %q: %w", source.Name, err)
		}
		pack, err := LoadDir(dir, source.Name, mode, opts)
		if err != nil {
			return nil, err
		}
		packs = append(packs, pack)
	}
	return packs, nil
}

func ResolveSourcePath(source, root string) (string, error) {
	scheme, value, ok := strings.Cut(source, ":")
	if !ok || value == "" {
		return "", fmt.Errorf("source must use scheme:path form")
	}
	switch scheme {
	case "path", "file":
		if filepath.IsAbs(value) {
			return filepath.Clean(value), nil
		}
		return filepath.Clean(filepath.Join(root, value)), nil
	default:
		return "", fmt.Errorf("unsupported enabled source scheme %q", scheme)
	}
}

func LoadDir(root, sourceName, mode string, opts LoadOptions) (Pack, error) {
	if sourceName == "" {
		sourceName = filepath.Base(root)
	}
	if mode == "" {
		mode = "advisory"
	}
	pack := Pack{Root: root, SourceName: sourceName, Mode: mode}
	required, err := loadRequiredFields(filepath.Join(root, "schemas", "rule.schema.yaml"))
	if err != nil {
		pack.Findings = append(pack.Findings, Finding{Severity: "error", Path: filepath.Join(root, "schemas", "rule.schema.yaml"), Message: err.Error()})
		return pack, nil
	}
	seen := map[string]bool{}
	groupSet := map[string]bool{}
	rulesRoot := filepath.Join(root, "rules")
	if err := filepath.WalkDir(rulesRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			pack.Findings = append(pack.Findings, Finding{Severity: "error", Path: path, Message: walkErr.Error()})
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rule, findings := parseRuleFile(path, rulesRoot, sourceName, mode, required, opts)
		pack.Findings = append(pack.Findings, findings...)
		if rule.ID == "" {
			return nil
		}
		if seen[rule.ID] {
			pack.Findings = append(pack.Findings, Finding{Severity: "error", Path: path, Message: fmt.Sprintf("duplicate rule id %q", rule.ID)})
			return nil
		}
		seen[rule.ID] = true
		groupSet[rule.Group] = true
		pack.Rules = append(pack.Rules, rule)
		return nil
	}); err != nil {
		return pack, err
	}
	for group := range groupSet {
		pack.Groups = append(pack.Groups, group)
	}
	sort.Strings(pack.Groups)
	sort.Slice(pack.Rules, func(i, j int) bool { return pack.Rules[i].ID < pack.Rules[j].ID })
	return pack, nil
}

func loadRequiredFields(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var schema schemaShape
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	required := map[string]bool{}
	for _, field := range schema.Required {
		required[field] = true
	}
	return required, nil
}

func parseRuleFile(path, rulesRoot, sourceName, mode string, required map[string]bool, opts LoadOptions) (Rule, []Finding) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Rule{}, []Finding{{Severity: "error", Path: path, Message: err.Error()}}
	}
	frontmatter, err := extractFrontMatter(string(data))
	if err != nil {
		return Rule{}, []Finding{{Severity: "error", Path: path, Message: err.Error()}}
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatter), &raw); err != nil {
		return Rule{}, []Finding{{Severity: "error", Path: path, Message: err.Error()}}
	}
	var meta ruleFrontMatter
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return Rule{}, []Finding{{Severity: "error", Path: path, Message: err.Error()}}
	}
	var findings []Finding
	for field := range required {
		if isEmptyField(raw[field]) {
			findings = append(findings, Finding{Severity: "error", Path: path, Message: fmt.Sprintf("missing required field %q", field)})
		}
	}
	if meta.Version != nil {
		findings = append(findings, Finding{Severity: "error", Path: path, Message: "per-rule version is not supported"})
	}
	if applies, ok := raw["applies_when"].(map[string]interface{}); ok {
		if _, ok := applies["risk_floor"]; ok {
			findings = append(findings, Finding{Severity: "error", Path: path, Message: "risk_floor must be top-level, not under applies_when"})
		}
	}
	switch meta.Status {
	case "draft", "active", "deprecated":
	default:
		findings = append(findings, Finding{Severity: "error", Path: path, Message: fmt.Sprintf("status %q must be draft, active, or deprecated", meta.Status)})
	}
	if meta.RiskFloor != "" {
		switch meta.RiskFloor {
		case "low", "medium", "high", "critical":
		default:
			findings = append(findings, Finding{Severity: "error", Path: path, Message: fmt.Sprintf("risk_floor %q must be low, medium, high, or critical", meta.RiskFloor)})
		}
	}
	for _, domain := range meta.ReviewDomains {
		if opts.KnownDomains != nil && !opts.KnownDomains[domain] {
			findings = append(findings, Finding{Severity: "warning", Path: path, Message: fmt.Sprintf("review domain %q is not configured for this project", domain)})
		}
	}
	if mode == "blocking" && opts.KnownEvidence != nil {
		for _, evidence := range meta.RequiredEvidence {
			if !opts.KnownEvidence[evidence] {
				findings = append(findings, Finding{Severity: "warning", Path: path, Message: fmt.Sprintf("required evidence %q has no known configured gate or recorded evidence type", evidence)})
			}
		}
	}
	group := groupForRule(path, rulesRoot, sourceName)
	return Rule{
		ID:                  meta.ID,
		Title:               meta.Title,
		Status:              meta.Status,
		AppliesWhen:         meta.AppliesWhen,
		RiskFloor:           meta.RiskFloor,
		RequiredEvidence:    meta.RequiredEvidence,
		RecommendedCommands: meta.RecommendedCommands,
		ReviewDomains:       meta.ReviewDomains,
		StopConditions:      meta.StopConditions,
		RelatedRules:        meta.RelatedRules,
		FilePath:            path,
		SourceName:          sourceName,
		Group:               group,
		Mode:                mode,
	}, findings
}

func extractFrontMatter(text string) (string, error) {
	if !strings.HasPrefix(text, "---\n") {
		return "", fmt.Errorf("missing YAML front matter")
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", fmt.Errorf("malformed YAML front matter")
	}
	return rest[:end], nil
}

func isEmptyField(value interface{}) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func groupForRule(path, rulesRoot, sourceName string) string {
	rel, err := filepath.Rel(rulesRoot, path)
	if err != nil {
		return sourceName
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 {
		return sourceName
	}
	return sourceName + "." + parts[0]
}

func ruleSourceMode(source config.RuleSource) string {
	if strings.TrimSpace(source.Mode) == "" {
		return "advisory"
	}
	return strings.TrimSpace(source.Mode)
}

func ReviewDomainSet(cfg config.Config) map[string]bool {
	out := map[string]bool{}
	for _, profile := range cfg.WorkstreamProfiles {
		for _, domain := range profile.ReviewDomains {
			out[domain] = true
		}
	}
	return out
}

func ConfigGateEvidenceSet(cfg config.Config) map[string]bool {
	out := map[string]bool{}
	for _, profile := range cfg.WorkstreamProfiles {
		for _, gate := range profile.Gates {
			if gate.EvidenceType != "" {
				out[gate.EvidenceType] = true
			}
		}
	}
	return out
}

func EvidenceTypes(packs []Pack) map[string][]string {
	out := map[string][]string{}
	for _, pack := range packs {
		for _, rule := range pack.Rules {
			for _, evidence := range rule.RequiredEvidence {
				out[evidence] = append(out[evidence], rule.ID)
			}
		}
	}
	for evidence := range out {
		sort.Strings(out[evidence])
	}
	return out
}

func HasErrors(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return true
		}
	}
	return false
}
