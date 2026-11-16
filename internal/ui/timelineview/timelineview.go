// Package timelineview holds the timeline's card in the Info view: what
// the pane's coding agent did, turn by turn, from the records its agent
// keeps beside its conversation, as hylad reads them.
package timelineview

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/info"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

const (
	// tick is the time between two reads of the records while the agent
	// runs.
	tick = 3 * time.Second
	// idle is much longer, between two reads once its last turn ended.
	idle = 30 * time.Second
)

// App is what the card asks of the app.
type App interface {
	// AgentPane is the pane of the session the user is in, and its agent.
	AgentPane(s *proto.Session) *proto.Pane
	// TimelineNow is what the card shows of a pane, and SetTimeline holds
	// it.
	TimelineNow(pane string) *Info
	SetTimeline(pane string, st *Info)
	// SelectedSession is the session the window shows, whose FocusOf is
	// the pane the user is in, and whose host HostOf is.
	SelectedSession() *proto.Session
	FocusOf(s *proto.Session) string
	HostOf(id string) *host.Host
	// Main runs fn on the main thread.
	Main(fn func())
	// AfterFrame runs fn after the frame.
	AfterFrame(fn func())
	// The rest, as the Info view's card asks it.
	info.App
}

// Info is what the Info view shows of a pane's timeline.
type Info struct {
	Pane   string
	ReadAt time.Time
	Busy   bool
	Timely *proto.Timeline
	Err    string
}

// timelineGlyph is a line of dots: the turns of a timeline.
var timelineGlyph = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M8 1.5v13"/><circle cx="3.5" cy="4.5" r="1.6" fill="currentColor" stroke="none"/><circle cx="3.5" cy="11.5" r="1.6" fill="currentColor" stroke="none"/></svg>`))

// pane is the pane the card shows: the one the user is in while it runs an
// agent, else the session's first one that does.
func pane(a App) *proto.Pane {
	s := a.SelectedSession()
	if s == nil {
		return nil
	}
	if p := a.AgentPane(s); p != nil && proto.StartsTimeline(p.Agent) {
		return p
	}
	for i := range s.Panes {
		if proto.StartsTimeline(s.Panes[i].Agent) {
			return &s.Panes[i]
		}
	}
	return nil
}

// InfoOf returns a pane's timeline as the card shows it, made the first
// time.
func InfoOf(a App, p *proto.Pane) *Info {
	st := a.TimelineNow(p.ID)
	if st == nil || st.Pane != p.ID {
		st = &Info{Pane: p.ID}
		a.SetTimeline(p.ID, st)
	}
	return st
}

// Tick reads a pane's timeline as it falls due: every tick while its last
// turn runs, else every idle.
func Tick(a App, c *ui.Context, st *Info, p *proto.Pane) {
	due := idle
	if runs(st.Timely) {
		due = tick
	}
	if st.Busy || time.Since(st.ReadAt) >= due {
		if !st.Busy {
			st.ReadAt, st.Busy = time.Now(), true
			read(a, st, p)
		}
	}
	c.After(tick)
}

// runs tells whether a timeline's last turn is still going.
func runs(t *proto.Timeline) bool {
	if t == nil || len(t.Turns) == 0 {
		return false
	}
	last := t.Turns[len(t.Turns)-1]
	return last.End == 0
}

// read asks hylad for a pane's timeline.
func read(a App, st *Info, p *proto.Pane) {
	cl := a.HostOf(p.ID).Client
	if cl == nil {
		st.Busy = false
		return
	}
	go func() {
		var r proto.Timeline
		err := cl.Call("timeline.get", proto.TimelineParams{Pane: p.ID}, &r)
		a.Main(func() {
			if a.TimelineNow(p.ID) != st {
				return
			}
			st.Busy = false
			if err != nil {
				st.Err = locales.InfoTimelineDown()
				return
			}
			st.Timely, st.ReadAt, st.Err = &r, time.Now(), ""
		})
	}()
}

// Card draws the timeline's card: the turns of the agent in the pane the
// user is in, the newest first, each a line of steps. A card has no body
// while no pane runs an agent whose records Hyla reads.
func Card(a App, c *ui.Context, pal *theme.Palette) {
	p := pane(a)
	if p == nil {
		return
	}
	st := InfoOf(a, p)
	Tick(a, c, st, p)
	t := st.Timely
	if t == nil {
		return
	}
	aside := ""
	if n := len(t.Turns); n > 0 {
		aside = locales.InfoTimelineTurns(strconv.Itoa(n))
	}
	if runs(t) {
		// What runs has a card that paints again, for its bars to blink.
		c.After(tick / 3)
	}
	info.Card(c, a, pal, info.FoldTimeline, timelineGlyph, locales.InfoTimeline(), aside, func() {
		switch {
		case len(t.Turns) == 0 && t.Note != "":
			info.Empty(c, pal, t.Note)
		case len(t.Turns) == 0:
			info.Empty(c, pal, locales.InfoTimelineEmpty())
		default:
			turns(c, pal, t, st.Err)
		}
	})
}

// shown is how many turns the card lists, the newest first.
const shown = 12

// turns lists a timeline's turns, the newest first.
func turns(c *ui.Context, pal *theme.Palette, t *proto.Timeline, down string) {
	info.Rows(c, func() {
		n := len(t.Turns)
		for i := n - 1; i >= 0 && n-i <= shown; i-- {
			if i != n-1 {
				info.Rule(c, pal)
			}
			turn(c, pal, t.Turns[i], t.Read)
		}
		if down != "" {
			info.Rule(c, pal)
			info.Empty(c, pal, down)
		}
	})
}

// turn is one of a timeline's turns: when it began, its first words, its
// length and state, then a line of the steps it ran, each a bar from its
// start to its end, and a row for each step that waits for the user or
// failed.
func turn(c *ui.Context, pal *theme.Palette, t proto.TimelineStep, read int64) {
	end := t.End
	if end == 0 {
		end = read
	}
	span := end - t.Start
	if span <= 0 {
		span = 1
	}
	ui.Column(c).Padding(6, 12, 8, 12).Gap(6).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, clock(t.Start)).FontSize(11.5).TextColor(pal.TextFaint).FontFeatures("tnum").Shrink(0)
			ui.Text(c, t.Title).FontSize(12.5).TextColor(pal.Text).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
			ui.Text(c, stateWord(t.State, end-t.Start)).FontSize(11).TextColor(stateColor(pal, t.State)).FontFeatures("tnum").SingleLine().Shrink(0)
		})
		line(c, pal, t.Children, t.Start, span, read)
		for _, s := range t.Children {
			if s.State == proto.TimelineWaiting || s.State == proto.TimelineFailed {
				stepRow(c, pal, s)
			}
		}
	})
}

// laneH is the height of a turn's line's lanes, in points.
const laneH = 5

// line draws a turn's line: a lane for what the agent ran, wrote and
// reasoned, then a lane for each subagent it gave work to, a bar from a
// step's start to its end of the turn's span. A bar that still runs reaches
// now.
func line(c *ui.Context, pal *theme.Palette, steps []proto.TimelineStep, start, span int64, read int64) {
	all := lanesOf(steps)
	ui.Box(c).Height(float32(len(all)) * (laneH + gap)).Radius(2).Draw(func(p *ui.Painter, r ui.Rect) {
		for i, bars := range all {
			y := r.Y + float32(i)*(laneH+gap)
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: laneH}, pal.Text.Alpha(0.05), 2)
			for _, s := range bars {
				if x, w, ok := spanOf(s, start, span); ok {
					p.Fill(ui.Rect{X: r.X + (r.W-2)*float32(x), Y: y, W: max((r.W-2)*float32(w), 2), H: laneH}, barColor(p, pal, s), 2)
				}
			}
		}
		if len(all) > 0 && read > start && read < start+span {
			x := r.X + (r.W-2)*float32(read-start)/float32(span)
			p.Fill(ui.Rect{X: x, Y: r.Y, W: 1, H: r.H}, pal.Text.Alpha(0.5), 0)
		}
	})
}

// gap is between two lanes of a turn's line.
const gap = 2

// lanesOf is a turn's lanes: the steps' own bars in the first, in their
// kinds' colours, and the subagents' in the lanes after it.
func lanesOf(steps []proto.TimelineStep) [][]proto.TimelineStep {
	first := make([]proto.TimelineStep, 0, len(steps)*2)
	var subs [][]proto.TimelineStep
	for _, s := range steps {
		if s.Kind == proto.TimelineSubagent {
			subs = append(subs, []proto.TimelineStep{s})
			continue
		}
		if s.Kind != proto.TimelineExchange {
			first = append(first, s)
			continue
		}
		for _, ch := range s.Children {
			// The step's own reasoning and words: its tools are its bars.
			switch ch.Kind {
			case proto.TimelineThink, proto.TimelineText:
				first = append(first, ch)
			}
		}
	}
	// A turn with nothing but its exchanges still has its line.
	if len(first) == 0 && len(subs) == 0 {
		first = steps
	}
	return append([][]proto.TimelineStep{first}, subs...)
}

// spanOf is a step's span in a turn's: where it begins, of 1, and how much
// of it it takes. A step that still runs reaches the end.
func spanOf(s proto.TimelineStep, start, span int64) (at, width float64, ok bool) {
	end := s.End
	if end == 0 {
		end = start + span
	}
	at = float64(s.Start-start) / float64(span)
	if at >= 1 {
		return 0, 0, false
	}
	at = max(at, 0)
	width = min(float64(end-s.Start)/float64(span), 1-at)
	return at, width, width > 0
}

// barColor is a step's colour: its kind's, and what it is doing. One that
// runs blinks, as the agent waits on it or is busy in it.
func barColor(p *ui.Painter, pal *theme.Palette, s proto.TimelineStep) ui.Color {
	switch s.State {
	case proto.TimelineWaiting:
		return pal.Attention
	case proto.TimelineFailed:
		return pal.Danger
	case proto.TimelineRunning:
		p.After(600 * time.Millisecond)
		if p.Now().UnixMilli()/600%2 == 0 {
			return pal.Text.Alpha(0.75)
		}
		return pal.Text.Alpha(0.4)
	}
	return kindColor(pal, s.Kind)
}

// stepRow is a step the user wants of a turn: a dot of its state, its
// kind and name, its detail, and what it waits for or failed at.
func stepRow(c *ui.Context, pal *theme.Palette, s proto.TimelineStep) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(5, 5).Radius(3).Background(stateColor(pal, s.State)).Shrink(0)
		ui.Text(c, s.Title).FontSize(12).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(0).MaxWidth(120)
		if s.Detail != "" {
			ui.Text(c, s.Detail).FontSize(11.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
		} else {
			ui.Spacer(c)
		}
		ui.Text(c, stateWord(s.State, s.End-s.Start)).FontSize(11).TextColor(stateColor(pal, s.State)).FontFeatures("tnum").Shrink(0)
	})
}

// stateWord is a step's state as what it is doing, with its length.
func stateWord(state string, ms int64) string {
	d := length(ms)
	switch state {
	case proto.TimelineWaiting:
		return locales.InfoTimelineWaiting() + " · " + d
	case proto.TimelineRunning:
		return d
	case proto.TimelineFailed:
		return locales.InfoTimelineFailed() + " · " + d
	}
	return d
}

// length is a time in ms as short words: 42s, 3m 20s, 1h 05m.
func length(ms int64) string {
	s := max(0, ms) / 1000
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm %02ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %02dm", s/3600, s%3600/60)
}

// clock is when a turn began, as its time of day.
func clock(ms int64) string {
	return time.UnixMilli(ms).Format("15:04")
}

// kindColor is the colour of a step's kind.
func kindColor(pal *theme.Palette, kind string) ui.Color {
	switch kind {
	case proto.TimelineTool:
		return pal.TextMuted.Alpha(0.5)
	case proto.TimelineThink:
		return pal.TextMuted.Alpha(0.22)
	case proto.TimelineText:
		return pal.TextMuted.Alpha(0.35)
	case proto.TimelineSubagent:
		return pal.Done.Alpha(0.6)
	case proto.TimelineExchange:
		return pal.TextMuted.Alpha(0.2)
	}
	return pal.TextMuted.Alpha(0.3)
}

// stateColor is a step's colour for its state.
func stateColor(pal *theme.Palette, state string) ui.Color {
	switch state {
	case proto.TimelineWaiting:
		return pal.Attention
	case proto.TimelineFailed:
		return pal.Danger
	case proto.TimelineRunning:
		return pal.Text
	}
	return pal.TextFaint
}
