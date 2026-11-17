// Package chat is the Chat view: a session's conversation with its agent as
// a chat — what the user asked, what the agent wrote and reasoned, and the
// tools it ran — from the records the agent keeps beside its conversation
// (chat.get). A live turn shows the step it runs now, with its length, and
// a way to stop it.
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
	"hyla/internal/ui/paints"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// Refresh is how soon the view reads a conversation again while a step of
// it runs: a pane that works tells a new one at once.
const Refresh = time.Second

// Show is how many of a conversation's lines the view shows when it is
// long: the end of it.
const Show = 400

// Width is how wide a conversation's column is, its bubbles inside it: as
// a chat app's, a long line reads best at a width.
const Width = 720

// bubble is how wide a bubble's text may be: most of the column, as a
// chat app's.
const bubble = Width * 0.88

// State is the Chat view's, by session.
type State struct {
	Feeds map[string]*Feed // the sessions' conversations
	// Tools are the tools that show their arguments and output, by
	// session and the tool's line, and Runs the runs of tools that show
	// the tools they hold.
	Tools map[string]map[string]bool
	Runs  map[string]map[string]bool
}

// Feed is a session's conversation, as the host in it read it.
type Feed struct {
	H    *host.Host // the host of the read
	At   time.Time  // when the last read answered
	Busy bool       // a read is under way
	Got  bool       // a read answered
	Err  string     // why the last read failed
	Chat proto.Chat
	// Draft is the prompt being written, which Enter sends.
	Draft string
	// List is the conversation's place, which follows its end.
	List ui.ListState
}

// App is what the view asks of the app.
type App interface {
	// Chat is the view's state, by session.
	Chat() *State
	// Feed is a session's conversation.
	Feed(session string) *Feed
	// HostOf is the host a session runs on.
	HostOf(session string) *host.Host
	// Main runs fn on the main thread.
	Main(fn func())
	// StopPane interrupts the program in a pane, as its terminal's
	// interrupt key does, and SendPane types a prompt into it.
	StopPane(pane string)
	SendPane(pane, text string)
}

// Changed tells the view that a host shows a new state: its conversations
// read their lines again, as a turn ends.
func Changed(x App, h *host.Host) {
	for id, f := range x.Chat().Feeds {
		if x.HostOf(id) == h {
			f.Got = false
		}
	}
}

// Body is the view: the conversation, in a column of a comfortable width.
func Body(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, pane string) {
	f := x.Feed(s.ID)
	Read(x, s, pane, f)
	ui.Column(c.Key("lines")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		lines(x, c, pal, s, f, pane)
	})
	switch {
	case pane == "" || f.Chat.Note != "":
	case f.Chat.Input || f.Chat.Turn:
		composer(x, c, pal, f, pane)
	default:
		// A pane that takes no prompt, as one whose chat has not run a
		// turn yet, says so.
		ui.Text(c, locales.ChatNoInput()).FontSize(11.5).TextColor(pal.TextFaint).
			TextAlign(ui.Center).Padding(0, 16, 14, 16)
	}
}

// composer is the chat's input, at the foot of the column: a prompt to the
// pane's agent, which Enter sends, and the stop of a turn that runs.
func composer(x App, c *ui.Context, pal *theme.Palette, f *Feed, pane string) {
	send := func() {
		text := strings.TrimSpace(f.Draft)
		if text == "" {
			return
		}
		f.Draft = ""
		x.SendPane(pane, text)
	}
	ui.Column(c).Padding(0, 16, 12, 16).Children(func() {
		box := ui.Row(c.Key("composer")).AlignSelf(ui.Center).WidthPercent(100).MaxWidth(Width).MinHeight(44).Radius(14).
			Background(pal.Text.Alpha(0.05)).Padding(0, 6, 0, 12).Gap(6).AlignItems(ui.Center)
		box.Children(func() {
			field := ui.TextInputBase(c, &f.Draft).Grow(1).MinWidth(0).FontSize(13).
				Placeholder(locales.ChatPlaceholder()).Label(locales.ChatSend())
			if field.Submitted() {
				send()
			}
			if m := model(f.Chat.Model); m != "" {
				ui.Text(c, m).FontSize(11).TextColor(pal.TextFaint).SingleLine().Shrink(0)
			}
			if running(f) {
				paints.IconButton(c, "chat-stop", glyph.Close, locales.ChatStop(), pal, func() { x.StopPane(pane) })
				return
			}
			paints.IconButton(c, "chat-send", glyph.Return, locales.ChatSend(), pal, send)
		})
	})
}

// model is a model's name as the composer shows it: "claude-opus-5" as
// "Claude Opus 5", with the date a snapshot's name ends in left out.
func model(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "-")
	words := parts[:0]
	for _, p := range parts {
		// A snapshot's date, as 20251101: the model is the same one.
		if len(p) == 8 && strings.Trim(p, "0123456789") == "" {
			continue
		}
		if p != "" {
			words = append(words, strings.ToUpper(p[:1])+p[1:])
		}
	}
	return strings.Join(words, " ")
}

// running tells whether a step of the conversation runs now.
func running(f *Feed) bool {
	for _, l := range f.Chat.Lines {
		if l.Kind == proto.ChatTool && l.State == proto.TimelineRunning {
			return true
		}
	}
	return false
}

// Read reads a session's conversation, as the view opens and as the host's
// state changes.
func Read(x App, s *proto.Session, pane string, f *Feed) {
	h := x.HostOf(s.ID)
	if f.Busy || f.Got && f.Chat.Lines != nil && time.Since(f.At) < Refresh {
		return
	}
	if h.Client == nil {
		// A host that connects reads it as it does.
		return
	}
	f.Busy, f.Got, f.H = true, true, h
	cl := h.Client
	params := proto.ChatParams{Session: s.ID, Pane: pane}
	// It reads from a goroutine of its own: a call from the frame's would
	// wait on the frame.
	go func() {
		var out proto.Chat
		err := cl.Call("chat.get", params, &out)
		x.Main(func() {
			if x.Feed(s.ID) != f {
				return
			}
			f.Busy, f.At = false, time.Now()
			if err != nil {
				f.Err = err.Error()
				return
			}
			f.Err, f.Chat = "", out
		})
	}()
}

// empty says that the chat has nothing to show.
func empty(c *ui.Context, pal *theme.Palette, title, detail string) {
	ui.Column(c).Grow(1).Center().Gap(6).Padding(24).Children(func() {
		ui.Text(c, title).FontSize(14).FontWeight(600).TextColor(pal.Text).TextAlign(ui.Center)
		ui.Text(c, detail).FontSize(12.5).TextColor(pal.TextMuted).MaxWidth(300).TextAlign(ui.Center).LineHeight(1.4)
	})
}

// item is a line of the view: a line of the conversation, or a run of tools
// the agent ran one after the other, which the view folds into one row.
type item struct {
	at  int   // the line, for a line of its own
	run []int // the lines of a run of tools
}

// items is a conversation's lines as the view lays them out: the tools the
// agent ran in a row, one after the other, gathered.
func items(ls []proto.ChatLine) []item {
	var out []item
	for i := 0; i < len(ls); i++ {
		if ls[i].Kind != proto.ChatTool {
			out = append(out, item{at: i})
			continue
		}
		run := []int{i}
		for i+1 < len(ls) && ls[i+1].Kind == proto.ChatTool {
			i++
			run = append(run, i)
		}
		if len(run) == 1 {
			out = append(out, item{at: run[0]})
			continue
		}
		out = append(out, item{run: run})
	}
	return out
}

// lines shows the conversation: what the user asked to the right, the
// agent's words and reasoning to the left, and the tools it ran, a run of
// them folded to one row.
func lines(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, f *Feed, pane string) {
	ls := f.Chat.Lines
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
		case f.Chat.Note != "":
			msg = f.Chat.Note
		}
		empty(c, pal, locales.SideChat(), msg)
		return
	}
	f.List.FollowEnd, f.List.FollowAnimation = true, &ui.Smooth
	its := items(ls)
	ui.List(c.Key("chat"), &f.List, len(its), func(i int) {
		it := its[i]
		if it.run != nil {
			run(x, c, pal, s, f, pane, ls, it.run)
			return
		}
		line(x, c, pal, s, f, pane, it.at, ls[it.at])
	}).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Padding(18, 16, 12, 8).Gap(10)
}

// read tells that a read answered before.
func (f *Feed) read() bool { return f.Chat.Lines != nil }

// line is a line of the conversation.
func line(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, f *Feed, pane string, at int, l proto.ChatLine) {
	text := strings.TrimSpace(l.Text)
	if text == "" && l.Detail == "" && l.Title == "" {
		return
	}
	// The column's: a line of the conversation, in the middle of the pane
	// at a fixed width, as wide as it.
	ui.Column(c).AlignSelf(ui.Center).WidthPercent(100).MaxWidth(Width).AlignItems(ui.Stretch).Children(func() {
		switch l.Kind {
		case proto.ChatTool:
			tool(x, c, pal, s.ID, pane, at, l, f.Chat.Read, true)
			return
		case proto.ChatThink:
			ui.Column(c).MaxWidth(Width*0.9).Padding(6, 2).Gap(2).Children(func() {
				ui.Text(c, locales.ChatThought()).FontSize(11).FontWeight(600).TextColor(pal.TextFaint)
				markdownView(c, text, pal.TextMuted, pal.TextMuted.Alpha(0.08), pal)
			})
			return
		case proto.ChatUser:
			// The user's prompt: its bubble as wide as the column, which a
			// long prompt takes.
			ui.Row(c).AlignSelf(ui.Stretch).Justify(ui.End).Children(func() {
				ui.Box(c).Grow(1)
				ui.Column(c).Padding(8, 13).Radius(14).Background(pal.Accent).Children(func() {
					markdownView(c, text, onAccent, ui.RGBA(255, 255, 255, 0.16), pal)
				})
			})
			return
		}
		// The agent's: its words on the column itself, as a document reads,
		// a line of it at a time.
		ui.Row(c).AlignSelf(ui.Stretch).MaxWidth(bubble).Children(func() {
			ui.Column(c).Padding(4, 2).Children(func() {
				markdownView(c, text, pal.Text, pal.Text.Alpha(0.07), pal)
			})
		})
	})
}

// run is a run of tools: one row that counts them, which a click opens to
// the tools themselves, and the one that runs now at its end.
func run(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, f *Feed, pane string, ls []proto.ChatLine, at []int) {
	key := strconv.Itoa(at[0])
	expanded := openRun(x, s.ID, key, false)
	running := -1
	for _, i := range at {
		if ls[i].State == proto.TimelineRunning {
			running = i
		}
	}
	// The run is a line of the conversation as any other: in the column, as
	// wide as it.
	ui.Column(c).AlignSelf(ui.Center).WidthPercent(100).MaxWidth(Width).AlignItems(ui.Stretch).Children(func() {
		runHead(x, c, pal, s, f, pane, ls, at, key, running, expanded)
	})
}

// runHead is a run's row, and the tools under it while it is open.
func runHead(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, f *Feed, pane string, ls []proto.ChatLine, at []int, key string, running int, expanded bool) {
	head := ui.Row(c.Key("run-"+key)).Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(6).Cursor(ui.CursorPointer)
	if head.Hovered() {
		head.Background(pal.RowHover)
	}
	if head.Clicked() {
		openRun(x, s.ID, key, true)
		c.Invalidate()
	}
	head.Children(func() {
		if running >= 0 {
			// What runs says so, with the step it runs and its length: a
			// turn the user can stop.
			c.After(800 * time.Millisecond)
			ui.Box(c).Size(6, 6).Radius(3).Background(pal.Attention).Shrink(0)
			ui.Text(c, locales.ChatRunning()+" · "+ls[running].Title).FontSize(12).FontWeight(500).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			if arg := oneLine(ls[running].Detail); arg != "" {
				ui.Text(c, arg).Font("monospace").FontSize(11).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			}
			ui.Spacer(c)
			ui.Text(c, length(ls[running], f.Chat.Read)).FontSize(11).TextColor(pal.TextMuted).FontFeatures("tnum").Shrink(0)
			if pane != "" {
				paints.IconButton(c, "stop-"+key, glyph.Close, locales.ChatStop(), pal, func() { x.StopPane(pane) })
			}
			return
		}
		ui.Icon(c, glyph.Terminal).Size(12, 12).TextColor(pal.TextFaint).Shrink(0)
		ui.Text(c, locales.ChatRanTools(strconv.Itoa(len(at)))).FontSize(12).FontWeight(500).TextColor(pal.TextMuted).SingleLine().Shrink(0)
		ui.Spacer(c)
		ui.Text(c, length(ls[at[len(at)-1]], f.Chat.Read)).FontSize(11).TextColor(pal.TextFaint).FontFeatures("tnum").Shrink(0)
		ui.Icon(c, glyph.Chevron).Size(10, 10).TextColor(pal.TextFaint).Shrink(0).Rotate(turn(expanded))
	})
	if !expanded {
		return
	}
	ui.Column(c).Gap(2).Padding(0, 6).Children(func() {
		for _, i := range at {
			tool(x, c, pal, s.ID, pane, i, ls[i], f.Chat.Read, false)
		}
	})
}

// turn is a chevron's turn: right while folded, down while open.
func turn(open bool) float32 {
	if open {
		return 90
	}
	return 0
}

// openRun tells that a run of tools shows the tools it holds, and the click
// opens or folds them.
func openRun(x App, session, key string, click bool) bool {
	st := x.Chat()
	if st.Runs == nil {
		st.Runs = map[string]map[string]bool{}
	}
	runs := st.Runs[session]
	if runs == nil {
		runs = map[string]bool{}
		st.Runs[session] = runs
	}
	if click {
		runs[key] = !runs[key]
	}
	return runs[key]
}

// tool is a tool the agent ran: its name, what it ran, how long it took,
// and the arguments and output under it, which a click folds or opens.
func tool(x App, c *ui.Context, pal *theme.Palette, session, pane string, at int, l proto.ChatLine, read int64, framed bool) {
	key := strconv.Itoa(at)
	args := oneLine(l.Detail)
	if l.Detail == l.Title {
		args = ""
	}
	if args == l.Title {
		args = ""
	}
	card := ui.Row(c.Key("tool-"+key)).Gap(7).AlignItems(ui.Center).MinHeight(30).Padding(0, 8).Radius(7).Cursor(ui.CursorPointer)
	if framed {
		card.Background(pal.Text.Alpha(0.04))
	}
	if card.Hovered() {
		card.Background(pal.RowHover)
	}
	if card.Clicked() {
		openTool(x, session, key, true)
		c.Invalidate()
	}
	open := false
	running := l.State == proto.TimelineRunning && pane != ""
	if running {
		c.After(800 * time.Millisecond)
	}
	card.Children(func() {
		open = openTool(x, session, key, false)
		if running {
			ui.Box(c).Size(6, 6).Radius(3).Background(pal.Attention).Shrink(0)
		} else {
			ui.Icon(c, glyph.Gear).Size(12, 12).TextColor(pal.TextFaint).Shrink(0)
		}
		ui.Text(c, l.Title).FontSize(12).FontWeight(500).TextColor(pal.Text).SingleLine().Shrink(0)
		if args != "" {
			ui.Text(c, args).Font("monospace").FontSize(11).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		} else {
			ui.Spacer(c)
		}
		if d := length(l, read); d != "" {
			ui.Text(c, d).FontSize(11).TextColor(pal.TextFaint).FontFeatures("tnum").Shrink(0)
		}
		if running {
			paints.IconButton(c, "stop-"+key, glyph.Close, locales.ChatStop(), pal, func() { x.StopPane(pane) })
			return
		}
		ui.Icon(c, glyph.Chevron).Size(10, 10).TextColor(pal.TextFaint).Shrink(0).Rotate(turn(open))
	})
	if !open || l.Detail == "" {
		return
	}
	ui.Scroll(c.Key("detail-"+key)).MaxHeight(240).Padding(8, 10).Margin(2, 20, 2, 8).Radius(8).Background(pal.Text.Alpha(0.03)).Children(func() {
		ui.Text(c, short(l.Detail, 4000)).Font("monospace").FontSize(11.5).TextColor(pal.TextMuted).LineHeight(1.35).Wrap().Selectable()
	})
}

// oneLine is a tool's arguments as one line of its row: its first line, cut.
func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 90 {
		return string(r[:90]) + "…"
	}
	return s
}

// length is how long a tool ran, or has been running.
func length(l proto.ChatLine, read int64) string {
	if l.Start == 0 {
		return ""
	}
	end := l.End
	if end == 0 {
		end = read
	}
	s := (end - l.Start) / 1000
	switch {
	case s < 1:
		return ""
	case s < 60:
		return strconv.Itoa(int(s)) + "s"
	case s < 3600:
		return strconv.Itoa(int(s/60)) + "m " + pad(int(s%60)) + "s"
	}
	return strconv.Itoa(int(s/3600)) + "h " + pad(int(s%3600/60)) + "m"
}

// pad is a number under ten with its nought.
func pad(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// themes are the Markdown themes of the chat windows, by day and by
// night: a message's colors, which differ, its own.
var themes = map[string]markdown.Theme{}

// onAccent is the color of the text on the user's prompts.
var onAccent = ui.Hex("#ffffff")

// markdownView shows a message's text as Markdown, as a chat app does:
// headings, lists, quotes, tables, code blocks, and links, which open in
// the browser.
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

// openTool tells that a conversation's tool, at a line, shows its details:
// a click on its line folds or opens them.
func openTool(x App, session, key string, open bool) bool {
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
	if open {
		keys[key] = !keys[key]
	}
	return keys[key]
}

// short is s in one line, cut at n characters.
func short(s string, n int) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
