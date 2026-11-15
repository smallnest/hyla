package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/magpie"
	"hyla/internal/proto"
	"hyla/internal/ui/info"
	"hyla/internal/ui/theme"

	"hyla/internal/ui/glyph"
	"hyla/locales"
)

// Magpie is a coding agents' gateway that runs on this machine, on port
// 3425 (MAGPIE_URL, else MAGPIE_PORT, asks another), and keeps a file of
// what they spent (~/.config/magpie/usage.jsonl). The Info view shows its
// usage while it runs: today's and this pane's conversation's, the plan
// windows closest to their limit, and its latest call.

const (
	// magpieTick is the time between two reads of what it keeps, and
	// magpieSniff, much longer, between two looks for it while it ran.
	magpieTick  = 15 * time.Second
	magpieSniff = time.Minute
	// magpiePages bounds the reads of its requests, which one session's
	// calls may lie deep in, and magpieWait keeps them off the redraws:
	// a read that takes that long is off the table until it answers.
	magpiePages = 16
	magpieWait  = time.Minute
)

// magpieInfo is what the Info view shows of Magpie.
type magpieInfo struct {
	readAt  time.Time
	sniffAt time.Time
	busy    bool
	ran     bool // it answered once: the file may change again
	usage   *magpie.Usage
	err     string
}

// magpieOf returns Magpie's Info view, made the first time.
func (a *App) magpieOf() *magpieInfo {
	if a.Magpie == nil {
		a.Magpie = &magpieInfo{}
	}
	return a.Magpie
}

// magpieTick reads Magpie as it falls due, and looks for it while it ran,
// as the Info view shows.
func (a *App) magpieTick(c *ui.Context) {
	st := a.magpieOf()
	due := magpieTick
	if !st.ran {
		due = magpieSniff
	}
	if !st.busy && st.usage == nil && time.Since(st.readAt) >= magpieWait {
		st.busy = false
	}
	if !st.busy && (st.usage == nil || time.Since(st.readAt) >= due) {
		st.readAt, st.busy = time.Now(), true
		a.readMagpie(st)
	}
	if st.usage == nil && time.Since(st.readAt) < magpieWait {
		c.After(magpieWait / 10)
		return
	}
	c.After(magpieTick / 2)
}

// readMagpie asks for Magpie's usage, and for how many calls the session
// of the pane the user is in made.
func (a *App) readMagpie(st *magpieInfo) {
	base := magpie.URL()
	go func() {
		// The daemon answers the session while the reads wait on Magpie.
		session := a.paneSession()
		u, err := magpie.Fetch(base, session)
		a.update(func() {
			if a.Magpie != st {
				return
			}
			st.busy = false
			if err != nil {
				if st.usage == nil {
					st.err = locales.InfoMagpieDown()
				}
				return
			}
			st.ran, st.usage, st.err = true, u, ""
		})
	}()
}

// paneSession is the coding agent's session of the pane the user is in, as
// its hooks told hylad: "" while it told of none.
func (a *App) paneSession() string {
	s := a.session(a.Selected)
	if s == nil {
		return ""
	}
	p := s.PaneByID(a.focusOf(s))
	if p == nil || p.Agent == "" {
		return ""
	}
	cl := a.hostOf(p.ID).Client
	if cl == nil {
		return ""
	}
	var r proto.AgentSessionResult
	if cl.Call("pane.agentSession", proto.AgentSessionParams{PID: p.PID, Agent: p.Agent}, &r) != nil {
		return ""
	}
	return r.Session
}

// --- the Info view's card ---

// magpieGlyph is a paper plane, the call it sends for the agent.
var magpieGlyph = glyph.Drawn(`<path d="M3 4.5 13 8 9.5 9.5 8 13 3 4.5Z"/><path d="M5.8 6.2c1.9.4 3.3.3 4.6-.6"/>`)

// infoMagpie draws Magpie's card last: while it answers, what it spent
// today and this conversation spent through it, the plan windows closest
// to their limits and its latest call; while it does not, why it does not,
// once it answered or as a message it installed it.
func (a *App) infoMagpie(c *ui.Context, pal *theme.Palette) {
	st := a.infoMagpieState(c)
	if st.usage == nil && st.err == "" {
		return
	}
	aside := ""
	if st.err == "" {
		aside = magpie.Cost(st.usage.Today.Cost)
	}
	info.Card(c, a, pal, info.FoldMagpie, magpieGlyph, locales.InfoMagpie(), aside, func() {
		switch {
		case st.usage == nil:
			info.Empty(c, pal, st.err)
		default:
			infoMagpieUsage(c, pal, st.usage, st.err)
		}
	})
}

// infoMagpieUsage lists what Magpie spent, then its sessions before now,
// another's or the one that ended.
func infoMagpieUsage(c *ui.Context, pal *theme.Palette, u *magpie.Usage, down string) {
	info.Rows(c, func() {
		infoMetric(c, pal, locales.InfoMagpieCalls(), strconv.Itoa(u.Today.Calls))
		info.Rule(c, pal)
		infoMetric(c, pal, locales.InfoMagpieTokens(), magpie.Tokens(u.Today.Count()))
		info.Rule(c, pal)
		infoMetricSub(c, pal, locales.InfoMagpieLeft(), magpie.Hit(u.Today), locales.InfoMagpieCache())
		if u.Session > 0 || u.SessErr {
			info.Rule(c, pal)
			infoMetric(c, pal, locales.InfoMagpieSession(), magpieSessionLine(u))
		}
		for i, w := range u.Windows {
			if i >= 4 {
				break
			}
			info.Rule(c, pal)
			infoMetricSub(c, pal, w.Provider, magpie.Left(w.Left), w.Name)
		}
		if u.Latest.Served != "" {
			info.Rule(c, pal)
			infoMetric(c, pal, locales.InfoMagpieLatest(), magpieCallLine(u.Latest))
		}
		if down != "" {
			info.Rule(c, pal)
			info.Empty(c, pal, down)
		}
	})
}

// infoMagpieState is Magpie's Info view, read as the card shows it.
func (a *App) infoMagpieState(c *ui.Context) *magpieInfo {
	st := a.magpieOf()
	a.magpieTick(c)
	return st
}

// infoMetric is a row of a card: a label, and a value on the right.
func infoMetric(c *ui.Context, pal *theme.Palette, label, value string) {
	ui.Row(c).MinHeight(32).Padding(0, 12).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
		infoMetricValue(c, pal, value)
	})
}

// infoMetricSub is a metric of two lines: its value over a word of what it
// counts, or a detail of it.
func infoMetricSub(c *ui.Context, pal *theme.Palette, label, value, sub string) {
	ui.Row(c).MinHeight(42).Padding(6, 12).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
		ui.Column(c).Gap(1).AlignItems(ui.End).Shrink(1).MinWidth(80).Children(func() {
			infoMetricValue(c, pal, value)
			ui.Text(c, sub).FontSize(11).TextColor(pal.TextFaint).SingleLine()
		})
	})
}

// infoMetricValue is a metric's value, on the right.
func infoMetricValue(c *ui.Context, pal *theme.Palette, value string) {
	ui.Text(c, value).FontSize(12.5).FontWeight(500).TextColor(pal.Text).FontFeatures("tnum").SingleLine().Shrink(0)
}

// magpieSessionLine is this conversation's calls, and their cost.
func magpieSessionLine(u *magpie.Usage) string {
	if u.SessErr {
		return "500+"
	}
	return strconv.Itoa(u.Session) + " · " + magpie.Cost(u.SessionCost)
}

// magpieCallLine is the latest call: its status, what served it and how
// long it took.
func magpieCallLine(l magpie.Call) string {
	line := strconv.Itoa(l.Status) + " " + l.Served
	if l.Ms > 0 {
		line += " · " + magpie.FmtTime(l.Ms)
	}
	return line
}
