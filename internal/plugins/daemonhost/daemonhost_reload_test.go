package daemonhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
)

func TestManagerIsEnabledAndReloadRestartStopBackoff(t *testing.T) {
	isEnabled := func(m *Manager, id string) bool {
		for _, s := range m.ListStatuses() {
			if s.ID == id {
				return s.Enabled && !s.PendingInstall
			}
		}
		return false
	}
	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pDir := filepath.Join(base, "helper-plugin")
	_ = os.MkdirAll(pDir, 0o755)
	tomlSrc := fmt.Sprintf(`
api_version = 1
id = "com.example.reload"
name = "Reload"
version = "1.0.0"

[permissions]
scopes = ["agent.provider", "session.read", "pane.write"]

[daemon]
command = [%q]
auto_start = true
`, exe)
	_ = os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(tomlSrc), 0o644)

	t.Setenv("HYLA_TEST_PLUGIN_HELPER", "1")
	t.Setenv("HYLA_TEST_HELPER_MODE", "stdio")

	if _, err := st.Link(pDir, false); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(ManagerOptions{
		HostKind: HostKindDaemon,
		Store:    st,
		Bridge:   &fakeBridge{},
	})
	defer mgr.ShutdownAll()

	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}
	if !isEnabled(mgr, "com.example.reload") {
		t.Fatal("expected enabled=true after initial Reload")
	}

	// Wait for auto-started plugin process to run.
	deadline := time.Now().Add(5 * time.Second)
	var firstPID int
	for time.Now().Before(deadline) {
		for _, s := range mgr.ListStatuses() {
			if s.ID == "com.example.reload" && s.State == StateRunning && s.PID > 0 {
				firstPID = s.PID
			}
		}
		if firstPID > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if firstPID == 0 {
		t.Fatalf("expected running PID, got %+v", mgr.ListStatuses())
	}

	// Granting permissions and calling Reload must restart the running plugin with a new PID.
	if err := st.SetPermission("com.example.reload", manifest.PermAgentProvider, true); err != nil {
		t.Fatal(err)
	}
	if err := st.SetPermission("com.example.reload", manifest.PermSessionRead, true); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}

	var secondPID int
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, s := range mgr.ListStatuses() {
			if s.ID == "com.example.reload" && s.State == StateRunning && s.PID > 0 && s.PID != firstPID {
				secondPID = s.PID
			}
		}
		if secondPID > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if secondPID == 0 || secondPID == firstPID {
		t.Fatalf("expected Reload with permission change to restart plugin (firstPID=%d, statuses=%+v)", firstPID, mgr.ListStatuses())
	}

	// Verify newly granted host.getState now succeeds on restarted process.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var det rpc.AgentDetectResult
	if err := mgr.Call(ctx, "com.example.reload", "agent.detect", rpc.AgentDetectParams{Screen: "call-state"}, &det); err != nil {
		t.Fatalf("Call after permission reload failed: %v", err)
	}
	if det.State != proto.Working {
		t.Fatalf("unexpected det=%+v", det)
	}

	// Disabling must flip IsEnabled=false and stop any running process/backoff timer cleanly.
	if err := st.Enable("com.example.reload", false); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}
	if isEnabled(mgr, "com.example.reload") {
		t.Fatal("expected enabled=false after disable + Reload")
	}
	for _, s := range mgr.ListStatuses() {
		if s.ID == "com.example.reload" && s.State != StateStopped {
			t.Fatalf("expected stopped state after disable, got %+v", s)
		}
	}
}

func TestManagerEnabledAutoStartsAndDisabledStops(t *testing.T) {
	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pDir := filepath.Join(base, "lifecycle-plugin")
	_ = os.MkdirAll(pDir, 0o755)
	tomlSrc := fmt.Sprintf(`
api_version = 1
id = "com.example.lifecycle"
name = "Lifecycle"
version = "1.0.0"

[daemon]
command = [%q]
auto_start = false
`, exe)
	_ = os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(tomlSrc), 0o644)

	t.Setenv("HYLA_TEST_PLUGIN_HELPER", "1")
	t.Setenv("HYLA_TEST_HELPER_MODE", "stdio")

	// Install disabled initially.
	if _, err := st.Link(pDir, false); err != nil {
		t.Fatal(err)
	}
	if err := st.Enable("com.example.lifecycle", false); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var transitions []string
	mgr := NewManager(ManagerOptions{
		HostKind: HostKindDaemon,
		Store:    st,
		Bridge:   &fakeBridge{},
		OnStatusChange: func(pluginID, state string) {
			if pluginID == "com.example.lifecycle" {
				mu.Lock()
				transitions = append(transitions, state)
				mu.Unlock()
			}
		},
	})
	defer mgr.ShutdownAll()

	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)

	getStatus := func(m *Manager) rpc.PluginStatus {
		for _, s := range m.ListStatuses() {
			if s.ID == "com.example.lifecycle" {
				return s
			}
		}
		t.Fatal("missing com.example.lifecycle in ListStatuses")
		return rpc.PluginStatus{}
	}
	waitState := func(m *Manager, want string) rpc.PluginStatus {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			s := getStatus(m)
			if s.State == want && (want != StateRunning || s.PID > 0) {
				return s
			}
			time.Sleep(25 * time.Millisecond)
		}
		s := getStatus(m)
		t.Fatalf("timed out waiting for state %q; got %+v", want, s)
		return s
	}

	// 1. Starts disabled -> StateStopped (PID 0), Restart refuses with ErrCodePermissionDenied.
	s := getStatus(mgr)
	if s.State != StateStopped || s.PID != 0 {
		t.Fatalf("expected disabled plugin to stay stopped on startup, got %+v", s)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rerr := mgr.Restart(ctx, "com.example.lifecycle")
	var rpcErr *rpc.Error
	if !errors.As(rerr, &rpcErr) || rpcErr.Code != rpc.ErrCodePermissionDenied {
		t.Fatalf("Restart disabled plugin error = %v, want ErrCodePermissionDenied", rerr)
	}

	// 2. Runtime enable -> automatically starts and becomes running even with auto_start = false.
	if err := st.Enable("com.example.lifecycle", true); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}
	s = waitState(mgr, StateRunning)
	if s.PID <= 0 {
		t.Fatalf("expected running PID > 0, got %+v", s)
	}
	mu.Lock()
	sawRunning := slices.Contains(transitions, StateRunning)
	mu.Unlock()
	if !sawRunning {
		t.Fatalf("expected OnStatusChange to observe StateRunning, got %v", transitions)
	}

	// 3. Runtime disable -> stops cleanly to StateStopped and notifies OnStatusChange.
	if err := st.Enable("com.example.lifecycle", false); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}
	s = waitState(mgr, StateStopped)
	if s.PID != 0 {
		t.Fatalf("expected PID 0 after disable, got %+v", s)
	}
	mu.Lock()
	sawStopped := slices.Contains(transitions, StateStopped)
	mu.Unlock()
	if !sawStopped {
		t.Fatalf("expected OnStatusChange to observe StateStopped, got %v", transitions)
	}

	// 4. Fresh manager startup when enabled -> automatically running.
	if err := st.Enable("com.example.lifecycle", true); err != nil {
		t.Fatal(err)
	}
	mgr2 := NewManager(ManagerOptions{
		HostKind: HostKindDaemon,
		Store:    st,
		Bridge:   &fakeBridge{},
	})
	defer mgr2.ShutdownAll()
	if err := mgr2.Reload(); err != nil {
		t.Fatal(err)
	}
	waitState(mgr2, StateRunning)

	// 5. Fresh manager startup when disabled -> stays stopped.
	if err := st.Enable("com.example.lifecycle", false); err != nil {
		t.Fatal(err)
	}
	mgr3 := NewManager(ManagerOptions{
		HostKind: HostKindDaemon,
		Store:    st,
		Bridge:   &fakeBridge{},
	})
	defer mgr3.ShutdownAll()
	if err := mgr3.Reload(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	s3 := getStatus(mgr3)
	if s3.State != StateStopped || s3.PID != 0 {
		t.Fatalf("expected fresh manager with disabled plugin to stay stopped, got %+v", s3)
	}
}
