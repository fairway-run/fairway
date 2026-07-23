#!/usr/bin/env bash
set -euo pipefail

tool_cache_root=${1:?usage: resolve_goreleaser_action_tool.sh <runner-tool-cache> <version>}
version=${2:?GoReleaser version is required}

[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
  printf 'invalid GoReleaser version: %s\n' "$version" >&2
  exit 1
}

version_root="$tool_cache_root/goreleaser-action/$version"
if [[ ! -d "$version_root" || -L "$version_root" ]]; then
  printf 'GoReleaser action toolcache root is unavailable: %s\n' "$version_root" >&2
  exit 1
fi

shopt -s nullglob
candidates=("$version_root"/*/goreleaser)
shopt -u nullglob

(( ${#candidates[@]} == 1 )) || {
  printf 'expected one pinned GoReleaser tool, found %d\n' "${#candidates[@]}" >&2
  exit 1
}

tool=${candidates[0]}
if [[ ! -f "$tool" || -L "$tool" || ! -x "$tool" ]]; then
  printf 'pinned GoReleaser tool must be an executable regular non-symlink file\n' >&2
  exit 1
fi

printf '%s\n' "$tool"
