package tui

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"github.com/maoyeedy/gh-star-lists/internal/search"
)

type pane int

const (
	paneList pane = iota
	paneRepo
)

type sortListsKey int

const (
	sortListsGitHub sortListsKey = iota
	sortListsName
	sortListsRepos
	sortListsAdded
)

type sortReposKey int

const (
	sortReposGitHub sortReposKey = iota
	sortReposName
	sortReposStars
	sortReposPushed
	sortReposLanguage
	sortReposStarredAt
)

type repoCacheKey struct {
	listID     string
	withTopics bool
}

type repoCacheState int

const (
	repoCacheIdle repoCacheState = iota
	repoCacheLoading
	repoCacheLoaded
	repoCacheError
)

type repoCacheEntry struct {
	state repoCacheState
	repos []githubapi.Repository
	err   error
	gen   uint64
}

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
	cachedLangWidth int
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
// Returns a tea.Cmd (may be nil or a tea.Batch).
func (m *model) schedulePreload() tea.Cmd {
	const maxInFlight = 3
	var cmds []tea.Cmd
	for m.preloadInFlight < maxInFlight && len(m.preloadQueue) > 0 {
		listID := m.preloadQueue[0]
		m.preloadQueue = m.preloadQueue[1:]
		key := repoCacheKey{listID, false}
		if e := m.repoCache[key]; e != nil && e.state != repoCacheIdle {
			continue // already loading or loaded: skip
		}
		m.repoCache[key] = &repoCacheEntry{state: repoCacheLoading, gen: m.generation}
		m.preloadInFlight++
		capturedID := listID // capture for closure
		cmds = append(cmds, loadReposCmd(m.ctx, m.svc, capturedID, false, m.generation))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// focusList sets the list cursor to idx, resolves focusedList, and updates the
// repo pane immediately from the cache. Returns a cmd if a load must be started.
func (m *model) focusList(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.displayedLists) {
		return nil
	}
	m.previewOffset = 0
	m.listCursor = idx
	list := m.displayedLists[idx]
	// Find the pointer in m.lists to keep focusedList pointing at the canonical slice.
	for i := range m.lists {
		if m.lists[i].ID == list.ID {
			m.focusedList = &m.lists[i]
			break
		}
	}
	key := repoCacheKey{list.ID, false}
	e := m.repoCache[key]
	switch {
	case e != nil && e.state == repoCacheLoaded:
		// Populate display slice immediately from cache.
		sorted := make([]githubapi.Repository, len(e.repos))
		copy(sorted, e.repos)
		sortRepos(sorted, m.sortRepos)
		m.displayedRepos = sorted
		if m.searchActive && m.searchQuery != "" {
			*m = m.rebuildDisplayed()
		}
		m.repoCursor = 0
		m.repoOffset = 0
		return nil
	case e != nil && e.state == repoCacheLoading:
		m.displayedRepos = nil
		return nil
	case e != nil && e.state == repoCacheError:
		m.displayedRepos = nil
		return nil
	default: // repoCacheIdle or absent: promote and start
		// Remove from queue if present, prepend.
		newQueue := make([]string, 0, len(m.preloadQueue)+1)
		newQueue = append(newQueue, list.ID)
		for _, id := range m.preloadQueue {
			if id != list.ID {
				newQueue = append(newQueue, id)
			}
		}
		m.preloadQueue = newQueue
		m.displayedRepos = nil
		return m.schedulePreload()
	}
}

func (m *model) setRepoCursor(idx int) {
	if m.repoCursor != idx {
		m.previewOffset = 0
	}
	m.repoCursor = idx
}

// currentRepos returns the repos for the focused list (using current withTopics state).
// Returns nil when loading, idle, or no list is focused.
func (m model) currentRepos() []githubapi.Repository {
	if m.focusedList == nil {
		return nil
	}
	e := m.repoCache[repoCacheKey{m.focusedList.ID, m.showPreview}]
	if e == nil || e.state != repoCacheLoaded {
		return nil
	}
	return e.repos
}

// anyPending reports whether any async work is in progress.
func (m model) anyPending() bool {
	if m.listsLoading || m.mutationPending {
		return true
	}
	for _, e := range m.repoCache {
		if e.state == repoCacheLoading {
			return true
		}
	}
	return false
}

// invalidatable is satisfied by cacheService.
type invalidatable interface {
	Invalidate()
}

func (m model) Init() tea.Cmd {
	return tea.Batch(loadListsCmd(m.ctx, m.svc), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case listsLoadedMsg:
		m.lists = msg.lists
		m.listsLoading = false
		sortStarLists(m.lists, m.sortLists)
		m = m.rebuildDisplayed()
		if m.listCursor >= len(m.displayedLists) && len(m.displayedLists) > 0 {
			m.listCursor = len(m.displayedLists) - 1
		}
		// Eager initial load: auto-focus first list and build preload queue.
		if m.focusedList == nil && len(m.lists) > 0 {
			m.focusedList = &m.lists[0]
			m.repoCursor = 0
			m.previewOffset = 0
			m.repoOffset = 0
			m.selected = nil
			// Build the preload queue from the sorted displayed list IDs.
			// Put the focused list first (it gets the first slot).
			m.preloadQueue = make([]string, 0, len(m.displayedLists))
			m.preloadQueue = append(m.preloadQueue, m.focusedList.ID)
			for _, l := range m.displayedLists {
				if l.ID != m.focusedList.ID {
					m.preloadQueue = append(m.preloadQueue, l.ID)
				}
			}
			m.preloadInFlight = 0
			preloadCmd := (&m).schedulePreload()
			return m, preloadCmd
		}
		return m, nil

	case reposLoadedMsg:
		if msg.gen != m.generation {
			return m, nil // stale: drop
		}
		key := repoCacheKey{msg.listID, msg.withTopics}
		if msg.err != nil {
			m.repoCache[key] = &repoCacheEntry{state: repoCacheError, err: msg.err, gen: msg.gen}
		} else {
			entry := &repoCacheEntry{state: repoCacheLoaded, repos: msg.repos, gen: msg.gen}
			m.repoCache[key] = entry
			// Update displayed slice only if this is the focused list and matches current withTopics.
			if m.focusedList != nil && msg.listID == m.focusedList.ID &&
				msg.withTopics == m.showPreview {
				sorted := make([]githubapi.Repository, len(entry.repos))
				copy(sorted, entry.repos)
				sortRepos(sorted, m.sortRepos)
				m.displayedRepos = sorted
				if m.searchActive && m.searchQuery != "" {
					m = m.rebuildDisplayed()
				}
				if m.repoCursor >= len(m.displayedRepos) && len(m.displayedRepos) > 0 {
					(&m).setRepoCursor(len(m.displayedRepos) - 1)
				}
			}
			// Refresh focusedList pointer only if this load is for the focused list.
			if m.focusedList != nil && m.focusedList.ID == msg.listID {
				for i := range m.lists {
					if m.lists[i].ID == msg.listID {
						m.focusedList = &m.lists[i]
						break
					}
				}
			}
			// Drop selected keys that no longer exist in the refreshed repo list.
			if len(m.selected) > 0 {
				repos := m.currentRepos()
				existing := make(map[string]struct{}, len(repos))
				for _, r := range repos {
					existing[r.NameWithOwner] = struct{}{}
				}
				for nwo := range m.selected {
					if _, ok := existing[nwo]; !ok {
						delete(m.selected, nwo)
					}
				}
			}
		}
		// Only count non-topics loads toward the preloader inflight cap.
		if !msg.withTopics {
			if m.preloadInFlight > 0 {
				m.preloadInFlight--
			}
		}
		return m, (&m).schedulePreload()

	case errMsg:
		m.err = msg.err
		m.listsLoading = false
		for k, e := range m.repoCache {
			if e.state == repoCacheLoading {
				m.repoCache[k] = &repoCacheEntry{state: repoCacheError, err: msg.err, gen: e.gen}
			}
		}
		return m, nil

	case mutationDoneMsg:
		m.mutationPending = false
		if msg.err != nil {
			// Keep modal open, show error inline.
			if m.modal != nil {
				m.modal.submitting = false
				m.modal.submitErr = msg.err.Error()
			}
			return m, nil
		}
		m.modal = nil
		m.statusMsg = "Done."
		m.statusExpiry = time.Now().Add(2 * time.Second)
		m.listsLoading = true
		cmds := []tea.Cmd{loadListsCmd(m.ctx, m.svc), statusClearCmd(m.statusExpiry)}
		// Invalidate repo cache for focused list.
		if m.focusedList != nil {
			delete(m.repoCache, repoCacheKey{m.focusedList.ID, false})
			delete(m.repoCache, repoCacheKey{m.focusedList.ID, true})
		}
		// For repo-pane mutations, trigger re-fetch of the focused list.
		if m.active == paneRepo && m.focusedList != nil {
			key := repoCacheKey{m.focusedList.ID, m.showPreview}
			m.repoCache[key] = &repoCacheEntry{state: repoCacheLoading, gen: m.generation}
			cmds = append(
				cmds,
				loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.showPreview, m.generation),
			)
		}
		return m, tea.Batch(cmds...)

	case bulkDoneMsg:
		m.mutationPending = false
		m.selected = nil
		switch {
		case msg.failed > 0 && m.modal != nil:
			m.statusMsg = ""
			m.modal.submitting = false
			m.modal.submitErr = ""
			m.modal.bulkFailure = &bulkFailureState{
				verb:       msg.verb,
				succeeded:  msg.succeeded,
				failedNWOs: append([]string(nil), msg.failedNWOs...),
			}
			if msg.succeeded == 0 {
				return m, nil
			}
		case msg.failed > 0:
			m.modal = nil
			names := msg.failedNWOs
			var failDetail string
			switch {
			case len(names) == 0:
				failDetail = ""
			case len(names) <= 3:
				failDetail = " (" + strings.Join(names, ", ") + ")"
			default:
				failDetail = " (" + strings.Join(
					names[:3],
					", ",
				) + fmt.Sprintf(
					", +%d more)",
					len(names)-3,
				)
			}
			m.statusMsg = fmt.Sprintf(
				"%d %s, %d failed%s",
				msg.succeeded,
				msg.verb,
				msg.failed,
				failDetail,
			)
		default:
			m.modal = nil
			m.statusMsg = fmt.Sprintf("%d repos %s.", msg.succeeded, msg.verb)
		}
		var cmds []tea.Cmd
		if m.statusMsg != "" {
			m.statusExpiry = time.Now().Add(2 * time.Second)
			cmds = append(cmds, statusClearCmd(m.statusExpiry))
		}
		m.listsLoading = true
		cmds = append(cmds, loadListsCmd(m.ctx, m.svc))
		// Invalidate repo cache for focused list.
		if m.focusedList != nil {
			delete(m.repoCache, repoCacheKey{m.focusedList.ID, false})
			delete(m.repoCache, repoCacheKey{m.focusedList.ID, true})
		}
		if m.active == paneRepo && m.focusedList != nil {
			key := repoCacheKey{m.focusedList.ID, m.showPreview}
			m.repoCache[key] = &repoCacheEntry{state: repoCacheLoading, gen: m.generation}
			cmds = append(
				cmds,
				loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.showPreview, m.generation),
			)
		}
		return m, tea.Batch(cmds...)

	case statusExpiredMsg:
		m.statusMsg = ""
		return m, nil

	case tea.MouseWheelMsg:
		if m.modal != nil || m.searchActive {
			return m, nil
		}
		var delta int
		switch msg.Button {
		case tea.MouseWheelUp:
			delta = -1
		case tea.MouseWheelDown:
			delta = 1
		default:
			return m, nil
		}
		// Scroll the pane under the pointer, regardless of which pane is active.
		g := calcPaneGeometry(m.width, m.showPreview)
		switch {
		case msg.X < g.sep1Col:
			// List pane.
			m.listCursor = clampInt(m.listCursor+delta, 0, len(m.displayedLists)-1)
			m = m.slideListOffset()
		case m.showPreview && g.sep2Col >= 0 && msg.X > g.sep2Col:
			// Preview pane wheel scroll.
			contentH := m.height - 2
			viewH := contentH
			content := countPreviewLines(m, g.previewWidth, contentH)
			previewDelta := 3
			if delta < 0 {
				previewDelta = -3
			}
			m.previewOffset = slidePreviewOffset(m.previewOffset, previewDelta, content, viewH)
		default:
			// Repo pane (between sep1Col and sep2Col, or sep2Col < 0).
			(&m).setRepoCursor(clampInt(m.repoCursor+delta, 0, len(m.displayedRepos)-1))
			m = m.slideRepoOffset()
		}
		return m, nil

	case tea.MouseClickMsg:
		if m.modal != nil || m.searchActive {
			return m, nil
		}
		var cmd tea.Cmd
		m, cmd = m.handleMouseClick(msg)
		return m, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.anyPending() {
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.modal != nil {
			updated, cmd := m.modal.update(msg)
			if updated == nil && cmd != nil {
				// Modal signalled submit: keep it open in submitting state.
				m.modal.submitting = true
				m.modal.submitErr = ""
				m.modal.bulkFailure = nil
				m.mutationPending = true
				return m, tea.Batch(cmd, m.spinner.Tick)
			}
			m.modal = updated
			return m, cmd
		}
		if m.searchActive {
			return m.handleSearchKey(msg)
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Back):
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		// Clear selection first if any; second Esc then navigates back / quits.
		if len(m.selected) > 0 {
			m.selected = nil
			return m, nil
		}
		if m.active == paneRepo {
			m.active = paneList
			return m, nil
		}
		return m, tea.Quit

	case key.Matches(msg, keys.Left):
		if m.active == paneRepo {
			m.active = paneList
		}
		return m, nil

	case key.Matches(msg, keys.Right):
		if m.active == paneList && m.focusedList != nil {
			m.active = paneRepo
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		if m.active == paneList {
			newIdx := clampInt(m.listCursor-1, 0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(clampInt(m.repoCursor-1, 0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.active == paneList {
			newIdx := clampInt(m.listCursor+1, 0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(clampInt(m.repoCursor+1, 0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.PgUp):
		paneH := max(1, m.height-2)
		if m.active == paneList {
			newIdx := clampInt(m.listCursor-(paneH-1), 0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(clampInt(m.repoCursor-(paneH-1), 0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.PgDn):
		paneH := max(1, m.height-2)
		if m.active == paneList {
			newIdx := clampInt(m.listCursor+(paneH-1), 0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(clampInt(m.repoCursor+(paneH-1), 0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.Home):
		if m.active == paneList {
			cmd := (&m).focusList(0)
			m.listOffset = 0
			return m, cmd
		}
		(&m).setRepoCursor(0)
		m.repoOffset = 0
		return m, nil

	case key.Matches(msg, keys.End):
		if m.active == paneList {
			newIdx := max(0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(max(0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.Enter):
		return m.handleEnter()

	case key.Matches(msg, keys.Open):
		return m.handleOpen()

	case key.Matches(msg, keys.Sort):
		m = m.cycleSort()
		return m, nil

	case key.Matches(msg, keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, keys.Help):
		m.showHelp = !m.showHelp
		return m, nil

	case key.Matches(msg, keys.CreateList):
		mo, focusCmd := newCreateListModal(m.ctx, m.svc)
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.EditList):
		if m.active != paneList || len(m.displayedLists) == 0 {
			return m, nil
		}
		mo, focusCmd := newEditListModal(m.ctx, m.svc, m.displayedLists[m.listCursor])
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.DeleteList):
		if m.active != paneList || len(m.displayedLists) == 0 {
			return m, nil
		}
		mo, focusCmd := newDeleteListModal(m.ctx, m.svc, m.displayedLists[m.listCursor])
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.AddRepo):
		if m.active != paneRepo || len(m.displayedRepos) == 0 || len(m.lists) == 0 {
			return m, nil
		}
		if len(m.selected) > 0 {
			m.modal = newBulkAddModal(m.ctx, m.svc, m.selectedNWOs(), m.lists)
		} else {
			m.modal = newAddRepoModal(m.ctx, m.svc, m.displayedRepos[m.repoCursor], m.lists)
		}
		return m, nil

	case key.Matches(msg, keys.RemoveRepo):
		if m.active != paneRepo || len(m.displayedRepos) == 0 || m.focusedList == nil {
			return m, nil
		}
		if len(m.selected) > 0 {
			m.modal = newBulkRemoveModal(m.ctx, m.svc, m.selectedNWOs(), m.focusedList.ID)
		} else {
			repo := m.displayedRepos[m.repoCursor]
			m.modal = newRemoveRepoModal(m.ctx, m.svc, repo, m.focusedList.ID)
		}
		return m, nil

	case key.Matches(msg, keys.MoveRepo):
		if m.active != paneRepo || len(m.displayedRepos) == 0 ||
			m.focusedList == nil || len(m.lists) < 2 {
			return m, nil
		}
		if len(m.selected) > 0 {
			m.modal = newBulkMoveModal(m.ctx, m.svc, m.selectedNWOs(), m.lists, m.focusedList.ID)
		} else {
			repo := m.displayedRepos[m.repoCursor]
			m.modal = newMoveRepoModal(m.ctx, m.svc, repo, m.lists, m.focusedList.ID)
		}
		return m, nil

	case key.Matches(msg, keys.UnstarRepo):
		if m.active != paneRepo || len(m.displayedRepos) == 0 {
			return m, nil
		}
		repo := m.displayedRepos[m.repoCursor]
		mo, focusCmd := newUnstarRepoModal(m.ctx, m.svc, repo)
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.CopyList):
		if m.active != paneList || len(m.displayedLists) == 0 || len(m.lists) < 2 {
			return m, nil
		}
		m.modal = newCopyListModal(m.ctx, m.svc, m.displayedLists[m.listCursor], m.lists)
		return m, nil

	case key.Matches(msg, keys.MergeList):
		if m.active != paneList || len(m.displayedLists) == 0 || len(m.lists) < 2 {
			return m, nil
		}
		m.modal = newMergeListModal(m.ctx, m.svc, m.displayedLists[m.listCursor], m.lists)
		return m, nil

	case key.Matches(msg, keys.Preview):
		m.showPreview = !m.showPreview
		m.previewOffset = 0
		if m.showPreview && m.focusedList != nil {
			// Only schedule a withTopics=true load for the focused list if not already loaded/loading.
			topicsKey := repoCacheKey{m.focusedList.ID, true}
			e := m.repoCache[topicsKey]
			if e == nil || e.state == repoCacheIdle {
				m.repoCache[topicsKey] = &repoCacheEntry{state: repoCacheLoading, gen: m.generation}
				return m, loadReposCmd(m.ctx, m.svc, m.focusedList.ID, true, m.generation)
			}
		}
		return m, nil

	case key.Matches(msg, keys.Search):
		m.searchActive = true
		m.searchQuery = ""
		m.listCursor = 0
		m.repoCursor = 0
		m.previewOffset = 0
		m.listOffset = 0
		m.repoOffset = 0
		m = m.rebuildDisplayed()
		return m, nil

	case key.Matches(msg, keys.Select):
		if m.active != paneRepo || len(m.displayedRepos) == 0 {
			return m, nil
		}
		nwo := m.displayedRepos[m.repoCursor].NameWithOwner
		if m.selected == nil {
			m.selected = make(map[string]struct{})
		}
		if _, ok := m.selected[nwo]; ok {
			delete(m.selected, nwo)
		} else {
			m.selected[nwo] = struct{}{}
		}
		return m, nil
	}

	return m, nil
}

// selectedNWOs returns sorted NameWithOwner strings from the selection set.
func (m model) selectedNWOs() []string {
	return slices.Sorted(maps.Keys(m.selected))
}

func (m model) handleMouseClick(msg tea.MouseClickMsg) (model, tea.Cmd) {
	contentRow := msg.Y - 1 // row 0 is the header
	if contentRow < 0 {
		return m, nil
	}
	g := calcPaneGeometry(m.width, m.showPreview)
	if msg.X < g.sep1Col {
		// List pane click.
		m.active = paneList
		idx := contentRow + m.listOffset
		if idx < 0 || idx >= len(m.displayedLists) {
			return m, nil
		}

		// Double-click detection: two clicks on same pane+row within 300ms drills to repo pane.
		now := time.Now()
		if m.lastClickPane == int(paneList) && m.lastClickIndex == idx &&
			!m.lastClickTime.IsZero() && now.Sub(m.lastClickTime) < 300*time.Millisecond {
			// Double-click: drill into the list, ensure load starts if idle.
			_ = (&m).focusList(idx)
			m.active = paneRepo
			m.repoCursor = 0
			m.previewOffset = 0
			m.repoOffset = 0
			m.selected = nil
			// Reset tracker.
			m.lastClickTime = time.Time{}
			return m, nil
		}
		// Single click.
		if idx != m.listCursor {
			cmd := (&m).focusList(idx)
			m.lastClickPane = int(paneList)
			m.lastClickIndex = idx
			m.lastClickTime = now
			return m, cmd
		}
		// Already focused: if idle, trigger load without switching pane.
		m.lastClickPane = int(paneList)
		m.lastClickIndex = idx
		m.lastClickTime = now
		if m.focusedList != nil {
			cacheKey := repoCacheKey{m.focusedList.ID, false}
			e := m.repoCache[cacheKey]
			if e == nil || e.state == repoCacheIdle {
				cmd := (&m).focusList(idx)
				return m, cmd
			}
		}
		return m, nil
	} else if msg.X > g.sep1Col && (g.sep2Col < 0 || msg.X < g.sep2Col) {
		// Repo pane click.
		if m.focusedList != nil && len(m.displayedRepos) > 0 {
			m.active = paneRepo
			idx := contentRow + m.repoOffset
			if idx >= 0 && idx < len(m.displayedRepos) {
				(&m).setRepoCursor(idx)
			}
		}
	}
	// Clicks in the preview pane (msg.X > g.sep2Col when showPreview) are no-ops for now.
	return m, nil
}

func (m model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.searchActive = false
		m.searchQuery = ""
		m = m.rebuildDisplayed()
		m.listCursor = 0
		m.repoCursor = 0
		m.previewOffset = 0
		m.listOffset = 0
		m.repoOffset = 0
		return m, nil
	case key.Matches(msg, keys.Enter):
		m.searchActive = false
		return m, nil
	case msg.Code == tea.KeyBackspace || msg.Code == tea.KeyDelete:
		m.searchQuery = dropLastRune(m.searchQuery)
		m = m.rebuildDisplayed()
		m.listCursor = 0
		m.repoCursor = 0
		m.previewOffset = 0
		m.listOffset = 0
		m.repoOffset = 0
		return m, nil
	}
	// Pass navigation keys to handleKey so arrows/PgDn still work.
	if key.Matches(msg, keys.Up) || key.Matches(msg, keys.Down) ||
		key.Matches(msg, keys.PgUp) || key.Matches(msg, keys.PgDn) ||
		key.Matches(msg, keys.Home) || key.Matches(msg, keys.End) {
		return m.handleKey(msg)
	}
	if msg.Text != "" {
		m.searchQuery += msg.Text
		m = m.rebuildDisplayed()
		m.listCursor = 0
		m.repoCursor = 0
		m.previewOffset = 0
		m.listOffset = 0
		m.repoOffset = 0
	}
	return m, nil
}

func (m model) slideListOffset() model {
	paneH := max(1, m.height-2)
	if m.listCursor < m.listOffset {
		m.listOffset = m.listCursor
	} else if m.listCursor >= m.listOffset+paneH {
		m.listOffset = m.listCursor - paneH + 1
	}
	m.listOffset = clampInt(m.listOffset, 0, max(0, len(m.displayedLists)-paneH))
	return m
}

// repoPaneH returns the effective number of scrollable repo rows in the repo
// pane (full pane content height; no heading overhead).
func (m model) repoPaneH() int {
	return max(1, m.height-2)
}

func (m model) slideRepoOffset() model {
	paneH := m.repoPaneH()
	if m.repoCursor < m.repoOffset {
		m.repoOffset = m.repoCursor
	} else if m.repoCursor >= m.repoOffset+paneH {
		m.repoOffset = m.repoCursor - paneH + 1
	}
	m.repoOffset = clampInt(m.repoOffset, 0, max(0, len(m.displayedRepos)-paneH))
	return m
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m model) rebuildDisplayed() model {
	repos := m.currentRepos()
	if m.searchQuery == "" {
		m.displayedLists = m.lists
		m.displayedRepos = repos
	} else {
		m.displayedLists = search.FilterStarLists(m.lists, m.searchQuery)
		m.displayedRepos = search.FilterRepositories(repos, m.searchQuery)
	}
	return m
}

func dropLastRune(s string) string {
	_, size := utf8.DecodeLastRuneInString(s)
	if size == 0 {
		return s
	}
	return s[:len(s)-size]
}

// slidePreviewOffset clamps next = current+delta within [0, max(0, contentH-viewH)].
func slidePreviewOffset(current, delta, contentH, viewH int) int {
	next := current + delta
	maxOffset := contentH - viewH
	if maxOffset < 0 {
		maxOffset = 0
	}
	if next < 0 {
		next = 0
	}
	if next > maxOffset {
		next = maxOffset
	}
	return next
}

// countPreviewLines returns the number of content lines the preview pane would produce
// for the given repo (before scroll offset is applied).
func countPreviewLines(m model, w, _ int) int {
	if m.active != paneRepo || len(m.displayedRepos) == 0 {
		return 1
	}
	return len(m.previewContentLines(w))
}

// ensureRepoWidths computes and caches the star and language column widths for the
// current displayedRepos slice. The sentinel incorporates the focused list ID, sort
// key, search query, and count, so it auto-invalidates when any of those change.
func (m *model) ensureRepoWidths() {
	sig := ""
	if m.focusedList != nil {
		sig = m.focusedList.ID
	}
	sig += fmt.Sprintf("|%d|%s|%d", m.sortRepos, m.searchQuery, len(m.displayedRepos))
	if sig == m.cachedRepoSig {
		return // cache hit
	}
	m.cachedRepoSig = sig
	maxStars, maxLang := 0, 0
	for _, r := range m.displayedRepos {
		if n := len(fmt.Sprintf("%d", r.StargazerCount)); n > maxStars {
			maxStars = n
		}
		if n := len(r.Language); n > maxLang {
			maxLang = n
		}
	}
	if maxLang > 12 {
		maxLang = 12
	}
	m.cachedStarWidth = maxStars
	m.cachedLangWidth = maxLang
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		if len(m.displayedLists) == 0 {
			return m, nil
		}
		var cmd tea.Cmd
		key := repoCacheKey{m.displayedLists[m.listCursor].ID, m.showPreview}
		e := m.repoCache[key]
		if e == nil || e.state == repoCacheIdle {
			// Idle: start the load via focusList.
			cmd = (&m).focusList(m.listCursor)
		}
		m.active = paneRepo
		m.repoCursor = 0
		m.previewOffset = 0
		m.repoOffset = 0
		m.selected = nil
		return m, cmd
	}
	// paneRepo: open in browser
	return m.openFocusedRepoURL()
}

func (m model) handleOpen() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		if len(m.displayedLists) == 0 {
			return m, nil
		}
		url := m.displayedLists[m.listCursor].URL
		if url != "" && m.openBrowser != nil {
			_ = m.openBrowser(url)
		}
		return m, nil
	}
	return m.openFocusedRepoURL()
}

func (m model) openFocusedRepoURL() (tea.Model, tea.Cmd) {
	if len(m.displayedRepos) == 0 {
		return m, nil
	}
	url := m.displayedRepos[m.repoCursor].URL
	if url != "" && m.openBrowser != nil {
		_ = m.openBrowser(url)
	}
	return m, nil
}

func (m model) cycleSort() model {
	if m.active == paneList {
		m.sortLists = (m.sortLists + 1) % 4
		sortStarLists(m.lists, m.sortLists)
		m.listCursor = 0
		m.listOffset = 0
	} else {
		m.sortRepos = (m.sortRepos + 1) % 6
		sortRepos(m.displayedRepos, m.sortRepos)
		m.repoCursor = 0
		m.previewOffset = 0
		m.repoOffset = 0
	}
	return m
}

func (m model) handleRefresh() (tea.Model, tea.Cmd) {
	if inv, ok := m.svc.(invalidatable); ok {
		inv.Invalidate()
	}
	m.generation++
	m.repoCache = make(map[repoCacheKey]*repoCacheEntry)
	m.preloadQueue = nil
	m.preloadInFlight = 0
	m.listsLoading = true
	m.err = nil
	m.lists = nil
	m.displayedRepos = nil
	m.focusedList = nil
	m.active = paneList
	m.listCursor = 0
	m.listOffset = 0
	m.repoCursor = 0
	m.previewOffset = 0
	m.repoOffset = 0
	m.cachedRepoSig = ""
	return m, loadListsCmd(m.ctx, m.svc)
}

func (m model) View() tea.View {
	content := m.renderContent()
	v := tea.NewView(content)
	v.AltScreen = true
	if m.mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m model) renderContent() string {
	if m.err != nil {
		return styleError.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\nPress q to quit."
	}
	if m.showHelp {
		return m.renderHelp()
	}
	base := m.renderLayout()
	if m.modal != nil {
		box := styleModalBorder.Render(m.modal.view())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return base
}

func (m model) renderLayout() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	contentH := m.height - 2

	g := calcPaneGeometry(m.width, m.showPreview)

	if m.showPreview {
		// Three-column layout: lists | repos | preview
		leftPane := m.renderListPane(g.leftWidth, contentH)
		midPane := m.renderRepoPane(g.repoWidth, contentH)
		previewPane := m.renderPreviewPane(g.previewWidth, contentH)

		sep := "|"
		rows := make([]string, contentH)
		leftLines := strings.Split(leftPane, "\n")
		midLines := strings.Split(midPane, "\n")
		previewLines := strings.Split(previewPane, "\n")

		for i := 0; i < contentH; i++ {
			l, mid, r := "", "", ""
			if i < len(leftLines) {
				l = leftLines[i]
			}
			if i < len(midLines) {
				mid = midLines[i]
			}
			if i < len(previewLines) {
				r = previewLines[i]
			}
			l = padRight(l, g.leftWidth)
			mid = padRight(mid, g.repoWidth)
			rows[i] = l + sep + mid + sep + r
		}

		header := m.renderHeader()
		footer := m.renderFooter()
		return header + "\n" + strings.Join(rows, "\n") + "\n" + footer
	}

	// Two-column layout.
	leftPane := m.renderListPane(g.leftWidth, contentH)
	rightPane := m.renderRepoPane(g.repoWidth, contentH)

	separator := "|"
	rows := make([]string, contentH)
	leftLines := strings.Split(leftPane, "\n")
	rightLines := strings.Split(rightPane, "\n")

	for i := 0; i < contentH; i++ {
		l := ""
		r := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		l = padRight(l, g.leftWidth)
		rows[i] = l + separator + r
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	body := strings.Join(rows, "\n")
	return header + "\n" + body + "\n" + footer
}

func padRight(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}

func padLeft(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis >= width {
		return s
	}
	return strings.Repeat(" ", width-vis) + s
}

func (m model) renderHeader() string {
	const appName = "gh star-lists"
	appW := lipgloss.Width(appName)

	// Build separator and list name segments.
	sep := styleSeparator.Render(" > ")
	sepW := lipgloss.Width(sep)

	// Sort label (only when non-default).
	sortLabel := m.currentSortLabel()
	sortSuffix := ""
	sortSuffixW := 0
	if sortLabel != "" {
		sortSuffix = "  [sort: " + sortLabel + "]"
		sortSuffixW = lipgloss.Width(sortSuffix)
	}

	if m.focusedList == nil {
		// No list focused: just app name, no sort label.
		return styleAppTitle.Render(appName)
	}

	// Available budget after app name + separator.
	budget := m.width - appW - sepW
	if budget < 0 {
		budget = 0
	}

	// Try to fit: list name + sort suffix.
	listName := m.focusedList.Name
	listNameW := lipgloss.Width(listName)

	if listNameW+sortSuffixW <= budget {
		// Everything fits.
		return styleAppTitle.Render(appName) +
			sep +
			stylePaneTitle.Render(listName) +
			stylePaneSubtitle.Render(sortSuffix)
	}

	// Sort label doesn't fit: drop it, try list name alone.
	if listNameW <= budget {
		return styleAppTitle.Render(appName) +
			sep +
			stylePaneTitle.Render(listName)
	}

	// Truncate list name to budget.
	const ellipsis = "..."
	ellipsisW := len(ellipsis)
	// Trim runes until visual width fits.
	runes := []rune(listName)
	for len(runes) > 0 && lipgloss.Width(string(runes))+ellipsisW > budget {
		runes = runes[:len(runes)-1]
	}
	truncated := string(runes) + ellipsis
	return styleAppTitle.Render(appName) +
		sep +
		stylePaneTitle.Render(truncated)
}

func (m model) currentSortLabel() string {
	if m.active == paneList {
		switch m.sortLists {
		case sortListsName:
			return "name"
		case sortListsRepos:
			return "repos"
		case sortListsAdded:
			return "added"
		default:
			return ""
		}
	}
	switch m.sortRepos {
	case sortReposName:
		return "name"
	case sortReposStars:
		return "stars"
	case sortReposPushed:
		return "pushed"
	case sortReposLanguage:
		return "language"
	case sortReposStarredAt:
		return "starred"
	default:
		return ""
	}
}

// renderHint renders a single (key, description) pair with styling.
func renderHint(k, desc string) string {
	return styleFooterKey.Render(k) + " " + styleFooterText.Render(desc)
}

// joinHints joins rendered hint pairs with two spaces between them.
func joinHints(hints []string) string {
	return strings.Join(hints, "  ")
}

func (m model) renderFooter() string {
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		return styleSuccess.Render(m.statusMsg)
	}
	if m.searchActive {
		return joinHints([]string{
			renderHint("/", "search"),
			renderHint("esc", "clear"),
			renderHint("enter", "done"),
			renderHint("up/down", "navigate"),
		})
	}
	if m.active == paneRepo {
		hints := []string{
			renderHint("/", "search"),
			renderHint("space", "select"),
		}
		if len(m.selected) > 0 {
			hints = append(
				hints,
				styleFooterText.Render(fmt.Sprintf("[%d selected]", len(m.selected))),
			)
		}
		hints = append(hints,
			renderHint("o", "browser"),
			renderHint("?", "help"),
			renderHint("q", "quit"),
		)
		return joinHints(hints)
	}
	return joinHints([]string{
		renderHint("/", "search"),
		renderHint("enter", "open"),
		renderHint("s", "sort"),
		renderHint("?", "help"),
		renderHint("q", "quit"),
	})
}

func (m model) renderListPane(w, h int) string {
	totalH := h
	out := make([]string, 0, totalH)

	if m.searchActive && m.active == paneList {
		// Build search bar with optional N/total count on the right.
		prefix := styleSearchPrompt.Render("/") + " "
		prefixW := lipgloss.Width(prefix)

		countStr := ""
		countW := 0
		total := len(m.lists)
		displayed := len(m.displayedLists)
		candidate := fmt.Sprintf("%d/%d", displayed, total)
		candidateW := lipgloss.Width(candidate)
		// Show count only when at least 4 columns remain for the query after prefix + count + gap.
		if prefixW+4+2+candidateW <= w {
			countStr = stylePaneSubtitle.Render(candidate)
			countW = candidateW
		}

		// Remaining width for query display.
		queryBudget := w - prefixW - countW
		if countW > 0 {
			queryBudget -= 2 // gap between query and count
		}
		if queryBudget < 0 {
			queryBudget = 0
		}

		qDisplay := m.searchQuery
		if lipgloss.Width(qDisplay) > queryBudget {
			// Truncate from left.
			tail := ""
			for _, r := range qDisplay {
				candidate := "..." + tail + string(r)
				if lipgloss.Width(candidate) <= queryBudget {
					tail += string(r)
				}
			}
			qDisplay = "..." + tail
		}

		bar := prefix + qDisplay
		if countStr != "" {
			barW := lipgloss.Width(bar)
			gap := w - barW - countW
			if gap < 1 {
				gap = 1
			}
			bar = bar + strings.Repeat(" ", gap) + countStr
		}
		out = append(out, padRight(bar, w))
		h--
	}

	if m.listsLoading && m.focusedList == nil {
		out = append(out, "  Loading "+m.spinner.View())
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	if len(m.displayedLists) == 0 {
		label := "(no lists)"
		if m.searchQuery != "" {
			q := m.searchQuery
			if utf8.RuneCountInString(q) > 20 {
				q = string([]rune(q)[:20]) + "..."
			}
			label = "(no matches for \"" + q + "\")"
		}
		out = append(out, label)
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	const cursorWidth = 2 // "> " or "  "
	start := m.listOffset
	end := min(start+h, len(m.displayedLists))
	for i := start; i < end; i++ {
		l := m.displayedLists[i]
		cursor := "  "
		isCursor := i == m.listCursor

		// Format count right-side.
		countRaw := fmt.Sprintf("%d", l.RepoCount)
		countStyled := stylePaneSubtitle.Render(countRaw)
		countW := lipgloss.Width(countRaw)

		// Available for name: total - cursor - spacer(1) - count.
		maxNameW := w - cursorWidth - 1 - countW
		if maxNameW < 1 {
			maxNameW = 1
		}

		name := l.Name
		nameW := lipgloss.Width(name)
		if nameW > maxNameW {
			// Truncate with ellipsis.
			const ellipsis = "..."
			ellipsisW := lipgloss.Width(ellipsis)
			runes := []rune(name)
			for len(runes) > 0 && lipgloss.Width(string(runes))+ellipsisW > maxNameW {
				runes = runes[:len(runes)-1]
			}
			name = string(runes) + ellipsis
		}

		if isCursor {
			cursor = "> "
			if m.active == paneList {
				name = styleCursorActive.Render(name)
			} else {
				name = styleCursorInactive.Render(name)
			}
		}

		row := cursor + name
		rowW := lipgloss.Width(row)
		space := w - rowW - countW
		if space < 1 {
			space = 1
		}
		out = append(out, row+strings.Repeat(" ", space)+countStyled)
	}
	for len(out) < totalH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// starGlyph is the Unicode star character (U+2605) used in star count display.
// Must be a Unicode escape (not a raw rune) to satisfy ascii-check.
const starGlyph = "\u2605"

func (m model) renderRepoPane(w, h int) string {
	totalH := h
	out := make([]string, 0, totalH)

	// Search bar (active search in repo pane).
	if m.searchActive && m.active == paneRepo {
		qDisplay := m.searchQuery
		prefix := styleSuccess.Render("/") + " "
		prefixW := 2 // "/" + space
		if lipgloss.Width(prefix)+lipgloss.Width(qDisplay) > w {
			tail := ""
			for _, r := range qDisplay {
				candidate := "..." + string([]rune(tail)) + string(r)
				if prefixW+lipgloss.Width(candidate) <= w {
					tail += string(r)
				}
			}
			qDisplay = "..." + tail
		}
		bar := prefix + qDisplay
		out = append(out, padRight(bar, w))
		h--
	}

	// No list focused yet.
	if m.focusedList == nil {
		out = append(out, styleFaint.Render("(no list selected)"))
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	// Loading/error state: check focused list's cache entry.
	if m.focusedList != nil {
		entry := m.repoCache[repoCacheKey{m.focusedList.ID, m.showPreview}]
		if entry != nil && entry.state == repoCacheError {
			errStr := entry.err.Error()
			const maxErrW = 60
			if len(errStr) > maxErrW {
				errStr = errStr[:maxErrW] + "..."
			}
			out = append(out, styleError.Render("error: "+errStr+"  (ctrl+r to retry)"))
			for len(out) < totalH {
				out = append(out, "")
			}
			return strings.Join(out, "\n")
		}
		if entry == nil || entry.state == repoCacheLoading {
			out = append(out, "  Loading "+m.spinner.View())
			for len(out) < totalH {
				out = append(out, "")
			}
			return strings.Join(out, "\n")
		}
	}

	// Empty state.
	if len(m.displayedRepos) == 0 {
		label := "(no repos)"
		if m.searchQuery != "" {
			q := m.searchQuery
			if utf8.RuneCountInString(q) > 20 {
				q = string([]rune(q)[:20]) + "..."
			}
			label = "(no matches for \"" + q + "\")"
		}
		out = append(out, styleFaint.Render(label))
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	// ---- Width-based feature flags ----
	showBadges := w >= 55
	showLang := w >= 42
	showStars := w >= 30

	hasSel := len(m.selected) > 0

	// ---- Column widths (cached across frames) ----
	(&m).ensureRepoWidths()

	// starWidth: digits + " " + glyph = cachedStarWidth+2; minimum 4.
	starWidth := 4
	if sw := m.cachedStarWidth + 2; sw > starWidth {
		starWidth = sw
	}
	// langWidth: minimum 4; capped at 12 by ensureRepoWidths.
	langWidth := 4
	if lw := m.cachedLangWidth; lw > langWidth {
		langWidth = lw
	}

	start := m.repoOffset
	end := min(start+h, len(m.displayedRepos))

	const (
		cursorW = 2 // "> " or "  "
		markerW = 4 // "[x] " or "[ ] " - only when hasSel
	)

	for i := start; i < end; i++ {
		r := m.displayedRepos[i]
		isCursor := i == m.repoCursor
		_, checked := m.selected[r.NameWithOwner]

		// -- Cursor prefix (2 chars) --
		var cursorStr string
		if isCursor {
			if m.active == paneRepo {
				cursorStr = styleCursorActive.Render("> ")
			} else {
				cursorStr = styleCursorInactive.Render("> ")
			}
		} else {
			cursorStr = "  "
		}

		// -- Selection marker (4 chars, only when hasSel) --
		var markerStr string
		if hasSel {
			if checked {
				markerStr = styleChecked.Render("[x]") + " "
			} else {
				markerStr = styleFaint.Render("[ ]") + " "
			}
		}

		// -- Stars field --
		var starsStr string
		if showStars {
			countRaw := fmt.Sprintf("%d", r.StargazerCount)
			// Right-align count within (starWidth - 2) then append " " + glyph.
			countFieldW := starWidth - 2 // space + glyph
			if countFieldW < 1 {
				countFieldW = 1
			}
			paddedCount := padLeft(countRaw, countFieldW)
			starsStr = styleRepoStars.Render(paddedCount+" "+starGlyph) + "  "
		}

		// -- Language field --
		var langStr string
		if showLang {
			lang := r.Language
			if lang == "" {
				langStr = styleFaint.Render(padRight("-", langWidth)) + "  "
			} else {
				lw := lipgloss.Width(lang)
				if lw > langWidth {
					// Truncate with ellipsis.
					runes := []rune(lang)
					for len(runes) > 0 && lipgloss.Width(string(runes))+3 > langWidth {
						runes = runes[:len(runes)-1]
					}
					lang = string(runes) + "..."
				}
				langStr = styleRepoLanguage.Render(padRight(lang, langWidth)) + "  "
			}
		}

		// -- Compute remaining width for name (and optional badges + age) --
		fixedW := cursorW
		if hasSel {
			fixedW += markerW
		}
		if showStars {
			fixedW += starWidth + 2 // field + two spaces separator
		}
		if showLang {
			fixedW += langWidth + 2 // field + two spaces separator
		}
		nameAvail := w - fixedW
		if nameAvail < 12 {
			nameAvail = 12
		}

		// -- Badges --
		var badgesRaw string
		if showBadges {
			if r.IsFork {
				badgesRaw += " fork"
			}
			if r.IsArchived {
				badgesRaw += " archived"
			}
		}

		// -- Name: truncate raw, then style --
		nameRaw := r.NameWithOwner
		// Reserve space for badges if they exist.
		badgesW := lipgloss.Width(badgesRaw)
		nameMaxW := nameAvail
		if badgesW > 0 && nameAvail > badgesW+6 {
			nameMaxW = nameAvail - badgesW
		} else {
			// Not enough room for badges.
			badgesRaw = ""
		}

		if lipgloss.Width(nameRaw) > nameMaxW {
			const ellipsis = "..."
			runes := []rune(nameRaw)
			for len(runes) > 0 && lipgloss.Width(string(runes))+lipgloss.Width(ellipsis) > nameMaxW {
				runes = runes[:len(runes)-1]
			}
			nameRaw = string(runes) + ellipsis
		}

		var nameStr string
		if isCursor {
			if m.active == paneRepo {
				nameStr = styleRepoNameFocused.Render(nameRaw)
			} else {
				nameStr = styleRepoNameInactive.Render(nameRaw)
			}
		} else {
			nameStr = styleRepoName.Render(nameRaw)
		}

		var badgesStr string
		if badgesRaw != "" {
			badgesStr = styleRepoBadge.Render(badgesRaw)
		}

		// -- Assemble row --
		row := cursorStr + markerStr + starsStr + langStr + nameStr + badgesStr

		out = append(out, row)
	}

	for len(out) < totalH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// truncateToWidth truncates a raw (unstyled) string so its visual width is at
// most maxW. Returns the (possibly shortened) string.
func truncateToWidth(s string, maxW int) string {
	if lipgloss.Width(s) <= maxW {
		return s
	}
	const ellipsis = "..."
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+lipgloss.Width(ellipsis) > maxW {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

// previewContentLines builds the full list of styled content lines for the focused
// repo in the preview pane. The returned slice is not clipped to any height; the
// caller applies the scroll offset and pads to the desired height.
func (m model) previewContentLines(w int) []string {
	repo := m.displayedRepos[m.repoCursor]
	// Use the withTopics=true cache entry for topic data when available.
	if m.focusedList != nil {
		if e := m.repoCache[repoCacheKey{m.focusedList.ID, true}]; e != nil &&
			e.state == repoCacheLoaded && m.repoCursor < len(e.repos) {
			repo = e.repos[m.repoCursor]
		}
	}

	maxW := w - 2
	if maxW < 1 {
		maxW = 1
	}

	now := time.Now().UTC()
	var lines []string

	// Line 1: NameWithOwner
	lines = append(lines, stylePaneTitle.Render(truncateToWidth(repo.NameWithOwner, maxW)))

	// Line 2: URL
	lines = append(lines, styleRepoURL.Render(truncateToWidth(repo.URL, maxW)))

	// Blank line
	lines = append(lines, "")

	// Line 4: stars  language  badge
	starsStr := styleRepoStars.Render(
		fmt.Sprintf("%s %s", formatStars(repo.StargazerCount), starGlyph),
	)

	langStr := ""
	if repo.Language != "" {
		langStr = "  " + styleRepoLanguage.Render(repo.Language)
	} else {
		langStr = "  " + styleEmptyState.Render("-")
	}

	var badge string
	switch {
	case repo.IsArchived:
		badge = "  " + styleRepoBadge.Render("archived")
	case repo.IsFork:
		badge = "  " + styleRepoBadge.Render("fork")
	default:
		badge = "  " + styleRepoBadge.Render("source")
	}

	lines = append(lines, starsStr+langStr+badge)

	// Blank line
	lines = append(lines, "")

	// Description
	lines = append(lines, stylePaneSubtitle.Render("Description"))
	if repo.Description != "" {
		lines = append(lines, styleRepoName.Render(truncateToWidth(repo.Description, maxW)))
	} else {
		lines = append(lines, styleEmptyState.Render("(no description)"))
	}

	// Blank line
	lines = append(lines, "")

	// License
	licenseVal := repo.License
	if licenseVal == "" {
		licenseVal = styleEmptyState.Render("-")
	}
	lines = append(lines, stylePaneSubtitle.Render("License:")+" "+licenseVal)

	// Pushed
	lines = append(lines, stylePaneSubtitle.Render("Pushed:")+" "+shortAge(repo.PushedAt, now))

	// Starred
	starredVal := repo.StarredAt
	if starredVal == "" {
		starredVal = styleEmptyState.Render("-")
	} else {
		starredVal = shortAge(repo.StarredAt, now)
	}
	lines = append(lines, stylePaneSubtitle.Render("Starred:")+" "+starredVal)

	// Topics
	topicsVal := ""
	if len(repo.Topics) > 0 {
		topicsVal = truncateToWidth(strings.Join(repo.Topics, ", "), maxW)
	} else {
		topicsVal = styleEmptyState.Render("-")
	}
	lines = append(lines, stylePaneSubtitle.Render("Topics:")+" "+topicsVal)

	return lines
}

func (m model) renderPreviewPane(w, h int) string {
	if m.active != paneRepo || len(m.displayedRepos) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Render("(select a repo)")
	}

	lines := m.previewContentLines(w)

	// Apply scroll offset: slice [previewOffset, previewOffset+h].
	contentLen := len(lines)
	start := m.previewOffset
	if start > contentLen {
		start = contentLen
	}
	end := start + h
	if end > contentLen {
		end = contentLen
	}
	visible := lines[start:end]
	// Pad to height.
	for len(visible) < h {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}

func formatStars(n int) string {
	switch {
	case n >= 1000000:
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func (m model) renderHelp() string {
	// Narrow terminal fallback: single-column list.
	if m.width > 0 && m.width < 50 {
		lines := []string{
			stylePaneTitle.Render("Key Bindings"),
			"",
			fmt.Sprintf("  %-16s %s", "up/k", "move up"),
			fmt.Sprintf("  %-16s %s", "down/j", "move down"),
			fmt.Sprintf("  %-16s %s", "pgup", "page up"),
			fmt.Sprintf("  %-16s %s", "pgdn", "page down"),
			fmt.Sprintf("  %-16s %s", "g", "top"),
			fmt.Sprintf("  %-16s %s", "G", "bottom"),
			fmt.Sprintf("  %-16s %s", "left", "focus lists"),
			fmt.Sprintf("  %-16s %s", "right", "focus repos"),
			fmt.Sprintf("  %-16s %s", "enter", "open/select"),
			fmt.Sprintf("  %-16s %s", "esc", "back/quit"),
			fmt.Sprintf("  %-16s %s", "/", "search"),
			fmt.Sprintf("  %-16s %s", "space", "select"),
			fmt.Sprintf("  %-16s %s", "a", "add repo"),
			fmt.Sprintf("  %-16s %s", "x", "remove repo"),
			fmt.Sprintf("  %-16s %s", "m", "move repo"),
			fmt.Sprintf("  %-16s %s", "u", "unstar repo"),
			fmt.Sprintf("  %-16s %s", "p", "preview"),
			fmt.Sprintf("  %-16s %s", "o", "open browser"),
			fmt.Sprintf("  %-16s %s", "n/e/d", "list CRUD"),
			fmt.Sprintf("  %-16s %s", "c/C", "copy/merge"),
			fmt.Sprintf("  %-16s %s", "ctrl+r", "refresh"),
			fmt.Sprintf("  %-16s %s", "?", "toggle help"),
			fmt.Sprintf("  %-16s %s", "q", "quit"),
			"",
			styleFaint.Render("Press ? to close"),
		}
		return strings.Join(lines, "\n")
	}

	// Two-column table: Navigation | Actions.
	left := []string{
		"up/k   move up",
		"down/j move down",
		"pgup   page up",
		"pgdn   page down",
		"g      top",
		"G      bottom",
		"left   focus lists",
		"right  focus repos",
		"enter  open/select",
		"esc    back/quit",
		"?      toggle help",
	}
	right := []string{
		"/      search",
		"space  select",
		"a      add repo",
		"x      remove repo",
		"m      move repo",
		"u      unstar repo",
		"p      preview",
		"o      open browser",
		"n/e/d  list CRUD",
		"c/C    copy/merge",
		"ctrl+r refresh",
		"q      quit",
	}

	// Pad the shorter column with empty strings.
	for len(left) < len(right) {
		left = append(left, "")
	}
	for len(right) < len(left) {
		right = append(right, "")
	}

	lines := []string{
		stylePaneTitle.Render("Key Bindings"),
		"",
		fmt.Sprintf("  %-22s  %s", "Navigation", "Actions"),
		fmt.Sprintf("  %-22s  %s", "----------", "----------"),
	}
	for i := range left {
		lines = append(lines, fmt.Sprintf("  %-22s  %s", left[i], right[i]))
	}
	lines = append(lines, "")
	lines = append(lines, styleFaint.Render("Press ? to close"))
	return strings.Join(lines, "\n")
}

// shortAge formats an RFC3339 timestamp as a human-readable age string.
func shortAge(value string, now time.Time) string {
	if value == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if parsed.After(now) {
		return parsed.Format("2006-01-02")
	}
	duration := now.Sub(parsed)
	switch {
	case duration < time.Minute:
		return "now"
	case duration < time.Hour:
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	case duration < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	case duration < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(duration.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(duration.Hours()/(24*365)))
	}
}
