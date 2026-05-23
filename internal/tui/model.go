package tui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type pane int

const (
	paneList pane = iota
	paneRepo
)

type model struct {
	svc         githubapi.Service
	openBrowser func(string) error
	noColor     bool
	mouse       bool
	ctx         context.Context

	active      pane
	lists       []domain.StarList
	focusedList *domain.StarList

	preloader *preloader

	listsLoading    bool
	mutationPending bool

	listCursor int
	listOffset int
	repoCursor int
	repoOffset int

	sortLists sortListsKey
	sortRepos sortReposKey

	err error

	width  int
	height int

	modal        *modal
	statusMsg    string
	statusExpiry time.Time

	listSearchActive bool
	listSearchQuery  string
	repoSearchActive bool
	repoSearchQuery  string
	displayedLists   []domain.StarList
	displayedRepos   []domain.Repository

	selected map[string]struct{} // NameWithOwner of checked repos

	spinner spinner.Model
}

func newModel(ctx context.Context, svc githubapi.Service, opts Options) model {
	return model{
		svc:          svc,
		openBrowser:  opts.OpenBrowser,
		noColor:      opts.NoColor,
		mouse:        opts.Mouse,
		ctx:          ctx,
		listsLoading: true,
		preloader:    newPreloader(),
		spinner:      spinner.New(spinner.WithSpinner(spinner.Line)),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(loadListsCmd(m.ctx, m.svc), m.spinner.Tick)
}
