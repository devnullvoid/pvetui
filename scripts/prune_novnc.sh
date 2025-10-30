#!/usr/bin/env bash
# * Prunes non-runtime files from internal/vnc/novnc after a git subtree update.
set -euo pipefail

NOVNC_DIR="$(dirname "$0")/../internal/vnc/novnc"

rm -rf \
  "$NOVNC_DIR/tests" \
  "$NOVNC_DIR/docs" \
  "$NOVNC_DIR/snap" \
  "$NOVNC_DIR/eslint.config.mjs" \
  "$NOVNC_DIR/karma.conf.cjs" \
  "$NOVNC_DIR/po" \
  "$NOVNC_DIR/utils" \
  "$NOVNC_DIR/README.md" \
  "$NOVNC_DIR/package.json" \
  "$NOVNC_DIR/AUTHORS"

echo "[*] Pruned unnecessary files from $NOVNC_DIR."
