package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/subagentsview"
	"hyla/locales"
)

// subagentsDemo gives a test app the subagents of its first session's first
// pane: one that runs, with the step it runs now, and one that ended.
func subagentsDemo(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	s := a.session("s1")
	p := &s.Panes[0]
	p.Agent, p.Title = "claude", "hyla"
	const start = int64(1_761_285_600_000)
	a.Subagents = &subagentsview.Info{Session: "s1", ReadAt: time.Now(), Res: &proto.Subagents{
		Read: start + 130_000,
		Panes: []proto.PaneSubagents{{
			Session: "s1", Pane: p.ID, Agent: "claude",
			Subs: []proto.Subagent{
				{ID: "a1", Type: "Explore", Task: "Look through the store's tests", Tool: "Grep",
					Turn: "Run the tests", Start: start, State: proto.TimelineRunning},
				{ID: "a2", Type: "general-purpose", Task: "Write the changelog",
					Start: start, End: start + 60_000, State: proto.TimelineDone},
			},
		}},
	}}
	return a
}

// TestSubagentsSide lays out the side panel's Subagents view: what the
// pane's agent gave its work to, the step that runs now, and how long each
// ran.
func TestSubagentsSide(t *testing.T) {
	a := subagentsDemo(t)
	a.showSide(sideSubagents)
	tt := ui.NewTester(a.view, 1100, 900)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	shot(t, tt, "subagents")
	for _, want := range []string{
		locales.SideSubagents(),
		locales.SubagentsRunning("1") + " · " + locales.SubagentsDone("1"),
		"Explore", "Look through the store's tests", "Grep",
		"general-purpose", "Write the changelog",
	} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("no %q in the view", want)
		}
	}
}

// A narrow panel names its views by their icons alone, which leaves the
// header's close button its room, and shows the subagents as a wide one
// does.
func TestSubagentsSideNarrow(t *testing.T) {
	a := subagentsDemo(t)
	a.Prefs.SideWidth = 300
	a.showSide(sideSubagents)
	tt := ui.NewTester(a.view, 1100, 900)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	shot(t, tt, "subagents-narrow")
	if _, ok := tt.Find(locales.SideSubagents()); ok {
		t.Error("a narrow panel named its views")
	}
	for _, want := range []string{"Explore", "Look through the store's tests", "Write the changelog"} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("no %q in a narrow view", want)
		}
	}
}
