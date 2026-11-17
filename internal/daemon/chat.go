package daemon

import (
	"errors"
	"os"
	"strings"
	"time"

	"hyla/internal/proto"
	"hyla/internal/timeline"
	"hyla/locales"
)

// chatOf is a pane's conversation, from the records its agent keeps beside
// it: what the user asked, what the agent wrote and reasoned, and the tools
// it ran. A pane whose agent keeps none Hyla reads has a note in place of
// the lines.
func (d *Daemon) chatOf(p proto.ChatParams) (proto.Chat, error) {
	pn := d.findPane(p.Pane)
	if pn == nil && p.Pane != "" {
		return proto.Chat{}, os.ErrNotExist
	}
	if pn == nil && p.Session != "" {
		// The session's own pane: the one the user is in, else its first,
		// that runs an agent Hyla reads.
		d.mu.Lock()
		if s := d.findSessionLocked(p.Session); s != nil {
			for _, q := range append([]*pane{focusedLocked(s)}, s.panes...) {
				q.mu.Lock()
				agent, conv := q.info.Agent, q.info.AgentSessionID
				q.mu.Unlock()
				// A pane that runs its agent without a terminal, as a
				// chat's does, names no agent: its conversation does.
				if proto.StartsTimeline(agent) || conv != "" {
					pn = q
					break
				}
			}
		}
		d.mu.Unlock()
	}
	var agent, conv, dir, pane string
	takes := false
	if pn != nil {
		pn.mu.Lock()
		agent, conv, dir, pane = pn.info.Agent, pn.info.AgentSessionID, pn.info.Dir, pn.info.ID
		takes = pn.term != nil && pn.proc != nil
		pn.mu.Unlock()
		if pdir := paneDirOf(pn); pdir.Dir != "" {
			dir = pdir.Dir
		}
	}
	// A pane that runs its agent without a terminal, as a Feishu chat's
	// does, names no agent: its conversation's id says which one.
	if agent == "" && conv != "" {
		agent, _ = timeline.FindConv(conv)
	}
	now := time.Now()
	out := proto.Chat{Session: p.Session, Pane: pane, Agent: agent, Read: now.UnixMilli(), Input: takes}
	// A chat drives a pane that runs no terminal, and runs its prompt as a
	// turn, once it has a conversation to go on with.
	out.Turn = !takes && conv != "" && d.feishuBot() != nil
	if pn != nil {
		out.Session = pn.sid
	}
	switch {
	case pn == nil:
		out.Note = locales.InfoTimelineNoPane()
	case !proto.StartsTimeline(agent):
		if agent == "" {
			out.Note = locales.InfoTimelineNoAgent()
		} else {
			out.Note = locales.InfoTimelineUnread(agent)
		}
	}
	if out.Note != "" {
		return out, nil
	}
	out.Lines, out.Model = d.chatLines(agent, conv, dir, now)
	if len(out.Lines) == 0 && out.Model == "" {
		out.Note = locales.InfoTimelineEmpty()
	}
	return out, nil
}

// chatSend sends a prompt to a pane's agent: typed into its terminal where
// it runs one, else as a turn of the chat that drives it, which runs the
// agent in the session's background as the chat's own turns do.
func (d *Daemon) chatSend(p proto.ChatSendParams) (bool, error) {
	pn := d.findPane(p.Pane)
	if pn == nil && p.Session != "" {
		d.mu.Lock()
		if s := d.findSessionLocked(p.Session); s != nil {
			pn = focusedLocked(s)
		}
		d.mu.Unlock()
	}
	if pn == nil {
		return false, os.ErrNotExist
	}
	text := strings.TrimSpace(p.Text)
	if text == "" {
		return false, nil
	}
	pn.mu.Lock()
	runs, sid := pn.term != nil && pn.proc != nil, pn.sid
	pn.mu.Unlock()
	if runs {
		pn.input([]byte(text))
		pn.input([]byte("\r"))
		return true, nil
	}
	b := d.feishuBot()
	if b == nil {
		return false, errors.New("no chat drives this session")
	}
	return true, b.Prompt(sid, text)
}

// chatMax is how many lines of a long conversation a pane shows: the end of
// it.
const chatMax = 400

// chatLines is a pane's conversation, read again only when the records it
// comes from change.
func (d *Daemon) chatLines(agent, conv, dir string, now time.Time) ([]proto.ChatLine, string) {
	file := timeline.Find(agent, conv, dir)
	if file == "" {
		return nil, ""
	}
	size, mod := fileStamp(file)
	if size == 0 {
		return nil, ""
	}
	d.chatMu.Lock()
	seen, ok := d.chatSeen[file]
	d.chatMu.Unlock()
	if ok && seen.size == size && seen.mod == mod {
		return seen.lines, seen.model
	}
	var steps []timeline.Step
	var model string
	var err error
	switch agent {
	case "dsh":
		steps, err = timeline.DSHFile(file, now)
	case "codex":
		steps, err = timeline.CodexFile(file, now)
	case "opencode":
		steps, err = timeline.OpenCode(file, conv, dir, now)
	default:
		steps, err = timeline.ClaudeFile(file, now)
		model = timeline.ClaudeModel(file)
	}
	if err != nil {
		return nil, ""
	}
	lines := chatSteps(steps)
	d.chatMu.Lock()
	if d.chatSeen == nil {
		d.chatSeen = map[string]chatRead{}
	}
	d.chatSeen[file] = chatRead{size: size, mod: mod, lines: lines, model: model}
	d.chatMu.Unlock()
	return lines, model
}

// fileStamp is what a read of a file's records is kept against: its size,
// and its time, and those of the files beside it that hold what it has not
// written into it yet, as SQLite's write-ahead log, which changes while the
// file itself does not.
func fileStamp(path string) (size, mod int64) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		size += st.Size()
		if m := st.ModTime().UnixMilli(); m > mod {
			mod = m
		}
	}
	return size, mod
}

// chatRead is what a read of a file's records gave, kept until the file
// changes.
type chatRead struct {
	size, mod int64
	lines     []proto.ChatLine
	model     string
}

// chatSteps is a timeline's turns as a conversation, oldest first.
func chatSteps(turns []timeline.Step) []proto.ChatLine {
	var out []proto.ChatLine
	var walk func(steps []timeline.Step)
	walk = func(steps []timeline.Step) {
		for _, s := range steps {
			switch s.Kind {
			case timeline.Turn:
				out = append(out, proto.ChatLine{Kind: proto.ChatUser, Text: whole(s), Start: s.Start})
				walk(s.Children)
			case timeline.Exchange:
				walk(s.Children)
			case timeline.Text:
				out = append(out, proto.ChatLine{Kind: proto.ChatAgent, Text: whole(s), Start: s.Start, End: s.End, State: string(s.State)})
			case timeline.Think:
				out = append(out, proto.ChatLine{Kind: proto.ChatThink, Text: whole(s), Start: s.Start, End: s.End, State: string(s.State)})
			case timeline.Tool, timeline.Subagent:
				out = append(out, proto.ChatLine{Kind: proto.ChatTool, Title: s.Title, Detail: whole(s), Start: s.Start, End: s.End, State: string(s.State)})
			default:
				walk(s.Children)
			}
		}
	}
	walk(turns)
	if len(out) > chatMax {
		out = out[len(out)-chatMax:]
	}
	return out
}

// whole is a step's words, or a tool's arguments and output, as the chat
// view shows them: the whole of what the timeline keeps a line of.
func whole(s timeline.Step) string {
	if s.Full != "" {
		return s.Full
	}
	if s.Detail != "" {
		return s.Detail
	}
	return s.Title
}
