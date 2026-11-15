// Package paints draws what the app marks a thing with: a session's
// status, a shortcut's keys, and how long ago a time was.
package paints

import (
	"fmt"
	"math"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"hyla/internal/proto"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Ago is how long ago a time in Unix milliseconds was, shortly, as
// proto.Ago writes it, in the language the app shows.
func Ago(ms int64, now time.Time) string {
	if ms == 0 {
		return ""
	}
	d := now.Sub(time.UnixMilli(ms))
	switch {
	case d < time.Minute:
		return locales.AgoNow()
	case d < time.Hour:
		return locales.AgoMinutes(int(d / time.Minute))
	case d < 24*time.Hour:
		return locales.AgoHours(int(d / time.Hour))
	case d < 7*24*time.Hour:
		return locales.AgoDays(int(d / (24 * time.Hour)))
	}
	return locales.AgoWeeks(int(d / (7 * 24 * time.Hour)))
}

func Kbd(c *ui.Context, pal *theme.Palette, keys string) {
	ui.Text(c, keys).FontSize(11).TextColor(pal.TextMuted).Padding(1, 5).Radius(4).
		Background(pal.Kbd).SingleLine()
}

// statusIcon draws a session's status, as the sidebar shows it: a ring
// turning while it works, an amber ring when it needs the user, a filled
// check when it finished unseen, a faint check once seen.
func Status(c *ui.Context, pal *theme.Palette, st proto.Status, unread bool, size float32) ui.Element {
	return StatusP(c, pal, st, unread, -1, size)
}

// StatusP is Status with a program's progress (OSC 7501), 0–100,
// which a working ring fills instead of turning; -1 for none.
func StatusP(c *ui.Context, pal *theme.Palette, st proto.Status, unread bool, progress int, size float32) ui.Element {
	box := ui.Box(c).Size(size, size).Shrink(0)
	switch {
	case st == proto.Working:
		box.Label(locales.StatusWorking())
	case st == proto.Waiting:
		box.Label(locales.StatusNeedsYou())
	case st == proto.Exited:
		box.Label(locales.StatusExited())
	case st == proto.Failed:
		box.Label(locales.StatusFailed())
	case unread:
		box.Label(locales.StatusDone())
	}
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		rad := min(r.W, r.H)/2 - 1
		switch {
		case st == proto.Working && progress >= 0:
			var track ui.Path
			track.Circle(cx, cy, rad)
			p.StrokePath(&track, 1.7, pal.Text.Alpha(0.13))
			// From the top, clockwise: the circle starts at 3 o'clock.
			p.Transform(ui.Rotation(-90), cx, cy, func() {
				var arc ui.Path
				arc.Circle(cx, cy, rad)
				p.StrokePath(arc.Trim(0, max(0.02, float32(progress)/100)), 1.7, pal.Text)
			})
		case st == proto.Working:
			var track ui.Path
			track.Circle(cx, cy, rad)
			p.StrokePath(&track, 1.7, pal.Text.Alpha(0.13))
			p.AnimationFrame()
			deg := float32(p.Now().UnixMilli()%1000) / 1000 * 360
			p.Transform(ui.Rotation(deg), cx, cy, func() {
				var arc ui.Path
				arc.Circle(cx, cy, rad)
				p.StrokePath(arc.Trim(0, 0.3), 1.7, pal.Text)
			})
		case st == proto.Waiting:
			var ring ui.Path
			ring.Circle(cx, cy, rad)
			p.StrokePath(&ring, 1.7, pal.Attention)
			p.AnimationFrame()
			pulse := float32(0.5 + 0.5*math.Sin(float64(p.Now().UnixMilli()%1600)/1600*2*math.Pi))
			var dot ui.Path
			dot.Circle(cx, cy, rad*0.42)
			p.FillPath(&dot, pal.Attention.Alpha(0.65+0.35*pulse))
		case st == proto.Failed:
			var disc ui.Path
			disc.Circle(cx, cy, rad+0.5)
			col := pal.Danger
			if !unread {
				col = pal.Danger.Alpha(0.75)
			}
			p.FillPath(&disc, col)
			p.Line(cx, cy-rad*0.45, cx, cy+rad*0.08, 1.7, pal.OnDone)
			var dot ui.Path
			dot.Circle(cx, cy+rad*0.42, 1)
			p.FillPath(&dot, pal.OnDone)
		case st == proto.Exited:
			col := pal.TextFaint
			if unread {
				col = pal.Danger
			}
			var ring ui.Path
			ring.Circle(cx, cy, rad)
			p.StrokePath(&ring, 1.5, col)
			k := rad * 0.38
			p.Line(cx-k, cy-k, cx+k, cy+k, 1.5, col)
			p.Line(cx+k, cy-k, cx-k, cy+k, 1.5, col)
		case unread:
			var disc ui.Path
			disc.Circle(cx, cy, rad+0.5)
			p.FillPath(&disc, pal.Done)
			check := (&ui.Path{}).MoveTo(cx-rad*0.42, cy+0.2).LineTo(cx-rad*0.08, cy+rad*0.36).LineTo(cx+rad*0.46, cy-rad*0.34)
			p.StrokePath(check, 1.6, pal.OnDone)
		default:
			check := (&ui.Path{}).MoveTo(cx-rad*0.62, cy+0.4).LineTo(cx-rad*0.12, cy+rad*0.5).LineTo(cx+rad*0.66, cy-rad*0.46)
			p.StrokePath(check, 1.5, pal.TextFaint)
		}
	})
	return box
}

// IconButton is a quiet square button with an icon, as in a toolbar.
func IconButton(c *ui.Context, key string, svg *ui.SVG, label string, pal *theme.Palette, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Size(28, 26).Radius(6).Center().TextColor(pal.TextMuted).Label(label).Tooltip(label)
	if b.Pressed() {
		b.Background(pal.RowHover.Alpha(0.12))
	} else if b.Hovered() {
		b.Background(pal.RowHover)
		b.TextColor(pal.Text)
	}
	b.Children(func() { ui.Icon(c, svg).Size(16, 16) })
	if b.Clicked() {
		fn()
	}
}

// tipsAt are where PointerTooltip's tips show, by key: where the pointer
// rested as each came, which they keep while they show.
var tipsAt = map[string]ui.Point{}

// PointerTooltip shows s as el's tooltip, as Element.Tooltip does, but
// where macOS shows its tooltips: below the pointer where it came to rest,
// clear of the arrow, rather than centered above el, which a long row
// puts far from where the user looks.
func PointerTooltip(c *ui.Context, el ui.Element, key, s string) {
	el.Description(s)
	const below = 18 // the arrow's height, from its tip
	px, py, _ := el.PointerPosition()
	at, showing := tipsAt[key]
	if !showing {
		b := el.Bounds()
		at = ui.Point{X: b.X + px, Y: b.Y + py + below}
	}
	var spot ui.Element
	ui.Overlay(c, func() {
		spot = ui.Box(c.Key(key+"-at")).Absolute().Left(at.X).Top(at.Y).Size(0, 0).PassThrough()
	})
	shown := false
	ui.TooltipBase(c, el, func(tip ui.Element) {
		shown = true
		th := c.Theme()
		fill, text := th.Inverse, th.InverseText
		if fill == ui.Transparent {
			fill = th.Text
		}
		if text == ui.Transparent {
			text = th.Background
		}
		tip.MaxWidth(th.Space(80)).Padding(th.Space(1.25), th.Space(2)).Radius(th.Space(1.25)).
			Background(fill).TextColor(text).FontSize(th.FontSize-1).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.2))
		tip.Children(func() { ui.Text(c, s) })
		// A box where the pointer came to rest, in the window, for the tip
		// to attach to: a place in the row would be laid out a frame behind
		// the row's, and the tip would show under the pointer, on the row
		// after this one, while it keeps a place of its own.
		tip.AttachTo(spot, ui.AnchorTopLeft, ui.AnchorTopLeft)
	})
	if shown {
		tipsAt[key] = at
	} else {
		delete(tipsAt, key)
	}
}

// Stamp is what of a worktree's status changes its changes.
func Stamp(g *proto.GitInfo) string {
	if g == nil {
		return ""
	}
	return fmt.Sprint(g.Root, g.Head, g.Branch, g.Files, g.Added, g.Removed)
}

// ReplaceText makes a buffer's text text by one edit of what differs
// between them, on whole characters.
func ReplaceText(b *editor.Buffer, text string) {
	cur := b.String()
	if cur == text {
		return
	}
	p := 0
	for p < len(cur) && p < len(text) && cur[p] == text[p] {
		p++
	}
	for p > 0 && p < len(cur) && !utf8.RuneStart(cur[p]) {
		p--
	}
	q := 0
	for q < len(cur)-p && q < len(text)-p && cur[len(cur)-1-q] == text[len(text)-1-q] {
		q++
	}
	for q > 0 && !utf8.RuneStart(cur[len(cur)-q]) {
		q--
	}
	b.Edit(editor.Edit{Start: p, End: len(cur) - q, Text: text[p : len(text)-q]})
}
