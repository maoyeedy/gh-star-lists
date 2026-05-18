package command

import (
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/format"
)

const helpTextFull = `gh star-lists

Query and manage GitHub Star Lists through the GitHub CLI authentication context.

Usage:
  gh star-lists [list] [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache-ttl <DURATION>] [--output <FILE>] [--template <STR>] [--jq <QUERY>] [--no-color] [--plain | --json | --tsv]
  gh star-lists repos <LIST_ID_OR_NAME> [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache-ttl <DURATION>] [--output <FILE>] [--template <STR>] [--jq <QUERY>] [--no-color] [--plain | --json | --tsv] [--web] [--unlisted] [--all] [--search <STR>]
  gh star-lists create <NAME> [--description <STR>] [--private | --public] [--dry-run]
  gh star-lists edit <LIST_ID_OR_NAME> [--name <STR>] [--description <STR>] [--private | --public] [--dry-run]
  gh star-lists delete <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists add <REPO> --to <LIST_ID_OR_NAME> [--dry-run]
  gh star-lists remove <REPO> --from <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists move <REPO> --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists copy --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--delete-source] [--yes] [--dry-run]
  gh star-lists merge --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists unstar <REPO> [--yes] [--dry-run]
  gh star-lists --help

Commands:
  list              List your Star Lists. This is the default command.
  repos <LIST_ID_OR_NAME>   List repositories in a Star List. Accepts a list ID or
                    a case-insensitive name (fetches lists to resolve).
  create <NAME>     Create a new Star List with the given name.
  edit <LIST_ID_OR_NAME>    Update a Star List's name, description, or visibility.
  delete <LIST_ID_OR_NAME>  Delete a Star List (removes all repos from it).
  add <REPO>        Add a starred repository to a Star List.
  remove <REPO>     Remove a repository from a Star List.
  move <REPO>       Move a repository between Star Lists.
  copy              Copy all repositories from one Star List to another.
  merge             Merge all repositories from one Star List into another,
                    then delete the source list.
  unstar <REPO>     Unstar a repository.

Output:
  human             Table output optimized for reading (default).
  --plain           Detailed plain text output.
  --json            Machine-readable JSON output.
  --tsv             Tab-separated output for scripts.
  --output <FILE>   Write output to a file instead of stdout. Prompts before overwriting an existing file; pass --yes to skip the prompt.
  --template <STR>  Go template string applied to JSON data (implies --json data model).
  --jq <QUERY>      jq-style query applied to JSON output (implies --json).
  --no-color        Disable ANSI color in human output.
  --web             Open the Star List in a browser (repos only; no output flags).
  --unlisted        Show starred repos not in any Star List (repos only; N+1 API calls).
  --all             Show all starred repositories (repos only).

Sorting:
  --sort <KEY1,KEY2>  Sort results locally after fetching all pages.
                    Comma-separate or repeat for secondary sort.
                    Use "key:asc" or "key:desc" for per-key direction.
                    List keys: added, name, repos.
                    Repository keys: name, stars, pushed, language, starred.
  --desc            Reverse an explicit --sort order.
  --limit <N>       Show only the first N results (applied server-side when possible).

Filtering:
  --filter <KEY:VALUE>  Filter results after fetching all pages (repeatable).
                    Keys: name (case-insensitive contains), fork, language, archived,
                    license, min-stars, max-stars, topic (repos only).
  --search <STR>   Fuzzy-match repositories by name, description, and language.

Caching:
  --cache-ttl <DURATION>  Override the in-memory cache TTL (default: 5m). Pass 0 to disable.

Examples:
  gh star-lists
  gh star-lists --sort name
  gh star-lists --plain
  gh star-lists list --tsv --sort repos --desc
  gh star-lists repos UL_kwDOExample --sort stars --desc
  gh star-lists repos UL_kwDOExample --json
  gh star-lists repos UL_kwDOExample --filter language:Go --sort language
  gh star-lists repos UL_kwDOExample --web
  gh star-lists repos --unlisted --sort starred
  gh star-lists repos --all --json
  gh star-lists create "My List" --description "Tools I use"
  gh star-lists edit "My List" --name "Better Name" --private
  gh star-lists delete "Old List" --yes
  gh star-lists add cli/cli --to "My List"
  gh star-lists remove cli/cli --from "My List"
  gh star-lists move cli/cli --from "Old" --to "New"
  gh star-lists copy --from tools --to backup
  gh star-lists merge --from old --to new

Authentication:
  Uses the user's existing gh authentication. This command does not store tokens.

Host:
  --host <HOST>     GitHub Enterprise hostname (default: github.com).
`

const helpTextCompact = `gh star-lists

Query and manage GitHub Star Lists through the GitHub CLI.

Usage:
  gh star-lists [list] [flags]
  gh star-lists <command> [flags]
  gh star-lists <command> --help
  gh star-lists --full

Commands:
  list    List your Star Lists (default).
  repos   List repositories in a Star List.
  create  Create a new Star List.
  edit    Update a Star List.
  delete  Delete a Star List.
  add     Add a starred repo to a Star List.
  remove  Remove a repo from a Star List.
  move    Move a repo between Star Lists.
  copy    Copy all repos from one list to another.
  merge   Merge one list into another and delete the source.
  unstar  Unstar a repository.

Aliases: ls=list  rm=remove  mv=move  cp=copy

Examples:
  gh star-lists                                         # list your Star Lists
  gh star-lists repos <LIST_ID_OR_NAME>                 # view repos in a list
  gh star-lists repos <LIST> --sort stars --desc --filter language:Go
  gh star-lists add <REPO> --to <LIST>                  # add a repo (prompts when omitted in a TTY)
  gh star-lists list --json | jq '.[].name'             # script-friendly

Run gh star-lists <command> --help for command details.
Run gh star-lists --full for the complete flag reference.

Authentication:
  Uses the user's existing gh authentication. This command does not store tokens.
`

var commandHelp = map[Action]string{
	ActionList: `gh star-lists list

List your GitHub Star Lists.

Usage:
  gh star-lists [list] [flags]

Flags:
  -s, --sort <KEY>          Sort by: added, name, repos.
  -d, --desc                Reverse sort order.
  -l, --limit <N>           Show first N results.
  -f, --filter <KEY:VALUE>  Filter by: name (case-insensitive contains).
  --json                Machine-readable JSON output.
  --tsv                 Tab-separated output.
  --plain               Detailed plain output.
  --no-color            Disable ANSI color.
  --output <FILE>       Write output to file.
  --template <STR>      Go template string (implies JSON data model).
  --jq <QUERY>          jq-style filter (implies JSON).
  --cache-ttl <D>       Override cache TTL (default: 5m). Pass 0 to disable.

Examples:
  gh star-lists
  gh star-lists list --sort repos --desc
  gh star-lists list --tsv
`,

	ActionRepos: `gh star-lists repos

List repositories in a Star List.

Usage:
  gh star-lists repos <LIST_ID_OR_NAME> [flags]
  gh star-lists repos                         (prompts for list in a TTY)
  gh star-lists repos --unlisted [flags]
  gh star-lists repos --all [flags]

Flags:
  -s, --sort <KEY>          Sort by: name, stars, pushed, language, starred.
  -d, --desc                Reverse sort order.
  -l, --limit <N>           Show first N results.
  -f, --filter <KEY:VALUE>  Filter by: name, fork, language, archived, license,
                            min-stars, max-stars, topic.
  -S, --search <STR>        Fuzzy-match by name, description, language.
  --json                Machine-readable JSON output.
  --tsv                 Tab-separated output.
  --plain               Detailed plain output.
  -w, --web             Open list in browser (no output flags).
  --unlisted            Show starred repos not in any list (N+1 API calls).
  --all                 Show all starred repos.
  --no-color            Disable ANSI color.
  --output <FILE>       Write output to file.
  --template <STR>      Go template string (implies JSON data model).
  --jq <QUERY>          jq-style filter (implies JSON).
  --cache-ttl <D>       Override cache TTL (default: 5m). Pass 0 to disable.

Examples:
  gh star-lists repos "Go Tools"
  gh star-lists repos UL_kwDOExample --sort stars --desc --filter language:Go
  gh star-lists repos --unlisted --sort starred
`,

	ActionCreate: `gh star-lists create

Create a new GitHub Star List.

Usage:
  gh star-lists create <NAME> [flags]
  gh star-lists create                        (prompts for name, description, visibility in a TTY)

Flags:
  -D, --description <STR>   Description for the new list.
  --private             Create as private (default: public).
  --public              Create as public.
  --dry-run             Print what would happen without making changes.

Examples:
  gh star-lists create "Go Tools"
  gh star-lists create "Private Notes" --description "Personal bookmarks" --private
`,

	ActionEdit: `gh star-lists edit

Update a Star List's name, description, or visibility.

Usage:
  gh star-lists edit <LIST_ID_OR_NAME> [flags]
  gh star-lists edit <LIST_ID_OR_NAME>        (prompts for fields in a TTY)

At least one of --name, --description, --private, or --public is required in non-interactive environments.

Flags:
  -n, --name <STR>          New name for the list.
  -D, --description <STR>   New description.
  --private             Set visibility to private.
  --public              Set visibility to public.
  --dry-run             Print what would happen without making changes.

Examples:
  gh star-lists edit "Go Tools" --name "Go Libraries"
  gh star-lists edit "Go Tools" --description "Updated description" --private
`,

	ActionDelete: `gh star-lists delete

Delete a Star List. This removes the list but does not unstar its repositories.

Usage:
  gh star-lists delete <LIST_ID_OR_NAME> [flags]

Flags:
  --yes       Skip confirmation prompt (required in non-interactive environments).
  --dry-run   Print what would happen without making changes.

Safety: requires --yes, --dry-run, or interactive confirmation.

Examples:
  gh star-lists delete "Old List" --yes
  gh star-lists delete "Old List" --dry-run
`,

	ActionAdd: `gh star-lists add

Add a starred repository to a Star List.

Usage:
  gh star-lists add <REPO> --to <LIST_ID_OR_NAME> [flags]
  gh star-lists add <REPO>                         (prompts for list in a TTY)

Flags:
  --to <LIST>   Target Star List (name or ID). Prompts when omitted in a TTY.
  --dry-run     Print what would happen without making changes.

Examples:
  gh star-lists add cli/cli --to "Go Tools"
  gh star-lists add cli/cli   # interactive list picker in a TTY
`,

	ActionRemove: `gh star-lists remove

Remove a repository from a Star List.

Usage:
  gh star-lists remove <REPO> --from <LIST_ID_OR_NAME> [flags]
  gh star-lists remove <REPO>                           (prompts for list in a TTY)

Flags:
  --from <LIST>   Source Star List (name or ID). Prompts when omitted in a TTY.
  --yes           Skip confirmation prompt.
  --dry-run       Print what would happen without making changes.

Safety: requires --yes, --dry-run, or interactive confirmation.

Examples:
  gh star-lists remove cli/cli --from "Go Tools" --yes
  gh star-lists remove cli/cli   # interactive list picker in a TTY
`,

	ActionMove: `gh star-lists move

Move a repository from one Star List to another.

Usage:
  gh star-lists move <REPO> --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [flags]
  gh star-lists move <REPO>   (prompts for both lists in a TTY)

Flags:
  --from <LIST>   Source Star List (name or ID). Prompts when omitted in a TTY.
  --to <LIST>     Target Star List (name or ID). Prompts when omitted in a TTY.
  --yes           Skip confirmation prompt.
  --dry-run       Print what would happen without making changes.

Safety: requires --yes, --dry-run, or interactive confirmation.

Examples:
  gh star-lists move cli/cli --from "Old" --to "New" --yes
  gh star-lists move cli/cli   # interactive list pickers in a TTY
`,

	ActionCopy: `gh star-lists copy

Copy all repositories from one Star List to another.

Usage:
  gh star-lists copy --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [flags]
  gh star-lists copy   (prompts for both lists in a TTY)

Flags:
  --from <LIST>     Source Star List (name or ID). Prompts when omitted in a TTY.
  --to <LIST>       Target Star List (name or ID). Prompts when omitted in a TTY.
  --delete-source   Delete the source list after copying (use merge instead).
  --yes             Skip confirmation prompt (required when using --delete-source).
  --dry-run         Print what would happen without making changes.

Examples:
  gh star-lists copy --from "Work" --to "Archive"
  gh star-lists copy   # interactive list pickers in a TTY
`,

	ActionMerge: `gh star-lists merge

Merge all repositories from one Star List into another, then delete the source.

Usage:
  gh star-lists merge --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [flags]
  gh star-lists merge   (prompts for both lists in a TTY)

Flags:
  --from <LIST>   Source Star List to merge from (name or ID). Prompts when omitted in a TTY.
  --to <LIST>     Target Star List to merge into (name or ID). Prompts when omitted in a TTY.
  --yes           Skip confirmation prompt.
  --dry-run       Print what would happen without making changes.

Safety: requires --yes, --dry-run, or interactive confirmation. Source list is deleted.

Examples:
  gh star-lists merge --from "Old" --to "New" --yes
  gh star-lists merge   # interactive list pickers in a TTY
`,

	ActionUnstar: `gh star-lists unstar

Unstar a repository entirely (removes from all Star Lists).

Usage:
  gh star-lists unstar <REPO> [flags]

Flags:
  --yes       Skip confirmation prompt.
  --dry-run   Print what would happen without making changes.

Safety: requires --yes, --dry-run, or interactive confirmation.

Examples:
  gh star-lists unstar cli/cli --yes
  gh star-lists unstar cli/cli --dry-run
`,
}

func applyBold(s string, options format.Options) string {
	if !options.Color {
		return s
	}
	var b strings.Builder
	boldFn := format.Bold(true)
	cyanFn := format.Cyan(true)
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "gh star-lists"):
			b.WriteString(boldFn(line))
		case strings.HasPrefix(line, "  gh star-lists"):
			b.WriteString(boldFn(line))
		case strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "  "):
			b.WriteString(boldFn(line))
		case strings.HasPrefix(strings.TrimSpace(line), "--") || strings.HasPrefix(strings.TrimSpace(line), "-"):
			b.WriteString(cyanFn(line))
		default:
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func HelpTextFor(topic Action, full bool, options format.Options) string {
	if full {
		return applyBold(helpTextFull, options)
	}
	if topic != "" {
		if body, ok := commandHelp[topic]; ok {
			return applyBold(body, options)
		}
	}
	return applyBold(helpTextCompact, options)
}

func HelpText() string {
	return helpTextFull
}

func HelpTextWithOptions(options format.Options) string {
	return applyBold(helpTextFull, options)
}

func UsageText() string {
	return `Usage:
  gh star-lists [list] [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache-ttl <DURATION>] [--output <FILE>] [--template <STR>] [--jq <QUERY>] [--no-color] [--plain | --json | --tsv]
  gh star-lists repos <LIST_ID_OR_NAME> [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache-ttl <DURATION>] [--output <FILE>] [--template <STR>] [--jq <QUERY>] [--no-color] [--plain | --json | --tsv] [--web] [--unlisted] [--all] [--search <STR>]
  gh star-lists create <NAME> [--description <STR>] [--private | --public] [--dry-run]
  gh star-lists edit <LIST_ID_OR_NAME> [--name <STR>] [--description <STR>] [--private | --public] [--dry-run]
  gh star-lists delete <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists add <REPO> --to <LIST_ID_OR_NAME> [--dry-run]
  gh star-lists remove <REPO> --from <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists move <REPO> --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists copy --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--delete-source] [--yes] [--dry-run]
  gh star-lists merge --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists unstar <REPO> [--yes] [--dry-run]
  gh star-lists --help
`
}
