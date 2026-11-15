// Package tray holds the app's icon in the menu bar of macOS, or the
// notification area of Windows and Linux, and the menu it opens: the
// window, Settings, updates, and quitting. While it shows, closing the
// window leaves the app running there.
package tray

import (
	_ "embed"
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"hyla/internal/settings"
	"hyla/internal/ui/menus"
	"hyla/locales"
)

// trayTemplate is the frog of the app's icon, black on transparent for
// macOS to tint: 14 points tall in the menu bar's 18, as other apps'
// icons are, at 2×.
//
//go:embed icons/tray-template.png
var trayTemplate []byte

// trayColor is the app's icon, for the notification area.
//
//go:embed icons/tray.png
var trayColor []byte

// App is what the icon and its menu ask of the app.
type App interface {
	// Settings are the settings, whose icon the tray follows.
	Settings() *settings.Settings
	// Update runs fn on the main thread, ShowWindow brings the main
	// window to the front, and Title is an action's title for the menu.
	Update(fn func())
	ShowWindow()
	Title(id string) string
	// OpenSettings shows the Settings window, and ShowQuick the quick
	// terminal.
	OpenSettingsAt()
	ShowQuick()
	// TrayNow is the tray the app holds, and SetTray holds another, or
	// none.
	TrayNow() *mygo.Tray
	SetTray(t *mygo.Tray)
}

// Shows tells whether the app's icon shows in the menu bar or the
// notification area.
func Shows(s settings.Settings) bool {
	return runtime.GOOS != "darwin" || s.AppIcon != "dock"
}

// Policy is macOS's activation policy for a setting: no Dock icon for the
// menu bar alone.
func Policy(s settings.Settings) mygo.ActivationPolicy {
	if s.AppIcon == "menubar" {
		return mygo.ActivationPolicyAccessory
	}
	return mygo.ActivationPolicyRegular
}

// Change makes the app's icon show where the settings now say, as the
// user chose another place for it.
func Change(a App) {
	if runtime.GOOS == "darwin" {
		mygo.App.SetActivationPolicy(Policy(*a.Settings()))
		// Without a Dock icon, macOS may leave the app inactive behind
		// another's windows: the Settings window the user chose in stays
		// in front.
		mygo.App.Focus()
	}
	Apply(a)
}

// Apply adds or removes the icon in the menu bar or notification area, as
// the settings say, in the running app.
func Apply(a App) {
	switch show := Shows(*a.Settings()); {
	case show && a.TrayNow() == nil:
		icon, template := trayColor, false
		if runtime.GOOS == "darwin" {
			icon, template = trayTemplate, true
		}
		t, err := mygo.NewTray(mygo.TrayOptions{ID: "hyla", Icon: icon, IconIsTemplate: template, ToolTip: mygo.App.Name(), Menu: Menu(a)})
		if err != nil {
			log.Println("hyla: tray:", err)
			return
		}
		// Windows' left click, where the right one opens the menu.
		t.OnClick(a.ShowWindow)
		a.SetTray(t)
	case !show && a.TrayNow() != nil:
		a.TrayNow().Destroy()
		a.SetTray(nil)
	}
}

// Menu is the icon's menu: the window, Settings, updates, and quitting.
func Menu(a App) *mygo.Menu {
	// The updater's window in front, as from the menu bar alone, where the
	// app may not be.
	check := updater.MenuItem()
	click := check.Click
	check.Click = func(it *mygo.MenuItem, w *mygo.Window) {
		mygo.App.Focus()
		click(it, w)
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: locales.TrayOpen(mygo.App.Name()), Click: func(*mygo.MenuItem, *mygo.Window) { a.ShowWindow() }},
		{Label: a.Title(menus.QuickAction), Click: func(*mygo.MenuItem, *mygo.Window) { a.Update(a.ShowQuick) }},
		mygo.Separator(),
		{Label: a.Title("settings.open"), Click: func(*mygo.MenuItem, *mygo.Window) {
			mygo.App.Focus()
			a.Update(a.OpenSettingsAt)
		}},
		check,
		mygo.Separator(),
		{Label: locales.MenuQuitApp(mygo.App.Name()), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}
