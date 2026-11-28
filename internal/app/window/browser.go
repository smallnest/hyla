package window

import (
	"github.com/egoist/mygo"

	"hyla/internal/app/favicon"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/webpane"
	"hyla/locales"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/theme"
)

var _ webpane.ReplayApp = (*App)(nil)

// The browser panes are internal/ui/webpane's: the page, its toolbar and
// its find, which this file answers for.

type webEntry = webpane.Entry

// Icons is the cache the browser panes' icons are kept in.
func (a *App) B2F(v bool) float32 { return layout.B2F(v) }

// ChromeStateOf is where the sidebar's animation puts things, as a
// browser's header reads it.
func (a *App) ChromeStateOf() panes.Chrome {
	return panes.Chrome{SideW: a.ChromeState.SideW, MainPad: a.ChromeState.MainPad, HeaderPad: a.ChromeState.HeaderPad}
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

func (a *App) ensureWeb(p *proto.Pane) *webpane.Entry { return webpane.Ensure(a, p) }

func (a *App) DropWeb(id string) { webpane.DropWeb(a, id) }

func (a *App) paneBrowser(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, p *proto.Pane, top, left, right, multi, focused, hovered bool) bool {
	return webpane.Pane(a, c, pal, bar, shown, s, p, top, left, right, multi, focused, hovered)
}

// sessionOfPane is the session a pane belongs to: nil when none has it.
func (a *App) sessionOfPane(id string) *proto.Session {
	for i := range a.Sessions() {
		if a.Sessions()[i].PaneByID(id) != nil {
			return &a.Sessions()[i]
		}
	}
	return nil
}

func (a *App) Shows(id string) bool { return webpane.Shows(a, id) }

func (a *App) findInPage(e *webpane.Entry, next, back bool) { webpane.FindInPage(a, e, next, back) }

func (a *App) ReplayAskNow() *webpane.ReplayAsk { return a.ReplayAsk }

func (a *App) SetReplayAsk(d *webpane.ReplayAsk) { a.ReplayAsk = d }

func (a *App) ToggleBrowserRecord(paneID string) {
	e := a.Webs[paneID]
	if e == nil {
		return
	}
	h := a.hostOf(paneID)
	if h == nil || h.Client == nil {
		a.toast(locales.HostNotConnected(a.selectedHost().Label()))
		return
	}
	cl := h.Client
	stop := e.Recording
	go func() {
		method := "browser.record.start"
		if stop {
			method = "browser.record.stop"
		}
		var st proto.BrowserRecordStatus
		err := cl.Call(method, proto.BrowserRecordParams{Pane: paneID}, &st)
		a.Main(func() {
			if err != nil {
				a.toast(err.Error())
				return
			}
			cur := a.Webs[paneID]
			if cur == nil {
				return
			}
			cur.Recording = st.Recording
			cur.RecordName = st.Name
			cur.RecordSteps = st.Steps
			if st.Recording {
				a.toast(locales.BrowserRecordStartedToast())
			} else if st.SavedPath != "" {
				cur.LastFlowPath = st.SavedPath
				a.toast(locales.BrowserRecordSavedToast(st.SavedPath))
			}
		})
	}()
}

func (a *App) OpenBrowserReplay(paneID string) {
	h := a.hostOf(paneID)
	workDir, lastFlow := "", ""
	if s := a.sessionOfPane(paneID); s != nil {
		if p := s.PaneByID(paneID); p != nil {
			workDir = p.WorkDir()
		}
	}
	if e := a.Webs[paneID]; e != nil {
		lastFlow = e.LastFlowPath
	}
	webpane.OpenReplay(a, h, paneID, workDir, lastFlow)
}

func (a *App) RecordBrowserNav(paneID, op, url string) {
	h := a.hostOf(paneID)
	if h == nil || h.Client == nil {
		return
	}
	if e := a.Webs[paneID]; e != nil {
		e.RecordSteps++
	}
	h.Client.Go("browser.record.step", proto.BrowserParams{
		ID:   paneID,
		Op:   op,
		Text: url,
	}, nil)
}
