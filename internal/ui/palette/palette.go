// Package palette is the command palette: a field that finds sessions,
// conversations and commands as one types.
package palette

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/fuzzy"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/settings/search"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Page is a page of the Settings window that the palette offers.
type Page struct {
	ID    string
	Title string
}

// Command is a command the palette offers that is not an action's: what
// runs it, and how the app writes its title.
type Command struct {
	Title  string
	Detail string
	Keys   string
	Words  string
	Top    bool // what the query asks for outright, before any match
	Run    func()
}

// App is what the palette finds and runs, as the app gives it.
type App interface {
	// Settings is the settings the view reads.
	Settings() *settings.Settings
	// Projects are the projects the app knows.
	Projects() []proto.Project
	// Groups are the app's sessions, as the sidebar shows them.
	Groups() [][]*proto.Session
	// HostOf is the host a session or a pane is of.
	HostOf(id string) *host.Host
	// Local is this machine's host.
	Local() *host.Host
	// Shown is the session shown.
	Shown() string
	// Session is a session the app knows, nil while none.
	Session(id string) *proto.Session
	// Collapsed is the projects whose sessions the sidebar hides, and sets
	// one so.
	Collapsed() map[string]bool
	// Update runs fn on the app's own goroutine.
	Update(fn func())
	// Toast shows a message.
	Toast(msg string)
	// AllHosts are the app's hosts.
	AllHosts() []*host.Host
	// Pages are the Settings' pages.
	Pages() []Page
	// ActionTitle is how the app writes an action's title.
	ActionTitle(id string) string
	// Run runs an action.
	Run(id string)
	// OpenSettings shows a page of the Settings window.
	OpenSettings(id string)
	// NewSession starts a session in a project.
	NewSession(projectID string)
	// AddPending adds a session a host's hylad started, before the app
	// hears of it another way.
	AddPending(h *host.Host, s proto.Session)
	// SelectSession shows a session.
	SelectSession(id string)
	// ResumeConversation shows the session running a conversation of a
	// host's, or starts one resuming it.
	ResumeConversation(h *host.Host, cv proto.Conversation)
	// RestartDaemon restarts hylad into this version of Hyla.
	RestartDaemon()
	// FocusTerminal focuses the terminal the user works in.
	FocusTerminal()
}

// State is the palette's state.
type State struct {
	Open   bool
	Query  string
	Index  int
	Scroll ui.ScrollState
	Reveal bool // the chosen item scrolls into view, once
	// convs are the conversations the hosts' hylads remember, the last to
	// run first, as they come; only shows them alone (Resume
	// Conversation…), and gen tells the palette they came for.
	convs []hostConversation
	only  bool
	gen   int
}

// hostConversation is a conversation a host's hylad remembers.
type hostConversation struct {
	host *host.Host
	proto.Conversation
}

// recentConversations is how many conversations the palette shows before
// the user types.
const recentConversations = 5

// item is a session, a conversation or a command the palette offers.
type item struct {
	session *proto.Session
	conv    *hostConversation
	title   string
	detail  string
	keys    string
	run     func()
	score   int
	words   string // what finds it besides its title and detail, unshown
	top     bool   // what the query asks for outright, before any match
}

// Open opens the palette, and asks the hosts for their conversations.
func Open(st *State, app App) {
	*st = State{Open: true, Reveal: true, gen: st.gen + 1}
	loadConversations(st, app)
}

// OpenConversations opens the palette on the conversations alone.
func OpenConversations(st *State, app App) {
	Open(st, app)
	st.only = true
}

// loadConversations asks every host's hylad for the conversations it
// remembers. An older one knows of none.
func loadConversations(st *State, app App) {
	gen := st.gen
	for _, h := range app.AllHosts() {
		cl := h.Client
		if cl == nil {
			continue
		}
		go func() {
			var list []proto.Conversation
			if cl.Call("conversations", nil, &list) != nil {
				return
			}
			app.Update(func() {
				if st.gen != gen || !st.Open {
					return
				}
				for _, cv := range list {
					st.convs = append(st.convs, hostConversation{h, cv})
				}
				slices.SortStableFunc(st.convs, func(x, y hostConversation) int { return cmp.Compare(y.Updated, x.Updated) })
			})
		}()
	}
}

// conversationItems are the palette's conversations: those no pane runs,
// as a session the palette has already shows them; all of them with only.
func conversationItems(st *State, app App) []item {
	var items []item
	names := map[string]string{}
	for _, p := range app.Projects() {
		names[p.ID] = p.Name
	}
	now := time.Now()
	for i := range st.convs {
		cv := &st.convs[i]
		if cv.Session != "" && !st.only {
			continue
		}
		detail := cmp.Or(proto.AgentNames[cv.Agent], cv.Agent)
		if cv.Name != "" && cv.Title != "" {
			detail = cv.Title + " · " + detail
		}
		if n := names[cv.Project]; n != "" {
			detail += " · " + n
		}
		if cv.Session != "" {
			detail += " · " + locales.PaletteConversationRunning()
		} else {
			detail += " · " + proto.Ago(cv.Updated, now)
		}
		h, conv := cv.host, cv.Conversation
		items = append(items, item{conv: &st.convs[i], title: conv.Label(), detail: detail,
			words: conv.Title + " " + conv.Name + " " + conv.Dir + " " + conv.Agent + " " + conv.ID,
			run:   func() { app.ResumeConversation(h, conv) }})
	}
	return items
}

// Items are what the palette offers: sessions, conversations and commands.
func Items(st *State, app App) []item {
	if st.only {
		return conversationItems(st, app)
	}
	var items []item
	names := map[string]string{}
	for _, p := range app.Projects() {
		names[p.ID] = p.Name
	}
	for _, g := range app.Groups() {
		for _, s := range g {
			id := s.ID
			detail := names[s.Project]
			if s.Process != "" {
				detail += " · " + s.Process
			}
			h := app.HostOf(id)
			items = append(items, item{session: s, title: s.DisplayName(app.Settings().SessionName), detail: detail + " · " + h.ShortDir(s.WorkDir()),
				run: func() { delete(app.Collapsed(), s.Project); app.SelectSession(id) }})
		}
	}
	var cmds []item
	// Commands are found by their English names too, in any language.
	en := locales.In("en")
	// An address typed opens in a browser beside the pane the user is in.
	if q := strings.TrimSpace(st.Query); proto.IsAddress(q) && app.Session(app.Shown()) != nil {
		cmds = append(cmds, item{title: locales.PaletteOpenInBrowser(), detail: proto.BrowserURL(q), top: true,
			run: func() { app.Run("pane.browser") }})
	}
	for _, act := range actions.All {
		id := act.ID
		// A global shortcut shows too, marked, though the menus leave it to
		// the system to take.
		keys := actions.Glyphs(actions.Keys(act, app.Settings().Shortcuts))
		if keys != "" && act.Global {
			keys += " " + locales.PaletteShortcutGlobal()
		}
		cmds = append(cmds, item{title: strings.TrimSuffix(app.ActionTitle(act.ID), "…"), keys: keys,
			words: act.Title + " " + search.ActionWords[id], run: func() { app.Run(id) }})
	}
	for _, page := range app.Pages() {
		id := page.ID
		cmds = append(cmds, item{title: locales.PaletteSettingsPage(func() string { return page.Title }()), words: "settings preferences " + id + " " + search.PageWords[id], run: func() { app.OpenSettings(id) }})
	}
	cmds = append(cmds, item{title: locales.PaletteRestartService(), detail: locales.PaletteRestartServiceDetail(),
		words: en.PaletteRestartService() + " " + en.PaletteRestartServiceDetail(), run: app.RestartDaemon})
	for _, p := range app.Projects() {
		id := p.ID
		detail := app.Local().ShortDir(p.Dir)
		cmds = append(cmds, item{title: locales.NewSessionIn(p.Name), detail: detail, words: en.NewSessionIn(p.Name), run: func() { app.NewSession(id) }})
	}
	convs := conversationItems(st, app)
	if st.Query == "" && len(convs) > recentConversations {
		convs = convs[:recentConversations]
	}
	items = append(items, convs...)
	return append(items, cmds...)
}

// What an item is, which its group in the list goes by.
const (
	kindSession = iota
	kindConversation
	kindCommand
)

func (it *item) kind() int {
	switch {
	case it.session != nil:
		return kindSession
	case it.conv != nil:
		return kindConversation
	}
	return kindCommand
}

// Title is what the palette finds and shows of its items, in order.
func Titles(st *State, app App) []string {
	var titles []string
	for _, it := range Items(st, app) {
		titles = append(titles, it.title)
	}
	return titles
}

// View draws the palette, and runs what the user chose.
func View(c *ui.Context, st *State, app App, pal *theme.Palette) {
	if !st.Open {
		return
	}
	var items []item
	for _, it := range Items(st, app) {
		sc := fuzzy.Score(st.Query, it.title+" "+it.detail)
		sc = max(sc, fuzzy.Words(st.Query, it.words))
		if t := fuzzy.Score(st.Query, it.title); t >= 0 {
			sc = max(sc, t+50)
		}
		if it.top {
			sc = math.MaxInt32
		}
		if sc < 0 {
			continue
		}
		it.score = sc
		items = append(items, it)
	}
	if st.Query != "" {
		slices.SortStableFunc(items, func(x, y item) int { return y.score - x.score })
	}
	st.Index = min(max(st.Index, 0), max(len(items)-1, 0))

	var chosen func()
	label, placeholder, none := locales.PaletteLabel(), locales.PalettePlaceholder(), locales.PaletteNoMatches()
	if st.only {
		label, placeholder = locales.PaletteConversationsLabel(), locales.PaletteConversationsPlaceholder()
		if st.Query == "" {
			none = locales.PaletteNoConversations()
		}
	}
	ui.DialogBase(c.Key("palette"), &st.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay).Justify(ui.Start).Padding(84, 0, 0, 0)
		style.Floating(panel.Width(600).MaxHeight(460).Radius(14).Background(pal.Popover), pal).Clip().
			Label(label)
		ui.Row(c).Height(50).Shrink(0).Padding(0, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, glyph.Search).Size(16, 16).TextColor(pal.TextMuted)
			in := ui.TextInputBase(c.Key("q"), &st.Query).Grow(1).FontSize(15).Placeholder(placeholder).AutoFocus()
			if in.Changed() {
				st.Index, st.Reveal = 0, true
			}
			if c.Shortcut(0, ui.KeyDown) || c.Shortcut(ui.Ctrl, ui.KeyN) {
				st.Index, st.Reveal = min(st.Index+1, len(items)-1), true
			}
			if c.Shortcut(0, ui.KeyUp) || c.Shortcut(ui.Ctrl, ui.KeyP) {
				st.Index, st.Reveal = max(st.Index-1, 0), true
			}
			if (in.Submitted() || c.Shortcut(0, ui.KeyEnter)) && len(items) > 0 {
				chosen = items[st.Index].run
			}
		})
		list := ui.Scroll(c.Key("results")).Grow(1).MinHeight(0).Padding(6).TrackScroll(&st.Scroll)
		reveal := st.Reveal
		st.Reveal = false
		list.Children(func() {
			if len(items) == 0 {
				ui.Text(c, none).FontSize(13).TextColor(pal.TextFaint).Padding(12, 12).LineHeight(1.4)
			}
			lastKind := kindSession
			for i, it := range items {
				kind := it.kind()
				// Conversations head their group even first, as with no
				// sessions, or they would read as sessions.
				if (i > 0 || kind == kindConversation) && kind != lastKind && st.Query == "" && !st.only {
					title := locales.PaletteCommands()
					if kind == kindConversation {
						title = locales.PaletteConversations()
					}
					ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(pal.TextFaint).Padding(10, 10, 4, 10)
				}
				lastKind = kind
				row := ui.Row(c.Key(i)).Height(38).Padding(0, 10).Gap(10).AlignItems(ui.Center).Radius(8)
				if i == st.Index {
					row.Background(pal.Accent.Alpha(0.12))
					if reveal {
						row.ScrollIntoView()
					}
				} else if row.Hovered() {
					row.Background(pal.RowHover)
				}
				if row.Clicked() {
					chosen = it.run
				}
				row.Children(func() {
					switch kind {
					case kindSession:
						paints.Status(c, pal, it.session.Status, it.session.Unread, 14)
					case kindConversation:
						ui.Icon(c, glyph.ProgramIcon(&proto.Pane{Agent: it.conv.Agent})).Size(14, 14).TextColor(pal.TextFaint)
					default:
						ui.Icon(c, glyph.Return).Size(14, 14).TextColor(pal.TextFaint)
					}
					ui.Text(c, it.title).FontSize(13.5).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(0).MaxWidth(300)
					ui.Text(c, it.detail).FontSize(12.5).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
					if it.keys != "" {
						paints.Kbd(c, pal, it.keys)
					}
				})
			}
		})
	})
	if chosen != nil {
		st.Open = false
		chosen()
	}
	if !st.Open && chosen == nil {
		app.FocusTerminal()
	}
}
