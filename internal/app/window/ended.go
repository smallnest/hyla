package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/history"
	"hyla/internal/app/host"
	"hyla/internal/cli"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/theme"
)

// The app, as the sidebar's history asks and runs what it offers.
var _ history.App = (*App)(nil)

func (a *App) SyncEnded() { history.Sync(a) }

// loadHostConversations asks a host's hylad for the conversations it
// remembers, for the sidebar, as of gen.
func (a *App) loadHostConversations(h *host.Host, gen uint64) {
	history.LoadHostConversations(a, h, gen)
}

func (a *App) endedSections(c *ui.Context, pal *theme.Palette) { history.Sections(c, a, pal) }

func (a *App) CallOn(h *host.Host, method string, params any, done func(err error)) {
	a.callOn(h, method, params, done)
}

func (a *App) Ask(q history.Confirm) {
	a.Confirm = &confirm.Ask{
		Open: true, Title: q.Title, Detail: q.Detail, Action: q.Action, Destroy: q.Destroy, Run: q.Run,
	}
}

func (a *App) Copied() *string { return &a.CopyText }

func (a *App) SavePrefs() { a.savePrefs() }

func (a *App) SectionPrefs() *history.Prefs {
	return &history.Prefs{EndedOpen: &a.Prefs.EndedOpen, ArchivedOpen: &a.Prefs.ArchivedOpen}
}

func (a *App) ShowAll() *bool { return &a.EndedAll }

func (a *App) AllHosts() []*host.Host { return a.Machines }

func (a *App) Skills() []history.SkillState {
	states := cli.SkillStates()
	out := make([]history.SkillState, len(states))
	for i, st := range states {
		out[i] = history.SkillState{State: st.State}
	}
	return out
}

func (a *App) Skill() *history.Skill {
	return &history.Skill{Missing: a.SkillMissing, At: a.SkillsAt}
}
