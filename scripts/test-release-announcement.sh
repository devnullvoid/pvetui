#!/usr/bin/env bash
set -euo pipefail

# Local helper for testing release announcement generation without posting.
# Edit the variables below as needed, or override via environment vars.
#
# Examples:
#   ./scripts/test-release-announcement.sh
#   TAG=v1.0.20 ./scripts/test-release-announcement.sh
#   ANNOUNCE_AI_ENABLED=true ANNOUNCE_AI_BASE_URL="https://api.groq.com/openai/v1" \
#     ANNOUNCE_AI_MODEL="llama-3.1-8b-instant" ANNOUNCE_AI_API_KEY="..." \
#     ./scripts/test-release-announcement.sh

TAG="${TAG:-v1.0.20}"
PROJECT_NAME="${PROJECT_NAME:-pvetui}"
REPO_URL="${REPO_URL:-https://github.com/devnullvoid/pvetui}"
CHANGELOG_PATH="${CHANGELOG_PATH:-CHANGELOG.md}"

# AI options (optional)
# Set ANNOUNCE_AI_ENABLED=true to test AI summaries.
ANNOUNCE_AI_ENABLED="${ANNOUNCE_AI_ENABLED:-false}"
ANNOUNCE_AI_BASE_URL="${ANNOUNCE_AI_BASE_URL:-}"
ANNOUNCE_AI_MODEL="${ANNOUNCE_AI_MODEL:-}"
ANNOUNCE_AI_API_KEY="${ANNOUNCE_AI_API_KEY:-${CEREBRAS_API_KEY:-${OPENROUTER_API_KEY:-${GROQ_API_KEY:-${MISTRAL_API_KEY:-}}}}}"
ANNOUNCE_AI_TIMEOUT_SECONDS="${ANNOUNCE_AI_TIMEOUT_SECONDS:-20}"
ANNOUNCE_AI_DEBUG="${ANNOUNCE_AI_DEBUG:-true}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RELEASE_URL="${REPO_URL%/}/releases/tag/${TAG}"

echo "Testing announcement generation (dry-run)"
echo "  TAG:                 $TAG"
echo "  PROJECT_NAME:        $PROJECT_NAME"
echo "  RELEASE_URL:         $RELEASE_URL"
echo "  CHANGELOG_PATH:      $CHANGELOG_PATH"
echo "  ANNOUNCE_AI_ENABLED: $ANNOUNCE_AI_ENABLED"
if [[ -n "$ANNOUNCE_AI_BASE_URL" ]]; then
  echo "  ANNOUNCE_AI_BASE_URL: $ANNOUNCE_AI_BASE_URL"
fi
if [[ -n "$ANNOUNCE_AI_MODEL" ]]; then
  echo "  ANNOUNCE_AI_MODEL:    $ANNOUNCE_AI_MODEL"
fi
if [[ -n "$ANNOUNCE_AI_API_KEY" ]]; then
  echo "  ANNOUNCE_AI_API_KEY:  [set]"
else
  echo "  ANNOUNCE_AI_API_KEY:  [missing]"
fi

cd "$REPO_ROOT"
PROJECT_NAME="$PROJECT_NAME" \
  ANNOUNCE_AI_ENABLED="$ANNOUNCE_AI_ENABLED" \
  ANNOUNCE_AI_BASE_URL="$ANNOUNCE_AI_BASE_URL" \
  ANNOUNCE_AI_MODEL="$ANNOUNCE_AI_MODEL" \
  ANNOUNCE_AI_API_KEY="$ANNOUNCE_AI_API_KEY" \
  ANNOUNCE_AI_TIMEOUT_SECONDS="$ANNOUNCE_AI_TIMEOUT_SECONDS" \
  ANNOUNCE_AI_DEBUG="$ANNOUNCE_AI_DEBUG" \
  python3 scripts/post-release-announcement.py \
  --tag "$TAG" \
  --release-url "$RELEASE_URL" \
  --changelog "$CHANGELOG_PATH" \
  --dry-run
