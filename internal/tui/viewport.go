package tui

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

const headingRows = 2

// repoPaneH returns the effective number of scrollable repo rows in the repo
// pane (full pane content height; no heading overhead).
func (m model) repoPaneH() int {
	return max(1, m.height-headingRows)
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
