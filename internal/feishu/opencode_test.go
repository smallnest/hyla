package feishu

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A turn's progress is its server's events: the words of its messages as
// the agent writes them, the tool it runs, and what its newest call is
// about.
func TestTurnProgress(t *testing.T) {
	p := &turnProgress{}
	stream := strings.Join([]string{
		`data: {"type":"session.text.started","data":{"sessionID":"ses_1"}}`,
		``,
		`data: {"type":"session.text.delta","data":{"sessionID":"ses_1","delta":"Looking at "}}`,
		``,
		`data: {"type":"session.text.delta","data":{"sessionID":"ses_1","delta":"the docs."}}`,
		``,
		`data: {"type":"session.tool.input.started","data":{"sessionID":"ses_1","name":"shell"}}`,
		``,
		`data: {"type":"session.tool.input.delta","data":{"sessionID":"ses_1","delta":"{\"command\":"}}`,
		``,
		`data: {"type":"session.tool.input.ended","data":{"sessionID":"ses_1","text":"{\"command\":\"go test ./internal/feishu\"}"}}`,
		``,
		// Another chat's turn is no part of this one's.
		`data: {"type":"session.text.delta","data":{"sessionID":"ses_2","delta":"elsewhere"}}`,
		``,
		``,
	}, "\n")
	p.read(strings.NewReader(stream), "ses_1")
	name, n, arg, text := p.snapshot()
	if name != "shell" || n != 1 || arg != "go test ./internal/feishu" || text != "Looking at the docs." {
		t.Errorf("the progress is %q, %d tools, %q, %q", name, n, arg, text)
	}
	// The call the server made says what it is about, parsed.
	p.read(strings.NewReader(`data: {"type":"session.tool.called","data":{"sessionID":"ses_1","name":"read","input":{"filePath":"internal/feishu/card.go"}}}`+"\n\n"), "ses_1")
	if name, n, arg, _ := p.snapshot(); name != "read" || n != 1 || arg != "internal/feishu/card.go" {
		t.Errorf("the call is %q, %d tools, %q", name, n, arg)
	}
	// A message of its own takes the words anew, and a call's own its
	// arguments.
	again := strings.Join([]string{
		`data: {"type":"session.text.started","data":{"sessionID":"ses_1"}}`,
		``,
		`data: {"type":"session.text.delta","data":{"sessionID":"ses_1","delta":"Second."}}`,
		``,
		`data: {"type":"session.tool.input.started","data":{"sessionID":"ses_1","name":"shell"}}`,
		``,
		``,
	}, "\n")
	p.read(strings.NewReader(again), "ses_1")
	if name, n, arg, text := p.snapshot(); text != "Second." || name != "shell" || n != 2 || arg != "" {
		t.Errorf("the progress is %q, %d tools, %q, %q", name, n, arg, text)
	}
}

// The server's stream is taken with the password the commands are given,
// and a password it refuses leaves no progress.
func TestTurnProgressWatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, pw, ok := r.BasicAuth(); !ok || u != "opencode" || pw != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		io.WriteString(w, `data: {"type":"session.text.delta","data":{"sessionID":"ses_1","delta":"Hi"}}`+"\n\n")
	}))
	defer srv.Close()
	p := &turnProgress{}
	go p.watch(srv.URL, "pw", "ses_1")
	waitFor(t, "the stream's words", func() bool {
		_, _, _, text := p.snapshot()
		return text == "Hi"
	})
	p.stop()

	wrong := &turnProgress{}
	wrong.watch(srv.URL, "no", "ses_1")
	if _, _, _, text := wrong.snapshot(); text != "" {
		t.Errorf("a refused stream reads %q", text)
	}
}

// A working card shows the server's progress as the tool it runs, what the
// call is about, and the words it wrote; and nothing where there is none.
func TestProgressBody(t *testing.T) {
	got := progressBody("shell", 3, "go test ./internal/feishu", "Looking\nat the docs.", "en")
	if !strings.HasPrefix(got, "⏳ **shell** · 3 tools\n`go test ./internal/feishu`") || !strings.Contains(got, "Looking") {
		t.Errorf("the card reads %q", got)
	}
	if got := progressBody("shell", 1, "", "", "en"); got != "⏳ **shell** · 1 tool" {
		t.Errorf("a tool alone reads %q", got)
	}
	if got := progressBody("", 0, "", "", "en"); got != "" {
		t.Errorf("no progress reads %q", got)
	}
}

// A call's arguments are the card's line as what the call is about: the
// command it runs, the file it reads, the pattern it looks for; and none
// where they say nothing of the kind.
func TestToolArg(t *testing.T) {
	for _, c := range []struct{ tool, in, want string }{
		{"shell", `{"command":"go build ./...","description":"Build it"}`, "go build ./..."},
		{"read", `{"filePath":"internal/feishu/card.go"}`, "internal/feishu/card.go"},
		{"read", `{"file_path":"/tmp/a.go"}`, "/tmp/a.go"},
		{"grep", `{"pattern":"toolArg","path":"internal"}`, "toolArg"},
		{"webfetch", `{"url":"https://example.com"}`, "https://example.com"},
		{"task", `{"description":"Find it","prompt":"a long prompt"}`, "Find it"},
		// A tool the card does not know shows no line, rather than one of
		// its keys taken for what the call is about.
		{"todowrite", `{"todos":[{"content":"x"}]}`, ""},
		// Arguments that are no object.
		{"shell", `"ls -la"`, "ls -la"},
		{"x", `["a","b"]`, "a b"},
		{"x", ``, ""},
		// A call's words are one line, cut to the width of a phone's.
		{"shell", `{"command":"echo one\necho two"}`, "echo one echo two"},
		{"shell", `{"command":"` + strings.Repeat("x", 200) + `"}`, strings.Repeat("x", argWidth) + "…"},
	} {
		if got := toolArg(c.in); got != c.want {
			t.Errorf("toolArg(%s %s) = %q, want %q", c.tool, c.in, got, c.want)
		}
	}
}
