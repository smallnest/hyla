package sync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"hyla/internal/settings"
)

// A snapshot sealed with a passphrase opens with it, and with no other: a
// wrong one, or a byte changed on the server, keeps its contents inside.
func TestSealOpen(t *testing.T) {
	s := Snapshot{Version: snapshotVersion, UpdatedAt: 1, Host: "laptop", Hash: "h", Content: "theme = hyla\n"}
	sealed, err := seal("correct horse", s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(sealed), magic) {
		t.Errorf("sealed snapshot starts %q, want %q", sealed[:len(magic)], magic)
	}
	if strings.Contains(string(sealed), "theme") {
		t.Error("the configuration shows in the sealed bytes")
	}
	got, err := open("correct horse", sealed)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "laptop" || got.Content != s.Content || got.Hash != s.Hash {
		t.Errorf("opened %+v, want %+v", got, s)
	}
	if _, err := open("wrong horse", sealed); !errors.Is(err, ErrPassphrase) {
		t.Errorf("another passphrase: %v, want ErrPassphrase", err)
	}
	// A byte of the ciphertext, and of the salt, both fail to open.
	for _, i := range []int{len(sealed) - 1, len(magic) + 1} {
		bad := append([]byte(nil), sealed...)
		bad[i] ^= 0x40
		if _, err := open("correct horse", bad); !errors.Is(err, ErrPassphrase) {
			t.Errorf("byte %d changed: %v, want ErrPassphrase", i, err)
		}
	}
	if _, err := open("correct horse", []byte("PK\x03\x04 not hyla")); err == nil {
		t.Error("a file that is not a snapshot opened")
	}
}

// davServer is a WebDAV folder in memory: the hyla folder, and one file in
// it.
func davServer(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	files := map[string][]byte{}
	mux := http.NewServeMux()
	mux.HandleFunc("/dav/hyla/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			b, ok := files["config.enc"]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(b)
		case http.MethodPut:
			b, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if r.URL.Path != "/dav/hyla/config.enc" {
				http.Error(w, "wrong path "+r.URL.Path, http.StatusBadRequest)
				return
			}
			files["config.enc"] = b
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, "unexpected "+r.Method, http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// machine is a computer syncing with the server: its configuration dir, and
// the sync settings it keeps, whose Last tells what it last agreed on.
type machine struct {
	t    *testing.T
	dir  string
	host string
	cfg  Config
}

func newMachine(t *testing.T, host, url string) *machine {
	t.Helper()
	dir := t.TempDir()
	return &machine{t: t, dir: dir, host: host, cfg: Config{
		Backend:    "webdav",
		WebDAV:     WebDAV{URL: url},
		Passphrase: "correct horse",
	}}
}

func (m *machine) path() string { return filepath.Join(m.dir, "config") }

func (m *machine) write(content string) {
	m.t.Helper()
	if err := os.WriteFile(m.path(), []byte(content), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

func (m *machine) read() string {
	m.t.Helper()
	b, err := os.ReadFile(m.path())
	if err != nil {
		m.t.Fatal(err)
	}
	return string(b)
}

// run syncs as this machine, keeping what the sync left.
func (m *machine) run(way string) Result {
	m.t.Helper()
	m.t.Setenv("HYLA_CONFIG", m.path())
	settings.Reload()
	res, cfg, err := Run(context.Background(), m.cfg, way, m.host)
	if err != nil {
		m.t.Fatalf("%s, %q: %v", m.host, way, err)
	}
	m.cfg = cfg
	return res
}

// Two machines and a server: the first makes the snapshot, the second takes
// it, both go on agreeing, and a change on either side reaches the other.
func TestSyncTwoMachines(t *testing.T) {
	srv := davServer(t)
	a := newMachine(t, "laptop", srv.URL+"/dav/")
	b := newMachine(t, "desk", srv.URL+"/dav/")
	a.write("theme = hyla\nfont-size = 13\n")

	if res := a.run(""); res.Action != "created" {
		t.Fatalf("the first sync = %q, want created", res.Action)
	}
	if res := b.run(""); res.Action != "pulled" {
		t.Fatalf("a new machine's first sync = %q, want pulled", res.Action)
	}
	if got, want := b.read(), a.read(); got != want {
		t.Errorf("the second machine reads %q, want %q", got, want)
	}
	if res := a.run(""); res.Action != "unchanged" {
		t.Errorf("a second sync = %q, want unchanged", res.Action)
	}

	// The second machine changes the configuration; the first, which did
	// not, takes it.
	b.write("theme = hyla\nfont-size = 15\n")
	if res := b.run(""); res.Action != "pushed" {
		t.Fatalf("a change = %q, want pushed", res.Action)
	}
	if res := a.run(""); res.Action != "pulled" {
		t.Fatalf("the other machine = %q, want pulled", res.Action)
	}
	if got := a.read(); !strings.Contains(got, "font-size = 15") {
		t.Errorf("the other machine reads %q, want the change", got)
	}

	// A direction pushes or pulls whatever the hashes say.
	a.write("theme = hyla\nfont-size = 16\n")
	if res := a.run("push"); res.Action != "pushed" {
		t.Errorf("push = %q, want pushed", res.Action)
	}
	if res := b.run("pull"); res.Action != "pulled" {
		t.Errorf("pull = %q, want pulled", res.Action)
	}
	if got := b.read(); !strings.Contains(got, "font-size = 16") {
		t.Errorf("the pulled machine reads %q", got)
	}
}

// Both sides changed: this machine's stays, and the server's is kept beside
// it, named for the machine that wrote it.
func TestSyncConflictKeepsTheServerSide(t *testing.T) {
	srv := davServer(t)
	a := newMachine(t, "laptop", srv.URL+"/dav/")
	b := newMachine(t, "desk", srv.URL+"/dav/")
	a.write("font-size = 13\n")
	a.run("")
	b.run("")

	// Each machine changes its own copy. The first to sync has no conflict:
	// the server had not moved since it last agreed. The second has one,
	// both sides having changed.
	a.write("font-size = 13\ncursor-style = block\n")
	b.write("font-size = 13\ncursor-style = bar\n")
	if res := a.run(""); res.Action != "pushed" || res.Backup != "" {
		t.Fatalf("the first machine to sync = %+v, want a push, and no copy", res)
	}

	res := b.run("")
	if res.Action != "pushed" {
		t.Fatalf("a conflict = %q, want pushed", res.Action)
	}
	if res.Backup == "" {
		t.Fatal("a conflict kept no copy of the server's configuration")
	}
	kept, err := os.ReadFile(res.Backup)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(kept), "cursor-style = block") {
		t.Errorf("the copy holds %q, want the other machine's", kept)
	}
	if !strings.Contains(filepath.Base(res.Backup), "laptop") {
		t.Errorf("the copy is %q, want it named for the machine that wrote it", res.Backup)
	}
	if got := b.read(); !strings.Contains(got, "bar") {
		t.Errorf("this machine reads %q, want its own change", got)
	}
}

// A snapshot sealed with another passphrase is no snapshot this machine can
// read, and reading it changes nothing here.
func TestSyncWrongPassphrase(t *testing.T) {
	srv := davServer(t)
	a := newMachine(t, "laptop", srv.URL+"/dav/")
	a.write("font-size = 13\n")
	a.run("")

	b := newMachine(t, "desk", srv.URL+"/dav/")
	b.cfg.Passphrase = "another horse"
	b.write("font-size = 14\n")
	b.t.Setenv("HYLA_CONFIG", b.path())
	settings.Reload()
	if _, _, err := Run(context.Background(), b.cfg, "", b.host); !errors.Is(err, ErrPassphrase) {
		t.Fatalf("another passphrase: %v, want ErrPassphrase", err)
	}
	if got := b.read(); got != "font-size = 14\n" {
		t.Errorf("the configuration changed to %q", got)
	}
}

// The object a bucket holds is signed as version 4 signs it, and the service
// reads back what was written.
func TestS3RoundTrip(t *testing.T) {
	var mu sync.Mutex
	var stored []byte
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			if stored == nil {
				http.NotFound(w, r)
				return
			}
			w.Write(stored)
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			stored = b
			auth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	st, err := newS3(S3{Endpoint: srv.URL, Bucket: "me", AccessKey: "AK", SecretKey: "SK", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := st.Get(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an empty bucket: %v, want ErrNotFound", err)
	}
	if err := st.Put(ctx, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"AWS4-HMAC-SHA256 Credential=AK/", "/us-east-1/s3/aws4_request", "SignedHeaders=host;x-amz-content-sha256;x-amz-date", "Signature="} {
		if !strings.Contains(auth, want) {
			t.Errorf("Authorization %q lacks %q", auth, want)
		}
	}
	got, err := st.Get(ctx)
	if err != nil || string(got) != "hello" {
		t.Errorf("the bucket reads %q, %v", got, err)
	}
	if got := st.url(); !strings.Contains(got, "/me/hyla/config.enc") {
		t.Errorf("the object's url is %q, want the bucket and the key", got)
	}
}

// The repository's file is read, and written as a commit that names the
// machine, with the blob's id when it was there.
func TestGitHubRoundTrip(t *testing.T) {
	var mu sync.Mutex
	var stored []byte
	var sha string
	var commit map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/repos/me/dotfiles/contents/hyla/config.enc" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if stored == nil {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"content": base64.StdEncoding.EncodeToString(stored),
				"sha":     sha,
			})
		case http.MethodPut:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			commit = body
			raw, _ := base64.StdEncoding.DecodeString(body["content"].(string))
			stored = raw
			sha = "abc123"
			json.NewEncoder(w).Encode(map[string]any{"content": map[string]any{"sha": sha}})
		}
	}))
	defer srv.Close()

	st, err := newGitHub(GitHub{Repo: "me/dotfiles", Token: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	st.api, st.host = srv.URL, "laptop"
	ctx := context.Background()
	if _, err := st.Get(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an empty repository: %v, want ErrNotFound", err)
	}
	if err := st.Put(ctx, []byte("font-size = 13\n")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(commit["message"].(string), "laptop") {
		t.Errorf("the commit says %q, want the machine's name", commit["message"])
	}
	if _, ok := commit["sha"]; ok {
		t.Error("the first write named a blob the repository had not")
	}
	got, err := st.Get(ctx)
	if err != nil || string(got) != "font-size = 13\n" {
		t.Errorf("the repository reads %q, %v", got, err)
	}
	// The second write replaces the blob it read.
	if err := st.Put(ctx, []byte("font-size = 14\n")); err != nil {
		t.Fatal(err)
	}
	if commit["sha"] != "abc123" {
		t.Errorf("the second write names %v, want the blob it replaces", commit["sha"])
	}
}

func TestConfigured(t *testing.T) {
	for _, c := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{"nothing", Config{}, false},
		{"no passphrase", Config{Backend: "webdav", WebDAV: WebDAV{URL: "https://x"}}, false},
		{"webdav", Config{Backend: "webdav", Passphrase: "p", WebDAV: WebDAV{URL: "https://x"}}, true},
		{"webdav without an address", Config{Backend: "webdav", Passphrase: "p"}, false},
		{"s3", Config{Backend: "s3", Passphrase: "p", S3: S3{Endpoint: "https://x", Bucket: "b", AccessKey: "a", SecretKey: "s"}}, true},
		{"s3 without a secret", Config{Backend: "s3", Passphrase: "p", S3: S3{Endpoint: "https://x", Bucket: "b", AccessKey: "a"}}, false},
		{"github", Config{Backend: "github", Passphrase: "p", GitHub: GitHub{Repo: "me/x", Token: "t"}}, true},
		{"github without an owner", Config{Backend: "github", Passphrase: "p", GitHub: GitHub{Repo: "x", Token: "t"}}, false},
		{"an unknown server", Config{Backend: "ftp", Passphrase: "p"}, false},
	} {
		if got := Configured(c.cfg); got != c.want {
			t.Errorf("%s: Configured = %v, want %v", c.name, got, c.want)
		}
	}
}

// The defaults of a sync file, and that what it holds is for its user alone.
func TestConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HYLA_CONFIG", filepath.Join(dir, "config"))
	settings.Reload()
	c := Load()
	if c.S3.Region != "us-east-1" || c.S3.Prefix != "hyla" || c.GitHub.Path != "hyla/config.enc" {
		t.Errorf("defaults = %+v", c)
	}
	c.Backend, c.Passphrase, c.GitHub.Token = "github", "p", "tok"
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Errorf("sync.json is %v, want 0600: it holds a passphrase and a token", mode)
	}
	if got := Load(); got.Backend != "github" || got.Passphrase != "p" {
		t.Errorf("read back %+v", got)
	}
}
