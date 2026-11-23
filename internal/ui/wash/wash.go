// Package wash paints the wash: the color under Hyla's window and its
// cards, a background's gradient and the glow from its top right, and the
// shapes a scene draws under the cards, as the Settings window shows them.
package wash

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/manifest"
)

// PanePicture is a terminal's card showing the terminal's picture, pane
// image, in place of its background: the picture as fit says, as faint as
// the picture shows, over the card's color at the rest, so that the text
// stays legible. It leaves the glass out, as a window's own picture does:
// a frosted card under the picture would hide it.
type PanePicture struct {
	Bitmap  *ui.Bitmap
	Opacity float32
	Fit     string   // pane image fit: "fill", "fit", "stretch", "center" or "tile"
	Base    ui.Color // the card's color under the picture
}

// PaintMaterial draws the picture as fit says, then the card's color over
// it, at the rest: as faint as the picture shows, and the text legible
// whatever the picture.
func (m *PanePicture) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	p.Fill(box, m.Base, radii[0])
	if nw, nh := ui.ImageSize(m.Bitmap); m.Bitmap != nil && m.Opacity > 0 && nw > 0 && nh > 0 && box.W > 0 && box.H > 0 {
		m.paint(p, box, radii, nw, nh)
	}
	// The card's color over it, at the rest, as faint as the picture
	// shows: the text stays legible whatever the picture.
	p.Fill(box, m.Base.Alpha(1-m.Opacity), radii[0])
}

// paint draws the picture as fit says: "fill" as wide as the card, from
// its top; "fit" all of it within the card; "stretch" to its edges;
// "center" its own size at its middle; and "tile" its own size from there.
func (m *PanePicture) paint(p *ui.Painter, box ui.Rect, radii [4]float32, nw, nh float32) {
	p.Clip(box, radii[0], func() {
		switch m.Fit {
		case "stretch":
			p.ImageRegion(m.Bitmap, ui.Rect{}, box, ui.ImageOptions{})
		case "fit":
			scale := min(box.W/nw, box.H/nh)
			picture(p, m.Bitmap, ui.Rect{X: box.X + (box.W-nw*scale)/2, Y: box.Y + (box.H-nh*scale)/2, W: nw * scale, H: nh * scale}, box)
		case "center":
			picture(p, m.Bitmap, ui.Rect{X: box.X + (box.W-nw)/2, Y: box.Y + (box.H-nh)/2, W: nw, H: nh}, box)
		case "tile":
			x0, y0 := box.X+(box.W-nw)/2, box.Y+(box.H-nh)/2
			for ty := y0 - float32(math.Ceil(float64((y0-box.Y)/nh)))*nh; ty < box.Y+box.H; ty += nh {
				for tx := x0 - float32(math.Ceil(float64((x0-box.X)/nw)))*nw; tx < box.X+box.W; tx += nw {
					picture(p, m.Bitmap, ui.Rect{X: tx, Y: ty, W: nw, H: nh}, box)
				}
			}
		default: // "fill"
			picture(p, m.Bitmap, ui.Rect{X: box.X, Y: box.Y, W: box.W, H: nh * box.W / nw}, box)
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

// PaneFrost is a terminal's card with the terminal blur on: the wash it
// paints, over the system's glass (macOS 26 and later), which frosts what
// shows through it as a card of the Dock's material. A system without
// Liquid Glass leaves the wash alone, as plain transparency.
type PaneFrost struct {
	Wash     *PaneWash
	Material string // "glass" or "glass-clear"
}

func (f PaneFrost) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	f.Wash.PaintMaterial(p, box, radii)
}

// NativeGlass places the system's glass under the card, which the wash,
// as translucent as the pane, shows through.
func (f PaneFrost) NativeGlass() (ui.NativeGlassSpec, bool) {
	style := glass.Regular
	if f.Material == "glass-clear" {
		style = glass.Clear
	}
	return ui.NativeGlassSpec{Style: int(style)}, true
}

// PanePlastic is a terminal's card with the plastic material: the wash it
// paints, over a plain blur of what shows through, as a frosted content
// background. It draws no system glass; the renderer's blur looks the same
// on every platform, and a system without Liquid Glass keeps the frost.
type PanePlastic struct {
	Wash *PaneWash
}

func (p PanePlastic) PaintMaterial(painter *ui.Painter, box ui.Rect, radii [4]float32) {
	glass.Blur{Radius: 18}.PaintMaterial(painter, box, radii)
	p.Wash.PaintMaterial(painter, box, radii)
}

// PaneWash paints a card as backdrop paints the window: the background's
// gradient, then its glow from the top right over it, at the terminal
// opacity. Nothing else covers what shows through it, so that at 0% the
// desktop alone shows, as the window's 0% shows it.
type PaneWash struct {
	Opacity    float32
	Background string
	Dark       bool
}

// PaintMaterial paints the wash over box, rounded by radii.
func (w *PaneWash) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if box.W <= 0 || box.H <= 0 {
		return
	}
	base, glow := Washes(w.Background, w.Dark)
	if a := w.Opacity; a > 0 {
		// Copies, as the wash the theme has stays as it is.
		base.Stops = Faded(base.Stops, a)
		glow.Stops = Faded(glow.Stops, a)
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

// Faded returns stops as another frame draws them: at opacity a, their
// colors' alphas times it.
func Faded(stops []ui.GradientStop, a float32) []ui.GradientStop {
	out := make([]ui.GradientStop, len(stops))
	for i, st := range stops {
		out[i] = ui.GradientStop{Color: st.Color.Alpha(a), At: st.At}
	}
	return out
}

// A background's base and glow, made once for each background and theme,
// as the theme has them: a frame asks for them each time, and what a frame
// makes of its own is garbage at the next frame.
// PluginWashEntry describes an enabled plugin background wash for settings/UI.
type PluginWashEntry struct {
	PluginID string
	ID       string
	Name     string
	Wash     Wash
}

var (
	pluginMu    sync.RWMutex
	pluginTable = map[string]PluginWashEntry{}
	pluginList  []PluginWashEntry
	washEpoch   uint64

	// Washes returns a background's base and glow, as backdrop and a card
	// ask for them.
	Washes = washOf
	// washOf returns a background's base and glow, made once per reload epoch.
	washOf = func() func(id string, dark bool) (base, glow ui.LinearGradient) {
		type key struct {
			epoch uint64
			id    string
			dark  bool
		}
		var mu sync.Mutex
		kept := map[key][2]ui.LinearGradient{}
		return func(id string, dark bool) (base, glow ui.LinearGradient) {
			pluginMu.RLock()
			epoch := washEpoch
			w, ok := lookupWashLocked(id)
			pluginMu.RUnlock()
			k := key{epoch: epoch, id: id, dark: dark}
			mu.Lock()
			if g, hit := kept[k]; hit {
				mu.Unlock()
				return g[0], g[1]
			}
			mu.Unlock()
			if !ok {
				w = Table["rex"]
			}
			glowOf := func(c ui.Color) ui.LinearGradient {
				return ui.LinearGradient{Angle: 225, Oklab: true, Stops: []ui.GradientStop{{Color: c, At: 0}, {Color: c.Alpha(0), At: 0.5}}}
			}
			angle := w.LightAngle
			if angle == 0 {
				angle = 180
			}
			if dark {
				if w.DarkAngle != 0 {
					angle = w.DarkAngle
				}
				base, glow = ui.LinearGradient{Angle: angle, Oklab: true, Stops: Stops(w.Dark...)}, glowOf(w.DarkGlow)
			} else {
				base, glow = ui.LinearGradient{Angle: angle, Oklab: true, Stops: Stops(w.Light...)}, glowOf(w.LightGlow)
			}
			mu.Lock()
			if len(kept) > 256 {
				clear(kept)
			}
			kept[k] = [2]ui.LinearGradient{base, glow}
			mu.Unlock()
			return base, glow
		}
	}()
)

func lookupWashLocked(id string) (Wash, bool) {
	if w, ok := Table[id]; ok {
		return w, true
	}
	if e, ok := pluginTable[id]; ok {
		return e.Wash, true
	}
	return Wash{}, false
}

// Known reports whether id names either a built-in wash or an active plugin wash.
func Known(id string) bool {
	if _, ok := Table[id]; ok {
		return true
	}
	pluginMu.RLock()
	_, ok := pluginTable[id]
	pluginMu.RUnlock()
	return ok
}

// PluginList returns active plugin washes sorted by ID.
func PluginList() []PluginWashEntry {
	pluginMu.RLock()
	defer pluginMu.RUnlock()
	out := make([]PluginWashEntry, len(pluginList))
	copy(out, pluginList)
	return out
}

type washTOML struct {
	ID         string   `toml:"id"`
	Name       string   `toml:"name"`
	Light      []string `toml:"light"`
	LightGlow  string   `toml:"light_glow"`
	LightAngle float32  `toml:"light_angle"`
	Dark       []string `toml:"dark"`
	DarkGlow   string   `toml:"dark_glow"`
	DarkAngle  float32  `toml:"dark_angle"`
}

// ReloadPlugins parses plugin washes/*.toml and swaps active plugin washes atomically.
func ReloadPlugins(assets []manifest.WashAsset) error {
	nextMap := map[string]PluginWashEntry{}
	var errs []error
	for _, a := range assets {
		if a.ID == "" || a.Path == "" {
			continue
		}
		if _, isBuiltin := Table[a.ID]; isBuiltin {
			errs = append(errs, fmt.Errorf("wash %q from plugin %s conflicts with built-in wash", a.ID, a.PluginID))
			continue
		}
		data, err := os.ReadFile(a.Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("wash %s/%s: %w", a.PluginID, a.ID, err))
			continue
		}
		var doc washTOML
		if _, err := toml.Decode(string(data), &doc); err != nil {
			errs = append(errs, fmt.Errorf("wash %s/%s: %w", a.PluginID, a.ID, err))
			continue
		}
		if len(doc.Light) < 2 || len(doc.Dark) < 2 {
			errs = append(errs, fmt.Errorf("wash %s/%s: light and dark must each define at least 2 color stops", a.PluginID, a.ID))
			continue
		}
		lg, err := parseWashColor(doc.LightGlow)
		if err != nil {
			errs = append(errs, fmt.Errorf("wash %s/%s light_glow: %w", a.PluginID, a.ID, err))
			continue
		}
		dg, err := parseWashColor(doc.DarkGlow)
		if err != nil {
			errs = append(errs, fmt.Errorf("wash %s/%s dark_glow: %w", a.PluginID, a.ID, err))
			continue
		}
		name := a.Name
		if name == "" {
			name = doc.Name
		}
		if name == "" {
			name = a.ID
		}
		nextMap[a.ID] = PluginWashEntry{
			PluginID: a.PluginID,
			ID:       a.ID,
			Name:     name,
			Wash: Wash{
				Light:      doc.Light,
				Dark:       doc.Dark,
				LightGlow:  lg,
				DarkGlow:   dg,
				LightAngle: doc.LightAngle,
				DarkAngle:  doc.DarkAngle,
			},
		}
	}
	nextList := make([]PluginWashEntry, 0, len(nextMap))
	for _, e := range nextMap {
		nextList = append(nextList, e)
	}
	sort.Slice(nextList, func(i, j int) bool { return nextList[i].ID < nextList[j].ID })

	pluginMu.Lock()
	pluginTable = nextMap
	pluginList = nextList
	washEpoch++
	pluginMu.Unlock()
	return errors.Join(errs...)
}

func parseWashColor(raw string) (ui.Color, error) {
	if strings.TrimSpace(raw) == "" {
		return ui.RGBA(0, 0, 0, 0), nil
	}
	c, ok := ParseColor(raw)
	if !ok {
		return ui.Color{}, fmt.Errorf("unsupported color %q", raw)
	}
	return c, nil
}

// ParseColor parses "none", "transparent", "#rgb", "#rrggbb", "#rrggbbaa",
// "rgb(r,g,b)", and "rgba(r,g,b,a)".
func ParseColor(raw string) (ui.Color, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ui.Color{}, false
	}
	if s == "none" || s == "transparent" {
		return ui.RGBA(0, 0, 0, 0), true
	}
	if strings.HasPrefix(s, "#") {
		hex := s[1:]
		switch len(hex) {
		case 3:
			r, err1 := strconv.ParseUint(strings.Repeat(hex[0:1], 2), 16, 8)
			g, err2 := strconv.ParseUint(strings.Repeat(hex[1:2], 2), 16, 8)
			b, err3 := strconv.ParseUint(strings.Repeat(hex[2:3], 2), 16, 8)
			if err1 != nil || err2 != nil || err3 != nil {
				return ui.Color{}, false
			}
			return ui.RGBA(uint8(r), uint8(g), uint8(b), 1), true
		case 6:
			for i := 0; i < 6; i++ {
				c := hex[i]
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					return ui.Color{}, false
				}
			}
			return ui.Hex(s), true
		case 8:
			r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
			g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
			b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
			a, err4 := strconv.ParseUint(hex[6:8], 16, 8)
			if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
				return ui.Color{}, false
			}
			return ui.RGBA(uint8(r), uint8(g), uint8(b), float32(a)/255), true
		default:
			return ui.Color{}, false
		}
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "rgba(") && strings.HasSuffix(low, ")") {
		parts := strings.Split(low[5:len(low)-1], ",")
		if len(parts) != 4 {
			return ui.Color{}, false
		}
		r, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		g, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		b, e3 := strconv.Atoi(strings.TrimSpace(parts[2]))
		a, e4 := strconv.ParseFloat(strings.TrimSpace(parts[3]), 32)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
			return ui.Color{}, false
		}
		return ui.RGBA(uint8(r), uint8(g), uint8(b), float32(max(0, min(1, a)))), true
	}
	if strings.HasPrefix(low, "rgb(") && strings.HasSuffix(low, ")") {
		parts := strings.Split(low[4:len(low)-1], ",")
		if len(parts) != 3 {
			return ui.Color{}, false
		}
		r, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		g, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		b, e3 := strconv.Atoi(strings.TrimSpace(parts[2]))
		if e1 != nil || e2 != nil || e3 != nil || r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
			return ui.Color{}, false
		}
		return ui.RGBA(uint8(r), uint8(g), uint8(b), 1), true
	}
	return ui.Color{}, false
}

// Stops puts colors at even places along a gradient, the first at 0
// and the last at 1.
func Stops(hexes ...string) []ui.GradientStop {
	out := make([]ui.GradientStop, len(hexes))
	for i, h := range hexes {
		out[i] = ui.GradientStop{Color: ui.Hex(h), At: float32(i) / float32(len(hexes)-1)}
	}
	return out
}

// Wash is a background's colors: its base's stops, top to bottom unless
// an angle turns them, and the color of its glow from the top right.
type Wash struct {
	Light, Dark         []string
	LightGlow, DarkGlow ui.Color
	// LightAngle and DarkAngle turn the base, in degrees clockwise from
	// upwards, where a wash runs otherwise than top to bottom: 0 is down.
	LightAngle, DarkAngle float32
	// scene draws shapes on the base and glow, under the cards: a
	// treeline, the moon, a teletype's ruling. nil is none.
	Scene func(p *ui.Painter, r ui.Rect, dark bool, alpha float32)
}

var Table = map[string]Wash{
	// Rex's: pink lavender above, rose at the top right, butter cream below.
	"rex": {
		Light: []string{"#ebe0f3", "#f3e7f1", "#f7eee4", "#f6efcf"}, LightGlow: ui.RGBA(247, 220, 241, 0.75),
		Dark: []string{"#17141f", "#191620", "#1b1916"}, DarkGlow: ui.RGBA(120, 70, 120, 0.22),
	},
	"mist": {
		Light: []string{"#e3e9f4", "#ebeff6", "#f1f2f3", "#f2f1ec"}, LightGlow: ui.RGBA(210, 225, 248, 0.75),
		Dark: []string{"#12151b", "#14171d", "#17181b"}, DarkGlow: ui.RGBA(70, 100, 150, 0.22),
	},
	"meadow": {
		Light: []string{"#e2eee5", "#ebf2e7", "#f3f2e2", "#f5f0d6"}, LightGlow: ui.RGBA(214, 240, 222, 0.75),
		Dark: []string{"#111814", "#141a16", "#181a14"}, DarkGlow: ui.RGBA(70, 130, 95, 0.2),
	},
	"plain": {
		Light: []string{"#eceef4", "#eceef4"}, LightGlow: ui.RGBA(0, 0, 0, 0),
		Dark: []string{"#141418", "#141418"}, DarkGlow: ui.RGBA(0, 0, 0, 0),
	},
	// Ember's: a fire at the bottom left, dark brick at the top right.
	"ember": {
		Light: []string{"#3a1a1c", "#5c2623", "#8a3d2c", "#c4703a", "#ecab63"}, LightGlow: ui.RGBA(255, 150, 90, 0.35),
		Dark: []string{"#100708", "#1a0c0b", "#2a1310", "#3d1c15", "#57301f"}, DarkGlow: ui.RGBA(255, 120, 60, 0.18),
	},
	// Tidal's: deep water below, a pale band at the horizon, night above,
	// turned so that it runs from the top left corner down.
	"tidal": {
		Light: []string{"#1f4050", "#2f6272", "#6b9aa4", "#d7c9a8", "#efe3c8"}, LightGlow: ui.RGBA(255, 236, 200, 0.5), LightAngle: 135,
		Dark: []string{"#070f16", "#0d1c26", "#163440", "#234b56", "#2e5b63"}, DarkGlow: ui.RGBA(140, 200, 210, 0.16), DarkAngle: 135,
	},
	// Umbra's: deep indigo with violet at the upper right and its dark
	// under it; the moon, faint, at the lower left.
	"umbra": {
		Light: []string{"#3b3a6b", "#504a7d", "#6b5f8e", "#8a7a9e", "#a99bb0"}, LightGlow: ui.RGBA(190, 170, 255, 0.45),
		Dark: []string{"#08080f", "#0d0c18", "#131126", "#191632", "#1f1a3b"}, DarkGlow: ui.RGBA(120, 100, 220, 0.2),
		Scene: umbraScene,
	},
	// Static's: a field of teal, turned, and the faint lines of a
	// teletype's ruling across it.
	"static": {
		Light: []string{"#dfe6e4", "#e9eeeb", "#f1f3ee", "#f5f4ea"}, LightGlow: ui.RGBA(160, 220, 200, 0.5), LightAngle: 160,
		Dark: []string{"#0c1413", "#101a18", "#141f1c", "#17251f"}, DarkGlow: ui.RGBA(80, 200, 170, 0.14), DarkAngle: 160,
		Scene: staticScene,
	},
	// Indigo's: the royal blue of the screenshot that asked for it, as a
	// wash of the dusk: violet at the top, its own blue at the foot.
	"indigo": {
		Light: []string{"#dcdff7", "#c9cdf5", "#a8b0ea", "#7e8cdc"}, LightGlow: ui.RGBA(86, 103, 243, 0.4),
		Dark: []string{"#10142c", "#141a3a", "#1a2150", "#232d6b"}, DarkGlow: ui.RGBA(86, 103, 243, 0.22),
	},
	// The Dock's: the desktop behind the window, frosted, under a thin
	// neutral veil that grounds what sits on it, in the Dock's gray.
	"dock": {
		Light: []string{"#f3f4f6", "#eeeff2", "#f6f6f7"}, LightGlow: ui.RGBA(0, 0, 0, 0),
		Dark: []string{"#131315", "#161618", "#19191b"}, DarkGlow: ui.RGBA(0, 0, 0, 0),
	},
}

// umbraScene draws Umbra's moon: a soft disc at the lower left, Faded at
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
