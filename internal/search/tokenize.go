package search

import (
	"strings"
	"unicode"
)

// Tokens returns the normalized searchable tokens extracted from text.
func Tokens(text string) []string { return searchTerms(text) }

func searchTerms(text string) []string {
	text = normalize(text)
	if text == "" {
		return nil
	}
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func normalize(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

// walkTokens invokes fn(start, end) for each maximal run of letter/number runes in text.
// fn returns false to stop early.
func walkTokens(text string, fn func(start, end int) bool) {
	start, inTok := -1, false
	for i, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if !inTok {
				start = i
				inTok = true
			}
		} else if inTok {
			if !fn(start, i) {
				return
			}
			inTok = false
		}
	}
	if inTok {
		fn(start, len(text))
	}
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
