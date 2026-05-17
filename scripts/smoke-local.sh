#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f go.mod ]]; then
  echo "error: scripts/smoke-local.sh must be run from the repository root" >&2
  exit 1
fi

binary="./gh-star-lists$(go env GOEXE)"

go test ./...
go vet ./...
go build -o "$binary" .

help="$($binary --help)"
[[ "$help" == *"gh star-lists"* ]]
[[ "$help" == *"Usage:"* ]]
[[ "$help" == *"Commands:"* ]]
[[ "$help" == *"repos <LIST_ID>"* ]]

set +e
stderr="$($binary repos 2>&1 >/dev/null)"
status=$?
set -e
[[ "$status" -eq 2 ]]
[[ "$stderr" == *"missing list id for repos"* ]]
[[ "$stderr" == *"Usage:"* ]]

set +e
stderr="$($binary list --json --tsv 2>&1 >/dev/null)"
status=$?
set -e
[[ "$status" -eq 2 ]]
[[ "$stderr" == *"cannot combine --json and --tsv"* ]]

set +e
stderr="$($binary stars 2>&1 >/dev/null)"
status=$?
set -e
[[ "$status" -eq 2 ]]
[[ "$stderr" == *"unknown command \"stars\""* ]]

echo "local smoke passed"
