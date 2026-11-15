package window

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/findbar"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/webpane"

	"hyla/internal/proto"
)

// The find bar is internal/ui/findbar's, which this file answers.

type findState = findbar.State

func (a *App) FindState() *findbar.State { return &a.Find }

// TermOf is a pane's terminal in the window.
func (a *App) TermOf(pane string) *terminal.Terminal {
	if e := a.Terms[pane]; e != nil {
		return e.term
	}
	return nil
}

func (a *App) openFind() { findbar.Open(a) }

func (a *App) findStep(back bool) { findbar.Step(a, back) }

func (a *App) closeFind() { findbar.Close(a) }

func (a *App) findTerm() *terminal.Terminal { return a.TermOf(a.Find.Pane) }

func (a *App) FindInPage(e *webpane.Entry, next, back bool) { webpane.FindInPage(a, e, next, back) }

func (a *App) findBar(c *ui.Context, pal *theme.Palette, p *proto.Pane, t *terminal.Terminal) {
	findbar.Bar(a, c, pal, p, t)
}
