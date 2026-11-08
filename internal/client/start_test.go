//go:build !windows

package client

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"hyla/internal/proto"
)

// hyladOfTest is a hylad a test starts in place of startOne, which hylad
// is this executable, not a test binary: it listens on hylad's socket and
// answers as hylad does, with Started 42, so that a test tells it from
// another.
type hyladOfTest struct {
	ln     net.Listener
	starts atomic.Int32
}

// start makes startOne the test's hylad.
func (h *hyladOfTest) start(t *testing.T) {
	t.Helper()
	old := startOne
	startOne = func() error {
		h.starts.Add(1)
		if h.ln != nil {
			return nil
		}
		ln, err := net.Listen("unix", proto.SocketPath())
		if err != nil {
			return err
		}
		h.ln = ln
		go h.answer()
		return nil
	}
	t.Cleanup(func() {
		startOne = old
		if h.ln != nil {
			h.ln.Close()
		}
	})
}

func (h *hyladOfTest) answer() {
	for {
		c, err := h.ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			c.Write([]byte(`{"version":` + strconv.Itoa(proto.Version) + `,"pid":` + strconv.Itoa(os.Getpid()) + `,"started":42}` + "\n"))
			buf := make([]byte, 1024)
			for {
				if _, err := c.Read(buf); err != nil {
					return
				}
			}
		}()
	}
}

// shortHome sets hylad's directory, and a socket under /tmp, which a
// Unix socket's address holds: a test's temporary path is longer.
func shortHome(t *testing.T) {
	t.Helper()
	t.Setenv("HYLA_HOME", t.TempDir())
	dir, err := os.MkdirTemp("/tmp", "hyla-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("HYLA_SOCKET", filepath.Join(dir, "s"))
}

// hyladStartWait shortens the wait, as a hylad of a test answers at once.
func hyladStartWait(t *testing.T) {
	t.Helper()
	old := startWait
	startWait = 100 * time.Millisecond
	t.Cleanup(func() { startWait = old })
}

// A hylad that holds its lock and answers none, as one whose socket went,
// is asked to stop, saving its sessions, and the hylad that starts in its
// place answers: opening Hyla starts hylad, not merely starts it when
// none runs.
func TestDialStopsAHyladWhoseSocketWent(t *testing.T) {
	hyladStartWait(t)
	// It goes as an interrupted hylad does, saving its sessions.
	d := fakeDaemon(t, `trap "exit 0" TERM; while :; do sleep 1; done`)
	new := &hyladOfTest{}
	new.start(t)

	// Its socket went: the app finds no one to connect to, and the lock
	// still names it.
	d.ln.Close()                  // its socket goes with it
	os.Remove(proto.SocketPath()) // when it did not
	cl, err := Connect(false)
	if err != nil {
		t.Fatalf("connecting past a hylad whose socket went: %v", err)
	}
	defer cl.Close()
	if cl.Welcome.Started != 42 {
		t.Errorf("connected to %+v, want the hylad that started", cl.Welcome)
	}
	if !d.gone() {
		t.Error("the hylad whose socket went runs")
	}
	if n := new.starts.Load(); n != 1 {
		t.Errorf("started %d hylads, want once", n)
	}
}

// A hylad that answers is the app's: none starts in its place.
func TestDialLeavesAHyladThatAnswers(t *testing.T) {
	hyladStartWait(t)
	d := fakeDaemon(t, "sleep 600")
	new := &hyladOfTest{}
	new.start(t)

	cl, err := Connect(false)
	if err != nil {
		t.Fatalf("connecting to the hylad that runs: %v", err)
	}
	defer cl.Close()
	if cl.Welcome.PID != d.cmd.Process.Pid {
		t.Errorf("connected to %+v, want the hylad that runs, pid %d", cl.Welcome, d.cmd.Process.Pid)
	}
	if n := new.starts.Load(); n != 0 {
		t.Errorf("started %d hylads beside the one that runs", n)
	}
}

// With no hylad running, as after a reboot, one starts: the app's launch
// brings hylad up.
func TestDialStartsAHylad(t *testing.T) {
	shortHome(t)
	hyladStartWait(t)
	new := &hyladOfTest{}
	new.start(t)

	cl, err := Connect(false)
	if err != nil {
		t.Fatalf("connecting with no hylad: %v", err)
	}
	defer cl.Close()
	if cl.Welcome.Started != 42 {
		t.Errorf("connected to %+v, want the hylad that started", cl.Welcome)
	}
	if n := new.starts.Load(); n != 1 {
		t.Errorf("started %d hylads, want once", n)
	}
}

// A lock that names a process which is not hylad stops none: an unrelated
// program is not killed for a hylad that is gone.
func TestDialLeavesAProcessThatIsNotHylad(t *testing.T) {
	shortHome(t)
	hyladStartWait(t)
	new := &hyladOfTest{}
	new.start(t)

	// This test process holds the lock, as a process the lock names that
	// is not hylad.
	f, err := os.OpenFile(filepath.Join(proto.Dir(), "hylad.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0); err != nil {
		t.Fatal(err)
	}

	_, err = Connect(false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("connecting: %v, want the process the lock names left alone, and the user told", err)
	}
	if n := new.starts.Load(); n != 0 {
		t.Errorf("started %d hylads beside the process the lock names", n)
	}
	if !Running(os.Getpid()) {
		t.Fatal("the test process is gone")
	}
}

// socketDead reads a socket as dead when its file went, and as alive when
// it is a socket that refuses no connection of this user's.
func TestSocketDead(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "gone")
	if !socketDead(gone, os.ErrNotExist) {
		t.Error("a socket that went is not dead")
	}
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !socketDead(plain, errors.New("not a socket")) {
		t.Error("a file that is no socket is not dead")
	}
	sock := filepath.Join(dir, "s")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if socketDead(sock, errors.New("in use")) {
		t.Error("a socket that listens is dead")
	}
}
