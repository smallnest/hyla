package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func procName(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func procArgs(pid int) []string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(b), "\x00"), "\x00")
}

func procCwd(pid int) string {
	dir, _ := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	return dir
}

// procParent returns a process's parent, 0 when it is gone: the field
// of /proc/pid/stat after its state, past the name in parentheses.
// procList returns nil: ps lists the processes on Linux.
func procList() []osProc { return nil }

func procParent(pid int) int {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0
	}
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return 0
	}
	f := strings.Fields(string(b[i+1:]))
	if len(f) < 2 {
		return 0
	}
	ppid, _ := strconv.Atoi(f[1])
	return ppid
}

func procAgyConversation(pid int) string {
	if pid <= 0 {
		return ""
	}
	fdDir := "/proc/" + strconv.Itoa(pid) + "/fd"
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return ""
	}
	bestID, bestPrio := "", 0
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join(fdDir, e.Name()))
		if err != nil {
			continue
		}
		id, prio := agyConversationFromPath(target)
		if prio == 2 {
			return id
		}
		if prio > bestPrio {
			bestID, bestPrio = id, prio
		}
	}
	return bestID
}
