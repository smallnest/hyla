// Package timeline reads the coding agents' own records and lays them out
// as a timeline: the turns, steps and tools an agent ran, and the
// subagents it gave its work to. It reads Claude Code's transcripts and
// DeepSeek Harness's session logs where the agent keeps them, and leaves
// out the agents whose records it does not read, which have Hyla's
// statuses alone.
package timeline

import (
	"strings"
	"time"
)

// State is what a step, a turn or a subagent is doing.
type State string

const (
	Done    State = "done"
	Running State = "running"
	Waiting State = "waiting" // blocked on the user: an approval, a question
	Failed  State = "failed"
)

// Kind is what a step is.
type Kind string

const (
	Turn     Kind = "turn"     // what the user asked, and the agent's work on it
	Exchange Kind = "step"     // one of a turn's exchanges with the model
	Tool     Kind = "tool"     // a tool the agent ran
	Think    Kind = "think"    // the agent's reasoning
	Text     Kind = "text"     // what the agent wrote
	Subagent Kind = "subagent" // an agent the agent started for its work
)

// Step is one thing an agent did, from when it started to when it ended.
// Its children are what it holds: a turn's steps, a step's tools.
type Step struct {
	Kind     Kind
	Title    string // the tool's name, or a line of the words
	Detail   string // the tool's arguments or output, or what it waits for
	Start    int64  // unix ms
	End      int64  // unix ms, 0 while it runs
	State    State
	Children []Step
}

// Running tells whether the step is still going.
func (s Step) Running() bool { return s.End == 0 }

// Duration is how long the step took, or has been going at at when it
// runs.
func (s Step) Duration(at time.Time) time.Duration {
	end := s.End
	if end == 0 {
		end = at.UnixMilli()
	}
	if d := time.Duration(end-s.Start) * time.Millisecond; d > 0 {
		return d
	}
	return 0
}

// Span is when a timeline starts and ends: its earliest start, and its
// latest end, which is now while a step runs.
func Span(steps []Step, now time.Time) (start, end time.Time) {
	sh := span{}
	sh.walk(steps, now.UnixMilli())
	return time.UnixMilli(sh.start), time.UnixMilli(sh.end)
}

// span collects a set of steps' bounds.
type span struct {
	start, end int64 // 0 until set
}

func (s *span) walk(steps []Step, at int64) {
	for _, st := range steps {
		if st.Start > 0 && (s.start == 0 || st.Start < s.start) {
			s.start = st.Start
		}
		e := st.End
		if e == 0 {
			e = at
		}
		if e > s.end {
			s.end = e
		}
		s.walk(st.Children, at)
	}
}

// line is the first line of a text, cut to n characters.
func line(text string, n int) string {
	text = strings.TrimSpace(text)
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		text = text[:i]
	}
	return cut(text, n)
}

// cut is text cut to n characters, its end marked with an ellipsis.
func cut(text string, n int) string {
	r := []rune(text)
	if len(r) <= n {
		return text
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}
