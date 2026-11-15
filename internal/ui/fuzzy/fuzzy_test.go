package fuzzy

import "testing"

// A query finds its letters in order, and finds nothing it lacks.
func TestScore(t *testing.T) {
	if got := Score("", "anything"); got != 0 {
		t.Errorf("an empty query scores %d, want 0", got)
	}
	for _, c := range []struct{ query, text string }{
		{"nt", "Session: New Terminal"},
		{"newterm", "Session: New Terminal"},
		{"snt", "Session: New Terminal"},
	} {
		if got := Score(c.query, c.text); got < 0 {
			t.Errorf("%q does not find %q: %d", c.query, c.text, got)
		}
	}
	for _, c := range []struct{ query, text string }{
		{"xyz", "Session: New Terminal"},
		{"eq", "Session: New Terminal"}, // out of order
	} {
		if got := Score(c.query, c.text); got != -1 {
			t.Errorf("%q finds %q: %d, want -1", c.query, c.text, got)
		}
	}
	// A word's start scores above a run inside a word.
	start := Score("term", "New Terminal")
	inside := Score("ermi", "New Terminal")
	if start <= inside {
		t.Errorf("a word's start scores %d, inside %d: want the start higher", start, inside)
	}
}

// Every term of a query starts a word of what it finds, never fewer.
func TestWords(t *testing.T) {
	if got := Words("  ", "anything"); got != -1 {
		t.Errorf("an empty query scores %d, want -1", got)
	}
	if got := Words("new term", "Session: New Terminal"); got < 0 {
		t.Errorf("new term does not find New Terminal: %d", got)
	}
	if got := Words("new missed", "Session: New Terminal"); got != -1 {
		t.Errorf("a term that starts no word scores %d, want -1", got)
	}
	// Letters in order are no match: that is Score's work.
	if got := Words("snt", "Session: New Terminal"); got != -1 {
		t.Errorf("out-of-order letters score %d, want -1", got)
	}
}
