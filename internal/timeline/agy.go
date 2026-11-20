package timeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// AgyRead bounds how much of an Antigravity CLI transcript a read parses: the
// end of it, which is the part the user looks at.
const AgyRead = 8 << 20

const agyHistoryRead = 2 << 20

// agyHome is Antigravity CLI's data directory.
func agyHome() string {
	for _, env := range []string{"AGY_DATA_DIR", "ANTIGRAVITY_DATA_DIR"} {
		if d := os.Getenv(env); d != "" {
			return d
		}
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".gemini", "antigravity-cli")
}

// agyTranscriptPath returns the transcript path for conv if it exists and is a regular file.
func agyTranscriptPath(root, conv string) string {
	if conv == "" || strings.ContainsAny(conv, `/\`) || strings.HasPrefix(conv, ".") {
		return ""
	}
	p := filepath.Join(root, "brain", conv, ".system_generated", "logs", "transcript.jsonl")
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return ""
	}
	return p
}

// agyByID is the Antigravity CLI transcript named by conversation ID.
func agyByID(root, conv string) string {
	return agyTranscriptPath(root, conv)
}

// agyTranscript finds the transcript for conv, else the newest top-level
// Antigravity CLI conversation that ran in dir.
func agyTranscript(conv, dir string) string {
	root := agyHome()
	if conv != "" {
		if p := agyTranscriptPath(root, conv); p != "" {
			return p
		}
	}
	if dir == "" {
		return ""
	}
	dbPath := agyFromSummariesDB(root, dir)
	histPath := agyFromHistory(root, dir)
	switch {
	case dbPath == "":
		return histPath
	case histPath == "" || dbPath == histPath:
		return dbPath
	}
	dbSt, dbErr := os.Stat(dbPath)
	histSt, histErr := os.Stat(histPath)
	if histErr == nil && (dbErr != nil || histSt.ModTime().After(dbSt.ModTime())) {
		return histPath
	}
	return dbPath
}

func agyFromSummariesDB(root, dir string) string {
	dbFile := filepath.Join(root, "conversation_summaries.db")
	st, err := os.Stat(dbFile)
	if err != nil || st.IsDir() {
		return ""
	}
	db, err := openStore(dbFile)
	if err != nil {
		return ""
	}
	defer db.Close()

	rows, err := db.Query(`SELECT conversation_id, workspace_uris
		FROM conversation_summaries
		WHERE coalesce(parent_conversation_id, '') = ''
		ORDER BY last_modified_time DESC`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	for rows.Next() {
		var id, rawURIs string
		if err := rows.Scan(&id, &rawURIs); err != nil {
			return ""
		}
		if !agyWorkspaceMatches(rawURIs, dir) {
			continue
		}
		if p := agyTranscriptPath(root, id); p != "" {
			return p
		}
	}
	return ""
}

func agyWorkspaceMatches(rawURIs, dir string) bool {
	if strings.TrimSpace(rawURIs) == "" {
		return false
	}
	var uris []string
	if json.Unmarshal([]byte(rawURIs), &uris) != nil {
		return false
	}
	for _, u := range uris {
		if agyURIMatchesDir(u, dir) {
			return true
		}
	}
	return false
}

func agyURIMatchesDir(rawURI, dir string) bool {
	if rawURI == "" || dir == "" {
		return false
	}
	if !strings.Contains(rawURI, "://") {
		return agySameDir(rawURI, dir)
	}
	u, err := url.Parse(rawURI)
	if err != nil || !strings.EqualFold(u.Scheme, "file") {
		return false
	}
	p := u.Path
	if len(p) >= 3 && p[0] == '/' && ((p[1] >= 'A' && p[1] <= 'Z') || (p[1] >= 'a' && p[1] <= 'z')) && p[2] == ':' {
		p = p[1:]
	}
	return agySameDir(filepath.FromSlash(p), dir)
}

func agySameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ca, cb := filepath.Clean(a), filepath.Clean(b)
	if ca == cb || (runtime.GOOS == "windows" && strings.EqualFold(ca, cb)) {
		return true
	}
	ea, errA := filepath.EvalSymlinks(ca)
	eb, errB := filepath.EvalSymlinks(cb)
	if errA == nil && errB == nil {
		return ea == eb || (runtime.GOOS == "windows" && strings.EqualFold(ea, eb))
	}
	return false
}

func agyFromHistory(root, dir string) string {
	b, err := readTail(filepath.Join(root, "history.jsonl"), agyHistoryRead)
	if err != nil {
		return ""
	}
	lines := bytes.Split(b, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		raw := bytes.TrimSpace(lines[i])
		if len(raw) == 0 {
			continue
		}
		var item struct {
			Workspace      string `json:"workspace"`
			ConversationID string `json:"conversationId"`
		}
		if json.Unmarshal(raw, &item) != nil || item.ConversationID == "" {
			continue
		}
		if !agySameDir(item.Workspace, dir) {
			continue
		}
		if p := agyTranscriptPath(root, item.ConversationID); p != "" {
			return p
		}
	}
	return ""
}

// AgyFile reads an Antigravity CLI transcript as a timeline, bounded by AgyRead.
func AgyFile(path string, now time.Time) ([]Step, error) {
	b, err := readTail(path, AgyRead)
	if err != nil {
		return nil, err
	}
	start := now
	if st, err := os.Stat(path); err == nil {
		start = st.ModTime().Add(-time.Hour)
	}
	return Agy(b, start, now), nil
}

// AgyModel returns the model last reported by a PLANNER_RESPONSE entry in path.
func AgyModel(path string) string {
	b, err := readTail(path, AgyRead)
	if err != nil {
		return ""
	}
	model := ""
	for _, l := range bytes.Split(b, []byte("\n")) {
		l = bytes.TrimSpace(l)
		if len(l) == 0 {
			continue
		}
		var e struct {
			Type  string `json:"type"`
			Model string `json:"model"`
		}
		if json.Unmarshal(l, &e) == nil && e.Type == "PLANNER_RESPONSE" && e.Model != "" {
			model = e.Model
		}
	}
	return model
}

type agyEntry struct {
	StepIndex int           `json:"step_index"`
	Source    string        `json:"source"`
	Type      string        `json:"type"`
	Status    string        `json:"status"`
	CreatedAt string        `json:"created_at"`
	Model     string        `json:"model"`
	Content   string        `json:"content"`
	Thinking  string        `json:"thinking"`
	Error     string        `json:"error"`
	ToolCalls []agyToolCall `json:"tool_calls"`
}

type agyToolCall struct {
	Name string                     `json:"name"`
	Args map[string]json.RawMessage `json:"args"`
}

type agyStepRef struct {
	turn  int
	child int
}

type agyPendingCall struct {
	ref      agyStepRef
	toolName string
}

// Agy reads an Antigravity CLI compact JSONL transcript as a timeline.
func Agy(data []byte, start, now time.Time) []Step {
	var turns []Step
	cur := -1
	var pendingCalls []agyPendingCall
	var plannerOpen []agyStepRef
	activeBG := map[string]agyStepRef{}
	subagentSteps := map[string]agyStepRef{}
	lastEndedTurn := false

	lastClock := start.UnixMilli()
	nowMs := now.UnixMilli()
	at := func(rawTime string) int64 {
		if t, ok := parseTime(rawTime); ok {
			lastClock = t
			return t
		}
		lastClock++
		return lastClock
	}

	closeRefs := func(refs []agyStepRef, end int64) {
		for _, r := range refs {
			if r.turn < 0 || r.turn >= len(turns) || r.child < 0 || r.child >= len(turns[r.turn].Children) {
				continue
			}
			c := &turns[r.turn].Children[r.child]
			if c.End == 0 {
				c.End = max(end, c.Start)
				if c.State == Running {
					c.State = Done
				}
			}
		}
	}

	stepAt := func(r agyStepRef) *Step {
		if r.turn < 0 || r.turn >= len(turns) || r.child < 0 || r.child >= len(turns[r.turn].Children) {
			return nil
		}
		return &turns[r.turn].Children[r.child]
	}

	for _, l := range bytes.Split(data, []byte("\n")) {
		l = bytes.TrimSpace(l)
		if len(l) == 0 {
			continue
		}
		var e agyEntry
		if json.Unmarshal(l, &e) != nil {
			continue
		}
		t := at(e.CreatedAt)

		switch e.Type {
		case "USER_INPUT":
			text := agyUserPrompt(e.Content)
			if text == "" {
				continue
			}
			if n := len(turns); n > 0 && turns[n-1].End == 0 {
				agyCloseFinishedTurn(&turns[n-1], t)
			}
			turns = append(turns, Step{
				Kind:  Turn,
				Title: line(text, 120),
				Full:  text,
				Start: t,
				State: Running,
			})
			cur = len(turns) - 1
			pendingCalls = nil
			plannerOpen = nil
			activeBG = map[string]agyStepRef{}
			subagentSteps = map[string]agyStepRef{}
			lastEndedTurn = false

		case "PLANNER_RESPONSE":
			if cur < 0 {
				continue
			}
			for _, pc := range pendingCalls {
				if st := stepAt(pc.ref); st != nil && st.End == 0 && st.State != Waiting {
					st.End = max(t, st.Start)
					if st.State == Running {
						st.State = Done
					}
				}
			}
			pendingCalls = nil
			closeRefs(plannerOpen, t)
			plannerOpen = nil

			if thinking := strings.TrimSpace(e.Thinking); thinking != "" {
				turns[cur].Children = append(turns[cur].Children, Step{
					Kind:  Think,
					Title: line(thinking, 80),
					Full:  thinking,
					Start: t,
					State: Running,
				})
				plannerOpen = append(plannerOpen, agyStepRef{turn: cur, child: len(turns[cur].Children) - 1})
			}
			if content := strings.TrimSpace(e.Content); content != "" {
				turns[cur].Children = append(turns[cur].Children, Step{
					Kind:  Text,
					Title: line(content, 80),
					Full:  content,
					Start: t,
					State: Running,
				})
				plannerOpen = append(plannerOpen, agyStepRef{turn: cur, child: len(turns[cur].Children) - 1})
			}

			if len(e.ToolCalls) > 0 {
				lastEndedTurn = false
				for _, tc := range e.ToolCalls {
					st := agyBuildToolStep(tc, t)
					turns[cur].Children = append(turns[cur].Children, st)
					pendingCalls = append(pendingCalls, agyPendingCall{
						ref:      agyStepRef{turn: cur, child: len(turns[cur].Children) - 1},
						toolName: tc.Name,
					})
				}
			} else {
				closeRefs(plannerOpen, t)
				plannerOpen = nil
				if len(activeBG) == 0 && e.Status == "DONE" {
					lastEndedTurn = true
				}
			}

		case "SYSTEM_MESSAGE":
			if cur < 0 {
				continue
			}
			sender, body := agyParseSystemMessage(e.Content)
			if sender == "" {
				continue
			}
			if ref, ok := activeBG[sender]; ok {
				if st := stepAt(ref); st != nil {
					st.End = max(t, st.Start)
					if agySystemMsgFailed(body) {
						st.State = Failed
					} else {
						st.State = Done
					}
					if st.Kind != Subagent && body != "" {
						st.Detail = joinLines(st.Detail, line(body, 160))
						st.Full = joinLines(st.Full, body)
					}
				}
				delete(activeBG, sender)
				lastEndedTurn = false
			} else if ref, ok := subagentSteps[sender]; ok {
				if st := stepAt(ref); st != nil {
					st.End = max(t, st.Start)
				}
			}

		case "ERROR_MESSAGE":
			if cur < 0 {
				continue
			}
			msg := firstOf(e.Error, e.Content)
			if strings.TrimSpace(msg) != "" {
				turns[cur].Children = append(turns[cur].Children, Step{
					Kind:  Text,
					Title: line(msg, 80),
					Full:  msg,
					Start: t,
					End:   t,
					State: Failed,
				})
			}

		case "CHECKPOINT", "CONVERSATION_HISTORY":
			continue

		default:
			if cur < 0 || e.Source != "MODEL" || len(pendingCalls) == 0 {
				continue
			}
			call := pendingCalls[0]
			pendingCalls = pendingCalls[1:]

			createdAt, completedAt, body := agyParseToolResult(e.Content, t)
			closeRefs(plannerOpen, createdAt)
			plannerOpen = nil

			st := stepAt(call.ref)
			if st == nil {
				continue
			}
			if createdAt > st.Start {
				st.Start = createdAt
			}

			switch {
			case e.Status == "ERROR" || e.Error != "":
				errMsg := firstOf(e.Error, body)
				st.End = max(completedAt, st.Start)
				st.State = Failed
				if errMsg != "" {
					if st.Kind != Subagent {
						st.Detail = joinLines(st.Detail, line(errMsg, 160))
					}
					st.Full = joinLines(st.Full, errMsg)
				}

			case e.Status == "RUNNING":
				if taskID := agyExtractTaskID(body); taskID != "" {
					st.State = Running
					st.End = 0
					activeBG[taskID] = call.ref
				} else {
					st.State = Running
					st.End = 0
				}

			case st.Kind == Subagent:
				if subID := agyExtractSubagentID(body); subID != "" {
					st.State = Running
					st.End = 0
					activeBG[subID] = call.ref
					subagentSteps[subID] = call.ref
				} else {
					st.End = max(completedAt, st.Start)
					st.State = Done
				}

			default:
				st.End = max(completedAt, st.Start)
				if code, ok := agyExitCode(body); ok && code != 0 {
					st.State = Failed
				} else {
					st.State = Done
				}
				if body != "" {
					st.Detail = joinLines(st.Detail, line(body, 160))
					st.Full = joinLines(st.Full, body)
				}
			}
		}
	}

	for i := 0; i < len(turns)-1; i++ {
		if turns[i].End == 0 {
			agyCloseFinishedTurn(&turns[i], turns[i+1].Start)
		}
	}
	if len(turns) > 0 {
		last := &turns[len(turns)-1]
		hasOpenChild := false
		hasWaitingChild := false
		for i := range last.Children {
			c := &last.Children[i]
			switch c.State {
			case Waiting:
				c.End = 0
				hasOpenChild = true
				hasWaitingChild = true
			case Running:
				if i+1 < len(last.Children) && (c.Kind == Think || c.Kind == Text) {
					c.End = max(last.Children[i+1].Start, c.Start)
					c.State = Done
				} else {
					c.End = 0
					hasOpenChild = true
				}
			}
		}

		if last.End == 0 {
			stillRunning := !lastEndedTurn || len(pendingCalls) > 0 || len(activeBG) > 0 || hasOpenChild
			if stillRunning {
				if hasWaitingChild {
					last.State = Waiting
				} else {
					last.State = Running
				}
				last.End = 0
			} else {
				agyCloseFinishedTurn(last, minTurnEnd(last, nowMs))
			}
		}
	}

	return order(turns)
}

func minTurnEnd(t *Step, nowMs int64) int64 {
	end := t.Start
	for _, c := range t.Children {
		if c.End > end {
			end = c.End
		} else if c.Start > end {
			end = c.Start
		}
	}
	if end == 0 {
		return nowMs
	}
	return end
}

func agyCloseFinishedTurn(t *Step, fallbackEnd int64) {
	fallback := max(t.Start, fallbackEnd)
	end := t.Start
	for i := range t.Children {
		c := &t.Children[i]
		if c.End == 0 {
			if i+1 < len(t.Children) && t.Children[i+1].Start >= c.Start {
				c.End = t.Children[i+1].Start
			} else {
				c.End = max(fallback, c.Start)
			}
			if c.State == Running || c.State == Waiting {
				c.State = Done
			}
		}
		if c.End > end {
			end = c.End
		}
	}
	if len(t.Children) == 0 {
		end = fallback
	}
	t.End = end
	if t.State != Failed {
		t.State = Done
	}
}

func agyBuildToolStep(tc agyToolCall, t int64) Step {
	switch tc.Name {
	case "invoke_subagent":
		role := agyArgStr(tc.Args["Role"])
		typeName := agyArgStr(tc.Args["TypeName"])
		prompt := agyArgStr(tc.Args["Prompt"])
		title := line(firstOf(role, firstOf(typeName, prompt)), 120)
		if title == "" {
			title = "subagent"
		}
		return Step{
			Kind:   Subagent,
			Title:  title,
			Detail: typeName,
			Full:   prompt,
			Start:  t,
			State:  Running,
		}
	case "swarm":
		if strings.EqualFold(agyArgStr(tc.Args["action"]), "add") {
			name := agyArgStr(tc.Args["name"])
			brief := agyArgStr(tc.Args["brief"])
			title := line(firstOf(name, brief), 120)
			if title == "" {
				title = "swarm"
			}
			return Step{
				Kind:   Subagent,
				Title:  title,
				Detail: "swarm",
				Full:   brief,
				Start:  t,
				State:  Running,
			}
		}
	}

	state := Running
	if tc.Name == "ask_question" {
		state = Waiting
	}
	return Step{
		Kind:   Tool,
		Title:  firstOf(tc.Name, "tool"),
		Detail: agyToolDetail(tc.Args),
		Full:   agyToolFull(tc.Args),
		Start:  t,
		State:  state,
	}
}

// agyUserPrompt extracts the user prompt text from an Antigravity USER_INPUT message.
func agyUserPrompt(content string) string {
	const openTag = "<USER_REQUEST>"
	const closeTag = "</USER_REQUEST>"
	if i := strings.Index(content, openTag); i >= 0 {
		after := content[i+len(openTag):]
		if j := strings.Index(after, closeTag); j >= 0 {
			return strings.TrimSpace(after[:j])
		}
		return strings.TrimSpace(after)
	}
	for _, tag := range []string{"<ADDITIONAL_METADATA>", "<USER_SETTINGS_CHANGE>"} {
		if i := strings.Index(content, tag); i >= 0 {
			content = content[:i]
		}
	}
	return strings.TrimSpace(content)
}

// agyArgStr unwraps Antigravity's double-encoded string args when present.
func agyArgStr(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		var inner string
		if len(s) >= 2 && s[0] == '"' && json.Unmarshal([]byte(s), &inner) == nil {
			return inner
		}
		return s
	}
	return short(raw, 200)
}

func agyToolDetail(args map[string]json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	for _, key := range []string{"toolSummary", "CommandLine", "Path", "query", "Description", "Role", "Pattern", "Url", "Prompt"} {
		if raw, ok := args[key]; ok {
			if v := strings.TrimSpace(agyArgStr(raw)); v != "" {
				return line(v, 200)
			}
		}
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		if v := strings.TrimSpace(agyArgStr(args[k])); v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	return line(strings.Join(parts, " "), 200)
}

func agyToolFull(args map[string]json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	unwrapped := make(map[string]any, len(args))
	for k, raw := range args {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			var v any
			if json.Unmarshal([]byte(s), &v) == nil {
				unwrapped[k] = v
			} else {
				unwrapped[k] = s
			}
			continue
		}
		var v any
		if json.Unmarshal(raw, &v) == nil {
			unwrapped[k] = v
		} else {
			unwrapped[k] = string(raw)
		}
	}
	b, err := json.Marshal(unwrapped)
	if err != nil {
		return ""
	}
	return string(b)
}

func agyParseToolResult(content string, fallback int64) (createdAt, completedAt int64, body string) {
	createdAt, completedAt = fallback, fallback
	hasCreated, hasCompleted := false, false

	rest := content
	for rest != "" {
		var firstLine, next string
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			firstLine, next = rest[:nl], rest[nl+1:]
		} else {
			firstLine, next = rest, ""
		}
		trimmed := strings.TrimSpace(firstLine)
		switch {
		case strings.HasPrefix(trimmed, "Created At: "):
			if !hasCreated {
				if ts, ok := parseTime(strings.TrimSpace(strings.TrimPrefix(trimmed, "Created At: "))); ok {
					createdAt, hasCreated = ts, true
				}
			}
			rest = next
		case strings.HasPrefix(trimmed, "Completed At: "):
			if !hasCompleted {
				if ts, ok := parseTime(strings.TrimSpace(strings.TrimPrefix(trimmed, "Completed At: "))); ok {
					completedAt, hasCompleted = ts, true
				}
			}
			rest = next
		default:
			return createdAt, completedAt, strings.TrimSpace(rest)
		}
	}
	return createdAt, completedAt, ""
}

func agyExtractTaskID(body string) string {
	const marker = "task id: "
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	s := body[i+len(marker):]
	if end := strings.IndexAny(s, " \t\r\n"); end >= 0 {
		s = s[:end]
	}
	return strings.TrimSpace(s)
}

func agyExtractSubagentID(body string) string {
	start := strings.IndexByte(body, '{')
	end := strings.LastIndexByte(body, '}')
	if start < 0 || end <= start {
		return ""
	}
	var payload struct {
		ConversationID string `json:"conversationId"`
	}
	if json.Unmarshal([]byte(body[start:end+1]), &payload) != nil {
		return ""
	}
	return strings.TrimSpace(payload.ConversationID)
}

func agyParseSystemMessage(content string) (sender, body string) {
	const msgPrefix = "[Message] "
	i := strings.Index(content, msgPrefix)
	if i < 0 {
		return "", ""
	}
	rest := content[i+len(msgPrefix):]
	rest = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "</SYSTEM_MESSAGE>"))

	const senderKey = "sender="
	si := strings.Index(rest, senderKey)
	if si < 0 {
		return "", ""
	}
	afterSender := rest[si+len(senderKey):]
	if sp := strings.IndexAny(afterSender, " \t\r\n"); sp >= 0 {
		sender = afterSender[:sp]
	} else {
		sender = afterSender
	}
	const contentKey = "content="
	if ci := strings.Index(afterSender, contentKey); ci >= 0 {
		body = strings.TrimSpace(afterSender[ci+len(contentKey):])
	}
	return strings.TrimSpace(sender), body
}

func agyExitCode(body string) (int, bool) {
	const prefix = "Command exited with code "
	if !strings.HasPrefix(body, prefix) {
		return 0, false
	}
	var code int
	if _, err := fmt.Sscanf(body[len(prefix):], "%d", &code); err != nil {
		return 0, false
	}
	return code, true
}

func agySystemMsgFailed(body string) bool {
	const resMarker = "finished with result:\n"
	if i := strings.Index(body, resMarker); i >= 0 {
		if code, ok := agyExitCode(strings.TrimSpace(body[i+len(resMarker):])); ok && code != 0 {
			return true
		}
	}
	firstLine := body
	if nl := strings.IndexByte(firstLine, '\n'); nl >= 0 {
		firstLine = firstLine[:nl]
	}
	return strings.Contains(firstLine, " was canceled with result:")
}
