#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
OUT=${1:-}
WORK=$(mktemp -d "${TMPDIR:-/tmp}/fairway-restricted-advisory.XXXXXX")
cleanup() {
  rm -rf "$WORK"
}
trap cleanup EXIT

if [[ -z "$OUT" ]]; then
  echo "usage: restricted_advisory_rehearsal.sh <new-artifact-directory>" >&2
  exit 2
fi
OUT=$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")
if [[ -e "$OUT" ]]; then
  echo "output path already exists" >&2
  exit 1
fi

BIN=${FAIRWAY_BIN:-"$WORK/fairway"}
if [[ -z "${FAIRWAY_BIN:-}" ]]; then
  GOCACHE=${GOCACHE:-/tmp/fairway-go-cache} go build -o "$BIN" "$ROOT/cmd/fairway"
fi

openssl genpkey -algorithm ED25519 -out "$WORK/advisory-key.pem" >/dev/null 2>&1
export FAIRWAY_REHEARSAL_ADVISORY_PRIVATE_KEY
export FAIRWAY_REHEARSAL_ADVISORY_PUBLIC_KEY
FAIRWAY_REHEARSAL_ADVISORY_PRIVATE_KEY=$(openssl pkey -in "$WORK/advisory-key.pem" -outform DER | base64 | tr -d '\n')
FAIRWAY_REHEARSAL_ADVISORY_PUBLIC_KEY=$(openssl pkey -in "$WORK/advisory-key.pem" -pubout -outform DER | base64 | tr -d '\n')

cat >"$WORK/advisory.json" <<'JSON'
{
  "schema": "fairway.security-advisory.v1",
  "id": "FAIRWAY-SA-2099-001",
  "published_at": "2099-01-02T03:04:05Z",
  "severity": "high",
  "summary": "Synthetic restricted-channel advisory rehearsal",
  "affected_versions": ["9.9.8"],
  "fixed_versions": ["9.9.9"],
  "mitigations": ["Retain the prior verified rollback bundle until local acceptance completes"],
  "vex_updates": [
    {
      "vulnerability_id": "CVE-2099-0001",
      "status": "fixed",
      "justification": "Synthetic channel rehearsal only"
    }
  ],
  "patch_bundle_id": "fairway-offline-9.9.9",
  "rollback_bundle_id": "fairway-offline-9.9.8",
  "support_track": "lts",
  "synthetic": true,
  "authority_boundary": ""
}
JSON
printf '%s\n' 'synthetic opaque offline patch bundle; do not install' >"$WORK/patch.bin"

"$BIN" security advisory export \
  --advisory "$WORK/advisory.json" \
  --patch-bundle "$WORK/patch.bin" \
  --out "$WORK/exported" \
  --signing-key-env FAIRWAY_REHEARSAL_ADVISORY_PRIVATE_KEY >/dev/null

mkdir -p "$WORK/removable-media"
cp -R "$WORK/exported" "$WORK/removable-media/package"
rm -rf "$WORK/exported"

"$BIN" security advisory verify \
  --dir "$WORK/removable-media/package" \
  --expected-id FAIRWAY-SA-2099-001 \
  --expected-patch-bundle-id fairway-offline-9.9.9 \
  --expected-rollback-bundle-id fairway-offline-9.9.8 \
  --trusted-public-key-env FAIRWAY_REHEARSAL_ADVISORY_PUBLIC_KEY \
  --format json >"$WORK/verification.json"

"$BIN" security advisory acknowledge \
  --dir "$WORK/removable-media/package" \
  --expected-id FAIRWAY-SA-2099-001 \
  --expected-patch-bundle-id fairway-offline-9.9.9 \
  --expected-rollback-bundle-id fairway-offline-9.9.8 \
  --trusted-public-key-env FAIRWAY_REHEARSAL_ADVISORY_PUBLIC_KEY \
  --customer-ref restricted-lab-001 \
  --status received \
  --at 2099-01-02T04:00:00Z \
  --out "$WORK/acknowledgement.json" >/dev/null

cp -R "$WORK/removable-media/package" "$WORK/tampered"
printf '%s\n' 'tamper' >>"$WORK/tampered/patch/patch-bundle.bin"
if "$BIN" security advisory verify \
  --dir "$WORK/tampered" \
  --expected-id FAIRWAY-SA-2099-001 \
  --expected-patch-bundle-id fairway-offline-9.9.9 \
  --expected-rollback-bundle-id fairway-offline-9.9.8 \
  --trusted-public-key-env FAIRWAY_REHEARSAL_ADVISORY_PUBLIC_KEY >/dev/null 2>&1; then
  echo "tampered advisory package unexpectedly verified" >&2
  exit 1
fi

unset FAIRWAY_REHEARSAL_ADVISORY_PRIVATE_KEY FAIRWAY_REHEARSAL_ADVISORY_PUBLIC_KEY
rm -f "$WORK/advisory-key.pem"

mkdir -p "$OUT"
cp -R "$WORK/removable-media/package" "$OUT/package"
cp "$WORK/verification.json" "$WORK/acknowledgement.json" "$OUT/"
patch_sha=$(shasum -a 256 "$OUT/package/patch/patch-bundle.bin" | awk '{print $1}')
cat >"$OUT/summary.json" <<JSON
{
  "schema": "fairway.restricted-advisory-rehearsal.v1",
  "result": "pass",
  "advisory_id": "FAIRWAY-SA-2099-001",
  "patch_bundle_id": "fairway-offline-9.9.9",
  "rollback_bundle_id": "fairway-offline-9.9.8",
  "patch_sha256": "$patch_sha",
  "signature_verified_with_pinned_key": true,
  "inventory_verified": true,
  "acknowledgement_recorded": true,
  "tampered_patch_rejected": true,
  "patch_imported": false,
  "deployed": false,
  "synthetic": true,
  "ephemeral_private_key_retained": false,
  "authority_boundary": "channel rehearsal only; not vulnerability remediation, patch import, deployment approval, certification, compliance, or risk acceptance"
}
JSON

printf 'restricted_advisory_rehearsal: pass\nartifact: %s\n' "$OUT"
