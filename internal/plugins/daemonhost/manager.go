package daemonhost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
)

const (
	HostKindDaemon = "daemon"
	HostKindApp    = "app"

	StateStopped  = "stopped"
	StateStarting = "starting"
	StateRunning  = "running"
	StateDegraded = "degraded"
	StateError    = "error"
)

// How long a plugin's process gets to come up and answer initialize: one
// that is slow to start on a loaded machine, as a TypeScript plugin under
// bun may be, took longer than the ten seconds these had.
const (
	pluginStartTimeout = 60 * time.Second
	pluginInitTimeout  = 60 * time.Second
	// pluginBootTimeout is how long an auto-start, or a restart after a
	// crash, waits for a plugin to come up: its start and its initialize.
	pluginBootTimeout = 3 * time.Minute
)

// HostBridge handles Plugin -> Host RPC callbacks on behalf of plugins.
type HostBridge interface {
	GetState(ctx context.Context, pluginID string, p rpc.HostGetStateParams) (*rpc.HostGetStateResult, error)
	ReadScreen(ctx context.Context, pluginID string, p rpc.PaneReadScreenParams) (*rpc.PaneReadScreenResult, error)
	GetEnv(ctx context.Context, pluginID string, p rpc.PaneGetEnvParams) (*rpc.PaneGetEnvResult, error)
	CreateSession(ctx context.Context, pluginID string, p proto.CreateParams) (*rpc.SessionCreateResult, error)
	SendInput(ctx context.Context, pluginID string, p rpc.PaneSendInputParams) error
	SendKey(ctx context.Context, pluginID string, p rpc.PaneSendKeyParams) error
	ReportStatus(ctx context.Context, pluginID string, p rpc.PaneReportStatusParams) error
	FocusPane(ctx context.Context, pluginID string, p rpc.PaneFocusParams) error
	InvalidateUI(ctx context.Context, pluginID string, p rpc.UIInvalidateParams) error
	OpenPane(ctx context.Context, pluginID string, p rpc.UIOpenPaneParams) error
	GetTimeline(ctx context.Context, pluginID string, p rpc.PaneGetTimelineParams) (*rpc.PaneGetTimelineResult, error)
	NotifyUI(ctx context.Context, pluginID string, p rpc.UINotifyParams) error
	WebViewEmit(ctx context.Context, pluginID string, p rpc.WebViewEmitParams) error
}

// BuiltinRunnerFunc runs an in-process built-in plugin speaking JSON-RPC over peer.
type BuiltinRunnerFunc func(ctx context.Context, peer *rpc.Peer)

var (
	builtinRunnersMu sync.RWMutex
	builtinRunners   = map[string]BuiltinRunnerFunc{} // "<id>|<hostKind>" -> fn
)

// RegisterBuiltinRunner registers a built-in plugin RPC server runnable inside hylad or window.
func RegisterBuiltinRunner(pluginID, hostKind string, fn BuiltinRunnerFunc) {
	if pluginID == "" || hostKind == "" || fn == nil {
		return
	}
	builtinRunnersMu.Lock()
	builtinRunners[pluginID+"|"+hostKind] = fn
	builtinRunnersMu.Unlock()
}

func getBuiltinRunner(pluginID, hostKind string) BuiltinRunnerFunc {
	builtinRunnersMu.RLock()
	fn := builtinRunners[pluginID+"|"+hostKind]
	builtinRunnersMu.RUnlock()
	return fn
}

type ManagerOptions struct {
	HostKind        string // HostKindDaemon or HostKindApp
	HylaVersion     string
	Store           *manifest.Store
	Bridge          HostBridge
	LocaleFunc      func() string
	ThemeFunc       func() (appearance string, dark bool)
	OnSettingsSaved func(pluginID string)
	OnStatusChange  func(pluginID, state string)
}

// Manager supervises out-of-process and built-in plugin hosts for either hylad or the GUI app.
type Manager struct {
	opts ManagerOptions

	mu      sync.Mutex
	plugins map[string]*managedPlugin
	closed  bool
}

type managedPlugin struct {
	installed     manifest.InstalledPlugin
	spec          *manifest.HostProcessSpec
	state         string
	pid           int
	startedAt     time.Time
	lastError     string
	crashes       []time.Time
	subscriptions []string
	capabilities  []string
	peer          *rpc.Peer
	cmd           *exec.Cmd
	logWriter     *rotatingLogWriter
	cleanup       []func()
	runCleanup    func()
	waitDone      chan struct{}
	cancel        context.CancelFunc
	stopping      bool
	startWait     chan struct{} // closed when current start attempt completes
	startCancel   context.CancelFunc
	backoffTimer  *time.Timer
}

// NewManager creates a new plugin host supervisor.
func NewManager(opts ManagerOptions) *Manager {
	if opts.HostKind == "" {
		opts.HostKind = HostKindDaemon
	}
	if opts.Store == nil {
		opts.Store = manifest.DefaultStore()
	}
	if opts.LocaleFunc == nil {
		opts.LocaleFunc = func() string { return "en" }
	}
	if opts.ThemeFunc == nil {
		opts.ThemeFunc = func() (string, bool) { return "system", false }
	}
	return &Manager{
		opts:    opts,
		plugins: make(map[string]*managedPlugin),
	}
}

func (m *Manager) notifyStatus(id, state string) {
	if m.opts.OnStatusChange != nil {
		m.opts.OnStatusChange(id, state)
	}
}

// Reload discovers installed plugins, reconciles active specs, stops disabled/removed
// plugins, and auto-starts enabled plugins.
func (m *Manager) Reload() error {
	list, err := m.opts.Store.Discover()
	if err != nil {
		return err
	}
	return m.ReloadWithPlugins(list)
}

// ReloadWithPlugins reconciles active specs against a pre-discovered plugin list.
func (m *Manager) ReloadWithPlugins(list []manifest.InstalledPlugin) error {

	var toStop []*managedPlugin
	var toAutoStart []string
	var statusStopped []string
	var cancels []context.CancelFunc

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	seen := make(map[string]bool, len(list))
	for _, p := range list {
		id := p.Entry.ID
		seen[id] = true
		spec := m.specFor(p)
		cur := m.plugins[id]
		if !p.Entry.Enabled || p.Entry.PendingInstall || p.Manifest == nil || spec == nil {
			if cur != nil {
				if cur.state != StateStopped {
					statusStopped = append(statusStopped, id)
				}
				if cur.backoffTimer != nil {
					cur.backoffTimer.Stop()
					cur.backoffTimer = nil
				}
				if cur.startCancel != nil {
					cancels = append(cancels, cur.startCancel)
					cur.startCancel = nil
				}
				cur.startWait = nil
				if cur.peer != nil || cur.cmd != nil {
					toStop = append(toStop, m.detachLocked(cur))
				}
			}
			m.plugins[id] = &managedPlugin{
				installed: p,
				spec:      spec,
				state:     StateStopped,
			}
			continue
		}
		if cur == nil {
			mp := &managedPlugin{
				installed: p,
				spec:      spec,
				state:     StateStopped,
			}
			m.plugins[id] = mp
			toAutoStart = append(toAutoStart, id)
			continue
		}
		changed := cur.installed.Entry.SHA256 != p.Entry.SHA256 ||
			cur.installed.RootDir != p.RootDir ||
			!slices.Equal(cur.installed.Entry.GrantedPermissions, p.Entry.GrantedPermissions) ||
			!reflect.DeepEqual(cur.spec, spec)
		cur.installed = p
		cur.spec = spec
		if changed {
			if cur.backoffTimer != nil {
				cur.backoffTimer.Stop()
				cur.backoffTimer = nil
			}
			cur.crashes = nil
			cur.lastError = ""
			if cur.startCancel != nil {
				cancels = append(cancels, cur.startCancel)
				cur.startCancel = nil
			}
			cur.startWait = nil
			if cur.peer != nil || cur.cmd != nil {
				toStop = append(toStop, m.detachLocked(cur))
			} else if cur.state == StateDegraded || cur.state == StateError {
				cur.state = StateStopped
			}
			toAutoStart = append(toAutoStart, id)
			continue
		}
		if cur.state == StateStopped && cur.peer == nil && cur.startWait == nil {
			cur.crashes = nil
			cur.lastError = ""
			toAutoStart = append(toAutoStart, id)
		}
	}
	for id, mp := range m.plugins {
		if !seen[id] {
			if mp.state != StateStopped {
				statusStopped = append(statusStopped, id)
			}
			if mp.backoffTimer != nil {
				mp.backoffTimer.Stop()
				mp.backoffTimer = nil
			}
			if mp.startCancel != nil {
				cancels = append(cancels, mp.startCancel)
				mp.startCancel = nil
			}
			mp.startWait = nil
			if mp.peer != nil || mp.cmd != nil {
				toStop = append(toStop, m.detachLocked(mp))
			}
			delete(m.plugins, id)
		}
	}
	m.mu.Unlock()

	for _, cfn := range cancels {
		cfn()
	}
	for _, mp := range toStop {
		m.terminateDetached(mp)
	}
	for _, id := range statusStopped {
		m.notifyStatus(id, StateStopped)
	}
	for _, id := range toAutoStart {
		go func(pid string) {
			ctx, cancel := context.WithTimeout(context.Background(), pluginBootTimeout)
			defer cancel()
			_ = m.EnsureStarted(ctx, pid)
		}(id)
	}
	return nil
}

func (m *Manager) specFor(p manifest.InstalledPlugin) *manifest.HostProcessSpec {
	if p.Manifest == nil {
		return nil
	}
	if m.opts.HostKind == HostKindApp {
		return p.Manifest.App
	}
	return p.Manifest.Daemon
}

// ShutdownAll gracefully terminates every running plugin process cleanly.
func (m *Manager) ShutdownAll() {
	m.mu.Lock()
	m.closed = true
	var active []*managedPlugin
	var cancels []context.CancelFunc
	for _, mp := range m.plugins {
		if mp.backoffTimer != nil {
			mp.backoffTimer.Stop()
			mp.backoffTimer = nil
		}
		if mp.startCancel != nil {
			cancels = append(cancels, mp.startCancel)
			mp.startCancel = nil
		}
		mp.startWait = nil
		if mp.peer != nil || mp.cmd != nil {
			active = append(active, m.detachLocked(mp))
		}
	}
	m.mu.Unlock()

	for _, cfn := range cancels {
		cfn()
	}

	var wg sync.WaitGroup
	for _, mp := range active {
		wg.Add(1)
		go func(proc *managedPlugin) {
			defer wg.Done()
			m.terminateDetached(proc)
		}(mp)
	}
	wg.Wait()
}

// HasKeepAliveRunning reports whether any enabled plugin in this host requires keeping hylad alive.
func (m *Manager) HasKeepAliveRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mp := range m.plugins {
		if mp.installed.Entry.Enabled && mp.spec != nil && mp.spec.KeepAlive &&
			(mp.state == StateRunning || mp.state == StateStarting) {
			return true
		}
	}
	return false
}

// Restart resets circuit breaker state and restarts an enabled plugin cleanly.
func (m *Manager) Restart(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	mp, ok := m.plugins[pluginID]
	if ok && (!mp.installed.Entry.Enabled || mp.installed.Entry.PendingInstall) {
		m.mu.Unlock()
		return rpc.NewError(rpc.ErrCodePermissionDenied, fmt.Sprintf("plugin %q is disabled", pluginID))
	}
	var detached *managedPlugin
	var prevStart chan struct{}
	var prevCancel context.CancelFunc
	wasActive := false
	if ok {
		wasActive = mp.state != StateStopped
		if mp.backoffTimer != nil {
			mp.backoffTimer.Stop()
			mp.backoffTimer = nil
		}
		mp.crashes = nil
		mp.lastError = ""
		if mp.startWait != nil {
			prevStart = mp.startWait
			prevCancel = mp.startCancel
			mp.startWait = nil
			mp.startCancel = nil
		}
		if mp.peer != nil || mp.cmd != nil {
			detached = m.detachLocked(mp)
		}
		mp.state = StateStopped
	}
	m.mu.Unlock()
	if prevCancel != nil {
		prevCancel()
	}
	if prevStart != nil {
		select {
		case <-prevStart:
		case <-ctx.Done():
			return rpc.NewError(rpc.ErrCodeTimeout, ctx.Err().Error())
		}
	}
	if detached != nil {
		m.terminateDetached(detached)
	}
	if wasActive {
		m.notifyStatus(pluginID, StateStopped)
	}
	return m.EnsureStarted(ctx, pluginID)
}

// EnsureStarted launches and initializes pluginID if it is enabled and not yet running.
func (m *Manager) EnsureStarted(ctx context.Context, pluginID string) error {
	for {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return rpc.NewError(rpc.ErrCodeTargetNotFound, "plugin host shutting down")
		}
		mp, ok := m.plugins[pluginID]
		if !ok {
			m.mu.Unlock()
			// Refresh once in case plugin was freshly linked/installed.
			if err := m.Reload(); err != nil {
				return err
			}
			m.mu.Lock()
			mp, ok = m.plugins[pluginID]
			if !ok {
				m.mu.Unlock()
				return rpc.NewError(rpc.ErrCodeTargetNotFound, fmt.Sprintf("plugin %q not found", pluginID))
			}
		}
		if !mp.installed.Entry.Enabled {
			m.mu.Unlock()
			return rpc.NewError(rpc.ErrCodePermissionDenied, fmt.Sprintf("plugin %q is disabled", pluginID))
		}
		if mp.spec == nil {
			m.mu.Unlock()
			return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, fmt.Sprintf("plugin %q has no %s host process", pluginID, m.opts.HostKind))
		}
		if mp.state == StateRunning && mp.peer != nil {
			m.mu.Unlock()
			return nil
		}
		if mp.state == StateDegraded {
			lastErr := mp.lastError
			m.mu.Unlock()
			return rpc.NewError(rpc.ErrCodeInternal, fmt.Sprintf("plugin %q degraded (crash circuit breaker): %s", pluginID, lastErr))
		}
		if mp.startWait != nil {
			wait := mp.startWait
			m.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return rpc.NewError(rpc.ErrCodeTimeout, ctx.Err().Error())
			}
		}
		if mp.backoffTimer != nil {
			mp.backoffTimer.Stop()
			mp.backoffTimer = nil
		}
		waitCh := make(chan struct{})
		startCtx, startCancel := context.WithCancel(ctx)
		mp.startWait = waitCh
		mp.startCancel = startCancel
		mp.state = StateStarting
		installed := mp.installed
		spec := *mp.spec
		m.mu.Unlock()
		m.notifyStatus(pluginID, StateStarting)

		err := m.startProcess(startCtx, pluginID, installed, &spec, waitCh)

		startCancel()
		var finalState string
		m.mu.Lock()
		mp = m.plugins[pluginID]
		if mp != nil && mp.startWait == waitCh {
			mp.startWait = nil
			mp.startCancel = nil
			if err != nil {
				if mp.installed.Entry.Enabled {
					mp.state = StateError
					mp.lastError = err.Error()
				} else {
					mp.state = StateStopped
				}
				finalState = mp.state
			}
		}
		m.mu.Unlock()
		close(waitCh)
		if err != nil && finalState != "" {
			m.notifyStatus(pluginID, finalState)
		}
		return err
	}
}

func (m *Manager) startProcess(ctx context.Context, id string, p manifest.InstalledPlugin, spec *manifest.HostProcessSpec, waitCh chan struct{}) error {
	dataDir, err := m.opts.Store.DataDir(id)
	if err != nil {
		return err
	}
	builtinFn := getBuiltinRunner(id, m.opts.HostKind)
	inProcessBuiltin := builtinFn != nil && p.Builtin

	procCtx, procCancel := context.WithCancel(context.Background())
	var (
		peer       *rpc.Peer
		cmd        *exec.Cmd
		cleanups   []func()
		acceptConn func(context.Context) error
		lw         *rotatingLogWriter
	)

	cleanupAll := func() {
		procCancel()
		if peer != nil {
			_ = peer.Close()
		}
		killProcessTree(cmd)
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Wait()
		}
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	if inProcessBuiltin {
		cHost, cPlugin := net.Pipe()
		peer = rpc.NewPeer(cHost, cHost, cHost)
		pluginPeer := rpc.NewPeer(cPlugin, cPlugin, cPlugin)
		go func() {
			defer cPlugin.Close()
			builtinFn(procCtx, pluginPeer)
		}()
	} else {
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return err
		}
		hasDualHosts := p.Manifest != nil && p.Manifest.Daemon != nil && p.Manifest.App != nil
		logPath, err := m.opts.Store.HostLogPath(id, m.opts.HostKind, hasDualHosts)
		if err != nil {
			return err
		}
		lw, err = newRotatingLogWriter(logPath)
		if err != nil {
			return err
		}
		cleanups = append(cleanups, func() { _ = lw.Close() })

		argv, err := resolveCommand(p.RootDir, spec.Command)
		if err != nil {
			cleanupAll()
			return err
		}
		cmd = exec.Command(argv[0], argv[1:]...)
		if p.RootDir != "" {
			cmd.Dir = p.RootDir
		}
		groupClean, err := setProcGroupAttr(cmd)
		if err != nil {
			cleanupAll()
			return err
		}
		cleanups = append(cleanups, groupClean)
		cmd.Stderr = lw

		env := append(os.Environ(),
			"HYLA_PLUGIN_ID="+id,
			"HYLA_PLUGIN_DIR="+p.RootDir,
			"HYLA_PLUGIN_DATA_DIR="+dataDir,
			"HYLA_PLUGIN_HOST="+m.opts.HostKind,
			"HYLA_PLUGIN_LOCALE="+m.opts.LocaleFunc(),
			"HYLA_PLUGIN_LOG="+logPath,
		)
		for k, v := range p.Manifest.Env {
			env = append(env, k+"="+v)
		}

		if spec.Transport == "socket" {
			ln, addr, sockClean, err := listenPluginSocket(id, dataDir, m.opts.HostKind)
			if err != nil {
				cleanupAll()
				return err
			}
			cleanups = append(cleanups, sockClean)
			tokenBytes := make([]byte, 32)
			_, _ = rand.Read(tokenBytes)
			token := hex.EncodeToString(tokenBytes)
			env = append(env, "HYLA_PLUGIN_SOCKET="+addr, "HYLA_PLUGIN_TOKEN="+token)
			cmd.Env = env
			cmd.Stdout = lw
			if err := cmd.Start(); err != nil {
				cleanupAll()
				return fmt.Errorf("start %s plugin: %w", id, err)
			}
			cleanups = append(cleanups, attachWindowsJob(cmd))
			acceptConn = func(waitCtx context.Context) error {
				connCh := make(chan net.Conn, 1)
				errCh := make(chan error, 1)
				go func() {
					c, aerr := ln.Accept()
					_ = ln.Close()
					if aerr != nil {
						errCh <- aerr
						return
					}
					connCh <- c
				}()
				select {
				case <-waitCtx.Done():
					_ = ln.Close()
					return rpc.NewError(rpc.ErrCodeTimeout, "timed out waiting for plugin socket connection")
				case aerr := <-errCh:
					return aerr
				case c := <-connCh:
					authedR, verr := rpc.VerifyAuthFrame(c, token)
					if verr != nil {
						_ = c.Close()
						return verr
					}
					peer = rpc.NewPeer(authedR, c, c)
					return nil
				}
			}
		} else {
			stdin, err := cmd.StdinPipe()
			if err != nil {
				cleanupAll()
				return err
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				cleanupAll()
				return err
			}
			cmd.Env = env
			if err := cmd.Start(); err != nil {
				cleanupAll()
				return fmt.Errorf("start %s plugin: %w", id, err)
			}
			cleanups = append(cleanups, attachWindowsJob(cmd))
			rwc := struct {
				io.Reader
				io.WriteCloser
			}{stdout, stdin}
			peer = rpc.NewPeer(rwc, rwc, rwc)
		}
	}

	if acceptConn != nil {
		waitCtx, waitCancel := context.WithTimeout(ctx, pluginStartTimeout)
		err := acceptConn(waitCtx)
		waitCancel()
		if err != nil {
			cleanupAll()
			return err
		}
	}

	m.registerBridgeHandlers(id, peer)
	peer.Start(procCtx)

	cfgMap, _ := m.opts.Store.LoadConfig(id)
	appName, dark := m.opts.ThemeFunc()
	initCtx, initCancel := context.WithTimeout(ctx, pluginInitTimeout)
	defer initCancel()

	var initRes rpc.InitializeResult
	err = peer.Call(initCtx, "initialize", rpc.InitializeParams{
		HylaVersion:        m.opts.HylaVersion,
		Host:               m.opts.HostKind,
		PluginID:           id,
		PluginDir:          p.RootDir,
		DataDir:            dataDir,
		Locale:             m.opts.LocaleFunc(),
		Appearance:         appName,
		Dark:               dark,
		GrantedPermissions: append([]string(nil), p.Entry.GrantedPermissions...),
		Settings:           cfgMap,
		Capabilities:       []string{proto.CapPluginsV1},
	}, &initRes)
	if err != nil {
		cleanupAll()
		return fmt.Errorf("initialize %s: %w", id, err)
	}
	_ = peer.Notify("initialized", struct{}{})

	pid := 0
	if cmd != nil && cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cleanupAll()
		return rpc.NewError(rpc.ErrCodeTargetNotFound, "plugin host shutting down")
	}
	mp := m.plugins[id]
	if mp == nil || mp.startWait != waitCh || !mp.installed.Entry.Enabled {
		m.mu.Unlock()
		cleanupAll()
		return rpc.NewError(rpc.ErrCodeTargetNotFound, "plugin start superseded, removed, or disabled")
	}
	waitDone := make(chan struct{})
	runCleanup := sync.OnceFunc(func() {
		procCancel()
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	})

	mp.peer = peer
	mp.cmd = cmd
	mp.logWriter = lw
	mp.cleanup = cleanups
	mp.runCleanup = runCleanup
	mp.waitDone = waitDone
	mp.cancel = procCancel
	mp.pid = pid
	mp.startedAt = time.Now()
	mp.state = StateRunning
	mp.lastError = ""
	mp.subscriptions = initRes.Subscriptions
	mp.capabilities = initRes.Capabilities
	mp.stopping = false
	m.mu.Unlock()

	go m.watchProcess(id, mp, cmd, peer, waitDone, runCleanup)
	m.notifyStatus(id, StateRunning)
	return nil
}

func resolveCommand(rootDir string, argv []string) ([]string, error) {
	return resolveCommandForOS(rootDir, argv, runtime.GOOS)
}

func resolveCommandForOS(rootDir string, argv []string, goos string) ([]string, error) {
	if len(argv) == 0 || argv[0] == "" {
		return nil, fmt.Errorf("empty command")
	}
	out := make([]string, len(argv))
	copy(out, argv)
	cmd0 := out[0]
	if !filepath.IsAbs(cmd0) && strings.ContainsRune(cmd0, '/') && rootDir != "" {
		resolved, err := manifest.ResolveWithin(rootDir, cmd0)
		if err != nil {
			return nil, err
		}
		if goos == "windows" {
			resolved = preferWindowsExe(resolved)
		}
		out[0] = resolved
	} else if !filepath.IsAbs(cmd0) && rootDir != "" {
		cand := filepath.Join(rootDir, cmd0)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			out[0] = cand
		} else if goos == "windows" {
			if exe := preferWindowsExe(cand); exe != cand {
				out[0] = exe
			}
		}
	}
	return out, nil
}

func preferWindowsExe(path string) string {
	if strings.HasSuffix(strings.ToLower(path), ".exe") {
		return path
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return path
	}
	exe := path + ".exe"
	if st, err := os.Stat(exe); err == nil && !st.IsDir() {
		return exe
	}
	return path
}

func (m *Manager) watchProcess(id string, mp *managedPlugin, cmd *exec.Cmd, peer *rpc.Peer, waitDone chan struct{}, runCleanup func()) {
	var waitErr error
	if cmd != nil {
		waitErr = cmd.Wait()
	} else {
		<-peer.Done()
		waitErr = peer.Err()
	}
	close(waitDone)
	_ = peer.Close()
	if runCleanup != nil {
		runCleanup()
	}

	m.mu.Lock()
	cur := m.plugins[id]
	if cur != mp || mp.peer != peer {
		// Already replaced/detached.
		m.mu.Unlock()
		return
	}
	expectedStop := m.closed || mp.stopping || !mp.installed.Entry.Enabled
	mp.peer = nil
	mp.cmd = nil
	mp.pid = 0
	mp.logWriter = nil
	mp.cleanup = nil
	mp.cancel = nil
	if expectedStop {
		mp.state = StateStopped
		m.mu.Unlock()
		m.notifyStatus(id, StateStopped)
		return
	}

	now := time.Now()
	cutoff := now.Add(-60 * time.Second)
	var recent []time.Time
	for _, ts := range mp.crashes {
		if ts.After(cutoff) {
			recent = append(recent, ts)
		}
	}
	recent = append(recent, now)
	mp.crashes = recent
	if waitErr != nil {
		mp.lastError = waitErr.Error()
	} else {
		mp.lastError = "process exited unexpectedly"
	}

	maxCrashes := 5
	if mp.spec != nil && mp.spec.MaxCrashesPerMinute > 0 {
		maxCrashes = mp.spec.MaxCrashesPerMinute
	}
	if len(recent) >= maxCrashes {
		mp.state = StateDegraded
		m.mu.Unlock()
		m.notifyStatus(id, StateDegraded)
		return
	}

	mp.state = StateError
	backoff := crashBackoff(len(recent), mp.spec)
	shouldRestart := mp.spec != nil && mp.installed.Entry.Enabled
	if shouldRestart {
		mp.backoffTimer = time.AfterFunc(backoff, func() {
			ctx, c := context.WithTimeout(context.Background(), pluginBootTimeout)
			defer c()
			_ = m.EnsureStarted(ctx, id)
		})
	}
	m.mu.Unlock()
	m.notifyStatus(id, StateError)
}

func crashBackoff(crashesInWindow int, spec *manifest.HostProcessSpec) time.Duration {
	steps := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second}
	idx := max(0, min(crashesInWindow-1, len(steps)-1))
	d := steps[idx]
	if spec != nil && spec.MinRestartMS > 0 {
		minD := time.Duration(spec.MinRestartMS) * time.Millisecond
		scale := minD / time.Second
		if scale > 1 {
			d = d * scale
			if d > 60*time.Second {
				d = 60 * time.Second
			}
		} else if d < minD {
			d = minD
		}
	}
	return d
}

func (m *Manager) detachLocked(mp *managedPlugin) *managedPlugin {
	mp.stopping = true
	if mp.backoffTimer != nil {
		mp.backoffTimer.Stop()
		mp.backoffTimer = nil
	}
	cp := &managedPlugin{
		installed:  mp.installed,
		spec:       mp.spec,
		peer:       mp.peer,
		cmd:        mp.cmd,
		logWriter:  mp.logWriter,
		cleanup:    mp.cleanup,
		runCleanup: mp.runCleanup,
		waitDone:   mp.waitDone,
		cancel:     mp.cancel,
	}
	mp.peer = nil
	mp.cmd = nil
	mp.pid = 0
	mp.logWriter = nil
	mp.cleanup = nil
	mp.runCleanup = nil
	mp.waitDone = nil
	mp.cancel = nil
	mp.state = StateStopped
	return cp
}

func (m *Manager) terminateDetached(mp *managedPlugin) {
	if mp == nil {
		return
	}
	if mp.peer != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = mp.peer.Call(shutCtx, "shutdown", struct{}{}, nil)
		cancel()
		_ = mp.peer.Notify("exit", struct{}{})
		_ = mp.peer.Close()
	}
	if mp.cmd != nil && mp.cmd.Process != nil && mp.waitDone != nil {
		select {
		case <-mp.waitDone:
		case <-time.After(1 * time.Second):
			killProcessTree(mp.cmd)
			<-mp.waitDone
		}
	} else if mp.waitDone != nil {
		<-mp.waitDone
	}
	if mp.runCleanup != nil {
		mp.runCleanup()
	}
}

// Call invokes a Host -> Plugin RPC method with permission gating and lazy activation.
func (m *Manager) Call(ctx context.Context, pluginID, method string, params any, out any) error {
	m.mu.Lock()
	_, known := m.plugins[pluginID]
	m.mu.Unlock()
	if !known {
		if err := m.Reload(); err != nil {
			return err
		}
	}
	if !m.allowHostToPluginCall(pluginID, method) {
		return rpc.NewError(rpc.ErrCodePermissionDenied, fmt.Sprintf("plugin %q lacks permission for %s", pluginID, method))
	}
	if err := m.EnsureStarted(ctx, pluginID); err != nil {
		return err
	}
	m.mu.Lock()
	mp := m.plugins[pluginID]
	var peer *rpc.Peer
	if mp != nil {
		peer = mp.peer
	}
	m.mu.Unlock()
	if peer == nil {
		return rpc.NewError(rpc.ErrCodeTargetNotFound, fmt.Sprintf("plugin %q is not running", pluginID))
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	return peer.Call(ctx, method, params, out)
}

// NotifyTopic sends event.notify to all running plugins subscribed to topic (or "*").
func (m *Manager) NotifyTopic(topic string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	params := rpc.EventNotifyParams{Topic: topic, Data: raw}

	m.mu.Lock()
	var targets []*rpc.Peer
	for _, mp := range m.plugins {
		if mp.peer == nil || mp.state != StateRunning {
			continue
		}
		if len(mp.subscriptions) == 0 || slices.Contains(mp.subscriptions, topic) || slices.Contains(mp.subscriptions, "*") {
			targets = append(targets, mp.peer)
		}
	}
	m.mu.Unlock()

	for _, peer := range targets {
		_ = peer.Notify("event.notify", params)
	}
}

// NotifyPlugin sends event.notify to a specific running plugin.
func (m *Manager) NotifyPlugin(pluginID, topic string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	m.mu.Lock()
	mp := m.plugins[pluginID]
	var peer *rpc.Peer
	if mp != nil && mp.state == StateRunning {
		peer = mp.peer
	}
	m.mu.Unlock()
	if peer != nil {
		_ = peer.Notify("event.notify", rpc.EventNotifyParams{Topic: topic, Data: raw})
	}
}

// DiffAndNotifyState computes state deltas and emits session.state / pane.status / pane.osc / theme.changed.
func (m *Manager) DiffAndNotifyState(prev, next proto.State, prevDark, nextDark bool, nextTheme string) {
	if prevDark != nextDark {
		m.NotifyTopic("theme.changed", map[string]any{
			"dark":      nextDark,
			"themeName": nextTheme,
		})
	}

	prevSess := make(map[string]proto.Session, len(prev.Sessions))
	prevPane := make(map[string]proto.Pane)
	for _, s := range prev.Sessions {
		prevSess[s.ID] = s
		for _, p := range s.Panes {
			prevPane[p.ID] = p
		}
	}

	changedState := len(prev.Sessions) != len(next.Sessions)
	for _, s := range next.Sessions {
		ps, ok := prevSess[s.ID]
		if !ok || ps.Name != s.Name || ps.Focus != s.Focus || len(ps.Panes) != len(s.Panes) {
			changedState = true
		}
		for _, p := range s.Panes {
			pp, ok := prevPane[p.ID]
			if !ok {
				changedState = true
				m.NotifyTopic("pane.status", map[string]any{
					"sessionId": s.ID,
					"paneId":    p.ID,
					"status":    p.Status,
					"message":   p.Message,
					"progress":  p.Progress,
					"kind":      p.Kind,
					"agent":     p.Agent,
				})
				continue
			}
			if pp.Status != p.Status || pp.Message != p.Message || pp.Progress != p.Progress || pp.Kind != p.Kind || pp.Agent != p.Agent {
				m.NotifyTopic("pane.status", map[string]any{
					"sessionId": s.ID,
					"paneId":    p.ID,
					"status":    p.Status,
					"message":   p.Message,
					"progress":  p.Progress,
					"kind":      p.Kind,
					"agent":     p.Agent,
				})
			}
			if len(pp.Records) != len(p.Records) || pp.Title != p.Title {
				m.NotifyTopic("pane.osc", map[string]any{
					"sessionId": s.ID,
					"paneId":    p.ID,
					"title":     p.Title,
					"records":   p.Records,
				})
			}
		}
	}
	if changedState {
		m.NotifyTopic("session.state", map[string]any{
			"sessions": len(next.Sessions),
			"projects": len(next.Projects),
		})
	}
}

// ListStatuses returns merged manifest + runtime status entries for all discovered plugins.
func (m *Manager) ListStatuses() []rpc.PluginStatus {
	list, err := m.opts.Store.Discover()
	if err != nil {
		list = nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(list) == 0 && len(m.plugins) > 0 {
		for _, mp := range m.plugins {
			list = append(list, mp.installed)
		}
	}
	out := make([]rpc.PluginStatus, 0, len(list))
	now := time.Now()
	locale := ""
	if m.opts.LocaleFunc != nil {
		locale = m.opts.LocaleFunc()
	}
	for _, p := range list {
		p = manifest.LocalizePlugin(p, locale)
		st := rpc.PluginStatus{
			ID:                 p.Entry.ID,
			Enabled:            p.Entry.Enabled,
			Source:             p.Entry.Source,
			LinkPath:           p.Entry.LinkPath,
			PendingInstall:     p.Entry.PendingInstall,
			Builtin:            p.Builtin,
			LoadError:          p.LoadError,
			GrantedPermissions: append([]string(nil), p.Entry.GrantedPermissions...),
			State:              StateStopped,
		}
		if p.Manifest != nil {
			mf := p.Manifest
			st.Name = mf.Name
			st.Version = mf.Version
			st.Description = mf.Description
			st.Author = mf.Author
			st.Homepage = mf.Homepage
			st.RequestedPermissions = append([]string(nil), mf.Permissions.Scopes...)
			st.Slots = append([]manifest.SlotSpec(nil), mf.UI.Slots...)
			st.SettingsSchema = append([]manifest.SettingSpec(nil), mf.Settings...)
			st.Actions = append([]manifest.ActionSpec(nil), mf.Actions...)
			switch {
			case mf.Daemon != nil && mf.App != nil:
				st.Host = "both"
			case mf.Daemon != nil:
				st.Host = HostKindDaemon
			case mf.App != nil:
				st.Host = HostKindApp
			default:
				st.Host = "pack"
			}
			if mf.Daemon != nil {
				st.KeepAlive = mf.Daemon.KeepAlive
			}
		} else {
			st.Name = p.Entry.ID
			st.Version = p.Entry.Version
		}
		if mp := m.plugins[p.Entry.ID]; mp != nil {
			st.State = mp.state
			st.PID = mp.pid
			st.CrashCount = len(mp.crashes)
			st.LastError = mp.lastError
			if mp.state == StateRunning && !mp.startedAt.IsZero() {
				st.UptimeMS = now.Sub(mp.startedAt).Milliseconds()
			}
		}
		if !p.Entry.Enabled || p.Entry.PendingInstall {
			st.State = StateStopped
			st.PID = 0
			st.LastError = ""
			st.UptimeMS = 0
		}
		out = append(out, st)
	}
	return out
}

// Logs tails the stderr log file(s) of pluginID.
func (m *Manager) Logs(pluginID string, lines int) (rpc.PluginLogsResult, error) {
	p, txt, count, err := ReadPluginLogs(m.opts.Store, pluginID, lines)
	if err != nil {
		return rpc.PluginLogsResult{}, err
	}
	return rpc.PluginLogsResult{Path: p, Text: txt, Lines: count}, nil
}

// RunningForAgent returns the plugin ID providing agent if any active plugin matches.
func (m *Manager) PluginForAgent(agent string) (string, bool) {
	if meta, ok := proto.AgentProvider(agent); ok && meta.PluginID != "" {
		return meta.PluginID, true
	}
	return "", false
}

func (m *Manager) hasPermission(pluginID, scope string) bool {
	if scope == "" {
		return true
	}
	m.mu.Lock()
	mp := m.plugins[pluginID]
	m.mu.Unlock()
	if mp == nil {
		return false
	}
	if !mp.installed.Entry.Enabled {
		return false
	}
	return mp.installed.Entry.HasPermission(scope)
}

func (m *Manager) allowHostToPluginCall(pluginID, method string) bool {
	switch method {
	case "agent.detect", "agent.resumeCommand", "agent.sendChat":
		return m.hasPermission(pluginID, manifest.PermAgentProvider)
	case "agent.readTimeline", "agent.readChat", "agent.readSubagents":
		return m.hasPermission(pluginID, manifest.PermAgentProvider) || m.hasPermission(pluginID, manifest.PermTimelineRead)
	case "ui.render", "ui.event":
		return m.hasPermission(pluginID, manifest.PermUISlot)
	case "webview.rpc":
		return m.hasPermission(pluginID, manifest.PermUIWebView)
	default:
		return true
	}
}

func requiredPluginToHostPerm(method string) string {
	switch method {
	case "host.getState":
		return manifest.PermSessionRead
	case "pane.readScreen", "pane.getEnv":
		return manifest.PermPaneRead
	case "pane.getTimeline":
		return manifest.PermTimelineRead
	case "session.create":
		return manifest.PermSessionCreate
	case "pane.sendInput", "pane.sendKey", "pane.reportStatus":
		return manifest.PermPaneWrite
	case "pane.focus":
		return manifest.PermPaneFocus
	case "ui.invalidate", "ui.openPane":
		return manifest.PermUISlot
	case "ui.notify":
		return manifest.PermNotifications
	case "webview.emit":
		return manifest.PermUIWebView
	default:
		return ""
	}
}

func (m *Manager) checkBridgePerm(id, method string) error {
	req := requiredPluginToHostPerm(method)
	if req != "" && !m.hasPermission(id, req) {
		return rpc.NewError(rpc.ErrCodePermissionDenied, fmt.Sprintf("plugin %q lacks permission %q for %s", id, req, method))
	}
	return nil
}

func handleBridgeCall[P any, R any](m *Manager, id string, peer *rpc.Peer, method string, allowEmpty bool, fn func(context.Context, string, P) (R, error)) {
	peer.Handle(method, func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		if err := m.checkBridgePerm(id, method); err != nil {
			return nil, err
		}
		if m.opts.Bridge == nil {
			return nil, rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "bridge unavailable")
		}
		var p P
		if !allowEmpty || len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				if !allowEmpty {
					return nil, rpc.NewError(rpc.ErrCodeInvalidParams, err.Error())
				}
			}
		}
		return fn(ctx, id, p)
	})
}

func handleBridgeExec[P any](m *Manager, id string, peer *rpc.Peer, method string, optionalBridge, allowEmpty bool, fn func(context.Context, string, P) error) {
	peer.Handle(method, func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		if err := m.checkBridgePerm(id, method); err != nil {
			return nil, err
		}
		if m.opts.Bridge == nil {
			if optionalBridge {
				return struct{}{}, nil
			}
			return nil, rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "bridge unavailable")
		}
		var p P
		if !allowEmpty || len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				if !allowEmpty {
					return nil, rpc.NewError(rpc.ErrCodeInvalidParams, err.Error())
				}
			}
		}
		return struct{}{}, fn(ctx, id, p)
	})
}

func (m *Manager) registerBridgeHandlers(id string, peer *rpc.Peer) {
	handleBridgeCall(m, id, peer, "host.getState", true, func(ctx context.Context, id string, p rpc.HostGetStateParams) (*rpc.HostGetStateResult, error) {
		return m.opts.Bridge.GetState(ctx, id, p)
	})
	handleBridgeCall(m, id, peer, "pane.readScreen", false, func(ctx context.Context, id string, p rpc.PaneReadScreenParams) (*rpc.PaneReadScreenResult, error) {
		return m.opts.Bridge.ReadScreen(ctx, id, p)
	})
	handleBridgeCall(m, id, peer, "pane.getEnv", false, func(ctx context.Context, id string, p rpc.PaneGetEnvParams) (*rpc.PaneGetEnvResult, error) {
		return m.opts.Bridge.GetEnv(ctx, id, p)
	})
	handleBridgeCall(m, id, peer, "pane.getTimeline", false, func(ctx context.Context, id string, p rpc.PaneGetTimelineParams) (*rpc.PaneGetTimelineResult, error) {
		return m.opts.Bridge.GetTimeline(ctx, id, p)
	})
	handleBridgeCall(m, id, peer, "session.create", false, func(ctx context.Context, id string, p proto.CreateParams) (*rpc.SessionCreateResult, error) {
		return m.opts.Bridge.CreateSession(ctx, id, p)
	})

	handleBridgeExec(m, id, peer, "pane.sendInput", false, false, func(ctx context.Context, id string, p rpc.PaneSendInputParams) error {
		return m.opts.Bridge.SendInput(ctx, id, p)
	})
	handleBridgeExec(m, id, peer, "pane.sendKey", false, false, func(ctx context.Context, id string, p rpc.PaneSendKeyParams) error {
		return m.opts.Bridge.SendKey(ctx, id, p)
	})
	handleBridgeExec(m, id, peer, "pane.reportStatus", false, false, func(ctx context.Context, id string, p rpc.PaneReportStatusParams) error {
		return m.opts.Bridge.ReportStatus(ctx, id, p)
	})
	handleBridgeExec(m, id, peer, "pane.focus", false, false, func(ctx context.Context, id string, p rpc.PaneFocusParams) error {
		return m.opts.Bridge.FocusPane(ctx, id, p)
	})
	handleBridgeExec(m, id, peer, "ui.invalidate", true, true, func(ctx context.Context, id string, p rpc.UIInvalidateParams) error {
		return m.opts.Bridge.InvalidateUI(ctx, id, p)
	})
	handleBridgeExec(m, id, peer, "ui.openPane", false, false, func(ctx context.Context, id string, p rpc.UIOpenPaneParams) error {
		if p.Plugin == "" {
			p.Plugin = id
		}
		return m.opts.Bridge.OpenPane(ctx, id, p)
	})
	handleBridgeExec(m, id, peer, "ui.notify", true, false, func(ctx context.Context, id string, p rpc.UINotifyParams) error {
		return m.opts.Bridge.NotifyUI(ctx, id, p)
	})
	handleBridgeExec(m, id, peer, "webview.emit", false, false, func(ctx context.Context, id string, p rpc.WebViewEmitParams) error {
		return m.opts.Bridge.WebViewEmit(ctx, id, p)
	})

	peer.Handle("plugin.getSettings", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.PluginGetSettingsParams
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &p)
		}
		cfg, err := m.opts.Store.LoadConfig(id)
		if err != nil {
			return nil, err
		}
		if len(p.Keys) > 0 {
			filtered := make(map[string]any, len(p.Keys))
			for _, k := range p.Keys {
				if v, ok := cfg[k]; ok {
					filtered[k] = v
				}
			}
			cfg = filtered
		}
		res := rpc.PluginGetSettingsResult{
			Settings:   cfg,
			SecretsSet: map[string]bool{},
		}
		m.mu.Lock()
		mp := m.plugins[id]
		var schema []manifest.SettingSpec
		if mp != nil && mp.installed.Manifest != nil {
			schema = mp.installed.Manifest.Settings
		}
		m.mu.Unlock()
		if p.IncludeSecrets && !m.hasPermission(id, manifest.PermSecrets) {
			return nil, rpc.NewError(rpc.ErrCodePermissionDenied, fmt.Sprintf("plugin %q lacks permission %q", id, manifest.PermSecrets))
		}
		if p.IncludeSecrets {
			res.Secrets = map[string]string{}
		}
		for _, spec := range schema {
			if spec.Type != "secret" {
				continue
			}
			val, ok, err := m.opts.Store.GetSecret(id, spec.Key)
			if err == nil && ok {
				res.SecretsSet[spec.Key] = true
				if p.IncludeSecrets {
					res.Secrets[spec.Key] = val
				}
			}
		}
		return res, nil
	})

	peer.Handle("plugin.setSettings", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.PluginSetSettingsParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, rpc.NewError(rpc.ErrCodeInvalidParams, err.Error())
		}
		if len(p.Secrets) > 0 && !m.hasPermission(id, manifest.PermSecrets) {
			return nil, rpc.NewError(rpc.ErrCodePermissionDenied, fmt.Sprintf("plugin %q lacks permission %q", id, manifest.PermSecrets))
		}
		if p.Settings != nil {
			cur, err := m.opts.Store.LoadConfig(id)
			if err != nil {
				return nil, err
			}
			for k, v := range p.Settings {
				if v == nil {
					delete(cur, k)
				} else {
					cur[k] = v
				}
			}
			if err := m.opts.Store.SaveConfig(id, cur); err != nil {
				return nil, err
			}
		}
		for k, v := range p.Secrets {
			if v == "" {
				if err := m.opts.Store.DeleteSecret(id, k); err != nil {
					return nil, err
				}
			} else if err := m.opts.Store.SetSecret(id, k, v); err != nil {
				return nil, err
			}
		}
		if m.opts.OnSettingsSaved != nil && (p.Settings != nil || len(p.Secrets) > 0) {
			m.opts.OnSettingsSaved(id)
		}
		return struct{}{}, nil
	})
}
