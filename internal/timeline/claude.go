package timeline

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// ClaudeRead is how much of a Claude Code transcript a read parses: the
// end of it, which is the part the user looks at.
const ClaudeRead = 8 << 20

// Claude reads a Claude Code transcript as a timeline: each prompt the
// session got begins a turn, which the steps that follow it fill, and its
// tool calls are the tools the turn ran. A subagent, which Claude Code
// runs as the Agent tool, is a step of its own, with what its answer said
// of it.
//
// The line's own time is the transcript's when there is one; a
// conversation that tells none is laid out from start, the first step's
// time, in the order it wrote them, and the steps after it were written
// are running at now. now is the time a step still running has reached.
func Claude(data []byte, start, now time.Time) []Step {
	var turns []Step
	cur := -1 // the turn being written
	// tools are the tool calls of the current run of assistant messages,
	// by id, which their results end.
	tools := map[string]*Step{}
	var last int64 // the last line's time, unix ms
	clocked := false
	at := func(e claudeEntry) int64 {
		if t, ok := parseTime(e.Timestamp); ok {
			last, clocked = t, true
			return t
		}
		// A line without one comes a moment after the one before, and
		// the first, start comes a moment later: the timeline is from
		// start.
		last++
		return last
	}
	last = start.UnixMilli()
	nowMs := now.UnixMilli()
	_ = clocked
	begin := func(title string, t int64) {
		turns = append(turns, Step{Kind: Turn, Title: title, Start: t, State: Running})
	}
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		var e claudeEntry
		if json.Unmarshal(l, &e) != nil || e.IsSidechain {
			continue
		}
		// A meta line, as a summary, tells of a step later: no.
		if e.IsMeta {
			continue
		}
		switch e.Type {
		case "user":
			if text, ok := promptText(e.Message.Content); ok {
				if strings.HasPrefix(text, "<") && !strings.HasPrefix(text, "<user") {
					// A reminder the client wrote, not the user: no turn.
					continue
				}
				// What the user said ends the turn before: its answer.
				if n := len(turns); n > 0 && turns[n-1].End == 0 {
					closeTurn(&turns[n-1], at(e))
				}
				begin(line(text, 120), at(e))
				cur, tools = len(turns)-1, map[string]*Step{}
				continue
			}
			for _, part := range partsOf(e.Message.Content) {
				if part.Type != "tool_result" {
					continue
				}
				end := at(e)
				if st := tools[part.ToolUseID]; st != nil {
					st.End = end
					// A subagent's answer is what it was asked, not
					// what it said of itself.
					if st.Kind != Subagent {
						st.Detail = joinLines(st.Detail, resultText(part.Content))
					}
					if part.IsError {
						st.State = Failed
					} else {
						st.State = Done
					}
					delete(tools, part.ToolUseID)
				}
				endStep(turns, cur, end)
			}
		case "assistant":
			for _, part := range partsOf(e.Message.Content) {
				switch part.Type {
				case "thinking":
					addStep(turns, cur, Step{Kind: Think, Title: line(part.Thinking, 80), Start: at(e), State: Running})
				case "text":
					addStep(turns, cur, Step{Kind: Text, Title: line(part.Text, 80), Start: at(e), State: Running})
				case "tool_use":
					st := Step{Kind: Tool, Title: part.Name, Detail: short(part.Input, 200), Start: at(e), State: Running}
					if isSubagent(part.Name) {
						st.Kind = Subagent
					}
					st = subagentDetail(st, part.Input)
					addStep(turns, cur, st)
					// The tool is the last step of the turn, which its
					// result, in a message that is not the prompt, ends.
					if n := len(turns); n > 0 && cur == n-1 && part.ID != "" {
						if k := len(turns[cur].Children); k > 0 {
							tools[part.ID] = &turns[cur].Children[k-1]
						}
					}
				}
			}
		}
	}
	// A step without an end is one the agent gave up or never told: it
	// stays running while the agent still works, and ends where the next
	// step began otherwise.
	for i := range turns {
		closeTurn(&turns[i], nowMs)
	}
	return order(turns)
}

// closeTurn gives a turn's steps their ends: the words and reasoning at
// the next step's start, and the steps that never said, at nowMs, which is
// the next prompt's time when one came, else the read's. A turn ended
// ends at its last step's end, as the answer its agent gave; one whose
// last step is the open words runs on.
func closeTurn(t *Step, nowMs int64) {
	closeOpen(t, nowMs)
	if last := len(t.Children) - 1; last >= 0 && openStep(t.Children[last]) {
		return
	}
	if t.End == 0 && len(t.Children) > 0 {
		t.End, t.State = t.Children[len(t.Children)-1].End, Done
	}
}

// openStep tells whether a step, or one of its children, still runs.
func openStep(s Step) bool {
	if s.End == 0 {
		return true
	}
	for _, c := range s.Children {
		if openStep(c) {
			return true
		}
	}
	return false
}

// closeOpen ends the steps of a turn that have no end: at the end of the
// turn, which is its last child's start, else now while it runs.
func closeOpen(t *Step, nowMs int64) {
	for i := range t.Children {
		c := &t.Children[i]
		closeOpen(c, nowMs)
		if c.End != 0 {
			continue
		}
		end := nowMs
		if i+1 < len(t.Children) {
			end = t.Children[i+1].Start
		}
		if end < c.Start {
			end = c.Start
		}
		c.End, c.State = end, Done
	}
	if t.End == 0 && len(t.Children) > 0 {
		t.End = t.Children[len(t.Children)-1].End
	}
}

// endStep ends a turn's open steps at end, as a tool's result found
// them: the words and reasoning it answered ran to then, as no tool
// pending any more.
func endStep(turns []Step, i int, end int64) {
	if i < 0 || i >= len(turns) {
		return
	}
	children := turns[i].Children
	for n := range children {
		if c := &children[n]; c.End == 0 {
			c.End = end
			if c.State == Running {
				c.State = Done
			}
		}
	}
}

// addStep adds a step to a turn, which its own start cannot come before.
func addStep(turns []Step, i int, s Step) {
	if i < 0 || i >= len(turns) {
		return
	}
	children := turns[i].Children
	if n := len(children); n > 0 && s.Start < children[n-1].Start {
		s.Start = children[n-1].Start
	}
	turns[i].Children = append(children, s)
}

// order sorts a timeline's steps by when they started, turns and all: a
// transcript written in time reads in time, and one without times keeps
// the order it wrote.
func order(steps []Step) []Step {
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].Start != steps[j].Start {
			return steps[i].Start < steps[j].Start
		}
		return steps[i].End < steps[j].End
	})
	return steps
}

// claudeEntry is a Claude Code transcript's line, as this package reads
// it: the messages, their parts, and the time the line was written.
type claudeEntry struct {
	Type        string `json:"type"`
	Timestamp   string `json:"timestamp"`
	IsSidechain bool   `json:"isSidechain"`
	IsMeta      bool   `json:"isMeta"`
	Message     struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// claudePart is a part of a message: its text, its reasoning, or a tool
// call, an answer to one.
type claudePart struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

// promptText is a message's content where it is a prompt: the text the
// user typed, which is no part of a list.
func promptText(raw json.RawMessage) (string, bool) {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return "", false
	}
	return text, true
}

// partsOf is a message's content as parts: a prompt's text is no part.
func partsOf(raw json.RawMessage) []claudePart {
	var parts []claudePart
	if json.Unmarshal(raw, &parts) != nil {
		return nil
	}
	return parts
}

// isSubagent tells whether a tool starts an agent of its own: Claude
// Code's Agent, which was Task.
func isSubagent(name string) bool { return name == "Agent" || name == "Task" }

// subagentDetail names a subagent by what it was asked to do, and tells
// what it is.
func subagentDetail(st Step, input json.RawMessage) Step {
	var in struct {
		Description  string `json:"description"`
		SubagentType string `json:"subagent_type"`
	}
	if json.Unmarshal(input, &in) == nil {
		if in.Description != "" {
			st.Title = in.Description
		} else if in.SubagentType != "" {
			st.Title = in.SubagentType
		}
		if in.SubagentType != "" {
			st.Detail = in.SubagentType
		}
	}
	if st.Title == "" {
		st.Title = "subagent"
	}
	return st
}

// resultText is a tool result's text: from a string, or from its parts.
func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return line(s, 160)
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(p.Text)
			}
		}
		return line(b.String(), 160)
	}
	return ""
}

// short is JSON, as a tool's arguments read: one line, cut to n.
func short(raw json.RawMessage, n int) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return line(string(raw), n)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return line(string(raw), n)
	}
	return line(string(b), n)
}

// joinLines adds a detail to another, one per line.
func joinLines(a, b string) string {
	switch {
	case b == "":
		return a
	case a == "":
		return b
	}
	return a + "\n" + b
}

// parseTime reads an RFC 3339 time as unix ms.
func parseTime(s string) (int64, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0, false
	}
	return t.UnixMilli(), true
}

// readTail reads the end of a file, up to n bytes: what a read of a long
// transcript parses.
func readTail(path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var off int64
	if st.Size() > n {
		off = st.Size() - n
	}
	b, err := io.ReadAll(io.NewSectionReader(f, off, st.Size()-off))
	if err != nil {
		return nil, err
	}
	// A cut line is no line: drop the first, partial one.
	if off > 0 {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	return b, nil
}

// ClaudeFile reads a Claude Code transcript as a timeline, the end of it
// that ClaudeRead bounds. A conversation that tells no times is laid out
// from start.
func ClaudeFile(path string, now time.Time) ([]Step, error) {
	b, err := readTail(path, ClaudeRead)
	if err != nil {
		return nil, err
	}
	start := now
	if st, err := os.Stat(path); err == nil {
		start = st.ModTime().Add(-time.Hour)
	}
	return Claude(b, start, now), nil
}
