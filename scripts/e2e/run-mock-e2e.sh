#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BIN_DIR="$ROOT_DIR/bin"
MOCK_PORT=${MOCK_PORT:-18086}
OUT_DIR="$ROOT_DIR/scripts/e2e/output"
GOLDEN="$ROOT_DIR/scripts/e2e/golden/pvetui-mock.txt"
TAPE="$ROOT_DIR/scripts/e2e/pvetui-mock.tape"
SPEC="$ROOT_DIR/docs/api/pve-openapi.yaml"
UPDATE=${UPDATE:-0}

command -v vhs >/dev/null 2>&1 || { echo "vhs not found; install with 'go install github.com/charmbracelet/vhs@latest'" >&2; exit 1; }

mkdir -p "$BIN_DIR" "$OUT_DIR" "$(dirname "$GOLDEN")"

# Build pvetui if missing
if [ ! -x "$BIN_DIR/pvetui" ]; then
  echo "Building pvetui binary..."
  (cd "$ROOT_DIR" && go build -o "$BIN_DIR/pvetui" ./cmd/pvetui)
fi

# Start mock API
echo "Starting mock API on port $MOCK_PORT..."
MOCK_LOG="$OUT_DIR/mock.log"
go run "$ROOT_DIR/cmd/pve-mock-api" -spec "$SPEC" -port "$MOCK_PORT" >"$MOCK_LOG" 2>&1 &
MOCK_PID=$!
trap 'kill $MOCK_PID 2>/dev/null || true' EXIT

# Wait for mock to come up
for i in {1..50}; do
  if curl -sf "http://127.0.0.1:$MOCK_PORT/access/ticket" >/dev/null; then
    break
  fi
  sleep 0.1
done

# Write temp config pointing at mock
CFG=$(mktemp)
cat >"$CFG" <<EOF
active_profile: mock
profiles:
  - name: mock
    addr: http://127.0.0.1:$MOCK_PORT
    user: testuser@pam
    password: testpass
    insecure: true
EOF

export PVETUI_CONFIG="$CFG"

echo "Running VHS tape..."
vhs "$TAPE"

OUTPUT="$OUT_DIR/pvetui-mock.txt"

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
