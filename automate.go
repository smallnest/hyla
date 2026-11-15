package main

import (
	"github.com/egoist/mygo"

	"hyla/internal/app/browse"
	"hyla/internal/app/host"
	"hyla/internal/proto"
)

// browse does what hyla browser asks of a browser pane's page, and
// answers: what waits on the page waits off the main thread. It is a
// remote control of the app, as hyla do is, and takes the user's leave
// as hyla do does. The commands are `internal/app/browse`.

var _ browse.App = (*App)(nil)

// browse is the commands', on a host.
func (a *App) browse(h *host.Host, r *proto.BrowserRequest, answer func(*proto.BrowserResult, error)) {
	browse.Do(a, h, r, answer)
}

// RemoteControl tells that commands may drive this window.
func (a *App) RemoteControl(h *host.Host) bool { return a.remoteControl(h) }

// WebPage is a pane's page, as hyla browser drives it, and whether the
// pane had one: one never shown gets it now.
func (a *App) WebPage(pane string) (*browse.Pane, bool) {
	s := a.sessionOfPane(pane)
	if s == nil {
		return &browse.Pane{}, false
	}
	p := s.PaneByID(pane)
	_, had := a.Webs[p.ID]
	e := a.ensureWeb(p)
	var page *mygo.Page
	if e.view != nil {
		page = e.view.Page()
	}
	return &browse.Pane{Page: page, View: e.view, Err: e.err}, had
}

// NextLoad returns what the page's next load sends its end to.
func (a *App) NextLoad(pane string) <-chan error {
	s := a.sessionOfPane(pane)
	if s == nil {
		return nil
	}
	return a.Webs[pane].nextLoad()
}

// Load sends a page to an address, and tells hylad of it.
func (a *App) Load(pane, url string) {
	e := a.Webs[pane]
	e.load(url)
	a.tellPage(pane, e)
}

// Loading says that a page loads anew.
func (a *App) Loading(pane string) { a.Webs[pane].loading = true }

// Staged says that the window lays the page out unseen, or stops.
func (a *App) Staged(pane string, on bool) {
	a.update(func() { a.Webs[pane].staged = on })
}
