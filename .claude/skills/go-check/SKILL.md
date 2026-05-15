---
name: go-check
description: Run full Go validation pipeline for this repo — goimports, vet, and tests. Use this when the user asks to validate, check, verify, or run tests on the Go code. Also use after making changes to Go files to confirm everything passes. Invoke with /go-check.
disable-model-invocation: true
---

Run each step in sequence. Stop immediately on first failure and report the failing step's full output. Do not continue past a failure.

1. `goimports -w .`
2. `go vet ./...`
3. `go test ./...`

On success: report all three steps passed.
On failure: name the failing step, show its stdout/stderr, and stop.
