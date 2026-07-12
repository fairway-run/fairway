package provenance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/networkpolicy"
)

const maxNarrativeResponseBytes = 64 << 10
const maxNarrativePromptBytes = 256 << 10

type ExplainNarrative struct {
	Schema            string               `json:"schema"`
	Provider          string               `json:"provider"`
	Model             string               `json:"model,omitempty"`
	Statements        []NarrativeStatement `json:"statements"`
	GroundedSchema    string               `json:"grounded_packet_schema"`
	GroundedCommit    string               `json:"grounded_commit"`
	Advisory          bool                 `json:"advisory"`
	AuthorityBoundary string               `json:"authority_boundary"`
}

type NarrativeStatement struct {
	Label     string   `json:"label"`
	Text      string   `json:"text"`
	Citations []string `json:"citations,omitempty"`
}

type ollamaNarrativeRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format"`
}

type ollamaNarrativeResponse struct {
	Response string `json:"response"`
}

func GenerateExplainNarrative(ctx context.Context, cfg config.Config, packet ExplainCodePacket, adapterName string) (ExplainNarrative, error) {
	adapter, err := explainNarrativeAdapter(cfg.AdvisoryAdapters, adapterName)
	if err != nil {
		return ExplainNarrative{}, err
	}
	if firstNonEmpty(strings.TrimSpace(adapter.Type), "noop") != "local_ollama" {
		return ExplainNarrative{}, fmt.Errorf("advisory explain narrative currently supports local_ollama adapters only")
	}
	if !containsToken(adapter.Capabilities, "explain_code_narrative") {
		return ExplainNarrative{}, fmt.Errorf("advisory provider adapter lacks explain_code_narrative capability")
	}
	if !containsToken(adapter.AllowedActions, "render_packet") {
		return ExplainNarrative{}, fmt.Errorf("advisory provider adapter does not allow render_packet")
	}
	if strings.TrimSpace(adapter.Model) == "" {
		return ExplainNarrative{}, fmt.Errorf("local_ollama advisory narrative adapter requires model")
	}
	endpoint, err := narrativeEndpoint(adapter.EndpointEnv)
	if err != nil {
		return ExplainNarrative{}, err
	}
	prompt, err := narrativePrompt(packet)
	if err != nil {
		return ExplainNarrative{}, err
	}
	body, err := json.Marshal(ollamaNarrativeRequest{Model: strings.TrimSpace(adapter.Model), Prompt: prompt, Stream: false, Format: "json"})
	if err != nil {
		return ExplainNarrative{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ExplainNarrative{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := networkpolicy.NewLoopbackHTTPClient(30*time.Second, !config.IsSovereignOffline(cfg))
	resp, err := client.Do(req)
	if err != nil {
		return ExplainNarrative{}, fmt.Errorf("advisory narrative request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ExplainNarrative{}, fmt.Errorf("advisory narrative endpoint returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxNarrativeResponseBytes+1))
	if err != nil {
		return ExplainNarrative{}, fmt.Errorf("advisory narrative response read failed")
	}
	if len(data) > maxNarrativeResponseBytes {
		return ExplainNarrative{}, fmt.Errorf("advisory narrative response exceeds %d bytes", maxNarrativeResponseBytes)
	}
	var providerResponse ollamaNarrativeResponse
	if err := json.Unmarshal(data, &providerResponse); err != nil || strings.TrimSpace(providerResponse.Response) == "" {
		return ExplainNarrative{}, fmt.Errorf("advisory narrative provider returned an invalid response envelope")
	}
	var narrative ExplainNarrative
	if err := decodeStrictNarrativeJSON([]byte(providerResponse.Response), &narrative); err != nil {
		return ExplainNarrative{}, fmt.Errorf("advisory narrative provider returned invalid JSON")
	}
	narrative.Provider = strings.TrimSpace(adapter.Name)
	narrative.Model = strings.TrimSpace(adapter.Model)
	narrative.GroundedSchema = packet.Schema
	narrative.GroundedCommit = packet.Git.Commit
	narrative.Advisory = true
	narrative.AuthorityBoundary = "generated narrative is advisory display output only; it is not recorded provenance and grants no task, review, merge, deploy, release, credential, public-exposure, or live-operation authority"
	if err := ValidateExplainNarrative(packet, narrative); err != nil {
		return ExplainNarrative{}, err
	}
	return narrative, nil
}

func ValidateExplainNarrative(packet ExplainCodePacket, narrative ExplainNarrative) error {
	if narrative.Schema != "fairway.explain-narrative.v1" {
		return fmt.Errorf("unsupported advisory narrative schema %q", narrative.Schema)
	}
	if len(narrative.Statements) == 0 || len(narrative.Statements) > 64 {
		return fmt.Errorf("advisory narrative requires 1..64 statements")
	}
	allowedRefs := map[string]bool{}
	for _, ref := range packet.MachineInferenceInputs {
		allowedRefs[ref] = true
	}
	for i := range narrative.Statements {
		statement := &narrative.Statements[i]
		statement.Label = strings.TrimSpace(statement.Label)
		statement.Text = strings.TrimSpace(statement.Text)
		switch statement.Label {
		case "recorded", "inferred", "unknown":
		default:
			return fmt.Errorf("advisory narrative statement %d has invalid label", i+1)
		}
		if statement.Text == "" || len(statement.Text) > 2048 {
			return fmt.Errorf("advisory narrative statement %d text must be 1..2048 bytes", i+1)
		}
		warnings := []string{}
		if redactString(statement.Text, &warnings, "narrative", fmt.Sprintf("statement:%d", i+1)) != statement.Text || unsafeNarrativeText(statement.Text) {
			return fmt.Errorf("advisory narrative statement %d contains privacy-rejected content", i+1)
		}
		statement.Citations = uniqueSortedStrings(statement.Citations)
		if statement.Label != "unknown" && len(statement.Citations) == 0 {
			return fmt.Errorf("advisory narrative statement %d requires a grounded citation", i+1)
		}
		for _, citation := range statement.Citations {
			if !allowedRefs[citation] {
				return fmt.Errorf("advisory narrative statement %d cites an unknown packet fact", i+1)
			}
		}
	}
	return nil
}

func explainNarrativeAdapter(adapters []config.AdvisoryAdapter, name string) (config.AdvisoryAdapter, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return config.AdvisoryAdapter{}, fmt.Errorf("--narrative-provider is required")
	}
	for _, adapter := range adapters {
		if strings.TrimSpace(adapter.Name) != name {
			continue
		}
		mode := firstNonEmpty(strings.TrimSpace(adapter.Mode), "advisory")
		if mode == "disabled" {
			return config.AdvisoryAdapter{}, fmt.Errorf("advisory provider adapter is disabled")
		}
		return adapter, nil
	}
	return config.AdvisoryAdapter{}, fmt.Errorf("configured advisory provider adapter not found")
}

func narrativeEndpoint(envName string) (string, error) {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		return "", fmt.Errorf("local_ollama advisory narrative adapter requires endpoint_env")
	}
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return "", fmt.Errorf("advisory narrative endpoint environment variable is unset")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("advisory narrative endpoint must be a loopback HTTP base URL")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("advisory narrative endpoint must be loopback-only")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("advisory narrative endpoint must be a base URL without a path")
	}
	parsed.Path = "/api/generate"
	return parsed.String(), nil
}

func narrativePrompt(packet ExplainCodePacket) (string, error) {
	data, err := json.Marshal(packet)
	if err != nil {
		return "", err
	}
	prompt := "Return JSON only using schema fairway.explain-narrative.v1 with statements [{label,text,citations}]. " +
		"Labels are recorded, inferred, or unknown. Recorded and inferred statements require citations copied exactly from machine_inference_inputs. " +
		"Do not invent history, include source bodies, prompts, transcripts, tool bodies, credentials, secrets, or claim authority. Grounded packet:\n" + string(data)
	if len(prompt) > maxNarrativePromptBytes {
		return "", fmt.Errorf("grounded narrative prompt exceeds %d bytes", maxNarrativePromptBytes)
	}
	return prompt, nil
}

func decodeStrictNarrativeJSON(data []byte, target *ExplainNarrative) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("trailing narrative JSON")
	}
	return nil
}

func containsToken(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	var out []string
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func unsafeNarrativeText(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"raw_prompt", "raw prompt", "transcript:", "tool_body", "tool body:", "generated_content", "authorization:", "-----begin private key-----"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
