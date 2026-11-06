package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The picture behind the sidebar is as tall as 80% of it, vertically
// centred, and against its left edge.
func TestSidebarArtPlacement(t *testing.T) {
	x, y, w, h := sidebarArtPlacement(720, 40, 80)
	if x != 8 || y != 72 || w != 288 || h != 576 {
		t.Fatalf("a portrait picture: x %v, y %v, w %v, h %v", x, y, w, h)
	}
	if y+h/2 != 360 {
		t.Errorf("not centred: %v to %v of 720", y, y+h)
	}
	x, y, w, h = sidebarArtPlacement(500, 100, 100)
	if x != 8 || y != 50 || w != 400 || h != 400 {
		t.Errorf("a square picture: x %v, y %v, w %v, h %v", x, y, w, h)
	}
	if x, y, w, h := sidebarArtPlacement(0, 40, 80); w != 0 || h != 0 {
		t.Errorf("no window: x %v, y %v, w %v, h %v", x, y, w, h)
	}
}

// A chosen picture shows behind the sidebar, and goes as the setting is
// cleared.
func TestSidebarArtShows(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 720)
	tt.Frame()
	before := tt.Image()

	// A green, which the wash is not.
	pic := image.NewRGBA(image.Rect(0, 0, 40, 80))
	for y := range 80 {
		for x := range 40 {
			pic.Set(x, y, color.RGBA{G: 255, A: 255})
		}
	}
	a.set.SidebarImage = "/tmp/picture.png"
	a.set.SidebarImageOpacity = 1
	a.art = sidebarArt{path: a.set.SidebarImage, bm: ui.NewBitmap(pic)}
	tt.Frame()
	after := tt.Image()

	// The picture's 80%: below the sidebar's header, above its footer.
	inside := func(img *image.RGBA, x, y int) color.RGBA { return img.RGBAAt(x, y) }
	if c := inside(after, 20, 620); c.G < 200 || c.R > 60 || c.B > 60 {
		t.Errorf("no picture behind the sidebar at 20,620: %v", c)
	}
	if inside(after, 20, 60) != inside(before, 20, 60) {
		t.Errorf("the picture covers the sidebar's header")
	}
	if inside(after, 20, 690) != inside(before, 20, 690) {
		t.Errorf("the picture runs past 80%% of the sidebar: %v, %v", inside(before, 20, 690), inside(after, 20, 690))
	}
	shot(t, tt, "sidebar-art")

	// At no opacity it shows not at all.
	a.set.SidebarImageOpacity = 0
	tt.Frame()
	if got := inside(tt.Image(), 20, 620); got != inside(before, 20, 620) {
		t.Errorf("the picture shows at 0 opacity: %v, was %v", got, inside(before, 20, 620))
	}

	a.set.SidebarImage = ""
	tt.Frame()
	if got := inside(tt.Image(), 20, 620); got != inside(before, 20, 620) {
		t.Errorf("the picture stays after it is cleared: %v, was %v", got, inside(before, 20, 620))
	}
}

// A file ui.DecodeBitmap reads no picture of is no picture.
func TestSidebarArtNoPicture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("no picture here"), 0o600); err != nil {
		t.Fatal(err)
	}
	if bm := decodeSidebarArt(path); bm != nil {
		t.Errorf("a text file: %v", bm)
	}
}
