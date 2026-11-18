package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/ui/settings/search"
	"hyla/internal/ui/theme"
)

type stubApp struct {
	App
	ss *State
	pv PluginsApp
}

func (s *stubApp) SettingsState() *State  { return s.ss }
func (s *stubApp) PluginsView() PluginsApp { return s.pv }

type recordingPluginsApp struct {
	plugins []manifest.InstalledPlugin
	cfg     map[string]any
	secrets map[string]string

	enabledCalls []bool
	permCalls    []struct {
		scope   string
		granted bool
	}
	saveCalls    []map[string]any
	secretCalls  []string
	mountCalls   []string
	renderSlotFn func(c *ui.Context, pal *theme.Palette, mount, pluginID string)
}

func (r *recordingPluginsApp) List() []manifest.InstalledPlugin { return r.plugins }
func (r *recordingPluginsApp) DaemonStatuses() []rpc.PluginStatus {
	return []rpc.PluginStatus{{ID: "com.example.ui", State: "running"}}
}
func (r *recordingPluginsApp) AppStatuses() []rpc.PluginStatus { return nil }
func (r *recordingPluginsApp) ReloadAll()                      {}
func (r *recordingPluginsApp) InstallPackage()                 {}
func (r *recordingPluginsApp) SetEnabled(id string, on bool) {
	r.enabledCalls = append(r.enabledCalls, on)
	r.plugins[0].Entry.Enabled = on
}
func (r *recordingPluginsApp) SetPermission(id, scope string, granted bool) {
	r.permCalls = append(r.permCalls, struct {
		scope   string
		granted bool
	}{scope, granted})
	if granted {
		if !slices.Contains(r.plugins[0].Entry.GrantedPermissions, scope) {
			r.plugins[0].Entry.GrantedPermissions = append(r.plugins[0].Entry.GrantedPermissions, scope)
		}
	} else {
		r.plugins[0].Entry.GrantedPermissions = slices.DeleteFunc(r.plugins[0].Entry.GrantedPermissions, func(s string) bool { return s == scope })
	}
}
func (r *recordingPluginsApp) Restart(id string)         {}
func (r *recordingPluginsApp) Remove(id string)          {}
func (r *recordingPluginsApp) ReadLogs(id string) string { return "" }
func (r *recordingPluginsApp) LoadConfig(id string) map[string]any {
	out := map[string]any{}
	for k, v := range r.cfg {
		out[k] = v
	}
	return out
}
func (r *recordingPluginsApp) SaveConfig(id string, cfg map[string]any) {
	cp := map[string]any{}
	for k, v := range cfg {
		cp[k] = v
	}
	r.cfg = cp
	r.saveCalls = append(r.saveCalls, cp)
}
func (r *recordingPluginsApp) HasSecret(id, key string) bool { return r.secrets[key] != "" }
func (r *recordingPluginsApp) SetSecret(id, key, val string) {
	r.secrets[key] = val
	r.secretCalls = append(r.secretCalls, val)
}
func (r *recordingPluginsApp) SettingsPages() []PluginSettingsPage { return nil }
func (r *recordingPluginsApp) RenderSettingsPage(c *ui.Context, pal *theme.Palette, pageID string) bool {
	return false
}
func (r *recordingPluginsApp) RenderSettingsMount(c *ui.Context, pal *theme.Palette, mount, pluginID string) {
	r.mountCalls = append(r.mountCalls, mount+":"+pluginID)
	if r.renderSlotFn != nil {
		r.renderSlotFn(c, pal, mount, pluginID)
	}
}
func (r *recordingPluginsApp) SearchEntries() []search.PluginEntry { return nil }

func TestPluginsSettingsInteractiveCommits(t *testing.T) {
	pv := &recordingPluginsApp{
		cfg:     map[string]any{},
		secrets: map[string]string{},
		plugins: []manifest.InstalledPlugin{{
			Entry: manifest.RegistryEntry{
				ID:                 "com.example.ui",
				Enabled:            true,
				Version:            "1.0.0",
				GrantedPermissions: []string{"session.read"},
			},
			Manifest: &manifest.Manifest{
				ID:          "com.example.ui",
				Name:        "UI Demo",
				Version:     "1.0.0",
				Permissions: manifest.PermissionsSpec{Scopes: []string{"session.read"}},
				Settings: []manifest.SettingSpec{
					{Key: "flag", Title: "Feature Flag", Type: "boolean", Default: false},
					{Key: "mode", Title: "Mode", Type: "select", Default: "alpha", Options: []string{"alpha", "beta"}},
					{Key: "endpoint", Title: "Endpoint", Type: "string", Placeholder: "http://..."},
					{Key: "token", Title: "API Token", Type: "secret", Placeholder: "secret"},
				},
			},
		}},
	}
	app := &stubApp{
		ss: &State{Page: "plugins"},
		pv: pv,
	}

	tt := ui.NewTester(func(c *ui.Context) {
		pal := theme.For(c)
		ui.Scroll(c.Key("root-scroll")).Fill().Children(func() {
			ui.Column(c).Width(700).Padding(20).Children(func() {
				pluginsPage(app, c, pal)
			})
		})
	}, 760, 900)
	tt.Frame()

	// Find and click the enable toggle switch.
	var switches []ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleSwitch {
			switches = append(switches, a)
		}
	}
	if len(switches) == 0 {
		t.Fatalf("expected plugin enable RoleSwitch in AccessAll: %+v", tt.AccessAll())
	}
	sw := switches[0]
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	if len(pv.enabledCalls) != 1 || pv.enabledCalls[0] {
		t.Fatalf("expected SetEnabled(false) commit, got %v", pv.enabledCalls)
	}

	// Re-enable for remaining tests.
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	if len(pv.enabledCalls) != 2 || !pv.enabledCalls[1] {
		t.Fatalf("expected SetEnabled(true) commit, got %v", pv.enabledCalls)
	}

	// Open settings/permissions drawer by clicking Settings button.
	if err := tt.Click("Settings"); err != nil {
		// Locale string might vary if not "Settings"; search button role if needed.
		var clicked bool
		for _, a := range tt.AccessAll() {
			if a.Role == ui.RoleButton && strings.Contains(strings.ToLower(a.Name), "setting") {
				tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
				clicked = true
				break
			}
		}
		if !clicked {
			t.Fatalf("failed to open plugin drawer: %v (nodes=%+v)", err, tt.AccessAll())
		}
	}
	tt.Frame()

	// Locate permission switch, boolean setting switch, popup trigger, text fields, and Save buttons.
	switches = nil
	var inputs []ui.AccessInfo
	var saves []ui.AccessInfo
	var popupBtn *ui.AccessInfo
	for _, a := range tt.AccessAll() {
		switch a.Role {
		case ui.RoleSwitch:
			switches = append(switches, a)
		case ui.RoleTextField:
			inputs = append(inputs, a)
		case ui.RoleButton:
			if a.Name == "Save" || strings.Contains(strings.ToLower(a.Name), "save") {
				cp := a
				saves = append(saves, cp)
			} else if a.Name == "alpha" || a.Value == "alpha" {
				cp := a
				popupBtn = &cp
			}
		}
	}
	if len(switches) < 3 {
		t.Fatalf("expected 3 switches (enable, permission, boolean), got %d (%+v)", len(switches), switches)
	}

	// Permission toggle (switch[1]): currently true -> toggle to false.
	tt.ClickAt(switches[1].Bounds.X+switches[1].Bounds.W/2, switches[1].Bounds.Y+switches[1].Bounds.H/2)
	if len(pv.permCalls) != 1 || pv.permCalls[0].scope != "session.read" || pv.permCalls[0].granted {
		t.Fatalf("permission toggle did not commit: %+v", pv.permCalls)
	}

	// Boolean setting toggle (switch[2]): currently false -> toggle to true.
	tt.ClickAt(switches[2].Bounds.X+switches[2].Bounds.W/2, switches[2].Bounds.Y+switches[2].Bounds.H/2)
	if len(pv.saveCalls) == 0 || pv.cfg["flag"] != true {
		t.Fatalf("boolean setting toggle did not commit: saves=%+v cfg=%+v", pv.saveCalls, pv.cfg)
	}

	// Select popup: click trigger "alpha", then click "beta".
	if popupBtn == nil {
		for _, a := range tt.AccessAll() {
			if a.Name == "alpha" || a.Value == "alpha" || strings.Contains(a.Name, "alpha") {
				cp := a
				popupBtn = &cp
				break
			}
		}
	}
	if popupBtn == nil {
		t.Fatalf("could not find select popup trigger in %+v", tt.AccessAll())
	}
	tt.ClickAt(popupBtn.Bounds.X+popupBtn.Bounds.W/2, popupBtn.Bounds.Y+popupBtn.Bounds.H/2)
	tt.Frame()
	if err := tt.Click("beta"); err != nil {
		t.Fatalf("click popup item beta failed: %v", err)
	}
	if pv.cfg["mode"] != "beta" {
		t.Fatalf("select popup change did not commit mode=beta: cfg=%+v", pv.cfg)
	}

	// Secret & string TextFields: type into secret & endpoint fields, rebuild frame in between, then click Save.
	if len(inputs) < 2 || len(saves) < 2 {
		t.Fatalf("expected >=2 inputs and >=2 Save buttons, got inputs=%d saves=%d", len(inputs), len(saves))
	}
	// Secret input (first in schema? Wait: in settings schema flag(bool), mode(select), endpoint(string), token(secret); so inputs[0]=endpoint, inputs[1]=token)
	tt.ClickAt(inputs[0].Bounds.X+10, inputs[0].Bounds.Y+inputs[0].Bounds.H/2)
	tt.Type("http://127.0.0.1:9999")
	tt.Frame() // rebuild must NOT wipe draft!
	tt.ClickAt(saves[0].Bounds.X+saves[0].Bounds.W/2, saves[0].Bounds.Y+saves[0].Bounds.H/2)
	if pv.cfg["endpoint"] != "http://127.0.0.1:9999" {
		t.Fatalf("string text field did not survive rebuild / commit on Save: cfg=%+v", pv.cfg)
	}

	tt.ClickAt(inputs[1].Bounds.X+10, inputs[1].Bounds.Y+inputs[1].Bounds.H/2)
	tt.Type("tok-xyz")
	tt.Frame()
	tt.ClickAt(saves[1].Bounds.X+saves[1].Bounds.W/2, saves[1].Bounds.Y+saves[1].Bounds.H/2)
	if pv.secrets["token"] != "tok-xyz" {
		t.Fatalf("secret text field did not survive rebuild / commit on Save: secrets=%+v", pv.secrets)
	}
}

func TestPluginsDrawerButtonAppearsForSlotOnlyPlugin(t *testing.T) {
	pv := &recordingPluginsApp{
		cfg:     map[string]any{},
		secrets: map[string]string{},
		plugins: []manifest.InstalledPlugin{{
			Entry: manifest.RegistryEntry{
				ID:                 "com.example.slotonly",
				Enabled:            true,
				Version:            "1.0.0",
				GrantedPermissions: []string{manifest.PermUISlot},
			},
			Manifest: &manifest.Manifest{
				ID:      "com.example.slotonly",
				Name:    "Slot Only",
				Version: "1.0.0",
				UI: manifest.UISpec{
					Slots: []manifest.SlotSpec{{
						ID:    "drawer",
						Mount: manifest.MountSettingsPlugins,
						Title: "Connection",
						Mode:  "declarative",
					}},
				},
			},
		}},
		renderSlotFn: func(c *ui.Context, pal *theme.Palette, mount, pluginID string) {
			ui.Text(c.Key("slot-marker"), "Custom Drawer Body").TextColor(pal.Text)
		},
	}
	app := &stubApp{
		ss: &State{Page: "plugins"},
		pv: pv,
	}
	tt := ui.NewTester(func(c *ui.Context) {
		pal := theme.For(c)
		ui.Scroll(c.Key("root-scroll")).Fill().Children(func() {
			ui.Column(c).Width(700).Padding(20).Children(func() {
				pluginsPage(app, c, pal)
			})
		})
	}, 760, 600)
	tt.Frame()

	if err := tt.Click("Settings"); err != nil {
		t.Fatalf("expected Settings drawer button for slot-only plugin: %v (texts=%v)", err, tt.Texts())
	}
	tt.Frame()
	if app.ss.PluginDetail != "com.example.slotonly" {
		t.Fatalf("expected PluginDetail=com.example.slotonly, got %q", app.ss.PluginDetail)
	}
	if !tt.HasText("Custom Drawer Body") {
		t.Fatalf("expected custom slot body inside drawer, texts=%v", tt.Texts())
	}
	if !slices.Contains(pv.mountCalls, "settings.plugins:com.example.slotonly") {
		t.Fatalf("expected RenderSettingsMount call for settings.plugins, got %v", pv.mountCalls)
	}
}

func TestPluginsDraftSyncsExternalConfigAndPreservesEdits(t *testing.T) {
	pv := &recordingPluginsApp{
		cfg:     map[string]any{"endpoint": "http://old", "count": 5},
		secrets: map[string]string{},
		plugins: []manifest.InstalledPlugin{{
			Entry: manifest.RegistryEntry{
				ID:      "com.example.syncdraft",
				Enabled: true,
				Version: "1.0.0",
			},
			Manifest: &manifest.Manifest{
				ID:      "com.example.syncdraft",
				Name:    "Sync Draft",
				Version: "1.0.0",
				Settings: []manifest.SettingSpec{
					{Key: "endpoint", Title: "Endpoint", Type: "string"},
					{Key: "count", Title: "Count", Type: "integer"},
				},
			},
		}},
	}
	app := &stubApp{
		ss: &State{Page: "plugins", PluginDetail: "com.example.syncdraft"},
		pv: pv,
	}
	tt := ui.NewTester(func(c *ui.Context) {
		pal := theme.For(c)
		ui.Scroll(c.Key("root-scroll")).Fill().Children(func() {
			ui.Column(c).Width(700).Padding(20).Children(func() {
				pluginsPage(app, c, pal)
			})
		})
	}, 760, 600)
	tt.Frame()

	// Untouched draft reflects external config update (e.g., via plugin.setSettings).
	pv.cfg["endpoint"] = "http://updated"
	tt.Frame()
	if got := app.ss.PluginDraft["com.example.syncdraft|endpoint"]; got != "http://updated" {
		t.Fatalf("expected untouched draft synced to http://updated, got %q", got)
	}

	// User-edited draft is preserved even if external config changes underneath.
	var inputs []ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleTextField {
			inputs = append(inputs, a)
		}
	}
	if len(inputs) < 2 {
		t.Fatalf("expected 2 text fields, got %d", len(inputs))
	}
	tt.ClickAt(inputs[0].Bounds.X+10, inputs[0].Bounds.Y+inputs[0].Bounds.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("http://user-wip")
	tt.Frame()

	pv.cfg["endpoint"] = "http://external-overwrite"
	tt.Frame()
	if got := app.ss.PluginDraft["com.example.syncdraft|endpoint"]; got != "http://user-wip" {
		t.Fatalf("expected user-edited draft preserved, got %q", got)
	}

	// Invalid integer input leaves cfg unchanged on Save.
	tt.ClickAt(inputs[1].Bounds.X+10, inputs[1].Bounds.Y+inputs[1].Bounds.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("not-an-int")
	tt.Frame()
	var saves []ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "Save" {
			saves = append(saves, a)
		}
	}
	if len(saves) < 2 {
		t.Fatalf("expected 2 Save buttons, got %d", len(saves))
	}
	tt.ClickAt(saves[1].Bounds.X+saves[1].Bounds.W/2, saves[1].Bounds.Y+saves[1].Bounds.H/2)
	if pv.cfg["count"] != 5 {
		t.Fatalf("invalid integer should not overwrite cfg[count]=5, got %v", pv.cfg["count"])
	}
}
