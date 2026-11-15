package layout

import (
	"github.com/egoist/mygo"
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Carder is what the card of the session shown asks of the app.
type Carder interface {
	// SessionOr is a session the app knows, nil for none, and CacheShown
	// tells whether the state came; HostOf is the machine it runs on, and
	// Local this machine.
	SessionOr(id string) *proto.Session
	CacheShown() bool
	HostOf(id string) *host.Host
	Local() *host.Host
	// Glass is a color of the window, as the settings make it.
	Glass(c ui.Color, pal *theme.Palette) ui.Color
	// Panes draws a session's panes, right telling that they reach the
	// window's right edge, and CardDrop takes the files dropped on them.
	PanesIn(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, right bool)
	CardDrop(s *proto.Session, files []string)
	// RightShown reports whether the side panel shows, and SideEdge and
	// SidePanel draw its edge and it.
	RightShown() bool
	SideEdge(c *ui.Context, pal *theme.Palette, row float32)
	SidePanel(c *ui.Context, pal *theme.Palette, s *proto.Session, width float32, right bool)
	SideWidth(rowW float32) float32
	// PaneRef is the handle the panes' row binds, and RowW the row's
	// width, as the last frame had it.
	PaneRef(id string) *ui.Handle
	RowW() float32
	// HeaderPad and ButtonsLeft are where the card's header starts, and
	// its buttons with the sidebar hidden.
	HeaderPad() float32
	ButtonsLeft(bar ui.TitleBar) float32
	// ShortcutTip is a button's tooltip with its action's shortcut, and
	// Shortcut the shortcut alone.
	ShortcutTip(title, id string) string
	Shortcut(id string) string
	// ToggleSidebar shows the sidebar, or hides it, and NewSession starts
	// a session; AskSSHHost asks for a machine to run one on.
	ToggleSidebar()
	NewSession(project string)
	AskSSHHost()
	OpenProjectDialog()
	// RestartDaemon restarts hylad into this version, and Retry connects
	// to it again.
	RestartDaemon()
	Retry(h *host.Host)
	// Window is the app's window, and WindowControls draws its buttons on
	// its title.
	Window() *mygo.Window
	WindowControls(c *ui.Context, pal *theme.Palette)
	// SwitchMode is how a session comes in place of another, as the
	// settings say.
	SwitchMode() panes.SessionSwitch
}

// SideGap is the visual space between the panes and the side panel: the
// panes' gap, halved.
const SideGap = 4

// Message is a card's message: a title, a detail and an action.
func Message(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func()) {
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

// Empty is the card of no session: what the app may start, and what it
// says of the CLI.
func Empty(x Carder, c *ui.Context, pal *theme.Palette) {
	ui.Column(c).Fill().Center().Gap(0).Padding(24).Children(func() {
		ui.Box(c).Size(52, 52).Radius(14).Center().Background(pal.Chip).TextColor(pal.TextMuted).Margin(0, 0, 18, 0).Children(func() {
			ui.Icon(c, glyph.Terminal).Size(26, 26)
		})
		ui.Text(c, locales.EmptyTitle()).FontSize(19).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, locales.EmptyDetail()).
			FontSize(13.5).TextColor(pal.TextMuted).MaxWidth(380).TextAlign(ui.Center).LineHeight(1.45).Margin(8, 0, 22, 0)
		ui.Column(c).Gap(12).Children(func() {
			ui.Row(c).Gap(10).Children(func() {
				if style.Button(c, "empty-new", locales.ActionNewSession(), x.Shortcut("session.new"), true, pal) {
					x.NewSession("")
				}
				if style.Button(c, "empty-open", locales.ActionOpenProject(), x.Shortcut("project.open"), false, pal) {
					x.OpenProjectDialog()
				}
			})
			// Another machine's, as the footer adds: an SSH host, a pod, or
			// one that runs Hyla, whose sessions show here. A quiet button,
			// a link under the two of this machine.
			if style.QuietButton(c, "empty-connect", locales.EmptyConnect(), locales.EmptyConnectDetail(), pal) {
				x.AskSSHHost()
			}
		})
		ui.Column(c).Margin(40, 0, 0, 0).Padding(14, 16).Gap(6).Radius(10).Background(pal.Chip).MaxWidth(420).Children(func() {
			ui.Text(c, locales.EmptyForAgents()).FontSize(11.5).FontWeight(600).TextColor(pal.TextMuted)
			ui.Text(c, "hyla new -n review -- claude\nhyla prompt review \"run the tests and fix them\"\nhyla wait review && hyla read review").
				Font("monospace").FontSize(12).TextColor(pal.Text).LineHeight(1.5).Selectable()
		})
	})
}

// Card draws the card of the session id: its panes, and the side panel,
// when the window is wide enough for both; or, for no session, what the
// app has to say.
func Card(x Carder, c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, id string) {
	c = c.Key("session-" + id)
	s := x.SessionOr(id)
	h := x.Local()
	if s != nil {
		h = x.HostOf(s.ID)
	}
	if s != nil && h.Conn != host.StateVersion && x.CacheShown() {
		// Files dropped on the content go to the session's panes, on them,
		// between them or on the panel beside them (drop_files.go).
		box := ui.Column(c.Key("card")).Grow(1).Basis(0).MinWidth(0).MinHeight(0)
		if files := box.DroppedFiles(); len(files) > 0 {
			x.CardDrop(s, files)
		}
		box.Children(func() {
			if !x.RightShown() {
				x.PanesIn(c, pal, bar, shown, s, true)
				return
			}
			// The panes, then the side panel, which takes the window's
			// right edge.
			ref := x.PaneRef("with-side")
			row := ui.Row(c.Key("with-side")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Bind(ref)
			w := x.RowW()
			if w == 0 {
				w = ref.Bounds(c).W
			}
			row.Children(func() {
				// The gap between the panes and the side panel is this
				// column's end margin, which the panel's edge drags (side.go).
				ui.Column(c.Key("panes")).Grow(1).Basis(0).MinWidth(0).AlignItems(ui.Stretch).Padding(0, SideGap, 0, 0).Children(func() {
					x.PanesIn(c, pal, bar, shown, s, false)
				})
				x.SideEdge(c, pal, w)
				if x.RightShown() {
					x.SidePanel(c, pal, s, x.SideWidth(w), true)
				}
			})
		})
		return
	}
	// No session: one card, for what the app has to say.
	card := style.Raised(ui.Column(c).Grow(1).MinHeight(0).Radius(14).Background(x.Glass(pal.Card, pal)), pal).Clip()
	card.Children(func() {
		left := x.HeaderPad()
		ui.Row(c).Height(HeaderH).Shrink(0).AlignItems(ui.Center).Padding(0, 10, 0, left).DragWindow().Children(func() {
			if shown < 0.5 {
				ui.Row(c).Absolute().Left(x.ButtonsLeft(bar)).Top(0).Bottom(0).AlignItems(ui.Center).Gap(2).Opacity(1 - 2*shown).Children(func() {
					paints.IconButton(c, "show-sidebar", glyph.Sidebar, x.ShortcutTip(locales.SidebarShow(), "sidebar.toggle"), pal, x.ToggleSidebar)
					paints.IconButton(c, "new-session-2", glyph.Plus, x.ShortcutTip(locales.ActionNewSession(), "session.new"), pal, func() { x.NewSession("") })
				})
			}
			ui.Text(c, "Hyla").FontSize(14).FontWeight(600).TextColor(pal.Text)
			ui.Spacer(c)
			x.WindowControls(c, pal)
		})
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			switch {
			case h.Conn == host.StateVersion:
				Message(c, pal, locales.UpdatedTitle(), locales.UpdatedDetail(), locales.CommonRestartService(), x.RestartDaemon)
			case h.Conn == host.StateFailed && h.Client == nil:
				Message(c, pal, locales.ConnCantReach(), h.Err, locales.CommonTryAgain(), func() { x.Retry(h) })
			case !x.CacheShown():
				ui.Column(c).Fill().Center().Children(func() {
					ui.Spinner(c).Label(locales.StatusConnectingLabel())
				})
			default:
				Empty(x, c, pal)
			}
		})
	})
}
