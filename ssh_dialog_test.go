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
	"hyla/internal/remote"

	"hyla/locales"
)

// The sidebar's Add SSH Host opens a terminal running ssh to the address
// typed, in the sidebar's one SSH project, named for the machine: the
// machine it reaches needs no Hyla.
func TestAddSSHHostFromSidebar(t *testing.T) {
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
			if p.Name != locales.SshHostsProject() {
				t.Errorf("the project is named %q, want %q", p.Name, locales.SshHostsProject())
			}
			return proto.Project{ID: "ssh-p", Name: p.Name}, nil
		case "create":
			var p proto.CreateParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			creates <- p
			return proto.Session{ID: "ssh-s", Project: p.Project, Name: p.Name}, nil
		default:
			return nil, errors.New("unexpected method " + method)
		}
	})
	defer stop()
	a.local().cl = cl

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	// Add SSH Host is in the sidebar's New Session menu, where the app's
	// ways of beginning work are.
	if err := tt.Click(locales.ActionNewSession()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem(locales.ActionAddSSHHost()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.sshAsk == nil || !a.sshAsk.open {
		t.Fatalf("Add SSH Host opened no dialog")
	}
	tt.Type("root@example.com:2222")
	tt.Frame()
	// The name's field says what the machine will be called while it is
	// empty: the address's host.
	if !tt.HasText(locales.SetHostNamePlaceholder("example.com")) {
		t.Fatalf("the name field offers no name; texts %q", tt.Texts())
	}
	shot(t, tt, "add-ssh-host")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.sshAsk != nil {
		t.Errorf("the dialog stays open, with %q", a.sshAsk.err)
	}

	var p proto.CreateParams
	select {
	case p = <-creates:
	case <-time.After(2 * time.Second):
		t.Fatal("no ssh session started")
	}
	if want := []string{"ssh", "-p", "2222", "root@example.com"}; !slices.Equal(p.Command, want) {
		t.Errorf("the session runs %q, want %q", p.Command, want)
	}
	if p.Project != "ssh-p" || p.Name != "example.com" || p.Dir != proto.Home() {
		t.Errorf("the session is %+v", p)
	}
	if a.prefs.SSHProject != "ssh-p" || len(a.prefs.SSHHosts) != 1 {
		t.Fatalf("the sidebar records %q and %+v", a.prefs.SSHProject, a.prefs.SSHHosts)
	}
	if h := a.prefs.SSHHosts[0]; h.Name != "example.com" || h.Addr != "root@example.com:2222" || h.Session != "ssh-s" {
		t.Errorf("the machine is %+v", h)
	}
	// An SSH terminal is no Hyla host: nothing is added to Settings.
	if len(a.set.Hosts) != 0 {
		t.Errorf("the dialog added a Hyla host: %+v", a.set.Hosts)
	}
	tt.Frame()
	if !tt.HasText("example.com") || !tt.HasText("root@example.com:2222") || !tt.HasText(locales.SshHostsProject()) {
		t.Errorf("the sidebar shows no SSH project row; texts %q", tt.Texts())
	}
	shot(t, tt, "sidebar-ssh-host")
}

// The address the project lists already is one machine: connecting again
// shows its terminal, and adds no second row.
func TestAddSSHHostOnce(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	projects, creates := 0, 0
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "project.create":
			projects++
			return proto.Project{ID: fmt.Sprintf("ssh-p%d", projects), Name: "ssh"}, nil
		case "create":
			creates++
			var p proto.CreateParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			return proto.Session{ID: fmt.Sprintf("ssh-s%d", creates), Project: p.Project}, nil
		default:
			return nil, errors.New("unexpected method " + method)
		}
	})
	defer stop()
	a.local().cl = cl

	for i := range 2 {
		a.sshAsk = &sshAsk{open: true, addr: "root@example.com:2222", name: "buildbox"}
		a.addSSHHost()
		deadline := time.Now().Add(2 * time.Second)
		for len(a.prefs.SSHHosts) == 0 || a.prefs.SSHHosts[0].Session == "" {
			if time.Now().After(deadline) {
				t.Fatalf("terminal %d started none", i+1)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	if projects != 1 || creates != 1 || len(a.prefs.SSHHosts) != 1 {
		t.Errorf("%d projects, %d terminals, %+v", projects, creates, a.prefs.SSHHosts)
	}
}

// The dialog takes SSH addresses alone: the other ways of reaching a
// machine, Hyla hosts among them, are Settings › Hyla Hosts.
func TestAddSSHHostOnlySSH(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1100, 700)
	for _, addr := range []string{"http://devbox:7473", "wsl://Ubuntu"} {
		a.sshAsk = &sshAsk{open: true, addr: addr}
		tt.Frame()
		if err := tt.Click(locales.CommonConnect()); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.sshAsk == nil || a.sshAsk.err != locales.SshHostOnly() {
			t.Errorf("%s: the dialog said %v, want %q", addr, a.sshAsk, locales.SshHostOnly())
		}
	}
}

// sshArgs takes what ssh does: the port as its own option, and the user
// and host as a destination.
func TestSSHArgs(t *testing.T) {
	for _, c := range []struct {
		addr string
		want []string
	}{
		{"root@example.com:2222", []string{"ssh", "-p", "2222", "root@example.com"}},
		{"ssh://me@devbox", []string{"ssh", "me@devbox"}},
		{"devbox", []string{"ssh", "devbox"}},
	} {
		addr, err := remote.ParseAddr(c.addr)
		if err != nil {
			t.Fatalf("%s: %v", c.addr, err)
		}
		if got := sshArgs(addr); !slices.Equal(got, c.want) {
			t.Errorf("sshArgs(%q) = %q, want %q", c.addr, got, c.want)
		}
	}
}

// The SSH project lists its machines, each its name over the address ssh
// runs to; a click opens the machine's terminal or starts one.
func TestSidebarSSHHost(t *testing.T) {
	if testing.Short() {
		t.Skip("a terminal")
	}
	a := testApp(t)
	a.local().conn = connReady
	a.mergeState(a.local(), proto.State{Projects: []proto.Project{{ID: "p-ssh", Name: locales.SshHostsProject()}}})
	a.prefs.SSHProject = "p-ssh"
	a.prefs.SSHHosts = []sshHost{{Name: "example.com", Addr: "root@example.com:2222"}}
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
		return proto.Session{ID: "ssh-s", Project: p.Project, Name: p.Name}, nil
	})
	defer stop()
	a.local().cl = cl

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText(locales.SshHostsProject()) || !tt.HasText("example.com") || !tt.HasText("root@example.com:2222") {
		t.Fatalf("the SSH project shows no machine; texts %q", tt.Texts())
	}
	if err := tt.Click("root@example.com:2222"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	select {
	case p := <-creates:
		if want := []string{"ssh", "-p", "2222", "root@example.com"}; !slices.Equal(p.Command, want) {
			t.Errorf("the row runs %q, want %q", p.Command, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("clicking the machine started no terminal")
	}
	// The terminal is the machine's row, not a session of its own.
	deadline := time.Now().Add(2 * time.Second)
	for a.prefs.SSHHosts[0].Session == "" {
		if time.Now().After(deadline) {
			t.Fatal("the row kept no terminal")
		}
		time.Sleep(2 * time.Millisecond)
	}
	tt.Frame()
	if n := countText(tt.Texts(), "example.com"); n != 1 {
		t.Errorf("the machine shows %d times: %q", n, tt.Texts())
	}
	shot(t, tt, "sidebar-ssh-host")
}

// A machine whose terminal ended, as ssh's does when the connection
// drops, offers to reconnect it, and reconnecting starts a new terminal.
func TestSSHHostReconnect(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	a.mergeState(a.local(), proto.State{
		Projects: []proto.Project{{ID: "p-ssh", Name: locales.SshHostsProject()}},
		Sessions: []proto.Session{{
			Pane: proto.Pane{Status: proto.Exited, Progress: -1},
			ID:   "ssh-s", Project: "p-ssh", Name: "example.com",
			Panes: []proto.Pane{{ID: "sp", Status: proto.Exited, Progress: -1}},
		}},
	})
	a.prefs.SSHProject = "p-ssh"
	a.prefs.SSHHosts = []sshHost{{Name: "example.com", Addr: "root@example.com:2222", Session: "ssh-s"}}
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
		return proto.Session{ID: "ssh-s2", Project: p.Project, Name: p.Name}, nil
	})
	defer stop()
	a.local().cl = cl

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if err := tt.RightClick("root@example.com:2222"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	want := []string{locales.ActionReconnectHost(), locales.ActionDisconnectHost(), "-", locales.HostCopyAddress(), locales.HostRemove()}
	if got := tt.Menu(); !reflect.DeepEqual(got, want) {
		t.Fatalf("the row's menu shows %q, want %q", got, want)
	}
	if err := tt.ChooseMenuItem(locales.ActionReconnectHost()); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-creates:
		if want := []string{"ssh", "-p", "2222", "root@example.com"}; !slices.Equal(p.Command, want) {
			t.Errorf("the new terminal runs %q, want %q", p.Command, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Reconnect Host started no terminal")
	}
	deadline := time.Now().Add(2 * time.Second)
	for a.prefs.SSHHosts[0].Session != "ssh-s2" {
		if time.Now().After(deadline) {
			t.Fatalf("the row kept %q", a.prefs.SSHHosts[0].Session)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// A machine's menu is the same whether its terminal is open or not: a
// click shows one that is, and reconnecting starts a new one in place of
// the old, as ssh's does when the connection drops mid-session.
func TestSSHHostReconnectOpen(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	a.mergeState(a.local(), proto.State{
		Projects: []proto.Project{{ID: "p-ssh", Name: locales.SshHostsProject()}},
		Sessions: []proto.Session{{
			Pane: proto.Pane{Status: proto.Idle, Progress: -1},
			ID:   "ssh-s", Project: "p-ssh", Name: "example.com",
			Panes: []proto.Pane{{ID: "sp", Status: proto.Idle, Progress: -1}},
		}},
	})
	a.prefs.SSHProject = "p-ssh"
	a.prefs.SSHHosts = []sshHost{{Name: "example.com", Addr: "root@example.com:2222", Session: "ssh-s"}}
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
		return proto.Session{ID: "ssh-s2", Project: p.Project, Name: p.Name}, nil
	})
	defer stop()
	a.local().cl = cl

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if err := tt.RightClick("root@example.com:2222"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	want := []string{locales.ActionReconnectHost(), locales.ActionDisconnectHost(), "-", locales.HostCopyAddress(), locales.HostRemove()}
	if got := tt.Menu(); !reflect.DeepEqual(got, want) {
		t.Fatalf("the open row's menu shows %q, want %q", got, want)
	}
	if err := tt.ChooseMenuItem(locales.ActionReconnectHost()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-creates:
	case <-time.After(2 * time.Second):
		t.Fatal("Reconnect Host started no terminal")
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(a.prefs.SSHHosts) == 0 || a.prefs.SSHHosts[0].Session != "ssh-s2" {
		if time.Now().After(deadline) {
			t.Fatalf("the hosts kept %+v", a.prefs.SSHHosts)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// A machine's menu offers to disconnect it: its terminal ends, and ssh
// with it, while its row stays, and a click on the row connects again.
func TestSSHHostDisconnect(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	a.mergeState(a.local(), proto.State{
		Projects: []proto.Project{{ID: "p-ssh", Name: locales.SshHostsProject()}},
		Sessions: []proto.Session{{
			Pane: proto.Pane{Status: proto.Idle, Progress: -1},
			ID:   "ssh-s", Project: "p-ssh", Name: "example.com",
			Panes: []proto.Pane{{ID: "sp", Status: proto.Idle, Progress: -1}},
		}},
	})
	a.prefs.SSHProject = "p-ssh"
	a.prefs.SSHHosts = []sshHost{{Name: "example.com", Addr: "root@example.com:2222", Session: "ssh-s"}}
	closes := make(chan proto.CloseParams, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		if method != "close" {
			return nil, errors.New("unexpected method " + method)
		}
		var p proto.CloseParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		closes <- p
		return nil, nil
	})
	defer stop()
	a.local().cl = cl

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if err := tt.RightClick("root@example.com:2222"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem(locales.ActionDisconnectHost()); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-closes:
		if p.ID != "ssh-s" {
			t.Errorf("the terminal closed is %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Disconnect Host closed no terminal")
	}
	tt.Frame()
	if len(a.prefs.SSHHosts) != 1 || a.prefs.SSHHosts[0].Session != "" {
		t.Fatalf("the row kept %+v", a.prefs.SSHHosts)
	}
	if !tt.HasText("root@example.com:2222") {
		t.Errorf("the row left the sidebar; texts %q", tt.Texts())
	}
	if a.session("ssh-s") != nil {
		t.Errorf("the terminal stays in the app")
	}
	// A machine with no terminal has nothing to disconnect.
	if err := tt.RightClick("root@example.com:2222"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem(locales.ActionDisconnectHost()); err == nil {
		t.Error("a machine with no terminal offers to disconnect it")
	}
	tt.CloseMenu()
}

// An SSH project with no machine yet shows the row that adds one.
func TestSidebarSSHProjectEmpty(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), proto.State{Projects: []proto.Project{{ID: "p-ssh", Name: locales.SshHostsProject()}}})
	a.prefs.SSHProject = "p-ssh"
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText(locales.ActionAddSSHHost()) {
		t.Fatalf("no row adds a machine; texts %q", tt.Texts())
	}
	shot(t, tt, "sidebar-ssh-project-empty")
	if err := tt.Click(locales.ActionAddSSHHost()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.sshAsk == nil || !a.sshAsk.open {
		t.Fatal("the row opened no dialog")
	}
}

// A terminal of an SSH machine says the address ssh runs to, not the
// folder it runs in, as its second line.
func TestSSHSessionRowShowsAddress(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), proto.State{
		Projects: []proto.Project{{ID: "p-ssh", Name: locales.SshHostsProject()}},
		Sessions: []proto.Session{{
			ID: "ssh-s", Project: "p-ssh", Name: "example.com",
			Panes: []proto.Pane{{ID: "sp", Dir: "/home/me", Status: proto.Idle, Progress: -1}},
		}},
	})
	a.prefs.SSHProject = "p-ssh"
	a.prefs.SSHHosts = []sshHost{{Name: "example.com", Addr: "root@example.com:2222", Session: "ssh-s"}}
	a.prefs.SidebarGroup = "none"
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText("root@example.com:2222") {
		t.Fatalf("the terminal shows no address; texts %q", tt.Texts())
	}
	if tt.HasText("/home/me") {
		t.Errorf("the terminal shows its folder too: %q", tt.Texts())
	}
	shot(t, tt, "sidebar-ssh-session")
}

// A machine's row gives its terminal's status up, under the pointer, to a
// close button of the same size, which ends the terminal and the machine
// with it, as Remove does, without its question.
func TestSSHHostHoverClose(t *testing.T) {
	a := testApp(t)
	a.prefs.SSHProject = "ssh-p"
	a.prefs.SSHHosts = []sshHost{{Name: "box", Addr: "me@box", Session: "ssh-s"}}
	a.st.Projects = append(a.st.Projects, proto.Project{ID: "ssh-p", Name: locales.SshHostsProject()})
	base := a.st.Sessions[0]
	base.ID, base.Name, base.Project, base.Process, base.Status = "ssh-s", "box", "ssh-p", "ssh", proto.Working
	a.st.Sessions = append(a.st.Sessions, base)
	a.prefs.SSHHosts[0].Session = "ssh-s"
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	title, ok := tt.Find("box")
	if !ok {
		t.Fatalf("no machine's row; texts %q", tt.Texts())
	}
	label := locales.ActionCloseSession()
	tt.Move(title.X+title.W/2, title.Y+title.H/2)
	tt.Frame()
	button, ok := tt.Find(label)
	if !ok {
		t.Fatal("hover did not show close button")
	}
	if after, _ := tt.Find("box"); after != title {
		t.Fatalf("hover moved the name: %v → %v", title, after)
	}
	tt.Press(button.X+button.W/2, button.Y+button.H/2)
	tt.Frame()
	tt.Release(button.X+button.W/2, button.Y+button.H/2)
	tt.Frame()
	if a.confirm != nil && a.confirm.open {
		t.Fatal("closing from the row asked first")
	}
	if len(a.prefs.SSHHosts) != 0 || a.session("ssh-s") != nil {
		t.Errorf("the machine stays: %+v, %+v", a.prefs.SSHHosts, a.st.Sessions)
	}
}
