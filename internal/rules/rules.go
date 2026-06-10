package rules

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
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
	SourcePaths   []string `yaml:"source_paths"`
	TargetPaths   []string `yaml:"target_paths"`
	Tags          []string `yaml:"tags"`
	TaskKinds     []string `yaml:"task_kinds"`
	Profiles      []string `yaml:"profiles"`
	ReviewDomains []string `yaml:"review_domains"`
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
	Root            string
	KnownDomains    map[string]bool
	KnownEvidence   map[string]bool
	IncludeDisabled bool
}

type Match struct {
	Rule    Rule
	Status  string
	Reasons []string
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
		if mode == "disabled" && !opts.IncludeDisabled {
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

func MatchTask(cfg config.Config, packs []Pack, task store.Task) []Match {
	groupSet := boundRuleGroups(cfg, task.Definition.Profile)
	var matches []Match
	for _, pack := range packs {
		for _, rule := range pack.Rules {
			status, reasons := matchRule(groupSet, rule, task.Definition)
			if rule.Mode == "disabled" {
				status = "disabled"
				if len(reasons) == 0 {
					reasons = append(reasons, "rule source is disabled")
				}
			}
			matches = append(matches, Match{Rule: rule, Status: status, Reasons: reasons})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		ri, rj := riskRank(matches[i].Rule.RiskFloor), riskRank(matches[j].Rule.RiskFloor)
		if ri != rj {
			return ri > rj
		}
		return matches[i].Rule.ID < matches[j].Rule.ID
	})
	return matches
}

func matchRule(boundGroups map[string]bool, rule Rule, task store.TaskDefinition) (string, []string) {
	var reasons []string
	if len(boundGroups) > 0 && !boundGroups[rule.Group] {
		reasons = append(reasons, fmt.Sprintf("group %s is not bound to profile %s", rule.Group, task.Profile))
	}
	if len(rule.AppliesWhen.SourcePaths) > 0 && !anyGlobMatch(rule.AppliesWhen.SourcePaths, task.SourcePaths) {
		reasons = append(reasons, "source paths do not match")
	}
	if len(rule.AppliesWhen.TargetPaths) > 0 && !anyGlobMatch(rule.AppliesWhen.TargetPaths, task.TargetPaths) {
		reasons = append(reasons, "target paths do not match")
	}
	if len(rule.AppliesWhen.Tags) > 0 && !anyOverlap(rule.AppliesWhen.Tags, task.Tags) {
		reasons = append(reasons, "tags do not match")
	}
	if len(rule.AppliesWhen.TaskKinds) > 0 && !contains(rule.AppliesWhen.TaskKinds, task.Kind) {
		reasons = append(reasons, "task kind does not match")
	}
	if len(rule.AppliesWhen.Profiles) > 0 && !contains(rule.AppliesWhen.Profiles, task.Profile) {
		reasons = append(reasons, "profile filter does not match")
	}
	if len(rule.AppliesWhen.ReviewDomains) > 0 && !anyOverlap(rule.AppliesWhen.ReviewDomains, task.ReviewDomains) {
		reasons = append(reasons, "review domains do not match")
	}
	if rule.RiskFloor != "" && riskRank(task.RiskLevel) < riskRank(rule.RiskFloor) {
		reasons = append(reasons, fmt.Sprintf("task risk %q is below floor %q", firstNonEmpty(task.RiskLevel, "low"), rule.RiskFloor))
	}
	if len(reasons) > 0 {
		return "non_applicable", reasons
	}
	return "selected", nil
}

func boundRuleGroups(cfg config.Config, profileName string) map[string]bool {
	if profileName == "" {
		return nil
	}
	for _, profile := range cfg.WorkstreamProfiles {
		if profile.Name != profileName || len(profile.RuleGroups) == 0 {
			continue
		}
		out := map[string]bool{}
		for _, group := range profile.RuleGroups {
			out[group] = true
		}
		return out
	}
	return nil
}

func anyGlobMatch(patterns, values []string) bool {
	for _, pattern := range patterns {
		for _, value := range values {
			if globMatch(pattern, value) {
				return true
			}
		}
	}
	return false
}

func globMatch(pattern, value string) bool {
	pattern = filepath.ToSlash(pattern)
	value = filepath.ToSlash(value)
	if strings.HasSuffix(pattern, "/**") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "/**")+"/") || value == strings.TrimSuffix(pattern, "/**")
	}
	ok, err := path.Match(pattern, value)
	return err == nil && ok
}

func anyOverlap(a, b []string) bool {
	set := map[string]bool{}
	for _, value := range b {
		set[value] = true
	}
	for _, value := range a {
		if set[value] {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func riskRank(risk string) int {
	switch risk {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low", "":
		return 1
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
