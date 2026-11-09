package main

import (
	"math"
	"sync"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

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
	base, glow := washes(s.Background, pal.dark)
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
		base.Stops = faded(base.Stops, alpha)
		glow.Stops = faded(glow.Stops, alpha)
	}
	c.Root().LinearGradient(base)
	row.LinearGradient(glow)
	if scene := washTable[s.Background].scene; scene != nil {
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
		m := &panePicture{opacity: s.PaneImageOpacity, fit: s.PaneImageFit, base: pal.card}
		if art != nil {
			m.bm = art.bm
			if art.card != nil {
				m.base = *art.card
			}
		}
		return m
	}
	if !s.Vibrancy || s.PaneOpacity >= 1 {
		return nil
	}
	w := &paneWash{opacity: s.PaneOpacity, background: s.Background, dark: pal.dark}
	if s.PaneBlur {
		if s.PaneMaterial == "plastic" {
			return panePlastic{wash: w}
		}
		return paneFrost{wash: w, material: s.PaneMaterial}
	}
	return w
}

// panePicture is a terminal's card showing the terminal's picture, pane
// image, in place of its background: the picture as fit says, as faint as
// the picture shows, over the card's color at the rest, so that the text
// stays legible. It leaves the glass out, as a window's own picture does:
// a frosted card under the picture would hide it.
type panePicture struct {
	bm      *ui.Bitmap
	opacity float32
	fit     string   // pane image fit: "fill", "fit", "stretch", "center" or "tile"
	base    ui.Color // the card's color under the picture
}

// PaintMaterial draws the picture as fit says, then the card's color over
// it, at the rest: as faint as the picture shows, and the text legible
// whatever the picture.
func (m *panePicture) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	p.Fill(box, m.base, radii[0])
	if nw, nh := ui.ImageSize(m.bm); m.bm != nil && m.opacity > 0 && nw > 0 && nh > 0 && box.W > 0 && box.H > 0 {
		m.paint(p, box, radii, nw, nh)
	}
	// The card's color over it, at the rest, as faint as the picture
	// shows: the text stays legible whatever the picture.
	p.Fill(box, m.base.Alpha(1-m.opacity), radii[0])
}

// paint draws the picture as fit says: "fill" as wide as the card, from
// its top; "fit" all of it within the card; "stretch" to its edges;
// "center" its own size at its middle; and "tile" its own size from there.
func (m *panePicture) paint(p *ui.Painter, box ui.Rect, radii [4]float32, nw, nh float32) {
	p.Clip(box, radii[0], func() {
		switch m.fit {
		case "stretch":
			p.ImageRegion(m.bm, ui.Rect{}, box, ui.ImageOptions{})
		case "fit":
			scale := min(box.W/nw, box.H/nh)
			picture(p, m.bm, ui.Rect{X: box.X + (box.W-nw*scale)/2, Y: box.Y + (box.H-nh*scale)/2, W: nw * scale, H: nh * scale}, box)
		case "center":
			picture(p, m.bm, ui.Rect{X: box.X + (box.W-nw)/2, Y: box.Y + (box.H-nh)/2, W: nw, H: nh}, box)
		case "tile":
			x0, y0 := box.X+(box.W-nw)/2, box.Y+(box.H-nh)/2
			for ty := y0 - float32(math.Ceil(float64((y0-box.Y)/nh)))*nh; ty < box.Y+box.H; ty += nh {
				for tx := x0 - float32(math.Ceil(float64((x0-box.X)/nw)))*nw; tx < box.X+box.W; tx += nw {
					picture(p, m.bm, ui.Rect{X: tx, Y: ty, W: nw, H: nh}, box)
				}
			}
		default: // "fill"
			picture(p, m.bm, ui.Rect{X: box.X, Y: box.Y, W: box.W, H: nh * box.W / nw}, box)
		}
	})
}

// picture draws the bitmap at place in the card, cropped by it, in bands
// of the card's height: the scene draws an image whole where its first
// pixels fall in the view, and no further than the view is large, so an
// image larger than the view would show its top stretched, where a band
// no larger than it shows where it lies.
func picture(p *ui.Painter, bm *ui.Bitmap, place, box ui.Rect) {
	left, top := max(place.X, box.X), max(place.Y, box.Y)
	right, bottom := min(place.X+place.W, box.X+box.W), min(place.Y+place.H, box.Y+box.H)
	if right <= left || bottom <= top {
		return
	}
	nw, nh := ui.ImageSize(bm)
	// The part of the picture that shows, in its pixels.
	src := ui.Rect{
		X: (left - place.X) / place.W * nw, Y: (top - place.Y) / place.H * nh,
		W: (right - left) / place.W * nw, H: (bottom - top) / place.H * nh,
	}
	// Eight rows of the card a band, at most.
	cardBands := 8
	lines := max(int(math.Ceil(float64(src.H/((bottom-top)/float32(cardBands))))), 1)
	band, high := src.H/float32(lines), (bottom-top)/float32(lines)
	for i := range lines {
		p.ImageRegion(bm, ui.Rect{X: src.X, Y: src.Y + float32(i)*band, W: src.W, H: band},
			ui.Rect{X: left, Y: top + float32(i)*high, W: right - left, H: high}, ui.ImageOptions{})
	}
}

// paneFrost is a terminal's card with the terminal blur on: the wash it
// paints, over the system's glass (macOS 26 and later), which frosts what
// shows through it as a card of the Dock's material. A system without
// Liquid Glass leaves the wash alone, as plain transparency.
type paneFrost struct {
	wash     *paneWash
	material string // "glass" or "glass-clear"
}

func (f paneFrost) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	f.wash.PaintMaterial(p, box, radii)
}

// NativeGlass places the system's glass under the card, which the wash,
// as translucent as the pane, shows through.
func (f paneFrost) NativeGlass() (ui.NativeGlassSpec, bool) {
	style := glass.Regular
	if f.material == "glass-clear" {
		style = glass.Clear
	}
	return ui.NativeGlassSpec{Style: int(style)}, true
}

// panePlastic is a terminal's card with the plastic material: the wash it
// paints, over a plain blur of what shows through, as a frosted content
// background. It draws no system glass; the renderer's blur looks the same
// on every platform, and a system without Liquid Glass keeps the frost.
type panePlastic struct {
	wash *paneWash
}

func (p panePlastic) PaintMaterial(painter *ui.Painter, box ui.Rect, radii [4]float32) {
	glass.Blur{Radius: 18}.PaintMaterial(painter, box, radii)
	p.wash.PaintMaterial(painter, box, radii)
}

// paneWash paints a card as backdrop paints the window: the background's
// gradient, then its glow from the top right over it, at the terminal
// opacity. Nothing else covers what shows through it, so that at 0% the
// desktop alone shows, as the window's 0% shows it.
type paneWash struct {
	opacity    float32
	background string
	dark       bool
}

// PaintMaterial paints the wash over box, rounded by radii.
func (w *paneWash) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if box.W <= 0 || box.H <= 0 {
		return
	}
	base, glow := washes(w.background, w.dark)
	if a := w.opacity; a > 0 {
		// Copies, as the wash the theme has stays as it is.
		base.Stops = faded(base.Stops, a)
		glow.Stops = faded(glow.Stops, a)
	} else {
		// A wash of none: the gradient's stops at 0.5, which a gradient of
		// one shows as one color throughout.
		base.Stops = []ui.GradientStop{{Color: ui.Color{A: 0}, At: 0.5}}
		glow.Stops = []ui.GradientStop{{Color: ui.Color{A: 0}, At: 0.5}}
	}
	p.FillGradient(box, base, radii[0])
	// The glow covers the card and as much again, and fades at 0.5 of that.
	p.FillGradient(ui.Rect{X: box.X, Y: box.Y, W: box.W, H: box.H * 2}, glow, 0)
}

// faded returns stops as another frame draws them: at opacity a, their
// colors' alphas times it.
func faded(stops []ui.GradientStop, a float32) []ui.GradientStop {
	out := make([]ui.GradientStop, len(stops))
	for i, st := range stops {
		out[i] = ui.GradientStop{Color: st.Color.Alpha(a), At: st.At}
	}
	return out
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

// A background's base and glow, made once for each background and theme,
// as the theme has them: a frame asks for them each time, and what a frame
// makes of its own is garbage at the next frame.
var (
	// washes returns a background's base and glow, as backdrop and a card
	// ask for them.
	washes = washOf
	// washOf returns a background's base and glow, made.
	washOf = memoWash(func(id string, dark bool) (base, glow ui.LinearGradient) {
		w, ok := washTable[id]
		if !ok {
			w = washTable["rex"]
		}
		glowOf := func(c ui.Color) ui.LinearGradient {
			return ui.LinearGradient{Angle: 225, Oklab: true, Stops: []ui.GradientStop{{Color: c, At: 0}, {Color: c.Alpha(0), At: 0.5}}}
		}
		angle := w.lightAngle
		if angle == 0 {
			angle = 180
		}
		if dark {
			if w.darkAngle != 0 {
				angle = w.darkAngle
			}
			return ui.LinearGradient{Angle: angle, Oklab: true, Stops: stopsOf(w.dark...)}, glowOf(w.darkGlow)
		}
		return ui.LinearGradient{Angle: angle, Oklab: true, Stops: stopsOf(w.light...)}, glowOf(w.lightGlow)
	})
)

// memoWash keeps what make returns for each background and theme, made
// once: a theme's colors are set as the program starts, and do not change
// under it.
func memoWash(make func(id string, dark bool) (base, glow ui.LinearGradient)) func(id string, dark bool) (base, glow ui.LinearGradient) {
	type key struct {
		id   string
		dark bool
	}
	var mu sync.Mutex
	kept := map[key][2]ui.LinearGradient{}
	return func(id string, dark bool) (base, glow ui.LinearGradient) {
		k := key{id, dark}
		mu.Lock()
		defer mu.Unlock()
		if g, ok := kept[k]; ok {
			return g[0], g[1]
		}
		base, glow = make(id, dark)
		kept[k] = [2]ui.LinearGradient{base, glow}
		return base, glow
	}
}

// stopsOf puts colors at even places along a gradient, the first at 0
// and the last at 1.
func stopsOf(hexes ...string) []ui.GradientStop {
	out := make([]ui.GradientStop, len(hexes))
	for i, h := range hexes {
		out[i] = ui.GradientStop{Color: ui.Hex(h), At: float32(i) / float32(len(hexes)-1)}
	}
	return out
}

// wash is a background's colors: its base's stops, top to bottom unless
// an angle turns them, and the color of its glow from the top right.
type wash struct {
	light, dark         []string
	lightGlow, darkGlow ui.Color
	// lightAngle and darkAngle turn the base, in degrees clockwise from
	// upwards, where a wash runs otherwise than top to bottom: 0 is down.
	lightAngle, darkAngle float32
	// scene draws shapes on the base and glow, under the cards: a
	// treeline, the moon, a teletype's ruling. nil is none.
	scene func(p *ui.Painter, r ui.Rect, dark bool, alpha float32)
}

var washTable = map[string]wash{
	// Rex's: pink lavender above, rose at the top right, butter cream below.
	"rex": {
		light: []string{"#ebe0f3", "#f3e7f1", "#f7eee4", "#f6efcf"}, lightGlow: ui.RGBA(247, 220, 241, 0.75),
		dark: []string{"#17141f", "#191620", "#1b1916"}, darkGlow: ui.RGBA(120, 70, 120, 0.22),
	},
	"mist": {
		light: []string{"#e3e9f4", "#ebeff6", "#f1f2f3", "#f2f1ec"}, lightGlow: ui.RGBA(210, 225, 248, 0.75),
		dark: []string{"#12151b", "#14171d", "#17181b"}, darkGlow: ui.RGBA(70, 100, 150, 0.22),
	},
	"meadow": {
		light: []string{"#e2eee5", "#ebf2e7", "#f3f2e2", "#f5f0d6"}, lightGlow: ui.RGBA(214, 240, 222, 0.75),
		dark: []string{"#111814", "#141a16", "#181a14"}, darkGlow: ui.RGBA(70, 130, 95, 0.2),
	},
	"plain": {
		light: []string{"#eceef4", "#eceef4"}, lightGlow: ui.RGBA(0, 0, 0, 0),
		dark: []string{"#141418", "#141418"}, darkGlow: ui.RGBA(0, 0, 0, 0),
	},
	// Ember's: a fire at the bottom left, dark brick at the top right.
	"ember": {
		light: []string{"#3a1a1c", "#5c2623", "#8a3d2c", "#c4703a", "#ecab63"}, lightGlow: ui.RGBA(255, 150, 90, 0.35),
		dark: []string{"#100708", "#1a0c0b", "#2a1310", "#3d1c15", "#57301f"}, darkGlow: ui.RGBA(255, 120, 60, 0.18),
	},
	// Tidal's: deep water below, a pale band at the horizon, night above,
	// turned so that it runs from the top left corner down.
	"tidal": {
		light: []string{"#1f4050", "#2f6272", "#6b9aa4", "#d7c9a8", "#efe3c8"}, lightGlow: ui.RGBA(255, 236, 200, 0.5), lightAngle: 135,
		dark: []string{"#070f16", "#0d1c26", "#163440", "#234b56", "#2e5b63"}, darkGlow: ui.RGBA(140, 200, 210, 0.16), darkAngle: 135,
	},
	// Umbra's: deep indigo with violet at the upper right and its dark
	// under it; the moon, faint, at the lower left.
	"umbra": {
		light: []string{"#3b3a6b", "#504a7d", "#6b5f8e", "#8a7a9e", "#a99bb0"}, lightGlow: ui.RGBA(190, 170, 255, 0.45),
		dark: []string{"#08080f", "#0d0c18", "#131126", "#191632", "#1f1a3b"}, darkGlow: ui.RGBA(120, 100, 220, 0.2),
		scene: umbraScene,
	},
	// Static's: a field of teal, turned, and the faint lines of a
	// teletype's ruling across it.
	"static": {
		light: []string{"#dfe6e4", "#e9eeeb", "#f1f3ee", "#f5f4ea"}, lightGlow: ui.RGBA(160, 220, 200, 0.5), lightAngle: 160,
		dark: []string{"#0c1413", "#101a18", "#141f1c", "#17251f"}, darkGlow: ui.RGBA(80, 200, 170, 0.14), darkAngle: 160,
		scene: staticScene,
	},
	// The Dock's: the desktop behind the window, frosted, under a thin
	// neutral veil that grounds what sits on it, in the Dock's gray.
	"dock": {
		light: []string{"#f3f4f6", "#eeeff2", "#f6f6f7"}, lightGlow: ui.RGBA(0, 0, 0, 0),
		dark: []string{"#131315", "#161618", "#19191b"}, darkGlow: ui.RGBA(0, 0, 0, 0),
	},
}

// umbraScene draws Umbra's moon: a soft disc at the lower left, faded at
// its edge, and the faint ring around it.
func umbraScene(p *ui.Painter, r ui.Rect, dark bool, alpha float32) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	cx, cy := r.X+r.W*0.22, r.Y+r.H*0.72
	rad := min(r.W, r.H) * 0.09
	core := ui.Oklch(0.86, 0.03, 85)
	if dark {
		core = ui.Oklch(0.93, 0.02, 85)
	}
	p.FillRadialGradient(ui.Rect{X: cx - rad, Y: cy - rad, W: 2 * rad, H: 2 * rad}, ui.RadialGradient{
		Stops: []ui.GradientStop{{Color: core.Alpha(0.55 * alpha), At: 0.72}, {Color: core.Alpha(0), At: 1}},
	}, rad*0.999)
	ring := (&ui.Path{}).MoveTo(cx+rad*1.5, cy)
	for i := 1; i <= 64; i++ {
		a := float64(i) / 64 * 2 * math.Pi
		ring.LineTo(cx+rad*1.5*float32(math.Cos(a)), cy+rad*1.5*float32(math.Sin(a)))
	}
	p.StrokePath(ring, 1, core.Alpha(0.12*alpha))
}

// staticScene draws Static's teletype ruling: faint horizontal lines a
// little apart, and a column of sprocket holes down the left, as a
// printer's paper has.
func staticScene(p *ui.Painter, r ui.Rect, dark bool, alpha float32) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	ink := ui.Oklch(0.25, 0.02, 180)
	if dark {
		ink = ui.Oklch(0.85, 0.03, 170)
	}
	const step = 18
	for y := r.Y + step; y < r.Y+r.H; y += step {
		p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: 1}, ink.Alpha(0.035*alpha), 0)
	}
	for y := r.Y + 9; y < r.Y+r.H; y += step * 2 {
		p.Fill(ui.Rect{X: r.X + 22, Y: y, W: 5, H: 5}, ink.Alpha(0.06*alpha), 1)
	}
}
