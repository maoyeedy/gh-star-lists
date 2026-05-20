package tui

// paneGeometry holds the computed column widths and separator positions for
// the current terminal width and preview visibility.
type paneGeometry struct {
	leftWidth    int // list pane width (columns)
	repoWidth    int // repo pane width (columns)
	previewWidth int // preview pane width (0 when showPreview=false)
	sep1Col      int // column index of separator between list and repo panes
	sep2Col      int // column index of separator between repo and preview panes; -1 when showPreview=false
}

// calcPaneGeometry returns the pane geometry for the given terminal width and
// preview visibility. It encodes the same thresholds that were previously
// inlined in renderLayout and handleMouseClick.
//
// Three-column (showPreview=true):
//   - totalWidth > 120: leftW = 22% (min 20), midW = 28% (min 24), previewW = remainder
//   - totalWidth <= 120: leftW = 28 (fixed), midW = 36 (fixed), previewW = remainder (min 20)
//
// Two-column (showPreview=false):
//   - totalWidth > 100: leftW = 30% (min 28), repoW = remainder
//   - totalWidth <= 100: leftW = 36 (fixed), repoW = remainder (min 10)
func calcPaneGeometry(totalWidth int, showPreview bool) paneGeometry {
	const sepW = 1
	if showPreview {
		leftW := 28
		midW := 36
		if totalWidth > 120 {
			leftW = totalWidth * 22 / 100
			if leftW < 20 {
				leftW = 20
			}
			midW = totalWidth * 28 / 100
			if midW < 24 {
				midW = 24
			}
		}
		previewW := totalWidth - leftW - midW - 2*sepW
		if previewW < 20 {
			previewW = 20
		}
		return paneGeometry{
			leftWidth:    leftW,
			repoWidth:    midW,
			previewWidth: previewW,
			sep1Col:      leftW,
			sep2Col:      leftW + sepW + midW,
		}
	}

	// Two-column layout.
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
		leftWidth:    leftW,
		repoWidth:    rightW,
		previewWidth: 0,
		sep1Col:      leftW,
		sep2Col:      -1,
	}
}
