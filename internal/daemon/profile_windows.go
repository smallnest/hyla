package daemon

import "log"

// profiling lets a hylad that runs from a shell, as one that is built
// into a profile, tell where its CPU and memory go. Windows has none of
// the signals that ask for the profiles (profile_unix.go), so it writes
// none: a profile of hylad there builds it with the Go tooling's.
func startProfiling(lg *log.Logger) {}
