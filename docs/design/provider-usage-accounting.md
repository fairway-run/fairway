# Provider Usage Accounting

Status: first slice implemented in `FW-123`

Fairway should be able to report how much provider capacity was used per task,
session, epic, day, role, and provider when that data is available. This is an
operational insight signal, not a task completion gate.

## Goals

- Attribute provider usage to Fairway tasks and sessions.
- Support Codex, Claude, Gemini, shell, tmux, and future providers without
  making Fairway depend on provider APIs.
- Make provider usage comparable enough to identify expensive task classes,
  expensive workflows, and opportunities to improve tools, prompts, runbooks,
  automation, or delegation strategy.
- Preserve a privacy boundary: record counts and source metadata, not prompts,
  transcripts, secrets, model inputs, or generated content.
- Allow reports to explain which tasks consumed unusually high usage.

## Core Principle

Fairway should use a provider adapter model, not one universal parser.

```text
provider-specific runtime output -> provider adapter -> normalized Fairway usage event
```

Fairway core owns the normalized schema, persistence, task/session attribution,
rollups, and reporting. Provider adapters own the provider-specific mechanics:
reading usage metadata, parsing CLI summaries, sampling running totals, or
recording manual usage values. This keeps Fairway maintainable as providers
change output formats and usage semantics.

## Data Sources

Providers expose usage differently. Fairway should support both patterns:

1. Explicit per-run usage from the provider, when available.
2. Running-total snapshots captured by an adapter at task/session start and
   completion.

When only running totals are available, Fairway derives usage as:

```text
derived_delta = completed_snapshot - started_snapshot
```

The record must include a confidence/source field so reports can distinguish
provider-reported totals from locally derived estimates.

## Implemented Fields

Provider usage records should be provider-neutral:

| Field | Meaning |
|---|---|
| `provider` | Provider label such as `codex`, `claude`, `gemini`, or `shell`. |
| `external_session_id` | Provider-side session/thread/run id, when known. |
| `session_id` | Fairway session id, when known. |
| `task_id` | Fairway task id receiving attribution. |
| `role` | Fairway role/lane receiving attribution. |
| `phase` | Optional work phase such as `implementation`, `review`, `ci`, `deploy`, or `uat`. |
| `started_at` | Start timestamp for the measured window. |
| `completed_at` | End timestamp for the measured window. |
| `started_token_snapshot` | Optional provider running total at start. |
| `completed_token_snapshot` | Optional provider running total at completion. |
| `input_tokens` | Optional provider-reported input tokens. |
| `cached_input_tokens` | Optional provider-reported cached input tokens. |
| `uncached_input_tokens` | Optional derived or provider-reported uncached input tokens. |
| `output_tokens` | Optional provider-reported output tokens. |
| `reasoning_tokens` | Optional provider-reported reasoning tokens. |
| `total_tokens` | Optional provider-reported or derived total tokens. |
| `source` | `provider_reported`, `derived_snapshot`, `manual`, or `unknown`. |
| `confidence` | `exact`, `estimated`, or `unknown`. |
| `elapsed_seconds` | Optional measured elapsed seconds. |
| `model` | Optional provider model label. |
| `metadata_json` | Optional small key/value metadata. Must not contain prompts, transcripts, secrets, inputs, outputs, messages, or generated content. |

## Adapter Boundary

Fairway core should not poll provider APIs directly. Provider/session adapters
should translate provider-specific usage into Fairway records or checkpoints.

Expected adapter behavior:

| Adapter | Expected source |
|---|---|
| Codex | `response.completed` usage or equivalent structured output when available; otherwise start/end usage snapshots. |
| Claude | CLI/session usage summary if exposed; otherwise tmux transcript summary or manual snapshot. |
| Gemini | Provider usage metadata if exposed; otherwise start/end snapshots. |
| tmux/shell | Elapsed time and optional manually supplied usage only. |

Adapters may emit partial records. Unknown fields should remain null or
`unknown`; adapters must not invent zero values for unavailable usage.

Codex should be the first concrete adapter because it can expose detailed token
usage, including cached input tokens. Cached tokens are important for cost
planning: a task with high input tokens and a high cache ratio has different
optimization needs than a task with the same input volume and no cache benefit.

The Codex adapter must not make Fairway core depend on private Codex local
state such as `~/.codex/*.sqlite`, auth caches, transcripts, prompts, generated
content, or undocumented log formats. Those files may change across Codex
updates. The supported boundary is that Codex-specific tooling supplies usage
values to Fairway through `fairway record usage` or `provider-event.sh`.

Acceptable Codex ingestion paths:

- provider-reported response usage supplied by a Codex wrapper, hook, or
  observable event surface;
- explicit start/end running-total snapshots supplied by the caller;
- manual values entered during session closeout when no structured usage is
  available.

Private local Codex storage may be useful for one-off diagnosis, but it is not
an API contract and should not be embedded in Fairway core.

## CLI Contract

Provider adapters record usage through Fairway commands. The smallest direct
path is:

```bash
fairway record usage <task-id> \
  --provider codex \
  --session-id <fairway-session-id> \
  --external-session-id <provider-session-id> \
  --role backend \
  --phase implementation \
  --source provider_reported \
  --confidence exact \
  --input-tokens 12000 \
  --cached-input-tokens 9000 \
  --output-tokens 2200 \
  --reasoning-tokens 600 \
  --total-tokens 14800 \
  --elapsed-seconds 420 \
  --model gpt-5-codex
```

When only running totals are available:

```bash
fairway record usage <task-id> \
  --provider codex \
  --source derived_snapshot \
  --confidence estimated \
  --started-token-snapshot 100000 \
  --completed-token-snapshot 108500
```

When usage is unavailable, adapters may still record attribution with unknown
counts:

```bash
fairway record usage <task-id> \
  --provider shell \
  --session-id <session-id> \
  --source unknown \
  --confidence unknown
```

Unknown numeric fields are stored as `NULL`, not `0`.

`examples/session-adapters/provider-event.sh` accepts the same usage fields and
forwards them to `fairway record usage` after refreshing the session record.
This keeps Fairway core provider-neutral while giving Codex, Claude, Gemini,
tmux, and shell adapters a stable ingestion point.

## Reporting

Reports should show usage only when available. Missing usage must display as
`unknown`, not zero.

Useful rollups:

- by task;
- by epic;
- by role;
- by provider;
- by day;
- by task class or kind;
- by validation phase, such as CI, CD, UAT, review, or implementation;
- by external tracker issue when a Plane/Jira/Linear link exists.

Usage should help retrospectives and planning. It should not by itself mark a
task pass/fail, block completion, or imply quality.

Implemented visibility:

- `fairway task-detail <task-id>` shows usage events and provider rollups.
- `fairway usage report --by provider|task|epic|role|day|kind|phase` shows
  attribution rollups.
- `/tasks/<task-id>` shows provider usage for the task when present.
- `/reports` shows provider, role, kind, phase, and day usage rollups for the
  selected report window.

Pricing, budget enforcement, and cost accounting are intentionally out of scope
for this slice.

Useful questions:

- Which task classes consume the most provider tokens?
- Which tasks have poor cached-token ratios and would benefit from better
  context reuse or prompt/layout changes?
- Which workflows spend high tokens during CI/deploy idle windows?
- Which provider is most efficient for implementation, review, ops monitoring,
  or documentation tasks?
- Which expensive tasks should become scripts, checks, runbooks, dashboards, or
  reusable Fairway/agent tooling?

## Compatibility

`provider_usage_events` is append-only telemetry. Existing task, evidence,
review, session, and checkpoint rows do not require backfill. Missing usage
means unknown usage, not zero usage.
