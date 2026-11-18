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

func TestPluginAgentIntegrationsCrossAgentAndNoOpAvoidsWrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HYLA_HOME", filepath.Join(home, ".hyla"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, ".pi", "agent"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	pluginDir := filepath.Join(home, "multi-plugin")
	_ = os.MkdirAll(filepath.Join(pluginDir, "hooks"), 0o755)
	_ = os.MkdirAll(filepath.Join(pluginDir, "skills", "cl-sk"), 0o755)
	_ = os.MkdirAll(filepath.Join(pluginDir, "skills", "ag-sk"), 0o755)
	_ = os.WriteFile(filepath.Join(pluginDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "com.example.multi"
name = "Multi Agent"
version = "1.0.0"

[[hooks]]
agent = "claude"
event = "Stop"
script = "hooks/claude.sh"

[[hooks]]
agent = "pi"
event = "session_start"
script = "hooks/pi.ts"

[[skills]]
agent = "claude"
name = "cl-sk"
dir = "skills/cl-sk"

[[skills]]
agent = "codex"
name = "ag-sk"
dir = "skills/ag-sk"
`), 0o644)
	_ = os.WriteFile(filepath.Join(pluginDir, "hooks", "claude.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755)
	_ = os.WriteFile(filepath.Join(pluginDir, "hooks", "pi.ts"), []byte("export default () => {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(pluginDir, "skills", "cl-sk", "SKILL.md"), []byte("claude\n"), 0o644)
	_ = os.WriteFile(filepath.Join(pluginDir, "skills", "ag-sk", "SKILL.md"), []byte("agents\n"), 0o644)

	st := &manifest.Store{BaseDir: t.TempDir()}
	if _, err := st.Link(pluginDir, true); err != nil {
		t.Fatal(err)
	}

	// Only claude base skill installed -> syncExisting must NOT install agents/pi assets.
	if err := writeSkill(filepath.Join(home, ".claude", "skills", "hyla")); err != nil {
		t.Fatal(err)
	}
	if err := syncExistingPluginAgentIntegrations(st); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "cl-sk", "SKILL.md")); err != nil {
		t.Fatalf("expected claude plugin skill installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "ag-sk")); !os.IsNotExist(err) {
		t.Fatal("must not install agents plugin skill when base agents skill is missing")
	}

	// Install pi hook via explicit "pi" filter, then remove via "claude" filter -> pi hook file MUST stay!
	if err := applyPluginAgentIntegrations(st, "pi", true); err != nil {
		t.Fatal(err)
	}
	piHook := filepath.Join(home, ".pi", "agent", "extensions", "hyla-plugin-com.example.multi-pi.ts")
	if _, err := os.Stat(piHook); err != nil {
		t.Fatalf("expected pi hook installed: %v", err)
	}
	if err := applyPluginAgentIntegrations(st, "claude", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(piHook); err != nil {
		t.Fatalf("claude filter deleted pi hook file: %v", err)
	}

	// Re-sync existing targets first, then verify a subsequent repeat no-op does not touch state file.
	if err := syncExistingPluginAgentIntegrations(st); err != nil {
		t.Fatal(err)
	}
	statePath := pluginAgentAssetsStatePath(st)
	beforeInfo, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncExistingPluginAgentIntegrations(st); err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatalf("expected zero writes to %s on no-op sync", statePath)
	}

	// execSinglePluginHook quotes binary paths containing spaces safely.
	spaceHome := filepath.Join(home, "dir with spaces")
	_ = os.MkdirAll(filepath.Join(spaceHome, "bin"), 0o755)
	fakeBin := filepath.Join(spaceHome, "bin", "hyla")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\necho ran\n"), 0o755)
	hookScript := filepath.Join(pluginDir, "hooks", "spaced.sh")
	_ = os.WriteFile(hookScript, []byte("#!/bin/sh\nout=$(__HYLA__)\n[ \"$out\" = \"ran\" ]\n"), 0o755)

	t.Setenv("HYLA_HOME", spaceHome)
	if err := execSinglePluginHook("com.example.multi", pluginDir, "claude", "Stop", hookScript, nil); err != nil {
		t.Fatalf("execSinglePluginHook with spaces in HYLA_HOME failed: %v", err)
	}
	if quoted := shellQuote("/tmp/my app/hyla"); quoted != "'/tmp/my app/hyla'" && quoted != `"/tmp/my app/hyla"` {
		t.Fatalf("unexpected shellQuote: %q", quoted)
	}
}
