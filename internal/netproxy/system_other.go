//go:build !darwin && !windows

package netproxy

// systemProxyURL is none: on these systems a program reads its proxy from
// the environment, which Proxy already does.
func systemProxyURL() string { return "" }
