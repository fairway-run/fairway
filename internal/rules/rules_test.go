package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

func TestLoadDirValidatesRulesGroupsAndFindings(t *testing.T) {
	root := t.TempDir()
	writeRulePackFile(t, root, "schemas/rule.schema.yaml", `type: object
required:
  - id
  - title
  - status
`)
	writeRulePackFile(t, root, "rules/core/contract.md", `---
id: platform.contract-first
title: Contract first
status: draft
applies_when:
  source_paths:
    - doc/api/**
risk_floor: medium
required_evidence:
  - generated-artifacts-clean
review_domains:
  - backend
---

body
`)
	writeRulePackFile(t, root, "rules/security/bad.md", `---
id: platform.bad
title: Bad
version: 0.1.0
status: active
applies_when:
  risk_floor: high
review_domains:
  - security
---

body
`)

	pack, err := LoadDir(root, "platform", "blocking", LoadOptions{
		KnownDomains:  map[string]bool{"backend": true},
		KnownEvidence: map[string]bool{"generated-artifacts-clean": true},
	})
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	if len(pack.Rules) != 2 {
		t.Fatalf("rules=%d, want 2", len(pack.Rules))
	}
	if got := strings.Join(pack.Groups, ","); got != "platform.core,platform.security" {
		t.Fatalf("groups=%q", got)
	}
	var messages []string
	for _, finding := range pack.Findings {
		messages = append(messages, finding.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{
		"per-rule version is not supported",
		"risk_floor must be top-level",
		`review domain "security" is not configured`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("findings=%q, want %q", joined, want)
		}
	}
	if !HasErrors(pack.Findings) {
		t.Fatal("expected error findings")
	}
}

func writeRulePackFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMatchTaskUsesAxesRiskAndProfileBinding(t *testing.T) {
	cfg := config.Defaults(t.TempDir())
	cfg.WorkstreamProfiles = []config.WorkstreamProfile{
		{Name: "platform-foundation", RuleGroups: []string{"platform.core"}},
	}
	packs := []Pack{{
		SourceName: "platform",
		Mode:       "advisory",
		Rules: []Rule{
			{
				ID:        "platform.contract-first",
				Group:     "platform.core",
				RiskFloor: "medium",
				AppliesWhen: AppliesWhen{
					SourcePaths: []string{"doc/api/**"},
					Tags:        []string{"surface:api"},
					TaskKinds:   []string{"task"},
					Profiles:    []string{"platform-foundation"},
				},
			},
			{
				ID:        "platform.security",
				Group:     "platform.security",
				RiskFloor: "high",
				AppliesWhen: AppliesWhen{
					Tags: []string{"surface:api"},
				},
			},
			{
				ID:    "platform.disabled",
				Group: "platform.core",
				Mode:  "disabled",
			},
		},
	}}
	task := store.Task{Definition: store.TaskDefinition{
		ID:          "FW-001",
		Kind:        "task",
		Profile:     "platform-foundation",
		SourcePaths: []string{"doc/api/openapi.yaml"},
		Tags:        []string{"surface:api"},
		RiskLevel:   "medium",
	}}
	matches := MatchTask(cfg, packs, task)
	byID := map[string]Match{}
	for _, match := range matches {
		byID[match.Rule.ID] = match
	}
	if got := byID["platform.contract-first"].Status; got != "selected" {
		t.Fatalf("contract status=%q", got)
	}
	if got := byID["platform.security"].Status; got != "non_applicable" {
		t.Fatalf("security status=%q", got)
	}
	if !strings.Contains(strings.Join(byID["platform.security"].Reasons, ";"), "not bound") {
		t.Fatalf("security reasons=%v", byID["platform.security"].Reasons)
	}
	if got := byID["platform.disabled"].Status; got != "disabled" {
		t.Fatalf("disabled status=%q", got)
	}
}
