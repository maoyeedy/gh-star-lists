package tui

// paneGeometry holds the computed column widths and separator positions for
// the current terminal width.
type paneGeometry struct {
	leftWidth int // list pane width (columns)
	repoWidth int // repo pane width (columns)
	sep1Col   int // column index of separator between list and repo panes
}

// calcPaneGeometry returns the two-column pane geometry for the given terminal width.
//
//   - totalWidth > 100: leftW = 30% (min 28), repoW = remainder
//   - totalWidth <= 100: leftW = 36 (fixed), repoW = remainder (min 10)
func calcPaneGeometry(totalWidth int) paneGeometry {
	const sepW = 1

	leftW := 36
	if totalWidth > 100 {
		leftW = totalWidth * 30 / 100
		if leftW < 28 {
			leftW = 28
		}
	}
	rightW := totalWidth - leftW - sepW
	if rightW < 10 {
		rightW = 10
	}
	return paneGeometry{
		leftWidth: leftW,
		repoWidth: rightW,
		sep1Col:   leftW,
	}
}
