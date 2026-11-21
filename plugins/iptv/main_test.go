package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

func TestParseM3UAndTXTGenre(t *testing.T) {
	m3u := `#EXTM3U x-tvg-url="https://epg.example.com/guide.xml"
#EXTINF:-1 tvg-id="CCTV1" tvg-name="CCTV-1" tvg-logo="https://img.example.com/cctv1.png" group-title="央视频道", CCTV-1 综合
#EXTVLCOPT:http-user-agent=CustomTV/1.0
#EXTVLCOPT:http-referrer=https://tv.example.com/
https://cdn.example.com/cctv1/index.m3u8$HD高清
#EXTINF:-1 tvg-name="Broken" group-title="其他", 跳过无效地址
rtp://239.0.0.1:5000
#EXTINF:-1 tvg-name="BBC" tvg-logo="https://img.example.com/bbc.png" group-title="News", BBC World News
https://cdn.example.com/bbc/playlist.m3u8
#EXTINF:-1 tvg-id="CCTV-1" tvg-name="CCTV-1" group-title="更新时间",2026-08-27 14:06:22
https://cdn.example.com/notice/index.m3u8
`
	raws, epg, err := ParseM3U([]byte(m3u))
	if err != nil {
		t.Fatalf("ParseM3U error: %v", err)
	}
	if epg != "https://epg.example.com/guide.xml" {
		t.Fatalf("unexpected epgURL=%q", epg)
	}
	if len(raws) != 2 {
		t.Fatalf("expected 2 valid HTTP(S) channels, got %d (%+v)", len(raws), raws)
	}
	if raws[0].Name != "CCTV-1 综合" || raws[0].LogoURL != "https://img.example.com/cctv1.png" || raws[0].Group != "央视频道" {
		t.Fatalf("unexpected channel 0: %+v", raws[0])
	}
	if raws[0].UserAgent != "CustomTV/1.0" || raws[0].Referer != "https://tv.example.com/" || raws[0].Label != "HD高清" || raws[0].StreamURL != "https://cdn.example.com/cctv1/index.m3u8" {
		t.Fatalf("unexpected VLC/label options: %+v", raws[0])
	}

	txt := `卫视频道,#genre#
湖南卫视,https://a.example.com/hunan.m3u8$主线路#https://b.example.com/hunan.m3u8$备线路
`
	txts, err := ParseTXTGenre([]byte(txt))
	if err != nil || len(txts) != 2 || txts[0].Group != "卫视频道" || txts[1].Label != "备线路" {
		t.Fatalf("ParseTXTGenre unexpected: %+v err=%v", txts, err)
	}

	exported := ExportM3U([]Channel{{
		ID:      "ch-1",
		Name:    "CCTV-1 综合",
		TVGID:   "CCTV1",
		LogoURL: "https://img.example.com/cctv1.png",
		Group:   "央视频道",
		Streams: []StreamLine{{URL: "https://cdn.example.com/cctv1/index.m3u8", UserAgent: "CustomTV/1.0"}},
	}})
	reParsed, _, err := ParseM3U([]byte(exported))
	if err != nil || len(reParsed) != 1 || reParsed[0].UserAgent != "CustomTV/1.0" {
		t.Fatalf("ExportM3U round-trip failed: %+v err=%v", reParsed, err)
	}
}

func TestHTMLAutoDiscoveryAndMergeChannels(t *testing.T) {
	var srvURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/domestic/ipv4/all/m3u", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Mimics a Next.js page containing static scripts and an embedded .m3u playlist URL.
		_, _ = io.WriteString(w, `<!doctype html><html><head><script src="/_next/app.js"></script></head><body><a href="`+srvURL+`/tv/iptv4.m3u">Download M3U</a></body></html>`)
	})
	mux.HandleFunc("/tv/iptv4.m3u", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `#EXTM3U
#EXTINF:-1 tvg-logo="https://img.example.com/cctv13.png" group-title="央视频道", CCTV13
https://s1.example.com/cctv13.m3u8
#EXTINF:-1 tvg-logo="https://img.example.com/hunan.png" group-title="卫视频道", 湖南卫视
https://s1.example.com/hunan.m3u8
`)
	})
	mux.HandleFunc("/extra.m3u", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `#EXTM3U
#EXTINF:-1 group-title="央视频道", CCTV13
https://s2.example.com/cctv13-backup.m3u8
#EXTINF:-1 tvg-logo="https://img.example.com/doc.png" group-title="纪录频道", CCTV9
https://s2.example.com/cctv9.m3u8
`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	nc := newNetClients()
	cfg := NetConfig{ProxyMode: "direct"}

	src1, chs1, err := ResolveAndFetchSource(context.Background(), nc, cfg, srv.URL+"/domestic/ipv4/all/m3u", "")
	if err != nil {
		t.Fatalf("HTML auto-discovery failed: %v", err)
	}
	if src1.ResolvedURL != srv.URL+"/tv/iptv4.m3u" || len(chs1) != 2 {
		t.Fatalf("unexpected resolved source: %+v len=%d", src1, len(chs1))
	}

	src2, chs2, err := ResolveAndFetchSource(context.Background(), nc, cfg, srv.URL+"/extra.m3u", "Backup")
	if err != nil {
		t.Fatalf("extra source fetch failed: %v", err)
	}

	merged, groups := MergeChannels([]Source{src1, src2}, map[string][]RawChannel{
		src1.ID: chs1,
		src2.ID: chs2,
	})
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged channels, got %d", len(merged))
	}
	// CCTV13 should consolidate both streams while keeping the logo from src1.
	if merged[0].Name != "CCTV13" || len(merged[0].Streams) != 2 || merged[0].LogoURL == "" {
		t.Fatalf("unexpected merged CCTV13: %+v", merged[0])
	}
	if len(groups) != 3 || groups[0] != "央视频道" || groups[1] != "卫视频道" || groups[2] != "纪录频道" {
		t.Fatalf("unexpected priority group ordering: %v", groups)
	}
}

func TestMediaServerHLSRewriteRangeAndLogos(t *testing.T) {
	tsSegment := []byte{0x47, 0x40, 0x00, 0x10, 0xAA, 0xBB, 0xCC, 0xDD}
	keyBytes := []byte("1234567890abcdef")

	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = io.WriteString(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\nsub/media.m3u8?auth=1\n")
	})
	mux.HandleFunc("/sub/media.m3u8", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("auth") != "1" {
			http.Error(w, "missing auth query", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = io.WriteString(w, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-KEY:METHOD=AES-128,URI=\"/enc.key\"\n#EXTINF:6.0,\nseg0.ts?tk=9\n")
	})
	mux.HandleFunc("/enc.key", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(keyBytes)
	})
	mux.HandleFunc("/sub/seg0.ts", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("tk") != "9" {
			http.Error(w, "missing token", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write(tsSegment)
	})
	mux.HandleFunc("/clip.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/12")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("MP4!"))
	})
	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	})

	up := httptest.NewServer(mux)
	defer up.Close()

	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := Source{ID: "src-1", Name: "Test", InputURL: up.URL + "/list.m3u", Enabled: true}
	if _, err := st.UpsertSource(src, []RawChannel{
		{Name: "Live HLS", Group: "央视频道", LogoURL: up.URL + "/logo.png", StreamURL: up.URL + "/master.m3u8"},
		{Name: "Loop MP4", Group: "电影频道", StreamURL: up.URL + "/clip.mp4"},
	}); err != nil {
		t.Fatal(err)
	}
	_, channels, _, _, _, _ := st.Snapshot()

	nc := newNetClients()
	ms, err := NewMediaServer(st, nc, func() NetConfig { return NetConfig{ProxyMode: "direct"} })
	if err != nil {
		t.Fatal(err)
	}
	defer ms.Close()

	var hlsCh, mp4Ch Channel
	for _, ch := range channels {
		if ch.Name == "Live HLS" {
			hlsCh = ch
		} else if ch.Name == "Loop MP4" {
			mp4Ch = ch
		}
	}

	// Wrong token rejected.
	playURL := ms.PlayURL(hlsCh.ID, 0, hlsCh.Streams[0].URL)
	badResp, err := http.Get(strings.Replace(playURL, ms.token, "badtok", 1))
	if err != nil {
		t.Fatal(err)
	}
	badResp.Body.Close()
	if badResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong token, got %d", badResp.StatusCode)
	}

	// Unknown HLS target key rejected (no open proxy).
	unknownHLS, err := http.Get(ms.baseURL + "/hls/notregistered.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	unknownHLS.Body.Close()
	if unknownHLS.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unregistered HLS key, got %d", unknownHLS.StatusCode)
	}

	// Master -> Sub Media Playlist -> AES Key + Segment TS end-to-end proxy rewrite.
	masterResp, err := http.Get(playURL)
	if err != nil {
		t.Fatal(err)
	}
	masterBody, _ := io.ReadAll(masterResp.Body)
	masterResp.Body.Close()
	if masterResp.StatusCode != http.StatusOK || !strings.Contains(string(masterBody), "/"+ms.token+"/hls/") || !strings.Contains(string(masterBody), ".m3u8") {
		t.Fatalf("master playlist rewrite failed: status=%d body=%q", masterResp.StatusCode, string(masterBody))
	}

	var subProxyURL string
	for _, line := range strings.Split(string(masterBody), "\n") {
		if strings.HasPrefix(line, "http://") {
			subProxyURL = strings.TrimSpace(line)
			break
		}
	}
	if subProxyURL == "" {
		t.Fatalf("missing rewritten sub playlist URL in:\n%s", string(masterBody))
	}

	subResp, err := http.Get(subProxyURL)
	if err != nil {
		t.Fatal(err)
	}
	subBody, _ := io.ReadAll(subResp.Body)
	subResp.Body.Close()
	if subResp.StatusCode != http.StatusOK || !strings.Contains(string(subBody), ".ts") || !strings.Contains(string(subBody), ".key") {
		t.Fatalf("sub playlist rewrite failed: status=%d body=%q", subResp.StatusCode, string(subBody))
	}

	var keyProxyURL, segProxyURL string
	for _, line := range strings.Split(string(subBody), "\n") {
		if strings.Contains(line, `URI="`) {
			_, rest, _ := strings.Cut(line, `URI="`)
			keyProxyURL, _, _ = strings.Cut(rest, `"`)
		} else if strings.HasPrefix(line, "http://") {
			segProxyURL = strings.TrimSpace(line)
		}
	}
	kResp, err := http.Get(keyProxyURL)
	if err != nil {
		t.Fatal(err)
	}
	kb, _ := io.ReadAll(kResp.Body)
	kResp.Body.Close()
	if string(kb) != string(keyBytes) {
		t.Fatalf("proxied key=%q, want %q", string(kb), string(keyBytes))
	}

	sResp, err := http.Get(segProxyURL)
	if err != nil {
		t.Fatal(err)
	}
	sb, _ := io.ReadAll(sResp.Body)
	sResp.Body.Close()
	if string(sb) != string(tsSegment) {
		t.Fatalf("proxied ts segment mismatch: got %x want %x", sb, tsSegment)
	}

	// Non-HLS 206 missing Content-Length derives Content-Length=4 from Content-Range.
	mp4Req, _ := http.NewRequest(http.MethodGet, ms.PlayURL(mp4Ch.ID, 0, mp4Ch.Streams[0].URL), nil)
	mp4Req.Header.Set("Range", "bytes=0-3")
	mp4Resp, err := http.DefaultClient.Do(mp4Req)
	if err != nil {
		t.Fatal(err)
	}
	mp4Bytes, _ := io.ReadAll(mp4Resp.Body)
	mp4Resp.Body.Close()
	if mp4Resp.StatusCode != http.StatusPartialContent || mp4Resp.Header.Get("Content-Length") != "4" || string(mp4Bytes) != "MP4!" {
		t.Fatalf("mp4 range proxy failed: status=%d cl=%q body=%q", mp4Resp.StatusCode, mp4Resp.Header.Get("Content-Length"), string(mp4Bytes))
	}

	// Logo proxy & disk cache.
	logoURL := ms.RegisterLogo(up.URL + "/logo.png")
	lResp, err := http.Get(logoURL)
	if err != nil {
		t.Fatal(err)
	}
	lBytes, _ := io.ReadAll(lResp.Body)
	lResp.Body.Close()
	if lResp.StatusCode != http.StatusOK || len(lBytes) == 0 || st.StorageUsage().LogosCount != 1 {
		t.Fatalf("logo proxy/cache failed: status=%d storage=%+v", lResp.StatusCode, st.StorageUsage())
	}
}

func TestPluginHarnessRPCAndAutoMute(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/s1.m3u", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "#EXTM3U\n#EXTINF:-1 group-title=\"央视频道\", CCTV1\nhttps://a.example.com/1.m3u8\n")
	})
	mux.HandleFunc("/s2.m3u", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "#EXTM3U\n#EXTINF:-1 group-title=\"央视频道\", CCTV1\nhttps://b.example.com/1.m3u8\n#EXTINF:-1 group-title=\"卫视频道\", 东方卫视\nhttps://b.example.com/df.m3u8\n")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	h := hylaplugin.NewTestHarness(newPlugin())
	defer h.Close()

	var mu sync.Mutex
	var opened []hylaplugin.UIOpenPaneParams
	var emitted []hylaplugin.WebViewEmitParams

	h.RegisterHostHandler("host.getState", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return hylaplugin.HostGetStateResult{
			ActiveSession: "s1",
			FocusedPane:   "p1",
			State: hylaplugin.State{
				Sessions: []hylaplugin.Session{
					{ID: "s1", Focus: "p1", Panes: []hylaplugin.Pane{{ID: "p1", Agent: "claude", Status: hylaplugin.StatusWorking}}},
				},
			},
		}, nil
	})
	h.RegisterHostHandler("ui.openPane", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p hylaplugin.UIOpenPaneParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		opened = append(opened, p)
		mu.Unlock()
		return struct{}{}, nil
	})
	h.RegisterHostHandler("webview.emit", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p hylaplugin.WebViewEmitParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		emitted = append(emitted, p)
		mu.Unlock()
		return struct{}{}, nil
	})
	h.RegisterHostHandler("plugin.getSettings", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return hylaplugin.PluginGetSettingsResult{
			Settings: map[string]any{
				"proxy_mode":         "direct",
				"auto_failover":      true,
				"auto_mute_on_agent": true,
			},
		}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	var initRes hylaplugin.InitializeResult
	if err := h.Call(ctx, "initialize", hylaplugin.InitializeParams{
		HylaVersion: "3.20.0",
		Host:        "app",
		PluginID:    pluginID,
		DataDir:     dataDir,
		Locale:      "zh-Hans",
		Settings: map[string]any{
			"proxy_mode":         "direct",
			"auto_failover":      true,
			"auto_mute_on_agent": true,
		},
	}, &initRes); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	var actRes hylaplugin.ActionInvokeResult
	if err := h.Call(ctx, "action.invoke", hylaplugin.ActionInvokeParams{
		ActionID:  "open",
		SessionID: "s1",
		PaneID:    "p1",
		Locale:    "zh-Hans",
	}, &actRes); err != nil || !actRes.OK {
		t.Fatalf("action.invoke open failed: res=%+v err=%v", actRes, err)
	}
	mu.Lock()
	if len(opened) != 1 || opened[0].Plugin != pluginID || opened[0].Title != "IPTV 直播" {
		t.Fatalf("unexpected opened pane: %+v", opened)
	}
	mu.Unlock()

	// Batch multi-line URL import
	addParams, _ := json.Marshal(map[string]string{
		"input": srv.URL + "/s1.m3u\n" + srv.URL + "/s2.m3u\n",
	})
	var rpcRes hylaplugin.WebViewRPCResult
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID: slotID,
		Method: "iptv.addSources",
		Params: addParams,
	}, &rpcRes); err != nil {
		t.Fatalf("iptv.addSources: %v", err)
	}
	var addOut struct {
		Added     int `json:"added"`
		Bootstrap struct {
			Sources  []Source  `json:"sources"`
			Channels []Channel `json:"channels"`
		} `json:"bootstrap"`
	}
	if err := json.Unmarshal(rpcRes.Data, &addOut); err != nil || addOut.Added != 2 || len(addOut.Bootstrap.Channels) != 2 {
		t.Fatalf("unexpected batch addSources output: %s err=%v", string(rpcRes.Data), err)
	}
	chID := addOut.Bootstrap.Channels[0].ID

	// Favorite + watch history + export
	favParams, _ := json.Marshal(map[string]string{"channelId": chID})
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{SlotID: slotID, Method: "iptv.toggleFavorite", Params: favParams}, &rpcRes); err != nil {
		t.Fatalf("iptv.toggleFavorite: %v", err)
	}
	watchParams, _ := json.Marshal(map[string]any{"channelId": chID, "lineIdx": 1})
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{SlotID: slotID, Method: "iptv.recordWatch", Params: watchParams}, &rpcRes); err != nil {
		t.Fatalf("iptv.recordWatch: %v", err)
	}
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{SlotID: slotID, Method: "iptv.exportM3U", Params: json.RawMessage(`{}`)}, &rpcRes); err != nil {
		t.Fatalf("iptv.exportM3U: %v", err)
	}
	if !strings.Contains(string(rpcRes.Data), "https://b.example.com/1.m3u8") {
		t.Fatalf("exportM3U missing backup stream: %s", string(rpcRes.Data))
	}

	if _, err := os.Stat(filepath.Join(dataDir, "state.json")); err != nil {
		t.Fatalf("expected state.json persisted: %v", err)
	}

	// Coding agent working -> idle triggers iptv.autoMute
	_ = h.Call(ctx, "event.notify", hylaplugin.EventNotifyParams{
		Topic: "pane.status",
		Data:  json.RawMessage(`{"sessionId":"s1","paneId":"p1","agent":"claude","status":"working"}`),
	}, nil)
	_ = h.Call(ctx, "event.notify", hylaplugin.EventNotifyParams{
		Topic: "pane.status",
		Data:  json.RawMessage(`{"sessionId":"s1","paneId":"p1","agent":"claude","status":"idle"}`),
	}, nil)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(emitted)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(emitted) != 1 || emitted[0].Event != "iptv.autoMute" {
		t.Fatalf("expected iptv.autoMute emit, got %+v", emitted)
	}
}

func TestPluginHarnessEPG(t *testing.T) {
	start := time.Now().Add(-5 * time.Minute).In(time.FixedZone("CST", 8*3600))
	epgXML := `<?xml version="1.0" encoding="UTF-8"?><tv>` +
		`<programme channel="CCTV1" start="` + start.Format("20060102150405 -0700") + `" stop="` + start.Add(time.Hour).Format("20060102150405 -0700") + `"><title>新闻联播</title><desc>daily news</desc></programme>` +
		`<programme channel="CCTV1" start="` + start.Add(time.Hour).Format("20060102150405 -0700") + `" stop="` + start.Add(2*time.Hour).Format("20060102150405 -0700") + `"><title>电视剧</title></programme>` +
		`</tv>`

	mux := http.NewServeMux()
	mux.HandleFunc("/epg.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, epgXML)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	// The import refuses a stream that is not http(s): the stream names
	// the test server too.
	mux.HandleFunc("/s1.m3u", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `#EXTM3U x-tvg-url="http://`+r.Host+`/epg.xml"`+"\n"+
			`#EXTINF:-1 group-title="央视频道", CCTV-1 综合`+"\n"+
			"http://"+r.Host+"/live.m3u8\n")
	})

	h := hylaplugin.NewTestHarness(newPlugin())
	defer h.Close()
	var mu sync.Mutex
	var emitted []hylaplugin.WebViewEmitParams
	h.RegisterHostHandler("webview.emit", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p hylaplugin.WebViewEmitParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		emitted = append(emitted, p)
		mu.Unlock()
		return struct{}{}, nil
	})
	h.RegisterHostHandler("plugin.getSettings", func(ctx context.Context, _ json.RawMessage) (any, error) {
		return hylaplugin.PluginGetSettingsResult{Settings: map[string]any{"proxy_mode": "direct"}}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var initRes hylaplugin.InitializeResult
	if err := h.Call(ctx, "initialize", hylaplugin.InitializeParams{
		HylaVersion: "3.20.0", Host: "app", PluginID: pluginID,
		DataDir: t.TempDir(), Locale: "zh-Hans",
		Settings: map[string]any{"proxy_mode": "direct"},
	}, &initRes); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	addParams, _ := json.Marshal(map[string]string{"input": srv.URL + "/s1.m3u"})
	var rpcRes hylaplugin.WebViewRPCResult
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{SlotID: slotID, Method: "iptv.addSources", Params: addParams}, &rpcRes); err != nil {
		t.Fatalf("iptv.addSources: %v", err)
	}

	// The guide arrives after the import: poll the RPC, then wait for the emit.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{SlotID: slotID, Method: "iptv.programs", Params: json.RawMessage(`{}`)}, &rpcRes); err != nil {
			t.Fatalf("iptv.programs: %v", err)
		}
		var out struct {
			Programs map[string][]EPGProgram `json:"programs"`
			EPG      map[string]any          `json:"epg"`
		}
		if err := json.Unmarshal(rpcRes.Data, &out); err != nil {
			t.Fatalf("decode programs: %s err=%v", string(rpcRes.Data), err)
		}
		if list := out.Programs["cctv-1 综合"]; len(list) == 2 {
			if list[0].Title != "新闻联播" || list[1].Title != "电视剧" {
				t.Fatalf("unexpected guide order: %+v", list)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("guide did not load: %s", string(rpcRes.Data))
		}
		time.Sleep(25 * time.Millisecond)
	}
	// The emit comes from the fetch, after the programmes are in place.
	var gotEmit bool
	emitDeadline := time.Now().Add(5 * time.Second)
	for !gotEmit && time.Now().Before(emitDeadline) {
		mu.Lock()
		for _, ev := range emitted {
			if ev.Event == "iptv.epg" {
				gotEmit = true
			}
		}
		mu.Unlock()
		if !gotEmit {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if !gotEmit {
		t.Fatalf("expected an iptv.epg emit after the fetch, got %+v", emitted)
	}

	// A new pane reads the guide from the cache file.
	var boot hylaplugin.WebViewRPCResult
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{SlotID: slotID, Method: "iptv.bootstrap", Params: json.RawMessage(`{}`)}, &boot); err != nil {
		t.Fatalf("iptv.bootstrap: %v", err)
	}
	var snap struct {
		Programs map[string][]EPGProgram `json:"programs"`
		Channels []Channel               `json:"channels"`
	}
	if err := json.Unmarshal(boot.Data, &snap); err != nil || len(snap.Programs) == 0 || len(snap.Channels) != 1 {
		t.Fatalf("bootstrap without the guide: %s err=%v", string(boot.Data), err)
	}
}
