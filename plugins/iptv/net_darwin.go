//go:build darwin

package main

import (
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var proxyCache struct {
	mu  sync.Mutex
	at  time.Time
	url string
}

func systemProxyURL() string {
	proxyCache.mu.Lock()
	defer proxyCache.mu.Unlock()
	if !proxyCache.at.IsZero() && time.Since(proxyCache.at) < time.Minute {
		return proxyCache.url
	}
	out, err := exec.Command("scutil", "--proxy").Output()
	u := ""
	if err == nil {
		u = parseScutilProxy(string(out))
	}
	proxyCache.at = time.Now()
	proxyCache.url = u
	return u
}

func parseScutilProxy(out string) string {
	fields := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), " : ")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	proxyFor := func(scheme string) string {
		if fields[scheme+"Enable"] != "1" || fields[scheme+"Proxy"] == "" {
			return ""
		}
		port := fields[scheme+"Port"]
		if port == "" {
			port = "80"
		}
		return "http://" + net.JoinHostPort(fields[scheme+"Proxy"], port)
	}
	if u := proxyFor("HTTPS"); u != "" {
		return u
	}
	return proxyFor("HTTP")
}
