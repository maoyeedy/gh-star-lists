# TUI v2 Performance Follow-up - Ship Record

## What shipped

This follow-up closes the remaining performance gaps from the 02-perf plan: TUI spinner state no longer scans every cached repo entry per frame, topics preloads no longer wait for all basic preloads to finish, and the disk cache now bounds storage growth while reducing read-path latency and duplicate concurrent fills.

- TUI pending-state checks now use an O(1) loading counter instead of scanning the repo cache.
- Topics preloads can start for any list whose basic repositories are already cached, even while other basic preloads are still queued or in flight.
- Disk cache writes now run behind the read response, with same-key concurrent misses deduplicated so only one inner API call fills the cache.
- Disk cache storage is capped at 200 files, evicting the oldest files after successful writes.
- Regression tests now pin loading-count transitions, per-list topics readiness, disk cache eviction, and same-key fill deduplication.

## Files changed

| File | Change |
|------|--------|
| `internal/tui/cache.go` | Added loading-count bookkeeping helpers and relaxed topics preload scheduling. |
| `internal/tui/input.go` | Updated preview-topic loading to use the shared loading counter and topics concurrency cap. |
| `internal/tui/update.go` | Routed cache state transitions through loading-count helpers and schedules basic/topics preload work together after repo loads. |
| `internal/tui/cache_test.go` | Added coverage for per-list topics preload readiness. |
| `internal/tui/update_cache_test.go` | Added loading-count transition coverage and updated pending-state assertions. |
| `internal/githubapi/diskcache.go` | Added async write-behind, max-entry eviction, same-key fill deduplication, and generation guards for invalidation. |
| `internal/githubapi/diskcache_test.go` | Added async-write waits for existing tests plus eviction and concurrent-fill regression coverage. |

## Design notes

- **Loading counter:** cache mutations go through helper methods so `loadingCount` stays aligned with loading entries and `anyPending` remains O(1).
- **Per-list topics readiness:** topics scheduling relies on each candidate list's basic cache state instead of a global basic-preload idle gate, preserving focused-first ordering while avoiding unnecessary stalls.
- **Write-behind cache fills:** first callers return after the inner service succeeds and start disk writes asynchronously; waiters for the same key wait for the fill to finish, then re-read from disk to avoid duplicate inner calls.
- **Generation guards:** invalidation increments a private generation so async writes started before invalidation cannot repopulate stale cache files.
