package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// TestProfile draws the window as Hyla does, over a terminal that a shell
// writes to, and writes where the CPU and the memory go, into $HYLA_PROFILE
// (testdata/profiles when it is set-only, scratchpad): the frames of an
// agent's output are the app's most of its cost.
//
//	HYLA_PROFILE=/tmp/prof go test -run TestProfile -count=1 .
//	go tool pprof -top -alloc_space /tmp/prof/heap.pprof
func TestProfile(t *testing.T) {
	dir := os.Getenv("HYLA_PROFILE")
	if dir == "" {
		t.Skip("HYLA_PROFILE is not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := testApp(t)
	// The pane the window shows, as the app attaches it.
	term, err := terminal.New(terminal.Options{Conn: idleConn{make(chan struct{})}})
	if err != nil {
		t.Fatal(err)
	}
	a.terms["a"] = &termEntry{term: term}
	t.Cleanup(func() { term.Close() })
	tt := ui.NewTester(a.view, 1180, 760)
	tt.Frame()

	// A shell's output, as an agent's: colored lines, a growing screen.
	line := "\x1b[32mhyla\x1b[0m \x1b[1mprofile\x1b[0m — %03d %s\n"
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			term.Feed([]byte(fmt.Sprintf(line, i, strings.Repeat("work ", 8))))
			time.Sleep(time.Millisecond)
		}
	}()

	cpu, err := os.Create(filepath.Join(dir, "cpu.pprof"))
	if err != nil {
		t.Fatal(err)
	}
	defer cpu.Close()
	// Its own, unless go test profiles the run (-cpuprofile).
	own := pprof.StartCPUProfile(cpu) == nil
	end := time.Now().Add(10 * time.Second)
	for time.Now().Before(end) {
		tt.Frame()
		time.Sleep(16 * time.Millisecond)
	}
	if own {
		pprof.StopCPUProfile()
	}

	for _, name := range []string{"heap", "allocs"} {
		f, err := os.Create(filepath.Join(dir, name+".pprof"))
		if err != nil {
			t.Fatal(err)
		}
		if err := pprof.Lookup(name).WriteTo(f, 0); err != nil {
			t.Error(err)
		}
		f.Close()
	}
	shot(t, tt, "profile")
	t.Logf("%d goroutines; profiles in %s", runtime.NumGoroutine(), dir)
}
