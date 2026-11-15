// Package layout holds where the window's parts go: the title bar, the
// sidebar's slide, the cards' column, and the rows of a session.
package layout

import (
	"math"
	"time"

	ui "github.com/egoist/mygo/ui"

	"hyla/internal/platform"
)

const (
	// BarMid is the line the title bar's contents center on: the traffic
	// lights, the sidebar's buttons, the panes' headers. TopBar is the
	// sidebar's title bar, centered on it, and HeaderH a header of a card,
	// which starts CardGap down, centered on it too.
	BarMid  = 30
	TopBar  = 2 * BarMid
	HeaderH = 2 * (BarMid - CardGap)
	CardGap = 8 // around the card
	RowH    = 34
	RowH2   = 48 // a detailed row, of two lines
	// RowInset is where a sidebar row's text starts: a project's name and
	// its sessions', the badge telling the project from its children.
	RowInset = 12
	// ProjectBadge is the square before a project's name, with its first
	// letter; ProjectBadgeGap is the room after it.
	ProjectBadge    = 20
	ProjectBadgeGap = 6
	// RowGap is what a row leaves between its parts: the mark, the name,
	// the time.
	RowGap = 10
)

// Chrome is where the sidebar's animation puts things, in whole pixels so
// that edges stay crisp. Each is rounded once, from one progress quantized
// to the sidebar's pixels, so that they move together: rounded apart,
// they would step at different frames, and a title, their sum, would
// shake by a pixel as the spring settles.
type Chrome struct {
	Shown     float32 // the sidebar's progress, 0 hidden to 1 shown
	SideW     float32 // the sidebar's width as shown
	MainPad   float32 // the left padding of the cards' column
	HeaderPad float32 // the left padding of the header at the left edge
}

// ButtonsLeft places the sidebar's buttons that the header holds while
// the sidebar hides, in the header's coordinates, so that they stay where
// they are in the window, after the traffic lights, as the card slides.
func (ch Chrome) ButtonsLeft(bar ui.TitleBar) float32 {
	return Lead(bar) - (ch.SideW + ch.MainPad)
}

// lead is where the sidebar's buttons start in the window, with the sidebar
// shown or not: after the traffic lights on macOS; elsewhere inside the
// card's edge, which they come over as the sidebar hides.
func Lead(bar ui.TitleBar) float32 {
	if platform.OwnControls {
		return bar.Left + 2*CardGap
	}
	return bar.Left + CardGap
}

// SidebarMotion is the sidebar's slide.
var SidebarMotion = ui.Animation{Duration: 260 * time.Millisecond, Ease: ui.CubicBezier(0.25, 1, 0.5, 1)}

// For is where the sidebar's animation puts things, from its progress,
// its width and the title bar.
func For(progress, width float32, bar ui.TitleBar) Chrome {
	round := func(v float32) float32 { return float32(math.Round(float64(v))) }
	w := round(width * min(max(progress, 0), 1))
	s := w / width
	pad := round(CardGap * (1 - s))
	// Where the header's title starts in the window, rounded once: after
	// the traffic lights and the sidebar's buttons with the sidebar
	// hidden, 16 into the card with it shown.
	hidden, shown := Lead(bar)+16+60, width+16
	x := round(hidden + (shown-hidden)*s)
	return Chrome{Shown: s, SideW: w, MainPad: pad, HeaderPad: x - w - pad}
}

// B2F is a boolean as a float, 1 for true.
func B2F(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
