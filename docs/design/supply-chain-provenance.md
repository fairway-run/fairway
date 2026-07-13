# Supply-Chain Provenance

Fairway records coordination provenance for governed agentic engineering. It
explains why work happened, who or what executed it, which evidence existed,
which reviews and gates were satisfied, and which commits or releases carried
the result.

Fairway is not a compiler, package manager, CI runner, artifact signer,
attestation authority, or runtime dependency. Source code, build artifacts,
release assets, CI/CD systems, signing keys, and deployment platforms remain
owned by the project and its existing supply-chain tooling. Fairway contributes
metadata that connects those systems to the human and provider coordination
record.

## Goals

- Give consumers a deterministic way to audit task-to-commit-to-release
  history.
- Keep work provenance separate from build provenance while allowing release
  attestations to reference Fairway bundles.
- Preserve privacy: store references, summaries, counts, and gate results, not
  raw prompts, transcripts, tool bodies, secrets, generated content dumps, or
  provider auth state.
- Support future SLSA, in-toto, or similar attestations without requiring those
  systems for the first implementation.
- Make retention, backup, and tamper-evidence requirements explicit before
  adding export and manifest commands.

## Non-Goals

- Fairway does not rebuild software, decide whether an artifact is trustworthy,
  or replace SBOM/signing/attestation systems.
- Fairway does not ingest private provider databases, chat transcripts, raw
  prompt bodies, raw tool call payloads, auth tokens, or generated output dumps.
- Fairway does not make provider usage, token cost, model choice, or adapter
  output a completion, review, merge, deploy, or release gate by itself.
- Fairway does not make the read-only dashboard a provenance mutation surface.
  Dashboard views may display provenance status and links; CLI/operator actions
  record provenance.

## Provenance Domains

| Domain | Owner | Fairway record |
|---|---|---|
| Source code | Project Git repository | Commit SHA, branch, changed path summary, task linkage |
| Build artifacts | CI/release tooling | Artifact URL or digest reference, release verification evidence |
| CI/CD | External CI/deploy system | Run URL/id, result, command text, evidence result, monitor handback |
| Fairway work provenance | Fairway DB | Task metadata, state history, checkpoints, sessions, handoffs, reviews, evidence refs, waits, batches |
| Provider usage | Provider/session adapters | Normalized usage counts and safe metadata only |
| Prompts and context packets | Fairway packet surfaces | Prompt-packet metadata, objective, scope, acceptance, forbidden actions, cited facts, evidence refs |
| Evidence | Operator/provider commands | Command text, result, artifact path/reference, artifact type, notes |
| Review and release decisions | Fairway reviews and release packets | Required domains, verdicts, reviewed commit, release verify outcome, waivers |

The design treats provenance as a read model over existing Fairway state first.
A future store change may add cached bundles or manifests, but the first
implementation should export from task, session, checkpoint, evidence, review,
notification, batch, usage, and release records instead of creating a second
source of truth.

## Privacy Boundary

Fairway provenance exports must not include:

- raw secrets, credentials, tokens, kubeconfigs, signing material, or provider
  auth state;
- raw prompt bodies by default;
- private transcripts or provider chat histories;
- raw tool request/response bodies;
- generated-content dumps;
- local paths that expose secrets unless the operator explicitly records a safe
  redacted artifact reference.

Allowed fields are bounded metadata:

- task IDs, titles, roles, owners, status, tags, profiles, risk, and acceptance;
- commit SHAs, branch names, changed-path summaries, release tags, and artifact
  digests or public/private references;
- command text that operators intentionally recorded as evidence;
- evidence result, artifact path/reference, artifact type, duration, and notes;
- review domains, verdicts, reviewed commit SHA, and reviewer identity;
- session IDs, providers, roles, backend labels, lifecycle timestamps, and
  transcript path references without transcript contents;
- provider usage counts, source/confidence, model labels, phase, and safe
  metadata;
- prompt-packet objective, scope, acceptance, forbidden actions, cited Fairway
  facts, validation gates, and evidence refs.

Public provenance path fields are repository-relative and slash-normalized.
Absolute paths inside the repository are rewritten relative to the repository
root. Absolute paths outside the repository, `file://` references, path
traversal, and Windows-drive paths outside the repository fail closed as
`<redacted-local-path>` with a privacy warning. Recorded command, acceptance,
and checkpoint text also removes repository roots and absolute local path
tokens. For remote URLs, Fairway preserves the scheme, host, and path, removes
userinfo, sanitizes query values and fragments, and always redacts known
credential-bearing query keys before export.

If a consumer needs prompt or transcript retention for another compliance
system, that system owns the content store. Fairway may link to an externally
controlled redacted artifact only after the operator records it as evidence.

## Task-To-Commit-To-Release Linkage

A complete provenance chain should answer:

1. Which Fairway task or batch authorized the work?
2. Which session or lane executed it?
3. Which evidence and checks were recorded before closeout?
4. Which review domains approved or requested changes?
5. Which commit SHA satisfied the task?
6. Which release or deploy run included that commit?
7. Which release verification checks, waivers, and known limits were recorded?

The preferred linkage is:

```text
task -> evidence/checkpoints/sessions/reviews -> commit_sha
commit_sha -> batch/release packet -> release tag/assets/verification
release tag -> provenance bundle or attestation reference
```

When a commit contains multiple task slices, the release provenance bundle
should list each task ID and the reviewed commit or range that carried it. When
work is docs-only or design-only, the same chain still applies: evidence and
review prove why the documentation changed and which boundary it described.

## Export Shape

`FW-232` adds task-scoped and range-scoped provenance export:

```bash
fairway provenance report --task <task-id> --format markdown
fairway provenance report --since 168h --format json
fairway provenance prompt-packet --task <task-id>
```

The canonical JSON object is deterministic enough for audit diffs:

```json
{
  "schema": "fairway.provenance.v1",
  "generated_at": "2026-06-29T00:00:00Z",
  "project": {
    "name": "fairway",
    "db_path": ".fairway/state.db",
    "config_path": ".fairway/config.toml"
  },
  "scope": {
    "task_id": "FW-231",
    "since": "",
    "until": ""
  },
  "tasks": [
    {
      "id": "FW-231",
      "title": "Define Fairway supply-chain provenance model",
      "status": "done",
      "role": "arch",
      "risk_level": "high",
      "commit_refs": ["<sha>"],
      "evidence_refs": [],
      "review_refs": [],
      "session_refs": [],
      "checkpoint_refs": [],
      "usage_refs": []
    }
  ],
  "privacy": {
    "raw_prompts_included": false,
    "transcripts_included": false,
    "tool_bodies_included": false,
    "generated_content_included": false,
    "redaction_applied": false
  },
  "warnings": []
}
```

Markdown exports should present the same fields in operator-readable sections:
scope, task summary, evidence, reviews, commits, release linkage, warnings, and
privacy statement.

Release refs must come from explicit release-run or release-verification
evidence, such as `artifact_type = release-verify`, `artifact_type =
release-run`, `fairway release verify`, or `fairway packet release-run`.
Ordinary evidence notes or commands that merely contain the word "release" do
not prove release lineage.

Release verification accepts an explicit provenance bundle reference:

```bash
fairway release verify \
  --version v0.1.2 \
  --tag v0.1.2 \
  --source-sha <sha> \
  --provenance-bundle artifacts/fairway-provenance-v0.1.2.json \
  ...
```

When a bundle path is supplied, the path must exist. Fairway warns when no
bundle is supplied, or when the bundle does not mention the release version or
source SHA. That keeps release provenance visible without making Fairway an
artifact signer, SBOM system, SLSA generator, or in-toto attestation authority.

## Release Assurance Bundle

`fairway release assurance export` packages candidate artifacts with generated
checksums and detached Ed25519 signatures plus the required evidence classes:
SBOM, VEX, dependency inventory, license inventory/disposition, source provenance, build
provenance, build recipe, test summary, and vulnerability disposition. The
signed manifest binds release version, source SHA, builder identity, policy
version, file digests, and measured SLSA properties. Signing material is read
only from the named environment variable.

`fairway release assurance verify` is offline and fail-closed. It requires a
pinned public key and exact expected version, source, builder, and policy. It
rejects missing or unknown files, digest or detached-signature mismatch,
missing evidence classes, and a manifest that claims a SLSA level. Measured
fields report only properties for which the release pipeline supplied evidence;
hermetic and reproducible remain false unless an independently reviewed build
process proves them. A valid bundle is release evidence, not certification,
dependency trust, deployment approval, or risk acceptance.

## Prompt-Packet Export

Prompt packets are provenance inputs, not arbitrary transcript storage. A task
prompt packet may include:

- objective and task acceptance;
- source facts cited from Fairway state, evidence, reviews, and checkpoints;
- intended files or surfaces;
- forbidden actions and trust boundaries;
- validation gates and required review domains;
- handoff instructions and next safe action;
- evidence refs already known.

It must not include raw previous chat, private transcripts, provider auth
state, secrets, raw tool bodies, or generated output dumps. When a task needs a
large context artifact, the packet should link to an evidence artifact or
memory packet reference and rely on that artifact's retention/redaction policy.

## Retention Classes

| Class | Examples | Retention posture |
|---|---|---|
| Source-control provenance | design docs, release notes, reviewed config, public assessments | Commit in Git when public and non-sensitive |
| Fairway DB provenance | tasks, state history, evidence refs, reviews, sessions, checkpoints, usage counts | Back up with the Fairway DB; export deterministic bundles for release/audit |
| Local scratch | tmp memory files, local logs, provider scratch output | Keep out of Git; promote only curated summaries or redacted artifacts |
| Sensitive evidence refs | private CI URLs, incident artifacts, security logs | Store reference and classification; external system owns access control |
| Compliance artifacts | signed attestations, hash manifests, archival bundles | Store in external archive or release assets; Fairway records digest/ref |

Backups must preserve enough state to reconstruct provenance exports: the
SQLite DB, active config, relevant source commits, and any exported provenance
bundle or hash manifest. Backups must not sweep in secrets just because a local
artifact path was recorded.

## Tamper-Evidence Requirements

`FW-234` adds the first content-free hash manifest surface:

```bash
fairway provenance manifest \
  --path artifacts/fairway-provenance-v0.1.2.json \
  --path artifacts/fairway-provenance-v0.1.2.md \
  --format json
```

The manifest hashes selected evidence or provenance exports with SHA-256. It
records path, byte count, hash, and status, but never embeds file content. It
reports missing files, rejects directories, and refuses suspicious
secret-bearing path names such as `secret`, `token`, `password`,
`credential`, `private-key`, `apikey`, or `api_key`.

The model requirement is:

- provenance bundles are deterministic for the same DB/config/source state;
- exported bundles can be hashed;
- a manifest can list bundle hashes and selected artifact hashes without
  copying artifact contents into Fairway;
- missing artifacts and changed artifact hashes are explicit findings;
- manifests never require copying sensitive artifact contents into Fairway.

Tamper-evidence is evidence of change, not proof of benign or malicious
intent. Operators still review findings against source control, release assets,
and external artifact stores.

## Consumer Audit Questions

A consumer should be able to ask:

- Why did this work happen?
- Which task/profile/risk boundary authorized it?
- Which provider/session/lane executed it?
- Which commands or artifacts proved it?
- Which review domains approved it, changed it, or waived it?
- Which commit and release carried it?
- Were there known blockers, warnings, or missing evidence?
- Were any sensitive data classes intentionally excluded?

Fairway should answer these questions from recorded metadata and references,
without requiring a provider chat replay.

## Follow-On Slices

- `FW-232`: implemented deterministic provenance report and prompt-packet
  export.
- `FW-233`: link release-run packets and release verification to provenance
  bundles or attestation references.
- `FW-234`: add hash manifests, retention guidance, and backup posture.
- `FW-236`: add a safe evidence artifact viewer and redaction gate before any
  dashboard artifact preview.
