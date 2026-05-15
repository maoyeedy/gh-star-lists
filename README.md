# gh-star-lists

Explore your GitHub Star Lists from terminal.

## Install

```sh
gh extension install maoyeedy/gh-star-lists
```

## Commands

```text
gh star-lists [list] [FLAGS]
gh star-lists repos <LIST_ID_OR_NAME> [FLAGS]
```

**Flags:** `--sort <KEY>` `--desc` `--limit <N>` `--filter <KEY:VALUE>` `--cache` `--output <FILE>` `--template <STR>` `--no-color` `--web` `--unlisted` `--plain` `--json` `--tsv`

```sh
gh star-lists                                      # human table (default)
gh star-lists --sort name --desc --limit 10
gh star-lists list --sort repos --desc             # sort by repo count
gh star-lists repos "Game Dev"                     # by name (case-insensitive)
gh star-lists repos UL_kwDOExample --sort stars --desc
gh star-lists repos UL_kwDOExample --filter name:go --filter fork:false
gh star-lists repos UL_kwDOExample --filter language:Go
gh star-lists repos UL_kwDOExample --web           # open in browser
gh star-lists repos --unlisted --sort starred      # starred not in any list
```

Default human output:

```text
NAME           ADDED    ID
Game Dev       6d ago   UL_SAMPLE_GODOT
CLI Tools      1mo ago  UL_SAMPLE_CLI
```

**Sort keys — lists:** `added`, `name`, `repos` | **repos:** `name`, `stars`, `pushed`, `language`, `starred`

**Filter keys:** `name` (case-insensitive contains) | `fork` (repos only; `true`/`false`) | `language` (repos only; case-insensitive)

## Output

```sh
gh star-lists --json
gh star-lists --tsv
gh star-lists --plain                              # multiline detail view
gh star-lists --output stars.json
gh star-lists --template '{{range .}}{{.name}} {{.id}}\n{{end}}'
```

**JSON/TSV fields — lists:** `name`, `description`, `lastAddedAt`, `id`

**JSON/TSV fields — repos:** `nameWithOwner`, `description`, `isFork`, `stargazerCount`, `pushedAt`, `url`, `language`, `starredAt`

`--cache` cache API responses in memory 5 min. Useful for chaining multiple calls. Recommended with `--unlisted`.

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

# Repos by language
gh star-lists repos "$LIST_ID" --filter language:Go --sort language

# Star count via template
gh star-lists repos "$LIST_ID" --sort stars --desc --limit 10 \
  --template '{{range .}}{{.stargazerCount}} {{.nameWithOwner}}\n{{end}}'

# Unlisted starred repos (not in any list)
gh star-lists repos --unlisted --sort starred --desc

# Open in browser
gh star-lists repos "$LIST_ID" --web
```

## Development

```sh
make check   # scripts/check.sh: test, vet, build
make lint    # go vet
make smoke   # scripts/smoke-local.sh
scripts/smoke-gh-extension.sh
```

```sh
pre-commit install   # enable pre-commit hooks (requires pre-commit)
```
