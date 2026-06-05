#!/usr/bin/env bash
set -euo pipefail

binary_path="${1:?binary path required}"
goos="${2:?goos required}"
goarch="${3:?goarch required}"

if [[ "$goos" != "darwin" ]]; then
  exit 0
fi

: "${MACOS_CODESIGN_IDENTITY:?MACOS_CODESIGN_IDENTITY is required for darwin signing}"
: "${MACOS_NOTARY_KEY:?MACOS_NOTARY_KEY is required for darwin notarization}"
: "${MACOS_NOTARY_KEY_ID:?MACOS_NOTARY_KEY_ID is required for darwin notarization}"
: "${MACOS_NOTARY_ISSUER_ID:?MACOS_NOTARY_ISSUER_ID is required for darwin notarization}"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

key_file="$tmpdir/AuthKey_${MACOS_NOTARY_KEY_ID}.p8"
archive="$tmpdir/fairway-${goarch}.zip"

printf '%s' "$MACOS_NOTARY_KEY" | base64 -D > "$key_file"

codesign_args=(
  --force
  --timestamp
  --options runtime
  --sign "$MACOS_CODESIGN_IDENTITY"
)

if [[ -n "${MACOS_CODESIGN_KEYCHAIN:-}" ]]; then
  codesign_args+=(--keychain "$MACOS_CODESIGN_KEYCHAIN")
fi

codesign "${codesign_args[@]}" "$binary_path"

codesign --verify --strict --verbose=4 "$binary_path"

ditto -c -k --keepParent "$binary_path" "$archive"

xcrun notarytool submit "$archive" \
  --key "$key_file" \
  --key-id "$MACOS_NOTARY_KEY_ID" \
  --issuer "$MACOS_NOTARY_ISSUER_ID" \
  --wait
