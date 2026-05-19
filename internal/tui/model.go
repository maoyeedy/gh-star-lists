package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
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
}

func newModel(ctx context.Context, svc githubapi.Service, opts Options) model {
	return model{
		svc:         svc,
		openBrowser: opts.OpenBrowser,
		noColor:     opts.NoColor,
		ctx:         ctx,
		loading:     true,
	}
}

// invalidatable is satisfied by cacheService.
type invalidatable interface {
	Invalidate()
}

func (m model) Init() tea.Cmd {
	return loadListsCmd(m.ctx, m.svc)
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
		if m.listCursor >= len(m.lists) && len(m.lists) > 0 {
			m.listCursor = len(m.lists) - 1
		}
		return m, nil

	case reposLoadedMsg:
		m.repos = msg.repos
		m.loading = false
		sortRepos(m.repos, m.sortRepos)
		if m.repoCursor >= len(m.repos) && len(m.repos) > 0 {
			m.repoCursor = len(m.repos) - 1
		}
		// update focusedList from current lists
		for i := range m.lists {
			if m.lists[i].ID == msg.listID {
				m.focusedList = &m.lists[i]
				break
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

	case statusExpiredMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyPressMsg:
		if m.modal != nil {
			updated, cmd := m.modal.update(msg)
			m.modal = updated
			return m, cmd
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
		if m.active == paneRepo {
			m.active = paneList
			m.repos = nil
			m.focusedList = nil
			m.repoCursor = 0
			m.repoOffset = 0
			return m, nil
		}
		return m, tea.Quit

	case key.Matches(msg, keys.Up):
		m = m.moveCursor(-1)
		return m, nil

	case key.Matches(msg, keys.Down):
		m = m.moveCursor(1)
		return m, nil

	case key.Matches(msg, keys.PgUp):
		paneH := max(1, m.height-2)
		if m.active == paneList {
			m.listCursor = clampInt(m.listCursor-(paneH-1), 0, len(m.lists)-1)
			m = m.slideListOffset()
		} else {
			m.repoCursor = clampInt(m.repoCursor-(paneH-1), 0, len(m.repos)-1)
			m = m.slideRepoOffset()
		}
		return m, nil

	case key.Matches(msg, keys.PgDn):
		paneH := max(1, m.height-2)
		if m.active == paneList {
			m.listCursor = clampInt(m.listCursor+(paneH-1), 0, len(m.lists)-1)
			m = m.slideListOffset()
		} else {
			m.repoCursor = clampInt(m.repoCursor+(paneH-1), 0, len(m.repos)-1)
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
			m.listCursor = max(0, len(m.lists)-1)
			m = m.slideListOffset()
		} else {
			m.repoCursor = max(0, len(m.repos)-1)
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
		if m.active != paneList || len(m.lists) == 0 {
			return m, nil
		}
		mo, focusCmd := newEditListModal(m.ctx, m.svc, m.lists[m.listCursor])
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.DeleteList):
		if m.active != paneList || len(m.lists) == 0 {
			return m, nil
		}
		mo, focusCmd := newDeleteListModal(m.ctx, m.svc, m.lists[m.listCursor])
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.AddRepo):
		if m.active != paneRepo || len(m.repos) == 0 || len(m.lists) == 0 {
			return m, nil
		}
		repo := m.repos[m.repoCursor]
		m.modal = newAddRepoModal(m.ctx, m.svc, repo, m.lists)
		return m, nil

	case key.Matches(msg, keys.RemoveRepo):
		if m.active != paneRepo || len(m.repos) == 0 || m.focusedList == nil {
			return m, nil
		}
		repo := m.repos[m.repoCursor]
		m.modal = newRemoveRepoModal(m.ctx, m.svc, repo, m.focusedList.ID)
		return m, nil

	case key.Matches(msg, keys.MoveRepo):
		if m.active != paneRepo || len(m.repos) == 0 || m.focusedList == nil || len(m.lists) < 2 {
			return m, nil
		}
		repo := m.repos[m.repoCursor]
		m.modal = newMoveRepoModal(m.ctx, m.svc, repo, m.lists, m.focusedList.ID)
		return m, nil

	case key.Matches(msg, keys.UnstarRepo):
		if m.active != paneRepo || len(m.repos) == 0 {
			return m, nil
		}
		repo := m.repos[m.repoCursor]
		mo, focusCmd := newUnstarRepoModal(m.ctx, m.svc, repo)
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.CopyList):
		if m.active != paneList || len(m.lists) == 0 || len(m.lists) < 2 {
			return m, nil
		}
		m.modal = newCopyListModal(m.ctx, m.svc, m.lists[m.listCursor], m.lists)
		return m, nil

	case key.Matches(msg, keys.MergeList):
		if m.active != paneList || len(m.lists) == 0 || len(m.lists) < 2 {
			return m, nil
		}
		m.modal = newMergeListModal(m.ctx, m.svc, m.lists[m.listCursor], m.lists)
		return m, nil

	case key.Matches(msg, keys.Preview):
		m.showPreview = !m.showPreview
		if m.showPreview && m.active == paneRepo && m.focusedList != nil {
			m.loading = true
			return m, loadReposCmd(m.ctx, m.svc, m.focusedList.ID, true)
		}
		return m, nil
	}

	return m, nil
}

func (m model) moveCursor(delta int) model {
	if m.active == paneList {
		m.listCursor = clampInt(m.listCursor+delta, 0, len(m.lists)-1)
		m = m.slideListOffset()
	} else {
		m.repoCursor = clampInt(m.repoCursor+delta, 0, len(m.repos)-1)
		m = m.slideRepoOffset()
	}
	return m
}

func (m model) slideListOffset() model {
	paneH := max(1, m.height-2)
	if m.listCursor < m.listOffset {
		m.listOffset = m.listCursor
	} else if m.listCursor >= m.listOffset+paneH {
		m.listOffset = m.listCursor - paneH + 1
	}
	m.listOffset = clampInt(m.listOffset, 0, max(0, len(m.lists)-paneH))
	return m
}

func (m model) slideRepoOffset() model {
	paneH := max(1, m.height-2)
	if m.repoCursor < m.repoOffset {
		m.repoOffset = m.repoCursor
	} else if m.repoCursor >= m.repoOffset+paneH {
		m.repoOffset = m.repoCursor - paneH + 1
	}
	m.repoOffset = clampInt(m.repoOffset, 0, max(0, len(m.repos)-paneH))
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

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		if len(m.lists) == 0 {
			return m, nil
		}
		focused := m.lists[m.listCursor]
		m.focusedList = &m.lists[m.listCursor]
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
		if len(m.lists) == 0 {
			return m, nil
		}
		url := m.lists[m.listCursor].URL
		if url != "" && m.openBrowser != nil {
			_ = m.openBrowser(url)
		}
		return m, nil
	}
	return m.openFocusedRepoURL()
}

func (m model) openFocusedRepoURL() (tea.Model, tea.Cmd) {
	if len(m.repos) == 0 {
		return m, nil
	}
	url := m.repos[m.repoCursor].URL
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
	return v
}

func (m model) renderContent() string {
	if m.err != nil {
		return styleError.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\nPress q to quit."
	}
	if m.loading {
		return "Loading..."
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
	var hints string
	if m.active == paneRepo {
		hints = "a:add  x:remove  m:move  u:unstar  p:preview  enter/o:open  esc:back  s:sort  pg/g/G:scroll  ?:help  q:quit"
	} else {
		hints = "n:new  e:edit  d:del  c:copy  C:merge  enter:select  o:open  s:sort  ctrl+r:refresh  pg/g/G:scroll  ?:help  q:quit"
	}
	return styleFooter.Render(hints)
}

func (m model) renderListPane(w, h int) string {
	if len(m.lists) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Render("(no lists)")
	}
	lines := make([]string, 0, h)
	start := m.listOffset
	end := min(start+h, len(m.lists))
	for i := start; i < end; i++ {
		l := m.lists[i]
		cursor := "  "
		name := l.Name
		if i == m.listCursor {
			cursor = "> "
			name = styleSelected.Render(name)
		}
		age := shortAge(l.LastAddedAt, time.Now().UTC())
		right := fmt.Sprintf("(%d repos, %s)", l.RepoCount, age)
		right = styleFaint.Render(right)
		row := cursor + name
		rowW := lipgloss.Width(row)
		rightW := lipgloss.Width(right)
		space := w - rowW - rightW - 2
		if space < 1 {
			space = 1
		}
		row = row + strings.Repeat(" ", space) + right
		lines = append(lines, row)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m model) renderRepoPane(w, h int) string {
	if m.focusedList == nil {
		placeholder := "(press enter to view repos)"
		return lipgloss.NewStyle().Width(w).Height(h).Render(placeholder)
	}
	if len(m.repos) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Render("(no repos)")
	}
	lines := make([]string, 0, h)
	start := m.repoOffset
	end := min(start+h, len(m.repos))
	for i := start; i < end; i++ {
		r := m.repos[i]
		cursor := "  "
		name := r.NameWithOwner
		if i == m.repoCursor {
			cursor = "> "
			name = styleSelected.Render(name)
		}
		lang := r.Language
		if lang == "" {
			lang = "    "
		} else if len(lang) > 8 {
			lang = lang[:8]
		}
		stars := formatStars(r.StargazerCount)
		pushed := shortAge(r.PushedAt, time.Now().UTC())
		meta := fmt.Sprintf("%-8s %6s* %s", lang, stars, pushed)
		meta = styleFaint.Render(meta)
		row := cursor + name
		rowW := lipgloss.Width(row)
		metaW := lipgloss.Width(meta)
		space := w - rowW - metaW - 2
		if space < 1 {
			space = 1
		}
		row = row + strings.Repeat(" ", space) + meta
		lines = append(lines, row)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m model) renderPreviewPane(w, h int) string {
	if m.active != paneRepo || len(m.repos) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Render("(select a repo)")
	}
	repo := m.repos[m.repoCursor]

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
	lines := []string{
		stylePaneTitle.Render("Key Bindings"),
		"",
		fmt.Sprintf("  %-16s %s", keys.Up.Help().Key, keys.Up.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Down.Help().Key, keys.Down.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.PgUp.Help().Key, keys.PgUp.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.PgDn.Help().Key, keys.PgDn.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Home.Help().Key, keys.Home.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.End.Help().Key, keys.End.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Enter.Help().Key, keys.Enter.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Back.Help().Key, keys.Back.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Open.Help().Key, keys.Open.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Sort.Help().Key, keys.Sort.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Refresh.Help().Key, keys.Refresh.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Help.Help().Key, keys.Help.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Quit.Help().Key, keys.Quit.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.CreateList.Help().Key, keys.CreateList.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.EditList.Help().Key, keys.EditList.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.DeleteList.Help().Key, keys.DeleteList.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.AddRepo.Help().Key, keys.AddRepo.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.RemoveRepo.Help().Key, keys.RemoveRepo.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.MoveRepo.Help().Key, keys.MoveRepo.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.UnstarRepo.Help().Key, keys.UnstarRepo.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.CopyList.Help().Key, keys.CopyList.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.MergeList.Help().Key, keys.MergeList.Help().Desc),
		fmt.Sprintf("  %-16s %s", keys.Preview.Help().Key, keys.Preview.Help().Desc),
		"",
		styleFaint.Render("Press ? to close help"),
	}
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
