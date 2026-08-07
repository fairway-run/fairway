# Quickstart

This path creates one complete Fairway control record in an existing Git
repository. It introduces only the vocabulary needed for the first result:
work, decision, evidence, and closeout.

The path was rehearsed from a clean temporary repository on 2026-07-11. The
commands completed in less than one second of machine execution after the
binary and repository baseline were available. The full evidence, failed
attempts, defaults, and cleanup proof are in the
[five-minute assessment](assessment/fairway-five-minute-first-value-2026-07-11.md).

## Prerequisites

- Git repository with a committed `main` branch.
- Fairway installed or built from the current source.

Install the current release on macOS:

```bash
brew tap fairway-run/tap
brew install --cask fairway
fairway version
```

Or run the current source:

```bash
go install github.com/fairway-run/fairway/cmd/fairway@latest
fairway version
```

## 1. Initialize Fairway

From the repository root:

```bash
fairway init
fairway agent-contract status
git add .fairway/.gitignore .fairway/AGENTS.md .fairway/config.toml
git commit -m "chore: initialize Fairway"
fairway doctor
```

`fairway init` creates local configuration, an ignored SQLite DB, and the
agent breadcrumb. Commit the generated control files before completing work;
`fairway work close` fails closed on an uncommitted worktree.

After upgrading Fairway, `fairway preflight` reports agent-contract drift.
Review it with `fairway agent-contract plan`, then apply a compatible update
with `fairway agent-contract apply`. Keep project-specific additions in
`.fairway/AGENTS.local.md`.

`fairway doctor` should report `doctor_ok: true`. Follow a failing diagnostic
before continuing. Warnings name their owner and suggested command.

## 2. Create And Start One Work Item

```bash
fairway add FV-001 \
  --title "Verify the local Fairway control record" \
  --role operator

fairway work start FV-001 \
  --session-id first-value \
  --role operator \
  --provider shell \
  --backend shell \
  --summary "Complete the first bounded Fairway record"
```

The task is the accountable intent. The session is the replaceable execution
attachment. Nothing has been approved, merged, deployed, or released.

## 3. Record A Material Decision

```bash
fairway decision record FV-001 \
  --decision "Keep first-value proof in Fairway" \
  --trigger "The setup check needs a durable rationale" \
  --alternative "Leave the result only in shell history" \
  --chosen "Record the decision and verification in the local Fairway DB" \
  --reason "The next operator can inspect the claim without provider chat" \
  --risk "Local metadata only; no external action" \
  --validation "fairway task-detail FV-001" \
  --fact-ref "task:FV-001"
```

The decision explains a choice. It is not approval, provenance for an external
fact, or authority to perform a consequential action.

## 4. Verify And Close

For this setup task, the passing doctor result is the evidence:

```bash
fairway work verify FV-001 \
  --command-text "fairway doctor" \
  --result pass

fairway work close FV-001 --session-id first-value
```

`work close` composes the existing workflow, evidence, review, and
merge-readiness checks. It does not create missing review or grant promotion
authority.

## 5. Inspect The Record

```bash
fairway task-detail FV-001
```

The readback should show:

- status `done`;
- the `fairway doctor` evidence row;
- the recorded decision and its authority boundary;
- the `todo -> in_progress -> done` history.

That is the first value: another operator or provider can recover what was
intended, decided, checked, and closed without reading this shell session.

## Optional: Open The Dashboard

```bash
fairway dashboard
```

The dashboard opens on loopback by default. Shared/public access, detached
lifecycle, multi-project mode, and identity-aware proxy setup are advanced
operator topics, not first-run requirements.

The single-project dashboard opens on Overview. It explains the accountability
chain using current project coverage and one cited Quality Record. Continue to
Wall for live provider activity, Board for bounded work, Quality for lifecycle
coverage, Reports for delivery history, or Controls for measured control
effectiveness.

## What To Learn Next

Choose the next page by need:

- Run normal work: [Agent guide](agent-guide.md)
- Practice replacing a provider:
  [Provider replacement quickstart](provider-replacement-quickstart.md)
- Understand the minimum model: [Concepts](design/concepts.md)
- Configure roles, routes, and profiles: [Configuration reference](config-reference.md)
- Operate the dashboard: [Dashboard](design/dashboard.md)
- Understand authority limits: [Product boundaries](design/product-boundaries.md)
- Set up multiple roles or worktrees: [Worktrees](design/worktrees.md)
- Run a small-team host: [Small-team lab deployment](operations/small-team-lab-deployment.md)

Do not add worktrees, shared servers, provider adapters, review matrices, or
release gates to the first path unless the work actually crosses that boundary.
