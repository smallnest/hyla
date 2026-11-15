// Package history is the sidebar's conversations: the ended and the
// archived ones that hylad remembers, which a click resumes.
package history

import (
	"cmp"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/platform"
	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

const endedShown = 5

// Confirm is a question the app asks before it does something that can
// not be undone.
type Confirm struct {
	Title, Detail, Action string
	Destroy               bool
	Run                   func()
}

// App is what the history asks and runs, as the app gives it.
type App interface {
	// Update runs fn on the app's own goroutine.
	Update(fn func())
	// Toast shows a message.
	Toast(msg string)
	// CallOn calls a method of a host's hylad.
	CallOn(h *host.Host, method string, params any, done func(err error))
	// ResumeConversation shows the session running a conversation of a
	// host's, or starts one resuming it.
	ResumeConversation(h *host.Host, cv proto.Conversation)
	// Ask asks a question, which runs what it says once answered.
	Ask(q Confirm)
	// OpenSettings shows a page of the Settings window.
	OpenSettings(id string)
	// AllHosts are the app's hosts.
	AllHosts() []*host.Host
	// Copied is the text the window copies next, and sets.
	Copied() *string
	// SavePrefs saves what the window remembers.
	SavePrefs()
	// SectionPrefs is what the sidebar's sections show, and sets.
	SectionPrefs() *Prefs
	// ShowAll is whether the Ended section shows all, and sets.
	ShowAll() *bool
	// Skill is whether the user has Hyla's skill, as of when it was
	// asked, and sets them.
	Skill() *Skill
	// Skills are each kind of agent's Hyla skill, as the app knows them.
	Skills() []SkillState
}

// Prefs are what the sidebar's sections remember.
type Prefs struct {
	EndedOpen    *bool
	ArchivedOpen *bool
}

// SkillState is how a kind of agent stands with Hyla's skill.
type SkillState struct {
	State string // "installed", "outdated" or "missing"
}

// Skill is whether no kind of agent has Hyla's skill, as of At.
type Skill struct {
	Missing bool
	At      time.Time
}

// Ended is a conversation of a host, for the sidebar.
type Ended struct {
	H *host.Host
	C proto.Conversation
}

// loadHostConversations asks a host's hylad for the conversations it
// remembers, for the sidebar, as of the host's count of the changes to
// them gen: a later answer, or a turn of Session history off, drops it.
func LoadHostConversations(x App, h *host.Host, gen uint64) {
	h.ConvAsked = true
	cl := h.Client
	if cl == nil {
		h.ConvAsked = false
		return
	}
	go func() {
		var list []proto.Conversation
		err := cl.Call("conversations", nil, &list)
		x.Update(func() {
			if h.ConvGen != gen || !h.ConvAsked {
				return
			}
			h.ConvAsked = false
			if err != nil {
				return
			}
			h.Convs, h.ConvKnown = list, true
		})
	}()
}

// syncEnded keeps the sidebar's ended conversations asking for those of
// every host, while Session history is on.
func Sync(x App) {
	for _, h := range x.AllHosts() {
		if h.Client == nil || h.ConvAsked || h.ConvKnown {
			continue
		}
		LoadHostConversations(x, h, h.ConvGen)
	}
}

func Sections(c *ui.Context, x App, pal *theme.Palette) {
	var ended, archived []Ended
	for _, h := range x.AllHosts() {
		for _, cv := range h.Convs {
			// A conversation a pane runs is in the sidebar's sessions.
			if cv.Session != "" {
				continue
			}
			if cv.Archived {
				archived = append(archived, Ended{h, cv})
			} else {
				ended = append(ended, Ended{h, cv})
			}
		}
	}
	newest := func(x, y Ended) int { return cmp.Compare(y.C.Updated, x.C.Updated) }
	slices.SortStableFunc(ended, newest)
	slices.SortStableFunc(archived, newest)
	if len(ended)+len(archived) == 0 {
		skillHint(c, x, pal)
		return
	}
	section(c, x, pal, "ended", locales.EndedTitle(), ended, x.SectionPrefs().EndedOpen)
	section(c, x, pal, "archived", locales.EndedArchived(), archived, x.SectionPrefs().ArchivedOpen)
}

func section(c *ui.Context, x App, pal *theme.Palette, key, title string, list []Ended, open *bool) {
	if len(list) == 0 {
		return
	}
	inset := float32(rowInset)
	if pal.Dark {
		inset++
	}
	hdr := ui.Row(c.Key("section-"+key)).Height(28).Margin(18, 0, 3, 0).Padding(0, 6, 0, inset).Gap(6).AlignItems(ui.Center).Radius(6)
	if hdr.Clicked() {
		*open = !*open
		x.SavePrefs()
	}
	hdr.ContextMenu(func(m *ui.Menu) { sectionMenu(x, m, key, list) })
	hdr.Children(func() {
		ui.Text(c, title).FontSize(12.5).FontWeight(600).TextColor(pal.TextMuted).SingleLine()
		ui.Textf(c, "%d", len(list)).FontSize(12.5).TextColor(pal.TextFaint).FontFeatures("tnum")
		chev := ui.Icon(c, glyph.Chevron).Size(11, 11).TextColor(pal.TextFaint)
		chev.Rotate(chev.AnimateTo("turn", 90*b2f(*open), ui.Snappy))
	})
	if !*open {
		return
	}
	shown := list
	if key == "ended" && !*x.ShowAll() && len(list) > endedShown {
		shown = list[:endedShown]
	}
	now := c.Now()
	for _, he := range shown {
		row(c, x, pal, he.H, he.C, now)
	}
	if len(shown) < len(list) {
		more := ui.Row(c.Key("show-all")).Height(26).Padding(0, 6, 0, inset).AlignItems(ui.Center).Radius(6)
		if more.Hovered() {
			more.Background(pal.RowHover)
		}
		if more.Clicked() {
			*x.ShowAll() = true
		}
		more.Children(func() {
			ui.Text(c, locales.EndedShowAll(len(list))).FontSize(12.5).TextColor(pal.TextFaint).SingleLine()
		})
	}
}

func row(c *ui.Context, x App, pal *theme.Palette, h *host.Host, cv proto.Conversation, now time.Time) {
	key := "e-" + h.Name + "-" + cv.Agent + "-" + cv.ID
	row := ui.Row(c.Key(key)).Height(44).Padding(0, 6, 0, rowInset).Gap(10).Radius(8).AlignItems(ui.Center)
	paints.PointerTooltip(c, row, "tip-"+key, h.ShortDir(cv.Dir))
	hovered := row.Hovered()
	if hovered {
		row.Background(pal.RowHover)
	}
	if row.Clicked() {
		x.ResumeConversation(h, cv)
	}
	row.ContextMenu(func(m *ui.Menu) { menu(x, m, h, cv) })
	row.Children(func() {
		i := glyph.ProgramIcons[cv.Agent]
		if i == nil {
			i = glyph.Terminal
		}
		ui.Icon(c, i).Size(16, 16).TextColor(pal.TextMuted).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, cv.Label()).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
			})
			ui.Text(c, proto.DirName(cv.Dir)).FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
		})
		if hovered && !cv.Archived {
			paints.IconButton(c, "archive", glyph.Archive, locales.EndedArchive(), pal, func() {
				conversationCall(x, h, "conversation.archive", cv, nil)
			})
		} else {
			ui.Text(c, paints.Ago(cv.Updated, now)).FontSize(12).TextColor(pal.TextFaint).SingleLine().Shrink(0)
		}
	})
}

// conversationCall asks a host to forget or archive a conversation, as
// the sidebar's menus do.
func conversationCall(x App, h *host.Host, method string, cv proto.Conversation, run func()) {
	x.CallOn(h, method, proto.ConversationParams{Agent: cv.Agent, ID: cv.ID, Archived: method == "conversation.archive"}, func(err error) {
		if err != nil {
			x.Toast(err.Error())
			return
		}
		if run != nil {
			run()
		}
	})
}

func menu(x App, m *ui.Menu, h *host.Host, cv proto.Conversation) {
	if m.Item(locales.EndedResume()).Chosen() {
		x.ResumeConversation(h, cv)
	}
	if cv.Archived {
		if m.Item(locales.EndedRestore()).Chosen() {
			conversationCall(x, h, "conversation.unarchive", cv, nil)
		}
	} else if m.Item(locales.EndedArchive()).Chosen() {
		conversationCall(x, h, "conversation.archive", cv, nil)
	}
	m.Separator()
	if m.Item(locales.SessionCopyId()).Chosen() {
		*x.Copied() = cv.ID
	}
	if m.Item(locales.SessionCopyDirectory()).Disabled(cv.Dir == "").Chosen() {
		*x.Copied() = cv.Dir
	}
	if m.Item(locales.SessionRevealIn(platform.FileManager())).Disabled(cv.Dir == "").Chosen() {
		platform.OpenPath(cv.Dir)
	}
	if !cv.Archived {
		m.Separator()
		if m.Item(locales.EndedDelete()).Chosen() {
			conversationCall(x, h, "conversation.forget", cv, nil)
		}
	}
}

func sectionMenu(x App, m *ui.Menu, key string, list []Ended) {
	each := func(method string) {
		for _, he := range list {
			conversationCall(x, he.H, method, he.C, nil)
		}
	}
	if key == "ended" {
		if m.Item(locales.EndedArchiveAll()).Chosen() {
			each("conversation.archive")
		}
		return
	}
	if m.Item(locales.EndedDeleteAll()).Chosen() {
		x.Ask(
			Confirm{Title: locales.EndedDeleteAllTitle(len(list)), Detail: locales.EndedDeleteAllDetail(),
				Action: locales.EndedDelete(), Destroy: true, Run: func() { each("conversation.forget") }})
	}
}

// skillHint tells, while nothing has ended, that ended sessions need
// Hyla's skill, which tells hylad each conversation's id.
func skillHint(c *ui.Context, x App, pal *theme.Palette) {
	if now := c.Now(); now.Sub(x.Skill().At) > 5*time.Second {
		x.Skill().At = now
		x.Skill().Missing = slices.ContainsFunc(x.Skills(), func(st SkillState) bool { return st.State == "missing" })
	}
	if !x.Skill().Missing {
		return
	}
	hint := ui.Row(c.Key("ended-hint")).Margin(18, 0, 3, 0).Padding(6, 6, 6, rowInset).Radius(6)
	if hint.Hovered() {
		hint.Background(pal.RowHover)
	}
	if hint.Clicked() {
		x.OpenSettings("agents")
	}
	hint.Children(func() {
		ui.Text(c, locales.EndedSetUpSkill()).FontSize(12).TextColor(pal.TextFaint).Grow(1).MinWidth(0)
	})
}

// rowInset is the sidebar's rows' inset from its edges.
const rowInset = 12

// b2f is b as 0, or 1 for true.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// pointerTooltip shows s as el's tooltip, where macOS shows its tooltips.
func pointerTooltip(c *ui.Context, el ui.Element, key, s string) {
	paints.PointerTooltip(c, el, key, s)
}
