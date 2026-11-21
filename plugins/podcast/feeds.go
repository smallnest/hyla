package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	defaultITunesBaseURL = "https://itunes.apple.com"
	maxFeedBytes         = 16 << 20
	maxHTMLPageBytes     = 4 << 20
)

type SearchResult struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	FeedURL    string `json:"feedUrl"`
	ArtworkURL string `json:"artworkUrl,omitempty"`
	CoverURL   string `json:"coverUrl,omitempty"`
	Genre      string `json:"genre,omitempty"`
	Episodes   int    `json:"episodes,omitempty"`
}

type OPMLEntry struct {
	Title   string `json:"title"`
	FeedURL string `json:"feedUrl"`
	SiteURL string `json:"siteUrl,omitempty"`
}

var (
	appleIDRe        = regexp.MustCompile(`id(\d{5,})`)
	xyzPodcastRe     = regexp.MustCompile(`xiaoyuzhoufm\.com/podcast/([A-Za-z0-9]+)`)
	xyzEpisodeRe     = regexp.MustCompile(`xiaoyuzhoufm\.com/episode/([A-Za-z0-9]+)`)
	nextDataRe       = regexp.MustCompile(`(?is)<script[^>]*id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)
	linkTagRe        = regexp.MustCompile(`(?is)<link\b([^>]+)>`)
	attrRe           = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	htmlTagRe        = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlBreakRe      = regexp.MustCompile(`(?is)</?(?:br|p|div|li|tr|h[1-6]|blockquote)\b[^>]*>`)
	multiNewlineRe   = regexp.MustCompile(`\n{3,}`)
	chapterLineRe    = regexp.MustCompile(`^\s*(?:[-*•]\s*)?[\[\(]?(\d{1,2}:\d{2}(?::\d{2})?)[\]\)]?\s*(?:[-–—:|]\s*)?(.+?)\s*$`)
)

func ParseFeed(data []byte, feedURL string) (Feed, []Episode, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}

	var rootName string
	for {
		tok, err := dec.Token()
		if err != nil {
			return Feed{}, nil, fmt.Errorf("parse feed XML: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			rootName = strings.ToLower(se.Name.Local)
			break
		}
	}

	switch rootName {
	case "rss", "rdf":
		return parseRSS(dec, feedURL)
	case "feed":
		return parseAtom(dec, feedURL)
	default:
		return Feed{}, nil, fmt.Errorf("unsupported feed root element <%s>", rootName)
	}
}

func parseRSS(dec *xml.Decoder, feedURL string) (Feed, []Episode, error) {
	feed := Feed{
		ID:         makeFeedID(feedURL),
		FeedURL:    feedURL,
		SourceType: "rss",
	}
	var episodes []Episode

	var inChannel, inItem, inImage, inPscChapters bool
	var curEp Episode
	var curPath []string

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Feed{}, nil, fmt.Errorf("decode RSS: %w", err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(el.Name.Local)
			space := strings.ToLower(el.Name.Space)
			curPath = append(curPath, local)
			switch {
			case local == "channel":
				inChannel = true
			case inChannel && local == "item":
				inItem = true
				curEp = Episode{FeedID: feed.ID}
			case inChannel && !inItem && local == "image":
				if href := xmlAttr(el.Attr, "href"); href != "" && feed.ArtworkURL == "" {
					feed.ArtworkURL = href
				} else {
					inImage = true
				}
			case inItem && local == "image":
				if href := xmlAttr(el.Attr, "href"); href != "" && curEp.ImageURL == "" {
					curEp.ImageURL = href
				}
			case inItem && local == "enclosure":
				u := xmlAttr(el.Attr, "url")
				if u != "" && curEp.AudioURL == "" {
					curEp.AudioURL = u
					curEp.AudioType = xmlAttr(el.Attr, "type")
					if n, err := strconv.ParseInt(xmlAttr(el.Attr, "length"), 10, 64); err == nil && n > 0 {
						curEp.AudioBytes = n
					}
				}
			case inItem && local == "content" && strings.Contains(space, "search.yahoo.com/mrss"):
				u := xmlAttr(el.Attr, "url")
				t := xmlAttr(el.Attr, "type")
				if u != "" && curEp.AudioURL == "" && (strings.HasPrefix(t, "audio/") || isAudioURL(u)) {
					curEp.AudioURL = u
					curEp.AudioType = t
				}
			case inItem && local == "thumbnail":
				if u := xmlAttr(el.Attr, "url"); u != "" && curEp.ImageURL == "" {
					curEp.ImageURL = u
				}
			case inItem && local == "transcript":
				u := xmlAttr(el.Attr, "url")
				t := xmlAttr(el.Attr, "type")
				if u != "" && shouldPreferTranscript(curEp.TranscriptType, t) {
					curEp.TranscriptURL = u
					curEp.TranscriptType = t
				}
			case inItem && local == "chapters":
				if u := xmlAttr(el.Attr, "url"); u != "" {
					curEp.ChaptersURL = u
				} else {
					inPscChapters = true
				}
			case inItem && inPscChapters && local == "chapter":
				start := parseDurationSec(xmlAttr(el.Attr, "start"))
				title := strings.TrimSpace(xmlAttr(el.Attr, "title"))
				if title != "" {
					curEp.Chapters = append(curEp.Chapters, Chapter{
						StartSec: start,
						Title:    title,
						URL:      xmlAttr(el.Attr, "href"),
						Img:      xmlAttr(el.Attr, "image"),
					})
				}
			case inChannel && !inItem && local == "link":
				if rel := xmlAttr(el.Attr, "rel"); rel == "" || rel == "alternate" {
					if href := xmlAttr(el.Attr, "href"); href != "" && feed.SiteURL == "" {
						feed.SiteURL = href
					}
				}
			default:
				if inChannel && !inItem && len(curPath) >= 2 && curPath[len(curPath)-2] == "channel" {
					text, err := readElementText(dec)
					if err == nil {
						curPath = curPath[:len(curPath)-1]
						applyRSSChannelField(&feed, local, space, text)
					}
				} else if inChannel && inImage && local == "url" {
					text, err := readElementText(dec)
					if err == nil {
						curPath = curPath[:len(curPath)-1]
						if feed.ArtworkURL == "" {
							feed.ArtworkURL = strings.TrimSpace(text)
						}
					}
				} else if inItem && !inPscChapters && isRSSItemTextTag(local, space) {
					text, err := readElementText(dec)
					if err == nil {
						curPath = curPath[:len(curPath)-1]
						applyRSSItemField(&curEp, local, space, text)
					}
				}
			}

		case xml.EndElement:
			local := strings.ToLower(el.Name.Local)
			if len(curPath) > 0 {
				curPath = curPath[:len(curPath)-1]
			}
			switch local {
			case "channel":
				inChannel = false
			case "image":
				inImage = false
			case "chapters":
				inPscChapters = false
			case "item":
				if inItem {
					finalizeEpisode(&feed, &curEp)
					if curEp.AudioURL != "" || curEp.Title != "" {
						episodes = append(episodes, curEp)
					}
					inItem = false
				}
			}
		}
	}

	if feed.Title == "" {
		feed.Title = feedURL
	}
	for i := range episodes {
		episodes[i].FeedTitle = feed.Title
		if episodes[i].ImageURL == "" {
			episodes[i].ImageURL = feed.ArtworkURL
		}
	}
	return feed, episodes, nil
}

func isRSSItemTextTag(local, space string) bool {
	switch local {
	case "title", "guid", "link", "description", "summary", "subtitle", "encoded", "pubdate", "author", "creator", "duration":
		return true
	}
	return false
}

func applyRSSChannelField(feed *Feed, local, space, text string) {
	val := strings.TrimSpace(text)
	switch local {
	case "title":
		if feed.Title == "" {
			feed.Title = html.UnescapeString(val)
		}
	case "description", "summary", "subtitle":
		if val != "" && (feed.Description == "" || local == "description") {
			feed.Description = cleanHTMLText(val)
		}
	case "author", "managingeditor", "creator":
		if feed.Author == "" && val != "" {
			feed.Author = html.UnescapeString(val)
		}
	case "link":
		if feed.SiteURL == "" && !strings.Contains(space, "atom") && val != "" {
			feed.SiteURL = val
		}
	case "language":
		if feed.Language == "" && val != "" {
			feed.Language = val
		}
	}
}

func applyRSSItemField(ep *Episode, local, space, text string) {
	val := strings.TrimSpace(text)
	switch local {
	case "title":
		if ep.Title == "" {
			ep.Title = html.UnescapeString(val)
		}
	case "guid":
		if ep.GUID == "" {
			ep.GUID = val
		}
	case "link":
		if ep.LinkURL == "" && val != "" {
			ep.LinkURL = val
		}
	case "pubdate":
		if ts := parsePubDate(val); ts > 0 {
			ep.PublishedAt = ts
		}
	case "duration":
		if d := parseDurationSec(val); d > 0 {
			ep.DurationSec = d
		}
	case "summary", "subtitle":
		if ep.Summary == "" && val != "" {
			ep.Summary = cleanHTMLText(val)
		}
	case "description":
		if val != "" && ep.Description == "" {
			ep.Description = cleanHTMLText(val)
		}
	case "encoded":
		if val != "" {
			cleaned := cleanHTMLText(val)
			if len(cleaned) > len(ep.Description) {
				ep.Description = cleaned
			}
		}
	}
}

func parseAtom(dec *xml.Decoder, feedURL string) (Feed, []Episode, error) {
	feed := Feed{
		ID:         makeFeedID(feedURL),
		FeedURL:    feedURL,
		SourceType: "rss",
	}
	var episodes []Episode
	var inEntry bool
	var curEp Episode

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Feed{}, nil, fmt.Errorf("decode Atom: %w", err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(el.Name.Local)
			switch {
			case local == "entry":
				inEntry = true
				curEp = Episode{FeedID: feed.ID}
			case !inEntry && local == "link":
				rel := xmlAttr(el.Attr, "rel")
				href := xmlAttr(el.Attr, "href")
				if (rel == "" || rel == "alternate") && href != "" && feed.SiteURL == "" {
					feed.SiteURL = href
				}
			case inEntry && local == "link":
				rel := xmlAttr(el.Attr, "rel")
				href := xmlAttr(el.Attr, "href")
				t := xmlAttr(el.Attr, "type")
				if rel == "enclosure" && href != "" && curEp.AudioURL == "" {
					curEp.AudioURL = href
					curEp.AudioType = t
					if n, err := strconv.ParseInt(xmlAttr(el.Attr, "length"), 10, 64); err == nil {
						curEp.AudioBytes = n
					}
				} else if (rel == "" || rel == "alternate") && href != "" && curEp.LinkURL == "" {
					curEp.LinkURL = href
				}
			case inEntry && local == "transcript":
				u := xmlAttr(el.Attr, "url")
				t := xmlAttr(el.Attr, "type")
				if u != "" && shouldPreferTranscript(curEp.TranscriptType, t) {
					curEp.TranscriptURL = u
					curEp.TranscriptType = t
				}
			case inEntry && local == "chapters":
				if u := xmlAttr(el.Attr, "url"); u != "" {
					curEp.ChaptersURL = u
				}
			case inEntry && (local == "title" || local == "id" || local == "published" || local == "updated" || local == "summary" || local == "content" || local == "duration"):
				text, err := readElementText(dec)
				if err == nil {
					val := strings.TrimSpace(text)
					switch local {
					case "title":
						if curEp.Title == "" {
							curEp.Title = html.UnescapeString(val)
						}
					case "id":
						if curEp.GUID == "" {
							curEp.GUID = val
						}
					case "published", "updated":
						if curEp.PublishedAt == 0 {
							curEp.PublishedAt = parsePubDate(val)
						}
					case "summary":
						curEp.Summary = cleanHTMLText(val)
					case "content":
						cleaned := cleanHTMLText(val)
						if len(cleaned) > len(curEp.Description) {
							curEp.Description = cleaned
						}
					case "duration":
						curEp.DurationSec = parseDurationSec(val)
					}
				}
			case !inEntry && (local == "title" || local == "subtitle" || local == "name" || local == "logo" || local == "icon"):
				text, err := readElementText(dec)
				if err == nil {
					val := strings.TrimSpace(text)
					switch local {
					case "title":
						if feed.Title == "" {
							feed.Title = html.UnescapeString(val)
						}
					case "subtitle":
						if feed.Description == "" {
							feed.Description = cleanHTMLText(val)
						}
					case "name":
						if feed.Author == "" {
							feed.Author = html.UnescapeString(val)
						}
					case "logo", "icon":
						if feed.ArtworkURL == "" {
							feed.ArtworkURL = val
						}
					}
				}
			}
		case xml.EndElement:
			if strings.ToLower(el.Name.Local) == "entry" && inEntry {
				finalizeEpisode(&feed, &curEp)
				if curEp.AudioURL != "" || curEp.Title != "" {
					episodes = append(episodes, curEp)
				}
				inEntry = false
			}
		}
	}

	if feed.Title == "" {
		feed.Title = feedURL
	}
	for i := range episodes {
		episodes[i].FeedTitle = feed.Title
		if episodes[i].ImageURL == "" {
			episodes[i].ImageURL = feed.ArtworkURL
		}
	}
	return feed, episodes, nil
}

func readElementText(dec *xml.Decoder) (string, error) {
	var b strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return b.String(), err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			depth++
			b.WriteByte('<')
			b.WriteString(v.Name.Local)
			b.WriteByte('>')
		case xml.EndElement:
			depth--
			if depth > 0 {
				b.WriteString("</")
				b.WriteString(v.Name.Local)
				b.WriteByte('>')
			}
		case xml.CharData:
			b.Write(v)
		}
	}
	return b.String(), nil
}

func xmlAttr(attrs []xml.Attr, localName string) string {
	for _, a := range attrs {
		if strings.EqualFold(a.Name.Local, localName) {
			return strings.TrimSpace(a.Value)
		}
	}
	return ""
}

func shouldPreferTranscript(existingType, nextType string) bool {
	rank := func(t string) int {
		lt := strings.ToLower(t)
		switch {
		case strings.Contains(lt, "vtt"):
			return 4
		case strings.Contains(lt, "srt"):
			return 3
		case strings.Contains(lt, "json"):
			return 2
		case lt != "":
			return 1
		default:
			return 0
		}
	}
	return rank(nextType) > rank(existingType) || existingType == ""
}

func finalizeEpisode(feed *Feed, ep *Episode) {
	if ep.Summary == "" && ep.Description != "" {
		ep.Summary = truncateRunes(strings.ReplaceAll(ep.Description, "\n", " "), 220)
	}
	if ep.Description == "" {
		ep.Description = ep.Summary
	}
	if len(ep.Chapters) == 0 && ep.Description != "" {
		ep.Chapters = extractShowNoteChapters(ep.Description)
	}
	key := ep.GUID
	if key == "" {
		key = ep.AudioURL
	}
	if key == "" {
		key = ep.Title
	}
	if ep.ID == "" {
		ep.ID = makeEpisodeID(feed.ID, key)
	}
}

func truncateRunes(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= maxRunes {
		return string(r)
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

func cleanHTMLText(raw string) string {
	if raw == "" {
		return ""
	}
	s := htmlBreakRe.ReplaceAllString(raw, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		out = append(out, trimmed)
	}
	res := strings.TrimSpace(strings.Join(out, "\n"))
	return multiNewlineRe.ReplaceAllString(res, "\n\n")
}

func extractShowNoteChapters(desc string) []Chapter {
	var out []Chapter
	for _, line := range strings.Split(desc, "\n") {
		m := chapterLineRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		sec := parseDurationSec(m[1])
		title := strings.TrimSpace(m[2])
		if title == "" || len([]rune(title)) < 2 {
			continue
		}
		if len(out) > 0 && sec < out[len(out)-1].StartSec {
			continue
		}
		out = append(out, Chapter{
			StartSec: sec,
			Title:    title,
		})
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

func parseDurationSec(raw string) float64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	if !strings.Contains(s, ":") {
		f, err := strconv.ParseFloat(s, 64)
		if err == nil && f >= 0 {
			return f
		}
		return 0
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	var total float64
	for i, part := range parts {
		part = strings.ReplaceAll(strings.TrimSpace(part), ",", ".")
		v, err := strconv.ParseFloat(part, 64)
		if err != nil || v < 0 {
			return 0
		}
		if i < len(parts)-1 {
			total = (total + v) * 60
		} else {
			total += v
		}
	}
	return total
}

func parsePubDate(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC822Z,
		time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}

func isAudioURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	p := strings.ToLower(u.Path)
	for _, ext := range []string{".mp3", ".m4a", ".wav", ".ogg", ".aac", ".opus", ".flac", ".m4b"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func SearchITunes(ctx context.Context, client *http.Client, itunesBase, query, country string) ([]SearchResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("search query is empty")
	}
	if itunesBase == "" {
		itunesBase = defaultITunesBaseURL
	}
	c := strings.ToLower(strings.TrimSpace(country))
	if c == "" || c == "auto" {
		if hasCJK(q) {
			c = "cn"
		} else {
			c = "us"
		}
	}
	u := fmt.Sprintf("%s/search?media=podcast&entity=podcast&term=%s&country=%s&limit=25",
		strings.TrimRight(itunesBase, "/"),
		url.QueryEscape(q),
		url.QueryEscape(c),
	)
	b, _, err := fetchBytes(ctx, client, u, 4<<20)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Results []struct {
			CollectionID   int64  `json:"collectionId"`
			CollectionName string `json:"collectionName"`
			ArtistName     string `json:"artistName"`
			FeedURL        string `json:"feedUrl"`
			ArtworkURL600  string `json:"artworkUrl600"`
			ArtworkURL100  string `json:"artworkUrl100"`
			PrimaryGenre   string `json:"primaryGenreName"`
			TrackCount     int    `json:"trackCount"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, fmt.Errorf("decode iTunes response: %w", err)
	}
	out := make([]SearchResult, 0, len(payload.Results))
	for _, r := range payload.Results {
		if strings.TrimSpace(r.FeedURL) == "" {
			continue
		}
		art := r.ArtworkURL600
		if art == "" {
			art = r.ArtworkURL100
		}
		out = append(out, SearchResult{
			ID:         strconv.FormatInt(r.CollectionID, 10),
			Title:      r.CollectionName,
			Author:     r.ArtistName,
			FeedURL:    r.FeedURL,
			ArtworkURL: art,
			Genre:      r.PrimaryGenre,
			Episodes:   r.TrackCount,
		})
	}
	return out, nil
}

func LookupITunesFeed(ctx context.Context, client *http.Client, itunesBase, itunesID string) (string, string, error) {
	if itunesBase == "" {
		itunesBase = defaultITunesBaseURL
	}
	u := fmt.Sprintf("%s/lookup?id=%s&entity=podcast", strings.TrimRight(itunesBase, "/"), url.QueryEscape(itunesID))
	b, _, err := fetchBytes(ctx, client, u, 2<<20)
	if err != nil {
		return "", "", err
	}
	var payload struct {
		Results []struct {
			FeedURL       string `json:"feedUrl"`
			ArtworkURL600 string `json:"artworkUrl600"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return "", "", err
	}
	for _, r := range payload.Results {
		if strings.TrimSpace(r.FeedURL) != "" {
			return r.FeedURL, r.ArtworkURL600, nil
		}
	}
	return "", "", fmt.Errorf("no RSS feed found for Apple Podcasts ID %s", itunesID)
}

func ResolveAndFetchInput(ctx context.Context, client *http.Client, itunesBase, rawInput string) (Feed, []Episode, error) {
	raw := strings.TrimSpace(rawInput)
	u, err := validateHTTPURL(raw)
	if err != nil {
		return Feed{}, nil, err
	}
	host := strings.ToLower(u.Hostname())

	// 1. Apple Podcasts link
	if strings.Contains(host, "podcasts.apple.com") {
		if m := appleIDRe.FindStringSubmatch(u.Path); len(m) == 2 {
			feedURL, art, err := LookupITunesFeed(ctx, client, itunesBase, m[1])
			if err != nil {
				return Feed{}, nil, err
			}
			f, eps, err := fetchAndParseFeed(ctx, client, feedURL, "apple")
			if err != nil {
				return Feed{}, nil, err
			}
			if f.ArtworkURL == "" && art != "" {
				f.ArtworkURL = art
			}
			return f, eps, nil
		}
	}

	// 2. Xiaoyuzhou FM link
	if strings.Contains(host, "xiaoyuzhoufm.com") {
		return resolveXiaoyuzhouURL(ctx, client, raw)
	}

	// 3. Direct audio link
	if isAudioURL(raw) {
		feed := Feed{
			ID:         makeFeedID("direct://links"),
			Title:      "Direct Audio Links",
			FeedURL:    "direct://links",
			SourceType: "direct",
		}
		base := u.Path
		if idx := strings.LastIndexByte(base, '/'); idx >= 0 {
			base = base[idx+1:]
		}
		if base == "" {
			base = u.Host
		}
		ep := Episode{
			ID:          makeEpisodeID(feed.ID, raw),
			FeedID:      feed.ID,
			FeedTitle:   feed.Title,
			GUID:        raw,
			Title:       base,
			AudioURL:    raw,
			LinkURL:     raw,
			PublishedAt: time.Now().UnixMilli(),
		}
		return feed, []Episode{ep}, nil
	}

	// 4. RSS/Atom feed or webpage containing alternate RSS link
	body, finalURL, err := fetchBytes(ctx, client, raw, maxFeedBytes)
	if err != nil {
		return Feed{}, nil, err
	}
	trimmed := bytes.TrimSpace(body)
	if bytes.HasPrefix(trimmed, []byte("<")) && !looksLikeHTML(trimmed) {
		return ParseFeed(body, finalURL)
	}
	if rssLink := findAlternateFeedURL(string(body), finalURL); rssLink != "" {
		return fetchAndParseFeed(ctx, client, rssLink, "rss")
	}
	return ParseFeed(body, finalURL)
}

func fetchAndParseFeed(ctx context.Context, client *http.Client, feedURL, sourceType string) (Feed, []Episode, error) {
	b, finalURL, err := fetchBytes(ctx, client, feedURL, maxFeedBytes)
	if err != nil {
		return Feed{}, nil, err
	}
	f, eps, err := ParseFeed(b, finalURL)
	if err != nil {
		return Feed{}, nil, err
	}
	if sourceType != "" {
		f.SourceType = sourceType
	}
	return f, eps, nil
}

func looksLikeHTML(b []byte) bool {
	head := strings.ToLower(string(b[:min(len(b), 512)]))
	return strings.Contains(head, "<!doctype html") || strings.Contains(head, "<html")
}

func findAlternateFeedURL(htmlText, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	matches := linkTagRe.FindAllStringSubmatch(htmlText, -1)
	for _, m := range matches {
		attrs := parseHTMLAttrs(m[1])
		rel := strings.ToLower(attrs["rel"])
		typ := strings.ToLower(attrs["type"])
		href := strings.TrimSpace(attrs["href"])
		if href == "" {
			continue
		}
		if strings.Contains(rel, "alternate") && (strings.Contains(typ, "rss") || strings.Contains(typ, "atom") || strings.Contains(typ, "xml")) {
			if ref, err := url.Parse(href); err == nil {
				return base.ResolveReference(ref).String()
			}
		}
	}
	return ""
}

func parseHTMLAttrs(raw string) map[string]string {
	out := map[string]string{}
	for _, m := range attrRe.FindAllStringSubmatch(raw, -1) {
		k := strings.ToLower(m[1])
		v := m[2]
		if v == "" {
			v = m[3]
		}
		out[k] = html.UnescapeString(v)
	}
	return out
}

func resolveXiaoyuzhouURL(ctx context.Context, client *http.Client, rawURL string) (Feed, []Episode, error) {
	htmlBytes, _, err := fetchBytes(ctx, client, rawURL, maxHTMLPageBytes)
	if err != nil {
		return Feed{}, nil, err
	}
	f, eps, err := parseXiaoyuzhouHTML(htmlBytes, rawURL)
	if err != nil {
		// Fallback to xyzfm RSS feed for podcast URLs when page structure omits __NEXT_DATA__
		if m := xyzPodcastRe.FindStringSubmatch(rawURL); len(m) == 2 {
			rss := "https://feed.xyzfm.space/" + m[1]
			if rf, reps, rerr := fetchAndParseFeed(ctx, client, rss, "xiaoyuzhou"); rerr == nil {
				return rf, reps, nil
			}
		}
		return Feed{}, nil, err
	}
	// If podcast page yielded a feed RSS URL or pid, prefer full RSS when available.
	if f.FeedURL != "" && strings.Contains(f.FeedURL, "feed.xyzfm.space/") && len(eps) == 0 {
		if rf, reps, rerr := fetchAndParseFeed(ctx, client, f.FeedURL, "xiaoyuzhou"); rerr == nil {
			return rf, reps, nil
		}
	}
	return f, eps, nil
}

func parseXiaoyuzhouHTML(htmlBytes []byte, pageURL string) (Feed, []Episode, error) {
	m := nextDataRe.FindSubmatch(htmlBytes)
	if len(m) < 2 {
		return Feed{}, nil, fmt.Errorf("missing __NEXT_DATA__ in Xiaoyuzhou page")
	}
	var doc struct {
		Props struct {
			PageProps struct {
				Podcast *xyzPodcast `json:"podcast"`
				Episode *xyzEpisode `json:"episode"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &doc); err != nil {
		return Feed{}, nil, fmt.Errorf("parse Xiaoyuzhou __NEXT_DATA__: %w", err)
	}

	if p := doc.Props.PageProps.Podcast; p != nil {
		feedURL := pageURL
		if p.Pid != "" {
			feedURL = "https://feed.xyzfm.space/" + p.Pid
		}
		f := Feed{
			ID:          makeFeedID(feedURL),
			Title:       p.Title,
			Author:      p.Author,
			Description: cleanHTMLText(p.Description),
			FeedURL:     feedURL,
			SiteURL:     pageURL,
			ArtworkURL:  p.Image.best(),
			SourceType:  "xiaoyuzhou",
			Language:    "zh",
		}
		eps := make([]Episode, 0, len(p.Episodes))
		for _, xe := range p.Episodes {
			ep := convertXyzEpisode(f, xe)
			if ep.AudioURL != "" {
				eps = append(eps, ep)
			}
		}
		return f, eps, nil
	}

	if xe := doc.Props.PageProps.Episode; xe != nil {
		feedTitle := xe.Podcast.Title
		if feedTitle == "" {
			feedTitle = "小宇宙"
		}
		feedURL := pageURL
		if xe.Podcast.Pid != "" {
			feedURL = "https://www.xiaoyuzhoufm.com/podcast/" + xe.Podcast.Pid
		}
		f := Feed{
			ID:         makeFeedID(feedURL),
			Title:      feedTitle,
			Author:     xe.Podcast.Author,
			FeedURL:    feedURL,
			SiteURL:    feedURL,
			ArtworkURL: xe.Podcast.Image.best(),
			SourceType: "xiaoyuzhou",
			Language:   "zh",
		}
		ep := convertXyzEpisode(f, *xe)
		return f, []Episode{ep}, nil
	}

	return Feed{}, nil, fmt.Errorf("no podcast or episode found in Xiaoyuzhou page")
}

type xyzImage struct {
	PicURL    string `json:"picUrl"`
	LargeURL  string `json:"largePicUrl"`
	MiddleURL string `json:"middlePicUrl"`
}

func (img xyzImage) best() string {
	if img.LargeURL != "" {
		return img.LargeURL
	}
	if img.PicURL != "" {
		return img.PicURL
	}
	return img.MiddleURL
}

type xyzEpisode struct {
	Eid       string   `json:"eid"`
	Title     string   `json:"title"`
	Shownotes string   `json:"shownotes"`
	Duration  float64  `json:"duration"`
	PubDate   string   `json:"pubDate"`
	Image     xyzImage `json:"image"`
	Enclosure struct {
		URL string `json:"url"`
	} `json:"enclosure"`
	Media struct {
		Source struct {
			URL string `json:"url"`
		} `json:"source"`
	} `json:"media"`
	Podcast struct {
		Pid    string   `json:"pid"`
		Title  string   `json:"title"`
		Author string   `json:"author"`
		Image  xyzImage `json:"image"`
	} `json:"podcast"`
}

type xyzPodcast struct {
	Pid         string       `json:"pid"`
	Title       string       `json:"title"`
	Author      string       `json:"author"`
	Description string       `json:"description"`
	Image       xyzImage     `json:"image"`
	Episodes    []xyzEpisode `json:"episodes"`
}

func convertXyzEpisode(f Feed, xe xyzEpisode) Episode {
	audio := xe.Enclosure.URL
	if audio == "" {
		audio = xe.Media.Source.URL
	}
	img := xe.Image.best()
	if img == "" {
		img = f.ArtworkURL
	}
	desc := cleanHTMLText(xe.Shownotes)
	ep := Episode{
		ID:          makeEpisodeID(f.ID, xe.Eid+"|"+audio),
		FeedID:      f.ID,
		FeedTitle:   f.Title,
		GUID:        xe.Eid,
		Title:       xe.Title,
		Description: desc,
		Summary:     truncateRunes(strings.ReplaceAll(desc, "\n", " "), 220),
		AudioURL:    audio,
		ImageURL:    img,
		DurationSec: xe.Duration,
		PublishedAt: parsePubDate(xe.PubDate),
		Chapters:    extractShowNoteChapters(desc),
	}
	if xe.Eid != "" {
		ep.LinkURL = "https://www.xiaoyuzhoufm.com/episode/" + xe.Eid
	}
	return ep
}

type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    struct {
		Title string `xml:"title"`
	} `xml:"head"`
	Body struct {
		Outlines []opmlOutline `xml:"outline"`
	} `xml:"body"`
}

type opmlOutline struct {
	Text     string        `xml:"text,attr,omitempty"`
	Title    string        `xml:"title,attr,omitempty"`
	Type     string        `xml:"type,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Outlines []opmlOutline `xml:"outline,omitempty"`
}

func ExportOPML(feeds []Feed) string {
	var doc opmlDoc
	doc.Version = "2.0"
	doc.Head.Title = "Hyla Podcast Subscriptions"
	for _, f := range feeds {
		if f.SourceType == "direct" || f.FeedURL == "" {
			continue
		}
		doc.Body.Outlines = append(doc.Body.Outlines, opmlOutline{
			Text:    f.Title,
			Title:   f.Title,
			Type:    "rss",
			XMLURL:  f.FeedURL,
			HTMLURL: f.SiteURL,
		})
	}
	b, _ := xml.MarshalIndent(doc, "", "  ")
	return xml.Header + string(b) + "\n"
}

func ParseOPML(data []byte) ([]OPMLEntry, error) {
	var doc opmlDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse OPML: %w", err)
	}
	var out []OPMLEntry
	seen := map[string]bool{}
	var walk func([]opmlOutline)
	walk = func(list []opmlOutline) {
		for _, o := range list {
			u := strings.TrimSpace(o.XMLURL)
			if u != "" && !seen[u] {
				if _, err := validateHTTPURL(u); err == nil {
					seen[u] = true
					title := strings.TrimSpace(o.Title)
					if title == "" {
						title = strings.TrimSpace(o.Text)
					}
					out = append(out, OPMLEntry{
						Title:   title,
						FeedURL: u,
						SiteURL: strings.TrimSpace(o.HTMLURL),
					})
				}
			}
			if len(o.Outlines) > 0 {
				walk(o.Outlines)
			}
		}
	}
	walk(doc.Body.Outlines)
	return out, nil
}
