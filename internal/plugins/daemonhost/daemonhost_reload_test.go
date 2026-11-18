package daemonhost

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
)

func TestManagerIsEnabledAndReloadRestartStopBackoff(t *testing.T) {
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
	if !mgr.IsEnabled("com.example.reload") {
		t.Fatal("expected IsEnabled=true after initial Reload")
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
	if mgr.IsEnabled("com.example.reload") {
		t.Fatal("expected IsEnabled=false after disable + Reload")
	}
	for _, s := range mgr.ListStatuses() {
		if s.ID == "com.example.reload" && s.State != StateStopped {
			t.Fatalf("expected stopped state after disable, got %+v", s)
		}
	}
}
