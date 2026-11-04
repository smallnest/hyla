// Package build tells what build of Hyla runs: its version, and whether
// it is the server build (cmd/hyla-server), which runs hylad and the CLI
// alone, with no app, and updates itself.
package build

import "github.com/egoist/mygo"

// Server is set by the server build.
var Server bool

// Version is the version of Hyla that runs: the app's, or the one the
// server build was made of.
func Version() string { return mygo.App.Version() }

// Daemon is what hylad's code is made of, which the app sets (hyladBuild)
// and hylad tells clients (proto.Welcome.Build): an update that leaves it
// as it was needs no restart of hylad. The server build leaves it empty.
var Daemon string
