// Package art holds pictures, as the Appearance page chooses them: an
// image file of this machine, decoded on a goroutine. The sidebar's draws
// behind its sessions, as tall as 80% of the sidebar, vertically centred,
// and against its left edge; a terminal's fills its card, under its text.
package art

import (
	"os"

	"github.com/egoist/mygo"
	ui "github.com/egoist/mygo/ui"
)

// Gap is the room a picture leaves the sidebar's left edge.
const Gap = 8

// Height is how much of the sidebar's height a picture takes.
const Height = 0.8

// Types are the files a dialog offers: the images ui.DecodeBitmap reads.
var Types = []string{"png", "jpg", "jpeg", "gif", "webp", "bmp"}

// Picture is a picture, and the file it is of: the sidebar's, or a
// terminal's.
type Picture struct {
	Path string     // the file, as the settings name it; "" for none
	BM   *ui.Bitmap // nil while it comes, or when it is no picture
	// Card is the color of a terminal's card, for its picture to show
	// over, as the card says it: dimmed when the user is in another
	// pane. Nil for the sidebar's picture.
	Card *ui.Color
}

// Picker is what a picture of this machine is chosen by.
type Picker interface {
	// DialogWindow is the window the system's dialog opens in front of.
	DialogWindow() *mygo.Window
	// Window is the app's window, nil in a test.
	Window() *mygo.Window
	// Update runs fn on the main thread.
	Update(fn func())
}

// Pick asks for a picture of this machine with the system's dialog, and
// keeps its path where set points. changed runs after the path is kept.
func Pick(p Picker, set *string, title, detail, button, filter string, changed func()) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: p.DialogWindow(), Title: title, ButtonLabel: button,
			Message: detail,
			Filters: []mygo.FileFilter{{Name: filter, Extensions: Types}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		p.Update(func() {
			*set = paths[0]
			changed()
		})
	}()
}

// Load reads and decodes the picture of a path off the main thread,
// keeping the bitmap when it comes.
func Load(l Picker, p *Picture, path string) {
	p.Path, p.BM = path, nil
	if path == "" {
		return
	}
	// A test has no window to wait for, and decodes now.
	if l.Window() == nil {
		p.BM = Decode(path)
		return
	}
	go func() {
		bm := Decode(path)
		l.Update(func() {
			if p.Path != path {
				return // another picture chosen while this was read
			}
			p.BM = bm
		})
	}()
}

// Decode reads a file and decodes it, nil for no picture.
func Decode(path string) *ui.Bitmap {
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

// Placement is where a picture goes in a window winH tall: as tall as
// 80% of it, vertically centred, and against the sidebar's left edge, as
// wide as the picture's shape makes it.
func Placement(winH, nw, nh float32) (x, y, w, h float32) {
	if winH <= 0 || nw <= 0 || nh <= 0 {
		return
	}
	h = Height * winH
	// Centred: the room above the picture is half of what it leaves.
	return Gap, (winH - h) / 2, h * nw / nh, h
}
