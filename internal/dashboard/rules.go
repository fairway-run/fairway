package dashboard

import (
	"context"
	"sort"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/rules"
	"github.com/subashram/fairway/internal/store"
)

type TaskRuleStatus struct {
	ID               string
	Title            string
	Group            string
	Mode             string
	MatchStatus      string
	RiskFloor        string
	RequiredEvidence []string
	MissingEvidence  []string
	ReviewDomains    []string
	StopConditions   []string
	Reasons          []string
}

type ReportRuleSummary struct {
	SelectedRules      int `json:"selected_rules"`
	TasksWithRuleGaps  int `json:"tasks_with_rule_gaps"`
	BlockingRuleGaps   int `json:"blocking_rule_gaps"`
	AdvisoryRuleGaps   int `json:"advisory_rule_gaps"`
	NonApplicableRules int `json:"non_applicable_rules"`
}

func (s *Server) taskRuleStatuses(ctx context.Context, task store.Task, evidence []store.Evidence) []TaskRuleStatus {
	packs, err := rules.LoadConfigured(s.cfg, s.root, rules.LoadOptions{
		Root:            s.root,
		KnownDomains:    rules.ReviewDomainSet(s.cfg),
		KnownEvidence:   rules.ConfigGateEvidenceSet(s.cfg),
		IncludeDisabled: true,
	})
	if err != nil {
		return nil
	}
	return taskRuleStatusesFromPacks(s.cfg, packs, task, evidence)
}

func taskRuleStatusesFromPacks(cfg config.Config, packs []rules.Pack, task store.Task, evidence []store.Evidence) []TaskRuleStatus {
	evidenceTypes := evidenceTypeSet(evidence)
	matches := rules.MatchTask(cfg, packs, task)
	out := make([]TaskRuleStatus, 0, len(matches))
	for _, match := range matches {
		status := TaskRuleStatus{
			ID:               match.Rule.ID,
			Title:            match.Rule.Title,
			Group:            match.Rule.Group,
			Mode:             match.Rule.Mode,
			MatchStatus:      match.Status,
			RiskFloor:        match.Rule.RiskFloor,
			RequiredEvidence: append([]string(nil), match.Rule.RequiredEvidence...),
			ReviewDomains:    append([]string(nil), match.Rule.ReviewDomains...),
			StopConditions:   append([]string(nil), match.Rule.StopConditions...),
			Reasons:          append([]string(nil), match.Reasons...),
		}
		if status.Mode == "" {
			status.Mode = "advisory"
		}
		for _, required := range match.Rule.RequiredEvidence {
			if !evidenceTypes[required] {
				status.MissingEvidence = append(status.MissingEvidence, required)
			}
		}
		out = append(out, status)
	}
	return out
}

func reportRuleSummary(cfg config.Config, packs []rules.Pack, facts []reportTaskFacts) ReportRuleSummary {
	var summary ReportRuleSummary
	tasksWithRuleGaps := map[string]bool{}
	for _, fact := range facts {
		statuses := taskRuleStatusesFromPacks(cfg, packs, fact.Task, fact.Evidence)
		for _, status := range statuses {
			switch status.MatchStatus {
			case "selected":
				summary.SelectedRules++
				if len(status.MissingEvidence) > 0 {
					tasksWithRuleGaps[fact.Task.Definition.ID] = true
					if status.Mode == "blocking" {
						summary.BlockingRuleGaps++
					} else {
						summary.AdvisoryRuleGaps++
					}
				}
			case "non_applicable":
				summary.NonApplicableRules++
			}
		}
	}
	summary.TasksWithRuleGaps = len(tasksWithRuleGaps)
	return summary
}

func evidenceTypeSet(evidence []store.Evidence) map[string]bool {
	out := map[string]bool{}
	for _, ev := range evidence {
		if ev.ArtifactType != "" {
			out[ev.ArtifactType] = true
		}
	}
	return out
}

func selectedRuleStatuses(statuses []TaskRuleStatus) []TaskRuleStatus {
	var out []TaskRuleStatus
	for _, status := range statuses {
		if status.MatchStatus == "selected" {
			out = append(out, status)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
