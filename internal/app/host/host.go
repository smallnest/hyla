// Package host is hylad as the app has it: one per machine, this
// machine's first, and the others reached by an address and a password,
// over a tcp:// listener, or by a tailcat pair code. A host holds its
// connection, what went wrong with it, and what its hylad says of it.
package host

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"hyla/internal/buildinfo"
	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/shell"
	"hyla/internal/update"
)

// State is how the app stands with a host's hylad.
type State int

const (
	// StateConnecting is a connection attempt under way, or one not made.
	StateConnecting State = iota
	// StateReady is a connected hylad.
	StateReady
	// StateVersion is a hylad of another version, which the app does not
	// drive.
	StateVersion
	// StateFailed is a connection that failed.
	StateFailed
)

// Host is hylad, as the app has it: its connection, and what it holds.
// This machine's is first; the others are reached by an address and a
// password, over a tcp:// listener.
type Host struct {
	Name        string // "" for this machine
	Addr, Token string // a remote machine's tcp:// address and password
	// TC is a machine reached by a tailcat pair code instead: Code is the
	// code, and Port the listener's. BT is a machine reached through a
	// BeaconTower encrypted relay: Code holds the bt1_... pair code.
	TC   bool
	BT   bool
	Code string
	Port string

	Client    *client.Client
	Conn      State
	Err       string // why the last connection failed
	DaemonApp string // the version of Hyla hylad runs
	DaemonFP  string // what its code is made of (proto.Welcome.Build)
	Info      proto.HostInfo
	Retries   int
	Gen       int  // connection attempts: the results of one another followed are dropped
	Dialing   bool // a connection attempt is under way
	Hylad     string
	Loaded    bool // a state came
	// Convs are the conversations hylad remembers, for the sidebar, as of
	// its count of their changes ConvGen; ConvAsked tells that they're
	// asked for.
	Convs     []proto.Conversation
	ConvGen   uint64
	ConvKnown bool
	ConvAsked bool
	ViewSent  *string
}

// relaunchedEnv is the environment a relaunched app finds, which says it
// went into that Hyla already: a loop of relaunches stops there.
const relaunchedEnv = "HYLA_RELAUNCHED"

// RelaunchFor relaunches the app into the Hyla hylad runs, when it is
// newer, which a remote machine's update leaves it in.
func RelaunchFor(v string) {
	app := mygo.App.Version()
	if !released(v) || !released(app) || update.Compare(v, app) <= 0 ||
		!mygo.Updater.Enabled() || os.Getenv(relaunchedEnv) == v {
		return
	}
	os.Setenv(relaunchedEnv, v)
	mygo.App.Relaunch()
}

// released tells a version of Hyla that has a release to install: not a
// development build's.
func released(v string) bool { return v != "" && !strings.Contains(v, "dev") && !mygo.IsDev() }

// Remote reports whether h is another machine's.
func (h *Host) Remote() bool { return h.Name != "" }

// Label names the host in words: this machine, which has no name of its
// own.
func (h *Host) Label() string {
	if h.Name == "" {
		return "local"
	}
	return h.Name
}

// ShortDir is a directory as the sidebar and the tooltips show it, from
// this machine's home; another machine's as it is there.
func (h *Host) ShortDir(dir string) string {
	if h.Remote() {
		return dir
	}
	return proto.ShortDir(dir)
}

// JoinPath makes a path of this machine, whose separator is its own.
func (h *Host) JoinPath(parts ...string) string { return filepath.Join(parts...) }

// Windows reports whether this machine is one of Windows: a path is
// absolute there only with a drive, and its programs are PowerShell's.
func (h *Host) Windows() bool { return runtime.GOOS == "windows" }

// Dialer opens connections to hylad: this machine's socket, or another
// machine's tcp:// listener, with its password.
func (h *Host) Dialer() client.Dialer {
	switch {
	case !h.Remote():
		return client.Local{NoStart: true}
	case h.BT:
		return beaconTowerDialer{code: h.Code}
	case h.TC:
		return tailcatDialer{code: h.Code, port: h.Port}
	}
	return tcpDialer{addr: h.Addr, token: h.Token}
}

// Stale reports whether hylad runs an older Hyla than the app: what this
// machine's hylad is, before it is restarted into the app's.
func (h *Host) Stale() bool {
	if h.Remote() || h.Conn != StateReady || h.DaemonApp == "" {
		return false
	}
	return buildinfo.Stale(h.DaemonApp, h.DaemonFP)
}

// ID tells a hylad apart from any other: its machine, its pid and when it
// started.
func ID(cl *client.Client) string {
	name, _ := os.Hostname()
	return fmt.Sprintf("%s/%d/%d", name, cl.Welcome.PID, cl.Welcome.Started)
}

// EditorCommand edits a file in a pane, with the editor this machine's
// shell names, $VISUAL or $EDITOR, else vi, or on Windows edit.
func EditorCommand(h *Host, file string) []string {
	if !h.Windows() {
		return []string{"sh", "-c", `exec ${VISUAL:-${EDITOR:-vi}} "$1"`, "sh", file}
	}
	return []string{"powershell.exe", "-NoProfile", "-Command",
		`$e = if ($env:VISUAL) { $env:VISUAL } elseif ($env:EDITOR) { $env:EDITOR } else { 'edit' }; Invoke-Expression ($e + ' ' + ` + shell.PSQuote(shell.PSQuote(file)) + `)`}
}

// PastedPath writes a path of a host as one word of its shells: of a
// POSIX shell, else, on Windows, of PowerShell, the shell of its panes:
// in single quotes, in which nothing expands, unless it is all letters,
// digits and . _ - : \ /.
func PastedPath(h *Host, p string) string {
	if !h.Windows() {
		return shellEscape(p)
	}
	for _, r := range p {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(`._-:\/`, r)) {
			return shell.PSQuote(p)
		}
	}
	return p
}

// shellEscape writes a path as one word of a POSIX shell: backslashed
// where a shell would read a character as its own.
func shellEscape(p string) string {
	var b strings.Builder
	for _, r := range p {
		safe := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-+@%:,=", r) || r > 0x7f
		if !safe {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// tcpDialer opens connections to a machine's tcp:// listener: the
// password first, which the listener admits, then Hyla's own protocol.
type tcpDialer struct {
	addr, token string
}

// tailcatDialer reaches a machine by its pair code, through the relay the
// code names: the port is its listener's, which carries Hyla's protocol.
type tailcatDialer struct{ code, port string }

func (d tailcatDialer) Dial() (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return remote.OpenTailcat(ctx, d.code, d.port)
}

// beaconTowerDialer reaches a machine through its BeaconTower encrypted relay
// pool using the self-contained bt1_... pair code.
type beaconTowerDialer struct{ code string }

func (d beaconTowerDialer) Dial() (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return remote.OpenBeaconTower(ctx, d.code)
}

func (d tcpDialer) Dial() (net.Conn, error) {
	c, err := net.DialTimeout("tcp", d.addr, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if _, err := c.Write([]byte(d.token + "\n")); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
