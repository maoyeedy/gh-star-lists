package tui

import (
	"testing"
)

func TestPaneGeometryTwoColumn(t *testing.T) {
	t.Parallel()
	widths := []int{80, 100, 120, 160}
	for _, w := range widths {
		g := calcPaneGeometry(w)
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
	}
}
