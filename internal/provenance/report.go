package provenance

import (
	"context"
	"fmt"
	"net/url"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

type Options struct {
	TaskID      string
	Since       time.Duration
	GeneratedAt time.Time
}

type Report struct {
	Schema      string       `json:"schema"`
	GeneratedAt string       `json:"generated_at"`
	Project     Project      `json:"project"`
	Scope       Scope        `json:"scope"`
	Tasks       []TaskRecord `json:"tasks"`
	Privacy     Privacy      `json:"privacy"`
	Warnings    []string     `json:"warnings,omitempty"`
}

type Project struct {
	Name       string `json:"name"`
	DBPath     string `json:"db_path,omitempty"`
	ConfigPath string `json:"config_path,omitempty"`
}

type Scope struct {
	TaskID string `json:"task_id,omitempty"`
	Since  string `json:"since,omitempty"`
	Until  string `json:"until,omitempty"`
}

type Privacy struct {
	RawPromptsIncluded       bool     `json:"raw_prompts_included"`
	TranscriptsIncluded      bool     `json:"transcripts_included"`
	ToolBodiesIncluded       bool     `json:"tool_bodies_included"`
	GeneratedContentIncluded bool     `json:"generated_content_included"`
	RedactionApplied         bool     `json:"redaction_applied"`
	ExcludedContent          []string `json:"excluded_content"`
}

type TaskRecord struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Status          string            `json:"status"`
	Role            string            `json:"role"`
	Owner           string            `json:"owner,omitempty"`
	Profile         string            `json:"profile,omitempty"`
	RiskLevel       string            `json:"risk_level,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Acceptance      []string          `json:"acceptance,omitempty"`
	SourcePaths     []string          `json:"source_paths,omitempty"`
	TargetPaths     []string          `json:"target_paths,omitempty"`
	CommitRefs      []string          `json:"commit_refs,omitempty"`
	EvidenceRefs    []EvidenceRef     `json:"evidence_refs,omitempty"`
	ReviewRefs      []ReviewRef       `json:"review_refs,omitempty"`
	CheckpointRefs  []CheckpointRef   `json:"checkpoint_refs,omitempty"`
	SessionRefs     []SessionRef      `json:"session_refs,omitempty"`
	UsageRefs       []UsageRef        `json:"usage_refs,omitempty"`
	HandoffRefs     []HandoffRef      `json:"handoff_refs,omitempty"`
	ReleaseRefs     []string          `json:"release_refs,omitempty"`
	ValidationGates []string          `json:"validation_gates,omitempty"`
	PrivacyWarnings []string          `json:"privacy_warnings,omitempty"`
	ProvenanceFacts map[string]string `json:"provenance_facts,omitempty"`
}

type EvidenceRef struct {
	Ref          string `json:"ref"`
	Result       string `json:"result,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	ArtifactType string `json:"artifact_type,omitempty"`
	CommandText  string `json:"command_text,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

type ReviewRef struct {
	Ref       string `json:"ref"`
	Domain    string `json:"domain,omitempty"`
	Reviewer  string `json:"reviewer"`
	Verdict   string `json:"verdict"`
	Commit    string `json:"commit,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type CheckpointRef struct {
	ID           int64  `json:"id"`
	State        string `json:"state"`
	Owner        string `json:"owner,omitempty"`
	Summary      string `json:"summary,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

type SessionRef struct {
	ID        string `json:"id"`
	Provider  string `json:"provider,omitempty"`
	Backend   string `json:"backend,omitempty"`
	Role      string `json:"role,omitempty"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at,omitempty"`
	EndedAt   string `json:"ended_at,omitempty"`
}

type UsageRef struct {
	ID          int64  `json:"id"`
	Provider    string `json:"provider"`
	SessionID   string `json:"session_id,omitempty"`
	Role        string `json:"role,omitempty"`
	Phase       string `json:"phase,omitempty"`
	Source      string `json:"source"`
	Confidence  string `json:"confidence"`
	TotalTokens *int   `json:"total_tokens,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type HandoffRef struct {
	ID        int64  `json:"id"`
	FromRole  string `json:"from_role,omitempty"`
	ToRole    string `json:"to_role"`
	CreatedAt string `json:"created_at,omitempty"`
}

type PromptPacket struct {
	Schema           string            `json:"schema"`
	GeneratedAt      string            `json:"generated_at"`
	TaskID           string            `json:"task_id"`
	Objective        string            `json:"objective"`
	Scope            []string          `json:"scope,omitempty"`
	Acceptance       []string          `json:"acceptance,omitempty"`
	SourceFacts      []string          `json:"source_facts"`
	ForbiddenActions []string          `json:"forbidden_actions"`
	ValidationGates  []string          `json:"validation_gates,omitempty"`
	EvidenceRefs     []EvidenceRef     `json:"evidence_refs,omitempty"`
	ReviewRefs       []ReviewRef       `json:"review_refs,omitempty"`
	Privacy          Privacy           `json:"privacy"`
	Warnings         []string          `json:"warnings,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

var secretPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`), `${1}<redacted>`},
	{regexp.MustCompile(`(?i)((?:access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|ssh[_-]?private[_-]?key|api[_-]?key|token|secret|password|cookie|set-cookie)=)([^&\s"']+)`), `${1}<redacted>`},
	{regexp.MustCompile(`(?i)("(?:access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|ssh[_-]?private[_-]?key|api[_-]?key|token|secret|password|cookie|set-cookie)"\s*:\s*")([^"]+)(")`), `${1}<redacted>${3}`},
	{regexp.MustCompile(`(?i)((?:authorization|cookie|set-cookie):\s*)([^\r\n]+)`), `${1}<redacted>`},
}

var (
	windowsAbsolutePath = regexp.MustCompile(`(?i)^(?:[A-Z]:[\\/]|\\\\[?A-Z0-9._$-])`)
	absoluteTextPath    = regexp.MustCompile(`(^|[^A-Za-z0-9._~+@%/\\-])((?:/[A-Za-z0-9._~+@%,-][^\s"'<>]*)|(?:[A-Za-z]:[\\/][^\s"'<>]*)|(?:\\\\[?A-Za-z0-9._$-][^\s"'<>]*))`)
	sensitiveQueryKey   = regexp.MustCompile(`(?i)^(?:access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|ssh[_-]?private[_-]?key|api[_-]?key|token|secret|password|authorization|cookie|set-cookie)$`)
)

const redactedLocalPath = "<redacted-local-path>"

func Build(ctx context.Context, cfg config.Config, repositoryRoot, configPath string, s *store.Store, opts Options) (Report, error) {
	generated := opts.GeneratedAt
	if generated.IsZero() {
		generated = time.Now().UTC()
	}
	var pathWarnings []string
	report := Report{
		Schema:      "fairway.provenance.v1",
		GeneratedAt: generated.Format(time.RFC3339),
		Project: Project{
			Name:       cfg.Fairway.ProjectName,
			DBPath:     publicProvenancePath(repositoryRoot, cfg.Fairway.DBPath, &pathWarnings, "project:db_path"),
			ConfigPath: publicProvenancePath(repositoryRoot, configPath, &pathWarnings, "project:config_path"),
		},
		Privacy: defaultPrivacy(),
	}
	if len(pathWarnings) > 0 {
		report.Privacy.RedactionApplied = true
		report.Warnings = append(report.Warnings, pathWarnings...)
	}
	if opts.TaskID != "" {
		report.Scope.TaskID = opts.TaskID
	}
	var sinceCutoff time.Time
	if opts.Since > 0 {
		sinceCutoff = generated.Add(-opts.Since)
		report.Scope.Since = sinceCutoff.Format(time.RFC3339)
		report.Scope.Until = generated.Format(time.RFC3339)
	}

	var tasks []store.Task
	if opts.TaskID != "" {
		task, _, _, _, _, err := s.TaskDetail(ctx, opts.TaskID)
		if err != nil {
			return Report{}, err
		}
		tasks = []store.Task{task}
	} else {
		all, err := s.AllTasks(ctx)
		if err != nil {
			return Report{}, err
		}
		for _, task := range all {
			if sinceCutoff.IsZero() || taskInWindow(task, sinceCutoff) {
				tasks = append(tasks, task)
			}
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Definition.ID < tasks[j].Definition.ID })

	checkpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return Report{}, err
	}
	sessions, err := s.Sessions(ctx, true)
	if err != nil {
		return Report{}, err
	}
	for _, task := range tasks {
		record, warnings, err := buildTaskRecord(ctx, s, task, checkpoints, sessions, repositoryRoot)
		if err != nil {
			return Report{}, err
		}
		if len(warnings) > 0 {
			report.Privacy.RedactionApplied = true
			report.Warnings = append(report.Warnings, warnings...)
		}
		report.Tasks = append(report.Tasks, record)
	}
	sort.Strings(report.Warnings)
	return report, nil
}

func BuildPromptPacket(ctx context.Context, cfg config.Config, repositoryRoot, configPath string, s *store.Store, taskID string, generated time.Time) (PromptPacket, error) {
	if generated.IsZero() {
		generated = time.Now().UTC()
	}
	report, err := Build(ctx, cfg, repositoryRoot, configPath, s, Options{TaskID: taskID, GeneratedAt: generated})
	if err != nil {
		return PromptPacket{}, err
	}
	if len(report.Tasks) == 0 {
		return PromptPacket{}, store.ErrNotFound
	}
	task := report.Tasks[0]
	packet := PromptPacket{
		Schema:      "fairway.prompt-packet.v1",
		GeneratedAt: report.GeneratedAt,
		TaskID:      task.ID,
		Objective:   task.Title,
		Scope:       append([]string{}, task.TargetPaths...),
		Acceptance:  append([]string{}, task.Acceptance...),
		SourceFacts: []string{
			fmt.Sprintf("task:%s status=%s role=%s risk=%s", task.ID, task.Status, task.Role, firstNonEmpty(task.RiskLevel, "unknown")),
			fmt.Sprintf("project:%s config=%s db=%s", report.Project.Name, report.Project.ConfigPath, report.Project.DBPath),
		},
		ForbiddenActions: []string{
			"do not include raw secrets, credentials, or provider auth state",
			"do not include raw prompt bodies, private transcripts, raw tool bodies, or generated-content dumps",
			"do not approve reviews, merge, push, deploy, release, or mutate dashboard state from this packet",
		},
		ValidationGates: task.ValidationGates,
		EvidenceRefs:    task.EvidenceRefs,
		ReviewRefs:      task.ReviewRefs,
		Privacy:         report.Privacy,
		Warnings:        report.Warnings,
		Metadata: map[string]string{
			"config_path": report.Project.ConfigPath,
			"db_path":     report.Project.DBPath,
			"schema":      report.Schema,
		},
	}
	for _, ref := range task.CheckpointRefs {
		packet.SourceFacts = append(packet.SourceFacts, fmt.Sprintf("checkpoint:%d state=%s", ref.ID, ref.State))
	}
	for _, ref := range task.ReviewRefs {
		packet.SourceFacts = append(packet.SourceFacts, fmt.Sprintf("%s verdict=%s domain=%s", ref.Ref, ref.Verdict, firstNonEmpty(ref.Domain, "unspecified")))
	}
	return packet, nil
}

func buildTaskRecord(ctx context.Context, s *store.Store, task store.Task, checkpoints []store.Checkpoint, sessions []store.Session, repositoryRoot string) (TaskRecord, []string, error) {
	detailTask, _, evidence, handoffs, reviews, err := s.TaskDetail(ctx, task.Definition.ID)
	if err != nil {
		return TaskRecord{}, nil, err
	}
	usage, err := s.ProviderUsageForTask(ctx, task.Definition.ID)
	if err != nil {
		return TaskRecord{}, nil, err
	}
	var warnings []string
	record := TaskRecord{
		ID:              detailTask.Definition.ID,
		Title:           detailTask.Definition.Title,
		Status:          detailTask.Status,
		Role:            detailTask.Definition.Role,
		Owner:           detailTask.Owner,
		Profile:         detailTask.Definition.Profile,
		RiskLevel:       detailTask.Definition.RiskLevel,
		Tags:            append([]string{}, detailTask.Definition.Tags...),
		Acceptance:      redactPublicStrings(detailTask.Definition.AcceptanceChecks, repositoryRoot, &warnings, task.Definition.ID, "acceptance"),
		SourcePaths:     publicProvenancePaths(detailTask.Definition.SourcePaths, repositoryRoot, &warnings, task.Definition.ID, "source_paths"),
		TargetPaths:     publicProvenancePaths(detailTask.Definition.TargetPaths, repositoryRoot, &warnings, task.Definition.ID, "target_paths"),
		ProvenanceFacts: map[string]string{},
	}
	if detailTask.CommitSHA != "" {
		record.CommitRefs = []string{detailTask.CommitSHA}
	}
	if len(evidence) == 0 {
		warnings = append(warnings, fmt.Sprintf("task=%s has no evidence refs", task.Definition.ID))
	}
	for i, ev := range evidence {
		ref := fmt.Sprintf("evidence:%s:%d", task.Definition.ID, i+1)
		command := redactPublicText(ev.CommandText, repositoryRoot, &warnings, task.Definition.ID, ref+":command")
		artifact := publicProvenancePath(repositoryRoot, redactString(ev.ArtifactPath, &warnings, task.Definition.ID, ref+":artifact"), &warnings, ref+":artifact")
		record.EvidenceRefs = append(record.EvidenceRefs, EvidenceRef{
			Ref:          ref,
			Result:       ev.Result,
			ArtifactPath: artifact,
			ArtifactType: ev.ArtifactType,
			CommandText:  command,
			CreatedAt:    ev.CreatedAt,
		})
		if isValidationEvidence(ev) {
			record.ValidationGates = append(record.ValidationGates, command)
		}
		if releaseRef, ok := explicitReleaseRef(ev, artifact, command); ok {
			record.ReleaseRefs = appendUnique(record.ReleaseRefs, releaseRef)
		}
	}
	for i, review := range reviews {
		record.ReviewRefs = append(record.ReviewRefs, ReviewRef{
			Ref:       fmt.Sprintf("review:%s:%d", task.Definition.ID, i+1),
			Domain:    review.Domain,
			Reviewer:  review.Reviewer,
			Verdict:   review.Verdict,
			Commit:    review.Commit,
			CreatedAt: review.CreatedAt,
		})
	}
	for _, cp := range checkpoints {
		if cp.TaskID != task.Definition.ID {
			continue
		}
		record.CheckpointRefs = append(record.CheckpointRefs, CheckpointRef{
			ID:           cp.ID,
			State:        cp.State,
			Owner:        cp.Owner,
			Summary:      redactPublicText(cp.Summary, repositoryRoot, &warnings, task.Definition.ID, fmt.Sprintf("checkpoint:%d:summary", cp.ID)),
			ArtifactPath: publicProvenancePath(repositoryRoot, redactString(cp.ArtifactPath, &warnings, task.Definition.ID, fmt.Sprintf("checkpoint:%d:artifact", cp.ID)), &warnings, fmt.Sprintf("checkpoint:%d:artifact", cp.ID)),
			CreatedAt:    cp.CreatedAt,
		})
	}
	for _, session := range sessions {
		if session.TaskID != task.Definition.ID {
			continue
		}
		record.SessionRefs = append(record.SessionRefs, SessionRef{
			ID:        session.ID,
			Provider:  session.Provider,
			Backend:   session.SessionBackend,
			Role:      session.Role,
			Status:    session.Status,
			StartedAt: session.StartedAt,
			EndedAt:   session.EndedAt,
		})
	}
	for _, event := range usage {
		record.UsageRefs = append(record.UsageRefs, UsageRef{
			ID:          event.ID,
			Provider:    event.Provider,
			SessionID:   event.SessionID,
			Role:        event.Role,
			Phase:       event.Phase,
			Source:      event.Source,
			Confidence:  event.Confidence,
			TotalTokens: event.TotalTokens,
			CreatedAt:   event.CreatedAt,
		})
	}
	for _, handoff := range handoffs {
		record.HandoffRefs = append(record.HandoffRefs, HandoffRef{ID: handoff.ID, FromRole: handoff.FromRole, ToRole: handoff.ToRole, CreatedAt: handoff.CreatedAt})
	}
	record.PrivacyWarnings = append(record.PrivacyWarnings, warnings...)
	record.ProvenanceFacts["evidence_count"] = fmt.Sprintf("%d", len(record.EvidenceRefs))
	record.ProvenanceFacts["review_count"] = fmt.Sprintf("%d", len(record.ReviewRefs))
	record.ProvenanceFacts["checkpoint_count"] = fmt.Sprintf("%d", len(record.CheckpointRefs))
	record.ProvenanceFacts["session_count"] = fmt.Sprintf("%d", len(record.SessionRefs))
	record.ProvenanceFacts["usage_count"] = fmt.Sprintf("%d", len(record.UsageRefs))
	return record, warnings, nil
}

func taskInWindow(task store.Task, since time.Time) bool {
	for _, value := range []string{task.CompletedAt, task.UpdatedAt} {
		if value == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			t, err = time.Parse(time.RFC3339, value)
		}
		if err == nil && !t.Before(since) {
			return true
		}
	}
	return false
}

func defaultPrivacy() Privacy {
	return Privacy{
		RawPromptsIncluded:       false,
		TranscriptsIncluded:      false,
		ToolBodiesIncluded:       false,
		GeneratedContentIncluded: false,
		ExcludedContent: []string{
			"raw secrets",
			"provider auth state",
			"raw prompt bodies by default",
			"private transcripts",
			"raw tool bodies",
			"generated-content dumps",
		},
	}
}

func redactStrings(values []string, warnings *[]string, taskID, field string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, redactString(value, warnings, taskID, field))
	}
	return out
}

func redactPublicStrings(values []string, repositoryRoot string, warnings *[]string, taskID, field string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, redactPublicText(value, repositoryRoot, warnings, taskID, field))
	}
	return out
}

func publicProvenancePaths(values []string, repositoryRoot string, warnings *[]string, taskID, field string) []string {
	out := make([]string, 0, len(values))
	for i, value := range values {
		out = append(out, publicProvenancePath(repositoryRoot, value, warnings, fmt.Sprintf("task=%s field=%s:%d", taskID, field, i+1)))
	}
	return out
}

func publicProvenancePath(repositoryRoot, value string, warnings *[]string, field string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.Contains(value, "://") {
		if strings.HasPrefix(lower, "file://") {
			appendPathWarning(warnings, field)
			return redactedLocalPath
		}
		return publicProvenanceURL(value, repositoryRoot, warnings, field)
	}

	valueWindows := windowsAbsolutePath.MatchString(value)
	rootWindows := windowsAbsolutePath.MatchString(repositoryRoot)
	if !valueWindows && filepath.IsAbs(value) {
		value = canonicalPublicPath(value)
	}
	if !rootWindows && filepath.IsAbs(repositoryRoot) {
		repositoryRoot = canonicalPublicPath(repositoryRoot)
	}
	normalized := pathpkg.Clean(strings.ReplaceAll(value, `\`, "/"))
	root := pathpkg.Clean(strings.ReplaceAll(strings.TrimSpace(repositoryRoot), `\`, "/"))
	valueAbsolute := strings.HasPrefix(normalized, "/") || valueWindows
	if !valueAbsolute {
		if normalized == ".." || strings.HasPrefix(normalized, "../") {
			appendPathWarning(warnings, field)
			return redactedLocalPath
		}
		return normalized
	}

	if root != "" && root != "." && (valueWindows == rootWindows) {
		candidateCmp, rootCmp := normalized, root
		if valueWindows {
			candidateCmp = strings.ToLower(candidateCmp)
			rootCmp = strings.ToLower(rootCmp)
		}
		if candidateCmp == rootCmp {
			return "."
		}
		if strings.HasPrefix(candidateCmp, strings.TrimSuffix(rootCmp, "/")+"/") {
			return normalized[len(strings.TrimSuffix(root, "/"))+1:]
		}
	}
	appendPathWarning(warnings, field)
	return redactedLocalPath
}

func publicProvenanceURL(value, repositoryRoot string, warnings *[]string, field string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		appendPathWarning(warnings, field)
		return redactedLocalPath
	}
	if parsed.User != nil {
		parsed.User = nil
		appendPathWarning(warnings, field+":userinfo")
	}
	if parsed.RawQuery != "" {
		query, err := url.ParseQuery(parsed.RawQuery)
		if err != nil {
			parsed.RawQuery = ""
			appendPathWarning(warnings, field+":query")
		} else {
			for key, values := range query {
				if sensitiveQueryKey.MatchString(strings.TrimSpace(key)) {
					for i := range values {
						values[i] = "<redacted>"
					}
					query[key] = values
					appendPathWarning(warnings, field+":query:"+key)
					continue
				}
				for i, queryValue := range values {
					values[i] = redactPublicText(queryValue, repositoryRoot, warnings, "public-provenance", field+":query:"+key)
				}
				query[key] = values
			}
			parsed.RawQuery = query.Encode()
		}
	}
	if parsed.Fragment != "" {
		parsed.Fragment = redactPublicText(parsed.Fragment, repositoryRoot, warnings, "public-provenance", field+":fragment")
		parsed.RawFragment = ""
	}
	return parsed.String()
}

func canonicalPublicPath(value string) string {
	original := filepath.Clean(value)
	candidate := original
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return filepath.Clean(resolved)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return original
		}
		tail = append(tail, filepath.Base(candidate))
		candidate = parent
	}
}

func redactPublicText(value, repositoryRoot string, warnings *[]string, taskID, field string) string {
	redacted := redactString(value, warnings, taskID, field)
	root := strings.TrimRight(strings.TrimSpace(repositoryRoot), `/\`)
	if root != "" {
		for _, candidate := range []string{root, strings.ReplaceAll(root, `/`, `\`)} {
			var replaced bool
			redacted, replaced = replacePathRoot(redacted, candidate)
			if replaced {
				appendPathWarning(warnings, fmt.Sprintf("task=%s field=%s", taskID, field))
			}
		}
	}
	cleaned := absoluteTextPath.ReplaceAllString(redacted, `${1}`+redactedLocalPath)
	if cleaned != redacted {
		appendPathWarning(warnings, fmt.Sprintf("task=%s field=%s", taskID, field))
	}
	return cleaned
}

func replacePathRoot(value, root string) (string, bool) {
	if root == "" {
		return value, false
	}
	searchValue, searchRoot := value, root
	if windowsAbsolutePath.MatchString(root) {
		searchValue = strings.ToLower(value)
		searchRoot = strings.ToLower(root)
	}
	var out strings.Builder
	replaced := false
	for offset := 0; offset < len(value); {
		relative := strings.Index(searchValue[offset:], searchRoot)
		if relative < 0 {
			out.WriteString(value[offset:])
			break
		}
		start := offset + relative
		end := start + len(root)
		beforeOK := start == 0 || isPathTextBoundary(value[start-1])
		afterOK := end == len(value) || value[end] == '/' || value[end] == '\\' || isPathTextBoundary(value[end])
		if beforeOK && afterOK {
			out.WriteString(value[offset:start])
			out.WriteByte('.')
			offset = end
			replaced = true
			continue
		}
		out.WriteString(value[offset : start+1])
		offset = start + 1
	}
	return out.String(), replaced
}

func isPathTextBoundary(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n', '"', '\'', '=', '(', ')', '[', ']', '{', '}', ':', ',', ';':
		return true
	default:
		return false
	}
}

func appendPathWarning(warnings *[]string, field string) {
	if warnings == nil {
		return
	}
	*warnings = appendUnique(*warnings, "redacted local path "+field)
}

func redactString(value string, warnings *[]string, taskID, field string) string {
	redacted := value
	for _, pattern := range secretPatterns {
		redacted = pattern.re.ReplaceAllString(redacted, pattern.repl)
	}
	if redacted != value && warnings != nil {
		*warnings = appendUnique(*warnings, fmt.Sprintf("redacted sensitive-looking value task=%s field=%s", taskID, field))
	}
	return redacted
}

func isValidationEvidence(ev store.Evidence) bool {
	text := strings.ToLower(ev.CommandText + " " + ev.ArtifactType + " " + ev.Notes)
	for _, marker := range []string{"go test", "go vet", "config validate", "workflow check", "diff --check", "release verify", "npm run build"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func explicitReleaseRef(ev store.Evidence, artifactPath, commandText string) (string, bool) {
	artifactType := strings.ToLower(strings.TrimSpace(ev.ArtifactType))
	switch artifactType {
	case "release-run", "release-verify", "release-verification", "release-asset", "release-asset_utility", "release-attestation":
		return firstNonEmpty(artifactPath, commandText), true
	}
	command := strings.ToLower(strings.TrimSpace(ev.CommandText))
	for _, marker := range []string{"fairway release verify", "fairway packet release-run"} {
		if strings.HasPrefix(command, marker) || strings.Contains(command, " "+marker) {
			return firstNonEmpty(artifactPath, commandText), true
		}
	}
	return "", false
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
