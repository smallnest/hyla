package menus

import (
	"testing"

	"github.com/egoist/mygo"

	"hyla/internal/actions"
	"hyla/internal/settings"
)

type stubMenusApp struct {
	st settings.Settings
}

func (s *stubMenusApp) Settings() *settings.Settings         { return &s.st }
func (s *stubMenusApp) Recording() string                    { return "" }
func (s *stubMenusApp) RunID(id string, args map[string]string) {}
func (s *stubMenusApp) Update(fn func())                     { fn() }
func (s *stubMenusApp) SettingsWindow() *mygo.Window         { return nil }
func (s *stubMenusApp) CloseSettings()                       {}

func TestPluginMenuItemsExcludesContextMenuOnly(t *testing.T) {
	t.Cleanup(actions.ClearPluginActions)
	actions.SetPluginActions("com.example", []actions.Action{
		{ID: "plugin.com.example.menu", Title: "Top Level", Menu: "Plugins", PluginID: "com.example"},
		{ID: "plugin.com.example.ctx", Title: "Context Only", Menu: "Context", PluginID: "com.example"},
	})
	items := PluginMenuItems(&stubMenusApp{})
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	var hasMenu, hasCtx bool
	for _, id := range ids {
		if id == "plugin.com.example.menu" {
			hasMenu = true
		}
		if id == "plugin.com.example.ctx" {
			hasCtx = true
		}
	}
	if !hasMenu || hasCtx {
		t.Fatalf("PluginMenuItems ids=%v, want top-level included and Context excluded", ids)
	}
}
