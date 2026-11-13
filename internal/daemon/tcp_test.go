package daemon

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	hylaclient "hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/settings"
)

// passDialer is a client of a tcp:// listener: the password, then Hyla's
// own protocol, as the app reaches another machine.
type passDialer struct {
	addr, token string
}

func (p passDialer) Dial() (net.Conn, error) {
	c, err := net.DialTimeout("tcp", p.addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if _, err := c.Write([]byte(p.token + "\n")); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// tcpListenerFor starts a tcp:// listener on a port of loopback, with the
// password token, and returns its address.
func tcpListenerFor(t *testing.T, d *Daemon, token string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_CONFIG", filepath.Join(home, "config"))
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()

	set := settings.Load()
	set.Listen = []string{fmt.Sprintf("tcp://127.0.0.1:%d", port)}
	set.ListenToken = token
	if err := settings.Save(set); err != nil {
		t.Fatal(err)
	}
	settings.Reload()
	l := &listeners{d: d, on: map[string]*listener{}}
	t.Cleanup(l.close)
	l.set(set.Listen)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for end := time.Now().Add(5 * time.Second); ; {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			c.Close()
			break
		}
		if time.Now().After(end) {
			t.Fatalf("listening on %s: %v", addr, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return addr
}

// A tcp:// listener takes Hyla's own protocol, with the password of the
// configuration, and refuses a connection without it.
func TestTCPListenerHyla(t *testing.T) {
	d := PocketForTest(log.New(io.Discard, "", 0))
	d.clients = map[*proto.ClientInfo]struct{}{}
	t.Cleanup(d.stop)
	addr := tcpListenerFor(t, d, "s3cret-pass")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := hylaclient.ConnectVia(passDialer{addr, "s3cret-pass"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Welcome.Version != proto.Version {
		t.Errorf("welcome %+v", c.Welcome)
	}
	var st proto.State
	if err := c.Call("state", nil, &st); err != nil {
		t.Fatal(err)
	}
	if _, err := hylaclient.AttachVia(passDialer{addr, "s3cret-pass"}, "nope"); err == nil {
		t.Error("a pane of no session attached")
	}
	_ = ctx

	// A password that is not the configuration's opens nothing.
	if _, err := hylaclient.ConnectVia(passDialer{addr, "wrong"}); err == nil {
		t.Error("a wrong password reached hylad")
	}
}

// HylaPocket answers on the same listener, by its HTTP paths, and only
// with the password in its Authorization header.
func TestTCPListenerPocket(t *testing.T) {
	d := PocketForTest(log.New(io.Discard, "", 0))
	t.Cleanup(d.stop)
	addr := tcpListenerFor(t, d, "s3cret-pass")

	get := func(auth string) *http.Response {
		t.Helper()
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		req := "GET /pocket/api/control HTTP/1.1\r\nHost: " + addr + "\r\n"
		if auth != "" {
			req += "Authorization: Bearer " + auth + "\r\n"
		}
		req += "Connection: close\r\n\r\n"
		if _, err := c.Write([]byte(req)); err != nil {
			t.Fatal(err)
		}
		res, err := http.ReadResponse(bufio.NewReader(c), nil)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := get(""); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("without the password: %d", res.StatusCode)
	}
	if res := get("wrong"); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("a wrong password: %d", res.StatusCode)
	}
	// The password lets the request through to HylaPocket's own server,
	// which wants a WebSocket here: not what this test sends.
	if res := get("s3cret-pass"); res.StatusCode == http.StatusUnauthorized {
		t.Errorf("the password was refused: %d", res.StatusCode)
	}
	// A path of Hyla's own is no part of the pocket server.
	{
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		fmt.Fprintf(c, "GET /hyla/mux HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer s3cret-pass\r\nConnection: close\r\n\r\n", addr)
		res, err := http.ReadResponse(bufio.NewReader(c), nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("a path of no pocket: %d", res.StatusCode)
		}
	}
}

// A connection that starts as HTTP is HylaPocket's, and one that starts
// with a password is Hyla's.
func TestLooksHTTP(t *testing.T) {
	for line, want := range map[string]bool{
		"GET /pocket/api/control HTTP/1.1\r\n": true,
		"POST /x HTTP/1.0\r\n":                 true,
		"OPTIONS * HTTP/1.1\r\n":               true,
		"secret-password\n":                    false,
		"{\"kind\":\"control\"}\n":             false,
	} {
		if got := looksHTTP(line); got != want {
			t.Errorf("looksHTTP(%q) = %v", strings.TrimSpace(line), got)
		}
	}
}
