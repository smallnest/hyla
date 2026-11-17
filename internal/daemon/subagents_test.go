package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"hyla/internal/proto"
)

// A pane's subagents are the ones its agent's hook told of: what it gave
// its work to, while it runs and once it ended.
func TestPaneSubagents(t *testing.T) {
	d := testDaemon(t)
	pn := &pane{info: proto.Pane{ID: "p1", Agent: "claude"}}
	d.panes["p1"] = pn
	now := time.Now()

	start := proto.SubagentParams{Pane: "p1", ID: "a1", Op: "start", Type: "Explore", Task: "Look at the tests"}
	if ok, err := d.subagentSet(start); err != nil || !ok {
		t.Fatalf("start %v %v", ok, err)
	}
	d.subagentSet(proto.SubagentParams{Pane: "p1", ID: "a1", Op: "start", Type: "Explore", Task: "Look at the tests"})
	ps := d.paneSubagents(pn, now)
	if len(ps.Subs) != 1 {
		t.Fatalf("subagents %+v", ps)
	}
	sb := ps.Subs[0]
	if sb.Type != "Explore" || sb.Task != "Look at the tests" || sb.State != proto.TimelineRunning || sb.End != 0 {
		t.Errorf("running %+v", sb)
	}

	d.subagentSet(proto.SubagentParams{Pane: "p1", ID: "a1", Op: "stop"})
	ps = d.paneSubagents(pn, now)
	if sb := ps.Subs[0]; sb.State != proto.TimelineDone || sb.End == 0 {
		t.Errorf("ended %+v", sb)
	}

	// A stop without a start, as after hylad restarted, is no subagent.
	if ok, _ := d.subagentSet(proto.SubagentParams{Pane: "p1", ID: "a2", Op: "stop"}); ok {
		t.Error("a stop alone was taken")
	}
}

// A pane's subagents are the ones its agent's records tell, where no hook
// came: the agents it gave its work to, in the turn that gave it.
func TestRecordedSubagents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	dir := "/tmp/my-proj"
	project := filepath.Join(home, ".claude", "projects", "-tmp-my-proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"type":"user","timestamp":"2026-11-16T10:00:00.000Z","message":{"role":"user","content":"Fix it"}}
{"type":"assistant","timestamp":"2026-11-16T10:00:05.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"Agent","input":{"description":"Explore the tests","subagent_type":"Explore"}}]}}
{"type":"user","timestamp":"2026-11-16T10:00:19.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"done"}]}}
`
	if err := os.WriteFile(filepath.Join(project, "sess-1.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	d := testDaemon(t)
	pn := &pane{info: proto.Pane{ID: "p1", Dir: dir, Agent: "claude", AgentSessionID: "sess-1"}}
	d.panes["p1"] = pn
	ps := d.paneSubagents(pn, time.Now())
	if len(ps.Subs) != 1 {
		t.Fatalf("subagents %+v", ps)
	}
	sb := ps.Subs[0]
	if sb.Type != "Explore" || sb.Task != "Explore the tests" || sb.Turn != "Fix it" {
		t.Errorf("subagent %+v", sb)
	}
	if sb.State != proto.TimelineDone || sb.End == 0 {
		t.Errorf("state %+v", sb)
	}
	// The records are read again only when they change: the same read
	// comes from what the first one kept.
	if again := d.recordedSubagents("claude", "sess-1", dir, time.Now()); len(again) != 1 || again[0].Task != sb.Task {
		t.Errorf("again %+v", again)
	}
}
