//go:build windows

package settings

import "syscall"

// SystemLanguage is the language of the Windows UI, as "zh-Hans", which
// the setting "system" follows.
func SystemLanguage() string {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	r, _, _ := proc.Call()
	// A Windows language ID is a primary language and a sublanguage:
	// Chinese's sublanguage says which script, as 0x0404 is taiwan's.
	switch lang := uint16(r); lang & 0x3ff {
	case 0x04:
		switch lang {
		case 0x0404, 0x0c04, 0x1404, 0x1004: // Taiwan, Hong Kong, Macau, Singapore
			return "zh-Hant"
		}
		return "zh-Hans"
	case 0x11:
		return "ja"
	case 0x07:
		return "de"
	case 0x09:
		return "en"
	}
	return ""
}
