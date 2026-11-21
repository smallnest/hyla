package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	htmlpkg "html"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

const maxPlaylistBytes = 16 << 20

type Source struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	InputURL      string `json:"inputUrl"`
	ResolvedURL   string `json:"resolvedUrl,omitempty"`
	RawContent    string `json:"rawContent,omitempty"`
	Enabled       bool   `json:"enabled"`
	ChannelCount  int    `json:"channelCount"`
	EPGURL        string `json:"epgUrl,omitempty"`
	AddedAt       int64  `json:"addedAt"`
	LastFetchedAt int64  `json:"lastFetchedAt,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}

type RawChannel struct {
	Name      string `json:"name"`
	TVGID     string `json:"tvgId,omitempty"`
	TVGName   string `json:"tvgName,omitempty"`
	LogoURL   string `json:"logoUrl,omitempty"`
	Group     string `json:"group,omitempty"`
	StreamURL string `json:"streamUrl"`
	Label     string `json:"label,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	Referer   string `json:"referer,omitempty"`
}

type StreamLine struct {
	URL        string `json:"url"`
	SourceID   string `json:"sourceId"`
	SourceName string `json:"sourceName"`
	Label      string `json:"label,omitempty"`
	UserAgent  string `json:"userAgent,omitempty"`
	Referer    string `json:"referer,omitempty"`
	PlayURL    string `json:"playUrl,omitempty"`
}

type Channel struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	NormName     string       `json:"normName"`
	TVGID        string       `json:"tvgId,omitempty"`
	LogoURL      string       `json:"logoUrl,omitempty"`
	LogoProxyURL string       `json:"logoProxyUrl,omitempty"`
	Group        string       `json:"group"`
	Streams      []StreamLine `json:"streams"`
	Favorite     bool         `json:"favorite,omitempty"`
}

type PresetSource struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	NameZh  string `json:"nameZh"`
	NameJa  string `json:"nameJa"`
	Desc    string `json:"desc"`
	DescZh  string `json:"descZh"`
	DescJa  string `json:"descJa"`
	URL     string `json:"url"`
	Country string `json:"country"`
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

func normalizeChannelName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return ""
	}
	fields := strings.Fields(s)
	return strings.ToLower(strings.Join(fields, " "))
}

var (
	hrefAttrRe = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)
	rawURLRe   = regexp.MustCompile(`https?://[^\s"'<>\\` + "`" + `]+`)
	vlcUARe    = regexp.MustCompile(`(?i)^#EXTVLCOPT:http-user-agent\s*=\s*(.+)$`)
	vlcRefRe   = regexp.MustCompile(`(?i)^#EXTVLCOPT:http-referrer\s*=\s*(.+)$`)
	extGrpRe   = regexp.MustCompile(`(?i)^#EXTGRP:\s*(.+)$`)
	// playlistNoticeRe finds a title that holds only a date and a time.
	playlistNoticeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}`)
)

func parseEXTINFAttributes(line string) (attrs map[string]string, title string) {
	attrs = make(map[string]string)
	inQuote := byte(0)
	commaIdx := -1
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			continue
		}
		if c == ',' {
			commaIdx = i
		}
	}
	meta := line
	if commaIdx >= 0 {
		meta = line[:commaIdx]
		title = strings.TrimSpace(line[commaIdx+1:])
	}

	// Scan key="val" or key='val' or key=unquoted pairs inside meta.
	i := 0
	for i < len(meta) {
		for i < len(meta) && (meta[i] == ' ' || meta[i] == '\t') {
			i++
		}
		eq := strings.IndexByte(meta[i:], '=')
		if eq < 0 {
			break
		}
		keyStart := i
		// Key starts after last whitespace before '='.
		seg := meta[i : i+eq]
		if sp := strings.LastIndexAny(seg, " \t:"); sp >= 0 {
			keyStart = i + sp + 1
		}
		key := strings.ToLower(strings.TrimSpace(meta[keyStart : i+eq]))
		valStart := i + eq + 1
		if valStart >= len(meta) {
			break
		}
		var val string
		if meta[valStart] == '"' || meta[valStart] == '\'' {
			q := meta[valStart]
			endQ := strings.IndexByte(meta[valStart+1:], q)
			if endQ >= 0 {
				val = meta[valStart+1 : valStart+1+endQ]
				i = valStart + 1 + endQ + 1
			} else {
				val = meta[valStart+1:]
				i = len(meta)
			}
		} else {
			sp := strings.IndexAny(meta[valStart:], " \t")
			if sp >= 0 {
				val = meta[valStart : valStart+sp]
				i = valStart + sp + 1
			} else {
				val = meta[valStart:]
				i = len(meta)
			}
		}
		if key != "" {
			attrs[key] = strings.TrimSpace(val)
		}
	}
	return attrs, title
}

func ParseM3U(raw []byte) (channels []RawChannel, epgURL string, err error) {
	text := strings.TrimPrefix(string(raw), "\xef\xbb\xbf")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	var (
		hasPending bool
		cur        RawChannel
		curGroup   string
	)
	resetPending := func() {
		hasPending = false
		cur = RawChannel{}
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "#EXTM3U") {
			attrs, _ := parseEXTINFAttributes(line)
			if epgURL == "" {
				if v := attrs["x-tvg-url"]; v != "" {
					epgURL = v
				} else if v := attrs["url-tvg"]; v != "" {
					epgURL = v
				}
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "#EXTINF") {
			attrs, title := parseEXTINFAttributes(line)
			cur = RawChannel{
				Name:    title,
				TVGID:   attrs["tvg-id"],
				TVGName: attrs["tvg-name"],
				LogoURL: attrs["tvg-logo"],
				Group:   attrs["group-title"],
			}
			if cur.Group == "" {
				cur.Group = attrs["group"]
			}
			if cur.Name == "" {
				cur.Name = cur.TVGName
			}
			if cur.Name == "" {
				cur.Name = cur.TVGID
			}
			hasPending = true
			continue
		}
		if m := vlcUARe.FindStringSubmatch(line); len(m) == 2 {
			cur.UserAgent = strings.TrimSpace(m[1])
			continue
		}
		if m := vlcRefRe.FindStringSubmatch(line); len(m) == 2 {
			cur.Referer = strings.TrimSpace(m[1])
			continue
		}
		if m := extGrpRe.FindStringSubmatch(line); len(m) == 2 {
			g := strings.TrimSpace(m[1])
			if hasPending && cur.Group == "" {
				cur.Group = g
			}
			curGroup = g
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Stream URL line (may contain multiple '#' variants or '$label').
		streamURL := line
		label := ""
		if idx := strings.Index(streamURL, "$"); idx > 0 && (strings.HasPrefix(strings.ToLower(streamURL), "http://") || strings.HasPrefix(strings.ToLower(streamURL), "https://")) {
			label = strings.TrimSpace(streamURL[idx+1:])
			streamURL = strings.TrimSpace(streamURL[:idx])
		}
		if u, uerr := validateHTTPURL(streamURL); uerr != nil {
			resetPending()
			continue
		} else {
			streamURL = u.String()
		}

		if !hasPending {
			cur = RawChannel{
				Name:  channelNameFromURL(streamURL),
				Group: curGroup,
			}
		}
		if cur.Group == "" {
			cur.Group = curGroup
		}
		if cur.Name == "" {
			cur.Name = channelNameFromURL(streamURL)
		}
		// Some generated playlists start with a notice whose title is
		// the time of the last check, in the tvg-id of a channel. That
		// entry is not a channel.
		if playlistNoticeRe.MatchString(cur.Name) {
			resetPending()
			continue
		}
		cur.StreamURL = streamURL
		cur.Label = label
		channels = append(channels, cur)
		resetPending()
	}

	if len(channels) == 0 {
		return nil, epgURL, fmt.Errorf("no playable HTTP(S) channels found in M3U playlist")
	}
	return channels, epgURL, nil
}

func ParseTXTGenre(raw []byte) ([]RawChannel, error) {
	text := strings.TrimPrefix(string(raw), "\xef\xbb\xbf")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var out []RawChannel
	curGroup := ""

	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, ",#genre#") {
			curGroup = strings.TrimSpace(strings.TrimSuffix(line, ",#genre#"))
			continue
		}
		name, urlsPart, ok := strings.Cut(line, ",")
		if !ok {
			continue
		}
		chName := strings.TrimSpace(name)
		for _, candidate := range strings.Split(urlsPart, "#") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			label := ""
			if idx := strings.Index(candidate, "$"); idx > 0 {
				label = strings.TrimSpace(candidate[idx+1:])
				candidate = strings.TrimSpace(candidate[:idx])
			}
			u, uerr := validateHTTPURL(candidate)
			if uerr != nil {
				continue
			}
			out = append(out, RawChannel{
				Name:      chName,
				Group:     curGroup,
				StreamURL: u.String(),
				Label:     label,
			})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no playable channels found in TXT genre list")
	}
	return out, nil
}

func channelNameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	base := path.Base(u.Path)
	if base != "" && base != "/" && base != "." {
		base = strings.TrimSuffix(base, path.Ext(base))
		if base != "" {
			return base
		}
	}
	return u.Host
}

func parsePlaylistAuto(raw []byte) (channels []RawChannel, epgURL string, err error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(string(raw), "\xef\xbb\xbf"))
	upper := strings.ToUpper(trimmed)
	if strings.Contains(upper, "#EXTINF") || strings.HasPrefix(upper, "#EXTM3U") {
		return ParseM3U(raw)
	}
	if strings.Contains(trimmed, ",#genre#") || (!looksLikeHTML(trimmed) && strings.Contains(trimmed, ",http")) {
		if chs, terr := ParseTXTGenre(raw); terr == nil && len(chs) > 0 {
			return chs, "", nil
		}
	}
	if !looksLikeHTML(trimmed) {
		// Bare list of http(s) stream URLs, one per line.
		if chs, _, merr := ParseM3U(raw); merr == nil && len(chs) > 0 {
			return chs, "", nil
		}
	}
	return nil, "", fmt.Errorf("unrecognized playlist format")
}

func looksLikeHTML(trimmed string) bool {
	low := strings.ToLower(trimmed)
	if len(low) > 512 {
		low = low[:512]
	}
	return strings.HasPrefix(low, "<!doctype html") || strings.HasPrefix(low, "<html") || strings.Contains(low, "<head>") || strings.Contains(low, "<body")
}

func discoverM3UURLsFromHTML(rawHTML []byte, pageURL string) []string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	pageCanon := strings.TrimRight(base.String(), "/")
	s := string(rawHTML)
	// Normalize JSON escaped slashes/unicode ampersands commonly found in Next.js / SPA payloads.
	normalized := strings.ReplaceAll(s, `\/`, `/`)
	normalized = strings.ReplaceAll(normalized, `\u0026`, `&`)
	normalized = htmlpkg.UnescapeString(normalized)

	type scored struct {
		u     string
		score int
	}
	seen := make(map[string]bool)
	var list []scored

	addCandidate := func(raw string, bonus int) {
		raw = strings.TrimSpace(strings.TrimRight(raw, `\"',);]`))
		if raw == "" {
			return
		}
		ref, rerr := url.Parse(raw)
		if rerr != nil {
			return
		}
		resolved := base.ResolveReference(ref)
		if !strings.EqualFold(resolved.Scheme, "http") && !strings.EqualFold(resolved.Scheme, "https") {
			return
		}
		resolved.Fragment = ""
		canon := resolved.String()
		if strings.TrimRight(canon, "/") == pageCanon {
			return
		}
		lowPath := strings.ToLower(resolved.Path)
		for _, bad := range []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".woff", ".woff2", ".ico"} {
			if strings.HasSuffix(lowPath, bad) {
				return
			}
		}
		if strings.Contains(lowPath, "/cdn-cgi/") {
			return
		}
		score := bonus
		if strings.HasSuffix(lowPath, ".m3u") {
			score += 100
		} else if strings.HasSuffix(lowPath, ".m3u8") {
			score += 80
		} else if strings.HasSuffix(lowPath, ".txt") {
			score += 60
		} else if strings.Contains(strings.ToLower(canon), "m3u") || strings.Contains(strings.ToLower(canon), "iptv") {
			score += 20
		} else {
			return
		}
		if !seen[canon] {
			seen[canon] = true
			list = append(list, scored{u: canon, score: score})
		}
	}

	for _, m := range hrefAttrRe.FindAllStringSubmatch(normalized, -1) {
		addCandidate(m[1], 10)
	}
	for _, match := range rawURLRe.FindAllString(normalized, -1) {
		addCandidate(match, 0)
	}

	sort.SliceStable(list, func(i, j int) bool {
		return list[i].score > list[j].score
	})
	out := make([]string, 0, len(list))
	for _, it := range list {
		out = append(out, it.u)
	}
	return out
}

func ResolveAndFetchSource(ctx context.Context, nc *netClients, cfg NetConfig, input string, hintName string) (Source, []RawChannel, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Source{}, nil, fmt.Errorf("input must not be empty")
	}

	// Raw pasted M3U/TXT content (no HTTP fetch required).
	if strings.Contains(trimmed, "\n") || strings.HasPrefix(strings.ToUpper(trimmed), "#EXTM3U") || strings.Contains(trimmed, ",#genre#") {
		if chs, epg, err := parsePlaylistAuto([]byte(trimmed)); err == nil && len(chs) > 0 {
			name := strings.TrimSpace(hintName)
			if name == "" {
				name = fmt.Sprintf("Local Playlist (%d)", len(chs))
			}
			src := Source{
				ID:           "src-" + shortHash("raw:"+trimmed),
				Name:         name,
				RawContent:   trimmed,
				Enabled:      true,
				ChannelCount: len(chs),
				EPGURL:       epg,
			}
			return src, chs, nil
		}
	}

	u, err := validateHTTPURL(trimmed)
	if err != nil {
		return Source{}, nil, err
	}
	canonURL := u.String()
	data, finalURL, _, err := nc.fetchBytes(ctx, canonURL, cfg, maxPlaylistBytes)
	if err != nil {
		return Source{}, nil, err
	}

	if chs, epg, perr := parsePlaylistAuto(data); perr == nil && len(chs) > 0 {
		return Source{
			ID:           "src-" + shortHash(canonURL),
			Name:         deriveSourceName(hintName, canonURL, finalURL),
			InputURL:     canonURL,
			ResolvedURL:  finalURL,
			Enabled:      true,
			ChannelCount: len(chs),
			EPGURL:       epg,
		}, chs, nil
	}

	// If HTML page, discover embedded/linked M3U playlists.
	if looksLikeHTML(strings.TrimSpace(string(data))) {
		cands := discoverM3UURLsFromHTML(data, finalURL)
		limit := len(cands)
		if limit > 8 {
			limit = 8
		}
		var lastErr error
		for _, cand := range cands[:limit] {
			subData, subFinal, _, ferr := nc.fetchBytes(ctx, cand, cfg, maxPlaylistBytes)
			if ferr != nil {
				lastErr = ferr
				continue
			}
			if chs, epg, perr := parsePlaylistAuto(subData); perr == nil && len(chs) > 0 {
				return Source{
					ID:           "src-" + shortHash(canonURL),
					Name:         deriveSourceName(hintName, canonURL, subFinal),
					InputURL:     canonURL,
					ResolvedURL:  subFinal,
					Enabled:      true,
					ChannelCount: len(chs),
					EPGURL:       epg,
				}, chs, nil
			}
		}
		if lastErr != nil {
			return Source{}, nil, fmt.Errorf("HTML page %s linked M3U failed: %w", canonURL, lastErr)
		}
		return Source{}, nil, fmt.Errorf("no valid M3U/TXT playlist found at or linked from %s", canonURL)
	}

	return Source{}, nil, fmt.Errorf("response from %s is not a valid M3U/TXT playlist", canonURL)
}

func deriveSourceName(hintName, inputURL, resolvedURL string) string {
	if s := strings.TrimSpace(hintName); s != "" {
		return s
	}
	u, err := url.Parse(resolvedURL)
	if err != nil || u.Host == "" {
		u, _ = url.Parse(inputURL)
	}
	if u == nil {
		return inputURL
	}
	base := path.Base(u.Path)
	if base != "" && base != "/" && base != "." {
		return u.Host + "/" + base
	}
	return u.Host
}

func ParseInputLines(raw string) (urls []string, rawPlaylist string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ""
	}
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "#EXTM3U") || strings.Contains(upper, "#EXTINF") || strings.Contains(trimmed, ",#genre#") || strings.Contains(trimmed, ",http") {
		return nil, trimmed
	}
	lines := strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
	seen := make(map[string]bool)
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if u, err := validateHTTPURL(l); err == nil {
			s := u.String()
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		} else {
			// Single invalid input preserves exact error on caller.
			out = append(out, l)
		}
	}
	return out, ""
}

func MergeChannels(sources []Source, perSource map[string][]RawChannel) ([]Channel, []string) {
	var (
		order    []string
		byNorm   = make(map[string]*Channel)
		seenURLs = make(map[string]map[string]bool)
	)
	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		raws := perSource[src.ID]
		for _, rc := range raws {
			if _, err := validateHTTPURL(rc.StreamURL); err != nil {
				continue
			}
			norm := normalizeChannelName(rc.Name)
			if norm == "" {
				continue
			}
			grp := strings.TrimSpace(rc.Group)
			if grp == "" {
				grp = "Other"
			}
			ch, exists := byNorm[norm]
			if !exists {
				ch = &Channel{
					ID:       "ch-" + shortHash(norm),
					Name:     strings.TrimSpace(rc.Name),
					NormName: norm,
					TVGID:    strings.TrimSpace(rc.TVGID),
					LogoURL:  strings.TrimSpace(rc.LogoURL),
					Group:    grp,
				}
				byNorm[norm] = ch
				seenURLs[norm] = make(map[string]bool)
				order = append(order, norm)
			} else {
				if ch.LogoURL == "" && strings.TrimSpace(rc.LogoURL) != "" {
					ch.LogoURL = strings.TrimSpace(rc.LogoURL)
				}
				if ch.TVGID == "" && strings.TrimSpace(rc.TVGID) != "" {
					ch.TVGID = strings.TrimSpace(rc.TVGID)
				}
				if (ch.Group == "" || ch.Group == "Other") && grp != "Other" {
					ch.Group = grp
				}
			}
			if seenURLs[norm][rc.StreamURL] {
				continue
			}
			seenURLs[norm][rc.StreamURL] = true
			ch.Streams = append(ch.Streams, StreamLine{
				URL:        rc.StreamURL,
				SourceID:   src.ID,
				SourceName: src.Name,
				Label:      rc.Label,
				UserAgent:  rc.UserAgent,
				Referer:    rc.Referer,
			})
		}
	}

	channels := make([]Channel, 0, len(order))
	groupSet := make(map[string]bool)
	for _, norm := range order {
		ch := byNorm[norm]
		if len(ch.Streams) == 0 {
			continue
		}
		channels = append(channels, *ch)
		groupSet[ch.Group] = true
	}

	priorityGroups := map[string]int{
		"央视频道": 1,
		"卫视频道": 2,
		"港澳台":  3,
		"电影频道": 4,
		"纪录频道": 5,
		"体育频道": 6,
		"新闻频道": 7,
		"儿童频道": 8,
		"音乐频道": 9,
		"地方频道": 10,
	}
	groups := make([]string, 0, len(groupSet))
	for g := range groupSet {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		pi, okI := priorityGroups[groups[i]]
		pj, okJ := priorityGroups[groups[j]]
		if okI && okJ {
			return pi < pj
		}
		if okI != okJ {
			return okI
		}
		return groups[i] < groups[j]
	})
	return channels, groups
}

func ExportM3U(channels []Channel) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, ch := range channels {
		for _, st := range ch.Streams {
			fmt.Fprintf(&b, "#EXTINF:-1 tvg-id=%q tvg-name=%q tvg-logo=%q group-title=%q, %s\n",
				ch.TVGID, ch.Name, ch.LogoURL, ch.Group, ch.Name)
			if st.UserAgent != "" {
				fmt.Fprintf(&b, "#EXTVLCOPT:http-user-agent=%s\n", st.UserAgent)
			}
			if st.Referer != "" {
				fmt.Fprintf(&b, "#EXTVLCOPT:http-referrer=%s\n", st.Referer)
			}
			b.WriteString(st.URL)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
