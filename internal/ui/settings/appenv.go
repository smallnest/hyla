package settings

import (
	"os"
	"slices"
	"strings"
)

// AppEnv is the app's environment, which the bot's agent commands
// run in, as hylad reads it now: a command reaches hylad as its window
// starts it, or from a login shell's. The variables a login shell's
// profile reads live, as a token or a socket that changes, are read
// again with each setting the group applies. The variables of AUTH,
// Sockets, MCP proxies and tokens are left out, and every variable with
// a secret in its name: the configuration keeps them as they were.
func AppEnv() []string {
	drop := func(k string) bool {
		k = strings.ToUpper(k)
		for _, p := range []string{"AUTH", "TOKEN", "SECRET", "PASSWORD", "KEY", "CREDENTIAL", "SOCK", "PROXY"} {
			if strings.Contains(k, p) {
				return true
			}
		}
		return k == "PATH" || k == "PWD" || k == "OLDPWD" || k == "SHLVL" || k == "_" || k == "TMPDIR" ||
			strings.HasPrefix(k, "TMP") || k == "TERM" || k == "COLORTERM" || k == "SHELL" || strings.HasPrefix(k, "SSH_")
	}
	// The providers' keys, as ANTHROPIC_API_KEY, which an agent needs.
	keep := []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "DEEPSEEK_API_KEY", "MOONSHOT_API_KEY",
		"ARK_API_KEY", "ZHIPU_API_KEY", "MIMOCODE_API_KEY", "OPENROUTER_API_KEY", "GROQ_API_KEY", "MISTRAL_API_KEY", "XAI_API_KEY"}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if len(kv) > 32<<10 || strings.ContainsAny(kv, "\n\r\x00") {
			continue
		}
		if slices.Contains(keep, k) || !drop(k) {
			env = append(env, kv)
		}
	}
	return env
}
