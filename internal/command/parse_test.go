package command_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/command"
	"github.com/maoyeedy/gh-star-lists/internal/format"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want command.Parsed
	}{
		{
			name: "empty argv defaults to human list",
			argv: nil,
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputHuman},
		},
		{
			name: "explicit list",
			argv: []string{"list"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputHuman},
		},
		{
			name: "repos captures list id",
			argv: []string{"repos", "UL_kwDOExample"},
			want: command.Parsed{
				Action: command.ActionRepos,
				ListID: "UL_kwDOExample",
				Mode:   format.OutputHuman,
			},
		},
		{
			name: "repos without list parses for interactive run",
			argv: []string{"repos"},
			want: command.Parsed{
				Action: command.ActionRepos,
				Mode:   format.OutputHuman,
			},
		},
		{
			name: "short repo flags",
			argv: []string{
				"repos",
				"UL_1",
				"-s",
				"stars",
				"-d",
				"-l",
				"5",
				"-f",
				"language:Go",
				"-S",
				"cli",
				"-w",
			},
			want: command.Parsed{
				Action:   command.ActionRepos,
				ListID:   "UL_1",
				Mode:     format.OutputHuman,
				SortKeys: []string{"stars"},
				SortDesc: true,
				Limit:    5,
				Filters:  []command.Filter{{Key: "language", Value: "go"}},
				Search:   "cli",
				Web:      true,
			},
		},
		{
			name: "repos unlisted sort starred",
			argv: []string{"repos", "--unlisted", "--sort", "starred"},
			want: command.Parsed{
				Action:   command.ActionRepos,
				Mode:     format.OutputHuman,
				Unlisted: true,
				SortKeys: []string{"starred"},
			},
		},
		{
			name: "repos all sort starred",
			argv: []string{"repos", "--all", "--sort", "starred"},
			want: command.Parsed{
				Action:   command.ActionRepos,
				Mode:     format.OutputHuman,
				All:      true,
				SortKeys: []string{"starred"},
			},
		},
		{
			name: "long help short circuits",
			argv: []string{"--help"},
			want: command.Parsed{Action: command.ActionHelp, Mode: format.OutputHuman},
		},
		{
			name: "help with command sets topic",
			argv: []string{"repos", "-h"},
			want: command.Parsed{
				Action:    command.ActionHelp,
				HelpTopic: "repos",
				Mode:      format.OutputHuman,
			},
		},
		{
			name: "help with alias sets canonical topic",
			argv: []string{"ls", "--help"},
			want: command.Parsed{
				Action:    command.ActionHelp,
				HelpTopic: "list",
				Mode:      format.OutputHuman,
			},
		},
		{
			name: "help with unknown command gives top-level",
			argv: []string{"bogus", "--help"},
			want: command.Parsed{Action: command.ActionHelp, Mode: format.OutputHuman},
		},
		{
			name: "--full alone shows full help",
			argv: []string{"--full"},
			want: command.Parsed{
				Action:   command.ActionHelp,
				FullHelp: true,
				Mode:     format.OutputHuman,
			},
		},
		{
			name: "--help --full shows full help",
			argv: []string{"--help", "--full"},
			want: command.Parsed{
				Action:   command.ActionHelp,
				FullHelp: true,
				Mode:     format.OutputHuman,
			},
		},
		{
			name: "ls alias resolves to list",
			argv: []string{"ls"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputHuman},
		},
		{
			name: "rm alias resolves to remove with flags",
			argv: []string{"rm", "owner/repo", "--from", "My List", "--yes"},
			want: command.Parsed{
				Action:     command.ActionRemove,
				RepoName:   "owner/repo",
				FromListID: "My List",
				Yes:        true,
				Mode:       format.OutputHuman,
			},
		},
		{
			name: "mv alias resolves to move",
			argv: []string{"mv", "owner/repo", "--from", "A", "--to", "B"},
			want: command.Parsed{
				Action:     command.ActionMove,
				RepoName:   "owner/repo",
				FromListID: "A",
				ToListID:   "B",
				Mode:       format.OutputHuman,
			},
		},
		{
			name: "cp alias resolves to copy",
			argv: []string{"cp", "--from", "A", "--to", "B"},
			want: command.Parsed{
				Action:     command.ActionCopy,
				FromListID: "A",
				ToListID:   "B",
				Mode:       format.OutputHuman,
			},
		},
		{
			name: "short edit flags",
			argv: []string{"edit", "UL_1", "-n", "New Name", "-D", ""},
			want: command.Parsed{
				Action:         command.ActionEdit,
				ListID:         "UL_1",
				Name:           "New Name",
				Description:    "",
				DescriptionSet: true,
				Mode:           format.OutputHuman,
			},
		},
		{
			name: "add without --to parses cleanly",
			argv: []string{"add", "owner/repo"},
			want: command.Parsed{
				Action:   command.ActionAdd,
				RepoName: "owner/repo",
				Mode:     format.OutputHuman,
			},
		},
		{
			name: "remove without --from parses cleanly",
			argv: []string{"remove", "owner/repo"},
			want: command.Parsed{
				Action:   command.ActionRemove,
				RepoName: "owner/repo",
				Mode:     format.OutputHuman,
			},
		},
		{
			name: "move without flags parses cleanly",
			argv: []string{"move", "owner/repo"},
			want: command.Parsed{
				Action:   command.ActionMove,
				RepoName: "owner/repo",
				Mode:     format.OutputHuman,
			},
		},
		{
			name: "copy without flags parses cleanly",
			argv: []string{"copy"},
			want: command.Parsed{
				Action: command.ActionCopy,
				Mode:   format.OutputHuman,
			},
		},
		{
			name: "json before subcommand",
			argv: []string{"--json", "repos", "list-id"},
			want: command.Parsed{
				Action: command.ActionRepos,
				ListID: "list-id",
				Mode:   format.OutputJSON,
			},
		},
		{
			name: "tsv after subcommand",
			argv: []string{"list", "--tsv"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputTSV},
		},
		{
			name: "json after repos id",
			argv: []string{"repos", "list-id", "--json"},
			want: command.Parsed{
				Action: command.ActionRepos,
				ListID: "list-id",
				Mode:   format.OutputJSON,
			},
		},
		{
			name: "plain before subcommand",
			argv: []string{"--plain", "list"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputPlain},
		},
		{
			name: "plain after repos id",
			argv: []string{"repos", "list-id", "--plain"},
			want: command.Parsed{
				Action: command.ActionRepos,
				ListID: "list-id",
				Mode:   format.OutputPlain,
			},
		},
		{
			name: "fzf after subcommand",
			argv: []string{"list", "--fzf"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputFZF},
		},
		{
			name: "fzf after repos id",
			argv: []string{"repos", "list-id", "--fzf"},
			want: command.Parsed{
				Action: command.ActionRepos,
				ListID: "list-id",
				Mode:   format.OutputFZF,
			},
		},
		{
			name: "sort list by name",
			argv: []string{"list", "--sort", "name"},
			want: command.Parsed{
				Action:   command.ActionList,
				Mode:     format.OutputHuman,
				SortKeys: []string{"name"},
			},
		},
		{
			name: "sort default list by added descending",
			argv: []string{"--sort", "added", "--desc"},
			want: command.Parsed{
				Action:   command.ActionList,
				Mode:     format.OutputHuman,
				SortKeys: []string{"added"},
				SortDesc: true,
			},
		},
		{
			name: "sort repos by stars descending",
			argv: []string{"--sort", "stars", "repos", "list-id", "--desc"},
			want: command.Parsed{
				Action:   command.ActionRepos,
				ListID:   "list-id",
				Mode:     format.OutputHuman,
				SortKeys: []string{"stars"},
				SortDesc: true,
			},
		},
		{
			name: "sort repos by pushed after id",
			argv: []string{"repos", "list-id", "--sort", "pushed"},
			want: command.Parsed{
				Action:   command.ActionRepos,
				ListID:   "list-id",
				Mode:     format.OutputHuman,
				SortKeys: []string{"pushed"},
			},
		},
		{
			name: "multiple sort keys",
			argv: []string{"list", "--sort", "added,name"},
			want: command.Parsed{
				Action:   command.ActionList,
				Mode:     format.OutputHuman,
				SortKeys: []string{"added", "name"},
			},
		},
		{
			name: "repeated sort flags",
			argv: []string{"list", "--sort", "name", "--sort", "added"},
			want: command.Parsed{
				Action:   command.ActionList,
				Mode:     format.OutputHuman,
				SortKeys: []string{"name", "added"},
			},
		},
		{
			name: "limit valid",
			argv: []string{"list", "--limit", "5"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputHuman, Limit: 5},
		},
		{
			name: "limit with repos",
			argv: []string{"repos", "UL_1", "--limit", "10"},
			want: command.Parsed{
				Action: command.ActionRepos,
				ListID: "UL_1",
				Mode:   format.OutputHuman,
				Limit:  10,
			},
		},
		{
			name: "limit with sort",
			argv: []string{"list", "--sort", "name", "--limit", "3"},
			want: command.Parsed{
				Action:   command.ActionList,
				Mode:     format.OutputHuman,
				SortKeys: []string{"name"},
				Limit:    3,
			},
		},
		{
			name: "filter name",
			argv: []string{"list", "--filter", "name:go"},
			want: command.Parsed{
				Action:  command.ActionList,
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "name", Value: "go"}},
			},
		},
		{
			name: "filter fork on repos",
			argv: []string{"repos", "UL_1", "--filter", "fork:false"},
			want: command.Parsed{
				Action:  command.ActionRepos,
				ListID:  "UL_1",
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "fork", Value: "false"}},
			},
		},
		{
			name: "multiple filters on repos",
			argv: []string{"repos", "UL_1", "--filter", "name:go", "--filter", "fork:true"},
			want: command.Parsed{
				Action:  command.ActionRepos,
				ListID:  "UL_1",
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "name", Value: "go"}, {Key: "fork", Value: "true"}},
			},
		},
		{
			name: "template flag with mode override",
			argv: []string{"list", "--template", "{{.name}}"},
			want: command.Parsed{
				Action:   command.ActionList,
				Mode:     format.OutputTemplate,
				Template: "{{.name}}",
			},
		},
		{
			name: "template with repos",
			argv: []string{"repos", "UL_1", "--template", "{{.nameWithOwner}}"},
			want: command.Parsed{
				Action:   command.ActionRepos,
				ListID:   "UL_1",
				Mode:     format.OutputTemplate,
				Template: "{{.nameWithOwner}}",
			},
		},
		{
			name: "template overrides json",
			argv: []string{"--json", "--template", "{{.name}}"},
			want: command.Parsed{
				Action:   command.ActionList,
				Mode:     format.OutputTemplate,
				Template: "{{.name}}",
			},
		},
		{
			name: "output flag",
			argv: []string{"list", "--output", "out.txt"},
			want: command.Parsed{
				Action:     command.ActionList,
				Mode:       format.OutputHuman,
				OutputPath: "out.txt",
			},
		},
		{
			name: "no-color flag",
			argv: []string{"list", "--no-color"},
			want: command.Parsed{
				Action:  command.ActionList,
				Mode:    format.OutputHuman,
				NoColor: true,
			},
		},
		{
			name: "filter language on repos",
			argv: []string{"repos", "UL_1", "--filter", "language:Go"},
			want: command.Parsed{
				Action:  command.ActionRepos,
				ListID:  "UL_1",
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "language", Value: "go"}},
			},
		},
		{
			name: "sort repos by language",
			argv: []string{"repos", "UL_1", "--sort", "language"},
			want: command.Parsed{
				Action:   command.ActionRepos,
				ListID:   "UL_1",
				Mode:     format.OutputHuman,
				SortKeys: []string{"language"},
			},
		},
		{
			name: "sort list by repos count",
			argv: []string{"list", "--sort", "repos"},
			want: command.Parsed{
				Action:   command.ActionList,
				Mode:     format.OutputHuman,
				SortKeys: []string{"repos"},
			},
		},
		{
			name: "web flag on repos",
			argv: []string{"repos", "UL_1", "--web"},
			want: command.Parsed{
				Action: command.ActionRepos,
				ListID: "UL_1",
				Mode:   format.OutputHuman,
				Web:    true,
			},
		},
		{
			name: "unlisted flag on repos",
			argv: []string{"repos", "--unlisted"},
			want: command.Parsed{
				Action:   command.ActionRepos,
				Mode:     format.OutputHuman,
				Unlisted: true,
			},
		},
		{
			name: "filter min-stars negative clamps to zero",
			argv: []string{"repos", "UL_1", "--filter", "min-stars:-5"},
			want: command.Parsed{
				Action:  command.ActionRepos,
				ListID:  "UL_1",
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "min-stars", Value: "0"}},
			},
		},
		{
			name: "filter max-stars negative clamps to zero",
			argv: []string{"repos", "UL_1", "--filter", "max-stars:-1"},
			want: command.Parsed{
				Action:  command.ActionRepos,
				ListID:  "UL_1",
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "max-stars", Value: "0"}},
			},
		},
		{
			name: "filter min-stars zero is accepted as-is",
			argv: []string{"repos", "UL_1", "--filter", "min-stars:0"},
			want: command.Parsed{
				Action:  command.ActionRepos,
				ListID:  "UL_1",
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "min-stars", Value: "0"}},
			},
		},
		{
			name: "filter topic single value accepted",
			argv: []string{"repos", "UL_1", "--filter", "topic:go"},
			want: command.Parsed{
				Action:  command.ActionRepos,
				ListID:  "UL_1",
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "topic", Value: "go"}},
			},
		},
		{
			name: "filter topic repeated flags AND",
			argv: []string{"repos", "UL_1", "--filter", "topic:go", "--filter", "topic:rust"},
			want: command.Parsed{
				Action: command.ActionRepos,
				ListID: "UL_1",
				Mode:   format.OutputHuman,
				Filters: []command.Filter{
					{Key: "topic", Value: "go"},
					{Key: "topic", Value: "rust"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := command.Parse(tt.argv)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", tt.argv, err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse(%q) = %#v, want %#v", tt.argv, got, tt.want)
			}
		})
	}
}

func TestParseUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		argv        []string
		wantMessage string
	}{
		{
			name:        "list extra arg",
			argv:        []string{"list", "extra"},
			wantMessage: "too many arguments",
		},
		{
			name:        "repos extra arg",
			argv:        []string{"repos", "id", "extra"},
			wantMessage: "too many arguments",
		},
		{
			name:        "version flag is not supported",
			argv:        []string{"-V"},
			wantMessage: "unknown flag",
		},
		{
			name:        "conflicting output flags",
			argv:        []string{"--json", "--tsv"},
			wantMessage: "cannot combine --plain, --json, --tsv, and --fzf",
		},
		{
			name:        "conflicting plain output flag",
			argv:        []string{"--plain", "--json"},
			wantMessage: "cannot combine --plain, --json, --tsv, and --fzf",
		},
		{
			name:        "conflicting fzf and json",
			argv:        []string{"--fzf", "--json"},
			wantMessage: "cannot combine --plain, --json, --tsv, and --fzf",
		},
		{name: "unknown flag", argv: []string{"--xml"}, wantMessage: "unknown flag"},
		{
			name:        "sort missing value",
			argv:        []string{"--sort"},
			wantMessage: "missing value for --sort",
		},
		{
			name:        "sort empty value",
			argv:        []string{"--sort", ""},
			wantMessage: "empty value for --sort",
		},
		{
			name:        "sort flag as value",
			argv:        []string{"--sort", "--desc"},
			wantMessage: "missing value for --sort",
		},
		{
			name:        "desc without sort",
			argv:        []string{"--desc"},
			wantMessage: "--desc requires --sort",
		},
		{
			name:        "list unsupported sort key",
			argv:        []string{"list", "--sort", "stars"},
			wantMessage: "unsupported sort key \"stars\" for list",
		},
		{
			name:        "repos unsupported sort key",
			argv:        []string{"repos", "id", "--sort", "added"},
			wantMessage: "unsupported sort key \"added\" for repos",
		},
		{name: "empty flag value", argv: []string{""}, wantMessage: "empty argument"},
		{
			name:        "limit missing value",
			argv:        []string{"--limit"},
			wantMessage: "missing value for --limit",
		},
		{
			name:        "limit zero",
			argv:        []string{"list", "--limit", "0"},
			wantMessage: "invalid value for --limit",
		},
		{
			name:        "limit negative",
			argv:        []string{"list", "--limit", "-1"},
			wantMessage: "invalid value for --limit",
		},
		{
			name:        "limit non-numeric",
			argv:        []string{"list", "--limit", "abc"},
			wantMessage: "invalid value for --limit",
		},
		{
			name:        "filter missing value",
			argv:        []string{"--filter"},
			wantMessage: "missing value for --filter",
		},
		{
			name:        "filter missing colon",
			argv:        []string{"--filter", "name"},
			wantMessage: "invalid filter",
		},
		{
			name:        "filter empty key",
			argv:        []string{"--filter", ":value"},
			wantMessage: "invalid filter",
		},
		{
			name:        "filter unknown key",
			argv:        []string{"--filter", "badkey:true"},
			wantMessage: "unknown filter key",
		},
		{
			name:        "filter fork on list",
			argv:        []string{"list", "--filter", "fork:false"},
			wantMessage: "filter key \"fork\" is only supported for repos",
		},
		{
			name:        "filter language on list",
			argv:        []string{"list", "--filter", "language:Go"},
			wantMessage: "filter key \"language\" is only supported for repos",
		},
		{
			name:        "sort language on list",
			argv:        []string{"list", "--sort", "language"},
			wantMessage: "unsupported sort key \"language\" for list",
		},
		{
			name:        "sort repos count on repos action",
			argv:        []string{"repos", "UL_1", "--sort", "repos"},
			wantMessage: "unsupported sort key \"repos\" for repos",
		},
		{
			name:        "web flag on list",
			argv:        []string{"list", "--web"},
			wantMessage: "--web is only supported for repos",
		},
		{
			name:        "web flag on default list",
			argv:        []string{"--web"},
			wantMessage: "--web is only supported for repos",
		},
		{
			name:        "web combined with json",
			argv:        []string{"repos", "UL_1", "--web", "--json"},
			wantMessage: "--web cannot be combined with output flags",
		},
		{
			name:        "web combined with tsv",
			argv:        []string{"repos", "UL_1", "--web", "--tsv"},
			wantMessage: "--web cannot be combined with output flags",
		},
		{
			name:        "web combined with fzf",
			argv:        []string{"repos", "UL_1", "--web", "--fzf"},
			wantMessage: "--web cannot be combined with output flags",
		},
		{
			name:        "unlisted on list",
			argv:        []string{"list", "--unlisted"},
			wantMessage: "--unlisted is only supported for repos",
		},
		{
			name:        "unlisted on default list",
			argv:        []string{"--unlisted"},
			wantMessage: "--unlisted is only supported for repos",
		},
		{
			name:        "unlisted with list id",
			argv:        []string{"repos", "UL_1", "--unlisted"},
			wantMessage: "--unlisted does not accept a list id",
		},
		{
			name:        "sort starred on list",
			argv:        []string{"list", "--sort", "starred"},
			wantMessage: "unsupported sort key \"starred\" for list",
		},
		{
			name:        "filter bad fork value",
			argv:        []string{"repos", "UL_1", "--filter", "fork:yes"},
			wantMessage: "invalid filter value for fork",
		},
		{
			name:        "output missing value",
			argv:        []string{"--output"},
			wantMessage: "missing value for --output",
		},
		{
			name:        "template missing value",
			argv:        []string{"--template"},
			wantMessage: "missing value for --template",
		},
		{
			name:        "template empty value",
			argv:        []string{"--template", ""},
			wantMessage: "empty value for --template",
		},
		{
			name:        "multi-sort invalid key in first",
			argv:        []string{"list", "--sort", "stars,name"},
			wantMessage: "unsupported sort key \"stars\" for list",
		},
		{
			name:        "multi-sort invalid key in second",
			argv:        []string{"list", "--sort", "name,stars"},
			wantMessage: "unsupported sort key \"stars\" for list",
		},
		{
			name:        "copy rejects identical from and to",
			argv:        []string{"copy", "--from", "Work", "--to", "Work"},
			wantMessage: "copy requires distinct --from and --to",
		},
		{
			name:        "move rejects identical from and to",
			argv:        []string{"move", "owner/repo", "--from", "A", "--to", "A"},
			wantMessage: "move requires distinct --from and --to",
		},
		{
			name:        "filter topic comma-separated rejected",
			argv:        []string{"repos", "UL_1", "--filter", "topic:go,rust"},
			wantMessage: "only one topic per --filter",
		},
		{
			name:        "filter min-stars non-integer rejected",
			argv:        []string{"repos", "UL_1", "--filter", "min-stars:abc"},
			wantMessage: "expected integer",
		},
		{
			name:        "fzf on create rejected",
			argv:        []string{"create", "MyList", "--fzf"},
			wantMessage: "output flags are not supported for write commands",
		},
		{
			name:        "search on create rejected",
			argv:        []string{"create", "MyList", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
		{
			name:        "search on edit rejected",
			argv:        []string{"edit", "UL_1", "--name", "New", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
		{
			name:        "search on delete rejected",
			argv:        []string{"delete", "UL_1", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
		{
			name:        "search on add rejected",
			argv:        []string{"add", "owner/repo", "--to", "UL_1", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
		{
			name:        "search on remove rejected",
			argv:        []string{"remove", "owner/repo", "--from", "UL_1", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
		{
			name: "search on move rejected",
			argv: []string{
				"move",
				"owner/repo",
				"--from",
				"A",
				"--to",
				"B",
				"--search",
				"foo",
			},
			wantMessage: "--search is only supported for repos",
		},
		{
			name:        "search on copy rejected",
			argv:        []string{"copy", "--from", "A", "--to", "B", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
		{
			name:        "search on unstar rejected",
			argv:        []string{"unstar", "owner/repo", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := command.Parse(tt.argv)
			if err == nil {
				t.Fatalf("Parse(%q) returned nil error", tt.argv)
			}

			var usageErr *command.UsageError
			if !errors.As(err, &usageErr) {
				t.Fatalf("Parse(%q) error %T, want *command.UsageError", tt.argv, err)
			}

			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf(
					"Parse(%q) error %q, want message containing %q",
					tt.argv,
					err,
					tt.wantMessage,
				)
			}
		})
	}
}

func TestParseMouseFlag(t *testing.T) {
	t.Parallel()
	// --mouse on tui sets Mouse = true
	p, err := command.Parse([]string{"tui", "--mouse"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Mouse {
		t.Error("expected Mouse = true")
	}

	// --mouse on a non-tui action returns an error
	_, err = command.Parse([]string{"list", "--mouse"})
	if err == nil {
		t.Error("expected error for --mouse on non-tui action")
	}
}
