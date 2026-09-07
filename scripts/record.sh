#!/usr/bin/env bash
# Render docs/demo.tape with VHS.
# Usage: ./scripts/record.sh [revision]
#   revision  Git hash / ref / A..B range. Default: HEAD.
#             Swap this in later when you pick a showcase commit.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMMIT="${1:-HEAD}"
TAPE_SRC="${ROOT}/docs/demo.tape"

if ! command -v vhs >/dev/null 2>&1; then
  echo "record.sh: vhs is not on PATH. Install https://github.com/charmbracelet/vhs" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "record.sh: go is not on PATH" >&2
  exit 1
fi

cd "$ROOT"

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "record.sh: not a git repository" >&2
  exit 1
fi

if [[ "$COMMIT" == *..* ]]; then
  HASH="$COMMIT"
else
  HASH="$(git rev-parse --short "$COMMIT")"
fi

WORKDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

echo "building gitlogue ..."
go build -o "$WORKDIR/gitlogue" ./cmd/gitlogue

# Keep Output paths relative to the repo root.
sed "s/__COMMIT__/${HASH}/g" "$TAPE_SRC" > "$WORKDIR/demo.tape"

echo "recording gitlogue ${HASH} ..."
PATH="${WORKDIR}:${PATH}" vhs "$WORKDIR/demo.tape"
echo "wrote docs/demo.gif and docs/demo.mp4"
