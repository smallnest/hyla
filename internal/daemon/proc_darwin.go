package daemon

import (
	"bytes"
	"encoding/binary"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// procName returns the name of a process, as ps shows it.
func procName(pid int) string {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return ""
	}
	b := kp.Proc.P_comm[:]
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// procArgs returns the arguments a process was started with.
func procArgs(pid int) []string {
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil || len(buf) < 4 {
		return nil
	}
	argc := int(binary.LittleEndian.Uint32(buf))
	buf = buf[4:]
	// The executable's path, then NULs up to the arguments.
	i := bytes.IndexByte(buf, 0)
	if i < 0 {
		return nil
	}
	buf = buf[i:]
	for len(buf) > 0 && buf[0] == 0 {
		buf = buf[1:]
	}
	args := make([]string, 0, argc)
	for len(args) < argc && len(buf) > 0 {
		i := bytes.IndexByte(buf, 0)
		if i < 0 {
			args = append(args, string(buf))
			break
		}
		args = append(args, string(buf[:i]))
		buf = buf[i+1:]
	}
	return args
}

var (
	libprocOnce   sync.Once
	procPidinfo   func(pid int32, flavor int32, arg uint64, buffer unsafe.Pointer, size int32) int32
	procPidfdinfo func(pid int32, fd int32, flavor int32, buffer unsafe.Pointer, size int32) int32
)

func initLibproc() {
	libprocOnce.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&procPidinfo, lib, "proc_pidinfo")
		purego.RegisterLibFunc(&procPidfdinfo, lib, "proc_pidfdinfo")
	})
}

// procList returns every process, as ps lists them.
func procList() []osProc {
	buf, err := unix.SysctlRaw("kern.proc.all")
	if err != nil {
		return nil
	}
	// The kernel's struct kinfo_proc, without Go's tail padding.
	const size = int(unsafe.Offsetof(unix.KinfoProc{}.Eproc)) + int(unsafe.Sizeof(unix.Eproc{})) - 4
	out := make([]osProc, 0, len(buf)/size)
	put := func(pid int, kp *unix.KinfoProc) {
		comm := kp.Proc.P_comm[:]
		if i := bytes.IndexByte(comm, 0); i >= 0 {
			comm = comm[:i]
		}
		out = append(out, osProc{PID: pid, PPID: int(kp.Eproc.Ppid), CPU: float64(kp.Proc.P_pctcpu) / float64(1<<16) * 100, Rune: string(comm)})
	}
	for ; len(buf) >= size; buf = buf[size:] {
		var kp unix.KinfoProc
		copy((*[1 << 16]byte)(unsafe.Pointer(&kp))[:size], buf[:size])
		put(int(kp.Proc.P_pid), &kp)
	}
	return out
}

// procCwd returns the working directory of a process, through
// proc_pidinfo's PROC_PIDVNODEPATHINFO.
func procCwd(pid int) string {
	initLibproc()
	if procPidinfo == nil {
		return ""
	}
	// struct proc_vnodepathinfo: the current directory's vnode_info_path
	// (a vnode_info of 152 bytes, then the path of MAXPATHLEN), then the
	// root directory's.
	const vnodeInfo, maxPath = 152, 1024
	var buf [2 * (vnodeInfo + maxPath)]byte
	const pidVnodePathInfo = 9
	n := procPidinfo(int32(pid), pidVnodePathInfo, 0, unsafe.Pointer(&buf[0]), int32(len(buf)))
	if n <= 0 {
		return ""
	}
	path := buf[vnodeInfo : vnodeInfo+maxPath]
	if i := bytes.IndexByte(path, 0); i >= 0 {
		path = path[:i]
	}
	return string(path)
}

// procParent returns a process's parent, 0 when it is gone.
func procParent(pid int) int {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return 0
	}
	return int(kp.Eproc.Ppid)
}

// procAgyConversation inspects open file descriptors of an agy process to find
// its active conversation ID (presence/<id>.lock or conversations/<id>.db).
func procAgyConversation(pid int) string {
	if pid <= 0 {
		return ""
	}
	initLibproc()
	if procPidinfo == nil || procPidfdinfo == nil {
		return ""
	}
	const (
		procPIDListFDs        = 1
		proxFDTypeVnode       = 1
		procPIDFDVnodePathInf = 2
		vnodePathOffset       = 176
	)
	n := procPidinfo(int32(pid), procPIDListFDs, 0, nil, 0)
	if n <= 0 {
		return ""
	}
	if n > 1<<20 {
		n = 1 << 20
	}
	buf := make([]byte, n)
	n2 := procPidinfo(int32(pid), procPIDListFDs, 0, unsafe.Pointer(&buf[0]), int32(len(buf)))
	if n2 <= 0 {
		return ""
	}
	bestID, bestPrio := "", 0
	var vbuf [1200]byte
	for off := 0; off+8 <= int(n2); off += 8 {
		fd := int32(binary.LittleEndian.Uint32(buf[off:]))
		fdtype := binary.LittleEndian.Uint32(buf[off+4:])
		if fdtype != proxFDTypeVnode {
			continue
		}
		vn := procPidfdinfo(int32(pid), fd, procPIDFDVnodePathInf, unsafe.Pointer(&vbuf[0]), int32(len(vbuf)))
		if vn <= vnodePathOffset || int(vn) > len(vbuf) {
			continue
		}
		raw := vbuf[vnodePathOffset:vn]
		if i := bytes.IndexByte(raw, 0); i >= 0 {
			raw = raw[:i]
		}
		id, prio := agyConversationFromPath(string(raw))
		if prio == 2 {
			return id
		}
		if prio > bestPrio {
			bestID, bestPrio = id, prio
		}
	}
	return bestID
}
