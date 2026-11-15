package window

import (
	"hyla/internal/proto"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/panes"
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
func (a *App) DropTerms(pane string)     { a.dropTerm(pane) }
func (a *App) Shows(pane string) bool    { return a.shows(pane) }

func (a *App) splitPane(s *proto.Session, at, direction string) { panes.SplitPane(a, s, at, direction) }

func (a *App) split(s *proto.Session, p proto.CreateParams) { panes.Split(a, s, p) }

func (a *App) closePane(s *proto.Session, pane string, ask bool) { panes.ClosePane(a, s, pane, ask) }

func (a *App) zoomPane(s *proto.Session, pane string) { panes.ZoomPane(a, s, pane) }

func (a *App) focusDirection(dir string) { panes.FocusDirection(a, dir) }

func (a *App) cyclePane(d int) { panes.CyclePane(a, d) }
