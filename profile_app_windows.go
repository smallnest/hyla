//go:build windows

package main

// profiling writes the window's profiles on the signals that ask for
// them, which Windows has not: it writes none, and a profile of the
// window there builds it with the Go tooling's.
func startAppProfiling() {}
