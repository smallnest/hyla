// Package paneinfo reads what the Info view shows of a pane: where it
// works, the processes of its terminal, and the ports they listen on.
package paneinfo

import (
	"time"

	"hyla/internal/client"
	"hyla/internal/proto"
)

const (
	// Idle is the time between the reads of a pane's Info view whose
	// shell ran nothing yet.
	Idle = 3 * time.Second

	// Tick is the time between the reads of a pane's working directory;
	// ProcsTick the time between two listings of its processes, which
	// carry their ports, from as slow a tool as lsof.
	// Cached bounds the views kept.
	Tick      = time.Second
	ProcsTick = 2 * time.Second
	Cached    = 32
)

// State is what the Info view shows of a pane, kept by the app across the
// builds and read again as the view shows.
type State struct {
	ReadAt   time.Time
	Busy     bool   // a read is under way
	Shell    int    // the pane's shell, as it was read of
	Dir      string // the working directory, as the header shows it
	Procs    []proto.Process
	ProcsAt  bool      // the processes were listed
	ListedAt time.Time // when they were
	Ports    int
	ListErr  string // why the processes did not list
	// Feishu is the bot's state, as hylad last told it, and FeishuAt when
	// it was; FeishuGot tells that a read of it is under way.
	Feishu    *proto.FeishuState
	FeishuAt  time.Time
	FeishuGot bool
}

// panes are the states of the panes, by pane.
type panes map[string]*State

// Panes keeps the Info views' states, by pane.
type Panes struct {
	byPane panes
}

// Of returns a pane's Info view, made for the pane as it is now.
func (p *Panes) Of(pane *proto.Pane) *State {
	st := p.byPane[pane.ID]
	if st != nil && st.Shell != pane.PID {
		st = nil
	}
	if st != nil && st.Shell <= 0 && pane.PID > 0 {
		// The shell starts as the view first shows: the reads look again.
		st.Shell, st.ReadAt, st.ProcsAt = pane.PID, time.Time{}, false
	}
	if st == nil {
		if p.byPane == nil {
			p.byPane = panes{}
		}
		if len(p.byPane) >= Cached {
			for id := range p.byPane {
				if id != pane.ID {
					delete(p.byPane, id)
				}
			}
		}
		st = &State{Shell: pane.PID}
		p.byPane[pane.ID] = st
	}
	return st
}

// Read reads a pane's working directory from its host's files, as dir
// gives its working directory and cl its hylad. Update runs fn on the
// main thread.
func Read(st *State, cl *client.Client, dir string, update func(func())) {
	if cl == nil || st.Busy {
		return
	}
	st.ReadAt, st.Busy = time.Now(), true
	go func() {
		abs := readDir(cl, dir)
		if abs == "" {
			abs = dir
		}
		update(func() {
			st.ReadAt, st.Busy, st.Dir = time.Now(), false, abs
		})
	}()
}

// readDir returns the working directory a host lists, as it is: a
// directory gone is "".
func readDir(cl *client.Client, dir string) string {
	if dir == "" {
		return ""
	}
	var l proto.FSList
	if cl.Call("fs.list", proto.FSListParams{Dir: dir}, &l) != nil {
		return ""
	}
	return l.Dir
}

// ReadProcs lists a pane's processes, and the ports they hold. Update
// runs fn on the main thread.
func ReadProcs(st *State, cl *client.Client, pane *proto.Pane, update func(func())) {
	if cl == nil || pane.PID <= 0 {
		return
	}
	go func() {
		var procs []proto.Process
		err := cl.Call("pane.processes", proto.ProcessesParams{PID: pane.PID}, &procs)
		update(func() {
			if err != nil {
				st.ProcsAt, st.ListedAt, st.ListErr = true, time.Now(), err.Error()
				return
			}
			st.ProcsAt, st.ListedAt, st.Procs, st.ListErr, st.Ports = true, time.Now(), procs, "", 0
			for _, pr := range procs {
				st.Ports += len(pr.Ports)
			}
		})
	}()
}
