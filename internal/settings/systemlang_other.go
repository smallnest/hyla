//go:build !darwin && !linux && !windows

package settings

// SystemLanguage is none where hyla does not know the system's: an app
// written in these languages reads them there.
func SystemLanguage() string { return "" }
