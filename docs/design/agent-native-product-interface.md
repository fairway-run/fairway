# Agent-native product interface

## Product statement

Fairway is an agent-native engineering control plane that gives software agents
durable context and gives humans accountable decisions, evidence, and promotion
control.

Agents are the primary operational users. Humans remain the authority for
consequential judgment. Agent-first does not mean agent-authoritative.

## Primary design test

Every Fairway capability should satisfy both questions:

1. Can a replacement agent safely continue this work without rereading the
   provider conversation?
2. Can a responsible human understand and defend the result without trusting
   the agent's narrative?

A feature that serves only one side is incomplete. Raw agent convenience without
accountability is unsafe; human-facing ceremony that agents cannot use
reliably becomes process drag.

## Primary workflow

```text
agent reads bounded context
        |
        v
agent performs scoped work
        |
        v
Fairway records observed facts, material decisions, and proof
        |
        v
independent controls evaluate review and promotion boundaries
        |
        v
human approves, challenges, or accepts consequential outcomes
```

The CLI, JSON contracts, context packets, recipes, rules, sessions, checkpoints,
and evidence models are primary agent interfaces. The dashboard is a human and
coordinator control-room projection over the same durable facts, not a separate
workflow authority.

## Authority boundaries

| Actor or surface | May do | Must not do implicitly |
|---|---|---|
| Agent | Read context, propose and implement scoped changes, draft decisions, run bounded validation, record facts. | Approve its own consequential work, infer credentials, grant live authority, or rewrite history. |
| Fairway | Store task state, observed facts, curated decisions, evidence, reviews, waits, and promotion gates; produce deterministic packets. | Invent historical reasoning, execute arbitrary provider work, or become autonomous approval authority. |
| Reviewer | Compare intent, diff, decisions, evidence, and policy; accept or reject within a named domain. | Treat agent narrative as proof or approve outside assigned authority. |
| Human operator | Authorize consequential actions, credentials, production mutation, release, and risk acceptance. | Delegate accountability to generated prose. |
| LLM explanation provider | Turn a grounded packet into readable narrative and bounded inference. | Become provenance, silently fill unknown history, or mutate Fairway state. |

## Grounded code explanation

The target interaction is:

```bash
fairway explain code packages/platform/iam/session_store.go \
  --line 142 \
  --format packet
```

Fairway resolves the code location to a grounded packet containing:

- source repository, path, symbol, line, commit, and diff facts;
- owning tasks, declared scope, contracts, and acceptance criteria;
- current and superseded material decisions;
- evidence, CI, UAT, and review verdicts;
- canonical architecture, policy, and operational references;
- conflicts, missing provenance, and confidence limits.

The deterministic packet is useful without an LLM and is the authority supplied
to any explanation provider.

## LLM narrative boundary

An optional configured advisory provider may turn the grounded packet into a
human-readable explanation. Fairway requires the output to distinguish:

- `recorded`: directly supported by cited Fairway, Git, contract, evidence, or
  review facts;
- `inferred`: a bounded interpretation derived from code or related facts;
- `unknown`: absent, contradictory, or insufficient provenance.

Example:

```text
Recorded:
Session authorization moved into the shared lookup in commit abc123 under
IAM-142. Decision D-142 says handler-only enforcement left two bypass paths.

Inferred:
The centralized placement also reduces the chance that future handlers omit the
same check.

Unknown:
No accepted record explains why the cache key format changed in the same commit.
```

Generated narrative is never written back as historical truth. It may propose a
missing decision or documentation update, but normal review is required before
that proposal becomes accepted provenance.

## Reproducibility posture

Fairway does not claim deterministic LLM execution. It creates a replayable,
auditable engineering packet that allows independently produced implementations
to be evaluated against the same:

- objective and boundaries;
- contracts and source facts;
- material decisions and accepted deviations;
- source, tool, model, dependency, and environment identities when available;
- validation commands, evidence hashes, and review requirements;
- closeout and promotion decisions.

The target is outcome equivalence and accountable re-execution, not identical
token streams or byte-identical generated code.

## Progressive disclosure

Agent-first usage must remain fast:

- routine reversible work uses compact `work` commands and advisory decision
  guidance;
- advanced inspection exposes the underlying task, session, checkpoint,
  decision, evidence, and review facts;
- consequential boundaries retain explicit review and human authorization.

Explainability cannot become a mandatory LLM call. Grounded packet generation
is local and deterministic; narrative generation is optional. Missing provider
access must not block normal task continuation or closeout.

## Privacy

Grounded packets and narratives exclude secrets, credentials, raw prompts,
chain-of-thought, provider-private transcripts, raw tool bodies, and unredacted
artifact content. Optional forensic transcript references follow the retention
and access rules in
[`task-decision-memory.md`](task-decision-memory.md) and remain outside normal
explain output.

## Delivery sequence

1. Complete common-path and task-decision primitives.
2. Implement deterministic `explain code` packet generation.
3. Pilot packet usefulness with maintainers and replacement agents.
4. Add optional advisory-provider narrative generation.
5. Measure citation accuracy, unknown labeling, resume quality, authoring cost,
   and incorrect inference rate before broader promotion.

The release must state which steps are implemented, advisory, experimental, or
planned. Product positioning must not outrun executable behavior.
