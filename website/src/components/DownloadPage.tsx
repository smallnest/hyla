import { For, Show, createSignal, onCleanup, onMount } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { Link } from './Link';
import { AppleIcon, ArrowUpRightIcon, CheckIcon, CopyIcon, DownloadIcon, LinuxIcon, PhoneIcon, WindowsIcon } from './Icons';
import { GITHUB_URL } from './SiteHeader';
import { localize, pageHead, type Locale } from '../lib/i18n';

// The page is static, so the latest release is found in the browser: ask
// GitHub for it, keep it for a few hours, and until it answers (or if it
// never does) send every link to the releases page, which is always right.
const VERSION_KEY = 'hyla.release';

function cachedVersion(): string | undefined {
  try {
    const raw = localStorage.getItem(VERSION_KEY);
    if (!raw) return undefined;
    const { v, at } = JSON.parse(raw) as { v?: string; at?: number };
    if (typeof v === 'string' && typeof at === 'number' && Date.now() - at < 6 * 3600_000) return v;
  } catch {
    /* a storage that refuses is a storage we do without */
  }
  return undefined;
}

async function fetchVersion(): Promise<string | undefined> {
  try {
    const res = await fetch('https://api.github.com/repos/smallnest/hyla/releases/latest', {
      headers: { accept: 'application/vnd.github+json' },
    });
    if (!res.ok) return undefined;
    const tag = ((await res.json()) as { tag_name?: string }).tag_name;
    const v = tag?.replace(/^v/, '');
    if (!v) return undefined;
    try {
      localStorage.setItem(VERSION_KEY, JSON.stringify({ v, at: Date.now() }));
    } catch {
      /* the links still work without the cache */
    }
    return v;
  } catch {
    return undefined;
  }
}

const copy = {
  en: {
    title: 'Download | Hyla',
    description: 'Download Hyla, a terminal for coding agents, for macOS, Windows and Linux.',
    heading: 'Download Hyla',
    lead: 'For macOS, Windows and Linux.',
    notes: (v: string) => `${v} release notes`,
    latestNotes: 'Latest release',
    yours: 'Your system',
    platformNotes: {
      mac: 'Install with Homebrew, or open the disk image and drag Hyla into your Applications folder.',
      windows: 'Run the installer, which needs no administrator rights. Hyla needs Windows 10 1809 or later.',
      linux: 'Install the package, or run the script to install Hyla in ~/.local without root.',
    },
    cliTitle: 'Then, Meet the CLI',
    cli: () => (
      <>
        Choose <strong>Hyla › Install Command Line Tool…</strong> to put <code>hyla</code> on your PATH, and let your
        agents drive it.
      </>
    ),
    quickStart: 'Quick Start',
    allReleases: 'All Releases',
    copyCommand: 'Copy command',
    copiedCommand: 'Copied',
    androidNote: 'Hyla in your pocket: the sessions of this machine on a phone, by a tailcat pair code or an address and a password.',
    apk: 'Download APK',
  },
  ja: {
    title: 'ダウンロード | Hyla',
    description: 'コーディングエージェントのためのターミナル Hyla を、macOS、Windows、Linux 向けにダウンロード。',
    heading: 'Hyla をダウンロード',
    lead: 'macOS、Windows、Linux に対応。',
    notes: (v: string) => `${v} リリースノート`,
    latestNotes: '最新リリース',
    yours: 'お使いのシステム',
    platformNotes: {
      mac: 'Homebrew でインストールするか、ディスクイメージを開いて Hyla を“アプリケーション”フォルダにドラッグしてください。',
      windows: 'インストーラを実行してください。管理者権限は不要です。Hyla には Windows 10 1809 以降が必要です。',
      linux: 'パッケージをインストールするか、スクリプトを実行して root なしで ~/.local に Hyla をインストールしてください。',
    },
    cliTitle: '次は CLI を使ってみよう',
    cli: () => (
      <>
        <strong>Hyla › コマンドラインツールをインストール…</strong> を選ぶと <code>hyla</code> が PATH
        に追加され、エージェントから操作できるようになります。
      </>
    ),
    quickStart: 'クイックスタート',
    allReleases: 'すべてのリリース',
    copyCommand: 'コマンドをコピー',
    copiedCommand: 'コピーしました',
    androidNote: 'ポケットの中の Hyla: tailcat のペアコード、またはアドレスとパスワードで、このマシンのセッションをスマートフォンから。',
    apk: 'APK をダウンロード',
  },
  zh: {
    title: '下载 | Hyla',
    description: '下载 Hyla，为编程智能体打造的终端，支持 macOS、Windows 和 Linux。',
    heading: '下载 Hyla',
    lead: '支持 macOS、Windows 和 Linux。',
    notes: (v: string) => `${v} 更新说明`,
    latestNotes: '最新版本',
    yours: '你的系统',
    platformNotes: {
      mac: '用 Homebrew 安装，或打开磁盘映像，把 Hyla 拖进“应用程序”文件夹。',
      windows: '运行安装程序，无需管理员权限。Hyla 需要 Windows 10 1809 或更高版本。',
      linux: '安装软件包，或运行脚本把 Hyla 装到 ~/.local，无需 root。',
    },
    cliTitle: '然后，认识一下命令行',
    cli: () => (
      <>
        选择 <strong>Hyla › 安装命令行工具…</strong>，把 <code>hyla</code> 加入 PATH，让你的智能体来驱动它。
      </>
    ),
    quickStart: '快速上手',
    allReleases: '所有版本',
    copyCommand: '复制命令',
    copiedCommand: '已复制',
    androidNote: '口袋里的 Hyla：用 tailcat 配对码，或地址与密码，在手机上读取这台机器的会话。',
    apk: '下载 APK',
  },
};

export function downloadHead(locale: Locale) {
  return pageHead(locale, '/download', copy[locale].title, copy[locale].description);
}

const POCKET_URL = 'https://github.com/smallnest/hyla-pocket';

const INSTALL_SCRIPTS: Partial<Record<OS, string>> = {
  mac: 'brew tap smallnest/hyla && brew trust smallnest/hyla && brew install --cask hyla',
  linux: `curl -fsSL ${GITHUB_URL}/releases/latest/download/install.sh | sh`,
};

type OS = 'mac' | 'windows' | 'linux';
type Arch = 'arm' | 'x86';

// The release's assets, as mygo build names them since 2.1.22.
const platforms: {
  os: OS;
  name: string;
  icon: typeof AppleIcon;
  files: { label: string; kind: string; arch: Arch; asset: (v: string) => string }[];
}[] = [
  {
    os: 'mac',
    name: 'macOS',
    icon: AppleIcon,
    files: [
      { label: 'Apple Silicon', kind: '.dmg', arch: 'arm', asset: (v) => `hyla-${v}-arm64.dmg` },
      { label: 'Intel', kind: '.dmg', arch: 'x86', asset: (v) => `hyla-${v}-amd64.dmg` },
    ],
  },
  {
    os: 'windows',
    name: 'Windows',
    icon: WindowsIcon,
    files: [
      { label: 'x64', kind: '.exe', arch: 'x86', asset: (v) => `hyla-setup-${v}-amd64.exe` },
      { label: 'ARM64', kind: '.exe', arch: 'arm', asset: (v) => `hyla-setup-${v}-arm64.exe` },
    ],
  },
  {
    os: 'linux',
    name: 'Linux',
    icon: LinuxIcon,
    files: [
      { label: 'x64', kind: '.deb', arch: 'x86', asset: (v) => `hyla_${v}_amd64.deb` },
      { label: 'ARM64', kind: '.deb', arch: 'arm', asset: (v) => `hyla_${v}_arm64.deb` },
    ],
  },
];

type UAData = { platform?: string; getHighEntropyValues?: (hints: string[]) => Promise<{ architecture?: string }> };

function detectOS(): OS | undefined {
  const ua = navigator.userAgent;
  const platform = (navigator as { userAgentData?: UAData }).userAgentData?.platform || navigator.platform || ua;
  if (/mac/i.test(platform)) return 'mac';
  if (/win/i.test(platform)) return 'windows';
  if (/linux|x11/i.test(platform) && !/android/i.test(ua)) return 'linux';
}

// Only Chromium tells the architecture; elsewhere the first file is the pick.
async function detectArch(): Promise<Arch | undefined> {
  const data = (navigator as { userAgentData?: UAData }).userAgentData;
  const arch = await data?.getHighEntropyValues?.(['architecture']).then((v) => v.architecture, () => undefined);
  if (arch === 'arm') return 'arm';
  if (arch === 'x86') return 'x86';
}

// The command without the `$` prompt, and a copy control visitors look for
// beside it. Each card keeps its own copied state.
function InstallScript(props: { script: string; copyLabel: string; copiedLabel: string }) {
  const [copied, setCopied] = createSignal(false);
  let reset: number | undefined;
  onCleanup(() => clearTimeout(reset));

  const copy = () => {
    navigator.clipboard?.writeText(props.script).then(
      () => {
        setCopied(true);
        clearTimeout(reset);
        reset = window.setTimeout(() => setCopied(false), 2000);
      },
      () => {},
    );
  };

  return (
    <div class="platform__script">
      <pre>
        <span class="t-accent">$</span> {props.script}
      </pre>
      <button
        type="button"
        class="platform__copy"
        classList={{ 'is-copied': copied() }}
        title={copied() ? props.copiedLabel : props.copyLabel}
        aria-label={copied() ? props.copiedLabel : props.copyLabel}
        onClick={copy}
      >
        <Show when={copied()} fallback={<CopyIcon size={14} />}>
          <CheckIcon size={14} />
        </Show>
      </button>
    </div>
  );
}

export default function DownloadPage(props: { locale: Locale }) {
  const c = () => copy[props.locale];
  const [os, setOS] = createSignal<OS>();
  const [arch, setArch] = createSignal<Arch>();
  const [version, setVersion] = createSignal<string>();

  onMount(() => {
    setOS(detectOS());
    detectArch().then(setArch);
    setVersion(cachedVersion());
    fetchVersion().then((v) => v && setVersion(v));
  });

  const tag = () => (version() ? `v${version()}` : undefined);
  const href = (asset: (v: string) => string) => {
    const v = version();
    return v ? `${GITHUB_URL}/releases/download/v${v}/${asset(v)}` : `${GITHUB_URL}/releases/latest`;
  };
  const isPick = (p: (typeof platforms)[number], f: (typeof platforms)[number]['files'][number]) =>
    p.os === os() && (arch() ? f.arch === arch() : f === p.files[0]);

  return (
    <main class="download">
      <section class="wrapper download-head">
        <img src="/icon.png" alt="" width={88} height={88} class="cta__icon" />
        <h1 class="display">{c().heading}</h1>
        <p class="lead lead--center">{c().lead}</p>
        <p class="download-head__version">
          <a href={version() ? `${GITHUB_URL}/releases/tag/${tag()}` : `${GITHUB_URL}/releases/latest`}>
            {version() ? c().notes(tag()!) : c().latestNotes} <ArrowUpRightIcon size={13} />
          </a>
        </p>
      </section>

      <section class="wrapper platforms">
        <For each={platforms}>
          {(p) => (
            <article class="platform" classList={{ 'is-yours': p.os === os() }}>
              <div class="platform__head">
                <Dynamic component={p.icon} size={26} class="platform__icon" />
                <h2>{p.name}</h2>
                <Show when={p.os === os()}>
                  <span class="platform__badge">{c().yours}</span>
                </Show>
              </div>
              <p>{c().platformNotes[p.os]}</p>
              <div class="platform__files">
                <For each={p.files}>
                  {(f) => (
                    <a href={href(f.asset)} class="button" classList={{ 'button--primary': isPick(p, f) }}>
                      <DownloadIcon size={16} />
                      {f.label}
                      <span class="platform__kind">{f.kind}</span>
                    </a>
                  )}
                </For>
              </div>
              <Show when={INSTALL_SCRIPTS[p.os]}>
                {(script) => (
                  <InstallScript script={script()} copyLabel={c().copyCommand} copiedLabel={c().copiedCommand} />
                )}
              </Show>
            </article>
          )}
        </For>
        <article class="platform">
          <div class="platform__head">
            <PhoneIcon size={26} class="platform__icon" />
            <h2>Android</h2>
          </div>
          <p>{c().androidNote}</p>
          <div class="platform__files">
            <a href={`${POCKET_URL}/releases/latest`} class="button button--primary">
              <DownloadIcon size={16} />
              {c().apk}
              <span class="platform__kind">.apk</span>
            </a>
            <a href={`${POCKET_URL}/releases`} class="button">
              {c().allReleases}
            </a>
          </div>
        </article>
      </section>

      <section class="wrapper cta">
        <h2 class="title">{c().cliTitle}</h2>
        <p class="lead lead--center">{c().cli()}</p>
        <div class="actions actions--center">
          <Link href={localize('/docs/quick-start', props.locale)} class="button button--primary">
            {c().quickStart}
          </Link>
          <a href={`${GITHUB_URL}/releases`} class="button">
            {c().allReleases}
          </a>
        </div>
      </section>
    </main>
  );
}
