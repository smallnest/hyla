package window

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"hyla/internal/app/host"
	"hyla/internal/plugins/uihost"
	"hyla/internal/proto"
	"hyla/locales"
)

func TestMatchesWhenAgentForkable(t *testing.T) {
	sess := &proto.Session{ID: "s1"}
	cases := []struct {
		name string
		pane *proto.Pane
		want bool
	}{
		{"nil pane", nil, false},
		{"plain shell", &proto.Pane{ID: "p1", Shell: "zsh"}, false},
		{"agent without session", &proto.Pane{ID: "p1", Agent: "claude"}, false},
		{"unsupported agent", &proto.Pane{ID: "p1", Agent: "dsh", AgentSession: &proto.AgentSession{Agent: "dsh", ID: "sess-1"}}, false},
		{"exited agent", &proto.Pane{ID: "p1", Status: proto.Exited, Agent: "claude", AgentSession: &proto.AgentSession{Agent: "claude", ID: "sess-1"}}, false},
		{"claude with session", &proto.Pane{ID: "p1", Agent: "claude", AgentSession: &proto.AgentSession{Agent: "claude", ID: "sess-1"}}, true},
		{"codex with session", &proto.Pane{ID: "p1", Agent: "codex", AgentSession: &proto.AgentSession{Agent: "codex", ID: "sess-2"}}, true},
		{"pi with session", &proto.Pane{ID: "p1", Agent: "pi", AgentSession: &proto.AgentSession{Agent: "pi", ID: "sess-3"}}, true},
		{"opencode with session", &proto.Pane{ID: "p1", Agent: "opencode", AgentSession: &proto.AgentSession{Agent: "opencode", ID: "ses_4"}}, true},
	}
	for _, tc := range cases {
		if got := uihost.MatchesWhen("pane.agentForkable", sess, tc.pane); got != tc.want {
			t.Errorf("%s: MatchesWhen = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestForkPaneActionAndPalette(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady

	calls := make(chan proto.ForkParams, 2)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		if method != "pane.fork" {
			return nil, errors.New("unexpected method " + method)
		}
		var p proto.ForkParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		calls <- p
		return proto.Session{
			ID:    "s1",
			Focus: "p2",
			Panes: []proto.Pane{
				{ID: "p1", Agent: "claude", AgentSession: &proto.AgentSession{Agent: "claude", ID: "sess-1"}},
				{ID: "p2"},
			},
		}, nil
	})
	a.Local().Client = cl
	defer stop()

	// 1. Plain shell pane: not in palette, and pane.fork sends no RPC.
	a.St.Sessions = []proto.Session{{
		ID:    "s1",
		Focus: "p1",
		Panes: []proto.Pane{{ID: "p1", Shell: "zsh"}},
	}}
	a.Selected = "s1"
	a.Focus["s1"] = "p1"

	wantTitle := locales.ActionForkSession()
	if slices.Contains(a.paletteItems(), wantTitle) {
		t.Fatalf("palette offered %q on plain shell pane", wantTitle)
	}
	a.run("pane.fork", nil)
	select {
	case got := <-calls:
		t.Fatalf("pane.fork unexpectedly sent RPC %+v on plain shell pane", got)
	case <-time.After(80 * time.Millisecond):
	}

	// 2. Pane with forkable agent session: shown in palette and sends pane.fork RPC.
	a.St.Sessions = []proto.Session{{
		ID:    "s1",
		Focus: "p1",
		Panes: []proto.Pane{{
			ID:           "p1",
			Agent:        "claude",
			AgentSession: &proto.AgentSession{Agent: "claude", ID: "sess-1"},
		}},
	}}
	if !slices.Contains(a.paletteItems(), wantTitle) {
		t.Fatalf("palette did not offer %q on forkable agent pane", wantTitle)
	}
	a.run("pane.fork", nil)
	select {
	case got := <-calls:
		if got.Pane != "p1" || got.Direction != "right" {
			t.Fatalf("pane.fork params = %+v, want Pane=p1 Direction=right", got)
		}
	case <-time.After(time.Second):
		t.Fatal("pane.fork did not send RPC")
	}
}
