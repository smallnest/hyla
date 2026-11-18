package theme

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
	"hyla/internal/ui/wash"
)

// ParseColor parses hex (#rgb, #rrggbb, #rrggbbaa), rgb(), rgba(), and
// transparent color literals into a ui.Color.
func ParseColor(raw string) (ui.Color, bool) {
	return wash.ParseColor(raw)
}

// WashOpacity is how much of the window's background the wash covers
// while the material shows: the setting's, else the default.
func WashOpacity(s settings.Settings) float32 {
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
func Backdrop(c *ui.Context, pal *Palette, row ui.Element, s settings.Settings) {
	base, glow := wash.Washes(s.Background, pal.Dark)
	if s.Background == "plain" {
		// The material alone is as light or as dark as the wallpaper
		// under it: a base of the sidebar's color keeps the text legible,
		// letting the desktop show a little.
		bg := pal.Sidebar
		if s.Vibrancy {
			bg = bg.Alpha(WashOpacity(s))
		}
		c.Root().Background(bg)
		return
	}
	alpha := float32(1)
	if s.Vibrancy {
		// The wash is translucent over the material, or over the desktop
		// when the window is blurless: its glow too, as at 0% none of it
		// shows.
		alpha = WashOpacity(s)
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
		row.Draw(func(p *ui.Painter, r ui.Rect) { scene(p, r, pal.Dark, alpha) })
	}
}

// PaneGlass is what a terminal's card shows in place of its background:
// the terminal's picture over the card's color, when it has one, else while
// the terminals are translucent, the window's background at the terminal
// opacity, so that 0% shows the desktop alone, as the window's 0% does. The
// card is not glass, which fogs what it covers and would leave a card
// opaque whatever its opacity. An opaque card, or one in a window that is
// not translucent, is the card's background. The picture's bitmap is nil
// while it comes, or when it is no picture.
func PaneGlass(pal *Palette, s settings.Settings, bm *ui.Bitmap, card *ui.Color) ui.Material {
	if s.PaneImage != "" {
		m := &wash.PanePicture{Opacity: s.PaneImageOpacity, Fit: s.PaneImageFit, Base: pal.Card}
		m.Bitmap = bm
		if card != nil {
			m.Base = *card
		}
		return m
	}
	if !s.Vibrancy || s.PaneOpacity >= 1 {
		return nil
	}
	w := &wash.PaneWash{Opacity: s.PaneOpacity, Background: s.Background, Dark: pal.Dark}
	if s.PaneBlur {
		if s.PaneMaterial == "plastic" {
			return wash.PanePlastic{Wash: w}
		}
		return wash.PaneFrost{Wash: w, Material: s.PaneMaterial}
	}
	return w
}

// TranslucentGlass makes surfaces that sit on the wash, the sidebar's rows
// and the cards, let it through, a tenth of what the wash lets through of
// the desktop, so that a thin wash still shows a faint surface: with
// translucency off they stay as they are.
func TranslucentGlass(c ui.Color, pal *Palette, s settings.Settings) ui.Color {
	if !s.Vibrancy {
		return c
	}
	return c.Alpha(min(1, 1-0.2*WashOpacity(s)))
}
