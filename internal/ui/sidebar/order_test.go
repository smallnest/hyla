package sidebar

import (
	"slices"
	"testing"

	"hyla/internal/app/host"
	"hyla/internal/proto"
)

// fakeApp is the app the order asks: its sessions, its projects, and what
// the user moved.
type fakeApp struct {
	sessions []proto.Session
	projects []proto.Project
	prefs    Prefs
	moved    []string // the sessions MoveSession moved, and to where they
	saved    int      // how often Save kept the preferences
}

var local = &host.Host{}

func (f *fakeApp) MachinesNow() []*host.Host    { return []*host.Host{local} }
func (f *fakeApp) HostOf(string) *host.Host     { return local }
func (f *fakeApp) SessionsNow() []proto.Session { return f.sessions }
func (f *fakeApp) ProjectsNow() []proto.Project { return f.projects }
func (f *fakeApp) Session(id string) *proto.Session {
	for i := range f.sessions {
		if f.sessions[i].ID == id {
			return &f.sessions[i]
		}
	}
	return nil
}
func (f *fakeApp) PrefsNow() Prefs    { return f.prefs }
func (f *fakeApp) SelectedID() string { return "" }
func (f *fakeApp) MoveSession(id, project string) {
	f.moved = append(f.moved, id+"->"+project)
	for i := range f.sessions {
		if f.sessions[i].ID == id {
			f.sessions[i].Project = project
		}
	}
}

func sideApp() *fakeApp {
	f := &fakeApp{
		projects: []proto.Project{{ID: "a", Name: "a"}, {ID: "b", Name: "b"}, {ID: "ssh", Name: "ssh"}, {ID: "k8s", Name: "k8s"}},
		sessions: []proto.Session{
			{ID: "a1", Project: "a"}, {ID: "a2", Project: "a"}, {ID: "a3", Project: "a"},
			{ID: "b1", Project: "b"},
			{ID: "s1", Project: "ssh"},
		},
	}
	f.prefs.HiddenProjects = map[string]bool{"ssh": true, "k8s": true}
	f.prefs.SessionProjects = map[string]string{}
	f.prefs.SetSessionProjects = func(m map[string]string) { f.prefs.SessionProjects = m }
	f.prefs.Save = func() { f.saved++ }
	return f
}

// byCreatedSorts makes the sidebar list the sessions in the order they
// were made, as their ids name it, and the ones that moved last, as the
// daemon appends them.
func byCreatedSorts(t *testing.T, f *fakeApp) {
	t.Helper()
	f.prefs.SessionOrder = []string{"a2", "a3", "b1", "s1"}
	f.prefs.SetSessionOrder = func(o []string) { f.prefs.SessionOrder = slices.Clone(o) }
}

// groupOf is a project's sessions, as the sidebar lists them.
func groupOf(f *fakeApp, id string) []string {
	for _, g := range Groups(f) {
		if g.Project.ID == id {
			return g.Sessions
		}
	}
	return nil
}

// TestMoveSessionTo checks that a session moves to another project, at a
// place there, and that it joins that project's list.
func TestMoveSessionTo(t *testing.T) {
	f := sideApp()
	byCreatedSorts(t, f)
	MoveSessionTo(f, "a3", "b", "b1", true)
	if got := f.moved; !slices.Equal(got, []string{"a3->b"}) {
		t.Fatalf("moved %v, want a3 to b", got)
	}
	if got := groupOf(f, "b"); !slices.Equal(got, []string{"b1", "a3"}) {
		t.Errorf("b's sessions = %v, want a3 after b1", got)
	}
	if got := groupOf(f, "a"); slices.Contains(got, "a3") {
		t.Errorf("a's sessions = %v, want a3 gone", got)
	}
	if f.saved != 1 {
		t.Errorf("saved %d times, want once", f.saved)
	}
}

// TestMoveSessionToAppends checks a move to a project alone: the session
// goes last there.
func TestMoveSessionToAppends(t *testing.T) {
	f := sideApp()
	byCreatedSorts(t, f)
	MoveSessionTo(f, "a1", "b", "", false)
	if got := groupOf(f, "b"); !slices.Equal(got, []string{"b1", "a1"}) {
		t.Errorf("b's sessions = %v, want a1 last", got)
	}
}

// TestMoveSessionToRefuses checks that no session moves to or from the
// SSH and K8s projects, or to the project it is in.
func TestMoveSessionToRefuses(t *testing.T) {
	f := sideApp()
	MoveSessionTo(f, "a1", "ssh", "", false)
	MoveSessionTo(f, "s1", "a", "", false)
	MoveSessionTo(f, "a1", "a", "", false)
	byCreatedSorts(t, f)
	MoveSessionTo(f, "a1", "a", "a2", false)
	if len(f.moved) != 0 {
		t.Errorf("moved %v, want none", f.moved)
	}
	if _, _, ok := SessionProjectsOf(f.prefs.SessionProjects, "a1"); ok {
		t.Error("a session that did not move has a place")
	}
	if f.saved != 0 {
		t.Error("a move to a place the session does not have saved the preferences")
	}
}

// TestJoinSessions checks that the sessions the user moved into a project
// sit where they left them, though the daemon lists them last.
func TestJoinSessions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ids   []string
		moved map[string]string
		want  []string
	}{
		{"after one", []string{"b1", "m1", "m2"}, map[string]string{"m1": "b1\x00m2", "m2": "m1\x00"}, []string{"b1", "m1", "m2"}},
		{"before one", []string{"b1", "b2", "m1"}, map[string]string{"m1": "\x00b1"}, []string{"m1", "b1", "b2"}},
		{"gone", []string{"b1", "m1"}, map[string]string{"m1": "nope\x00"}, []string{"b1", "m1"}},
		{"none moved", []string{"b1", "b2"}, nil, []string{"b1", "b2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := sideApp()
			f.prefs.SessionProjects = tc.moved
			ids := slices.Clone(tc.ids)
			JoinSessions(f, ids)
			if !slices.Equal(ids, tc.want) {
				t.Errorf("sessions = %v, want %v", ids, tc.want)
			}
		})
	}
}
