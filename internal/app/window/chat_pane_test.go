package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/chat"
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
	p.Agent, p.Title = "claude", "hyla"
	const start = int64(1_761_285_600_000)
	at := func(n int64) int64 { return start + n*1000 }
	a.PaneChatMap = map[string]bool{p.ID: true}
	a.Side.Chat.Feeds = map[string]*chat.Feed{"s1": {
		At: time.Now(), Got: true,
		Chat: proto.Chat{Session: "s1", Pane: p.ID, Agent: "claude", Read: at(400), Model: "claude-opus-5",
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
