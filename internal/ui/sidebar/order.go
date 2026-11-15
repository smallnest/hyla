// Package sidebar holds the sessions the sidebar lists: the groups of a
// project's sessions, the order they and their projects are in, and the
// order the user sees them in.
package sidebar

import (
	"slices"
	"sort"
	"time"

	"hyla/internal/app/host"
	"hyla/internal/proto"

	"hyla/locales"
)

// Prefs are the preferences the order reads and keeps, as the app holds
// them.
type Prefs struct {
	// Collapsed are the projects whose sessions the sidebar hides.
	Collapsed map[string]bool
	// ProjectOrder and SessionOrder are the ids as the user dragged them.
	ProjectOrder []string
	SessionOrder []string
	// SidebarGroup is how the sidebar lists the sessions: by project,
	// by none, or by date, and SidebarOrder whether the ones created
	// first or updated last come first.
	SidebarGroup string
	SidebarOrder string
	// SetProjectOrder and SetSessionOrder make an order the app's, which
	// Save keeps.
	SetProjectOrder func(order []string)
	SetSessionOrder func(order []string)
	// Save keeps the preferences.
	Save func()
}

// App is what the sidebar's order asks of the app.
type App interface {
	// MachinesNow are the machines the app holds, this machine's first,
	// and HostOf the machine a session or a project is of.
	MachinesNow() []*host.Host
	HostOf(id string) *host.Host
	// SessionsNow and ProjectsNow are the state the app shows, and
	// Session one of its sessions.
	SessionsNow() []proto.Session
	ProjectsNow() []proto.Project
	Session(id string) *proto.Session
	// PrefsNow are the preferences the order keeps.
	PrefsNow() Prefs
	// SelectedID is the session the window shows.
	SelectedID() string
}

// Group is a project with its sessions, as the sidebar lists them.
type Group struct {
	// Host is the machine the project runs on, and Project it.
	Host    *host.Host
	Project proto.Project
	// Sessions are the ids of the project's sessions, as the sidebar
	// lists them.
	Sessions []string
	// First: the first of its host's in the sidebar, whose header tells
	// how the app stands with the host.
	First bool
}

// Key is what the group's collapse is remembered by: its project's id.
func (g Group) Key() string { return g.Project.ID }

// DragKey is what a session drag stays within: the group's key, which the
// sessions of no project have none of, so it takes their host's name.
func (g Group) DragKey() string {
	if g.Project.ID == "" {
		return "other:" + g.Host.Name
	}
	return g.Project.ID
}

// Groups returns the projects of every host, as the sidebar lists them.
func Groups(a App) []Group {
	var gs []Group
	for _, h := range a.MachinesNow() {
		gs = append(gs, HostGroups(a, h)...)
	}
	return gs
}

// HostGroups returns a host's projects, with their sessions.
func HostGroups(a App, h *host.Host) []Group {
	var projects []proto.Project
	for _, p := range a.ProjectsNow() {
		if a.HostOf(p.ID) == h {
			projects = append(projects, p)
		}
	}
	OrderProjects(a, projects)
	var gs []Group
	known := map[string]bool{}
	for _, p := range projects {
		known[p.ID] = true
		if p.Default {
			p.Name = locales.DefaultProject()
		}
		g := Group{Host: h, Project: p}
		for i := range a.SessionsNow() {
			if a.SessionsNow()[i].Project == p.ID {
				g.Sessions = append(g.Sessions, a.SessionsNow()[i].ID)
			}
		}
		SortSessions(a, g.Sessions)
		OrderSessions(a, g.Sessions)
		gs = append(gs, g)
	}
	other := Group{Host: h, Project: proto.Project{ID: "", Name: locales.SidebarOtherProject()}}
	for i := range a.SessionsNow() {
		if s := &a.SessionsNow()[i]; !known[s.Project] && a.HostOf(s.ID) == h {
			other.Sessions = append(other.Sessions, s.ID)
		}
	}
	if len(other.Sessions) > 0 {
		SortSessions(a, other.Sessions)
		OrderSessions(a, other.Sessions)
		gs = append(gs, other)
	}
	return gs
}

// OrderProjects sorts ps as the user dragged them, by the ids in
// ProjectOrder; projects it does not name keep the daemon's order, after
// them.
func OrderProjects(a App, ps []proto.Project) {
	// The default project comes first, whatever the order says.
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].Default && !ps[j].Default })
	if len(a.PrefsNow().ProjectOrder) == 0 {
		return
	}
	at := map[string]int{}
	for i, id := range a.PrefsNow().ProjectOrder {
		if _, ok := at[id]; !ok {
			at[id] = i
		}
	}
	// Within what the drag order leaves, the default project stays first.
	sort.SliceStable(ps, func(i, j int) bool {
		ii, iok := at[ps[i].ID]
		jj, jok := at[ps[j].ID]
		switch {
		case ps[i].Default != ps[j].Default:
			return ps[i].Default
		case iok && jok:
			return ii < jj
		case iok:
			return true
		case jok:
			return false
		}
		return false
	})
}

// OrderSessions puts a project's sessions in the order the user dragged
// them into, by the ids in SessionOrder; the sessions it does not name
// follow them, in the order the sidebar's Order By gave them, as the
// projects ProjectOrder does not name follow the ones it does.
func OrderSessions(a App, ss []string) {
	if len(a.PrefsNow().SessionOrder) == 0 {
		return
	}
	at := map[string]int{}
	for i, id := range a.PrefsNow().SessionOrder {
		if _, ok := at[id]; !ok {
			at[id] = i
		}
	}
	sort.SliceStable(ss, func(i, j int) bool {
		ii, iok := at[ss[i]]
		jj, jok := at[ss[j]]
		switch {
		case iok && jok:
			return ii < jj
		case iok:
			return true
		case jok:
			return false
		}
		return false
	})
}

// ProjectDrag is a project dragged by its name to another place under the
// same host.
type ProjectDrag struct {
	Host *host.Host
	ID   string
}

// MoveProject puts a project drag before or after target, under its
// host, and keeps the order in the app's preferences.
func MoveProject(a App, d ProjectDrag, target string, after bool) {
	if d.ID == "" || target == "" || d.ID == target {
		return
	}
	var order []string
	for _, g := range HostGroups(a, d.Host) {
		if g.Project.ID != "" {
			order = append(order, g.Project.ID)
		}
	}
	i := slices.Index(order, d.ID)
	j := slices.Index(order, target)
	if i < 0 || j < 0 {
		return
	}
	order = slices.Delete(order, i, i+1)
	j = slices.Index(order, target)
	if after {
		j++
	}
	order = slices.Insert(order, j, d.ID)
	// Replace this host's ids in the order, leaving the other hosts' as
	// they are: only their order among themselves matters.
	hostIDs := map[string]bool{}
	for _, g := range HostGroups(a, d.Host) {
		if g.Project.ID != "" {
			hostIDs[g.Project.ID] = true
		}
	}
	prefs := a.PrefsNow()
	prefs.SetProjectOrder(mergeOrder(prefs.ProjectOrder, hostIDs, order))
	prefs.Save()
}

// SessionDrag is a session dragged by its row to another place in its
// project's list, which group says by its dragKey.
type SessionDrag struct {
	// Group is the drag key of the group the session comes from, and ID
	// the session.
	Group string
	ID    string
}

// ReorderSession puts a session drag before or after target, in the list of
// the group it comes from, and keeps the order in the app's preferences.
func ReorderSession(a App, d SessionDrag, target string, after bool) {
	if d.ID == "" || target == "" || d.ID == target || d.Group == "" {
		return
	}
	var sessions []string
	for _, g := range Groups(a) {
		if g.DragKey() == d.Group {
			sessions = g.Sessions
			// As the user sees them, whatever order the drag order gave
			// them.
			order := slices.Clone(sessions)
			SortSessions(a, order)
			OrderSessions(a, order)
			sessions = order
			break
		}
	}
	order := slices.Clone(sessions)
	i := slices.Index(order, d.ID)
	if i < 0 || !slices.Contains(order, target) {
		return
	}
	order = slices.Delete(order, i, i+1)
	j := slices.Index(order, target)
	if after {
		j++
	}
	order = slices.Insert(order, j, d.ID)
	// Replace these sessions' ids in the order, leaving the others' as
	// they are: only their order among themselves matters.
	ids := map[string]bool{}
	for _, id := range sessions {
		ids[id] = true
	}
	prefs := a.PrefsNow()
	prefs.SetSessionOrder(mergeOrder(prefs.SessionOrder, ids, order))
	prefs.Save()
}

// mergeOrder puts the ids of a list as the user dragged them, order, in
// the order of every list, which keeps those ids of the other lists as
// they are: only their order among themselves matters, not where they
// sit among another list's. ids are the list's.
func mergeOrder(was []string, ids map[string]bool, order []string) []string {
	out := make([]string, 0, len(was)+len(order))
	for _, id := range was {
		if !ids[id] {
			out = append(out, id)
		}
	}
	return append(out, order...)
}

// SortSessions orders sessions as the sidebar's Order says: the ones
// created first, or the ones updated last.
func SortSessions(a App, ss []string) {
	order := a.PrefsNow().SidebarOrder
	if order == "" {
		order = "created"
	}
	at := func(id string) *proto.Session {
		if s := a.Session(id); s != nil {
			return s
		}
		return &proto.Session{}
	}
	sort.SliceStable(ss, func(i, j int) bool {
		si, sj := at(ss[i]), at(ss[j])
		if order == "updated" {
			return SessionUpdated(si) > SessionUpdated(sj)
		}
		return si.Created < sj.Created
	})
}

// SessionUpdated is when a session last did something, for the updated
// order: its program's last write, else its status's since, else its
// creation.
func SessionUpdated(s *proto.Session) int64 {
	if s.Activity > 0 {
		return s.Activity
	}
	if s.Since > 0 {
		return s.Since
	}
	return s.Created
}

// FlatSessions are a host's sessions, ordered, for a sidebar that does
// not group them by project.
func FlatSessions(a App, h *host.Host) []string {
	var out []string
	for i := range a.SessionsNow() {
		if a.HostOf(a.SessionsNow()[i].ID) == h {
			out = append(out, a.SessionsNow()[i].ID)
		}
	}
	SortSessions(a, out)
	return out
}

// DateGroup is sessions created on one day, for the sidebar's By Date.
type DateGroup struct {
	// Day is the day, and Sessions the ids of its sessions.
	Day      time.Time
	Sessions []string
}

// DateGroups groups a host's sessions by the day they were created, the
// newest day first, with each day's sessions in the chosen order.
func DateGroups(a App, h *host.Host) []DateGroup {
	var all []string
	for i := range a.SessionsNow() {
		if a.HostOf(a.SessionsNow()[i].ID) == h {
			all = append(all, a.SessionsNow()[i].ID)
		}
	}
	created := func(id string) int64 {
		if s := a.Session(id); s != nil {
			return s.Created
		}
		return 0
	}
	sort.SliceStable(all, func(i, j int) bool { return created(all[i]) > created(all[j]) })
	var gs []DateGroup
	for _, id := range all {
		d := time.UnixMilli(created(id))
		d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
		if len(gs) == 0 || !gs[len(gs)-1].Day.Equal(d) {
			gs = append(gs, DateGroup{Day: d})
		}
		gs[len(gs)-1].Sessions = append(gs[len(gs)-1].Sessions, id)
	}
	for i := range gs {
		SortSessions(a, gs[i].Sessions)
	}
	return gs
}

// DateLabel names a day in the sidebar's By Date: today, yesterday, or
// the date.
func DateLabel(day, now time.Time) string {
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	switch {
	case day.Equal(today):
		return locales.SidebarToday()
	case day.Equal(today.AddDate(0, 0, -1)):
		return locales.SidebarYesterday()
	}
	return day.Format("2006-01-02")
}

// ProjectName returns a project's name, or "".
func ProjectName(a App, id string) string {
	for _, p := range a.ProjectsNow() {
		if p.ID == id {
			return p.Name
		}
	}
	return ""
}

// Collapsed reports whether a group's sessions are hidden in the sidebar.
func Collapsed(a App, g Group) bool { return a.PrefsNow().Collapsed[g.Key()] }

// VisibleOrder returns the sessions in the sidebar's order, those of
// collapsed projects left out unless selected.
func VisibleOrder(a App) []string {
	var ids []string
	switch a.PrefsNow().SidebarGroup {
	case "none":
		for _, h := range a.MachinesNow() {
			ids = append(ids, FlatSessions(a, h)...)
		}
		return ids
	case "date":
		for _, h := range a.MachinesNow() {
			for _, dg := range DateGroups(a, h) {
				ids = append(ids, dg.Sessions...)
			}
		}
		return ids
	}
	for _, g := range Groups(a) {
		for _, id := range g.Sessions {
			if !Collapsed(a, g) || id == a.SelectedID() {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
