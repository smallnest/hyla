// Package menus holds the app's menu, its items and the actions' titles:
// what a menu, the command palette and the tray show of an action, and
// the shortcut it takes.
package menus

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/settings"
	uisettings "hyla/internal/ui/settings"
	"hyla/locales"
)

// App is what the menus ask of the app they act on.
type App interface {
	// Settings are the settings, whose shortcuts the menus show, and
	// Recording is the action a shortcut is typed for, while one is.
	Settings() *settings.Settings
	Recording() string
	// Run runs an action, as a menu's item does, and Update runs fn on
	// the main thread.
	RunID(id string, args map[string]string)
	Update(fn func())
	// SettingsWindow is the Settings window, while it is open, and
	// CloseSettings closes it.
	SettingsWindow() *mygo.Window
	CloseSettings()
}

// QuickAction is the quick terminal's action.
const QuickAction = "quickTerminal.toggle"

// KeyNames are the keys a shortcut may name, as Settings names them.
var KeyNames = uisettings.KeyNames

// Item is the menu item of an action, which runs it on the main thread.
func Item(a App, id string) *mygo.MenuItem {
	act, _ := actions.Find(id)
	return &mygo.MenuItem{
		ID:          id,
		Label:       Title(a, id),
		Accelerator: accelerator(a, act),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			// Closing is the window's in front: Settings closes itself.
			if (id == "pane.close" || id == "session.close") && a.SettingsWindow() != nil && mygo.FocusedWindow() == a.SettingsWindow() {
				a.CloseSettings()
				return
			}
			a.Update(func() { a.RunID(id, nil) })
		},
	}
}

// titles are the actions' titles in the language the app shows; an
// Action's Title stays English, for `hyla actions`.
var titles = map[string]func() string{
	"settings.open":       locales.ActionSettings,
	"cli.install":         locales.ActionInstallCli,
	"app.quitAll":         locales.ActionQuitAll,
	"session.new":         locales.ActionNewSession,
	"session.newHome":     locales.ActionNewSessionHome,
	"session.newWorktree": locales.ActionNewSessionWorktree,
	"project.open":        locales.ActionOpenProject,
	"palette.open":        locales.ActionGoToSession,
	"conversation.resume": locales.ActionResumeConversation,
	"session.attention":   locales.ActionJumpToAttention,
	"session.next":        locales.ActionNextSession,
	"session.previous":    locales.ActionPreviousSession,
	"session.rename":      locales.ActionRenameSession,
	"pane.restart":        locales.ActionRestartPane,
	"pane.close":          locales.ActionClose,
	"session.close":       locales.ActionCloseSession,
	"find.open":           locales.ActionFind,
	"find.next":           locales.ActionFindNext,
	"find.previous":       locales.ActionFindPrevious,
	"pane.split.right":    locales.ActionSplitRight,
	"pane.split.down":     locales.ActionSplitDown,
	"pane.browser":        locales.ActionNewBrowser,
	"timeline.open":       locales.ActionTimelinePane,
	"timeline.window":     locales.ActionTimelineWindow,
	"pane.zoom":           locales.ActionZoomPane,
	"pane.next":           locales.ActionNextPane,
	"pane.previous":       locales.ActionPreviousPane,
	"pane.left":           locales.ActionPaneLeft,
	"pane.right":          locales.ActionPaneRight,
	"pane.up":             locales.ActionPaneAbove,
	"pane.down":           locales.ActionPaneBelow,
	"sidebar.toggle":      locales.ActionToggleSidebar,
	"side.toggle":         locales.ActionTogglePanel,
	"changes.toggle":      locales.ActionToggleChanges,
	"files.toggle":        locales.ActionToggleFiles,
	"info.toggle":         locales.ActionToggleInfo,
	"subagents.toggle":    locales.ActionToggleSubagents,
	"text.bigger":         locales.ActionBigger,
	"text.smaller":        locales.ActionSmaller,
	"text.reset":          locales.ActionActualSize,
	"pet.toggle":          locales.ActionTogglePet,
	QuickAction:           locales.ActionQuickTerminal,
}

// Title is an action's title as the menus, the palette and Settings show
// it.
func Title(a App, id string) string {
	if t, ok := titles[id]; ok {
		return t()
	}
	act, _ := actions.Find(id)
	return act.Title
}

// accelerator is an action's shortcut in the menus: none while the user
// types a new one in Settings, and none of a global one, which the system
// takes before the menus.
func accelerator(a App, act actions.Action) string {
	if a.Recording() != "" || act.Global {
		return ""
	}
	return actions.Keys(act, a.Settings().Shortcuts)
}

// Items returns the items of a menu's actions, in their order, with
// separators where the groups change.
func Items(a App, menu string, groups ...[]string) []*mygo.MenuItem {
	var out []*mygo.MenuItem
	for i, g := range groups {
		if i > 0 {
			out = append(out, mygo.Separator())
		}
		for _, id := range g {
			out = append(out, Item(a, id))
		}
	}
	return out
}

// Menu is the app's menu: its own, the sessions', panes' and views', the
// edit menu's and the window's.
func Menu(a App) *mygo.Menu {
	appMenu := []*mygo.MenuItem{{Role: mygo.RoleAbout}, updater.MenuItem(), mygo.Separator(), Item(a, "settings.open"), Item(a, "cli.install"),
		mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(),
		{Label: locales.MenuQuitApp(mygo.App.Name()), Accelerator: QuitKeys, ToolTip: locales.MenuQuitTip(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
		Item(a, "app.quitAll")}
	// The app menu is macOS's: MyGo leaves it out elsewhere, with its items'
	// shortcuts, so they go in a menu of Hyla's there, without macOS's own.
	hyla := &mygo.MenuItem{Role: mygo.RoleAppMenu, Submenu: appMenu}
	if runtime.GOOS != "darwin" {
		hyla = &mygo.MenuItem{Label: "Hyla", Submenu: []*mygo.MenuItem{Item(a, "settings.open"), Item(a, "cli.install"), updater.MenuItem(), mygo.Separator(),
			{Label: locales.MenuQuit(), Accelerator: QuitKeys, ToolTip: locales.MenuQuitTip(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
			Item(a, "app.quitAll")}}
	}
	view := append(Items(a, "View", []string{"sidebar.toggle", "side.toggle", "changes.toggle", "files.toggle", "info.toggle", "subagents.toggle", "pet.toggle"}, []string{"text.bigger", "text.smaller", "text.reset"}, []string{QuickAction}),
		mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleToggleFullScreen})
	// The inspector is a development build's: a production build leaves it
	// out, and its window's DevTools off, where the item would do nothing.
	if mygo.IsDev() {
		view = append(view, &mygo.MenuItem{Role: mygo.RoleToggleDevTools})
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		hyla,
		{Label: locales.MenuSession(), Submenu: Items(a, "Session",
			[]string{"session.new", "session.newHome", "session.newWorktree", "project.open"},
			[]string{"palette.open", "session.attention", "session.next", "session.previous"},
			[]string{"session.rename", "pane.restart"},
			[]string{"pane.close", "session.close"})},
		// The Edit menu's own items, then finding.
		{Role: mygo.RoleEditMenu, Submenu: append([]*mygo.MenuItem{{Role: mygo.RoleUndo}, {Role: mygo.RoleRedo}, mygo.Separator(),
			{Role: mygo.RoleCut}, {Role: mygo.RoleCopy}, {Role: mygo.RolePaste}, {Role: mygo.RoleSelectAll}, mygo.Separator()},
			Items(a, "Edit", []string{"find.open", "find.next", "find.previous"})...)},
		{Label: locales.MenuPane(), Submenu: Items(a, "Pane",
			[]string{"pane.split.right", "pane.split.down"},
			[]string{"pane.browser", "timeline.open"},
			[]string{"pane.zoom"},
			[]string{"pane.next", "pane.previous", "pane.left", "pane.right", "pane.up", "pane.down"})},
		{Label: locales.MenuView(), Submenu: view},
		{Role: mygo.RoleWindowMenu},
	})
}

// Keys is an action's shortcut as the menus show it, "" when it has none.
func Keys(a App, id string) string {
	act, _ := actions.Find(id)
	return actions.Glyphs(actions.Keys(act, a.Settings().Shortcuts))
}

// Tip is a button's tooltip: what it does, with its action's shortcut.
func Tip(a App, title, id string) string {
	if k := Keys(a, id); k != "" {
		return title + " (" + k + ")"
	}
	return title
}

// Shortcut shows an action's shortcut by a context menu's item.
func Shortcut(a App, it *ui.MenuItem, id string) *ui.MenuItem {
	act, _ := actions.Find(id)
	parts := strings.Split(actions.Keys(act, a.Settings().Shortcuts), "+")
	var mods ui.Modifiers
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "cmdorctrl", "commandorcontrol":
			mods |= ui.Cmd
		case "cmd", "command", "super", "meta", "win":
			mods |= ui.Super
		case "ctrl", "control":
			mods |= ui.Ctrl
		case "alt", "option":
			mods |= ui.Alt
		case "shift":
			mods |= ui.Shift
		}
	}
	for k, name := range KeyNames() {
		if strings.EqualFold(name, parts[len(parts)-1]) {
			return it.Shortcut(mods, k)
		}
	}
	return it
}

// QuitKeys quits the app: Control+Q is the terminal's on Windows and
// Linux.
var QuitKeys = func() string {
	if runtime.GOOS == "darwin" {
		return "CmdOrCtrl+Q"
	}
	return "Ctrl+Shift+Q"
}()
