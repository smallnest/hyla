// Package theme holds Hyla's colors: the palette of each appearance, and
// the terminal's colors on it.
package theme

import "github.com/egoist/mygo/ui"

// Palette is hyla's colors, for the light and the dark appearance.
type Palette struct {
	Dark bool

	Sidebar      ui.Color // under the sidebar where no material shows
	Text         ui.Color
	TextMuted    ui.Color // project headers, times
	TextFaint    ui.Color // counts, hints
	RowHover     ui.Color
	RowSelected  ui.Color
	RowSelBorder ui.Color
	RowShadow    ui.Color

	Card        ui.Color // the terminal's card
	CardBorder  ui.Color
	CardShadow  ui.Color
	CardShadow2 ui.Color
	FloatShadow ui.Color
	CardDim     ui.Color // a pane the user is not in
	// edge and edgeHigh are the hairlines around surfaces in the dark:
	// cards and rows, and what floats or takes a click.
	Edge     ui.Color
	EdgeHigh ui.Color
	// veil fades a pane the user is not in, its text too, as Ghostty's
	// unfocused-split-opacity.
	Veil     ui.Color
	ButtonBg ui.Color
	// Controls' faces: buttons, fields, selects, a segmented control's
	// choice. Light ones stand on a soft shadow; dark ones on their
	// lighter fill alone, with a hint of light along their top, as macOS
	// draws them.
	Control       ui.Color
	ControlHover  ui.Color
	ControlShadow bool
	ControlLight  ui.Color
	Divider       ui.Color
	Chip          ui.Color

	Attention ui.Color // needs you
	AttnSoft  ui.Color
	Done      ui.Color // the filled check
	OnDone    ui.Color
	Online    ui.Color
	Danger    ui.Color
	Accent    ui.Color
	// group is a sidebar group's name, a project's or a day's: lighter than
	// its rows' names, which it tells apart.
	Group ui.Color

	// The timeline's bars, by what a step is and what it waits for: soft
	// colours, which a card of turns is read at a glance and no alarm.
	TlThink ui.Color // the agent's reasoning
	TlText  ui.Color // what the agent wrote
	TlTool  ui.Color // the tools it ran
	TlSub   ui.Color // the agents it gave its work to
	TlWait  ui.Color // a tool that waits for the user
	TlFail  ui.Color // a tool that failed

	Overlay   ui.Color // behind dialogs
	Popover   ui.Color
	Kbd       ui.Color
	KbdBorder ui.Color
}

var Light = Palette{
	Sidebar:      ui.Hex("#eceef4"),
	Text:         ui.Hex("#1d1d1f"),
	TextMuted:    ui.Hex("#6e6e78"),
	TextFaint:    ui.Hex("#a3a3ad"),
	Group:        ui.Hex("#888892"),
	RowHover:     ui.RGBA(0, 0, 0, 0.04),
	RowSelected:  ui.Hex("#ffffff"),
	RowSelBorder: ui.RGBA(0, 0, 0, 0.05),
	RowShadow:    ui.RGBA(20, 20, 50, 0.08),

	Card:          ui.Hex("#ffffff"),
	CardBorder:    ui.RGBA(0, 0, 0, 0.07),
	CardShadow:    ui.RGBA(0, 0, 0, 0.07),
	CardShadow2:   ui.RGBA(0, 0, 0, 0.035),
	FloatShadow:   ui.RGBA(20, 20, 40, 0.14),
	CardDim:       ui.RGBA(255, 255, 255, 0.58),
	Veil:          ui.RGBA(250, 248, 252, 0.28),
	ButtonBg:      ui.Hex("#ffffff"),
	Control:       ui.Hex("#ffffff"),
	ControlHover:  ui.Hex("#f7f7f9"),
	ControlShadow: true,
	Divider:       ui.RGBA(0, 0, 0, 0.06),
	Chip:          ui.RGBA(0, 0, 0, 0.045),

	TlThink:   ui.RGBA(150, 143, 210, 0.80),
	TlText:    ui.RGBA(110, 165, 145, 0.80),
	TlTool:    ui.RGBA(190, 175, 150, 0.90),
	TlSub:     ui.RGBA(120, 155, 200, 0.80),
	TlWait:    ui.RGBA(214, 160, 74, 0.85),
	TlFail:    ui.RGBA(203, 118, 112, 0.85),
	Attention: ui.Hex("#c27b0f"),
	AttnSoft:  ui.RGBA(232, 154, 40, 0.13),
	Done:      ui.Hex("#1d1d1f"),
	OnDone:    ui.Hex("#ffffff"),
	Online:    ui.Hex("#248a3d"),
	Danger:    ui.Hex("#d93a35"),
	Accent:    ui.Hex("#3b6cf6"),

	Overlay:   ui.RGBA(10, 10, 30, 0.12),
	Popover:   ui.Hex("#ffffff"),
	Kbd:       ui.RGBA(0, 0, 0, 0.04),
	KbdBorder: ui.RGBA(0, 0, 0, 0.09),
}

var Dark = Palette{
	Dark:         true,
	Sidebar:      ui.Hex("#141418"),
	Text:         ui.Hex("#ececf0"),
	TextMuted:    ui.Hex("#a9a9b4"),
	TextFaint:    ui.Hex("#83838f"),
	Group:        ui.Hex("#90909b"),
	RowHover:     ui.RGBA(255, 255, 255, 0.05),
	RowSelected:  ui.RGBA(255, 255, 255, 0.10),
	RowSelBorder: ui.RGBA(255, 255, 255, 0.06),
	RowShadow:    ui.RGBA(0, 0, 0, 0.25),

	Card:         ui.Hex("#232328"),
	CardBorder:   ui.RGBA(255, 255, 255, 0.08),
	CardShadow:   ui.RGBA(0, 0, 0, 0.35),
	CardShadow2:  ui.RGBA(0, 0, 0, 0.2),
	FloatShadow:  ui.RGBA(0, 0, 0, 0.45),
	CardDim:      ui.RGBA(35, 35, 40, 0.5),
	Veil:         ui.RGBA(14, 13, 18, 0.42),
	Edge:         ui.RGBA(255, 255, 255, 0.075),
	EdgeHigh:     ui.RGBA(255, 255, 255, 0.11),
	ButtonBg:     ui.Hex("#404047"),
	Control:      ui.Hex("#404047"),
	ControlHover: ui.Hex("#4a4a52"),
	ControlLight: ui.RGBA(255, 255, 255, 0.09),
	Divider:      ui.RGBA(255, 255, 255, 0.07),
	Chip:         ui.RGBA(255, 255, 255, 0.07),

	TlThink:   ui.RGBA(140, 132, 200, 0.85),
	TlText:    ui.RGBA(120, 172, 152, 0.85),
	TlTool:    ui.RGBA(168, 155, 134, 0.85),
	TlSub:     ui.RGBA(126, 156, 196, 0.85),
	TlWait:    ui.RGBA(206, 154, 70, 0.85),
	TlFail:    ui.RGBA(190, 120, 114, 0.85),
	Attention: ui.Hex("#f0a33a"),
	AttnSoft:  ui.RGBA(240, 163, 58, 0.15),
	Done:      ui.Hex("#ececf0"),
	OnDone:    ui.Hex("#161618"),
	Online:    ui.Hex("#30d158"),
	Danger:    ui.Hex("#ff6961"),
	Accent:    ui.Hex("#6b93ff"),

	Overlay:   ui.RGBA(0, 0, 0, 0.35),
	Popover:   ui.Hex("#232327"),
	Kbd:       ui.RGBA(255, 255, 255, 0.06),
	KbdBorder: ui.RGBA(255, 255, 255, 0.1),
}

// For returns the palette of the appearance c draws in.
func For(c *ui.Context) *Palette {
	if c.Theme().Dark {
		return &Dark
	}
	return &Light
}
