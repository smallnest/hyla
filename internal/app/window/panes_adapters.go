package window

import (
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/theme"
)

// The app, as the pane actions ask it.
var _ panes.App = (*App)(nil)

func (a *App) SelectedSession() *proto.Session { return a.session(a.Selected) }

func (a *App) FocusMap() map[string]string {
	if a.Focus == nil {
		a.Focus = map[string]string{}
	}
	return a.Focus
}

func (a *App) SetConfirm(q *confirm.Ask) { a.Confirm = q }

func (a *App) CloseSession(id string, ask bool) { a.closeSession(id, ask) }

func (a *App) CallPane(id, method string, params any, done func(err error)) {
	a.call(id, method, params, done)
}

func (a *App) DropTerms(pane string) { a.dropTerm(pane) }

func (a *App) DropWeb(pane string) { a.dropWeb(pane) }

func (a *App) ReplaceSession(h *host.Host, s proto.Session) { a.replaceSession(h, s) }

func (a *App) EnsureTerm(pane string) { a.ensureTerm(pane) }

func (a *App) Touch(pane string) { a.touch(pane) }

func (a *App) Shows(pane string) bool { return a.shows(pane) }

func (a *App) splitPane(s *proto.Session, at, direction string) { panes.SplitPane(a, s, at, direction) }

func (a *App) split(s *proto.Session, p proto.CreateParams) { panes.Split(a, s, p) }

func (a *App) closePane(s *proto.Session, pane string, ask bool) { panes.ClosePane(a, s, pane, ask) }

func (a *App) zoomPane(s *proto.Session, pane string) { panes.ZoomPane(a, s, pane) }

func (a *App) focusDirection(dir string) { panes.FocusDirection(a, dir) }

func (a *App) cyclePane(d int) { panes.CyclePane(a, d) }

func (a *App) PaneMessage(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func()) {
	a.message(c, pal, title, detail, action, fn)
}
