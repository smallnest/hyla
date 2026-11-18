package feishuplugin

import (
	"testing"

	"hyla/internal/plugins/manifest"
	"hyla/internal/settings"
)

func TestSyncToLegacyClearsAppAndAllow(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := tmp + "/config"
	t.Setenv("HYLA_DIR", tmp)
	t.Setenv("HYLA_CONFIG", cfgPath)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")

	if err := settings.SetFeishu(settings.Feishu{
		App:   "cli_initial",
		Allow: []string{"ou_1", "ou_2"},
	}); err != nil {
		t.Fatal(err)
	}
	st := manifest.DefaultStore()
	if err := st.SaveConfig(PluginID, map[string]any{
		"app":   "",
		"allow": "",
	}); err != nil {
		t.Fatal(err)
	}
	if err := SyncToLegacy(st); err != nil {
		t.Fatal(err)
	}
	got := settings.Load().Feishu
	if got == nil || got.App != "" || len(got.Allow) != 0 {
		t.Fatalf("expected cleared app and allow in legacy settings, got %+v", got)
	}
}
