package side

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/changesview"
	"hyla/internal/ui/chat"
	"hyla/internal/ui/files"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// The views' ids, as prefs.SideTab holds one.
const (
	Changes = "changes"
	Files   = "files"
	Info    = "info"
)

const (
	// Default is the panel's width, as it first opens, and Min the
	// narrowest it takes; MinPanes is the room it leaves the panes.
	Default  = 420
	Min      = 300
	MinPanes = 360
	// HiddenWidth is the width it adds to a window that hides it.
	HiddenWidth = 24
	// MinCards is the room the panel leaves the panes: in a window
	// narrower, it gets out of their way (State.Crowded).
	MinCards = 420
)

// App is what the panel asks of the app it shows in.
type App interface {
	// Panel is the panel's state.
	Panel() *State
	// SideVisible is the view the panel shows, SideTabPref the view it
	// showed last, and SideWidthPref its width; SavePrefs keeps them.
	SideVisible() *string
	SideTabPref() *string
	SideWidthPref() *float32
	SavePrefs()
	// FileViewer is a file of Files, open over the panel.
	FileViewer(c *ui.Context, pal *theme.Palette, s *proto.Session, v *files.View)
	// FilesState is the Files view's state.
	FilesState() *files.State
	// PaneRef is the handle a pane's terminal binds.
	PaneRef(id string) *ui.Handle
	// RunLater runs fn after the frame.
	RunLater(fn func())
	// Tip adds an action's shortcut, in words, to a title.
	Tip(title, id string) string
	// FocusTerminal gives the keyboard to the session's terminal.
	FocusTerminal()
	// WindowControls draws the window's buttons on the right of its title.
	WindowControls(c *ui.Context, pal *theme.Palette)
	// Motion is how the panel slides in.
	Motion() ui.Animation
	// CardGap, HeaderHeight and Gap are the room between the cards and
	// the window's edge, the headers' height, and the gap between the
	// panes and the panel.
	CardGapIn() float32
	HeaderHeightIn() float32
	Gap() float32
	// SettingsOf is the settings the panel draws with, and CardGapIn is
	// the room between a card and the window's edge, which RowW takes.
	SettingsOf() *settings.Settings
	// ChangesSide, FilesSide and InfoSide are the views, in a session's
	// pane.
	ChangesSide(c *ui.Context, pal *theme.Palette, s *proto.Session, width float32)
	FilesSide(c *ui.Context, pal *theme.Palette, s *proto.Session)
	InfoSide(c *ui.Context, pal *theme.Palette, s *proto.Session)
	// PluginTabs lists custom side-panel tabs contributed by enabled plugins.
	PluginTabs(s *proto.Session) []PluginTab
	// PluginSide renders the body of a custom plugin side-panel tab.
	PluginSide(c *ui.Context, pal *theme.Palette, s *proto.Session, tabID string, width float32)
}

// PluginTab describes a custom side-panel tab registered by an enabled plugin.
type PluginTab struct {
	ID    string
	Title string
	Icon  *ui.SVG
}

// State is the panel's, beside the prefs that open it and size it.
type State struct {
	// Drag is the width as a drag of the edge began, 0 while none.
	Drag float32
	// Focused tells that the panel has the keyboard: its Escape, its
	// diff's find.
	Focused bool
	// View is the view shown last, kept as the panel closes.
	View string
	// Crowded tells that the window is too narrow for the panel beside
	// the panes (MinCards), which it then leaves to them while it is
	// open, unless the user opens it anyway, Forced, until the window
	// widens.
	Crowded, Forced bool
	// RowW is the width of the row of the panes and the panel this frame,
	// of which the panel's comes (RowW).
	RowW    float32
	Changes changesview.State
	Files   files.State
	// FileOpeners tells that the left control of the header is pressed,
	// which shows the viewer, the file open, over the panel.
	FileOpeners bool
	Info        ui.ScrollState // the Info view's place
	Chat        chat.State
}

// Toggle opens or closes the panel, whatever its view: closed, it opens
// on view, or on the view the user saw last.
func Toggle(x App, view string) {
	st := x.Panel()
	switch {
	case !RightShown(x):
		// Closed: it opens on view, or on the view seen last.
		v := view
		if v == "" {
			v = st.View
		}
		if v == "" {
			v = Changes
		}
		st.View, *x.SideVisible(), *x.SideTabPref() = v, v, v
		// Shown anyway, though the window crowds it.
		st.Forced = st.Crowded
	case view == "" || Tab(x) == view:
		st.View, *x.SideTabPref() = Tab(x), Tab(x)
		*x.SideVisible() = ""
		st.Forced = false
		st.Focused = false
		x.FocusTerminal()
	default:
		// Another view chosen: the panel turns to it.
		st.View, *x.SideVisible(), *x.SideTabPref() = view, view, view
	}
	x.SavePrefs()
}

// Tab is the tab the panel shows, or showed last while it hides.
func Tab(x App) string {
	tab := *x.SideVisible()
	if tab == "" {
		tab = *x.SideTabPref()
	}
	if tab != Files && tab != Info && !strings.HasPrefix(tab, "plugin:") {
		tab = Changes
	}
	return tab
}

// RightShown tells that the panel shows.
func RightShown(x App) bool {
	st := x.Panel()
	return *x.SideVisible() != "" && (!st.Crowded || st.Forced)
}

// View is the view the panel shows, or would show when opened: the last
// it showed, Changes for none.
func View(x App) string {
	if v := *x.SideVisible(); v != "" {
		return v
	}
	if v := x.Panel().View; v != "" {
		return v
	}
	return Changes
}

// Show shows a view of the panel, keeping it open when it shows it.
func Show(x App, view string) {
	if *x.SideVisible() != view || *x.SideTabPref() != view {
		*x.SideVisible(), *x.SideTabPref() = view, view
		x.SavePrefs()
	}
}

// RowW is the width of a row of the panes and the panel in a window winW
// wide, of which the panel's comes.
func RowW(winW, sideW, mainPad, cardGap float32) float32 {
	if winW <= 0 {
		return 0
	}
	return winW - sideW - cardGap - mainPad + HiddenWidth
}

// Width is the panel's width in a row of width w, leaving the panes their
// room.
func Width(width, gap, w float32) float32 {
	sw := width
	if sw == 0 {
		sw = Default
	}
	if w > 0 {
		// The panel's own gap from the row's edge is its margin.
		sw = min(sw, w-MinPanes-gap)
	}
	return max(sw, Min)
}

// Edge is the gap between the panes and the panel, which a drag moves,
// and whose edge takes the pointer on either side of it, over the panes
// and the panel too, which would take it from all but its gap.
func Edge(x App, c *ui.Context, pal *theme.Palette, row float32) {
	st := x.Panel()
	d := ui.Box(c.Key("side-edge")).Absolute().Top(0).Bottom(0).Width(x.Gap() + 8).Right(Width(*x.SideWidthPref(), x.Gap(), row)).
		Cursor(ui.CursorResizeColumn).KeepFocus()
	if d.Hovered() || d.Pressed() {
		d.Draw(func(pn *ui.Painter, r ui.Rect) {
			pn.Fill(ui.Rect{X: r.X + r.W/2 - 1.5, Y: r.Y + r.H/2 - 18, W: 3, H: 36}, pal.TextFaint.Alpha(0.7), 1.5)
		})
	}
	if dx, _, ok := d.Dragged(); ok {
		if st.Drag == 0 {
			st.Drag = Width(*x.SideWidthPref(), x.Gap(), row)
		}
		st.Drag -= dx
		*x.SideWidthPref() = min(max(st.Drag, Min), max(row-MinPanes, Min))
	} else if st.Drag != 0 && !d.Pressed() {
		st.Drag = 0
		x.SavePrefs()
	}
	if d.DoubleClicked() {
		*x.SideWidthPref() = Default
		x.SavePrefs()
	}
}

// Panel is the panel's card: its header, then its view. right tells that
// it touches the window's right edge, where its header drags the window
// and holds the window's controls.
func Panel(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, width float32, right bool) {
	st := x.Panel()
	card := ui.Column(c.Key("side")).Width(width).Shrink(0).MinHeight(0).Radius(14).Clip().
		Background(theme.TranslucentGlass(pal.Card, pal, *x.SettingsOf())).
		Focusable().Bind(x.PaneRef("side-card"))
	style.Raised(card, pal)
	st.Focused = card.FocusWithin()
	// It slides in from the window's edge, past which it starts, and
	// fades as it comes, and out the same way as it closes, as the
	// sidebar does.
	in := card.AnimateTo("in", b2f(RightShown(x)), x.Motion())
	card.Opacity(in)
	card.Margin(0, (1-in)*x.CardGapIn(), 0, x.Gap())
	view, set := View(x), ""
	card.Children(func() {
		ui.Row(c.Key("hdr")).Height(x.HeaderHeightIn()).Shrink(0).AlignItems(ui.Center).Padding(0, 10, 0, 16).Gap(8).Children(func() {
			// The views, the one shown marked, as a pane's header shows
			// its program. A narrow panel names the one it shows, and
			// leaves the others to their icons, which their titles tell:
			// the header keeps room for its close button and the window's.
			labels := width >= tabLabelsWidth
			tabButton(x, c, pal, Info, locales.SideInfo(), glyph.Info, view == Info, "info.toggle", labels, func() { set = Info })
			tabButton(x, c, pal, Changes, locales.SideChanges(), glyph.Diff, view == Changes, "changes.toggle", labels, func() { set = Changes })
			tabButton(x, c, pal, Files, locales.SideFiles(), glyph.Folder, view == Files, "files.toggle", labels, func() { set = Files })
			for _, pt := range x.PluginTabs(s) {
				svg := pt.Icon
				if svg == nil {
					svg = glyph.Plugins
				}
				pid := pt.ID
				tabButton(x, c, pal, pid, pt.Title, svg, view == pid, "", labels, func() { set = pid })
			}
			if set != "" {
				x.RunLater(func() { Show(x, set) })
			}
			// The header names no folder: that is the Info view's first
			// card's title, the Files view's tree's root, and the Changes
			// view's worktree it lists; the tabs alone leave its buttons
			// their room.
			ui.Box(c.Key("drag")).Grow(1).AlignSelf(ui.Stretch).DragWindow()
			if view == Files && st.Files.Views[s.ID] != nil {
				paints.IconButton(c, "files-toggle-view", glyph.Doc, x.Tip(locales.FilesCloseFile(), "files-close"), pal, func() {
					x.RunLater(func() {
						if fv := x.FilesState().Views[s.ID]; fv != nil {
							fv.Open = false
						}
					})
				})
			}
			paints.IconButton(c, "close-side", glyph.Close, x.Tip(locales.SideClose(), "changes.toggle"), pal, func() {
				x.RunLater(func() { Toggle(x, "") })
			})
			if right {
				x.WindowControls(c, pal)
			}
		})
		ui.Column(c.Key("body")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			switch view {
			case Files:
				x.FilesSide(c, pal, s)
				return
			case Info:
				x.InfoSide(c, pal, s)
				return
			}
			if strings.HasPrefix(view, "plugin:") {
				x.PluginSide(c, pal, s, view, width)
				return
			}
			x.ChangesSide(c, pal, s, width)
		})
		// The viewer of a file of Files, over the tab while its button in
		// the header, its icon, tells so.
		if fv := x.FilesState().Views[s.ID]; view == Files && fv != nil && fv.Open {
			ui.Column(c.Key("file")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
				x.FileViewer(c, pal, s, fv)
			})
		}
	})
	if card.Shortcut(0, ui.KeyEscape) {
		x.FocusTerminal()
	}
}

// tabLabelsWidth is the panel's narrowest width that names every view: a
// narrower one names the view it shows, and shows the others' icons alone.
const tabLabelsWidth = 420

// tabButton is a view's name in the panel's header, a click showing it: the
// one shown in the text's color, the others faint. labels tells whether the
// header names every view, or only the one it shows, the others' icons
// saying which they are. The header drags the window where no tab is, as
// the window controls are, which the tabs are part of as windowControls'
// buttons are.
func tabButton(x App, c *ui.Context, pal *theme.Palette, id, label string, svg *ui.SVG, on bool, action string, labels bool, fn func()) {
	fg := pal.TextFaint
	if on {
		fg = pal.Text
	}
	b := ui.ButtonBase(c.Key("tab-"+id)).Height(26).Gap(5).Padding(0, 7).Radius(6).AlignItems(ui.Center).Shrink(0).
		Cursor(ui.CursorPointer).Label(x.Tip(label, action))
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
		// The view shown is named where the others are not: the header
		// says where the user is, as the panel is narrow.
		if labels || on {
			ui.Text(c, label).FontSize(14).FontWeight(600).TextColor(fg).SingleLine().Shrink(0)
		}
	})
}

// b2f is a bool as 0 or 1, for animations.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
