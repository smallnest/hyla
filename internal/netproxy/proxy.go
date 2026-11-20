// Package netproxy gives Hyla's own HTTP the proxy the system's network
// settings name.
//
// A browser reaches the network through the system's proxy; a program the
// window started — from the Finder, from Explorer — has no environment of
// its own, and Go's net/http reads the proxy from the environment alone. A
// marketplace install that went direct on a machine behind a system proxy
// waited for a download that went the slow way, or not at all.
package netproxy

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// systemProxyFn is the system's proxy, as the platform reads it: "" where it
// names none. A test sets it; nothing else does.
var systemProxyFn = systemProxyURL

// override is the proxy the app's settings name, in http-proxy: the user's
// word on every download, which wins over the environment and the system.
var override atomic.Pointer[func() string]

// SetOverride makes f the settings' word on the proxy: the value it returns,
// where it names one, is the proxy every request goes through. A program
// calls it once, with the settings it reads.
func SetOverride(f func() string) {
	if f == nil {
		override.Store(nil)
		return
	}
	override.Store(&f)
}

// Install makes net/http's default transport ask Proxy, and with it every
// client that names no transport of its own — Hyla's downloads among them.
func Install() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = Proxy
	}
}

// Proxy is the proxy an HTTP request goes through: the one http-proxy names,
// or else the environment's, as HTTPS_PROXY and HTTP_PROXY say it, or else
// the system's.
func Proxy(req *http.Request) (*url.URL, error) {
	if f := override.Load(); f != nil {
		if raw := strings.TrimSpace((*f)()); raw != "" {
			return proxyURL(req, raw)
		}
	}
	// An environment that names a proxy is the user's word on this program,
	// and net/http already knows how to honour NO_PROXY within it.
	if u, err := http.ProxyFromEnvironment(req); err != nil || u != nil {
		return u, err
	}
	return proxyURL(req, systemProxyFn())
}

// proxyURL is raw as a proxy for req, or none where raw names none, and
// where the request is to this machine or the network it is on: a proxy
// would only add a hop to a pane's gateway or a NAS.
func proxyURL(req *http.Request, raw string) (*url.URL, error) {
	if raw == "" || req == nil || req.URL == nil {
		return nil, nil
	}
	if bypass(req.URL.Hostname()) {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, nil
	}
	return u, nil
}

// bypass tells a host the system proxy leaves alone: this machine's, a name
// that stays on the local network, or a private address.
func bypass(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "" || h == "localhost" || h == "::1" || strings.HasSuffix(h, ".local") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	return false
}

// Transport is an http.Transport that goes through Proxy, with the rest of
// net/http's own.
func Transport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = Proxy
	return t
}

// Client is an http.Client that waits as long as timeout for each request,
// and goes through the proxy Proxy names.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: Transport()}
}
