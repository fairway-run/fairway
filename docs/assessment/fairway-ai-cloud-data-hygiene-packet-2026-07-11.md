# AI Cloud Fairway Data Hygiene Packet

Date: 2026-07-11
Task: FW-322
Source database: `.fairway/platform-foundation.db`
Released reader: Fairway v0.1.12

## Decision

Do not delete, rewrite, compact, checkpoint, or archive source rows under
FW-322. The current data set contains durable engineering evidence, and the
v0.1.12 performance assessment identified product read-model and SSE defects
that remain reproducible without treating historical data as disposable.

Use backup, export, derived active views, and explicit retention policy as
separate controls:

1. backup/export protects canonical state and supports restore;
2. read-model filtering controls what routine dashboards hydrate;
3. lifecycle projection prevents old facts from appearing actionable;
4. WAL maintenance is an operator action performed only after backup and after
   active readers are understood;
5. deletion or physical archival requires a separately reviewed migration with
   restore and provenance proof.

## Read-Only Classification

| Class | Count or size | Interpretation | Action |
|---|---:|---|---|
| done tasks | 1,702 | Terminal provenance; every task has evidence | Retain; use bounded/default filters |
| blocked tasks | 37 | Non-terminal work requiring an explicit decision | Keep visible; do not archive as history |
| todo tasks | 8 | Current queue | Keep visible |
| stale checkpoint rows | 216 | Non-closed checkpoints with past `target_close_by`; older rows may be superseded by later lifecycle facts | Derive latest actionable state; do not rewrite history |
| unacknowledged handoffs older than one hour | 245 | Historical handoff facts can outlive terminal tasks or later closeout | Suppress terminal/superseded actionability in read models; retain facts |
| evidence rows | 4,976 | Engineering proof and references | Retain |
| history rows | 5,873 | State provenance | Retain |
| sessions | 1,194 | Provider/lane lifecycle evidence | Retain under explicit metadata policy |
| reviews | 1,379 | Governance evidence | Retain |
| notifications | 1,229 | Delivery/audit evidence | Retain |
| SQLite database | 13,410,304 bytes | Canonical local store | Back up and integrity-check |
| SQLite WAL | 14,980,352 bytes | Runtime journal, not an archive tier | Reassess after FW-316 and under FW-323 |

The database reports `journal_mode=wal`, `wal_autocheckpoint=1000`, and
`integrity_check=ok`. A large WAL while two dashboards and an SSE client are
active is a runtime observation. It is not evidence that old task rows should
be deleted.

## Backup And Restore Proof

Artifacts were written only under `/tmp/fairway-fw322-hygiene`:

- `state.db`: Fairway `db backup` output;
- `export.json`: Fairway `db export` output;
- `SHA256SUMS`: backup/export hashes;
- `source-classification.txt`: read-only source counts;
- `restore-readback.txt`: restored table counts and integrity result;
- `restore-ready.txt`: ready-queue readback from the backup;
- `restore-reconcile.txt`: active reconciliation from the backup.

Hashes:

```text
f8364823a7c75d05bce5a1b609cdd065cee8bbbee8ee8603ae10ba9540583a20  state.db
2de8a0cd83c54a06c4d0c9c8d196352a373da49cde614d0d15ec3e12ce73ef6b  export.json
```

Restore validation passed:

- SQLite integrity: `ok`;
- 1,747 tasks;
- 4,976 evidence rows;
- 5,873 state-history rows;
- 3,777 checkpoints;
- 1,194 sessions;
- 1,379 reviews;
- 1,229 notifications;
- Fairway config validation: pass;
- Fairway ready readback: pass;
- active reconciliation: no findings.

Source database and WAL sizes and modification timestamps were identical before
and after backup/export/readback. No source cleanup or WAL checkpoint ran.
The backup hash above was captured after restore validation; opening the backup
through Fairway can update SQLite file bookkeeping, so a pre-readback hash must
not be presented as the final archived artifact hash.

## Recommended Operating Packet

Before any future maintenance:

```bash
fairway workflow check --mode deploy --require-clean --require-pushed
fairway db backup <reviewed-backup-dir>/state.db
fairway db export <reviewed-backup-dir>/export.json
shasum -a 256 <reviewed-backup-dir>/state.db \
  <reviewed-backup-dir>/export.json
fairway --db <reviewed-backup-dir>/state.db config validate
fairway --db <reviewed-backup-dir>/state.db ready
fairway --db <reviewed-backup-dir>/state.db reconcile active --dry-run
```

Record binary version, config identity, project ID, source commit, database
schema version, artifact hashes, restore output, owner, and retention date.
Backup storage encryption and access belong to the deployment environment, not
to arbitrary Fairway evidence text.

## Retention And Read-Model Rules

- Task definitions, state history, reviews, decisions, evidence metadata,
  handoffs, notifications, and lifecycle records remain canonical append-only
  facts unless a reviewed schema migration defines otherwise.
- Routine wall/board/task views should hydrate active scope plus bounded recent
  history. Explicit reports/exports may request full history.
- Stale checkpoint counts should represent the latest actionable checkpoint per
  task, not count every superseded historical row as current debt.
- Handoff/wait projections should continue suppressing terminal or superseded
  actionability while retaining the underlying audit fact.
- Artifact files follow their own configured roots, redaction, and retention;
  deleting an external artifact must not fabricate deletion of its Fairway
  evidence reference.
- SQLite WAL files are managed as database runtime state. Do not copy only the
  main DB file while writers are active; use Fairway backup or a reviewed
  SQLite-safe snapshot procedure.

## Follow-Up Ownership

- FW-316 owns the confirmed idle SSE CPU defect.
- FW-317 through FW-321 own product read-model/routing latency and correctness.
- FW-323 remeasures dual-process SQLite, WAL, cache, CPU, and RSS behavior after
  FW-316 so runtime conclusions are not confounded by the known busy loop.

This packet does not close any product performance defect by proposing a smaller
fixture. It authorizes no deletion, migration, store switch, dashboard write,
public exposure, release, deploy, or live-operation action.
