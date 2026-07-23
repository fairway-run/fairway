#!/usr/bin/env bash
set -euo pipefail

repo=${1:?usage: download_release_rehearsal.sh <repo> <version> <source-sha> <run-id> <output-dir>}
version=${2:?version is required}
source_sha=${3:?source sha is required}
run_id=${4:?rehearsal run id is required}
output_dir=${5:?output directory is required}

: "${GH_TOKEN:?GH_TOKEN is required}"

[[ "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || {
  printf 'repository must use owner/name form\n' >&2
  exit 1
}
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
  printf 'version must match vX.Y.Z\n' >&2
  exit 1
}
[[ "$source_sha" =~ ^[0-9a-f]{40}$ ]] || {
  printf 'source sha must be a lowercase 40-character commit\n' >&2
  exit 1
}
[[ "$run_id" =~ ^[1-9][0-9]*$ ]] || {
  printf 'rehearsal run id must be a positive integer\n' >&2
  exit 1
}
[[ ! -e "$output_dir" ]] || {
  printf 'rehearsal download output already exists: %s\n' "$output_dir" >&2
  exit 1
}

output_parent=$(dirname "$output_dir")
mkdir -p "$output_parent"
staging_dir=$(mktemp -d "$output_parent/.fairway-release-rehearsal.XXXXXX")
chmod 0700 "$staging_dir"
cleanup() {
  rm -rf "$staging_dir"
}
trap cleanup EXIT HUP INT TERM

run_json=$(gh api "repos/$repo/actions/runs/$run_id")
jq -e \
  --arg sha "$source_sha" \
  '.status == "completed"
   and .conclusion == "success"
   and .event == "workflow_dispatch"
   and .head_sha == $sha
   and .head_branch == "main"
   and .path == ".github/workflows/release-rehearsal.yml"' \
  <<<"$run_json" >/dev/null || {
    printf 'rehearsal run does not match the required successful main-branch workflow identity\n' >&2
    exit 1
  }

artifact_name="fairway-release-rehearsal-${version}-${source_sha}"
artifacts_json=$(gh api "repos/$repo/actions/runs/$run_id/artifacts?per_page=100")
artifact_count=$(jq --arg name "$artifact_name" '[.artifacts[] | select(.name == $name and .expired == false)] | length' <<<"$artifacts_json")
[[ "$artifact_count" == "1" ]] || {
  printf 'expected exactly one unexpired rehearsal artifact named %s, found %s\n' "$artifact_name" "$artifact_count" >&2
  exit 1
}

gh run download "$run_id" --repo "$repo" --name "$artifact_name" --dir "$staging_dir"
[[ -f "$staging_dir/rehearsal.json" && ! -L "$staging_dir/rehearsal.json" ]] || {
  printf 'downloaded rehearsal artifact is missing its regular manifest\n' >&2
  exit 1
}
mv "$staging_dir" "$output_dir"
trap - EXIT HUP INT TERM
printf '%s\n' "$output_dir"
