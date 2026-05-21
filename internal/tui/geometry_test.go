package tui

import (
	"testing"
)

func TestPaneGeometryTwoColumn(t *testing.T) {
	t.Parallel()
	widths := []int{80, 100, 120, 160}
	for _, w := range widths {
		g := calcPaneGeometry(w, false)
		// Separator occupies exactly 1 column: leftWidth + sep(1) + repoWidth == totalWidth.
		if got := g.leftWidth + 1 + g.repoWidth; got != w {
			t.Errorf(
				"width=%d: leftWidth(%d)+1+repoWidth(%d)=%d, want %d",
				w,
				g.leftWidth,
				g.repoWidth,
				got,
				w,
			)
		}
		// sep1Col is at leftWidth.
		if g.sep1Col != g.leftWidth {
			t.Errorf("width=%d: sep1Col=%d, want %d", w, g.sep1Col, g.leftWidth)
		}
		// No preview in 2-col mode.
		if g.previewWidth != 0 {
			t.Errorf("width=%d: previewWidth=%d, want 0", w, g.previewWidth)
		}
		// sep2Col must be -1 in 2-col mode.
		if g.sep2Col != -1 {
			t.Errorf("width=%d: sep2Col=%d, want -1", w, g.sep2Col)
		}
	}
}

// TestPaneGeometryThreeColumn verifies three-column layout invariants across a
// range of terminal widths.
func TestPaneGeometryThreeColumn(t *testing.T) {
	t.Parallel()
	widths := []int{100, 120, 160, 200}
	for _, w := range widths {
		g := calcPaneGeometry(w, true)
		// Two separators: leftWidth + sep(1) + repoWidth + sep(1) + previewWidth == totalWidth.
		total := g.leftWidth + 1 + g.repoWidth + 1 + g.previewWidth
		if total != w {
			t.Errorf("width=%d: leftWidth(%d)+1+repoWidth(%d)+1+previewWidth(%d)=%d, want %d",
				w, g.leftWidth, g.repoWidth, g.previewWidth, total, w)
		}
		// sep1Col is at leftWidth.
		if g.sep1Col != g.leftWidth {
			t.Errorf("width=%d: sep1Col=%d, want %d (leftWidth)", w, g.sep1Col, g.leftWidth)
		}
		// sep2Col is at leftWidth + 1 + repoWidth.
		wantSep2 := g.leftWidth + 1 + g.repoWidth
		if g.sep2Col != wantSep2 {
			t.Errorf("width=%d: sep2Col=%d, want %d", w, g.sep2Col, wantSep2)
		}
		// preview pane must have positive width.
		if g.previewWidth <= 0 {
			t.Errorf("width=%d: previewWidth=%d, want >0", w, g.previewWidth)
		}
	}
}
