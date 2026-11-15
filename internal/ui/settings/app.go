package settings

import (
	"hyla/internal/app/host"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/settings"
	"hyla/internal/ui/settings/search"
	"hyla/internal/ui/theme"
)

// App is what the window asks of the app it shows in.
type App interface {
	// SettingsState is what the window keeps: its page, its fields and its
	// questions.
	SettingsState() *State
	// Settings is the settings the controls edit.
	Settings() *settings.Settings
	// SettingsApplied is the settings as last applied and saved.
	SettingsApplied() settings.Settings
	// ApplySettings makes settings the app's, saving them.
	ApplySettings(s settings.Settings)
	// Main runs fn on the main thread.
	Main(fn func())
	// SettingsWindow is the window, while it is open.
	SettingsWindow() *mygo.Window
	// SetSettingsWindow holds the window, or none.
	SetSettingsWindow(w *mygo.Window)
	// RootWindow is the app's own window.
	RootWindow() *mygo.Window
	// WindowControls draws the window's buttons on the right of its title,
	// where the system does not.
	WindowControls(c *ui.Context, pal *theme.Palette)
	// TopInset is the height of the app's title bar, and BarMid where its
	// title and controls sit.
	TopInset() float32
	BarMid() float32
	// Vibrancy is the window's material, for settings.
	Vibrancy(s settings.Settings) mygo.Vibrancy
	// Borderless is the theme a window's edge-less content draws with.
	Borderless(c *ui.Context, pal *theme.Palette) *ui.Theme
	// HeaderHeight is the height of the Settings pages' titles.
	HeaderHeight() float32
	// CardGap is the room between a window's cards and its edge.
	CardGap() float32
	// CLIPath is where hyla is installed, or none.
	CLIPath() string
	// InstallCLI installs hyla.
	InstallCLI()
	// HostCount is how many machines the app has, this one among them.
	HostCount() int
	// Keys is an action's shortcut, as the app shows it.
	Keys(id string) string
	// Tip adds an action's shortcut, in words, to a title.
	Tip(title, id string) string
	// ActionTitle names an action.
	ActionTitle(id string) string
	// StartRecording tells that a shortcut is typed, or that none is.
	StartRecording(id string)
	// RecordKey takes a key typed for an action's shortcut.
	RecordKey(act actions.Action, mods ui.Modifiers, key ui.Key) bool
	// SetShortcut binds an action's shortcut.
	SetShortcut(id, accel string)
	// RefreshAgents reads again where hyla is installed.
	RefreshAgents()
	// RefreshService asks hylad how it is.
	RefreshService()
	// RestartDaemon restarts hylad.
	RestartDaemon()
	// Menu is the app's menu, which gives up the shortcuts while one is
	// typed.
	Menu() *mygo.Menu
	// ApplyGlobalShortcuts binds the actions the system takes.
	ApplyGlobalShortcuts()
	// OpenConfig opens the configuration in the user's editor.
	OpenConfig()
	// PickSidebarImage and PickPaneImage choose the sidebar's and the
	// panes' image.
	PickSidebarImage()
	PickPaneImage()
	// PetGallery names the pet shown, and PetChooser chooses it.
	PetGallery() string
	PetChooser(c *ui.Context, pal *theme.Palette)
	// PetGrow makes the pet the size its slider shows.
	PetGrow()
	// TermThemes are the app's terminal themes, light and dark.
	TermThemes() (light, dark *terminal.Theme)
	// FontFeatures are the terminal's font features.
	FontFeatures() []string
	// FallbackFonts are the fonts tried after the terminal's own.
	FallbackFonts() []string
	// AddFallback adds a fallback font, or removes the one at at.
	AddFallback(font string)
	// RemoveFallback removes the fallback font at.
	RemoveFallback(at int)
	// BundledFont is the font hyla brings.
	BundledFont() string
	// UIFeatures converts features as terminal.Font takes them to those
	// that ui.Span.Features takes.
	UIFeatures(fs []string) string
	// Sync, Feishu, Tailcat and P2P are the pages' and the hosts' forms,
	// which stay in the root.
	Sync(c *ui.Context, pal *theme.Palette)
	Feishu(c *ui.Context, pal *theme.Palette)
	Tailcat(c *ui.Context, pal *theme.Palette)
	P2P(c *ui.Context, pal *theme.Palette)
	// PetKnob notes the pet's slider, as the pet grows.
	PetKnob()
	// Local is this machine, and CallOn asks it something, on the main
	// thread.
	Local() *host.Host
	CallOnLocal(method string, params any, done func(err error))
}

// RunOnMain runs fn on the app's main thread, and draws.
func RunOnMain(x App, fn func()) { x.Main(fn) }

// Invalidate makes the window draw again, when it is open.
func Invalidate(x App) {
	if w := x.SettingsWindow(); w != nil {
		w.Invalidate()
	}
}

// DurationWords tells a duration in words.
func DurationWords(d time.Duration) string { return durationWords(d) }

// Refresh agents and the service, as the page asks.
func RefreshAgents(x App)  { refreshAgents(x) }
func RefreshService(x App) { refreshService(x) }

// Pages are the window's pages, as the command palette lists them.
type Page struct {
	ID    string
	Title string
}

// Pages returns the window's pages.
func Pages() []Page {
	pages := make([]Page, len(settingsPages))
	for i, p := range settingsPages {
		pages[i] = Page{ID: p.id, Title: p.title()}
	}
	return pages
}

// CheckUpdates asks the updater for a newer app.
func CheckUpdates() { checkUpdates() }

// FontOptions are the families the font menu offers.
func FontOptions() []string { return fontOptions() }

// CJKFamily tells, by the name, that a family draws Chinese, Japanese or
// Korean.
func CJKFamily(name string) bool { return cjkFamily(name) }

// BackgroundName names a wash as Settings offers it.
func BackgroundName(id, name string) string { return backgroundName(id, name) }

// SearchResults returns what a query finds, by page, for the pages and the
// app's titles.
func SearchResults(x App, query string) map[string][]string {
	return search.Results(query, x.Settings().Shortcuts, searchPages(), x.ActionTitle)
}

// FontChoices are the fonts the picker offers: hyla's own first.
func FontChoices() []string { return fontChoices }

// StartRecording, RecordKey and SetShortcut are the keyboard page's, which
// the root's menu reads too.
func StartRecording(x App, id string)                                  { setRecording(x, id) }
func RecordKey(x App, a actions.Action, m ui.Modifiers, k ui.Key) bool { return recordKey(x, a, m, k) }
func SetShortcut(x App, id, accel string)                              { setShortcut(x, id, accel) }

// OpenConfig opens the configuration in the user's editor.
func OpenConfig(x App) { openConfig(x) }
