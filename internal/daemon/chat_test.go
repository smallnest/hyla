package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"hyla/internal/proto"
)

// A pane's conversation is the turns of its agent's records: what the user
// asked, what the agent wrote, and the tools it ran, with their lengths and
// the model that answered.
func TestChatOf(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	dir := "/tmp/my-proj"
	project := filepath.Join(home, ".claude", "projects", "-tmp-my-proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"type":"user","timestamp":"2026-11-17T10:00:00.000Z","message":{"role":"user","content":"Why does playback lag?"}}
{"type":"assistant","timestamp":"2026-11-17T10:00:05.000Z","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"I will put the loading note under the controls."},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"grep -rn loading src"}}]}}
{"type":"user","timestamp":"2026-11-17T10:00:12.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"src/player.ts:42: loading"}]}}
{"type":"assistant","timestamp":"2026-11-17T10:00:20.000Z","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"The player shows nothing until the stream starts."}]}}
`
	if err := os.WriteFile(filepath.Join(project, "sess-1.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	d := testDaemon(t)
	pn := &pane{info: proto.Pane{ID: "p1", Dir: dir, Agent: "claude", AgentSessionID: "sess-1"}}
	d.panes["p1"] = pn
	ch, err := d.chatOf(proto.ChatParams{Session: "s1", Pane: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Agent != "claude" || ch.Note != "" || ch.Model != "claude-opus-5" {
		t.Fatalf("chat %+v", ch)
	}
	if len(ch.Lines) != 4 {
		t.Fatalf("lines %+v", ch.Lines)
	}
	if l := ch.Lines[0]; l.Kind != proto.ChatUser || l.Text != "Why does playback lag?" {
		t.Errorf("first %+v", l)
	}
	if l := ch.Lines[1]; l.Kind != proto.ChatAgent || l.Text == "" || l.End == 0 {
		t.Errorf("second %+v", l)
	}
	tool := ch.Lines[2]
	if tool.Kind != proto.ChatTool || tool.Title != "Bash" || tool.Start == 0 || tool.End == 0 {
		t.Errorf("tool %+v", tool)
	}
	if tool.End-tool.Start != 7000 {
		t.Errorf("tool ran %dms, want 7s", tool.End-tool.Start)
	}
	if l := ch.Lines[3]; l.Kind != proto.ChatAgent || l.Text != "The player shows nothing until the stream starts." {
		t.Errorf("last %+v", l)
	}
	// An agent whose records Hyla does not read has a note, and no lines.
	pn.info.Agent = "gemini"
	ch, err = d.chatOf(proto.ChatParams{Pane: "p1"})
	if err != nil || ch.Note == "" || len(ch.Lines) != 0 {
		t.Fatalf("gemini chat %+v, %v", ch, err)
	}
}
