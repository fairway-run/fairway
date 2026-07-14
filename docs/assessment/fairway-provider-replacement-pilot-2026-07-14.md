# Fairway Provider Replacement Pilot

Date: 2026-07-14

Task: `FW-336`

## Objective

Prove that a replacement provider can resume one bounded, reversible Fairway
documentation task from repository and Fairway facts without access to the
source provider's private transcript.

## Source Provider Boundary

- provider: Codex
- session: `codex-fw-336-provider-source`
- started: `2026-07-14T18:45:40Z`
- source revision: `d1698c5`
- baseline worktree: clean
- Fairway evidence: `943`
- original Fairway decision: `9` selected Claude as the replacement provider
- execution Fairway decision: `10` superseded decision `9` after the Claude
  and Gemini provider surfaces failed before repository access, and selected a
  fresh Codex CLI session as the bounded fallback
- bounded implementation: add this assessment and an optional
  provider-replacement quickstart focused on user value

The source provider intentionally stopped after recording the task attachment,
active checkpoint, Git baseline, bounded evidence, decision, and this scaffold.
The replacement provider must not receive or depend on the source provider's
private transcript.

## Replacement Provider Readback

Recovered by the replacement provider from `AGENTS.md`, required repo
guidance, `fairway task-detail FW-336`, `fairway decision list FW-336`, Git
status/diff/log, Fairway checkpoints, and the repository docs. No private
provider transcript or provider chat/session file was read.

- provider and session: Codex CLI, session
  `codex-cli-fw-336-provider-replacement`
- replacement type: same-vendor fresh-session replacement. This proves that a
  new Codex attachment can resume from durable Fairway and repository facts; it
  does not prove cross-vendor replacement completed successfully.
- resume timestamp: `2026-07-14T18:48:25Z` from the recorded Fairway session
  and checkpoint `691`
- recovered facts:
    - `FW-336` is `in_progress`, owned by `governance`, and depends on
      `FW-335`.
    - acceptance requires a real bounded source-provider start, replacement
      without private transcript access, and an optional value-focused second
      quickstart with timing, recovered facts, rough edges, cleanup, and result.
    - decision `9` originally chose Claude for this optional quickstart and
      assessment. Decision `10` superseded that provider choice after Claude
      and Gemini failed before repository access, preserved those attempts as
      evidence, added `docs/provider-replacement-quickstart.md` to task scope,
      and selected a fresh Codex CLI session as the bounded fallback.
    - source-provider evidence recorded a clean `main` baseline at
      `d1698c5`, then created only this assessment scaffold and stopped before
      implementing the optional quickstart.
    - handoff `38` instructed the replacement provider to complete the
      assessment and quickstart from Fairway and Git facts only.
    - checkpoint `687` recorded source-provider intent; checkpoints `689` and
      `690` recorded failed Claude and Gemini replacement starts before repo
      access; checkpoint `691` recorded this fresh Codex replacement.
    - current Git status before implementation had only this untracked
      assessment file, and `HEAD` remained `d1698c5`.
- unrecoverable context:
    - source-provider private reasoning, prompt wording, and any transcript-only
      notes;
    - exact evidence row numeric ids beyond those rendered in task detail and
      handoff text;
    - whether the source provider had unpublished preferences for page wording
      beyond the scaffold and decision;
    - cross-vendor implementation behavior, because Claude failed with HTTP
      401 before repository access and Gemini failed because the installed
      client tier was unsupported.
- implementation completed:
    - filled this assessment with recovered facts, missing context, elapsed
      timing, rough edges, cleanup, and result;
    - added
      [Provider Replacement Quickstart](../provider-replacement-quickstart.md);
    - linked the optional quickstart from [Quickstart](../quickstart.md).
- validation:
    - scoped whitespace check passed:

      ```bash
      git diff --check -- \
        docs/assessment/fairway-provider-replacement-pilot-2026-07-14.md \
        docs/provider-replacement-quickstart.md \
        docs/quickstart.md
      ```

    - `npx --yes markdownlint-cli2 ...` was attempted as a scoped Markdown
      check but could not install because registry access failed with
      `ENOTFOUND`; no local `markdownlint` binary was available.
    - local line-length scan returned no lines over 100 characters.
    - `GOCACHE=/tmp/fairway-go-cache go test ./docs` passed.
- elapsed time: same-vendor replacement session started at
  `2026-07-14T18:48:25Z`; final scoped validation completed at
  `2026-07-14T18:53:34Z`, for roughly 5 minutes 9 seconds from replacement
  session start to validated local diff. Cross-vendor attempts did not reach
  repository access, so no cross-vendor elapsed implementation time is claimed.
- rough edges and extra ceremony:
    - `fairway checkpoint status` does not accept a task id, so the replacement
      provider had to filter global checkpoint output manually.
    - task detail rendered evidence but not the source-provider evidence row
      numbers referenced by the decisions; the handoff text preserved enough
      context to continue.
    - provider replacement required reading several governance docs before a
      documentation-only change; this is correct for repository contributors
      but heavier than a consumer quickstart should feel.
    - cross-vendor replacement was blocked by local provider availability, not
      by Fairway task recovery.
- cleanup proof:
    - no task state, review state, commits, staging, pushes, dashboards,
      releases, or external systems were changed by this replacement provider;
    - the implementation is documentation-only and remains reversible as a Git
      diff.

## Result

The same-vendor fresh-session replacement succeeded for this bounded
documentation implementation. The replacement provider recovered intent,
scope, baseline Git state, prior provider failures, and next actions from
Fairway and repository facts without private transcript access.

The result should be treated as validated practice for one internal task, not a
universal product claim. Cross-vendor replacement remains partially exercised:
Fairway preserved the handoff boundary when Claude and Gemini failed before
repository access, but those attempts did not complete implementation.
