package search

import (
	"reflect"
	"strings"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func makeRepo(nameWithOwner, description, language string, stars int) githubapi.Repository {
	return githubapi.Repository{
		ID:             nameWithOwner,
		NameWithOwner:  nameWithOwner,
		Description:    description,
		Language:       language,
		StargazerCount: stars,
		URL:            "https://github.com/" + nameWithOwner,
	}
}

func makeList(name, description string) githubapi.StarList {
	return githubapi.StarList{
		ID:          name,
		Name:        name,
		Description: description,
		URL:         "https://github.com/stars/user/lists/" + name,
	}
}

func repoNames(repos []githubapi.Repository) []string {
	out := make([]string, len(repos))
	for i, r := range repos {
		out[i] = r.NameWithOwner
	}
	return out
}

func listNames(lists []githubapi.StarList) []string {
	out := make([]string, len(lists))
	for i, l := range lists {
		out[i] = l.Name
	}
	return out
}

func TestFilterRepositories(t *testing.T) {
	t.Parallel()

	gin := makeRepo("gin-gonic/gin", "HTTP web framework written in Go", "Go", 70000)
	echo := makeRepo("labstack/echo", "High performance HTTP framework", "Go", 28000)
	react := makeRepo(
		"facebook/react",
		"A declarative library for building user interfaces",
		"JavaScript",
		220000,
	)
	rails := makeRepo("rails/rails", "Ruby on Rails web framework", "Ruby", 55000)
	all := []githubapi.Repository{gin, echo, react, rails}

	tests := []struct {
		name  string
		query string
		repos []githubapi.Repository
		want  []string
	}{
		{
			name:  "empty query returns all repos unchanged",
			query: "",
			repos: all,
			want:  []string{"gin-gonic/gin", "labstack/echo", "facebook/react", "rails/rails"},
		},
		{
			name:  "whitespace only returns all",
			query: "   \t  ",
			repos: all,
			want:  []string{"gin-gonic/gin", "labstack/echo", "facebook/react", "rails/rails"},
		},
		{
			name:  "no matches returns empty",
			query: "kubernetes",
			repos: all,
			want:  []string{},
		},
		{
			name:  "name match ranks gin first",
			query: "gin",
			repos: all,
			want:  []string{"gin-gonic/gin"},
		},
		{
			name:  "multi-term AND semantics filter out partial matches",
			query: "web framework",
			repos: all,
			want:  []string{"gin-gonic/gin", "rails/rails"},
		},
		{
			name:  "plural folding matches library/libraries",
			query: "library",
			repos: []githubapi.Repository{
				makeRepo("a/lib", "Reusable libraries for parsing", "Go", 100),
			},
			want: []string{"a/lib"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterRepositories(tt.repos, tt.query)
			names := repoNames(got)
			if !reflect.DeepEqual(names, tt.want) {
				t.Fatalf("FilterRepositories(%q) = %v; want %v", tt.query, names, tt.want)
			}
		})
	}
}

func TestFilterStarLists(t *testing.T) {
	t.Parallel()

	tools := makeList("tools", "CLI tools and utilities")
	infra := makeList("infra", "Infrastructure and DevOps")
	webdev := makeList("webdev", "Web development frameworks and libraries")
	all := []githubapi.StarList{tools, infra, webdev}

	tests := []struct {
		name  string
		query string
		lists []githubapi.StarList
		want  []string
	}{
		{
			name:  "empty query returns all unchanged",
			query: "",
			lists: all,
			want:  []string{"tools", "infra", "webdev"},
		},
		{
			name:  "no match returns empty",
			query: "kubernetes",
			lists: all,
			want:  []string{},
		},
		{
			name:  "name match",
			query: "tools",
			lists: all,
			want:  []string{"tools"},
		},
		{
			name:  "description match",
			query: "devops",
			lists: all,
			want:  []string{"infra"},
		},
		{
			name:  "multi-term AND",
			query: "web frameworks",
			lists: all,
			want:  []string{"webdev"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterStarLists(tt.lists, tt.query)
			names := listNames(got)
			if !reflect.DeepEqual(names, tt.want) {
				t.Fatalf("FilterStarLists(%q) = %v; want %v", tt.query, names, tt.want)
			}
		})
	}
}

func TestFilterRepositoriesOrdering(t *testing.T) {
	t.Parallel()

	exact := makeRepo("foo/go", "static site generator", "Rust", 100)
	prefix := makeRepo("foo/gopher", "tools", "Go", 200)
	descOnly := makeRepo("foo/zzz", "written in go", "Rust", 50)

	got := FilterRepositories([]githubapi.Repository{descOnly, prefix, exact}, "go")
	if len(got) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(got))
	}
	if got[0].NameWithOwner != "foo/go" {
		t.Errorf("first should be exact name match foo/go, got %s", got[0].NameWithOwner)
	}
	if got[2].NameWithOwner != "foo/zzz" {
		t.Errorf("last should be description-only foo/zzz, got %s", got[2].NameWithOwner)
	}
}

func TestFilterRepositoriesStarsTiebreaker(t *testing.T) {
	t.Parallel()

	low := makeRepo("foo/echo", "x", "Go", 100)
	high := makeRepo("bar/echo", "x", "Go", 5000)

	got := FilterRepositories([]githubapi.Repository{low, high}, "echo")
	if got[0].NameWithOwner != "bar/echo" {
		t.Errorf("higher stars should rank first, got %s", got[0].NameWithOwner)
	}
}

func TestRepositorySearchScore(t *testing.T) {
	t.Parallel()

	base := makeRepo("acme/widget", "A widget framework for things", "Go", 100)

	tests := []struct {
		name     string
		repo     githubapi.Repository
		terms    []string
		phrase   string
		wantZero bool
	}{
		{
			name:     "term not present returns zero (AND semantics)",
			repo:     base,
			terms:    []string{"unrelated"},
			phrase:   "unrelated",
			wantZero: true,
		},
		{
			name:     "empty terms returns zero",
			repo:     base,
			terms:    nil,
			phrase:   "",
			wantZero: true,
		},
		{
			name:   "name match scores nonzero",
			repo:   base,
			terms:  []string{"widget"},
			phrase: "widget",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var editPrev, editCurr []int
			got := scoreRepository(
				tt.repo,
				tt.terms,
				tt.phrase,
				&editPrev,
				&editCurr,
			)
			if tt.wantZero && got != 0 {
				t.Fatalf("want 0, got %d", got)
			}
			if !tt.wantZero && got == 0 {
				t.Fatalf("want nonzero score, got 0")
			}
		})
	}
}

func TestRepositorySearchScoreFieldWeights(t *testing.T) {
	t.Parallel()

	nameRepo := makeRepo("foo/alpha-tools", "neutral text here", "neutral", 0)
	ownerRepo := makeRepo("alpha-corp/bar", "neutral text here", "neutral", 0)
	descRepo := makeRepo("foo/bar", "alpha is mentioned here", "neutral", 0)
	langRepo := makeRepo("foo/bar", "neutral text here", "alpha lang", 0)

	terms := []string{"alpha"}
	phrase := "alpha"

	var editPrev, editCurr []int
	nameScore := scoreRepository(nameRepo, terms, phrase, &editPrev, &editCurr)
	ownerScore := scoreRepository(ownerRepo, terms, phrase, &editPrev, &editCurr)
	descScore := scoreRepository(descRepo, terms, phrase, &editPrev, &editCurr)
	langScore := scoreRepository(langRepo, terms, phrase, &editPrev, &editCurr)

	if nameScore <= ownerScore {
		t.Errorf("name match should outscore owner match: name=%d owner=%d", nameScore, ownerScore)
	}
	if ownerScore <= descScore {
		t.Errorf("owner match should outscore description: owner=%d desc=%d", ownerScore, descScore)
	}
	if descScore <= langScore {
		t.Errorf("description should outscore language: desc=%d lang=%d", descScore, langScore)
	}
}

func TestBoundedEditDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a, b     string
		limit    int
		wantDist int
		wantOK   bool
	}{
		{
			name: "identical strings distance zero",
			a:    "kube", b: "kube", limit: 2, wantDist: 0, wantOK: true,
		},
		{name: "single substitution", a: "kube", b: "kubo", limit: 2, wantDist: 1, wantOK: true},
		{name: "single deletion", a: "kube", b: "kub", limit: 2, wantDist: 1, wantOK: true},
		{name: "over limit returns false", a: "abcdef", b: "zzzzzz", limit: 2, wantOK: false},
		{name: "empty a returns false", a: "", b: "abc", limit: 2, wantOK: false},
		{name: "empty b returns false", a: "abc", b: "", limit: 2, wantOK: false},
		{
			name: "length diff over limit short circuits",
			a:    "a", b: "abcdef", limit: 2, wantOK: false,
		},
		{
			name: "row min pruning long mismatched",
			a:    "aaaaaaaaaa", b: "zzzzzzzzzz", limit: 2, wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var editPrev, editCurr []int
			dist, ok := boundedEditDistance(tt.a, tt.b, tt.limit, &editPrev, &editCurr)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (dist=%d)", ok, tt.wantOK, dist)
			}
			if tt.wantOK && dist != tt.wantDist {
				t.Fatalf("dist = %d, want %d", dist, tt.wantDist)
			}
		})
	}
}

func TestSingularToken(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"queries":   "query",
		"libraries": "library",
		"libs":      "lib",
		"cats":      "cat",
		"s":         "s",
		"is":        "is",
		"ies":       "ies",
		"go":        "go",
	}

	for input, want := range cases {
		input, want := input, want
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if got := singularToken(input); got != want {
				t.Errorf("singularToken(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestEquivalentToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want bool
	}{
		{"go", "go", true},
		{"libs", "lib", true},
		{"libraries", "library", true},
		{"go", "rust", false},
		{"queries", "queries", true},
		{"car", "bar", false},
	}

	for _, c := range cases {
		c := c
		t.Run(c.a+"_"+c.b, func(t *testing.T) {
			t.Parallel()
			if got := equivalentToken(c.a, c.b); got != c.want {
				t.Errorf("equivalentToken(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestTokens(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"  HELLO,  WORLD!  ", []string{"hello", "world"}},
		{"foo-bar_baz/qux", []string{"foo", "bar", "baz", "qux"}},
		{"", nil},
		{"   ", nil},
	}

	for _, c := range cases {
		c := c
		t.Run(c.input, func(t *testing.T) {
			t.Parallel()
			got := Tokens(c.input)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Tokens(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	if got := normalize("  Hello World  "); got != "hello world" {
		t.Errorf("normalize trims and lowers, got %q", got)
	}
	if got := normalize(""); got != "" {
		t.Errorf("normalize empty, got %q", got)
	}
}

func TestIsSubsequence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		needle, haystack string
		want             bool
	}{
		{"", "abc", true},
		{"abc", "abc", true},
		{"ac", "abc", true},
		{"abc", "ac", false},
		{"xyz", "abcxyz", true},
		{"xyz", "abc", false},
	}

	for _, c := range cases {
		c := c
		t.Run(c.needle+"_in_"+c.haystack, func(t *testing.T) {
			t.Parallel()
			if got := isSubsequence(c.needle, c.haystack); got != c.want {
				t.Errorf("isSubsequence(%q, %q) = %v, want %v", c.needle, c.haystack, got, c.want)
			}
		})
	}
}

func TestWalkTokens(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"foo-bar_baz", []string{"foo", "bar", "baz"}},
		{"caf\u00e9 r\u00e9sum\u00e9", []string{"caf\u00e9", "r\u00e9sum\u00e9"}},
		{"hello\U0001F44Bworld", []string{"hello", "world"}},
		{"", nil},
		{"---", nil},
	}

	for _, c := range cases {
		c := c
		t.Run(c.input, func(t *testing.T) {
			t.Parallel()
			var got []string
			walkTokens(c.input, func(start, end int) bool {
				got = append(got, c.input[start:end])
				return true
			})
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("walkTokens(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestPhraseBonusWithinSingleField(t *testing.T) {
	t.Parallel()

	// Description contains the exact phrase -- phrase bonus (+120) should apply.
	withPhrase := makeRepo("foo/bar", "Go web framework for building APIs", "Go", 100)
	// Description has both terms but not adjacent -- no phrase bonus.
	withoutPhrase := makeRepo(
		"foo/baz",
		"web things and also a framework for building stuff",
		"Go",
		100,
	)

	terms := []string{"web", "framework"}
	phrase := "web framework"
	var ep1, ec1, ep2, ec2 []int
	scoreWith := scoreRepository(withPhrase, terms, phrase, &ep1, &ec1)
	scoreWithout := scoreRepository(withoutPhrase, terms, phrase, &ep2, &ec2)

	if scoreWith == 0 {
		t.Fatal("expected nonzero score for phrase match in description")
	}
	if scoreWith <= scoreWithout {
		t.Errorf("phrase match (%d) should outscore non-phrase match (%d)", scoreWith, scoreWithout)
	}
}

func TestMaxDistance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		term string
		want int
	}{
		{strings.Repeat("a", 1), 1},
		{strings.Repeat("a", 7), 1},
		{strings.Repeat("a", 8), 2},
		{strings.Repeat("a", 10), 2},
		{strings.Repeat("a", 11), 3},
		{strings.Repeat("a", 50), 3},
	}

	for _, c := range cases {
		c := c
		t.Run(c.term, func(t *testing.T) {
			t.Parallel()
			if got := maxDistance(c.term); got != c.want {
				t.Errorf("maxDistance(len=%d) = %d, want %d", len(c.term), got, c.want)
			}
		})
	}
}
