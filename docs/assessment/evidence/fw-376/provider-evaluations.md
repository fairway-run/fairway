# FW-376 Isolated Provider Evaluations

This file is a bounded evaluation summary, not a full provider transcript. It
retains the fixed evaluation contract, decisive excerpts, findings, and counts
used for the pilot decision.

## Evaluation Contract

Each provider received only the named JSON packet and was prohibited from
reading repositories, `tmp-ux`, Git history, prior evaluations, or external
sources. The provider had to report the objective, authority decisions,
blockers, next action, answer the node/terminal identity question, classify
claim authority, identify unsupported claims, and provide explicit counts for
clarifications, repeated investigation, and authority-selection errors.

## Initial Evaluation

Packet: `cold-start-before-review.json`

The provider recovered the kind security-matrix objective, authorization
separation, external-host blockers, and dependency-safe next action with zero
clarifications and zero repeated investigations. It also identified ambiguous
active/done checkpoint history and an overly broad blocker statement.

Independent review then found an issue the provider missed: the packet marked
the lifecycle model canonical even though its own frontmatter declared
`source_of_truth: false` and `implementation_state: not-assessed`. The initial
authority-error result is therefore recorded as one, not zero.

## Corrected Evaluation

Packet: `cold-start-corrected.json`

The isolated provider reported:

> Build the SPIFFE node-agent and terminal identity security-failure matrix,
> grounded in the canonical ADR and existing local proofs, while explicitly
> leaving external-host attestation, high-assurance attestation, SPIRE HA, and
> production operations gated.

It classified the evidence as follows:

> Canonical claims: the verified SPIFFE ADR-derived pages and their authority
> boundaries. Operational/draft design: `architecture/node-agent-lifecycle.md`
> and `open-questions.md`; explicitly non-authoritative and
> implementation-not-assessed. Live task facts: current Fairway statuses,
> blockers, checkpoints, and clean Git state, not architecture authority.

It refused to overstate the cross-task answer:

> The packet does not establish that SPIFFE identity alone provides one-way or
> audited access. It lacks authoritative protocol details for task
> directionality, signatures, replay protection, audit records, correlation,
> and rejection behavior. Those properties appear only as incomplete
> operational design intent.

Its precise next action was to start the kind-capable matrix cases while
marking external-host, high-assurance, HA, and production-operation cases as
gated by their blocked pilots.

Remaining packet-quality findings were historical active/done excerpts without
chronology labels, duplicated blockers, generated inspect-status next actions,
source-SHA/current-commit ambiguity, and live checkpoint claims that must not
be read as canonical evidence.

Final counts:

- clarification questions required: 0;
- repeated investigations needed before starting: 0;
- authority-selection errors: 0.
