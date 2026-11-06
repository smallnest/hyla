package main

import (
	"fmt"
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
	if paneGlass(pal, s) == nil {
		t.Fatal("a translucent pane has no material")
	}
	s.PaneOpacity = 1
	if paneGlass(pal, s) != nil {
		t.Error("an opaque pane has a material")
	}
	s.PaneOpacity = 0.5
	s.Vibrancy = false
	if paneGlass(pal, s) != nil {
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
	m := paneGlass(pal, s)
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
	if spec, ok := paneGlass(pal, s).(paneFrost).NativeGlass(); !ok || spec.Style != int(glass.Clear) {
		t.Errorf("the clear frost's glass is %+v, %v", spec, ok)
	}
	s.PaneBlur = false
	if _, ok := paneGlass(pal, s).(*paneWash); !ok {
		t.Errorf("without terminal blur, the card is %T", paneGlass(pal, s))
	}
}

// Plastic is a plain blur with the wash, not the system's glass.
func TestPanePlastic(t *testing.T) {
	pal := &lightPalette
	s := settings.Settings{Vibrancy: true, PaneOpacity: 0.5, PaneBlur: true, PaneMaterial: "plastic"}
	m := paneGlass(pal, s)
	p, ok := m.(panePlastic)
	if !ok {
		t.Fatalf("plastic is %T", m)
	}
	if _, ok := any(p).(ui.NativeGlassMaterial); ok {
		t.Error("plastic asks for the system's glass")
	}
}
