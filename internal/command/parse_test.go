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
			name: "long help short circuits",
			argv: []string{"--help"},
			want: command.Parsed{Action: command.ActionHelp, Mode: format.OutputHuman},
		},
		{
			name: "short help short circuits before missing repos id",
			argv: []string{"repos", "-h"},
			want: command.Parsed{Action: command.ActionHelp, Mode: format.OutputHuman},
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
			name: "cache flag",
			argv: []string{"list", "--cache"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputHuman, Cache: true},
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
			name: "sort repos by starred",
			argv: []string{"repos", "--unlisted", "--sort", "starred"},
			want: command.Parsed{
				Action:   command.ActionRepos,
				Mode:     format.OutputHuman,
				Unlisted: true,
				SortKeys: []string{"starred"},
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
				Action:  command.ActionRepos,
				ListID:  "UL_1",
				Mode:    format.OutputHuman,
				Filters: []command.Filter{{Key: "topic", Value: "go"}, {Key: "topic", Value: "rust"}},
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
		{name: "unknown command", argv: []string{"stars"}, wantMessage: "unknown command"},
		{name: "repos missing id", argv: []string{"repos"}, wantMessage: "missing list id"},
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
			name:        "conflicting output flags",
			argv:        []string{"--json", "--tsv"},
			wantMessage: "cannot combine --plain, --json, and --tsv",
		},
		{
			name:        "conflicting plain output flag",
			argv:        []string{"--plain", "--json"},
			wantMessage: "cannot combine --plain, --json, and --tsv",
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
			name:        "merge rejects identical from and to case insensitive",
			argv:        []string{"merge", "--from", "Work", "--to", "work"},
			wantMessage: "merge requires distinct --from and --to",
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
			name:        "search on move rejected",
			argv:        []string{"move", "owner/repo", "--from", "A", "--to", "B", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
		{
			name:        "search on copy rejected",
			argv:        []string{"copy", "--from", "A", "--to", "B", "--search", "foo"},
			wantMessage: "--search is only supported for repos",
		},
		{
			name:        "search on merge rejected",
			argv:        []string{"merge", "--from", "A", "--to", "B", "--search", "foo"},
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
