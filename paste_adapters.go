package main

import (
	"github.com/egoist/mygo/transfer"

	"hyla/internal/app/paste"
)

// The app, as a paste asks it.
var _ paste.App = (*App)(nil)

// pasteImage takes a paste of the clipboard's image as the path of a file
// with it.
func (a *App) pasteImage(id string, d transfer.Data) bool { return paste.Image(a, id, d) }

// pasteShell is the name of the shell of a pane, or of the pane in front
// of a session.
func (a *App) pasteShell(id string) string { return paste.PaneShell(a, id) }
