package timeline

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CodexRead bounds how much of a Codex rollout a read parses: the end of
// it, which is the part the user looks at.
const CodexRead = 8 << 20

// codexHome is Codex's directory, as its own command reads it.
func codexHome() string {
	if d := os.Getenv("CODEX_HOME"); d != "" {
		return d
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".codex")
}

// codexRollout is the rollout of a Codex conversation: the one whose name
// ends with the conversation's ID, else the newest one the directory ran
// in. It returns "" where it finds none.
func codexRollout(conv, dir string) string {
	root := filepath.Join(codexHome(), "sessions")
	if conv != "" {
		if p := codexByID(root, conv); p != "" {
			return p
		}
	}
	if dir == "" {
		return ""
	}
	return codexNewest(root, dir)
}

// codexFiles are a rollout root's files, newest first: a rollout's name
// holds the time it began.
func codexFiles(root string) []string {
	var files []string
	fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		files = append(files, filepath.Join(root, p))
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}

// codexByID is the rollout of a conversation, by its ID.
func codexByID(root, conv string) string {
	for _, p := range codexFiles(root) {
		if strings.HasSuffix(p, conv+".jsonl") {
			return p
		}
	}
	return ""
}

// codexNewest is the newest rollout that ran in dir.
func codexNewest(root, dir string) string {
	for _, p := range codexFiles(root) {
		if codexDirOf(p) == dir {
			return p
		}
	}
	return ""
}

// codexDirOf is the directory a rollout ran in, as its first line, the
// session's meta, tells it.
func codexDirOf(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	// The session's meta carries the model's instructions, so its line is
	// long.
	line, err := bufio.NewReader(f).ReadBytes('\n')
	if len(line) == 0 && err != nil {
		return ""
	}
	var e codexEntry
	if json.Unmarshal(bytes.TrimSpace(line), &e) != nil || e.Type != "session_meta" {
		return ""
	}
	var p struct {
		Cwd string `json:"cwd"`
	}
	if json.Unmarshal(e.Payload, &p) != nil {
		return ""
	}
	return p.Cwd
}

// CodexFile reads a Codex rollout as a timeline: the end of it that
// CodexRead bounds.
func CodexFile(path string, now time.Time) ([]Step, error) {
	b, err := readTail(path, CodexRead)
	if err != nil {
		return nil, err
	}
	start := now
	if st, err := os.Stat(path); err == nil {
		start = st.ModTime().Add(-time.Hour)
	}
	return Codex(b, start, now), nil
}

// codexEntry is a line of a rollout: the time it was written, what it is,
// and what that says.
type codexEntry struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// codexPayload is a line's payload: a message, its reasoning, a tool the
// model called or the answer to one, or an event of the turn.
type codexPayload struct {
	Type  string `json:"type"`
	Role  string `json:"role"`
	Model string `json:"model"`
	// A message's or a reasoning's words.
	Content []codexText `json:"content"`
	Summary []codexText `json:"summary"`
	// The turn an event tells of, as task_started, task_complete and
	// turn_aborted name it.
	TurnID string `json:"turn_id"`
	// A function call's, and the answer to it.
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	CallID    string `json:"call_id"`
	Output    string `json:"output"`
}

// codexText is a part of a message's content.
type codexText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Codex reads a rollout as a timeline: each message the user sent begins a
// turn, and what the model reasoned, wrote and ran after it are the turn's
// steps, the tools from when the model called them to when their output
// came. A turn ends with the task's completion, or runs while the agent
// still works. now is the time a step still running has reached.
func Codex(data []byte, start, now time.Time) []Step {
	var turns []Step
	cur := -1    // the turn being written
	turnID := "" // the turn's, as Codex names it
	tools := map[string]*Step{}
	last := start.UnixMilli()
	nowMs := now.UnixMilli()
	at := func(e codexEntry) int64 {
		if t, ok := parseTime(e.Timestamp); ok {
			last = t
			return t
		}
		// A line without a time comes a moment after the one before.
		last++
		return last
	}
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		var e codexEntry
		if json.Unmarshal(l, &e) != nil {
			continue
		}
		var p codexPayload
		if len(e.Payload) > 0 && json.Unmarshal(e.Payload, &p) != nil {
			continue
		}
		t := at(e)
		switch e.Type {
		case "response_item":
			switch p.Type {
			case "message":
				switch p.Role {
				case "user":
					text := codexTextOf(p.Content)
					if text == "" || codexContext(text) {
						continue
					}
					// The prompt begins a turn, and ends the one before
					// it, whose answer it is.
					if n := len(turns); n > 0 && turns[n-1].End == 0 {
						codexEnd(turns, n-1, t, Done, nowMs)
					}
					turns = append(turns, Step{Kind: Turn, Title: line(text, 120), Start: t, State: Running})
					cur, tools = len(turns)-1, map[string]*Step{}
				case "assistant":
					if text := codexTextOf(p.Content); text != "" {
						addStep(turns, cur, Step{Kind: Text, Title: line(text, 80), Start: t, State: Running})
					}
				}
			case "reasoning":
				text := codexTextOf(p.Content)
				if text == "" {
					text = codexTextOf(p.Summary)
				}
				if text != "" {
					addStep(turns, cur, Step{Kind: Think, Title: line(text, 80), Start: t, State: Running})
				}
			case "function_call":
				st := Step{Kind: Tool, Title: p.Name, Detail: line(arguments(p.Arguments), 200), Start: t, State: Running}
				if st.Title == "" {
					st.Title = "tool"
				}
				addStep(turns, cur, st)
				if p.CallID != "" && cur >= 0 {
					if n := len(turns[cur].Children); n > 0 {
						tools[p.CallID] = &turns[cur].Children[n-1]
					}
				}
			case "function_call_output":
				// The answer to a call ends its tool, and tells how it
				// went by the code the program left.
				if st := tools[p.CallID]; st != nil {
					st.End, st.State = t, Done
					if code, ok := codexExit(p.Output); ok && code != 0 {
						st.State = Failed
					}
					st.Detail = joinLines(st.Detail, line(codexSaid(p.Output), 160))
					delete(tools, p.CallID)
				}
			}
		case "event_msg":
			switch p.Type {
			case "task_started":
				turnID = p.TurnID
			case "task_complete", "turn_aborted":
				// A turn's end names the turn: one of another turn's, as a
				// file that holds a restart does, is no end of this one.
				if p.TurnID != "" && turnID != "" && p.TurnID != turnID {
					continue
				}
				state := Done
				if p.Type == "turn_aborted" {
					state = Failed
				}
				codexEnd(turns, cur, t, state, nowMs)
			}
		}
	}
	// A turn whose task never said it ended ends with its steps: it runs
	// while the agent does.
	for i := range turns {
		codexEnd(turns, i, 0, Running, nowMs)
	}
	return order(turns)
}

// codexEnd ends a turn at t, or leaves it running, with the steps that
// still run: they end where the next step began, and the last, with the
// turn, or at now while the turn runs.
func codexEnd(turns []Step, i int, t int64, state State, nowMs int64) {
	if i < 0 || i >= len(turns) {
		return
	}
	turn := &turns[i]
	for n := range turn.Children {
		c := &turn.Children[n]
		if c.End != 0 {
			continue
		}
		if t == 0 && n == len(turn.Children)-1 {
			// The last step of a turn that runs runs still: the bar
			// reaches now, and blinks.
			c.State = Running
			continue
		}
		end := nowMs
		if n+1 < len(turn.Children) {
			end = turn.Children[n+1].Start
		} else if t != 0 {
			end = t
		}
		c.End = max(end, c.Start)
		if c.State == Running {
			c.State = Done
		}
	}
	if t == 0 {
		return
	}
	turn.End, turn.State = t, state
}

// codexContext tells a user message the client wrote, not the user: the
// instructions Codex puts in the conversation, and what it says of a turn
// that ended.
func codexContext(text string) bool {
	return strings.HasPrefix(text, "<") || strings.HasPrefix(text, "# AGENTS.md instructions")
}

// codexTextOf is a message's or a reasoning's words: its text parts'.
func codexTextOf(parts []codexText) string {
	var b strings.Builder
	for _, p := range parts {
		if strings.TrimSpace(p.Text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.Text)
	}
	return b.String()
}

// codexExit is the exit code a tool's output tells, and whether it told
// one: Codex's shell writes it above the output.
func codexExit(out string) (int, bool) {
	const tell = "Process exited with code "
	i := strings.LastIndex(out, tell)
	if i < 0 {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(out[i+len(tell):], "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

// codexSaid is a tool's output, without the lines Codex's shell writes
// above it.
func codexSaid(out string) string {
	if i := strings.Index(out, "Output:\n"); i >= 0 {
		out = out[i+len("Output:\n"):]
	}
	return strings.TrimSpace(out)
}
