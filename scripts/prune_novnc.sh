#!/usr/bin/env bash
# * Prunes non-runtime files from internal/vnc/novnc after a git subtree update.
set -euo pipefail

NOVNC_DIR="$(dirname "$0")/../internal/vnc/novnc"

rm -rf \
  "$NOVNC_DIR/.github" \
  "$NOVNC_DIR/.gitignore" \
  "$NOVNC_DIR/.gitmodules" \
  "$NOVNC_DIR/tests" \
  "$NOVNC_DIR/docs" \
  "$NOVNC_DIR/snap" \
  "$NOVNC_DIR/eslint.config.mjs" \
  "$NOVNC_DIR/karma.conf.cjs" \
  "$NOVNC_DIR/po" \
  "$NOVNC_DIR/utils" \
  "$NOVNC_DIR/AUTHORS"
# "$NOVNC_DIR/README.md" \
# "$NOVNC_DIR/package.json" \

echo "[*] Pruned unnecessary files from $NOVNC_DIR."

# Fix for go:embed - Go's embed package excludes directories named 'vendor' by design
# Rename vendor/ to lib/ and update import paths to make assets embeddable with go install
if [ -d "$NOVNC_DIR/vendor" ]; then
  echo "[*] Renaming vendor/ to lib/ (go:embed excludes 'vendor' directories)..."
  mv "$NOVNC_DIR/vendor" "$NOVNC_DIR/lib"

  # Update import paths in JavaScript files
  sed -i 's|../vendor/pako|../lib/pako|g' "$NOVNC_DIR/core/deflator.js"
  sed -i 's|../vendor/pako|../lib/pako|g' "$NOVNC_DIR/core/inflator.js"

  echo "[*] Updated vendor→lib references in deflator.js and inflator.js"
fi
