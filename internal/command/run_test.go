package command_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/command"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type fakeService struct {
	listCalls    int
	reposCalls   int
	reposListIDs []string
	lists        []githubapi.StarList
	repos        []githubapi.Repository
	listErr      error
	reposErr     error
}

func (f *fakeService) ListStarLists(context.Context) ([]githubapi.StarList, error) {
	f.listCalls++
	return f.lists, f.listErr
}

func (f *fakeService) ListRepositories(_ context.Context, listID string) ([]githubapi.Repository, error) {
	f.reposCalls++
	f.reposListIDs = append(f.reposListIDs, listID)
	return f.repos, f.reposErr
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func fixtureService() *fakeService {
	return &fakeService{
		lists: []githubapi.StarList{
			{Name: "Go Tools", Description: "CLI helpers", LastAddedAt: "2024-05-01T12:00:00Z", ID: "UL_1"},
		},
		repos: []githubapi.Repository{
			{NameWithOwner: "cli/cli", Description: "GitHub CLI", IsFork: false, StargazerCount: 41000, PushedAt: "2024-05-01T12:00:00Z", URL: "https://github.com/cli/cli"},
		},
	}
}

func TestRunHelpWritesStdoutAndDoesNotUseService(t *testing.T) {
	t.Parallel()

	svc := &fakeService{listErr: errors.New("not implemented"), reposErr: errors.New("not implemented")}
	var stdout, stderr strings.Builder

	code := command.Run(context.Background(), []string{"--help"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("Run help exit = %d, want %d", code, command.ExitSuccess)
	}
	if got := stdout.String(); !strings.Contains(got, "gh star-lists") || !strings.Contains(got, "repos <LIST_ID>") {
		t.Fatalf("help stdout missing command details:\n%s", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("help stderr = %q, want empty", stderr.String())
	}
	if svc.listCalls != 0 || svc.reposCalls != 0 {
		t.Fatalf("service calls on help: list=%d repos=%d", svc.listCalls, svc.reposCalls)
	}
}

func TestRunWritesListOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "empty args default human list",
			argv: nil,
			want: "Go Tools\n" +
				"  Description: CLI helpers\n" +
				"  Last added: 2024-05-01T12:00:00Z\n" +
				"  ID: UL_1\n",
		},
		{
			name: "json",
			argv: []string{"list", "--json"},
			want: "[{\"name\":\"Go Tools\",\"description\":\"CLI helpers\",\"lastAddedAt\":\"2024-05-01T12:00:00Z\",\"id\":\"UL_1\"}]\n",
		},
		{
			name: "tsv",
			argv: []string{"list", "--tsv"},
			want: "Go Tools\tCLI helpers\t2024-05-01T12:00:00Z\tUL_1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := fixtureService()
			var stdout, stderr strings.Builder

			code := command.Run(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf("Run(%q) exit = %d, want %d; stderr=%q", tt.argv, code, command.ExitSuccess, stderr.String())
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%q) stderr = %q, want empty", tt.argv, stderr.String())
			}
			if svc.listCalls != 1 || svc.reposCalls != 0 {
				t.Fatalf("service calls list=%d repos=%d, want list=1 repos=0", svc.listCalls, svc.reposCalls)
			}
		})
	}
}

func TestRunWritesReposOutputWithParsedListID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "json",
			argv: []string{"repos", "UL_1", "--json"},
			want: "[{\"nameWithOwner\":\"cli/cli\",\"description\":\"GitHub CLI\",\"isFork\":false,\"stargazerCount\":41000,\"pushedAt\":\"2024-05-01T12:00:00Z\",\"url\":\"https://github.com/cli/cli\"}]\n",
		},
		{
			name: "tsv",
			argv: []string{"repos", "UL_1", "--tsv"},
			want: "cli/cli\tGitHub CLI\tno\t41000\t2024-05-01T12:00:00Z\thttps://github.com/cli/cli\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := fixtureService()
			var stdout, stderr strings.Builder

			code := command.Run(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf("Run(%q) exit = %d, want %d; stderr=%q", tt.argv, code, command.ExitSuccess, stderr.String())
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%q) stderr = %q, want empty", tt.argv, stderr.String())
			}
			if svc.listCalls != 0 || svc.reposCalls != 1 {
				t.Fatalf("service calls list=%d repos=%d, want list=0 repos=1", svc.listCalls, svc.reposCalls)
			}
			if got := strings.Join(svc.reposListIDs, ","); got != "UL_1" {
				t.Fatalf("repos list IDs = %q, want UL_1", got)
			}
		})
	}
}

func TestRunEmptyResultsSucceed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{name: "empty lists human", argv: []string{"list"}, want: "No Star Lists found.\n"},
		{name: "empty lists json", argv: []string{"list", "--json"}, want: "[]\n"},
		{name: "empty repos tsv", argv: []string{"repos", "UL_1", "--tsv"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &fakeService{}
			var stdout, stderr strings.Builder

			code := command.Run(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf("Run(%q) exit = %d, want %d; stderr=%q", tt.argv, code, command.ExitSuccess, stderr.String())
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout = %q, want %q", tt.argv, got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%q) stderr = %q, want empty", tt.argv, stderr.String())
			}
		})
	}
}

func TestRunUsageErrorWritesStderrExitUsageAndDoesNotUseService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		argv    []string
		wantErr string
	}{
		{name: "missing repos id", argv: []string{"repos"}, wantErr: "error: missing list id for repos"},
		{name: "unknown command", argv: []string{"stars"}, wantErr: "unknown command \"stars\""},
		{name: "extra list args", argv: []string{"list", "extra"}, wantErr: "too many arguments for list"},
		{name: "extra repos args", argv: []string{"repos", "UL_1", "extra"}, wantErr: "too many arguments for repos"},
		{name: "conflicting output flags", argv: []string{"list", "--json", "--tsv"}, wantErr: "cannot combine --json and --tsv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &fakeService{listErr: errors.New("not implemented"), reposErr: errors.New("not implemented")}
			var stdout, stderr strings.Builder

			code := command.Run(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitUsage {
				t.Fatalf("Run(%q) exit = %d, want %d", tt.argv, code, command.ExitUsage)
			}
			if stdout.Len() != 0 {
				t.Fatalf("usage stdout = %q, want empty", stdout.String())
			}
			gotErr := stderr.String()
			if !strings.Contains(gotErr, tt.wantErr) || !strings.Contains(gotErr, "Usage:") {
				t.Fatalf("usage stderr missing diagnostic/help:\n%s", gotErr)
			}
			if svc.listCalls != 0 || svc.reposCalls != 0 {
				t.Fatalf("service calls on usage error: list=%d repos=%d", svc.listCalls, svc.reposCalls)
			}
		})
	}
}

func TestRunRuntimeAPIErrorsReturnFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		argv      []string
		svc       *fakeService
		wantErr   []string
		wantList  int
		wantRepos int
	}{
		{
			name:     "empty args default list",
			argv:     nil,
			svc:      &fakeService{listErr: errors.New("not implemented")},
			wantErr:  []string{"error: failed to list Star Lists: not implemented"},
			wantList: 1,
		},
		{
			name:     "explicit list auth failure",
			argv:     []string{"list"},
			svc:      &fakeService{listErr: errors.New("GitHub GraphQL request failed: Bad credentials")},
			wantErr:  []string{"error: failed to list Star Lists: GitHub GraphQL request failed: Bad credentials", "gh auth status"},
			wantList: 1,
		},
		{
			name:      "repos graphql failure",
			argv:      []string{"repos", "UL_kwDOExample"},
			svc:       &fakeService{reposErr: errors.New("GitHub GraphQL request failed: secondary rate limit")},
			wantErr:   []string{"error: failed to list repositories for Star List \"UL_kwDOExample\": GitHub GraphQL request failed: secondary rate limit"},
			wantRepos: 1,
		},
		{
			name:      "repos inaccessible list",
			argv:      []string{"repos", "UL_missing"},
			svc:       &fakeService{reposErr: githubapi.ErrInaccessibleList},
			wantErr:   []string{"error: failed to list repositories for Star List \"UL_missing\": GitHub Star List is inaccessible or is not a UserList", "deleted, private, inaccessible to this account, or from another GitHub account"},
			wantRepos: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder

			code := command.Run(context.Background(), tt.argv, &stdout, &stderr, tt.svc)

			if code != command.ExitFailure {
				t.Fatalf("Run(%q) exit = %d, want %d", tt.argv, code, command.ExitFailure)
			}
			if stdout.Len() != 0 {
				t.Fatalf("runtime failure stdout = %q, want empty", stdout.String())
			}
			gotErr := stderr.String()
			for _, want := range tt.wantErr {
				if !strings.Contains(gotErr, want) {
					t.Fatalf("runtime failure stderr = %q, want %q", gotErr, want)
				}
			}
			if tt.svc.listCalls != tt.wantList || tt.svc.reposCalls != tt.wantRepos {
				t.Fatalf("service calls list=%d repos=%d, want list=%d repos=%d", tt.svc.listCalls, tt.svc.reposCalls, tt.wantList, tt.wantRepos)
			}
		})
	}
}

func TestRunNilServiceReturnsFailure(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder

	code := command.Run(context.Background(), []string{"list"}, &stdout, &stderr, nil)

	if code != command.ExitFailure {
		t.Fatalf("Run nil service exit = %d, want %d", code, command.ExitFailure)
	}
	if stdout.Len() != 0 {
		t.Fatalf("Run nil service stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "GitHub service is not configured") {
		t.Fatalf("stderr = %q, want service diagnostic", stderr.String())
	}
}

func TestRunWriteFailuresReturnFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		argv    []string
		svc     *fakeService
		wantErr string
	}{
		{name: "help", argv: []string{"--help"}, svc: &fakeService{}, wantErr: "failed to write help"},
		{name: "data output", argv: []string{"list"}, svc: fixtureService(), wantErr: "failed to write output"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr strings.Builder

			code := command.Run(context.Background(), tt.argv, errWriter{}, &stderr, tt.svc)

			if code != command.ExitFailure {
				t.Fatalf("Run %s write failure exit = %d, want %d", tt.name, code, command.ExitFailure)
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("stderr = %q, want %q diagnostic", stderr.String(), tt.wantErr)
			}
		})
	}
}
