package market

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"hyla/internal/plugins/manifest"
)

func TestEmbeddedCatalogValid(t *testing.T) {
	rootBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin_market.json"))
	if err != nil {
		t.Fatalf("read root plugin_market.json: %v", err)
	}
	if !bytes.Equal(embeddedPluginMarket, rootBytes) {
		t.Fatal("internal/plugins/market/plugin_market.json is out of sync with repository root plugin_market.json")
	}
	cat, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded() error: %v", err)
	}
	if cat.Version != 1 || len(cat.Plugins) == 0 {
		t.Fatalf("unexpected embedded catalog: %+v", cat)
	}

	for _, spec := range []struct {
		id     string
		subdir string
	}{
		{"hyla.timeline", "timeline"},
		{"hyla.podcast", "podcast"},
		{"hyla.iptv", "iptv"},
	} {
		var entry *Plugin
		for i := range cat.Plugins {
			if cat.Plugins[i].ID == spec.id {
				entry = &cat.Plugins[i]
				break
			}
		}
		if entry == nil {
			t.Fatalf("embedded catalog missing %s", spec.id)
		}

		for _, plat := range []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"} {
			goos, goarch, _ := strings.Cut(plat, "/")
			dl, err := entry.ResolveDownload(goos, goarch)
			if err != nil {
				t.Fatalf("ResolveDownload(%s, %s): %v", spec.id, plat, err)
			}
			wantSuffix := "/" + spec.id + "-" + entry.Version + "-" + goos + "-" + goarch + ".hylapkg"
			if !strings.HasSuffix(dl.URL, wantSuffix) {
				t.Errorf("%s URL(%s) = %q, want suffix %q", spec.id, plat, dl.URL, wantSuffix)
			}
			if !strings.HasSuffix(dl.SHA256URL, wantSuffix+".sha256") {
				t.Errorf("%s SHA256URL(%s) = %q, want suffix %q", spec.id, plat, dl.SHA256URL, wantSuffix+".sha256")
			}
		}

		// Cross-check root plugins/<subdir>/hyla-plugin.toml against plugin_market.json.
		mf, err := manifest.LoadFile(filepath.Join("..", "..", "..", "plugins", spec.subdir))
		if err != nil {
			t.Fatalf("load %s plugin manifest: %v", spec.subdir, err)
		}
		if mf.ID != entry.ID || mf.Version != entry.Version {
			t.Errorf("%s manifest mismatch: toml=(%s %s) market=(%s %s)", spec.subdir, mf.ID, mf.Version, entry.ID, entry.Version)
		}
		if !slices.Equal(mf.Permissions.Scopes, entry.Permissions) {
			t.Errorf("%s permissions mismatch: toml=%v market=%v", spec.subdir, mf.Permissions.Scopes, entry.Permissions)
		}
	}
}

func TestParseAndLocalization(t *testing.T) {
	raw := []byte(`{
  "version": 1,
  "plugins": [
    {
      "id": "org.example.demo",
      "name": "Demo",
      "author": "Alice",
      "description": "English desc",
      "homepage": "https://example.com/demo",
      "version": "1.2.0",
      "permissions": ["session.read"],
      "i18n": {
        "zh-Hans": { "name": "演示", "description": "中文描述" },
        "ja": { "name": "デモ", "description": "日本語説明" }
      },
      "download_url": "https://example.com/{id}-{version}-{os}-{arch}.hylapkg"
    }
  ]
}`)
	cat, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse valid: %v", err)
	}

	zh := cat.InLanguage("zh-CN")
	if zh.Plugins[0].Name != "演示" || zh.Plugins[0].Description != "中文描述" {
		t.Errorf("zh-CN localization = %+v", zh.Plugins[0])
	}
	ja := cat.InLanguage("ja-JP")
	if ja.Plugins[0].Name != "デモ" || ja.Plugins[0].Description != "日本語説明" {
		t.Errorf("ja-JP localization = %+v", ja.Plugins[0])
	}
	// Original catalog remains unmodified.
	if cat.Plugins[0].Name != "Demo" {
		t.Errorf("source catalog mutated: %+v", cat.Plugins[0])
	}

	if !HasUpdate("1.0.0", "1.2.0") || HasUpdate("1.2.0", "1.2.0") || HasUpdate("2.0.0", "1.2.0") {
		t.Errorf("unexpected HasUpdate results")
	}

	invalidCases := []struct {
		name string
		json string
	}{
		{"bad_version", `{"version":2,"plugins":[]}`},
		{"bad_id", `{"version":1,"plugins":[{"id":"bad","name":"X","version":"1.0.0","download_url":"https://x/a.hylapkg"}]}`},
		{"bad_scheme", `{"version":1,"plugins":[{"id":"org.ex.a","name":"X","version":"1.0.0","download_url":"file:///a.hylapkg"}]}`},
		{"bad_perm", `{"version":1,"plugins":[{"id":"org.ex.a","name":"X","version":"1.0.0","permissions":["nope"],"download_url":"https://x/a.hylapkg"}]}`},
		{"duplicate", `{"version":1,"plugins":[{"id":"org.ex.a","name":"X","version":"1.0.0","download_url":"https://x/a.hylapkg"},{"id":"org.ex.a","name":"Y","version":"1.0.0","download_url":"https://x/b.hylapkg"}]}`},
	}
	for _, tc := range invalidCases {
		if _, err := Parse([]byte(tc.json)); err == nil {
			t.Errorf("%s: expected Parse error", tc.name)
		}
	}
}

func TestDownloadAndInstall(t *testing.T) {
	packPlugin := func(id, version string, perms []string) (string, string) {
		t.Helper()
		src := t.TempDir()
		toml := "api_version = 1\nid = \"" + id + "\"\nname = \"Test Plugin\"\nversion = \"" + version + "\"\n[permissions]\nscopes = [\"" + strings.Join(perms, "\", \"") + "\"]\n"
		if err := os.WriteFile(filepath.Join(src, manifest.ManifestFileName), []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), id+"-"+version+".hylapkg")
		res, err := manifest.Pack(src, out)
		if err != nil {
			t.Fatal(err)
		}
		return out, res.SHA256
	}

	goodPkg, goodSHA := packPlugin("org.example.good", "1.1.0", []string{"session.read", "pane.write"})
	goodBytes, err := os.ReadFile(goodPkg)
	if err != nil {
		t.Fatal(err)
	}

	otherPkg, otherSHA := packPlugin("org.example.other", "1.0.0", []string{"session.read"})
	otherBytes, err := os.ReadFile(otherPkg)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/market.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":1,"plugins":[{"id":"org.example.good","name":"Remote Good","version":"1.1.0","download_url":"https://example.com/a.hylapkg"}]}`))
	})
	mux.HandleFunc("/good.hylapkg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(goodBytes)
	})
	mux.HandleFunc("/good.hylapkg.sha256", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(goodSHA + "  good.hylapkg\n"))
	})
	mux.HandleFunc("/bad.hylapkg.sha256", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("0", 64) + "  good.hylapkg\n"))
	})
	mux.HandleFunc("/other.hylapkg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(otherBytes)
	})
	mux.HandleFunc("/other.hylapkg.sha256", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(otherSHA + "  other.hylapkg\n"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &Client{HTTP: srv.Client(), URL: srv.URL + "/market.json"}

	// FetchWithFallback remote success.
	cat, remote, err := client.FetchWithFallback(context.Background())
	if err != nil || !remote || len(cat.Plugins) != 1 || cat.Plugins[0].Name != "Remote Good" {
		t.Fatalf("FetchWithFallback remote = (%+v, remote=%v, err=%v)", cat, remote, err)
	}

	// FetchWithFallback offline fallback to embedded catalog.
	downClient := &Client{HTTP: srv.Client(), URL: srv.URL + "/missing.json"}
	fallbackCat, remote, err := downClient.FetchWithFallback(context.Background())
	if err == nil || remote || fallbackCat == nil || len(fallbackCat.Plugins) == 0 {
		t.Fatalf("FetchWithFallback offline = (%+v, remote=%v, err=%v)", fallbackCat, remote, err)
	}

	// Happy path install: only market-approved permission "session.read" is granted, NOT extra "pane.write".
	store := &manifest.Store{BaseDir: t.TempDir()}
	goodEntry := Plugin{
		ID:          "org.example.good",
		Name:        "Good",
		Version:     "1.1.0",
		Permissions: []string{"session.read"},
		DownloadURL: srv.URL + "/good.hylapkg",
		SHA256URL:   srv.URL + "/good.hylapkg.sha256",
	}
	inst, err := client.DownloadAndInstall(context.Background(), store, goodEntry, "darwin", "arm64", "3.18.0")
	if err != nil {
		t.Fatalf("DownloadAndInstall happy path failed: %v", err)
	}
	if !inst.Entry.Enabled || inst.Entry.Version != "1.1.0" {
		t.Fatalf("unexpected installed entry: %+v", inst.Entry)
	}
	if !slices.Equal(inst.Entry.GrantedPermissions, []string{"session.read"}) {
		t.Fatalf("expected only [session.read] granted, got %v", inst.Entry.GrantedPermissions)
	}

	// Checksum mismatch rejected.
	badSHAEntry := goodEntry
	badSHAEntry.SHA256URL = srv.URL + "/bad.hylapkg.sha256"
	if _, err := client.DownloadAndInstall(context.Background(), store, badSHAEntry, "darwin", "arm64", "3.18.0"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}

	// Plugin ID mismatch rejected.
	mismatchEntry := goodEntry
	mismatchEntry.DownloadURL = srv.URL + "/other.hylapkg"
	mismatchEntry.SHA256URL = srv.URL + "/other.hylapkg.sha256"
	if _, err := client.DownloadAndInstall(context.Background(), store, mismatchEntry, "darwin", "arm64", "3.18.0"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected ID mismatch error, got %v", err)
	}
}
