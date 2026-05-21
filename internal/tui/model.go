package tui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
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
	lists       []githubapi.StarList
	focusedList *githubapi.StarList

	repoCache       map[repoCacheKey]*repoCacheEntry
	generation      uint64
	listsLoading    bool
	mutationPending bool

	// render-width cache (invalidated by cachedRepoSig sentinel)
	cachedStarWidth int
	cachedRepoSig   string

	// preview pane scroll offset (lines scrolled down)
	previewOffset int

	preloadQueue    []string // list IDs waiting to be loaded, in sorted order
	preloadInFlight int      // number of loads currently in flight (cap: 3)

	listCursor int
	listOffset int
	repoCursor int
	repoOffset int

	sortLists sortListsKey
	sortRepos sortReposKey

	err error

	showHelp bool
	width    int
	height   int

	modal        *modal
	statusMsg    string
	statusExpiry time.Time
	showPreview  bool

	searchActive   bool
	searchQuery    string
	displayedLists []githubapi.StarList
	displayedRepos []githubapi.Repository

	selected map[string]struct{} // NameWithOwner of checked repos

	// Double-click tracking for list-pane rows.
	lastClickPane  int
	lastClickIndex int
	lastClickTime  time.Time

	spinner spinner.Model
}

func newModel(ctx context.Context, svc githubapi.Service, opts Options) model {
	return model{
		svc:             svc,
		openBrowser:     opts.OpenBrowser,
		noColor:         opts.NoColor,
		mouse:           opts.Mouse,
		ctx:             ctx,
		listsLoading:    true,
		repoCache:       make(map[repoCacheKey]*repoCacheEntry),
		spinner:         spinner.New(spinner.WithSpinner(spinner.Line)),
		preloadQueue:    nil,
		preloadInFlight: 0,
	}
}

// schedulePreload starts up to (3 - m.preloadInFlight) repo loads from m.preloadQueue.

func (m model) Init() tea.Cmd {
	return tea.Batch(loadListsCmd(m.ctx, m.svc), m.spinner.Tick)
}
