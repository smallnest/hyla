package daemonhost

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
)

func boolPtr(b bool) *bool { return &b }

func TestMain(m *testing.M) {
	if os.Getenv("HYLA_TEST_PLUGIN_HELPER") == "1" {
		runTestPluginHelper()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runTestPluginHelper() {
	mode := os.Getenv("HYLA_TEST_MODE")
	if mode == "crash" {
		fmt.Fprintln(os.Stderr, "boom crash")
		os.Exit(2)
	}

	var peer *rpc.Peer
	if sock := os.Getenv("HYLA_PLUGIN_SOCKET"); sock != "" {
		conn, err := dialTestSocket(sock)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		if err := rpc.WriteAuthFrame(conn, os.Getenv("HYLA_PLUGIN_TOKEN")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		peer = rpc.NewPeer(conn, conn, conn)
	} else {
		peer = rpc.NewPeer(os.Stdin, os.Stdout, nil)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var eventsMu sync.Mutex
	var events []string
	peer.Handle("initialize", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return rpc.InitializeResult{
			Name:          "Helper Plugin",
			Version:       "1.0.0",
			Subscriptions: []string{"theme.changed"},
		}, nil
	})
	peer.Handle("initialized", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		fmt.Fprintln(os.Stderr, "helper initialized")
		if mode == "crash-after-init" {
			go func() {
				time.Sleep(10 * time.Millisecond)
				os.Exit(2)
			}()
		}
		return nil, nil
	})
	peer.Handle("event.notify", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		var ev rpc.EventNotifyParams
		_ = json.Unmarshal(params, &ev)
		eventsMu.Lock()
		events = append(events, ev.Topic)
		eventsMu.Unlock()
		return nil, nil
	})
	peer.Handle("agent.detect", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		if strings.Contains(string(params), "call-state") {
			var st rpc.HostGetStateResult
			if err := peer.Call(ctx, "host.getState", rpc.HostGetStateParams{}, &st); err != nil {
				return nil, err
			}
			return rpc.AgentDetectResult{State: proto.Working, Rule: "host-state:" + st.ActiveSession}, nil
		}
		if strings.Contains(string(params), "call-send") {
			if err := peer.Call(ctx, "pane.sendInput", rpc.PaneSendInputParams{Pane: "p1", Data: "hi"}, nil); err != nil {
				return nil, err
			}
		}
		if strings.Contains(string(params), "call-timeline") {
			var tl rpc.PaneGetTimelineResult
			if err := peer.Call(ctx, "pane.getTimeline", rpc.PaneGetTimelineParams{Pane: "p1"}, &tl); err != nil {
				return nil, err
			}
			return rpc.AgentDetectResult{State: proto.Working, Rule: "tl:" + tl.Timeline.Agent}, nil
		}
		if strings.Contains(string(params), "settings-roundtrip") {
			if err := peer.Call(ctx, "plugin.setSettings", rpc.PluginSetSettingsParams{
				Settings: map[string]any{"tokenCount": 7},
			}, nil); err != nil {
				return nil, err
			}
			var got rpc.PluginGetSettingsResult
			if err := peer.Call(ctx, "plugin.getSettings", rpc.PluginGetSettingsParams{}, &got); err != nil {
				return nil, err
			}
			return rpc.AgentDetectResult{State: proto.Idle, Rule: fmt.Sprintf("cfg=%.0f", got.Settings["tokenCount"])}, nil
		}
		eventsMu.Lock()
		evCopy := append([]string(nil), events...)
		eventsMu.Unlock()
		return rpc.AgentDetectResult{State: proto.Waiting, Rule: "events:" + strings.Join(evCopy, ",")}, nil
	})
	peer.Handle("shutdown", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		cancel()
		return struct{}{}, nil
	})
	peer.Handle("exit", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		_ = peer.Close()
		cancel()
		return nil, nil
	})

	_ = peer.Serve(ctx)
}

func dialTestSocket(addr string) (net.Conn, error) {
	return net.Dial("unix", addr)
}

type fakeBridge struct {
	stateCalls atomic.Int64
}

func (b *fakeBridge) GetState(ctx context.Context, id string, p rpc.HostGetStateParams) (*rpc.HostGetStateResult, error) {
	b.stateCalls.Add(1)
	return &rpc.HostGetStateResult{ActiveSession: "sess-42", ControlAllowed: true}, nil
}
func (b *fakeBridge) ReadScreen(ctx context.Context, id string, p rpc.PaneReadScreenParams) (*rpc.PaneReadScreenResult, error) {
	return &rpc.PaneReadScreenResult{Text: "screen"}, nil
}
func (b *fakeBridge) GetEnv(ctx context.Context, id string, p rpc.PaneGetEnvParams) (*rpc.PaneGetEnvResult, error) {
	return &rpc.PaneGetEnvResult{Dir: "/tmp"}, nil
}
func (b *fakeBridge) CreateSession(ctx context.Context, id string, p proto.CreateParams) (*rpc.SessionCreateResult, error) {
	return &rpc.SessionCreateResult{SessionID: "s1", PaneID: "p1"}, nil
}
func (b *fakeBridge) SendInput(ctx context.Context, id string, p rpc.PaneSendInputParams) error {
	return nil
}
func (b *fakeBridge) SendKey(ctx context.Context, id string, p rpc.PaneSendKeyParams) error {
	return nil
}
func (b *fakeBridge) ReportStatus(ctx context.Context, id string, p rpc.PaneReportStatusParams) error {
	return nil
}
func (b *fakeBridge) FocusPane(ctx context.Context, id string, p rpc.PaneFocusParams) error {
	return nil
}
func (b *fakeBridge) InvalidateUI(ctx context.Context, id string, p rpc.UIInvalidateParams) error {
	return nil
}
func (b *fakeBridge) OpenPane(ctx context.Context, id string, p rpc.UIOpenPaneParams) error {
	return nil
}
func (b *fakeBridge) GetTimeline(ctx context.Context, id string, p rpc.PaneGetTimelineParams) (*rpc.PaneGetTimelineResult, error) {
	return &rpc.PaneGetTimelineResult{Timeline: proto.Timeline{Pane: p.Pane, Agent: "claude"}}, nil
}
func (b *fakeBridge) NotifyUI(ctx context.Context, id string, p rpc.UINotifyParams) error {
	return nil
}
func (b *fakeBridge) WebViewPictureInPicture(context.Context, string, rpc.WebViewPIPParams) error {
	return nil
}

func (b *fakeBridge) WebViewEmit(ctx context.Context, id string, p rpc.WebViewEmitParams) error {
	return nil
}

func TestManagerStdioSocketLifecycleAndPermissions(t *testing.T) {
	for _, transport := range []string{"stdio", "socket"} {
		t.Run(transport, func(t *testing.T) {
			base := t.TempDir()
			st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

			pDir := filepath.Join(base, "dev-plugin")
			if err := os.MkdirAll(pDir, 0o755); err != nil {
				t.Fatal(err)
			}
			tomlSrc := fmt.Sprintf(`
api_version = 1
id = "com.example.helper"
name = "Helper"
version = "1.0.0"

[permissions]
scopes = ["agent.provider", "session.read", "pane.write"]

[daemon]
command = [%q]
transport = %q
auto_start = false

[env]
HYLA_TEST_PLUGIN_HELPER = "1"
HYLA_TEST_MODE = "ok"
`, os.Args[0], transport)
			if err := os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(tomlSrc), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := st.Link(pDir, false); err != nil {
				t.Fatal(err)
			}
			// Grant only agent.provider + session.read initially (NOT pane.write).
			if err := st.GrantPermissions("com.example.helper", []string{"agent.provider", "session.read"}); err != nil {
				t.Fatal(err)
			}

			bridge := &fakeBridge{}
			mgr := NewManager(ManagerOptions{
				HostKind:    HostKindDaemon,
				HylaVersion: "3.16.1",
				Store:       st,
				Bridge:      bridge,
			})
			t.Cleanup(mgr.ShutdownAll)

			if err := mgr.Reload(); err != nil {
				t.Fatal(err)
			}
			// Auto-start is false: not running yet, HasKeepAliveRunning must be false.
			if mgr.HasKeepAliveRunning() {
				t.Fatal("HasKeepAliveRunning should be false for non-keepalive lazy plugin")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Lazy start via Call + bidirectional host.getState callback.
			var det rpc.AgentDetectResult
			if err := mgr.Call(ctx, "com.example.helper", "agent.detect", rpc.AgentDetectParams{Screen: "call-state"}, &det); err != nil {
				t.Fatalf("Call agent.detect: %v", err)
			}
			if det.State != proto.Working || det.Rule != "host-state:sess-42" {
				t.Fatalf("unexpected detect result: %+v", det)
			}
			if got := bridge.stateCalls.Load(); got != 1 {
				t.Fatalf("bridge.stateCalls = %d, want 1", got)
			}

			// Call without pane.write permission must return ErrCodePermissionDenied.
			err := mgr.Call(ctx, "com.example.helper", "agent.detect", rpc.AgentDetectParams{Screen: "call-send"}, &det)
			var rpcErr *rpc.Error
			if !errors.As(err, &rpcErr) || rpcErr.Code != rpc.ErrCodePermissionDenied {
				t.Fatalf("expected ErrCodePermissionDenied from ungranted pane.sendInput, got %v", err)
			}

			// Call without timeline.read permission must return ErrCodePermissionDenied.
			err = mgr.Call(ctx, "com.example.helper", "agent.detect", rpc.AgentDetectParams{Screen: "call-timeline"}, &det)
			rpcErr = nil
			if !errors.As(err, &rpcErr) || rpcErr.Code != rpc.ErrCodePermissionDenied {
				t.Fatalf("expected ErrCodePermissionDenied from ungranted pane.getTimeline, got %v", err)
			}

			// Grant timeline.read and verify pane.getTimeline succeeds.
			if err := st.GrantPermissions("com.example.helper", []string{"agent.provider", "session.read", manifest.PermTimelineRead}); err != nil {
				t.Fatal(err)
			}
			if err := mgr.Reload(); err != nil {
				t.Fatal(err)
			}
			if err := mgr.Call(ctx, "com.example.helper", "agent.detect", rpc.AgentDetectParams{Screen: "call-timeline"}, &det); err != nil {
				t.Fatalf("expected pane.getTimeline to succeed after granting timeline.read: %v", err)
			}
			if det.Rule != "tl:claude" {
				t.Fatalf("unexpected call-timeline rule: %q", det.Rule)
			}

			// Settings roundtrip works cleanly via Store.
			if err := mgr.Call(ctx, "com.example.helper", "agent.detect", rpc.AgentDetectParams{Screen: "settings-roundtrip"}, &det); err != nil {
				t.Fatalf("settings roundtrip Call failed: %v", err)
			}
			if det.Rule != "cfg=7" {
				t.Fatalf("unexpected settings roundtrip rule: %q", det.Rule)
			}

			// Notifications delivered.
			mgr.DiffAndNotifyState(proto.State{}, proto.State{Sessions: []proto.Session{{ID: "s1"}}}, false, true, "hyla")
			time.Sleep(50 * time.Millisecond)
			if err := mgr.Call(ctx, "com.example.helper", "agent.detect", rpc.AgentDetectParams{Screen: "check"}, &det); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(det.Rule, "theme.changed") {
				t.Fatalf("expected subscribed theme.changed notification, got %q", det.Rule)
			}

			logs, err := mgr.Logs("com.example.helper", 20)
			if err != nil || !strings.Contains(logs.Text, "helper initialized") {
				t.Fatalf("unexpected plugin logs: %+v, err=%v", logs, err)
			}
		})
	}
}

func TestManagerCircuitBreakerDegraded(t *testing.T) {
	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

	pDir := filepath.Join(base, "crash-plugin")
	_ = os.MkdirAll(pDir, 0o755)
	tomlSrc := fmt.Sprintf(`
api_version = 1
id = "com.example.crasher"
name = "Crasher"
version = "0.1.0"

[daemon]
command = [%q]
auto_start = true
min_restart_ms = 20
max_crashes_per_minute = 2

[env]
HYLA_TEST_PLUGIN_HELPER = "1"
HYLA_TEST_MODE = "crash-after-init"
`, os.Args[0])
	_ = os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(tomlSrc), 0o644)
	if _, err := st.Link(pDir, true); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(ManagerOptions{HostKind: HostKindDaemon, Store: st})
	t.Cleanup(mgr.ShutdownAll)
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	degraded := false
	for time.Now().Before(deadline) {
		for _, s := range mgr.ListStatuses() {
			if s.ID == "com.example.crasher" && s.State == StateDegraded {
				degraded = true
				break
			}
		}
		if degraded {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !degraded {
		t.Fatal("expected crash loop to trigger StateDegraded automatically")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := mgr.EnsureStarted(ctx, "com.example.crasher")
	if err == nil || !strings.Contains(err.Error(), "degraded") {
		t.Fatalf("expected degraded error, got %v", err)
	}
}

func TestRotatingLogWriter(t *testing.T) {
	p := filepath.Join(t.TempDir(), "plugin.log")
	lw, err := newRotatingLogWriter(p)
	if err != nil {
		t.Fatal(err)
	}
	defer lw.Close()

	chunk := make([]byte, 1<<20) // 1 MiB
	for i := range chunk {
		chunk[i] = 'a'
	}
	chunk[len(chunk)-1] = '\n'
	for range 5 {
		if _, err := lw.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(p + ".1"); err != nil {
		t.Fatalf("expected rotated .1 backup log file: %v", err)
	}
	_, lines := TailLog(p, 5)
	if lines == 0 {
		t.Fatal("expected TailLog to return lines after rotation")
	}
}

func TestResolveCommandWindowsExe(t *testing.T) {
	root := t.TempDir()
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = realRoot
	}
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exePath := filepath.Join(binDir, "hyla-timeline.exe")
	if err := os.WriteFile(exePath, []byte("exe"), 0o755); err != nil {
		t.Fatal(err)
	}

	gotWin, err := resolveCommandForOS(root, []string{"./bin/hyla-timeline", "--flag"}, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if len(gotWin) != 2 || gotWin[0] != exePath || gotWin[1] != "--flag" {
		t.Fatalf("windows resolved = %v, want [%s --flag]", gotWin, exePath)
	}

	gotUnix, err := resolveCommandForOS(root, []string{"./bin/hyla-timeline"}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	wantUnix := filepath.Join(binDir, "hyla-timeline")
	if len(gotUnix) != 1 || gotUnix[0] != wantUnix {
		t.Fatalf("linux resolved = %v, want [%s]", gotUnix, wantUnix)
	}
}

var _ = io.EOF
var _ = bufio.ErrTooLong
