package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// The EPG keeps the programmes of the channels that the user watches, not
// the whole guide: an XMLTV guide for a full Chinese playlist weighs about
// three megabytes and lists more than ten thousand programmes that nobody
// opens.

const epgWindow = 3 * 24 * time.Hour

// epgMatcher tells which channel of the pane an XMLTV channel id serves.
// The guides write a channel as "CCTV1", "CCTV-1", "CCTV1综合", "CCTV5PLUS"
// or "CCTV5+"; the playlist writes "CCTV-1 综合", "CCTV-1 高清", or gives
// "CCTV1" in tvg-id.
//
// A name becomes spaced words and numbers: "CCTV-1 综合" is "cctv 1 综合".
// A shorter form names its channel only when no channel holds more words
// after it: "cctv 1" names "CCTV-1 综合", which holds "综合", and it names
// nothing else.
type epgMatcher struct {
	exact map[string]string   // spaced name to the channel name
	heads map[string][]string // first two words, as "cctv 1", to the channels that hold them
	plain map[string]string   // first two words to the channel that ends there, as "cctv 1"
	plus  map[string]bool     // first two words of a channel with a plus, as "cctv 5"
	// rev names the channels of the match, as channelsRevOf does.
	rev string
}

// epgWords splits a name into its words and numbers: "CCTV-1 综合" becomes
// "cctv" "1" "综合", and "CCTV5PLUS" becomes "cctv" "5" "plus".
func epgWords(n string) []string {
	var words []string
	for _, word := range strings.Fields(strings.ReplaceAll(n, "+", " + ")) {
		for {
			i := strings.IndexFunc(word, func(r rune) bool { return r >= '0' && r <= '9' })
			if i < 0 {
				if word = strings.Trim(word, "-_."); word != "" {
					words = append(words, word)
				}
				break
			}
			j := i
			for j < len(word) && word[j] >= '0' && word[j] <= '9' {
				j++
			}
			if head := strings.TrimRight(word[:i], "-_."); head != "" {
				words = append(words, head)
			}
			words = append(words, word[i:j])
			word = strings.TrimLeft(word[j:], "-_.")
		}
	}
	return words
}

// epgSpaced joins the words of a name, as "cctv 1 综合". A plus becomes the
// word "plus", so that "CCTV5+" and "CCTV5PLUS" share the words.
func epgSpaced(n string) string {
	return strings.Join(epgWords(n), " ")
}

// epgHead makes the first two words, as "cctv 1": the head and the number.
func epgHead(n string) string {
	words := epgWords(n)
	if len(words) < 2 {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:2], " ")
}

func newEPGMatcher(channels []Channel) *epgMatcher {
	m := &epgMatcher{exact: map[string]string{}, heads: map[string][]string{}, plain: map[string]string{}, plus: map[string]bool{}}
	for _, ch := range channels {
		name, tvg := normalizeChannelName(ch.Name), normalizeChannelName(ch.TVGID)
		if name == "" {
			continue
		}
		for _, n := range []string{name, tvg} {
			if n == "" {
				continue
			}
			for _, key := range epgIDForms(n) {
				if _, ok := m.exact[key]; !ok {
					m.exact[key] = name
				}
			}
		}
		head, words := epgHead(name), len(epgWords(normalizeChannelName(ch.Name)))
		m.heads[head] = append(m.heads[head], name)
		if words == 2 {
			m.plain[head] = name
		}
		if strings.Contains(name, "+") {
			m.plus[head] = true
		}
	}
	return m
}

// epgIDForms makes the ids that a guide can write for a channel of the
// playlist. It leaves out the head and the number alone: a guide writes
// "CCTV1" for "CCTV-1 综合" only when no channel holds the number, and
// channelFor tells that case.
func epgIDForms(n string) []string {
	words := epgWords(n)
	out := []string{strings.Join(words, " ")}
	if len(words) < 2 {
		return out
	}
	// "CCTV1综合" and "cctv1 综合" for "CCTV-1 综合".
	out = append(out, words[0]+words[1]+" "+strings.Join(words[2:], " "))
	if len(words) == 2 || (len(words) > 2 && words[2] != "+" && words[2] != "plus") {
		out = append(out, words[0]+words[1])
	}
	// "CCTV5PLUS" and "cctv 5 plus" for "CCTV5+". The head and number
	// in one word holds the plus too: "cctv5+" is "CCTV5+".
	if i := strings.Index(n, "+"); i >= 0 {
		out = append(out, words[0]+words[1]+"+", words[0]+words[1]+" plus")
		if len(words) > 3 {
			out = append(out, words[0]+words[1]+" plus "+strings.Join(words[3:], " "))
		}
	}
	return out
}

// channelFor returns the name of the pane channel that an XMLTV id serves.
// The empty name means that no channel takes it.
func (m *epgMatcher) channelFor(xmltvID string) string {
	raw := normalizeChannelName(xmltvID)
	spaced := epgSpaced(raw)
	if spaced == "" {
		return ""
	}
	if name, ok := m.exact[spaced]; ok {
		return name
	}
	// "CCTV-1" and "CCTV5+": the signs of a name say which channel it is.
	if name, ok := m.exact[strings.ReplaceAll(strings.ReplaceAll(raw, " ", ""), "-", "")]; ok && strings.ContainsAny(raw, "+-") {
		return name
	}
	words := strings.Fields(spaced)
	// The guide writes "CCTV5PLUS" for "CCTV5+": the plus becomes a word.
	plus := false
	for i := range words {
		if words[i] == "plus" {
			words[i], plus = "+", true
		}
	}
	if len(words) < 2 {
		return ""
	}
	head := strings.Join(words[:2], " ")
	// The words of a name join in more than one way: "CCTV1综合" for
	// "CCTV-1 综合", and "cctv5" for the head of "CCTV5+".
	if name, ok := m.exact[strings.Join(words, "")]; ok && (plus || len(words) > 2) {
		return name
	}
	// The plus is part of the name: "CCTV5+" and "CCTV5" differ.
	if plus == m.plus[head] {
		if len(words) > 2 && words[2] != "+" {
			if name, ok := m.exact[words[0]+words[1]]; ok {
				return name // "CCTV-1" for "CCTV-1 综合"
			}
		}
		// "CCTV13" for "CCTV-13 新闻": the guide leaves out the topic.
		if m.plain[head] == "" && len(m.heads[head]) == 1 {
			return m.heads[head][0]
		}
	}
	// A guide writes more of a name than the playlist, as "CCTV1 综合 高清":
	// the longest key that ends a word of the name wins.
	flat := strings.Join(words, "")
	best, bestLen := "", 0
	for key, name := range m.exact {
		flatKey := strings.ReplaceAll(key, " ", "")
		if len(flatKey) >= len(flat) || !strings.HasPrefix(flat, flatKey) || len(flatKey) <= bestLen {
			continue
		}
		rest := []rune(flat[len(flatKey):])
		if rest[0] >= '0' && rest[0] <= '9' {
			continue // "cctv1" does not begin "cctv10"
		}
		best, bestLen = name, len(flatKey)
	}
	return best
}

type EPGProgram struct {
	ID        string `json:"id,omitempty"`
	ChannelID string `json:"channelId"` // XMLTV channel id
	Title     string `json:"title"`
	Desc      string `json:"desc,omitempty"`
	Category  string `json:"category,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Start     int64  `json:"start"` // Unix milliseconds
	Stop      int64  `json:"stop"`  // Unix milliseconds, 0 if unknown
}

type EPGGuide struct {
	ByChannel map[string][]EPGProgram `json:"byChannel"` // channel name to its programmes
	URL       string                  `json:"url,omitempty"`
	FetchedAt int64                   `json:"fetchedAt,omitempty"`
	Programs  int                     `json:"programs"`
}

// readXMLTV reads the programmes of a guide in a stream, so that a large
// guide never stays in memory as a whole. It keeps the programmes of the
// window only.
func readXMLTV(r io.Reader, now time.Time, loc *time.Location) (*epgRawGuide, error) {
	if loc == nil {
		loc = time.Local
	}
	windowStart := now.Add(-epgPastWindow).Unix()
	windowEnd := now.Add(epgWindow).Unix()
	raw := &epgRawGuide{started: now}
	seen := map[string]bool{}

	dec := xml.NewDecoder(r)
	dec.Strict = false
	dec.CharsetReader = charsetReader
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if len(raw.programs) > 0 {
				break // Keep what a cut or broken guide gave.
			}
			return nil, fmt.Errorf("read XMLTV guide: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "programme" {
			continue
		}
		var prog struct {
			Channel  string   `xml:"channel,attr"`
			Start    string   `xml:"start,attr"`
			Stop     string   `xml:"stop,attr"`
			Title    string   `xml:"title"`
			SubTitle string   `xml:"sub-title"`
			Desc     string   `xml:"desc"`
			Category []string `xml:"category"`
			Icon     xmlIcon  `xml:"icon"`
		}
		if err := dec.DecodeElement(&prog, &se); err != nil {
			continue
		}
		start, ok := parseXMLTVTime(prog.Start, loc)
		if !ok {
			continue
		}
		if start.Unix() > windowEnd {
			continue
		}
		if stop, ok := parseXMLTVTime(prog.Stop, loc); ok && !stop.IsZero() && stop.Unix() < windowStart {
			continue
		}
		id := normalizeChannelName(prog.Channel) + "@" + strings.TrimSpace(prog.Start)
		if id == "@" || seen[id] {
			continue
		}
		seen[id] = true
		raw.programs = append(raw.programs, epgRawProgram{
			xmltvID:  prog.Channel,
			startRaw: prog.Start,
			stopRaw:  prog.Stop,
			title:    strings.TrimSpace(prog.Title),
			sub:      strings.TrimSpace(prog.SubTitle),
			desc:     strings.TrimSpace(prog.Desc),
			cats:     prog.Category,
			icon:     strings.TrimSpace(prog.Icon.Src),
		})
	}
	return raw, nil
}

// match pairs the programmes with the channels that the playlist holds now.
func (raw *epgRawGuide) match(channels []Channel, loc *time.Location) (*EPGGuide, *epgMatcher) {
	if loc == nil {
		loc = time.Local
	}
	m := newEPGMatcher(channels)
	guide := &EPGGuide{
		ByChannel: map[string][]EPGProgram{},
		FetchedAt: raw.started.UnixMilli(),
	}
	for _, p := range raw.programs {
		name := m.channelFor(p.xmltvID)
		if name == "" {
			continue
		}
		start, ok := parseXMLTVTime(p.startRaw, loc)
		if !ok {
			continue
		}
		var stop int64
		if t, ok := parseXMLTVTime(p.stopRaw, loc); ok {
			stop = t.UnixMilli()
		}
		title := p.title
		if p.sub != "" {
			title = title + " · " + p.sub
		}
		if title == "" {
			continue
		}
		guide.ByChannel[name] = append(guide.ByChannel[name], EPGProgram{
			ID:        name + "@" + p.startRaw,
			ChannelID: strings.TrimSpace(p.xmltvID),
			Title:     title,
			Desc:      p.desc,
			Category:  strings.TrimSpace(strings.Join(p.cats, " / ")),
			Icon:      p.icon,
			Start:     start.UnixMilli(),
			Stop:      stop,
		})
		guide.Programs++
	}
	for name, list := range guide.ByChannel {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Start < list[j].Start })
		guide.ByChannel[name] = list
	}
	m.rev = channelsRevOf(channels)
	return guide, m
}

type xmlIcon struct {
	Src string `xml:"src,attr"`
}

var xmltvLayouts = []string{"20060102150405 -0700", "20060102150405 -07:00", "20060102150405", "200601021504", "20060102"}

// parseXMLTVTime reads the XMLTV time format, as in "20261121004500 +0800".
func parseXMLTVTime(s string, loc *time.Location) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range xmltvLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// charsetReader reads a guide that is not in UTF-8, as the Chinese guides
// that keep GBK titles are.
func charsetReader(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	case "gb2312", "gbk", "gb18030":
		return transform.NewReader(input, simplifiedchinese.GB18030.NewDecoder()), nil
	}
	return input, nil
}

// ---- Guide service ----

const (
	maxEPGBytes      = 64 << 20
	epgCacheLifetime = 6 * time.Hour
	// The guide holds the programmes from two hours back on.
	epgPastWindow = 2 * time.Hour
)

type epgService struct {
	store *Store
	netc  *netClients
	cfgFn func() NetConfig
	// onReady runs when the pane opens or a fetch ends, so that a pane
	// that waits for the guide gets it without a second call.
	onReady func()

	mu        sync.Mutex
	guide     *EPGGuide
	raw       *epgRawGuide // the programmes as read, for a new match
	matcher   *epgMatcher  // the match that made guide
	urls      []string
	updatedAt int64
	fetching  map[string]bool
	// rev names the channels that the guide was matched with, so that a
	// changed channel list gets a new match without a fetch.
	rev    string
	cancel context.CancelFunc
	bgCtx  context.Context
}

func newEPGService(ctx context.Context, store *Store, netc *netClients, cfgFn func() NetConfig) *epgService {
	bgCtx, cancel := context.WithCancel(ctx)
	svc := &epgService{
		store:    store,
		netc:     netc,
		cfgFn:    cfgFn,
		fetching: map[string]bool{},
		bgCtx:    bgCtx,
		cancel:   cancel,
	}
	if data, err := os.ReadFile(store.epgPath()); err == nil {
		var cached epgCacheFile
		if json.Unmarshal(data, &cached) == nil && cached.Guide.ByChannel != nil {
			svc.guide = &cached.Guide
			svc.urls = cached.URLs
			svc.updatedAt = cached.UpdatedAt
		}
	}
	return svc
}

func (s *epgService) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

type epgRawProgram struct {
	xmltvID  string
	startRaw string
	stopRaw  string
	title    string
	sub      string
	desc     string
	cats     []string
	icon     string
}

type epgRawGuide struct {
	programs []epgRawProgram
	started  time.Time
}

type epgCacheFile struct {
	URLs      []string `json:"urls"`
	UpdatedAt int64    `json:"updatedAt"`
	Guide     EPGGuide `json:"guide"`
}

// epgURLsOf lists the guides that the sources name, as in
// #EXTM3U x-tvg-url="a.xml,b.xml".
func epgURLsOf(sources []Source) []string {
	seen := map[string]bool{}
	var out []string
	for _, src := range sources {
		for _, part := range strings.Split(strings.ReplaceAll(src.EPGURL, "|", ","), ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			if _, err := validateHTTPURL(part); err != nil {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}

// programGuide returns the programmes of each channel, the channel names
// keys the map. It also returns the time of the last fetch.
func (s *epgService) programGuide() (map[string][]EPGProgram, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string][]EPGProgram{}
	if s.guide == nil {
		return out, 0
	}
	since := time.Now().Add(-epgPastWindow).UnixMilli()
	for name, list := range s.guide.ByChannel {
		kept := make([]EPGProgram, 0, len(list))
		for _, p := range list {
			if p.Stop > 0 && p.Stop < since {
				continue
			}
			kept = append(kept, p)
		}
		if len(kept) > 0 {
			out[name] = kept
		}
	}
	return out, s.updatedAt
}

func (s *epgService) status() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := "idle"
	if len(s.fetching) > 0 {
		state = "running"
	} else if s.guide != nil && s.guide.Programs > 0 {
		state = "ready"
	}
	programs := 0
	channels := 0
	if s.guide != nil {
		programs = s.guide.Programs
		channels = len(s.guide.ByChannel)
	}
	return map[string]any{
		"state":     state,
		"updatedAt": s.updatedAt,
		"programs":  programs,
		"channels":  channels,
		"urls":      append([]string(nil), s.urls...),
	}
}

// rematchIfStale matches the guide it has with these channels when the
// channels changed since the last match. Without it, a channel from a
// source that the user just added shows no programme until the next
// fetch of the guide.
func (s *epgService) rematchIfStale(channels []Channel) {
	s.mu.Lock()
	m := s.matcher
	stale := m != nil && m.rev != channelsRevOf(channels)
	s.mu.Unlock()
	if stale {
		s.rematch(channels)
	}
}

// channelsRevOf names the channels by their ids and names.
func channelsRevOf(channels []Channel) string {
	h := sha256.New()
	for _, ch := range channels {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", ch.ID, ch.Name, ch.NormName)
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func (s *epgService) idle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.fetching) == 0
}

// start returns at once and fetches the guides in the background. channelFn
// gives the channels that the guide keeps now; it runs on each fetch, so
// that a channel added later finds its programmes on the next refresh.
func (s *epgService) start(channelFn func() []Channel, urls []string) {
	if len(urls) == 0 {
		return
	}
	s.mu.Lock()
	s.urls = urls
	due := s.guide == nil || s.guide.Programs == 0 || time.Since(time.UnixMilli(s.updatedAt)) > epgCacheLifetime
	var todo []string
	for _, u := range urls {
		if due && !s.fetching[u] {
			s.fetching[u] = true
			todo = append(todo, u)
		}
	}
	s.mu.Unlock()
	for _, u := range todo {
		go s.fetchOne(u, channelFn)
	}
}

// refresh fetches the guides now, whatever their age.
func (s *epgService) refresh(channelFn func() []Channel, urls []string) {
	if len(urls) == 0 {
		return
	}
	s.mu.Lock()
	s.urls = urls
	var todo []string
	for _, u := range urls {
		if !s.fetching[u] {
			s.fetching[u] = true
			todo = append(todo, u)
		}
	}
	s.mu.Unlock()
	for _, u := range todo {
		go s.fetchOne(u, channelFn)
	}
}

func (s *epgService) fetchOne(rawURL string, channelFn func() []Channel) {
	defer func() {
		s.mu.Lock()
		delete(s.fetching, rawURL)
		ready := s.onReady
		s.mu.Unlock()
		if ready != nil {
			ready()
		}
	}()
	ctx, cancel := context.WithTimeout(s.bgCtx, 90*time.Second)
	defer cancel()

	resp, err := s.netc.doSmart(ctx, http.MethodGet, rawURL, requestHeaders{Accept: "application/xml,text/xml,*/*"}, s.cfgFn(), false, false)
	if err != nil {
		log.Printf("iptv epg %s: %v", rawURL, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("iptv epg %s: HTTP %s", rawURL, resp.Status)
		return
	}
	// The playlist may name a gzip file. Go unzips only when the server
	// gives Content-Encoding.
	body := io.Reader(io.LimitReader(resp.Body, maxEPGBytes))
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(rawURL)), ".gz") {
		if zr, zerr := gzip.NewReader(body); zerr == nil {
			defer zr.Close()
			body = io.LimitReader(zr, maxEPGBytes)
		}
	}
	raw, err := readXMLTV(body, time.Now(), nil)
	if err != nil {
		log.Printf("iptv epg %s: %v", rawURL, err)
		return
	}
	guide, matcher := raw.match(channelFn(), nil)
	guide.URL = rawURL

	s.mu.Lock()
	s.guide = guide
	s.raw = raw
	s.matcher = matcher
	s.updatedAt = guide.FetchedAt
	data, _ := json.Marshal(epgCacheFile{URLs: s.urls, UpdatedAt: s.updatedAt, Guide: *guide})
	s.mu.Unlock()
	if err := os.WriteFile(s.store.epgPath(), data, 0o644); err != nil {
		log.Printf("iptv epg cache: %v", err)
	}
}

// rematch pairs the programmes again, after the channel list changed.
func (s *epgService) rematch(channels []Channel) {
	s.mu.Lock()
	raw := s.raw
	s.mu.Unlock()
	// A guide from the cache has no programmes as read: only its next
	// fetch matches it again.
	if raw == nil {
		return
	}
	guide, matcher := raw.match(channels, nil)
	s.mu.Lock()
	if s.guide != nil {
		guide.URL = s.guide.URL
	}
	s.guide = guide
	s.matcher = matcher
	data, _ := json.Marshal(epgCacheFile{URLs: s.urls, UpdatedAt: s.updatedAt, Guide: *guide})
	s.mu.Unlock()
	_ = os.WriteFile(s.store.epgPath(), data, 0o644)
}

// setOnReady sets the function that runs when a fetch ends. One call at a
// time runs it: the calls come from the fetches, and a fetch runs alone.
func (s *epgService) setOnReady(fn func()) {
	s.mu.Lock()
	s.onReady = fn
	s.mu.Unlock()
}

// guideURLs returns the URLs that the guide was read from.
func (s *epgService) guideURLs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.urls...)
}
