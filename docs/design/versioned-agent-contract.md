# Versioned Project Agent Contract

## Purpose

Fairway evolves independently from provider models and from the repositories
that use it. A binary upgrade must not silently rewrite project process, while
an old generated agent file must not prevent projects from adopting improved
coordination behavior.

The project contract therefore has its own compatibility identity. Binary
SemVer identifies the executable; agent-contract schema and revision identify
workflow compatibility.

## Managed Contract

`fairway init` writes `.fairway/AGENTS.md` as a managed contract with:

```text
schema
revision
generated_by
content_sha256
```

- `schema` changes for incompatible contract structure or behavior.
- `revision` changes for compatible process guidance updates.
- `generated_by` records provenance but does not itself require an update.
- `content_sha256` detects edits inside the managed contract.

Repository-specific instructions belong in `.fairway/AGENTS.local.md`. The
managed contract tells agents to read that file after the Fairway contract.

## Upgrade States

```text
missing
legacy_unmanaged
current
update_available
locally_modified
incompatible
```

The command surface is:

```bash
fairway agent-contract status
fairway agent-contract plan
fairway agent-contract apply
fairway agent-contract apply --adopt-legacy
```

`status` and `plan` are read-only. `apply` writes atomically. Legacy adoption
copies the existing unversioned file to `.fairway/AGENTS.local.md` before
writing the managed contract. It refuses to overwrite an existing local file.

`fairway preflight` includes the comparison automatically. Missing, legacy, or
older compatible contracts produce warnings. Locally modified managed content
and contracts newer than the running binary produce preflight issues.

Locally modified managed content fails closed. The operator must move local
policy into `AGENTS.local.md` and restore the managed block before applying an
update. A contract with a schema newer than the binary supports requires a
binary upgrade.

## Binary Upgrade Semantics

A new Fairway binary does not update project process when the contract revision
is unchanged. This avoids coupling unrelated binary fixes to agent behavior.

When the embedded revision changes:

1. `status` reports `update_available`;
2. `plan` reports the target schema, revision, action, and compatibility;
3. the operator reviews the release guidance;
4. `apply` replaces only the verified managed contract;
5. repository-local instructions remain separate and unchanged.

`fairway init --refresh-agent-contract` uses the same safe apply path. It may
adopt a legacy contract, but it does not overwrite modified managed content.

## Model Evolution

Provider model behavior can change rapidly, but project authority must remain
stable. Model-specific prompting guidance may evolve within a compatible
revision. Changes to source-of-truth rules, authority boundaries, required
state transitions, or destructive-action policy require a schema or reviewed
revision change rather than an implicit binary-side prompt update.

The contract remains provider-neutral. Codex, Claude, Gemini, shell, and future
providers consume the same project contract and local extension.
