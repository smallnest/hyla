//go:build !windows

package daemon

import "testing"

// The profiles are a development build's, on the directory $HYLA_PROFILE
// names: one of a release takes none, and so does one that names no
// directory.
func TestProfileDir(t *testing.T) {
	for _, c := range []struct {
		name string
		dev  bool
		dir  string
		want string
	}{
		{"a development build, with a directory", true, "/tmp/prof", "/tmp/prof"},
		{"a development build, with none", true, "", ""},
		{"a release build, with a directory", false, "/tmp/prof", ""},
		{"a release build, with none", false, "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.dev {
				t.Setenv("MYGO_ENV", "")
			} else {
				t.Setenv("MYGO_ENV", "production")
			}
			t.Setenv(profileDirEnv, c.dir)
			if got := profileDir(); got != c.want {
				t.Errorf("profileDir() = %q, want %q", got, c.want)
			}
		})
	}
}
