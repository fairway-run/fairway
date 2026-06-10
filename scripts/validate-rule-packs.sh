#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -eq 0 ]]; then
  cat >&2 <<'USAGE'
usage: scripts/validate-rule-packs.sh <rule-pack-dir>...

Validates local Fairway rule-pack directories with `fairway rules validate`.
Set FAIRWAY_BIN=/path/to/fairway to use a checked-out or downloaded binary.
USAGE
  exit 2
fi

fairway_bin="${FAIRWAY_BIN:-fairway}"

for pack in "$@"; do
  echo "validating rule pack: ${pack}"
  "${fairway_bin}" rules validate "${pack}"
done
