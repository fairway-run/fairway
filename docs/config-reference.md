# Config reference

Fairway reads `.fairway/config.toml` from the repo root by default. Override with `--config <path>` or `FAIRWAY_CONFIG=<path>`.

Track memory does not add provider configuration. `fairway memory` stores
curated resume summaries and source fact references in the Fairway DB. It does
not store provider credentials, raw prompts, transcripts, cookies, tokens, or
private provider state.

Supply-chain provenance exports are derived from existing task, evidence,
review, session, checkpoint, batch, usage, and release records. There is no
provenance-specific credential, provider, or content-capture setting in the
current config. See
[supply-chain-provenance.md](design/supply-chain-provenance.md) for the privacy
boundary and future export model.

Tamper-evident evidence manifests are also config-free. Use
`fairway provenance manifest --path <file>` on selected exported provenance
bundles or redacted artifacts. The command hashes files and records path/hash
metadata only; it does not store artifact contents, secrets, or credentials in
Fairway.

## Full schema

```toml
[fairway]
project_name = "myrepo"                # default: basename of repo root; must be unique in registry
db_path = ".fairway/state.db"          # relative to repo root
queue_source = "inline"                # "inline" | "yaml:<path>" | "json:<path>"
main_branch = "main"                   # base branch worktrees branch off of
task_id_pattern = "^[A-Z]+-[0-9]+$"    # regex enforced by add/import/update
local_artifact_paths = ["dist/fairway"] # optional local evidence artifact dirs

[dashboard]
listen = "127.0.0.1:7878"
auto_open = true                       # open browser when `fairway dashboard` starts

[worktrees]
root = "../worktrees"
naming = "{repo}-{role}"
review_branch_naming = "review/{role}"

[sessions]
default_backend = "shell"              # shell | tmux | zellij
stale_after = "12h"

[coordinator]
max_primary_tracks = 1
max_sidecar_tracks = 1
max_review_tracks = 1
checkpoint_stale_after = "24h"
notification_ack_timeout = "24h"

[[roles]]
name = "backend"
branch = "agent/backend"
provider = "claude"                    # informational; not enforced

[[roles]]
name = "ui"
branch = "agent/ui"
provider = "codex"

[[review_routes]]
match = "doc/api/**"
reviewer = "arch"

[[review_routes]]
match = "doc/governance/**"
reviewer = "governance"

[[provider_targets]]
domain = "security"
provider = "codex"
type = "thread"
target = "codex-thread-id-or-adapter-target"

[[advisory_provider_adapters]]
name = "local-rules"
provider = "ollama"
type = "local_ollama"                 # noop | rules-only | local_ollama | local_llamacpp | openai-compatible | codex | claude | gemini
mode = "advisory"                     # advisory | report_only | disabled
trust = "low"                         # low | medium | high
model = "llama3.1"
endpoint_env = "FAIRWAY_OLLAMA_ENDPOINT"
capabilities = ["summarize_evidence", "rank_ready_tasks"]
allowed_actions = ["inspect_task", "render_packet"]

[[external_notifiers]]
name = "control-log"
type = "log"                           # noop | log
mode = "dry_run"                       # dry_run | disabled
target_env = "FAIRWAY_NOTIFY_LOG"
domains = ["coordinator", "ops"]
template_name = "control_room_handoff"

[[workstream_profiles]]
name = "platform-foundation"
task_kinds = ["architecture-map", "boundary-guard", "facade"]
dashboard_groups = ["architecture maps", "boundary guards", "facades"]
review_domains = ["architecture", "security"]
route_samples = ["doc/api/openapi.yaml", "cmd/api/routes.go"]

[[review_profiles]]
name = "micro-slice"
mode = "advisory"                    # advisory | blocking; default blocking
match_kinds = ["task"]
match_risk_levels = ["low"]
match_tags = ["review:micro"]
required_review_domains = ["governance"]
waive_review_domains = ["backend"]
defer_review_domains = ["ops"]
safe_iteration_zone = true
safe_iteration_defect_class = "harness"
safe_iteration_control = "non-live disposable boundary"
extra_reviewer_rationale = "governance catches evidence contract drift"
process_hypothesis = "one governance review catches evidence drift without full matrix overhead"
outcome_metrics = ["defects_caught", "cycle_time", "avoided_unsafe_actions"]

[[workstream_profiles.gates]]
name = "security-review"
group = "security gates"
mode = "advisory"                     # advisory | blocking | report_only
task_kinds = ["facade"]               # optional; omit to apply to all profile task kinds
evidence_type = "security-review"
required_evidence_count = 1
accepted_results = ["pass", "partial"]
artifact_required = true
owner_signoff_required = false
expires_after = "720h"
description = "Security review evidence should be attached before release readiness."

[[packet_templates]]
profiles = ["platform-foundation"]
name = "architecture-map"
required_fields = ["scope", "current_owner", "target_owner", "migration_risk", "acceptance"]
optional_fields = ["source_doc"]

[states]
allowed = ["todo", "in_progress", "blocked", "done"]
terminal = ["done"]
# transitions = [["todo","in_progress"], ...]   # optional; permissive if omitted

[gates]
require_evidence_before_done = false
require_review_before_done = false
require_handoff_before_merge_ready = false
require_blocked_reason = true
allow_force_without_reason = false

[task_kinds]
allowed = ["epic", "story", "task", "bug", "spike"]   # optional; free-text if omitted
default = "task"

[task_priorities]
default = 2
levels = [
  { rank = 0, label = "P0", description = "drop everything" },
  { rank = 1, label = "P1", description = "this sprint" },
  { rank = 2, label = "P2", description = "soon" },
  { rank = 3, label = "P3", description = "eventually" },
]
```

## Section reference

### `[fairway]`

| Key | Type | Default | Description |
|---|---|---|---|
| `project_name` | string | basename of repo root | Label used by the multi-project dashboard. Must be unique across `~/.fairway/registry.toml`; multiple registry entries may share one repo path only when their names and DB/config identities differ. |
| `db_path` | string | `.fairway/state.db` | SQLite DB path. Relative to repo root unless absolute. Fairway opens SQLite with WAL mode and a bounded 5s busy timeout so short local write bursts can wait for the current writer instead of failing immediately with `SQLITE_BUSY`. |
| `queue_source` | string | `inline` | `inline` (DB-only task definitions), `yaml:<path>` or `json:<path>` (active backlog definition for import/reconciliation; runtime execution state still lives in the DB). |
| `main_branch` | string | `main` | Base branch new worktree branches are created from. |
| `task_id_pattern` | string | `^[A-Z]+-[0-9]+$` | Regex enforced for task IDs. GPUaaS parity configs use a wider pattern for legacy IDs such as `A-DEMO-UAT-001` and `A-PROV-REMOVE-SSH`. |
| `local_artifact_paths` | []string | `[]` | Optional repo-relative directories or files that may appear as untracked local evidence artifacts without making `merge-ready`, `workflow check`, or `workflow closeout` dirty. Recorded Fairway evidence artifact paths are also allowed for that task. Tracked source changes and arbitrary untracked files remain dirty. |

### `[dashboard]`

| Key | Type | Default | Description |
|---|---|---|---|
| `listen` | string | `127.0.0.1:7878` | HTTP listen address. Bind to `127.0.0.1` unless you understand the auth implications. |
| `auto_open` | bool | `true` | Open the system browser when `fairway dashboard` starts. |
| `read_only` | bool | `false` | Disable dashboard mutation endpoints and hide mutation controls. Use for shared dashboard views behind an identity-aware proxy. |
| `trusted_proxy` | string | `none` | Deployment note for trusted proxy mode. Supported values: `none`, `cloudflare_access`, `identity_aware_proxy`. Fairway does not trust identity headers unless the origin is exclusively reachable through that proxy and JWT/header verification is handled. See [Trusted Proxy Identity Verification](design/trusted-proxy-identity-verification.md) for the planned verifier model. |

Fairway has one dashboard. `/` serves the wall view, `/board` serves the
operator board, `/board?tab=diagnostics` serves diagnostics, `/reports` serves
retrospectives, and `/tasks/<id>` serves task detail. There is no dashboard
version selector; `[dashboard] surface` is not part of the active config
contract.

For shared read-only viewing, keep `listen = "127.0.0.1:7878"`, set
`read_only = true`, and expose the origin only through a trusted tunnel/proxy.
See [dashboard-sharing.md](design/dashboard-sharing.md).

### `[worktrees]`

| Key | Type | Default | Description |
|---|---|---|---|
| `root` | string | `../worktrees` | Parent directory for per-role worktrees. Relative to repo root. |
| `naming` | string | `{repo}-{role}` | Worktree directory name template. `{repo}` is the basename of the primary checkout, `{role}` is the role name. |
| `review_branch_naming` | string | `review/{role}` | Local branch template used by `fairway review checkout`. |

### `[sessions]`

| Key | Type | Default | Description |
|---|---|---|---|
| `default_backend` | string | `shell` | Default backend for optional `fairway session launch`. Core queue operations do not require launch adapters. |
| `stale_after` | duration | `12h` | Session reconciliation threshold for missing PID/backend sessions. |

Provider runtime watchers do not need provider API configuration in Fairway
core. Keep provider credentials, polling, and API-specific state outside the
Fairway config; adapters should write generic `session upsert`, `checkpoint
record`, `record evidence`, and `record handoff` events using provider labels
such as `codex`, `claude`, `gemini`, or `shell`.

### `[[review_profiles]]`

Review profiles define risk-scaled review policy. They are deterministic rules
used by `merge-ready`, `task-detail`, `review-waits`, and coordinator plan
output. They do not approve reviews, waive safety gates, or authorize live
execution by themselves.

Profiles are evaluated in file order. The first matching profile can add
required review domains, waive or defer domains for the current slice, inherit
domains from an approved parent/group packet, and explain why extra reviewers
improve risk control.

Fairway also provides built-in default profiles named `prototype-first`,
`reversible`, `irreversible`, `live-boundary`, and `release-boundary`. These
defaults cover the common small-team policy: uncertain reversible product/UX
work can run as prototype-first, reversible non-live work is advisory and
evidence-led, while irreversible, live, and release boundaries remain blocking.
A configured `[[review_profiles]]` entry with the same `name` replaces the
built-in default for that name.

For `prototype-first` work, use evidence artifact types
`prototype-artifact`, `owner-usage-proof`, `prototype-gap-list`, and
`stabilization-decision` to show the build-use-learn loop before stabilizing
contracts or moving to a stricter profile.

Key fields:

| Key | Type | Description |
|---|---|---|
| `name` | string | Profile name, such as `micro-slice`, `grouped-slice`, `epic`, `launch`, `live-window`, `deploy`, or `production-readiness`. |
| `mode` | string | `advisory` or `blocking`. New process rules should usually start advisory with a stated hypothesis before becoming blocking defaults. |
| `match_kinds`, `match_risk_levels`, `match_tags`, `match_authoring_domains`, `match_owning_domains`, `match_paths` | []string | Match task metadata and source/target paths. Empty lists do not restrict matching. |
| `required_review_domains` | []string | Review domains added by the profile. Task-level `review_domains` still apply unless waived or deferred. |
| `inherit_from_parent`, `inherit_review_domains` | bool, []string | Allow child domains to be covered by matching approved parent reviews. |
| `waive_review_domains`, `defer_review_domains` | []string | Mark domains as waived for this slice or deferred to parent/epic/release review. |
| `safe_iteration_zone` | bool | Marks approved non-live/disposable boundaries where setup, readback, harness, classifier, or provider-shape fixes can iterate with lightweight review. |
| `safe_iteration_defect_class`, `safe_iteration_control` | string | Explain the expected defect class and risk-control value for this profile. |
| `extra_reviewer_rationale` | string | Explains why any extra reviewers reduce risk or cycle time. |
| `process_hypothesis` | string | States the speed, quality, or safety hypothesis for a new review/gate process pilot. |
| `outcome_metrics` | []string | Names outcomes to review in `fairway review-policy report`, such as `defects_caught`, `rework_reduced`, `blocked_time`, `cycle_time`, and `avoided_unsafe_actions`. |
| `no_inheritance_kinds`, `no_inheritance_risk_levels`, `no_inheritance_tags`, `no_inheritance_paths` | []string | Triggers that force direct review instead of inherited/grouped review. Use these for authority expansion, environment mutation, credentials, safety-gate weakening, live/deploy/public exposure, compliance, or enforcement boundaries. |
| `group_review` | bool | Lets coordinator plan recommend grouped review for related ready tasks that match this profile. |

### `[[provider_targets]]`

Provider targets map review or work domains to notification destinations. They
are routing metadata only. They do not store provider credentials and they do
not grant review approval, task completion, merge, push, deploy, or release
authority.

```toml
[[provider_targets]]
domain = "security"
provider = "codex"
type = "thread"       # generic | thread | tmux | cli | webhook
target = "019e..."

[[provider_targets]]
domain = "ops"
provider = "tmux"
type = "tmux"
target = "ops:0.1"
```

Notification adapters may use these targets to send a provider-specific prompt
or event, then record the result with `fairway record notification`. Fairway
keeps the durable state machine separate: a delivered notification is only proof
that the target was contacted, not proof that review happened.

`type = "thread"` is routing metadata unless a host-specific adapter or desktop
thread tool is actually available and invoked. Recording a Fairway notification
or handoff does not by itself send a prompt into an existing Codex Desktop
thread. Agents must distinguish "Fairway handoff/notification recorded" from
"thread steered" and should fall back to a manual relay block when the host
does not expose a thread messaging tool. See
[agent-guide.md](agent-guide.md#thread-steering-vs-fairway-notification).

| Key | Type | Default | Description |
|---|---|---|---|
| `domain` | string | — | Review domain or target role, such as `architecture`, `security`, `governance`, `backend`, `frontend`, or `ops`. |
| `provider` | string | — | Provider/adapter label, such as `codex`, `claude`, `tmux`, `shell`, or `webhook`. |
| `type` | string | `generic` | Destination type: `generic`, `thread`, `tmux`, `cli`, or `webhook`. |
| `target` | string | — | Provider-local target id. Do not put secrets or bearer URLs here. |

`fairway advisory validate` uses provider targets only to warn when a
`wake_provider` recommendation is not routable. The warning does not grant send
authority, dashboard mutation authority, review approval, merge, deploy, or
live-operation permission.

Wake surfaces also use provider targets for static routability checks. Review
wait wakes, generic wait wakes, completion-handback wakes, live-operation
closeout wakes, and provider-session handoff wakes must have a configured target
for the next owner/domain before Fairway can claim delivery. If the mapping is
missing, dry-run output names `mapping_required`; `--send` records
`notification_failed` evidence instead of silently parking the wait or claiming
thread delivery.

### `[[advisory_provider_adapters]]`

Advisory provider adapters declare optional recommendation sources for
`fairway advisory validate` and future bounded coordinator surfaces. They are
configuration metadata only. They do not store prompts, transcripts, raw tool
bodies, provider-private data, auth tokens, cookies, or credentials; they also
do not grant approval, review, wake, merge, deploy, release, or live-operation
authority.

```toml
[[advisory_provider_adapters]]
name = "local-rules"
provider = "ollama"
type = "local_ollama"
mode = "advisory"
trust = "low"
model = "llama3.1"
endpoint_env = "FAIRWAY_OLLAMA_ENDPOINT"
capabilities = ["summarize_evidence", "rank_ready_tasks"]
allowed_actions = ["inspect_task", "render_packet", "wake_provider"]
```

Use `fairway advisory adapters` to inspect configured adapters. Disabled
adapters are hidden by default; use `--include-disabled` for an operator audit.
Use `fairway advisory validate --provider <name>` to validate a recommendation
against the adapter's mode and `allowed_actions`. The validated recommendation
may be recorded as `advisory-recommendation` evidence only; Fairway does not
apply the recommendation automatically.

| Key | Type | Default | Description |
|---|---|---|---|
| `name` | string | — | Stable adapter name used by `--provider`. |
| `provider` | string | — | Provider label, such as `ollama`, `codex`, `claude`, or `gemini`. |
| `type` | string | `noop` | Adapter type: `noop`, `rules-only`, `local_ollama`, `local_llamacpp`, `openai-compatible`, `codex`, `claude`, or `gemini`. |
| `mode` | string | `advisory` | `advisory`, `report_only`, or `disabled`. Disabled adapters cannot validate recommendations. |
| `trust` | string | `low` | Trust label: `low`, `medium`, or `high`. Low-trust output with risk flags must stay human-reviewed. |
| `model` | string | — | Optional model label for operator visibility. |
| `endpoint_env` | string | — | Optional environment variable name for an endpoint URL. This is an env var name, not the endpoint value and not a credential. |
| `capabilities` | []string | — | Tokenized advisory capabilities for reporting and review. |
| `allowed_actions` | []string | all bounded advisory actions | Optional subset of the advisory action enum accepted from this adapter. |

### `[[external_notifiers]]`

External notifiers declare optional notification sinks for operator-controlled
notification workflows. `noop` notifiers remain dry-run only. `log` and
`webhook` notifiers can be enabled for real delivery only with `mode = "send"`.
Destinations and credentials are read from environment variables at send time;
Fairway records only the env var name or safe target label, never webhook URLs,
tokens, arbitrary prompts, transcripts, or raw tool bodies. The dashboard never
calls notifier send paths and does not gain send authority.

```toml
[[external_notifiers]]
name = "control-log"
type = "log"
mode = "dry_run"
target_env = "FAIRWAY_NOTIFY_LOG"
domains = ["coordinator", "ops"]
template_name = "control_room_handoff"

[[external_notifiers]]
name = "control-webhook"
type = "webhook"
mode = "send"
target_env = "FAIRWAY_NOTIFY_WEBHOOK_URL"
token_env = "FAIRWAY_NOTIFY_WEBHOOK_TOKEN"
domains = ["coordinator", "ops"]
template_name = "control_room_handoff"
rate_limit_per_minute = 30
```

Use `fairway notify notifiers` to inspect configured notifiers. Use
`fairway notify dry-run --notifier <name> --task <task-id> --domain <domain>` to
render a bounded notification request from the configured fixed template, or
pass `--template <name>` to choose another fixed template label. With
`--record-intent`, Fairway records a `record notification` row with state
`intent` only. That is a durable coordination fact, not proof that an external
system or provider thread was contacted. Fairway stores the template label in
the notification reason, not arbitrary prompt text for later replay.

Use `fairway notify send --notifier <name> --task <task-id> --domain <domain>`
only for explicitly configured `mode = "send"` notifiers. Send records a
`sent` notification row before delivery and then records either
`notification_delivered` or `notification_failed`. Webhook send uses HTTP POST
with a fixed JSON body rendered from current task/domain/template metadata.
If `token_env` is configured and set, the value is sent as a bearer token but is
not printed or recorded. The optional `--target` value is only a safe display
label and is restricted to letters, digits, dots, dashes, and underscores.
Rate limiting is per notifier send attempt and degrades to
`notification_failed` evidence instead of silent loss.

| Key | Type | Default | Description |
|---|---|---|---|
| `name` | string | — | Stable notifier name used by `fairway notify dry-run --notifier` and `fairway notify send --notifier`. |
| `type` | string | `noop` | Notifier type: `noop`, `log`, or `webhook`. `noop` cannot use `mode = "send"`. |
| `mode` | string | `dry_run` | `dry_run`, `send`, or `disabled`. Disabled notifiers fail closed. |
| `target_env` | string | — | Environment variable name for the destination, such as a log path or webhook URL. Required for `mode = "send"`. This is an env var name, not a secret or URL value. |
| `token_env` | string | — | Optional environment variable name for a webhook bearer token. The value is used only at send time and is not recorded. |
| `domains` | []string | all domains | Optional allowed notification domains/roles. |
| `template_name` | string | — | Optional fixed-template label for operator review. |
| `rate_limit_per_minute` | int | `30` for send mode | Maximum sends per notifier per minute. `0` uses the send-mode default. |

### `[[provider_model_prices]]`

Provider model prices are advisory calculator inputs for
`fairway usage cost-report`. They convert already-recorded provider usage
counts into planning estimates. They do not configure provider credentials, do
not poll provider APIs, and do not create budget, approval, merge, completion,
or release gates.

Prices are expressed in dollars per million tokens. Use `model = "*"` for a
provider default, `provider = "*"` for a model default, or both as a global
fallback.

```toml
[[provider_model_prices]]
provider = "codex"
model = "gpt-5-codex"
input_per_million = 1.25
cached_input_per_million = 0.125
output_per_million = 10.0
reasoning_per_million = 10.0

[[provider_model_prices]]
provider = "codex"
model = "snapshot-only-model"
total_per_million = 2.0
```

| Key | Type | Default | Description |
|---|---|---|---|
| `provider` | string | — | Provider label matching `record usage --provider`, or `*` for a fallback. |
| `model` | string | — | Model label matching `record usage --model`, or `*` for a fallback. |
| `input_per_million` | float | — | Uncached input-token price. When cached tokens are known, Fairway charges `input - cached` here. |
| `cached_input_per_million` | float | — | Cached input-token price. |
| `output_per_million` | float | — | Output-token price. |
| `reasoning_per_million` | float | — | Reasoning-token price when providers report it separately. |
| `total_per_million` | float | — | Fallback total-token price for records that only have `total_tokens` or derived snapshots. |

Each row must include `provider`, `model`, and at least one non-negative price
field. Missing token counts or missing matching prices stay `unknown` in cost
reports rather than being treated as zero.

### `[coordinator]`

| Key | Type | Default | Description |
|---|---|---|---|
| `max_primary_tracks` | int | `1` | Advisory limit for active primary work in `fairway coordinator preflight`. |
| `max_sidecar_tracks` | int | `1` | Advisory limit for active side tracks/checkpoints. |
| `max_review_tracks` | int | `1` | Advisory limit for active review/verification tracks. |
| `checkpoint_stale_after` | duration | `24h` | Checkpoints older than this are stale unless the checkpoint state is `awaiting_input`, `done`, `parked`, or `abandoned`. |
| `notification_ack_timeout` | duration | `24h` | How long a `sent` provider/thread notification may wait without acknowledgement, `review_recorded`, or a real matching review before coordinator plan escalates it as `stale-sent`. |

### `[[roles]]`

| Key | Type | Default | Description |
|---|---|---|---|
| `name` | string | — | Role identifier. Must be unique. Referenced by task `role` and handoffs. |
| `branch` | string | `agent/<name>` | Long-lived branch for this role. |
| `provider` | string | — | Informational tag (e.g. `claude`, `codex`, `gemini`). Not enforced. |

### `[[review_routes]]`

Ordered list. The first matching glob wins.

| Key | Type | Default | Description |
|---|---|---|---|
| `match` | string | — | Glob matched against paths touched in the task's commits. |
| `reviewer` | string | — | Role name to route the review to. Must match a configured role. |

### `[[workstream_profiles]]`

Named coordination profiles for architecture-aware work. These are advisory
configuration today: validation accepts them, `fairway adoption artifact` uses
`route_samples`, reports named profile gates, and evaluates those gates against
matching task evidence rows. Future dashboard/packet work can consume the same
metadata without changing the file shape.

| Key | Type | Default | Description |
|---|---|---|---|
| `name` | string | — | Stable profile name, for example `platform-foundation`, `release-readiness`, or `sdk-readiness`. Must be unique. |
| `task_kinds` | []string | — | Task kinds associated with this profile. If `[task_kinds].allowed` is configured, every profile kind must appear there. |
| `dashboard_groups` | []string | — | Human-facing groups a dashboard can use to cluster tasks for this profile. |
| `rule_groups` | []string | — | Rule groups bound to this profile, using `<rule-source-name>.<rules-subdirectory>` names such as `fairway-platform.core`. |
| `tag_groups` | []table | — | Optional recommended tag display groups for dashboards/reports. These are advisory; task tags remain generic task metadata. |
| `review_domains` | []string | — | Review domains that may be required for readiness, distinct from first-match assignment routes. |
| `route_samples` | []string | — | Paths sampled by `fairway adoption artifact` when no `--route` flags are provided. |

#### `[[workstream_profiles.tag_groups]]`

Recommended tag groups under the preceding profile. They document common
cross-cutting tags without making those tags core Fairway grammar.

| Key | Type | Default | Description |
|---|---|---|---|
| `name` | string | — | Human-facing group name, for example `release environments` or `security programs`. Must be unique within the profile. |
| `tags` | []string | — | Recommended tags in display order. Supports simple and key:value tags. |
| `description` | string | — | Optional explanation for operators and profile authors. |

#### `[[workstream_profiles.gates]]`

Named readiness gates under the preceding profile.

| Key | Type | Default | Description |
|---|---|---|---|
| `name` | string | — | Gate name, for example `security-review`, `uat-evidence`, `release-risk`, or `sdk-readiness`. Must be unique within the profile. |
| `group` | string | derived from task kind or evidence type | Optional dashboard/report grouping label, for example `boundary guards`, `release evidence`, `security gates`, or `SDK readiness`. Use it when a profile has many gates and the default gate-by-gate view would be noisy. |
| `mode` | string | — | `advisory`, `blocking`, or `report_only`. Missing `blocking` gates fail `merge-ready`; missing `advisory` and `report_only` gates are warnings. |
| `task_kinds` | []string | — | Optional task-kind filter for this gate. Omit to apply the gate to every task kind in the profile. |
| `evidence_type` | string | — | Optional evidence type this gate expects. |
| `required_evidence_count` | int | `0` | Minimum count expected for this evidence type in adoption gate evaluation. If omitted but other evidence requirements are present, evaluation treats the gate as needing at least one matching row. |
| `accepted_results` | []string | — | Accepted `task_evidence.result` values: `pass`, `fail`, `partial`, `skipped`, or `blocked`. Rows with other results do not count. |
| `artifact_required` | bool | `false` | Whether each counted evidence row must include an artifact path or URL. |
| `owner_signoff_required` | bool | `false` | Whether each counted evidence row's notes must contain `signoff` or `sign-off`. |
| `expires_after` | duration | — | Duration after which an evidence row is considered stale and no longer counted, for example `720h`. |
| `description` | string | — | Optional human-readable description. |

### `[[packet_templates]]`

Declarative packet template metadata. `fairway packet template <name> <task-id>`
validates required fields and renders a packet with task detail, evidence, and
review context. The current built-in packet commands still render their
specific packet shapes, but templates let projects add profile-specific packets
without Fairway code changes.

For deploy handoffs, use a reusable template such as
`environment-deploy-preflight`; see
[environment-deploy-preflight.md](design/environment-deploy-preflight.md) for
the recommended fields, evidence types, and readiness gates.

| Key | Type | Default | Description |
|---|---|---|---|
| `profiles` | []string | — | Optional list of workstream profile names this packet template belongs to. If profiles are configured, references must match a configured profile. |
| `name` | string | — | Packet template name. Must be unique. |
| `required_fields` | []string | — | Required field names for template validation/rendering. |
| `optional_fields` | []string | — | Optional field names. A field cannot appear in both required and optional lists. |

Render a configured template with repeated `--field key=value` arguments:

```bash
fairway packet template architecture-map T-010 \
  --field scope="route ownership" \
  --field current_owner=mixed \
  --field target_owner=D-arch \
  --field migration_risk="route moves can hide auth regressions" \
  --field acceptance="owners and review routes are explicit"
```

### `[[rule_sources]]`

Rule sources configure reusable operating rule packs. They are documented in
[Rule packs](design/rule-packs.md). The first implementation is local-first:
enabled sources must use `path:` or `file:`.

The examples below show possible shapes, not automatic adoption. A source is
live only when the active project config points at a local path that exists and
the project has completed the adoption checklist in
[Rule packs](design/rule-packs.md#project-adoption-checklist). Keep optional
or future remote sources `disabled` until fetch/cache policy exists.

```toml
[[rule_sources]]
name = "fairway-platform"
source = "path:../fairway-rules-platform"
mode = "advisory"

[[rule_sources]]
name = "gpuaas"
source = "path:../fairway-rules-gpuaas"
mode = "disabled" # enable only after local path and vocabulary validation

[[rule_sources]]
name = "codeguard"
source = "github:fairway-run/fairway-rules-codeguard"
mode = "disabled"
commit_sha = "0123456789abcdef0123456789abcdef01234567"
checksum = "sha256:..."
```

| Key | Type | Default | Description |
|---|---|---|---|
| `name` | string | — | Unique source name. Group names are derived from this value plus the pack `rules/` subdirectory. |
| `source` | string | — | Source reference. Initial enabled support is local `path:` or `file:` only. Remote schemes are represented but not fetched. |
| `mode` | string | `advisory` | `advisory`, `blocking`, or `disabled`. |
| `commit_sha` | string | — | Future immutable remote pin metadata. Required for represented `github:` sources. |
| `checksum` | string | — | Future remote content checksum metadata. Required for represented `github:` sources. |

Modes:

- `advisory`: recommend rules and evidence without blocking closeout.
- `blocking`: missing required evidence blocks configured readiness checks.
- `disabled`: keep the source configured but do not evaluate it.

Missing or unreadable local sources are mode-sensitive. Advisory sources become
error-severity load findings in rule CLI JSON/human output while other valid
sources still load. Blocking sources fail closed and stop the command.

Remote `github:` sources must remain `disabled` until safe fetch/cache support
lands. They must include both `commit_sha` and `checksum` so mutable branch/tag
references do not become blocking authority by accident.

### `[states]`

| Key | Type | Default | Description |
|---|---|---|---|
| `allowed` | []string | `["todo","in_progress","blocked","done"]` | All states tasks may occupy. |
| `terminal` | []string | `["done"]` | Subset of `allowed` considered terminal. Tasks in terminal states do not move without `--reopen`. |
| `transitions` | [][2]string | — | Optional whitelist of allowed transitions. `["*", "x"]` means any state may transition to `x`. Omit for permissive mode. |

### `[gates]`

| Key | Type | Default | Description |
|---|---|---|---|
| `require_evidence_before_done` | bool | `false` | If true, a task cannot transition to a terminal state without at least one `task_evidence` row. |
| `require_review_before_done` | bool | `false` | If true, a task cannot transition to a terminal state without at least one `task_reviews` row with `verdict = "approve"`. |
| `require_handoff_before_merge_ready` | bool | `false` | If true, `fairway merge-ready` requires at least one handoff row. Useful for coordinated PR handoff workflows. |
| `require_blocked_reason` | bool | `true` | If true, transitions into `blocked` require `--reason` so timing and health reports can explain the blocker. |
| `allow_force_without_reason` | bool | `false` | If false, forced transitions still require a reason so overrides remain auditable. |

### `[task_kinds]`

| Key | Type | Default | Description |
|---|---|---|---|
| `allowed` | []string | — | Optional whitelist for `task_definitions.kind`. Omit for free-text. |
| `default` | string | `task` | Kind assigned when `fairway add` or `fairway spawn` omits `--kind`. |

See [docs/design/hierarchy.md](design/hierarchy.md) for the hierarchy model and the `fairway spawn` command.

### `[task_priorities]`

| Key | Type | Default | Description |
|---|---|---|---|
| `default` | int | — | Priority assigned when `fairway add` / `fairway spawn` omits `--priority`. |
| `levels` | `[]{rank,label,description?}` | — | Optional label table. The stored value is always the integer `rank`; labels are display-time only. Omit `[task_priorities]` entirely to leave priority as a free integer with no labels. |

Lower `rank` is more urgent. Priority is cross-cutting — it overrides epic boundaries in `fairway ready` and dashboard backlog sort.

## Task metadata

Tasks may carry profile-aware metadata in YAML/JSON imports and through
`fairway add`, `fairway spawn`, and `fairway update` flags:

| Field / flag | Type | Description |
|---|---|---|
| `profile` / `--profile` | string | Workstream profile name. Validated when `[[workstream_profiles]]` exists. |
| `owning_domain` / `--owning-domain` | string | Architecture or product domain that owns the task. |
| `owning_layer` / `--owning-layer` | string | Layer such as `api`, `service`, `frontend`, `guard`, or `release`. |
| `source_paths` / `--source-paths` | []string / CSV | Current paths, inputs, or surfaces affected by the task. |
| `target_paths` / `--target-paths` | []string / CSV | Target paths, outputs, or artifacts expected from the task. |
| `review_domains` / `--review-domains` | []string / CSV | Review domains expected for the task. |
| `tags` / `--tag` | []string / CSV | Generic cross-cutting tags. Supports simple tags and key:value tags such as `environment:staging`. |
| `risk_level` / `--risk-level` | string | Lightweight risk label. Fairway does not hardcode allowed values. |
| `migration_type` / `--migration-type` | string | Shape of the work, such as `facade`, `boundary-guard`, or `ownership-map`. |

This metadata is intentionally generic. It powers architecture-aware
coordination without making Fairway specific to GPUaaS, ARC, or any one repo.
It also drives `fairway audit work-coverage`: changed files are matched against
task `source_paths` and `target_paths`, and done tasks with `review_domains`
are checked for matching approved review rows.

## Validation

`fairway init` writes a default config. `fairway config validate` checks an existing one. Errors are reported with file path and line number.

## `fairway init` defaults

`fairway init` writes the concrete defaults shown above, not a commented sample.
The initial config has permissive completion gates but requires a reason when a
task enters `blocked`:

- `task_id_pattern = "^[A-Z]+-[0-9]+$"`
- `require_evidence_before_done = false`
- `require_review_before_done = false`
- `require_handoff_before_merge_ready = false`
- `require_blocked_reason = true`
- `allow_force_without_reason = false`

This keeps first-run friction low while ensuring blocked work is explainable.
