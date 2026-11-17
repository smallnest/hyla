package window

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
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

// The View menu carries the side panel's views, the Subagents one too: a
// view no menu opens is one the user cannot find.
func TestViewMenuSubagents(t *testing.T) {
	mygo.Use(updater.Plugin)
	a := testApp(t)
	m := a.Menu()
	for _, id := range []string{"subagents.open", "info.toggle", "files.toggle", "changes.toggle"} {
		if m.ItemByID(id) == nil {
			t.Errorf("no %s in the menus", id)
		}
	}
}

// TestSubagentsOpenSplits splits a subagents pane beside the pane the user
// is in, which the layout, the zoom and the closing treat as any other.
func TestSubagentsOpenSplits(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	got := make(chan proto.CreateParams, 2)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "split":
			var p proto.CreateParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			got <- p
			return proto.Session{ID: "s1"}, nil
		case "focus":
			return nil, nil
		}
		return nil, errors.New("unexpected method " + method)
	})
	a.Local().Client = cl
	defer stop()

	a.run("subagents.open", nil)
	select {
	case p := <-got:
		if p.Split != "a" || !p.Subagents || p.Direction != "right" || p.Timeline != "" {
			t.Fatalf("Subagents split %+v, want a subagents pane beside a", p)
		}
	case <-time.After(time.Second):
		t.Fatal("Subagents split no pane")
	}
}

// A session whose subagents pane is open already takes the user to it,
// rather than splitting a second one.
func TestSubagentsOpenGoesToItsPane(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "sub", Subagents: true, Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "sub", "right", 0)
	s.Focus = "a"
	split := make(chan struct{}, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "split":
			split <- struct{}{}
			return proto.Session{ID: "s1"}, nil
		case "focus":
			return nil, nil
		}
		return nil, errors.New("unexpected method " + method)
	})
	a.Local().Client = cl
	defer stop()

	a.run("subagents.open", nil)
	select {
	case <-split:
		t.Fatal("Subagents split a second pane beside a's")
	case <-time.After(50 * time.Millisecond):
	}
	if got := a.Focus["s1"]; got != "sub" {
		t.Errorf("the user is in %q, want the subagents pane", got)
	}
}

// TestSubagentsPaneDemo lays out a session with a subagents pane beside its
// agent, for a look at it with HYLA_SHOTS.
func TestSubagentsPaneDemo(t *testing.T) {
	a := subagentsDemo(t)
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "sub", Subagents: true, Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "sub", "right", 0)
	s.Focus = "a"
	tt := ui.NewTester(a.view, 1280, 720)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	shot(t, tt, "subagents-pane")
	for _, want := range []string{locales.SideSubagents(), "Explore", "Look through the store's tests", "Write the changelog"} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("no %q in the subagents pane", want)
		}
	}
}
