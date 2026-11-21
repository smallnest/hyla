(() => {
  const I18N = {
    en: {
      title: "IPTV",
      allChannels: "Channels",
      favorites: "Favorites",
      recent: "Recent",
      sources: "Sources",
      allGroups: "All",
      searchPlaceholder: "Search channels (/ )…",
      addSource: "+ Add Source",
      refresh: "Refresh",
      settings: "Settings",
      welcomeTitle: "Live TV, Minimal & Crisp",
      welcomeSub: "Paste a single M3U link, a batch of M3U URLs (one per line), a web page hosting M3U links, or raw playlist text. Channels automatically merge with logos and backup lines.",
      sourceInputLabel: "M3U / TXT URL(s), Web Page URL, or Playlist Text",
      sourceInputPlaceholder: "https://raw.githubusercontent.com/best-fan/iptv-sources/master/cn_all_status.m3u8\n# One URL per line, or paste #EXTM3U content",
      sourceNameLabel: "Custom Name (optional for single source)",
      sourceNamePlaceholder: "My Live TV",
      importBtn: "Import Sources",
      importing: "Importing…",
      starterTitle: "One-Click Starter Sources",
      subscribePreset: "Add",
      addedNotice: "Imported {n} source(s)",
      lineLabel: "Line",
      prevChannel: "Prev",
      nextChannel: "Next",
      play: "Play",
      pause: "Pause",
      reload: "Reload",
      mute: "Mute",
      unmute: "Unmute",
      pip: "PiP",
      theater: "Theater",
      exitTheater: "Exit Theater",
      channelsDrawer: "Channels",
      closePlayer: "Close",
      buffering: "Buffering stream…",
      switchingLine: "Stream error — switching to backup Line {n}…",
      allLinesFailed: "All stream lines failed for this channel.",
      cspMediaBlocked: "Media playback was blocked by Hyla WebView CSP. Please update/rebuild Hyla.",
      noChannelsMatch: "No channels match the current filter.",
      emptyFavorites: "Click ★ on any channel card to pin your favorites here.",
      emptyRecent: "Channels you watch will appear here for instant zapping.",
      enabled: "Enabled",
      disabled: "Disabled",
      enable: "Enable",
      disable: "Disable",
      remove: "Remove",
      exportM3U: "Export Merged M3U",
      clearLogos: "Clear Logo Cache",
      logoStorage: "Cached logos",
      showMore: "Show More ({n} remaining)",
      settingsTitle: "IPTV Settings",
      proxyModeLabel: "Proxy Mode",
      httpProxyLabel: "Custom HTTP Proxy",
      autoFailoverLabel: "Auto-failover across backup stream lines",
      autoMuteLabel: "Auto-mute when coding agent finishes",
      userAgentLabel: "Custom User-Agent (optional)",
      save: "Save",
      close: "Close",
      copy: "Copy",
      copied: "Copied!",
      dismiss: "Dismiss",
      autoMutedBanner: "Muted because coding agent finished working",
      channelsUnit: "channels",
      guide: "Guide",
      guideNow: "On Now",
      guideOnNow: "ON NOW",
      guideNext: "Next: {title}",
      guideUpNext: "Up Next",
      guideEmpty: "No programmes for this channel yet.",
      guideLoading: "Loading the guide…",
      guideOff: "The playlist names no EPG guide: add one with x-tvg-url.",
      guideRefresh: "Refresh Guide",
      guideUpdated: "Guide updated {when}",
      guideAgo: "Guide: {when}",
      justNow: "just now",
      minutesAgo: "{n} min ago",
      hoursAgo: "{n} h ago",
      daysAgo: "{n} d ago",
      nowLive: "LIVE",
      progressPct: "{n}% done",
      yesterday: "Yesterday"
    },
    "zh-Hans": {
      title: "IPTV 直播",
      allChannels: "全部频道",
      favorites: "收藏",
      recent: "最近观看",
      sources: "节目源",
      allGroups: "全部",
      searchPlaceholder: "搜索频道 ( / )…",
      addSource: "+ 添加源",
      refresh: "刷新",
      settings: "设置",
      welcomeTitle: "纯净简约的桌面电视墙",
      welcomeSub: "支持输入单个或一组 M3U 链接（每行一个）、包含 M3U 的网页地址，或直接粘贴 M3U / #genre# 内容。重复频道将自动整合多条备用线路与高清台标。",
      sourceInputLabel: "M3U 链接（支持多行批量）、网页地址或播单原文",
      sourceInputPlaceholder: "https://raw.githubusercontent.com/best-fan/iptv-sources/master/cn_all_status.m3u8\n# 可换行粘贴多个 M3U 地址，或直接粘贴 #EXTM3U 文本",
      sourceNameLabel: "自定义名称（单个源可选填）",
      sourceNamePlaceholder: "国内 IPv4 直播源",
      importBtn: "导入直播源",
      importing: "正在解析导入…",
      starterTitle: "推荐开箱直播源（点击一键导入）",
      subscribePreset: "导入",
      addedNotice: "成功导入 {n} 个直播源",
      lineLabel: "线路",
      prevChannel: "上一台",
      nextChannel: "下一台",
      play: "播放",
      pause: "暂停",
      reload: "重连",
      mute: "静音",
      unmute: "取消静音",
      pip: "画中画",
      theater: "影院模式",
      exitTheater: "退出影院",
      channelsDrawer: "频道列表",
      closePlayer: "关闭",
      buffering: "正在缓冲直播流…",
      switchingLine: "当前线路异常，正自动切换至备用线路 {n}…",
      allLinesFailed: "该频道所有可用线路均播放失败。",
      cspMediaBlocked: "当前 Hyla 客户端 WebView CSP 拦截了本地流媒体，请更新并重启 Hyla。",
      noChannelsMatch: "没有找到匹配的电视频道。",
      emptyFavorites: "将鼠标悬停在频道卡片上点击 ☆ 即可收藏常看频道。",
      emptyRecent: "观看过的电视频道会按时间排列在这里，方便快速换台。",
      enabled: "已启用",
      disabled: "已停用",
      enable: "启用",
      disable: "停用",
      remove: "删除",
      exportM3U: "导出合并后的 M3U",
      clearLogos: "清理台标缓存",
      logoStorage: "已缓存台标",
      showMore: "加载更多（剩余 {n} 个频道）",
      settingsTitle: "IPTV 设置",
      proxyModeLabel: "网络代理模式",
      httpProxyLabel: "自定义 HTTP 代理",
      autoFailoverLabel: "当前线路异常时自动切换备用线路",
      autoMuteLabel: "编程 Agent 完成工作时自动静音",
      userAgentLabel: "自定义 User-Agent（可选）",
      save: "保存",
      close: "关闭",
      copy: "复制",
      copied: "已复制！",
      dismiss: "忽略",
      autoMutedBanner: "检测到编程 Agent 已完成工作，已自动静音直播",
      channelsUnit: "个频道",
      guide: "节目单",
      guideNow: "正在播出",
      guideOnNow: "正在播出",
      guideNext: "接下来：{title}",
      guideUpNext: "接下来",
      guideEmpty: "该频道暂无节目信息。",
      guideLoading: "正在加载节目单…",
      guideOff: "直播源未提供 EPG 节目单，可在 M3U 中添加 x-tvg-url。",
      guideRefresh: "刷新节目单",
      guideUpdated: "节目单更新于 {when}",
      guideAgo: "节目单：{when}",
      justNow: "刚刚",
      minutesAgo: "{n} 分钟前",
      hoursAgo: "{n} 小时前",
      daysAgo: "{n} 天前",
      nowLive: "直播中",
      progressPct: "已播 {n}%",
      yesterday: "昨天"
    },
    ja: {
      title: "IPTV",
      allChannels: "チャンネル",
      favorites: "お気に入り",
      recent: "履歴",
      sources: "ソース",
      allGroups: "すべて",
      searchPlaceholder: "チャンネル検索 ( / )…",
      addSource: "+ ソース追加",
      refresh: "更新",
      settings: "設定",
      welcomeTitle: "シンプルなライブ TV ウォール",
      welcomeSub: "単一または複数行の M3U URL、M3U を含む Web ページ、またはプレイリストテキストを貼り付けられます。同一チャンネルはロゴ・複数回線付きで自動統合されます。",
      sourceInputLabel: "M3U URL（複数行可）・Web ページ URL・プレイリスト本文",
      sourceInputPlaceholder: "https://raw.githubusercontent.com/best-fan/iptv-sources/master/cn_all_status.m3u8\n# 1行に1つ URL、または #EXTM3U テキストを貼り付け",
      sourceNameLabel: "表示名（単一ソース時は任意）",
      sourceNamePlaceholder: "ライブ TV",
      importBtn: "インポート",
      importing: "読み込み中…",
      starterTitle: "おすすめスターターソース",
      subscribePreset: "追加",
      addedNotice: "{n} 件のソースを読み込みました",
      lineLabel: "回線",
      prevChannel: "前へ",
      nextChannel: "次へ",
      play: "再生",
      pause: "一時停止",
      reload: "再読込",
      mute: "ミュート",
      unmute: "解除",
      pip: "PiP",
      theater: "シアター",
      exitTheater: "シアター終了",
      channelsDrawer: "チャンネル",
      closePlayer: "閉じる",
      buffering: "ストリームを読み込み中…",
      switchingLine: "エラーのため回線 {n} へ自動切替中…",
      allLinesFailed: "このチャンネルのすべての回線が再生に失敗しました。",
      cspMediaBlocked: "Hyla WebView の CSP によりブロックされました。Hyla を再ビルドして再起動してください。",
      noChannelsMatch: "一致するチャンネルがありません。",
      emptyFavorites: "チャンネルカード上の ☆ をクリックしてお気に入りに追加できます。",
      emptyRecent: "視聴したチャンネルがここに表示されます。",
      enabled: "有効",
      disabled: "無効",
      enable: "有効化",
      disable: "無効化",
      remove: "削除",
      exportM3U: "統合 M3U 書き出し",
      clearLogos: "ロゴキャッシュ削除",
      logoStorage: "ロゴキャッシュ",
      showMore: "さらに表示（残り {n}）",
      settingsTitle: "IPTV 設定",
      proxyModeLabel: "プロキシモード",
      httpProxyLabel: "カスタム HTTP プロキシ",
      autoFailoverLabel: "エラー時にバックアップ回線へ自動切替",
      autoMuteLabel: "エージェント完了時に自動ミュート",
      userAgentLabel: "カスタム User-Agent（任意）",
      save: "保存",
      close: "閉じる",
      copy: "コピー",
      copied: "コピーしました",
      dismiss: "閉じる",
      autoMutedBanner: "コーディングエージェント完了のため自動ミュートしました",
      channelsUnit: "チャンネル",
      guide: "番組表",
      guideNow: "放送中",
      guideOnNow: "放送中",
      guideNext: "次: {title}",
      guideUpNext: "このあと",
      guideEmpty: "このチャンネルの番組情報はまだありません。",
      guideLoading: "番組表を読み込み中…",
      guideOff: "プレイリストに EPG がありません: x-tvg-url で追加してください。",
      guideRefresh: "番組表を更新",
      guideUpdated: "番組表を {when} に更新",
      guideAgo: "番組表: {when}",
      justNow: "たった今",
      minutesAgo: "{n} 分前",
      hoursAgo: "{n} 時間前",
      daysAgo: "{n} 日前",
      nowLive: "LIVE",
      progressPct: "{n}% 経過",
      yesterday: "昨日"
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

  const PAGE_STEP = 120;
  const state = {
    tab: "channels", // "channels" | "favorites" | "recent" | "sources"
    group: "",
    query: "",
    sources: [],
    channels: [],
    groups: [],
    favorites: [],
    recent: [],
    presets: [],
    programs: {},
    epg: { state: "idle", updatedAt: 0, programs: 0, channels: 0, urls: [] },
    settings: { proxyMode: "auto", autoFailover: true, autoMuteOnAgent: false },
    storage: { logosCount: 0, logosBytes: 0 },
    jobs: {},
    currentChannelId: "",
    currentLineIdx: 0,
    triedLines: {},
    buffering: false,
    muted: false,
    volume: 1.0,
    resolution: "",
    theaterFull: false,
    drawerOpen: true,
    hudVisible: true,
    visibleLimit: PAGE_STEP,
    banner: null,
    cspBlocked: false
  };

  let videoEl = null;
  let hudHideTimer = null;
  let stallTimer = null;

  const appRootEl = document.getElementById("app-root");
  const brandTitleEl = document.getElementById("brand-title");
  const navTabsEl = document.getElementById("nav-tabs");
  const searchInputEl = document.getElementById("search-input");
  const addBtnEl = document.getElementById("add-btn");
  const refreshBtnEl = document.getElementById("refresh-btn");
  const settingsBtnEl = document.getElementById("settings-btn");
  const jobPillEl = document.getElementById("job-pill");
  const bannerEl = document.getElementById("banner");
  const theaterSectionEl = document.getElementById("theater-section");
  const mainEl = document.getElementById("main");
  const modalBackdropEl = document.getElementById("modal-backdrop");

  function el(tag, cls, text) {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  function formatBytes(b) {
    const n = Number(b) || 0;
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  }

  function monogramText(name) {
    const clean = String(name || "TV").replace(/\s+/g, "").trim();
    if (/^[A-Za-z0-9+.\-]+/.test(clean)) {
      return clean.slice(0, 6);
    }
    return clean.slice(0, 4) || "TV";
  }

  function gradientFor(seed) {
    const palettes = [
      ["#2563eb", "#7c3aed"],
      ["#0ea5e9", "#2563eb"],
      ["#ec4899", "#8b5cf6"],
      ["#10b981", "#059669"],
      ["#f59e0b", "#ef4444"],
      ["#6366f1", "#3b82f6"],
      ["#14b8a6", "#0284c7"],
      ["#f43f5e", "#e11d48"]
    ];
    let h = 0;
    const s = String(seed || "");
    for (let i = 0; i < s.length; i++) {
      h = ((h << 5) - h + s.charCodeAt(i)) | 0;
    }
    const pair = palettes[Math.abs(h) % palettes.length];
    return `linear-gradient(135deg, ${pair[0]}, ${pair[1]})`;
  }

  function buildLogoElement(logoProxyUrl, title, small, eager) {
    if (logoProxyUrl) {
      const pad = el("div", small ? "hud-logo-wrap" : "logo-pad");
      const img = el("img", small ? "" : "logo-img");
      img.src = logoProxyUrl;
      img.alt = "";
      img.decoding = "async";
      img.loading = (small || eager) ? "eager" : "lazy";
      img.addEventListener("error", () => {
        if (small) {
          pad.remove();
          return;
        }
        const mono = el("div", "logo-monogram", monogramText(title));
        mono.style.background = gradientFor(title);
        pad.replaceWith(mono);
      });
      pad.appendChild(img);
      return pad;
    }
    if (small) return null;
    const mono = el("div", "logo-monogram", monogramText(title));
    mono.style.background = gradientFor(title);
    return mono;
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
      const ab = el("button", "glass-btn small primary", state.banner.actionLabel);
      ab.type = "button";
      ab.addEventListener("click", state.banner.onAction);
      acts.appendChild(ab);
    }
    const db = el("button", "glass-btn small", t().dismiss);
    db.type = "button";
    db.addEventListener("click", clearBanner);
    acts.appendChild(db);
    bannerEl.appendChild(acts);
  }

  function applyBootstrap(snap) {
    if (!snap) return;
    if (snap.locale) locale = resolveLocaleKey(snap.locale);
    if (snap.programs) state.programs = snap.programs;
    if (snap.epg) state.epg = snap.epg;
    state.sources = snap.sources || [];
    state.channels = snap.channels || [];
    state.groups = snap.groups || [];
    state.favorites = snap.favorites || [];
    state.recent = snap.recent || [];
    state.presets = snap.presets || [];
    state.settings = snap.settings || state.settings;
    state.storage = snap.storage || state.storage;
    if (Array.isArray(snap.activeJobs)) {
      const nextJobs = {};
      for (const j of snap.activeJobs) nextJobs[j.id] = j;
      state.jobs = nextJobs;
    }
    if (state.group && !state.groups.includes(state.group)) {
      state.group = "";
    }
  }

  let epgPollTimer = null;

  async function bootstrap() {
    try {
      const snap = await window.hyla.rpc("iptv.bootstrap", {});
      applyBootstrap(snap);
      render();
      watchGuide();
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    }
  }

  // watchGuide reads the guide once more while a fetch runs or the guide
  // is empty; the plugin also sends an event when a fetch ends.
  function watchGuide() {
    if (epgPollTimer) {
      clearTimeout(epgPollTimer);
      epgPollTimer = null;
    }
    const urls = (state.epg && state.epg.urls) || [];
    const waiting = (state.epg && state.epg.state === "running") || (urls.length > 0 && !epgOn());
    if (!waiting) return;
    epgPollTimer = setTimeout(async () => {
      try {
        const res = await window.hyla.rpc("iptv.programs", {});
        if (res && res.programs) state.programs = res.programs;
        if (res && res.epg) state.epg = res.epg;
        render();
      } catch {}
      watchGuide();
    }, 1500);
  }

  // ---- Program guide ----

  function channelKey(ch) {
    return String((ch && ch.normName) || "").trim();
  }

  function programsFor(ch) {
    if (!ch) return [];
    return state.programs[channelKey(ch)] || [];
  }

  function hhmm(ms) {
    const d = new Date(ms);
    return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  }

  function dayLabel(ms) {
    const d = new Date(ms);
    const now = new Date();
    const key = (x) => `${x.getFullYear()}-${x.getMonth()}-${x.getDate()}`;
    if (key(d) === key(now)) return "";
    const y = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
    if (key(d) === key(y)) return t().yesterday;
    return `${d.getMonth() + 1}/${d.getDate()}`;
  }

  // programAt tells which programme is on at now, and which comes next.
  function programAt(list, now) {
    now = now || Date.now();
    let current = null;
    let next = null;
    for (let i = 0; i < list.length; i++) {
      const p = list[i];
      const stop = p.stop > 0 ? p.stop : (i + 1 < list.length ? list[i + 1].start : 0);
      if (p.start <= now && (stop === 0 || stop > now)) {
        current = { program: p, stop };
        next = i + 1 < list.length ? list[i + 1] : null;
        return { current, next };
      }
      if (p.start > now) {
        next = p;
        break;
      }
    }
    return { current, next };
  }

  function progressOf(program, stop, now) {
    if (!program || !program.stop || program.stop <= program.start) return 0;
    const pct = Math.round(((now - program.start) / (program.stop - program.start)) * 100);
    return Math.max(0, Math.min(100, Number.isFinite(pct) ? pct : 0));
  }

  function agoText(ms) {
    if (!ms) return "";
    const d = t();
    const diff = Math.max(0, Date.now() - ms);
    const min = Math.floor(diff / 60000);
    if (min < 1) return d.justNow;
    if (min < 60) return d.minutesAgo.replace("{n}", String(min));
    const hours = Math.floor(min / 60);
    if (hours < 24) return d.hoursAgo.replace("{n}", String(hours));
    return d.daysAgo.replace("{n}", String(Math.floor(hours / 24)));
  }

  function epgOn() {
    return Object.keys(state.programs).length > 0;
  }

  function renderNowRow() {
    const now = Date.now();
    const rows = [];
    for (const ch of state.channels) {
      const list = programsFor(ch);
      if (!list.length) continue;
      const { current, next } = programAt(list, now);
      if (!current) continue;
      rows.push({ ch, current, next });
    }
    if (rows.length === 0) return null;

    const wrap = el("section", "now-rail");
    const head = el("div", "now-head");
    head.appendChild(el("span", "now-dot"));
    head.appendChild(el("span", "now-title", t().guideNow));
    if (state.epg && state.epg.updatedAt) head.appendChild(el("span", "now-updated", t().guideAgo.replace("{when}", agoText(state.epg.updatedAt))));
    wrap.appendChild(head);

    const track = el("div", "now-track");
    for (const { ch, current, next } of rows) {
      const card = el("button", "now-card" + (ch.id === state.currentChannelId ? " playing" : ""));
      card.type = "button";
      card.title = next ? t().guideNext.replace("{title}", next.title) : "";
      const top = el("div", "now-card-top");
      const logo = buildLogoElement(ch.logoProxyUrl, ch.name, true);
      if (logo) top.appendChild(logo);
      top.appendChild(el("span", "now-card-ch", ch.name));
      const badge = el("span", "now-live", t().nowLive);
      top.appendChild(badge);
      card.appendChild(top);
      card.appendChild(el("div", "now-card-title", current.program.title));
      const times = el("div", "now-card-times", `${hhmm(current.program.start)} – ${current.program.stop ? hhmm(current.program.stop) : ""}`);
      card.appendChild(times);
      const bar = el("div", "now-bar");
      const fill = el("div", "now-bar-fill");
      fill.style.width = `${progressOf(current.program, current.stop, now)}%`;
      bar.appendChild(fill);
      card.appendChild(bar);
      card.addEventListener("click", () => playChannel(ch, state.currentLineIdx && ch.id === state.currentChannelId ? state.currentLineIdx : 0, true));
      track.appendChild(card);
    }
    wrap.appendChild(track);
    return wrap;
  }

  function renderGuideView() {
    const d = t();
    const now = Date.now();

    const tools = el("div", "guide-tools");
    const status = el("div", "guide-status");
    if (state.epg.state === "running") {
      status.textContent = d.guideLoading;
    } else if (state.epg.updatedAt) {
      status.textContent = d.guideUpdated.replace("{when}", agoText(state.epg.updatedAt));
    } else if (state.epg.urls && state.epg.urls.length === 0) {
      status.textContent = d.guideOff;
    }
    tools.appendChild(status);
    const refresh = el("button", "glass-btn small", d.guideRefresh);
    refresh.type = "button";
    refresh.addEventListener("click", async () => {
      refresh.disabled = true;
      try {
        const res = await window.hyla.rpc("iptv.refreshEPG", { force: true });
        if (res && res.epg) state.epg = res.epg;
      } catch (err) {
        showBanner(err.message || String(err), { error: true });
      }
      refresh.disabled = false;
      renderMain();
    });
    tools.appendChild(refresh);
    mainEl.appendChild(tools);

    const list = filteredChannels();
    if (list.length === 0) {
      mainEl.appendChild(el("div", "empty-box", d.noChannelsMatch));
      return;
    }

    const rows = el("div", "guide-rows");
    for (const ch of list) {
      const listP = programsFor(ch);
      const row = el("div", "guide-row" + (ch.id === state.currentChannelId ? " playing" : ""));
      const side = el("button", "guide-ch");
      side.type = "button";
      const logo = buildLogoElement(ch.logoProxyUrl, ch.name, true);
      if (logo) side.appendChild(logo);
      side.appendChild(el("span", "guide-ch-name", ch.name));
      side.addEventListener("click", () => playChannel(ch, ch.id === state.currentChannelId ? state.currentLineIdx : 0, true));
      row.appendChild(side);

      const track = el("div", "guide-track");
      if (listP.length === 0) {
        track.appendChild(el("span", "guide-empty", d.guideEmpty));
      } else {
        const { current } = programAt(listP, now);
        for (const p of listP.slice(0, 40)) {
          const isNow = current && current.program === p;
          const cell = el("button", "guide-prog" + (isNow ? " live" : "") + (p.stop > 0 && p.stop < now ? " past" : ""));
          cell.type = "button";
          cell.title = p.desc || p.category || p.title;
          const day = dayLabel(p.start);
          cell.appendChild(el("span", "guide-time", day ? `${day} ${hhmm(p.start)}` : hhmm(p.start)));
          cell.appendChild(el("span", "guide-title", p.title));
          if (isNow) {
            const bar = el("span", "guide-bar");
            const fill = el("span", "guide-bar-fill");
            fill.style.width = `${progressOf(p, p.stop, now)}%`;
            bar.appendChild(fill);
            cell.appendChild(bar);
          }
          if (p.stop > p.start) {
            cell.style.setProperty("--span", String(Math.max(4, Math.min(12, (p.stop - p.start) / 60000 / 10))));
          }
          cell.addEventListener("click", () => playChannel(ch, 0, ch.id !== state.currentChannelId));
          track.appendChild(cell);
        }
      }
      row.appendChild(track);
      rows.appendChild(row);
    }
    mainEl.appendChild(rows);
  }

  function findChannel(id) {
    return state.channels.find((c) => c.id === id) || null;
  }

  function filteredChannels() {
    const q = state.query.trim().toLowerCase();
    let list = state.channels;
    if (state.tab === "favorites") {
      list = list.filter((c) => c.favorite);
    } else if (state.tab === "recent") {
      const byId = new Map(state.channels.map((c) => [c.id, c]));
      list = state.recent.map((r) => byId.get(r.channelId)).filter(Boolean);
    }
    return list.filter((c) => {
      if (state.tab === "channels" && state.group && c.group !== state.group) return false;
      if (!q) return true;
      return (c.name || "").toLowerCase().includes(q) || (c.group || "").toLowerCase().includes(q);
    });
  }

  function clearTimers() {
    if (hudHideTimer) {
      clearTimeout(hudHideTimer);
      hudHideTimer = null;
    }
    if (stallTimer) {
      clearTimeout(stallTimer);
      stallTimer = null;
    }
  }

  function bumpHUD() {
    state.hudVisible = true;
    const shell = document.getElementById("theater-shell");
    if (shell) shell.classList.add("hud-visible");
    if (hudHideTimer) clearTimeout(hudHideTimer);
    hudHideTimer = setTimeout(() => {
      if (videoEl && !videoEl.paused) {
        state.hudVisible = false;
        const sh = document.getElementById("theater-shell");
        if (sh) sh.classList.remove("hud-visible");
      }
    }, 3200);
  }

  function ensureVideoElement() {
    if (videoEl) return videoEl;
    const v = document.createElement("video");
    v.id = "iptv-video";
    v.className = "theater-video";
    v.playsInline = true;
    v.setAttribute("webkit-playsinline", "true");
    v.preload = "auto";
    v.volume = state.volume;
    v.muted = state.muted;

    v.addEventListener("loadstart", () => {
      state.buffering = true;
      updateStageOverlay();
    });
    v.addEventListener("waiting", () => {
      state.buffering = true;
      updateStageOverlay();
      scheduleStallFailover();
    });
    v.addEventListener("stalled", () => {
      state.buffering = true;
      updateStageOverlay();
      scheduleStallFailover();
    });
    v.addEventListener("playing", () => {
      if (stallTimer) {
        clearTimeout(stallTimer);
        stallTimer = null;
      }
      state.buffering = false;
      clearBanner();
      updateStageOverlay();
      updateResolutionBadge();
      updatePlayPauseButton();
      bumpHUD();
    });
    v.addEventListener("canplay", () => {
      if (stallTimer) {
        clearTimeout(stallTimer);
        stallTimer = null;
      }
      state.buffering = false;
      updateStageOverlay();
      updateResolutionBadge();
    });
    v.addEventListener("loadedmetadata", updateResolutionBadge);
    v.addEventListener("resize", updateResolutionBadge);
    v.addEventListener("pause", () => {
      state.hudVisible = true;
      const sh = document.getElementById("theater-shell");
      if (sh) sh.classList.add("hud-visible");
      updatePlayPauseButton();
    });
    v.addEventListener("play", updatePlayPauseButton);
    v.addEventListener("error", () => {
      handleStreamFailure();
    });

    videoEl = v;
    return v;
  }

  function scheduleStallFailover() {
    if (stallTimer) clearTimeout(stallTimer);
    if (!state.settings.autoFailover || !state.currentChannelId) return;
    const snapCh = state.currentChannelId;
    const snapLine = state.currentLineIdx;
    stallTimer = setTimeout(() => {
      if (state.currentChannelId === snapCh && state.currentLineIdx === snapLine && state.buffering && videoEl && !videoEl.paused) {
        handleStreamFailure();
      }
    }, 12000);
  }

  function handleStreamFailure() {
    if (stallTimer) {
      clearTimeout(stallTimer);
      stallTimer = null;
    }
    state.buffering = false;
    updateStageOverlay();
    if (state.cspBlocked) return;

    const ch = findChannel(state.currentChannelId);
    if (!ch || !ch.streams || ch.streams.length === 0) return;

    if (state.settings.autoFailover !== false) {
      state.triedLines[state.currentLineIdx] = true;
      for (let step = 1; step < ch.streams.length; step++) {
        const nextIdx = (state.currentLineIdx + step) % ch.streams.length;
        if (!state.triedLines[nextIdx]) {
          showBanner(t().switchingLine.replace("{n}", String(nextIdx + 1)));
          playChannel(ch, nextIdx, false);
          return;
        }
      }
    }
    showBanner(t().allLinesFailed, { error: true });
  }

  function updateStageOverlay() {
    const ov = document.getElementById("stage-overlay");
    if (!ov) return;
    ov.hidden = !state.buffering;
  }

  function updatePlayPauseButton() {
    const btn = document.getElementById("hud-pp-btn");
    if (!btn || !videoEl) return;
    btn.textContent = videoEl.paused ? t().play : t().pause;
  }

  function updateResolutionBadge() {
    if (!videoEl) return;
    const h = Number(videoEl.videoHeight) || 0;
    const w = Number(videoEl.videoWidth) || 0;
    let res = "";
    if (h >= 2100 || w >= 3800) res = "4K";
    else if (h >= 1000) res = "1080P";
    else if (h >= 700) res = "720P";
    else if (h >= 400) res = `${h}P`;
    state.resolution = res;
    const badge = document.getElementById("hud-res-badge");
    if (badge) {
      badge.textContent = res;
      badge.hidden = !res;
    }
  }

  function playChannel(ch, lineIdx, resetTried) {
    if (!ch || !ch.streams || ch.streams.length === 0) return;
    const idx = Math.max(0, Math.min(ch.streams.length - 1, Number(lineIdx) || 0));
    const stream = ch.streams[idx];
    if (!stream || !stream.playUrl) return;

    if (resetTried !== false || state.currentChannelId !== ch.id) {
      state.triedLines = {};
    }
    state.triedLines[idx] = true;
    state.currentChannelId = ch.id;
    state.currentLineIdx = idx;
    state.buffering = true;
    state.resolution = "";
    state.cspBlocked = false;

    const v = ensureVideoElement();
    v.src = stream.playUrl;
    v.volume = state.volume;
    v.muted = state.muted;
    v.load();

    // Call play() synchronously inside click activation for WKWebView.
    let playPromise = null;
    try {
      playPromise = v.play();
    } catch (err) {
      showBanner((err && err.message) || String(err), { error: true });
    }
    if (playPromise && typeof playPromise.catch === "function") {
      playPromise.catch((err) => {
        if (err && err.name === "AbortError") return;
        handleStreamFailure();
      });
    }

    renderTheater();
    updateChannelTilePlayingState();
    bumpHUD();

    window.hyla.rpc("iptv.recordWatch", { channelId: ch.id, lineIdx: idx }).then((res) => {
      if (res && Array.isArray(res.recent)) state.recent = res.recent;
    }).catch(() => {});
  }

  function stopPlayback() {
    clearTimers();
    if (videoEl) {
      videoEl.pause();
      videoEl.removeAttribute("src");
      videoEl.load();
    }
    state.currentChannelId = "";
    state.currentLineIdx = 0;
    state.buffering = false;
    state.theaterFull = false;
    appRootEl.classList.remove("theater-full", "drawer-hidden");
    renderTheater();
    updateChannelTilePlayingState();
  }

  function zapChannel(delta) {
    const list = filteredChannels();
    if (list.length === 0) return;
    let idx = list.findIndex((c) => c.id === state.currentChannelId);
    if (idx < 0) idx = 0;
    else idx = (idx + delta + list.length) % list.length;
    playChannel(list[idx], 0, true);
  }

  function toggleTheaterFull(forceVal) {
    state.theaterFull = typeof forceVal === "boolean" ? forceVal : !state.theaterFull;
    appRootEl.classList.toggle("theater-full", state.theaterFull);
    appRootEl.classList.toggle("drawer-hidden", state.theaterFull && !state.drawerOpen);
    renderTheater();
  }

  function togglePictureInPicture() {
    if (!videoEl) return;
    if (document.pictureInPictureElement) {
      document.exitPictureInPicture().catch(() => {});
      return;
    }
    if (typeof videoEl.requestPictureInPicture === "function") {
      videoEl.requestPictureInPicture().catch(() => {});
    } else if (typeof videoEl.webkitSetPresentationMode === "function") {
      const next = videoEl.webkitPresentationMode === "picture-in-picture" ? "inline" : "picture-in-picture";
      try { videoEl.webkitSetPresentationMode(next); } catch {}
    }
  }

  function renderTopbar() {
    const d = t();
    document.title = d.title;
    document.documentElement.lang = resolveLocaleKey(locale);
    brandTitleEl.textContent = d.title;
    searchInputEl.placeholder = d.searchPlaceholder;
    addBtnEl.textContent = d.addSource;
    refreshBtnEl.textContent = d.refresh;
    settingsBtnEl.textContent = "⚙ " + d.settings;

    navTabsEl.textContent = "";
    for (const [id, label] of [
      ["channels", d.allChannels],
      ["guide", d.guide],
      ["favorites", d.favorites],
      ["recent", d.recent],
      ["sources", d.sources]
    ]) {
      const b = el("button", "nav-btn" + (state.tab === id ? " active" : ""), label);
      b.type = "button";
      b.addEventListener("click", () => {
        state.tab = id;
        state.visibleLimit = PAGE_STEP;
        if (state.theaterFull && id === "sources") {
          toggleTheaterFull(false);
          return;
        }
        renderMain();
        renderTopbar();
      });
      navTabsEl.appendChild(b);
    }

    const active = Object.values(state.jobs).filter((j) => j.state === "running");
    if (active.length === 0) {
      jobPillEl.hidden = true;
      jobPillEl.textContent = "";
    } else {
      const j = active[0];
      const pct = j.progress > 0 ? ` ${j.progress}%` : "";
      jobPillEl.textContent = `${j.message || j.kind}${pct}`;
      jobPillEl.hidden = false;
    }
  }

  function renderTheater() {
    const ch = findChannel(state.currentChannelId);
    if (!ch) {
      theaterSectionEl.hidden = true;
      theaterSectionEl.textContent = "";
      return;
    }
    const d = t();
    theaterSectionEl.hidden = false;

    let shell = document.getElementById("theater-shell");
    if (!shell) {
      theaterSectionEl.textContent = "";
      shell = el("div", "theater-shell hud-visible");
      shell.id = "theater-shell";
      shell.addEventListener("mousemove", bumpHUD);
      shell.addEventListener("click", (ev) => {
        if (ev.target === videoEl) bumpHUD();
      });
      shell.appendChild(ensureVideoElement());

      const overlay = el("div", "stage-overlay");
      overlay.id = "stage-overlay";
      overlay.appendChild(el("div", "spinner"));
      overlay.appendChild(el("span", "", d.buffering));
      shell.appendChild(overlay);

      const hudTop = el("div", "hud-top");
      hudTop.id = "hud-top";
      const hudBottom = el("div", "hud-bottom");
      hudBottom.id = "hud-bottom";
      shell.appendChild(hudTop);
      shell.appendChild(hudBottom);
      theaterSectionEl.appendChild(shell);
    }
    updateStageOverlay();

    const hudTop = document.getElementById("hud-top");
    hudTop.textContent = "";
    const topL = el("div", "hud-left");
    const smallLogo = buildLogoElement(ch.logoProxyUrl, ch.name, true);
    if (smallLogo) topL.appendChild(smallLogo);
    topL.appendChild(el("span", "hud-ch-title", ch.name));
    const liveBadge = el("span", "live-pill");
    liveBadge.appendChild(el("span", "live-pill-dot"));
    liveBadge.appendChild(document.createTextNode("LIVE"));
    topL.appendChild(liveBadge);
    if (ch.group) topL.appendChild(el("span", "hud-badge", ch.group));
    const resBadge = el("span", "hud-badge", state.resolution);
    resBadge.id = "hud-res-badge";
    resBadge.hidden = !state.resolution;
    topL.appendChild(resBadge);

    const topR = el("div", "hud-right");
    if (ch.streams && ch.streams.length > 1) {
      ch.streams.forEach((st, i) => {
        const lbl = st.label ? `${d.lineLabel} ${i + 1} · ${st.label}` : `${d.lineLabel} ${i + 1}`;
        const lb = el("button", "hud-btn" + (i === state.currentLineIdx ? " active" : ""), lbl);
        lb.type = "button";
        lb.addEventListener("click", () => playChannel(ch, i, true));
        topR.appendChild(lb);
      });
    }
    const favBtn = el("button", "hud-btn" + (ch.favorite ? " active" : ""), ch.favorite ? "★" : "☆");
    favBtn.type = "button";
    favBtn.addEventListener("click", () => toggleFavorite(ch.id));
    topR.appendChild(favBtn);

    const closeBtn = el("button", "hud-btn", d.closePlayer);
    closeBtn.type = "button";
    closeBtn.addEventListener("click", stopPlayback);
    topR.appendChild(closeBtn);

    hudTop.appendChild(topL);
    hudTop.appendChild(topR);

    // What is on, and what comes next, under the channel name.
    let upNext = document.getElementById("hud-next");
    if (upNext) upNext.remove();
    const { current: nowProg, next: nextProg } = programAt(programsFor(ch));
    if (nowProg || nextProg) {
      upNext = el("div", "hud-next");
      upNext.id = "hud-next";
      if (nowProg) {
        const nowLine = el("div", "hud-next-now");
        nowLine.appendChild(el("span", "hud-next-title", nowProg.program.title));
        if (nowProg.program.stop) nowLine.appendChild(el("span", "hud-next-time", `${hhmm(nowProg.program.start)} – ${hhmm(nowProg.program.stop)}`));
        upNext.appendChild(nowLine);
        const bar = el("div", "hud-next-bar");
        const fill = el("div", "hud-next-fill");
        fill.style.width = `${progressOf(nowProg.program, nowProg.stop, Date.now())}%`;
        bar.appendChild(fill);
        upNext.appendChild(bar);
      }
      if (nextProg) {
        upNext.appendChild(el("div", "hud-next-coming", `${d.guideUpNext} · ${hhmm(nextProg.start)} · ${nextProg.title}`));
      }
      shell.insertBefore(upNext, document.getElementById("hud-bottom"));
    }

    const hudBottom = document.getElementById("hud-bottom");
    hudBottom.textContent = "";
    const botL = el("div", "hud-left");

    const prevB = el("button", "hud-btn", "‹ " + d.prevChannel);
    prevB.type = "button";
    prevB.addEventListener("click", () => zapChannel(-1));
    const ppB = el("button", "hud-btn active", videoEl && videoEl.paused ? d.play : d.pause);
    ppB.id = "hud-pp-btn";
    ppB.type = "button";
    ppB.addEventListener("click", () => {
      if (!videoEl) return;
      if (videoEl.paused) videoEl.play().catch(() => {});
      else videoEl.pause();
    });
    const nextB = el("button", "hud-btn", d.nextChannel + " ›");
    nextB.type = "button";
    nextB.addEventListener("click", () => zapChannel(1));
    const reloadB = el("button", "hud-btn", d.reload);
    reloadB.type = "button";
    reloadB.addEventListener("click", () => playChannel(ch, state.currentLineIdx, true));
    botL.appendChild(prevB);
    botL.appendChild(ppB);
    botL.appendChild(nextB);
    botL.appendChild(reloadB);

    const botR = el("div", "hud-right");
    const muteB = el("button", "hud-btn", state.muted || state.volume === 0 ? "🔇" : "🔊");
    muteB.type = "button";
    muteB.title = state.muted ? d.unmute : d.mute;
    const volSl = el("input", "hud-vol");
    volSl.type = "range";
    volSl.min = "0";
    volSl.max = "1";
    volSl.step = "0.05";
    volSl.value = String(state.muted ? 0 : state.volume);
    muteB.addEventListener("click", () => {
      state.muted = !state.muted;
      if (videoEl) videoEl.muted = state.muted;
      muteB.textContent = state.muted || state.volume === 0 ? "🔇" : "🔊";
      volSl.value = String(state.muted ? 0 : state.volume);
    });
    volSl.addEventListener("input", () => {
      const val = Math.max(0, Math.min(1, Number(volSl.value) || 0));
      state.volume = val;
      state.muted = val === 0;
      if (videoEl) {
        videoEl.volume = val;
        videoEl.muted = state.muted;
      }
      muteB.textContent = state.muted ? "🔇" : "🔊";
    });
    botR.appendChild(muteB);
    botR.appendChild(volSl);

    const pipB = el("button", "hud-btn", d.pip);
    pipB.type = "button";
    pipB.addEventListener("click", togglePictureInPicture);
    botR.appendChild(pipB);

    if (state.theaterFull) {
      const drB = el("button", "hud-btn" + (state.drawerOpen ? " active" : ""), d.channelsDrawer);
      drB.type = "button";
      drB.addEventListener("click", () => {
        state.drawerOpen = !state.drawerOpen;
        appRootEl.classList.toggle("drawer-hidden", !state.drawerOpen);
        renderTheater();
      });
      botR.appendChild(drB);
    }

    const thB = el("button", "hud-btn" + (state.theaterFull ? " active" : ""), state.theaterFull ? d.exitTheater : d.theater);
    thB.type = "button";
    thB.addEventListener("click", () => toggleTheaterFull());
    botR.appendChild(thB);

    hudBottom.appendChild(botL);
    hudBottom.appendChild(botR);
  }

  function updateChannelTilePlayingState() {
    const tiles = mainEl.querySelectorAll(".ch-tile");
    for (const tile of tiles) {
      const isPlaying = tile.dataset.channelId === state.currentChannelId;
      tile.classList.toggle("playing", isPlaying);
      let eq = tile.querySelector(".eq-badge");
      if (isPlaying && !eq) {
        eq = el("div", "eq-badge");
        eq.appendChild(el("div", "eq-bar"));
        eq.appendChild(el("div", "eq-bar"));
        eq.appendChild(el("div", "eq-bar"));
        const stage = tile.querySelector(".logo-stage");
        if (stage) stage.appendChild(eq);
      } else if (!isPlaying && eq) {
        eq.remove();
      }
    }
  }

  async function toggleFavorite(channelId) {
    try {
      const res = await window.hyla.rpc("iptv.toggleFavorite", { channelId });
      const ch = findChannel(channelId);
      if (ch && res) ch.favorite = !!res.favorite;
      if (res && Array.isArray(res.favorites)) state.favorites = res.favorites;
      renderTheater();
      renderMain();
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
    }
  }

  function renderWelcomeView() {
    const d = t();
    const loc = resolveLocaleKey(locale);
    const hero = el("div", "welcome-hero");
    hero.appendChild(el("div", "welcome-title", d.welcomeTitle));
    hero.appendChild(el("div", "welcome-sub", d.welcomeSub));

    const ta = el("textarea", "textarea");
    ta.rows = 4;
    ta.placeholder = d.sourceInputPlaceholder;
    hero.appendChild(ta);

    const impBtn = el("button", "glass-btn primary", d.importBtn);
    impBtn.type = "button";
    impBtn.addEventListener("click", async () => {
      const val = ta.value.trim();
      if (!val) return;
      impBtn.disabled = true;
      impBtn.textContent = d.importing;
      await addSources(val, "");
      impBtn.disabled = false;
      impBtn.textContent = d.importBtn;
    });
    hero.appendChild(impBtn);

    hero.appendChild(el("div", "form-label", d.starterTitle));
    const pGrid = el("div", "preset-grid");
    for (const p of state.presets) {
      const card = el("div", "preset-card");
      const pName = loc === "zh-Hans" ? (p.nameZh || p.name) : loc === "ja" ? (p.nameJa || p.name) : p.name;
      const pDesc = loc === "zh-Hans" ? (p.descZh || p.desc) : loc === "ja" ? (p.descJa || p.desc) : p.desc;
      card.appendChild(el("div", "preset-title", pName));
      card.appendChild(el("div", "preset-desc", pDesc));
      const b = el("button", "glass-btn small primary", d.subscribePreset);
      b.type = "button";
      b.addEventListener("click", async () => {
        b.disabled = true;
        b.textContent = d.importing;
        await addSources(p.url, pName);
        b.disabled = false;
        b.textContent = d.subscribePreset;
      });
      card.appendChild(b);
      pGrid.appendChild(card);
    }
    hero.appendChild(pGrid);
    mainEl.appendChild(hero);
  }

  function renderSourcesView() {
    const d = t();
    const loc = resolveLocaleKey(locale);
    const wrap = el("div", "sources-wrap");

    const topTools = el("div", "topbar-right");
    const addB = el("button", "glass-btn small primary", d.addSource);
    addB.type = "button";
    addB.addEventListener("click", openAddSourceModal);
    topTools.appendChild(addB);

    if (state.channels.length > 0) {
      const expB = el("button", "glass-btn small", d.exportM3U);
      expB.type = "button";
      expB.addEventListener("click", async () => {
        const res = await window.hyla.rpc("iptv.exportM3U", {});
        openExportModal(d.exportM3U, (res && res.m3u) || "");
      });
      topTools.appendChild(expB);
    }
    if (state.storage && state.storage.logosCount > 0) {
      const clrB = el("button", "glass-btn small", `${d.clearLogos} (${state.storage.logosCount} · ${formatBytes(state.storage.logosBytes)})`);
      clrB.type = "button";
      clrB.addEventListener("click", async () => {
        const res = await window.hyla.rpc("iptv.clearLogoCache", {});
        if (res && res.storage) state.storage = res.storage;
        renderMain();
      });
      topTools.appendChild(clrB);
    }
    wrap.appendChild(topTools);

    if (state.sources.length === 0) {
      renderWelcomeView();
      return;
    }

    for (const src of state.sources) {
      const card = el("div", "source-card");
      const info = el("div", "source-info");
      info.appendChild(el("div", "source-name", `${src.name} (${src.channelCount || 0} ${d.channelsUnit})`));
      const urlText = src.resolvedUrl && src.resolvedUrl !== src.inputUrl
        ? `${src.inputUrl} → ${src.resolvedUrl}`
        : (src.inputUrl || "Pasted M3U");
      info.appendChild(el("div", "source-meta", urlText));
      if (src.lastError) info.appendChild(el("div", "source-err", src.lastError));
      card.appendChild(info);

      const acts = el("div", "source-actions");
      const tog = el("button", "glass-btn small", src.enabled ? d.disable : d.enable);
      tog.type = "button";
      tog.addEventListener("click", async () => {
        const res = await window.hyla.rpc("iptv.toggleSource", { sourceId: src.id, enabled: !src.enabled });
        if (res && res.bootstrap) applyBootstrap(res.bootstrap);
        render();
      });
      acts.appendChild(tog);

      const rem = el("button", "glass-btn small danger", d.remove);
      rem.type = "button";
      rem.addEventListener("click", async () => {
        const res = await window.hyla.rpc("iptv.removeSource", { sourceId: src.id });
        if (res && res.bootstrap) applyBootstrap(res.bootstrap);
        render();
      });
      acts.appendChild(rem);
      card.appendChild(acts);
      wrap.appendChild(card);
    }

    const pTitle = el("div", "form-label", d.starterTitle);
    pTitle.style.marginTop = "8px";
    wrap.appendChild(pTitle);
    const pGrid = el("div", "preset-grid");
    for (const p of state.presets) {
      const card = el("div", "preset-card");
      const pName = loc === "zh-Hans" ? (p.nameZh || p.name) : loc === "ja" ? (p.nameJa || p.name) : p.name;
      const pDesc = loc === "zh-Hans" ? (p.descZh || p.desc) : loc === "ja" ? (p.descJa || p.desc) : p.desc;
      card.appendChild(el("div", "preset-title", pName));
      card.appendChild(el("div", "preset-desc", pDesc));
      const b = el("button", "glass-btn small primary", d.subscribePreset);
      b.type = "button";
      b.addEventListener("click", async () => {
        b.disabled = true;
        await addSources(p.url, pName);
        b.disabled = false;
      });
      card.appendChild(b);
      pGrid.appendChild(card);
    }
    wrap.appendChild(pGrid);
    mainEl.appendChild(wrap);
  }

  function renderMain() {
    mainEl.textContent = "";
    const d = t();

    if (state.tab === "sources") {
      renderSourcesView();
      return;
    }
    if (state.tab === "guide") {
      renderGuideView();
      return;
    }
    if (state.sources.length === 0 && state.channels.length === 0) {
      renderWelcomeView();
      return;
    }

    const nowRow = state.tab === "channels" && !state.query && !state.group ? renderNowRow() : null;
    if (nowRow) mainEl.appendChild(nowRow);

    if (state.tab === "channels" && state.groups.length > 1) {
      const rail = el("div", "group-rail");
      const allP = el("button", "group-pill" + (!state.group ? " active" : ""), `${d.allGroups} (${state.channels.length})`);
      allP.type = "button";
      allP.addEventListener("click", () => {
        state.group = "";
        state.visibleLimit = PAGE_STEP;
        renderMain();
      });
      rail.appendChild(allP);

      for (const g of state.groups) {
        const gp = el("button", "group-pill" + (state.group === g ? " active" : ""), g);
        gp.type = "button";
        gp.addEventListener("click", () => {
          state.group = g;
          state.visibleLimit = PAGE_STEP;
          renderMain();
        });
        rail.appendChild(gp);
      }
      mainEl.appendChild(rail);
    }

    const list = filteredChannels();
    if (list.length === 0) {
      const emptyMsg = state.tab === "favorites"
        ? d.emptyFavorites
        : state.tab === "recent"
          ? d.emptyRecent
          : d.noChannelsMatch;
      mainEl.appendChild(el("div", "empty-box", emptyMsg));
      return;
    }

    const grid = el("div", "ch-grid");
    const slice = list.slice(0, state.visibleLimit);
    for (let i = 0; i < slice.length; i++) {
      const ch = slice[i];
      const isPlaying = ch.id === state.currentChannelId;
      const tile = el("div", "ch-tile" + (isPlaying ? " playing" : ""));
      tile.dataset.channelId = ch.id;
      tile.tabIndex = 0;

      const stage = el("div", "logo-stage");
      stage.appendChild(buildLogoElement(ch.logoProxyUrl, ch.name, false, i < 36));

      if (isPlaying) {
        const eq = el("div", "eq-badge");
        eq.appendChild(el("div", "eq-bar"));
        eq.appendChild(el("div", "eq-bar"));
        eq.appendChild(el("div", "eq-bar"));
        stage.appendChild(eq);
      }

      const favB = el("button", "tile-fav" + (ch.favorite ? " active" : ""), ch.favorite ? "★" : "☆");
      favB.type = "button";
      favB.addEventListener("click", (ev) => {
        ev.stopPropagation();
        toggleFavorite(ch.id);
      });
      stage.appendChild(favB);

      if (ch.streams && ch.streams.length > 1) {
        stage.appendChild(el("span", "tile-lines", `${ch.streams.length} ${d.lineLabel}`));
      }
      tile.appendChild(stage);

      const meta = el("div", "ch-meta");
      meta.appendChild(el("div", "ch-name", ch.name));
      const nowInfo = programAt(programsFor(ch));
      if (nowInfo.current) {
        meta.appendChild(el("div", "ch-now", nowInfo.current.program.title));
      } else if (ch.group) {
        meta.appendChild(el("div", "ch-group", ch.group));
      }
      tile.appendChild(meta);

      tile.addEventListener("click", () => playChannel(ch, 0, true));
      tile.addEventListener("keydown", (ev) => {
        if (ev.key === "Enter" || ev.key === " ") {
          ev.preventDefault();
          playChannel(ch, 0, true);
        }
      });
      grid.appendChild(tile);
    }
    mainEl.appendChild(grid);

    if (list.length > slice.length) {
      const rem = list.length - slice.length;
      const moreWrap = el("div", "load-more-wrap");
      const moreBtn = el("button", "glass-btn", d.showMore.replace("{n}", String(rem)));
      moreBtn.type = "button";
      moreBtn.addEventListener("click", () => {
        const prevTop = mainEl.scrollTop;
        state.visibleLimit += PAGE_STEP;
        renderMain();
        mainEl.scrollTop = prevTop;
      });
      moreWrap.appendChild(moreBtn);
      mainEl.appendChild(moreWrap);
    }
  }

  function render() {
    renderTopbar();
    renderBanner();
    renderTheater();
    renderMain();
  }

  async function addSources(input, name) {
    try {
      const res = await window.hyla.rpc("iptv.addSources", { input, name: name || "" });
      if (res && res.bootstrap) applyBootstrap(res.bootstrap);
      if (res && Array.isArray(res.warnings) && res.warnings.length > 0) {
        showBanner(res.warnings.join(" · "));
      } else if (res && res.added > 0) {
        showBanner(t().addedNotice.replace("{n}", String(res.added)));
      }
      state.tab = "channels";
      state.visibleLimit = PAGE_STEP;
      render();
      return true;
    } catch (err) {
      showBanner(err.message || String(err), { error: true });
      return false;
    }
  }

  function openModal(title, buildBody) {
    modalBackdropEl.hidden = false;
    modalBackdropEl.textContent = "";
    const box = el("div", "modal");
    const hdr = el("div", "modal-hdr");
    hdr.appendChild(el("div", "modal-title", title));
    const cls = el("button", "glass-btn small", t().close);
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

  function openAddSourceModal() {
    const d = t();
    const loc = resolveLocaleKey(locale);
    openModal(d.addSource, (box) => {
      const r1 = el("div", "form-row");
      r1.appendChild(el("label", "form-label", d.sourceInputLabel));
      const ta = el("textarea", "textarea");
      ta.rows = 5;
      ta.placeholder = d.sourceInputPlaceholder;
      r1.appendChild(ta);
      box.appendChild(r1);

      const r2 = el("div", "form-row");
      r2.appendChild(el("label", "form-label", d.sourceNameLabel));
      const nameIn = el("input", "input");
      nameIn.type = "text";
      nameIn.placeholder = d.sourceNamePlaceholder;
      r2.appendChild(nameIn);
      box.appendChild(r2);

      const subB = el("button", "glass-btn primary", d.importBtn);
      subB.type = "button";
      subB.addEventListener("click", async () => {
        const v = ta.value.trim();
        if (!v) return;
        subB.disabled = true;
        subB.textContent = d.importing;
        const ok = await addSources(v, nameIn.value.trim());
        subB.disabled = false;
        subB.textContent = d.importBtn;
        if (ok) closeModal();
      });
      box.appendChild(subB);

      if (state.presets.length > 0) {
        box.appendChild(el("div", "form-label", d.starterTitle));
        const pGrid = el("div", "preset-grid");
        for (const p of state.presets) {
          const pc = el("div", "preset-card");
          const pName = loc === "zh-Hans" ? (p.nameZh || p.name) : loc === "ja" ? (p.nameJa || p.name) : p.name;
          pc.appendChild(el("div", "preset-title", pName));
          const pb = el("button", "glass-btn small primary", d.subscribePreset);
          pb.type = "button";
          pb.addEventListener("click", async () => {
            pb.disabled = true;
            pb.textContent = d.importing;
            const ok = await addSources(p.url, pName);
            pb.disabled = false;
            pb.textContent = d.subscribePreset;
            if (ok) closeModal();
          });
          pc.appendChild(pb);
          pGrid.appendChild(pc);
        }
        box.appendChild(pGrid);
      }
      setTimeout(() => ta.focus(), 20);
    });
  }

  function openExportModal(title, text) {
    const d = t();
    openModal(title, (box) => {
      const ta = el("textarea", "textarea");
      ta.rows = 12;
      ta.readOnly = true;
      ta.value = text;
      box.appendChild(ta);
      const cp = el("button", "glass-btn small primary", d.copy);
      cp.type = "button";
      cp.addEventListener("click", () => {
        ta.select();
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(ta.value).catch(() => {});
        }
        cp.textContent = d.copied;
      });
      box.appendChild(cp);
    });
  }

  function openSettingsModal() {
    const d = t();
    const s = state.settings || {};
    openModal(d.settingsTitle, (box) => {
      const pmRow = el("div", "form-row");
      pmRow.appendChild(el("label", "form-label", d.proxyModeLabel));
      const pmSel = el("select", "select");
      for (const m of ["auto", "direct", "system", "custom"]) {
        const opt = el("option", "", m);
        opt.value = m;
        if (m === (s.proxyMode || "auto")) opt.selected = true;
        pmSel.appendChild(opt);
      }
      pmRow.appendChild(pmSel);
      box.appendChild(pmRow);

      const pxRow = el("div", "form-row");
      pxRow.appendChild(el("label", "form-label", d.httpProxyLabel));
      const pxIn = el("input", "input");
      pxIn.type = "text";
      pxIn.value = s.httpProxy || "";
      pxIn.placeholder = "http://127.0.0.1:7890";
      pxRow.appendChild(pxIn);
      box.appendChild(pxRow);

      const uaRow = el("div", "form-row");
      uaRow.appendChild(el("label", "form-label", d.userAgentLabel));
      const uaIn = el("input", "input");
      uaIn.type = "text";
      uaIn.value = s.userAgent || "";
      uaIn.placeholder = "Mozilla/5.0 ...";
      uaRow.appendChild(uaIn);
      box.appendChild(uaRow);

      const foRow = el("label", "check-row");
      const foChk = document.createElement("input");
      foChk.type = "checkbox";
      foChk.checked = s.autoFailover !== false;
      foRow.appendChild(foChk);
      foRow.appendChild(el("span", "", d.autoFailoverLabel));
      box.appendChild(foRow);

      const amRow = el("label", "check-row");
      const amChk = document.createElement("input");
      amChk.type = "checkbox";
      amChk.checked = !!s.autoMuteOnAgent;
      amRow.appendChild(amChk);
      amRow.appendChild(el("span", "", d.autoMuteLabel));
      box.appendChild(amRow);

      const saveBtn = el("button", "glass-btn primary", d.save);
      saveBtn.type = "button";
      saveBtn.addEventListener("click", async () => {
        saveBtn.disabled = true;
        try {
          const res = await window.hyla.rpc("iptv.saveSettings", {
            proxyMode: pmSel.value,
            httpProxy: pxIn.value.trim(),
            autoFailover: foChk.checked,
            autoMuteOnAgent: amChk.checked,
            userAgent: uaIn.value.trim()
          });
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

  // Event wiring
  searchInputEl.addEventListener("input", () => {
    state.query = searchInputEl.value;
    state.visibleLimit = PAGE_STEP;
    if (state.tab === "sources") state.tab = "channels";
    renderMain();
    renderTopbar();
  });

  addBtnEl.addEventListener("click", openAddSourceModal);
  refreshBtnEl.addEventListener("click", async () => {
    const res = await window.hyla.rpc("iptv.refreshSources", {}).catch((err) => {
      showBanner(err.message || String(err), { error: true });
      return null;
    });
    if (res && res.job) state.jobs[res.job.id] = res.job;
    renderTopbar();
  });
  settingsBtnEl.addEventListener("click", openSettingsModal);

  document.addEventListener("securitypolicyviolation", (ev) => {
    const dir = String((ev && (ev.effectiveDirective || ev.violatedDirective)) || "").toLowerCase();
    const uri = String((ev && ev.blockedURI) || "");
    if (dir.startsWith("media-src") || (dir.startsWith("default-src") && (uri.includes("/play/") || uri.startsWith("http://127.0.0.1")))) {
      state.cspBlocked = true;
      showBanner(t().cspMediaBlocked, { error: true });
    }
  });

  document.addEventListener("keydown", (ev) => {
    if (!modalBackdropEl.hidden) {
      if (ev.key === "Escape") closeModal();
      return;
    }
    if (ev.metaKey || ev.ctrlKey || ev.altKey) return;
    const tag = (ev.target && ev.target.tagName) || "";
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || (ev.target && ev.target.isContentEditable)) {
      if (ev.key === "Escape" && ev.target === searchInputEl) searchInputEl.blur();
      return;
    }

    if (ev.key === "/") {
      ev.preventDefault();
      searchInputEl.focus();
      searchInputEl.select();
    } else if (ev.code === "Space" || ev.key === "k" || ev.key === "K") {
      if (videoEl && state.currentChannelId) {
        ev.preventDefault();
        if (videoEl.paused) videoEl.play().catch(() => {});
        else videoEl.pause();
      }
    } else if (ev.key === "ArrowUp") {
      if (state.currentChannelId) {
        ev.preventDefault();
        zapChannel(-1);
      }
    } else if (ev.key === "ArrowDown") {
      if (state.currentChannelId) {
        ev.preventDefault();
        zapChannel(1);
      }
    } else if (ev.key === "m" || ev.key === "M") {
      if (videoEl && state.currentChannelId) {
        state.muted = !state.muted;
        videoEl.muted = state.muted;
        renderTheater();
      }
    } else if (ev.key === "g" || ev.key === "G") {
      state.tab = state.tab === "guide" ? "channels" : "guide";
      state.visibleLimit = PAGE_STEP;
      renderMain();
      renderTopbar();
    } else if (ev.key === "f" || ev.key === "F") {
      if (state.currentChannelId) toggleTheaterFull();
    } else if (ev.key === "Escape") {
      if (state.theaterFull) toggleTheaterFull(false);
    }
  });

  if (window.hyla && typeof window.hyla.on === "function") {
    window.hyla.on("iptv.job", async (job) => {
      if (!job || !job.id) return;
      if (job.state === "running") {
        state.jobs[job.id] = job;
        renderTopbar();
        return;
      }
      delete state.jobs[job.id];
      if (job.state === "error") {
        showBanner(job.error || "Job failed", { error: true });
      }
      if (job.kind === "refresh") {
        await bootstrap();
      } else {
        renderTopbar();
      }
    });

    window.hyla.on("iptv.autoMute", () => {
      if (videoEl && state.currentChannelId && !state.muted) {
        state.muted = true;
        videoEl.muted = true;
        renderTheater();
        showBanner(t().autoMutedBanner, {
          actionLabel: t().unmute,
          onAction: () => {
            clearBanner();
            state.muted = false;
            if (videoEl) videoEl.muted = false;
            renderTheater();
          }
        });
      }
    });

    window.hyla.on("iptv.epg", (data) => {
      if (!data) return;
      if (data.programs) state.programs = data.programs;
      if (data.epg) state.epg = data.epg;
      // The guide belongs to the video as well: the up-next line shows now.
      if (state.currentChannelId) renderTheater();
      renderMain();
    });

    window.hyla.on("iptv.settingsChanged", (data) => {
      if (data && data.settings) state.settings = data.settings;
    });

    // The guide moves on while a pane stays open: a minute of refresh keeps
    // the progress bars and the on-now rail true.
    setInterval(() => {
      if (!epgOn()) return;
      renderMain();
      if (state.currentChannelId) renderTheater();
    }, 60000);

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
