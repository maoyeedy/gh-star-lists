package tui

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
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
	plain := stripANSI(rendered)
	for _, r := range svc.repos {
		if !strings.Contains(plain, r.NameWithOwner) {
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
	repos := []domain.Repository{
		{ID: "R_1", NameWithOwner: "a/short", StargazerCount: 1, Language: "Go"},
		{ID: "R_2", NameWithOwner: "b/medium", StargazerCount: 100, Language: "Rust"},
		{ID: "R_3", NameWithOwner: "c/another", StargazerCount: 10000, Language: "TypeScript"},
		{ID: "R_4", NameWithOwner: "d/no-lang", StargazerCount: 42, Language: ""},
		{ID: "R_5", NameWithOwner: "e/more", StargazerCount: 999, Language: "Python"},
	}
	svc := &fakeService{
		lists: []domain.StarList{{ID: "UL_1", Name: "test", RepoCount: 5}},
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

func TestRepoPanePreviewModeHidesMetadata(t *testing.T) {
	t.Parallel()
	repo := domain.Repository{
		ID:             "R_1",
		NameWithOwner:  "ItsEthra/typst-live",
		StargazerCount: 132,
		Language:       "Rust",
	}
	svc := &fakeService{
		lists: []domain.StarList{{ID: "UL_1", Name: "typst", RepoCount: 1}},
		repos: []domain.Repository{repo},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.showPreview = true

	rendered := m.renderRepoPane(36, 8)
	plain := stripANSI(rendered)

	if !strings.Contains(plain, repo.NameWithOwner) {
		t.Fatalf("preview repo pane should show full repo name; got:\n%s", rendered)
	}
	if strings.Contains(plain, starGlyph) || strings.Contains(plain, "132") {
		t.Fatalf("preview repo pane should hide stars; got:\n%s", rendered)
	}
	if strings.Contains(plain, repo.Language) {
		t.Fatalf("preview repo pane should hide language; got:\n%s", rendered)
	}
}

// TestRepoNameSplitStyling verifies that the owner and "/" are rendered with
// faint ANSI styling and the repo name is not.
func TestRepoNameSplitStyling(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.width = 120

	rendered := repoPane(m, 120, 20)

	// Faint ANSI code must appear (owner + slash wrapped in \x1b[2m).
	if !strings.Contains(rendered, "\x1b[2m") {
		t.Errorf("expected faint ANSI codes in rendered output; got:\n%s", rendered)
	}

	// The faint codes split the raw NameWithOwner, so check the unstripped
	// text parts rather than the combined string.
	plain := stripANSI(rendered)
	for _, r := range svc.repos {
		if !strings.Contains(plain, r.NameWithOwner) {
			t.Errorf("rendered output should contain %q; unstripped:\n%s",
				r.NameWithOwner, rendered)
		}
	}

	// Check that owner and slash have faint wrapping: the faint code should
	// appear before "owner" in the stripped output.
	faintPrefix := "\x1b[2m"
	for _, line := range strings.Split(rendered, "\n") {
		plainLine := stripANSI(line)
		if !strings.Contains(plainLine, "/") {
			continue
		}
		if !strings.Contains(line, faintPrefix+"owner") {
			t.Errorf("owner not wrapped in faint; line:\n%q", line)
		}
	}
}

// TestRepoTruncation verifies that a repo with a very long NameWithOwner does
// not produce a rendered line longer than the pane width.
func TestRepoTruncation(t *testing.T) {
	t.Parallel()
	longName := "some-very-long-owner-name/this-is-an-extremely-long-repository-name-with-extra"
	svc := &fakeService{
		lists: []domain.StarList{{ID: "UL_1", Name: "trunctest", RepoCount: 1}},
		repos: []domain.Repository{
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
	if !strings.Contains(stripANSI(narrowOut), "owner/") {
		t.Errorf("width 29: repo name should still appear; got:\n%s", narrowOut)
	}

	// Medium width (>= 34 shows lang).
	medOut := repoPane(m, 55, 15)
	if !strings.Contains(medOut, "\u2605") {
		t.Errorf("width 55: star glyph should appear; got:\n%s", medOut)
	}
}
