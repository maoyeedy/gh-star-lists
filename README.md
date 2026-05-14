# gh-star-lists

GitHub CLI extension for reading GitHub Star Lists from the terminal. Use `gh starred` for the account-wide starred repository list; use this extension for Star Lists created in GitHub's UI.

The extension is read-only and uses your existing `gh` authentication. It does not store tokens.

## Install

```sh
gh extension install maoyeedy/gh-star-lists
gh star-lists --help
```

For local development:

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
gh star-lists [list] [--sort <KEY>] [--desc] [--plain | --json | --tsv]
gh star-lists repos <LIST_ID> [--sort <KEY>] [--desc] [--plain | --json | --tsv]
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

List repositories in one Star List:

```sh
gh star-lists repos UL_kwDOExample
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

## Output Fields

Star List JSON and TSV fields:

1. `name`
2. `description`
3. `lastAddedAt`
4. `id`

Repository JSON and TSV fields:

1. `nameWithOwner`
2. `description`
3. `isFork` (`yes` or `no` in TSV)
4. `stargazerCount`
5. `pushedAt`
6. `url`

Human output is the default and is optimized for reading. `--plain` prints the detailed text view. JSON uses lowerCamelCase fields. TSV preserves the field order above.

## Sorting

Results use GitHub's default order unless `--sort` is provided. Explicit sorts are applied locally after all pages are fetched, so every output mode uses the same order.

Star List sort keys:

- `added`
- `name`

Repository sort keys:

- `name`
- `stars`
- `pushed`

Explicit sorts are ascending by default. Add `--desc` to reverse the order.

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

# Repositories sorted by stars
gh star-lists repos "$LIST_ID" --sort stars --desc
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
