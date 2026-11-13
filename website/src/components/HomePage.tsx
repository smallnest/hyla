import { For, createSignal, onCleanup, onMount } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { Link } from './Link';
import AppWindow from './AppWindow';
import CliShowcase from './CliShowcase';
import SessionWall from './SessionWall';
import HeroTerminal from './HeroTerminal';
import { ArrowUpRightIcon, BellIcon, BotIcon, DownloadIcon, LayersIcon, PauseIcon, PlayIcon, ServerIcon, TerminalIcon } from './Icons';
import { localize, pageHead, type Locale } from '../lib/i18n';

const icons = [LayersIcon, BellIcon, BotIcon, TerminalIcon, ServerIcon];
const featureLinks = ['/docs/sessions', '/docs/status', '/docs/agents', '/docs/configuration', '/docs/quick-start'];

// Every machine Hyla drives, or is made of: the ticker between the hero and
// the window.
const ticker = [
  'Claude Code',
  'Codex',
  'OpenCode',
  'Pi',
  'Gemini',
  'Ghostty',
  'tmux',
  'lazygit',
  'yazi',
  'Android',
  'pair code',
  'hyla CLI',
];

const LINUX_POCKET = 'https://github.com/smallnest/hyla-pocket';

const copy = {
  en: {
    title: 'Hyla — a terminal for coding agents',
    description:
      'Hyla is a terminal for coding agents. Sessions keep running when you quit, on your computer or another machine, and a phone reads them too.',
    eyebrow: 'Hyla · protocol 4',
    hero: ['Run your', 'agents.'],
    heroAccent: 'Keep your place.',
    lead: 'One window for every agent in flight. Sessions outlive the window, the sidebar says who needs you, and your phone reads the same sessions from anywhere.',
    getStarted: 'Get started',
    download: 'Download',
    stats: ['sessions outlive the window', 'agents: claude · codex · opencode', 'macos · windows · linux'],
    appKicker: '01 — the window',
    appTitle: 'Every agent, one window',
    appLead:
      'An agent reviews your change, lazygit shows the diff, and the program runs beside them. The sidebar answers the only question that matters: who needs you right now?',
    demoKicker: '02 — the loop',
    demoTitle: 'Start it, close it, come back',
    demoLead:
      'Sessions run in hylad, a daemon of their own: quit Hyla and they keep going, on this machine or another. Open it again and everything is where you left it.',
    demoLabels: { working: 'working', done: 'turn done' },
    cliKicker: '03 — the shell',
    cliTitle: 'Drive it from the shell',
    cliLead:
      "Start sessions, split panes, type into them, wait for them and read their screens, from your shell or your agent's.",
    featuresKicker: '04 — what is inside',
    featuresTitle: 'Built for many agents at once',
    featuresLead:
      'A terminal drawn by Ghostty, a daemon that keeps sessions, a CLI agents can drive, and a sidebar that tells the truth.',
    features: [
      {
        title: 'Sessions that outlive the window',
        body: 'Every session runs in hylad, which keeps it, its scrollback and its panes after Hyla closes.',
        points: ['survives quitting', 'restored after a reboot', 'grouped by project'],
        label: 'Sessions & projects',
      },
      {
        title: 'Know which one needs you',
        body: 'Working, waiting, idle, done or failed: the ones waiting on you stand out, and one shortcut takes you there.',
        points: ['⌘J jumps to it', 'notified when an agent finishes', 'dock badge counts them'],
        label: 'Status & notifications',
      },
      {
        title: 'A CLI agents can drive',
        body: 'Agents start dev servers, hand work to other agents, wait for tests and report their own progress.',
        points: ['split, send, wait, read'],
        label: 'Coding agents',
      },
      {
        title: 'Ghostty under the hood',
        body: 'Terminals render with libghostty: fast, correct and familiar.',
        points: ['every Ghostty theme', 'plain-text config'],
        label: 'Configuration',
      },
      {
        title: 'Another machine, or a phone',
        body: 'Reach another machine by its address and a password, or scan one code with your phone.',
        points: ['no relay needed', 'no account'],
        label: 'Remote access',
      },
    ],
    pocketKicker: '05 — android',
    pocketTitle: 'Hyla, in your pocket',
    pocketLead:
      'HylaPocket is an Android client for the sessions on this machine: it reaches hylad over tailcat by its pair code, or by the address and password of a network listener, and shows the same sessions, terminals and splits.',
    pocketPoints: [
      'Every session, live, with the panes of each',
      'Type with the terminal key row: esc, tab, ctrl, alt, arrows',
      'Split panes, and come back to where you left off',
    ],
    pocketMeta: ['signed APK', 'android 9+', 'free'],
    pocketDownload: 'Download APK',
    pocketReleases: 'All releases',
    pocketSource: 'Source',
    pauseTicker: 'Stop the scrolling band',
    playTicker: 'Start the scrolling band',
    ctaKicker: '06 — start',
    ctaTitle: 'Hop in.',
    ctaLead: 'Install Hyla, teach your agents to use it, and stop losing track of them.',
    ctaComment: '# after installing the command line tool',
    getStarted2: 'Quick start',
    cliReference: 'CLI reference',
  },
  ja: {
    title: 'Hyla — コーディングエージェントのためのターミナル',
    description:
      'Hyla はコーディングエージェントのためのターミナルです。終了してもセッションは動き続け、スマートフォンからも同じセッションを読めます。',
    eyebrow: 'Hyla · protocol 4',
    hero: ['走らせたまま、', '続きはそのまま。'],
    heroAccent: '',
    lead: '進行中のすべてのエージェントをひとつのウインドウに。セッションはウインドウより長生きし、サイドバーが対応が必要なものを教え、スマートフォンからも同じセッションを読めます。',
    getStarted: 'はじめる',
    download: 'ダウンロード',
    stats: ['セッションはウインドウより長生き', 'claude · codex · opencode', 'macos · windows · linux'],
    appKicker: '01 — ウインドウ',
    appTitle: 'すべてのエージェントを、ひとつのウインドウに',
    appLead:
      'エージェントが変更をレビューし、lazygit が差分を表示し、その横でプログラムが動く。対応が必要なセッションはサイドバーが教えてくれます。',
    demoKicker: '02 — ループ',
    demoTitle: '始めて、閉じて、また戻る',
    demoLead:
      'セッションは専用のデーモン hylad の中で動きます。Hyla を終了しても、別のマシンでも動き続け、再び開けばすべてが元のままです。',
    demoLabels: { working: '作業中', done: 'ターン完了' },
    cliKicker: '03 — シェル',
    cliTitle: 'シェルから操作する',
    cliLead:
      'あなたのシェルからでもエージェントのシェルからでも、セッションの開始、ペインの分割、入力の送信、完了の待機、画面の読み取りができます。',
    featuresKicker: '04 — 中身',
    featuresTitle: 'たくさんのエージェントを同時に',
    featuresLead:
      'Ghostty が描くターミナル、セッションを保つデーモン、エージェントが操作できる CLI、そして正直なサイドバー。',
    features: [
      {
        title: 'ウインドウより長生きするセッション',
        body: 'セッションは hylad の中で動き、Hyla を閉じた後もスクロールバックとペインを保ちます。',
        points: ['終了しても残る', '再起動後に復元', 'プロジェクトごとにまとまる'],
        label: 'セッションとプロジェクト',
      },
      {
        title: '対応が必要なものがわかる',
        body: '作業中、待機中、アイドル、完了、失敗。あなたを待っているものが目立ち、ショートカットひとつで移動できます。',
        points: ['⌘J で移動', '完了時に通知', 'Dock のバッジ'],
        label: '状態と通知',
      },
      {
        title: 'エージェントが操作できる CLI',
        body: 'エージェントは開発サーバーを起動し、作業を任せ、テストを待ち、進捗を報告できます。',
        points: ['split、send、wait、read'],
        label: 'コーディングエージェント',
      },
      {
        title: '中身は Ghostty',
        body: '描画は libghostty: 速く、正確で、おなじみです。',
        points: ['Ghostty の全テーマ', 'プレーンテキスト設定'],
        label: '設定',
      },
      {
        title: '別のマシンでも、スマートフォンでも',
        body: 'アドレスとパスワードで別のマシンへ。スマートフォンはコードをひとつスキャンするだけ。',
        points: ['リレー不要', 'アカウント不要'],
        label: 'リモートアクセス',
      },
    ],
    pocketKicker: '05 — android',
    pocketTitle: 'ポケットの中の Hyla',
    pocketLead:
      'HylaPocket はこのマシンのセッションを読む Android クライアントです。tailcat のペアコード、またはネットワークリスナーのアドレスとパスワードで hylad に接続し、同じセッション、ターミナル、分割を表示します。',
    pocketPoints: [
      'すべてのセッションをライブで、ペインごと',
      'esc、tab、ctrl、alt、矢印のキー行で入力',
      'ペインを分割し、前回の場所に戻る',
    ],
    pocketMeta: ['署名済み APK', 'android 9+', '無料'],
    pocketDownload: 'APK をダウンロード',
    pocketReleases: 'すべてのリリース',
    pocketSource: 'ソース',
    pauseTicker: '流れる帯を止める',
    playTicker: '流れる帯を再開する',
    ctaKicker: '06 — はじめる',
    ctaTitle: '飛び込もう。',
    ctaLead: 'Hyla をインストールし、エージェントに使い方を教え、見失うのをやめましょう。',
    ctaComment: '# コマンドラインツールをインストールした後',
    getStarted2: 'クイックスタート',
    cliReference: 'CLI リファレンス',
  },
  zh: {
    title: 'Hyla —— 为编程智能体打造的终端',
    description:
      'Hyla 是为编程智能体打造的终端。退出后会话照常运行，手机也能读取同一批会话。',
    eyebrow: 'Hyla · protocol 4',
    hero: ['智能体一直跑，', '关闭应用也不停。'],
    heroAccent: '',
    lead: '所有进行中的智能体都在一个窗口里。会话比窗口活得更久，边栏告诉你谁需要你，手机也能随时读到同一批会话。',
    getStarted: '快速上手',
    download: '下载',
    stats: ['会话比窗口活得更久', 'claude · codex · opencode', 'macos · windows · linux'],
    appKicker: '01 — 窗口',
    appTitle: '所有智能体，一个窗口',
    appLead:
      '一个智能体在审查你的改动，lazygit 显示差异，程序在一旁运行。边栏回答唯一重要的问题：现在谁需要你？',
    demoKicker: '02 — 循环',
    demoTitle: '启动、关掉、再回来',
    demoLead:
      '会话跑在专属守护进程 hylad 里：退出 Hyla、甚至换台机器都不影响，重新打开一切都在原处。',
    demoLabels: { working: '工作中', done: '本轮完成' },
    cliKicker: '03 — 命令行',
    cliTitle: '在 Shell 里驱动它',
    cliLead: '在你的 Shell 或智能体的 Shell 里新建、拆分、输入、等待并读取屏幕。',
    featuresKicker: '04 — 里面有什么',
    featuresTitle: '为同时跑很多智能体而生',
    featuresLead: 'Ghostty 渲染的终端、保活会话的守护进程、智能体能驱动的 CLI，还有一个不说谎的边栏。',
    features: [
      {
        title: '比窗口活得更久的会话',
        body: '每个会话都在 hylad 里，关闭 Hyla 后仍保留会话、回滚缓冲和窗格。',
        points: ['退出不中断', '重启后恢复', '按项目分组'],
        label: '会话与项目',
      },
      {
        title: '一眼看出谁需要你',
        body: '工作中、等待中、空闲、已完成、失败：等你的那个最醒目，一个快捷键就跳过去。',
        points: ['⌘J 直达', '完成时通知', '程序坞徽标计数'],
        label: '状态与通知',
      },
      {
        title: '智能体能驱动的 CLI',
        body: '智能体可以起开发服务器、把活交给别的智能体、等测试并报告进度。',
        points: ['split、send、wait、read'],
        label: '编程智能体',
      },
      {
        title: '底层是 Ghostty',
        body: '终端由 libghostty 渲染：快、准、熟悉。',
        points: ['全部 Ghostty 主题', '纯文本配置'],
        label: '配置',
      },
      {
        title: '另一台机器，或者一部手机',
        body: '用地址和密码连另一台机器；手机扫一个码就行。',
        points: ['不需要中继', '不需要账号'],
        label: '远程访问',
      },
    ],
    pocketKicker: '05 — android',
    pocketTitle: '口袋里的 Hyla',
    pocketLead:
      'HylaPocket 是读取这台机器会话的 Android 客户端：用 tailcat 配对码，或用网络监听的地址与密码连上 hylad，看到同样的会话、终端和分屏。',
    pocketPoints: ['每个会话实时同步，含所有窗格', '用终端键行输入：esc、tab、ctrl、alt、方向键', '可以分屏，回来后接着上次的位置'],
    pocketMeta: ['已签名 APK', 'android 9+', '免费'],
    pocketDownload: '下载 APK',
    pocketReleases: '所有版本',
    pocketSource: '源码',
    pauseTicker: '暂停滚动条',
    playTicker: '继续滚动条',
    ctaKicker: '06 — 开始',
    ctaTitle: '上车吧。',
    ctaLead: '安装 Hyla，教会智能体使用它，从此不再找不到它们。',
    ctaComment: '# 安装命令行工具之后',
    getStarted2: '快速上手',
    cliReference: '命令行参考',
  },
};

export function homeHead(locale: Locale) {
  return pageHead(locale, '/', copy[locale].title, copy[locale].description);
}

export default function HomePage(props: { locale: Locale }) {
  const c = () => copy[props.locale];
  const to = (path: string) => localize(path, props.locale);

  let main!: HTMLElement;
  let hero!: HTMLElement;
  const [tickerPaused, setTickerPaused] = createSignal(false);

  onMount(() => {
    const less = matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (less) return;
    // Reveal: nothing is hidden until the script says the page moves.
    main.classList.add('hx--anim');
    const rows = main.querySelectorAll<HTMLElement>('.hx-reveal');
    if (typeof IntersectionObserver !== 'function') {
      rows.forEach((el) => el.classList.add('is-in'));
      return;
    }
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (!e.isIntersecting) continue;
          e.target.classList.add('is-in');
          io.unobserve(e.target);
        }
      },
      { rootMargin: '0px 0px -12% 0px', threshold: 0.12 },
    );
    rows.forEach((el) => io.observe(el));
    onCleanup(() => io.disconnect());
  });

  // The hero's light follows the pointer, in one write a frame.
  let frame = 0;
  let at: { x: number; y: number } | undefined;
  const aim = (e: PointerEvent) => {
    at = { x: e.clientX, y: e.clientY };
    if (frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      if (!at || !hero) return;
      const r = hero.getBoundingClientRect();
      hero.style.setProperty('--hx-mx', `${((at.x - r.left) / r.width) * 100}%`);
      hero.style.setProperty('--hx-my', `${((at.y - r.top) / r.height) * 100}%`);
    });
  };
  onCleanup(() => {
    if (frame) cancelAnimationFrame(frame);
  });

  return (
    <main class="hx" ref={main}>
      {/* Hero */}
      <section class="hx-hero" ref={hero} onPointerMove={aim}>
        <div class="hx-hero__bg" aria-hidden="true">
          <div class="hx-grid" />
          <div class="hx-beams">
            <div class="hx-beam hx-beam--a" />
            <div class="hx-beam hx-beam--b" />
          </div>
          <div class="hx-spot" />
        </div>
        <div class="hx-wrap hx-hero__inner">
          <div>
            <p class="hx-eyebrow">
              <span class="hx-eyebrow__dot" />
              {c().eyebrow}
            </p>
            <h1 class="hx-title">
              <span class="hx-title__line">
                <span style={{ '--i': 0 }}>{c().hero[0]}</span>
              </span>
              <span class="hx-title__line hx-title__line--accent">
                <span style={{ '--i': 1 }}>{c().hero[1]}</span>
              </span>
            </h1>
            <p class="hx-lead">{c().lead}</p>
            <div class="hx-actions">
              <Link href={to('/docs/quick-start')} class="hx-btn hx-btn--primary">
                {c().getStarted} <ArrowUpRightIcon size={15} />
              </Link>
              <Link href={to('/download')} class="hx-btn">
                <DownloadIcon size={15} /> {c().download}
              </Link>
            </div>
            <ul class="hx-stats">
              <For each={c().stats}>{(s) => <li>{s}</li>}</For>
            </ul>
          </div>
          <div class="hx-art">
            <SessionWall />
            <div class="hx-art__scan" />
            <span class="hx-art__tag">sessions · live</span>
          </div>
        </div>
      </section>

      {/* Ticker: a band of what Hyla drives, which a reader can stop. */}
      <div class="hx-ticker" classList={{ 'is-paused': tickerPaused() }}>
        <div class="hx-ticker__mask" aria-hidden="true">
          <div class="hx-ticker__scroller">
            <For each={[0, 1]}>
              {() => (
                <div class="hx-ticker__row">
                  <For each={ticker}>{(t) => <span>{t}</span>}</For>
                </div>
              )}
            </For>
          </div>
        </div>
        <button
          type="button"
          class="hx-ticker__pause"
          aria-label={tickerPaused() ? c().playTicker : c().pauseTicker}
          title={tickerPaused() ? c().playTicker : c().pauseTicker}
          onClick={() => setTickerPaused(!tickerPaused())}
        >
          {tickerPaused() ? <PlayIcon size={13} /> : <PauseIcon size={13} />}
        </button>
      </div>

      {/* The window */}
      <section class="hx-section">
        <div class="hx-wrap">
          <header class="hx-head hx-reveal">
            <div class="hx-head__copy">
              <span class="hx-kicker">{c().appKicker}</span>
              <h2 class="hx-title2">{c().appTitle}</h2>
              <p class="hx-sub">{c().appLead}</p>
            </div>
          </header>
          <div class="hx-stage hx-reveal">
            <div class="stage__frame">
              <AppWindow />
            </div>
          </div>
        </div>
      </section>

      {/* The loop */}
      <section class="hx-section">
        <div class="hx-wrap hx-demo">
          <div class="hx-reveal">
            <span class="hx-kicker">{c().demoKicker}</span>
            <h2 class="hx-title2">{c().demoTitle}</h2>
            <p class="hx-sub">{c().demoLead}</p>
            <ul class="hx-stats">
              <li>hylad</li>
              <li>restore-sessions</li>
              <li>resume-agents</li>
            </ul>
          </div>
          <div class="hx-reveal">
            <HeroTerminal labels={c().demoLabels} />
          </div>
        </div>
      </section>

      {/* CLI */}
      <section class="hx-section">
        <div class="hx-wrap">
          <header class="hx-head hx-reveal">
            <div class="hx-head__copy">
              <span class="hx-kicker">{c().cliKicker}</span>
              <h2 class="hx-title2">{c().cliTitle}</h2>
              <p class="hx-sub">{c().cliLead}</p>
            </div>
            <Link href={to('/docs/cli')} class="hx-btn">
              {c().cliReference} <ArrowUpRightIcon size={15} />
            </Link>
          </header>
          <div class="hx-reveal">
            <CliShowcase />
          </div>
        </div>
      </section>

      {/* Features */}
      <section class="hx-section">
        <div class="hx-wrap">
          <header class="hx-head hx-reveal">
            <div class="hx-head__copy">
              <span class="hx-kicker">{c().featuresKicker}</span>
              <h2 class="hx-title2">{c().featuresTitle}</h2>
              <p class="hx-sub">{c().featuresLead}</p>
            </div>
          </header>
          <div class="hx-bento">
            <For each={c().features}>
              {(f, i) => (
                <Link
                  href={to(featureLinks[i()]!)}
                  class="hx-card hx-reveal"
                  classList={{ 'hx-card--wide': i() < 2 }}
                >
                  <span class="hx-card__num">{String(i() + 1).padStart(2, '0')}</span>
                  <Dynamic component={icons[i()]} size={22} class="hx-card__icon" />
                  <h3>{f.title}</h3>
                  <p>{f.body}</p>
                  <ul class="hx-card__list">
                    <For each={f.points}>{(p) => <li>{p}</li>}</For>
                  </ul>
                  <span class="hx-card__go">
                    {f.label} <ArrowUpRightIcon size={13} />
                  </span>
                </Link>
              )}
            </For>
          </div>
        </div>
      </section>

      {/* Pocket */}
      <section class="hx-section" id="pocket">
        <div class="hx-wrap">
          <div class="hx-pocket hx-reveal">
            <div>
              <span class="hx-kicker">{c().pocketKicker}</span>
              <h2 class="hx-title2">{c().pocketTitle}</h2>
              <p class="hx-sub">{c().pocketLead}</p>
              <ul class="hx-pocket__points">
                <For each={c().pocketPoints}>{(p) => <li>{p}</li>}</For>
              </ul>
              <div class="hx-actions">
                <a class="hx-btn hx-btn--primary" href={`${LINUX_POCKET}/releases/latest`}>
                  <DownloadIcon size={15} /> {c().pocketDownload}
                </a>
                <a class="hx-btn" href={`${LINUX_POCKET}/releases`}>
                  {c().pocketReleases} <ArrowUpRightIcon size={15} />
                </a>
                <a class="hx-btn" href={LINUX_POCKET}>
                  {c().pocketSource}
                </a>
              </div>
              <ul class="hx-pocket__meta">
                <For each={c().pocketMeta}>{(m) => <li>{m}</li>}</For>
              </ul>
            </div>
            <div class="hx-pocket__art" aria-hidden="true">
              <div class="hx-phone">
                <div class="hx-phone__notch" />
                <div class="hx-phone__screen">
                  <div class="hx-phone__row">
                    <i />
                    <b>review</b> <span>claude · working</span>
                  </div>
                  <div class="hx-phone__row hx-phone__row--dim">
                    <i />
                    <b>build</b> <span>npm test · waiting</span>
                  </div>
                  <div class="hx-phone__row hx-phone__row--dim">
                    <i />
                    <b>docs</b> <span>zsh · idle</span>
                  </div>
                  <pre class="hx-phone__code">{'$ git status\n M src/app.tsx\n$ '}</pre>
                  <div class="hx-phone__keys">
                    <i />
                    <i />
                    <i />
                    <i />
                    <i />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section class="hx-cta">
        <div class="hx-wrap">
          <span class="hx-kicker">{c().ctaKicker}</span>
          <h2 class="hx-cta__title">{c().ctaTitle}</h2>
          <p class="hx-cta__lead">{c().ctaLead}</p>
          <pre class="hx-cmdbox">
            <span>$</span> hyla skill setup{'\n'}
            <span>$</span> hyla new -n my-task -- claude
          </pre>
          <div class="hx-actions">
            <Link href={to('/docs/quick-start')} class="hx-btn hx-btn--primary">
              {c().getStarted2} <ArrowUpRightIcon size={15} />
            </Link>
            <Link href={to('/download')} class="hx-btn">
              <DownloadIcon size={15} /> {c().download}
            </Link>
          </div>
        </div>
      </section>
    </main>
  );
}
