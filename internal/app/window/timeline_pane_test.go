package window

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/timelineview"
	"hyla/locales"
)

// The menu's Timeline splits a pane beside the one the user is in,
// showing that pane's timeline: the daemon makes it as a pane, which the
// layout, the zoom and the closing treat as any other.
func TestTimelineOpenSplits(t *testing.T) {
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

	a.run("timeline.open", nil)
	select {
	case p := <-got:
		if p.Split != "a" || p.Timeline != "a" || p.Direction != "right" {
			t.Fatalf("Timeline split %+v, want a's timeline beside a", p)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeline split no pane")
	}
}

// A pane whose timeline is open already takes the user to it, rather than
// splitting a second one.
func TestTimelineOpenGoesToItsPane(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "t", Timeline: "a", Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "t", "right", 0)
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

	a.run("timeline.open", nil)
	select {
	case <-split:
		t.Fatal("Timeline split a second pane beside a's")
	case <-time.After(50 * time.Millisecond):
	}
	if got := a.Focus["s1"]; got != "t" {
		t.Errorf("the user is in %q, want the pane a's timeline is in", got)
	}
}

// In a timeline whose pane closed, Timeline takes the session's own pane,
// rather than showing the timeline itself.
func TestTimelineOpenInATimeline(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "t", Timeline: "gone", Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "t", "right", 0)
	s.Focus = "t"
	got := make(chan proto.CreateParams, 1)
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

	a.run("timeline.open", nil)
	select {
	case p := <-got:
		if p.Timeline != "a" || p.Split != "a" {
			t.Fatalf("Timeline shows %+v, want the session's pane a and its timeline beside it", p)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeline split no pane")
	}
}

// TestTimelinePaneDemo lays out a session with a timeline pane beside its
// agent, for a look at it with HYLA_SHOTS.
func TestTimelinePaneDemo(t *testing.T) {
	a := timelineDemo(t)
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "t", Timeline: "a", Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "t", "right", 0)
	s.Focus = "a"
	tt := ui.NewTester(a.view, 1280, 720)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	shot(t, tt, "timeline-pane")
	if _, ok := tt.Find("Fix the login bug in the auth package"); !ok {
		t.Errorf("the timeline pane shows no turn; texts %q", tt.Texts())
	}
}

// A timeline pane whose pane runs no agent whose records Hyla reads says
// why, in place of the turns.
func TestTimelinePaneNoAgent(t *testing.T) {
	a := testApp(t)
	s := a.session("s1")
	s.Panes[0].Agent = "gemini"
	s.Panes = append(s.Panes, proto.Pane{ID: "t", Timeline: "a", Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "t", "right", 0)
	tt := ui.NewTester(a.view, 1280, 720)
	tt.Frame()
	if _, ok := tt.Find(locales.InfoTimelineUnread("gemini")); !ok {
		t.Errorf("the timeline pane shows no word of an agent whose records Hyla does not read; texts %q", tt.Texts())
	}
}

// A pane that works again reads its timeline at once, though its last turn
// ended: the timeline shows the new turn within a tick, not at the end of
// the idle.
func TestTimelineReadsWhenTheAgentWorks(t *testing.T) {
	a := testApp(t)
	s := a.session("s1")
	s.Panes[0].Agent = "claude"
	s.Panes = append(s.Panes, proto.Pane{ID: "t", Timeline: "a", Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "t", "right", 0)
	reads := make(chan string, 4)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		if method == "timeline.get" {
			var p proto.TimelineParams
			json.Unmarshal(raw, &p)
			reads <- p.Pane
		}
		return nil, nil
	})
	a.Local().Client = cl
	defer stop()
	// A turn that ended five seconds ago: an idle pane would read it in
	// thirty.
	read := func(status proto.Status) bool {
		a.SetTimeline("a", &timelineview.Info{Pane: "a", ReadAt: time.Now().Add(-5 * time.Second)})
		a.session("s1").Panes[0].Status = status
		tt := ui.NewTester(a.view, 1280, 720)
		tt.Frame()
		tt.Frame()
		select {
		case <-reads:
			return true
		case <-time.After(300 * time.Millisecond):
			return false
		}
	}
	if !read(proto.Working) {
		t.Error("a working pane's timeline was not read")
	}
	if read(proto.Idle) {
		t.Error("an idle pane's timeline was read again too soon")
	}
}

// A timeline pane whose pane closed shows the session's own pane, rather
// than nothing.
func TestTimelinePaneWithoutItsPane(t *testing.T) {
	a := testApp(t)
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "t", Timeline: "gone", Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "t", "right", 0)
	shown := a.timelineShown(s, s.PaneByID("t"))
	if shown == nil || shown.ID != "a" {
		t.Fatalf("the timeline shows %v, want the session's pane a", shown)
	}
}
