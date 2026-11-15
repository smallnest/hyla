package window

import (
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/k8sdialog"
	"hyla/internal/ui/sidebar"
	"hyla/internal/ui/sidebarview"
	"hyla/internal/ui/sshdialog"
	"hyla/internal/ui/theme"
)

// The app, as the sidebar asks it.
var _ sidebarview.App = (*App)(nil)

// The app, as the sidebar's order asks it.
var _ sidebar.App = order{}

// order is the app, as the sidebar's order asks it: its preferences are
// the order's alone.
type order struct{ *App }

func (o order) PrefsNow() sidebar.Prefs { return o.PrefsNowAll().Prefs }

func (a *App) ProjectState() []proto.Project { return a.St.Projects }

func (a *App) SessionState() []proto.Session { return a.St.Sessions }

// SelectedSession is the session the user looks at, nil for none, and
// SessionOf a session the app knows.
func (a *App) SelectedSession() *proto.Session    { return a.session(a.Selected) }
func (a *App) SessionOf(id string) *proto.Session { return a.session(id) }

// RenameFrames counts the frame a rename began in.
func (a *App) RenameFrames() *int { return &a.RenameFramesF }

// RunAction runs an action, as the sidebar's menu does.
func (a *App) RunAction(id string, args map[string]string) { a.run(id, args) }

// order is the app, as the sidebar's order asks it.
func (a *App) order() sidebar.App { return order{a} }

// Root is the app as the sidebar's order asks it.
func (a *App) Root() sidebarview.Rooter { return orderRoot{a} }

type orderRoot struct{ *App }

func (r orderRoot) PrefsNowAll() sidebarview.Prefs   { return r.App.PrefsNowAll() }
func (r orderRoot) PrefsNow() sidebar.Prefs          { return r.App.PrefsNowAll().Prefs }
func (r orderRoot) MachinesNow() []*host.Host        { return r.App.Machines }
func (r orderRoot) SessionsNow() []proto.Session     { return r.App.St.Sessions }
func (r orderRoot) ProjectsNow() []proto.Project     { return r.App.St.Projects }
func (r orderRoot) Session(id string) *proto.Session { return r.App.session(id) }
func (r orderRoot) SelectedID() string               { return r.App.Selected }

// HostList are the hosts the app holds, this machine's first.
func (a *App) HostList() []*host.Host { return a.Machines }

// Shortcut is an action's key, as the menus show it.
func (a *App) Shortcut(id string) string { return a.keys(id) }

// ShortcutTip is a button's tooltip, with its action's shortcut.
func (a *App) ShortcutTip(title, id string) string { return a.tip(title, id) }

// ShortcutItem shows an action's shortcut by a context menu's item.
func (a *App) ShortcutItem(it *ui.MenuItem, id string) *ui.MenuItem { return a.menuShortcut(it, id) }

func (a *App) DaemonStale() bool                          { return a.daemonStale() }
func (a *App) AllProjectsCollapsed() bool                 { return a.allProjectsCollapsed() }
func (a *App) JumpTo(want func(*proto.Session) bool) bool { return a.jumpTo(want) }
func (a *App) MoveSession(id, project string)             { a.moveSession(id, project) }

// OpenLink opens what a path names.
func (a *App) OpenLink(id, link string) bool { return a.openLink(id, link) }

// Later_ runs fn after the frame.
func (a *App) Later_(fn func()) { a.RunLater(fn) }

func (a *App) ToggleProject(id string) { a.toggleProject(id) }

// PutCopy makes a text the user's to paste.
func (a *App) PutCopy(text string) { a.CopyText = text }

// RenameState is the name the user types, of a session or a project.
func (a *App) RenameState() *sidebarview.Rename {
	return &sidebarview.Rename{Session: a.Renaming, Project: a.RenamingProject, Buf: &a.RenameBuf}
}

// Drag is the place a dragged row last showed its line.
func (a *App) Drag() *sidebarview.Drag {
	return &sidebarview.Drag{At: a.DragAtAddr, After: a.DragAfterSide}
}

// SetSidebarGroup lists the sessions by project, none or date, and keeps
// it.
func (a *App) SetSidebarGroup(group string) {
	a.Prefs.SidebarGroup = group
	a.savePrefs()
}

// SetSidebarOrder orders the sessions by when they were created, or
// updated, and keeps it.
func (a *App) SetSidebarOrder(order string) {
	a.Prefs.SidebarOrder = order
	a.savePrefs()
}

func (a *App) SSHHostOfSession(id string) *sshdialog.Host { return a.sshHostOfSession(id) }
func (a *App) K8sPodOfSession(id string) *k8sdialog.Pod   { return a.k8sPodOfSession(id) }
func (a *App) SSHRows(c *ui.Context, pal *theme.Palette)  { sshdialog.Rows(a, c, pal) }
func (a *App) K8sRows(c *ui.Context, pal *theme.Palette)  { k8sdialog.Rows(a, c, pal) }

func (a *App) Sidebar(c *ui.Context, pal *theme.Palette, bar ui.TitleBar) {
	sidebarview.Sidebar(a, c, pal, bar)
}
