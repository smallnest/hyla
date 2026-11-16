package timeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A directory's name in the agents' records is its every character that is
// not a letter, a digit or "-", as "-": Claude Code's projects use it, and
// DeepSeek Harness's sessions put it between two dashes.
func TestEncodeDir(t *testing.T) {
	if got := encodeDir("/Users/me/ai/my-app_v2"); got != "-Users-me-ai-my-app-v2" {
		t.Errorf("encodeDir %q", got)
	}
	if got := "--" + strings.Trim(encodeDir("/Users/me/ai/hyla"), "-") + "--"; got != "--Users-me-ai-hyla--" {
		t.Errorf("the DSH name %q", got)
	}
}

// Find reads the agents' records from the test's home.
func TestFind(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("DSH_HOME", "")
	dir := "/Users/me/api"
	claude := filepath.Join(home, ".claude", "projects", "-Users-me-api")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(claude, "old.jsonl")
	newest := filepath.Join(claude, "new.jsonl")
	for _, f := range []string{old, newest} {
		if err := os.WriteFile(f, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	os.Chtimes(old, now.Add(-time.Hour), now.Add(-time.Hour))
	os.Chtimes(newest, now, now)
	if got := Find("claude", "", dir); got != newest {
		t.Errorf("claude %q, want %q", got, newest)
	}
	if got := Find("claude", "new", dir); got != newest {
		t.Errorf("claude by id %q, want %q", got, newest)
	}
	if got := Find("claude", "../etc/passwd", ""); got != "" {
		t.Errorf("claude by a bad id %q", got)
	}

	session := filepath.Join(home, ".dsh", "sessions", "--Users-me-api--", "session-1")
	if err := os.MkdirAll(session, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(session, "session.v4.jsonl.zstd")
	if err := os.WriteFile(log, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Find("dsh", "", dir); got != log {
		t.Errorf("dsh %q, want %q", got, log)
	}
	if got := Find("codex", "", dir); got != "" {
		t.Errorf("codex %q", got)
	}
}
