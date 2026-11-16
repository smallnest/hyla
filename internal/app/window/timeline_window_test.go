package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/locales"
)

// TestTimelineWindowDemo lays out the timeline's window, for a look at it
// with HYLA_SHOTS.
func TestTimelineWindowDemo(t *testing.T) {
	a := timelineDemo(t)
	a.TimelineMark = "claude"
	tt := ui.NewTester(a.timelineWindow, 680, 760)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	shot(t, tt, "timeline-window")
	if _, ok := tt.Find(locales.InfoTimelineTurns("2")); ok {
		t.Error("the window shows the turns' count, which its header leaves out")
	}
}

// A window of a pane that runs no agent whose records Hyla reads says
// why, beside the agent's name.
func TestTimelineWindowNoAgent(t *testing.T) {
	a := testApp(t)
	a.TimelineMark = "gemini"
	s := a.session("s1")
	s.FocusPane().Agent = "gemini"
	tt := ui.NewTester(a.timelineWindow, 680, 760)
	tt.Frame()
	if _, ok := tt.Find(locales.InfoTimelineUnread("gemini")); !ok {
		t.Error("the window shows no word of an agent whose records Hyla does not read")
	}
}
