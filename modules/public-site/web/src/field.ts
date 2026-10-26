import { clock, createRenderer } from './field-core';
import type { FieldMessage } from './field.worker';

// 字母点阵背景（绘制见 field-core.ts）：画布单独一个合成层，绘制放进 Web Worker（独立线程），主线程忙时点阵照常流动；
// 浏览器不支持把画布交给 Worker 时退回主线程绘制。首页默认不显示，点进 AI 问答时从点击处铺开；问答页铺满点阵并盖一层磨砂玻璃。
// 首页进问答是页内切换（见 welcome.ts），画布和 Worker 一直不停；直接打开问答页时，若刚从首页跳过来（页内切换失败的退路），接着那次铺开。

const entryKey = 'nw-field-entry';

export type LetterField = {
  // 从点击处铺开点阵（进入 AI 问答），并记下来，万一要整页跳转，问答页还能接着铺。
  enter(x: number, y: number): void;
  // 接上首页留下的铺开；没有（直接打开问答页、刷新）就返回 false。
  resume(): boolean;
  fade(to: number, ms: number): Promise<void>;
};

let shared: LetterField | null = null;

// 一个页面只有一份点阵：页内切换后继续用同一块画布。
export function startLetterField(initial: number): LetterField {
  if (shared) return shared;
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const canvas = document.createElement('canvas');
  canvas.className = 'nw-field';
  canvas.setAttribute('aria-hidden', 'true');
  document.body.prepend(canvas);
  const size = () => ({ width: window.innerWidth, height: window.innerHeight, ratio: Math.min(2, window.devicePixelRatio || 1) });
  const color = () => getComputedStyle(canvas).color;
  let send: (message: FieldMessage) => void;
  if ('transferControlToOffscreen' in canvas) {
    const worker = new Worker(new URL('./field.worker.ts', import.meta.url), { type: 'module' });
    const offscreen = canvas.transferControlToOffscreen();
    worker.postMessage({ type: 'init', canvas: offscreen, ...size(), color: color(), visible: initial, still } satisfies FieldMessage, [offscreen]);
    send = (message) => worker.postMessage(message);
  } else {
    const renderer = createRenderer(canvas, { visible: initial, still, color: color() });
    const { width, height, ratio } = size();
    renderer.resize(width, height, ratio);
    const loop = (now: number) => { renderer.frame(now); requestAnimationFrame(loop); };
    requestAnimationFrame(loop);
    send = (message) => {
      if (message.type === 'resize') renderer.resize(message.width, message.height, message.ratio);
      else if (message.type === 'color') renderer.color(message.value);
      else if (message.type === 'enter') renderer.enter(message.x, message.y, message.t0);
      else if (message.type === 'fade') renderer.fade(message.to, message.ms);
    };
  }
  // 只在窗口尺寸真的变了才重建网格；页面内容变高变矮不影响点阵。
  let last = size();
  window.addEventListener('resize', () => {
    const next = size();
    if (next.width === last.width && next.height === last.height && next.ratio === last.ratio) return;
    last = next;
    send({ type: 'resize', ...next });
  });
  // 切换明暗主题时字母换成新的前景色。
  new MutationObserver(() => send({ type: 'color', value: color() })).observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'data-theme'] });

  const start = (x: number, y: number, t0: number) => send({ type: 'enter', x, y, t0 });
  shared = {
    enter(x, y) {
      const t0 = clock();
      start(x, y, t0);
      try { sessionStorage.setItem(entryKey, JSON.stringify({ x: x / last.width, y: y / last.height, t0 })); } catch { /* 隐私模式 */ }
    },
    resume() {
      let entry: { x: number; y: number; t0: number } | null = null;
      try {
        entry = JSON.parse(sessionStorage.getItem(entryKey) ?? 'null');
        sessionStorage.removeItem(entryKey);
      } catch { /* 隐私模式或记录损坏 */ }
      if (!entry || clock() - entry.t0 > 4) return false;
      start(entry.x * last.width, entry.y * last.height, entry.t0);
      return true;
    },
    fade(to, ms) {
      send({ type: 'fade', to, ms });
      return new Promise((done) => setTimeout(done, still ? 0 : ms));
    },
  };
  return shared;
}

// 页内切换成功后用不上整页跳转的续接记录，清掉。
export function forgetEntry() {
  try { sessionStorage.removeItem(entryKey); } catch { /* 隐私模式 */ }
}
