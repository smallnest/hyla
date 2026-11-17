package window

import (
	"testing"
	"time"

	"hyla/internal/app/host"
)

// A pane's terminal whose stream ended is dropped and another attached, so
// that the pane takes input again; a terminal another entry replaced is
// left alone, and a pane that is gone gets none.
func TestWatchTerm(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	s := a.session("s1")
	p := s.FocusPane()
	ended := make(chan struct{})
	close(ended)

	e := &termEntry{started: time.Now().Add(-time.Second)}
	a.Terms[p.ID] = e
	a.watchTerm(p.ID, e, ended)
	again := a.Terms[p.ID]
	if again == nil || again == e {
		t.Fatalf("the pane's terminal is %v, want one attached again", again)
	}

	// The entry that replaced it attached on its own: its end is not this
	// one's.
	other := &termEntry{started: time.Now()}
	a.Terms[p.ID] = other
	a.watchTerm(p.ID, e, ended)
	if a.Terms[p.ID] != other {
		t.Error("a stale terminal's end dropped the one that replaced it")
	}

	// A stream that ended at once is no reason to attach again: it would
	// end as quickly.
	young := &termEntry{started: time.Now()}
	a.Terms[p.ID] = young
	a.watchTerm(p.ID, young, ended)
	if a.Terms[p.ID] != nil {
		t.Error("a terminal that ended at once left a new one behind")
	}

	// A pane the app no longer knows is dropped, and nothing attaches.
	a.Terms[p.ID] = e
	s.Panes = nil
	a.watchTerm(p.ID, e, ended)
	if a.Terms[p.ID] != nil {
		t.Error("a terminal of a gone pane stayed")
	}
}
