package cli

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"hyla/internal/actions"
	"hyla/internal/plugins/manifest"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/wash"
)

func TestCLIPreloadBaseActionsWashAndDo(t *testing.T) {
	tmp := t.TempDir()
	hylaDir := filepath.Join(tmp, "hyla-home")
	cfgPath := filepath.Join(tmp, "config")
	if err := os.MkdirAll(hylaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYLA_DIR", hylaDir)
	t.Setenv("HYLA_CONFIG", cfgPath)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")
	// The socket this test takes for its fake daemon is its own, and short
	// enough for one: a test run inside Hyla inherits HYLA_SOCKET, the
	// hylad the user runs, and taking that one would leave it running with
	// nothing to connect to.
	sockDir, err := os.MkdirTemp("", "hy")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	t.Setenv("HYLA_SOCKET", filepath.Join(sockDir, "h.sock"))
	t.Cleanup(func() {
		actions.ClearPluginActions()
		proto.ClearPluginAgents()
		wash.ReloadPlugins(nil)
		settings.SetPluginBackgrounds(nil)
	})

	store := manifest.DefaultStore()
	pluginID := "com.example.pack"
	pdir, err := store.PluginDir(pluginID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pdir, "washes"), 0o755); err != nil {
		t.Fatal(err)
	}
	washTOML := `id = "aurora"
name = "Aurora"
light = ["#eef4ff", "#ddeaff"]
dark = ["#0b1322", "#14223a"]
`
	if err := os.WriteFile(filepath.Join(pdir, "washes", "aurora.toml"), []byte(washTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	tomlBody := `
id = "com.example.pack"
name = "Pack"
version = "1.0.0"
api_version = 1

[[washes]]
id = "aurora"
name = "Aurora"
file = "washes/aurora.toml"

[[actions]]
id = "ping"
title = "Ping Pack"
remote = true
`
	if err := os.WriteFile(filepath.Join(pdir, manifest.ManifestFileName), []byte(tomlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRegistry(&manifest.Registry{
		Version: 1,
		Plugins: map[string]manifest.RegistryEntry{
			pluginID: {ID: pluginID, Enabled: true, Version: "1.0.0"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	actID := manifest.QualifyActionID(pluginID, "ping")
	if err := os.WriteFile(cfgPath, []byte("background = aurora\nkeybind = cmd+alt+p="+actID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// 1. hyla config check validates plugin background & action keybinds cleanly.
	if code := Run([]string{"config", "check"}); code != 0 {
		t.Fatalf("hyla config check returned %d, want 0", code)
	}

	// 2. hyla actions lists remote plugin action.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := Run([]string{"actions"})
	_ = w.Close()
	os.Stdout = oldStdout
	if code != 0 {
		t.Fatalf("hyla actions returned %d, want 0", code)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	if !strings.Contains(string(buf[:n]), actID) {
		t.Fatalf("expected %q in hyla actions output:\n%s", actID, buf[:n])
	}

	// 3. hyla do <plugin-action> dials socket and dispatches DoParams.
	sockPath := proto.SocketPath()
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var (
		mu      sync.Mutex
		gotDo   proto.DoParams
		didCall bool
	)
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				sc := bufio.NewScanner(c)
				if !sc.Scan() {
					return
				}
				// First line is Hello; answer with Welcome.
				wb, _ := json.Marshal(proto.Welcome{
					Version:      proto.Version,
					PID:          os.Getpid(),
					Started:      1,
					Capabilities: []string{proto.CapPluginsV1},
				})
				_, _ = c.Write(append(wb, '\n'))

				for sc.Scan() {
					var req proto.Request
					if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
						return
					}
					var msg proto.Message
					msg.ID = req.ID
					switch req.Method {
					case "subscribe", "state":
						msg.Result = proto.State{}
					case "do":
						pb, _ := json.Marshal(req.Params)
						var dp proto.DoParams
						_ = json.Unmarshal(pb, &dp)
						mu.Lock()
						gotDo = dp
						didCall = true
						mu.Unlock()
						msg.Result = true
					default:
						msg.Result = true
					}
					mb, _ := json.Marshal(msg)
					_, _ = c.Write(append(mb, '\n'))
				}
			}(conn)
		}
	}()

	if code := Run([]string{"do", actID, "k=v"}); code != 0 {
		t.Fatalf("hyla do %s returned %d, want 0", actID, code)
	}
	_ = ln.Close()
	<-doneCh

	mu.Lock()
	defer mu.Unlock()
	if !didCall || gotDo.Action != actID || gotDo.Args["k"] != "v" {
		t.Fatalf("unexpected DoParams recorded: didCall=%v got=%+v", didCall, gotDo)
	}
}
