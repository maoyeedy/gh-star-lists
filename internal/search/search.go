package search

import (
	"sort"
	"strings"
	"unicode"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

// Field is a scored text field used as input to Score.
type Field struct {
	Text   string
	Weight int
}

// FilterRepositories returns repos matching query ranked by relevance
// (score desc, stars desc, name asc). Returns repos unchanged if query is empty.
func FilterRepositories(repos []githubapi.Repository, query string) []githubapi.Repository {
	query = strings.TrimSpace(query)
	if query == "" {
		return repos
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}
	phrase := normalize(query)

	var editPrev, editCurr []int
	tokenCache := make(map[string][]string)

	type repoMatch struct {
		repo  githubapi.Repository
		score int
	}
	matches := make([]repoMatch, 0, len(repos))
	for _, repo := range repos {
		score := scoreRepository(repo, terms, phrase, tokenCache, &editPrev, &editCurr)
		if score > 0 {
			matches = append(matches, repoMatch{repo: repo, score: score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if matches[i].repo.StargazerCount != matches[j].repo.StargazerCount {
			return matches[i].repo.StargazerCount > matches[j].repo.StargazerCount
		}
		return strings.Compare(
			strings.ToLower(matches[i].repo.NameWithOwner),
			strings.ToLower(matches[j].repo.NameWithOwner),
		) < 0
	})
	out := make([]githubapi.Repository, len(matches))
	for i, m := range matches {
		out[i] = m.repo
	}
	return out
}

// FilterStarLists returns star lists matching query ranked by relevance
// (score desc, name asc). Returns lists unchanged if query is empty.
func FilterStarLists(lists []githubapi.StarList, query string) []githubapi.StarList {
	query = strings.TrimSpace(query)
	if query == "" {
		return lists
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}
	phrase := normalize(query)

	var editPrev, editCurr []int
	tokenCache := make(map[string][]string)

	type listMatch struct {
		list  githubapi.StarList
		score int
	}
	matches := make([]listMatch, 0, len(lists))
	for _, list := range lists {
		score := scoreStarList(list, terms, phrase, tokenCache, &editPrev, &editCurr)
		if score > 0 {
			matches = append(matches, listMatch{list: list, score: score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return strings.Compare(
			strings.ToLower(matches[i].list.Name),
			strings.ToLower(matches[j].list.Name),
		) < 0
	})
	out := make([]githubapi.StarList, len(matches))
	for i, m := range matches {
		out[i] = m.list
	}
	return out
}

// Tokens returns the normalized searchable tokens extracted from text.
func Tokens(text string) []string { return searchTerms(text) }

func scorePrepared(
	prepared []preparedField,
	terms []string,
	phrase, allText string,
	editPrev, editCurr *[]int,
) int {
	if len(terms) == 0 {
		return 0
	}

	score := 0
	if phrase != "" && strings.Contains(allText, phrase) {
		score += 120
	}
	for _, term := range terms {
		best := 0
		for _, field := range prepared {
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

func scoreRepository(
	repo githubapi.Repository,
	terms []string,
	phrase string,
	tokenCache map[string][]string,
	editPrev, editCurr *[]int,
) int {
	owner, name, _ := strings.Cut(repo.NameWithOwner, "/")

	normName := normalize(name)
	normNameWithOwner := normalize(repo.NameWithOwner)
	normOwner := normalize(owner)
	normDesc := normalize(repo.Description)
	normLang := normalize(repo.Language)

	prepared := make([]preparedField, 0, 5)

	if normName != "" {
		prepared = append(
			prepared,
			preparedField{normName, cachedSearchTerms(tokenCache, normName), 120},
		)
	}
	if normNameWithOwner != "" {
		prepared = append(
			prepared,
			preparedField{normNameWithOwner, cachedSearchTerms(tokenCache, normNameWithOwner), 100},
		)
	}
	if normOwner != "" {
		prepared = append(
			prepared,
			preparedField{normOwner, cachedSearchTerms(tokenCache, normOwner), 80},
		)
	}
	if normDesc != "" {
		prepared = append(
			prepared,
			preparedField{normDesc, cachedSearchTerms(tokenCache, normDesc), 55},
		)
	}
	if normLang != "" {
		prepared = append(
			prepared,
			preparedField{normLang, cachedSearchTerms(tokenCache, normLang), 45},
		)
	}

	var sb strings.Builder
	sb.Grow(len(normNameWithOwner) + 1 + len(normDesc) + 1 + len(normLang))
	sb.WriteString(normNameWithOwner)
	sb.WriteByte(' ')
	sb.WriteString(normDesc)
	sb.WriteByte(' ')
	sb.WriteString(normLang)
	return scorePrepared(prepared, terms, phrase, sb.String(), editPrev, editCurr)
}

func scoreStarList(
	list githubapi.StarList,
	terms []string,
	phrase string,
	tokenCache map[string][]string,
	editPrev, editCurr *[]int,
) int {
	normName := normalize(list.Name)
	normDesc := normalize(list.Description)

	prepared := make([]preparedField, 0, 2)

	if normName != "" {
		prepared = append(
			prepared,
			preparedField{normName, cachedSearchTerms(tokenCache, normName), 120},
		)
	}
	if normDesc != "" {
		prepared = append(
			prepared,
			preparedField{normDesc, cachedSearchTerms(tokenCache, normDesc), 70},
		)
	}

	var sb strings.Builder
	sb.Grow(len(normName) + 1 + len(normDesc))
	sb.WriteString(normName)
	sb.WriteByte(' ')
	sb.WriteString(normDesc)
	return scorePrepared(prepared, terms, phrase, sb.String(), editPrev, editCurr)
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
		case equivalentToken(token, term):
			best = max(best, f.weight+80)
		case strings.HasPrefix(token, term):
			best = max(best, f.weight+65)
		case strings.Contains(token, term):
			best = max(best, f.weight+45)
		case isSubsequence(term, token):
			best = max(best, f.weight+20)
		}
		if distance, ok := boundedEditDistance(
			term, token, maxDistance(term), editPrev, editCurr,
		); ok {
			best = max(best, f.weight+35-(distance*10))
		}
	}
	if strings.Contains(f.text, term) {
		best = max(best, f.weight+50)
	}
	return best
}

func equivalentToken(left, right string) bool {
	if left == right {
		return true
	}
	return singularToken(left) == singularToken(right)
}

func singularToken(token string) string {
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
	text = normalize(text)
	if text == "" {
		return nil
	}
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// cachedSearchTerms returns tokens for text, reusing cache to avoid re-tokenizing.
// Callers must not mutate the returned slice.
func cachedSearchTerms(cache map[string][]string, text string) []string {
	if tokens, ok := cache[text]; ok {
		return tokens
	}
	tokens := searchTerms(text)
	cache[text] = tokens
	return tokens
}

func normalize(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func maxDistance(term string) int {
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
