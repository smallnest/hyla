(() => {
  const MAX_TURNS = 100;

  const I18N = {
    en: {
      title: "Timeline",
      refresh: "Refresh",
      empty: "No turns yet.",
      noPane: "No pane available.",
      waiting: "Waiting",
      failed: "Failed",
      think: "Think",
      text: "Text",
      tool: "Tool",
      sub: "Subagent",
      step: "Step",
      wait: "Waiting",
      fail: "Failed",
      loadError: "Unable to read timeline"
    },
    "zh-Hans": {
      title: "时间线",
      refresh: "刷新",
      empty: "暂无对话轮次。",
      noPane: "没有可显示的窗格。",
      waiting: "等待中",
      failed: "失败",
      think: "思考",
      text: "回复",
      tool: "工具",
      sub: "子代理",
      step: "步骤",
      wait: "等待",
      fail: "失败",
      loadError: "无法读取时间线"
    },
    ja: {
      title: "タイムライン",
      refresh: "更新",
      empty: "まだターンがありません。",
      noPane: "表示できるペインがありません。",
      waiting: "待機中",
      failed: "失敗",
      think: "思考",
      text: "応答",
      tool: "ツール",
      sub: "サブエージェント",
      step: "ステップ",
      wait: "待機",
      fail: "失敗",
      loadError: "タイムラインを取得できません"
    }
  };

  function resolveLocaleKey(raw) {
    const norm = String(raw || "").trim().replace(/_/g, "-").toLowerCase();
    if (norm === "zh-hans" || norm === "zh-cn" || norm === "zh-sg" || norm === "zh" || norm.startsWith("zh-hans")) {
      return "zh-Hans";
    }
    if (norm === "ja" || norm.startsWith("ja-")) {
      return "ja";
    }
    if (norm === "en" || norm.startsWith("en-")) {
      return "en";
    }
    return "en";
  }

  let locale = resolveLocaleKey(new URLSearchParams(window.location.search).get("locale") || "en");
  let overrideTargetPane = "";
  let expandedTurns = new Set();
  let lastSnapshot = null;
  let pollTimer = null;
  let dirtyTimer = null;
  let inFlight = false;
  let queuedRefresh = false;

  const paneSelect = document.getElementById("pane-select");
  const refreshBtn = document.getElementById("refresh-btn");
  const contentEl = document.getElementById("content");

  function dict() {
    return I18N[resolveLocaleKey(locale)] || I18N.en;
  }

  function kindLabel(kind) {
    const t = dict();
    if (kind === "think") return t.think;
    if (kind === "text") return t.text;
    if (kind === "tool") return t.tool;
    if (kind === "subagent") return t.sub;
    if (kind === "step") return t.step;
    return kind || "";
  }

  function applyStaticLocale() {
    const t = dict();
    document.title = t.title;
    document.documentElement.lang = resolveLocaleKey(locale);
    refreshBtn.textContent = t.refresh;
    document.querySelectorAll("[data-i18n]").forEach((el) => {
      const k = el.getAttribute("data-i18n");
      if (t[k]) el.textContent = t[k];
    });
  }

  function formatClock(ms) {
    if (!ms) return "";
    const d = new Date(ms);
    const hh = String(d.getHours()).padStart(2, "0");
    const mm = String(d.getMinutes()).padStart(2, "0");
    return `${hh}:${mm}`;
  }

  function formatDuration(ms) {
    const s = Math.max(0, Math.floor((ms || 0) / 1000));
    if (s < 60) return `${s}s`;
    if (s < 3600) {
      const m = Math.floor(s / 60);
      const rem = String(s % 60).padStart(2, "0");
      return `${m}m ${rem}s`;
    }
    const h = Math.floor(s / 3600);
    const m = String(Math.floor((s % 3600) / 60)).padStart(2, "0");
    return `${h}h ${m}m`;
  }

  function stateLabel(state, ms) {
    const d = formatDuration(ms);
    const t = dict();
    if (state === "waiting") return `${t.waiting} · ${d}`;
    if (state === "failed") return `${t.failed} · ${d}`;
    return d;
  }

  function lanesOf(steps) {
    const first = [];
    const subs = [];
    for (const s of steps || []) {
      if (s.kind === "subagent") {
        subs.push([s]);
        continue;
      }
      if (s.kind !== "step") {
        first.push(s);
        continue;
      }
      for (const ch of s.children || []) {
        if (ch.kind === "think" || ch.kind === "text") {
          first.push(ch);
        }
      }
    }
    if (first.length === 0 && subs.length === 0) {
      return [steps || []];
    }
    return [first, ...subs];
  }

  function flattenSteps(steps) {
    const out = [];
    for (const s of steps || []) {
      if (s.kind === "step" && Array.isArray(s.children) && s.children.length > 0) {
        for (const ch of s.children) {
          out.push(ch);
        }
      } else {
        out.push(s);
      }
    }
    return out;
  }

  function spanOf(step, turnStart, turnSpan) {
    const end = step.end && step.end > 0 ? step.end : turnStart + turnSpan;
    let at = (step.start - turnStart) / turnSpan;
    if (at >= 1) return null;
    if (at < 0) at = 0;
    const w = Math.min((end - step.start) / turnSpan, 1 - at);
    if (w <= 0) return null;
    return { leftPct: at * 100, widthPct: w * 100 };
  }

  function makeStepRow(s, fallbackEnd) {
    const row = document.createElement("div");
    row.className = "step-row";

    const dot = document.createElement("span");
    dot.className = `step-dot ${s.state || "done"}`;
    row.appendChild(dot);

    const title = document.createElement("span");
    title.className = "step-title";
    title.textContent = s.title || kindLabel(s.kind);
    title.title = title.textContent;
    row.appendChild(title);

    if (s.detail) {
      const detail = document.createElement("span");
      detail.className = "step-detail";
      detail.textContent = s.detail;
      detail.title = s.detail;
      row.appendChild(detail);
    } else {
      const spacer = document.createElement("span");
      spacer.className = "step-detail";
      row.appendChild(spacer);
    }

    const end = s.end && s.end > 0 ? s.end : fallbackEnd || s.start;
    const dur = document.createElement("span");
    dur.className = `step-dur ${s.state || "done"}`;
    dur.textContent = stateLabel(s.state, end - s.start);
    row.appendChild(dur);
    return row;
  }

  function renderNote(text) {
    contentEl.textContent = "";
    const box = document.createElement("div");
    box.className = "note-box";
    box.textContent = text;
    contentEl.appendChild(box);
  }

  function renderHeader(snap) {
    const cands = snap.candidates || [];

    if (cands.length > 1) {
      paneSelect.hidden = false;
      paneSelect.textContent = "";
      for (const c of cands) {
        const opt = document.createElement("option");
        opt.value = c.id;
        opt.textContent = c.label || c.id;
        if (c.id === snap.targetPane) {
          opt.selected = true;
        }
        paneSelect.appendChild(opt);
      }
    } else {
      paneSelect.hidden = true;
    }
  }

  function renderSnapshot(snap) {
    lastSnapshot = snap;
    if (snap.locale) {
      locale = resolveLocaleKey(snap.locale);
    }
    applyStaticLocale();
    renderHeader(snap);

    const t = dict();
    if (!snap.targetPane) {
      renderNote(t.noPane);
      return;
    }
    const tl = snap.timeline;
    if (!tl) {
      renderNote(snap.error ? `${t.loadError}: ${snap.error}` : t.empty);
      return;
    }
    const turns = tl.turns || [];
    if (turns.length === 0) {
      renderNote(tl.note || snap.error || t.empty);
      return;
    }

    contentEl.textContent = "";
    const card = document.createElement("div");
    card.className = "turns-card";

    const readAt = tl.read || Date.now();
    const minIndex = Math.max(0, turns.length - MAX_TURNS);

    for (let i = turns.length - 1; i >= minIndex; i--) {
      const turn = turns[i];
      const turnKey = `${turn.start}:${turn.title || i}`;
      const end = turn.end && turn.end > 0 ? turn.end : readAt;
      const span = Math.max(1, end - turn.start);

      const el = document.createElement("div");
      el.className = "turn";
      el.addEventListener("click", () => {
        if (expandedTurns.has(turnKey)) {
          expandedTurns.delete(turnKey);
        } else {
          expandedTurns.add(turnKey);
        }
        renderSnapshot(snap);
      });

      const hdr = document.createElement("div");
      hdr.className = "turn-header";

      const clk = document.createElement("span");
      clk.className = "turn-clock";
      clk.textContent = formatClock(turn.start);
      hdr.appendChild(clk);

      const ttl = document.createElement("span");
      ttl.className = "turn-title";
      ttl.textContent = turn.title || "";
      ttl.title = turn.title || "";
      hdr.appendChild(ttl);

      const st = document.createElement("span");
      st.className = `turn-state ${turn.state || "done"}`;
      st.textContent = stateLabel(turn.state, end - turn.start);
      hdr.appendChild(st);
      el.appendChild(hdr);

      const lanesEl = document.createElement("div");
      lanesEl.className = "lanes";
      for (const laneSteps of lanesOf(turn.children)) {
        const lane = document.createElement("div");
        lane.className = "lane";
        for (const step of laneSteps) {
          const sp = spanOf(step, turn.start, span);
          if (!sp) continue;
          const bar = document.createElement("div");
          bar.className = `bar kind-${step.kind || "tool"} state-${step.state || "done"}`;
          bar.style.left = `${sp.leftPct.toFixed(3)}%`;
          bar.style.width = `${sp.widthPct.toFixed(3)}%`;
          const stepEnd = step.end && step.end > 0 ? step.end : end;
          const kLbl = kindLabel(step.kind);
          const sTitle = step.title && step.title !== step.kind ? step.title : kLbl;
          const tip = [
            kLbl && sTitle !== kLbl ? `${kLbl}: ${sTitle}` : sTitle,
            step.detail || "",
            stateLabel(step.state, stepEnd - step.start)
          ].filter(Boolean).join(" · ");
          bar.title = tip;
          lane.appendChild(bar);
        }
        lanesEl.appendChild(lane);
      }

      if (readAt > turn.start && readAt < turn.start + span) {
        const cur = document.createElement("div");
        cur.className = "read-cursor";
        cur.style.left = `${(((readAt - turn.start) / span) * 100).toFixed(3)}%`;
        lanesEl.appendChild(cur);
      }
      el.appendChild(lanesEl);

      for (const ch of turn.children || []) {
        if (ch.state === "waiting" || ch.state === "failed") {
          el.appendChild(makeStepRow(ch, end));
        }
      }

      if (expandedTurns.has(turnKey)) {
        const allSteps = flattenSteps(turn.children);
        if (allSteps.length > 0) {
          const det = document.createElement("div");
          det.className = "details-list";
          for (const step of allSteps) {
            det.appendChild(makeStepRow(step, end));
          }
          el.appendChild(det);
        }
      }

      card.appendChild(el);
    }

    if (snap.error) {
      const errRow = document.createElement("div");
      errRow.className = "inline-note";
      errRow.textContent = `${t.loadError}: ${snap.error}`;
      card.appendChild(errRow);
    }

    contentEl.appendChild(card);
  }

  function schedulePoll(ms) {
    if (pollTimer) {
      clearTimeout(pollTimer);
    }
    const delay = typeof ms === "number" && ms >= 1000 ? ms : 30000;
    pollTimer = setTimeout(() => {
      refresh();
    }, delay);
  }

  async function refresh() {
    if (inFlight) {
      queuedRefresh = true;
      return;
    }
    inFlight = true;
    try {
      const params = overrideTargetPane ? { targetPane: overrideTargetPane } : {};
      const snap = await window.hyla.rpc("timeline.snapshot", params);
      renderSnapshot(snap);
      schedulePoll(snap.pollMs);
    } catch (err) {
      renderNote(`${dict().loadError}: ${err && err.message ? err.message : String(err)}`);
      schedulePoll(5000);
    } finally {
      inFlight = false;
      if (queuedRefresh) {
        queuedRefresh = false;
        refresh();
      }
    }
  }

  paneSelect.addEventListener("change", () => {
    overrideTargetPane = paneSelect.value || "";
    refresh();
  });

  refreshBtn.addEventListener("click", () => {
    refresh();
  });

  if (window.hyla && typeof window.hyla.on === "function") {
    window.hyla.on("timeline.dirty", () => {
      if (dirtyTimer) clearTimeout(dirtyTimer);
      dirtyTimer = setTimeout(() => {
        refresh();
      }, 250);
    });
    window.hyla.on("locale.changed", (data) => {
      if (data && data.locale) {
        locale = resolveLocaleKey(data.locale);
      }
      applyStaticLocale();
      if (lastSnapshot) {
        lastSnapshot.locale = locale;
        renderSnapshot(lastSnapshot);
      }
      refresh();
    });
    window.hyla.on("theme.changed", () => {
      applyStaticLocale();
    });
  }

  applyStaticLocale();
  refresh();
})();
