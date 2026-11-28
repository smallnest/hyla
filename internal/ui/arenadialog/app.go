package arenadialog

import (
	"hyla/internal/app/host"
	"hyla/internal/proto"
)

// App is what the Multi-Agent Arena dialog asks of the app window.
type App interface {
	SelectedHost() *host.Host
	SelectedSession() *proto.Session
	ArenaAskNow() *Ask
	SetArenaAsk(d *Ask)
	SelectSession(id string)
	Toast(msg string)
	Main(fn func())
	FocusTerminal()
}
