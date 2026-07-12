# Regression Packets

Consumer projects added two quality surfaces after the queue existed: workflow regression
packs and bug-fix review packets. Fairway keeps both as generated packet/catalog
commands, not as a built-in test runner.

## Workflow Regression Packs

A workflow regression pack is a named release-risk bundle. It describes a user
or operator workflow, its owner, target environments, required seed data, proof
commands, artifacts, current automation, and known gaps.

```bash
fairway regression-pack list
fairway regression-pack show <pack-id>
fairway regression-pack validate [<catalog-path>]
```

The catalog format is implementation-defined in v1, but each pack must include:

| Field | Purpose |
|---|---|
| `id` | Stable identifier used in context and bugfix packets. |
| `title` | Human-readable workflow name. |
| `owner` | Role responsible for keeping the pack useful. |
| `target_environments` | Local, CI, demo, staging, or other environments. |
| `blocking` | Whether failure blocks release. |
| `required_seed_data` | Assumptions the proof depends on. |
| `lowest_reliable_layer` | Unit, integration, browser, remote smoke, or manual smoke. |
| `required_proof` | Commands or manual checks that satisfy the pack. |
| `artifact_requirements` | Screenshots, logs, reports, or URLs expected as evidence. |
| `current_automation` | Existing coverage. |
| `gaps` | Known missing coverage. |

Fairway records pack runs as normal evidence. The pack catalog gives reviewers a
shared checklist; it does not replace CI, Playwright, or product-specific smoke
tools.

## Bug-Fix Review Packets

Bug fixes need a root-cause handoff that prevents symptom-only patches from
being marked done.

```bash
fairway packet bugfix <task-id> \
  --bug-summary <text> \
  --root-cause <text> \
  --owning-layer <text> \
  --impact <text> \
  --proof-command <command> \
  --regression-coverage <text> \
  [--workflow-pack <pack-id>] \
  [--scope-drift <text>] \
  [--residual-risk <text>] \
  [--follow-up <task-id-or-text>]
```

The command renders Markdown by default and can write to `--output <path>`.

## Packet Shape

```markdown
# Bug-Fix Review Packet: T-042

bug_summary:
owning_layer:
workflow_pack:

## Root Cause

## Impact

## Proof Command

## Regression Coverage

## Scope Drift

## Residual Risk

## Follow-Up / Blocker

## Review Questions
```

## Rules

- A bug-fix task cannot be done with only a symptom patch.
- Proof must exercise the root cause, not just the visible failure.
- Regression coverage should land at the lowest reliable layer.
- Deferred regression coverage needs a linked blocker or release-blocking
  follow-up.
- Workflow-affecting fixes should name the relevant regression pack.
