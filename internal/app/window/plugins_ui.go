package window

import (
	"context"
	"maps"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/app/host"
	"hyla/internal/build"
	"hyla/internal/cli"
	"hyla/internal/plugins/daemonhost"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/plugins/uihost"
	"hyla/internal/proto"
	"hyla/internal/ui/files"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/info"
	uisettings "hyla/internal/ui/settings"
	"hyla/internal/ui/settings/kit"
	"hyla/internal/ui/settings/search"
	"hyla/internal/ui/side"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// infoPluginCards renders enabled info.card slots contributed by plugins.
func (a *App) infoPluginCards(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	if a.Plugins == nil {
		return
	}
	hasMagpieFixture := a.hasMagpieFixture()
	h := a.pluginHostForSession(s)
	hostKey := a.pluginHostKey(h)

	for _, sl := range a.effectivePluginSlots(h, "info.card", s, p) {
		if sl.PluginID == "hyla.magpie" && hasMagpieFixture {
			continue
		}
		if sl.Host == "daemon" && (a.Plugins.DaemonFn == nil || a.Plugins.DaemonFn(hostKey) == nil) {
			continue
		}
		var sessID, paneID, projID string
		if s != nil {
			sessID, projID = s.ID, s.Project
		}
		if p != nil {
			paneID = p.ID
		}
		req := rpc.UIRenderParams{
			SlotID:    sl.Slot.ID,
			Mount:     "info.card",
			SessionID: sessID,
			PaneID:    paneID,
			ProjectID: projID,
			Dark:      pal.Dark,
			Locale:    a.pluginLocaleValue(),
		}
		cr := a.Plugins.RenderSlot(hostKey, sl, req)
		// Skip slots that explicitly returned no UI tree (e.g., unconfigured optional plugin).
		if !cr.Loading && cr.Error == "" && cr.Result != nil && cr.Result.Root == nil {
			continue
		}
		title := sl.Slot.Title
		aside := ""
		if cr.Result != nil {
			if cr.Result.Title != "" {
				title = cr.Result.Title
			}
			if cr.Result.Badge != "" {
				aside = cr.Result.Badge
			} else if cr.Result.Subtitle != "" {
				aside = cr.Result.Subtitle
			}
			// Unwrap outer infoCard root so info.Card isn't double-nested.
			if cr.Result.Root != nil && cr.Result.Root.Type == "infoCard" {
				if title == "" {
					if t, ok := cr.Result.Root.Props["title"].(string); ok {
						title = t
					}
				}
				if aside == "" {
					if as, ok := cr.Result.Root.Props["aside"].(string); ok {
						aside = as
					}
				}
				unwrapped := *cr.Result
				unwrapped.Root = &rpc.UINode{
					Type:     "column",
					Key:      cr.Result.Root.Key,
					Style:    cr.Result.Root.Style,
					Children: cr.Result.Root.Children,
				}
				cr.Result = &unwrapped
			}
		}
		svg, _ := a.Plugins.IconForSlot(sl.PluginDir, sl.Slot.Icon)
		if svg == nil {
			svg = glyph.Plugins
		}
		foldKey := "plugin:" + sl.PluginID + ":" + sl.Slot.ID
		slotCopy := sl
		info.Card(c, a, pal, foldKey, svg, title, aside, func() {
			ui.Column(c.Key("plg-card-body-"+foldKey)).Padding(8, 12, 12, 12).Gap(6).AlignItems(ui.Stretch).Children(func() {
				a.renderPluginSlot(c, pal, hostKey, slotCopy, sessID, paneID, cr, false)
			})
		})
	}
}

// PluginTabs returns custom side-panel tabs registered by enabled plugins.
func (a *App) PluginTabs(s *proto.Session) []side.PluginTab {
	if a.Plugins == nil {
		return nil
	}
	var p *proto.Pane
	if s != nil {
		p = s.FocusPane()
	}
	h := a.pluginHostForSession(s)
	hostKey := a.pluginHostKey(h)
	var tabs []side.PluginTab
	for _, sl := range a.effectivePluginSlots(h, "side.tab", s, p) {
		if sl.Host == "daemon" && (a.Plugins.DaemonFn == nil || a.Plugins.DaemonFn(hostKey) == nil) {
			continue
		}
		svg, _ := a.Plugins.IconForSlot(sl.PluginDir, sl.Slot.Icon)
		tabs = append(tabs, side.PluginTab{
			ID:    "plugin:" + sl.PluginID + ":" + sl.Slot.ID,
			Title: sl.Slot.Title,
			Icon:  svg,
		})
	}
	return tabs
}

// PluginSide renders the body of a custom plugin side-panel tab.
func (a *App) PluginSide(c *ui.Context, pal *theme.Palette, s *proto.Session, tabID string, width float32) {
	if a.Plugins == nil {
		a.ChangesSide(c, pal, s, width)
		return
	}
	pluginID, slotID, ok := parsePluginTabID(tabID)
	if !ok {
		a.ChangesSide(c, pal, s, width)
		return
	}
	h := a.pluginHostForSession(s)
	hostKey := a.pluginHostKey(h)
	sl, found := a.findEffectiveSlot(h, pluginID, slotID)
	if !found || sl.Slot.Mount != "side.tab" {
		a.ChangesSide(c, pal, s, width)
		return
	}
	if sl.Host == "daemon" && (a.Plugins.DaemonFn == nil || a.Plugins.DaemonFn(hostKey) == nil) {
		a.SideMessage(c, pal, locales.PluginUnavailableTitle(), locales.PluginUnavailableDetail(pluginID))
		return
	}
	var sessID, paneID, projID string
	if s != nil {
		sessID, projID = s.ID, s.Project
		paneID = a.focusOf(s)
	}
	req := rpc.UIRenderParams{
		SlotID:    sl.Slot.ID,
		Mount:     "side.tab",
		SessionID: sessID,
		PaneID:    paneID,
		ProjectID: projID,
		Dark:      pal.Dark,
		Locale:    a.pluginLocaleValue(),
		Width:     width,
	}
	cr := a.Plugins.RenderSlot(hostKey, sl, req)
	ui.Scroll(c.Key("plg-side-" + tabID)).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c.Key("plg-side-col-"+tabID)).Padding(8, 12, 12, 12).Gap(8).AlignItems(ui.Stretch).Children(func() {
			a.renderPluginSlot(c, pal, hostKey, sl, sessID, paneID, cr, false)
		})
	})
}

func parsePluginTabID(tabID string) (pluginID, slotID string, ok bool) {
	rest, has := strings.CutPrefix(tabID, "plugin:")
	if !has {
		return "", "", false
	}
	pid, sid, sep := strings.Cut(rest, ":")
	if !sep || pid == "" || sid == "" {
		return "", "", false
	}
	return pid, sid, true
}

// PluginPaneBody renders custom declarative or sandboxed WebView panes, or a
// graceful fallback card when the plugin is missing, disabled, or unreachable.
func (a *App) PluginPaneBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	if p == nil {
		return
	}
	closeFallback := func() {
		a.PaneMessage(c, pal, locales.PluginUnavailableTitle(), locales.PluginUnavailableDetail(p.Plugin), locales.PluginClosePane(), func() {
			if s != nil {
				sid, pid := s.ID, p.ID
				a.RunLater(func() {
					if cur := a.session(sid); cur != nil {
						a.closePane(cur, pid, false)
					}
				})
			}
		})
	}
	h := a.pluginHostForSession(s)
	hostKey := a.pluginHostKey(h)
	if a.Plugins == nil || !a.isEffectivePluginEnabled(h, p.Plugin) {
		closeFallback()
		return
	}
	sl, found := a.findEffectiveSlot(h, p.Plugin, p.PluginView)
	if !found {
		closeFallback()
		return
	}
	// Remote hosts require local plugin static assets for webview panes and daemon support.
	if h.Remote() && sl.Slot.Mode == "webview" && (!a.hostSupportsPlugins(h) || sl.PluginDir == "") {
		closeFallback()
		return
	}
	if sl.Host == "daemon" && (a.Plugins.DaemonFn == nil || a.Plugins.DaemonFn(hostKey) == nil) {
		closeFallback()
		return
	}

	var sessID, projID string
	if s != nil {
		sessID, projID = s.ID, s.Project
	}

	if sl.Slot.Mode == "webview" {
		entry := a.Plugins.Web.EnsurePaneWebView(a.Win, sessID, p, sl.Slot)
		if entry.Err != "" {
			a.PaneMessage(c, pal, locales.PluginUnavailableTitle(), entry.Err, locales.PluginClosePane(), func() {
				if s != nil {
					sid, pid := s.ID, p.ID
					a.RunLater(func() {
						if cur := a.session(sid); cur != nil {
							a.closePane(cur, pid, false)
						}
					})
				}
			})
			return
		}
		if entry.View == nil {
			// Headless tester environment without native MainWindow.
			ui.Column(c.Key("plg-web-headless-" + p.ID)).Fill().Center().Padding(20).Children(func() {
				ui.Text(c, p.PaneName()).FontSize(13).TextColor(pal.TextMuted)
			})
			return
		}
		ui.WebView(c.Key("plg-web-"+p.ID), entry.View).Grow(1).Bind(a.paneRef(p.ID))
		return
	}

	req := rpc.UIRenderParams{
		SlotID:    sl.Slot.ID,
		Mount:     "pane",
		SessionID: sessID,
		PaneID:    p.ID,
		ProjectID: projID,
		Dark:      pal.Dark,
		Locale:    a.pluginLocaleValue(),
		Args:      p.PluginArgs,
	}
	cr := a.Plugins.RenderSlot(hostKey, sl, req)
	ui.Scroll(c.Key("plg-pane-" + p.ID)).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c.Key("plg-pane-col-"+p.ID)).Padding(12, 16, 16, 16).Gap(8).AlignItems(ui.Stretch).Children(func() {
			a.renderPluginSlot(c, pal, hostKey, sl, sessID, p.ID, cr, false)
		})
	})
}

func (a *App) renderPluginSlot(c *ui.Context, pal *theme.Palette, hostKey string, sl uihost.SlotEntry, sessID, paneID string, cr uihost.CachedRender, settingsSearch bool) {
	if cr.Loading {
		ui.Row(c.Key("loading")).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.PluginLoading()).FontSize(12).TextColor(pal.TextMuted)
		})
		return
	}
	if cr.Error != "" {
		ui.Text(c.Key("err"), cr.Error).FontSize(12).TextColor(pal.Danger).Wrap()
		return
	}
	if cr.Result == nil || cr.Result.Root == nil {
		return
	}
	if !cr.InFlight && cr.Result.RefreshMS > 0 {
		due := time.Duration(cr.Result.RefreshMS)*time.Millisecond - time.Since(cr.FetchedAt)
		if due < 50*time.Millisecond {
			due = 50 * time.Millisecond
		}
		c.After(due)
	}
	uihost.Render(c, uihost.RenderContext{
		PluginID:       sl.PluginID,
		SlotID:         sl.Slot.ID,
		PluginDir:      sl.PluginDir,
		Palette:        pal,
		State:          a.Plugins.State,
		SettingsSearch: settingsSearch,
		OnEvent: func(componentID, eventType string, value any) {
			a.Plugins.DispatchEvent(hostKey, sl, rpc.UIEventParams{
				SlotID:      sl.Slot.ID,
				ComponentID: componentID,
				EventType:   eventType,
				Value:       value,
				SessionID:   sessID,
				PaneID:      paneID,
				Locale:      a.pluginLocaleValue(),
			})
		},
	}, cr.Result.Root)
}

// PaneContextMenu builds the context menu on pane headers, including plugin pane.menu items.
func (a *App) PaneContextMenu(m *ui.Menu, s *proto.Session, p *proto.Pane) {
	if s == nil || p == nil {
		return
	}
	sid, pid := s.ID, p.ID
	if a.menuShortcut(m.Item(locales.ActionSplitRight()), "pane.split.right").Chosen() {
		a.RunLater(func() {
			if cur := a.session(sid); cur != nil {
				a.splitPane(cur, pid, "right")
			}
		})
	}
	if a.menuShortcut(m.Item(locales.ActionSplitDown()), "pane.split.down").Chosen() {
		a.RunLater(func() {
			if cur := a.session(sid); cur != nil {
				a.splitPane(cur, pid, "down")
			}
		})
	}
	if len(s.Panes) > 1 {
		if a.menuShortcut(m.Item(locales.ActionZoomPane()), "pane.zoom").Chosen() {
			a.RunLater(func() {
				if cur := a.session(sid); cur != nil {
					a.zoomPane(cur, pid)
				}
			})
		}
	}
	if !p.Shown() {
		if a.menuShortcut(m.Item(locales.ActionRestartPane()), "pane.restart").Chosen() {
			a.RunLater(func() { a.RestartSession(pid) })
		}
	}
	a.appendPluginMountMenuItems(m, "pane.menu", s, p, nil)
	m.Separator()
	if a.menuShortcut(m.Item(locales.ActionClose()), "pane.close").Chosen() {
		a.RunLater(func() {
			if cur := a.session(sid); cur != nil {
				a.closePane(cur, pid, true)
			}
		})
	}
}

// PluginSessionMenu injects enabled sidebar.menu plugin actions into sidebar session menus.
func (a *App) PluginSessionMenu(m *ui.Menu, s *proto.Session) {
	var p *proto.Pane
	if s != nil {
		p = s.FocusPane()
	}
	a.appendPluginMountMenuItems(m, "sidebar.menu", s, p, nil)
}

// PluginFileMenu injects enabled files.menu plugin actions into Files context menus.
func (a *App) PluginFileMenu(m *ui.Menu, s *proto.Session, pane string, it files.Item) {
	var p *proto.Pane
	if s != nil {
		p = s.PaneByID(pane)
	}
	args := map[string]string{
		"path": it.Path,
		"rel":  it.Rel,
		"dir":  strconv.FormatBool(it.Dir),
	}
	a.appendPluginMountMenuItems(m, "files.menu", s, p, args)
}

func (a *App) appendPluginMountMenuItems(m *ui.Menu, mount string, s *proto.Session, p *proto.Pane, args map[string]string) bool {
	if a.Plugins == nil {
		return false
	}
	h := a.pluginHostForSession(s)
	slots := a.effectivePluginSlots(h, mount, s, p)
	if len(slots) == 0 {
		return false
	}
	var sessID, paneID, projID string
	if s != nil {
		sessID, projID = s.ID, s.Project
	}
	if p != nil {
		paneID = p.ID
	}
	hostKey := a.pluginHostKey(h)
	m.Separator()
	for _, sl := range slots {
		slot := sl
		title := slot.Slot.Title
		if title == "" {
			title = slot.Slot.ID
		}
		if m.Item(title).Chosen() {
			actionID := slot.Slot.Action
			if actionID == "" {
				actionID = slot.Slot.ID
			}
			fullID := manifest.QualifyActionID(slot.PluginID, actionID)
			a.RunLater(func() {
				if _, ok := actions.Find(fullID); ok {
					a.runPluginAction(fullID, args)
					return
				}
				if slot.Slot.Mode == "declarative" || slot.Slot.Mode == "webview" {
					cur := s
					if sessID != "" {
						cur = a.session(sessID)
					}
					a.openPluginPane(cur, slot.PluginID, slot.Slot.ID, title, "", args)
					return
				}
				// Dispatch direct action.invoke to plugin host.
				var actSpec manifest.ActionSpec
				if snap := a.Plugins.Snapshot(); snap != nil {
					if mSpec, ok := snap.ActionTargets[fullID]; ok {
						actSpec = mSpec.Action
					}
				}
				if actSpec.ID == "" {
					actSpec = manifest.ActionSpec{
						ID:     actionID,
						Title:  title,
						Target: slot.Host,
					}
				}
				meta := manifest.PluginActionMeta{
					PluginID: slot.PluginID,
					FullID:   fullID,
					Action:   actSpec,
				}
				go func() {
					res, err := a.Plugins.CallAction(hostKey, meta, rpc.ActionInvokeParams{
						ActionID:  actionID,
						Args:      args,
						SessionID: sessID,
						PaneID:    paneID,
						ProjectID: projID,
						Host:      hostKey,
					})
					a.main(func() {
						if err != nil {
							a.toast(err.Error())
						} else if res != nil && res.Message != "" {
							a.toast(res.Message)
						}
						if a.Win != nil {
							a.Win.Invalidate()
						}
					})
				}()
			})
		}
	}
	return true
}

func (a *App) pluginHostForSession(s *proto.Session) *host.Host {
	if s != nil {
		return a.hostOf(s.ID)
	}
	return a.Local()
}

func (a *App) effectivePluginSlots(h *host.Host, mount string, s *proto.Session, p *proto.Pane) []uihost.SlotEntry {
	if a.Plugins == nil {
		return nil
	}
	if h == nil || !h.Remote() {
		return a.Plugins.Slots(mount, s, p)
	}
	var out []uihost.SlotEntry
	seen := map[string]bool{}
	for _, sl := range a.Plugins.Slots(mount, s, p) {
		if sl.Host == "app" {
			key := sl.PluginID + ":" + sl.Slot.ID
			seen[key] = true
			out = append(out, sl)
		}
	}
	if a.hostSupportsPlugins(h) {
		hostKey := a.pluginHostKey(h)
		for _, st := range a.daemonPluginStatuses(hostKey) {
			if !st.Enabled || st.PendingInstall {
				continue
			}
			hasSlot := slices.Contains(st.GrantedPermissions, manifest.PermUISlot)
			hasWeb := slices.Contains(st.GrantedPermissions, manifest.PermUIWebView)
			localDir := ""
			if localSl, ok := a.Plugins.FindSlot(st.ID, ""); ok {
				localDir = localSl.PluginDir
			} else if snap := a.Plugins.Snapshot(); snap != nil {
				for _, ip := range snap.Plugins {
					if ip.Entry.ID == st.ID {
						localDir = ip.RootDir
						break
					}
				}
			}
			for _, spec := range st.Slots {
				if spec.Mount != mount {
					continue
				}
				if spec.Mode == "webview" && !hasWeb {
					continue
				}
				if spec.Mode != "webview" && !hasSlot {
					continue
				}
				slHost := spec.Host
				if slHost == "" {
					slHost = "daemon"
				}
				if slHost == "app" {
					continue
				}
				if !uihost.MatchesWhen(spec.When, s, p) {
					continue
				}
				key := st.ID + ":" + spec.ID
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, uihost.SlotEntry{
					PluginID:  st.ID,
					PluginDir: localDir,
					Slot:      spec,
					Host:      slHost,
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Slot.Order != out[j].Slot.Order {
			return out[i].Slot.Order < out[j].Slot.Order
		}
		if out[i].PluginID != out[j].PluginID {
			return out[i].PluginID < out[j].PluginID
		}
		return out[i].Slot.ID < out[j].Slot.ID
	})
	return out
}

func (a *App) findEffectiveSlot(h *host.Host, pluginID, slotID string) (uihost.SlotEntry, bool) {
	if a.Plugins == nil {
		return uihost.SlotEntry{}, false
	}
	if h == nil || !h.Remote() {
		return a.Plugins.FindSlot(pluginID, slotID)
	}
	if localSl, ok := a.Plugins.FindSlot(pluginID, slotID); ok && localSl.Host == "app" {
		return localSl, true
	}
	if a.hostSupportsPlugins(h) {
		hostKey := a.pluginHostKey(h)
		localDir := ""
		if snap := a.Plugins.Snapshot(); snap != nil {
			for _, ip := range snap.Plugins {
				if ip.Entry.ID == pluginID {
					localDir = ip.RootDir
					break
				}
			}
		}
		for _, st := range a.daemonPluginStatuses(hostKey) {
			if st.ID != pluginID || !st.Enabled || st.PendingInstall {
				continue
			}
			for _, spec := range st.Slots {
				if spec.ID == slotID || slotID == "" {
					slHost := spec.Host
					if slHost == "" {
						slHost = "daemon"
					}
					return uihost.SlotEntry{
						PluginID:  st.ID,
						PluginDir: localDir,
						Slot:      spec,
						Host:      slHost,
					}, true
				}
			}
		}
	}
	return uihost.SlotEntry{}, false
}

func (a *App) isEffectivePluginEnabled(h *host.Host, pluginID string) bool {
	if a.Plugins == nil || pluginID == "" {
		return false
	}
	if h == nil || !h.Remote() {
		return a.Plugins.IsPluginEnabled(pluginID)
	}
	if a.hostSupportsPlugins(h) {
		for _, st := range a.daemonPluginStatuses(a.pluginHostKey(h)) {
			if st.ID == pluginID && st.Enabled && !st.PendingInstall {
				return true
			}
		}
	}
	return a.Plugins.IsPluginEnabled(pluginID)
}

type cachedPluginConfig struct {
	modTime time.Time
	size    int64
	exists  bool
	cfg     map[string]any
}

type cachedPluginSecret struct {
	has bool
	at  time.Time
}

type pluginUICache struct {
	mu          sync.Mutex
	configs     map[string]cachedPluginConfig
	secrets     map[string]cachedPluginSecret
	appStatuses []rpc.PluginStatus
	appStatusAt time.Time
}

func (a *App) invalidatePluginStatusCache() {
	a.pluginCache.mu.Lock()
	a.pluginCache.appStatuses = nil
	a.pluginCache.appStatusAt = time.Time{}
	a.pluginCache.mu.Unlock()
}

func (a *App) invalidatePluginUICache() {
	a.pluginCache.mu.Lock()
	clear(a.pluginCache.configs)
	clear(a.pluginCache.secrets)
	a.pluginCache.appStatuses = nil
	a.pluginCache.appStatusAt = time.Time{}
	a.pluginCache.mu.Unlock()
}

func (a *App) invalidatePluginUICacheFor(id string) {
	if id == "" {
		a.invalidatePluginUICache()
		return
	}
	a.pluginCache.mu.Lock()
	delete(a.pluginCache.configs, id)
	prefix := id + "\x00"
	for k := range a.pluginCache.secrets {
		if strings.HasPrefix(k, prefix) {
			delete(a.pluginCache.secrets, k)
		}
	}
	a.pluginCache.mu.Unlock()
}

// appPluginsSettings adapts *App and uihost.Coordinator to Settings › Plugins.
type appPluginsSettings struct {
	a *App
}

func (p *appPluginsSettings) store() *manifest.Store {
	if p.a.Plugins != nil && p.a.Plugins.Store != nil {
		return p.a.Plugins.Store
	}
	return manifest.DefaultStore()
}

func (p *appPluginsSettings) List() []manifest.InstalledPlugin {
	if p.a.Plugins != nil {
		if snap := p.a.Plugins.Snapshot(); snap != nil {
			return snap.Plugins
		}
	}
	list, _ := p.store().Discover()
	return list
}

func (p *appPluginsSettings) DaemonStatuses() []rpc.PluginStatus {
	return p.a.daemonPluginStatuses("local")
}

func (p *appPluginsSettings) AppStatuses() []rpc.PluginStatus {
	if p.a.Plugins == nil || p.a.Plugins.AppMgr == nil {
		return nil
	}
	p.a.pluginCache.mu.Lock()
	if !p.a.pluginCache.appStatusAt.IsZero() && time.Since(p.a.pluginCache.appStatusAt) < time.Second {
		out := slices.Clone(p.a.pluginCache.appStatuses)
		p.a.pluginCache.mu.Unlock()
		return out
	}
	p.a.pluginCache.mu.Unlock()

	list := p.a.Plugins.AppMgr.ListStatuses()
	p.a.pluginCache.mu.Lock()
	p.a.pluginCache.appStatuses = slices.Clone(list)
	p.a.pluginCache.appStatusAt = time.Now()
	p.a.pluginCache.mu.Unlock()
	return list
}

func (p *appPluginsSettings) ReloadAll() {
	if err := p.a.reloadPlugins(true); err != nil {
		p.a.toast(err.Error())
	}
}

func (p *appPluginsSettings) InstallPackage() {
	win := p.a.SetWin
	st := p.store()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      win,
			Title:       locales.SetPluginsInstallDialog(),
			ButtonLabel: locales.CommonOpen(),
			Filters:     []mygo.FileFilter{{Name: "Hyla Plugin", Extensions: []string{"hylapkg"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		_, ierr := st.InstallPackage(paths[0], manifest.InstallOptions{
			Enable:         true,
			GrantAll:       false,
			CurrentVersion: build.Version(),
		})
		if ierr == nil {
			_ = cli.SyncPluginAgentIntegrations(st, nil)
		}
		p.a.main(func() {
			if ierr != nil {
				p.a.toast(ierr.Error())
				if p.a.SetWin != nil {
					p.a.SetWin.Invalidate()
				}
				return
			}
			p.a.State.Soon()
			_ = p.a.reloadPlugins(true)
		})
	}()
}

func (p *appPluginsSettings) SetEnabled(id string, on bool) {
	st := p.store()
	if err := st.Enable(id, on); err != nil {
		p.a.toast(err.Error())
		return
	}
	_ = cli.SyncPluginAgentIntegrations(st, nil)
	p.a.State.Soon()
	_ = p.a.reloadPlugins(true)
}

func (p *appPluginsSettings) SetPermission(id, scope string, granted bool) {
	st := p.store()
	if err := st.SetPermission(id, scope, granted); err != nil {
		p.a.toast(err.Error())
		return
	}
	_ = cli.SyncPluginAgentIntegrations(st, nil)
	p.a.State.Soon()
	_ = p.a.reloadPlugins(true)
}

func (p *appPluginsSettings) Restart(id string) {
	var (
		enabled   bool
		hasApp    bool
		hasDaemon bool
	)
	for _, plg := range p.List() {
		if plg.Entry.ID == id {
			enabled = plg.Entry.Enabled && !plg.Entry.PendingInstall
			hasApp = plg.Manifest != nil && plg.Manifest.App != nil
			hasDaemon = plg.Manifest != nil && plg.Manifest.Daemon != nil
			break
		}
	}
	if !enabled {
		return
	}
	p.a.invalidatePluginUICache()
	if hasApp && p.a.Plugins != nil && p.a.Plugins.AppMgr != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = p.a.Plugins.AppMgr.Restart(ctx, id)
			p.a.invalidatePluginUICache()
			p.a.InvalidatePluginUI()
		}()
	}
	if hasDaemon {
		h := p.a.Local()
		if p.a.hostSupportsPlugins(h) && h.Client != nil {
			p.a.localDaemonReloadPending.Add(1)
			h.Client.Go("plugin.restart", rpc.PluginControlParams{PluginID: id}, func(err error) {
				if err != nil {
					p.a.consumeLocalDaemonReload()
					p.a.main(func() { p.a.toast(err.Error()) })
					return
				}
				p.a.RefreshDaemonPluginStatuses(h)
			})
		}
	}
}

func (p *appPluginsSettings) Remove(id string) {
	st := p.store()
	if err := st.Remove(id, false); err != nil {
		p.a.toast(err.Error())
		return
	}
	_ = cli.SyncPluginAgentIntegrations(st, nil)
	if p.a.Ss.PluginDetail == id {
		p.a.Ss.PluginDetail = ""
	}
	if p.a.Ss.PluginLogsID == id {
		p.a.Ss.PluginLogsID = ""
		p.a.Ss.PluginLogsText = ""
	}
	p.a.State.Soon()
	_ = p.a.reloadPlugins(true)
}

func (p *appPluginsSettings) ReadLogs(id string) string {
	_, text, _, err := daemonhost.ReadPluginLogs(p.store(), id, 100)
	if err != nil {
		return err.Error()
	}
	return strings.TrimRight(text, "\n")
}

func (p *appPluginsSettings) LoadConfig(id string) map[string]any {
	st := p.store()
	cfgPath, pathErr := st.ConfigPath(id)
	var (
		statOK  bool
		exists  bool
		modTime time.Time
		size    int64
	)
	if pathErr == nil {
		if fi, err := os.Stat(cfgPath); err == nil {
			statOK, exists, modTime, size = true, true, fi.ModTime(), fi.Size()
		} else if os.IsNotExist(err) {
			statOK, exists = true, false
		}
	}

	p.a.pluginCache.mu.Lock()
	if ent, ok := p.a.pluginCache.configs[id]; ok && statOK && ent.exists == exists && (!exists || (ent.modTime.Equal(modTime) && ent.size == size)) {
		out := maps.Clone(ent.cfg)
		p.a.pluginCache.mu.Unlock()
		return out
	}
	p.a.pluginCache.mu.Unlock()

	cfg, err := st.LoadConfig(id)
	if err != nil {
		return map[string]any{}
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	p.a.pluginCache.mu.Lock()
	if p.a.pluginCache.configs == nil {
		p.a.pluginCache.configs = make(map[string]cachedPluginConfig)
	}
	p.a.pluginCache.configs[id] = cachedPluginConfig{
		modTime: modTime,
		size:    size,
		exists:  exists,
		cfg:     maps.Clone(cfg),
	}
	p.a.pluginCache.mu.Unlock()
	return cfg
}

func (p *appPluginsSettings) SaveConfig(id string, cfg map[string]any) {
	st := p.store()
	if err := st.SaveConfig(id, cfg); err != nil {
		p.a.toast(err.Error())
		return
	}
	p.a.invalidatePluginUICacheFor(id)
	p.a.State.Soon()
	_ = p.a.reloadPlugins(true)
}

func (p *appPluginsSettings) HasSecret(id, key string) bool {
	cacheKey := id + "\x00" + key
	p.a.pluginCache.mu.Lock()
	if ent, ok := p.a.pluginCache.secrets[cacheKey]; ok && time.Since(ent.at) < 2*time.Second {
		p.a.pluginCache.mu.Unlock()
		return ent.has
	}
	p.a.pluginCache.mu.Unlock()

	v, ok, _ := p.store().GetSecret(id, key)
	has := ok && v != ""
	p.a.pluginCache.mu.Lock()
	if p.a.pluginCache.secrets == nil {
		p.a.pluginCache.secrets = make(map[string]cachedPluginSecret)
	}
	p.a.pluginCache.secrets[cacheKey] = cachedPluginSecret{has: has, at: time.Now()}
	p.a.pluginCache.mu.Unlock()
	return has
}

func (p *appPluginsSettings) SetSecret(id, key, val string) {
	if err := p.store().SetSecret(id, key, val); err != nil {
		p.a.toast(err.Error())
		return
	}
	cacheKey := id + "\x00" + key
	p.a.pluginCache.mu.Lock()
	if p.a.pluginCache.secrets == nil {
		p.a.pluginCache.secrets = make(map[string]cachedPluginSecret)
	}
	p.a.pluginCache.secrets[cacheKey] = cachedPluginSecret{has: val != "", at: time.Now()}
	p.a.pluginCache.mu.Unlock()
	p.a.State.Soon()
	_ = p.a.reloadPlugins(true)
}

func (p *appPluginsSettings) SettingsPages() []uisettings.PluginSettingsPage {
	if p.a.Plugins == nil {
		return nil
	}
	slots := p.a.Plugins.Slots(manifest.MountSettingsPage, nil, nil)
	if len(slots) == 0 {
		return nil
	}
	names := map[string]string{}
	if snap := p.a.Plugins.Snapshot(); snap != nil {
		for _, ip := range snap.Plugins {
			if ip.Manifest != nil {
				names[ip.Entry.ID] = ip.Manifest.Name
			}
		}
	}
	var out []uisettings.PluginSettingsPage
	for _, sl := range slots {
		if sl.Slot.Mode == "webview" {
			continue
		}
		title := sl.Slot.Title
		if title == "" {
			title = names[sl.PluginID]
		}
		if title == "" {
			title = sl.Slot.ID
		}
		svg, _ := p.a.Plugins.IconForSlot(sl.PluginDir, sl.Slot.Icon)
		if svg == nil && sl.Slot.Icon != "" {
			svg, _ = glyph.Try(sl.Slot.Icon)
		}
		if svg == nil {
			svg = glyph.Plugins
		}
		out = append(out, uisettings.PluginSettingsPage{
			ID:       "plugin:" + sl.PluginID + ":" + sl.Slot.ID,
			PluginID: sl.PluginID,
			SlotID:   sl.Slot.ID,
			Title:    title,
			Icon:     svg,
		})
	}
	return out
}

func slotHasHostProcess(plg *manifest.InstalledPlugin, slHost string) bool {
	if plg == nil || plg.Manifest == nil {
		return false
	}
	switch slHost {
	case "app":
		return plg.Manifest.App != nil
	case "daemon":
		return plg.Manifest.Daemon != nil
	default:
		return plg.Manifest.App != nil || plg.Manifest.Daemon != nil
	}
}

func (p *appPluginsSettings) RenderSettingsPage(c *ui.Context, pal *theme.Palette, pageID string) bool {
	if p.a.Plugins == nil {
		return false
	}
	pluginID, slotID, ok := parsePluginTabID(pageID)
	if !ok {
		return false
	}
	sl, found := p.a.Plugins.FindSlot(pluginID, slotID)
	if !found || sl.Slot.Mount != manifest.MountSettingsPage || sl.Slot.Mode == "webview" {
		return false
	}
	var plg *manifest.InstalledPlugin
	if snap := p.a.Plugins.Snapshot(); snap != nil {
		for i := range snap.Plugins {
			if snap.Plugins[i].Entry.ID == pluginID {
				plg = &snap.Plugins[i]
				break
			}
		}
	}
	if plg == nil || plg.Manifest == nil {
		return false
	}
	// Static packs (or slots with no matching host process) render [[settings]] only.
	if !slotHasHostProcess(plg, sl.Host) {
		return false
	}
	if sl.Host == "daemon" && (p.a.Plugins.DaemonFn == nil || p.a.Plugins.DaemonFn("local") == nil) {
		if len(plg.Manifest.Settings) == 0 {
			ui.Text(c.Key("plg-page-unavail-"+pageID), locales.PluginUnavailableDetail(pluginID)).
				FontSize(12.5).TextColor(pal.TextMuted).Wrap()
			return true
		}
		return false
	}
	req := rpc.UIRenderParams{
		SlotID: sl.Slot.ID,
		Mount:  manifest.MountSettingsPage,
		Dark:   pal.Dark,
		Locale: p.a.pluginLocaleValue(),
	}
	cr := p.a.Plugins.RenderSlot("local", sl, req)
	if !cr.Loading && cr.Error == "" && cr.Result != nil && cr.Result.Root == nil {
		return false
	}
	ui.Column(c.Key("plg-page-slot-" + pageID)).Gap(16).AlignItems(ui.Stretch).Children(func() {
		p.a.renderPluginSlot(c, pal, "local", sl, "", "", cr, true)
	})
	return true
}

func (p *appPluginsSettings) RenderSettingsMount(c *ui.Context, pal *theme.Palette, mount, pluginID string) {
	if p.a.Plugins == nil {
		return
	}
	slots := p.a.Plugins.Slots(mount, nil, nil)
	if len(slots) == 0 {
		return
	}
	snap := p.a.Plugins.Snapshot()
	hasHostProc := func(pid, slHost string) bool {
		if snap == nil {
			return true
		}
		for i := range snap.Plugins {
			if snap.Plugins[i].Entry.ID == pid {
				return slotHasHostProcess(&snap.Plugins[i], slHost)
			}
		}
		return false
	}
	for _, sl := range slots {
		if sl.Slot.Mode == "webview" {
			continue
		}
		if pluginID != "" && sl.PluginID != pluginID {
			continue
		}
		if !hasHostProc(sl.PluginID, sl.Host) {
			continue
		}
		if sl.Host == "daemon" && (p.a.Plugins.DaemonFn == nil || p.a.Plugins.DaemonFn("local") == nil) {
			if mount == manifest.MountSettingsPlugins {
				ui.Text(c.Key("plg-slot-unavail-"+sl.PluginID+"-"+sl.Slot.ID), locales.PluginUnavailableDetail(sl.PluginID)).
					FontSize(12).TextColor(pal.TextMuted).Wrap()
			}
			continue
		}
		req := rpc.UIRenderParams{
			SlotID: sl.Slot.ID,
			Mount:  mount,
			Dark:   pal.Dark,
			Locale: p.a.pluginLocaleValue(),
		}
		cr := p.a.Plugins.RenderSlot("local", sl, req)
		if !cr.Loading && cr.Error == "" && cr.Result != nil && cr.Result.Root == nil {
			continue
		}
		title := sl.Slot.Title
		if cr.Result != nil && cr.Result.Title != "" {
			title = cr.Result.Title
		}
		slotCopy := sl
		slotKey := "plg-mount-" + mount + "-" + sl.PluginID + "-" + sl.Slot.ID
		if mount == manifest.MountSettingsPlugins {
			ui.Column(c.Key(slotKey)).Padding(4, 0, 0, 0).Gap(6).AlignItems(ui.Stretch).Children(func() {
				if title != "" && (cr.Loading || cr.Error != "" || (cr.Result != nil && cr.Result.Root != nil && !hasSettingsGroupOrCard(cr.Result.Root))) {
					hdrRow := ui.Row(c.Key(slotKey+"-hdr-row")).Padding(0, 2)
					hdrRow.Children(func() {
						kit.Highlight(c, hdrRow, title, pal, 4, 2)
						ui.Text(c.Key(slotKey+"-hdr"), title).FontSize(12).FontWeight(600).TextColor(pal.TextMuted)
					})
				}
				p.a.renderPluginSlot(c, pal, "local", slotCopy, "", "", cr, true)
			})
			continue
		}
		needsGroupWrap := cr.Loading || cr.Error != "" || (cr.Result != nil && cr.Result.Root != nil && !hasSettingsGroupOrCard(cr.Result.Root))
		if needsGroupWrap {
			kit.Group(c, pal, title, "", func() {
				ui.Column(c.Key(slotKey)).Padding(8, 0).Gap(8).AlignItems(ui.Stretch).Children(func() {
					p.a.renderPluginSlot(c, pal, "local", slotCopy, "", "", cr, true)
				})
			})
		} else {
			ui.Column(c.Key(slotKey)).Gap(16).AlignItems(ui.Stretch).Children(func() {
				p.a.renderPluginSlot(c, pal, "local", slotCopy, "", "", cr, true)
			})
		}
	}
}

func hasSettingsGroupOrCard(n *rpc.UINode) bool {
	if n == nil {
		return false
	}
	if n.Type == "settingsGroup" || n.Type == "infoCard" {
		return true
	}
	if n.Type == "column" && len(n.Children) > 0 {
		for _, ch := range n.Children {
			if hasSettingsGroupOrCard(ch) {
				return true
			}
		}
	}
	return false
}

func (p *appPluginsSettings) SearchEntries() []search.PluginEntry {
	installed := p.List()
	if len(installed) == 0 {
		return nil
	}
	firstPageByPlugin := map[string]string{}
	for _, pp := range p.SettingsPages() {
		if firstPageByPlugin[pp.PluginID] == "" {
			firstPageByPlugin[pp.PluginID] = pp.ID
		}
	}
	var out []search.PluginEntry
	for _, ip := range installed {
		if ip.Manifest == nil || ip.Entry.PendingInstall {
			continue
		}
		m := ip.Manifest
		rowTitle := uisettings.PluginRowTitle(ip)
		plgWords := strings.Join([]string{ip.Entry.ID, m.Name, m.Description, m.Author}, " ")
		out = append(out, search.PluginEntry{
			Page:  "plugins",
			Label: rowTitle,
			Words: plgWords,
		})
		for _, sp := range m.Settings {
			lbl := sp.Title
			if lbl == "" {
				lbl = sp.Key
			}
			words := strings.Join([]string{sp.Key, sp.Description, m.Name, ip.Entry.ID}, " ")
			out = append(out, search.PluginEntry{
				Page:     "plugins",
				Label:    lbl,
				Words:    words,
				PluginID: ip.Entry.ID,
			})
			if pgID := firstPageByPlugin[ip.Entry.ID]; pgID != "" {
				out = append(out, search.PluginEntry{
					Page:  pgID,
					Label: lbl,
					Words: words,
				})
			}
		}
		for _, sc := range m.Permissions.Scopes {
			out = append(out, search.PluginEntry{
				Page:     "plugins",
				Label:    sc,
				Words:    m.Name + " " + ip.Entry.ID + " permission",
				PluginID: ip.Entry.ID,
			})
		}
		if ip.Entry.Enabled && ip.Entry.HasPermission(manifest.PermUISlot) {
			for _, sl := range m.UI.Slots {
				if sl.Mode == "webview" {
					continue
				}
				slotTitle := sl.Title
				if slotTitle == "" {
					slotTitle = m.Name
				}
				if slotTitle == "" {
					slotTitle = sl.ID
				}
				slotWords := strings.Join([]string{sl.ID, sl.Title, m.Name, ip.Entry.ID, m.Description}, " ")
				switch {
				case sl.Mount == manifest.MountSettingsPage:
					pgID := "plugin:" + ip.Entry.ID + ":" + sl.ID
					out = append(out, search.PluginEntry{
						Page:  pgID,
						Label: slotTitle,
						Words: slotWords,
					})
					if p.a.Plugins != nil {
						if cr, ok := p.a.Plugins.CachedRender("local", ip.Entry.ID, sl.ID, "", ""); ok && cr.Result != nil && cr.Result.Root != nil {
							out = appendCachedNodeSearchEntries(out, pgID, "", plgWords, cr.Result.Root, 0)
						}
					}
				case sl.Mount == manifest.MountSettingsPlugins:
					out = append(out, search.PluginEntry{
						Page:     "plugins",
						Label:    slotTitle,
						Words:    slotWords,
						PluginID: ip.Entry.ID,
					})
					if p.a.Plugins != nil {
						if cr, ok := p.a.Plugins.CachedRender("local", ip.Entry.ID, sl.ID, "", ""); ok && cr.Result != nil && cr.Result.Root != nil {
							out = appendCachedNodeSearchEntries(out, "plugins", ip.Entry.ID, plgWords, cr.Result.Root, 0)
						}
					}
				default:
					if targetPage, ok := manifest.BuiltInSettingsMount(sl.Mount); ok {
						out = append(out, search.PluginEntry{
							Page:  targetPage,
							Label: slotTitle,
							Words: slotWords,
						})
						if p.a.Plugins != nil {
							if cr, ok := p.a.Plugins.CachedRender("local", ip.Entry.ID, sl.ID, "", ""); ok && cr.Result != nil && cr.Result.Root != nil {
								out = appendCachedNodeSearchEntries(out, targetPage, "", plgWords, cr.Result.Root, 0)
							}
						}
					}
				}
			}
		}
	}
	return out
}

func appendCachedNodeSearchEntries(out []search.PluginEntry, pageID, drawerPluginID, baseWords string, n *rpc.UINode, depth int) []search.PluginEntry {
	if n == nil || depth > 32 {
		return out
	}
	strProp := func(k string) string {
		if n.Props == nil {
			return ""
		}
		if s, ok := n.Props[k].(string); ok {
			return s
		}
		return ""
	}
	switch n.Type {
	case "settingsGroup":
		if title := strProp("title"); title != "" {
			note := strProp("note")
			out = append(out, search.PluginEntry{
				Page:     pageID,
				Label:    title,
				Words:    strings.TrimSpace(note + " " + baseWords),
				PluginID: drawerPluginID,
			})
		}
	case "settingsRow":
		if lbl := strProp("label"); lbl != "" {
			detail := strProp("detail")
			out = append(out, search.PluginEntry{
				Page:     pageID,
				Label:    lbl,
				Words:    strings.TrimSpace(detail + " " + baseWords),
				PluginID: drawerPluginID,
			})
		}
	case "infoCard":
		if title := strProp("title"); title != "" {
			aside := strProp("aside")
			out = append(out, search.PluginEntry{
				Page:     pageID,
				Label:    title,
				Words:    strings.TrimSpace(aside + " " + baseWords),
				PluginID: drawerPluginID,
			})
		}
	case "text":
		txt := strProp("text")
		if txt == "" {
			txt = strProp("value")
		}
		if txt != "" {
			out = append(out, search.PluginEntry{
				Page:     pageID,
				Label:    txt,
				Words:    baseWords,
				PluginID: drawerPluginID,
			})
		}
	case "button":
		lbl := strProp("label")
		if lbl == "" {
			lbl = strProp("title")
		}
		if lbl != "" {
			out = append(out, search.PluginEntry{
				Page:     pageID,
				Label:    lbl,
				Words:    baseWords,
				PluginID: drawerPluginID,
			})
		}
	}
	for _, ch := range n.Children {
		out = appendCachedNodeSearchEntries(out, pageID, drawerPluginID, baseWords, ch, depth+1)
	}
	return out
}
