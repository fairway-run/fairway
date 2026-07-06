package dashboard

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/subashram/fairway/internal/store"
)

type apiStatusResponse struct {
	Project           string `json:"project"`
	Mode              string `json:"mode"`
	ReadOnly          bool   `json:"read_only"`
	WritesEnabled     bool   `json:"writes_enabled"`
	DashboardReadOnly bool   `json:"dashboard_read_only"`
	TrustedProxy      string `json:"trusted_proxy,omitempty"`
	IdentityMode      string `json:"identity_mode"`
}

type apiActor struct {
	Subject string
	Role    string
	Source  string
}

type apiTaskRow struct {
	ID           string   `json:"id"`
	ParentID     string   `json:"parent_id,omitempty"`
	Kind         string   `json:"kind,omitempty"`
	Title        string   `json:"title"`
	Role         string   `json:"role"`
	Status       string   `json:"status"`
	Owner        string   `json:"owner,omitempty"`
	Claimant     string   `json:"claimant,omitempty"`
	ReviewStatus string   `json:"review_status,omitempty"`
	Profile      string   `json:"profile,omitempty"`
	RiskLevel    string   `json:"risk_level,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

type apiTasksResponse struct {
	Tasks []apiTaskRow `json:"tasks"`
	Count int          `json:"count"`
}

type apiTaskDetailResponse struct {
	Task        apiTaskRow           `json:"task"`
	Definition  store.TaskDefinition `json:"definition"`
	Transitions []apiTransition      `json:"transitions"`
	Evidence    []apiEvidence        `json:"evidence"`
	Handoffs    []store.Handoff      `json:"handoffs"`
	Reviews     []apiReview          `json:"reviews"`
}

type apiTransition struct {
	FromStatus string `json:"from_status,omitempty"`
	ToStatus   string `json:"to_status"`
	Actor      string `json:"actor,omitempty"`
	Reason     string `json:"reason,omitempty"`
	At         string `json:"at,omitempty"`
}

type apiEvidence struct {
	CommandText     string `json:"command_text,omitempty"`
	Result          string `json:"result,omitempty"`
	ArtifactPath    string `json:"artifact_path,omitempty"`
	ArtifactType    string `json:"artifact_type,omitempty"`
	DurationSeconds *int   `json:"duration_seconds,omitempty"`
	Notes           string `json:"notes,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
}

type apiReview struct {
	Reviewer  string `json:"reviewer,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Verdict   string `json:"verdict,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Commit    string `json:"commit,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type apiSummaryResponse struct {
	Project    string `json:"project"`
	Total      int    `json:"total"`
	Todo       int    `json:"todo"`
	Ready      int    `json:"ready"`
	InProgress int    `json:"in_progress"`
	Blocked    int    `json:"blocked"`
	Done       int    `json:"done"`
}

func (s *Server) ReadOnlyAPIHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", s.apiStatus)
	mux.HandleFunc("/api/v1/tasks", s.apiTasks)
	mux.HandleFunc("/api/v1/tasks/", s.apiTaskDetail)
	mux.HandleFunc("/api/v1/reports/summary", s.apiSummary)
	mux.HandleFunc("/", s.apiIndex)
	return mux
}

func (s *Server) apiIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !apiRequireGET(w, r) {
		return
	}
	if !s.apiAuthorizeRead(w, r) {
		return
	}
	apiWriteJSON(w, http.StatusOK, map[string]any{
		"mode":            "read_only",
		"read_only":       true,
		"writes_enabled":  false,
		"available_paths": []string{"/api/v1/status", "/api/v1/tasks", "/api/v1/tasks/{task_id}", "/api/v1/reports/summary"},
	})
}

func (s *Server) apiStatus(w http.ResponseWriter, r *http.Request) {
	if !apiRequireGET(w, r) {
		return
	}
	if !s.apiAuthorizeRead(w, r) {
		return
	}
	apiWriteJSON(w, http.StatusOK, apiStatusResponse{
		Project:           s.cfg.Fairway.ProjectName,
		Mode:              "read_only",
		ReadOnly:          true,
		WritesEnabled:     false,
		DashboardReadOnly: s.cfg.Dashboard.ReadOnly,
		TrustedProxy:      s.cfg.Dashboard.TrustedProxy,
		IdentityMode:      s.serverIdentityMode(),
	})
}

func (s *Server) apiTasks(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/tasks" {
		http.NotFound(w, r)
		return
	}
	if !apiRequireGET(w, r) {
		return
	}
	if !s.apiAuthorizeRead(w, r) {
		return
	}
	tasks, err := s.store.AllTasks(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, err)
		return
	}
	rows := make([]apiTaskRow, 0, len(tasks))
	for _, task := range tasks {
		rows = append(rows, apiTask(task))
	}
	apiWriteJSON(w, http.StatusOK, apiTasksResponse{Tasks: rows, Count: len(rows)})
}

func (s *Server) apiTaskDetail(w http.ResponseWriter, r *http.Request) {
	if !apiRequireGET(w, r) {
		return
	}
	if !s.apiAuthorizeRead(w, r) {
		return
	}
	taskID := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	if taskID == "" || strings.Contains(taskID, "/") {
		http.NotFound(w, r)
		return
	}
	task, transitions, evidence, handoffs, reviews, err := s.store.TaskDetail(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			apiError(w, http.StatusNotFound, err)
			return
		}
		apiError(w, http.StatusInternalServerError, err)
		return
	}
	apiWriteJSON(w, http.StatusOK, apiTaskDetailResponse{
		Task:        apiTask(task),
		Definition:  task.Definition,
		Transitions: apiTransitions(transitions),
		Evidence:    apiEvidenceRows(evidence),
		Handoffs:    handoffs,
		Reviews:     apiReviewRows(reviews),
	})
}

func (s *Server) apiSummary(w http.ResponseWriter, r *http.Request) {
	if !apiRequireGET(w, r) {
		return
	}
	if !s.apiAuthorizeRead(w, r) {
		return
	}
	tasks, err := s.store.AllTasks(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, err)
		return
	}
	ready, err := s.store.Ready(r.Context(), "", nil)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err)
		return
	}
	readySet := map[string]bool{}
	for _, task := range ready {
		readySet[task.Definition.ID] = true
	}
	summary := apiSummaryResponse{Project: s.cfg.Fairway.ProjectName, Total: len(tasks), Ready: len(readySet)}
	for _, task := range tasks {
		switch task.Status {
		case "todo":
			summary.Todo++
		case "in_progress":
			summary.InProgress++
		case "blocked":
			summary.Blocked++
		case "done":
			summary.Done++
		}
	}
	apiWriteJSON(w, http.StatusOK, summary)
}

func apiTask(task store.Task) apiTaskRow {
	return apiTaskRow{
		ID:           task.Definition.ID,
		ParentID:     task.Definition.ParentID,
		Kind:         task.Definition.Kind,
		Title:        task.Definition.Title,
		Role:         task.Definition.Role,
		Status:       task.Status,
		Owner:        task.Owner,
		Claimant:     task.Claimant,
		ReviewStatus: task.ReviewStatus,
		Profile:      task.Definition.Profile,
		RiskLevel:    task.Definition.RiskLevel,
		Tags:         task.Definition.Tags,
		UpdatedAt:    task.UpdatedAt,
	}
}

func apiTransitions(transitions []store.Transition) []apiTransition {
	out := make([]apiTransition, 0, len(transitions))
	for _, transition := range transitions {
		out = append(out, apiTransition{
			FromStatus: transition.FromStatus,
			ToStatus:   transition.ToStatus,
			Actor:      transition.Actor,
			Reason:     transition.Reason,
			At:         transition.At,
		})
	}
	return out
}

func apiEvidenceRows(evidence []store.Evidence) []apiEvidence {
	out := make([]apiEvidence, 0, len(evidence))
	for _, row := range evidence {
		out = append(out, apiEvidence{
			CommandText:     row.CommandText,
			Result:          row.Result,
			ArtifactPath:    row.ArtifactPath,
			ArtifactType:    row.ArtifactType,
			DurationSeconds: row.DurationSeconds,
			Notes:           row.Notes,
			CreatedAt:       row.CreatedAt,
		})
	}
	return out
}

func apiReviewRows(reviews []store.Review) []apiReview {
	out := make([]apiReview, 0, len(reviews))
	for _, review := range reviews {
		out = append(out, apiReview{
			Reviewer:  review.Reviewer,
			Domain:    review.Domain,
			Verdict:   review.Verdict,
			Reason:    review.Reason,
			Commit:    review.Commit,
			CreatedAt: review.CreatedAt,
		})
	}
	return out
}

func apiRequireGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	apiError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	return false
}

func (s *Server) apiAuthorizeRead(w http.ResponseWriter, r *http.Request) bool {
	actor, err := s.apiActor(r)
	if err != nil {
		status := http.StatusUnauthorized
		if strings.HasPrefix(err.Error(), "forbidden:") {
			status = http.StatusForbidden
		}
		if strings.HasPrefix(err.Error(), "unimplemented:") {
			status = http.StatusNotImplemented
		}
		apiError(w, status, err)
		return false
	}
	if !s.apiRoleAllowed(actor.Role) {
		apiError(w, http.StatusForbidden, fmt.Errorf("forbidden: role %q is not allowed for read:api", actor.Role))
		return false
	}
	if actor.Role != "viewer" && actor.Role != "admin" {
		apiError(w, http.StatusForbidden, fmt.Errorf("forbidden: role %q cannot execute read:api", actor.Role))
		return false
	}
	return true
}

func (s *Server) apiActor(r *http.Request) (apiActor, error) {
	switch s.serverIdentityMode() {
	case "no_edge_local":
		return apiActor{Subject: "local", Role: "viewer", Source: "no_edge_local"}, nil
	case "api_token":
		return s.apiTokenActor(r)
	case "trusted_proxy_read_only":
		return s.trustedProxyActor(r)
	case "service_account", "mtls_service_account":
		return apiActor{}, fmt.Errorf("unimplemented: identity mode %q is configured as a fail-closed placeholder", s.serverIdentityMode())
	default:
		return apiActor{}, fmt.Errorf("unsupported identity mode")
	}
}

func (s *Server) apiTokenActor(r *http.Request) (apiActor, error) {
	want := strings.TrimSpace(os.Getenv(s.cfg.Server.APITokenEnv))
	if want == "" {
		return apiActor{}, errors.New("missing_identity: server API token environment variable is not set")
	}
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return apiActor{}, errors.New("missing_identity: bearer token is required")
	}
	got := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if got == "" {
		return apiActor{}, errors.New("missing_identity: bearer token is required")
	}
	if !constantTimeTokenEqual(got, want) {
		return apiActor{}, errors.New("invalid_identity: bearer token proof failed")
	}
	role := strings.TrimSpace(s.cfg.Server.APITokenRole)
	if role == "" {
		role = "viewer"
	}
	return apiActor{Subject: "api-token:" + tokenFingerprint(got), Role: role, Source: "api_token"}, nil
}

func (s *Server) trustedProxyActor(r *http.Request) (apiActor, error) {
	if !s.cfg.Server.TrustedProxyVerified {
		return apiActor{}, errors.New("forbidden: trusted proxy identity is advisory until verification is enabled")
	}
	proofHeader := strings.TrimSpace(s.cfg.Server.TrustedProxyProofHeader)
	identityHeader := strings.TrimSpace(s.cfg.Server.TrustedProxyIdentityHeader)
	if proofHeader == "" || identityHeader == "" {
		return apiActor{}, errors.New("missing_identity: trusted proxy proof configuration is incomplete")
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get(proofHeader)), "true") {
		return apiActor{}, errors.New("missing_identity: trusted proxy proof is required")
	}
	identity := strings.TrimSpace(r.Header.Get(identityHeader))
	if identity == "" {
		return apiActor{}, errors.New("missing_identity: trusted proxy identity is required")
	}
	if want := strings.TrimSpace(s.cfg.Server.TrustedProxyIssuer); want != "" {
		got := strings.TrimSpace(r.Header.Get(s.cfg.Server.TrustedProxyIssuerHeader))
		if got != want {
			return apiActor{}, errors.New("invalid_identity: trusted proxy issuer mismatch")
		}
	}
	if want := strings.TrimSpace(s.cfg.Server.TrustedProxyAudience); want != "" {
		got := strings.TrimSpace(r.Header.Get(s.cfg.Server.TrustedProxyAudienceHeader))
		if got != want {
			return apiActor{}, errors.New("invalid_identity: trusted proxy audience mismatch")
		}
	}
	return apiActor{Subject: redactIdentity(identity), Role: "viewer", Source: "trusted_proxy_read_only"}, nil
}

func (s *Server) serverIdentityMode() string {
	mode := strings.TrimSpace(s.cfg.Server.IdentityMode)
	if mode == "" {
		return "no_edge_local"
	}
	return mode
}

func (s *Server) apiRoleAllowed(role string) bool {
	for _, allowed := range s.cfg.Server.AllowedRoles {
		if allowed == role {
			return true
		}
	}
	return false
}

func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:12]
}

func constantTimeTokenEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func redactIdentity(identity string) string {
	if at := strings.LastIndex(identity, "@"); at >= 0 && at < len(identity)-1 {
		return "redacted@" + identity[at+1:]
	}
	if identity == "" {
		return ""
	}
	return "redacted"
}

func apiError(w http.ResponseWriter, status int, err error) {
	apiWriteJSON(w, status, map[string]string{"error": err.Error()})
}

func apiWriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
