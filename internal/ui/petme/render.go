package petme

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/petspot"
	"hyla/internal/pets"
	"hyla/internal/proto"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// waitingNow tells whether the session the window shows waits on the
// user: the pet's hop softens then.
func waitingNow(a App) bool {
	s := a.SelectedSession()
	return s != nil && s.Status == proto.Waiting
}

// tick is how often the pet draws its next frame: slowly when the
// desktop asks for less motion, and when the user is elsewhere.
func tick(a App) time.Duration {
	switch {
	case a.PetState().Calm:
		return 250 * time.Millisecond
	case !a.Focused():
		return Calm
	}
	return Frame
}

// render is the pet's window's content, built each frame.
// petView is the pet's window: its speech, when it has something to say,
// over the pet.
func view(a App, c *ui.Context) {
	c.Root().Background(ui.Transparent)
	pal := theme.For(c)
	now := c.Now()
	mood, n := MoodNow(a.Sessions(), a.PetState().LastActive, now)
	label, fill, fg := speech(a, pal, mood, n)
	// The window is shorter than the pet and its speech when the screen
	// leaves little room above the main window: the speech gives way, so
	// that no half a bubble shows, and the pet shows whole.
	_, h := c.Size()
	set := Shows(a)
	bodyH := float32(petspot.BodyH)
	if set != nil {
		bodyH = BodyH(a)
	}
	roomy := h >= bodyH+Top-1
	showing := roomy && Says(now, a.PetState().Since, mood)
	var body ui.Element
	clicked := false
	ui.Column(c).Fill().Justify(ui.End).AlignItems(ui.Center).Children(func() {
		if roomy {
			speech := ui.Column(c.Key("speech")).AlignItems(ui.Center)
			speech.Opacity(speech.Animate("shown", b2f(showing), 180*time.Millisecond))
			speech.Children(func() {
				bubble := ui.Row(c.Key("bubble")).Height(BubbleH).Padding(0, 11).Radius(BubbleH / 2).Gap(6).
					AlignItems(ui.Center).Background(fill).MaxWidth(petspot.W - 6).Shrink(1)
				if mood == Working {
					bubble.Shadow(0, 2, 10, 0, pal.FloatShadow).Border(0.5, pal.CardBorder)
				}
				bubble.Children(func() {
					if mood == Working && showing {
						ui.Spinner(c).Size(11, 11).TextColor(fg)
					}
					ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(fg).SingleLine().Ellipsis("…")
				})
				ui.Box(c.Key("tail")).Width(12).Height(5).Draw(func(p *ui.Painter, r ui.Rect) {
					tail := (&ui.Path{}).MoveTo(r.X, r.Y).LineTo(r.X+r.W, r.Y).LineTo(r.X+r.W/2, r.Y+r.H).Close()
					p.FillPath(tail, fill)
				})
				clicked = clicked || (showing && bubble.Clicked())
			})
		}

		name := locales.PetName()
		if set != nil {
			name = set.pet.Name
		}
		// Its width keeps its frame's proportions, at any size.
		wide := float32(petspot.W)
		if set != nil {
			wide = Wide(a)
		}
		body = ui.Box(c.Key("hyla")).Size(wide, bodyH).Shrink(0).Cursor(ui.CursorGrab).
			Description(petDescription(name, label))
		hovered := body.Hovered()
		state, since := pets.Idle, a.PetState().Since
		if set != nil {
			state, since = StateAt(a, mood, now)
		}
		body.Draw(func(p *ui.Painter, r ui.Rect) {
			if set != nil {
				DrawView(a, p, r, set, state, FrameAt(a, state, since, p.Now()))
			} else {
				Draw(p, r, a.PetState(), a.Focused(), waitingNow(a), mood, hovered)
			}
			p.After(tick(a))
		})
		Drag(a, body)
		clicked = clicked || body.Clicked()
		if a.PetState().Moved {
			// A drag is not a pat: it only let go.
			clicked = false
			if !a.PetState().Dragging {
				a.PetState().Moved = false
			}
		}
	})
	if clicked {
		Petted(a, now, mood)
	}

	// Rebuild when what the pet says, or how awake it is, turns over;
	// its motion repaints without a build, in petDraw.
	if mood == Idle {
		if at := a.PetState().LastActive.Add(Doze); at.After(now) {
			c.After(at.Sub(now) + 16*time.Millisecond)
		}
	}
	if showing && (mood == Working || mood == Done || mood == Failed) {
		if d := SaysFor(mood) - now.Sub(a.PetState().Since); d > 0 {
			c.After(d + 16*time.Millisecond)
		}
	}
	if d := Wink - now.Sub(a.PetState().Wink); d > 0 {
		c.After(d + 16*time.Millisecond)
	}
}

// b2f is a bool as 0 or 1, for an animation's value.
func b2f(v bool) float32 {
	if v {
		return 1
	}
	return 0
}

// petDescription is what assistive technology hears of the pet: its
// name, and what it says, if anything.
func petDescription(name, label string) string {
	if label == "" {
		return name
	}
	return name + ": " + label
}

// petSpeech says what the pet's bubble says, and how it looks.
func speech(a App, pal *theme.Palette, mood Mood, n int) (label string, fill, fg ui.Color) {
	switch mood {
	case Waiting:
		return locales.FooterNeedsYou(n), pal.Attention, ui.Hex("#241a08")
	case Failed:
		label = locales.StatusFailed()
		if n > 1 {
			label += " · " + strconv.Itoa(n)
		}
		return label, pal.Danger, ui.Hex("#ffffff")
	case Done:
		return locales.FooterDone(n), pal.Done, pal.OnDone
	case Working:
		return locales.FooterWorking(n), pal.Popover, pal.TextMuted
	}
	return "", ui.Transparent, ui.Transparent
}
