package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	appsettings "hyla/internal/settings"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/market"
	"hyla/internal/plugins/rpc"
	"hyla/internal/ui/settings/search"
	"hyla/internal/ui/theme"
)

type stubApp struct {
	App
	ss *State
	pv PluginsApp
}

func (s *stubApp) SettingsState() *State           { return s.ss }
func (s *stubApp) PluginsView() PluginsApp         { return s.pv }
func (s *stubApp) Settings() *appsettings.Settings { return &appsettings.Settings{} }
func (s *stubApp) ActionTitle(id string) string    { return id }

type recordingPluginsApp struct {
	plugins     []manifest.InstalledPlugin
	cfg         map[string]any
	secrets     map[string]string
	daemonState string

	enabledCalls []bool
	permCalls    []struct {
		scope   string
		granted bool
	}
	restartCalls []string
	saveCalls    []map[string]any
	secretCalls  []string
	mountCalls   []string
	searchCalls  int
	renderSlotFn func(c *ui.Context, pal *theme.Palette, mount, pluginID string)

	market      MarketState
	refreshCnt  int
	installMkt  []market.Plugin
	openedURLs  []string
}

func (r *recordingPluginsApp) List() []manifest.InstalledPlugin { return r.plugins }
func (r *recordingPluginsApp) DaemonStatuses() []rpc.PluginStatus {
	if len(r.plugins) == 0 {
		return nil
	}
	st := r.daemonState
	if st == "" {
		st = "running"
	}
	if !r.plugins[0].Entry.Enabled {
		st = "stopped"
	}
	return []rpc.PluginStatus{{ID: r.plugins[0].Entry.ID, State: st, Enabled: r.plugins[0].Entry.Enabled}}
}
func (r *recordingPluginsApp) AppStatuses() []rpc.PluginStatus { return nil }
func (r *recordingPluginsApp) ReloadAll()                      {}
func (r *recordingPluginsApp) InstallPackage()                 {}
func (r *recordingPluginsApp) SetEnabled(id string, on bool) {
	r.enabledCalls = append(r.enabledCalls, on)
	r.plugins[0].Entry.Enabled = on
	if on {
		r.daemonState = "running"
	} else {
		r.daemonState = "stopped"
	}
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
func (r *recordingPluginsApp) Restart(id string) { r.restartCalls = append(r.restartCalls, id) }
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
func (r *recordingPluginsApp) SearchEntries() []search.PluginEntry {
	r.searchCalls++
	return nil
}
func (r *recordingPluginsApp) Market() MarketState { return r.market }
func (r *recordingPluginsApp) RefreshMarket()      { r.refreshCnt++ }
func (r *recordingPluginsApp) InstallMarketPlugin(p market.Plugin) {
	r.installMkt = append(r.installMkt, p)
}
func (r *recordingPluginsApp) OpenPluginURL(rawURL string) {
	r.openedURLs = append(r.openedURLs, rawURL)
}

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
	if d := app.ss.PluginDrafts["com.example.syncdraft|endpoint"]; d == nil || d.Value != "http://updated" {
		t.Fatalf("expected untouched draft synced to http://updated, got %v", d)
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
	if d := app.ss.PluginDrafts["com.example.syncdraft|endpoint"]; d == nil || d.Value != "http://user-wip" {
		t.Fatalf("expected user-edited draft preserved, got %v", d)
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

func TestPluginSettingsBlankSecretAndClearNumericAndSearchShortCircuit(t *testing.T) {
	pv := &recordingPluginsApp{
		cfg:     map[string]any{"count": 9},
		secrets: map[string]string{"token": "existing-secret"},
		plugins: []manifest.InstalledPlugin{{
			Entry: manifest.RegistryEntry{ID: "com.example.secnum", Enabled: true, Version: "1.0.0"},
			Manifest: &manifest.Manifest{
				ID:      "com.example.secnum",
				Name:    "SecNum",
				Version: "1.0.0",
				Settings: []manifest.SettingSpec{
					{Key: "token", Type: "secret", Title: "Token"},
					{Key: "count", Type: "integer", Title: "Count", Default: 3},
				},
			},
		}},
	}
	app := &stubApp{
		ss: &State{Page: "plugins", PluginDetail: "com.example.secnum"},
		pv: pv,
	}

	// Empty query must short-circuit searchSettings without invoking SearchEntries().
	searchSettings(app, nil)
	if pv.searchCalls != 0 {
		t.Fatalf("expected 0 SearchEntries calls on empty query, got %d", pv.searchCalls)
	}
	app.ss.Query = "tok"
	searchSettings(app, nil)
	if pv.searchCalls != 1 {
		t.Fatalf("expected 1 SearchEntries call on non-empty query, got %d", pv.searchCalls)
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

	var saves []ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "Save" {
			saves = append(saves, a)
		}
	}
	if len(saves) < 2 {
		t.Fatalf("expected 2 Save buttons, got %d", len(saves))
	}
	// Clicking Save with blank secret field must NOT call SetSecret("").
	tt.ClickAt(saves[0].Bounds.X+saves[0].Bounds.W/2, saves[0].Bounds.Y+saves[0].Bounds.H/2)
	tt.Frame()
	if len(pv.secretCalls) != 0 || pv.secrets["token"] != "existing-secret" {
		t.Fatalf("blank secret Save should preserve existing secret, secretCalls=%v", pv.secretCalls)
	}

	// Clearing integer field back to "" and clicking Save deletes cfg["count"] so default applies.
	var inputs []ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleTextField {
			inputs = append(inputs, a)
		}
	}
	if len(inputs) < 2 {
		t.Fatalf("expected 2 inputs, got %d", len(inputs))
	}
	tt.ClickAt(inputs[1].Bounds.X+10, inputs[1].Bounds.Y+inputs[1].Bounds.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	tt.ClickAt(saves[1].Bounds.X+saves[1].Bounds.W/2, saves[1].Bounds.Y+saves[1].Bounds.H/2)
	tt.Frame()
	if _, exists := pv.cfg["count"]; exists {
		t.Fatalf("expected clearing integer setting to delete key from config, got %v", pv.cfg["count"])
	}

	// Large float64 integers unmarshaled from JSON (e.g. 1_000_000) must not render in scientific notation ("1e+06").
	pv.cfg["count"] = float64(1000000)
	app.ss.PluginDrafts = nil
	tt.Frame()
	if got := app.ss.PluginDrafts["com.example.secnum|count"]; got == nil || got.Value != "1000000" {
		t.Fatalf("expected large float64 integer rendered as '1000000', got %+v", got)
	}
	tt.ClickAt(saves[1].Bounds.X+saves[1].Bounds.W/2, saves[1].Bounds.Y+saves[1].Bounds.H/2)
	tt.Frame()
	if pv.cfg["count"] != 1000000 {
		t.Fatalf("expected saving large float64 integer to keep 1000000, got %v (%T)", pv.cfg["count"], pv.cfg["count"])
	}
}

func TestPluginsSettingsDisabledStateAndRestartButton(t *testing.T) {
	pv := &recordingPluginsApp{
		cfg:         map[string]any{},
		secrets:     map[string]string{},
		daemonState: "running",
		plugins: []manifest.InstalledPlugin{{
			Entry: manifest.RegistryEntry{
				ID:      "com.example.proc",
				Enabled: true,
				Version: "1.0.0",
			},
			Manifest: &manifest.Manifest{
				ID:      "com.example.proc",
				Name:    "Proc Demo",
				Version: "1.0.0",
				Daemon: &manifest.HostProcessSpec{
					Command: []string{"echo"},
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
	}, 760, 600)
	tt.Frame()

	findRestart := func() ui.AccessInfo {
		for _, a := range tt.AccessAll() {
			if a.Role == ui.RoleButton && a.Name == "Restart" {
				return a
			}
		}
		t.Fatalf("expected Restart button in AccessAll: %+v", tt.AccessAll())
		return ui.AccessInfo{}
	}
	findSwitch := func() ui.AccessInfo {
		for _, a := range tt.AccessAll() {
			if a.Role == ui.RoleSwitch {
				return a
			}
		}
		t.Fatalf("expected enable switch in AccessAll")
		return ui.AccessInfo{}
	}

	// Enabled -> shows running and Restart button is enabled & clickable.
	if !tt.HasText("running") {
		t.Fatalf("expected running status text; texts=%v", tt.Texts())
	}
	rst := findRestart()
	if rst.Disabled {
		t.Fatal("expected Restart button enabled when plugin is enabled")
	}
	tt.ClickAt(rst.Bounds.X+rst.Bounds.W/2, rst.Bounds.Y+rst.Bounds.H/2)
	tt.Frame()
	if len(pv.restartCalls) != 1 || pv.restartCalls[0] != "com.example.proc" {
		t.Fatalf("restartCalls = %v, want [com.example.proc]", pv.restartCalls)
	}

	// Toggle switch OFF -> shows stopped, Restart stays visible with Disabled=true and ignores clicks.
	sw := findSwitch()
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	tt.Frame()
	if !tt.HasText("stopped") {
		t.Fatalf("expected stopped status text after disable; texts=%v", tt.Texts())
	}
	rst = findRestart()
	if !rst.Disabled {
		t.Fatal("expected Restart button disabled when plugin is disabled")
	}
	tt.ClickAt(rst.Bounds.X+rst.Bounds.W/2, rst.Bounds.Y+rst.Bounds.H/2)
	tt.Frame()
	if len(pv.restartCalls) != 1 {
		t.Fatalf("disabled Restart button should not trigger Restart; restartCalls=%v", pv.restartCalls)
	}

	// Toggle switch ON -> shows running and Restart becomes enabled & clickable again.
	sw = findSwitch()
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	tt.Frame()
	if !tt.HasText("running") {
		t.Fatalf("expected running status text after enable; texts=%v", tt.Texts())
	}
	rst = findRestart()
	if rst.Disabled {
		t.Fatal("expected Restart button re-enabled when plugin is enabled")
	}
	tt.ClickAt(rst.Bounds.X+rst.Bounds.W/2, rst.Bounds.Y+rst.Bounds.H/2)
	tt.Frame()
	if len(pv.restartCalls) != 2 {
		t.Fatalf("expected Restart to work after re-enabling; restartCalls=%v", pv.restartCalls)
	}
}

func TestPluginsMarketplaceRenderConfirmAndInstall(t *testing.T) {
	pv := &recordingPluginsApp{
		market: MarketState{
			Plugins: []market.Plugin{
				{
					ID:          "org.example.newpkg",
					Name:        "New Pack",
					Author:      "Hyla",
					Description: "Fresh marketplace plugin",
					Homepage:    "https://github.com/smallnest/hyla",
					Version:     "1.2.0",
					Permissions: []string{"ui.webview", "session.read"},
					DownloadURL: "https://example.com/new.hylapkg",
				},
				{
					ID:          "org.example.uptodate",
					Name:        "Up To Date",
					Author:      "Hyla",
					Version:     "1.0.0",
					DownloadURL: "https://example.com/cur.hylapkg",
				},
				{
					ID:          "org.example.outdated",
					Name:        "Outdated",
					Author:      "Hyla",
					Version:     "2.0.0",
					DownloadURL: "https://example.com/up.hylapkg",
				},
			},
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
	}, 760, 900)
	tt.Frame()

	if !tt.HasText("Plugin Marketplace") || !tt.HasText("New Pack · v1.2.0") || !tt.HasText("No plugins installed") {
		t.Fatalf("expected Marketplace even with zero installed plugins; texts=%v", tt.Texts())
	}

	// Click Refresh button.
	if err := tt.Click("Refresh"); err != nil {
		t.Fatalf("click Refresh: %v", err)
	}
	tt.Frame()
	if pv.refreshCnt != 1 {
		t.Fatalf("expected refreshCnt=1, got %d", pv.refreshCnt)
	}

	// Click Project button opens homepage.
	if err := tt.Click("Project"); err != nil {
		t.Fatalf("click Project: %v", err)
	}
	tt.Frame()
	if len(pv.openedURLs) != 1 || pv.openedURLs[0] != "https://github.com/smallnest/hyla" {
		t.Fatalf("openedURLs = %v", pv.openedURLs)
	}

	// Click Install -> sets confirmation dialog listing declared permissions before installing.
	if err := tt.Click("Install"); err != nil {
		t.Fatalf("click Install: %v", err)
	}
	tt.Frame()
	if len(pv.installMkt) != 0 {
		t.Fatalf("expected install not to run before confirmation")
	}
	if app.ss.Confirm == nil || !app.ss.Confirm.Open ||
		!strings.Contains(app.ss.Confirm.Title, "New Pack") ||
		!strings.Contains(app.ss.Confirm.Detail, "ui.webview") ||
		!strings.Contains(app.ss.Confirm.Detail, "session.read") {
		t.Fatalf("unexpected confirmation ask: %+v", app.ss.Confirm)
	}
	app.ss.Confirm.Run()
	if len(pv.installMkt) != 1 || pv.installMkt[0].ID != "org.example.newpkg" {
		t.Fatalf("installMkt after confirm = %+v", pv.installMkt)
	}

	// Now simulate installed plugins: "uptodate" (1.0.0) and "outdated" (1.0.0 -> 2.0.0 available).
	pv.plugins = []manifest.InstalledPlugin{
		{
			Entry:    manifest.RegistryEntry{ID: "org.example.uptodate", Enabled: true, Version: "1.0.0"},
			Manifest: &manifest.Manifest{ID: "org.example.uptodate", Name: "Up To Date", Version: "1.0.0"},
		},
		{
			Entry:    manifest.RegistryEntry{ID: "org.example.outdated", Enabled: true, Version: "1.0.0"},
			Manifest: &manifest.Manifest{ID: "org.example.outdated", Name: "Outdated", Version: "1.0.0"},
		},
	}
	tt.Frame()

	var foundInstalledDisabled, foundUpdateEnabled bool
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "Installed" && a.Disabled {
			foundInstalledDisabled = true
		}
		if a.Role == ui.RoleButton && a.Name == "Update" && !a.Disabled {
			foundUpdateEnabled = true
		}
	}
	if !foundInstalledDisabled || !foundUpdateEnabled {
		t.Fatalf("expected disabled Installed & enabled Update buttons; installed=%v update=%v access=%+v",
			foundInstalledDisabled, foundUpdateEnabled, tt.AccessAll())
	}
}
