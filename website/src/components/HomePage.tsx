import { For, createSignal, onCleanup, onMount } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { Link } from './Link';
import CliShowcase from './CliShowcase';
import SessionWall from './SessionWall';
import HeroTerminal from './HeroTerminal';
import {
  ArrowUpRightIcon,
  BellIcon,
  BotIcon,
  DownloadIcon,
  LayersIcon,
  PauseIcon,
  PhoneIcon,
  PlayIcon,
  ServerIcon,
  TerminalIcon,
} from './Icons';
import { localize, pageHead, type Locale } from '../lib/i18n';

const icons = [TerminalIcon, LayersIcon, ServerIcon, BellIcon, BotIcon, PhoneIcon];
const featureLinks = [
  '/docs/configuration',
  '/docs/panes',
  '/docs/sessions',
  '/docs/status',
  '/docs/agents',
  '/docs/quick-start',
];

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

// Each card says where it comes from: the terminal Hyla forked, or Hyla
// itself.
type Feature = {
  from: 'kero' | 'hyla';
  title: string;
  body: string;
  points: string[];
  label: string;
};

const copy = {
  en: {
    title: 'Hyla — a terminal for coding agents',
    description:
      'A terminal for coding agents: sessions outlive the window, the sidebar says who needs you, and a phone reads the same sessions.',
    eyebrow: 'Hyla · protocol 4',
    hero: ['Run your', 'agents.'],
    lead: 'One window for every agent in flight. Sessions outlive the window, and the sidebar says who needs you.',
    getStarted: 'Get started',
    download: 'Download',
    stats: ['sessions outlive the window', 'ghostty · panes · themes', 'macos · windows · linux'],
    appKicker: '01 — the window',
    appTitle: 'Every agent, one window',
    appLead: 'An agent reviews, lazygit shows the diff, and the dev server runs beside them.',
    appAlt: 'Hyla’s window: a session list in the sidebar, and an agent working in the pane beside it',
    demoKicker: '02 — the loop',
    demoTitle: 'Start it, close it, come back',
    demoLead: 'Sessions live in hylad, so quitting Hyla leaves them running.',
    demoLabels: { working: 'working', done: 'turn done' },
    cliKicker: '03 — the shell',
    cliTitle: 'Drive it from the shell',
    cliLead: 'Start, split, type, wait and read — from your shell or your agent’s.',
    featuresKicker: '04 — what is inside',
    featuresTitle: 'Kero’s terminal, Hyla’s sessions',
    featuresLead: 'Rendering, panes and config come from Kero. Sessions, agents and phones are Hyla’s own.',
    features: [
      {
        from: 'kero',
        title: 'Ghostty draws it',
        body: 'libghostty renders every pane: fast, correct, familiar.',
        points: ['every Ghostty theme', 'plain-text config'],
        label: 'Configuration',
      },
      {
        from: 'kero',
        title: 'Panes, tabs, splits',
        body: 'Split, zoom and rearrange; tmux habits keep working.',
        points: ['tmux-style keys', 'drag to split'],
        label: 'Panes',
      },
      {
        from: 'hyla',
        title: 'Sessions outlive the window',
        body: 'Every session runs in hylad, scrollback and panes included.',
        points: ['survives quitting', 'back after a reboot'],
        label: 'Sessions',
      },
      {
        from: 'hyla',
        title: 'Know which one needs you',
        body: 'Working, waiting, done: the one waiting on you stands out.',
        points: ['⌘J jumps to it', 'notified when a turn ends'],
        label: 'Status',
      },
      {
        from: 'hyla',
        title: 'Agents drive the CLI',
        body: 'They split panes, wait for tests and report progress. Feishu chats run the same turns.',
        points: ['split · send · wait · read', 'Feishu / Lark'],
        label: 'Agents',
      },
      {
        from: 'hyla',
        title: 'Another machine, or a phone',
        body: 'An address and a password reach another machine; the APK reads the same sessions.',
        points: ['no relay, no account', 'Android APK'],
        label: 'Remote',
      },
    ] as Feature[],
    pocketKicker: '05 — android',
    pocketTitle: 'Hyla, in your pocket',
    pocketLead: 'HylaPocket reads this machine’s sessions from an Android phone.',
    pocketAlt: 'Hyla Pocket on Android: a session’s transcript and the terminal key row',
    pocketPoints: ['Every session, with its panes', 'Key row: esc, tab, ctrl, alt', 'Split, and pick up where you left off'],
    pocketMeta: ['signed APK', 'android 9+', 'free'],
    pocketDownload: 'Download APK',
    pocketReleases: 'All releases',
    pocketSource: 'Source',
    pauseTicker: 'Stop the scrolling band',
    playTicker: 'Start the scrolling band',
    ctaKicker: '06 — start',
    ctaTitle: 'Hop in.',
    ctaLead: 'Install Hyla, and stop losing track of your agents.',
    getStarted2: 'Quick start',
    cliReference: 'CLI reference',
  },
  ja: {
    title: 'Hyla — コーディングエージェントのためのターミナル',
    description:
      'コーディングエージェントのためのターミナル。セッションは終了後も動き続け、サイドバーが対応が必要なものを教え、スマートフォンからも同じセッションを読めます。',
    eyebrow: 'Hyla · protocol 4',
    hero: ['走らせたまま、', '続きはそのまま。'],
    lead: '進行中のエージェントをひとつのウインドウに。セッションはウインドウより長生きし、サイドバーが対応が必要なものを教えます。',
    getStarted: 'はじめる',
    download: 'ダウンロード',
    stats: ['セッションはウインドウより長生き', 'ghostty · ペイン · テーマ', 'macos · windows · linux'],
    appKicker: '01 — ウインドウ',
    appTitle: 'すべてのエージェントを、ひとつのウインドウに',
    appLead: 'エージェントがレビューし、lazygit が差分を出し、その横で開発サーバーが動く。',
    appAlt: 'Hyla のウインドウ: サイドバーのセッション一覧と、その横で動くエージェント',
    demoKicker: '02 — ループ',
    demoTitle: '始めて、閉じて、また戻る',
    demoLead: 'セッションは hylad の中で動くので、Hyla を終了してもそのままです。',
    demoLabels: { working: '作業中', done: 'ターン完了' },
    cliKicker: '03 — シェル',
    cliTitle: 'シェルから操作する',
    cliLead: '開始、分割、入力、待機、読み取り。あなたのシェルからでも、エージェントのシェルからでも。',
    featuresKicker: '04 — 中身',
    featuresTitle: 'Kero のターミナル、Hyla のセッション',
    featuresLead: '描画、ペイン、設定は Kero から。セッション、エージェント、スマートフォンは Hyla のもの。',
    features: [
      {
        from: 'kero',
        title: '描画は Ghostty',
        body: 'libghostty がすべてのペインを描きます: 速く、正確で、おなじみ。',
        points: ['Ghostty の全テーマ', 'プレーンテキスト設定'],
        label: '設定',
      },
      {
        from: 'kero',
        title: 'ペイン、タブ、分割',
        body: '分割、ズーム、並べ替え。tmux の習慣もそのまま。',
        points: ['tmux 風のキー', 'ドラッグで分割'],
        label: 'ペイン',
      },
      {
        from: 'hyla',
        title: 'ウインドウより長生きするセッション',
        body: 'セッションは hylad の中で、スクロールバックもペインも保ちます。',
        points: ['終了しても残る', '再起動後に復元'],
        label: 'セッション',
      },
      {
        from: 'hyla',
        title: '対応が必要なものがわかる',
        body: '作業中、待機中、完了。待っているものが目立ちます。',
        points: ['⌘J で移動', '完了時に通知'],
        label: '状態',
      },
      {
        from: 'hyla',
        title: 'エージェントが CLI を動かす',
        body: 'ペインを分割し、テストを待ち、進捗を報告。Feishu の会話も同じターンです。',
        points: ['split · send · wait · read', 'Feishu / Lark'],
        label: 'エージェント',
      },
      {
        from: 'hyla',
        title: '別のマシンでも、スマートフォンでも',
        body: 'アドレスとパスワードで別のマシンへ。APK は同じセッションを読みます。',
        points: ['リレーもアカウントも不要', 'Android APK'],
        label: 'リモート',
      },
    ] as Feature[],
    pocketKicker: '05 — android',
    pocketTitle: 'ポケットの中の Hyla',
    pocketLead: 'HylaPocket は、このマシンのセッションを Android から読むクライアントです。',
    pocketAlt: 'Android の Hyla Pocket: セッションの記録とターミナルのキー行',
    pocketPoints: ['すべてのセッションを、ペインごと', 'キー行: esc、tab、ctrl、alt', '分割して、前回の場所へ'],
    pocketMeta: ['署名済み APK', 'android 9+', '無料'],
    pocketDownload: 'APK をダウンロード',
    pocketReleases: 'すべてのリリース',
    pocketSource: 'ソース',
    pauseTicker: '流れる帯を止める',
    playTicker: '流れる帯を再開する',
    ctaKicker: '06 — はじめる',
    ctaTitle: '飛び込もう。',
    ctaLead: 'Hyla を入れて、エージェントを見失うのをやめましょう。',
    getStarted2: 'クイックスタート',
    cliReference: 'CLI リファレンス',
  },
  zh: {
    title: 'Hyla —— 为编程智能体打造的终端',
    description: '为编程智能体打造的终端：退出后会话照常运行，边栏告诉你谁需要你，手机也能读同一批会话。',
    eyebrow: 'Hyla · protocol 4',
    hero: ['智能体一直跑，', '关闭应用也不停。'],
    lead: '所有进行中的智能体都在一个窗口里。会话比窗口活得更久，边栏告诉你谁需要你。',
    getStarted: '快速上手',
    download: '下载',
    stats: ['会话比窗口活得更久', 'ghostty · 窗格 · 主题', 'macos · windows · linux'],
    appKicker: '01 — 窗口',
    appTitle: '所有智能体，一个窗口',
    appLead: '一个智能体在审查改动，lazygit 显示差异，开发服务器在一旁运行。',
    appAlt: 'Hyla 窗口：边栏是会话列表，旁边是一个正在工作的智能体',
    demoKicker: '02 — 循环',
    demoTitle: '启动、关掉、再回来',
    demoLead: '会话跑在 hylad 里，退出 Hyla 也不影响。',
    demoLabels: { working: '工作中', done: '本轮完成' },
    cliKicker: '03 — 命令行',
    cliTitle: '在 Shell 里驱动它',
    cliLead: '新建、拆分、输入、等待、读屏：在你的 Shell 里，或在智能体的 Shell 里。',
    featuresKicker: '04 — 里面有什么',
    featuresTitle: 'Kero 的终端，Hyla 的会话',
    featuresLead: '渲染、窗格和配置来自 Kero；会话、智能体和手机是 Hyla 自己的。',
    features: [
      {
        from: 'kero',
        title: '底层是 Ghostty',
        body: 'libghostty 渲染每个窗格：快、准、熟悉。',
        points: ['全部 Ghostty 主题', '纯文本配置'],
        label: '配置',
      },
      {
        from: 'kero',
        title: '窗格、标签与分屏',
        body: '拆分、缩放、重排；tmux 的习惯照旧。',
        points: ['tmux 风格按键', '拖动即分屏'],
        label: '窗格',
      },
      {
        from: 'hyla',
        title: '比窗口活得更久的会话',
        body: '每个会话都在 hylad 里，回滚缓冲和窗格一起保留。',
        points: ['退出不中断', '重启后恢复'],
        label: '会话',
      },
      {
        from: 'hyla',
        title: '一眼看出谁需要你',
        body: '工作中、等待中、已完成：等你的那个最醒目。',
        points: ['⌘J 直达', '完成时通知'],
        label: '状态',
      },
      {
        from: 'hyla',
        title: '智能体能驱动 CLI',
        body: '拆分窗格、等待测试、报告进度；飞书的对话也是同一轮会话。',
        points: ['split · send · wait · read', '飞书 / Lark'],
        label: '智能体',
      },
      {
        from: 'hyla',
        title: '另一台机器，或者一部手机',
        body: '地址加密码就连上另一台机器；APK 读同一批会话。',
        points: ['不需要中继和账号', 'Android APK'],
        label: '远程',
      },
    ] as Feature[],
    pocketKicker: '05 — android',
    pocketTitle: '口袋里的 Hyla',
    pocketLead: 'HylaPocket 用 Android 手机读这台机器的会话。',
    pocketAlt: 'Android 上的 Hyla Pocket：会话记录与终端键行',
    pocketPoints: ['每个会话，连同窗格', '键行：esc、tab、ctrl、alt', '分屏，回来接着上次'],
    pocketMeta: ['已签名 APK', 'android 9+', '免费'],
    pocketDownload: '下载 APK',
    pocketReleases: '所有版本',
    pocketSource: '源码',
    pauseTicker: '暂停滚动条',
    playTicker: '继续滚动条',
    ctaKicker: '06 — 开始',
    ctaTitle: '上车吧。',
    ctaLead: '装上 Hyla，从此不再找不到你的智能体。',
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

      {/* The window, as it is */}
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
            <div class="hx-shot">
              <img src="/hyla.jpg" alt={c().appAlt} width={2750} height={1652} loading="lazy" decoding="async" />
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

      {/* What is inside, and where it came from */}
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
                  <div class="hx-card__top">
                    <span class="hx-card__num">{String(i() + 1).padStart(2, '0')}</span>
                    <span class="hx-card__tag" classList={{ 'hx-card__tag--hyla': f.from === 'hyla' }}>
                      {f.from}
                    </span>
                  </div>
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
            <div class="hx-pocket__art">
              <div class="hx-phone-shot">
                <img src="/pocket.jpg" alt={c().pocketAlt} width={1280} height={2772} loading="lazy" decoding="async" />
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
