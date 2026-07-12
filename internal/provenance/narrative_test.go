package provenance

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/subashram/fairway/internal/config"
)

func TestNarrativeEndpointFailsClosedOutsideLoopback(t *testing.T) {
	for _, value := range []string{
		"https://127.0.0.1:11434",
		"http://192.0.2.10:11434",
		"http://user:secret@127.0.0.1:11434",
		"http://127.0.0.1:11434/custom",
		"http://127.0.0.1:11434?token=secret",
	} {
		t.Setenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT", value)
		if _, err := narrativeEndpoint("FAIRWAY_TEST_NARRATIVE_ENDPOINT"); err == nil || strings.Contains(err.Error(), value) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("value=%q err=%v", value, err)
		}
	}
	t.Setenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT", "http://127.0.0.1:11434")
	endpoint, err := narrativeEndpoint("FAIRWAY_TEST_NARRATIVE_ENDPOINT")
	if err != nil || endpoint != "http://127.0.0.1:11434/api/generate" {
		t.Fatalf("endpoint=%q err=%v", endpoint, err)
	}
	_ = os.Unsetenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT")
}

func TestSovereignOfflineNarrativeAdapterUsesNumericLoopbackWithoutProxyDNSOrRedirect(t *testing.T) {
	var proxyHits atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		proxyHits.Add(1)
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "http://198.51.100.20/escape", http.StatusFound)
			return
		}
		fmt.Fprint(w, `{"response":"{\"schema\":\"fairway.explain-narrative.v1\",\"statements\":[{\"label\":\"recorded\",\"text\":\"Task exists.\",\"citations\":[\"task:T-001\"]}]}"}`)
	}))
	defer server.Close()

	cfg := config.Defaults(t.TempDir())
	cfg.Runtime.Profile = config.RuntimeProfileSovereignOffline
	cfg.AdvisoryAdapters = []config.AdvisoryAdapter{{
		Name: "local", Provider: "ollama", Type: "local_ollama", Mode: "advisory", Model: "test",
		EndpointEnv: "FAIRWAY_TEST_NARRATIVE_ENDPOINT", Capabilities: []string{"explain_code_narrative"}, AllowedActions: []string{"render_packet"},
	}}
	packet := ExplainCodePacket{Schema: "fairway.explain-code.v1", MachineInferenceInputs: []string{"task:T-001"}}
	packet.Git.Commit = "abc1234"
	t.Setenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT", server.URL)
	if _, err := GenerateExplainNarrative(context.Background(), cfg, packet, "local"); err != nil {
		t.Fatalf("numeric loopback narrative: %v", err)
	}
	if proxyHits.Load() != 0 {
		t.Fatalf("proxy hits = %d, want 0", proxyHits.Load())
	}

	localhostEndpoint := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	t.Setenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT", localhostEndpoint)
	if _, err := GenerateExplainNarrative(context.Background(), cfg, packet, "local"); err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("DNS endpoint error = %v, want fail-closed request", err)
	}

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://198.51.100.20/escape", http.StatusFound)
	}))
	defer redirect.Close()
	t.Setenv("FAIRWAY_TEST_NARRATIVE_ENDPOINT", redirect.URL)
	if _, err := GenerateExplainNarrative(context.Background(), cfg, packet, "local"); err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("redirect error = %v, want fail-closed request", err)
	}
}

func TestValidateExplainNarrativeRequiresKnownCitationsAndPrivacy(t *testing.T) {
	packet := ExplainCodePacket{MachineInferenceInputs: []string{"task:T-001", "evidence:T-001:1"}}
	valid := ExplainNarrative{
		Schema: "fairway.explain-narrative.v1",
		Statements: []NarrativeStatement{
			{Label: "recorded", Text: "The task exists.", Citations: []string{"task:T-001"}},
			{Label: "inferred", Text: "The evidence supports a bounded interpretation.", Citations: []string{"evidence:T-001:1"}},
			{Label: "unknown", Text: "No record explains the later caller."},
		},
	}
	if err := ValidateExplainNarrative(packet, valid); err != nil {
		t.Fatal(err)
	}
	unknown := valid
	unknown.Statements = []NarrativeStatement{{Label: "recorded", Text: "Claim", Citations: []string{"task:OTHER"}}}
	if err := ValidateExplainNarrative(packet, unknown); err == nil || !strings.Contains(err.Error(), "unknown packet fact") {
		t.Fatalf("unknown citation err=%v", err)
	}
	uncited := valid
	uncited.Statements = []NarrativeStatement{{Label: "inferred", Text: "Claim"}}
	if err := ValidateExplainNarrative(packet, uncited); err == nil || !strings.Contains(err.Error(), "requires a grounded citation") {
		t.Fatalf("uncited err=%v", err)
	}
	secret := valid
	secret.Statements = []NarrativeStatement{{Label: "recorded", Text: "access_token=do-not-render", Citations: []string{"task:T-001"}}}
	if err := ValidateExplainNarrative(packet, secret); err == nil || !strings.Contains(err.Error(), "privacy-rejected") {
		t.Fatalf("secret err=%v", err)
	}
}

func TestDecodeStrictNarrativeJSONRejectsUnknownAndTrailingFields(t *testing.T) {
	for _, raw := range []string{
		`{"schema":"fairway.explain-narrative.v1","statements":[],"unexpected":true}`,
		`{"schema":"fairway.explain-narrative.v1","statements":[]} {}`,
	} {
		var narrative ExplainNarrative
		if err := decodeStrictNarrativeJSON([]byte(raw), &narrative); err == nil {
			t.Fatalf("expected strict JSON rejection for %s", raw)
		}
	}
}

func TestNarrativePromptIsBounded(t *testing.T) {
	packet := ExplainCodePacket{Schema: "fairway.explain-code.v1", Facts: []ExplainFact{{Ref: "fact:1", Summary: strings.Repeat("x", maxNarrativePromptBytes)}}}
	if _, err := narrativePrompt(packet); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("prompt err=%v", err)
	}
}
