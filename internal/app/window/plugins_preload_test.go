package window

import (
	"os"
	"path/filepath"
	"testing"

	"hyla/internal/actions"
	"hyla/internal/plugins/manifest"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/wash"
)

func TestPreloadPluginBasePreventsConfigProblems(t *testing.T) {
	tmp := t.TempDir()
	hylaDir := filepath.Join(tmp, "hyla-home")
	cfgPath := filepath.Join(tmp, "config")
	t.Setenv("HYLA_DIR", hylaDir)
	t.Setenv("HYLA_CONFIG", cfgPath)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")
	t.Cleanup(func() {
		actions.ClearPluginActions()
		proto.ClearPluginAgents()
		wash.ReloadPlugins(nil)
		settings.SetPluginBackgrounds(nil)
	})

	store := manifest.DefaultStore()
	pluginID := "com.example.look"
	pdir, err := store.PluginDir(pluginID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pdir, "washes"), 0o755); err != nil {
		t.Fatal(err)
	}
	washTOML := `id = "aurora"
name = "Aurora"
light = ["#eef4ff", "#ddeaff"]
dark = ["#0b1322", "#14223a"]
`
	if err := os.WriteFile(filepath.Join(pdir, "washes", "aurora.toml"), []byte(washTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	tomlBody := `
id = "com.example.look"
name = "Look"
version = "1.0.0"
api_version = 1

[[washes]]
id = "aurora"
name = "Aurora"
file = "washes/aurora.toml"

[[actions]]
id = "apply"
title = "Apply Aurora"
remote = true
`
	if err := os.WriteFile(filepath.Join(pdir, manifest.ManifestFileName), []byte(tomlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRegistry(&manifest.Registry{
		Version: 1,
		Plugins: map[string]manifest.RegistryEntry{
			pluginID: {ID: pluginID, Enabled: true, Version: "1.0.0"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("background = aurora\nkeybind = cmd+alt+l="+manifest.QualifyActionID(pluginID, "apply")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	preloadPluginBase()
	loaded := settings.Load()
	if loaded.Background != "aurora" {
		t.Fatalf("Background=%q, want aurora", loaded.Background)
	}
	if probs := settings.Problems(); len(probs) != 0 {
		t.Fatalf("unexpected settings problems after preloadPluginBase: %v", probs)
	}
}
