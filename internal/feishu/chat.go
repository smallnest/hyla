package feishu

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"

	"hyla/internal/proto"
	"hyla/internal/settings"
)

// A turn shows in a chat as one card of the bot's (feishu-cards): it says
// the turn runs, shows what the turn printed as it goes, and takes the
// answer as it ends. The user's message keeps its text, and the reaction
// that marks the turn while it runs.

// turnCard is a turn's card: the message the bot sent for the turn, which
// its updates patch.
type turnCard struct {
	key    string
	chatID string
	thread string // the topic of the turn, as its root message: "" for the chat's own
	msg    string // the card's message, which its updates patch
	lang   string // the language of the words the card shows
	state  string // working, done, asks or error
	text   string // the card's body, as Feishu took it
}

// newTurnCard makes a turn's card for a chat, which the turn's start
// opens, and which its answer ends: key is the chat's, a group's topic
// one of its own.
func (b *Bot) newTurnCard(key, chatID, lang string) *turnCard {
	return &turnCard{key: key, chatID: chatID, thread: threadOfKey(key), lang: lang, state: "working"}
}

// sendCard sends a card to a chat, or into the topic the turn is in,
// which Feishu takes as a reply to the message that began it.
func (b *Bot) sendCard(chatID, thread, shown string) (string, error) {
	if thread != "" {
		return b.msgs.SendCardReply(thread, shown)
	}
	return b.msgs.SendCard("chat_id", chatID, shown)
}

// card tells the turn's card, as it is now: sent as the turn's first
// card, and patched as the turn goes. It reports whether Feishu took it:
// a chat without cards, or one Feishu refuses, gets the turn's words as a
// message.
func (b *Bot) card(c *turnCard, state, text, note string) bool {
	if c == nil || !cards() {
		return false
	}
	body := html.UnescapeString(plainBody(text))
	if note != "" {
		body += "\n\n" + note
	}
	if body == "" && state == "working" {
		// Feishu takes no card without a body: a turn card says the
		// agent thinks until it has words of its own to show.
		body = "_" + word(c.lang, "state.thinking") + "_"
	}
	if c.msg != "" && c.state == state && c.text == body {
		// The card says this already: a patch of the same content is no
		// update, and Feishu counts one.
		return true
	}
	shown, err := cardJSON(Card{Title: titleOf(text), Text: body, State: state}, c.lang)
	if err != nil {
		b.logf("feishu: a card: %v", err)
		return false
	}
	if c.msg == "" {
		msg, err := b.sendCard(c.chatID, c.thread, shown)
		if err != nil {
			b.logf("feishu: sending a card: %v", err)
			return false
		}
		c.msg = msg
	} else if err := b.msgs.UpdateCard(c.msg, shown); err != nil {
		b.logf("feishu: updating a card: %v", err)
		return false
	}
	c.state, c.text = state, body
	return true
}

// cardTries is how many times a turn's card is patched into its answer:
// Feishu refuses a patch it takes for too frequent (230020), so a second
// try follows a pause, and a new card after them.
const cardTries = 3

// finishCard patches the turn's card into its answer, and reports whether
// Feishu took it: the answer goes as a new card when the turn's own goes
// unanswered.
func (b *Bot) finishCard(c *turnCard, card Card) bool {
	if c == nil || c.msg == "" || !cards() {
		return false
	}
	shown, err := cardJSON(card, c.lang)
	if err != nil {
		b.logf("feishu: a card: %v", err)
		return false
	}
	for i := 0; i < cardTries; i++ {
		if err := b.msgs.UpdateCard(c.msg, shown); err == nil {
			c.state, c.text = card.State, card.Text
			return true
		} else {
			b.logf("feishu: finishing a card: %v", err)
			time.Sleep(cardRetry * time.Duration(i+1))
		}
	}
	return false
}

// cardEvery is how often a working card may be patched: the screen
// changes many times a second, and Feishu takes a patch for too frequent.
var cardEvery = 1500 * time.Millisecond

// cardRetry is how long a failed patch of a turn's answer waits for the
// next, which is longer each try.
var cardRetry = 400 * time.Millisecond

// cardTail is a turn's text as far as it goes, as a working card shows
// it: the rows it printed since it began, without the screen's escapes.
func cardTail(screen string, start int) string { return clean(since(screen, start, toolTail)) }

// thinking is a working card's body before the turn has words of its own:
// its state and the time it has run, so that a long step shows that the
// agent is at work.
func thinking(c *turnCard, run time.Duration) string {
	return "_" + word(c.lang, "state.thinking") + " " + durationText(run) + "_"
}

// watchCard follows a turn of the pane from base on until it ends: the
// agent idle once seen working, or the pane waiting for the user. The
// state, body and failed are the turn's end.
func (b *Bot) watchCard(c *turnCard, id string, base int, stop time.Time) (state, body string, failed, seen bool) {
	timeout := time.Duration(b.cfg.Timeout)
	if timeout <= 0 {
		timeout = settings.FeishuDefaultTimeout
	}
	if stop.IsZero() {
		stop = time.Now().Add(timeout)
	}
	var p proto.Pane
	var last, still time.Time
	for time.Now().Before(stop) {
		time.Sleep(waits.poll)
		if err := b.api.Call("wait", proto.WaitParams{ID: id, Timeout: int64(waits.poll / time.Millisecond)}, &p); err != nil && !timedOut(err) {
			return "error", "", true, !last.IsZero()
		}
		now := time.Now()
		if p.Status == proto.Working {
			last, still = now, now
		}
		switch {
		case p.Status == proto.Waiting:
			text := b.read(id)
			if seen := since(text, base, replyTail); seen != "" && written(text) == "" &&
				clean(seen) == clean(textNote+since(text, base, toolTail)) {
				// The question is in the pane, and the card shows it.
				continue
			}
			head := ""
			if !last.IsZero() {
				head = textNote
			}
			return "asks", head + since(text, base, replyTail), false, !last.IsZero()
		case p.Status == proto.Failed:
			return "error", orDash(p.Message) + "\n\n" + since(b.read(id), base, toolTail), true, !last.IsZero()
		case !last.IsZero() && !still.IsZero() && now.Sub(still) >= waits.quiet:
			return "done", "", false, true
		}
	}
	return "done", "", false, !last.IsZero()
}

// read is a pane's screen, with its scrollback.
func (b *Bot) read(id string) string {
	var rd proto.ReadResult
	if err := b.api.Call("read", proto.ReadParams{ID: id, Scrollback: true}, &rd); err != nil {
		return ""
	}
	return rd.Text
}

// toolTail is the rows of the pane's text a working card shows.
const toolTail = 16

// cardValue is a button's value in a card.
func cardValue(v map[string]any, key string) (string, bool) {
	x, ok := v[key]
	if !ok {
		return "", false
	}
	return fmt.Sprint(x), true
}

// onCardAction answers the click of one of the bot's cards: the button
// that opens its session in Hyla.
func (b *Bot) onCardAction(ctx context.Context, e *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if e == nil || e.Event == nil || e.Event.Action == nil {
		return nil, nil
	}
	openID := ""
	if e.Event.Operator != nil {
		openID = e.Event.Operator.OpenID
	}
	if v, ok := cardValue(e.Event.Action.Value, "hyla"); ok && v == "perm" {
		return b.onAskAction(e)
	}
	if v, ok := cardValue(e.Event.Action.Value, "hyla"); ok && v == "choice" {
		return b.onChoiceAction(e)
	}
	if v, ok := cardValue(e.Event.Action.Value, "hyla"); !ok || v != "open" {
		return nil, nil
	}
	msg := ""
	status := "warning"
	if id := deref(&e.Event.Context.OpenMessageID); id != "" {
		if r, ok := b.note(id); ok {
			if r.Session != "" {
				if _, err := b.api.State(); err == nil {
					if err := b.api.Call("select", proto.FocusParams{Session: r.Session}, nil); err == nil {
						msg, status = word(langOf(r.Turn), "opened"), "success"
					} else {
						msg = err.Error()
					}
				}
			}
		}
	}
	if msg == "" && openID == "" {
		return nil, nil
	}
	if msg == "" {
		msg = word("en_us", "gone")
	}
	return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: status, Content: msg}}, nil
}

// markOn is the reaction a turn's message takes as the turn starts, which
// the turn takes back as it ends.
const markOn = "OnIt"

// marked is a message that takes the turn's marks.
type marked struct{ msg string }

// markOn marks the message of a turn as it starts, as the agent works.
func (b *Bot) markOn(msg string) *marked {
	if msg == "" || !cards() {
		return nil
	}
	if err := b.msgs.React(msg, markOn); err != nil {
		b.logf("feishu: marking a message: %v", err)
		return nil
	}
	return &marked{msg}
}

// off takes the mark back, as the answer comes.
func (b *Bot) off(m *marked) {
	if m == nil {
		return
	}
	if err := b.msgs.Unreact(m.msg, markOn); err != nil {
		b.logf("feishu: taking a mark back: %v", err)
	}
}
