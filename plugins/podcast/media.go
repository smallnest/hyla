package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxCoverBytes      = 5 << 20
	maxProbeRangeBytes = 64 << 10
)

type MediaServer struct {
	store     *Store
	proxyFn   func() string
	token     string
	baseURL   string
	ln        net.Listener
	srv       *http.Server
	coverMu   sync.RWMutex
	coverURLs map[string]string

	clientMu       sync.Mutex
	cachedProxy    string
	streamClient   *http.Client
	coverClient    *http.Client
	streamTrans    *http.Transport
	coverTransport *http.Transport
}

func NewMediaServer(store *Store, proxyFn func() string) (*MediaServer, error) {
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
		store:     store,
		proxyFn:   proxyFn,
		token:     token,
		baseURL:   "http://" + ln.Addr().String() + "/" + token,
		ln:        ln,
		coverURLs: make(map[string]string),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/"+token+"/audio", ms.handleAudio)
	mux.HandleFunc("/"+token+"/cover", ms.handleCover)
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
	if ms == nil {
		return
	}
	if ms.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = ms.srv.Shutdown(ctx)
	}
	ms.clientMu.Lock()
	if ms.streamTrans != nil {
		ms.streamTrans.CloseIdleConnections()
		ms.streamTrans = nil
	}
	if ms.coverTransport != nil {
		ms.coverTransport.CloseIdleConnections()
		ms.coverTransport = nil
	}
	ms.streamClient = nil
	ms.coverClient = nil
	ms.clientMu.Unlock()
}

func (ms *MediaServer) BaseURL() string {
	if ms == nil {
		return ""
	}
	return ms.baseURL
}

func (ms *MediaServer) RegisterCover(rawURL string) string {
	if ms == nil {
		return ""
	}
	u, err := validateHTTPURL(rawURL)
	if err != nil {
		return ""
	}
	clean := u.String()
	key := shortHash(clean)
	ms.coverMu.Lock()
	ms.coverURLs[key] = clean
	ms.coverMu.Unlock()
	return ms.baseURL + "/cover?id=" + url.QueryEscape(key)
}

func (ms *MediaServer) PlayURL(episodeID string) string {
	if ms == nil || episodeID == "" {
		return ""
	}
	return ms.baseURL + "/audio?id=" + url.QueryEscape(episodeID)
}

func (ms *MediaServer) currentProxy() string {
	if ms.proxyFn != nil {
		return strings.TrimSpace(ms.proxyFn())
	}
	return ""
}

func (ms *MediaServer) clients() (streamClient, coverClient *http.Client) {
	proxy := ms.currentProxy()
	ms.clientMu.Lock()
	defer ms.clientMu.Unlock()
	if ms.streamClient != nil && ms.coverClient != nil && ms.cachedProxy == proxy {
		return ms.streamClient, ms.coverClient
	}
	if ms.streamTrans != nil {
		ms.streamTrans.CloseIdleConnections()
	}
	if ms.coverTransport != nil {
		ms.coverTransport.CloseIdleConnections()
	}
	sc := streamingHTTPClient(proxy, 0)
	cc := httpClient(proxy, 20*time.Second)
	ms.cachedProxy = proxy
	ms.streamClient = sc
	ms.coverClient = cc
	ms.streamTrans, _ = sc.Transport.(*http.Transport)
	ms.coverTransport, _ = cc.Transport.(*http.Transport)
	return sc, cc
}

func (ms *MediaServer) handleAudio(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, If-Range")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	epID := strings.TrimSpace(r.URL.Query().Get("id"))
	if epID == "" {
		http.NotFound(w, r)
		return
	}
	ep, ok := ms.store.GetEpisode(epID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if ep.DownloadedFile != "" {
		localPath := ms.store.DownloadPath(ep.DownloadedFile)
		if f, err := os.Open(localPath); err == nil {
			defer f.Close()
			if fi, err := f.Stat(); err == nil && !fi.IsDir() {
				if ct := resolveAudioContentType("", ep.AudioType, localPath, ep.AudioURL); ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				http.ServeContent(w, r, filepath.Base(localPath), fi.ModTime(), f)
				return
			}
		}
	}

	if _, err := validateHTTPURL(ep.AudioURL); err != nil {
		http.NotFound(w, r)
		return
	}
	upReq, err := newHTTPRequest(r.Context(), r.Method, ep.AudioURL, nil)
	if err != nil {
		log.Printf("podcast media: build request failed for %s: %v", ep.ID, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	upReq.Header.Set("Accept", "*/*")
	upReq.Header.Set("Accept-Encoding", "identity")
	reqRange := strings.TrimSpace(r.Header.Get("Range"))
	if reqRange != "" {
		upReq.Header.Set("Range", reqRange)
	}
	if ifRng := strings.TrimSpace(r.Header.Get("If-Range")); ifRng != "" {
		upReq.Header.Set("If-Range", ifRng)
	}

	client, _ := ms.clients()
	resp, err := client.Do(upReq)
	if err != nil {
		log.Printf("podcast media: upstream request failed for %s (%s): %v", ep.ID, ep.AudioURL, err)
		http.Error(w, fmt.Sprintf("upstream audio error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("podcast media: upstream HTTP %d for %s (%s)", resp.StatusCode, ep.ID, ep.AudioURL)
		for _, hdr := range []string{"Content-Type", "Content-Length"} {
			if v := resp.Header.Get(hdr); v != "" {
				w.Header().Set(hdr, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, io.LimitReader(resp.Body, 4096))
		}
		return
	}

	finalURL := ep.AudioURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	ct := resolveAudioContentType(resp.Header.Get("Content-Type"), ep.AudioType, finalURL, ep.AudioURL)

	// macOS AVFoundation sends an initial "Range: bytes=0-1" probe and expects 206 Partial Content.
	// If an upstream podcast server ignores Range and returns 200 OK, synthesize a valid 206 response
	// for small zero-based probes so playback never fails with MEDIA_ERR_SRC_NOT_SUPPORTED.
	if resp.StatusCode == http.StatusOK && r.Method == http.MethodGet {
		if end, ok := parseSmallZeroRange(reqRange); ok {
			need := end + 1
			buf, rerr := io.ReadAll(io.LimitReader(resp.Body, need))
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
	buf := make([]byte, 32<<10)
	_, _ = io.CopyBuffer(fw, resp.Body, buf)
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
	if err != nil || end < 0 || end >= maxProbeRangeBytes {
		return 0, false
	}
	return end, true
}

func isPlayableMediaMIME(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	mt, _, err := mime.ParseMediaType(raw)
	if err != nil {
		mt = strings.ToLower(strings.TrimSpace(strings.Split(raw, ";")[0]))
	} else {
		mt = strings.ToLower(mt)
	}
	if strings.HasPrefix(mt, "audio/") || strings.HasPrefix(mt, "video/") || mt == "application/ogg" {
		return mt, true
	}
	return "", false
}

func audioMIMEFromPathOrURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	p := raw
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		p = u.Path
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".m4b", ".mp4":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".opus":
		return "audio/opus"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".webm":
		return "audio/webm"
	default:
		return ""
	}
}

func resolveAudioContentType(upstreamCT, declaredType string, sources ...string) string {
	if mt, ok := isPlayableMediaMIME(upstreamCT); ok {
		return mt
	}
	if mt, ok := isPlayableMediaMIME(declaredType); ok {
		return mt
	}
	for _, src := range sources {
		if mt := audioMIMEFromPathOrURL(src); mt != "" {
			return mt
		}
	}
	return "audio/mpeg"
}

func (ms *MediaServer) handleCover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
	ms.coverMu.RLock()
	rawURL, ok := ms.coverURLs[key]
	ms.coverMu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")

	cacheFile := filepath.Join(ms.store.coversDir(), filepath.Base(key))
	if data, err := os.ReadFile(cacheFile); err == nil && len(data) > 0 {
		ct := http.DetectContentType(data)
		w.Header().Set("Content-Type", ct)
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
		return
	}

	_, client := ms.clients()
	data, _, err := fetchBytes(r.Context(), client, rawURL, maxCoverBytes)
	if err != nil {
		log.Printf("podcast media: cover fetch failed (%s): %v", rawURL, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	_ = os.WriteFile(cacheFile, data, 0o644)
	ct := http.DetectContentType(data)
	w.Header().Set("Content-Type", ct)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}
