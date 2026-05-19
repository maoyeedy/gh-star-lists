package command

import (
	"sort"
	"strings"
	"unicode"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type repositorySearchMatch struct {
	repo  githubapi.Repository
	score int
}

func searchRepositories(repos []githubapi.Repository, query string) []githubapi.Repository {
	query = strings.TrimSpace(query)
	if query == "" {
		return repos
	}

	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}
	phrase := normalizeSearchText(query)

	var editPrev, editCurr []int
	tokenCache := make(map[string][]string)

	matches := make([]repositorySearchMatch, 0, len(repos))
	for _, repo := range repos {
		score := repositorySearchScore(repo, terms, phrase, tokenCache, &editPrev, &editCurr)
		if score > 0 {
			matches = append(matches, repositorySearchMatch{repo: repo, score: score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if matches[i].repo.StargazerCount != matches[j].repo.StargazerCount {
			return matches[i].repo.StargazerCount > matches[j].repo.StargazerCount
		}
		cmp, _ := compareRepositories(
			matches[i].repo,
			matches[j].repo,
			[]string{SortKeyName},
			nil,
			false,
		)
		return cmp < 0
	})

	out := make([]githubapi.Repository, 0, len(matches))
	for _, match := range matches {
		out = append(out, match.repo)
	}
	return out
}

func repositorySearchScore(
	repo githubapi.Repository,
	terms []string,
	phrase string,
	tokenCache map[string][]string,
	editPrev, editCurr *[]int,
) int {
	owner, name, _ := strings.Cut(repo.NameWithOwner, "/")
	normNameWithOwner := normalizeSearchText(repo.NameWithOwner)
	normDescription := normalizeSearchText(repo.Description)
	normLanguage := normalizeSearchText(repo.Language)
	rawFields := []searchField{
		{text: name, weight: 120},
		{text: repo.NameWithOwner, weight: 100},
		{text: owner, weight: 80},
		{text: repo.Description, weight: 55},
		{text: repo.Language, weight: 45},
	}
	fields := make([]preparedField, 0, len(rawFields))
	for _, f := range rawFields {
		text := normalizeSearchText(f.text)
		if text == "" {
			continue
		}
		fields = append(fields, preparedField{
			text:   text,
			tokens: cachedSearchTerms(tokenCache, text),
			weight: f.weight,
		})
	}

	score := 0
	allText := strings.Join([]string{normNameWithOwner, normDescription, normLanguage}, " ")
	if phrase != "" && strings.Contains(allText, phrase) {
		score += 120
	}

	for _, term := range terms {
		best := 0
		for _, field := range fields {
			if s := field.scoreTerm(term, editPrev, editCurr); s > best {
				best = s
			}
		}
		if best == 0 {
			return 0
		}
		score += best
	}
	return score
}

type searchField struct {
	text   string
	weight int
}

type preparedField struct {
	text   string
	tokens []string
	weight int
}

func (f preparedField) scoreTerm(term string, editPrev, editCurr *[]int) int {
	if f.text == term {
		return f.weight + 100
	}

	best := 0
	for _, token := range f.tokens {
		switch {
		case equivalentSearchToken(token, term):
			best = max(best, f.weight+80)
		case strings.HasPrefix(token, term):
			best = max(best, f.weight+65)
		case strings.Contains(token, term):
			best = max(best, f.weight+45)
		case isSubsequence(term, token):
			best = max(best, f.weight+20)
		}

		if distance, ok := boundedEditDistance(
			term,
			token,
			maxSearchDistance(term),
			editPrev,
			editCurr,
		); ok {
			best = max(best, f.weight+35-(distance*10))
		}
	}

	if strings.Contains(f.text, term) {
		best = max(best, f.weight+50)
	}
	return best
}

func equivalentSearchToken(left, right string) bool {
	if left == right {
		return true
	}
	return singularSearchToken(left) == singularSearchToken(right)
}

func singularSearchToken(token string) string {
	switch {
	case strings.HasSuffix(token, "ies") && len(token) > 3:
		return strings.TrimSuffix(token, "ies") + "y"
	case strings.HasSuffix(token, "s") && len(token) > 3:
		return strings.TrimSuffix(token, "s")
	default:
		return token
	}
}

func searchTerms(text string) []string {
	text = normalizeSearchText(text)
	if text == "" {
		return nil
	}
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// cachedSearchTerms returns tokens for text, using cache to avoid re-tokenizing identical strings.
// Callers must not mutate the returned slice.
func cachedSearchTerms(cache map[string][]string, text string) []string {
	if tokens, ok := cache[text]; ok {
		return tokens
	}
	tokens := searchTerms(text)
	cache[text] = tokens
	return tokens
}

func normalizeSearchText(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func maxSearchDistance(term string) int {
	switch n := len(term); {
	case n <= 7:
		return 1
	case n <= 10:
		return 2
	default:
		return 3
	}
}

func growIntSlice(s *[]int, need int) {
	if cap(*s) < need {
		*s = make([]int, need)
	} else {
		*s = (*s)[:need]
	}
}

func boundedEditDistance(a, b string, limit int, prevBuf, currBuf *[]int) (int, bool) {
	if a == "" || b == "" {
		return 0, false
	}
	diff := len(a) - len(b)
	if diff < 0 {
		diff = -diff
	}
	if diff > limit {
		return 0, false
	}

	n := len(b) + 1
	growIntSlice(prevBuf, n)
	growIntSlice(currBuf, n)
	prev := *prevBuf
	curr := *currBuf

	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min(
				prev[j]+1,
				min(curr[j-1]+1, prev[j-1]+cost),
			)
			rowMin = min(rowMin, curr[j])
		}
		if rowMin > limit {
			return 0, false
		}
		prev, curr = curr, prev
	}
	distance := prev[len(b)]
	return distance, distance <= limit
}

func isSubsequence(needle, haystack string) bool {
	if needle == "" {
		return true
	}
	j := 0
	for i := 0; i < len(haystack) && j < len(needle); i++ {
		if haystack[i] == needle[j] {
			j++
		}
	}
	return j == len(needle)
}
