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
	"sync/atomic"
	"testing"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

func TestParseRSSAndAtomFeeds(t *testing.T) {
	rssXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
     xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"
     xmlns:podcast="https://podcastindex.org/namespace/1.0"
     xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>AI Builders Weekly</title>
    <link>https://example.com/show</link>
    <description><![CDATA[<p>Deep engineering talks.</p>]]></description>
    <language>en-us</language>
    <itunes:author>Alice &amp; Bob</itunes:author>
    <itunes:image href="https://example.com/cover.jpg"/>
    <item>
      <title>Episode 1: Agents in Practice</title>
      <guid>ep-1</guid>
      <link>https://example.com/ep1</link>
      <pubDate>Mon, 02 Jan 2026 15:04:05 +0000</pubDate>
      <itunes:duration>1:02:05</itunes:duration>
      <enclosure url="https://cdn.example.com/ep1.mp3" type="audio/mpeg" length="123456"/>
      <podcast:transcript url="https://cdn.example.com/ep1.vtt" type="text/vtt"/>
      <podcast:chapters url="https://cdn.example.com/ep1.chapters.json" type="application/json+chapters"/>
      <content:encoded><![CDATA[<p>00:00 Intro<br/>12:30 Tool Use</p>]]></content:encoded>
    </item>
  </channel>
</rss>`

	feed, eps, err := ParseFeed([]byte(rssXML), "https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("ParseFeed RSS error: %v", err)
	}
	if feed.Title != "AI Builders Weekly" || feed.Author != "Alice & Bob" || feed.ArtworkURL != "https://example.com/cover.jpg" {
		t.Fatalf("unexpected feed metadata: %+v", feed)
	}
	if len(eps) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(eps))
	}
	ep := eps[0]
	if ep.AudioURL != "https://cdn.example.com/ep1.mp3" || ep.DurationSec != 3725 {
		t.Fatalf("unexpected episode audio/duration: %+v", ep)
	}
	if ep.TranscriptURL != "https://cdn.example.com/ep1.vtt" || ep.ChaptersURL != "https://cdn.example.com/ep1.chapters.json" {
		t.Fatalf("unexpected podcast 2.0 tags: %+v", ep)
	}
	if len(ep.Chapters) != 2 || ep.Chapters[1].StartSec != 750 || ep.Chapters[1].Title != "Tool Use" {
		t.Fatalf("expected extracted show note chapters, got %+v", ep.Chapters)
	}

	atomXML := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Tech Cast</title>
  <link rel="alternate" href="https://example.com/atom-site"/>
  <author><name>Carol</name></author>
  <entry>
    <title>Atom Ep 1</title>
    <id>atom-1</id>
    <published>2026-03-01T10:00:00Z</published>
    <link rel="enclosure" href="https://cdn.example.com/atom1.m4a" type="audio/mp4" length="9000"/>
    <summary>Hello Atom</summary>
  </entry>
</feed>`

	af, aeps, err := ParseFeed([]byte(atomXML), "https://example.com/atom.xml")
	if err != nil {
		t.Fatalf("ParseFeed Atom error: %v", err)
	}
	if af.Title != "Atom Tech Cast" || af.Author != "Carol" || len(aeps) != 1 || aeps[0].AudioURL != "https://cdn.example.com/atom1.m4a" {
		t.Fatalf("unexpected Atom parse result: feed=%+v eps=%+v", af, aeps)
	}
}

func TestTranscriptParsersAndExports(t *testing.T) {
	vtt := "WEBVTT\n\n00:00:01.000 --> 00:00:04.500\n<v Host>Hello world.</v>\n\n00:05.000 --> 00:09.000\nSecond line."
	segs := ParseVTT(vtt)
	if len(segs) != 2 || segs[0].Text != "Hello world." || segs[1].StartSec != 5 {
		t.Fatalf("ParseVTT unexpected: %+v", segs)
	}

	srt := "1\n00:00:01,200 --> 00:00:03,400\nFirst cue\n\n2\n00:00:04,000 --> 00:00:06,000\nSecond cue\n\n3\n00:00:07,000 --> 00:00:09,000\nLine one\nLine two"
	srtSegs := ParseSRT(srt)
	if len(srtSegs) != 3 || srtSegs[0].StartSec < 1.19 || srtSegs[0].StartSec > 1.21 {
		t.Fatalf("ParseSRT unexpected: %+v", srtSegs)
	}
	if srtSegs[2].Text != "Line one Line two" || srtSegs[2].Translation != "" {
		t.Fatalf("monolingual multiline cue should join cleanly: %+v", srtSegs[2])
	}

	biSRT := "1\n00:00:01,000 --> 00:00:03,000\nHello world.\n你好世界。\n\n2\n00:00:04,000 --> 00:00:06,000\nGood morning.\nおはようございます。"
	biSegs := ParseSRT(biSRT)
	if len(biSegs) != 2 || biSegs[0].Text != "Hello world." || biSegs[0].Translation != "你好世界。" || biSegs[1].Translation != "おはようございます。" {
		t.Fatalf("bilingual SRT parse unexpected: %+v", biSegs)
	}

	jsonTr := `{"segments":[{"startTime":1.5,"endTime":4.0,"body":"Welcome"}]}`
	jSegs, err := ParsePodcastJSONTranscript([]byte(jsonTr))
	if err != nil || len(jSegs) != 1 || jSegs[0].Text != "Welcome" {
		t.Fatalf("ParsePodcastJSONTranscript unexpected: %+v err=%v", jSegs, err)
	}

	segs[0].Translation = "你好世界。"
	segs[1].Translation = "第二行。"
	ep := Episode{Title: "Demo Ep", FeedTitle: "Demo Show", LinkURL: "https://example.com/ep"}
	for _, tc := range []struct {
		fmt  string
		mode string
		want string
	}{
		{"srt", "bilingual", "Hello world.\n你好世界。"},
		{"vtt", "original", "WEBVTT"},
		{"md", "translation", "**[00:01]** 你好世界。"},
		{"txt", "bilingual", "Second line.\n第二行。"},
	} {
		out := ExportTranscript(ep, segs, tc.fmt, tc.mode)
		if !strings.Contains(out, tc.want) {
			t.Errorf("ExportTranscript(%s, %s) missing %q:\n%s", tc.fmt, tc.mode, tc.want, out)
		}
	}
}

func TestResolversAndOPML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rss.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<rss version="2.0"><channel><title>Resolved Show</title><item><title>Ep A</title><guid>a</guid><enclosure url="https://cdn.example.com/a.mp3" type="audio/mpeg"/></item></channel></rss>`)
	})
	var srvURL string
	mux.HandleFunc("/lookup", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"results":[{"feedUrl":"`+srvURL+`/rss.xml","artworkUrl600":"https://cdn.example.com/art.jpg"}]}`)
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"results":[{"collectionId":123,"collectionName":"Go Cast","artistName":"Gopher","feedUrl":"`+srvURL+`/rss.xml","primaryGenreName":"Technology"}]}`)
	})
	mux.HandleFunc("/site.html", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<!doctype html><html><head><link rel="alternate" type="application/rss+xml" href="/rss.xml"></head><body>hi</body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	// 1. iTunes search
	res, err := SearchITunes(context.Background(), srv.Client(), srv.URL, "Go", "auto")
	if err != nil || len(res) != 1 || res[0].Title != "Go Cast" {
		t.Fatalf("SearchITunes failed: res=%+v err=%v", res, err)
	}

	// 2. HTML <link rel="alternate">
	f, eps, err := ResolveAndFetchInput(context.Background(), srv.Client(), srv.URL, srv.URL+"/site.html")
	if err != nil || f.Title != "Resolved Show" || len(eps) != 1 {
		t.Fatalf("HTML feed discovery failed: f=%+v eps=%+v err=%v", f, eps, err)
	}

	// 3. Xiaoyuzhou __NEXT_DATA__ HTML parser
	xyzHTML := `<html><head><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"podcast":{"pid":"abc123","title":"小宇宙测试","author":"主播","episodes":[{"eid":"ep99","title":"第1期","shownotes":"00:00 开场\n05:00 正题","duration":600,"enclosure":{"url":"https://media.xyzcdn.net/test.m4a"}}]}}}}</script></head></html>`
	xf, xeps, err := parseXiaoyuzhouHTML([]byte(xyzHTML), "https://www.xiaoyuzhoufm.com/podcast/abc123")
	if err != nil || xf.Title != "小宇宙测试" || len(xeps) != 1 || xeps[0].AudioURL != "https://media.xyzcdn.net/test.m4a" {
		t.Fatalf("parseXiaoyuzhouHTML failed: f=%+v eps=%+v err=%v", xf, xeps, err)
	}

	// 4. OPML export + parse
	opml := ExportOPML([]Feed{f})
	entries, err := ParseOPML([]byte(opml))
	if err != nil || len(entries) != 1 || entries[0].FeedURL != srv.URL+"/rss.xml" {
		t.Fatalf("OPML round-trip failed: entries=%+v err=%v", entries, err)
	}
}

func TestMediaServerAuthRangeAndCovers(t *testing.T) {
	audioPayload := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	mux := http.NewServeMux()
	mux.HandleFunc("/audio.mp3", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "audio.mp3", time.Time{}, strings.NewReader(string(audioPayload)))
	})
	mux.HandleFunc("/cover.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	})
	up := httptest.NewServer(mux)
	defer up.Close()

	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := Feed{ID: "fd-1", Title: "Show", FeedURL: up.URL + "/feed.xml"}
	ep := Episode{ID: "ep-1", FeedID: "fd-1", Title: "Ep 1", AudioURL: up.URL + "/audio.mp3"}
	if _, _, err := st.UpsertFeedAndEpisodes(f, []Episode{ep}); err != nil {
		t.Fatal(err)
	}

	ms, err := NewMediaServer(st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ms.Close()

	// Wrong token rejected
	resp, err := http.Get(strings.Replace(ms.PlayURL("ep-1"), ms.token, "wrong", 1))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong token, got %d", resp.StatusCode)
	}

	// Unknown episode rejected (SSRF protection)
	resp, err = http.Get(ms.PlayURL("ep-unknown"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown episode, got %d", resp.StatusCode)
	}

	// Range request forwarded cleanly
	req, _ := http.NewRequest(http.MethodGet, ms.PlayURL("ep-1"), nil)
	req.Header.Set("Range", "bytes=10-15")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent || string(b) != "ABCDEF" {
		t.Fatalf("Range request failed: status=%d body=%q", resp.StatusCode, string(b))
	}

	// Cover allowlist & caching
	covURL := ms.RegisterCover(up.URL + "/cover.png")
	cResp, err := http.Get(covURL)
	if err != nil {
		t.Fatal(err)
	}
	cBytes, _ := io.ReadAll(cResp.Body)
	cResp.Body.Close()
	if cResp.StatusCode != http.StatusOK || len(cBytes) == 0 {
		t.Fatalf("cover proxy failed: status=%d", cResp.StatusCode)
	}
}

func TestMediaServerContentTypeAndRangeProbeFallback(t *testing.T) {
	payload := []byte("ID3-FAKE-AUDIO-STREAM-PAYLOAD-0123456789")
	mux := http.NewServeMux()
	// Upstream ignores Range header (returns 200 OK) and sends generic application/octet-stream.
	mux.HandleFunc("/episode.m4a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", "40")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(payload)
		}
	})
	up := httptest.NewServer(mux)
	defer up.Close()

	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := Feed{ID: "fd-m", Title: "Show", FeedURL: up.URL + "/feed.xml"}
	ep := Episode{ID: "ep-m4a", FeedID: "fd-m", Title: "M4A Ep", AudioURL: up.URL + "/episode.m4a"}
	if _, _, err := st.UpsertFeedAndEpisodes(f, []Episode{ep}); err != nil {
		t.Fatal(err)
	}

	ms, err := NewMediaServer(st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ms.Close()

	// OPTIONS CORS preflight
	optReq, _ := http.NewRequest(http.MethodOptions, ms.PlayURL("ep-m4a"), nil)
	optResp, err := http.DefaultClient.Do(optReq)
	if err != nil {
		t.Fatal(err)
	}
	optResp.Body.Close()
	if optResp.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS status=%d, want 204", optResp.StatusCode)
	}

	// AVFoundation initial bytes=0-1 probe gets synthesized 206 + audio/mp4 even when upstream returns 200 + octet-stream.
	req, _ := http.NewRequest(http.MethodGet, ms.PlayURL("ep-m4a"), nil)
	req.Header.Set("Range", "bytes=0-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("probe status=%d, want 206", resp.StatusCode)
	}
	if string(b) != "ID" {
		t.Fatalf("probe body=%q, want %q", string(b), "ID")
	}
	if gotCT := resp.Header.Get("Content-Type"); gotCT != "audio/mp4" {
		t.Fatalf("Content-Type=%q, want audio/mp4", gotCT)
	}
	if gotCR := resp.Header.Get("Content-Range"); gotCR != "bytes 0-1/40" {
		t.Fatalf("Content-Range=%q, want bytes 0-1/40", gotCR)
	}

	// Upstream 206 omitting Content-Length (as Cloudflare/Beehiiv often do) gets Content-Length derived from Content-Range.
	mux.HandleFunc("/partial-no-cl.mp3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Range", "bytes 0-9/40")
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload[:10])
	})
	epNoCL := Episode{ID: "ep-nocl", FeedID: "fd-m", Title: "No CL Ep", AudioURL: up.URL + "/partial-no-cl.mp3"}
	if _, _, err := st.UpsertFeedAndEpisodes(f, []Episode{epNoCL}); err != nil {
		t.Fatal(err)
	}
	req2, _ := http.NewRequest(http.MethodGet, ms.PlayURL("ep-nocl"), nil)
	req2.Header.Set("Range", "bytes=0-9")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusPartialContent || len(b2) != 10 {
		t.Fatalf("partial-no-cl failed: status=%d len=%d", resp2.StatusCode, len(b2))
	}
	if gotCL := resp2.Header.Get("Content-Length"); gotCL != "10" {
		t.Fatalf("expected derived Content-Length=10, got %q", gotCL)
	}
}

func TestAIAndTranslationMocks(t *testing.T) {
	var rateLimitMode atomic.Bool
	var translateHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "reading guide editor") {
			_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"oneLiner\":\"Core summary\",\"takeaways\":[\"Point 1\"],\"timeline\":[{\"time\":\"01:30\",\"title\":\"Topic\",\"summary\":\"Details\"}],\"quotes\":[{\"time\":\"02:00\",\"text\":\"Quote text\",\"note\":\"Important\"}]}"}}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Answer referencing [01:30]."}}]}`)
	})
	mux.HandleFunc("/translate_a/single", func(w http.ResponseWriter, r *http.Request) {
		translateHits.Add(1)
		if rateLimitMode.Load() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		q := r.URL.Query().Get("q")
		if strings.Contains(q, googleBatchMarker) {
			parts := strings.Split(q, googleBatchMarker)
			tr := make([]string, len(parts))
			for i := range parts {
				tr[i] = "译文" + strings.TrimSpace(parts[i])
			}
			out, _ := json.Marshal([][][]any{{{strings.Join(tr, "\n"+googleBatchMarker+"\n"), q}}})
			_, _ = w.Write(out)
			return
		}
		_, _ = w.Write([]byte(`[[["测试释义","word"]]]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := AIConfig{
		Provider: "custom",
		BaseURL:  srv.URL + "/v1",
		Model:    "test-model",
		APIKey:   "sk-test",
	}
	ep := Episode{Title: "AI Talk"}
	segs := []TranscriptSegment{
		{ID: 1, StartSec: 90, EndSec: 100, Text: "Hello"},
		{ID: 2, StartSec: 120, EndSec: 130, Text: "World"},
	}

	guide, err := GenerateGuide(context.Background(), srv.Client(), cfg, ep, segs, "zh-Hans")
	if err != nil || guide.OneLiner != "Core summary" || len(guide.Timeline) != 1 || guide.Timeline[0].TimeSec != 90 {
		t.Fatalf("GenerateGuide failed: guide=%+v err=%v", guide, err)
	}

	ans, err := AskEpisode(context.Background(), srv.Client(), cfg, ep, segs, nil, "What was discussed?", "en")
	if err != nil || !strings.Contains(ans, "[01:30]") {
		t.Fatalf("AskEpisode failed: ans=%q err=%v", ans, err)
	}

	// Incremental progressive translation skips already-translated segments when force=false.
	partialSegs := []TranscriptSegment{
		{ID: 1, StartSec: 90, EndSec: 100, Text: "Hello", Translation: "已有译文"},
		{ID: 2, StartSec: 120, EndSec: 130, Text: "World"},
	}
	var progSnapshots int
	trSegs, err := TranslateSegmentsProgressive(context.Background(), srv.Client(), cfg, srv.URL, partialSegs, "zh-CN", false, func(done, total int, partial []TranscriptSegment) {
		progSnapshots++
	})
	if err != nil || len(trSegs) != 2 || trSegs[0].Translation != "已有译文" || !strings.Contains(trSegs[1].Translation, "测试释义") || progSnapshots == 0 {
		t.Fatalf("TranslateSegmentsProgressive failed: segs=%+v err=%v snaps=%d", trSegs, err, progSnapshots)
	}

	// Full batch translation
	trFull, err := TranslateSegments(context.Background(), srv.Client(), cfg, srv.URL, segs, "zh-CN", nil)
	if err != nil || len(trFull) != 2 || !strings.Contains(trFull[0].Translation, "译文") {
		t.Fatalf("TranslateSegments failed: segs=%+v err=%v", trFull, err)
	}

	// HTTP 429 must abort batch immediately without retrying per-segment.
	translateHits.Store(0)
	rateLimitMode.Store(true)
	_, rlErr := TranslateSegments(context.Background(), srv.Client(), cfg, srv.URL, segs, "zh-CN", nil)
	if rlErr == nil || !strings.Contains(rlErr.Error(), "429") || translateHits.Load() != 1 {
		t.Fatalf("expected single-hit 429 abort, got err=%v hits=%d", rlErr, translateHits.Load())
	}
}

func TestPluginHarnessActionRPCAndAutoPause(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<rss version="2.0" xmlns:podcast="https://podcastindex.org/namespace/1.0"><channel><title>Harness Cast</title><item><title>Pilot</title><guid>pilot-1</guid><enclosure url="`+r.Host+`" type="audio/mpeg"/><podcast:transcript url="http://`+r.Host+`/sub.vtt" type="text/vtt"/></item></channel></rss>`)
	})
	mux.HandleFunc("/sub.vtt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "WEBVTT\n\n00:00:00.000 --> 00:00:03.000\nWelcome to Hyla Podcast.")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Fix enclosure URL cleanly using srv.URL
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<rss version="2.0" xmlns:podcast="https://podcastindex.org/namespace/1.0"><channel><title>Harness Cast</title><item><title>Pilot</title><guid>pilot-1</guid><enclosure url="`+srv.URL+`/audio.mp3" type="audio/mpeg"/><podcast:transcript url="`+srv.URL+`/sub.vtt" type="text/vtt"/></item></channel></rss>`)
	})
	srv2 := httptest.NewServer(mux2)
	defer srv2.Close()

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
			Settings: map[string]any{"auto_pause_on_agent": true},
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
		Settings:    map[string]any{"auto_pause_on_agent": true},
	}, &initRes); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if initRes.Name != "Podcast" {
		t.Fatalf("unexpected initRes: %+v", initRes)
	}

	// Open pane action
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
	if len(opened) != 1 || opened[0].Plugin != pluginID || opened[0].View != slotID || opened[0].Title != "播客" || !opened[0].FocusExisting {
		t.Fatalf("unexpected opened pane: %+v", opened)
	}
	mu.Unlock()

	// Subscribe + episodeDetail + saveProgress + vocabAdd via webview.rpc
	subParams, _ := json.Marshal(map[string]string{"input": srv2.URL + "/feed.xml"})
	var rpcRes hylaplugin.WebViewRPCResult
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID: slotID,
		Method: "podcast.subscribe",
		Params: subParams,
	}, &rpcRes); err != nil {
		t.Fatalf("podcast.subscribe: %v", err)
	}
	var subOut struct {
		Feed     Feed      `json:"feed"`
		Episodes []Episode `json:"episodes"`
	}
	if err := json.Unmarshal(rpcRes.Data, &subOut); err != nil || len(subOut.Episodes) != 1 {
		t.Fatalf("decode subscribe out: %s err=%v", string(rpcRes.Data), err)
	}
	epID := subOut.Episodes[0].ID

	detParams, _ := json.Marshal(map[string]string{"episodeId": epID})
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID: slotID,
		Method: "podcast.episodeDetail",
		Params: detParams,
	}, &rpcRes); err != nil {
		t.Fatalf("podcast.episodeDetail: %v", err)
	}
	var detOut struct {
		Detail EpisodeDetail `json:"detail"`
	}
	if err := json.Unmarshal(rpcRes.Data, &detOut); err != nil || len(detOut.Detail.Segments) != 1 {
		t.Fatalf("expected auto-fetched VTT segment, got %s err=%v", string(rpcRes.Data), err)
	}

	progParams, _ := json.Marshal(map[string]any{"episodeId": epID, "positionSec": 42, "playedDeltaSec": 12})
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID: slotID,
		Method: "podcast.saveProgress",
		Params: progParams,
	}, &rpcRes); err != nil {
		t.Fatalf("podcast.saveProgress: %v", err)
	}

	vocabParams, _ := json.Marshal(VocabEntry{Word: "ephemeral", Meaning: "短暂的", EpisodeID: epID, TimestampSec: 42})
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID: slotID,
		Method: "podcast.vocabAdd",
		Params: vocabParams,
	}, &rpcRes); err != nil {
		t.Fatalf("podcast.vocabAdd: %v", err)
	}

	// Verify data persisted in state.json
	if _, err := os.Stat(filepath.Join(dataDir, "state.json")); err != nil {
		t.Fatalf("expected state.json persisted: %v", err)
	}

	// Coding agent working -> idle triggers podcast.autoPause
	if err := h.Call(ctx, "event.notify", hylaplugin.EventNotifyParams{
		Topic: "pane.status",
		Data:  json.RawMessage(`{"sessionId":"s1","paneId":"p1","agent":"claude","status":"working"}`),
	}, nil); err != nil {
		t.Fatalf("notify working: %v", err)
	}
	if err := h.Call(ctx, "event.notify", hylaplugin.EventNotifyParams{
		Topic: "pane.status",
		Data:  json.RawMessage(`{"sessionId":"s1","paneId":"p1","agent":"claude","status":"idle"}`),
	}, nil); err != nil {
		t.Fatalf("notify idle: %v", err)
	}

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
	if len(emitted) != 1 || emitted[0].Event != "podcast.autoPause" {
		t.Fatalf("expected podcast.autoPause emit, got %+v", emitted)
	}
}
