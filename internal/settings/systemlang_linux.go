//go:build linux

package settings

import (
	"os"
	"strings"
)

// SystemLanguage is the language the system's locale names, as "zh-CN",
// which the setting "system" follows.
func SystemLanguage() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if locale := os.Getenv(name); locale != "" {
			if tag := localeTag(locale); tag != "" {
				return tag
			}
		}
	}
	return ""
}

// localeTag is a POSIX locale as a language tag: zh_CN.UTF-8 is "zh-CN",
// and C or POSIX is none.
func localeTag(locale string) string {
	if i := strings.IndexAny(locale, ".@"); i >= 0 {
		locale = locale[:i]
	}
	tag := strings.ReplaceAll(locale, "_", "-")
	if tag == "C" || tag == "POSIX" {
		return ""
	}
	return tag
}
