package info

import (
	"testing"
)

// A process's time as ps tells it reads in words: days as hours.
func TestDuration(t *testing.T) {
	for in, want := range map[string]string{
		"62:17:03": "62h 17m", "62-17:03": "1488h 17m", "62-17:03:00": "1505h 3m", "02:14": "2m 14s", "51": "51s", "51s": "51s", "": "",
		"1:02:03": "1h 2m", "0:00:05": "5s", "62h17m03s": "62h 17m",
	} {
		if got := duration(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
