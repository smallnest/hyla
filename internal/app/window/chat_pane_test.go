package window

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/proto"
	"hyla/internal/ui/chat"
	"hyla/internal/ui/panes"
	"hyla/locales"
)

// chatDemo gives a test app a conversation in its first session's pane: what
// the user asked, the agent's words, a run of tools that ended, and a turn
// that runs now.
func chatDemo(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	s := a.session("s1")
	p := &s.Panes[0]
	p.AgentSessionID, p.Title = "ses_x", "hyla"
	const start = int64(1_761_285_600_000)
	at := func(n int64) int64 { return start + n*1000 }
	s.Zoomed = p.ID // one pane, as wide as the window: the column floats in it
	a.Side.Chat.Feeds = map[string]*chat.Feed{"s1": {
		At: time.Now(), Got: true,
		Chat: proto.Chat{Session: "s1", Pane: p.ID, Agent: "claude", Read: at(400), Model: "claude-opus-5", Input: true,
			Lines: []proto.ChatLine{
				{Kind: proto.ChatUser, Text: "打开详情页点播放，进度条半天不走，要不要加 loading 提示？", Start: at(0)},
				{Kind: proto.ChatAgent, Text: "I'll put the loading note under the controls, so it doesn't squeeze the seek bar.", Start: at(2), End: at(6)},
				{Kind: proto.ChatTool, Title: "Bash", Detail: "grep -rn loading src", Start: at(6), End: at(9), State: proto.TimelineDone},
				{Kind: proto.ChatTool, Title: "Read", Detail: "src/player.ts", Start: at(9), End: at(11), State: proto.TimelineDone},
				{Kind: proto.ChatTool, Title: "Edit", Detail: "src/player.ts", Start: at(11), End: at(14), State: proto.TimelineDone},
				{Kind: proto.ChatAgent, Text: "Done. The note shows while the stream starts.", Start: at(14), End: at(18)},
				{Kind: proto.ChatUser, Text: "CI 过了就合并，然后打 tag 发版", Start: at(20)},
				{Kind: proto.ChatTool, Title: "Bash", Detail: "gh pr merge --squash", Start: at(22), State: proto.TimelineRunning},
			}},
	}}
	return a
}

// TestChatBody lays out the Chat view: the user's prompts to the right, the
// agent's words on the column, a run of tools folded to one row, the step
// that runs now with its stop, and the composer under it.
func TestChatBody(t *testing.T) {
	a := chatDemo(t)
	tt := ui.NewTester(a.view, 1280, 760)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	shot(t, tt, "chat")
	for _, want := range []string{
		"打开详情页点播放，进度条半天不走，要不要加 loading 提示？",
		"I'll put the loading note under the controls, so it doesn't squeeze the seek bar.",
		locales.ChatRanTools("3"),
		"Done. The note shows while the stream starts.",
		"Claude Opus 5",
	} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("no %q in the chat", want)
		}
	}
}

// A pane shows its session's conversation in its terminal's place once its
// agent runs one, with no toggle: a chat that drives a pane without a
// terminal gives it that, and a plain shell keeps its terminal.
func TestPanesWithAChat(t *testing.T) {
	driven := &proto.Pane{AgentSessionID: "ses_x"}
	if !panes.HasChat(driven) {
		t.Error("a pane whose agent runs without a terminal shows no chat")
	}
	for _, p := range []*proto.Pane{{Shell: "zsh"}, {Agent: "claude", AgentSessionID: "ses_x"}} {
		if panes.HasChat(p) {
			t.Errorf("the pane %+v shows a chat, not its terminal", p)
		}
	}
}

// The composer takes a prompt, and Enter sends it to the pane's program.
func TestChatComposerSends(t *testing.T) {
	a, f := chatApp(t)
	// The pane alone, so the composer is under the conversation it sends to.
	a.session("s1").Zoomed = "a"
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	chatSettle(tt, func() bool { return tt.HasText("Fix the build") })
	// The composer is at the foot of the pane: a click gives it the keys.
	if r, ok := tt.Find("Claude Opus 5"); ok {
		tt.ClickAt(r.X-300, r.Y+r.H/2)
	} else {
		tt.ClickAt(800, 800-30)
	}
	tt.Frame()
	tt.Type("hello")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	chatSettle(tt, func() bool { return strings.Contains(f.typed(), "hello\r") })
	if got := f.typed(); !strings.Contains(got, "hello\r") {
		t.Errorf("the pane's program got %q, want the prompt and Enter", got)
	}
}

// A pane that runs no terminal, as a chat's does, takes no prompt here: it
// says where the prompt goes instead, and has no composer.
func TestChatNoInput(t *testing.T) {
	a := chatDemo(t)
	f := a.Side.Chat.Feeds["s1"]
	f.Chat.Input = false
	tt := ui.NewTester(a.view, 1280, 760)
	for i := 0; i < 20 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	if _, ok := tt.Find(locales.ChatNoInput()); !ok {
		t.Errorf("the pane says nothing of where a prompt goes: %v", tt.Texts())
	}
	if _, ok := tt.Find("Claude Opus 5"); ok {
		t.Error("a pane that takes no prompt shows the composer")
	}
}

// A prompt that did not go says why, under the box, and keeps its words.
func TestChatSendFails(t *testing.T) {
	a := chatDemo(t)
	f := a.Side.Chat.Feeds["s1"]
	f.Draft = "hello"
	// A daemon that answers no: the pane takes no prompt.
	a.Local().Client = nil
	a.SendPane("s1", f.Chat.Pane, "hello")
	if f.SendErr == "" {
		t.Fatal("a prompt that did not go says nothing")
	}
	if f.Draft != "hello" {
		t.Errorf("the prompt went with the failure: draft %q", f.Draft)
	}
	tt := ui.NewTester(a.view, 1280, 760)
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find(locales.ChatSendFailed(f.SendErr)); !ok {
		t.Errorf("the view says nothing of the failure: %v", tt.Texts())
	}
}

// The vertical rail on the chat's left edge shows a tick per turn, previews a
// turn's prompt under the pointer, and scrolls the conversation to the turn a
// click picks.
func TestChatTimelineRail(t *testing.T) {
	a := chatDemo(t)
	f := a.Side.Chat.Feeds["s1"]
	const start = int64(1_761_285_600_000)
	var ls []proto.ChatLine
	for i := 1; i <= 18; i++ {
		n := strconv.Itoa(i)
		t0 := start + int64(i)*60_000
		ls = append(ls,
			proto.ChatLine{Kind: proto.ChatUser, Text: "Prompt turn #" + n + ": check the logs and configuration", Start: t0},
			proto.ChatLine{Kind: proto.ChatAgent, Text: "Answer for turn #" + n + ".\n\nLine two of the response.\n\nLine three of the response.", Start: t0 + 1000, End: t0 + 4000},
		)
	}
	f.Chat.Lines = ls

	var railBox ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		railBox = f.Rail.Bounds(c)
	}, 1280, 760)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()

	if railBox.W <= 0 || railBox.H <= 0 {
		t.Fatalf("the turn rail has no bounds: %+v", railBox)
	}
	if !tt.HasText("Prompt turn #18") {
		t.Fatalf("chat did not start at the newest turn: %v", tt.Texts())
	}
	if tt.HasText("Prompt turn #1:") {
		t.Fatalf("oldest turn should be scrolled above view initially")
	}

	// Hovering the first tick shows its prompt in a preview card without scrolling.
	firstX := railBox.X + 10
	firstY := railBox.Y + 4 + 4
	tt.Move(firstX, firstY)
	tt.Frame()
	tt.Advance(150 * time.Millisecond)
	tt.Frame()
	shot(t, tt, "chat-rail-hover")
	if !tt.HasText("Prompt turn #1: check the logs and configuration") {
		t.Errorf("hovering the first tick did not preview its prompt: %v", tt.Texts())
	}

	// Clicking the first tick scrolls the chat to the first turn.
	tt.ClickAt(firstX, firstY)
	for i := 0; i < 20 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(100 * time.Millisecond)
	}
	tt.Move(600, 400)
	tt.Frame()
	shot(t, tt, "chat-rail-top")
	if !tt.HasText("Prompt turn #1: check the logs and configuration") {
		t.Errorf("clicking the first tick did not scroll to turn #1: %v", tt.Texts())
	}
	if tt.HasText("Prompt turn #18:") {
		t.Errorf("newest turn is still in view after jumping to turn #1")
	}

	// Clicking the last tick returns to the newest turn.
	lastY := railBox.Y + railBox.H - 4 - 4
	tt.ClickAt(firstX, lastY)
	for i := 0; i < 20 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(100 * time.Millisecond)
	}
	tt.Move(600, 400)
	tt.Frame()
	tt.SetDark(true)
	for i := 0; i < 10 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(100 * time.Millisecond)
	}
	shot(t, tt, "chat-rail-dark")
	if !tt.HasText("Prompt turn #18: check the logs and configuration") {
		t.Errorf("clicking the last tick did not scroll back to turn #18: %v", tt.Texts())
	}
}

// The palette toggles the chat in a terminal's pane, and only the palette:
// the action is in no menu, has no shortcut, and a pane whose agent runs
// without a terminal shows the conversation anyway.
func TestToggleChatFromThePalette(t *testing.T) {
	a := testApp(t)
	s := a.session("s1")
	s.Focus = "a"
	a.Selected = "s1"
	a.toggleChat()
	if !a.PaneChatMap["a"] {
		t.Fatal("the pane shows no chat")
	}
	a.toggleChat()
	if a.PaneChatMap["a"] {
		t.Error("the pane kept the chat")
	}
	s.Panes[0].AgentSessionID = "ses_x"
	a.toggleChat()
	if a.PaneChatMap["a"] {
		t.Error("a pane without a terminal toggled")
	}

	for _, act := range actions.List() {
		if act.ID != "pane.chat" {
			continue
		}
		if act.Menu != "" || act.NoPalette || actions.Keys(act, nil) != "" {
			t.Errorf("the toggle is %+v: it belongs to the palette alone", act)
		}
		return
	}
	t.Error("the palette has no toggle")
}
