package petme

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/petspot"
	"hyla/internal/proto"

	"hyla/locales"
)

// W is the pet window's width and H its height: wide enough for a
// sentence, tall enough for its speech above the pet. BubbleH is the
// height of its speech's bubble. Top is the room above the pet's body in
// the window: petspot's body leaves it, for the speech.
const (
	W       = petspot.W
	H       = petspot.H
	BubbleH = 26
	Top     = H - petspot.BodyH
)

// window is the app's window, as the pet's window follows it.
type window interface {
	IsDestroyed() bool
	IsVisible() bool
	IsMinimized() bool
	IsFullScreen() bool
	Bounds() mygo.Rectangle
	Focus()
	// OnMove, OnResize, OnFocus, OnShow, OnHide, OnMinimize, OnRestore,
	// OnEnterFullScreen and OnLeaveFullScreen keep the pet where the
	// window is.
	OnMove(fn func()) func()
	OnResize(fn func()) func()
	OnFocus(fn func()) func()
	OnShow(fn func()) func()
	OnHide(fn func()) func()
	OnMinimize(fn func()) func()
	OnRestore(fn func()) func()
	OnEnterFullScreen(fn func()) func()
	OnLeaveFullScreen(fn func()) func()
}

// Window is the pet's window: its speech, when it has something to
// say, over the pet.
func Window(a App) func(c *ui.Context) { return func(c *ui.Context) { view(a, c) } }

// Sync shows or hides the pet, as the settings and the window say.
func Sync(a App) {
	win := a.MainWindow()
	if win == nil || win.IsDestroyed() {
		return
	}
	switch {
	case a.Settings().Pet && a.PetState().Win == nil:
		Show(a)
	case !a.Settings().Pet && a.PetState().Win != nil:
		Close(a)
	}
}

// Show opens the pet's window over the main one, and keeps it there.
func Show(a App) {
	st := a.PetState()
	now := time.Now()
	st.Since, st.LastActive, st.Wink = now, now, time.Time{}
	st.Calm = mygo.Theme.Preferences().ReduceMotion
	st.Mood, _ = MoodNow(a.Sessions(), st.LastActive, now)
	st.SetAgain = false
	Reload(a)
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:             locales.PetName(),
		Width:             W,
		Height:            H,
		Frameless:         true,
		Transparent:       true,
		DisableShadow:     true,
		DisableResize:     true,
		DisableMove:       true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableClose:      true,
		DisableFullScreen: true,
		SkipTaskbar:       true,
		AlwaysOnTop:       true,
		Hidden:            true,
		Parent:            a.MainWindow(),
		Content:           ui.View(Window(a)),
	})
	st.Win = win
	main := a.MainWindow()
	st.Offs = []func(){
		// Closed with the app, and never in the way of quitting.
		win.OnClosed(func() {
			if st.Win == win {
				st.Win = nil
			}
			a.SetPetSet(nil)
		}),
		main.OnMove(func() { Place(a) }),
		main.OnResize(func() { Place(a) }),
		// A window ordered in, as the app comes to the front, goes over
		// its children: the pet comes above it again.
		main.OnFocus(func() { Raise(a) }),
		main.OnShow(func() { Place(a) }),
		main.OnHide(win.Hide),
		main.OnMinimize(win.Hide),
		main.OnRestore(func() { Place(a) }),
		main.OnEnterFullScreen(win.Hide),
		main.OnLeaveFullScreen(func() { Place(a) }),
		mygo.Screen.OnDisplaysChanged(func() { Place(a) }),
	}
	// The pet shows while this app is in front, above the window's kind
	// of window, and goes as another app comes forward, as the window's
	// edge does: a child window's ordering left it behind the window.
	st.AppOffs = []func(){
		mygo.App.OnActivate(func(bool) { Raise(a) }),
		mygo.App.OnDidResignActive(func() {
			if st.Win != nil {
				st.Win.Hide()
			}
		}),
	}
	Place(a)
}

// Close takes the pet's window away.
func Close(a App) {
	st := a.PetState()
	st.Dragging, st.Moved = false, false
	for _, off := range st.Offs {
		off()
	}
	st.Offs = nil
	for _, off := range st.AppOffs {
		off()
	}
	st.AppOffs = nil
	if st.Win != nil {
		win := st.Win
		st.Win = nil
		win.Destroy()
	}
}

// Place puts the pet's window on the main window's edge: above its top
// edge where the screen leaves room, else standing inside at its bottom
// edge, as ELY does. It hides the pet while the window is away.
func Place(a App) {
	st := a.PetState()
	main := a.MainWindow()
	if st.Win == nil || main == nil || main.IsDestroyed() {
		return
	}
	if !main.IsVisible() || main.IsMinimized() || main.IsFullScreen() {
		st.Win.Hide()
		return
	}
	b := main.Bounds()
	d := mygo.Screen.DisplayNearestPoint(mygo.Point{X: b.X + b.Width, Y: b.Y})
	r, _ := petspot.Spot(b, d.WorkArea, a.Settings().PetOffset, BodyH(a), Head(a))
	was := st.Win.Bounds()
	if was != r {
		st.Win.SetBounds(r)
		if was.Width != r.Width || was.Height != r.Height {
			// The build tells how much room there is for its speech.
			st.Win.Invalidate()
		}
	}
	// Shown, or shown again: a drag of the main window, or its ordering,
	// leaves the pet behind it, where its bottom part goes under the
	// window; ShowInactive orders it above its parent as it places it.
	st.Win.ShowInactive()
}

// Raise puts the pet above the main window again, whose ordering takes it
// over its children: as the window takes the keyboard, and as the app
// comes to the front.
func Raise(a App) {
	st := a.PetState()
	main := a.MainWindow()
	if st.Win == nil || main == nil || main.IsDestroyed() {
		return
	}
	if !main.IsVisible() || main.IsMinimized() || main.IsFullScreen() {
		return
	}
	st.Win.ShowInactive()
}

// GrowRead puts the pet at its size, and where it stands now, on the next
// frame: a slider dragged asks for this, rather than the settings
// applied, and saved, on each frame of the drag.
func GrowRead(a App) {
	if a.SettingsWindow() != nil {
		Grow(a)
	}
	Place(a)
}

// Note takes the sessions as they just changed: a mood that changes
// starts its reaction, and work moving keeps the pet awake. The daemon
// sends a whole state, so this is where the pet hears of it.
func Note(a App, at time.Time) {
	st := a.PetState()
	if st.Win == nil {
		return
	}
	m, _ := MoodNow(a.Sessions(), st.LastActive, at)
	if m != Idle && m != Asleep {
		st.LastActive = at
	}
	if m != st.Mood {
		st.Mood, st.Since = m, at
	}
	st.Win.Invalidate()
}

// Petted is a click on the pet: it wakes, winks, or waves as a Codex pet,
// and takes the user to a session that waits on them, if another one
// does.
func Petted(a App, now time.Time, mood Mood) {
	st := a.PetState()
	st.LastActive, st.Since, st.Mood = now, now, mood
	st.Wink, st.Wave = now, now
	if mood == Waiting {
		if s := a.SelectedSession(); s == nil || s.Status != proto.Waiting {
			a.JumpToAttention()
		}
	}
	if st.Win != nil {
		st.Win.Invalidate()
	}
	if main := a.MainWindow(); main != nil {
		main.Focus()
		a.FocusTerminal()
	}
}

// Drag moves the pet along its edge while the pointer drags its body. The
// pointer's screen x, not its x in the window, sets where it goes: the
// window follows it, so the local x would stand still.
func Drag(a App, body ui.Element) {
	st := a.PetState()
	_, _, pressed := body.Dragged()
	if pressed {
		if !st.Dragging {
			x, _, over := body.PointerPosition()
			if !over {
				return
			}
			st.Dragging, st.Moved, st.Grab = true, false, x
			st.LastActive = time.Now()
			if cx, ok := cursorX(a); ok {
				st.StartX = cx
			} else {
				st.StartX = 0
			}
		}
		if cx, ok := cursorX(a); ok {
			if abs(cx-st.StartX) > 2 {
				st.Moved = true
			}
			if st.Moved {
				dragTo(a, cx-st.Grab)
			}
		}
		return
	}
	if st.Dragging {
		st.Dragging = false
		if st.Moved {
			Drop(a)
		}
	}
}

// cursorX is the pointer's x on screen, where the pet's window moves.
func cursorX(a App) (float32, bool) {
	if a.PetState().Win == nil || a.MainWindow() == nil {
		return 0, false
	}
	return float32(mygo.Screen.CursorScreenPoint().X), true
}

// dragTo puts the pet's window at left, as a custom position across the
// main window, and moves it there.
func dragTo(a App, left float32) {
	st := a.PetState()
	main := a.MainWindow()
	if st.Win == nil || main == nil {
		return
	}
	off := petspot.OffsetAt(main.Bounds(), left)
	off = float32(math.Round(float64(off)*1000)) / 1000
	a.Settings().PetOffset = off
	Place(a)
}

// Drop keeps where a drag left the pet.
func Drop(a App) {
	if a.PetState().Win == nil {
		return
	}
	a.SavePetSettings()
}

// abs is a float's magnitude.
func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
