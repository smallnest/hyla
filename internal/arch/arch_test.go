package arch

import (
	"slices"
	"testing"
)

func TestLayersNamed(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range Layers {
		if l.Name == "" || seen[l.Name] {
			t.Fatalf("layer %q is empty or named twice", l.Name)
		}
		seen[l.Name] = true
		if len(l.Package) == 0 {
			t.Errorf("layer %s names no imports", l.Name)
		}
	}
}

// TestLayerAbove checks that no layer names a package of a layer above
// it: a list that does names what a package may not import anyway.
func TestLayerAbove(t *testing.T) {
	at := map[string]int{}
	for i, l := range Layers {
		at[l.Name] = i
	}
	for i, l := range Layers {
		if l.Name == "root" {
			continue
		}
		for _, p := range l.Package {
			if p == "*" {
				t.Errorf("layer %s names every package", l.Name)
				continue
			}
			l2, ok := LayerOf(p)
			if !ok || l2.Name == "root" {
				t.Errorf("%s: %s is not a package of internal/", l.Name, p)
				continue
			}
			if at[l2.Name] > i {
				t.Errorf("%s names %s, of the layer above it", l.Name, p)
			}
		}
	}
}

func TestLayerOf(t *testing.T) {
	cases := []struct {
		pkg  string
		want string
	}{
		{"hyla", "root"},
		{"hyla/locales", "root"},
		{"hyla/internal/proto", "base"},
		{"hyla/internal/arch", "base"},
		{"hyla/internal/settings", "wire"},
		{"hyla/internal/remote", "wire"},
		{"hyla/internal/daemon", "daemon"},
		{"hyla/internal/daemon/sub", "daemon"},
		{"hyla/internal/client", "daemon"},
		{"hyla/internal/cli", "cli"},
	}
	for _, c := range cases {
		l, ok := LayerOf(c.pkg)
		if !ok {
			t.Errorf("%s: not of this module", c.pkg)
			continue
		}
		if l.Name != c.want {
			t.Errorf("%s: layer %s, want %s", c.pkg, l.Name, c.want)
		}
	}
	if _, ok := LayerOf("github.com/egoist/mygo"); ok {
		t.Error("mygo is of this module")
	}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		pkg     string
		imports []string
		want    []string
	}{
		// Below is always allowed: the daemon runs on the base.
		{"hyla/internal/daemon", []string{"hyla/internal/proto", "hyla/internal/client"}, nil},
		// Above is not.
		{"hyla/internal/proto", []string{"hyla/internal/daemon"}, []string{"hyla/internal/daemon"}},
		{"hyla/internal/settings", []string{"hyla/internal/cli"}, []string{"hyla/internal/cli"}},
		// The root is a package of its own.
		{"hyla/internal/service", []string{"hyla"}, []string{"hyla"}},
		// locales, and what internal/ gains before it is listed, are
		// below every layer.
		{"hyla/internal/proto", []string{"hyla/locales", "hyla/internal/newpkg"}, nil},
		// What is not of this module is outside the rules.
		{"hyla/internal/proto", []string{"github.com/egoist/mygo"}, nil},
		// A package may import itself, as its files do.
		{"hyla/internal/proto", []string{"hyla/internal/proto"}, nil},
	}
	for _, c := range cases {
		if got := Check(c.pkg, c.imports); !slices.Equal(got, c.want) {
			t.Errorf("%s imports %v: %v, want %v", c.pkg, c.imports, got, c.want)
		}
	}
}

func TestImported(t *testing.T) {
	for _, p := range []string{"hyla", "hyla/internal/proto"} {
		if !Imported(p) {
			t.Errorf("%s: not imported", p)
		}
	}
	for _, p := range []string{"hylaa", "hylax/internal/proto", "github.com/x"} {
		if Imported(p) {
			t.Errorf("%s: imported", p)
		}
	}
}
