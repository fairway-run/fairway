# Fairway v0.1.12 Release Preparation

Date: 2026-07-11

Task: FW-311

## Candidate

- Version candidate: `v0.1.12`.
- Previous released tag: `v0.1.11` at
  `3a4a496acbb9d71f9aa783d265609cf767c09d6d`.
- Source SHA before this release-prep documentation commit:
  `fb2973a172a767acb1f65cf75516457a21bead01`.
- Final release source SHA must be the reviewed, committed, pushed FW-311
  closeout SHA. A separate publish task must record that exact SHA before tag
  creation.
- Scope: reviewed changes after `v0.1.11` for common-path automation,
  decision/track memory, AI Cloud consumer gaps, measured advisory intent-diff,
  deterministic grounded explanation, and optional advisory narrative.

## Included Work

- FW-291, FW-292, and FW-305: common-path contract, atomic work start/status,
  and stable provider lifecycle identity in custom summaries.
- FW-303 and FW-307: privacy-bounded task decision records, independent quality
  assessment, supersession, accepted scope additions, and lifecycle design.
- FW-293: guarded work verification and closeout composed from existing Fairway
  gates.
- FW-306 and FW-294: track-memory lifecycle history plus progressive
  common-path dashboard guidance.
- FW-299, FW-297, FW-298, FW-300, and FW-301: lifecycle-aware failure routing,
  reviewer-domain route preflight, wait hygiene, managed binary cache, and
  consumer capability/minimum-version readiness.
- FW-295: measured common-path pilot with exact read-only SQL and an explicit
  decision not to promote blocking reversible-work deviations from insufficient
  precision data.
- FW-304: deterministic advisory intent-to-diff classification using declared
  scope and independently accepted decisions.
- FW-308, FW-309, and FW-310: agent-native explainability contract,
  deterministic grounded code packet, and optional citation-validated
  loopback advisory narrative.
- FW-312: deterministic append-order latest-review reads discovered and fixed
  during full train validation.

All listed implementation and design tasks are `done` with their configured
reviews approved. The parent FW-290 remains an organizational epic and is not a
publish authority.

## Measured Promotion Decision

The FW-295 pilot showed materially shorter active-to-done and
validation-to-done time with complete session, active-checkpoint, and passing
evidence coverage. It also showed increased review/notification overhead and
did not contain labeled intent-to-diff precision or false-positive data.
Therefore:

- keep the common path;
- keep reversible intent-to-diff classification advisory;
- gather labeled outcomes before any later promotion proposal; and
- keep existing security, live, deploy, release, credential, public-exposure,
  migration, irreversible, and other consequential controls blocking.

## Explainability Boundary

- `fairway explain code` exports deterministic recorded Git/Fairway facts,
  conflicts, missing provenance, and reference-only inference inputs.
- It does not emit source bodies, raw prompts, private transcripts, raw tool
  bodies, generated-content dumps, credentials, or secrets.
- The optional narrative path supports only explicitly configured loopback
  `local_ollama`. It rejects redirects, proxies, non-loopback resolution,
  unknown citations, unsupported/trailing JSON, secret-like output, prompts
  over 256 KiB, and responses over 64 KiB.
- Generated narrative is display-only. It is not deterministic provenance and
  is never written back as evidence, a task decision, review, or history.
- No credentialed remote provider path, dashboard send/write authority,
  approval, merge, deploy, release, public exposure, or live-operation
  authority is added.

## Validation Packet

Each included implementation slice recorded focused tests, full tests, vet,
diff/config/workflow/reconcile evidence, independent required reviews, exact
commit evidence, and exact-source CI/docs deployment evidence.

FW-311 release-prep validation requires:

- `go test ./...`;
- `go test -tags=integration ./...`;
- `go vet ./...`;
- `git diff --check`;
- YAML parse for the product backlog;
- `go run ./cmd/fairway config validate`;
- `go run ./cmd/fairway workflow check`;
- `go run ./cmd/fairway reconcile active --dry-run`;
- `goreleaser check`;
- a local binary built with release ldflags that reports `0.1.12` and passes
  `ready` plus `reconcile active --dry-run` smoke.

Before tag creation, the publish task must additionally pass:

- `go run ./cmd/fairway workflow check --mode deploy --require-clean --require-pushed`;
- exact local/remote source SHA readback;
- absence of an existing local or remote `v0.1.12` tag; and
- release credential/workflow readiness without exposing secret values.

## Separate Release Boundaries

FW-311 authorizes release preparation only. It does not authorize a tag,
GitHub/GoReleaser publication, Homebrew update, public release declaration,
dashboard restart, or public exposure change.

A separate reviewed publish task must own:

- annotated `v0.1.12` tag creation at the exact reviewed source SHA;
- push to the normal release remote;
- GitHub Actions/GoReleaser monitoring;
- release asset, checksum, Homebrew tap/cask, and `brew fetch` verification; and
- release/docs evidence.

A separate reviewed dashboard lifecycle task must own:

- installation of the released binary outside consumer worktrees;
- restart of read-only/public port 7878 and local full-access port 7879;
- distinct durable pid/log files;
- status/version/binary/config/listen/mode readback; and
- local HTTP plus Cloudflare Access boundary probes.

## Known Limits

- Reversible intent-to-diff remains advisory pending measured precision.
- LLM output is optional, local-only in this slice, and non-authoritative.
- Go is the only committed symbol language resolved by the first explain slice.
- Managed binary lifecycle does not download or restart automatically.
- Consumer readiness reports do not install, migrate, or upgrade.
- Shared-team write pilots, trusted-proxy verification, non-loopback server
  exposure, Postgres runtime storage, dashboard mutation, provider-send, and
  autonomous approval remain outside this release boundary.
