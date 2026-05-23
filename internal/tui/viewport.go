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
