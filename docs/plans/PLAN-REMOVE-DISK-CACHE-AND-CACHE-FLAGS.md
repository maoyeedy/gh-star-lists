# Plan: Remove disk cache and cache CLI flags - Ship Record

## What shipped

Deleted the disk-cache subsystem (~830 lines prod + 572 lines tests) and both
cache-related CLI flags (`--no-cache`, `--cache-ttl`). The only remaining
cache layer is the in-memory `cacheService` (5-minute TTL, lives with the
process). TUI refresh invalidation continues to work via
`svc.(interface{ Invalidate() }).Invalidate()`. Users who pass the removed
flags now get a clean unknown-flag usage error (exit code 2).

- Deleted the six disk-cache implementation files (`diskcache.go`,
  `diskcache_store.go`, `diskcache_coalesce.go`, `diskcache_invalidate.go`,
  `diskcache_policy.go`, `diskcache_test.go`), removing ~1400 lines and the
  `diskCacheRepository` shadow type that coupled the cache layer to
  `domain.Repository` field-by-field.
- Removed `--no-cache` and `--cache-ttl` flags from `Parse`, `Parsed`, all
  help text, and all test cases. Both flags now error as unknown.
- Simplified `run_setup.go`: the service is now unconditionally wrapped with
  `NewCacheServiceWithOptions(svc, CacheOptions{TTL: 5*time.Minute})`. No
  branching on a `cacheTTL` flag; no `originalService` plumbing.
- Deleted `combinedInvalidator`, `wrapServiceForTUI`, and the `originalSvc`/
  `cacheTTL` threading through `runTUIAction` -> `launchTUI`. The TUI now
  receives the already-cached service directly.
- Updated `CLAUDE.md` decorator chain (dropped `diskCacheService`) and
  split-file list (removed five `diskcache_*.go` entries).
- Updated `README.md` feature line from "in-memory + disk cache" to
  "in-memory cache".

## Files changed

| File | Change |
|------|--------|
| `internal/githubapi/diskcache.go` | Deleted - disk-cache service decorator |
| `internal/githubapi/diskcache_store.go` | Deleted - `diskCacheRepository` shadow type and BoltDB store |
| `internal/githubapi/diskcache_coalesce.go` | Deleted - request coalescing for disk cache |
| `internal/githubapi/diskcache_invalidate.go` | Deleted - disk-cache invalidation helpers |
| `internal/githubapi/diskcache_policy.go` | Deleted - TTL and path policy |
| `internal/githubapi/diskcache_test.go` | Deleted - disk-cache test suite |
| `internal/command/parse.go` | Removed `--cache-ttl` and `--no-cache` cases, conflict check, `CacheTTL` field assignments |
| `internal/command/types.go` | Removed `Parsed.CacheTTL *time.Duration` field |
| `internal/command/run_setup.go` | Replaced branching cache setup with unconditional `NewCacheServiceWithOptions`; removed `originalService` and `cacheTTL` from `runInvocation` |
| `internal/command/run_action.go` | Dropped `inv.originalService` and `inv.cacheTTL` from `launchTUI` call |
| `internal/command/run_tui.go` | Deleted `combinedInvalidator`, `wrapServiceForTUI`; simplified `launchTUI` signature |
| `internal/command/help.go` | Removed all `--cache-ttl` and `--no-cache` rows from every help section |
| `internal/command/parse_test.go` | Removed four cache-flag test cases and unused `ptrDuration` helper |
| `internal/command/run_test.go` | Removed `--cache-ttl` positive help assertion and `TestRunNoCacheDisablesCache` |
| `CLAUDE.md` | Decorator chain: removed `diskCacheService`; split-file list: removed five diskcache files |
| `README.md` | Feature line: "in-memory + disk cache" -> "in-memory cache" |

## Design notes

- **Both flags dropped rather than retargeted.** `--cache-ttl` could have
  been retargeted to control the in-memory TTL, but the user-visible value
  would be small (in-memory cache dies with the process) and keeping it would
  preserve the CLI/TUI behavioural asymmetry this change exists to remove.
- **Unconditional in-memory wrapping.** Previously the service was
  conditionally wrapped depending on the `--cache-ttl` flag. Now
  `prepareRunInvocation` always wraps with a fixed 5-minute TTL. The TTL is
  written as an explicit literal (`5 * time.Minute`) to document that it is
  the only TTL in the codebase; `defaultCacheTTL` in `cache.go` is the
  single source of truth.
- **TUI cold-start after disk cache removal: <1s.** Measured on a live
  account immediately after the change. No follow-up for a replacement
  persistent cache is needed at this time; revisit only if this regresses.
