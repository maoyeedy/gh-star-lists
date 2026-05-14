# gh-star-lists

`gh-star-lists` is a GitHub CLI extension for reading GitHub Star Lists from the command line. It fills the gap between GitHub's built-in starred-repository support and Star Lists: use `gh starred` for plain starred repositories, and use this GraphQL-backed extension when you need the lists you created in GitHub's Star Lists UI.

v0.1 is read-only. It lists your Star Lists and the repositories inside a Star List; it does not modify Star Lists or repositories.

## Install

Install the released extension with GitHub CLI:

```sh
gh extension install maoyeedy/gh-star-lists
```

For local development from a checkout, install the current directory instead:

```sh
gh extension install . --force
```

Then verify the extension entrypoint is available:

```sh
gh star-lists --help
```

## Authentication and scope

`gh-star-lists` uses your existing `gh` authentication context. Sign in with `gh auth login` before running commands that query GitHub.

The extension does not store tokens. Authentication is delegated to GitHub CLI, and v0.1 only performs read-only GraphQL queries for Star Lists and repositories in those lists.

## Commands

```text
gh star-lists [list] [--json | --tsv]
gh star-lists repos <LIST_ID> [--json | --tsv]
gh star-lists --help
```

Common examples:

```sh
# List Star Lists in the default human-readable format.
gh star-lists

# Print Star List IDs for scripting.
gh star-lists --json | jq -r '.[].id'

# Use the first TSV row's ID to inspect that list's repositories.
list_id="$(gh star-lists --tsv | awk -F '\t' 'NR == 1 { print $4 }')"
gh star-lists repos "$list_id" --json | jq -r '.[].nameWithOwner'

# Export repositories in a spreadsheet-friendly format.
gh star-lists repos "$list_id" --tsv > star-list-repos.tsv
```

## Recipes

Use JSON or TSV output with standard shell tools for local filtering and sorting:

```sh
# Print Star List names only.
gh star-lists --json | jq -r '.[].name'

# Print Star List names and IDs.
gh star-lists --tsv | cut -f1,4

# Print repository URLs from one Star List.
gh star-lists repos "$LIST_ID" --json | jq -r '.[].url'

# Sort repositories by star count, highest first.
gh star-lists repos "$LIST_ID" --tsv | sort -k4,4nr
```

This extension reads GitHub **Star Lists** only. For the account-wide starred repository list, prefer GitHub CLI's built-in `gh starred` command.

### `gh star-lists [list]`

Lists your Star Lists. `list` is optional because it is the default command.

Human-readable output is the default:

```text
Learning
  Description: Repositories to study later
  Last added: 2025-01-15T12:34:56Z
  ID: UL_kwDOExample
```

JSON output is a fixed lowerCamelCase array with these fields:

1. `name`
2. `description`
3. `lastAddedAt`
4. `id`

```sh
gh star-lists list --json
```

```json
[
  {
    "name": "Learning",
    "description": "Repositories to study later",
    "lastAddedAt": "2025-01-15T12:34:56Z",
    "id": "UL_kwDOExample"
  }
]
```

TSV output uses this fixed field order:

1. `name`
2. `description`
3. `lastAddedAt`
4. `id`

```sh
gh star-lists list --tsv
```

```text
Learning	Repositories to study later	2025-01-15T12:34:56Z	UL_kwDOExample
```

### `gh star-lists repos <LIST_ID>`

Lists repositories in a Star List. Get `<LIST_ID>` from `gh star-lists`, `gh star-lists list --json`, or `gh star-lists list --tsv`.

Human-readable output is the default:

```text
cli/cli
  Description: GitHub's official command line tool
  Fork: no
  Stars: 41000
  Pushed: 2025-01-15T12:34:56Z
  URL: https://github.com/cli/cli
```

JSON output is a fixed lowerCamelCase array with these fields:

1. `nameWithOwner`
2. `description`
3. `isFork`
4. `stargazerCount`
5. `pushedAt`
6. `url`

```sh
gh star-lists repos UL_kwDOExample --json
```

```json
[
  {
    "nameWithOwner": "cli/cli",
    "description": "GitHub's official command line tool",
    "isFork": false,
    "stargazerCount": 41000,
    "pushedAt": "2025-01-15T12:34:56Z",
    "url": "https://github.com/cli/cli"
  }
]
```

TSV output uses this fixed field order:

1. `nameWithOwner`
2. `description`
3. `isFork` as `yes` or `no`
4. `stargazerCount`
5. `pushedAt`
6. `url`

```sh
gh star-lists repos UL_kwDOExample --tsv
```

```text
cli/cli	GitHub's official command line tool	no	41000	2025-01-15T12:34:56Z	https://github.com/cli/cli
```

## v0.1 limitations and non-goals

v0.1 intentionally has no cache state: each query fetches live paginated Star Lists data through GitHub GraphQL using your current `gh` authentication context.

v0.1 also does not accept advanced filtering, sorting, limit flags, selectable JSON fields, shell completion, or interactive selection. Use the stable JSON or TSV output with shell tools such as `jq`, `sort`, `grep`, `awk`, or `head` when you need local composition.

v0.1 is read-only. It has no write operations, mutations, token storage, Star List management commands, or repository management commands.

## Development

Run the local checks before publishing changes:

```sh
go test ./...
go vet ./...
go build
```

Run the full local pre-release checklist before tagging:

```sh
go test ./...
go vet ./...
go build
bash scripts/smoke-local.sh
```

The first three commands verify the Go code directly. The smoke script repeats those checks, verifies auth-free help and usage-error paths, installs the checkout as a `gh` extension, and verifies `gh star-lists --help`. It does not run live Star List queries. If an existing local extension provider already owns `gh star-lists`, the script stops before removing it; opt into replacement only when intended:

```sh
GH_STAR_LISTS_REPLACE_EXTENSION=1 bash scripts/smoke-local.sh
```

To prove the final extension wiring manually:

```sh
gh extension install . --force
gh star-lists --help
```

### Optional manual live verification

These checks are optional and depend on your authenticated GitHub account having Star Lists. They are useful for separating live account/API issues from local install or smoke-check failures, but they are not required for CI, release verification, or `bash scripts/smoke-local.sh`.

1. Confirm GitHub CLI authentication is available:

   ```sh
   gh auth status
   ```

2. List Star Lists in each supported output mode:

   ```sh
   gh star-lists
   gh star-lists --json | jq 'length'
   gh star-lists --tsv
   ```

3. Pick a real list ID from the JSON or TSV output and inspect its repositories:

   ```sh
   gh star-lists repos <LIST_ID>
   gh star-lists repos <LIST_ID> --json | jq 'length'
   gh star-lists repos <LIST_ID> --tsv
   ```

4. Check the inaccessible-list diagnostic stays on stderr and exits non-zero:

   ```sh
   gh star-lists repos INVALID_LIST_ID >/tmp/gh-star-lists-out.txt
   ```

Use a `<LIST_ID>` returned by either list command. Empty output can be valid if the authenticated account has no Star Lists or the selected list has no repositories.

## Release workflow

Releases are tag-driven. Pushing a `v*` tag runs the GitHub Actions release workflow, which uses `cli/gh-extension-precompile@v2` and the project Go version to build release assets for GitHub CLI extension installation.

## Troubleshooting

- `gh star-lists --help` should work without contacting GitHub. If it does not, reinstall the extension with `gh extension install . --force` from a clean checkout.
- If a local checkout or another provider owns the extension name, inspect and replace it deliberately:

  ```sh
  gh extension list
  gh extension remove star-lists
  gh extension install maoyeedy/gh-star-lists
  ```

- Query commands use the existing `gh` auth state. If GitHub returns an authentication error, run `gh auth status` or `gh auth login`.
- Local verification requires both `go` and `gh` in `PATH`; missing tools should be fixed before running the development or smoke-check commands.
