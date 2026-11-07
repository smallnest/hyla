package main

import (
	"fmt"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
)

// A translucent terminal's card has a material, and an opaque one none, in
// a window that is translucent too.
func TestPaneGlass(t *testing.T) {
	pal := &lightPalette
	s := settings.Settings{Vibrancy: true, PaneOpacity: 0.5}
	if paneGlass(pal, s, nil) == nil {
		t.Fatal("a translucent pane has no material")
	}
	s.PaneOpacity = 1
	if paneGlass(pal, s, nil) != nil {
		t.Error("an opaque pane has a material")
	}
	s.PaneOpacity = 0.5
	s.Vibrancy = false
	if paneGlass(pal, s, nil) != nil {
		t.Error("a pane in a window that is not translucent has a material")
	}
}

// A pane keeps its terminal, and its text, with a card material, and the
// desktop shows through the card at 0% ($HYLA_SHOTS).
func TestPaneGlassKeepsTheTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads libghostty-vt")
	}
	if err := terminal.Load(); err != nil {
		t.Fatal(err)
	}
	for _, opacity := range []float32{settings.DefaultPaneOpacity, 0} {
		a := testApp(t)
		a.prefs.Side = ""
		term, err := terminal.New(terminal.Options{Conn: idleConn{make(chan struct{})}})
		if err != nil {
			t.Fatal(err)
		}
		term.Feed([]byte("ready\r\n"))
		a.terms["a"], a.terms["b"] = &termEntry{term: term}, &termEntry{}
		a.set.PaneOpacity = opacity
		tt := ui.NewTester(a.view, 1100, 640)
		tt.Advance(time.Second)
		tt.Frame()
		b, ok := tt.Find("Terminal")
		if !ok || b.W <= 0 {
			t.Errorf("opacity %v: no terminal among %v", opacity, tt.Texts())
			term.Close()
			continue
		}
		tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
		tt.Frame()
		if f := a.focusOf(&a.st.Sessions[0]); f != "a" {
			t.Errorf("opacity %v: a press on the terminal focused %q", opacity, f)
		}
		shot(t, tt, fmt.Sprintf("pane-glass-%.2f", opacity))
		term.Close()
	}
}

// Terminal blur has the system's glass under the card, which the wash
// shows through; without it, the wash alone.
func TestPaneFrost(t *testing.T) {
	pal := &lightPalette
	s := settings.Settings{Vibrancy: true, PaneOpacity: 0.5, PaneBlur: true, PaneMaterial: "glass"}
	m := paneGlass(pal, s, nil)
	f, ok := m.(paneFrost)
	if !ok {
		t.Fatalf("terminal blur has no glass: %T", m)
	}
	if _, ok := any(f).(ui.NativeGlassMaterial); !ok {
		t.Error("the frost asks for no system glass")
	}
	if spec, ok := f.NativeGlass(); !ok || spec.Style != int(glass.Regular) {
		t.Errorf("the frost's glass is %+v, %v", spec, ok)
	}
	s.PaneMaterial = "glass-clear"
	if spec, ok := paneGlass(pal, s, nil).(paneFrost).NativeGlass(); !ok || spec.Style != int(glass.Clear) {
		t.Errorf("the clear frost's glass is %+v, %v", spec, ok)
	}
	s.PaneBlur = false
	if _, ok := paneGlass(pal, s, nil).(*paneWash); !ok {
		t.Errorf("without terminal blur, the card is %T", paneGlass(pal, s, nil))
	}
}

// Plastic is a plain blur with the wash, not the system's glass.
func TestPanePlastic(t *testing.T) {
	pal := &lightPalette
	s := settings.Settings{Vibrancy: true, PaneOpacity: 0.5, PaneBlur: true, PaneMaterial: "plastic"}
	m := paneGlass(pal, s, nil)
	p, ok := m.(panePlastic)
	if !ok {
		t.Fatalf("plastic is %T", m)
	}
	if _, ok := any(p).(ui.NativeGlassMaterial); ok {
		t.Error("plastic asks for the system's glass")
	}
}

// A terminal's picture, chosen, takes the card's background in place of the
// window's, translucent or not, and the card's color when none shows
// ($HYLA_SHOTS).
func TestPanePicture(t *testing.T) {
	pal := &lightPalette
	s := settings.Settings{PaneImage: "/tmp/picture.png", PaneImageOpacity: 0.6}
	m := paneGlass(pal, s, nil)
	p, ok := m.(*panePicture)
	if !ok || p.bm != nil || p.opacity != 0.6 || p.base != pal.card {
		t.Fatalf("a picture while it comes: %#v", m)
	}
	pic := image.NewRGBA(image.Rect(0, 0, 40, 80))
	withBM := &sidebarArt{path: s.PaneImage, bm: ui.NewBitmap(pic)}
	if ready := paneGlass(pal, s, withBM).(*panePicture); ready.bm == nil || ready.opacity != 0.6 {
		t.Errorf("a picture ready: %#v", ready)
	}
	// A dimmed card's color shows under it.
	dim := pal.cardDim
	art := &sidebarArt{path: s.PaneImage, card: &dim}
	if dimmed := paneGlass(pal, s, art).(*panePicture); dimmed.base != dim {
		t.Errorf("dimmed card: base %v, want %v", dimmed.base, dim)
	}

	// The card shows it in place of the window's background: a picture
	// ready, and none when it comes or the user chose none.
	a := testApp(t)
	a.set.PaneImage, a.set.PaneImageOpacity = s.PaneImage, 1
	a.paneArt = sidebarArt{path: a.set.PaneImage, bm: ui.NewBitmap(pic)}
	if p := a.paneBackground(pal, pal.card, &a.st.Sessions[0].Panes[0]).(*panePicture); p.bm == nil || p.base != pal.card {
		t.Errorf("the pane's card: %#v", p)
	}
	// A window that is not translucent: the card's color alone.
	a.set.PaneImage, a.set.Vibrancy = "", false
	if m := a.paneBackground(pal, pal.card, &a.st.Sessions[0].Panes[0]); m != nil {
		t.Errorf("no picture, and an opaque window: %T", m)
	}
}

// A card with a picture paints it as large as it covers, from the card's
// top, and over the card's color at the rest.
func TestPanePicturePaints(t *testing.T) {
	// Its own size: red at its top and green below it, in bands of rows.
	pic := image.NewRGBA(image.Rect(0, 0, 40, 84))
	for y := range 84 {
		for x := range 40 {
			c := color.RGBA{G: 255, A: 255}
			if y < 12 {
				c = color.RGBA{R: 255, A: 255}
			}
			pic.Set(x, y, c)
		}
	}
	m := &panePicture{bm: ui.NewBitmap(pic), opacity: 1, base: lightPalette.card}
	box := ui.Rect{X: 0, Y: 0, W: 200, H: 100}
	render := func() *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			ui.Box(c).Fill().Material(m)
		}, int(box.W), int(box.H), 1)
	}
	img := render()
	// As tall as the card, its top at the card's top: red there, green
	// below.
	if c := img.RGBAAt(100, 2); c.R < 200 || c.G > 60 || c.B > 60 {
		t.Errorf("the card's top: %v, want the picture's red", c)
	}
	if c := img.RGBAAt(100, 98); c.R > 60 || c.G < 200 || c.B > 60 {
		t.Errorf("the card's bottom: %v, want the picture's green", c)
	}
	// At half its opacity, the card's color shows through.
	m.opacity = 0.5
	img = render()
	if c := img.RGBAAt(100, 2); c.G < 60 || c.R < 120 {
		t.Errorf("at half opacity: %v, want the picture and the card's color mixed", c)
	}
}

// A card paints its picture as its fit says: stretched to it, all of it
// within it, its own size at its middle, or tiled from there.
func TestPanePictureFits(t *testing.T) {
	// Red at its top and green below, both in its top half.
	pic := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := range 40 {
		for x := range 40 {
			c := color.RGBA{G: 255, A: 255}
			if y < 16 {
				c = color.RGBA{R: 255, A: 255}
			}
			pic.Set(x, y, c)
		}
	}
	box := ui.Rect{X: 0, Y: 0, W: 200, H: 100}
	render := func(fit string) *image.RGBA {
		m := &panePicture{bm: ui.NewBitmap(pic), opacity: 1, fit: fit, base: lightPalette.card}
		return ui.Render(func(c *ui.Context) {
			ui.Box(c).Fill().Material(m)
		}, int(box.W), int(box.H), 1)
	}
	isRed := func(c color.RGBA) bool { return c.R > 200 && c.G < 60 }
	isGreen := func(c color.RGBA) bool { return c.R < 60 && c.G > 200 }
	// The card's color, where no picture shows.
	plain := func(c color.RGBA) bool { return !isRed(c) && !isGreen(c) }

	// Stretched: the picture's rows run from the card's top to its bottom.
	img := render("stretch")
	if c := img.RGBAAt(5, 2); !isRed(c) {
		t.Errorf("stretch, top: %v, want red", c)
	}
	if c := img.RGBAAt(5, 90); !isGreen(c) {
		t.Errorf("stretch, bottom: %v, want green", c)
	}
	if c := img.RGBAAt(195, 90); !isGreen(c) {
		t.Errorf("stretch, right: %v, want green", c)
	}

	// All of it: as large as the card holds, its shape kept, in the card's
	// middle: left and right of it, the card's color.
	img = render("fit")
	if c := img.RGBAAt(100, 2); !isRed(c) {
		t.Errorf("fit, top: %v, want red", c)
	}
	if c := img.RGBAAt(100, 70); !isGreen(c) {
		t.Errorf("fit, below the middle: %v, want green", c)
	}
	if c := img.RGBAAt(5, 50); !plain(c) {
		t.Errorf("fit, left: %v, want the card's color", c)
	}

	// Its own size at the middle.
	img = render("center")
	if c := img.RGBAAt(100, 42); !isRed(c) {
		t.Errorf("center, top: %v, want red", c)
	}
	if c := img.RGBAAt(100, 52); !isGreen(c) {
		t.Errorf("center, below the middle: %v, want green", c)
	}
	if c := img.RGBAAt(100, 5); !plain(c) {
		t.Errorf("center, above: %v, want the card's color", c)
	}
	if c := img.RGBAAt(5, 50); !plain(c) {
		t.Errorf("center, left: %v, want the card's color", c)
	}

	// Tiled from the middle: the picture again to the right of it.
	img = render("tile")
	if c := img.RGBAAt(152, 42); !isRed(c) {
		t.Errorf("tile, next to the middle: %v, want red", c)
	}
	if c := img.RGBAAt(152, 52); !isGreen(c) {
		t.Errorf("tile, next to the middle: %v, want green", c)
	}
	if c := img.RGBAAt(100, 42); !isRed(c) {
		t.Errorf("tile, the middle: %v, want red", c)
	}
}
