package window

import (
	"time"

	"hyla/internal/app/host"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/theme"
)

const paneGap = 8 // between panes, where the divider is

// sideGap is the visual space between the panes and the side panel: the
// panes' end margin plus the panel's left margin, half a pane gap on
// either side.
const sideGap = paneGap / 2

// paneRef returns the handle a pane's terminal binds, which its focus
// follows.
func (a *App) paneRef(id string) *ui.Handle {
	if a.Refs == nil {
		a.Refs = map[string]*ui.Handle{}
	}
	h := a.Refs[id]
	if h == nil {
		h = &ui.Handle{}
		a.Refs[id] = h
	}
	return h
}

// focusOf returns the pane of a session the user is in.
func (a *App) focusOf(s *proto.Session) string {
	if id := a.Focus[s.ID]; id != "" && s.PaneByID(id) != nil {
		return id
	}
	if p := s.FocusPane(); p != nil {
		return p.ID
	}
	return ""
}

// shownPanes returns the panes of a session the window shows, as panes
// lays them out: the zoomed one alone, or the one the user is in without
// a layout.
func shownPanes(s *proto.Session) []string {
	p := s.PaneByID(s.Zoomed)
	if p == nil && s.Layout != nil && len(s.Panes) > 1 {
		ids := make([]string, len(s.Panes))
		for i, p := range s.Panes {
			ids[i] = p.ID
		}
		return ids
	}
	if p == nil {
		p = s.FocusPane()
	}
	if p == nil {
		return nil
	}
	return []string{p.ID}
}

// setFocus moves the user into a pane, and tells hylad, which remembers it.
func (a *App) setFocus(s *proto.Session, pane string) {
	if a.Focus == nil {
		a.Focus = map[string]string{}
	}
	if a.Focus[s.ID] == pane && s.Focus == pane {
		return
	}
	a.Focus[s.ID] = pane
	s.Focus = pane
	if s.Zoomed != "" {
		s.Zoomed = pane
	}
	a.EnsureTerm(pane)
	a.Touch(pane)
	a.Call(s.ID, "focus", proto.FocusParams{Session: s.ID, Pane: pane}, nil)
}

// focusTerminal gives the keyboard to the pane the user is in.
func (a *App) focusTerminal() {
	if s := a.session(a.Selected); s != nil {
		if id := a.focusOf(s); id != "" {
			a.paneRef(id).Focus()
		}
		a.claimShown()
	}
}

// panes shows a session's panes in their layout: the zoomed one alone.
// right tells that they reach the window's right edge, which the side
// panel takes while it shows.
func (a *App) PanesIn(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, right bool) {
	// A session shown over another, as when the user moves from one to the
	// next, comes in as panes says, and its panes move to their place in
	// the frames that come after.
	switched := false
	if a.ShownSessionID != "" && a.ShownSessionID != s.ID {
		a.switchTo(c)
		switched = true
	}
	a.settle()
	a.ShownSessionID = s.ID
	sw := panes.Switch(a.settingsIn.SessionSwitch)
	slot := ui.Box(c.Key("slot-" + s.ID)).Absolute().Left(0).Top(0).Right(0).Bottom(0)
	// The frames that move a session of the view build it, as those of a
	// Transition: frames that only paint would show the panes back at
	// their place, with a copy of their Transition.
	if a.SwitchInState {
		c.AnimationFrame()
	}
	slot.AnimatePose(panes.Pose(sw, a.SwitchFromMotion, a.SwitchInState), switchSpring)
	// A session that a switch brings in shows as a new element, where it
	// comes, as one built for the first time does, and moves from there as
	// poses do: one that the frames build before has the place its poses
	// had, and would jump there.
	from := ui.Motion{}
	if switched {
		from = a.SwitchFromMotion
	}
	slot.Children(func() {
		show := ui.Box(c.Key("show-" + s.ID)).Absolute().Left(0).Top(0).Right(0).Bottom(0)
		show.AnimatePose(panes.Pose(sw, from, true), switchSpring)
		show.Children(func() {
			// The frames' wrapper, which the session that leaves goes away
			// with, in the dissolve: shown in full.
			view := ui.Box(c.Key("session-view")).Absolute().Left(0).Top(0).Right(0).Bottom(0)
			view.Transition(panes.Transition(sw))
			view.Children(func() { a.sessionPanes(c, pal, bar, shown, s, right) })
		})
	})
}

// switchTo tells that the frame about to build shows a session in place
// of the one before (shownSession): the session comes in as panes says,
// at once, from switchFrom, and shows at its place in the next build.
func (a *App) switchTo(c *ui.Context) {
	sw := panes.Switch(a.settingsIn.SessionSwitch)
	a.SwitchInState = false
	if sw == panes.SwitchNone {
		return
	}
	a.SwitchFromMotion, a.SwitchInState = panes.Motion(sw), true
	a.SwitchAtTime = c.Now()
	c.After(switchLand)
}

// switchSpring moves a session as a slide show's page moves: a spring
// that arrives slowly, settling without a bounce, so that the motion
// shows, as a page's slide does.
var switchSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond}}

// switchLand is how long a switch moves before a frame built for another
// reason, as a key's, shows its session at its place.
const switchLand = 400 * time.Millisecond

// settle lands a switch once its frames go on: a frame built later, for
// what the user asks, as a key, shows the panes at their place, and not
// back where they came from.
func (a *App) settle() {
	if !a.SwitchInState || time.Since(a.SwitchAtTime) < switchLand {
		return
	}
	a.SwitchInState, a.SwitchFromMotion = false, ui.Motion{}
}

// sessionPanes lays out a session's panes: the zoomed one alone, else
// their layout.
func (a *App) sessionPanes(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, right bool) {
	// One pane shows its chat in place of its terminal, alone, so that
	// the chat has the room side by side would leave it.
	if len(s.Zoomed) == 0 || s.Zoomed == a.focusOf(s) {
		if p0 := s.PaneByID(a.focusOf(s)); p0 != nil && a.PaneChatMap[p0.ID] && a.FeishuOn() {
			panes.PaneCard(a, c, pal, bar, shown, s, p0, true, true, right, false)
			return
		}
	}
	multi := len(s.Panes) > 1
	if z := s.PaneByID(s.Zoomed); z != nil || s.Layout == nil || !multi {
		p := z
		if p == nil {
			p = s.FocusPane()
		}
		if p != nil {
			panes.PaneCard(a, c, pal, bar, shown, s, p, true, true, right, multi)
		}
		return
	}
	panes.LayoutNode(a, c, pal, bar, shown, s, s.Layout, "", true, true, right)
}

// ReplaceSession puts a session as hylad answered into the state, before
// the state that has it comes.
func (a *App) ReplaceSession(h *host.Host, ns proto.Session) {
	for i := range a.St.Sessions {
		if a.St.Sessions[i].ID == ns.ID {
			a.St.Sessions[i] = ns
			return
		}
	}
	a.addPending(h, ns)
}

// RunLater runs fn once the view is built: for actions that change the
// panes or sessions being built.
func (a *App) RunLater(fn func()) { a.Later = append(a.Later, fn) }

// closeFocused closes the pane the user is in, or the session with its
// last pane.
func (a *App) closeFocused() {
	if s := a.session(a.Selected); s != nil {
		a.closePane(s, a.focusOf(s), true)
	}
}
