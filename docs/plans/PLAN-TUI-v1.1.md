# TUI v1.1 -- Ship record

**Shipped:** 2026-05-19

## What shipped

**Prerequisite:** TUI v1.0 (browse-only two-pane browser, `internal/tui/` exists).

### All 9 mutations wired

| Key | Action | Modal type |
|---|---|---|
| `n` | Create list | Multi-field form: name, description, public/private toggle |
| `e` | Edit list | Same form, prefilled name+description; visibility unset (no `IsPrivate` on `StarList`) |
| `d` | Delete list | Confirm: type list name; Enter submits only when match |
| `a` | Add focused repo to another list | List picker: all lists |
| `x` | Remove focused repo from current list | y/N confirm |
| `m` | Move focused repo to another list | List picker: excludes current |
| `u` | Unstar focused repo | Confirm: type `NameWithOwner` |
| `c` | Copy current list contents to another | List picker: excludes source |
| `C` | Merge current list into another (destructive) | List picker + auto-delete source |

All modal keys enforce pane guards: `n/e/d/c/C` no-op in repo pane; `a/x/m/u` no-op in list pane.

**Destructive confirmations** (`d`, `u`) require the user to type the exact name before submitting; Enter only fires when `textinput.Value() == expected`.

**Bulk copy/merge** (`c`, `C`) use `golang.org/x/sync/errgroup` with `SetLimit(5)`, mirroring `runListCopy` in `command/run.go`. Each repo's current membership set is fetched individually inside the fan-out.

### Extended repo sort

Sort cycle extended from 4 to 6 modes: `github -> name -> stars -> pushed -> language -> starred`.

- `sortReposLanguage`: case-insensitive ascending; empty language sorts last; tiebreak `NameWithOwner`.
- `sortReposStarredAt`: descending (newest first); empty `StarredAt` sorts last (only populated by `ListStarredRepositories`; list-scoped repos display "-").

### Detail preview pane

`p` toggles a third column (lists | repos | preview) rendering:
name, URL, description, language, license, archived/fork flags, pushed-at, starred-at, topics.

Topics require `WithTopics: true` on `ListRepositories` -- passed only when `showPreview == true`. Toggling preview on in repo pane re-dispatches `loadReposCmd(..., withTopics: true)`.

### Status toast system

`mutationDoneMsg{err: nil}` sets `statusMsg = "Done."` for 2 seconds, displayed in `renderFooter` via `styleSuccess`. `tea.Tick` fires `statusExpiredMsg` to clear.

### Footer hints

Footer now shows pane-specific mutation hints:
- **List pane:** `n:new  e:edit  d:del  c:copy  C:merge  enter:select ...`
- **Repo pane:** `a:add  x:remove  m:move  u:unstar  p:preview  enter/o:open ...`

## Architecture notes

### Modal overlay

`modal` struct in `internal/tui/modal.go`. When `model.modal != nil`:
- `Update` routes `tea.KeyPressMsg` to `modal.update(msg)` instead of `handleKey`.
- `renderContent` calls `lipgloss.Place(width, height, Center, Center, box)` replacing the base layout. lipgloss v2 has no z-order primitive; "replace" is the conventional approach.
- Esc always cancels and returns `(nil, nil)`.

### Mutation commands

All commands in `internal/tui/messages.go`. Shape: `func xyzCmd(ctx, svc, ...) tea.Cmd` returning a closure that calls the service and returns `mutationDoneMsg{kind, err}`.

Repo membership set-diff logic replicates `runRepoListMutation` (`command/run.go:631-699`):
`GetRepositoryMemberships` -> build `map[string]struct{}` -> mutate -> `slices.Sorted(maps.Keys(next))` -> `UpdateRepositoryLists`.

The TUI never imports `internal/command`.

### Cache invalidation

Do NOT call `Invalidate()` after mutations. `cacheService` auto-invalidates inside each write method (`cache.go:180,192,201,216,224,232`). After `mutationDoneMsg` success, `model.Update` dispatches `loadListsCmd` (and `loadReposCmd` when in repo pane) to re-fetch fresh data.

### Design decisions made at implementation time

- `StarList` has no `IsPrivate` field (write-only) -- edit modal prefills name+description only; visibility toggle starts unset; `Private *bool` is nil unless user picks.
- Copy/merge: errgroup with limit 5 (mirrors CLI), no typed-name confirm for `C` (Enter on picker is the confirmation).
- Modal overlay: replace-mode (not z-order splice).

## Files created / modified

- `internal/tui/modal.go` -- NEW; `modalKind` enum, `modal` struct, all constructors and Update/View methods
- `internal/tui/model.go` -- added fields (`modal`, `statusMsg`, `statusExpiry`, `showPreview`), message routing, key handlers, three-column layout, preview render, footer mutation hints
- `internal/tui/messages.go` -- `mutationDoneMsg`, `statusExpiredMsg`, `statusClearCmd`, all 9 mutation commands + `loadReposCmd` extended with `withTopics bool`
- `internal/tui/keys.go` -- 10 new bindings (n/e/d/a/x/m/u/c/C/p)
- `internal/tui/styles.go` -- `styleSuccess`, `styleModalBorder`, `styleModalTitle`
- `internal/tui/model_test.go` -- 56 tests total (v1.0: 28, v1.1 additions: 28); `recordingFakeService`, `repoMutationFakeService`, `copyMergeFakeService`, `topicTrackingService`

## Deferred from v1.1

| Deferred | Reason | Planned in |
|---|---|---|
| Typed-name confirm for merge (C) | Two-step picker+confirm modal not implemented | v1.2 |
| Fuzzy search (/) | Requires extracting internal/search/ shared package | v1.2 |
| Multi-select (space) | Depends on mutations being mature | v1.2 |
| Mouse support | Low priority; keyboard-first | v1.2 |
| Viewport scrolling | Requires bubbles/viewport swap | v1.2 |
| YAML config / themes | No config system yet | v2 |
| Bare `gh star-lists` opens TUI by default | Deferred until TUI validated in real use | v2 |
