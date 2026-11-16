package timeline

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

// When is a test's time, unix ms: 2026-11-16T10:00:00Z.
var when = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

// at is a time a little after When, as a line time.
func at(d time.Duration) string { return when.Add(d).Format("2006-01-02T15:04:05.000Z") }

// ms is at's unix ms.
func ms(d time.Duration) int64 { return when.Add(d).UnixMilli() }

// claudeLines is a Claude Code transcript of the lines given.
func claudeLines(lines ...string) []byte {
	return []byte(joined(lines))
}

func joined(lines []string) string {
	var b bytes.Buffer
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	return b.String()
}

// A prompt begins a turn; its tools, its reasoning and its words are the
// turn's steps, each from when it started to when its result came; and a
// subagent, Claude Code's Agent tool, is a step of its own.
func TestClaudeTimeline(t *testing.T) {
	data := claudeLines(
		`{"type":"user","timestamp":"`+at(0)+`","isSidechain":false,"message":{"role":"user","content":"Fix the login bug"}}`,
		`{"type":"assistant","timestamp":"`+at(time.Second)+`","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Look at the file first."},{"type":"text","text":"I will read it."}]}}`,
		`{"type":"assistant","timestamp":"`+at(2*time.Second)+`","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/a.go"}}]}}`,
		`{"type":"user","timestamp":"`+at(3*time.Second)+`","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"package a"}]}}`,
		`{"type":"assistant","timestamp":"`+at(4*time.Second)+`","message":{"role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Agent","input":{"description":"Explore the tests","subagent_type":"Explore"}}]}}`,
		`{"type":"user","timestamp":"`+at(9*time.Second)+`","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"Async agent launched successfully."}]}]}}`,
		`{"type":"assistant","timestamp":"`+at(10*time.Second)+`","message":{"role":"assistant","content":[{"type":"tool_use","id":"t3","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"user","timestamp":"`+at(11*time.Second)+`","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t3","content":"FAIL","is_error":true}]}}`,
		`{"type":"user","timestamp":"`+at(12*time.Second)+`","message":{"role":"user","content":"<system-reminder>not a prompt</system-reminder>"}}`,
		`{"type":"user","timestamp":"`+at(13*time.Second)+`","message":{"role":"user","content":"And the logout too"}}`,
		`{"type":"assistant","timestamp":"`+at(14*time.Second)+`","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}`,
	)
	turns := Claude(data, when, when.Add(20*time.Second))
	if len(turns) != 2 {
		t.Fatalf("%d turns, want 2", len(turns))
	}
	first := turns[0]
	if first.Kind != Turn || first.Title != "Fix the login bug" || first.Start != ms(0) {
		t.Errorf("turn %+v", first)
	}
	want := []struct {
		kind  Kind
		title string
		start time.Duration
		end   time.Duration
		state State
	}{
		{Think, "Look at the file first.", time.Second, 3 * time.Second, Done},
		{Text, "I will read it.", time.Second, 3 * time.Second, Done},
		{Tool, "Read", 2 * time.Second, 3 * time.Second, Done},
		{Subagent, "Explore the tests", 4 * time.Second, 9 * time.Second, Done},
		{Tool, "Bash", 10 * time.Second, 11 * time.Second, Failed},
	}
	if len(first.Children) != len(want) {
		t.Fatalf("%d steps, want %d: %+v", len(first.Children), len(want), first.Children)
	}
	for i, w := range want {
		c := first.Children[i]
		if c.Kind != w.kind || c.Title != w.title || c.Start != ms(w.start) || c.End != ms(w.end) || c.State != w.state {
			t.Errorf("step %d = %+v, want %+v", i, c, w)
		}
	}
	if d := first.Children[3].Detail; d != "Explore" {
		t.Errorf("subagent detail %q", d)
	}
	if d := first.Children[1].Detail; d != "" {
		t.Errorf("text detail %q, want none", d)
	}
	// The reminder is no turn of its own.
	if turns[1].Title != "And the logout too" {
		t.Errorf("second turn %q", turns[1].Title)
	}
	// The turn ends with its answer, which ends where the lines it came
	// with did.
	if turns[1].End == 0 || turns[1].End > ms(20*time.Second) || turns[1].State != Running {
		t.Errorf("second turn end %d state %s, want its answer still running at the read", turns[1].End, turns[1].State)
	}
}

// A conversation whose lines tell no time is laid out from start, in the
// order it wrote, and its last step runs at now.
func TestClaudeUntimed(t *testing.T) {
	data := claudeLines(
		`{"type":"user","message":{"role":"user","content":"Hello"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hi."}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"Read","input":{"file_path":"/a"}}]}}`,
	)
	start := when.Add(-time.Minute)
	turns := Claude(data, start, when)
	if len(turns) != 1 || turns[0].Start < start.UnixMilli() || turns[0].Start > start.Add(time.Millisecond).UnixMilli() {
		t.Fatalf("turns %+v, want one from %v", turns, start)
	}
	kids := turns[0].Children
	if len(kids) != 2 || kids[0].Start > kids[1].Start || kids[0].End != kids[1].Start {
		t.Errorf("steps %+v", kids)
	}
	if last := kids[len(kids)-1]; last.End != when.UnixMilli() {
		t.Errorf("the last step %+v, want it ended at the read", last)
	}
}

// A sidechain line, a subagent's own, is no turn of the parent's, and a
// meta line is no step.
func TestClaudeLeavesOut(t *testing.T) {
	data := claudeLines(
		`{"type":"user","message":{"role":"user","content":"Go"}}`,
		`{"type":"user","isMeta":true,"message":{"role":"user","content":"Caveat: the messages below were generated"}}`,
		`{"type":"assistant","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"sub"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"main"}]}}`,
		`not json`,
	)
	turns := Claude(data, when, when)
	if len(turns) != 1 || len(turns[0].Children) != 1 || turns[0].Children[0].Title != "main" {
		t.Errorf("turns %+v", turns)
	}
}

// A DeepSeek Harness turn holds its steps, a step its tools; a tool's end
// is its job's settled time, and an approval waits while it is asked.
func TestDSHTimeline(t *testing.T) {
	// d is a line at n seconds after When.
	d := func(n int, typ, body string) string {
		return `{"type":"` + typ + `","time":` + itoa64(ms(time.Duration(n)*time.Second)) + `,"data":` + body + `}`
	}
	data := []byte(joined([]string{
		d(0, "turn/start", `{"turn":1}`),
		d(1, "user/message", `{"content":[{"type":"text","text":"Audit the release"}]}`),
		d(2, "step/start", `{"turn":1,"step":1}`),
		d(3, "assistant/message", `{"turn":1,"step":1,"message":{"role":"assistant","content":[{"type":"reasoning","text":"Check the tags."},{"type":"text","text":"On it."}]}}`),
		d(4, "tool/call", `{"turn":1,"step":1,"callId":"c1","name":"bash","arguments":"git tag"}`),
		d(5, "job/registered", `{"jobId":"j1","kind":"bash","label":"git tag","startedAt":`+itoa64(ms(4*time.Second))+`,"toolCall":{"callId":"c1"}}`),
		d(9, "job/settled", `{"jobId":"j1","kind":"bash","label":"git tag","startedAt":`+itoa64(ms(4*time.Second))+`,"finishedAt":`+itoa64(ms(8*time.Second))+`,"status":"completed","toolCall":{"callId":"c1"}}`),
		d(9, "tool/result", `{"turn":1,"step":1,"message":{"toolCallId":"c1","content":[{"type":"text","text":"v3.11.3"}],"isError":false}}`),
		d(10, "step/end", `{"turn":1,"step":1}`),
		d(10, "step/start", `{"turn":1,"step":2}`),
		d(11, "approval/asked", `{"id":"a1","toolName":"bash","callId":"c2","reason":"escalate sandbox"}`),
		d(15, "approval/decided", `{"id":"a1","outcome":"denied"}`),
		d(15, "step/end", `{"turn":1,"step":2}`),
		d(16, "turn/end", `{"turn":1,"reason":{"kind":"completed"}}`),
	}))
	turns := DSH(data, when)
	if len(turns) != 1 {
		t.Fatalf("%d turns", len(turns))
	}
	turn := turns[0]
	if turn.Title != "Audit the release" || turn.Start != ms(0) || turn.End != ms(16*time.Second) || turn.State != Done {
		t.Errorf("turn %+v", turn)
	}
	// Two steps, and the approval the turn waited for.
	if len(turn.Children) != 3 {
		t.Fatalf("%d steps: %+v", len(turn.Children), turn.Children)
	}
	first := turn.Children[0]
	if len(first.Children) != 3 {
		t.Fatalf("step 1: %+v", first.Children)
	}
	if c := first.Children[0]; c.Kind != Think || c.Title != "Check the tags." {
		t.Errorf("reasoning %+v", c)
	}
	if c := first.Children[1]; c.Kind != Text || c.Title != "On it." {
		t.Errorf("text %+v", c)
	}
	// The tool is the job's span: from its started time to its settle.
	if c := first.Children[2]; c.Kind != Tool || c.Title != "bash" || c.Start != ms(4*time.Second) || c.End != ms(8*time.Second) || c.State != Done {
		t.Errorf("tool %+v", c)
	}
	// The approval is the step's, which waits for it.
	if c := turn.Children[2]; c.Title != "approval: bash" || c.State != Failed || c.Start != ms(11*time.Second) || c.End != ms(15*time.Second) {
		t.Errorf("approval %+v", c)
	}
	// The turn's end is its own line's, not its approval's.
	if turn.End != ms(16*time.Second) {
		t.Errorf("turn end %d", turn.End)
	}
}

// A step still running has no end, and reaches now; a turn run on a
// context message, as another agent's, is named by its turn.
func TestDSHUnfinished(t *testing.T) {
	data := []byte(joined([]string{
		`{"type":"turn/start","time":` + itoa64(ms(0)) + `,"data":{"turn":1}}`,
		`{"type":"user/message","time":` + itoa64(ms(0)) + `,"data":{"content":[{"type":"text","text":"[Message from Chief] do it"}]}}`,
		`{"type":"step/start","time":` + itoa64(ms(time.Second)) + `,"data":{"turn":1,"step":1}}`,
	}))
	turns := DSH(data, when.Add(10*time.Second))
	if len(turns) != 1 || turns[0].Title != "turn 1" {
		t.Fatalf("turns %+v", turns)
	}
	if s := turns[0].Children[0]; s.End != ms(10*time.Second) || s.State != Done {
		t.Errorf("the step %+v, want it ended at the read", s)
	}
	start, end := Span(turns, when.Add(10*time.Second))
	if start.UnixMilli() != ms(0) || end.UnixMilli() != ms(10*time.Second) {
		t.Errorf("span %v..%v", start, end)
	}
}

// DSHFile reads a session log, which DeepSeek Harness compresses.
func TestDSHFileCompressed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.v4.jsonl.zstd")
	if err := writeZstd(path, []byte(joined([]string{
		`{"type":"turn/start","time":` + itoa64(ms(0)) + `,"data":{"turn":1}}`,
		`{"type":"step/start","time":` + itoa64(ms(0)) + `,"data":{"turn":1,"step":1}}`,
		`{"type":"step/end","time":` + itoa64(ms(time.Second)) + `,"data":{"turn":1,"step":1}}`,
		`{"type":"turn/end","time":` + itoa64(ms(time.Second)) + `,"data":{"turn":1,"reason":{"kind":"completed"}}}`,
	}))); err != nil {
		t.Fatal(err)
	}
	turns, err := DSHFile(path, when)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || turns[0].End != ms(time.Second) {
		t.Errorf("turns %+v", turns)
	}
}

// ClaudeFile reads the end of a transcript, not the first cut bytes of a
// line.
func TestClaudeFileTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	head := claudeLines(`{"type":"user","message":{"role":"user","content":"old"}}`)
	if err := os.WriteFile(path, append(head, claudeLines(`{"type":"user","message":{"role":"user","content":"new"}}`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	turns, err := ClaudeFile(path, when)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 || turns[1].Title != "new" {
		t.Errorf("turns %+v", turns)
	}
}

// A tool result's text is a step's detail, as its output.
func TestTexts(t *testing.T) {
	if got := line("a\nb", 10); got != "a" {
		t.Errorf("line %q", got)
	}
	if got := cut("abcdef", 3); got != "abc…" {
		t.Errorf("cut %q", got)
	}
	if got := resultText([]byte(`[{"type":"text","text":"one"},{"type":"image"},{"type":"text","text":"two"}]`)); got != "one" {
		t.Errorf("result text %q", got)
	}
	if got := resultText([]byte(`"plain"`)); got != "plain" {
		t.Errorf("result string %q", got)
	}
	if got := short([]byte(`{"b":1,"a":2}`), 100); got != `{"a":2,"b":1}` {
		t.Errorf("short %q", got)
	}
}

func itoa64(n int64) string {
	var b []byte
	if n == 0 {
		return "0"
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// writeZstd writes data, compressed, as DeepSeek Harness writes its logs.
func writeZstd(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := zstd.NewWriter(f)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		return err
	}
	return w.Close()
}
