package petme

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/pets"
	"hyla/internal/proto"
	"hyla/internal/settings"

	"hyla/locales"
)

// App is what the pet asks of the app it follows.
type App interface {
	// Settings are the settings the pet follows, and the pet set the one
	// it shows where it is no frog.
	Settings() *settings.Settings
	PetSet() *View
	SetPetSet(v *View)
	// PetState is the pet's window and what it last felt.
	PetState() *State
	// Update runs fn on the main thread, Toast tells a failure, and
	// Generation counts the pets installed or removed, which a list and
	// the pet's window read again.
	Update(fn func())
	Toast(msg string)
	Generation() int
	BumpGeneration()
	// List is the pets the settings may show, as last read.
	PetListOf() *List
	// SelectedSession is the session the window shows, and Focused tells
	// that the app is in front.
	SelectedSession() *proto.Session
	Focused() bool
	// Sessions are the sessions the app knows, MainWindow the window the
	// pet sits on, and SettingsWindow the Settings window, while open.
	Sessions() []proto.Session
	MainWindow() *mygo.Window
	SettingsWindow() *mygo.Window
	// JumpToAttention takes the user to a session that waits on them,
	// FocusTerminal gives the keyboard to the pane the user is in, and
	// ApplySettings applies the settings, as a drag of the edge leaves them.
	JumpToAttention()
	FocusTerminal()
	SavePetSettings()
}

// Pet sets are the pets on the window's edge beside Hyla, the tree frog:
// Codex pets' spritesheets, which the pet shows frame by frame for each
// mood, where Hyla, the tree frog is drawn (draw.go).

const (
	setFrame  = 150 * time.Millisecond // the Codex app's pace, 6⅔ frames a second
	setCalm   = time.Second            // with less motion
	setFrame0 = 82                     // how tall the Codex app shows a frame, in DIPs, its feet on the edge
)

// Ground is how tall a Codex pet's frame shows, in DIPs, from its size
// setting: feet on the edge.
func Ground(a App) float32 {
	n := float64(setFrame0) * float64(a.Settings().PetSize)
	return float32(math.Round(n))
}

// Wide is how wide a Codex pet's frame shows, in DIPs.
func Wide(a App) float32 {
	n := float64(Ground(a)) * pets.FrameW / pets.FrameH
	return float32(math.Round(n))
}

// BodyH is how tall the room for a Codex pet is: its frame, and a little,
// where it stands.
func BodyH(a App) float32 { return Ground(a) + 8 }

// List is the pets the settings may show, read for the pet chosen in
// For, as Generation counted installations.
type List struct {
	Pets []pets.Builtin
	For  string
	Gen  int
}

// View is the pet hyla shows, where it is no frog.
type View struct {
	name string // the setting's name, which finds the pet again
	pet  *pets.Pet
}

// Load reads the pet the settings show, if it is no frog, and says when
// it is not there, for the pet shows Hyla, the tree frog then.
func Load(a App) *View {
	if a.Settings().PetSet == settings.PetSetHyla {
		return nil
	}
	p, err := pets.LoadID(a.Settings().PetSet)
	if err != nil {
		if !a.PetState().SetAgain {
			// Once a window: the pet's view asks every frame.
			a.PetState().SetAgain = true
			a.Toast(locales.SetPetMissing(a.Settings().PetSet, err.Error()))
		}
		return nil
	}
	return &View{name: a.Settings().PetSet, pet: p}
}

// Reload reads the pet again, as settings changed, or a pack installed,
// and so the pets the settings may show.
func Reload(a App) {
	listAgain(a)
	a.BumpGeneration()
	a.SetPetSet(Load(a))
	invalidate(a)
}

// Grow reads the pet again at its new size, while the settings keep the
// old one: no list of pets reads again until they do.
func Grow(a App) {
	a.SetPetSet(Load(a))
	invalidate(a)
}

func invalidate(a App) {
	if st := a.PetState(); st.Win != nil {
		st.Win.Invalidate()
	}
}

// listTick is the timer that reads the pets the settings may show again,
// for a pet installed from the CLI, without settings changed.
var listTick *time.Timer

// listAgain reads the pets the settings may show again, once in a while:
// until then, a list reads them once.
func listAgain(a App) {
	if listTick != nil {
		listTick.Stop()
	}
	listTick = time.AfterFunc(1500*time.Millisecond, func() { a.Update(func() { a.BumpGeneration() }) })
}

// Head is how much of the room a Codex pet takes above the edge its frame
// leaves empty above its art: the pet shows whole there while its frame's
// top is under the screen's.
func Head(a App) float32 {
	set := a.PetSet()
	if set == nil || set.pet.Top == 0 {
		return 0
	}
	return float32(set.pet.Top) * Ground(a) / pets.FrameH
}

// state is the spritesheet row of each mood: an agent that works is at
// its desk, one that waits asks for you, a failure fails, and a finish
// jumps. A pet at rest, or asleep, is idle: it rests as it always does.
func state(mood Mood) pets.State {
	switch mood {
	case Working:
		return pets.Working
	case Waiting:
		return pets.Waiting
	case Done:
		return pets.Jump
	case Failed:
		return pets.Fail
	}
	return pets.Idle
}

// StateAt is the row the pet shows now, and when it began: the wave it
// answers a pat with, for Pat, then its mood's row.
func StateAt(a App, mood Mood, now time.Time) (pets.State, time.Time) {
	st := a.PetState()
	if now.Sub(st.Wave) < Pat {
		return pets.Wave, st.Wave
	}
	return state(mood), st.Since
}

// blinks is how often a pet at rest blinks, its idle row's last frames,
// and blink how long the blink shows.
const (
	blinks = 5 * time.Second
	blink  = 300 * time.Millisecond
)

// FrameAt is the frame of a state at now, which turns over since.
func FrameAt(a App, state pets.State, since, now time.Time) int {
	if state == pets.Idle {
		// The idle row plays from its first frame, and now and then its
		// last ones close its eyes.
		if rest := now.Sub(since) % blinks; rest < blink {
			if frames := pets.Frames[pets.Idle]; frames > 1 {
				return frames - 1
			}
		}
	}
	period := setFrame
	if a.PetState().Calm {
		period = setCalm
	}
	n := max(pets.Frames[state], 1)
	return int((max(now.Sub(since), 0) / period) % time.Duration(n))
}

// Shows is the pet the window shows, read once.
func Shows(a App) *View {
	if a.PetSet() == nil {
		a.SetPetSet(Load(a))
	}
	return a.PetSet()
}

// DrawView draws a Codex pet into its box, feet on the window's edge, as
// the Codex app draws it: a frame of the sheet, the row of its mood, cut
// out and shown a pixel art pixel for a few of the screen's.
func DrawView(a App, p *ui.Painter, r ui.Rect, set *View, state pets.State, frame int) {
	src := set.pet.Frame(state, frame)
	h := Ground(a)
	w := h * pets.FrameW / pets.FrameH
	x, y := r.X+(r.W-w)/2, r.Y+r.H-h
	// Its shadow, as much taller as it is.
	k := h / setFrame0
	p.FillPath(ellipse(r.X+r.W/2, r.Y+r.H-1.5*k, w*0.38, 5*k), ui.RGBA(15, 25, 20, 0.13))
	f := ui.Rect{X: float32(src.Min.X), Y: float32(src.Min.Y), W: pets.FrameW, H: pets.FrameH}
	p.ImageRegion(set.pet.Sheet, f, ui.Rect{X: x, Y: y, W: w, H: h}, ui.ImageOptions{})
}
