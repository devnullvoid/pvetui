#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BIN_DIR="$ROOT_DIR/bin"
OUT_DIR="$ROOT_DIR/scripts/media/output"
SPEC="$ROOT_DIR/docs/api/pve-openapi.yaml"
FIXTURE="$ROOT_DIR/scripts/media/mock-fixture.yml"
MOCK_PORT=${MOCK_PORT:-18086}
PVETUI_MEDIA_PROFILE=${PVETUI_MEDIA_PROFILE:-mock}

if [ "$#" -eq 0 ]; then
  set -- "$ROOT_DIR/docs/screenshots.tape"
fi

command -v vhs >/dev/null 2>&1 || {
  echo "vhs not found; install with 'go install github.com/charmbracelet/vhs@latest'" >&2
  exit 1
}

mkdir -p "$BIN_DIR" "$OUT_DIR"

echo "Building pvetui binary..."
(cd "$ROOT_DIR" && go build -o "$BIN_DIR/pvetui" ./cmd/pvetui)

echo "Building mock API binary..."
(cd "$ROOT_DIR" && go build -o "$BIN_DIR/pve-mock-api" ./cmd/pve-mock-api)

WORK_DIR=$(mktemp -d)
CFG="$WORK_DIR/config.yml"
CACHE_DIR="$WORK_DIR/cache"
MOCK_LOG="$OUT_DIR/mock.log"
mkdir -p "$CACHE_DIR"

cleanup() {
  if [ -n "${MOCK_PID:-}" ]; then
    kill "$MOCK_PID" 2>/dev/null || true
    wait "$MOCK_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

echo "Starting mock API on port $MOCK_PORT..."
"$BIN_DIR/pve-mock-api" -spec "$SPEC" -fixture "$FIXTURE" -port "$MOCK_PORT" >"$MOCK_LOG" 2>&1 &
MOCK_PID=$!

ready=0
for _ in {1..80}; do
  if ! kill -0 "$MOCK_PID" 2>/dev/null; then
    echo "mock API exited before becoming ready; see $MOCK_LOG" >&2
    exit 1
  fi
  if curl -sf "http://127.0.0.1:$MOCK_PORT/api2/json/version" >/dev/null; then
    ready=1
    break
  fi
  sleep 0.1
done

if [ "$ready" -ne 1 ]; then
  echo "mock API did not become ready; see $MOCK_LOG" >&2
  exit 1
fi

cat >"$CFG" <<EOF
profiles:
  $PVETUI_MEDIA_PROFILE:
    addr: http://127.0.0.1:$MOCK_PORT
    api_path: /api2/json
    user: root
    password: mockpass
    realm: pam
    insecure: true
    ssh_user: root
    vm_ssh_user: demo
default_profile: $PVETUI_MEDIA_PROFILE
debug: false
show_icons: true
cache_dir: "$CACHE_DIR"
theme:
  name: "default"
  colors: {}
plugins:
  enabled:
    - "guest-insights"
EOF

export PVETUI_MEDIA_CONFIG="$CFG"
export PVETUI_MEDIA_CACHE_DIR="$CACHE_DIR"
export PVETUI_MEDIA_PROFILE

for tape in "$@"; do
  echo "Running VHS tape: $tape"
  (cd "$ROOT_DIR" && vhs "$tape")
done
