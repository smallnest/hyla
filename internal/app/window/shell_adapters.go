package window

import (
	"hyla/internal/app/browse"
	"hyla/internal/app/host"
	"hyla/internal/app/shell"
	"hyla/internal/proto"
	"hyla/internal/ui/webpane"
)

// The app, as the commands a shell sends it ask it.
var _ shell.App = (*App)(nil)

// browse is the commands', on a host.
func (a *App) browse(h *host.Host, r *proto.BrowserRequest, answer func(*proto.BrowserResult, error)) {
	shell.Browse(a, h, r, answer)
}

// RemoteControl tells that commands may drive this window.
func (a *App) RemoteControl(h *host.Host) bool { return a.remoteControl(h) }

func (a *App) WebPage(pane string) (*browse.Pane, bool) { return shell.WebPage(a, pane) }

func (a *App) NextLoad(pane string) <-chan error { return shell.NextLoad(a, pane) }

func (a *App) Load(pane, url string) { shell.Load(a, pane, url) }

func (a *App) Loading(pane string) { shell.Loading(a, pane) }

func (a *App) Staged(pane string, on bool) { shell.Staged(a, pane, on) }

// EnsureWeb is the page a pane's browser shows, made if it had none.
func (a *App) EnsureWeb(p *proto.Pane) *webpane.Entry { return a.ensureWeb(p) }

// TellPage tells a host's page where it went.
func (a *App) TellPage(pane string, e *webpane.Entry) { webpane.TellPage(a, pane, e) }

// EntryOf is the page of a pane, as a page's commands read it.
func (a *App) EntryOf(pane string) *webpane.Entry { return a.Webs[pane] }
