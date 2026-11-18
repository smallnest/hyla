package daemon

import (
	"encoding/json"
	"io"
	"log"
	"testing"

	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
)

func TestDaemonPluginListCallAndPluginPanePersistence(t *testing.T) {
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
		proto.ClearPluginAgents()
	})

	list := d.pluginList()
	foundMagpie := false
	for _, st := range list {
		if st.ID == "hyla.magpie" {
			foundMagpie = true
		}
	}
	if !foundMagpie {
		t.Fatalf("expected built-in hyla.magpie in plugin.list, got %+v", list)
	}
	if d.plugins.HasKeepAliveRunning() {
		t.Fatal("HasKeepAliveRunning must be false when no keep_alive plugins are running")
	}

	// Call ui.render on hyla.magpie via daemon pluginCall proxy
	rawParams, _ := json.Marshal(rpc.UIRenderParams{SlotID: "magpie", Mount: "info.card"})
	resRaw, err := d.pluginCall(rpc.PluginCallParams{
		PluginID: "hyla.magpie",
		Method:   "ui.render",
		Params:   rawParams,
	})
	if err != nil {
		t.Fatalf("pluginCall ui.render hyla.magpie: %v", err)
	}
	var uiRes rpc.UIRenderResult
	if err := json.Unmarshal(resRaw, &uiRes); err != nil {
		t.Fatalf("unmarshal ui.render result: %v", err)
	}
	if uiRes.Root == nil || uiRes.Root.Type != "infoCard" {
		t.Fatalf("unexpected hyla.magpie render result: %+v", uiRes)
	}

	// Plugin pane creation + saveState / restorePane roundtrip without starting a PTY shell.
	d.projects = []proto.Project{{ID: "proj1", Name: "Home", Dir: dir, Default: true}}
	sess, err := d.create(proto.CreateParams{
		Project:     "proj1",
		Plugin:      "hyla.magpie",
		PluginView:  "dashboard",
		PluginTitle: "Magpie Stats",
		PluginArgs:  map[string]string{"tab": "today"},
	})
	if err != nil {
		t.Fatalf("create plugin pane: %v", err)
	}
	if len(sess.Panes) != 1 || !sess.Panes[0].PluginPane() || sess.Panes[0].PaneName() != "Magpie Stats" {
		t.Fatalf("unexpected plugin pane created: %+v", sess)
	}

	d.saveState()
	restored := d.restorePane("s-restored", savedPane{
		ID:          sess.Panes[0].ID,
		Plugin:      "hyla.magpie",
		PluginView:  "dashboard",
		PluginTitle: "Magpie Stats",
		PluginArgs:  map[string]string{"tab": "today"},
		Cols:        80,
		Rows:        24,
	})
	if restored == nil || !restored.info.PluginPane() || restored.term != nil || restored.proc != nil {
		t.Fatalf("unexpected restored plugin pane: %+v", restored)
	}

	// Plugin agent process detection & resumeCommand
	proto.SetPluginAgents("com.example.myagent", []proto.AgentMeta{{
		ID:         "myagent",
		Name:       "My Agent",
		Processes:  []string{"myagent-cli"},
		ResumeArgv: []string{"myagent-cli", "--resume", "{id}"},
	}})
	if got := detectAgent("myagent-cli", nil); got != "myagent" {
		t.Fatalf("detectAgent(myagent-cli) = %q, want myagent", got)
	}
	cmd := resumeCommand(&proto.AgentSession{Agent: "myagent", ID: "conv-123"})
	if len(cmd) != 3 || cmd[0] != "myagent-cli" || cmd[2] != "conv-123" {
		t.Fatalf("unexpected plugin resumeCommand: %v", cmd)
	}
}
