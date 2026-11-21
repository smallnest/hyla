package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const userAgent = "Hyla-Podcast/1.0 (+https://github.com/smallnest/hyla)"

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

func httpTransport(proxyOverride string) *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.DialContext = (&net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	base.TLSHandshakeTimeout = 15 * time.Second
	base.ResponseHeaderTimeout = 30 * time.Second
	override := strings.TrimSpace(proxyOverride)
	base.Proxy = func(req *http.Request) (*url.URL, error) {
		if req != nil && req.URL != nil && isLocalHost(req.URL.Hostname()) {
			return nil, nil
		}
		if override != "" {
			u, err := url.Parse(override)
			if err == nil && u.Scheme != "" && u.Host != "" {
				return u, nil
			}
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
	return base
}

func httpClient(proxyOverride string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: httpTransport(proxyOverride),
	}
}

func streamingHTTPClient(proxyOverride string, timeout time.Duration) *http.Client {
	tr := httpTransport(proxyOverride)
	tr.DisableCompression = true
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}
}

func newHTTPRequest(ctx context.Context, method, rawURL string, body io.Reader) (*http.Request, error) {
	u, err := validateHTTPURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func fetchBytes(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, string, error) {
	req, err := newHTTPRequest(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %s for %s", resp.Status, rawURL)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(b)) > maxBytes {
		return nil, "", fmt.Errorf("response from %s exceeds %d bytes", rawURL, maxBytes)
	}
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return b, finalURL, nil
}
