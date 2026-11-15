// Package chat is the Chat view: a session's conversation as a chat —
// the prompts the session got, the tools its agent ran, and the answers
// it gave, which the agent keeps in its transcript, with the Feishu chat
// the session is with, whose turns the chat got. The session's host reads
// it (feishu.feed) as the view opens and as the chat's turns go on.
package chat

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// Refresh is how soon the view reads a chat window again: a chat's turn
// ends with the host's state, as it goes idle.
const Refresh = time.Second

// Show is how many of a conversation's lines the view shows when it is
// long: the end of it.
const Show = 200

// Width is how wide a conversation's column is, its bubbles inside it: as
// a chat app's, a long line reads best at a width.
const Width = 720

// bubble is how wide a bubble's text may be: most of the column, as a
// chat app's.
const bubble = Width * 0.88

// State is the Chat view's, by session.
type State struct {
	Feeds map[string]*Feed // the sessions' chat windows
	// Tools are the tools that show their details, by session and the
	// tool's line.
	Tools map[string]map[string]bool
}

// Feed is a session's chat window, as the host in it read it.
type Feed struct {
	H    *host.Host // the host of the read
	At   time.Time  // when the last read answered
	Busy bool       // a read is under way
	Got  bool       // a read answered: the turns of the chat are as it says
	Err  string     // why the last read failed
	Feed proto.FeishuFeed
	// Pick is the chat the user chose, "" for the session's window, and
	// List the conversation's place, which follows its end.
	Pick string
	List ui.ListState
}

// App is what the view asks of the app.
type App interface {
	// Chat is the view's state, by session.
	Chat() *State
	// Feed is a session's chat window.
	Feed(session string) *Feed
	// HostOf is the host a session runs on.
	HostOf(session string) *host.Host
	// Main runs fn on the main thread.
	Main(fn func())
	// Message is the view's, as it has nothing to show.
	Message(c *ui.Context, pal *theme.Palette, title, detail string)
}

// Changed tells the view that a host shows a new state: the hosts' chat
// windows read their turns again, as a chat's turn ends.
func Changed(x App, h *host.Host) {
	for id, f := range x.Chat().Feeds {
		if x.HostOf(id) == h {
			f.Got = false
		}
	}
}

// Body is the view: the conversation, in a column of a comfortable width.
// It shows the chat's turns, which Feishu sends: a prompt is not written
// here.
func Body(x App, c *ui.Context, pal *theme.Palette, s *proto.Session) {
	f := x.Feed(s.ID)
	Read(x, s, f)
	ui.Column(c.Key("lines")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		lines(x, c, pal, s, f)
	})
}

// read reads a session's chat window, as the view opens and after a
// chat's turn, and as the host's state changes.
func Read(x App, s *proto.Session, f *Feed) {
	h := x.HostOf(s.ID)
	if f.Busy || f.Got && len(f.Feed.Lines) > 0 && time.Since(f.At) < Refresh {
		return
	}
	if h.Client == nil {
		// A host that connects reads it as it does.
		return
	}
	f.Busy, f.Got, f.H = true, true, h
	cl := h.Client
	params := proto.FeishuFeedParams{Session: s.ID, Chat: f.Pick}
	// It reads from a goroutine of its own: a call from the frame's would
	// wait on the frame.
	go func() {
		var out proto.FeishuFeed
		err := cl.Call("feishu.feed", params, &out)
		x.Main(func() {
			if x.Feed(s.ID) != f {
				return
			}
			f.Busy, f.At = false, time.Now()
			if err != nil {
				f.Err = err.Error()
				return
			}
			f.Err, f.Feed = "", out
		})
	}()
}

// lines shows the conversation: what the user wrote to the right, the
// agent's messages to the left, its tools faint, and each turn's answer
// marked.
func lines(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, f *Feed) {
	ls := f.Feed.Lines
	if len(ls) > Show {
		ls = ls[len(ls)-Show:]
	}
	if len(ls) == 0 {
		msg := locales.ChatNothing()
		switch {
		case f.Busy && !f.read():
			msg = locales.ChatReadingNow()
		case f.Err != "":
			msg = f.Err
		case f.Feed.Chat == "":
			msg = locales.ChatSessionNone()
		}
		x.Message(c, pal, locales.SideChat(), msg)
		return
	}
	f.List.FollowEnd, f.List.FollowAnimation = true, &ui.Smooth
	ui.List(c.Key("chat"), &f.List, len(ls), func(i int) {
		line(x, c, pal, f, s.ID, i, ls[i])
	}).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Padding(18, 16, 12, 8).Gap(10)
}

// read tells that a read of the window answered before.
func (f *Feed) read() bool { return f.Feed.ID != "" }

// line is a line of the conversation.
func line(x App, c *ui.Context, pal *theme.Palette, f *Feed, session string, i int, l proto.FeishuLine) {
	text := strings.TrimSpace(l.Text)
	if text == "" && l.Detail == "" && l.Title == "" {
		return
	}
	// The column's: a line of the conversation, in the middle of the
	// pane at a fixed width, as wide as it, its own text inside a
	// bubble, the agent's or the user's.
	ui.Column(c).AlignSelf(ui.Center).WidthPercent(100).MaxWidth(Width).AlignItems(ui.Stretch).Children(func() {
		switch l.Kind {
		case "tool":
			tool(x, c, pal, session, i, l)
			return
		case "think":
			ui.Column(c).MaxWidth(Width*0.9).Padding(6, 2).Gap(2).Children(func() {
				ui.Text(c, locales.ChatThought()).FontSize(11).FontWeight(600).TextColor(pal.TextFaint)
				markdownView(c, text, pal.TextMuted, pal.TextMuted.Alpha(0.08), pal)
			})
			return
		case "note", "user":
			// The user's prompt: a Feishu message's, or one typed in
			// the terminal, its bubble as wide as the column, which a
			// long prompt takes.
			ui.Row(c).AlignSelf(ui.Stretch).Justify(ui.End).Children(func() {
				ui.Box(c).Grow(1)
				ui.Column(c).Padding(8, 13).Radius(14).Background(pal.Accent).Children(func() {
					markdownView(c, text, onAccent, ui.RGBA(255, 255, 255, 0.16), pal)
				})
			})
			return
		}
		// The agent's: its text on a soft bubble, the turn's answer a
		// shade stronger.
		bg := pal.Text.Alpha(0.05)
		if l.Answer {
			bg = pal.Text.Alpha(0.085)
		}
		ui.Row(c).AlignSelf(ui.Stretch).MaxWidth(bubble).Children(func() {
			ui.Column(c).Padding(8, 13).Radius(14).Background(bg).Children(func() {
				markdownView(c, text, pal.Text, pal.Text.Alpha(0.07), pal)
			})
		})
	})
}

// themes are the Markdown themes of the chat windows, by day and by
// night: a message's colors, which differ, its own.
var themes = map[string]markdown.Theme{}

// onAccent is the color of the text on the user's prompts.
var onAccent = ui.Hex("#ffffff")

// markdownView shows a message's text as Markdown, as a chat app does:
// headings, lists, quotes, tables, code blocks, and links, which open in
// the browser. fg is the color of its text and code the fill of code
// spans, which differ with the message's surface.
func markdownView(c *ui.Context, text string, fg, code ui.Color, pal *theme.Palette) {
	key := "light"
	if pal.Dark {
		key = "dark"
	}
	th, ok := themes[key]
	if !ok || th.Text != fg || th.CodeBackground != code {
		th = markdown.Theme{
			FontSize:            13,
			LineHeight:          1.5,
			Text:                fg,
			Muted:               fg.Alpha(0.68),
			Strong:              fg,
			Code:                fg,
			CodeBackground:      code,
			BlockSpacing:        0.55,
			ItemSpacing:         0.2,
			QuoteBar:            code.Mix(fg, 0.3),
			QuoteText:           fg.Alpha(0.75),
			Rule:                code,
			CodeBlockBackground: code,
			CodeBlockBorder:     code,
			TableBorder:         code,
			TableHeader:         code,
		}
		themes[key] = th
	}
	markdown.View(c, text, &markdown.Options{
		Theme:    &th,
		WrapCode: true,
		OnLink:   func(url string) { c.OpenURL(url) },
	})
}

// tool is a tool's line: its icon and its name, and what it ran, with
// the output under it, which a click folds or opens.
func tool(x App, c *ui.Context, pal *theme.Palette, session string, at int, l proto.FeishuLine) {
	g, name := glyph.Gear, strings.TrimPrefix(l.Text, "Tool ")
	if l.Text == "Result" {
		g, name = glyph.Return, locales.ChatOutput()
	}
	// A tool's name is its line, and what it ran, after it: a line
	// without a name, as an older host's, says no more than it holds.
	arg := l.Title
	if name == l.Text {
		arg = ""
	}
	if arg == "" {
		arg = strings.TrimSpace(l.Detail)
	}
	ui.Column(c).AlignSelf(ui.Stretch).Children(func() {
		row := ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, g).Size(11, 11).TextColor(pal.TextFaint).Shrink(0)
			ui.Text(c, name).FontSize(11.5).FontWeight(500).TextColor(pal.TextMuted).SingleLine().Shrink(0)
			if arg != "" {
				ui.Text(c, ShortText(arg, 90)).Font("monospace").FontSize(11).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			}
		})
		// A tool's arguments open under its line, and its output, which
		// the click takes from the line.
		open := openTool(x, session, at, row.Clicked())
		if row.Clicked() {
			c.Invalidate()
		}
		var detail string
		switch {
		case l.Text == "Result":
			if l.Detail != "" {
				detail = l.Detail
			}
		case l.Title != "":
			detail = l.Detail
		}
		if detail == "" || !open {
			return
		}
		ui.Scroll(c.Key("detail")).MaxHeight(240).Padding(8, 10).Margin(4, 20, 0, 18).Radius(8).Background(pal.Text.Alpha(0.04)).Children(func() {
			ui.Text(c, ShortText(detail, 4000)).Font("monospace").FontSize(11.5).TextColor(pal.TextMuted).LineHeight(1.35).Wrap().Selectable()
		})
	})
}

// openTool tells that a conversation's tool, at a line, shows its
// details: a click on its line folds or opens them.
func openTool(x App, session string, at int, open bool) bool {
	t := x.Chat().Tools
	if t == nil {
		t = map[string]map[string]bool{}
		x.Chat().Tools = t
	}
	keys := t[session]
	if keys == nil {
		keys = map[string]bool{}
		t[session] = keys
	}
	key := strconv.Itoa(at)
	if open {
		keys[key] = !keys[key]
	}
	return keys[key]
}

// ShortText is s in one line, cut at n characters.
func ShortText(s string, n int) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
