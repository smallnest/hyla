package main

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/palette"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// The app, as the palette finds and runs what it offers.
var _ palette.App = (*App)(nil)

func (a *App) Settings() *settings.Settings { return &a.settingsIn }

func (a *App) Projects() []proto.Project { return a.St.Projects }

func (a *App) Groups() [][]*proto.Session {
	groups := a.groups()
	out := make([][]*proto.Session, len(groups))
	for i, g := range groups {
		out[i] = g.sessions
	}
	return out
}

func (a *App) HostOf(id string) *host.Host { return a.hostOf(id) }

func (a *App) Local() *host.Host { return a.local() }

func (a *App) Shown() string { return a.Selected }

func (a *App) Session(id string) *proto.Session { return a.session(id) }

func (a *App) Collapsed() map[string]bool {
	if a.Prefs.Collapsed == nil {
		a.Prefs.Collapsed = map[string]bool{}
	}
	return a.Prefs.Collapsed
}


func (a *App) Pages() []palette.Page {
	pages := make([]palette.Page, len(settingsPages))
	for i, p := range settingsPages {
		pages[i] = palette.Page{ID: p.id, Title: p.title()}
	}
	return pages
}

func (a *App) ActionTitle(id string) string { return actionTitle(id) }

func (a *App) Run(id string) { a.run(id, nil) }

func (a *App) OpenSettings(id string) { a.openSettings(id) }

func (a *App) NewSession(projectID string) { a.newSession(projectID) }

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
		a.update(func() {
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

// openPalette opens the command palette.
func (a *App) openPalette() { palette.Open(&a.Pal, a) }

// openConversations opens the palette on the conversations alone.
func (a *App) openConversations() { palette.OpenConversations(&a.Pal, a) }

func (a *App) paletteItems() []string { return palette.Titles(&a.Pal, a) }

func (a *App) paletteView(c *ui.Context, pal *theme.Palette) { palette.View(c, &a.Pal, a, pal) }

func (a *App) confirmView(c *ui.Context, pal *theme.Palette) {
	if a.Confirm != nil && confirmDialog(c, pal, &a.Confirm) {
		a.focusTerminal()
	}
}

// confirmDialog asks *pending's question in c's window, and clears it once
// answered, which it reports.
func confirmDialog(c *ui.Context, pal *theme.Palette, pending **confirmation) bool {
	cf := *pending
	run, alt := false, false
	ui.DialogBase(c.Key("confirm"), &cf.open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		width := float32(380)
		if cf.alt != "" {
			width = 460 // room for a third button
		}
		style.Floating(panel.Width(width).Padding(22).Gap(8).Radius(14).Background(pal.Popover), pal).Label(cf.title)
		ui.Text(c, cf.title).FontSize(15).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, cf.detail).FontSize(13).TextColor(pal.TextMuted).LineHeight(1.4)
		ui.Row(c).Gap(8).Justify(ui.End).Margin(14, 0, 0, 0).Children(func() {
			if cf.ends {
				// Return may be what a program reads, so this question
				// takes a click, or Escape: the focus goes to the panel,
				// not to Cancel, which a Return would press.
				ui.Box(c).Focus()
				if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
					cf.open = false
				}
				if style.Button(c, "ok", cf.action, "", true, pal) {
					run = true
				}
				return
			}
			if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				cf.open = false
			}
			if cf.alt != "" && style.Button(c, "alt", cf.alt, "", false, pal) {
				alt = true
			}
			if style.DefaultButton(c, "ok", cf.action, "↩", true, pal) || c.Shortcut(0, ui.KeyEnter) {
				run = true
			}
		})
		if !cf.ends && c.Shortcut(0, ui.KeyEnter) {
			run = true
		}
	})
	if run {
		cf.open = false
		cf.run()
	} else if alt {
		cf.open = false
		cf.altRun()
	}
	// Only once the answer came: what it runs may clear *pending itself,
	// which removeHost does as the page, rebuilt, sees the host gone.
	// Another question asked in its place, as the page's own, stays.
	if cf.open {
		return false
	}
	if *pending == cf {
		*pending = nil
	}
	return true
}

// shortcuts are the window's keys that no menu item has.
func (a *App) shortcuts(c *ui.Context) {
	if a.Pal.Open || a.Picker.Open || a.Confirm != nil || a.WtAsk != nil || a.SshAsk != nil || a.K8sAsk != nil {
		return
	}
	keys := []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}
	for i, k := range keys {
		if c.Shortcut(ui.Cmd, k) {
			ids := a.visibleOrder()
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
