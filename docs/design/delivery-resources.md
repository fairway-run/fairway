# Typed Delivery Resources

Fairway delivery resources are read-only records derived from existing task and evidence state. They make operational objects visible without adding a second store or granting operational authority.

## Resource Classes

The first projection covers resources that repeatedly appear in release, deploy, and handoff work:

- environments
- dashboards
- documentation portals
- binaries
- release artifacts
- CI pipelines
- preflight packets
- rehearsal targets

The projection is intentionally conservative. It classifies a resource only when task metadata or evidence clearly references one of these resource classes.

## Record Shape

Each resource row includes:

- project
- type
- name
- owner
- current state
- source task ID and title
- provenance
- last verified timestamp
- last verified commit or version when evidence includes it
- required evidence
- open blockers
- next safe action
- evidence references

The source task remains the durable work unit. The resource row is an operator-facing read model over that task, not a replacement for task state.

## State Model

Resource state is derived from task status and evidence:

- `verified`: latest meaningful evidence passed.
- `handoff_ready`: the source task is done and the latest passing evidence is a readiness, handoff, or packet artifact.
- `stale`: latest passing evidence is older than the refresh window.
- `recorded`: the source task is done but current verification evidence is absent.
- `unverified`: no accepted verification evidence exists yet.
- `needs_attention`: latest evidence is partial.
- `failed_verification`: latest evidence failed or was blocked.
- `blocked`: the source task is blocked.

Failed, partial, and blocked evidence is treated as an open blocker until a newer accepted verification row supersedes it.

## Surfaces

The CLI exposes the projection through:

```bash
fairway delivery resources [--type <type>] [--project <project>] [--stale] [--format text|json]
```

The dashboard reports page includes a Delivery Resources panel. The JSON reports export includes the same resource records.

## Authority Boundary

Delivery resources are read-only. They do not authorize:

- deployment
- rollback
- dashboard restart
- docs publish
- release tagging or publication
- DNS, tunnel, or public exposure changes
- review approval
- merge
- live operations

The `next_safe_action` field is guidance for the next task or evidence step. It is not an execution grant.

## Privacy Boundary

The projection stores and renders metadata only. It does not ingest artifact contents, secrets, credentials, raw provider transcripts, prompt bodies, auth tokens, cookies, or raw logs. Artifact paths remain references to existing evidence rows and must still follow the evidence artifact viewer redaction boundary before content is displayed.

## Multi-Project Behavior

When the dashboard attaches multiple registered Fairway projects, delivery resource names are scoped by project. Duplicate names across projects are valid and remain distinguishable through the `project` and `source_task_id` fields.

## Future Work

Future slices can replace the initial classifier with explicit resource facts emitted by packet templates, release verification, dashboard lifecycle commands, and deploy preflight commands. That should still be projected from existing facts unless a separately reviewed store migration is justified.
