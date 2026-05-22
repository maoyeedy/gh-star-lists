# TUI v2 — Performance and cache — Ship Record

## What shipped

Improved TUI perceived speed and API efficiency: focused-list loads are never blocked behind low-value preloads, topics preload in the background after basic loads complete, and cold TUI starts can render cached data from disk before network refresh.

- **Load priority:** when the user focuses an uncached list, in-flight preloads for non-focused lists are cancelled via child-context cancellation, freeing concurrency slots so the focused list loads immediately. Two-layer staleness protection (context cancellation + generation check + cache-entry existence guard) ensures late messages never corrupt state.
- **Topics preload:** after basic `withTopics=false` preloads are idle, low-priority `withTopics=true` loads fill the preview cache for visible lists (focused first, cap 2). Toggling preview off cancels in-flight topics loads. Non-focused topics loads never block basic repo list loads.
- **Disk cache:** `DiskCacheService` in `githubapi` persists list and repository read responses to `$XDG_CACHE_HOME/gh-star-lists/` with TTL-based freshness. Wired only for the TUI path — CLI retains its existing in-memory cache. Mutations invalidate affected disk entries. Respects `--no-cache` and `--cache-ttl`.
- **Perf tests:** deterministic fake-service tests pin load ordering, stale-load dropping, topics-preload enable/disable, disk cache hit/miss, TTL expiry, and invalidation after mutations.

## Files changed

| File | Change |
|------|--------|
| `internal/tui/cache.go` | Extracted `preloader` struct with cancellation support; added `scheduleTopicsPreload`, `cancelTopicsPreloads`, `enqueueFront`, `clear`, `anyPendingInCache` |
| `internal/tui/messages.go` | Added `ctx.Err()` guards in `loadReposCmd` to short-circuit cancelled preloads |
| `internal/tui/model.go` | Replaced scattered preload fields with `preloader *preloader` struct |
| `internal/tui/input.go` | `focusList` cancels non-focused in-flight loads; preview toggle calls `cancelTopicsPreloads` |
| `internal/tui/update.go` | `reposLoadedMsg` handler validates cache entry existence; chains `scheduleTopicsPreload`; `handleRefresh` uses `preloader.clear()` |
| `internal/tui/render_preview.go` | Preview pane prefers `withTopics=true` cache entry |
| `internal/githubapi/diskcache.go` | New: `DiskCacheService` with TTL-based disk read cache for TUI startup |
| `internal/githubapi/diskcache_test.go` | New: `TestDiskCacheWarmStart`, `TestDiskCacheInvalidation`, `TestDiskCacheTTLExpiry` |
| `internal/command/run.go` | TUI path wraps service chain: in-memory cache → disk cache → production; `combinedInvalidator` clears both on refresh |
| `internal/tui/cache_test.go` | Added `TestFocusedLoadPriority`, `TestTopicsPreloadOnlyWhenPreviewEnabled` |
| `internal/tui/update_cache_test.go` | Added `TestStaleLoadDropped` |

Plus test-file field renames (mock structs wiring through `m.preloader`) in `modal_test.go`, `mouse_test.go`, `navigation_cache_test.go`, `navigation_keys_test.go`, `preview_test.go`, `render_content_test.go`, `render_footer.go`, `render_list.go`, `render_list_test.go`, `search.go`, `search_test.go`, `spinner_test.go`, `update_refresh_test.go`.

## Design notes

- **Cancellation over debounce.** The plan allowed either approach. Cancellation was chosen: `context.WithCancel` per preload, stored in `preloader.preloadCancels`. On focus change, non-focused cancels fire immediately, freeing in-flight slots. This gives prompt focused-list loading even against slow networks, whereas debounce would still wait for stale in-flight loads to complete.
- **`preloader` struct extraction.** `golangci-lint --fix` consolidated `cache`, `generation`, `queue`, `inFlight`, and `preloadCancels` into a `preloader` struct with methods. This was a linter-driven refactor, not manual. Downstream phases adapted to the new field paths naturally.
- **Topics preload as a separate scheduling pass.** `scheduleTopicsPreload()` only runs when the basic queue is empty and `inFlight == 0`. This guarantees topics loads never compete with basic repo-list loads for network bandwidth, at the cost of delaying topic availability until all visible lists are loaded.
- **Disk cache is TUI-only.** The disk cache wraps only for `ActionTUI` and the `ActionList` fallback-to-TUI path. CLI actions use the existing in-memory `cacheService` unchanged. This keeps the CLI contract intact — disk I/O never affects scripted `gh star-lists repos` output.
- **Combined invalidation.** `wrapServiceForTUI` decorates both in-memory and disk caches with a `combinedInvalidator` so the TUI's `svc.(invalidatable).Invalidate()` on manual refresh clears both layers atomically.
