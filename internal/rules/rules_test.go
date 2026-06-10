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

func TestLoadConfiguredDegradesMissingAdvisorySource(t *testing.T) {
	root := t.TempDir()
	writeMinimalRulePack(t, root, "rules-one", "one.contract")
	writeMinimalRulePack(t, root, "rules-two", "two.contract")
	cfg := config.Defaults(root)
	cfg.RuleSources = []config.RuleSource{
		{Name: "one", Source: "path:rules-one", Mode: "advisory"},
		{Name: "missing", Source: "path:missing-rules", Mode: "advisory"},
		{Name: "two", Source: "path:rules-two", Mode: "advisory"},
	}

	packs, err := LoadConfigured(cfg, root, LoadOptions{Root: root})
	if err != nil {
		t.Fatalf("LoadConfigured() error = %v", err)
	}
	if len(packs) != 3 {
		t.Fatalf("packs=%d, want valid one + missing advisory + valid two", len(packs))
	}
	byName := map[string]Pack{}
	for _, pack := range packs {
		byName[pack.SourceName] = pack
	}
	if len(byName["one"].Rules) != 1 || len(byName["two"].Rules) != 1 {
		t.Fatalf("valid packs did not load: %+v", packs)
	}
	missing := byName["missing"]
	if missing.Mode != "advisory" || len(missing.Rules) != 0 || len(missing.Findings) != 1 {
		t.Fatalf("missing advisory pack=%+v, want one error finding and no rules", missing)
	}
	finding := missing.Findings[0]
	if finding.Severity != "error" || finding.Path == "" || !strings.Contains(finding.Message, `rule source "missing" mode=advisory`) {
		t.Fatalf("missing advisory finding=%+v, want source/mode/path context", finding)
	}
}

func TestLoadConfiguredFailsClosedForMissingBlockingSource(t *testing.T) {
	root := t.TempDir()
	writeMinimalRulePack(t, root, "rules-valid", "valid.contract")
	cfg := config.Defaults(root)
	cfg.RuleSources = []config.RuleSource{
		{Name: "valid", Source: "path:rules-valid", Mode: "advisory"},
		{Name: "blocking-missing", Source: "path:missing-rules", Mode: "blocking"},
	}

	_, err := LoadConfigured(cfg, root, LoadOptions{Root: root})
	if err == nil {
		t.Fatal("LoadConfigured() succeeded, want missing blocking source error")
	}
	if !strings.Contains(err.Error(), `rule source "blocking-missing" mode=blocking`) || !strings.Contains(err.Error(), "missing-rules") {
		t.Fatalf("error=%v, want source name/mode/path", err)
	}
}

func TestLoadConfiguredHandlesUnreadableExistingSourcesByMode(t *testing.T) {
	root := t.TempDir()
	writeMinimalRulePack(t, root, "rules-valid", "valid.contract")
	if err := os.MkdirAll(filepath.Join(root, "rules-unreadable", "rules", "core"), 0o755); err != nil {
		t.Fatal(err)
	}

	advisory := config.Defaults(root)
	advisory.RuleSources = []config.RuleSource{
		{Name: "valid", Source: "path:rules-valid", Mode: "advisory"},
		{Name: "unreadable", Source: "path:rules-unreadable", Mode: "advisory"},
	}
	packs, err := LoadConfigured(advisory, root, LoadOptions{Root: root})
	if err != nil {
		t.Fatalf("advisory LoadConfigured() error = %v", err)
	}
	if len(packs) != 2 {
		t.Fatalf("packs=%d, want valid plus unreadable advisory finding", len(packs))
	}
	if len(packs[1].Findings) != 1 || !strings.Contains(packs[1].Findings[0].Message, "no such file or directory") {
		t.Fatalf("unreadable advisory findings=%+v", packs[1].Findings)
	}

	blocking := config.Defaults(root)
	blocking.RuleSources = []config.RuleSource{
		{Name: "unreadable", Source: "path:rules-unreadable", Mode: "blocking"},
	}
	_, err = LoadConfigured(blocking, root, LoadOptions{Root: root})
	if err == nil {
		t.Fatal("blocking LoadConfigured() succeeded, want unreadable source error")
	}
	if !strings.Contains(err.Error(), `rule source "unreadable" mode=blocking`) || !strings.Contains(err.Error(), "rule.schema.yaml") {
		t.Fatalf("error=%v, want unreadable blocking source name/mode/schema path", err)
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

func writeMinimalRulePack(t *testing.T, root, rel, ruleID string) {
	t.Helper()
	writeRulePackFile(t, root, filepath.Join(rel, "schemas/rule.schema.yaml"), `type: object
required:
  - id
  - title
  - status
`)
	writeRulePackFile(t, root, filepath.Join(rel, "rules/core/rule.md"), `---
id: `+ruleID+`
title: Minimal rule
status: active
---

body
`)
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
