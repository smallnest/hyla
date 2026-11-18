package settings

// LanguageOf is the language the app shows for the language setting pref:
// the one it names, or the system's where it follows the system. A
// process that has no window, as hylad has none, reads the setting alone,
// and its "system" is the system's own.
func LanguageOf(pref string) string {
	if pref != "" && pref != "system" {
		return pref
	}
	return SystemLanguage()
}
