package main

import (
	"testing"

	"hyla/internal/ui/magpieview"
)

// TestPaneSessionCounted checks that Magpie's session is the one the
// pane's agent told hylad, and nothing while it told of none.
func TestPaneSessionCounted(t *testing.T) {
	a := testApp(t)
	p := a.session("s1").FocusPane()
	if s := magpieview.PaneSession(a); s != "" {
		t.Errorf("a pane without an agent has session %q", s)
	}
	p.Agent = "claude"
	// Without hylad's answer, the pane's agent has no session.
	if s := magpieview.PaneSession(a); s != "" {
		t.Errorf("an agent whose session hylad does not tell has %q", s)
	}
}
