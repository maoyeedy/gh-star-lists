# Plan: TUI v1.1 — Mutations + Preview + Sort parity

## Goal

Close the gap between the TUI and `examples/fzf-browse.sh`. After v1.1 the TUI
should be a complete replacement for the fzf script for daily use.

**Prerequisite:** TUI v1 shipped (browse-only, `internal/tui/` exists).

## Scope

| In | Out |
|---|---|
| All 9 mutation actions via Bubble Tea modal | Mouse support |
| Detail preview pane (right side, toggle) | Multi-select / bulk ops |
| Repo sort: add `language` + `starred` to cycle | Fuzzy search |
| Status toasts (post-mutation, ~2s auto-dismiss) | YAML config / themes |
| "coming soon" placeholder keys → real handlers | Changing default bare-command behavior |

## Mutations (9 actions)

Wire through `githubapi.Service` only. Never call GraphQL from TUI.

For repo mutations (`add`, `remove`, `move`, `unstar`), resolving the repo ID
requires `service.GetRepositoryMemberships`. The membership map is already used
by `command.runRepoListMutation` — replicate the same pre-fetch pattern.

| Key | Action | Modal type |
|---|---|---|
| `n` | Create list | Multi-field form: name, description, public/private toggle |
| `e` | Edit list | Same form, prefilled; Esc cancels without save |
| `d` | Delete list | Confirm: type list name to confirm, Enter submits |
| `a` | Add focused repo to another list | List picker: shows all lists, marks current membership |
| `x` | Remove focused repo from current list | Confirm: "remove repo from list? y/N" |
| `m` | Move focused repo to another list | List picker (excludes current list) |
| `u` | Unstar focused repo | Confirm: type repo name to confirm |
| `c` | Copy current list contents into another | List picker |
| `C` | Merge current list into another (destructive) | List picker + confirm |

All modal keys must be no-ops in the opposite pane (e.g., `n/e/d/c/C` do
nothing in repo pane; `a/x/m/u` do nothing in list pane).

**Destructive confirmations** (`d`, `x`, `u`, `C`) require the user to type
the exact name before submitting. Bubble Tea `textinput` for the typed value.

**Non-destructive pickers** (`a`, `m`, `c`) use an inline scrollable list
overlay. Arrow keys move; Enter confirms; Esc cancels.

## Modal architecture

Add `internal/tui/modal.go`:

```go
type modalKind int
const (
    modalNone modalKind = iota
    modalCreateList
    modalEditList
    modalDeleteList
    modalPickList      // used by add, move, copy, merge
    modalConfirmText   // typed-name confirmation (delete, unstar, merge)
    modalConfirmYesNo  // simple y/N (remove)
)

type modal struct {
    kind     modalKind
    title    string
    // for form modals (create/edit)
    fields   []textinput.Model
    focused  int
    // for list-picker modals
    choices  []githubapi.StarList
    cursor   int
    // for confirm-text modals
    input    textinput.Model
    expected string   // value user must type
    // for yes/no confirms
    confirmed bool
    // action to dispatch after modal confirm
    onConfirm func(m model) (model, tea.Cmd)
}
```

The root model gains a `modal *modal` field. When `modal != nil`:
- `View()` renders the modal overlay on top of the layout.
- `Update` routes key events to `modal.Update`, not `handleKey`.
- Esc always cancels the modal and returns to browse.
- Mutation completion sets `modal = nil`, triggers pane refresh, sets a
  status toast.

Use `charm.land/bubbles/v2/textinput` for text fields.

## Status toast system

Add `statusMsg` field (`string`) and `statusExpiry` field (`time.Time`) to
the root model. After a mutation:
```go
m.statusMsg = "List deleted."
m.statusExpiry = time.Now().Add(2 * time.Second)
```
In `renderFooter`, if `statusMsg != ""` and not expired, render it in green
instead of the normal key hints. On expiry, clear via `tea.Tick`:
```go
func statusClearCmd(expiry time.Time) tea.Cmd {
    return tea.Tick(time.Until(expiry)+10*time.Millisecond, func(time.Time) tea.Msg {
        return statusExpiredMsg{}
    })
}
```

## Detail preview pane

Add `showPreview bool` to the root model, toggled by `p`. When on:
- Three-column layout: lists | repos | preview.
- Preview pane renders: name, URL, description, language, license, archived,
  fork, pushed-at, starred-at, topics.
- `Topics` requires `ListRepositories(ctx, id, ListOptions{WithTopics: true})`.
  Pass `WithTopics: true` only when preview is visible (`showPreview == true`).
  Add `showPreview` to `loadReposCmd` so it selects the right options.
- Add `p` to keymap and footer hints.

Preview renders as a vertical card, no box-drawing (ASCII `|` separator only).

## Extended repo sort

Extend `sortReposKey` iota:
```go
const (
    sortReposGitHub sortReposKey = iota
    sortReposName
    sortReposStars
    sortReposPushed
    sortReposLanguage   // NEW
    sortReposStarredAt  // NEW
)
```
Cycle: `github -> name -> stars -> pushed -> language -> starred -> github`
(6 modes, `% 6`). Matches `fzf-browse.sh` exactly.

`sortReposLanguage`: sort by `Language` ascending, case-insensitive; empty
language sorts last. Tiebreak: `NameWithOwner`.

`sortReposStarredAt`: sort by `StarredAt` descending; empty `StarredAt` sorts
last. `StarredAt` is only populated by `ListStarredRepositories`, not
`ListRepositories` — display "-" for empty values; the sort still works
(empty strings sort consistently).

## Files to create / modify

- `internal/tui/modal.go` — new; modal struct, Update, View for all modal types.
- `internal/tui/model.go` — add `modal`, `statusMsg`, `statusExpiry`,
  `showPreview` fields; new key handlers (`n/e/d/a/x/m/u/c/C/p`); wire
  modal render into `View`; three-column layout when preview active.
- `internal/tui/messages.go` — add mutation result messages (`mutationDoneMsg`,
  `statusExpiredMsg`), mutation commands (`createListCmd`, `deleteListCmd`, etc.).
- `internal/tui/sort.go` — add `sortReposLanguage`, `sortReposStarredAt` cases.
- `internal/tui/keys.go` — add bindings for `n/e/d/a/x/m/u/c/C/p`.
- `internal/tui/styles.go` — add `styleSuccess`, `styleModal*` styles.

## Test additions

- Modal open/cancel for each type.
- Confirm-text: wrong input doesn't submit; correct input submits and triggers
  mutation command.
- List-picker: arrow navigation, Enter selects, Esc cancels.
- Mutation command success → pane refreshed, status set.
- Mutation command failure → error banner shown, modal closed.
- Sort extended: `language` and `starred` modes sort correctly.
- Preview pane: `WithTopics: true` passed when `showPreview == true`.

## Verification

```
make check
make ascii-check
# Interactive: create a list, edit it, add a repo, move it, delete the list.
# Confirm each pane refreshes after each mutation.
# Confirm Esc cancels mid-modal without side effects.
```
