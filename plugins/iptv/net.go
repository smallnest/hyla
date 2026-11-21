package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15"

type NetConfig struct {
	ProxyMode       string `json:"proxyMode"` // "auto" | "direct" | "system" | "custom"
	HTTPProxy       string `json:"httpProxy"`
	AutoFailover    bool   `json:"autoFailover"`
	AutoMuteOnAgent bool   `json:"autoMuteOnAgent"`
	UserAgent       string `json:"userAgent"`
}

func defaultNetConfig() NetConfig {
	return NetConfig{
		ProxyMode:    "auto",
		AutoFailover: true,
	}
}

func buildNetConfig(settings map[string]any) NetConfig {
	cfg := defaultNetConfig()
	if settings == nil {
		return cfg
	}
	if v, ok := settings["proxy_mode"].(string); ok && strings.TrimSpace(v) != "" {
		mode := strings.ToLower(strings.TrimSpace(v))
		switch mode {
		case "auto", "direct", "system", "custom":
			cfg.ProxyMode = mode
		}
	}
	if v, ok := settings["http_proxy"].(string); ok {
		cfg.HTTPProxy = strings.TrimSpace(v)
	}
	if v, ok := settings["auto_failover"].(bool); ok {
		cfg.AutoFailover = v
	}
	if v, ok := settings["auto_mute_on_agent"].(bool); ok {
		cfg.AutoMuteOnAgent = v
	}
	if v, ok := settings["user_agent"].(string); ok {
		cfg.UserAgent = strings.TrimSpace(v)
	}
	return cfg
}

func validateHTTPURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("URL must not be empty")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return nil, fmt.Errorf("unsupported URL scheme %q (expected http or https)", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, fmt.Errorf("URL %q missing host", trimmed)
	}
	return u, nil
}

func isLocalHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "" || h == "localhost" || h == "::1" || strings.HasSuffix(h, ".local") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	return false
}

func resolveEnvOrSystemProxy(req *http.Request) (*url.URL, error) {
	if req != nil && req.URL != nil && isLocalHost(req.URL.Hostname()) {
		return nil, nil
	}
	if u, err := http.ProxyFromEnvironment(req); err != nil || u != nil {
		return u, err
	}
	if sys := strings.TrimSpace(systemProxyURL()); sys != "" {
		if u, err := url.Parse(sys); err == nil && u.Scheme != "" && u.Host != "" {
			return u, nil
		}
	}
	return nil, nil
}

func newTransport(proxyFn func(*http.Request) (*url.URL, error), disableCompression bool, headerTimeout time.Duration) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{
		Timeout:   8 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.ResponseHeaderTimeout = headerTimeout
	tr.Proxy = proxyFn
	tr.DisableCompression = disableCompression
	return tr
}

type netClients struct {
	mu           sync.Mutex
	proxyDirect  *http.Client
	proxySystem  *http.Client
	customProxy  string
	proxyCustom  *http.Client
	streamDirect *http.Client
	streamSystem *http.Client
	streamCustom *http.Client
}

func newNetClients() *netClients {
	return &netClients{}
}

func (nc *netClients) closeIdle() {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	for _, c := range []*http.Client{nc.proxyDirect, nc.proxySystem, nc.proxyCustom, nc.streamDirect, nc.streamSystem, nc.streamCustom} {
		if c != nil {
			if tr, ok := c.Transport.(*http.Transport); ok {
				tr.CloseIdleConnections()
			}
		}
	}
}

func (nc *netClients) ensure(customProxy string) (directP, systemP, customP, directS, systemS, customS *http.Client) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	cp := strings.TrimSpace(customProxy)
	if nc.proxyDirect == nil {
		nc.proxyDirect = &http.Client{Timeout: 20 * time.Second, Transport: newTransport(nil, false, 15*time.Second)}
		nc.proxySystem = &http.Client{Timeout: 20 * time.Second, Transport: newTransport(resolveEnvOrSystemProxy, false, 15*time.Second)}
		nc.streamDirect = &http.Client{Transport: newTransport(nil, true, 8*time.Second)}
		nc.streamSystem = &http.Client{Transport: newTransport(resolveEnvOrSystemProxy, true, 12*time.Second)}
	}
	if cp != nc.customProxy {
		nc.customProxy = cp
		nc.proxyCustom = nil
		nc.streamCustom = nil
	}
	if cp != "" && nc.proxyCustom == nil {
		parsed, err := url.Parse(cp)
		var fn func(*http.Request) (*url.URL, error)
		if err == nil && parsed.Scheme != "" && parsed.Host != "" {
			fn = func(req *http.Request) (*url.URL, error) {
				if req != nil && req.URL != nil && isLocalHost(req.URL.Hostname()) {
					return nil, nil
				}
				return parsed, nil
			}
		}
		nc.proxyCustom = &http.Client{Timeout: 20 * time.Second, Transport: newTransport(fn, false, 15*time.Second)}
		nc.streamCustom = &http.Client{Transport: newTransport(fn, true, 12*time.Second)}
	}
	return nc.proxyDirect, nc.proxySystem, nc.proxyCustom, nc.streamDirect, nc.streamSystem, nc.streamCustom
}

type requestHeaders struct {
	UserAgent string
	Referer   string
	Range     string
	IfRange   string
	Accept    string
	Identity  bool
}

func (nc *netClients) doSmart(ctx context.Context, method, rawURL string, hdrs requestHeaders, cfg NetConfig, preferDirect, streaming bool) (*http.Response, error) {
	u, err := validateHTTPURL(rawURL)
	if err != nil {
		return nil, err
	}
	dp, sp, cp, ds, ss, cs := nc.ensure(cfg.HTTPProxy)
	direct := dp
	system := sp
	custom := cp
	if streaming {
		direct = ds
		system = ss
		custom = cs
	}

	var candidates []*http.Client
	mode := strings.ToLower(strings.TrimSpace(cfg.ProxyMode))
	switch mode {
	case "direct":
		candidates = []*http.Client{direct}
	case "system":
		candidates = []*http.Client{system}
	case "custom":
		if custom != nil {
			candidates = []*http.Client{custom}
		} else {
			candidates = []*http.Client{system}
		}
	default: // "auto"
		if preferDirect {
			candidates = []*http.Client{direct, system}
			if custom != nil {
				candidates = append(candidates, custom)
			}
		} else {
			if custom != nil {
				candidates = []*http.Client{custom, direct}
			} else {
				candidates = []*http.Client{system, direct}
			}
		}
	}

	ua := strings.TrimSpace(hdrs.UserAgent)
	if ua == "" {
		ua = strings.TrimSpace(cfg.UserAgent)
	}
	if ua == "" {
		ua = defaultUserAgent
	}

	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", ua)
		if hdrs.Referer != "" {
			req.Header.Set("Referer", hdrs.Referer)
		}
		if hdrs.Range != "" {
			req.Header.Set("Range", hdrs.Range)
		}
		if hdrs.IfRange != "" {
			req.Header.Set("If-Range", hdrs.IfRange)
		}
		if hdrs.Accept != "" {
			req.Header.Set("Accept", hdrs.Accept)
		}
		if hdrs.Identity {
			req.Header.Set("Accept-Encoding", "identity")
		}
		return req, nil
	}

	var lastErr error
	for i, client := range candidates {
		if client == nil {
			continue
		}
		req, err := buildReq()
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if resp.StatusCode >= 500 && i < len(candidates)-1 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %s", resp.Status)
			continue
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no HTTP client available")
	}
	return nil, lastErr
}

func (nc *netClients) fetchBytes(ctx context.Context, rawURL string, cfg NetConfig, maxBytes int64) ([]byte, string, string, error) {
	resp, err := nc.doSmart(ctx, http.MethodGet, rawURL, requestHeaders{Accept: "*/*"}, cfg, false, false)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("HTTP %s for %s", resp.Status, rawURL)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", "", err
	}
	if int64(len(b)) > maxBytes {
		return nil, "", "", fmt.Errorf("response from %s exceeds %d bytes", rawURL, maxBytes)
	}
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return b, finalURL, resp.Header.Get("Content-Type"), nil
}
