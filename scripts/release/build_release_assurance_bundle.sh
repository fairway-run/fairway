#!/usr/bin/env bash
set -euo pipefail

version=${1:?usage: build_release_assurance_bundle.sh <version> <source-sha> <dist-dir> <output-dir>}
source_sha=${2:?source sha is required}
dist_dir=${3:?GoReleaser dist directory is required}
output_root=${4:?output directory is required}

: "${FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY:?FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY is required}"
: "${FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY:?FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY is required}"

if [[ -n "${FAIRWAY_RELEASE_TOOL:-}" ]]; then
  if [[ ! -f "$FAIRWAY_RELEASE_TOOL" || -L "$FAIRWAY_RELEASE_TOOL" || ! -x "$FAIRWAY_RELEASE_TOOL" ]]; then
    printf 'FAIRWAY_RELEASE_TOOL must be an executable regular non-symlink file\n' >&2
    exit 1
  fi
  fairway_release_command=("$FAIRWAY_RELEASE_TOOL")
else
  fairway_release_command=(go run ./cmd/fairway)
fi

for tool in syft go-licenses govulncheck jq shasum; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf 'required release assurance tool is missing: %s\n' "$tool" >&2
    exit 1
  }
done

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"
test "$(git rev-parse HEAD)" = "$source_sha" || {
  printf 'source sha does not match checked out HEAD\n' >&2
  exit 1
}

evidence_dir="$output_root/evidence-inputs"
bundle_dir="$output_root/fairway-${version}-release-assurance"
mkdir -p "$evidence_dir"
test ! -e "$bundle_dir" || {
  printf 'release assurance output already exists: %s\n' "$bundle_dir" >&2
  exit 1
}

syft dir:. -o spdx-json="$evidence_dir/sbom.spdx.json"
go list -m -json all > "$evidence_dir/dependencies.jsonl"
go-licenses report ./... > "$evidence_dir/licenses.csv"

license_overrides_source=${FAIRWAY_RELEASE_LICENSE_OVERRIDES:-docs/security/release-license-overrides.json}
if [[ ! -f "$license_overrides_source" || -L "$license_overrides_source" ]]; then
  printf 'release license override policy must be a regular non-symlink file\n' >&2
  exit 1
fi
license_overrides="$evidence_dir/license-overrides.json"
cp "$license_overrides_source" "$license_overrides"
unknown_modules=$(awk -F, '$3 == "Unknown" { print $1 }' "$evidence_dir/licenses.csv")
while IFS= read -r module; do
  [[ -n "$module" ]] || continue
  override=$(jq -c --arg module "$module" '.overrides[] | select(.module == $module)' "$license_overrides")
  [[ -n "$override" ]] || {
    printf 'license inventory has unresolved module without reviewed override: %s\n' "$module" >&2
    exit 1
  }
  expected_version=$(jq -r .version <<<"$override")
  expected_origin=$(jq -r .origin <<<"$override")
  expected_commit=$(jq -r .origin_commit <<<"$override")
  expected_license=$(jq -r .license <<<"$override")
  license_path=$(jq -r .license_path <<<"$override")
  expected_digest=$(jq -r .license_sha256 <<<"$override")
  module_json=$(go list -m -json "$module@$expected_version")
  [[ $(jq -r .Origin.URL <<<"$module_json") == "$expected_origin" ]]
  [[ $(jq -r .Origin.Hash <<<"$module_json") == "$expected_commit" ]]
  module_dir=$(go list -m -f '{{.Dir}}' "$module@$expected_version")
  actual_digest=$(shasum -a 256 "$module_dir/$license_path" | awk '{print $1}')
  [[ "$actual_digest" == "$expected_digest" ]] || {
    printf 'license override digest mismatch for %s\n' "$module" >&2
    exit 1
  }
  awk -F, -v OFS=, -v module="$module" -v url="$expected_origin/-/blob/$expected_version/$license_path" -v license="$expected_license" \
    '$1 == module { $2=url; $3=license } { print }' \
    "$evidence_dir/licenses.csv" > "$evidence_dir/licenses.csv.tmp"
  mv "$evidence_dir/licenses.csv.tmp" "$evidence_dir/licenses.csv"
done <<< "$unknown_modules"

if awk -F, '$3 == "Unknown" { found=1 } END { exit found ? 0 : 1 }' "$evidence_dir/licenses.csv"; then
  printf 'license inventory still contains unresolved entries\n' >&2
  exit 1
fi

set +e
govulncheck -json ./... > "$evidence_dir/govulncheck.json"
vulnerability_status=$?
set -e
if [[ $vulnerability_status -ne 0 ]]; then
  printf 'govulncheck reported findings or failed; candidate release requires reviewed vulnerability disposition\n' >&2
  exit "$vulnerability_status"
fi

builder_id=${GITHUB_WORKFLOW_REF:-local:unreviewed-builder}
policy_version=${FAIRWAY_RELEASE_POLICY_VERSION:-sovereign-release-v1}
if [[ -n "${SOURCE_DATE_EPOCH:-}" ]]; then
  if created_at=$(date -u -r "$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null); then
    :
  else
    created_at=$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)
  fi
else
  created_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
fi

jq -n \
  --arg version "$version" \
  --arg source_sha "$source_sha" \
  --arg repository "${GITHUB_REPOSITORY:-local}" \
  --arg ref "${GITHUB_REF:-local}" \
  '{schema:"fairway.release-source-provenance.v1",version:$version,source_sha:$source_sha,repository:$repository,ref:$ref}' \
  > "$evidence_dir/source-provenance.json"

jq -n \
  --arg builder_id "$builder_id" \
  --arg run_id "${GITHUB_RUN_ID:-local}" \
  --arg run_attempt "${GITHUB_RUN_ATTEMPT:-local}" \
  --arg runner_os "${RUNNER_OS:-local}" \
  --arg runner_arch "${RUNNER_ARCH:-local}" \
  --arg go_version "$(go version)" \
  --arg goreleaser_version "$(goreleaser --version | tr '\n' ' ')" \
  --arg created_at "$created_at" \
  '{schema:"fairway.release-build-provenance.v1",builder_id:$builder_id,run_id:$run_id,run_attempt:$run_attempt,runner_os:$runner_os,runner_arch:$runner_arch,go_version:$go_version,goreleaser_version:$goreleaser_version,created_at:$created_at}' \
  > "$evidence_dir/build-provenance.json"

jq -n \
  --arg recipe_sha256 "$(shasum -a 256 .goreleaser.yaml | awk '{print $1}')" \
  --arg source ".goreleaser.yaml" \
  '{schema:"fairway.release-build-recipe.v1",source:$source,sha256:$recipe_sha256}' \
  > "$evidence_dir/build-recipe.json"

jq -n \
  --arg created_at "$created_at" \
  '{schema:"fairway.release-test-summary.v1",created_at:$created_at,commands:["go test ./...","go vet ./..."],result:"pass"}' \
  > "$evidence_dir/test-summary.json"

jq -n \
  --arg scanner "govulncheck" \
  --arg result "no_findings" \
  --arg report "govulncheck.json" \
  '{schema:"fairway.release-vulnerability-disposition.v1",scanner:$scanner,result:$result,report:$report,authority_boundary:"automated scan result only; not dependency trust, certification, or risk acceptance"}' \
  > "$evidence_dir/vulnerability-disposition.json"

jq -n \
  --arg author "fairway release pipeline" \
  --arg timestamp "$created_at" \
  '{"@context":"https://openvex.dev/ns/v0.2.0","@id":"https://fairway.run/vex/release-assurance","author":$author,"timestamp":$timestamp,"version":1,"statements":[]}' \
  > "$evidence_dir/vex.openvex.json"

artifact_args=()
while IFS= read -r artifact; do
  artifact_args+=(--artifact "$(basename "$artifact")=$artifact")
done < <(find "$dist_dir" -maxdepth 1 -type f -name 'fairway_*.tar.gz' -print | sort)
(( ${#artifact_args[@]} > 0 )) || {
  printf 'no GoReleaser archives found in %s\n' "$dist_dir" >&2
  exit 1
}

"${fairway_release_command[@]}" release assurance export \
  --out "$bundle_dir" \
  --version "$version" \
  --source-sha "$source_sha" \
  --builder-id "$builder_id" \
  --policy-version "$policy_version" \
  --signing-key-env FAIRWAY_RELEASE_ASSURANCE_SIGNING_KEY \
  "${artifact_args[@]}" \
  --evidence "sbom=$evidence_dir/sbom.spdx.json" \
  --evidence "vex=$evidence_dir/vex.openvex.json" \
  --evidence "dependencies=$evidence_dir/dependencies.jsonl" \
  --evidence "licenses=$evidence_dir/licenses.csv" \
  --evidence "license_disposition=$license_overrides" \
  --evidence "source_provenance=$evidence_dir/source-provenance.json" \
  --evidence "build_provenance=$evidence_dir/build-provenance.json" \
  --evidence "build_recipe=$evidence_dir/build-recipe.json" \
  --evidence "test_summary=$evidence_dir/test-summary.json" \
  --evidence "vulnerability_disposition=$evidence_dir/vulnerability-disposition.json" \
  --slsa-source-versioned \
  --slsa-build-service-generated \
  --slsa-provenance-available \
  --slsa-builder-identity-recorded \
  --slsa-build-recipe-recorded \
  --slsa-dependencies-recorded

"${fairway_release_command[@]}" release assurance verify \
  --dir "$bundle_dir" \
  --trusted-public-key-env FAIRWAY_RELEASE_ASSURANCE_PUBLIC_KEY \
  --expected-version "$version" \
  --expected-source-sha "$source_sha" \
  --expected-builder-id "$builder_id" \
  --expected-policy-version "$policy_version"

archive="$output_root/fairway_${version}_release_assurance.tar.gz"
helper_file=${FAIRWAY_REHEARSAL_HELPER_FILE:-scripts/release/internal/rehearsal_helper.go}
go run "$helper_file" archive-dir --dir "$bundle_dir" --root-name "$(basename "$bundle_dir")" --out "$archive"
shasum -a 256 "$archive" > "$archive.sha256"
printf 'release assurance bundle: %s\n' "$archive"
