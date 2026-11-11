// Package feishu connects a Feishu app to hylad's sessions: a message to
// the bot, or to it in a group, goes to the coding agent in the chat's
// session, and the end of the pane's screen comes back to the chat
// (docs/feishu.md).
package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"hyla/internal/proto"
	"hyla/internal/settings"
)

// Luna is the API: what the bridge asks of hylad, which inside hylad is
// hylad itself (the bridge's Local), and larkAPI of Feishu, so that a
// test takes a fake of each.
type Luna interface {
	// State is every session, as the "state" method gives it.
	State() (proto.State, error)
	// Call runs a control method, as a client's, decoding into out.
	Call(method string, params, out any) error
	// Kept is a message as Feishu has it, which the hosts of a shared app
	// hear.
	Kept(id string) (proto.FeishuMessage, error)
}

// hylad is the API of the hylad that runs the bot, for what every host
// of a shared app shares: Run sets it, and a test's bot keeps what it
// asks of the fake.
var hylad Luna

// clientCall is the part of client.Client a Luna is built from.
type clientCall interface {
	Call(method string, params, out any) error
}

// Remote is a Luna over a client's connection.
type Remote struct{ C clientCall }

type myLuna = Luna

func (r Remote) State() (proto.State, error) {
	var st proto.State
	err := r.C.Call("state", nil, &st)
	return st, err
}

func (r Remote) Call(method string, params, out any) error { return r.C.Call(method, params, out) }

func (r Remote) Kept(id string) (proto.FeishuMessage, error) {
	var res proto.FeishuMessage
	err := r.C.Call("feishu.message", proto.IDParams{ID: id}, &res)
	return res, err
}

// Messages is what Feishu does for a chat.
type Messages interface {
	// Send sends text to a chat, or to a person by open_id, and returns
	// the message's ID.
	Send(receiveIDType, receiveID, text string) (string, error)
	// SendCard sends a card's JSON to a chat, and returns the message's
	// ID.
	SendCard(receiveIDType, receiveID, cardJSON string) (string, error)
	// SendCardReply sends a card's JSON as an answer to a message, in the
	// topic the message is in, and returns the card's message ID.
	SendCardReply(messageID, cardJSON string) (string, error)
	// UpdateCard replaces the card of a message the bot sent.
	UpdateCard(messageID, cardJSON string) error
	// DeleteMessage takes back a message the bot sent, as the card of a
	// prompt that waited its turn.
	DeleteMessage(messageID string) error
	// React marks a message with an emoji, by its Feishu name, as
	// "OnIt".
	React(messageID, emoji string) error
	// Unreact takes back a reaction that React gave, which a message
	// shows one of each app's at a time.
	Unreact(messageID, emoji string) error
	// Download is a message's file or image, by its file_key or
	// image_key.
	Download(key string, image bool) ([]byte, error)
	// SendReply sends text as an answer to a message, and returns the
	// answer's ID.
	SendReply(messageID, text string) (string, error)
	// Get is a message by its ID.
	Get(messageID string) (feishuMessage, error)
}

// feishuMessage is a message an event names.
type feishuMessage struct {
	MessageId   *string `json:"message_id,omitempty"`
	ChatId      *string `json:"chat_id,omitempty"`
	ChatType    *string `json:"chat_type,omitempty"`
	MessageType *string `json:"msg_type,omitempty"`
	Content     *string `json:"content,omitempty"`
	ParentId    *string `json:"parent_id,omitempty"`
	RootId      *string `json:"root_id,omitempty"`
}

func (a *larkAPI) Get(messageID string) (feishuMessage, error) {
	var m feishuMessage
	res, err := a.c.Im.Message.Get(context.Background(),
		larkim.NewGetMessageReqBuilder().MessageId(messageID).Build())
	if err != nil {
		return m, err
	}
	if !res.Success() {
		return m, fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	if res.Data == nil || len(res.Data.Items) == 0 || res.Data.Items[0] == nil {
		return m, errors.New("Feishu returned no message")
	}
	it := res.Data.Items[0]
	m.MessageId, m.ChatId, m.MessageType = it.MessageId, it.ChatId, it.MsgType
	if it.Body != nil {
		m.Content = it.Body.Content
	}
	m.ParentId, m.RootId = it.ParentId, it.RootId
	return m, nil
}

// Bot is one machine's connection to a Feishu app.
type Bot struct {
	cfg settings.Feishu
	// yolo is feishu-yolo, the configuration's switch for every agent's
	// yolo mode, which the agent's own settings set against.
	yolo  bool
	env   []string
	api   myLuna
	lark  *lark.Client
	ws    *larkws.Client
	msgs  Messages
	files Files  // where the messages' files go, nil in a test that takes none
	me    string // the open_id of the one who ran hyla feishu setup
	logf  func(format string, args ...any)
	sync  bool // turns run in the handler's goroutine, for tests
	// began is when this bot started: the turns that ran before it are
	// the ones it takes on.
	began time.Time

	mu sync.Mutex
	// saveMu guards the file the bindings go to: the turns of several
	// chats save at once, and one writer at a time writes it.
	saveMu sync.Mutex
	chats  map[string]*chat // by chat key: the turns' own state
	// bindings is what each chat's turns go on with, by chat key: the
	// session, the agent, the directory its commands run in, and the
	// conversation it continues. It is the only source of them;
	// feishu-chats.json keeps a copy, which load replaces the first with,
	// once, as the bot starts.
	bindings map[string]binding
	seen     map[string]bool // event IDs, for a while
	order    []string        // seen's, oldest first
	// notes tell what the bot's cards are about, by message ID, for the
	// button's click: hylad's directory keeps them across its restarts.
	notes map[string]record
	// heard tells, by event and host, when that host's answer came, as
	// Unix milliseconds.
	heard    map[string]int64
	hostname string
	// notice tells a chat that it has no host to take its messages.
	notice func(chatID, text string)
	// answers tells the messages this bot answered, which the hosts of
	// the app hear and this one hears again.
	answered map[string]bool
	// wants is the host the chat's answer names, and claims the event it
	// answers.
	wants  map[string]string
	claims map[string]string
	// noticeSend is where a message that names an event goes: a chat's,
	// in place of an answer.
	noticeSend func(receiveIDType, receiveID, text string) (string, error)
	// times are the chats' turns, for a chat window, and pins the chat
	// each session's window sends to, nil in a test that has none.
	times map[string]turnTimes
	pins  map[string]string
	// Chats reads the chats the bot is in, for a chat window's.
	Chats func(ctx context.Context) ([]proto.FeishuChat, error)
}

// binding is what a chat's turns go on with, and the only source of it:
// the Hyla session the chat is bound to, the agent its turns run, the
// directory their commands run in, and the conversation the agent names,
// which the next turn continues. Empty means "not known", never "none":
// a turn that learns nothing leaves the field as it was.
type binding struct {
	Session      string `json:"session,omitempty"`
	Agent        string `json:"agent,omitempty"`
	Dir          string `json:"dir,omitempty"`
	Conversation string `json:"conversation,omitempty"`
}

// chat is a Feishu chat, bound to a session as its messages say.
type chat struct {
	chat  string // its chat ID, which its answers go to
	event string // the event its answer names
	key   string // the chat's key
	busy  bool   // a turn runs: the bot takes the next one after it
	// running is the turn the chat runs, as its message left it.
	running turnInfo
	// peer is the open_id of the one the chat is with, for a p2p chat:
	// a window's prompt reaches it as the user's, which the bot answers.
	peer string

	// host is the machine the chat's messages go to, set by /host: "" for
	// the one that hears them first, as hylad's directory keeps it.
	host string
	// allowed are the tools the chat allowed for the session, which its
	// next turns run without asking.
	allowed map[string]bool
	// waiting are the prompts that came while a turn ran: the next one
	// waits, and the queue's size holds the rest back.
	waiting []waitingTurn
	// held are the waiting prompts' cards, by message ID, which the
	// turns they wait for take back.
	held []string
}

// waitingTurn is a prompt that came while the chat's turn ran, and the
// message it came in.
type waitingTurn struct {
	// key is the chat struct it waits in: a group's thread has one of
	// its own, and a stop must find it there.
	key    string
	chatID string
	openID string
	group  bool
	text   string
	m      *larkim.EventMessage
}

// turnInfo is the turn that runs in a chat: when it started, its
// message, and the event its end answers. A turn carries its own, so that
// the next one, or this bot's take on a turn of a restart, cannot change
// it under it.
type turnInfo struct {
	start time.Time
	chat  string
	text  string
	event string
	// asked is the answer a follow-up message answers, which the agent's
	// answer of the turn it continues may come under.
	asked string
	// pane is the pane an answer of the chat's went to, and start is
	// zero: the turn is the answer's, and the prompt's, whose text the
	// answer ends.
	pane string
	// window is a turn a chat window's prompt started, which the app's
	// chat window shows.
	window bool
}

// Config is what a bot needs.
type Config struct {
	Settings settings.Feishu
	Secret   settings.FeishuSecret
	Luna     Luna
	// Yolo is feishu-yolo, the configuration's switch for every agent's
	// yolo mode, which the agent's own settings set against.
	Yolo bool
	Log  func(format string, args ...any)
	// Env is the environment the app runs in, which the commands it runs
	// take: hylad's, which runs them, may be a launchd job's.
	Env []string
}

// New makes the bot; Run holds its connection.
func New(c Config) *Bot {
	if c.Log == nil {
		c.Log = func(string, ...any) {}
	}
	b := &Bot{
		cfg:      c.Settings,
		yolo:     c.Yolo,
		env:      c.Env,
		api:      c.Luna,
		me:       c.Secret.Me,
		logf:     c.Log,
		chats:    map[string]*chat{},
		bindings: map[string]binding{},
		seen:     map[string]bool{},
		notes:    map[string]record{},
		heard:    map[string]int64{},
		wants:    map[string]string{},
		claims:   map[string]string{},
		times:    map[string]turnTimes{},
		pins:     map[string]string{},
		answered: map[string]bool{},
		began:    time.Now(),
	}
	b.lark = lark.NewClient(c.Secret.AppID, c.Secret.AppSecret, lark.WithLogLevel(larkcore.LogLevelError))
	b.msgs = &larkAPI{c: b.lark}
	b.Chats = larkChats{c: b.lark}.Chats
	b.files = b.msgs
	b.noticeSend = b.msgs.Send
	event := larkdispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(b.onMessage).OnP2CardActionTrigger(b.onCardAction)
	b.ws = larkws.NewClient(c.Secret.AppID, c.Secret.AppSecret,
		larkws.WithEventHandler(event),
		larkws.WithLogLevel(larkcore.LogLevelError),
		larkws.WithOnError(func(err error) { b.logf("feishu: %v", err) }),
		larkws.WithOnDisconnected(func() { b.logf("feishu: disconnected") }),
		larkws.WithOnReconnecting(func() { b.logf("feishu: reconnecting") }),
		larkws.WithOnReconnected(func() { b.logf("feishu: reconnected") }))
	return b
}

// Run holds the connection until ctx ends, reconnecting as Feishu asks.
func (b *Bot) Run(ctx context.Context) error {
	hylad = b.api
	b.load()
	return b.ws.Start(ctx)
}

// Files downloads the messages' files from Feishu.
type Files interface {
	Download(key string, image bool) ([]byte, error)
	// SendReply sends text as an answer to a message, and returns the
	// answer's ID.
	SendReply(messageID, text string) (string, error)
}

// larkAPI sends to Feishu.
type larkAPI struct{ c *lark.Client }

func (a *larkAPI) Send(receiveIDType, receiveID, text string) (string, error) {
	content, _ := json.Marshal(map[string]string{"text": text})
	res, err := a.c.Im.Message.Create(context.Background(),
		larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(receiveIDType).
			Body(larkim.NewCreateMessageReqBodyBuilder().ReceiveId(receiveID).MsgType("text").Content(string(content)).Build()).
			Build())
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	if res.Data == nil || res.Data.MessageId == nil {
		return "", errors.New("Feishu returned no message")
	}
	return *res.Data.MessageId, nil
}

func (a *larkAPI) SendCard(receiveIDType, receiveID, cardJSON string) (string, error) {
	res, err := a.c.Im.Message.Create(context.Background(),
		larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(receiveIDType).
			Body(larkim.NewCreateMessageReqBodyBuilder().ReceiveId(receiveID).MsgType("interactive").Content(cardJSON).Build()).
			Build())
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	if res.Data == nil || res.Data.MessageId == nil {
		return "", errors.New("Feishu returned no message")
	}
	return *res.Data.MessageId, nil
}

func (a *larkAPI) DeleteMessage(messageID string) error {
	res, err := a.c.Im.Message.Delete(context.Background(),
		larkim.NewDeleteMessageReqBuilder().MessageId(messageID).Build())
	if err != nil {
		return err
	}
	if !res.Success() {
		return fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	return nil
}

func (a *larkAPI) UpdateCard(messageID, cardJSON string) error {
	res, err := a.c.Im.Message.Patch(context.Background(),
		larkim.NewPatchMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewPatchMessageReqBodyBuilder().Content(cardJSON).Build()).
			Build())
	if err != nil {
		return err
	}
	if !res.Success() {
		return fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	return nil
}

func (a *larkAPI) React(messageID, emoji string) error {
	res, err := a.c.Im.MessageReaction.Create(context.Background(),
		larkim.NewCreateMessageReactionReqBuilder().
			MessageId(messageID).
			Body(larkim.NewCreateMessageReactionReqBodyBuilder().
				ReactionType(larkim.NewEmojiBuilder().EmojiType(emoji).Build()).
				Build()).
			Build())
	if err != nil {
		return err
	}
	if !res.Success() {
		return fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	return nil
}

func (a *larkAPI) Unreact(messageID, emoji string) error {
	list, err := a.c.Im.MessageReaction.List(context.Background(),
		larkim.NewListMessageReactionReqBuilder().MessageId(messageID).ReactionType(emoji).Build())
	if err != nil {
		return err
	}
	if !list.Success() {
		return fmt.Errorf("Feishu answered %d %s", list.Code, list.Msg)
	}
	if list.Data == nil {
		return nil
	}
	for _, r := range list.Data.Items {
		if r == nil || r.ReactionId == nil {
			continue
		}
		res, err := a.c.Im.MessageReaction.Delete(context.Background(),
			larkim.NewDeleteMessageReactionReqBuilder().MessageId(messageID).ReactionId(*r.ReactionId).Build())
		if err != nil {
			return err
		}
		if !res.Success() {
			return fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
		}
	}
	return nil
}

func (a *larkAPI) Download(key string, image bool) ([]byte, error) {
	typ := "file"
	if image {
		typ = "image"
	}
	res, err := a.c.Im.MessageResource.Get(context.Background(),
		larkim.NewGetMessageResourceReqBuilder().MessageId(key).FileKey(key).Type(typ).Build())
	if err != nil {
		return nil, err
	}
	if !res.Success() {
		return nil, fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	return io.ReadAll(io.LimitReader(res.File, 64<<20))
}

// SendCardReply sends a card into the topic of the message it answers: a
// group's thread is a topic, and a card in the chat alone would leave it.
func (a *larkAPI) SendCardReply(messageID, cardJSON string) (string, error) {
	res, err := a.c.Im.Message.Reply(context.Background(),
		larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType("interactive").
				Content(cardJSON).
				ReplyInThread(true).
				Build()).
			Build())
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	if res.Data == nil || res.Data.MessageId == nil {
		return "", errors.New("Feishu returned no message")
	}
	return *res.Data.MessageId, nil
}

func (a *larkAPI) SendReply(messageID, text string) (string, error) {
	content, _ := json.Marshal(map[string]string{"text": text})
	res, err := a.c.Im.Message.Reply(context.Background(),
		larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().MsgType("text").Content(string(content)).Build()).
			Build())
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	if res.Data == nil || res.Data.MessageId == nil {
		return "", errors.New("Feishu returned no message")
	}
	return *res.Data.MessageId, nil
}

// Probe takes a tenant access token for an app, telling a wrong app ID or
// secret. The rest, as the bot capability and scopes, shows when the
// connection and the first reply fail, and the log says why.
func Probe(appID, appSecret string) error {
	body, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	res, err := http.Post(lark.FeishuBaseUrl+"/open-apis/auth/v3/tenant_access_token/internal",
		"application/json; charset=utf-8", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("Feishu is unreachable: %w", err)
	}
	defer res.Body.Close()
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return fmt.Errorf("Feishu answered %s", res.Status)
	}
	if out.Code != 0 {
		return fmt.Errorf("Feishu refused the app: %d %s (check the App ID and App Secret)", out.Code, out.Msg)
	}
	return nil
}

// SendText sends text to a chat (receive_id_type chat_id) or a person
// (open_id), as hyla feishu send does.
func SendText(appID, appSecret, receiveIDType, receiveID, text string) error {
	c := lark.NewClient(appID, appSecret, lark.WithLogLevel(larkcore.LogLevelError))
	_, err := (&larkAPI{c: c}).Send(receiveIDType, receiveID, text)
	return err
}
