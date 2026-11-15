package main

import (
	"time"

	"github.com/egoist/mygo"

	"hyla/internal/proto"
	"hyla/internal/ui/petme"
)

// The pet lives in a small, transparent window of its own, perched on the
// main window's top edge, and follows the sessions (internal/ui/petme).
// By default it is Hyla the frog: it peeks over the edge, its hands on the
// title bar, hops when a session needs the user, scans left and right
// while an agent works, cheers a finish, shakes at a failure, and dozes
// when everything is calm. Settings › Appearance shows a Codex pet there
// instead, as a spritesheet of frames. A click pets it, and takes the user
// to the session that needs them.

// The app, as the pet asks it.
var _ petme.App = (*App)(nil)

// The pet's files, as the app's: syncPet and its events, and the settings
// pages' calls, go to internal/ui/petme, whose App this is.

func (a *App) syncPet() { petme.Sync(a) }

func (a *App) petShow() { petme.Show(a) }

func (a *App) petClose() { petme.Close(a) }

func (a *App) petPlace() { petme.Place(a) }

func (a *App) petRaise() { petme.Raise(a) }

func (a *App) petGrow() { petme.GrowRead(a) }

func (a *App) petNote(at time.Time) { petme.Note(a, at) }

// petSetReload reads the pet again, as settings changed, or a pack
// installed, and so the pets the settings may show.
func (a *App) petSetReload() { petme.Reload(a) }

// The app, as the pet asks it.
func (a *App) MainWindow() *mygo.Window { return a.Win }

func (a *App) Sessions() []proto.Session { return a.St.Sessions }

func (a *App) Focused() bool { return a.focused }

func (a *App) PetSet() *petme.View { return a.PetSetView }

func (a *App) SetPetSet(v *petme.View) { a.PetSetView = v }

func (a *App) PetState() *petme.State { return &a.Pet }

func (a *App) PetListOf() *petme.List { return &a.PetList }

func (a *App) Generation() int { return a.petsGen }

func (a *App) BumpGeneration() { a.petsGen++ }

func (a *App) JumpToAttention() { a.jumpToAttention() }

// setFocused tells that the app is in front, or leaves.
func (a *App) setFocused(v bool) { a.focused = v }

// SavePetSettings applies the settings, as a drag of the pet's edge
// leaves them.
func (a *App) SavePetSettings() { a.applySettings(a.settingsIn) }
