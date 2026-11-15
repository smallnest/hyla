package window

import (
	"github.com/egoist/mygo"

	"hyla/internal/ui/tray"
)

// The app, as the icon in the menu bar or the notification area asks it.
var _ tray.App = (*App)(nil)

func (a *App) ShowWindow() { a.showWin() }

func (a *App) QuitItem(id string) string { return a.ActionTitle(id) }

func (a *App) OpenSettingsAt() { a.openSettings("") }

func (a *App) ShowQuick() { a.showQuick() }

func (a *App) TrayNow() *mygo.Tray { return a.Tray }

func (a *App) SetTray(t *mygo.Tray) { a.Tray = t }

// changeAppIcon shows the app's icon where the settings now say.
func (a *App) changeAppIcon() { tray.Change(a) }

// applyTray adds or removes the app's icon, as the settings say.
func (a *App) applyTray() { tray.Apply(a) }

// trayMenu is the icon's menu.
func (a *App) trayMenu() *mygo.Menu { return tray.Menu(a) }

// showWin brings the main window to the front, made anew if it was
// closed.
func (a *App) showWin() {
	if a.Win == nil {
		return
	}
	if !a.WinOpen {
		a.openWindow()
	}
	if a.Win.IsMinimized() {
		a.Win.Restore()
	}
	a.Win.Show()
	a.Win.Focus()
	mygo.App.Focus()
}

// windowClosed lets go of what the main window showed: its panes'
// terminals and pages, which a window made anew attaches again.
func (a *App) windowClosed() {
	a.WinOpen = false
	a.setFocused(false)
	for id := range a.Terms {
		a.dropTerm(id)
	}
	for id := range a.Webs {
		a.dropWeb(id)
	}
	a.reportView()
}

// lastWindowClosed quits, unless the app's icon shows in the menu bar or
// the notification area, which opens the window again. A Linux desktop
// without a tray has no icon.
func (a *App) lastWindowClosed() {
	if a.Tray == nil {
		mygo.App.Quit()
	}
}

// Title is an action's title, for the icon's menu.
func (a *App) Title(id string) string { return a.ActionTitle(id) }
