package daemon

import (
	"os"
	"time"

	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/timeline"
	"hyla/locales"
)

// timelineOf is a pane's timeline, from the records its agent keeps beside
// its conversation. An agent whose records the timeline does not read has
// a note in place of the turns.
func (d *Daemon) timelineOf(p proto.TimelineParams) (proto.Timeline, error) {
	pn := d.findPane(p.Pane)
	if pn == nil && p.Pane != "" {
		return proto.Timeline{}, os.ErrNotExist
	}
	var agent, conv, dir string
	if pn != nil {
		pn.mu.Lock()
		agent, conv, dir = pn.info.Agent, pn.info.AgentSessionID, pn.info.Dir
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
	at := now.UnixMilli()
	t := proto.Timeline{Session: p.Session, Agent: agent, Read: at}
	if pn != nil {
		t.Pane = pn.info.ID
	}
	var steps []timeline.Step
	var err error
	words := locales.In(settings.LanguageOf(settings.Load().Language))
	switch {
	case pn == nil:
		t.Note = words.InfoTimelineNoPane()
	case !proto.StartsTimeline(agent):
		if agent == "" {
			t.Note = words.InfoTimelineNoAgent()
		} else {
			t.Note = words.InfoTimelineUnread(agent)
		}
	default:
		if tl, handled, perr := d.pluginTimeline(p, agent, conv, dir, pn); handled {
			return tl, perr
		}
		file := timeline.Find(agent, conv, dir)
		if file == "" {
			t.Note = words.InfoTimelineNone(dir)
			break
		}
		switch agent {
		case "dsh":
			steps, err = timeline.DSHFile(file, now)
		case "codex":
			steps, err = timeline.CodexFile(file, now)
		case "opencode":
			steps, err = timeline.OpenCode(file, conv, dir, now)
		case "agy", "antigravity":
			steps, err = timeline.AgyFile(file, now)
		default:
			steps, err = timeline.ClaudeFile(file, now)
		}
	}
	if err != nil {
		return proto.Timeline{}, err
	}
	t.Turns = timelineSteps(steps)
	return t, nil
}

// timelineSteps is steps as the app reads them.
func timelineSteps(steps []timeline.Step) []proto.TimelineStep {
	out := make([]proto.TimelineStep, len(steps))
	for i, s := range steps {
		out[i] = proto.TimelineStep{
			Kind:     string(s.Kind),
			Title:    s.Title,
			Detail:   s.Detail,
			Start:    s.Start,
			End:      s.End,
			State:    string(s.State),
			Children: timelineSteps(s.Children),
		}
	}
	return out
}
