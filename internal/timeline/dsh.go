package timeline

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// DSHRead bounds how large a DeepSeek Harness session log a read decodes:
// the logs are compressed, and a long one is large.
const DSHRead = 64 << 20

// dshEntry is a line of DeepSeek Harness's session log: its type, the
// time it happened, and what the type says.
type dshEntry struct {
	Type string          `json:"type"`
	Time int64           `json:"time"`
	Data json.RawMessage `json:"data"`
}

// dshTurn is a turn/start, a step/start or a turn/end's data.
type dshTurn struct {
	Turn   int `json:"turn"`
	Step   int `json:"step"`
	Reason struct {
		Kind string `json:"kind"`
	} `json:"reason"`
}

// dshTool is a tool/call's data.
type dshTool struct {
	CallID    string `json:"callId"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// dshJob is a job/registered or a job/settled's data.
type dshJob struct {
	JobID      string `json:"jobId"`
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
	Status     string `json:"status"`
	ToolCall   struct {
		CallID string `json:"callId"`
	} `json:"toolCall"`
}

// dshApproval is an approval/asked or an approval/decided's data.
type dshApproval struct {
	ID       string `json:"id"`
	ToolName string `json:"toolName"`
	Reason   string `json:"reason"`
	Outcome  string `json:"outcome"`
}

// dshMessage is an assistant/message's data: its words, and the tools it
// asked for, which its tool/call lines ran.
type dshMessage struct {
	Turn    int `json:"turn"`
	Step    int `json:"step"`
	Message struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			ToolName string `json:"name"`
			Input    any    `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

// DSH reads a DeepSeek Harness session log as a timeline: each turn is
// the user's, its steps are the exchanges with the model, and a step's
// tools are the calls the model made in it, a long one a job it settled.
// now is the time a step still running has reached.
func DSH(data []byte, now time.Time) []Step {
	var turns []Step
	steps := map[int]*Step{} // by step number, within its turn
	jobs := map[string]*Step{}
	tools := map[string]*Step{}
	at := func(e dshEntry) int64 {
		if e.Time > 0 {
			return e.Time
		}
		if n := len(turns); n > 0 && len(turns[n-1].Children) > 0 {
			return turns[n-1].Children[len(turns[n-1].Children)-1].Start
		}
		return now.UnixMilli()
	}
	lastTurn := func() *Step {
		if len(turns) == 0 {
			turns = append(turns, Step{Kind: Turn, Title: "turn", Start: now.UnixMilli(), State: Running})
		}
		return &turns[len(turns)-1]
	}
	// add appends a step to a step's children, a fresh slice each time.
	add := func(to *Step, s Step) *Step {
		to.Children = append(to.Children, s)
		return &to.Children[len(to.Children)-1]
	}
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		var e dshEntry
		if json.Unmarshal(l, &e) != nil {
			continue
		}
		t := at(e)
		switch e.Type {
		case "turn/start":
			var d dshTurn
			json.Unmarshal(e.Data, &d)
			turns = append(turns, Step{Kind: Turn, Title: "turn " + itoa(d.Turn), Start: t, State: Running})
			steps = map[int]*Step{}
		case "user/message":
			// The turn's words, unless they are the agent's own context:
			// a handoff, or a message from another agent.
			if n := len(turns); n > 0 {
				if title := firstText(e.Data); title != "" && !contextText(title) {
					turns[n-1].Title = line(title, 120)
				}
			}
		case "step/start":
			var d dshTurn
			json.Unmarshal(e.Data, &d)
			tr := lastTurn()
			st := add(tr, Step{Kind: Exchange, Title: "step " + itoa(d.Step), Start: t, State: Running})
			steps[d.Step] = st
		case "assistant/message":
			var d dshMessage
			json.Unmarshal(e.Data, &d)
			st := steps[d.Step]
			if st == nil {
				st = add(lastTurn(), Step{Kind: Exchange, Title: "step " + itoa(d.Step), Start: t, State: Running})
				steps[d.Step] = st
			}
			for _, part := range d.Message.Content {
				switch part.Type {
				case "reasoning", "thinking":
					if strings.TrimSpace(part.Text) != "" {
						add(st, Step{Kind: Think, Title: line(part.Text, 80), Start: t, State: Running})
					}
				case "text":
					if strings.TrimSpace(part.Text) != "" {
						add(st, Step{Kind: Text, Title: line(part.Text, 80), Start: t, State: Running})
					}
				case "tool_use":
					if part.ToolName != "" {
						if tool := tools[argString(part.Input, "id")]; tool == nil {
							add(st, Step{Kind: Tool, Title: part.ToolName, Start: t, State: Running})
						}
					}
				}
			}
		case "step/end":
			var d dshTurn
			json.Unmarshal(e.Data, &d)
			if st := steps[d.Step]; st != nil {
				st.End, st.State = t, Done
			}
			delete(steps, d.Step)
		case "turn/end":
			var d dshTurn
			json.Unmarshal(e.Data, &d)
			if n := len(turns); n > 0 {
				tr := &turns[n-1]
				tr.End, tr.State = t, Done
				if d.Reason.Kind != "" && d.Reason.Kind != "completed" && d.Reason.Kind != "aborted" {
					tr.State = Failed
				}
			}
		case "tool/call":
			var d dshTool
			json.Unmarshal(e.Data, &d)
			to := steps[dshStepOf(e.Data)]
			if to == nil {
				to = lastTurn()
			}
			// A tool's arguments as it read them: its command, its path.
			st := add(to, Step{Kind: Tool, Title: d.Name, Detail: line(arguments(d.Arguments), 200), Start: t, State: Running})
			if d.CallID != "" {
				tools[d.CallID] = st
			}
		case "job/registered":
			var d dshJob
			json.Unmarshal(e.Data, &d)
			start := d.StartedAt
			if start == 0 {
				start = t
			}
			var st *Step
			if d.ToolCall.CallID != "" {
				st = tools[d.ToolCall.CallID]
			}
			if st == nil {
				st = add(lastTurn(), Step{Kind: Tool, Title: d.Kind, Start: start, State: Running})
			}
			st.Title, st.Detail = jobTitle(d), line(d.Label, 200)
			st.Start = start
			if d.JobID != "" {
				jobs[d.JobID] = st
			}
		case "job/settled":
			var d dshJob
			json.Unmarshal(e.Data, &d)
			st := jobs[d.JobID]
			if st == nil {
				continue
			}
			st.End, st.State = d.FinishedAt, Done
			if st.End == 0 {
				st.End = t
			}
			if d.Status != "" && d.Status != "completed" {
				st.State = Failed
			}
		case "tool/result":
			var d struct {
				ToolCallID string `json:"toolCallId"`
				IsError    bool   `json:"isError"`
				Content    []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}
			var m struct {
				Message struct {
					ToolCallID string `json:"toolCallId"`
					IsError    bool   `json:"isError"`
					Content    []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			}
			json.Unmarshal(e.Data, &m)
			d.ToolCallID, d.IsError, d.Content = m.Message.ToolCallID, m.Message.IsError, m.Message.Content
			st := tools[d.ToolCallID]
			if st == nil {
				continue
			}
			// A job's end is its settled time, not when its output came.
			if st.End == 0 {
				st.End = t
			}
			if st.State == Running {
				st.State = Done
			}
			if d.IsError {
				st.State = Failed
			}
			for _, c := range d.Content {
				if c.Type == "text" && c.Text != "" {
					st.Detail = joinLines(st.Detail, line(c.Text, 160))
					break
				}
			}
		case "approval/asked":
			var d dshApproval
			json.Unmarshal(e.Data, &d)
			// The approval waits in place of the turn's step, which
			// waits for it: the two are one step, the step's waiting.
			st := add(lastTurn(), Step{Kind: Tool, Title: "approval: " + d.ToolName, Detail: line(d.Reason, 160), Start: t, State: Waiting})
			if d.ID != "" {
				jobs["approval:"+d.ID] = st
			}
		case "approval/decided":
			var d dshApproval
			json.Unmarshal(e.Data, &d)
			if st := jobs["approval:"+d.ID]; st != nil {
				st.End, st.State = t, Done
				if d.Outcome != "allowed-once" && d.Outcome != "allowed" {
					st.State = Failed
				}
			}
		}
	}
	for i := range turns {
		closeOpen(&turns[i], now.UnixMilli())
	}
	return order(turns)
}

// arguments is a tool call's arguments as they read: its command or its
// path, else the JSON.
func arguments(raw string) string {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) == nil {
		for _, k := range []string{"cmd", "command", "file_path", "path", "pattern", "query", "url", "description"} {
			if s, ok := m[k].(string); ok && s != "" {
				return s
			}
		}
	}
	return raw
}

// dshStepOf is the step number a tool call's data names.
func dshStepOf(raw json.RawMessage) int {
	var d struct {
		Step int `json:"step"`
	}
	json.Unmarshal(raw, &d)
	return d.Step
}

// jobTitle names a job by its kind, and what it ran.
func jobTitle(d dshJob) string {
	switch {
	case d.Kind != "" && d.Label != "":
		return d.Kind
	case d.Kind != "":
		return d.Kind
	}
	return "job"
}

// contextText tells whether a message is the agent's context, which a
// turn's title leaves out: a handoff's, or another agent's.
func contextText(text string) bool {
	for _, p := range []string{"[Handoff", "[Message from", "[Automated", "[System"} {
		if strings.HasPrefix(strings.TrimSpace(text), p) {
			return true
		}
	}
	return false
}

// firstText is a user message's first text.
func firstText(raw json.RawMessage) string {
	var d struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &d) != nil {
		return ""
	}
	for _, c := range d.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			return c.Text
		}
	}
	return ""
}

// argString is a field of a decoded argument object, "" for none.
func argString(v any, key string) string {
	if m, ok := v.(map[string]any); ok {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	return ""
}

// itoa is a number in words.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// DSHFile reads a DeepSeek Harness session log, which is zstd-compressed,
// as a timeline. ClaudeRead is what it decodes at most.
func DSHFile(path string, now time.Time) ([]Step, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := zstd.NewReader(f, zstd.WithDecoderConcurrency(1))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var b bytes.Buffer
	if _, err := b.ReadFrom(zr.IOReadCloser()); err != nil && b.Len() == 0 {
		return nil, err
	}
	data := b.Bytes()
	if len(data) > DSHRead {
		// The end of a long log: its start's lines are cut.
		data = data[len(data)-DSHRead:]
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		}
	}
	return DSH(data, now), nil
}
