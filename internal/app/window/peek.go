package window

import (
	"github.com/egoist/mygo"
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/ui/panes"
)

// The app, as an image's preview asks it.
var _ panes.Peeker = (*App)(nil)

// peek is what the Info view shows of an image a path names.
type peek = panes.Peek

func (a *App) ImagePeek(paneID, link string, at, term ui.Rect) {
	panes.Image(a, a.peekView, &a.Peek, paneID, link, at, term)
}

func (a *App) closePeek() { panes.Close(&a.Peek) }

func (a *App) peekView(c *ui.Context) { panes.View(c, &a.Peek) }

func (a *App) endPeek() { panes.End(a, &a.Peek) }

func (a *App) Window() *mygo.Window { return a.Win }

func (a *App) ResolvePath(id, link string) string { return a.resolvePath(id, link) }
