package buildinfo

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestHyladSource checks that hyladSource and hyladModules hold all the
// code hylad runs, or an update that changes it would leave hylad as it
// was. A package missing from the copy is of one no platform builds, and
// TestSourceSync reports one synced another platform's files.
func TestHyladSource(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		cmd := exec.Command("go", "list", "-e", "-deps", "-f", "{{if .Module}}{{.Module.Main}} {{.Module.Path}} {{.ImportPath}}{{end}}", "./internal/daemon")
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			f := strings.Fields(line)
			if f[0] == "true" {
				dir := strings.TrimPrefix(f[2], f[1]+"/")
				if _, err := fs.Stat(hyladSource, SourceDir+"/"+dir); err != nil {
					t.Errorf("%s: hyladSource has no %s", goos, dir)
				}
			} else if !slices.Contains(hyladModules, f[1]) {
				t.Errorf("%s: hyladModules has no %s", goos, f[1])
			}
		}
	}
}

// TestSourceSync checks the copy against the code it copies: each file
// of it is its original, less tests and other platforms' files, as
// scripts/sync-buildinfo.sh writes it, and nothing is missing.
func TestSourceSync(t *testing.T) {
	synced := map[string]bool{}
	fs.WalkDir(hyladSource, SourceDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, SourceDir+"/")
		synced[rel] = true
		b, err := hyladSource.ReadFile(p)
		if err != nil {
			t.Error(err)
			return nil
		}
		orig, err := fs.ReadFile(origin, rel)
		if err != nil {
			t.Errorf("hyladSource/%s: %v", rel, err)
			return nil
		}
		if !bytes.Equal(b, orig) {
			t.Errorf("hyladSource/%s differs from %s: run scripts/sync-buildinfo.sh", rel, rel)
		}
		return nil
	})
	// And nothing is missing: what go list names, minus what no
	// platform builds.
	cmd := exec.Command("go", "list", "-e", "-deps", "-json=ImportPath,Dir,Module,GoFiles,CgoFiles,EmbedFiles", "./internal/daemon")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range listed(out) {
		if !synced[rel] {
			t.Errorf("hyladSource has no %s: run scripts/sync-buildinfo.sh", rel)
		}
	}
}

// origin is the module's own code, which hyladSource copies.
var origin = os.DirFS("../..")

// root is the module's directory, where go list runs.
var root = "../.."

// listed reads the files go list names for the packages hylad runs, at
// their paths from the module's root.
func listed(out []byte) []string {
	var files []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p struct {
			Dir                                           string
			Module                                        struct{ Main bool }
			GoFiles, CgoFiles, IgnoredGoFiles, EmbedFiles []string
		}
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		if !p.Module.Main {
			continue
		}
		dir := strings.TrimSuffix(strings.TrimPrefix(p.Dir, moduleRoot()+"/"), "/")
		for _, f := range slices.Concat(p.GoFiles, p.CgoFiles, p.IgnoredGoFiles, p.EmbedFiles) {
			files = append(files, dir+"/"+f)
		}
	}
	return files
}

// moduleRoot is the directory the tests run from, two levels up.
func moduleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return strings.TrimSuffix(wd, "/internal/buildinfo")
}
