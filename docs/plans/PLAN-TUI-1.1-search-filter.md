# TUI v1.1 — Ship Record

## What shipped

All 9 mutations, extended repo sort, detail preview pane, status toasts, pane-specific footer hints.

**Mutations:**

| Key | Action | Modal type |
|-----|--------|------------|
| `n` | Create list | Multi-field form (name, description, public/private) |
| `e` | Edit list | Prefilled form |
| `d` | Delete list | Type-to-confirm |
| `a` | Add repo to list | List picker (all lists) |
| `x` | Remove repo from list | y/N confirm |
| `m` | Move repo to list | List picker (excludes current) |
| `u` | Unstar repo | Type-to-confirm |
| `c` | Copy list to another | List picker (excludes source) |
| `C` | Merge list into another (destructive) | List picker + auto-delete source |

All modal keys enforce pane guards. Destructive actions (`d`, `u`) require typing the exact name. Bulk copy/merge use `errgroup` with `SetLimit(5)`.

**Extended repo sort:** Sort cycle extended to 6 modes: `github → name → stars → pushed → language → starred`. Added `sortReposLanguage` (case-insensitive, empty last) and `sortReposStarredAt` (newest first).

**Detail preview pane:** `p` toggles a third column (lists | repos | preview) showing name, URL, description, language, license, archived/fork flags, activity dates, topics. Topics fetched on-demand with `WithTopics: true` passed only when preview is active.

**Status toasts:** `mutationDoneMsg` sets a 2-second styled toast in `renderFooter` via `tea.Tick` → `statusExpiredMsg`.

**Footer hints:** Pane-specific key hints shown in footer (list pane: *n/e/d/c/C/enter...*; repo pane: *a/x/m/u/p/enter/o...*).

## Files changed

| File | Change |
|------|--------|
| `internal/tui/modal.go` | NEW — `modalKind` enum, `modal` struct, constructors, Update/View |
| `internal/tui/model.go` | Added `modal`, `statusMsg`, `statusExpiry`, `showPreview`; 3-column layout, preview render |
| `internal/tui/messages.go` | `mutationDoneMsg`, `statusExpiredMsg`, 9 mutation cmds + `loadReposCmd` extended with `withTopics` |
| `internal/tui/keys.go` | 10 new bindings (n/e/d/a/x/m/u/c/C/p) |
| `internal/tui/styles.go` | `styleSuccess`, `styleModalBorder`, `styleModalTitle` |
| `internal/tui/model_test.go` | 56 tests total (28 new); `recordingFakeService`, `repoMutationFakeService`, `copyMergeFakeService`, `topicTrackingService` |

## Design notes

- **Modal overlay:** Replace-mode (`lipgloss.Place` over base layout), not z-order. Esc always cancels and returns `(nil, nil)`.
- **Mutation commands:** Shape is `func xyzCmd(ctx, svc, ...) tea.Cmd` returning closure → service call → `mutationDoneMsg{kind, err}`. TUI never imports `internal/command`.
- **Cache invalidation:** Do NOT call `Invalidate()` after mutations. `cacheService` auto-invalidates inside each write method. After `mutationDoneMsg`, `Update` dispatches `loadListsCmd` / `loadReposCmd`.
- `StarList` has no `IsPrivate` field (write-only). Visibility toggle starts unset; `Private *bool` is nil unless user picks.
- Copy/merge: errgroup with limit 5, no typed-name confirm for `C`.
