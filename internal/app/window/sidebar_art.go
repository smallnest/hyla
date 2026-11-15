package window

// Pictures, as the Appearance page chooses them: an image file of this
// machine, decoded on a goroutine. The sidebar's draws behind its
// sessions, as tall as 80% of the sidebar, vertically centred, and against
// its left edge; a terminal's fills its card, under its text.

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/art"
	"hyla/locales"
)

// sidebarArt is a picture, and the file it is of: the sidebar's, or a
// terminal's.
type sidebarArt = art.Picture

// pickSidebarImage asks for a picture of this machine for the sidebar,
// with the system's dialog, and keeps its path in the settings.
func (a *App) pickSidebarImage() {
	a.pickImage(&a.settingsIn.SidebarImage, locales.SetSidebarImage(), locales.SetSidebarImageDetail())
}

// pickPaneImage asks for a picture of this machine for the terminals,
// with the system's dialog, and keeps its path in the settings.
func (a *App) pickPaneImage() {
	a.pickImage(&a.settingsIn.PaneImage, locales.SetPaneImage(), locales.SetPaneImageDetail())
}

// pickImage asks for a picture of this machine with the system's dialog,
// and keeps its path where set points.
func (a *App) pickImage(set *string, title, detail string) {
	art.Pick(a, set, title, detail, locales.CommonOpen(), locales.SetSidebarImageFilter(), func() {
		if a.SetWin != nil {
			a.SetWin.Invalidate()
		}
	})
}

// DialogWindow is the window the system's dialog opens in front of: the
// Settings window, while it is open.
func (a *App) DialogWindow() *mygo.Window {
	if a.SetWin != nil {
		return a.SetWin
	}
	return a.Win
}

// sidebarArtView draws the picture behind the sidebar's sessions: as tall
// as 80% of the sidebar, vertically centred, and against its left edge,
// as wide as its shape makes it, and cropped by the sidebar where that
// is wider than the sidebar.
func (a *App) sidebarArtView(c *ui.Context) {
	if a.SidebarArt.Path != a.settingsIn.SidebarImage {
		a.loadSidebarArt(a.settingsIn.SidebarImage)
	}
	bm := a.SidebarArt.BM
	if bm == nil {
		return
	}
	_, winH := c.Size()
	nw, nh := ui.ImageSize(bm)
	x, y, w, h := art.Placement(winH, nw, nh)
	if w <= 0 || h <= 0 {
		return
	}
	// Under the sidebar's rows, which paint over the picture.
	ui.Image(c, bm).Absolute().Left(x).Top(y).Size(w, h).ZIndex(-1).PassThrough().
		Opacity(a.settingsIn.SidebarImageOpacity)
}

// loadSidebarArt reads and decodes the picture of a path off the main
// thread, keeping the bitmap when it comes.
func (a *App) loadSidebarArt(path string) {
	art.Load(a, &a.SidebarArt, path)
}

// loadPaneArt reads and decodes a terminal's picture, as loadSidebarArt
// does the sidebar's.
func (a *App) loadPaneArt(path string) {
	art.Load(a, &a.PaneArtimage, path)
}
