# TUI v2 — Default experience and cleanup — Ship Record

## What shipped

Made the TUI the default interactive experience while keeping the CLI scripting surface intact. Split the monolithic TUI files into focused components that can support follow-up UX and performance work without destabilizing behavior.

- Per-pane search state: list search and repo search retain independent query text; clearing one pane does not affect the other
- Component boundaries: extracted `preloader` type for cache/preload logic, standalone `formatPreviewContent` for preview formatting, standalone `renderFooter` for footer rendering — root model delegates to these narrow helpers
- Contextual help: footer and full help both derive key text from `key.Binding.Help()` definitions; footer content differs by active pane (list, repo, repo-with-selection, search, modal). Adding a key binding now only requires updating `keys.go`
- Default flow audit: confirmed `gh star-lists` opens TUI only for interactive no-arg/no-output-flag use; all CLI output paths remain script-safe with no regressions

## Files changed

| File | Change |
|------|--------|
| `internal/tui/search_test.go` | Fixed stale references to removed `m.searchQuery` field |
| `internal/tui/cache.go` | Extracted `preloader` type with cache/preload/inflight/cancel methods |
| `internal/tui/model.go` | Replaced raw cache fields with `*preloader` pointer |
| `internal/tui/update.go` | Delegated cache operations through `m.preloader.*` |
| `internal/tui/input.go` | Delegated cache access through `m.preloader.*` |
| `internal/tui/render_repo.go` | Delegated cache access through `m.preloader.*` |
| `internal/tui/render_preview.go` | Extracted `formatPreviewContent(repo, w)` standalone function |
| `internal/tui/render_footer.go` | Extracted `renderFooter(...)` standalone function; contextual per-pane binding display |
| `internal/tui/render.go` | Calls standalone `renderFooter` instead of model method |
| `internal/tui/render_help.go` | Rewrote to derive all help content from `keys.helpGroups()` and `key.Binding.Help()` |
| `internal/tui/keys.go` | Added `helpGroup` struct, `helpGroups()`, `footerBindings()` — single source of truth for key text |
| `internal/tui/render_header_footer_test.go` | Updated for standalone `renderFooter` signature |
| `internal/tui/*_test.go` | Updated cache field references to `m.preloader.*` |

## Design notes

- **Key bindings as single source of truth.** Both footer help and full help modal derive key/description text from `key.Binding.Help()`. The `helpGroups()` method organizes bindings into logical groups (Navigation, List Actions, Repo Actions, Selection/Bulk, Preview, View, Quit). No hand-maintained duplicate help strings remain.
- **Preloader extraction boundary.** The `preloader` type owns cache map, generation counter, preload queue, inflight counter, and cancel map. The root model holds a `*preloader` pointer. Some call sites (`focusList` cancellation, `countPreviewLines`) still reach into model state — those were noted as follow-up candidates but deferred since extracting them would require parameter threading beyond the phase scope.
