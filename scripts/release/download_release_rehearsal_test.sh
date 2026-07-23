#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
script="$root/scripts/release/download_release_rehearsal.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fake_bin="$tmp/bin"
mkdir -p "$fake_bin"
cat > "$fake_bin/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$1 $2" == "api repos/fairway-run/fairway/actions/runs/123" ]]; then
  cat <<JSON
{"status":"completed","conclusion":"success","event":"workflow_dispatch","head_sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","head_branch":"main","path":".github/workflows/release-rehearsal.yml"}
JSON
elif [[ "$1 $2" == "api repos/fairway-run/fairway/actions/runs/123/artifacts?per_page=100" ]]; then
  cat <<JSON
{"artifacts":[{"name":"fairway-release-rehearsal-v1.2.3-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expired":false}]}
JSON
elif [[ "$1 $2" == "run download" ]]; then
  if [[ "${FAKE_GH_DOWNLOAD_FAIL:-0}" == "1" ]]; then
    exit 1
  fi
  while (($#)); do
    if [[ "$1" == "--dir" ]]; then
      shift
      printf '{"schema":"fairway.release-rehearsal.v1"}\n' > "$1/rehearsal.json"
      exit 0
    fi
    shift
  done
  exit 1
else
  printf 'unexpected gh invocation: %s\n' "$*" >&2
  exit 1
fi
EOF
chmod +x "$fake_bin/gh"

export PATH="$fake_bin:$PATH"
export GH_TOKEN=test-only
output="$tmp/output"
"$script" fairway-run/fairway v1.2.3 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 123 "$output" >/dev/null
test -f "$output/rehearsal.json"

if "$script" fairway-run/fairway v1.2.3 bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb 123 "$tmp/wrong-sha" >/dev/null 2>&1; then
  printf 'wrong source SHA unexpectedly passed\n' >&2
  exit 1
fi
if "$script" fairway-run/fairway v1.2.3 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 0 "$tmp/wrong-run" >/dev/null 2>&1; then
  printf 'invalid run id unexpectedly passed\n' >&2
  exit 1
fi

export FAKE_GH_DOWNLOAD_FAIL=1
failed_output="$tmp/download-failure"
if "$script" fairway-run/fairway v1.2.3 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 123 "$failed_output" >/dev/null 2>&1; then
  printf 'failed artifact download unexpectedly passed\n' >&2
  exit 1
fi
test ! -e "$failed_output"
if find "$tmp" -maxdepth 1 -name '.fairway-release-rehearsal.*' -print -quit | grep -q .; then
  printf 'failed artifact download left a staging directory\n' >&2
  exit 1
fi
printf 'download release rehearsal tests passed\n'
