#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
MOCK_PORT=${MOCK_PORT:-18086}
PVETUI_MEDIA_PROFILE=${PVETUI_MEDIA_PROFILE:-mock}
OUT_DIR="$ROOT_DIR/scripts/e2e/output"
GOLDEN="$ROOT_DIR/scripts/e2e/golden/pvetui-mock.txt"
TAPE="$ROOT_DIR/scripts/e2e/pvetui-mock.tape"
UPDATE=${UPDATE:-0}

command -v vhs >/dev/null 2>&1 || { echo "vhs not found; install with 'go install github.com/charmbracelet/vhs@latest'" >&2; exit 1; }

mkdir -p "$OUT_DIR" "$(dirname "$GOLDEN")"

echo "Running VHS tape..."
MOCK_PORT="$MOCK_PORT" PVETUI_MEDIA_PROFILE="$PVETUI_MEDIA_PROFILE" "$ROOT_DIR/scripts/media/run-vhs.sh" "$TAPE"

RAW_OUTPUT="$OUT_DIR/pvetui-mock.raw.txt"
OUTPUT="$OUT_DIR/pvetui-mock.txt"

if [ -f "$RAW_OUTPUT" ]; then
  sed -i -E \
    -e 's#/tmp/tmp\.[^/]+/config\.yml#<mock-config>#g' \
    -e 's#http://127\.0\.0\.1:[0-9]+#http://127.0.0.1:<mock-port>#g' \
    "$RAW_OUTPUT"
fi

if ! grep -q "Cluster Status" "$RAW_OUTPUT"; then
  echo "VHS output did not contain the cluster status panel" >&2
  exit 1
fi

if ! grep -q "pve" "$RAW_OUTPUT"; then
  echo "VHS output did not contain the mock node" >&2
  exit 1
fi

{
  echo "pvetui mock VHS smoke test"
  echo "cluster_status=present"
  echo "mock_node=pve"
  echo "profile=$PVETUI_MEDIA_PROFILE"
} >"$OUTPUT"

if [ "$UPDATE" -eq 1 ]; then
  echo "Updating golden at $GOLDEN"
  cp "$OUTPUT" "$GOLDEN"
else
  if ! diff -u "$GOLDEN" "$OUTPUT"; then
    echo "Golden mismatch. To update, rerun with UPDATE=1" >&2
    exit 1
  fi
fi

echo "E2E mock run completed successfully"
