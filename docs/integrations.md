# Integrations

This page lists checked-in Fairway integration surfaces and their current
status. Product names appear only where a specific adapter, protocol, or
configuration value requires them.

## Status Labels

- **Implemented:** current command/script/config path is covered by repository
  validation.
- **Experimental:** bounded pilot; not a default production authority path.
- **Design only:** documented contract without a supported runtime adapter.
- **Compatibility:** retained for existing consumers, not recommended for new
  adoption.

## Provider And Utility Sessions

| Surface | Status | Use | Boundary |
|---|---|---|---|
| `fairway session upsert`, `session launch`, `work start` | Implemented | Attach a provider or utility to durable work | Registration does not claim hidden authority or prove provider delivery |
| `examples/session-adapters/shell.sh` | Implemented example | Run a shell-backed provider command with session metadata | Command and credentials remain operator-owned |
| `examples/session-adapters/tmux.sh` | Implemented example | Attach a durable tmux pane and transcript reference | Transcript is optional context, not task authority |
| `examples/session-adapters/zellij.sh` | Implemented example | Attach a zellij-backed lane | Same session/checkpoint boundary as tmux |
| `examples/session-adapters/provider-event.sh` | Implemented example | Map started/waiting/completed/failed provider events into Fairway | Event mapping cannot close reviews or promote work |
| `examples/session-adapters/codex-usage-adapter.sh` | Implemented adapter | Map supported Codex structured token-count events into bounded usage rows | Count ingestion only; no prompt, transcript, tool-body, cost, or completion authority |
| OTel ingestion through `provider-otel-ingest.sh` | Implemented adapter | Normalize supported provider telemetry, including configured Claude Code OTel shapes | Allow-listed usage metadata only; no private provider state scraping |

Use `fairway doctor` and provider capability readiness before relying on a
provider surface for thread steering, Git, browser, network, or filesystem
operations.

## Harness Records And Evaluators

| Surface | Status | Use | Boundary |
|---|---|---|---|
| `contract harness-record` | Implemented | Inspect the versioned external-run, observation, evaluator-result, and batch contracts | Describes accepted records; does not advertise a provider/protocol adapter |
| `harness ingest`, `runs`, and `record` | Implemented | Append and inspect source-qualified facts from an execution surface | Does not run a harness, import private context, change task state, accept evidence, or create review |
| `harness report` and task-detail analysis | Experimental | Project one named compatible evaluator cohort, missing denominators, and cited trajectory patterns | Report-only; no ranking, provider command, redirect, review, or promotion authority |
| GPUaaS configured-packet fixture/pilot | Validated practice | Demonstrate replay, controlled repeated-pattern calibration, and a new passing evaluator observation | One bounded consumer; not general effectiveness or adapter availability proof |
| MCP / ACP / A2A harness-record mapping | Design only | Describe how protocol-owned run/tool/task facts could map into the neutral record | No supported MCP, ACP, or A2A adapter is claimed |
| OpenTelemetry harness-record mapping | Design only | Correlate trace identity or measurements to runs/observations | Separate from the implemented allow-listed provider-usage adapter above; no general OTel-to-harness-record adapter is claimed |
| Fairway-Seaway adapter | Design only | Correlate optional Seaway admission/events/results to Fairway tasks and records | No released transport, shared state, runtime approval inheritance, or supported adapter |

See [Harness interoperability](design/harness-interoperability.md) for the
schema, privacy, compatibility, and authority contract.

## Source Control

| Surface | Status | Use | Boundary |
|---|---|---|---|
| Git worktree/branch checks | Implemented | Compare task and session posture with the current repository | Git remains authoritative for files, commits, branches, and remotes |
| `workflow check`, `merge-ready`, `workflow closeout` | Implemented | Report dirty, unpushed, review, evidence, session, and promotion blockers | Reports do not merge or push |
| `worktree setup`, review checkout, lane runtime | Implemented | Prepare explicit local execution surfaces | Destructive cleanup and remote promotion remain explicit operator actions |

Remote forge APIs are not required for core operation. Where release or CI
workflows use GitHub, the named service is an implementation detail of those
checked-in workflows, not a Fairway ecosystem category.

## CI/CD And Deterministic Utilities

| Surface | Status | Use | Boundary |
|---|---|---|---|
| `examples/session-adapters/ci-monitor.sh` | Implemented example | Track a bounded CI command as a utility session | Fairway monitors; the CI system executes |
| delivery resources and deploy runs | Implemented | Record CI/deploy/UAT/release run identity, status, evidence, and handback | Does not authorize deploy or release |
| environment rehearsal packets | Implemented | Render/instantiate preflight expectations and waits | Packet creation does not run commands or mutate an environment |
| `release verify` and release-run packets | Implemented | Check release assets, version, source, and Homebrew posture | Publish/tag/restart remain separately authorized actions |
| rule-pack CI examples for GitHub Actions | Implemented example | Validate rule packs in the named CI implementation | Example does not make Fairway a CI runner |

## Issue Systems

| Surface | Status | Use | Boundary |
|---|---|---|---|
| tracker links and reports | Implemented | Attach planning-system identity and report mapping state | Fairway DB remains execution truth |
| Plane tracker commands | Experimental | Exercise import/link/export semantics against the checked-in Plane adapter | Credentials and remote apply require explicit config; no generic issue-system parity claim |
| Other issue-system adapters | Design only | Provider-neutral contract is documented in `design/issue-tracker-integrations.md` | Do not infer support from a named example in historical docs |

## Identity, Proxy, And Shared-Team Surfaces

| Surface | Status | Use | Boundary |
|---|---|---|---|
| read-only dashboard mode | Implemented | Share Fairway state without dashboard mutation controls | Network, domain, identity, and proxy policy remain deployment-owned |
| shared-team server read-only API | Implemented pilot | Serve bounded task/read-model data on loopback | Non-loopback exposure requires separately reviewed deployment controls |
| API-token identity/command authorization guard | Experimental | Verify configured command-scoped roles for server pilots | Tokens come from environment; identity does not imply unrestricted authority |
| append-only and guarded write API pilots | Experimental | Record bounded evidence/checkpoint/status/review commands with idempotency/conflict checks | Not a generic dashboard write surface; no merge/deploy/live commands |
| Cloudflare Access trusted-proxy config value | Implemented metadata | Document one supported upstream proxy pattern for read-only sharing | Core JWT/header verifier remains unimplemented; upstream proof and origin isolation are required |

## Notifications

Fairway records notification intent, attempted delivery, delivery proof,
failure, and acknowledgement through configured provider/notifier adapters.
Dry-run/logging paths are safe defaults. Real delivery requires explicit target
mapping, environment-sourced credentials, redaction, rate limits, and audit
evidence. The dashboard remains read-only and does not send messages.

## Structured Output

Commands that support `--json` expose agent-consumable structured output. Treat
the documented schema and command result as the contract; do not parse human
prose when structured output exists. Structured output reports Fairway state
and does not expand command authority.

## Integration Checklist

Before enabling an adapter:

1. name the external owner and authoritative facts;
2. run the capability/preflight command on the actual execution surface;
3. configure credentials through environment or the owning secret store;
4. verify project/task scope and command-scoped role;
5. test failure, retry, duplicate, redaction, and acknowledgement behavior;
6. record evidence and rollback/disable steps;
7. keep the adapter advisory until measured evidence justifies stronger policy.

See [Ecosystem](ecosystem.md) for the product-neutral responsibility model and
[Product boundaries](design/product-boundaries.md) for authority invariants.
