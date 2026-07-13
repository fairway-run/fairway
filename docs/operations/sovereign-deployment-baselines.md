# Sovereign Deployment Baselines

FW-348 packages three versioned Fairway deployment posture baselines:

- `examples/sovereign-deployment-baselines/v1/single-host.yaml`;
- `examples/sovereign-deployment-baselines/v1/managed-service.yaml`; and
- `examples/sovereign-deployment-baselines/v1/container-orchestration.yaml`.

They define engineering expectations for restricted-technology and sovereign
deployment preparation. They are not installers, hardening scripts,
certifications, compliance statements, deployment approvals, or risk
acceptance.

## Control Coverage

Every baseline must retain the complete v1 control vocabulary:

- immutable verified artifact source and local assets;
- explicit customer data paths and signed or digest-pinned configuration;
- drift detection;
- dedicated service identity, least privilege, and customer secret storage;
- host or service firewalling and explicit network allowlists;
- non-root execution, read-only application filesystems, and SELinux, AppArmor,
  seccomp, or reviewed equivalent confinement;
- exact status, version, listener, pid, log, schema, and configuration readback;
- CPU, memory, storage, connection, file-descriptor, route-latency, backup, and
  recovery budgets;
- backup/restore, key recovery, upgrade/rollback, and disaster recovery.

The topology-specific text makes these controls concrete for a local host, an
internally managed service, or a container orchestration deployment. A custom
baseline can add controls but cannot remove the required v1 controls or the
authority prohibitions.

## Observation Packet

Copy the example observation and replace every reference with current evidence
from the exact deployment:

```bash
cp examples/sovereign-deployment-baselines/v1/single-host-observation.example.yaml \
  /private/path/fairway-single-host-observation.yaml
```

Each control result is one of:

| Status | Meaning |
|---|---|
| `pass` | The operator observed the expectation and recorded required evidence references. |
| `fail` | The observed posture contradicts the expectation. |
| `not_observed` | The required readback or proof is missing. |
| `not_applicable` | The platform cannot use that exact mechanism; this passes only when the baseline permits it and the result includes both rationale and evidence for an equivalent reviewed boundary. |

Evidence references are metadata only. Do not put secrets, tokens, private
keys, raw prompts, transcripts, raw tool bodies, generated-content dumps, or
unredacted customer data in an observation packet.

## Validate

```bash
fairway readiness deployment \
  --baseline examples/sovereign-deployment-baselines/v1/single-host.yaml \
  --observation /private/path/fairway-single-host-observation.yaml
```

Use `--format json` or global `--json` for a deterministic
`fairway.sovereign-deployment-baseline-report.v1` report. The report contains
the selected baseline identity/version/topology, observation identity/time,
counts, and sorted deviations with the expectation and suggested next action.
It exits nonzero when blocking deviations remain.

The validator reads two bounded local YAML files. It rejects symlinks, remote
URLs, unknown fields, multiple YAML documents, oversized files, unknown or
duplicate controls, baseline/observation identity mismatch, future timestamps,
private or executable content markers, and weakened authority declarations.
It does not run commands, inspect a remote service, apply configuration, repair
drift, deploy software, use credentials, send provider messages, or mutate
Fairway task state.

## Promotion Boundary

A `ready: true` report means only that the supplied observation packet has one
acceptable result and required evidence reference for every baseline control.
It does not establish that the evidence is authentic, sufficient for a
particular jurisdiction, or accepted by an assessor. Package the report and
underlying evidence through the assurance workflow, preserve independent
review, and leave certification or authorization conclusions to the qualified
external authority.
