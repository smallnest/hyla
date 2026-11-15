// Package petme holds the pet on the window's top edge: what it feels,
// what it says, the windows it opens and the window that shows it. By
// default it is Hyla the frog, drawn frame by frame; a Codex pet's
// spritesheet shows instead where the settings choose one.
package petme

import (
	"time"

	"hyla/internal/proto"
)

// Mood is what the pet shows: the most urgent thing among the sessions.
type Mood int

const (
	Idle Mood = iota
	Working
	Waiting
	Done
	Failed
	Asleep
)

// Timing: how often the pet draws, and how long what it says stays.
const (
	Frame  = 33 * time.Millisecond // 30 frames a second
	Calm   = 100 * time.Millisecond
	Doze   = 45 * time.Second // all calm this long, and it sleeps
	Wink   = 700 * time.Millisecond
	Pat    = 1200 * time.Millisecond // a pat shows its wave this long
	Bubble = 5 * time.Second         // a working or finished word stays this long
	Linger = 9 * time.Second         // a failure or a finish says so this long
)

// MoodNow tells what the pet should feel now: the most urgent thing
// among the sessions, and how many sessions are that. A host the app lost
// says nothing, as for the sidebar's counts.
func MoodNow(sessions []proto.Session, lastActive time.Time, at time.Time) (Mood, int) {
	waiting, failed, done, working := 0, 0, 0, 0
	for i := range sessions {
		s := &sessions[i]
		switch {
		case s.Status == proto.Waiting:
			waiting++
		case s.Status == proto.Failed:
			failed++
		case s.Unread && (s.Status == proto.Idle || s.Status == proto.Exited):
			done++
		case s.Status == proto.Working:
			working++
		}
	}
	switch {
	case waiting > 0:
		return Waiting, waiting
	case failed > 0:
		return Failed, failed
	case done > 0:
		return Done, done
	case working > 0:
		return Working, working
	}
	if !lastActive.IsZero() && at.Sub(lastActive) > Doze {
		return Asleep, 0
	}
	return Idle, 0
}

// Says tells whether the pet's bubble shows now.
func Says(now, since time.Time, mood Mood) bool {
	switch mood {
	case Waiting:
		return true
	case Working, Done, Failed:
		return now.Sub(since) < SaysFor(mood)
	}
	return false
}

// SaysFor is how long a bubbled mood says so; waiting stays until it is
// answered.
func SaysFor(mood Mood) time.Duration {
	switch mood {
	case Done, Failed:
		return Linger
	default:
		return Bubble
	}
}
