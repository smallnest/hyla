package marks

import (
	"os"
	"path/filepath"
	"testing"

	"hyla/internal/plugins/manifest"
)

func TestLoadPluginMarks(t *testing.T) {
	t.Cleanup(func() { LoadPluginMarks(nil) })

	dir := t.TempDir()
	svgPath := filepath.Join(dir, "aider.svg")
	if err := os.WriteFile(svgPath, []byte(`<svg viewBox="0 0 16 16" fill="#ff5500"><path d="M0 0h16v16z"/></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}

	LoadPluginMarks([]manifest.MarkAsset{{
		PluginID:  "com.example.aider",
		Agent:     "aider",
		Path:      svgPath,
		FullColor: true,
	}})

	svg, col, ok := Agent("aider")
	if !ok || svg == nil {
		t.Fatalf("Agent(aider) = (%v, %v, %v), want loaded SVG", svg, col, ok)
	}
	full, known := MarkFull("aider")
	if !known || full == nil {
		t.Fatalf("MarkFull(aider) = (%v, %v), want full color SVG", full, known)
	}
	if _, logoKnown := LogoOf("aider"); !logoKnown {
		t.Fatal("LogoOf(aider) should be known")
	}

	// Built-in agent marks remain protected against plugin overrides.
	if _, _, ok := Agent("claude"); !ok {
		t.Fatal("built-in claude mark should still resolve")
	}
}
