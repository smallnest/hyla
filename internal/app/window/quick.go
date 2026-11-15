package window

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/quick"
	"hyla/internal/ui/side"
	"hyla/internal/ui/theme"
)

// The quick terminal's state, and its panel, are internal/ui/quick's,
// which this file answers. QuickAction is the settings page's name of
// its action.
const QuickAction = quick.Action

type quickTerm = quick.State

func (a *App) Quick() *quick.State { return &a.quick }

func (a *App) Recording() string { return a.Ss.Recording }

func (a *App) Quitting() bool { return a.quittingApp }

func (a *App) LocalHost() *host.Host { return a.Local() }

func (a *App) SessionOf(id string) *proto.Session { return a.session(id) }

func (a *App) QuickSession() string { return a.Prefs.QuickSession }

func (a *App) SetQuickSession(id string) { a.Prefs.QuickSession = id }

func (a *App) TermOptions(id string, conn *quick.Gated) terminal.Options {
	return a.termOptions(id, conn)
}

func (a *App) PaneMessageOfQuick(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func()) {
	a.PaneMessage(c, pal, title, detail, action, fn)
}

func (a *App) Chrome() panes.Chrome {
	return panes.Chrome{SideW: a.ChromeState.SideW, MainPad: a.ChromeState.MainPad, HeaderPad: a.ChromeState.HeaderPad, Lead: layout.Lead(a.Bar)}
}

func (a *App) RightShown() bool { return a.rightShown() }

func (a *App) SideWidth(rowW float32) float32 { return a.sideWidth(rowW) }

func (a *App) SideState() *side.State { return &a.Side }

func (a *App) ReportView() { a.ReportViewNow() }

// applyGlobalShortcuts registers the quick terminal's shortcut.
func (a *App) applyGlobalShortcuts() { quick.ApplyGlobalShortcuts(a) }

func (a *App) toggleQuick() { quick.Toggle(a) }

func (a *App) showQuick() { quick.Show(a) }

func (a *App) hideQuick() { quick.Hide(a) }

func (a *App) quickChanged() { quick.Changed(a) }

func (a *App) attachQuick() { quick.Attach(a) }

func (a *App) quickTermFor(id string) *terminal.Terminal { return quick.TermFor(a, id) }

// TermFor returns the terminal of a pane, the window's or the quick
// terminal's, else nil.
func (a *App) TermFor(id string) *terminal.Terminal {
	if e := a.Terms[id]; e != nil && e.term != nil {
		return e.term
	}
	return a.quickTermFor(id)
}

func (a *App) dropQuick() { quick.Drop(a) }
