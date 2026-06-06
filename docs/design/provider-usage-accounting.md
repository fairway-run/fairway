# Provider Usage Accounting

Status: backlog design note

Fairway should be able to report how much provider capacity was used per task,
session, epic, day, role, and provider when that data is available. This is an
operational insight signal, not a task completion gate.

## Goals

- Attribute provider usage to Fairway tasks and sessions.
- Support Codex, Claude, Gemini, shell, tmux, and future providers without
  making Fairway depend on provider APIs.
- Preserve a privacy boundary: record counts and source metadata, not prompts,
  transcripts, secrets, model inputs, or generated content.
- Allow reports to explain which tasks consumed unusually high usage.

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
| `output_tokens` | Optional provider-reported output tokens. |
| `total_tokens` | Optional provider-reported or derived total tokens. |
| `usage_source` | `provider_reported`, `derived_snapshot`, `manual`, or `unknown`. |
| `confidence` | `exact`, `estimated`, or `unknown`. |

## Adapter Boundary

Fairway core should not poll provider APIs directly. Provider/session adapters
should translate provider-specific usage into Fairway records or checkpoints.

The same model can be used by:

- provider-event adapters;
- tmux transcript/watch adapters that see provider usage summaries;
- manual session closeout checkpoints;
- future Codex/Claude/Gemini-specific watchers.

## Reporting

Reports should show usage only when available. Missing usage must display as
`unknown`, not zero.

Useful rollups:

- by task;
- by epic;
- by role;
- by provider;
- by day;
- by external tracker issue when a Plane/Jira/Linear link exists.

Usage should help retrospectives and planning. It should not by itself mark a
task pass/fail, block completion, or imply quality.

## Future Task

Tracked by `FW-123`.
