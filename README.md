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

Omit repo/list args in a TTY for interactive prompts. Add `--dry-run` to preview writes.

## Full Reference

```sh
gh star-lists --help
```

## Development

```sh
make check   # test + vet + build
```
