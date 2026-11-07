package main

import (
	"hyla/internal/proto"

	"hyla/locales"
)

// Files dragged from this machine onto what a session shows, its panes or
// the panel beside them, go to the session's panes: those of another
// machine's put them where their programs run, and paste their paths there;
// those of this machine stay where they are, and paste their paths into
// their programs (drop_upload.go).
//
// A window's content floats the files a drag carries over it, and the app
// fades a file dragged over a page where a terminal is drawn, the page
// being over the content and the files under it.

// cardDrop takes the files dropped on the card of the session s, on its
// panes or the panel beside them.
func (a *App) cardDrop(s *proto.Session, files []string) {
	if s == nil || len(files) == 0 {
		return
	}
	p := a.dropPane(s)
	if p == nil {
		return
	}
	h := a.hostOf(p.ID)
	if h.remote() {
		if h.conn != connReady {
			a.toast(locales.HostNotConnected(h.label()))
			return
		}
		a.dropUpload(p.ID, files)
		return
	}
	// The file stays where it is, of this machine, and its path goes into
	// the program there, as it is.
	a.pasteDrop(p.ID, files)
}

// dropPane returns the pane of the session s that files dropped on its
// card go to: the one the pointer is in, else the session's.
func (a *App) dropPane(s *proto.Session) *proto.Pane {
	if p := s.PaneByID(a.hoverPane); p != nil {
		return p
	}
	return s.FocusPane()
}
