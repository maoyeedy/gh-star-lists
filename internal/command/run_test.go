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
	listCalls           int
	reposCalls          int
	starredCalls        int
	getRepoCalls        int
	createCalls         int
	updateCalls         int
	deleteCalls         int
	updateRepoListCalls int
	addStarCalls        int
	removeStarCalls     int
	reposListIDs        []string
	lists               []githubapi.StarList
	repos               []githubapi.Repository
	reposByList         map[string][]githubapi.Repository
	starred             []githubapi.Repository
	gotRepo             githubapi.Repository
	createdList         githubapi.StarList
	updatedList         githubapi.StarList
	createdInput        githubapi.StarListInput
	updatedInput        githubapi.UpdateStarListInput
	updatedRepoID       string
	updatedListIDs      []string
	deletedListID       string
	addedStarID         string
	removedStarID       string
	listErr             error
	reposErr            error
	starredErr          error
	getRepoErr          error
	createErr           error
	updateErr           error
	deleteErr           error
	updateRepoListErr   error
	addStarErr          error
	removeStarErr       error
}

func (f *fakeService) ListStarLists(
	context.Context,
	...githubapi.ListOptions,
) ([]githubapi.StarList, error) {
	f.listCalls++
	return f.lists, f.listErr
}

func (f *fakeService) ListRepositories(
	_ context.Context,
	listID string,
	_ ...githubapi.ListOptions,
) ([]githubapi.Repository, error) {
	f.reposCalls++
	f.reposListIDs = append(f.reposListIDs, listID)
	if f.reposByList != nil {
		return f.reposByList[listID], f.reposErr
	}
	return f.repos, f.reposErr
}

func (f *fakeService) ListStarredRepositories(
	_ context.Context,
	_ ...githubapi.ListOptions,
) ([]githubapi.Repository, error) {
	f.starredCalls++
	return f.starred, f.starredErr
}

func (f *fakeService) GetRepository(
	_ context.Context,
	nameWithOwner string,
) (githubapi.Repository, error) {
	f.getRepoCalls++
	if f.getRepoErr != nil {
		return githubapi.Repository{}, f.getRepoErr
	}
	if f.gotRepo.ID != "" {
		return f.gotRepo, nil
	}
	return githubapi.Repository{ID: "R_1", NameWithOwner: nameWithOwner}, nil
}

func (f *fakeService) GetRepositoryMemberships(
	_ context.Context,
	nameWithOwner string,
) (string, []string, error) {
	if f.getRepoErr != nil {
		return "", nil, f.getRepoErr
	}
	repoID := "R_1"
	if f.gotRepo.ID != "" {
		repoID = f.gotRepo.ID
	}
	var listIDs []string
	for listID, repos := range f.reposByList {
		for _, repo := range repos {
			if repo.NameWithOwner == nameWithOwner {
				listIDs = append(listIDs, listID)
				if repo.ID != "" {
					repoID = repo.ID
				}
				break
			}
		}
	}
	return repoID, listIDs, nil
}

func (f *fakeService) CreateStarList(
	_ context.Context,
	input githubapi.StarListInput,
) (githubapi.StarList, error) {
	f.createCalls++
	f.createdInput = input
	if f.createErr != nil {
		return githubapi.StarList{}, f.createErr
	}
	if f.createdList.ID != "" {
		return f.createdList, nil
	}
	return githubapi.StarList{Name: input.Name, ID: "UL_new"}, nil
}

func (f *fakeService) UpdateStarList(
	_ context.Context,
	input githubapi.UpdateStarListInput,
) (githubapi.StarList, error) {
	f.updateCalls++
	f.updatedInput = input
	if f.updateErr != nil {
		return githubapi.StarList{}, f.updateErr
	}
	if f.updatedList.ID != "" {
		return f.updatedList, nil
	}
	return githubapi.StarList{Name: input.Name, ID: input.ID}, nil
}

func (f *fakeService) DeleteStarList(_ context.Context, listID string) error {
	f.deleteCalls++
	f.deletedListID = listID
	return f.deleteErr
}

func (f *fakeService) UpdateRepositoryLists(
	_ context.Context,
	repoID string,
	listIDs []string,
) error {
	f.updateRepoListCalls++
	f.updatedRepoID = repoID
	f.updatedListIDs = append([]string(nil), listIDs...)
	return f.updateRepoListErr
}

func (f *fakeService) AddStar(_ context.Context, repoID string) error {
	f.addStarCalls++
	f.addedStarID = repoID
	return f.addStarErr
}

func (f *fakeService) RemoveStar(_ context.Context, repoID string) error {
	f.removeStarCalls++
	f.removedStarID = repoID
	return f.removeStarErr
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
			name: "fzf",
			argv: []string{"list", "--fzf"},
			want: "Go Tools\tUL_1\t3\thttps://github.com/stars/maoyeedy/lists/go-tools\tCLI helpers\t2024-05-01T12:00:00Z\n",
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
		{
			name: "fzf",
			argv: []string{"repos", "UL_1", "--fzf"},
			want: "cli/cli\t41000\t\thttps://github.com/cli/cli\tGitHub CLI\t2024-05-01T12:00:00Z\tno\n",
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
		{
			name: "empty lists human",
			argv: []string{"list"},
			want: "No Star Lists found.\nCreate one with `gh star-lists create <NAME>`.\n",
		},
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
		name      string
		argv      []string
		wantErr   string
		wantUsage bool
	}{
		{
			name:      "missing repos id",
			argv:      []string{"repos"},
			wantErr:   "repos requires <LIST_ID_OR_NAME>",
			wantUsage: true,
		},
		{name: "unknown command", argv: []string{"stars"}, wantErr: "unknown command \"stars\""},
		{
			name:      "extra list args",
			argv:      []string{"list", "extra"},
			wantErr:   "too many arguments for list",
			wantUsage: true,
		},
		{
			name:      "extra repos args",
			argv:      []string{"repos", "UL_1", "extra"},
			wantErr:   "too many arguments for repos",
			wantUsage: true,
		},
		{
			name:      "conflicting output flags",
			argv:      []string{"list", "--json", "--tsv"},
			wantErr:   "cannot combine --plain, --json, --tsv, and --fzf",
			wantUsage: true,
		},
		{
			name:      "conflicting fzf and json",
			argv:      []string{"list", "--fzf", "--json"},
			wantErr:   "cannot combine --plain, --json, --tsv, and --fzf",
			wantUsage: true,
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
			if !strings.Contains(gotErr, tt.wantErr) ||
				(tt.wantUsage && !strings.Contains(gotErr, "Usage:")) {
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

func TestRunWebErrors(t *testing.T) {
	t.Run("list API error", func(t *testing.T) {
		svc := fixtureService()
		svc.listErr = errors.New("GitHub GraphQL request failed: forbidden")
		orig := command.OpenBrowserForTest(func(string) error { return nil })
		defer command.OpenBrowserForTest(orig)

		var stdout, stderr strings.Builder
		code := runCommand(
			context.Background(),
			[]string{"repos", "Go Tools", "--web"},
			&stdout,
			&stderr,
			svc,
		)

		if code != command.ExitFailure {
			t.Fatalf("exit = %d, want ExitFailure; stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "GitHub GraphQL request failed") {
			t.Fatalf("stderr = %q, want GraphQL error", stderr.String())
		}
	})

	t.Run("open browser error", func(t *testing.T) {
		svc := fixtureService()
		orig := command.OpenBrowserForTest(func(string) error {
			return errors.New("browse failed")
		})
		defer command.OpenBrowserForTest(orig)

		var stdout, stderr strings.Builder
		code := runCommand(
			context.Background(),
			[]string{"repos", "Go Tools", "--web"},
			&stdout,
			&stderr,
			svc,
		)

		if code != command.ExitFailure {
			t.Fatalf("exit = %d, want ExitFailure; stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "failed to open browser") {
			t.Fatalf("stderr = %q, want browser failure diagnostic", stderr.String())
		}
	})
}

func TestRunAllStarredRepos(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	svc.starred = []githubapi.Repository{
		{
			NameWithOwner:  "cli/cli",
			Description:    "GitHub CLI",
			StargazerCount: 41000,
			PushedAt:       "2024-05-01T12:00:00Z",
			URL:            "https://github.com/cli/cli",
		},
	}
	var stdout, stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"repos", "--all", "--tsv"},
		&stdout,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if svc.starredCalls != 1 {
		t.Fatalf("starredCalls = %d, want 1", svc.starredCalls)
	}
	if svc.reposCalls != 0 {
		t.Fatalf("reposCalls = %d, want 0 (--all skips list repos)", svc.reposCalls)
	}
	if !strings.Contains(stdout.String(), "cli/cli") {
		t.Fatalf("stdout = %q, want cli/cli", stdout.String())
	}
}

func TestRunJQ(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		argv    []string
		wantOut string
	}{
		{
			name:    "list name field",
			argv:    []string{"list", "--jq", ".[].name"},
			wantOut: "Go Tools\n",
		},
		{
			name:    "repos nameWithOwner field",
			argv:    []string{"repos", "UL_1", "--jq", ".[].nameWithOwner"},
			wantOut: "cli/cli\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := fixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
			}
			if got := stdout.String(); got != tt.wantOut {
				t.Fatalf("stdout = %q, want %q", got, tt.wantOut)
			}
		})
	}
}

func TestRunOutputFileError(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	outPath := t.TempDir() + "/missing/out.txt"
	var stderr strings.Builder

	code := runCommand(
		context.Background(),
		[]string{"list", "--output", outPath},
		io.Discard,
		&stderr,
		svc,
	)

	if code != command.ExitFailure {
		t.Fatalf("exit = %d, want ExitFailure", code)
	}
	if !strings.Contains(stderr.String(), "failed to open output file") {
		t.Fatalf("stderr = %q, want output file error diagnostic", stderr.String())
	}
}

func TestRunOutputFileExistsWithYesOverwrites(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	dir := t.TempDir()
	outPath := dir + "/output.txt"

	if err := os.WriteFile(outPath, []byte("old content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stderr strings.Builder
	code := runCommand(
		context.Background(),
		[]string{"list", "--output", outPath, "--yes"},
		io.Discard,
		&stderr,
		svc,
	)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "old content") {
		t.Fatalf("output file still contains old content after --yes overwrite")
	}
}

func TestRunOutputFileExistsNoYesNonTTYFails(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	dir := t.TempDir()
	outPath := dir + "/output.txt"

	if err := os.WriteFile(outPath, []byte("old content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stderr strings.Builder
	code := runCommand(
		context.Background(),
		[]string{"list", "--output", outPath},
		io.Discard,
		&stderr,
		svc,
	)

	if code != command.ExitFailure {
		t.Fatalf("exit = %d, want ExitFailure", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr = %q, want 'already exists' diagnostic", stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "old content" {
		t.Fatalf("output file was modified despite failure; got %q", string(data))
	}
}

func TestRunDryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mkSvc    func() *fakeService
		argv     []string
		wantOut  string
		wantOut2 string
		checkFn  func(*testing.T, *fakeService)
	}{
		{
			name:    "create",
			mkSvc:   fixtureService,
			argv:    []string{"create", "newlist", "--dry-run"},
			wantOut: `Would create Star List "newlist".`,
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.createCalls != 0 {
					t.Fatalf("createCalls = %d, want 0", svc.createCalls)
				}
			},
		},
		{
			name:    "edit",
			mkSvc:   fixtureService,
			argv:    []string{"edit", "Go Tools", "--name", "x", "--dry-run"},
			wantOut: `Would update Star List "Go Tools".`,
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.updateCalls != 0 {
					t.Fatalf("updateCalls = %d, want 0", svc.updateCalls)
				}
			},
		},
		{
			name:    "delete",
			mkSvc:   fixtureService,
			argv:    []string{"delete", "Go Tools", "--dry-run"},
			wantOut: `Would delete Star List "Go Tools".`,
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.deleteCalls != 0 {
					t.Fatalf("deleteCalls = %d, want 0", svc.deleteCalls)
				}
			},
		},
		{
			name:    "add",
			mkSvc:   fixtureService,
			argv:    []string{"add", "cli/cli", "--to", "Go Tools", "--dry-run"},
			wantOut: `Would add cli/cli to "Go Tools".`,
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.updateRepoListCalls != 0 {
					t.Fatalf("updateRepoListCalls = %d, want 0", svc.updateRepoListCalls)
				}
			},
		},
		{
			name:    "remove",
			mkSvc:   fixtureService,
			argv:    []string{"remove", "cli/cli", "--from", "Go Tools", "--dry-run"},
			wantOut: `Would remove cli/cli from "Go Tools".`,
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.updateRepoListCalls != 0 {
					t.Fatalf("updateRepoListCalls = %d, want 0", svc.updateRepoListCalls)
				}
			},
		},
		{
			name:    "move",
			mkSvc:   sortableFixtureService,
			argv:    []string{"move", "cli/cli", "--from", "zeta", "--to", "Alpha", "--dry-run"},
			wantOut: `Would move cli/cli from "zeta" to "Alpha".`,
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.updateRepoListCalls != 0 {
					t.Fatalf("updateRepoListCalls = %d, want 0", svc.updateRepoListCalls)
				}
			},
		},
		{
			name:    "copy",
			mkSvc:   sortableFixtureService,
			argv:    []string{"copy", "--from", "zeta", "--to", "Alpha", "--dry-run"},
			wantOut: `Would copy 3 repositories from "zeta" to "Alpha".`,
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.updateRepoListCalls != 0 {
					t.Fatalf("updateRepoListCalls = %d, want 0", svc.updateRepoListCalls)
				}
			},
		},
		{
			name:     "merge",
			mkSvc:    sortableFixtureService,
			argv:     []string{"merge", "--from", "zeta", "--to", "Alpha", "--dry-run"},
			wantOut:  `Would merge 3 repositories from "zeta" to "Alpha".`,
			wantOut2: `Would delete source Star List "zeta".`,
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.deleteCalls != 0 {
					t.Fatalf("deleteCalls = %d, want 0", svc.deleteCalls)
				}
			},
		},
		{
			name:    "unstar",
			mkSvc:   fixtureService,
			argv:    []string{"unstar", "cli/cli", "--dry-run"},
			wantOut: "Would unstar cli/cli.",
			checkFn: func(t *testing.T, svc *fakeService) {
				if svc.removeStarCalls != 0 {
					t.Fatalf("removeStarCalls = %d, want 0", svc.removeStarCalls)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := tt.mkSvc()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
			}
			got := stdout.String()
			if !strings.Contains(got, tt.wantOut) {
				t.Fatalf("stdout = %q, want %q", got, tt.wantOut)
			}
			if tt.wantOut2 != "" && !strings.Contains(got, tt.wantOut2) {
				t.Fatalf("stdout = %q, want %q", got, tt.wantOut2)
			}
			if tt.checkFn != nil {
				tt.checkFn(t, svc)
			}
		})
	}
}

func TestRunUnlistedEmpty(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	svc.starred = []githubapi.Repository{
		{NameWithOwner: "cli/cli", URL: "https://github.com/cli/cli"},
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
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if svc.starredCalls != 1 {
		t.Fatalf("starredCalls = %d, want 1", svc.starredCalls)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty (all starred repos already in lists)", stdout.String())
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

func TestRunNestedHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		argv     []string
		wantText string
		wantNot  string
	}{
		{
			name:     "top-level help shows compact",
			argv:     []string{"--help"},
			wantText: "gh star-lists <command> --help",
			wantNot:  "--cache-ttl",
		},
		{
			name:     "repos help shows repos section",
			argv:     []string{"repos", "--help"},
			wantText: "--unlisted",
			wantNot:  "gh star-lists <command> --help",
		},
		{
			name:     "add help shows --to flag",
			argv:     []string{"add", "--help"},
			wantText: "--to <LIST_ID_OR_NAME>",
		},
		{
			name:     "remove help shows --from flag",
			argv:     []string{"remove", "--help"},
			wantText: "--from <LIST_ID_OR_NAME>",
		},
		{
			name:     "full flag shows full reference",
			argv:     []string{"--full"},
			wantText: "--cache-ttl",
		},
		{
			name:     "alias ls help shows list section",
			argv:     []string{"ls", "--help"},
			wantText: "gh star-lists list",
		},
		{
			name:     "nil service still works for help",
			argv:     []string{"--help"},
			wantText: "gh star-lists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &fakeService{
				listErr:  errors.New("must not call service"),
				reposErr: errors.New("must not call service"),
			}
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("help stderr = %q, want empty", stderr.String())
			}
			if svc.listCalls != 0 || svc.reposCalls != 0 {
				t.Fatalf("service calls on help: list=%d repos=%d", svc.listCalls, svc.reposCalls)
			}
			if got := stdout.String(); !strings.Contains(got, tt.wantText) {
				t.Fatalf("stdout missing %q:\n%s", tt.wantText, got)
			}
			if tt.wantNot != "" {
				if got := stdout.String(); strings.Contains(got, tt.wantNot) {
					t.Fatalf("stdout should not contain %q but does:\n%s", tt.wantNot, got)
				}
			}
		})
	}
}

func TestRunNonTTYMissingListSelectorFailsWithUsageError(t *testing.T) {
	t.Parallel()

	// In test env canPrompt() is false (no TTY), so missing selectors return ExitUsage.
	tests := []struct {
		name    string
		argv    []string
		wantErr string
	}{
		{
			name:    "add without --to",
			argv:    []string{"add", "owner/repo"},
			wantErr: "add requires --to",
		},
		{
			name:    "remove without --from",
			argv:    []string{"remove", "owner/repo"},
			wantErr: "remove requires --from",
		},
		{
			name:    "move without flags",
			argv:    []string{"move", "owner/repo"},
			wantErr: "move requires --from and --to",
		},
		{
			name:    "copy without flags",
			argv:    []string{"copy"},
			wantErr: "copy requires --from and --to",
		},
		{
			name:    "merge without flags",
			argv:    []string{"merge"},
			wantErr: "merge requires --from and --to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := fixtureService()
			var stdout, stderr strings.Builder

			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitUsage {
				t.Fatalf("exit = %d, want ExitUsage; stderr=%q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.wantErr)
			}
		})
	}
}

func TestRunPromptForListOnAdd(t *testing.T) {
	svc := &fakeService{
		lists: []githubapi.StarList{
			{Name: "Go Tools", ID: "UL_1", RepoCount: 3},
			{Name: "Rust Libs", ID: "UL_2", RepoCount: 1},
		},
	}

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)
	var promptedLabel string
	var promptedChoices []string
	prevPromptForList := command.PromptForListForTest(
		func(label, _ string, choices []string) (int, error) {
			promptedLabel = label
			promptedChoices = choices
			return 1, nil // pick "Rust Libs"
		},
	)
	defer command.PromptForListForTest(prevPromptForList)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"add", "cli/cli"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if svc.updateRepoListCalls != 1 {
		t.Fatalf("updateRepoListCalls = %d, want 1", svc.updateRepoListCalls)
	}
	if len(svc.updatedListIDs) == 0 || svc.updatedListIDs[0] != "UL_2" {
		t.Fatalf("updated list IDs = %v, want [UL_2]", svc.updatedListIDs)
	}
	if !strings.Contains(promptedLabel, "--to") {
		t.Fatalf("prompt label = %q, want mention of --to", promptedLabel)
	}
	if len(promptedChoices) != 2 {
		t.Fatalf("prompt choices = %v, want 2 options", promptedChoices)
	}
}

func TestRunPromptCancelledExitsCleanly(t *testing.T) {
	svc := fixtureService()

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)
	prevPromptForList := command.PromptForListForTest(
		func(label, _ string, choices []string) (int, error) {
			return 0, command.ErrPromptCancelled
		},
	)
	defer command.PromptForListForTest(prevPromptForList)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"add", "cli/cli"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess on cancellation; stderr=%q", code, stderr.String())
	}
	if svc.updateRepoListCalls != 0 {
		t.Fatalf(
			"updateRepoListCalls = %d, want 0 (no mutation on cancel)",
			svc.updateRepoListCalls,
		)
	}
	if !strings.Contains(stderr.String(), "No changes made") {
		t.Fatalf("stderr = %q, want 'No changes made' message", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on cancellation", stdout.String())
	}
}

func TestRunPromptForMoveExcludesFromInToChoices(t *testing.T) {
	svc := &fakeService{
		lists: []githubapi.StarList{
			{Name: "List A", ID: "UL_1", RepoCount: 1},
			{Name: "List B", ID: "UL_2", RepoCount: 2},
			{Name: "List C", ID: "UL_3", RepoCount: 3},
		},
		reposByList: map[string][]githubapi.Repository{
			"UL_1": {{NameWithOwner: "cli/cli", ID: "R_1"}},
		},
	}

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)

	var toChoices []string
	callCount := 0
	prevPromptForList := command.PromptForListForTest(
		func(label, _ string, choices []string) (int, error) {
			callCount++
			if callCount == 1 {
				// --from prompt: return index 0 = "List A (UL_1)"
				return 0, nil
			}
			// --to prompt: choices should not include List A
			toChoices = choices
			return 0, nil // pick "List B"
		},
	)
	defer command.PromptForListForTest(prevPromptForList)

	prevConfirmAction := command.ConfirmActionForTest(func(prompt string) (bool, error) {
		return true, nil
	})
	defer command.ConfirmActionForTest(prevConfirmAction)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"move", "cli/cli"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if callCount != 2 {
		t.Fatalf("prompt called %d times, want 2 (from + to)", callCount)
	}
	// --to choices must not include "List A" (the --from selection)
	for _, c := range toChoices {
		if strings.Contains(c, "List A") {
			t.Fatalf("to choices %v should not include List A (the from selection)", toChoices)
		}
	}
	if len(toChoices) != 2 {
		t.Fatalf("to choices = %v, want 2 options (List B + List C)", toChoices)
	}
}

func TestRunDuplicateListNamesIncludeIDInPicker(t *testing.T) {
	svc := &fakeService{
		lists: []githubapi.StarList{
			{Name: "Go Tools", ID: "UL_1", RepoCount: 3},
			{Name: "Go Tools", ID: "UL_2", RepoCount: 1},
			{Name: "Rust Libs", ID: "UL_3", RepoCount: 2},
		},
	}

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)

	var capturedChoices []string
	prevPromptForList := command.PromptForListForTest(
		func(label, _ string, choices []string) (int, error) {
			capturedChoices = choices
			return 0, nil
		},
	)
	defer command.PromptForListForTest(prevPromptForList)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"add", "cli/cli"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}

	if len(capturedChoices) != 3 {
		t.Fatalf("prompt choices = %v, want 3 choices", capturedChoices)
	}

	// "Go Tools" is duplicated, so labels should include IDs
	for _, c := range capturedChoices[:2] {
		if !strings.Contains(c, "UL_") {
			t.Fatalf("duplicate name choice %q should contain list ID", c)
		}
	}

	// "Rust Libs" is unique, label should be compact without ID
	if strings.Contains(capturedChoices[2], "UL_") {
		t.Fatalf("unique name choice %q should not contain list ID", capturedChoices[2])
	}
}

func TestRunPromptForReposList(t *testing.T) {
	svc := &fakeService{
		lists: []githubapi.StarList{
			{Name: "Go Tools", ID: "UL_1", RepoCount: 1},
			{Name: "Rust Libs", ID: "UL_2", RepoCount: 1},
		},
		reposByList: map[string][]githubapi.Repository{
			"UL_2": {{NameWithOwner: "rust-lang/rust", URL: "https://github.com/rust-lang/rust"}},
		},
	}

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)
	prevPromptForList := command.PromptForListForTest(
		func(label, _ string, choices []string) (int, error) {
			if !strings.Contains(label, "Star List") {
				t.Fatalf("prompt label = %q, want Star List", label)
			}
			return 1, nil
		},
	)
	defer command.PromptForListForTest(prevPromptForList)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"repos", "--plain"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if len(svc.reposListIDs) != 1 || svc.reposListIDs[0] != "UL_2" {
		t.Fatalf("reposListIDs = %v, want [UL_2]", svc.reposListIDs)
	}
	if !strings.Contains(stdout.String(), "rust-lang/rust") {
		t.Fatalf("stdout = %q, want selected list repository", stdout.String())
	}
}

func TestRunPromptForCreateInputs(t *testing.T) {
	svc := fixtureService()

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)
	inputs := []string{"New List", "Prompted description"}
	prevPromptInput := command.PromptInputForTest(func(label, defaultValue string) (string, error) {
		if len(inputs) == 0 {
			t.Fatalf("unexpected input prompt %q", label)
		}
		value := inputs[0]
		inputs = inputs[1:]
		return value, nil
	})
	defer command.PromptInputForTest(prevPromptInput)
	prevPromptForList := command.PromptForListForTest(
		func(label, _ string, choices []string) (int, error) {
			if label != "Visibility:" {
				t.Fatalf("visibility prompt label = %q", label)
			}
			return 1, nil
		},
	)
	defer command.PromptForListForTest(prevPromptForList)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"create"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if svc.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", svc.createCalls)
	}
	if svc.createdInput.Name != "New List" ||
		svc.createdInput.Description != "Prompted description" ||
		!svc.createdInput.Private {
		t.Fatalf("createdInput = %+v, want prompted private list", svc.createdInput)
	}
}

func TestRunEditNoSelectionShowsNoChanges(t *testing.T) {
	svc := &fakeService{
		lists: []githubapi.StarList{
			{Name: "Go Tools", ID: "UL_1", RepoCount: 3},
		},
	}

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)
	prevPromptMulti := command.PromptMultiSelectForTest(
		func(label string, defaults, choices []string) ([]int, error) {
			return []int{}, nil
		},
	)
	defer command.PromptMultiSelectForTest(prevPromptMulti)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"edit", "Go Tools"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if svc.updateCalls != 0 {
		t.Fatalf("updateCalls = %d, want 0 (no mutation on no selection)", svc.updateCalls)
	}
	if !strings.Contains(stderr.String(), "No changes made.") {
		t.Fatalf("stderr = %q, want 'No changes made.'", stderr.String())
	}
}

func TestRunPromptForEditFields(t *testing.T) {
	svc := fixtureService()

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)
	prevPromptMulti := command.PromptMultiSelectForTest(
		func(label string, defaults, choices []string) ([]int, error) {
			if !strings.Contains(label, "fields") {
				t.Fatalf("multi prompt label = %q, want fields", label)
			}
			return []int{0, 2}, nil
		},
	)
	defer command.PromptMultiSelectForTest(prevPromptMulti)
	prevPromptInput := command.PromptInputForTest(func(label, defaultValue string) (string, error) {
		if label != "New name:" {
			t.Fatalf("input label = %q, want New name", label)
		}
		return "Renamed", nil
	})
	defer command.PromptInputForTest(prevPromptInput)
	prevPromptForList := command.PromptForListForTest(
		func(label, _ string, choices []string) (int, error) {
			if label != "Visibility:" {
				t.Fatalf("visibility prompt label = %q", label)
			}
			return 0, nil
		},
	)
	defer command.PromptForListForTest(prevPromptForList)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"edit", "Go Tools"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if svc.updateCalls != 1 {
		t.Fatalf("updateCalls = %d, want 1", svc.updateCalls)
	}
	if svc.updatedInput.Name != "Renamed" {
		t.Fatalf("updated name = %q, want Renamed", svc.updatedInput.Name)
	}
	if svc.updatedInput.Private == nil || *svc.updatedInput.Private {
		t.Fatalf("updated private = %v, want false pointer", svc.updatedInput.Private)
	}
	if svc.updatedInput.Description != "" {
		t.Fatalf(
			"updated description = %q, want untouched empty value",
			svc.updatedInput.Description,
		)
	}
}

func TestRunConfirmationPromptNamesTarget(t *testing.T) {
	tests := []struct {
		name       string
		argv       []string
		wantPrompt string
	}{
		{
			name:       "delete names list",
			argv:       []string{"delete", "Go Tools"},
			wantPrompt: `"Go Tools"`,
		},
		{
			name:       "unstar names repo",
			argv:       []string{"unstar", "cli/cli"},
			wantPrompt: "cli/cli",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := fixtureService()
			prevCanPrompt := command.CanPromptForTest(func() bool { return true })
			defer command.CanPromptForTest(prevCanPrompt)

			var capturedPrompt string
			prevConfirm := command.ConfirmActionForTest(func(prompt string) (bool, error) {
				capturedPrompt = prompt
				return true, nil
			})
			defer command.ConfirmActionForTest(prevConfirm)

			var stdout, stderr strings.Builder
			code := runCommand(context.Background(), tt.argv, &stdout, &stderr, svc)

			if code != command.ExitSuccess {
				t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
			}
			if !strings.Contains(capturedPrompt, tt.wantPrompt) {
				t.Fatalf(
					"confirm prompt = %q, want it to contain %q",
					capturedPrompt,
					tt.wantPrompt,
				)
			}
		})
	}
}

func TestRunEditDefaultsPreloaded(t *testing.T) {
	svc := &fakeService{
		lists: []githubapi.StarList{
			{Name: "Go Tools", Description: "CLI helpers", ID: "UL_1", RepoCount: 3},
		},
	}

	prevCanPrompt := command.CanPromptForTest(func() bool { return true })
	defer command.CanPromptForTest(prevCanPrompt)
	prevPromptMulti := command.PromptMultiSelectForTest(
		func(label string, defaults, choices []string) ([]int, error) {
			return []int{0, 1}, nil // select Name and Description
		},
	)
	defer command.PromptMultiSelectForTest(prevPromptMulti)

	var capturedNameDefault, capturedDescDefault string
	prevPromptInput := command.PromptInputForTest(func(label, defaultValue string) (string, error) {
		switch label {
		case "New name:":
			capturedNameDefault = defaultValue
			return "Renamed", nil
		case "New description:":
			capturedDescDefault = defaultValue
			return "Updated desc", nil
		}
		return defaultValue, nil
	})
	defer command.PromptInputForTest(prevPromptInput)

	var stdout, stderr strings.Builder
	code := runCommand(context.Background(), []string{"edit", "Go Tools"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	if capturedNameDefault != "Go Tools" {
		t.Fatalf("name default = %q, want 'Go Tools'", capturedNameDefault)
	}
	if capturedDescDefault != "CLI helpers" {
		t.Fatalf("description default = %q, want 'CLI helpers'", capturedDescDefault)
	}
}

func TestRunNoCacheDisablesCache(t *testing.T) {
	t.Parallel()

	svc := fixtureService()
	var stdout, stderr strings.Builder

	code := runCommand(context.Background(), []string{"list", "--no-cache"}, &stdout, &stderr, svc)

	if code != command.ExitSuccess {
		t.Fatalf("exit = %d, want ExitSuccess; stderr=%q", code, stderr.String())
	}
	// listCalls must be 1: no cache wrapping means direct service hit
	if svc.listCalls != 1 {
		t.Fatalf("listCalls = %d, want 1", svc.listCalls)
	}
}

func TestRunNoColorDisablesColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
	}{
		{name: "usage error no color", argv: []string{"list", "--bad", "--no-color"}},
		{name: "repos usage no color", argv: []string{"repos", "--no-color"}},
	}

	noColorOptions := func(mode format.OutputMode) format.Options {
		return format.Options{Mode: mode, Width: 120, Color: false}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder
			code := command.RunWithOptions(
				context.Background(),
				tt.argv,
				&stdout,
				&stderr,
				fixtureService(),
				noColorOptions,
			)

			if code != command.ExitUsage {
				t.Fatalf("exit = %d, want ExitUsage", code)
			}
			got := stderr.String()
			if strings.Contains(got, "\x1b[") {
				t.Fatalf("stderr with --no-color contains ANSI escape:\n%s", got)
			}
		})
	}
}

func TestRunColorizesHumanDiagnostics(t *testing.T) {
	colorOptions := func(mode format.OutputMode) format.Options {
		return format.Options{Mode: mode, Width: 120, Color: true}
	}
	var stdout, stderr strings.Builder

	code := command.RunWithOptions(
		context.Background(),
		[]string{"list", "--bad"},
		&stdout,
		&stderr,
		fixtureService(),
		colorOptions,
	)

	if code != command.ExitUsage {
		t.Fatalf("exit = %d, want ExitUsage", code)
	}
	if !strings.Contains(stderr.String(), "\x1b[33merror: unknown flag") {
		t.Fatalf("stderr = %q, want yellow usage diagnostic", stderr.String())
	}
}
