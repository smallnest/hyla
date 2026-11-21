package uihost

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"

	"hyla/internal/plugins/manifest"
	"hyla/internal/ui/theme"
)

func TestStrictCSPCSSVariablesAndReleasePlugin(t *testing.T) {
	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

	pDir := filepath.Join(base, "web-plugin")
	if err := os.MkdirAll(filepath.Join(pDir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "com.example.strict"
name = "Strict"
version = "1.0.0"

[permissions]
scopes = ["ui.webview"]

[[ui.slots]]
id = "board"
mount = "pane"
title = "Board"
mode = "webview"
web_root = "web"
entry = "index.html"
`), 0o644)
	_ = os.WriteFile(filepath.Join(pDir, "web", "index.html"), []byte("<html>ok</html>"), 0o644)
	if _, err := st.Link(pDir, true); err != nil {
		t.Fatal(err)
	}

	bridge := NewWebBridge(st, func() *theme.Palette { return &theme.Light }, nil)
	req := httptest.NewRequest(http.MethodGet, "hylaplugin://com.example.strict/index.html", nil)
	rec := httptest.NewRecorder()
	bridge.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "hylaplugin:") {
		t.Fatalf("CSP must use strict 'self' rather than broad hylaplugin: wildcard: %q", csp)
	}
	if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-src 'none'") || !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("unexpected strict CSP: %q", csp)
	}

	cssReq := httptest.NewRequest(http.MethodGet, "hylaplugin://com.example.strict/_hyla/theme.css", nil)
	cssRec := httptest.NewRecorder()
	bridge.ServeHTTP(cssRec, cssReq)
	css := cssRec.Body.String()
	for _, wantVar := range []string{
		"--hyla-muted:", "--hyla-text-muted:", "--hyla-font-ui:", "--hyla-font-mono:",
		"--hyla-tl-think:", "--hyla-tl-text:", "--hyla-tl-tool:", "--hyla-tl-sub:", "--hyla-tl-wait:", "--hyla-tl-fail:",
	} {
		if !strings.Contains(css, wantVar) {
			t.Fatalf("missing %s in ThemeCSS:\n%s", wantVar, css)
		}
	}

	pg1, pg2 := new(mygo.Page), new(mygo.Page)
	bridge.bindings[pg1] = pageBinding{PluginID: "com.example.strict", SlotID: "board", PaneID: "p1", Page: pg1}
	bridge.bindings[pg2] = pageBinding{PluginID: "com.example.other", SlotID: "x", PaneID: "p2", Page: pg2}
	bridge.paneEntries["p1"] = &WebEntry{PluginID: "com.example.strict", PaneID: "p1"}
	bridge.paneEntries["p2"] = &WebEntry{PluginID: "com.example.other", PaneID: "p2"}
	bridge.sessions["com.example.strict"] = mygo.NewSession("plugin:com.example.strict")

	bridge.ReleasePlugin("com.example.strict")
	if _, ok := bridge.bindings[pg1]; ok {
		t.Fatal("expected pg1 binding removed on ReleasePlugin")
	}
	if _, ok := bridge.paneEntries["p1"]; ok {
		t.Fatal("expected p1 paneEntry removed on ReleasePlugin")
	}
	if _, ok := bridge.sessions["com.example.strict"]; ok {
		t.Fatal("expected isolated session removed on ReleasePlugin")
	}
	if _, ok := bridge.bindings[pg2]; !ok {
		t.Fatal("expected other plugin binding preserved")
	}
}

func TestWebViewCSPWithNetworkPermission(t *testing.T) {
	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

	writeWebPlugin := func(id, perms string) {
		t.Helper()
		pDir := filepath.Join(base, id)
		if err := os.MkdirAll(filepath.Join(pDir, "web"), 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "`+id+`"
name = "Test"
version = "1.0.0"

[permissions]
scopes = [`+perms+`]

[[ui.slots]]
id = "main"
mount = "pane"
title = "Main"
mode = "webview"
web_root = "web"
entry = "index.html"
`), 0o644)
		_ = os.WriteFile(filepath.Join(pDir, "web", "index.html"), []byte("<html>ok</html>"), 0o644)
		if _, err := st.Link(pDir, true); err != nil {
			t.Fatal(err)
		}
	}

	writeWebPlugin("com.example.nonet", `"ui.webview"`)
	writeWebPlugin("com.example.withnet", `"ui.webview", "network"`)

	bridge := NewWebBridge(st, func() *theme.Palette { return &theme.Light }, nil)

	noNetReq := httptest.NewRequest(http.MethodGet, "hylaplugin://com.example.nonet/index.html", nil)
	noNetRec := httptest.NewRecorder()
	bridge.ServeHTTP(noNetRec, noNetReq)
	if noNetRec.Code != http.StatusOK {
		t.Fatalf("nonet status=%d", noNetRec.Code)
	}
	noNetCSP := noNetRec.Header().Get("Content-Security-Policy")
	if strings.Contains(noNetCSP, "media-src") {
		t.Fatalf("plugin without network permission must not receive media-src: %q", noNetCSP)
	}
	if !strings.Contains(noNetCSP, "img-src 'self' data: blob:") {
		t.Fatalf("unexpected nonet img-src: %q", noNetCSP)
	}

	netReq := httptest.NewRequest(http.MethodGet, "hylaplugin://com.example.withnet/index.html", nil)
	netRec := httptest.NewRecorder()
	bridge.ServeHTTP(netRec, netReq)
	if netRec.Code != http.StatusOK {
		t.Fatalf("withnet status=%d", netRec.Code)
	}
	netCSP := netRec.Header().Get("Content-Security-Policy")
	for _, want := range []string{
		"default-src 'self'",
		"media-src 'self' http://127.0.0.1:* http://localhost:* https: data: blob:",
		"img-src 'self' http://127.0.0.1:* http://localhost:* https: data: blob:",
		"connect-src 'self'",
		"frame-src 'none'",
		"object-src 'none'",
	} {
		if !strings.Contains(netCSP, want) {
			t.Fatalf("withnet CSP missing %q: %q", want, netCSP)
		}
	}

	noNetInst, _ := bridge.lookupPlugin(st, "com.example.nonet")
	if got := webViewAutoplay(noNetInst); got != mygo.AutoplayUserGesture {
		t.Fatalf("nonet webViewAutoplay = %v, want AutoplayUserGesture", got)
	}
	netInst, _ := bridge.lookupPlugin(st, "com.example.withnet")
	if got := webViewAutoplay(netInst); got != mygo.AutoplayAllow {
		t.Fatalf("withnet webViewAutoplay = %v, want AutoplayAllow", got)
	}
}
