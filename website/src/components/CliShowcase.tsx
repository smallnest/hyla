import { createSignal, For } from 'solid-js';

type Line = { cmd?: string; out?: string; dim?: boolean };

const tabs: { id: string; lines: Line[] }[] = [
  {
    id: 'sessions',
    lines: [
      { cmd: 'hyla new -n fix-login -- claude' },
      { out: 'k3f9a2' },
      { cmd: 'hyla ls' },
      { out: 'hyla' },
      { out: '  k3f9a2  fix-login       working   claude' },
      { out: '  p81c0d  review-pr-42    waiting   codex' },
      { out: '  d07e11  dev server      idle      zsh' },
    ],
  },
  {
    id: 'split',
    lines: [
      { cmd: 'hyla split --down -- npm run dev' },
      { out: 'p2c4e8' },
      { cmd: 'hyla wait p2c4e8 --for idle --timeout 1m' },
      { out: '  VITE v8.0.0  ready in 212 ms' },
      { out: '  ➜  Local:   http://localhost:5173/' },
    ],
  },
  {
    id: 'prompt',
    lines: [
      { cmd: 'hyla prompt review "summarize the diff in two lines"' },
      { out: '● The redirect now keeps its query string after the' },
      { out: '  session expires; loginURL() builds it from the request.' },
      { cmd: 'hyla read tests -n 3' },
      { out: 'ok   hyla/auth   0.412s' },
    ],
  },
  {
    id: 'remote',
    lines: [
      { cmd: 'hyla pocket tcp enable 0.0.0.0:7297' },
      { cmd: 'hyla pocket addr' },
      { out: '192.168.1.5:7297' },
      { out: '# add it on another machine, or scan the code on a phone', dim: true },
    ],
  },
  {
    id: 'report',
    lines: [
      { cmd: 'hyla report working "Migrating the database" --progress 40' },
      { cmd: 'hyla report blocked "Approve the production deploy?"' },
      { out: '# the session turns to waiting, and Hyla notifies you', dim: true },
      { cmd: 'hyla report done "Deployed to staging"' },
    ],
  },
];

export default function CliShowcase() {
  const [tab, setTab] = createSignal(tabs[0].id);
  const current = () => tabs.find((t) => t.id === tab())!;
  return (
    <div class="cli-stage">
      <div class="cli-stage__rays" aria-hidden="true" />
      <div class="cli-window">
        {/* Keyed on the tab, so its lines play in again. */}
        <For each={[current()]}>
          {(t) => (
            <pre class="cli-window__body">
              <For each={t.lines}>
                {(line, i) => (
                  <span class="cli-line" style={{ 'animation-delay': `${i() * 90}ms` }}>
                    {line.cmd ? (
                      <>
                        <span class="t-accent">$</span> {line.cmd}
                      </>
                    ) : (
                      <span class={line.dim ? 't-dim' : 't-out'}>{line.out}</span>
                    )}
                    {'\n'}
                  </span>
                )}
              </For>
              <span class="cli-line" style={{ 'animation-delay': `${t.lines.length * 90}ms` }}>
                <span class="t-accent">$</span> <span class="cursor" />
              </span>
            </pre>
          )}
        </For>
        <div class="cli-tabs" role="tablist">
          <For each={tabs}>
            {(t) => (
              <button
                role="tab"
                aria-selected={t.id === tab()}
                classList={{ 'is-active': t.id === tab() }}
                onClick={() => setTab(t.id)}
              >
                {t.id}
              </button>
            )}
          </For>
        </div>
      </div>
    </div>
  );
}
