// Package uihost renders Layer 3A Declarative UI schemas natively into mygo/ui,
// coordinates async non-blocking plugin UI slots and actions, and hosts
// Layer 3B sandboxed hylaplugin:// WebViews.
package uihost

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/settings/kit"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
)

// EventCallback is invoked asynchronously off interactive widget actions.
type EventCallback func(componentID, eventType string, value any)

type cachedAsset struct {
	svg *ui.SVG
	bmp *ui.Bitmap
}

// RenderState stores ephemeral UI widget bindings across rebuilds (never ui.Element references).
type RenderState struct {
	mu             sync.Mutex
	bools          map[string]*bool
	lastBoolProps  map[string]bool
	strings        map[string]*string
	lastStrProps   map[string]string
	floats         map[string]*float64
	lastFloatProps map[string]float64
	images         map[string]cachedAsset
}

func NewRenderState() *RenderState {
	return &RenderState{
		bools:          make(map[string]*bool),
		lastBoolProps:  make(map[string]bool),
		strings:        make(map[string]*string),
		lastStrProps:   make(map[string]string),
		floats:         make(map[string]*float64),
		lastFloatProps: make(map[string]float64),
		images:         make(map[string]cachedAsset),
	}
}

func (rs *RenderState) ClearAssets() {
	rs.mu.Lock()
	clear(rs.images)
	rs.mu.Unlock()
}

func (rs *RenderState) getBool(key string, initial bool) *bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	p, ok := rs.bools[key]
	if !ok {
		v := initial
		p = &v
		rs.bools[key] = p
	}
	return p
}

func (rs *RenderState) syncBoolProp(key string, propVal bool) *bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.lastBoolProps == nil {
		rs.lastBoolProps = make(map[string]bool)
	}
	p, ok := rs.bools[key]
	if !ok {
		v := propVal
		p = &v
		rs.bools[key] = p
		rs.lastBoolProps[key] = propVal
		return p
	}
	if rs.lastBoolProps[key] != propVal {
		*p = propVal
		rs.lastBoolProps[key] = propVal
	}
	return p
}

func (rs *RenderState) syncStringProp(key, propVal string) *string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.lastStrProps == nil {
		rs.lastStrProps = make(map[string]string)
	}
	p, ok := rs.strings[key]
	if !ok {
		v := propVal
		p = &v
		rs.strings[key] = p
		rs.lastStrProps[key] = propVal
		return p
	}
	if rs.lastStrProps[key] != propVal {
		*p = propVal
		rs.lastStrProps[key] = propVal
	}
	return p
}

func (rs *RenderState) syncFloatProp(key string, propVal float64) *float64 {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.lastFloatProps == nil {
		rs.lastFloatProps = make(map[string]float64)
	}
	p, ok := rs.floats[key]
	if !ok {
		v := propVal
		p = &v
		rs.floats[key] = p
		rs.lastFloatProps[key] = propVal
		return p
	}
	if rs.lastFloatProps[key] != propVal {
		*p = propVal
		rs.lastFloatProps[key] = propVal
	}
	return p
}

func (rs *RenderState) loadImage(pluginDir, rel string) (svg *ui.SVG, bmp *ui.Bitmap) {
	if pluginDir == "" || rel == "" {
		return nil, nil
	}
	cacheKey := pluginDir + "\x00" + rel
	rs.mu.Lock()
	if cached, ok := rs.images[cacheKey]; ok {
		rs.mu.Unlock()
		return cached.svg, cached.bmp
	}
	rs.mu.Unlock()

	var asset cachedAsset
	if fullPath, err := manifest.ResolveWithin(pluginDir, rel); err == nil {
		if data, rerr := os.ReadFile(fullPath); rerr == nil {
			if strings.HasSuffix(strings.ToLower(fullPath), ".svg") {
				if parsed, perr := ui.ParseSVG(data); perr == nil {
					asset.svg = parsed
				}
			} else if decoded, derr := ui.DecodeImage(data); derr == nil {
				if b, ok := decoded.(*ui.Bitmap); ok {
					asset.bmp = b
				}
			}
		}
	}
	rs.mu.Lock()
	if rs.images == nil {
		rs.images = make(map[string]cachedAsset)
	}
	rs.images[cacheKey] = asset
	rs.mu.Unlock()
	return asset.svg, asset.bmp
}

// RenderContext holds scoped metadata for rendering a declarative UINode tree.
type RenderContext struct {
	PluginID       string
	SlotID         string
	PluginDir      string
	Palette        *theme.Palette
	State          *RenderState
	SettingsSearch bool
	OnEvent        EventCallback
}

// Render builds native mygo/ui elements from root using stable c.Key identities.
func Render(c *ui.Context, rc RenderContext, root *rpc.UINode) {
	if root == nil {
		return
	}
	if rc.Palette == nil {
		rc.Palette = theme.For(c)
	}
	if rc.State == nil {
		rc.State = defaultRenderState
	}
	scope := "plg:" + rc.PluginID + ":" + rc.SlotID
	renderNode(c, rc, root, scope, 0)
}

var defaultRenderState = NewRenderState()

func nodeKey(scope string, n *rpc.UINode, idx int) string {
	k := n.Key
	if k == "" {
		k = n.ID
	}
	if k == "" {
		k = fmt.Sprintf("%s-%d", n.Type, idx)
	}
	return scope + ":" + k
}

func renderNode(c *ui.Context, rc RenderContext, n *rpc.UINode, scope string, idx int) {
	if n == nil {
		return
	}
	pal := rc.Palette
	key := nodeKey(scope, n, idx)
	compID := n.ID
	if compID == "" {
		compID = n.Key
	}

	switch n.Type {
	case "column":
		el := ui.Column(c.Key(key))
		applyContainerStyle(el, n.Style, pal)
		el.Children(func() {
			for i, ch := range n.Children {
				renderNode(c, rc, ch, key, i)
			}
		})

	case "row":
		el := ui.Row(c.Key(key))
		applyContainerStyle(el, n.Style, pal)
		el.Children(func() {
			for i, ch := range n.Children {
				renderNode(c, rc, ch, key, i)
			}
		})

	case "box":
		el := ui.Box(c.Key(key))
		applyContainerStyle(el, n.Style, pal)
		if propBool(n.Props, "center", false) {
			el.Center()
		}
		el.Children(func() {
			for i, ch := range n.Children {
				renderNode(c, rc, ch, key, i)
			}
		})

	case "spacer":
		if n.Style != nil && (n.Style.Width > 0 || n.Style.Height > 0) {
			b := ui.Box(c.Key(key))
			if n.Style.Width > 0 {
				b.Width(n.Style.Width)
			}
			if n.Style.Height > 0 {
				b.Height(n.Style.Height)
			}
		} else {
			ui.Spacer(c)
		}

	case "divider":
		d := ui.Box(c.Key(key)).Shrink(0)
		col := pal.Divider
		if n.Style != nil && n.Style.Color != "" {
			if parsed, ok := ResolveColor(n.Style.Color, pal); ok {
				col = parsed
			}
		}
		d.Background(col)
		if propBool(n.Props, "vertical", false) {
			d.Width(1).FillHeight()
		} else {
			d.Height(1).FillWidth()
		}

	case "scroll":
		sc := ui.Scroll(c.Key(key))
		if n.Style != nil {
			applyContainerStyle(sc, n.Style, pal)
		} else {
			sc.Grow(1).MinHeight(0)
		}
		sc.Children(func() {
			for i, ch := range n.Children {
				renderNode(c, rc, ch, key, i)
			}
		})

	case "list":
		gap := float32(4)
		if n.Style != nil && n.Style.Gap > 0 {
			gap = n.Style.Gap
		}
		dividers := propBool(n.Props, "dividers", false)
		col := ui.Column(c.Key(key)).Gap(gap)
		applyContainerStyle(col, n.Style, pal)
		col.Children(func() {
			if len(n.Children) == 0 {
				if emptyText := propString(n.Props, "emptyText", ""); emptyText != "" {
					ui.Text(c.Key(key+"-empty"), emptyText).FontSize(12.5).TextColor(pal.TextFaint)
				}
				return
			}
			for i, ch := range n.Children {
				if i > 0 && dividers {
					ui.Box(c.Key(fmt.Sprintf("%s-div-%d", key, i))).Height(1).FillWidth().Background(pal.Divider)
				}
				renderNode(c, rc, ch, key, i)
			}
		})

	case "infoCard":
		title := propString(n.Props, "title", "")
		aside := propString(n.Props, "aside", propString(n.Props, "badge", ""))
		collapsible := propBool(n.Props, "collapsible", title != "")
		foldedPtr := rc.State.getBool(key+":folded", propBool(n.Props, "collapsed", false))

		card := ui.Column(c.Key(key)).Radius(12).Background(pal.Chip).Clip()
		applyContainerStyle(card, n.Style, pal)
		card.Children(func() {
			if title != "" {
				renderCardHeader(c, rc, pal, key, title, aside, collapsible, foldedPtr)
			}
			if !collapsible || !*foldedPtr {
				ui.Column(c.Key(key + "-body")).Padding(10, 12).Gap(6).Children(func() {
					for i, ch := range n.Children {
						renderNode(c, rc, ch, key, i)
					}
				})
			}
		})

	case "settingsGroup":
		title := propString(n.Props, "title", "")
		note := propString(n.Props, "note", "")
		ui.Column(c.Key(key)).Gap(8).Children(func() {
			if title != "" {
				trow := ui.Row(c.Key(key + "-title-row")).Padding(0, 4)
				trow.Children(func() {
					if rc.SettingsSearch {
						kit.Highlight(c, trow, title, pal, 2, 3)
					}
					ui.Text(c.Key(key+"-title"), title).FontSize(12).FontWeight(600).TextColor(pal.TextMuted)
				})
			}
			box := ui.Column(c.Key(key + "-box")).Radius(12).Background(pal.Chip).Padding(4, 16)
			box.Children(func() {
				for i, ch := range n.Children {
					if i > 0 {
						ui.Box(c.Key(fmt.Sprintf("%s-sep-%d", key, i))).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
					}
					renderNode(c, rc, ch, key, i)
				}
			})
			if note != "" {
				ui.Text(c.Key(key+"-note"), note).FontSize(12).TextColor(pal.TextFaint).LineHeight(1.4).Padding(0, 4)
			}
		})

	case "settingsRow":
		label := propString(n.Props, "label", "")
		detail := propString(n.Props, "detail", "")
		row := ui.Row(c.Key(key)).Padding(10, 0).Gap(16).AlignItems(ui.Center)
		applyContainerStyle(row, n.Style, pal)
		row.Children(func() {
			if rc.SettingsSearch {
				kit.Highlight(c, row, label, pal, 10, -2)
			}
			ui.Column(c.Key(key + "-lbl")).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c.Key(key+"-l"), label).FontSize(13.5).TextColor(pal.Text)
				if detail != "" {
					ui.Text(c.Key(key+"-d"), detail).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
				}
			})
			ui.Row(c.Key(key + "-ctrl")).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
				for i, ch := range n.Children {
					renderNode(c, rc, ch, key, i)
				}
			})
		})

	case "text":
		txt := propString(n.Props, "text", propString(n.Props, "value", ""))
		el := ui.Text(c.Key(key), txt).TextColor(toneColor(propString(n.Props, "tone", ""), pal))
		applyTextStyle(el, n.Props, n.Style, pal)

	case "richText":
		row := ui.Row(c.Key(key)).Wrap().Gap(4).AlignItems(ui.Baseline)
		applyContainerStyle(row, n.Style, pal)
		row.Children(func() {
			if rawSpans, ok := n.Props["spans"].([]any); ok && len(rawSpans) > 0 {
				for i, item := range rawSpans {
					m, ok := item.(map[string]any)
					if !ok {
						continue
					}
					txt := propString(m, "text", "")
					el := ui.Text(c.Key(fmt.Sprintf("%s-sp-%d", key, i)), txt).TextColor(toneColor(propString(m, "tone", ""), pal))
					if propBool(m, "bold", false) {
						el.FontWeight(600)
					}
					if propBool(m, "mono", false) {
						el.Font("monospace")
					}
					if fs := propFloat32(m, "fontSize", 0); fs > 0 {
						el.FontSize(fs)
					} else {
						el.FontSize(12.5)
					}
					if colStr := propString(m, "color", ""); colStr != "" {
						if parsed, ok := ResolveColor(colStr, pal); ok {
							el.TextColor(parsed)
						}
					}
				}
			} else {
				txt := propString(n.Props, "text", propString(n.Props, "value", ""))
				el := ui.Text(c.Key(key+"-t"), txt).TextColor(toneColor(propString(n.Props, "tone", ""), pal))
				applyTextStyle(el, n.Props, n.Style, pal)
			}
		})

	case "badge":
		txt := propString(n.Props, "text", propString(n.Props, "label", ""))
		tone := propString(n.Props, "tone", "")
		fg, bg := badgeColors(tone, pal)
		b := ui.Row(c.Key(key)).Padding(2, 7).Radius(999).Background(bg).AlignItems(ui.Center).Shrink(0)
		b.Children(func() {
			ui.Text(c.Key(key+"-t"), txt).FontSize(11).FontWeight(600).TextColor(fg).SingleLine()
		})

	case "icon":
		name := propString(n.Props, "name", "")
		size := propFloat32(n.Props, "size", 14)
		col := toneColor(propString(n.Props, "tone", ""), pal)
		if n.Style != nil && n.Style.Color != "" {
			if parsed, ok := ResolveColor(n.Style.Color, pal); ok {
				col = parsed
			}
		}
		if svg := resolveIconSVG(rc.State, rc.PluginDir, name); svg != nil {
			ui.Icon(c.Key(key), svg).Size(size, size).TextColor(col).Shrink(0)
		} else {
			ui.Box(c.Key(key)).Size(size*0.6, size*0.6).Radius(size * 0.3).Background(col).Shrink(0)
		}

	case "image":
		srcRel := propString(n.Props, "src", propString(n.Props, "path", ""))
		w := propFloat32(n.Props, "width", 24)
		h := propFloat32(n.Props, "height", 24)
		rad := propFloat32(n.Props, "radius", 0)
		if n.Style != nil {
			if n.Style.Width > 0 {
				w = n.Style.Width
			}
			if n.Style.Height > 0 {
				h = n.Style.Height
			}
			if n.Style.Radius > 0 {
				rad = n.Style.Radius
			}
		}
		svg, bmp := rc.State.loadImage(rc.PluginDir, srcRel)
		switch {
		case svg != nil:
			ui.Icon(c.Key(key), svg).Size(w, h).Shrink(0)
		case bmp != nil:
			img := ui.Image(c.Key(key), bmp).Size(w, h).Shrink(0)
			if rad > 0 {
				img.Radius(rad)
			}
		default:
			if rad == 0 {
				rad = 4
			}
			ui.Box(c.Key(key)).Size(w, h).Radius(rad).Background(pal.Divider)
		}

	case "spinner":
		size := propFloat32(n.Props, "size", 14)
		col := toneColor(propString(n.Props, "tone", "accent"), pal)
		lbl := propString(n.Props, "label", propString(n.Props, "text", ""))
		drawSpin := func(k string) {
			sp := ui.Box(c.Key(k)).Size(size, size).Radius(size / 2).Shrink(0)
			pulse := sp.Animate("spin-pulse", 1, 900*time.Millisecond)
			sp.Background(col.Alpha(0.2 + 0.6*pulse))
		}
		if lbl != "" {
			ui.Row(c.Key(key)).Gap(6).AlignItems(ui.Center).Children(func() {
				drawSpin(key + "-dot")
				ui.Text(c.Key(key+"-lbl"), lbl).FontSize(12).TextColor(pal.TextMuted).SingleLine()
			})
		} else {
			drawSpin(key)
		}

	case "progress":
		val := propFloat64(n.Props, "value", 0)
		if val < 0 {
			val = 0
		}
		if val > 1 {
			if val <= 100 {
				val = val / 100
			} else {
				val = 1
			}
		}
		h := float32(6)
		if n.Style != nil && n.Style.Height > 0 {
			h = n.Style.Height
		}
		fillCol := toneColor(propString(n.Props, "tone", "accent"), pal)
		trackCol := pal.Text.Alpha(0.1)
		if pal.Dark {
			trackCol = ui.RGBA(255, 255, 255, 0.14)
		}
		pct := float32(val)
		bar := ui.Box(c.Key(key)).Height(h).FillWidth().Radius(h / 2).Background(trackCol).Clip()
		if n.Style != nil && n.Style.Width > 0 {
			bar.Width(n.Style.Width)
		}
		bar.Draw(func(p *ui.Painter, r ui.Rect) {
			if r.W <= 0 || r.H <= 0 || pct <= 0 {
				return
			}
			w := r.W * pct
			if w < r.H && pct > 0 {
				w = min(r.W, r.H)
			}
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, fillCol, r.H/2)
		})

	case "button":
		label := propString(n.Props, "label", propString(n.Props, "title", compID))
		variant := propString(n.Props, "variant", "secondary")
		disabled := propBool(n.Props, "disabled", false)
		keys := propString(n.Props, "keys", "")
		if disabled {
			btn := ui.ButtonBase(c.Key(key)).Height(28).Padding(0, 14).Radius(7).
				AlignItems(ui.Center).Cursor(ui.CursorDefault).Label(label).Disabled(true)
			switch variant {
			case "ghost", "quiet":
				btn.Padding(4, 8).Radius(6).TextColor(pal.TextMuted)
			case "danger":
				btn.Background(pal.Danger).TextColor(ui.Hex("#ffffff"))
			case "primary":
				btn.Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
			default:
				btn.Background(pal.Control).TextColor(pal.TextMuted)
			}
			btn.Children(func() {
				ui.Text(c.Key(key+"-lbl"), label).FontSize(12.5).FontWeight(500).SingleLine()
			})
			return
		}
		var clicked bool
		switch variant {
		case "primary":
			clicked = style.SizedButton(c, key, label, keys, true, 28, 7, pal)
		case "ghost", "quiet":
			clicked = style.QuietButton(c, key, label, propString(n.Props, "tooltip", ""), pal)
		case "danger":
			btn := ui.ButtonBase(c.Key(key)).Height(28).Padding(0, 12).Radius(7).AlignItems(ui.Center).Background(pal.Danger).TextColor(ui.Hex("#ffffff")).Label(label)
			btn.Children(func() { ui.Text(c.Key(key+"-lbl"), label).FontSize(12.5).FontWeight(500).SingleLine() })
			clicked = btn.Clicked()
		default:
			clicked = style.SizedButton(c, key, label, keys, false, 28, 7, pal)
		}
		if clicked && rc.OnEvent != nil {
			evName := n.OnClick
			if evName == "" {
				evName = "click"
			}
			emitAsync(rc.OnEvent, compID, evName, nil)
		}

	case "toggle":
		val := propBool(n.Props, "value", propBool(n.Props, "checked", false))
		ptr := rc.State.syncBoolProp(key, val)
		sw := style.Toggle(c.Key(key), ptr, pal)
		sw.OnChange(func() {
			if rc.OnEvent != nil {
				evName := n.OnChange
				if evName == "" {
					evName = "change"
				}
				emitAsync(rc.OnEvent, compID, evName, *ptr)
			}
		})

	case "segmented":
		opts := parseOptions(n.Props)
		if len(opts) == 0 {
			return
		}
		vals := make([]string, len(opts))
		labels := make([]string, len(opts))
		valid := make(map[string]bool, len(opts))
		for i, o := range opts {
			vals[i] = o.Value
			labels[i] = o.Label
			valid[o.Value] = true
		}
		curVal := propString(n.Props, "value", vals[0])
		if !valid[curVal] {
			curVal = vals[0]
		}
		ptr := rc.State.syncStringProp(key, curVal)
		if !valid[*ptr] {
			*ptr = curVal
		}
		kit.Segmented(c, key, ptr, vals, labels, func() {
			if rc.OnEvent != nil {
				evName := n.OnChange
				if evName == "" {
					evName = "change"
				}
				emitAsync(rc.OnEvent, compID, evName, *ptr)
			}
		})

	case "select":
		opts := parseOptions(n.Props)
		if len(opts) == 0 {
			return
		}
		vals := make([]string, len(opts))
		labels := make(map[string]string, len(opts))
		for i, o := range opts {
			vals[i] = o.Value
			labels[o.Value] = o.Label
		}
		cur := propString(n.Props, "value", vals[0])
		width := float32(180)
		if n.Style != nil && n.Style.Width > 0 {
			width = n.Style.Width
		}
		ptr := rc.State.syncStringProp(key, cur)
		if _, ok := labels[*ptr]; !ok {
			*ptr = cur
		}
		trig := controls.Popup(c, key, ptr, vals, func(v string) string {
			if l, ok := labels[v]; ok {
				return l
			}
			return v
		}, width, pal)
		trig.OnChange(func() {
			if rc.OnEvent != nil {
				evName := n.OnChange
				if evName == "" {
					evName = "change"
				}
				emitAsync(rc.OnEvent, compID, evName, *ptr)
			}
		})

	case "textInput":
		cur := propString(n.Props, "value", "")
		placeholder := propString(n.Props, "placeholder", "")
		width := float32(0)
		if n.Style != nil && n.Style.Width > 0 {
			width = n.Style.Width
		}
		ptr := rc.State.syncStringProp(key, cur)
		in := controls.TextField(c, key, ptr, placeholder, width, pal)
		in.OnChange(func() {
			if rc.OnEvent != nil {
				evName := n.OnChange
				if evName == "" {
					evName = "change"
				}
				emitAsync(rc.OnEvent, compID, evName, *ptr)
			}
		})
		in.OnSubmit(func() {
			if rc.OnEvent != nil {
				evName := n.OnSubmit
				if evName == "" {
					evName = "submit"
				}
				emitAsync(rc.OnEvent, compID, evName, *ptr)
			}
		})

	case "slider":
		lo := propFloat64(n.Props, "min", 0)
		hi := propFloat64(n.Props, "max", 100)
		val := propFloat64(n.Props, "value", lo)
		width := float32(160)
		if n.Style != nil && n.Style.Width > 0 {
			width = n.Style.Width
		}
		ptr := rc.State.syncFloatProp(key, val)
		sl := controls.Slider(c, key, ptr, lo, hi, width, pal)
		sl.OnChange(func() {
			if rc.OnEvent != nil {
				evName := n.OnChange
				if evName == "" {
					evName = "change"
				}
				emitAsync(rc.OnEvent, compID, evName, *ptr)
			}
		})
	}
}

func emitAsync(cb EventCallback, compID, eventType string, value any) {
	go cb(compID, eventType, value)
}

func renderCardHeader(c *ui.Context, rc RenderContext, pal *theme.Palette, key, title, aside string, collapsible bool, folded *bool) {
	head := ui.ButtonBase(c.Key(key + "-head")).Height(38).Padding(0, 12).Gap(8).AlignItems(ui.Center)
	if collapsible {
		head.Cursor(ui.CursorPointer)
	}
	head.Children(func() {
		ui.Text(c.Key(key+"-ht"), title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		ui.Spacer(c)
		if aside != "" {
			ui.Text(c.Key(key+"-ha"), aside).FontSize(11.5).TextColor(pal.TextFaint).SingleLine().Shrink(0)
		}
		if collapsible {
			turn := float32(90)
			if *folded {
				turn = 0
			}
			ui.Icon(c.Key(key+"-chev"), glyph.Chevron).Size(11, 11).TextColor(pal.TextFaint).Shrink(0).Rotate(turn)
		}
	})
	if collapsible && head.Clicked() {
		*folded = !*folded
	}
	if !collapsible || !*folded {
		ui.Box(c.Key(key + "-hrule")).Height(1).FillWidth().Background(pal.Divider)
	}
}

func resolveIconSVG(rs *RenderState, pluginDir, name string) *ui.SVG {
	if name == "" {
		return nil
	}
	if strings.HasSuffix(strings.ToLower(name), ".svg") && pluginDir != "" {
		svg, _ := rs.loadImage(pluginDir, name)
		return svg
	}
	svg, _ := glyph.Try(name)
	return svg
}

func applyContainerStyle(el ui.Element, st *rpc.UIStyle, pal *theme.Palette) {
	if st == nil {
		return
	}
	if st.Width > 0 {
		el.Width(st.Width)
	}
	if st.Height > 0 {
		el.Height(st.Height)
	}
	if st.MinWidth > 0 {
		el.MinWidth(st.MinWidth)
	}
	if st.MinHeight > 0 {
		el.MinHeight(st.MinHeight)
	}
	if st.MaxWidth > 0 {
		el.MaxWidth(st.MaxWidth)
	}
	if st.MaxHeight > 0 {
		el.MaxHeight(st.MaxHeight)
	}
	if st.Flex > 0 {
		el.Grow(st.Flex)
	}
	if st.Gap > 0 {
		el.Gap(st.Gap)
	}
	if st.Radius > 0 {
		el.Radius(st.Radius)
	}
	if len(st.Padding) > 0 {
		switch len(st.Padding) {
		case 1:
			el.Padding(st.Padding[0])
		case 2:
			el.Padding(st.Padding[0], st.Padding[1])
		case 4:
			el.Padding(st.Padding[0], st.Padding[1], st.Padding[2], st.Padding[3])
		}
	}
	if st.Wrap {
		el.Wrap()
	}
	switch st.Align {
	case "start":
		el.AlignItems(ui.Start)
	case "center":
		el.AlignItems(ui.Center)
	case "end":
		el.AlignItems(ui.End)
	case "stretch":
		el.AlignItems(ui.Stretch)
	}
	switch st.Justify {
	case "start":
		el.Justify(ui.Start)
	case "center":
		el.Justify(ui.Center)
	case "end":
		el.Justify(ui.End)
	case "between":
		el.Justify(ui.SpaceBetween)
	}
	if st.Background != "" {
		if col, ok := ResolveColor(st.Background, pal); ok {
			el.Background(col)
		}
	}
	if st.Border != "" {
		if col, ok := ResolveColor(st.Border, pal); ok {
			bw := st.BorderSize
			if bw <= 0 {
				bw = 1
			}
			el.Border(bw, col)
		}
	}
}

func applyTextStyle(el ui.Element, props map[string]any, st *rpc.UIStyle, pal *theme.Palette) {
	fontSize := float32(13)
	if st != nil && st.FontSize > 0 {
		fontSize = st.FontSize
	}
	el.FontSize(fontSize)
	if st != nil {
		if st.FontWeight > 0 {
			el.FontWeight(st.FontWeight)
		}
		if st.Monospace {
			el.Font("monospace")
		}
		if st.Tabular {
			el.FontFeatures("tnum")
		}
		if st.Color != "" {
			if col, ok := ResolveColor(st.Color, pal); ok {
				el.TextColor(col)
			}
		}
	}
	if propBool(props, "singleLine", false) {
		el.SingleLine().Ellipsis("…")
	}
	if propBool(props, "wrap", false) || (st != nil && st.Wrap) {
		el.Wrap()
	}
	if propBool(props, "tabular", false) {
		el.FontFeatures("tnum")
	}
	if propBool(props, "mono", false) {
		el.Font("monospace")
	}
}

func toneColor(tone string, pal *theme.Palette) ui.Color {
	switch tone {
	case "muted":
		return pal.TextMuted
	case "faint":
		return pal.TextFaint
	case "accent":
		return pal.Accent
	case "attention", "warning":
		return pal.Attention
	case "done", "success":
		return pal.Done
	case "danger", "error":
		return pal.Danger
	default:
		return pal.Text
	}
}

func badgeColors(tone string, pal *theme.Palette) (fg, bg ui.Color) {
	switch tone {
	case "accent":
		return pal.Accent, pal.Accent.Alpha(0.14)
	case "attention", "warning":
		return pal.Attention, pal.Attention.Alpha(0.15)
	case "done", "success":
		return pal.Done, pal.Done.Alpha(0.15)
	case "danger", "error":
		return pal.Danger, pal.Danger.Alpha(0.15)
	default:
		return pal.TextMuted, pal.Text.Alpha(0.08)
	}
}

// ResolveColor parses "$token[@alpha]", "#rgb", "#rrggbb", "#rrggbbaa", "rgb(r,g,b)", and "rgba(r,g,b,a)".
func ResolveColor(raw string, pal *theme.Palette) (ui.Color, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ui.Color{}, false
	}
	if strings.HasPrefix(s, "$") && pal != nil {
		name, alphaStr, hasAlpha := strings.Cut(s[1:], "@")
		var c ui.Color
		switch name {
		case "text":
			c = pal.Text
		case "textMuted", "muted":
			c = pal.TextMuted
		case "textFaint", "faint":
			c = pal.TextFaint
		case "accent":
			c = pal.Accent
		case "attention":
			c = pal.Attention
		case "attnSoft":
			c = pal.AttnSoft
		case "done":
			c = pal.Done
		case "danger":
			c = pal.Danger
		case "card", "bg":
			c = pal.Card
		case "chip":
			c = pal.Chip
		case "divider":
			c = pal.Divider
		case "control":
			c = pal.Control
		case "popover":
			c = pal.Popover
		case "sidebar":
			c = pal.Sidebar
		case "tlThink":
			c = pal.TlThink
		case "tlText":
			c = pal.TlText
		case "tlTool":
			c = pal.TlTool
		case "tlSub":
			c = pal.TlSub
		case "tlWait":
			c = pal.TlWait
		case "tlFail":
			c = pal.TlFail
		default:
			return ui.Color{}, false
		}
		if hasAlpha {
			if a, err := strconv.ParseFloat(strings.TrimSpace(alphaStr), 32); err == nil {
				c = c.Alpha(float32(min(max(a, 0), 1)))
			}
		}
		return c, true
	}
	return theme.ParseColor(s)
}

type selectOption struct {
	Value string
	Label string
}

func parseOptions(props map[string]any) []selectOption {
	raw, ok := props["options"].([]any)
	if !ok {
		return nil
	}
	var out []selectOption
	for _, it := range raw {
		switch v := it.(type) {
		case string:
			out = append(out, selectOption{Value: v, Label: v})
		case map[string]any:
			val := propString(v, "value", propString(v, "id", ""))
			lbl := propString(v, "label", propString(v, "title", val))
			if val != "" {
				out = append(out, selectOption{Value: val, Label: lbl})
			}
		}
	}
	return out
}

func propString(props map[string]any, key, def string) string {
	if props == nil {
		return def
	}
	if v, ok := props[key]; ok && v != nil {
		switch s := v.(type) {
		case string:
			return s
		case fmt.Stringer:
			return s.String()
		default:
			return fmt.Sprint(v)
		}
	}
	return def
}

func propBool(props map[string]any, key string, def bool) bool {
	if props == nil {
		return def
	}
	if v, ok := props[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func propFloat32(props map[string]any, key string, def float32) float32 {
	if props == nil {
		return def
	}
	if v, ok := props[key]; ok {
		switch n := v.(type) {
		case float64:
			return float32(n)
		case float32:
			return n
		case int:
			return float32(n)
		}
	}
	return def
}

func propFloat64(props map[string]any, key string, def float64) float64 {
	if props == nil {
		return def
	}
	if v, ok := props[key]; ok {
		switch n := v.(type) {
		case float64:
			if !math.IsNaN(n) {
				return n
			}
		case float32:
			return float64(n)
		case int:
			return float64(n)
		}
	}
	return def
}
