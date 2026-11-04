package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"go/build/constraint"
	"io/fs"
	"path"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/egoist/mygo"

	"hyla/internal/build"
)

// hyladSource is the code of Hyla's packages that hylad runs (go list
// -deps ./internal/daemon), which TestHyladSource keeps whole.
//
//go:embed internal/actions internal/adblock internal/build internal/clilink internal/daemon internal/detach internal/detect internal/feishu internal/git internal/proto internal/remote internal/settings internal/shell internal/tailscale internal/update
var hyladSource embed.FS

// hyladModules are the other modules whose code hylad runs, which
// TestHyladSource keeps whole too.
var hyladModules = []string{
	"filippo.io/edwards25519",
	"github.com/BurntSushi/toml",
	"github.com/anmitsu/go-shlex",
	"github.com/coder/websocket",
	"github.com/creachadair/msync",
	"github.com/creack/pty",
	"github.com/dblohm7/wingoes",
	"github.com/ebitengine/purego",
	"github.com/egoist/mygo",
	"github.com/fxamacker/cbor/v2",
	"github.com/gaissmai/bart",
	"github.com/go-json-experiment/json",
	"github.com/go-text/typesetting",
	"github.com/godbus/dbus/v5",
	"github.com/gogo/protobuf",
	"github.com/golang/groupcache",
	"github.com/google/btree",
	"github.com/gorilla/websocket",
	"github.com/hashicorp/yamux",
	"github.com/hdevalence/ed25519consensus",
	"github.com/jsimonetti/rtnetlink",
	"github.com/klauspost/compress",
	"github.com/kr/fs",
	"github.com/larksuite/oapi-sdk-go/v3",
	"github.com/mdlayher/netlink",
	"github.com/mdlayher/socket",
	"github.com/mitchellh/go-ps",
	"github.com/pires/go-proxyproto",
	"github.com/pkg/sftp",
	"github.com/safchain/ethtool",
	"github.com/tailscale/certstore",
	"github.com/tailscale/gliderssh",
	"github.com/tailscale/go-winio",
	"github.com/tailscale/hujson",
	"github.com/tailscale/peercred",
	"github.com/tailscale/tailcat",
	"github.com/tailscale/web-client-prebuilt",
	"github.com/tailscale/wireguard-go",
	"github.com/u-root/u-root",
	"github.com/x448/float16",
	"go4.org/mem",
	"go4.org/netipx",
	"golang.org/x/crypto",
	"golang.org/x/exp",
	"golang.org/x/image",
	"golang.org/x/net",
	"golang.org/x/sync",
	"golang.org/x/sys",
	"golang.org/x/term",
	"golang.org/x/text",
	"golang.org/x/time",
	"golang.zx2c4.com/wintun",
	"golang.zx2c4.com/wireguard/windows",
	"gvisor.dev/gvisor",
	"tailscale.com",
}

func init() { build.Daemon = hyladBuild() }

// hyladBuild fingerprints the code hylad runs on this platform: its
// packages' files, less tests and files of other platforms, the versions
// of its modules, and Go's.
func hyladBuild() string {
	h := sha256.New()
	fmt.Fprintln(h, runtime.Version())
	fs.WalkDir(hyladSource, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		b, err := hyladSource.ReadFile(p)
		if err != nil || !builds(p, b) {
			return err
		}
		fmt.Fprintln(h, p, len(b))
		h.Write(b)
		return nil
	})
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, m := range bi.Deps {
			if slices.Contains(hyladModules, m.Path) {
				if m.Replace != nil {
					m = m.Replace
				}
				fmt.Fprintln(h, m.Path, m.Version, m.Sum)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// goos are the platforms a file's name or build constraint can name, and
// the ones of them that the unix constraint means.
var (
	goos     = []string{"aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios", "js", "linux", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows"}
	unixGoos = []string{"aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios", "linux", "netbsd", "openbsd", "solaris"}
)

// builds reports whether a file of hyladSource is code built on this
// platform: no test, and of no other platform by its name or its
// constraint. Files that are no Go count, as code may embed them.
func builds(p string, b []byte) bool {
	if !strings.HasSuffix(p, ".go") {
		return true
	}
	name := strings.TrimSuffix(path.Base(p), ".go")
	if strings.HasSuffix(name, "_test") {
		return false
	}
	// name_GOOS, name_GOARCH or name_GOOS_GOARCH.
	parts := strings.Split(name, "_")
	for i := max(1, len(parts)-2); i < len(parts); i++ {
		if slices.Contains(goos, parts[i]) && parts[i] != runtime.GOOS {
			return false
		}
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "package ") {
			break
		}
		if constraint.IsGoBuild(line) {
			if expr, err := constraint.Parse(line); err == nil {
				return expr.Eval(func(tag string) bool {
					return tag == runtime.GOOS || tag == runtime.GOARCH ||
						tag == "unix" && slices.Contains(unixGoos, runtime.GOOS)
				})
			}
		}
	}
	return true
}

// hyladStale reports whether hylad, of the version and fingerprint its
// Welcome tells, runs other code than this app's, as after an update that
// changed it: an earlier hylad, which tells no fingerprint, of another
// version.
func hyladStale(app, fp string) bool {
	if fp != "" {
		return fp != build.Daemon
	}
	return app != mygo.App.Version()
}
