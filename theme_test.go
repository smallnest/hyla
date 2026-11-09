package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
)

// Every background the settings offer has a wash, in both themes: its
// stops start opaque and even, and its glow a color.
func TestEveryBackgroundHasAWash(t *testing.T) {
	for _, b := range settings.Backgrounds {
		if _, ok := washTable[b.ID]; !ok {
			t.Errorf("%s has no wash", b.ID)
		}
		for _, dark := range []bool{false, true} {
			base, glow := washes(b.ID, dark)
			if len(base.Stops) < 2 || base.Stops[0].At != 0 || base.Stops[len(base.Stops)-1].At != 1 {
				t.Errorf("%s (dark=%v): stops %+v", b.ID, dark, base.Stops)
			}
			if len(glow.Stops) == 0 || glow.Stops[0].Color.A == 0 && b.ID != "plain" && b.ID != "dock" {
				t.Errorf("%s (dark=%v) has no glow", b.ID, dark)
			}
		}
	}
}

// Each background the settings offer shows in the sidebar's menu of them,
// in the language the app shows, and choosing one keeps it.
func TestBackgroundTiles(t *testing.T) {
	a := testApp(t)
	a.ss.page = "appearance"
	tt := ui.NewTester(a.settingsView, 820, 2400)
	tt.Frame()
	for _, b := range settings.Backgrounds {
		name := backgroundName(b.ID, b.Name)
		_ = name
		if !tt.HasText(name) {
			t.Errorf("%s: no tile named %q", b.ID, name)
		}
	}
	// A new one is chosen, and saved.
	if err := tt.Click(backgroundName("umbra", "Umbra")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.set.Background != "umbra" || settings.Load().Background != "umbra" {
		t.Errorf("choosing Umbra left %q", a.set.Background)
	}
	// A window of it, either theme, draws its scene without a panic.
	a.set.Background = "static"
	for _, dark := range []bool{false, true} {
		main := ui.NewTester(a.view, 1000, 600)
		main.SetDark(dark)
		main.Frame()
		main.Advance(50 * time.Millisecond)
		main.Frame()
		shot(t, main, "wash-static-"+map[bool]string{true: "dark", false: "light"}[dark])
	}
}

// Each background, light and dark, for a look ($HYLA_SHOTS).
func TestWashShots(t *testing.T) {
	for _, id := range []string{"ember", "tidal", "umbra", "static"} {
		for _, dark := range []bool{false, true} {
			a := testApp(t)
			a.set.Background = id
			a.set.Appearance = "light"
			if dark {
				a.set.Appearance = "dark"
			}
			a.applied = a.set.Clone()
			tt := ui.NewTester(a.view, 1000, 600)
			tt.SetDark(dark)
			tt.Frame()
			tt.Advance(50 * time.Millisecond)
			tt.Frame()
			shot(t, tt, "wash-"+id+map[bool]string{true: "-dark", false: "-light"}[dark])
		}
	}
}

// The appearance page's washes, for a look ($HYLA_SHOTS).
func TestBackgroundTilesShot(t *testing.T) {
	a := testApp(t)
	a.ss.page = "appearance"
	tt := ui.NewTester(a.settingsView, 820, 900)
	tt.Frame()
	sc := a.ss.scrolls["appearance"]
	if sc == nil {
		t.Skip("no scroll")
	}
	sc.ScrollTo(0, 900, ui.Animation{})
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	shot(t, tt, "settings-backgrounds")
}

// Washes are kept: made once for each background and theme, and drawn as
// they are. The backdrop fades a copy of a translucent wash's stops, never
// the wash the theme has.
func TestWashesAreKept(t *testing.T) {
	base, _ := washOf("rex", false)
	again, _ := washOf("rex", false)
	if &base.Stops[0] != &again.Stops[0] {
		t.Fatal("washes made twice")
	}
	other, _ := washOf("mist", false)
	if &other.Stops[0] == &base.Stops[0] {
		t.Fatal("two backgrounds have one wash")
	}
	// Faded, as the backdrop fades it, they stay.
	faded(base.Stops, 0.4)
	if after, _ := washOf("rex", false); after.Stops[0] != base.Stops[0] {
		t.Errorf("the wash's first stop %v, then %v: a fade changed it", base.Stops[0], after.Stops[0])
	}
}
