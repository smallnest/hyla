package daemon

import "testing"

// A pane a phone took for its own screen goes back to the desktop's size
// when the phone lets it go: the client still showing it takes it, as one
// leaving does (TestPaneSizeClaimerLeaves). A pane no client shows keeps
// the size it has.
func TestPaneSizeRelease(t *testing.T) {
	s := testPane(t)
	size := paneSize(t, s)
	desktop := attached(t, s)
	s.ask(desktop, 90, 30, 9, 18)
	s.claim(desktop)
	size("the desktop's", 90, 30)

	// HylaPocket takes the pane and sizes it for the phone's screen.
	s.resize(40, 20, 0, 0)
	size("the phone's", 40, 20)

	// The phone lets the pane go: it is the desktop's again.
	s.release()
	size("back to the desktop's", 90, 30)

	// With no client showing it, the pane keeps the size it has, as the
	// last client asked for.
	s.detach(desktop)
	s.resize(40, 20, 0, 0)
	s.release()
	size("no client's", 40, 20)
}
