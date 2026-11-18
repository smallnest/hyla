package theme

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"hyla/internal/plugins/manifest"
)

func TestReloadPluginThemes(t *testing.T) {
	t.Cleanup(func() { _ = ReloadPluginThemes(nil) })

	dir := t.TempDir()
	path := filepath.Join(dir, "aurora-dark.ghostty")
	err := os.WriteFile(path, []byte(`
background = #111827
foreground = #e5e7eb
cursor-color = #38bdf8
selection-background = #1e3a8a
palette = 0=#111827
palette = 1=#f87171
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if err := ReloadPluginThemes([]manifest.ThemeAsset{{
		PluginID: "com.example.themes",
		Name:     "Aurora Dark",
		Path:     path,
	}}); err != nil {
		t.Fatalf("ReloadPluginThemes: %v", err)
	}

	if !slices.Contains(TerminalThemes(), "Aurora Dark") {
		t.Fatal("TerminalThemes missing Aurora Dark")
	}
	th, err := TerminalTheme("Aurora Dark")
	if err != nil || th == nil {
		t.Fatalf("TerminalTheme(Aurora Dark) failed: %v", err)
	}
	if _, err := TerminalTheme("NonExistentPluginTheme"); err == nil {
		t.Fatal("expected error for unknown theme")
	}
}
