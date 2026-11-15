package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/chat"
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

// readChat reads a session's chat window, as the view opens and after a
// chat's turn, and as the host's state changes.
func (a *App) readChat(s *proto.Session) {
	f := a.Feed(s.ID)
	chat.Read(a, s, f)
}

// chatChanged tells the view that a host shows a new state: the hosts'
// chat windows read their turns again, as a chat's turn ends.
func (a *App) chatChanged(h *host.Host) { chat.Changed(a, h) }

// chatBody is the view, in a session's pane.
func (a *App) chatBody(c *ui.Context, pal *theme.Palette, s *proto.Session) {
	chat.Body(a, c, pal, s)
}

var _ = locales.ChatNothing
