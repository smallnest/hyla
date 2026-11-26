package daemon

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"hyla/internal/proto"
	"hyla/internal/settings"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// signed returns an OSC 7577 body with its mac, as a program computes it.
func signed(token, body string) string {
	sum := sha256.Sum256([]byte(body + ":" + token))
	return body + ":mac=" + hex.EncodeToString(sum[:])
}

func report(pid int, now time.Time, token string, pairs ...string) string {
	body := fmt.Sprintf("pid=%d:time=%d", pid, now.Unix())
	for _, p := range pairs {
		body += ":" + p
	}
	return signed(token, body)
}

func (s *pane) takeResume(t *testing.T, body string, now time.Time) {
	t.Helper()
	s.mu.Lock()
	s.handleResumeReport(body, now)
	s.mu.Unlock()
}

func TestResumeToken(t *testing.T) {
	a, b := newResumeToken(), newResumeToken()
	if len(a) != 43 || len(b) != 43 {
		t.Fatalf("tokens %q and %q, want 43 characters", a, b)
	}
	if a == b {
		t.Fatal("two terminals share a token")
	}
	for _, c := range a {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			t.Fatalf("token %q holds %q", a, c)
		}
	}
}

func TestOSC7577Report(t *testing.T) {
	s := testPane(t)
	if s.resumeToken == "" {
		t.Fatal("a pane has no token")
	}
	now := time.Now()
	argv := "argv=" + strings.Join([]string{b64("nvim"), b64("-S"), b64("/tmp/s.vim")}, ",")
	body := report(4242, now, s.resumeToken, argv, "cwd="+b64("/tmp"))
	s.output([]byte("\x1b]7577;" + body + "\x1b\\"))
	if s.resume == nil {
		t.Fatal("no record")
	}
	if s.resume.PID != 4242 || strings.Join(s.resume.Argv, " ") != "nvim -S /tmp/s.vim" || s.resume.Cwd != "/tmp" {
		t.Fatalf("record %+v", *s.resume)
	}
	clear := report(4242, now.Add(2*time.Second), s.resumeToken, "clear=1")
	s.output([]byte("\x1b]7577;" + clear + "\x07"))
	if s.resume != nil {
		t.Fatalf("clear left %+v", *s.resume)
	}
}

// A report the token does not sign is ignored: it leaves a record alone.
func TestOSC7577BadMac(t *testing.T) {
	s := testPane(t)
	now := time.Now()
	good := report(1, now, s.resumeToken, "argv="+b64("htop"))
	s.takeResume(t, good, now)
	if s.resume == nil {
		t.Fatal("no record to keep")
	}
	bad := report(1, now, "dummy-token-replace-me", "argv="+b64("rm"), "clear=1")
	s.takeResume(t, bad, now)
	if s.resume == nil || strings.Join(s.resume.Argv, " ") != "htop" {
		t.Fatalf("a bad mac changed the record: %+v", s.resume)
	}
}

// A report that passes the mac but breaks a rule rejects its owner's
// records: its command is no longer the one the record holds.
func TestOSC7577Rejects(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		more string
	}{
		{"relative path", []string{"./run.sh"}, ""},
		{"dash first", []string{"-rf"}, ""},
		{"empty first", []string{""}, ""},
		{"one of argv and clear", []string{"htop"}, "clear=1"},
		{"unknown need", []string{"htop"}, "need=later"},
		{"relative cwd", []string{"htop"}, "cwd=" + b64("project")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := testPane(t)
			now := time.Now()
			s.takeResume(t, report(7, now, s.resumeToken, "argv="+b64("htop")), now)
			if s.resume == nil {
				t.Fatal("no record to reject")
			}
			var els []string
			for _, a := range c.argv {
				els = append(els, b64(a))
			}
			pairs := []string{"argv=" + strings.Join(els, ",")}
			if c.more != "" {
				pairs = append(pairs, c.more)
			}
			s.takeResume(t, report(7, now.Add(time.Second), s.resumeToken, pairs...), now.Add(time.Second))
			if s.resume != nil {
				t.Fatalf("a rejected report kept %+v", *s.resume)
			}
		})
	}
	// Over 64 elements is a rejection too.
	s := testPane(t)
	now := time.Now()
	s.takeResume(t, report(7, now, s.resumeToken, "argv="+b64("htop")), now)
	els := make([]string, 65)
	for i := range els {
		els[i] = b64("a")
	}
	s.takeResume(t, report(7, now.Add(time.Second), s.resumeToken, "argv="+strings.Join(els, ",")), now.Add(time.Second))
	if s.resume != nil {
		t.Fatal("an oversized argv kept the record")
	}
}

// An old or out-of-order report is ignored, not rejected.
func TestOSC7577Time(t *testing.T) {
	s := testPane(t)
	now := time.Now()
	s.takeResume(t, report(3, now, s.resumeToken, "argv="+b64("htop")), now)
	if s.resume == nil {
		t.Fatal("no record")
	}
	old := report(3, now.Add(-2*time.Minute), s.resumeToken, "argv="+b64("rm"), "clear=1")
	s.takeResume(t, old, now)
	if s.resume == nil || strings.Join(s.resume.Argv, " ") != "htop" {
		t.Fatalf("an old report changed the record: %+v", s.resume)
	}
	back := report(3, now.Add(-time.Second), s.resumeToken, "clear=1")
	s.takeResume(t, back, now)
	if s.resume == nil {
		t.Fatal("an earlier report cleared a later one")
	}
}

// Base64 without its padding is as good as with it.
func TestOSC7577Unpadded(t *testing.T) {
	s := testPane(t)
	now := time.Now()
	body := fmt.Sprintf("pid=4:time=%d:argv=%s", now.Unix(), base64.RawStdEncoding.EncodeToString([]byte("lazygit")))
	s.takeResume(t, signed(s.resumeToken, body), now)
	if s.resume == nil || s.resume.Argv[0] != "lazygit" {
		t.Fatalf("record %+v", s.resume)
	}
}

// A full reset (RIS) does not touch a resume record: the sequence is not
// signed, so displayed text must not clear what the user may resume.
func TestFullResetKeepsResume(t *testing.T) {
	s := testPane(t)
	now := time.Now()
	s.takeResume(t, report(5, now, s.resumeToken, "argv="+b64("htop")), now)
	s.output([]byte("\x1bc"))
	if s.resume == nil {
		t.Fatal("RIS cleared the resume record")
	}
}

// The owner must be the pane's shell or a process under it.
func TestOSC7577Owner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("psList is POSIX here")
	}
	if len(psList()) == 0 {
		t.Skip("no process list")
	}
	s := testPane(t)
	now := time.Now()
	s.shellPID = os.Getpid()
	s.takeResume(t, report(os.Getpid(), now, s.resumeToken, "argv="+b64("htop")), now)
	if s.resume == nil {
		t.Fatal("the shell's own report was refused")
	}
	s.takeResume(t, report(1, now.Add(time.Second), s.resumeToken, "argv="+b64("rm")), now.Add(time.Second))
	if s.resume == nil || strings.Join(s.resume.Argv, " ") != "htop" {
		t.Fatalf("a foreign pid changed the record: %+v", s.resume)
	}
}

// The daemon never hands its own OSC7577_TOKEN down: each terminal makes
// its own, and a program reports only for the terminal that set it.
func TestResumeTokenNotInherited(t *testing.T) {
	t.Setenv("HYLA_CONFIG", t.TempDir()+"/config")
	t.Setenv(resumeTokenEnv, "a-token-of-the-terminal-above")
	d := &Daemon{dir: t.TempDir()}
	for _, kv := range d.environment("s", "p", "/bin/zsh", nil) {
		if strings.HasPrefix(kv, resumeTokenEnv+"=") {
			t.Fatalf("the daemon passed on %q", kv)
		}
	}
}

// A restored pane runs a program's reported command only when the user
// allowed it, and only for the programs they named.
func TestResumeCommandFor(t *testing.T) {
	dir := t.TempDir()
	sp := savedPane{ID: "p", Dir: dir, Resume: &resumeRecord{PID: 9, Argv: []string{"nvim", "-S", "/tmp/s.vim"}}}
	if _, _, ok := resumeCommandFor(sp, settings.Settings{}); ok {
		t.Error("resumed without resume-programs")
	}
	set := settings.Settings{ResumePrograms: true}
	argv, got, ok := resumeCommandFor(sp, set)
	if !ok || strings.Join(argv, " ") != "nvim -S /tmp/s.vim" || got != dir {
		t.Fatalf("resume %q in %q, %v", argv, got, ok)
	}
	set.ResumeCommands = []string{"htop"}
	if _, _, ok := resumeCommandFor(sp, set); ok {
		t.Error("resumed a program the list leaves out")
	}
	set.ResumeCommands = []string{"nvim"}
	if _, _, ok := resumeCommandFor(sp, set); !ok {
		t.Error("refused a program the list names")
	}
	// A list of names matches an absolute path by its name.
	sp.Resume.Argv = []string{"/opt/homebrew/bin/htop"}
	set.ResumeCommands = []string{"htop"}
	if _, _, ok := resumeCommandFor(sp, set); !ok {
		t.Error("refused a path whose name the list names")
	}
	// The reported directory is the one to resume in, and one that is
	// gone is not resumed.
	cwd := t.TempDir()
	sp.Resume.Argv, sp.Resume.Cwd = []string{"nvim"}, cwd
	set.ResumeCommands = nil
	if _, got, ok := resumeCommandFor(sp, set); !ok || got != cwd {
		t.Errorf("resumed in %q, %v, want %q", got, ok, cwd)
	}
	sp.Resume.Cwd = filepath.Join(cwd, "gone")
	if _, _, ok := resumeCommandFor(sp, set); ok {
		t.Error("resumed in a directory that is gone")
	}
}

// A record saved with the pane is a record restored with it.
func TestResumeSaved(t *testing.T) {
	d := testDaemon(t)
	s := &session{id: "s1"}
	p := &pane{d: d, sid: "s1", info: proto.Pane{ID: "p1", Dir: t.TempDir(), Status: proto.Idle}}
	p.resume = &resumeRecord{PID: 9, Argv: []string{"nvim", "-S", "/tmp/s.vim"}, Cwd: "/tmp"}
	s.panes = []*pane{p}
	d.sessions = []*session{s}
	d.saveState()
	b, err := os.ReadFile(d.statePath())
	if err != nil {
		t.Fatal(err)
	}
	var st savedState
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Sessions) != 1 || len(st.Sessions[0].Panes) != 1 {
		t.Fatalf("saved %+v", st.Sessions)
	}
	r := st.Sessions[0].Panes[0].Resume
	if r == nil || r.PID != 9 || strings.Join(r.Argv, " ") != "nvim -S /tmp/s.vim" || r.Cwd != "/tmp" {
		t.Fatalf("saved %+v", r)
	}
}
