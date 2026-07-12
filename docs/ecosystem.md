# Fairway Ecosystem

Fairway is the independent engineering record and control layer for
agent-driven delivery. It composes with the systems that create, schedule,
version, verify, discuss, secure, and promote software; it does not absorb
their responsibilities.

This page defines durable categories. It intentionally does not name, rank, or
compare individual products.

## Responsibility Map

| Category | Owns | Sends to Fairway | Receives from Fairway | Does not delegate to Fairway |
|---|---|---|---|---|
| Coding agent | Investigation, implementation, local testing, bounded review | Session lifecycle, checkpoints, evidence, handback, bounded usage metadata | Task packet, acceptance, cited facts, required checks, unresolved waits | Credentials, self-approval, risk acceptance, merge/deploy/live authority |
| Agent orchestrator | Provider scheduling, thread/process steering, capacity decisions | Delivery proof, provider state, exceptions | Ready work, deterministic next action, capability/routability status | Product truth, approval, hidden task mutation, automatic promotion |
| Source control | Files, commits, branches, history, remote collaboration | Branch/commit/remote facts | Expected branch/worktree posture, promotion checks | Repository integrity, merge execution, access control |
| CI/CD | Build, test, package, deploy, release execution | Run status, immutable links, result evidence, rollback/closeout | Expected validation, deploy-run packet, unresolved handback | Test execution, artifact signing, deploy authorization |
| Issue system | Roadmap, stakeholder planning, discussion, prioritization | Imported definition or durable link | Execution status/export summary | Fairway runtime state, session truth, evidence, review materialization |
| Identity and security controls | Authentication, authorization, secrets, network and origin policy | Verified actor/proxy facts and policy result | Required role/command boundary, audit context | Credential custody, identity proof generation, public gateway operation |
| Fairway | Intent, decisions, evidence references, reviews, waits, execution attachment, control checks, promotion posture | Structured facts from every category | Deterministic read models, packets, reports, guards, handbacks | Implementation, CI execution, planning authority, IAM, autonomous promotion |

## The Independent Record

Fairway does not copy every external system into its DB. It stores the minimum
engineering record needed to connect them:

- stable task and ownership state;
- material decisions and cited Fairway facts;
- command results and safe artifact references;
- attributable domain reviews;
- provider/utility session lifecycle;
- waits, notifications, handbacks, and next actions;
- source/release/deploy posture needed by configured guards.

The external system remains authoritative for the thing it performs or hosts.
A commit SHA is a source-control fact. A CI URL and result are CI facts. An
identity assertion is useful only after the configured trust boundary verifies
it. Fairway links these facts without turning a summary into provenance.

## Composition Rules

### Facts Cross Boundaries, Authority Does Not

An adapter can report that a provider session ended, a notification was
delivered, or a CI run passed. That fact does not approve review, accept risk,
or authorize promotion.

### Commands Stay Scoped

Write-capable shared-team surfaces use command-shaped APIs and explicit roles.
Fairway does not expose generic row mutation or arbitrary SQL as an integration
contract.

### Failure Remains Visible

If an adapter cannot deliver, verify identity, reach a provider, or read an
external result, Fairway records a failure or durable wait. It does not infer
success from silence.

### Privacy Is A Contract

Provider and external adapters may report bounded metadata. They do not store
raw prompts, private transcripts, tool bodies, generated-content dumps,
credentials, or secret values by default.

### Promotion Is External And Explicit

Fairway can report merge-, release-, deploy-, or live-readiness. The owning
source-control, CI/CD, security, and operator surfaces still authorize and
perform the consequential action.

## Where To Go Next

- [Integrations](integrations.md): implemented adapters and technical setup
- [Product boundaries](design/product-boundaries.md): durable authority limits
- [Architecture](architecture.md): Fairway component and data flow
- [Provider capability readiness](design/provider-surface-capability-readiness.md): capability probes and fail-closed provider routing
- [Shared-team server API](design/shared-team-server-api.md): command and identity boundary
