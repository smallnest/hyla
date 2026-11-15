package main

import (
	"hyla/internal/app/host"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/layout"
	"hyla/internal/ui/theme"

	"hyla/internal/ui/side"
	"hyla/internal/ui/style"
	"hyla/locales"
)

func (a *App) view(c *ui.Context) {
	pal := theme.For(c)
	c.SetTheme(borderless(c, pal))

	if a.UndoAction != nil {
		// The removal's own words, even as another toast came first.
		c.ToastAction(a.UndoMessage, locales.CommonUndo(), a.UndoAction)
		a.PendingToast, a.UndoAction, a.UndoMessage = "", nil, ""
	} else if a.PendingToast != "" {
		c.Toast(a.PendingToast)
		a.PendingToast = ""
	}
	a.shortcuts(c)
	a.State.Tick(a)

	bar := c.TitleBar()
	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	theme.Backdrop(c, pal, row, a.settingsIn)
	// A curve, not a spring: a spring's long tail crawls under a pixel a
	// frame, which whole pixels turn into hops and holds, a shake as it
	// settles. This one lands in about 13 frames, moving in every one.
	var progress float32
	shown := a.ChromeState.Shown
	// The panel is not crowded by a window too narrow for it beside the
	// panes, which it then leaves to them while it is open, unless the
	// user opens it anyway (sideState.forced). The row's width is this
	// frame's, as its bounds have it: the last frame's would lag a frame
	// as the window resizes.
	winW, _ := c.Size()
	a.Bar = bar
	a.Side.Crowded = winW > 0 && sideRowW(winW, a.ChromeState) < side.MinCards
	a.Side.RowW = sideRowW(winW, a.ChromeState)
	if !a.Side.Crowded {
		a.Side.Forced = false
	}
	row.Children(func() {
		progress = ui.Column(c.Key("sidebar")).AnimateTo("shown", layout.B2F(!a.Prefs.SidebarHidden), layout.SidebarMotion)
		a.ChromeState = layout.For(progress, a.Prefs.SidebarWidth, bar)
		w := a.ChromeState.SideW
		side := ui.Row(c).Width(w).Shrink(0).AlignItems(ui.Stretch).ClipX()
		if h := a.local(); h.Conn == host.StateReady && h.Client != nil && !a.Prefs.SidebarHidden && !a.Pal.Open && !a.Picker.Open && a.SshAsk == nil && a.K8sAsk == nil && (a.Confirm == nil || !a.Confirm.Open) {
			if files := side.DroppedFiles(); len(files) > 0 {
				go a.openProjectDirs(h, h.Client, slices.Clone(files), true)
			}
			if side.FileDragOver() {
				side.Background(pal.Accent.Alpha(0.08)).Draw(func(p *ui.Painter, r ui.Rect) {
					p.Stroke(ui.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 8}, pal.Accent.Alpha(0.55), 8, 1)
				})
			}
		}
		side.Children(func() {
			ui.Column(c).Width(a.Prefs.SidebarWidth).Shrink(0).Margin(0, 0, 0, w-a.Prefs.SidebarWidth).
				Opacity(0.4 + 0.6*shown).Children(func() {
				a.sidebarArtView(c)
				a.sidebarView(c, pal, bar)
			})
		})
		// The gaps around the cards move the window too.
		main := ui.Column(c).Grow(1).MinWidth(0).Padding(layout.CardGap, layout.CardGap, layout.CardGap, a.ChromeState.MainPad).DragWindow()
		main.Children(func() {
			if shown > 0.01 {
				a.resizeHandle(c, pal)
			}
			layout.Card(a, c, pal, bar, shown, a.Selected)
		})
	})
	// A page staged for a screenshot takes the window's size, under its
	// content, which hides it: at no opacity, it paints no hole there.
	for id, e := range a.Webs {
		if e.Staged && e.View != nil {
			ui.Box(c.Key("staged-" + id)).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Stretch).Opacity(0).Children(func() {
				ui.WebView(c.Key("web"), e.View).Grow(1)
			})
		}
	}
	a.paletteView(c, pal)
	a.pickerView(c, pal)
	a.confirmView(c, pal)
	a.worktreeView(c, pal)
	a.sshView(c, pal) // the dialog that adds a machine
	a.k8sView(c, pal)
	// A preview that showed no more fades in anew next time.
	if !a.PreviewDrawn {
		a.PreviewPane = ""
	}
	a.PreviewDrawn = false
	// An image previewed no more closes, and lets go of its bitmap.
	a.endPeek()
	if later := a.Later; len(later) > 0 {
		a.Later = nil
		for _, fn := range later {
			fn()
		}
		c.Invalidate()
	}
}

// resizeHandle is the sidebar's edge, which a drag moves.
func (a *App) resizeHandle(c *ui.Context, pal *theme.Palette) {
	// A drag leaves the keyboard in the pane the user is in.
	h := ui.Box(c.Key("resize")).Absolute().Left(-4).Top(0).Bottom(0).Width(9).Cursor(ui.CursorResizeColumn).KeepFocus()
	if dx, _, ok := h.Dragged(); ok {
		if a.SideDrag == 0 {
			a.SideDrag = a.Prefs.SidebarWidth
		}
		a.SideDrag += dx
		a.Prefs.SidebarWidth = min(max(a.SideDrag, minSidebar), maxSidebar)
	} else if a.SideDrag != 0 && !h.Pressed() {
		a.SideDrag = 0
		a.savePrefs()
	}
	if h.DoubleClicked() {
		a.Prefs.SidebarWidth = defaultSidebar
		a.savePrefs()
	}
}

// --- the sidebar ---
// --- the card ---

// sessionSwitch is how a session shown in place of another, as when the
// user moves from one session to the next, comes, as a slide show's
// transitions do: it fades in as the session it leaves, as the frame
// before showed it, blurs and fades away over it (dissolve); it slides in
// from the right (slide, push, cover); it grows into place (zoom); or it
// shows at once (none).
//
// The dissolve is a transition of the mygo view: the copy leaving takes
// neither the pointer nor the focus, but it stays in the chain a press
// goes down, so a moving session fades under a still pointer alone where
// the pointer is not. The others move the panes as their poses go, from
// where the switch brings them, to their place, which the frames after
// the one that brings them in pose (switchTo, settle, panes).
type sessionSwitch int

const (
	switchNone sessionSwitch = iota
	switchDissolve
	switchSlide
	switchPush
	switchCover
	switchZoom
)

// sessionSwitch returns how a session comes in place of another, as the
// settings say (session-switch).
func (a *App) sessionSwitch() sessionSwitch {
	switch a.settingsIn.SessionSwitch {
	case "slide":
		return switchSlide
	case "push":
		return switchPush
	case "cover":
		return switchCover
	case "zoom":
		return switchZoom
	case "none":
		return switchNone
	}
	return switchDissolve
}

// transition returns the transition of the frame that holds a session's
// panes: the dissolve fades one in as the one it leaves, as the frame
// before showed it, blurs and fades away over it.
func (sw sessionSwitch) transition() ui.ElementTransition {
	if sw != switchDissolve {
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
func (sw sessionSwitch) motion() ui.Motion {
	switch sw {
	case switchSlide, switchPush, switchCover:
		return ui.Motion{XPercent: 100}
	case switchZoom:
		return ui.Motion{Opacity: 1, Scale: 0.9}
	}
	return ui.Motion{}
}

// pose returns where the panes show as a switch comes: if in, away from
// their place, as from says, faded out where from fades them in, as a
// Pose's Fade of 1 does; else where they go, there.
func (sw sessionSwitch) pose(from ui.Motion, in bool) ui.Pose {
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

func (a *App) message(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func()) {
	ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
		ui.Text(c, title).FontSize(17).FontWeight(600).TextColor(pal.Text)
		ui.Text(c, detail).FontSize(13).TextColor(pal.TextMuted).MaxWidth(420).TextAlign(ui.Center).LineHeight(1.4)
		if action != "" {
			ui.Box(c).Height(8)
			if style.Button(c, "msg-action", action, "", true, pal) {
				fn()
			}
		}
	})
}
