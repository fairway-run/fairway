package deploymentbaseline

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	BaselineSchema    = "fairway.sovereign-deployment-baseline.v1"
	ObservationSchema = "fairway.sovereign-deployment-observation.v1"
	ReportSchema      = "fairway.sovereign-deployment-baseline-report.v1"
	maxDocumentSize   = 1 << 20
)

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

var requiredControlIDs = []string{
	"artifact-source", "backup-restore", "configuration-signing", "customer-secret-store",
	"data-paths", "disaster-recovery", "drift-detection", "firewall", "key-recovery",
	"least-privilege", "local-assets", "mandatory-access-control", "network-allowlist",
	"read-only-filesystem", "resource-budget", "rootless-runtime", "service-identity",
	"status-version-logs", "upgrade-rollback",
}

var requiredProhibitedActions = []string{
	"accept_risk", "approve", "certify", "change_public_exposure", "declare_compliance", "deploy",
	"merge", "mutate_system", "release", "run_live_operation", "send_provider_message", "use_credentials",
}

type Baseline struct {
	Schema      string    `json:"schema" yaml:"schema"`
	ID          string    `json:"id" yaml:"id"`
	Version     string    `json:"version" yaml:"version"`
	Title       string    `json:"title" yaml:"title"`
	Description string    `json:"description" yaml:"description"`
	Topology    string    `json:"topology" yaml:"topology"`
	Controls    []Control `json:"controls" yaml:"controls"`
	Authority   Authority `json:"authority" yaml:"authority"`
}

type Control struct {
	ID                 string   `json:"id" yaml:"id"`
	Title              string   `json:"title" yaml:"title"`
	Category           string   `json:"category" yaml:"category"`
	Severity           string   `json:"severity" yaml:"severity"`
	Expectation        string   `json:"expectation" yaml:"expectation"`
	EvidenceRequired   bool     `json:"evidence_required" yaml:"evidence_required"`
	AllowNotApplicable bool     `json:"allow_not_applicable,omitempty" yaml:"allow_not_applicable,omitempty"`
	NextAction         string   `json:"next_action" yaml:"next_action"`
	Tags               []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type Authority struct {
	Mode              string   `json:"mode" yaml:"mode"`
	ProhibitedActions []string `json:"prohibited_actions" yaml:"prohibited_actions"`
}

type Observation struct {
	Schema          string               `json:"schema" yaml:"schema"`
	BaselineID      string               `json:"baseline_id" yaml:"baseline_id"`
	BaselineVersion string               `json:"baseline_version" yaml:"baseline_version"`
	Topology        string               `json:"topology" yaml:"topology"`
	DeploymentID    string               `json:"deployment_id" yaml:"deployment_id"`
	ObservedAt      string               `json:"observed_at" yaml:"observed_at"`
	Results         []ObservationResult  `json:"results" yaml:"results"`
	Authority       ObservationAuthority `json:"authority" yaml:"authority"`
}

type ObservationResult struct {
	ControlID    string   `json:"control_id" yaml:"control_id"`
	Status       string   `json:"status" yaml:"status"`
	EvidenceRefs []string `json:"evidence_refs,omitempty" yaml:"evidence_refs,omitempty"`
	Notes        string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ObservationAuthority struct {
	Mode string `json:"mode" yaml:"mode"`
}

type Report struct {
	Schema            string      `json:"schema"`
	Ready             bool        `json:"ready"`
	BaselineID        string      `json:"baseline_id"`
	BaselineVersion   string      `json:"baseline_version"`
	Topology          string      `json:"topology"`
	DeploymentID      string      `json:"deployment_id"`
	ObservedAt        string      `json:"observed_at"`
	ControlCount      int         `json:"control_count"`
	PassingCount      int         `json:"passing_count"`
	BlockingCount     int         `json:"blocking_deviation_count"`
	AdvisoryCount     int         `json:"advisory_deviation_count"`
	Deviations        []Deviation `json:"deviations,omitempty"`
	AuthorityBoundary string      `json:"authority_boundary"`
}

type Deviation struct {
	ControlID     string   `json:"control_id"`
	Title         string   `json:"title"`
	Category      string   `json:"category"`
	Severity      string   `json:"severity"`
	Status        string   `json:"status"`
	Expectation   string   `json:"expectation"`
	EvidenceRefs  []string `json:"evidence_refs,omitempty"`
	Notes         string   `json:"notes,omitempty"`
	SuggestedNext string   `json:"suggested_next_action"`
}

func LoadBaseline(path string) (Baseline, error) {
	var baseline Baseline
	if err := loadYAML(path, "deployment baseline", &baseline); err != nil {
		return Baseline{}, err
	}
	if err := ValidateBaseline(baseline); err != nil {
		return Baseline{}, err
	}
	return baseline, nil
}

func LoadObservation(path string) (Observation, error) {
	var observation Observation
	if err := loadYAML(path, "deployment observation", &observation); err != nil {
		return Observation{}, err
	}
	if err := ValidateObservation(observation); err != nil {
		return Observation{}, err
	}
	return observation, nil
}

func loadYAML(path, kind string, target any) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("%s path is required", kind)
	}
	if strings.Contains(path, "://") {
		return fmt.Errorf("%s must be a local YAML file", kind)
	}
	clean := filepath.Clean(path)
	info, err := os.Lstat(clean)
	if err != nil {
		return fmt.Errorf("read %s: %w", kind, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a non-symlink regular file", kind)
	}
	if info.Size() > maxDocumentSize {
		return fmt.Errorf("%s exceeds %d bytes", kind, maxDocumentSize)
	}
	if ext := strings.ToLower(filepath.Ext(clean)); ext != ".yaml" && ext != ".yml" {
		return fmt.Errorf("%s must use .yaml or .yml", kind)
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return fmt.Errorf("read %s: %w", kind, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", kind, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s must contain one YAML document", kind)
		}
		return fmt.Errorf("decode %s: %w", kind, err)
	}
	return nil
}

func ValidateBaseline(b Baseline) error {
	if b.Schema != BaselineSchema {
		return fmt.Errorf("unsupported deployment baseline schema %q", b.Schema)
	}
	if err := validIdentifier("baseline id", b.ID); err != nil {
		return err
	}
	if err := validIdentifier("baseline version", b.Version); err != nil {
		return err
	}
	if err := validText("baseline title", b.Title, true); err != nil {
		return err
	}
	if err := validText("baseline description", b.Description, true); err != nil {
		return err
	}
	if !oneOf(b.Topology, "single_host", "managed_service", "container_orchestration") {
		return fmt.Errorf("unsupported deployment topology %q", b.Topology)
	}
	if b.Authority.Mode != "observe_only" {
		return errors.New("deployment baseline authority.mode must be observe_only")
	}
	if err := requireSet("prohibited action", b.Authority.ProhibitedActions, requiredProhibitedActions); err != nil {
		return err
	}
	seen := map[string]Control{}
	for i, control := range b.Controls {
		if err := validateControl(control); err != nil {
			return fmt.Errorf("control %d: %w", i+1, err)
		}
		if _, ok := seen[control.ID]; ok {
			return fmt.Errorf("duplicate deployment control id %q", control.ID)
		}
		seen[control.ID] = control
	}
	for _, required := range requiredControlIDs {
		control, ok := seen[required]
		if !ok {
			return fmt.Errorf("required deployment control %q is missing", required)
		}
		if control.Severity != "blocking" {
			return fmt.Errorf("required deployment control %q must be blocking", required)
		}
		if !control.EvidenceRequired {
			return fmt.Errorf("required deployment control %q must require evidence", required)
		}
	}
	return nil
}

func validateControl(c Control) error {
	if err := validIdentifier("control id", c.ID); err != nil {
		return err
	}
	if err := validText("control title", c.Title, true); err != nil {
		return err
	}
	if !oneOf(c.Category, "artifact", "configuration", "identity", "network", "runtime", "resilience", "observability") {
		return fmt.Errorf("unsupported control category %q", c.Category)
	}
	if !oneOf(c.Severity, "blocking", "advisory") {
		return fmt.Errorf("unsupported control severity %q", c.Severity)
	}
	if err := validText("control expectation", c.Expectation, true); err != nil {
		return err
	}
	if err := validText("control next_action", c.NextAction, true); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, tag := range c.Tags {
		if err := validIdentifier("control tag", tag); err != nil {
			return err
		}
		if seen[tag] {
			return fmt.Errorf("duplicate control tag %q", tag)
		}
		seen[tag] = true
	}
	return nil
}

func ValidateObservation(o Observation) error {
	if o.Schema != ObservationSchema {
		return fmt.Errorf("unsupported deployment observation schema %q", o.Schema)
	}
	for name, value := range map[string]string{
		"baseline id": o.BaselineID, "baseline version": o.BaselineVersion, "deployment id": o.DeploymentID,
	} {
		if err := validIdentifier(name, value); err != nil {
			return err
		}
	}
	if !oneOf(o.Topology, "single_host", "managed_service", "container_orchestration") {
		return fmt.Errorf("unsupported deployment observation topology %q", o.Topology)
	}
	observedAt, err := time.Parse(time.RFC3339, o.ObservedAt)
	if err != nil {
		return errors.New("deployment observation observed_at must be RFC3339")
	}
	if observedAt.After(time.Now().UTC().Add(5 * time.Minute)) {
		return errors.New("deployment observation observed_at is in the future")
	}
	if o.Authority.Mode != "observation_only" {
		return errors.New("deployment observation authority.mode must be observation_only")
	}
	seen := map[string]bool{}
	for i, result := range o.Results {
		if err := validIdentifier("observation control id", result.ControlID); err != nil {
			return fmt.Errorf("result %d: %w", i+1, err)
		}
		if seen[result.ControlID] {
			return fmt.Errorf("duplicate observation control id %q", result.ControlID)
		}
		seen[result.ControlID] = true
		if !oneOf(result.Status, "pass", "fail", "not_observed", "not_applicable") {
			return fmt.Errorf("result %s has unsupported status %q", result.ControlID, result.Status)
		}
		if result.Notes != "" {
			if err := validText("observation notes", result.Notes, false); err != nil {
				return fmt.Errorf("result %s: %w", result.ControlID, err)
			}
		}
		for _, ref := range result.EvidenceRefs {
			if err := validReference(ref); err != nil {
				return fmt.Errorf("result %s: %w", result.ControlID, err)
			}
		}
	}
	return nil
}

func Evaluate(b Baseline, o Observation) (Report, error) {
	if err := ValidateBaseline(b); err != nil {
		return Report{}, err
	}
	if err := ValidateObservation(o); err != nil {
		return Report{}, err
	}
	if o.BaselineID != b.ID || o.BaselineVersion != b.Version || o.Topology != b.Topology {
		return Report{}, errors.New("deployment observation does not match baseline id, version, and topology")
	}
	results := make(map[string]ObservationResult, len(o.Results))
	for _, result := range o.Results {
		results[result.ControlID] = result
	}
	report := Report{
		Schema: ReportSchema, Ready: true, BaselineID: b.ID, BaselineVersion: b.Version,
		Topology: b.Topology, DeploymentID: o.DeploymentID, ObservedAt: o.ObservedAt,
		ControlCount:      len(b.Controls),
		AuthorityBoundary: "read-only deviation report; not certification, compliance, approval, risk acceptance, deployment authorization, or system mutation",
	}
	known := map[string]bool{}
	for _, control := range b.Controls {
		known[control.ID] = true
		result, ok := results[control.ID]
		status := result.Status
		if !ok {
			status = "not_observed"
		}
		passing := status == "pass" && (!control.EvidenceRequired || len(result.EvidenceRefs) > 0)
		if status == "not_applicable" && control.AllowNotApplicable && len(result.EvidenceRefs) > 0 && strings.TrimSpace(result.Notes) != "" {
			passing = true
		}
		if passing {
			report.PassingCount++
			continue
		}
		deviation := Deviation{
			ControlID: control.ID, Title: control.Title, Category: control.Category, Severity: control.Severity,
			Status: status, Expectation: control.Expectation, EvidenceRefs: append([]string(nil), result.EvidenceRefs...),
			Notes: result.Notes, SuggestedNext: control.NextAction,
		}
		if status == "pass" && control.EvidenceRequired && len(result.EvidenceRefs) == 0 {
			deviation.Status = "missing_evidence"
		}
		if status == "not_applicable" && !control.AllowNotApplicable {
			deviation.Status = "not_applicable_rejected"
		}
		report.Deviations = append(report.Deviations, deviation)
		if control.Severity == "blocking" {
			report.BlockingCount++
			report.Ready = false
		} else {
			report.AdvisoryCount++
		}
	}
	for controlID := range results {
		if !known[controlID] {
			return Report{}, fmt.Errorf("deployment observation contains unknown control %q", controlID)
		}
	}
	sort.Slice(report.Deviations, func(i, j int) bool { return report.Deviations[i].ControlID < report.Deviations[j].ControlID })
	return report, nil
}

func validIdentifier(name, value string) error {
	if !identifierPattern.MatchString(strings.TrimSpace(value)) {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

func validText(name, value string, required bool) error {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len(value) > 2048 || strings.ContainsAny(value, "\x00\r") || containsPrivateOrExecutable(value) {
		return fmt.Errorf("%s contains prohibited private or executable content", name)
	}
	return nil
}

func validReference(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 1024 || strings.ContainsAny(value, "\x00\r\n") || containsPrivateOrExecutable(value) {
		return errors.New("evidence reference is invalid or contains prohibited private content")
	}
	return nil
}

func containsPrivateOrExecutable(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"authorization: bearer", "api_key=", "apikey=", "client_secret=", "password=", "private_key=",
		"raw_prompt", "raw transcript", "tool_body", "generated_content", "curl ", "wget ", "kubectl apply", "terraform apply",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func requireSet(kind string, values, required []string) error {
	seen := map[string]bool{}
	for _, value := range values {
		if err := validIdentifier(kind, value); err != nil {
			return err
		}
		if seen[value] {
			return fmt.Errorf("duplicate %s %q", kind, value)
		}
		seen[value] = true
	}
	for _, value := range required {
		if !seen[value] {
			return fmt.Errorf("required %s %q is missing", kind, value)
		}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
