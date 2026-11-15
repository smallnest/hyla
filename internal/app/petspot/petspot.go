// Package petspot places Hyla the frog, and the Codex pets, on the
// screen: above the main window's top edge where the screen leaves room,
// else inside the window at its bottom edge.
package petspot

import (
	"math"

	"github.com/egoist/mygo"

	"hyla/internal/pets"
)

const (
	// W and H are the pet window's width and height: wide enough for a
	// sentence, tall enough for its speech above the pet.
	W = 190
	H = 118
	// BodyH is the height of the frog's body, and Perch how far its hands
	// reach over the window's top edge.
	BodyH = 87
	Perch = 12
)

// Room is where the pet's speech starts, and how tall its room is, for
// a pet whose hands are over the edge at perch, in a screen whose work
// area starts at room: the room runs from the top of its speech to the
// frame's feet, where a Codex pet's stand, setH below the edge. The speech
// gives way where the screen leaves little room for it, then the frog's
// frame goes while it still fits.
func Room(perch, room, setH float32) (y, height float32) {
	// What it says, from its top to its hands, never to above the
	// screen, and never taller than the frog's frame.
	speech := float32(H)
	if setH < speech {
		speech = setH
	}
	if perch-room < speech {
		speech = perch - room
	}
	// The frog's frame, where the edge is too high for a Codex pet, still
	// fits down to where the screen ends.
	cell := speech
	if foot := room + setH; BodyH < foot && foot < perch && foot < cell {
		cell = foot
	}
	return perch - cell, speech
}

// Spot is where the pet's window goes for the main window at win, in a
// screen's work area: above the window's top edge where the screen leaves
// room for it, hands over the edge, else standing inside the window at its
// bottom edge. offset is where across the window its middle stands, 0 at
// the left to 1 at the right. setH is the room a Codex pet needs above the
// edge, and head how much of that room its frame leaves empty above its
// art; inside tells that it stands inside the window.
func Spot(win, work mygo.Rectangle, offset float32, setH, head float32) (mygo.Rectangle, bool) {
	// A Codex pet as wide as a frog's window might be wider.
	wide := float32(W)
	if setH > BodyH {
		wide = float32(math.Round(float64(setH) * pets.FrameW / pets.FrameH))
	}
	off := offset
	if off < 0 {
		off = 0
	}
	if off > 1 {
		off = 1
	}
	top := float32(win.Y)
	winWide := float32(win.Width)
	room := float32(work.Y)
	x := float32(win.X) + off*winWide - wide/2
	if x < float32(work.X)+8 {
		x = float32(work.X) + 8
	}
	if x > float32(work.X)+float32(work.Width)-wide-8 {
		x = float32(work.X) + float32(work.Width) - wide - 8
	}
	perch := top + Perch
	y, height := Room(perch, room, setH)
	// A Codex pet taller than the frog takes the whole of its window, so
	// that all of it shows above the edge.
	if setH+Perch >= H {
		y, height = perch-setH, setH+Perch
	}
	// No room above the edge for what the pet shows: it stands inside the
	// window at its bottom edge, within the window. The room the frame
	// leaves empty above its art goes under the screen with it.
	inside := perch-setH+head < room
	if inside {
		if x < float32(win.X)+8 {
			x = float32(win.X) + 8
		}
		if x > float32(win.X)+winWide-wide-8 {
			x = float32(win.X) + winWide - wide - 8
		}
		y, height = float32(win.Y+win.Height-H), float32(H)
		// A Codex pet taller than the frog needs the top of its window.
		if setH+Perch >= H {
			// And it stands at the window's right rather than out of it.
			y, height = float32(win.Y+win.Height)-setH-Perch, setH+Perch
			if x > float32(win.X)+winWide-wide-8 {
				x = float32(win.X) + winWide - wide - 8
			}
			if x < float32(win.X)+8 {
				x = float32(win.X) + 8
			}
		}
	}
	// The screen keeps a window's top from going above the menu bar: the
	// frame's empty room above the art is what gives way, its window
	// starting there rather than the pet sinking with it.
	if !inside && y < room {
		height -= room - y
		y = room
	}
	return mygo.Rectangle{X: int(math.Round(float64(x))), Y: int(math.Round(float64(y))), Width: int(math.Round(float64(wide))), Height: int(math.Round(float64(height)))}, inside
}

// OffsetAt is where the middle of a pet placed at left stands across
// a main window, from 0 at its left edge to 1 at its right.
func OffsetAt(main mygo.Rectangle, left float32) float32 {
	if main.Width <= 0 {
		return 0.5
	}
	center := left + W/2
	return min(max((center-float32(main.X))/float32(main.Width), 0), 1)
}
