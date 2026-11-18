package palette

import (
	"slices"
	"testing"

	"hyla/internal/actions"
	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/settings"
)

type stubPaletteApp struct {
	st      settings.Settings
	session *proto.Session
}

func (a *stubPaletteApp) Settings() *settings.Settings                 { return &a.st }
func (a *stubPaletteApp) Projects() []proto.Project                    { return nil }
func (a *stubPaletteApp) Groups() [][]*proto.Session                   { return nil }
func (a *stubPaletteApp) HostOf(string) *host.Host                     { return &host.Host{} }
func (a *stubPaletteApp) Local() *host.Host                            { return &host.Host{} }
func (a *stubPaletteApp) Shown() string                                { return a.session.ID }
func (a *stubPaletteApp) Session(id string) *proto.Session             { return a.session }
func (a *stubPaletteApp) Collapsed() map[string]bool                   { return map[string]bool{} }
func (a *stubPaletteApp) Update(fn func())                             { fn() }
func (a *stubPaletteApp) AllHosts() []*host.Host                       { return nil }
func (a *stubPaletteApp) Pages() []Page                                { return nil }
func (a *stubPaletteApp) ActionTitle(id string) string                 { act, _ := actions.Find(id); return act.Title }
func (a *stubPaletteApp) Run(string)                                   {}
func (a *stubPaletteApp) OpenSettings(string)                          {}
func (a *stubPaletteApp) NewSession(string)                            {}
func (a *stubPaletteApp) SelectSession(string)                         {}
func (a *stubPaletteApp) ResumeConversation(*host.Host, proto.Conversation) {}
func (a *stubPaletteApp) RestartDaemon()                               {}
func (a *stubPaletteApp) FocusTerminal()                               {}

func TestPaletteHonorsNoPaletteAndWhen(t *testing.T) {
	t.Cleanup(actions.ClearPluginActions)
	actions.SetPluginActions("ex", []actions.Action{
		{ID: "plugin.ex.visible", Title: "Visible Action", PluginID: "ex"},
		{ID: "plugin.ex.hidden", Title: "Hidden Action", PluginID: "ex", NoPalette: true},
		{ID: "plugin.ex.claude", Title: "Claude Action", PluginID: "ex", When: "agent:claude"},
		{ID: "plugin.ex.codex", Title: "Codex Action", PluginID: "ex", When: "agent:codex"},
	})
	app := &stubPaletteApp{
		session: &proto.Session{
			ID:    "s1",
			Focus: "p1",
			Panes: []proto.Pane{{ID: "p1", Agent: "claude"}},
		},
	}
	titles := Titles(&State{Open: true}, app)
	if !slices.Contains(titles, "Visible Action") {
		t.Fatalf("expected Visible Action in palette titles, got %v", titles)
	}
	if slices.Contains(titles, "Hidden Action") {
		t.Fatalf("NoPalette action must not appear in palette: %v", titles)
	}
	if !slices.Contains(titles, "Claude Action") || slices.Contains(titles, "Codex Action") {
		t.Fatalf("When clause mismatch in palette titles: %v", titles)
	}
}
