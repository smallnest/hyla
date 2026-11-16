package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"

	"hyla/internal/proto"
)

// A pane's timeline is the turns of the records its agent keeps: Claude
// Code's transcript of the pane's conversation, in the project of its
// directory; and DeepSeek Harness's log of the newest session there.
func TestTimelineOf(t *testing.T) {
	// The agents' directories, as the environment tells them: the first,
	// which some caches keep, is the test's.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("DSH_HOME", "")
	d := testDaemon(t)
	dir := "/tmp/my-proj"
	project := filepath.Join(home, ".claude", "projects", "-tmp-my-proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(project, "sess-1.jsonl")
	lines := `{"type":"user","timestamp":"2026-11-16T10:00:00.000Z","message":{"role":"user","content":"Fix it"}}
{"type":"assistant","timestamp":"2026-11-16T10:00:05.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"Bash","input":{"command":"go test"}}]}}
{"type":"user","timestamp":"2026-11-16T10:00:09.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"ok"}]}}
`
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	claude := &pane{info: proto.Pane{ID: "p1", Dir: dir, Agent: "claude", AgentSessionID: "sess-1"}}
	d.panes["p1"] = claude
	tm, err := d.timelineOf(proto.TimelineParams{Pane: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if tm.Agent != "claude" || tm.Note != "" || len(tm.Turns) != 1 {
		t.Fatalf("timeline %+v", tm)
	}
	turn := tm.Turns[0]
	if turn.Title != "Fix it" || len(turn.Children) != 1 || turn.Children[0].Kind != proto.TimelineTool {
		t.Errorf("turn %+v", turn)
	}

	// Without the conversation's ID, the newest transcript of the
	// directory's.
	claude.info.AgentSessionID = ""
	if tm, err := d.timelineOf(proto.TimelineParams{Pane: "p1"}); err != nil || len(tm.Turns) != 1 {
		t.Fatalf("timeline %+v, %v", tm, err)
	}

	// An agent whose records Hyla does not read has a note, and no turns.
	codex := &pane{info: proto.Pane{ID: "p2", Dir: dir, Agent: "codex"}}
	d.panes["p2"] = codex
	if tm, err := d.timelineOf(proto.TimelineParams{Pane: "p2"}); err != nil || tm.Note == "" || len(tm.Turns) != 0 {
		t.Fatalf("codex timeline %+v, %v", tm, err)
	}

	// DeepSeek Harness's log, in the directory's sessions.
	sdir := filepath.Join(home, ".dsh", "sessions", "--tmp-my-proj--", "session-7")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w, _ := zstd.NewWriter(&buf)
	w.Write([]byte(`{"type":"turn/start","time":1763287200000,"data":{"turn":1}}
{"type":"user/message","time":1763287200000,"data":{"content":[{"type":"text","text":"Audit"}]}}
{"type":"turn/end","time":1763287209000,"data":{"turn":1,"reason":{"kind":"completed"}}}
`))
	w.Close()
	if err := os.WriteFile(filepath.Join(sdir, "session.v4.jsonl.zstd"), buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	dsh := &pane{info: proto.Pane{ID: "p3", Dir: dir, Agent: "dsh"}}
	d.panes["p3"] = dsh
	tm, err = d.timelineOf(proto.TimelineParams{Pane: "p3"})
	if err != nil || len(tm.Turns) != 1 || tm.Turns[0].Title != "Audit" {
		t.Fatalf("dsh timeline %+v, %v", tm, err)
	}

	// A pane that does not exist is an error.
	if _, err := d.timelineOf(proto.TimelineParams{Pane: "nope"}); err == nil {
		t.Error("a timeline of no pane")
	}
}
