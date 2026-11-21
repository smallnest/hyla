package main

import (
	"os"
	"strings"
)

// isTestBinary tells a test run from the plugin: the test binary's name
// ends in ".test". A test gives a copy of iptv.md and no network.
func isTestBinary() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.HasSuffix(exe, ".test")
}
