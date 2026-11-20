//go:build windows

package netproxy

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

// systemProxyURL is the proxy Windows' Internet Settings name, as
// "http://127.0.0.1:7890", read at most once a minute: a browser reaches the
// network through it, and Hyla, started from the Start menu, has no
// environment that says so.
func systemProxyURL() string {
	proxyCache.mu.Lock()
	defer proxyCache.mu.Unlock()
	if now := time.Now(); now.Sub(proxyCache.at) < time.Minute {
		return proxyCache.url
	}
	proxyCache.at, proxyCache.url = time.Now(), readProxy()
	return proxyCache.url
}

var proxyCache struct {
	mu  sync.Mutex
	at  time.Time
	url string
}

// readProxy reads the WinINet settings a browser obeys.
func readProxy() string {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return ""
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil || server == "" {
		return ""
	}
	// A per-scheme setting is "http=host:port;https=host:port", and HTTPS
	// is the one a download goes through.
	if strings.Contains(server, "=") {
		for _, part := range strings.Split(server, ";") {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(part), "https="); ok {
				return "http://" + rest
			}
		}
		for _, part := range strings.Split(server, ";") {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(part), "http="); ok {
				return "http://" + rest
			}
		}
		return ""
	}
	return "http://" + server
}
