package timeline

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	// OpenCode keeps its records in SQLite, which this package reads.
	_ "modernc.org/sqlite"
)

// OpenCodeRead is how many of a conversation's messages a read parses: the
// end of it, which is the part the user looks at.
const OpenCodeRead = 400

// openCodeDB is the store OpenCode keeps its conversations in: opencode.db,
// in the directory OPENCODE_DATA_DIR names, else the data directory its
// platform gives it.
func openCodeDB() string {
	var dirs []string
	if d := os.Getenv("OPENCODE_DATA_DIR"); d != "" {
		dirs = append(dirs, d)
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		dirs = append(dirs, filepath.Join(d, "opencode"))
	}
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		dirs = append(dirs, filepath.Join(d, "opencode"))
	}
	home, _ := os.UserHomeDir()
	dirs = append(dirs, filepath.Join(home, ".local", "share", "opencode"))
	for _, d := range dirs {
		for _, name := range []string{"opencode.db", "opencode.db3"} {
			p := filepath.Join(d, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}

// OpenCode reads an OpenCode conversation as a timeline: each message the
// user sent begins a turn, and what the model reasoned, wrote and ran
// after it are the turn's steps. A subtask is a subagent, which OpenCode
// runs as a tool or a part of its own, and a tool that waits for the user
// is the turn's waiting.
//
// The conversation is conv's, or, with none, the newest one the store has
// of dir. now is the time a step still running has reached.
func OpenCode(path, conv, dir string, now time.Time) ([]Step, error) {
	db, err := openStore(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	v2, err := openCodeV2(db)
	if err != nil {
		return nil, err
	}
	session, err := openCodeSession(db, v2, conv, dir)
	if err != nil || session == "" {
		return nil, err
	}
	var msgs []openCodeMessage
	if v2 {
		msgs, err = openCodeV2Messages(db, session)
	} else {
		msgs, err = openCodeLegacyMessages(db, session)
	}
	if err != nil {
		return nil, err
	}
	return openCodeTurns(msgs, now), nil
}

// openStore opens OpenCode's store read only: it is the running agent's,
// and a read of it must neither lock nor change it.
func openStore(path string) (*sql.DB, error) {
	u := &url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", "ro")
	q.Set("_pragma", "busy_timeout(2000)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	// A store that will not answer as it is, as one whose write-ahead log
	// this account cannot map, says so now, rather than on the first read.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// openCodeV2 tells which of the store's two shapes it has: the messages in
// tables of their own, which OpenCode writes now, or a message and a part
// in two tables, which it wrote before.
func openCodeV2(db *sql.DB) (bool, error) {
	return openCodeTable(db, "session_message")
}

// openCodeSession is the conversation a read parses: conv's, or the newest
// of the store's in dir.
func openCodeSession(db *sql.DB, v2 bool, conv, dir string) (string, error) {
	table := "session"
	if v2 {
		table = "session_v2"
	}
	if strings.HasPrefix(conv, "ses_") {
		var id string
		err := db.QueryRow(`select id from `+table+` where id = ?`, conv).Scan(&id)
		switch {
		case err == nil:
			return id, nil
		case err != sql.ErrNoRows:
			return "", err
		}
	}
	if dir == "" {
		return "", nil
	}
	var id string
	// A subtask's session has a parent, and is a step of its own: the
	// pane's conversation is one without.
	err := db.QueryRow(`select id from `+table+` where directory = ? and parent_id is null
		order by time_updated desc limit 1`, dir).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// openCodeMessage is a message of the store: who wrote it, when, and its
// parts.
type openCodeMessage struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Parts []openCodePart
}

// openCodePart is a part of a message: its reasoning, its words, a tool it
// ran, or a subtask it gave its work to. The store's two shapes name
// their times differently, so a part reads all of them.
type openCodePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Time struct {
		Created   int64 `json:"created"`
		Ran       int64 `json:"ran"`
		Completed int64 `json:"completed"`
		Start     int64 `json:"start"`
		End       int64 `json:"end"`
	} `json:"time"`
	// Name is the tool's, in the shape that tells it so; Tool in the
	// other.
	Name  string `json:"name"`
	Tool  string `json:"tool"`
	Call  string `json:"callID"`
	State struct {
		Status string          `json:"status"`
		Input  json.RawMessage `json:"input"`
		Output string          `json:"output"`
		// Content is what a tool said, in the shape the store writes now:
		// its output as parts, as a text of its own.
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error string `json:"error"`
		Title string `json:"title"`
		Time  struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		} `json:"time"`
	} `json:"state"`
	// A subtask's: the agent it runs, and what it was asked.
	Agent       string `json:"agent"`
	Description string `json:"description"`

	// at is when the store wrote the part, for a part that tells no time
	// of its own.
	at int64
}

// openCodeV2Message is a message of the store as it writes it now: the
// message and its parts together.
type openCodeV2Message struct {
	Time struct {
		Created   int64 `json:"created"`
		Completed int64 `json:"completed"`
	} `json:"time"`
	Text    string         `json:"text"`
	Content []openCodePart `json:"content"`
}

// openCodeV2Messages reads a conversation's messages, the last
// OpenCodeRead of it, in the order the store wrote them. A message's parts
// come with it, or, in the shape the store writes now, from the table
// beside it (session_part), which leaves the message's own data with none.
func openCodeV2Messages(db *sql.DB, session string) ([]openCodeMessage, error) {
	rows, err := db.Query(`select id, type, time_created, data from session_message
		where session_id = ? and type in ('user', 'assistant')
		order by seq desc limit ?`, session, OpenCodeRead)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []openCodeMessage
	for rows.Next() {
		var m openCodeMessage
		var raw string
		if err := rows.Scan(&m.ID, &m.Role, &m.Start, &raw); err != nil {
			return nil, err
		}
		var d openCodeV2Message
		if json.Unmarshal([]byte(raw), &d) != nil {
			continue
		}
		if d.Time.Created != 0 {
			m.Start = d.Time.Created
		}
		m.End = d.Time.Completed
		if m.Role == "user" {
			m.Parts = []openCodePart{{Type: "text", Text: d.Text, at: m.Start}}
		} else {
			m.Parts = d.Content
			for i := range m.Parts {
				if m.Parts[i].at == 0 {
					m.Parts[i].at = m.Start
				}
			}
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The query reads the newest first, and the timeline reads in time.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	if err := openCodeV2Parts(db, msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// openCodeV2Parts reads the parts a message's own data did not carry, as
// the store writes them now: a row a part, in the message's order, in the
// table session_part. A message whose data has its parts keeps them.
func openCodeV2Parts(db *sql.DB, msgs []openCodeMessage) error {
	byID := make(map[string]int, len(msgs))
	var ids []any
	for i, m := range msgs {
		if m.Role != "assistant" || len(m.Parts) > 0 {
			continue
		}
		byID[m.ID] = i
		ids = append(ids, m.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	// A store that writes a message's parts in its data, as the ones
	// before do, has no table beside it.
	ok, err := openCodeTable(db, "session_part")
	if err != nil || !ok {
		return err
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	rows, err := db.Query(`select message_id, time_created, data from session_part
		where message_id in (`+marks+`) order by message_id, ordinal`, ids...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var msg, raw string
		var at int64
		if err := rows.Scan(&msg, &at, &raw); err != nil {
			return err
		}
		i, ok := byID[msg]
		if !ok {
			continue
		}
		var p openCodePart
		if json.Unmarshal([]byte(raw), &p) != nil {
			continue
		}
		p.at = at
		msgs[i].Parts = append(msgs[i].Parts, p)
	}
	return rows.Err()
}

// openCodeTable tells whether the store keeps a table: the versions of the
// store differ in the tables they have.
func openCodeTable(db *sql.DB, name string) (bool, error) {
	var n int
	err := db.QueryRow(`select count(*) from sqlite_master where type = 'table' and name = ?`, name).Scan(&n)
	return n > 0, err
}

// openCodeLegacyMessages reads a conversation's messages and their parts,
// as the store wrote them before, in the order they were written.
func openCodeLegacyMessages(db *sql.DB, session string) ([]openCodeMessage, error) {
	rows, err := db.Query(`select id, json_extract(data, '$.role'), time_created,
		coalesce(json_extract(data, '$.time.completed'), 0)
		from message where session_id = ? order by time_created desc, id desc limit ?`, session, OpenCodeRead)
	if err != nil {
		return nil, err
	}
	var msgs []openCodeMessage
	for rows.Next() {
		var m openCodeMessage
		if err := rows.Scan(&m.ID, &m.Role, &m.Start, &m.End); err != nil {
			rows.Close()
			return nil, err
		}
		msgs = append(msgs, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The query reads the newest first, and the timeline reads in time.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	byID := make(map[string]int, len(msgs))
	for i := range msgs {
		byID[msgs[i].ID] = i
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	parts, err := db.Query(`select message_id, time_created, data from part where session_id = ?
		order by time_created, id`, session)
	if err != nil {
		return nil, err
	}
	defer parts.Close()
	for parts.Next() {
		var msg, raw string
		var at int64
		if err := parts.Scan(&msg, &at, &raw); err != nil {
			return nil, err
		}
		i, ok := byID[msg]
		if !ok {
			continue
		}
		var p openCodePart
		if json.Unmarshal([]byte(raw), &p) != nil {
			continue
		}
		p.at = at
		msgs[i].Parts = append(msgs[i].Parts, p)
	}
	return msgs, parts.Err()
}

// openCodeTurns lays the messages out as turns: the user's message is the
// turn, and what the model did after it are its steps.
func openCodeTurns(msgs []openCodeMessage, now time.Time) []Step {
	var turns []Step
	// open tells that the last answer of the turn has not ended: the turn
	// runs.
	open := false
	for _, m := range msgs {
		switch m.Role {
		case "user":
			turns = append(turns, Step{Kind: Turn, Title: line(m.prompt(), 120), Full: m.prompt(), Start: m.Start, State: Running})
			open = false
		case "assistant":
			if len(turns) == 0 {
				// An answer with no prompt before it, as a summary's: no
				// turn of the user's.
				continue
			}
			cur := &turns[len(turns)-1]
			cur.Children = append(cur.Children, openCodeSteps(m)...)
			open = m.End == 0
			if !open {
				cur.End = m.End
			}
		}
	}
	nowMs := now.UnixMilli()
	for i := range turns {
		closeTurnEnds(&turns[i], open && i == len(turns)-1, nowMs)
	}
	return order(turns)
}

// openCodeSteps is what a message did: its reasoning, its words, the tools
// it ran and the subtasks it gave its work to. A part that tells no end of
// its own ended with its message.
func openCodeSteps(m openCodeMessage) []Step {
	var steps []Step
	for _, p := range m.Parts {
		switch p.Type {
		case "reasoning", "text":
			if strings.TrimSpace(p.Text) == "" {
				continue
			}
			kind := Think
			if p.Type == "text" {
				kind = Text
			}
			steps = append(steps, Step{Kind: kind, Title: line(p.Text, 80),
				Start: p.start(), End: p.end(), State: Done})
		case "tool":
			steps = append(steps, openCodeTool(p))
		case "subtask":
			steps = append(steps, openCodeSubtask(p))
		}
	}
	if m.End != 0 {
		for i := range steps {
			if steps[i].End != 0 || steps[i].State == Waiting {
				continue
			}
			steps[i].End, steps[i].State = m.End, Done
		}
	}
	return steps
}

// openCodeTool is a tool a message ran: what it was and what it said, from
// when it began to when it ended, or what it waits for.
func openCodeTool(p openCodePart) Step {
	name := firstOf(p.Name, p.Tool)
	out := p.output()
	st := Step{Kind: Tool, Title: name, Detail: line(firstOf(p.State.Title, short(p.State.Input, 200)), 200),
		Full:  firstOf(firstOf(out, p.State.Title), string(p.State.Input)),
		Start: p.start(), End: p.end(), State: Running}
	if st.Title == "" {
		st.Title = "tool"
	}
	if openCodeSubagent(name) {
		// An agent of its own, which OpenCode runs as a tool, named for
		// what it was asked.
		st.Kind, st.Detail, st.Title = Subagent, name, name
		var in struct {
			Description string `json:"description"`
			Agent       string `json:"subagent_type"`
		}
		if json.Unmarshal(p.State.Input, &in) == nil {
			if in.Description != "" {
				st.Title = line(in.Description, 120)
			} else if in.Agent != "" {
				st.Title = in.Agent
			}
		}
	}
	switch p.State.Status {
	case "completed":
		st.State = Done
		if st.End == 0 {
			st.End = max(p.at, st.Start)
		}
	case "error":
		st.State = Failed
		if st.End == 0 {
			st.End = max(p.at, st.Start)
		}
		st.Detail = joinLines(st.Detail, line(firstOf(p.State.Error, out), 160))
		st.Full = joinLines(st.Full, firstOf(p.State.Error, out))
	case "pending":
		// It asks the user: the turn waits on it.
		st.State, st.End = Waiting, 0
	}
	return st
}

// openCodeSubtask is a subagent a message started: what it was asked, and
// the agent that ran.
func openCodeSubtask(p openCodePart) Step {
	st := Step{Kind: Subagent, Title: line(p.Description, 120), Detail: p.Agent,
		Start: p.start(), State: Running}
	if st.Title == "" {
		st.Title = p.Agent
	}
	return st
}

// openCodeSubagent tells whether a tool runs an agent of its own.
func openCodeSubagent(name string) bool {
	switch name {
	case "task", "subagent", "tool_use":
		return true
	}
	return false
}

// output is what a tool said: its output, in the shape that names it so,
// or the texts of its content, in the shape that keeps its words there.
func (p openCodePart) output() string {
	if strings.TrimSpace(p.State.Output) != "" {
		return p.State.Output
	}
	var b strings.Builder
	for _, c := range p.State.Content {
		if c.Type != "" && c.Type != "text" {
			continue
		}
		if strings.TrimSpace(c.Text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(c.Text)
	}
	return b.String()
}

// start is when a part began: its own time, as either shape of the store
// gives it, else the message's.
func (p openCodePart) start() int64 {
	for _, t := range []int64{p.Time.Start, p.Time.Ran, p.State.Time.Start, p.Time.Created, p.at} {
		if t != 0 {
			return t
		}
	}
	return 0
}

// end is when a part ended, 0 while it runs.
func (p openCodePart) end() int64 {
	for _, t := range []int64{p.Time.Completed, p.Time.End, p.State.Time.End} {
		if t != 0 {
			return t
		}
	}
	return 0
}

// prompt is a user message's words: its text parts'.
func (m openCodeMessage) prompt() string {
	var b strings.Builder
	for _, p := range m.Parts {
		if p.Type != "text" || strings.TrimSpace(p.Text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.Text)
	}
	return b.String()
}

// closeTurnEnds gives a turn's steps their ends: what runs reaches now, an
// approval waits where it is, and the turn ends with its last step, or
// runs while one runs or waits, or while its last answer is open.
func closeTurnEnds(t *Step, open bool, nowMs int64) {
	runs := open
	for i := range t.Children {
		c := &t.Children[i]
		switch c.State {
		case Waiting:
			// An approval waits for the user: the timeline shows it, and
			// the turn has not ended.
			runs = true
			c.End = 0
			continue
		case Running:
			runs = true
		}
		if c.End == 0 {
			c.End = max(nowMs, c.Start)
		}
	}
	if runs {
		t.End, t.State = 0, Running
		return
	}
	t.State = Done
	// The turn ends with its answer, which its message told; one that
	// told no time of its own ends with its last step.
	if t.End != 0 {
		return
	}
	for i := len(t.Children) - 1; i >= 0; i-- {
		if t.Children[i].End != 0 {
			t.End = t.Children[i].End
			return
		}
	}
	t.End = nowMs
}

// firstOf is the first of two words that is set.
func firstOf(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
