package main

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/anmitsu/go-shlex"
	"github.com/egoist/mygo/ui"

	"hyla/internal/client"
	"hyla/internal/proto"

	"hyla/locales"
)

// k8sAsk is the dialog that adds a kubectl terminal to the sidebar: the
// command that enters a pod, as kubectl exec takes it.
type k8sAsk struct {
	open bool
	cmd  string
	err  string
}

// k8sPod is a pod the sidebar's K8s project lists: its name, the kubectl
// command that enters it, and the terminal it has open, if any.
type k8sPod struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Session string `json:"session,omitempty"`
}

var (
	// errK8sNotExec is a command that is not a kubectl exec.
	errK8sNotExec = errors.New("k8s: not a kubectl exec")
	// errK8sNoPod is a kubectl exec that names no pod.
	errK8sNoPod = errors.New("k8s: the kubectl exec names no pod")
)

// askK8sPod opens the dialog.
func (a *App) askK8sPod() {
	if a.k8sAsk == nil {
		a.k8sAsk = &k8sAsk{open: true}
	}
}

// k8sView draws the dialog while it is open, and opens the terminal once
// the user says so.
func (a *App) k8sView(c *ui.Context, pal *palette) {
	d := a.k8sAsk
	if d == nil {
		return
	}
	connect := false
	ui.DialogBase(c.Key("k8s"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.overlay)
		floating(panel.Width(520).Padding(22).Gap(8).Radius(14).Background(pal.popover), pal).Label(locales.ActionAddK8sPod())
		ui.Text(c, locales.ActionAddK8sPod()).FontSize(15).FontWeight(650).TextColor(pal.text)
		ui.Text(c, locales.K8sPodDetail()).FontSize(13).TextColor(pal.textMuted).LineHeight(1.4)
		ui.Row(c).Margin(8, 0, 1, 0).Children(func() {
			in := textField(c, "k8s-cmd", &d.cmd, locales.K8sPodCommandPlaceholder(), 0, pal).Font("monospace")
			in.AutoFocus()
			if in.Submitted() {
				connect = true
			}
		})
		if d.err != "" {
			ui.Text(c, d.err).FontSize(12).TextColor(pal.danger)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Margin(14, 0, 0, 0).Children(func() {
			if button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				d.open = false
			}
			if button(c, "ok", locales.CommonConnect(), "↩", true, pal) {
				connect = true
			}
		})
	})
	if connect {
		a.addK8sPod()
	}
	if !d.open {
		a.k8sAsk = nil
		a.focusTerminal()
	}
}

// k8sProjectName is what the sidebar's K8s project is called, in the
// language the app shows as it is made.
func k8sProjectName() string { return locales.K8sPodsProject() }

// k8sProjectOf is the sidebar's K8s project, nil while there is none.
func (a *App) k8sProjectOf() *proto.Project {
	if a.prefs.K8sProject == "" {
		return nil
	}
	return a.project(a.prefs.K8sProject)
}

// addK8sPod adds the pod the dialog names to the K8s project, which it
// makes if there is none, and opens a terminal that enters it.
func (a *App) addK8sPod() {
	d := a.k8sAsk
	if d == nil {
		return
	}
	cmd := strings.TrimSpace(d.cmd)
	if cmd == "" {
		d.err = locales.K8sPodOnly()
		return
	}
	_, pod, err := k8sCommand(cmd)
	switch {
	case errors.Is(err, errK8sNoPod):
		d.err = locales.K8sPodNeedsPod()
		return
	case err != nil:
		d.err = locales.K8sPodOnly()
		return
	}
	if a.local().cl == nil {
		d.err = locales.SshNoService()
		return
	}
	// The command the project lists already: its terminal comes, rather
	// than a second row for the same pod.
	for i := range a.prefs.K8sPods {
		if a.prefs.K8sPods[i].Command == cmd {
			d.open = false
			a.connectK8s(i)
			return
		}
	}
	d.open = false
	a.withK8sProject(func(p proto.Project) { a.addK8sPodTo(p, pod, cmd) })
}

// withK8sProject runs fn with the sidebar's K8s project, making it first
// if there is none, off the UI thread as the daemon call is.
func (a *App) withK8sProject(fn func(proto.Project)) {
	if p := a.k8sProjectOf(); p != nil {
		fn(*p)
		return
	}
	h := a.local()
	if h.cl == nil || !h.loaded {
		a.toast(locales.SshNoService())
		return
	}
	cl := h.cl
	go func() {
		p, err := k8sProject(cl, k8sProjectName())
		a.main(func() {
			if err != nil {
				a.toast(err.Error())
				return
			}
			a.prefs.K8sProject = p.ID
			a.savePrefs()
			if a.project(p.ID) == nil {
				a.st.Projects = append(a.st.Projects, p)
			}
			fn(p)
		})
	}()
}

// k8sProject makes the sidebar's K8s project: one of this machine's,
// named for the sidebar, with no directory of its own.
func k8sProject(cl *client.Client, name string) (proto.Project, error) {
	var p proto.Project
	err := cl.Call("project.create", proto.ProjectParams{Name: name}, &p)
	return p, err
}

// addK8sPodTo adds a pod to the K8s project and opens a terminal that
// enters it.
func (a *App) addK8sPodTo(p proto.Project, name, cmd string) {
	a.prefs.K8sPods = append(a.prefs.K8sPods, k8sPod{Name: name, Command: cmd})
	a.savePrefs()
	a.connectK8s(len(a.prefs.K8sPods) - 1)
}

// connectK8s shows a pod's terminal, starting one in the K8s project if
// it has none.
func (a *App) connectK8s(i int) {
	if i < 0 || i >= len(a.prefs.K8sPods) {
		return
	}
	a.showOrStartK8s(a.prefs.K8sPods[i])
}

// showOrStartK8s shows the terminal a pod's row has, if it still runs,
// else starts one: a row's click.
func (a *App) showOrStartK8s(pod k8sPod) {
	if s := a.session(pod.Session); s != nil && s.Status != proto.Exited {
		a.selectSession(s.ID)
		return
	}
	a.startK8s(pod)
}

// connectK8sTo connects to a pod again, in the row it was read from,
// whatever its terminal does: one still open closes, and the new
// terminal takes its place, so that the command finds the row once it
// comes.
func (a *App) connectK8sTo(pod k8sPod) {
	if s := a.session(pod.Session); s != nil {
		a.closeSession(s.ID, false)
	}
	a.startK8s(pod)
}

// startK8s starts a pod's terminal, in the K8s project.
func (a *App) startK8s(pod k8sPod) {
	args, _, err := k8sCommand(pod.Command)
	if err != nil {
		return
	}
	h := a.local()
	if h.cl == nil {
		a.toast(locales.SshNoService())
		return
	}
	a.withK8sProject(func(p proto.Project) {
		cl := h.cl
		go func() {
			var s proto.Session
			err := cl.Call("create", proto.CreateParams{Project: p.ID, Dir: proto.Home(), Name: pod.Name, Command: args}, &s)
			a.main(func() {
				if err != nil {
					a.toast(err.Error())
					return
				}
				// The row may have gone as the terminal came, and another
				// may have taken its place: the command finds it.
				i := slices.IndexFunc(a.prefs.K8sPods, func(p k8sPod) bool { return p.Command == pod.Command })
				if i < 0 {
					return
				}
				a.prefs.K8sPods[i].Session = s.ID
				a.savePrefs()
				delete(a.prefs.Collapsed, p.ID)
				a.addPending(h, s)
				a.selectSession(s.ID)
			})
		}()
	})
}

// k8sPodOfSession is the pod a session is the terminal of, nil for none.
func (a *App) k8sPodOfSession(id string) *k8sPod {
	if id == "" {
		return nil
	}
	for i := range a.prefs.K8sPods {
		if a.prefs.K8sPods[i].Session == id {
			return &a.prefs.K8sPods[i]
		}
	}
	return nil
}

// k8sDrag is a pod dragged by its row to another place in the K8s
// project.
type k8sDrag struct {
	command string
}

// moveK8sPod puts the pod of the command from where the pod of the
// command to is, before or after it, and keeps the order in the app's
// preferences.
func (a *App) moveK8sPod(from, to string, after bool) {
	at := func(cmd string) int {
		return slices.IndexFunc(a.prefs.K8sPods, func(p k8sPod) bool { return p.Command == cmd })
	}
	i, j := at(from), at(to)
	if from == "" || from == to || i < 0 || j < 0 {
		return
	}
	pod := a.prefs.K8sPods[i]
	a.prefs.K8sPods = slices.Delete(a.prefs.K8sPods, i, i+1)
	j = at(to)
	if after {
		j++
	}
	a.prefs.K8sPods = slices.Insert(a.prefs.K8sPods, j, pod)
	a.savePrefs()
}

// removeK8sPod removes a pod from the K8s project, ending its terminal
// as closing one does.
func (a *App) removeK8sPod(cmd string) {
	for i, pod := range a.prefs.K8sPods {
		if pod.Command != cmd {
			continue
		}
		a.prefs.K8sPods = slices.Delete(a.prefs.K8sPods, i, i+1)
		a.savePrefs()
		a.undoSays(locales.K8sPodRemoved(pod.Name), func() {
			a.prefs.K8sPods = slices.Insert(a.prefs.K8sPods, min(i, len(a.prefs.K8sPods)), pod)
			a.savePrefs()
		})
		if pod.Session != "" {
			// The question above said it goes.
			a.closeSession(pod.Session, false)
		}
		return
	}
}

// clearK8sProject forgets the K8s project, whose terminals go with it.
func (a *App) clearK8sProject() {
	a.prefs.K8sProject, a.prefs.K8sPods = "", nil
	a.savePrefs()
}

// k8sCommand reads a kubectl exec command, as the user typed it, and says
// the arguments to run and the pod it enters: kubectl exec -it web-0 -n
// prod -- /bin/bash enters web-0.
func k8sCommand(cmd string) ([]string, string, error) {
	args, err := shlex.Split(cmd, true)
	if err != nil || len(args) == 0 {
		return nil, "", errK8sNotExec
	}
	if filepath.Base(args[0]) != "kubectl" {
		return nil, "", errK8sNotExec
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
		return nil, "", errK8sNotExec
	}
	// The pod is what follows exec, before --, past kubectl's flags: -n
	// and the like take a value of their own.
	for i := exec + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			if k8sFlagValue(arg) {
				i++
			}
			continue
		}
		return args, arg, nil
	}
	return nil, "", errK8sNoPod
}

// k8sFlagValue reports whether a kubectl flag takes its value as the next
// argument, as -n prod does, unless it is written -n=prod.
func k8sFlagValue(flag string) bool {
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

// k8sPodRow is a pod in the K8s project: its name over the command that
// enters it, a click showing its terminal or starting one.
func (a *App) k8sPodRow(c *ui.Context, pal *palette, i int) {
	pod := a.prefs.K8sPods[i]
	s := a.session(pod.Session)
	if s != nil && s.Project != a.prefs.K8sProject {
		s = nil
	}
	selected := s != nil && s.ID == a.selected
	inset := float32(rowInset + projectBadge + projectBadgeGap)
	detailed := a.set.SidebarRows != "compact"
	height := float32(rowH)
	if detailed {
		height = rowH2
	}
	row := ui.Row(c.Key("k8s-"+pod.Command)).Height(height).Margin(0, 0, 1, 0).Padding(0, 10, 0, inset).Gap(10).AlignItems(ui.Center).Radius(8)
	row.Border(1, ui.Transparent)
	bg := ui.Transparent
	switch {
	case selected:
		bg = translucentGlass(pal.rowSelected, pal, a.set)
		chosen(row, pal)
	case row.Hovered():
		bg = translucentGlass(pal.rowHover, pal, a.set)
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	// The row is its pod's handle: dragged, it moves the pod to another
	// place in the project, its terminal with it.
	row.Drag(k8sDrag{command: pod.Command}).Cursor(ui.CursorGrab)
	if row.Dragging() {
		row.Opacity(0.35)
	}
	if d, over := ui.DragOver[k8sDrag](row); over && d.command != pod.Command {
		_, y, _ := row.PointerPosition()
		after := y > height/2
		a.dragAt, a.dragAfter = pod.Command, after
		row.Background(pal.rowHover)
		dragLine(row, pal, after)
	}
	if d, ok := ui.Drop[k8sDrag](row); ok && d.command != pod.Command {
		_, y, _ := row.PointerPosition()
		after := y > height/2
		if a.dragAt == pod.Command {
			// The side the insertion line last showed.
			after = a.dragAfter
		}
		to := pod.Command
		a.defer_(func() { a.moveK8sPod(d.command, to, after) })
	}
	pointerTooltip(c, row, "tip-k8s-"+pod.Command, pod.Command)
	row.ContextMenu(func(m *ui.Menu) {
		// Reconnecting goes on a terminal that ended, as kubectl's does
		// when the pod goes, and starts the terminal anew, or replaces one
		// still open: a click on the row shows that one. The row it read
		// is the pod's, whatever its terminal's end does to the list.
		if m.Item(locales.ActionReconnectHost()).Chosen() {
			a.connectK8sTo(pod)
		}
		m.Separator()
		if m.Item(locales.K8sCopyCommand()).Chosen() {
			a.copyText = pod.Command
		}
		if m.Item(locales.HostRemove()).Chosen() {
			name, cmd := pod.Name, pod.Command
			a.confirm = &confirmation{
				open:    true,
				title:   locales.K8sRemoveTitle(name),
				detail:  locales.K8sRemoveDetail(),
				action:  locales.CommonRemove(),
				destroy: true,
				run:     func() { a.removeK8sPod(cmd) },
			}
		}
	})
	if row.Clicked() && !selected {
		a.connectK8s(i)
	}
	row.Children(func() {
		ui.Icon(c, terminalIcon).Size(13, 13).TextColor(pal.textMuted).Shrink(0)
		top := func() {
			ui.Text(c, pod.Name).FontSize(13).TextColor(pal.text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		}
		if !detailed {
			top()
			ui.Spacer(c)
			ui.Text(c, pod.Command).Font("monospace").FontSize(12).TextColor(pal.textFaint).SingleLine().Ellipsis("…").Shrink(2).MaxWidth(220)
		} else {
			// The name and the command over the row's width, as a detailed
			// session's row has its name and detail.
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(top)
				ui.Text(c, pod.Command).Font("monospace").FontSize(12).TextColor(pal.textFaint).SingleLine().Ellipsis("…").MinWidth(0)
			})
		}
		if s == nil {
			// A pod with no terminal yet keeps the slot the status (or,
			// under the pointer, the close button) takes.
			ui.Box(c).Size(20, 20).Shrink(0)
			return
		}
		// Close ends the terminal, as Remove does, without its question.
		statusSlot(c, pal, row.Hovered(), s.Status, s.Unread, s.Progress, func() {
			cmd := pod.Command
			a.defer_(func() { a.removeK8sPod(cmd) })
		})
	})
}

// k8sEmpty is the K8s project with no pod in it yet: the row that adds
// one, in place of a project's sessions.
func (a *App) k8sEmpty(c *ui.Context, pal *palette) {
	inset := float32(rowInset + projectBadge + projectBadgeGap)
	row := ui.Row(c.Key("k8s-empty")).Height(rowH).Margin(0, 0, 1, 0).Padding(0, 10, 0, inset).Gap(10).AlignItems(ui.Center).Radius(8)
	if row.Hovered() {
		row.Background(pal.rowHover)
	}
	if row.Clicked() {
		a.askK8sPod()
	}
	row.Children(func() {
		ui.Icon(c, plusIcon).Size(13, 13).TextColor(pal.textMuted).Shrink(0)
		ui.Text(c, locales.ActionAddK8sPod()).FontSize(13).TextColor(pal.textMuted).SingleLine()
	})
}
