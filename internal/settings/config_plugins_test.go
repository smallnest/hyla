package settings

import (
	"strings"
	"testing"

	"hyla/internal/actions"
)

func TestPluginKeybindAndBackgroundConfig(t *testing.T) {
	t.Cleanup(func() {
		actions.ClearPluginActions()
		SetPluginBackgrounds(nil)
	})

	// Disabled / future plugin action keybind succeeds without config problem.
	s, probs := parse("keybind = cmd+alt+n=com.acme.notes.open\n")
	if len(probs) != 0 {
		t.Fatalf("parse valid plugin keybind produced problems: %v", probs)
	}
	if s.Shortcuts["com.acme.notes.open"] != "CmdOrCtrl+Alt+N" {
		t.Fatalf("unexpected shortcut normalization: %q", s.Shortcuts["com.acme.notes.open"])
	}

	// Formatting preserves shortcuts for temporarily disabled plugins.
	cfgText := format(s, "")
	if !strings.Contains(cfgText, "com.acme.notes.open") {
		t.Fatalf("format dropped disabled plugin keybind:\n%s", cfgText)
	}

	// Legacy built-in timeline.open keybind migrates cleanly to hyla.timeline.open.
	sMig, probs := parse("keybind = cmd+alt+t=timeline.open\n")
	if len(probs) != 0 {
		t.Fatalf("legacy timeline.open migration produced problems: %v", probs)
	}
	if sMig.Shortcuts["hyla.timeline.open"] != "CmdOrCtrl+Alt+T" {
		t.Fatalf("expected hyla.timeline.open shortcut CmdOrCtrl+Alt+T, got %+v", sMig.Shortcuts)
	}

	// Built-in namespace typo still produces config problem.
	if _, probs := parse("keybind = cmd+alt+n=pane.split.bogus\n"); len(probs) == 0 {
		t.Fatal("expected problem for invalid built-in action typo pane.split.bogus")
	}

	// Plugin wash ID accepted when registered, rejected when unknown.
	SetPluginBackgrounds([]string{"aurora"})
	s2, probs := parse("background = aurora\n")
	if len(probs) != 0 {
		t.Fatalf("parse plugin background produced problems: %v", probs)
	}
	if s2.Background != "aurora" {
		t.Fatalf("Background = %q, want aurora", s2.Background)
	}
	if _, probs := parse("background = nonexistent-wash\n"); len(probs) == 0 {
		t.Fatal("expected problem for unknown background")
	}
}
