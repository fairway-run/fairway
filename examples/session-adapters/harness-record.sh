#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "usage: harness-record.sh <harness-record-batch.json>" >&2
  exit 2
fi

fairway contract harness-record --format json >/dev/null
exec fairway harness ingest --file "$1"
