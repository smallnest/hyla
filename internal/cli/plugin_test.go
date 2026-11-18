package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"hyla/internal/actions"
	"hyla/internal/detect"
	"hyla/internal/plugins/manifest"
	"hyla/internal/proto"
	"hyla/internal/ui/wash"
)

func TestCLIPluginInitPackInstallPermissionsRemove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")

	work := t.TempDir()
	packDir := filepath.Join(work, "sample-pack")
	if err := pluginCmd([]string{"init", "--template", "pack", "--id", "com.example.pack", "--name", "Sample Pack", packDir}); err != nil {
		t.Fatalf("init pack: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("missing manifest: %v", err)
	}

	goDir := filepath.Join(work, "sample-go")
	if err := pluginCmd([]string{"init", "--template", "go", "--id", "com.example.go", goDir}); err != nil {
		t.Fatalf("init go: %v", err)
	}
	if _, err := os.Stat(filepath.Join(goDir, "main.go")); err != nil {
		t.Fatalf("missing go main.go: %v", err)
	}

	tsDir := filepath.Join(work, "sample-ts")
	if err := pluginCmd([]string{"init", "--template", "ts-webview", "--id", "com.example.ts", tsDir}); err != nil {
		t.Fatalf("init ts-webview: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tsDir, "web", "index.html")); err != nil {
		t.Fatalf("missing web/index.html: %v", err)
	}

	// Pack & install ts-webview plugin without granting permissions initially.
	pkgPath := filepath.Join(work, "sample-ts.hylapkg")
	if err := pluginCmd([]string{"pack", tsDir, "-o", pkgPath}); err != nil {
		t.Fatalf("pack: %v", err)
	}
	if err := pluginCmd([]string{"install", pkgPath}); err != nil {
		t.Fatalf("install: %v", err)
	}

	st := manifest.DefaultStore()
	reg, err := st.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := reg.Plugins["com.example.ts"]
	if !ok || !entry.Enabled {
		t.Fatalf("expected com.example.ts installed & enabled, got %+v ok=%v", entry, ok)
	}
	if len(entry.GrantedPermissions) != 0 {
		t.Fatalf("expected 0 granted permissions without --grant-all, got %v", entry.GrantedPermissions)
	}

	// Grant permission scope via CLI.
	if err := pluginCmd([]string{"permissions", "com.example.ts", "grant", manifest.PermUIWebView}); err != nil {
		t.Fatalf("permissions grant: %v", err)
	}
	reg, _ = st.LoadRegistry()
	if !slices.Contains(reg.Plugins["com.example.ts"].GrantedPermissions, manifest.PermUIWebView) {
		t.Fatalf("expected ui.webview granted, got %v", reg.Plugins["com.example.ts"].GrantedPermissions)
	}

	// Disable, re-enable, check logs cleanly, and remove.
	if err := pluginCmd([]string{"disable", "com.example.ts"}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	reg, _ = st.LoadRegistry()
	if reg.Plugins["com.example.ts"].Enabled {
		t.Fatal("expected plugin disabled")
	}

	if err := pluginCmd([]string{"logs", "com.example.ts", "-n", "10"}); err != nil {
		t.Fatalf("logs: %v", err)
	}
	if err := pluginCmd([]string{"ls", "--json"}); err != nil {
		t.Fatalf("ls --json: %v", err)
	}
	if err := pluginCmd([]string{"rm", "com.example.ts", "--purge"}); err != nil {
		t.Fatalf("rm: %v", err)
	}
	reg, _ = st.LoadRegistry()
	if _, exists := reg.Plugins["com.example.ts"]; exists {
		t.Fatal("expected com.example.ts removed from registry")
	}
}

func TestPackTemplateManifestWashAndDetectParseCleanly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")
	t.Cleanup(func() {
		actions.ClearPluginActions()
		proto.ClearPluginAgents()
		detect.ClearPlugins()
		_ = wash.ReloadPlugins(nil)
	})

	packDir := filepath.Join(t.TempDir(), "pack")
	if err := pluginCmd([]string{"init", "--template", "pack", "--id", "com.example.packcheck", "--name", "Pack Check", packDir}); err != nil {
		t.Fatalf("init pack: %v", err)
	}
	m, err := manifest.LoadFile(packDir)
	if err != nil {
		t.Fatalf("manifest.LoadFile: %v", err)
	}
	if m.ID != "com.example.packcheck" {
		t.Fatalf("manifest id = %q", m.ID)
	}

	st := manifest.DefaultStore()
	if _, err := st.Link(packDir, true); err != nil {
		t.Fatalf("Link: %v", err)
	}
	snap, err := manifest.ApplyBase(st)
	if err != nil {
		t.Fatalf("ApplyBase: %v", err)
	}
	if len(snap.Errors) > 0 {
		t.Fatalf("ApplyBase snap.Errors = %v", snap.Errors)
	}
	if err := wash.ReloadPlugins(snap.Washes); err != nil {
		t.Fatalf("wash.ReloadPlugins: %v", err)
	}
	foundWash := false
	for _, w := range wash.PluginList() {
		if w.ID == "aurora-mist" {
			foundWash = true
		}
	}
	if !foundWash {
		t.Fatal("expected aurora-mist wash loaded from pack scaffold")
	}
	res := detect.Detect("sample-agent", detect.Input{Screen: "Allow this tool? Approve?"})
	if res.State != detect.Blocked || !res.Visible {
		t.Fatalf("unexpected detect result for pack scaffold: %+v", res)
	}
}
