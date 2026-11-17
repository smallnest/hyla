// Package subagentsview holds the side panel's Subagents view: the agents a
// session's coding agents gave their work to, what each was asked, from when
// to when it ran, and the step it runs now while it runs.
package subagentsview

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/info"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

const (
	// tick is the time between two reads while a subagent runs, and idle,
	// much longer, once none does.
	tick = 1500 * time.Millisecond
	idle = 10 * time.Second
)

// App is what the view asks of the app it shows in.
type App interface {
	// SelectedSession is the session the window shows, and HostOf the
	// machine a session or a pane is of.
	SelectedSession() *proto.Session
	HostOf(id string) *host.Host
	// SubagentsNow is what the view shows of the session's panes, and
	// SetSubagents holds it.
	SubagentsNow() *Info
	SetSubagents(st *Info)
	// SetFocus focuses a pane of the session.
	SetFocus(s *proto.Session, pane string)
	// Main runs fn on the main thread.
	Main(fn func())
	// AfterFrame runs fn after the frame.
	AfterFrame(fn func())
}

// Info is what the view shows of a session's subagents.
type Info struct {
	Session string
	ReadAt  time.Time
	Busy    bool
	Res     *proto.Subagents
	Err     string
}

// Side draws the Subagents view of a session: a line of what its agents'
// subagents do, then the panes that have any, each with its subagents.
func Side(x App, c *ui.Context, pal *theme.Palette, s *proto.Session) {
	st := InfoOf(x, s)
	tickRead(x, c, st, s)
	ui.Scroll(c.Key("subagents")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Padding(6, 10, 8, 10).AlignItems(ui.Stretch).Gap(2).Children(func() {
			if st.Err != "" {
				info.Empty(c, pal, st.Err)
				return
			}
			res := st.Res
			if res == nil {
				info.Empty(c, pal, locales.SubagentsWait())
				return
			}
			running, done, failed := counts(res)
			ui.Text(c, locales.SubagentsRunning(strconv.Itoa(running))+" · "+locales.SubagentsDone(strconv.Itoa(done+failed))).
				FontSize(11.5).TextColor(pal.TextFaint).FontFeatures("tnum").Padding(2, 4, 6, 4)
			if len(res.Panes) == 0 {
				info.Empty(c, pal, locales.SubagentsEmpty())
				return
			}
			for _, ps := range res.Panes {
				paneHead(c, pal, s, ps)
				// An agent whose records Hyla does not read says so.
				if ps.Note != "" {
					info.Empty(c, pal, ps.Note)
					continue
				}
				for i, sb := range ps.Subs {
					subagentRow(x, c, pal, s, ps, i, sb, res.Read)
				}
			}
		})
	})
}

// counts is how many of a read's subagents run, ended and failed.
func counts(res *proto.Subagents) (running, done, failed int) {
	for _, ps := range res.Panes {
		for _, sb := range ps.Subs {
			switch sb.State {
			case proto.TimelineRunning:
				running++
			case proto.TimelineFailed:
				failed++
			default:
				done++
			}
		}
	}
	return running, done, failed
}

// InfoOf returns a session's subagents as the view shows them, made the
// first time the session shows.
func InfoOf(x App, s *proto.Session) *Info {
	st := x.SubagentsNow()
	if st == nil || st.Session != s.ID {
		st = &Info{Session: s.ID}
		x.SetSubagents(st)
	}
	return st
}

// tickRead asks hylad for the session's subagents as it falls due: every
// tick while one runs, else every idle, and at once when the view comes to
// a session it has not read.
func tickRead(x App, c *ui.Context, st *Info, s *proto.Session) {
	due := idle
	if anyRunning(st.Res) {
		due = tick
	}
	if st.Busy || time.Since(st.ReadAt) >= due {
		if !st.Busy {
			st.ReadAt, st.Busy = time.Now(), true
			read(x, st, s)
		}
	}
	c.After(tick)
}

// anyRunning tells whether a read has a subagent that still runs.
func anyRunning(res *proto.Subagents) bool {
	if res == nil {
		return false
	}
	for _, ps := range res.Panes {
		for _, sb := range ps.Subs {
			if sb.State == proto.TimelineRunning {
				return true
			}
		}
	}
	return false
}

// read asks hylad for a session's subagents.
func read(x App, st *Info, s *proto.Session) {
	h := x.HostOf(s.ID)
	if h == nil || h.Client == nil {
		st.Busy = false
		return
	}
	cl := h.Client
	go func() {
		var r proto.Subagents
		err := cl.Call("subagents.get", proto.SubagentsParams{Session: s.ID}, &r)
		x.Main(func() {
			if x.SubagentsNow() != st {
				return
			}
			st.Busy = false
			if err != nil {
				st.Err = locales.SubagentsDown()
				return
			}
			st.Res, st.ReadAt, st.Err = &r, time.Now(), ""
		})
	}()
}

// paneHead is the name of a pane whose agent gave its work to others, and
// the agent that did.
func paneHead(c *ui.Context, pal *theme.Palette, s *proto.Session, ps proto.PaneSubagents) {
	name := ps.Pane
	if p := s.PaneByID(ps.Pane); p != nil {
		if p.Title != "" {
			name = p.Title
		} else if p.Process != "" {
			name = p.Process
		}
	}
	ui.Row(c).Padding(6, 4, 4, 4).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, glyph.Terminal).Size(12, 12).TextColor(pal.TextFaint).Shrink(0)
		ui.Text(c, name).FontSize(11.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		// The agent, where it is not what the pane is called already.
		if ps.Agent != "" && ps.Agent != name {
			ui.Text(c, ps.Agent).FontSize(11).TextColor(pal.TextFaint).SingleLine().Shrink(0)
		}
	})
}

// subagentRow is one subagent: a dot of its state, what it is and was
// asked, how long it ran, and the step it runs now while it runs. A click
// goes to its pane.
func subagentRow(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, ps proto.PaneSubagents, i int, sb proto.Subagent, read int64) {
	key := "sub-" + ps.Pane + "-" + strconv.Itoa(i)
	row := ui.Row(c.Key(key)).MinHeight(48).Padding(0, 8).Gap(9).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if row.Hovered() {
		row.Background(pal.RowHover)
	}
	if row.Clicked() {
		x.AfterFrame(func() { x.SetFocus(s, ps.Pane) })
	}
	row.Children(func() {
		stateDot(c, pal, sb.State)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				kind := sb.Type
				if kind == "" {
					kind = locales.SubagentsAgent()
				}
				ui.Text(c, kind).FontSize(12.5).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
				ui.Spacer(c)
				ui.Text(c, length(sb, read)).FontSize(11).TextColor(stateColor(pal, sb.State)).FontFeatures("tnum").Shrink(0)
			})
			if sb.Task != "" {
				ui.Text(c, sb.Task).FontSize(11.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…")
			}
			if meta := metaOf(pal, sb); meta != "" {
				ui.Text(c, meta).FontSize(11).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
			}
		})
	})
}

// metaOf is what a subagent does now or what gave it the work: the step it
// runs, else the turn of the agent's that asked it.
func metaOf(pal *theme.Palette, sb proto.Subagent) string {
	if sb.State == proto.TimelineRunning && sb.Tool != "" {
		return sb.Tool
	}
	return sb.Turn
}

// stateDot is a subagent's state, as a dot.
func stateDot(c *ui.Context, pal *theme.Palette, state string) {
	ui.Box(c).Size(7, 7).Radius(4).Shrink(0).Background(stateColor(pal, state))
}

// stateColor is a subagent's colour for its state.
func stateColor(pal *theme.Palette, state string) ui.Color {
	switch state {
	case proto.TimelineRunning:
		return pal.TlSub
	case proto.TimelineFailed:
		return pal.TlFail
	}
	return pal.TextFaint
}

// length is how long a subagent ran, or has been running at the read.
func length(sb proto.Subagent, read int64) string {
	end := sb.End
	if end == 0 {
		end = read
	}
	if end == 0 {
		end = time.Now().UnixMilli()
	}
	ms := end - sb.Start
	if ms < 0 {
		ms = 0
	}
	s := ms / 1000
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm %02ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %02dm", s/3600, s%3600/60)
}
