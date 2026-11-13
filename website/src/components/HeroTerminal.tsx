import { For, Show, createSignal, onCleanup, onMount } from 'solid-js';

/** A line of the demo terminal: a command being typed, its output, or the
 * status it ends with. */
type Line = { kind: 'cmd' | 'out' | 'ok'; text: string };

/**
 * HeroTerminal plays a small Hyla session in a terminal: the command types
 * itself, the session appears, and the turn ends. It loops while it shows,
 * stops off screen, and shows the whole thing at once for a reader who asked
 * for less motion.
 */
export default function HeroTerminal(props: {
  labels: { done: string; working: string };
}) {
  const scenario: Line[] = [
    { kind: 'cmd', text: 'hyla new -n review -- claude' },
    { kind: 'out', text: '◇ m4e7b1  review · /src  …  ' + props.labels.working },
    { kind: 'out', text: '◇ split right · ghostty  ready' },
    { kind: 'ok', text: '✓ ' + props.labels.done + ' · 12.4s' },
  ];
  const command = scenario[0]!.text;

  // The whole turn shows first, which is what a page without JavaScript, a
  // reader of less motion, and a snapshot of the page get; the script then
  // plays it from the top.
  const [typed, setTyped] = createSignal(command);
  const [lines, setLines] = createSignal<Line[]>(scenario);
  const [busy, setBusy] = createSignal(false);

  let host!: HTMLDivElement | undefined;
  let timer: number | undefined;
  let observer: IntersectionObserver | undefined;

  const reduced = () =>
    typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

  const settle = () => {
    setTyped(command);
    setLines(scenario);
    setBusy(false);
    if (timer !== undefined) window.clearInterval(timer);
    timer = undefined;
  };

  const play = () => {
    if (timer !== undefined || reduced()) return;
    let at = 0;
    let printed = 0;
    let paused = 0;
    setTyped('');
    setLines([]);
    setBusy(true);
    timer = window.setInterval(() => {
      if (paused > 0) {
        paused -= 40;
        return;
      }
      if (at < command.length) {
        at += 1;
        setTyped(command.slice(0, at));
        return;
      }
      if (printed < scenario.length - 1) {
        setLines((ls) => [...ls, scenario[++printed]!]);
        paused = 420;
        return;
      }
      // The turn is over: let it rest, then run it again.
      setBusy(false);
      if (timer !== undefined) window.clearInterval(timer);
      timer = window.setTimeout(() => {
        timer = undefined;
        if (visible) play();
      }, 3600);
    }, 40);
  };

  let visible = true;

  onMount(() => {
    if (reduced()) {
      settle();
      return;
    }
    if (typeof IntersectionObserver === 'function' && host) {
      observer = new IntersectionObserver(
        (entries) => {
          visible = entries.some((e) => e.isIntersecting);
          if (visible) play();
          else if (timer !== undefined) {
            window.clearInterval(timer);
            timer = undefined;
          }
        },
        { threshold: 0.15 },
      );
      observer.observe(host);
    } else {
      play();
    }
  });

  onCleanup(() => {
    observer?.disconnect();
    if (timer !== undefined) {
      window.clearInterval(timer);
      window.clearTimeout(timer);
    }
  });

  return (
    <div class="hx-term" ref={host}>
      <div class="hx-term__bar" aria-hidden="true">
        <i />
        <i />
        <i />
        <span>hyla — review</span>
      </div>
      <div class="hx-term__body" role="img" aria-label={`hyla new -n review -- claude, then ${props.labels.done}`}>
        <span class="hx-term__line">
          <span class="hx-prompt">$ </span>
          <span class="hx-cmd">{typed()}</span>
          <Show when={busy()}>
            <span class="hx-caret" />
          </Show>
        </span>
        <For each={lines()}>
          {(l) => (
            <span class="hx-term__line" classList={{ 'hx-ok': l.kind === 'ok', 'hx-out': l.kind === 'out' }}>
              {l.text}
            </span>
          )}
        </For>
      </div>
    </div>
  );
}
