package tui

// paneGeometry holds the computed column widths and separator positions for
// the current terminal width.
type paneGeometry struct {
	leftWidth  int  // list pane width (columns)
	repoWidth  int  // repo pane width (columns)
	sep1Col    int  // column index of separator between list and repo panes
	singlePane bool // true when only the active pane should be rendered
}

// calcPaneGeometry returns the two-column pane geometry for the given terminal width.
func calcPaneGeometry(totalWidth int) paneGeometry {
	const (
		listPaneW           = 24
		sepW                = 1
		singlePaneThreshold = 72
	)

	leftW := listPaneW
	rightW := totalWidth - listPaneW - sepW
	if rightW < 10 {
		rightW = 10
	}
	return paneGeometry{
		leftWidth:  leftW,
		repoWidth:  rightW,
		sep1Col:    leftW,
		singlePane: totalWidth < singlePaneThreshold,
	}
}
