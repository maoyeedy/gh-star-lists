---
name: go-check
description: Run full Go validation pipeline - goimports, vet, tests. Use when user asks validate, check, verify, or run Go tests. Also after editing Go files to confirm pass. Invoke /go-check.
disable-model-invocation: true
---

Run steps sequentially. Stop on first failure. Report failing step full output. No continue past failure.

1. `goimports -w .`
2. `go vet ./...`
3. `go test ./...`

On success: report all three passed.
On failure: name failing step, show stdout/stderr, stop.
