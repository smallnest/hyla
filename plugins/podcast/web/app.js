(() => {
  const I18N = {
    en: {
      title: "Podcast",
      home: "Home",
      discover: "Discover",
      library: "Library",
      vocabStats: "Vocab & Stats",
      refresh: "Refresh",
      settings: "Settings",
      subscribePlaceholder: "Paste RSS, Apple Podcasts, Xiaoyuzhou, or MP3 URL…",
      subscribe: "Subscribe",
      searchPlaceholder: "Search global podcasts (Apple Podcasts)…",
      search: "Search",
      favoritesOnly: "Favorites",
      allEpisodes: "All",
      emptyHome: "No episodes yet. Subscribe to a podcast from Discover or paste a feed URL above.",
      play: "Play",
      pause: "Pause",
      resume: "Resume",
      queueAdd: "+ Queue",
      queued: "Queued",
      favorite: "Star",
      favorited: "Starred",
      download: "Download",
      offline: "Offline",
      deleteOffline: "Remove Download",
      back: "← Back",
      openLink: "Open Source ↗",
      subtitles: "Subtitles",
      aiGuide: "AI Guide",
      aiQA: "AI Q&A",
      showNotes: "Show Notes",
      bilingual: "Bilingual",
      translationOnly: "Translated",
      originalOnly: "Original",
      translate: "Translate",
      transcribe: "Cloud ASR",
      importSub: "Import SRT/VTT",
      exportSub: "Export",
      searchTranscript: "Search transcript…",
      autoScroll: "Auto-scroll",
      noSubtitles: "No subtitles yet. Try Cloud ASR, paste an SRT/VTT transcript, or pick a Podcasting 2.0 show.",
      generateGuide: "Generate AI Guide",
      regenerateGuide: "Regenerate",
      oneLiner: "Summary",
      takeaways: "Key Takeaways",
      timeline: "Timeline",
      quotes: "Notable Quotes",
      noGuide: "Click Generate AI Guide to summarize this episode with timestamps and quotes.",
      askPlaceholder: "Ask anything about this episode…",
      send: "Ask",
      clearChat: "Clear",
      starter1: "Summarize the 3 biggest insights",
      starter2: "What disagreements or surprises came up?",
      starter3: "List actionable recommendations with timestamps",
      chapters: "Chapters",
      notes: "Show Notes",
      catAll: "All",
      catAI: "AI & Frontier",
      catTech: "Tech",
      catEng: "Engineering",
      catBusiness: "Business",
      catZH: "Chinese",
      catEnglish: "English Learning",
      subscribed: "Subscribed",
      unsubscribe: "Unsubscribe",
      playAll: "Play All",
      shows: "Subscribed Shows",
      storage: "Offline Storage",
      clearCache: "Clear Downloads & Covers",
      exportOPML: "Export OPML",
      importOPML: "Import OPML",
      totalTime: "Total Listening",
      completedEps: "Completed",
      savedWords: "Saved Words",
      hourlyChart: "24h Listening Clock",
      topShows: "Top Shows",
      vocabNotebook: "Vocabulary Notebook",
      reviewCards: "Flashcard Review",
      exportCSV: "Export CSV",
      reveal: "Reveal Meaning",
      markReviewed: "Got It",
      nextCard: "Next",
      close: "Close",
      save: "Save",
      copy: "Copy",
      copied: "Copied!",
      saveVocab: "Save to Vocab",
      savedVocab: "Saved!",
      wordCardTitle: "Word Lookup",
      pronounce: "Listen",
      autoPaused: "Paused automatically because coding agent finished",
      dismiss: "Dismiss",
      copyQuote: "Copy Quote",
      settingsTitle: "Podcast Settings",
      autoPauseLabel: "Auto-pause when coding agent finishes",
      aiProviderLabel: "AI Provider",
      aiBaseUrlLabel: "AI Base URL (optional)",
      aiModelLabel: "AI Model (optional)",
      aiKeyLabel: "AI API Key (saved in Keychain)",
      translateEngineLabel: "Translation Engine",
      asrBaseUrlLabel: "Cloud ASR Base URL",
      asrModelLabel: "Cloud ASR Model",
      asrKeyLabel: "Cloud ASR API Key (saved in Keychain)",
      proxyLabel: "HTTP Proxy (optional)",
      keySaved: "Key saved in Keychain",
      pasteOPML: "Paste OPML XML below:",
      pasteTranscript: "Paste SRT, VTT, or timestamped text below:",
      importBtn: "Import",
      regionAuto: "Auto",
      noResults: "No podcasts matched your query.",
      emptyVocab: "Click any English word in Subtitles to look it up and save it here.",
      allShows: "All Shows",
      playbackErrorPrefix: "Playback error: ",
      cspMediaBlocked: "Audio playback was blocked by the running Hyla app's WebView CSP. Please rebuild/update and restart Hyla so network plugins can load local media.",
      buffering: "Buffering…",
      playingState: "Playing",
      liveCaption: "Live Subtitles",
      untranslatedBadge: "Untranslated",
      untranslatedHint: "No translations yet. Click Translate to show bilingual subtitles.",
      partialTranslated: "Translated",
      continueTranslate: "Continue Translating",
      translatingNow: "Translating subtitles…",
      mute: "Mute",
      unmute: "Unmute",
      openSettings: "Settings"
    },
    "zh-Hans": {
      title: "播客",
      home: "首页",
      discover: "发现",
      library: "订阅库",
      vocabStats: "生词与统计",
      refresh: "刷新",
      settings: "设置",
      subscribePlaceholder: "粘贴 RSS、Apple Podcasts、小宇宙或音频链接…",
      subscribe: "订阅",
      searchPlaceholder: "搜索全球播客（Apple Podcasts 目录）…",
      search: "搜索",
      favoritesOnly: "只看收藏",
      allEpisodes: "全部",
      emptyHome: "还没有单集。去「发现」页一键订阅精选播客，或在上方粘贴订阅地址。",
      play: "播放",
      pause: "暂停",
      resume: "继续播放",
      queueAdd: "+ 队列",
      queued: "已在队列",
      favorite: "收藏",
      favorited: "已收藏",
      download: "离线下载",
      offline: "已离线",
      deleteOffline: "删除离线",
      back: "← 返回",
      openLink: "原网页 ↗",
      subtitles: "双语字幕",
      aiGuide: "AI 导读",
      aiQA: "AI 问答",
      showNotes: "节目简介",
      bilingual: "双语",
      translationOnly: "译文",
      originalOnly: "原文",
      translate: "翻译字幕",
      transcribe: "云端识别",
      importSub: "导入字幕",
      exportSub: "导出",
      searchTranscript: "搜索文稿…",
      autoScroll: "跟随滚动",
      noSubtitles: "暂无字幕。可点击「云端识别」、导入 SRT/VTT 字幕，或播放支持 Podcasting 2.0 字幕的节目。",
      generateGuide: "生成 AI 导读",
      regenerateGuide: "重新生成",
      oneLiner: "一句话总结",
      takeaways: "核心观点",
      timeline: "时间线",
      quotes: "原话摘录",
      noGuide: "点击「生成 AI 导读」，提炼核心观点、时间线与原话引用。",
      askPlaceholder: "基于本集内容提问…",
      send: "提问",
      clearChat: "清空",
      starter1: "总结这期最重要的 3 个洞见",
      starter2: "嘉宾之间有哪些分歧或反常识观点？",
      starter3: "列出可落地的建议并附带时间戳",
      chapters: "章节",
      notes: "单集简介",
      catAll: "全部",
      catAI: "AI 前沿",
      catTech: "科技评论",
      catEng: "工程技术",
      catBusiness: "商业创业",
      catZH: "中文播客",
      catEnglish: "英语学习",
      subscribed: "已订阅",
      unsubscribe: "取消订阅",
      playAll: "播放全部",
      shows: "已订阅节目",
      storage: "离线存储",
      clearCache: "清理下载与封面缓存",
      exportOPML: "导出 OPML",
      importOPML: "导入 OPML",
      totalTime: "累计收听",
      completedEps: "已听完",
      savedWords: "生词本",
      hourlyChart: "24 小时收听生物钟",
      topShows: "常听节目",
      vocabNotebook: "生词本",
      reviewCards: "卡片复习",
      exportCSV: "导出 CSV",
      reveal: "查看释义",
      markReviewed: "记住了",
      nextCard: "下一个",
      close: "关闭",
      save: "保存",
      copy: "复制",
      copied: "已复制！",
      saveVocab: "加入生词本",
      savedVocab: "已加入！",
      wordCardTitle: "查词卡片",
      pronounce: "发音",
      autoPaused: "检测到编程 Agent 已完成工作，已自动暂停播客",
      dismiss: "忽略",
      copyQuote: "复制金句",
      settingsTitle: "播客设置",
      autoPauseLabel: "编程 Agent 完成时自动暂停播客",
      aiProviderLabel: "AI 服务商",
      aiBaseUrlLabel: "AI 接口地址（可选）",
      aiModelLabel: "AI 模型（可选）",
      aiKeyLabel: "AI API Key（保存在系统钥匙串）",
      translateEngineLabel: "字幕翻译引擎",
      asrBaseUrlLabel: "云端语音识别地址",
      asrModelLabel: "云端语音识别模型",
      asrKeyLabel: "云端识别 Key（保存在系统钥匙串）",
      proxyLabel: "HTTP 代理（可选）",
      keySaved: "钥匙串已配置 Key",
      pasteOPML: "粘贴 OPML XML 内容：",
      pasteTranscript: "粘贴 SRT、VTT 或带时间戳文稿：",
      importBtn: "导入",
      regionAuto: "自动",
      noResults: "未找到匹配的播客。",
      emptyVocab: "在「双语字幕」中点击任意英文单词即可查词并存进生词本。",
      allShows: "全部节目",
      playbackErrorPrefix: "播放出错：",
      cspMediaBlocked: "当前运行的 Hyla 客户端 WebView CSP 拦截了本地音频流。请重新构建/更新并重启 Hyla 后再试。",
      buffering: "缓冲中…",
      playingState: "播放中",
      liveCaption: "实时双语字幕",
      untranslatedBadge: "待翻译",
      untranslatedHint: "当前为原文字幕，点击「翻译字幕」即可显示双语对照。",
      partialTranslated: "已翻译",
      continueTranslate: "继续翻译",
      translatingNow: "正在翻译字幕…",
      mute: "静音",
      unmute: "取消静音",
      openSettings: "去设置"
    },
    ja: {
      title: "ポッドキャスト",
      home: "ホーム",
      discover: "発見",
      library: "ライブラリ",
      vocabStats: "単語・統計",
      refresh: "更新",
      settings: "設定",
      subscribePlaceholder: "RSS、Apple Podcasts、小宇宙、音声 URL を貼り付け…",
      subscribe: "購読",
      searchPlaceholder: "世界のポッドキャストを検索 (Apple Podcasts)…",
      search: "検索",
      favoritesOnly: "お気に入り",
      allEpisodes: "すべて",
      emptyHome: "エピソードがありません。「発見」タブから購読するか URL を貼り付けてください。",
      play: "再生",
      pause: "一時停止",
      resume: "再開",
      queueAdd: "+ キュー",
      queued: "キュー済み",
      favorite: "お気に入り",
      favorited: "登録済み",
      download: "保存",
      offline: "オフライン",
      deleteOffline: "削除",
      back: "← 戻る",
      openLink: "元ページ ↗",
      subtitles: "字幕",
      aiGuide: "AI ガイド",
      aiQA: "AI Q&A",
      showNotes: "概要",
      bilingual: "二言語",
      translationOnly: "翻訳",
      originalOnly: "原文",
      translate: "翻訳",
      transcribe: "クラウド文字起こし",
      importSub: "字幕取込",
      exportSub: "書き出し",
      searchTranscript: "文字起こしを検索…",
      autoScroll: "自動追従",
      noSubtitles: "字幕がありません。クラウド文字起こしまたは SRT/VTT 取り込みを利用してください。",
      generateGuide: "AI ガイド生成",
      regenerateGuide: "再生成",
      oneLiner: "要約",
      takeaways: "重要ポイント",
      timeline: "タイムライン",
      quotes: "注目フレーズ",
      noGuide: "「AI ガイド生成」で要約・タイムライン・引用を作成します。",
      askPlaceholder: "この回について質問する…",
      send: "質問",
      clearChat: "クリア",
      starter1: "最も重要な3つの洞察を要約して",
      starter2: "議論が分かれた点や意外な発見は？",
      starter3: "実践できるポイントをタイムスタンプ付きで",
      chapters: "チャプター",
      notes: "エピソード概要",
      catAll: "すべて",
      catAI: "AI・先端",
      catTech: "テック",
      catEng: "開発",
      catBusiness: "ビジネス",
      catZH: "中国語",
      catEnglish: "英語学習",
      subscribed: "購読中",
      unsubscribe: "解除",
      playAll: "すべて再生",
      shows: "購読中の番組",
      storage: "オフライン容量",
      clearCache: "キャッシュ削除",
      exportOPML: "OPML 書き出し",
      importOPML: "OPML 取り込み",
      totalTime: "総再生時間",
      completedEps: "再生完了",
      savedWords: "単語帳",
      hourlyChart: "24時間再生リズム",
      topShows: "よく聴く番組",
      vocabNotebook: "単語帳",
      reviewCards: "カード復習",
      exportCSV: "CSV 書き出し",
      reveal: "意味を表示",
      markReviewed: "覚えた",
      nextCard: "次へ",
      close: "閉じる",
      save: "保存",
      copy: "コピー",
      copied: "コピーしました",
      saveVocab: "単語帳に保存",
      savedVocab: "保存済み",
      wordCardTitle: "単語辞書",
      pronounce: "発音",
      autoPaused: "コーディングエージェント完了のため自動一時停止しました",
      dismiss: "閉じる",
      copyQuote: "引用コピー",
      settingsTitle: "ポッドキャスト設定",
      autoPauseLabel: "エージェント完了時に自動一時停止",
      aiProviderLabel: "AI プロバイダー",
      aiBaseUrlLabel: "AI ベース URL（任意）",
      aiModelLabel: "AI モデル（任意）",
      aiKeyLabel: "AI API Key（キーチェーン保存）",
      translateEngineLabel: "翻訳エンジン",
      asrBaseUrlLabel: "クラウド ASR ベース URL",
      asrModelLabel: "クラウド ASR モデル",
      asrKeyLabel: "クラウド ASR Key（キーチェーン保存）",
      proxyLabel: "HTTP プロキシ（任意）",
      keySaved: "キーチェーンに保存済み",
      pasteOPML: "OPML XML を貼り付け：",
      pasteTranscript: "SRT / VTT / タイムスタンプ付きテキストを貼り付け：",
      importBtn: "取り込む",
      regionAuto: "自動",
      noResults: "一致する番組が見つかりませんでした。",
      emptyVocab: "字幕の英単語をクリックすると意味を調べて保存できます。",
      allShows: "すべての番組",
      playbackErrorPrefix: "再生エラー: ",
      cspMediaBlocked: "現在起動中の Hyla の WebView CSP によりローカル音声がブロックされました。Hyla を再ビルドまたは更新して再起動してください。",
      buffering: "バッファ中…",
      playingState: "再生中",
      liveCaption: "ライブ字幕",
      untranslatedBadge: "未翻訳",
      untranslatedHint: "翻訳がまだありません。「翻訳」をクリックすると二言語字幕を表示します。",
      partialTranslated: "翻訳済み",
      continueTranslate: "続きを翻訳",
      translatingNow: "字幕を翻訳中…",
      mute: "ミュート",
      unmute: "ミュート解除",
      openSettings: "設定を開く"
    }
  };

  function resolveLocaleKey(raw) {
    const norm = String(raw || "").trim().replace(/_/g, "-").toLowerCase();
    if (norm === "zh-hans" || norm === "zh-cn" || norm === "zh-sg" || norm === "zh" || norm.startsWith("zh-hans")) return "zh-Hans";
    if (norm === "ja" || norm.startsWith("ja-")) return "ja";
    return "en";
  }

  let locale = resolveLocaleKey(new URLSearchParams(window.location.search).get("locale") || "en");
  const t = () => I18N[resolveLocaleKey(locale)] || I18N.en;

  const state = {
    tab: "home",
    selectedFeedId: "",
    selectedEpId: "",
    epSubTab: "subtitles",
    subMode: "bilingual",
    autoScroll: true,
    favoritesOnly: false,
    homeQuery: "",
    discoverCat: "all",
    searchQuery: "",
    searchCountry: "auto",
    searchResults: null,
    subSearch: "",
    feeds: [],
    episodes: [],
    queue: [],
    vocab: [],
    stats: { totalSec: 0, hourlySec: [], dailySec: {}, showSec: {} },
    discover: [],
    settings: {},
    storage: { downloadsCount: 0, downloadsBytes: 0, coversBytes: 0, cacheBytes: 0 },
    jobs: {},
    epDetail: null,
    currentEpId: "",
    playing: false,
    buffering: false,
    playbackRate: 1.0,
    volume: 1.0,
    muted: false,
    lastSavedPos: 0,
    lastSaveAt: 0,
    playedAccSec: 0,
    lastTickAt: 0,
    activeCueIdx: -1,
    banner: null,
    askingAI: false,
    cspMediaBlocked: false
  };

  const navTabsEl = document.getElementById("nav-tabs");
  const refreshBtn = document.getElementById("refresh-btn");
  const settingsBtn = document.getElementById("settings-btn");
  const jobPillEl = document.getElementById("job-pill");
  const bannerEl = document.getElementById("banner");
  const mainEl = document.getElementById("main");
  const miniPlayerEl = document.getElementById("mini-player");
  const modalBackdropEl = document.getElementById("modal-backdrop");
  const audioEl = document.getElementById("audio-el");

  function el(tag, cls, text) {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  function safeHttpUrl(raw) {
    if (!raw) return "";
    try {
      const u = new URL(raw);
      if (u.protocol === "http:" || u.protocol === "https:") return u.toString();
    } catch {}
    return "";
  }

  function formatClock(sec) {
    sec = Math.max(0, Math.floor(Number(sec) || 0));
    const h = Math.floor(sec / 3600);
    const m = Math.floor((sec % 3600) / 60);
    const s = sec % 60;
    if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
    return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }

  function parseClock(raw) {
    const parts = String(raw || "").split(":").map(Number);
    if (parts.some(Number.isNaN)) return 0;
    if (parts.length === 2) return parts[0] * 60 + parts[1];
    if (parts.length === 3) return parts[0] * 3600 + parts[1] * 60 + parts[2];
    return 0;
  }

  function formatDate(ms) {
    if (!ms) return "";
    try {
      return new Date(ms).toLocaleDateString(resolveLocaleKey(locale), { month: "short", day: "numeric", year: "numeric" });
    } catch {
      return "";
    }
  }

  function formatBytes(b) {
    const n = Number(b) || 0;
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
    return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }

  function formatHours(sec) {
    const s = Math.floor(Number(sec) || 0);
    if (s < 60) return `${s}s`;
    if (s < 3600) return `${Math.floor(s / 60)}m`;
    return `${(s / 3600).toFixed(1)}h`;
  }

  function targetLangCode() {
    const loc = resolveLocaleKey(locale);
    if (loc === "ja") return "ja";
    if (loc === "en") return "en";
    return "zh-CN";
  }

  function makeCover(coverUrl, title, sizeCls) {
    const cls = ["cover", sizeCls || ""].filter(Boolean).join(" ");
    if (coverUrl) {
      const img = el("img", cls);
      img.src = coverUrl;
      img.alt = "";
      img.loading = "lazy";
      img.addEventListener("error", () => {
        const ph = el("div", cls + " cover-placeholder", (title || "P").trim().slice(0, 2).toUpperCase());
        img.replaceWith(ph);
      });
      return img;
    }
    return el("div", cls + " cover-placeholder", (title || "P").trim().slice(0, 2).toUpperCase());
  }

  function showBanner(text, opts) {
    state.banner = { text, ...(opts || {}) };
    renderBanner();
  }

  function clearBanner() {
    state.banner = null;
    renderBanner();
  }

  function renderBanner() {
    if (!state.banner) {
      bannerEl.hidden = true;
      bannerEl.textContent = "";
      return;
    }
    bannerEl.hidden = false;
    bannerEl.className = state.banner.error ? "banner error" : "banner";
    bannerEl.textContent = "";
    bannerEl.appendChild(el("span", "", state.banner.text));
    const acts = el("div", "banner-actions");
    if (state.banner.actionLabel && state.banner.onAction) {
      const ab = el("button", "btn small primary", state.banner.actionLabel);
      ab.type = "button";
      ab.addEventListener("click", state.banner.onAction);
      acts.appendChild(ab);
    }
    const db = el("button", "btn small", t().dismiss);
    db.type = "button";
    db.addEventListener("click", clearBanner);
    acts.appendChild(db);
    bannerEl.appendChild(acts);
  }

  function renderJobPill() {
    const active = Object.values(state.jobs).filter((j) => j.state === "running");
    if (active.length === 0) {
      jobPillEl.hidden = true;
      jobPillEl.textContent = "";
      return;
    }
    const j = active[0];
    const pct = j.progress > 0 ? ` ${j.progress}%` : "";
    jobPillEl.textContent = `${j.message || j.kind}${pct}`;
    jobPillEl.hidden = false;
  }

  function renderTopbar() {
    const d = t();
    document.title = d.title;
    document.documentElement.lang = resolveLocaleKey(locale);
    refreshBtn.textContent = d.refresh;
    settingsBtn.textContent = "⚙ " + d.settings;

    navTabsEl.textContent = "";
    const tabs = [
      ["home", d.home],
      ["discover", d.discover],
      ["library", d.library],
      ["vocab", d.vocabStats]
    ];
    for (const [id, label] of tabs) {
      const b = el("button", "nav-tab" + (state.tab === id && !state.selectedEpId ? " active" : ""), label);
      b.type = "button";
      b.addEventListener("click", () => {
        state.selectedEpId = "";
        if (id !== "library") state.selectedFeedId = "";
        state.tab = id;
        render();
      });
      navTabsEl.appendChild(b);
    }
    renderJobPill();
  }

  function activeTranslateJobFor(epId) {
    if (!epId) return null;
    return Object.values(state.jobs).find((j) => j && j.kind === "translate" && j.targetId === epId && j.state === "running") || null;
  }

  function countTranslatedSegments(segs) {
    if (!Array.isArray(segs)) return 0;
    let n = 0;
    for (const s of segs) {
      if (s && String(s.translation || "").trim()) n++;
    }
    return n;
  }

  function subModeLabel(mode) {
    const d = t();
    if (mode === "translation") return d.translationOnly;
    if (mode === "original") return d.originalOnly;
    return d.bilingual;
  }

  function updatePlaybackIndicators() {
    const d = t();
    const epPlayBtn = document.getElementById("ep-play-btn");
    if (epPlayBtn && state.selectedEpId === state.currentEpId) {
      if (state.buffering && !audioEl.paused) epPlayBtn.textContent = d.buffering;
      else epPlayBtn.textContent = audioEl.paused ? d.play : d.pause;
    }
    renderMiniPlayer();
    updateLiveCaption();
  }

  async function triggerTranslate(epId, force) {
    try {
      const res = await window.hyla.rpc("podcast.translateTranscript", {
        episodeId: epId,
        targetLang: targetLangCode(),
        force: !!force
      });
      if (res && res.job) state.jobs[res.job.id] = res.job;
      renderJobPill();
      renderSubStatusBox(findEpisode(epId), state.epDetail, document.getElementById("sub-status"));
      updateLiveCaption(true);
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    }
  }

  function findEpisode(id) {
    return state.episodes.find((e) => e.id === id) || null;
  }

  function upsertEpisode(ep) {
    if (!ep || !ep.id) return;
    const idx = state.episodes.findIndex((e) => e.id === ep.id);
    if (idx >= 0) {
      state.episodes[idx] = ep;
    } else {
      state.episodes.unshift(ep);
    }
  }

  async function bootstrap() {
    try {
      const snap = await window.hyla.rpc("podcast.bootstrap", {});
      applyBootstrap(snap);
      render();
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    }
  }

  function applyBootstrap(snap) {
    if (!snap) return;
    if (snap.locale) locale = resolveLocaleKey(snap.locale);
    state.feeds = snap.feeds || [];
    state.episodes = snap.episodes || [];
    state.queue = snap.queue || [];
    state.vocab = snap.vocab || [];
    state.stats = snap.stats || { totalSec: 0, hourlySec: [], dailySec: {}, showSec: {} };
    state.discover = snap.discover || [];
    state.settings = snap.settings || {};
    state.storage = snap.storage || state.storage;
    if (Array.isArray(snap.activeJobs)) {
      const nextJobs = {};
      for (const j of snap.activeJobs) nextJobs[j.id] = j;
      state.jobs = nextJobs;
    }
  }

  function describePlaybackError(err) {
    const d = t();
    if (state.cspMediaBlocked) {
      return d.cspMediaBlocked;
    }
    const me = audioEl && audioEl.error;
    const parts = [];
    if (err && (err.message || String(err))) {
      parts.push(err.message || String(err));
    }
    if (me && me.code) {
      const labels = {
        1: "MEDIA_ERR_ABORTED",
        2: "MEDIA_ERR_NETWORK",
        3: "MEDIA_ERR_DECODE",
        4: "MEDIA_ERR_SRC_NOT_SUPPORTED"
      };
      const tag = labels[me.code] || ("code " + me.code);
      const extra = me.message ? `${tag}: ${me.message}` : tag;
      if (!parts.some((p) => p.includes(tag))) parts.push(extra);
    }
    return d.playbackErrorPrefix + (parts.join(" · ") || "Unknown error");
  }

  async function openEpisode(epId, autoPlay, seekSec) {
    state.selectedEpId = epId;
    state.epDetail = null;
    const existing = findEpisode(epId);
    const shouldStart = !!(autoPlay || typeof seekSec === "number");
    let startedPlayback = false;
    if (existing && shouldStart) {
      startedPlayback = true;
      void playEpisode(existing, typeof seekSec === "number" ? seekSec : undefined, autoPlay !== false);
    }
    render();
    try {
      const res = await window.hyla.rpc("podcast.episodeDetail", { episodeId: epId });
      if (res && res.episode) upsertEpisode(res.episode);
      state.epDetail = res ? res.detail : null;
      if (!startedPlayback && shouldStart && res && res.episode) {
        await playEpisode(res.episode, typeof seekSec === "number" ? seekSec : undefined, autoPlay !== false);
      }
      render();
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    }
  }

  async function playEpisode(ep, seekToSec, startPlay) {
    if (!ep || !ep.playUrl) return;
    const isSame = state.currentEpId === ep.id && audioEl.src;
    if (!isSame) {
      // Snapshot and flush previous episode progress in the background so audioEl.play()
      // stays synchronous within the user click activation in WKWebView/WebKit.
      void flushProgress(false);
      state.cspMediaBlocked = false;
      state.currentEpId = ep.id;
      audioEl.src = ep.playUrl;
      audioEl.load();
      audioEl.playbackRate = state.playbackRate;
      audioEl.volume = state.volume;
      audioEl.muted = !!state.muted;
      const targetPos = typeof seekToSec === "number" ? seekToSec : (ep.positionSec > 2 && !ep.completed ? ep.positionSec : 0);
      if (targetPos > 0) {
        const applyPos = () => {
          try { audioEl.currentTime = targetPos; } catch {}
        };
        if (audioEl.readyState >= 1) applyPos();
        else audioEl.addEventListener("loadedmetadata", applyPos, { once: true });
      }
      state.lastSavedPos = targetPos;
    } else if (typeof seekToSec === "number") {
      try { audioEl.currentTime = Math.max(0, seekToSec); } catch {}
    }
    let playPromise = null;
    if (startPlay !== false) {
      state.cspMediaBlocked = false;
      state.buffering = true;
      try {
        playPromise = audioEl.play();
      } catch (err) {
        state.playing = false;
        state.buffering = false;
        showBanner(describePlaybackError(err), { error: true });
      }
    }
    updatePlaybackIndicators();
    if (playPromise) {
      try {
        await playPromise;
        state.playing = true;
        state.lastTickAt = Date.now();
        clearBanner();
      } catch (err) {
        if (err && err.name === "AbortError") return;
        state.playing = false;
        state.buffering = false;
        showBanner(describePlaybackError(err), { error: true });
      }
      updatePlaybackIndicators();
    }
  }

  async function togglePlayPause() {
    if (!state.currentEpId) {
      if (state.selectedEpId) {
        const ep = findEpisode(state.selectedEpId);
        if (ep) await playEpisode(ep, undefined, true);
      }
      return;
    }
    if (audioEl.paused) {
      state.cspMediaBlocked = false;
      try {
        await audioEl.play();
        state.playing = true;
        state.lastTickAt = Date.now();
        clearBanner();
      } catch (err) {
        if (!err || err.name !== "AbortError") {
          state.playing = false;
          showBanner(describePlaybackError(err), { error: true });
        }
      }
    } else {
      audioEl.pause();
      state.playing = false;
      await flushProgress(false);
    }
    renderMiniPlayer();
    if (state.selectedEpId === state.currentEpId) render();
  }

  function seekRelative(delta) {
    if (!state.currentEpId) return;
    const next = Math.max(0, Math.min(audioEl.duration || 1e9, (audioEl.currentTime || 0) + delta));
    audioEl.currentTime = next;
    flushProgress(false);
    highlightActiveCue();
  }

  function seekAbsolute(sec) {
    if (state.selectedEpId && state.currentEpId !== state.selectedEpId) {
      const ep = findEpisode(state.selectedEpId);
      if (ep) {
        playEpisode(ep, sec, true);
        return;
      }
    }
    if (!state.currentEpId) return;
    audioEl.currentTime = Math.max(0, sec);
    if (audioEl.paused) {
      audioEl.play().catch(() => {});
      state.playing = true;
    }
    flushProgress(false);
    highlightActiveCue();
  }

  async function flushProgress(completed) {
    if (!state.currentEpId) return;
    const epId = state.currentEpId;
    const pos = Number(audioEl.currentTime) || 0;
    const dur = Number(audioEl.duration) || 0;
    const delta = state.playedAccSec;
    state.playedAccSec = 0;
    state.lastSaveAt = Date.now();
    try {
      const res = await window.hyla.rpc("podcast.saveProgress", {
        episodeId: epId,
        positionSec: pos,
        durationSec: dur,
        completed: !!completed,
        playedDeltaSec: delta
      });
      if (res && res.episode) upsertEpisode(res.episode);
      if (res && res.stats) state.stats = res.stats;
    } catch {}
  }

  async function toggleFavorite(epId) {
    try {
      const res = await window.hyla.rpc("podcast.toggleFavorite", { episodeId: epId });
      if (res && res.episode) upsertEpisode(res.episode);
      render();
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    }
  }

  async function toggleQueue(epId) {
    const next = state.queue.includes(epId)
      ? state.queue.filter((id) => id !== epId)
      : [...state.queue, epId];
    try {
      const res = await window.hyla.rpc("podcast.queueSet", { queue: next });
      state.queue = (res && res.queue) || next;
      render();
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    }
  }

  async function subscribeInput(input, discoverId, category) {
    try {
      const res = await window.hyla.rpc("podcast.subscribe", { input: input || "", discoverId: discoverId || "", category: category || "" });
      if (res && res.feed) {
        const idx = state.feeds.findIndex((f) => f.id === res.feed.id);
        if (idx >= 0) state.feeds[idx] = res.feed;
        else state.feeds.push(res.feed);
      }
      if (res && Array.isArray(res.episodes)) {
        for (const ep of res.episodes) upsertEpisode(ep);
        state.episodes.sort((a, b) => (b.publishedAt || 0) - (a.publishedAt || 0));
      }
      render();
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    }
  }

  function renderEpisodeCard(ep) {
    const d = t();
    const card = el("div", "card clickable");
    card.appendChild(makeCover(ep.coverUrl, ep.feedTitle || ep.title));
    const body = el("div", "card-body");

    const title = el("div", "card-title", ep.title);
    title.addEventListener("click", () => openEpisode(ep.id, false));
    body.appendChild(title);

    const meta = el("div", "card-meta");
    if (ep.feedTitle) meta.appendChild(el("span", "", ep.feedTitle));
    if (ep.publishedAt) meta.appendChild(el("span", "", "· " + formatDate(ep.publishedAt)));
    if (ep.durationSec > 0) meta.appendChild(el("span", "", "· " + formatClock(ep.durationSec)));
    if (ep.hasTranscript) meta.appendChild(el("span", "pill accent", d.subtitles));
    if (ep.downloadedFile) meta.appendChild(el("span", "pill done", d.offline));
    body.appendChild(meta);

    if (ep.durationSec > 0 && ep.positionSec > 0) {
      const pct = Math.min(100, Math.round((ep.positionSec / ep.durationSec) * 100));
      const track = el("div", "progress-track");
      const fill = el("div", "progress-fill");
      fill.style.width = `${pct}%`;
      track.appendChild(fill);
      body.appendChild(track);
    }

    const actions = el("div", "card-actions");
    const isPlayingThis = state.currentEpId === ep.id && !audioEl.paused;
    const playBtn = el("button", "btn small primary", isPlayingThis ? d.pause : d.play);
    playBtn.type = "button";
    playBtn.addEventListener("click", (ev) => {
      ev.stopPropagation();
      if (state.currentEpId === ep.id) {
        togglePlayPause();
      } else {
        openEpisode(ep.id, true);
      }
    });
    actions.appendChild(playBtn);

    const qBtn = el("button", "btn small", state.queue.includes(ep.id) ? d.queued : d.queueAdd);
    qBtn.type = "button";
    qBtn.addEventListener("click", (ev) => {
      ev.stopPropagation();
      toggleQueue(ep.id);
    });
    actions.appendChild(qBtn);

    const starBtn = el("button", "btn small", ep.favorited ? "★ " + d.favorited : "☆ " + d.favorite);
    starBtn.type = "button";
    starBtn.addEventListener("click", (ev) => {
      ev.stopPropagation();
      toggleFavorite(ep.id);
    });
    actions.appendChild(starBtn);

    body.appendChild(actions);
    card.appendChild(body);
    card.addEventListener("click", () => openEpisode(ep.id, false));
    return card;
  }

  function renderHome() {
    const d = t();
    const subRow = el("form", "section-bar");
    const urlIn = el("input", "input grow");
    urlIn.type = "text";
    urlIn.placeholder = d.subscribePlaceholder;
    const subBtn = el("button", "btn primary", d.subscribe);
    subBtn.type = "submit";
    subRow.appendChild(urlIn);
    subRow.appendChild(subBtn);
    subRow.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const val = urlIn.value.trim();
      if (!val) return;
      subBtn.disabled = true;
      await subscribeInput(val, "", "");
      urlIn.value = "";
      subBtn.disabled = false;
    });
    mainEl.appendChild(subRow);

    const filterRow = el("div", "section-bar");
    const qIn = el("input", "input grow");
    qIn.type = "search";
    qIn.placeholder = d.searchTranscript;
    qIn.value = state.homeQuery;
    qIn.addEventListener("input", () => {
      state.homeQuery = qIn.value;
      renderEpisodesListOnly(listWrap);
    });
    filterRow.appendChild(qIn);

    const favChip = el("button", "chip" + (state.favoritesOnly ? " active" : ""), "★ " + d.favoritesOnly);
    favChip.type = "button";
    favChip.addEventListener("click", () => {
      state.favoritesOnly = !state.favoritesOnly;
      render();
    });
    filterRow.appendChild(favChip);
    mainEl.appendChild(filterRow);

    const listWrap = el("div", "card-list");
    mainEl.appendChild(listWrap);
    renderEpisodesListOnly(listWrap);
  }

  function renderEpisodesListOnly(container) {
    const d = t();
    container.textContent = "";
    const q = state.homeQuery.trim().toLowerCase();
    const filtered = state.episodes.filter((ep) => {
      if (state.favoritesOnly && !ep.favorited) return false;
      if (state.selectedFeedId && ep.feedId !== state.selectedFeedId) return false;
      if (!q) return true;
      return (ep.title || "").toLowerCase().includes(q) ||
        (ep.feedTitle || "").toLowerCase().includes(q) ||
        (ep.summary || "").toLowerCase().includes(q);
    });
    if (filtered.length === 0) {
      container.appendChild(el("div", "empty-box", d.emptyHome));
      return;
    }
    for (const ep of filtered.slice(0, 120)) {
      container.appendChild(renderEpisodeCard(ep));
    }
  }

  function renderDiscover() {
    const d = t();
    const searchForm = el("form", "section-bar");
    const qIn = el("input", "input grow");
    qIn.type = "search";
    qIn.placeholder = d.searchPlaceholder;
    qIn.value = state.searchQuery;
    const regionSel = el("select", "select");
    for (const [val, lbl] of [["auto", d.regionAuto], ["cn", "CN"], ["us", "US"], ["jp", "JP"]]) {
      const opt = el("option", "", lbl);
      opt.value = val;
      if (val === state.searchCountry) opt.selected = true;
      regionSel.appendChild(opt);
    }
    regionSel.addEventListener("change", () => { state.searchCountry = regionSel.value; });
    const sBtn = el("button", "btn primary", d.search);
    sBtn.type = "submit";
    searchForm.appendChild(qIn);
    searchForm.appendChild(regionSel);
    searchForm.appendChild(sBtn);
    searchForm.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const q = qIn.value.trim();
      state.searchQuery = q;
      if (!q) {
        state.searchResults = null;
        render();
        return;
      }
      sBtn.disabled = true;
      try {
        const res = await window.hyla.rpc("podcast.search", { query: q, country: state.searchCountry });
        state.searchResults = (res && res.results) || [];
        render();
      } catch (err) {
        showBanner(err.message || String(err), { error: true });
      } finally {
        sBtn.disabled = false;
      }
    });
    mainEl.appendChild(searchForm);

    const subbedUrls = new Set(state.feeds.map((f) => f.feedUrl));

    if (Array.isArray(state.searchResults)) {
      const hdr = el("div", "section-bar");
      hdr.appendChild(el("span", "section-title", `${d.search}: "${state.searchQuery}"`));
      const clr = el("button", "btn small", d.close);
      clr.type = "button";
      clr.addEventListener("click", () => {
        state.searchResults = null;
        render();
      });
      hdr.appendChild(clr);
      mainEl.appendChild(hdr);

      if (state.searchResults.length === 0) {
        mainEl.appendChild(el("div", "empty-box", d.noResults));
      } else {
        const grid = el("div", "card-grid");
        for (const sr of state.searchResults) {
          const card = el("div", "card");
          card.appendChild(makeCover(sr.coverUrl, sr.title));
          const body = el("div", "card-body");
          body.appendChild(el("div", "card-title", sr.title));
          body.appendChild(el("div", "card-meta", [sr.author, sr.genre].filter(Boolean).join(" · ")));
          const acts = el("div", "card-actions");
          const isSub = subbedUrls.has(sr.feedUrl);
          const b = el("button", "btn small " + (isSub ? "" : "primary"), isSub ? d.subscribed : d.subscribe);
          b.type = "button";
          b.disabled = isSub;
          b.addEventListener("click", async () => {
            b.disabled = true;
            await subscribeInput(sr.feedUrl, "", sr.genre || "");
          });
          acts.appendChild(b);
          body.appendChild(acts);
          card.appendChild(body);
          grid.appendChild(card);
        }
        mainEl.appendChild(grid);
      }
    }

    const cats = [
      ["all", d.catAll],
      ["ai", d.catAI],
      ["eng", d.catEng],
      ["tech", d.catTech],
      ["business", d.catBusiness],
      ["zh", d.catZH],
      ["english", d.catEnglish]
    ];
    const chipBar = el("div", "chips");
    for (const [cid, clbl] of cats) {
      const c = el("button", "chip" + (state.discoverCat === cid ? " active" : ""), clbl);
      c.type = "button";
      c.addEventListener("click", () => {
        state.discoverCat = cid;
        render();
      });
      chipBar.appendChild(c);
    }
    mainEl.appendChild(chipBar);

    const grid = el("div", "card-grid");
    const loc = resolveLocaleKey(locale);
    for (const item of state.discover) {
      if (state.discoverCat !== "all" && item.category !== state.discoverCat) continue;
      const card = el("div", "card");
      card.appendChild(makeCover(item.coverUrl, item.title));
      const body = el("div", "card-body");
      body.appendChild(el("div", "card-title", item.title));
      body.appendChild(el("div", "card-meta", `${item.author} · ${item.language.toUpperCase()}`));
      const desc = loc === "zh-Hans" ? (item.descriptionZh || item.description) : loc === "ja" ? (item.descriptionJa || item.description) : item.description;
      body.appendChild(el("div", "card-desc", desc));
      const acts = el("div", "card-actions");
      const isSub = subbedUrls.has(item.feedUrl);
      const b = el("button", "btn small " + (isSub ? "" : "primary"), isSub ? d.subscribed : d.subscribe);
      b.type = "button";
      b.disabled = isSub;
      b.addEventListener("click", async () => {
        b.disabled = true;
        await subscribeInput("", item.id, item.category);
      });
      acts.appendChild(b);
      body.appendChild(acts);
      card.appendChild(body);
      grid.appendChild(card);
    }
    mainEl.appendChild(grid);
  }

  function renderLibrary() {
    const d = t();
    if (state.selectedFeedId) {
      const feed = state.feeds.find((f) => f.id === state.selectedFeedId);
      const top = el("div", "section-bar");
      const bk = el("button", "btn small", d.back);
      bk.type = "button";
      bk.addEventListener("click", () => {
        state.selectedFeedId = "";
        render();
      });
      top.appendChild(bk);
      if (feed) {
        top.appendChild(el("strong", "", feed.title));
        const playAll = el("button", "btn small primary", d.playAll);
        playAll.type = "button";
        playAll.addEventListener("click", async () => {
          const eps = state.episodes.filter((e) => e.feedId === feed.id);
          if (eps.length > 0) {
            state.queue = eps.slice(1, 30).map((e) => e.id);
            const openPromise = openEpisode(eps[0].id, true);
            await window.hyla.rpc("podcast.queueSet", { queue: state.queue }).catch(() => {});
            await openPromise;
          }
        });
        top.appendChild(playAll);

        const unsub = el("button", "btn small danger", d.unsubscribe);
        unsub.type = "button";
        unsub.addEventListener("click", async () => {
          const snap = await window.hyla.rpc("podcast.unsubscribe", { feedId: feed.id });
          applyBootstrap(snap);
          state.selectedFeedId = "";
          render();
        });
        top.appendChild(unsub);
      }
      mainEl.appendChild(top);
      const list = el("div", "card-list");
      renderEpisodesListOnly(list);
      mainEl.appendChild(list);
      return;
    }

    const tools = el("div", "section-bar");
    const expBtn = el("button", "btn small", d.exportOPML);
    expBtn.type = "button";
    expBtn.addEventListener("click", async () => {
      const res = await window.hyla.rpc("podcast.exportOPML", {});
      openExportModal(d.exportOPML, res.opml || "");
    });
    tools.appendChild(expBtn);

    const impBtn = el("button", "btn small", d.importOPML);
    impBtn.type = "button";
    impBtn.addEventListener("click", openImportOPMLModal);
    tools.appendChild(impBtn);
    mainEl.appendChild(tools);

    mainEl.appendChild(el("div", "section-title", `${d.shows} (${state.feeds.length})`));
    if (state.feeds.length === 0) {
      mainEl.appendChild(el("div", "empty-box", d.emptyHome));
    } else {
      const grid = el("div", "card-grid");
      for (const f of state.feeds) {
        const card = el("div", "card clickable");
        card.appendChild(makeCover(f.coverUrl, f.title));
        const body = el("div", "card-body");
        body.appendChild(el("div", "card-title", f.title));
        body.appendChild(el("div", "card-meta", [f.author, `${f.episodeCount || 0} eps`].filter(Boolean).join(" · ")));
        if (f.description) body.appendChild(el("div", "card-desc", f.description));
        card.appendChild(body);
        card.addEventListener("click", () => {
          state.selectedFeedId = f.id;
          render();
        });
        grid.appendChild(card);
      }
      mainEl.appendChild(grid);
    }

    mainEl.appendChild(el("div", "section-title", d.storage));
    const stBox = el("div", "card");
    const stBody = el("div", "card-body");
    stBody.appendChild(el("div", "card-meta", `${d.offline}: ${state.storage.downloadsCount || 0} (${formatBytes(state.storage.downloadsBytes)}) · Cache: ${formatBytes((state.storage.coversBytes || 0) + (state.storage.cacheBytes || 0))}`));
    const clrBtn = el("button", "btn small danger", d.clearCache);
    clrBtn.type = "button";
    clrBtn.addEventListener("click", async () => {
      const res = await window.hyla.rpc("podcast.clearCache", {});
      if (res && res.bootstrap) applyBootstrap(res.bootstrap);
      render();
    });
    const acts = el("div", "card-actions");
    acts.appendChild(clrBtn);
    stBody.appendChild(acts);
    stBox.appendChild(stBody);
    mainEl.appendChild(stBox);
  }

  function renderEpisodeView() {
    const d = t();
    const ep = findEpisode(state.selectedEpId);
    if (!ep) {
      state.selectedEpId = "";
      render();
      return;
    }
    const det = state.epDetail || { episodeId: ep.id, segments: [], chapters: ep.chapters || [], guide: null, chat: [] };

    const topNav = el("div", "section-bar");
    const bk = el("button", "btn small", d.back);
    bk.type = "button";
    bk.addEventListener("click", () => {
      state.selectedEpId = "";
      render();
    });
    topNav.appendChild(bk);
    if (ep.feedTitle) topNav.appendChild(el("span", "card-meta", ep.feedTitle));
    const link = safeHttpUrl(ep.linkUrl);
    if (link) {
      const a = el("a", "ext-link", d.openLink);
      a.href = link;
      topNav.appendChild(a);
    }
    mainEl.appendChild(topNav);

    const hdr = el("div", "ep-header");
    hdr.appendChild(makeCover(ep.coverUrl, ep.feedTitle || ep.title, "large"));
    const hBody = el("div", "card-body");
    hBody.appendChild(el("div", "card-title", ep.title));
    hBody.appendChild(el("div", "card-meta", [formatDate(ep.publishedAt), ep.durationSec > 0 ? formatClock(ep.durationSec) : ""].filter(Boolean).join(" · ")));
    const hActs = el("div", "card-actions");

    const starBtn = el("button", "btn small", ep.favorited ? "★ " + d.favorited : "☆ " + d.favorite);
    starBtn.type = "button";
    starBtn.addEventListener("click", () => toggleFavorite(ep.id));
    hActs.appendChild(starBtn);

    if (ep.downloadedFile) {
      const delDl = el("button", "btn small", d.deleteOffline + ` (${formatBytes(ep.downloadedBytes)})`);
      delDl.type = "button";
      delDl.addEventListener("click", async () => {
        const res = await window.hyla.rpc("podcast.deleteDownload", { episodeId: ep.id });
        if (res && res.episode) upsertEpisode(res.episode);
        if (res && res.storage) state.storage = res.storage;
        render();
      });
      hActs.appendChild(delDl);
    } else if (ep.audioUrl) {
      const dlBtn = el("button", "btn small", d.download);
      dlBtn.type = "button";
      dlBtn.addEventListener("click", async () => {
        const res = await window.hyla.rpc("podcast.downloadEpisode", { episodeId: ep.id });
        if (res && res.job) state.jobs[res.job.id] = res.job;
        renderJobPill();
      });
      hActs.appendChild(dlBtn);
    }
    hBody.appendChild(hActs);
    hdr.appendChild(hBody);
    mainEl.appendChild(hdr);

    // Player Panel
    const player = el("div", "player-panel");
    const isCurrent = state.currentEpId === ep.id;
    const curSec = isCurrent ? (Number(audioEl.currentTime) || 0) : (ep.positionSec || 0);
    const durSec = isCurrent ? (Number(audioEl.duration) || ep.durationSec || 0) : (ep.durationSec || 0);

    const seekRow = el("div", "seek-row");
    const curLbl = el("span", "time-label", formatClock(curSec));
    curLbl.id = "player-cur-lbl";
    const slider = el("input", "range-slider");
    slider.id = "player-slider";
    slider.type = "range";
    slider.min = "0";
    slider.max = String(Math.max(1, Math.floor(durSec || 100)));
    slider.value = String(Math.floor(curSec));
    slider.addEventListener("input", () => {
      curLbl.textContent = formatClock(slider.value);
    });
    slider.addEventListener("change", () => {
      seekAbsolute(Number(slider.value) || 0);
    });
    const durLbl = el("span", "time-label right", formatClock(durSec));
    durLbl.id = "player-dur-lbl";
    seekRow.appendChild(curLbl);
    seekRow.appendChild(slider);
    seekRow.appendChild(durLbl);
    player.appendChild(seekRow);

    const transRow = el("div", "transport-row");
    const tLeft = el("div", "transport-left");
    const rwBtn = el("button", "btn small", "-15s");
    rwBtn.type = "button";
    rwBtn.addEventListener("click", () => seekRelative(-15));
    const playBtnLabel = (isCurrent && !audioEl.paused) ? (state.buffering ? d.buffering : d.pause) : d.play;
    const playBtn = el("button", "btn small primary", playBtnLabel);
    playBtn.id = "ep-play-btn";
    playBtn.type = "button";
    playBtn.addEventListener("click", () => {
      if (isCurrent) togglePlayPause();
      else playEpisode(ep, undefined, true);
    });
    const fwBtn = el("button", "btn small", "+30s");
    fwBtn.type = "button";
    fwBtn.addEventListener("click", () => seekRelative(30));
    tLeft.appendChild(rwBtn);
    tLeft.appendChild(playBtn);
    tLeft.appendChild(fwBtn);

    const tRight = el("div", "transport-right");
    const volWrap = el("div", "vol-wrap");
    const muteBtn = el("button", "btn small", state.muted || state.volume === 0 ? "🔇" : "🔊");
    muteBtn.id = "ep-mute-btn";
    muteBtn.type = "button";
    muteBtn.title = state.muted ? d.unmute : d.mute;
    muteBtn.setAttribute("aria-label", muteBtn.title);
    const volSlider = el("input", "vol-slider");
    volSlider.type = "range";
    volSlider.min = "0";
    volSlider.max = "1";
    volSlider.step = "0.05";
    volSlider.value = String(state.muted ? 0 : state.volume);
    muteBtn.addEventListener("click", () => {
      state.muted = !state.muted;
      audioEl.muted = state.muted;
      muteBtn.textContent = state.muted || state.volume === 0 ? "🔇" : "🔊";
      muteBtn.title = state.muted ? d.unmute : d.mute;
      volSlider.value = String(state.muted ? 0 : state.volume);
    });
    volSlider.addEventListener("input", () => {
      const v = Math.max(0, Math.min(1, Number(volSlider.value) || 0));
      state.volume = v;
      state.muted = v === 0;
      audioEl.volume = v;
      audioEl.muted = state.muted;
      muteBtn.textContent = state.muted ? "🔇" : "🔊";
      muteBtn.title = state.muted ? d.unmute : d.mute;
    });
    volWrap.appendChild(muteBtn);
    volWrap.appendChild(volSlider);
    tRight.appendChild(volWrap);

    const spdBtn = el("button", "btn small", `${state.playbackRate}x`);
    spdBtn.type = "button";
    spdBtn.addEventListener("click", cycleSpeed);
    tRight.appendChild(spdBtn);

    transRow.appendChild(tLeft);
    transRow.appendChild(tRight);
    player.appendChild(transRow);
    mainEl.appendChild(player);

    if (Array.isArray(det.segments) && det.segments.length > 0) {
      const liveBox = el("div", "live-caption");
      liveBox.id = "live-caption";
      mainEl.appendChild(liveBox);
      updateLiveCaption(true, ep, det);
    }

    // Sub-tabs
    const subTabs = el("div", "subtabs");
    for (const [sid, slbl] of [
      ["subtitles", d.subtitles],
      ["guide", d.aiGuide],
      ["qa", d.aiQA],
      ["notes", d.showNotes]
    ]) {
      const b = el("button", "nav-tab" + (state.epSubTab === sid ? " active" : ""), slbl);
      b.type = "button";
      b.addEventListener("click", () => {
        state.epSubTab = sid;
        render();
      });
      subTabs.appendChild(b);
    }
    mainEl.appendChild(subTabs);

    if (state.epSubTab === "subtitles") {
      renderSubtitlesTab(ep, det);
    } else if (state.epSubTab === "guide") {
      renderGuideTab(ep, det);
    } else if (state.epSubTab === "qa") {
      renderQATab(ep, det);
    } else {
      renderNotesTab(ep, det);
    }
  }

  function findCueForTime(segs, now) {
    if (!Array.isArray(segs) || segs.length === 0) return { idx: -1, seg: null };
    let lastPassed = 0;
    for (let i = 0; i < segs.length; i++) {
      const st = Number(segs[i].startSec) || 0;
      const en = Number(segs[i].endSec) || st + 5;
      if (now >= st && now <= en + 0.25) {
        return { idx: i, seg: segs[i] };
      }
      if (st <= now) lastPassed = i;
    }
    return { idx: lastPassed, seg: segs[lastPassed] };
  }

  function updateLiveCaption(forceRedraw, epArg, detArg) {
    const box = document.getElementById("live-caption");
    if (!box) return;
    const ep = epArg || findEpisode(state.selectedEpId);
    const det = detArg || state.epDetail;
    const segs = (det && det.segments) || [];
    if (!ep || segs.length === 0) {
      box.hidden = true;
      return;
    }
    box.hidden = false;
    const d = t();
    const isCurrent = state.currentEpId === ep.id;
    const now = isCurrent ? (Number(audioEl.currentTime) || 0) : (ep.positionSec || 0);
    const { idx, seg } = findCueForTime(segs, now);
    if (!seg) return;

    const job = activeTranslateJobFor(ep.id);
    const bufState = isCurrent && state.buffering && !audioEl.paused ? "buf" : "";
    const cueKey = [idx, state.subMode, seg.text || "", seg.translation || "", job ? (job.progress || 0) : -1, bufState].join("|");
    if (!forceRedraw && box.dataset.cueKey === cueKey) return;
    box.dataset.cueKey = cueKey;

    box.textContent = "";
    const meta = el("div", "live-caption-meta");
    const tsBtn = el("button", "ts-chip", formatClock(seg.startSec));
    tsBtn.type = "button";
    tsBtn.addEventListener("click", () => seekAbsolute(seg.startSec));
    meta.appendChild(tsBtn);
    meta.appendChild(el("span", "pill accent", `${d.liveCaption} · ${subModeLabel(state.subMode)}`));
    if (bufState) {
      meta.appendChild(el("span", "pill", d.buffering));
    }
    box.appendChild(meta);

    if (state.subMode !== "translation" || !seg.translation) {
      const origEl = el("div", "live-caption-orig");
      appendWordClickableText(origEl, seg.text || "", seg, ep);
      box.appendChild(origEl);
    }
    if (state.subMode !== "original") {
      if (seg.translation) {
        box.appendChild(el("div", "live-caption-trans", seg.translation));
      } else {
        const hintRow = el("div", "live-caption-hint");
        if (job) {
          const pct = job.progress > 0 ? ` ${job.progress}%` : "";
          hintRow.appendChild(el("span", "card-meta", `${d.translatingNow}${pct}`));
        } else {
          hintRow.appendChild(el("span", "card-meta", d.untranslatedHint));
          const trNowBtn = el("button", "btn small primary", d.translate);
          trNowBtn.type = "button";
          trNowBtn.addEventListener("click", () => triggerTranslate(ep.id, false));
          hintRow.appendChild(trNowBtn);
        }
        box.appendChild(hintRow);
      }
    }
  }

  function renderSubStatusBox(ep, det, container) {
    if (!container || !ep || !det) return;
    const d = t();
    const segs = det.segments || [];
    const trCount = countTranslatedSegments(segs);
    const job = activeTranslateJobFor(ep.id);

    if (segs.length === 0 || state.subMode === "original" || (trCount === segs.length && !job)) {
      container.hidden = true;
      container.textContent = "";
      return;
    }

    container.hidden = false;
    container.textContent = "";
    const top = el("div", "sub-status-top");
    if (job) {
      const pct = Math.max(0, Math.min(100, Number(job.progress) || 0));
      top.appendChild(el("span", "card-meta", `${d.translatingNow} ${pct}% (${trCount}/${segs.length})`));
      container.appendChild(top);
      const track = el("div", "progress-track");
      const fill = el("div", "progress-fill");
      fill.style.width = `${pct}%`;
      track.appendChild(fill);
      container.appendChild(track);
      return;
    }

    const msg = trCount > 0
      ? `${d.partialTranslated} ${trCount}/${segs.length}`
      : d.untranslatedHint;
    top.appendChild(el("span", "card-meta", msg));
    const acts = el("div", "card-actions");
    const trBtn = el("button", "btn small primary", trCount > 0 ? d.continueTranslate : d.translate);
    trBtn.type = "button";
    trBtn.addEventListener("click", () => triggerTranslate(ep.id, false));
    acts.appendChild(trBtn);
    const cfgBtn = el("button", "btn small", d.openSettings);
    cfgBtn.type = "button";
    cfgBtn.addEventListener("click", openSettingsModal);
    acts.appendChild(cfgBtn);
    top.appendChild(acts);
    container.appendChild(top);
  }

  function renderSubtitlesTab(ep, det) {
    const d = t();
    const segs = det.segments || [];
    const trCount = countTranslatedSegments(segs);
    const toolbar = el("div", "section-bar");

    const segCtrl = el("div", "seg-control");
    for (const [mid, mlbl] of [
      ["bilingual", d.bilingual],
      ["original", d.originalOnly],
      ["translation", d.translationOnly]
    ]) {
      const mb = el("button", "seg-btn" + (state.subMode === mid ? " active" : ""), mlbl);
      mb.type = "button";
      mb.addEventListener("click", () => {
        const prevScroll = mainEl.scrollTop;
        state.subMode = mid;
        render();
        mainEl.scrollTop = prevScroll;
      });
      segCtrl.appendChild(mb);
    }
    toolbar.appendChild(segCtrl);

    if (segs.length > 0) {
      const trLabel = (trCount > 0 && trCount < segs.length) ? d.continueTranslate : d.translate;
      const trBtn = el("button", "btn small", trLabel);
      trBtn.type = "button";
      trBtn.addEventListener("click", () => triggerTranslate(ep.id, false));
      toolbar.appendChild(trBtn);
    }

    const asrBtn = el("button", "btn small", d.transcribe);
    asrBtn.type = "button";
    asrBtn.addEventListener("click", async () => {
      const res = await window.hyla.rpc("podcast.transcribeEpisode", { episodeId: ep.id });
      if (res && res.job) state.jobs[res.job.id] = res.job;
      renderJobPill();
    });
    toolbar.appendChild(asrBtn);

    const impBtn = el("button", "btn small", d.importSub);
    impBtn.type = "button";
    impBtn.addEventListener("click", () => openImportTranscriptModal(ep.id));
    toolbar.appendChild(impBtn);

    if (segs.length > 0) {
      const expBtn = el("button", "btn small", d.exportSub);
      expBtn.type = "button";
      expBtn.addEventListener("click", () => openExportTranscriptModal(ep.id));
      toolbar.appendChild(expBtn);
    }
    mainEl.appendChild(toolbar);

    if (segs.length === 0) {
      mainEl.appendChild(el("div", "empty-box", d.noSubtitles));
      return;
    }

    const statusBox = el("div", "sub-status");
    statusBox.id = "sub-status";
    mainEl.appendChild(statusBox);
    renderSubStatusBox(ep, det, statusBox);

    const searchRow = el("div", "section-bar");
    const sq = el("input", "input grow");
    sq.type = "search";
    sq.placeholder = d.searchTranscript;
    sq.value = state.subSearch;
    sq.addEventListener("input", () => {
      state.subSearch = sq.value;
      renderCueList(ep, det, cueListEl);
    });
    const scrollChip = el("button", "chip" + (state.autoScroll ? " active" : ""), d.autoScroll);
    scrollChip.type = "button";
    scrollChip.addEventListener("click", () => {
      state.autoScroll = !state.autoScroll;
      render();
    });
    searchRow.appendChild(sq);
    searchRow.appendChild(scrollChip);
    mainEl.appendChild(searchRow);

    const cueListEl = el("div", "cue-list");
    cueListEl.id = "cue-list";
    mainEl.appendChild(cueListEl);
    renderCueList(ep, det, cueListEl);
  }

  function appendWordClickableText(container, text, seg, ep) {
    const re = /[A-Za-z]+(?:['’-][A-Za-z]+)*/g;
    let lastIdx = 0;
    let m;
    while ((m = re.exec(text)) !== null) {
      if (m.index > lastIdx) {
        container.appendChild(document.createTextNode(text.slice(lastIdx, m.index)));
      }
      const word = m[0];
      if (word.length >= 2) {
        const span = el("span", "word-tok", word);
        span.addEventListener("click", (ev) => {
          ev.stopPropagation();
          openWordModal(word, seg, ep);
        });
        container.appendChild(span);
      } else {
        container.appendChild(document.createTextNode(word));
      }
      lastIdx = m.index + word.length;
    }
    if (lastIdx < text.length) {
      container.appendChild(document.createTextNode(text.slice(lastIdx)));
    }
  }

  function renderCueList(ep, det, container) {
    const d = t();
    container.textContent = "";
    const q = state.subSearch.trim().toLowerCase();
    const segs = det.segments || [];
    for (let i = 0; i < segs.length; i++) {
      const seg = segs[i];
      if (q && !(seg.text || "").toLowerCase().includes(q) && !(seg.translation || "").toLowerCase().includes(q)) {
        continue;
      }
      const row = el("div", "cue-row");
      row.dataset.idx = String(i);
      row.dataset.start = String(seg.startSec);
      row.dataset.end = String(seg.endSec);

      const top = el("div", "cue-top");
      const topLeft = el("div", "cue-top-left");
      topLeft.appendChild(el("span", "cue-time", formatClock(seg.startSec)));
      if (state.subMode === "translation" && !seg.translation) {
        topLeft.appendChild(el("span", "pill", d.untranslatedBadge));
      }
      top.appendChild(topLeft);
      const copyBtn = el("button", "btn small", d.copyQuote);
      copyBtn.type = "button";
      copyBtn.addEventListener("click", (ev) => {
        ev.stopPropagation();
        const quote = `"${seg.text}"${seg.translation ? "\n" + seg.translation : ""}\n— ${ep.feedTitle || ""} · ${ep.title} [${formatClock(seg.startSec)}]`;
        copyText(quote);
      });
      top.appendChild(copyBtn);
      row.appendChild(top);

      if (state.subMode === "bilingual") {
        const orig = el("div", "cue-orig");
        appendWordClickableText(orig, seg.text || "", seg, ep);
        row.appendChild(orig);
        if (seg.translation) {
          row.appendChild(el("div", "cue-trans", seg.translation));
        }
      } else if (state.subMode === "original") {
        const orig = el("div", "cue-orig");
        appendWordClickableText(orig, seg.text || "", seg, ep);
        row.appendChild(orig);
      } else {
        if (seg.translation) {
          row.appendChild(el("div", "cue-trans-primary", seg.translation));
        } else {
          const origFallback = el("div", "cue-orig fallback");
          appendWordClickableText(origFallback, seg.text || "", seg, ep);
          row.appendChild(origFallback);
        }
      }

      row.addEventListener("click", () => {
        seekAbsolute(seg.startSec);
      });
      container.appendChild(row);
    }
    highlightActiveCue();
  }

  function highlightActiveCue() {
    updateLiveCaption(false);
    const cueList = document.getElementById("cue-list");
    if (!cueList || state.currentEpId !== state.selectedEpId) return;
    const now = Number(audioEl.currentTime) || 0;
    const rows = cueList.querySelectorAll(".cue-row");
    let matched = null;
    for (const row of rows) {
      const st = Number(row.dataset.start) || 0;
      const en = Number(row.dataset.end) || st + 5;
      const active = now >= st && now <= en + 0.25;
      row.classList.toggle("active", active);
      if (active && !matched) matched = row;
    }
    if (matched && state.autoScroll) {
      matched.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }

  function renderGuideTab(ep, det) {
    const d = t();
    const bar = el("div", "section-bar");
    const genBtn = el("button", "btn small primary", det.guide ? d.regenerateGuide : d.generateGuide);
    genBtn.type = "button";
    genBtn.addEventListener("click", async () => {
      const res = await window.hyla.rpc("podcast.aiGuide", { episodeId: ep.id });
      if (res && res.job) state.jobs[res.job.id] = res.job;
      renderJobPill();
    });
    bar.appendChild(genBtn);
    mainEl.appendChild(bar);

    if (!det.guide) {
      mainEl.appendChild(el("div", "empty-box", d.noGuide));
      return;
    }
    const g = det.guide;
    const box = el("div", "guide-box");
    if (g.oneLiner) {
      box.appendChild(el("div", "guide-h", d.oneLiner));
      box.appendChild(el("div", "", g.oneLiner));
    }
    if (Array.isArray(g.takeaways) && g.takeaways.length > 0) {
      box.appendChild(el("div", "guide-h", d.takeaways));
      const ul = el("ul", "guide-list");
      for (const item of g.takeaways) ul.appendChild(el("li", "", item));
      box.appendChild(ul);
    }
    if (Array.isArray(g.timeline) && g.timeline.length > 0) {
      box.appendChild(el("div", "guide-h", d.timeline));
      for (const tl of g.timeline) {
        const row = el("div", "");
        const chip = el("button", "ts-chip", `[${formatClock(tl.timeSec)}]`);
        chip.type = "button";
        chip.addEventListener("click", () => seekAbsolute(tl.timeSec));
        row.appendChild(chip);
        row.appendChild(el("strong", "", " " + tl.title));
        if (tl.summary) row.appendChild(el("div", "card-meta", tl.summary));
        box.appendChild(row);
      }
    }
    if (Array.isArray(g.quotes) && g.quotes.length > 0) {
      box.appendChild(el("div", "guide-h", d.quotes));
      for (const q of g.quotes) {
        const row = el("div", "");
        const chip = el("button", "ts-chip", `[${formatClock(q.timeSec)}]`);
        chip.type = "button";
        chip.addEventListener("click", () => seekAbsolute(q.timeSec));
        row.appendChild(chip);
        row.appendChild(document.createTextNode(" “" + q.text + "”"));
        if (q.note) row.appendChild(el("div", "card-meta", q.note));
        box.appendChild(row);
      }
    }
    mainEl.appendChild(box);
  }

  function appendTimestampRichText(container, text) {
    const re = /\[(\d{1,2}:\d{2}(?::\d{2})?)\]/g;
    let last = 0;
    let m;
    while ((m = re.exec(text)) !== null) {
      if (m.index > last) container.appendChild(document.createTextNode(text.slice(last, m.index)));
      const sec = parseClock(m[1]);
      const btn = el("button", "ts-chip", `[${m[1]}]`);
      btn.type = "button";
      btn.addEventListener("click", () => seekAbsolute(sec));
      container.appendChild(btn);
      last = m.index + m[0].length;
    }
    if (last < text.length) container.appendChild(document.createTextNode(text.slice(last)));
  }

  function renderQATab(ep, det) {
    const d = t();
    const starters = el("div", "chips");
    for (const sText of [d.starter1, d.starter2, d.starter3]) {
      const c = el("button", "chip", sText);
      c.type = "button";
      c.addEventListener("click", () => sendQuestion(ep.id, sText));
      starters.appendChild(c);
    }
    mainEl.appendChild(starters);

    const thread = el("div", "chat-thread");
    for (const msg of det.chat || []) {
      const box = el("div", "chat-msg " + (msg.role === "user" ? "user" : "assistant"));
      box.appendChild(el("div", "chat-msg-role", msg.role === "user" ? "You" : "AI"));
      const body = el("div", "");
      appendTimestampRichText(body, msg.content || "");
      box.appendChild(body);
      thread.appendChild(box);
    }
    mainEl.appendChild(thread);

    const form = el("form", "section-bar");
    const inEl = el("input", "input grow");
    inEl.type = "text";
    inEl.placeholder = d.askPlaceholder;
    const sendBtn = el("button", "btn small primary", d.send);
    sendBtn.type = "submit";
    sendBtn.disabled = state.askingAI;
    const clrBtn = el("button", "btn small", d.clearChat);
    clrBtn.type = "button";
    clrBtn.addEventListener("click", async () => {
      const res = await window.hyla.rpc("podcast.aiClearChat", { episodeId: ep.id });
      if (state.epDetail) state.epDetail.chat = (res && res.chat) || [];
      render();
    });
    form.appendChild(inEl);
    form.appendChild(sendBtn);
    form.appendChild(clrBtn);
    form.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const q = inEl.value.trim();
      if (!q) return;
      inEl.value = "";
      await sendQuestion(ep.id, q);
    });
    mainEl.appendChild(form);
  }

  async function sendQuestion(epId, question) {
    if (state.askingAI) return;
    state.askingAI = true;
    render();
    try {
      const res = await window.hyla.rpc("podcast.aiAsk", { episodeId: epId, question });
      if (state.epDetail && res && res.chat) state.epDetail.chat = res.chat;
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    } finally {
      state.askingAI = false;
      render();
    }
  }

  function renderNotesTab(ep, det) {
    const d = t();
    const chapters = (det.chapters && det.chapters.length > 0) ? det.chapters : (ep.chapters || []);
    if (chapters.length > 0) {
      const box = el("div", "notes-box");
      box.style.marginBottom = "10px";
      box.appendChild(el("div", "guide-h", d.chapters));
      for (const ch of chapters) {
        const row = el("div", "");
        const chip = el("button", "ts-chip", `[${formatClock(ch.startSec)}]`);
        chip.type = "button";
        chip.addEventListener("click", () => seekAbsolute(ch.startSec));
        row.appendChild(chip);
        row.appendChild(document.createTextNode(" " + ch.title));
        box.appendChild(row);
      }
      mainEl.appendChild(box);
    }
    const notes = el("div", "notes-box");
    notes.appendChild(el("div", "guide-h", d.notes));
    const pre = el("div", "");
    pre.style.whiteSpace = "pre-wrap";
    pre.style.lineHeight = "1.55";
    pre.textContent = ep.description || ep.summary || "";
    notes.appendChild(pre);
    mainEl.appendChild(notes);
  }

  function renderVocabAndStats() {
    const d = t();
    const completedCount = state.episodes.filter((e) => e.completed).length;

    const sGrid = el("div", "stats-grid");
    for (const [val, lbl] of [
      [formatHours(state.stats.totalSec || 0), d.totalTime],
      [String(completedCount), d.completedEps],
      [String(state.vocab.length), d.savedWords]
    ]) {
      const c = el("div", "stat-card");
      c.appendChild(el("div", "stat-val", val));
      c.appendChild(el("div", "stat-lbl", lbl));
      sGrid.appendChild(c);
    }
    mainEl.appendChild(sGrid);

    const hourly = state.stats.hourlySec || [];
    const maxH = Math.max(1, ...hourly);
    const clkCard = el("div", "guide-box");
    clkCard.appendChild(el("div", "guide-h", d.hourlyChart));
    const bars = el("div", "clock-bars");
    for (let h = 0; h < 24; h++) {
      const bar = el("div", "clock-bar");
      const v = hourly[h] || 0;
      bar.style.height = `${Math.max(4, Math.round((v / maxH) * 46))}px`;
      bar.title = `${String(h).padStart(2, "0")}:00 · ${formatHours(v)}`;
      bars.appendChild(bar);
    }
    clkCard.appendChild(bars);

    const showEntries = Object.entries(state.stats.showSec || {}).sort((a, b) => b[1] - a[1]).slice(0, 5);
    if (showEntries.length > 0) {
      clkCard.appendChild(el("div", "guide-h", d.topShows));
      for (const [name, sec] of showEntries) {
        clkCard.appendChild(el("div", "card-meta", `${name} · ${formatHours(sec)}`));
      }
    }
    mainEl.appendChild(clkCard);

    const vBar = el("div", "section-bar");
    vBar.style.marginTop = "12px";
    vBar.appendChild(el("strong", "", `${d.vocabNotebook} (${state.vocab.length})`));
    if (state.vocab.length > 0) {
      const revBtn = el("button", "btn small primary", d.reviewCards);
      revBtn.type = "button";
      revBtn.addEventListener("click", openFlashcardModal);
      vBar.appendChild(revBtn);

      const csvBtn = el("button", "btn small", d.exportCSV);
      csvBtn.type = "button";
      csvBtn.addEventListener("click", async () => {
        const res = await window.hyla.rpc("podcast.vocabExportCSV", {});
        openExportModal(d.exportCSV, res.csv || "");
      });
      vBar.appendChild(csvBtn);
    }
    mainEl.appendChild(vBar);

    if (state.vocab.length === 0) {
      mainEl.appendChild(el("div", "empty-box", d.emptyVocab));
      return;
    }
    const list = el("div", "card-list");
    for (const v of state.vocab) {
      const card = el("div", "card");
      const body = el("div", "card-body");
      const top = el("div", "card-meta");
      top.appendChild(el("strong", "", v.word));
      if (v.phonetic) top.appendChild(el("span", "", v.phonetic));
      if (v.reviewCount > 0) top.appendChild(el("span", "pill", `×${v.reviewCount}`));
      body.appendChild(top);
      body.appendChild(el("div", "", v.meaning));
      if (v.sentence) body.appendChild(el("div", "card-meta", v.sentence));
      const acts = el("div", "card-actions");
      if (v.episodeId) {
        const jumpBtn = el("button", "btn small", `[${formatClock(v.timestampSec || 0)}] ${v.episodeTitle || ""}`);
        jumpBtn.type = "button";
        jumpBtn.addEventListener("click", () => openEpisode(v.episodeId, true, v.timestampSec || 0));
        acts.appendChild(jumpBtn);
      }
      const delBtn = el("button", "btn small danger", d.deleteOffline);
      delBtn.type = "button";
      delBtn.textContent = "×";
      delBtn.addEventListener("click", async () => {
        const res = await window.hyla.rpc("podcast.vocabDelete", { id: v.id });
        if (res && res.vocab) state.vocab = res.vocab;
        render();
      });
      acts.appendChild(delBtn);
      body.appendChild(acts);
      card.appendChild(body);
      list.appendChild(card);
    }
    mainEl.appendChild(list);
  }

  function renderMiniPlayer() {
    const ep = findEpisode(state.currentEpId);
    if (!ep) {
      miniPlayerEl.hidden = true;
      miniPlayerEl.textContent = "";
      return;
    }
    const d = t();
    miniPlayerEl.hidden = false;
    miniPlayerEl.textContent = "";
    miniPlayerEl.appendChild(makeCover(ep.coverUrl, ep.feedTitle || ep.title, "small"));

    const meta = el("div", "mini-meta");
    meta.appendChild(el("div", "mini-title", ep.title));
    const statusLabel = (state.buffering && !audioEl.paused) ? ` · ${d.buffering}` : "";
    meta.appendChild(el("div", "mini-sub", `${ep.feedTitle || ""} · ${formatClock(audioEl.currentTime || 0)} / ${formatClock(audioEl.duration || ep.durationSec || 0)}${statusLabel}`));
    meta.addEventListener("click", () => openEpisode(ep.id, false));
    miniPlayerEl.appendChild(meta);

    const ppLabel = !audioEl.paused && state.buffering ? d.buffering : (audioEl.paused ? d.play : d.pause);
    const pp = el("button", "btn small primary", ppLabel);
    pp.type = "button";
    pp.addEventListener("click", togglePlayPause);
    miniPlayerEl.appendChild(pp);

    const spd = el("button", "btn small", `${state.playbackRate}x`);
    spd.type = "button";
    spd.addEventListener("click", cycleSpeed);
    miniPlayerEl.appendChild(spd);
  }

  function cycleSpeed() {
    const rates = [0.75, 1.0, 1.25, 1.5, 1.75, 2.0];
    const idx = rates.indexOf(state.playbackRate);
    state.playbackRate = rates[(idx + 1) % rates.length];
    audioEl.playbackRate = state.playbackRate;
    renderMiniPlayer();
    if (state.selectedEpId) render();
  }

  function copyText(txt) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(txt).catch(() => {});
    }
  }

  function openModal(title, buildBody) {
    modalBackdropEl.hidden = false;
    modalBackdropEl.textContent = "";
    const box = el("div", "modal");
    const hdr = el("div", "modal-hdr");
    hdr.appendChild(el("div", "modal-title", title));
    const cls = el("button", "btn small", t().close);
    cls.type = "button";
    cls.addEventListener("click", closeModal);
    hdr.appendChild(cls);
    box.appendChild(hdr);
    buildBody(box);
    modalBackdropEl.appendChild(box);
  }

  function closeModal() {
    modalBackdropEl.hidden = true;
    modalBackdropEl.textContent = "";
  }

  modalBackdropEl.addEventListener("click", (ev) => {
    if (ev.target === modalBackdropEl) closeModal();
  });

  function openExportModal(title, text) {
    const d = t();
    openModal(title, (box) => {
      const ta = el("textarea", "textarea");
      ta.rows = 12;
      ta.readOnly = true;
      ta.value = text;
      box.appendChild(ta);
      const row = el("div", "card-actions");
      const cp = el("button", "btn small primary", d.copy);
      cp.type = "button";
      cp.addEventListener("click", () => {
        ta.select();
        copyText(ta.value);
        cp.textContent = d.copied;
      });
      row.appendChild(cp);
      box.appendChild(row);
    });
  }

  function openExportTranscriptModal(epId) {
    const d = t();
    openModal(d.exportSub, (box) => {
      const fmtSel = el("select", "select");
      for (const f of ["srt", "vtt", "md", "txt"]) fmtSel.appendChild(el("option", "", f.toUpperCase())).value = f;
      const modeSel = el("select", "select");
      for (const [mv, ml] of [["bilingual", d.bilingual], ["translation", d.translationOnly], ["original", d.originalOnly]]) {
        const o = el("option", "", ml);
        o.value = mv;
        modeSel.appendChild(o);
      }
      const ta = el("textarea", "textarea");
      ta.rows = 10;
      ta.readOnly = true;

      async function loadPreview() {
        const res = await window.hyla.rpc("podcast.exportTranscript", {
          episodeId: epId,
          format: fmtSel.value,
          mode: modeSel.value
        });
        ta.value = (res && res.content) || "";
      }
      fmtSel.addEventListener("change", loadPreview);
      modeSel.addEventListener("change", loadPreview);

      const bar = el("div", "section-bar");
      bar.appendChild(fmtSel);
      bar.appendChild(modeSel);
      box.appendChild(bar);
      box.appendChild(ta);

      const row = el("div", "card-actions");
      const cp = el("button", "btn small primary", d.copy);
      cp.type = "button";
      cp.addEventListener("click", () => {
        ta.select();
        copyText(ta.value);
        cp.textContent = d.copied;
      });
      row.appendChild(cp);
      box.appendChild(row);
      loadPreview();
    });
  }

  function openImportTranscriptModal(epId) {
    const d = t();
    openModal(d.importSub, (box) => {
      box.appendChild(el("div", "form-label", d.pasteTranscript));
      const ta = el("textarea", "textarea");
      ta.rows = 10;
      box.appendChild(ta);
      const btn = el("button", "btn small primary", d.importBtn);
      btn.type = "button";
      btn.addEventListener("click", async () => {
        const res = await window.hyla.rpc("podcast.importTranscript", { episodeId: epId, content: ta.value });
        if (res && res.detail) state.epDetail = res.detail;
        closeModal();
        render();
      });
      box.appendChild(btn);
    });
  }

  function openImportOPMLModal() {
    const d = t();
    openModal(d.importOPML, (box) => {
      box.appendChild(el("div", "form-label", d.pasteOPML));
      const ta = el("textarea", "textarea");
      ta.rows = 10;
      box.appendChild(ta);
      const btn = el("button", "btn small primary", d.importBtn);
      btn.type = "button";
      btn.addEventListener("click", async () => {
        btn.disabled = true;
        try {
          const res = await window.hyla.rpc("podcast.importOPML", { xml: ta.value });
          if (res && res.bootstrap) applyBootstrap(res.bootstrap);
          closeModal();
          render();
        } catch (err) {
          showBanner(err.message || String(err), { error: true });
          btn.disabled = false;
        }
      });
      box.appendChild(btn);
    });
  }

  function openWordModal(word, seg, ep) {
    const d = t();
    openModal(d.wordCardTitle, async (box) => {
      const loading = el("div", "card-meta", `${word}…`);
      box.appendChild(loading);
      try {
        const res = await window.hyla.rpc("podcast.wordLookup", {
          word,
          sentence: seg.text || "",
          targetLang: targetLangCode()
        });
        loading.remove();
        const hdr = el("div", "section-bar");
        hdr.appendChild(el("strong", "", res.word || word));
        if (res.phonetic) hdr.appendChild(el("span", "card-meta", res.phonetic));
        if (res.partOfSpeech) hdr.appendChild(el("span", "pill", res.partOfSpeech));
        if (res.audioUrl && safeHttpUrl(res.audioUrl)) {
          const audBtn = el("button", "btn small", "🔊 " + d.pronounce);
          audBtn.type = "button";
          audBtn.addEventListener("click", () => {
            new Audio(res.audioUrl).play().catch(() => {});
          });
          hdr.appendChild(audBtn);
        }
        box.appendChild(hdr);
        if (res.meaning) box.appendChild(el("div", "", res.meaning));
        if (res.contextNote) box.appendChild(el("div", "card-meta", res.contextNote));
        for (const def of res.definitions || []) {
          box.appendChild(el("div", "card-meta", "• " + def));
        }

        const saveBtn = el("button", "btn small primary", d.saveVocab);
        saveBtn.type = "button";
        saveBtn.addEventListener("click", async () => {
          const vRes = await window.hyla.rpc("podcast.vocabAdd", {
            word: res.word || word,
            phonetic: res.phonetic || "",
            meaning: res.meaning || (res.definitions && res.definitions[0]) || "",
            sentence: seg.text || "",
            translation: seg.translation || "",
            episodeId: ep.id,
            episodeTitle: ep.title,
            timestampSec: seg.startSec || 0
          });
          if (vRes && vRes.vocab) state.vocab = vRes.vocab;
          saveBtn.textContent = d.savedVocab;
          saveBtn.disabled = true;
        });
        box.appendChild(saveBtn);
      } catch (err) {
        loading.textContent = err.message || String(err);
      }
    });
  }

  function openFlashcardModal() {
    const d = t();
    let idx = 0;
    let revealed = false;
    openModal(d.reviewCards, (box) => {
      const cardHost = el("div", "guide-box");
      box.appendChild(cardHost);

      function drawCard() {
        cardHost.textContent = "";
        if (state.vocab.length === 0) {
          closeModal();
          return;
        }
        const v = state.vocab[idx % state.vocab.length];
        cardHost.appendChild(el("strong", "", v.word + (v.phonetic ? " " + v.phonetic : "")));
        if (v.sentence) cardHost.appendChild(el("div", "card-meta", v.sentence));
        if (revealed) {
          cardHost.appendChild(el("div", "", v.meaning));
          if (v.translation) cardHost.appendChild(el("div", "card-meta", v.translation));
        }
        const row = el("div", "card-actions");
        if (!revealed) {
          const rev = el("button", "btn small primary", d.reveal);
          rev.type = "button";
          rev.addEventListener("click", () => { revealed = true; drawCard(); });
          row.appendChild(rev);
        } else {
          const ok = el("button", "btn small primary", d.markReviewed);
          ok.type = "button";
          ok.addEventListener("click", async () => {
            const res = await window.hyla.rpc("podcast.vocabReview", { id: v.id });
            if (res && res.vocab) state.vocab = res.vocab;
            idx++;
            revealed = false;
            drawCard();
          });
          row.appendChild(ok);
        }
        const nxt = el("button", "btn small", d.nextCard);
        nxt.type = "button";
        nxt.addEventListener("click", () => { idx++; revealed = false; drawCard(); });
        row.appendChild(nxt);
        cardHost.appendChild(row);
      }
      drawCard();
    });
  }

  function openSettingsModal() {
    const d = t();
    const s = state.settings || {};
    openModal(d.settingsTitle, (box) => {
      const autoRow = el("label", "section-bar");
      const autoChk = document.createElement("input");
      autoChk.type = "checkbox";
      autoChk.checked = s.autoPauseOnAgent !== false;
      autoRow.appendChild(autoChk);
      autoRow.appendChild(el("span", "", d.autoPauseLabel));
      box.appendChild(autoRow);

      function addSelect(label, val, opts) {
        const w = el("div", "form-row");
        w.appendChild(el("label", "form-label", label));
        const sel = el("select", "select");
        for (const o of opts) {
          const opt = el("option", "", o);
          opt.value = o;
          if (o === val) opt.selected = true;
          sel.appendChild(opt);
        }
        w.appendChild(sel);
        box.appendChild(w);
        return sel;
      }

      function addInput(label, val, placeholder, type) {
        const w = el("div", "form-row");
        w.appendChild(el("label", "form-label", label));
        const inp = el("input", "input");
        inp.type = type || "text";
        inp.value = val || "";
        inp.placeholder = placeholder || "";
        w.appendChild(inp);
        box.appendChild(w);
        return inp;
      }

      const provSel = addSelect(d.aiProviderLabel, s.aiProvider || "deepseek", ["deepseek", "openai", "openrouter", "siliconflow", "kimi", "ollama", "custom"]);
      const aiUrlIn = addInput(d.aiBaseUrlLabel, s.aiBaseUrl || "", "https://api.deepseek.com/v1");
      const aiModelIn = addInput(d.aiModelLabel, s.aiModel || "", "deepseek-chat");
      const aiKeyIn = addInput(d.aiKeyLabel, "", s.hasAiKey ? d.keySaved : "sk-...", "password");
      const trSel = addSelect(d.translateEngineLabel, s.translateEngine || "google", ["google", "ai"]);
      const asrUrlIn = addInput(d.asrBaseUrlLabel, s.asrBaseUrl || "", "https://api.groq.com/openai/v1");
      const asrModelIn = addInput(d.asrModelLabel, s.asrModel || "whisper-large-v3-turbo", "whisper-large-v3-turbo");
      const asrKeyIn = addInput(d.asrKeyLabel, "", s.hasAsrKey ? d.keySaved : "gsk_...", "password");
      const proxyIn = addInput(d.proxyLabel, s.httpProxy || "", "http://127.0.0.1:7890");

      const saveBtn = el("button", "btn small primary", d.save);
      saveBtn.type = "button";
      saveBtn.addEventListener("click", async () => {
        saveBtn.disabled = true;
        try {
          const payload = {
            autoPauseOnAgent: autoChk.checked,
            aiProvider: provSel.value,
            aiBaseUrl: aiUrlIn.value,
            aiModel: aiModelIn.value,
            translateEngine: trSel.value,
            asrBaseUrl: asrUrlIn.value,
            asrModel: asrModelIn.value,
            httpProxy: proxyIn.value
          };
          if (aiKeyIn.value.trim()) payload.aiApiKey = aiKeyIn.value.trim();
          if (asrKeyIn.value.trim()) payload.asrApiKey = asrKeyIn.value.trim();
          const res = await window.hyla.rpc("podcast.saveSettings", payload);
          if (res && res.settings) state.settings = res.settings;
          closeModal();
        } catch (err) {
          showBanner(err.message || String(err), { error: true });
          saveBtn.disabled = false;
        }
      });
      box.appendChild(saveBtn);
    });
  }

  function render() {
    renderTopbar();
    renderBanner();
    mainEl.textContent = "";
    if (state.selectedEpId) {
      renderEpisodeView();
    } else if (state.tab === "discover") {
      renderDiscover();
    } else if (state.tab === "library") {
      renderLibrary();
    } else if (state.tab === "vocab") {
      renderVocabAndStats();
    } else {
      renderHome();
    }
    renderMiniPlayer();
  }

  // Detect CSP blocks on local media streaming (e.g. running an older Hyla binary before WebView CSP update).
  document.addEventListener("securitypolicyviolation", (ev) => {
    const dir = String((ev && (ev.effectiveDirective || ev.violatedDirective)) || "").toLowerCase();
    const uri = String((ev && ev.blockedURI) || "");
    if (dir.startsWith("media-src") || (dir.startsWith("default-src") && (uri.includes("/audio") || uri.startsWith("http://127.0.0.1") || uri.startsWith("http://localhost")))) {
      state.cspMediaBlocked = true;
      showBanner(t().cspMediaBlocked, { error: true });
    }
  });

  // Audio events
  audioEl.addEventListener("error", () => {
    if (!state.currentEpId) return;
    state.playing = false;
    state.buffering = false;
    state.lastTickAt = 0;
    showBanner(describePlaybackError(null), { error: true });
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("loadstart", () => {
    if (!state.currentEpId) return;
    state.buffering = true;
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("waiting", () => {
    if (!state.currentEpId || audioEl.paused) return;
    state.buffering = true;
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("stalled", () => {
    if (!state.currentEpId || audioEl.paused) return;
    state.buffering = true;
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("canplay", () => {
    if (!state.currentEpId) return;
    state.buffering = false;
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("playing", () => {
    if (!state.currentEpId) return;
    state.playing = true;
    state.buffering = false;
    state.lastTickAt = Date.now();
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("timeupdate", () => {
    const now = Date.now();
    if (!audioEl.paused && state.lastTickAt > 0) {
      const dt = (now - state.lastTickAt) / 1000;
      if (dt > 0 && dt < 5) state.playedAccSec += dt;
    }
    state.lastTickAt = now;

    if (state.selectedEpId === state.currentEpId) {
      const curLbl = document.getElementById("player-cur-lbl");
      const slider = document.getElementById("player-slider");
      if (curLbl) curLbl.textContent = formatClock(audioEl.currentTime || 0);
      if (slider && document.activeElement !== slider) slider.value = String(Math.floor(audioEl.currentTime || 0));
      highlightActiveCue();
    }
    if (now - state.lastSaveAt > 5000) {
      flushProgress(false);
    }
    renderMiniPlayer();
  });

  audioEl.addEventListener("loadedmetadata", () => {
    if (state.selectedEpId === state.currentEpId) {
      const durLbl = document.getElementById("player-dur-lbl");
      const slider = document.getElementById("player-slider");
      if (durLbl) durLbl.textContent = formatClock(audioEl.duration || 0);
      if (slider && audioEl.duration) slider.max = String(Math.floor(audioEl.duration));
    }
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("pause", () => {
    state.playing = false;
    state.buffering = false;
    state.lastTickAt = 0;
    flushProgress(false);
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("play", () => {
    state.playing = true;
    state.lastTickAt = Date.now();
    updatePlaybackIndicators();
  });

  audioEl.addEventListener("ended", async () => {
    state.playing = false;
    state.buffering = false;
    state.lastTickAt = 0;
    await flushProgress(true);
    if (state.queue.length > 0) {
      const nextId = state.queue.shift();
      await window.hyla.rpc("podcast.queueSet", { queue: state.queue }).catch(() => {});
      const nextEp = findEpisode(nextId);
      if (nextEp) {
        await openEpisode(nextId, true);
        return;
      }
    }
    render();
  });

  // Topbar actions
  refreshBtn.addEventListener("click", async () => {
    const res = await window.hyla.rpc("podcast.refreshFeeds", {}).catch((err) => {
      showBanner(err.message || String(err), { error: true });
      return null;
    });
    if (res && res.job) state.jobs[res.job.id] = res.job;
    renderJobPill();
  });
  settingsBtn.addEventListener("click", openSettingsModal);

  // Keyboard shortcuts
  document.addEventListener("keydown", (ev) => {
    if (!modalBackdropEl.hidden) {
      if (ev.key === "Escape") closeModal();
      return;
    }
    if (ev.metaKey || ev.ctrlKey || ev.altKey) return;
    const tag = (ev.target && ev.target.tagName) || "";
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || (ev.target && ev.target.isContentEditable)) return;

    if (ev.code === "Space") {
      ev.preventDefault();
      togglePlayPause();
    } else if (ev.key === "ArrowLeft") {
      ev.preventDefault();
      seekRelative(ev.shiftKey ? -5 : -15);
    } else if (ev.key === "ArrowRight") {
      ev.preventDefault();
      seekRelative(ev.shiftKey ? 5 : 30);
    } else if (ev.key === "ArrowUp" || ev.key === "ArrowDown") {
      const segs = state.epDetail && state.epDetail.segments;
      if (state.selectedEpId && state.epSubTab === "subtitles" && Array.isArray(segs) && segs.length > 0) {
        ev.preventDefault();
        const cur = Number(audioEl.currentTime) || 0;
        let idx = segs.findIndex((s) => cur >= s.startSec && cur <= s.endSec + 0.25);
        if (idx < 0) {
          idx = segs.findIndex((s) => s.startSec > cur);
          if (idx < 0) idx = segs.length - 1;
        }
        const nextIdx = Math.max(0, Math.min(segs.length - 1, idx + (ev.key === "ArrowUp" ? -1 : 1)));
        seekAbsolute(segs[nextIdx].startSec);
      }
    } else if (ev.key === "c" || ev.key === "C") {
      if (state.selectedEpId) {
        const prevScroll = mainEl.scrollTop;
        state.subMode = state.subMode === "bilingual" ? "translation" : state.subMode === "translation" ? "original" : "bilingual";
        render();
        mainEl.scrollTop = prevScroll;
      }
    } else if (ev.key === "s" || ev.key === "S") {
      cycleSpeed();
    }
  });

  // Host push events
  if (window.hyla && typeof window.hyla.on === "function") {
    window.hyla.on("podcast.autoPause", () => {
      if (!audioEl.paused && state.currentEpId) {
        audioEl.pause();
        state.playing = false;
        state.buffering = false;
        flushProgress(false);
        showBanner(t().autoPaused, {
          actionLabel: t().resume,
          onAction: () => {
            clearBanner();
            togglePlayPause();
          }
        });
        updatePlaybackIndicators();
      }
    });

    window.hyla.on("podcast.job", async (job) => {
      if (!job || !job.id) return;
      if (job.state === "running") {
        state.jobs[job.id] = job;
        renderJobPill();
        if (job.kind === "translate" && state.selectedEpId && job.targetId === state.selectedEpId && state.epDetail) {
          const ep = findEpisode(state.selectedEpId);
          if (Array.isArray(job.segments) && job.segments.length > 0) {
            const prevScroll = mainEl.scrollTop;
            state.epDetail.segments = job.segments;
            const cueListEl = document.getElementById("cue-list");
            if (cueListEl && ep) renderCueList(ep, state.epDetail, cueListEl);
            updateLiveCaption(true, ep, state.epDetail);
            mainEl.scrollTop = prevScroll;
          } else {
            updateLiveCaption(true, ep, state.epDetail);
          }
          renderSubStatusBox(ep, state.epDetail, document.getElementById("sub-status"));
        }
        return;
      }
      delete state.jobs[job.id];
      renderJobPill();
      if (job.state === "error") {
        showBanner(job.error || "Job failed", { error: true });
        if (state.selectedEpId && job.targetId === state.selectedEpId && state.epDetail) {
          const ep = findEpisode(state.selectedEpId);
          renderSubStatusBox(ep, state.epDetail, document.getElementById("sub-status"));
          updateLiveCaption(true, ep, state.epDetail);
        }
        return;
      }
      if (job.episode) upsertEpisode(job.episode);
      if (job.storage) state.storage = job.storage;
      if (job.kind === "refresh") {
        await bootstrap();
        return;
      }
      if (state.selectedEpId && job.targetId === state.selectedEpId && state.epDetail) {
        if (job.segments) state.epDetail.segments = job.segments;
        if (job.guide) state.epDetail.guide = job.guide;
      }
      const prevScroll = mainEl.scrollTop;
      render();
      mainEl.scrollTop = prevScroll;
    });

    window.hyla.on("podcast.settingsChanged", (data) => {
      if (data && data.settings) state.settings = data.settings;
    });

    window.hyla.on("locale.changed", (data) => {
      if (data && data.locale) locale = resolveLocaleKey(data.locale);
      render();
    });

    window.hyla.on("theme.changed", () => {
      renderTopbar();
    });
  }

  bootstrap();
})();
