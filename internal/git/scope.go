package git

import (
	"path"
	"sort"
	"strings"
)

type ScopePath struct {
	Path            string `json:"path"`
	Status          string `json:"status"`
	OwnershipDomain string `json:"ownership_domain"`
	MatchedScope    string `json:"matched_scope,omitempty"`
}

type ScopeEvaluation struct {
	Mode              string      `json:"mode"`
	ChangedPaths      int         `json:"changed_paths"`
	DeclaredPaths     int         `json:"declared_paths"`
	DecisionExplained int         `json:"decision_explained_paths"`
	UnexplainedPaths  int         `json:"unexplained_paths"`
	Rows              []ScopePath `json:"rows"`
}

// ChangedScopeFiles combines current worktree changes with committed branch
// changes relative to base. It is read-only and returns normalized repo paths.
func ChangedScopeFiles(root, base string) ([]string, error) {
	status, err := Check(root, base)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, changed := range status.ChangedFiles {
		changed = normalizeScopePath(changed)
		if changed != "" {
			seen[changed] = true
		}
	}
	if strings.TrimSpace(base) != "" && status.Branch != strings.TrimSpace(base) {
		committed, err := ChangedFiles(root, base)
		if err != nil {
			return nil, err
		}
		for _, changed := range committed {
			changed = normalizeScopePath(changed)
			if changed != "" {
				seen[changed] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for changed := range seen {
		out = append(out, changed)
	}
	sort.Strings(out)
	return out, nil
}

func EvaluateScope(changed, declared, acceptedDecisionScope []string) ScopeEvaluation {
	report := ScopeEvaluation{Mode: "advisory", Rows: []ScopePath{}}
	for _, changedPath := range normalizedUniquePaths(changed) {
		row := ScopePath{Path: changedPath, Status: "unexplained", OwnershipDomain: scopeOwnershipDomain(changedPath)}
		if matched := matchingScope(changedPath, declared); matched != "" {
			row.Status = "declared"
			row.MatchedScope = matched
			report.DeclaredPaths++
		} else if matched := matchingScope(changedPath, acceptedDecisionScope); matched != "" {
			row.Status = "accepted_decision"
			row.MatchedScope = matched
			report.DecisionExplained++
		} else {
			report.UnexplainedPaths++
		}
		report.Rows = append(report.Rows, row)
	}
	report.ChangedPaths = len(report.Rows)
	return report
}

func matchingScope(changed string, scopes []string) string {
	for _, scope := range scopes {
		scope = normalizeScopePath(scope)
		if scope == "" {
			continue
		}
		if changed == scope || strings.HasPrefix(changed, strings.TrimSuffix(scope, "/")+"/") {
			return scope
		}
		if strings.HasSuffix(scope, "/**") && strings.HasPrefix(changed, strings.TrimSuffix(scope, "**")) {
			return scope
		}
		if strings.ContainsAny(scope, "*?[") {
			if matched, err := path.Match(scope, changed); err == nil && matched {
				return scope
			}
		}
	}
	return ""
}

// PathMatchesScope returns the normalized scope entry that covers repoPath.
// It is shared by deterministic read models that need the same task-path rules.
func PathMatchesScope(repoPath string, scopes []string) string {
	return matchingScope(normalizeScopePath(repoPath), scopes)
}

func normalizedUniquePaths(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		value = normalizeScopePath(value)
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeScopePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.TrimPrefix(value, "./")
	value = strings.TrimSuffix(value, "/")
	return value
}

func scopeOwnershipDomain(value string) string {
	switch {
	case strings.HasPrefix(value, "internal/dashboard/assets/"):
		return "ui"
	case strings.HasPrefix(value, "docs/"), strings.HasPrefix(value, "governance/"), value == "README.md", value == "AGENTS.md":
		return "governance"
	case strings.HasPrefix(value, ".github/"), strings.HasPrefix(value, "scripts/"), value == "Makefile", strings.HasPrefix(value, ".goreleaser"):
		return "ops"
	case strings.HasPrefix(value, "cmd/"), strings.HasPrefix(value, "internal/"), strings.HasSuffix(value, ".go"):
		return "backend"
	default:
		return "unknown"
	}
}
