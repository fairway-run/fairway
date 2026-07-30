package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	"golang.org/x/sys/unix"
)

const (
	agentContractSchema       = 1
	agentContractRevision     = "2026-07-30.1"
	agentContractStartPrefix  = "<!-- fairway-agent-contract "
	agentContractStartSuffix  = " -->"
	agentContractEndMarker    = "<!-- /fairway-agent-contract -->"
	agentContractLocalName    = "AGENTS.local.md"
	agentContractStateMissing = "missing"
)

type agentContractMetadata struct {
	Schema        int    `json:"schema"`
	Revision      string `json:"revision"`
	GeneratedBy   string `json:"generated_by"`
	ContentSHA256 string `json:"content_sha256"`
}

type agentContractStatus struct {
	Path             string `json:"path"`
	LocalPath        string `json:"local_path"`
	State            string `json:"state"`
	Schema           int    `json:"schema,omitempty"`
	Revision         string `json:"revision,omitempty"`
	GeneratedBy      string `json:"generated_by,omitempty"`
	TargetSchema     int    `json:"target_schema"`
	TargetRevision   string `json:"target_revision"`
	BinaryVersion    string `json:"binary_version"`
	ContentSHA256    string `json:"content_sha256,omitempty"`
	TargetSHA256     string `json:"target_content_sha256"`
	ContentModified  bool   `json:"content_modified"`
	RequiresAdoption bool   `json:"requires_adoption"`
	UpdateAvailable  bool   `json:"update_available"`
	Compatible       bool   `json:"compatible"`
	Action           string `json:"action"`
	Reason           string `json:"reason"`
	ReadOnly         bool   `json:"read_only"`
	fileSHA256       string
}

type parsedAgentContract struct {
	Metadata agentContractMetadata
	Prefix   string
	Body     string
	Suffix   string
}

func cmdAgentContract(opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		subcommandUsage("agent-contract", "status|plan|apply [--adopt-legacy]")
		return nil
	}
	action := args[0]
	if action != "status" && action != "plan" && action != "apply" {
		return fmt.Errorf("unknown agent-contract action %q", action)
	}
	if isHelpOnly(args[1:]) {
		usage := ""
		if action == "apply" {
			usage = "[--adopt-legacy]"
		}
		subcommandUsage("agent-contract "+action, usage)
		return nil
	}
	fs := flag.NewFlagSet("agent-contract "+action, flag.ContinueOnError)
	adoptLegacy := fs.Bool("adopt-legacy", false, "preserve a legacy unmanaged contract as AGENTS.local.md before writing the managed contract")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected agent-contract %s arguments: %s", action, strings.Join(fs.Args(), " "))
	}
	if action != "apply" && *adoptLegacy {
		return fmt.Errorf("agent-contract %s does not accept --adopt-legacy", action)
	}
	path := agentContractPath(opts)
	status, err := inspectAgentContract(path)
	if err != nil {
		return err
	}
	if action == "apply" {
		status, err = applyAgentContract(path, *adoptLegacy)
		if err != nil {
			return err
		}
	}
	if opts.JSON {
		return printJSON(status)
	}
	printAgentContractStatus(action, status)
	return nil
}

func agentContractPath(opts globalOptions) string {
	configPath := opts.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath = config.DefaultConfigPath
	}
	return filepath.Join(filepath.Dir(configPath), "AGENTS.md")
}

func inspectAgentContract(path string) (agentContractStatus, error) {
	status := agentContractStatus{
		Path:           path,
		LocalPath:      filepath.Join(filepath.Dir(path), agentContractLocalName),
		TargetSchema:   agentContractSchema,
		TargetRevision: agentContractRevision,
		BinaryVersion:  version,
		TargetSHA256:   hashAgentContractBody(agentContractBody()),
		Compatible:     true,
		ReadOnly:       true,
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		status.State = agentContractStateMissing
		status.Action = "apply"
		status.Reason = "managed agent contract does not exist"
		return status, nil
	}
	if err != nil {
		return status, err
	}
	status.fileSHA256 = hashAgentContractBytes(data)
	parsed, err := parseAgentContract(string(data))
	if err != nil {
		if !strings.Contains(string(data), agentContractStartPrefix) {
			status.State = "legacy_unmanaged"
			status.RequiresAdoption = true
			status.Action = "apply --adopt-legacy"
			status.Reason = "existing agent contract has no Fairway management metadata"
			return status, nil
		}
		status.State = "locally_modified"
		status.ContentModified = true
		status.Action = "move local edits to AGENTS.local.md and restore the managed block"
		status.Reason = err.Error()
		return status, nil
	}
	status.Schema = parsed.Metadata.Schema
	status.Revision = parsed.Metadata.Revision
	status.GeneratedBy = parsed.Metadata.GeneratedBy
	status.ContentSHA256 = parsed.Metadata.ContentSHA256
	if hashAgentContractBody(parsed.Body) != parsed.Metadata.ContentSHA256 {
		status.State = "locally_modified"
		status.ContentModified = true
		status.Action = "move local edits to AGENTS.local.md and restore the managed block"
		status.Reason = "managed agent contract content hash does not match its metadata"
		return status, nil
	}
	if parsed.Metadata.Schema > agentContractSchema {
		status.State = "incompatible"
		status.Compatible = false
		status.Action = "upgrade the Fairway binary"
		status.Reason = "project agent contract schema is newer than this binary supports"
		return status, nil
	}
	if parsed.Metadata.Schema < agentContractSchema {
		status.State = "update_available"
		status.UpdateAvailable = true
		status.Action = "apply"
		status.Reason = "managed agent contract schema is older than the binary target"
		return status, nil
	}
	revisionOrder, validRevisions := compareAgentContractRevisions(parsed.Metadata.Revision, agentContractRevision)
	if !validRevisions {
		status.State = "incompatible"
		status.Compatible = false
		status.Action = "reconcile the agent contract revision metadata"
		status.Reason = "agent contract revision is not in the supported date.sequence format"
		return status, nil
	}
	if revisionOrder > 0 {
		status.State = "incompatible"
		status.Compatible = false
		status.Action = "upgrade the Fairway binary"
		status.Reason = "project agent contract revision is newer than this binary provides"
		return status, nil
	}
	if revisionOrder < 0 {
		status.State = "update_available"
		status.UpdateAvailable = true
		status.Action = "apply"
		status.Reason = "managed agent contract revision is older than the binary target"
		return status, nil
	}
	if hashAgentContractBody(parsed.Body) != status.TargetSHA256 {
		status.State = "locally_modified"
		status.ContentModified = true
		status.Action = "move local edits to AGENTS.local.md and restore the managed block"
		status.Reason = "managed agent contract content does not match the binary target for this revision"
		return status, nil
	}
	status.State = "current"
	status.Action = "none"
	status.Reason = "managed agent contract matches the binary target revision"
	return status, nil
}

func applyAgentContract(path string, adoptLegacy bool) (agentContractStatus, error) {
	var applied agentContractStatus
	err := withAgentContractLock(path, func() error {
		var err error
		applied, err = applyAgentContractLocked(path, adoptLegacy)
		return err
	})
	return applied, err
}

func applyAgentContractLocked(path string, adoptLegacy bool) (agentContractStatus, error) {
	status, err := inspectAgentContract(path)
	if err != nil {
		return status, err
	}
	switch status.State {
	case "current":
		return status, nil
	case agentContractStateMissing:
		if err := writeAgentContractAtomicIfUnchanged(path, initAgentContract(), "", true); err != nil {
			return status, err
		}
	case "legacy_unmanaged":
		if !adoptLegacy {
			return status, errors.New("legacy unmanaged agent contract requires --adopt-legacy")
		}
		legacy, err := os.ReadFile(path)
		if err != nil {
			return status, err
		}
		if err := verifyAgentContractSnapshot(path, status.fileSHA256, false); err != nil {
			return status, err
		}
		if err := writeAgentContractExclusive(status.LocalPath, string(legacy)); err != nil {
			return status, err
		}
		if err := writeAgentContractAtomicIfUnchanged(path, initAgentContract(), status.fileSHA256, false); err != nil {
			if cleanupErr := os.Remove(status.LocalPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				return status, fmt.Errorf("write managed agent contract: %w; remove partial local adoption: %v", err, cleanupErr)
			}
			return status, err
		}
	case "update_available":
		data, err := os.ReadFile(path)
		if err != nil {
			return status, err
		}
		parsed, err := parseAgentContract(string(data))
		if err != nil {
			return status, err
		}
		replacement := parsed.Prefix + initAgentContract() + strings.TrimPrefix(parsed.Suffix, "\n")
		if err := writeAgentContractAtomicIfUnchanged(path, replacement, status.fileSHA256, false); err != nil {
			return status, err
		}
	case "locally_modified":
		return status, errors.New("managed agent contract was locally modified; move project instructions to AGENTS.local.md before applying")
	case "incompatible":
		return status, errors.New(status.Reason)
	default:
		return status, fmt.Errorf("unsupported agent contract state %q", status.State)
	}
	applied, err := inspectAgentContract(path)
	if err != nil {
		return applied, err
	}
	applied.ReadOnly = false
	return applied, nil
}

func parseAgentContract(value string) (parsedAgentContract, error) {
	start := strings.Index(value, agentContractStartPrefix)
	if start < 0 {
		return parsedAgentContract{}, errors.New("managed agent contract start marker is missing")
	}
	metaStart := start + len(agentContractStartPrefix)
	metaEndRelative := strings.Index(value[metaStart:], agentContractStartSuffix)
	if metaEndRelative < 0 {
		return parsedAgentContract{}, errors.New("managed agent contract metadata marker is malformed")
	}
	metaEnd := metaStart + metaEndRelative
	var metadata agentContractMetadata
	if err := json.Unmarshal([]byte(value[metaStart:metaEnd]), &metadata); err != nil {
		return parsedAgentContract{}, errors.New("managed agent contract metadata is invalid")
	}
	bodyStart := metaEnd + len(agentContractStartSuffix)
	if strings.HasPrefix(value[bodyStart:], "\n") {
		bodyStart++
	}
	endRelative := strings.Index(value[bodyStart:], agentContractEndMarker)
	if endRelative < 0 {
		return parsedAgentContract{}, errors.New("managed agent contract end marker is missing")
	}
	end := bodyStart + endRelative
	body := value[bodyStart:end]
	return parsedAgentContract{
		Metadata: metadata,
		Prefix:   value[:start],
		Body:     body,
		Suffix:   value[end+len(agentContractEndMarker):],
	}, nil
}

func initAgentContract() string {
	body := agentContractBody()
	metadata := agentContractMetadata{
		Schema:        agentContractSchema,
		Revision:      agentContractRevision,
		GeneratedBy:   version,
		ContentSHA256: hashAgentContractBody(body),
	}
	raw, _ := json.Marshal(metadata)
	return agentContractStartPrefix + string(raw) + agentContractStartSuffix + "\n" +
		body + agentContractEndMarker + "\n"
}

func hashAgentContractBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func hashAgentContractBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func compareAgentContractRevisions(current, target string) (int, bool) {
	currentDate, currentSequence, currentOK := parseAgentContractRevision(current)
	targetDate, targetSequence, targetOK := parseAgentContractRevision(target)
	if !currentOK || !targetOK {
		return 0, false
	}
	if currentDate.Before(targetDate) {
		return -1, true
	}
	if currentDate.After(targetDate) {
		return 1, true
	}
	switch {
	case currentSequence < targetSequence:
		return -1, true
	case currentSequence > targetSequence:
		return 1, true
	default:
		return 0, true
	}
}

func parseAgentContractRevision(value string) (time.Time, int, bool) {
	separator := strings.LastIndex(strings.TrimSpace(value), ".")
	if separator < 0 {
		return time.Time{}, 0, false
	}
	date, err := time.Parse("2006-01-02", value[:separator])
	if err != nil {
		return time.Time{}, 0, false
	}
	sequence, err := strconv.Atoi(value[separator+1:])
	if err != nil || sequence < 0 {
		return time.Time{}, 0, false
	}
	return date, sequence, true
}

func withAgentContractLock(path string, fn func() error) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	lockRoot := filepath.Join(os.TempDir(), "fairway-agent-contract-locks")
	if err := os.MkdirAll(lockRoot, 0o700); err != nil {
		return err
	}
	lockPath := filepath.Join(lockRoot, hashAgentContractBytes([]byte(absolute))+".lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	return fn()
}

func writeAgentContractAtomicIfUnchanged(path, content, expectedSHA256 string, expectMissing bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agent-contract-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := verifyAgentContractSnapshot(path, expectedSHA256, expectMissing); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func writeAgentContractExclusive(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agent-contract-local-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; reconcile local instructions before adoption", path)
		}
		return err
	}
	return nil
}

func verifyAgentContractSnapshot(path, expectedSHA256 string, expectMissing bool) error {
	data, err := os.ReadFile(path)
	if expectMissing {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("%s was created while applying the agent contract; refusing to overwrite it", path)
	}
	if err != nil {
		return err
	}
	if hashAgentContractBytes(data) != expectedSHA256 {
		return fmt.Errorf("%s changed while applying the agent contract; retry after reconciling the new content", path)
	}
	return nil
}

func printAgentContractStatus(action string, status agentContractStatus) {
	fmt.Printf("agent_contract_%s: %s\n", action, status.State)
	fmt.Printf("path: %s\nlocal_path: %s\nschema: %d\ntarget_schema: %d\nrevision: %s\ntarget_revision: %s\ngenerated_by: %s\nbinary_version: %s\ncontent_sha256: %s\ntarget_content_sha256: %s\ncompatible: %t\ncontent_modified: %t\nupdate_available: %t\nrequires_adoption: %t\nread_only: %t\naction: %s\nreason: %s\n",
		status.Path,
		status.LocalPath,
		status.Schema,
		status.TargetSchema,
		firstNonEmpty(status.Revision, "none"),
		status.TargetRevision,
		firstNonEmpty(status.GeneratedBy, "none"),
		status.BinaryVersion,
		firstNonEmpty(status.ContentSHA256, "none"),
		status.TargetSHA256,
		status.Compatible,
		status.ContentModified,
		status.UpdateAvailable,
		status.RequiresAdoption,
		status.ReadOnly,
		status.Action,
		status.Reason,
	)
}
