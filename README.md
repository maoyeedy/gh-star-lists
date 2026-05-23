# gh-star-lists

Explore, sort, filter, and manage GitHub Star Lists from the terminal. Uses your `gh` auth — no tokens stored.

## Install

```sh
gh extension install maoyeedy/gh-star-lists
```

Requires `gh` authenticated with Star Lists enabled.

## Quick Start

```sh
gh star-lists                          # list all star lists
gh star-lists repos "Game Dev"         # repos in a list (name or ID)
gh star-lists repos --unlisted         # starred not in any list
gh star-lists repos --all --sort starred --desc   # all starred, newest first
gh star-lists create "Tools" -D "CLI tools"       # create a list
gh star-lists add cli/cli --to "Tools"            # add a repo to a list
```

## Development

- **Go 1.26** with `github.com/cli/go-gh/v2` (GraphQL client, auth)
- TUI: `charm.land/bubbletea/v2` + `bubbles/v2` + `lipgloss/v2`
- Output: JSON/TSV/plain/template/fzf via `internal/format`
- Other: in-memory cache, fuzzy search, retry with backoff
- CI: golangci-lint, pre-commit, `gh-extension-precompile` for releases
- Packages: `command` (parse/dispatch) → `githubapi` (sole API boundary) → `format`/`tui` (output)
