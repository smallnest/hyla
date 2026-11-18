package window

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/chat"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// The Chat view is a session's conversation as a chat: the prompts the
// session got, the tools its agent ran, and the answers it gave, which
// the agent keeps in its transcript, with the Feishu chat the session is
// with, whose turns the chat got. A prompt sent here goes to the
// session's pane, as a prompt for its agent. The session's host reads it
// (feishu.feed) as the view opens and as the chat's turns go on; the view
// is `internal/ui/chat`.

var _ chat.App = (*App)(nil)

// Chat is the view's state.
func (a *App) Chat() *chat.State { return &a.Side.Chat }

// Feed is a session's chat window.
func (a *App) Feed(id string) *chat.Feed {
	ch := &a.Side.Chat
	if ch.Feeds == nil {
		ch.Feeds = map[string]*chat.Feed{}
	}
	f := ch.Feeds[id]
	if f == nil {
		f = &chat.Feed{}
		ch.Feeds[id] = f
		f.Got = true // a read at once
	}
	return f
}

// toggleChat gives the pane the user is in the conversation in place of its
// terminal, or its terminal again. A pane whose agent runs without a
// terminal shows the conversation anyway, so it has nothing to toggle. The
// palette runs this: no menu has it, and no shortcut.
func (a *App) toggleChat() {
	s := a.session(a.Selected)
	if s == nil {
		return
	}
	p := s.PaneByID(a.focusOf(s))
	if p == nil || p.Browser() || p.FilePane() || p.TimelinePane() || p.SubagentsPane() || p.PluginPane() || panes.HasChat(p) {
		return
	}
	if a.PaneChatMap == nil {
		a.PaneChatMap = map[string]bool{}
	}
	a.PaneChatMap[p.ID] = !a.PaneChatMap[p.ID]
}

// readChat reads a session's chat window, as the view opens and after a
// chat's turn, and as the host's state changes.
func (a *App) readChat(s *proto.Session) {
	f := a.Feed(s.ID)
	chat.Read(a, s, a.focusOf(s), f)
}

// chatChanged tells the view that a host shows a new state: the hosts'
// chat windows read their turns again, as a chat's turn ends.
func (a *App) chatChanged(h *host.Host) { chat.Changed(a, h) }

// ChatBody is the view, in a session's pane.
func (a *App) ChatBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	chat.Body(a, c, pal, s, p.ID)
}

// SendPane sends a prompt to a pane's agent, as the composer does: typed
// into its terminal where it runs one, else a turn of the chat that drives
// it. A prompt that did not go says why, under the box it was written in.
func (a *App) SendPane(session, pane, text string) {
	h := a.hostOf(pane)
	if h == nil || h.Client == nil {
		a.sendFailed(session, text, "no host is connected for the pane")
		return
	}
	cl := h.Client
	go func() {
		var ok bool
		err := cl.Call("chat.send", proto.ChatSendParams{Session: session, Pane: pane, Text: text}, &ok)
		if err == nil && !ok {
			err = errNoPanePrompt
		}
		if err != nil {
			a.main(func() { a.sendFailed(session, text, err.Error()) })
			return
		}
		// The turn shows as it starts: the view reads the conversation
		// again at once.
		a.main(func() { a.Feed(session).Got = false })
	}()
}

// errNoPanePrompt says that a pane took no prompt: it runs no terminal, and
// no chat drives it.
var errNoPanePrompt = errors.New("this pane takes no prompt")

// sendFailed says why a prompt did not go, and gives its words back.
func (a *App) sendFailed(session, text, why string) {
	f := a.Feed(session)
	f.SendErr = why
	if strings.TrimSpace(f.Draft) == "" {
		f.Draft = text
	}
}

// StopPane stops the turn a pane runs: the program in its terminal, as its
// interrupt key does, or the chat's turn, which runs without one.
func (a *App) StopPane(pane string) {
	h := a.hostOf(pane)
	if h == nil || h.Client == nil {
		return
	}
	cl := h.Client
	go func() {
		// A pane that runs no terminal has no program to interrupt: the
		// chat's turn is the one to stop.
		if cl.Call("send", proto.SendParams{ID: pane, Data: "\x03"}, nil) != nil {
			cl.Call("agent.stop", proto.SessionParams{ID: pane}, nil)
		}
	}()
}

var _ = locales.ChatNothing
