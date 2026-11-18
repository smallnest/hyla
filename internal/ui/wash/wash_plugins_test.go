package wash

import (
	"os"
	"path/filepath"
	"testing"

	"hyla/internal/plugins/manifest"
)

func TestReloadPluginWashes(t *testing.T) {
	t.Cleanup(func() { _ = ReloadPlugins(nil) })

	dir := t.TempDir()
	path := filepath.Join(dir, "aurora.toml")
	err := os.WriteFile(path, []byte(`
id = "aurora"
name = "Aurora"
light = ["#dbeafe", "#ede9fe", "#fce7f3"]
light_glow = "#a7f3d080"
light_angle = 135
dark = ["#090d16", "#111827"]
dark_glow = "rgba(56, 189, 248, 0.2)"
dark_angle = 135
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if err := ReloadPlugins([]manifest.WashAsset{{
		PluginID: "com.example.aurora",
		ID:       "aurora",
		Name:     "Aurora",
		Path:     path,
	}}); err != nil {
		t.Fatalf("ReloadPlugins: %v", err)
	}
	if !Known("aurora") {
		t.Fatal("expected aurora wash to be Known")
	}
	base, glow := Washes("aurora", true)
	if len(base.Stops) != 2 || base.Angle != 135 || len(glow.Stops) != 2 {
		t.Fatalf("unexpected wash gradients: base=%+v glow=%+v", base, glow)
	}

	// Conflicting built-in ID must fail without replacing built-in wash.
	conflictPath := filepath.Join(dir, "rex.toml")
	_ = os.WriteFile(conflictPath, []byte(`
light = ["#000000", "#111111"]
dark = ["#000000", "#111111"]
`), 0o644)
	if err := ReloadPlugins([]manifest.WashAsset{{
		PluginID: "com.example.bad",
		ID:       "rex",
		Name:     "Bad Rex",
		Path:     conflictPath,
	}}); err == nil {
		t.Fatal("expected conflict error when overriding built-in rex wash")
	}
}
