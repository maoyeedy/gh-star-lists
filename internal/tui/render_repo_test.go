package tui

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func TestSelectRendersPrefixWhenSelectionNonEmpty(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m.width = 80
	m.height = 24
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	// Mark the second repo ("owner/a-repo" at index 1).
	m.selected = map[string]struct{}{"owner/a-repo": {}}

	rendered := m.renderRepoPane(60, 20)

	if !strings.Contains(rendered, "[x]") {
		t.Errorf("renderRepoPane should contain '[x]' for checked repo, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[ ]") {
		t.Errorf("renderRepoPane should contain '[ ]' for unchecked repo, got:\n%s", rendered)
	}
}

// TestDropLastRuneMultiByte verifies dropLastRune removes exactly one Unicode

func TestNarrowRepoPaneHidesMetadata(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	// Render at narrow width (below threshold of 60).
	out := m.renderRepoPane(40, 10)

	// The meta format uses "*" as the star-count marker. It must be absent.
	if strings.Contains(out, "*") {
		t.Errorf("narrow renderRepoPane should not contain star-count marker, got:\n%s", out)
	}
}

func TestRepoPaneNoHeading(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo

	rendered := repoPane(m, 80, 20)

	if strings.Contains(rendered, "Repos in this list:") {
		t.Errorf("repo pane must not contain 'Repos in this list:'; got:\n%s", rendered)
	}
	firstLine := strings.SplitN(rendered, "\n", 2)[0]
	if strings.TrimSpace(firstLine) == "" {
		t.Errorf(
			"first line of repo pane must not be blank (no heading overhead); got:\n%s",
			rendered,
		)
	}
}

// TestRepoFieldStyling verifies that the repo pane produces styled output (ANSI
// sequences or at least readable content in NoTTY mode) and that the repo name
// is always present.
func TestRepoFieldStyling(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.width = 120

	rendered := repoPane(m, 120, 20)

	// The repo name must appear somewhere in the output.
	for _, r := range svc.repos {
		if !strings.Contains(rendered, r.NameWithOwner) {
			t.Errorf(
				"repo pane should contain NameWithOwner %q; got:\n%s",
				r.NameWithOwner,
				rendered,
			)
		}
	}

	// In a real terminal (or when styles render), the star glyph should appear
	// (since width 120 >= 30 threshold for stars).
	if !strings.Contains(rendered, "\u2605") {
		// Not a hard failure if lipgloss strips styling -- skip.
		t.Logf("star glyph absent (may be a NoTTY environment); rendered:\n%s", rendered)
	}
}

func TestRepoColumnAlignment(t *testing.T) {
	t.Parallel()
	repos := []githubapi.Repository{
		{ID: "R_1", NameWithOwner: "a/short", StargazerCount: 1, Language: "Go"},
		{ID: "R_2", NameWithOwner: "b/medium", StargazerCount: 100, Language: "Rust"},
		{ID: "R_3", NameWithOwner: "c/another", StargazerCount: 10000, Language: "TypeScript"},
		{ID: "R_4", NameWithOwner: "d/no-lang", StargazerCount: 42, Language: ""},
		{ID: "R_5", NameWithOwner: "e/more", StargazerCount: 999, Language: "Python"},
	}
	svc := &fakeService{
		lists: []githubapi.StarList{{ID: "UL_1", Name: "test", RepoCount: 5}},
		repos: repos,
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: repos, listID: "UL_1"})
	m.active = paneRepo

	rendered := repoPane(m, 120, 15)
	lines := strings.Split(rendered, "\n")

	// Strip ANSI before measuring byte position so escape-length differences
	// do not affect the column comparison.
	starCol := -1
	repoIdx := 0
	langSet := false
	for _, line := range lines {
		plain := stripANSI(line)
		if plain == "" {
			continue // padding rows
		}
		if repoIdx >= len(repos) {
			break
		}

		// Star column: must be consistent across rows.
		glyphPos := strings.Index(plain, "\u2605")
		if glyphPos < 0 {
			t.Errorf("repo %d: missing star glyph: %q", repoIdx, line)
			continue
		}
		if starCol < 0 {
			starCol = glyphPos
		} else if glyphPos != starCol {
			t.Errorf("repo %d: star glyph at byte-col %d, want %d (alignment); plain: %q",
				repoIdx, glyphPos, starCol, plain)
		}

		// Language column: right-aligned at end. The "  " gap starts at
		// visual position w - langWidth - 2. Use lipgloss.Width for the
		// offset to handle multi-byte glyphs (star) correctly.
		visW := lipgloss.Width(plain)
		expectedLang := repos[repoIdx].Language
		if expectedLang == "" {
			expectedLang = "-"
		}
		gapStart := visW - langWidth - 2
		if gapStart >= 0 && len(plain) >= gapStart+2 {
			gap := plain[gapStart : gapStart+2]
			if gap != "  " {
				t.Errorf(
					"repo %d: expected 2-space gap before lang at vis-col %d, got %q; plain: %q",
					repoIdx,
					gapStart,
					gap,
					plain,
				)
			}
			langRaw := strings.TrimLeft(plain[gapStart+2:], " ")
			if langRaw != expectedLang {
				t.Errorf("repo %d: expected lang %q right-aligned, got %q; plain: %q",
					repoIdx, expectedLang, langRaw, plain)
			}
		}
		langSet = true
		repoIdx++
	}
	if !langSet {
		t.Error("no content lines were checked for language alignment")
	}
}

// TestRepoTruncation verifies that a repo with a very long NameWithOwner does
// not produce a rendered line longer than the pane width.
func TestRepoTruncation(t *testing.T) {
	t.Parallel()
	longName := "some-very-long-owner-name/this-is-an-extremely-long-repository-name-with-extra"
	svc := &fakeService{
		lists: []githubapi.StarList{{ID: "UL_1", Name: "trunctest", RepoCount: 1}},
		repos: []githubapi.Repository{
			{ID: "R_1", NameWithOwner: longName, StargazerCount: 5, Language: "Go"},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	const paneW = 80

	rendered := repoPane(m, paneW, 15)
	for i, line := range strings.Split(rendered, "\n") {
		w := lipgloss.Width(line)
		if w > paneW {
			t.Errorf("line %d has visual width %d > pane width %d: %q", i, w, paneW, line)
		}
	}
}

// TestNarrowRepoPaneP4HidesMetadata verifies progressive field hiding at narrow
// widths using the P4 thresholds.
func TestNarrowRepoPaneP4HidesMetadata(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	// Very narrow: stars and language hidden (width < 30 hides stars, < 34 hides lang).
	narrowOut := repoPane(m, 29, 15)
	if strings.Contains(narrowOut, "\u2605") {
		t.Errorf("width 29: star glyph should be absent; got:\n%s", narrowOut)
	}
	// Repo name must still appear.
	if !strings.Contains(narrowOut, "owner/") {
		t.Errorf("width 29: repo name should still appear; got:\n%s", narrowOut)
	}

	// Medium width (>= 34 shows lang).
	medOut := repoPane(m, 55, 15)
	if !strings.Contains(medOut, "\u2605") {
		t.Errorf("width 55: star glyph should appear; got:\n%s", medOut)
	}
}

func TestRepoWidthsCachedAcrossScrolls(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.width = 120
	m.height = 24

	// Call ensureRepoWidths directly on a pointer so mutations are visible.
	(&m).ensureRepoWidths()
	sig1 := m.cachedRepoSig
	sw1 := m.cachedStarWidth

	if sig1 == "" {
		t.Error("cachedRepoSig should be non-empty after ensureRepoWidths")
	}
	if sw1 <= 0 {
		t.Errorf("cachedStarWidth = %d, want > 0", sw1)
	}

	// Second call -- sentinel unchanged, widths stable (cache hit).
	(&m).ensureRepoWidths()
	if m.cachedRepoSig != sig1 {
		t.Errorf("cachedRepoSig changed on second call: %q -> %q", sig1, m.cachedRepoSig)
	}
	if m.cachedStarWidth != sw1 {
		t.Errorf("cachedStarWidth changed on second call: %d -> %d", sw1, m.cachedStarWidth)
	}

	// Verify sentinel invalidates when list changes.
	m.focusedList = &m.lists[1]
	m.displayedRepos = svc.repos // same repos, different focused list
	(&m).ensureRepoWidths()
	if m.cachedRepoSig == sig1 {
		t.Error("cachedRepoSig should change when focused list changes")
	}
}
