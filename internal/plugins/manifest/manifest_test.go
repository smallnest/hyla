package manifest

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"hyla/internal/actions"
	"hyla/internal/detect"
	"hyla/internal/proto"
)

func boolPtr(b bool) *bool { return &b }

func TestValidateID(t *testing.T) {
	valid := []string{
		"com.example.plugin",
		"sh.hyla.aider-pack",
		"io.github.user1.my-plugin-2",
		"hyla.magpie",
	}
	for _, id := range valid {
		if err := ValidateID(id); err != nil {
			t.Errorf("ValidateID(%q) unexpected error: %v", id, err)
		}
	}
	invalid := []string{
		"",
		"single",
		"com.example_bad.plugin",
		"Com.Example.Plugin",
		"-com.example.plugin",
		"com.-example.plugin",
		"com.example-.plugin",
		"a.b", // labels must be >=2 chars
		"com.example..plugin",
		strings.Repeat("a", 120) + ".plugin",
	}
	for _, id := range invalid {
		if err := ValidateID(id); err == nil {
			t.Errorf("ValidateID(%q) succeeded, expected error", id)
		}
	}
}

func TestManifestParseValidation(t *testing.T) {
	raw := []byte(`
api_version = 1
id = "com.acme.aider"
name = "Aider Support"
version = "0.2.0"

[engines]
hyla = ">=3.0.0"

[permissions]
scopes = ["agent.provider", "ui.slot", "ui.webview"]

[daemon]
command = ["bin/aider-plugin", "--stdio"]
transport = "stdio"
activation_events = ["onAgent:aider"]

[[agents]]
id = "aider"
name = "Aider"
detect = "detect/aider.toml"
mark = "marks/aider.svg"
timeline = true

[[actions]]
id = "open"
title = "Open Aider Tool"
shortcut = "CmdOrCtrl+Alt+A"
remote = true

[[ui.slots]]
id = "panel"
mount = "pane"
title = "Aider"
mode = "webview"
web_root = "web/dist"
entry = "index.html"
`)
	m, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if err := m.CompatibleWith("3.16.1"); err != nil {
		t.Fatalf("CompatibleWith failed: %v", err)
	}
	if err := m.CompatibleWith("2.1.0"); err == nil {
		t.Fatal("expected CompatibleWith(2.1.0) to fail")
	}

	// WebView without ui.webview permission fails
	badWeb := []byte(`
api_version = 1
id = "com.acme.badweb"
name = "Bad"
version = "0.1.0"
[[ui.slots]]
id = "w"
mount = "pane"
title = "W"
mode = "webview"
`)
	if _, err := Parse(badWeb); err == nil {
		t.Fatal("expected error for webview without ui.webview permission")
	}

	// Traversal path in detect fails
	badPath := []byte(`
api_version = 1
id = "com.acme.badpath"
name = "Bad"
version = "0.1.0"
[[agents]]
id = "x"
name = "X"
detect = "../secret.toml"
`)
	if _, err := Parse(badPath); err == nil {
		t.Fatal("expected error for escaping detect path")
	}

	// Settings mounts validation: settings.page, settings.plugins, and built-in settings.<page>
	validSettingsMounts := []string{
		"settings.page",
		"settings.plugins",
		"settings.general",
		"settings.appearance",
		"settings.terminal",
		"settings.shell",
		"settings.browser",
		"settings.notifications",
		"settings.keyboard",
		"settings.feishu",
		"settings.agents",
		"settings.sync",
		"settings.hosts",
		"settings.advanced",
	}
	for _, mt := range validSettingsMounts {
		doc := []byte(`
api_version = 1
id = "com.acme.setmount"
name = "Settings Mount"
version = "0.1.0"
[permissions]
scopes = ["ui.slot"]
[[ui.slots]]
id = "slot1"
mount = "` + mt + `"
title = "Custom"
mode = "declarative"
`)
		if _, err := Parse(doc); err != nil {
			t.Fatalf("Parse mount=%q failed: %v", mt, err)
		}
	}

	// Unknown settings mount fails
	badSetMount := []byte(`
api_version = 1
id = "com.acme.badset"
name = "Bad"
version = "0.1.0"
[[ui.slots]]
id = "s1"
mount = "settings.nonexistent"
title = "Bad"
`)
	if _, err := Parse(badSetMount); err == nil {
		t.Fatal("expected error for unknown settings mount")
	}

	// WebView mode on non-pane mounts fails
	for _, badMount := range []string{"settings.page", "info.card", "side.tab", "palette", "sidebar.menu"} {
		badSetWeb := []byte(`
api_version = 1
id = "com.acme.badsetweb"
name = "Bad"
version = "0.1.0"
[permissions]
scopes = ["ui.webview"]
[[ui.slots]]
id = "s1"
mount = "` + badMount + `"
title = "Bad"
mode = "webview"
`)
		if _, err := Parse(badSetWeb); err == nil {
			t.Fatalf("expected error for webview mode on %s", badMount)
		}
	}

	// Unsafe hook/skill/wash/pet identifiers rejected cleanly
	for _, badTOML := range []string{
		"[[hooks]]\nagent = \"claude; rm -rf /\"\nevent = \"Stop\"\nscript = \"h.sh\"\n",
		"[[hooks]]\nagent = \"claude\"\nevent = \"Stop && id\"\nscript = \"h.sh\"\n",
		"[[skills]]\nagent = \"claude\"\nname = \"../escape\"\ndir = \"sk\"\n",
		"[[washes]]\nid = \"bad wash\"\nname = \"Bad\"\nfile = \"w.toml\"\n",
		"[[pets]]\nid = \"pet;id\"\ndir = \"p\"\n",
	} {
		doc := append([]byte("api_version = 1\nid = \"com.acme.badtok\"\nname = \"Bad\"\nversion = \"0.1.0\"\n"), []byte(badTOML)...)
		if _, err := Parse(doc); err == nil {
			t.Fatalf("expected identifier validation failure for:\n%s", badTOML)
		}
	}
}

func TestStoreConfigSecretsAndPermissions(t *testing.T) {
	base := t.TempDir()
	st := &Store{BaseDir: base, UseKeychain: boolPtr(false)}

	id := "com.example.secret-test"
	if err := st.SaveConfig(id, map[string]any{"endpoint": "https://api.example.com", "limit": float64(42)}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	cfg, err := st.LoadConfig(id)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg["endpoint"] != "https://api.example.com" || cfg["limit"] != float64(42) {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}

	if err := st.SetSecret(id, "api_key", "sk-12345"); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	val, ok, err := st.GetSecret(id, "api_key")
	if err != nil || !ok || val != "sk-12345" {
		t.Fatalf("GetSecret = (%q, %v, %v)", val, ok, err)
	}
	sp, _ := st.SecretsPath(id)
	info, err := os.Stat(sp)
	if err != nil {
		t.Fatalf("stat secrets file: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("secrets file mode = %o, want 0600", info.Mode().Perm())
	}
	if err := st.DeleteSecret(id, "api_key"); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	if _, ok, err := st.GetSecret(id, "api_key"); err != nil || ok {
		t.Fatalf("expected deleted secret, got ok=%v err=%v", ok, err)
	}

	// Directory-discovered plugin supports SetPermission & GrantPermissions, and Remove(..., true) purges secrets and logs.
	dirPlugID := "com.example.dirplug"
	dirPlugRoot := filepath.Join(st.PluginsDir(), dirPlugID)
	_ = os.MkdirAll(dirPlugRoot, 0o755)
	_ = os.WriteFile(filepath.Join(dirPlugRoot, ManifestFileName), []byte(`
api_version = 1
id = "com.example.dirplug"
name = "Dir Plug"
version = "1.0.0"
[permissions]
scopes = ["session.read", "secrets"]
[[settings]]
key = "tok"
title = "Token"
type = "secret"
`), 0o644)
	if err := st.SetPermission(dirPlugID, PermSessionRead, true); err != nil {
		t.Fatalf("SetPermission on dir plugin: %v", err)
	}
	if err := st.SetSecret(dirPlugID, "tok", "secret-val"); err != nil {
		t.Fatal(err)
	}
	appLog, _ := st.AppLogPath(dirPlugID)
	_ = os.MkdirAll(filepath.Dir(appLog), 0o755)
	_ = os.WriteFile(appLog, []byte("app log\n"), 0o644)
	if err := st.Remove(dirPlugID, true); err != nil {
		t.Fatalf("Remove dir plugin: %v", err)
	}
	if _, err := os.Stat(appLog); !os.IsNotExist(err) {
		t.Fatalf("expected app log deleted by Remove(..., true), err=%v", err)
	}
	if _, ok, _ := st.GetSecret(dirPlugID, "tok"); ok {
		t.Fatal("expected secret purged by Remove(..., true)")
	}
}

func TestPackInstallAndSecurityChecks(t *testing.T) {
	t.Cleanup(func() {
		actions.ClearPluginActions()
		proto.ClearPluginAgents()
		detect.ClearPlugins()
	})

	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "detect"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestTOML := `
api_version = 1
id = "com.example.aider-pack"
name = "Aider Pack"
version = "1.2.3"

[permissions]
scopes = ["ui.slot"]

[[agents]]
id = "aider"
name = "Aider"
detect = "detect/aider.toml"

[[actions]]
id = "status"
title = "Aider Status"
`
	if err := os.WriteFile(filepath.Join(src, ManifestFileName), []byte(manifestTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "detect", "aider.toml"), []byte("[[rules]]\nid=\"aider.work\"\nstate=\"working\"\ncontains=[\"Thinking\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "bin", "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	outZip := filepath.Join(t.TempDir(), "aider.hylapkg")
	packRes, err := Pack(src, outZip)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if packRes.Files != 3 || packRes.SHA256 == "" {
		t.Fatalf("unexpected Pack result: %+v", packRes)
	}

	storeDir := t.TempDir()
	st := &Store{BaseDir: storeDir, UseKeychain: boolPtr(false)}
	inst, err := st.InstallPackage(outZip, InstallOptions{Enable: true, GrantAll: true})
	if err != nil {
		t.Fatalf("InstallPackage: %v", err)
	}
	if inst.Entry.SHA256 != packRes.SHA256 || !inst.Entry.HasPermission("ui.slot") {
		t.Fatalf("unexpected installed entry: %+v", inst.Entry)
	}
	scriptInfo, err := os.Stat(filepath.Join(inst.RootDir, "bin", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && scriptInfo.Mode().Perm()&0o100 == 0 {
		t.Fatalf("executable bit lost on run.sh: mode=%o", scriptInfo.Mode().Perm())
	}

	snap, err := ApplyBase(st)
	if err != nil {
		t.Fatalf("ApplyBase: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected snap errors: %v", snap.Errors)
	}
	if proto.AgentDisplayName("aider") != "Aider" {
		t.Fatalf("expected aider registered in proto, got %q", proto.AgentDisplayName("aider"))
	}
	if res := detect.Detect("aider", detect.Input{Screen: "Thinking now"}); res.State != detect.Working {
		t.Fatalf("expected aider working, got %+v", res)
	}
	if _, ok := actions.Find("com.example.aider-pack.status"); !ok {
		t.Fatal("expected qualified plugin action registered")
	}

	// Disable plugin: base action moves to disabled list, agent removed.
	if err := st.Enable("com.example.aider-pack", false); err != nil {
		t.Fatal(err)
	}
	snap, err = ApplyBase(st)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := actions.Find("com.example.aider-pack.status"); ok {
		t.Fatal("disabled plugin action should not be active")
	}
	if !actions.IsPluginActionID("com.example.aider-pack.status") {
		t.Fatal("disabled plugin action should still pass IsPluginActionID")
	}
	if detect.Known("aider") {
		t.Fatal("disabled plugin detect should not remain active")
	}

	// Reject zip traversal & symlink archives.
	badTraversal := writeCustomZip(t, func(zw *zip.Writer) {
		addZipEntry(t, zw, "../escape.txt", []byte("bad"), 0o644)
		addZipEntry(t, zw, ManifestFileName, []byte(manifestTOML), 0o644)
	})
	if _, err := st.InstallPackage(badTraversal, InstallOptions{Enable: true}); err == nil {
		t.Fatal("expected path traversal archive to be rejected")
	}

	badSymlink := writeCustomZip(t, func(zw *zip.Writer) {
		hdr := &zip.FileHeader{Name: "evil-link"}
		hdr.SetMode(fs.ModeSymlink | 0o777)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("/etc/passwd"))
		addZipEntry(t, zw, ManifestFileName, []byte(manifestTOML), 0o644)
	})
	if _, err := st.InstallPackage(badSymlink, InstallOptions{Enable: true}); err == nil {
		t.Fatal("expected symlink archive to be rejected")
	}
}

func writeCustomZip(t *testing.T, fn func(*zip.Writer)) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fn(zw)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "test.hylapkg")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func addZipEntry(t *testing.T, zw *zip.Writer, name string, data []byte, mode fs.FileMode) {
	t.Helper()
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
	hdr.SetMode(mode)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
}

// A builtin's manifest is written once, in the source language, and the
// words a language has of it are what the app asks for there: one that
// says nothing of a language, or has no words of its own, says the
// source's.
func TestBuiltinWords(t *testing.T) {
	source := &Manifest{ID: "hyla.board", Name: "Board", Description: "A board"}
	french := &Manifest{ID: "hyla.board", Name: "Board", Description: "Un tableau"}
	b := BuiltinPlugin{Manifest: source, Words: func(locale string) *Manifest {
		if locale == "fr" {
			return french
		}
		return nil
	}}
	if got := b.InLanguage("fr"); got != french {
		t.Errorf("fr says %q", got.Description)
	}
	if got := b.InLanguage("en"); got != source {
		t.Errorf("en says %q", got.Description)
	}
	if got := (BuiltinPlugin{Manifest: source}).InLanguage("fr"); got != source {
		t.Errorf("a builtin of one language says %q", got.Description)
	}
}

func TestManifestI18nResolutionAndApplyBase(t *testing.T) {
	tomlSrc := `
api_version = 1
id = "com.example.i18n"
name = "Timeline"
description = "Default EN desc"
version = "1.0.0"

[[actions]]
id = "open"
title = "Open Timeline Pane"
target = "app"

[[ui.slots]]
id = "timeline"
mount = "pane"
title = "Timeline"

[[settings]]
key = "url"
title = "Gateway URL"
description = "EN URL desc"
type = "string"
placeholder = "http://localhost"

[i18n.zh-Hans]
name = "时间线"
description = "中文描述"

[i18n.zh-Hans.actions]
open = "打开时间线窗格"

[i18n.zh-Hans.slots]
timeline = "时间线"

[i18n.zh-Hans.settings.url]
title = "网关地址"
description = "中文地址描述"
placeholder = "请输入地址"

[i18n.ja]
name = "タイムライン"
description = "日本語の説明"

[i18n.ja.actions]
open = "タイムラインペインを開く"

[i18n.ja.slots]
timeline = "タイムライン"
`
	m, err := Parse([]byte(tomlSrc))
	if err != nil {
		t.Fatalf("Parse manifest: %v", err)
	}

	// Exact + alias lookup for zh-Hans / zh-CN / zh.
	for _, loc := range []string{"zh-Hans", "zh-CN", "zh_cn", "zh"} {
		zh := m.InLanguage(loc)
		if zh.Name != "时间线" || zh.Description != "中文描述" ||
			zh.Actions[0].Title != "打开时间线窗格" ||
			zh.UI.Slots[0].Title != "时间线" ||
			zh.Settings[0].Title != "网关地址" ||
			zh.Settings[0].Description != "中文地址描述" ||
			zh.Settings[0].Placeholder != "请输入地址" {
			t.Fatalf("locale %q resolution mismatch: %+v", loc, zh)
		}
	}

	// Prefix fallback ja-JP -> ja.
	ja := m.InLanguage("ja-JP")
	if ja.Name != "タイムライン" || ja.Actions[0].Title != "タイムラインペインを開く" || ja.Settings[0].Title != "Gateway URL" {
		t.Fatalf("locale ja-JP fallback mismatch: %+v", ja)
	}

	// Source manifest remains completely untouched.
	if m.Name != "Timeline" || m.Actions[0].Title != "Open Timeline Pane" || m.Settings[0].Title != "Gateway URL" {
		t.Fatalf("source manifest was mutated: %+v", m)
	}

	// ApplyBaseInLanguage populates actions & Snapshot in requested locale.
	base := t.TempDir()
	useKC := false
	st := &Store{BaseDir: base, UseKeychain: &useKC}
	pDir := filepath.Join(base, "plg")
	if err := os.MkdirAll(pDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDir, ManifestFileName), []byte(tomlSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Link(pDir, true); err != nil {
		t.Fatal(err)
	}
	snap, err := ApplyBaseInLanguage(st, "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Plugins) != 1 || snap.Plugins[0].Manifest.Name != "时间线" {
		t.Fatalf("ApplyBaseInLanguage did not localize snapshot plugin: %+v", snap.Plugins)
	}
	if meta := snap.ActionTargets["com.example.i18n.open"]; meta.Action.Title != "打开时间线窗格" {
		t.Fatalf("ApplyBaseInLanguage did not localize action target: %+v", meta)
	}
}
