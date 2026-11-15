package window

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/platform"
	"hyla/internal/settings"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/petme"
	uisettings "hyla/internal/ui/settings"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

var _ uisettings.App = (*App)(nil)

// HostCount is how many machines the app has.
func (a *App) HostCount() int { return len(a.Machines) }

// HeaderHeight is the height of the Settings pages' titles.
func (a *App) HeaderHeight() float32 { return 2 * (layout.BarMid - layout.CardGap) }

// openSettings shows the Settings window, on a page.
func (a *App) openSettings(page string) {
	uisettings.Open(a, page, func(content any) {
		a.SetWin = mygo.NewWindow(platform.Frame(mygo.WindowOptions{
			Title: locales.SettingsTitle(), Width: 820, Height: 620, MinWidth: 700, MinHeight: 480,
			StateKey: "settings",
			Vibrancy: vibrancyFor(a.settingsIn),
			Content:  ui.View(content.(func(c *ui.Context))),
		}, layout.TopBar, layout.BarMid))
	})
}

// SettingsState, Settings and SettingsApplied are the window's state, its
// settings and those as last applied.
func (a *App) SettingsState() *uisettings.State           { return &a.Ss }
func (a *App) SettingsApplied() settings.Settings         { return a.Applied }
func (a *App) Main(fn func())                             { a.main(fn) }
func (a *App) SettingsWindow() *mygo.Window               { return a.SetWin }
func (a *App) SetSettingsWindow(w *mygo.Window)           { a.SetWin = w }
func (a *App) RootWindow() *mygo.Window                   { return a.Win }
func (a *App) TopInset() float32                          { return layout.TopBar }
func (a *App) BarMid() float32                            { return layout.BarMid }
func (a *App) CardGap() float32                           { return layout.CardGap }
func (a *App) Vibrancy(s settings.Settings) mygo.Vibrancy { return vibrancyFor(s) }

func (a *App) CallOnLocal(method string, params any, done func(err error)) {
	a.callOn(a.Local(), method, params, done)
}

// WindowControls draws the window's own buttons, where the system draws
// none.
func (a *App) WindowControls(c *ui.Context, pal *theme.Palette) { windowControls(c, pal, a.SetWin) }

// Borderless is the theme a window's edge-less content draws with.
func (a *App) Borderless(c *ui.Context, pal *theme.Palette) *ui.Theme { return borderless(c, pal) }

// CLIPath is where hyla is installed, or none; InstallCLI installs it.
func (a *App) CLIPath() string { return cliInstalled() }
func (a *App) InstallCLI()     { a.installCLI() }

// Keys, Tip and ActionTitle name an action and its shortcut.
func (a *App) Keys(id string) string       { return a.keys(id) }
func (a *App) Tip(title, id string) string { return a.tip(title, id) }
func (a *App) StartRecording(id string)    { recording(a, id) }

func (a *App) SetShortcut(id, accel string)                 { setShortcut(a, id, accel) }
func (a *App) RefreshAgents()                               { refreshAgents(a) }
func (a *App) RefreshService()                              { refreshService(a) }
func (a *App) Menu() *mygo.Menu                             { return a.menu() }
func (a *App) ApplyGlobalShortcuts()                        { a.applyGlobalShortcuts() }
func (a *App) OpenConfig()                                  { openConfig(a) }
func (a *App) PickSidebarImage()                            { a.pickSidebarImage() }
func (a *App) PickPaneImage()                               { a.pickPaneImage() }
func (a *App) PetGallery() string                           { return petme.Gallery(a) }
func (a *App) PetChooser(c *ui.Context, pal *theme.Palette) { petme.Chooser(a, c, pal) }
func (a *App) PetGrow()                                     { petme.GrowRead(a) }
func (a *App) PetKnob()                                     { a.Ss.PetKnob = true }

// TermThemes are the terminal's colors, light and dark.
func (a *App) TermThemes() (light, dark *terminal.Theme) { return a.termThemes() }

// FontFeatures are the terminal's font features, and UIFeatures converts
// them as the UI takes them.
func (a *App) FontFeatures() []string        { return a.fontFeatures() }
func (a *App) UIFeatures(fs []string) string { return uiFeatures(fs) }
func (a *App) BundledFont() string           { return bundledFont }

// FallbackFonts are the fonts tried after the terminal's own.
func (a *App) FallbackFonts() []string { return a.fallbackFonts() }

// AddFallback adds a fallback font; RemoveFallback removes the one at.
func (a *App) AddFallback(font string) { a.addFallback(font, -1) }
func (a *App) RemoveFallback(at int)   { a.removeFallback(at) }

// Sync, Feishu, Tailcat and P2P are the pages and the forms the root keeps.
func (a *App) Sync(c *ui.Context, pal *theme.Palette)    { a.syncPage(c, pal) }
func (a *App) Feishu(c *ui.Context, pal *theme.Palette)  { uisettings.Feishu(a, c, pal) }
func (a *App) Tailcat(c *ui.Context, pal *theme.Palette) { a.tailcatGroup(c, pal) }
func (a *App) P2P(c *ui.Context, pal *theme.Palette)     { a.p2pGroup(c, pal) }

// The rest of what the root kept of the window.
func (a *App) checkUpdates()                        { uisettings.CheckUpdates() }
func (a *App) durationWords(d time.Duration) string { return uisettings.DurationWords(d) }

// recording starts or ends typing a shortcut.
func recording(a *App, id string) { uisettings.StartRecording(a, id) }

// setShortcut binds an action's shortcut.
func setShortcut(a *App, id, accel string) { uisettings.SetShortcut(a, id, accel) }

// refreshAgents reads again where hyla is installed; refreshService asks
// hylad how it is; openConfig opens the configuration.
func refreshAgents(a *App)  { uisettings.RefreshAgents(a) }
func refreshService(a *App) { uisettings.RefreshService(a) }
func openConfig(a *App)     { uisettings.OpenConfig(a) }
