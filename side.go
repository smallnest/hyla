package main

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/changesview"
	"hyla/internal/ui/files"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/theme"

	"hyla/internal/ui/chat"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/style"
	"hyla/locales"
)

// The side panel stands at the right of a session's panes, a card of its
// own, with what the session's worktree is about. It has views, each a
// list whose item opens in the panel as a file: Changes, the files the
// session's agent or the user changed, each opening as its diff; and
// Files (files.go), the worktree's tree, each file opening as its text.

// The views' ids, as prefs.SideTab holds one.
const (
	sideChanges = "changes"
	sideFiles   = "files"
	sideInfo    = "info"
)

const (
	defaultSide = 420 // the panel's width, as it first opens
	minSide     = 300
	minPanes    = 360 // the room the panel leaves the panes
	splitWidth  = 760 // past which diff-layout auto shows diffs side by side
)

// sideState is the panel's, beside the prefs that open it and size it.
type sideState struct {
	drag    float32 // the width as a drag of the edge began, 0 while none
	focused bool    // the panel has the keyboard: its Escape, its diff's find
	view    string  // the view shown last, kept as the panel closes
	// crowded tells that the window is too narrow for the panel beside
	// the panes (minCards), which it then leaves to them while it is
	// open, unless the user opens it anyway, forced, until the window
	// widens.
	crowded, forced bool
	// rowW is the width of the row of the panes and the panel this frame,
	// of which the panel's comes (sideRowW).
	rowW    float32
	changes changesview.State
	files   files.State
	// fileOpeners tells that the left control of the header is pressed,
	// which shows the viewer, the file open, over the panel.
	fileOpeners bool
	info        infoState
	chat        chat.State
}

// infoState is the Info view's, which keeps its place as the views change.
type infoState struct {
	scroll ui.ScrollState
}

// toggleSide opens or closes the panel, whatever its view: closed, it
// opens on view, or on the view the user saw last.
func (a *App) toggleSide(view string) {
	switch {
	case !a.rightShown():
		// Closed: it opens on view, or on the view seen last.
		v := view
		if v == "" {
			v = a.Side.view
		}
		if v == "" {
			v = sideChanges
		}
		a.Side.view, a.Prefs.Side, a.Prefs.SideTab = v, v, v
		// Shown anyway, though the window crowds it.
		a.Side.forced = a.Side.crowded
	case view == "" || a.sideTab() == view:
		a.Side.view, a.Prefs.SideTab = a.sideTab(), a.sideTab()
		a.Prefs.Side = ""
		a.Side.forced = false
		a.Side.focused = false
		a.focusTerminal()
	default:
		// Another view chosen: the panel turns to it.
		a.Side.view, a.Prefs.Side, a.Prefs.SideTab = view, view, view
	}
	a.savePrefs()
}

// sideTab is the tab the panel shows, or showed last while it hides.
func (a *App) sideTab() string {
	tab := a.Prefs.Side
	if tab == "" {
		tab = a.Prefs.SideTab
	}
	if tab != sideFiles && tab != sideInfo {
		tab = sideChanges
	}
	return tab
}

// rightShown tells that the panel shows.
func (a *App) rightShown() bool { return a.Prefs.Side != "" && (!a.Side.crowded || a.Side.forced) }

// minCards is the room the panel leaves the panes: in a window narrower,
// it gets out of their way (sideState.crowded).
const minCards = 420

// hiddenSideWidth is the panel's width it adds to a window that hides it.
const hiddenSideWidth = 24

// sideView is the view the panel shows, or would show when opened: the
// last it showed, Changes for none.
func (a *App) sideView() string {
	if a.Prefs.Side != "" {
		return a.Prefs.Side
	}
	if a.Side.view != "" {
		return a.Side.view
	}
	return sideChanges
}

// showSide shows a view of the panel, keeping it open when it shows it.
func (a *App) showSide(view string) {
	if a.Prefs.Side != view || a.Prefs.SideTab != view {
		a.Prefs.Side, a.Prefs.SideTab = view, view
		a.savePrefs()
	}
}

// sideRowW is the width of a row of the panes and the panel in a window
// winW wide, of which the panel's comes.
func sideRowW(winW float32, ch chrome) float32 {
	if winW <= 0 {
		return 0
	}
	return winW - ch.sideW - cardGap - ch.mainPad + hiddenSideWidth
}

// sideWidth is the panel's width in a row of width w, leaving the panes
// their room.
func (a *App) sideWidth(w float32) float32 {
	sw := a.Prefs.SideWidth
	if sw == 0 {
		sw = defaultSide
	}
	if w > 0 {
		// The panel's own gap from the row's edge is its margin.
		sw = min(sw, w-minPanes-sideGap)
	}
	return max(sw, minSide)
}

// sideEdge is the gap between the panes and the panel, which a drag moves,
// and whose edge takes the pointer on either side of it, over the panes
// and the panel too, which would take it from all but its gap.
func (a *App) sideEdge(c *ui.Context, pal *theme.Palette, row float32) {
	d := ui.Box(c.Key("side-edge")).Absolute().Top(0).Bottom(0).Width(sideGap + 8).Right(a.sideWidth(row)).
		Cursor(ui.CursorResizeColumn).KeepFocus()
	if d.Hovered() || d.Pressed() {
		d.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X + r.W/2 - 1.5, Y: r.Y + r.H/2 - 18, W: 3, H: 36}, pal.TextFaint.Alpha(0.7), 1.5)
		})
	}
	if dx, _, ok := d.Dragged(); ok {
		if a.Side.drag == 0 {
			a.Side.drag = a.sideWidth(row)
		}
		a.Side.drag -= dx
		a.Prefs.SideWidth = min(max(a.Side.drag, minSide), max(row-minPanes, minSide))
	} else if a.Side.drag != 0 && !d.Pressed() {
		a.Side.drag = 0
		a.savePrefs()
	}
	if d.DoubleClicked() {
		a.Prefs.SideWidth = defaultSide
		a.savePrefs()
	}
}

// sidePanel is the panel's card: its header, then its view. top and right
// tell that it touches the window's top and right edges, where its header
// drags the window and holds the window's controls (windowControls).
func (a *App) sidePanel(c *ui.Context, pal *theme.Palette, s *proto.Session, width float32, right bool) {
	card := ui.Column(c.Key("side")).Width(width).Shrink(0).MinHeight(0).Radius(14).Clip().Background(theme.TranslucentGlass(pal.Card, pal, a.settingsIn)).
		Focusable().Bind(a.paneRef("side-card"))
	style.Raised(card, pal)
	a.Side.focused = card.FocusWithin()
	// It slides in from the window's edge, past which it starts, and
	// fades as it comes, and out the same way as it closes, as the
	// sidebar does.
	in := card.AnimateTo("in", b2f(a.rightShown()), sidebarMotion)
	card.Opacity(in)
	card.Margin(0, (1-in)*cardGap, 0, sideGap)
	view, set := a.sideView(), ""
	card.Children(func() {
		ui.Row(c.Key("hdr")).Height(headerH).Shrink(0).AlignItems(ui.Center).Padding(0, 10, 0, 16).Gap(8).Children(func() {
			// The views, the one shown marked, as a pane's header shows
			// its program.
			a.sideTabButton(c, pal, sideInfo, locales.SideInfo(), glyph.Info, view == sideInfo, "info.toggle", func() { set = sideInfo })
			a.sideTabButton(c, pal, sideChanges, locales.SideChanges(), glyph.Diff, view == sideChanges, "changes.toggle", func() { set = sideChanges })
			a.sideTabButton(c, pal, sideFiles, locales.SideFiles(), glyph.Folder, view == sideFiles, "files.toggle", func() { set = sideFiles })
			if set != "" {
				a.defer_(func() { a.showSide(set) })
			}
			// The header names no folder: that is the Info view's first
			// card's title, the Files view's tree's root, and the Changes
			// view's worktree it lists; the tabs alone leave its buttons
			// their room.
			ui.Box(c.Key("drag")).Grow(1).AlignSelf(ui.Stretch).DragWindow()
			if view == sideFiles && a.Side.files.Views[s.ID] != nil {
				paints.IconButton(c, "files-toggle-view", glyph.Doc, a.tip(locales.FilesCloseFile(), "files-close"), pal, func() {
					a.defer_(func() {
						if fv := a.Side.files.Views[s.ID]; fv != nil {
							fv.Open = false
						}
					})
				})
			}
			paints.IconButton(c, "close-side", glyph.Close, a.tip(locales.SideClose(), "changes.toggle"), pal, func() { a.defer_(func() { a.toggleSide("") }) })
			if right {
				windowControls(c, pal, a.Win)
			}
		})
		ui.Column(c.Key("body")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			switch view {
			case sideFiles:
				a.filesSide(c, pal, s)
				return
			case sideInfo:
				a.infoView(c, pal, s)
				return
			}
			a.ChangesView(c, pal, s, width)
		})
		// The viewer of a file of Files, over the tab while its button in
		// the header, its icon, tells so.
		if fv := a.Side.files.Views[s.ID]; view == sideFiles && fv != nil && fv.Open {
			ui.Column(c.Key("file")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
				a.fileViewer(c, pal, s, fv)
			})
		}
	})
	if card.Shortcut(0, ui.KeyEscape) {
		a.focusTerminal()
	}
}

// sideTabButton is a view's name in the panel's header, a click showing it: the
// one shown in the text's color, the others faint. The header drags the
// window where no tab is, as the window controls are, which the tabs are
// part of as windowControls' buttons are.
func (a *App) sideTabButton(c *ui.Context, pal *theme.Palette, id, label string, svg *ui.SVG, on bool, action string, fn func()) {
	fg := pal.TextFaint
	if on {
		fg = pal.Text
	}
	b := ui.ButtonBase(c.Key("tab-"+id)).Height(26).Gap(5).Padding(0, 7).Radius(6).AlignItems(ui.Center).Shrink(0).
		Cursor(ui.CursorPointer).Label(a.tip(label, action))
	if on {
		b.Background(pal.Chip)
	} else if b.Hovered() {
		b.Background(pal.RowHover)
	}
	if b.Clicked() {
		fn()
	}
	b.Children(func() {
		ui.Icon(c, svg).Size(14, 14).TextColor(fg)
		ui.Text(c, label).FontSize(14).FontWeight(600).TextColor(fg).SingleLine().Shrink(0)
	})
}
