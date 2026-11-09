package daemon

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"
)

// profiling lets a hylad that runs from a shell, as one that is built
// into /tmp for a profile, tell where its CPU and memory go, without a
// server of its own: $HYLA_PROFILE names the directory of the profiles
// it writes, and signals ask for them.
//
//	SIGUSR1  starts a CPU profile, into cpu.pprof
//	SIGUSR2  stops it, and writes the heap profile, heap.pprof
//	SIGQUIT  writes the goroutine profile, goroutine.pprof
//
// One of a release has none of this: it is for developing Hyla, and a
// release has not the -X that says it is one.
const profileDirEnv = "HYLA_PROFILE"

// profiler is the CPU profile running, if any, and when the profiles
// last went.
var profiler = profilerT{infosAt: 60 * time.Second}

// profilerT is a profile of the daemon's CPU, while it is taken.
type profilerT struct {
	mu      sync.Mutex
	cpu     *os.File
	infosAt time.Duration // how long after a CPU profile goes, once the CPU is hot, infos go too
	last    time.Time     // when infos last went
}

// startProfiling attaches the signals to the profiles, when
// $HYLA_PROFILE says where to write them.
func startProfiling(lg *log.Logger) {
	dir := os.Getenv(profileDirEnv)
	// One of a release is the user's, and a release to be: no profile of
	// theirs.
	if dir == "" || mygo.IsDev() {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		lg.Printf("profiling: %v", err)
		return
	}
	// Go gives one of SIGQUIT and no other process on macOS can ask for
	// one, so the profiles come on the signals that Go keeps: a SIGUSR2
	// that pprof lets through does nothing, and a SIGUSR1, as Go's
	// pprof starts them, which takes them.
	lg.Printf("profiling into %s: SIGUSR1 takes the CPU profile, and the heap, allocations, blocks and mutexes; SIGUSR2 ends the CPU profile", dir)
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for sig := range sigs {
			lg.Printf("profiling: %v", sig)
			switch sig {
			case syscall.SIGUSR1:
				profiler.startCPU(dir, lg)
			case syscall.SIGUSR2:
				profiler.stop(dir, lg)
			}
		}
	}()
}

func (p *profilerT) startCPU(dir string, lg *log.Logger) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cpu != nil {
		lg.Printf("profiling: the CPU profile is running")
		return
	}
	path := filepath.Join(dir, "cpu.pprof")
	f, err := os.Create(path)
	if err != nil {
		lg.Printf("profiling: %v", err)
		return
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		lg.Printf("profiling: %v", err)
		return
	}
	p.cpu = f
	lg.Printf("profiling: the CPU profile goes to %s", path)
	if time.Since(p.last) > p.infosAt {
		p.infos(dir, lg)
	}
}

// stop stops a CPU profile, saving it, and takes the heap's.
func (p *profilerT) stop(dir string, lg *log.Logger) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cpu == nil {
		lg.Printf("profiling: no CPU profile runs")
		return
	}
	pprof.StopCPUProfile()
	p.cpu.Close()
	p.cpu = nil
	p.infos(dir, lg)
}

// infos takes the heap profile and the allocation profile: what a
// daemon that keeps sessions for days holds, and what it gave away.
func (p *profilerT) infos(dir string, lg *log.Logger) {
	p.last = time.Now()
	for _, name := range []string{"heap", "allocs", "block", "mutex"} {
		path := filepath.Join(dir, name+".pprof")
		f, err := os.Create(path)
		if err != nil {
			lg.Printf("profiling: %v", err)
			continue
		}
		prof := pprof.Lookup(name)
		if prof == nil {
			f.Close()
			continue
		}
		if err := prof.WriteTo(f, 0); err != nil {
			lg.Printf("profiling: %v", err)
		}
		f.Close()
		lg.Printf("profiling: %s goes to %s", name, path)
	}
}
