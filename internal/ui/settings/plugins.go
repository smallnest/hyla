package settings

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/settings/kit"
	"hyla/internal/ui/settings/search"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// PluginSettingsPage describes a standalone top-level Settings page contributed
// by an enabled plugin via [[ui.slots]] with mount = "settings.page".
type PluginSettingsPage struct {
	ID       string
	PluginID string
	SlotID   string
	Title    string
	Icon     *ui.SVG
}

// PluginsApp exposes plugin lifecycle, permissions, configuration, and logs to Settings › Plugins.
type PluginsApp interface {
	List() []manifest.InstalledPlugin
	DaemonStatuses() []rpc.PluginStatus
	AppStatuses() []rpc.PluginStatus
	ReloadAll()
	InstallPackage()
	SetEnabled(id string, on bool)
	SetPermission(id, scope string, granted bool)
	Restart(id string)
	Remove(id string)
	ReadLogs(id string) string
	LoadConfig(id string) map[string]any
	SaveConfig(id string, cfg map[string]any)
	HasSecret(id, key string) bool
	SetSecret(id, key, val string)
	SettingsPages() []PluginSettingsPage
	RenderSettingsPage(c *ui.Context, pal *theme.Palette, pageID string) bool
	RenderSettingsMount(c *ui.Context, pal *theme.Palette, mount, pluginID string)
	SearchEntries() []search.PluginEntry
}

// PluginRowTitle returns the display title shown for a plugin row in Settings › Plugins.
func PluginRowTitle(plg manifest.InstalledPlugin) string {
	name := plg.Entry.ID
	if plg.Manifest != nil && plg.Manifest.Name != "" {
		name = plg.Manifest.Name
	}
	if plg.Entry.Version != "" {
		name += " · v" + plg.Entry.Version
	}
	return name
}

func pluginsPage(x App, c *ui.Context, pal *theme.Palette) {
	ss := x.SettingsState()
	pa := x.PluginsView()
	if pa == nil {
		return
	}
	if ss.PluginDrafts == nil {
		ss.PluginDrafts = map[string]*PluginFieldDraft{}
	}

	kit.Group(c, pal, locales.PagePlugins(), locales.SetPluginsHeaderDetail(), func() {
		ui.Row(c.Key("plg-toolbar")).Padding(8, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			if style.Button(c, "plg-install", locales.SetPluginsInstallPkg(), "", true, pal) {
				pa.InstallPackage()
			}
			if style.Button(c, "plg-reload", locales.SetPluginsReload(), "", false, pal) {
				pa.ReloadAll()
			}
		})
	})

	installed := pa.List()
	if len(installed) == 0 {
		kit.Group(c, pal, locales.SetPluginsInstalledGroup(), "", func() {
			kit.Row(c, pal, locales.SetPluginsEmptyTitle(), locales.SetPluginsEmptyDetail(), func() {})
		})
		return
	}

	statusByID := map[string]rpc.PluginStatus{}
	for _, st := range pa.DaemonStatuses() {
		statusByID[st.ID] = st
	}
	for _, st := range pa.AppStatuses() {
		if cur, exists := statusByID[st.ID]; !exists || st.State == "running" || st.State == "degraded" || cur.State == "" || cur.State == "stopped" {
			statusByID[st.ID] = st
		}
	}

	kit.Group(c, pal, locales.SetPluginsInstalledGroup(), "", func() {
		for _, p := range installed {
			plg := p
			id := plg.Entry.ID
			st := statusByID[id]

			name := PluginRowTitle(plg)

			var metaParts []string
			metaParts = append(metaParts, id)
			switch {
			case plg.Entry.PendingInstall:
				metaParts = append(metaParts, locales.SetPluginsPendingDetail())
			case plg.Builtin:
				metaParts = append(metaParts, locales.SetPluginsBuiltin())
			case plg.Entry.Source == "link" && plg.Entry.LinkPath != "":
				metaParts = append(metaParts, locales.SetPluginsLinked(plg.Entry.LinkPath))
			}
			if st.State != "" && !plg.Entry.PendingInstall {
				stateStr := st.State
				if st.PID > 0 {
					stateStr += fmt.Sprintf(" (pid %d)", st.PID)
				}
				metaParts = append(metaParts, stateStr)
			}
			detail := strings.Join(metaParts, " · ")

			hasSettingsSlot := pluginHasSettingsDrawerSlot(plg.Manifest)
			kit.Row(c, pal, name, detail, func() {
				if plg.Manifest != nil && (len(plg.Manifest.Permissions.Scopes) > 0 || len(plg.Manifest.Settings) > 0 || hasSettingsSlot) {
					label := locales.SetPluginsSettings()
					if ss.PluginDetail == id {
						label = locales.ActionClose()
					}
					if style.Button(c, "plg-cfg-"+id, label, "", false, pal) {
						if ss.PluginDetail == id {
							ss.PluginDetail = ""
						} else {
							ss.PluginDetail = id
						}
					}
				}
				if plg.Manifest != nil && (plg.Manifest.Daemon != nil || plg.Manifest.App != nil) && plg.Entry.Enabled && !plg.Entry.PendingInstall {
					if style.Button(c, "plg-rst-"+id, locales.SetPluginsRestart(), "", false, pal) {
						pa.Restart(id)
					}
				}
				logLbl := locales.SetPluginsLogs()
				if ss.PluginLogsID == id {
					logLbl = locales.SetPluginsHideLogs()
				}
				if style.Button(c, "plg-log-"+id, logLbl, "", false, pal) {
					if ss.PluginLogsID == id {
						ss.PluginLogsID = ""
						ss.PluginLogsText = ""
					} else {
						ss.PluginLogsID = id
						ss.PluginLogsText = pa.ReadLogs(id)
					}
				}
				if !plg.Builtin {
					if style.Button(c, "plg-rm-"+id, locales.SetPluginsRemove(), "", false, pal) {
						pid, pname := id, name
						ss.Confirm = &confirm.Ask{
							Open:    true,
							Title:   locales.SetPluginsConfirmRemoveTitle(pname),
							Detail:  locales.SetPluginsConfirmRemoveDetail(),
							Action:  locales.SetPluginsRemove(),
							Destroy: true,
							Run:     func() { pa.Remove(pid) },
						}
					}
				}
				if !plg.Entry.PendingInstall {
					on := plg.Entry.Enabled
					style.Toggle(c.Key("plg-en-"+id), &on, pal).OnChange(func() {
						pa.SetEnabled(id, on)
					})
				}
			})

			errMsg := plg.LoadError
			if errMsg == "" {
				errMsg = st.LastError
			}
			if errMsg != "" {
				ui.Text(c.Key("plg-err-"+id), errMsg).FontSize(12).TextColor(pal.Danger).LineHeight(1.35).Wrap().Padding(0, 0, 8, 0)
			}

			if ss.PluginDetail == id && plg.Manifest != nil {
				pluginDetailDrawer(c, pal, ss, pa, plg)
			}
			if ss.PluginLogsID == id {
				txt := ss.PluginLogsText
				if txt == "" {
					txt = "(empty)"
				}
				ui.Column(c.Key("plg-logs-"+id)).Height(140).Radius(8).Padding(8, 10).Margin(0, 0, 10, 0).Background(pal.Card).Children(func() {
					ui.Scroll(c.Key("plg-log-scroll-"+id)).Grow(1).MinHeight(0).Children(func() {
						ui.Text(c, txt).Font("monospace").FontSize(11).TextColor(pal.TextMuted).Wrap()
					})
				})
			}
		}
	})
}

func pluginHasSettingsDrawerSlot(m *manifest.Manifest) bool {
	if m == nil {
		return false
	}
	for _, sl := range m.UI.Slots {
		if sl.Mount == manifest.MountSettingsPlugins && sl.Mode != "webview" {
			return true
		}
	}
	return false
}

func pluginDetailDrawer(c *ui.Context, pal *theme.Palette, ss *State, pa PluginsApp, plg manifest.InstalledPlugin) {
	id := plg.Entry.ID
	m := plg.Manifest
	hasSlot := pluginHasSettingsDrawerSlot(m)
	ui.Column(c.Key("plg-drawer-"+id)).Padding(8, 12, 10, 12).Margin(0, 0, 8, 0).Radius(8).Background(pal.Card).Gap(6).Children(func() {
		if len(m.Permissions.Scopes) > 0 {
			ui.Text(c, locales.SetPluginsPermissions()).FontSize(12).FontWeight(600).TextColor(pal.TextMuted)
			for _, scope := range m.Permissions.Scopes {
				sc := scope
				row := ui.Row(c.Key("plg-perm-"+id+"-"+sc)).Padding(3, 0).Gap(10).AlignItems(ui.Center)
				row.Children(func() {
					kit.Highlight(c, row, sc, pal, 6, 0)
					ui.Text(c, sc).Font("monospace").FontSize(12).TextColor(pal.Text).Grow(1)
					granted := slices.Contains(plg.Entry.GrantedPermissions, sc)
					style.Toggle(c.Key("plg-ptog-"+id+"-"+sc), &granted, pal).OnChange(func() {
						pa.SetPermission(id, sc, granted)
					})
				})
			}
		}
		if len(m.Settings) > 0 {
			ui.Text(c, locales.SetPluginsSettings()).FontSize(12).FontWeight(600).TextColor(pal.TextMuted).Padding(4, 0, 0, 0)
			cfg := pa.LoadConfig(id)
			for _, spec := range m.Settings {
				sp := spec
				draftKey := id + "|" + sp.Key
				lbl := sp.Title
				if lbl == "" {
					lbl = sp.Key
				}
				row := ui.Row(c.Key("plg-set-"+draftKey)).Padding(4, 0).Gap(10).AlignItems(ui.Center)
				row.Children(func() {
					kit.Highlight(c, row, lbl, pal, 6, 0)
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, lbl).FontSize(12.5).TextColor(pal.Text)
						if sp.Description != "" {
							ui.Text(c, sp.Description).FontSize(11.5).TextColor(pal.TextMuted).Wrap()
						}
					})
					renderPluginSettingControl(c, pal, ss, pa, id, cfg, sp)
				})
			}
		}
		if hasSlot {
			if plg.Entry.Enabled && !plg.Entry.PendingInstall && plg.Entry.HasPermission(manifest.PermUISlot) {
				pa.RenderSettingsMount(c, pal, manifest.MountSettingsPlugins, id)
			} else if len(m.Permissions.Scopes) == 0 && len(m.Settings) == 0 {
				ui.Text(c.Key("plg-slot-unavail-"+id), locales.PluginUnavailableDetail(id)).FontSize(12).TextColor(pal.TextMuted).Wrap()
			}
		}
	})
}

func pluginSettingsPage(x App, c *ui.Context, pal *theme.Palette, pageID string) {
	pa := x.PluginsView()
	if pa == nil {
		return
	}
	ss := x.SettingsState()
	if ss.PluginDrafts == nil {
		ss.PluginDrafts = map[string]*PluginFieldDraft{}
	}
	pa.RenderSettingsPage(c, pal, pageID)
	pages := pa.SettingsPages()
	var matchedPage *PluginSettingsPage
	for i := range pages {
		if pages[i].ID == pageID {
			matchedPage = &pages[i]
			break
		}
	}
	if matchedPage == nil {
		return
	}
	firstPageID := ""
	for _, pp := range pages {
		if pp.PluginID == matchedPage.PluginID {
			firstPageID = pp.ID
			break
		}
	}
	if firstPageID != pageID {
		return
	}
	for _, plg := range pa.List() {
		if plg.Entry.ID == matchedPage.PluginID && plg.Manifest != nil && len(plg.Manifest.Settings) > 0 {
			pluginSettingsGroup(c, pal, ss, pa, plg)
			break
		}
	}
}

func pluginSettingsGroup(c *ui.Context, pal *theme.Palette, ss *State, pa PluginsApp, plg manifest.InstalledPlugin) {
	id := plg.Entry.ID
	cfg := pa.LoadConfig(id)
	kit.Group(c, pal, locales.SetPluginsSettings(), "", func() {
		for _, spec := range plg.Manifest.Settings {
			sp := spec
			lbl := sp.Title
			if lbl == "" {
				lbl = sp.Key
			}
			kit.Row(c, pal, lbl, sp.Description, func() {
				renderPluginSettingControl(c, pal, ss, pa, id, cfg, sp)
			})
		}
	})
}

func renderPluginSettingControl(c *ui.Context, pal *theme.Palette, ss *State, pa PluginsApp, id string, cfg map[string]any, sp manifest.SettingSpec) {
	draftKey := id + "|" + sp.Key
	switch sp.Type {
	case "boolean":
		val := false
		if cur, ok := cfg[sp.Key]; ok {
			val, _ = cur.(bool)
		} else {
			val, _ = sp.Default.(bool)
		}
		style.Toggle(c.Key("plg-btog-"+draftKey), &val, pal).OnChange(func() {
			cfg[sp.Key] = val
			pa.SaveConfig(id, cfg)
		})
	case "select":
		val := ""
		if v, ok := cfg[sp.Key]; ok && v != nil {
			val = fmt.Sprint(v)
		} else if sp.Default != nil {
			val = fmt.Sprint(sp.Default)
		}
		if val == "" && len(sp.Options) > 0 {
			val = sp.Options[0]
		}
		controls.Popup(c, "plg-sel-"+draftKey, &val, sp.Options, func(o string) string { return o }, 160, pal).OnChange(func() {
			cfg[sp.Key] = val
			pa.SaveConfig(id, cfg)
		})
	case "secret":
		secKey := "secret:" + draftKey
		ptr := pluginDraftPtr(ss, secKey, "")
		ph := sp.Placeholder
		if ph == "" && pa.HasSecret(id, sp.Key) {
			ph = "••••••••"
		}
		in := controls.TextField(c, "plg-sec-"+draftKey, ptr, ph, 170, pal)
		if in.Valid() {
			in.Password()
		}
		saveSecret := func() {
			if *ptr == "" {
				return
			}
			pa.SetSecret(id, sp.Key, *ptr)
			*ptr = ""
		}
		in.OnSubmit(saveSecret)
		if style.Button(c, "plg-sec-save-"+draftKey, locales.CommonSave(), "", true, pal) {
			saveSecret()
		}
	default: // string, integer, number
		initVal := ""
		if v, exists := cfg[sp.Key]; exists && v != nil {
			initVal = formatSettingValue(v)
		} else if sp.Default != nil {
			initVal = formatSettingValue(sp.Default)
		}
		ptr := pluginDraftPtr(ss, draftKey, initVal)
		in := controls.TextField(c, "plg-in-"+draftKey, ptr, sp.Placeholder, 180, pal)
		if in.Valid() {
			raw := strings.TrimSpace(*ptr)
			switch sp.Type {
			case "integer":
				if raw != "" {
					if _, err := strconv.Atoi(raw); err != nil {
						in.TextColor(pal.Danger)
					}
				}
			case "number":
				if raw != "" {
					if _, err := strconv.ParseFloat(raw, 64); err != nil {
						in.TextColor(pal.Danger)
					}
				}
			}
		}
		saveText := func() {
			raw := strings.TrimSpace(*ptr)
			switch sp.Type {
			case "integer":
				if raw == "" {
					delete(cfg, sp.Key)
				} else {
					n, err := strconv.Atoi(raw)
					if err != nil {
						return
					}
					cfg[sp.Key] = n
				}
			case "number":
				if raw == "" {
					delete(cfg, sp.Key)
				} else {
					f, err := strconv.ParseFloat(raw, 64)
					if err != nil {
						return
					}
					cfg[sp.Key] = f
				}
			default:
				cfg[sp.Key] = *ptr
			}
			pa.SaveConfig(id, cfg)
			if raw == "" && (sp.Type == "integer" || sp.Type == "number") {
				def := ""
				if sp.Default != nil {
					def = fmt.Sprint(sp.Default)
				}
				*ptr = def
				if d := ss.PluginDrafts[draftKey]; d != nil {
					d.init = def
				}
			}
		}
		in.OnSubmit(saveText)
		if style.Button(c, "plg-in-save-"+draftKey, locales.CommonSave(), "", false, pal) {
			saveText()
		}
	}
}

func pluginDraftPtr(ss *State, draftKey, initVal string) *string {
	if ss.PluginDrafts == nil {
		ss.PluginDrafts = map[string]*PluginFieldDraft{}
	}
	d, ok := ss.PluginDrafts[draftKey]
	if !ok {
		d = &PluginFieldDraft{Value: initVal, init: initVal}
		ss.PluginDrafts[draftKey] = d
		return &d.Value
	}
	if d.init != initVal {
		if d.Value == d.init {
			d.Value = initVal
		}
		d.init = initVal
	}
	return &d.Value
}

func formatSettingValue(v any) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(n), 'f', -1, 32)
	default:
		return fmt.Sprint(v)
	}
}
