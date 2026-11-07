package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"

	"hyla/locales"
)

// The sidebar's Add K8s Pod opens a terminal running the kubectl exec
// typed, in the sidebar's one K8s project, named for the pod.
func TestAddK8sPodFromSidebar(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
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
	a.local().cl = cl

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if err := tt.Click(locales.ActionAddK8sPod()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.k8sAsk == nil || !a.k8sAsk.open {
		t.Fatalf("Add K8s Pod opened no dialog")
	}
	shot(t, tt, "add-k8s-pod")
	tt.Type("kubectl exec -it web-0 -n prod -- /bin/bash")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.k8sAsk != nil {
		t.Errorf("the dialog stays open, with %q", a.k8sAsk.err)
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
	if a.prefs.K8sProject != "k8s-p" || len(a.prefs.K8sPods) != 1 {
		t.Fatalf("the sidebar records %q and %+v", a.prefs.K8sProject, a.prefs.K8sPods)
	}
	if pod := a.prefs.K8sPods[0]; pod.Name != "web-0" || pod.Session != "k8s-s" {
		t.Errorf("the pod is %+v", pod)
	}
	// A kubectl terminal is no Hyla host: nothing is added to Settings.
	if len(a.set.Hosts) != 0 {
		t.Errorf("the dialog added a Hyla host: %+v", a.set.Hosts)
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
	a.local().conn = connReady
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
	a.local().cl = cl

	for i := range 2 {
		a.k8sAsk = &k8sAsk{open: true, cmd: "kubectl exec -it web-0 -n prod -- /bin/bash"}
		a.addK8sPod()
		deadline := time.Now().Add(2 * time.Second)
		for len(a.prefs.K8sPods) == 0 || a.prefs.K8sPods[0].Session == "" {
			if time.Now().After(deadline) {
				t.Fatalf("terminal %d started none", i+1)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	if projects != 1 || creates != 1 || len(a.prefs.K8sPods) != 1 {
		t.Errorf("%d projects, %d terminals, %+v", projects, creates, a.prefs.K8sPods)
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
		a.k8sAsk = &k8sAsk{open: true, cmd: c.cmd}
		tt.Frame()
		if err := tt.Click(locales.CommonConnect()); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if c.want == "" {
			continue
		}
		if a.k8sAsk == nil || a.k8sAsk.err != c.want {
			t.Errorf("%q: the dialog said %v, want %q", c.cmd, a.k8sAsk, c.want)
		}
	}
	if len(a.set.Hosts) != 0 {
		t.Errorf("the dialog added a Hyla host: %+v", a.set.Hosts)
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
		{"kubectl logs web-0", errK8sNotExec},
		{"ssh devbox", errK8sNotExec},
		{"kubectl exec -it -n prod -- sh", errK8sNoPod},
		{"kubectl exec -it 'web-0 -- sh", errK8sNotExec},
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
	a.local().conn = connReady
	a.mergeState(a.local(), proto.State{Projects: []proto.Project{{ID: "p-k8s", Name: locales.K8sPodsProject()}}})
	a.prefs.K8sProject = "p-k8s"
	a.prefs.K8sPods = []k8sPod{{Name: "web-0", Command: "kubectl exec -it web-0 -n prod -- /bin/bash"}}
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
	a.local().cl = cl

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
	for a.prefs.K8sPods[0].Session == "" {
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
	a.prefs.K8sPods, a.prefs.K8sProject = nil, "p-k8s"
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
	if a.k8sAsk == nil || !a.k8sAsk.open {
		t.Fatal("the row opened no dialog")
	}
}

// A pod offers to reconnect whatever its terminal does: one that ended,
// as kubectl's does when the pod goes, and one still open, which
// reconnecting replaces. Either way it starts a new terminal, in the
// row's place.
func TestK8sPodReconnect(t *testing.T) {
	for _, open := range []bool{false, true} {
		a := testApp(t)
		a.local().conn = connReady
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
		a.prefs.K8sProject = "p-k8s"
		a.prefs.K8sPods = []k8sPod{{Name: "web-0", Command: "kubectl exec -it web-0 -n prod -- /bin/bash", Session: "k8s-s"}}
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
		a.local().cl = cl

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
		for len(a.prefs.K8sPods) == 0 || a.prefs.K8sPods[0].Session != "k8s-s2" {
			if time.Now().After(deadline) {
				stop()
				t.Fatalf("open %v: the pods kept %+v", open, a.prefs.K8sPods)
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
	a.prefs.K8sProject = "p-k8s"
	a.prefs.K8sPods = []k8sPod{{Name: "web-0", Command: cmd, Session: "k8s-s"}}
	a.prefs.SidebarGroup = "none"
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
