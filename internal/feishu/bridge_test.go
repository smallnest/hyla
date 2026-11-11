package feishu

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"

	"hyla/internal/proto"
	"hyla/internal/settings"
)

// fakeLuna is hylad: sessions, panes, and what the bot sends.
type fakeLuna struct {
	host     string   // a host.info's hostname
	order    []string // the methods that act on a session
	sessions []proto.Session
	sent     []proto.SendParams
	sentMu   sync.Mutex // held while a test reads sent, which a turn writes as it runs
	created  []proto.CreateParams
	screen   string
	waits    int
	waitEnd  proto.Status // what the wait ends with, working while there are waits left
	closed   []string
	busy     bool     // the pane keeps working
	files    []string // what each agent.output gives: "2" then ""
	ran      []proto.BackgroundParams
	input    []string      // what agent.send wrote to a command's input
	mu       chan struct{} // held while a test reads what a goroutine writes
	// script is what a pane shows, and its status, each time it is read:
	// a turn of the agent whose status lags behind its screen, as
	// Claude Code's for a moment.
	script []struct {
		screen string
		status proto.Status
	}
	reads    []proto.ReadParams     // what was read
	msgs     []*larkim.EventMessage // the messages sent or heard, by ID
	kept     map[string]string      // the responses hylad keeps, by their events
	keeps    []string               // the events hylad keeps a response of
	took     map[string]string      // the host that took an event, by its message
	keptChat map[string]string      // the chat of a response hylad keeps, by its event
	byID     map[string]*string     // a sent message's chat, whose message Feishu gives by ID alone
	answers  []string               // the answer directories a pane was given
	onRead   func(string)           // what the first read does, as an agent's hook waking
	calls    map[string]func()      // what a method does as it is called
	hold     bool                   // the screen stays as set, not the script's last
}

func (f *fakeLuna) State() (proto.State, error) {

	out := proto.State{}
	for _, s := range f.sessions {
		// As hylad does: the session's fields are its lead pane's,
		// which the state gives anew.
		s.Pane = s.Panes[0]
		out.Sessions = append(out.Sessions, s)
	}
	return out, nil
}

func (f *fakeLuna) Call(method string, params, out any) error {
	b, _ := json.Marshal(params)
	// Each method's params, by name: SendParams' ID is a pane's, and
	// WaitParams' too, which a struct of embedded types would not tell.
	var p struct {
		ID, Data, Project, Dir, Text, Parent string
		Event, Host, Chat                    string
		Command                              []string
		Scrollback                           bool
		Lines                                int
		Status                               []proto.Status
		Exit                                 bool
		Timeout                              int64
		Out                                  string
		From                                 int64
		Pane                                 string
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	send := proto.SendParams{ID: p.ID, Data: p.Data}
	id := proto.IDParams{ID: p.ID}
	create := proto.CreateParams{Project: p.Project, Dir: p.Dir, Command: p.Command}
	decode := func(v any) { x, _ := json.Marshal(v); json.Unmarshal(x, out) }
	switch method {
	case "state":
		st, _ := f.State()
		decode(st)
	case "pane":
		var p proto.Pane
		if len(f.sessions) > 0 {
			p = f.sessions[0].Panes[0]
		}
		decode(p)
	case "send":
		if f.calls["send"] != nil {
			f.calls["send"]()
		}
		f.sentMu.Lock()
		f.sent = append(f.sent, send)
		f.sentMu.Unlock()
	case "read":
		if f.calls["read"] != nil {
			f.calls["read"]()
		}
		text := f.screen
		if f.onRead != nil {
			f.onRead(id.ID)
		}
		if len(f.script) > 0 {
			text = f.script[0].screen
			f.script = f.script[1:]
			if !f.hold {
				f.screen = text
			}
		}
		decode(proto.ReadResult{Text: text})
	case "feishu.reply":
		// A message the bot sent, and then its answers, as hylad keeps
		// them for Feishu.
		f.reads = append(f.reads, proto.ReadParams{ID: p.ID})
		if out != nil {
			decode("om_x")
		}
	case "host.info":
		host := firstNotEmpty(f.host, "mac")
		decode(proto.HostInfo{Hostname: host})
	case "select":
		f.order = append(f.order, "select")
	case "wait":
		f.waits++
		if f.busy {
			f.sessions[0].Panes[0].Status = proto.Working
		} else if f.working() {
			// A turn seen working: the pane stays so until its screen
			// says the turn is over.
			f.sessions[0].Panes[0].Status = proto.Working
		} else {
			f.sessions[0].Panes[0].Status = proto.Idle
		}
		decode(f.sessions[0].Panes[0])
		st := f.waitEnd
		if f.waits > 1 && f.waitEnd == proto.Working {
			st = proto.Idle
		}
		f.sessions[0].Panes[0].Status = st
		decode(f.sessions[0].Panes[0])
	case "agent.background":
		f.ran = append(f.ran, proto.BackgroundParams{ID: p.ID, Out: p.Out, Dir: p.Dir, Command: p.Command})
		if out != nil {
			decode(true)
		}
	case "agent.send":
		f.mu <- struct{}{}
		f.input = append(f.input, p.Text)
		<-f.mu
		if out != nil {
			decode(true)
		}
	case "pane.answer":
		f.answers = append(f.answers, p.Dir)
		if out != nil {
			decode(proto.Pane{ID: p.Pane, FeishuAnswer: p.Dir})
		}
	case "agent.output":
		// What the command printed, then its end.
		text := f.files[0]
		f.files = f.files[1:]
		res := proto.Output{Text: text, From: int64(len(text)), Ended: text == ""}
		decode(res)
	case "session.hasBackground":
		decode(false)
	case "session.setBackground":
		if out != nil {
			decode(true)
		}
	case "create":
		f.created = append(f.created, create)
		s := proto.Session{ID: "s-new", Name: "review"}
		s.Panes = []proto.Pane{{ID: "p-new", Status: proto.Idle}}
		s.Pane = s.Panes[0]
		f.sessions = append(f.sessions, s)
		if r, ok := out.(*proto.Session); ok {
			*r = s // a value beside the pointer: hylad's client fills both
		}
	case "rename":
		if out != nil {
			decode(true)
		}
	case "close":
		f.closed = append(f.closed, id.ID)
		if out != nil {
			decode(true)
		}
	case "feishu.feed":
		if r, ok := out.(*proto.FeishuFeed); ok {
			r.ID, r.Name = "s1", "api"
			for i := 0; i < 5; i++ {
				r.Lines = append(r.Lines, proto.FeishuLine{Kind: "agent", Text: "line"})
			}
		}
	case "feishu.message":
		// A message Feishu has: the one f.msgs has, which the hosts
		// hear.
		if r, ok := out.(*proto.FeishuMessage); ok {
			if m, err := f.Kept(p.ID); err == nil {
				// As hylad's, which Feishu's JSON fills.
				x, _ := json.Marshal(m)
				json.Unmarshal(x, r)
			}
		}
	case "feishu.keep":
		// A turn's response, which every host answers.
		f.kept[p.Event] = p.Text
		f.keptChat[p.Event] = p.Chat
		f.keeps = append(f.keeps, p.Event)
		if out != nil {
			decode(true)
		}
	case "feishu.kept":
		// Hylad keeps a turn's response.
		out := &proto.FeishuKept{Turns: []proto.FeishuTurn{}}
		for i := len(f.keeps) - 1; i >= 0; i-- {
			if ev := f.keeps[i]; f.kept[ev] != "" {
				out.Turns = append(out.Turns, proto.FeishuTurn{Event: ev, Text: f.kept[ev]})
				break
			}
		}
		decode(out)
	case "feishu.take":
		// The host that took an event: the one hylad keeps, or the one
		// that asks, which it keeps then.
		taken := f.took[p.Event]
		if taken == "" {
			taken = p.Host
			f.took[p.Event] = taken
		}
		decode(proto.FeishuTaken{Host: taken})
	}
	return nil
}

// working tells a turn seen working, whose screen says the turn is over.
func (f *fakeLuna) working() bool {
	if len(f.script) == 0 {
		return false
	}
	return f.script[0].status == proto.Working
}

// Kept is a message Feishu has: the one f.msgs has, which the hosts
// hear.
func (f *fakeLuna) Kept(id string) (proto.FeishuMessage, error) {
	for _, m := range f.msgs {
		if m != nil && deref(m.MessageId) == id {
			c := *m
			if chat := f.byID[id]; chat != nil {
				c = proto.FeishuMessage{ChatId: chat}
			}
			return c, nil
		}
	}
	return proto.FeishuMessage{}, errors.New("Feishu has no such message")
}

type fakeMessages struct {
	sent     []string
	shown    []string // each card sent, as its JSON: a turn's, or a queued prompt's
	cards    []string // each card's JSON, sent or updated
	updates  []string // each update's message ID, and the card's JSON
	deleted  []string // each message the bot took back
	replies  []string // each reply to a message
	reacts   []string
	unreacts []string
	fail     bool // sending a card fails: the answers go as messages
	// patchFail tells a Feishu that refuses the update of a message, as
	// one it takes for too frequent (230020) does.
	patchFail bool
	// topics are where the cards went, in order: the root message of the
	// topic a card answered, "" for a card of the chat's own.
	topics  []string
	keep    *fakeLuna // what hylad keeps of what is sent
	targets []string  // each message's receive ID, "kind id"
	kept    int       // the messages sent, which the kept ones count
	// send sends the text of every message, as a chat's does; false when
	// the tests read the cards alone.
	send bool
}

func (m *fakeMessages) Send(kind, id, text string) (string, error) {
	if m.send || m.fail {
		m.sent = append(m.sent, text)
	}
	m.targets = append(m.targets, kind+" "+id)
	// What hylad keeps of what is sent, which another host hears: Feishu,
	// as the message that every host finds.
	m.kept++
	id = fmt.Sprintf("om_sent_%d", m.kept)
	m.keep.msgs = append(m.keep.msgs, &larkim.EventMessage{MessageId: str(id), ChatId: str("c1"), ChatType: str("p2p"), MessageType: str("text"), Content: str(text)})
	m.keep.kept[id] = text
	m.keep.byID[id] = str("c1")
	return id, nil
}

func (m *fakeMessages) SendCard(kind, id, cardJSON string) (string, error) {
	// A turn's card, as schema 2.0's is, fails as Feishu refused one: an
	// answer's, with the quotes of a string's JSON, stays a card.
	t := strings.TrimSpace(cardJSON)
	if m.fail && false && len(t) > 1 && t[1] == 's' {
		return "", fmt.Errorf("no card")
	}
	m.shown = append(m.shown, cardJSON)
	m.cards = append(m.cards, cardJSON)
	m.topics = append(m.topics, "")
	return fmt.Sprintf("om_card_%d", len(m.cards)), nil
}

// SendCardReply sends a card into the topic of the message it answers.
func (m *fakeMessages) SendCardReply(messageID, cardJSON string) (string, error) {
	m.shown = append(m.shown, cardJSON)
	m.cards = append(m.cards, cardJSON)
	m.topics = append(m.topics, messageID)
	return fmt.Sprintf("om_card_%d", len(m.cards)), nil
}

func (m *fakeMessages) DeleteMessage(id string) error {
	m.deleted = append(m.deleted, id)
	return nil
}

func (m *fakeMessages) UpdateCard(id, cardJSON string) error {
	if m.patchFail {
		return fmt.Errorf("Feishu answered 230020 update too frequent")
	}
	m.updates = append(m.updates, id+": "+cardJSON+fmt.Sprintf(" [marks %v]", m.reacts))
	m.cards = append(m.cards, cardJSON)
	return nil
}

// waitFor waits for what a turn does as it runs, which a test's goroutine
// sees a moment later.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if ok() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s: nothing", what)
}

// sends is what the bot sent, as a test reads it once its turns may run
// still.
func (f *fakeLuna) sends() []proto.SendParams {
	f.sentMu.Lock()
	defer f.sentMu.Unlock()
	return append([]proto.SendParams{}, f.sent...)
}

// sendReply records a reply apart from a card's message: sending replies
// fails, as Feishu's does for an app that no longer replies to events,
// unless the cards fail too, as an app without them.
func (m *fakeMessages) SendReply(id, text string) (string, error) {
	if !m.fail {
		return "", fmt.Errorf("no reply")
	}
	m.sent = append(m.sent, text)
	m.replies = append(m.replies, text)
	return m.keptReply(id, text), nil
}

// keptReply is a reply Feishu has: which every host hears, as the
// message of its ID.
func (m *fakeMessages) keptReply(parent, text string) string {
	if m.keep == nil {
		return fmt.Sprintf("om_reply_%d", len(m.replies))
	}
	id := fmt.Sprintf("om_reply_%d", len(m.replies))
	m.keep.msgs = append(m.keep.msgs, &larkim.EventMessage{MessageId: str(id), ParentId: str(parent), ChatId: str("c1"), ChatType: str("p2p"), MessageType: str("text"), Content: str(text)})
	m.keep.kept[id] = text
	m.keep.byID[id] = str("c1")
	return id
}

func (m *fakeMessages) Get(id string) (feishuMessage, error) {
	return feishuMessage{MessageId: str(id), ChatId: str("c1")}, nil
}

func (m *fakeMessages) React(id, emoji string) error {
	m.reacts = append(m.reacts, id+":"+emoji)
	return nil
}

func (m *fakeMessages) Unreact(id, emoji string) error {
	m.unreacts = append(m.unreacts, id+":"+emoji)
	return nil
}

func (m *fakeMessages) Download(key string, image bool) ([]byte, error) {
	return []byte("\x89PNG\r\n\x1a\n0000"), nil
}

// lastCard is the last card sent or updated, nil for none.
func (m *fakeMessages) lastCard(t *testing.T) map[string]any {
	var c map[string]any
	if len(m.cards) == 0 {
		if t != nil {
			t.Helper()
			t.Fatal("no card")
		}
		return nil
	}
	if err := json.Unmarshal([]byte(m.cards[len(m.cards)-1]), &c); err != nil && t != nil {
		t.Helper()
		t.Fatal(err)
	}
	return c
}

// cardText is the text of the last card.
func (m *fakeMessages) cardText() string { return cardText(m.lastCard(nil)) }

// cardText is a card's text.
func cardText(c map[string]any) string {
	body, _ := c["body"].(map[string]any)
	elems, _ := body["elements"].([]any)
	text := ""
	for _, e := range elems {
		m, _ := e.(map[string]any)
		// The state's line comes first, its text after it; the footer is
		// no text.
		size, _ := m["text_size"].(string)
		if (m["tag"] == "markdown" || m["tag"] == "div") && size != "notation" {
			s, _ := m["content"].(string)
			if t, ok := m["text"].(map[string]any); ok {
				s, _ = t["content"].(string)
			}
			text += s + "\n"
		}
	}
	return text
}

func newBot(t *testing.T) (*Bot, *fakeLuna, *fakeMessages) {
	t.Helper()
	hylad = nil
	hylad = nil
	t.Cleanup(func() { hylad = nil })
	t.Setenv("HYLA_HOME", t.TempDir())
	t.Setenv("HYLA_CONFIG", filepath.Join(t.TempDir(), "config"))
	settings.Reload()
	defer settings.Reload()
	luna := &fakeLuna{sessions: []proto.Session{session()}}
	msgs := &fakeMessages{}
	luna.calls = map[string]func(){}
	luna.mu = make(chan struct{}, 1)
	luna.kept = map[string]string{}
	luna.took = map[string]string{}
	luna.byID = map[string]*string{}
	luna.keptChat = map[string]string{}
	luna.msgs = append(luna.msgs, &larkim.EventMessage{MessageId: str("om_user"), ChatId: str("c1"), ChatType: str("p2p"), MessageType: str("text"), Content: jsonText("hi")})
	b := New(Config{
		Settings: settings.Feishu{App: "cli_a", Allow: []string{"*"}, Timeout: int64(settings.FeishuDefaultTimeout), Agent: "default"},
		Secret:   settings.FeishuSecret{AppID: "cli_a", AppSecret: "x", Me: "ou_me"},
		Luna:     luna,
		Log:      t.Logf,
	})
	b.load()
	msgs.keep = luna
	hylad = luna
	b.msgs = msgs
	b.files = msgs
	b.sync = true
	// A turn of a test starts no OpenCode server: what its command
	// reports is what the card shows.
	b.noOC = true
	// The bot's own identity, which a group's mention of it carries.
	b.botID, b.botName = "ou_bot", "hyla"
	return b, luna, msgs
}

// bindChat gives a chat a binding in a test, as a turn of it would, and
// the chat state a turn leaves.
func bindChat(b *Bot, key string, bnd binding) {
	b.mu.Lock()
	if bnd != (binding{}) {
		b.bindings[key] = bnd
	}
	b.chatLocked(key)
	b.mu.Unlock()
}

func session() proto.Session {
	s := proto.Session{ID: "s1", Name: "api"}
	s.Panes = []proto.Pane{{ID: "p1", Status: proto.Idle, Dir: "/tmp/api"}}
	s.Pane = s.Panes[0]
	return s
}

func str(s string) *string { return &s }

// nextEvent numbers the events, which Feishu never repeats.
var nextEvent int

func textMsg(chat, from, text string) *larkim.P2MessageReceiveV1 {
	content, _ := json.Marshal(map[string]string{"text": text})
	nextEvent++
	return &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{AppID: "cli_a", EventID: fmt.Sprintf("ev_%d_%d", nextEvent, len(text))}},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender:  &larkim.EventSender{SenderId: &larkim.UserId{OpenId: str(from)}},
			Message: &larkim.EventMessage{ChatId: str(chat), ChatType: str("p2p"), MessageType: str("text"), Content: str(string(content))},
		},
	}
}

// textMsgFrom is a text message with its ID, as a turn names it.
func textMsgFrom(chat, id, text string) *larkim.P2MessageReceiveV1 {
	e := textMsg(chat, "ou_me", text)
	e.Event.Message.MessageId = str(id)
	return e
}

// last is the last answer: its card's text, as feishu-cards sends it, or
// the message that answers without cards.
func last(t *testing.T, msgs *fakeMessages) string {
	t.Helper()
	if msgs.fail {
		if len(msgs.sent) == 0 {
			return ""
		}
		return msgs.sent[len(msgs.sent)-1]
	}
	if len(msgs.cards) == 0 {
		return ""
	}
	return cardText(parseCard(t, msgs.cards[len(msgs.cards)-1]))
}

// jsonText is a message's content as Feishu sends a text message's: JSON.
func jsonText(text string) *string {
	b, _ := json.Marshal(map[string]string{"text": text})
	return str(string(b))
}

// /new starts a session, and a prompt goes to its pane, then Enter, and
// the reply holds what the turn printed.
func TestPrompt(t *testing.T) {
	b, luna, msgs := newBot(t)
	b.onMessage(nil, textMsg("c1", "ou_me", "/new"))
	if len(luna.created) != 1 || !strings.Contains(last(t, msgs), `"review" is ready`) {
		t.Fatalf("created %+v, replied %q", luna.created, last(t, msgs))
	}
	if got := b.bindingOf("c1").Session; got != "s-new" {
		t.Fatalf("the chat is bound to %q", got)
	}

	luna.screen = "old\n$ "
	luna.waitEnd = proto.Working
	shortWaits(t)
	b.onMessage(nil, textMsg("c1", "ou_me", "fix the tests"))
	// The turn, which reads the pane until the agent stops, may write
	// still: its prompt is the last sent.
	var sent []proto.SendParams
	waitFor(t, "the prompt", func() bool {
		if s := luna.sends(); len(s) >= 2 {
			sent = s[len(s)-2:]
			return true
		}
		return false
	})
	if sent[0].Data != "fix the tests" || sent[1].Data != "\r" || sent[0].ID != "p-new" {
		t.Fatalf("sent %+v", sent)
	}
	luna.screen = "old\n$ \n● Fixed 2 tests.\n"
	b.onMessage(nil, textMsg("c1", "ou_me", "/ls"))
	if len(luna.sends()) == 0 {
		t.Fatal("no prompt went")
	}
}

// A prompt that comes while a turn runs waits its turn, one deep, in a
// card titled for the queue, which goes as the turn that runs it begins;
// a command does not wait, which /stop needs.
func TestPromptWaitsItsTurn(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	luna.screen = "old\n$ "
	// The turn another message started, which the chat still runs.
	if !b.enter("c1") {
		t.Fatal("the chat took no turn")
	}
	b.onMessage(nil, textMsg("c1", "ou_me", "second"))
	// Its answer is a card: blue, titled for the queue, over the prompt.
	queued := parseCard(t, msgs.shown[len(msgs.shown)-1])
	if got, head := cardText(queued), queued["header"].(map[string]any); !strings.Contains(got, "second") ||
		head["template"] != "blue" ||
		!strings.Contains(head["title"].(map[string]any)["content"].(string), "queue") {
		t.Fatalf("the queued prompt's card is %v", queued)
	}
	if slices.Contains(luna.sent, proto.SendParams{ID: "p1", Data: "second"}) {
		t.Fatalf("the prompt ran while a turn ran: %+v", luna.sent)
	}
	// One deep: the next prompt is told the one before waits.
	b.onMessage(nil, textMsg("c1", "ou_me", "third"))
	if got := cardText(parseCard(t, msgs.shown[len(msgs.shown)-1])); !strings.Contains(got, "one waits") {
		t.Fatalf("a third prompt replies %q", got)
	}
	// A command runs while the turn runs, and leaves it running.
	b.onMessage(nil, textMsg("c1", "ou_me", "/status"))
	if got := last(t, msgs); !strings.Contains(got, "idle") {
		t.Fatalf("/status beside a turn replies %q", got)
	}
	if !b.chat("c1").busy {
		t.Fatal("a command ended the turn that ran")
	}
	// The turn ends, and the waiting prompt runs, its card gone.
	b.leave("c1")
	if !slices.Contains(luna.sent, proto.SendParams{ID: "p1", Data: "second"}) {
		t.Fatalf("the waiting prompt did not run: %+v", luna.sent)
	}
	if len(msgs.deleted) != 1 || msgs.deleted[0] != "om_card_1" {
		t.Fatalf("the queued prompt's card goes as %v", msgs.deleted)
	}
	if b.chat("c1").busy {
		t.Fatal("the chat runs a turn after the queued one ended")
	}
}

// /stop ends the turn it comes beside, and the prompt that waited with
// it, whose card goes too.
func TestStopDropsTheWaitingPrompt(t *testing.T) {
	b, luna, msgs := newBot(t)
	bindChat(b, "c1", binding{Session: "s1"})
	b.chat("c1").chat = "c1"
	b.leave("c1")
	b.enter("c1")
	b.onMessage(nil, textMsg("c1", "ou_me", "second"))
	queued := parseCard(t, msgs.shown[len(msgs.shown)-1])
	if head := queued["header"].(map[string]any); head["template"] != "blue" ||
		!strings.Contains(head["title"].(map[string]any)["content"].(string), "queue") {
		t.Fatalf("a prompt while a turn runs is queued as %v", queued)
	}
	b.onMessage(nil, textMsg("c1", "ou_me", "/stop"))
	if got := last(t, msgs); !strings.Contains(got, "Interrupted") || !strings.Contains(got, "Dropped") {
		t.Fatalf("/stop beside a turn replies %q", got)
	}
	if len(msgs.deleted) != 1 || msgs.deleted[0] != "om_card_1" {
		t.Fatalf("a stop leaves the queued prompt's card: %v", msgs.deleted)
	}
	b.leave("c1")
	if slices.Contains(luna.sent, proto.SendParams{ID: "p1", Data: "second"}) {
		t.Fatalf("a stopped turn ran the prompt that waited: %+v", luna.sent)
	}
}

// A stop ends a group thread's waiting prompt too, with its card: the
// thread waits in a chat struct of its own.
func TestStopDropsAThreadsWaitingPrompt(t *testing.T) {
	b, luna, msgs := newBot(t)
	b.cfg.Allow = []string{"ou_me"}
	prompt := func(text string) *larkim.P2MessageReceiveV1 {
		m := textMsg("g1", "ou_me", text)
		*m.Event.Message.ChatType = "group"
		m.Event.Message.RootId = str("om_root")
		m.Event.Message.Mentions = []*larkim.MentionEvent{{Key: str("@_user_1"), Id: &larkim.UserId{OpenId: str("ou_bot")}, MentionedType: str("bot")}}
		return m
	}
	bindChat(b, "g1", binding{Session: "s1"})
	b.chat("g1").chat = "g1"
	b.leave("g1")
	b.enter(chatKey("g1", "om_root"))
	b.onMessage(nil, prompt("second"))
	if len(msgs.shown) != 1 {
		t.Fatalf("a prompt while the thread's turn runs is queued as %v", msgs.shown)
	}
	// The queued prompt waits in the topic it came in.
	if len(msgs.topics) != 1 || msgs.topics[0] != "om_root" {
		t.Fatalf("the queued prompt's card went to %v", msgs.topics)
	}
	b.onMessage(nil, prompt("/stop"))
	if got := last(t, msgs); !strings.Contains(got, "Dropped") {
		t.Fatalf("/stop in a group thread replies %q", got)
	}
	if len(msgs.deleted) != 1 || msgs.deleted[0] != "om_card_1" {
		t.Fatalf("a stop leaves the thread's queued card: %v", msgs.deleted)
	}
	b.leave(chatKey("g1", "om_root"))
	if slices.Contains(luna.sent, proto.SendParams{ID: "p1", Data: "second"}) {
		t.Fatalf("a stopped thread ran the prompt that waited: %+v", luna.sent)
	}
}

// The reply is what the turn added: not the old screen, and the last
// rows only.
func TestReplyTail(t *testing.T) {
	start := "a\nb\nc\n"
	screen := start + strings.Repeat("x\n", 40) + "done\n"
	got := since(screen, rows(start), 3)
	if want := "x\nx\ndone"; got != want {
		t.Errorf("tail %q, want %q", got, want)
	}
	if got := since("nothing new\n", rows("a\nb\nc\n"), 5); got != "" {
		t.Errorf("nothing new gives %q", got)
	}
}

// A message from someone not allowed is refused, and nothing runs.
func TestNotAllowed(t *testing.T) {
	b, luna, msgs := newBot(t)
	b.cfg.Allow = nil
	b.onMessage(nil, textMsg("c1", "ou_stranger", "/new"))
	if len(luna.created) != 0 || len(msgs.cards) != 1 || !strings.Contains(last(t, msgs), "not allowed") {
		t.Fatalf("created %+v, replied %q", luna.created, msgs.sent)
	}
	// Who ran setup may, and another event of the same ID is ignored.
	dup := textMsg("c2", "ou_me", "/new")
	b.onMessage(nil, dup)
	b.onMessage(nil, dup)
	if len(luna.created) != 1 {
		t.Errorf("created %d sessions, want 1: a redelivered event runs twice", len(luna.created))
	}
}

// In a group, only a message that @-mentions the bot, or answers it,
// acts.
func TestGroupMentions(t *testing.T) {
	b, luna, msgs := newBot(t)
	m := textMsg("g1", "ou_me", "/new")
	*m.Event.Message.ChatType = "group"
	b.onMessage(nil, m)
	if len(luna.created) != 0 {
		t.Fatal("a group message without a mention acted")
	}
	m = textMsg("g1", "ou_me", "/new")
	*m.Event.Message.ChatType = "group"
	m.Event.Message.Mentions = []*larkim.MentionEvent{{Key: str("@_user_1"), Id: &larkim.UserId{OpenId: str("ou_bot")}, MentionedType: str("bot")}}
	b.onMessage(nil, m)
	if len(luna.created) != 1 || last(t, msgs) == "" {
		t.Fatalf("a mention didn't act: %+v", luna.created)
	}
	if got := b.bindingOf(chatKey("g1", "")).Session; got != "s-new" {
		t.Errorf("the group is bound to %q", got)
	}
}

// Commands: /ls marks the chat's session, /use binds by name, /status
// tells what the pane does, /close ends it once idle.
func TestCommands(t *testing.T) {
	b, luna, msgs := newBot(t)
	b.onMessage(nil, textMsg("c1", "ou_me", "/use api"))
	if !strings.Contains(last(t, msgs), "This chat is api") {
		t.Fatalf("/use replied %q", last(t, msgs))
	}
	b.onMessage(nil, textMsg("c1", "ou_me", "/ls"))
	if !strings.Contains(last(t, msgs), "→ ○ api") {
		t.Errorf("/ls lists %q", last(t, msgs))
	}
	b.onMessage(nil, textMsg("c1", "ou_me", "/status"))
	if !strings.Contains(last(t, msgs), "idle") || !strings.Contains(last(t, msgs), "/tmp/api") {
		t.Errorf("/status shows %q", last(t, msgs))
	}
	luna.sessions[0].Panes[0].Status = proto.Working
	b.onMessage(nil, textMsg("c1", "ou_me", "/close"))
	if len(luna.closed) != 0 {
		t.Error("/close ended a session working")
	}
	luna.sessions[0].Panes[0].Status = proto.Idle
	b.onMessage(nil, textMsg("c1", "ou_me", "/close"))
	if !slices.Equal(luna.closed, []string{"s1"}) || b.bindingOf("c1").Session != "" {
		t.Errorf("closed %v, bound %q", luna.closed, b.bindingOf("c1").Session)
	}
	b.onMessage(nil, textMsg("c1", "ou_me", "/nosuch"))
	if !strings.Contains(last(t, msgs), "/help") {
		t.Errorf("an unknown command replies %q", last(t, msgs))
	}
}

// A chat keeps its session as the bot restarts, and loses the binding
// when the session is gone.
func TestBindsPersist(t *testing.T) {
	b, luna, msgs := newBot(t)
	b.onMessage(nil, textMsg("c1", "ou_me", "/use api"))
	again := New(Config{Settings: b.cfg, Secret: settings.FeishuSecret{AppID: "cli_a", AppSecret: "x", Me: "ou_me"}, Luna: luna, Log: t.Logf})
	again.msgs = msgs
	again.sync = true
	again.load()
	if got := again.bindingOf("c1").Session; got != "s1" {
		t.Fatalf("after a restart the chat is bound to %q", got)
	}
	luna.sessions = nil
	again.onMessage(nil, textMsg("c1", "ou_me", "hello"))
	if !strings.Contains(last(t, msgs), "gone") || again.bindingOf("c1").Session != "" {
		t.Fatalf("a prompt to a gone session replied %q, bound %q", last(t, msgs), again.bindingOf("c1").Session)
	}
}

// A chat's binding is the chat's, whichever topic a message came in: one
// whose session went in a group thread unbinds the chat, and leaves no
// binding of the topic's.
func TestGoneSessionUnbindsTheChatFromATopic(t *testing.T) {
	b, luna, msgs := newBot(t)
	bindChat(b, "g1", binding{Session: "s1"})
	luna.sessions = nil
	b.onMessage(nil, groupMsgOf("g1", "t1", "m1", "hello"))
	if got := last(t, msgs); !strings.Contains(got, "gone") {
		t.Fatalf("a prompt to a gone session replied %q", got)
	}
	if got := b.bindingOf("g1").Session; got != "" {
		t.Errorf("the chat stays bound to %q after its session went in a topic", got)
	}
	if got := b.bindingOf(chatKey("g1", "t1")).Session; got != "" {
		t.Errorf("the topic holds a binding of its own: %q", got)
	}
}

// A chat's agent conversation goes on across a restart of the bot, as its
// bound session does: the next turn continues the conversation the last
// one left, not a new one.
func TestChatContinuesAcrossRestarts(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	b.cfg.Run = settings.FeishuRunCommand
	b.cfg.Agent = "opencode"
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	luna.files = []string{
		`{"type":"step_start","sessionID":"ses_A"}`, "",
		`{"type":"step_start","sessionID":"ses_B"}`, "",
	}
	b.onMessage(nil, textMsg("c1", "ou_me", "first"))
	if got := b.bindingOf("c1").Conversation; got != "ses_A" {
		t.Fatalf("the first turn's conversation is %q", got)
	}

	again := New(Config{Settings: b.cfg, Secret: settings.FeishuSecret{AppID: "cli_a", AppSecret: "x", Me: "ou_me"}, Luna: luna, Log: t.Logf})
	again.msgs = msgs
	again.sync = true
	again.load()
	if got := again.bindingOf("c1").Conversation; got != "ses_A" {
		t.Fatalf("after a restart the chat's conversation is %q", got)
	}
	again.onMessage(nil, textMsg("c1", "ou_me", "second"))
	if len(luna.ran) != 2 || !slices.Contains(luna.ran[1].Command, "ses_A") {
		t.Fatalf("the turn after a restart ran %v, not the conversation before it", luna.ran)
	}
}

// A chat's binding is the chat's, whichever topic a message came in: the
// turn of a second topic goes on with the conversation the first left.
func TestBindingAcrossTopics(t *testing.T) {
	b, luna, _ := newBot(t)
	shortWaits(t)
	b.cfg.Run = settings.FeishuRunCommand
	b.cfg.Agent = "opencode"
	bindChat(b, "g1", binding{Session: "s1"})
	luna.files = []string{
		`{"type":"step_start","sessionID":"ses_A"}`, "",
		`{"type":"step_start","sessionID":"ses_B"}`, "",
	}
	b.onMessage(nil, groupMsgOf("g1", "t1", "m1", "first"))
	b.onMessage(nil, groupMsgOf("g1", "t2", "m2", "second"))
	if len(luna.ran) != 2 || !slices.Contains(luna.ran[1].Command, "ses_A") {
		t.Fatalf("the second topic's turn ran %v, not the chat's conversation", luna.ran)
	}
	if got := b.bindingOf("g1"); got.Conversation != "ses_B" {
		t.Errorf("the chat's conversation is %q, want ses_B", got.Conversation)
	}
}

// A file of before the bindings, with the sessions in binds and what a
// chat went on with in chats, loads as the chat's binding.
func TestLegacyBindingsFile(t *testing.T) {
	b, _, _ := newBot(t)
	old := `{"binds":{"c1":"s1"},"chats":{"c1":{"agent":"opencode","dir":"/tmp/api","sid":"ses_A"}}}`
	if err := os.WriteFile(bindFile(), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	b.load()
	want := binding{Session: "s1", Agent: "opencode", Dir: "/tmp/api", Conversation: "ses_A"}
	if got := b.bindingOf("c1"); got != want {
		t.Fatalf("the old file loads %+v, want %+v", got, want)
	}
	b.save()
	data, err := os.ReadFile(bindFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"bindings"`) || strings.Contains(string(data), `"binds"`) {
		t.Errorf("saved %s", data)
	}
}

// The turns of several chats save at once, and the file they load from
// still holds every binding: one writer at a time writes it.
func TestConcurrentSaves(t *testing.T) {
	b, luna, _ := newBot(t)
	const n = 40
	for i := 0; i < n; i++ {
		bindChat(b, fmt.Sprintf("c%d", i), binding{
			Session:      fmt.Sprintf("s%d", i),
			Agent:        "opencode",
			Conversation: fmt.Sprintf("conv%d", i),
		})
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.save()
		}()
	}
	wg.Wait()
	again := New(Config{Settings: b.cfg, Secret: settings.FeishuSecret{AppID: "cli_a", AppSecret: "x", Me: "ou_me"}, Luna: luna, Log: t.Logf})
	again.load()
	for i := 0; i < n; i++ {
		want := binding{Session: fmt.Sprintf("s%d", i), Agent: "opencode", Conversation: fmt.Sprintf("conv%d", i)}
		if got := again.bindingOf(fmt.Sprintf("c%d", i)); got != want {
			t.Errorf("chat c%d loads %+v, want %+v", i, got, want)
		}
	}
}

// groupMsgOf is a group message in a topic, as Feishu sends one.
func groupMsgOf(chat, root, id, text string) *larkim.P2MessageReceiveV1 {
	e := textMsgFrom(chat, id, text)
	e.Event.Message.ChatType = str("group")
	e.Event.Message.RootId = str(root)
	e.Event.Message.Mentions = []*larkim.MentionEvent{{Key: str("@_user_1"), Id: &larkim.UserId{OpenId: str("ou_bot")}, MentionedType: str("bot")}}
	return e
}

// shortWaits makes the waits the driving relies on short, for the tests.
func shortWaits(t *testing.T) {
	t.Helper()
	// The waits end as the test does, before its cleanups, which wait for
	// the turns that no later test's waits would hold to their own.
	t.Cleanup(func() {
		waits = theWaits
		turnMu.Unlock()
	})
	turnMu.Lock()
	waits.start, waits.settle, waits.quiet, waits.grace, waits.poll = 60*time.Millisecond, 60*time.Millisecond, 150*time.Millisecond, 250*time.Millisecond, 60*time.Millisecond
}

// theWaits are the driving's waits as the program has them.
var theWaits = waits

// turnMu holds through a test that makes the waits short: since those
// outlast it until its cleanups, the turns of the next would wait for the
// one they held here to end.
var turnMu sync.Mutex

// A turn whose agent's screen, drawn as a prompt again, tells nothing of
// its answer is answered from the passage its Stop hook wrote.
func TestPromptAnsweredWhileIdle(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	stuckAfter = 150 * time.Millisecond
	t.Cleanup(func() { stuckAfter = 5 * time.Second })
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	luna.hold = true
	// The turn: the agent takes the prompt, and keeps a status it had
	// before it, working; its answer is written as the pane shows it.
	luna.screen = "❯ 1+1=?\n\n» 2\n\n──────────────────────────────────────\n❯ "
	luna.script = []struct {
		screen string
		status proto.Status
	}{{"old\n", proto.Working}, {"old\n", proto.Working}, {"old\n", proto.Working}, {"old\n", proto.Working}, {"old\n", proto.Working}}
	b.onMessage(nil, textMsg("c1", "ou_me", "1+1=?"))
	if got := last(t, msgs); !strings.Contains(got, "2") || strings.Contains(got, "terminal's") {
		t.Fatalf("replied %q", got)
	}
}

// A pane that draws its screen anew, as a TUI does, keeps a turn's text in
// the rows below its prompt, which the rows of its screen never grow for.
func TestPromptOfAViewport(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	luna.hold = true
	// The screen before the turn, and after it: as many rows either way.
	luna.screen = viewportBefore
	for range 8 {
		luna.script = append(luna.script, struct {
			screen string
			status proto.Status
		}{viewportAfter, proto.Idle})
	}
	b.onMessage(nil, textMsg("c1", "ou_me", "2+2=?"))
	if got := last(t, msgs); !strings.Contains(got, "2+2=4") || strings.Contains(got, "printed nothing new") {
		t.Fatalf("replied %q", got)
	}
}

// A status the daemon keeps across a restart, as one a program reported,
// does not hold a prompt forever.
func TestPromptStuckWorking(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	stuckAfter = 100 * time.Millisecond
	t.Cleanup(func() { stuckAfter = 5 * time.Second })
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	luna.busy = true
	luna.script = []struct {
		screen string
		status proto.Status
	}{{"old\n", proto.Working}, {"old\n", proto.Working}, {"old\n", proto.Working}, {"old\n", proto.Working}, {"old\n", proto.Working}}
	b.onMessage(nil, textMsg("c1", "ou_me", "fix it"))
	if got := last(t, msgs); !strings.Contains(got, "cannot tell") && !strings.Contains(got, "text") {
		t.Fatalf("replied %q", got)
	}
}

// In the command mode, a prompt runs the agent's own command in the
// background, and its text is the reply.
func TestCommandMode(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	b.cfg.Run = settings.FeishuRunCommand
	b.cfg.Agent = "claude"
	b.yolo = true // feishu-yolo's default
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	luna.files = []string{"2", ""}
	b.onMessage(nil, textMsg("c1", "ou_me", "1+1=?"))
	// The card's answer, then the turn's state.
	got := last(t, msgs)
	if !strings.Contains(got, "2") {
		t.Fatalf("replied %q", got)
	}
	if len(luna.ran) != 1 {
		t.Fatalf("ran %+v", luna.ran)
	}
	cmd := strings.Join(luna.ran[0].Command, " ")
	// With yolo on, Claude Code asks no program for a permission: the
	// option that makes each tool ask it alone fails every tool. Its
	// messages are a stream, which holds what the turn used, and its
	// prompt an argument: no -p, whose stdin holds its prompt alone.
	if !strings.HasPrefix(cmd, "claude --dangerously-skip-permissions --session-id ") ||
		!strings.HasSuffix(cmd, " --output-format stream-json --verbose 1+1=?") ||
		strings.Contains(cmd, " -p ") || strings.Contains(cmd, "--input-format") || strings.Contains(cmd, "--permission-prompt-tool") {
		t.Errorf("ran %q", cmd)
	}
	if luna.ran[0].Dir == "" || !filepath.IsAbs(luna.ran[0].Out) {
		t.Errorf("ran in %q, output %q", luna.ran[0].Dir, luna.ran[0].Out)
	}
}

// Each agent's command continues the chat's conversation.
func TestCommandContinues(t *testing.T) {
	b, _, _ := newBot(t)
	first := b.backgroundCommand("opencode", "sid-1", "", "hi", false)
	next := b.backgroundCommand("opencode", "sid-2", "ses_9", "again", false)
	if strings.Join(next, " ") != "opencode run --format json --session ses_9 again" || strings.Join(first, " ") != "opencode run --format json --session ses_sid1 hi" {
		t.Errorf("%q, %q", first, next)
	}
	// A chat's ID is no session of OpenCode's own: its turn makes one
	// that carries its prefix.
	if cmd := b.backgroundCommand("opencode", "", "3f2a-1b", "x", false); !slices.Contains(cmd, "ses_3f2a1b") {
		t.Errorf("opencode %q", cmd)
	}
	next = b.backgroundCommand("codex", "sid-2", "th-1", "again", false)
	if strings.Join(next, " ") != "codex exec --json resume th-1 --skip-git-repo-check again" {
		t.Errorf("codex %q", next)
	}
	if sid := conversation("opencode", `{"type":"step_start","sessionID":"ses_7"}`, "x"); sid != "ses_7" {
		t.Errorf("opencode continues %q", sid)
	}
}

// DSH runs its tasks headless, as JSON events: its final event is the
// answer, and its session event names the conversation it continues.
func TestDSHCommand(t *testing.T) {
	b, _, _ := newBot(t)
	first := b.backgroundCommand("dsh", "1234-5678", "", "hi", false)
	next := b.backgroundCommand("dsh", "", "session-9", "again", false)
	if strings.Join(first, " ") != "dsh headless --json hi" ||
		strings.Join(next, " ") != "dsh headless --json --session-id session-9 again" {
		t.Errorf("%q, %q", first, next)
	}
	// DSH has no yolo option: a turn adds none, whether or not it is on.
	for _, told := range []bool{false, true} {
		if cmd := strings.Join(b.backgroundCommand("dsh", "", "session-9", "go", told), " "); cmd != "dsh headless --json --session-id session-9 go" {
			t.Errorf("yolo %q", cmd)
		}
	}
	// Its command from feishu-agent-command's own option goes with the
	// command, before the subcommand, as its program takes it.
	b.cfg.Agents = map[string]string{"dsh": "dsh --permission-mode read-only"}
	if cmd := strings.Join(b.backgroundCommand("dsh", "s", "", "x", true), " "); cmd != "dsh --permission-mode read-only headless --json x" {
		t.Errorf("written %q", cmd)
	}
	b.cfg.Agents = nil
	sid := conversation("dsh", `{"type":"session","sessionId":"session-1","cwd":"/x"}`+"\n"+
		`{"type":"text","text":"hi"}`+"\n"+
		`{"type":"final","text":"2"}`+"\n", "x")
	if sid != "session-1" {
		t.Errorf("dsh continues %q", sid)
	}
	// The final event is its answer; thinking and tools are not.
	raw := `{"type":"session","sessionId":"session-1"}` + "\n" +
		`{"type":"thinking","text":"hmm"}` + "\n" +
		`{"type":"tool_call","callId":"c","tool":"bash","input":{}}` + "\n" +
		`{"type":"tool_result","callId":"c","status":"completed","result":"…"}` + "\n" +
		`{"type":"text","text":"2"}` + "\n" +
		`{"type":"status","phase":"turn_end","turn":1,"reason":{"kind":"completed"}}` + "\n" +
		`{"type":"final","text":"1+1=2"}` + "\n"
	if got := commandText("dsh", raw); got != "1+1=2" {
		t.Errorf("answered %q", got)
	}
	// A failure before the turn has no final event.
	if got := commandText("dsh", `{"type":"error","message":"session \"x\" does not exist"}`+"\n"); got != `session "x" does not exist` {
		t.Errorf("failed %q", got)
	}
	// A turn that fails in-turn ends with an empty final event.
	failed := `{"type":"status","phase":"turn_end","turn":1,"reason":{"kind":"error","message":"no model"}}` + "\n" +
		`{"type":"final","text":""}` + "\n"
	if got := commandText("dsh", failed); got != "The turn ended: error (no model)" {
		t.Errorf("a failed turn %q", got)
	}
}

// splitCommand cuts the commands, and leaves prompts, and // paths.
func TestSplitCommand(t *testing.T) {
	for _, c := range []struct{ in, cmd, rest string }{
		{"/new api pi", "new", "api pi"},
		{"/New", "new", ""},
		{"/help@hyla-bot please", "help", "please"},
		{"fix /new", "", "fix /new"},
		{"//server/share", "", "//server/share"},
	} {
		if cmd, rest := splitCommand(c.in); cmd != c.cmd || strings.TrimSpace(rest) != c.rest {
			t.Errorf("%q gives %q %q", c.in, cmd, rest)
		}
	}
}

func (f *fakeLuna) hostname() string { return firstNotEmpty(f.host, "mac") }

// cardMsgOf is a message event, as Feishu sends one: its parent, and
// what it says.
func cardMsgOf(chat, id, parent, content string) *larkim.P2MessageReceiveV1 {
	nextEvent++
	return &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{AppID: "cli_a", EventID: fmt.Sprintf("evm_%d_%s", nextEvent, id)}},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender:  &larkim.EventSender{SenderId: &larkim.UserId{OpenId: str("ou_me")}},
			Message: &larkim.EventMessage{MessageId: str(id), ParentId: str(parent), ChatId: str(chat), ChatType: str("p2p"), MessageType: str("text"), Content: str(content)},
		},
	}
}

// replyContent is a reply's text, as Feishu reads it: the text itself.
func replyContent(content string) string {
	var c struct{ Text string }
	json.Unmarshal([]byte(content), &c)
	return c.Text
}

// replyNamed is a reply's content: the event it names, and its text.
func replyNamed(event, text string) string {
	lines := ""
	if !strings.HasPrefix(text, "hyla-") {
		lines = hopText(event)
	}
	content, _ := json.Marshal(map[string]string{"text": lines + text})
	return replyContent(string(content))
}

// Two hosts hear every message: linux answers a message that names
// nothing, and mac, the chat's host, answers linux's answer with the
// prompt. For /host mac, linux answers mac's message before it, and
// leaves the prompt that comes with it to mac.
func TestHostHandoff(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	b.cfg.Host = []string{"mac", "linux"}
	luna.host, b.hostname = "linux", "linux"
	// A message Feishu has: what it says, and the message it answers,
	// whose text Feishu gives the other hosts by its ID and chat alone.
	mac := func() { b.hostname, luna.host = "mac", "mac" }
	linux := func() { b.hostname, luna.host = "linux", "linux" }
	// Feishu has the message the first one answers, and the answer mac's
	// prompt replies to: the first host that hears a message takes it.
	luna.msgs = append(luna.msgs, &larkim.EventMessage{MessageId: str("om_user"), ChatId: str("c1"), ChatType: str("p2p"), MessageType: str("text"), Content: jsonText("ask")})
	luna.msgs = append(luna.msgs, &larkim.EventMessage{MessageId: str("om_sent_1"), ChatId: str("c1"), ChatType: str("p2p"), MessageType: str("text"), Content: jsonText("hi")})
	bindChat(b, "c1", binding{Session: "s1", Agent: "claude", Dir: "/tmp"})
	b.sync = true
	// The turn's text, after what the pane had.
	luna.script = append(luna.script, struct {
		screen string
		status proto.Status
	}{screen: "> "}, struct {
		screen string
		status proto.Status
	}{screen: "> hi\nI fixed the tests.\n> "})
	luna.screen = "> hi\nI fixed the tests.\n> "
	// A message Feishu's JSON fills: its text comes from it, not from
	// the struct a test set.
	linux()
	m := &larkim.P2MessageReceiveV1{}
	json.Unmarshal([]byte(`{"event":{"sender":{"sender_id":{"open_id":"ou_me"}},"message":{"message_id":"om_a","parent_id":"om_user","chat_id":"c1","chat_type":"p2p","message_type":"text","content":"{\"text\":\"hi\"}"}}}`), m)
	b.onMessage(nil, m)

	// The message answers one that names nothing, so linux, which answers
	// prompts, answers it, with its screen. Its text comes from Feishu's
	// JSON, not from the struct a test set, which says "not text".
	if n := len(luna.sent); n != 2 || luna.sent[0].Data != "hi" || luna.sent[1].Data == "hi" {
		t.Fatalf("a message's text: %+v", luna.sent)
	}

	// mac answers linux's answer: a prompt in a reply that names the
	// event, and the chat's host, "" until /host names one.
	mac()
	luna.sent = nil
	b.onMessage(nil, cardMsgOf("c1", "om_b", "om_sent_1", "hi"))
	// The prompt, which Enter ends.
	if !slices.Contains(luna.sent, proto.SendParams{ID: "p1", Data: "hi"}) {
		t.Fatalf("mac sent %+v", luna.sent)
	}
	b.chats["c1"].busy = false
	b.mu.Lock()
	b.chatLocked("c1").host = ""
	b.mu.Unlock()

	// /host mac: the chat's host, which mac is, answers the prompt, and the
	// next message answers its answer.
	mac()
	replies := len(msgs.cards)
	b.onMessage(nil, textMsg("c1", "ou_me", "/host mac"))
	if n := len(msgs.cards); n != replies+1 || !strings.Contains(last(t, msgs), "Messages here go to mac.") {
		t.Fatalf("mac sent %+v", luna.sent)
	}
	if b.chat("c1").host != "mac" {
		t.Fatalf("the chat's host is %q", b.chat("c1").host)
	}
	// linux leaves the message, which answers mac's answer.
	linux()
	n := len(luna.sent)
	b.onMessage(nil, cardMsgOf("c1", "om_b2", "om_sent_2", "run it"))
	if len(luna.sent) != n || luna.created != nil {
		t.Fatalf("linux, not mac, ran it: %+v, created %+v", luna.sent, luna.created)
	}

	// The prompt replies to the host's answer, which names mac: mac runs
	// it, and linux leaves it.
	luna.sent = nil
	luna.created = nil
	b.onMessage(nil, cardMsgOf("c1", "om_linux", "om_sent_3", "/new"))
	if len(luna.sent) != 0 || len(luna.created) != 0 {
		t.Fatalf("linux, not the host, acted: %+v, created %+v", luna.sent, luna.created)
	}
	// mac hears a new event for it: one Feishu sends each host alone, whose
	// ID linux did not see.
	mac()
	ev := textMsg("c1", "ou_me", "/new")
	ev.EventV2Base.Header.EventID = "ev_new"
	luna.created = nil
	b.onMessage(nil, ev)
	if len(luna.created) != 1 {
		t.Fatalf("mac created %+v, sent %+v", luna.created, luna.sent)
	}
}

// A turn shows as one card of the bot's: it says the turn works as it
// starts, and takes the answer as it ends. The user's message keeps its
// text, and takes the work's marks.
func TestCardTurn(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	stuckAfter = 100 * time.Millisecond
	t.Cleanup(func() { stuckAfter = 5 * time.Second })
	b.cfg.Run = settings.FeishuRunCommand
	luna.files = []string{`{"type":"assistant","message":{"content":[{"type":"text","text":"Fixed 2 tests."}]}}`, "", ""}
	bindChat(b, "c1", binding{Session: "s1", Agent: "claude", Dir: "/tmp"})
	b.onMessage(nil, cardMsgOf("c1", "om_t", "om_user", "fix the tests"))
	// One card, sent as the turn starts: the work's, over the prompt.
	if len(msgs.shown) != 1 {
		t.Fatalf("cards sent %d", len(msgs.shown))
	}
	working := parseCard(t, msgs.shown[0])
	if head := working["header"].(map[string]any); head["template"] != "grey" ||
		head["title"].(map[string]any)["content"] != "🔵 Running…" {
		t.Errorf("the turn's card is %v", working)
	}
	// The same card takes the answer: green, with a footer of what the
	// turn used, and the answer's words.
	if len(msgs.updates) != 1 || !strings.HasPrefix(msgs.updates[0], "om_card_1: ") {
		t.Fatalf("the turn's card updates are %v", msgs.updates)
	}
	if !strings.Contains(msgs.updates[0], "[marks [om_t:OnIt]]") {
		t.Errorf("the answer comes as the turn's mark goes: %s", msgs.updates[0][strings.LastIndex(msgs.updates[0], "["):])
	}
	last := parseCard(t, cardOf(msgs.updates[0]))
	if head := last["header"].(map[string]any); head["template"] != "green" ||
		head["title"].(map[string]any)["content"] != "🟢 Fixed 2 tests." {
		t.Errorf("the answer's card is %v", last)
	}
	if !strings.Contains(cardText(last), "Fixed 2 tests.") ||
		!strings.Contains(footerOf(last), "<text_tag color='green'>Done</text_tag>") ||
		!strings.Contains(footerOf(last), "· ") {
		t.Errorf("the answer's card is %v", last)
	}
	// What the turn used, as the footer shows it: the tokens it took,
	// with the cached ones, and gave, and the context.
	u := UsageOf("claude", `{"type":"result","usage":{"input_tokens":4200,"output_tokens":680,"cache_read_input_tokens":12000},`+
		`"modelUsage":{"m":{"contextWindow":200000,"canonicalModel":"claude-sonnet-5-5"}}}`)
	if u.Input != 16200 || u.Output != 680 || u.Cache != 12000 || u.Context != 16880 || u.MaxContext != 200000 ||
		u.Model != "claude-sonnet-5-5" {
		t.Errorf("the usage is %+v", u)
	}
	if got := u.text(); got != "↑16.2k · ↓680 · cache 12k · ctx 16.8k/200k 8%" {
		t.Errorf("the usage reads %q", got)
	}
	if footer := (Card{State: "done", Usage: u, Duration: time.Minute}).footer("en_us"); !strings.HasPrefix(footer, "<text_tag color='green'>Done</text_tag> · `claude-sonnet-5-5` · ↑16.2k") {
		t.Errorf("the footer reads %q", footer)
	}
	if countText(999) != "999" || countText(4240) != "4.2k" || countText(1_500_000) != "1.5m" || countText(200000) != "200k" {
		t.Errorf("the counts are %q %q", countText(4240), countText(1_500_000))
	}
	if durationText(1500*time.Millisecond) != "2s" || durationText(84*time.Second) != "1m24s" || durationText(time.Hour+5*time.Minute+3*time.Second) != "1h5m3s" {
		t.Errorf("the times are %q", durationText(84*time.Second))
	}
	// The message takes the mark as the turn starts, and gives it back as
	// the answer comes.
	if !slices.Equal(msgs.reacts, []string{"om_t:OnIt"}) || !slices.Equal(msgs.unreacts, []string{"om_t:OnIt"}) {
		t.Errorf("the marks are %q, taken back %q", msgs.reacts, msgs.unreacts)
	}
}

// A turn's card shows what the turn prints as it goes, a patch at a time,
// and keeps the same message for its answer.
func TestCardShowsTheTurn(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	defer func(d time.Duration) { cardEvery = d }(cardEvery)
	defer func(d time.Duration) { stuckAfter = d }(stuckAfter)
	cardEvery = time.Millisecond
	stuckAfter = 400 * time.Millisecond
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	// A pane that works for the first look, and prints a row each look
	// after: the bot sees it work, and then idle with a still screen.
	luna.waitEnd = proto.Working
	luna.screen = "old\n$ "
	printed := 0
	luna.onRead = func(string) {
		if printed >= 3 {
			return
		}
		printed++
		luna.screen = "old\n$ " + strings.Repeat("step\n", printed)
	}
	b.onMessage(nil, textMsg("c1", "ou_me", "what changed?"))
	if len(msgs.shown) != 1 {
		t.Fatalf("cards sent %d", len(msgs.shown))
	}
	// One message from the start to the end: the working card's patches,
	// then its answer.
	if len(msgs.updates) < 2 {
		t.Fatalf("the card's updates are %v", msgs.updates)
	}
	for _, u := range msgs.updates {
		if !strings.HasPrefix(u, "om_card_1: ") {
			t.Errorf("an update of another message: %s", u)
		}
	}
	prog := false
	for _, u := range msgs.updates[:len(msgs.updates)-1] {
		if strings.Contains(cardText(parseCard(t, cardOf(u))), "step") {
			prog = true
		}
	}
	if !prog {
		t.Errorf("no patch shows the turn: %v", msgs.updates)
	}
}

// A group's message that @s another bot is no message to this bot, which
// answers only a mention of itself.
func TestAnotherBotsMention(t *testing.T) {
	bot := func(openID, id, text string) *larkim.P2MessageReceiveV1 {
		m := textMsg("g1", "ou_me", text)
		m.Event.Message.MessageId = str(id)
		*m.Event.Message.ChatType = "group"
		m.Event.Message.Mentions = []*larkim.MentionEvent{{Key: str("@_user_1"), Id: &larkim.UserId{OpenId: str(openID)}, MentionedType: str("bot")}}
		return m
	}
	b, luna, msgs := newBot(t)
	shortWaits(t)
	b.cfg.Allow = []string{"ou_me"}
	bindChat(b, "g1", binding{Session: "s1"})
	b.leave("g1")
	// Another bot's mention is the message's, and the chat's session is
	// no part of it.
	b.onMessage(nil, bot("ou_other", "om_other", "@_user_1 read this"))
	if n := len(luna.sent); n != 0 {
		t.Fatalf("the bot ran another bot's mention: %+v", luna.sent)
	}
	if len(msgs.cards) != 0 {
		t.Fatalf("the bot answered another bot's mention: %v", msgs.cards)
	}
	// Its own mention is.
	b.onMessage(nil, bot("ou_bot", "om_own", "@_user_1 read this"))
	if len(luna.sent) == 0 {
		t.Fatal("the bot took no prompt from its own mention")
	}
}

// A prompt in a group's topic shows there: the turn's card goes into the
// topic the prompt came in, and its answer patches that card.
func TestTopicCard(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	b.sync = true
	b.cfg.Allow = []string{"ou_me"}
	bindChat(b, "g1", binding{Session: "s1"})
	b.leave(chatKey("g1", "om_root"))
	luna.screen = "old\n$ "
	luna.waitEnd = proto.Working
	m := textMsg("g1", "ou_me", "fix it")
	*m.Event.Message.ChatType = "group"
	m.Event.Message.MessageId = str("om_msg")
	m.Event.Message.RootId = str("om_root")
	m.Event.Message.Mentions = []*larkim.MentionEvent{{Key: str("@_user_1"), Id: &larkim.UserId{OpenId: str("ou_bot")}, MentionedType: str("bot")}}
	b.onMessage(nil, m)
	if len(msgs.shown) != 1 || len(msgs.topics) != 1 || msgs.topics[0] != "om_root" {
		t.Fatalf("the turn's card went to %v: %v", msgs.topics, msgs.shown)
	}
	// The answer is the topic's card, patched, not one of the chat's own.
	if len(msgs.updates) == 0 || !strings.HasPrefix(msgs.updates[0], "om_card_1: ") {
		t.Fatalf("the turn's updates are %v", msgs.updates)
	}
	if len(msgs.topics) != 1 {
		t.Errorf("the answer is a card of its own: %v", msgs.topics)
	}
}

// A working card shows the tool a background agent runs and what it says,
// and none of the stream of events it says them in.
func TestBackgroundTail(t *testing.T) {
	step := `{"type":"step_start","part":{"type":"step-start"}}`
	tool := `{"type":"tool_use","part":{"type":"tool","tool":"shell"}}`
	said := `{"type":"text","part":{"type":"text","text":"Looking at the docs."}}`
	// The events alone are the tool it runs, and no text.
	if got := backgroundTail("opencode", step+"\n"+tool, "en"); got != "⏳ **shell** · 1 tool" {
		t.Errorf("the events read %q", got)
	}
	// The words it says, under the tool.
	got := backgroundTail("opencode", step+"\n"+tool+"\n"+said, "en")
	if !strings.HasPrefix(got, "⏳ **shell** · 1 tool") || !strings.Contains(got, "Looking at the docs.") {
		t.Errorf("the turn reads %q", got)
	}
	// The words alone, before a tool runs.
	if got := backgroundTail("opencode", step+"\n"+said, "en"); strings.Contains(got, "⏳") || !strings.Contains(got, "Looking at the docs.") {
		t.Errorf("the words read %q", got)
	}
	// An output of no events is its own text, as a command's is.
	if got := backgroundTail("dsh", "line one\nline two", "en"); got != "line one\nline two" {
		t.Errorf("a plain output reads %q", got)
	}
}

// A background turn shows as its own card, whichever agent runs it: the
// card says the turn runs as it starts, and takes the answer as it ends.
func TestCommandModeCard(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	b.cfg.Run = settings.FeishuRunCommand
	b.cfg.Agent = "opencode"
	bindChat(b, "c1", binding{Session: "s1", Agent: "opencode"})
	b.leave("c1")
	luna.files = []string{`{"type":"text","part":{"type":"text","text":"Pong"}}`, ""}
	b.onMessage(nil, textMsg("c1", "ou_me", "ping"))
	if len(msgs.shown) != 1 {
		t.Fatalf("cards sent %d", len(msgs.shown))
	}
	working := parseCard(t, msgs.shown[0])
	if head := working["header"].(map[string]any); head["template"] != "grey" ||
		head["title"].(map[string]any)["content"] != "🔵 Running…" {
		t.Errorf("the turn's card is %v", working)
	}
	// Feishu takes no card without a body: the card says the agent
	// thinks until it has words of its own.
	if working["body"] == nil || !strings.Contains(cardText(working), "Thinking") {
		t.Errorf("the turn's card has no body: %v", working)
	}
	last := parseCard(t, msgs.cards[len(msgs.cards)-1])
	if last["header"].(map[string]any)["template"] != "green" || !strings.Contains(cardText(last), "Pong") {
		t.Errorf("the answer's card is %v", last)
	}
}

// A card Feishu refuses to patch into its answer leaves the answer to a
// card of its own, which the turn's message stays as it was: Feishu
// refuses the patches of a message it takes for too frequent (230020).
func TestCardPatchRefused(t *testing.T) {
	b, luna, msgs := newBot(t)
	shortWaits(t)
	defer func(d time.Duration) { cardRetry = d }(cardRetry)
	cardRetry = time.Millisecond
	b.cfg.Run = settings.FeishuRunCommand
	b.cfg.Agent = "claude"
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	luna.files = []string{"2", ""} // the command's text, then its end
	msgs.patchFail = true
	b.onMessage(nil, textMsg("c1", "ou_me", "1+1=?"))
	if len(msgs.updates) != 0 {
		t.Errorf("Feishu took the patches it refuses: %v", msgs.updates)
	}
	if len(msgs.shown) != 2 {
		t.Fatalf("cards sent %d: %v", len(msgs.shown), msgs.shown)
	}
	last := parseCard(t, msgs.shown[len(msgs.shown)-1])
	if last["header"].(map[string]any)["template"] != "green" || !strings.Contains(cardText(last), "2") {
		t.Errorf("the answer's card is %v", last)
	}
}

// footerOf is a card's footer: its last line of notation.
func footerOf(c map[string]any) string {
	body, _ := c["body"].(map[string]any)
	elems, _ := body["elements"].([]any)
	for i := len(elems) - 1; i >= 0; i-- {
		m, _ := elems[i].(map[string]any)
		if m["text_size"] == "notation" {
			s, _ := m["content"].(string)
			return s
		}
	}
	return ""
}

// cardOf is an update's card's JSON, without the marks it notes.
func cardOf(update string) string {
	_, rest, _ := strings.Cut(update, ": ")
	return rest[:strings.Index(rest, " [marks")]
}

// parseCard is a card's JSON, as a map.
func parseCard(t *testing.T, shown string) map[string]any {
	t.Helper()
	var c map[string]any
	if err := json.Unmarshal([]byte(shown), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// An image goes to the session by its saved path.
func TestImage(t *testing.T) {
	t.Setenv("HYLA_HOME", t.TempDir())
	b, luna, msgs := newBot(t)
	bindChat(b, "c1", binding{Session: "s1"})
	b.leave("c1")
	luna.waitEnd = proto.Working
	b.onMessage(nil, &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{AppID: "cli_a", EventID: "ev_img"}},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender:  &larkim.EventSender{SenderId: &larkim.UserId{OpenId: str("ou_me")}},
			Message: &larkim.EventMessage{ChatId: str("c1"), ChatType: str("p2p"), MessageType: str("image"), Content: str(`{"image_key":"img_v3_123"}`)},
		},
	})
	if len(luna.sent) == 0 || !strings.Contains(luna.sent[0].Data, "feishu-files") || !strings.Contains(luna.sent[0].Data, ".png") {
		t.Fatalf("sent %+v, replied %q", luna.sent, last(t, msgs))
	}
}

// The button of a card opens its session in Hyla, its end in the app.
func TestCardButton(t *testing.T) {
	b, luna, _ := newBot(t)
	luna.msgs = []*larkim.EventMessage{{MessageId: str("om_c"), ChatId: str("c1"), MessageType: str("interactive")}}
	b.remember("om_c", record{Chat: "c1", Session: "s1"})
	res, err := b.onCardAction(nil, &callback.CardActionTriggerEvent{Event: &callback.CardActionTriggerRequest{
		Operator: &callback.Operator{OpenID: "ou_me"},
		Action:   &callback.CallBackAction{Value: map[string]any{"hyla": "open"}},
		Context:  &callback.Context{OpenMessageID: "om_c"},
	}})
	if err != nil || res == nil || res.Toast == nil || res.Toast.Type != "success" {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if !slices.Contains(luna.order, "select") {
		t.Fatalf("the click sent %v", luna.order)
	}
	// Feishu rejects schema 2.0 cards with an action element: the open
	// button is an element of the body.
	c := Card{Title: "t", Session: "s1", State: "done"}
	if body, err := cardJSON(c, "en"); err != nil || strings.Contains(body, `"action"`) || !strings.Contains(body, `"hyla":"open"`) {
		t.Errorf("card %s, %v", body, err)
	}
	// The record of a card's session stays in hylad's directory.
	b2, _, _ := newBot(t)
	b2.remember("om_c", record{Chat: "c1", Session: "s1"})
	b2.loadNotes()
	if r, ok := b2.note("om_c"); !ok || r.Session != "s1" {
		t.Fatalf("note %+v", r)
	}
}

// A turn whose card Feishu refuses answers outside it: with cards on, an
// answer's card; with them off, a message. Its answer is not lost.
func TestCardRefused(t *testing.T) {
	// The waits are the first iteration's alone: the second, in the same
	// test, would wait for their end.
	set := false
	for _, cards := range []bool{true, false} {
		t.Setenv("HYLA_CONFIG", filepath.Join(t.TempDir(), "config"))
		os.WriteFile(os.Getenv("HYLA_CONFIG"), []byte(fmt.Sprintf("feishu-cards = %v\n", cards)), 0o600)
		settings.Reload()
		defer settings.Reload()
		b, luna, msgs := newBot(t)
		if !set {
			shortWaits(t)
			set = true
		}
		msgs.fail = true
		b.sync = true
		b.cfg.Run = settings.FeishuRunCommand
		b.cfg.Agent = "claude"
		bindChat(b, "c1", binding{Session: "s1"})
		b.leave("c1")
		luna.files = []string{"2", ""} // the command's text, then its end
		b.onMessage(nil, textMsg("c1", "ou_me", "1+1=?"))
		answer := func(s []string) bool {
			return slices.ContainsFunc(s, func(x string) bool { return strings.Contains(x, "2") })
		}
		if !answer(msgs.replies) && !answer(msgs.cards) {
			t.Errorf("cards %v: answered %q %q", cards, msgs.replies, msgs.cards)
		}
	}
}

// A rich text's text and images.
func TestPost(t *testing.T) {
	content := `{"zh_cn":{"title":"Look","content":[[{"tag":"text","text":"see "},{"tag":"img","image_key":"img_1"}]]}}`
	if got := postText(content); !strings.HasPrefix(got, "Look\nsee") {
		t.Errorf("post text %q", got)
	}
	_, images := messageFiles(content)
	if len(images) != 1 || images[0].key != "img_1" {
		t.Errorf("images %+v", images)
	}
}

// hold is a call the messages take a moment to answer, as Feishu's do:
// the bot's work is not all done before it replies.

// writeAnswerLater writes an answer, as a hook that takes a moment does.
func writeAnswerLater(dir, conversation, text string) {
	go func() { writeAnswer(dir, conversation, text) }()
}

// writeAnswer writes an answer as an agent's hook does.
func writeAnswer(dir, conversation, text string) {
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, conversation+".json"), []byte(text), 0o600)
}

// answerFile is an answer an agent's hook writes, as hyla answer does:
// the conversation's first.
func answerFile(t *testing.T, dir, conversation, text string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, conversation+".json")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The agent's hook writes each turn's answer to the file the prompt gives
// its pane, and the reply is that answer, not the terminal's frames.

// An answer to the agent's question ends the turn: the hook writes its
// answer there, and the reply is that answer.
func TestHookAnswerToQuestion(t *testing.T) {
	b, luna, msgs := newBot(t)
	b.bind("c1", session())
	setup := luna.sessions[0]
	// As hylad does, the session's fields are its lead pane's too.
	setup.Agent, setup.Status, setup.Message = "claude", proto.Waiting, "Which branch?"
	setup.Panes[0] = setup.Pane
	luna.sessions[0] = setup
	shortWaits(t)
	dir := answerDir("claude", "p1")
	defer os.RemoveAll(dir)
	luna.screen = "Which branch?\n"
	luna.waitEnd = proto.Idle
	luna.onRead = func(id string) {
		if luna.answers != nil {
			// The hook writes once the bot gave the pane the directory.
			writeAnswerLater(dir, "turn2", "On main.\n")
			luna.onRead = nil
		}
	}
	luna.calls["send"] = func() {
		time.Sleep(20 * time.Millisecond)
	}
	// The prompt that asked it is the turn the answer continues.
	b.beginTurn("c1", &turnInfo{start: time.Now(), chat: "c1", text: "Which branch?", event: "om_question"})
	defer func() {
		b.mu.Lock()
		c := b.chatLocked("c1")
		defer b.mu.Unlock()
		if c.running.text != "" {
			t.Errorf("the turn still runs: %+v", c.running)
		}
	}()

	b.onMessage(nil, textMsg("c1", "ou_me", "/answer main"))

	// The bot gives the pane the turn's answer directory, as the turn
	// began, and follows the answer under it.
	for _, d := range luna.answers {
		if d != dir {
			t.Fatalf("answered in %q, not %q", luna.answers, dir)
		}
	}
	sent := luna.sent
	// The chat typed the answer, not drive, and Enter: the pane's hook
	// names the turn's answer.
	if len(sent) != 1 || sent[0].Data != "" || len(luna.answers) != 1 {
		t.Fatalf("sent %+v", sent)
	}
	got := last(t, msgs)
	if !strings.Contains(got, "On main.") || strings.Contains(got, "Which branch?") && strings.Contains(got, "no final") {
		t.Fatalf("replied %q", got)
	}
	if b.bindingOf("c1").Conversation != "turn2" {
		t.Errorf("the conversation is %q", b.bindingOf("c1").Conversation)
	}
}

// A prompt whose agent's Stop hook writes the turn's answer is replied
// with that answer, beside the terminal's rows, and the next message
// continues the hook's conversation.
func TestHookAnswerToPrompt(t *testing.T) {
	b, luna, msgs := newBot(t)
	b.bind("c1", session())
	setup := luna.sessions[0]
	setup.Agent = "claude"
	setup.Panes[0] = setup.Pane
	luna.sessions[0] = setup
	shortWaits(t)
	dir := answerDir("claude", "p1")
	defer os.RemoveAll(dir)
	luna.screen = "$ "
	// The hook writes as the agent stops, after the turn's rows.
	luna.waitEnd = proto.Idle
	luna.calls["read"] = func() {
		if luna.answers != nil {
			writeAnswer(dir, "turn3", "All tests pass.\n")
		}
	}

	b.onMessage(nil, textMsg("c1", "ou_me", "Run the tests"))
	if len(luna.answers) != 1 || luna.answers[0] != dir {
		t.Fatalf("answered in %q", luna.answers)
	}
	got := last(t, msgs)
	if !strings.Contains(got, "All tests pass.") {
		t.Fatalf("replied %q", got)
	}
	if b.bindingOf("c1").Conversation != "turn3" {
		t.Errorf("the conversation is %q", b.bindingOf("c1").Conversation)
	}
}

// With feishu-yolo on, which is the default, the sessions from Feishu
// take the agent's yolo option, and the background turns, after the
// agent's subcommand.
func TestYoloCommands(t *testing.T) {
	b, _, _ := newBot(t)
	b.yolo = true
	if cmd, after := b.sessionCommand("codex"); strings.Join(cmd, " ") != "codex --dangerously-bypass-approvals-and-sandbox" || !after {
		t.Errorf("codex starts %q, after %v", cmd, after)
	}
	if cmd := b.backgroundCommand("codex", "sid", "", "hi", true); !slices.Contains(cmd, "--dangerously-bypass-approvals-and-sandbox") {
		t.Errorf("codex runs %q", cmd)
	}
	if cmd := b.backgroundCommand("opencode", "sid", "", "hi", false); slices.Contains(cmd, "--auto") {
		t.Errorf("opencode runs %q, with its chat's session without the option", cmd)
	}
	b.cfg.Agents = map[string]string{"pi": "pi --model fast"}
	if cmd, _ := b.sessionCommand("pi"); strings.Join(cmd, " ") != "pi --yolo --model fast" {
		t.Errorf("pi starts %q", cmd)
	}
	// An agent's own setting turns the agent's off.
	b.cfg.Yolo = map[string]bool{"claude": false}
	if cmd, _ := b.sessionCommand("claude"); strings.Join(cmd, " ") != "claude" {
		t.Errorf("claude starts %q", cmd)
	}
	if b.yoloFor("claude") {
		t.Error("claude's turns ask the chat for permissions with its yolo option off")
	}
}

// An OpenCode turn's footer shows its tokens, and the context it took,
// which the agent does not size.
func TestUsageOfOpenCode(t *testing.T) {
	raw := `{"type":"step_finish","timestamp":3,"sessionID":"ses_1","part":{"type":"step-finish",` +
		`"tokens":{"input":21500,"output":469,"reasoning":0,"cache":{"read":39900,"write":0}}}}`
	u := UsageOf("opencode", raw)
	if u.Input != 61400 || u.Output != 469 || u.Cache != 39900 || u.Context != 61869 || u.MaxContext != 0 {
		t.Errorf("the usage is %+v", u)
	}
	if got := u.text(); got != "↑61.4k · ↓469 · cache 39.9k · ctx 61.8k" {
		t.Errorf("the usage reads %q", got)
	}
}

// A Codex turn's footer shows what its completed event names.
func TestUsageOfCodex(t *testing.T) {
	raw := `{"type":"turn.completed","usage":{"input_tokens":4200,"output_tokens":680,"cached_input_tokens":1200}}`
	u := UsageOf("codex", raw)
	if u.Input != 4200 || u.Output != 680 || u.Cache != 1200 || u.Context != 4880 {
		t.Errorf("the usage is %+v", u)
	}
	if got := u.text(); got != "↑4.2k · ↓680 · cache 1.2k · ctx 4.8k" {
		t.Errorf("the usage reads %q", got)
	}
	if footer := (Card{State: "done", Usage: u, Duration: 16 * time.Second}).footer("en_us"); !strings.Contains(footer, "↑4.2k") {
		t.Errorf("the footer reads %q", footer)
	}
}

// A Pi turn's footer shows its model, tokens and context.
func TestUsageOfPi(t *testing.T) {
	raw := `{"type":"message_end","message":{"role":"assistant","model":"claude-sonnet-4-5",` +
		`"usage":{"input":4200,"output":680,"cacheRead":12000,"cacheWrite":300,` +
		`"totalTokens":17180,"cost":{"total":0.1}}}}`
	u := UsageOf("pi", raw)
	if u.Model != "claude-sonnet-4-5" || u.Input != 16500 || u.Output != 680 || u.Cache != 12000 || u.Context != 17180 {
		t.Errorf("the usage is %+v", u)
	}
	if got := u.text(); got != "↑16.5k · ↓680 · cache 12k · ctx 17.1k" {
		t.Errorf("the usage reads %q", got)
	}
}

// A Gemini turn's footer shows the model its stats name.
func TestUsageOfGemini(t *testing.T) {
	raw := `{"response":"Hello.","stats":{"models":{"gemini-2.5-pro":{"tokens":` +
		`{"input":1000,"candidates":200,"cached":400}}}}}`
	u := UsageOf("gemini", raw)
	if u.Model != "gemini-2.5-pro" || u.Input != 1000 || u.Output != 200 || u.Cache != 400 || u.Context != 1200 {
		t.Errorf("the usage is %+v", u)
	}
	if got := u.text(); got != "↑1k · ↓200 · cache 400 · ctx 1.2k" {
		t.Errorf("the usage reads %q", got)
	}
}
