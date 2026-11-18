package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
)

// TestResumePluginProcessHelper acts as a minimal JSON-RPC 2.0 daemon plugin
// process when invoked with HYLA_TEST_RESUME_PLUGIN=1.
func TestResumePluginProcessHelper(t *testing.T) {
	if os.Getenv("HYLA_TEST_RESUME_PLUGIN") != "1" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	peer := rpc.NewPeer(os.Stdin, os.Stdout, nil)
	peer.Handle("initialize", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return rpc.InitializeResult{
			Name:    "resume-helper",
			Version: "1.0.0",
		}, nil
	})
	peer.Handle("agent.resumeCommand", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		var p rpc.AgentResumeCommandParams
		_ = json.Unmarshal(params, &p)
		return rpc.AgentResumeCommandResult{
			Argv: []string{"/bin/sh", "-c", fmt.Sprintf("printf '%%s\\n' %q && sleep 0.2", p.Agent+":"+p.ConversationID+"@"+p.Dir)},
		}, nil
	})
	peer.Handle("shutdown", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		return map[string]any{"ok": true}, nil
	})
	_ = peer.Serve(ctx)
	os.Exit(0)
}

func TestSDKWireFormatsProto(t *testing.T) {
	tlRaw := `{
		"timeline": {
			"session": "s1",
			"pane": "p1",
			"agent": "demo-agent",
			"read": 1700000000,
			"turns": [{
				"kind": "turn",
				"title": "Inspect repo",
				"detail": "ok",
				"start": 100,
				"end": 200,
				"state": "done",
				"children": [{"kind": "tool", "title": "read_file", "state": "done"}]
			}],
			"note": "ok"
		}
	}`
	var tl rpc.AgentReadTimelineResult
	if err := json.Unmarshal([]byte(tlRaw), &tl); err != nil {
		t.Fatal(err)
	}
	if len(tl.Timeline.Turns) != 1 || tl.Timeline.Turns[0].Kind != proto.TimelineTurn || tl.Timeline.Turns[0].Title != "Inspect repo" ||
		tl.Timeline.Turns[0].State != proto.TimelineDone || len(tl.Timeline.Turns[0].Children) != 1 ||
		tl.Timeline.Turns[0].Children[0].Title != "read_file" {
		t.Fatalf("unexpected timeline unmarshal: %+v", tl.Timeline)
	}

	chRaw := `{
		"chat": {
			"session": "s1",
			"pane": "p1",
			"agent": "demo-agent",
			"model": "demo-1",
			"input": true,
			"turn": false,
			"lines": [{"kind": "agent", "text": "hello", "state": "done"}]
		}
	}`
	var ch rpc.AgentReadChatResult
	if err := json.Unmarshal([]byte(chRaw), &ch); err != nil {
		t.Fatal(err)
	}
	if ch.Chat.Model != "demo-1" || !ch.Chat.Input || len(ch.Chat.Lines) != 1 || ch.Chat.Lines[0].Text != "hello" {
		t.Fatalf("unexpected chat unmarshal: %+v", ch.Chat)
	}

	subRaw := `{
		"subs": [{"id": "sub1", "type": "explore", "task": "Worker", "tool": "grep", "state": "running"}]
	}`
	var sub rpc.AgentReadSubagentsResult
	if err := json.Unmarshal([]byte(subRaw), &sub); err != nil {
		t.Fatal(err)
	}
	if len(sub.Subs) != 1 || sub.Subs[0].Type != "explore" || sub.Subs[0].Task != "Worker" || sub.Subs[0].State != proto.TimelineRunning {
		t.Fatalf("unexpected subagent unmarshal: %+v", sub.Subs)
	}
}

func TestPluginResumeTemplateTokensAndValidation(t *testing.T) {
	proto.SetPluginAgents("com.example.tpl", []proto.AgentMeta{{
		ID:         "tpl-agent",
		Name:       "Template Agent",
		PluginID:   "com.example.tpl",
		ResumeArgv: []string{"tpl", "--resume", "{session_id}", "--tag", "{id}"},
	}})
	defer proto.ClearPluginAgents()

	got := resumeCommand(&proto.AgentSession{Agent: "tpl-agent", ID: "conv-123"})
	want := []string{"tpl", "--resume", "conv-123", "--tag", "conv-123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resumeCommand=%v, want %v", got, want)
	}
	if bad := resumeCommand(&proto.AgentSession{Agent: "tpl-agent", ID: "../bad"}); bad != nil {
		t.Fatalf("expected nil for invalid session id, got %v", bad)
	}
	if !canResumeAgentSession(&proto.AgentSession{Agent: "tpl-agent", ID: "conv-123"}) {
		t.Fatal("expected template agent session to be resumable")
	}
}

func TestPluginResumeCommandEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	hylaDir := filepath.Join(tmp, "hyla-home")
	workDir := filepath.Join(tmp, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hylaDir, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(tmp, "config")
	if err := os.WriteFile(cfgPath, []byte("session-history = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYLA_DIR", hylaDir)
	t.Setenv("HYLA_CONFIG", cfgPath)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")
	t.Setenv("HYLA_TEST_RESUME_PLUGIN", "1")
	proto.ClearPluginAgents()
	defer proto.ClearPluginAgents()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	store := &manifest.Store{BaseDir: hylaDir}
	pluginID := "com.example.resumer"
	pdir, err := store.PluginDir(pluginID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	tomlBody := fmt.Sprintf(`
id = %q
name = "Resumer"
version = "1.0.0"
api_version = 1

[permissions]
scopes = ["agent.provider"]

[daemon]
command = [%q, "-test.run=^TestResumePluginProcessHelper$"]
auto_start = true

[[agents]]
id = "rpc-agent"
name = "RPC Agent"
`, pluginID, exe)
	if err := os.WriteFile(filepath.Join(pdir, manifest.ManifestFileName), []byte(tomlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRegistry(&manifest.Registry{
		Version: 1,
		Plugins: map[string]manifest.RegistryEntry{
			pluginID: {
				ID:                 pluginID,
				Enabled:            true,
				GrantedPermissions: []string{"agent.provider"},
				Version:            "1.0.0",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{
		dir:         hylaDir,
		exe:         exe,
		started:     time.Now(),
		log:         log.New(io.Discard, "", 0),
		subs:        map[*controlConn]struct{}{},
		watchers:    map[*controlConn]struct{}{},
		pocketWatch: map[*PocketWatch]struct{}{},
		clients:     map[*proto.ClientInfo]struct{}{},
		panes:       map[string]*pane{},
		changeCh:    make(chan struct{}, 1),
		saveCh:      make(chan struct{}, 1),
		quit:        make(chan struct{}),
		idleAt:      time.Now(),
	}
	d.initPlugins()
	defer func() {
		if d.plugins != nil {
			d.plugins.ShutdownAll()
		}
	}()

	// Wait for plugin process to initialize.
	deadline := time.Now().Add(5 * time.Second)
	var found *rpc.PluginStatus
	for time.Now().Before(deadline) {
		for _, st := range d.plugins.ListStatuses() {
			if st.ID == pluginID {
				cp := st
				found = &cp
				break
			}
		}
		if found != nil && found.State == "running" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if found == nil || found.State != "running" {
		t.Fatalf("plugin failed to reach running state: %+v", d.plugins.ListStatuses())
	}

	// 1. Direct pluginResumeCommand passes Dir & validates ID.
	cmd := d.pluginResumeCommand(&proto.AgentSession{Agent: "rpc-agent", ID: "conv-99"}, workDir)
	if len(cmd) == 0 || cmd[0] != "/bin/sh" {
		t.Fatalf("unexpected pluginResumeCommand result: %v", cmd)
	}
	if bad := d.pluginResumeCommand(&proto.AgentSession{Agent: "rpc-agent", ID: "bad/id"}, workDir); bad != nil {
		t.Fatalf("expected nil for unsafe id, got %v", bad)
	}

	// 2. conversations store accepts plugin agent without static resume_argv, and resumeConversation calls plugin RPC.
	d.convs.note(proto.Conversation{Agent: "rpc-agent", ID: "conv-99", Dir: workDir, Name: "rpc-sess"}, time.Now().UnixMilli())
	d.saveConversations()
	d.convs.mu.Lock()
	d.convs.list = nil
	d.convs.mu.Unlock()
	d.loadConversations()
	if _, ok := d.convs.find("rpc-agent", "conv-99"); !ok {
		t.Fatal("expected loadConversations to preserve plugin agent conversation")
	}

	sinfo, err := d.resumeConversation(proto.ConversationParams{Agent: "rpc-agent", ID: "conv-99"})
	if err != nil {
		t.Fatalf("resumeConversation failed: %v", err)
	}
	if len(sinfo.Panes) == 0 {
		t.Fatalf("expected resumed pane in %+v", sinfo)
	}

	// 3. pane.agentSession RPC accepts plugin agent and closing moves it to ended, then ended.resume launches via plugin RPC.
	pnID := sinfo.Panes[0].ID
	pid := 0
	d.mu.Lock()
	if pn := d.panes[pnID]; pn != nil {
		pn.mu.Lock()
		pid = pn.shellPID
		pn.mu.Unlock()
	}
	d.mu.Unlock()
	rawAS, _ := json.Marshal(proto.AgentSessionParams{
		PID:     pid,
		Agent:   "rpc-agent",
		Session: "conv-100",
		Dir:     workDir,
	})
	asRes, rpcErr := d.call(nil, "pane.agentSession", rawAS)
	if rpcErr != nil {
		t.Fatalf("pane.agentSession failed: %v", rpcErr)
	}
	if _, ok := asRes.(proto.AgentSessionResult); !ok {
		t.Fatalf("unexpected agentSession result type: %T", asRes)
	}

	if err := d.removeSession(sinfo.ID); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	hasEnded := slices.ContainsFunc(d.ended, func(e proto.EndedSession) bool {
		return e.Agent == "rpc-agent" && e.ID == "conv-100"
	})
	d.mu.Unlock()
	if !hasEnded {
		t.Fatal("expected endedLocked to record plugin agent session")
	}

	resumed, err := d.endedCall("ended.resume", "conv-100")
	if err != nil {
		t.Fatalf("ended.resume failed: %v", err)
	}
	rs, ok := resumed.(proto.Session)
	if !ok || len(rs.Panes) == 0 {
		t.Fatalf("unexpected ended.resume result: %#v", resumed)
	}
	_ = d.removeSession(rs.ID)
}
