#!/usr/bin/env bash
set -euo pipefail

bundle=${1:?usage: offline_distribution_rehearsal.sh <bundle-dir> <artifact-dir> <current-version> <rollback-version>}
artifacts=${2:?artifact directory is required}
current_version=${3:?current version is required}
rollback_version=${4:?rollback version is required}
: "${FAIRWAY_OFFLINE_TRUSTED_PUBLIC_KEY:?FAIRWAY_OFFLINE_TRUSTED_PUBLIC_KEY is required}"

for tool in git shasum; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf 'required offline rehearsal tool is missing: %s\n' "$tool" >&2
    exit 1
  }
done

bundle=$(cd "$bundle" && pwd -P)
mkdir -p "$artifacts"
artifacts=$(cd "$artifacts" && pwd -P)
work=$(mktemp -d "${TMPDIR:-/tmp}/fairway-offline-rehearsal.XXXXXX")
cleanup() {
  chmod -R u+w "$work" 2>/dev/null || true
  rm -rf "$work"
  if [[ ! -e "$work" ]]; then
    printf 'cleanup=pass\nwork_path=%s\n' "$work" > "$artifacts/cleanup.txt"
  else
    printf 'cleanup=fail\nwork_path=%s\n' "$work" > "$artifacts/cleanup.txt"
  fi
}
trap cleanup EXIT

media="$work/removable-media/fairway-bundle"
prefix="$work/install-root"
workspace="$work/workspace"
mkdir -p "$(dirname "$media")" "$workspace/.fairway"
cp -R "$bundle" "$media"
chmod -R a-w "$media"

"$media/scripts/verify.sh" > "$artifacts/verify.txt"
"$media/scripts/rollback.sh" "$prefix" > "$artifacts/install-rollback-initial.txt"
binary="$prefix/bin/fairway"
rollback_readback=$($binary version)
test "$rollback_readback" = "${rollback_version#v}"

cp "$media/assets/configuration/fairway-config.toml" "$workspace/.fairway/config.toml"
git -C "$workspace" init -q -b main
git -C "$workspace" config user.email offline-rehearsal@example.invalid
git -C "$workspace" config user.name 'Offline Rehearsal'
(
  cd "$workspace"
  "$binary" --config "$workspace/.fairway/config.toml" add OFFLINE-001 --title 'Offline lifecycle compatibility proof' --role ops
  "$binary" --config "$workspace/.fairway/config.toml" task-detail OFFLINE-001 > "$artifacts/task-before-upgrade.txt"
  mkdir -p "$artifacts/backups"
  "$binary" --config "$workspace/.fairway/config.toml" db backup "$artifacts/backups/pre-upgrade.db" > "$artifacts/backup-pre-upgrade.txt"
)

"$media/scripts/install.sh" "$prefix" > "$artifacts/install-current.txt"
current_readback=$($binary version)
test "$current_readback" = "${current_version#v}"
(
  cd "$workspace"
  "$binary" --config "$workspace/.fairway/config.toml" config validate > "$artifacts/config-current.txt"
  "$binary" --config "$workspace/.fairway/config.toml" task-detail OFFLINE-001 > "$artifacts/task-current.txt"
  "$binary" --config "$workspace/.fairway/config.toml" db backup "$artifacts/backups/post-upgrade.db" > "$artifacts/backup-post-upgrade.txt"
)

"$media/scripts/rollback.sh" "$prefix" > "$artifacts/rollback-final.txt"
rollback_final=$($binary version)
test "$rollback_final" = "${rollback_version#v}"
(
  cd "$workspace"
  "$binary" --config "$workspace/.fairway/config.toml" config validate > "$artifacts/config-rollback.txt"
  "$binary" --config "$workspace/.fairway/config.toml" task-detail OFFLINE-001 > "$artifacts/task-after-rollback.txt"
  "$binary" --config "$workspace/.fairway/config.toml" db backup "$artifacts/backups/post-rollback.db" > "$artifacts/backup-post-rollback.txt"
)

shasum -a 256 "$binary" "$workspace/.fairway/config.toml" "$workspace/.fairway/state.db" "$artifacts"/backups/*.db > "$artifacts/digests.txt"
cat > "$artifacts/readback.txt" <<EOF
result=pass
media_path=$media
binary_path=$binary
config_path=$workspace/.fairway/config.toml
data_path=$workspace/.fairway/state.db
current_version=$current_readback
rollback_version=$rollback_final
pre_upgrade_backup=$artifacts/backups/pre-upgrade.db
post_upgrade_backup=$artifacts/backups/post-upgrade.db
post_rollback_backup=$artifacts/backups/post-rollback.db
authority_boundary=disconnected compatibility rehearsal only; not installation authorization, deployment approval, certification, compliance, or production readiness
EOF
