package main

import (
	"github.com/egoist/mygo"

	"hyla/internal/app/favicon"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/webpane"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/theme"
)

// The browser panes are internal/ui/webpane's: the page, its toolbar and
// its find, which this file answers for.

type webEntry = webpane.Entry

// Icons is the cache the browser panes' icons are kept in.
func (a *App) B2F(v bool) float32 { return b2f(v) }

// ChromeStateOf is where the sidebar's animation puts things, as a
// browser's header reads it.
func (a *App) ChromeStateOf() panes.Chrome {
	return panes.Chrome{SideW: a.ChromeState.sideW, MainPad: a.ChromeState.mainPad, HeaderPad: a.ChromeState.headerPad}
}

func (a *App) FindQuery() string { return a.Find.Query }

func (a *App) FindInput() *ui.Handle { return &a.Find.Input }

func (a *App) SetFindText(q string) {
	a.Find.Query = q
	a.Find.Focus = true
}

func (a *App) CloseFindBar() { a.closeFind() }

func (a *App) Panes() panes.App { return a }

func (a *App) WebsNow() map[string]*webpane.Entry { return a.Webs }

func (a *App) AdRules() []*mygo.ContentRules { return a.adRules() }

func (a *App) Icons() *favicon.Cache { return &a.IconsCache }

// IconCache is the field Icons answers, which the pages read where the
// app's method is another.
func (a *App) IconCache() *favicon.Cache { return &a.IconsCache }

// openBrowser opens a browser pane beside the pane the user is in, or the
// address bar of the browser they are in.
func (a *App) openBrowser(url string) {
	s := a.session(a.Selected)
	if s == nil {
		return
	}
	at := a.focusOf(s)
	if e := a.Webs[at]; e != nil && url == "" {
		e.FocusAddress = true
		return
	}
	if url == "" {
		url = proto.BlankURL
	}
	a.split(s, proto.CreateParams{Split: at, Direction: "right", URL: url})
}

// focusAddress gives the address bar of the browser the user is in the
// keyboard.
func (a *App) focusAddress() {
	if s := a.session(a.Selected); s != nil {
		if e := a.Webs[a.focusOf(s)]; e != nil {
			e.FocusAddress = true
		}
	}
}

func (a *App) ensureWeb(p *proto.Pane) *webpane.Entry { return webpane.Ensure(a, p) }

func (a *App) dropWeb(id string) { webpane.DropWeb(a, id) }

func (a *App) paneBrowser(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, p *proto.Pane, top, left, right, multi, focused, hovered bool) bool {
	return webpane.Pane(a, c, pal, bar, shown, s, p, top, left, right, multi, focused, hovered)
}

func (a *App) sessionOfPane(id string) *proto.Session { return webpane.SessionOfPane(a, id) }

func (a *App) shows(id string) bool { return webpane.Shows(a, id) }

func (a *App) findInPage(e *webpane.Entry, next, back bool) { webpane.FindInPage(a, e, next, back) }
