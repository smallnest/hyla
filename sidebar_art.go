package main

// Pictures, as the Appearance page chooses them: an image file of this
// machine, decoded on a goroutine. The sidebar's draws behind its
// sessions, as tall as 80% of the sidebar, vertically centred, and against
// its left edge; a terminal's fills its card, under its text.

import (
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/locales"
)

// sidebarArtGap is the room the picture leaves the sidebar's left edge.
const sidebarArtGap = 8

// sidebarArtHeight is how much of the sidebar's height the picture takes.
const sidebarArtHeight = 0.8

// sidebarImageTypes are the files the dialog offers: the images
// ui.DecodeBitmap reads, as peekable names them.
var sidebarImageTypes = []string{"png", "jpg", "jpeg", "gif", "webp", "bmp"}

// sidebarArt is a picture, and the file it is of: the sidebar's, or a
// terminal's.
type sidebarArt struct {
	path string     // the file, as the settings name it; "" for none
	bm   *ui.Bitmap // nil while it comes, or when it is no picture
	// card is the color of a terminal's card, for its picture to show
	// over, as the card says it: dimmed when the user is in another
	// pane. Nil for the sidebar's picture.
	card *ui.Color
}

// pickSidebarImage asks for a picture of this machine for the sidebar,
// with the system's dialog, and keeps its path in the settings.
func (a *App) pickSidebarImage() {
	a.pickImage(&a.set.SidebarImage, locales.SetSidebarImage(), locales.SetSidebarImageDetail())
}

// pickPaneImage asks for a picture of this machine for the terminals,
// with the system's dialog, and keeps its path in the settings.
func (a *App) pickPaneImage() {
	a.pickImage(&a.set.PaneImage, locales.SetPaneImage(), locales.SetPaneImageDetail())
}

// pickImage asks for a picture of this machine with the system's dialog,
// and keeps its path where set points.
func (a *App) pickImage(set *string, title, detail string) {
	win := a.setWin
	if win == nil {
		win = a.win
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win, Title: title, ButtonLabel: locales.CommonOpen(),
			Message: detail,
			Filters: []mygo.FileFilter{{Name: locales.SetSidebarImageFilter(), Extensions: sidebarImageTypes}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.update(func() {
			*set = paths[0]
			if a.setWin != nil {
				a.setWin.Invalidate()
			}
		})
	}()
}

// sidebarArtView draws the picture behind the sidebar's sessions: as tall
// as 80% of the sidebar, vertically centred, and against its left edge,
// as wide as its shape makes it, and cropped by the sidebar where that
// is wider than the sidebar.
func (a *App) sidebarArtView(c *ui.Context) {
	if a.art.path != a.set.SidebarImage {
		a.loadSidebarArt(a.set.SidebarImage)
	}
	bm := a.art.bm
	if bm == nil {
		return
	}
	_, winH := c.Size()
	nw, nh := ui.ImageSize(bm)
	x, y, w, h := sidebarArtPlacement(winH, nw, nh)
	if w <= 0 || h <= 0 {
		return
	}
	// Under the sidebar's rows, which paint over the picture.
	ui.Image(c, bm).Absolute().Left(x).Top(y).Size(w, h).ZIndex(-1).PassThrough().
		Opacity(a.set.SidebarImageOpacity)
}

// sidebarArtPlacement is where the picture goes in a window winH tall: as
// tall as 80% of it, vertically centred, and against the sidebar's left
// edge, as wide as the picture's shape makes it.
func sidebarArtPlacement(winH, nw, nh float32) (x, y, w, h float32) {
	if winH <= 0 || nw <= 0 || nh <= 0 {
		return
	}
	h = sidebarArtHeight * winH
	// Centred: the room above the picture is half of what it leaves.
	return sidebarArtGap, (winH - h) / 2, h * nw / nh, h
}

// loadSidebarArt reads and decodes the picture of a path off the main
// thread, keeping the bitmap when it comes.
func (a *App) loadSidebarArt(path string) {
	s := &a.art
	s.path, s.bm = path, nil
	if path == "" {
		return
	}
	// A test has no window to wait for, and decodes now.
	if a.win == nil {
		s.bm = decodeSidebarArt(path)
		return
	}
	go func() {
		bm := decodeSidebarArt(path)
		a.update(func() {
			if a.art.path != path {
				return // another picture chosen while this was read
			}
			a.art.bm = bm
		})
	}()
}

// loadPaneArt reads and decodes a terminal's picture, as loadSidebarArt
// does the sidebar's.
func (a *App) loadPaneArt(path string) {
	s := &a.paneArt
	s.path, s.bm = path, nil
	if path == "" {
		return
	}
	if a.win == nil {
		s.bm = decodeSidebarArt(path)
		return
	}
	go func() {
		bm := decodeSidebarArt(path)
		a.update(func() {
			if a.paneArt.path != path {
				return // another picture chosen while this was read
			}
			a.paneArt.bm = bm
		})
	}()
}

// decodeSidebarArt reads a file and decodes it, nil for no picture.
func decodeSidebarArt(path string) *ui.Bitmap {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	bm, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bm
}
