(() => {
  const I18N = {
    en: {
      title: "Pomodoro",
      tabTimer: "Timer",
      tabReport: "Report",
      settings: "Settings",
      pomodoro: "Pomodoro",
      shortBreak: "Short Break",
      longBreak: "Long Break",
      focusTag: "FOCUS",
      shortTag: "SHORT BREAK",
      longTag: "LONG BREAK",
      start: "Start",
      pause: "Pause",
      resume: "Resume",
      reset: "Reset",
      skip: "Skip",
      noTaskFocus: "Time to focus!",
      noTaskShort: "Time for a short break.",
      noTaskLong: "Time for a long break.",
      tasks: "Tasks",
      addTask: "Add Task",
      taskPlaceholder: "What are you working on?",
      estPomos: "Est Pomodoros",
      addNote: "+ Add Note",
      hideNote: "Hide Note",
      notePlaceholder: "Optional notes or checklist…",
      save: "Save",
      cancel: "Cancel",
      delete: "Delete",
      emptyTasks: "Add a task to track pomodoros and estimate your finish time.",
      clearFinished: "Clear finished tasks",
      clearAct: "Clear act pomodoros",
      clearAll: "Clear all tasks",
      pomosLabel: "Pomos:",
      finishAtLabel: "Finish At:",
      todayFocus: "Today's Focus",
      todayPomos: "Pomodoros Today",
      streak: "Current Streak",
      daysUnit: "{n}d",
      goalProgress: "{done} / {goal} goal",
      allTimeSub: "{pomos} total · {dur}",
      last7Days: "Last 7 Days",
      goalLine: "Goal: {n}",
      recentSessions: "Recent Focus Sessions",
      clearHistory: "Clear History",
      noHistory: "Complete a focus session to see your history here.",
      untitledFocus: "Focus Session",
      settingsTitle: "Pomodoro Settings",
      secTimer: "Timer (minutes)",
      secSound: "Sound",
      secNotify: "Notifications",
      lblPomodoroMin: "Pomodoro",
      lblShortMin: "Short Break",
      lblLongMin: "Long Break",
      lblInterval: "Long Break Interval",
      lblDailyGoal: "Daily Goal (pomos)",
      lblAutoBreaks: "Auto-start Breaks",
      lblAutoPomos: "Auto-start Pomodoros",
      lblAlarmSound: "Alarm Sound",
      lblAlarmVol: "Alarm Volume",
      lblAlarmRepeat: "Repeat",
      lblAmbientSound: "Ambient Focus Sound",
      lblAmbientVol: "Ambient Volume",
      lblDesktopNotify: "Desktop Notifications",
      testSound: "Test",
      soundBell: "Bell",
      soundBowl: "Singing Bowl",
      soundMarimba: "Marimba",
      soundDigital: "Digital",
      soundNone: "Mute",
      ambNone: "Off",
      ambTick: "Soft Tick",
      ambRain: "Warm Rain",
      ambBrown: "Brown Noise",
      phaseDoneFocus: "Focus session complete — take a break.",
      phaseDoneBreak: "Break finished — ready to focus?",
      hourShort: "{h}h {m}m",
      minShort: "{m}m"
    },
    "zh-Hans": {
      title: "番茄钟",
      tabTimer: "计时",
      tabReport: "统计",
      settings: "设置",
      pomodoro: "番茄专注",
      shortBreak: "短休息",
      longBreak: "长休息",
      focusTag: "专注",
      shortTag: "短休息",
      longTag: "长休息",
      start: "开始",
      pause: "暂停",
      resume: "继续",
      reset: "重置",
      skip: "跳过",
      noTaskFocus: "专注当下，开始一个番茄钟！",
      noTaskShort: "起来活动一下，短休片刻。",
      noTaskLong: "完成了一组专注，好好长休放松。",
      tasks: "任务清单",
      addTask: "添加任务",
      taskPlaceholder: "接下来要专注完成什么？",
      estPomos: "预计番茄数",
      addNote: "+ 添加备注",
      hideNote: "收起备注",
      notePlaceholder: "可选备注或拆分步骤…",
      save: "保存",
      cancel: "取消",
      delete: "删除",
      emptyTasks: "添加一个任务以记录番茄数并估算完成时间。",
      clearFinished: "清除已完成任务",
      clearAct: "重置已完成番茄数",
      clearAll: "清空所有任务",
      pomosLabel: "番茄数：",
      finishAtLabel: "预计完成：",
      todayFocus: "今日专注",
      todayPomos: "今日完成",
      streak: "连续专注",
      daysUnit: "{n} 天",
      goalProgress: "目标 {done} / {goal}",
      allTimeSub: "累计 {pomos} 个 · {dur}",
      last7Days: "近 7 天专注",
      goalLine: "每日目标：{n}",
      recentSessions: "最近专注记录",
      clearHistory: "清空记录",
      noHistory: "完成一个专注阶段后，记录会显示在这里。",
      untitledFocus: "自由专注",
      settingsTitle: "番茄钟设置",
      secTimer: "计时时长（分钟）",
      secSound: "声音与白噪音",
      secNotify: "桌面通知",
      lblPomodoroMin: "专注",
      lblShortMin: "短休息",
      lblLongMin: "长休息",
      lblInterval: "长休息间隔",
      lblDailyGoal: "每日目标",
      lblAutoBreaks: "自动开始休息",
      lblAutoPomos: "自动开始下一轮专注",
      lblAlarmSound: "提示音",
      lblAlarmVol: "提示音音量",
      lblAlarmRepeat: "重复次数",
      lblAmbientSound: "专注环境音",
      lblAmbientVol: "环境音音量",
      lblDesktopNotify: "计时结束时发送桌面通知",
      testSound: "试听",
      soundBell: "清脆铃声",
      soundBowl: "颂钵余韵",
      soundMarimba: "木琴和弦",
      soundDigital: "电子提示",
      soundNone: "静音",
      ambNone: "关闭",
      ambTick: "轻柔滴答",
      ambRain: "细雨沙沙",
      ambBrown: "棕噪深潜",
      phaseDoneFocus: "专注结束，休息一下吧。",
      phaseDoneBreak: "休息结束，准备开始下一轮专注！",
      hourShort: "{h}小时{m}分",
      minShort: "{m}分钟"
    },
    ja: {
      title: "ポモドーロ",
      tabTimer: "タイマー",
      tabReport: "レポート",
      settings: "設定",
      pomodoro: "ポモドーロ",
      shortBreak: "短い休憩",
      longBreak: "長い休憩",
      focusTag: "集中",
      shortTag: "小休憩",
      longTag: "長休憩",
      start: "開始",
      pause: "一時停止",
      resume: "再開",
      reset: "リセット",
      skip: "スキップ",
      noTaskFocus: "集中する時間です！",
      noTaskShort: "少し休憩しましょう。",
      noTaskLong: "長い休憩でリフレッシュしましょう。",
      tasks: "タスク",
      addTask: "タスクを追加",
      taskPlaceholder: "今取り組むタスクは？",
      estPomos: "見積ポモ数",
      addNote: "+ メモを追加",
      hideNote: "メモを隠す",
      notePlaceholder: "任意のメモや手順…",
      save: "保存",
      cancel: "キャンセル",
      delete: "削除",
      emptyTasks: "タスクを追加すると完了予想時刻を計算できます。",
      clearFinished: "完了タスクを削除",
      clearAct: "実績ポモ数をリセット",
      clearAll: "すべて削除",
      pomosLabel: "ポモ数：",
      finishAtLabel: "完了予定：",
      todayFocus: "今日の集中",
      todayPomos: "今日のポモ数",
      streak: "連続日数",
      daysUnit: "{n}日",
      goalProgress: "目標 {done} / {goal}",
      allTimeSub: "累計 {pomos} 回 · {dur}",
      last7Days: "直近7日間",
      goalLine: "目標: {n}",
      recentSessions: "最近の集中履歴",
      clearHistory: "履歴をクリア",
      noHistory: "集中セッションを完了すると履歴が表示されます。",
      untitledFocus: "集中セッション",
      settingsTitle: "ポモドーロ設定",
      secTimer: "タイマー（分）",
      secSound: "サウンド",
      secNotify: "通知",
      lblPomodoroMin: "集中",
      lblShortMin: "短い休憩",
      lblLongMin: "長い休憩",
      lblInterval: "長い休憩の間隔",
      lblDailyGoal: "1日の目標",
      lblAutoBreaks: "休憩を自動開始",
      lblAutoPomos: "集中を自動開始",
      lblAlarmSound: "アラーム音",
      lblAlarmVol: "アラーム音量",
      lblAlarmRepeat: "繰り返し",
      lblAmbientSound: "集中環境音",
      lblAmbientVol: "環境音音量",
      lblDesktopNotify: "デスクトップ通知を送る",
      testSound: "試聴",
      soundBell: "ベル",
      soundBowl: "シンギングボウル",
      soundMarimba: "マリンバ",
      soundDigital: "デジタル",
      soundNone: "ミュート",
      ambNone: "オフ",
      ambTick: "ソフトチック",
      ambRain: "雨音",
      ambBrown: "ブラウンノイズ",
      phaseDoneFocus: "集中完了！休憩しましょう。",
      phaseDoneBreak: "休憩終了！次の集中を始めましょう。",
      hourShort: "{h}時間{m}分",
      minShort: "{m}分"
    }
  };

  function normalizeLocale(loc) {
    const l = String(loc || "").trim().toLowerCase().replaceAll("_", "-");
    if (l === "zh-hans" || l === "zh-cn" || l === "zh-sg" || l === "zh" || l.startsWith("zh-hans")) {
      return "zh-Hans";
    }
    if (l === "ja" || l.startsWith("ja-")) {
      return "ja";
    }
    if (l === "en" || l.startsWith("en-")) {
      return "en";
    }
    return "en";
  }

  function detectInitialLocale() {
    const qp = new URLSearchParams(window.location.search).get("locale");
    return normalizeLocale(qp || navigator.language || "en");
  }

  function detectThemeMode() {
    const cs = getComputedStyle(document.documentElement).colorScheme || "";
    if (cs.trim() === "dark") {
      document.documentElement.dataset.theme = "dark";
      return;
    }
    if (cs.trim() === "light") {
      document.documentElement.dataset.theme = "light";
      return;
    }
    const bg = getComputedStyle(document.documentElement).getPropertyValue("--hyla-bg");
    const m = bg.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/i);
    if (m) {
      const lum = 0.2126 * Number(m[1]) + 0.7152 * Number(m[2]) + 0.0722 * Number(m[3]);
      document.documentElement.dataset.theme = lum < 128 ? "dark" : "light";
      return;
    }
    const prefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
    document.documentElement.dataset.theme = prefersDark ? "dark" : "light";
  }

  // --- Pure Local Web Audio Engine ---
  class SoundEngine {
    constructor() {
      this.ctx = null;
      this.ambientMode = "none";
      this.ambientVolume = 0.35;
      this.ambientActive = false;
      this.noiseSource = null;
      this.ambientGain = null;
      this.tickTimer = null;
    }

    ensureContext() {
      if (!this.ctx) {
        const AC = window.AudioContext || window.webkitAudioContext;
        if (!AC) return null;
        this.ctx = new AC();
      }
      if (this.ctx.state === "suspended") {
        this.ctx.resume().catch(() => {});
      }
      return this.ctx;
    }

    unlock() {
      this.ensureContext();
    }

    playChime(kind, volumePct, repeatCount) {
      if (!kind || kind === "none" || volumePct <= 0) return;
      const ctx = this.ensureContext();
      if (!ctx) return;

      const vol = Math.max(0, Math.min(1, Number(volumePct) / 100));
      const repeats = Math.max(1, Math.min(5, Number(repeatCount) || 1));

      for (let i = 0; i < repeats; i++) {
        const start = ctx.currentTime + i * 0.95;
        this.triggerChimeOnce(ctx, kind, vol, start);
      }
    }

    triggerChimeOnce(ctx, kind, vol, t0) {
      if (kind === "bell") {
        this.tone(ctx, 880, "sine", t0, 0.75, vol * 0.34);
        this.tone(ctx, 1318.5, "sine", t0 + 0.03, 0.65, vol * 0.18);
        this.tone(ctx, 1760, "triangle", t0, 0.35, vol * 0.08);
      } else if (kind === "bowl") {
        this.tone(ctx, 432, "sine", t0, 1.4, vol * 0.38);
        this.tone(ctx, 648, "sine", t0 + 0.02, 1.1, vol * 0.15);
        this.tone(ctx, 864, "sine", t0 + 0.04, 0.85, vol * 0.08);
      } else if (kind === "marimba") {
        const notes = [523.25, 659.25, 783.99, 1046.5];
        notes.forEach((freq, idx) => {
          this.tone(ctx, freq, "triangle", t0 + idx * 0.11, 0.42, vol * 0.28);
        });
      } else if (kind === "digital") {
        this.tone(ctx, 987.77, "sine", t0, 0.14, vol * 0.25);
        this.tone(ctx, 1318.51, "sine", t0 + 0.17, 0.2, vol * 0.25);
      }
    }

    tone(ctx, freq, type, startTime, duration, peakGain) {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = type;
      osc.frequency.setValueAtTime(freq, startTime);

      gain.gain.setValueAtTime(0.0001, startTime);
      gain.gain.exponentialRampToValueAtTime(Math.max(0.0002, peakGain), startTime + 0.015);
      gain.gain.exponentialRampToValueAtTime(0.0001, startTime + duration);

      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start(startTime);
      osc.stop(startTime + duration + 0.05);
    }

    syncAmbient(mode, volumePct, shouldPlay) {
      const nextMode = mode || "none";
      const nextVol = Math.max(0, Math.min(1, Number(volumePct ?? 35) / 100));
      const active = Boolean(shouldPlay && nextMode !== "none" && nextVol > 0);

      if (active === this.ambientActive && nextMode === this.ambientMode) {
        this.ambientVolume = nextVol;
        if (this.ambientGain && this.ctx) {
          this.ambientGain.gain.setTargetAtTime(nextVol * 0.25, this.ctx.currentTime, 0.15);
        }
        return;
      }

      this.stopAmbient();
      this.ambientMode = nextMode;
      this.ambientVolume = nextVol;
      this.ambientActive = active;

      if (!active) return;

      const ctx = this.ensureContext();
      if (!ctx) return;

      if (nextMode === "tick") {
        this.playTickPulse();
        this.tickTimer = window.setInterval(() => this.playTickPulse(), 1000);
        return;
      }

      const bufferSize = ctx.sampleRate * 3;
      const buffer = ctx.createBuffer(1, bufferSize, ctx.sampleRate);
      const data = buffer.getChannelData(0);

      if (nextMode === "brown") {
        let lastOut = 0;
        for (let i = 0; i < bufferSize; i++) {
          const white = Math.random() * 2 - 1;
          data[i] = (lastOut + 0.02 * white) / 1.02;
          lastOut = data[i];
          data[i] *= 3.2;
        }
      } else {
        // Warm rain (filtered pink-ish noise)
        let b0 = 0, b1 = 0, b2 = 0;
        for (let i = 0; i < bufferSize; i++) {
          const white = Math.random() * 2 - 1;
          b0 = 0.99765 * b0 + white * 0.099046;
          b1 = 0.96300 * b1 + white * 0.2965164;
          b2 = 0.57000 * b2 + white * 1.0526913;
          data[i] = (b0 + b1 + b2 + white * 0.1848) * 0.14;
        }
      }

      const src = ctx.createBufferSource();
      src.buffer = buffer;
      src.loop = true;

      const filter = ctx.createBiquadFilter();
      if (nextMode === "brown") {
        filter.type = "lowpass";
        filter.frequency.value = 340;
      } else {
        filter.type = "bandpass";
        filter.frequency.value = 1150;
        filter.Q.value = 0.55;
      }

      const gain = ctx.createGain();
      gain.gain.setValueAtTime(0.0001, ctx.currentTime);
      gain.gain.linearRampToValueAtTime(nextVol * 0.25, ctx.currentTime + 0.35);

      src.connect(filter);
      filter.connect(gain);
      gain.connect(ctx.destination);
      src.start();

      this.noiseSource = src;
      this.ambientGain = gain;
    }

    playTickPulse() {
      const ctx = this.ctx;
      if (!ctx || ctx.state !== "running") return;
      const t = ctx.currentTime;
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "triangle";
      osc.frequency.setValueAtTime(1100, t);
      osc.frequency.exponentialRampToValueAtTime(380, t + 0.025);
      gain.gain.setValueAtTime(Math.max(0.0002, this.ambientVolume * 0.08), t);
      gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.032);
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start(t);
      osc.stop(t + 0.04);
    }

    stopAmbient() {
      this.ambientActive = false;
      if (this.tickTimer !== null) {
        clearInterval(this.tickTimer);
        this.tickTimer = null;
      }
      if (this.noiseSource) {
        try {
          this.noiseSource.stop();
          this.noiseSource.disconnect();
        } catch (_) {}
        this.noiseSource = null;
      }
      if (this.ambientGain) {
        try {
          this.ambientGain.disconnect();
        } catch (_) {}
        this.ambientGain = null;
      }
    }
  }

  // --- App State ---
  const DIAL_RADIUS = 106;
  const DIAL_CIRCUMFERENCE = 2 * Math.PI * DIAL_RADIUS;
  const AMBIENT_CYCLE = ["none", "tick", "rain", "brown"];

  const state = {
    locale: detectInitialLocale(),
    view: "timer",
    snapshot: null,
    clockOffsetMs: 0,
    composerOpen: false,
    composerEditId: "",
    composerEst: 1,
    composerNoteOpen: false,
    menuOpen: false,
    settingsOpen: false,
    draftSettings: null,
    toastTimer: null
  };

  const sound = new SoundEngine();

  function t(key, vars) {
    const dict = I18N[state.locale] || I18N.en;
    let str = dict[key] ?? I18N.en[key] ?? key;
    if (vars) {
      for (const [k, v] of Object.entries(vars)) {
        str = str.replaceAll(`{${k}}`, String(v));
      }
    }
    return str;
  }

  function escapeHTML(str) {
    return String(str ?? "")
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;")
      .replaceAll("'", "&#39;");
  }

  function formatMMSS(sec) {
    const s = Math.max(0, Math.floor(sec));
    const m = Math.floor(s / 60);
    const rem = s % 60;
    return `${String(m).padStart(2, "0")}:${String(rem).padStart(2, "0")}`;
  }

  function formatDuration(mins) {
    const m = Math.max(0, Math.round(mins));
    const h = Math.floor(m / 60);
    const rem = m % 60;
    if (h > 0) {
      return t("hourShort", { h, m: rem });
    }
    return t("minShort", { m });
  }

  function currentRemainingSec(snap) {
    if (!snap || !snap.timer) return 0;
    const tm = snap.timer;
    if (!tm.running || !tm.endAtMs) {
      return tm.remainingSec;
    }
    const serverNow = Date.now() + state.clockOffsetMs;
    const diff = tm.endAtMs - serverNow;
    if (diff <= 0) return 0;
    return Math.min(tm.totalSec, Math.ceil(diff / 1000));
  }

  function showToast(msg) {
    const banner = document.getElementById("toast-banner");
    if (!banner) return;
    banner.textContent = msg;
    banner.hidden = false;
    if (state.toastTimer) clearTimeout(state.toastTimer);
    state.toastTimer = setTimeout(() => {
      banner.hidden = true;
    }, 4500);
  }

  function applySnapshot(res) {
    if (!res) return;
    if (res.locale) {
      state.locale = normalizeLocale(res.locale);
    }
    if (res.snapshot) {
      state.snapshot = res.snapshot;
      if (res.snapshot.serverNowMs) {
        state.clockOffsetMs = res.snapshot.serverNowMs - Date.now();
      }
    }
    renderAll();
  }

  async function rpc(method, params) {
    const res = await window.hyla.rpc(method, params || {});
    applySnapshot(res);
    return res;
  }

  // --- SVG Dial Ticks Init ---
  function initDialTicks() {
    const g = document.getElementById("dial-ticks");
    if (!g || g.childElementCount > 0) return;
    const frag = document.createDocumentFragment();
    for (let i = 0; i < 60; i++) {
      const deg = i * 6;
      const major = i % 5 === 0;
      const y1 = major ? 12 : 16;
      const y2 = 22;
      const line = document.createElementNS("http://www.w3.org/2000/svg", "line");
      line.setAttribute("x1", "140");
      line.setAttribute("y1", String(y1));
      line.setAttribute("x2", "140");
      line.setAttribute("y2", String(y2));
      line.setAttribute("transform", `rotate(${deg} 140 140)`);
      line.setAttribute("class", major ? "dial-tick major" : "dial-tick");
      frag.appendChild(line);
    }
    g.appendChild(frag);
  }

  // --- Render Functions ---
  function renderAll() {
    const snap = state.snapshot;
    if (!snap) return;

    const timer = snap.timer;
    const settings = snap.settings;
    document.documentElement.dataset.phase = timer.phase;
    document.title = `${formatMMSS(currentRemainingSec(snap))} · ${t("title")}`;

    renderTopbar(snap);
    renderDial(snap);
    renderFocusStrip(snap);
    renderTasks(snap);
    if (state.view === "report") {
      renderReport(snap);
    }

    const shouldPlayAmbient = timer.running && timer.phase === "pomodoro" && settings.ambientSound !== "none";
    sound.syncAmbient(settings.ambientSound, settings.ambientVolume, shouldPlayAmbient);
  }

  function renderTopbar(snap) {
    document.getElementById("brand-title").textContent = t("title");
    document.getElementById("nav-timer").textContent = t("tabTimer");
    document.getElementById("nav-report").textContent = t("tabReport");
    document.getElementById("settings-label").textContent = t("settings");

    document.getElementById("nav-timer").classList.toggle("active", state.view === "timer");
    document.getElementById("nav-report").classList.toggle("active", state.view === "report");
    document.getElementById("timer-view").hidden = state.view !== "timer";
    document.getElementById("report-view").hidden = state.view !== "report";

    const ambBtn = document.getElementById("ambient-btn");
    const ambMode = snap.settings.ambientSound || "none";
    const ambMap = { none: "ambNone", tick: "ambTick", rain: "ambRain", brown: "ambBrown" };
    document.getElementById("ambient-label").textContent = t(ambMap[ambMode] || "ambNone");
    const playing = snap.timer.running && snap.timer.phase === "pomodoro" && ambMode !== "none";
    ambBtn.classList.toggle("active", ambMode !== "none");
    ambBtn.classList.toggle("playing", playing);
  }

  function renderDial(snap) {
    const tm = snap.timer;
    const rem = currentRemainingSec(snap);
    const total = Math.max(1, tm.totalSec || 1);
    const remainingRatio = Math.max(0, Math.min(1, rem / total));
    const elapsedRatio = 1 - remainingRatio;

    document.getElementById("phase-pomodoro").textContent = t("pomodoro");
    document.getElementById("phase-short").textContent = t("shortBreak");
    document.getElementById("phase-long").textContent = t("longBreak");

    document.querySelectorAll(".phase-btn").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.phase === tm.phase);
    });

    const tagKey = tm.phase === "short_break" ? "shortTag" : tm.phase === "long_break" ? "longTag" : "focusTag";
    document.getElementById("dial-phase-tag").textContent = `${t(tagKey)} · #${tm.sessionNumber || 1}`;
    document.getElementById("dial-time").textContent = formatMMSS(rem);

    // Progress arc + tip
    const arc = document.getElementById("dial-progress");
    arc.style.strokeDasharray = DIAL_CIRCUMFERENCE.toFixed(2);
    arc.style.strokeDashoffset = (DIAL_CIRCUMFERENCE * elapsedRatio).toFixed(2);

    const tipWrap = document.getElementById("dial-tip-wrap");
    const angle = (elapsedRatio * 360).toFixed(2);
    tipWrap.setAttribute("transform", `rotate(${angle} 140 140)`);

    // Light up ticks proportional to elapsed fraction
    const litCount = Math.round(elapsedRatio * 60);
    const ticks = document.getElementById("dial-ticks").children;
    for (let i = 0; i < ticks.length; i++) {
      ticks[i].classList.toggle("lit", i < litCount);
    }

    // Cycle dots
    const dotsEl = document.getElementById("cycle-dots");
    const interval = Math.max(1, Math.min(12, snap.settings.longBreakInterval || 4));
    const cycle = tm.cycleCount || 0;
    dotsEl.innerHTML = "";
    for (let i = 0; i < interval; i++) {
      const dot = document.createElement("span");
      let cls = "cycle-dot";
      if (i < cycle) {
        cls += " done";
      } else if (i === cycle && tm.phase === "pomodoro") {
        cls += " current";
      }
      dot.className = cls;
      dotsEl.appendChild(dot);
    }

    // Primary Start / Pause button
    const startBtn = document.getElementById("start-btn");
    const startIcon = document.getElementById("start-icon");
    const startLabel = document.getElementById("start-label");
    startBtn.classList.toggle("running", tm.running);

    if (tm.running) {
      startLabel.textContent = t("pause");
      startIcon.innerHTML = `<svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor"><rect x="3" y="2.5" width="3.5" height="11" rx="1"/><rect x="9.5" y="2.5" width="3.5" height="11" rx="1"/></svg>`;
    } else {
      const pausedMidway = rem > 0 && rem < total;
      startLabel.textContent = pausedMidway ? t("resume") : t("start");
      startIcon.innerHTML = `<svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor"><path d="M4 2.8v10.4l8.8-5.2L4 2.8Z"/></svg>`;
    }

    document.getElementById("reset-btn").title = t("reset");
    document.getElementById("skip-btn").title = t("skip");
  }

  function renderFocusStrip(snap) {
    const tm = snap.timer;
    const badge = document.getElementById("focus-strip-badge");
    const text = document.getElementById("focus-strip-text");
    badge.textContent = `#${tm.sessionNumber || 1}`;

    const activeTask = (snap.tasks || []).find((x) => x.id === snap.activeTaskId);
    if (activeTask) {
      text.textContent = activeTask.title;
      return;
    }
    if (tm.phase === "short_break") {
      text.textContent = t("noTaskShort");
    } else if (tm.phase === "long_break") {
      text.textContent = t("noTaskLong");
    } else {
      text.textContent = t("noTaskFocus");
    }
  }

  function renderTasks(snap) {
    const tasks = snap.tasks || [];
    document.getElementById("tasks-title").textContent = t("tasks");
    document.getElementById("tasks-count").textContent = String(tasks.length);
    document.getElementById("add-task-label").textContent = t("addTask");
    document.getElementById("act-clear-finished").textContent = t("clearFinished");
    document.getElementById("act-clear-act").textContent = t("clearAct");
    document.getElementById("act-clear-all").textContent = t("clearAll");

    const listEl = document.getElementById("task-list");
    listEl.innerHTML = "";

    if (tasks.length === 0) {
      const empty = document.createElement("div");
      empty.className = "empty-state";
      empty.textContent = t("emptyTasks");
      listEl.appendChild(empty);
    } else {
      tasks.forEach((task, index) => {
        const row = document.createElement("div");
        const isActive = task.id === snap.activeTaskId;
        row.className = `task-row${isActive ? " active" : ""}${task.completed ? " completed" : ""}`;
        row.dataset.taskId = task.id;

        const check = document.createElement("button");
        check.type = "button";
        check.className = "task-check";
        check.innerHTML = task.completed
          ? `<svg viewBox="0 0 14 14" width="11" height="11" fill="none"><path d="M2.8 7.2 5.6 10l5.6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`
          : "";
        check.addEventListener("click", (e) => {
          e.stopPropagation();
          rpc("pomodoro.task.toggle", { id: task.id }).catch(console.error);
        });

        const main = document.createElement("div");
        main.className = "task-main";
        const title = document.createElement("div");
        title.className = "task-title";
        title.textContent = task.title;
        main.appendChild(title);
        if (task.note) {
          const note = document.createElement("div");
          note.className = "task-note";
          note.textContent = task.note;
          main.appendChild(note);
        }

        const right = document.createElement("div");
        right.className = "task-right";

        const pomos = document.createElement("span");
        pomos.className = "task-pomos";
        pomos.textContent = `${task.donePomos || 0} / ${task.estPomos || 1}`;
        right.appendChild(pomos);

        const actions = document.createElement("div");
        actions.className = "task-actions";

        if (index > 0) {
          const upBtn = makeMiniBtn("↑", "Move up", () => rpc("pomodoro.task.move", { id: task.id, delta: -1 }));
          actions.appendChild(upBtn);
        }
        if (index < tasks.length - 1) {
          const downBtn = makeMiniBtn("↓", "Move down", () => rpc("pomodoro.task.move", { id: task.id, delta: 1 }));
          actions.appendChild(downBtn);
        }
        const editBtn = makeMiniBtn("✎", "Edit", () => openComposerForEdit(task));
        const delBtn = makeMiniBtn("✕", "Delete", () => rpc("pomodoro.task.delete", { id: task.id }), true);
        actions.appendChild(editBtn);
        actions.appendChild(delBtn);

        right.appendChild(actions);
        row.appendChild(check);
        row.appendChild(main);
        row.appendChild(right);

        row.addEventListener("click", () => {
          rpc("pomodoro.task.select", { id: task.id }).catch(console.error);
        });

        listEl.appendChild(row);
      });
    }

    renderEstimator(snap);
    renderComposerText();
  }

  function makeMiniBtn(label, title, onClick, isDanger) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = `mini-icon-btn${isDanger ? " danger" : ""}`;
    btn.textContent = label;
    btn.title = title;
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      Promise.resolve(onClick()).catch(console.error);
    });
    return btn;
  }

  function renderEstimator(snap) {
    const tasks = snap.tasks || [];
    const bar = document.getElementById("estimator-bar");
    if (tasks.length === 0) {
      bar.hidden = true;
      return;
    }

    let totalEst = 0;
    let totalDone = 0;
    let remainingPomos = 0;

    for (const task of tasks) {
      const est = Math.max(1, task.estPomos || 1);
      const done = Math.max(0, task.donePomos || 0);
      totalEst += est;
      totalDone += done;
      if (!task.completed) {
        remainingPomos += Math.max(0, est - done);
      }
    }

    bar.hidden = false;
    document.getElementById("est-pomos-label").textContent = t("pomosLabel");
    document.getElementById("est-pomos-val").textContent = `${totalDone} / ${totalEst}`;
    document.getElementById("est-finish-label").textContent = t("finishAtLabel");

    const cfg = snap.settings;
    const tm = snap.timer;
    let remainSec = 0;
    if (remainingPomos > 0) {
      if (tm.phase === "pomodoro") {
        remainSec += currentRemainingSec(snap);
        const extraPomos = remainingPomos - 1;
        remainSec += extraPomos * cfg.pomodoroMinutes * 60;
      } else {
        remainSec += currentRemainingSec(snap);
        remainSec += remainingPomos * cfg.pomodoroMinutes * 60;
      }
      // Estimate breaks between remaining pomodoros
      const interval = Math.max(1, cfg.longBreakInterval || 4);
      let cycle = tm.cycleCount || 0;
      const breaksCount = Math.max(0, remainingPomos - 1);
      for (let i = 0; i < breaksCount; i++) {
        cycle++;
        if (cycle >= interval) {
          remainSec += cfg.longBreakMinutes * 60;
          cycle = 0;
        } else {
          remainSec += cfg.shortBreakMinutes * 60;
        }
      }
    }

    if (remainSec <= 0) {
      document.getElementById("est-finish-val").textContent = "—";
      return;
    }
    const finishDate = new Date(Date.now() + remainSec * 1000);
    const hh = String(finishDate.getHours()).padStart(2, "0");
    const mm = String(finishDate.getMinutes()).padStart(2, "0");
    document.getElementById("est-finish-val").textContent = `${hh}:${mm} (${formatDuration(remainSec / 60)})`;
  }

  function renderComposerText() {
    document.getElementById("comp-title").placeholder = t("taskPlaceholder");
    document.getElementById("comp-est-label").textContent = t("estPomos");
    document.getElementById("comp-note").placeholder = t("notePlaceholder");
    document.getElementById("comp-note-toggle").textContent = state.composerNoteOpen ? t("hideNote") : t("addNote");
    document.getElementById("comp-save").textContent = t("save");
    document.getElementById("comp-cancel").textContent = t("cancel");
    document.getElementById("comp-delete").textContent = t("delete");

    document.querySelectorAll(".chip-btn").forEach((chip) => {
      chip.classList.toggle("active", Number(chip.dataset.est) === state.composerEst);
    });
  }

  function openComposerForAdd() {
    state.composerOpen = true;
    state.composerEditId = "";
    state.composerEst = 1;
    state.composerNoteOpen = false;
    document.getElementById("comp-title").value = "";
    document.getElementById("comp-note").value = "";
    document.getElementById("comp-est-val").value = "1";
    document.getElementById("comp-note").hidden = true;
    document.getElementById("comp-delete").hidden = true;
    document.getElementById("task-composer").hidden = false;
    document.getElementById("add-task-btn").hidden = true;
    renderComposerText();
    document.getElementById("comp-title").focus();
  }

  function openComposerForEdit(task) {
    state.composerOpen = true;
    state.composerEditId = task.id;
    state.composerEst = Math.max(1, task.estPomos || 1);
    state.composerNoteOpen = Boolean(task.note);
    document.getElementById("comp-title").value = task.title || "";
    document.getElementById("comp-note").value = task.note || "";
    document.getElementById("comp-est-val").value = String(state.composerEst);
    document.getElementById("comp-note").hidden = !state.composerNoteOpen;
    document.getElementById("comp-delete").hidden = false;
    document.getElementById("task-composer").hidden = false;
    document.getElementById("add-task-btn").hidden = true;
    renderComposerText();
    document.getElementById("comp-title").focus();
  }

  function closeComposer() {
    state.composerOpen = false;
    state.composerEditId = "";
    document.getElementById("task-composer").hidden = true;
    document.getElementById("add-task-btn").hidden = false;
  }

  async function submitComposer() {
    const title = document.getElementById("comp-title").value.trim();
    if (!title) {
      document.getElementById("comp-title").focus();
      return;
    }
    const note = document.getElementById("comp-note").value.trim();
    const est = Math.max(1, Math.min(99, Number(document.getElementById("comp-est-val").value) || 1));

    if (state.composerEditId) {
      await rpc("pomodoro.task.update", { id: state.composerEditId, title, note, estPomos: est });
    } else {
      await rpc("pomodoro.task.add", { title, note, estPomos: est });
    }
    closeComposer();
  }

  // --- Report Rendering ---
  function renderReport(snap) {
    const rep = snap.report;
    if (!rep) return;

    document.getElementById("kpi-focus-label").textContent = t("todayFocus");
    document.getElementById("kpi-focus-val").textContent = formatDuration(rep.todayMinutes || 0);
    document.getElementById("kpi-focus-sub").textContent = t("allTimeSub", {
      pomos: rep.totalPomos || 0,
      dur: formatDuration(rep.totalMinutes || 0)
    });

    const goal = Math.max(1, rep.dailyGoal || 8);
    const done = rep.todayPomos || 0;
    const pct = Math.min(100, Math.round((done / goal) * 100));
    document.getElementById("kpi-pomos-label").textContent = t("todayPomos");
    document.getElementById("kpi-pomos-val").textContent = String(done);
    document.getElementById("kpi-goal-fill").style.width = `${pct}%`;
    document.getElementById("kpi-goal-sub").textContent = t("goalProgress", { done, goal });

    document.getElementById("kpi-streak-label").textContent = t("streak");
    document.getElementById("kpi-streak-val").textContent = t("daysUnit", { n: rep.streakDays || 0 });
    document.getElementById("kpi-total-sub").textContent = formatDuration(rep.totalMinutes || 0);

    document.getElementById("chart-title").textContent = t("last7Days");
    document.getElementById("chart-goal-legend").textContent = t("goalLine", { n: goal });
    render7DayChart(rep.last7Days || [], goal);

    document.getElementById("history-title").textContent = t("recentSessions");
    document.getElementById("clear-history-btn").textContent = t("clearHistory");

    const histEl = document.getElementById("history-list");
    histEl.innerHTML = "";
    const recent = rep.recent || [];
    if (recent.length === 0) {
      const empty = document.createElement("div");
      empty.className = "empty-state";
      empty.textContent = t("noHistory");
      histEl.appendChild(empty);
      return;
    }

    recent.forEach((rec) => {
      const row = document.createElement("div");
      row.className = "history-row";

      const title = document.createElement("span");
      title.className = "history-title";
      title.textContent = rec.taskTitle || t("untitledFocus");

      const meta = document.createElement("span");
      meta.className = "history-meta";
      const dt = new Date(rec.completedAt);
      const hhmm = `${String(dt.getHours()).padStart(2, "0")}:${String(dt.getMinutes()).padStart(2, "0")}`;
      meta.textContent = `${rec.day} ${hhmm} · ${formatDuration(rec.durationMin)}`;

      row.appendChild(title);
      row.appendChild(meta);
      histEl.appendChild(row);
    });
  }

  function render7DayChart(days, goal) {
    const wrap = document.getElementById("chart-wrap");
    const w = 680;
    const h = 184;
    const padL = 24;
    const padR = 16;
    const padT = 22;
    const padB = 28;
    const innerW = w - padL - padR;
    const innerH = h - padT - padB;

    const maxPomos = Math.max(goal, ...days.map((d) => d.pomos || 0), 4);
    const barGap = innerW / Math.max(1, days.length);
    const barW = Math.min(36, barGap * 0.52);

    const goalY = padT + innerH - (goal / maxPomos) * innerH;

    let barsSVG = "";
    days.forEach((d, idx) => {
      const pomos = d.pomos || 0;
      const barH = pomos > 0 ? Math.max(6, (pomos / maxPomos) * innerH) : 3;
      const cx = padL + idx * barGap + barGap / 2;
      const x = cx - barW / 2;
      const y = padT + innerH - barH;
      const label = (d.day || "").slice(5);
      const fill = pomos > 0 ? "url(#chart-bar-grad)" : "var(--dial-track)";

      barsSVG += `<rect x="${x.toFixed(1)}" y="${y.toFixed(1)}" width="${barW.toFixed(1)}" height="${barH.toFixed(1)}" rx="6" fill="${fill}" />`;
      if (pomos > 0) {
        barsSVG += `<text x="${cx.toFixed(1)}" y="${(y - 6).toFixed(1)}" text-anchor="middle" font-size="11" font-weight="600" fill="var(--hyla-text)">${pomos}</text>`;
      }
      barsSVG += `<text x="${cx.toFixed(1)}" y="${(h - 8).toFixed(1)}" text-anchor="middle" font-size="11" fill="var(--hyla-text-muted)">${escapeHTML(label)}</text>`;
    });

    wrap.innerHTML = `
      <svg class="chart-svg" viewBox="0 0 ${w} ${h}" preserveAspectRatio="xMidYMid meet">
        <defs>
          <linearGradient id="chart-bar-grad" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="var(--mode-accent)" />
            <stop offset="100%" stop-color="var(--mode-accent-2)" stop-opacity="0.72" />
          </linearGradient>
        </defs>
        <line x1="${padL}" y1="${(padT + innerH).toFixed(1)}" x2="${w - padR}" y2="${(padT + innerH).toFixed(1)}" stroke="var(--hyla-divider)" stroke-width="1" />
        <line x1="${padL}" y1="${goalY.toFixed(1)}" x2="${w - padR}" y2="${goalY.toFixed(1)}" stroke="var(--mode-accent)" stroke-opacity="0.45" stroke-width="1.2" stroke-dasharray="4 4" />
        ${barsSVG}
      </svg>`;
  }

  // --- Settings Modal ---
  function openSettingsModal() {
    if (!state.snapshot) return;
    state.draftSettings = { ...state.snapshot.settings };
    state.settingsOpen = true;
    renderSettingsModal();
    document.getElementById("modal-backdrop").hidden = false;
  }

  function closeSettingsModal() {
    state.settingsOpen = false;
    document.getElementById("modal-backdrop").hidden = true;
  }

  function renderSettingsModal() {
    const d = state.draftSettings;
    if (!d) return;

    document.getElementById("modal-title").textContent = t("settingsTitle");
    document.getElementById("modal-cancel-btn").textContent = t("cancel");
    document.getElementById("modal-save-btn").textContent = t("save");

    const body = document.getElementById("modal-body");
    body.innerHTML = `
      <div class="set-section">
        <div class="set-heading">${escapeHTML(t("secTimer"))}</div>
        <div class="set-grid-3">
          <div class="set-field">
            <label for="set-pomo">${escapeHTML(t("lblPomodoroMin"))}</label>
            <input class="set-input" id="set-pomo" type="number" min="1" max="180" value="${d.pomodoroMinutes}" />
          </div>
          <div class="set-field">
            <label for="set-short">${escapeHTML(t("lblShortMin"))}</label>
            <input class="set-input" id="set-short" type="number" min="1" max="60" value="${d.shortBreakMinutes}" />
          </div>
          <div class="set-field">
            <label for="set-long">${escapeHTML(t("lblLongMin"))}</label>
            <input class="set-input" id="set-long" type="number" min="1" max="90" value="${d.longBreakMinutes}" />
          </div>
        </div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblInterval"))}</span>
          <input class="set-input" id="set-interval" style="width:72px;text-align:center" type="number" min="1" max="12" value="${d.longBreakInterval}" />
        </div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblDailyGoal"))}</span>
          <input class="set-input" id="set-goal" style="width:72px;text-align:center" type="number" min="1" max="48" value="${d.dailyGoalPomos}" />
        </div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblAutoBreaks"))}</span>
          <label class="switch">
            <input id="set-autobreaks" type="checkbox" ${d.autoStartBreaks ? "checked" : ""} />
            <span class="switch-slider"></span>
          </label>
        </div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblAutoPomos"))}</span>
          <label class="switch">
            <input id="set-autopomos" type="checkbox" ${d.autoStartPomodoros ? "checked" : ""} />
            <span class="switch-slider"></span>
          </label>
        </div>
      </div>

      <div class="set-section">
        <div class="set-heading">${escapeHTML(t("secSound"))}</div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblAlarmSound"))}</span>
          <div class="set-row-controls">
            <select class="set-select" id="set-alarm" style="width:132px">
              <option value="bell" ${d.alarmSound === "bell" ? "selected" : ""}>${escapeHTML(t("soundBell"))}</option>
              <option value="bowl" ${d.alarmSound === "bowl" ? "selected" : ""}>${escapeHTML(t("soundBowl"))}</option>
              <option value="marimba" ${d.alarmSound === "marimba" ? "selected" : ""}>${escapeHTML(t("soundMarimba"))}</option>
              <option value="digital" ${d.alarmSound === "digital" ? "selected" : ""}>${escapeHTML(t("soundDigital"))}</option>
              <option value="none" ${d.alarmSound === "none" ? "selected" : ""}>${escapeHTML(t("soundNone"))}</option>
            </select>
            <button class="glass-btn small" type="button" id="set-test-alarm">${escapeHTML(t("testSound"))}</button>
          </div>
        </div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblAlarmVol"))}</span>
          <div class="set-row-controls">
            <input class="set-range" id="set-alarm-vol" type="range" min="0" max="100" value="${d.alarmVolume}" />
            <span id="set-alarm-vol-val" style="width:34px;text-align:right;font-variant-numeric:tabular-nums">${d.alarmVolume}%</span>
          </div>
        </div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblAlarmRepeat"))}</span>
          <input class="set-input" id="set-alarm-repeat" style="width:72px;text-align:center" type="number" min="1" max="5" value="${d.alarmRepeat}" />
        </div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblAmbientSound"))}</span>
          <select class="set-select" id="set-ambient" style="width:140px">
            <option value="none" ${d.ambientSound === "none" ? "selected" : ""}>${escapeHTML(t("ambNone"))}</option>
            <option value="tick" ${d.ambientSound === "tick" ? "selected" : ""}>${escapeHTML(t("ambTick"))}</option>
            <option value="rain" ${d.ambientSound === "rain" ? "selected" : ""}>${escapeHTML(t("ambRain"))}</option>
            <option value="brown" ${d.ambientSound === "brown" ? "selected" : ""}>${escapeHTML(t("ambBrown"))}</option>
          </select>
        </div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblAmbientVol"))}</span>
          <div class="set-row-controls">
            <input class="set-range" id="set-ambient-vol" type="range" min="0" max="100" value="${d.ambientVolume}" />
            <span id="set-ambient-vol-val" style="width:34px;text-align:right;font-variant-numeric:tabular-nums">${d.ambientVolume}%</span>
          </div>
        </div>
      </div>

      <div class="set-section">
        <div class="set-heading">${escapeHTML(t("secNotify"))}</div>
        <div class="set-row">
          <span class="set-row-label">${escapeHTML(t("lblDesktopNotify"))}</span>
          <label class="switch">
            <input id="set-notify" type="checkbox" ${d.desktopNotifications ? "checked" : ""} />
            <span class="switch-slider"></span>
          </label>
        </div>
      </div>
    `;

    document.getElementById("set-alarm-vol").addEventListener("input", (e) => {
      document.getElementById("set-alarm-vol-val").textContent = `${e.target.value}%`;
    });
    document.getElementById("set-ambient-vol").addEventListener("input", (e) => {
      document.getElementById("set-ambient-vol-val").textContent = `${e.target.value}%`;
    });
    document.getElementById("set-test-alarm").addEventListener("click", () => {
      sound.unlock();
      const kind = document.getElementById("set-alarm").value;
      const vol = Number(document.getElementById("set-alarm-vol").value);
      sound.playChime(kind, vol, 1);
    });
  }

  async function saveSettingsModal() {
    const next = {
      pomodoroMinutes: Number(document.getElementById("set-pomo").value) || 25,
      shortBreakMinutes: Number(document.getElementById("set-short").value) || 5,
      longBreakMinutes: Number(document.getElementById("set-long").value) || 15,
      longBreakInterval: Number(document.getElementById("set-interval").value) || 4,
      dailyGoalPomos: Number(document.getElementById("set-goal").value) || 8,
      autoStartBreaks: document.getElementById("set-autobreaks").checked,
      autoStartPomodoros: document.getElementById("set-autopomos").checked,
      alarmSound: document.getElementById("set-alarm").value,
      alarmVolume: Number(document.getElementById("set-alarm-vol").value),
      alarmRepeat: Number(document.getElementById("set-alarm-repeat").value) || 2,
      ambientSound: document.getElementById("set-ambient").value,
      ambientVolume: Number(document.getElementById("set-ambient-vol").value),
      desktopNotifications: document.getElementById("set-notify").checked
    };
    await rpc("pomodoro.settings.update", next);
    closeSettingsModal();
  }

  async function cycleAmbientSound() {
    if (!state.snapshot) return;
    sound.unlock();
    const cur = state.snapshot.settings.ambientSound || "none";
    const idx = AMBIENT_CYCLE.indexOf(cur);
    const nextMode = AMBIENT_CYCLE[(idx + 1) % AMBIENT_CYCLE.length];
    const next = { ...state.snapshot.settings, ambientSound: nextMode };
    await rpc("pomodoro.settings.update", next);
  }

  // --- Wire Events ---
  function wireEvents() {
    initDialTicks();
    detectThemeMode();

    document.querySelectorAll(".nav-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        state.view = btn.dataset.view;
        renderAll();
      });
    });

    document.querySelectorAll(".phase-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        rpc("pomodoro.setPhase", { phase: btn.dataset.phase }).catch(console.error);
      });
    });

    document.getElementById("start-btn").addEventListener("click", () => {
      sound.unlock();
      rpc("pomodoro.toggle").catch(console.error);
    });

    document.getElementById("reset-btn").addEventListener("click", () => {
      rpc("pomodoro.reset").catch(console.error);
    });

    document.getElementById("skip-btn").addEventListener("click", () => {
      rpc("pomodoro.skip").catch(console.error);
    });

    document.getElementById("ambient-btn").addEventListener("click", () => {
      cycleAmbientSound().catch(console.error);
    });

    document.getElementById("settings-btn").addEventListener("click", openSettingsModal);
    document.getElementById("modal-close-btn").addEventListener("click", closeSettingsModal);
    document.getElementById("modal-cancel-btn").addEventListener("click", closeSettingsModal);
    document.getElementById("modal-save-btn").addEventListener("click", () => {
      saveSettingsModal().catch(console.error);
    });
    document.getElementById("modal-backdrop").addEventListener("click", (e) => {
      if (e.target.id === "modal-backdrop") closeSettingsModal();
    });

    // Tasks menu
    const menuBtn = document.getElementById("tasks-menu-btn");
    const menuEl = document.getElementById("tasks-menu");
    menuBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      state.menuOpen = !state.menuOpen;
      menuEl.hidden = !state.menuOpen;
    });
    document.addEventListener("click", () => {
      if (state.menuOpen) {
        state.menuOpen = false;
        menuEl.hidden = true;
      }
    });

    document.getElementById("act-clear-finished").addEventListener("click", () => {
      menuEl.hidden = true;
      state.menuOpen = false;
      rpc("pomodoro.task.clearFinished").catch(console.error);
    });
    document.getElementById("act-clear-act").addEventListener("click", () => {
      menuEl.hidden = true;
      state.menuOpen = false;
      rpc("pomodoro.task.clearAct").catch(console.error);
    });
    document.getElementById("act-clear-all").addEventListener("click", () => {
      menuEl.hidden = true;
      state.menuOpen = false;
      rpc("pomodoro.task.clearAll").catch(console.error);
    });

    // Task composer
    document.getElementById("add-task-btn").addEventListener("click", openComposerForAdd);
    document.getElementById("comp-cancel").addEventListener("click", closeComposer);
    document.getElementById("comp-save").addEventListener("click", () => {
      submitComposer().catch(console.error);
    });
    document.getElementById("comp-delete").addEventListener("click", async () => {
      if (state.composerEditId) {
        await rpc("pomodoro.task.delete", { id: state.composerEditId });
      }
      closeComposer();
    });
    document.getElementById("comp-title").addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        submitComposer().catch(console.error);
      } else if (e.key === "Escape") {
        e.preventDefault();
        closeComposer();
      }
    });
    document.getElementById("comp-est-dec").addEventListener("click", () => {
      state.composerEst = Math.max(1, state.composerEst - 1);
      document.getElementById("comp-est-val").value = String(state.composerEst);
      renderComposerText();
    });
    document.getElementById("comp-est-inc").addEventListener("click", () => {
      state.composerEst = Math.min(99, state.composerEst + 1);
      document.getElementById("comp-est-val").value = String(state.composerEst);
      renderComposerText();
    });
    document.getElementById("comp-est-val").addEventListener("input", (e) => {
      state.composerEst = Math.max(1, Math.min(99, Number(e.target.value) || 1));
      renderComposerText();
    });
    document.querySelectorAll(".chip-btn").forEach((chip) => {
      chip.addEventListener("click", () => {
        state.composerEst = Number(chip.dataset.est) || 1;
        document.getElementById("comp-est-val").value = String(state.composerEst);
        renderComposerText();
      });
    });
    document.getElementById("comp-note-toggle").addEventListener("click", () => {
      state.composerNoteOpen = !state.composerNoteOpen;
      document.getElementById("comp-note").hidden = !state.composerNoteOpen;
      renderComposerText();
      if (state.composerNoteOpen) document.getElementById("comp-note").focus();
    });

    // Report clear history
    document.getElementById("clear-history-btn").addEventListener("click", () => {
      rpc("pomodoro.history.clear").catch(console.error);
    });

    // Global keyboard shortcuts
    document.addEventListener("keydown", (e) => {
      const tag = document.activeElement?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
        if (e.key === "Escape" && state.settingsOpen) {
          closeSettingsModal();
        }
        return;
      }
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      if (e.key === "Escape") {
        if (state.settingsOpen) closeSettingsModal();
        else if (state.composerOpen) closeComposer();
        else if (state.menuOpen) {
          state.menuOpen = false;
          menuEl.hidden = true;
        }
        return;
      }
      if (state.settingsOpen) return;

      if (e.code === "Space") {
        e.preventDefault();
        sound.unlock();
        rpc("pomodoro.toggle").catch(console.error);
      } else if (e.key === "r" || e.key === "R") {
        e.preventDefault();
        rpc("pomodoro.reset").catch(console.error);
      } else if (e.key === "n" || e.key === "N") {
        e.preventDefault();
        rpc("pomodoro.skip").catch(console.error);
      } else if (e.key === "1") {
        rpc("pomodoro.setPhase", { phase: "pomodoro" }).catch(console.error);
      } else if (e.key === "2") {
        rpc("pomodoro.setPhase", { phase: "short_break" }).catch(console.error);
      } else if (e.key === "3") {
        rpc("pomodoro.setPhase", { phase: "long_break" }).catch(console.error);
      } else if (e.key === "m" || e.key === "M") {
        cycleAmbientSound().catch(console.error);
      } else if (e.key === ",") {
        e.preventDefault();
        openSettingsModal();
      }
    });

    // Host push events
    if (window.hyla && typeof window.hyla.on === "function") {
      window.hyla.on("pomodoro.state", (payload) => {
        applySnapshot(payload);
      });
      window.hyla.on("pomodoro.phaseCompleted", (payload) => {
        applySnapshot(payload);
        const ev = payload?.event;
        if (ev) {
          sound.playChime(ev.alarmSound, ev.alarmVolume, ev.alarmRepeat);
          showToast(ev.completedPhase === "pomodoro" ? t("phaseDoneFocus") : t("phaseDoneBreak"));
        }
      });
      window.hyla.on("locale.changed", (payload) => {
        if (payload?.locale) {
          state.locale = normalizeLocale(payload.locale);
          renderAll();
        }
      });
      window.hyla.on("theme.changed", (payload) => {
        if (typeof payload?.dark === "boolean") {
          document.documentElement.dataset.theme = payload.dark ? "dark" : "light";
        } else {
          setTimeout(detectThemeMode, 80);
        }
      });
    }

    // Smooth UI countdown tick while timer runs
    window.setInterval(() => {
      if (state.snapshot && state.snapshot.timer && state.snapshot.timer.running) {
        renderDial(state.snapshot);
        renderEstimator(state.snapshot);
        document.title = `${formatMMSS(currentRemainingSec(state.snapshot))} · ${t("title")}`;
      }
    }, 200);
  }

  async function boot() {
    wireEvents();
    try {
      await rpc("pomodoro.bootstrap");
    } catch (err) {
      showToast(err?.message || "Failed to connect to Pomodoro plugin");
    }
  }

  boot();
})();
