# Workstream Profile Guide

Workstream profiles let a project describe the shape of a track without making
Fairway project-specific. Use them when a repo has recurring coordination needs
such as release readiness, platform foundation work, frontend migration,
service extraction, SDK readiness, or security hardening.

Profiles are operational in readiness checks and dashboard visibility. Fairway
validates the config, uses `route_samples` in `fairway adoption artifact`,
evaluates named profile gates against recorded evidence rows, applies blocking
profile gates in `fairway merge-ready`, and groups dashboard tasks by
profile/kind when metadata is present. The dashboard can also filter by
profile, kind, owning domain, risk, and review domain; it also evaluates
profile gates in the main view and bounds task/activity tables for larger
tracks. Tasks can carry profile-aware metadata such as owning domain/layer,
source/target paths, review domains, risk level, and migration type.
Template-rendered packets are
available through `fairway packet template`; guard reports are recorded through
`fairway record guard-report`.

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
group = "security gates"
mode = "advisory"
task_kinds = ["release-risk"]
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

Use `advisory` for expectations that should appear in reports and
`merge-ready` warnings but should not block readiness yet. Use `blocking` for
gates that must pass before `merge-ready` succeeds. Use `report_only` for early
guard rails where findings are still being measured.

## Evidence Requirements

Profile gates can describe the evidence a workstream expects:

- `group` clusters related gates in dashboard and report surfaces. Use stable
  nouns such as `boundary guards`, `release evidence`, `security gates`, or
  `SDK readiness` so large tracks show readiness posture first instead of a
  flat list of every check.
- `task_kinds` narrows a gate to specific task kinds inside the profile. Omit it
  when every task kind in the profile needs the same evidence.
- `required_evidence_count` says how many matching evidence records should
  exist.
- `accepted_results` lists acceptable evidence results. Use values from
  `fairway record evidence`: `pass`, `fail`, `partial`, `skipped`, or
  `blocked`.
- `artifact_required` says the evidence should include a durable path or URL.
- `owner_signoff_required` says a named owner approval is expected.
- `expires_after` records how long the evidence remains fresh.

These fields are validated, evaluated in adoption artifacts, and enforced by
`merge-ready` when the gate mode is `blocking`.

Gate evaluation first matches profile gates to tasks by the profile's
`task_kinds`, then narrows again by the gate's optional `task_kinds`. It then
counts evidence rows that satisfy every configured requirement:

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

The dashboard shows the same profile gates as a live readiness panel. Use it
while agents are working so missing evidence is visible before `merge-ready` or
`readiness report` becomes the first place the team sees the gap.

For large tracks, group gates by evidence family and make the default dashboard
exception-first: missing blocking gates should be visible immediately, passed
gates should be rollups, and detailed gate rows should be used for drill-down.
This is the pattern that kept the GPUaaS platform-foundation track readable:
maps first, guard visibility second, facade implementation third.

## Task Metadata

Use task metadata when the workstream needs architecture context that should
survive handoffs and exports:

```bash
fairway add PF-003 \
  --title "Introduce platform evidence facade" \
  --role backend \
  --kind facade \
  --profile platform-foundation \
  --owning-domain platform \
  --owning-layer service \
  --source-paths cmd/api/evidence.go,packages/services \
  --target-paths packages/services/platform/evidence \
  --review-domains architecture,security \
  --risk-level high \
  --migration-type facade
```

The same fields are accepted in YAML/JSON imports. See
[`examples/platform-foundation-queue.yaml`](../examples/platform-foundation-queue.yaml)
for a generic platform-foundation queue.

For platform-foundation reshuffles, keep the queue dependency order explicit:
ownership maps first, report-only boundary guards second, facade or
vertical-slice implementation third. In practice, facade tasks should depend on
both the relevant `architecture-map` task and a `boundary-guard` task unless the
orchestrator records an explicit residual-risk exception. This makes the
discipline visible to agents and reviewers before work starts.

For planning-heavy or documentation-heavy tracks, make the task owner an
orchestrator role and keep architecture, security, frontend, ops, and governance
as review domains. This avoids accidental self-review when a path such as
`doc/architecture/**` routes to the architecture reviewer. Implementation tasks
can still be owned by the lane doing the work, such as `backend` or `ui`.

## Project Task Taxonomy

Document project-specific task prefixes in the profile guide or project
backlog, not in Fairway core. Prefixes such as `CI-FIX-*`, `CD-FIX-*`,
`UAT-BUG-*`, `OPS-FIX-*`, `HARNESS-FIX-*`, and `DOC-FIX-*` are useful
conventions for release-readiness or stabilization tracks, but they do not
change Fairway's generic task model.

When a project uses prefixes, define what each prefix means, which task kind it
maps to, which review domains normally apply, and which evidence type proves
the follow-up. Keep the same taxonomy aligned across:

- imported queue IDs,
- `task_kinds`,
- dashboard groups,
- tracker labels,
- CI/deploy/UAT monitor recommendations,
- review-domain rules.

If a different project uses `BUILD-*`, `REL-*`, or ordinary issue IDs, Fairway
should work the same way as long as the profile and queue metadata define the
coordination contract.

## Agent Guidance

Agents should treat profile config as the local contract for that workstream.
For example, if a task kind is `architecture-map`, the agent should use the
matching packet command or configured packet template fields and record evidence
against the named gates. If the profile is missing a gate or field, update the
config or docs rather than inventing a private checklist in chat.

```bash
fairway packet template architecture-map T-010 \
  --field scope="route ownership" \
  --field current_owner=mixed \
  --field target_owner=D-arch \
  --field migration_risk="route moves can hide auth regressions" \
  --field acceptance="owners and review routes are explicit"
```

Guard reports should be recorded as evidence rather than pasted into chat:

```bash
fairway record guard-report T-011 \
  --guard import-boundary \
  --mode report_only \
  --finding "cmd/api imports billing internals" \
  --false-positive "generated client code" \
  --allowed-debt "legacy route package" \
  --graduation-criteria "zero critical findings for two releases" \
  --artifact dist/import-boundary.json
```

Use readiness reports when a workstream or release needs a gate summary:

```bash
fairway readiness report --profile release-readiness
fairway --json readiness report --profile platform-foundation
```

Tasks with `review_domains` require one approved review per domain before
`merge-ready` succeeds. Use domain names that match the reviewer identities your
repo records, such as `architecture`, `security`, `frontend`, `ops`, or
`governance`. If the repo uses lane IDs as reviewers, such as `D-arch` or
`E-governance`, use those exact lane IDs in `review_domains`; otherwise the
queue can import successfully but `merge-ready` cannot be satisfied by recorded
reviews.
