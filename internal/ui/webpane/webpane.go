package webpane

import (
	"errors"
	"runtime"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/webicon"
	"hyla/locales"
)

// Entry is a browser pane's web view in the app: hylad keeps the pane's
// place and page, the app the page itself, which it loads where hylad
// last had it and tells hylad of as the user goes to others.
type Entry struct {
	View *mygo.WebView
	Err  string
	// Loads are what the page's next load reports to, as hyla browser
	// waits on it.
	Loads []chan error
	// URL and Title are the page's, as it last committed; Told is what
	// hylad was last told.
	URL, Title string
	Told       [2]string
	Loading    bool
	Back, Fwd  bool
	// Address is the address bar's text: the page's URL, unless the user
	// is typing in it (editing); FocusAddress has it take the keyboard,
	// its text selected, as a blank page asks for.
	Address      string
	Editing      bool
	FocusAddress bool
	// Focused tells that the web view was given the keyboard since its
	// element took the focus, which a press on its page takes back.
	Focused bool
	// The find bar's matches: a search runs off the main thread, and the
	// last asked for wins.
	Found    mygo.FindResult
	FindSeq  int
	FindText string
	// IconURL is the URL of the icon the page told: until it does, its
	// site's last shows. Icon is the page, as the icons ask it.
	IconURL string
	Icon    webicon.Site
	// Staged lays the page out unseen, for hyla browser screenshot of a
	// pane never shown, which has no size, and so no picture.
	Staged bool
	// Browser verification flow recording & replay state for this pane.
	Recording    bool
	RecordName   string
	RecordSteps  int
	LastFlowPath string
	Replaying    bool
}

// browserUA makes the page look like Safari's on macOS: sites serve a bare
// WKWebView, whose user agent names no browser, older pages.
func browserUA() string {
	if runtime.GOOS == "darwin" {
		return "Version/18.0 Safari/605.1.15"
	}
	return ""
}

// ensureWeb makes the web view of a browser pane, on the page hylad has.
// Ensure makes the web view of a browser pane, on the page hylad has.
func Ensure(a App, p *proto.Pane) *Entry {
	if e := a.WebsNow()[p.ID]; e != nil {
		return e
	}
	e := &Entry{URL: p.URL, Title: p.Title, Address: p.URL, Told: [2]string{p.URL, p.Title}}
	if p.URL == proto.BlankURL {
		e.Address, e.FocusAddress = "", true
	}
	a.WebsNow()[p.ID] = e
	if a.MainWindow() == nil {
		return e
	}
	v, err := a.MainWindow().NewWebView(mygo.WebViewOptions{
		Page: mygo.PageOptions{DevTools: mygo.DevToolsEnabled, UserAgentSuffix: browserUA(), ContentRules: a.AdRules()},
	})
	if err != nil {
		e.Err = err.Error()
		return e
	}
	e.View = v
	id := p.ID
	page := v.Page()
	// The page's events come on the main thread: they change what the
	// pane shows, and what hylad keeps.
	page.OnDidNavigate(func(url string) {
		a.Update(func() {
			if a.WebsNow()[id] != e {
				return
			}
			e.URL, e.Err, e.IconURL = url, "", ""
			if e.View != nil {
				e.Icon = webicon.Site{URL: url, Page: e.View.Page()}
			}
			if !e.Editing {
				e.Address = url
			}
			e.SyncNav()
			TellPage(a, id, e)
		})
	})
	page.OnPageTitleUpdated(func(t *mygo.TitleEvent) {
		a.Update(func() {
			if a.WebsNow()[id] != e {
				return
			}
			e.Title = t.Title
			// A page of an app changes its URL without navigating, and its
			// title with it.
			if url := page.URL(); url != "" {
				e.URL, e.Icon.URL = url, url
				if !e.Editing {
					e.Address = url
				}
			}
			e.SyncNav()
			TellPage(a, id, e)
			// A page of an app changes its icon as it does its title.
			webicon.Found(a, &e.Icon)
		})
	})
	page.OnDidFinishLoad(func() {
		a.Update(func() {
			e.SyncNav()
			e.LoadEnded(nil)
			webicon.Found(a, &e.Icon)
		})
	})
	page.OnDidFailLoad(func(err *mygo.LoadError) {
		a.Update(func() {
			e.SyncNav()
			// A navigation the user left before it ended fails too: the
			// one that left it ends later.
			if err.Code != -999 && err.Description != "" {
				e.Err = err.Description
				e.LoadEnded(errors.New(err.Description))
			}
		})
	})
	page.OnRenderProcessGone(func(string) { page.Reload() })
	if p.URL != proto.BlankURL {
		e.Load(p.URL)
	}
	return e
}

// syncNav reads where the page can go and whether it loads.
// SyncNav reads where the page can go and whether it loads.
func (e *Entry) SyncNav() {
	if e.View == nil || e.View.IsDestroyed() {
		return
	}
	page := e.View.Page()
	e.Back, e.Fwd, e.Loading = page.CanGoBack(), page.CanGoForward(), page.IsLoading()
}

// Load goes to what the user typed, as a URL or a search.
func (e *Entry) Load(text string) {
	u := proto.BrowserURL(text)
	e.URL, e.Address, e.Err = u, u, ""
	if e.View == nil {
		return
	}
	if err := e.View.Page().LoadURL(u); err != nil {
		e.Err = err.Error()
	}
	e.Loading = true
}

// Shell is what a page of a browser pane asks of the app, as the
// commands a shell sends it drive it.
type Shell interface {
	App
	// EntryOf is the page of a pane: nil when the pane has none.
	EntryOf(pane string) *Entry
	// TellPage tells hylad the page a browser pane shows.
	TellPage(pane string, e *Entry)
	// UpdateNow runs fn on the main thread.
	UpdateNow(fn func())
}

// TellPage tells hylad the page a browser pane shows, which it keeps for
// a restore, and the title that names the pane.
func TellPage(a App, id string, e *Entry) {
	if e.URL == "" || e.Told == [2]string{e.URL, e.Title} {
		return
	}
	e.Told = [2]string{e.URL, e.Title}
	if s := a.SessionOfPane(id); s != nil {
		if p := s.PaneByID(id); p != nil {
			p.URL, p.Title = e.URL, e.Title
		}
	}
	a.CallPane(id, "pane.page", proto.PageParams{ID: id, URL: e.URL, Title: e.Title}, nil)
}

// DropWeb destroys a pane's page, and forgets it.
func DropWeb(a App, id string) {
	if e := a.WebsNow()[id]; e != nil {
		if e.View != nil {
			e.View.Destroy()
		}
		delete(a.WebsNow(), id)
	}
}

// paneBrowser is a browser pane: its toolbar, which is its header, over
// its page; it reports whether the pane is being dragged by its toolbar.
// Pane is a browser pane: its toolbar, which is its header, over its
// page; it reports whether the pane is being dragged by its toolbar.
func Pane(a App, c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, p *proto.Pane, top, left, right, multi, focused, hovered bool) (dragging bool) {
	e := Ensure(a, p)
	ref := a.PaneRef(p.ID)
	// What the page was as the build began: the address bar, which holds
	// the pane's handle while there is no page, may go to one as it
	// builds, and the page then takes the handle in the next build.
	blank := e.URL == proto.BlankURL
	var tools ui.Element
	tools, dragging = browserBar(a, c, pal, bar, shown, s, p, e, top, left, right, multi, focused, hovered)
	if blank && e.URL != proto.BlankURL {
		c.Invalidate()
	}
	ui.Box(c.Key("body")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		switch {
		case blank:
			// Nothing to show yet: the address bar asks.
			ui.Column(c).Fill().Center().Gap(10).Children(func() {
				ui.Icon(c, glyph.Globe).Size(36, 36).TextColor(pal.TextFaint)
				ui.Text(c, locales.BrowserBlank()).FontSize(13).TextColor(pal.TextMuted)
			})
		case e.View == nil:
			a.PaneMessage(c, pal, locales.BrowserCantShow(), e.Err, "", nil)
		default:
			web := ui.WebView(c.Key("web"), e.View).Grow(1).Bind(ref)
			// The page took the keyboard as it was clicked: the pane is
			// the one the user is in.
			if web.Hovered() && tools.PressedOutside() && a.FocusOf(s) != p.ID {
				a.SetFocus(s, p.ID)
			}
			// The focus came to its element, from the keyboard or the
			// app: the page takes it.
			switch {
			case ref.Focused(c) && !e.Focused:
				e.Focused = true
				e.View.Focus()
				if a.FocusOf(s) != p.ID {
					a.SetFocus(s, p.ID)
				}
			case !ref.Focused(c):
				e.Focused = false
			}
			if e.Err != "" {
				browserError(a, c, pal, e)
			}
			if a.FindPane() == p.ID {
				Find(a, c, pal, e)
			}
		}
	})
	return dragging
}

// browserBar is a browser pane's header: back, forward, reload and the
// address bar, which the pane is dragged by, then the pane's buttons. As
// a terminal's header, it drags the window where it is empty, and keeps
// clear of the traffic lights and the sidebar's buttons.
func browserBar(a App, c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, p *proto.Pane, e *Entry, top, left, right, multi, focused, hovered bool) (ui.Element, bool) {
	pad := float32(16)
	if top && left {
		pad = a.ChromeStateOf().HeaderPad
	}
	// The first button's icon starts where a header's title does.
	h := ui.Row(c.Key("hdr")).Height(a.HeaderHeightIn()).Shrink(0).AlignItems(ui.Center).Padding(0, 10, 0, pad-6).Gap(2)
	if h.Pressed() && !focused {
		a.SetFocus(s, p.ID)
		a.PaneRef(p.ID).Focus()
	}
	dragging := false
	h.Children(func() {
		if top && left {
			panes.SidebarButtons(a.Panes(), c, pal, bar, shown)
		}
		navButton(c, "back", glyph.ChevronLeft, locales.BrowserBack(), e.Back, pal, func() {
			e.View.Page().GoBack()
			if e.Recording {
				a.RecordBrowserNav(p.ID, "back", "")
			}
		})
		navButton(c, "forward", glyph.Chevron, locales.BrowserForward(), e.Fwd, pal, func() {
			e.View.Page().GoForward()
			if e.Recording {
				a.RecordBrowserNav(p.ID, "forward", "")
			}
		})
		if e.Loading {
			navButton(c, "stop", glyph.Close, locales.BrowserStop(), true, pal, func() { e.View.Page().Stop(); e.Loading = false })
			// Loading ends with an event, which a page that never ends
			// loading sends late: look again meanwhile.
			c.After(500 * time.Millisecond)
			e.SyncNav()
		} else {
			navButton(c, "reload", glyph.Reload, locales.BrowserReload(), e.View != nil && e.URL != proto.BlankURL, pal, func() {
				e.View.Page().Reload()
				e.Loading = true
				if e.Recording {
					a.RecordBrowserNav(p.ID, "reload", "")
				}
			})
		}
		gap := ui.Box(c.Key("drag-l")).Width(8).Shrink(0).AlignSelf(ui.Stretch)
		dragging = addressBar(a, c, pal, s, p, e, multi)
		recordButton(a, c, pal, p, e)
		navButton(c, "replay", glyph.Play, locales.BrowserReplay(), !e.Replaying, pal, func() {
			a.OpenBrowserReplay(p.ID)
		})
		// The room the address bar leaves moves the window, and zooms it
		// on a double click.
		rest := ui.Box(c.Key("drag")).Grow(1).MinWidth(8).AlignSelf(ui.Stretch)
		if top {
			gap.DragWindow()
			rest.DragWindow()
		}
		// The room past the address bar takes the buttons as they show.
		panes.PaneControls(a.Panes(), c, pal, s, p, (hovered || focused && multi) && !dragging, multi)
		if top && right {
			a.Panes().WindowCtl(c, pal)
		}
	})
	return h, dragging
}

// navButton is a toolbar's button, faint while it can do nothing.
func navButton(c *ui.Context, key string, svg *ui.SVG, label string, enabled bool, pal *theme.Palette, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Size(28, 26).Radius(6).Center().TextColor(pal.TextMuted).Label(label).Tooltip(label).Disabled(!enabled)
	switch {
	case !enabled:
		b.TextColor(pal.TextFaint.Alpha(0.6))
	case b.Pressed():
		b.Background(pal.RowHover.Alpha(0.12))
	case b.Hovered():
		b.Background(pal.RowHover).TextColor(pal.Text)
	}
	b.Children(func() { ui.Icon(c, svg).Size(15, 15) })
	if enabled && b.Clicked() {
		fn()
	}
}

func recordButton(a App, c *ui.Context, pal *theme.Palette, p *proto.Pane, e *Entry) {
	svg, label := glyph.Record, locales.BrowserRecordStart()
	if e.Recording {
		svg, label = glyph.StopSquare, locales.BrowserRecordStop()
	}
	b := ui.ButtonBase(c.Key("record")).Size(28, 26).Radius(6).Center().Label(label).Tooltip(label)
	switch {
	case e.Recording:
		b.TextColor(pal.Danger)
		if b.Pressed() {
			b.Background(pal.Danger.Alpha(0.18))
		} else if b.Hovered() {
			b.Background(pal.Danger.Alpha(0.12))
		}
	case b.Pressed():
		b.Background(pal.RowHover.Alpha(0.12)).TextColor(pal.Text)
	case b.Hovered():
		b.Background(pal.RowHover).TextColor(pal.Text)
	default:
		b.TextColor(pal.TextMuted)
	}
	b.Children(func() { ui.Icon(c, svg).Size(14, 14) })
	if b.Clicked() {
		a.ToggleBrowserRecord(p.ID)
	}
}

// addressBar is where the page's URL shows, and the user types another, or
// words to search for. With several panes, its globe is the pane's handle,
// as a terminal's title is: it reports whether it is being dragged.
func addressBar(a App, c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane, e *Entry, multi bool) (dragging bool) {
	blank := e.URL == proto.BlankURL
	// It takes the room up to a width that reads well, and leaves the
	// rest to move the window by.
	frame := ui.Row(c.Key("address")).Grow(100).MinWidth(120).MaxWidth(760).Height(28).Padding(0, 10, 0, 4).Gap(3).Radius(8).
		AlignItems(ui.Center).Background(pal.Chip).Cursor(ui.CursorText)
	var in ui.Element
	frame.Children(func() {
		grip := ui.Row(c.Key("grip")).Size(22, 22).Radius(6).Center().Shrink(0).TextColor(pal.TextFaint)
		if multi {
			grip.Drag(panes.PaneDrag{Session: s.ID, Pane: p.ID}).Tooltip(locales.PaneDragTip())
			dragging = grip.Dragging()
			switch {
			case dragging:
				style.Floating(grip.Background(pal.Popover), pal)
			case grip.Hovered():
				grip.Background(pal.RowHover).TextColor(pal.TextMuted).Cursor(ui.CursorGrab)
			}
		}
		grip.Children(func() {
			if icon := webicon.Icon(a, &e.Icon); icon != nil && !blank {
				ui.Image(c, icon).Size(14, 14).Radius(3)
			} else {
				ui.Icon(c, glyph.Globe).Size(13, 13)
			}
		})
		in = ui.TextInputBase(c.Key("in"), &e.Address).Grow(1).MinWidth(0).FontSize(12.5).
			Placeholder(locales.BrowserAddress()).Label(locales.BrowserAddress())
		if blank {
			// With no page, the pane's keyboard is the address bar's.
			in.Bind(a.PaneRef(p.ID))
		}
	})
	focused := in.Focused()
	e.Editing = focused
	glow := frame.Animate("glow", a.B2F(focused), 120*time.Millisecond)
	if glow > 0 {
		frame.Shadow(0, 0, 0, 3*glow, pal.Accent.Alpha(0.3*glow))
	}
	if frame.Pressed() && !focused {
		in.Focus()
	}
	if e.FocusAddress {
		e.FocusAddress = false
		in.Focus().SetTextSelection(0, utf8.RuneCountInString(e.Address))
		if a.FocusOf(s) != p.ID {
			a.SetFocus(s, p.ID)
		}
	}
	if focused && a.FocusOf(s) != p.ID {
		a.SetFocus(s, p.ID)
	}
	if in.Submitted() && e.Address != "" {
		e.Load(e.Address)
		if e.Recording {
			a.RecordBrowserNav(p.ID, "open", e.URL)
		}
		// The page takes the keyboard, through its element, which the
		// field gives the focus up to.
		e.Focused = false
		a.PaneRef(p.ID).Focus()
		TellPage(a, p.ID, e)
	}
	// Escape gives the field back the page's URL, as a browser's does.
	if frame.Shortcut(0, ui.KeyEscape) {
		e.Address = e.URL
		if e.URL == proto.BlankURL {
			e.Address = ""
		}
	}
	if !focused && e.URL != proto.BlankURL {
		e.Address = e.URL
	}
	return dragging
}

// browserError shows why the page failed to load, over its top.
func browserError(a App, c *ui.Context, pal *theme.Palette, e *Entry) {
	bar := ui.Row(c.Key("error")).Absolute().Top(8).Left(12).Right(12).Padding(8, 8, 8, 12).Gap(8).
		AlignItems(ui.Center).Radius(10).Background(pal.Popover)
	style.Floating(bar, pal).Children(func() {
		ui.Text(c, e.Err).FontSize(12.5).TextColor(pal.Danger).Grow(1).MinWidth(0).Ellipsis("…").SingleLine()
		if style.Button(c, "retry", locales.CommonTryAgain(), "", false, pal) {
			e.Load(e.URL)
		}
		paints.IconButton(c, "dismiss", glyph.Close, locales.FindClose(), pal, func() { e.Err = "" })
	})
}

// find is the find bar's field: its query, and the field the bar binds.
type find struct {
	q     string
	input *ui.Handle
}

// Find is the find bar over a browser's page, as over a terminal.
func Find(a App, c *ui.Context, pal *theme.Palette, e *Entry) {
	f := &find{q: a.FindQuery(), input: a.FindInput()}
	if e.FindText != f.q {
		FindInPage(a, e, false, false)
	}
	bar := ui.Row(c.Key("find")).Absolute().Top(8).Right(12).Height(36).Padding(0, 4, 0, 12).Gap(4).
		AlignItems(ui.Center).Radius(10).Background(pal.Popover).Label(locales.FindLabel())
	style.Floating(bar, pal).Children(func() {
		ui.Icon(c, glyph.Search).Size(13, 13).TextColor(pal.TextMuted).Shrink(0)
		in := ui.TextInputBase(c.Key("q"), &f.q).Width(180).FontSize(13).Placeholder(locales.FindPlaceholder()).Bind(f.input)
		// The bar opens with its query selected, which the field takes at
		// its place, out of the build.
		a.RunLater(func() { in.Focus().SetTextSelection(0, utf8.RuneCountInString(f.q)) })
		if in.Submitted() {
			FindInPage(a, e, true, c.Modifiers()&ui.Shift != 0)
		}
		count := ""
		switch {
		case f.q == "":
		case e.Found.Matches == 0:
			count = locales.FindNoMatches()
		default:
			count = locales.FindCount(panes.Itoa(e.Found.Active), panes.Itoa(e.Found.Matches))
		}
		ui.Text(c, count).FontSize(12).FontFeatures("tnum").TextColor(pal.TextFaint).SingleLine().MinWidth(56).TextAlign(ui.End).Margin(0, 4, 0, 0)
		paints.IconButton(c, "previous", glyph.ChevronUp, a.Tip(locales.ActionFindPrevious(), "find.previous"), pal, func() { FindInPage(a, e, true, true) })
		paints.IconButton(c, "next", glyph.ChevronDown, a.Tip(locales.ActionFindNext(), "find.next"), pal, func() { FindInPage(a, e, true, false) })
		paints.IconButton(c, "close", glyph.Close, locales.FindClose(), pal, func() { a.RunLater(a.CloseFindBar) })
	})
	if bar.Shortcut(0, ui.KeyEscape) {
		a.RunLater(a.CloseFindBar)
	}
}

// Shows reports whether the app draws a pane itself, rather than a
// terminal's cells: a browser's page, or a file's text.
func Shows(a App, id string) bool {
	if s := a.SessionOfPane(id); s != nil {
		return s.PaneByID(id).Shown()
	}
	return false
}

// FindInPage searches the page for the find bar's query, off the main
// thread, where the page answers: next goes on from the match shown.
func FindInPage(a App, e *Entry, next, back bool) {
	if e.View == nil {
		return
	}
	q := a.FindQuery()
	e.FindText = q
	e.FindSeq++
	seq, page := e.FindSeq, e.View.Page()
	if q == "" {
		e.Found = mygo.FindResult{}
		go page.StopFindInPage()
		return
	}
	go func() {
		res, err := page.FindInPage(q, mygo.FindOptions{FindNext: next, Backward: back})
		a.Update(func() {
			if err == nil && e.FindSeq == seq {
				e.Found = res
			}
		})
	}()
}

// nextLoad returns what the page's next load sends its end to: nil, or
// why it failed.
// NextLoad returns what the page's next load sends its end to: nil, or
// why it failed.
func (e *Entry) NextLoad() <-chan error {
	ch := make(chan error, 1)
	e.Loads = append(e.Loads, ch)
	return ch
}

// LoadEnded tells those waiting for the page's load that it ended.
func (e *Entry) LoadEnded(err error) {
	for _, ch := range e.Loads {
		ch <- err
	}
	e.Loads = nil
}
