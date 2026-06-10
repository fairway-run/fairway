# Governed Agentic Engineering

Fairway supports a specific way of building software: multiple coding agents do
substantial implementation work, while the engineering organization keeps
ownership, verification, review, release posture, and human accountability
visible.

This is governed agentic engineering. It treats agents as high-throughput
engineering attachments, not as authorities. The durable engineering loop still
requires clear task ownership, reproducible evidence, independent review,
explicit promotion, and a responsible human who can explain why the system is
built the way it is.

## Definition

Governed agentic engineering is software delivery where:

- agents can implement, test, document, and investigate in parallel;
- task state, evidence, reviews, and handoffs live outside provider chat;
- verification is structural rather than optional;
- high-risk changes receive independent domain review;
- promotion to shared branches, CI, deploy, or release is explicit;
- humans remain accountable for architecture, risk, and final judgment.

The core shift is not "AI writes code." The core shift is that software teams
can safely delegate more implementation work when the surrounding engineering
system records what happened, why it happened, who reviewed it, and what proof
exists.

## Principles

| Principle | Failure it prevents | Fairway mechanism |
|---|---|---|
| Evidence before done | Tasks closed on agent confidence | Evidence rows name the exact command, result, and artifact |
| Durable task state | Work hidden in provider chat | SQLite execution store, task history, checkpoints, sessions |
| No self-review | The author approving their own work | Reviewer identity separated from review domain; merge readiness checks domains |
| Independent review domains | One lane vouching for areas it does not own | Review routes, required domains, risk-based review rules |
| Explicit handoff delivery | Work waiting silently on a lane that was never told | Handoff records, notification states, coordinator findings |
| Provider edges fail closed | A confused session corrupting durable state | Adapter trust boundary and session/task checks |
| Push is promotion | Remote branch sprawl and unreviewed CI triggers | Push-intent evidence and lane closeout checks |
| Debt is named | Historical gaps silently blessed later | Review-debt inventory, artifact-backed review, explicit waivers |
| Repeated checks become tools | Agents spend cycles polling and retyping commands | CI/deploy/UAT utilities with deterministic evidence |
| Rules are reusable | One project's habits stay tribal | Rule packs and workstream profiles |

These rules are not meant to add ceremony for its own sake. They encode the
checks teams already need when several implementation lanes move faster than a
single person can manually supervise in real time.

## The Human Comprehension Anchor

The main risk in agent-heavy work is throughput exceeding comprehension. A
project can accumulate many correct-looking commits, test outputs, and review
comments while the responsible humans gradually lose the ability to explain the
system.

The standard is not that a human reads every generated line. At this scale,
that is not realistic, and it was never consistently true in large human-only
teams either. The standard is:

> For any subsystem, a responsible human can reconstruct why it is built that
> way and would catch a confidently wrong review verdict.

Fairway is designed around that anchor. Evidence should let a reviewer rerun or
inspect the exact proof. Review domains should route high-risk changes to the
right kind of judgment. Design documents should preserve intent, not merely
describe code after the fact.

## Operating Practices

### Read The Diffs That Matter

Humans should spend attention where judgment matters most:

- schemas and migrations;
- state machines;
- authentication, authorization, and tenant boundaries;
- audit, logging, and privacy boundaries;
- release, deploy, and rollback paths;
- provider/session trust boundaries;
- changes that move ownership between subsystems.

Fairway review domains exist so these changes reach the right reviewers.

### Evidence Names The Command

"Tests pass" is not evidence. A useful evidence row records:

- exact command;
- result;
- artifact path;
- environment or source SHA when relevant;
- residual risk or follow-up task when partial.

This lets another person, agent, or utility reconstruct the verification path.

### Reviews Are Verdicts, Not Reactions

Review is not a chat acknowledgment. It is a recorded verdict against a task,
domain, evidence set, and changed surface.

Good review records answer:

- what was reviewed;
- which domain was represented;
- which evidence was trusted;
- what was approved or rejected;
- what remains out of scope.

### Promotion Is Explicit

Local lane work is cheap. Shared branch promotion is not. A branch pushed to a
remote, a CI run on `main`, a deploy, and a release all change shared state.

Governed agentic engineering treats those as promotion events:

- local lane commit;
- Fairway evidence;
- required review;
- merge-ready;
- merge to main;
- CI/deploy monitor;
- closeout.

### Disagreement Is Healthy

A review lane that always approves has stopped reviewing. Track changes
requested, waivers, blocked tasks, and cold re-review findings as process
health signals. The goal is not zero friction. The goal is useful friction at
the boundaries where mistakes are expensive.

## What This Model Is Not

Governed agentic engineering is not:

- autonomous software delivery;
- agents approving their own work;
- a substitute for architecture ownership;
- a replacement for CI, deploy, or incident response;
- a way to avoid reading critical diffs;
- a claim that process alone proves correctness.

Fairway coordinates the work. It does not grant authority to the provider that
generated the work.

## Why Fairway Exists

Agentic engineering needs a local execution record that issue trackers, CI
logs, and provider chats do not provide by themselves:

- who is working on which task;
- which session or utility is attached;
- whether the task is truly active, stale, blocked, or review-gated;
- what evidence exists;
- which review domains approved or requested changes;
- whether the lane is merge-ready;
- what was promoted to a shared branch or release path.

Fairway keeps that record close to the repo, visible in the CLI and dashboard,
and portable across providers.

## One-Line Version

Governed agentic engineering is high-delegation software delivery where agents
do much of the work, but evidence, review, ownership, promotion, and human
comprehension remain first-class engineering controls.
