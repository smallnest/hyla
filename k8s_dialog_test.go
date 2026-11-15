package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"hyla/internal/app/host"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"

	"hyla/internal/ui/k8sdialog"
	"hyla/locales"
)

// The sidebar's Add K8s Pod opens a terminal running the kubectl exec
// typed, in the sidebar's one K8s project, named for the pod.
func TestAddK8sPodFromSidebar(t *testing.T) {
	a := testApp(t)
	a.local().Conn = host.StateReady
	creates := make(chan proto.CreateParams, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "project.create":
			var p proto.ProjectParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			if p.Name != locales.K8sPodsProject() {
				t.Errorf("the project is named %q, want %q", p.Name, locales.K8sPodsProject())
			}
			return proto.Project{ID: "k8s-p", Name: p.Name}, nil
		case "create":
			var p proto.CreateParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			creates <- p
			return proto.Session{ID: "k8s-s", Project: p.Project, Name: p.Name}, nil
		default:
			return nil, errors.New("unexpected method " + method)
		}
	})
	defer stop()
	a.local().Client = cl

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	// Add K8s Pod is in the sidebar's New Session menu, where the app's
	// ways of beginning work are.
	if err := tt.Click(locales.ActionNewSession()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem(locales.ActionAddK8sPod()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.K8sAsk == nil || !a.K8sAsk.Open {
		t.Fatalf("Add K8s Pod opened no dialog")
	}
	shot(t, tt, "add-k8s-pod")
	tt.Type("kubectl exec -it web-0 -n prod -- /bin/bash")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.K8sAsk != nil {
		t.Errorf("the dialog stays open, with %q", a.K8sAsk.Err)
	}

	var p proto.CreateParams
	select {
	case p = <-creates:
	case <-time.After(2 * time.Second):
		t.Fatal("no kubectl terminal started")
	}
	want := []string{"kubectl", "exec", "-it", "web-0", "-n", "prod", "--", "/bin/bash"}
	if !slices.Equal(p.Command, want) {
		t.Errorf("the session runs %q, want %q", p.Command, want)
	}
	if p.Project != "k8s-p" || p.Name != "web-0" || p.Dir != proto.Home() {
		t.Errorf("the session is %+v", p)
	}
	if a.Prefs.K8sProject != "k8s-p" || len(a.Prefs.K8sPods) != 1 {
		t.Fatalf("the sidebar records %q and %+v", a.Prefs.K8sProject, a.Prefs.K8sPods)
	}
	if pod := a.Prefs.K8sPods[0]; pod.Name != "web-0" || pod.Session != "k8s-s" {
		t.Errorf("the pod is %+v", pod)
	}
	tt.Frame()
	if !tt.HasText(locales.K8sPodsProject()) || !tt.HasText("web-0") {
		t.Errorf("the sidebar shows no K8s project row; texts %q", tt.Texts())
	}
	shot(t, tt, "sidebar-k8s-pod")
}

// The command the project lists already is one pod: connecting again
// shows its terminal, and adds no second row.
func TestAddK8sPodOnce(t *testing.T) {
	a := testApp(t)
	a.local().Conn = host.StateReady
	projects, creates := 0, 0
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "project.create":
			projects++
			return proto.Project{ID: fmt.Sprintf("k8s-p%d", projects), Name: "k8s"}, nil
		case "create":
			creates++
			var p proto.CreateParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			return proto.Session{ID: fmt.Sprintf("k8s-s%d", creates), Project: p.Project}, nil
		default:
			return nil, errors.New("unexpected method " + method)
		}
	})
	defer stop()
	a.local().Client = cl

	for i := range 2 {
		a.K8sAsk = &k8sdialog.Ask{Open: true, Cmd: "kubectl exec -it web-0 -n prod -- /bin/bash"}
		k8sdialog.Add(a)
		deadline := time.Now().Add(2 * time.Second)
		for len(a.Prefs.K8sPods) == 0 || a.Prefs.K8sPods[0].Session == "" {
			if time.Now().After(deadline) {
				t.Fatalf("terminal %d started none", i+1)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	if projects != 1 || creates != 1 || len(a.Prefs.K8sPods) != 1 {
		t.Errorf("%d projects, %d terminals, %+v", projects, creates, a.Prefs.K8sPods)
	}
}

// The dialog takes kubectl exec alone, and says what it lacks.
func TestAddK8sPodOnlyExec(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1100, 700)
	for _, c := range []struct {
		cmd, want string
	}{
		{"kubectl logs web-0", locales.K8sPodOnly()},
		{"ssh devbox", locales.K8sPodOnly()},
		{"kubectl exec -it -n prod -- sh", locales.K8sPodNeedsPod()},
		{"kubectl exec -it web-0 -- sh", ""},
	} {
		a.K8sAsk = &k8sdialog.Ask{Open: true, Cmd: c.cmd}
		tt.Frame()
		if err := tt.Click(locales.CommonConnect()); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if c.want == "" {
			continue
		}
		if a.K8sAsk == nil || a.K8sAsk.Err != c.want {
			t.Errorf("%q: the dialog said %v, want %q", c.cmd, a.K8sAsk, c.want)
		}
	}
}

// k8sCommand reads the pod of a kubectl exec whatever its flags, and
// refuses what is no kubectl exec.
func TestK8sCommand(t *testing.T) {
	for _, c := range []struct {
		cmd, pod string
	}{
		{"kubectl exec -it web-0 -n prod -- /bin/bash", "web-0"},
		{"kubectl -n prod exec web-1 -c app -- sh", "web-1"},
		{"kubectl exec -n prod -it web-2 -- sh", "web-2"},
		{"kubectl exec --namespace=prod web-3 -- sh", "web-3"},
		{"/opt/homebrew/bin/kubectl exec web-4 -- sh", "web-4"},
	} {
		_, pod, err := k8sCommand(c.cmd)
		if err != nil || pod != c.pod {
			t.Errorf("k8sCommand(%q) = %q, %v, want %q", c.cmd, pod, err, c.pod)
		}
	}
	for _, c := range []struct {
		cmd  string
		want error
	}{
		{"kubectl logs web-0", k8sdialog.ErrNotExec},
		{"ssh devbox", k8sdialog.ErrNotExec},
		{"kubectl exec -it -n prod -- sh", k8sdialog.ErrNoPod},
		{"kubectl exec -it 'web-0 -- sh", k8sdialog.ErrNotExec},
	} {
		if _, _, err := k8sCommand(c.cmd); !errors.Is(err, c.want) {
			t.Errorf("k8sCommand(%q) = %v, want %v", c.cmd, err, c.want)
		}
	}
}

// The K8s project lists its pods, each its name over the command; a click
// opens the pod's terminal or starts one, and an empty project offers to
// add one.
func TestSidebarK8sPod(t *testing.T) {
	a := testApp(t)
	a.local().Conn = host.StateReady
	a.mergeState(a.local(), proto.State{Projects: []proto.Project{{ID: "p-k8s", Name: locales.K8sPodsProject()}}})
	a.Prefs.K8sProject = "p-k8s"
	a.Prefs.K8sPods = []k8sPod{{Name: "web-0", Command: "kubectl exec -it web-0 -n prod -- /bin/bash"}}
	creates := make(chan proto.CreateParams, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		if method != "create" {
			return nil, errors.New("unexpected method " + method)
		}
		var p proto.CreateParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		creates <- p
		return proto.Session{ID: "k8s-s", Project: p.Project, Name: p.Name}, nil
	})
	defer stop()
	a.local().Client = cl

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText(locales.K8sPodsProject()) || !tt.HasText("web-0") || !tt.HasText("kubectl exec -it web-0 -n prod -- /bin/bash") {
		t.Fatalf("the K8s project shows no pod; texts %q", tt.Texts())
	}
	if err := tt.Click("web-0"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	select {
	case p := <-creates:
		want := []string{"kubectl", "exec", "-it", "web-0", "-n", "prod", "--", "/bin/bash"}
		if !slices.Equal(p.Command, want) {
			t.Errorf("the row runs %q, want %q", p.Command, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("clicking the pod started no terminal")
	}
	// The terminal is the pod's row, not a session of its own.
	deadline := time.Now().Add(2 * time.Second)
	for a.Prefs.K8sPods[0].Session == "" {
		if time.Now().After(deadline) {
			t.Fatal("the row kept no terminal")
		}
		time.Sleep(2 * time.Millisecond)
	}
	tt.Frame()
	if n := countText(tt.Texts(), "web-0"); n != 1 {
		t.Errorf("the pod shows %d times: %q", n, tt.Texts())
	}

	// Empty, the project offers the row that adds a pod.
	a.Prefs.K8sPods, a.Prefs.K8sProject = nil, "p-k8s"
	tt = ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText(locales.ActionAddK8sPod()) {
		t.Fatalf("no row adds a pod; texts %q", tt.Texts())
	}
	shot(t, tt, "sidebar-k8s-empty")
	if err := tt.Click(locales.ActionAddK8sPod()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.K8sAsk == nil || !a.K8sAsk.Open {
		t.Fatal("the row opened no dialog")
	}
}

// Dragging a pod's row moves it in the K8s project, as a project's
// sessions move among themselves.
func TestK8sPodDrag(t *testing.T) {
	a := testApp(t)
	a.St.Projects = append(a.St.Projects, proto.Project{ID: "p-k8s", Name: locales.K8sPodsProject()})
	a.Prefs.K8sProject = "p-k8s"
	a.Prefs.K8sPods = []k8sPod{
		{Name: "one", Command: "kubectl exec -it one -- sh"},
		{Name: "two", Command: "kubectl exec -it two -- sh"},
	}
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	dragRow(t, tt, "two", "one", false)
	var cmds []string
	for _, p := range a.Prefs.K8sPods {
		cmds = append(cmds, p.Command)
	}
	want := []string{"kubectl exec -it two -- sh", "kubectl exec -it one -- sh"}
	if !slices.Equal(cmds, want) {
		t.Errorf("the K8s project lists %v, want %v", cmds, want)
	}
}

// A pod offers to reconnect whatever its terminal does: one that ended,
// as kubectl's does when the pod goes, and one still open, which
// reconnecting replaces. Either way it starts a new terminal, in the
// row's place.
func TestK8sPodReconnect(t *testing.T) {
	for _, open := range []bool{false, true} {
		a := testApp(t)
		a.local().Conn = host.StateReady
		st := proto.State{Projects: []proto.Project{{ID: "p-k8s", Name: locales.K8sPodsProject()}}}
		status := proto.Exited
		if open {
			status = proto.Idle
		}
		st.Sessions = []proto.Session{{
			Pane: proto.Pane{Status: status, Progress: -1},
			ID:   "k8s-s", Project: "p-k8s", Name: "web-0",
			Panes: []proto.Pane{{ID: "kp", Status: status, Progress: -1}},
		}}
		a.mergeState(a.local(), st)
		a.Prefs.K8sProject = "p-k8s"
		a.Prefs.K8sPods = []k8sPod{{Name: "web-0", Command: "kubectl exec -it web-0 -n prod -- /bin/bash", Session: "k8s-s"}}
		creates := make(chan proto.CreateParams, 1)
		cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
			if method != "create" {
				return nil, errors.New("unexpected method " + method)
			}
			var p proto.CreateParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			creates <- p
			return proto.Session{ID: "k8s-s2", Project: p.Project, Name: p.Name}, nil
		})
		a.local().Client = cl

		tt := ui.NewTester(a.view, 1100, 700)
		tt.Frame()
		if err := tt.RightClick("web-0"); err != nil {
			stop()
			t.Fatal(err)
		}
		tt.Frame()
		want := []string{locales.ActionReconnectHost(), "-", locales.K8sCopyCommand(), locales.HostRemove()}
		if got := tt.Menu(); !reflect.DeepEqual(got, want) {
			stop()
			t.Fatalf("open %v: the menu shows %q, want %q", open, got, want)
		}
		if err := tt.ChooseMenuItem(locales.ActionReconnectHost()); err != nil {
			stop()
			t.Fatal(err)
		}
		select {
		case p := <-creates:
			want := []string{"kubectl", "exec", "-it", "web-0", "-n", "prod", "--", "/bin/bash"}
			if !slices.Equal(p.Command, want) {
				t.Errorf("open %v: the new terminal runs %q, want %q", open, p.Command, want)
			}
		case <-time.After(2 * time.Second):
			stop()
			t.Fatalf("open %v: Reconnect Host started no terminal", open)
		}
		deadline := time.Now().Add(2 * time.Second)
		for len(a.Prefs.K8sPods) == 0 || a.Prefs.K8sPods[0].Session != "k8s-s2" {
			if time.Now().After(deadline) {
				stop()
				t.Fatalf("open %v: the pods kept %+v", open, a.Prefs.K8sPods)
			}
			time.Sleep(2 * time.Millisecond)
		}
		stop()
	}
}

// A terminal of a pod says the command that entered it, not the folder it
// runs in, as its second line.
func TestK8sSessionRowShowsCommand(t *testing.T) {
	a := testApp(t)
	cmd := "kubectl exec -it web-0 -n prod -- /bin/bash"
	a.mergeState(a.local(), proto.State{
		Projects: []proto.Project{{ID: "p-k8s", Name: locales.K8sPodsProject()}},
		Sessions: []proto.Session{{
			ID: "k8s-s", Project: "p-k8s", Name: "web-0",
			Panes: []proto.Pane{{ID: "kp", Dir: "/home/me", Status: proto.Idle, Progress: -1}},
		}},
	})
	a.Prefs.K8sProject = "p-k8s"
	a.Prefs.K8sPods = []k8sPod{{Name: "web-0", Command: cmd, Session: "k8s-s"}}
	a.Prefs.SidebarGroup = "none"
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText(cmd) {
		t.Fatalf("the terminal shows no command; texts %q", tt.Texts())
	}
	if tt.HasText("/home/me") {
		t.Errorf("the terminal shows its folder too: %q", tt.Texts())
	}
	shot(t, tt, "sidebar-k8s-session")
}

// A pod's row gives its terminal's status up, under the pointer, to a
// close button of the same size, which ends the terminal and the pod with
// it, as Remove does, without its question.
func TestK8sPodHoverClose(t *testing.T) {
	a := testApp(t)
	a.Prefs.K8sProject = "k8s-p"
	a.Prefs.K8sPods = []k8sPod{{Name: "web-0", Command: "kubectl exec -it web-0 -- bash", Session: "k8s-s"}}
	a.St.Projects = append(a.St.Projects, proto.Project{ID: "k8s-p", Name: locales.K8sPodsProject()})
	base := a.St.Sessions[0]
	base.ID, base.Name, base.Project, base.Process, base.Status = "k8s-s", "web-0", "k8s-p", "kubectl", proto.Working
	a.St.Sessions = append(a.St.Sessions, base)
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	title, ok := tt.Find("web-0")
	if !ok {
		t.Fatalf("no pod's row; texts %q", tt.Texts())
	}
	label := locales.ActionCloseSession()
	tt.Move(title.X+title.W/2, title.Y+title.H/2)
	tt.Frame()
	button, ok := tt.Find(label)
	if !ok {
		t.Fatal("hover did not show close button")
	}
	if after, _ := tt.Find("web-0"); after != title {
		t.Fatalf("hover moved the name: %v → %v", title, after)
	}
	tt.Press(button.X+button.W/2, button.Y+button.H/2)
	tt.Frame()
	tt.Release(button.X+button.W/2, button.Y+button.H/2)
	tt.Frame()
	if a.Confirm != nil && a.Confirm.Open {
		t.Fatal("closing from the row asked first")
	}
	if len(a.Prefs.K8sPods) != 0 || a.session("k8s-s") != nil {
		t.Errorf("the pod stays: %+v, %+v", a.Prefs.K8sPods, a.St.Sessions)
	}
}
