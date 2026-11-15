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
	// Icons is the cache the icons are kept in, and IconCache the field
	// it is.
	Icons() *favicon.Cache
	IconCache() *favicon.Cache
	// MainWindow is the window the panes show in, nil before it opens.
	MainWindow() *mygo.Window
	// WebsNow are the pages, by pane, and Update runs fn on the main
	// thread.
	WebsNow() map[string]*Entry
	Update(fn func())
	// Settings are the settings, and AdRules the pages' rules.
	Settings() *settings.Settings
	AdRules() []*mygo.ContentRules
	// Sessions are the sessions the app knows, and SelectedSession the
	// session the window shows.
	Sessions() []proto.Session
	SelectedSession() *proto.Session
	// FocusOf and SetFocus are the pane the user is in; PaneRef its
	// handle.
	FocusOf(s *proto.Session) string
	SetFocus(s *proto.Session, pane string)
	PaneRef(id string) *ui.Handle
	// Split makes a pane beside another, CallPane asks hylad of a pane,
	// and PaneMessage says what a pane cannot show.
	Split(s *proto.Session, p proto.CreateParams)
	CallPane(id, method string, params any, done func(err error))
	PaneMessage(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func())
	// The find bar's pane and query, which SetFind sets, the CloseFind
	// that closes it, and Tip.
	FindPane() string
	FindQuery() string
	// FindInput is the find bar's field, and SetFindText its text.
	FindInput() *ui.Handle
	SetFindText(q string)
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
}
