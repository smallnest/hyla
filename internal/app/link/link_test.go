package link

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/proto"
)

type stubApp struct {
	mu         sync.Mutex
	machines   []*host.Host
	dropped    int
	forgotten  int
	invalid    int
	reported   int
	reselected int
}

func (s *stubApp) MachinesNow() []*host.Host           { return s.machines }
func (s *stubApp) UpdateNow(fn func())                 { fn() }
func (s *stubApp) HostOf(string) *host.Host            { return s.machines[0] }
func (s *stubApp) Local() *host.Host                   { return s.machines[0] }
func (s *stubApp) ReportViewNow()                      { s.mu.Lock(); s.reported++; s.mu.Unlock() }
func (s *stubApp) TakeStateOf(*host.Host, proto.State) {}
func (s *stubApp) ReselectNow()                        { s.mu.Lock(); s.reselected++; s.mu.Unlock() }
func (s *stubApp) SelectSession(string)                {}
func (s *stubApp) DropHostTerms(*host.Host)            { s.mu.Lock(); s.dropped++; s.mu.Unlock() }
func (s *stubApp) StatusesForget(*host.Host)           { s.mu.Lock(); s.forgotten++; s.mu.Unlock() }
func (s *stubApp) InvalidateHosts()                    { s.mu.Lock(); s.invalid++; s.mu.Unlock() }
func (s *stubApp) EventOf(*host.Host, *client.Client, proto.Message) {
}

type pipeDialer struct {
	started int64
}

func (d pipeDialer) Dial() (net.Conn, error) {
	c1, c2 := net.Pipe()
	go func() {
		defer c2.Close()
		r := bufio.NewReader(c2)
		if _, err := r.ReadString('\n'); err != nil {
			return
		}
		w := proto.Welcome{Version: proto.Version, PID: os.Getpid(), Started: d.started}
		b, _ := json.Marshal(w)
		c2.Write(append(b, '\n'))
		io.Copy(io.Discard, r)
	}()
	return c1, nil
}

func connectStubClient(t *testing.T, started int64) *client.Client {
	t.Helper()
	cl, err := client.ConnectVia(pipeDialer{started: started})
	if err != nil {
		t.Fatalf("ConnectVia: %v", err)
	}
	t.Cleanup(func() { cl.Close() })
	return cl
}

func TestDisconnectedDuringInFlightDialDoesNotDeadlock(t *testing.T) {
	h := &host.Host{Conn: host.StateConnecting}
	a := &stubApp{machines: []*host.Host{h}}

	cl1 := connectStubClient(t, 1)
	h.Gen++
	Connected(a, h, h.Gen, cl1, nil)
	if h.Conn != host.StateReady || h.Client != cl1 {
		t.Fatalf("initial connect: conn=%v client=%v", h.Conn, h.Client)
	}

	// Simulate RestartEnded -> ConnectHost while cl1 is still recorded as h.Client.
	h.Gen++
	dialGen := h.Gen
	h.Dialing = true

	// Late cl1 disconnect callback arrives before the in-flight dial finishes.
	disconnected(a, h, cl1)
	if h.Client != nil {
		t.Fatalf("h.Client = %v; want nil", h.Client)
	}
	if h.Gen != dialGen {
		t.Fatalf("h.Gen = %d; want %d (must not bump during active dial)", h.Gen, dialGen)
	}
	if !h.Dialing {
		t.Fatal("h.Dialing became false before in-flight dial completed")
	}

	// In-flight dial completes with cl2 and must be accepted, not discarded.
	cl2 := connectStubClient(t, 2)
	Connected(a, h, dialGen, cl2, nil)
	if h.Conn != host.StateReady || h.Client != cl2 || h.Dialing {
		t.Fatalf("after dial complete: conn=%v client=%v dialing=%v", h.Conn, h.Client, h.Dialing)
	}
	select {
	case <-cl2.Done():
		t.Fatal("cl2 was unexpectedly closed")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestResetDetachesAndIgnoresStaleDisconnect(t *testing.T) {
	h := &host.Host{Conn: host.StateConnecting}
	a := &stubApp{machines: []*host.Host{h}}

	cl1 := connectStubClient(t, 1)
	h.Gen++
	Connected(a, h, h.Gen, cl1, nil)

	Reset(a, h)
	if h.Client != nil || h.Conn != host.StateConnecting || h.Dialing {
		t.Fatalf("after Reset: conn=%v client=%v dialing=%v", h.Conn, h.Client, h.Dialing)
	}
	if a.dropped == 0 || a.forgotten == 0 {
		t.Fatalf("Reset did not drop terms (%d) or forget statuses (%d)", a.dropped, a.forgotten)
	}
	select {
	case <-cl1.Done():
	case <-time.After(time.Second):
		t.Fatal("cl1 was not closed by Reset")
	}

	genAfterReset := h.Gen
	// Late disconnect for cl1 must be ignored because Reset already cleared h.Client.
	disconnected(a, h, cl1)
	if h.Gen != genAfterReset {
		t.Fatalf("stale disconnected bumped h.Gen from %d to %d", genAfterReset, h.Gen)
	}

	// Subsequent ConnectHost + Connected succeeds cleanly.
	h.Gen++
	dialGen := h.Gen
	h.Dialing = true
	cl2 := connectStubClient(t, 2)
	Connected(a, h, dialGen, cl2, nil)
	if h.Conn != host.StateReady || h.Client != cl2 || h.Dialing {
		t.Fatalf("after reconnect: conn=%v client=%v dialing=%v", h.Conn, h.Client, h.Dialing)
	}
}
