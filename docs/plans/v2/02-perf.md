# Plan: TUI v2 - Performance and cache

## Context

Improve perceived speed and API efficiency for the TUI without changing the CLI contract. The current session cache and cap-3 preloader work, but fast list navigation can leave the focused list waiting behind less relevant in-flight loads, preview topics are fetched only on demand, and cold starts always need network reads.

**Prerequisite:** TUI v2 cleanup has separated pane state and cache/preload logic enough to modify loading behavior safely.

## Scope

**In** | **Out**
---|---
Focused-list load prioritization | New GitHub API surface outside `githubapi.Service`
Debounced focus or cancellation for stale loads | Runtime account or host switching
Background `withTopics=true` preview preload | Fetching topics for CLI paths unless already requested
Disk cache for cold-start TUI reads | Disk cache for mutations or write queues
Cache invalidation on writes and refresh | Long-lived daemon or background sync process

## Current state

- `schedulePreload()` starts up to three `withTopics=false` list repo loads.
- Focus promotion prepends the focused list to the queue but cannot cancel already in-flight work.
- Preview toggling fetches `withTopics=true` only for the focused list.
- Cache is in-memory only and cleared on process exit.
- Manual refresh type-asserts `svc.(interface{ Invalidate() })`.

## Design

- Keep correctness simple: stale async messages must be dropped using generation IDs and/or canceled contexts.
- Prioritize the focused list over breadth preloading.
- Treat `withTopics=true` as lower priority than basic repo lists, except for the focused preview.
- Store only read cache data on disk; mutations must invalidate affected entries instead of replaying writes.

## Phases

| Phase | Goal | Parallel-with | Depends-on | Files | Subagent |
|---|---|---|---|---|---|
| P1 - Load priority | Ensure focused list loads are never blocked behind low-value preloads | none | none | `internal/tui/cache.go`, `internal/tui/messages.go`, `internal/tui/input.go` | general-purpose |
| P2 - Topics preload | Add low-priority background `withTopics=true` preload for visible lists | none | P1 | `internal/tui/cache.go`, `internal/tui/render_preview.go` | general-purpose |
| P3 - Disk cache | Add cold-start read cache under `githubapi` and wire it only into TUI-safe read paths | none | P1 | `internal/githubapi/cache.go`, `internal/tui/cache.go`, `internal/command/run.go` | general-purpose |
| P4 - Perf tests | Pin stale-load, topics-preload, and disk-cache behavior | none | P1, P2, P3 | TUI and `githubapi` tests | general-purpose |

Phase order: P1 -> P2 -> P3 -> P4.

---

### P1 - Load priority

Implement one of two acceptable approaches: cancel stale non-focused in-flight preload commands with per-command child contexts, or debounce list focus so rapid cursor movement only schedules the final focused list before breadth preload resumes. Preserve generation checks so late messages never overwrite current state.

```text
Exit criteria:
- Rapid list navigation prioritizes the final focused list.
- Stale repo load messages cannot replace current focused-list repos.
- Existing preload and navigation-cache tests pass.
```

### P2 - Topics preload

After basic `withTopics=false` preloads are complete or idle, schedule bounded `withTopics=true` loads for visible lists when preview is enabled. The focused list's preview data remains highest priority. Background topics preload must stop when preview is disabled or generation changes.

```text
Exit criteria:
- Focused preview opens with topics if already preloaded.
- Non-focused topics loads do not block basic repo list loads.
- Preview-off mode does not fetch topics.
```

### P3 - Disk cache

Add an opt-in disk read cache for TUI startup data. Cache list and repository read responses using stable keys that include host, list ID, `withTopics`, and cache version. Use TTL-based freshness. Writes and manual refresh must invalidate affected disk entries. Keep CLI `--no-cache` and `--cache-ttl` semantics intact.

```text
Exit criteria:
- Warm TUI startup can render cached lists/repos before network refresh.
- Mutations invalidate affected disk cache entries.
- Non-interactive CLI output remains governed by existing cache flags.
```

### P4 - Perf tests

Add deterministic fake-service tests for load ordering, cancellation/debounce, topics preload priority, disk cache hit/miss, TTL expiry, and invalidation after create/edit/delete/repo-list mutations.

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
| `TestFocusedLoadPriority` | Focused list load is scheduled before queued preloads |
| `TestStaleLoadDropped` | Late stale generation messages are ignored |
| `TestTopicsPreloadOnlyWhenPreviewEnabled` | Topic fetches are conditional and low priority |
| `TestDiskCacheWarmStart` | Cached reads seed TUI before network refresh |
| `TestDiskCacheInvalidation` | Writes and manual refresh clear affected entries |

## Verification

```text
go test ./internal/tui/... ./internal/githubapi/...
make check
make ascii-check
```

Manual smoke:

```text
go run . browse
# - Move quickly across many lists; focused repo pane loads promptly.
# - Toggle preview on; focused repo details populate, topics appear once available.
# - Restart TUI; cached lists/repos appear quickly, then refresh cleanly.
# - Perform add/remove/move; affected list refreshes and stale cache does not reappear.
```

## Critical files

- `internal/tui/cache.go` - preload queue and cache state
- `internal/tui/messages.go` - async load commands and generation messages
- `internal/githubapi/cache.go` - cache wrapper and invalidation behavior

## Reused utilities

- `errgroup.SetLimit` - bounded concurrent work
- `context.Context` / `context.CancelFunc` - cancellation of stale loads
- `repoCacheKey{listID, withTopics}` - existing in-memory cache key shape
- `invalidatable` interface - TUI manual refresh hook

## Out of scope

- Background daemon, sync service, or offline write queue
- Any new command-line cache management UI
- Fetching topics for default CLI repo listings when not requested by filters/templates
