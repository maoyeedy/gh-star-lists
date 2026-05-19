# TUI v1.0 — Ship Record

## What shipped

Initial two-pane TUI browser (`gh star-lists browse`). Lists in left pane, repos in right pane after drilling in with Enter.

**Stack:** `charm.land/bubbletea/v2 v2.0.6`, `charm.land/bubbles/v2 v2.1.0`, `charm.land/lipgloss/v2 v2.0.3`

**CLI wiring:**
- `ActionBrowse` constant, `tui` alias in `commandAliases`, positional case in `Parse`
- Incompatible flags rejected at parse time with usage error
- `runTUI` swappable var + `RunTUIForTest` hook in `run.go`
- TTY guard: `!canPrompt()` → `ExitUsage`
- `--no-color` propagates via `tui.Options.NoColor` → `colorprofile.NoTTY`
- Help registered in `commandHelp`, `helpTextCompact`, `helpTextFull`, `UsageText`

**TUI behavior:**
- Two panes: lists (left) + repos for focused list (right)
- Loading, empty, and error states
- Sort cycling — lists: `github → name → repos → added`; repos: `github → name → stars → pushed`
- Drill into list (Enter), back to lists (Esc), quit (Esc from list pane, `q`)
- Open in browser via `Options.OpenBrowser` seam
- Refresh (`ctrl+r`): calls `Invalidate()` on cache service
- Help overlay (`?`)
- `j/k` vim cursor aliases

**Tests:** Model updates, sort, navigation, cursor clamping, browser, help overlay, `shortAge`, resize, error/loading render. Non-TTY → `ExitUsage`; TUI calls `runTUI`; `--no-color` propagated.

## Files changed

| File | Change |
|------|--------|
| `internal/command/parse.go` | `ActionBrowse`, `tui` alias, incompatible flag rejection |
| `internal/command/run.go` | `runTUI` hook, TTY guard, `--no-color` wiring |
| `internal/command/help*.go` | Help/usage text for browse command |
| `internal/tui/model.go` | NEW — Model, Init, Update, View, key handling |
| `internal/tui/keys.go` | NEW — key binding definitions |
| `internal/tui/styles.go` | NEW — lipgloss styles |
| `internal/tui/messages.go` | NEW — message types and commands |
| `internal/tui/render.go` | NEW — two-pane layout, sort bar, help overlay |
| `internal/tui/model_test.go` | NEW — 28 tests |
| `internal/githubapi/cache.go` | `Invalidate()` method added |

## Design notes

- `Model.Init()` returns `tea.Cmd` only (not `(tea.Model, tea.Cmd)`).
- `Model.View()` returns `tea.View` struct; set `v.AltScreen = true`.
- `tea.WithAltScreen()` does not exist in v2.0.6; AltScreen is set in `View()`.
- No-color: `tea.WithColorProfile(colorprofile.NoTTY)`. No `DefaultRenderer().SetColorProfile()`.
- Key handling: `case tea.KeyPressMsg:`. `key.Matches` is generic `[Key fmt.Stringer]`.
- Quit: `tea.Quit` is `func() tea.Msg` returning `tea.QuitMsg{}`.
