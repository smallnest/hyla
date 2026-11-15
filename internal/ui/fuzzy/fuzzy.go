// Package fuzzy scores how well a query matches a text, as the palette
// and the picker find what the user types.
package fuzzy

import (
	"strings"
	"unicode"
)

// Score scores how well query matches s, as its letters in order: higher
// is better, -1 no match. Starts of words and runs score more.
func Score(query, s string) int {
	if query == "" {
		return 0
	}
	q := []rune(strings.ToLower(query))
	r := []rune(strings.ToLower(s))
	score, qi, run := 0, 0, 0
	for i := 0; i < len(r) && qi < len(q); i++ {
		if q[qi] == ' ' {
			qi++
			continue
		}
		if r[i] == q[qi] {
			bonus := 1
			if i == 0 || !unicode.IsLetter(r[i-1]) && !unicode.IsDigit(r[i-1]) {
				bonus += 6
			}
			run++
			score += bonus + run*2
			qi++
		} else {
			run = 0
		}
	}
	if qi < len(q) {
		return -1
	}
	return score*10 - len(r)
}

// Words scores how well query finds words, a list of them: each of its
// terms the start of one. Too many to match as letters in order, which
// any query would.
func Words(query, words string) int {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return -1
	}
	score := 0
	for _, t := range terms {
		best := -1
		for _, w := range strings.Fields(strings.ToLower(words)) {
			if strings.HasPrefix(w, t) {
				best = max(best, Score(t, w))
			}
		}
		if best < 0 {
			return -1
		}
		score += best
	}
	return score
}
