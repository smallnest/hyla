package timeline

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Find returns the path of the records of a pane's agent, as the agent
// keeps them: Claude Code's transcript of the conversation in conv, else
// the newest of dir's; DeepSeek Harness's log of dir's newest session;
// Codex's rollout of the conversation, else the newest that ran in dir;
// and OpenCode's store, which keeps every conversation. It returns ""
// where it finds none.
func Find(agent, conv, dir string) string {
	switch agent {
	case "claude":
		return claudeTranscript(conv, dir)
	case "dsh":
		return dshSession(dir)
	case "codex":
		return codexRollout(conv, dir)
	case "opencode":
		return openCodeDB()
	}
	return ""
}

// FindConv returns the agent, and the records it keeps, of a conversation
// whose id names it, for a pane that runs its agent without a terminal and
// names no agent: OpenCode's store, which keeps every conversation under its
// id, Claude Code's transcript of the id, and Codex's rollout. It returns ""
// where none keeps it.
func FindConv(conv string) (agent, file string) {
	if conv == "" {
		return "", ""
	}
	if db := openCodeDB(); db != "" && keepsOpenCode(db, conv) {
		return "opencode", db
	}
	if p := byID(filepath.Join(claudeHome(), "projects"), conv); p != "" {
		return "claude", p
	}
	if p := codexByID(filepath.Join(codexHome(), "sessions"), conv); p != "" {
		return "codex", p
	}
	return "", ""
}

// keepsOpenCode tells whether OpenCode's store keeps the conversation the id
// names.
func keepsOpenCode(path, conv string) bool {
	db, err := openStore(path)
	if err != nil {
		return false
	}
	defer db.Close()
	v2, err := openCodeV2(db)
	if err != nil {
		return false
	}
	id, err := openCodeSession(db, v2, conv, "")
	return err == nil && id == conv
}

// claudeHome is Claude Code's directory, as its own command reads it.
func claudeHome() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".claude")
}

// dshHome is DeepSeek Harness's directory.
func dshHome() string {
	if d := os.Getenv("DSH_HOME"); d != "" {
		return d
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".dsh")
}

// claudeTranscript is the file of a Claude Code conversation: the ID's,
// whose directory the projects' hold, else the newest of dir's.
func claudeTranscript(conv, dir string) string {
	root := filepath.Join(claudeHome(), "projects")
	if conv != "" {
		if file := byID(root, conv); file != "" {
			return file
		}
	}
	if dir == "" {
		return ""
	}
	return newest(filepath.Join(root, encodeDir(dir)), ".jsonl")
}

// byID is the transcript a conversation's ID names, which Claude Code
// keeps in the project of the conversation's directory.
func byID(root, conv string) string {
	// The ID names a file: nothing outside a project's.
	if strings.ContainsAny(conv, `/\`) || strings.HasPrefix(conv, ".") {
		return ""
	}
	projects, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		file := filepath.Join(root, p.Name(), conv+".jsonl")
		if _, err := os.Stat(file); err == nil {
			return file
		}
	}
	return ""
}

// encodeDir is a directory's path as the agents name it: every character
// that is not a letter, a digit or "-", as "-".
func encodeDir(dir string) string {
	var b strings.Builder
	for _, r := range dir {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteString("-")
		}
	}
	return b.String()
}

// newest is the newest file of a directory that ends in suffix, "" for
// none.
func newest(dir, suffix string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	found, when := "", time.Time{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		if st, err := e.Info(); err == nil && !st.ModTime().Before(when) {
			found, when = filepath.Join(dir, e.Name()), st.ModTime()
		}
	}
	return found
}

// dshSession is the log of DeepSeek Harness's newest session of a
// directory: sessions/<dir>/*/session.v4.jsonl.zstd.
func dshSession(dir string) string {
	if dir == "" {
		return ""
	}
	// The sessions' directory is the path's every character that is not a
	// letter, a digit or "-" as "-", between two dashes: encodeDir has
	// one at each end already.
	name := strings.Trim(encodeDir(dir), string(rune(45)))
	root := filepath.Join(dshHome(), "sessions", "--"+name+"--")
	type session struct {
		file string
		at   time.Time
	}
	var all []session
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		file := filepath.Join(root, e.Name(), "session.v4.jsonl.zstd")
		if st, err := os.Stat(file); err == nil {
			all = append(all, session{file, st.ModTime()})
		}
	}
	if len(all) == 0 {
		return ""
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].at.After(all[j].at) })
	return all[0].file
}
