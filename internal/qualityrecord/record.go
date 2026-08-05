package qualityrecord

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/store"
)

const Schema = "fairway.quality-record.v1"

type Record struct {
	Schema            string    `json:"schema"`
	Advisory          bool      `json:"advisory"`
	AuthorityBoundary string    `json:"authority_boundary"`
	TaskID            string    `json:"task_id"`
	GeneratedAt       string    `json:"generated_at"`
	Summary           Summary   `json:"summary"`
	Sections          []Section `json:"sections"`
}

type Summary struct {
	Present         int `json:"present"`
	Missing         int `json:"missing"`
	Unavailable     int `json:"unavailable"`
	Conflicting     int `json:"conflicting"`
	ExternallyOwned int `json:"externally_owned"`
}

type Section struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	State    string   `json:"state"`
	Boundary string   `json:"boundary,omitempty"`
	Facts    []Fact   `json:"facts,omitempty"`
	Sources  []Source `json:"sources"`
}

type Fact struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	SourceRef string `json:"source_ref"`
}

type Source struct {
	RecordType   string `json:"record_type"`
	Ref          string `json:"ref"`
	Availability string `json:"availability"`
}

func Build(ctx context.Context, s *store.Store, taskID string, now time.Time) (Record, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return Record{}, fmt.Errorf("task id is required")
	}
	task, transitions, evidence, handoffs, reviews, err := s.TaskDetail(ctx, taskID)
	if err != nil {
		return Record{}, err
	}
	decisions, err := s.TaskDecisions(ctx, taskID)
	if err != nil {
		return Record{}, err
	}
	commits, err := s.TaskCommits(ctx, taskID)
	if err != nil {
		return Record{}, err
	}
	outcomes, err := s.TaskOutcomes(ctx, taskID)
	if err != nil {
		return Record{}, err
	}
	friction, err := s.ControlFrictionSamples(ctx, taskID)
	if err != nil {
		return Record{}, err
	}
	allSessions, err := s.Sessions(ctx, true)
	if err != nil {
		return Record{}, err
	}
	allCheckpoints, err := s.Checkpoints(ctx, "", true)
	if err != nil {
		return Record{}, err
	}
	var sessions []store.Session
	for _, session := range allSessions {
		if session.TaskID == taskID {
			sessions = append(sessions, session)
		}
	}
	var checkpoints []store.Checkpoint
	for _, checkpoint := range allCheckpoints {
		if checkpoint.TaskID == taskID {
			checkpoints = append(checkpoints, checkpoint)
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	record := Record{
		Schema: Schema, Advisory: true, TaskID: taskID, GeneratedAt: now.UTC().Format(time.RFC3339Nano),
		AuthorityBoundary: "read-only projection of durable Fairway and cited external facts; cannot approve, waive, merge, deploy, release, accept risk, or mutate workflow",
	}
	record.Sections = []Section{
		intentSection(task),
		decisionSection(taskID, decisions),
		productionContextSection(taskID, sessions, checkpoints, commits),
		evidenceSection(taskID, evidence),
		verificationSection(taskID, evidence),
		judgmentSection(taskID, reviews),
		promotionSection(task, transitions, commits),
		outcomeSection(taskID, outcomes, friction),
		lessonSection(taskID, outcomes, handoffs),
	}
	for _, section := range record.Sections {
		switch section.State {
		case "present":
			record.Summary.Present++
		case "missing":
			record.Summary.Missing++
		case "unavailable":
			record.Summary.Unavailable++
		case "conflicting":
			record.Summary.Conflicting++
		case "externally_owned":
			record.Summary.ExternallyOwned++
		}
	}
	return record, nil
}

func intentSection(task store.Task) Section {
	ref := "task_definitions:" + task.Definition.ID
	section := Section{ID: "intent", Title: "Intent", State: "present", Sources: []Source{{RecordType: "task_definition", Ref: ref, Availability: "present"}}}
	for _, pair := range [][2]string{{"title", task.Definition.Title}, {"role", task.Definition.Role}, {"kind", task.Definition.Kind}, {"profile", task.Definition.Profile}, {"risk", task.Definition.RiskLevel}} {
		if strings.TrimSpace(pair[1]) != "" {
			section.Facts = append(section.Facts, Fact{Name: pair[0], Value: pair[1], SourceRef: ref})
		}
	}
	for i, check := range task.Definition.AcceptanceChecks {
		section.Facts = append(section.Facts, Fact{Name: fmt.Sprintf("acceptance_%d", i+1), Value: check, SourceRef: ref})
	}
	if strings.TrimSpace(task.Definition.Title) == "" || strings.TrimSpace(task.Definition.Role) == "" || len(task.Definition.AcceptanceChecks) == 0 {
		section.State = "missing"
		section.Boundary = "title, role, and at least one acceptance check are required for a complete intent projection"
	}
	return section
}

func decisionSection(taskID string, decisions []store.TaskDecision) Section {
	section := Section{ID: "decisions", Title: "Material Decisions", State: "missing", Sources: []Source{{RecordType: "task_decisions", Ref: "task_decisions:" + taskID, Availability: "missing"}}}
	if len(decisions) == 0 {
		return section
	}
	section.State = "present"
	section.Sources = nil
	for _, decision := range decisions {
		ref := fmt.Sprintf("task_decisions:%d", decision.ID)
		section.Sources = append(section.Sources, Source{RecordType: "task_decision", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: "decision", Value: decision.Decision + " -> " + decision.Chosen + " [" + decision.QualityState + "]", SourceRef: ref})
	}
	return section
}

func productionContextSection(taskID string, sessions []store.Session, checkpoints []store.Checkpoint, commits []store.TaskCommit) Section {
	section := Section{ID: "production_context", Title: "Production Context", State: "missing", Sources: []Source{{RecordType: "production_context", Ref: "task:" + taskID, Availability: "missing"}}}
	if len(sessions)+len(checkpoints)+len(commits) == 0 {
		return section
	}
	section.State, section.Sources = "present", nil
	for _, commit := range commits {
		ref := "task_commits:" + fmt.Sprint(commit.ID)
		section.Sources = append(section.Sources, Source{RecordType: "task_commit", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: "commit_" + commit.AssociationKind, Value: commit.CommitSHA, SourceRef: ref})
	}
	for _, session := range sessions {
		ref := "agent_sessions:" + session.ID
		section.Sources = append(section.Sources, Source{RecordType: "session", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: "session", Value: strings.Join([]string{session.Provider, session.SessionBackend, session.Status}, "/"), SourceRef: ref})
	}
	for _, checkpoint := range checkpoints {
		ref := fmt.Sprintf("task_checkpoints:%d", checkpoint.ID)
		section.Sources = append(section.Sources, Source{RecordType: "checkpoint", Ref: ref, Availability: "present"})
	}
	return section
}

func evidenceSection(taskID string, evidence []store.Evidence) Section {
	section := Section{ID: "evidence", Title: "Collected Evidence", State: "missing", Sources: []Source{{RecordType: "task_evidence", Ref: "task_evidence:" + taskID, Availability: "missing"}}}
	if len(evidence) == 0 {
		return section
	}
	section.State, section.Sources = "present", nil
	for i, item := range evidence {
		ref := evidenceRef(taskID, i, item)
		section.Sources = append(section.Sources, Source{RecordType: "task_evidence", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: item.Result, Value: strings.TrimSpace(item.CommandText), SourceRef: ref})
	}
	return section
}

func verificationSection(taskID string, evidence []store.Evidence) Section {
	section := Section{ID: "verification", Title: "Automatic Verification", State: "missing", Boundary: "recorded execution is evidence; verifier qualification remains a separate capability", Sources: []Source{{RecordType: "task_evidence", Ref: "task_evidence:" + taskID, Availability: "missing"}, {RecordType: "verifier_registry", Ref: "external:verifier-qualification", Availability: "externally_owned"}}}
	if len(evidence) == 0 {
		return section
	}
	section.State = "present"
	section.Sources = []Source{{RecordType: "verifier_registry", Ref: "external:verifier-qualification", Availability: "externally_owned"}}
	states := map[string]map[string]bool{}
	for i, item := range evidence {
		key := strings.TrimSpace(item.CommandText) + "\x00" + strings.TrimSpace(item.ArtifactType)
		if states[key] == nil {
			states[key] = map[string]bool{}
		}
		states[key][item.Result] = true
		ref := evidenceRef(taskID, i, item)
		section.Sources = append(section.Sources, Source{RecordType: "task_evidence", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: "verification", Value: item.Result + ": " + item.CommandText, SourceRef: ref})
	}
	for _, values := range states {
		positive := values["pass"]
		negative := values["fail"] || values["blocked"] || values["partial"]
		if positive && negative {
			section.State = "conflicting"
			section.Boundary = "the same verification identity has both passing and non-passing records; chronology and supersession require human interpretation"
			break
		}
	}
	return section
}

func judgmentSection(taskID string, reviews []store.Review) Section {
	section := Section{ID: "judgment", Title: "Human Judgment", State: "missing", Boundary: "reviewer competence, organizational authority, and authenticated identity remain externally owned", Sources: []Source{{RecordType: "task_reviews", Ref: "task_reviews:" + taskID, Availability: "missing"}, {RecordType: "reviewer_authority", Ref: "external:reviewer-authority", Availability: "externally_owned"}}}
	if len(reviews) == 0 {
		return section
	}
	section.State = "present"
	section.Sources = []Source{{RecordType: "reviewer_authority", Ref: "external:reviewer-authority", Availability: "externally_owned"}}
	for i, review := range reviews {
		ref := fmt.Sprintf("task_reviews:%s:%d", taskID, i+1)
		section.Sources = append(section.Sources, Source{RecordType: "task_review", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: review.Domain, Value: review.Verdict + " by " + review.Reviewer, SourceRef: ref})
	}
	return section
}

func promotionSection(task store.Task, transitions []store.Transition, commits []store.TaskCommit) Section {
	ref := "task_state:" + task.Definition.ID
	section := Section{ID: "promotion", Title: "Promotion Decision", State: "externally_owned", Boundary: "Fairway records readiness and task completion; Git, CI/CD, deployment systems, and accountable people retain promotion authority", Sources: []Source{{RecordType: "task_state", Ref: ref, Availability: "present"}, {RecordType: "promotion_authority", Ref: "external:promotion-authority", Availability: "externally_owned"}}, Facts: []Fact{{Name: "status", Value: task.Status, SourceRef: ref}}}
	if task.CommitSHA != "" {
		section.Facts = append(section.Facts, Fact{Name: "canonical_commit", Value: task.CommitSHA, SourceRef: ref})
	}
	for _, transition := range transitions {
		if transition.ToStatus == "done" {
			transitionRef := fmt.Sprintf("task_state_history:%d", transition.ID)
			section.Sources = append(section.Sources, Source{RecordType: "task_transition", Ref: transitionRef, Availability: "present"})
		}
	}
	_ = commits
	return section
}

func outcomeSection(taskID string, outcomes []store.TaskOutcome, friction []store.ControlFrictionSample) Section {
	section := Section{ID: "outcomes", Title: "Operational Outcomes", State: "unavailable", Boundary: "absence of a recorded outcome is not evidence that no outcome occurred", Sources: []Source{{RecordType: "task_outcomes", Ref: "task_outcomes:" + taskID, Availability: "unavailable"}}}
	if len(outcomes)+len(friction) == 0 {
		return section
	}
	section.State, section.Sources = "present", nil
	for _, outcome := range outcomes {
		ref := fmt.Sprintf("task_outcomes:%d", outcome.ID)
		section.Sources = append(section.Sources, Source{RecordType: "task_outcome", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: outcome.Kind, Value: first(outcome.SourceRef, outcome.RelatedTaskID, fmt.Sprint(outcome.TransitionID)), SourceRef: ref})
	}
	for _, sample := range friction {
		ref := fmt.Sprintf("control_friction_samples:%d", sample.ID)
		section.Sources = append(section.Sources, Source{RecordType: "control_friction", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: "friction_" + sample.Status, Value: sample.ControlID, SourceRef: ref})
	}
	return section
}

func lessonSection(taskID string, outcomes []store.TaskOutcome, handoffs []store.Handoff) Section {
	section := Section{ID: "lessons", Title: "Lessons And Controlled Improvement", State: "missing", Boundary: "lessons require an explicit corrective/superseding link or durable handoff; generated interpretation is not promoted into knowledge", Sources: []Source{{RecordType: "controlled_improvement", Ref: "task:" + taskID, Availability: "missing"}}}
	for _, outcome := range outcomes {
		if outcome.Kind != "corrective" && outcome.Kind != "superseding_task" {
			continue
		}
		if section.State == "missing" {
			section.State, section.Sources = "present", nil
		}
		ref := fmt.Sprintf("task_outcomes:%d", outcome.ID)
		section.Sources = append(section.Sources, Source{RecordType: "task_outcome", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: outcome.Kind, Value: outcome.RelatedTaskID, SourceRef: ref})
	}
	for _, handoff := range handoffs {
		if !strings.Contains(strings.ToLower(handoff.Payload), "lesson") {
			continue
		}
		if section.State == "missing" {
			section.State, section.Sources = "present", nil
		}
		ref := fmt.Sprintf("task_handoffs:%d", handoff.ID)
		section.Sources = append(section.Sources, Source{RecordType: "task_handoff", Ref: ref, Availability: "present"})
		section.Facts = append(section.Facts, Fact{Name: "lesson_handoff", Value: handoff.ToRole, SourceRef: ref})
	}
	return section
}

func evidenceRef(taskID string, index int, item store.Evidence) string {
	return fmt.Sprintf("task_evidence:%s:%s:%d", taskID, item.CreatedAt, index+1)
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && value != "0" {
			return value
		}
	}
	return "recorded"
}
