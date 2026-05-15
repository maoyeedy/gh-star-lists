#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f go.mod ]]; then
  echo "error: scripts/check.sh must be run from the repository root" >&2
  exit 1
fi

mkdir -p ./bin
binary="./bin/gh-star-lists$(go env GOEXE)"

go test ./...
go vet ./...
go build -o "$binary" .
