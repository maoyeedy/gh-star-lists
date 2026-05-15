package command_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/command"
	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type fakeService struct {
	listCalls    int
	reposCalls   int
	starredCalls int
	reposListIDs []string
	lists        []githubapi.StarList
	repos        []githubapi.Repository
	starred      []githubapi.Repository
	listErr      error
	reposErr     error
	starredErr   error
}

func (f *fakeService) ListStarLists(context.Context) ([]githubapi.StarList, error) {
	f.listCalls++
	return f.lists, f.listErr
}

func (f *fakeService) ListRepositories(
	_ context.Context,
	listID string,
) ([]githubapi.Repository, error) {
	f.reposCalls++
	f.reposListIDs = append(f.reposListIDs, listID)
	return f.repos, f.reposErr
}

func (f *fakeService) ListStarredRepositories(context.Context) ([]githubapi.Repository, error) {
	f.starredCalls++
	return f.starred, f.starredErr
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func fixtureService() *fakeService {
	return &fakeService{
		lists: []githubapi.StarList{
			{
				Name:        "Go Tools",
				Description: "CLI helpers",
				LastAddedAt: "2024-05-01T12:00:00Z",
				ID:          "UL_1",
				RepoCount:   3,
				URL:         "https://github.com/stars/maoyeedy/lists/go-tools",
			},
		},
		repos: []githubapi.Repository{
			{
				NameWithOwner:  "cli/cli",
				Description:    "GitHub CLI",
				IsFork:         false,
				StargazerCount: 41000,
				PushedAt:       "2024-05-01T12:00:00Z",
				URL:            "https://github.com/cli/cli",
			},
		},
	}
}

func sortableFixtureService() *fakeService {
	return &fakeService{
		lists: []githubapi.StarList{
			{
				Name:        "zeta",
				Description: "Last by name",
				LastAddedAt: "2024-05-03T12:00:00Z",
				ID:          "UL_3",
				RepoCount:   1,
				URL:         "https://github.com/stars/maoyeedy/lists/zeta",
			},
			{
				Name:        "Alpha",
				Description: "First by name",
				LastAddedAt: "2024-05-02T12:00:00Z",
				ID:          "UL_2",
				RepoCount:   5,
				URL:         "https://github.com/stars/maoyeedy/lists/alpha",
			},
			{
				Name:        "beta",
				Description: "Middle by name",
				LastAddedAt: "2024-05-01T12:00:00Z",
				ID:          "UL_1",
				RepoCount:   3,
				URL:         "https://github.com/stars/maoyeedy/lists/beta",
			},
		},
		repos: []githubapi.Repository{
			{
				NameWithOwner:  "owner/zeta",
				Description:    "Last by name",
				IsFork:         false,
				StargazerCount: 2,
				PushedAt:       "2024-05-01T12:00:00Z",
				URL:            "https://github.com/owner/zeta",
			},
			{
				NameWithOwner:  "owner/Alpha",
				Description:    "First by name",
				IsFork:         false,
				StargazerCount: 5,
				PushedAt:       "2024-05-03T12:00:00Z",
				URL:            "https://github.com/owner/Alpha",
			},
			{
				NameWithOwner:  "owner/beta",
				Description:    "Middle by name",
				IsFork:         false,
				StargazerCount: 3,
				PushedAt:       "2024-05-02T12:00:00Z",
				URL:            "https://github.com/owner/beta",
			},
		},
	}
}

func filterableFixtureService() *fakeService {
	return &fakeService{
		lists: []githubapi.StarList{
			{
				Name:        "Go Tools",
				Description: "CLI helpers",
				LastAddedAt: "2024-05-01T12:00:00Z",
				ID:          "UL_1",
				RepoCount:   10,
				URL:         "https://github.com/stars/maoyeedy/lists/go-tools",
			},
			{
				Name:        "Go Web",
				Description: "Web frameworks",
				LastAddedAt: "2024-05-02T12:00:00Z",
				ID:          "UL_2",
				RepoCount:   5,
				URL:         "https://github.com/stars/maoyeedy/lists/go-web",
			},
			{
				Name:        "Rust",
				Description: "Rust ecosystem",
				LastAddedAt: "2024-05-03T12:00:00Z",
				ID:          "UL_3",
				RepoCount:   8,
				URL:         "https://github.com/stars/maoyeedy/lists/rust",
			},
		},
		repos: []githubapi.Repository{
			{
				NameWithOwner:  "owner/go-lib",
				Description:    "Go library",
				IsFork:         false,
				StargazerCount: 100,
				PushedAt:       "2024-05-01T12:00:00Z",
				URL:            "https://github.com/owner/go-lib",
			},
			{
				NameWithOwner:  "owner/rust-tool",
				Description:    "Rust tool",
				IsFork:         true,
				StargazerCount: 200,
				PushedAt:       "2024-05-02T12:00:00Z",
				URL:            "https://github.com/owner/rust-tool",
			},
			{
				NameWithOwner:  "owner/go-app",
				Description:    "Go app",
				IsFork:         false,
				StargazerCount: 300,
				PushedAt:       "2024-05-03T12:00:00Z",
				URL:            "https://github.com/owner/go-app",
			},
		},
	}
}

func languageFixtureService() *fakeService {
	return &fakeService{
		lists: []githubapi.StarList{
			{Name: "Mixed", ID: "UL_1", URL: "https://github.com/stars/user/lists/mixed"},
		},
		repos: []githubapi.Repository{
			{
				NameWithOwner:  "owner/go-tool",
				Description:    "A Go tool",
				StargazerCount: 300,
				PushedAt:       "2024-05-03T12:00:00Z",
				URL:            "https://github.com/owner/go-tool",
				Language:       "Go",
			},
			{
				NameWithOwner:  "owner/rust-lib",
				Description:    "A Rust lib",
				StargazerCount: 200,
				PushedAt:       "2024-05-02T12:00:00Z",
				URL:            "https://github.com/owner/rust-lib",
				Language:       "Rust",
			},
			{
				NameWithOwner:  "owner/go-web",
				Description:    "A Go web",
				StargazerCount: 100,
				PushedAt:       "2024-05-01T12:00:00Z",
				URL:            "https://github.com/owner/go-web",
				Language:       "Go",
			},
		},
	}
}

func runCommand(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	service githubapi.Service,
) int {
	return command.RunWithOptions(ctx, args, stdout, stderr, service, testOutputOptions)
}

func testOutputOptions(mode format.OutputMode) format.Options {
	return format.Options{
		Mode:  mode,
		Width: 120,
		Now:   time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
	}
}

func TestRunHelpWritesStdoutAndDoesNotUseService(t *testing.T) {
	t.Parallel()

	svc := &fakeService{
		listErr:  errors.New("not implemented"),
		reposErr: errors.New("not implemented"),
	}
	var stdout, stderr strings.Builder

	code := runCommand(context.Background(), []string{"--help"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("Run help exit = %d, want %d", code, command.ExitSuccess)
	}
	if got := stdout.String(); !strings.Contains(got, "gh star-lists") ||
		!strings.Contains(got, "repos <LIST_ID_OR_NAME>") {
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
			want: "NAME      REPOS  ADDED   ID    URL\n" +
				"Go Tools  3      2y ago  UL_1  https://github.com/stars/maoyeedy/lists/go-tools\n",
		},
		{
			name: "plain list",
			argv: []string{"list", "--plain"},
			want: "Go Tools\n" +
				"  Description: CLI helpers\n" +
				"  Repos: 3\n" +
				"  Last added: 2024-05-01T12:00:00Z\n" +
				"  ID: UL_1\n" +
				"  URL: https://github.com/stars/maoyeedy/lists/go-tools\n",
		},
		{
			name: "json",
			argv: []string{"list", "--json"},
			want: "[{\"name\":\"Go Tools\",\"description\":\"CLI helpers\",\"lastAddedAt\":\"2024-05-01T12:00:00Z\",\"id\":\"UL_1\",\"repoCount\":3,\"url\":\"https://github.com/stars/maoyeedy/lists/go-tools\"}]\n",
		},
		{
			name: "tsv",
			argv: []string{"list", "--tsv"},
			want: "Go Tools\tCLI helpers\t3\t2024-05-01T12:00:00Z\tUL_1\thttps://github.com/stars/maoyeedy/lists/go-tools\n",
		},
		{
			name: "template",
			argv: []string{"list", "--template", "{{range .}}{{.name}}\n{{end}}"},
			want: "Go Tools\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := fixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run(%q) exit = %d, want %d; stderr=%q",
					tt.argv,
					code,
					command.ExitSuccess,
					stderr.String(),
				)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%q) stderr = %q, want empty", tt.argv, stderr.String())
			}
			if svc.listCalls != 1 || svc.reposCalls != 0 {
				t.Fatalf(
					"service calls list=%d repos=%d, want list=1 repos=0",
					svc.listCalls,
					svc.reposCalls,
				)
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
			want: "[{\"nameWithOwner\":\"cli/cli\",\"description\":\"GitHub CLI\",\"isFork\":false,\"stargazerCount\":41000,\"pushedAt\":\"2024-05-01T12:00:00Z\",\"url\":\"https://github.com/cli/cli\",\"language\":\"\"}]\n",
		},
		{
			name: "tsv",
			argv: []string{"repos", "UL_1", "--tsv"},
			want: "cli/cli\tGitHub CLI\tno\t41000\t2024-05-01T12:00:00Z\thttps://github.com/cli/cli\t\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := fixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run(%q) exit = %d, want %d; stderr=%q",
					tt.argv,
					code,
					command.ExitSuccess,
					stderr.String(),
				)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%q) stderr = %q, want empty", tt.argv, stderr.String())
			}
			if svc.listCalls != 1 || svc.reposCalls != 1 {
				t.Fatalf(
					"service calls list=%d repos=%d, want list=1 repos=1",
					svc.listCalls,
					svc.reposCalls,
				)
			}
			if got := strings.Join(svc.reposListIDs, ","); got != "UL_1" {
				t.Fatalf("repos list IDs = %q, want UL_1", got)
			}
		})
	}
}

func TestRunSortsListOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "name ascending",
			argv: []string{"list", "--sort", "name", "--tsv"},
			want: "Alpha\tFirst by name\t5\t2024-05-02T12:00:00Z\tUL_2\thttps://github.com/stars/maoyeedy/lists/alpha\n" +
				"beta\tMiddle by name\t3\t2024-05-01T12:00:00Z\tUL_1\thttps://github.com/stars/maoyeedy/lists/beta\n" +
				"zeta\tLast by name\t1\t2024-05-03T12:00:00Z\tUL_3\thttps://github.com/stars/maoyeedy/lists/zeta\n",
		},
		{
			name: "added descending",
			argv: []string{"list", "--sort", "added", "--desc", "--tsv"},
			want: "zeta\tLast by name\t1\t2024-05-03T12:00:00Z\tUL_3\thttps://github.com/stars/maoyeedy/lists/zeta\n" +
				"Alpha\tFirst by name\t5\t2024-05-02T12:00:00Z\tUL_2\thttps://github.com/stars/maoyeedy/lists/alpha\n" +
				"beta\tMiddle by name\t3\t2024-05-01T12:00:00Z\tUL_1\thttps://github.com/stars/maoyeedy/lists/beta\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := sortableFixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run(%q) exit = %d, want %d; stderr=%q",
					tt.argv,
					code,
					command.ExitSuccess,
					stderr.String(),
				)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
		})
	}
}

func TestRunSortsReposOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "name ascending",
			argv: []string{"repos", "UL_1", "--sort", "name", "--tsv"},
			want: "owner/Alpha\tFirst by name\tno\t5\t2024-05-03T12:00:00Z\thttps://github.com/owner/Alpha\t\n" +
				"owner/beta\tMiddle by name\tno\t3\t2024-05-02T12:00:00Z\thttps://github.com/owner/beta\t\n" +
				"owner/zeta\tLast by name\tno\t2\t2024-05-01T12:00:00Z\thttps://github.com/owner/zeta\t\n",
		},
		{
			name: "stars descending",
			argv: []string{"repos", "UL_1", "--sort", "stars", "--desc", "--tsv"},
			want: "owner/Alpha\tFirst by name\tno\t5\t2024-05-03T12:00:00Z\thttps://github.com/owner/Alpha\t\n" +
				"owner/beta\tMiddle by name\tno\t3\t2024-05-02T12:00:00Z\thttps://github.com/owner/beta\t\n" +
				"owner/zeta\tLast by name\tno\t2\t2024-05-01T12:00:00Z\thttps://github.com/owner/zeta\t\n",
		},
		{
			name: "pushed ascending",
			argv: []string{"repos", "UL_1", "--sort", "pushed", "--tsv"},
			want: "owner/zeta\tLast by name\tno\t2\t2024-05-01T12:00:00Z\thttps://github.com/owner/zeta\t\n" +
				"owner/beta\tMiddle by name\tno\t3\t2024-05-02T12:00:00Z\thttps://github.com/owner/beta\t\n" +
				"owner/Alpha\tFirst by name\tno\t5\t2024-05-03T12:00:00Z\thttps://github.com/owner/Alpha\t\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := sortableFixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run(%q) exit = %d, want %d; stderr=%q",
					tt.argv,
					code,
					command.ExitSuccess,
					stderr.String(),
				)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
		})
	}
}

func TestRunReposResolvesNameToID(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	var stdout, stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"repos", "Go Tools", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	if svc.listCalls != 1 {
		t.Fatalf("list calls = %d, want 1 (resolveListID)", svc.listCalls)
	}
	if len(svc.reposListIDs) != 1 || svc.reposListIDs[0] != "UL_1" {
		t.Fatalf("repos called with listID = %q, want UL_1", svc.reposListIDs)
	}
}

func TestRunReposFallsBackToID(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	var stdout, stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"repos", "nonexistent-name", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	if len(svc.reposListIDs) != 1 || svc.reposListIDs[0] != "nonexistent-name" {
		t.Fatalf("repos called with listID = %q, want nonexistent-name", svc.reposListIDs)
	}
}

func TestRunLimitsOutput(t *testing.T) {
	t.Parallel()

	svc := sortableFixtureService()
	var stdout, stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"list", "--limit", "2", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	want := "zeta\tLast by name\t1\t2024-05-03T12:00:00Z\tUL_3\thttps://github.com/stars/maoyeedy/lists/zeta\n" +
		"Alpha\tFirst by name\t5\t2024-05-02T12:00:00Z\tUL_2\thttps://github.com/stars/maoyeedy/lists/alpha\n"
	if got := stdout.String(); got != want {
		t.Fatalf("Run stdout mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRunLimitsOutputAfterSort(t *testing.T) {
	t.Parallel()

	svc := sortableFixtureService()
	var stdout, stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"list", "--sort", "name", "--limit", "2", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	// Sorted by name asc: Alpha, beta, zeta. Limit 2 should return first 2.
	want := "Alpha\tFirst by name\t5\t2024-05-02T12:00:00Z\tUL_2\thttps://github.com/stars/maoyeedy/lists/alpha\n" +
		"beta\tMiddle by name\t3\t2024-05-01T12:00:00Z\tUL_1\thttps://github.com/stars/maoyeedy/lists/beta\n"
	if got := stdout.String(); got != want {
		t.Fatalf("Run stdout mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRunWritesOutputFile(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	dir := t.TempDir()
	outPath := dir + "/output.txt"
	var stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"list", "--output", outPath},
		io.Discard,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "Go Tools") {
		t.Fatalf("output file content = %q, want Go Tools", string(data))
	}
}

func TestRunFiltersListOutput(t *testing.T) {
	t.Parallel()

	svc := filterableFixtureService()
	var stdout, stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"list", "--filter", "name:go", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	want := "Go Tools\tCLI helpers\t10\t2024-05-01T12:00:00Z\tUL_1\thttps://github.com/stars/maoyeedy/lists/go-tools\n" +
		"Go Web\tWeb frameworks\t5\t2024-05-02T12:00:00Z\tUL_2\thttps://github.com/stars/maoyeedy/lists/go-web\n"
	if got := stdout.String(); got != want {
		t.Fatalf("Run stdout mismatch\ngot:  %q\nwant: %q", got, want)
	}
	if svc.listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", svc.listCalls)
	}
}

func TestRunFiltersReposOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "filter name contains go",
			argv: []string{"repos", "UL_1", "--filter", "name:go", "--tsv"},
			want: "owner/go-lib\tGo library\tno\t100\t2024-05-01T12:00:00Z\thttps://github.com/owner/go-lib\t\n" +
				"owner/go-app\tGo app\tno\t300\t2024-05-03T12:00:00Z\thttps://github.com/owner/go-app\t\n",
		},
		{
			name: "filter non-fork",
			argv: []string{"repos", "UL_1", "--filter", "fork:false", "--tsv"},
			want: "owner/go-lib\tGo library\tno\t100\t2024-05-01T12:00:00Z\thttps://github.com/owner/go-lib\t\n" +
				"owner/go-app\tGo app\tno\t300\t2024-05-03T12:00:00Z\thttps://github.com/owner/go-app\t\n",
		},
		{
			name: "filter fork true",
			argv: []string{"repos", "UL_1", "--filter", "fork:true", "--tsv"},
			want: "owner/rust-tool\tRust tool\tyes\t200\t2024-05-02T12:00:00Z\thttps://github.com/owner/rust-tool\t\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := filterableFixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run(%q) exit = %d, want %d; stderr=%q",
					tt.argv,
					code,
					command.ExitSuccess,
					stderr.String(),
				)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
		})
	}
}

func TestRunFiltersCombined(t *testing.T) {
	t.Parallel()

	svc := filterableFixtureService()
	var stdout, stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"repos", "UL_1", "--filter", "name:go", "--filter", "fork:true", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	// AND logic: name contains "go" AND fork=true - no match in fixture
	if stdout.String() != "" {
		t.Fatalf("expected empty output for AND filter, got %q", stdout.String())
	}
}

func TestRunFiltersEmptyResult(t *testing.T) {
	t.Parallel()

	svc := filterableFixtureService()
	var stdout, stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"list", "--filter", "name:nonexistent", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	if stdout.String() != "" {
		t.Fatalf("expected empty output, got %q", stdout.String())
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

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run(%q) exit = %d, want %d; stderr=%q",
					tt.argv,
					code,
					command.ExitSuccess,
					stderr.String(),
				)
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
		{
			name:    "missing repos id",
			argv:    []string{"repos"},
			wantErr: "error: missing list id for repos",
		},
		{name: "unknown command", argv: []string{"stars"}, wantErr: "unknown command \"stars\""},
		{
			name:    "extra list args",
			argv:    []string{"list", "extra"},
			wantErr: "too many arguments for list",
		},
		{
			name:    "extra repos args",
			argv:    []string{"repos", "UL_1", "extra"},
			wantErr: "too many arguments for repos",
		},
		{
			name:    "conflicting output flags",
			argv:    []string{"list", "--json", "--tsv"},
			wantErr: "cannot combine --plain, --json, and --tsv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &fakeService{
				listErr:  errors.New("not implemented"),
				reposErr: errors.New("not implemented"),
			}
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

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
				t.Fatalf(
					"service calls on usage error: list=%d repos=%d",
					svc.listCalls,
					svc.reposCalls,
				)
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
			name: "explicit list auth failure",
			argv: []string{"list"},
			svc: &fakeService{
				listErr: errors.New("GitHub GraphQL request failed: Bad credentials"),
			},
			wantErr: []string{
				"error: failed to list Star Lists: GitHub GraphQL request failed: Bad credentials",
				"gh auth status",
			},
			wantList: 1,
		},
		{
			name: "repos graphql failure",
			argv: []string{"repos", "UL_kwDOExample"},
			svc: &fakeService{
				reposErr: errors.New("GitHub GraphQL request failed: secondary rate limit"),
			},
			wantErr: []string{
				"error: failed to list repositories for Star List \"UL_kwDOExample\": GitHub GraphQL request failed: secondary rate limit",
			},
			wantRepos: 1,
			wantList:  1,
		},
		{
			name: "repos inaccessible list",
			argv: []string{"repos", "UL_missing"},
			svc:  &fakeService{reposErr: githubapi.ErrInaccessibleList},
			wantErr: []string{
				"error: failed to list repositories for Star List \"UL_missing\": GitHub Star List is inaccessible or is not a UserList",
				"deleted, private, inaccessible to this account, or from another GitHub account",
			},
			wantRepos: 1,
			wantList:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, tt.svc)

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
				t.Fatalf(
					"service calls list=%d repos=%d, want list=%d repos=%d",
					tt.svc.listCalls,
					tt.svc.reposCalls,
					tt.wantList,
					tt.wantRepos,
				)
			}
		})
	}
}

func TestRunNilServiceReturnsFailure(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder

	code := runCommand(context.Background(), []string{"list"}, &stdout, &stderr, nil)

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

func TestRunUnlistedRepos(t *testing.T) {
	t.Parallel()

	// Two lists with three repos total; starred set has five repos.
	// Two starred repos (owner/d and owner/e) are not in any list.
	svc := &fakeService{
		lists: []githubapi.StarList{
			{Name: "List A", ID: "UL_1", URL: "https://github.com/stars/user/lists/a"},
			{Name: "List B", ID: "UL_2", URL: "https://github.com/stars/user/lists/b"},
		},
		repos: []githubapi.Repository{
			{NameWithOwner: "owner/a", URL: "https://github.com/owner/a"},
			{NameWithOwner: "owner/b", URL: "https://github.com/owner/b"},
			{NameWithOwner: "owner/c", URL: "https://github.com/owner/c"},
		},
		starred: []githubapi.Repository{
			{
				NameWithOwner: "owner/a",
				StarredAt:     "2026-05-01T00:00:00Z",
				URL:           "https://github.com/owner/a",
			},
			{
				NameWithOwner: "owner/b",
				StarredAt:     "2026-04-01T00:00:00Z",
				URL:           "https://github.com/owner/b",
			},
			{
				NameWithOwner: "owner/c",
				StarredAt:     "2026-03-01T00:00:00Z",
				URL:           "https://github.com/owner/c",
			},
			{
				NameWithOwner: "owner/d",
				StarredAt:     "2026-02-01T00:00:00Z",
				URL:           "https://github.com/owner/d",
			},
			{
				NameWithOwner: "owner/e",
				StarredAt:     "2026-01-01T00:00:00Z",
				URL:           "https://github.com/owner/e",
			},
		},
	}

	var stdout, stderr strings.Builder
	code := runCommand(
		context.Background(),
		[]string{"repos", "--unlisted", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	want := "owner/d\t\tno\t0\t\thttps://github.com/owner/d\t\n" +
		"owner/e\t\tno\t0\t\thttps://github.com/owner/e\t\n"
	if got := stdout.String(); got != want {
		t.Fatalf("Run --unlisted stdout mismatch\ngot:  %q\nwant: %q", got, want)
	}
	// Both lists fetched for their repos.
	if svc.reposCalls != 2 {
		t.Fatalf("repos calls = %d, want 2 (one per list)", svc.reposCalls)
	}
	if svc.starredCalls != 1 {
		t.Fatalf("starred calls = %d, want 1", svc.starredCalls)
	}
}

func TestRunUnlistedSortedByStarred(t *testing.T) {
	t.Parallel()

	svc := &fakeService{
		lists: []githubapi.StarList{
			{Name: "List A", ID: "UL_1", URL: "https://github.com/stars/user/lists/a"},
		},
		repos: []githubapi.Repository{
			{NameWithOwner: "owner/a", URL: "https://github.com/owner/a"},
		},
		starred: []githubapi.Repository{
			{
				NameWithOwner: "owner/a",
				StarredAt:     "2026-05-01T00:00:00Z",
				URL:           "https://github.com/owner/a",
			},
			{
				NameWithOwner: "owner/b",
				StarredAt:     "2026-02-01T00:00:00Z",
				URL:           "https://github.com/owner/b",
			},
			{
				NameWithOwner: "owner/c",
				StarredAt:     "2026-04-01T00:00:00Z",
				URL:           "https://github.com/owner/c",
			},
		},
	}

	var stdout, stderr strings.Builder
	code := runCommand(
		context.Background(),
		[]string{"repos", "--unlisted", "--sort", "starred", "--desc", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("Run exit = %d, want %d; stderr=%q", code, command.ExitSuccess, stderr.String())
	}
	// owner/c starred 2026-04, owner/b starred 2026-02; desc order = c first
	want := "owner/c\t\tno\t0\t\thttps://github.com/owner/c\t\n" +
		"owner/b\t\tno\t0\t\thttps://github.com/owner/b\t\n"
	if got := stdout.String(); got != want {
		t.Fatalf("Run --unlisted --sort starred stdout mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRunWebOpensListURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		argv    []string
		wantURL string
	}{
		{
			name:    "by name resolves URL",
			argv:    []string{"repos", "Go Tools", "--web"},
			wantURL: "https://github.com/stars/maoyeedy/lists/go-tools",
		},
		{
			name:    "by ID falls back to raw ID",
			argv:    []string{"repos", "UL_UNKNOWN", "--web"},
			wantURL: "UL_UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := fixtureService()
			var browsed string
			orig := command.OpenBrowserForTest(func(url string) error {
				browsed = url
				return nil
			})
			defer command.OpenBrowserForTest(orig)

			var stdout, stderr strings.Builder
			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run exit = %d, want %d; stderr=%q",
					code,
					command.ExitSuccess,
					stderr.String(),
				)
			}
			if browsed != tt.wantURL {
				t.Fatalf("openBrowser URL = %q, want %q", browsed, tt.wantURL)
			}
			if svc.reposCalls != 0 {
				t.Fatalf(
					"repos API calls = %d, want 0 (web should not fetch repos)",
					svc.reposCalls,
				)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestRunSortListByRepoCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "repos count ascending",
			argv: []string{"list", "--sort", "repos", "--tsv"},
			// zeta=1, beta=3, Alpha=5
			want: "zeta\tLast by name\t1\t2024-05-03T12:00:00Z\tUL_3\thttps://github.com/stars/maoyeedy/lists/zeta\n" +
				"beta\tMiddle by name\t3\t2024-05-01T12:00:00Z\tUL_1\thttps://github.com/stars/maoyeedy/lists/beta\n" +
				"Alpha\tFirst by name\t5\t2024-05-02T12:00:00Z\tUL_2\thttps://github.com/stars/maoyeedy/lists/alpha\n",
		},
		{
			name: "repos count descending",
			argv: []string{"list", "--sort", "repos", "--desc", "--tsv"},
			// Alpha=5, beta=3, zeta=1
			want: "Alpha\tFirst by name\t5\t2024-05-02T12:00:00Z\tUL_2\thttps://github.com/stars/maoyeedy/lists/alpha\n" +
				"beta\tMiddle by name\t3\t2024-05-01T12:00:00Z\tUL_1\thttps://github.com/stars/maoyeedy/lists/beta\n" +
				"zeta\tLast by name\t1\t2024-05-03T12:00:00Z\tUL_3\thttps://github.com/stars/maoyeedy/lists/zeta\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := sortableFixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run(%q) exit = %d, want %d; stderr=%q",
					tt.argv,
					code,
					command.ExitSuccess,
					stderr.String(),
				)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
		})
	}
}

func TestRunFilterAndSortByLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "filter language Go",
			argv: []string{"repos", "UL_1", "--filter", "language:Go", "--tsv"},
			want: "owner/go-tool\tA Go tool\tno\t300\t2024-05-03T12:00:00Z\thttps://github.com/owner/go-tool\tGo\n" +
				"owner/go-web\tA Go web\tno\t100\t2024-05-01T12:00:00Z\thttps://github.com/owner/go-web\tGo\n",
		},
		{
			name: "filter language case insensitive",
			argv: []string{"repos", "UL_1", "--filter", "language:go", "--tsv"},
			want: "owner/go-tool\tA Go tool\tno\t300\t2024-05-03T12:00:00Z\thttps://github.com/owner/go-tool\tGo\n" +
				"owner/go-web\tA Go web\tno\t100\t2024-05-01T12:00:00Z\thttps://github.com/owner/go-web\tGo\n",
		},
		{
			name: "sort by language ascending",
			argv: []string{"repos", "UL_1", "--sort", "language", "--tsv"},
			want: "owner/go-tool\tA Go tool\tno\t300\t2024-05-03T12:00:00Z\thttps://github.com/owner/go-tool\tGo\n" +
				"owner/go-web\tA Go web\tno\t100\t2024-05-01T12:00:00Z\thttps://github.com/owner/go-web\tGo\n" +
				"owner/rust-lib\tA Rust lib\tno\t200\t2024-05-02T12:00:00Z\thttps://github.com/owner/rust-lib\tRust\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := languageFixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf(
					"Run(%q) exit = %d, want %d; stderr=%q",
					tt.argv,
					code,
					command.ExitSuccess,
					stderr.String(),
				)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("Run(%q) stdout mismatch\ngot:  %q\nwant: %q", tt.argv, got, tt.want)
			}
		})
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
		{
			name:    "help",
			argv:    []string{"--help"},
			svc:     &fakeService{},
			wantErr: "failed to write help",
		},
		{
			name:    "data output",
			argv:    []string{"list"},
			svc:     fixtureService(),
			wantErr: "failed to write output",
		},
		{
			name:    "list json output",
			argv:    []string{"list", "--json"},
			svc:     fixtureService(),
			wantErr: "failed to write output",
		},
		{
			name:    "repos json output",
			argv:    []string{"repos", "UL_1", "--json"},
			svc:     fixtureService(),
			wantErr: "failed to write output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, errWriter{}, &stderr, tt.svc)

			if code != command.ExitFailure {
				t.Fatalf(
					"Run %s write failure exit = %d, want %d",
					tt.name,
					code,
					command.ExitFailure,
				)
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("stderr = %q, want %q diagnostic", stderr.String(), tt.wantErr)
			}
		})
	}
}
