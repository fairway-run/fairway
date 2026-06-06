# Provider Usage Accounting

Status: backlog design note

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

## Proposed Fields

Provider usage records should be provider-neutral:

| Field | Meaning |
|---|---|
| `provider` | Provider label such as `codex`, `claude`, `gemini`, or `shell`. |
| `session_id` | Fairway session id or external provider session id. |
| `task_id` | Fairway task id receiving attribution. |
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
| `usage_source` | `provider_reported`, `derived_snapshot`, `manual`, or `unknown`. |
| `confidence` | `exact`, `estimated`, or `unknown`. |

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
analysis: a task with high input tokens and a high cache ratio has different
optimization needs than a task with the same input volume and no cache benefit.

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

Useful questions:

- Which task classes consume the most provider tokens?
- Which tasks have poor cached-token ratios and would benefit from better
  context reuse or prompt/layout changes?
- Which workflows spend high tokens during CI/deploy idle windows?
- Which provider is most efficient for implementation, review, ops monitoring,
  or documentation tasks?
- Which expensive tasks should become scripts, checks, runbooks, dashboards, or
  reusable Fairway/agent tooling?

## Future Task

Tracked by `FW-123`.
