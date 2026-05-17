package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cli/go-gh/v2/pkg/browser"
	"github.com/cli/go-gh/v2/pkg/prompter"
	ghterm "github.com/cli/go-gh/v2/pkg/term"
	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"golang.org/x/sync/errgroup"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitUsage   = 2
)

var openBrowser = func(url string) error {
	return browser.New("", os.Stdout, os.Stderr).Browse(url)
}

var canPrompt = func() bool {
	return ghterm.IsTerminal(os.Stdin) && ghterm.IsTerminal(os.Stderr)
}

var confirmAction = func(prompt string) (bool, error) {
	return prompter.New(os.Stdin, os.Stdout, os.Stderr).Confirm(prompt, false)
}

func OpenBrowserForTest(fn func(string) error) func(string) error {
	prev := openBrowser
	openBrowser = fn
	return prev
}

func Run(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	service githubapi.Service,
) int {
	return RunWithOptions(ctx, args, stdout, stderr, service, format.DefaultOptions)
}

func RunWithOptions(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	service githubapi.Service,
	outputOptionsForMode func(format.OutputMode) format.Options,
) int {
	parsed, err := Parse(args)
	if err != nil {
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			_ = writeDiagnostic(stderr, "error: %s\n\n%s", usageErr.Message, UsageText())
			return ExitUsage
		}
		return writeFailure(stderr, err)
	}

	if outputOptionsForMode == nil {
		outputOptionsForMode = format.DefaultOptions
	}

	if parsed.Action == ActionHelp {
		helpOptions := outputOptionsForMode(format.OutputHuman)
		if _, err := io.WriteString(stdout, HelpTextWithOptions(helpOptions)); err != nil {
			_ = writeDiagnostic(stderr, "error: failed to write help: %v\n", err)
			return ExitFailure
		}
		return ExitSuccess
	}

	if service == nil {
		_ = writeDiagnostic(stderr, "error: GitHub service is not configured\n")
		return ExitFailure
	}

	outputOptions := outputOptionsForMode(parsed.Mode)
	outputOptions.Template = parsed.Template
	outputOptions.JQ = parsed.JQ
	if parsed.NoColor {
		outputOptions.Color = false
	}
	if parsed.Cache {
		service = githubapi.NewCacheService(service)
	}
	if parsed.OutputPath != "" {
		f, err := os.OpenFile(parsed.OutputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			_ = writeDiagnostic(stderr, "error: failed to open output file: %v\n", err)
			return ExitFailure
		}
		defer func() {
			_ = f.Close()
		}()
		stdout = f
	}
	switch parsed.Action {
	case ActionList:
		lists, err := service.ListStarLists(ctx, directListOptions(parsed))
		if err != nil {
			return writeRuntimeFailure(stderr, ActionList, "", err)
		}
		lists = filterStarLists(lists, parsed.Filters)
		sortStarLists(lists, parsed.SortKeys, parsed.SortTerms, parsed.SortDesc)
		if parsed.Limit > 0 && len(lists) > parsed.Limit {
			lists = lists[:parsed.Limit]
		}
		if err := format.WriteStarListsWithOptions(stdout, outputOptions, lists); err != nil {
			return writeFailure(stderr, fmt.Errorf("failed to write output: %w", err))
		}
	case ActionRepos:
		if parsed.Web {
			rl, err := resolveList(ctx, service, parsed.ListID)
			if err != nil {
				return writeRuntimeFailure(stderr, ActionRepos, parsed.ListID, err)
			}
			listURL := rl.URL
			if listURL == "" {
				listURL = rl.ID
			}
			if err := openBrowser(listURL); err != nil {
				return writeFailure(stderr, fmt.Errorf("failed to open browser: %w", err))
			}
			return ExitSuccess
		}
		repos, err := fetchReposForAction(ctx, service, parsed)
		if err != nil {
			return writeRuntimeFailure(stderr, ActionRepos, parsed.ListID, err)
		}
		repos = finalizeRepositories(repos, parsed)
		if err := format.WriteRepositoriesWithOptions(stdout, outputOptions, repos); err != nil {
			return writeFailure(stderr, fmt.Errorf("failed to write output: %w", err))
		}
	case ActionCreate:
		if parsed.DryRun {
			_, _ = fmt.Fprintf(stdout, "Would create Star List %q.\n", parsed.Name)
			return ExitSuccess
		}
		list, err := service.CreateStarList(ctx, githubapi.StarListInput{
			Name:        parsed.Name,
			Description: parsed.Description,
			Private:     parsed.Private,
		})
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.Name, err)
		}
		_, _ = fmt.Fprintf(stdout, "Created Star List %q (%s).\n", list.Name, list.ID)
	case ActionEdit:
		resolvedID, err := resolveListID(ctx, service, parsed.ListID)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.ListID, err)
		}
		if parsed.DryRun {
			_, _ = fmt.Fprintf(stdout, "Would update Star List %q.\n", parsed.ListID)
			return ExitSuccess
		}
		private := (*bool)(nil)
		if parsed.PrivateSet {
			private = &parsed.Private
		}
		list, err := service.UpdateStarList(ctx, githubapi.UpdateStarListInput{
			ID:          resolvedID,
			Name:        parsed.Name,
			Description: parsed.Description,
			Private:     private,
		})
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.ListID, err)
		}
		_, _ = fmt.Fprintf(stdout, "Updated Star List %q (%s).\n", list.Name, list.ID)
	case ActionDelete:
		if err := requireYes(parsed, "delete a Star List"); err != nil {
			return writeUsageFailure(stderr, err)
		}
		rl, err := resolveList(ctx, service, parsed.ListID)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.ListID, err)
		}
		if parsed.DryRun {
			_, _ = fmt.Fprintf(stdout, "Would delete Star List %q.\n", parsed.ListID)
			return ExitSuccess
		}
		if err := service.DeleteStarList(ctx, rl.ID); err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.ListID, err)
		}
		_, _ = fmt.Fprintf(stdout, "Deleted Star List %q.\n", parsed.ListID)
	case ActionAdd, ActionRemove, ActionMove:
		if parsed.Action != ActionAdd {
			if err := requireYes(parsed, string(parsed.Action)+" a repository"); err != nil {
				return writeUsageFailure(stderr, err)
			}
		}
		return runRepoListMutation(ctx, stdout, stderr, service, parsed)
	case ActionCopy, ActionMerge:
		if parsed.Action == ActionMerge || parsed.DeleteSource {
			if err := requireYes(parsed, string(parsed.Action)+" Star List contents"); err != nil {
				return writeUsageFailure(stderr, err)
			}
		}
		return runListCopy(ctx, stdout, stderr, service, parsed)
	case ActionUnstar:
		if err := requireYes(parsed, "unstar a repository"); err != nil {
			return writeUsageFailure(stderr, err)
		}
		repo, err := service.GetRepository(ctx, parsed.RepoName)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
		}
		if parsed.DryRun {
			_, _ = fmt.Fprintf(stdout, "Would unstar %s.\n", repo.NameWithOwner)
			return ExitSuccess
		}
		if err := service.RemoveStar(ctx, repo.ID); err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
		}
		_, _ = fmt.Fprintf(stdout, "Unstarred %s.\n", repo.NameWithOwner)
	default:
		panic(fmt.Sprintf("unhandled action %q - this is a bug in Parse", parsed.Action))
	}

	return ExitSuccess
}

func fetchReposForAction(
	ctx context.Context,
	service githubapi.Service,
	parsed Parsed,
) ([]githubapi.Repository, error) {
	switch {
	case parsed.Unlisted:
		lists, err := service.ListStarLists(ctx)
		if err != nil {
			return nil, err
		}
		index, err := loadMembershipIndex(ctx, service, lists)
		if err != nil {
			return nil, err
		}
		starred, err := service.ListStarredRepositories(ctx)
		if err != nil {
			return nil, err
		}
		unlisted := make([]githubapi.Repository, 0, len(starred))
		for _, r := range starred {
			if _, inList := index.byRepo[strings.ToLower(r.NameWithOwner)]; !inList {
				unlisted = append(unlisted, r)
			}
		}
		return unlisted, nil
	case parsed.All:
		return service.ListStarredRepositories(ctx, directListOptions(parsed))
	default:
		resolvedID, err := resolveListID(ctx, service, parsed.ListID)
		if err != nil {
			return nil, err
		}
		return service.ListRepositories(ctx, resolvedID, directListOptions(parsed))
	}
}

func finalizeRepositories(repos []githubapi.Repository, parsed Parsed) []githubapi.Repository {
	repos = filterRepositories(repos, parsed.Filters)
	repos = searchRepositories(repos, parsed.Search)
	sortRepositories(repos, parsed.SortKeys, parsed.SortTerms, parsed.SortDesc)
	if parsed.Limit > 0 && len(repos) > parsed.Limit {
		repos = repos[:parsed.Limit]
	}
	return repos
}

func runRepoListMutation(
	ctx context.Context,
	stdout, stderr io.Writer,
	service githubapi.Service,
	parsed Parsed,
) int {
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
	}
	var from, to githubapi.StarList
	if parsed.FromListID != "" {
		var ok bool
		from, ok = listByRaw(lists, parsed.FromListID)
		if !ok {
			return writeFailure(stderr, fmt.Errorf("Star List %q not found", parsed.FromListID))
		}
	}
	if parsed.ToListID != "" {
		var ok bool
		to, ok = listByRaw(lists, parsed.ToListID)
		if !ok {
			return writeFailure(stderr, fmt.Errorf("Star List %q not found", parsed.ToListID))
		}
	}
	if parsed.Action == ActionMove && from.ID == to.ID {
		return writeFailure(stderr, fmt.Errorf("--from and --to resolve to the same list"))
	}
	repoID, currentLists, err := service.GetRepositoryMemberships(ctx, parsed.RepoName)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
	}
	next := make(map[string]struct{}, len(currentLists)+1)
	for _, id := range currentLists {
		next[id] = struct{}{}
	}
	switch parsed.Action {
	case ActionAdd:
		next[to.ID] = struct{}{}
	case ActionRemove:
		delete(next, from.ID)
	case ActionMove:
		delete(next, from.ID)
		next[to.ID] = struct{}{}
	}
	listIDs := slices.Sorted(maps.Keys(next))
	if parsed.DryRun {
		_, _ = fmt.Fprintf(stdout, "Would %s %s.\n", parsed.Action, parsed.RepoName)
		return ExitSuccess
	}
	if err := service.UpdateRepositoryLists(ctx, repoID, listIDs); err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
	}
	_, _ = fmt.Fprintf(stdout, "%s %s.\n", pastTense(parsed.Action), parsed.RepoName)
	return ExitSuccess
}

func runListCopy(
	ctx context.Context,
	stdout, stderr io.Writer,
	service githubapi.Service,
	parsed Parsed,
) int {
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	fromList, fromFound := listByRaw(lists, parsed.FromListID)
	from := parsed.FromListID
	if fromFound {
		from = fromList.ID
	}
	toList, toFound := listByRaw(lists, parsed.ToListID)
	to := parsed.ToListID
	if toFound {
		to = toList.ID
	}
	if from == to {
		return writeFailure(stderr, fmt.Errorf("--from and --to resolve to the same list"))
	}
	index, err := loadMembershipIndex(ctx, service, lists)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	repos := index.reposByList[from]
	if !fromFound {
		repos, err = service.ListRepositories(ctx, from)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
		}
	}
	if parsed.DryRun {
		_, _ = fmt.Fprintf(stdout, "Would copy %d repositories from %q to %q.\n", len(repos), parsed.FromListID, parsed.ToListID)
		if parsed.DeleteSource || parsed.Action == ActionMerge {
			_, _ = fmt.Fprintf(stdout, "Would delete source Star List %q.\n", parsed.FromListID)
		}
		return ExitSuccess
	}
	var changed atomic.Int64
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(5)
	for _, repo := range repos {
		repo := repo
		group.Go(func() error {
			repoID, memberships, err := index.repositoryMemberships(groupCtx, service, repo.NameWithOwner)
			if err != nil {
				return fmt.Errorf("%s: %w", repo.NameWithOwner, err)
			}
			if _, ok := memberships[to]; ok {
				return nil
			}
			memberships[to] = struct{}{}
			if err := service.UpdateRepositoryLists(groupCtx, repoID, slices.Sorted(maps.Keys(memberships))); err != nil {
				return fmt.Errorf("%s: %w", repo.NameWithOwner, err)
			}
			changed.Add(1)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	if parsed.DeleteSource || parsed.Action == ActionMerge {
		if err := service.DeleteStarList(ctx, from); err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
		}
	}
	_, _ = fmt.Fprintf(stdout, "Copied %d repositories from %q to %q.\n", changed.Load(), parsed.FromListID, parsed.ToListID)
	return ExitSuccess
}

type repoMembership struct {
	repoID string
	lists  map[string]struct{}
}

type membershipIndex struct {
	byRepo      map[string]repoMembership
	reposByList map[string][]githubapi.Repository
}

func loadMembershipIndex(
	ctx context.Context,
	service githubapi.Service,
	lists []githubapi.StarList,
) (membershipIndex, error) {
	index := membershipIndex{
		byRepo:      make(map[string]repoMembership),
		reposByList: make(map[string][]githubapi.Repository),
	}
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(5)
	for _, list := range lists {
		list := list
		group.Go(func() error {
			repos, err := service.ListRepositories(groupCtx, list.ID)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			index.reposByList[list.ID] = repos
			for _, repo := range repos {
				key := strings.ToLower(repo.NameWithOwner)
				entry := index.byRepo[key]
				if entry.lists == nil {
					entry.lists = make(map[string]struct{})
				}
				entry.lists[list.ID] = struct{}{}
				if repo.ID != "" {
					entry.repoID = repo.ID
				}
				index.byRepo[key] = entry
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return membershipIndex{}, err
	}
	return index, nil
}

func (i membershipIndex) repositoryMemberships(
	ctx context.Context,
	service githubapi.Service,
	repoName string,
) (string, map[string]struct{}, error) {
	entry := i.byRepo[strings.ToLower(repoName)]
	memberships := maps.Clone(entry.lists)
	if memberships == nil {
		memberships = make(map[string]struct{})
	}
	repoID := entry.repoID
	if repoID == "" {
		repo, err := service.GetRepository(ctx, repoName)
		if err != nil {
			return "", nil, err
		}
		repoID = repo.ID
	}
	return repoID, memberships, nil
}

func listByRaw(lists []githubapi.StarList, raw string) (githubapi.StarList, bool) {
	for _, l := range lists {
		if strings.EqualFold(l.Name, raw) || l.ID == raw {
			return l, true
		}
	}
	return githubapi.StarList{}, false
}

func requireYes(parsed Parsed, action string) error {
	if parsed.Yes || parsed.DryRun {
		return nil
	}
	if canPrompt() {
		confirmed, err := confirmAction("Confirm " + action + "?")
		if err != nil {
			return err
		}
		if confirmed {
			return nil
		}
		return usage("%s was not confirmed", action)
	}
	return usage("%s requires --yes or --dry-run", action)
}

func writeUsageFailure(stderr io.Writer, err error) int {
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		_ = writeDiagnostic(stderr, "error: %s\n\n%s", usageErr.Message, UsageText())
		return ExitUsage
	}
	return writeFailure(stderr, err)
}

func pastTense(action Action) string {
	switch action {
	case ActionAdd:
		return "Added"
	case ActionRemove:
		return "Removed"
	case ActionMove:
		return "Moved"
	default:
		return "Updated"
	}
}

type resolvedList struct {
	ID  string
	URL string
}

func resolveList(ctx context.Context, service githubapi.Service, raw string) (resolvedList, error) {
	if raw == "" {
		return resolvedList{}, nil
	}
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return resolvedList{}, err
	}
	for _, l := range lists {
		if strings.EqualFold(l.Name, raw) || l.ID == raw {
			return resolvedList{ID: l.ID, URL: l.URL}, nil
		}
	}
	return resolvedList{ID: raw}, nil
}

func resolveListID(ctx context.Context, service githubapi.Service, raw string) (string, error) {
	r, err := resolveList(ctx, service, raw)
	if err != nil {
		return "", err
	}
	return r.ID, nil
}

func filterStarLists(lists []githubapi.StarList, filters []Filter) []githubapi.StarList {
	if len(filters) == 0 {
		return lists
	}
	out := make([]githubapi.StarList, 0, len(lists))
	for _, l := range lists {
		keep := true
		for _, f := range filters {
			if f.Key == FilterKeyName && !strings.Contains(strings.ToLower(l.Name), f.Value) {
				keep = false
			}
		}
		if keep {
			out = append(out, l)
		}
	}
	return out
}

func filterRepositories(repos []githubapi.Repository, filters []Filter) []githubapi.Repository {
	if len(filters) == 0 {
		return repos
	}
	out := make([]githubapi.Repository, 0, len(repos))
	for _, r := range repos {
		keep := true
		for _, f := range filters {
			switch f.Key {
			case FilterKeyName:
				if !strings.Contains(strings.ToLower(r.NameWithOwner), f.Value) {
					keep = false
				}
			case FilterKeyFork:
				wantFork := f.Value == "true"
				if r.IsFork != wantFork {
					keep = false
				}
			case FilterKeyLanguage:
				if strings.ToLower(r.Language) != f.Value {
					keep = false
				}
			case FilterKeyArchived:
				wantArchived := f.Value == "true"
				if r.IsArchived != wantArchived {
					keep = false
				}
			case FilterKeyLicense:
				if strings.ToLower(r.License) != f.Value {
					keep = false
				}
			case FilterKeyMinStars:
				minStars, _ := strconv.Atoi(f.Value)
				if r.StargazerCount < minStars {
					keep = false
				}
			case FilterKeyMaxStars:
				maxStars, _ := strconv.Atoi(f.Value)
				if r.StargazerCount > maxStars {
					keep = false
				}
			case FilterKeyTopic:
				if !hasTopic(r.Topics, f.Value) {
					keep = false
				}
			}
		}
		if keep {
			out = append(out, r)
		}
	}
	return out
}

func hasTopic(topics []string, want string) bool {
	for _, topic := range topics {
		if strings.ToLower(topic) == want {
			return true
		}
	}
	return false
}

func sortStarLists(lists []githubapi.StarList, sortKeys []string, sortTerms []SortTerm, desc bool) {
	if len(sortKeys) == 0 {
		return
	}

	sort.Slice(lists, func(i, j int) bool {
		cmp, termDesc := compareStarLists(lists[i], lists[j], sortKeys, sortTerms, desc)
		if termDesc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareStarLists(
	left, right githubapi.StarList,
	sortKeys []string,
	sortTerms []SortTerm,
	globalDesc bool,
) (int, bool) {
	for idx, key := range sortKeys {
		termDesc := globalDesc
		if len(sortTerms) > idx {
			termDesc = sortTerms[idx].Desc
		}
		var cmp int
		switch key {
		case SortKeyAdded:
			if left.LastAddedAt != right.LastAddedAt {
				cmp = strings.Compare(left.LastAddedAt, right.LastAddedAt)
			}
		case SortKeyName:
			leftName := strings.ToLower(left.Name)
			rightName := strings.ToLower(right.Name)
			if leftName != rightName {
				cmp = strings.Compare(leftName, rightName)
			}
		case SortKeyRepoCount:
			if left.RepoCount != right.RepoCount {
				cmp = left.RepoCount - right.RepoCount
			}
		}
		if cmp != 0 {
			return cmp, termDesc
		}
	}
	if left.Name != right.Name {
		return strings.Compare(left.Name, right.Name), false
	}
	return strings.Compare(left.ID, right.ID), false
}

func sortRepositories(repos []githubapi.Repository, sortKeys []string, sortTerms []SortTerm, desc bool) {
	if len(sortKeys) == 0 {
		return
	}

	sort.Slice(repos, func(i, j int) bool {
		cmp, termDesc := compareRepositories(repos[i], repos[j], sortKeys, sortTerms, desc)
		if termDesc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareRepositories(
	left, right githubapi.Repository,
	sortKeys []string,
	sortTerms []SortTerm,
	globalDesc bool,
) (int, bool) {
	for idx, key := range sortKeys {
		termDesc := globalDesc
		if len(sortTerms) > idx {
			termDesc = sortTerms[idx].Desc
		}
		var cmp int
		switch key {
		case SortKeyName:
			cmp = strings.Compare(
				strings.ToLower(left.NameWithOwner),
				strings.ToLower(right.NameWithOwner),
			)
		case SortKeyStars:
			if left.StargazerCount != right.StargazerCount {
				cmp = left.StargazerCount - right.StargazerCount
			}
		case SortKeyPushed:
			if left.PushedAt != right.PushedAt {
				cmp = strings.Compare(left.PushedAt, right.PushedAt)
			}
		case SortKeyLanguage:
			cmp = strings.Compare(strings.ToLower(left.Language), strings.ToLower(right.Language))
		case SortKeyStarred:
			if left.StarredAt != right.StarredAt {
				cmp = strings.Compare(left.StarredAt, right.StarredAt)
			}
		}
		if cmp != 0 {
			return cmp, termDesc
		}
	}
	if left.NameWithOwner != right.NameWithOwner {
		return strings.Compare(left.NameWithOwner, right.NameWithOwner), false
	}
	return strings.Compare(left.URL, right.URL), false
}

func directListOptions(parsed Parsed) githubapi.ListOptions {
	if parsed.Limit == 0 || len(parsed.Filters) > 0 || parsed.Search != "" || len(parsed.SortKeys) > 0 {
		return githubapi.ListOptions{}
	}
	return githubapi.ListOptions{Limit: parsed.Limit}
}

func writeFailure(stderr io.Writer, err error) int {
	_ = writeDiagnostic(stderr, "error: %v\n", err)
	return ExitFailure
}

func writeRuntimeFailure(stderr io.Writer, action Action, listID string, err error) int {
	_ = writeDiagnostic(stderr, "error: %s: %v\n", commandContext(action, listID), err)
	if errors.Is(err, githubapi.ErrInaccessibleList) {
		_ = writeDiagnostic(
			stderr,
			"The Star List ID may be deleted, private, inaccessible to this account, or from another GitHub account. Re-run `gh star-lists` with the intended account.\n",
		)
	} else if looksLikeAuthError(err) {
		_ = writeDiagnostic(
			stderr,
			"Run `gh auth status` to check GitHub CLI authentication, then `gh auth login` if needed.\n",
		)
	}
	return ExitFailure
}

func commandContext(action Action, listID string) string {
	switch action {
	case ActionRepos:
		return fmt.Sprintf("failed to list repositories for Star List %q", listID)
	default:
		return "failed to list Star Lists"
	}
}

func looksLikeAuthError(err error) bool {
	message := strings.ToLower(err.Error())
	authMarkers := []string{
		"authentication",
		"bad credentials",
		"gh auth",
		"oauth",
		"token",
		"unauthorized",
		"401",
	}
	for _, marker := range authMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func writeDiagnostic(stderr io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(stderr, format, args...)
	return err
}
