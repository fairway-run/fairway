# Product

## Product Definition

Fairway is the engineering control and accountability layer for agent-driven
software delivery.

It keeps accountable intent, material decisions, evidence, independent
judgment, and promotion state durable while coding agents and engineering tools
perform the work. The result is a replaceable-provider workflow: a provider can
stop, a context window can compact, or an orchestrator can change without
turning chat history into the system of record.

Fairway is local-first. Its default product shape is one Go binary, one SQLite
execution store, a CLI, and read-oriented dashboards. Coordination primitives
such as sessions, checkpoints, handoffs, waits, notifications, lanes, and
worktrees support the accountability model; they do not define the category.

## The Product Promise

For every bounded work item, a team should be able to determine:

- **Intent:** owner, scope, acceptance, risk, and current state.
- **Decision:** the material choice, alternatives, rationale, and cited facts.
- **Evidence:** commands and safe artifact references that support or contradict
  the current claim.
- **Judgment:** required review, recorded verdicts, and unresolved waits.
- **Promotion:** whether the work remains local and reversible or has satisfied
  the explicit controls for merge, release, deploy, or live execution.

Generated rationale, provider transcripts, and advisory recommendations may
help a person reason. They are not provenance, approval, or risk acceptance.

## Who It Is For

- Individual engineers using more than one coding-agent session.
- Small teams that need shared visibility without handing approval authority to
  a dashboard or orchestrator.
- Reviewers and operators who need to see missing evidence, stale work, and
  promotion blockers without reconstructing provider conversations.
- Platform teams that want a provider-neutral execution record alongside their
  existing source control, CI/CD, and planning systems.

## Capability And Claim Inventory

These labels are mandatory in public and canonical Fairway documentation.

| Label | Meaning | Current Fairway examples |
|---|---|---|
| **Implemented** | Present in the current source and covered by repository validation. | Local CLI/SQLite store; tasks, sessions, checkpoints, decisions, evidence, handoffs, reviews, waits, notifications; workflow and merge-readiness checks; read-oriented dashboards; packets and reports; release packaging. |
| **Validated practice** | Used in a bounded real workflow with durable evidence, but not claimed as universal or externally certified. | Internal consumer provider replacement, review/release coordination, environment rehearsal, and local shared-dashboard operation documented under `docs/assessment/`. |
| **Experimental** | Implemented as an explicit pilot or advisory surface and not the default authority path. | Shared-team server/write pilots, advisory provider narratives, notifier adapters, Postgres compatibility rehearsal, and prototype operating profiles. |
| **Planned** | Designed or tracked but not implemented as a supported runtime capability. | A production Postgres runtime adapter, broad tracker API adapters, and a reviewed shared-team production deployment path. |
| **Non-goal** | Deliberately outside Fairway authority. | Autonomous approval, risk acceptance, merge, push, deploy, live mutation, credential custody, transcript-as-authority, or regulatory certification. |

The dated [documentation inventory](assessment/fairway-documentation-inventory-2026-07-11.md)
assigns every existing page a canonical or supporting role. Dated assessments
are evidence inputs, not evergreen product authority.

## How Fairway Fits

| System | Owns | Fairway contribution |
|---|---|---|
| Coding agent or IDE | Code generation, investigation, provider interaction | Bounded task packet, durable attachment, checkpoint, evidence, and handback state |
| Git and forge | Source history, branches, pull requests, remote collaboration | Worktree/branch posture, commit evidence, promotion and merge-readiness checks |
| CI/CD | Build, test, package, deploy, release execution | Monitor state, evidence references, failure routing, release/deploy preflight and handback |
| Issue tracker | Roadmap, stakeholder planning, backlog discussion | Import/link/export context while retaining execution truth in Fairway |
| Agent orchestrator | Provider scheduling and steering | Deterministic next actions, waits, packets, capability checks, and authority guards |
| Identity proxy | Authentication and access policy | Read-only/shared boundary metadata and fail-closed configuration; no replacement for the proxy |

Fairway composes with these systems. It does not claim to replace them.

## Principles

1. **Accountability before automation.** Every consequential action has a named
   actor, boundary, and evidence expectation.
2. **Evidence before assertion.** Command results and safe artifacts support
   claims; generated summaries do not become proof by repetition.
3. **Independent judgment stays independent.** Review routing and inheritance
   cannot silently waive live, production, security, release, credential, or
   public-exposure boundaries.
4. **Promotion is explicit.** Local reversible work and remote or live action
   are different states with different controls.
5. **Deterministic state before advisory intelligence.** Fairway computes
   routine next actions from durable facts before asking a model or person to
   interpret exceptions.
6. **Local-first by default.** Core work requires no hosted Fairway service.
7. **Configurable policy, stable product grammar.** Projects configure roles,
   profiles, routes, and gates without hardcoding one consumer taxonomy into
   core.
8. **Provider-neutral records.** Provider sessions are replaceable attachments;
   the task, decision, evidence, and review record is durable.
9. **Progressive disclosure.** A common reversible path stays short; advanced
   coordination and consequential gates appear when the work requires them.
10. **No hidden authority.** A dashboard, adapter, watcher, recommendation, or
    notification does not silently gain approval, merge, deploy, or live-action
    authority.

## Source Of Truth

Versioned backlog and profile files define intended task shape. The Fairway DB
owns runtime claims, status, sessions, checkpoints, decisions, evidence,
reviews, waits, handbacks, and notifications. Git, CI/CD, issue trackers, and
provider systems remain authoritative for the facts they execute or host.

Fairway links those facts; it does not overwrite their ownership.

## Direction

Current product work focuses on making the accountability chain easier to
adopt, strengthening shared-team boundaries without abandoning local-first
operation, and measuring whether process improves speed, quality, or safety.

The versioned [product backlog](roadmap/fairway-product-backlog.yaml) records
planned work. Release scope and implemented behavior are reported in
[release notes](release-notes.md), not predicted here as dated version promises.

## Non-Goals

Fairway is not a workflow/DAG engine, CI runner, issue tracker, IAM provider,
LLM gateway, credential store, artifact signer, compliance certification
system, or autonomous engineering manager. It does not silently claim,
approve, merge, push, deploy, release, or mutate live environments.

The durable rules are in [Product boundaries](design/product-boundaries.md).
