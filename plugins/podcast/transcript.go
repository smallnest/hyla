package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultGoogleTranslateBase = "https://translate.googleapis.com"
	maxASRDirectBytes          = 24 << 20
	asrChunkSeconds            = 600
)

var (
	cueTagRe          = regexp.MustCompile(`<[^>]+>`)
	tsPrefixLineRe    = regexp.MustCompile(`^[\[\(]?(\d{1,2}:\d{2}(?::\d{2})?(?:[.,]\d{1,3})?)[\]\)]?\s*(.+)$`)
	googleBatchMarker = "HYLA_SEG_SEP_"
)

func ParseTranscriptAuto(data []byte, contentType, rawURL string) []TranscriptSegment {
	text := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	ct := strings.ToLower(contentType)
	lu := strings.ToLower(rawURL)

	if strings.Contains(ct, "json") || strings.HasSuffix(lu, ".json") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if segs, err := ParsePodcastJSONTranscript(data); err == nil && len(segs) > 0 {
			return segs
		}
	}
	if strings.Contains(ct, "vtt") || strings.HasSuffix(lu, ".vtt") || strings.HasPrefix(strings.ToUpper(trimmed), "WEBVTT") {
		if segs := ParseVTT(trimmed); len(segs) > 0 {
			return segs
		}
	}
	if strings.Contains(ct, "srt") || strings.HasSuffix(lu, ".srt") || strings.Contains(trimmed, "-->") {
		if segs := ParseSRT(trimmed); len(segs) > 0 {
			return segs
		}
		if segs := ParseVTT(trimmed); len(segs) > 0 {
			return segs
		}
	}
	return ParseTimestampedText(trimmed)
}

func ParseVTT(raw string) []TranscriptSegment {
	return parseCueFormat(raw, true)
}

func ParseSRT(raw string) []TranscriptSegment {
	return parseCueFormat(raw, false)
}

func parseCueFormat(raw string, isVTT bool) []TranscriptSegment {
	norm := strings.ReplaceAll(strings.TrimPrefix(raw, "\xef\xbb\xbf"), "\r\n", "\n")
	norm = strings.ReplaceAll(norm, "\r", "\n")
	blocks := strings.Split(norm, "\n\n")
	segs := make([]TranscriptSegment, 0, len(blocks))

	for _, blk := range blocks {
		lines := strings.Split(strings.TrimSpace(blk), "\n")
		if len(lines) == 0 {
			continue
		}
		first := strings.TrimSpace(lines[0])
		if isVTT && (strings.HasPrefix(strings.ToUpper(first), "WEBVTT") || strings.HasPrefix(first, "NOTE") || strings.HasPrefix(first, "STYLE") || strings.HasPrefix(first, "REGION")) {
			continue
		}
		arrowIdx := -1
		for i, l := range lines {
			if strings.Contains(l, "-->") {
				arrowIdx = i
				break
			}
		}
		if arrowIdx < 0 {
			continue
		}
		left, right, ok := strings.Cut(lines[arrowIdx], "-->")
		if !ok {
			continue
		}
		start := parseDurationSec(strings.Fields(left)[0])
		rightFields := strings.Fields(right)
		if len(rightFields) == 0 {
			continue
		}
		end := parseDurationSec(rightFields[0])
		if end < start {
			end = start
		}
		bodyLines := make([]string, 0, len(lines)-arrowIdx-1)
		for _, l := range lines[arrowIdx+1:] {
			cleaned := strings.TrimSpace(cueTagRe.ReplaceAllString(l, ""))
			if cleaned != "" {
				bodyLines = append(bodyLines, cleaned)
			}
		}
		text, trans := splitBilingualLines(bodyLines)
		if text == "" {
			continue
		}
		segs = append(segs, TranscriptSegment{
			ID:          len(segs) + 1,
			StartSec:    start,
			EndSec:      end,
			Text:        text,
			Translation: trans,
		})
	}
	return segs
}

func splitBilingualLines(bodyLines []string) (orig, trans string) {
	if len(bodyLines) == 0 {
		return "", ""
	}
	if len(bodyLines) == 2 {
		l0, l1 := bodyLines[0], bodyLines[1]
		cjk0, cjk1 := hasCJKRune(l0), hasCJKRune(l1)
		lat0, lat1 := hasLatinLetter(l0), hasLatinLetter(l1)
		if (cjk0 && !cjk1 && lat1) || (cjk1 && !cjk0 && lat0) {
			return l0, l1
		}
	}
	return strings.Join(bodyLines, " "), ""
}

func hasCJKRune(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}

func hasLatinLetter(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Latin, r) && unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func ParsePodcastJSONTranscript(raw []byte) ([]TranscriptSegment, error) {
	var doc struct {
		Segments []struct {
			StartTime float64 `json:"startTime"`
			EndTime   float64 `json:"endTime"`
			Start     float64 `json:"start"`
			End       float64 `json:"end"`
			Body      string  `json:"body"`
			Text      string  `json:"text"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(raw, &doc); err == nil && len(doc.Segments) > 0 {
		out := make([]TranscriptSegment, 0, len(doc.Segments))
		for _, s := range doc.Segments {
			txt := strings.TrimSpace(s.Body)
			if txt == "" {
				txt = strings.TrimSpace(s.Text)
			}
			if txt == "" {
				continue
			}
			st := s.StartTime
			if st == 0 && s.Start > 0 {
				st = s.Start
			}
			en := s.EndTime
			if en == 0 && s.End > 0 {
				en = s.End
			}
			if en < st {
				en = st
			}
			out = append(out, TranscriptSegment{
				ID:       len(out) + 1,
				StartSec: st,
				EndSec:   en,
				Text:     txt,
			})
		}
		return out, nil
	}

	var arr []struct {
		StartTime float64 `json:"startTime"`
		EndTime   float64 `json:"endTime"`
		Start     float64 `json:"start"`
		End       float64 `json:"end"`
		Body      string  `json:"body"`
		Text      string  `json:"text"`
	}
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, err
	}
	out := make([]TranscriptSegment, 0, len(arr))
	for _, s := range arr {
		txt := strings.TrimSpace(s.Body)
		if txt == "" {
			txt = strings.TrimSpace(s.Text)
		}
		if txt == "" {
			continue
		}
		st := s.StartTime
		if st == 0 && s.Start > 0 {
			st = s.Start
		}
		en := s.EndTime
		if en == 0 && s.End > 0 {
			en = s.End
		}
		out = append(out, TranscriptSegment{
			ID:       len(out) + 1,
			StartSec: st,
			EndSec:   en,
			Text:     txt,
		})
	}
	return out, nil
}

func ParsePodcastChaptersJSON(raw []byte) ([]Chapter, error) {
	var doc struct {
		Chapters []struct {
			StartTime float64 `json:"startTime"`
			Title     string  `json:"title"`
			URL       string  `json:"url"`
			Img       string  `json:"img"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	out := make([]Chapter, 0, len(doc.Chapters))
	for _, c := range doc.Chapters {
		title := strings.TrimSpace(c.Title)
		if title == "" {
			continue
		}
		out = append(out, Chapter{
			StartSec: c.StartTime,
			Title:    title,
			URL:      strings.TrimSpace(c.URL),
			Img:      strings.TrimSpace(c.Img),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].StartSec < out[j].StartSec
	})
	return out, nil
}

func ParseTimestampedText(raw string) []TranscriptSegment {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var out []TranscriptSegment
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := tsPrefixLineRe.FindStringSubmatch(line); len(m) == 3 {
			sec := parseDurationSec(m[1])
			txt := strings.TrimSpace(m[2])
			if txt == "" {
				continue
			}
			out = append(out, TranscriptSegment{
				ID:       len(out) + 1,
				StartSec: sec,
				Text:     txt,
			})
		} else {
			var start float64
			if len(out) > 0 {
				start = out[len(out)-1].StartSec + 8
			}
			out = append(out, TranscriptSegment{
				ID:       len(out) + 1,
				StartSec: start,
				Text:     line,
			})
		}
	}
	for i := range out {
		if out[i].EndSec <= out[i].StartSec {
			if i+1 < len(out) && out[i+1].StartSec > out[i].StartSec {
				out[i].EndSec = out[i+1].StartSec
			} else {
				out[i].EndSec = out[i].StartSec + 5
			}
		}
	}
	return out
}

func formatTimestampSRT(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	ms := int(sec*1000 + 0.5)
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	ms %= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func formatTimestampVTT(sec float64) string {
	return strings.ReplaceAll(formatTimestampSRT(sec), ",", ".")
}

func formatClockMMSS(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	total := int(sec)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func ExportTranscript(ep Episode, segs []TranscriptSegment, format, mode string) string {
	fmtName := strings.ToLower(strings.TrimSpace(format))
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		m = "bilingual"
	}

	lineFor := func(seg TranscriptSegment) string {
		orig := strings.TrimSpace(seg.Text)
		tr := strings.TrimSpace(seg.Translation)
		switch m {
		case "original":
			return orig
		case "translation":
			if tr != "" {
				return tr
			}
			return orig
		default:
			if tr != "" && tr != orig {
				return orig + "\n" + tr
			}
			return orig
		}
	}

	var b strings.Builder
	switch fmtName {
	case "vtt":
		b.WriteString("WEBVTT\n\n")
		for i, s := range segs {
			end := s.EndSec
			if end <= s.StartSec {
				end = s.StartSec + 4
			}
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, formatTimestampVTT(s.StartSec), formatTimestampVTT(end), lineFor(s))
		}
	case "md", "markdown":
		fmt.Fprintf(&b, "# %s\n\n", ep.Title)
		if ep.FeedTitle != "" {
			fmt.Fprintf(&b, "- Show: %s\n", ep.FeedTitle)
		}
		if ep.LinkURL != "" {
			fmt.Fprintf(&b, "- Link: %s\n", ep.LinkURL)
		}
		b.WriteString("\n")
		for _, s := range segs {
			ts := formatClockMMSS(s.StartSec)
			orig := strings.TrimSpace(s.Text)
			tr := strings.TrimSpace(s.Translation)
			switch m {
			case "original":
				fmt.Fprintf(&b, "- **[%s]** %s\n", ts, orig)
			case "translation":
				if tr == "" {
					tr = orig
				}
				fmt.Fprintf(&b, "- **[%s]** %s\n", ts, tr)
			default:
				if tr != "" && tr != orig {
					fmt.Fprintf(&b, "- **[%s]** %s\n  %s\n", ts, orig, tr)
				} else {
					fmt.Fprintf(&b, "- **[%s]** %s\n", ts, orig)
				}
			}
		}
	case "txt":
		for _, s := range segs {
			b.WriteString(lineFor(s))
			b.WriteString("\n\n")
		}
	default: // srt
		for i, s := range segs {
			end := s.EndSec
			if end <= s.StartSec {
				end = s.StartSec + 4
			}
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, formatTimestampSRT(s.StartSec), formatTimestampSRT(end), lineFor(s))
		}
	}
	return b.String()
}

func TranslateSegments(ctx context.Context, client *http.Client, cfg AIConfig, googleBase string, segs []TranscriptSegment, targetLang string, onProgress func(done, total int)) ([]TranscriptSegment, error) {
	return TranslateSegmentsProgressive(ctx, client, cfg, googleBase, segs, targetLang, false, func(done, total int, _ []TranscriptSegment) {
		if onProgress != nil {
			onProgress(done, total)
		}
	})
}

// TranslateSegmentsProgressive translates transcript segments in batches and emits incremental snapshots.
// When force is false and only some segments already have translations, only missing segments are translated.
func TranslateSegmentsProgressive(ctx context.Context, client *http.Client, cfg AIConfig, googleBase string, segs []TranscriptSegment, targetLang string, force bool, onProgress func(done, total int, partial []TranscriptSegment)) ([]TranscriptSegment, error) {
	if len(segs) == 0 {
		return segs, nil
	}
	if strings.TrimSpace(targetLang) == "" {
		targetLang = "zh-CN"
	}
	out := make([]TranscriptSegment, len(segs))
	copy(out, segs)

	existingCount := 0
	for _, s := range out {
		if strings.TrimSpace(s.Translation) != "" {
			existingCount++
		}
	}
	skipExisting := !force && existingCount > 0 && existingCount < len(out)
	pendingIdx := make([]int, 0, len(out))
	for i := range out {
		if !skipExisting || strings.TrimSpace(out[i].Translation) == "" {
			pendingIdx = append(pendingIdx, i)
		}
	}
	if len(pendingIdx) == 0 {
		if onProgress != nil {
			snap := make([]TranscriptSegment, len(out))
			copy(snap, out)
			onProgress(len(out), len(out), snap)
		}
		return out, nil
	}
	baseDone := len(out) - len(pendingIdx)

	if cfg.TranslateEngine == "ai" {
		if err := translateSegmentsAI(ctx, client, cfg, out, pendingIdx, baseDone, targetLang, onProgress); err != nil {
			return nil, err
		}
		return out, nil
	}

	if googleBase == "" {
		googleBase = defaultGoogleTranslateBase
	}
	const batchMaxCount = 12
	const batchMaxChars = 2200

	for i := 0; i < len(pendingIdx); {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		j := i
		chars := 0
		for j < len(pendingIdx) && j-i < batchMaxCount {
			l := len(out[pendingIdx[j]].Text)
			if j > i && chars+l > batchMaxChars {
				break
			}
			chars += l
			j++
		}
		batchTexts := make([]string, 0, j-i)
		for k := i; k < j; k++ {
			batchTexts = append(batchTexts, out[pendingIdx[k]].Text)
		}
		translated, err := googleTranslateBatch(ctx, client, googleBase, batchTexts, targetLang)
		if err != nil {
			return nil, err
		}
		for k := range translated {
			out[pendingIdx[i+k]].Translation = translated[k]
		}
		if onProgress != nil {
			snap := make([]TranscriptSegment, len(out))
			copy(snap, out)
			onProgress(baseDone+j, len(out), snap)
		}
		i = j
	}
	return out, nil
}

func googleTranslateBatch(ctx context.Context, client *http.Client, baseURL string, texts []string, targetLang string) ([]string, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if len(texts) == 1 {
		one, err := googleTranslateSingle(ctx, client, baseURL, texts[0], targetLang)
		if err != nil {
			return nil, err
		}
		return []string{one}, nil
	}
	sep := "\n" + googleBatchMarker + "\n"
	joined := strings.Join(texts, sep)
	res, err := googleTranslateSingle(ctx, client, baseURL, joined, targetLang)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(res, googleBatchMarker)
	if len(parts) == len(texts) {
		out := make([]string, len(parts))
		for i, p := range parts {
			out[i] = strings.TrimSpace(p)
		}
		return out, nil
	}
	// Fallback per-segment only when the server succeeded but altered batch delimiters.
	out := make([]string, len(texts))
	for i, txt := range texts {
		tr, serr := googleTranslateSingle(ctx, client, baseURL, txt, targetLang)
		if serr != nil {
			return nil, serr
		}
		out[i] = tr
	}
	return out, nil
}

func googleTranslateSingle(ctx context.Context, client *http.Client, baseURL, text, targetLang string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	u := fmt.Sprintf("%s/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&q=%s",
		strings.TrimRight(baseURL, "/"),
		url.QueryEscape(targetLang),
		url.QueryEscape(text),
	)
	b, _, err := fetchBytes(ctx, client, u, 2<<20)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 429") {
			return "", fmt.Errorf("Google Translate rate-limited (HTTP 429); switch Translation Engine to AI in Settings or retry later: %w", err)
		}
		return "", err
	}
	var raw []any
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", fmt.Errorf("decode Google Translate response: %w", err)
	}
	if len(raw) == 0 {
		return "", nil
	}
	sentences, ok := raw[0].([]any)
	if !ok {
		return "", nil
	}
	var out strings.Builder
	for _, item := range sentences {
		pair, ok := item.([]any)
		if !ok || len(pair) == 0 {
			continue
		}
		if s, ok := pair[0].(string); ok {
			out.WriteString(s)
		}
	}
	return strings.TrimSpace(out.String()), nil
}

func translateSegmentsAI(ctx context.Context, client *http.Client, cfg AIConfig, out []TranscriptSegment, pendingIdx []int, baseDone int, targetLang string, onProgress func(done, total int, partial []TranscriptSegment)) error {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("AI provider base URL or model is not configured")
	}
	const batchSize = 20
	for i := 0; i < len(pendingIdx); i += batchSize {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		end := min(len(pendingIdx), i+batchSize)
		type inItem struct {
			I int    `json:"i"`
			T string `json:"t"`
		}
		items := make([]inItem, 0, end-i)
		validIdx := make(map[int]bool, end-i)
		for k := i; k < end; k++ {
			segIdx := pendingIdx[k]
			validIdx[segIdx] = true
			items = append(items, inItem{I: segIdx, T: out[segIdx].Text})
		}
		inJSON, _ := json.Marshal(items)
		sysPrompt := fmt.Sprintf("You are a concise subtitle translator. Translate each item's `t` field naturally into %s. Return ONLY a JSON object {\"items\":[{\"i\":number,\"t\":\"translation\"}]} preserving every index `i`.", targetLang)
		respText, err := chatCompletion(ctx, client, cfg, []aiMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: string(inJSON)},
		}, true)
		if err != nil {
			return err
		}
		cleaned := stripJSONFences(respText)
		var outWrap struct {
			Items []inItem `json:"items"`
		}
		if err := json.Unmarshal([]byte(cleaned), &outWrap); err != nil {
			var arr []inItem
			if err2 := json.Unmarshal([]byte(cleaned), &arr); err2 != nil {
				return fmt.Errorf("parse AI translation JSON: %w", err)
			}
			outWrap.Items = arr
		}
		for _, it := range outWrap.Items {
			if validIdx[it.I] {
				out[it.I].Translation = strings.TrimSpace(it.T)
			}
		}
		if onProgress != nil {
			snap := make([]TranscriptSegment, len(out))
			copy(snap, out)
			onProgress(baseDone+end, len(out), snap)
		}
	}
	return nil
}

func TranscribeWithCloudASR(ctx context.Context, client *http.Client, cfg AIConfig, audioPath string, onProgress func(pct int, msg string)) ([]TranscriptSegment, error) {
	baseURL := strings.TrimSpace(cfg.ASRBaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(cfg.BaseURL)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("Cloud ASR Base URL is not configured (set e.g. https://api.groq.com/openai/v1 in Settings)")
	}
	apiKey := strings.TrimSpace(cfg.ASRAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(cfg.APIKey)
	}
	model := strings.TrimSpace(cfg.ASRModel)
	if model == "" {
		model = "whisper-large-v3-turbo"
	}

	fi, err := os.Stat(audioPath)
	if err != nil {
		return nil, err
	}
	if fi.Size() <= maxASRDirectBytes {
		if onProgress != nil {
			onProgress(35, "Uploading audio to Cloud ASR…")
		}
		return callWhisperEndpoint(ctx, client, baseURL, apiKey, model, audioPath, 0)
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("audio file is %.1f MiB (>24 MiB) and ffmpeg was not found on PATH for chunking; install ffmpeg or import an SRT/VTT transcript", float64(fi.Size())/(1024*1024))
	}
	tmpDir, err := os.MkdirTemp("", "hyla-podcast-asr-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if onProgress != nil {
		onProgress(15, "Splitting audio into chunks…")
	}
	pattern := filepath.Join(tmpDir, "chunk_%03d.mp3")
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-y", "-i", audioPath,
		"-f", "segment", "-segment_time", strconv.Itoa(asrChunkSeconds),
		"-ar", "16000", "-ac", "1", "-b:a", "48k",
		pattern,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg split failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	chunks, err := filepath.Glob(filepath.Join(tmpDir, "chunk_*.mp3"))
	if err != nil || len(chunks) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no audio chunks")
	}
	sort.Strings(chunks)

	var all []TranscriptSegment
	for idx, ch := range chunks {
		if onProgress != nil {
			pct := 20 + (idx*75)/len(chunks)
			onProgress(pct, fmt.Sprintf("Transcribing chunk %d/%d…", idx+1, len(chunks)))
		}
		offset := float64(idx * asrChunkSeconds)
		segs, err := callWhisperEndpoint(ctx, client, baseURL, apiKey, model, ch, offset)
		if err != nil {
			return nil, fmt.Errorf("chunk %d/%d: %w", idx+1, len(chunks), err)
		}
		for _, s := range segs {
			s.ID = len(all) + 1
			all = append(all, s)
		}
	}
	return all, nil
}

func callWhisperEndpoint(ctx context.Context, client *http.Client, baseURL, apiKey, model, filePath string, offsetSec float64) ([]TranscriptSegment, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}
	_ = mw.WriteField("model", model)
	_ = mw.WriteField("response_format", "verbose_json")
	_ = mw.WriteField("timestamp_granularities[]", "segment")
	if err := mw.Close(); err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/audio/transcriptions"
	req, err := newHTTPRequest(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ASR API HTTP %s: %s", resp.Status, truncateRunes(string(respBytes), 300))
	}

	var parsed struct {
		Text     string `json:"text"`
		Segments []struct {
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Text  string  `json:"text"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode ASR response: %w", err)
	}
	if len(parsed.Segments) == 0 && strings.TrimSpace(parsed.Text) != "" {
		return []TranscriptSegment{{
			ID:       1,
			StartSec: offsetSec,
			EndSec:   offsetSec + 10,
			Text:     strings.TrimSpace(parsed.Text),
		}}, nil
	}
	out := make([]TranscriptSegment, 0, len(parsed.Segments))
	for _, s := range parsed.Segments {
		txt := strings.TrimSpace(s.Text)
		if txt == "" {
			continue
		}
		out = append(out, TranscriptSegment{
			ID:       len(out) + 1,
			StartSec: s.Start + offsetSec,
			EndSec:   s.End + offsetSec,
			Text:     txt,
		})
	}
	return out, nil
}
