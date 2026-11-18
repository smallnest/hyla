package feishu

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hyla/internal/plugins/manifest"
	"hyla/internal/settings"
)

// What Hyla 3.17.0's hyla.feishu plugin kept of the bot is the
// configuration's after an upgrade, secret included, and the plugin is
// gone: the bot connects from the configuration alone.
func TestMigratePluginConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")
	settings.Reload()
	t.Cleanup(settings.Reload)

	st := &manifest.Store{BaseDir: home}
	cfg := map[string]any{
		"app":   "cli_migrated",
		"allow": "a@example.com, *",
		"agent": "claude",
		"run":   settings.FeishuRunCommand,
		"dir":   "/tmp/sessions",
	}
	if err := st.SaveConfig(pluginID, cfg); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSecret(pluginID, "appSecret", "s3cret"); err != nil {
		t.Fatal(err)
	}
	// The entry 3.17.0 wrote for the bundled plugin.
	reg := manifest.Registry{Version: 1, Plugins: map[string]manifest.RegistryEntry{
		pluginID: {ID: pluginID, Enabled: true, Source: "builtin"},
	}}
	b, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.RegistryPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigratePluginConfig(st); err != nil {
		t.Fatal(err)
	}

	f := settings.Load().Feishu
	if f == nil {
		t.Fatal("the configuration has no Feishu keys after the migration")
	}
	if f.App != "cli_migrated" || f.Agent != "claude" || f.Run != settings.FeishuRunCommand || f.Dir != "/tmp/sessions" {
		t.Fatalf("settings not migrated: %+v", f)
	}
	if len(f.Allow) != 2 || f.Allow[0] != "*" || f.Allow[1] != "a@example.com" {
		t.Fatalf("allow not migrated: %v", f.Allow)
	}
	if f.Timeout != int64(settings.FeishuDefaultTimeout) || f.Stream != int64(settings.FeishuDefaultStream) {
		t.Fatalf("the migration dropped the defaults: %+v", f)
	}
	sec, err := settings.LoadFeishuSecret()
	if err != nil {
		t.Fatal(err)
	}
	if sec.AppID != "cli_migrated" || sec.AppSecret != "s3cret" {
		t.Fatalf("secret not migrated: %+v", sec)
	}

	// The plugin's entry, config and secret are gone, and what it read
	// stays beside them.
	kept, err := st.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := kept.Plugins[pluginID]; ok {
		t.Fatal("the plugin's registry entry is still there")
	}
	p, err := st.ConfigPath(pluginID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("the plugin's config is still there: %v", err)
	}
	if _, err := os.Stat(p + ".migrated"); err != nil {
		t.Fatalf("the plugin's config was not kept for a look: %v", err)
	}
	if _, ok, err := st.GetSecret(pluginID, "appSecret"); err != nil || ok {
		t.Fatalf("the plugin's secret is still there: %v, %v", ok, err)
	}

	// A second start finds nothing to do, and keeps the configuration.
	if err := MigratePluginConfig(st); err != nil {
		t.Fatal(err)
	}
	if got := settings.Load().Feishu; got == nil || got.App != "cli_migrated" {
		t.Fatalf("the second migration changed the settings: %+v", got)
	}
}

// A machine that never had the plugin migrates nothing, and starts the
// bot from what the configuration already says.
func TestMigratePluginConfigWithoutPlugin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")
	settings.Reload()
	t.Cleanup(settings.Reload)

	if err := settings.SetFeishu(settings.Feishu{App: "cli_kept"}); err != nil {
		t.Fatal(err)
	}
	if err := MigratePluginConfig(&manifest.Store{BaseDir: home}); err != nil {
		t.Fatal(err)
	}
	if f := settings.Load().Feishu; f == nil || f.App != "cli_kept" {
		t.Fatalf("the migration changed a configured bot: %+v", f)
	}
	if _, err := os.Stat(filepath.Join(home, "plugin-config")); !os.IsNotExist(err) {
		t.Fatalf("the migration wrote plugin files: %v", err)
	}
}
