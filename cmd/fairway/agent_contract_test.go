package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentContractLifecycleAndBinaryVersionIndependence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".fairway", "AGENTS.md")

	status, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "missing" || status.Action != "apply" {
		t.Fatalf("missing status=%+v", status)
	}

	applied, err := applyAgentContract(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if applied.State != "current" || applied.ReadOnly {
		t.Fatalf("applied status=%+v", applied)
	}

	oldVersion := version
	version = "99.0.0"
	t.Cleanup(func() { version = oldVersion })
	current, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != "current" || current.GeneratedBy == version {
		t.Fatalf("binary-only version change altered contract status=%+v", current)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	oldRevision := strings.Replace(string(data), `"revision":"`+agentContractRevision+`"`, `"revision":"2026-07-22.1"`, 1)
	if err := os.WriteFile(path, []byte(oldRevision), 0o644); err != nil {
		t.Fatal(err)
	}
	update, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if update.State != "update_available" || !update.UpdateAvailable {
		t.Fatalf("revision drift status=%+v", update)
	}
	if _, err := applyAgentContract(path, false); err != nil {
		t.Fatal(err)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	newerRevision := strings.Replace(string(data), `"revision":"`+agentContractRevision+`"`, `"revision":"2099-01-01.1"`, 1)
	if err := os.WriteFile(path, []byte(newerRevision), 0o644); err != nil {
		t.Fatal(err)
	}
	newer, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if newer.State != "incompatible" || newer.Compatible {
		t.Fatalf("newer revision status=%+v", newer)
	}
	if _, err := applyAgentContract(path, false); err == nil || !strings.Contains(err.Error(), newer.Reason) {
		t.Fatalf("incompatible apply error=%v reason=%q", err, newer.Reason)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	locallyModified := strings.Replace(string(data), "Execution Source Of Truth", "Locally Modified Source Of Truth", 1)
	if err := os.WriteFile(path, []byte(locallyModified), 0o644); err != nil {
		t.Fatal(err)
	}
	modified, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if modified.State != "locally_modified" || !modified.ContentModified {
		t.Fatalf("modified status=%+v", modified)
	}
	if _, err := applyAgentContract(path, false); err == nil {
		t.Fatal("locally modified managed contract was overwritten")
	}

	if err := os.WriteFile(path, []byte(initAgentContract()), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	incompatibleBody := strings.Replace(string(data), `"schema":1`, `"schema":99`, 1)
	if err := os.WriteFile(path, []byte(incompatibleBody), 0o644); err != nil {
		t.Fatal(err)
	}
	incompatible, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if incompatible.State != "incompatible" || incompatible.Compatible {
		t.Fatalf("incompatible status=%+v", incompatible)
	}
}

func TestAgentContractEqualRevisionRequiresTargetContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".fairway", "AGENTS.md")
	parsed, err := parseAgentContract(initAgentContract())
	if err != nil {
		t.Fatal(err)
	}
	parsed.Body = strings.Replace(parsed.Body, "Execution Source Of Truth", "Alternate Source Of Truth", 1)
	parsed.Metadata.ContentSHA256 = hashAgentContractBody(parsed.Body)
	rawMetadata, err := json.Marshal(parsed.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	content := agentContractStartPrefix + string(rawMetadata) + agentContractStartSuffix + "\n" +
		parsed.Body + agentContractEndMarker + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "locally_modified" || !status.ContentModified {
		t.Fatalf("status=%+v", status)
	}
}

func TestAgentContractLegacyAdoptionPreservesLocalInstructions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".fairway", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "# Project Agent Rules\n\nKeep this local policy.\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "legacy_unmanaged" || !status.RequiresAdoption {
		t.Fatalf("legacy status=%+v", status)
	}
	if _, err := applyAgentContract(path, false); err == nil {
		t.Fatal("legacy contract applied without explicit adoption")
	}
	applied, err := applyAgentContract(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.State != "current" {
		t.Fatalf("applied status=%+v", applied)
	}
	local, err := os.ReadFile(filepath.Join(filepath.Dir(path), agentContractLocalName))
	if err != nil {
		t.Fatal(err)
	}
	if string(local) != legacy {
		t.Fatalf("local instructions=%q", local)
	}
}

func TestAgentContractUpdatePreservesSurroundingContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".fairway", "AGENTS.md")
	prefix := "# Project preface\n\n"
	suffix := "\n# Project appendix\n"
	managed := strings.Replace(initAgentContract(), `"revision":"`+agentContractRevision+`"`, `"revision":"2026-07-22.1"`, 1)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(prefix+managed+suffix), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := inspectAgentContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "update_available" {
		t.Fatalf("status=%+v", status)
	}
	if _, err := applyAgentContract(path, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), prefix) || !strings.HasSuffix(string(data), suffix) {
		t.Fatalf("surrounding project content was not preserved:\n%s", data)
	}
}

func TestAgentContractConditionalAndExclusiveWritesRefuseConcurrentContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalSHA := hashAgentContractBytes([]byte("original"))
	if err := os.WriteFile(path, []byte("concurrent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAgentContractAtomicIfUnchanged(path, "replacement", originalSHA, false); err == nil {
		t.Fatal("conditional write overwrote concurrent content")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "concurrent" {
		t.Fatalf("content=%q", data)
	}

	localPath := filepath.Join(root, agentContractLocalName)
	if err := writeAgentContractExclusive(localPath, "first"); err != nil {
		t.Fatal(err)
	}
	if err := writeAgentContractExclusive(localPath, "second"); err == nil {
		t.Fatal("exclusive write overwrote existing local contract")
	}
	local, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(local) != "first" {
		t.Fatalf("local content=%q", local)
	}
}
