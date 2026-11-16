package daemon

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"

	"hyla/internal/proto"
)

// A pane's timeline is the turns of the records its agent keeps: Claude
// Code's transcript of the pane's conversation, in the project of its
// directory; DeepSeek Harness's log of the newest session there; Codex's
// rollout of the conversation; and OpenCode's store, of the conversation
// the pane names.
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

	// OpenCode's store, which keeps every conversation: the newest of the
	// pane's directory, and the one the pane names.
	store := filepath.Join(home, "opencode")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(store, "opencode.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`create table session_v2 (id text primary key, directory text, parent_id text, time_updated integer)`,
		`create table session_message (id text primary key, session_id text, type text, seq integer, time_created integer, data text)`,
		`insert into session_v2 (id, directory, time_updated) values ('ses_1', '/tmp/my-proj', 1)`,
		`insert into session_message (id, session_id, type, seq, time_created, data) values
			('m1', 'ses_1', 'user', 1, 1763287200000, '{"text":"Ship it"}')`,
		`insert into session_message (id, session_id, type, seq, time_created, data) values
			('m2', 'ses_1', 'assistant', 2, 1763287201000, '{"time":{"completed":1763287205000},"content":[
				{"type":"tool","name":"bash","state":{"status":"completed","input":{"command":"go test"}},"time":{"created":1763287202000,"completed":1763287204000}}]}')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("OPENCODE_DATA_DIR", store)
	oc := &pane{info: proto.Pane{ID: "p4", Dir: dir, Agent: "opencode", AgentSessionID: "ses_1"}}
	d.panes["p4"] = oc
	tm, err = d.timelineOf(proto.TimelineParams{Pane: "p4"})
	if err != nil || len(tm.Turns) != 1 || tm.Turns[0].Title != "Ship it" {
		t.Fatalf("opencode timeline %+v, %v", tm, err)
	}
	if c := tm.Turns[0].Children; len(c) != 1 || c[0].Kind != proto.TimelineTool {
		t.Errorf("opencode turn %+v", tm.Turns[0])
	}
	// An OpenCode pane whose conversation the store has not seen has no
	// turns.
	oc.info.AgentSessionID = "ses_unknown"
	oc.info.Dir = "/tmp/elsewhere"
	if tm, err := d.timelineOf(proto.TimelineParams{Pane: "p4"}); err != nil || len(tm.Turns) != 0 {
		t.Fatalf("opencode timeline of no conversation %+v, %v", tm, err)
	}

	// Codex's rollout, by the conversation the pane names.
	codexHome := filepath.Join(home, ".codex")
	rolloutDir := filepath.Join(codexHome, "sessions", "2026", "11", "16")
	if err := os.MkdirAll(rolloutDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rollout := `{"timestamp":"2026-11-16T10:00:00.000Z","type":"session_meta","payload":{"session_id":"sess-codex","cwd":"/tmp/my-proj"}}
{"timestamp":"2026-11-16T10:00:01.000Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}
{"timestamp":"2026-11-16T10:00:02.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Audit the build"}]}}
{"timestamp":"2026-11-16T10:00:03.000Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go build ./...\"}","call_id":"c1"}}
{"timestamp":"2026-11-16T10:00:09.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"Process exited with code 0\nOutput:\nok"}}
{"timestamp":"2026-11-16T10:00:20.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1"}}
`
	if err := os.WriteFile(filepath.Join(rolloutDir, "rollout-2026-11-16T10-00-00-sess-codex.jsonl"), []byte(rollout), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", codexHome)
	cx := &pane{info: proto.Pane{ID: "p5", Dir: dir, Agent: "codex", AgentSessionID: "sess-codex"}}
	d.panes["p5"] = cx
	tm, err = d.timelineOf(proto.TimelineParams{Pane: "p5"})
	if err != nil || len(tm.Turns) != 1 || tm.Turns[0].Title != "Audit the build" {
		t.Fatalf("codex timeline %+v, %v", tm, err)
	}
	if c := tm.Turns[0].Children; len(c) != 1 || c[0].Kind != proto.TimelineTool {
		t.Errorf("codex turn %+v", tm.Turns[0])
	}

	// A pane that does not exist is an error.
	if _, err := d.timelineOf(proto.TimelineParams{Pane: "nope"}); err == nil {
		t.Error("a timeline of no pane")
	}
}
