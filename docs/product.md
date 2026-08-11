# Product

## Product Definition

Fairway is the local, harness-neutral coordination and engineering-record layer
for agent-driven software delivery. It keeps concurrent work connected across
tasks, lanes, worktrees, provider sessions, reviews, evidence, source control,
and CI/CD without becoming the coding-agent runtime.

The technical problem is fragmented execution state. Provider conversations
end, context windows compact, agents move between harnesses, and Git or CI can
show what ran without showing which bounded task authorized it, who owns the
next action, or which evidence and independent judgment remain missing.
Fairway maintains that cross-system state and computes deterministic readbacks
from it.

The supported Quality Record is one cited, read-only projection of this
engineering record across intent,
material decisions, production context, evidence, automatic verification,
human judgment, promotion, operational outcomes, and controlled lessons. Each
stage reports `present`, `missing`, `unavailable`, `conflicting`, or
`externally_owned`. The projection is a product capability, not a development
methodology, quality score, or source of approval authority.

It keeps accountable intent, material decisions, evidence, independent
judgment, and promotion state durable while coding agents and engineering tools
perform the work. The result is a replaceable-provider workflow: a provider can
stop, a context window can compact, or an orchestrator can change without
turning chat history into the system of record.

Fairway is local-first. Its default product shape is one Go binary, one SQLite
execution store, a CLI, and read-oriented dashboards. The portfolio Quality
workspace makes lifecycle-stage coverage and attention visible across tasks,
while task detail retains the cited facts and authority boundaries.
The single-project Overview gives first-time users a product-level path through
the accountability chain, current project coverage, one cited record, system
authority boundaries, and the specialist operational views.
Coordination primitives
such as sessions, checkpoints, handoffs, waits, notifications, lanes, and
worktrees are the technical control surface. Governance, compliance, and
assurance are possible consequences of explicit facts and boundaries; they are
not Fairway's primary category.

## Boundary With Agent Execution

Fairway coordinates work above individual agent runs. A coding harness or
runtime executes the run. Seaway is a separately scoped, optional runtime
product intended to provide a stable contract around one bounded agent run; it
is not required by Fairway and is not an implemented Fairway subsystem.

| Concern | Fairway | Coding runtime or optional Seaway |
|---|---|---|
| Unit of work | Task, lane, worktree assignment, review, and promotion record | One run or attempt against an explicit execution context |
| Lifecycle | Cross-run ownership, waits, handbacks, evidence coverage, and readiness | Admission, start, observation, cancellation, reconnect, and terminal result |
| Policy | Workflow gates, review domains, evidence expectations, and promotion boundaries | Provider/model eligibility, tools, credentials, files, network, data egress, and run-time approvals |
| Facts | Links task intent to sessions, commits, CI, reviews, outcomes, and externally owned evidence | Emits run events, artifacts, evidence references, usage, cost, and failure facts |
| Authority | Records and checks coordination and promotion posture without performing promotion | Controls only the execution capabilities its adapter or environment can honestly enforce |

The products remain independently useful. Fairway can coordinate Codex,
Claude Code, Gemini, Jcode, shell, CI, and other execution surfaces without
Seaway. Seaway can serve a CLI, CI job, gateway, IDE, or another control plane
without Fairway.

The state models do not merge:

- one Fairway task may have zero, one, or many runtime runs;
- a successful run does not complete a Fairway task;
- a run-time tool or egress approval is not an independent Fairway review;
- cancelling or losing a run does not silently transition task state;
- run evidence retains its source, integrity reference, and uncertainty when
  linked into Fairway; and
- a durable Fairway lane worktree and a disposable run workspace remain
  distinct even when they point at the same repository revision.

## Operating Model: Durable Record, Temporary Execution

Fairway does not require a permanent provider chat for every role. A team may
keep a small number of durable control surfaces for recurring cross-task
judgment, prioritization, coordination, or governance. Bounded implementation,
investigation, and review should use the smallest execution surface that can
finish the work safely.

| Execution surface | Use it when | Normal closeout |
|---|---|---|
| Subagent | Work is short, bounded to the current task, and does not need a separate human conversation. | Return evidence or findings to the parent, reconcile material facts into Fairway, then end the attachment. |
| Task-specific thread | Work needs independent interaction, review, approvals, long waits, or continuity across turns. | Record the handback and evidence, end the Fairway session, then archive the provider thread when no interaction remains. |
| Durable control surface | The same accountable function repeatedly makes cross-task decisions or steers work over time. | Keep the surface small and current; Fairway, not its transcript, remains the execution authority. |

The count and names of durable control surfaces are project choices, not
Fairway product grammar. A solo maintainer may need one. A larger program may
separate product or architecture judgment, delivery coordination, and
governance. Permanent implementation and reviewer chats are discouraged when
task-specific attachments can support independent work with less stale context.

Every material execution attachment still maps to a Fairway task and the
applicable session, checkpoint, evidence, review, or handback records required
by project policy. Material work delegated to a subagent must use a registered
provider session; the short direct-coordinator exception does not apply to
delegated work. A separate subagent or thread does not by itself establish
reviewer independence: the routed reviewer identity must differ from the task
owner and claimant and must satisfy the configured review domain.
Host-side subagent or thread history is useful context, but it does not replace
the Fairway record. Archiving a completed provider thread removes UI clutter;
it does not delete or weaken the task's durable engineering record.

Fairway also defines two complementary continuity capabilities. Its existing
database-backed track memory keeps active execution resumable across provider
replacement and context loss; legacy project-local memory files are migration
inputs, not a second authority. Engineering knowledge maintains
source-grounded, project-owned synthesis across tasks. Memory remains curated
operating context; knowledge remains derived and non-canonical until promoted
through normal documentation review. See
[Project working memory](design/project-working-memory.md) and
[Engineering knowledge](design/engineering-knowledge.md).

Reusable rule packs add a separate operating-knowledge layer. Fairway core
owns loading, matching, evidence expectations, and closeout readback; projects
and domain packs own the rules themselves. Assurance profiles then map recorded
facts into bounded readiness and evidence-gap reports without converting those
reports into certification or approval. Proposed execution profiles, including
the large-migration profile, compose these primitives but are not implemented
runtime capabilities until their release notes say otherwise.

Fairway also treats governance as an observable engineering system. Delivery
reports expose velocity and coordination overhead. Implemented advisory
control-effectiveness analytics now measure coverage, control-specific signal,
friction, and observable outcomes through the CLI and a read-only dashboard
without claiming causality or granting policy authority. Two GPUaaS pilots
validated coverage-first suppression and population-scale Quality Record
reconstruction while exposing adoption and instrumentation gaps. They did not
claim incremental control effectiveness or a complete AI Quality System. See
[Control effectiveness](design/control-effectiveness.md), the
[first GPUaaS pilot](assessment/gpuaas-control-effectiveness-pilot-2026-08-02.md),
and the [Quality Record pilot](assessment/gpuaas-quality-record-pilot-2026-08-05.md).

## The Product Promise

For every bounded work item, a team should be able to determine:

- **Intent:** owner, scope, acceptance, risk, and current state.
- **Decision:** the material choice, alternatives, rationale, and cited facts.
- **Evidence:** commands and safe artifact references that support or contradict
  the current claim.
- **Judgment:** required review, recorded verdicts, and unresolved waits.
- **Promotion:** whether the work remains local and reversible or has satisfied
  the explicit controls for merge, release, deploy, or live execution.
- **Outcome and lesson:** what happened after promotion and which reviewed
  process or engineering change follows from it.

`fairway quality-record <task-id>` projects these facts together and cites the
underlying records or external authority. It does not fill absent facts with a
generated narrative.

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
| **Implemented** | Present in the current source and covered by repository validation. | Local CLI/SQLite store; tasks, sessions, checkpoints, decisions, evidence, handoffs, reviews, waits, notifications; cited task and portfolio Quality Record projections; task-to-commit, structured-outcome, and attributable-friction records; versioned agent contracts; track memory; deterministic engineering-knowledge packets; local rule-pack matching; assurance evidence mapping and offline release packaging; workflow and merge-readiness checks; advisory control-effectiveness CLI/dashboard; read-oriented dashboards. |
| **Validated practice** | Used in a bounded real workflow with durable evidence, but not claimed as universal or externally certified. | Internal consumer provider replacement, memory/knowledge cold starts, review/release coordination, environment rehearsal, local shared-dashboard operation, and GPUaaS Quality Record/control-analytics data-quality calibration documented under `docs/assessment/`. |
| **Experimental** | Implemented as an explicit pilot or advisory surface and not the default authority path. | Shared-team server/write pilots, advisory provider narratives, notifier adapters, Postgres compatibility rehearsal, and prototype operating profiles. |
| **Planned** | Designed or tracked but not implemented as a supported runtime capability. | Migration execution profiles, a production Postgres runtime adapter, broad tracker API adapters, and a reviewed shared-team production deployment path. |
| **Non-goal** | Deliberately outside Fairway authority. | Autonomous approval, risk acceptance, merge, push, deploy, live mutation, credential custody, transcript-as-authority, or regulatory certification. |

The dated [documentation inventory](assessment/fairway-documentation-inventory-2026-07-11.md)
assigns every existing page a canonical or supporting role. Dated assessments
are evidence inputs, not evergreen product authority.

## How Fairway Fits

| System | Owns | Fairway contribution |
|---|---|---|
| Coding agent or IDE | Code generation, investigation, provider interaction | Bounded task packet, durable attachment, checkpoint, evidence, and handback state |
| Agent runtime or optional Seaway | Admission and execution of an individual run, including effective runtime policy, events, result, usage, and cost | Correlated run identity and facts without merging run state into task state |
| Git and forge | Source history, branches, pull requests, remote collaboration | Worktree/branch posture, commit evidence, promotion and merge-readiness checks |
| CI/CD | Build, test, package, deploy, release execution | Monitor state, evidence references, failure routing, release/deploy preflight and handback |
| Issue tracker | Roadmap, stakeholder planning, backlog discussion | Import/link/export context while retaining execution truth in Fairway |
| Agent orchestrator | Provider scheduling and steering | Deterministic next actions, waits, packets, capability checks, and authority guards |
| Identity proxy | Authentication and access policy | Read-only/shared boundary metadata and fail-closed configuration; no replacement for the proxy |

```text
Fairway: task / lane / worktree / session / review / evidence / readiness
                         |
                         | zero or more correlated runs
                         v
Coding runtime or optional Seaway: run policy / events / result / usage / cost
                         |
                         v
Coding harness / provider / tools / bounded execution environment
```

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
11. **Small control plane, elastic execution.** Keep recurring control surfaces
    few and stable; create and retire execution attachments at the boundary of
    the work they serve.

## Source Of Truth

Versioned backlog and profile files define intended task shape. The Fairway DB
owns runtime claims, status, sessions, checkpoints, decisions, evidence,
reviews, waits, handbacks, and notifications. Git, CI/CD, issue trackers, and
provider systems remain authoritative for the facts they execute or host.

Fairway links those facts; it does not overwrite their ownership.

## Direction

Current product work focuses on making the common multi-agent control loop
direct: identify active and stale work, preserve task and worktree ownership,
attach provider-neutral sessions, connect evidence and review, and expose the
next safe action. The Quality Record, shared-team boundaries, and control
analytics build on that technical record. Coverage and observational limits
remain visible; sparse data never becomes a reason to waive mandatory safety
invariants. "AI Engineering Quality System" remains a direction to evaluate,
not Fairway's category, a current completeness claim, or certification.

The versioned [product backlog](roadmap/fairway-product-backlog.yaml) records
planned work. Release scope and implemented behavior are reported in
[release notes](release-notes.md), not predicted here as dated version promises.

## Non-Goals

Fairway is not a workflow/DAG engine, CI runner, issue tracker, IAM provider,
LLM gateway, credential store, artifact signer, compliance certification
system, or autonomous engineering manager. It does not silently claim,
approve, merge, push, deploy, release, or mutate live environments.

The durable rules are in [Product boundaries](design/product-boundaries.md).
