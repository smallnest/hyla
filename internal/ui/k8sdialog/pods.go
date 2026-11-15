package k8sdialog

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/anmitsu/go-shlex"
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/marks"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Pod is a pod the sidebar's K8s project lists: its name, the kubectl
// command that enters it, and the terminal it has open, if any. The app
// keeps the pods in its prefs.
type Pod struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Session string `json:"session,omitempty"`
}

var (
	// ErrNotExec is a command that is not a kubectl exec.
	ErrNotExec = errors.New("k8s: not a kubectl exec")
	// ErrNoPod is a kubectl exec that names no pod.
	ErrNoPod = errors.New("k8s: the kubectl exec names no pod")
)

// Drag is a pod dragged by its row to another place in the K8s project.
type Drag struct {
	Command string
}

// App is what the pods of the K8s project ask of the app, and the dialog
// that adds one.
type App interface {
	// Settings are the settings the rows follow.
	Settings() *settings.Settings
	// PrefsK8sProject is the project the pods' terminals open in, and
	// PrefsK8sPods the pods; SavePrefs keeps them.
	PrefsK8sProject() string
	SetPrefsK8sProject(id string)
	PrefsK8sPods() []Pod
	SetPrefsK8sPods(ps []Pod)
	SavePrefs()
	// Local is this machine, Session and ProjectNow find a session and a
	// project the app knows, and SelectSession shows and CloseSession
	// closes one.
	Local() *host.Host
	Session(id string) *proto.Session
	ProjectNow(id string) *proto.Project
	SelectSession(id string)
	CloseSession(id string, ask bool)
	// AddProject shows a project the app's state does not have yet, and
	// AddPendingK8s holds its first terminal; OpenProject unfolds it.
	AddProject(p proto.Project)
	AddPendingK8s(s proto.Session)
	OpenProject(id string)
	// Main runs fn on the main thread, RunLater runs fn after the frame,
	// Toast tells a failure, and UndoSays says what a removal took away,
	// with an undo.
	Main(fn func())
	RunLater(fn func())
	Toast(msg string)
	UndoSays(msg string, undo func())
	// Prompt is the dialog that adds a pod, which SetPrompt opens and
	// closes, and FocusTerminal takes the keyboard back as it closes.
	K8sPrompt() *Ask
	SetK8sPrompt(d *Ask)
	FocusTerminal()
	// CopyAddress copies a pod's command, and ConfirmNow asks the app's
	// confirmation.
	CopyAddress(cmd string)
	ConfirmNow(q *confirm.Ask)
	// DragAt is the pod an insertion line last showed, and DragAfter the
	// side of it; SetDragAt keeps them.
	DragAt() string
	DragAfter() bool
	SetDragAt(cmd string, after bool)
	// SelectedSession is the session the window shows, and SidebarRows
	// the kind of row the sidebar draws.
	SelectedSession() *proto.Session
	SidebarRows() string
}

// Show draws the dialog while it is open, and opens the terminal once the
// user says so.
func Show(a App, c *ui.Context, pal *theme.Palette) {
	d := a.K8sPrompt()
	if View(c, pal, d) {
		Add(a)
	}
	if d != nil && !d.Open {
		a.SetK8sPrompt(nil)
		a.FocusTerminal()
	}
}

// Name is what the sidebar's K8s project is called, in the language the
// app shows as it is made.
func Name() string { return locales.K8sPodsProject() }

// Project is the sidebar's K8s project, nil while there is none.
func Project(a App) *proto.Project {
	if a.PrefsK8sProject() == "" {
		return nil
	}
	return a.ProjectNow(a.PrefsK8sProject())
}

// Add adds the pod the dialog's command enters to the K8s project, which
// it makes if there is none, and opens a terminal that enters it.
func Add(a App) {
	d := a.K8sPrompt()
	if d == nil {
		return
	}
	cmd := strings.TrimSpace(d.Cmd)
	if cmd == "" {
		d.Err = locales.K8sPodOnly()
		return
	}
	_, pod, err := Command(cmd)
	switch {
	case errors.Is(err, ErrNoPod):
		d.Err = locales.K8sPodNeedsPod()
		return
	case err != nil:
		d.Err = locales.K8sPodOnly()
		return
	}
	if a.Local().Client == nil {
		d.Err = locales.SshNoService()
		return
	}
	// The command the project lists already: its terminal comes, rather
	// than a second row for the same pod.
	for _, p := range a.PrefsK8sPods() {
		if p.Command == cmd {
			d.Open = false
			showOrStart(a, p)
			return
		}
	}
	d.Open = false
	withProject(a, func(p proto.Project) { addTo(a, p, pod, cmd) })
}

// withProject runs fn with the sidebar's K8s project, making it first if
// there is none, off the UI thread as the daemon call is.
func withProject(a App, fn func(proto.Project)) {
	if p := Project(a); p != nil {
		fn(*p)
		return
	}
	h := a.Local()
	// Its state has not come: the project may be one the app cannot see
	// yet, and a second one would follow it.
	if h.Client == nil || !h.Loaded {
		a.Toast(locales.SshNoService())
		return
	}
	cl := h.Client
	go func() {
		p, err := makeProject(cl, Name())
		a.Main(func() {
			if err != nil {
				a.Toast(err.Error())
				return
			}
			a.SetPrefsK8sProject(p.ID)
			a.SavePrefs()
			// The project shows as its first terminal comes, before the
			// daemon's state, which has it, does.
			if a.ProjectNow(p.ID) == nil {
				a.AddProject(p)
			}
			fn(p)
		})
	}()
}

// makeProject makes the sidebar's K8s project: one of this machine's,
// named for the sidebar, with no directory of its own.
func makeProject(cl *client.Client, name string) (proto.Project, error) {
	var p proto.Project
	err := cl.Call("project.create", proto.ProjectParams{Name: name}, &p)
	return p, err
}

// addTo adds a pod to the K8s project and opens a terminal that enters it.
func addTo(a App, p proto.Project, name, cmd string) {
	a.SetPrefsK8sPods(append(a.PrefsK8sPods(), Pod{Name: name, Command: cmd}))
	a.SavePrefs()
	showOrStart(a, Pod{Name: name, Command: cmd})
}

// showOrStart shows the terminal a pod's row has, if it still runs, else
// starts one: a row's click.
func showOrStart(a App, pod Pod) {
	if s := a.Session(pod.Session); s != nil && s.Status != proto.Exited {
		a.SelectSession(s.ID)
		return
	}
	Start(a, pod)
}

// ConnectTo connects to a pod again, in the row it was read from, whatever
// its terminal does: one still open closes, and the new terminal takes its
// place, so that the command finds the row once it comes.
func ConnectTo(a App, pod Pod) {
	if s := a.Session(pod.Session); s != nil {
		a.CloseSession(s.ID, false)
	}
	Start(a, pod)
}

// Start starts a pod's terminal, in the K8s project.
func Start(a App, pod Pod) {
	args, _, err := Command(pod.Command)
	if err != nil {
		return
	}
	h := a.Local()
	if h.Client == nil {
		a.Toast(locales.SshNoService())
		return
	}
	withProject(a, func(p proto.Project) {
		cl := h.Client
		go func() {
			var s proto.Session
			err := cl.Call("create", proto.CreateParams{Project: p.ID, Dir: proto.Home(), Name: pod.Name, Command: args}, &s)
			a.Main(func() {
				if err != nil {
					a.Toast(err.Error())
					return
				}
				// The row may have gone as the terminal came, and another
				// may have taken its place: the command finds it.
				ps := a.PrefsK8sPods()
				i := slices.IndexFunc(ps, func(p Pod) bool { return p.Command == pod.Command })
				if i < 0 {
					return
				}
				ps[i].Session = s.ID
				a.SetPrefsK8sPods(ps)
				a.SavePrefs()
				a.OpenProject(p.ID)
				a.AddPendingK8s(s)
				a.SelectSession(s.ID)
			})
		}()
	})
}

// OfSession is the pod a session is the terminal of, nil for none.
func OfSession(a App, id string) *Pod {
	if id == "" {
		return nil
	}
	ps := a.PrefsK8sPods()
	for i := range ps {
		if ps[i].Session == id {
			return &ps[i]
		}
	}
	return nil
}

// Move puts the pod of the command from where the pod of the command to
// is, before or after it, and keeps the order in the app's preferences.
func Move(a App, from, to string, after bool) {
	at := func(cmd string) int {
		return slices.IndexFunc(a.PrefsK8sPods(), func(p Pod) bool { return p.Command == cmd })
	}
	i, j := at(from), at(to)
	if from == "" || from == to || i < 0 || j < 0 {
		return
	}
	ps := a.PrefsK8sPods()
	pod := ps[i]
	ps = slices.Delete(ps, i, i+1)
	j = slices.IndexFunc(ps, func(p Pod) bool { return p.Command == to })
	if j < 0 {
		return
	}
	if after {
		j++
	}
	ps = slices.Insert(ps, j, pod)
	a.SetPrefsK8sPods(ps)
	a.SavePrefs()
}

// Remove removes a pod from the K8s project, ending its terminal as
// closing one does.
func Remove(a App, cmd string) {
	ps := a.PrefsK8sPods()
	for i, pod := range ps {
		if pod.Command != cmd {
			continue
		}
		ps = slices.Delete(ps, i, i+1)
		a.SetPrefsK8sPods(ps)
		a.SavePrefs()
		a.UndoSays(locales.K8sPodRemoved(pod.Name), func() {
			ps := a.PrefsK8sPods()
			a.SetPrefsK8sPods(slices.Insert(ps, min(i, len(ps)), pod))
			a.SavePrefs()
		})
		if pod.Session != "" {
			// The question above said it goes.
			a.CloseSession(pod.Session, false)
		}
		return
	}
}

// Clear forgets the K8s project, whose terminals go with it.
func Clear(a App) {
	a.SetPrefsK8sProject("")
	a.SetPrefsK8sPods(nil)
	a.SavePrefs()
}

// Command reads a kubectl exec command, as the user typed it, and says the
// arguments to run and the pod it enters: kubectl exec -it web-0 -n prod
// -- /bin/bash enters web-0.
func Command(cmd string) ([]string, string, error) {
	args, err := shlex.Split(cmd, true)
	if err != nil || len(args) == 0 {
		return nil, "", ErrNotExec
	}
	if filepath.Base(args[0]) != "kubectl" {
		return nil, "", ErrNotExec
	}
	exec := -1
	for i := 1; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		if args[i] == "exec" {
			exec = i
			break
		}
	}
	if exec < 0 {
		return nil, "", ErrNotExec
	}
	// The pod is what follows exec, before --, past kubectl's flags: -n
	// and the like take a value of their own.
	for i := exec + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			if flagValue(arg) {
				i++
			}
			continue
		}
		return args, arg, nil
	}
	return nil, "", ErrNoPod
}

// flagValue reports whether a kubectl flag takes its value as the next
// argument, as -n prod does, unless it is written -n=prod.
func flagValue(flag string) bool {
	if strings.Contains(flag, "=") {
		return false
	}
	switch flag {
	case "-n", "--namespace", "-c", "--container", "--context", "--cluster", "--kubeconfig",
		"--user", "--as", "--as-group", "--as-uid", "--request-timeout", "-s", "--server",
		"--token", "--certificate-authority", "--client-certificate", "--client-key",
		"--cache-dir", "--tls-server-name", "--profile", "--profile-output", "-v":
		return true
	}
	return false
}

// Rows draws the pods of the K8s project, after a project's sessions, and
// the row that adds one where there is none yet.
func Rows(a App, c *ui.Context, pal *theme.Palette) {
	ps := a.PrefsK8sPods()
	for i := range ps {
		row(a, c, pal, i)
	}
	if len(ps) == 0 {
		empty(a, c, pal)
	}
}

// Row draws a pod in the K8s project: its name over the command that
// enters it, a click showing its terminal or starting one.
func Row(a App, c *ui.Context, pal *theme.Palette, i int) { row(a, c, pal, i) }

func row(a App, c *ui.Context, pal *theme.Palette, index int) {
	pod := a.PrefsK8sPods()[index]
	project := a.PrefsK8sProject()
	s := a.Session(pod.Session)
	if s != nil && s.Project != project {
		s = nil
	}
	sel := a.SelectedSession()
	selected := s != nil && sel != nil && s.ID == sel.ID
	inset := float32(RowInset)
	detailed := a.SidebarRows() != "compact"
	height := float32(RowH)
	if detailed {
		height = RowH2
	}
	r := ui.Row(c.Key("k8s-"+pod.Command)).Height(height).Margin(0, 0, 1, 0).Padding(0, 10, 0, inset).Gap(Gap).AlignItems(ui.Center).Radius(8)
	r.Border(1, ui.Transparent)
	bg := ui.Transparent
	switch {
	case selected:
		bg = theme.TranslucentGlass(pal.RowSelected, pal, *a.Settings())
		style.Chosen(r, pal)
	case r.Hovered():
		bg = theme.TranslucentGlass(pal.RowHover, pal, *a.Settings())
	}
	r.Background(r.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	// The row is its pod's handle: dragged, it moves the pod to another
	// place in the project, its terminal with it.
	r.Drag(Drag{Command: pod.Command}).Cursor(ui.CursorGrab)
	if r.Dragging() {
		r.Opacity(0.35)
	}
	if d, over := ui.DragOver[Drag](r); over && d.Command != pod.Command {
		_, y, _ := r.PointerPosition()
		after := y > height/2
		a.SetDragAt(pod.Command, after)
		r.Background(pal.RowHover)
		insertLine(r, pal, after)
	}
	if d, ok := ui.Drop[Drag](r); ok && d.Command != pod.Command {
		_, y, _ := r.PointerPosition()
		after := y > height/2
		if a.DragAt() == pod.Command {
			// The side the insertion line last showed.
			after = a.DragAfter()
		}
		to, from := pod.Command, d.Command
		a.RunLater(func() { Move(a, from, to, after) })
	}
	paints.PointerTooltip(c, r, "tip-k8s-"+pod.Command, pod.Command)
	r.ContextMenu(func(m *ui.Menu) {
		// Reconnecting goes on a terminal that ended, as kubectl's does
		// when the pod goes, and starts the terminal anew, or replaces one
		// still open: a click on the row shows that one. The row it read
		// is the pod's, whatever its terminal's end does to the list.
		if m.Item(locales.ActionReconnectHost()).Chosen() {
			ConnectTo(a, pod)
		}
		m.Separator()
		if m.Item(locales.K8sCopyCommand()).Chosen() {
			a.CopyAddress(pod.Command)
		}
		if m.Item(locales.HostRemove()).Chosen() {
			name, cmd := pod.Name, pod.Command
			a.ConfirmNow(&confirm.Ask{
				Open:    true,
				Title:   locales.K8sRemoveTitle(name),
				Detail:  locales.K8sRemoveDetail(),
				Action:  locales.CommonRemove(),
				Destroy: true,
				Run:     func() { Remove(a, cmd) },
			})
		}
	})
	if r.Clicked() && !selected {
		showOrStart(a, pod)
	}
	r.Children(func() {
		// The pod's mark stands on the row's first line, beside its name,
		// as an agent's stands beside a session's; the command below starts
		// under the name, past the mark.
		mark := func(gap float32) {
			markSlot(c, gap).Children(func() {
				ui.Icon(c, glyph.Terminal).Size(marks.Size, marks.Size).TextColor(pal.TextMuted)
			})
		}
		top := func() {
			ui.Text(c, pod.Name).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		}
		if !detailed {
			mark(Gap)
			top()
			ui.Spacer(c)
			ui.Text(c, pod.Command).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(2).MaxWidth(220)
		} else {
			// The name and the command over the row's width, as a detailed
			// session's row has its name and detail.
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					mark(Gap)
					top()
				})
				cmd := ui.Text(c, pod.Command).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").MinWidth(0)
				cmd.Padding(0, 0, 0, Badge+BadgeGap)
			})
		}
		if s == nil {
			// A pod with no terminal yet keeps the slot the status (or,
			// under the pointer, the close button) takes.
			ui.Box(c).Size(20, 20).Shrink(0)
			return
		}
		// Close ends the terminal, as Remove does, without its question.
		statusSlot(c, pal, r.Hovered(), s.Status, s.Unread, s.Progress, func() {
			cmd := pod.Command
			a.RunLater(func() { Remove(a, cmd) })
		})
	})
}

// empty is the K8s project with no pod in it yet: the row that adds one,
// in place of a project's sessions.
func empty(a App, c *ui.Context, pal *theme.Palette) {
	inset := float32(RowInset)
	r := ui.Row(c.Key("k8s-empty")).Height(RowH).Margin(0, 0, 1, 0).Padding(0, 10, 0, inset).Gap(Gap).AlignItems(ui.Center).Radius(8)
	if r.Hovered() {
		r.Background(pal.RowHover)
	}
	if r.Clicked() {
		a.SetK8sPrompt(&Ask{Open: true})
	}
	r.Children(func() {
		markSlot(c, Gap).Children(func() {
			ui.Icon(c, glyph.Plus).Size(13, 13).TextColor(pal.TextMuted)
		})
		ui.Text(c, locales.ActionAddK8sPod()).FontSize(13).TextColor(pal.TextMuted).SingleLine()
	})
}

// The K8s project's rows, as the sidebar draws them: a row's height, its
// inset and the gap between its parts, and the square before a project's
// name with the room after it.
const (
	RowH     = 34
	RowH2    = 48 // a detailed row, of two lines
	RowInset = 12
	Gap      = 10
	Badge    = 20
	BadgeGap = 6
)

// markSlot is the room a row keeps before its name: as wide as a project's
// badge and the gap after it, less the gap the row itself leaves, so that
// the names of a project's sessions and its pods start at one place.
func markSlot(c *ui.Context, gap float32) ui.Element {
	return ui.Box(c).Size(Badge+BadgeGap-gap, marks.Size).Center().Shrink(0)
}

// statusSlot ends a row that stands for a terminal: its status, or, while
// the pointer is on the row, a close button in the status's fixed slot, so
// that hovering moves no text. close ends what the row's terminal runs.
func statusSlot(c *ui.Context, pal *theme.Palette, hovered bool, st proto.Status, unread bool, progress int, close func()) {
	ui.Box(c.Key("trailing")).Size(20, 20).Shrink(0).Center().Children(func() {
		if !hovered {
			paints.StatusP(c, pal, st, unread, progress, 15)
			return
		}
		b := ui.ButtonBase(c.Key("close")).Size(20, 20).Radius(5).Center().TextColor(pal.TextMuted).
			Label(locales.ActionCloseSession()).Tooltip(locales.ActionCloseSession())
		if b.Hovered() {
			b.Background(pal.RowHover).TextColor(pal.Text)
		}
		b.Children(func() { ui.Icon(c, glyph.Close).Size(13, 13) })
		if b.Clicked() {
			close()
		}
	})
}

// insertLine draws the line that tells where a dragged row lands: along
// its top edge, or its bottom one to go after it.
func insertLine(r ui.Element, pal *theme.Palette, after bool) {
	r.Draw(func(p *ui.Painter, box ui.Rect) {
		y := box.Y
		if after {
			y = box.Y + box.H - 2
		}
		p.Fill(ui.Rect{X: box.X, Y: y, W: box.W, H: 2}, pal.Accent, 1)
	})
}
