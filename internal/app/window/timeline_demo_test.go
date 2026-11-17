package window

import (
	"testing"

	"hyla/internal/proto"
	"hyla/internal/ui/timelineview"
)

// timelineDemo gives a test app the timeline of a turn that ended and of
// one that runs, of its first session's focused pane, an agent's.
func timelineDemo(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	s := a.session("s1")
	s.Focus = "a"
	p := s.FocusPane()
	p.Agent = "claude"
	st := a.InfoViews().Of(p)
	st.Dir = "/Users/smallnest/ai/hyla"
	// A turn, at n seconds after the start of the timeline.
	const start = int64(1_761_285_600_000) // 2025-10-25T10:40:00Z
	at := func(n int64) int64 { return start + n*1000 }
	a.SetTimeline(p.ID, &timelineview.Info{
		Pane: p.ID,
		Timely: &proto.Timeline{
			Session: "s1", Pane: p.ID, Agent: "claude", Read: at(154),
			Turns: []proto.TimelineStep{
				{
					Kind: proto.TimelineTurn, Title: "Fix the login bug in the auth package", Start: at(0), End: at(48), State: proto.TimelineDone,
					Children: []proto.TimelineStep{
						{Kind: proto.TimelineExchange, Title: "step", Start: at(1), End: at(48), State: proto.TimelineDone, Children: []proto.TimelineStep{
							{Kind: proto.TimelineThink, Title: "Look at the file first.", Start: at(2), End: at(6), State: proto.TimelineDone},
							{Kind: proto.TimelineTool, Title: "Read", Detail: "/a.go", Start: at(5), End: at(7), State: proto.TimelineDone},
							{Kind: proto.TimelineTool, Title: "Grep", Detail: "func Login", Start: at(9), End: at(13), State: proto.TimelineDone},
							{Kind: proto.TimelineTool, Title: "Bash", Detail: "go test ./auth/", Start: at(16), End: at(41), State: proto.TimelineFailed},
							{Kind: proto.TimelineTool, Title: "Edit", Detail: "auth/login.go", Start: at(42), End: at(44), State: proto.TimelineDone},
						}},
					},
				},
				{
					Kind: proto.TimelineTurn, Title: "Run the full test suite and summarise what you changed", Start: at(52), End: 0, State: proto.TimelineRunning,
					Children: []proto.TimelineStep{
						{Kind: proto.TimelineExchange, Title: "step 1", Start: at(53), End: at(70), State: proto.TimelineDone, Children: []proto.TimelineStep{
							{Kind: proto.TimelineThink, Title: "Plan the tests.", Start: at(54), End: at(58), State: proto.TimelineDone},
							{Kind: proto.TimelineTool, Title: "Bash", Detail: "go test ./...", Start: at(58), End: at(70), State: proto.TimelineDone},
						}},
						{Kind: proto.TimelineExchange, Title: "step 2", Start: at(71), End: 0, State: proto.TimelineRunning, Children: []proto.TimelineStep{
							{Kind: proto.TimelineText, Title: "All auth tests pass now.", Start: at(72), End: at(74), State: proto.TimelineDone},
							{Kind: proto.TimelineTool, Title: "Bash", Detail: "go vet ./...", Start: at(75), End: 0, State: proto.TimelineRunning},
						}},
						{Kind: proto.TimelineSubagent, Title: "Explore", Detail: "the store's tests", Start: at(80), End: 0, State: proto.TimelineRunning},
						{Kind: proto.TimelineTool, Title: "approval: bash", Detail: "rm -rf ./tmp", Start: at(120), End: at(130), State: proto.TimelineFailed},
						{Kind: proto.TimelineTool, Title: "approval: bash", Detail: "escalate sandbox", Start: at(140), End: 0, State: proto.TimelineWaiting},
					},
				},
			},
		},
	})
	return a
}
