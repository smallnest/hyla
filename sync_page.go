package main

import (
	"github.com/egoist/mygo/ui"

	configsync "hyla/internal/app/sync"
	"hyla/internal/ui/syncedit"
	"hyla/internal/ui/theme"
)

// The app, as the Sync page asks it.
var _ syncedit.App = (*App)(nil)

func (a *App) SyncState() *configsync.State { return &a.State }

// SyncSaved keeps the Sync page's changes, and starts the push they wait
// for.
func (a *App) SyncSaved() { a.State.Save(a) }

// SyncRun syncs this machine's configuration now.
func (a *App) SyncRun(way string) { a.State.Run(a, way) }

func (a *App) syncPage(c *ui.Context, pal *theme.Palette) { syncedit.Page(a, c, pal) }

func (a *App) syncLine() string { return syncedit.Line(&a.State) }
