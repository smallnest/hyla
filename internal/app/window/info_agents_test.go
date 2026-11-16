package window

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/marks"
	"hyla/locales"
)

// aLogo is an image of one colour, as a file the user gives.
func aLogo(t *testing.T, dir string, c color.RGBA) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestInfoAgents lays out the Info view's Agents card of an agent Hyla has
// no logo for, given one, for a look at it with HYLA_SHOTS.
func TestInfoAgents(t *testing.T) {
	a := testApp(t)
	a.showSide(sideInfo)
	s := a.session("s1")
	s.FocusPane().Agent = "acme"

	path := aLogo(t, t.TempDir(), color.RGBA{R: 200, G: 40, B: 40, A: 255})
	if err := marks.SetUserMark(marks.MarksDir(), "acme", path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { marks.RemoveUserMark("acme") })
	if _, ok := marks.LogoOf("acme"); !ok {
		t.Fatal("the logo the user gave is not read")
	}

	tt := ui.NewTester(a.view, 1100, 900)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	shot(t, tt, "info-agents")
	for _, text := range []string{locales.InfoAgents(), locales.InfoAgentsAdd(), locales.InfoAgentsRemove()} {
		if _, ok := tt.Find(text); !ok {
			t.Errorf("the card says nothing of %q", text)
		}
	}
}

// TestInfoAgentsNone checks the card of a pane that runs no agent.
func TestInfoAgentsNone(t *testing.T) {
	a := testApp(t)
	a.showSide(sideInfo)
	tt := ui.NewTester(a.view, 1100, 900)
	tt.Frame()
	if _, ok := tt.Find(locales.InfoAgentsNone()); !ok {
		t.Error("the card of a pane without an agent says nothing")
	}
}
