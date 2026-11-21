package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxLogoBytes      = 2 << 20
	maxHLSManifestB   = 4 << 20
	maxHLSEntries     = 4096
	maxProbeRangeB    = 64 << 10
)

type hlsTarget struct {
	URL       string
	UserAgent string
	Referer   string
}

type MediaServer struct {
	store     *Store
	netc      *netClients
	cfgFn     func() NetConfig
	token     string
	baseURL   string
	ln        net.Listener
	srv       *http.Server
	logoSem   chan struct{}

	logoMu   sync.RWMutex
	logoURLs map[string]string

	hlsMu      sync.RWMutex
	hlsTargets map[string]hlsTarget
	hlsOrder   []string
}

func NewMediaServer(store *Store, netc *netClients, cfgFn func() NetConfig) (*MediaServer, error) {
	tokBytes := make([]byte, 16)
	if _, err := rand.Read(tokBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokBytes)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ms := &MediaServer{
		store:      store,
		netc:       netc,
		cfgFn:      cfgFn,
		token:      token,
		baseURL:    "http://" + ln.Addr().String() + "/" + token,
		ln:         ln,
		logoSem:    make(chan struct{}, 12),
		logoURLs:   make(map[string]string),
		hlsTargets: make(map[string]hlsTarget),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/"+token+"/play/", ms.handlePlay)
	mux.HandleFunc("/"+token+"/hls/", ms.handleHLS)
	mux.HandleFunc("/"+token+"/logo", ms.handleLogo)
	ms.srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		_ = ms.srv.Serve(ln)
	}()
	return ms, nil
}

func (ms *MediaServer) Close() {
	if ms == nil || ms.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = ms.srv.Shutdown(ctx)
}

func (ms *MediaServer) currentCfg() NetConfig {
	if ms.cfgFn != nil {
		return ms.cfgFn()
	}
	return defaultNetConfig()
}

func (ms *MediaServer) RegisterLogo(rawURL string) string {
	if ms == nil || rawURL == "" {
		return ""
	}
	u, err := validateHTTPURL(rawURL)
	if err != nil {
		return ""
	}
	clean := u.String()
	key := shortHash(clean)
	ms.logoMu.Lock()
	ms.logoURLs[key] = clean
	ms.logoMu.Unlock()
	return ms.baseURL + "/logo?id=" + url.QueryEscape(key)
}

func (ms *MediaServer) PlayURL(channelID string, lineIdx int, streamURL string) string {
	if ms == nil || channelID == "" {
		return ""
	}
	ext := inferStreamExt(streamURL)
	return fmt.Sprintf("%s/play/%s/%d/stream%s", ms.baseURL, url.PathEscape(channelID), lineIdx, ext)
}

func inferStreamExt(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil {
		ext := strings.ToLower(path.Ext(u.Path))
		switch ext {
		case ".m3u8", ".mp4", ".m4v", ".mov", ".webm", ".ts", ".aac", ".mp3", ".flv":
			return ext
		}
		if strings.Contains(strings.ToLower(rawURL), "m3u8") {
			return ".m3u8"
		}
	}
	// Default extension helps AVFoundation probe HLS streams even on extensionless gateway URLs.
	return ".m3u8"
}

func (ms *MediaServer) registerHLSTarget(target hlsTarget) string {
	sum := sha256.Sum256([]byte(target.URL + "|" + target.UserAgent + "|" + target.Referer))
	key := hex.EncodeToString(sum[:16])

	ext := ""
	if u, err := url.Parse(target.URL); err == nil {
		e := strings.ToLower(path.Ext(u.Path))
		switch e {
		case ".m3u8", ".m3u", ".ts", ".m4s", ".m4a", ".m4v", ".mp4", ".aac", ".vtt", ".key":
			ext = e
		}
	}

	ms.hlsMu.Lock()
	if _, exists := ms.hlsTargets[key]; !exists {
		ms.hlsTargets[key] = target
		ms.hlsOrder = append(ms.hlsOrder, key)
		for len(ms.hlsOrder) > maxHLSEntries {
			old := ms.hlsOrder[0]
			ms.hlsOrder = ms.hlsOrder[1:]
			delete(ms.hlsTargets, old)
		}
	}
	ms.hlsMu.Unlock()
	return ms.baseURL + "/hls/" + key + ext
}

func (ms *MediaServer) lookupHLSTarget(key string) (hlsTarget, bool) {
	ms.hlsMu.RLock()
	t, ok := ms.hlsTargets[key]
	ms.hlsMu.RUnlock()
	return t, ok
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, If-Range")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, Content-Type")
}

func (ms *MediaServer) handlePlay(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prefix := "/" + ms.token + "/play/"
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	chID := parts[0]
	lineIdx, err := strconv.Atoi(parts[1])
	if err != nil || lineIdx < 0 {
		http.NotFound(w, r)
		return
	}
	ch, ok := ms.store.GetChannel(chID)
	if !ok || lineIdx >= len(ch.Streams) {
		http.NotFound(w, r)
		return
	}
	line := ch.Streams[lineIdx]
	ms.proxyMedia(w, r, hlsTarget{
		URL:       line.URL,
		UserAgent: line.UserAgent,
		Referer:   line.Referer,
	})
}

func (ms *MediaServer) handleHLS(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prefix := "/" + ms.token + "/hls/"
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	key := rest
	if dot := strings.IndexByte(key, '.'); dot > 0 {
		key = key[:dot]
	}
	key = strings.TrimSpace(key)
	target, ok := ms.lookupHLSTarget(key)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ms.proxyMedia(w, r, target)
}

func (ms *MediaServer) proxyMedia(w http.ResponseWriter, r *http.Request, target hlsTarget) {
	cfg := ms.currentCfg()
	reqRange := strings.TrimSpace(r.Header.Get("Range"))
	hdrs := requestHeaders{
		UserAgent: target.UserAgent,
		Referer:   target.Referer,
		Range:     reqRange,
		IfRange:   strings.TrimSpace(r.Header.Get("If-Range")),
		Accept:    "*/*",
		Identity:  true,
	}
	resp, err := ms.netc.doSmart(r.Context(), r.Method, target.URL, hdrs, cfg, true, true)
	if err != nil {
		log.Printf("iptv media: upstream failed (%s): %v", target.URL, err)
		http.Error(w, fmt.Sprintf("upstream media error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("iptv media: upstream HTTP %d (%s)", resp.StatusCode, target.URL)
		w.WriteHeader(resp.StatusCode)
		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, io.LimitReader(resp.Body, 4096))
		}
		return
	}

	finalURL := target.URL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	upCT := strings.ToLower(resp.Header.Get("Content-Type"))
	var prefixBuf []byte
	if r.Method == http.MethodGet && maybeHLSResponse(upCT, finalURL, reqRange) {
		head := make([]byte, 512)
		n, perr := io.ReadAtLeast(resp.Body, head, 7)
		if n > 0 {
			prefixBuf = head[:n]
			trimmedHead := strings.TrimSpace(strings.TrimPrefix(string(prefixBuf), "\xef\xbb\xbf"))
			if strings.HasPrefix(strings.ToUpper(trimmedHead), "#EXTM3U") {
				rest, rerr := io.ReadAll(io.LimitReader(resp.Body, maxHLSManifestB))
				if rerr != nil {
					http.Error(w, rerr.Error(), http.StatusBadGateway)
					return
				}
				full := strings.TrimSpace(strings.TrimPrefix(string(append(prefixBuf, rest...)), "\xef\xbb\xbf"))
				rewritten := ms.rewriteHLSManifest(full, finalURL, target)
				w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Content-Length", strconv.Itoa(len(rewritten)))
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, rewritten)
				return
			}
		} else if perr != nil && perr != io.EOF && perr != io.ErrUnexpectedEOF {
			http.Error(w, perr.Error(), http.StatusBadGateway)
			return
		}
	}

	bodyReader := io.Reader(resp.Body)
	if len(prefixBuf) > 0 {
		bodyReader = io.MultiReader(bytes.NewReader(prefixBuf), resp.Body)
	}

	ct := resolveVideoContentType(resp.Header.Get("Content-Type"), finalURL)
	if resp.StatusCode == http.StatusOK && r.Method == http.MethodGet {
		if end, ok := parseSmallZeroRange(reqRange); ok {
			buf, rerr := io.ReadAll(io.LimitReader(bodyReader, end+1))
			if rerr == nil && len(buf) > 0 {
				if ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				total := "*"
				if resp.ContentLength > 0 {
					total = strconv.FormatInt(resp.ContentLength, 10)
				}
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%s", len(buf)-1, total))
				w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(buf)
				return
			}
		}
	}

	for _, hdr := range []string{"Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if v := resp.Header.Get(hdr); v != "" {
			w.Header().Set(hdr, v)
		}
	}
	if resp.StatusCode == http.StatusPartialContent && w.Header().Get("Content-Length") == "" {
		if cl, ok := contentLengthFromRange(w.Header().Get("Content-Range")); ok {
			w.Header().Set("Content-Length", strconv.FormatInt(cl, 10))
		}
	}
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if w.Header().Get("Accept-Ranges") == "" {
		w.Header().Set("Accept-Ranges", "bytes")
	}

	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	fw := newFlushingWriter(w)
	_, _ = io.CopyBuffer(fw, bodyReader, make([]byte, 32<<10))
}

func maybeHLSResponse(ct, finalURL, reqRange string) bool {
	if reqRange != "" && !strings.HasPrefix(reqRange, "bytes=0-") {
		return false
	}
	if strings.Contains(ct, "mpegurl") || strings.Contains(ct, "x-mpegurl") {
		return true
	}
	if u, err := url.Parse(finalURL); err == nil {
		ext := strings.ToLower(path.Ext(u.Path))
		switch ext {
		case ".m3u8", ".m3u":
			return true
		case ".ts", ".mp4", ".m4v", ".m4s", ".mov", ".webm", ".flv", ".aac", ".mp3", ".mkv":
			return false
		}
	}
	// Extensionless gateway URLs (e.g. /channel/cctv17?...) get magic-byte sniff.
	return strings.Contains(ct, "text/plain") || (!strings.Contains(ct, "video/") && !strings.Contains(ct, "audio/") && !strings.Contains(ct, "mp2t"))
}

var hlsURIAttrRe = regexp.MustCompile(`URI="([^"]+)"`)

func (ms *MediaServer) rewriteHLSManifest(manifestText, baseURLStr string, parent hlsTarget) string {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return manifestText
	}
	resolveAndRegister := func(rawURI string) string {
		rawURI = strings.TrimSpace(rawURI)
		if rawURI == "" || strings.HasPrefix(strings.ToLower(rawURI), "data:") {
			return rawURI
		}
		ref, rerr := url.Parse(rawURI)
		if rerr != nil {
			return rawURI
		}
		abs := baseURL.ResolveReference(ref)
		if !strings.EqualFold(abs.Scheme, "http") && !strings.EqualFold(abs.Scheme, "https") {
			return rawURI
		}
		return ms.registerHLSTarget(hlsTarget{
			URL:       abs.String(),
			UserAgent: parent.UserAgent,
			Referer:   parent.Referer,
		})
	}

	norm := strings.ReplaceAll(strings.ReplaceAll(manifestText, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(norm, "\n")
	out := make([]string, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if strings.Contains(line, `URI="`) {
				line = hlsURIAttrRe.ReplaceAllStringFunc(line, func(m string) string {
					sub := hlsURIAttrRe.FindStringSubmatch(m)
					if len(sub) != 2 {
						return m
					}
					return `URI="` + resolveAndRegister(sub[1]) + `"`
				})
			}
			out = append(out, line)
			continue
		}
		out = append(out, resolveAndRegister(line))
	}
	return strings.Join(out, "\n") + "\n"
}

func resolveVideoContentType(upstreamCT, rawURL string) string {
	lowCT := strings.ToLower(strings.TrimSpace(strings.Split(upstreamCT, ";")[0]))
	if lowCT != "" && lowCT != "application/octet-stream" && lowCT != "binary/octet-stream" {
		return upstreamCT
	}
	if u, err := url.Parse(rawURL); err == nil {
		switch strings.ToLower(path.Ext(u.Path)) {
		case ".m3u8", ".m3u":
			return "application/vnd.apple.mpegurl"
		case ".ts":
			return "video/mp2t"
		case ".mp4", ".m4v", ".m4s":
			return "video/mp4"
		case ".webm":
			return "video/webm"
		case ".aac":
			return "audio/aac"
		case ".mp3":
			return "audio/mpeg"
		case ".flv":
			return "video/x-flv"
		}
	}
	return "video/mp2t"
}

func parseSmallZeroRange(hdr string) (int64, bool) {
	hdr = strings.TrimSpace(hdr)
	if !strings.HasPrefix(strings.ToLower(hdr), "bytes=") {
		return 0, false
	}
	spec := strings.TrimSpace(hdr[len("bytes="):])
	if strings.Contains(spec, ",") {
		return 0, false
	}
	startStr, endStr, ok := strings.Cut(spec, "-")
	if !ok || strings.TrimSpace(startStr) != "0" || strings.TrimSpace(endStr) == "" {
		return 0, false
	}
	end, err := strconv.ParseInt(strings.TrimSpace(endStr), 10, 64)
	if err != nil || end < 0 || end >= maxProbeRangeB {
		return 0, false
	}
	return end, true
}

func contentLengthFromRange(cr string) (int64, bool) {
	cr = strings.TrimSpace(cr)
	if !strings.HasPrefix(strings.ToLower(cr), "bytes ") {
		return 0, false
	}
	spec := strings.TrimSpace(cr[len("bytes "):])
	rngPart, _, ok := strings.Cut(spec, "/")
	if !ok {
		return 0, false
	}
	startStr, endStr, ok := strings.Cut(rngPart, "-")
	if !ok {
		return 0, false
	}
	start, err1 := strconv.ParseInt(strings.TrimSpace(startStr), 10, 64)
	end, err2 := strconv.ParseInt(strings.TrimSpace(endStr), 10, 64)
	if err1 != nil || err2 != nil || start < 0 || end < start {
		return 0, false
	}
	return end - start + 1, true
}

type flushingWriter struct {
	w  io.Writer
	fl http.Flusher
}

func newFlushingWriter(w http.ResponseWriter) io.Writer {
	fl, _ := w.(http.Flusher)
	if fl == nil {
		return w
	}
	return &flushingWriter{w: w, fl: fl}
}

func (f *flushingWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if n > 0 && f.fl != nil {
		f.fl.Flush()
	}
	return n, err
}

func (ms *MediaServer) handleLogo(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("id"))
	if key == "" {
		http.NotFound(w, r)
		return
	}
	ms.logoMu.RLock()
	rawURL, ok := ms.logoURLs[key]
	ms.logoMu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=604800")

	cacheFile := filepath.Join(ms.store.logosDir(), filepath.Base(key))
	if data, err := os.ReadFile(cacheFile); err == nil && len(data) > 0 {
		w.Header().Set("Content-Type", detectLogoContentType(data, rawURL))
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
		return
	}

	select {
	case ms.logoSem <- struct{}{}:
		defer func() { <-ms.logoSem }()
	case <-r.Context().Done():
		http.Error(w, "canceled", http.StatusRequestTimeout)
		return
	}

	// Double check cache in case another goroutine finished while waiting on semaphore.
	if data, err := os.ReadFile(cacheFile); err == nil && len(data) > 0 {
		w.Header().Set("Content-Type", detectLogoContentType(data, rawURL))
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := ms.netc.doSmart(ctx, http.MethodGet, rawURL, requestHeaders{Accept: "image/*,*/*"}, ms.currentCfg(), true, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, resp.Status, http.StatusBadGateway)
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxLogoBytes+1))
	if err != nil || len(data) == 0 || int64(len(data)) > maxLogoBytes {
		http.Error(w, "invalid logo response", http.StatusBadGateway)
		return
	}
	_ = os.WriteFile(cacheFile, data, 0o644)
	w.Header().Set("Content-Type", detectLogoContentType(data, rawURL))
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func detectLogoContentType(data []byte, rawURL string) string {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "<svg") || strings.Contains(strings.ToLower(trimmed), "<svg") {
		return "image/svg+xml"
	}
	if u, err := url.Parse(rawURL); err == nil && strings.EqualFold(path.Ext(u.Path), ".webp") {
		return "image/webp"
	}
	return http.DetectContentType(data)
}
