package webpane

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/favicon"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/theme"
)

// App is what the browser panes ask of the app.
type App interface {
	// Icons is the cache the icons are kept in.
	Icons() *favicon.Cache
	// MainWindow is the window the panes show in, nil before it opens.
	MainWindow() *mygo.Window
	// WebsNow are the pages, by pane, and Update runs fn on the main
	// thread.
	WebsNow() map[string]*Entry
	Update(fn func())
	// Settings are the settings, and AdRules the pages' rules.
	Settings() *settings.Settings
	AdRules() []*mygo.ContentRules
	SessionOfPane(pane string) *proto.Session
	// FocusOf and SetFocus are the pane the user is in; PaneRef its
	// handle.
	FocusOf(s *proto.Session) string
	SetFocus(s *proto.Session, pane string)
	PaneRef(id string) *ui.Handle
	CallPane(id, method string, params any, done func(err error))
	PaneMessage(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func())
	// The find bar's pane and query, which SetFind sets, the CloseFind
	// that closes it, and Tip.
	FindPane() string
	FindQuery() string
	// FindInput is the find bar's field.
	FindInput() *ui.Handle
	CloseFindBar()
	Tip(title, id string) string
	// B2F is a bool as 0 or 1, for an animation's value.
	B2F(v bool) float32
	// HeaderHeightIn is the height of a card's header, and RunLater runs
	// fn after the frame.
	RunLater(fn func())
	HeaderHeightIn() float32
	// ChromeStateOf is where the sidebar's animation puts things.
	ChromeStateOf() panes.Chrome
	// Panes is the app, as the panes' buttons ask it.
	Panes() panes.App
	// ToggleBrowserRecord starts or stops recording browser verification steps.
	ToggleBrowserRecord(paneID string)
	// OpenBrowserReplay opens the dialog to replay a saved verification flow.
	OpenBrowserReplay(paneID string)
	// RecordBrowserNav records a user-initiated toolbar/address-bar navigation step.
	RecordBrowserNav(paneID, op, url string)
}
