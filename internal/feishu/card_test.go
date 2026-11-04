package feishu

import "testing"

// A card the bot sent before the rename answers as Hyla's cards do.
func TestCardValueOfKeroCard(t *testing.T) {
	for _, tt := range []struct {
		value map[string]any
		want  string
		ok    bool
	}{
		{map[string]any{"hyla": "open"}, "open", true},
		{map[string]any{"kero": "open"}, "open", true},
		{map[string]any{"kero": "perm"}, "perm", true},
		{map[string]any{"other": "open"}, "", false},
		{nil, "", false},
	} {
		got, ok := cardValue(tt.value, "hyla")
		if got != tt.want || ok != tt.ok {
			t.Errorf("cardValue(%v) = %q, %v; want %q, %v", tt.value, got, ok, tt.want, tt.ok)
		}
	}
}
