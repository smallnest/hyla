//go:build !windows

package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"sync"
	"syscall"

	"github.com/egoist/mygo"
)

// A development build of Hyla, started with $HYLA_PROFILE, writes the
// profiles of its window there on the signals hylad takes:
//
//	SIGUSR1  starts a CPU profile, into app-cpu.pprof, and writes the heap,
//	         allocation, block and mutex profiles
//	SIGUSR2  ends the CPU profile, and writes them again
//
// It is the profile of what a window costs on screen: the tests draw
// through the headless renderer, which the window, drawing on the GPU,
// does not use. A release build writes none, as it is the user's.
const appProfileDirEnv = "HYLA_PROFILE"

// appProfiler is the CPU profile of the window running, if any.
type appProfiler struct {
	mu  sync.Mutex
	cpu *os.File
}

// appProfiles is the window's profiler.
var appProfiles appProfiler

// startAppProfiling attaches the signals to the window's profiles, when
// $HYLA_PROFILE says where to write them.
func startAppProfiling() {
	dir := os.Getenv(appProfileDirEnv)
	if dir == "" || !mygo.IsDev() {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("profiling: %v", err)
		return
	}
	log.Printf("profiling into %s: SIGUSR1 starts the CPU profile, SIGUSR2 ends it", dir)
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for sig := range sigs {
			switch sig {
			case syscall.SIGUSR1:
				appProfiles.start(dir)
			case syscall.SIGUSR2:
				appProfiles.stop(dir)
			}
		}
	}()
}

// start begins the window's CPU profile, and writes the others.
func (p *appProfiler) start(dir string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cpu != nil {
		log.Printf("profiling: the CPU profile is running")
		return
	}
	path := filepath.Join(dir, "app-cpu.pprof")
	f, err := os.Create(path)
	if err != nil {
		log.Printf("profiling: %v", err)
		return
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		log.Printf("profiling: %v", err)
		return
	}
	p.cpu = f
	log.Printf("profiling: the CPU profile goes to %s", path)
	p.infos(dir)
}

// stop ends the CPU profile, saving it, and writes the others again.
func (p *appProfiler) stop(dir string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cpu == nil {
		log.Printf("profiling: no CPU profile runs")
		return
	}
	pprof.StopCPUProfile()
	p.cpu.Close()
	p.cpu = nil
	p.infos(dir)
}

// infos writes what a window holds and gives away: the heap, the
// allocations, the blocks and the mutexes.
func (p *appProfiler) infos(dir string) {
	for _, name := range []string{"heap", "allocs", "block", "mutex"} {
		path := filepath.Join(dir, "app-"+name+".pprof")
		f, err := os.Create(path)
		if err != nil {
			log.Printf("profiling: %v", err)
			continue
		}
		prof := pprof.Lookup(name)
		if prof == nil {
			f.Close()
			continue
		}
		if err := prof.WriteTo(f, 0); err != nil {
			log.Printf("profiling: %v", err)
		}
		f.Close()
		log.Printf("profiling: %s goes to %s", name, path)
	}
}
