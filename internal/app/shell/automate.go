// Package shell runs the commands a shell, or an agent, sends a window:
// hyla do drives the app, and hyla browser drives a pane's page. Each
// answers off the main thread, as it waits on the window.
package shell

import (
	"github.com/egoist/mygo"

	"hyla/internal/app/browse"
	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/webpane"
)

// App is what the commands ask of the app they drive.
type App interface {
	webpane.Shell
	browse.App
	// EnsureWeb is the page the pane's browser shows, made if it had none.
	EnsureWeb(p *proto.Pane) *webpane.Entry
}

// Browse does what hyla browser asks of a browser pane's page, on a host.
func Browse(a App, h *host.Host, r *proto.BrowserRequest, answer func(*proto.BrowserResult, error)) {
	browse.Do(a, h, r, answer)
}

// WebPage is a pane's page, as hyla browser drives it, and whether the
// pane had one: one never shown gets it now.
func WebPage(a App, pane string) (*browse.Pane, bool) {
	s := a.SessionOfPane(pane)
	if s == nil {
		return &browse.Pane{}, false
	}
	p := s.PaneByID(pane)
	had := a.EntryOf(p.ID) != nil
	e := a.EnsureWeb(p)
	var page *mygo.Page
	if e.View != nil {
		page = e.View.Page()
	}
	return &browse.Pane{Page: page, View: e.View, Err: e.Err}, had
}

// NextLoad returns what the page's next load sends its end to.
func NextLoad(a App, pane string) <-chan error {
	if a.SessionOfPane(pane) == nil {
		return nil
	}
	return a.EntryOf(pane).NextLoad()
}

// Load sends a page to an address, and tells hylad of it.
func Load(a App, pane, url string) {
	e := a.EntryOf(pane)
	e.Load(url)
	a.TellPage(pane, e)
}

// Loading says that a page loads anew.
func Loading(a App, pane string) { a.EntryOf(pane).Loading = true }

// Staged says that the window lays the page out unseen, or stops.
func Staged(a App, pane string, on bool) {
	a.UpdateNow(func() { a.EntryOf(pane).Staged = on })
}
