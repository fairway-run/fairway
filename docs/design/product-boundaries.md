# Product Boundaries

Fairway is a coordination control plane for multi-agent engineering work. It
keeps execution state visible, auditable, and reviewable. It is not an
autonomous workflow engine.

This page defines the product boundary so orchestration, adapters, usage
accounting, and dashboard features do not drift into hidden decision-making.

## Fairway Will Do

- Track tasks, ownership, state transitions, evidence, handoffs, reviews,
  sessions, watchers, worktrees, batches, and closeout.
- Surface stale, unsafe, review-gated, approval-gated, utility-gated, or
  incomplete work.
- Recommend next actions through CLI reports, dashboard diagnostics, and
  dry-run controller plans.
- Record provider-neutral usage metadata when an adapter supplies it.
- Coordinate deterministic utilities such as CI monitors, codegen drift checks,
  release asset checks, and registry freshness checks.
- Integrate with planning systems such as Plane, Jira, Linear, and GitHub
  Issues while keeping local execution state in Fairway.

## Fairway Will Not Do

- Auto-claim ready tasks or silently transfer ownership between lanes.
- Auto-approve reviews or waive required review domains.
- Auto-merge branches or auto-push commits.
- Auto-delete local branches, worktrees, or task state without an explicit
  operator command.
- Perform destructive, production-impacting, credential, or approval-gated
  actions without an explicit stop condition and operator decision.
- Become a CI runner. Fairway records CI evidence and monitor handbacks; CI
  systems still execute CI.
- Become a workflow/DAG engine. Fairway coordinates human-paced engineering
  lanes; it does not replace Temporal, Cadence, Argo Workflows, or similar
  systems.
- Replace external planning tools. Plane, Jira, Linear, and GitHub Issues can
  mirror roadmap or stakeholder context; they do not own Fairway execution
  state.
- Become an LLM provider abstraction. Fairway records sessions and usage
  metadata supplied by adapters; providers remain external.
- Store prompts, transcripts, secrets, provider credentials, or private auth
  material by default.
- Gate task completion on token cost, provider spend, or model choice. Usage
  accounting is an operational planning signal, not a completion gate.
- Encode one project's taxonomy as core product grammar. Prefixes such as
  `CI-FIX-*`, `UAT-BUG-*`, or `OPS-FIX-*` belong to workstream profiles and
  project conventions.

## Controller Rule

The orchestration controller is advisory by default. It may classify state,
recommend actions, start explicitly configured utility monitors, and emit
provider continuation prompts when judgment is needed.

It must not silently claim, approve, merge, delete, push, deploy, or mutate
production-impacting state. Any future apply path must name the exact mutation,
show the dry-run plan, and keep stop conditions visible.

## Usage Accounting Rule

Provider usage records are for planning and retrospection:

- which provider or model was expensive,
- which task shapes consume more tokens,
- which work should be moved to deterministic utilities,
- which workflows should be batched.

Usage records must not include prompts, transcripts, secrets, or provider auth
material. Fairway may report counts, phases, roles, task IDs, provider names,
confidence, and source metadata.

## Adapter Rule

Adapters are edge contracts. Core Fairway remains provider-neutral.

- Provider adapters attach agent sessions and optional usage metadata.
- Utility adapters attach deterministic work such as CI polling.
- Tracker adapters mirror planning context to or from external issue trackers.

Adapters may feed Fairway evidence, sessions, checkpoints, usage, and links.
They do not decide task status, review approval, merge readiness, or release
promotion on their own.
