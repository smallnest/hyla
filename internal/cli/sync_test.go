package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"hyla/internal/settings"
	hylasync "hyla/internal/sync"
)

// `hyla sync` fills the settings the app uses, pushes the configuration to
// the server the settings name, and says where it got to.
func TestSyncCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_CONFIG", filepath.Join(home, "config"))
	settings.Reload()
	if err := os.WriteFile(filepath.Join(home, "config"), []byte("font-size = 13\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var stored []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			if stored == nil {
				http.NotFound(w, r)
				return
			}
			w.Write(stored)
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			stored = b
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	if err := syncCmd([]string{"now"}); err == nil {
		t.Fatal("a sync with no server said it worked")
	}
	for _, set := range [][2]string{{"backend", "webdav"}, {"webdav.url", srv.URL + "/dav/"}, {"passphrase", "correct horse"}} {
		if err := syncCmd([]string{"set", set[0], set[1]}); err != nil {
			t.Fatalf("set %s: %v", set[0], err)
		}
	}
	if err := syncCmd([]string{"now"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := append([]byte(nil), stored...)
	mu.Unlock()
	if len(got) == 0 {
		t.Fatal("the server holds no snapshot")
	}
	if bytes.Contains(got, []byte("font-size")) {
		t.Error("the configuration shows in what the server holds")
	}
	if !bytes.HasPrefix(got, []byte("HYLASYNC1")) {
		t.Errorf("the server holds %q, want a sealed snapshot", got[:12])
	}
	c := hylasync.Load()
	if c.Last.Hash == "" || c.Last.At == 0 {
		t.Errorf("the sync left nothing behind: %+v", c.Last)
	}
	if err := syncCmd([]string{"status"}); err != nil {
		t.Fatal(err)
	}
	if err := syncCmd([]string{"set", "backend", "ftp"}); err == nil {
		t.Error("an unknown server was taken")
	}
	if err := syncCmd([]string{"off"}); err != nil {
		t.Fatal(err)
	}
	if hylasync.Load().Backend != "" {
		t.Error("sync stayed on after off")
	}
}
