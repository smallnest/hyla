package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"hyla/internal/detect"
	"hyla/internal/proto"
	"hyla/internal/settings"
)

const (
	// replyTail is the rows of the pane's screen a reply ends with: the
	// end of the agent's answer, under its prompt.
	replyTail = 30
	// maxText is a text message's: Feishu takes 150 KB, and 30 for a
	// card.
	maxText = 120 << 10
	// seenCap is how many event IDs the bot remembers.
	seenCap = 512
	// queue is how many prompts a chat has waiting.
	queue = 1
)

// onMessage takes a message from Feishu, and answers it as it asks. A
// group's message it is not mentioned in is no message to it: the group
// may have several bots.
func (b *Bot) onMessage(clientCtx context.Context, e *larkim.P2MessageReceiveV1) error {
	// Its turns keep hylad at its work, as a connection that stays does.
	if _, err := b.api.State(); err == nil {
		b.api.Call("watch", map[string]bool{"stay": true}, nil)
	}
	if e != nil && e.EventV2Base != nil && e.EventV2Base.Header != nil {
	}
	if e == nil || e.Event == nil || e.Event.Message == nil {
		return nil
	}
	m := e.Event.Message
	if m.ChatId == nil {
		return nil
	}
	if e.EventV2Base != nil && e.EventV2Base.Header != nil {
		if h := e.EventV2Base.Header.AppID; h != "" && h != b.cfg.App {
			return nil // another app's, as a shared event subscription sends
		}
	}
	chatID := *m.ChatId
	// The message as it came, before the checks that may leave it
	// unanswered: a chat that gets no reply shows where it stopped.
	b.logf("feishu: message %s in %s, %s, %d bytes", deref(m.MessageId), chatID, deref(m.MessageType), len(deref(m.Content)))
	if e.EventV2Base != nil && e.EventV2Base.Header != nil {
		if id := e.EventV2Base.Header.EventID; id != "" && b.seenEvent(id) {
			return nil
		}
	}
	if !b.answers(chatID) {
		return nil
	}
	if leave, ok := b.windowEvent(m); leave && ok {
		return nil
	}
	group := deref(m.ChatType) == "group"
	if group && !b.mentioned(m) {
		// A reply to the bot in a group: its answer names the event, and
		// this message comes to it alone.
		if !b.replyToBot(m) {
			return nil
		}
	}
	openID, userID := "", ""
	if e.Event.Sender != nil && e.Event.Sender.SenderId != nil {
		openID = deref(e.Event.Sender.SenderId.OpenId)
		userID = deref(e.Event.Sender.SenderId.UserId)
	}
	if !b.allowed(openID, userID) {
		b.reply(chatID, "You are not allowed to use Hyla on this machine. Add your open_id ("+
			firstNotEmpty(openID, userID)+") to feishu-allow in ~/.config/hyla/config.")
		return nil
	}
	// A group's messages go to the session of the chat, and each reply
	// to the message it answers: the thread's root is the chat's topic.
	// A group's mentions read alike, a message's and a thread's. In
	// shared mode, every host with the bot leaves a message that follows
	// another host's answer to that host.
	if p := parentOf(m); p != "" {
		ev, leave := b.claim(p, chatID)
		if leave {
			return nil
		}
		if ev != "" {
			return nil
		}
	}
	key := chatKey(chatID, "")
	if group {
		key = chatKey(chatID, deref(m.RootId))
	}
	text := b.textOf(parentOf(m), m)
	if text == "" {
		return nil
	}
	// One message a chat at a time: a prompt while one runs waits its turn,
	// one deep, and a command runs at once, which /stop must, as the turn
	// it ends keeps the chat.
	if cmd, _ := splitCommand(text); cmd != "" && !b.enter(key) {
		b.goTurn(func() { b.turn(key, chatID, deref(m.MessageId), text) })
		return nil
	} else if cmd == "" && !b.enter(key) {
		if !b.hold(key, chatID, group, openID, m, text) {
			b.reply(chatID, "Hyla is still working on the last message there, and one waits. Wait for it, or send /stop.")
		}
		return nil
	}
	b.takeTurn(key, chatID, group, openID, m, text)
	return nil
}

// takeTurn runs a message as the chat's turn, which the caller took.
func (b *Bot) takeTurn(key, chatID string, group bool, openID string, m *larkim.EventMessage, text string) {
	response := deref(m.MessageId)
	asked := parentOf(m) // the answer a follow-up answers
	if b.shared() && len(asked) > 0 {
		response = asked
	}
	t := &turnInfo{start: time.Now(), chat: chatID, text: text, event: response,
		asked: asked}
	_, t.window = windowPrompt(deref(m.Content))
	b.mu.Lock()
	c0 := b.chatLocked(key)
	c0.chat, c0.event, c0.key = chatID, response, key
	if !group {
		c0.peer = openID
	}
	b.mu.Unlock()
	b.beginTurn(key, t)
	// The wait is over: its card goes as the turn's begins.
	b.dropHold(key)
	c := b.chat(key)
	b.goTurn(func() {
		defer b.leave(key)
		defer b.dropTurn(key, t)
		if takeOverOn && b.takeOver(c) {
			return
		}
		// The message marks the turn before the agent runs it, and once
		// it answers.
		m := b.markOn(response)
		defer b.off(m)
		b.turn(key, chatID, response, text)
	})
}

// takeOverOn tells whether a turn takes on a turn before it, which it
// finds running: a test that runs several turns of a chat at once, as
// its hosts hear them, turns it off.
var takeOverOn = true

// hostNote tells a chat that Feishu does not let the bot write there,
// as a group it was not added to.
const hostNote = "Feishu does not let Hyla write here. Add the bot to the group, and let feishu-host name the hosts that share the app; the chat's host goes among them."

// isCommand tells a message's text that is a command.
func isCommand(content string) bool {
	cmd, _ := splitCommand(messageText(content))
	return cmd != ""
}

// parentOf is a message's parent, the message it answers.
func parentOf(m *larkim.EventMessage) string { return deref(m.ParentId) }

// replyToBot tells a group message sent in a reply to one the bot sent,
// as the bot's card, which this event alone says of: the next event of
// a reply names no event.
func (b *Bot) replyToBot(m *larkim.EventMessage) bool {
	parent := deref(m.ParentId)
	if parent == "" {
		return false
	}
	b.mu.Lock()
	_, ok := b.notes[parent]
	b.mu.Unlock()
	return ok
}

// textOf is a message's text to the session: a text's, a file's or
// image's saved path, a rich text's; "" when it has none.
func (b *Bot) textOf(chatID string, m *larkim.EventMessage) string {
	content := deref(m.Content)
	switch deref(m.MessageType) {
	case "text":
		return withoutMentions(m)
	case "file", "image":
		return strings.TrimSpace(withoutMentionsText(m) + b.attachment(chatID, deref(m.MessageType), content))
	case "post":
		return strings.TrimSpace(postText(content) + b.attachment(chatID, "post", content))
	}
	b.reply(chatID, "Hyla takes text, files, images and rich text. Send one of them.")
	return ""
}

// turn runs a message: a command, or the prompt that says it. event is
// the event the message answers, which its answer names.
func (b *Bot) turn(key, chatID, event, text string) {
	cmd, rest := splitCommand(text)
	t := turnInfo{start: time.Now(), chat: chatID, text: text, event: event}
	if cmd == "" {
		if b.cfg.Run == settings.FeishuRunCommand {
			b.background(key, chatID, event, rest, t)
		} else {
			b.prompt(key, chatID, event, rest, t)
		}
		return
	}
	switch cmd {
	case "new":
		b.newSession(chatID, rest)
	case "use":
		b.use(chatID, strings.TrimSpace(rest))
	case "ls":
		b.list(chatID)
	case "status":
		b.status(chatID)
	case "stop":
		b.stop(key, chatID)
	case "answer":
		b.answer(key, chatID, strings.TrimSpace(rest))
	case "close":
		b.close(chatID)
	case "host":
		b.hostCommand(chatID, rest)
	case "help":
		b.reply(chatID, helpText)
	default:
		b.reply(chatID, fmt.Sprintf("No command /%s. Write /help for the commands.", cmd))
	}
}

// mentioned tells whether the message @-mentions the bot.
func (b *Bot) mentioned(m *larkim.EventMessage) bool {
	for _, mt := range m.Mentions {
		if mt == nil {
			continue
		}
		if deref(mt.MentionedType) == "bot" {
			return true
		}
	}
	return false
}

// withoutMentions is a text message's text without its @-mentions, as
// the bot is told them apart from what the user writes.
func withoutMentions(m *larkim.EventMessage) string { return withoutMentionsText(m) }

// withoutMentionsText is a message's text, as a text message has it,
// without its @-mentions.
func withoutMentionsText(m *larkim.EventMessage) string {
	text := messageText(deref(m.Content))
	for _, mt := range m.Mentions {
		if mt != nil && deref(mt.Key) != "" {
			text = strings.ReplaceAll(text, deref(mt.Key), "")
		}
	}
	return strings.TrimSpace(text)
}

// enter takes a chat's turn if no other runs.
func (b *Bot) enter(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.chatLocked(key)
	if c.busy {
		return false
	}
	c.busy = true
	return true
}

func (b *Bot) leave(key string) {
	b.mu.Lock()
	c := b.chatLocked(key)
	c.busy = false
	var next *waitingTurn
	if len(c.waiting) > 0 {
		next = &c.waiting[0]
		c.waiting = c.waiting[1:]
		// The waiting prompt takes the chat's turn, before another
		// message could take it.
		c.busy = true
	}
	b.mu.Unlock()
	if next != nil {
		b.takeTurn(key, next.chatID, next.group, next.openID, next.m, next.text)
	}
}

// hold puts a prompt in a chat's queue, one deep, for its turn's end to
// run, and tells the chat so in a card, which the turn that runs it, or
// a stop, takes back. It reports whether it found room.
func (b *Bot) hold(key, chatID string, group bool, openID string, m *larkim.EventMessage, text string) bool {
	b.mu.Lock()
	c := b.chatLocked(key)
	if len(c.waiting) >= queue {
		b.mu.Unlock()
		return false
	}
	c.waiting = append(c.waiting, waitingTurn{key: key, chatID: chatID, openID: openID, group: group, m: m, text: text})
	b.mu.Unlock()
	// Every other answer shows as a card, the queued prompt's too: blue,
	// titled for the queue, over the prompt. A chat that takes no cards
	// gets the words.
	if shown, err := heldCardJSON(text, langOf(text)); err == nil {
		// A prompt that waits in a topic waits there, as it came.
		if msg, err := b.sendCard(chatID, threadOfKey(key), shown); err == nil {
			b.holdCard(key, msg)
			return true
		}
	}
	b.reply(chatID, "Queued: Hyla sends it when the turn there ends.")
	return true
}

// holdCard keeps a waiting prompt's card by its message, for the turn
// that runs the prompt to take back.
func (b *Bot) holdCard(key, msg string) {
	if msg == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chatLocked(key).held = append(b.chatLocked(key).held, msg)
}

// dropHold takes back the waiting prompts' cards of a chat, which the
// turn it starts with leaves behind, or a stop ends.
func (b *Bot) dropHold(key string) {
	b.mu.Lock()
	c := b.chatLocked(key)
	cards := c.held
	c.held = nil
	b.mu.Unlock()
	for _, msg := range cards {
		if err := b.msgs.DeleteMessage(msg); err != nil {
			b.logf("feishu: taking back a queued prompt's card: %v", err)
		}
	}
}

// goTurn runs a turn, unless the tests take the turns in their own hands
// (sync).
func (b *Bot) goTurn(f func()) {
	if b.sync {
		f()
		return
	}
	go f()
}

// splitCommand cuts a slash command from a message: /new, /use, /help, or
// the ones /foo@bot-name in a group writes.
func splitCommand(text string) (cmd, rest string) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "/") || strings.HasPrefix(t, "//") {
		return "", t
	}
	word, rest, _ := strings.Cut(t[1:], " ")
	if i := strings.IndexAny(word, "@"); i >= 0 {
		word = word[:i]
	}
	return strings.ToLower(word), rest
}

const helpText = `Hyla

/new [project] [agent]   start a session, and bind this chat to it
/use <session>           bind this chat to an open session
/ls                      list the open sessions
/status                  the bound session's state, branch and changes
/stop                    interrupt the agent's current turn
/answer <text>           answer the question the agent asks
/close                   end the bound session

Other text goes to the bound session as a prompt.`

// newSession starts a session running the configured agent, and binds
// the chat to it.
func (b *Bot) newSession(chatID, rest string) {
	project, agent := "", ""
	for _, w := range strings.Fields(rest) {
		if w == "" {
			continue
		}
		if project == "" {
			project = w
			continue
		}
		agent = w
	}
	if agent == "" {
		agent = b.cfg.Agent
	}
	if agent != "" && agent != "default" && !knownAgent(agent) {
		b.reply(chatID, fmt.Sprintf("Hyla has no agent %q.", agent))
		return
	}
	var s proto.Session
	cmd := []string{}
	if agent != "default" {
		cmd, _ = b.sessionCommand(agent)
	}
	if err := b.api.Call("create", proto.CreateParams{Project: project, Dir: b.cfg.Dir, Command: cmd}, &s); err != nil {
		b.reply(chatID, "Hyla could not start a session: "+err.Error())
		return
	}
	b.bind(chatID, s)
	// Its pane's process, if there is one: where it works.
	where := ""
	if p := s.Pane; p.Git != nil {
		where = fmt.Sprintf(" in %s%s", p.Git.Branch, changes(p.Git))
	}
	b.reply(chatID, fmt.Sprintf("%q is ready%s. Write a prompt, and the agent gets it.", b.nameSession(s), where))
}

// use binds the chat to an open session.
func (b *Bot) use(chatID, ref string) {
	if ref == "" {
		b.reply(chatID, "Which session? /ls lists them.")
		b.list(chatID)
		return
	}
	st, err := b.api.State()
	if err != nil {
		b.reply(chatID, "Hyla is unreachable: "+err.Error())
		return
	}
	var found []*proto.Session
	for i := range st.Sessions {
		s := &st.Sessions[i]
		if !takesPrompts(s) {
			continue
		}
		n := s.DisplayName(settings.Load().SessionName)
		if n == ref || s.Name == ref || s.ID == ref || strings.HasPrefix(s.ID, ref) || strings.EqualFold(n, ref) {
			found = append(found, s)
		}
	}
	if len(found) == 0 {
		for i := range st.Sessions {
			s := &st.Sessions[i]
			n := s.DisplayName(settings.Load().SessionName)
			if s.ID == ref || strings.HasPrefix(s.ID, ref) || s.Name == ref || strings.EqualFold(n, ref) {
				b.reply(chatID, fmt.Sprintf("%s has no terminal to take prompts. /ls lists the sessions that do.", n))
				return
			}
		}
		b.reply(chatID, fmt.Sprintf("Hyla has no session %q to take prompts. /ls lists those that do.", ref))
		return
	}
	if len(found) > 1 {
		by := ""
		for _, s := range found {
			n := s.DisplayName(settings.Load().SessionName)
			if strings.EqualFold(n, ref) || n == ref {
				if by != "" {
					by = ""
					break
				}
				by = s.ID
			}
		}
		if by == "" {
			b.reply(chatID, fmt.Sprintf("Hyla has %d sessions named %q. /use their ids.", len(found), ref))
			return
		}
		found = []*proto.Session{sessionOf(found, by)}
	}
	b.bind(chatID, *found[0])
	b.reply(chatID, fmt.Sprintf("This chat is %s%s now.", found[0].DisplayName(settings.Load().SessionName), where(found[0])))
}

// list shows the open sessions.
func (b *Bot) list(chatID string) {
	st, err := b.api.State()
	if err != nil {
		b.reply(chatID, "Hyla is unreachable: "+err.Error())
		return
	}
	var out strings.Builder
	cur := b.bindingOf(bindingKey(chatID)).Session
	for _, s := range st.Sessions {
		if !takesPrompts(&s) {
			continue // no terminal to send a prompt to
		}
		mark := "·"
		if s.ID == cur {
			mark = "→"
		}
		fmt.Fprintf(&out, "%s %s %s%s\n", mark, stateDot(s), s.DisplayName(settings.Load().SessionName), where(&s))
	}
	if out.Len() == 0 {
		b.reply(chatID, "No session takes prompts. /new starts one.")
		return
	}
	b.reply(chatID, strings.TrimRight(out.String(), "\n"))
}

// firstLine is a text's first line, cut short.
func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	if r := []rune(line); len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return line
}

// status shows the bound session's state, branch and changes.
func (b *Bot) status(chatID string) {
	s, err := b.bound(chatID)
	if err != nil {
		b.reply(chatID, "This chat has no session yet. /new starts one.")
		return
	}
	p := leadPane(s)
	var out strings.Builder
	fmt.Fprintf(&out, "%s %s\n", s.DisplayName(settings.Load().SessionName), stateDot(*s))
	fmt.Fprintf(&out, "%s · %s\n", p.Status, orDash(p.Message))
	if g := p.Git; g != nil {
		fmt.Fprintf(&out, "%s%s\n", g.Branch, changes(g))
	}
	fmt.Fprintf(&out, "%s", orDash(p.WorkDir()))
	b.reply(chatID, out.String())
}

// stop interrupts the bound session's pane, as Ctrl-C. It drops what
// waited in key's chat, even as the session is gone.
func (b *Bot) stop(key, chatID string) {
	// A prompt that waited goes with the turn it waited for: it is not
	// run as the turn it waited for ends.
	dropped := ""
	if n := b.dropWaiting(key); n > 0 {
		dropped = " Dropped the prompt that waited."
	}
	s, err := b.bound(chatID)
	if err != nil {
		b.reply(chatID, "This chat has no session yet. /new starts one."+dropped)
		return
	}
	if has, err := b.hasBackground(s.ID); err == nil && has {

		if turn := b.runningTurn(key).text; turn != "" {
			if err := b.api.Call("agent.stop", proto.SessionParams{ID: s.ID}, nil); err != nil {
				b.reply(chatID, err.Error())
				return
			}
			b.reply(chatID, "Ended: "+firstLine(turn)+dropped)
			return
		}
		b.reply(chatID, "Nothing runs there."+dropped)
		return
	}
	if err := b.api.Call("send", proto.SendParams{ID: s.Pane.ID, Data: "\x03"}, nil); err != nil {
		b.reply(chatID, err.Error())
		return
	}
	b.reply(chatID, "Interrupted."+dropped)
}

// dropWaiting drops the prompts that waited in a chat, which a stop ends
// with the turn they waited for, and reports how many did. A group's
// threads wait in chat structs of their own, so key says which stops:
// the chat's, and each of its threads'.
func (b *Bot) dropWaiting(key string) int {
	b.mu.Lock()
	n := 0
	var cards []string
	for k, c := range b.chats {
		if !threadOf(k, key) {
			continue
		}
		n += len(c.waiting)
		c.waiting = nil
		cards = append(cards, c.held...)
		c.held = nil
	}
	b.mu.Unlock()
	// The stop ends the prompts' wait too: their cards go.
	for _, msg := range cards {
		if err := b.msgs.DeleteMessage(msg); err != nil {
			b.logf("feishu: taking back a queued prompt's card: %v", err)
		}
	}
	return n
}

// answer sends text to the bound session, and Enter: an answer to the
// question the agent asks.
func (b *Bot) answer(key, chatID, text string) {
	s, err := b.bound(chatID)
	if err != nil {
		b.reply(chatID, "This chat has no session yet. /new starts one.")
		return
	}
	if b.cfg.Run == settings.FeishuRunCommand {
		// A background session has no prompt to answer in the pane.
		if has, err := b.hasBackground(s.ID); err == nil && has {
			b.reply(chatID, "This session runs its agent in the background: send the prompt as a message.")
			return
		}
	}
	// An answer to the agent's question ends a turn, as a prompt does: the
	// pane's hook writes its answer, and the chat's host replies with it.
	if text != "" && s.Pane.Status == proto.Waiting {
		// The agent asks, and its hook names the answer: the host follows
		// the turn, and replies with it. Else the answer is text.
		b.answering(key, s, turnInfo{event: key, chat: chatID})
		return
	}
	if err := b.send(s.Pane.ID, text, true); err != nil {
		b.reply(chatID, err.Error())
		return
	}
	// Its turn keeps hylad at its work, as a connection that stays does.
	b.api.Call("watch", map[string]bool{"stay": true}, nil)
	var card *turnCard
	if cards() && len(text) > 0 {
		// The turn shows by its card, which /stop ends.
		card = b.newTurnCard(key, chatID, langOf(text))
		b.card(card, "working", "", "")
	}
	if card == nil {
		b.reply(chatID, "Sent. /status shows the turn.")
		return
	}
	b.goTurn(func() {
		state, body, failed, _ := b.watchCard(card, s.Pane.ID, rows(b.read(s.Pane.ID)), time.Time{})
		if failed || state == "" {
			state = "error"
		}
		// The turn's card takes its end: the question it asks, or what
		// went wrong, under the same message that said it runs.
		b.replyTurn(key, "", chatID, card, Card{Title: titleOf(body), Text: body, State: state},
			"Sent. /status shows the turn.")
	})
}

// answering hands an answer to the agent's question, which ends the turn
// the chat's host follows: the pane's hook writes its answer, and the
// chat's host replies with it.
func (b *Bot) answering(key string, s *proto.Session, t turnInfo) {
	pane := leadPane(s).ID
	agent := b.conversationAgent(t.chat, "", s.ID)
	answers := b.prepare(pane, agent)
	if answers != "" {
		var info proto.Pane
		b.api.Call("pane.answer", proto.AnswerParams{Pane: pane, Dir: answers}, &info)
	}
	if err := b.send(pane, "", false); err != nil {
		b.reply(t.chat, err.Error())
		return
	}
	t.pane, t.start = pane, time.Now()
	if !b.shared() {
		b.goTurn(func() { b.followAnswer(key, t.chat, agent, answers, t) })
		return
	}
	// One host follows the turn, the one the chat's messages go to.
	b.goTurn(func() {
		if host, _ := b.claim("", t.chat); host != "" && host != b.host() && b.takeAt(t.event, b.host()) != b.host() {
			return
		}
		b.followAnswer(key, t.chat, agent, answers, t)
	})
}

// followAnswer follows the turn an answer ends, and replies with the
// hook's answer of it. The turn began with a prompt: t, the turn that
// prompt began, or one that starts at the answer.
func (b *Bot) followAnswer(key, chatID, agent, answers string, t turnInfo) {
	var card *turnCard
	if cards() && t.event != "" {
		card = b.newTurnCard(key, chatID, langOf(t.text))
		b.card(card, "working", "", "")
	}
	// The answer continues the prompt's turn, which t notes: drive looks
	// for the pane and the agent no more.
	s := &proto.Session{ID: chatID, Pane: proto.Pane{ID: t.pane, Status: proto.Working}}
	r, err := b.drive(s, "", card, t, agent, answers)
	if err != nil {
		if !b.card(card, "error", err.Error(), "") {
			b.reply(chatID, err.Error())
		}
		return
	}
	if r.hook {
		// The hook's answer is the turn's text, not the terminal's, and
		// it names the conversation the answer continued.
		b.setBinding(bindingKey(chatID), binding{Conversation: r.conversation})
	}
	reply := b.replyWith(r)
	b.replyTurn(key, t.event, chatID, card, Card{Title: titleOf(reply), Text: reply}, reply)
}

// close ends the bound session, once no turn runs.
func (b *Bot) close(chatID string) {
	s, err := b.bound(chatID)
	if err != nil {
		b.reply(chatID, "This chat has no session yet. /new starts one.")
		return
	}
	if s.Pane.Status == proto.Working {
		b.reply(chatID, "The agent is working. /stop interrupts it, then /close.")
		return
	}
	if err := b.api.Call("close", proto.IDParams{ID: s.ID}, nil); err != nil {
		b.reply(chatID, err.Error())
		return
	}
	b.unbind(chatID)
	b.reply(chatID, "Closed. /new starts another.")
}

// bound is the chat's session, now.
func (b *Bot) bound(chatID string) (*proto.Session, error) {
	id := b.bindingOf(bindingKey(chatID)).Session
	if id == "" {
		return nil, errors.New("no session")
	}
	s, err := b.session(id)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// nameSession names an unnamed session, which a shell would name by its
// folder, as its agent has yet to start.
func (b *Bot) nameSession(s proto.Session) string {
	name := plainName(s.DisplayName(settings.Load().SessionName))
	if s.Name == "" && strings.TrimSpace(name) != "" {
		b.api.Call("rename", proto.RenameParams{ID: s.ID, Name: name}, nil)
	}
	return name
}

// leadPane is a session's lead pane, else its first: a session no
// client's state has told hylad the lead of has none.
func leadPane(s *proto.Session) proto.Pane {
	if s.Pane.ID != "" {
		return s.Pane
	}
	if len(s.Panes) > 0 {
		return s.Panes[0]
	}
	return proto.Pane{}
}

// session finds an open session by id.
func (b *Bot) session(id string) (*proto.Session, error) {
	st, err := b.api.State()
	if err != nil {
		return nil, err
	}
	for i := range st.Sessions {
		if st.Sessions[i].ID == id {
			return &st.Sessions[i], nil
		}
	}
	return nil, errors.New("the session is gone")
}

// plainName takes the quotes off a name, which a title may hold.
func plainName(s string) string {
	return strings.Trim(s, `"'`)
}

// bindingKey is where a chat's binding is kept. A chat has one, whichever
// topic its messages came in: a topic's turn is a turn of the chat, which
// the chat's turn state keeps, and not a conversation of its own.
func bindingKey(chatID string) string { return chatKey(chatID, "") }

// bind binds a chat to a Hyla session, which its turns go to.
func (b *Bot) bind(chatID string, s proto.Session) {
	b.setBinding(bindingKey(chatID), binding{Session: s.ID})
}

// unbind takes the chat's session, which is gone or closed, off the
// chat: what its turns went on with, its agent and conversation, stays.
func (b *Bot) unbind(chatID string) {
	key := bindingKey(chatID)
	b.mu.Lock()
	bnd := b.bindings[key]
	had := bnd.Session != ""
	bnd.Session = ""
	if bnd != (binding{}) {
		b.bindings[key] = bnd
	} else {
		delete(b.bindings, key)
	}
	b.mu.Unlock()
	if had {
		b.save()
	}
}

// bindingOf is a chat's binding, as the turns before this one left it.
func (b *Bot) bindingOf(key string) binding {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bindings[key]
}

// setBinding sets what a turn knows of a chat's binding, and saves it.
// A field the change leaves empty is one it did not learn: it stays as it
// was, so that a turn which tells no conversation keeps the one before.
func (b *Bot) setBinding(key string, change binding) {
	b.mu.Lock()
	bnd := b.bindings[key]
	if change.Session != "" {
		bnd.Session = change.Session
	}
	if change.Agent != "" {
		bnd.Agent = change.Agent
	}
	if change.Dir != "" {
		bnd.Dir = change.Dir
	}
	if change.Conversation != "" {
		bnd.Conversation = change.Conversation
	}
	if bnd != (binding{}) {
		b.bindings[key] = bnd
	} else {
		delete(b.bindings, key)
	}
	b.mu.Unlock()
	b.save()
}

// takesPrompts tells a session a prompt can go to: one with a terminal.
func takesPrompts(s *proto.Session) bool {
	for i := range s.Panes {
		if !s.Panes[i].Shown() {
			return true
		}
	}
	return false
}

func sessionOf(l []*proto.Session, id string) *proto.Session {
	for _, s := range l {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// bindFile is where the chats' bindings go: hylad's directory, kept with
// its sessions, so that a chat finds its session again after hylad
// restarts.
func bindFile() string { return filepath.Join(proto.Dir(), "feishu-chats.json") }

// legacyChat is what a chat went on with, as a file of before the
// bindings kept it: read, and written no more.
type legacyChat struct {
	Agent string `json:"agent,omitempty"`
	Dir   string `json:"dir,omitempty"`
	Sid   string `json:"sid,omitempty"`
}

// save writes the bindings: one writer at a time, since the turns of
// several chats save at once, into the one file they load from.
func (b *Bot) save() {
	b.saveMu.Lock()
	defer b.saveMu.Unlock()
	b.mu.Lock()
	w := chatsFile{Turns: maps.Clone(b.times), Pins: maps.Clone(b.pins), Bindings: maps.Clone(b.bindings)}
	allowed := map[string][]string{}
	for key, c := range b.chats {
		for tool := range c.allowed {
			allowed[key] = append(allowed[key], tool)
		}
	}
	b.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(bindFile()), 0o700); err != nil {
		return
	}
	data, _ := json.Marshal(struct {
		chatsFile
		Allowed map[string][]string `json:"allowed,omitempty"`
	}{w, allowed})
	tmp := bindFile() + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil && os.Rename(tmp, bindFile()) != nil {
		os.Remove(tmp)
	}
}

// load reads the bindings, replacing the bot's: the file is the only thing
// that carries a chat's session, agent and conversation across a restart,
// and the bot reads it once, as it starts, before a chat's turns run.
func (b *Bot) load() {
	data, err := os.ReadFile(bindFile())
	if err != nil {
		return
	}
	// A file of before bindings: the sessions in binds, and what each
	// chat went on with, by its agent's own key, in chats.
	var w struct {
		chatsFile
		Binds   map[string]string     `json:"binds"`
		Chats   map[string]legacyChat `json:"chats"`
		Allowed map[string][]string   `json:"allowed"`
	}
	if json.Unmarshal(data, &w) != nil {
		return
	}
	bindings := maps.Clone(w.Bindings)
	if bindings == nil {
		bindings = map[string]binding{}
	}
	for k, id := range w.Binds {
		bnd := bindings[k]
		bnd.Session = id
		bindings[k] = bnd
	}
	for k, m := range w.Chats {
		bnd := bindings[k]
		bnd.Agent, bnd.Dir, bnd.Conversation = m.Agent, m.Dir, m.Sid
		bindings[k] = bnd
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bindings = bindings
	b.times = map[string]turnTimes{}
	for c, t := range w.Turns {
		b.times[c] = t
	}
	b.pins = map[string]string{}
	for s, c := range w.Pins {
		b.pins[s] = c
	}
	for k, tools := range w.Allowed {
		c := b.chatLocked(k)
		c.allowed = map[string]bool{}
		for _, tool := range tools {
			c.allowed[tool] = true
		}
	}
}

func (b *Bot) reply(chatID, text string) {
	key := chatKey(chatID, "")
	b.mu.Lock()
	event := b.claims[key]
	b.mu.Unlock()
	b.replyAt(key, event, chatID, text)
}

// replyAt answers a message in a chat as a reply to it, which the hosts
// that share the app hear and the chat's answers follow: event is the
// event the message answers, "" for a message of its own. Its card has
// the answer's own state, and no time or usage.
func (b *Bot) replyAt(key, event, chatID, text string) {
	b.replyTurn(key, event, chatID, nil, Card{Title: titleOf(text)}, text)
}

// replyTurn ends a turn in a chat: text says what the turn did, as
// "Sent." does, and card is the turn's answer, which its card shows.
// A card is the turn's answer, and text a note about it. c is the turn's
// card, which its answer takes, and nil for a turn that had none.
func (b *Bot) replyTurn(key, event, chatID string, c *turnCard, card Card, text string) {
	text = truncate(strings.TrimSpace(text), maxText)
	if text == "" {
		return
	}
	if b.shared() {
		// An answer names the event, which every host hears, and the host
		// the chat's messages go to, which /host set. Hylad keeps the
		// response, which the other hosts answer the event's next
		// messages with: Feishu does not give a host another host's
		// messages.
		b.mu.Lock()
		want := b.wants[key]
		b.mu.Unlock()
		text = hopText(event) + fromText(want) + text
		b.noteTurn(event, chatID, text)
	}
	// Every answer shows as a card (feishu-cards), as the wrapper the
	// agent runs in shows its turns: no message without one. An app that
	// several hosts share keeps every answer, which the hosts answer the
	// next messages with: the card goes beside it.
	answer := strings.TrimSpace(card.Text)
	if answer == "" {
		answer = text
	}
	// A card of its own, without an answer, as a note's, has a card of
	// the answer's state.
	if card.State == "" {
		card.State = "done"
	}
	// A turn in a group's topic shows there, as its prompt does: the
	// answer's card goes into the topic of the turn, or the chat's own.
	thread := threadOfKey(key)
	if c != nil && c.thread != "" {
		thread = c.thread
	}
	if cards() {
		full := cardWith(card, answer, text)
		// The turn's own card takes the answer: the turn shows as one
		// message, from Running to its end. A card that is gone, or one
		// Feishu would not take, leaves the answer to a card of its own.
		msg := ""
		if b.finishCard(c, full) {
			msg = c.msg
		} else if sent, err := b.replyCard(chatID, thread, full); err == nil {
			msg = sent
		}
		if msg != "" {
			if full.State == "asks" && askedQuestion(full.Text) {
				// The agent asks a question, and its options go in the
				// card, as buttons: its text is the question alone.
				b.askOptions(newChoice(key, chatID, msg, answer))
			}
			if !b.shared() {
				return
			}
		}
	}
	text = strings.Join([]string{answer, strings.TrimSpace(text)}, "\n\n")
	var err error
	if event != "" {
		_, err = sendReply(b.msgs, event, text)
		if err == nil {
			return
		}
	}
	if _, err = b.msgs.Send("chat_id", chatID, text); err != nil {
		b.logf("feishu: reply to %s: %v", chatID, err)
	}
}

// titleLen is how long a card's title is, about: a prompt's first line,
// or its words, cut where they end.
const titleLen = 60

// titleOf is the title of the card that answers a turn: the first line
// of its text, all of it when it is short, and its words cut otherwise.
func titleOf(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#>*-• "))
	if r := []rune(line); len(r) > titleLen {
		cut := titleLen
		for i := titleLen; i > titleLen/2; i-- {
			if r[i-1] == ' ' {
				cut = i - 1
				break
			}
		}
		line = strings.TrimSpace(string(r[:cut]))
	}
	if line == "" {
		return text
	}
	return line
}

// cardWith is a turn's card as the answer makes it: its title the
// answer's first words, unless the turn named one, its text the answer,
// and note what the turn did under it, when it says more.
func cardWith(c Card, text, note string) Card {
	if c.Title == "" {
		c.Title = titleOf(text)
	}
	c.Text = strings.TrimSpace(text)
	if strings.TrimSpace(note) == strings.TrimSpace(c.Text) {
		note = ""
	}
	c.Note = note
	if c.State == "" {
		c.State = "done"
	}
	return c
}

// replyCard answers a turn in a chat with a card, in the topic the
// answer is of, and returns the message Feishu took it as.
func (b *Bot) replyCard(chatID, thread string, c Card) (string, error) {
	text := c.Text
	shown, err := cardJSON(c, langOf(text))
	if err != nil {
		return "", err
	}
	msg, err := b.sendCard(chatID, thread, shown)
	if err != nil {
		b.logf("feishu: sending a card: %v", err)
	}
	return msg, err
}

// shared tells an app that several hosts connect from.
func (b *Bot) shared() bool { return len(b.cfg.Host) > 1 }

// keyOf is a chat's key, for the bot its key is not the chat's ID, as
// the owner's.
func (b *Bot) keyOf(chatID string) string { return chatKey(chatID, "") }

// chatKey is a chat's: a group's thread is a chat of its own.
func chatKey(chatID, thread string) string {
	if thread == "" {
		return chatID
	}
	return chatID + "\x00" + thread
}

// threadOfKey is the topic a chat's key is of: the root message of a
// group's thread, "" for the chat's own.
func threadOfKey(key string) string {
	_, root, ok := strings.Cut(key, "\x00")
	if !ok {
		return ""
	}
	return root
}

// threadOf tells whether a chat's key, k, is the chat key names or one of
// its threads'.
func threadOf(k, key string) bool {
	return k == key || strings.HasPrefix(k, key+"\x00")
}

func (b *Bot) chat(key string) *chat {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.chatLocked(key)
}

func (b *Bot) chatLocked(key string) *chat {
	c := b.chats[key]
	if c == nil {
		c = &chat{}
		b.chats[key] = c
	}
	return c
}

func (b *Bot) seenEvent(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.seen[id] {
		return true
	}
	b.seen[id] = true
	b.order = append(b.order, id)
	if len(b.order) > seenCap {
		delete(b.seen, b.order[0])
		b.order = b.order[1:]
	}
	return false
}

func (b *Bot) allowed(openID, userID string) bool {
	f := &b.cfg
	for _, a := range f.Allow {
		if a == "*" || a != "" && (a == openID || a == userID) {
			return true
		}
	}
	// Only who ran hyla feishu setup, until feishu-allow names others.
	return b.me != "" && (openID == b.me || userID == b.me)
}

// messageText is the text of a text message's content: {"text":"…"}.
func messageText(content string) string {
	var c struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &c); err != nil {
		// Only the text's JSON: "\n" and the like.
		var s string
		if json.Unmarshal([]byte(content), &s) == nil {
			return strings.TrimSpace(s)
		}
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(c.Text)
}

// rows counts the pane's lines.
func rows(text string) int { return strings.Count(text, "\n") }

// since is the end of a pane's text, rows after start on: what a turn
// added, at most tail of it.
func since(text string, start, tail int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if start < len(lines) {
		lines = lines[start:]
	} else {
		lines = nil
	}
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[len(s)-n:]
	if i := strings.Index(cut, "\n"); i >= 0 {
		cut = cut[i+1:]
	}
	return "…\n" + cut
}

func stateDot(s proto.Session) string {
	switch {
	case s.Pane.Status == proto.Working:
		return "●"
	case s.Pane.Status == proto.Waiting, s.Pane.Status == proto.Failed:
		return "◐"
	case s.Pane.Status == proto.Exited:
		return "✕"
	}
	return "○"
}

func where(s *proto.Session) string {
	if s == nil {
		return ""
	}
	if p := s.Pane; p.Git != nil {
		return fmt.Sprintf(" — %s%s", p.Git.Branch, changes(p.Git))
	}
	if d := s.Pane.WorkDir(); d != "" {
		return " — " + d
	}
	return ""
}

func changes(g *proto.GitInfo) string {
	if g == nil || (g.Added == 0 && g.Removed == 0 && g.Files == 0) {
		return ""
	}
	return fmt.Sprintf(" +%d −%d", g.Added, g.Removed)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNotEmpty(l ...string) string {
	for _, s := range l {
		if s != "" {
			return s
		}
	}
	return ""
}

// discard closes a session the chat does not take.
func (b *Bot) discard(id string) {
	b.api.Call("close", proto.IDParams{ID: id}, nil)
}

// agentCommand is the command a new session runs the agent by: the
// feishu-agent-command line's words, or else the agent's program.
func agentCommand(f settings.Feishu, agent string) []string {
	if cmd := f.AgentCommand(agent); len(cmd) > 0 {
		return cmd
	}
	return []string{agent}
}

func knownAgent(agent string) bool { return detect.Known(agent) }

func sendReply(m Messages, id, text string) (string, error) {
	return m.SendReply(id, text)
}
