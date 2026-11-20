package daemon

import (
	"os"
	"sort"
	"time"

	"hyla/internal/proto"
	"hyla/internal/timeline"
	"hyla/locales"
)

// subagentSet applies a hook's word of a subagent of the pane's agent: it
// began, or it ended.
func (d *Daemon) subagentSet(p proto.SubagentParams) (bool, error) {
	if p.ID == "" {
		return false, nil
	}
	pn := d.findPane(p.Pane)
	if pn == nil {
		// The pane that runs the agent's session, as its hook tells it.
		pn, _ = d.agentPane(p.PID, p.Agent, p.Dir)
	}
	if pn == nil {
		return false, os.ErrNotExist
	}
	now := time.Now().UnixMilli()
	pn.mu.Lock()
	defer pn.mu.Unlock()
	if pn.subs == nil {
		pn.subs = map[string]proto.Subagent{}
	}
	s, known := pn.subs[p.ID]
	switch p.Op {
	case "stop":
		if !known {
			return false, nil
		}
		s.End, s.State, s.Tool = now, proto.TimelineDone, ""
	default:
		if !known {
			s = proto.Subagent{ID: p.ID, Start: now}
		}
		s.Type, s.Task, s.Path = p.Type, p.Task, p.Path
		s.State, s.End, s.Tool = proto.TimelineRunning, 0, ""
	}
	pn.subs[p.ID] = s
	pruneSubagents(pn.subs)
	return true, nil
}

// pruneSubagents bounds a pane's subagents to the newest, keeping what
// runs and the ones that ended last.
const pruneSubagentsAt = 24

func pruneSubagents(subs map[string]proto.Subagent) {
	if len(subs) <= pruneSubagentsAt {
		return
	}
	ended := make([]proto.Subagent, 0, len(subs))
	for _, s := range subs {
		if s.End != 0 {
			ended = append(ended, s)
		}
	}
	sort.SliceStable(ended, func(i, j int) bool { return ended[i].End < ended[j].End })
	for i := 0; i < len(subs)-pruneSubagentsAt && i < len(ended); i++ {
		delete(subs, ended[i].ID)
	}
}

// subagentsOf returns what the panes' agents gave their work to, from
// their hooks while they run, and from the records they keep where no hook
// came.
func (d *Daemon) subagentsOf(p proto.SubagentsParams) (proto.Subagents, error) {
	now := time.Now()
	out := proto.Subagents{Read: now.UnixMilli(), Panes: []proto.PaneSubagents{}}
	d.mu.Lock()
	var panes []*pane
	if p.Session == "" {
		for _, s := range d.sessions {
			panes = append(panes, s.panes...)
		}
	} else if s := d.findSessionLocked(p.Session); s != nil {
		panes = append(panes, s.panes...)
	}
	d.mu.Unlock()
	for _, pn := range panes {
		ps := d.paneSubagents(pn, now)
		if len(ps.Subs) == 0 && ps.Note == "" {
			continue
		}
		out.Panes = append(out.Panes, ps)
	}
	return out, nil
}

// paneSubagents is a pane's agent's subagents: the ones its hook told of,
// else the ones the agent's records tell.
func (d *Daemon) paneSubagents(pn *pane, now time.Time) proto.PaneSubagents {
	pn.mu.Lock()
	agent, conv, dir, id := pn.info.Agent, pn.info.AgentSessionID, pn.info.Dir, pn.info.ID
	live := make([]proto.Subagent, 0, len(pn.subs))
	for _, s := range pn.subs {
		live = append(live, s)
	}
	pn.mu.Unlock()
	out := proto.PaneSubagents{Session: pn.sid, Pane: id, Agent: agent}
	switch {
	case agent == "":
		return out
	case !proto.StartsTimeline(agent) && !pluginHasSubagents(agent):
		out.Note = locales.InfoTimelineUnread(agent)
		return out
	case len(live) > 0:
		sortSubagents(live)
		for i := range live {
			// What a subagent runs now, from its own transcript: the
			// hook tells when it began and ended, not what it does.
			if live[i].End == 0 {
				live[i].Tool = timeline.SubagentNow(live[i].Path)
			}
		}
		out.Subs = newest(live)
		return out
	}
	if ps, handled := d.pluginSubagents(pn, agent, conv, dir); handled {
		return ps
	}
	out.Subs = newest(d.recordedSubagents(agent, conv, dir, now))
	return out
}

func pluginHasSubagents(agent string) bool {
	meta, ok := proto.AgentProvider(agent)
	return ok && meta.PluginID != "" && meta.Subagents
}

// recordedSubagents is the subagents a pane's agent gave its work to, as
// its records tell them, read again only when the records change.
func (d *Daemon) recordedSubagents(agent, conv, dir string, now time.Time) []proto.Subagent {
	file := timeline.Find(agent, conv, dir)
	if file == "" {
		return nil
	}
	size, mod := fileStamp(file)
	if size == 0 {
		return nil
	}
	d.subsMu.Lock()
	seen, ok := d.subsSeen[file]
	d.subsMu.Unlock()
	if ok && seen.size == size && seen.mod == mod {
		return seen.subs
	}
	var steps []timeline.Step
	var err error
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
	if err != nil {
		return nil
	}
	found := timeline.Subagents(steps)
	subs := make([]proto.Subagent, 0, len(found))
	for _, s := range found {
		subs = append(subs, proto.Subagent{
			Type: s.Type, Task: s.Task, Turn: s.Turn,
			Start: s.Start, End: s.End, State: string(s.State),
		})
	}
	sortSubagents(subs)
	d.subsMu.Lock()
	if d.subsSeen == nil {
		d.subsSeen = map[string]subsRead{}
	}
	d.subsSeen[file] = subsRead{size: size, mod: mod, subs: subs}
	d.subsMu.Unlock()
	return subs
}

// subsRead is what a read of a file's records gave, kept until the file
// changes.
type subsRead struct {
	size, mod int64
	subs      []proto.Subagent
}

// shownSubagents bounds what a pane lists: what runs, and the ones that
// ended last.
const shownSubagents = 20

// newest is the subagents a pane lists, out of all it worked with.
func newest(subs []proto.Subagent) []proto.Subagent {
	if len(subs) <= shownSubagents {
		return subs
	}
	return subs[:shownSubagents]
}

// sortSubagents puts the ones that run first, then the newest first.
func sortSubagents(subs []proto.Subagent) {
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
}
