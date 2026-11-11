package feishu

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A turn's progress is its server's events: the words of its messages as
// the agent writes them, and the tool it runs.
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
		// Another chat's turn is no part of this one's.
		`data: {"type":"session.text.delta","data":{"sessionID":"ses_2","delta":"elsewhere"}}`,
		``,
		``,
	}, "\n")
	p.read(strings.NewReader(stream), "ses_1")
	name, n, text := p.snapshot()
	if name != "shell" || n != 1 || text != "Looking at the docs." {
		t.Errorf("the progress is %q, %d tools, %q", name, n, text)
	}
	// A message of its own takes the words anew.
	again := strings.Join([]string{
		`data: {"type":"session.text.started","data":{"sessionID":"ses_1"}}`,
		``,
		`data: {"type":"session.text.delta","data":{"sessionID":"ses_1","delta":"Second."}}`,
		``,
		``,
	}, "\n")
	p.read(strings.NewReader(again), "ses_1")
	if name, n, text := p.snapshot(); text != "Second." || name != "shell" || n != 1 {
		t.Errorf("the progress is %q, %d tools, %q", name, n, text)
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
		_, _, text := p.snapshot()
		return text == "Hi"
	})
	p.stop()

	wrong := &turnProgress{}
	wrong.watch(srv.URL, "no", "ses_1")
	if _, _, text := wrong.snapshot(); text != "" {
		t.Errorf("a refused stream reads %q", text)
	}
}

// A working card shows the server's progress as the tool it runs and the
// words it wrote, and nothing where there is none.
func TestProgressBody(t *testing.T) {
	got := progressBody("shell", 3, "Looking\nat the docs.", "en")
	if !strings.HasPrefix(got, "⏳ **shell** · 3 tools") || !strings.Contains(got, "Looking") {
		t.Errorf("the card reads %q", got)
	}
	if got := progressBody("shell", 1, "", "en"); got != "⏳ **shell** · 1 tool" {
		t.Errorf("a tool alone reads %q", got)
	}
	if got := progressBody("", 0, "", "en"); got != "" {
		t.Errorf("no progress reads %q", got)
	}
}
