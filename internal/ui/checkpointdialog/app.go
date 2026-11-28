package checkpointdialog

import (
	"hyla/internal/app/host"
	"hyla/internal/proto"
)

// App is what the Time-Travel Checkpoints dialog asks of the app window.
type App interface {
	SelectedHost() *host.Host
	SelectedSession() *proto.Session
	CheckpointAskNow() *Ask
	SetCheckpointAsk(d *Ask)
	SelectSession(id string)
	Toast(msg string)
	Main(fn func())
	FocusTerminal()
}
