package feishu

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"hyla/internal/proto"
	"hyla/internal/settings"
)

// background runs the prompt with the agent's own command, in the
// session's background, which tells the reply and no terminal shows: as
// claude -p, whose text is the answer. response is the event the answer
// replies to, which t has; a restart's take on the turn has none.
func (b *Bot) background(key, chatID, response, text string, t turnInfo) {
	bnd := b.bindingOf(bindingKey(chatID))
	agent := bnd.Agent
	if agent == "" {
		agent = b.cfg.Agent
	}
	if agent == "" || agent == "default" || !knownAgent(agent) {
		b.reply(chatID, "Which agent? /new <project> <agent> starts one, as /new api claude.")
		return
	}
	dir := bnd.Dir
	if dir == "" {
		dir = b.cfg.Dir
	}
	if dir == "" {
		dir = settings.UserHome()
	}
	if t.event == "" {
		t.event = response
	}
	session, made, err := b.backgroundSession(chatID, bnd.Session, agent, dir)
	if err != nil {
		b.reply(chatID, err.Error())
		return
	}
	// The conversation the chat went on with, as the turn before left it.
	resume := bnd.Conversation
	sid := resume
	if sid == "" {
		sid = newUUID()
	}
	// An OpenCode turn's words come from its server as it writes them,
	// which the command's stream reports a step at a time.
	var prog *turnProgress
	server, pass := "", ""
	if agent == "opencode" {
		if url, pw, err := b.openCode(); err == nil {
			server, pass = url, pw
			prog = &turnProgress{}
			go prog.watch(url, pw, opencodeSession(resume, sid))
			defer prog.stop()
		} else {
			b.logf("feishu: opencode's server: %v", err)
		}
	}
	cmd := b.turnCommand(agent, sid, resume, server, true)
	if cmd != nil {
		cmd = append(cmd, text)
	}
	asks := false
	if agent == "claude" {
		if b.yoloFor(agent) {
			// In yolo mode, Claude Code takes its prompt as -p's, whose
			// answer is its text alone. The stream's holds what the turn
			// used, for its card's footer.
			cmd = claudeStream(cmd)
		} else {
			// Claude Code asks the program that runs it for each tool
			// beyond its permissions, on a stream of its messages, which
			// its stream gives: its text replaces the one the command's
			// options ask.
			cmd = claudeStream(cmd)
			asks = true
		}
	}
	out := b.outFile(session)
	if agent == "claude" {
		out, _, _ = proto.AnswerPair(out)
	}
	// The app's run has its environment, while hylad's, which runs the
	// command, may have none of the agent's provider's.
	params := proto.BackgroundParams{ID: session, Command: cmd, Dir: dir, Out: out}
	for _, kv := range b.env {
		if strings.Contains(kv, "=") {
			params.Env = append(params.Env, kv)
		}
	}
	if pass != "" {
		// The command's server takes its password, which the bot made.
		params.Env = envWith(params.Env, "OPENCODE_PASSWORD", pass)
	}
	if err := b.api.Call("agent.background", params, nil); err != nil {
		b.reply(chatID, err.Error())
		return
	}
	// The chat window shows the conversation this turn runs from now on.
	b.setBinding(bindingKey(chatID), binding{Agent: agent, Dir: dir, Conversation: sid})
	if made && !t.window {
		// The user's session shows in the app at once, where its chat
		// window is: not one a prompt of the app's chat window made, which
		// shows the chat it asked.
		b.api.Call("select", proto.FocusParams{Session: session}, nil)
	}
	// The turn shows by its own card, where the chat has Feishu's cards:
	// every agent of a background turn prints what it does, which the
	// card takes, and its answer, which the same card ends with.
	var card *turnCard
	if cards() {
		card = b.newTurnCard(key, chatID, langOf(text))
		b.card(card, "working", "", "")
	}
	// The card takes the turn's new text as often as feishu-stream says,
	// and the time the turn has run until it has words, so that a card
	// of a long step is not still.
	shown := time.Now()
	started := shown
	every := cardEvery
	if s := time.Duration(b.cfg.Stream); s >= time.Second {
		every = s
	}
	stop := time.Now().Add(b.timeout())
	ended := make(chan struct{})
	defer close(ended)
	if asks {
		go b.answerAsks(key, chatID, session, stop, ended)
	}
	from := int64(0)
	answer := ""
	for {
		var out proto.Output
		if err := b.api.Call("agent.output", proto.OutputParams{ID: session, From: from}, &out); err != nil {
			if !b.card(card, "error", err.Error(), "") {
				b.reply(chatID, err.Error())
			}
			return
		}
		answer += out.Text
		from = out.From
		if out.Ended {
			b.finished(key, chatID, t.event, agent, dir, sid, text, out, answer, card, t.start)
			return
		}
		if card != nil && time.Since(shown) >= every {
			tail := ""
			if prog != nil {
				if name, n, words := prog.snapshot(); n > 0 || words != "" {
					tail = progressBody(name, n, words, langOf(text))
				}
			}
			if tail == "" {
				tail = backgroundTail(agent, answer, langOf(text))
			}
			if tail == "" {
				tail = thinking(card, time.Since(started))
			}
			if b.card(card, "working", tail, "") {
				shown = time.Now()
			}
		}
		if time.Now().After(stop) {
			b.setBinding(bindingKey(chatID), binding{Agent: agent, Dir: dir, Conversation: sid})
			if msg := fmt.Sprintf("Still working after %s. /stop ends it.", b.timeout()); !b.card(card, "working", msg, "") {
				b.reply(chatID, msg)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// finished ends a background turn: the chat's conversation continues,
// the card turns green or red, and the answer goes to the chat in a
// reply to the event.
func (b *Bot) finished(key, chatID, response, agent, dir, sid, text string, out proto.Output, answer string, card *turnCard, started time.Time) {
	out.Text = answer
	if sid = conversation(agent, answer, sid); sid == "" {
		sid = b.bindingOf(bindingKey(chatID)).Conversation
	}
	// A session the agent does not know, as one this turn's ID named
	// before the agent kept it, starts anew next turn: the chat goes on
	// with no history rather than failing every turn.
	if agent == "codex" && lostSession(answer) {
		sid = ""
	}
	b.setBinding(bindingKey(chatID), binding{Agent: agent, Dir: dir, Conversation: sid})
	reply := b.backgroundReply(agent, out)
	// A question of the agent, which the chat may choose to answer, is
	// the card's: its options are buttons under the question.
	state := "done"
	if q, ok := question(reply); ok {
		reply, state = q, "asks"
	}
	// The conversation a chat window shows keeps the turn's prompt and
	// answer, which the agent's command left no transcript for.
	b.keepTurn(agent, sid, text, reply)
	b.replyTurn(key, response, chatID, card, Card{Title: titleOf(reply), Text: reply, State: state,
		Duration: time.Since(started), Usage: UsageOf(agent, out.Text)}, reply)
}

// lostSession tells whether a Codex turn could not continue the session
// its ID named: it failed before it ran, and no event named another.
func lostSession(raw string) bool {
	for _, t := range []string{"invalid session id", "failed to resolve rollout path"} {
		if strings.Contains(raw, t) {
			return true
		}
	}
	return false
}

// backgroundSession makes the chat's session a background one, with the
// agent's name, or returns it if it is one already, and whether it makes
// one.
func (b *Bot) backgroundSession(chatID, id, agent, dir string) (string, bool, error) {
	if id != "" {
		var has bool
		if err := b.api.Call("session.hasBackground", proto.SessionParams{ID: id}, &has); err == nil && has {
			return id, false, nil
		}
	}
	project := ""
	if st, err := b.api.State(); err == nil {
		for _, s := range st.Sessions {
			if s.ID == id {
				project = s.Project
			}
		}
	}
	name := agent + " · " + filepath.Base(dir)
	if base := filepath.Base(dir); base == "" || base == "/" || base == "~" {
		name = agent
	}
	if project == "" {
		if st, err := b.api.State(); err == nil && len(st.Projects) > 0 {
			project = st.Projects[0].ID
		}
	}
	var s proto.Session
	if err := b.api.Call("create", proto.CreateParams{Project: project, Dir: dir, Name: name}, &s); err != nil {
		return "", false, fmt.Errorf("Hyla could not start a session: %w", err)
	}
	if err := b.api.Call("session.setBackground", proto.SessionParams{ID: s.ID}, nil); err != nil {
		return "", false, err
	}
	b.bind(chatID, s)
	return s.ID, true, nil
}

// opencodeSession is OpenCode's ID of the session a turn continues, or
// starts: one that carries its prefix is its own, and another, as a
// chat's, names one it makes.
func opencodeSession(resume, sid string) string {
	id := resume
	if id == "" {
		id = sid
	}
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "ses") {
		return id
	}
	return "ses_" + strings.ReplaceAll(id, "-", "")
}

// backgroundCommand is the agent's command with the prompt, and the
// options that tell the answer and continue the conversation: the first
// turn starts it with sid, and the others continue it with resume, which
// is the conversation's ID the first turn gave. told says whether the
// turn takes the yolo option as its settings have it, not as its chat
// told it for a session without it.
func (b *Bot) backgroundCommand(agent, sid, resume, text string, told bool) []string {
	cmd := b.turnCommand(agent, sid, resume, "", told)
	if cmd == nil {
		return nil
	}
	return append(cmd, text)
}

// turnCommand is a background turn's command without its prompt.
func (b *Bot) turnCommand(agent, sid, resume, server string, told bool) []string {
	again := resume != ""
	wrote := len(b.cfg.AgentCommand(agent)) > 0
	// The turn's words: those of feishu-agent-command, with the option
	// after the subcommand where the turn is on, or else as written; or
	// the program alone, without the option, since the turn's goes after
	// it.
	cmd := append([]string{}, agentCommand(b.cfg, agent)...)
	// on is whether the turn takes the option: as it told, or as the
	// settings have it where feishu-agent-command names it. DSH has none.
	on := told || settings.IsYolo(&b.cfg, b.yolo, agent)
	flag := slices.Clone(settings.YoloFlag(agent))
	if agent == "dsh" {
		// DSH takes its option as a setting the task and the stream go
		// after, not before its words, which its prompt would take for
		// its own. A subcommand it was written with goes, and is added
		// again below.
		if _, named := settings.AgentYolo(&b.cfg, agent); named {
			cmd = cmd[:1]
		}
	} else {
		cmd = append(cmd[:1], agentHead(b.cfg, agent, flag)...)
	}
	switch agent {
	case "claude":
		// The option goes before its words, which its prompt would take
		// for the prompt's own. Only the settings give it: a turn never.
		if settings.IsYolo(&b.cfg, b.yolo, agent) {
			cmd = append(cmd, flag...)
		}
		if again {
			cmd = append(cmd, "-p", "--output-format", "text", "--session-id", resume)
		} else {
			cmd = append(cmd, "-p", "--output-format", "text", "--session-id", sid)
		}
	case "codex":
		// Its events, as JSON lines, name the session it runs, which a
		// chat continues, and its messages, which are its answer: its
		// text alone names neither.
		cmd = append(cmd, "exec", "--json")
		if on {
			cmd = append(cmd, flag...)
		}
		if again {
			cmd = append(cmd, "resume", resume, "--skip-git-repo-check")
		}
	case "opencode":
		// Its events, as JSON lines, name the session, its answer, and
		// what the turn used. The server is the bot's, which reports the
		// words as the agent writes them.
		cmd = append(cmd, "run", "--format", "json")
		if server != "" {
			cmd = append(cmd, "--server", server)
		}
		// OpenCode names a session of its own ses_…, and makes the one
		// an ID it does not know names: a chat's ID, or a pane's
		// conversation's, is one of its own, which the turn keeps.
		if id := opencodeSession(resume, sid); id != "" {
			cmd = append(cmd, "--session", id)
		}
	case "gemini":
		// One JSON object, whose response is its answer and whose stats
		// name what the turn used.
		cmd = append(cmd, "-p", "--output-format", "json")
		if again {
			cmd = append(cmd, "--resume", resume)
		} else {
			cmd = append(cmd, "--session-id", sid)
		}
	case "pi":
		// Its events, as JSON, name its answer and what it used.
		if again {
			cmd = append(cmd, "-p", "--mode", "json", "--session-id", resume)
		} else {
			cmd = append(cmd, "-p", "--mode", "json", "--session-id", sid)
		}
	case "dsh":
		// Its events, as JSON lines, name the session it runs, which a
		// chat's next turns go on with, and its final event is its answer.
		cmd = append(cmd, "headless", "--json")
		// DSH has no option that runs it without its approvals, and its
		// sandbox is its own setting, DSH_PERMISSION_MODE: a yolo option
		// from its command goes, and the turn asks no approval.
		cmd = slices.DeleteFunc(cmd, func(w string) bool { return slices.Contains(flag, w) })
		// The first turn starts a session and its events name it; only
		// the turns after it go on with one.
		if resume != "" {
			cmd = append(cmd, "--session-id", resume)
		}
	default:
		if !again && !wrote && len(cmd) == 1 {
			return nil // no way told to run one prompt and end
		}
	}
	return cmd
}

// agentArgs is the command the agent's background turns start by, with
// the yolo option as the settings set it, or as the turn told it.
func (b *Bot) agentArgs(agent string, told bool) []string {
	cmd := append([]string{}, agentCommand(b.cfg, agent)...)
	on, _ := settings.AgentYolo(&b.cfg, agent)
	if told || on {
		return cmd
	}
	flag := settings.YoloFlag(agent)
	return append([]string{cmd[0]}, slices.DeleteFunc(cmd[1:], func(w string) bool { return slices.Contains(flag, w) })...)
}

// agentSubcommand is the word of the agent's command that runs its
// prompt and ends, for an agent that runs no prompt without it.
var agentSubcommand = map[string]string{
	"codex":    "exec",
	"opencode": "run",
	"dsh":      "headless",
}

// agentHead is the agent's command after its program, and before its
// words: the option where feishu-agent-command names it, which the
// program takes before its subcommand, or else nothing, since a turn's
// option goes after the subcommand. Claude Code's turns never name it
// here: its subcommand's options follow, and a prompt after them would
// take them for its own.
func agentHead(f settings.Feishu, agent string, flag []string) []string {
	if agent == "claude" {
		return nil
	}
	if _, named := settings.AgentYolo(&f, agent); named {
		return flag
	}
	return nil
}

// yoloFor is whether the agent's turns take its yolo option: as
// feishu-yolo sets it, or else as feishu-agent-command wrote it.
func (b *Bot) yoloFor(agent string) bool {
	if settings.IsYolo(&b.cfg, b.yolo, agent) {
		return true
	}
	_, named := settings.AgentYolo(&b.cfg, agent)
	return named
}

// sessionCommand is the command a session from Feishu starts the agent
// by, and whether its yolo option is the program's alone, which its
// subcommands do not take: its command as feishu-agent-command wrote it
// where that names the option, or else with the option before its
// arguments.
func (b *Bot) sessionCommand(agent string) (cmd []string, programOnly bool) {
	cmd = b.cfg.AgentArgs(b.yoloFor(agent), agent)
	// The option the program takes goes before its arguments, and those
	// of a background turn after the subcommand.
	if !b.yoloFor(agent) {
		return cmd, false
	}
	switch agent {
	case "codex":
		return cmd, true
	case "opencode", "pi", "qwen":
		// Their programs take the option, their subcommands do not.
		return cmd, false
	}
	return cmd, true
}

// claudeStream is a Claude Code command's words as its messages a stream
// hold what the turn used: no -p, which makes its answer its text alone,
// and the flags after the prompt, which the prompt would take for its
// own.
func claudeStream(cmd []string) []string {
	i := len(cmd) - 1 // the prompt
	out := append([]string{}, cmd[:i]...)
	for j := 0; j < len(out); j++ {
		if out[j] == "-p" || out[j] == "--print" {
			out = append(out[:j], out[j+1:]...)
			j--
		} else if out[j] == "--output-format" {
			out = append(out[:j], out[j+2:]...)
			j--
		}
	}
	out = append(out, claudeStreamFlags...)
	return append(out, cmd[i])
}

// conversation is the agent's session that a turn's output names, for
// the next turn to continue: the one the first turn started, or another
// its output says, as one the agent chose.
func conversation(agent, raw string, sid string) string {
	found := ""
	for _, line := range strings.Split(raw, "\n") {
		// OpenCode names its session sessionID, as Claude Code does
		// session_id, and Codex thread_id.
		var m struct {
			Type      string `json:"type"`
			Snake     string `json:"session_id"`
			Camel     string `json:"sessionID"`
			Thread    string `json:"thread_id"`
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal([]byte(line), &m) != nil {
			continue
		}
		switch {
		case m.Type == "session" && m.SessionID != "" && agent == "dsh":
			found = m.SessionID
		case m.Camel != "" && agent == "opencode":
			found = m.Camel
		case m.Thread != "":
			found = m.Thread
		case m.Snake != "" && (agent == "claude" || agent == "gemini"):
			found = m.Snake
		}
	}
	if found == "" && raw != "" && (agent == "claude" || agent == "pi") {
		return sid // the CLI keeps the one it was given
	}
	return found
}

// backgroundTail is a background turn's text as a working card shows it:
// the last tool the agent ran, and the last rows of what it said, and
// none of the stream it says them in.
func backgroundTail(agent, answer, lang string) string {
	text := commandText(agent, answer)
	if text == "" || text == strings.TrimSpace(answer) {
		// What the agent printed is its stream's own text, which it has
		// not made into words yet: the card waits for the words.
		if streamed(answer) {
			text = ""
		}
	}
	text = clean(since(text, 0, toolTail))
	line := toolLine(agent, answer, lang)
	switch {
	case line != "" && text != "":
		return line + "\n\n" + text
	case line != "":
		return line
	}
	return text
}

// streamed tells a turn's output that is a stream of JSON events, which
// the agent's parsing makes into words, and which no card shows as it is.
func streamed(raw string) bool {
	line, _, _ := strings.Cut(strings.TrimSpace(raw), "\n")
	return strings.HasPrefix(strings.TrimSpace(line), "{")
}

// toolLine is a background turn's tools as a working card shows them: the
// last one the agent ran, and how many it ran, as "⏳ shell · 5 tools".
func toolLine(agent, raw, lang string) string {
	name, n := lastTool(agent, raw)
	return toolSummary(name, n, lang)
}

// toolSummary is a turn's tools as a working card shows them.
func toolSummary(name string, n int, lang string) string {
	if n == 0 {
		return ""
	}
	if name == "" {
		name = word(lang, "tool.one")
	}
	key := "tools"
	if n == 1 {
		key = "tool"
	}
	return "⏳ **" + escapeMarkdown(name) + "** · " + fmt.Sprintf(word(lang, key), n)
}

// progressBody is a turn's progress as a working card shows it: the tool
// it runs, how many it ran, and the last rows of the words it wrote.
func progressBody(name string, n int, words, lang string) string {
	parts := make([]string, 0, 2)
	if line := toolSummary(name, n, lang); line != "" {
		parts = append(parts, line)
	}
	if said := clean(since(words, 0, toolTail)); said != "" {
		parts = append(parts, said)
	}
	return strings.Join(parts, "\n\n")
}

// lastTool is the last tool a background turn ran, and how many it ran:
// the tool_use of an OpenCode turn, the tool_use of a Claude Code one,
// and the items a Codex turn completes.
func lastTool(agent, raw string) (string, int) {
	name, n := "", 0
	for _, line := range strings.Split(raw, "\n") {
		switch agent {
		case "opencode":
			var e struct {
				Type string `json:"type"`
				Part *struct {
					Tool string `json:"tool"`
				} `json:"part"`
			}
			if json.Unmarshal([]byte(line), &e) == nil && e.Type == "tool_use" && e.Part != nil && e.Part.Tool != "" {
				name, n = e.Part.Tool, n+1
			}
		case "claude":
			var m struct {
				Type    string `json:"type"`
				Message *struct {
					Content []struct {
						Type string `json:"type"`
						Name string `json:"name"`
					} `json:"content"`
				} `json:"message"`
			}
			if json.Unmarshal([]byte(line), &m) != nil || m.Type != "assistant" || m.Message == nil {
				continue
			}
			for _, c := range m.Message.Content {
				if c.Type == "tool_use" && c.Name != "" {
					name, n = c.Name, n+1
				}
			}
		case "codex":
			var e struct {
				Type string `json:"type"`
				Item *struct {
					Type     string `json:"type"`
					ItemType string `json:"item_type"`
				} `json:"item"`
			}
			if json.Unmarshal([]byte(line), &e) != nil || e.Type != "item.completed" || e.Item == nil {
				continue
			}
			if t := firstNotEmpty(e.Item.Type, e.Item.ItemType); t != "" && t != "agent_message" && t != "reasoning" {
				name, n = t, n+1
			}
		}
	}
	return name, n
}

// backgroundReply is the answer a background turn gives: its text, or
// what ended it.
func (b *Bot) backgroundReply(agent string, out proto.Output) string {
	if out.Error != "" && out.Text == "" {
		return "The agent failed: " + out.Error
	}
	text := commandText(agent, out.Text)
	if strings.TrimSpace(text) == "" {
		if out.Exited != 0 {
			return fmt.Sprintf("The agent ended with code %d, and no output.", out.Exited)
		}
		return "The agent answered nothing."
	}
	if out.Exited != 0 {
		text += fmt.Sprintf("\n\nThe agent ended with code %d.", out.Exited)
	}
	return text
}

// commandText takes the answer from the agent's output: the text of its
// strings, or its last message, or the stream's, and the output itself
// when it is none of these.
func commandText(agent, raw string) string {
	switch agent {
	case "claude":
		var text strings.Builder
		for _, line := range strings.Split(raw, "\n") {
			var m struct {
				Type    string `json:"type"`
				Message *struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
				Result string `json:"result"`
				// Stream is true of the lines a turn asks the chat to
				// answer, which a permission's card shows.
				Stream bool `json:"stream"`
			}
			if json.Unmarshal([]byte(line), &m) != nil {
				t := strings.TrimSpace(line)
				// Claude Code's own reports to the terminal, such as a
				// model the gateway does not know, and its warning that
				// it got no input, are no part of the answer.
				if strings.HasPrefix(t, "[claude-code:") || strings.HasPrefix(t, "Warning: no stdin data received") || m.Stream {
					continue
				}
				if t != "" {
					if text.Len() > 0 {
						text.WriteString("\n")
					}
					text.WriteString(t)
				}
				continue
			}
			switch m.Type {
			case "assistant":
				if m.Message != nil {
					for _, c := range m.Message.Content {
						if c.Type == "text" && c.Text != "" {
							if text.Len() > 0 {
								text.WriteString("\n\n")
							}
							text.WriteString(c.Text)
						}
					}
				}
			case "result":
				if m.Result != "" && strings.TrimSpace(text.String()) == "" {
					return m.Result
				}
			}
		}
		return strings.TrimSpace(text.String())
	case "opencode":
		var text strings.Builder
		for _, line := range strings.Split(raw, "\n") {
			var e struct {
				Type string `json:"type"`
				Part *struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"part"`
			}
			if json.Unmarshal([]byte(line), &e) == nil && e.Type == "text" && e.Part != nil && e.Part.Text != "" {
				if text.Len() > 0 {
					text.WriteString("\n\n")
				}
				text.WriteString(e.Part.Text)
			}
		}
		if text.Len() > 0 {
			return text.String()
		}
	case "codex":
		// The events of --json: an item that completed and is the
		// agent's message is its answer, and a turn that failed says
		// why, as its error item or its error's message.
		var text strings.Builder
		failure := ""
		for _, line := range strings.Split(raw, "\n") {
			var e struct {
				Type    string `json:"type"`
				Message string `json:"message"`
				Item    *struct {
					Type     string `json:"type"`
					ItemType string `json:"item_type"`
					Text     string `json:"text"`
				} `json:"item"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(line), &e) != nil {
				continue
			}
			switch e.Type {
			case "item.completed":
				if e.Item == nil || e.Item.Text == "" {
					continue
				}
				if t := firstNotEmpty(e.Item.Type, e.Item.ItemType); t != "" && t != "agent_message" {
					continue
				}
				if text.Len() > 0 {
					text.WriteString("\n\n")
				}
				text.WriteString(e.Item.Text)
			case "turn.failed", "error":
				if e.Error != nil && e.Error.Message != "" {
					failure = e.Error.Message
				} else if e.Message != "" {
					failure = e.Message
				}
			}
		}
		if t := strings.TrimSpace(text.String()); t != "" {
			return t
		}
		if failure != "" {
			return failure
		}
	case "pi":
		// The events of --mode json: an assistant message's text is its
		// answer.
		var text strings.Builder
		for _, line := range strings.Split(raw, "\n") {
			var e struct {
				Type    string `json:"type"`
				Message *struct {
					Role    string `json:"role"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			}
			if json.Unmarshal([]byte(line), &e) != nil || e.Type != "message_end" || e.Message == nil || e.Message.Role != "assistant" {
				continue
			}
			for _, c := range e.Message.Content {
				if c.Type == "text" && c.Text != "" {
					if text.Len() > 0 {
						text.WriteString("\n\n")
					}
					text.WriteString(c.Text)
				}
			}
		}
		if t := strings.TrimSpace(text.String()); t != "" {
			return t
		}
	case "dsh":
		// The events of --json: the final event's text is its answer,
		// which a turn that failed may leave empty; an error event, or
		// the turn's end, says why it failed.
		final := ""
		failure := ""
		for _, line := range strings.Split(raw, "\n") {
			var e struct {
				Type    string `json:"type"`
				Phase   string `json:"phase"`
				Text    string `json:"text"`
				Message string `json:"message"`
				Reason  *struct {
					Kind    string `json:"kind"`
					Message string `json:"message"`
				} `json:"reason"`
			}
			if json.Unmarshal([]byte(line), &e) != nil {
				continue
			}
			switch {
			case e.Type == "final":
				final = e.Text
			case e.Type == "error":
				failure = e.Message
			case e.Type == "status" && e.Phase == "turn_end" && e.Reason != nil && e.Reason.Kind != "" && e.Reason.Kind != "completed":
				failure = "The turn ended: " + e.Reason.Kind
				if e.Reason.Message != "" {
					failure += " (" + e.Reason.Message + ")"
				}
			}
		}
		if t := strings.TrimSpace(final); t != "" {
			return t
		}
		if failure != "" {
			return failure
		}
	case "gemini":
		// One JSON object, whose response is its answer.
		var r struct {
			Response string `json:"response"`
			Error    *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(raw)), &r) == nil {
			if r.Response != "" {
				return r.Response
			}
			if r.Error != nil && r.Error.Message != "" {
				return r.Error.Message
			}
		}
	}
	return strings.TrimSpace(raw)
}

// hasBackground tells a session's that runs its agent's commands without
// a terminal.
func (b *Bot) hasBackground(id string) (bool, error) {
	var has bool
	err := b.api.Call("session.hasBackground", proto.SessionParams{ID: id}, &has)
	return has, err
}

func (b *Bot) outFile(session string) string {
	return filepath.Join(proto.Dir(), "feishu-run", session+".out")
}

// newUUID is a version 4 UUID, as the agents take a session's ID.
func newUUID() string {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		return fmt.Sprint(time.Now().UnixNano())
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}
