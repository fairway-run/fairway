# Provider Replacement Quickstart

This optional path shows the user value behind Fairway's provider-neutral
record: a new provider session can resume a bounded task from durable facts
without reading the previous provider's private transcript.

Use this after the main [Quickstart](quickstart.md). It is not a release,
review, push, deploy, or approval path.

## What This Proves

- Same-vendor fresh-session replacement: one provider family stops, and a new
  session from the same provider resumes from Fairway records, Git state, and
  repository docs.
- Cross-vendor replacement can additionally test whether a different provider
  family resumes from the same durable facts.

These are related but different claims. Same-vendor replacement proves that
chat memory is not required inside one provider ecosystem. Cross-vendor
replacement adds a provider-surface compatibility check: the second provider
must be installed, authenticated, capable of accessing the repository, and able
to run the required commands.

## 1. Start Bounded Work

Create or choose a small reversible task with clear acceptance. Start it with a
session and record the source attachment:

```bash
fairway work start FV-002 \
  --session-id source-provider-demo \
  --role operator \
  --provider codex \
  --backend shell \
  --summary "Start provider replacement demo from bounded docs work"
```

Record the current Git state before editing:

```bash
git status --short --branch
git rev-parse HEAD
fairway work verify FV-002 \
  --command-text "git status --short --branch && git rev-parse HEAD" \
  --result pass
```

## 2. Record The Material Choice

Capture why the task is safe to hand to a replacement provider:

```bash
fairway decision record FV-002 \
  --decision "Use a replacement provider for this bounded task" \
  --trigger "The task should survive loss of provider chat context" \
  --alternative "Continue only in the original provider session" \
  --chosen "Record intent, baseline, and evidence before replacement" \
  --reason "A new session can inspect Fairway and Git facts directly" \
  --risk "Replacement may find missing context; record gaps instead of guessing" \
  --validation "fairway task-detail FV-002" \
  --fact-ref "task:FV-002"
```

## 3. Stop Before Completion

Leave the original provider before the implementation is done. Record a handoff
that names what the next provider may read and what it must not use:

```bash
fairway record handoff FV-002 \
  --to operator \
  --payload "Use Fairway and Git facts only; do not read private transcripts."
```

The replacement report should still include recovered facts, missing context,
elapsed time, rough edges, validation, and cleanup.

## 4. Resume Fresh

Start a new session. For a same-vendor demo, use a fresh session from the same
provider. For a cross-vendor demo, use a different provider only after checking
that it can access the repository and run local commands.

The replacement provider should begin with:

```bash
fairway task-detail FV-002
fairway decision list FV-002
git status --short --branch
git diff --stat
git log --oneline -5
```

Then it should complete only the bounded task. Missing context is a result to
record, not a reason to invent intent.

## 5. Report The Outcome

The useful report is short and factual:

- recovered facts: task owner, status, acceptance, decisions, evidence, handoff,
  Git baseline, and current diff;
- unrecoverable context: transcript-only reasoning, unstated preferences,
  unavailable provider behavior, or missing artifacts;
- timing: source stop time, replacement start time, and elapsed implementation
  time when measured;
- rough edges: command friction, missing fields, unavailable providers, or
  unclear ceremony;
- cleanup: changed files, validation commands, and confirmation that no commit,
  push, deploy, release, review approval, or external mutation occurred unless
  explicitly authorized.

The first value is not that Fairway certifies the work. The value is that the
next operator can recover what was intended, what was decided, what was checked,
what is missing, and what remains local and reversible.
