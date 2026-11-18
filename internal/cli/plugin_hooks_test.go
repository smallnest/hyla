package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hyla/internal/plugins/manifest"
)

func TestPluginHooksAndSkillsSync(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HYLA_HOME", filepath.Join(home, ".hyla"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))

	// Create an existing Claude settings.json with a user hook to verify preservation.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userCfg := `{
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "/usr/local/bin/user-stop-hook"}]}
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(userCfg), 0o644); err != nil {
		t.Fatal(err)
	}

	// Link a local plugin with a Claude Stop hook and a skill.
	pluginDir := filepath.Join(home, "my-plugin")
	if err := os.MkdirAll(filepath.Join(pluginDir, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pluginDir, "skills", "extra-guide"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(pluginDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "com.example.claude-ext"
name = "Claude Ext"
version = "1.0.0"

[[hooks]]
agent = "claude"
event = "Stop"
script = "hooks/on-stop.sh"

[[skills]]
agent = "claude"
name = "extra-guide"
dir = "skills/extra-guide"
`), 0o644)
	_ = os.WriteFile(filepath.Join(pluginDir, "hooks", "on-stop.sh"), []byte("#!/bin/sh\necho hook:__HYLA__\n"), 0o755)
	_ = os.WriteFile(filepath.Join(pluginDir, "skills", "extra-guide", "SKILL.md"), []byte("Run __HYLA__ status\n"), 0o644)

	st := manifest.DefaultStore()
	if _, err := st.Link(pluginDir, true); err != nil {
		t.Fatalf("Link: %v", err)
	}

	if err := applyPluginAgentIntegrations(st, "claude", true); err != nil {
		t.Fatalf("applyPluginAgentIntegrations(install): %v", err)
	}

	// Skill installed and __HYLA__ expanded.
	skillData, err := os.ReadFile(filepath.Join(claudeDir, "skills", "extra-guide", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed plugin skill: %v", err)
	}
	if strings.Contains(string(skillData), "__HYLA__") {
		t.Fatalf("__HYLA__ placeholder was not expanded: %q", string(skillData))
	}

	// Hook injected alongside user hook.
	rawSettings, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(rawSettings, &parsed); err != nil {
		t.Fatal(err)
	}
	s := string(rawSettings)
	if !strings.Contains(s, "/usr/local/bin/user-stop-hook") {
		t.Fatalf("user hook was lost:\n%s", s)
	}
	if !strings.Contains(s, "hook plugin com.example.claude-ext claude Stop") {
		t.Fatalf("plugin hook missing:\n%s", s)
	}

	// Removing uninstalls plugin hook & skill while keeping user hook.
	if err := applyPluginAgentIntegrations(st, "claude", false); err != nil {
		t.Fatalf("applyPluginAgentIntegrations(remove): %v", err)
	}
	if _, err := os.Stat(filepath.Join(claudeDir, "skills", "extra-guide")); !os.IsNotExist(err) {
		t.Fatal("plugin skill directory should have been removed")
	}
	rawSettings, _ = os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	s = string(rawSettings)
	if strings.Contains(s, "hook plugin com.example.claude-ext") {
		t.Fatalf("plugin hook should have been removed:\n%s", s)
	}
	if !strings.Contains(s, "/usr/local/bin/user-stop-hook") {
		t.Fatalf("user hook should still exist:\n%s", s)
	}
}
