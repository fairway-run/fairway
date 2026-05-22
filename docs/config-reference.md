# Config reference

Fairway reads `.fairway/config.toml` from the repo root by default. Override with `--config <path>` or `FAIRWAY_CONFIG=<path>`.

## Full schema

```toml
[fairway]
project_name = "myrepo"                # default: basename of repo root; must be unique in registry
db_path = ".fairway/state.db"          # relative to repo root
queue_source = "inline"                # "inline" | "yaml:<path>" | "json:<path>"
main_branch = "main"                   # base branch worktrees branch off of

[dashboard]
listen = "127.0.0.1:7878"
auto_open = true                       # open browser when `fairway dashboard` starts

[worktrees]
root = "../worktrees"
naming = "{repo}-{role}"

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

## Validation

`fairway init` writes a default config. `fairway config validate` checks an existing one. Errors are reported with file path and line number.
