package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/remote"
)

// aRemoteHylad answers as another machine's hylad at a tcp:// listener:
// the password, then Hyla's own protocol. Its state has one session, of
// one pane.
func aRemoteHylad(t *testing.T, token string, st proto.State) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveRemoteHylad(c, token, st)
		}
	}()
	return ln.Addr().String()
}

func serveRemoteHylad(c net.Conn, token string, st proto.State) {
	defer c.Close()
	r := bufio.NewReaderSize(c, 64<<10)
	line, err := r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != token {
		return
	}
	hello, err := proto.ReadLine(r, 64<<10)
	if err != nil {
		return
	}
	var h proto.Hello
	if json.Unmarshal(hello, &h) != nil {
		return
	}
	json.NewEncoder(c).Encode(proto.Welcome{Version: proto.Version, PID: 1, Started: time.Now().UnixMilli(), App: "test"})
	if h.Kind == "attach" {
		// A pane's stream: the terminal's frames go the other way, and the
		// test needs only that it opened.
		io.Copy(io.Discard, r)
		return
	}
	for {
		line, err := proto.ReadLine(r, proto.MaxLine)
		if err != nil {
			return
		}
		var req proto.Request
		if json.Unmarshal(line, &req) != nil {
			return
		}
		var res any = map[string]any{}
		if req.Method == "state" {
			res = st
		}
		json.NewEncoder(c).Encode(map[string]any{"id": req.ID, "result": res})
	}
}

// remoteState is the state of the fake machine: a project with a session.
func remoteState() proto.State {
	return proto.State{
		Projects: []proto.Project{{ID: "rp", Name: "remote app", Dir: "/srv/app"}},
		Sessions: []proto.Session{{
			ID: "rs", Project: "rp", Name: "remote shell", Dir: "/srv/app",
			Panes:  []proto.Pane{{ID: "rpp", Dir: "/srv/app", Status: proto.Idle, Progress: -1}},
			Focus:  "rpp",
			Layout: nil,
		}},
	}
}

// A machine added by its address and password shows its sessions beside
// this one's, and a pane of it attaches through the same address.
func TestRemoteHost(t *testing.T) {
	a := testApp(t)
	addr := aRemoteHylad(t, "s3cret-pass", remoteState())
	if err := a.addRemoteHost("studio", "tcp://"+addr, "s3cret-pass", ""); err != nil {
		t.Fatal(err)
	}
	h := a.hostNamed("studio")
	if h == nil || len(a.hosts) != 2 {
		t.Fatalf("hosts %+v", a.hosts)
	}
	// The app connects, as its window's loop would: the test drives it.
	cl, err := client.ConnectVia(h.dialer())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	a.connected(h, h.gen, cl, nil)
	var st proto.State
	if err := cl.Call("state", nil, &st); err != nil {
		t.Fatal(err)
	}
	// As setState does, without the badge the app's own loop takes: a test
	// has none.
	h.loaded = true
	a.mergeState(h, st)
	if got := a.sessionsOf(h); len(got) != 1 || got[0].ID != "rs" {
		t.Fatalf("the machine holds %+v", got)
	}
	// Its session is the machine's, not this one's: what the app sends it
	// goes there, and its panes attach there.
	if a.hostOf("rs") != h || a.hostOf("rpp") != h || a.hostOf("rp") != h {
		t.Error("the session is not the machine's")
	}
	if a.hostOf("s1") == h {
		t.Error("a session of this machine is the machine's")
	}
	stream, err := client.AttachVia(h.dialer(), "rpp")
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()

	// The sidebar shows it under the machine's name.
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	for _, want := range []string{"studio", "remote app", "remote shell"} {
		if !tt.HasText(want) {
			t.Errorf("the sidebar shows no %q; texts %q", want, tt.Texts())
		}
	}
	// It goes as the user removes it.
	a.removeRemoteHost("studio")
	if len(a.hosts) != 1 || a.hostNamed("studio") != nil {
		t.Errorf("hosts %+v", a.hosts)
	}
	if a.hostOf("rs") != a.local() {
		t.Error("a session of the machine stayed")
	}
}

// A machine that answers with another password, or none, keeps this one's
// sessions alone: nothing of it shows.
func TestRemoteHostWrongPassword(t *testing.T) {
	a := testApp(t)
	addr := aRemoteHylad(t, "s3cret-pass", remoteState())
	if err := a.addRemoteHost("studio", addr, "wrong", ""); err != nil {
		t.Fatal(err)
	}
	h := a.hostNamed("studio")
	if _, err := client.ConnectVia(h.dialer()); err == nil {
		t.Fatal("a wrong password connected")
	}
	if got := a.sessionsOf(h); len(got) != 0 {
		t.Errorf("the machine holds %+v", got)
	}
}

// Settings › Tailcat lists the machines added, and takes another's
// address and password.
func TestRemoteHostsSettings(t *testing.T) {
	a := testApp(t)
	a.prefs.RemoteHosts = []remoteHost{{Name: "studio", Addr: "192.168.1.5:7297", Token: "s3cret-pass"}}
	a.ss.page = "hosts"
	tt := ui.NewTester(a.settingsView, 820, 900)
	tt.Frame()
	for _, want := range []string{"studio", "192.168.1.5:7297"} {
		if !tt.HasText(want) {
			t.Errorf("Settings shows no %q; texts %q", want, tt.Texts())
		}
	}
}

// The form takes an address and a password, and names the machine for the
// address when the user does not.
func TestAddRemoteHost(t *testing.T) {
	a := testApp(t)
	if err := a.addRemoteHost("", "192.168.1.5:7297", "s3cret-pass", ""); err != nil {
		t.Fatal(err)
	}
	if h := a.hostNamed("192.168.1.5"); h == nil {
		t.Fatalf("hosts %+v", a.prefs.RemoteHosts)
	}
	// A Tailcat machine comes as its pair code, or as the code a phone
	// scans, and its port is the listener's; Tailcat off reaches none.
	a.set.Tailcat = true
	k, err := remote.NewTailcatKey()
	if err != nil {
		t.Fatal(err)
	}
	k.Region = "1" // a region of its own: this test needs no relay
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	code, err := remote.TailcatAddress(ctx, k)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.addRemoteHost("relay", code, "", ""); err != nil {
		t.Fatal(err)
	}
	if h := a.hostNamed("relay"); h == nil || !h.tc || h.code != code || h.port == "" || h.addr != "tailcat://"+code {
		t.Errorf("the pair code is %+v", h)
	}
	if err := a.addRemoteHost("", "hyla://pair?code="+code+"&name=scanned", "", "7400"); err != nil {
		t.Fatal(err)
	}
	if h := a.hostNamed("scanned"); h == nil || h.code != code || h.port != "7400" {
		t.Errorf("the scanned code is %+v", h)
	}
	// A whole code pasted in the address field takes its password.
	if err := a.addRemoteHost("code", "tcp://192.168.1.7:7297?token=s3cret-pass", "", ""); err != nil {
		t.Fatal(err)
	}
	if h := a.hostNamed("code"); h == nil || h.addr != "192.168.1.7:7297" || h.token != "s3cret-pass" {
		t.Errorf("the code is %+v", h)
	}
	for _, c := range []struct{ name, addr, token string }{
		{"studio", "192.168.1.5", "s3cret-pass"},
		{"studio", "192.168.1.5:7297", ""},
		{"192.168.1.5", "192.168.1.6:7297", "s3cret-pass"},
	} {
		if err := a.addRemoteHost(c.name, c.addr, c.token, ""); err == nil {
			t.Errorf("%+v was taken", c)
		}
	}
}
