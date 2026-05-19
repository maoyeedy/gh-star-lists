# TUI v1.3 — Ship Record

## What shipped

Narrow-layout responsiveness, active/inactive cursors, pane-local loading spinner, mouse support (`--mouse`), compact footer, rebuilt help overlay, bulk-failure toast.

**Plumbing:**
- `--mouse` opt-in flag → `tui.Options.Mouse` → `v.MouseMode = tea.MouseModeCellMotion`
- `bulkDoneMsg.failedNWOs []string` — all bulk commands collect failures

**Render layer:**
- Active/inactive cursors: `styleCursorActive` (bold cyan) / `styleCursorInactive` (faint)
- List pane: bracket blob replaced with right-aligned columns `padLeft(N,4) + " | " + padLeft(age,8)`
- Repo pane: metadata hidden below 60-col threshold; 8-byte language clip removed
- Search bar: left-ellipsis truncation; empty-result hint shows query
- Footer: collapsed to core hints only
- Help overlay: two-column table (Navigation | Actions); single-column below 50 cols
- Bulk failure toast: up to 3 failed NWOs by name, `"+N more"` truncation
- Pane-local loading spinner: inline rotating frame (`| / - \`) in loading pane only; surround layout visible

**Behavior:**
- Esc preserves repos (no longer clears `m.repos`/`m.focusedList`/cursor/offset)
- `Left`/`Right` for explicit pane focus
- Mouse click hit-tests pane bounds, sets focus and cursor
- Mouse wheel scrolls active pane

**Tests:** `TestDropLastRuneMultiByte`, `TestSearchWhileFilterActiveActionKeys`, `TestNarrowRepoPaneHidesMetadata`, `TestFooterCoreHintsOnly`, `TestLoadingRendersInsidePane`, `TestHelpOverlayContainsV12Keys`, `TestMouseClickFocusesPane`, `TestParseMouseFlag`.

## Files changed

| File | Change |
|------|--------|
| `internal/command/parse.go` | `--mouse` flag, `tui.Options.Mouse` wiring |
| `internal/command/run.go` | Mouse option propagation |
| `internal/tui/keys.go` | `Left`, `Right` key bindings |
| `internal/tui/model.go` | Esc preserve, Left/Right focus, mouse handlers, `failedNWOs` |
| `internal/tui/messages.go` | `bulkDoneMsg.failedNWOs` field added |
| `internal/tui/styles.go` | `styleCursorActive`, `styleCursorInactive`, `padLeft` helper |
| `internal/tui/model_test.go` | 8 new tests |

## Design notes

- **Mouse API:** `WithMouseCellMotion()` as a `ProgramOption` does not exist in v2.0.6. Set per-frame in `View()` via `v.MouseMode = tea.MouseModeCellMotion`. Mouse enabled/disabled every frame — acceptable for now.
- **Spinner:** Manual `spinnerFrame int` + `tea.Tick` rather than `bubbles/spinner.Model`. Simpler for v1.3 scope. Must migrate to `spinner.Model` before adding full async spinner system.
- **Selection not cleared on pane switch:** `m.selected` survives Left/Right transitions. Only explicit Esc-with-selection clears it. Drilling into a different list while selection is live leaves stale references.
- **Shared search query:** A single `m.searchQuery` filters both panes. No independent per-pane search history.
- **`%-8s` language format:** B3 removed the 8-rune clip, but `"%-8s %6s* %s"` still overflows for "TypeScript"/"JavaScript". Gap enforcement (`available := w - metaW - 2`) prevents layout breakage but looks uneven.
