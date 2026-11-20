package uihost

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/daemonhost"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
)

// DaemonCaller abstracts calling a plugin hosted in local or remote hylad.
type DaemonCaller interface {
	CallPlugin(pluginID, method string, params any, out any) error
	PluginStatuses() []rpc.PluginStatus
}

// SlotEntry describes an enabled plugin UI slot ready for a mount point.
type SlotEntry struct {
	PluginID  string
	PluginDir string
	Slot      manifest.SlotSpec
	Host      string // "app" | "daemon"
}

// CachedRender holds the non-blocking cached render state for a slot+context key.
type CachedRender struct {
	Result    *rpc.UIRenderResult
	Loading   bool
	Error     string
	FetchedAt time.Time
	InFlight  bool
	Invalid   bool
	hasReq    bool
	reqDark   bool
	reqLocale string
}

type renderKey struct {
	hostKey   string
	pluginID  string
	slotID    string
	sessionID string
	paneID    string
}

// Coordinator ties together local [app] plugin processes, Declarative UI rendering cache,
// Sandboxed WebView bridge, and daemon plugin RPC calls.
type Coordinator struct {
	AppMgr     *daemonhost.Manager
	Web        *WebBridge
	State      *RenderState
	Store      *manifest.Store
	OnUpdate   func()
	OnMessage  func(msg string)
	DaemonFn   func(hostKey string) DaemonCaller
	RemoteRef  func(hostKey string) bool
	PaneHostFn func(sessionID, paneID string) string
	LocaleFunc            func() string
	ThemeFunc             func() (appearance string, dark bool)
	OnPluginSettingsSaved func(pluginID string)
	OnStatusChange        func(pluginID, state string)

	mu          sync.Mutex
	basePlugins []manifest.InstalledPlugin
	snap        *manifest.Snapshot
	slots       []SlotEntry
	renders     map[renderKey]*CachedRender
}

// NewCoordinator constructs the UI plugin coordinator.
func NewCoordinator(store *manifest.Store, appBridge daemonhost.HostBridge, hylaVersion string, onUpdate func()) *Coordinator {
	if store == nil {
		store = manifest.DefaultStore()
	}
	c := &Coordinator{
		State:    NewRenderState(),
		Store:    store,
		OnUpdate: onUpdate,
		renders:  make(map[renderKey]*CachedRender),
	}
	c.AppMgr = daemonhost.NewManager(daemonhost.ManagerOptions{
		HostKind:    daemonhost.HostKindApp,
		HylaVersion: hylaVersion,
		Store:       store,
		Bridge:      appBridge,
		LocaleFunc: func() string {
			if c.LocaleFunc != nil {
				return c.LocaleFunc()
			}
			return "en"
		},
		ThemeFunc: func() (string, bool) {
			if c.ThemeFunc != nil {
				return c.ThemeFunc()
			}
			return "system", false
		},
		OnSettingsSaved: func(pluginID string) {
			if c.OnPluginSettingsSaved != nil {
				c.OnPluginSettingsSaved(pluginID)
			}
		},
		OnStatusChange: func(pluginID, state string) {
			if c.OnStatusChange != nil {
				c.OnStatusChange(pluginID, state)
			}
		},
	})
	c.Web = NewWebBridge(store, nil, c.handleWebRPC)
	c.Web.SetLocaleFunc(c.currentLocale)
	return c
}

func (c *Coordinator) currentLocale() string {
	if c.LocaleFunc != nil {
		return strings.TrimSpace(c.LocaleFunc())
	}
	return ""
}

// Reload discovers enabled plugins, refreshes slot metadata, and syncs AppMgr.
func (c *Coordinator) Reload() (*manifest.Snapshot, error) {
	base, err := c.Store.Discover()
	if err != nil {
		return nil, err
	}
	baseCopy := slices.Clone(base)
	snap := manifest.ApplySnapshotInLanguage(baseCopy, c.currentLocale())
	c.mu.Lock()
	c.basePlugins = baseCopy
	c.mu.Unlock()
	return c.applyReloadedSnapshot(snap)
}

// ReloadWithSnapshot refreshes slot metadata and AppMgr from an already-computed snapshot.
func (c *Coordinator) ReloadWithSnapshot(snap *manifest.Snapshot) (*manifest.Snapshot, error) {
	if snap == nil {
		return c.Reload()
	}
	baseCopy := slices.Clone(snap.Plugins)
	localized := manifest.ApplySnapshotInLanguage(baseCopy, c.currentLocale())
	c.mu.Lock()
	c.basePlugins = baseCopy
	c.mu.Unlock()
	return c.applyReloadedSnapshot(localized)
}

func (c *Coordinator) applyReloadedSnapshot(snap *manifest.Snapshot) (*manifest.Snapshot, error) {
	slots := slotsFromSnapshot(snap)

	c.mu.Lock()
	c.snap = snap
	c.slots = slots
	clear(c.renders)
	c.mu.Unlock()

	if c.State != nil {
		c.State.ClearAssets()
	}
	if c.Web != nil {
		c.Web.SetPluginsSnapshot(snap.Plugins)
	}
	if c.AppMgr != nil {
		_ = c.AppMgr.ReloadWithPlugins(snap.Plugins)
	}
	return snap, nil
}

// ApplyLocale updates cached plugin manifests, actions, slots, and running plugins/WebViews for locale
// without restarting child processes.
func (c *Coordinator) ApplyLocale(locale string) {
	c.mu.Lock()
	baseCopy := slices.Clone(c.basePlugins)
	c.mu.Unlock()
	if len(baseCopy) == 0 && c.Store != nil {
		if discovered, err := c.Store.Discover(); err == nil {
			baseCopy = slices.Clone(discovered)
			c.mu.Lock()
			c.basePlugins = slices.Clone(baseCopy)
			c.mu.Unlock()
		}
	}
	if len(baseCopy) > 0 {
		snap := manifest.ApplySnapshotInLanguage(baseCopy, locale)
		slots := slotsFromSnapshot(snap)
		c.mu.Lock()
		c.snap = snap
		c.slots = slots
		clear(c.renders)
		c.mu.Unlock()
		if c.Web != nil {
			c.Web.SetPluginsSnapshot(snap.Plugins)
		}
	} else {
		c.mu.Lock()
		clear(c.renders)
		c.mu.Unlock()
	}
	if c.AppMgr != nil {
		c.AppMgr.NotifyTopic("locale.changed", map[string]any{"locale": locale})
	}
	if c.Web != nil {
		c.Web.BroadcastLocaleChange(locale)
	}
	if c.OnUpdate != nil {
		c.OnUpdate()
	}
}

func slotsFromSnapshot(snap *manifest.Snapshot) []SlotEntry {
	var slots []SlotEntry
	if snap == nil {
		return slots
	}
	for _, p := range snap.Plugins {
		if !p.Entry.Enabled || p.Entry.PendingInstall || p.Manifest == nil {
			continue
		}
		hasSlotPerm := p.Entry.HasPermission(manifest.PermUISlot)
		hasWebPerm := p.Entry.HasPermission(manifest.PermUIWebView)
		for _, s := range p.Manifest.UI.Slots {
			if s.Mode == "webview" && !hasWebPerm {
				continue
			}
			if s.Mode != "webview" && !hasSlotPerm {
				continue
			}
			host := s.Host
			if host == "" {
				if p.Manifest.App != nil {
					host = daemonhost.HostKindApp
				} else {
					host = daemonhost.HostKindDaemon
				}
			}
			slots = append(slots, SlotEntry{
				PluginID:  p.Entry.ID,
				PluginDir: p.RootDir,
				Slot:      s,
				Host:      host,
			})
		}
	}
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].Slot.Order != slots[j].Slot.Order {
			return slots[i].Slot.Order < slots[j].Slot.Order
		}
		if slots[i].PluginID != slots[j].PluginID {
			return slots[i].PluginID < slots[j].PluginID
		}
		return slots[i].Slot.ID < slots[j].Slot.ID
	})
	return slots
}

// Shutdown stops all [app] plugin child processes cleanly.
func (c *Coordinator) Shutdown() {
	if c.AppMgr != nil {
		c.AppMgr.ShutdownAll()
	}
}

// Snapshot returns the most recently loaded base plugin snapshot.
func (c *Coordinator) Snapshot() *manifest.Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snap
}

// Slots returns enabled slots matching mount and optional session/pane "when" context.
func (c *Coordinator) Slots(mount string, s *proto.Session, p *proto.Pane) []SlotEntry {
	c.mu.Lock()
	all := make([]SlotEntry, len(c.slots))
	copy(all, c.slots)
	c.mu.Unlock()

	var out []SlotEntry
	for _, sl := range all {
		if mount != "" && sl.Slot.Mount != mount {
			continue
		}
		if !MatchesWhen(sl.Slot.When, s, p) {
			continue
		}
		out = append(out, sl)
	}
	return out
}

// FindSlot locates a declared slot (for custom plugin pane rendering/title/mode).
func (c *Coordinator) FindSlot(pluginID, slotID string) (SlotEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, sl := range c.slots {
		if sl.PluginID != pluginID {
			continue
		}
		if slotID == "" || sl.Slot.ID == slotID {
			return sl, true
		}
	}
	return SlotEntry{}, false
}

// IsPluginEnabled reports whether pluginID is currently installed and enabled locally.
func (c *Coordinator) IsPluginEnabled(pluginID string) bool {
	c.mu.Lock()
	snap := c.snap
	c.mu.Unlock()
	if snap == nil {
		return false
	}
	for _, p := range snap.Plugins {
		if p.Entry.ID == pluginID {
			return p.Entry.Enabled && !p.Entry.PendingInstall && p.Manifest != nil
		}
	}
	return false
}

// Invalidate marks cached slot renders stale and requests a UI redraw.
func (c *Coordinator) Invalidate(pluginID string, slotIDs []string) {
	c.mu.Lock()
	for k, cr := range c.renders {
		if pluginID != "" && k.pluginID != pluginID {
			continue
		}
		if len(slotIDs) == 0 || slices.Contains(slotIDs, k.slotID) {
			cr.Invalid = true
		}
	}
	cb := c.OnUpdate
	c.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// RenderSlot returns the cached render state immediately without blocking the UI thread,
// scheduling an asynchronous ui.render fetch if needed.
func (c *Coordinator) RenderSlot(hostKey string, sl SlotEntry, req rpc.UIRenderParams) CachedRender {
	cacheKey := renderKey{
		hostKey:   hostKey,
		pluginID:  sl.PluginID,
		slotID:    sl.Slot.ID,
		sessionID: req.SessionID,
		paneID:    req.PaneID,
	}

	c.mu.Lock()
	cr := c.renders[cacheKey]
	if cr == nil {
		cr = &CachedRender{}
		c.renders[cacheKey] = cr
	}
	needFetch := false
	themeOrLocaleChanged := cr.hasReq && (cr.reqDark != req.Dark || cr.reqLocale != req.Locale)
	switch {
	case cr.InFlight:
		if themeOrLocaleChanged {
			cr.Invalid = true
		}
	case cr.Result == nil && cr.Error == "":
		needFetch = true
	case cr.Invalid || themeOrLocaleChanged:
		needFetch = true
	case cr.Result != nil && cr.Result.RefreshMS > 0 && time.Since(cr.FetchedAt) >= time.Duration(cr.Result.RefreshMS)*time.Millisecond:
		needFetch = true
	case cr.Error != "" && time.Since(cr.FetchedAt) >= 5*time.Second:
		needFetch = true
	}
	if needFetch {
		cr.InFlight = true
		cr.Loading = cr.Result == nil
		cr.Invalid = false
		cr.hasReq = true
		cr.reqDark = req.Dark
		cr.reqLocale = req.Locale
	}
	snapshot := *cr
	cb := c.OnUpdate
	c.mu.Unlock()

	if needFetch {
		go func() {
			var res rpc.UIRenderResult
			err := c.callHost(hostKey, sl.PluginID, sl.Host, "ui.render", req, &res)

			c.mu.Lock()
			cur := c.renders[cacheKey]
			if cur == cr {
				cur.InFlight = false
				cur.Loading = false
				cur.FetchedAt = time.Now()
				if err != nil {
					cur.Error = err.Error()
				} else {
					cur.Error = ""
					cur.Result = &res
				}
			}
			c.mu.Unlock()
			if cb != nil {
				cb()
			}
		}()
	}
	return snapshot
}

// CachedRender returns the currently cached render snapshot for a slot without triggering a fetch.
func (c *Coordinator) CachedRender(hostKey, pluginID, slotID, sessionID, paneID string) (CachedRender, bool) {
	cacheKey := renderKey{
		hostKey:   hostKey,
		pluginID:  pluginID,
		slotID:    slotID,
		sessionID: sessionID,
		paneID:    paneID,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cr := c.renders[cacheKey]
	if cr == nil {
		return CachedRender{}, false
	}
	return *cr, true
}

// DispatchEvent sends a ui.event RPC asynchronously and updates cached slot UI on completion.
func (c *Coordinator) DispatchEvent(hostKey string, sl SlotEntry, ev rpc.UIEventParams) {
	cacheKey := renderKey{
		hostKey:   hostKey,
		pluginID:  sl.PluginID,
		slotID:    sl.Slot.ID,
		sessionID: ev.SessionID,
		paneID:    ev.PaneID,
	}
	go func() {
		var res rpc.UIEventResult
		err := c.callHost(hostKey, sl.PluginID, sl.Host, "ui.event", ev, &res)
		c.mu.Lock()
		cb := c.OnUpdate
		onMsg := c.OnMessage
		if err == nil {
			if res.Root != nil {
				cur := c.renders[cacheKey]
				if cur == nil {
					cur = &CachedRender{}
					c.renders[cacheKey] = cur
				}
				updated := rpc.UIRenderResult{Root: res.Root}
				if cur.Result != nil {
					updated = *cur.Result
					updated.Root = res.Root
				}
				cur.Result = &updated
				cur.FetchedAt = time.Now()
			}
			if res.Invalidate {
				for k, cur := range c.renders {
					if k.pluginID == sl.PluginID {
						cur.Invalid = true
					}
				}
			}
		}
		c.mu.Unlock()
		if cb != nil {
			cb()
		}
		if onMsg != nil {
			if err != nil {
				onMsg(err.Error())
			} else if res.Message != "" {
				onMsg(res.Message)
			}
		}
	}()
}

// CallAction dispatches action.invoke to the appropriate plugin host ("app" vs "daemon").
func (c *Coordinator) CallAction(hostKey string, meta manifest.PluginActionMeta, params rpc.ActionInvokeParams) (*rpc.ActionInvokeResult, error) {
	hostKind := meta.Action.Target
	if hostKind != daemonhost.HostKindDaemon && hostKind != daemonhost.HostKindApp {
		// Choose [app] if plugin defines [app], else [daemon].
		c.mu.Lock()
		snap := c.snap
		c.mu.Unlock()
		hostKind = daemonhost.HostKindDaemon
		if snap != nil {
			for _, p := range snap.Plugins {
				if p.Entry.ID == meta.PluginID && p.Manifest != nil && p.Manifest.App != nil {
					hostKind = daemonhost.HostKindApp
					break
				}
			}
		}
	}
	var res rpc.ActionInvokeResult
	if err := c.callHost(hostKey, meta.PluginID, hostKind, "action.invoke", params, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Coordinator) handleWebRPC(ctx context.Context, pluginID, slotID, sessionID, paneID string, args map[string]string, method string, params json.RawMessage) (json.RawMessage, error) {
	hostKey := ""
	if c.PaneHostFn != nil {
		hostKey = c.PaneHostFn(sessionID, paneID)
	}
	isRemote := hostKey != "" && c.RemoteRef != nil && c.RemoteRef(hostKey)
	sl, ok := c.FindSlot(pluginID, slotID)
	hostKind := daemonhost.HostKindApp
	switch {
	case isRemote && c.DaemonFn != nil:
		if dc := c.DaemonFn(hostKey); dc != nil {
			for _, st := range dc.PluginStatuses() {
				if st.ID != pluginID {
					continue
				}
				if st.Host == daemonhost.HostKindDaemon || st.Host == "" {
					hostKind = daemonhost.HostKindDaemon
				}
				for _, s := range st.Slots {
					if s.ID == slotID && s.Host != "" {
						hostKind = s.Host
						break
					}
				}
				break
			}
		}
		if hostKind == daemonhost.HostKindApp && ok && sl.Host == daemonhost.HostKindDaemon {
			hostKind = daemonhost.HostKindDaemon
		}
	case ok:
		hostKind = sl.Host
	default:
		c.mu.Lock()
		snap := c.snap
		c.mu.Unlock()
		if snap != nil {
			for _, p := range snap.Plugins {
				if p.Entry.ID == pluginID && p.Manifest != nil {
					if p.Manifest.App != nil {
						hostKind = daemonhost.HostKindApp
					} else if p.Manifest.Daemon != nil {
						hostKind = daemonhost.HostKindDaemon
					}
					break
				}
			}
		}
	}
	req := rpc.WebViewRPCParams{
		SlotID:    slotID,
		Method:    method,
		Params:    params,
		SessionID: sessionID,
		PaneID:    paneID,
		Locale:    c.currentLocale(),
		Args:      args,
	}
	var res rpc.WebViewRPCResult
	if err := c.callHostWithContext(ctx, hostKey, pluginID, hostKind, "webview.rpc", req, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Coordinator) callHost(hostKey, pluginID, hostKind, method string, params any, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return c.callHostWithContext(ctx, hostKey, pluginID, hostKind, method, params, out)
}

func (c *Coordinator) callHostWithContext(ctx context.Context, hostKey, pluginID, hostKind, method string, params any, out any) error {
	if hostKind == daemonhost.HostKindApp && c.AppMgr != nil {
		return c.AppMgr.Call(ctx, pluginID, method, params, out)
	}
	if c.DaemonFn == nil {
		return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "daemon plugin caller unavailable")
	}
	dc := c.DaemonFn(hostKey)
	if dc == nil {
		return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "host does not support plugins")
	}
	return dc.CallPlugin(pluginID, method, params, out)
}

// IconForSlot safely resolves a slot icon SVG or Bitmap from inside the plugin's root.
func (c *Coordinator) IconForSlot(pluginDir, relPath string) (*ui.SVG, *ui.Bitmap) {
	if c.State == nil {
		return nil, nil
	}
	return c.State.loadImage(pluginDir, relPath)
}

// MatchesWhen evaluates plugin slot/action visibility conditions against session & pane state.
func MatchesWhen(when string, s *proto.Session, p *proto.Pane) bool {
	expr := strings.TrimSpace(when)
	if expr == "" || expr == "always" {
		return true
	}
	if p == nil && s != nil {
		p = s.FocusPane()
	}
	clauses := strings.FieldsFunc(expr, func(r rune) bool {
		return r == ',' || r == '&'
	})
	for _, cl := range clauses {
		cl = strings.TrimSpace(cl)
		if cl == "" {
			continue
		}
		neg := false
		if strings.HasPrefix(cl, "!") {
			neg = true
			cl = strings.TrimSpace(cl[1:])
		}
		matched := matchAtom(cl, s, p)
		if neg {
			matched = !matched
		}
		if !matched {
			return false
		}
	}
	return true
}

func matchAtom(atom string, s *proto.Session, p *proto.Pane) bool {
	if strings.HasPrefix(atom, "agent:") {
		want := strings.TrimPrefix(atom, "agent:")
		return p != nil && p.Agent == want
	}
	switch atom {
	case "session.selected":
		return s != nil
	case "project.selected":
		return s != nil && s.Project != ""
	case "pane.selected":
		return p != nil
	case "pane.terminal":
		return p != nil && !p.Shown()
	case "pane.agent":
		return p != nil && p.Agent != ""
	case "pane.browser":
		return p != nil && p.Browser()
	case "pane.file":
		return p != nil && p.FilePane()
	case "pane.plugin":
		return p != nil && p.PluginPane()
	default:
		return false
	}
}
