# Sovereign Customer Key Rehearsal

`fairway security rehearsal run` is an offline engineering exercise for the
customer-controlled identity and audit boundaries used by Fairway sovereign
profiles. It is intended for a disposable, network-denied Linux environment.
It is not a production key ceremony, identity provider, HSM integration, FIPS
validation, certification, deployment approval, or risk acceptance.

## Run

Provide an existing operator-controlled `tmpfs` directory for all private
material and a new directory on a distinct retained-evidence mount:

```bash
fairway security rehearsal run \
  --workspace /run/fairway-rehearsal \
  --out /evidence/fairway-customer-key-rehearsal \
  --project fairway-sovereign-rehearsal \
  --at 2026-07-12T12:00:00Z
```

`--at` must be within five minutes of the running host clock so the retained
report cannot be silently backdated. The operator remains responsible for the
host's trusted-time posture.

The command verifies `/proc/self/mountinfo`, refuses a workspace that is not
Linux `tmpfs`, and rejects retained output on the same or another tmpfs mount.
The evidence mount must be distinct and non-tmpfs so it survives private tmpfs
disposal. It generates three distinct Ed25519 roots: initial identity,
recovery identity, and audit signing. Private files are regular files with mode
`0600`; they stay under the tmpfs workspace and are removed before success.
Fairway does not print, log, persist in its project database, or retain the
private values. Removal is lifecycle evidence, not a secure-erasure claim for
arbitrary media; tmpfs disposal remains an operator responsibility.

The in-process rehearsal uses the real sovereign shared-API handler and signed
audit exporter to prove:

- a fresh viewer proof can read bounded API status;
- an operator proof cannot use the viewer read command;
- proof revocation, missing verification key, and key substitution fail closed;
- a distinct recovery root restores bounded viewer authorization;
- the audit package verifies under its pinned audit public key; and
- substituting another generated public key for the audit root is rejected.

No network listener is started and no request leaves the process. The command
does not download packages or invoke a provider, notifier, deployment, release,
dashboard mutation, approval, merge, public-exposure, or live-operation path.

## Retained Evidence

The output directory contains only:

- `report.json` with pass/fail states, public-key fingerprints, non-claims, and
  the authority boundary;
- initial and recovery identity public keys;
- the audit public key; and
- a signed audit export containing bounded metadata and no raw detail content.

Use `scripts/ci/sovereign_customer_key_rehearsal.sh` in a pinned Linux image.
The wrapper requires `/run` tmpfs, invokes the command, checks the report and
signed audit files, and rejects retained private/secret/token filenames and
common private-key or bearer-token markers. The customer still owns key
generation policy, custody, issuer hardening, revocation governance, trusted
time, independent retention, and production recovery.
