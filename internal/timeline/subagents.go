package timeline

import (
	"bytes"
	"encoding/json"
	"sort"
)

// SubagentRun is one agent a timeline's agent gave work to: the turn that
// gave it the work, what it was, and from when to when it ran.
type SubagentRun struct {
	Type  string // Explore, general-purpose, as the agent names it
	Task  string // what it was asked
	Turn  string // the turn of the agent's that gave it the work
	Start int64  // unix ms
	End   int64  // unix ms, 0 while it runs
	State State
}

// Subagents returns a timeline's subagents, the newest first: the ones that
// still run before those that ended, and each set in the order they began.
func Subagents(turns []Step) []SubagentRun {
	var subs []SubagentRun
	var walk func(turn string, steps []Step)
	walk = func(turn string, steps []Step) {
		for _, s := range steps {
			if s.Kind == Subagent {
				subs = append(subs, SubagentRun{
					Type: s.Detail, Task: s.Title, Turn: turn,
					Start: s.Start, End: s.End, State: s.State,
				})
				continue
			}
			walk(turn, s.Children)
		}
	}
	for _, t := range turns {
		walk(t.Title, t.Children)
	}
	sort.SliceStable(subs, func(i, j int) bool {
		runs, run := subs[i].End == 0, subs[j].End == 0
		if runs != run {
			return runs
		}
		if subs[i].Start != subs[j].Start {
			return subs[i].Start > subs[j].Start
		}
		return subs[i].Task < subs[j].Task
	})
	return subs
}

// SubagentRead is how much of a subagent's transcript a read parses: they
// are short, and the end of one is what it does now.
const SubagentRead = 512 << 10

// SubagentNow returns the step a subagent runs now, from its own
// transcript: the last tool it called that no result ended, or what it
// wrote last. It returns "" where the transcript tells none.
func SubagentNow(path string) string {
	if path == "" {
		return ""
	}
	data, err := readTail(path, SubagentRead)
	if err != nil {
		return ""
	}
	var calls []string          // the tool calls, in the order they came
	open := map[string]string{} // the calls by id, until their result ends them
	wrote := ""
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		var e claudeEntry
		if json.Unmarshal(l, &e) != nil {
			continue
		}
		switch e.Type {
		case "assistant":
			for _, part := range partsOf(e.Message.Content) {
				switch part.Type {
				case "tool_use":
					if part.ID == "" || part.Name == "" {
						continue
					}
					calls = append(calls, part.ID)
					open[part.ID] = part.Name
				case "text":
					if part.Text != "" {
						wrote = part.Text
					}
				}
			}
		case "user":
			for _, part := range partsOf(e.Message.Content) {
				if part.Type == "tool_result" {
					delete(open, part.ToolUseID)
				}
			}
		}
	}
	// The last call still open is the one that runs.
	for i := len(calls) - 1; i >= 0; i-- {
		if name, ok := open[calls[i]]; ok {
			return name
		}
	}
	return line(wrote, 80)
}
