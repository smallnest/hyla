import { onCleanup, onMount } from 'solid-js';

// A wall of live sessions, receding upward: every pane is an agent or a
// shell at work, and the one that passes the eye line is lit, so the wall
// reads as "many agents at once, one of them needs you". The columns drift
// at different phases, the pointer moves the wall a little, and a click
// sends a ring across it.
//
// Nothing here is random: the panes come from a fixed list, so the page
// looks the same on every load.

type Status = 'working' | 'waiting' | 'done' | 'idle';

type Pane = {
  name: string;
  agent: string;
  status: Status;
  lines: [string, string];
};

const PANES: Pane[] = [
  { name: 'review', agent: 'claude', status: 'waiting', lines: ['● Ran bun test', '✓ 3 pass · 5 expect()'] },
  { name: 'build', agent: 'vite', status: 'working', lines: ['$ vite build', '✓ built in 1.42s'] },
  { name: 'docs', agent: 'zsh', status: 'idle', lines: ['$ ls docs/', 'agents.md  cli.md'] },
  { name: 'migrate', agent: 'codex', status: 'working', lines: ['● ALTER TABLE users', '● 4.8k rows moved'] },
  { name: 'dev', agent: 'node', status: 'working', lines: ['$ bun run dev', 'localhost:8787'] },
  { name: 'tests', agent: 'go test', status: 'done', lines: ['✓ ok hyla 2.14s', '✓ ok internal 0.81s'] },
  { name: 'lint', agent: 'zsh', status: 'idle', lines: ['$ go vet ./...', 'no issues found'] },
  { name: 'ship', agent: 'claude', status: 'waiting', lines: ['? push to master?', '[y/N]  waiting'] },
  { name: 'infra', agent: 'ssh', status: 'working', lines: ['$ kubectl get pods', 'api-7f9c  Running'] },
  { name: 'release', agent: 'mygo', status: 'done', lines: ['✓ v3.8.1 tagged', '12 assets uploaded'] },
  { name: 'pocket', agent: 'adb', status: 'idle', lines: ['$ adb install', 'Success · 41 MB'] },
  { name: 'logs', agent: 'tail', status: 'done', lines: ['$ tail -f hylad.log', 'ready · 8421'] },
];

const STATUS: Record<Status, { color: string; label: string }> = {
  working: { color: '#fb6f02', label: 'working' },
  waiting: { color: '#b78cff', label: 'waiting' },
  done: { color: '#7de08a', label: 'done' },
  idle: { color: '#6b736d', label: 'idle' },
};

const MONO = 'ui-monospace, "SF Mono", Menlo, monospace';

const BASE_W = 176; // a pane, before its scale
const BASE_H = 92;
const GAP = 14;
const STEP = BASE_H + GAP;
const SPEED = 13; // px a second

// The dust that hangs over the wall: the same trick as the old hero's cloud,
// a pre-rendered glow per colour, drawn additively. Each speck has a depth,
// so the near ones are bigger and brighter and lean further with the pointer.
const DUST_TINTS = ['#fb6f02', '#ffb35c', '#6fe3ff', '#b78cff'];
const DUST_ALPHA = 0.34;

type Speck = { x: number; y: number; z: number; vx: number; vy: number; tint: number };
type Spark = { x: number; y: number; vx: number; vy: number; born: number; life: number; tint: number };

// A glow, once, from white at the centre through its colour to nothing, as
// the old cloud drew its points.
function glowSprite(color: string) {
  const size = 48;
  const c = document.createElement('canvas');
  c.width = c.height = size;
  const g = c.getContext('2d')!;
  const grad = g.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
  grad.addColorStop(0, '#ffffff');
  grad.addColorStop(0.2, color);
  grad.addColorStop(0.5, `${color}55`);
  grad.addColorStop(1, `${color}00`);
  g.fillStyle = grad;
  g.fillRect(0, 0, size, size);
  return c;
}

// A small seeded random, so the dust is the same on every load.
function mulberry32(seed: number) {
  return () => {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export default function SessionWall() {
  let host!: HTMLDivElement;
  let canvas!: HTMLCanvasElement;

  onMount(() => {
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    let w = 0;
    let h = 0;
    let dpr = 1;
    let cols = 3;
    let rows = 6;

    let frame = 0;
    let running = false;
    let visible = true;
    let started = 0;

    // Pointer parallax, eased a frame at a time.
    let aimX = 0;
    let aimY = 0;
    let atX = 0;
    let atY = 0;

    // Clicks, as rings that spread and go.
    const rings: { x: number; y: number; born: number }[] = [];

    // The dust and the sparks a lit pane gives off.
    const rand = mulberry32(0x51ed270b);
    const sprites = DUST_TINTS.map(glowSprite);
    let dust: Speck[] = [];
    const sparks: Spark[] = [];
    let lastEmit = 0;
    let lastFrame = 0;

    // Where the pointer is, in the frame: the dust leans away from it.
    let px = -9999;
    let py = -9999;

    const resize = () => {
      const r = host.getBoundingClientRect();
      w = Math.max(1, Math.round(r.width));
      h = Math.max(1, Math.round(r.height));
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = Math.round(w * dpr);
      canvas.height = Math.round(h * dpr);
      canvas.style.width = `${w}px`;
      canvas.style.height = `${h}px`;
      cols = w < 430 ? 2 : 3;
      const fit = Math.min(1, w / (cols * (BASE_W + GAP) + 32));
      rows = Math.max(5, Math.ceil(h / (STEP * fit)) + 2);
      atX = aimX = 0;
      atY = aimY = 0;

      const count = Math.round(Math.min(92, Math.max(18, (w * h) / 4300)));
      dust = Array.from({ length: count }, () => ({
        x: rand() * w,
        y: rand() * h,
        z: rand(),
        vx: (rand() - 0.5) * 3,
        vy: 0,
        tint: Math.floor(rand() * DUST_TINTS.length),
      }));

      paint(reduce ? 2600 : performance.now());
    };

    const draw = (now: number) => {
      const time = (now - started) / 1000;
      // The wall is drawn at a size that fits the frame: narrower ones get a
      // smaller wall, not a cropped one.
      const fit = Math.min(1, w / (cols * (BASE_W + GAP) + 32));
      const step = STEP * fit;
      const offset = SPEED * time;
      const total = rows * step;
      const target = h * 0.58;

      // Ease the pointer, so the wall leans rather than jumps.
      atX += (aimX - atX) * 0.08;
      atY += (aimY - atY) * 0.08;

      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);
      ctx.textAlign = 'left';
      ctx.textBaseline = 'alphabetic';

      const panes: { x: number; y: number; s: number; alpha: number; spot: number; d: number; c: number; pane: Pane }[] = [];

      for (let c = 0; c < cols; c++) {
        for (let r = 0; r < rows; r++) {
          let y = r * step - offset;
          y = ((y % total) + total) % total; // 0 .. total
          const top = y - step; // -step .. total - step

          // Nearer the bottom, bigger and brighter: the wall recedes upward.
          const k = Math.min(1, Math.max(0, (top + BASE_H * 0.5) / h));
          const s = (0.6 + 0.48 * k) * fit;
          const pane = PANES[(r + c * 5) % PANES.length];

          const fade =
            Math.min(1, Math.max(0, top / 90)) *
            Math.min(1, Math.max(0, (h - top - BASE_H * s) / 70));
          const alpha = fade * (0.24 + 0.76 * k);
          if (alpha <= 0.01) continue;

          const cx = w / 2 + (c - (cols - 1) / 2) * (BASE_W + GAP) * s + atX * 12 * (0.4 + k);
          const x = cx - (BASE_W * s) / 2;
          const cy = top + atY * 10 * (0.4 + k);
          const d = Math.abs(cy + (BASE_H * s) / 2 - target);

          panes.push({ x, y: cy, s, alpha, spot: 0, d, c, pane });
        }
      }

      // Only one pane is lit: the middle column's, nearest the eye line. It
      // is the one the wall is saying "this one needs you" about, so it comes
      // up a size, wears an orange rim and comes to the front.
      const mid = Math.floor(cols / 2);
      let lit = -1;
      let best = Infinity;
      const pick = (onlyMiddle: boolean) => {
        for (let i = 0; i < panes.length; i++) {
          const p = panes[i];
          if (p.alpha < 0.55) continue;
          if (onlyMiddle && p.c !== mid) continue;
          if (p.d < best) {
            best = p.d;
            lit = i;
          }
        }
      };
      pick(true);
      if (lit < 0) pick(false);

      if (lit >= 0) {
        const p = panes[lit];
        p.spot = Math.min(1, Math.max(0.35, 1 - p.d / 160));
        const grow = 1 + 0.09 * p.spot;
        p.x -= (BASE_W * p.s * (grow - 1)) / 2;
        p.y -= (BASE_H * p.s * (grow - 1)) / 2;
        p.s *= grow;
      }

      panes.sort((a, b) => (a.spot > 0 ? 1 : 0) - (b.spot > 0 ? 1 : 0) || a.y - b.y);

      for (const p of panes) {
        drawPane(ctx, p.pane, p.x, p.y, p.s, p.alpha, p.spot, time);
      }

      // The dust drifts up, leans with the pointer and is pushed away by it,
      // and the lit pane keeps giving off sparks.
      const dt = lastFrame ? Math.min(0.05, (now - lastFrame) / 1000) : 1 / 60;
      lastFrame = now;

      if (!reduce) {
        for (const d of dust) {
          d.x += (d.vx + Math.sin(time * 0.35 + d.z * 7) * 2.4) * dt;
          d.y += (d.vy - (7 + d.z * 13)) * dt;

          if (px > -9000) {
            const dx = d.x - px;
            const dy = d.y - py;
            const dist = Math.hypot(dx, dy);
            if (dist < 84 && dist > 0.01) {
              const push = (1 - dist / 84) * 170 * dt;
              d.vx += (dx / dist) * push;
              d.vy += (dy / dist) * push;
            }
          }

          const damp = Math.pow(0.12, dt);
          d.vx *= damp;
          d.vy *= damp;

          if (d.y < -26) {
            d.y = h + 26;
            d.x = rand() * w;
            d.vx = (rand() - 0.5) * 3;
          }
          if (d.x < -26) d.x = w + 26;
          if (d.x > w + 26) d.x = -26;
        }

        if (lit >= 0 && time - lastEmit > 1.35) {
          lastEmit = time;
          const p = panes[lit];
          const cx = p.x + (BASE_W * p.s) / 2;
          const burst = w < 430 ? 3 : 5;
          for (let i = 0; i < burst; i++) {
            sparks.push({
              x: cx + (rand() - 0.5) * BASE_W * p.s * 0.6,
              y: p.y + rand() * 6,
              vx: (rand() - 0.5) * 14,
              vy: -(16 + rand() * 26),
              born: now,
              life: 1300 + rand() * 900,
              tint: rand() < 0.7 ? 0 : 1,
            });
          }
        }
      }

      drawDust(ctx, dust, sprites, atX, atY, time);
      drawSparks(ctx, sparks, sprites, now);
      ctx.globalCompositeOperation = 'source-over';
      ctx.globalAlpha = 1;

      // Clicks: a ring that spreads and fades.
      for (let i = rings.length - 1; i >= 0; i--) {
        const ring = rings[i];
        const age = (now - ring.born) / 900;
        if (age >= 1) {
          rings.splice(i, 1);
          continue;
        }
        ctx.beginPath();
        ctx.arc(ring.x, ring.y, 10 + age * 90, 0, Math.PI * 2);
        ctx.strokeStyle = `rgba(251, 111, 2, ${0.55 * (1 - age)})`;
        ctx.lineWidth = 1.2;
        ctx.stroke();
      }

      // The wall's top and bottom fade into the frame, which hides the
      // panes that wrap around.
      const topH = Math.min(104, Math.max(52, h * 0.18));
      const topFade = ctx.createLinearGradient(0, 0, 0, topH);
      topFade.addColorStop(0, 'rgba(14, 16, 18, 0.99)');
      topFade.addColorStop(1, 'rgba(14, 16, 18, 0)');
      ctx.fillStyle = topFade;
      ctx.fillRect(0, 0, w, topH);

      const bottomH = Math.min(118, Math.max(58, h * 0.2));
      const bottomFade = ctx.createLinearGradient(0, h - bottomH, 0, h);
      bottomFade.addColorStop(0, 'rgba(14, 16, 18, 0)');
      bottomFade.addColorStop(1, 'rgba(14, 16, 18, 0.99)');
      ctx.fillStyle = bottomFade;
      ctx.fillRect(0, h - bottomH, w, bottomH);
    };

    const paint = (now: number) => {
      if (!started) started = now - (reduce ? 2600 : 0);
      draw(now);
    };

    const loop = (now: number) => {
      if (!running) return;
      draw(now);
      frame = requestAnimationFrame(loop);
    };

    const start = () => {
      if (running || reduce) return;
      running = true;
      frame = requestAnimationFrame(loop);
    };

    const stop = () => {
      running = false;
      if (frame) cancelAnimationFrame(frame);
      frame = 0;
    };

    if (!reduce) {
      const io = new IntersectionObserver(
        (entries) => {
          visible = entries.some((e) => e.isIntersecting);
          if (visible && !document.hidden) start();
          else stop();
        },
        { threshold: 0.05 },
      );
      io.observe(host);
      onCleanup(() => io.disconnect());
    }

    const onVisibility = () => {
      if (document.hidden) stop();
      else if (visible) start();
    };

    const onPointer = (event: PointerEvent) => {
      const r = host.getBoundingClientRect();
      aimX = (event.clientX - r.left) / r.width - 0.5;
      aimY = (event.clientY - r.top) / r.height - 0.5;
      px = event.clientX - r.left;
      py = event.clientY - r.top;
    };

    const onLeave = () => {
      aimX = 0;
      aimY = 0;
      px = -9999;
      py = -9999;
    };

    const onClick = (event: PointerEvent) => {
      if (reduce) return;
      const r = host.getBoundingClientRect();
      rings.push({ x: event.clientX - r.left, y: event.clientY - r.top, born: performance.now() });
      if (rings.length > 3) rings.shift();
    };

    host.addEventListener('pointermove', onPointer, { passive: true });
    host.addEventListener('pointerleave', onLeave, { passive: true });
    host.addEventListener('pointerdown', onClick, { passive: true });
    document.addEventListener('visibilitychange', onVisibility);

    const ro = new ResizeObserver(resize);
    ro.observe(host);
    resize();
    if (!reduce) start();

    onCleanup(() => {
      stop();
      ro.disconnect();
      host.removeEventListener('pointermove', onPointer);
      host.removeEventListener('pointerleave', onLeave);
      host.removeEventListener('pointerdown', onClick);
      document.removeEventListener('visibilitychange', onVisibility);
    });
  });

  return (
    <div class="morph" ref={host} aria-hidden="true">
      <canvas ref={canvas} />
    </div>
  );
}

function drawDust(
  ctx: CanvasRenderingContext2D,
  dust: Speck[],
  sprites: HTMLCanvasElement[],
  leanX: number,
  leanY: number,
  time: number,
) {
  ctx.globalCompositeOperation = 'lighter';
  for (const d of dust) {
    const r = 3.2 + d.z * 6.4;
    const x = d.x + leanX * (6 + d.z * 18);
    const y = d.y + leanY * (4 + d.z * 12) + Math.sin(time * 0.8 + d.z * 9) * 1.6;
    ctx.globalAlpha = (0.1 + d.z * DUST_ALPHA) * 0.9;
    ctx.drawImage(sprites[d.tint], x - r, y - r, r * 2, r * 2);
  }
  ctx.globalAlpha = 1;
}

function drawSparks(
  ctx: CanvasRenderingContext2D,
  sparks: Spark[],
  sprites: HTMLCanvasElement[],
  now: number,
) {
  ctx.globalCompositeOperation = 'lighter';
  for (let i = sparks.length - 1; i >= 0; i--) {
    const s = sparks[i];
    const age = (now - s.born) / s.life;
    if (age >= 1) {
      sparks.splice(i, 1);
      continue;
    }
    if (age > 0) {
      s.x += (s.vx * (1 - age)) * 0.016;
      s.y += (s.vy * (1 - age * 0.4)) * 0.016;
    }
    const r = 5 - age * 3;
    ctx.globalAlpha = (1 - age) * 0.8;
    ctx.drawImage(sprites[s.tint], s.x - r, s.y - r, r * 2, r * 2);
  }
  ctx.globalAlpha = 1;
}

function drawPane(
  ctx: CanvasRenderingContext2D,
  pane: Pane,
  x: number,
  y: number,
  s: number,
  alpha: number,
  spot: number,
  time: number,
) {
  const status = STATUS[pane.status];
  ctx.save();
  ctx.globalAlpha = alpha;
  ctx.translate(x, y);
  ctx.scale(s, s);

  // The light under the pane the eye line is on.
  if (spot > 0.02) {
    const halo = ctx.createRadialGradient(BASE_W / 2, BASE_H / 2, 8, BASE_W / 2, BASE_H / 2, BASE_W * 0.95);
    halo.addColorStop(0, `rgba(251, 111, 2, ${0.16 * spot})`);
    halo.addColorStop(1, 'rgba(251, 111, 2, 0)');
    ctx.fillStyle = halo;
    ctx.fillRect(-BASE_W * 0.5, -BASE_H * 0.7, BASE_W * 2, BASE_H * 2.4);
  }

  // The pane itself.
  rounded(ctx, 0, 0, BASE_W, BASE_H, 9);
  ctx.fillStyle = spot > 0.15 ? 'rgba(20, 17, 14, 0.96)' : 'rgba(11, 13, 14, 0.94)';
  ctx.fill();
  ctx.lineWidth = 1;
  ctx.strokeStyle = spot > 0.15 ? `rgba(251, 111, 2, ${0.3 + 0.45 * spot})` : 'rgba(255, 255, 255, 0.09)';
  ctx.stroke();

  if (spot > 0.15) {
    const phase = (time % 2.4) / 2.4;
    const pad = 3 + phase * 22;
    ctx.save();
    ctx.globalAlpha = alpha * (1 - phase) * 0.55 * spot;
    rounded(ctx, -pad, -pad, BASE_W + pad * 2, BASE_H + pad * 2, 9 + pad * 0.6);
    ctx.strokeStyle = 'rgba(251, 111, 2, 1)';
    ctx.lineWidth = 1;
    ctx.stroke();
    ctx.restore();

    ctx.save();
    ctx.shadowColor = 'rgba(251, 111, 2, 0.55)';
    ctx.shadowBlur = 16 * spot;
    rounded(ctx, 0.5, 0.5, BASE_W - 1, BASE_H - 1, 9);
    ctx.strokeStyle = `rgba(251, 111, 2, ${0.5 + 0.4 * spot})`;
    ctx.lineWidth = 1.2;
    ctx.stroke();
    ctx.restore();
  }

  // Its title bar.
  ctx.beginPath();
  ctx.moveTo(0, 24);
  ctx.lineTo(BASE_W, 24);
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.07)';
  ctx.lineWidth = 1;
  ctx.stroke();

  const lights = ['#ff5f57', '#febc2e', '#28c840'];
  for (let i = 0; i < 3; i++) {
    ctx.beginPath();
    ctx.arc(13 + i * 9, 12, 2.8, 0, Math.PI * 2);
    ctx.fillStyle = lights[i];
    ctx.fill();
  }

  ctx.textAlign = 'left';
  ctx.font = `600 10px ${MONO}`;
  ctx.fillStyle = '#d7ddd8';
  ctx.fillText(pane.name, 45, 16);

  ctx.textAlign = 'right';
  ctx.font = `400 9px ${MONO}`;
  ctx.fillStyle = '#6b736d';
  ctx.fillText(pane.agent, BASE_W - 12, 16);
  ctx.textAlign = 'left';

  // Two lines of what it is doing.
  ctx.font = `400 9.5px ${MONO}`;
  ctx.fillStyle = '#9aa39c';
  ctx.fillText(pane.lines[0], 12, 42);
  ctx.fillStyle = '#6b736d';
  ctx.fillText(pane.lines[1], 12, 57);

  // And its status, with the working ones beating.
  const pulse = pane.status === 'working' ? 0.5 + 0.5 * Math.sin(time * 3) : 1;
  ctx.beginPath();
  ctx.arc(14, BASE_H - 15, 3, 0, Math.PI * 2);
  ctx.fillStyle = status.color;
  ctx.globalAlpha = alpha * (0.45 + 0.55 * pulse);
  ctx.fill();
  ctx.globalAlpha = alpha;

  ctx.font = `400 9px ${MONO}`;
  ctx.fillStyle = status.color;
  const dots = pane.status === 'working' ? '.'.repeat(1 + (Math.floor(time * 2) % 3)) : '';
  ctx.fillText(status.label + dots, 23, BASE_H - 11);

  ctx.restore();
}

function rounded(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}
