#!/usr/bin/env bash
set -euo pipefail

current_assurance=${1:?usage: build_offline_distribution_bundle.sh <current-assurance-dir> <rollback-assurance-dir> <output-root>}
rollback_assurance=${2:?rollback assurance directory is required}
output_root=${3:?output root is required}

: "${FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY:?FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY is required}"
: "${FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY:?FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY is required}"

for tool in go git jq shasum uname; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf 'required offline bundle build tool is missing: %s\n' "$tool" >&2
    exit 1
  }
done

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"
current_assurance=$(cd "$current_assurance" && pwd -P)
rollback_assurance=$(cd "$rollback_assurance" && pwd -P)
mkdir -p "$output_root"
output_root=$(cd "$output_root" && pwd -P)

read_manifest_field() {
  local manifest=$1 field=$2
  jq -er "$field | strings | select(length > 0)" "$manifest"
}

current_manifest="$current_assurance/manifest.json"
rollback_manifest="$rollback_assurance/manifest.json"
test -f "$current_manifest" && test -f "$rollback_manifest"
current_version=$(read_manifest_field "$current_manifest" '.version')
current_sha=$(read_manifest_field "$current_manifest" '.source_sha')
current_builder=$(read_manifest_field "$current_manifest" '.builder_id')
current_policy=$(read_manifest_field "$current_manifest" '.policy_version')
rollback_version=$(read_manifest_field "$rollback_manifest" '.version')
rollback_sha=$(read_manifest_field "$rollback_manifest" '.source_sha')
rollback_builder=$(read_manifest_field "$rollback_manifest" '.builder_id')
rollback_policy=$(read_manifest_field "$rollback_manifest" '.policy_version')

test "$(git rev-parse HEAD)" = "$current_sha" || {
  printf 'current release source sha does not match checked out HEAD\n' >&2
  exit 1
}

targets=(darwin_amd64 darwin_arm64 linux_amd64 linux_arm64)
for release in current rollback; do
  if [[ "$release" == current ]]; then
    dir=$current_assurance
    version=${current_version#v}
  else
    dir=$rollback_assurance
    version=${rollback_version#v}
  fi
  for target in "${targets[@]}"; do
    test -f "$dir/artifacts/fairway_${version}_${target}.tar.gz" || {
      printf '%s release assurance is missing %s archive\n' "$release" "$target" >&2
      exit 1
    }
  done
done

work=$(mktemp -d "${TMPDIR:-/tmp}/fairway-offline-build.XXXXXX")
cleanup() { rm -rf "$work"; }
trap cleanup EXIT
mkdir -p "$work/verifiers"

asset_args=()
for target in "${targets[@]}"; do
  goos=${target%_*}
  goarch=${target#*_}
  verifier="$work/verifiers/fairway-offline-verify_${target}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags '-s -w' -o "$verifier" ./cmd/fairway-offline-verify
  chmod 0755 "$verifier"
  asset_args+=(--asset "verifier:$(basename "$verifier")=$verifier")
done

for doc in \
  docs/operations/sovereign-offline-bundle.md \
  docs/operations/sovereign-deployment-baselines.md \
  docs/security/sovereign-deployment-ready.md \
  docs/security/release-assurance-bundle.md \
  docs/config-reference.md; do
  test -f "$doc"
  asset_args+=(--asset "documentation:$(basename "$doc")=$repo_root/$doc")
done
asset_args+=(--asset "configuration:fairway-config.toml=$repo_root/examples/fairway-config.toml")
for baseline in "$repo_root"/examples/sovereign-deployment-baselines/v1/*.{yaml,yml}; do
  test -f "$baseline" || continue
  asset_args+=(--asset "deployment_baseline:$(basename "$baseline")=$baseline")
done

bundle_dir="$output_root/fairway_${current_version#v}_offline_distribution"
test ! -e "$bundle_dir" || {
  printf 'offline bundle output already exists: %s\n' "$bundle_dir" >&2
  exit 1
}
created_at=''
if [[ -n "${SOURCE_DATE_EPOCH:-}" ]]; then
  if created_at=$(date -u -r "$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null); then
    :
  else
    created_at=$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)
  fi
else
  created_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
fi

go run ./cmd/fairway release offline export \
  --out "$bundle_dir" \
  --current-assurance-dir "$current_assurance" \
  --rollback-assurance-dir "$rollback_assurance" \
  --trusted-public-key-env FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY \
  --signing-key-env FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY \
  --created-at "$created_at" \
  --current-version "$current_version" \
  --current-source-sha "$current_sha" \
  --current-builder-id "$current_builder" \
  --current-policy-version "$current_policy" \
  --rollback-version "$rollback_version" \
  --rollback-source-sha "$rollback_sha" \
  --rollback-builder-id "$rollback_builder" \
  --rollback-policy-version "$rollback_policy" \
  "${asset_args[@]}"

host_os=$(uname -s | tr '[:upper:]' '[:lower:]')
host_arch=$(uname -m)
case "$host_arch" in x86_64) host_arch=amd64 ;; arm64|aarch64) host_arch=arm64 ;; *) printf 'unsupported build host architecture\n' >&2; exit 1 ;; esac
"$bundle_dir/assets/verifier/fairway-offline-verify_${host_os}_${host_arch}" \
  --dir "$bundle_dir" \
  --trusted-public-key-env FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY \
  --expected-version "$current_version" \
  --expected-source-sha "$current_sha" \
  --expected-builder-id "$current_builder" \
  --expected-policy-version "$current_policy" \
  --expected-rollback-version "$rollback_version" \
  --expected-rollback-source-sha "$rollback_sha" \
  --expected-rollback-builder-id "$rollback_builder" \
  --expected-rollback-policy-version "$rollback_policy"

archive="$output_root/fairway_${current_version#v}_offline_distribution.tar.gz"
helper_file=${FAIRWAY_REHEARSAL_HELPER_FILE:-scripts/release/internal/rehearsal_helper.go}
go run "$helper_file" archive-dir --dir "$bundle_dir" --root-name "$(basename "$bundle_dir")" --out "$archive"
shasum -a 256 "$archive" > "$archive.sha256"
printf 'offline distribution bundle: %s\narchive: %s\nchecksum: %s\n' "$bundle_dir" "$archive" "$archive.sha256"
