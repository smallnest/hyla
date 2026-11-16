package proto

// The agent's timeline, as the Info view shows it: what a session's coding
// agent did, from the records its agent keeps beside its conversation
// (timeline.get).

// TimelineParams asks for a pane's timeline (timeline.get).
type TimelineParams struct {
	Session string `json:"session"`
	Pane    string `json:"pane,omitempty"`
}

// Timeline is a pane's timeline: its agent, and the turns it ran, oldest
// first. Read is unix ms, when hylad read the records; Note says why the
// timeline is what it is when the records tell no more, as an agent whose
// records hylad does not read.
type Timeline struct {
	Session string         `json:"session"`
	Pane    string         `json:"pane,omitempty"`
	Agent   string         `json:"agent,omitempty"`
	Read    int64          `json:"read"`
	Turns   []TimelineStep `json:"turns"`
	Note    string         `json:"note,omitempty"`
}

// A step's kind, as Timeline's steps name it.
const (
	TimelineTurn     = "turn"     // what the user asked, and the agent's work on it
	TimelineExchange = "step"     // one of a turn's exchanges with the model
	TimelineTool     = "tool"     // a tool the agent ran
	TimelineThink    = "think"    // the agent's reasoning
	TimelineText     = "text"     // what the agent wrote
	TimelineSubagent = "subagent" // an agent the agent started for its work
)

// A step's state, as Timeline's steps name it.
const (
	TimelineDone    = "done"
	TimelineRunning = "running"
	TimelineWaiting = "waiting" // blocked on the user
	TimelineFailed  = "failed"
)

// TimelineStep is one thing an agent did, from when it started to when it
// ended: a turn, and the steps in it, each with the steps it holds.
type TimelineStep struct {
	Kind     string         `json:"kind"`
	Title    string         `json:"title,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Start    int64          `json:"start"` // unix ms
	End      int64          `json:"end"`   // unix ms, 0 while it runs
	State    string         `json:"state"`
	Children []TimelineStep `json:"children,omitempty"`
}
