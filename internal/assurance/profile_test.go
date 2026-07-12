package assurance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProfileValidYAMLAndJSON(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "profile.yaml")
	if err := os.WriteFile(yamlPath, []byte(validProfileYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := LoadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != "example-assurance" || len(profile.Controls) != 1 {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	report := Report(profile)
	if !report.Valid || report.ControlCount != 1 || strings.Join(report.EvidenceClasses, ",") != "evidence,review" {
		t.Fatalf("unexpected report: %#v", report)
	}

	jsonPath := filepath.Join(dir, "profile.json")
	jsonData := `{"schema":"fairway.assurance-profile.v1","id":"json-profile","version":"v1","title":"JSON profile","description":"Evidence support only.","framework":{"id":"example","title":"Example","version":"v1","source":"https://example.com/framework"},"applicability":{"description":"Example projects."},"scope":{"types":["project"]},"controls":[{"id":"EX-1","title":"Evidence","objective":"Retain verification evidence.","responsibility":"product","assessment_objectives":["Inspect evidence references."],"evidence":[{"class":"evidence","minimum_count":1,"accepted_results":["pass"]}]}],"prohibited_claims":["certified","compliant","authorized"],"authority":{"mode":"evidence_only","prohibited_actions":["certify","declare_compliance","accept_risk","approve","mutate_workflow","merge","deploy","release","use_credentials","change_public_exposure","run_live_operation"]}}`
	if err := os.WriteFile(jsonPath, []byte(jsonData), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(jsonPath); err != nil {
		t.Fatal(err)
	}
}

func TestLoadProfileFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		replace string
		with    string
		want    string
	}{
		{"schema", ProfileSchema, "unexpected.schema", "unsupported assurance profile schema"},
		{"duplicate control", "prohibited_claims:", "  - id: EX-1\n    title: Duplicate\n    objective: Duplicate objective.\n    responsibility: product\n    assessment_objectives: [Inspect duplicate.]\n    evidence:\n      - class: audit\n        minimum_count: 1\n        accepted_results: [verified]\nprohibited_claims:", "duplicate assurance control id"},
		{"responsibility", "responsibility: shared", "responsibility: owner", "unsupported control responsibility"},
		{"evidence class", "class: review", "class: transcript", "unsupported evidence class"},
		{"freshness", "maximum_age: 720h", "maximum_age: forever", "maximum_age must be a positive duration"},
		{"duplicate applicability", "tags: [assurance]", "tags: [assurance, assurance]", "duplicate applicability tag"},
		{"missing claim guard", "  - authorized\n", "", "required prohibited claim"},
		{"missing authority guard", "    - run_live_operation\n", "", "required prohibited action"},
		{"secret text", "Retain independently reviewed evidence.", "password=SHOULD_NOT_RENDER", "prohibited secret-like or executable content"},
		{"executable text", "Retain independently reviewed evidence.", "curl https://example.com/run", "prohibited secret-like or executable content"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "profile.yaml")
			data := strings.Replace(validProfileYAML, tc.replace, tc.with, 1)
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadFile(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want substring %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "SHOULD_NOT_RENDER") {
				t.Fatalf("error echoed private input: %v", err)
			}
		})
	}
}

func TestLoadProfileRejectsUnknownFieldsAndUnsafeFiles(t *testing.T) {
	dir := t.TempDir()
	unknown := filepath.Join(dir, "unknown.yaml")
	if err := os.WriteFile(unknown, []byte(validProfileYAML+"unknown_field: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(unknown); err == nil || !strings.Contains(err.Error(), "field unknown_field not found") {
		t.Fatalf("unexpected unknown field error: %v", err)
	}

	target := filepath.Join(dir, "target.yaml")
	if err := os.WriteFile(target, []byte(validProfileYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(link); err == nil || !strings.Contains(err.Error(), "symlinks are not allowed") {
		t.Fatalf("unexpected symlink error: %v", err)
	}
	if _, err := LoadFile("https://example.com/profile.yaml"); err == nil || !strings.Contains(err.Error(), "local file") {
		t.Fatalf("unexpected remote error: %v", err)
	}
}

func TestListDirectoryIsSortedAndFailsClosed(t *testing.T) {
	dir := t.TempDir()
	writeAssuranceFile(t, dir, "b.yaml", strings.Replace(validProfileYAML, "id: example-assurance", "id: b-profile", 1))
	writeAssuranceFile(t, dir, "a.yaml", strings.Replace(validProfileYAML, "id: example-assurance", "id: a-profile", 1))
	reports, err := ListDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[0].ProfileID != "a-profile" || reports[1].ProfileID != "b-profile" {
		t.Fatalf("reports=%+v", reports)
	}
	writeAssuranceFile(t, dir, "bad.yaml", "schema: unexpected\n")
	if _, err := ListDirectory(dir); err == nil || !strings.Contains(err.Error(), "invalid assurance profile bad.yaml") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeAssuranceFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

const validProfileYAML = `schema: fairway.assurance-profile.v1
id: example-assurance
version: v1
title: Example assurance profile
description: Organizes recorded engineering evidence for assessment support.
framework:
  id: example-framework
  title: Example Framework
  version: v1
  source: https://example.com/framework
applicability:
  description: Reversible and consequential engineering work.
  task_kinds: [task]
  risk_levels: [high]
  tags: [assurance]
scope:
  types: [project, task_set, release]
controls:
  - id: EX-1
    title: Independent verification
    objective: Retain independently reviewed evidence.
    responsibility: shared
    assessment_objectives:
      - Inspect passing evidence and independent review references.
    evidence:
      - class: evidence
        minimum_count: 1
        maximum_age: 720h
        accepted_results: [pass, partial]
      - class: review
        minimum_count: 1
        accepted_results: [approve, changes]
prohibited_claims:
  - certified
  - compliant
  - authorized
authority:
  mode: evidence_only
  prohibited_actions:
    - certify
    - declare_compliance
    - accept_risk
    - approve
    - mutate_workflow
    - merge
    - deploy
    - release
    - use_credentials
    - change_public_exposure
    - run_live_operation
`
