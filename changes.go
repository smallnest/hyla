package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"hyla/internal/proto"
	"hyla/internal/ui/changesview"
	"hyla/internal/ui/theme"
)

// The app, as the Changes view asks it.
var _ changesview.App = (*App)(nil)

func (a *App) ChangesState() *changesview.State { return &a.Side.Changes }

func (a *App) ChangesView(c *ui.Context, pal *theme.Palette, s *proto.Session, width float32) {
	changesview.View(a, c, pal, s, width)
}

// changesDir is the worktree the Changes view lists.
func (a *App) changesDir(s *proto.Session) string { return changesview.Dir(a, s) }

func (a *App) lineCounts(c *ui.Context, pal *theme.Palette, added, removed int, size float32) {
	changesview.LineCounts(a, c, pal, added, removed, size)
}

func (a *App) DiffSplit(width float32) bool { return changesview.DiffSplit(a, width) }

func (a *App) CodeTheme(c *ui.Context, pal *theme.Palette) *editor.Theme {
	return changesview.CodeTheme(a, c, pal)
}

func (a *App) SideMessage(c *ui.Context, pal *theme.Palette, title, detail string) {
	changesview.Message(c, pal, title, detail)
}

// Message is the Chat view's, as it has nothing to show.
func (a *App) Message(c *ui.Context, pal *theme.Palette, title, detail string) {
	a.SideMessage(c, pal, title, detail)
}

// fileCollapsed tells whether a file's diff is folded.
func (a *App) fileCollapsed(scope string, f proto.GitFile) bool {
	return changesview.FileCollapsed(a, scope, f)
}

// showText puts a file's text in its diff, as it comes from hylad.
func (a *App) showText(sf *changesview.File, t proto.GitText) { changesview.ShowText(sf, t) }

func (a *App) codeTheme(c *ui.Context, pal *theme.Palette) *editor.Theme { return a.CodeTheme(c, pal) }
