# TUI v1.0 — Ship record

**Shipped:** 2026-05-19

## What shipped

**Stack:** `charm.land/bubbletea/v2 v2.0.6`, `charm.land/bubbles/v2 v2.1.0`, `charm.land/lipgloss/v2 v2.0.3`

**Entry point:** `gh star-lists browse` (alias `tui`). Bare `gh star-lists` unchanged.

**CLI wiring:**
- `ActionBrowse` constant, `tui` alias in `commandAliases`, positional case in `Parse`.
- Incompatible flags (`--json`, `--tsv`, `--fzf`, `--template`, `--jq`, `--output`, `--web`,
  `--unlisted`, `--all`, `--search`, `--limit`, `--filter`, `--sort`, `--desc`) rejected at parse
  time with a usage error.
- `runTUI` swappable var + `RunTUIForTest` hook in `run.go` (matches `openBrowser` / `canPrompt` pattern).
- TTY guard: `!canPrompt()` → `ExitUsage` with clear message.
- `--no-color` propagates via `tui.Options.NoColor` → `tea.WithColorProfile(colorprofile.NoTTY)`.
- Help registered in `commandHelp`, `helpTextCompact`, `helpTextFull`, `UsageText`.

**TUI behavior (`internal/tui/`):**
- Two panes: lists (left) + repos for focused list (right).
- Loading, empty, and error states.
- Sort cycling — lists: `github → name → repos → added`; repos: `github → name → stars → pushed`.
- Drill into list (Enter), back to lists (Esc), quit (Esc from list pane, `q`).
- Open in browser: `o` or Enter on focused repo, via `Options.OpenBrowser` seam.
- Refresh (`ctrl+r`): calls `Invalidate()` on cache service via `interface{ Invalidate() }` type assertion.
- `cacheService.Invalidate()` added to `internal/githubapi/cache.go`.
- Help overlay (`?` to toggle).
- `j/k` vim cursor aliases.

**Tests:**
- `internal/tui/model_test.go`: model updates, sort, navigation, cursor clamping, browser, help overlay, `shortAge`, window resize, error/loading render.
- `internal/command/run_test.go`: browse on non-TTY → `ExitUsage`; `tui` alias non-TTY; TTY calls `runTUI`; TUI error → `ExitFailure`; `browse --help`; `--json` rejected; extra positional rejected; `--no-color` propagated.

## Bubbletea v2 API notes (for future implementors)

- `Model.Init()` returns `tea.Cmd` only (not `(tea.Model, tea.Cmd)`).
- `Model.View()` returns `tea.View` struct; set `v.AltScreen = true` on the returned value.
- `tea.WithAltScreen()` does not exist; AltScreen is set in `View()`.
- No-color: `tea.WithColorProfile(colorprofile.NoTTY)` passed to `tea.NewProgram`. No `DefaultRenderer().SetColorProfile()`.
- Key handling: `case tea.KeyPressMsg:`. `key.Matches` is generic `[Key fmt.Stringer]`.
- Quit: `tea.Quit` is `func() tea.Msg` returning `tea.QuitMsg{}`.

## Deliberately deferred from v1.0

These were considered and explicitly excluded to keep v1.0 shippable in one session.

| Deferred | Reason | Planned in |
|---|---|---|
| All mutations (`n/e/d/a/x/m/u/c/C`) | Each needs a Bubble Tea modal + confirmation flow; too large for v1.0 | v1.1 |
| Detail preview pane | Requires layout change + `WithTopics: true` fetch guard | v1.1 |
| Repo sort: `language`, `starred` | Minor; fzf parity but not blocking | v1.1 |
| Status toasts | Depends on mutation system | v1.1 |
| Fuzzy search (`/`) | Requires extracting `internal/search/` shared package | v1.2 |
| Multi-select (`space`) | Depends on mutations being wired | v1.2 |
| Mouse support | Low priority; keyboard-first | v1.2 |
| Viewport scrolling | Requires swapping manual row render for `bubbles/viewport` | v1.2 |
| YAML config / themes | No config system yet | v2 |
| Disk cache | Not needed until sessions are slow in practice | v2 |
| Speculative prefetch | Premature optimization | v2 |
| Bare `gh star-lists` opens TUI by default | Deferred until TUI is validated in real use | v2 |
| `--no-tui` escape hatch | Only needed alongside the default switch | v2 |
| Multi-account / `--host` switching in TUI | Out of scope for TUI | — |
| Mouse support for terminal text selection | Fundamental conflict; document, not fix | — |
