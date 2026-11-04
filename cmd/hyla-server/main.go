// Hyla-server is the server build of Hyla: hylad and the hyla CLI,
// without the app, for a machine whose sessions Hyla on another one shows
// (Settings › Hosts), as a Linux box with no display. It installs in
// ~/.local with install-server.sh, which runs hylad as a service, and
// updates itself (hyla daemon update, or the app's Update Hyla).
//
// Its version comes with the build:
//
//	go build -ldflags "-X main.version=2.1.0 -X github.com/egoist/mygo.production=1" -o hyla ./cmd/hyla-server
package main

import (
	"fmt"
	"os"

	"github.com/egoist/mygo"

	"hyla/internal/build"
	"hyla/internal/cli"
	"hyla/internal/daemon"
	"hyla/internal/proto"
)

// version is the version of Hyla this build is of, set with -ldflags.
var version = "dev"

func main() {
	proto.AdoptLegacyEnv()
	build.Server = true
	mygo.App.SetVersion(version)
	if len(os.Args) > 1 && os.Args[1] == "__hylad" {
		if err := daemon.Run(daemon.Options{}); err != nil {
			fmt.Fprintln(os.Stderr, "hylad:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) < 2 {
		os.Exit(cli.Run([]string{"help"}))
	}
	if !cli.IsCommand(os.Args[1]) {
		fmt.Fprintf(os.Stderr, "hyla: unknown command %q (see hyla help)\n", os.Args[1])
		os.Exit(2)
	}
	os.Exit(cli.Run(os.Args[1:]))
}
