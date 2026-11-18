package daemon

import (
	"encoding/json"
	"io"
	"log"
	"testing"

	feishuplugin "hyla/internal/plugins/builtins/feishu"
	"hyla/internal/proto"
)

type stubChannelDriver struct {
	id        string
	active    bool
	keepAlive bool
	prompted  []string
	stopped   bool
}

func (s *stubChannelDriver) ID() string      { return s.id }
func (s *stubChannelDriver) Active() bool    { return s.active }
func (s *stubChannelDriver) KeepAlive() bool { return s.keepAlive }
func (s *stubChannelDriver) Prompt(session, text string) error {
	s.prompted = append(s.prompted, session+":"+text)
	return nil
}
func (s *stubChannelDriver) Call(method string, raw json.RawMessage) (any, bool, error) {
	if method == "stub.ping" {
		return "pong", true, nil
	}
	return nil, false, nil
}
func (s *stubChannelDriver) Stop() { s.stopped = true }

func TestChannelDriverDispatchAndChatSend(t *testing.T) {
	dir := t.TempDir()
	d := &Daemon{
		dir:      dir,
		log:      log.New(io.Discard, "", 0),
		panes:    map[string]*pane{},
		subs:     map[*controlConn]struct{}{},
		watchers: map[*controlConn]struct{}{},
		clients:  map[*proto.ClientInfo]struct{}{},
		changeCh: make(chan struct{}, 1),
		saveCh:   make(chan struct{}, 1),
		quit:     make(chan struct{}),
	}
	empty := ""
	d.viewed.Store(&empty)
	d.initPlugins()
	t.Cleanup(func() {
		if d.plugins != nil {
			d.plugins.ShutdownAll()
		}
		feishuplugin.SetRuntime(feishuplugin.RuntimeHooks{})
		proto.ClearPluginAgents()
	})

	// Unconfigured Feishu does not keep daemon alive.
	if d.channelKeepAlive() {
		t.Fatal("channelKeepAlive must be false when Feishu is unconfigured")
	}
	if d.channelActive() {
		t.Fatal("channelActive must be false when Feishu is unconfigured")
	}

	// Bundled hyla.feishu appears in plugin.list and has KeepAlive metadata.
	var foundFeishu bool
	for _, st := range d.pluginList() {
		if st.ID == feishuplugin.PluginID {
			foundFeishu = true
			if !st.Enabled || !st.KeepAlive {
				t.Fatalf("unexpected hyla.feishu status: %+v", st)
			}
		}
	}
	if !foundFeishu {
		t.Fatal("hyla.feishu missing from plugin.list")
	}

	// Disabling hyla.feishu updates feishu.status error cleanly.
	if err := d.pluginStore().Enable(feishuplugin.PluginID, false); err != nil {
		t.Fatal(err)
	}
	if d.isFeishuPluginEnabled() {
		t.Fatal("expected isFeishuPluginEnabled to reflect disabled state")
	}

	// Custom ChannelDriver routes Prompt, KeepAlive, Call, and Stop cleanly.
	stub := &stubChannelDriver{id: "com.example.slack", active: true, keepAlive: true}
	d.registerChannel(stub)
	if !d.channelActive() || !d.channelKeepAlive() {
		t.Fatal("expected custom ChannelDriver active and keepAlive")
	}
	res, err := d.call(&controlConn{}, "stub.ping", nil)
	if err != nil || res != "pong" {
		t.Fatalf("stub channel call = %v, %v", res, err)
	}

	d.sessions = []*session{{
		id:    "s1",
		focus: "p1",
		panes: []*pane{{
			sid:  "s1",
			info: proto.Pane{ID: "p1"},
		}},
	}}
	d.panes["p1"] = d.sessions[0].panes[0]

	sent, err := d.chatSend(proto.ChatSendParams{Session: "s1", Pane: "p1", Text: "hello channel"})
	if err != nil || !sent || len(stub.prompted) != 1 || stub.prompted[0] != "s1:hello channel" {
		t.Fatalf("chatSend through ChannelDriver failed: sent=%v err=%v prompted=%v", sent, err, stub.prompted)
	}

	// pane.answer sets both legacy FeishuAnswer and generalized AnswerDir.
	rawAns, _ := json.Marshal(proto.AnswerParams{Pane: "p1", Dir: "/tmp/ans"})
	if _, err := d.call(&controlConn{}, "pane.answer", rawAns); err != nil {
		t.Fatal(err)
	}
	pn := d.findPane("p1")
	pn.mu.Lock()
	ansLegacy, ansNew := pn.info.FeishuAnswer, pn.info.AnswerDir
	pn.mu.Unlock()
	if ansLegacy != "/tmp/ans" || ansNew != "/tmp/ans" {
		t.Fatalf("pane.answer did not populate both fields: FeishuAnswer=%q AnswerDir=%q", ansLegacy, ansNew)
	}

	d.stopChannels()
	if !stub.stopped {
		t.Fatal("stopChannels did not stop registered ChannelDriver")
	}
}
