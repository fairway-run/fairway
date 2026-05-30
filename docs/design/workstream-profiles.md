# Workstream Profiles

Workstream profiles keep Fairway generic while making it useful for
architecture-aware coordination. A profile is a named operating shape for a kind
of engineering work. It defines task kinds, packet templates, review domains,
evidence expectations, and dashboard grouping without making Fairway a CI/CD
tool, scanner, docs portal, or agent runner.

## Product Framing

Fairway is a local-first coordination control plane for multi-agent engineering
work: tasks, ownership, evidence, reviews, handoffs, sessions, readiness, and
risk.

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
task_kinds = ["architecture-map", "guard", "facade", "frontend-contract"]
dashboard_groups = ["architecture maps", "boundary guards", "facades", "frontend contracts"]
review_domains = ["architecture", "security", "frontend"]
route_samples = ["doc/api/openapi.yaml", "cmd/api/routes.go"]

[[workstream_profiles.gates]]
name = "security-review"
mode = "advisory"
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
task and fails when a `blocking` gate is missing. Dashboard grouping and packet
rendering should build on this same config rather than adding project-specific
flags.

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

Template output should include task detail, evidence, reviews, and the template
fields. The template validates shape; it does not run checks.

## Ownership Metadata

Architecture-aware work needs metadata that is useful across profiles:

- `owning_domain`,
- `owning_layer`,
- `source_paths`,
- `target_paths`,
- `review_domains`,
- `risk_level`,
- `migration_type`.

This should attach to task definitions or a task metadata table. It should stay
generic enough for platform foundations, frontend migrations, service
extractions, security hardening, and SDK readiness.

## Guard Reports As Evidence

Boundary guards should be structured evidence, not a separate execution engine.
Useful fields:

- guard name,
- mode: `report_only`, `warning`, or `blocking`,
- findings,
- false positives,
- allowed debt,
- graduation criteria,
- artifact path or URL.

Fairway should record and summarize the evidence. External scripts, CI jobs, or
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

`fairway parity artifact` remains as a compatibility spelling for GPUaaS-style
script-to-Fairway comparisons.

## Review Domains

First-match review routing is enough to assign an immediate reviewer. Merge
readiness eventually needs multiple required review domains:

- architecture,
- security,
- frontend,
- ops,
- governance.

The future model should distinguish "who picks this up now" from "which
domains must approve before merge/release readiness."

## Dashboard Grouping

Dashboard grouping should be configurable by task kind or profile. For
platform-foundation, useful groups are:

- architecture maps,
- guards,
- facades,
- frontend contracts,
- evidence tasks,
- release-risk tasks.

The dashboard should remain a coordination view over Fairway state, not a
project-specific portal.
