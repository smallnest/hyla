package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/settings"

	"hyla/locales"
)

// withHost adds a remote host to a test app, connected, with a session of
// its own.
func withHost(t *testing.T, a *App, name string) *host {
	t.Helper()
	addr, _ := remote.ParseAddr("me@" + name)
	h := &host{name: name, cfg: settings.Host{Name: name, Addr: addr.String()}, addr: addr, conn: connReady,
		info: proto.HostInfo{Hostname: name, OS: "linux", Arch: "amd64", Home: "/home/me", Hyla: "2.1.0", Server: true}}
	a.hosts = append(a.hosts, h)
	a.set.Hosts = append(a.set.Hosts, h.cfg)
	a.applied = a.set.Clone()
	h.loaded = true
	// As setState takes it, but for the Dock's badge, which needs the app running.
	a.mergeState(h, proto.State{
		Projects: []proto.Project{{ID: "rp-" + name, Name: "api", Dir: "/home/me/api"}},
		Sessions: []proto.Session{{
			ID: "rs-" + name, Project: "rp-" + name, Name: "agent on " + name,
			Pane:  proto.Pane{ID: "rpa-" + name, Dir: "/home/me/api", Status: proto.Waiting, Progress: -1},
			Panes: []proto.Pane{{ID: "rpa-" + name, Dir: "/home/me/api", Status: proto.Waiting, Progress: -1}},
		}},
	})
	return h
}

// Each host's state replaces its own part of what the app shows, in the
// hosts' order, and leaves the others', with what the app changed in them
// before their hylads' states come.
func TestHostsMergeState(t *testing.T) {
	a := testApp(t)
	local := a.local()
	a.mergeState(local, a.st)
	build := withHost(t, a, "build")
	if got := len(a.st.Sessions); got != 2 {
		t.Fatalf("%d sessions", got)
	}
	if a.st.Sessions[0].ID != "s1" || a.st.Sessions[1].ID != "rs-build" {
		t.Errorf("order %s, %s", a.st.Sessions[0].ID, a.st.Sessions[1].ID)
	}
	for id, want := range map[string]*host{"s1": local, "a": local, "p1": local, "rs-build": build, "rpa-build": build, "rp-build": build, "unknown": local} {
		if a.hostOf(id) != want {
			t.Errorf("hostOf(%s) = %q", id, a.hostOf(id).name)
		}
	}
	// A rename the app shows at once stays as this machine's state comes.
	a.session("rs-build").Name = "renamed"
	a.mergeState(local, proto.State{Projects: a.projectsOf(local), Sessions: a.sessionsOf(local)})
	if s := a.session("rs-build"); s == nil || s.Name != "renamed" {
		t.Errorf("the remote session after a local state: %+v", s)
	}
	// A session the app made on a host shows before the host says so.
	a.addPending(build, proto.Session{ID: "new", Project: "rp-build", Panes: []proto.Pane{{ID: "np"}}})
	a.mergeState(local, proto.State{Projects: a.projectsOf(local), Sessions: a.sessionsOf(local)})
	if a.session("new") == nil || a.hostOf("np") != build {
		t.Error("a pending session of a host went with this machine's state")
	}
	// A host let go of takes its sessions along.
	a.closeHost(build)
	if a.session("rs-build") != nil || a.session("s1") == nil {
		t.Errorf("after closing the host: %d sessions", len(a.st.Sessions))
	}
}

// The sidebar shows this machine's projects, unlabeled, then each host's
// under its name; a session of a host keeps its host's name in the
// window's title and its notifications.
func TestSidebarHosts(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	withHost(t, a, "build")
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	tt.Frame()
	for _, want := range []string{"demo", "build", "api", "agent on build"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in the sidebar: %q", want, tt.Texts())
		}
	}
	shot(t, tt, "sidebar-hosts")
	ids := a.visibleOrder()
	if len(ids) != 2 || ids[0] != "s1" || ids[1] != "rs-build" {
		t.Errorf("order %q", ids)
	}
	s := a.session("rs-build")
	if got := a.hostedName(s); got != "agent on build · build" {
		t.Errorf("hosted name %q", got)
	}
	if got := a.hostedName(a.session("s1")); got != "demo" {
		t.Errorf("this machine's session is named %q", got)
	}
	// A remote group folds as this machine's do.
	a.toggleProject("rp-build")
	if ids := a.visibleOrder(); len(ids) != 1 {
		t.Errorf("folded: %q", ids)
	}
}

// The sidebar is flat: a remote group carries its host's name, and the
// host has a row of its own only while it wants the user, as when it can't
// be reached; one with no sessions shows its name and the address to
// reach it at, so that a host added from the sidebar shows there.
func TestSidebarFlat(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	build := withHost(t, a, "build")
	empty := withHost(t, a, "empty")
	a.mergeState(empty, proto.State{})
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	shot(t, tt, "sidebar-flat")
	// build is named once, by its tag: no header.
	if n := countText(tt.Texts(), "build"); n != 1 {
		t.Errorf("build shows %d times: %q", n, tt.Texts())
	}
	tag, ok := tt.Find("build")
	api, ok2 := tt.Find("api")
	if !ok || !ok2 || abs32(tag.Y+tag.H/2-(api.Y+api.H/2)) > 1 || tag.X <= api.X {
		t.Errorf("build's tag %+v is not after its group's name %+v", tag, api)
	}
	if tt.HasText("No sessions") {
		t.Errorf("this machine claims no sessions: %q", tt.Texts())
	}
	// A host still being reached has a row that says so; one that is there
	// with nothing in it waits for what it tells.
	empty.conn, empty.loaded = connConnecting, false
	if a.hostNeedsRow(build) || !a.hostNeedsRow(empty) {
		t.Error("a host being reached has no row")
	}
	empty.conn, empty.loaded = connReady, false
	if a.hostNeedsRow(empty) {
		t.Error("a host with nothing in it has a row")
	}
	tt.Frame()
	if !tt.HasText("Connecting…") && !tt.HasText("Reconnecting") {
		t.Errorf("a host being reached says nothing: %q", tt.Texts())
	}
	empty.conn = connReady
	// Reconnecting, it has a row that says so, and its groups stay, their
	// tag faded; failing, its row says why.
	build.conn = connConnecting
	if !a.hostNeedsRow(build) {
		t.Error("a host reconnecting has no row")
	}
	build.conn, build.err = connFailed, "Connection refused"
	if !a.hostNeedsRow(build) {
		t.Error("a host the app can't reach has no row")
	}
	tt.Frame()
	if !tt.HasText("Connection refused") || !tt.HasText("agent on build") {
		t.Errorf("failing: %q", tt.Texts())
	}
}

// The sidebar's fold button folds every project at once, and unfolds them
// all once all are folded.
func TestSidebarFoldAll(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	withHost(t, a, "build")
	a.toggleProject("p1")
	if a.allProjectsCollapsed() {
		t.Error("one group folded reads as all")
	}
	a.toggleAllProjects()
	if !a.allProjectsCollapsed() {
		t.Error("fold all left a group open")
	}
	if ids := a.visibleOrder(); len(ids) != 1 {
		t.Errorf("folded: %q", ids)
	}
	a.toggleAllProjects()
	if a.allProjectsCollapsed() {
		t.Error("unfold all left a group folded")
	}
	if ids := a.visibleOrder(); len(ids) != 2 {
		t.Errorf("unfolded: %q", ids)
	}
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText("Collapse All Projects") {
		t.Errorf("no fold button: %q", tt.Texts())
	}
}

// With hosts, Open Project lists any machine's folders, this one's too,
// with a switch between them.
func TestPickerHosts(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	withHost(t, a, "build")
	a.picker = folderPicker{open: true, host: a.local(), list: &proto.FSList{Dir: "/home/me", Entries: []proto.FSEntry{{Name: "api", Dir: true}}}}
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	shot(t, tt, "picker-hosts")
	for _, h := range a.hosts {
		if _, ok := tt.Find(locales.PickerTitle(h.label())); !ok {
			t.Errorf("no switch to %s: %q", h.label(), tt.Texts())
		}
	}
}

// A host the app can't reach says why, and offers what fixes it: Hyla
// installed there, for one that has none.
func TestSidebarHostProblems(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	h := withHost(t, a, "build")
	h.conn, h.err, h.errKind = connFailed, "hyla is not installed on build", remote.ErrNotInstalled
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText("Hyla isn't installed on build") || !tt.HasText("Install Hyla…") {
		t.Errorf("no word of the missing Hyla: %q", tt.Texts())
	}
	h.errKind = remote.ErrAuth
	h.err = "Permission denied (publickey)."
	tt.Frame()
	if !tt.HasText("build refused the connection") {
		t.Errorf("no word of the refusal: %q", tt.Texts())
	}
	shot(t, tt, "sidebar-host-refused")
	// Its sessions show as they were, faded, and their panes say why.
	a.selectSession("rs-build")
	tt.Frame()
	if !tt.HasText("Permission denied (publickey).") {
		t.Errorf("the pane of a lost host: %q", tt.Texts())
	}
	h.conn, h.errKind = connVersion, nil
	h.daemonApp = "1.0.0"
	tt.Frame()
	if !tt.HasText("This app and Hyla on build speak different versions") || !tt.HasText("build runs Hyla 1.0.0") {
		t.Errorf("no word of the version: %q", tt.Texts())
	}
}

// Programs of a host drive the app only as the host's Remote Control
// lets them, whatever this machine's.
func TestRemoteControlPerHost(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	h := withHost(t, a, "build")
	a.set.RemoteControl = true
	if a.remoteControl(h) {
		t.Error("a host drives the app unasked")
	}
	if !a.remoteControl(a.local()) {
		t.Error("this machine's remote control is off")
	}
	h.cfg.RemoteControl = true
	if !a.remoteControl(h) {
		t.Error("a host the user trusts does not drive the app")
	}
}

func TestHostShortDir(t *testing.T) {
	h := &host{name: "b", info: proto.HostInfo{Home: "/home/me"}}
	for in, want := range map[string]string{"/home/me": "~", "/home/me/api": "~/api", "/home/meg": "/home/meg", "/srv": "/srv"} {
		if got := h.shortDir(in); got != want {
			t.Errorf("shortDir(%q) = %q", in, got)
		}
	}
	// A host of Windows: backslashes, and a drive's letter of either case.
	w := &host{name: "w", info: proto.HostInfo{OS: "windows", Home: `C:\Users\me`}}
	for in, want := range map[string]string{`C:\Users\me`: "~", `c:\users\me\api`: `~\api`, `C:\Users\meg`: `C:\Users\meg`, `D:\src`: `D:\src`} {
		if got := w.shortDir(in); got != want {
			t.Errorf("Windows: shortDir(%q) = %q", in, got)
		}
	}
	if got := w.joinPath(`C:\`, "src"); got != `C:\src` {
		t.Errorf("joinPath = %q", got)
	}
	// Quoted for PowerShell, the shell of Windows' panes, in which nothing
	// of a file's name runs.
	for in, want := range map[string]string{
		`C:\Temp\hyla-paste\x\pasted-image.png`: `C:\Temp\hyla-paste\x\pasted-image.png`,
		`C:\Temp\a b.png`:                       `'C:\Temp\a b.png'`,
		`C:\Temp\$(calc).png`:                   `'C:\Temp\$(calc).png'`,
		"C:\\Temp\\it\u2019s.png":               "'C:\\Temp\\it\u2019\u2019s.png'",
	} {
		if got := pastedPath(w, in); got != want {
			t.Errorf("pastedPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// A Windows host's file opens in its editor by name, whatever the name.
func TestEditorCommandWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("runs PowerShell")
	}
	w := &host{name: "w", info: proto.HostInfo{OS: "windows", Home: `C:\Users\me`}}
	for _, name := range []string{`C:\a b\x.txt`, "C:\\x\u2019+(Write-Output PWNED)+\u2019y.txt", `C:\$(Write-Output PWNED)'.txt`} {
		args := editorCommand(w, name)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(os.Environ(), "VISUAL=", "EDITOR=[Console]::OutputEncoding = [Text.Encoding]::UTF8; Write-Output")
		out, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(out)) != name {
			t.Errorf("%q: the editor got %q, %v", name, out, err)
		}
	}
}

func TestShellEscape(t *testing.T) {
	for in, want := range map[string]string{
		"/tmp/hyla-1000/paste/x/pasted-image.png": "/tmp/hyla-1000/paste/x/pasted-image.png",
		"/tmp/a b/it's (1).png":                   `/tmp/a\ b/it\'s\ \(1\).png`,
		"/tmp/$HOME;rm":                           `/tmp/\$HOME\;rm`,
	} {
		if got := shellEscape(in); got != want {
			t.Errorf("shellEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPickerItems(t *testing.T) {
	a := testApp(t)
	linux := &host{name: "b", info: proto.HostInfo{OS: "linux", Home: "/home/me"}}
	a.picker = folderPicker{open: true, host: linux, list: &proto.FSList{Dir: "/home/me", Parent: "/home", Entries: []proto.FSEntry{
		{Name: "api", Dir: true, Repo: true}, {Name: "notes.txt"}, {Name: "web", Dir: true},
	}}}
	var got []string
	for _, it := range a.pickerItems() {
		got = append(got, it.name)
	}
	if strings.Join(got, ",") != "..,api,web" {
		t.Errorf("items %q", got)
	}
	a.picker.query = "wb"
	if items := a.pickerItems(); len(items) != 1 || items[0].path != "/home/me/web" {
		t.Errorf("filtered %+v", items)
	}
	for q, want := range map[string]bool{"~/x": true, "/srv": true, "~": true, "src": false, "~x": false} {
		if isPathQuery(q, false) != want {
			t.Errorf("isPathQuery(%q)", q)
		}
	}
	for q, want := range map[string]bool{`C:\src`: true, "d:": false, `d:/x`: true, `~\x`: true, `\\nas\share`: true, "src": false, "1:": false} {
		if isPathQuery(q, true) != want {
			t.Errorf("Windows: isPathQuery(%q)", q)
		}
	}
	_ = fmt.Sprint()
}

// A split in a remote session is the host's at once, as its terminal
// attaches there before the host's state says so.
func TestRemoteSplitIsTheHosts(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	h := withHost(t, a, "build")
	s := *a.session("rs-build")
	s.Panes = append(s.Panes, proto.Pane{ID: "split-pane"})
	a.replaceSession(h, s)
	if a.hostOf("split-pane") != h {
		t.Error("the new pane of a remote split is this machine's")
	}
}

// A host's program drives the app's actions only on that host's
// sessions, never on this machine's.
func TestRemoteActionsStayOnTheirHost(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	h := withHost(t, a, "build")
	a.selected = "s1" // this machine's
	a.from = h
	defer func() { a.from = nil }()
	for _, id := range []string{"pane.restart", "pane.split.right", "pane.zoom", "changes.toggle"} {
		if a.fromMay(id, nil) {
			t.Errorf("%s from build ran on this machine's session", id)
		}
	}
	if a.fromMay("session.new", map[string]string{"project": "p1"}) {
		t.Error("build started a session in this machine's project")
	}
	if !a.fromMay("session.new", nil) || !a.fromMay("sidebar.toggle", nil) {
		t.Error("an action of build's own refused")
	}
	a.selected = "rs-build"
	if !a.fromMay("pane.split.right", nil) {
		t.Error("build may not split its own session")
	}
	a.from = a.local()
	a.selected = "rs-build"
	if !a.fromMay("pane.split.right", nil) {
		t.Error("this machine's hyla do refused")
	}
}

// A host newer than the app gets no Update, which would take it back.
func TestHostVersions(t *testing.T) {
	h := &host{name: "b", info: proto.HostInfo{Server: true, Updates: true, Hyla: "9.0.0"}}
	if h.behind() {
		t.Error("a newer host is behind")
	}
	h.info.Hyla = "0.1.0"
	h.daemonApp = "0.1.0"
	if h.stale() {
		t.Error("hylad of the bridge's version is stale")
	}
	h.conn, h.daemonApp, h.info.Hyla = connReady, "0.1.0", "0.2.0"
	if !h.stale() {
		t.Error("hylad older than the Hyla installed is not stale")
	}
	h.daemonApp = "0.3.0"
	if h.stale() {
		t.Error("hylad newer than the bridge that answered is stale")
	}
}

// A host of the desktop app that takes updates from here is told of as one
// to update; one that doesn't, as one to update there.
func TestDesktopHostVersionDetail(t *testing.T) {
	a := testApp(t)
	h := &host{name: "pc", info: proto.HostInfo{Hyla: "0.1.0"}}
	if got := a.hostVersionDetail(h); got != locales.HostDesktopDetail("pc", "0.1.0", mygo.App.Version()) {
		t.Errorf("without remote updates: %q", got)
	}
	h.info.Updates = true
	if got := a.hostVersionDetail(h); got != locales.HostVersionDetail("pc", "0.1.0", mygo.App.Version()) {
		t.Errorf("with remote updates: %q", got)
	}
}

// A host of another version is told of by whether Hyla there is older or
// newer than this app: only the older one is updated from here, asking
// first; the newer one, or one that has no version to tell, gets told.
func TestHostVersionWording(t *testing.T) {
	// Of a released Hyla's: only one of those updates a host.
	t.Setenv("MYGO_ENV", "production")
	old := mygo.App.Version()
	mygo.App.SetVersion("3.5.4")
	t.Cleanup(func() { mygo.App.SetVersion(old) })
	a := testApp(t)
	if !released(mygo.App.Version()) {
		t.Skip("a development build updates no host")
	}
	h := withHost(t, a, "build")
	h.conn, h.cl = connVersion, &client.Client{}
	h.info, h.rc = testHost(t, "build")
	h.info.Updates = true

	h.daemonApp = "99.0.0"
	title, detail := a.hostVersion(h)
	if title != locales.HostVersionTitle("build") || detail != locales.HostVersionNewer("build", "99.0.0", mygo.App.Version()) {
		t.Errorf("newer: %q, %q", title, detail)
	}
	a.offerHostUpdate(h)
	if a.confirm != nil {
		t.Errorf("a newer host is asked to update: %q", a.confirm.title)
	}
	if a.pendingToast != detail {
		t.Errorf("toast %q, want the newer detail %q", a.pendingToast, detail)
	}

	// The same app version by another protocol tells of both.
	a.pendingToast = ""
	h.daemonApp = mygo.App.Version()
	if _, got := a.hostVersion(h); got != locales.HostVersionDetail("build", mygo.App.Version(), mygo.App.Version()) {
		t.Errorf("same version: %q", got)
	}
	a.offerHostUpdate(h)
	if a.confirm != nil {
		t.Error("a host of the same version is asked to update")
	}

	// An older Hyla there is updated from here, once asked.
	h.daemonApp, a.pendingToast = "0.0.1", ""
	if _, got := a.hostVersion(h); got != locales.HostVersionOlder("build", "0.0.1", mygo.App.Version()) {
		t.Errorf("older: %q", got)
	}
	a.offerHostUpdate(h)
	if a.confirm == nil || a.confirm.title != locales.HostUpdateTitle("build", mygo.App.Version()) {
		t.Fatalf("an older host is not asked to update: %+v", a.confirm)
	}
	if a.pendingToast != "" {
		t.Errorf("toast %q", a.pendingToast)
	}
	a.confirm = nil
}

// Restarting Hyla on a host says so once it did, and so does the update
// that leads to it.
func TestHostRestartToast(t *testing.T) {
	a := testApp(t)
	h := withHost(t, a, "build")
	h.info, h.rc = testHost(t, "build")
	cl, err := client.ConnectVia(h.rc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	h.cl = cl
	for _, what := range []string{"restart", "update"} {
		var got error
		done := make(chan struct{})
		requestDoneForTest = func(h2 *host, err error) {
			got = err
			close(done)
		}
		a.hostRequest(h, what)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("the request did not end")
		}
		requestDoneForTest = func(*host, error) {}
		if got != nil {
			t.Fatalf("%s failed: %v", what, got)
		}
		want := locales.HostRestarted("build")
		if what == "update" {
			want = locales.HostUpdated("build", mygo.App.Version())
		}
		if got := a.pendingToast; got != want {
			t.Errorf("after %s: toast %q, want %q", what, got, want)
		}
		a.pendingToast = ""
	}
}

// newTestLink is a remote connection to the server of a test: what a
// remote connection is, over a socket of the test's, as the app dials it.
func newTestLink(t *testing.T, path string) remoteLink {
	t.Helper()
	l := &testLink{path: path}
	l.conn = newTestConn(t, path)
	l.done = make(chan struct{})
	return l
}

type testLink struct {
	path string
	conn *testConn
	done chan struct{}
}

func (l *testLink) Dial() (net.Conn, error) { return l.Open() }

func (l *testLink) Open() (net.Conn, error) {
	st, err := l.conn.sess.Open()
	if err != nil {
		return nil, err
	}
	return &testStream{Conn: st, r: bufio.NewReader(st)}, nil
}

func (l *testLink) Close() error              { l.conn.sess.Close(); return nil }
func (l *testLink) Welcome() proto.MuxWelcome { return l.conn.w }

func (l *testLink) Host(ctx context.Context, req proto.HostRequest) error {
	st, err := l.Open()
	if err != nil {
		return err
	}
	defer st.Close()
	b, _ := json.Marshal(req)
	if _, err := st.Write(append(b, '\n')); err != nil {
		return err
	}
	line, err := bufio.NewReader(st).ReadString('\n')
	if err != nil {
		return err
	}
	var r proto.HostReply
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		return err
	}
	if !r.OK {
		return errors.New(r.Error)
	}
	return nil
}
func (l *testLink) Done() <-chan struct{} { return l.done }

// testStream is a stream whose first reply line was read already.
type testStream struct {
	net.Conn
	r *bufio.Reader
}

func (s *testStream) Read(b []byte) (int, error) { return s.r.Read(b) }

// testHost is a hylad of a test, at a socket of the test's, which takes
// the host requests a remote connection asks, and a connection to it. It
// runs this app's version.
func testHost(t *testing.T, name string) (proto.HostInfo, remoteLink) {
	t.Helper()
	s := &remote.Server{
		Info: func() proto.HostInfo {
			return proto.HostInfo{Hostname: name, OS: "linux", Hyla: mygo.App.Version(), Server: true, Updates: true}
		},
		Stream: func(conn net.Conn, _ []byte, _ proto.MuxHello) { serveControl(conn, nil) },
		// It runs a released Hyla, which this app updates, and restarts.
		Update:  func(string) error { return nil },
		Restart: func() (func(), error) { return nil, nil },
	}
	path := filepath.Join(t.TempDir(), "m")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.Serve(conn)
		}
	}()
	link := newTestLink(t, path)
	return s.Info(), link
}

func countText(texts []string, s string) int {
	n := 0
	for _, t := range texts {
		if t == s {
			n++
		}
	}
	return n
}

func TestHostColor(t *testing.T) {
	h := &host{name: "devbox"}
	if hostColorName(h) != hostColorName(&host{name: "DevBox"}) {
		t.Error("the color of a name changes with its case")
	}
	h.cfg.Color = "teal"
	if hostColorName(h) != "teal" {
		t.Error("the configured color is not taken")
	}
	if lightPalette.hostColor(h) == darkPalette.hostColor(h) {
		t.Error("one color for light and dark")
	}
	if lightPalette.hostColor(&host{}) != lightPalette.textMuted {
		t.Error("this machine is colored")
	}
}

// Dark, a remote group's tag and a remote pane's header in its host's
// color ($HYLA_SHOTS).
func TestSidebarFlatDarkShot(t *testing.T) {
	a := testApp(t)
	a.mergeState(a.local(), a.st)
	withHost(t, a, "build").cfg.Color = "teal"
	withHost(t, a, "gpu")
	a.selectSession("rs-build")
	tt := ui.NewTester(a.view, 1100, 700)
	tt.SetDark(true)
	tt.Frame()
	tt.Advance(time.Second) // colors ease from the light frame's
	tt.Frame()
	shot(t, tt, "sidebar-flat-dark")
}

// Removing a host from Settings asks there, not in the main window, and
// removes it once the user says so.
func TestRemoveHostAsksInSettings(t *testing.T) {
	a := testApp(t)
	withHost(t, a, "build")
	a.ss.page = "hosts"
	tt := ui.NewTester(a.settingsView, 820, 640)
	tt.Frame()
	if err := tt.Click(locales.HostRemove()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.confirm != nil || a.ss.confirm == nil {
		t.Fatalf("the question went to the main window: %+v, settings %+v", a.confirm, a.ss.confirm)
	}
	if !tt.HasText(locales.HostRemoveTitle("build")) {
		t.Fatalf("Settings shows no question; texts %q", tt.Texts())
	}
	shot(t, tt, "settings-remove-host")
	if err := tt.Click(locales.CommonRemove()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ss.confirm != nil || len(a.set.Hosts) != 0 {
		t.Errorf("not removed: question %+v, hosts %+v", a.ss.confirm, a.set.Hosts)
	}
}

// Editing a host in Settings renames it in place, keeping its connection
// and sessions; a new address connects again; a name taken is refused.
func TestEditHost(t *testing.T) {
	a := testApp(t)
	build := withHost(t, a, "build")
	withHost(t, a, "gpu")
	a.ss.page = "hosts"
	tt := ui.NewTester(a.settingsView, 820, 640)
	tt.Frame()
	if err := tt.Click(locales.HostEdit()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ss.hostEdit.editing != "build" || !tt.HasText(locales.CommonSave()) {
		t.Fatalf("no editor: %+v; texts %q", a.ss.hostEdit, tt.Texts())
	}

	a.ss.hostEdit.name = "gpu"
	if err := tt.Click(locales.CommonSave()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ss.hostEdit.err != locales.SetHostNameTaken("gpu") {
		t.Fatalf("took a name taken: %+v", a.ss.hostEdit)
	}

	a.ss.hostEdit.name, a.ss.hostEdit.err = "builder", ""
	if err := tt.Click(locales.CommonSave()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.syncHosts()
	if h := a.hostNamed("builder"); h != build || a.set.Host("build") != nil || a.hostOf("rs-build") != build {
		t.Fatalf("renamed to %p, not %p; hosts %+v", h, build, a.set.Hosts)
	}
	if a.ss.hostEdit.editing != "" {
		t.Errorf("still editing %+v", a.ss.hostEdit)
	}

	tt.Frame()
	if err := tt.Click(locales.HostEdit()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.ss.hostEdit.addr = "me@builder.lan"
	if err := tt.Click(locales.CommonSave()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.syncHosts()
	if h := a.hostNamed("builder"); h == nil || h == build || h.addr.String() != "me@builder.lan" {
		t.Fatalf("a new address kept the connection: %+v", h)
	}
}
