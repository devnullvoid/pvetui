#!/usr/bin/env bash
set -euo pipefail

check_only=false
case "${1:-}" in
  --check) check_only=true ;;
  '') ;;
  *) printf 'Usage: bash scripts/update-nix-hash.sh [--check]\n' >&2; exit 2 ;;
esac
if (( $# > 1 )); then
  printf 'Usage: bash scripts/update-nix-hash.sh [--check]\n' >&2
  exit 2
fi

tmp=$(mktemp -d)
restore=false
cleanup() {
  if [ "$restore" = true ]; then
    cp "$tmp/flake.nix" flake.nix
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

build() {
  nix build .#default --no-link --print-out-paths > "$tmp/output" 2> "$tmp/log"
}

updated=false
if ! build; then
  cat "$tmp/log" >&2
  # Only pvetui's Go dependency hash may be repaired, never a flake input hash.
  new_hash=$(awk '
    /hash mismatch in fixed-output derivation .*pvetui-.*-go-modules/ { dependency = 1; next }
    dependency && /^[[:space:]]*got:[[:space:]]*sha256-[A-Za-z0-9+\/=]+[[:space:]]*$/ {
      sub(/^[[:space:]]*got:[[:space:]]*/, "")
      sub(/[[:space:]]*$/, "")
      print
      exit
    }
  ' "$tmp/log")
  if [ -z "$new_hash" ]; then
    printf 'Nix build failed without a repairable pvetui dependency hash mismatch.\n' >&2
    exit 1
  fi
  if [ "$check_only" = true ]; then
    printf 'Stale vendorHash; run bash scripts/update-nix-hash.sh and commit flake.nix.\n' >&2
    exit 1
  fi
  if [ "$(grep -Ec '^[[:space:]]*vendorHash = "[^"]*";[[:space:]]*$' flake.nix)" != 1 ]; then
    printf 'Expected exactly one vendorHash assignment in flake.nix.\n' >&2
    exit 1
  fi
  cp flake.nix "$tmp/flake.nix"
  restore=true
  sed -E "s|^([[:space:]]*vendorHash = \")[^\"]*(\";[[:space:]]*)$|\1${new_hash}\2|" flake.nix > "$tmp/updated.nix"
  cp "$tmp/updated.nix" flake.nix
  if cmp -s "$tmp/flake.nix" flake.nix; then
    printf 'Dependency hash did not change; refusing to claim success.\n' >&2
    exit 1
  fi
  printf 'Verifying updated vendorHash: %s\n' "$new_hash"
  if ! build; then
    cat "$tmp/log" >&2
    exit 1
  fi
  updated=true
fi

cat "$tmp/log" >&2
output=$(<"$tmp/output")
"$output/bin/pvetui" --version
if [ -n "${GITHUB_OUTPUT:-}" ]; then
  printf 'updated=%s\n' "$updated" >> "$GITHUB_OUTPUT"
fi
restore=false
printf 'Nix build and binary smoke test passed.\n'
