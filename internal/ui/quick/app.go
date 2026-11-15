package quick

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"

	"github.com/egoist/mygo/ui"
	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/side"
	"hyla/internal/ui/theme"
)

// App is what the quick terminal asks of the app it shows a session of.
type App interface {
	// Quick is the quick terminal's state.
	Quick() *State
	// Settings are the settings the app follows, and Recording the
	// shortcut the user types in Settings.
	Settings() *settings.Settings
	Recording() string
	// MainWindow is the app's window, which the panel shows over, and
	// Quitting tells that the app leaves.
	MainWindow() *mygo.Window
	Quitting() bool
	// Update runs fn on the main thread, and Toast tells a failure.
	Update(fn func())
	Toast(msg string)
	// LocalHost, HostOf and SessionOf find the machines and sessions.
	LocalHost() *host.Host
	HostOf(id string) *host.Host
	SessionOf(id string) *proto.Session
	// QuickSession is the session the quick terminal shows, which
	// SetQuickSession keeps, and SavePrefs the settings.
	QuickSession() string
	SetQuickSession(id string)
	SavePrefs()
	AddPending(h *host.Host, s proto.Session)
	// TermOptions are the terminal's options, as a pane's are.
	TermOptions(id string, conn *Gated) terminal.Options
	// The panel's place: Chrome, RightShown and SideWidth, at the side
	// panel's row width.
	Chrome() panes.Chrome
	RightShown() bool
	SideWidth(rowW float32) float32
	// TermThemes are the terminal's colors, light and dark.
	TermThemes() (light, dark *terminal.Theme)
	// ReportView tells the tray what the app shows.
	ReportView()
	// Side is the side panel's state, whose row width decides where the
	// panel shows.
	SideState() *side.State
	// Message says what the panel cannot show.
	PaneMessageOfQuick(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func())
}

// Gated holds back the stream's frames until the terminal takes the size
// of the first, which then shows the session as it is.
type Gated struct {
	*client.Stream
	Ready chan struct{}
}

func (g *Gated) Read(p []byte) (int, error) {
	<-g.Ready
	return g.Stream.Read(p)
}
