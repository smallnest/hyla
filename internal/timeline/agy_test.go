package timeline

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgyTranscriptParsing(t *testing.T) {
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)

	data := strings.Join([]string{
		`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-20T07:50:00Z","content":"<USER_REQUEST>\nFix login bug\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nlocal\n</ADDITIONAL_METADATA>\n<USER_SETTINGS_CHANGE>\nx\n</USER_SETTINGS_CHANGE>"}`,
		`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-20T07:50:01Z","model":"gemini-pro-orchestrator-neon","thinking":"Inspect auth handler first","tool_calls":[{"name":"read_file","args":{"Path":"\"/repo/auth.go\""}},{"name":"run_command","args":{"CommandLine":"\"go test ./...\""}},{"name":"invoke_subagent","args":{"Prompt":"\"Review edge cases\"","Role":"Code Reviewer","TypeName":"research","Workspace":"\"inherit\""}}]}`,
		`{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-11-20T07:50:05Z","content":"Created At: 2026-11-20T07:50:05Z\nCompleted At: 2026-11-20T07:50:05Z\nCreated At: 2026-11-20T07:50:05Z\nCompleted At: 2026-11-20T07:50:05Z\npackage auth"}`,
		`{"step_index":3,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-11-20T07:50:05Z","content":"Created At: 2026-11-20T07:50:05Z\nCompleted At: 2026-11-20T07:50:08Z\nCommand exited with code 1.\nFAIL auth_test.go"}`,
		`{"step_index":4,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-11-20T07:50:06Z","content":"Created At: 2026-11-20T07:50:06Z\nCompleted At: 2026-11-20T07:50:07Z\nCreated the following subagent:\n{\n  \"conversationId\": \"sub-123\",\n  \"logAbsoluteUri\": \"file:///tmp/sub.jsonl\"\n}"}`,
		`{"step_index":5,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-20T07:50:08Z","model":"gemini-pro-orchestrator-neon","content":"Waiting for sub-123."}`,
		`{"step_index":6,"source":"SYSTEM","type":"SYSTEM_MESSAGE","status":"DONE","created_at":"2026-11-20T07:50:15Z","content":"The following is a <SYSTEM_MESSAGE>\n<SYSTEM_MESSAGE>\n[Message] timestamp=2026-11-20T07:50:15Z sender=sub-123 priority=MESSAGE_PRIORITY_HIGH content=Subagent finished\n</SYSTEM_MESSAGE>"}`,
		`{"step_index":7,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-20T07:50:16Z","model":"gemini-flash-worker-agy","content":"All fixed and verified."}`,
		`{"step_index":8,"source":"SYSTEM","type":"CHECKPOINT","status":"DONE","created_at":"2026-11-20T07:50:16Z","content":"compacted"}`,
		`{"step_index":9,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-20T07:51:00Z","content":"<USER_REQUEST>\nRun background build\n</USER_REQUEST>"}`,
		`{"step_index":10,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-20T07:51:01Z","model":"gemini-flash-worker-agy","tool_calls":[{"name":"run_command","args":{"CommandLine":"\"go build ./...\""}},{"name":"read_file","args":{"Path":"\"missing.go\""}}]}`,
		`{"step_index":11,"source":"MODEL","type":"GENERIC","status":"RUNNING","created_at":"2026-11-20T07:51:02Z","content":"Created At: 2026-11-20T07:51:02Z\nTool is running as a background task with task id: conv-1/task-10\nTask Description: go build ./..."}`,
		`{"step_index":12,"source":"MODEL","type":"GENERIC","status":"ERROR","error":"file does not exist","created_at":"2026-11-20T07:51:02Z","content":"Created At: 2026-11-20T07:51:02Z\nCompleted At: 2026-11-20T07:51:03Z\nEncountered error in step execution: file does not exist"}`,
		`{"step_index":13,"source":"SYSTEM","type":"SYSTEM_MESSAGE","status":"DONE","created_at":"2026-11-20T07:51:10Z","content":"<SYSTEM_MESSAGE>\n[Message] timestamp=2026-11-20T07:51:10Z sender=conv-1/task-10 priority=MESSAGE_PRIORITY_HIGH content=Task id \"conv-1/task-10\" finished with result:\nCommand exited with code 0.\n</SYSTEM_MESSAGE>"}`,
		`{"step_index":14,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-20T07:51:11Z","model":"gemini-flash-worker-agy","tool_calls":[{"name":"ask_question","args":{"questions":"[{\"question\":\"Proceed?\"}]"}}]}`,
	}, "\n")

	turns := Agy([]byte(data), start, now)
	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2: %+v", len(turns), turns)
	}

	t0 := turns[0]
	if t0.Title != "Fix login bug" || t0.Full != "Fix login bug" || t0.State != Done {
		t.Fatalf("turn 0 = %+v", t0)
	}
	if t0.Start != agyTS("2026-11-20T07:50:00Z") || t0.End != agyTS("2026-11-20T07:50:16Z") {
		t.Fatalf("turn 0 span = [%d, %d]", t0.Start, t0.End)
	}
	// Expected children of turn 0:
	// 0: Think [07:50:01, 07:50:05] Done
	// 1: Tool read_file [07:50:05, 07:50:05] Done (/repo/auth.go)
	// 2: Tool run_command [07:50:05, 07:50:08] Failed (exit 1)
	// 3: Subagent Code Reviewer [07:50:06, 07:50:15] Done (detail="research")
	// 4: Text "Waiting for sub-123."
	// 5: Text "All fixed and verified."
	if len(t0.Children) != 6 {
		t.Fatalf("turn 0 children count = %d: %+v", len(t0.Children), t0.Children)
	}
	think := t0.Children[0]
	if think.Kind != Think || think.Start != agyTS("2026-11-20T07:50:01Z") || think.End != agyTS("2026-11-20T07:50:05Z") || think.State != Done {
		t.Errorf("think = %+v", think)
	}
	readTool := t0.Children[1]
	if readTool.Kind != Tool || readTool.Title != "read_file" || !strings.Contains(readTool.Detail, "/repo/auth.go") || readTool.State != Done {
		t.Errorf("read_file = %+v", readTool)
	}
	cmdTool := t0.Children[2]
	if cmdTool.Kind != Tool || cmdTool.Title != "run_command" || cmdTool.State != Failed || cmdTool.End != agyTS("2026-11-20T07:50:08Z") {
		t.Errorf("run_command = %+v", cmdTool)
	}
	sub := t0.Children[3]
	if sub.Kind != Subagent || sub.Title != "Code Reviewer" || sub.Detail != "research" || sub.State != Done || sub.End != agyTS("2026-11-20T07:50:15Z") {
		t.Errorf("subagent = %+v", sub)
	}
	if subs := Subagents(turns); len(subs) != 1 || subs[0].Task != "Code Reviewer" || subs[0].Type != "research" || subs[0].Turn != "Fix login bug" {
		t.Errorf("Subagents() = %+v", subs)
	}

	t1 := turns[1]
	if t1.Title != "Run background build" || t1.State != Waiting || t1.End != 0 {
		t.Fatalf("turn 1 = %+v, want Waiting & open", t1)
	}
	if len(t1.Children) != 3 {
		t.Fatalf("turn 1 children = %+v", t1.Children)
	}
	if bg := t1.Children[0]; bg.Title != "run_command" || bg.State != Done || bg.End != agyTS("2026-11-20T07:51:10Z") {
		t.Errorf("bg cmd = %+v", bg)
	}
	if errStep := t1.Children[1]; errStep.Title != "read_file" || errStep.State != Failed || !strings.Contains(errStep.Detail, "file does not exist") {
		t.Errorf("err step = %+v", errStep)
	}
	if ask := t1.Children[2]; ask.Title != "ask_question" || ask.State != Waiting || ask.End != 0 {
		t.Errorf("ask_question = %+v", ask)
	}

	// AgyModel picks the last model reported in PLANNER_RESPONSE.
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := AgyModel(path); got != "gemini-flash-worker-agy" {
		t.Errorf("AgyModel = %q, want gemini-flash-worker-agy", got)
	}
}

func TestAgyFinishedAndRunningStates(t *testing.T) {
	now := time.Date(2026, 11, 20, 8, 5, 0, 0, time.UTC)

	// Completed turn ends at last timestamp, NOT now.
	doneData := strings.Join([]string{
		`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-20T08:00:00Z","content":"<USER_REQUEST>\nSay hi\n</USER_REQUEST>"}`,
		`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-20T08:00:03Z","model":"gemini-3","content":"Hello!"}`,
	}, "\n")
	turns := Agy([]byte(doneData), now, now)
	if len(turns) != 1 || turns[0].State != Done || turns[0].End != agyTS("2026-11-20T08:00:03Z") {
		t.Fatalf("completed turn = %+v", turns)
	}

	// Background task still running keeps turn Running with open task.
	runningData := strings.Join([]string{
		`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-20T08:00:00Z","content":"<USER_REQUEST>\nBuild\n</USER_REQUEST>"}`,
		`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-20T08:00:01Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"\"make\""}}]}`,
		`{"step_index":2,"source":"MODEL","type":"GENERIC","status":"RUNNING","created_at":"2026-11-20T08:00:02Z","content":"Created At: 2026-11-20T08:00:02Z\nTool is running as a background task with task id: c/task-1\n"}`,
		`{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-20T08:00:03Z","content":"Waiting for build."}`,
	}, "\n")
	turns = Agy([]byte(runningData), now, now)
	if len(turns) != 1 || turns[0].State != Running || turns[0].End != 0 {
		t.Fatalf("running turn = %+v", turns)
	}
	if turns[0].Children[0].State != Running || turns[0].Children[0].End != 0 {
		t.Fatalf("open bg child = %+v", turns[0].Children[0])
	}

	// Canceled background command marks task Failed.
	canceledData := runningData + "\n" + `{"step_index":4,"source":"SYSTEM","type":"SYSTEM_MESSAGE","status":"DONE","created_at":"2026-11-20T08:00:10Z","content":"<SYSTEM_MESSAGE>\n[Message] timestamp=2026-11-20T08:00:10Z sender=c/task-1 priority=MESSAGE_PRIORITY_LOW content=Task id \"c/task-1\" was canceled with result:\nTool execution was canceled\n</SYSTEM_MESSAGE>"}`
	turns = Agy([]byte(canceledData), now, now)
	if turns[0].Children[0].State != Failed || turns[0].Children[0].End != agyTS("2026-11-20T08:00:10Z") {
		t.Fatalf("canceled bg child = %+v", turns[0].Children[0])
	}
}

func TestAgyFindAndFindConv(t *testing.T) {
	home := t.TempDir()
	agyRoot := filepath.Join(home, ".gemini", "antigravity-cli")
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("DSH_HOME", "")
	t.Setenv("OPENCODE_DATA_DIR", "")
	t.Setenv("AGY_DATA_DIR", agyRoot)
	t.Setenv("ANTIGRAVITY_DATA_DIR", "")

	writeTranscript := func(conv string, mod time.Time) string {
		p := filepath.Join(agyRoot, "brain", conv, ".system_generated", "logs", "transcript.jsonl")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		line := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-20T07:50:00Z","content":"<USER_REQUEST>\nHello\n</USER_REQUEST>"}` + "\n"
		if err := os.WriteFile(p, []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, mod, mod)
		return p
	}

	now := time.Now()
	pOld := writeTranscript("conv-old", now.Add(-time.Hour))
	pSub := writeTranscript("conv-sub", now.Add(time.Hour))
	pNew := writeTranscript("conv-new", now)
	_ = pSub

	// Direct ID lookup for both "agy" and "antigravity", and traversal guard.
	if got := Find("agy", "conv-old", ""); got != pOld {
		t.Errorf("Find(agy, conv-old) = %q, want %q", got, pOld)
	}
	if got := Find("antigravity", "conv-old", ""); got != pOld {
		t.Errorf("Find(antigravity, conv-old) = %q, want %q", got, pOld)
	}
	if got := Find("agy", "../etc/passwd", ""); got != "" {
		t.Errorf("Find traversal = %q, want empty", got)
	}
	if agent, file := FindConv("conv-new"); agent != "agy" || file != pNew {
		t.Errorf("FindConv(conv-new) = (%q, %q), want (agy, %q)", agent, file, pNew)
	}

	// SQLite workspace lookup ignores subagent rows (parent_conversation_id != '') and missing transcripts.
	db, err := sql.Open("sqlite", filepath.Join(agyRoot, "conversation_summaries.db"))
	if err != nil {
		t.Fatal(err)
	}
	dir := "/Users/me/proj"
	for _, q := range []string{
		`CREATE TABLE conversation_summaries (conversation_id TEXT PRIMARY KEY, last_modified_time DATETIME, workspace_uris TEXT, parent_conversation_id TEXT DEFAULT '')`,
		`INSERT INTO conversation_summaries VALUES ('conv-old', '2026-11-20 07:00:00', '["file:///Users/me/proj"]', '')`,
		`INSERT INTO conversation_summaries VALUES ('conv-new', '2026-11-20 08:00:00', '["file:///Users/me/proj"]', '')`,
		`INSERT INTO conversation_summaries VALUES ('conv-sub', '2026-11-20 09:00:00', '["file:///Users/me/proj"]', 'conv-new')`,
		`INSERT INTO conversation_summaries VALUES ('conv-ghost', '2026-11-20 10:00:00', '["file:///Users/me/proj"]', '')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	if got := Find("agy", "", dir); got != pNew {
		t.Errorf("Find(agy, '', dir) via DB = %q, want %q", got, pNew)
	}
	if got := Find("antigravity", "", dir); got != pNew {
		t.Errorf("Find(antigravity, '', dir) via DB = %q, want %q", got, pNew)
	}

	// history.jsonl fallback and newer-transcript preference.
	pHist := writeTranscript("conv-hist", now.Add(2*time.Hour))
	hist := `{"workspace":"/Users/me/proj","conversationId":"conv-old"}` + "\n" +
		`{"workspace":"/Users/me/proj","conversationId":"conv-hist"}` + "\n"
	if err := os.WriteFile(filepath.Join(agyRoot, "history.jsonl"), []byte(hist), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Find("agy", "", dir); got != pHist {
		t.Errorf("Find(agy, '', dir) preferring newer history = %q, want %q", got, pHist)
	}
	_ = os.Remove(filepath.Join(agyRoot, "conversation_summaries.db"))
	if got := Find("agy", "", dir); got != pHist {
		t.Errorf("Find(agy, '', dir) history fallback = %q, want %q", got, pHist)
	}
}

func agyTS(s string) int64 {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return t.UnixMilli()
}

