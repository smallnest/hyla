// Command fetch-lib downloads the native library that $MYGO_GHOSTTY_VT
// names, as mygo does for a program that is not packaged: as build-server.sh
// does, with more attempts, as GitHub's server answers the link of a release
// with 502 and 504 at times.
package main

import (
	"fmt"
	"os"

	"github.com/egoist/mygo/plugins/terminal"
)

func main() {
	if os.Getenv("MYGO_GHOSTTY_VT") == "" {
		fmt.Fprintln(os.Stderr, "fetch-lib: set MYGO_GHOSTTY_VT")
		os.Exit(2)
	}
	if err := terminal.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "fetch-lib:", err)
		os.Exit(1)
	}
}
