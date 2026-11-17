package timeline

import (
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// openTestStore makes an OpenCode store of the shape given, as the app
// writes it: v2 with the messages and their parts together, or the one
// before, a message and a part of its own. It returns the store's path.
func openTestStore(t *testing.T, v2 bool) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	stmts := []string{
		`create table session (id text primary key, directory text, parent_id text, time_updated integer)`,
		`create table message (id text primary key, session_id text, time_created integer, data text)`,
		`create table part (id text primary key, message_id text, session_id text, time_created integer, data text)`,
	}
	if v2 {
		stmts = []string{
			`create table session_v2 (id text primary key, directory text, parent_id text, time_updated integer)`,
			`create table session_message (id text primary key, session_id text, type text, seq integer, time_created integer, data text)`,
		}
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	return path, db
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

// js is a time as the store writes it.
func js(n int64) string { return strconv.FormatInt(n, 10) }

// summary is a timeline, one line a turn and one each of its steps: the
// kind, the title, the times and what it does.
func summary(turns []Step) []string {
	var out []string
	for _, t := range turns {
		out = append(out, string(t.Kind)+" "+t.Title+" "+times(t)+" "+string(t.State))
		for _, c := range t.Children {
			out = append(out, "  "+string(c.Kind)+" "+c.Title+" "+times(c)+" "+string(c.State))
		}
	}
	return out
}

func times(s Step) string {
	end := "-"
	if s.End != 0 {
		end = time.UnixMilli(s.End).UTC().Format("15:04:05.000")
	}
	return time.UnixMilli(s.Start).UTC().Format("15:04:05.000") + ".." + end
}

// A user message begins a turn, and what the model reasoned, wrote and ran
// after it are the turn's steps, from when each began to when it ended; a
// subtask is a subagent, and a tool that asks the user is the turn's
// waiting.
func TestOpenCodeTimeline(t *testing.T) {
	path, db := openTestStore(t, true)
	exec(t, db, `insert into session_v2 (id, directory, time_updated) values ('ses_1', '/tmp/app', 0)`)
	// A subtask's own session is a step of the pane's, not one of its own.
	exec(t, db, `insert into session_v2 (id, directory, parent_id, time_updated) values ('ses_2', '/tmp/app', 'ses_1', 0)`)
	// The first turn, which ended.
	exec(t, db, `insert into session_message (id, session_id, type, seq, time_created, data) values ('m1', 'ses_1', 'user', 10, ?, ?)`,
		ms(0), `{"time":{"created":`+js(ms(0))+`},"text":"fix the login bug"}`)
	exec(t, db, `insert into session_message (id, session_id, type, seq, time_created, data) values ('m2', 'ses_1', 'assistant', 11, ?, ?)`,
		ms(time.Second), `{"time":{"created":`+js(ms(time.Second))+`,"completed":`+js(ms(50*time.Second))+`},"content":[
			{"type":"reasoning","text":"Look at the file first.","time":{"created":`+js(ms(time.Second))+`,"completed":`+js(ms(5*time.Second))+`}},
			{"type":"tool","name":"read","state":{"status":"completed","input":{"filePath":"/a.go"}},"time":{"created":`+js(ms(5*time.Second))+`,"ran":`+js(ms(6*time.Second))+`,"completed":`+js(ms(7*time.Second))+`}},
			{"type":"tool","name":"shell","state":{"status":"error","input":{"command":"go test ./auth/"},"error":"exit status 1"},"time":{"created":`+js(ms(8*time.Second))+`,"ran":`+js(ms(9*time.Second))+`,"completed":`+js(ms(40*time.Second))+`}},
			{"type":"tool","name":"task","state":{"status":"completed","input":{"description":"explore the store"}},"time":{"created":`+js(ms(41*time.Second))+`,"completed":`+js(ms(48*time.Second))+`}},
			{"type":"text","text":"Fixed it.","time":{"created":`+js(ms(48*time.Second))+`,"completed":`+js(ms(49*time.Second))+`}}
		]}`)
	// The second turn, still running, with a tool that asks the user.
	exec(t, db, `insert into session_message (id, session_id, type, seq, time_created, data) values ('m3', 'ses_1', 'user', 30, ?, ?)`,
		ms(60*time.Second), `{"time":{"created":`+js(ms(60*time.Second))+`},"text":"and the tests?"}`)
	exec(t, db, `insert into session_message (id, session_id, type, seq, time_created, data) values ('m4', 'ses_1', 'assistant', 31, ?, ?)`,
		ms(61*time.Second), `{"time":{"created":`+js(ms(61*time.Second))+`},"content":[
			{"type":"reasoning","text":"Run them.\nThen read the failures.","time":{"created":`+js(ms(61*time.Second))+`,"completed":`+js(ms(63*time.Second))+`}},
			{"type":"tool","name":"shell","state":{"status":"pending","input":{"command":"go test ./..."}},"time":{"created":`+js(ms(63*time.Second))+`}}
		]}`)

	steps, err := OpenCode(path, "", "/tmp/app", when.Add(120*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"turn fix the login bug 10:00:00.000..10:00:50.000 done",
		"  think Look at the file first. 10:00:01.000..10:00:05.000 done",
		"  tool read 10:00:06.000..10:00:07.000 done",
		"  tool shell 10:00:09.000..10:00:40.000 failed",
		"  subagent explore the store 10:00:41.000..10:00:48.000 done",
		"  text Fixed it. 10:00:48.000..10:00:49.000 done",
		"turn and the tests? 10:01:00.000..- running",
		"  think Run them. 10:01:01.000..10:01:03.000 done",
		"  tool shell 10:01:03.000..- waiting",
	}
	check(t, summary(steps), want)
}

// The store before the messages and their parts together is read too: its
// tool's times are the part's, and its subtask a part of the message.
func TestOpenCodeLegacyStore(t *testing.T) {
	path, db := openTestStore(t, false)
	exec(t, db, `insert into session (id, directory, time_updated) values ('ses_1', '/tmp/app', 0)`)
	exec(t, db, `insert into message (id, session_id, time_created, data) values ('m1', 'ses_1', ?, ?)`,
		ms(0), `{"role":"user","time":{"created":`+js(ms(0))+`}}`)
	exec(t, db, `insert into message (id, session_id, time_created, data) values ('m2', 'ses_1', ?, ?)`,
		ms(0), `{"role":"assistant","time":{"created":`+js(ms(0))+`,"completed":`+js(ms(20*time.Second))+`}}`)
	exec(t, db, `insert into part (id, message_id, session_id, time_created, data) values ('p1', 'm1', 'ses_1', ?, '{"type":"text","text":"what broke?"}')`, ms(0))
	exec(t, db, `insert into part (id, message_id, session_id, time_created, data) values ('p2', 'm2', 'ses_1', ?, ?)`, ms(0),
		`{"type":"tool","tool":"bash","state":{"status":"completed","title":"go build ./...","time":{"start":`+js(ms(2*time.Second))+`,"end":`+js(ms(9*time.Second))+`}}}`)
	exec(t, db, `insert into part (id, message_id, session_id, time_created, data) values ('p3', 'm2', 'ses_1', ?, ?)`, ms(0),
		`{"type":"subtask","agent":"explore","description":"the store's tests"}`)

	steps, err := OpenCode(path, "", "/tmp/app", when.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"turn what broke? 10:00:00.000..10:00:20.000 done",
		"  tool bash 10:00:02.000..10:00:09.000 done",
		"  subagent the store's tests 10:00:00.000..10:00:20.000 done",
	}
	check(t, summary(steps), want)
}

// A read takes the conversation the pane names, or the newest of the
// pane's directory; one the store has never seen has no turns.
func TestOpenCodeSession(t *testing.T) {
	path, db := openTestStore(t, true)
	exec(t, db, `insert into session_v2 (id, directory, time_updated) values ('ses_old', '/tmp/app', 1)`)
	exec(t, db, `insert into session_v2 (id, directory, time_updated) values ('ses_new', '/tmp/app', 2)`)
	exec(t, db, `insert into session_v2 (id, directory, time_updated) values ('ses_other', '/tmp/other', 3)`)
	for _, s := range []string{"ses_old", "ses_new", "ses_other"} {
		exec(t, db, `insert into session_message (id, session_id, type, seq, time_created, data) values (?, ?, 'user', 1, ?, '{"text":"hi"}')`,
			"m-"+s, s, ms(0))
	}
	for _, c := range []struct{ conv, dir, want string }{
		{"", "/tmp/app", "ses_new"},
		{"ses_old", "", "ses_old"},
		{"ses_unknown", "/tmp/app", "ses_new"},
		{"", "/tmp/none", ""},
	} {
		got, err := sessionOf(t, path, c.conv, c.dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("conv %q in %q read %q, want %q", c.conv, c.dir, got, c.want)
		}
	}
}

// sessionOf is the conversation a read of the store takes.
func sessionOf(t *testing.T, path, conv, dir string) (string, error) {
	t.Helper()
	db, err := openStore(path)
	if err != nil {
		return "", err
	}
	defer db.Close()
	v2, err := openCodeV2(db)
	if err != nil {
		return "", err
	}
	return openCodeSession(db, v2, conv, dir)
}

func check(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("timeline is %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d is %q, want %q", i, got[i], want[i])
		}
	}
}

// A conversation whose id names it, in a pane that names no agent, is found
// by the store that keeps it.
func TestFindConv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OPENCODE_DATA_DIR", home)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	path := filepath.Join(home, "opencode.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec(t, db, `create table session (id text primary key, directory text, parent_id text, time_updated integer)`)
	exec(t, db, `insert into session (id, directory, time_updated) values ('ses_abc', '/tmp/p', 1)`)

	if agent, file := FindConv("ses_abc"); agent != "opencode" || file != path {
		t.Fatalf("FindConv %q %q, want opencode's store", agent, file)
	}
	if agent, _ := FindConv("ses_nonesuch"); agent != "" {
		t.Errorf("FindConv of a conversation none keeps: %q", agent)
	}
	if agent, _ := FindConv(""); agent != "" {
		t.Errorf("FindConv of no id: %q", agent)
	}
}
