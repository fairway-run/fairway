<p align="center">
  <img src="assets/logo-lockup.svg" alt="Fairway" width="240">
</p>

# Fairway

**A durable engineering control and evidence plane across agent harnesses.**

Fairway is the local, harness-neutral coordination and engineering-record plane
for work that spans coding agents, provider sessions, worktrees, source control,
and CI/CD. It keeps tasks, ownership, decisions, evidence, reviews, waits, and
readiness durable when an agent stops, a context window compacts, or execution
moves to another tool.

It gives a software team a durable answer to six practical questions:

1. What work was authorized, and who owns it now?
2. Which material decisions shaped the work?
3. What evidence supports the current claim?
4. Which evidence agrees, conflicts, or remains unavailable?
5. Which independent judgment or promotion authority is still required?
6. What outcome or controlled lesson was retained after promotion?

## The Working Model

Fairway supports a deliberate shift between two ways of working:

- collaborate while the problem, constraints, acceptance boundary, or
  verification method still needs to be discovered; and
- delegate when one slice has a clear owner, known dependencies, explicit
  acceptance, and an independently checkable result.

Evidence or independent review can send delegated work back into collaborative
diagnosis. Promotion remains a separate action owned by Git, CI/CD, operators,
or another accountable authority.

```text
collaborate -> bound -> delegate -> verify -> challenge -> promote -> retain
```

This model operates across architecture, implementation, review, integration,
deployment, and operations. It is not another SDLC phase or a claim that a
longer prompt makes ambiguous work safe to delegate. Read the canonical
[collaborative delegation model](docs/design/collaborative-delegation.md).

Coding agents and runtimes execute individual runs. Source control owns commits
and merges. CI/CD owns builds and deployments. Issue trackers own stakeholder
planning, and identity systems own access. Fairway connects their engineering
facts without taking their authority, so delivery does not depend on chat memory
or one provider session.

## Across Replaceable Harnesses

Fairway can now retain versioned, source-qualified external-run,
execution-observation, and evaluator-result records from execution surfaces.
One task may correlate with zero, one, or many runs without merging run state
into task state. A shared CLI/dashboard projection reports named evaluator
cohorts, explicit missing usage/cost denominators, and cited repeated-pattern
advisories.

```bash
fairway contract harness-record --format json
fairway harness ingest --file records.json
fairway harness runs --task <task-id> --format json
fairway harness report --task <task-id> --format json
```

The record ingestion is implemented. Outcome-efficiency and trajectory
analysis are experimental and were validated only in one bounded GPUaaS
consumer pilot. They do not redirect a provider, rank models, accept evidence,
satisfy review, or authorize promotion. See the
[interoperability contract](docs/design/harness-interoperability.md) and
[pilot assessment](docs/assessment/gpuaas-harness-trajectory-pilot-2026-08-21.md).

## Start Here

Install the current release:

```bash
brew tap fairway-run/tap
brew install --cask fairway
```

Or build from source:

```bash
go install github.com/fairway-run/fairway/cmd/fairway@latest
```

Initialize a repository and inspect the resulting control state:

```bash
fairway init
fairway config validate
fairway doctor
fairway ready
fairway dashboard
```

The single-project dashboard opens with active work and evidence-backed project
state, then links to task navigation, cited Quality Records, delivery reports,
control analysis, and diagnostics.

Continue with the [quickstart](docs/quickstart.md) to complete one bounded work
item and inspect its evidence and decision record. The clean-repository
rehearsal completed the command path in less than one second of machine
execution and documented the Git baseline and bootstrap-commit prerequisites.

## The Coordination Record

Fairway keeps ordinary agent work connected to explicit engineering controls:

| Control | Fairway record | Why it matters |
|---|---|---|
| Intent | task, owner, scope, acceptance | Agents start from bounded work rather than reconstructed chat. |
| Decisions | task decision and context packet | Material choices survive provider replacement and context compaction. |
| Evidence | command result and artifact reference | A claim can be checked without treating generated prose as proof. |
| Verification | cited evidence state and verifier boundary | Passing, missing, unavailable, and conflicting checks stay distinct. |
| Independent judgment | routed review and review wait | Required human or independent review stays visible and attributable. |
| Promotion | workflow, merge-ready, release, and deploy guards | Reversible implementation remains distinct from consequential release or live action. |
| Outcomes and lessons | structured outcome, friction, and controlled-improvement links | Promotion is not treated as the end of the quality record. |

Sessions, checkpoints, handoffs, waits, notifications, lanes, worktrees, and
dashboards make ownership and next actions visible across runs. Governance,
assurance, and a cited Quality Record are useful consequences of this technical
record; they are not Fairway's product category.

## The Product Model

Fairway builds four distinct capabilities on one local coordination record:

1. **Work coordination:** tasks, lanes, worktrees, sessions, ownership, waits,
   and deterministic next actions across runs.
2. **Engineering continuity:** track memory, cold starts, waits, and handoffs
   that survive provider replacement and context loss.
3. **Operating knowledge:** source-grounded project synthesis and reusable rule
   packs with provenance and freshness checks.
4. **Assurance and execution profiles:** bounded evidence mapping is
   implemented; specialized profiles such as large-migration execution remain
   explicitly planned until released.

These capabilities compose, but they do not share authority. Memory is curated
operating context, knowledge is derived until promoted, rule packs define
reusable expectations, and assurance reports readiness without approving it.

## What Is Implemented

The current binary provides:

- a local-first Go CLI and SQLite execution store;
- task, session, checkpoint, decision, evidence, handoff, review, wait, and
  notification records;
- a cited, read-only Quality Record spanning intent, decisions, production
  context, evidence, verification, judgment, promotion, outcomes, and lessons;
- durable task-to-commit association plus structured outcome and attributable
  control-friction records for forward measurement;
- workflow, review, merge-readiness, release, and reconciliation checks;
- database-backed track memory and deterministic engineering-knowledge packets;
- local rule-pack loading, matching, lint, and evidence expectations;
- assurance profiles and signed offline evidence packages with bounded claims;
- provider-neutral adapters and structured packets with bounded authority;
- versioned harness-run, observation, and evaluator-result ingestion with
  privacy-bounded replay/conflict handling;
- experimental named-cohort outcome-efficiency and cited trajectory advisory
  readback shared by CLI and task detail;
- read-only wall, board, task-detail, diagnostics, and report views;
- multi-project and shared-team pilot surfaces with explicit identity and
  write-mode guards;
- release binaries, Homebrew distribution, embedded help, and public docs.

See [Product](docs/product.md) for the evidence-backed capability status table.
Experimental and planned surfaces are labeled there rather than presented as
default production behavior.

## How Fairway Composes

- **Coding agents** implement, investigate, review, and report. Fairway records
  their attachment to durable work; it does not proxy their credentials or
  treat their transcripts as authority.
- **Git and hosting platforms** own commits, branches, pull requests, and remote
  promotion. Fairway checks and records that posture; it does not silently push
  or merge.
- **CI/CD systems** execute builds, tests, deployments, and release workflows.
  Fairway records evidence and handbacks; it is not a runner.
- **Issue trackers** own stakeholder planning. Fairway can import, link, or
  mirror planning context while its DB remains authoritative for execution
  state.
- **Agent orchestrators** may start or steer provider work. Fairway supplies
  deterministic state, packets, waits, and guards; orchestration output does
  not become approval or provenance by itself.

```text
Fairway: tasks / lanes / worktrees / sessions / reviews / evidence / readiness
                         |
                         | bounded context and correlated facts
                         v
Coding agents and optional Seaway: individual run execution
                         |
                         v
Models / tools / bounded execution environments

Git and forges own source and merge. CI/CD owns verification and deployment.
Trackers own planning. Identity systems own authentication and authorization.
```

[Seaway](docs/ecosystem.md#fairway-and-optional-seaway) is an optional run-control
product, not a Fairway requirement. Fairway works directly with existing coding
agents and execution surfaces; Seaway can serve other callers without Fairway.

## Authority Boundary

Fairway does not silently claim work, approve reviews, accept risk, merge,
push, deploy, release, mutate live environments, or store credentials. Its
dashboard is not a privileged approval surface. Advisory output must cite
durable facts and remains non-authoritative.

Read the full [product boundary](docs/design/product-boundaries.md) before
adding adapters, shared write surfaces, or automation around consequential
actions.

## Documentation

- [Quickstart](docs/quickstart.md): first bounded work item
- [Quality Record demo](docs/quality-record-demo.md): inspect one cited record
  and its measurement boundary
- [Product](docs/product.md): product promise, principles, and capability status
- [Concepts](docs/design/concepts.md): canonical concept map
- [Agent guide](docs/agent-guide.md): complete operating workflow
- [Architecture](docs/architecture.md): components and data flow
- [Ecosystem](docs/ecosystem.md): responsibility and composition boundaries
- [Integrations](docs/integrations.md): supported adapter and tool surfaces
- [Product boundaries](docs/design/product-boundaries.md): authority and non-goals
- [Configuration reference](docs/config-reference.md): exhaustive settings
- [Public docs](https://fairway.run): published portal

Maintainers should begin with [AGENTS.md](AGENTS.md). The complete documentation
disposition baseline is recorded in
[the FW-324 inventory](docs/assessment/fairway-documentation-inventory-2026-07-11.md).

## Status

Fairway is an actively developed local-first product with released binaries and
documented internal consumer use. That evidence is not a claim of broad market
adoption or regulatory compliance.

## License

Apache-2.0. See [LICENSE](LICENSE).
