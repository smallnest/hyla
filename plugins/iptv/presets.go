package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// The presets come from the repository's iptv.md, so that a change to a
// source reaches every pane without a new plugin. The plugin reads the
// file at startup and keeps the last good read on disk.

const (
	presetsURL        = "https://raw.githubusercontent.com/smallnest/hyla/master/iptv.md"
	presetsFileName   = "presets.md"
	presetsMaxBytes   = 1 << 20
	presetsCacheLife  = 6 * time.Hour
	presetsWaitOnOpen = 20 * time.Second
)

// fallbackPresets is what the pane shows when no network and no saved
// copy gives a source.
func fallbackPresets() []PresetSource {
	return []PresetSource{{
		ID:      "best-fan-cn",
		Name:    "Domestic Live TV (best-fan/iptv-sources)",
		NameZh:  "国内直播源 (best-fan/iptv-sources)",
		NameJa:  "中国テレビライブ (best-fan/iptv-sources)",
		Desc:    "CCTV, satellite channels and local stations, with logos and a measured response time",
		DescZh:  "央视、卫视与地方频道，带台标与实测响应时间",
		DescJa:  "CCTV・衛星放送・地方局。ロゴと実測応答時間付き",
		URL:     "https://raw.githubusercontent.com/best-fan/iptv-sources/master/cn_all_status.m3u8",
		Country: "CN",
	}}
}

// parsePresetsMarkdown reads the presets in a table of iptv.md. The row's
// first cell is the ID, the second its name and a description, the row
// its URL. A row with no http URL, no ID, or an ID used before is skipped.
func parsePresetsMarkdown(text string) ([]PresetSource, error) {
	var out []PresetSource
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) < 2 {
			continue
		}
		id := cells[0]
		if id == "" || seen[id] || strings.HasPrefix(id, ":") || isMarkdownRule(id) {
			continue
		}
		u := firstHTTPURL(strings.Join(cells[1:], " "))
		if u == "" {
			continue
		}
		if _, err := validateHTTPURL(u); err != nil {
			continue
		}
		name, _ := splitPresetName(cells[1])
		desc := ""
		if len(cells) > 3 {
			desc = cells[len(cells)-1]
		}
		if name == "" {
			name = id
		}
		seen[id] = true
		out = append(out, PresetSource{
			ID:      id,
			Name:    name,
			NameZh:  name,
			NameJa:  name,
			Desc:    desc,
			DescZh:  desc,
			DescJa:  desc,
			URL:     u,
			Country: countryOf(id),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no preset table row in %s", presetsURL)
	}
	return out, nil
}

func isMarkdownRule(s string) bool {
	return strings.Trim(s, ":- ") == ""
}

// firstHTTPURL returns the first http or https URL on a line, and the
// URL alone when it is a Markdown link, [name](https://...).
func firstHTTPURL(s string) string {
	if m := presetsLinkRe.FindStringSubmatch(s); len(m) == 2 {
		return m[1]
	}
	return presetsPlainRe.FindString(s)
}

var (
	presetsLinkRe  = regexp.MustCompile(`\]\(\s*(https?://[^\s)]+)\s*\)`)
	presetsPlainRe = regexp.MustCompile(`https?://[^\s)<>"']+`)
	presetsWrapRe  = regexp.MustCompile(`[\s　]*[（(\[][^）)\]]*[）)\]]\s*$`)
)

// splitPresetName takes the spoken name of a source off its English name,
// as "国内直播源 (best-fan/iptv-sources)" gives "国内直播源".
func splitPresetName(cell string) (name, desc string) {
	cell = strings.TrimSpace(presetsLinkRe.ReplaceAllString(cell, ""))
	cell = strings.TrimSpace(presetsPlainRe.ReplaceAllString(cell, ""))
	return presetsWrapRe.ReplaceAllString(cell, ""), ""
}

func countryOf(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "eng"), strings.Contains(lower, "intl"):
		return "INTL"
	case strings.Contains(lower, "zho"):
		return "ZH"
	default:
		return "CN"
	}
}

// presetList holds the presets, with the last good read saved on disk.
type presetList struct {
	mu        sync.Mutex
	dir       string
	offline   bool // the tests only give the file
	netc      *netClients
	cfgFn     func() NetConfig
	items     []PresetSource
	updatedAt time.Time
	inflight  bool
	closed    chan struct{} // closed after the first read ends
}

func newPresetList(dir string, netc *netClients, cfgFn func() NetConfig) *presetList {
	pl := &presetList{dir: dir, netc: netc, cfgFn: cfgFn, closed: make(chan struct{})}
	if data, err := os.ReadFile(pl.path()); err == nil {
		if items, perr := parsePresetsMarkdown(string(data)); perr == nil {
			pl.items = items
		}
	}
	return pl
}

func (pl *presetList) path() string { return pl.dir + string(os.PathSeparator) + presetsFileName }

// list returns the presets and starts a read when the copy is old.
func (pl *presetList) list() []PresetSource {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	items := pl.items
	if !pl.offline && (len(items) == 0 || time.Since(pl.updatedAt) > presetsCacheLife) {
		go pl.refresh()
	}
	if len(items) == 0 {
		items = fallbackPresets()
	}
	return append([]PresetSource(nil), items...)
}

// start reads the file in the background. The pane calls bootstrap once
// more when the read ends and adds a source.
func (pl *presetList) start() {
	if pl.offline {
		pl.mu.Lock()
		pl.closeFirst()
		pl.mu.Unlock()
		return
	}
	go pl.refresh()
}

// waitForFirst returns the presets after the first read, or once the
// wait is over.
func (pl *presetList) waitForFirst(ctx context.Context, wait time.Duration) []PresetSource {
	if pl.offline {
		return pl.list()
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-pl.closed:
	case <-t.C:
	case <-ctx.Done():
	}
	return pl.list()
}

// closeFirst marks the first read as over. The caller holds mu.
func (pl *presetList) closeFirst() {
	select {
	case <-pl.closed:
	default:
		close(pl.closed)
	}
}

func (pl *presetList) refresh() {
	if pl.offline {
		return
	}
	pl.mu.Lock()
	if pl.inflight {
		pl.mu.Unlock()
		return
	}
	pl.inflight = true
	pl.mu.Unlock()
	defer func() {
		pl.mu.Lock()
		if !pl.inflight {
			pl.mu.Unlock()
			return
		}
		pl.inflight = false
		pl.closeFirst()
		pl.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	resp, err := pl.netc.doSmart(ctx, http.MethodGet, presetsURL, requestHeaders{Accept: "text/markdown,text/plain,*/*"}, pl.cfgFn(), false, false)
	if err != nil {
		log.Printf("iptv presets %s: %v", presetsURL, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("iptv presets %s: HTTP %s", presetsURL, resp.Status)
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, presetsMaxBytes))
	if err != nil {
		log.Printf("iptv presets %s: %v", presetsURL, err)
		return
	}
	items, err := parsePresetsMarkdown(string(data))
	if err != nil {
		log.Printf("iptv presets %s: %v", presetsURL, err)
		return
	}
	pl.mu.Lock()
	pl.items = items
	pl.updatedAt = time.Now()
	pl.mu.Unlock()
	_ = os.WriteFile(pl.path(), data, 0o644)
}
