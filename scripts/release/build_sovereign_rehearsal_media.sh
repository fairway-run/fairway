#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
usage: build_sovereign_rehearsal_media.sh \
  --current-version <version> --rollback-ref <git-ref> --rollback-version <version> \
  --output-root <new-dir> --builder-id <id> --policy-version <id> --created-at <RFC3339>

Builds current and rollback archives, nested release assurance, signed offline
media, and a separate immutable trust-bootstrap packet. It never publishes,
tags, installs, deploys, or changes public exposure.
EOF
}

current_version=''
rollback_ref=''
rollback_version=''
output_root=''
builder_id=''
policy_version=''
created_at=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --current-version) current_version=${2:-}; shift 2 ;;
    --rollback-ref) rollback_ref=${2:-}; shift 2 ;;
    --rollback-version) rollback_version=${2:-}; shift 2 ;;
    --output-root) output_root=${2:-}; shift 2 ;;
    --builder-id) builder_id=${2:-}; shift 2 ;;
    --policy-version) policy_version=${2:-}; shift 2 ;;
    --created-at) created_at=${2:-}; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
done

for value in current_version rollback_ref rollback_version output_root builder_id policy_version created_at; do
  [[ -n "${!value}" ]] || { printf '%s is required\n' "$value" >&2; exit 2; }
done
for value in current_version rollback_version builder_id policy_version; do
  [[ "${!value}" =~ ^[A-Za-z0-9][A-Za-z0-9._:+-]{0,127}$ ]] || {
    printf '%s contains unsupported characters\n' "$value" >&2
    exit 2
  }
done
[[ "$created_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || {
  printf 'created_at must be UTC RFC3339 seconds\n' >&2
  exit 2
}
[[ "$output_root" = /* ]] || { printf 'output_root must be absolute\n' >&2; exit 2; }
[[ ! -e "$output_root" ]] || { printf 'output_root already exists\n' >&2; exit 1; }
output_parent=$(dirname "$output_root")
test -d "$output_parent"
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
helper_file="$script_dir/internal/rehearsal_helper.go"

phase='initialize-output-staging'
build_root=''
log_redirected=false
repo_root=''
work=''
current_tree=''
rollback_tree=''
private_key=''
public_key=''
restore_log_output() {
  if [[ "$log_redirected" == true ]]; then
    exec 1>&3 2>&4
    log_redirected=false
  fi
}
cleanup() {
  status=$?
  set +e
  restore_log_output
  unset FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY FAIRWAY_RELEASE_LICENSE_OVERRIDES FAIRWAY_RELEASE_TOOL
  [[ -n "$private_key" ]] && rm -f "$private_key"
  [[ -n "$repo_root" && -d "$current_tree" ]] && git -C "$repo_root" worktree remove --force "$current_tree" >/dev/null 2>&1
  [[ -n "$repo_root" && -d "$rollback_tree" ]] && git -C "$repo_root" worktree remove --force "$rollback_tree" >/dev/null 2>&1
  [[ -n "$work" ]] && rm -rf "$work"
  if [[ $status -ne 0 ]]; then
    if ! go run "$helper_file" failure-packet \
      --output "$output_root" --staging "$build_root" --phase "$phase" --exit-code "$status"; then
      # Keep the fallback bounded even when Go itself is the failed prerequisite.
      [[ -n "$build_root" ]] && rm -rf "$build_root"
      if [[ ! -e "$output_root" ]]; then
        mkdir -p "$output_root/diagnostics"
        printf '{"schema":"fairway.sovereign-rehearsal-build-failure.v1","phase":"%s","exit_code":%d,"private_signing_material":"not_retained","authority_boundary":"build diagnostic only; no release, publish, install, deploy, credential, public-exposure, or live authority"}\n' "$phase" "$status" > "$output_root/diagnostics/failure.json"
      fi
    fi
  elif [[ -n "$build_root" ]]; then
    rm -rf "$build_root"
  fi
  exit "$status"
}
trap cleanup EXIT

build_root=$(mktemp -d "$output_parent/.fairway-sovereign-rehearsal-staging.XXXXXX")
mkdir -p "$build_root/diagnostics" "$build_root/media" "$build_root/trust-bootstrap"
log_file="$build_root/diagnostics/build.log"
exec 3>&1 4>&2
exec >"$log_file" 2>&1
log_redirected=true
phase='tool-preflight'

for tool in go git goreleaser syft go-licenses govulncheck jq shasum grep find date; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf 'required sovereign rehearsal build tool is missing: %s\n' "$tool" >&2
    exit 1
  }
done

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"
assurance_builder="$repo_root/scripts/release/build_release_assurance_bundle.sh"
offline_builder="$repo_root/scripts/release/build_offline_distribution_bundle.sh"
current_sha=$(git rev-parse --verify 'HEAD^{commit}')
rollback_sha=$(git rev-parse --verify "${rollback_ref}^{commit}")
[[ "$current_sha" != "$rollback_sha" ]] || { printf 'rollback ref resolves to current source\n' >&2; exit 1; }

phase=initialize
work=$(mktemp -d "${TMPDIR:-/tmp}/fairway-sovereign-rehearsal.XXXXXX")
current_tree="$work/current"
rollback_tree="$work/rollback"
private_key="$work/release-private.b64"
public_key="$work/release-public.b64"

phase=resolve-source-worktrees
git worktree add --detach "$current_tree" "$current_sha"
git worktree add --detach "$rollback_tree" "$rollback_sha"

to_epoch() {
  local value=$1
  if date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$value" +%s >/dev/null 2>&1; then
    date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$value" +%s
  else
    date -u -d "$value" +%s
  fi
}
export SOURCE_DATE_EPOCH
SOURCE_DATE_EPOCH=$(to_epoch "$created_at")
export GITHUB_WORKFLOW_REF="$builder_id"
export FAIRWAY_RELEASE_POLICY_VERSION="$policy_version"
export FAIRWAY_REHEARSAL_HELPER_FILE="$helper_file"
export FAIRWAY_RELEASE_LICENSE_OVERRIDES="$current_tree/docs/security/release-license-overrides.json"
if [[ ! -f "$FAIRWAY_RELEASE_LICENSE_OVERRIDES" || -L "$FAIRWAY_RELEASE_LICENSE_OVERRIDES" ]]; then
  printf 'current reviewed release license override policy is unavailable\n' >&2
  exit 1
fi

phase=build-current-release-tool
export FAIRWAY_RELEASE_TOOL="$work/fairway-release-tool"
(cd "$current_tree" && go build -buildvcs=false -trimpath -o "$FAIRWAY_RELEASE_TOOL" ./cmd/fairway)

phase=generate-ephemeral-signing-key
go run "$helper_file" keygen --private "$private_key" --public "$public_key"
test "$(stat -f '%Lp' "$private_key" 2>/dev/null || stat -c '%a' "$private_key")" = 600
export FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY
export FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY
FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY=$(tr -d '\r\n' < "$private_key")
FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY=$(tr -d '\r\n' < "$public_key")
public_fingerprint=$(go run "$helper_file" fingerprint --public "$public_key")

build_archives() {
  local source_dir=$1 version=$2 dist_dir=$3
  mkdir -p "$dist_dir"
  for target in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
    local goos=${target%_*} goarch=${target#*_}
    local build_dir="$work/build-${version}-${target}"
    mkdir -p "$build_dir"
    (
      cd "$source_dir"
      CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
        go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$version" \
        -o "$build_dir/fairway" ./cmd/fairway
    )
    go run "$helper_file" archive-file \
      --input "$build_dir/fairway" --name fairway \
      --out "$dist_dir/fairway_${version}_${target}.tar.gz"
  done
}

phase=build-current-archives
current_dist="$work/current-dist"
build_archives "$current_tree" "$current_version" "$current_dist"
phase=build-rollback-archives
rollback_dist="$work/rollback-dist"
build_archives "$rollback_tree" "$rollback_version" "$rollback_dist"

phase=build-current-assurance
current_assurance_root="$work/current-assurance"
mkdir -p "$current_assurance_root"
(cd "$current_tree" && "$assurance_builder" "$current_version" "$current_sha" "$current_dist" "$current_assurance_root")
current_assurance="$current_assurance_root/fairway-${current_version}-release-assurance"
phase=build-rollback-assurance
rollback_assurance_root="$work/rollback-assurance"
mkdir -p "$rollback_assurance_root"
(cd "$rollback_tree" && "$assurance_builder" "$rollback_version" "$rollback_sha" "$rollback_dist" "$rollback_assurance_root")
rollback_assurance="$rollback_assurance_root/fairway-${rollback_version}-release-assurance"

phase=build-offline-media
(cd "$current_tree" && "$offline_builder" "$current_assurance" "$rollback_assurance" "$build_root/media")
bundle_dir="$build_root/media/fairway_${current_version#v}_offline_distribution"
archive="$build_root/media/fairway_${current_version#v}_offline_distribution.tar.gz"
manifest="$bundle_dir/manifest.json"
verifier_rel=assets/verifier/fairway-offline-verify_linux_arm64
verifier="$bundle_dir/$verifier_rel"
test -f "$archive" -a -f "$manifest" -a -f "$verifier"

phase=destroy-private-signing-material
rm -f "$private_key"
unset FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY
test ! -e "$private_key"

phase=write-immutable-trust-bootstrap
cp "$public_key" "$build_root/trust-bootstrap/release-assurance-public-key.b64"
chmod 0644 "$build_root/trust-bootstrap/release-assurance-public-key.b64"
archive_sha=$(shasum -a 256 "$archive" | awk '{print $1}')
manifest_sha=$(shasum -a 256 "$manifest" | awk '{print $1}')
verifier_sha=$(shasum -a 256 "$verifier" | awk '{print $1}')
helper_sha=$(shasum -a 256 "$helper_file" | awk '{print $1}')
builder_script_sha=$(shasum -a 256 "$repo_root/scripts/release/build_sovereign_rehearsal_media.sh" | awk '{print $1}')
assurance_builder_sha=$(shasum -a 256 "$assurance_builder" | awk '{print $1}')
offline_builder_sha=$(shasum -a 256 "$offline_builder" | awk '{print $1}')
license_policy_sha=$(shasum -a 256 "$FAIRWAY_RELEASE_LICENSE_OVERRIDES" | awk '{print $1}')
release_tool_sha=$(shasum -a 256 "$FAIRWAY_RELEASE_TOOL" | awk '{print $1}')
jq -n \
  --arg created_at "$created_at" \
  --arg public_fingerprint "$public_fingerprint" \
  --arg archive "$(basename "$archive")" --arg archive_sha "$archive_sha" \
  --arg manifest_sha "$manifest_sha" --arg verifier_rel "$verifier_rel" --arg verifier_sha "$verifier_sha" \
  --arg current_version "$current_version" --arg current_sha "$current_sha" \
  --arg rollback_version "$rollback_version" --arg rollback_sha "$rollback_sha" \
  --arg builder_id "$builder_id" --arg policy_version "$policy_version" \
  --arg helper_sha "$helper_sha" --arg builder_script_sha "$builder_script_sha" \
  --arg assurance_builder_sha "$assurance_builder_sha" --arg offline_builder_sha "$offline_builder_sha" \
  --arg license_policy_sha "$license_policy_sha" --arg release_tool_sha "$release_tool_sha" \
  '{schema:"fairway.sovereign-rehearsal-trust-bootstrap.v2",created_at:$created_at,
    public_key:{encoding:"base64-ed25519-public-key",file:"release-assurance-public-key.b64",fingerprint:$public_fingerprint},
    media:{path_class:"separate_sibling_directory",archive:$archive,sha256:$archive_sha,manifest_sha256:$manifest_sha},
    linux_arm64_verifier:{path:$verifier_rel,sha256:$verifier_sha},
    release:{current:{version:$current_version,source_sha:$current_sha},rollback:{version:$rollback_version,source_sha:$rollback_sha},builder_id:$builder_id,policy_version:$policy_version},
    build_inputs:{helper_sha256:$helper_sha,builder_script_sha256:$builder_script_sha,
      assurance_builder_sha256:$assurance_builder_sha,offline_builder_sha256:$offline_builder_sha,
      release_license_override_policy_sha256:$license_policy_sha,current_release_tool_sha256:$release_tool_sha},
    private_signing_material:"destroyed_after_packet_generation",
    authority_boundary:"rehearsal trust facts only; not certification, compliance, customer installation authorization, release, publication, deployment, credential authority, public exposure, or live-operation authority"}' \
  > "$build_root/trust-bootstrap/trust-bootstrap.json"

phase=write-readback
jq -n \
  --arg created_at "$created_at" --arg current_sha "$current_sha" --arg rollback_sha "$rollback_sha" \
  --arg archive_sha "$archive_sha" --arg public_fingerprint "$public_fingerprint" --arg verifier_sha "$verifier_sha" \
  --arg license_policy_sha "$license_policy_sha" \
  '{schema:"fairway.sovereign-rehearsal-build-readback.v1",result:"pass",created_at:$created_at,
    current_source_sha:$current_sha,rollback_source_sha:$rollback_sha,archive_sha256:$archive_sha,
    public_key_fingerprint:$public_fingerprint,linux_arm64_verifier_sha256:$verifier_sha,
    release_license_override_policy_sha256:$license_policy_sha,
    nested_release_assurance:"verified",offline_distribution:"verified",private_signing_material:"not_retained",public_release:false}' \
  > "$build_root/readback.json"

phase=finalize-retained-log
printf 'verified staged media archive sha256: %s\n' "$archive_sha"
restore_log_output

phase=promote-verified-output
go run "$helper_file" scan-promote --staging "$build_root" --output "$output_root"
build_root=''
phase=complete
printf 'sovereign rehearsal media: %s\ntrust bootstrap: %s\narchive sha256: %s\n' \
  "$output_root/media" "$output_root/trust-bootstrap" "$archive_sha"
