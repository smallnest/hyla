//go:build !windows

package daemon

import (
	"os"
	"slices"
	"syscall"
	"testing"
)

func TestStopNotifyIncludesTERMAndINT(t *testing.T) {
	sigs := stopNotify()
	if !slices.ContainsFunc(sigs, func(s os.Signal) bool { return s == syscall.SIGTERM }) {
		t.Errorf("stopNotify() = %v; missing SIGTERM", sigs)
	}
	if !slices.ContainsFunc(sigs, func(s os.Signal) bool { return s == syscall.SIGINT }) {
		t.Errorf("stopNotify() = %v; missing SIGINT", sigs)
	}
}
