package mqdialog

import (
	"hyla/internal/app/host"
	"hyla/internal/proto"
)

// App is what the Local Merge Queue dialog asks of the app window.
type App interface {
	SelectedHost() *host.Host
	SelectedSession() *proto.Session
	MQAskNow() *Ask
	SetMQAsk(d *Ask)
	SelectSession(id string)
	Toast(msg string)
	Main(fn func())
	FocusTerminal()
}
