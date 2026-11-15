package arch

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"testing"
)

// TestLayers checks the imports of every package of this module against
// its layer. It reads them from `go list`, for each platform Hyla builds
// for, as a file of one platform's imports alone would not show.
func TestLayers(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for pkg, imports := range packages(t, goos) {
			for _, bad := range Check(pkg, imports) {
				t.Errorf("%s: %s imports %s, which its layer does not allow", goos, pkg, bad)
			}
		}
	}
}

// packages reads each package of this module, and its imports, as go
// list gives them to build for goos.
func packages(t *testing.T, goos string) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-deps", "-json=ImportPath,Imports,Standard", "./...")
	// A test runs in its package's directory, where "./..." is not the
	// module's.
	cmd.Dir = "../.."
	cmd.Env = append(cmd.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: go list: %v", goos, err)
	}
	// go list writes one object after another, not an array.
	m := map[string][]string{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p struct {
			ImportPath string
			Imports    []string
			Standard   bool
		}
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
		if p.Standard || !Imported(p.ImportPath) {
			continue
		}
		m[p.ImportPath] = p.Imports
	}
	if len(m) == 0 {
		t.Fatalf("%s: no packages", goos)
	}
	return m
}
