package proto

// The chat view: a session's conversation with its agent, as a pane shows
// it — what the user asked, what the agent wrote and reasoned, and the
// tools it ran — from the records the agent keeps beside its conversation
// (chat.get), whatever chat it is in.

// ChatParams asks for a pane's conversation (chat.get): the pane named,
// else the session's pane that runs an agent.
type ChatParams struct {
	Session string `json:"session"`
	Pane    string `json:"pane,omitempty"`
}

// Chat is a pane's conversation, oldest first, and the word where the
// pane's agent keeps none Hyla reads. Read is unix ms, when hylad read the
// records.
type Chat struct {
	Session string     `json:"session"`
	Pane    string     `json:"pane,omitempty"`
	Agent   string     `json:"agent,omitempty"`
	Read    int64      `json:"read"`
	Model   string     `json:"model,omitempty"`
	Lines   []ChatLine `json:"lines,omitempty"`
	Note    string     `json:"note,omitempty"`
}

// A chat line's kind, as Chat's lines name it.
const (
	ChatUser  = "user"  // what the user asked
	ChatAgent = "agent" // what the agent wrote
	ChatThink = "think" // what it reasoned
	ChatTool  = "tool"  // a tool it ran, or an agent it gave its work to
)

// ChatLine is a line of a conversation: what the user asked, what the agent
// wrote or reasoned, or a tool it ran, from when to when.
type ChatLine struct {
	Kind string `json:"kind"`
	// Text is the words whole, for what the user and the agent said and
	// reasoned.
	Text string `json:"text,omitempty"`
	// Title is a tool's name, or what it ran, and Detail its arguments or
	// its output, which a view folds away.
	Title  string `json:"title,omitempty"`
	Detail string `json:"detail,omitempty"`
	Start  int64  `json:"start"`         // unix ms
	End    int64  `json:"end,omitempty"` // unix ms, 0 while it runs
	State  string `json:"state,omitempty"`
}
