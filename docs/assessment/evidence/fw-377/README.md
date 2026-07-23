# FW-377 Evidence

This directory contains bounded evidence for the GPUaaS memory and knowledge
adoption increment.

## Packet artifacts

- `repair-cold-start.json`
- `domains-cold-start.json`
- `identity-cold-start.json`

Each packet was generated from clean GPUaaS commit `ec2b93f67` with an
8,192-byte knowledge budget. The corresponding `*-timing.txt` file records the
wall, user, and system time from `/usr/bin/time`.

## Evaluation

`evaluate.sh` is the retained machine-checkable rubric. Run it from any working
directory; it resolves this evidence directory, scans retained packets for
secret patterns without printing matched content, and validates
`evaluation-summary.json`. The evaluation reads only retained evidence. It
checks explicit memory/task state, historical checkpoint labels, deduplicated
guidance, source freshness, intended first-page selection, clean Git posture,
and budget compliance.

The evidence is a cold-start and authority-selection assessment. It is not
implementation, security, release, or production-readiness proof for the
referenced GPUaaS workstreams.
