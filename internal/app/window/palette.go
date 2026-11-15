package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/menus"
	"hyla/internal/ui/palette"
	uisettings "hyla/internal/ui/settings"
	"hyla/internal/ui/sidebar"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// The app, as the palette finds and runs what it offers.
var _ palette.App = (*App)(nil)

func (a *App) Settings() *settings.Settings { return &a.settingsIn }

func (a *App) Projects() []proto.Project { return a.St.Projects }

func (a *App) Groups() [][]*proto.Session {
	groups := sidebar.Groups(a.order())
	out := make([][]*proto.Session, len(groups))
	for i, g := range groups {
		for _, id := range g.Sessions {
			if s := a.session(id); s != nil {
				out[i] = append(out[i], s)
			}
		}
	}
	return out
}

func (a *App) HostOf(id string) *host.Host { return a.hostOf(id) }

func (a *App) Shown() string { return a.Selected }

func (a *App) Session(id string) *proto.Session { return a.session(id) }

func (a *App) Collapsed() map[string]bool {
	if a.Prefs.Collapsed == nil {
		a.Prefs.Collapsed = map[string]bool{}
	}
	return a.Prefs.Collapsed
}

func (a *App) Pages() []palette.Page {
	pages := make([]palette.Page, len(uisettings.Pages()))
	for i, p := range uisettings.Pages() {
		pages[i] = palette.Page{ID: p.ID, Title: p.Title}
	}
	return pages
}

func (a *App) ActionTitle(id string) string { return menus.Title(a, id) }

func (a *App) Run(id string) { a.run(id, nil) }

func (a *App) OpenSettings(id string) { a.openSettings(id) }

// NewSessionIn starts a session in a project, or in none's folder.
func (a *App) NewSessionIn(projectID string) { a.SessionIn(projectID) }

// NewSession starts a session in a project, or in none's folder.
func (a *App) NewSession(projectID string) { a.SessionIn(projectID) }

// NewSessionInDir starts a session in a directory of a host, in its
// project.
func (a *App) NewSessionInDir(h *host.Host, dir string) {
	a.create(h, proto.CreateParams{Dir: dir}, "")
}

func (a *App) AddPending(h *host.Host, s proto.Session) { a.addPending(h, s) }

func (a *App) SelectSession(id string) { a.selectSession(id) }

func (a *App) ResumeConversation(h *host.Host, cv proto.Conversation) {
	cl := h.Client
	if cl == nil {
		a.toast(locales.HostNotConnected(h.Name))
		return
	}
	p := proto.ConversationParams{Agent: cv.Agent, ID: cv.ID}
	if cur := a.session(a.Selected); cur != nil && a.hostOf(cur.ID) == h {
		p.Cols, p.Rows = cur.Cols, cur.Rows
		if cur.Project == cv.Project {
			p.After = cur.ID
		}
	}
	go func() {
		var s proto.Session
		err := cl.Call("conversation.resume", p, &s)
		a.UpdateNow(func() {
			if err != nil {
				a.toast(err.Error())
				return
			}
			delete(a.Collapsed(), s.Project)
			a.addPending(h, s)
			a.selectSession(s.ID)
		})
	}()
}

func (a *App) RestartDaemon() { a.restartDaemon() }

func (a *App) FocusTerminal() { a.focusTerminal() }

// OpenPalette opens the command palette.
func (a *App) OpenPalette() { palette.Open(&a.Pal, a) }

// openConversations opens the palette on the conversations alone.
func (a *App) openConversations() { palette.OpenConversations(&a.Pal, a) }

func (a *App) paletteItems() []string { return palette.Titles(&a.Pal, a) }

func (a *App) Palette(c *ui.Context, pal *theme.Palette) { palette.View(c, &a.Pal, a, pal) }

func (a *App) confirmView(c *ui.Context, pal *theme.Palette) {
	if a.Confirm != nil && confirm.Dialog(c, pal, &a.Confirm) {
		a.focusTerminal()
	}
}

// Shortcuts are the window's keys that no menu item has.
func (a *App) Shortcuts(c *ui.Context) {
	if a.Pal.Open || a.Picker.Open || a.Confirm != nil || a.WtAsk != nil || a.SshAsk != nil || a.K8sAsk != nil {
		return
	}
	keys := []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}
	for i, k := range keys {
		if c.Shortcut(ui.Cmd, k) {
			ids := sidebar.VisibleOrder(a.order())
			if i < len(ids) {
				a.selectSession(ids[i])
			}
		}
	}
	if c.Shortcut(ui.Ctrl, ui.KeyTab) {
		a.selectOffset(1)
	}
	if c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyTab) {
		a.selectOffset(-1)
	}
	if a.CopyText != "" {
		c.WriteClipboard(a.CopyText)
		c.Toast(locales.ToastCopied())
		a.CopyText = ""
	}
}
