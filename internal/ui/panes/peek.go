package panes

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo"
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/ui/theme"
)

// Peek is the image a path printed in a pane shows while Command hovers
// it, in a flyout beside the path: the file, as of its modification time,
// decoded on a goroutine.
//
// The app draws the panes that hover it, and the frame's end closes a
// flyout no pane hovered.
type Peek struct {
	Path string // the file, or for a host's, what finds it there
	Name string // the file's name, as the caption says it
	Mod  time.Time
	BM   *ui.Bitmap // nil while it decodes, or when it is no image
	// Shown tells that a pane still hovers it this frame; the flyout
	// closes, and the bitmap goes, at the end of a frame without.
	Shown bool

	win    *mygo.Window // the flyout, once the bitmap is
	anchor mygo.Rectangle
	w, h   int
}

// peekable are the extensions of the images ui.DecodeBitmap reads.
var peekable = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true}

// maxPeekBytes keeps a huge file from being read whole for a glance.
const maxPeekBytes = 64 << 20

// The most a preview's picture takes, in DIPs; its padding, and the height
// of its caption.
const (
	peekW, peekH         = 360, 260
	peekPad, peekCaption = 6, 22
	// peekMinW leaves a small picture's caption room.
	peekMinW = 150
)

// Peeker is what a preview asks of the app it shows in.
type Peeker interface {
	// Window is the app's window, and ResolvePath the file a path
	// printed in a pane names, or "" for no such file.
	Window() *mygo.Window
	ResolvePath(id, link string) string
	// Update runs fn on the main thread, and draws.
	Update(fn func())
}

// Path is the file a path printed in a pane names: the link, without the
// line and column a compiler prints after it.
func Path(link string) string {
	return lineSuffix.ReplaceAllString(link, "")
}

var lineSuffix = regexp.MustCompile(`(:\d+){1,2}$`)

// Image shows the image a path names, which Command hovers in a pane's
// terminal at at, in the terminal's coordinates, in a flyout below the
// path, or above it where the screen has no room below. term is the
// terminal's box in the window. u draws the flyout's content.
func Image(a Peeker, u func(*ui.Context), p *Peek, paneID, link string, at, term ui.Rect) {
	if !peekable[strings.ToLower(filepath.Ext(Path(link)))] {
		return
	}
	if !local(a, p, paneID, link) {
		return
	}
	p.Shown = true
	if p.BM == nil {
		return
	}
	w, h := peekSize(p.BM)
	r := func(v float32) int { return int(math.Round(float64(v))) }
	anchor := mygo.Rectangle{X: r(term.X + at.X), Y: r(term.Y + at.Y), Width: r(at.W), Height: r(at.H)}
	fw, fh := r(max(w+2*peekPad, peekMinW)), r(h+peekPad+peekCaption)
	// The flyout is a window: it opens, or follows the link, once the
	// view is built.
	a.Update(func() { show(a, u, p, anchor, fw, fh) })
}

// local decodes the image of a path of this machine, as of its
// modification time, reporting whether there is one.
func local(a Peeker, p *Peek, paneID, link string) bool {
	path := a.ResolvePath(paneID, link)
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Size() > maxPeekBytes {
		return false
	}
	if p.Path != path || !p.Mod.Equal(fi.ModTime()) {
		Close(p)
		*p = Peek{Path: path, Name: filepath.Base(path), Mod: fi.ModTime()}
		mod := fi.ModTime()
		go func() {
			data, err := os.ReadFile(path)
			if err != nil {
				return
			}
			bm, err := ui.DecodeBitmap(data)
			if err != nil {
				return
			}
			a.Update(func() {
				if p.Path == path && p.Mod.Equal(mod) {
					p.BM = bm
				}
			})
		}()
	}
	return true
}

// peekSize is the size the picture shows at: its own, up to peekW by
// peekH.
func peekSize(bm *ui.Bitmap) (w, h float32) {
	pw, ph := bm.Size()
	if pw == 0 || ph == 0 {
		return 1, 1
	}
	s := min(1, peekW/float32(pw), peekH/float32(ph))
	return max(float32(pw)*s, 1), max(float32(ph)*s, 1)
}

// show opens the flyout of the image, w by h, against anchor, or moves it
// there.
func show(a Peeker, u func(*ui.Context), p *Peek, anchor mygo.Rectangle, w, h int) {
	if win := a.Window(); win == nil || p.BM == nil {
		return
	}
	if p.win == nil {
		win := mygo.NewFlyout(mygo.FlyoutOptions{
			Parent:       a.Window(),
			Anchor:       anchor,
			Gap:          6,
			Width:        w,
			Height:       h,
			Shadow:       true,
			CornerRadius: 10,
			Content:      ui.View(u),
		})
		win.OnClosed(func() {
			if p.win == win {
				p.win = nil
			}
		})
		p.win, p.anchor, p.w, p.h = win, anchor, w, h
		return
	}
	if anchor != p.anchor {
		p.anchor = anchor
		p.win.SetAnchor(anchor)
	}
	if w != p.w || h != p.h {
		p.w, p.h = w, h
		p.win.SetContentSize(w, h)
	}
}

// Close closes the flyout of the image, if open.
func Close(p *Peek) {
	if p.win != nil {
		p.win.Close()
		p.win = nil
	}
}

// MinWidth is the least width a flyout takes, for a small picture's
// caption.
const MinWidth = peekMinW

// Caption is the height of a flyout's caption.
const Caption = peekCaption

// Pad is the room between a flyout's picture and its edge.
const Pad = peekPad

// End closes a flyout no pane hovered this frame, and lets go of its
// bitmap. The app calls it at the end of a frame.
func End(a Peeker, p *Peek) {
	if !p.Shown {
		Close(p)
		*p = Peek{}
	}
	p.Shown = false
}

// View is the flyout's: the picture, with the file's name and the
// picture's size in pixels under it. The system draws its shadow and
// edge.
func View(c *ui.Context, p *Peek) {
	pal := theme.For(c)
	c.Root().Background(ui.Transparent)
	bm := p.BM
	if bm == nil {
		return
	}
	w, h := peekSize(bm)
	pw, ph := bm.Size()
	ui.Column(c).Fill().Padding(peekPad, peekPad, 0, peekPad).AlignItems(ui.Center).Radius(10).Background(pal.Popover).Children(func() {
		ui.Image(c, bm).Size(w, h).Fit(ui.Contain).Radius(5)
		ui.Row(c).Height(peekCaption).AlignSelf(ui.Stretch).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, p.Name).FontSize(11.5).TextColor(pal.TextMuted).SingleLine().Grow(1).MinWidth(0)
			ui.Text(c, Itoa(pw)+" × "+Itoa(ph)).FontSize(11.5).FontFeatures("tnum").TextColor(pal.TextFaint).SingleLine().Shrink(0)
		})
	})
}
