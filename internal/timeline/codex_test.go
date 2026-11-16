package timeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// codexRolloutOf is a rollout of the lines given, in a Codex home of its
// own, at the time its name tells.
func codexRolloutOf(t *testing.T, id string, lines ...string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "2026", "11", "16")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-11-16T10-00-00-"+id+".jsonl")
	if err := os.WriteFile(path, []byte(joined(lines)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)
	return path
}

// A message the user sent begins a turn, and what the model reasoned, wrote
// and ran after it are the turn's steps: a tool from the call to its
// output, failed where the program's exit code was no 0, and the turn ends
// with Codex's own word, or runs while the agent works.
func TestCodexTimeline(t *testing.T) {
	path := codexRolloutOf(t,
		"01a09fb5-6969-7b60-8291-641a2bf27d63",
		`{"timestamp":"`+at(0)+`","type":"session_meta","payload":{"session_id":"01a09fb5-6969-7b60-8291-641a2bf27d63","cwd":"/tmp/app"}}`,
		`{"timestamp":"`+at(0)+`","type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}`,
		// The instructions the client puts in, and not the user's words.
		`{"timestamp":"`+at(0)+`","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions\n<INSTRUCTIONS>be good"}]}}`,
		`{"timestamp":"`+at(time.Second)+`","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix the build"}]}}`,
		`{"timestamp":"`+at(2*time.Second)+`","type":"response_item","payload":{"type":"reasoning","content":[{"type":"reasoning_text","text":"Look at the log.\nThen build."}]}}`,
		`{"timestamp":"`+at(3*time.Second)+`","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go build ./...\"}","call_id":"c1"}}`,
		`{"timestamp":"`+at(9*time.Second)+`","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"Chunk ID: ab\nWall time: 1.0 seconds\nProcess exited with code 1\nOriginal token count: 3\nOutput:\n./a.go:3: undefined: x\n"}}`,
		`{"timestamp":"`+at(10*time.Second)+`","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Fixed it."}]}}`,
		`{"timestamp":"`+at(20*time.Second)+`","type":"event_msg","payload":{"type":"task_complete","turn_id":"t1"}}`,
		// A second turn, still running.
		`{"timestamp":"`+at(time.Minute)+`","type":"event_msg","payload":{"type":"task_started","turn_id":"t2"}}`,
		`{"timestamp":"`+at(time.Minute+time.Second)+`","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"and the tests?"}]}}`,
		`{"timestamp":"`+at(time.Minute+2*time.Second)+`","type":"response_item","payload":{"type":"reasoning","content":[{"type":"reasoning_text","text":"Run them."}]}}`,
		`{"timestamp":"`+at(time.Minute+3*time.Second)+`","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}","call_id":"c2"}}`,
	)
	steps, err := CodexFile(path, when.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"turn fix the build 10:00:01.000..10:00:20.000 done",
		"  think Look at the log. 10:00:02.000..10:00:03.000 done",
		"  tool exec_command 10:00:03.000..10:00:09.000 failed",
		"  text Fixed it. 10:00:10.000..10:00:20.000 done",
		"turn and the tests? 10:01:01.000..- running",
		"  think Run them. 10:01:02.000..10:01:03.000 done",
		"  tool exec_command 10:01:03.000..- running",
	}
	check(t, summary(steps), want)
}

// A turn the user interrupted ends failed, and the word Codex writes of it
// begins no turn of its own.
func TestCodexAbortedTurn(t *testing.T) {
	path := codexRolloutOf(t, "id-abort",
		`{"timestamp":"`+at(0)+`","type":"session_meta","payload":{"cwd":"/tmp/app"}}`,
		`{"timestamp":"`+at(0)+`","type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}`,
		`{"timestamp":"`+at(time.Second)+`","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"cd ~"}]}}`,
		`{"timestamp":"`+at(2*time.Second)+`","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<turn_aborted>\nThe user interrupted the previous turn on purpose."}]}}`,
		`{"timestamp":"`+at(3*time.Second)+`","type":"event_msg","payload":{"type":"turn_aborted","turn_id":"t1","reason":"interrupted"}}`,
	)
	steps, err := CodexFile(path, when.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Title != "cd ~" || steps[0].State != Failed {
		t.Fatalf("timeline %q", summary(steps))
	}
}

// A rollout is the one the pane's conversation names, else the newest that
// ran in the pane's directory.
func TestCodexRolloutOf(t *testing.T) {
	path := codexRolloutOf(t, "id-1",
		`{"timestamp":"`+at(0)+`","type":"session_meta","payload":{"cwd":"/tmp/app"}}`,
	)
	if got := codexRollout("id-1", ""); got != path {
		t.Errorf("the conversation's rollout is %q, want %q", got, path)
	}
	if got := codexRollout("", "/tmp/app"); got != path {
		t.Errorf("the directory's rollout is %q, want %q", got, path)
	}
	if got := codexRollout("", "/tmp/other"); got != "" {
		t.Errorf("a rollout of another directory is %q", got)
	}
}
