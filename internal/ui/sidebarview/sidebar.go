// Package sidebarview draws the window's sidebar: the buttons that hide
// it, fold it and add to it, the sessions under their projects or days, x
// machine's under its name, and the foot that tells what wants a look.
package sidebarview

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/app/gitinfo"
	"hyla/internal/app/host"
	"hyla/internal/platform"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/k8sdialog"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/marks"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/sidebar"
	"hyla/internal/ui/sshdialog"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// App is what the sidebar asks of the app it shows: its machines and the
// state they hold, the preferences the sidebar keeps, and the actions its
// rows and menus take.
type App interface {
	// Local is this machine, and HostList the hosts the app holds, this
	// machine's first; HostOf is of them, and RemoteHostHeader draws a
	// machine's header.
	Local() *host.Host
	HostList() []*host.Host
	HostOf(id string) *host.Host
	RemoteHostHeader(c *ui.Context, pal *theme.Palette, h *host.Host, top float32)
	// SessionOf is a session by its id, and SessionState the sessions and
	// ProjectState the projects the app shows.
	SessionOf(id string) *proto.Session
	SessionState() []proto.Session
	ProjectState() []proto.Project
	// Settings are the settings, and Prefs the preferences the sidebar
	// keeps.
	Settings() *settings.Settings
	PrefsNowAll() Prefs
	// Root is the app as the sidebar's order asks it.
	Root() Rooter
	// RenameState is the name the user types, of a session or a project,
	// and its frames; Drag() is the place a dragged row last showed its
	// line. Both change from the frame's end.
	RenameState() *Rename
	Drag() *Drag
	// RenameFrames counts the frame a rename began in.
	RenameFrames() *int
	// SetConfirm asks the app's confirmation.
	SetConfirm(a *confirm.Ask)
	// SSHRows and K8sRows draw the rows of the SSH and K8s projects.
	SSHRows(c *ui.Context, pal *theme.Palette)
	K8sRows(c *ui.Context, pal *theme.Palette)
	SetSidebarGroup(group string)
	SetSidebarOrder(order string)
	SavePrefs()
	// Selected is the session the window shows, and SelectSession shows
	// another.
	SelectedID() string
	SelectSession(id string)
	// Shortcut, ShortcutTip and ShortcutItem are an action's shortcut: its
	// keys, a tip that names it, and a menu item that shows it.
	Shortcut(id string) string
	ShortcutTip(title, id string) string
	ShortcutItem(it *ui.MenuItem, id string) *ui.MenuItem
	// Menu is the app's menu, and Run runs an action.
	Menu() *mygo.Menu
	RunAction(id string, args map[string]string)
	// ToggleSidebar shows the sidebar, or hides it, ToggleProject folds a
	// project or unfolds it, and AllProjectsCollapsed reports whether all
	// are folded; ToggleAllProjects folds them all, or unfolds them.
	ToggleSidebar()
	ToggleProject(id string)
	AllProjectsCollapsed() bool
	ToggleAllProjects()
	// NewSession starts a session in a project of this machine, or in
	// its home, and NewSessionOn one on a host; OpenProject opens one.
	NewSession(project string)
	NewSessionOn(h *host.Host, project string)
	OpenProjectDialog()
	// AskSSHHost and AskK8sPod ask for a machine to run a terminal on.
	AskSSHHost()
	AskK8sPod()
	// SSHHostOfSession and K8sPodOfSession are the machine or pod a
	// session is a terminal of, nil for none; ClearSSHProject and
	// ClearK8sProject forget the sidebar's project of them.
	SSHHostOfSession(id string) *sshdialog.Host
	K8sPodOfSession(id string) *k8sdialog.Pod
	ClearSSHProject()
	ClearK8sProject()
	// StartRename and CommitRename rename the session shown, and
	// StartProjectRename and CommitProjectRename a project.
	StartRename(id string)
	CommitRename()
	StartProjectRename(id string)
	CommitProjectRename()
	// CloseSession closes a session, asking first when asked to, and
	// CloseAllSessions the sessions of a project.
	CloseSession(id string, ask bool)
	CloseAllSessions(g sidebar.Group)
	RestartSession(id string)
	MoveSession(id, project string)
	// FocusTerminal gives the keyboard to the pane the user is in, and
	// FocusOf is that pane of a session.
	FocusTerminal()
	FocusOf(s *proto.Session) string
	// Call asks a session's hylad something, and ReportView tells it which
	// session shows.
	Call(id, method string, params any, done func(error))
	ReportView()
	// OpenLink opens what a path names, and ShowPaneChatIn shows a pane's
	// chat in place of its terminal.
	OpenLink(id, link string) bool
	ShowPaneChatIn(s *proto.Session, id string)
	// ProjectsOf are a host's projects.
	ProjectsOf(h *host.Host) []proto.Project
	// OpenPalette shows the palette, OpenSettings the Settings window at a
	// page, and OfferRestart asks to restart hylad; DaemonStale reports
	// that it runs another version.
	OpenPalette()
	OpenSettings(page string)
	OfferRestart()
	DaemonStale() bool
	// HostState draws how the app stands with a machine's hylad.
	HostState(c *ui.Context, pal *theme.Palette, h *host.Host)
	// JumpTo selects the first session want accepts, and JumpToAttention
	// the first that needs the user.
	JumpTo(accept func(*proto.Session) bool) bool
	JumpToAttention()
	// Later runs fn after the frame, and PutCopy makes a text the user's
	// to paste.
	Later_(fn func())
	PutCopy(text string)
}

// app is the sidebar's app, as the order asks it: it names the state as
// the order does.
// Prefs are the preferences the sidebar keeps: the order's, and the SSH and
// K8s projects whose terminals its rows are.
type Prefs struct {
	sidebar.Prefs
	SSHProject string
	K8sProject string
}

// Rename is a session's or a project's name, while the user types it.
type Rename struct {
	Session string // the session's id, "" while none is
	Project string // the project's id, "" while none is
	Buf     *string
}

// Drag is the place the insertion line last showed, as a row is dragged
// over another.
type Drag struct {
	At    string
	After bool
}

// Rooter is the app, as the sidebar's order keeps its preferences.
type Rooter = interface {
	sidebar.App
	PrefsNowAll() Prefs
}

// app makes the sidebar's order of the app it is given.
type app struct {
	App
	root Rooter
}

func (a app) MachinesNow() []*host.Host        { return a.HostList() }
func (a app) SessionsNow() []proto.Session     { return a.SessionState() }
func (a app) ProjectsNow() []proto.Project     { return a.ProjectState() }
func (a app) Session(id string) *proto.Session { return a.SessionOf(id) }
func (a app) SelectedID() string               { return a.SelectedID() }
func (a app) PrefsNow() sidebar.Prefs          { return a.root.PrefsNowAll().Prefs }

// Sidebar draws the sidebar: the buttons that hide it, fold it and add to
// it, the sessions under their projects or days, and the foot that tells
// what wants a look.
func Sidebar(x App, c *ui.Context, pal *theme.Palette, bar ui.TitleBar) {
	ui.Row(c).Height(layout.TopBar).Shrink(0).AlignItems(ui.Center).Padding(0, 12, 0, layout.Lead(bar)).Gap(2).DragWindow().Children(func() {
		// After the traffic lights on macOS, at the right; elsewhere the
		// window controls are at the right, and the buttons at the left,
		// where they stay as the sidebar hides (layout.Chrome.ButtonsLeft).
		if runtime.GOOS == "darwin" {
			ui.Spacer(c)
		}
		paints.IconButton(c, "hide-sidebar", glyph.Sidebar, x.ShortcutTip(locales.SidebarHide(), "sidebar.toggle"), pal, x.ToggleSidebar)
		foldButton(x, c, pal)
		sortButton(x, c, pal)
		newSessionButton(x, c, pal)
	})
	ui.Scroll(c.Key("sessions")).Grow(1).Padding(0, 10, 16, 10).Children(func() {
		n := 0 // the rows shown, for ⌘1–9 (Ctrl+1–9 elsewhere)
		if g := x.PrefsNowAll().SidebarGroup; g == "none" || g == "date" {
			sessionSidebar(x, c, pal, g, &n)
			return
		}
		gi := 0 // the groups shown, the first of which sits higher
		// This machine's projects first, unlabeled; then each host's,
		// under its name.
		if local := x.Local(); local.Loaded {
			for _, g := range sidebar.HostGroups(app{x, x.Root()}, local) {
				groupView(x, c, pal, g, gi, &n)
				gi++
			}
			if len(x.SessionState()) == 0 && local.Conn == host.StateReady {
				ui.Text(c, locales.SidebarNoSessions()).FontSize(12.5).TextColor(pal.TextFaint).Padding(8, 10)
			}
		}
		// Then each other machine's, under its name.
		for _, h := range x.HostList()[1:] {
			top := float32(18)
			if gi == 0 {
				top = 2
			}
			x.RemoteHostHeader(c, pal, h, top)
			gi++
			if h.Conn != host.StateReady {
				continue
			}
			for _, g := range sidebar.HostGroups(app{x, x.Root()}, h) {
				g.First = false
				groupView(x, c, pal, g, gi, &n)
				gi++
			}
		}
	})
	sidebarFooter(x, c, pal)
	c.After(20 * time.Second) // times ago move on
}

// foldButton folds every project in the sidebar, or unfolds them all once
// all are folded. A sidebar grouped by none or by date lists no projects,
// so it keeps the button out of the toolbar's way there.
func foldButton(x App, c *ui.Context, pal *theme.Palette) {
	g := x.PrefsNowAll().SidebarGroup
	if g != "" && g != "project" {
		return
	}
	svg, label := glyph.FoldAll, locales.SidebarCollapseAll()
	if x.AllProjectsCollapsed() {
		svg, label = glyph.UnfoldAll, locales.SidebarExpandAll()
	}
	paints.IconButton(c, "fold-sidebar", svg, label, pal, x.ToggleAllProjects)
}

// sortButton opens the sidebar's Group and Order menu, as Otty's sort
// icon does: how sessions group, and which order they take. It says
// "Group By" and "Order By" there, as each asks its own question.
func sortButton(x App, c *ui.Context, pal *theme.Palette) {
	p := x.PrefsNowAll()
	b := ui.ButtonBase(c.Key("sort-sidebar")).Size(28, 26).Radius(6).Center().TextColor(pal.TextMuted).
		Label(locales.SidebarSort()).Tooltip(locales.SidebarGroupBy() + ", " + locales.SidebarOrderBy())
	if b.Hovered() {
		b.Background(pal.RowHover).TextColor(pal.Text)
	}
	b.Menu(func(m *ui.Menu) {
		group := p.SidebarGroup
		if group == "" {
			group = "project"
		}
		order := p.SidebarOrder
		if order == "" {
			order = "created"
		}
		m.Item(locales.SidebarGroupBy()).Disabled(true)
		if m.Item(locales.SidebarGroupProject()).Checked(group == "project").Chosen() {
			x.SetSidebarGroup("project")
			x.SavePrefs()
		}
		if m.Item(locales.SidebarGroupNone()).Checked(group == "none").Chosen() {
			x.SetSidebarGroup("none")
			x.SavePrefs()
		}
		if m.Item(locales.SidebarGroupDate()).Checked(group == "date").Chosen() {
			x.SetSidebarGroup("date")
			x.SavePrefs()
		}
		m.Separator()
		m.Item(locales.SidebarOrderBy()).Disabled(true)
		if m.Item(locales.SidebarOrderCreated()).Checked(order == "created").Chosen() {
			x.SetSidebarOrder("created")
			x.SavePrefs()
		}
		if m.Item(locales.SidebarOrderUpdated()).Checked(order == "updated").Chosen() {
			x.SetSidebarOrder("updated")
			x.SavePrefs()
		}
	})
	b.Children(func() { ui.Icon(c, glyph.Sort).Size(16, 16) })
}

// newSessionButton opens the menu that starts work: a session, in x
// project of its own or this machine's home, and what the app connects
// to, a project to open, an SSH host or x K8s pod. It takes the place of
// the buttons the sidebar's foot used to hold for those last three, so
// that one menu holds what begins anything, and the foot stays quiet.
func newSessionButton(x App, c *ui.Context, pal *theme.Palette) {
	b := ui.ButtonBase(c.Key("new-session")).Size(28, 26).Radius(6).Center().TextColor(pal.TextMuted).
		Label(locales.ActionNewSession()).Tooltip(x.ShortcutTip(locales.ActionNewSession(), "session.new"))
	if b.Hovered() {
		b.Background(pal.RowHover).TextColor(pal.Text)
	}
	b.Menu(func(m *ui.Menu) {
		if x.ShortcutItem(m.Item(locales.ActionNewSession()).Icon(glyph.NewSessionMenuIcon), "session.new").Chosen() {
			x.NewSession("")
		}
		if x.ShortcutItem(m.Item(locales.ActionNewSessionHome()), "session.newHome").Chosen() {
			x.RunAction("session.newHome", nil)
		}
		m.Separator()
		if x.ShortcutItem(m.Item(locales.ActionOpenProject()).Icon(glyph.OpenProjectMenuIcon), "project.open").Chosen() {
			x.OpenProjectDialog()
		}
		if m.Item(locales.ActionAddSSHHost()).Icon(glyph.AddSSHMenuIcon).Chosen() {
			x.AskSSHHost()
		}
		if m.Item(locales.ActionAddK8sPod()).Icon(glyph.AddK8sMenuIcon).Chosen() {
			x.AskK8sPod()
		}
	})
	b.Children(func() { ui.Icon(c, glyph.Plus).Size(16, 16) })
}

// sessionSidebar lists every session, without projects: flat, or under
// the day it was created.
func sessionSidebar(x App, c *ui.Context, pal *theme.Palette, group string, n *int) {
	gi := 0
	local := x.Local()
	for _, h := range x.HostList() {
		if h == local && !h.Loaded {
			continue
		}
		empty := true
		if group == "date" {
			dgs := sidebar.DateGroups(app{x, x.Root()}, h)
			empty = len(dgs) == 0
			for _, dg := range dgs {
				top := float32(18)
				if gi == 0 && *n == 0 {
					top = 2
				}
				dateHeader(x, c, pal, "date-"+h.Name+"-"+sidebar.DateLabel(dg.Day, c.Now()), sidebar.DateLabel(dg.Day, c.Now()), len(dg.Sessions), top)
				gi++
				for _, id := range dg.Sessions {
					if s := x.SessionOf(id); s != nil {
						sessionRow(x, c, pal, s, n, layout.RowInset, "")
					}
				}
			}
		} else {
			ss := sidebar.FlatSessions(app{x, x.Root()}, h)
			empty = len(ss) == 0
			for _, id := range ss {
				if s := x.SessionOf(id); s != nil {
					sessionRow(x, c, pal, s, n, layout.RowInset, "")
				}
			}
		}
		if h == local && empty && h.Conn == host.StateReady && len(x.HostList()) == 1 {
			ui.Text(c, locales.SidebarNoSessions()).FontSize(12.5).TextColor(pal.TextFaint).Padding(8, 10)
		}
	}
}

// dateHeader is a day in the sidebar's By Date.
func dateHeader(x App, c *ui.Context, pal *theme.Palette, key, label string, count int, top float32) {
	inset := float32(layout.RowInset)
	if pal.Dark {
		inset++
	}
	ui.Row(c.Key(key)).Height(28).Margin(top, 0, 3, 0).Padding(0, 6, 0, inset).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12.5).FontWeight(600).TextColor(pal.Group).SingleLine().Shrink(1)
		ui.Textf(c, "%d", count).FontSize(12.5).TextColor(pal.TextFaint).FontFeatures("tnum")
	})
}

func groupView(x App, c *ui.Context, pal *theme.Palette, g sidebar.Group, gi int, n *int) {
	p := x.PrefsNowAll()
	id, key := g.Project.ID, g.Key()
	// The SSH and K8s projects open terminals of their own, not shells:
	// their rows are the machines and pods they list.
	ssh := id != "" && id == p.SSHProject
	k8s := id != "" && id == p.K8sProject
	collapsed := p.Collapsed[key]
	// current says this project holds the session in the card; folded, its
	// badge keeps a dot for it.
	current := slices.Contains(g.Sessions, x.SelectedID())
	needs := 0
	for _, id := range g.Sessions {
		if s := x.SessionOf(id); s != nil && s.Status == proto.Waiting {
			needs++
		}
	}
	// A group with a session that needs you wears attention's color, as
	// the session's status icon does, so that a project folded away still
	// tells that it holds one.
	urgent := needs > 0
	top := float32(18)
	if gi == 0 {
		top = 2
	}
	// Its name starts past its badge (sessionRow), where its sessions'
	// names start too; the badge tells the project from its children.
	inset := float32(layout.RowInset)
	if pal.Dark {
		inset++
	}
	hdr := ui.Row(c.Key("project-"+key)).Height(28).Margin(top, 0, 3, 0).Padding(0, 6, 0, inset).Gap(6).AlignItems(ui.Center).Radius(6)
	renaming := id != "" && x.RenameState().Project == id
	// The name is the project's handle: dragged, it moves the project to
	// another place under its host.
	if id != "" && !renaming {
		hdr.Drag(sidebar.ProjectDrag{Host: g.Host, ID: id}).Cursor(ui.CursorGrab)
		if hdr.Dragging() {
			hdr.Opacity(0.35)
		}
	}
	if id != "" {
		if d, over := ui.DragOver[sidebar.ProjectDrag](hdr); over && d.Host == g.Host && d.ID != id {
			_, y, _ := hdr.PointerPosition()
			after := y > 14
			x.Drag().At, x.Drag().After = id, after
			hdr.Background(pal.RowHover)
			sidebar.DragLine(hdr, pal, after)
		}
		if d, ok := ui.Drop[sidebar.ProjectDrag](hdr); ok && d.Host == g.Host && d.ID != id {
			_, y, _ := hdr.PointerPosition()
			after := y > 14
			if x.Drag().At == id {
				// The side the insertion line last showed.
				after = x.Drag().After
			}
			target := id
			x.Later_(func() { sidebar.MoveProject(app{x, x.Root()}, d, target, after) })
		}
	}
	hovered := hdr.Hovered()
	if hdr.Clicked() && !renaming {
		x.ToggleProject(key)
	}
	paints.PointerTooltip(c, hdr, "tip-project-"+key, projectTip(x, g))
	hdr.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.NewSessionIn(g.Project.Name)).Chosen() {
			x.NewSessionOn(g.Host, id)
		}
		label := locales.SidebarCollapse()
		if collapsed {
			label = locales.SidebarExpand()
		}
		if m.Item(label).Chosen() {
			x.ToggleProject(key)
		}
		if m.Item(locales.SessionRename()).Disabled(id == "").Chosen() {
			x.StartProjectRename(id)
		}
		m.Separator()
		// Close All Sessions ends the sessions the project holds, not the
		// project itself, asking first as closing the project does: what
		// runs in them ends with them.
		if m.Item(locales.SidebarCloseAllSessions()).Disabled(len(g.Sessions) == 0).Chosen() {
			name, count := g.Project.Name, len(g.Sessions)
			x.SetConfirm(&confirm.Ask{
				Open:    true,
				Title:   locales.ConfirmCloseAllSessionsTitle(count, name),
				Detail:  locales.ConfirmCloseAllSessionsDetail(count),
				Action:  locales.SidebarCloseAllSessions(),
				Destroy: true,
				Run:     func() { x.CloseAllSessions(g) },
			})
		}
		if m.Item(locales.SidebarCloseProjectMenu()).Disabled(id == "").Chosen() {
			name, count := g.Project.Name, len(g.Sessions)
			x.SetConfirm(&confirm.Ask{
				Open:    true,
				Title:   locales.ConfirmCloseTitle(name),
				Detail:  locales.ConfirmCloseProjectDetail(count),
				Action:  locales.SidebarCloseProject(),
				Destroy: true,
				Run: func() {
					// An SSH or K8s project closed is one the sidebar no
					// longer opens: its rows and their terminals went
					// with its project.
					if ssh {
						x.ClearSSHProject()
					}
					if k8s {
						x.ClearK8sProject()
					}
					x.Call(id, "project.close", proto.IDParams{ID: id}, nil)
				},
			})
		}
	})
	hdr.Children(func() {
		badgeColor := sidebar.BadgeColor(x.ProjectState(), g.Project)
		if urgent {
			badgeColor = pal.Attention
		}
		badge := ui.Box(c.Key("project-icon-"+key)).Size(layout.ProjectBadge, layout.ProjectBadge).Shrink(0)
		if ssh || k8s {
			// The SSH and K8s projects are machines rather than folders:
			// their mark stands before their name as an agent's stands
			// before a session's, in the colour the project wears.
			icon := glyph.Host
			if k8s {
				icon = glyph.K8s
			}
			badge.Center().Children(func() {
				ui.Icon(c, icon).Size(14, 14).TextColor(marks.Tint(badgeColor, pal.Dark, pal.Text))
			})
		} else {
			badge.Radius(5).Center().Background(badgeColor).Children(func() {
				ui.Text(c, sidebar.ProjectInitial(g.Project.Name)).FontSize(10.5).FontWeight(700).TextColor(ui.Hex("#ffffff"))
			})
		}
		if collapsed && current {
			// The project's sessions are folded away, the one in the card
			// too: a dot over the badge's corner says it is still this
			// project's; its white ring lifts it off any badge color. It is
			// the badge's own color, which attention's would hide.
			dot := pal.Danger
			if urgent {
				dot = sidebar.BadgeColor(x.ProjectState(), g.Project)
			}
			badge.DrawOver(func(p *ui.Painter, r ui.Rect) {
				cx, cy := r.X+r.W-1, r.Y+r.H-1
				outer, inner := float32(9), float32(5)
				p.Fill(ui.Rect{X: cx - outer/2, Y: cy - outer/2, W: outer, H: outer}, ui.Hex("#ffffff"), outer/2)
				p.Fill(ui.Rect{X: cx - inner/2, Y: cy - inner/2, W: inner, H: inner}, dot, inner/2)
			})
		}
		if renaming {
			in := ui.TextInputBase(c.Key("rename"), &*x.RenameState().Buf).Grow(1).MinWidth(0).FontSize(13).FontWeight(500).AutoFocus()
			if in.Shortcut(0, ui.KeyEscape) {
				x.RenameState().Project = ""
				x.FocusTerminal()
			} else if in.Submitted() || (!in.Focused() && !in.Pressed() && in.Valid() && renameStarted(x, c)) {
				x.CommitProjectRename()
			}
			return
		}
		ui.Text(c, g.Project.Name).FontSize(13).FontWeight(500).TextColor(pal.Group).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
		ui.Textf(c, "%d", len(g.Sessions)).FontSize(12.5).TextColor(pal.TextFaint).FontFeatures("tnum")
		chev := ui.Icon(c, glyph.Chevron).Size(11, 11).TextColor(pal.TextFaint)
		turn := chev.AnimateTo("turn", 90*(1-layout.B2F(collapsed)), ui.Snappy)
		chev.Rotate(turn).Opacity(chev.Animate("shown", layout.B2F(hovered || collapsed), 120*time.Millisecond))
		ui.Spacer(c)
		// This machine's hylad says how the app stands with it here, when
		// it reconnects, or wants restarting: the first group's header.
		if g.First {
			x.HostState(c, pal, g.Host)
		}
		if needs > 0 {
			ui.Text(c, locales.SidebarNeedsYou(needs)).FontSize(12.5).FontWeight(500).TextColor(pal.Attention).SingleLine()
		}
		if hovered {
			b := ui.ButtonBase(c.Key("add")).Size(20, 20).Radius(5).Center().TextColor(pal.TextMuted).Label(locales.NewSessionIn(g.Project.Name)).Tooltip(locales.ActionNewSession())
			if b.Hovered() {
				b.Background(pal.RowHover).TextColor(pal.Text)
			}
			b.Children(func() { ui.Icon(c, glyph.Plus).Size(13, 13) })
			if b.Clicked() {
				x.NewSessionOn(g.Host, id)
			}
		}
	})
	if collapsed {
		return
	}
	for _, id := range g.Sessions {
		s := x.SessionOf(id)
		// A machine's or pod's terminal is its row, not a session of its
		// own.
		if s == nil || ssh && x.SSHHostOfSession(s.ID) != nil || k8s && x.K8sPodOfSession(s.ID) != nil {
			continue
		}
		sessionRow(x, c, pal, s, n, float32(layout.RowInset), g.DragKey())
	}
	if ssh {
		x.SSHRows(c, pal)
	}
	if k8s {
		x.K8sRows(c, pal)
	}
}

// projectTip is a project's header tooltip: its directory, or the name of
// the sidebar's SSH and K8s projects, which have no directory of their
// own.
func projectTip(x App, g sidebar.Group) string {
	if p := x.PrefsNowAll(); g.Project.ID != "" && (g.Project.ID == p.SSHProject || g.Project.ID == p.K8sProject) {
		return g.Project.Name
	}
	return g.Host.ShortDir(g.Project.Dir)
}

func sessionRow(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, n *int, inset float32, group string) {
	p := x.PrefsNowAll()
	*n++
	index := *n
	selected := s.ID == x.SelectedID()
	// A detailed row gives the session's status message, else its branch
	// and changes, a line of its own under the name.
	detailed := x.Settings().SidebarRows != "compact"
	h := float32(layout.RowH)
	if detailed {
		h = layout.RowH2
	}
	row := ui.Row(c.Key(s.ID)).Height(h).Padding(0, 10, 0, inset).Gap(layout.RowGap).AlignItems(ui.Center).Radius(8).Margin(0, 0, 1, 0)
	hovered := row.Hovered()
	if pal.Dark {
		// Room for the chosen row's edge in every row, so that choosing
		// one moves no text.
		row.Border(1, ui.Transparent)
	}
	bg := ui.Transparent
	switch {
	case selected:
		bg = theme.TranslucentGlass(pal.RowSelected, pal, *x.Settings())
		style.Chosen(row, pal)
	case hovered:
		bg = theme.TranslucentGlass(pal.RowHover, pal, *x.Settings())
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	if (row.Pressed() || row.Clicked()) && !selected {
		x.SelectSession(s.ID)
	}
	if row.DoubleClicked() {
		x.StartRename(s.ID)
	}
	// The row is its session's handle within its project: dragged, it
	// moves the session to another place there, past the row of the one
	// it is dropped on, above or below its middle.
	if group != "" && x.RenameState().Session != s.ID {
		drag := sidebar.SessionDrag{Group: group, ID: s.ID}
		row.Drag(drag).Cursor(ui.CursorGrab)
		if row.Dragging() {
			row.Opacity(0.35)
		}
		if d, over := ui.DragOver[sidebar.SessionDrag](row); over && d.Group == group && d.ID != s.ID {
			_, y, _ := row.PointerPosition()
			after := y > h/2
			x.Drag().At, x.Drag().After = s.ID, after
			row.Background(pal.RowHover)
			sidebar.DragLine(row, pal, after)
		}
		if d, ok := ui.Drop[sidebar.SessionDrag](row); ok && d.Group == group && d.ID != s.ID {
			_, y, _ := row.PointerPosition()
			after := y > h/2
			if x.Drag().At == s.ID {
				// The side the insertion line last showed.
				after = x.Drag().After
			}
			target := s.ID
			x.Later_(func() { sidebar.ReorderSession(app{x, x.Root()}, d, target, after) })
		}
	}
	tip := x.HostOf(s.ID).ShortDir(s.WorkDir())
	// An SSH or K8s terminal says where it runs to, here and below.
	sshAddr := ""
	if h := x.SSHHostOfSession(s.ID); h != nil {
		sshAddr = h.Addr
		tip = sshAddr + "\n" + tip
	}
	k8sCmd := ""
	if pod := x.K8sPodOfSession(s.ID); pod != nil {
		k8sCmd = pod.Command
		tip = k8sCmd + "\n" + tip
	}
	ssh := sshAddr != ""
	if s.Process != "" {
		tip = s.Process + " · " + tip
	}
	if s.Message != "" {
		tip = s.Message + "\n" + tip
	}
	g := gitinfo.Session(s)
	if g != nil {
		tip += "\n" + gitinfo.Summary(g)
	}
	paints.PointerTooltip(c, row, "tip-"+s.ID, tip)
	// The mark of the agent the session runs, before its name, in the
	// agent's own colour: the row says which agent it is at a glance. A
	// session with no agent, or one whose agent Hyla has no mark for,
	// shows none rather than another agent's.
	agentLogo, agentBrand, agentKnown := marks.Agent(s.Agent)
	agentFull, _ := marks.MarkFull(s.Agent)
	// The user's own logo of an agent the set has none for, which the
	// slot draws instead of nothing.
	agentUser, userKnown := marks.LogoOf(s.Agent)
	if agentKnown {
		agentUser = nil
	}
	agentKnown = agentKnown || userKnown
	// The mark's slot is every row's, whether or not this session has an
	// agent, and as wide as the badge a project's name follows: x project's
	// names and its sessions' start at one place. gap is the row's own.
	mark := func(gap float32) {
		slot := markSlot(x, c, gap)
		if !agentKnown {
			return
		}
		slot.Children(func() {
			if agentFull != nil {
				ui.Image(c, agentFull).Size(marks.Size, marks.Size).Shrink(0)
				return
			}
			if agentUser != nil {
				ui.Image(c, agentUser).Size(marks.Size, marks.Size).Radius(3).Shrink(0)
				return
			}
			ui.Icon(c, agentLogo).Size(marks.Size, marks.Size).TextColor(marks.Tint(agentBrand, pal.Dark, pal.Text))
		})
	}
	row.ContextMenu(func(m *ui.Menu) { sessionMenu(x, m, s) })
	row.Children(func() {
		if x.RenameState().Session == s.ID {
			in := ui.TextInputBase(c.Key("rename"), &*x.RenameState().Buf).Grow(1).MinWidth(0).FontSize(13.5).FontWeight(600).AutoFocus()
			if in.Submitted() || in.Shortcut(0, ui.KeyEscape) {
				if in.Shortcut(0, ui.KeyEscape) {
					x.RenameState().Session = ""
					x.FocusTerminal()
				} else {
					x.CommitRename()
				}
			} else if !in.Focused() && !in.Pressed() && in.Valid() && renameStarted(x, c) {
				x.CommitRename()
			}
			return
		}
		name := func() {
			name := ui.Text(c, s.DisplayName(x.Settings().SessionName)).FontSize(13.5).FontWeight(400).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).TextColor(pal.Text)
			if s.Unread {
				name.FontWeight(650)
			}
			if s.Status == proto.Exited && !s.Unread {
				name.TextColor(pal.TextMuted)
			}
		}
		when := func() {
			cmd := c.Modifiers() == ui.Cmd
			if cmd && index <= 9 {
				ui.Text(c, actions.Glyphs(fmt.Sprintf("CmdOrCtrl+%d", index))).FontSize(12).TextColor(pal.TextMuted).FontFeatures("tnum").SingleLine().Shrink(0)
				return
			}
			// The name gives way to these, never they to the name.
			ui.Text(c, paints.Ago(s.Since, c.Now())).FontSize(12.5).TextColor(pal.TextFaint).FontFeatures("tnum").SingleLine().Shrink(0)
		}
		// A session goes at once, as Close Pane goes (^D), without the
		// menu's asking; after the frame, which its end reshapes.
		trailing := func() {
			id := s.ID
			statusSlot(x, c, pal, hovered, s.Status, s.Unread, s.Progress, func() { x.Later_(func() { x.CloseSession(id, true) }) })
		}
		if detailed {
			// The name, the time and the status over the line of what the
			// session is at, on the row's whole width, so that a long
			// branch reads as far as it can.
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					mark(8)
					name()
					ui.Spacer(c)
					when()
					trailing()
				})
				line, msg := rowDetail(x, s, g, x.HostOf(s.ID).ShortDir(s.WorkDir()))
				switch {
				case ssh:
					// An SSH terminal's second line is the address ssh
					// runs to, not what the folder it runs in is.
					line, msg = sshAddr, false
				case k8sCmd != "":
					// A K8s terminal's is the kubectl command that
					// enters its pod.
					line, msg = k8sCmd, false
				case p.SidebarGroup == "none" || p.SidebarGroup == "date":
					// Without project headers, the project says which list
					// the session is in, before what it is at.
					if p := sidebar.ProjectName(app{x, x.Root()}, s.Project); p != "" {
						line = p + " · " + line
					}
				}
				fg := pal.TextFaint
				if msg {
					fg = pal.TextMuted
				}
				// Under the name, past the mark's slot: the second line
				// stays its session's, not the mark's.
				ui.Text(c, line).FontSize(12).TextColor(fg).SingleLine().Ellipsis("…").MinWidth(0).
					Padding(0, 0, 0, layout.ProjectBadge+layout.ProjectBadgeGap)
			})
			return
		}
		mark(layout.RowGap)
		name()
		// An SSH or K8s terminal names what it reaches, by its name.
		if detail := sshAddr; detail != "" {
			ui.Text(c, detail).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(3).MinWidth(24).MaxWidth(160)
		} else if k8sCmd != "" {
			ui.Text(c, k8sCmd).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(3).MinWidth(24).MaxWidth(220)
		}
		// A branch of its own, as an agent's in a worktree, by the name.
		if b := gitinfo.OffBranch(g); b != "" {
			ui.Text(c, b).FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(3).MinWidth(24).MaxWidth(120)
		}
		// Without project headers, the project says which list it is.
		if p.SidebarGroup == "none" || p.SidebarGroup == "date" {
			if p := sidebar.ProjectName(app{x, x.Root()}, s.Project); p != "" {
				ui.Text(c, p).FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(2).MinWidth(0).MaxWidth(90)
			}
		}
		ui.Spacer(c)
		when()
		trailing()
	})
}

// markSlot is the room a row keeps before its name: as wide as a project's
// badge and the gap after it, less the gap the row itself leaves, so that x
// session's name starts where its project's does, and every row's lines up.
func markSlot(x App, c *ui.Context, gap float32) ui.Element {
	return ui.Box(c).Size(layout.ProjectBadge+layout.ProjectBadgeGap-gap, marks.Size).Center().Shrink(0)
}

// statusSlot ends a row that stands for a terminal: its status, or, while
// the pointer is on the row, a close button in the status's fixed slot, so
// that hovering moves no text. close ends what the row's terminal runs.
func statusSlot(x App, c *ui.Context, pal *theme.Palette, hovered bool, st proto.Status, unread bool, progress int, close func()) {
	ui.Box(c.Key("trailing")).Size(20, 20).Shrink(0).Center().Children(func() {
		if !hovered {
			paints.StatusP(c, pal, st, unread, progress, 15)
			return
		}
		b := ui.ButtonBase(c.Key("close")).Size(20, 20).Radius(5).Center().TextColor(pal.TextMuted).
			Label(locales.ActionCloseSession()).Tooltip(locales.ActionCloseSession())
		if b.Hovered() {
			b.Background(pal.RowHover).TextColor(pal.Text)
		}
		b.Children(func() { ui.Icon(c, glyph.Close).Size(13, 13) })
		if b.Clicked() {
			close()
		}
	})
}

// rowDetail is a detailed row's second line: the session's status
// message, else its branch and changes, else where it works.
func rowDetail(x App, s *proto.Session, g *proto.GitInfo, dir string) (line string, msg bool) {
	if s.Message != "" {
		// A notification's text can run over lines: the row has one.
		return strings.Join(strings.Fields(s.Message), " "), true
	}
	if g != nil {
		return gitinfo.Summary(g), false
	}
	return dir, false
}

// renameStarted keeps a rename from ending in the frame it began in,
// before its input took the focus.
func renameStarted(x App, c *ui.Context) bool {
	if *x.RenameFrames() < 3 {
		*x.RenameFrames()++
		c.Invalidate()
		return false
	}
	return true
}

func sessionMenu(x App, m *ui.Menu, s *proto.Session) {
	id := s.ID
	if x.ShortcutItem(m.Item(locales.SessionRename()), "session.rename").Chosen() {
		x.StartRename(id)
	}
	if s.Status == proto.Exited {
		if m.Item(locales.CommonRestart()).Chosen() {
			x.RestartSession(id)
		}
	}
	if s.Unread {
		if m.Item(locales.SessionMarkRead()).Chosen() {
			x.Call(id, "view", proto.ViewParams{ID: id}, func(error) { x.HostOf(id).ViewSent = nil; x.ReportView() })
		}
	}
	h := x.HostOf(id)
	m.Submenu(locales.SessionMoveToProject(), func(m *ui.Menu) {
		// A session moves among its host's projects alone.
		for _, p := range x.ProjectsOf(h) {
			if m.Item(p.Name).Checked(p.ID == s.Project).Disabled(p.ID == s.Project).Chosen() {
				pid := p.ID
				x.Later_(func() { x.MoveSession(id, pid) })
			}
		}
	})
	if x.Settings().Feishu != nil && x.Settings().Feishu.App != "" {
		// The session's conversation, as a chat: its agent's, with the
		// Feishu chat it is with (chat.go).
		// It shows in the pane the user is in, in its terminal's place.
		if m.Item(locales.ChatOpen()).Chosen() {
			pid := x.FocusOf(s)
			x.Later_(func() {
				if cur := x.SessionOf(id); cur != nil && pid != "" {
					x.ShowPaneChatIn(cur, pid)
				}
			})
		}
	}
	m.Separator()
	if m.Item(locales.SessionCopyId()).Chosen() {
		x.PutCopy(id)
	}
	if m.Item(locales.SessionCopyDirectory()).Disabled(s.WorkDir() == "").Chosen() {
		x.PutCopy(s.WorkDir())
	}
	if m.Item(locales.SessionRevealIn(platform.FileManager())).Disabled(s.WorkDir() == "").Chosen() {
		x.OpenLink(id, s.WorkDir())
	}
	m.Separator()
	if x.ShortcutItem(m.Item(locales.ActionCloseSession()), "session.close").Chosen() {
		x.Later_(func() { x.CloseSession(id, true) })
	}
}

// sidebarFooter is the sidebar's foot: the buttons for the palette and
// Settings; then, at the right, what across the sessions wants a look,
// each a button going to the next session of it, and nothing while
// nothing does, or the connection to hylad while there is none.
func sidebarFooter(x App, c *ui.Context, pal *theme.Palette) {
	ui.Row(c).Height(36).Shrink(0).Padding(0, 12, 4, 12).Gap(2).AlignItems(ui.Center).DragWindow().Children(func() {
		paints.IconButton(c, "palette", glyph.Search, x.ShortcutTip(locales.ActionGoToSession(), "palette.open"), pal, x.OpenPalette)
		paints.IconButton(c, "settings", glyph.Gear, x.ShortcutTip(locales.SettingsTitle(), "settings.open"), pal, func() { x.OpenSettings("") })
		ui.Spacer(c)
		switch x.Local().Conn {
		case host.StateReady:
			if x.DaemonStale() {
				restartChip(x, c, pal)
			}
			waiting := func(s *proto.Session) bool { return s.Status == proto.Waiting }
			working := func(s *proto.Session) bool { return s.Status == proto.Working }
			done := func(s *proto.Session) bool { return s.Status == proto.Idle && s.Unread }
			footerChip(x, c, pal, proto.Waiting, false, waiting, pal.Attention, locales.FooterNeedsYou, x.ShortcutTip(locales.FooterGoToIt(), "session.attention"), x.JumpToAttention)
			footerChip(x, c, pal, proto.Working, false, working, pal.TextMuted, locales.FooterWorking, locales.FooterGoToIt(), nil)
			footerChip(x, c, pal, proto.Idle, true, done, pal.TextMuted, locales.FooterDone, locales.FooterGoToIt(), nil)
		case host.StateConnecting:
			footerNote(x, c, pal, pal.TextFaint, locales.StatusConnecting())
		default:
			footerNote(x, c, pal, pal.Danger, locales.StatusDisconnected())
		}
	})
}

// footerChip counts the sessions that are, by is, as their rows' status
// icon and the count, in fg; a click goes to the next of them, by jump,
// or else after the one shown. one and many word its label and tooltip,
// as "%d session needs you". It shows nothing while none is.
func footerChip(x App, c *ui.Context, pal *theme.Palette, st proto.Status, unread bool, is func(*proto.Session) bool, fg ui.Color, say func(int) string, hint string, jump func()) {
	n := 0
	for i := range x.SessionState() {
		if is(&x.SessionState()[i]) {
			n++
		}
	}
	if n == 0 {
		return
	}
	words := say(n)
	b := ui.ButtonBase(c.Key("chip-"+string(st))).Height(24).Padding(0, 8, 0, 6).Gap(5).Radius(12).AlignItems(ui.Center).
		Label(words).Tooltip(words + " · " + hint)
	if b.Hovered() {
		b.Background(pal.RowHover)
	}
	b.Children(func() {
		paints.Status(c, pal, st, unread, 12)
		ui.Textf(c, "%d", n).FontSize(12).FontWeight(500).TextColor(fg).FontFeatures("tnum")
	})
	if b.Clicked() {
		if jump == nil {
			jump = func() { x.JumpTo(is) }
		}
		jump()
	}
}

// restartChip offers to restart hylad into this version of Hyla, while it
// runs another.
func restartChip(x App, c *ui.Context, pal *theme.Palette) {
	b := ui.ButtonBase(c.Key("chip-restart")).Height(24).Padding(0, 8, 0, 8).Gap(6).Radius(12).AlignItems(ui.Center).
		Label(locales.CommonRestartService()).Tooltip(locales.UpdateRestartTip())
	if b.Hovered() {
		b.Background(pal.RowHover)
	}
	b.Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(pal.Attention)
		ui.Text(c, locales.CommonRestartService()).FontSize(12).FontWeight(500).TextColor(pal.Attention).SingleLine()
	})
	if b.Clicked() {
		x.OfferRestart()
	}
}

// footerNote says how the connection to hylad is, by a dot.
func footerNote(x App, c *ui.Context, pal *theme.Palette, dot ui.Color, label string) {
	ui.Row(c).Gap(7).Padding(0, 6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(dot)
		// It has the room the icons leave: never a label cut with no sign.
		ui.Text(c, label).FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).
			Tooltip(locales.FooterServiceTip())
	})
}
