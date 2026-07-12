# Workstream Profiles

Workstream profiles keep Fairway generic while making it useful for
architecture-aware coordination. A profile is a named operating shape for a kind
of engineering work. It defines task kinds, packet templates, review domains,
evidence expectations, and dashboard grouping without making Fairway a CI/CD
tool, scanner, docs portal, or agent runner.

## Product Framing

Fairway supports governed agentic engineering with a local-first coordination
control plane: tasks, ownership, evidence, reviews, handoffs, sessions,
readiness, and risk.

Profiles should make that control plane more precise. They should not execute
builds, tests, scans, deployments, or documentation hosting.

## Candidate Profiles

| Profile | Use |
|---|---|
| `platform-foundation` | Ownership maps, boundary guards, facades, vertical slices, frontend contracts, release evidence. |
| `release-readiness` | Environment/ring gates, approvals, risks, exceptions, expiry, evidence rollups. |
| `frontend-migration` | Page contracts, component ownership, route moves, visual evidence, handoffs. |
| `service-extraction` | Source/target paths, adapters, rollback plans, dependency and API evidence. |
| `sdk-readiness` | API stability, examples, docs, quickstarts, internal/public readiness decisions. |
| `security-hardening` | Guard reports, findings, mitigations, residual risk, accepted debt. |

## Profile Contents

A profile may define:

- task kinds,
- task taxonomy conventions,
- packet templates,
- review routes or review domains,
- evidence expectations,
- dashboard grouping,
- adoption artifact route samples,
- release/readiness gates.

Profiles should be config-driven where possible. Hardcoded commands are useful
only for proving the first shapes; stable Fairway should let teams add their own
packet templates and task metadata without code changes.

## Config Surface

Fairway now accepts a minimal profile-as-config shape:

```toml
[[workstream_profiles]]
name = "platform-foundation"
task_kinds = ["architecture-map", "boundary-guard", "facade", "frontend-contract"]
dashboard_groups = ["architecture maps", "boundary guards", "facades", "frontend contracts"]
review_domains = ["architecture", "security", "frontend"]
route_samples = ["doc/api/openapi.yaml", "cmd/api/routes.go"]

[[workstream_profiles.gates]]
name = "security-review"
mode = "advisory"
task_kinds = ["facade"]
evidence_type = "security-review"
required_evidence_count = 1
accepted_results = ["pass", "partial"]
artifact_required = true
description = "Security review evidence should be attached before release readiness."

[[packet_templates]]
profiles = ["platform-foundation"]
name = "architecture-map"
required_fields = ["scope", "current_owner", "target_owner", "migration_risk", "acceptance"]
```

The first implementation use is intentionally small: `fairway config validate`
checks this shape, validates profile task kinds against configured task kinds,
validates packet-template profile references, and `fairway adoption artifact`
uses `route_samples` and evaluates named profile gates against matching task
evidence rows. `fairway merge-ready` evaluates the same gates for the target
task and fails when a `blocking` gate is missing. Gates can optionally narrow
themselves to specific task kinds inside the profile, which keeps
`boundary-guard` evidence from being required on unrelated facade or frontend
tasks. Tasks can also carry
profile-aware metadata (`profile`, `owning_domain`, `owning_layer`,
`source_paths`, `target_paths`, `review_domains`, `risk_level`,
`migration_type`) through imports and CLI flags. The dashboard now uses this
metadata for an initial workstream grouping by profile/kind, and later filters
and packet rendering should build on the same config rather than adding
project-specific flags.

## Task Taxonomy Conventions

Task IDs and follow-up prefixes are project conventions unless they are
explicitly defined by Fairway's generic task ID validation. Fairway core does
not assign semantic meaning to prefixes such as `CI-FIX-*`, `CD-FIX-*`,
`UAT-BUG-*`, `OPS-FIX-*`, `HARNESS-FIX-*`, or `DOC-FIX-*`.

Projects that want those labels should document them with the active
workstream profile and keep the taxonomy close to the gates, evidence types,
dashboard groups, and tracker labels that use it. For example, a
release-readiness profile may say:

| Prefix | Profile meaning |
|---|---|
| `CI-FIX-*` | Actionable CI/build/test/generated-contract follow-up. |
| `CD-FIX-*` | Deploy or release automation follow-up. |
| `UAT-BUG-*` | User-acceptance or smoke finding that needs product review. |
| `OPS-FIX-*` | Operational environment, credential, registry, or runner follow-up. |
| `HARNESS-FIX-*` | Test harness, fixture, or local reproduction follow-up. |
| `DOC-FIX-*` | Documentation, runbook, or generated documentation follow-up. |

The profile can then map the same taxonomy into task kinds, dashboard groups,
tracker labels, and review domains without requiring Go changes:

```toml
[[workstream_profiles]]
name = "release-readiness"
task_kinds = ["ci-fix", "deploy-fix", "uat-bug", "ops-fix", "harness-fix", "doc-fix"]
dashboard_groups = ["CI follow-ups", "deploy follow-ups", "UAT findings", "ops findings", "harness fixes", "docs fixes"]
review_domains = ["ops", "governance"]
```

Adapters or reports may recommend a project-specific prefix when configured or
documented by the active profile. They should still record ordinary Fairway
tasks, evidence, reviews, and gates; the prefix does not bypass ownership,
review-domain, or merge-readiness rules.

## Platform-Foundation Sequence

The platform-foundation profile should keep large architecture reshuffles in a
disciplined order:

1. ownership maps first,
2. report-only boundary guard visibility second,
3. facade or vertical-slice implementation third.

That sequence should be visible in imported queue dependencies. A facade task
should depend on the relevant `architecture-map` task and at least one
`boundary-guard` task unless the orchestrator records an explicit exception as
release-risk or residual-risk evidence. The point is not to block progress; it
is to keep file moves, package extraction, and route reshaping from starting
before ownership and guard visibility exist.

Fairway enforces the concrete dependencies provided by the imported queue and
uses profile gates to surface missing evidence. It does not infer architectural
order by itself.

## Packet Templates

The platform-foundation packet commands are the first implementation slice:

- `packet architecture-map`,
- `packet boundary-guard`,
- `packet vertical-slice`.

The intended generic model is declarative:

```toml
[[packet_templates]]
name = "architecture-map"
required_fields = ["scope", "current_owner", "target_owner", "migration_risk", "acceptance"]
optional_fields = ["source_doc"]

[[packet_templates]]
name = "boundary-guard"
required_fields = ["guard_intent", "graduation_criteria"]
optional_fields = ["finding", "false_positive", "proof_command"]
```

Template output includes task detail, evidence, reviews, and the supplied
template fields. The template validates shape; it does not run checks.

## Ownership Metadata

Architecture-aware work needs metadata that is useful across profiles:

- `owning_domain`,
- `owning_layer`,
- `source_paths`,
- `target_paths`,
- `review_domains`,
- `tags`,
- `risk_level`,
- `migration_type`.

This metadata attaches to task definitions and stays generic enough for
platform foundations, frontend migrations, service extractions, security
hardening, and SDK readiness.

Tags are cross-cutting grouping metadata. Use them when a task belongs to a
program or environment that cuts across profile, kind, owner, and review domain:
`production-readiness`, `security-review`, `docs-portal`, `mfa`,
`uat-hardening`, `pssm-closeout`, `fairway-process`, `environment:staging`, or
`environment:cloudflare`. Tags do not replace `profile`, `kind`,
`owning_domain`, `risk_level`, or `review_domains`; they add a secondary
grouping/filtering axis.

Profiles may recommend tag display groups:

```toml
[[workstream_profiles.tag_groups]]
name = "environments"
tags = ["environment:staging", "environment:cloudflare"]
description = "Deployment or validation environment buckets used by this profile."
```

## Guard Reports As Evidence

Boundary guards are structured evidence, not a separate execution engine.
`fairway record guard-report` stores a `guard-report` evidence row with
structured notes. Useful fields:

- guard name,
- mode: `report_only`, `warning`, or `blocking`,
- findings,
- false positives,
- allowed debt,
- graduation criteria,
- artifact path or URL.

Fairway records and summarizes the evidence. External scripts, CI jobs, or
scanners still produce the reports.

## Adoption Artifact

`fairway adoption artifact` is the generic readiness report. It should answer:

- Can this project safely switch more coordination to Fairway?
- What was imported?
- What is missing?
- Which gates are advisory vs. blocking?
- What evidence gaps exist?
- Which profile gates are satisfied or missing for matching tasks?
- What review routes and workstream samples are active?

`fairway parity artifact` remains as a compatibility spelling for legacy consumer
script-to-Fairway comparisons.

## Review Domains

First-match review routing is enough to assign an immediate reviewer. Merge
readiness also checks task-level `review_domains` and requires one approved
review whose reviewer matches each domain. These values must match the reviewer
identity that the project records, usually a configured role or lane:

- architecture,
- security,
- frontend,
- ops,
- governance.

For an A/B/C/D/E lane model, the same concept may be represented as
`D-arch`, `E-governance`, `B-ui`, or `C-ops`. Keep profile examples and imported
queue files aligned with the configured reviewer names.

This distinguishes "who picks this up now" from "which domains must approve
before merge/release readiness."

## Dashboard Grouping

Dashboard grouping starts with task metadata: tasks with a profile or
non-default kind appear in workstream sections grouped as `profile / kind`. For
platform-foundation, useful groups are:

- architecture maps,
- boundary guards,
- facades,
- frontend contracts,
- evidence tasks,
- release-risk tasks.

The dashboard should remain a coordination view over Fairway state, not a
project-specific portal.
