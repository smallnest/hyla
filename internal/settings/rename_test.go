package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hyla/internal/proto"
)

// The pet and the theme a 2.x, which called Hyla Kero, named are Hyla's.
func TestKeroNamesNormalize(t *testing.T) {
	s := Default()
	s.PetSet = "kero"
	s.LightTheme = "kero"
	s.DarkTheme = "kero"
	s.normalize()
	if s.PetSet != PetSetHyla {
		t.Errorf("pet-set = %q, want %q", s.PetSet, PetSetHyla)
	}
	if s.LightTheme != "hyla" || s.DarkTheme != "hyla" {
		t.Errorf("theme = %q, %q; want hyla, hyla", s.LightTheme, s.DarkTheme)
	}
}

// The configuration file of 2.x comes along to Hyla's, outside the
// directory on macOS.
func TestConfigPathAdoptsKeroConfig(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("HYLA_CONFIG", "")
	old := filepath.Join(base, strings.Replace(proto.Name(), "hyla", "kero", 1), "config")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("font-size = 15\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ConfigPath()
	want := filepath.Join(base, proto.Name(), "config")
	if got != want {
		t.Fatalf("ConfigPath() = %q, want %q", got, want)
	}
	if b, err := os.ReadFile(want); err != nil || string(b) != "font-size = 15\n" {
		t.Errorf("2.x's configuration did not come along: %q, %v", b, err)
	}
}
