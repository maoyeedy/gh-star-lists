# Plan: TUI v2 — Performance follow-up

## Context

Address remaining performance gaps from the 02-perf plan: the per-frame O(n) cache scan in spinner rendering, topics preload being gated behind all basic preloads completing, lack of disk cache eviction, synchronous disk writes adding latency to read responses, and no deduplication of concurrent disk cache fills for the same key.

**Prerequisite:** Plan 02-perf (docs/plans/v2/02-perf.md) shipped on `feat/tui`.

## Scope

**In** | **Out**
---|---
O(1) `anyPending` via loading counter | Background cache cleanup daemon
Relaxed topics preload gate (per-list readiness) | Disk cache write queue / batching
Disk cache max-entry cap with oldest-eviction | Configurable disk cache limits or CLI flags
Async disk writes (write-behind goroutine) | Disk cache for `GetRepositoryMemberships`
Concurrent fill deduplication for disk cache | Offline/pre-warmed cache seeding
Regression tests for all of the above | Cache compression or binary format

## Current state

- `anyPendingInCache()` (cache.go:101-108) iterates all `repoCache` entries on every `spinner.Tick` (every render frame). O(n) in number of lists.
- `scheduleTopicsPreload()` (cache.go:126-128) returns nil if `len(p.queue) > 0 || p.inFlight > 0`. Topics are blocked until ALL visible lists finish basic loading.
- `diskCacheService.writeToDisk()` (diskcache.go:106-125) is called synchronously in every read method before returning, adding disk I/O to read latency.
- No cap on cached files under `$XDG_CACHE_HOME/gh-star-lists/`. Only `Invalidate()` or mutations remove entries.
- Concurrent misses for the same disk cache key both call inner and both write to disk (last write wins).

## Design

- **Loading counter:** replace the `anyPendingInCache` map scan with an `int` counter incremented on `repoCacheLoading` entry creation and decremented on transition to loaded/error/cancelled. The `anyPending` check becomes `loadingCount > 0`.
- **Relaxed topics gate:** instead of requiring the entire basic queue to be empty and idle, check on a per-candidate basis — if a specific list's basic repos are cached (state == loaded), allow its topics load even when other lists are still queued. Keep the `topicsInFlight` cap at 2 and the focused-list-first ordering.
- **Disk cache eviction:** on each write, if the cache dir exceeds a hard cap (200 files), scan for the oldest files by modification time and delete the excess. Use `os.ReadDir` + `os.Stat` for simplicity — this runs synchronously but is cheap relative to the network call that preceded it.
- **Async disk writes:** fire a goroutine for `writeToDisk` in each read method. The goroutine owns the serialization and I/O. Errors are silently dropped (cache is best-effort). The caller returns immediately after the inner call.
- **Fill deduplication:** use a `sync.Mutex` guarded `map[string]chan struct{}` (in-flight write map). On cache miss, if another goroutine is already fetching the same key, block on the channel until the first writer signals completion, then read from disk. This avoids duplicate inner calls and duplicate disk writes.

## Phases

| Phase | Goal | Parallel-with | Depends-on | Files | Subagent |
|---|---|---|---|---|---|
| P1 — Cache micro-optimizations | O(1) pending check + relaxed topics preload gate | P2 | — | `internal/tui/cache.go`, `internal/tui/update.go` | general-purpose |
| P2 — Disk cache hardening | Eviction, async writes, fill deduplication | P1 | — | `internal/githubapi/diskcache.go` | general-purpose |
| P3 — Regression tests | Pin P1 and P2 behavior with deterministic tests | — | P1, P2 | `internal/tui/cache_test.go`, `internal/tui/update_cache_test.go`, `internal/githubapi/diskcache_test.go` | general-purpose |

Phase order: (P1 ∥ P2) → P3.

---

### P1 — Cache micro-optimizations

1. Replace `anyPendingInCache()` map scan with a `loadingCount int` field on `preloader`. Increment when a cache entry transitions to `repoCacheLoading`. Decrement when it transitions to `repoCacheLoaded`, `repoCacheError`, or is cancelled/cleared.

2. Relax `scheduleTopicsPreload()` gate: instead of returning nil when `len(p.queue) > 0 || p.inFlight > 0`, remove that early return and let the per-candidate basic-key check (`cache[basicKey] != repoCacheLoaded`) handle gating naturally. Each candidate whose basic repos are loaded can start a topics load independently. Keep the focused-first ordering and `topicsInFlight` cap.

```text
Exit criteria:
- anyPending is O(1) (loadingCount int, no map iteration).
- Topics preload can start for a list whose basic repos are cached, even when other lists are still in the basic queue.
- Existing preload and navigation-cache tests pass.
- make lint && make check passes.
```

### P2 — Disk cache hardening

1. Add a `maxEntries` cap (200) to `diskCacheService`. On each `writeToDisk`, if the cache dir exceeds the cap, scan for oldest files by `os.Stat` + `ModTime` and delete the excess.

2. Make `writeToDisk` asynchronous: fire a goroutine that serializes, writes to tmp file, and renames. The read method returns immediately after the inner call succeeds. Errors in the goroutine are silently dropped.

3. Deduplicate concurrent fills: add a `sync.Mutex` guarded `map[string]chan struct{}` to `diskCacheService`. On cache miss, check if a fill is already in-flight for the key. If so, wait on the channel, then re-read from disk. If not, create the channel, release the lock, call inner, write to disk, close the channel to unblock waiters, then delete the channel entry.

```text
Exit criteria:
- Cache dir does not exceed maxEntries after repeated writes.
- Read methods return without blocking on disk I/O.
- Concurrent goroutines hitting the same cache miss only trigger one inner call.
- make lint && make check passes.
```

### P3 — Regression tests

Add deterministic tests that pin the P1 and P2 changes:

- `TestLoadingCountO1` — verify `preloader.loadingCount` increments/decrements correctly across load, cancel, clear, and error paths.
- `TestTopicsPreloadPerListReadiness` — with one list's basic repos cached and another still queued, verify topics preload starts for the cached list.
- `TestDiskCacheEviction` — write more than `maxEntries` files, verify oldest are removed.
- `TestConcurrentDiskCacheFillDeduplication` — launch concurrent `ListRepositories` calls for the same key, verify inner is called exactly once.

```text
Exit criteria:
- go test ./internal/tui/... ./internal/githubapi/... passes.
- make check passes.
- make ascii-check passes.
```

---

## Tests

| Test | What it covers |
|---|---|
| `TestLoadingCountO1` | `loadingCount` increments on load start, decrements on load/cancel/clear/error |
| `TestTopicsPreloadPerListReadiness` | Topics preload starts for a list whose basic repos are cached while others are still queued |
| `TestDiskCacheEviction` | Writes beyond maxEntries evict oldest files by ModTime |
| `TestConcurrentDiskCacheFillDeduplication` | Concurrent same-key misses trigger exactly one inner call |

## Verification

```
go test ./internal/tui/... ./internal/githubapi/...
make check
make ascii-check
```

Manual smoke:

```
go run . browse
# - Move quickly across many lists; spinner overhead doesn't cause visible jank.
# - Toggle preview on; topics appear for the focused list while other lists are still loading.
# - Restart TUI; cold start renders cached data quickly (no disk write blocking).
# - Open several TUI sessions back-to-back; cache dir stays bounded (~200 files).
```

## Critical files

- `internal/tui/cache.go` — `preloader` struct (~line 32), `anyPendingInCache` (~line 101), `scheduleTopicsPreload` (~line 123), `clear` (~line 88), `focusList` (~line 176)
- `internal/tui/update.go` — `reposLoadedMsg` handler (~line 50), `handleRefresh` (~line 293)
- `internal/githubapi/diskcache.go` — `diskCacheService` struct (~line 35), `writeToDisk` (~line 106), read methods (~line 129–194)

## Reused utilities

- `preloader` struct and methods from P1 refactoring
- `diskCacheService` and `canonicalKey` / `cachePath` helpers
- `fakeCacheInner` in `internal/githubapi/diskcache_test.go` — existing test double
- `fakeService` / `fakeInvalidatableService` in `internal/tui/test_helpers_test.go`
