package window

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"

	"hyla/internal/ui/menus"
	"hyla/internal/ui/worktree"
)

// The app, as the menus ask it.
var _ menus.App = (*App)(nil)

func (a *App) RunID(id string, args map[string]string) { a.run(id, args) }

func (a *App) CloseSettings() { a.SetWin.Close() }

// item is the menu item of an action.
func (a *App) item(id string) *mygo.MenuItem { return menus.Item(a, id) }

// menu is the app's menu.
func (a *App) menu() *mygo.Menu { return menus.Menu(a) }

// keys is an action's shortcut as the menus show it.
func (a *App) keys(id string) string { return menus.Keys(a, id) }

// tip is a button's tooltip, with its action's shortcut.
func (a *App) tip(title, id string) string { return menus.Tip(a, title, id) }

// menuShortcut shows an action's shortcut by a context menu's item.
func (a *App) menuShortcut(it *ui.MenuItem, id string) *ui.MenuItem { return menus.Shortcut(a, it, id) }

// run runs one of the app's actions, from a menu, the palette or
// `hyla do`, on the main thread.
func (a *App) run(id string, args map[string]string) {
	if !a.fromMay(id, args) {
		return
	}
	s := a.session(a.Selected)
	switch id {
	case "settings.open":
		a.openSettings("")
	case "cli.install":
		a.installCLI()
	case "app.quitAll":
		a.quitAndEnd()
	case "session.new":
		// A directory hyla do names is of the host it runs on.
		switch dir := args["dir"]; {
		case dir != "":
			a.NewSessionInDir(a.actionHost(), dir)
		case a.From != nil:
			a.NewSessionOn(a.From, args["project"])
		default:
			a.SessionIn(args["project"])
		}
	case "session.newHome":
		a.NewSessionInDir(a.Local(), proto.Home())
	case "session.newWorktree":
		worktree.NewSession(a)
	case "project.open":
		a.OpenProjectDialog()
	case "palette.open":
		a.OpenPalette()
	case "session.attention":
		a.jumpToAttention()
	case "session.next":
		a.selectOffset(1)
	case "session.previous":
		a.selectOffset(-1)
	case "session.rename":
		a.StartRename(a.Selected)
	case "pane.restart":
		if s != nil {
			a.RestartSession(a.focusOf(s))
		}
	case "pane.close":
		a.closeFocused()
	case "session.close":
		a.CloseSession(a.Selected, true)
	case "find.open":
		// In the changes panel, its diff's find bar.
		if f := a.Side.Changes.Focus; a.Side.Focused && f != nil && f.D != nil {
			f.D.Find("")
			break
		}
		a.openFind()
	case "find.next":
		a.findStep(false)
	case "find.previous":
		a.findStep(true)
	case "pane.split.right":
		if s == nil {
			a.run("session.new", args)
			return
		}
		a.splitPane(s, "", "right")
	case "pane.split.down":
		a.splitPane(s, "", "down")
	case "pane.browser":
		a.openBrowser(args["url"])
	case "timeline.open":
		a.openTimeline()
	case "timeline.window":
		a.openTimelineWindow()
	case "pane.zoom":
		a.zoomPane(s, "")
	case "pane.next":
		a.cyclePane(1)
	case "pane.previous":
		a.cyclePane(-1)
	case "pane.left", "pane.right", "pane.up", "pane.down":
		a.focusDirection(id[len("pane."):])
	case "sidebar.toggle":
		a.ToggleSidebar()
	case "side.toggle":
		a.toggleSide("")
	case "changes.toggle":
		a.toggleSide(sideChanges)
	case "files.toggle":
		a.toggleSide(sideFiles)
	case "info.toggle":
		a.toggleSide(sideInfo)
	case "subagents.toggle":
		a.toggleSide(sideSubagents)
	case "subagents.open":
		a.openSubagents()
	case "pet.toggle":
		a.settingsIn.Pet = !a.settingsIn.Pet
		a.applySettings(a.settingsIn)
	case "text.bigger":
		a.zoom(1, false)
	case "text.smaller":
		a.zoom(-1, false)
	case "text.reset":
		a.zoom(0, true)
	case QuickAction:
		a.toggleQuick()
	}
	if a.Win != nil {
		a.Win.Invalidate()
	}
}

// quitKeys quits the app: Control+Q is the terminal's on Windows and Linux.
var quitKeys = func() string {
	if runtime.GOOS == "darwin" {
		return "CmdOrCtrl+Q"
	}
	return "Ctrl+Shift+Q"
}()

// fromMay reports whether an action a program asked for (hyla do) may
// run: every one may, of this machine's programs as of the app's.
func (a *App) fromMay(id string, args map[string]string) bool { return true }
