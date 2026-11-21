# Podcast Plugin (`hyla.podcast`)

Standalone sandboxed WebView pane plugin for Hyla inspired by [SubCast](https://subcast.qiaomu.ai/).

## Features

- **Universal Subscriptions**: Subscribe via RSS/Atom URL, Apple Podcasts link, Xiaoyuzhou FM link, website discovery, or direct audio URL, plus global Apple Podcasts search and OPML import/export.
- **Curated Discovery**: Built-in curated catalog across AI, Engineering, Business, and Chinese podcasts.
- **Bilingual Subtitles & Export**: Fetches Podcasting 2.0 VTT/SRT/JSON transcripts, imports local transcripts, runs Cloud ASR (`whisper-large-v3-turbo` / OpenAI-compatible `/audio/transcriptions`), translates via Google Translate or your configured AI model, and exports SRT, VTT, Markdown, and TXT.
- **AI Guide & Q&A**: Generates structured episode summaries, takeaways, clickable `[MM:SS]` timelines, and notable quotes, plus multi-turn Q&A over the transcript with timestamp jump links.
- **English Word Lookup & Vocabulary Notebook**: Click any English word in subtitles for dictionary phonetics/definitions and contextual translation, save with source sentence & timestamp, review with flashcards, and export CSV.
- **Listening Stats & Offline Cache**: Tracks listening time, top shows, and 24-hour activity; caches episodes offline for range-seekable playback.
- **Agent Auto-Pause**: Automatically pauses podcast playback when a coding agent in Hyla transitions from `working` to `idle` / `waiting` / `failed`.

## Build & Link for Development

```sh
cd plugins/podcast
mkdir -p bin
go build -o bin/hyla-podcast .
cd ../..
hyla plugin link ./plugins/podcast --grant-all
```

## Package & Install (`.hylapkg`)

```sh
cd plugins/podcast
mkdir -p bin
go build -o bin/hyla-podcast .
cd ../..
hyla plugin pack ./plugins/podcast -o hyla-podcast.hylapkg
hyla plugin install ./hyla-podcast.hylapkg --grant-all
```

Once enabled, open the pane via **Pane › Open Podcast Pane** (`⌥⌘P` on macOS, `Ctrl+Alt+P` on Windows/Linux) or from the Command Palette (`⌘P` / `Ctrl+Shift+P`).

## Settings

Configure in **Settings › Plugins › Podcast** or via the in-pane Settings drawer:

- `auto_pause_on_agent` — Pause playback when a coding agent finishes working (default `true`).
- `ai_provider` / `ai_base_url` / `ai_model` / `ai_api_key` — OpenAI-compatible model for AI guide, Q&A, and optional translation (`deepseek`, `openai`, `openrouter`, `siliconflow`, `kimi`, `ollama`, `custom`). Secrets are stored in the OS keychain.
- `translate_engine` — `google` (default, no key required) or `ai`.
- `asr_base_url` / `asr_model` / `asr_api_key` — Optional Cloud ASR endpoint (e.g. Groq or SiliconFlow) for episodes without transcripts.
- `http_proxy` — Optional HTTP/HTTPS proxy URL (falls back to environment & macOS system proxy).
