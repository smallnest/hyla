//go:build darwin

package netproxy

import (
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// systemProxyURL is the proxy the Mac's network settings name, as
// "http://127.0.0.1:7890", read at most once a minute: a browser reaches the
// network through it, and Hyla, started from the Finder, has no environment
// that says so.
func systemProxyURL() string {
	proxyCache.mu.Lock()
	defer proxyCache.mu.Unlock()
	if now := time.Now(); now.Sub(proxyCache.at) < time.Minute {
		return proxyCache.url
	}
	out, err := exec.Command("scutil", "--proxy").Output()
	url := ""
	if err == nil {
		url = parseScutil(string(out))
	}
	proxyCache.at, proxyCache.url = time.Now(), url
	return url
}

var proxyCache struct {
	mu  sync.Mutex
	at  time.Time
	url string
}

// parseScutil reads `scutil --proxy`'s dictionary: HTTPS's proxy where it is
// on, else HTTP's, as a URL a request can go through.
func parseScutil(out string) string {
	fields := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), " : ")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	proxy := func(scheme string) string {
		if fields[scheme+"Enable"] != "1" || fields[scheme+"Proxy"] == "" {
			return ""
		}
		port := fields[scheme+"Port"]
		if port == "" {
			port = "80"
		}
		return "http://" + net.JoinHostPort(fields[scheme+"Proxy"], port)
	}
	if u := proxy("HTTPS"); u != "" {
		return u
	}
	return proxy("HTTP")
}
