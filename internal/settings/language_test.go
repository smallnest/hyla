package settings

import "testing"

// A language the setting names is the app's, and one it leaves to the
// system is the system's own, never the word "system".
func TestLanguageOf(t *testing.T) {
	for _, pref := range []string{"en", "ja", "zh-Hans"} {
		if got := LanguageOf(pref); got != pref {
			t.Errorf("%q is the app's language as %q", pref, got)
		}
	}
	for _, pref := range []string{"", "system"} {
		if got := LanguageOf(pref); got == "system" {
			t.Errorf("%q says the language is %q", pref, got)
		}
	}
	if got := LanguageOf(""); got != SystemLanguage() {
		t.Errorf("following the system is %q, and the system's is %q", got, SystemLanguage())
	}
}
