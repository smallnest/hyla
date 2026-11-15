package quick

import (
	"log"
	"math"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// The quick terminal drops down from the top of the screen as its global
// shortcut is pressed, over every app, full-screen ones included, and goes
// back up as it is pressed again or as the user goes elsewhere, as
// Ghostty's quick terminal and Quake's console do. It shows a session of
// this machine's hylad, as the window does, so that it keeps running as it
// hides, as Hyla quits and as it updates; the sidebar lists it with the
// others.

const (
	// Action is the action that toggles the quick terminal.
	Action      = "quickTerminal.toggle"
	quickHeight = 0.4 // of the work area of the display with the pointer
	quickSlide  = 180 * time.Millisecond

	// quickErrDelay is how long the panel shows a spinner before an
	// error, so that its window comes up first.
	quickErrDelay = 150 * time.Millisecond
)

// Entry is a terminal the panel attached: the terminal, or why it did
// not.
type Entry struct {
	Term   *terminal.Terminal
	Stream *client.Stream
	Err    string
}

// State is the quick terminal's state, on the main thread.
type State struct {
	Win      *mygo.Window // the panel, made as it first shows
	Shown    bool         // shown, or sliding down
	Pane     string       // the pane its terminal shows
	E        *Entry       // that terminal, attached
	Ref      ui.Handle
	Creating bool // its session is being made
	Err      string
	// ErrAt is when the error shows: a moment after the view that first
	// has it, whose frame the panel's window does not show at once.
	ErrAt time.Time
	// Keys is the global shortcut registered, and Asked what the settings
	// asked for last, which failed when it is not Keys.
	Keys, Asked string
	Slides      atomic.Int64 // counts slides: the last one moves the panel
}

// ApplyGlobalShortcuts registers the quick terminal's shortcut as the
// settings say: none while the user types a shortcut in Settings, which
// would else toggle the terminal rather than record.
func ApplyGlobalShortcuts(a App) {
	q := a.Quick()
	want := ""
	if a.Recording() == "" {
		act, _ := actions.Find(Action)
		want = actions.Keys(act, a.Settings().Shortcuts)
	}
	if want == q.Asked {
		return
	}
	q.Asked = want
	if q.Keys != "" {
		mygo.GlobalShortcut.Unregister(q.Keys)
		q.Keys = ""
	}
	if want == "" {
		return
	}
	if err := mygo.GlobalShortcut.Register(want, func() { Toggle(a) }); err != nil {
		log.Println("hyla: quick terminal:", err)
		a.Toast(locales.ToastGlobalShortcutFailed(actions.Glyphs(want), err.Error()))
		return
	}
	q.Keys = want
}

// Toggle shows the quick terminal, or hides it.
func Toggle(a App) {
	if a.Quick().Shown {
		Hide(a)
	} else {
		Show(a)
	}
}

// quickSession returns the session the quick terminal shows, nil while it
// has none: hylad lost it, or the user closed it.
// session returns the session the quick terminal shows, nil while it
// has none: hylad lost it, or the user closed it.
func session(a App) *proto.Session {
	s := a.SessionOf(a.QuickSession())
	if s == nil || a.HostOf(s.ID) != a.LocalHost() {
		return nil
	}
	return s
}

// Show drops the quick terminal down on the display with the pointer,
// starting its session first if it has none.
func Show(a App) {
	q := a.Quick()
	if q.Win == nil {
		makeQuick(a)
	}
	q.Err, q.ErrAt = "", time.Time{}
	attachQuick(a)
	r := quickFrame(a)
	q.Shown = true
	q.Ref.Focus()
	slideQuick(a, r, r.Y-r.Height, r.Y, nil)
	q.Win.Show()
	q.Win.Focus()
	a.ReportView()
}

// PanesGap is the gap between a window's cards, SideGap the side panel's
// from them, which the panel takes as the panes' own. MinPanes is the
// narrowest the panes' column shows, a window narrower than that giving
// the panel a place of its own.
const (
	PanesGap = 8
	SideGap  = 8
	MinPanes = 360
)

// quickFrame is where the quick terminal shows: over the panes of the
// main window, as wide as they are and half its height, at their top
// left, so that it reads as the panes' own. With no window in sight, it
// drops from the top of the display with the pointer, as Ghostty's does.
func quickFrame(a App) mygo.Rectangle {
	main := a.MainWindow()
	if main != nil && !main.IsDestroyed() && main.IsVisible() && !main.IsMinimized() && !main.IsFullScreen() {
		b := main.Bounds()
		sideW, pad := int(a.Chrome().SideW), int(a.Chrome().MainPad)
		w := b.Width - sideW - pad - PanesGap
		if a.RightShown() {
			w -= int(a.SideWidth(a.SideState().RowW)) + SideGap
		}
		if w >= MinPanes {
			return mygo.Rectangle{X: b.X + sideW + pad, Y: b.Y + PanesGap, Width: w, Height: b.Height / 2}
		}
	}
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	wa := d.WorkArea
	return mygo.Rectangle{X: wa.X, Y: wa.Y, Width: wa.Width, Height: int(float64(wa.Height) * quickHeight)}
}

// Hide slides the quick terminal back up, which gives the keyboard back
// to the app the user was in.
func Hide(a App) {
	q := a.Quick()
	if !q.Shown {
		return
	}
	q.Shown = false
	r := q.Win.Bounds()
	slideQuick(a, r, r.Y, r.Y-r.Height, q.Win.Hide)
	a.ReportView()
}

// slideQuick moves the panel, as big as r, from the top edge from to to,
// then runs done: at once where the desktop asks for less motion.
func slideQuick(a App, r mygo.Rectangle, from, to int, done func()) {
	q := a.Quick()
	n := q.Slides.Add(1)
	if mygo.Theme.Preferences().ReduceMotion {
		r.Y = to
		q.Win.SetBounds(r)
		if done != nil {
			done()
		}
		return
	}
	r.Y = from
	q.Win.SetBounds(r)
	start := time.Now()
	go func() {
		tick := time.NewTicker(time.Second / 120)
		defer tick.Stop()
		for range tick.C {
			if q.Slides.Load() != n {
				return // another slide took over
			}
			p := min(float64(time.Since(start))/float64(quickSlide), 1)
			ease := 1 - math.Pow(1-p, 3)
			q.Win.SetPosition(r.X, from+int(math.Round(float64(to-from)*ease)))
			if p == 1 {
				if done != nil {
					q.Win.Update(func() {
						if q.Slides.Load() == n {
							done()
						}
					})
				}
				return
			}
		}
	}()
}

// makeQuick makes the quick terminal's panel, hidden.
func makeQuick(a App) {
	q := a.Quick()
	q.Win = mygo.NewWindow(mygo.WindowOptions{
		Title:         locales.ActionQuickTerminal(),
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		DisableResize: true,
		Width:         960,
		Height:        400,
		Content:       ui.View(func(c *ui.Context) { view(a, c) }),
	})
	// Closing it, as with ⌘W, hides it; a quit closes it with the app.
	q.Win.OnClose(func(e *mygo.CloseEvent) {
		if a.Quitting() {
			return
		}
		e.PreventDefault()
		Hide(a)
	})
	// It goes as the user goes elsewhere, as Ghostty's does.
	q.Win.OnBlur(func() { Hide(a) })
}

// attachQuick attaches the quick terminal to its session's pane, the one
// the user was in, making the session first if there is none.
func attachQuick(a App) {
	q := a.Quick()
	h := a.LocalHost()
	s := session(a)
	if s == nil {
		dropQuick(a)
		// Until hylad tells its sessions, the one of the last run may be
		// among them: the panel waits, and attaches as they come.
		if q.Creating {
			return
		}
		if h.Client == nil || h.Conn == host.StateFailed || h.Conn == host.StateVersion {
			// It shows that, with Try Again, instead of a spinner that
			// never ends. Connecting goes on to a terminal, or this.
			q.Err = h.Err
			if h.Conn != host.StateFailed {
				q.Err = locales.HostNotConnected(h.Label())
			}
			return
		}
		q.Creating = true
		cl := h.Client
		go func() {
			var s proto.Session
			err := cl.Call("create", proto.CreateParams{Dir: proto.Home(), Name: locales.ActionQuickTerminal()}, &s)
			q.Win.Update(func() {
				q.Creating = false
				if err != nil {
					q.Err = err.Error()
					return
				}
				a.AddPending(h, s)
				a.SetQuickSession(s.ID)
				a.SavePrefs()
				attachQuick(a)
				a.Update(func() {}) // the sidebar shows it
			})
		}()
		return
	}
	p := s.FocusPane()
	if p == nil || p.Browser() {
		return
	}
	if q.E != nil && q.Pane == p.ID && q.E.Err == "" {
		return
	}
	dropQuick(a)
	e := &Entry{}
	q.E, q.Pane = e, p.ID
	id := p.ID
	dialer := h.Dialer()
	go func() {
		stream, err := client.AttachVia(dialer, id)
		if err != nil {
			q.Win.Update(func() {
				if q.E == e {
					e.Err = err.Error()
				}
			})
			return
		}
		g := &Gated{Stream: stream, Ready: make(chan struct{})}
		o := a.TermOptions(id, g)
		o.ResizeByConn = true
		// No card under it: the terminal paints its background.
		o.Transparent = false
		o.Padding = terminal.Padding{X: 16, Y: 10}
		o.OnCopy = nil // the window's toast is not in sight
		term, err := terminal.New(o)
		if err != nil {
			stream.Close()
			q.Win.Update(func() {
				if q.E == e {
					e.Err = err.Error()
				}
			})
			return
		}
		term.Resize(stream.Welcome.Cols, stream.Welcome.Rows)
		stream.OnResized = term.Resize
		close(g.Ready)
		q.Win.Update(func() {
			if q.E != e {
				term.Close()
				stream.Close()
				return
			}
			e.Term, e.Stream = term, stream
			// It takes the panel's size, which another window may have
			// given another.
			go stream.Claim()
			q.Ref.Focus()
		})
		// The pane ended, as its shell exited: the next show starts
		// another, or shows the session's next pane.
		<-term.Done()
		q.Win.Update(func() {
			if q.E == e {
				dropQuick(a)
				Hide(a)
			}
		})
	}()
}

// Attach attaches the quick terminal to its session's pane.
func Attach(a App) { attachQuick(a) }

// Drop lets go of the quick terminal's terminal.
func Drop(a App) { dropQuick(a) }

func dropQuick(a App) {
	q := a.Quick()
	if q.E == nil {
		return
	}
	if q.E.Term != nil {
		q.E.Term.Close()
	}
	if q.E.Stream != nil {
		q.E.Stream.Close()
	}
	q.E, q.Pane = nil, ""
}

// Changed follows a state of hylad: the quick terminal attaches again
// when its pane is gone, as to the session's next one.
func Changed(a App) {
	q := a.Quick()
	if q.E == nil {
		if q.Shown && q.Err == "" {
			attachQuick(a)
		}
		return
	}
	s := session(a)
	if s == nil || s.PaneByID(q.Pane) == nil {
		dropQuick(a)
		if q.Shown {
			attachQuick(a)
		}
	}
}

// view is the panel's content: the terminal, or what keeps it.
func view(a App, c *ui.Context) {
	q := a.Quick()
	pal := theme.For(c)
	light, dark := a.TermThemes()
	bg := light.Background
	if c.Theme().Dark {
		bg = dark.Background
	}
	c.Root().Background(bg)
	switch {
	case q.E != nil && q.E.Term != nil:
		terminal.View(c.Key("term"), q.E.Term).Fill().Bind(&q.Ref)
	case q.Err != "" || q.E != nil && q.E.Err != "":
		if q.ErrAt.IsZero() {
			// The view came up with it: the error waits, so that the
			// panel comes up before it.
			q.ErrAt = time.Now().Add(quickErrDelay)
			c.After(quickErrDelay)
		}
		if time.Now().Before(q.ErrAt) {
			ui.Column(c).Fill().Center().Children(func() { ui.Spinner(c) })
			return
		}
		detail := q.Err
		if q.E != nil {
			detail = q.E.Err
		}
		a.PaneMessageOfQuick(c, pal, locales.PaneCantShow(), detail, locales.CommonTryAgain(), func() {
			q.Err, q.ErrAt = "", time.Time{}
			dropQuick(a)
			attachQuick(a)
		})
	default:
		ui.Column(c).Fill().Center().Children(func() { ui.Spinner(c) })
	}
}

// TermFor returns the quick terminal's terminal when it shows a pane,
// else nil.
func TermFor(a App, id string) *terminal.Terminal {
	if q := a.Quick(); q.Pane == id && q.E != nil {
		return q.E.Term
	}
	return nil
}
