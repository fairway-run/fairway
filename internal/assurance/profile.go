package assurance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ProfileSchema  = "fairway.assurance-profile.v1"
	maxProfileSize = 1 << 20
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	controlIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type Profile struct {
	Schema           string        `json:"schema" yaml:"schema"`
	ID               string        `json:"id" yaml:"id"`
	Version          string        `json:"version" yaml:"version"`
	Title            string        `json:"title" yaml:"title"`
	Description      string        `json:"description" yaml:"description"`
	Framework        Framework     `json:"framework" yaml:"framework"`
	Applicability    Applicability `json:"applicability" yaml:"applicability"`
	Scope            Scope         `json:"scope" yaml:"scope"`
	Controls         []Control     `json:"controls" yaml:"controls"`
	ProhibitedClaims []string      `json:"prohibited_claims" yaml:"prohibited_claims"`
	Authority        Authority     `json:"authority" yaml:"authority"`
}

type Framework struct {
	ID      string `json:"id" yaml:"id"`
	Title   string `json:"title" yaml:"title"`
	Version string `json:"version" yaml:"version"`
	Source  string `json:"source" yaml:"source"`
}

type Applicability struct {
	Description string   `json:"description" yaml:"description"`
	TaskKinds   []string `json:"task_kinds,omitempty" yaml:"task_kinds,omitempty"`
	RiskLevels  []string `json:"risk_levels,omitempty" yaml:"risk_levels,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type Scope struct {
	Types []string `json:"types" yaml:"types"`
}

type Control struct {
	ID                         string                `json:"id" yaml:"id"`
	Title                      string                `json:"title" yaml:"title"`
	Objective                  string                `json:"objective" yaml:"objective"`
	Responsibility             string                `json:"responsibility" yaml:"responsibility"`
	AssessmentObjectives       []string              `json:"assessment_objectives" yaml:"assessment_objectives"`
	Evidence                   []EvidenceRequirement `json:"evidence" yaml:"evidence"`
	ExternalAssessmentRequired bool                  `json:"external_assessment_required,omitempty" yaml:"external_assessment_required,omitempty"`
}

type EvidenceRequirement struct {
	Class           string   `json:"class" yaml:"class"`
	MinimumCount    int      `json:"minimum_count" yaml:"minimum_count"`
	MaximumAge      string   `json:"maximum_age,omitempty" yaml:"maximum_age,omitempty"`
	AcceptedResults []string `json:"accepted_results" yaml:"accepted_results"`
}

type Authority struct {
	Mode              string   `json:"mode" yaml:"mode"`
	ProhibitedActions []string `json:"prohibited_actions" yaml:"prohibited_actions"`
}

type ValidationReport struct {
	Schema            string   `json:"schema"`
	Valid             bool     `json:"valid"`
	ProfileID         string   `json:"profile_id,omitempty"`
	ProfileVersion    string   `json:"profile_version,omitempty"`
	FrameworkID       string   `json:"framework_id,omitempty"`
	FrameworkVersion  string   `json:"framework_version,omitempty"`
	ControlCount      int      `json:"control_count"`
	EvidenceClasses   []string `json:"evidence_classes,omitempty"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

var allowedScopeTypes = stringSet("project", "task_set", "release")
var allowedResponsibilities = stringSet("product", "customer", "shared", "external_assessor")
var allowedEvidenceClasses = stringSet(
	"task", "decision", "evidence", "review", "ci", "release", "provenance",
	"rehearsal", "exception", "external_assessment", "configuration",
	"backup_restore", "vulnerability", "identity", "audit",
)
var allowedResults = stringSet("pass", "partial", "blocked", "fail", "approve", "changes", "verified")
var requiredClaims = stringSet("certified", "compliant", "authorized")
var requiredActions = stringSet(
	"certify", "declare_compliance", "accept_risk", "approve", "mutate_workflow",
	"merge", "deploy", "release", "use_credentials", "change_public_exposure", "run_live_operation",
)

func LoadFile(path string) (Profile, error) {
	if path == "" {
		return Profile{}, errors.New("assurance profile path is required")
	}
	if strings.Contains(path, "://") {
		return Profile{}, errors.New("assurance profile must be a local file")
	}
	clean := filepath.Clean(path)
	info, err := os.Lstat(clean)
	if err != nil {
		return Profile{}, fmt.Errorf("read assurance profile: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Profile{}, errors.New("assurance profile symlinks are not allowed")
	}
	if !info.Mode().IsRegular() {
		return Profile{}, errors.New("assurance profile must be a regular file")
	}
	if info.Size() > maxProfileSize {
		return Profile{}, fmt.Errorf("assurance profile exceeds %d bytes", maxProfileSize)
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return Profile{}, fmt.Errorf("read assurance profile: %w", err)
	}

	var profile Profile
	switch strings.ToLower(filepath.Ext(clean)) {
	case ".json":
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&profile); err != nil {
			return Profile{}, fmt.Errorf("decode assurance profile JSON: %w", err)
		}
		if err := requireJSONEOF(decoder); err != nil {
			return Profile{}, err
		}
	case ".yaml", ".yml":
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&profile); err != nil {
			return Profile{}, fmt.Errorf("decode assurance profile YAML: %w", err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			if err == nil {
				return Profile{}, errors.New("assurance profile must contain one YAML document")
			}
			return Profile{}, fmt.Errorf("decode assurance profile YAML: %w", err)
		}
	default:
		return Profile{}, errors.New("assurance profile must use .json, .yaml, or .yml")
	}
	if err := Validate(profile); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func Validate(profile Profile) error {
	if profile.Schema != ProfileSchema {
		return fmt.Errorf("unsupported assurance profile schema %q", profile.Schema)
	}
	if err := validIdentifier("profile id", profile.ID, identifierPattern); err != nil {
		return err
	}
	if err := validIdentifier("profile version", profile.Version, identifierPattern); err != nil {
		return err
	}
	if err := validText("profile title", profile.Title, true); err != nil {
		return err
	}
	if err := validText("profile description", profile.Description, true); err != nil {
		return err
	}
	if err := validateFramework(profile.Framework); err != nil {
		return err
	}
	if err := validateApplicability(profile.Applicability); err != nil {
		return err
	}
	if len(profile.Scope.Types) == 0 {
		return errors.New("assurance profile scope.types is required")
	}
	if err := validateStringList("scope type", profile.Scope.Types, allowedScopeTypes, true); err != nil {
		return err
	}
	if len(profile.Controls) == 0 {
		return errors.New("assurance profile controls are required")
	}
	seen := make(map[string]bool, len(profile.Controls))
	for i, control := range profile.Controls {
		if err := validateControl(control); err != nil {
			return fmt.Errorf("control %d: %w", i+1, err)
		}
		if seen[control.ID] {
			return fmt.Errorf("duplicate assurance control id %q", control.ID)
		}
		seen[control.ID] = true
	}
	if err := validateRequiredSet("prohibited claim", profile.ProhibitedClaims, requiredClaims); err != nil {
		return err
	}
	if profile.Authority.Mode != "evidence_only" {
		return errors.New("assurance profile authority.mode must be evidence_only")
	}
	if err := validateRequiredSet("prohibited action", profile.Authority.ProhibitedActions, requiredActions); err != nil {
		return err
	}
	return nil
}

func Report(profile Profile) ValidationReport {
	classes := map[string]bool{}
	for _, control := range profile.Controls {
		for _, evidence := range control.Evidence {
			classes[evidence.Class] = true
		}
	}
	list := make([]string, 0, len(classes))
	for class := range classes {
		list = append(list, class)
	}
	sort.Strings(list)
	return ValidationReport{
		Schema:            "fairway.assurance-profile-validation.v1",
		Valid:             true,
		ProfileID:         profile.ID,
		ProfileVersion:    profile.Version,
		FrameworkID:       profile.Framework.ID,
		FrameworkVersion:  profile.Framework.Version,
		ControlCount:      len(profile.Controls),
		EvidenceClasses:   list,
		AuthorityBoundary: "evidence organization only; no certification, compliance, risk acceptance, workflow mutation, approval, merge, deploy, release, credential, public-exposure, or live-operation authority",
	}
}

func validateFramework(framework Framework) error {
	if err := validIdentifier("framework id", framework.ID, identifierPattern); err != nil {
		return err
	}
	if err := validText("framework title", framework.Title, true); err != nil {
		return err
	}
	if err := validIdentifier("framework version", framework.Version, identifierPattern); err != nil {
		return err
	}
	parsed, err := url.Parse(framework.Source)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("framework source must be an HTTPS URL without credentials, query, or fragment")
	}
	return nil
}

func validateApplicability(applicability Applicability) error {
	if err := validText("applicability description", applicability.Description, true); err != nil {
		return err
	}
	for name, values := range map[string][]string{
		"applicability task kind":  applicability.TaskKinds,
		"applicability risk level": applicability.RiskLevels,
		"applicability tag":        applicability.Tags,
	} {
		seen := map[string]bool{}
		for _, value := range values {
			if err := validIdentifier(name, value, identifierPattern); err != nil {
				return err
			}
			if seen[value] {
				return fmt.Errorf("duplicate %s %q", name, value)
			}
			seen[value] = true
		}
	}
	return nil
}

func validateControl(control Control) error {
	if err := validIdentifier("control id", control.ID, controlIDPattern); err != nil {
		return err
	}
	if err := validText("control title", control.Title, true); err != nil {
		return err
	}
	if err := validText("control objective", control.Objective, true); err != nil {
		return err
	}
	if !allowedResponsibilities[control.Responsibility] {
		return fmt.Errorf("unsupported control responsibility %q", control.Responsibility)
	}
	if control.Responsibility == "external_assessor" && !control.ExternalAssessmentRequired {
		return errors.New("external_assessor responsibility requires external_assessment_required")
	}
	if len(control.AssessmentObjectives) == 0 {
		return errors.New("assessment_objectives is required")
	}
	for _, objective := range control.AssessmentObjectives {
		if err := validText("assessment objective", objective, true); err != nil {
			return err
		}
	}
	if len(control.Evidence) == 0 {
		return errors.New("evidence requirements are required")
	}
	seenClasses := map[string]bool{}
	for _, requirement := range control.Evidence {
		if !allowedEvidenceClasses[requirement.Class] {
			return fmt.Errorf("unsupported evidence class %q", requirement.Class)
		}
		if seenClasses[requirement.Class] {
			return fmt.Errorf("duplicate evidence class %q", requirement.Class)
		}
		seenClasses[requirement.Class] = true
		if requirement.MinimumCount < 1 {
			return fmt.Errorf("evidence class %q minimum_count must be positive", requirement.Class)
		}
		if requirement.MaximumAge != "" {
			duration, err := time.ParseDuration(requirement.MaximumAge)
			if err != nil || duration <= 0 {
				return fmt.Errorf("evidence class %q maximum_age must be a positive duration", requirement.Class)
			}
		}
		if err := validateStringList("accepted result", requirement.AcceptedResults, allowedResults, true); err != nil {
			return err
		}
	}
	return nil
}

func validIdentifier(name, value string, pattern *regexp.Regexp) error {
	if !pattern.MatchString(value) {
		return fmt.Errorf("%s %q is invalid", name, value)
	}
	return nil
}

func validText(name, value string, required bool) error {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len(value) > 4096 {
		return fmt.Errorf("%s exceeds 4096 bytes", name)
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"authorization: bearer", "api_key=", "api-key=", "api_key:", "api-key:", "password=", "password:",
		"access_token=", "access_token:", "refresh_token=", "refresh_token:", "id_token=", "id_token:",
		"client_secret=", "client_secret:", "private_key=", "private_key:",
		"#!/bin/", "bash -c ", "sh -c ", "powershell -command", "cmd.exe /c", "curl http", "wget http", "$(", "`curl ", "`wget ",
	} {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("%s contains prohibited secret-like or executable content", name)
		}
	}
	return nil
}

func validateStringList(name string, values []string, allowed map[string]bool, required bool) error {
	if required && len(values) == 0 {
		return fmt.Errorf("%s list is required", name)
	}
	seen := map[string]bool{}
	for _, value := range values {
		if !allowed[value] {
			return fmt.Errorf("unsupported %s %q", name, value)
		}
		if seen[value] {
			return fmt.Errorf("duplicate %s %q", name, value)
		}
		seen[value] = true
	}
	return nil
}

func validateRequiredSet(name string, values []string, required map[string]bool) error {
	seen := map[string]bool{}
	for _, value := range values {
		if !identifierPattern.MatchString(value) {
			return fmt.Errorf("%s %q is invalid", name, value)
		}
		if seen[value] {
			return fmt.Errorf("duplicate %s %q", name, value)
		}
		seen[value] = true
	}
	for value := range required {
		if !seen[value] {
			return fmt.Errorf("required %s %q is missing", name, value)
		}
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("assurance profile must contain one JSON value")
		}
		return fmt.Errorf("decode assurance profile JSON: %w", err)
	}
	return nil
}

func stringSet(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
