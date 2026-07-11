package reviewstate

import (
	"sort"
	"strings"

	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/store"
)

type RoutingCoverageReport struct {
	OK      bool                   `json:"ok"`
	Scope   string                 `json:"scope"`
	Rows    []RoutingCoverageRow   `json:"rows"`
	Summary RoutingCoverageSummary `json:"summary"`
}

type RoutingCoverageSummary struct {
	Domains           int `json:"domains"`
	ConfiguredRoles   int `json:"configured_roles"`
	ConfiguredAliases int `json:"configured_aliases"`
	ReviewRoutes      int `json:"review_routes"`
	ProviderTargets   int `json:"provider_targets"`
	MissingMappings   int `json:"missing_mappings"`
}

type RoutingCoverageRow struct {
	Domain          string                  `json:"domain"`
	TaskIDs         []string                `json:"task_ids"`
	Routable        bool                    `json:"routable"`
	Resolution      string                  `json:"resolution"`
	ConfiguredRole  string                  `json:"configured_role,omitempty"`
	RoleProvider    string                  `json:"role_provider,omitempty"`
	AliasRole       string                  `json:"alias_role,omitempty"`
	AliasProvider   string                  `json:"alias_provider,omitempty"`
	ReviewRoutes    []string                `json:"review_routes,omitempty"`
	ProviderTargets []config.ProviderTarget `json:"provider_targets,omitempty"`
	Action          string                  `json:"action"`
	Reason          string                  `json:"reason"`
}

// ReviewRoutingCoverage reports how every review domain in the active backlog
// resolves without mutating review, wait, or notification state.
func ReviewRoutingCoverage(tasks []store.Task, opts ReviewWaitOptions) RoutingCoverageReport {
	taskIDsByDomain := map[string]map[string]bool{}
	for _, task := range tasks {
		if isTerminalStatus(task.Status, opts.Terminal) {
			continue
		}
		for _, domain := range normalizedUnique(task.Definition.ReviewDomains) {
			if taskIDsByDomain[domain] == nil {
				taskIDsByDomain[domain] = map[string]bool{}
			}
			taskIDsByDomain[domain][task.Definition.ID] = true
		}
	}

	domains := make([]string, 0, len(taskIDsByDomain))
	for domain := range taskIDsByDomain {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	report := RoutingCoverageReport{OK: true, Scope: "non_terminal_tasks"}
	for _, domain := range domains {
		row := RoutingCoverageRow{Domain: domain, Action: "none"}
		for taskID := range taskIDsByDomain[domain] {
			row.TaskIDs = append(row.TaskIDs, taskID)
		}
		sort.Strings(row.TaskIDs)

		for _, role := range opts.Roles {
			if strings.TrimSpace(role.Name) == domain {
				row.ConfiguredRole = domain
				row.RoleProvider = strings.TrimSpace(role.Provider)
				break
			}
		}
		if aliasRole := strings.TrimSpace(opts.DomainAliases[domain]); aliasRole != "" {
			row.AliasRole = aliasRole
			for _, role := range opts.Roles {
				if strings.TrimSpace(role.Name) == aliasRole {
					row.AliasProvider = strings.TrimSpace(role.Provider)
					break
				}
			}
		}
		for _, route := range opts.ReviewRoutes {
			if strings.TrimSpace(route.Reviewer) == domain {
				row.ReviewRoutes = append(row.ReviewRoutes, strings.TrimSpace(route.Match))
			}
		}
		sort.Strings(row.ReviewRoutes)
		for _, target := range opts.ProviderTargets {
			if strings.TrimSpace(target.Domain) == domain {
				row.ProviderTargets = append(row.ProviderTargets, target)
			}
		}
		sort.SliceStable(row.ProviderTargets, func(i, j int) bool {
			if row.ProviderTargets[i].Provider != row.ProviderTargets[j].Provider {
				return row.ProviderTargets[i].Provider < row.ProviderTargets[j].Provider
			}
			return row.ProviderTargets[i].Target < row.ProviderTargets[j].Target
		})
		if row.ConfiguredRole != "" {
			report.Summary.ConfiguredRoles++
		}
		if row.AliasRole != "" {
			report.Summary.ConfiguredAliases++
		}
		if len(row.ReviewRoutes) > 0 {
			report.Summary.ReviewRoutes++
		}
		if len(row.ProviderTargets) > 0 {
			report.Summary.ProviderTargets++
		}

		switch {
		case len(row.ProviderTargets) > 0:
			row.Routable = true
			row.Resolution = "provider_target"
			row.Reason = "domain has an explicit provider notification target"
		case row.AliasRole != "":
			row.Routable = true
			row.Resolution = "configured_alias"
			row.Reason = "domain maps through [review_domain_aliases] to a configured reviewer role"
		case row.ConfiguredRole != "":
			row.Routable = true
			row.Resolution = "configured_role"
			row.Reason = "domain matches a configured reviewer role"
		case len(row.ReviewRoutes) > 0:
			row.Routable = true
			row.Resolution = "review_route"
			row.Reason = "domain is named by a configured review route"
		default:
			row.Resolution = "missing"
			row.Action = "configure_review_mapping"
			row.Reason = "add a matching [[roles]], [[review_routes]], or [[provider_targets]] entry before routing review"
			report.OK = false
			report.Summary.MissingMappings++
		}
		report.Rows = append(report.Rows, row)
	}
	report.Summary.Domains = len(report.Rows)
	return report
}
