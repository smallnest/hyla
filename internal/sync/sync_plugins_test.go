package sync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"hyla/internal/plugins/manifest"
	"hyla/internal/settings"
)

func boolPtrSync(b bool) *bool { return &b }

type pluginSyncMachine struct {
	t    *testing.T
	home string
	cfg  Config
	host string
}

func newPluginSyncMachine(t *testing.T, host, url string) *pluginSyncMachine {
	t.Helper()
	home := t.TempDir()
	return &pluginSyncMachine{
		t:    t,
		home: home,
		host: host,
		cfg: Config{
			Backend:    "webdav",
			WebDAV:     WebDAV{URL: url},
			Passphrase: "sync secret phrase",
		},
	}
}

func (m *pluginSyncMachine) store() *manifest.Store {
	return &manifest.Store{BaseDir: m.home, UseKeychain: boolPtrSync(false)}
}

func (m *pluginSyncMachine) configPath() string { return filepath.Join(m.home, "config") }

func (m *pluginSyncMachine) writeConfig(s string) {
	m.t.Helper()
	if err := os.WriteFile(m.configPath(), []byte(s), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

func (m *pluginSyncMachine) readConfig() string {
	m.t.Helper()
	b, err := os.ReadFile(m.configPath())
	if err != nil {
		m.t.Fatal(err)
	}
	return string(b)
}

func (m *pluginSyncMachine) run(way string) Result {
	m.t.Helper()
	m.t.Setenv("HYLA_HOME", m.home)
	m.t.Setenv("HYLA_CONFIG", m.configPath())
	m.t.Setenv("HYLA_NO_KEYCHAIN", "1")
	settings.Reload()
	res, cfg, err := Run(context.Background(), m.cfg, way, m.host)
	if err != nil {
		m.t.Fatalf("%s (%q): %v", m.host, way, err)
	}
	m.cfg = cfg
	return res
}

func TestSyncPluginsStripsLinksMarksPendingAndExcludesSecrets(t *testing.T) {
	var mu sync.Mutex
	var rawServerBlob []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			if rawServerBlob == nil {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(rawServerBlob)
		case http.MethodPut:
			b, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			rawServerBlob = b
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, r.Method, http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	machA := newPluginSyncMachine(t, "laptop-a", srv.URL+"/dav/")
	machB := newPluginSyncMachine(t, "desk-b", srv.URL+"/dav/")

	machA.writeConfig("theme = hyla\nfont-size = 13\n")
	stA := machA.store()
	stB := machB.store()

	// Create linked plugin on Machine A only.
	linkedDir := filepath.Join(t.TempDir(), "linked-dev")
	if err := os.MkdirAll(linkedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linkedDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "com.example.linked"
name = "Linked Plugin"
version = "1.2.3"

[permissions]
scopes = ["ui.slot"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := stA.Link(linkedDir, true); err != nil {
		t.Fatal(err)
	}

	// Create packaged plugin unpacked locally on BOTH Machine A and Machine B.
	for _, st := range []*manifest.Store{stA, stB} {
		pkgDir, err := st.PluginDir("com.example.shared")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "com.example.shared"
name = "Shared Plugin"
version = "2.0.0"

[permissions]
scopes = ["ui.slot"]
`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := stA.SaveRegistry(&manifest.Registry{
		Version: 1,
		Plugins: map[string]manifest.RegistryEntry{
			"com.example.linked": {
				ID:                 "com.example.linked",
				Enabled:            true,
				Version:            "1.2.3",
				Source:             "link",
				LinkPath:           linkedDir,
				GrantedPermissions: []string{"ui.slot"},
			},
			"com.example.shared": {
				ID:                 "com.example.shared",
				Enabled:            true,
				Version:            "2.0.0",
				Source:             "package",
				SHA256:             "deadbeef",
				GrantedPermissions: []string{"ui.slot"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Non-secret config and secret on Machine A.
	if err := stA.SaveConfig("com.example.linked", map[string]any{"endpoint": "https://api.example.com", "limit": 42}); err != nil {
		t.Fatal(err)
	}
	if err := stA.SetSecret("com.example.linked", "apiKey", "TOP_SECRET_DO_NOT_SYNC"); err != nil {
		t.Fatal(err)
	}

	// Machine A pushes initial snapshot.
	if res := machA.run(""); res.Action != "created" {
		t.Fatalf("Machine A initial sync = %q, want created", res.Action)
	}

	// Inspect decrypted snapshot: Version must remain 1, LinkPath stripped, Secrets absent!
	mu.Lock()
	blob := append([]byte(nil), rawServerBlob...)
	mu.Unlock()
	snap, err := open("sync secret phrase", blob)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Version != 1 {
		t.Fatalf("Snapshot.Version = %d, want 1", snap.Version)
	}
	if snap.Plugins == nil {
		t.Fatal("expected Snapshot.Plugins populated")
	}
	if gotLink := snap.Plugins.Plugins["com.example.linked"].LinkPath; gotLink != "" {
		t.Fatalf("expected LinkPath stripped in snapshot, got %q", gotLink)
	}
	snapJSON, _ := json.Marshal(snap)
	if strings.Contains(string(snapJSON), "TOP_SECRET_DO_NOT_SYNC") || strings.Contains(string(snapJSON), linkedDir) {
		t.Fatalf("snapshot leaked secret or local link path: %s", snapJSON)
	}
	if len(snap.PluginConfig["com.example.linked"]) == 0 {
		t.Fatalf("expected non-secret plugin-config synced, got %+v", snap.PluginConfig)
	}

	// Machine B pulls: shared plugin is enabled, missing linked plugin becomes pendingInstall=true & disabled, config synced, secret excluded!
	if res := machB.run(""); res.Action != "pulled" {
		t.Fatalf("Machine B initial sync = %q, want pulled", res.Action)
	}
	regB, err := stB.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	sharedB := regB.Plugins["com.example.shared"]
	if !sharedB.Enabled || sharedB.PendingInstall {
		t.Fatalf("expected com.example.shared enabled & installed on B, got %+v", sharedB)
	}
	linkedB := regB.Plugins["com.example.linked"]
	if !linkedB.PendingInstall || linkedB.Enabled || linkedB.LinkPath != "" {
		t.Fatalf("expected com.example.linked marked pendingInstall=true, disabled, no linkPath on B, got %+v", linkedB)
	}
	cfgB, err := stB.LoadConfig("com.example.linked")
	if err != nil || cfgB["endpoint"] != "https://api.example.com" {
		t.Fatalf("expected plugin-config synced to B, got %+v err=%v", cfgB, err)
	}
	if val, ok, err := stB.GetSecret("com.example.linked", "apiKey"); err == nil && ok && val != "" {
		t.Fatalf("secret must NOT sync to B, got %q", val)
	}

	// Second sync on Machine B with no local changes must be "unchanged", preserving remote state.
	if res := machB.run(""); res.Action != "unchanged" {
		t.Fatalf("Machine B second sync = %q, want unchanged", res.Action)
	}

	// Machine B updates config only and pushes; Machine A pulls new config WITHOUT losing its enabled com.example.linked or local LinkPath!
	machB.writeConfig("theme = hyla\nfont-size = 18\n")
	if res := machB.run(""); res.Action != "pushed" {
		t.Fatalf("Machine B config change sync = %q, want pushed", res.Action)
	}
	if res := machA.run(""); res.Action != "pulled" {
		t.Fatalf("Machine A pull after B change = %q, want pulled", res.Action)
	}
	if !strings.Contains(machA.readConfig(), "font-size = 18") {
		t.Fatalf("Machine A config did not update: %s", machA.readConfig())
	}
	regA, err := stA.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	linkedA := regA.Plugins["com.example.linked"]
	if !linkedA.Enabled || linkedA.PendingInstall || linkedA.LinkPath != linkedDir {
		t.Fatalf("Machine A lost local link/enabled state after pull from B: %+v", linkedA)
	}

	// Changing ONLY plugin-config triggers push/pull even when main config file is unchanged.
	if err := stA.SaveConfig("com.example.linked", map[string]any{"endpoint": "https://v2.example.com", "limit": 99}); err != nil {
		t.Fatal(err)
	}
	if res := machA.run(""); res.Action != "pushed" {
		t.Fatalf("plugin-config-only edit on A = %q, want pushed", res.Action)
	}
	if res := machB.run(""); res.Action != "pulled" {
		t.Fatalf("Machine B sync after plugin-config edit = %q, want pulled", res.Action)
	}
	cfgB, err = stB.LoadConfig("com.example.linked")
	if err != nil || cfgB["endpoint"] != "https://v2.example.com" {
		t.Fatalf("updated plugin-config did not arrive on B: %+v err=%v", cfgB, err)
	}

	// Removing all non-builtin plugins & configs on A syncs deletions to B cleanly.
	_ = stA.Remove("com.example.shared", true)
	_ = stA.Remove("com.example.linked", true)
	if res := machA.run(""); res.Action != "pushed" {
		t.Fatalf("Machine A push after removing plugins = %q, want pushed", res.Action)
	}
	if res := machB.run(""); res.Action != "pulled" {
		t.Fatalf("Machine B pull after removing plugins = %q, want pulled", res.Action)
	}
	regB, _ = stB.LoadRegistry()
	if len(regB.Plugins) != 0 {
		t.Fatalf("expected all removed plugins cleared on B, got %+v", regB.Plugins)
	}
	if cfgPathB, _ := stB.ConfigPath("com.example.linked"); cfgPathB != "" {
		if _, err := os.Stat(cfgPathB); !os.IsNotExist(err) {
			t.Fatalf("expected deleted plugin-config removed on B, stat err=%v", err)
		}
	}

	// Conflict keeps both config.remote-* and *.plugins.json backup!
	machB.writeConfig("font-size = 20\n")
	if err := stB.SaveConfig("com.example.conflict", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if res := machB.run(""); res.Action != "pushed" {
		t.Fatalf("Machine B conflict setup push = %q", res.Action)
	}
	machA.writeConfig("font-size = 16\n")
	if err := stA.SaveConfig("com.example.conflict", map[string]any{"y": 2}); err != nil {
		t.Fatal(err)
	}
	resConflict := machA.run("")
	if resConflict.Backup == "" {
		t.Fatal("expected conflict backup path")
	}
	if _, err := os.Stat(resConflict.Backup + ".plugins.json"); err != nil {
		t.Fatalf("expected companion .plugins.json backup: %v", err)
	}
}
