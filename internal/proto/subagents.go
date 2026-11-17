package proto

// The coding agents' subagents, as the side panel's Subagents view shows
// them: the agents an agent gave its work to, as its hook tells hylad while
// they run (pane.subagent) and as its records tell where it keeps them
// (subagents.get).

// SubagentsParams asks for the subagents of a session's panes, or of every
// pane when Session is "" (subagents.get).
type SubagentsParams struct {
	Session string `json:"session,omitempty"`
}

// Subagents is what the panes' agents gave their work to, a pane after a
// pane; Read is unix ms, when hylad read them.
type Subagents struct {
	Read  int64           `json:"read"`
	Panes []PaneSubagents `json:"panes"`
}

// PaneSubagents is a pane's agent's subagents, the newest first, and the
// word where it has none to show: no agent, or one whose records Hyla does
// not read.
type PaneSubagents struct {
	Session string     `json:"session"`
	Pane    string     `json:"pane"`
	Agent   string     `json:"agent,omitempty"`
	Note    string     `json:"note,omitempty"`
	Subs    []Subagent `json:"subs,omitempty"`
}

// Subagent is one agent a pane's agent gave work to: what it was asked,
// what it is, from when to when, and the step it runs now while it runs.
type Subagent struct {
	// ID is the subagent's own id, which its hook tells, "" when the
	// records alone named it.
	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"` // Explore, general-purpose, as the agent names it
	Task string `json:"task,omitempty"` // what it was asked, in one line
	Tool string `json:"tool,omitempty"` // the step it runs now, while it runs
	Turn string `json:"turn,omitempty"` // the turn of the agent's that gave it the work
	// Path is the subagent's own transcript, when its agent keeps one:
	// what tells the step it runs now.
	Path  string `json:"path,omitempty"`
	Start int64  `json:"start"`         // unix ms
	End   int64  `json:"end,omitempty"` // unix ms, 0 while it runs
	State string `json:"state"`         // TimelineRunning, TimelineDone or TimelineFailed
}

// SubagentParams tells hylad of a subagent's start or end, from the pane's
// agent's hook (hyla hook): the agent gave its work to another.
type SubagentParams struct {
	ID      string `json:"id"`                // the subagent's id, its agent's
	Pane    string `json:"pane,omitempty"`    // the pane the hook runs in
	Agent   string `json:"agent,omitempty"`   // the agent that started it
	Session string `json:"session,omitempty"` // the agent's own session, which names the pane
	PID     int    `json:"pid,omitempty"`     // the hook's process, which names the pane
	Dir     string `json:"dir,omitempty"`
	Op      string `json:"op"`             // "start" or "stop"
	Type    string `json:"type,omitempty"` // agent_type
	Task    string `json:"task,omitempty"` // agent_prompt, in one line
	Path    string `json:"path,omitempty"` // the subagent's own transcript
}
