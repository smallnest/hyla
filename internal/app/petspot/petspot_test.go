package petspot

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestPetOffsetAt(t *testing.T) {
	main := mygo.Rectangle{X: 100, Width: 1000}
	for _, c := range []struct {
		left float32
		want float32
	}{{505, 0.5}, {5, 0}, {1005, 1}} {
		if got := OffsetAt(main, c.left); got != c.want {
			t.Errorf("left %v: offset %v, want %v", c.left, got, c.want)
		}
	}
}
