#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f go.mod ]]; then
  echo "error: scripts/smoke-gh-extension.sh must be run from the repository root" >&2
  exit 1
fi

command -v gh >/dev/null 2>&1 || {
  echo "error: gh must be on PATH" >&2
  exit 1
}

mkdir -p ./bin
binary="./bin/gh-star-lists$(go env GOEXE)"

go build -o "$binary" .
gh extension install . --force
gh star-lists --help | grep -F "repos <LIST_ID>"

echo "gh extension smoke passed"
