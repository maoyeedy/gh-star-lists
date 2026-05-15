# gh-star-lists

GitHub CLI extension for reading GitHub Star Lists from the terminal. Read-only; uses existing `gh` auth.

## Install

```sh
gh extension install maoyeedy/gh-star-lists
```

Requires a signed-in `gh` account with access to the Star Lists being read (`gh auth login`).

## Commands

```text
gh star-lists [list] [FLAGS]
gh star-lists repos <LIST_ID_OR_NAME> [FLAGS]
```

**Flags:** `--sort <KEY>` `--desc` `--limit <N>` `--filter <KEY:VALUE>` `--cache` `--output <FILE>` `--template <STR>` `--no-color` `--plain` `--json` `--tsv`

```sh
gh star-lists                                      # human table (default)
gh star-lists --sort name --desc --limit 10
gh star-lists repos "Game Dev"                     # by name (case-insensitive)
gh star-lists repos UL_kwDOExample --sort stars --desc
gh star-lists repos UL_kwDOExample --filter name:go --filter fork:false
```

Default human output:

```text
NAME           ADDED    ID
Game Dev       6d ago   UL_SAMPLE_GODOT
CLI Tools      1mo ago  UL_SAMPLE_CLI
```

**Sort keys — lists:** `added`, `name` | **repos:** `name`, `stars`, `pushed`

**Filter keys:** `name` (case-insensitive contains) | `fork` (repos only; `true`/`false`)

## Output

```sh
gh star-lists --json
gh star-lists --tsv
gh star-lists --plain                              # multiline detail view
gh star-lists --output stars.json
gh star-lists --template '{{range .}}{{.name}} {{.id}}\n{{end}}'
```

**JSON/TSV fields — lists:** `name`, `description`, `lastAddedAt`, `id`

**JSON/TSV fields — repos:** `nameWithOwner`, `description`, `isFork`, `stargazerCount`, `pushedAt`, `url`

`--cache` caches API responses in memory for 5 minutes — useful when chaining multiple calls.

## Recipes

```sh
# Star List names
gh star-lists --json | jq -r '.[].name'

# First Star List ID
LIST_ID="$(gh star-lists --tsv | awk -F '\t' 'NR == 1 { print $4 }')"

# Repo names and URLs
gh star-lists repos "$LIST_ID" --json | jq -r '.[] | "\(.nameWithOwner)\t\(.url)"'

# Top 5 repos by stars
gh star-lists repos "$LIST_ID" --sort stars --desc --limit 5

# Non-fork repos matching a keyword
gh star-lists repos "$LIST_ID" --filter name:golang --filter fork:false --json

# Star count via template
gh star-lists repos "$LIST_ID" --sort stars --desc --limit 10 \
  --template '{{range .}}{{.stargazerCount}} {{.nameWithOwner}}\n{{end}}'
```

## Development

```sh
make check   # goimports, vet, test, build
make lint    # golangci-lint
make smoke   # install as gh extension + smoke tests
```

```sh
pre-commit install   # enable pre-commit hooks (requires pre-commit)
```
