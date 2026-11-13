package feishu

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// An OpenCode turn of a background command reports a step as it ends, and
// says nothing while a step runs: a tool of half a minute leaves the card
// still. Its server knows more, and says it as it happens, so a turn of
// feishu-run = command goes through one of the bot's own, and the card
// takes the agent's words as it writes them. A turn of no server falls
// back to the command's stream, which is what it showed before.

// openCodeServer is the OpenCode server a bot's commands go through,
// which the bot starts for itself, and stops with it.
type openCodeServer struct {
	mu   sync.Mutex
	url  string
	pass string
	cmd  *exec.Cmd
}

// openCode gives the server's URL and password, starting it on the first
// turn, or again after it went.
func (b *Bot) openCode() (string, string, error) {
	if b.noOC {
		return "", "", errNoOpenCode
	}
	s := &b.oc
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.url != "" && openCodeAlive(s.url, s.pass) {
		return s.url, s.pass, nil
	}
	port, err := freePort()
	if err != nil {
		return "", "", err
	}
	pass, err := randomHex(24)
	if err != nil {
		return "", "", err
	}
	prog := "opencode"
	if cmd := b.cfg.AgentCommand("opencode"); len(cmd) > 0 {
		prog = cmd[0]
	}
	exe, err := exec.LookPath(prog)
	if err != nil {
		return "", "", err
	}
	cmd := exec.Command(exe, "serve", "--hostname", "127.0.0.1", "--port", strconv.Itoa(port))
	// The environment the turns run in is the server's too: the
	// provider's keys, and the password the commands take.
	cmd.Env = envWith(append(os.Environ(), b.env...), "OPENCODE_PASSWORD", pass)
	// Its reports are no part of the app's log, which the turns' fill.
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return "", "", err
	}
	go cmd.Wait()
	s.cmd, s.url, s.pass = cmd, fmt.Sprintf("http://127.0.0.1:%d", port), pass
	url := s.url
	for i := 0; i < 50; i++ {
		if openCodeAlive(url, pass) {
			return url, pass, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", "", fmt.Errorf("opencode serve did not answer on %s", url)
}

// stopOpenCode stops the server the bot started, as the app quits.
func (b *Bot) stopOpenCode() {
	s := &b.oc
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.cmd, s.url, s.pass = nil, "", ""
}

// openCodeAlive tells a server that answers, with the password that its
// commands are given.
func openCodeAlive(url, pass string) bool {
	req, err := http.NewRequest("GET", url+"/api/info", nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth("opencode", pass)
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// errNoOpenCode is a turn that shows what its command reports alone.
var errNoOpenCode = errors.New("no OpenCode server for this bot")

// envWith is an environment with the key's one value: the password's
// comes from the bot, not from what the app was started with.
func envWith(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, key+"="+value)
}

// freePort is a port of this machine that nothing listens on.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// randomHex is n bytes as hex, as a password is.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// turnProgress is a turn's progress as its server reports it: the tool
// the agent runs, what its newest call is about, how many it ran, and the
// words it writes.
type turnProgress struct {
	mu    sync.Mutex
	name  string
	tools int
	// input is the newest call's arguments, as its events stream them:
	// JSON text, which its ended event completes.
	input strings.Builder
	arg   string
	text  strings.Builder
	stopC context.CancelFunc
}

// watch follows the server's events while the turn it is of runs.
func (p *turnProgress) watch(url, pass, sid string) {
	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.stopC = cancel
	p.mu.Unlock()
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url+"/api/event", nil)
	if err != nil {
		return
	}
	req.SetBasicAuth("opencode", pass)
	// The stream runs as long as the turn: no timeout of a client's.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	p.read(resp.Body, sid)
}

// stop ends the watching, as the turn ends.
func (p *turnProgress) stop() {
	p.mu.Lock()
	cancel := p.stopC
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// snapshot is the progress as it is now: the tool, what its newest call
// is about, how many ran, and the words so far.
func (p *turnProgress) snapshot() (string, int, string, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.name, p.tools, p.arg, p.text.String()
}

// read takes the events of a server's stream, which are Server-Sent
// Events' data lines, each an event of the turn.
func (p *turnProgress) read(r io.Reader, sid string) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	data := ""
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if data != "" {
				p.event(data, sid)
				data = ""
			}
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		}
	}
}

// event takes one of the server's events, where it is of the turn.
func (p *turnProgress) event(raw, sid string) {
	var e struct {
		Type string `json:"type"`
		Data struct {
			SessionID string          `json:"sessionID"`
			Delta     string          `json:"delta"`
			Text      string          `json:"text"`
			Name      string          `json:"name"`
			Input     json.RawMessage `json:"input"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(raw), &e) != nil || e.Data.SessionID != sid {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch e.Type {
	case "session.text.started":
		// A message of its own: the words of the last are no part of it.
		p.text.Reset()
	case "session.text.delta":
		p.text.WriteString(e.Data.Delta)
	case "session.text.ended":
		if e.Data.Text != "" {
			p.text.Reset()
			p.text.WriteString(e.Data.Text)
		}
	case "session.tool.input.started":
		// A call of its own: what the last one took is no part of it.
		p.input.Reset()
		p.arg = ""
		if e.Data.Name != "" {
			p.name = e.Data.Name
			p.tools++
		}
	case "session.tool.input.delta":
		p.input.WriteString(e.Data.Delta)
	case "session.tool.input.ended":
		// The call's arguments whole, which the card shows a line of.
		in := e.Data.Text
		if in == "" {
			in = p.input.String()
		}
		p.arg = toolArg(in)
	case "session.tool.called":
		// The call as the server made it, its arguments parsed: the last
		// word on what it is about, where it carries them.
		if e.Data.Name != "" {
			p.name = e.Data.Name
		}
		if arg := toolArg(string(e.Data.Input)); arg != "" {
			p.arg = arg
		}
	}
}
