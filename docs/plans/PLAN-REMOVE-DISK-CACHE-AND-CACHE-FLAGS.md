# Plan: Remove disk cache and cache CLI flags

## Context

The disk cache feature (`internal/githubapi/diskcache_*.go`, ~830 lines prod + 572 lines tests) is wired only into the TUI path and exists to make cold TUI re-opens within 5 minutes paint instantly from `$XDG_CACHE_HOME/gh-star-lists/`. Its cost/benefit is upside-down: it carries a shadow `diskCacheRepository` type that couples to `domain.Repository` field-by-field (violating the "domain is the leaf" invariant in CLAUDE.md), an async-write lifecycle that has produced CI races (the unmerged `PLAN-ROBUST-DISK-CACHE-ASYNC-WRITES.md` adds more code to fix it), and a TUI-vs-CLI behavioural asymmetry where `--no-cache` means different things on each path. `gh` itself defaults to no cache (opt-in via `--cache`) and `gh-dash` is in-memory-only. Removing the disk cache and both cache-related CLI flags aligns this tool with that convention, deletes ~1400 lines of code + tests, eliminates a coupling violation, and removes a recurring source of test flake — at the cost of slower TUI cold starts (re-opens within 5 minutes lose their instant paint; first paint always waits on the network). The in-memory `cacheService` (intra-session, hardcoded 5-minute TTL) stays, and mutation invalidation stays. If cold-start latency turns out to be painful in practice, a much leaner replacement (single JSON file persisting only `[]domain.StarList`, ~50 lines) can be added back later — but only after measuring.

**Prerequisite:** Current `master` (commit `0a4a795` — TUI v1 shipped on `feat/tui`). The unmerged `PLAN-ROBUST-DISK-CACHE-ASYNC-WRITES.md` is superseded by this plan and should be deleted as part of P3.

## Scope

**In** | **Out**
---|---
Delete `diskcache.go`, `diskcache_store.go`, `diskcache_coalesce.go`, `diskcache_invalidate.go`, `diskcache_policy.go`, `diskcache_test.go` | Removing or altering the in-memory `cacheService` (it stays as-is)
Remove `--no-cache` and `--cache-ttl` flag parsing, validation, conflict check, `Parsed.CacheTTL` field, and all test cases | Changing mutation invalidation semantics in `cacheService`
Simplify `wrapServiceForTUI`, delete `combinedInvalidator`, delete `originalService` plumbing through `runInvocation` | Touching the `lazyService -> RetryService -> cacheService -> graphQLService` chain (RetryService and lazyService stay)
Update CLAUDE.md (decorator chain string, package map, split-file list), README ("in-memory + disk cache" line), `help.go` usage strings | Adding any replacement persistent cache in this plan (deferred — only revisit if cold-start latency is measured as painful)
Delete the superseded `docs/plans/PLAN-ROBUST-DISK-CACHE-ASYNC-WRITES.md` | Renaming or repurposing existing in-memory cache options/types

## Current state

- `internal/command/run_tui.go:77-96` wraps the production service with `NewDiskCacheService` then `NewCacheServiceWithOptions` then `combinedInvalidator`. CLI path does not.
- `internal/command/parse.go:26-27,83-99,253-259` accepts `--cache-ttl <duration>` and `--no-cache`, with a conflict check between the two.
- `internal/command/types.go:98` exposes `Parsed.CacheTTL *time.Duration`.
- `internal/command/run_setup.go:48-58,77-87` derives a `cacheTTL` and an `originalService` that are then threaded into `runInvocation` so that the TUI launch path can build its own decorator stack.
- `internal/command/run_action.go:393-401` (`runTUIAction`) passes `inv.service`, `inv.originalService`, and `inv.cacheTTL` into `launchTUI`.
- `internal/command/help.go:23,76,157,194,378,401,464` mentions both flags in usage and detailed help.
- `CLAUDE.md:11` documents the decorator chain including `diskCacheService`. `CLAUDE.md:49` lists all disk-cache split files. `README.md:29` includes "disk cache" in the feature list.
- `docs/plans/PLAN-ROBUST-DISK-CACHE-ASYNC-WRITES.md` is a still-unapplied plan to add an explicit async-write tracker to the disk cache. Superseded by this removal.

## Design

- **Keep in-memory cache TTL hardcoded.** The existing `defaultCacheTTL = 5 * time.Minute` in `internal/githubapi/cache.go:11` becomes the only TTL. No flag exposes it. If a user wants fresh data, they can quit and re-run; in-memory cache dies with the process. This matches `gh-dash`.
- **Drop both flags rather than retargeting one.** `--cache-ttl` could in principle be retargeted to control the in-memory cache only, but the user-visible value is small (in-memory cache only lives for the duration of one process), and keeping it would preserve the very ambiguity this plan exists to remove. Both flags go.
- **Keep `cacheService` as a thin in-memory decorator.** `Invalidate()` stays on `cacheService`; the TUI continues to invalidate on manual refresh via `svc.(interface{ Invalidate() }).Invalidate()`. No changes to mutation invalidation in `cacheService`.
- **`wrapServiceForTUI` becomes trivial.** It returns `NewCacheServiceWithOptions(svc, CacheOptions{TTL: defaultCacheTTL})`. The `originalSvc` argument is dropped and `combinedInvalidator` deleted.
- **`runInvocation.originalService` and `runInvocation.cacheTTL` are deleted.** Single service field threads through. `prepareRunInvocation` no longer branches on `cacheTTL`.
- **No backwards-compatibility hack.** Users who pass `--no-cache` or `--cache-ttl ...` get a clean unknown-flag error from `Parse`. This is consistent with the CLAUDE.md project rule "Avoid backwards-compatibility hacks".

## Phases

| Phase | Goal | Parallel-with | Depends-on | Files | Subagent |
|---|---|---|---|---|---|
| P1 — Rip out disk cache + cache flags | Delete disk-cache package files, remove cache flags from parse/types, simplify TUI wiring, drop `originalService`/`cacheTTL` plumbing. Build must pass. | — | — | `internal/githubapi/diskcache.go`, `internal/githubapi/diskcache_store.go`, `internal/githubapi/diskcache_coalesce.go`, `internal/githubapi/diskcache_invalidate.go`, `internal/githubapi/diskcache_policy.go`, `internal/githubapi/diskcache_test.go`, `internal/command/parse.go`, `internal/command/types.go`, `internal/command/run_setup.go`, `internal/command/run_action.go`, `internal/command/run_tui.go` | general-purpose |
| P2 — Test fixup and verification | Drop cache-flag test cases, run full test+lint+ascii suite, race test the `githubapi` package, manual cold-start TUI sanity. | — | P1 | `internal/command/parse_test.go`, `internal/command/run_test.go` | general-purpose |
| P3 — Documentation cleanup | Update CLAUDE.md decorator chain + split-file list + checklist references; update README feature line; strip both flags from `help.go`; delete the superseded async-writes plan. | — | P2 | `CLAUDE.md`, `README.md`, `internal/command/help.go`, `docs/plans/PLAN-ROBUST-DISK-CACHE-ASYNC-WRITES.md` | general-purpose |

Phase order: P1 → P2 → P3.

---

### P1 — Rip out disk cache + cache flags

Delete the disk-cache implementation and all surrounding plumbing so that the only cache layer left is the in-memory `cacheService`. After this phase, `go build ./...` and `go vet ./...` both pass, but tests referencing `--no-cache` / `--cache-ttl` / `CacheTTL` will fail — that is intentional and is fixed in P2.

- Delete the six disk-cache files outright:
  - `internal/githubapi/diskcache.go`
  - `internal/githubapi/diskcache_store.go`
  - `internal/githubapi/diskcache_coalesce.go`
  - `internal/githubapi/diskcache_invalidate.go`
  - `internal/githubapi/diskcache_policy.go`
  - `internal/githubapi/diskcache_test.go`
- File `internal/command/parse.go`:
  - Remove the `cacheTTL *time.Duration` and `noCacheFlag bool` locals (line ~26-27).
  - Remove the `--cache-ttl` case (line ~83-95) and the `--no-cache` case (line ~98-99).
  - Remove the conflict block (line ~253-259) that enforces `cannot combine --no-cache and --cache-ttl`.
  - Remove every `CacheTTL: cacheTTL` field assignment in the `Parsed{...}` returns (verified locations: line ~337, ~633, ~670; grep again before deleting).
  - If `time` import becomes unused after this, drop it.
- File `internal/command/types.go`:
  - Remove the `CacheTTL *time.Duration` field at line ~98 from `Parsed`.
  - If `time` import becomes unused, drop it.
- File `internal/command/run_setup.go`:
  - Remove the `cacheTTL` derivation block (line ~48-58). The service now needs unconditional in-memory wrapping: `service = githubapi.NewCacheServiceWithOptions(service, githubapi.CacheOptions{TTL: 5 * time.Minute})`. Keep `time.Minute` usage; the explicit literal documents the only TTL in the codebase.
  - Remove `originalService` local and `cacheTTL` local entirely.
  - Remove `originalService` and `cacheTTL` fields from the `runInvocation` struct (line ~15-25).
  - Remove the corresponding field assignments in the constructed `runInvocation{}` (line ~77-87).
- File `internal/command/run_action.go`:
  - `runTUIAction` (line ~385-402): drop the `inv.originalService` and `inv.cacheTTL` arguments to `launchTUI`. New signature is `launchTUI(inv.ctx, inv.stderr, inv.parsed, inv.service, inv.diagnosticOptions)`.
- File `internal/command/run_tui.go`:
  - Delete the entire `combinedInvalidator` struct and `newCombinedInvalidator` constructor (line ~25-51).
  - Replace `launchTUI` signature with `func launchTUI(ctx context.Context, stderr io.Writer, parsed Parsed, svc githubapi.Service, diagnosticOpts format.Options) int`.
  - Replace `wrapServiceForTUI` with a no-op pass-through OR remove it entirely. Since the in-memory cache wrapping now happens unconditionally in `prepareRunInvocation`, the TUI does not need any extra wrapping — the service it receives is already cached. Delete `wrapServiceForTUI` entirely; call `runTUI(ctx, svc, ...)` directly.
  - Drop `originalSvc` parameter and `cacheTTL` parameter throughout.
  - If `time` import becomes unused, drop it.
- After all edits, run `go tool goimports -w` on every changed file.

```text
Exit criteria:
- The six disk-cache files no longer exist
- grep -n '"--cache-ttl"\|"--no-cache"\|CacheTTL\|originalService\|combinedInvalidator\|wrapServiceForTUI\|NewDiskCacheService\|DiskCacheOptions' internal/command/ internal/githubapi/ main.go returns zero matches in non-test prod code
- go build ./... passes
- go vet ./... passes
- Test failures, if any, are limited to references to removed flags / removed Parsed.CacheTTL field — no other regressions
```

### P2 — Test fixup and verification

Update the command package tests so the suite is green again, then run the full verification gates. No new test files; fix-in-place.

- File `internal/command/parse_test.go`:
  - Remove the `cache-ttl flag` case (line ~362-368).
  - Remove the `cache-ttl zero disables` case (line ~371-377).
  - Remove the `no-cache flag` case (line ~380-386).
  - Remove the `no-cache and cache-ttl conflict` case (line ~631-633).
  - Remove `ptrDuration` helper if it becomes unused after the above edits; otherwise leave it.
  - Remove any `CacheTTL:` field references in `wantParsed` literals elsewhere in the file.
- File `internal/command/run_test.go`:
  - Line ~1947: the case asserting help text does **not** mention `--cache-ttl` — keep, this is now a permanent invariant. If the case was conditional on a particular help topic, audit whether `wantNot` semantics still make sense; otherwise leave it.
  - Line ~1968: the case asserting help text **does** mention `--cache-ttl` — delete this case (the assertion is now wrong).
  - Line ~2507: the `runCommand(... []string{"list", "--no-cache"}, ...)` invocation — delete the entire test case or rewrite it without the flag. If the test was specifically pinning `--no-cache` behaviour, delete it; otherwise rewrite the argv to `[]string{"list"}` and confirm the rest of the case still makes sense.
- Audit remaining references with `grep -rn 'cache-ttl\|no-cache\|CacheTTL' internal/` and clean up anything that turns up.
- Run the verification suite end to end:
  - `make test`
  - `make vet`
  - `make lint` (golangci-lint --fix)
  - `make build`
  - `make check`
  - `make ascii-check`
  - `go test -race ./internal/githubapi/ ./internal/command/`
- Manual smoke (record the cold-start time so the user can decide whether to revisit a leaner persistent cache later):
  - `./gh-star-lists tui` — confirm it launches against the real account, lists pane paints, repos pane paints, refresh key still works.
  - `time ./gh-star-lists tui` ... press `q` immediately after lists pane paints — note the time-to-first-paint.
  - `./gh-star-lists --no-cache` and `./gh-star-lists --cache-ttl 5m` — confirm both return a usage error mentioning unknown flag, exit code 2.

```text
Exit criteria:
- make test passes
- make vet passes
- make lint passes
- make build passes
- make check passes
- make ascii-check passes
- go test -race ./internal/githubapi/ ./internal/command/ passes
- grep -rn 'cache-ttl\|no-cache\|CacheTTL\|originalService\|combinedInvalidator\|NewDiskCacheService\|DiskCacheOptions' internal/ main.go returns zero matches
- Manual TUI cold start succeeds and time-to-first-paint is recorded in the PR description
```

### P3 — Documentation cleanup

Strip every remaining reference to disk cache or the removed flags from human-facing docs and project-instruction files, and delete the superseded async-writes plan. This is the only phase that touches markdown.

- File `CLAUDE.md`:
  - Line ~11 (decorator chain): change `lazyService -> RetryService -> cacheService -> diskCacheService -> graphQLService` to `lazyService -> RetryService -> cacheService -> graphQLService`.
  - Line ~49 (split-file list under `githubapi`): remove the five disk-cache file names (`diskcache.go`, `diskcache_policy.go`, `diskcache_store.go`, `diskcache_coalesce.go`, `diskcache_invalidate.go`).
  - Line ~69 (checklist "New cache?"): keep — the invariant still applies to the in-memory cache.
  - Audit the rest of the file (especially the "Cross-cutting concern? Service decorator..." bullet) — no other disk-cache wording should remain, but grep `grep -n 'disk' CLAUDE.md` to confirm.
- File `README.md`:
  - Line ~29: change `Other: in-memory + disk cache, fuzzy search, retry with backoff` to `Other: in-memory cache, fuzzy search, retry with backoff`.
- File `internal/command/help.go`:
  - Line ~23 (TUI usage line): remove `[--cache-ttl <duration>] [--no-cache]`.
  - Line ~76 (TUI flag block): remove the `--no-cache` line. Audit nearby lines for any `--cache-ttl` row and remove.
  - Line ~157 (likely list flag block): remove the `--no-cache` line.
  - Line ~194 (likely repos flag block): remove the `--no-cache` line.
  - Line ~378 (TUI usage block in `ActionTUI` help): remove `[--cache-ttl <duration>] [--no-cache]`.
  - Line ~401: remove the `--no-cache` row.
  - Line ~464 (combined usage at bottom): remove `[--cache-ttl <duration>] [--no-cache]`.
  - After editing, search the file for any remaining `cache-ttl` or `no-cache` string and remove it.
- File `docs/plans/PLAN-ROBUST-DISK-CACHE-ASYNC-WRITES.md`:
  - Delete the file. This plan was scoped to harden disk-cache async writes; that subsystem no longer exists.
- Final sweep: `grep -rn 'disk[- ]cache\|cache-ttl\|no-cache' README.md CLAUDE.md docs/ internal/command/help.go` should return zero matches.

```text
Exit criteria:
- grep -rn 'disk[- ]cache\|cache-ttl\|no-cache' README.md CLAUDE.md docs/ internal/command/help.go returns zero matches
- docs/plans/PLAN-ROBUST-DISK-CACHE-ASYNC-WRITES.md no longer exists
- make ascii-check still passes
- ./gh-star-lists --help shows no --cache-ttl or --no-cache row
- ./gh-star-lists tui --help shows no --cache-ttl or --no-cache row
- git --no-pager diff --check passes
```

---

## Tests

| Test | What it covers |
|---|---|
| Existing `internal/command/parse_test.go` cases | Confirm Parse rejects `--no-cache` and `--cache-ttl` as unknown flags (no new test needed — the default unknown-flag path in Parse already returns a `UsageError`; remove the old positive cases and rely on the existing unknown-flag coverage). |
| Existing `internal/command/run_test.go` help-text invariants | Help output never mentions removed flags. |
| Existing `internal/githubapi/cache_test.go` | In-memory cache hit/miss/invalidate behaviour is unchanged. |
| Existing `internal/command/run_test.go` TUI launch tests | Confirm TUI still wires `cacheService` and refresh still invalidates. |

No new tests are required. The change is a deletion; the in-memory cache layer already has its own coverage, and the unknown-flag path is already exercised by `Parse`.

## Verification

```text
make test
make vet
make lint
make build
make check
make ascii-check
go test -race ./internal/githubapi/ ./internal/command/
grep -rn 'cache-ttl\|no-cache\|CacheTTL\|originalService\|combinedInvalidator\|NewDiskCacheService\|DiskCacheOptions\|disk[- ]cache' internal/ main.go README.md CLAUDE.md docs/ internal/command/help.go
git --no-pager diff --check
```

Manual smoke:

```text
./gh-star-lists tui
# - TUI launches against real account
# - Lists pane paints; repos pane paints on focus
# - Refresh key (r) still invalidates in-memory cache and re-fetches
# - q quits cleanly

time ./gh-star-lists tui  # then immediately quit
# - Record cold-start time-to-first-paint in the PR description
# - If >2s and routinely painful, log a follow-up to consider a leaner JSON-of-lists-only persistent cache

./gh-star-lists --no-cache
# - Exit code 2, usage error about unknown flag

./gh-star-lists --cache-ttl 5m
# - Exit code 2, usage error about unknown flag

./gh-star-lists --help | grep -E 'cache-ttl|no-cache'
# - Zero matches

./gh-star-lists tui --help | grep -E 'cache-ttl|no-cache'
# - Zero matches

ls $XDG_CACHE_HOME/gh-star-lists/ 2>/dev/null
# - Directory may still exist from a previous run; new runs no longer touch it. Safe to delete manually; not required.
```

## Critical files

- `internal/githubapi/diskcache.go` — full deletion (whole file)
- `internal/githubapi/diskcache_store.go` — full deletion (incl. `diskCacheRepository` shadow type at line ~32-48 that this plan exists to remove)
- `internal/githubapi/diskcache_coalesce.go` — full deletion
- `internal/githubapi/diskcache_invalidate.go` — full deletion
- `internal/githubapi/diskcache_policy.go` — full deletion
- `internal/githubapi/diskcache_test.go` — full deletion
- `internal/command/parse.go` — `cacheTTL`/`noCacheFlag` locals (~line 26-27), `--cache-ttl` case (~line 83-95), `--no-cache` case (~line 98-99), conflict block (~line 253-259), `CacheTTL:` field references in all `Parsed{...}` returns (~lines 337, 633, 670 — re-grep before editing)
- `internal/command/types.go` — `Parsed.CacheTTL` field at line ~98
- `internal/command/run_setup.go` — `cacheTTL` derivation (~line 48-58), `originalService` local (~line 52), `runInvocation` struct fields (~line 15-25), constructed-struct assignments (~line 77-87)
- `internal/command/run_action.go` — `runTUIAction` arg list to `launchTUI` (~line 393-401)
- `internal/command/run_tui.go` — `combinedInvalidator` struct + ctor (~line 25-51), `launchTUI` signature (~line 53-75), `wrapServiceForTUI` (~line 77-96)
- `internal/command/parse_test.go` — cache-flag test cases (~lines 362-386, 631-633)
- `internal/command/run_test.go` — help-text invariant cases (~lines 1947, 1968), `--no-cache` runtime test (~line 2507)
- `internal/command/help.go` — flag rows at lines ~23, 76, 157, 194, 378, 401, 464
- `CLAUDE.md` — decorator chain line ~11, split-file list line ~49
- `README.md` — feature line ~29

## Reused utilities

- `NewCacheServiceWithOptions(svc, CacheOptions{TTL: 5*time.Minute})` (`internal/githubapi/cache.go:52`) — the only cache wrapping now needed
- `Invalidate()` on `cacheService` (`internal/githubapi/cache.go:242`) — TUI refresh path still calls this via `svc.(interface{ Invalidate() }).Invalidate()`; no replacement needed for `combinedInvalidator`
- `defaultCacheTTL = 5 * time.Minute` (`internal/githubapi/cache.go:11`) — the single source of truth for the cache TTL after this plan
- `UsageError` returned by `Parse` for unknown flags — already covers `--no-cache`/`--cache-ttl` rejection paths without new test cases

## Out of scope

- Replacement persistent cache (e.g. a 50-line JSON-of-lists-only snapshot) — deferred until cold-start latency is measured as painful in P2's manual smoke. If measurement shows the loss is real, file a follow-up plan; do not pre-emptively rebuild.
- Changing the in-memory cache TTL or exposing it via a flag — keep it hardcoded at 5 min.
- Refactoring the decorator chain beyond removing `diskCacheService` (e.g. merging `RetryService` and `cacheService`) — out of scope.
- Removing the `Invalidate()` interface assertion in the TUI — the in-memory `cacheService` still implements it.
- Removing `$XDG_CACHE_HOME/gh-star-lists/` directories from users' machines — leave any pre-existing files in place; they will simply never be read or written again.
