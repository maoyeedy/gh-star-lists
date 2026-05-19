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

type model struct {
	svc         githubapi.Service
	openBrowser func(string) error
	noColor     bool
	mouse       bool
	ctx         context.Context

	active      pane
	lists       []githubapi.StarList
	repos       []githubapi.Repository
	focusedList *githubapi.StarList

	listCursor int
	listOffset int
	repoCursor int
	repoOffset int

	sortLists sortListsKey
	sortRepos sortReposKey

	loading bool
	err     error

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

	selected       map[string]struct{} // NameWithOwner of checked repos
	bulkFailedNWOs []string

	spinnerFrame int
}

func newModel(ctx context.Context, svc githubapi.Service, opts Options) model {
	return model{
		svc:         svc,
		openBrowser: opts.OpenBrowser,
		noColor:     opts.NoColor,
		mouse:       opts.Mouse,
		ctx:         ctx,
		loading:     true,
	}
}

// invalidatable is satisfied by cacheService.
type invalidatable interface {
	Invalidate()
}

type spinnerTickMsg struct{}

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(loadListsCmd(m.ctx, m.svc), spinnerTickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case listsLoadedMsg:
		m.lists = msg.lists
		m.loading = false
		sortStarLists(m.lists, m.sortLists)
		m = m.rebuildDisplayed()
		if m.listCursor >= len(m.displayedLists) && len(m.displayedLists) > 0 {
			m.listCursor = len(m.displayedLists) - 1
		}
		return m, nil

	case reposLoadedMsg:
		m.repos = msg.repos
		m.loading = false
		sortRepos(m.repos, m.sortRepos)
		m = m.rebuildDisplayed()
		if m.repoCursor >= len(m.displayedRepos) && len(m.displayedRepos) > 0 {
			m.repoCursor = len(m.displayedRepos) - 1
		}
		// update focusedList from current lists
		for i := range m.lists {
			if m.lists[i].ID == msg.listID {
				m.focusedList = &m.lists[i]
				break
			}
		}
		// Drop selected keys that no longer exist in the refreshed repo list.
		if len(m.selected) > 0 {
			existing := make(map[string]struct{}, len(m.repos))
			for _, r := range m.repos {
				existing[r.NameWithOwner] = struct{}{}
			}
			for nwo := range m.selected {
				if _, ok := existing[nwo]; !ok {
					delete(m.selected, nwo)
				}
			}
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case mutationDoneMsg:
		m.modal = nil
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.statusMsg = "Done."
		m.statusExpiry = time.Now().Add(2 * time.Second)
		m.loading = true
		cmds := []tea.Cmd{loadListsCmd(m.ctx, m.svc), statusClearCmd(m.statusExpiry)}
		if m.active == paneRepo && m.focusedList != nil {
			cmds = append(cmds, loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.showPreview))
		}
		return m, tea.Batch(cmds...)

	case bulkDoneMsg:
		m.modal = nil
		m.selected = nil
		if msg.failed > 0 && msg.succeeded == 0 {
			m.err = fmt.Errorf("%d repos failed to %s", msg.failed, msg.verb)
			return m, nil
		}
		if msg.failed > 0 {
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
		} else {
			m.statusMsg = fmt.Sprintf("%d repos %s.", msg.succeeded, msg.verb)
		}
		m.statusExpiry = time.Now().Add(2 * time.Second)
		if msg.failed > 0 {
			m.bulkFailedNWOs = msg.failedNWOs
		} else {
			m.bulkFailedNWOs = nil
		}
		m.loading = true
		cmds := []tea.Cmd{loadListsCmd(m.ctx, m.svc), statusClearCmd(m.statusExpiry)}
		if m.active == paneRepo && m.focusedList != nil {
			cmds = append(cmds, loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.showPreview))
		}
		return m, tea.Batch(cmds...)

	case statusExpiredMsg:
		m.statusMsg = ""
		return m, nil

	case tea.MouseWheelMsg:
		if m.modal != nil || m.searchActive {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			m = m.moveCursor(-1)
		case tea.MouseWheelDown:
			m = m.moveCursor(1)
		}
		return m, nil

	case tea.MouseClickMsg:
		if m.modal != nil || m.searchActive {
			return m, nil
		}
		m = m.handleMouseClick(msg)
		return m, nil

	case spinnerTickMsg:
		if m.loading {
			m.spinnerFrame = (m.spinnerFrame + 1) % 4
			return m, spinnerTickCmd()
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.modal != nil {
			updated, cmd := m.modal.update(msg)
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
		if m.active == paneList && m.focusedList != nil && len(m.repos) > 0 {
			m.active = paneRepo
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		m = m.moveCursor(-1)
		return m, nil

	case key.Matches(msg, keys.Down):
		m = m.moveCursor(1)
		return m, nil

	case key.Matches(msg, keys.PgUp):
		paneH := max(1, m.height-2)
		if m.active == paneList {
			m.listCursor = clampInt(m.listCursor-(paneH-1), 0, len(m.displayedLists)-1)
			m = m.slideListOffset()
		} else {
			m.repoCursor = clampInt(m.repoCursor-(paneH-1), 0, len(m.displayedRepos)-1)
			m = m.slideRepoOffset()
		}
		return m, nil

	case key.Matches(msg, keys.PgDn):
		paneH := max(1, m.height-2)
		if m.active == paneList {
			m.listCursor = clampInt(m.listCursor+(paneH-1), 0, len(m.displayedLists)-1)
			m = m.slideListOffset()
		} else {
			m.repoCursor = clampInt(m.repoCursor+(paneH-1), 0, len(m.displayedRepos)-1)
			m = m.slideRepoOffset()
		}
		return m, nil

	case key.Matches(msg, keys.Home):
		if m.active == paneList {
			m.listCursor = 0
			m.listOffset = 0
		} else {
			m.repoCursor = 0
			m.repoOffset = 0
		}
		return m, nil

	case key.Matches(msg, keys.End):
		if m.active == paneList {
			m.listCursor = max(0, len(m.displayedLists)-1)
			m = m.slideListOffset()
		} else {
			m.repoCursor = max(0, len(m.displayedRepos)-1)
			m = m.slideRepoOffset()
		}
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
		if m.showPreview && m.active == paneRepo && m.focusedList != nil {
			m.loading = true
			return m, loadReposCmd(m.ctx, m.svc, m.focusedList.ID, true)
		}
		return m, nil

	case key.Matches(msg, keys.Search):
		m.searchActive = true
		m.searchQuery = ""
		m.listCursor = 0
		m.repoCursor = 0
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

func (m model) moveCursor(delta int) model {
	if m.active == paneList {
		m.listCursor = clampInt(m.listCursor+delta, 0, len(m.displayedLists)-1)
		m = m.slideListOffset()
	} else {
		m.repoCursor = clampInt(m.repoCursor+delta, 0, len(m.displayedRepos)-1)
		m = m.slideRepoOffset()
	}
	return m
}

func (m model) handleMouseClick(msg tea.MouseClickMsg) model {
	contentRow := msg.Y - 1 // row 0 is the header
	if contentRow < 0 {
		return m
	}
	leftW := 36
	if m.width > 100 {
		leftW = m.width * 30 / 100
	}
	if msg.X <= leftW {
		m.active = paneList
		idx := contentRow + m.listOffset
		if idx >= 0 && idx < len(m.displayedLists) {
			m.listCursor = idx
		}
	} else if msg.X > leftW+1 && m.focusedList != nil && len(m.displayedRepos) > 0 {
		m.active = paneRepo
		idx := contentRow + m.repoOffset
		if idx >= 0 && idx < len(m.displayedRepos) {
			m.repoCursor = idx
		}
	}
	return m
}

func (m model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.searchActive = false
		m.searchQuery = ""
		m = m.rebuildDisplayed()
		m.listCursor = 0
		m.repoCursor = 0
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

func (m model) slideRepoOffset() model {
	paneH := max(1, m.height-2)
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
	if m.searchQuery == "" {
		m.displayedLists = m.lists
		m.displayedRepos = m.repos
	} else {
		m.displayedLists = search.FilterStarLists(m.lists, m.searchQuery)
		m.displayedRepos = search.FilterRepositories(m.repos, m.searchQuery)
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

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		if len(m.displayedLists) == 0 {
			return m, nil
		}
		focused := m.displayedLists[m.listCursor]
		// Find pointer into backing slice so reposLoadedMsg can resolve it.
		m.focusedList = nil
		for i := range m.lists {
			if m.lists[i].ID == focused.ID {
				m.focusedList = &m.lists[i]
				break
			}
		}
		m.active = paneRepo
		m.repos = nil
		m.repoCursor = 0
		m.repoOffset = 0
		m.loading = true
		return m, loadReposCmd(m.ctx, m.svc, focused.ID, m.showPreview)
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
		sortRepos(m.repos, m.sortRepos)
		m.repoCursor = 0
		m.repoOffset = 0
	}
	return m
}

func (m model) handleRefresh() (tea.Model, tea.Cmd) {
	if inv, ok := m.svc.(invalidatable); ok {
		inv.Invalidate()
	}
	m.loading = true
	m.err = nil
	if m.active == paneRepo && m.focusedList != nil {
		return m, loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.showPreview)
	}
	m.lists = nil
	m.repos = nil
	m.focusedList = nil
	m.active = paneList
	m.listCursor = 0
	m.listOffset = 0
	m.repoCursor = 0
	m.repoOffset = 0
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

	if m.showPreview {
		// Three-column layout: lists | repos | preview
		leftW := 28
		midW := 36
		if m.width > 120 {
			leftW = m.width * 22 / 100
			midW = m.width * 28 / 100
		}
		sepW := 1
		previewW := m.width - leftW - midW - 2*sepW
		if previewW < 20 {
			previewW = 20
		}

		leftPane := m.renderListPane(leftW, contentH)
		midPane := m.renderRepoPane(midW, contentH)
		previewPane := m.renderPreviewPane(previewW, contentH)

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
			l = padRight(l, leftW)
			mid = padRight(mid, midW)
			rows[i] = l + sep + mid + sep + r
		}

		header := m.renderHeader()
		footer := m.renderFooter()
		return header + "\n" + strings.Join(rows, "\n") + "\n" + footer
	}

	// Original two-column layout (unchanged).
	leftW := 36
	if m.width > 100 {
		leftW = m.width * 30 / 100
	}
	sepW := 1
	rightW := m.width - leftW - sepW
	if rightW < 10 {
		rightW = 10
	}

	leftPane := m.renderListPane(leftW, contentH)
	rightPane := m.renderRepoPane(rightW, contentH)

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
		l = padRight(l, leftW)
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
	title := "gh star-lists browse"
	if m.focusedList != nil {
		title = fmt.Sprintf("gh star-lists browse > %s", m.focusedList.Name)
	}
	sortLabel := m.currentSortLabel()
	if sortLabel != "" {
		title = fmt.Sprintf("%s  [sort: %s]", title, sortLabel)
	}
	return stylePaneTitle.Render(title)
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

func (m model) renderFooter() string {
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		return styleSuccess.Render(m.statusMsg)
	}
	if m.searchActive {
		return styleFooter.Render("/:search  esc:clear  enter:done  up/down:navigate")
	}
	var hints string
	if m.active == paneRepo {
		selHint := ""
		if len(m.selected) > 0 {
			selHint = fmt.Sprintf("  [%d selected]", len(m.selected))
		}
		hints = "/ search  space select" + selHint + "  o browser  ? help  q quit"
	} else {
		hints = "/ search  enter open  s sort  ? help  q quit"
	}
	return styleFooter.Render(hints)
}

func (m model) renderListPane(w, h int) string {
	totalH := h
	out := make([]string, 0, totalH)

	if m.searchActive && m.active == paneList {
		qDisplay := m.searchQuery
		prefix := styleSuccess.Render("/") + " "
		prefixW := 2 // "/" + space
		if lipgloss.Width(prefix)+lipgloss.Width(qDisplay) > w {
			// Truncate from the left: show "/ ..." + tail.
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

	if m.loading && m.focusedList == nil {
		frame := []string{"|", "/", "-", "\\"}[m.spinnerFrame]
		out = append(out, "  Loading "+frame)
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

	start := m.listOffset
	end := min(start+h, len(m.displayedLists))
	for i := start; i < end; i++ {
		l := m.displayedLists[i]
		cursor := "  "
		name := l.Name
		if i == m.listCursor {
			cursor = "> "
			if m.active == paneList {
				name = styleCursorActive.Render(name)
			} else {
				name = styleCursorInactive.Render(name)
			}
		}
		age := shortAge(l.LastAddedAt, time.Now().UTC())
		repoStr := padLeft(fmt.Sprintf("%d", l.RepoCount), 4)
		ageStr := padLeft(age, 8)
		right := repoStr + " | " + ageStr
		right = styleFaint.Render(right)
		row := cursor + name
		rowW := lipgloss.Width(row)
		rightW := lipgloss.Width(right)
		space := w - rowW - rightW - 2
		if space < 1 {
			space = 1
		}
		out = append(out, row+strings.Repeat(" ", space)+right)
	}
	for len(out) < totalH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func (m model) renderRepoPane(w, h int) string {
	totalH := h
	out := make([]string, 0, totalH)

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

	if m.focusedList == nil {
		out = append(out, "(press enter to view repos)")
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	if m.loading && m.focusedList != nil {
		frame := []string{"|", "/", "-", "\\"}[m.spinnerFrame]
		out = append(out, "  Loading "+frame)
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	if len(m.displayedRepos) == 0 {
		label := "(no repos)"
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

	showMeta := w >= 60
	hasSel := len(m.selected) > 0
	start := m.repoOffset
	end := min(start+h, len(m.displayedRepos))
	for i := start; i < end; i++ {
		r := m.displayedRepos[i]
		cursor := "  "
		name := r.NameWithOwner
		_, checked := m.selected[r.NameWithOwner]
		if hasSel {
			if checked {
				name = styleChecked.Render("[x] " + name)
			} else {
				name = "[ ] " + name
			}
		}
		if i == m.repoCursor {
			cursor = "> "
			if m.active == paneRepo {
				name = styleCursorActive.Render(name)
			} else {
				name = styleCursorInactive.Render(name)
			}
		}
		if !showMeta {
			out = append(out, cursor+name)
			continue
		}
		lang := r.Language
		if lang == "" {
			lang = "    "
		}
		stars := formatStars(r.StargazerCount)
		pushed := shortAge(r.PushedAt, time.Now().UTC())
		meta := fmt.Sprintf("%-8s %6s* %s", lang, stars, pushed)
		meta = styleFaint.Render(meta)
		row := cursor + name
		rowW := lipgloss.Width(row)
		metaW := lipgloss.Width(meta)
		// Ensure at least 2-space gap; truncate title if needed.
		available := w - metaW - 2
		if rowW > available {
			// Clip name to fit: cursor is 2 bytes, clip name portion.
			// available - 2 (cursor width) - 3 (...) characters for the name runes
			maxNameRunes := available - 2 - 3
			if maxNameRunes < 1 {
				maxNameRunes = 1
			}
			if utf8.RuneCountInString(name) > maxNameRunes {
				name = string([]rune(name)[:maxNameRunes]) + "..."
			}
			row = cursor + name
			rowW = lipgloss.Width(row)
		}
		space := w - rowW - metaW - 2
		if space < 1 {
			space = 1
		}
		out = append(out, row+strings.Repeat(" ", space)+meta)
	}
	for len(out) < totalH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func (m model) renderPreviewPane(w, h int) string {
	if m.active != paneRepo || len(m.displayedRepos) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Render("(select a repo)")
	}
	repo := m.displayedRepos[m.repoCursor]

	archived := ""
	if repo.IsArchived {
		archived = "  [archived]"
	}
	fork := ""
	if repo.IsFork {
		fork = "  [fork]"
	}

	lang := repo.Language
	if lang == "" {
		lang = "-"
	}
	license := repo.License
	if license == "" {
		license = "-"
	}
	pushed := shortAge(repo.PushedAt, time.Now().UTC())
	starredAt := repo.StarredAt
	if starredAt == "" {
		starredAt = "-"
	}

	topics := "-"
	if len(repo.Topics) > 0 {
		topics = strings.Join(repo.Topics, ", ")
	}

	lines := []string{
		styleSelected.Render(repo.NameWithOwner) + archived + fork,
		styleFaint.Render(repo.URL),
		"",
	}
	if repo.Description != "" {
		lines = append(lines, repo.Description, "")
	}
	lines = append(lines,
		styleFaint.Render("Language:")+" "+lang,
		styleFaint.Render("License: ")+" "+license,
		styleFaint.Render("Pushed:  ")+" "+pushed,
		styleFaint.Render("Starred: ")+" "+starredAt,
		styleFaint.Render("Topics:  ")+" "+topics,
	)

	// Pad to height.
	result := strings.Join(lines, "\n")
	resultLines := strings.Split(result, "\n")
	for len(resultLines) < h {
		resultLines = append(resultLines, "")
	}
	if len(resultLines) > h {
		resultLines = resultLines[:h]
	}
	return strings.Join(resultLines, "\n")
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
