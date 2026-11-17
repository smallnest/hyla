package panes

import (
	"math"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// paneGap is the room between panes, where the divider is.
const paneGap = 8

// sidebarArt is a picture behind the terminals, and the file it is of.
type sidebarArt struct {
	path string     // the file, as the settings name it; "" for none
	bm   *ui.Bitmap // nil while it comes, or when it is no picture
	// card is the color of a terminal's card, for its picture to show
	// over, as the card says it: dimmed when the user is in another
	// pane.
	card *ui.Color
}

// sessionSwitch is how one session comes in place of the one before, as
// the settings say (session-switch).
type SessionSwitch int

const (
	SwitchNone SessionSwitch = iota
	SwitchDissolve
	SwitchSlide
	SwitchPush
	SwitchCover
	SwitchZoom
)

// transition returns the transition of the frame that holds a session's
// panes: the dissolve fades one in as the one it leaves, as the frame
// before showed it, blurs and fades away over it.
func (sw SessionSwitch) transition() ui.ElementTransition {
	if sw != SwitchDissolve {
		return ui.ElementTransition{Duration: -1}
	}
	return ui.ElementTransition{
		Animation:    &ui.Snappy,
		ExitDuration: 200 * time.Millisecond,
		Enter:        &ui.Motion{},
		Exit:         &ui.Motion{Opacity: 0, Blur: 8},
	}
}

// motion returns where a session comes in from: as it slides in from the
// right, or grows into place.
func (sw SessionSwitch) motion() ui.Motion {
	switch sw {
	case SwitchSlide, SwitchPush, SwitchCover:
		return ui.Motion{XPercent: 100}
	case SwitchZoom:
		return ui.Motion{Opacity: 1, Scale: 0.9}
	}
	return ui.Motion{}
}

// pose returns where a session's panes stand, as they come in (in) or
// when they land.
func (sw SessionSwitch) pose(from ui.Motion, in bool) ui.Pose {
	if !in {
		return ui.Pose{}
	}
	p := ui.Pose{X: from.X, Y: from.Y, XPercent: from.XPercent, YPercent: from.YPercent, Rotate: from.Rotate}
	if from.Opacity != 0 {
		// A pose's Fade goes from 0, opaque, to 1: as a motion's Opacity.
		p.Fade = from.Opacity
	}
	if from.Scale != 0 {
		p.Scale = from.Scale
	}
	return p
}

// Transition is a session's switch as a frame's transition.
func Transition(sw SessionSwitch) ui.ElementTransition { return sw.transition() }

// Motion is where a session comes in from, as it switches.
func Motion(sw SessionSwitch) ui.Motion { return sw.motion() }

// Pose is where a session's panes stand, as it comes in (in) or lands.
func Pose(sw SessionSwitch, from ui.Motion, in bool) ui.Pose { return sw.pose(from, in) }

// Switch is how a session comes in place of another, as the settings say.
func Switch(name string) SessionSwitch {
	switch name {
	case "slide":
		return SwitchSlide
	case "push":
		return SwitchPush
	case "cover":
		return SwitchCover
	case "zoom":
		return SwitchZoom
	case "none":
		return SwitchNone
	}
	return SwitchDissolve
}

// LayoutNode lays out a node of a session's layout: a pane, or two.
func LayoutNode(x App, c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, l *proto.Layout, path string, top, left, right bool) {
	if l.Leaf() {
		if p := s.PaneByID(l.Pane); p != nil {
			PaneCard(x, c, pal, bar, shown, s, p, top, left, right, true)
		}
		return
	}
	ratio := float32(l.Ratio)
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	// Row and Column each build an element, so make only the one used: a
	// Row made and replaced would stay, an empty sibling under the key.
	key := "split-" + s.ID + path
	var box ui.Element
	if l.Row {
		box = ui.Row(c.Key(key))
	} else {
		box = ui.Column(c.Key(key))
	}
	box.Grow(1).Basis(0).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Bind(x.PaneRef(key))
	box.Children(func() {
		ui.Column(c.Key("a")).Grow(ratio).Basis(0).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			LayoutNode(x, c, pal, bar, shown, s, l.A, path+"a", top, left, right && !l.Row)
		})
		divider(x, c, pal, s, l, key)
		ui.Column(c.Key("b")).Grow(1 - ratio).Basis(0).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			LayoutNode(x, c, pal, bar, shown, s, l.B, path+"b", top && l.Row, left && !l.Row, right)
		})
	})
}

// divider is the gap between two panes, which a drag moves.
func divider(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, l *proto.Layout, key string) {
	// A drag leaves the keyboard in the pane the user is in.
	d := ui.Box(c.Key("div")).Shrink(0).KeepFocus()
	if l.Row {
		d.Width(paneGap).Cursor(ui.CursorResizeColumn)
	} else {
		d.Height(paneGap).Cursor(ui.CursorResizeRow)
	}
	if d.Hovered() || d.Pressed() {
		// A short grip, so the gap shows it moves.
		d.Draw(func(p *ui.Painter, r ui.Rect) {
			if l.Row {
				p.Fill(ui.Rect{X: r.X + r.W/2 - 1.5, Y: r.Y + r.H/2 - 18, W: 3, H: 36}, pal.TextFaint.Alpha(0.7), 1.5)
			} else {
				p.Fill(ui.Rect{X: r.X + r.W/2 - 18, Y: r.Y + r.H/2 - 1.5, W: 36, H: 3}, pal.TextFaint.Alpha(0.7), 1.5)
			}
		})
	}
	if dx, dy, ok := d.Dragged(); ok {
		// The ratio shares out the room the panes take, the split's less
		// the divider's.
		b := x.PaneRef(key).Bounds(c)
		size, delta := b.W-paneGap, dx
		if !l.Row {
			size, delta = b.H-paneGap, dy
		}
		if size > 0 {
			// The ratio goes on past its bounds, unclamped, so the divider
			// comes back as the pointer comes back to it.
			held, r := x.LayoutDrag(key)
			if !held {
				rr := l.Ratio
				if rr <= 0 || rr >= 1 {
					rr = 0.5
				}
				r = rr
			}
			r += float64(delta / size)
			x.SetLayoutDrag(s.ID, key, r)
			l.Ratio = math.Min(math.Max(r, 0.12), 0.88)
		}
	} else if x.LayoutDragKey() == key && !d.Pressed() {
		// Only the divider dragged saves the layout: another of the same
		// session would save it every frame of the drag.
		x.ClearLayoutDrag()
		x.CallPane(s.ID, "layout", proto.LayoutParams{Session: s.ID, Layout: s.Layout.Clone()}, nil)
	}
	if d.DoubleClicked() {
		l.Ratio = 0.5
		x.CallPane(s.ID, "layout", proto.LayoutParams{Session: s.ID, Layout: s.Layout.Clone()}, nil)
	}
}

// PaneCard is a pane: a card with a header and the terminal. top, left and
// right tell that it touches the window's edges, where the header drags
// the window, keeps clear of the traffic lights on macOS, and holds the
// window controls elsewhere (windowControls).
func PaneCard(x App, c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, p *proto.Pane, top, left, right, multi bool) {
	// The session, as the app has it now: building this frame's tree takes
	// ensureTerm, which takes the keyboard from a pane the app has lost
	// while the one before still showed it.
	s = x.Session(s.ID)
	if s == nil {
		return
	}
	focused := p.ID == x.FocusOf(s)
	// A pane the user opened its session's chat in, alone or the zoomed
	// one, shows the chat in its terminal's place, which sessionPanes lays
	// out the one card for. Feishu being on alone says nothing: without
	// the pane's own choice, its session keeps its terminal.
	chat := focused && x.PaneChat()[p.ID] && (len(s.Zoomed) == 0 || s.Zoomed == p.ID)
	bg := pal.Card
	// A browser's page stays opaque, so its toolbar does too, and neither
	// fades: a page is read, not typed into, from beside it.
	if multi && !focused && !p.Shown() && x.Settings().DimUnfocused {
		bg = pal.CardDim
	}
	card := ui.Column(c.Key("pane-" + p.ID)).Grow(1).Basis(0).MinWidth(0).MinHeight(0).Radius(14).Clip().Bind(x.PaneRef("card-" + p.ID))
	// A terminal's card is translucent with the window: the others, a
	// browser's and a file's, are opaque.
	bgNow := card.AnimateColor("bg", bg, ui.Animation{Duration: 140 * time.Millisecond})
	if w := PaneBackground(x, pal, bg, p); w != nil {
		card.Material(w)
		if pal.Dark {
			style.DarkShadow(style.LitEdge(card, pal))
		} else {
			card.Shadow(0, 1, 2, 0, pal.CardShadow).Shadow(0, 1, 6, 0, pal.CardShadow2)
		}
	} else {
		style.Raised(card.Background(bgNow), pal)
	}
	hovered := card.Hovered()
	if chat {
		hovered = true
	}
	// The chat's card is opaque, as a browser's page is.
	if chat {
		card.Background(bgNow)
	}
	if x.Settings().FocusFollowsMouse && multi {
		followMouse(x, c, s, p, card, hovered, focused)
	}
	// The card takes presses: one on its padding, beside the terminal,
	// focuses the pane, rather than going on to what drags the window.
	if card.Pressed() && !focused {
		x.SetFocus(s, p.ID)
		x.PaneRef(p.ID).Focus()
	}

	// Another pane dragged over this one: where it would go.
	zone := ZoneNone
	if multi {
		if d, over := ui.DragOver[PaneDrag](card); over && d.Session == s.ID && d.Pane != p.ID {
			if px, py, in := card.PointerPosition(); in {
				b := x.PaneRef("card-" + p.ID).Bounds(c)
				zone = zoneAt(px/max(b.W, 1), py/max(b.H, 1))
			}
		}
		if d, ok := ui.Drop[PaneDrag](card); ok && d.Session == s.ID && d.Pane != p.ID {
			sid, target := s.ID, p.ID
			drop := x.LastZone()
			x.RunLater(func() { movePane(x, sid, d.Pane, target, drop) })
		}
	}
	if zone != ZoneNone {
		x.SetLastZone(zone)
	}

	dragging := false
	// dropping tells that files are dropping on the body of the pane, from
	// this machine or another Hyla, which it takes: a remote one puts them
	// where its program runs (drop_upload.go).
	dropping := false
	card.Children(func() {
		if p.FilePane() {
			dragging = paneHeader(x, c, pal, bar, shown, s, p, top, left, right, multi, focused, hovered)
			x.FilePane(c, pal, s, p)
		} else if p.TimelinePane() {
			dragging = paneHeader(x, c, pal, bar, shown, s, p, top, left, right, multi, focused, hovered)
			x.TimelineBody(c, pal, s, p)
		} else if p.SubagentsPane() {
			dragging = paneHeader(x, c, pal, bar, shown, s, p, top, left, right, multi, focused, hovered)
			x.SubagentsBody(c, pal, s, p)
		} else if p.Browser() {
			// A browser's address bar is its header.
			dragging = x.PaneBrowser(c, pal, bar, shown, s, p, top, left, right, multi, focused, hovered)
			dropping = dropFiles(x, c, pal, s, p)
		} else {
			dragging = paneHeader(x, c, pal, bar, shown, s, p, top, left, right, multi, focused, hovered)
			if chat {
				// The session's conversation, as a chat, in the terminal's
				// place (chat.go).
				dropping = dropFiles(x, c, pal, s, p)
				x.ChatBody(c, pal, s, p)
			} else {
				dropping = dropFiles(x, c, pal, s, p)
				paneTerminal(x, c, pal, s, p)
			}
		}
		if multi && !p.Shown() && x.Settings().DimUnfocused {
			// The terminals the user is not in fade, text and all.
			v := ui.Box(c.Key("veil")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Background(pal.Veil)
			v.Opacity(v.Animate("on", b2f(!focused), 160*time.Millisecond))
		}
		if zone != ZoneNone {
			dropPreview(x, c, pal, p.ID, zone)
		}
	})

	// The pane dragged sinks back where it was; one just moved springs in
	// at its new place.
	pose := ui.Pose{}
	switch {
	case dragging:
		pose = ui.Pose{Scale: 0.985, Fade: 0.45}
	case x.Arrived(p.ID):
		pose = ui.Pose{Scale: 0.94, Fade: 0.7}
		x.TakeArrived(p.ID)
		c.Invalidate()
	}
	card.AnimatePose(pose, ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: 0.18}})
	if dropping {
		// Files go in where its program runs.
		card.Draw(func(pr *ui.Painter, r ui.Rect) {
			pr.Stroke(ui.Rect{X: r.X + 1.5, Y: r.Y + 1.5, W: r.W - 3, H: r.H - 3}, pal.Accent.Alpha(0.65), 12, 2)
		})
	}
}

// PaneBackground is what a terminal's card shows in place of its
// background, bg being the card's color: the terminal's picture, when the
// user chose one, else the translucent glass while there is one. A
// browser's page and a file's pane keep their own.
func PaneBackground(x App, pal *theme.Palette, bg ui.Color, p *proto.Pane) ui.Material {
	if p.Shown() {
		return nil
	}
	st := x.Settings()
	if st.PaneImage != "" {
		art := x.PaneArt()
		if art.Path != st.PaneImage {
			x.LoadPaneArt(st.PaneImage)
		}
		return theme.PaneGlass(pal, *st, art.BM, &bg)
	}
	return theme.PaneGlass(pal, *st, nil, nil)
}

// followMouse focuses the pane the pointer moves into. Only the pointer
// moving does: a pane that comes under a pointer standing still, as another
// session shows or the layout changes, leaves the focus where the keyboard
// put it. And only going in does, so the keyboard can take the focus from
// the pane the pointer rests in.
func followMouse(x App, c *ui.Context, s *proto.Session, p *proto.Pane, card ui.Element, hovered, focused bool) {
	px, py, over := card.PointerPosition()
	if !over {
		if x.HoverPane() == p.ID {
			x.SetHoverPane("")
		}
		return
	}
	b := x.PaneRef("card-" + p.ID).Bounds(c)
	at := [2]float32{b.X + px, b.Y + py}
	moved := at != x.HoverAt()
	x.SetHoverAt(at)
	// A press begun in another pane, as a selection dragged on past its
	// edge, does not hover this one.
	if !hovered || x.HoverPane() == p.ID {
		return
	}
	x.SetHoverPane(p.ID)
	if moved && !focused {
		x.SetFocus(s, p.ID)
		x.PaneRef(p.ID).Focus()
	}
}

// paneDrag is a pane dragged by its title to another place.
// PaneDrag is a pane dragged by its title to another place.
type PaneDrag struct{ Session, Pane string }

// Zone is where a pane dropped on another goes: beside it, on a side,
// or in its place, which the other takes (swap).
type Zone int

const (
	ZoneNone Zone = iota
	ZoneLeft
	ZoneRight
	ZoneUp
	ZoneDown
	ZoneSwap
)

// zoneAt returns the zone under the pointer, at u, v of a pane (0 to 1):
// its middle swaps, else the nearest edge.
func zoneAt(u, v float32) Zone {
	if u > 0.3 && u < 0.7 && v > 0.3 && v < 0.7 {
		return ZoneSwap
	}
	best, zone := u, ZoneLeft
	for _, e := range []struct {
		d float32
		z Zone
	}{{1 - u, ZoneRight}, {v, ZoneUp}, {1 - v, ZoneDown}} {
		if e.d < best {
			best, zone = e.d, e.z
		}
	}
	return zone
}

func (z Zone) side() string {
	return [...]string{"", "left", "right", "up", "down", ""}[z]
}

// dropFiles takes the files dropped on the body of the pane p, from this
// machine: one of another machine's puts them where its program runs, and
// pastes their paths there; one of this machine's pastes their paths, as
// they are (drop_upload.go). It reports whether the pane shows a drop.
func dropFiles(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) bool {
	body := ui.Box(c.Key("drop")).Absolute().Fill().PassThrough()
	files := body.DroppedFiles()
	over := body.FileDragOver()
	if len(files) > 0 {
		x.RunLater(func() { x.PasteDrop(p.ID, files) })
	}
	return over
}

// dropPreview shows, over the pane under the pointer, the room the
// dragged pane would take, gliding from zone to zone, with what dropping
// does.
func dropPreview(x App, c *ui.Context, pal *theme.Palette, target string, zone Zone) {
	x.DrawnPreview()
	preview := x.Preview()
	since := preview.Since
	if preview.Key != target {
		since = c.Now()
	}
	x.SetPreview(target, since)
	// left, top, width, height in percent of the pane
	r := [...][4]float32{
		ZoneLeft:  {0, 0, 50, 100},
		ZoneRight: {50, 0, 50, 100},
		ZoneUp:    {0, 0, 100, 50},
		ZoneDown:  {0, 50, 100, 50},
		ZoneSwap:  {0, 0, 100, 100},
	}[zone]
	label := map[Zone]string{
		ZoneLeft: locales.PaneSplitLeft(), ZoneRight: locales.ActionSplitRight(), ZoneUp: locales.PaneSplitUp(), ZoneDown: locales.ActionSplitDown(), ZoneSwap: locales.PaneSwap(),
	}[zone]
	dropIcon := glyph.SplitRight
	if zone == ZoneUp || zone == ZoneDown {
		dropIcon = glyph.SplitDown
	}
	if zone == ZoneSwap {
		dropIcon = glyph.Swap
	}
	fadeIn := min(float32(c.Now().Sub(since))/float32(140*time.Millisecond), 1)
	if fadeIn < 1 {
		c.AnimationFrame()
	}
	spring := ui.Animation{Spring: ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: 0.12}}
	o := ui.Column(c).Key("drop-preview").Absolute().PassThrough().Padding(8).Opacity(fadeIn)
	o.LeftPercent(o.AnimateTo("l", r[0], spring)).TopPercent(o.AnimateTo("t", r[1], spring)).
		WidthPercent(o.AnimateTo("w", r[2], spring)).HeightPercent(o.AnimateTo("h", r[3], spring))
	o.Children(func() {
		room := ui.Column(c).Grow(1).Radius(10).AlignItems(ui.Center).
			Background(pal.Accent.Alpha(0.10)).InsetShadow(0, 0, 28, 0, pal.Accent.Alpha(0.22))
		room.Children(func() {
			// The label sits high in the room, clear of the title the
			// pointer carries through its middle.
			ui.Box(c).HeightPercent(22).Shrink(0)
			pill := ui.Row(c).Height(30).Padding(0, 12).Gap(7).Radius(15).AlignItems(ui.Center).
				Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
			style.Floating(pill, pal).Children(func() {
				ui.Icon(c, dropIcon).Size(14, 14)
				ui.Text(c, label).FontSize(12.5).FontWeight(600).SingleLine()
			})
			pill.AnimatePose(ui.Pose{Scale: 0.9 + 0.1*fadeIn}, ui.Snappy)
		})
	})
}

// movePane puts a pane dropped on another beside it, or in its place.
func movePane(x App, sid, pane, target string, zone Zone) {
	s := x.Session(sid)
	if s == nil || s.Layout == nil || zone == ZoneNone {
		return
	}
	if zone == ZoneSwap {
		s.Layout = s.Layout.Swap(pane, target)
		x.SetArrived(target)
	} else {
		s.Layout = s.Layout.Move(pane, target, zone.side())
	}
	x.SetArrived(pane)
	s.Zoomed = ""
	x.SetFocus(s, pane)
	x.PaneRef(pane).Focus()
	x.CallPane(sid, "layout", proto.LayoutParams{Session: sid, Layout: s.Layout.Clone()}, nil)
}

// paneHeader builds a pane's header, and reports whether its title is
// being dragged.
func paneHeader(x App, c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, p *proto.Pane, top, left, right, multi, focused, hovered bool) (dragging bool) {
	pad := float32(16)
	if top && left {
		// With the sidebar hidden, the header at the window's top left
		// starts after the traffic lights and the sidebar's buttons,
		// which come here.
		pad = x.ChromeForPanel().HeaderPad
	}
	h := ui.Row(c.Key("hdr")).Height(x.HeaderHeightIn()).Shrink(0).AlignItems(ui.Center).Padding(0, 10, 0, pad).Gap(8)
	if h.Pressed() && !focused {
		x.SetFocus(s, p.ID)
		x.PaneRef(p.ID).Focus()
	}
	h.Children(func() {
		if top && left {
			SidebarButtons(x, c, pal, bar, shown)
		}
		// The title is the pane's handle: dragged, it takes the pane to
		// another place in the session.
		grip := ui.Row(c.Key("grip")).Gap(8).AlignItems(ui.Center).Shrink(1).MinWidth(0).Height(28).Padding(0, 8).Margin(0, 0, 0, -8).Radius(8)
		if multi {
			grip.Drag(PaneDrag{s.ID, p.ID}).Tooltip(locales.PaneDragTip())
			dragging = grip.Dragging()
			switch {
			case dragging:
				// What follows the pointer is a copy of the grip: a chip.
				style.Floating(grip.Background(pal.Popover), pal)
			case grip.Hovered():
				grip.Background(pal.RowHover).Cursor(ui.CursorGrab)
			}
		}
		bare := p.TimelinePane() || p.SubagentsPane()
		grip.Children(func() {
			// A timeline's header names the agent whose turns it shows,
			// and wears no program's icon: the pane runs none. The
			// subagents' wears none either, and names the view.
			mark := p
			if p.TimelinePane() {
				if q := s.TimelineShown(p); q != nil {
					mark = q
				}
			}
			if multi && !bare {
				if p.Status != proto.Idle || p.Unread {
					paints.StatusP(c, pal, p.Status, p.Unread, p.Progress, 13)
				} else {
					ui.Icon(c, glyph.ProgramIcon(mark)).Size(14, 14).TextColor(pal.TextFaint)
				}
			}
			title, size := p.PaneName(), float32(13.5)
			if p.TimelinePane() && mark.Agent != "" {
				title = mark.Agent
			}
			if p.SubagentsPane() {
				title = locales.SideSubagents()
			}
			if !multi {
				title, size = s.DisplayName(x.Settings().SessionName), 14
			}
			name := ui.Text(c, title).FontSize(size).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(30)
			if multi && !focused && !dragging {
				name.TextColor(pal.TextMuted)
			}
			if !multi && name.DoubleClicked() {
				x.StartRename(s.ID)
			}
			// The home directory names itself "~", which the title may
			// already say. A timeline names the agent alone: the pane it
			// shows sits beside it with the directory and the branch.
			if !bare {
				if dir := x.HostOf(p.ID).ShortDir(p.WorkDir()); dir != title {
					ui.Text(c, dir).FontSize(13).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(2).MinWidth(0)
				}
			}
		})
		if p.Git != nil && !bare {
			x.GitChip(c, pal, s, p)
		}
		// The header's empty stretch moves the window, as a title bar
		// does, and zooms it on a double click: the header around it
		// takes clicks, which focus the pane.
		gap := ui.Box(c.Key("drag")).Grow(1).AlignSelf(ui.Stretch)
		if top {
			gap.DragWindow()
		}
		paneChips(x, c, pal, p, !multi)
		PaneControls(x, c, pal, s, p, (hovered || focused && multi) && !dragging, multi)
		if top && right {
			x.WindowCtl(c, pal)
		}
	})
	return dragging
}

// SidebarButtons are the sidebar's buttons, which the header at the
// window's top left holds while the sidebar hides.
func SidebarButtons(x App, c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32) {
	if shown >= 0.5 {
		return
	}
	ui.Row(c).Absolute().Left(x.ChromeForPanel().ButtonsLeft()).Top(0).Bottom(0).AlignItems(ui.Center).Gap(2).Opacity(1 - 2*shown).Children(func() {
		paints.IconButton(c, "show-sidebar", glyph.Sidebar, x.Tip(locales.SidebarShow(), "sidebar.toggle"), pal, x.ToggleSidebar)
		paints.IconButton(c, "new-session-2", glyph.Plus, x.Tip(locales.ActionNewSession(), "session.new"), pal, func() { x.NewSessionIn("") })
	})
}

// PaneControls are a header's buttons acting on its pane, which show
// while the pointer is over it.
func PaneControls(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane, show, multi bool) {
	ctl := ui.Row(c.Key("controls")).Gap(0).AlignItems(ui.Center)
	ctl.Opacity(ctl.Animate("shown", b2f(show), 120*time.Millisecond)).Children(func() {
		if !show {
			return
		}
		// The chat takes a terminal's place, and only a Feishu chat drives
		// a pane: the turn it drives offers the button while it runs or
		// waits, as does a pane whose chat shows, the terminal a click
		// away. The turn over, the button goes.
		if x.FeishuOn() && (p.FeishuChat() || x.PaneChat()[p.ID]) {
			if x.PaneChat()[p.ID] {
				paints.IconButton(c, "chat-toggle", glyph.Terminal, x.Tip(locales.ActionShowTerminal(), "pane.chat"), pal, func() { hidePaneChat(x, s, p.ID) })
			} else {
				paints.IconButton(c, "chat-toggle", glyph.Chat, x.Tip(locales.SideChat(), "pane.chat"), pal, func() { showPaneChat(x, s, p.ID) })
			}
		}
		paints.IconButton(c, "split-r", glyph.SplitRight, x.Tip(locales.ActionSplitRight(), "pane.split.right"), pal, func() { SplitPane(x, s, p.ID, "right") })
		paints.IconButton(c, "split-d", glyph.SplitDown, x.Tip(locales.ActionSplitDown(), "pane.split.down"), pal, func() { SplitPane(x, s, p.ID, "down") })
		if multi {
			sid, pid := s.ID, p.ID
			zoom := func() { x.RunLater(func() { ZoomPane(x, x.Session(sid), pid) }) }
			if s.Zoomed == p.ID {
				paints.IconButton(c, "zoom", glyph.Unzoom, x.Tip(locales.PaneShowAll(), "pane.zoom"), pal, zoom)
			} else {
				paints.IconButton(c, "zoom", glyph.Zoom, x.Tip(locales.ActionZoomPane(), "pane.zoom"), pal, zoom)
			}
			paints.IconButton(c, "close", glyph.Close, x.Tip(locales.PaneClose(), "pane.close"), pal, func() {
				x.RunLater(func() { ClosePane(x, x.Session(sid), pid, true) })
			})
		}
	})
}

// showPaneChat shows the focused pane's chat in its terminal's place:
// the window shows the pane alone, so that the chat has its place.
// ShowPaneChatView shows a pane's chat in its terminal's place, as the
// pane's controls do.
func ShowPaneChatView(x App, s *proto.Session, id string) { showPaneChat(x, s, id) }

func showPaneChat(x App, s *proto.Session, id string) {
	x.ShowPaneChat(id)
	// Zoomed is its own zoom, so the call is the pane's place, not a
	// toggle of it.
	if s.Zoomed == "" && (len(s.Panes) > 1 || s.Layout != nil) {
		s.Zoomed = id
		x.SetFocus(s, id)
		x.CallPane(s.ID, "zoom", proto.FocusParams{Session: s.ID, Pane: id}, nil)
	}
}

// hidePaneChat shows the focused pane's terminal again, and the panes
// side by side, zoomed only for the chat.
func hidePaneChat(x App, s *proto.Session, id string) {
	x.HidePaneChat(id)
	if s.Zoomed == id {
		ZoomPane(x, s, id)
	}
}

// paneChips says what the pane is doing. With several panes the title
// already leads with the status icon, so the chips go without (icon false).
func paneChips(x App, c *ui.Context, pal *theme.Palette, p *proto.Pane, icon bool) {
	switch p.Status {
	case proto.Waiting:
		msg := p.Message
		if msg == "" {
			msg = map[string]string{"permission": locales.PaneNeedsPermission(), "question": locales.PaneHasQuestion(), "auth": locales.PaneNeedsSignIn()}[p.Kind]
		}
		if msg == "" {
			msg = locales.StatusNeedsYou()
		}
		chip(c, pal, pal.AttnSoft, pal.Attention, func() {
			if icon {
				paints.Status(c, pal, p.Status, p.Unread, 11)
			}
			ui.Text(c, msg).FontSize(12).FontWeight(500).SingleLine().Ellipsis("…").MaxWidth(320)
		})
	case proto.Working:
		chip(c, pal, pal.Chip, pal.TextMuted, func() {
			if icon {
				paints.StatusP(c, pal, p.Status, false, p.Progress, 11)
			}
			label := paints.Ago(p.Since, c.Now())
			if p.Progress >= 0 {
				label = itoa(p.Progress) + "%"
			}
			if p.Message != "" {
				label = p.Message + " · " + label
			}
			ui.Text(c, label).FontSize(12).FontFeatures("tnum").SingleLine().Ellipsis("…").MaxWidth(320)
		})
	case proto.Failed:
		chip(c, pal, pal.Danger.Alpha(0.1), pal.Danger, func() {
			if icon {
				paints.Status(c, pal, p.Status, true, 11)
			}
			msg := p.Message
			if msg == "" {
				msg = locales.StatusFailed()
			}
			ui.Text(c, msg).FontSize(12).FontWeight(500).SingleLine().Ellipsis("…").MaxWidth(320)
		})
	case proto.Exited:
		chip(c, pal, pal.Danger.Alpha(0.1), pal.Danger, func() {
			ui.Text(c, locales.PaneExited(itoa(p.ExitCode))).FontSize(12).FontWeight(500).SingleLine()
		})
		if style.Button(c, "restart-"+p.ID, locales.CommonRestart(), "", false, pal) {
			x.RestartSession(p.ID)
		}
	case proto.Idle:
		if p.Message != "" {
			chip(c, pal, pal.Chip, pal.TextMuted, func() {
				if icon {
					paints.Status(c, pal, p.Status, p.Unread, 11)
				}
				ui.Text(c, p.Message).FontSize(12).SingleLine().Ellipsis("…").MaxWidth(320)
			})
		}
	}
	if p.Process != "" && p.Status != proto.Exited {
		chip(c, pal, pal.Chip, pal.TextMuted, func() {
			ui.Text(c, p.Process).FontSize(12).SingleLine()
		}).Tooltip(locales.PaneForegroundTip())
	}
}

// paneTerminal is a pane's terminal, under its header, or why it shows
// none.
func paneTerminal(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	x.EnsureTerm(p.ID)
	// The terminal fills the card under the header: its own padding
	// takes the colors of the cells beside it, so that a full-screen
	// program's background reaches the card's edges.
	ui.Box(c.Key("body")).Grow(1).MinHeight(0).Children(func() {
		var term *terminal.Terminal
		attaching, err, started := false, "", time.Time{}
		if e := x.Term(p.ID); e != nil {
			term, err, started = e.Term, e.Err, e.Started
		}
		switch {
		case term != nil:
			ref := x.PaneRef(p.ID)
			terminal.View(c.Key("term"), term).Fill().Bind(ref)
			if p.Status == proto.Exited {
				exitedBar(x, c, pal, s, p)
			}
			if link, isPath, at := term.HoveredLink(); isPath {
				x.ImagePeek(p.ID, link, at, ref.Bounds(c))
			}
			if ref.Focused(c) && x.FocusOf(s) != p.ID {
				x.SetFocus(s, p.ID)
			}
			if x.FindPane() == p.ID {
				x.FindBar(c, pal, p, term)
			}
			_ = attaching
		case err != "":
			x.PaneMessage(c, pal, locales.PaneCantShow(), err, locales.CommonTryAgain(), func() {
				x.DropTerms(p.ID)
				x.EnsureTerm(p.ID)
			})
		default:
			// Attaching takes a moment for a long scrollback: a spinner
			// only once it is a noticeable one.
			if !started.IsZero() && time.Since(started) > 250*time.Millisecond {
				ui.Column(c).Fill().Center().Children(func() { ui.Spinner(c).Label(locales.PaneAttaching()) })
			} else {
				c.After(260 * time.Millisecond)
			}
		}
	})
}

// exitedBar says, over the last of the pane's screen, that its shell exited,
// and gives the way to start it again: a dead terminal takes no input, and
// the screen alone says nothing of what to do.
func exitedBar(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	ui.Row(c).Absolute().Left(8).Right(8).Bottom(8).MinHeight(34).Radius(10).Padding(0, 10, 0, 12).Gap(8).
		AlignItems(ui.Center).Background(pal.Popover).Children(func() {
		ui.Text(c, locales.PaneExited(itoa(p.ExitCode))).FontSize(12).FontWeight(500).TextColor(pal.Text).SingleLine().Shrink(0)
		ui.Text(c, locales.PaneExitedDetail()).FontSize(11.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		ui.Spacer(c)
		if style.Button(c, "restart-"+p.ID, locales.CommonRestart(), "", true, pal) {
			x.RunLater(func() { x.RestartSession(p.ID) })
		}
		if style.Button(c, "close-exited-"+p.ID, locales.PaneClose(), "", false, pal) {
			x.RunLater(func() { ClosePane(x, s, p.ID, false) })
		}
	})
}

// chip draws a small pill of a pane's state, in given colors.
func chip(c *ui.Context, pal *theme.Palette, bg, fg ui.Color, fn func()) ui.Element {
	return ui.Row(c).Height(22).Padding(0, 9).Gap(6).Radius(11).AlignItems(ui.Center).Background(bg).TextColor(fg).Children(fn)
}

// itoa is an int in decimal.
func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// b2f is a bool as 0 or 1, for animations.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// Itoa is an int in decimal.
func Itoa(n int) string { return itoa(n) }
