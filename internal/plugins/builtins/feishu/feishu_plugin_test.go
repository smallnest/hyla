package feishuplugin

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"hyla/internal/plugins/daemonhost"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/settings"
)

func boolPtr(b bool) *bool { return &b }

func TestFeishuPluginRegistrationAndShouldStart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HYLA_HOME", dir)
	t.Setenv("HYLA_CONFIG", filepath.Join(dir, "config"))
	t.Setenv("HYLA_NO_KEYCHAIN", "1")

	st := &manifest.Store{BaseDir: dir, UseKeychain: boolPtr(false)}
	if _, err := manifest.ApplyBase(st); err != nil {
		t.Fatal(err)
	}
	list, err := st.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var found *manifest.InstalledPlugin
	for i := range list {
		if list[i].Entry.ID == PluginID {
			found = &list[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected %s in discovered plugins", PluginID)
	}
	if !found.Entry.Enabled || !found.Builtin || !found.Manifest.Daemon.KeepAlive {
		t.Fatalf("unexpected built-in Feishu entry: %+v, daemon=%+v", found.Entry, found.Manifest.Daemon)
	}

	// Unconfigured: must NOT auto-start or hold hylad alive.
	if ShouldAutoStart(st) {
		t.Fatal("ShouldAutoStart must be false when Feishu is not configured")
	}

	// Legacy feishu-app + feishu.json must migrate cleanly into plugin-config and enable auto-start.
	if err := settings.SetFeishu(settings.Feishu{
		App:   "cli_legacy_123",
		Allow: []string{"ou_1", "*"},
		Agent: "claude",
		Run:   settings.FeishuRunSession,
	}); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveFeishuSecret(settings.FeishuSecret{
		AppID:     "cli_legacy_123",
		AppSecret: "sec_abc",
	}); err != nil {
		t.Fatal(err)
	}
	if !ShouldAutoStart(st) {
		t.Fatal("ShouldAutoStart must be true once legacy Feishu settings and secret exist")
	}
	res := Resolve(st)
	if res.Settings.App != "cli_legacy_123" || !res.HasSecret || res.Secret.AppSecret != "sec_abc" {
		t.Fatalf("unexpected migrated Resolved config: %+v", res)
	}
	pcfg, err := st.LoadConfig(PluginID)
	if err != nil {
		t.Fatal(err)
	}
	if pcfg["app"] != "cli_legacy_123" || pcfg["agent"] != "claude" {
		t.Fatalf("expected legacy config migrated into plugin-config, got %+v", pcfg)
	}
}

func TestFeishuPluginSettingsSyncToLegacy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HYLA_HOME", dir)
	t.Setenv("HYLA_CONFIG", filepath.Join(dir, "config"))
	t.Setenv("HYLA_NO_KEYCHAIN", "1")

	st := &manifest.Store{BaseDir: dir, UseKeychain: boolPtr(false)}
	if err := st.SaveConfig(PluginID, map[string]any{
		"app":   "cli_plug_99",
		"allow": "ou_a, ou_b",
		"agent": "codex",
		"run":   settings.FeishuRunSession,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSecret(PluginID, "appSecret", "plug_secret"); err != nil {
		t.Fatal(err)
	}
	if err := SyncToLegacy(st); err != nil {
		t.Fatal(err)
	}
	loaded := settings.Load()
	if loaded.Feishu == nil || loaded.Feishu.App != "cli_plug_99" || loaded.Feishu.Agent != "codex" {
		t.Fatalf("expected legacy settings synced, got %+v", loaded.Feishu)
	}
	sec, err := settings.LoadFeishuSecret()
	if err != nil || sec.AppID != "cli_plug_99" || sec.AppSecret != "plug_secret" {
		t.Fatalf("expected legacy feishu.json synced, got %+v err=%v", sec, err)
	}
}

func TestFeishuPluginDeclarativeCardAndReconnect(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HYLA_HOME", dir)
	t.Setenv("HYLA_CONFIG", filepath.Join(dir, "config"))
	t.Setenv("HYLA_NO_KEYCHAIN", "1")

	restarted := 0
	SetRuntime(RuntimeHooks{
		BaseDir: func() string { return dir },
		Status: func() RuntimeStatus {
			return RuntimeStatus{
				Configured: true,
				Secret:     true,
				Connected:  true,
				App:        "cli_card_1",
				Allow:      []string{"*"},
				Agent:      "claude",
				Run:        settings.FeishuRunCommand,
			}
		},
		Restart: func(force bool) error {
			if force {
				restarted++
			}
			return nil
		},
	})
	t.Cleanup(func() { SetRuntime(RuntimeHooks{}) })

	st := &manifest.Store{BaseDir: dir, UseKeychain: boolPtr(false)}
	mgr := daemonhost.NewManager(daemonhost.ManagerOptions{
		HostKind:    daemonhost.HostKindDaemon,
		HylaVersion: "3.16.1",
		Store:       st,
	})
	t.Cleanup(mgr.ShutdownAll)
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var render rpc.UIRenderResult
	if err := mgr.Call(ctx, PluginID, "ui.render", rpc.UIRenderParams{SlotID: "status", Mount: "info.card"}, &render); err != nil {
		t.Fatalf("ui.render error: %v", err)
	}
	if render.Root == nil || render.Root.Type != "infoCard" {
		t.Fatalf("expected infoCard root, got %+v", render.Root)
	}
	raw, _ := json.Marshal(render.Root)
	if len(raw) == 0 {
		t.Fatal("empty render")
	}

	var act rpc.ActionInvokeResult
	if err := mgr.Call(ctx, PluginID, "action.invoke", rpc.ActionInvokeParams{ActionID: "reconnect"}, &act); err != nil {
		t.Fatalf("action.invoke error: %v", err)
	}
	if !act.OK || restarted != 1 {
		t.Fatalf("expected reconnect invocation, ok=%v restarted=%d", act.OK, restarted)
	}
}
