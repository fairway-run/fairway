// Package harnessrecord defines privacy-bounded records imported from execution surfaces.
package harnessrecord

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/secretscan"
)

const (
	BatchSchema       = "fairway.harness-record-batch.v1"
	ExternalRunSchema = "fairway.harness.external-run.v1"
	ObservationSchema = "fairway.harness.execution-observation.v1"
	EvaluationSchema  = "fairway.harness.evaluator-result.v1"
	maxInputBytes     = 4 << 20
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,199}$`)
var sha256Hex = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

// RecordRef fully qualifies a source-owned record identity.
type RecordRef struct {
	SourceID string `json:"source_id"`
	ID       string `json:"id"`
}

// Batch is one atomically validated harness-record input.
type Batch struct {
	Schema           string            `json:"schema"`
	ExternalRuns     []ExternalRun     `json:"external_runs,omitempty"`
	Observations     []Observation     `json:"observations,omitempty"`
	EvaluatorResults []EvaluatorResult `json:"evaluator_results,omitempty"`
}

// ExternalRun identifies one external execution attempt.
type ExternalRun struct {
	Schema         string         `json:"schema"`
	SourceID       string         `json:"source_id"`
	SourceVersion  string         `json:"source_version"`
	ExternalRunID  string         `json:"external_run_id"`
	TaskID         string         `json:"task_id"`
	SubmissionID   string         `json:"submission_id"`
	SessionID      string         `json:"session_id,omitempty"`
	CallerWorkID   string         `json:"caller_work_id,omitempty"`
	PriorRunRef    *RecordRef     `json:"prior_run_ref,omitempty"`
	Provider       string         `json:"provider,omitempty"`
	Model          string         `json:"model,omitempty"`
	Harness        string         `json:"harness,omitempty"`
	Repository     string         `json:"repository,omitempty"`
	Revision       string         `json:"revision,omitempty"`
	Branch         string         `json:"branch,omitempty"`
	WorkspaceRef   string         `json:"workspace_ref,omitempty"`
	TraceID        string         `json:"trace_id,omitempty"`
	TerminalStatus string         `json:"terminal_status,omitempty"`
	ObservedAt     string         `json:"observed_at"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// Observation is one sourced experiment or material execution fact.
type Observation struct {
	Schema              string         `json:"schema"`
	ObservationID       string         `json:"observation_id"`
	SourceID            string         `json:"source_id"`
	SourceVersion       string         `json:"source_version"`
	TaskID              string         `json:"task_id"`
	ExternalRunRef      *RecordRef     `json:"external_run_ref,omitempty"`
	Kind                string         `json:"kind"`
	SubjectType         string         `json:"subject_type"`
	SubjectRef          string         `json:"subject_ref"`
	Summary             string         `json:"summary"`
	ObservedAt          string         `json:"observed_at"`
	Outcome             string         `json:"outcome"`
	SourceMode          string         `json:"source_mode"`
	Hypothesis          string         `json:"hypothesis,omitempty"`
	ExpectedObservation string         `json:"expected_observation,omitempty"`
	ActualMeasurement   string         `json:"actual_measurement,omitempty"`
	Units               string         `json:"units,omitempty"`
	ArtifactRef         string         `json:"artifact_ref,omitempty"`
	ArtifactSHA256      string         `json:"artifact_sha256,omitempty"`
	Confidence          *float64       `json:"confidence,omitempty"`
	ConfidenceMethod    string         `json:"confidence_method,omitempty"`
	Uncertainty         string         `json:"uncertainty,omitempty"`
	Completeness        string         `json:"completeness,omitempty"`
	ActionFingerprint   string         `json:"action_fingerprint,omitempty"`
	RecommendedAction   string         `json:"recommended_action,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
}

// EvaluatorResult records how one named evaluator assessed a bounded subject.
type EvaluatorResult struct {
	Schema           string         `json:"schema"`
	EvaluationID     string         `json:"evaluation_id"`
	SourceID         string         `json:"source_id"`
	SourceVersion    string         `json:"source_version"`
	TaskID           string         `json:"task_id"`
	ExternalRunRef   *RecordRef     `json:"external_run_ref,omitempty"`
	ObservationRef   *RecordRef     `json:"observation_ref,omitempty"`
	EvaluatorID      string         `json:"evaluator_id"`
	EvaluatorVersion string         `json:"evaluator_version"`
	SubjectType      string         `json:"subject_type"`
	SubjectRef       string         `json:"subject_ref"`
	Result           string         `json:"result"`
	Mode             string         `json:"mode"`
	EvaluatedAt      string         `json:"evaluated_at"`
	Environment      string         `json:"environment,omitempty"`
	RepositoryRev    string         `json:"repository_revision,omitempty"`
	RubricVersion    string         `json:"rubric_version,omitempty"`
	SampleCount      *int           `json:"sample_count,omitempty"`
	ExclusionCount   *int           `json:"exclusion_count,omitempty"`
	Measurement      string         `json:"measurement,omitempty"`
	Threshold        string         `json:"threshold,omitempty"`
	ArtifactRef      string         `json:"artifact_ref,omitempty"`
	ArtifactSHA256   string         `json:"artifact_sha256,omitempty"`
	Confidence       *float64       `json:"confidence,omitempty"`
	ConfidenceMethod string         `json:"confidence_method,omitempty"`
	Uncertainty      string         `json:"uncertainty,omitempty"`
	Completeness     string         `json:"completeness,omitempty"`
	Summary          string         `json:"summary,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// DecodeBatch reads one strict, duplicate-key-free batch.
func DecodeBatch(r io.Reader) (Batch, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxInputBytes+1))
	if err != nil {
		return Batch{}, err
	}
	if len(data) > maxInputBytes {
		return Batch{}, errors.New("harness record batch exceeds 4 MiB")
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return Batch{}, err
	}
	if err := rejectAmbiguousExplicitValues(data); err != nil {
		return Batch{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var batch Batch
	if err := dec.Decode(&batch); err != nil {
		return Batch{}, fmt.Errorf("decode harness record batch: %w", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return Batch{}, errors.New("harness record batch contains trailing JSON")
	}
	if err := ValidateBatch(batch, time.Now().UTC()); err != nil {
		return Batch{}, err
	}
	return batch, nil
}

// ValidateBatch validates schemas, enums, references, privacy, and bounded values.
func ValidateBatch(batch Batch, now time.Time) error {
	if batch.Schema != BatchSchema {
		return fmt.Errorf("unsupported harness batch schema %q", batch.Schema)
	}
	if len(batch.ExternalRuns)+len(batch.Observations)+len(batch.EvaluatorResults) == 0 {
		return errors.New("harness record batch is empty")
	}
	seen := map[string]bool{}
	for i := range batch.ExternalRuns {
		r := &batch.ExternalRuns[i]
		if r.Schema != ExternalRunSchema || !validID(r.SourceID) || !validID(r.ExternalRunID) || !validID(r.SubmissionID) || !validID(r.TaskID) || strings.TrimSpace(r.SourceVersion) == "" {
			return fmt.Errorf("invalid external run at index %d", i)
		}
		if err := validTime(r.ObservedAt, now); err != nil {
			return fmt.Errorf("external run %s: %w", r.ExternalRunID, err)
		}
		if err := validateRef(r.PriorRunRef); err != nil {
			return fmt.Errorf("external run %s prior run: %w", r.ExternalRunID, err)
		}
		if err := validateMetadata(r.Metadata); err != nil {
			return fmt.Errorf("external run %s: %w", r.ExternalRunID, err)
		}
		if err := validateSafe(r); err != nil {
			return fmt.Errorf("external run %s: %w", r.ExternalRunID, err)
		}
		key := "run/" + r.SourceID + "/" + r.ExternalRunID
		if seen[key] {
			return fmt.Errorf("duplicate batch identity %s", key)
		}
		seen[key] = true
	}
	for i := range batch.Observations {
		o := &batch.Observations[i]
		if o.Schema != ObservationSchema || !validID(o.SourceID) || !validID(o.ObservationID) || !validID(o.TaskID) || strings.TrimSpace(o.SourceVersion) == "" || strings.TrimSpace(o.SubjectType) == "" || strings.TrimSpace(o.SubjectRef) == "" || strings.TrimSpace(o.Summary) == "" {
			return fmt.Errorf("invalid observation at index %d", i)
		}
		if !oneOf(o.Kind, "experiment", "execution", "artifact", "policy", "usage", "terminal") || !oneOf(o.Outcome, "confirmed", "rejected", "inconclusive", "blocked", "unavailable") || !oneOf(o.SourceMode, "measured", "reported", "derived", "human_judgment") {
			return fmt.Errorf("invalid observation enum at index %d", i)
		}
		if o.Kind == "experiment" && (strings.TrimSpace(o.Hypothesis) == "" || strings.TrimSpace(o.ExpectedObservation) == "") {
			return fmt.Errorf("experiment observation %s requires hypothesis and expected_observation", o.ObservationID)
		}
		if err := validTime(o.ObservedAt, now); err != nil {
			return fmt.Errorf("observation %s: %w", o.ObservationID, err)
		}
		if err := validateRef(o.ExternalRunRef); err != nil {
			return fmt.Errorf("observation %s run: %w", o.ObservationID, err)
		}
		if err := validateConfidence(o.Confidence, o.ConfidenceMethod); err != nil {
			return fmt.Errorf("observation %s: %w", o.ObservationID, err)
		}
		if err := validateMetadata(o.Metadata); err != nil {
			return fmt.Errorf("observation %s: %w", o.ObservationID, err)
		}
		if o.ArtifactSHA256 != "" && !sha256Hex.MatchString(o.ArtifactSHA256) {
			return fmt.Errorf("observation %s: artifact_sha256 must be 64 hexadecimal characters", o.ObservationID)
		}
		if err := validateSafe(o); err != nil {
			return fmt.Errorf("observation %s: %w", o.ObservationID, err)
		}
		key := "observation/" + o.SourceID + "/" + o.ObservationID
		if seen[key] {
			return fmt.Errorf("duplicate batch identity %s", key)
		}
		seen[key] = true
	}
	for i := range batch.EvaluatorResults {
		e := &batch.EvaluatorResults[i]
		if e.Schema != EvaluationSchema || !validID(e.SourceID) || !validID(e.EvaluationID) || !validID(e.TaskID) || strings.TrimSpace(e.SourceVersion) == "" || !validID(e.EvaluatorID) || strings.TrimSpace(e.EvaluatorVersion) == "" || strings.TrimSpace(e.SubjectType) == "" || strings.TrimSpace(e.SubjectRef) == "" {
			return fmt.Errorf("invalid evaluator result at index %d", i)
		}
		if !oneOf(e.Result, "pass", "fail", "partial", "inconclusive", "error", "unavailable") || !oneOf(e.Mode, "deterministic", "statistical", "human_judgment") {
			return fmt.Errorf("invalid evaluator result enum at index %d", i)
		}
		if err := validTime(e.EvaluatedAt, now); err != nil {
			return fmt.Errorf("evaluation %s: %w", e.EvaluationID, err)
		}
		if err := validateRef(e.ExternalRunRef); err != nil {
			return fmt.Errorf("evaluation %s run: %w", e.EvaluationID, err)
		}
		if err := validateRef(e.ObservationRef); err != nil {
			return fmt.Errorf("evaluation %s observation: %w", e.EvaluationID, err)
		}
		if err := validateConfidence(e.Confidence, e.ConfidenceMethod); err != nil {
			return fmt.Errorf("evaluation %s: %w", e.EvaluationID, err)
		}
		if err := validateMetadata(e.Metadata); err != nil {
			return fmt.Errorf("evaluation %s: %w", e.EvaluationID, err)
		}
		if e.ArtifactSHA256 != "" && !sha256Hex.MatchString(e.ArtifactSHA256) {
			return fmt.Errorf("evaluation %s: artifact_sha256 must be 64 hexadecimal characters", e.EvaluationID)
		}
		if err := validateSafe(e); err != nil {
			return fmt.Errorf("evaluation %s: %w", e.EvaluationID, err)
		}
		key := "evaluation/" + e.SourceID + "/" + e.EvaluationID
		if seen[key] {
			return fmt.Errorf("duplicate batch identity %s", key)
		}
		seen[key] = true
	}
	return nil
}

// Canonical returns the compact typed JSON and its SHA-256 digest.
func Canonical(value any) ([]byte, string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

func validID(value string) bool { return identifier.MatchString(strings.TrimSpace(value)) }
func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
func validTime(value string, now time.Time) error {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return errors.New("timestamp must be RFC3339")
	}
	if parsed.After(now.Add(5 * time.Minute)) {
		return errors.New("timestamp is too far in the future")
	}
	return nil
}
func validateRef(ref *RecordRef) error {
	if ref == nil {
		return nil
	}
	if !validID(ref.SourceID) || !validID(ref.ID) {
		return errors.New("reference requires valid source_id and id")
	}
	return nil
}
func validateConfidence(value *float64, method string) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 1 {
		return errors.New("confidence must be between 0 and 1")
	}
	if strings.TrimSpace(method) == "" {
		return errors.New("confidence_method is required with confidence")
	}
	return nil
}
func validateSafe(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > 256<<10 {
		return errors.New("record exceeds 256 KiB")
	}
	if secretscan.Contains(data) {
		return errors.New("record contains secret-like material")
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if key := prohibitedContentKey(decoded); key != "" {
		return fmt.Errorf("record contains prohibited content field %q", key)
	}
	if !boundedJSON(decoded, 0) {
		return errors.New("record metadata exceeds maximum JSON depth")
	}
	return nil
}

func prohibitedContentKey(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if prohibitedContentKeyName(key) {
				return key
			}
			if found := prohibitedContentKey(child); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := prohibitedContentKey(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func prohibitedContentKeyName(key string) bool {
	lower := strings.ToLower(key)
	tokens := strings.FieldsFunc(lower, func(r rune) bool { return r == '_' || r == '-' || r == '.' || r == '/' || r == ':' })
	has := func(want string) bool {
		for _, token := range tokens {
			if token == want {
				return true
			}
		}
		return false
	}
	normalized := strings.Join(tokens, "")
	return normalized == "prompt" || normalized == "reasoning" ||
		strings.Contains(normalized, "rawprompt") ||
		strings.Contains(normalized, "privatereasoning") ||
		strings.Contains(normalized, "chainofthought") ||
		strings.Contains(normalized, "transcript") ||
		strings.Contains(normalized, "rawtoolbody") ||
		strings.Contains(normalized, "generatedcontent") ||
		strings.Contains(normalized, "credential") ||
		(has("raw") && has("prompt")) ||
		(has("private") && has("reasoning")) ||
		(has("chain") && has("thought")) ||
		(has("raw") && has("tool") && has("body")) ||
		(has("generated") && has("content")) || has("transcript") || has("credential") || has("credentials")
}

func validateMetadata(metadata map[string]any) error {
	for key := range metadata {
		if !strings.ContainsAny(key, ".:/") {
			return fmt.Errorf("metadata key %q must be namespaced", key)
		}
		if len(key) > 200 {
			return errors.New("metadata key exceeds 200 characters")
		}
	}
	return nil
}

func rejectAmbiguousExplicitValues(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var walk func(any, bool) error
	walk = func(current any, root bool) error {
		switch typed := current.(type) {
		case nil:
			return errors.New("explicit null is not allowed; omit optional fields")
		case string:
			if typed == "" {
				return errors.New("explicit empty string is not allowed; omit optional fields")
			}
		case map[string]any:
			if !root && len(typed) == 0 {
				return errors.New("explicit empty object is not allowed; omit optional fields")
			}
			for _, child := range typed {
				if err := walk(child, false); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := walk(child, false); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(value, true)
}

func boundedJSON(value any, depth int) bool {
	if depth > 8 {
		return false
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if !boundedJSON(child, depth+1) {
				return false
			}
		}
	case []any:
		for _, child := range typed {
			if !boundedJSON(child, depth+1) {
				return false
			}
		}
	}
	return true
}

func rejectDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch delim := tok.(type) {
		case json.Delim:
			switch delim {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					keyTok, err := dec.Token()
					if err != nil {
						return err
					}
					key, ok := keyTok.(string)
					if !ok {
						return errors.New("invalid JSON object key")
					}
					if seen[key] {
						return fmt.Errorf("duplicate JSON object key %q", key)
					}
					seen[key] = true
					if err := walk(); err != nil {
						return err
					}
				}
				_, err = dec.Token()
				return err
			case '[':
				for dec.More() {
					if err := walk(); err != nil {
						return err
					}
				}
				_, err = dec.Token()
				return err
			}
		}
		return nil
	}
	return walk()
}
