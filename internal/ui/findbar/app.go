package findbar

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/webpane"
)

// App is what the find bar asks of the app whose panes it searches.
type App interface {
	// FindState is the bar's state.
	FindState() *State
	// SelectedSession is the session the window shows, and FocusOf the
	// pane the user is in.
	SelectedSession() *proto.Session
	FocusOf(s *proto.Session) string
	// TermOf is a pane's terminal in the window, and WebsNow the
	// browser panes' pages.
	TermOf(pane string) *terminal.Terminal
	WebsNow() map[string]*webpane.Entry
	FindInPage(e *webpane.Entry, next, back bool)
	// PaneRef is a pane's handle.
	PaneRef(id string) *ui.Handle
	// RunLater runs fn after the frame, and Tip adds an action's shortcut
	// to a title.
	RunLater(fn func())
	Tip(title, id string) string
}
