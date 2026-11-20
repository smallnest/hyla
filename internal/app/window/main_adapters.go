package window

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/platform"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/tray"
	"hyla/locales"

	uisettings "hyla/internal/ui/settings"
)

// The window, as cmd/hyla's program runs it: the hooks the program sets,
// and what it asks the window to do at each.

// New is the window as Hyla starts: its settings read, its terms and
// pages waiting for their hosts.
func New() *App { return newApp() }

// Appearance is the theme the window starts in, and Language its locale.
func (a *App) Appearance() mygo.ThemeSource { return mygo.ThemeSource(a.settingsIn.Appearance) }

func (a *App) Language() string { return uisettings.LocaleTag(a.settingsIn.Language) }

// LanguageChanged draws anew what the language changes: the menu, the
// settings window's title, the pet's, and the tray's menu.
func (a *App) LanguageChanged() {
	a.UpdateNow(func() {
		a.refreshPluginLocaleAndTheme()
		if a.Plugins != nil {
			a.Plugins.ApplyLocale(a.pluginLocaleValue())
			a.invalidatePluginStatusCache()
		}
		mygo.App.SetMenu(a.menu())
		if a.Win != nil {
			a.Win.Invalidate()
		}
		if a.SetWin != nil {
			a.SetWin.SetTitle(locales.SettingsTitle())
			a.SetWin.Invalidate()
		}
		if a.Pet.Win != nil {
			a.Pet.Win.SetTitle(locales.PetName())
		}
		if a.Tray != nil {
			a.Tray.SetMenu(a.trayMenu())
		}
	})
}

// BecomesActive tells that the app came in front, and Resigns that it
// went: each tells the hosts which session the user looks at.
func (a *App) BecomesActive() {
	mygo.ClearNotifications()
	a.UpdateNow(func() { a.Active = true; a.ReportViewNow() })
}

func (a *App) Resigns() { a.UpdateNow(func() { a.Active = false; a.ReportViewNow() }) }

// Activated shows the window when nothing of the app shows.
func (a *App) Activated(hasVisible bool) {
	if !hasVisible {
		a.ShowWindow()
	}
}

// EveryWindowClosed quits, unless the app's icon shows in the menu bar.
func (a *App) EveryWindowClosed() { a.lastWindowClosed() }

// BeforeQuit answers a quit: false takes it back while what it asks is
// answered.
func (a *App) BeforeQuit(e *mygo.QuitEvent) {
	// quitRequested saves the prefs too.
	if !a.quitRequested() {
		e.PreventDefault()
	}
}

// WillQuit destroys the tray's icon, which Windows would leave in the
// notification area until the pointer passes over it.
func (a *App) WillQuit() {
	a.dropAllHongguo()
	a.shutdownPlugins()
	if a.Tray != nil {
		a.Tray.Destroy()
	}
}

// MenuBarOnly starts the app with no Dock icon, for the menu bar alone.
func (a *App) MenuBarOnly() { mygo.App.SetActivationPolicy(tray.Policy(a.settingsIn)) }

// Start begins what the window runs for itself: its hosts, this
// machine's configuration, the terminals the window shows, and the pets.
func (a *App) Start() {
	a.openWindow()
	a.startPluginHooks()
	a.applyTray()
	a.applyGlobalShortcuts()
	a.IdleTime = mygo.Power.IdleTime
	a.syncPet()
	go a.watchReturn()
	a.connect()
	a.syncRemoteHosts()
	go a.watchConfig()
}

// openWindow makes the main window: as Hyla starts, and again after
// closing it left Hyla running in the menu bar or the notification area.
func (a *App) openWindow() {
	a.Win = mygo.NewWindow(platform.Frame(mygo.WindowOptions{
		Title:     "Hyla",
		Width:     1180,
		Height:    760,
		MinWidth:  560,
		MinHeight: 360,
		StateKey:  "main",
		Vibrancy:  vibrancyFor(a.settingsIn),
		Content:   ui.View(a.view),
	}, layout.TopBar, layout.BarMid))
	a.WinOpen = true
	a.UpdateTitle()
	a.Win.OnFocus(func() { a.UpdateNow(func() { a.setFocused(true); a.claimShown(); a.ReportViewNow() }) })
	a.Win.OnBlur(func() { a.UpdateNow(func() { a.setFocused(false); a.ReportViewNow() }) })
	a.Win.OnClosed(a.windowClosed)
}
