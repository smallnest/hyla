package uihost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
	"hyla/internal/ui/theme"
)

const (
	WebScheme = "hylaplugin"
	EventName = "hyla:plugin"
)

// WebEventPayload pushes plugin events cleanly to sandboxed WebView pages via EmitPage.
type WebEventPayload struct {
	Event  string `json:"event"`
	SlotID string `json:"slotId,omitempty"`
	Data   string `json:"data,omitempty"` // JSON-encoded payload parsed by /_hyla/bridge.js
}

// PluginWebEvent is the sole package-level mygo.Event used to push events to untrusted pages.
var PluginWebEvent = mygo.NewEvent[WebEventPayload](EventName)

// WebRPCHandler dispatches authenticated POST /_hyla/rpc calls from a sandboxed plugin WebView.
type WebRPCHandler func(ctx context.Context, pluginID, slotID, sessionID, paneID string, args map[string]string, method string, params json.RawMessage) (json.RawMessage, error)

// WebEntry tracks an instantiated sandboxed WebView for a plugin pane.
type WebEntry struct {
	View      *mygo.WebView
	PluginID  string
	SlotID    string
	SessionID string
	PaneID    string
	Err       string
}

type pageBinding struct {
	PluginID  string
	SlotID    string
	SessionID string
	PaneID    string
	Args      map[string]string
	Page      *mygo.Page
}

// WebBridge serves hylaplugin://<plugin-id>/... and manages isolated plugin WebViews.
type WebBridge struct {
	mu            sync.RWMutex
	store         *manifest.Store
	themeFn       func() *theme.Palette
	localeFn      func() string
	rpcFn         WebRPCHandler
	openExtFn     func(string)
	mainFn        func(func())
	sessions      map[string]*mygo.Session
	bindings      map[*mygo.Page]pageBinding
	paneEntries   map[string]*WebEntry // keyed by paneID
	callerPageFn  func(context.Context) *mygo.Page
	emitPageFn    func(*mygo.Page, WebEventPayload) error
	togglePIPFn   func(*mygo.Page) // the tests name it; a page calls TogglePictureInPicture
	pluginCache   map[string]manifest.InstalledPlugin
	pluginsCached bool
	regExists     bool
	regModTime    time.Time
	regSize       int64
}

// NewWebBridge creates a WebBridge backed by store and dynamic host callbacks.
func NewWebBridge(store *manifest.Store, themeFn func() *theme.Palette, rpcFn WebRPCHandler) *WebBridge {
	if store == nil {
		store = manifest.DefaultStore()
	}
	if themeFn == nil {
		themeFn = func() *theme.Palette { return &theme.Light }
	}
	return &WebBridge{
		store:        store,
		themeFn:      themeFn,
		rpcFn:        rpcFn,
		sessions:     make(map[string]*mygo.Session),
		bindings:     make(map[*mygo.Page]pageBinding),
		paneEntries:  make(map[string]*WebEntry),
		callerPageFn: mygo.CallerPage,
		emitPageFn:   PluginWebEvent.EmitPage,
	}
}

// GlobalWebBridge is the process-wide handler registered for hylaplugin:// before MainWindow creation.
var (
	globalBridgeMu sync.RWMutex
	globalBridge   *WebBridge
	protoRegOnce   sync.Once
	protoRegErr    error
)

// RegisterProtocol registers the single global hylaplugin:// protocol handler with mygo.
// Safe to call before mygo.NewWindow; subsequent calls update the underlying active WebBridge.
func RegisterProtocol(b *WebBridge) error {
	globalBridgeMu.Lock()
	if b != nil {
		globalBridge = b
	}
	globalBridgeMu.Unlock()

	protoRegOnce.Do(func() {
		protoRegErr = mygo.Protocol.HandleFunc(WebScheme, func(w http.ResponseWriter, r *http.Request) {
			globalBridgeMu.RLock()
			cur := globalBridge
			globalBridgeMu.RUnlock()
			if cur == nil {
				http.Error(w, "plugin web bridge unavailable", http.StatusServiceUnavailable)
				return
			}
			cur.ServeHTTP(w, r)
		})
	})
	return protoRegErr
}

// SetCallbacks updates dynamic app callbacks (RPC dispatcher, theme resolver, external link opener, main dispatcher).
func (b *WebBridge) SetCallbacks(store *manifest.Store, themeFn func() *theme.Palette, rpcFn WebRPCHandler, openExt func(string), mainFn func(func())) {
	b.mu.Lock()
	if store != nil {
		b.store = store
		b.pluginsCached = false
		b.pluginCache = nil
	}
	if themeFn != nil {
		b.themeFn = themeFn
	}
	if rpcFn != nil {
		b.rpcFn = rpcFn
	}
	if openExt != nil {
		b.openExtFn = openExt
	}
	if mainFn != nil {
		b.mainFn = mainFn
	}
	b.mu.Unlock()
}

// SetLocaleFunc configures a callback returning the active plugin locale string.
func (b *WebBridge) SetLocaleFunc(fn func() string) {
	b.mu.Lock()
	b.localeFn = fn
	b.mu.Unlock()
}

// EnsurePaneWebView creates or returns an existing Untrusted, session-isolated WebView for pane p.
func (b *WebBridge) EnsurePaneWebView(win *mygo.Window, sessionID string, p *proto.Pane, slot manifest.SlotSpec) *WebEntry {
	b.mu.Lock()
	if existing := b.paneEntries[p.ID]; existing != nil {
		b.mu.Unlock()
		return existing
	}
	b.mu.Unlock()

	entry := &WebEntry{
		PluginID:  p.Plugin,
		SlotID:    slot.ID,
		SessionID: sessionID,
		PaneID:    p.ID,
	}
	if win == nil {
		b.mu.Lock()
		b.paneEntries[p.ID] = entry
		b.mu.Unlock()
		return entry
	}

	b.mu.RLock()
	store := b.store
	b.mu.RUnlock()
	inst, _ := b.lookupPlugin(store, p.Plugin)

	sess := b.sessionFor(p.Plugin)
	// Leave OnWillDownload and SetPermissionHandler unset so Untrusted defaults
	// deny downloads and browser permissions cleanly.
	wv, err := win.NewWebView(mygo.WebViewOptions{
		Page: mygo.PageOptions{
			Untrusted: true,
			Session:   sess,
			DevTools:  mygo.DevToolsEnabled,
			Autoplay:  webViewAutoplay(inst),
			// WKWebView keeps picture in picture to Safari, and a live TV
			// pane wants it: a small window over other apps. Allow it for a
			// plugin that the user granted network access.
			PictureInPicture: webViewAllowsPictureInPicture(inst),
		},
	})
	if err != nil {
		entry.Err = err.Error()
		b.mu.Lock()
		b.paneEntries[p.ID] = entry
		b.mu.Unlock()
		return entry
	}
	entry.View = wv

	pg := wv.Page()
	b.mu.Lock()
	b.bindings[pg] = pageBinding{
		PluginID:  p.Plugin,
		SlotID:    slot.ID,
		SessionID: sessionID,
		PaneID:    p.ID,
		Args:      maps.Clone(p.PluginArgs),
		Page:      pg,
	}
	b.paneEntries[p.ID] = entry
	openExt := b.openExtFn
	b.mu.Unlock()

	pg.OnWillNavigate(func(e *mygo.NavigateEvent) {
		u, perr := url.Parse(e.URL)
		if perr != nil || u.Scheme != WebScheme || u.Host != p.Plugin {
			e.PreventDefault()
			if e.UserInitiated && perr == nil && (u.Scheme == "http" || u.Scheme == "https") && openExt != nil {
				openExt(e.URL)
			}
		}
	})

	startFile := slot.Entry
	if startFile == "" {
		startFile = "index.html"
	}
	startFile = strings.TrimPrefix(path.Clean("/"+startFile), "/")
	targetURL := fmt.Sprintf("%s://%s/%s?slot=%s", WebScheme, p.Plugin, startFile, url.QueryEscape(slot.ID))
	b.mu.RLock()
	locFn := b.localeFn
	b.mu.RUnlock()
	if locFn != nil {
		if loc := strings.TrimSpace(locFn()); loc != "" {
			targetURL += "&locale=" + url.QueryEscape(loc)
		}
	}
	pg.LoadURL(targetURL)
	return entry
}

// ReleasePane closes and unbinds any WebView associated with paneID.
func (b *WebBridge) ReleasePane(paneID string) {
	b.mu.Lock()
	entry := b.paneEntries[paneID]
	delete(b.paneEntries, paneID)
	if entry != nil && entry.View != nil {
		delete(b.bindings, entry.View.Page())
	}
	b.mu.Unlock()
	if entry != nil && entry.View != nil {
		entry.View.Destroy()
	}
}

// ReleaseStale closes any plugin pane WebView whose pane ID is no longer in live.
func (b *WebBridge) ReleaseStale(live map[string]bool) {
	b.mu.Lock()
	var stale []*WebEntry
	for id, entry := range b.paneEntries {
		if live[id] {
			continue
		}
		delete(b.paneEntries, id)
		if entry != nil && entry.View != nil {
			delete(b.bindings, entry.View.Page())
			stale = append(stale, entry)
		}
	}
	b.mu.Unlock()
	for _, entry := range stale {
		entry.View.Destroy()
	}
}

// ReleasePlugin destroys all open WebViews belonging to pluginID and drops its isolated session.
func (b *WebBridge) ReleasePlugin(pluginID string) {
	b.mu.Lock()
	var doomed []*WebEntry
	for paneID, entry := range b.paneEntries {
		if entry != nil && entry.PluginID == pluginID {
			delete(b.paneEntries, paneID)
			if entry.View != nil {
				delete(b.bindings, entry.View.Page())
				doomed = append(doomed, entry)
			}
		}
	}
	for pg, bind := range b.bindings {
		if bind.PluginID == pluginID {
			delete(b.bindings, pg)
		}
	}
	delete(b.sessions, pluginID)
	b.mu.Unlock()
	for _, entry := range doomed {
		entry.View.Destroy()
	}
}

// ReleaseDisabledPlugins closes any WebViews whose plugin is no longer enabled for WebView use.
func (b *WebBridge) ReleaseDisabledPlugins(enabled map[string]bool) {
	b.mu.Lock()
	var ids []string
	seen := map[string]bool{}
	for _, entry := range b.paneEntries {
		if entry != nil && !enabled[entry.PluginID] && !seen[entry.PluginID] {
			seen[entry.PluginID] = true
			ids = append(ids, entry.PluginID)
		}
	}
	b.mu.Unlock()
	for _, id := range ids {
		b.ReleasePlugin(id)
	}
}

// Emit pushes a server event strictly to matching untrusted WebView pages via EmitPage.
func (b *WebBridge) Emit(pluginID, slotID, eventName string, data json.RawMessage) {
	b.mu.RLock()
	var targets []*mygo.Page
	for _, bind := range b.bindings {
		if bind.PluginID != pluginID {
			continue
		}
		if slotID != "" && bind.SlotID != slotID {
			continue
		}
		if bind.Page != nil {
			targets = append(targets, bind.Page)
		}
	}
	mainFn := b.mainFn
	emitFn := b.emitPageFn
	if emitFn == nil {
		emitFn = PluginWebEvent.EmitPage
	}
	b.mu.RUnlock()

	payload := WebEventPayload{
		Event:  eventName,
		SlotID: slotID,
		Data:   string(data),
	}
	send := func() {
		for _, pg := range targets {
			_ = emitFn(pg, payload)
		}
	}
	if len(targets) == 0 {
		return
	}
	if mainFn != nil {
		mainFn(send)
	} else {
		send()
	}
}

// BroadcastThemeChange notifies all open plugin WebViews that theme tokens changed.
func (b *WebBridge) BroadcastThemeChange(dark bool, themeName string) {
	b.broadcastAll("theme.changed", map[string]any{"dark": dark, "themeName": themeName})
}

// BroadcastLocaleChange notifies all open plugin WebViews that the app language changed.
// PictureInPicture moves a video of a plugin's open pane into picture in
// picture, a small window over every app's, or back when one is out.
func (b *WebBridge) PictureInPicture(pluginID string) error {
	b.mu.RLock()
	var targets []*mygo.Page
	for _, bind := range b.bindings {
		if bind.PluginID == pluginID && bind.Page != nil {
			targets = append(targets, bind.Page)
		}
	}
	mainFn := b.mainFn
	toggleFn := b.togglePIPFn
	b.mu.RUnlock()
	if len(targets) == 0 {
		return fmt.Errorf("plugin %q has no open pane", pluginID)
	}
	toggle := func() {
		for _, pg := range targets {
			if toggleFn != nil {
				toggleFn(pg)
				continue
			}
			pg.TogglePictureInPicture()
		}
	}
	if mainFn != nil {
		mainFn(toggle)
	} else {
		toggle()
	}
	return nil
}

func (b *WebBridge) BroadcastLocaleChange(locale string) {
	b.broadcastAll("locale.changed", map[string]any{"locale": locale})
}

func (b *WebBridge) broadcastAll(eventName string, body any) {
	b.mu.RLock()
	var targets []pageBinding
	for _, bind := range b.bindings {
		if bind.Page != nil {
			targets = append(targets, bind)
		}
	}
	mainFn := b.mainFn
	emitFn := b.emitPageFn
	if emitFn == nil {
		emitFn = PluginWebEvent.EmitPage
	}
	b.mu.RUnlock()
	if len(targets) == 0 {
		return
	}
	data, _ := json.Marshal(body)
	send := func() {
		for _, bind := range targets {
			_ = emitFn(bind.Page, WebEventPayload{
				Event:  eventName,
				SlotID: bind.SlotID,
				Data:   string(data),
			})
		}
	}
	if mainFn != nil {
		mainFn(send)
	} else {
		send()
	}
}

func (b *WebBridge) sessionFor(pluginID string) *mygo.Session {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.sessions[pluginID]; ok {
		return s
	}
	s := mygo.NewSession("plugin:" + pluginID)
	b.sessions[pluginID] = s
	return s
}

// SetPluginsSnapshot replaces the cached plugin map from an already-discovered
// manifest list so ServeHTTP asset requests avoid repeated Store.Discover scans.
func (b *WebBridge) SetPluginsSnapshot(plugins []manifest.InstalledPlugin) {
	m := make(map[string]manifest.InstalledPlugin, len(plugins))
	for _, p := range plugins {
		m[p.Entry.ID] = p
	}
	b.mu.Lock()
	store := b.store
	var regExists bool
	var modTime time.Time
	var regSize int64
	if store != nil {
		if fi, err := os.Stat(store.RegistryPath()); err == nil {
			regExists = true
			modTime = fi.ModTime()
			regSize = fi.Size()
		}
	}
	b.pluginCache = m
	b.pluginsCached = true
	b.regExists = regExists
	b.regModTime = modTime
	b.regSize = regSize
	b.mu.Unlock()
}

func (b *WebBridge) lookupPlugin(store *manifest.Store, pluginID string) (*manifest.InstalledPlugin, error) {
	if store == nil {
		store = manifest.DefaultStore()
	}
	var regExists bool
	var modTime time.Time
	var regSize int64
	if fi, err := os.Stat(store.RegistryPath()); err == nil {
		regExists = true
		modTime = fi.ModTime()
		regSize = fi.Size()
	}

	b.mu.RLock()
	cacheValid := b.pluginsCached && b.store == store && b.regExists == regExists &&
		(!regExists || (b.regModTime.Equal(modTime) && b.regSize == regSize))
	if cacheValid {
		inst, ok := b.pluginCache[pluginID]
		b.mu.RUnlock()
		if !ok {
			return nil, nil
		}
		return &inst, nil
	}
	b.mu.RUnlock()

	plugins, err := store.Discover()
	if err != nil {
		return nil, err
	}
	b.SetPluginsSnapshot(plugins)
	for i := range plugins {
		if plugins[i].Entry.ID == pluginID {
			cp := plugins[i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (b *WebBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pluginID := strings.ToLower(strings.TrimSpace(r.URL.Host))
	if err := manifest.ValidateID(pluginID); err != nil {
		http.Error(w, "invalid plugin id", http.StatusBadRequest)
		return
	}

	b.mu.RLock()
	store := b.store
	themeFn := b.themeFn
	rpcFn := b.rpcFn
	callerFn := b.callerPageFn
	b.mu.RUnlock()

	inst, err := b.lookupPlugin(store, pluginID)
	if err != nil {
		http.Error(w, "plugin registry unavailable", http.StatusInternalServerError)
		return
	}
	if inst == nil || !inst.Entry.Enabled || inst.Manifest == nil || !inst.Entry.HasPermission(manifest.PermUIWebView) {
		http.Error(w, "plugin webview forbidden or disabled", http.StatusForbidden)
		return
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", webViewCSP(inst))

	cleanPath := path.Clean("/" + r.URL.Path)
	switch cleanPath {
	case "/_hyla/theme.css":
		pal := themeFn()
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = io.WriteString(w, ThemeCSS(pal))
		return

	case "/_hyla/bridge.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, bridgeJS)
		return

	case "/_hyla/rpc":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		pg := callerFn(r.Context())
		if pg == nil {
			http.Error(w, "missing caller page", http.StatusForbidden)
			return
		}
		b.mu.RLock()
		bind, ok := b.bindings[pg]
		b.mu.RUnlock()
		if !ok || bind.PluginID != pluginID {
			http.Error(w, "caller page mismatch", http.StatusForbidden)
			return
		}
		if rpcFn == nil {
			http.Error(w, "rpc handler unavailable", http.StatusServiceUnavailable)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
		if err != nil {
			http.Error(w, "failed reading body", http.StatusBadRequest)
			return
		}
		if len(body) > 1<<20 {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		var req struct {
			SlotID string          `json:"slotId"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Method) == "" {
			http.Error(w, "invalid rpc json", http.StatusBadRequest)
			return
		}
		slotID := bind.SlotID
		if req.SlotID != "" {
			slotID = req.SlotID
		}
		data, err := rpcFn(r.Context(), pluginID, slotID, bind.SessionID, bind.PaneID, maps.Clone(bind.Args), req.Method, req.Params)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err != nil {
			code := rpc.ErrCodeInternal
			if re, ok := err.(*rpc.Error); ok {
				code = re.Code
			}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": code, "message": err.Error()},
			})
			return
		}
		if len(data) == 0 {
			data = json.RawMessage("null")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": data})
		return
	}

	// Serve static plugin assets within slot.WebRoot
	slotID := r.URL.Query().Get("slot")
	if pg := callerFn(r.Context()); pg != nil {
		b.mu.RLock()
		if bind, ok := b.bindings[pg]; ok && bind.PluginID == pluginID && slotID == "" {
			slotID = bind.SlotID
		}
		b.mu.RUnlock()
	}
	slot, ok := findWebViewSlot(inst.Manifest, slotID)
	if !ok {
		http.Error(w, "webview slot not declared", http.StatusNotFound)
		return
	}
	webRootRel := slot.WebRoot
	if webRootRel == "" {
		webRootRel = "web"
	}
	webRootAbs, err := manifest.ResolveWithin(inst.RootDir, webRootRel)
	if err != nil {
		http.Error(w, "invalid web root", http.StatusForbidden)
		return
	}

	rel := strings.TrimPrefix(cleanPath, "/")
	if rel == "" {
		if slot.Entry != "" {
			rel = slot.Entry
		} else {
			rel = "index.html"
		}
	}
	targetFile, err := manifest.ResolveWithin(webRootAbs, rel)
	if err != nil {
		http.Error(w, "invalid asset path", http.StatusForbidden)
		return
	}
	info, err := os.Stat(targetFile)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(targetFile)
	if err != nil {
		http.Error(w, "failed reading asset", http.StatusInternalServerError)
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(targetFile))
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", ct)
	_, _ = w.Write(data)
}

func webViewCSP(inst *manifest.InstalledPlugin) string {
	if inst != nil && inst.Entry.HasPermission(manifest.PermNetwork) {
		return "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' http://127.0.0.1:* http://localhost:* https: data: blob:; media-src 'self' http://127.0.0.1:* http://localhost:* https: data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'none'; object-src 'none'"
	}
	return "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'none'; object-src 'none'"
}

func webViewAutoplay(inst *manifest.InstalledPlugin) mygo.Autoplay {
	if inst != nil && inst.Entry.HasPermission(manifest.PermNetwork) {
		return mygo.AutoplayAllow
	}
	return mygo.AutoplayUserGesture
}

// webViewAllowsPictureInPicture allows picture in picture for a plugin
// with network access, which plays media. A plugin without it has no
// media to move.
func webViewAllowsPictureInPicture(inst *manifest.InstalledPlugin) bool {
	return inst != nil && inst.Entry.HasPermission(manifest.PermNetwork)
}

func findWebViewSlot(m *manifest.Manifest, slotID string) (manifest.SlotSpec, bool) {
	if m == nil {
		return manifest.SlotSpec{}, false
	}
	for _, s := range m.UI.Slots {
		if s.Mode != "webview" {
			continue
		}
		if slotID == "" || s.ID == slotID {
			return s, true
		}
	}
	return manifest.SlotSpec{}, false
}

// ThemeCSS renders CSS custom properties matching Hyla's active Palette.
func ThemeCSS(pal *theme.Palette) string {
	if pal == nil {
		pal = &theme.Light
	}
	scheme := "light"
	if pal.Dark {
		scheme = "dark"
	}
	cssCol := func(c ui.Color) string {
		return fmt.Sprintf("rgba(%d, %d, %d, %.3f)", c.R, c.G, c.B, float64(c.A)/255)
	}
	return fmt.Sprintf(`:root {
  color-scheme: %s;
  --hyla-bg: %s;
  --hyla-sidebar: %s;
  --hyla-card: %s;
  --hyla-chip: %s;
  --hyla-popover: %s;
  --hyla-control: %s;
  --hyla-text: %s;
  --hyla-muted: %s;
  --hyla-text-muted: %s;
  --hyla-faint: %s;
  --hyla-accent: %s;
  --hyla-attention: %s;
  --hyla-done: %s;
  --hyla-danger: %s;
  --hyla-divider: %s;
  --hyla-tl-think: %s;
  --hyla-tl-text: %s;
  --hyla-tl-tool: %s;
  --hyla-tl-sub: %s;
  --hyla-tl-wait: %s;
  --hyla-tl-fail: %s;
  --hyla-font-ui: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  --hyla-font-mono: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
html, body {
  margin: 0;
  padding: 0;
  background: transparent;
  color: var(--hyla-text);
  font-family: var(--hyla-font-ui);
}
`, scheme,
		cssCol(pal.Card),
		cssCol(pal.Sidebar),
		cssCol(pal.Card),
		cssCol(pal.Chip),
		cssCol(pal.Popover),
		cssCol(pal.Control),
		cssCol(pal.Text),
		cssCol(pal.TextMuted),
		cssCol(pal.TextMuted),
		cssCol(pal.TextFaint),
		cssCol(pal.Accent),
		cssCol(pal.Attention),
		cssCol(pal.Done),
		cssCol(pal.Danger),
		cssCol(pal.Divider),
		cssCol(pal.TlThink),
		cssCol(pal.TlText),
		cssCol(pal.TlTool),
		cssCol(pal.TlSub),
		cssCol(pal.TlWait),
		cssCol(pal.TlFail),
	)
}

const bridgeJS = `(() => {
  const listeners = new Map();
  function dispatch(ev, data) {
    const set = listeners.get(ev);
    if (!set) return;
    for (const fn of set) {
      try { fn(data); } catch (e) { console.error(e); }
    }
  }
  if (window.mygo && typeof window.mygo.on === "function") {
    window.mygo.on("hyla:plugin", (payload) => {
      if (!payload || !payload.event) return;
      let parsed = null;
      if (payload.data) {
        try { parsed = JSON.parse(payload.data); } catch { parsed = payload.data; }
      }
      if (payload.event === "theme.changed") {
        const link = document.querySelector('link[href*="/_hyla/theme.css"]');
        if (link) {
          link.href = "/_hyla/theme.css?t=" + Date.now();
        }
      }
      dispatch(payload.event, parsed);
    });
  }
  window.hyla = {
    async rpc(method, params, slotId) {
      const resp = await fetch("/_hyla/rpc", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ method, params: params ?? {}, slotId: slotId ?? "" })
      });
      const json = await resp.json();
      if (!resp.ok || json.error) {
        throw new Error(json?.error?.message || ("RPC failed: " + resp.status));
      }
      return json.result;
    },
    on(event, fn) {
      let set = listeners.get(event);
      if (!set) { set = new Set(); listeners.set(event, set); }
      set.add(fn);
      return () => set.delete(fn);
    }
  };
})();
`
