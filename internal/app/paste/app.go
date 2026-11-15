package paste

import (
	"github.com/egoist/mygo/plugins/terminal"

	"hyla/internal/proto"
)

// App is what a paste asks of the app it pastes into.
type App interface {
	// TermFor is a pane's terminal, nil while it attaches none.
	TermFor(id string) *terminal.Terminal
	// Sessions are the sessions the app knows.
	Sessions() []proto.Session
	// Session is a session the app knows, nil for none.
	Session(id string) *proto.Session
	// Toast tells what a paste could not do.
	Toast(msg string)
}
