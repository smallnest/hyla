//go:build darwin

package settings

import (
	"os/exec"
	"strings"
	"sync"
)

// SystemLanguage is the Mac's first preferred language, as "zh-Hans-CN",
// which the setting "system" follows: a process with no window, as hylad,
// has no other way to know it.
func SystemLanguage() string { return macSystemLanguage() }

// macSystemLanguage reads it once, from the system's own preferences: a
// change of language shows in the app it restarts, and comes to hylad
// with it.
var macSystemLanguage = sync.OnceValue(func() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return ""
	}
	// ( "zh-Hans-CN", "en-US" ) gives its languages, the first preferred.
	for _, word := range strings.Fields(string(out)) {
		if tag := strings.Trim(word, `(),"`); tag != "" {
			return tag
		}
	}
	return ""
})
