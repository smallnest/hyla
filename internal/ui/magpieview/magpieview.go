// Package magpieview holds Magpie's card in the Info view: what the coding
// agents' gateway spent, read from it while it answers, and why the card
// is empty while it does not.
package magpieview

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/app/magpie"
	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/info"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

const (
	// tick is the time between two reads of what it keeps, and sniff,
	// much longer, between two looks for it while it ran.
	tick  = 15 * time.Second
	sniff = time.Minute
	// pages bounds the reads of its requests, which one session's calls
	// may lie deep in, and wait keeps them off the redraws: a read that
	// takes that long is off the table until it answers.
	pages = 16
	wait  = time.Minute
)

// Magpie is a coding agents' gateway that runs on this machine, on port
// 3425 (MAGPIE_URL, else MAGPIE_PORT, asks another), and keeps a file of
// what they spent (~/.config/magpie/usage.jsonl). The Info view shows its
// usage while it runs: today's and this pane's conversation's, the plan
// windows closest to their limit, and its latest call.

// App is what the card asks of the app.
type App interface {
	// MagpieNow is what the card shows of Magpie, and SetMagpie holds it.
	MagpieNow() *Info
	SetMagpie(st *Info)
	// SelectedSession is the session the window shows, and FocusOf its
	// pane the user is in, whose host HostOf is.
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

// Info is what the Info view shows of Magpie.
type Info struct {
	ReadAt  time.Time
	SniffAt time.Time
	Busy    bool
	Ran     bool // it answered once: the file may change again
	Usage   *magpie.Usage
	Err     string
}

// magpieOf returns Magpie's Info view, made the first time.
func Of(a App) *Info {
	st := a.MagpieNow()
	if st == nil {
		st = &Info{}
		a.SetMagpie(st)
	}
	return st
}

// tick reads Magpie as it falls due, and looks for it while it ran,
// as the Info view shows.
// Tick reads Magpie as it falls due, and looks for it while it ran, as
// the Info view shows.
func Tick(a App, c *ui.Context) {
	st := Of(a)
	due := tick
	if !st.Ran {
		due = sniff
	}
	if !st.Busy && st.Usage == nil && time.Since(st.ReadAt) >= wait {
		st.Busy = false
	}
	if !st.Busy && (st.Usage == nil || time.Since(st.ReadAt) >= due) {
		st.ReadAt, st.Busy = time.Now(), true
		read(a, st)
	}
	if st.Usage == nil && time.Since(st.ReadAt) < wait {
		c.After(wait / 10)
		return
	}
	c.After(tick / 2)
}

// read asks for Magpie's usage, and for how many calls the session
// of the pane the user is in made.
func read(a App, st *Info) {
	base := magpie.URL()
	go func() {
		// The daemon answers the session while the reads wait on Magpie.
		session := PaneSession(a)
		u, err := magpie.Fetch(base, session)
		a.Main(func() {
			if a.MagpieNow() != st {
				return
			}
			st.Busy = false
			if err != nil {
				if st.Usage == nil {
					st.Err = locales.InfoMagpieDown()
				}
				return
			}
			st.Ran, st.Usage, st.Err = true, u, ""
		})
	}()
}

// paneSession is the coding agent's session of the pane the user is in, as
// its hooks told hylad: "" while it told of none.
// PaneSession is the coding agent's session of the pane the user is in,
// as its hooks told hylad: "" while it told of none.
func PaneSession(a App) string {
	s := a.SelectedSession()
	if s == nil {
		return ""
	}
	p := s.PaneByID(a.FocusOf(s))
	if p == nil || p.Agent == "" {
		return ""
	}
	cl := a.HostOf(p.ID).Client
	if cl == nil {
		return ""
	}
	var r proto.AgentSessionResult
	if cl.Call("pane.agentSession", proto.AgentSessionParams{PID: p.PID, Agent: p.Agent}, &r) != nil {
		return ""
	}
	return r.Session
}

// magpieGlyph is a paper plane, the call it sends for the agent.
var magpieGlyph = glyph.Drawn(`<path d="M3 4.5 13 8 9.5 9.5 8 13 3 4.5Z"/><path d="M5.8 6.2c1.9.4 3.3.3 4.6-.6"/>`)

// infoMagpie draws Magpie's card last: while it answers, what it spent
// today and this conversation spent through it, the plan windows closest
// to their limits and its latest call; while it does not, why it does not,
// once it answered or as a message it installed it.
// Card draws Magpie's card last: while it answers, what it spent today
// and this conversation spent through it, the plan windows closest to
// their limits and its latest call; while it does not, why it does not,
// once it answered or as a message it installed it.
func Card(a App, c *ui.Context, pal *theme.Palette) {
	st := State(a, c)
	if st.Usage == nil && st.Err == "" {
		return
	}
	aside := ""
	if st.Err == "" {
		aside = magpie.Cost(st.Usage.Today.Cost)
	}
	info.Card(c, a, pal, info.FoldMagpie, magpieGlyph, locales.InfoMagpie(), aside, func() {
		switch {
		case st.Usage == nil:
			info.Empty(c, pal, st.Err)
		default:
			usageRows(c, pal, st.Usage, st.Err)
		}
	})
}

// usageRows lists what Magpie spent, then its sessions before now,
// another's or the one that ended.
func usageRows(c *ui.Context, pal *theme.Palette, u *magpie.Usage, down string) {
	info.Rows(c, func() {
		metric(c, pal, locales.InfoMagpieCalls(), strconv.Itoa(u.Today.Calls))
		info.Rule(c, pal)
		metric(c, pal, locales.InfoMagpieTokens(), magpie.Tokens(u.Today.Count()))
		info.Rule(c, pal)
		metricSub(c, pal, locales.InfoMagpieLeft(), magpie.Hit(u.Today), locales.InfoMagpieCache())
		if u.Session > 0 || u.SessErr {
			info.Rule(c, pal)
			metric(c, pal, locales.InfoMagpieSession(), sessionLine(u))
		}
		for i, w := range u.Windows {
			if i >= 4 {
				break
			}
			info.Rule(c, pal)
			metricSub(c, pal, w.Provider, magpie.Left(w.Left), w.Name)
		}
		if u.Latest.Served != "" {
			info.Rule(c, pal)
			metric(c, pal, locales.InfoMagpieLatest(), callLine(u.Latest))
		}
		if down != "" {
			info.Rule(c, pal)
			info.Empty(c, pal, down)
		}
	})
}

// State is Magpie's Info view, read as the card shows it.
func State(a App, c *ui.Context) *Info {
	st := Of(a)
	Tick(a, c)
	return st
}

// metric is a row of a card: a label, and a value on the right.
func metric(c *ui.Context, pal *theme.Palette, label, value string) {
	ui.Row(c).MinHeight(32).Padding(0, 12).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
		metricValue(c, pal, value)
	})
}

// metricSub is a metric of two lines: its value over a word of what it
// counts, or a detail of it.
func metricSub(c *ui.Context, pal *theme.Palette, label, value, sub string) {
	ui.Row(c).MinHeight(42).Padding(6, 12).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
		ui.Column(c).Gap(1).AlignItems(ui.End).Shrink(1).MinWidth(80).Children(func() {
			metricValue(c, pal, value)
			ui.Text(c, sub).FontSize(11).TextColor(pal.TextFaint).SingleLine()
		})
	})
}

// metricValue is a metric's value, on the right.
func metricValue(c *ui.Context, pal *theme.Palette, value string) {
	ui.Text(c, value).FontSize(12.5).FontWeight(500).TextColor(pal.Text).FontFeatures("tnum").SingleLine().Shrink(0)
}

// sessionLine is this conversation's calls, and their cost.
func sessionLine(u *magpie.Usage) string {
	if u.SessErr {
		return "500+"
	}
	return strconv.Itoa(u.Session) + " · " + magpie.Cost(u.SessionCost)
}

// callLine is the latest call: its status, what served it and how
// long it took.
func callLine(l magpie.Call) string {
	line := strconv.Itoa(l.Status) + " " + l.Served
	if l.Ms > 0 {
		line += " · " + magpie.FmtTime(l.Ms)
	}
	return line
}
