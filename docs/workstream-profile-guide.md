# Workstream Profile Guide

Workstream profiles let a project describe the shape of a track without making
Fairway project-specific. Use them when a repo has recurring coordination needs
such as release readiness, platform foundation work, frontend migration,
service extraction, SDK readiness, or security hardening.

Profiles are advisory in the current release. Fairway validates the config,
uses `route_samples` in `fairway adoption artifact`, and evaluates named
profile gates against recorded evidence rows. Dashboard grouping, structured
guard evidence, task ownership metadata, and template-rendered packets are
planned follow-on work.

## When To Add One

Add a profile when agents or reviewers keep asking the same coordination
questions:

- Which task kinds belong to this workstream?
- Which paths should prove review routing is configured?
- Which evidence or approvals are expected before readiness?
- Which review domains should be considered for merge or release decisions?
- Which packet fields should agents fill out for this track?

Do not add a profile for one-off task instructions. Put those in the task notes
or a context packet instead.

## Minimal Example

```toml
[[workstream_profiles]]
name = "release-readiness"
task_kinds = ["release-risk", "uat-evidence", "approval"]
dashboard_groups = ["release risks", "uat evidence", "approvals"]
review_domains = ["security", "ops", "governance"]
route_samples = ["scripts/release/check.sh", "doc/release/runbook.md"]

[[workstream_profiles.gates]]
name = "security-review"
mode = "advisory"
evidence_type = "security-review"
required_evidence_count = 1
accepted_results = ["pass", "partial"]
artifact_required = true
expires_after = "720h"
description = "Security review evidence should be attached before release readiness."

[[workstream_profiles.gates]]
name = "release-owner-approval"
mode = "blocking"
evidence_type = "approval"
required_evidence_count = 1
owner_signoff_required = true

[[packet_templates]]
profiles = ["release-readiness"]
name = "release-risk"
required_fields = ["risk", "owner", "severity", "mitigation", "residual_risk"]
optional_fields = ["expiry", "accepted_by"]
```

## Gate Modes

Use `advisory` for expectations that should appear in reports but do not block
state transitions yet. Use `blocking` for gates the team intends to enforce.
Use `report_only` for early guard rails where findings are still being
measured.

Current Fairway reports these modes; it does not yet enforce named profile
gates.

## Evidence Requirements

Profile gates can describe the evidence a workstream expects:

- `required_evidence_count` says how many matching evidence records should
  exist.
- `accepted_results` lists acceptable evidence results. Use values from
  `fairway record evidence`: `pass`, `fail`, `partial`, `skipped`, or
  `blocked`.
- `artifact_required` says the evidence should include a durable path or URL.
- `owner_signoff_required` says a named owner approval is expected.
- `expires_after` records how long the evidence remains fresh.

These fields are validated and evaluated in adoption artifacts today. They are
not yet enforced by `merge-ready`.

Gate evaluation matches profile gates to tasks by `task_kinds`, then counts
evidence rows that satisfy every configured requirement:

- `evidence_type` matches `fairway record evidence --artifact-type`.
- `accepted_results` matches `--result`.
- `artifact_required` requires the counted row to include `--artifact`.
- `expires_after` requires the counted row's `created_at` timestamp to be
  within the configured duration.
- `owner_signoff_required` requires the counted row's notes to contain
  `signoff` or `sign-off`.

For `required_evidence_count > 1`, every counted row must satisfy all of the
gate's configured evidence requirements. Stale rows or rows missing required
artifacts/signoff do not contribute to the count.

## Adoption Artifact

After adding or changing a profile:

```bash
fairway config validate
fairway adoption artifact
fairway --json adoption artifact > .fairway/adoption.json
```

The artifact should show the profile gates, route samples, and gate evaluation
summary you configured. If route samples do not resolve to the expected
reviewers, update `[[review_routes]]` before asking agents to rely on the
profile. If a gate reports missing tasks, record the required evidence or adjust
the gate before treating the workstream as ready.

## Agent Guidance

Agents should treat profile config as the local contract for that workstream.
For example, if a task kind is `architecture-map`, the agent should use the
matching packet command or packet template fields and record evidence against
the named gates. If the profile is missing a gate or field, update the config or
docs rather than inventing a private checklist in chat.
