# gh-star-lists

GitHub CLI extension for reading GitHub Star Lists from the terminal. Read-only; uses existing `gh` authentication. Does not store tokens.

## Install

```sh
gh extension install maoyeedy/gh-star-lists
gh star-lists --help
```

Local development:

```sh
gh extension install . --force
gh star-lists --help
```

## Authentication

```sh
gh auth status
gh auth login
```

Query commands require a signed-in GitHub CLI account with access to the Star Lists being read.

## Commands

```text
gh star-lists [list] [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--no-color] [--plain | --json | --tsv]
gh star-lists repos <LIST_ID_OR_NAME> [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--no-color] [--plain | --json | --tsv]
gh star-lists --help
```

`list` is the default command:

```sh
gh star-lists
gh star-lists --plain
gh star-lists --json
gh star-lists --tsv
gh star-lists --sort name
gh star-lists --sort added --desc
```

List repositories in a Star List by ID or case-insensitive name:

```sh
gh star-lists repos UL_kwDOExample
gh star-lists repos "Game Dev"
gh star-lists repos UL_kwDOExample --plain
gh star-lists repos UL_kwDOExample --json
gh star-lists repos UL_kwDOExample --tsv
gh star-lists repos UL_kwDOExample --sort stars --desc
```

Default human output is a compact table:

```text
NAME           ADDED   ID
Game Dev       6d ago  UL_SAMPLE_GODOT
CLI Tools      1mo ago UL_SAMPLE_CLI
AI             1mo ago UL_SAMPLE_AI
```

Use `--plain` for the detailed multiline view:

```text
Game Dev
  Description: Game engine resources
  Last added: 2026-05-08T03:14:54Z
  ID: UL_SAMPLE_GODOT
```

## Sorting

Results use GitHub's default order unless `--sort` is provided. Explicit sorts are applied locally after all pages are fetched, so every output mode uses the same order.

```sh
# Single key
gh star-lists --sort name
gh star-lists repos UL_kwDOExample --sort stars --desc

# Multi-key: comma-separated or repeat --sort
gh star-lists --sort added,name
gh star-lists repos UL_kwDOExample --sort stars --sort name

# Reverse order
gh star-lists --sort name --desc
```

Star List sort keys: `added`, `name`
Repository sort keys: `name`, `stars`, `pushed`

## Filtering

Filter results after fetching. Repeat `--filter` for AND logic.

```sh
# Name contains (case-insensitive)
gh star-lists --filter name:go
gh star-lists repos UL_kwDOExample --filter name:cli

# Fork status (repos only)
gh star-lists repos UL_kwDOExample --filter fork:false
gh star-lists repos UL_kwDOExample --filter fork:true

# Combined filters
gh star-lists repos UL_kwDOExample --filter name:go --filter fork:false
```

Filter keys: `name` (case-insensitive contains), `fork` (repos only; `true`/`false`).

## Limiting

```sh
# Top N results after sort
gh star-lists --sort stars --desc --limit 10
gh star-lists repos UL_kwDOExample --sort pushed --desc --limit 5
```

## Output control

```sh
# Write to file
gh star-lists --output stars.json

# Go template (data model is JSON)
gh star-lists --template '{{range .}}{{.name}} {{.id}}\n{{end}}'
gh star-lists repos UL_kwDOExample --template '{{range .}}{{.nameWithOwner}}\t{{.stargazerCount}}\n{{end}}'

# Disable ANSI color
gh star-lists --no-color
```

## Caching

```sh
gh star-lists --cache
```

Caches API responses in memory for 5 minutes. Useful when chaining multiple `gh star-lists` calls — the second call reuses the first call's cached data.

## Output Fields

Star List JSON and TSV fields: `name`, `description`, `lastAddedAt`, `id`

Repository JSON and TSV fields: `nameWithOwner`, `description`, `isFork` (`yes`/`no` in TSV), `stargazerCount`, `pushedAt`, `url`

JSON uses lowerCamelCase fields. TSV preserves the field order above.

## Recipes

```sh
# Star List names
gh star-lists --json | jq -r '.[].name'

# Star List names and IDs
gh star-lists --tsv | cut -f1,4

# First Star List ID
LIST_ID="$(gh star-lists --tsv | awk -F '\t' 'NR == 1 { print $4 }')"

# Repository names and URLs
gh star-lists repos "$LIST_ID" --json | jq -r '.[] | "\(.nameWithOwner)\t\(.url)"'

# Top 5 repos by stars
gh star-lists repos "$LIST_ID" --sort stars --desc --limit 5

# Non-fork repos matching a keyword
gh star-lists repos "$LIST_ID" --filter name:golang --filter fork:false --json

# Repo name and star count via template
gh star-lists repos "$LIST_ID" --sort stars --desc --limit 10 --template '{{range .}}{{.stargazerCount}} {{.nameWithOwner}}\n{{end}}'
```

## Development

```sh
go test ./...
go vet ./...
go build
```

Pre-release smoke check:

```sh
bash scripts/smoke-local.sh
```

If a local extension already owns `gh star-lists`, replace it deliberately:

```sh
GH_STAR_LISTS_REPLACE_EXTENSION=1 bash scripts/smoke-local.sh
```

Optional live checks:

```sh
gh auth status
gh star-lists
gh star-lists --json | jq 'length'
gh star-lists --tsv
gh star-lists repos <LIST_ID>
gh star-lists repos <LIST_ID> --json | jq 'length'
gh star-lists repos INVALID_LIST_ID >/tmp/gh-star-lists-out.txt
```

## Release

Releases are tag-driven. Push a `v*` tag and wait for the Release workflow to upload precompiled assets through `cli/gh-extension-precompile@v2`.

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
gh run list -R maoyeedy/gh-star-lists --limit 5
gh release view vX.Y.Z -R maoyeedy/gh-star-lists
gh extension install maoyeedy/gh-star-lists
gh star-lists --help
```

The first install attempt can fail if the release workflow is still running.

## Troubleshooting

```sh
gh extension list
gh extension remove star-lists
gh extension install maoyeedy/gh-star-lists
```

- Auth errors: run `gh auth status`, then `gh auth login` if needed.
- Inaccessible list IDs can mean deleted, private, wrong-account, or non-Star-List IDs.
- Empty output can be valid when the account has no Star Lists or the selected list has no repositories.
