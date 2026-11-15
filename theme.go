package main

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/wash"

	"hyla/internal/settings"
)

// palette is hyla's colors, for the light and the dark appearance.
type palette struct {
	dark bool

	sidebar      ui.Color // under the sidebar where no material shows
	text         ui.Color
	textMuted    ui.Color // project headers, times
	textFaint    ui.Color // counts, hints
	rowHover     ui.Color
	rowSelected  ui.Color
	rowSelBorder ui.Color
	rowShadow    ui.Color

	card        ui.Color // the terminal's card
	cardBorder  ui.Color
	cardShadow  ui.Color
	cardShadow2 ui.Color
	floatShadow ui.Color
	cardDim     ui.Color // a pane the user is not in
	// edge and edgeHigh are the hairlines around surfaces in the dark:
	// cards and rows, and what floats or takes a click.
	edge     ui.Color
	edgeHigh ui.Color
	// veil fades a pane the user is not in, its text too, as Ghostty's
	// unfocused-split-opacity.
	veil     ui.Color
	buttonBg ui.Color
	// Controls' faces: buttons, fields, selects, a segmented control's
	// choice. Light ones stand on a soft shadow; dark ones on their
	// lighter fill alone, with a hint of light along their top, as macOS
	// draws them.
	control       ui.Color
	controlHover  ui.Color
	controlShadow bool
	controlLight  ui.Color
	divider       ui.Color
	chip          ui.Color

	attention ui.Color // needs you
	attnSoft  ui.Color
	done      ui.Color // the filled check
	onDone    ui.Color
	danger    ui.Color
	accent    ui.Color
	// group is a sidebar group's name, a project's or a day's: lighter than
	// its rows' names, which it tells apart.
	group ui.Color

	overlay   ui.Color // behind dialogs
	popover   ui.Color
	kbd       ui.Color
	kbdBorder ui.Color
}

var lightPalette = palette{
	sidebar:      ui.Hex("#eceef4"),
	text:         ui.Hex("#1d1d1f"),
	textMuted:    ui.Hex("#6e6e78"),
	textFaint:    ui.Hex("#a3a3ad"),
	group:        ui.Hex("#888892"),
	rowHover:     ui.RGBA(0, 0, 0, 0.04),
	rowSelected:  ui.Hex("#ffffff"),
	rowSelBorder: ui.RGBA(0, 0, 0, 0.05),
	rowShadow:    ui.RGBA(20, 20, 50, 0.08),

	card:          ui.Hex("#ffffff"),
	cardBorder:    ui.RGBA(0, 0, 0, 0.07),
	cardShadow:    ui.RGBA(0, 0, 0, 0.07),
	cardShadow2:   ui.RGBA(0, 0, 0, 0.035),
	floatShadow:   ui.RGBA(20, 20, 40, 0.14),
	cardDim:       ui.RGBA(255, 255, 255, 0.58),
	veil:          ui.RGBA(250, 248, 252, 0.28),
	buttonBg:      ui.Hex("#ffffff"),
	control:       ui.Hex("#ffffff"),
	controlHover:  ui.Hex("#f7f7f9"),
	controlShadow: true,
	divider:       ui.RGBA(0, 0, 0, 0.06),
	chip:          ui.RGBA(0, 0, 0, 0.045),

	attention: ui.Hex("#c27b0f"),
	attnSoft:  ui.RGBA(232, 154, 40, 0.13),
	done:      ui.Hex("#1d1d1f"),
	onDone:    ui.Hex("#ffffff"),
	danger:    ui.Hex("#d93a35"),
	accent:    ui.Hex("#3b6cf6"),

	overlay:   ui.RGBA(10, 10, 30, 0.12),
	popover:   ui.Hex("#ffffff"),
	kbd:       ui.RGBA(0, 0, 0, 0.04),
	kbdBorder: ui.RGBA(0, 0, 0, 0.09),
}

var darkPalette = palette{
	dark:         true,
	sidebar:      ui.Hex("#141418"),
	text:         ui.Hex("#ececf0"),
	textMuted:    ui.Hex("#a9a9b4"),
	textFaint:    ui.Hex("#83838f"),
	group:        ui.Hex("#90909b"),
	rowHover:     ui.RGBA(255, 255, 255, 0.05),
	rowSelected:  ui.RGBA(255, 255, 255, 0.10),
	rowSelBorder: ui.RGBA(255, 255, 255, 0.06),
	rowShadow:    ui.RGBA(0, 0, 0, 0.25),

	card:         ui.Hex("#232328"),
	cardBorder:   ui.RGBA(255, 255, 255, 0.08),
	cardShadow:   ui.RGBA(0, 0, 0, 0.35),
	cardShadow2:  ui.RGBA(0, 0, 0, 0.2),
	floatShadow:  ui.RGBA(0, 0, 0, 0.45),
	cardDim:      ui.RGBA(35, 35, 40, 0.5),
	veil:         ui.RGBA(14, 13, 18, 0.42),
	edge:         ui.RGBA(255, 255, 255, 0.075),
	edgeHigh:     ui.RGBA(255, 255, 255, 0.11),
	buttonBg:     ui.Hex("#404047"),
	control:      ui.Hex("#404047"),
	controlHover: ui.Hex("#4a4a52"),
	controlLight: ui.RGBA(255, 255, 255, 0.09),
	divider:      ui.RGBA(255, 255, 255, 0.07),
	chip:         ui.RGBA(255, 255, 255, 0.07),

	attention: ui.Hex("#f0a33a"),
	attnSoft:  ui.RGBA(240, 163, 58, 0.15),
	done:      ui.Hex("#ececf0"),
	onDone:    ui.Hex("#161618"),
	danger:    ui.Hex("#ff6961"),
	accent:    ui.Hex("#6b93ff"),

	overlay:   ui.RGBA(0, 0, 0, 0.35),
	popover:   ui.Hex("#232327"),
	kbd:       ui.RGBA(255, 255, 255, 0.06),
	kbdBorder: ui.RGBA(255, 255, 255, 0.1),
}

func paletteFor(c *ui.Context) *palette {
	if c.Theme().Dark {
		return &darkPalette
	}
	return &lightPalette
}

// Terminal themes on the card's color, with palettes tuned to read on it:
// Apple's system colors, a little deeper in the light.
var lightTermTheme = &terminal.Theme{
	Foreground:    ui.Hex("#1d1d1f"),
	Background:    lightPalette.card,
	Cursor:        ui.Hex("#1d1d1f"),
	CursorText:    ui.Hex("#ffffff"),
	Selection:     ui.Hex("#cfdcfb"),
	SelectionText: ui.Hex("#1d1d1f"),
	Palette: [16]ui.Color{
		ui.Hex("#1d1d1f"), ui.Hex("#d3302f"), ui.Hex("#22863a"), ui.Hex("#b08800"),
		ui.Hex("#2f5bd8"), ui.Hex("#a033c4"), ui.Hex("#0f8199"), ui.Hex("#8e8e93"),
		ui.Hex("#5f5f66"), ui.Hex("#ef4444"), ui.Hex("#2fa84f"), ui.Hex("#d4a017"),
		ui.Hex("#3b82f6"), ui.Hex("#c04ee0"), ui.Hex("#14a3bf"), ui.Hex("#b4b4bb"),
	},
}

var darkTermTheme = &terminal.Theme{
	Foreground:    ui.Hex("#e6e6ea"),
	Background:    darkPalette.card,
	Cursor:        ui.Hex("#e6e6ea"),
	CursorText:    ui.Hex("#161618"),
	Selection:     ui.Hex("#2f4170"),
	SelectionText: ui.Hex("#ffffff"),
	Palette: [16]ui.Color{
		ui.Hex("#3a3a40"), ui.Hex("#ff6b64"), ui.Hex("#4fd17a"), ui.Hex("#f2c14e"),
		ui.Hex("#6b9bff"), ui.Hex("#d07cf0"), ui.Hex("#4cc9e0"), ui.Hex("#c9c9cf"),
		ui.Hex("#6c6c78"), ui.Hex("#ff8a84"), ui.Hex("#74e397"), ui.Hex("#ffd479"),
		ui.Hex("#8fb3ff"), ui.Hex("#e09cf7"), ui.Hex("#7fdcec"), ui.Hex("#f5f5f7"),
	},
}

// washOpacity is how much of the window's background the wash covers
// while the material shows: the setting's, else the default.
func washOpacity(s settings.Settings) float32 {
	if s.WindowOpacity >= 0 && s.WindowOpacity <= 1 {
		return s.WindowOpacity
	}
	return settings.DefaultWindowOpacity
}

// backdrop paints the window's wash of color, under the sidebar and
// between the cards: the base on the root, a glow from the top right on
// row, which holds everything else, and a scene's shapes over both. It lies over the window's material
// where there is one, and over the desktop when the window is blurless;
// the plain one is the material or the desktop alone.
func backdrop(c *ui.Context, pal *palette, row ui.Element, s settings.Settings) {
	base, glow := wash.Washes(s.Background, pal.dark)
	if s.Background == "plain" {
		// The material alone is as light or as dark as the wallpaper
		// under it: a base of the sidebar's color keeps the text legible,
		// letting the desktop show a little.
		bg := pal.sidebar
		if s.Vibrancy {
			bg = bg.Alpha(washOpacity(s))
		}
		c.Root().Background(bg)
		return
	}
	alpha := float32(1)
	if s.Vibrancy {
		// The wash is translucent over the material, or over the desktop
		// when the window is blurless: its glow too, as at 0% none of it
		// shows.
		alpha = washOpacity(s)
		base.Stops = wash.Faded(base.Stops, alpha)
		glow.Stops = wash.Faded(glow.Stops, alpha)
	}
	c.Root().LinearGradient(base)
	row.LinearGradient(glow)
	if scene := wash.Table[s.Background].Scene; scene != nil {
		// Its shapes lie over the wash as it shows: as faint as the
		// window's material makes it.
		// The scene takes the wash's opacity in its colors, as the wash
		// does.
		row.Draw(func(p *ui.Painter, r ui.Rect) { scene(p, r, pal.dark, alpha) })
	}
}

// paneGlass is what a terminal's card shows in place of its background:
// the terminal's picture over the card's color, when it has one, else while
// the terminals are translucent, the window's background at the terminal
// opacity, so that 0% shows the desktop alone, as the window's 0% does. The
// card is not glass, which fogs what it covers and would leave a card
// opaque whatever its opacity. An opaque card, or one in a window that is
// not translucent, is the card's background. The picture's bitmap is nil
// while it comes, or when it is no picture.
func paneGlass(pal *palette, s settings.Settings, art *sidebarArt) ui.Material {
	if s.PaneImage != "" {
		m := &wash.PanePicture{Opacity: s.PaneImageOpacity, Fit: s.PaneImageFit, Base: pal.card}
		if art != nil {
			m.Bitmap = art.bm
			if art.card != nil {
				m.Base = *art.card
			}
		}
		return m
	}
	if !s.Vibrancy || s.PaneOpacity >= 1 {
		return nil
	}
	w := &wash.PaneWash{Opacity: s.PaneOpacity, Background: s.Background, Dark: pal.dark}
	if s.PaneBlur {
		if s.PaneMaterial == "plastic" {
			return wash.PanePlastic{Wash: w}
		}
		return wash.PaneFrost{Wash: w, Material: s.PaneMaterial}
	}
	return w
}

// translucentGlass makes surfaces that sit on the wash, the sidebar's rows
// and the cards, let it through, a tenth of what the wash lets through of
// the desktop, so that a thin wash still shows a faint surface: with
// translucency off they stay as they are.
func translucentGlass(c ui.Color, pal *palette, s settings.Settings) ui.Color {
	if !s.Vibrancy {
		return c
	}
	return c.Alpha(min(1, 1-0.2*washOpacity(s)))
}
