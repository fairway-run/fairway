# Config reference

Fairway reads `.fairway/config.toml` from the repo root by default. Override with `--config <path>` or `FAIRWAY_CONFIG=<path>`.

## Full schema

```toml
[fairway]
project_name = "myrepo"                # default: basename of repo root; must be unique in registry
db_path = ".fairway/state.db"          # relative to repo root
queue_source = "inline"                # "inline" | "yaml:<path>" | "json:<path>"
main_branch = "main"                   # base branch worktrees branch off of
task_id_pattern = "^[A-Z]+-[0-9]+$"    # regex enforced by add/import/update

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

[[workstream_profiles]]
name = "platform-foundation"
task_kinds = ["architecture-map", "boundary-guard", "facade"]
dashboard_groups = ["architecture maps", "boundary guards", "facades"]
review_domains = ["architecture", "security"]
route_samples = ["doc/api/openapi.yaml", "cmd/api/routes.go"]

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
| `project_name` | string | basename of repo root | Label used by the multi-project dashboard. Must be unique across `~/.fairway/registry.toml`. |
| `db_path` | string | `.fairway/state.db` | SQLite DB path. Relative to repo root unless absolute. |
| `queue_source` | string | `inline` | `inline` (DB is authoritative), `yaml:<path>` or `json:<path>` (DB is still authoritative; the file is for bootstrap import only). |
| `main_branch` | string | `main` | Base branch new worktree branches are created from. |
| `task_id_pattern` | string | `^[A-Z]+-[0-9]+$` | Regex enforced for task IDs. GPUaaS parity configs use a wider pattern for legacy IDs such as `A-DEMO-UAT-001` and `A-PROV-REMOVE-SSH`. |

### `[dashboard]`

| Key | Type | Default | Description |
|---|---|---|---|
| `listen` | string | `127.0.0.1:7878` | HTTP listen address. Bind to `127.0.0.1` unless you understand the auth implications. |
| `auto_open` | bool | `true` | Open the system browser when `fairway dashboard` starts. |

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

### `[coordinator]`

| Key | Type | Default | Description |
|---|---|---|---|
| `max_primary_tracks` | int | `1` | Advisory limit for active primary work in `fairway coordinator preflight`. |
| `max_sidecar_tracks` | int | `1` | Advisory limit for active side tracks/checkpoints. |
| `max_review_tracks` | int | `1` | Advisory limit for active review/verification tracks. |
| `checkpoint_stale_after` | duration | `24h` | Checkpoints older than this are stale unless the checkpoint state is `awaiting_input`, `done`, `parked`, or `abandoned`. |

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
| `review_domains` | []string | — | Review domains that may be required for readiness, distinct from first-match assignment routes. |
| `route_samples` | []string | — | Paths sampled by `fairway adoption artifact` when no `--route` flags are provided. |

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
| `levels` | []{rank,label,description?} | — | Optional label table. The stored value is always the integer `rank`; labels are display-time only. Omit `[task_priorities]` entirely to leave priority as a free integer with no labels. |

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
| `risk_level` / `--risk-level` | string | Lightweight risk label. Fairway does not hardcode allowed values. |
| `migration_type` / `--migration-type` | string | Shape of the work, such as `facade`, `boundary-guard`, or `ownership-map`. |

This metadata is intentionally generic. It powers architecture-aware
coordination without making Fairway specific to GPUaaS, ARC, or any one repo.

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
