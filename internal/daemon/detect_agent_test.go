package daemon

import "testing"

// The Antigravity CLI runs as agy, and its scripts as antigravity.js: both
// are agents hylad knows, so that the sidebar gives them a mark.
func TestDetectAgents(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"agy", nil, "agy"},
		{"agy-1.5.9", nil, "agy"},
		{"bash", []string{"-c", "agy"}, "agy"},
		{"node", []string{"/usr/local/lib/node_modules/antigravity-cli/bin/antigravity.js"}, "antigravity"},
		{"antigravity", nil, "antigravity"},
		{"gemini", nil, "gemini"},
		{"claude", nil, "claude"},
		{"bash", nil, ""},
	} {
		if got := detectAgent(c.name, c.args); got != c.want {
			t.Errorf("%s %q: %q, want %q", c.name, c.args, got, c.want)
		}
	}
}
