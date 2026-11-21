package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultDictAPIBase = "https://api.dictionaryapi.dev/api/v2/entries/en"
	maxTranscriptChars = 70000
)

type AIConfig struct {
	AutoPauseOnAgent bool   `json:"autoPauseOnAgent"`
	Provider         string `json:"aiProvider"`
	BaseURL          string `json:"aiBaseUrl"`
	Model            string `json:"aiModel"`
	APIKey           string `json:"-"`
	HasAIKey         bool   `json:"hasAiKey"`
	TranslateEngine  string `json:"translateEngine"`
	ASRBaseURL       string `json:"asrBaseUrl"`
	ASRModel         string `json:"asrModel"`
	ASRAPIKey        string `json:"-"`
	HasASRKey        bool   `json:"hasAsrKey"`
	HTTPProxy        string `json:"httpProxy"`
}

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type WordLookupResult struct {
	Word        string   `json:"word"`
	Phonetic    string   `json:"phonetic,omitempty"`
	AudioURL    string   `json:"audioUrl,omitempty"`
	PartOfSpeech string  `json:"partOfSpeech,omitempty"`
	Definitions []string `json:"definitions,omitempty"`
	Meaning     string   `json:"meaning,omitempty"`
	ContextNote string   `json:"contextNote,omitempty"`
}

func providerPreset(provider string) (baseURL, model string) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "https://api.openai.com/v1", "gpt-4o-mini"
	case "openrouter":
		return "https://openrouter.ai/api/v1", "deepseek/deepseek-chat"
	case "siliconflow":
		return "https://api.siliconflow.cn/v1", "Qwen/Qwen2.5-72B-Instruct"
	case "kimi":
		return "https://api.moonshot.cn/v1", "moonshot-v1-8k"
	case "ollama":
		return "http://127.0.0.1:11434/v1", "qwen2.5:7b"
	case "custom":
		return "", ""
	default: // deepseek
		return "https://api.deepseek.com/v1", "deepseek-chat"
	}
}

func buildAIConfig(settings map[string]any, secrets map[string]string, secretsSet map[string]bool) AIConfig {
	strVal := func(k, def string) string {
		if settings != nil {
			if v, ok := settings[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
		return def
	}
	boolVal := func(k string, def bool) bool {
		if settings != nil {
			if v, ok := settings[k].(bool); ok {
				return v
			}
		}
		return def
	}

	provider := strVal("ai_provider", "deepseek")
	defURL, defModel := providerPreset(provider)

	baseURL := strVal("ai_base_url", defURL)
	model := strVal("ai_model", defModel)
	aiKey := ""
	if secrets != nil {
		aiKey = strings.TrimSpace(secrets["ai_api_key"])
	}
	asrKey := ""
	if secrets != nil {
		asrKey = strings.TrimSpace(secrets["asr_api_key"])
	}

	return AIConfig{
		AutoPauseOnAgent: boolVal("auto_pause_on_agent", true),
		Provider:         provider,
		BaseURL:          baseURL,
		Model:            model,
		APIKey:           aiKey,
		HasAIKey:         aiKey != "" || (secretsSet != nil && secretsSet["ai_api_key"]),
		TranslateEngine:  strVal("translate_engine", "google"),
		ASRBaseURL:       strVal("asr_base_url", ""),
		ASRModel:         strVal("asr_model", "whisper-large-v3-turbo"),
		ASRAPIKey:        asrKey,
		HasASRKey:        asrKey != "" || (secretsSet != nil && secretsSet["asr_api_key"]),
		HTTPProxy:        strVal("http_proxy", ""),
	}
}

func chatCompletion(ctx context.Context, client *http.Client, cfg AIConfig, messages []aiMessage, jsonMode bool) (string, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return "", fmt.Errorf("AI base URL is empty")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return "", fmt.Errorf("AI model is empty")
	}
	if strings.TrimSpace(cfg.APIKey) == "" && !strings.EqualFold(cfg.Provider, "ollama") && !isLocalEndpoint(cfg.BaseURL) {
		return "", fmt.Errorf("AI API Key is not configured (set it in Podcast Settings or Settings › Plugins)")
	}

	payload := map[string]any{
		"model":    cfg.Model,
		"messages": messages,
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, _ := json.Marshal(payload)

	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := newHTTPRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI API HTTP %s: %s", resp.Status, truncateRunes(string(respBytes), 400))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &out); err != nil {
		return "", fmt.Errorf("decode AI response: %w", err)
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("AI API error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("AI response contained no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func isLocalEndpoint(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return isLocalHost(u.Hostname())
}

func stripJSONFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "```") {
		if idx := strings.IndexByte(trimmed, '\n'); idx >= 0 {
			trimmed = trimmed[idx+1:]
		}
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	}
	return strings.TrimSpace(trimmed)
}

func formatTranscriptContext(ep Episode, segs []TranscriptSegment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\n", ep.Title)
	if ep.FeedTitle != "" {
		fmt.Fprintf(&b, "Show: %s\n", ep.FeedTitle)
	}
	if len(segs) == 0 {
		if ep.Description != "" {
			b.WriteString("\nShow Notes:\n")
			b.WriteString(truncateRunes(ep.Description, 24000))
		}
		return b.String()
	}
	b.WriteString("\nTranscript:\n")
	for _, s := range segs {
		line := fmt.Sprintf("[%s] %s\n", formatClockMMSS(s.StartSec), strings.TrimSpace(s.Text))
		if b.Len()+len(line) > maxTranscriptChars {
			b.WriteString("...[truncated]\n")
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

func responseLanguageInstruction(locale string) string {
	switch normalizeLocale(locale) {
	case "zh-Hans":
		return "Simplified Chinese (简体中文)"
	case "ja":
		return "Japanese (日本語)"
	default:
		return "English"
	}
}

func GenerateGuide(ctx context.Context, client *http.Client, cfg AIConfig, ep Episode, segs []TranscriptSegment, locale string) (*AIGuide, error) {
	lang := responseLanguageInstruction(locale)
	sysPrompt := fmt.Sprintf(`You are an expert podcast reading guide editor. Write the entire output in %s.
Return ONLY valid JSON with this exact schema:
{
  "oneLiner": "one concise sentence capturing the core thesis",
  "takeaways": ["3 to 6 concrete takeaways"],
  "timeline": [{"time": "MM:SS", "title": "section title", "summary": "brief summary"}],
  "quotes": [{"time": "MM:SS", "text": "notable quote from transcript", "note": "why it matters"}]
}`, lang)

	userContent := formatTranscriptContext(ep, segs)
	raw, err := chatCompletion(ctx, client, cfg, []aiMessage{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userContent},
	}, true)
	if err != nil {
		return nil, err
	}

	cleaned := stripJSONFences(raw)
	var parsed struct {
		OneLiner  string   `json:"oneLiner"`
		Takeaways []string `json:"takeaways"`
		Timeline  []struct {
			Time    string  `json:"time"`
			TimeSec float64 `json:"timeSec"`
			Title   string  `json:"title"`
			Summary string  `json:"summary"`
		} `json:"timeline"`
		Quotes []struct {
			Time    string  `json:"time"`
			TimeSec float64 `json:"timeSec"`
			Text    string  `json:"text"`
			Note    string  `json:"note"`
		} `json:"quotes"`
	}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("parse AI guide JSON: %w", err)
	}

	guide := &AIGuide{
		OneLiner:    strings.TrimSpace(parsed.OneLiner),
		Takeaways:   parsed.Takeaways,
		Model:       cfg.Model,
		GeneratedAt: time.Now().UnixMilli(),
	}
	for _, tl := range parsed.Timeline {
		sec := tl.TimeSec
		if sec == 0 && tl.Time != "" {
			sec = parseDurationSec(tl.Time)
		}
		if strings.TrimSpace(tl.Title) == "" {
			continue
		}
		guide.Timeline = append(guide.Timeline, GuideTimelineItem{
			TimeSec: sec,
			Title:   strings.TrimSpace(tl.Title),
			Summary: strings.TrimSpace(tl.Summary),
		})
	}
	for _, q := range parsed.Quotes {
		sec := q.TimeSec
		if sec == 0 && q.Time != "" {
			sec = parseDurationSec(q.Time)
		}
		if strings.TrimSpace(q.Text) == "" {
			continue
		}
		guide.Quotes = append(guide.Quotes, GuideQuote{
			TimeSec: sec,
			Text:    strings.TrimSpace(q.Text),
			Note:    strings.TrimSpace(q.Note),
		})
	}
	return guide, nil
}

func AskEpisode(ctx context.Context, client *http.Client, cfg AIConfig, ep Episode, segs []TranscriptSegment, history []ChatMessage, question, locale string) (string, error) {
	lang := responseLanguageInstruction(locale)
	sysPrompt := fmt.Sprintf(`You are an AI podcast assistant helping the listener understand an episode. Answer in %s.
Ground answers in the episode content provided below. Whenever referencing a specific point or quote, cite timestamps in [MM:SS] or [HH:MM:SS] format so the user can click to jump there.

%s`, lang, formatTranscriptContext(ep, segs))

	msgs := []aiMessage{{Role: "system", Content: sysPrompt}}
	start := 0
	if len(history) > 10 {
		start = len(history) - 10
	}
	for _, m := range history[start:] {
		if m.Role == "user" || m.Role == "assistant" {
			msgs = append(msgs, aiMessage{Role: m.Role, Content: m.Content})
		}
	}
	msgs = append(msgs, aiMessage{Role: "user", Content: strings.TrimSpace(question)})
	return chatCompletion(ctx, client, cfg, msgs, false)
}

func LookupWord(ctx context.Context, client *http.Client, cfg AIConfig, dictBase, googleBase, word, sentence, targetLang string) (*WordLookupResult, error) {
	clean := strings.Trim(strings.TrimSpace(word), ".,!?;:\"'()[]{}“”‘’—–")
	if clean == "" {
		return nil, fmt.Errorf("word is empty")
	}
	if dictBase == "" {
		dictBase = defaultDictAPIBase
	}
	if googleBase == "" {
		googleBase = defaultGoogleTranslateBase
	}
	if targetLang == "" {
		targetLang = "zh-CN"
	}

	res := &WordLookupResult{Word: clean}

	dictURL := strings.TrimRight(dictBase, "/") + "/" + url.PathEscape(strings.ToLower(clean))
	if b, _, err := fetchBytes(ctx, client, dictURL, 1<<20); err == nil {
		var entries []struct {
			Phonetic  string `json:"phonetic"`
			Phonetics []struct {
				Text  string `json:"text"`
				Audio string `json:"audio"`
			} `json:"phonetics"`
			Meanings []struct {
				PartOfSpeech string `json:"partOfSpeech"`
				Definitions  []struct {
					Definition string `json:"definition"`
				} `json:"definitions"`
			} `json:"meanings"`
		}
		if json.Unmarshal(b, &entries) == nil && len(entries) > 0 {
			ent := entries[0]
			res.Phonetic = ent.Phonetic
			for _, ph := range ent.Phonetics {
				if res.Phonetic == "" && ph.Text != "" {
					res.Phonetic = ph.Text
				}
				if res.AudioURL == "" && ph.Audio != "" {
					res.AudioURL = ph.Audio
				}
			}
			for _, mn := range ent.Meanings {
				if res.PartOfSpeech == "" && mn.PartOfSpeech != "" {
					res.PartOfSpeech = mn.PartOfSpeech
				}
				for _, df := range mn.Definitions {
					if df.Definition != "" && len(res.Definitions) < 3 {
						res.Definitions = append(res.Definitions, df.Definition)
					}
				}
			}
		}
	}

	if gloss, err := googleTranslateSingle(ctx, client, googleBase, clean, targetLang); err == nil && gloss != "" {
		res.Meaning = gloss
	}

	if strings.TrimSpace(sentence) != "" && (cfg.APIKey != "" || strings.EqualFold(cfg.Provider, "ollama") || isLocalEndpoint(cfg.BaseURL)) {
		prompt := fmt.Sprintf("Explain the word %q as used in this sentence: %q. Give a concise %s meaning in 1-2 sentences.", clean, sentence, responseLanguageInstruction(targetLang))
		if note, err := chatCompletion(ctx, client, cfg, []aiMessage{{Role: "user", Content: prompt}}, false); err == nil {
			res.ContextNote = note
		}
	}

	if res.Meaning == "" && len(res.Definitions) > 0 {
		res.Meaning = res.Definitions[0]
	}
	return res, nil
}
