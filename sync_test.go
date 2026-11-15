package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	hylasync "hyla/internal/sync"
	"hyla/locales"
)

// Settings › Sync takes the server, its credentials and the passphrase, and
// says where the last sync got to.
func TestSyncSettingsPage(t *testing.T) {
	a := testApp(t)
	a.State.Cfg = hylasync.Config{
		Backend:    "webdav",
		WebDAV:     hylasync.WebDAV{URL: "https://dav.jianguoyun.com/dav/", User: "me"},
		Passphrase: "correct horse",
		Auto:       true,
		Last:       hylasync.State{Hash: "abcdef0123456789", Host: "desk", At: time.Now().Add(-2 * time.Minute).UnixMilli()},
	}
	a.State.Saved, a.State.Loaded, a.State.Action, a.State.Host = a.State.Cfg, true, "pulled", "desk"
	a.Ss.page = "sync"
	tt := ui.NewTester(a.settingsView, 900, 900)
	for i := 0; i < 30 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	for _, want := range []string{locales.PageSync(), locales.SetSyncServer(), "WebDAV", locales.SetSyncNow(), locales.SetSyncAuto()} {
		if !tt.HasText(want) {
			t.Errorf("Settings › Sync shows no %q; texts %q", want, tt.Texts())
		}
	}
	if got := a.syncLine(); got == "" || got == locales.SetSyncNever() || !strings.Contains(got, "desk") {
		t.Errorf("the page says %q, want the last sync, from desk", got)
	}
	shot(t, tt, "settings-sync")

	// A server whose fields are missing says so, rather than syncing.
	a.State.Cfg.WebDAV.URL = ""
	if hylasync.Configured(a.State.Cfg) {
		t.Error("a WebDAV server with no address counts as set")
	}
}
