#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: sovereign_customer_key_rehearsal.sh <new-artifact-directory>" >&2
  exit 2
fi

fairway_bin="${FAIRWAY_BIN:-fairway}"
artifact_dir="$1"
if [[ -e "$artifact_dir" ]]; then
  echo "artifact directory already exists: $artifact_dir" >&2
  exit 1
fi
if [[ ! -d /run ]]; then
  echo "rehearsal requires Linux /run tmpfs" >&2
  exit 1
fi

workspace="$(mktemp -d /run/fairway-sovereign-keys.XXXXXX)"
cleanup() {
  rm -rf "$workspace"
}
trap cleanup EXIT

"$fairway_bin" security rehearsal run \
  --workspace "$workspace" \
  --out "$artifact_dir" \
  --project fairway-sovereign-rehearsal \
  --at "$(date -u +%Y-%m-%dT%H:%M:%SZ)"

test -f "$artifact_dir/report.json"
test -f "$artifact_dir/audit-export/manifest.json"
test -f "$artifact_dir/audit-export/signature.json"
grep -q '"ok": true' "$artifact_dir/report.json"
grep -q '"private_keys_destroyed": true' "$artifact_dir/report.json"
if find "$artifact_dir" -type f \( -iname '*private*' -o -iname '*secret*' -o -iname '*token*' \) -print -quit | grep -q .; then
  echo "retained rehearsal output contains a forbidden private/secret/token filename" >&2
  exit 1
fi
if grep -R -E -i 'BEGIN[[:space:]]+(OPENSSH|RSA|EC|PRIVATE)|authorization:[[:space:]]*bearer|private[_ -]?key[[:space:]]*[:=]' "$artifact_dir" >/dev/null; then
  echo "retained rehearsal output failed secret marker scan" >&2
  exit 1
fi

echo "sovereign customer-key rehearsal passed: $artifact_dir"
