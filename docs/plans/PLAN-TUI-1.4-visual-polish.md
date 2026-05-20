# TUI v1.4 — Ship Record

## What shipped

v1.4 upgraded `go run . browse` from a sparse plaintext TUI to a visually structured terminal workbench matching the clarity of the fzf reference. All changes are in rendering, styling, and small interaction quality — async preload, session cache, request generation, and stale-response filtering remain deferred to v1.5.

**Styles and spinner (P1)**
- Expanded `internal/tui/styles.go` from 9 generic styles to 24 semantic style variables covering every palette role: app title, pane chrome, repo fields (stars, language, name variants, URL, badge), search prompt, empty/loading state, footer key/text, and modal chrome. Names are semantic (`styleRepoStars`) not color-named.
- Migrated hand-rolled spinner (`spinnerFrame`, `spinnerTickMsg`, `spinnerTickCmd`) to `bubbles/spinner.Model` with `spinner.Line` (preserves `|/-\` look). Spinner ticks only while loading state is active.

**Shared geometry (P2)**
- Replaced three drifting width-math sites (one per layout branch + mouse handler) with a single `calcPaneGeometry(totalWidth, showPreview) paneGeometry` helper in `internal/tui/geometry.go`. Render layout and mouse hit-testing now agree on pane boundaries in both two- and three-pane modes.
- Mouse wheel now scrolls the pane under the pointer (by X coordinate) instead of the active pane.

**Header, footer, list pane (P3)**
- Header restyle: `styleAppTitle` ("gh star-lists", always shown), `styleSeparator` (" > "), `stylePaneTitle` (list name), `stylePaneSubtitle` ("[sort: X]"). Priority truncation: sort label drops first, list name truncated second, app name never truncated.
- Footer restyle: key tokens use `styleFooterKey` (bold), descriptions use `styleFooterText` (faint). Hints hide rather than wrap at narrow widths.
- List rows simplified: cursor + name + elastic space + right-aligned muted count. Age column and internal `|` separator removed.
- Search bar shows `N/total` count indicator right-aligned (hidden when too narrow to fit).

**Repo pane rebuild + eager initial load (P4, with post-ship refinements)**
- `renderRepoPane` rebuilt with field-level styled columns: cursor, optional `[x]`/`[ ]` selection marker, right-aligned stars + escaped `★` glyph (`styleRepoStars`), left-aligned language clamped [4,12] chars (`styleRepoLanguage`), `NameWithOwner` (default / bold-accent / faint by focus/activity), muted fork/archived badges.
- Pushed age ("last updated") removed entirely from the repo list — it cluttered the right edge without adding scanning value. No flag or config is provided to re-enable it. The preview pane still shows "Pushed:" for repos where it is relevant.
- Column widths (star field width, language field width) computed from currently visible rows after filter/scroll, not all repos.
- Narrow-width progressive hiding: badges (<55), language (<42), stars (<30). Repo name and cursor always survive; minimum 12 columns reserved for names.
- Contextual right-pane heading: accented list name followed by a blank spacer line. The "Repos in this list: N" count subtitle was removed — the count adds noise without aiding navigation.
- Eager initial load: `listsLoadedMsg` now auto-focuses the first sorted list, resets cursor/offset/selection, and triggers `loadReposCmd` immediately. The `(press enter to view repos)` placeholder is removed.

**Preview pane + interaction fixes (P5)**
- `renderPreviewPane` rebuilt as a styled detail block: `stylePaneTitle` for NameWithOwner, `styleRepoURL` for URL, metadata bar (stars + language + source/fork/archived badge), description section with `(no description)` fallback, license/pushed/starred/topics with `-` fallbacks. Nothing overflows the preview column width.
- `m.selected` cleared on every focused-list change path: eager load, Enter drill, double-click drill.
- `m.repoCursor` and `m.repoOffset` reset to 0 on every focused-list change.
- Double-click tracking: second click on the same list row within 300ms drills to repo pane. Single click sets focus/cursor only.
- TUI `OpenBrowser` callback now uses `io.Discard` for both stdout and stderr of the child process so browser noise cannot corrupt the alt-screen. Non-TUI `--web` path unchanged.

## Files changed

| File | Change |
|------|--------|
| `internal/tui/styles.go` | Full rewrite: 24 semantic styles + 3 backward-compat aliases (styleFaint, styleFooter, styleSelected) |
| `internal/tui/geometry.go` | New file: `paneGeometry` struct + `calcPaneGeometry` replacing three drifting inline width-math sites |
| `internal/tui/model.go` | Spinner migration; `renderHeader`, `renderFooter`, `renderListPane`, `renderRepoPane`, `renderPreviewPane` rebuilt; `listsLoadedMsg` eager load; double-click fields + logic; `repoPaneH()` helper; `starGlyph` constant; `truncateToWidth` helper; `renderHint`/`joinHints` footer helpers |
| `internal/tui/model_test.go` | ~25 new tests across all phases; 2 existing tests updated for eager-load behavior |
| `internal/command/run.go` | TUI `OpenBrowser` option wraps browser call with `io.Discard` stderr |

## Design notes

- **Backward-compat style aliases**: `styleFaint`, `styleFooter`, `styleSelected` kept as aliases during the render-function rewrites so each phase compiled independently. They can be inlined in a future cleanup pass.
- **Sequential phases despite single-file ownership**: all five implementation phases touched `internal/tui/model.go`. Parallel worktree phases would have produced merge conflicts; sequential subagents with per-phase exit criteria kept each change reviewable and the gate clean.
- **`repoPaneH()` subtracts heading rows from scroll window**: the 2-row contextual heading (list name + blank spacer) reduces the scrollable viewport. `slideRepoOffset` uses `repoPaneH()` to keep the cursor visible. If heading row count changes, update `repoPaneH()`.
- **Esc closes help overlay**: pressing Esc (`keys.Back`) when `m.showHelp` is true now closes the help panel instead of triggering the back/quit logic. The `?` key toggles it; Esc is the intuitive close. No footer hint needed — it matches standard modal conventions.
- **Star glyph as escaped literal**: `starGlyph = "★"` in Go source passes `make ascii-check`. One test (`TestPreviewDetailBlock`) initially used a raw `★` character — fixed to `"★"` during final verification.
- **Column widths from visible rows**: star-field width and language-field width are recomputed each render from the visible viewport slice (post-filter, post-scroll), not all repos. This keeps each viewport locally tidy without a separate pre-pass.
