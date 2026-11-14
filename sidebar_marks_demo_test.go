package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
)

// TestSidebarMarks lays the sidebar out with a session per agent, for a
// look at the marks in $HYLA_SHOTS.
func TestSidebarMarks(t *testing.T) {
	a := testApp(t)
	names := map[string]string{
		"claude": "review-pr-42", "codex": "bump-deps", "opencode": "hooks",
		"dsh": "index-raw-docs", "cursor-agent": "fix-login-redirect",
		"kimi": "translate-docs", "qwen": "trace-slow-query",
		"windsurf": "profile-startup", "aider": "tidy-imports", "": "make test",
	}
	order := []string{"claude", "codex", "opencode", "dsh", "cursor-agent", "kimi", "qwen", "windsurf", "aider", ""}
	var ss []proto.Session
	for i, agent := range order {
		p := proto.Pane{ID: fmt.Sprintf("p%d", i), Dir: "/Users/smallnest/ai/hyla", Status: proto.Idle, Progress: -1, Agent: agent, Process: agent}
		ss = append(ss, proto.Session{ID: fmt.Sprintf("s%d", i), Project: "p1", Name: names[agent], Pane: p, Panes: []proto.Pane{p}})
	}
	a.st.Projects = []proto.Project{{ID: "p1", Name: "hyla", Dir: "/Users/smallnest/ai/hyla"}}
	a.st.Sessions = ss
	a.loaded, a.selected = true, "s0"
	for i, c := range []struct {
		dark    bool
		compact bool
	}{{false, false}, {true, false}, {false, true}} {
		name := []string{"sidebar-marks", "sidebar-marks-dark", "sidebar-marks-compact"}[i]
		dark := c.dark
		a.set.SidebarRows = "detailed"
		if c.compact {
			a.set.SidebarRows = "compact"
		}
		tt := ui.NewTesterWith(a.view, 420, 620, ui.TesterOptions{Dark: dark})
		for i := 0; i < 60 && tt.FramePending(); i++ {
			tt.Frame()
			tt.Advance(time.Second)
		}
		shot(t, tt, name)
	}
}
