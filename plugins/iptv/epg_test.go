package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The guides write the number of a channel in more than one way, and some
// put the topic after it. The playlist writes it its own way.
func TestEPGMatcher(t *testing.T) {
	m := newEPGMatcher([]Channel{
		{Name: "CCTV-1 综合"},
		{Name: "CCTV-13 新闻"},
		{Name: "CCTV5 体育"},
		{Name: "CCTV5+ 体育赛事"},
		{Name: "湖南卫视"},
		{Name: "凤凰卫视中文台"},
	})
	cases := map[string]string{
		"CCTV1":       "cctv-1 综合",
		"CCTV-1":      "cctv-1 综合",
		"CCTV1综合":     "cctv-1 综合",
		"CCTV1高清":     "cctv-1 综合",
		"CCTV13":      "cctv-13 新闻",
		"CCTV5":       "", // "CCTV5+" shares the number: the guide never leaves out a plus
		"CCTV5体育":     "cctv5 体育",
		"CCTV5+":      "cctv5+ 体育赛事",
		"CCTV5+-体育赛事": "cctv5+ 体育赛事",
		"CCTV5PLUS":   "cctv5+ 体育赛事",
		"CCTV10":      "",
		"cctv-1 综合":   "cctv-1 综合",
		"湖南卫视":        "湖南卫视",
		"凤凰卫视":        "",
		"":            "",
	}
	for id, want := range cases {
		if got := m.channelFor(id); got != want {
			t.Errorf("channelFor(%q) = %q, want %q", id, got, want)
		}
	}

	// A tvg-id names its channel even when the guide writes it alone.
	m2 := newEPGMatcher([]Channel{{Name: "CCTV-1 综合", TVGID: "CCTV1"}})
	for _, id := range []string{"CCTV1", "cctv1", "CCTV-1"} {
		if got := m2.channelFor(id); got != "cctv-1 综合" {
			t.Errorf("tvg-id match channelFor(%q) = %q", id, got)
		}
	}
	if got := m2.channelFor("CCTV10"); got != "" {
		t.Errorf("channelFor(CCTV10) = %q, want no match", got)
	}
}

func TestParseXMLTVWindowAndFields(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, loc)
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="CCTV1"><display-name>CCTV1</display-name></channel>
  <programme channel="CCTV1" start="20261121120000 +0800" stop="20261121130000 +0800">
    <title lang="zh">新闻联播</title><desc lang="zh">daily news</desc><category>新闻</category>
  </programme>
  <programme channel="CCTV1" start="20261120100000 +0800" stop="20261120110000 +0800"><title>old</title></programme>
  <programme channel="CCTV1" start="20261225090000 +0800" stop="20261225093000 +0800"><title>far</title></programme>
  <programme channel="CCTV1" start="20261121130000 +0800" stop="20261121140000 +0800">
    <title>电视剧</title><sub-title>第 4 集</sub-title>
  </programme>
  <programme channel="Unknown TV" start="20261121120000 +0800" stop="20261121130000 +0800"><title>x</title></programme>
  <programme channel="CCTV1" start="broken"><title>bad time</title></programme>
</tv>`
	raw, err := readXMLTV(strings.NewReader(xml), now, loc)
	if err != nil {
		t.Fatalf("readXMLTV: %v", err)
	}
	// Every programme in the window stays, with its channel: the match
	// picks the channels after the read.
	if len(raw.programs) != 3 {
		t.Fatalf("expected 3 programmes in the window, got %d: %+v", len(raw.programs), raw.programs)
	}
	guide, _ := raw.match([]Channel{{Name: "CCTV-1 综合"}}, loc)
	list := guide.ByChannel["cctv-1 综合"]
	if len(list) != 2 {
		t.Fatalf("expected 2 matched programmes, got %+v", guide.ByChannel)
	}
	first := list[0]
	if first.Title != "新闻联播" || first.Desc != "daily news" || first.Category != "新闻" || first.ChannelID != "CCTV1" {
		t.Fatalf("unexpected programme: %+v", first)
	}
	if first.Start != now.UnixMilli() || first.Stop != now.Add(time.Hour).UnixMilli() {
		t.Fatalf("unexpected times: %+v", first)
	}
	if list[1].Title != "电视剧 · 第 4 集" {
		t.Fatalf("sub-title not joined: %q", list[1].Title)
	}
	if guide.Programs != 2 {
		t.Fatalf("guide.Programs = %d", guide.Programs)
	}
}

func TestParseXMLTVGBK(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, loc)
	// <title>新闻</title> in GB18030.
	gbk := []byte{0xd0, 0xc2, 0xce, 0xc5}
	head := `<?xml version="1.0" encoding="GB2312"?><tv><programme channel="CCTV1" start="20261121120000 +0800" stop="20261121130000 +0800"><title>`
	tail := `</title></programme></tv>`
	xml := append(append([]byte(head), gbk...), []byte(tail)...)

	raw, err := readXMLTV(strings.NewReader(string(xml)), now, loc)
	if err != nil {
		t.Fatalf("readXMLTV GBK: %v", err)
	}
	guide, _ := raw.match([]Channel{{Name: "CCTV1"}}, loc)
	list := guide.ByChannel["cctv1"]
	if len(list) != 1 || list[0].Title != "新闻" {
		t.Fatalf("GBK title not decoded: %+v", guide.ByChannel)
	}
}

func TestEPGURLsOfSources(t *testing.T) {
	urls := epgURLsOf([]Source{
		{EPGURL: "https://a.example.com/epg.xml, https://b.example.com/epg.xml"},
		{EPGURL: "https://a.example.com/epg.xml|ftp://c.example.com/epg.xml"},
		{EPGURL: ""},
	})
	if len(urls) != 2 || urls[0] != "https://a.example.com/epg.xml" || urls[1] != "https://b.example.com/epg.xml" {
		t.Fatalf("unexpected guide urls: %v", urls)
	}
}

func TestEPGServiceRefreshAndMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/epg.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><tv>`+
			`<programme channel="CCTV1" start="`+epgTime(time.Now())+`" stop="`+epgTime(time.Now().Add(time.Hour))+`"><title>News</title></programme>`+
			`</tv>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := newEPGService(context.Background(), store, newNetClients(), defaultNetConfig)
	defer svc.Close()

	chs := []Channel{{Name: "CCTV-1 综合"}}
	if _, updated := svc.programGuide(); updated != 0 {
		t.Fatalf("guide before a fetch")
	}
	svc.refresh(func() []Channel { return chs }, []string{srv.URL + "/epg.xml"})
	deadline := time.Now().Add(5 * time.Second)
	for svc.status()["state"] != "ready" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	guide, updated := svc.programGuide()
	if updated == 0 || len(guide["cctv-1 综合"]) != 1 || guide["cctv-1 综合"][0].Title != "News" {
		t.Fatalf("unexpected guide after fetch: %+v (status %v)", guide, svc.status())
	}

	// Another store reads the guide from the cache file.
	store2, err := NewStore(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	svc2 := newEPGService(context.Background(), store2, newNetClients(), defaultNetConfig)
	defer svc2.Close()
	if g, _ := svc2.programGuide(); len(g["cctv-1 综合"]) != 1 {
		t.Fatalf("guide cache not read back: %+v", g)
	}
}

func TestEPGRematchAfterChannelChange(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/epg.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><tv>`+
			`<programme channel="CCTV1" start="`+epgTime(time.Now())+`" stop="`+epgTime(time.Now().Add(time.Hour))+`"><title>News</title></programme>`+
			`<programme channel="CCTV2" start="`+epgTime(time.Now())+`" stop="`+epgTime(time.Now().Add(time.Hour))+`"><title>Money</title></programme>`+
			`</tv>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := newEPGService(context.Background(), store, newNetClients(), defaultNetConfig)
	defer svc.Close()

	chs := []Channel{{ID: "a", Name: "CCTV-1 综合"}}
	svc.rematchIfStale(chs)
	svc.refresh(func() []Channel { return chs }, []string{srv.URL + "/epg.xml"})
	deadline := time.Now().Add(5 * time.Second)
	for svc.status()["state"] != "ready" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	// A new source adds CCTV-2. The pane matches the guide again without
	// a second fetch of it.
	chs = append(chs, Channel{ID: "b", Name: "CCTV-2", NormName: "cctv-2"})
	svc.rematchIfStale(chs)
	guide, _ := svc.programGuide()
	if len(guide["cctv-1 综合"]) != 1 || len(guide["cctv-2"]) != 1 || guide["cctv-2"][0].Title != "Money" {
		t.Fatalf("new channel has no programme: %+v", guide)
	}

	// The channel list unchanged keeps the match it has.
	before := svc.status()["programs"]
	svc.rematchIfStale(chs)
	if after := svc.status()["programs"]; after != before {
		t.Fatalf("a second match with the same channels changed the guide: %v -> %v", before, after)
	}

	// A guide from the cache file, with no programmes as read, still
	// keeps the programmes of a channel that stays.
	store2, err := NewStore(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	svc2 := newEPGService(context.Background(), store2, newNetClients(), defaultNetConfig)
	defer svc2.Close()
	svc2.rematchIfStale(chs[:1])
	if g, _ := svc2.programGuide(); len(g["cctv-1 综合"]) != 1 {
		t.Fatalf("cached channel lost its programmes: %+v", g)
	}
}

func epgTime(at time.Time) string {
	return at.UTC().Format("20060102150405 -0700")
}
